# M1 实施计划：检索、链接、健康 + 变更、事务与恢复

> 状态：已批准执行（用户 2026-11-18 回复「通过，开始 T00」，批准 B-M1-05、B-M1-06）。按用户指示一次执行一个任务，每完成一个汇报并停下等审查。
> 本次目标：交付技术设计第 9 节能力 C4（检索、链接、健康）与 C5（变更、事务、审计、来源）。闭环为「搜中文关键词并打开结果 → 看反向链接 → 在树里新建、重命名、归档一篇文档 → 在操作日志里恢复」。
> 上一里程碑：docs/PLAN-M0.md（已完成，远端 `main` 为 `39041a0`）。M1 完成后，下一里程碑用 docs/PLAN-M2.md。

## 1. 目标、范围与授权

### 1.1 交付的可观察结果

1. 打开知识库后，索引在后台建立并显示进度，界面不等待；之后外部改动在 1 秒级内进入索引。
2. Ctrl+P 或侧栏「搜索」条打开搜索面板：中文关键词能命中，结果带高亮片段和匹配字段，可排序、可筛选，↑↓ / Enter / Esc 可用。
3. 阅读模式和编辑器里的 wikilink 按「路径 → 文件名 → 标题」三层解析；文档面板底部有反向链接小节。
4. 健康检查能算出 4 类问题，健康面板可用（用户入口随 M3 总览接上，见 1.3）。
5. 树里能新建文件 / 目录、重命名、编辑别名、归档、删除、拖放移动。每个操作都先出变更计划卡，确认后执行，并出报告卡。
6. 破坏性操作开事务，写进 `.mile-kb/transactions/`，格式与旧版字节级兼容；操作日志面板能看到每条记录，已完成的事务可以恢复，外部改过的文件标冲突、不覆盖。
7. 复制到 `projects/` 时记录来源；来源面板显示 7 种状态；「来源改了」时可以一键从来源同步（D-009，新行为）。
8. 粘贴或拖入图片会保存到 `raws/attachments/` 并插入链接，图片字节不走 base64（D-009）。
9. 编辑器里 Ctrl+点击未创建的 wikilink，生成新建计划。

### 1.2 关联需求

FR-SRCH、FR-LINK（标题层、反向链接、未创建链接的新建）、FR-HLTH、FR-MUT、FR-PROV、FR-TREE（变更操作、内联编辑、拖放）、FR-AREA（别名编辑；检索按系统区过滤，「进检索」开关见 1.3）、FR-DOC（图片粘贴与拖入）、NFR-PERF（检索响应、外部改动同步含索引、批量变更不阻塞、搜索 200 条与操作日志 200 条滚动），以及 PRD 10.2 中与写用户文件相关的条目。

### 1.3 明确不做（M1 非目标）

| 不做 | 原因 / 去向 |
| --- | --- |
| AI、对话、Agent 发起的变更（origin 不是 user 的计划） | M2。MutationService 的 origin 校验与 Agent 两道权限判定接口在 M1 预留，Agent 写权限判定在 M2 实现 |
| 总览磁贴，包括「健康」磁贴 | M3。健康面板 M1 做好，用户可点的入口随总览接上；M1 用 e2e 直接打开面板验证（用户 2026-11-18 选择） |
| 侧栏底部「导入技能」「设置」按钮 | 对应功能分别在 M3、M2 交付时再加；M1 只加「操作日志」（用户 2026-11-18 选择） |
| 设置页「分类与检索」一节（M65 写分类），以及「进检索」开关对索引的影响 | M3。旧版索引不读这个开关（`IndexService.ts` 没有任何 categories 引用，`ignored` 只由系统区判定决定，L464），与 PRD FR-AREA「即时生效」不一致。M1 与旧版一致：索引只按系统区过滤，不读 categories.json；M3 做设置页时再定开关怎么生效（用户 2026-11-18 选择，D-009 第 5 条） |
| 技能导入向导（M14、M15） | M3 |
| 旧版 `index-<vaultId>.json`、`audit.json` 的读取或迁移 | 不做。PRD Q2：应用级数据只导入设置和最近记录；索引是派生数据 |
| 令牌文件按 DESIGN.md §3 拆分 | 维持 M0 的决定，留到 M3 |

### 1.4 审批记录

见第 8 节。用户确认前不执行第 4 节的任何任务。

## 2. 前置资料与现状

| 输入 | 版本或实际情况 | 缺口 | 是否阻断 |
| --- | --- | --- | --- |
| PRD / UI 稿 / DESIGN | PRD v1.5、UI设计稿 v1.0、DESIGN v1.0 已审核 | FR-PROV 的同步行为按 D-009 改写（T00） | 否 |
| API.md | v1.0；M19–M35、E02、E03 已有详细定义 | M19 传输方式、§5 图片地址形式按 D-009 改写（T00） | 否 |
| 数据库设计 | v1.0；3.4 provenance、3.5 事务、4.3 audit.jsonl、4.5 索引库、§6、§8、§9 已定稿 | 无 | 否 |
| 技术设计 | v1.0；R-2 已通过（p95 10.6ms，首建 1.33s，增量 1000 篇 204ms）；链接解析须改哈希实现后重测 | 无 | 否 |
| M0 代码 | `internal/{pathguard,settings,vault,render,watch,service,contract}`、前端 stores 与组件；`render.LinkResolver` 已为标题层留口；`service/render.go` 的 `vaultResolver` 只做两层 | 见下方 M0 遗留 | 否 |
| 旧版实现 | 盘点见第 2.1 节 | — | 否 |
| Skills | kb-index-sqlite、go-service-engineering、markdown-render-pipeline、legacy-parity-reference、mygo-web-app、design-tokens、react-best-practices | 实际调用情况仍未验证 | 否 |
| mygo 能力 | `Shell.TrashItem` 在 Windows 用 IFileOperation 实现（`internal/windows/trash.go`）；`Protocol` 处理器能拿到请求方法和请求体（`protocol.go:147-153`） | WebView2 自定义协议是否真的把 POST 请求体交给处理器，未实测（T12 第一步验证） | 否，阻断的是 T12 |
| 依赖 | modernc.org/sqlite v1.62.0（技术设计 2.1 已确认，M0 未引入） | T02 引入 | 否 |

M0 遗留（M1 内处理或继续跟踪）：

| 遗留 | 处理 |
| --- | --- |
| `package.json` 的 `test:e2e` 指向不存在的 `./e2e`；`test:fuzz` 对多个包同时用 `-fuzz`，Go 不允许。README 却标为已验证，这是 M0 的记录错误 | T00 修复并如实更正 README |
| `internal/service/image.go` 与 API.md §5 有 4 处偏差：地址是「路径 + vault 参数」而不是一次性 token；没有 `CallerWindow` 校验；没有 8 MiB 上限；按扩展名而不是文件头判断格式，也没有 `Cache-Control: no-store` | 地址形式改契约跟代码（D-009 第 6 条，T00）；其余 4 项在 T12 补齐 |
| 13 项窗口手动确认（spikes/r4-window/RESULT.md 第 4 节） | 仍等用户回填，不阻断 M1 |
| 旧版性能基线未测（PRD Q7） | T13 测量，用 M1 才有的检索和批量变更场景（M0 路线图 3.5） |
| 读模式单独计时未做 | T13 一并测 |

### 2.1 旧版实现盘点（只读参考，路径相对 `Mile-Knowledge-Base-master/`）

| 能力 | 主进程 / 共享代码 | 旧版测试（用例数） | 界面 |
| --- | --- | --- | --- |
| 分词与解析 | `src/shared/tokenize.ts`（92 行）、`src/main/services/tokenize.ts`（178 行：`tokenizePath`、`parseDocument`、`buildExcerpt`） | tokenize（21） | — |
| 高亮 | `src/shared/search.ts` `highlightRanges` L142 | searchHighlight（9） | `SearchPanel.tsx` `Excerpt` L48 |
| 索引与检索 | `IndexService.ts`（903 行）：`build`、`updatePath`、`search`；权重 L103-109；全词命中 ×2 L795 | indexService（43） | `SearchPanel.tsx`（430）、`stores/searchStore.ts`（204） |
| 链接 | `src/shared/links.ts`（225 行）：`extractLinks` L79、`resolveLink` L188-213；`IndexService.links` L544 | links（27）、linkGraph（12） | `BacklinksPanel.tsx`（105）、`stores/linkStore.ts` |
| 健康 | `IndexService.healthReport` L602；`src/shared/kbHealth.ts` | indexService 中 L459、L481 等 | `HealthPanel.tsx`（239） |
| 变更 | `MutationService.ts`（479 行）；`src/shared/mutation.ts`（130 行：`assessRisk` L125、`DESTRUCTIVE_KINDS` L26） | mutationService（26） | `MutationPlanCard.tsx`（165）、`MutationReportCard.tsx`（117）、`stores/mutationStore.ts`（109） |
| 事务 | `TransactionService.ts`（326 行） | transactionService（14） | — |
| 操作日志 | `AuditLedger.ts`（159 行，旧版是单文件 `audit.json` 整写；新版按数据库设计 4.3 改 JSONL） | auditLedger（12） | `AuditPanel.tsx`（183） |
| 来源 | `ProvenanceService.ts`（148 行）、`src/shared/provenance.ts`（167 行） | provenance（12）、provenanceService（16） | `ProvenancePanel.tsx`（124） |
| 行级差异 | `src/shared/lineDiff.ts`（178 行） | lineDiff（10） | 计划卡的编辑预览 |
| 图片 | `VaultService.saveImage` L391-425、`src/shared/attachments.ts`（55 行） | saveImage（11） | `editor/editorInteractions.ts` `insertImages` L58-76 |
| 树操作 | — | — | `TreeNode.tsx`（306）、`InlineRename.tsx`（98）、`InlineCreate.tsx`（83）、`InlineAliasEdit.tsx`（103）；拖放 MIME `application/x-mile-path` |
| wikilink 新建 | `src/shared/wikiNavigation.ts` `newNotePathFor` L57-73 | — | `editorInteractions.ts` `followWikiLink` L24-56 |

旧版行为要点（实现时以旧版代码为准，这里只列容易做错的）：

- 用户发起（origin=user）的计划不过 Agent 权限矩阵，但所有路径仍做库内约束；用户可以操作 `.mile-kb` 内的文件，Agent 不行（`MutationService.ts:69`、`:419`）。
- 每次尝试的操作都写操作记录，包括失败和跳过的（`MutationService.ts:170`）。
- 拖放没有独立写路径，直接生成 move 计划（`TreeNode.tsx:104-124`）。
- 别名编辑不走变更计划，写 `.mile-kb/settings/aliases.json`（`TreeNode.tsx:78-83`，M66）。
- 执行完成后立即刷新受影响的目录，不等监听（`mutationStore.ts:89-93`）。

## 3. 总路线

M0 已完成。M1 为本计划。M2、M3 的划分见 docs/PLAN-M0.md 第 3 节，仍未审批，M1 完成后再细化。

## 4. 任务清单

执行方式按用户 2026-11-18 的要求：**一次只做一个任务，做完汇报并停下，等用户审查后再做下一个**。状态取值：待做、进行中、阻塞、待验证、已验证。

| 编号 | 目标 | 修改范围 | 依赖 | 产物 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- | --- |
| T00 | 契约回写与门禁修复 | docs/{API.md,PRD.md,decisions.md,技术设计.md}、README、package.json、scripts/、internal/pathguard/pathguard_test.go | — | D-009 落到 API.md M19、PRD FR-PROV；`test:e2e`、`test:fuzz` 能真实运行 | 两个门禁命令各做一次「故意失败 → 非零退出，恢复 → 通过」；README 状态与实际一致 | 已验证 |
| T01 | 分词、文档解析、片段与高亮 | `internal/index/{tokenize.go,tokenize_test.go,fixtures_test.go,testdata/tokens.json}`；`src/lib/{search.ts,tokenize.test.ts,search.test.ts,tokenParity.test.ts}` | T00 | Go 与 TS 两份分词实现 | 移植 tokenize 21 例、searchHighlight 9 例全部通过；Go 与 TS 对同一份夹具输出的 token 序列逐项相同 | 已验证 |
| T02 | SQLite 索引库与增量更新 | `internal/index/{store.go,build.go,update.go,migrations/001_init.sql}`、`internal/service/index.go`（M25、M26、E02）、`internal/service/service.go`（接监听批次、移除知识库时删库） | T01 | `index/<vaultId>.db`；后台首建；监听批次增量写；状态事件 | V-IDX 全部通过 | 已验证 |
| T03 | 检索与搜索面板 | `internal/index/search.go`、`internal/service/index.go`（M24）、`src/components/SearchPanel.tsx`、`src/stores/searchStore.ts`、侧栏「搜索」条与 Ctrl+P | T02 | M24；搜索面板 | V-SRCH 全部通过；对照截图 26、27、28 | 已验证 |
| T04 | 链接图、三层解析与反向链接 | `internal/index/links.go`、`resolver.go`；`internal/service/render.go`（用索引解析器替换 `vaultResolver`）；`internal/service/index.go`（M27）；`src/components/BacklinksPanel.tsx` | T02 | 哈希三层解析；links 表增量维护；M27；反向链接小节 | V-LINK 全部通过；对照截图 20 | 待做 |
| T05 | 健康检查与健康面板 | `internal/index/health.go`、`internal/service/index.go`（M28）、`src/components/HealthPanel.tsx` | T04 | M28；健康面板（暂无用户入口） | V-HLTH 全部通过；对照截图 33 | 待做 |
| T06 | 变更计划（不落盘） | `internal/mutation/{plan.go,risk.go,validate.go,names.go}`、`internal/service/mutation.go`（M29）、`internal/service/doc.go`（M20–M23） | T00 | 计划生成、命名校验、风险等级 | V-PLAN 全部通过；本任务的任何代码路径都不写磁盘（测试断言） | 待做 |
| T07 | 事务与执行 | `internal/mutation/{transaction.go,execute.go,ops.go}`、`internal/service/mutation.go`（M30）；回收站经 service 注入 `mygo.Shell.TrashItem` | T06 | 8 种操作；破坏性操作开事务；部分失败语义 | V-EXEC、V-TX 全部通过 | 待做 |
| T08 | 操作日志与恢复 | `internal/mutation/{audit.go,restore.go}`、`internal/service/mutation.go`（M31–M33、E03） | T07 | `audit.jsonl`；恢复预检与提交 | V-AUDIT、V-RESTORE 全部通过；**独立审查**（覆盖 T06–T08） | 待做 |
| T09 | 来源追踪与一键同步 | `internal/mutation/provenance.go`、`internal/service/mutation.go`（M34、M35）、`src/components/ProvenancePanel.tsx` | T08 | provenance.json 读写；7 种状态；「从来源同步」 | V-PROV 全部通过；对照截图 21 | 待做 |
| T10 | 计划卡、报告卡、操作日志面板 | `src/stores/mutationStore.ts`、`src/components/{MutationPlanCard,MutationReportCard,AuditPanel}.tsx`、`src/lib/lineDiff.ts`、侧栏底部「操作日志」 | T08 | 前端确认与执行流程 | V-CARD 全部通过；对照截图 29、30、31、32 | 待做 |
| T11 | 树的变更操作 | `src/components/{VaultSidebar,TreeNode,InlineRename,InlineCreate,InlineAliasEdit}.tsx`、`src/stores/treeStore.ts`；`internal/settings`（M66 写 aliases.json）；侧栏顶部新建按钮 | T10 | 悬停操作、内联新建 / 重命名 / 别名、拖放 | V-TREE-M 全部通过；对照截图 14、15、15b | 待做 |
| T12 | 图片粘贴与 wikilink 新建 | `internal/service/image.go`（上传处理器）、`internal/vault/attachments.go`、`src/editor/`、`vite.config.ts`（CSP） | T07、T11 | 协议上传保存图片；Ctrl+点击未创建链接生成新建计划 | V-IMG 全部通过；**独立审查**（覆盖 T09–T12 的写路径与上传端点） | 待做 |
| T13 | 闭环验收、兼容与性能 | `scripts/e2e/ac-m1.sh`、`scripts/e2e/perf.sh`、`scripts/compat/`、本计划第 6 节记录 | T03–T12 | 端到端用例、新旧版往返兼容、性能与旧版基线 | 第 6 节验收全部有实际证据；**整体独立审查** | 待做 |
| T14 | 收尾 | README、CLAUDE.md、本计划 | T13 | 文档同步；推送前确认 | 用户确认后推送 | 待做 |

依赖关系：T01→T02→{T03, T04→T05}；T00→T06→T07→T08→{T09, T10→T11}；T12 依赖 T07 与 T11；T13 依赖全部。按「一次一个」执行时的顺序为 T00、T01、T02、T03、T04、T05、T06、T07、T08、T09、T10、T11、T12、T13、T14。

## 5. 单任务精确交接

### T00 契约回写与门禁修复

- 写 decisions.md **D-009**，记录用户 2026-11-18 的 4 项选择：
  1. 图片字节经 `kbimg` 协议 POST 上传，不走 base64；
  2. 「来源改了」时提供真正的一键同步（新行为，旧版只是打开来源）；
  3. 健康面板 M1 交付，用户入口随 M3 总览；
  4. 侧栏按钮只放已可用的功能；
  5. 「进检索」开关 M1 不读，与旧版一致，M3 再定（PRD FR-AREA 同步标注与旧版的差异）；
  6. 图片协议地址沿用 M0 已实现的 `http://kbimg.localhost/<库内相对路径>?vault=<vaultId>`，不做一次性 token；安全靠每次请求的校验保证（见 T12）。
- API.md：
  - M19 改为「协议端点，不是绑定方法」：`POST http://kbimg.localhost/__upload?vault=<vaultId>`，请求体为图片原始字节，`Content-Type` 不采信；响应 `application/json`，`{path, existed}` 或 `{failure}`；失败码沿用 `too-large`（>8 MiB）、`invalid-input`（魔数不认识、SVG、ICO）、`vault-unavailable`、`path-escape`（附件目录是指向库外的联接点）。
  - 2.2 节表尾「共 73 个绑定方法」改为 72；§3 映射表中 `vault:save-image` 一行由「same」改为「协议端点（§5），不再经绑定方法」；
  - §5：地址形式按 D-009 第 6 条改写，删去「一次性 token」；授权规则 5 条保留，并补上传端点的规则（只接受 POST、`CallerWindow` 非空、请求体上限 8 MiB、按文件头判格式）。
  - M34/M35 旁补「同步」说明：同步不是新方法，前端用 M29 生成一条 `edit` 计划（`source`=副本路径，`text`=来源当前内容，`expectedSourceRevision`=副本当前 revision），经计划卡确认后用 M30 执行；执行成功后 Go 把该记录的 `baseRevision`、`baseHash` 更新为来源当前值，状态回到「一致」。
- PRD：FR-PROV 写明同步走变更计划、可从操作日志恢复、成功后更新基线；FR-AREA 的「即时生效」后注明「旧版索引未实现，本期 M1 不生效，M3 定」；FR-DOC 图片一条去掉「阶段 07 设计」的待定措辞，改为引用 API.md M19。修订记录记 v1.6。
- 门禁修复：
  - `test:e2e` 改为依次运行 `scripts/e2e/` 下的验收脚本（前提是先做 e2e 构建，命令写进 README）；
  - `test:fuzz` 改为对每个含 `Fuzz` 函数的包分别运行（目前是 `internal/pathguard`、`internal/render`），时长沿用 60s；
  - 两条命令各做一次负向对照，结果记入第 9 节；README 中 M0 误标「已验证」的两行按实际情况更正。

### T01 分词与解析

- Go：`internal/index` 包，依赖方向 `index → vault → pathguard`，不 import mygo。
  - `Tokenize(text string) []string`：按 PRD FR-SRCH 和旧版 `shared/tokenize.ts`：中日韩连续文字切二元组并保留单字，不跨标点；英文和数字小写，词内保留 `.-+#`；去重并保持首次出现顺序；输入超过 200 000 字符先截断。
  - `TokenizePath(rel string) []string`：每段出整段、分词结果、按 `.-_` 拆开的结果。
  - `ParseDocument(rel, text string) Parsed`：标题按 frontmatter title → 第一个标题行 → 文件名；tags 支持列表、`[a,b]`、`a,b`；未闭合的 frontmatter 容错（旧版 tokenize.test L121）。
  - `BuildExcerpt(body string, tokens []string) string`：最早命中位置前后各 60 字符，折叠空白，加省略号；找不到返回空串。
- TS：`src/lib/tokenize.ts` 逐行移植旧版 `shared/tokenize.ts`，`src/lib/highlight.ts` 移植 `highlightRanges`（字面 indexOf、大小写不敏感、合并重叠，不编译正则）。理由：高亮要和索引的分词对齐（旧版 searchHighlight L67），而 API.md 的 `SearchHit` 没有高亮区间字段，不改契约。
- 一致性测试：`internal/index/testdata/tokens.json` 的夹具 = 旧版 tokenize 21 例的全部输入，加上以下边界各至少 1 条：中英混排、标点边界、`v4.1`、`c#`、全角字符、emoji 代理对、20 万字符截断边界、空串。Go 测试生成期望输出；vitest 读同一文件，断言 TS 输出逐项相同。两边任何一边改动导致不一致都会失败。

### T02 索引库

- 位置：`App.Path(PathUserData)/index/<vaultId>.db`；DSN 与 schema 按数据库设计 4.5，`001_init.sql` 用 `go:embed`，建 5 张表：`documents`、`doc_fts`、`doc_body`（正文副本，独立表，Q-C 已关闭）、`links`、`meta`。`meta.schema_version` 与代码不符且没有迁移路径时，删库重建。
- 写入只在一个写协程里进行，`SetMaxOpenConns(1)`；读查询走另一个只读连接池（WAL）。
- 首建：打开知识库后后台进行，按批提交（每批 500 篇一个事务），每批推一次 E02 `index-status`（phase=scanning、scanned、skipped）；完成后 phase=ready。期间检索返回 `partial=true`。
- 范围：排除 `node_modules`、`.git`、`.mile-kb`；只有 FR-SRCH 列出的扩展名读正文；非 UTF-8 或超过 2 MiB 只索引路径（`body_indexed=0`）；系统区写入但标 `ignored=1`，系统区按 `vault.AreaFor` 判定（与旧版 `classifyTopLevel` 一致）；不读 categories.json（1.3）。
- 增量：`service.startWatch` 收到一批目录后，先推 E01（保持 M0 行为），再把这批目录交给索引写协程：按目录比对库内记录与磁盘（mtime + size，有变化再读内容），一个 SQLite 事务完成文档、FTS、links、受影响反向边的更新，`meta.index_revision` 加 1。
- 自写回环：MutationService 执行后把 `touchedDirectories` 也交给同一入口，与监听批次合并去重。
- 移除知识库（M10）时关闭并删除 `index/<vaultId>.db`（数据库设计 §7）。
- M25 `status`、M26 `rebuild`：`rebuild` 立即返回当前状态，后台删表重建。
- 测试先行：移植 indexService 中建库、增量、删除、系统区、持久化重启不重扫（L318 的语义改为「重启后不全量重建」）等用例；新增：schema_version 不符时重建；损坏库文件时重建；监听批次 1000 个文件在一个事务内完成。

### T03 检索与搜索面板

- M24 按 API.md 4.3：`text` 超过 200 字符截断；`limit` 1–200 默认 50；过滤与排序按 FR-SRCH；`excerpt` 用 `doc_body`，`matchedFields` 在 Go 侧对命中行回列计算（数据库设计 4.5「代表查询」）。
- 打分：`bm25(doc_fts, 12, 8, 6, 4, 1)`。PRD v1.1 已说明排序不要求与旧版逐条一致；旧版「多词全命中 ×2」用「全部 token 命中的结果排在部分命中之前」实现，测试用旧版 L157「标题命中排在仅正文命中之前」及全命中优先两条断言。
- 前端：SearchPanel 位于文档面板顶部，最大高 420px；输入停止 150ms 后查询（旧版 searchStore 的去抖值在实现时核对并沿用）；结果列表用 react-virtual；底部「N 条 · X ms」；空结果、索引未完成、已选筛选三种状态对照截图 26、27、28。
- 入口：Ctrl+P、侧栏顶部「搜索 Ctrl P」条；Esc 关闭；打开结果后关闭；保留上次查询与筛选（UI设计稿第 2 节）。

### T04 链接图

- 抽取：移植 `shared/links.ts` 的 `extractLinks`：`[[目标]]`、`[[目标|别名]]`、`[文字](目标)`；图片语法不算；代码块和行内代码里的不抽。
- 解析：三张哈希表——规范路径（含去扩展名形式，小写）、小写文件名去扩展名、小写去空白标题；按层查找，某层唯一命中即成功，某层多候选就停并返回候选数（旧版 links.ts:183-213）。三张表在首建后从 documents 表载入内存，随增量批次更新。
- 存储：links 表按 `from_path` 整组替换；目标文档新建、改名、标题变化时，用 `links` 表和三张哈希表找出受影响的引用方，只重算这些行（P-1）。
- 渲染：`service/render.go` 的 `vaultResolver` 改为委托索引解析器，渲染缓存键加上 `index_revision`，标题变化后阅读模式能更新。索引未就绪时退回 M0 的两层解析。
- M27 `links`：outgoing + backlinks；反向链接只算已解析的边，排除自引用和系统区。
- 前端：BacklinksPanel 在文档面板底部，列反向链接和未解析链接及原因（候选 N 个 / 断链），点击打开。对照截图 20。
- 测试：移植 links 27 例、linkGraph 12 例；render golden 中标题层用例从「推迟」改为必须一致。基准：1 万篇样本全量解析耗时与增量 1000 篇耗时，写入第 6 节（技术设计 R-2 要求的重测）。

### T05 健康检查

- 4 类问题按 FR-HLTH；孤立文档排除 README、SKILL.md、系统区、非文本文件、未读正文的文件；每类最多 200 条；结果按 `index_revision` 缓存；索引未完成时 `partial=true`。
- 前端：HealthPanel 4 个标签页，计数 + 列表，点击条目打开文档，Esc 关闭。没有用户入口（1.3），e2e 通过 `window.__mkbApp` 打开面板验证。
- 测试：移植 indexService L459、L481 一组；新增缓存命中与失效测试。

### T06 变更计划

- `internal/mutation` 包，依赖方向 `mutation → vault → pathguard`，不 import mygo；回收站、时间、随机数都从 service 注入，便于测试。
- M29 `plan`：按 API.md 4.4 校验（≤500 项、id ≤128、reason ≤500、路径校验、expectedRevision ≥0）；风险用移植的 `assessRisk` 计算，不采信前端；目标已存在一律拒绝（`name-conflict`），唯一例外是只改大小写的重命名。
- 命名校验按 FR-TREE 全部规则；新建文件没有扩展名补 `.md`；`planMove` 不能拖进自身或子树。
- 归档目标：`_archives/<原相对路径>`，重名时追加 ISO 时间戳（旧版 `doArchive` L341-349 的具体格式在实现时核对并写进测试）。
- M20–M23 包装为对应的 PlanRequest，返回 `PlanResult`，不落盘。
- origin：M1 只接受 `user`。其他 origin 返回 `invalid-input`，M2 再接入 Agent 权限判定（旧版两道判定 `MutationService.ts:79-96`）。
- 测试先行：移植 mutationService 中计划相关用例（L92、L129 等）与 lineDiff 10 例（lineDiff 在 Go 侧也要有一份，供 M2 的写入预览；本任务只移植测试与实现，不接界面）；新增断言：本任务的所有方法执行前后，样本库文件的 SHA-256 与文件列表完全不变。

### T07 事务与执行

- 事务格式按数据库设计 3.5，逐字节对齐旧版：tx id 为 `<毫秒时间 36 进制>-<uuid 前 8 位>`；`payload/<sha256(相对路径)>` 存操作前的原始字节；目录只记标记；原本不存在记 `absent`；manifest 用 tmp + rename；JSON 序列化遵守 3.0.1（`SetEscapeHTML(false)`、键序、末尾换行、保留未知字段）。
- 执行顺序：建事务目录并写 manifest（status=open）→ 逐项先写 pre-image 再改真实文件 → 全部成功后写 after-image 并置 committed；中途失败置 partial，已完成项保留，后续项记 skipped，不自动回滚（FR-MUT）。
- 只对 edit、move、archive、trash、permanentDelete 开事务。`permanentDelete` 在 `freshApproval != true` 时返回 `permission-denied`，不进入执行。
- 复制目录递归复制，复制文件后校验哈希；目标在 `projects/` 下时记录来源（T09 实现记录函数，T07 先留接口并在 T09 接上）。
- 移到回收站：经 service 注入 `mygo.Shell.TrashItem`（Windows 为 IFileOperation）。回收站里的文件不作为恢复来源；事务照常先存 pre-image，恢复从 payload 写回原路径。
- 写文件统一走 `vault` 包的原子写，保留权限与 BOM / 换行（M0 T07b 的规则）；路径全部过 `pathguard.Resolve`。
- 执行完成返回 `touchedDirectories`，交给索引入口（T02）。
- 测试先行：移植 mutationService 执行相关用例（L147 归档、L182 永久删除无批准、L232 中途失败）、transactionService 14 例；新增：回收站注入失败时该项 failed、后续 skipped；事务目录被构造成联接点时拒绝。

### T08 操作日志与恢复

- `audit.jsonl` 按数据库设计 4.3：位于应用数据目录根；每行一条 ActionRecord；启动时读入并按 id 去重；运行期只追加；超过 2000 条时截断重写一次保留最近 2000 条，截断重写同样用 tmp + rename，期间读取以内存 Map 为准；坏行跳过并计数。每次追加后推 E03。
- 每次尝试的操作都写记录，包括失败与跳过；移除知识库不清操作记录。
- M31 按 API.md；M32 `rewindPrepare` 逐条比对 after-image，外部改过的标 `restorable=false`；M33 `rewindCommit` 只恢复可恢复项，冲突项不覆盖，恢复后把对应记录置 `reverted` / `conflicted` 并追加新记录。
- 测试先行：移植 auditLedger 12 例（「整份重写」相关的断言改成 JSONL 语义，逐条说明）；移植 transactionService 中恢复相关用例；新增 PRD AC-MUT-1（归档后外部改归档文件 → 恢复标冲突不覆盖）。
- **独立审查**：新开子代理，输入为 API.md 4.4、数据库设计 3.5 / 4.3 / §6、本计划 T06–T08、完整 diff 与测试输出，不附作者结论；修复与复审最多 3 轮。

### T09 来源追踪与一键同步

- provenance.json 按数据库设计 3.4，稳定序列化逐字节复刻 `serializeProvenance`；只在复制目标以 `projects/` 开头（大小写不敏感）时记录；一次 copy 操作只在目标上记一条，目录复制也只记目录本身这一条，不逐个文件记录（旧版 `MutationService.ts:307-316`、`ProvenanceService.record` L79-93）。
- 7 种状态按旧版 `assessProvenance`（provenance.ts:57-107）运行时计算，不落盘。
- 一键同步（D-009）：
  1. 只有状态为「来源改了」时显示「从来源同步」按钮；
  2. 点击后前端用 M29 生成一条 edit 计划，走计划卡确认；
  3. M30 执行成功后，MutationService 更新该记录的 `baseRevision`、`baseHash`、`copiedAt`；
  4. 同步可以从操作日志恢复。恢复后，副本回到同步前的内容；基线不回退，状态会显示为「副本改了」。这一条是新行为的推论，审批时请确认。
- 前端：ProvenancePanel 显示状态边框、文字标签、基于的版本、打开来源、同步按钮。对照截图 21。
- 测试先行：移植 provenance 12 例、provenanceService 16 例（「Agent 不能写 provenance.json」一例推迟到 M2）；新增同步用例：来源改了 → 同步 → 状态一致；两边都改了 → 没有同步按钮。

### T10 计划卡、报告卡、操作日志面板

- mutationStore：`pending` / `running` 只在前端；`running` 期间禁用确认；执行后立即刷新 `touchedDirectories` 与根目录（旧版 mutationStore L89-93）。
- MutationPlanCard：风险等级、操作清单（最多 12 条）、恢复方式说明；永久删除加警示和「我了解风险」预确认，`freshApproval` 只在这次显式确认后为 true。MutationReportCard：完成、失败、跳过分列。
- AuditPanel：侧栏底部「操作日志」打开，最大高 280px，显示最近 200 条（虚拟化），已完成且有事务的记录带「恢复」按钮；恢复先预检，再确认提交；空状态「还没有记录」。
- 对照截图 29、30、31、32。前端单测：mutationStore 状态流转。

### T11 树的变更操作

- 悬停或聚焦时出现：重命名、别名（仅目录）、归档、删除；所有操作都生成计划；没有原生右键菜单。
- 内联新建、重命名、别名：Enter 提交、Esc 取消；重命名默认选中主干名；命名校验错误就地提示（规则在 Go，前端只显示返回的 failure）。
- 别名（M66）：写 `.mile-kb/settings/aliases.json`，按数据库设计 3.3 与 3.0.1；空值删除该条；上限 2000。字节往返测试：读入含未知键和 HTML 字符的样本后写回，字节一致。
- 拖放：MIME `application/x-mile-path`，拖到目录上生成 move 计划；不能拖进自身或子树。
- 侧栏顶部加新建文件、新建目录按钮与「搜索 Ctrl P」条；底部只加「操作日志」。
- 已打开标签在文件被移动、归档、删除后的表现：开工时先读旧版 `tabsStore` / `documentStore`，把结论写进第 9 节再动手。如果旧版没有专门的跟随逻辑，新版也不做：标签按「源路径被外部删除」处理（重读得到 `not-found` 后提示），不新增行为。
- 对照截图 13、14、15、15b。

### T12 图片粘贴与 wikilink 新建

- 第一步先验证：用 e2e 构建在真实 WebView2 里从页面 `fetch` POST 一个 1 MiB 二进制到 `http://kbimg.localhost/__upload`，处理器读到的字节数与 SHA-256 必须一致。不一致或收不到请求体，就停下向用户报告，不换方案。
- 上传是独立的处理器，路径 `/__upload`，不在读图处理器上加分支。规则：
  - 只接受 POST，其他方法 405；
  - `mygo.CallerWindow(r.Context())` 必须非空，否则 404（API.md §5 规则 2）；
  - 每次请求重新解析并 `Lstat` 附件目录，确认仍在库内。
- 读图处理器同时补齐 M0 的 4 处偏差（第 2 节）：`CallerWindow` 校验、单张 8 MiB 上限（超限 403）、按文件头判格式（SVG / ICO 403）、`Cache-Control: no-store`。补齐过程按 TDD，先写能失败的测试。
- 上传处理：请求体最多读 8 MiB + 1 字节，超过返回 `too-large`；按魔数判 png、jpg、gif、webp、bmp、avif，SVG 与 ICO 拒绝；保存到 `raws/attachments/pasted-<sha256 前 12 位>.<ext>`，同内容已存在时 `existed=true` 不重写；附件目录是指向库外的联接点时拒绝（旧版 saveImage.test L79）。
- 写入只经 vault 包的原子写；保存图片不开事务、不写操作记录（旧版行为；实现时以旧版 `VaultService.saveImage` 核对）。
- CSP：`connect-src` 加 `http://kbimg.localhost`。
- 编辑器：粘贴、拖入图片 → 上传 → 在光标处插入 `![](raws/attachments/...)`。Ctrl+点击 wikilink：已解析就打开；未创建就按 `newNotePathFor` 规则生成新建计划（内容 `# <名字>`），走计划卡。
- 测试先行：移植 saveImage 11 例；新增上传处理器测试：GET 405、无调用窗口 404、超限、伪造扩展名、空体；读图处理器测试：无调用窗口 404、超过 8 MiB 403、扩展名是 .png 但内容是 SVG 时 403、响应头含 `no-store`。
- **独立审查**：覆盖 T09–T12 的写路径（同步、别名、上传端点、wikilink 新建）。

### T13 闭环验收、兼容与性能

- 端到端：`scripts/e2e/ac-m1.sh` 跑第 6 节 AC-M1 全部用例。
- 新旧版往返兼容，第一步先验证可行性：在临时目录写一个入口，按旧版 tsconfig 的 `@shared/*` 别名解析，用 bun 实例化旧版 `TransactionService` 和调用 `serializeProvenance`，不拉起 Electron。
  - 不行的话，退一步：在临时目录建一份 vitest 配置，`root` 与别名指向旧版 `src/`，缓存目录也放在临时目录，用旧版已安装的依赖运行一次性用例。旧版目录里不新增、不修改任何文件。
  - 两条路都不通时停下报告，不改写旧版代码。
  - 字节级：用 bun 直接执行旧版 `src/shared/provenance.ts` 等纯函数和 `TransactionService`，在临时目录生成样本；新版读入再写出，断言字节一致；反向用新版写出、旧版读入，断言字段一致。
  - 事务互通：新版建的事务由旧版 `TransactionService.restore` 恢复，旧版建的事务由新版恢复，两边结果都正确。
- 性能：第 6 节指标在「典型」「大」两套样本上实测；同样样本在旧版上测冷启动、检索响应和外部改 1000 个文件，作为 PRD Q7 的基线（沙箱方式同 design/baseline/capture.cjs，不改旧版文件）。
- **整体独立审查**：M1 全部 diff，输入同 T08。

### T14 收尾

- 更新 README、CLAUDE.md、本计划账本；列出要推送的提交，询问用户后推送。

## 6. 验证与验收

| 编号 | 给定条件 | 操作 | 预期结果 | 方式 |
| --- | --- | --- | --- | --- |
| V-IDX-1 | 「大」样本，首次打开 | 打开知识库 | 树首屏照常 ≤300ms；E02 推送进度；完成后 phase=ready、documentCount 与磁盘一致 | e2e + Go 测试 |
| V-IDX-2 | 已建好索引 | 重启应用 | 不全量重建；文档数一致 | Go 测试 |
| V-IDX-3 | schema_version 不符 / 库文件损坏 | 打开 | 删库后台重建，不崩溃 | Go 测试 |
| V-IDX-4 | 「大」样本 | 外部脚本一次改 1000 个文件，每个文件写入唯一标记词 | 从脚本写完最后一个文件起计时，到 M24 能查到全部 1000 个标记为止，≤1s 为达标、超出如实记录；同期 WebView 主线程没有超过 200ms 的长任务（PerformanceObserver，口径同 M0 perf.sh） | e2e + bench |
| V-IDX-5 | 移除知识库 | M10 | `index/<vaultId>.db` 被删除 | Go 测试 |
| V-SRCH-1 | 正文含「知识库」 | 搜「知识」 | 命中，匹配字段为正文，片段高亮（AC-SRCH-1） | e2e |
| V-SRCH-2 | `.obsidian` 下有匹配文件 | 默认搜索 / 勾选包含系统区 | 默认不出现；勾选后出现并标 ignored（AC-AREA-1） | Go 测试 + e2e |
| V-SRCH-3 | 只在路径里命中 | 搜索 | 片段为空，不伪造 | Go 测试 |
| V-SRCH-4 | 1 万篇样本 / 典型样本 | 索引 ready 后，50 次随机二字中文查询 | 计时为 M24 调用发出到返回（含绑定往返，不含界面渲染）；p95 大 ≤200ms、典型 ≤100ms；界面「X ms」显示的就是 Go 侧 `tookMs` | bench |
| V-LINK-1 | 两个同名 a.md | 点 `[[a]]` | 「（有多个同名）」，不跳转（AC-LINK-1） | e2e |
| V-LINK-2 | 某文档标题为「设计原则」，文件名不同 | `[[设计原则]]` | 按标题层解析成功 | Go 测试 + render golden |
| V-LINK-3 | A 链接 B | 改 B 的标题 / 删除 B / 新建原本缺失的目标 | 只重算受影响行；反向链接与断链状态正确；没有幽灵反向链接 | Go 测试 |
| V-HLTH-1 | 样本含 4 类问题 | M28 | 每类计数与列表正确；索引未完成时 partial=true | Go 测试 + e2e |
| V-PLAN-1 | 同级已有 Readme.md | 新建 readme.md | `name-conflict`（AC-TREE-1） | Go 测试 + e2e |
| V-PLAN-2 | 21 项计划 | 预览 | 风险为高（AC-MUT-2） | Go 测试 |
| V-PLAN-3 | 任意计划请求 | M20–M23、M29 | 磁盘文件与列表前后完全一致 | Go 测试 |
| V-EXEC-1 | 计划第 3 项会失败 | 执行 | 报告 partial；前 2 项完成；后续 skipped；没有自动回滚 | Go 测试 |
| V-EXEC-2 | 含永久删除，freshApproval 不为 true | 执行 | `permission-denied`，文件仍在，没有事务目录 | Go 测试 |
| V-TX-1 | 编辑一个 CRLF + BOM 文件 | 执行后看事务 | payload 与原文件字节完全相同；manifest 字段与键序符合 3.5 | Go 测试 |
| V-AUDIT-1 | 连续追加 100 条后重启 | 读取 | 条数一致；造一条坏行后跳过并计数 | Go 测试 |
| V-RESTORE-1 | 归档后外部又改了归档文件 | 恢复 | 该项冲突，不覆盖（AC-MUT-1） | Go 测试 + e2e |
| V-PROV-1 | 副本和来源都改过 | 来源面板 | 「两边都改了」，没有同步按钮（AC-PROV-1） | Go 测试 + e2e |
| V-PROV-2 | 只有来源改了 | 点「从来源同步」并确认 | 副本内容等于来源；状态变「一致」；操作日志有记录 | e2e |
| V-PROV-3 | provenance.json 记录顺序打乱 | 读入后写回 | 输出与旧版序列化字节一致 | Go 测试 |
| V-CARD-1 | 永久删除计划 | 计划卡 | 有警示和「我了解风险」预确认；未勾选时确认按钮不可用 | e2e |
| V-TREE-M-1 | 目录 | 拖进它自己的子目录 | 不生成计划 | e2e |
| V-TREE-M-2 | 编辑别名 | 保存 | aliases.json 字节符合 3.3 与 3.0.1；侧栏显示别名，悬停显示真实名 | Go 测试 + e2e |
| V-IMG-1 | 页面粘贴一张 PNG 两次 | 上传 | 第一次写入，第二次 `existed=true`；文件名为 sha256 前 12 位 | e2e |
| V-IMG-2 | 上传 SVG / 9 MiB 文件 / GET 请求 | 处理器 | 分别 `invalid-input` / `too-large` / 拒绝 | Go 测试 |
| V-COMPAT-1 | 新版建的事务 | 旧版恢复 | 恢复成功，字节正确 | 兼容脚本 |
| V-COMPAT-2 | 旧版建的事务与 provenance.json | 新版读取、恢复、写回 | 字节一致，恢复正确 | 兼容脚本 |
| AC-M1-1 | 「典型」样本 | 搜中文关键词 → 打开结果 → 看反向链接 → 在树里新建一篇 → 重命名 → 归档 → 操作日志里恢复 → 重启 | 每步与截图和 PRD 一致；重启后 `index/<vaultId>.db` 仍在且 documentCount 不变，`audit.jsonl` 条数不减，恢复过的记录状态为 reverted | e2e |

性能指标（PRD 10.1，G-PERF 仍为警告级，T13 测出旧版基线后按 PRD Q7 校准）：

| 指标 | 目标 | 新版实测 | 旧版基线 |
| --- | --- | --- | --- |
| 检索响应 p95 | 典型 ≤100ms，大 ≤200ms | 未测 | 未测 |
| 外部改动同步（含索引） | ≤1s | 未测 | 旧版承诺 1 秒 |
| 外部批量改 1000 个文件期间主线程 | 无 >200ms 任务 | 未测（M0 只测了树） | 机制性卡顿（P-1） |
| 索引首建（1 万篇） | 后台完成，界面可操作 | 未测（R-2 spike 1.33s） | 未测 |
| 链接全量解析（1 万篇） | 无目标，记录数值 | 未测（spike 线性扫描 23.8s，不作基线） | 未测 |
| 搜索 200 条、操作日志 200 条滚动 | 60fps | 未测 | 未虚拟化 |
| 读模式端到端（M0 遗留） | 无目标，记录数值 | 未测 | 未测 |

## 7. 协作、审查与执行方式

- 执行者：主会话，一次一个任务（第 4 节）。需要大范围只读查找（如旧版行为核对）时可以开只读子代理，结论回到主会话后再动手。
- 共享产物负责人：API.md、数据库设计与 `internal/contract` 由主会话维护；`src/mygo.ts` 只通过 `mygo generate` 生成。
- 写用户文件的模块边界：`.mile-kb/` 下除 `settings/aliases.json`（M66，经 settings 包）外，只经 `internal/mutation` 写；树操作、拖放、wikilink 新建、来源同步都必须走变更计划，不开第二条写入路径。
- 测试先行：Go 的业务逻辑（分词、解析、索引、检索、链接、健康、计划、风险、命名、事务、执行、日志、恢复、来源、上传处理）一律先写失败测试再实现；旧版测试按第 2.1 节清单移植，移植时改写的断言逐条在测试注释里说明原因。
- TDD 例外（需批准）：T00 的文档与配置（用负向对照验证）、纯视觉组件（用 e2e 截图与人工对照验证）、T13 的兼容与性能脚本本身。
- 独立审查：T08 后（T06–T08 变更核心）、T12 后（T09–T12 写路径与上传端点）、T13 整体。审查者是新开的子代理，输入为批准的契约、任务范围、完整 diff 与测试输出，不附作者结论；修复与复审最多 3 轮，仍有阻断问题就暂停并报告。
- 提交：每个已验证的任务提交一次，只暂存本任务文件，按 M0 T00 的提交前检查清单执行；不推送，T14 询问后推送。

## 8. 来源基线与批准记录

| 资料 | 版本 / 日期 | 适用范围 |
| --- | --- | --- |
| docs/PRD.md | v1.5（T00 后 v1.6），2026-11-18 | 第 1.2 节所列功能 |
| DESIGN.md | v1.0 | T03、T04、T05、T09、T10、T11 |
| docs/UI设计稿.md + design/baseline | v1.0 | 截图 13、14、15、15b、20、21、26–33 |
| docs/技术设计.md | v1.0 | 全部 |
| docs/API.md | v1.0（T00 后改 M19 与 §5） | M10（删索引库）、M19–M35、M66；E01–E03 |
| docs/数据库设计.md | v1.0 | 3.0.1、3.2（只读）、3.3、3.4、3.5、4.3、4.5、§6–§9 |
| docs/decisions.md | D-001 至 D-008，T00 新增 D-009 | 全部 |
| 旧版代码 | Mile-Knowledge-Base-master v2.7.0，只读 | 第 2.1 节 |

| 批准标识 | 内容 | 依据和时间 | 状态 |
| --- | --- | --- | --- |
| B-M1-01 | 图片经 `kbimg` 协议 POST 上传，不走 base64 | 用户 2026-11-18 选择 | 已确认 |
| B-M1-02 | 健康面板 M1 交付，用户入口随 M3 总览 | 用户 2026-11-18 选择 | 已确认 |
| B-M1-03 | 侧栏按钮只放已可用的功能 | 用户 2026-11-18 选择 | 已确认 |
| B-M1-04 | 「来源改了」时提供一键同步（新行为） | 用户 2026-11-18 选择 | 已确认 |
| B-M1-07 | 「进检索」开关 M1 不读，与旧版一致，M3 再定 | 用户 2026-11-18 选择 | 已确认 |
| B-M1-08 | 图片协议地址沿用 M0 的「路径 + vault 参数」，API.md §5 改为跟代码；T12 补齐 4 项校验 | 用户 2026-11-18 指示「使用你推荐的」 | 已确认 |
| B-M1-05 | 一键同步被恢复后基线不回退（T09 第 4 点） | 用户 2026-11-18「通过，开始 T00」 | 已确认 |
| B-M1-06 | 本计划第 1.3 节非目标、第 4 节任务与顺序、第 7 节 TDD 例外与审查安排；执行节奏沿用 M0 期间的指示「一次做一个，审查后再继续」，不按 WORKFLOW 的「批准范围内连续执行」 | 用户 2026-11-18「通过，开始 T00」 | 已确认 |

## 9. 恢复入口与执行账本

### 下次会话先读

- 当前阶段：M1 已批准，执行中。
- 下一个任务：T00（进行中）。
- 工作区：`main` 与 `origin/main` 同为 `39041a0`，工作区干净（本计划文件除外）。
- 待决事项：B-M1-05、B-M1-06；13 项窗口手动确认（M0 遗留）。

| 时间 | 任务与状态变化 | 文件 / 版本范围 | 验证与证据 | 待决事项 | 下一任务 |
| --- | --- | --- | --- | --- | --- |
| 2026-11-18 | 计划草案完成 | docs/PLAN-M1.md v0.1 | 自查见第 10 节；旧版盘点由只读子代理完成并抽查 | B-M1-05、B-M1-06 | T00 |
| 2026-11-18 | 独立审查（只读子代理，不附作者结论）：阻断 2 项、需要修改 6 项、建议 6 项，引用抽查 30 余处全部属实。已修订为 v0.2：阻断 1「进检索」由用户定为 M1 不读（B-M1-07）；阻断 2 补上传与读图的授权校验（T12、B-M1-08）；其余逐条落到 T00、T01、T02、T07、T08、T09、T11、T13 与第 6 节 | docs/PLAN-M1.md v0.2 | 两个阻断项由主会话亲自复核：旧版 `IndexService.ts` 无 categories 引用；`image.go` 无 CallerWindow 与大小检查 | B-M1-05、B-M1-06 | T00 |

## 10. 自查

- 覆盖：第 1.2 节每个需求都有任务与验收；PRD 第 12 节中属于 M1 的 AC（AC-AREA-1、AC-TREE-1、AC-LINK-1、AC-SRCH-1、AC-MUT-1、AC-MUT-2、AC-PROV-1）都映射到第 6 节；非目标写明去向。
- 接口一致：方法与事件编号都来自 API.md 2.2、2.3；存储格式只引用数据库设计的小节，本计划不另定义字段。T00 只改 M19 与同步说明这两处，都来自用户的选择。
- 未定值：检索去抖时间、归档时间戳格式写明「按旧版代码核对后实现」；已打开标签在文件移动后的表现给了默认口径（旧版没有就不做）。目录复制的来源记录粒度已按旧版定死。
- 已知风险：
  - T07、T08、T12 写用户文件，安排了两次独立审查和 T13 的新旧版往返测试；
  - T12 依赖 WebView2 自定义协议能收到 POST 请求体，未实测，第一步先验证；上传端点是新的写入口，授权校验写进了交接，T12 后独立审查；
  - T13 的兼容脚本依赖旧版模块能在 bun 下单独运行，未实测；
  - 链接全量解析和增量重算的耗时要在哈希实现上重测，结果可能要求调整 T04 的实现。
| 2026-11-18 | T03 完成 | `internal/index/search.go`：M24（查询截断 200、token 转 OR 的 FTS 查询、bm25 12/8/6/4/1、全命中优先、区域/扩展名/时间过滤、标签 OR 过滤、片段取自 doc_body、匹配字段按 FTS 列回算）；`internal/service/index.go` Search（未就绪时 partial）；`src/lib/transport.ts` indexApi；`src/stores/searchStore.ts`（180ms 去抖、查询序号丢弃过期响应、筛选/排序/选中项、E02 应用）；`src/components/SearchPanel.tsx` + `src/styles/search.css`（标题行 SEARCH 计数、输入框右侧排序分段、功能区/时间/扩展名筛选、右侧匹配字段、底栏快捷键提示）；侧栏「搜索 Ctrl P」入口与全局 Ctrl+P；Empty/无结果/索引未完成状态 | index 包 9 项检索测试：中文二元组命中与片段、标题权重高于正文、全 token 命中优先、系统区过滤与 includeSystem、扩展名/区域过滤、标签 OR、updatedAfter、仅路径命中的片段为空、空查询与 limit/截断。e2e：Ctrl+P 打开面板 → 索引 ready → 搜「知识库」得 1 条、匹配正文、片段高亮 1 处、底栏「1 条结果 · N ms · ↑↓ 选择 · Enter 打开 · Esc 关闭」；切路径排序、加 txt 过滤后 0 条、清除恢复；Enter 打开该文档（activePath 正确、面板关闭、标签 1 个）；截图 design/m0/t03-search-1440x900.png 与旧版 26 对照布局一致 | 相关性排序在 Go 侧对最多 2000 条候选做全命中重排（超过该数的超大结果集按 bm25 截断），已在代码注明；28 张空状态截图未逐张对照；深色模式未单独截图 | T04 |
| 2026-11-18 | T02 完成 | `internal/index`：OpenIndex（DSN 带 busy_timeout/WAL/NORMAL，写池单连接 + 读池）、嵌入式 001_init.sql（documents/doc_fts/doc_body/links/meta，documents 增 `token_count` 列、meta 增 `token_count` 键，数据库设计 4.5 已同步）、Build（按批 500 事务、跳过 node_modules/.git/.mile-kb、junction 逐个 Resolve、文本扩展名且 ≤2 MiB 且 UTF-8 才读正文）、Update（目录批次一事务：新增/变更重索引、消失删除、无变化不加 revision）、FTSQuery/BodyText/Meta 读接口；`internal/service/index.go`：M25 Status、M26 Rebuild（删库重建）、E02 状态事件、单写 worker 与溢出转全量；打开/注册/启动恢复三处接上，Forget 删库 | index 包 12 项测试（含建库计数、FTS 二元组命中与单字语义、增量增删改、无变化 revision 不变、重开不重建、schema 版本不符重建、目录归一）；service 集成测试 1 项（后台建库到 ready、重建、移除后无残留文件）；全仓 go test、vet、tsc、vitest 58 项通过。两次变异检验：去掉 .mile-kb 排除 → 建库测试失败；关闭增量删除 → 删文件测试失败，均已还原 | 首建进度事件未在真实窗口验证（T03 e2e 一并看）；`İ` 类大小写折叠差异沿用 T01 记录 | T03 |
| 2026-11-18 | T01 完成 | `internal/index`：Tokenize、TokenizePath、ParseDocument、BuildExcerpt（Go）+ 27 项移植/新增测试 + 24 行跨语言夹具；`src/lib/search.ts`：tokenize、MATCH_FIELD_LABEL/describeMatchedFields、highlightRanges + 21 项 tokenize 测试、9 项高亮测试、24 行对拍测试。计划里的 `highlight.ts` 合并进 `search.ts`（同一批渲染纯函数，避免两个十行文件） | go test 全绿（index 28 项）；vitest 58 项；tsc、vet 通过；fuzz 2s×2 目标通过。用旧版 `src/shared/tokenize.ts` 实跑同一夹具对拍：24 行中 23 行逐字一致，唯一差异是 emoji 行——旧版按 UTF-16 码元切，且 CJK 类最后一档到 U+FAFF 把代理区也圈进去了，于是产出半个字符的垃圾 token；Go 按码点、TS 显式跳过代理码元，两边一致且不产生破损 token。另：旧版最后一档是 U+8C48（豈）到 U+FAFF，Go 初版误写成 U+F900，已修正 | 有意差异：emoji/代理区切分（见上）；`İ` 等全大写折叠差异未纳入夹具 | T02 |
| 2026-11-18 | T00 完成 | D-009（6 项决定）写入 decisions.md；API.md：M19 改协议端点、方法总数 73→72、§3 映射改 change、§4.2 与 §5 重写（路径 + vault 参数、上传端点、5 条授权规则）；PRD v1.6：FR-AREA「进检索」备注、FR-PROV 同步规则、FR-DOC 图片传输、AC-PROV-2；技术设计门禁表接入状态；README 两条命令更正；`scripts/test-fuzz.sh`、`scripts/test-e2e.sh` 新增；`ac-m0.sh` 失败时返回非零 | fuzz：正向 3s×2 目标通过、负向（临时必失败目标）退出 1；e2e：正向 15/15 退出 0、负向（翻转一条断言）14/15 退出 1。fuzz 首次实跑抓到 `FuzzValidateRel` 断言过严（把「名字含 ..」当成越界，`..0` 其实是合法名）：修正断言为「任何路径段都不是 . 或 ..」，补 `..0`/`a..b.md`/`notes..txt` 三个合法用例并把 `..0` 加入 fuzz 种子；这不是产品代码缺陷，M0 的 fuzz 门禁当时从未真正跑过所以没暴露 | 无 | T01 |
| 2026-11-18 | 计划已批准（B-M1-05、B-M1-06），开始 T00 | docs/PLAN-M1.md v1.0 | 用户回复「通过，开始 T00」 | — | T00 |
