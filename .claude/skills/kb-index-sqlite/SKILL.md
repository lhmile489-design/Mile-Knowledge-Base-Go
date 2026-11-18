---
name: kb-index-sqlite
description: Mile Knowledge Base（Go 重写版）的 SQLite FTS5 检索索引：modernc.org/sqlite v1.62.0 纯 Go 驱动与 DSN 参数、旧版分词器（CJK 二元组、词内符号、路径分词）移植与预切词入列、FTS5 表结构建议（分列 bm25 权重 12/8/6/4/1）、匹配理由与片段高亮、过滤与上限、增量与批量事务、链接边增量重算、每库一文件放应用数据目录、测试与性能基准。只要在写或改 internal/index、检索、链接图、健康检查、索引持久化，都应加载本 skill。
---

# kb-index-sqlite（Mile Knowledge Base 版）

实现 PRD FR-SRCH / FR-LINK / FR-HLTH 的派生索引。业务规则以 `docs/PRD.md` 为准；路径与文件读写纪律见 `go-service-engineering`。

## 1. 何时加载 / 不加载

- 加载：写 `internal/index/`；用 modernc.org/sqlite 建库/迁移/写事务；实现检索、分词、链接图、健康检查；索引性能与基准。
- 不加载：路径授权与 `.mile-kb/` 格式（`go-service-engineering`）；Markdown 渲染（`markdown-render-pipeline`）；mygo 绑定层（`mygo-web-app`）。

## 2. 驱动核对结论（本机实测 2026-11-18）

模块缓存 `C:\Users\LHmile\go\pkg\mod\modernc.org\sqlite@v1.62.0` 已核对：

- **驱动名** `sqlite`：`sql.Register(driverName, ...)`，`sqlite.go:50,56`；`import _ "modernc.org/sqlite"` 后 `sql.Open("sqlite", dsn)`。
- **FTS5 内置**：`lib/sqlite.go:3496` `const SQLITE_ENABLE_FTS5 = 1`；`defensive_test.go:409` 有 `CREATE VIRTUAL TABLE ft USING fts5(body)` 实测。
- **DSN 参数**（URL query 形态，`sqlite.go:336-520`）：
  - `_pragma=<stmt>` 可重复，逐条执行 `pragma <stmt>`（如 `_pragma=journal_mode(WAL)` 需走 shorthand 或 `_pragma=busy_timeout(5000)`）；
  - shorthand：`_busy_timeout` / `_timeout`（毫秒）、`_journal` / `_journal_mode`、`_foreign_keys` / `_fk`、`_synchronous` / `_sync`、`_auto_vacuum` / `_vacuum`、`_query_only`；
  - `_pragma` 值必须是单语句（StrictPragmas 开启时多语句直接报错，`sqlite.go:336-343`）；
  - 应用顺序：busy_timeout → auto_vacuum → `_pragma` 列表（busy_timeout 排序提前）→ foreign_keys → journal_mode → synchronous → query_only（`sqlite.go:452-525`）。

推荐 DSN（索引库是派生数据，可牺牲一点持久性换速度）：

```text
file:<abs-path>?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)
```

## 3. 分词（移植旧版 `src/shared/tokenize.ts`）

**为什么必须预切词（R-2 实测）**：FTS5 内置 `unicode61` 对 CJK 的行为是**整段连续 CJK 当一个 token**（`知识库管理` → 1 个 token，不拆字也不拆词，`fts5vocab` 实测 `term="知识库管理"`）。所以 raw 模式下中文二字词完全无法命中；trigram 按 3 字符切、`知识` 是 2 字符不满足最小长度也不命中。**结论：必须先在 Go 侧切成二元组再写入**（技术设计 2.3）。

做法：**Go 侧把文本切成 token 后用空格拼接写入 FTS5 列**，FTS5 用默认 `unicode61` tokenize 即可（token 已是空格分隔、且内部已无 `unicode61` 的分隔符，被当原子单位不再拆）；查询侧用同一函数切词后按空格拼成 FTS5 查询串。这样召回口径与旧版一致，且 bm25 词频语义保留。R-2 实测：预切词 + unicode61 在大库上检索 p95=10.6ms（PRD 目标 ≤200ms，余量 18×）。

移植规则（tokenize.ts:33-91，逐一对应）：

1. 输入截断到 200 000 字符。
2. **CJK 段**：连续 CJK（统一表意 + 扩展 A + 兼容表意 + 假名，正则范围见 tokenize.ts:24）切成二元组；单字保留单字；**不跨段**（标点断段）。「知识库管理」→ `知识 识库 库管 管理`。
3. **英文数字段**：`[A-Za-z0-9_]` 为词字符；`.`、`-`、`+`、`#` 只在后面紧跟词字符时并入词内（`v4.1`、`c#` 不拆；句末 `end.` 不带点）。转小写。
4. 去重（`Set`，保留首次出现顺序）。
5. **路径分词**（旧版 `main/services/tokenize.ts:tokenizePath`）：路径按 `/` 分段，每段产出三种 token——整段小写、分词结果、再按 `.` `-` `_` 拆分的结果（因为 `.` 是词内字符，不加这层 `huge` 搜不到 `huge.md`）。
6. 查询用同一 `tokenize`，但**查询 token 不去重无妨**；拼 FTS5 查询时空格连接（AND 语义）或按 PRD 召回规则选 OR——PRD 要求「任一词命中即召回」（FR-SRCH 旧版行为参考），即用 `OR` 连接。200 字符查询上限在切词前生效。
7. **单字 CJK 保留单字 token**，写入 FTS5 后能被 MATCH 命中（R-2 §7 实测确认；样本中 `中`/`库` 0 hits 是因为它们从未独立成字段，都在长 CJK 段里被二元组吸收，符合预期）。

## 4. 表结构（建议，阶段 08 定稿）

索引库放应用数据目录：`<userData>/index/<vaultId>.db`，每库一文件，可删除重建。

```sql
-- 文档表：一行一篇文档
CREATE TABLE IF NOT EXISTS documents (
  path        TEXT PRIMARY KEY,      -- canonical vault 相对路径
  title       TEXT NOT NULL,
  area        TEXT NOT NULL,         -- skills/raws/.../system/other
  tags        TEXT NOT NULL DEFAULT '',  -- JSON 数组或空格分隔
  extension   TEXT NOT NULL,
  mtime_ms    INTEGER NOT NULL,
  size        INTEGER NOT NULL,
  ignored     INTEGER NOT NULL DEFAULT 0,  -- 系统区
  body_indexed INTEGER NOT NULL DEFAULT 1, -- 大文件/非UTF-8 只索引路径
  updated_at  TEXT NOT NULL
);

-- FTS5 虚表：不挂 content（R-2 §10.1）。五列都存预切词的 token 串。
-- 不能用 content='documents' 外部分表：预切词列值（token 串）≠ documents 列值（原文），
-- external content 要求虚表列值与主表列值一致。
CREATE VIRTUAL TABLE IF NOT EXISTS doc_fts USING fts5(
  title,        -- 标题（frontmatter title / 首个标题行 / 文件名）
  tags,         -- 标签
  headings,     -- 各标题行
  path,         -- 路径 token（§3.5 三形态）
  body,         -- 正文 token（空格拼接）
  tokenize='unicode61'
);

-- 链接边表（已解析与未解析都记，candidates 语义与旧版一致）
CREATE TABLE IF NOT EXISTS links (
  from_path   TEXT NOT NULL,
  target      TEXT NOT NULL,         -- 原始链接文本
  kind        TEXT NOT NULL,         -- wiki | internal | external | other
  to_path     TEXT,                  -- 解析到的路径；未解析为 NULL
  candidates  INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (from_path, target)
);
CREATE INDEX IF NOT EXISTS links_to ON links(to_path);

CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
-- meta: schema_version, index_revision, last_built_at
```

- **bm25 列权重沿用旧版比例** `12/8/6/4/1`（PRD FR-SRCH）：`bm25(doc_fts, 12.0, 8.0, 6.0, 4.0, 1.0)` 对应 title/tags/headings/path/body 列顺序，R-2 §10.2 实测排序正常可用。
- **匹配理由**（`matchedFields`，命中哪些字段）：FTS5 的 `offsets()`/`snippet()`/`highlight()` 辅助函数**未编译进 modernc.org/sqlite**（`no such function` 实测，R-2 §5），不能用。**推荐实现**：检出 rowid 后一次 `SELECT rowid, title, tags, headings, path, body FROM doc_fts WHERE rowid IN (?)` 拿回五列预切词串，Go 侧对每个查询 token 做 contains（空格包夹匹配）。R-2 实测 50 行结果页 ~1ms（UNION column-filter 22ms、逐行 5 次 count 25ms 都太慢）。注意 `unicode61` 会把含 `-` `.` `+` `#` 的 token 再拆成子 token（`doc-00042` → `doc` `00042`），matched 计算要按 `unicode61` 口径再拆一次。命中列名映射到 PRD 的 title/heading/tag/path/body 文案，不许为空命中。
- **片段**：首个命中位置前后各 60 字符，取不到为空串不伪造（旧版 `main/services/tokenize.ts:buildExcerpt:162-180`，先归一空白再加 `…` 前后缀）。`snippet()` 不可用，片段在 Go 侧 `buildExcerpt` 算。R-2 §9 建议阶段 08 存正文副本（约 +30MB/10k 篇）：否则 excerpt 需按 path 读文件，有 IO 与缓存复杂度；「索引是派生数据可删重建」不违背「文件是唯一事实源」。
- **高亮**：按大小写不敏感的字面匹配（旧版 `shared/search.ts:highlightRanges`）；查询词不编译成正则。
- **过滤**：area、extension、tags、updatedAfter 时间档（全部/1天/7天/30天）、`ignored` 默认排除（`includeSystem` 才纳入）。
- **上限**：查询 ≤200 字符截断；默认 50 条、最多 200 条（`DEFAULT_SEARCH_LIMIT`/`MAX_SEARCH_LIMIT`）。

## 5. 增量与性能（PRD P-1/P-2/P-6/P-7）

1. **首建后台跑**：打开知识库后界面与树先就绪；扫描 + 建库在后台 goroutine，索引进度经事件推给前端；未就绪时查询返回 `partial: true` 并标注「索引未完成」。
2. **单写协程**：所有写库经一个 goroutine + channel 串行；`database/sql` 池 `SetMaxOpenConns(1)`，避免 SQLITE_BUSY。
3. **批量事务**：外部一次改 N 文件时，去抖合并 → 按目录合并受影响集 → 一个 `BEGIN ... COMMIT` 里完成所有 DELETE/INSERT；**绝不逐文件 COMMIT**（旧版每文件整份重写快照，IndexService.ts:835-858）。
4. **增量更新单路径**：`updatePath` 语义同旧版（IndexService.ts:206-244）——按 canonical 记账、NEVER_INDEX（node_modules/.git/.mile-kb）剔除、删除时移除行与链接边。
5. **链接只重算受影响边**（PRD FR-LINK 新增约束，P-1）：旧版每次变更 `reprojectAllLinks` 全量重投影（IndexService.ts:710-740）。新版：变更文件的出边删除重插；指向被删/被改标题文档的入边按 links 表 `to_path`/`target` 反查受影响源文档，只对这些文档重跑 `resolveLink`，不全库重投影。
6. **排除目录**：`node_modules`、`.git`、`.mile-kb` 永不进索引（按 canonical 第一段判定，防短名绕过，IndexService.ts:44-51）；系统区默认不进结果但可 `includeSystem`。
7. **只索引文本扩展名正文**：md/markdown/txt/json/yaml/yml/toml/ts/tsx/js/jsx/py/go/java/rs/sh/css/html（`shared/search.ts:TEXT_EXTENSIONS`）；非 UTF-8 或 >2 MiB 只索引路径（`body_indexed=0`）。
8. **索引库可删**：meta.schema_version 不符 → 整库删除重建；旧版 `index-<vaultId>.json` 不读不迁移。

## 6. 链接解析（旧版 `shared/links.ts`，索引侧落地）

- 抽取：`extractLinks`（links.ts:82-115）跳过代码围栏与行内 code；`[[target|alias]]` 取 `|` 左侧，`[text](target)` 排除 `!` 前缀图片与空 target。
- 解析 `resolveLink`（links.ts:188-221）三层顺序，每层须**唯一命中**才算成功：
  1. 精确相对路径（可省扩展名）；
  2. 文件名（去扩展名）；
  3. 标题（trim + 小写）。
  多候选报 `candidates=N`（>1），无候选报 0；不自动挑。
- 索引侧：文档表先全量落库（链接解析需要全库候选）→ 再从 links 表 + documents 算边。候选集 = `SELECT path, title FROM documents WHERE ignored=0`；解析结果写回 `links.to_path/candidates`。
- **resolveLink 必须建哈希索引再解析，不能逐链接线性扫全库**（主调度 2026-11-18 确认）：spike 里全量解析 23.8s 是因为 resolveLink 对每条链接线性扫描全部文档——这不是基线。正式实现先从 documents 建三个哈希表：① 规范化路径（含去扩展名）→ 文档；② 小写文件名去扩展名 → 文档列表；③ 小写去空白标题 → 文档列表。之后 resolveLink 三层查找全部是 O(1) 哈希命中。全量解析放首建后台 goroutine，增量用 links 表反查受影响边（R-2 §10.5）。
- 反向链接 = `links.to_path = ? AND from_path != ?`（自引用不计），只含已解析边。

## 7. 健康检查（FR-HLTH，旧版 `IndexService.healthReport`）

4 类问题全部从索引库派生，不新增持久化；按 `meta.index_revision` 缓存报告，索引动则失效：

- 未解析链接：links 表 `to_path IS NULL`（kind IN wiki/internal），带 candidates 原因；
- 孤立文档：无入链且无出链，排除 README、SKILL.md、ignored、非文本、body_indexed=0；
- 同名不同目录：按文件名分组计数 >1；
- 缺标题：title 回退成文件名（=basename 去扩展名）。
每类最多 200 条（`KB_HEALTH_MAX_ITEMS`）；索引未就绪标 `partial`。

## 8. 测试与基准（PRD 10.1、技术设计 R-2）

1. **样本**：`t.TempDir()` 造「大」库——10 000 篇 md + 约 5000 文件的代码目录；「典型」库用真实知识库。
2. **分词对拍**：旧版 `tests/unit/tokenize.test.ts` 移植成 Go table test；CJK 二元组、词内 `.` `-` `+` `#`、20 万截断、路径三形态逐条对齐。
3. **链接对拍**：`links.test.ts`、`linkGraph.test.ts`；`indexService.test.ts`（23K，最完整行为规格）按用例逐项对应。
4. **性能基准**（`testing.B`）：
   - 检索 p95：大库上 p95 ≤ 200ms，典型 ≤ 100ms；用 `sort + p95` 统计 N 次查询；
   - 外部改 1000 文件：去抖合并 + 单事务写入，总时长与期间无阻塞写锁；
   - 首建索引：大库后台建库时长与内存。
5. **重建演练**：删库文件 → 重启 → 后台重建 → 结果与重建前一致（文档数、抽样命中）。
6. 模糊测试：`tokenize` 与 `extractLinks` 用 `go test -fuzz` 喂随机字节。

## 9. 交付前自检

- [ ] DSN 带 busy_timeout + WAL；写库走单写协程，变更合并成一个事务
- [ ] CJK 二元组不跨段；词内符号规则与旧版一致；路径三形态 token 齐全
- [ ] FTS5 列顺序与 bm25 权重 12/8/6/4/1 对齐；能给出 matchedFields
- [ ] 片段 ±60 字符、取不到为空；高亮字面匹配；查询 ≤200 字符
- [ ] 链接只重算受影响边；NEVER_INDEX 按 canonical 判定
- [ ] 索引库在应用数据目录、每库一文件、可删除重建
- [ ] 未就绪返回 partial；大库检索 p95 ≤ 200ms 有基准证据
