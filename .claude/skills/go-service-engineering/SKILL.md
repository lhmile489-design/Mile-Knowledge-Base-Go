---
name: go-service-engineering
description: Mile Knowledge Base（Go 重写版）的 Go 本地后端工程法：internal 分包与依赖方向、ctx 取消贯穿、Windows 路径两道校验（拼写层 + 真实路径层，防 8.3 短名与目录联接）、文本文件 UTF-8/BOM/换行还原与原子写入、.mile-kb/ 格式与旧版字节级兼容、应用数据原子写入与 .corrupt 备份、旧版设置一次性导入、IO 并发上限与批量写入、日志错误脱敏、go test/-race/-fuzz 与旧版单测对拍。只要在写或改本项目 internal/ 下的 Go 代码、处理文件路径或读写用户文件、写设置或应用数据，都应加载本 skill。
---

# go-service-engineering（Mile Knowledge Base 版）

业务规则以 `docs/PRD.md` 为准，旧版行为以 `E:\WebApp\Mile-Project\Mile-Knowledge-Base-master` 源码为准（引用时附 file:line）。本 skill 只管 Go 后端工程纪律，不发明 PRD 没有的需求。

## 1. 何时加载 / 不加载

- 加载：写 `internal/` 下任何 Go 包；读写知识库或用户文件；做路径授权；写 `.mile-kb/` 内任何文件；写应用数据（设置、操作日志、对话历史）；做旧版设置导入；写 Go 测试。
- 不加载：mygo 窗口、绑定、Channel、打包（用 `mygo-web-app` / `mygo-app-scaffold`）；SQLite FTS5 索引的表结构与分词落法（用 `kb-index-sqlite`）；Markdown 渲染管线（用 `markdown-render-pipeline`）；前端代码（`react-best-practices`）。

## 2. 目录与依赖方向（建议，阶段 07 定稿）

对应技术设计第 3 节。以下为建议草案，最终以 docs/技术设计.md 定稿为准：

```text
internal/
├── contract/   前端可见类型：请求、结果、事件载荷、错误码。不依赖其他业务包
├── pathguard/  路径规范化、真实路径解析、授权判断、canonical 判定。纯函数，不 import mygo
├── vault/      知识库登记、脚手架、树、文本读写（编码、换行、原子写入、revision）
├── index/      扫描、分词、SQLite FTS5、链接图、健康检查（细则见 kb-index-sqlite）
├── render/     goldmark → HTML、wikilink、bluemonday 清理、增量渲染
├── mutation/   变更计划、事务、操作日志、恢复、provenance
├── engine/     AI 引擎、Agent、工具循环、权限门、路由、steering
├── appdata/    应用设置、最近记录、对话历史、旧版导入。只写应用数据目录
└── service/    mygo 绑定层：装配依赖、校验入参、转换错误、定向推送
```

依赖方向：`service -> vault/index/mutation/engine/appdata -> pathguard -> contract`。下层不反向引用上层；只有 `service` 和 `main` import mygo，使 pathguard/vault/mutation 全部能用 `go test` 覆盖。

## 3. ctx 与取消

1. 绑定方法第一个参数是 `context.Context`；扫描、索引、引擎请求都从这个 ctx 派生，**不用 `context.Background()`**。
2. 打开知识库后的后台任务（监听、首建索引）挂在该知识库的 ctx 上：库被移除或应用退出时取消。
3. 批量处理每批（如每 100 文件）检查一次 `ctx.Err()`；文件 IO 分段读，段间检查。
4. 引擎流式请求的取消必须一路传到 `http.Request`（`http.NewRequestWithContext`）；PRD FR-AI-ENG 要求空闲 300 秒中止。

## 4. 路径授权（PRD 10.2，旧版 `src/main/security/pathContainment.ts`）

两道判定缺一不可：**先按路径拼写判断，再按真实路径判断**。所有路径（用户、Agent、前端传入）都走 pathguard 唯一入口。

### 4.1 拼写层（旧版 `normalizeRelativePath`，pathContainment.ts:88-135）

纯语法层，不碰文件系统：

- 非字符串 / 含 NUL / 长度 > 1024 / 空串（除非 allowRoot）→ 拒绝。
- `//`、`\\` 开头 → unc；`/`、`\`、盘符 `X:` 开头 → absolute。
- 分段（`/` 或 `\`）后逐段校验：`.` / `..` → dot-segment；含 `<>:"|?*` 或 ≤0x1F 控制字符 → reserved-name；首尾是点或空格 → reserved-name；基名是 DOS 保留名（con/prn/aux/nul/com1-9/lpt1-9，按扩展名前判断）→ reserved-name。
- 通过后拼接为 `/` 分隔的相对路径返回。

### 4.2 真实路径层（旧版 `resolveWithinRoot`，pathContainment.ts:148-189）

- `root` 必须是调用方已 realpath 的 canonical 路径。
- 从目标路径起，向上退到最深的现存祖先做 realpath，确认 `isContained(root, real)`；不存在的尾段拼回去后再算一次 `canonical`（realpath 反推的相对路径）。
- Go 侧用 `filepath.EvalSymlinks` + `filepath.Rel` 实现；`filepath.Rel` 结果不以 `..` 开头且非绝对路径才算在内（前缀判断会把 `C:\docs2` 当成 `C:\docs` 子目录）。
- **canonical 是权限判定依据**：8.3 短名（`MILE-K~1` 即 `.mile-kb`）、指回库内的 junction、大小写变体都会被拼写层漏过，只有 canonical 能挡（旧版注释 pathContainment.ts:41-49，已实测复现）。系统区禁写、`.mile-kb` 判断一律用 canonical。
- Windows 比较用 `strings.EqualFold` 或先 ToLower，不直接用字符串前缀。

### 4.3 重解析点

Go 1.23 起 `filepath.EvalSymlinks` 不再解析 Windows 挂载点（目录联接），`os.Lstat` 报 `ModeIrregular` 而非 `ModeSymlink`。范围判断时对路径每一级 `os.Lstat`，任一级带 `ModeSymlink` 或 `ModeIrregular` 即拒绝。扫描时同样跳过并计数（`skippedReparse`）。

### 4.4 Agent 侧系统区禁写（PRD FR-AI-PERM，旧版 `src/shared/agentPolicy.ts:25-44`）

全局禁写前缀（按 canonical 判定）：`.mile-kb/config.json`、`.mile-kb/transactions/`、`.mile-kb/index/`、`.mile-kb/provenance.json`、`.mile-kb/settings/`、`.mile-kb/drafts/`，以及所有点号目录。任何执行模式（含全自动）都拒绝。

## 5. 文本文件读写（PRD FR-DOC，旧版 `src/main/services/textFile.ts`）

三条不变量，来自旧版 textFile.ts 头注释：buffer 内一律 LF；BOM 与换行风格读取时记录、写回时还原；写入是 tmp + rename 原子操作。

1. **读取**：整文件读入后做 UTF-8 往返校验——旧版用「`Buffer.from(raw,'utf8').equals(buffer)`」挡住 UTF-16 与二进制（textFile.ts:67-70）；Go 侧 `utf8.Valid` 即可。不猜编码，不用替换字符顶替，报「不支持的编码」。
2. **BOM 与换行**：开头 BOM 记录后剥掉；主导换行按多数判定（CRLF 与 LF 计数取多者，textFile.ts:36-44），不是遇到第一个就下结论。
3. **写回**：先把 LF 正文按记录的 EOL 展开（先归一再展开，避免重复插 `\r`），有 BOM 则补回；同目录写临时文件 → 保留原文件权限（旧版 stat mode 后 chmod tmp，textFile.ts:115-116）→ `rename`。
4. **阈值**（PRD FR-DOC）：>1 MiB 不进阅读模式；>2 MiB 只读预览；单次写入上限 2 MiB 拒绝不截断；图片上限 8 MiB。
5. **revision 指纹**：大小 + mtime + SHA-256（磁盘原始字节哈希）。保存时指纹不符不写盘，返回冲突。读取返回 `hash/size/mtimeMs`（textFile.ts:23-31）。
6. 只读场景用 `os.Open`（只读），不申请写权限。

## 6. `.mile-kb/` 格式兼容（D-003，PRD 10.3）

知识库内 `.mile-kb/` 下的文件必须与旧版**字节级兼容**：新旧版可以轮流打开同一个库。每个文件的字段、默认值、序列化顺序见 `references/mile-kb-formats.md`（以旧版源码逐文件核对，不确定处标「未核实」）。要点：

- 所有 JSON 一律 `JSON.stringify(obj, null, 2)` + 结尾 `\n`，UTF-8 无 BOM；Go 侧 `json.MarshalIndent(v, "", "  ")` + `append('\n')`。
- 稳定序列化：provenance.json 条目按 copyPath 排序、对象键固定顺序（旧版 `shared/provenance.ts:serializeProvenance`）。其他文件字段顺序保持与旧版一致，避免 Git 噪声 diff。
- 原子写入：同目录 `.<随机>.tmp` → write → rename；失败清理 tmp，不动原文件。
- 磁盘上的值同样不可信：读取时逐字段 coerce，非法值回退默认或丢弃该条，不让整个文件失效（旧版 AppConfigStore、AuditLedger、ProviderStore 都是这个模式）。
- `.mile-kb/index/`、`config.json` 旧版声明但当前代码未见读写（标「未核实」）；新版 SQLite 索引放**应用数据目录**，不进 `.mile-kb/`（PRD v1.1 FR-SRCH）。

## 7. 应用数据（只写应用数据目录）

设置、操作日志、对话历史、索引库、最近文档全部写 `mygo.App.Path(mygo.PathUserData)`，永远不写知识库（对话历史旧版即在 userData，PRD FR-AI-HIST）。

1. 单一写入者 + 写入串行化：包级锁或写链，避免并发 patch 互相覆盖（旧版 ConfigStore.writeChain）。
2. 原子写入同 §6；字段带 `version` 便于以后迁移。
3. **损坏保留**：读到无法解析的 JSON 时，先把原文件改名为 `<file>.<时间>.corrupt`（旧版 `ConfigStore.load`，ConfigStore.ts:133-139），再按默认值运行；不静默删除，不把内容写进日志。
4. **旧版一次性导入（PRD 10.3，R-7）**：首次启动读旧版 userData 的 `settings.json`、`recent-documents.json`，字段映射见 `references/mile-kb-formats.md` §应用数据。只读旧文件，不修改；导入失败按默认值运行并提示。对话历史不导入；旧版 index-*.json 不读。
5. 旧版 userData 推断路径：`%APPDATA%\mile-knowledge-base\`（依据见 references，productName 为 `mile-knowledge-base`，appId `io.mile.knowledgebase`；Electron `app.getPath('userData')` 在 Windows 上为 Roaming 下的 productName 目录）。

## 8. 性能约束（PRD 1.2 根因 P-1 / P-2 / P-7）

1. **IO 并发上限**：目录扫描、批量索引、哈希计算都用 `errgroup` 或固定 worker 池（建议 ≤ runtime.NumCPU），不得无界 goroutine。
2. **批量处理不逐文件持久化**：旧版每个文件变更都整份重写索引快照和事务清单（IndexService.ts:835-858、AuditLedger.ts、persist 串链）。新版：变更按目录合并、写库合并成一个事务、落盘节流（索引写入见 kb-index-sqlite）。
3. **不重复 stat/readdir**：文件树懒加载一次只读一层（PRD FR-TREE）；批量处理一次性收集后批量 stat，不逐个 syscall。
4. 外部一次改 1000 文件时：去抖合并 → 按目录合并受影响项 → 单写协程批量事务，期间界面可操作（PRD 10.1、AC-PERF-1）。

## 9. 日志与错误脱敏（PRD 10.2，旧版 `src/main/security/redact.ts`）

- `log/slog`，不记录正文与完整路径；发版写 `mygo.App.Path(mygo.PathLogs)`。
- 错误出口统一脱敏：vault 根路径 → `<vault>`，用户主目录 → `<home>`（先替换 vault 再替换 home，因 vault 通常在 home 下）。
- `KEY=value` / `token:` 形态中 ≥8 字符的值 → `<redacted>`；`sk-`/`pk-`/`ghp-`/`gho-`/`xoxb-`/`xoxp-` 前缀 key 一律替换。
- 不向前端发送调用栈；错误信息只说类别（越界、不存在、无权限、编码不支持），不带未授权文件内容与完整路径。

## 10. 测试策略

1. `pathguard`、`vault`（文本读写部分）、`mutation`（provenance/风险/命名校验）、`appdata` 全部是纯 Go，`t.TempDir()` 搭样本即可覆盖。
2. **与旧版单测对拍**（最值得移植成 Go 测试的旧版文件，`tests/unit/`）：
   - `pathContainment.test.ts`、`pathAliasBypass.test.ts` —— 两道判定与 8.3 短名绕过；
   - `textFile.test.ts`、`documentWrite.test.ts` —— BOM/EOL 还原、原子写、revision；
   - `tokenize.test.ts` —— 分词二元组、词内符号、截断（与 kb-index-sqlite 共用）；
   - `links.test.ts`、`linkGraph.test.ts` —— 链接抽取与解析；
   - `indexService.test.ts` —— 索引行为规格（23K，最重要）；
   - `mutationService.test.ts`、`transactionService.test.ts`、`auditLedger.test.ts`、`provenance.test.ts`、`provenanceService.test.ts` —— 变更/事务/日志/来源；
   - `configStore.test.ts`、`appConfig.test.ts`、`appConfigStore.test.ts`、`recentDocuments.test.ts` —— 设置与应用数据；
   - `agentPolicy.test.ts`、`executionMode.test.ts` —— 权限门。
3. 路径解析、分词、链接抽取用 `go test -fuzz`：断言「任何输入要么被拒绝，要么结果在授权范围内」。
4. `go test -race` 覆盖并发：监听事件 + 用户操作同时写索引/日志。
5. 取消测试：批量索引中取消 ctx，断言 goroutine 退出、计数归零。
6. 命令：`go test ./...`、`go vet ./...`、`go test -race ./internal/...`。

## 11. 交付前自检

- [ ] 所有读写从调用方 ctx 派生；批量任务分批检查 ctx.Err()
- [ ] 路径全部经过拼写层 + 真实路径层两道判定，权限判断用 canonical，不用字符串前缀
- [ ] UNC、目录联接、8.3 短名、双重编码、`..`、DOS 保留名都有单测
- [ ] 文本读写：UTF-8 严格校验、BOM/EOL 还原、原子写入保留权限、阈值拒绝不截断
- [ ] `.mile-kb/` 写出的字节与旧版格式一致（对照 references/mile-kb-formats.md）
- [ ] 应用数据只写 userData，原子写入，损坏时备份 `.corrupt`
- [ ] 批量变更合并成事务写入，没有逐文件整份持久化
- [ ] 日志与错误里没有正文、完整路径、密钥明文、调用栈
