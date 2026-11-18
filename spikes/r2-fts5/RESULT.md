# R-2 SQLite FTS5 中文检索实测结果

> 阶段 07 技术风险实测 · 2026-11-18
> 代码：`spikes/r2-fts5/`（独立 go.mod，modernc.org/sqlite v1.62.0）
> 依据：PRD FR-SRCH（v1.1）、FR-LINK、FR-HLTH、10.1；技术设计 2.3、7（R-2）
> 验证对象：`.claude/skills/kb-index-sqlite/` 的表结构与分词建议

## 1. 环境

| 项 | 值 |
| --- | --- |
| OS | Windows 11 Pro for Workstations |
| Go | go1.27.3 windows/amd64 |
| SQLite | modernc.org/sqlite v1.62.0（纯 Go，FTS5 内置） |
| FTS5 分词器 | unicode61（内置）、trigram（备选）、porter |
| 样本 | 10 000 篇中英混合 md，正文 0.5–20 KB，含 frontmatter title/tags、标题行、wikilink |

## 2. 关键结论

| 指标 | PRD 10.1 目标 | 实测 | 判定 |
| --- | --- | --- | --- |
| 检索 p95（大库） | ≤ 200ms | **10.6ms** | 达标，余量 18× |
| 中文二字词命中 | 能命中 | 命中，matched=title+headings+body | 达标 |
| 匹配理由（matchedFields） | 能给出 | 能给出，一次 SQL + Go 侧匹配 | 达标 |
| 片段 ±60 字符 | 取不到为空 | buildExcerpt 正确，p95 ≈ 0 | 达标 |
| 批量变更 1000 文件 | 不阻塞界面 | 单事务 204ms，期间查询 p95=6.3ms | 达标 |
| 首建 10k | 后台可接受 | 1.33s，17.7MB | 达标 |
| 链接全量解析 | — | 23.8s（18654 边，Go 侧 resolveLink） | 可接受（一次性） |
| 链接增量重算 | 只重算受影响边 | 5.2ms（1 篇标题变更，1 条受影响边） | 达标 |

**总体判定：达标。** 预切词 + unicode61 + bm25 列权重 12/8/6/4/1 的方案在大库上远超 PRD 指标。

## 3. Tokenizer 对比（3000 篇）

| 方案 | 建库 | 库大小 | "知识" hits | "检索" hits | "v4.1" hits | 判定 |
| --- | --- | --- | --- | --- | --- | --- |
| **预切词 + unicode61** | 384ms | 5.4MB | 934 | 1290 | 470 | **推荐** |
| raw + unicode61 | 304ms | 4.0MB | **0** | 476 | 470 | CJK 不可用 |
| raw + trigram | 460ms | 9.1MB | **0** | 0 | 470 | CJK 不可用 |

**unicode61 对 CJK 的行为**：整段连续 CJK 当一个 token（`知识库管理` → 1 个 token），不拆字也不拆词。验证：`fts5vocab` 显示 `term="知识库管理"`。所以 raw 模式下中文二字词完全无法命中。

**trigram 对 CJK 的行为**：trigram 按 3 字符切，`知识` 是 2 字符不满足最小长度，所以也不命中。trigram 适合子串匹配但中文 2 字词是常见查询，不可用。

**预切词方案的正确性**：Go 侧把文本切成旧版 tokenize 的 token（CJK 二元组、英文词内符号），用空格拼接写入 FTS5 列；unicode61 把每个空格分隔的 token 当原子单位，不会进一步拆分（因为 token 内已无 unicode61 的分隔符）。

## 4. 表结构建议（验证后）

skill 建议的表结构基本可行，但 **content='documents' 外部分表模式不可用**——因为预切词的列值（token 串）与 documents 表的列（原文）不一致，external content 要求虚表列值与主表列值相同。正确做法：**不挂 content，FTS5 列存预切词 token 串，documents 表存元数据**。

```sql
CREATE TABLE documents (
  path TEXT PRIMARY KEY, title TEXT NOT NULL, area TEXT NOT NULL,
  tags TEXT NOT NULL DEFAULT '', extension TEXT NOT NULL,
  mtime_ms INTEGER NOT NULL, size INTEGER NOT NULL,
  ignored INTEGER NOT NULL DEFAULT 0, body_indexed INTEGER NOT NULL DEFAULT 1,
  updated_at TEXT NOT NULL
);
CREATE VIRTUAL TABLE doc_fts USING fts5(
  title, tags, headings, path, body,
  tokenize='unicode61'   -- 五列都是预切词 token 串
);
-- links 表不变
```

## 5. 匹配理由的正确实现（关键发现）

FTS5 的 `offsets()`/`snippet()`/`highlight()` 辅助函数**未编译进 modernc.org/sqlite**（`no such function` 实测）。不能用这些函数。

三种 matched-fields 实现实测耗时（50 行结果页）：

| 方案 | 耗时 | 判定 |
| --- | --- | --- |
| UNION + column-filter（`title : "知识"`） | 22ms | 太慢 |
| 逐行 5 次 count（50×5=250 次查询） | 25ms | 太慢 |
| **SELECT 5 列 + Go 侧 contains** | **1ms** | **推荐** |

**推荐实现**：一次 `SELECT rowid, title, tags, headings, path, body FROM doc_fts WHERE rowid IN (?)` 拿 5 列预切词串，Go 侧对每个查询 token 做空格包夹 contains。注意：unicode61 会把含 `-` `.` `+` `#` 的 Go token 再拆成子 token（`doc-00042` → `doc` `00042`），所以 matched 计算也要按 unicode61 口径再拆一次。

## 6. 分词移植验证

`tokenize.go` 移植了旧版 `tokenize.ts` 的全部规则，`tokenize_test.go` 移植了 `tokenize.test.ts` 的 11 个用例 + `searchHighlight.test.ts` 的 6 个用例 + `parseDocument` 的 10 个用例 + 路径三形态 1 个 = **28 个测试全部通过**。

**与旧版的唯一差异**：JS 按 UTF-16 code unit 截断/切片，Go 按 rune。BMP 内中文（U+4E00–U+9FFF）一致；超出 BMP 的 emoji/罕见字会有差异，知识库场景影响可忽略。

## 7. 单字 CJK 行为确认

单字 CJK（`中`）在旧版 tokenize 里产出单字 token，写入 FTS5 后能被 MATCH 命中。样本中 `中`/`库` 0 hits 是因为这两个字符从未作为独立字段出现（都在长 CJK 段里被二元组吸收），符合预期，不是 bug。

## 8. 链接解析

- 全量解析 18654 条边（10k 篇，~30% 带 wikilink）：Go 侧 resolveLink 耗时 23.8s，写回 234ms。
- 增量：改 1 篇标题 → 找受影响边（links.target ∈ {旧标题, 旧文件名, 新标题}）→ 只重算 1 条边 = 5.2ms。
- **建议**：全量解析放首建后台 goroutine；增量用 links 表反查。

## 9. 正文副本问题（PRD 待决）

本 spike 中 body 存内存 map 代替磁盘读。**建议阶段 08 存正文副本**：理由：
1. 预切词的 body 列已经存了 token 串（17.7MB for 10k 篇），再存原文副本增加 ~30MB 可接受；
2. 不存副本则 excerpt 需按 path 读文件，有 IO 开销和缓存复杂度；
3. PRD 明确「索引是派生数据，可以删除重建」，存副本不违背「文件是唯一事实源」。

## 10. 对 kb-index-sqlite skill 的修正清单

1. **§4 表结构**：`content='documents'` 外部分表模式不可用（预切词列值 ≠ 主表列值）。应改为「不挂 content，FTS5 列存预切词 token 串」。references/schema-and-queries.md 第 74 行的「两种都可行」应明确推荐不挂 content。
2. **§4 bm25 权重**：`bm25(doc_fts, 12.0, 8.0, 6.0, 4.0, 1.0)` 实测可用，排序正常。
3. **§4 匹配理由**：FTS5 的 `offsets()`/`snippet()`/`highlight()` 在 modernc 不可用。推荐改为「SELECT 5 列 + Go 侧 contains」方案（1ms），并注明 unicode61 会再拆含 `-` `.` `+` `#` 的 token。
4. **§3 分词**：补充「单字 CJK 保留单字 token，写入 FTS5 后能被 MATCH 命中」（实测确认）。
5. **§6 链接解析**：全量解析 18654 边耗时 23.8s，建议明确「首建全量解析放后台 goroutine，不影响界面」。
6. **新增**：unicode61 对 CJK 的行为（整段当 1 token，不拆字）——这是选预切词方案的直接依据，值得写进 skill 备查。

## 11. 剩余风险

| 风险 | 影响 | 缓解 |
| --- | --- | --- |
| UTF-16 vs rune 截断差异 | 超出 BMP 的字符（emoji、罕见 CJK 扩展）截断点不同 | 知识库场景影响极小，可接受 |
| trigram 备选不可用 | 如果预切词方案未来需换，无原生 fallback | 预切词已验证，不需要 fallback |
| 大文件 body_indexed=0 | >2MiB 只索引路径，excerpt 取不到 | PRD 已规定「只索引路径」，excerpt 留空符合预期 |
| links 全量解析 23.8s | 首次建库时间较长 | 后台 goroutine，进度事件推送，不阻塞 |

## 12. 复现命令

```bash
cd spikes/r2-fts5
go mod tidy
go test -v          # 28 个分词/解析/高亮测试
go build .
./r2fts5 bench-tokenizers   # tokenizer 对比
./r2fts5 build              # 10k 建库
./r2fts5 query              # 37 条查询 p50/p95/p99
./r2fts5 excerpt            # 片段+高亮耗时
./r2fts5 incr               # 1000 篇增量 + 并发查询
./r2fts5 links              # 链接全量+增量
./r2fts5 probe              # unicode61 CJK 切分探测
./r2fts5 offsets            # matched-fields 策略对比
```
