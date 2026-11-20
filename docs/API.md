# 接口设计契约

> 状态：v1.0 草案（阶段 08），已通过设计交接检查并修订，用户 2026-11-18 审核通过；全部接口「已设计、未实现」。
> 适用迭代：R1 重构对齐版；业务依据 [PRD.md](PRD.md) v1.5；技术依据 [技术设计.md](技术设计.md) v1.0。
> 形式：本应用是单进程桌面应用，「接口」不是 HTTP API，而是 mygo 绑定服务（前端得到 Promise）、`mygo.Channel[T]`（流式）与定向 Event（Go 推前端）。前端只经 `src/lib/transport.ts` 调用生成的 `src/mygo.ts`。
> 持久化格式（`.mile-kb/` 文件、SQLite 索引、应用数据文件）不在本文，见 [数据库设计.md](数据库设计.md)。
> 旧版频道全集：`Mile-Knowledge-Base-master/src/shared/ipc.ts`（841 行）；入参校验：`src/main/ipc/validate.ts`；错误码：`src/shared/errors.ts`。引用旧版行号以 2026-11-18 盘点为准。

## 1. 全局约定

### 1.1 约定总表

| 类别 | 结论 | 依据 |
| --- | --- | --- |
| 接口形式 | mygo 绑定服务方法（请求-响应，Promise）；`*mygo.Channel[T]` 参数承载引擎流式输出；`mygo.Event[T]` 承载 Go → 前端的定向通知。无 HTTP 路由、无版本路径；契约演进靠 `mygo generate` 重新生成 `src/mygo.ts` 并整体发布 | mygo `docs/bindings.md`；技术设计 §8 |
| 服务划分 | 按技术设计 §3 模块拆成 9 个绑定服务（**建议名，实现时可微调但需回写本文**）：`WindowService`、`VaultService`、`DocService`、`IndexService`、`MutationService`、`RenderService`、`AgentService`、`ConversationService`、`SettingsService`（`ContextService` 并入 `AgentService`，`ProvenanceService` 并入 `MutationService`——见 §3 目录）。服务结构体不放状态，只做装配、入参校验、错误转换、定向推送 | mygo-web-app skill `bindings-and-channels.md` §1 |
| 身份与权限 | 单用户本机应用，无凭据、无会话 token、无登录态。权限在 Go 侧强制执行：(a) 所有路径经 `pathguard` 两道判定（拼写层 + 真实路径层），前端传的 vaultId 只允许 `[A-Za-z0-9_-]` ≤128 字符；(b) 窗口身份用 `mygo.CallerWindow(ctx)` 取，**不信前端传的窗口 id**；(c) Agent 写操作另过系统区禁写与执行模式判定（agnes-engine skill §7-8） | PRD §3、§10.2；go-service-engineering §4 |
| 错误语义 | Go 方法返回 `error` → 前端拿到 `CallError`（只有 `name`/`method`/`message` 三个字段，**不能携带结构化错误码**，见 §1.2）。因此可预期失败一律放进**返回值联合类型**（`failure` 字段 + `code`），只有意外故障（panic、内部状态丢失、IO 不可恢复）才返回 Go `error`。错误码表见 §1.3 | mygo-runtime `index.ts:198-207`；mygo-web-app skill `bindings-and-channels.md` §2 |
| 跨端数据语义 | vaultId：16 位十六进制（真实路径小写 SHA-256 前 16 位，与旧版 `vaultIdFor` 一致，技术设计 §4.7）。路径：一律库内相对、`/` 分隔、空串表示库根，Go 侧规范化后使用；前端永远拿不到绝对路径（仅展示用途的 `displayName`/`root` 例外，见 §4.1）。时间：ISO 8601 UTC 字符串（`time.Time` → RFC 3339）。revision：从 1 递增的整数（`number`，JS 安全范围）。枚举：全部小写字符串。空值：缺省字段用 `omitempty`；`null` 只表示「显式清空」（如 `lastVaultId: null`）。字节数、大小：`int64`。整数范围：不超出 JS `Number.MAX_SAFE_INTEGER` | PRD FR-VLT；旧版 `shared/vault.ts:15-18` |
| 集合查询 | 无 offset/cursor 分页；用 `limit` 上限截断，各上限沿用 PRD 与旧版 validate.ts：检索默认 50 最多 200（`search.ts:84-85`）；操作日志最多 1000（validate.ts:385）；最近文档 12（recentDocuments.ts:14）；最近知识库 12（settings.ts:131）；对话 200/库（conversation.ts:19）；技能导入一次 ≤200 个（validate.ts:753）。超限一律拒绝或截断并标注，不自动翻页 | 各旧版文件行号如左 |
| 写入可靠性 | 文档保存用 `expectedRevision` 乐观并发：不符返回 `conflict` 不写盘（FR-DOC）。变更计划先 `plan` 后 `execute` 两段：执行前逐条复核 expectedSourceRevision（旧版 mutation.ts:6 注释）。幂等：`saveImage` 按内容哈希去重；`open`/`retry`/`create` 幂等可重入。取消：所有长操作从绑定方法的 `ctx` 派生，前端 `AbortSignal` / `channel.close()` / 关窗都会取消 | PRD FR-DOC、§9；go-service-engineering §3 |
| 资源与失败 | 大小上限：单文档读 2 MiB（>1 MiB 禁阅读模式、>2 MiB 只读，vault.ts:146-147）、写 2 MiB 拒绝不截断（document.ts:87）、图片 8 MiB（document.ts:97）、技能包 2000 文件/50 MiB（FR-SKILL）。超时：模型拉取 10s/2MiB 响应（FR-AI-PROV）；引擎流式空闲 300s 中止（FR-AI-ENG）。取消统一用 `ctx`。无应用级限流（本机单用户），但审批卡与两步确认有防误触规则（FR-AI-PERM） | PRD 各节；旧版行号如左 |
| 诊断与脱敏 | 错误出口统一脱敏：vault 根 → `<vault>`、用户主目录 → `<home>`、`KEY=value`/`token:` ≥8 字符的值、sk-/pk-/ghp-/gho-/xoxb-/xoxp- 前缀 key 一律替换；不向前端发调用栈、不发完整路径、不发文件正文（PRD §10.2）。诊断入口：设置页「关于与诊断」返回引擎状态、重启次数、能力、key 存放位置说明（FR-SET） | PRD §10.2；go-service-engineering §9 |

### 1.2 mygo 错误传递核对结论

已核对 mygo v0.10.0 源码：`mygo-runtime` 的 `CallError` 接口**只有 `name`、`method`、`message` 三个字段**（`packages/runtime/src/index.ts:198-207`），Go 侧 `error` 的文本原样进入 `message`，没有 code 字段、不能携带结构化错误码。

因此本契约规定：

- **可预期失败**（参数校验、not-found、权限、超限、编码、冲突、计划被拒等 12 类 + AI 侧 4 类，见 §1.3）**不返回 Go error**，而是返回带 `failure` 字段的结果结构（判别联合），TS 侧类型安全地 `switch (result.failure?.code)`：

```go
// internal/contract
type Failure struct {
    Code   string `json:"code"`             // §1.3 表内值
    Detail string `json:"detail"`           // 用户可读一句话，已脱敏
    Size   int64  `json:"size,omitempty"`   // too_large 时给实际大小
    Limit  int64  `json:"limit,omitempty"`  // too_large 时给上限
}

// 用法：每个可能可预期失败的返回结构都带 *Failure 字段
type LoadResult struct {
    Doc      *DocPayload `json:"doc,omitempty"`
    Failure  *Failure    `json:"failure,omitempty"`
    Revision int64       `json:"revision,omitempty"`
}
```

- **意外故障**（程序缺陷、内部不可恢复状态）才返回 Go `error` → 前端 `CallError`，transport 层映射为通用文案「发生未知错误，请重试」并写日志。
- 这样设计的原因：旧版用 `Result<T>` 包裹所有 invoke（`shared/ipc.ts:453-454`），新版用「返回值联合 + CallError 兜底」达到同等的类型安全，且比 IPC 时代更严格——可预期失败连 rejection 都不产生。

### 1.3 错误码表（完整清单）

沿用旧版 12 个码（`shared/errors.ts:6-19`），新增 AI/引擎侧 4 个。`Code` 列即 `Failure.Code` 的字符串；「可重试」指用户层面是否值得重试；「前端提示」是 transport → 行内警示卡的建议文案方向（最终文案以 DESIGN.md 为准）。

| code | 含义 | 可重试 | 前端提示方向 | 来源 |
| --- | --- | --- | --- | --- |
| `invalid-input` | 入参校验失败（类型、长度、枚举、格式） | 否（先改输入） | 具体字段错误 | 旧版 errors.ts:7 |
| `path-escape` | 路径越出知识库 / 命中禁写区 | 否 | 没有访问权限 | 旧版 :8 |
| `not-found` | 文件、目录、知识库、对话、事务不存在 | 否 | 对象不存在 | 旧版 :9 |
| `not-a-file` | 目标是目录不是文件 | 否 | 不是文件 | 旧版 :10 |
| `not-a-directory` | 目标是文件不是目录 | 否 | 不是目录 | 旧版 :11 |
| `too-large` | 超过大小/数量上限（带 Size/Limit 字段） | 否 | 超出上限 | 旧版 :12 |
| `unsupported-encoding` | 文件不是严格 UTF-8 | 否 | 不支持的编码 | 旧版 :13 |
| `revision-mismatch` | 保存时磁盘已被外部修改 | 用户选择后 | 磁盘版本已变化 | 旧版 :14 |
| `vault-unavailable` | 知识库已断开 / 未登记 | 是（重连后） | 知识库不可用 | 旧版 :15 |
| `permission-denied` | 无权限（系统区禁写、未批准、永久删除缺新鲜审批） | 用户操作后 | 没有权限 | 旧版 :16 |
| `io-failure` | 磁盘/系统调用失败 | 是 | 操作失败，请重试 | 旧版 :17 |
| `unsupported` | 功能不支持（如非 openai 供应商调用引擎） | 否 | 暂不支持 | 旧版 :18 |
| `engine-unavailable` | 引擎未配置供应商 / 未就绪 / 出错 | 是（配置后） | 引擎不可用 + 原因 | 新增（FR-AI-ENG） |
| `provider-error` | 供应商请求失败（网络、限流、5xx、鉴权） | 视 detail（自动重试中或已耗尽） | 供应商错误 + 脱敏原因 | 新增（FR-AI-ENG/PROV） |
| `approval-expired` | 审批卡已失效（回合取消、引擎重启、超时） | 否 | 审批已失效 | 新增（FR-AI-PERM） |
| `secret-unavailable` | 系统加密不可用 / 凭据管理器失败 | 否 | 系统加密不可用 | 新增（FR-AI-PROV，mygo `ErrUnsupported`） |
| `skill-conflict` | 技能名冲突或目标已存在、恢复时原位置被占用（detail 给出另一处位置） | 用户改名/处理后 | 名称或目标冲突 | 新增（FR-SKM-DIST/UNINST，§12） |
| `skill-protected` | 内置技能不可卸载/覆盖，或工具未启用 | 否 | 内置技能只读 | 新增（FR-SKM-UNINST，§12） |
| `backup-missing` | 备份已被清理或记录不可恢复 | 否 | 备份不存在 | 新增（FR-SKM-UNINST，§12） |

### 1.4 通用入参校验规则（所有方法）

以下规则对**所有**绑定方法生效，不再在每个接口重复（来源：旧版 `ipc/validate.ts`，行号标注在括号内）：

| 项 | 规则 | 来源 |
| --- | --- | --- |
| vaultId | 字符串必填，`[A-Za-z0-9_-]+`，≤128 字符 | validate.ts:117-122 |
| 相对路径 | `pathguard` 两道判定；拼写层：≤1024、拒绝 NUL/UNC/绝对路径/盘符/`.`/`..` 片段/`<>:"\|?*` 与控制字符/首尾点或空格/DOS 保留名 | validate.ts:187-191；go-service-engineering §4.1 |
| AgentId | `curator/scout/distiller/general` 之一 | validate.ts:124-127 |
| requestId（审批） | `[A-Za-z0-9_-]+`，≤256 | validate.ts:131-134 |
| optionId | 非空字符串 ≤256，不限制字符集（透传回引擎） | validate.ts:144-146 |
| id 类（taskId/referenceId/conversationId/planId/transactionId/configId） | 各自字符集 `[A-Za-z0-9_-]`（referenceId 另允许 `:`），长度上限见各接口 | validate.ts:153-184, 388-394, 708-711 |
| 字符串通用上限 | 默认 4096 字符；超长策略：自由文本截断（`truncate`），prompt 等「所见即所发」字段拒绝（`reject`） | validate.ts:56, 74-83, 148-151 |
| 布尔/整数 | 严格类型检查，拒绝真值字符串与浮点 | validate.ts:85-108 |
| 对象/数组 | 严格形状检查，白名单外键静默丢弃不报错；非法值抛错前不写任何一部分 | validate.ts:1-9 头注释 |
| 未提供字段 | 不写入（局部 patch 不清空用户其它设置） | 同上 |

## 2. 服务总览与接口目录

### 2.1 服务清单

| 服务（建议名） | 职责 | 对应 internal 包 | 覆盖 PRD |
| --- | --- | --- | --- |
| `WindowService` | 标题栏按钮、关窗询问、系统对话框（选目录/保存）、外链、托盘状态 | `main`/`service` | FR-WIN、FR-THEME |
| `VaultService` | 知识库登记/打开/切换/断开/新建向导/脚手架/监听状态 | `vault` | FR-VLT、FR-AREA（分类读侧） |
| `DocService` | 树懒加载、文档读写、图片保存、revision 冲突 | `vault` | FR-TREE、FR-DOC |
| `IndexService` | 全文检索、链接图、健康检查、索引状态/重建 | `index` | FR-SRCH、FR-LINK、FR-HLTH |
| `MutationService` | 变更计划/执行、事务恢复、操作日志、来源追踪 | `mutation` | FR-MUT、FR-PROV |
| `RenderService` | Markdown → HTML（阅读模式与对话块）、链接分类、图片 URL 生成 | `render` | FR-DOC、FR-LINK、FR-AI-CHAT |
| `AgentService` | 4 个 Agent 会话、流式回合、审批、路由预览、共享上下文 | `engine`/`agent` | FR-AI-* 全部 |
| `ConversationService` | 对话历史 CRUD、搜索、导出、接着问激活 | `conversation`/`appdata` | FR-AI-HIST |
| `SettingsService` | 应用设置、布局、主题、供应商、API key、分类/别名、最近文档、旧版导入 | `settings`/`appdata` | FR-SET、FR-AI-PROV、FR-OVW（最近文档） |
| `SkillService` | 工具注册表与探测、两套技能扫描、分发/卸载/收编、备份与恢复、操作记录、清理备份 | `skills` | FR-SKM-TOOL/DETECT/SCAN/DIST/UNINST/ADOPT/LOG（M4） |

### 2.2 接口目录（绑定方法）

「Go 签名」省略首个 `ctx context.Context` 参数（TS 侧不出现）；`→ T` 表示返回 `(T, error)`；`failure` 表示返回值内含 `*Failure` 判别字段。状态列统一为「已设计、未实现」。

| # | 方法（TS 形状） | 所属服务 | PRD | Go 签名要点 | 可预期失败 |
| --- | --- | --- | --- | --- | --- |
| M01 | `WindowService.minimize()` | Window | FR-WIN | `func (WindowService) Minimize(ctx) error` | — |
| M02 | `WindowService.toggleMaximize()` | Window | FR-WIN | `ToggleMaximize(ctx) error`（Go 按 IsMaximized 切换） | — |
| M03 | `WindowService.close()` | Window | FR-WIN | `Close(ctx) error`（走 OnClose 拦截） | — |
| M04 | `WindowService.ready()` | Window | 全部 | `Ready(ctx) error`（就绪握手，幂等） | — |
| M05 | `WindowService.openExternal(url)` | Window | FR-LINK、§10.2 | `OpenExternal(ctx, url string) error` | `invalid-input`（非 http/https） |
| M06 | `WindowService.pickVaultDir()` | Window | FR-VLT | `PickVaultDir(ctx) (*PickResult, error)` | —（cancelled 是正常返回非错误） |
| M07 | `VaultService.list()` | Vault | FR-VLT | `List(ctx) (*VaultListResult, error)` | — |
| M08 | `VaultService.open(vaultId)` | Vault | FR-VLT | `Open(ctx, vaultId string) (*VaultListResult, error)` | `vault-unavailable` |
| M09 | `VaultService.choose()` | Vault | FR-VLT | `Choose(ctx) (*ChooseVaultResult, error)` | — |
| M10 | `VaultService.forget(vaultId)` | Vault | FR-VLT | `Forget(ctx, vaultId string) (*VaultListResult, error)` | `vault-unavailable` |
| M11 | `VaultService.retry(vaultId)` | Vault | FR-VLT | `Retry(ctx, vaultId string) (*VaultListResult, error)` | —（仍断开时留在 unavailable） |
| M12 | `VaultService.pickFolder()` | Vault | FR-VLT 向导 | `PickFolder(ctx) (*PickFolderResult, error)` | — |
| M13 | `VaultService.createVault(token, confirmNonEmpty)` | Vault | FR-VLT 向导 | `CreateVault(ctx, token string, confirm bool) (*ChooseVaultResult, error)` | `invalid-input`、`permission-denied`（非空未确认） |
| M14 | `VaultService.pickSkillSource()` | Vault | FR-SKILL | `PickSkillSource(ctx) (*PickSkillSourceResult, error)` | — |
| M15 | `VaultService.importSkills(vaultId, token, category, names)` | Vault | FR-SKILL | `ImportSkills(ctx, ...) (*ImportSkillsResult, error)` | `invalid-input`、`too-large`、`io-failure` |
| M16 | `DocService.tree(vaultId, path)` | Doc | FR-TREE | `Tree(ctx, vaultId, path string) (*TreeResult, error)` | `vault-unavailable`、`not-a-directory`、`path-escape` |
| M17 | `DocService.read(vaultId, path)` | Doc | FR-DOC | `Read(ctx, vaultId, path string) (*ReadResult, error)` | `not-found`、`not-a-file`、`too-large`、`unsupported-encoding`、`path-escape` |
| M18 | `DocService.write(vaultId, path, expectedRevision, text)` | Doc | FR-DOC | `Write(ctx, vaultId, path string, expectedRev int64, text string) (*WriteOutcome, error)` | `revision-mismatch`（返回 conflict 形态）、`too-large`、`path-escape` |
| M19 | 协议端点 `POST http://kbimg.localhost/__upload?vault=<id>`（§5） | Doc | FR-DOC | 不是绑定方法；请求体为图片原始字节，响应 JSON `{path, existed}` 或 `{failure}` | `too-large`、`invalid-input`（非图片魔数、SVG/ICO）、`vault-unavailable`、`path-escape` |
| M20 | `DocService.rename(vaultId, path, newName)` | Doc | FR-TREE | `Rename(ctx, ...) (*PlanResult, error)` | 同 M21 |
| M21 | `DocService.createFile(vaultId, dirPath, name)` | Doc | FR-TREE | `CreateFile(ctx, ...) (*PlanResult, error)` | 走变更计划：返回 plan 供确认，不直接落盘 |
| M22 | `DocService.createDir(vaultId, dirPath, name)` | Doc | FR-TREE | `CreateDir(ctx, ...) (*PlanResult, error)` | 同上 |
| M23 | `DocService.planMove(vaultId, sources, targetDir)` | Doc | FR-TREE 拖放 | `PlanMove(ctx, ...) (*PlanResult, error)` | — |
| M24 | `IndexService.search(query)` | Index | FR-SRCH | `Search(ctx, q SearchQuery) (*SearchResult, error)` | `vault-unavailable`、`invalid-input` |
| M25 | `IndexService.status(vaultId)` | Index | FR-SRCH | `Status(ctx, vaultId string) (*IndexStatus, error)` | `vault-unavailable` |
| M26 | `IndexService.rebuild(vaultId)` | Index | FR-SRCH | `Rebuild(ctx, vaultId string) (*IndexStatus, error)` | `vault-unavailable` |
| M27 | `IndexService.links(vaultId, path)` | Index | FR-LINK | `Links(ctx, vaultId, path string) (*LinkGraphResult, error)` | `not-found` |
| M28 | `IndexService.health(vaultId)` | Index | FR-HLTH | `Health(ctx, vaultId string) (*KbHealthReport, error)` | `vault-unavailable` |
| M29 | `MutationService.plan(request)` | Mutation | FR-MUT | `Plan(ctx, req PlanRequest) (*PlanResult, error)` | `invalid-input`、`path-escape`、`name-conflict`（目标已存在） |
| M30 | `MutationService.execute(plan)` | Mutation | FR-MUT | `Execute(ctx, plan MutationPlan) (*MutationReport, error)` | `invalid-input`、`permission-denied`（permanentDelete 无 freshApproval）、`io-failure` |
| M31 | `MutationService.auditList(vaultId?, planId?, limit?)` | Mutation | FR-MUT | `AuditList(ctx, ...) (*AuditListResult, error)` | `invalid-input` |
| M32 | `MutationService.rewindPrepare(vaultId, transactionId)` | Mutation | FR-MUT | `RewindPrepare(ctx, ...) (*RestorePreview, error)` | `not-found`、`invalid-input` |
| M33 | `MutationService.rewindCommit(vaultId, transactionId)` | Mutation | FR-MUT | `RewindCommit(ctx, ...) (*RestoreResult, error)` | `not-found`、`io-failure` |
| M34 | `MutationService.provenanceList(vaultId)` | Mutation | FR-PROV | `ProvenanceList(ctx, vaultId string) ([]ProvenanceEntry, error)` | `vault-unavailable` |
| M35 | `MutationService.provenanceState(vaultId, path)` | Mutation | FR-PROV | `ProvenanceState(ctx, vaultId, path string) (*ProvenanceStateResult, error)` | `not-found` |
| M36 | `RenderService.renderDocument(vaultId, path, revision)` | Render | FR-DOC | `RenderDocument(ctx, ...) (*RenderResult, error)` | `not-found`、`too-large`、`revision-mismatch` |
| M37 | `RenderService.renderBlock(text, kind)` | Render | FR-AI-CHAT | `RenderBlock(ctx, text string, kind string) (*BlockResult, error)` | `invalid-input` |
| M38 | `AgentService.list(agent, vaultId)` | Agent | FR-AI-AGT | `List(ctx, agent, vaultId string) (*AgentListResult, error)` | `vault-unavailable` |
| M39 | `AgentService.prompt(agent, vaultId, text, ch)` | Agent | FR-AI-ENG/CHAT | `Prompt(ctx, agent, vaultId, text string, ch *mygo.Channel[StreamFrame]) (*PromptResult, error)` | `invalid-input`、`engine-unavailable`、`too-large`（>32000 字符拒绝） |
| M40 | `AgentService.cancel(agent)` | Agent | FR-AI-ENG | `Cancel(ctx, agent string) error` | — |
| M41 | `AgentService.approve(requestId, optionId)` | Agent | FR-AI-PERM | `Approve(ctx, requestId, optionId string) error` | `approval-expired`、`invalid-input` |
| M42 | `AgentService.reject(requestId)` | Agent | FR-AI-PERM | `Reject(ctx, requestId string) error` | `approval-expired`、`invalid-input` |
| M43 | `AgentService.reset(vaultId)` | Agent | FR-AI-HIST | `Reset(ctx, vaultId string) error` | — |
| M44 | `AgentService.routePreview(text)` | Agent | FR-AI-ROUTE | `RoutePreview(ctx, text string) (*RoutePreviewResult, error)` | — |
| M45 | `AgentService.contextSnapshot(vaultId)` | Agent | FR-AI-CTX | `ContextSnapshot(ctx, vaultId string) (*SharedContext, error)` | `vault-unavailable` |
| M46 | `AgentService.contextPin(vaultId, path, note?)` | Agent | FR-AI-CTX | `ContextPin(ctx, vaultId, path string, note *string) (*SharedContext, error)` | `not-found`、`path-escape` |
| M47 | `AgentService.contextUnpin(referenceId)` | Agent | FR-AI-CTX | `ContextUnpin(ctx, referenceId string) (*SharedContext, error)` | `invalid-input` |
| M48 | `AgentService.contextSetTaskStatus(taskId, status)` | Agent | FR-AI-CTX | `ContextSetTaskStatus(ctx, taskId, status string) (*SharedContext, error)` | `invalid-input` |
| M49 | `AgentService.contextReportDocument(vaultId, path?, revision, dirty)` | Agent | FR-AI-CTX | `ContextReportDocument(ctx, ...) error` | `invalid-input`、`path-escape` |
| M50 | `ConversationService.list(vaultId)` | Conversation | FR-AI-HIST | `List(ctx, vaultId string) (*ConversationIndex, error)` | `vault-unavailable` |
| M51 | `ConversationService.get(vaultId, id)` | Conversation | FR-AI-HIST | `Get(ctx, vaultId, id string) (*ConversationGetResult, error)` | `not-found` |
| M52 | `ConversationService.create(vaultId)` | Conversation | FR-AI-HIST | `Create(ctx, vaultId string) (*ConversationMeta, error)` | `vault-unavailable` |
| M53 | `ConversationService.rename(vaultId, id, title)` | Conversation | FR-AI-HIST | `Rename(ctx, ...) (*ConversationMeta, error)` | `not-found`、`invalid-input` |
| M54 | `ConversationService.pin(vaultId, id, pinned)` | Conversation | FR-AI-HIST | `Pin(ctx, ...) (*ConversationMeta, error)` | `not-found` |
| M55 | `ConversationService.remove(vaultId, id)` | Conversation | FR-AI-HIST | `Remove(ctx, vaultId, id string) error` | `not-found` |
| M56 | `ConversationService.activate(vaultId, id)` | Conversation | FR-AI-HIST | `Activate(ctx, ...) (*ConversationGetResult, error)` | `not-found`、`invalid-input`（Agent 运行中拒绝切换） |
| M57 | `ConversationService.search(vaultId, query)` | Conversation | FR-AI-HIST | `Search(ctx, ...) (*ConversationSearchResult, error)` | `invalid-input` |
| M58 | `ConversationService.export(vaultId, id)` | Conversation | FR-AI-HIST | `Export(ctx, ...) (*ConversationExportResult, error)` | `not-found`、`io-failure` |
| M59 | `ConversationService.clear(vaultId)` | Conversation | FR-AI-HIST | `Clear(ctx, vaultId string) (*ConversationClearResult, error)` | `vault-unavailable` |
| M60 | `SettingsService.get()` | Settings | FR-SET | `Get(ctx) (*SettingsResult, error)` | — |
| M61 | `SettingsService.patch(patch)` | Settings | FR-SET | `Patch(ctx, patch SettingsPatch) (*SettingsResult, error)` | `invalid-input` |
| M62 | `SettingsService.appConfigGet(vaultId)` | Settings | FR-SET | `AppConfigGet(ctx, vaultId string) (*AppConfigResult, error)` | `vault-unavailable` |
| M63 | `SettingsService.appConfigPatchAgent(vaultId, agent, patch)` | Settings | FR-AI-PROV | `AppConfigPatchAgent(ctx, ...) (*AppConfigResult, error)` | `invalid-input` |
| M64 | `SettingsService.appConfigResetSteering(vaultId, agent)` | Settings | FR-AI-PROV | `AppConfigResetSteering(ctx, ...) (*AppConfigResult, error)` | `invalid-input` |
| M65 | `SettingsService.appConfigSetCategories(vaultId, categories)` | Settings | FR-AREA | `AppConfigSetCategories(ctx, ...) (*AppConfigResult, error)` | `invalid-input` |
| M66 | `SettingsService.appConfigPatchAliases(vaultId, aliases)` | Settings | FR-AREA | `AppConfigPatchAliases(ctx, ...) (*AppConfigResult, error)` | `invalid-input` |
| M67 | `SettingsService.providers(vaultId)` | Settings | FR-AI-PROV | `Providers(ctx, vaultId string) (*ProvidersResult, error)` | `vault-unavailable` |
| M68 | `SettingsService.saveProviders(vaultId, providers)` | Settings | FR-AI-PROV | `SaveProviders(ctx, ...) (*ProvidersResult, error)` | `invalid-input`、`io-failure` |
| M69 | `SettingsService.setKey(envName, value)` | Settings | FR-AI-PROV | `SetKey(ctx, envName, value string) (*ProvidersResult, error)`；无 vaultId，成功后按**活动知识库**刷新快照（与旧版 `register.ts:773-787` 一致），没有活动库时只回该 envName 的凭据状态 | `invalid-input`、`secret-unavailable`、`too-large`（>2560 字节） |
| M70 | `SettingsService.fetchModels(kind, baseUrl, apiKeyEnv, apiKey?)` | Settings | FR-AI-PROV | `FetchModels(ctx, ...) (*FetchModelsResult, error)` | `invalid-input`、`permission-denied`（401/403）、`unsupported`（404/405）、`io-failure`（网络/超时/非 JSON/5xx）、`too-large`（响应 >2 MiB） |
| M71 | `SettingsService.skillCatalog(vaultId)` | Settings | FR-AI-PROV | `SkillCatalog(ctx, vaultId string) (*SkillCatalogResult, error)` | `vault-unavailable` |
| M72 | `SettingsService.recentDocuments(vaultId)` | Settings | FR-OVW | `RecentDocuments(ctx, vaultId string) (*RecentDocumentsResult, error)` | `vault-unavailable` |
| M73 | `SettingsService.noteDocumentOpened(vaultId, path)` | Settings | FR-OVW | `NoteDocumentOpened(ctx, ...) (*RecentDocumentsResult, error)` | `not-found`、`invalid-input` |
| M74 | `SettingsService.aboutInfo()` | Settings | FR-SET | `AboutInfo(ctx) (*AboutInfoResult, error)` | `io-failure` |
| M75 | `SettingsService.copyStoragePath(kind)` | Settings | FR-SET | `CopyStoragePath(ctx, kind string) (*CopyStoragePathResult, error)` | `invalid-input`、`unsupported`（credentials）、`io-failure` |
| M76 | `SkillService.registry()` | Skills | FR-SKM-TOOL/DETECT | `Registry(ctx) (*SkillRegistryResult, error)` | — |
| M77 | `SkillService.scan(vaultId)` | Skills | FR-SKM-SCAN | `Scan(ctx, vaultId string) (*SkillScanResult, error)` | `invalid-input` |
| M78 | `SkillService.distributePreview(vaultId, skillPath, targets, mode)` | Skills | FR-SKM-DIST | `DistributePreview(ctx, ...) (*SkillPreviewResult, error)` | `invalid-input`、`not-found`、`skill-conflict` |
| M79 | `SkillService.distribute(vaultId, skillPath, targets, mode)` | Skills | FR-SKM-DIST | `Distribute(ctx, ...) (*SkillOpResult, error)` | `invalid-input`、`not-found`、`skill-conflict`、`skill-protected`、`io-failure` |
| M80 | `SkillService.uninstallPreview(toolId, path)` | Skills | FR-SKM-UNINST | `UninstallPreview(ctx, ...) (*SkillUninstallPreview, error)` | `invalid-input`、`not-found`、`skill-protected` |
| M81 | `SkillService.uninstall(toolId, path)` | Skills | FR-SKM-UNINST | `Uninstall(ctx, ...) (*SkillOpResult, error)` | `invalid-input`、`not-found`、`skill-protected`、`io-failure` |
| M82 | `SkillService.adopt(toolId, path, category, linkBack, extras)` | Skills | FR-SKM-ADOPT | `Adopt(ctx, ...) (*SkillAdoptResult, error)` | `invalid-input`、`not-found`、`skill-conflict`、`too-large`、`io-failure` |
| M83 | `SkillService.restore(backupId)` | Skills | FR-SKM-UNINST/LOG | `Restore(ctx, backupId string) (*SkillRestoreResult, error)` | `invalid-input`、`backup-missing`、`skill-conflict`、`io-failure` |
| M84 | `SkillService.log(limit)` | Skills | FR-SKM-LOG | `Log(ctx, limit int) (*SkillLogResult, error)` | `invalid-input` |
| M85 | `SkillService.backups()` | Skills | FR-SKM-UNINST（Q-SKM-3） | `Backups(ctx) (*SkillBackupsResult, error)` | — |
| M86 | `SkillService.cleanBackups(ids)` | Skills | FR-SKM-UNINST（Q-SKM-3） | `CleanBackups(ctx, ids []string) (*SkillBackupsResult, error)` | `invalid-input`、`io-failure` |

共 85 个绑定方法；M19 是协议端点（§5），不在绑定方法内。

### 2.3 Channel 与事件目录

| # | 名称 | 方向 | 载荷 | 触发时机 | 关联方法/PRD |
| --- | --- | --- | --- | --- | --- |
| C01 | `mygo.Channel[StreamFrame]`（`Prompt` 参数） | Go → 前端，单次调用 | `StreamFrame` 联合（§6） | 引擎回合期间持续推帧 | M39；FR-AI-ENG/CHAT |
| E01 | `vault-changed` | Go → 前端 | `{vaultId, directories[], sequence}` | 文件监听检测到目录内容变化（已合并去抖） | FR-TREE；旧版 `vault:changed` |
| E02 | `index-status` | Go → 前端 | `IndexStatus` | 索引阶段变化与进度（scanning/ready/error） | FR-SRCH；旧版 `index:changed` |
| E03 | `audit-changed` | Go → 前端 | `{vaultId?, sequence}` | 操作日志新增记录 | FR-MUT；旧版 `audit:changed` |
| E04 | `settings-changed` | Go → 前端 | `AppSettings` | 应用设置持久化成功 | FR-SET；旧版 `settings:changed` |
| E05 | `theme-changed` | Go → 前端 | `{mode, resolved}` | 主题切换生效 | FR-THEME；旧版 `theme:changed` |
| E06 | `window-state` | Go → 前端 | `{maximized, focused}` | 最大化/焦点变化 | FR-WIN；旧版 `window:state` |
| E07 | `conversation-changed` | Go → 前端 | `{vaultId}` | 对话历史落盘（新对话/新事件/删除） | FR-AI-HIST；旧版 `conversation:changed` |
| E08 | `app-config-changed` | Go → 前端 | `{vaultId}` | Agent 通过工具修改了 AppConfig（如 set_aliases） | FR-AI-AGT；旧版 `app-config:changed` |
| E09 | `context-changed` | Go → 前端 | `SharedContext` | 共享上下文变化（pin/task/文档状态） | FR-AI-CTX；旧版 `context:changed` |
| E10 | `engine-status` | Go → 前端 | `EngineStatus`（M38 同一定义：`{phase, message?, capabilities?}`） | 引擎未配置/就绪/出错（原 sidecar 状态） | FR-AI-ENG；旧版 `sidecar` 事件 |
| E11 | `app-notice` | Go → 前端 | `{text, level}` | 不阻断的提示（设置导入失败、监听降级等） | 新增（FR-SET 提示、R-1 降级提示） |
| E12 | `skills-changed` | Go → 前端 | `{sequence}` | 后台扫描完成或技能操作完成 | M76–M86；FR-SKM-SCAN/LOG |

共 1 个 Channel 参数 + 11 个定向事件。引擎流式帧、审批卡、用量、工具调用都走 C01 Channel 的 `StreamFrame` 联合（§6），不另立事件——只有跨调用持久的状态（索引、日志、设置、上下文）才用 Event。

## 3. 旧版频道 → 新版接口映射表

「处理」列：`same`=能力等价迁移；`merge`=合并进其他接口；`split`=拆分；`change`=形式变化；`drop`=删除。

| 旧版频道（ipc.ts:46-125） | 新版接口 | 处理 | 说明 |
| --- | --- | --- | --- |
| `vault:list` | `VaultService.list` M07 | same | — |
| `vault:choose` | `VaultService.choose` M09 | same | — |
| `vault:open` | `VaultService.open` M08 | same | — |
| `vault:forget` | `VaultService.forget` M10 | same | — |
| `vault:retry` | `VaultService.retry` M11 | same | — |
| `vault:tree` | `DocService.tree` M16 | same | 懒加载一层 |
| `vault:read` | `DocService.read` M17 | same | — |
| `vault:read-image` | **删除** | drop | 被自定义图片协议 `kbimg://` 替代（§5）。旧版用 base64 data URI 回传（ipc.ts:236-256），新版图片不经 IPC，彻底解决 P-5 型传输与 8MiB base64 膨胀 |
| `vault:save-image` | 协议端点 `kbimg` `POST /__upload`（M19，§5） | change | 旧版经 IPC 传字节；新版由页面直接 POST 到自定义协议，Go 侧仍做 vaultId 校验、文件头判定与内容去重。不经 base64（PRD FR-DOC、D-009） |
| `vault:write` | `DocService.write` M18 | same | — |
| `vault:mutate` | `MutationService.execute` M30 | split | 旧版一个频道既做计划内嵌校验又执行；新版把「生成计划」拆给 M20-M23/M29（前端先拿 plan 显示确认卡），`execute` 只收确认过的完整 plan |
| `vault:pick-folder` | `VaultService.pickFolder` M12 | same | 10 分钟 token |
| `vault:create` | `VaultService.createVault` M13 | same | — |
| `vault:pick-skill-source` | `VaultService.pickSkillSource` M14 | same | — |
| `vault:import-skills` | `VaultService.importSkills` M15 | same | — |
| `vault:watch` | **删除** | drop | 旧版注册时存在常量但不在 REGISTERED_CHANNELS（ipc.ts:131-193 无此项），监听由打开知识库自动触发，无需单独调用 |
| `vault:changed` | `vault-changed` E01 | same | — |
| `index:search` | `IndexService.search` M24 | same | — |
| `index:status` | `IndexService.status` M25 | same | — |
| `index:rebuild` | `IndexService.rebuild` M26 | same | — |
| `index:changed` | `index-status` E02 | same | — |
| `index:links` | `IndexService.links` M27 | same | — |
| `index:health` | `IndexService.health` M28 | same | — |
| `provenance:list` | `MutationService.provenanceList` M34 | merge | 并入 MutationService（同属 `internal/mutation`） |
| `provenance:state` | `MutationService.provenanceState` M35 | merge | 同上 |
| `agent:list` | `AgentService.list` M38 | same | 懒创建会话 |
| `agent:prompt` | `AgentService.prompt` M39 | change | 旧版 `prompt` 返回 stopReason 且事件走单独广播 `agent:event`；新版把 `agent:event` 整个并入 `Prompt` 的 `Channel[StreamFrame]`——帧随回合走、回合结束 Channel 关，比全局广播更贴合「一次回答」的生命周期，也消除 P-5 的每 token 跨进程 3 跳 |
| `agent:cancel` | `AgentService.cancel` M40 | same | — |
| `agent:set-config` | **删除** | drop | 旧版给 sidecar 的通用 config 透传；新版引擎在 Go 进程内，Agent 配置走 `appConfigPatchAgent` M63（FR-AI-PROV），不再有此入口 |
| `agent:approve` | `AgentService.approve` M41 | same | — |
| `agent:reject` | `AgentService.reject` M42 | same | — |
| `agent:reset` | `AgentService.reset` M43 | same | 新对话时清 4 个会话 |
| `context:snapshot` | `AgentService.contextSnapshot` M45 | same | — |
| `context:changed` | `context-changed` E09 | same | — |
| `context:pin` | `AgentService.contextPin` M46 | same | — |
| `context:unpin` | `AgentService.contextUnpin` M47 | same | — |
| `context:task` | `AgentService.contextSetTaskStatus` M48 | same | — |
| `context:document` | `AgentService.contextReportDocument` M49 | same | — |
| `audit:list` | `MutationService.auditList` M31 | same | — |
| `audit:changed` | `audit-changed` E03 | same | — |
| `rewind:prepare` | `MutationService.rewindPrepare` M32 | same | — |
| `rewind:commit` | `MutationService.rewindCommit` M33 | same | — |
| `settings:get` | `SettingsService.get` M60 | same | — |
| `settings:patch` | `SettingsService.patch` M61 | same | — |
| `settings:changed` | `settings-changed` E04 | same | — |
| `settings:recent-documents` | `SettingsService.recentDocuments` M72 | same | — |
| `settings:note-document-opened` | `SettingsService.noteDocumentOpened` M73 | same | — |
| `conversation:list` | `ConversationService.list` M50 | same | — |
| `conversation:get` | `ConversationService.get` M51 | same | — |
| `conversation:create` | `ConversationService.create` M52 | same | — |
| `conversation:rename` | `ConversationService.rename` M53 | same | — |
| `conversation:pin` | `ConversationService.pin` M54 | same | — |
| `conversation:remove` | `ConversationService.remove` M55 | same | — |
| `conversation:activate` | `ConversationService.activate` M56 | same | — |
| `conversation:search` | `ConversationService.search` M57 | same | — |
| `conversation:export` | `ConversationService.export` M58 | same | — |
| `conversation:clear` | `ConversationService.clear` M59 | same | — |
| `conversation:changed` | `conversation-changed` E07 | same | — |
| `app-config:get` | `SettingsService.appConfigGet` M62 | same | — |
| `app-config:patch-agent` | `SettingsService.appConfigPatchAgent` M63 | same | — |
| `app-config:reset-steering` | `SettingsService.appConfigResetSteering` M64 | same | — |
| `app-config:set-categories` | `SettingsService.appConfigSetCategories` M65 | same | — |
| `app-config:patch-aliases` | `SettingsService.appConfigPatchAliases` M66 | same | — |
| `app-config:providers` | `SettingsService.providers` M67 | same | — |
| `app-config:save-providers` | `SettingsService.saveProviders` M68 | same | — |
| `app-config:set-key` | `SettingsService.setKey` M69 | same | — |
| `app-config:skill-catalog` | `SettingsService.skillCatalog` M71 | same | — |
| `app-config:fetch-models` | `SettingsService.fetchModels` M70 | same | — |
| `app-config:changed` | `app-config-changed` E08 | same | — |
| `theme:changed` | `theme-changed` E05 | same | — |
| `window:state` | `window-state` E06 | same | — |
| `window:command` | `WindowService.minimize/toggleMaximize/close` M01-M03 | split | 一个命令枚举拆成三个方法，类型更直接 |
| `agent:event` | **删除**（并入 C01 Channel） | merge | 见 M39 行；全局广播改为随回合的 Channel |
| 引擎 RPC（`EngineHost` 内部） | **删除** | drop | 旧版引擎是 utilityProcess、经内部 RPC 与 Main 通信；新版引擎在 Go 进程内运行，无此边界（技术设计 §4.6，agnes-engine §13） |
| `vault:read-image` 的 data URI 通道 | `kbimg://` 自定义协议（§5） | change | 见上 |
| （无） | `SettingsService.aboutInfo` M74 | add | 旧版关于页直接读渲染进程内的 store 与构建注入的版本号，没有对应频道 |
| （无） | `SettingsService.copyStoragePath` M75 | add | 旧版复制按钮在渲染进程用剪贴板 API 写 `credentialsPath`；新版不向前端发完整路径，改由宿主写剪贴板（D-013） |

汇总：67 个旧版请求频道中，`same` 58 个、`merge` 3 个（provenance×2 + agent:event）、`split` 2 个（vault:mutate → plan/execute、window:command → 3 个方法）、`drop` 4 个（vault:read-image、vault:watch、agent:set-config、引擎 RPC）、`change` 2 个（agent:prompt 返回值不变但事件并入 Channel、vault:read-image 改 kbimg 协议——该行在表中按「删除 + 协议替代」记入 drop 与 change 各一次）。10 个旧版广播事件全部保留：9 个仍是 Event，`agent:event` 并入 `Prompt` 的 Channel；另新增 `engine-status`（原 sidecar 语义，独立于回合）与 `app-notice`。

## 4. 单接口详细定义

同类接口共用字段表，校验上限以旧版 validate.ts 与 PRD 为准并标来源行号。所有方法的 `vaultId`/`path`/id 类入参先过 §1.4 通用校验，不再重复列出。

### 4.1 VaultService（知识库登记与向导）

#### M07 `list()` — 已登记知识库列表

- 用途：启动时取回全部已登记知识库、活动库、已断开库。
- 前置：无。
- 出参 `VaultListResult`：

| 字段 | 类型 | 含义 | 来源 |
| --- | --- | --- | --- |
| `vaults` | `Vault[]` | 可用知识库，`{id, root, displayName, createdAt, lastOpenedAt}`；`root` 是 canonical realpath 仅供显示 | 旧版 vault.ts:80-87 |
| `activeVaultId` | `string \| null` | 当前打开的知识库 | — |
| `unavailable` | `UnavailableVault[]` | 已断开：`{id, root, displayName, reason}`，reason 只有 `missing`/`no-permission` | 旧版 vault.ts:89-104 |

#### M09 `choose()` — 系统对话框选目录并登记

- 用途：欢迎页「选择目录」；弹 `Dialog.Open(Directory:true)`，选中则登记并打开。
- 出参 `ChooseVaultResult`：`{cancelled: boolean, vault?: Vault, scaffoldedDirs?: string[] \| null}`；`scaffoldedDirs` 为首次登记时自动建的规范目录，`null` 表示登记的是已有知识库（旧版 ipc.ts:467-473）。
- `cancelled=true` 是正常返回，不是错误。

#### M08/M10/M11 `open / forget / retry`

- 入参：`{vaultId}`（§1.4）。
- `forget` 副作用：停止监听、清除该库 revision 与最近文档；对话历史保留（PRD FR-VLT）。
- `retry` 仍打不开时**不是错误**：返回的 `unavailable` 里它仍在（旧版 MileApi 注释，ipc.ts:735）。
- 三者都回 `VaultListResult`。

#### M12/M13 `pickFolder` / `createVault` — 新建向导

- `pickFolder` 出参 `PickFolderResult`：`{cancelled, token?, displayName?, empty: boolean, entryCount: number, willCreate: string[]}`。`token` 10 分钟有效、不透明、前端拿不到绝对路径（PRD FR-VLT；旧版 ipc.ts:476-486）。判断 `empty` 时忽略 `.ds_store`/`thumbs.db`/`desktop.ini`，点号开头项算内容。
- `createVault` 入参：`{token: string, confirmNonEmpty: boolean}`；token 走 `assertPickToken`（validate.ts:759-761）；目录非空且 `confirmNonEmpty=false` → `permission-denied`（旧版语义，validate.ts:763-768）。
- `createVault` 副作用：建 `raws/`、`_archives/`、`projects/`、`tools/`、`AI Knowledge base/`、`dev-constraints/` 及各 README.md；已存在跳过（PRD FR-VLT）。

#### M14/M15 `pickSkillSource` / `importSkills` — 技能导入向导

- `pickSkillSource` 出参 `PickSkillSourceResult`：`{cancelled, token?, candidates: [{name, fileCount}]}`。
- `importSkills` 入参：`{vaultId, token, category, names: string[]}`；`category`/`name` 均为单段名 ≤200 字符（validate.ts:771-793 先做形状校验），真正「单段、不以点开头」判定在 skillImport 业务层（FR-SKILL：≤80 字符）；`names` 一次 ≤200（validate.ts:753）。
- 出参 `ImportSkillsResult`：`{imported: string[], skipped: [{name, reason: 'exists'\|'unknown'\|'too-large'\|'failed'}]}`（ipc.ts:507-520）。
- 上限：单技能包 ≤2000 文件、50 MiB；跳过符号链接与点号项（FR-SKILL）。

### 4.2 DocService（树、读写、图片）

#### M16 `tree(vaultId, path)` — 懒加载一层

- 入参 `path`：相对路径，空串表示根。
- 出参 `TreeResult`：`{vaultId, path, entries: FileEntry[], skillCategoryCount?}`；`FileEntry{vaultId, path, name, kind:'file'\|'directory', area, size, mtimeMs, revision, ignored, childCount?, hash?}`（vault.ts:106-119）。目录在前、组内按名称大小写不敏感排序（FR-TREE）。
- 失败：`not-a-directory` / `path-escape` / `vault-unavailable`。

#### M17 `read(vaultId, path)` — 读文档

- 出参 `ReadResult{doc?: DocumentSnapshot, failure?}`；`DocumentSnapshot{vaultId, path, text, revision, hash, encoding:'utf8', size, eol:'lf'\|'crlf', bom, degraded?}`（vault.ts:123-136）。`text` 恒为 LF；`degraded` 取 `'large-source-only'`（>1 MiB）或 `'large-read-only'`（>2 MiB）。
- 失败：`not-found`/`not-a-file`/`too-large`/`unsupported-encoding`/`path-escape`。

#### M18 `write(vaultId, path, expectedRevision, text)` — 原子保存

- 入参：`text` 恒为 LF，Go 按目标文件原风格还原 BOM/EOL；`expectedRevision` 0 表示调用方认为目标不存在。`text` 字节上限 2 MiB，**超过拒绝不截断**（document.ts:87；validate.ts:298-304）。
- 出参 `WriteOutcome` 判别联合（document.ts:82）：
  - `{status:'written', revision, hash, size}`；
  - `{status:'conflict', diskText, diskRevision, diskHash, diskSize, baseRevision}` —— 对应 `revision-mismatch`，磁盘未被修改，`diskText` 由 Go 回传供合并比较（FR-DOC 三选）。
- 写回是 tmp+rename 原子操作，保留原文件权限（go-service-engineering §5.3）。

#### M19 `saveImage` — 保存粘贴/拖入图片（协议端点，不是绑定方法）

- 形式：`POST http://kbimg.localhost/__upload?vault=<vaultId>`，请求体是图片原始字节；不经 base64、不经绑定方法（D-009）。
- 与读图共用 `kbimg` 协议，但处理器独立，不共用读图分支（§5）。
- 上限 8 MiB（document.ts:97；validate.ts:284-295），超过返回 `too-large`，不截断。
- Go 侧按文件头判类型：png/jpg/gif/webp/bmp/avif；SVG/ICO 拒绝 `invalid-input`（FR-DOC）。
- 出参 `{path, existed}`：`path` 是 `raws/attachments/pasted-<sha256前12位>.<ext>`（FR-DOC；旧版 attachments.ts:49），`existed=true` 表示同内容已存在未重复写盘（ipc.ts:268-272）。
- 保存图片不开事务、不写操作记录（旧版行为，`VaultService.saveImage` L391-425）。

#### M20-M23 树操作 → 统一返回 `PlanResult`

- `rename(vaultId, path, newName)`、`createFile(vaultId, dirPath, name)`、`createDir(vaultId, dirPath, name)`、`planMove(vaultId, sources: string[], targetDir)`。
- **统一说明**：这四个方法都不直接落盘，而是返回 `PlanResult{plan?: MutationPlan, failure?}`——前端拿到 plan 后走「变更计划卡 → 用户确认 → `MutationService.execute`」流程（FR-MUT「所有操作都生成变更计划」）。
- 命名校验（FR-TREE，全部在 Go 侧）：非空 ≤200 字符；禁 `<>:"|?*\/` 与控制字符；不能是 `.`/`..`；不能以点或空格结尾；DOS 保留名按扩展名前基名判定；同级重名按大小写不敏感判冲突（`name-conflict`）；新建文件无扩展名补 `.md`。
- `planMove` 不能拖进自身或子树（FR-TREE），`sources` 每项过 §1.4 路径校验。

### 4.3 IndexService（检索、链接、健康）

#### M24 `search(query)` — 全文检索

- 入参 `SearchQuery{vaultId, text, filters?, limit?, sort?}`（search.ts:29-35）。`text` ≤200 字符**截断不拒绝**（validate.ts:315-318）；`limit` 1–200 默认 50（search.ts:84-85）；`sort ∈ relevance|updated|path`。
- `filters{areas?, extensions?, tags?, updatedAfter?, includeSystem?}`：areas 白名单外静默丢弃；extensions ≤20 条去点转小写；tags ≤20 条各 ≤64 字符转小写；`includeSystem` 只有严格 `true` 才纳入（validate.ts:328-357）。
- 出参 `SearchResult{vaultId, query, hits: SearchHit[], total, partial, tookMs}`；`SearchHit{path, title, area, revision, mtimeMs, size, score, matchedFields, excerpt, tags, ignored}`（search.ts:41-56）。`partial=true` 表示索引未就绪、结果可能不完整（FR-SRCH）。
- `excerpt` 取首个命中前后各 60 字符、找不到留空不伪造；`matchedFields ∈ title|heading|tag|path|body`（FR-SRCH）。

#### M25/M26 `status` / `rebuild`

- 出参 `IndexStatus{vaultId?, phase:'idle'\|'scanning'\|'ready'\|'error', documentCount, tokenCount, scanned, skipped, revision, lastBuiltAt, errorMessage?}`（search.ts:70-81）。`rebuild` 触发后台重建并立即返回当前 status。

#### M27 `links(vaultId, path)` — 链接图

- 出参 `LinkGraphResult{path, outgoing: ResolvedLink[], backlinks: string[]}`；`ResolvedLink{target, text, kind:'wiki'\|'internal'\|'external'\|'other', path: string\|null, candidates: number}`（links.ts:46-65）。`candidates`：1=唯一命中、0=断链、>1=同名多个（未解析，前端显示「有 N 个同名目标」不替用户挑）。反向链接只含已解析的边，排除自引用与系统区（FR-LINK）。

#### M28 `health(vaultId)` — 健康检查

- 出参 `KbHealthReport{vaultId, unresolvedLinks, orphans, duplicates, untitled, indexRevision, partial}`（kbHealth.ts:39-52）。每类 ≤200 条（kbHealth.ts:55）；`partial` 标注索引未完成（FR-HLTH）。

### 4.4 MutationService（变更、事务、日志、来源）

#### M29 `plan(request)` — 生成变更计划

- `origin` 接受 `user` 与 4 个 Agent id；Agent 计划在生成时与执行时各过一次 `internal/policy` 权限门（系统区与点号目录一律 `permission-denied`，`permanentDelete` 另需新鲜批准），用户计划不受该门约束。
- 入参 `PlanRequest{vaultId, origin, actions: MutationAction[]}`；`MutationAction{id, kind, source?, target?, expectedSourceRevision?, expectedTargetRevision?, text?, reason}`（mutation.ts:34-44）。`kind ∈ create|createDirectory|edit|move|copy|archive|trash|permanentDelete`；`origin ∈ user|curator|scout|distiller|general`。
- Go 侧校验（validate.ts:202-276）：`actions` 非空 ≤500（mutation.ts:116）；每条 `id` ≤128、`reason` ≤500、source/target 过路径校验、expectedRevision ≥0 整数；`risk` 由 Go 用 `assessRisk` 计算**不采信前端传的等级**（含 permanentDelete 或 >20 项 → high；含破坏性 → medium）。
- 出参 `PlanResult{plan?: MutationPlan, failure?}`；目标已存在一律拒绝（唯一例外：只改大小写的重命名，`name-conflict`）（FR-MUT）。

#### M30 `execute(plan)` — 执行已确认计划

- 计划的「待确认 / 执行中」状态**是前端状态，不经 API 暴露**（与旧版一致：`mutationStore.ts` 的 `pending`/`running` 只存在渲染进程，`vault:mutate` 调用即执行）。`MutationPlan` 对象本身不带状态字段；PRD §7 状态机的「待确认 → 执行中 → 已完成/部分完成/失败」对应「前端持有 plan 未提交 → `execute` 调用在途 → `MutationReport.status`」。
- 执行中再次提交：前端在 `running` 期间禁用确认按钮（mutationStore `running: boolean`）；Go 侧 `execute` 无计划级互斥锁，重复提交按新计划照常执行——防重复靠前端禁用与 `expectedSourceRevision` 复核（执行前逐条比对磁盘，外部已变的项失败，mutation.ts:6 注释）。
- 执行中关闭窗口：走关窗询问（FR-WIN）；用户确认退出则进程结束，已完成的操作已落盘并有事务记录，未完成项不会执行——恢复走操作日志「恢复」，与部分失败同路径。
- 入参：完整 `MutationPlan`（含 Go 算好的 `risk`；`freshApproval` 仅在用户本次显式确认时为 true）。
- 强制规则：**含 `permanentDelete` 且 `freshApproval != true` → `permission-denied`，连执行都不进入**（validate.ts:263-265）。
- 出参 `MutationReport{planId, status:'completed'\|'partial'\|'failed', completed, failed, remaining, transactionId?, touchedDirectories}`（mutation.ts:84-94）。部分失败标 `partial` 不自动回滚；遇到错误停止，后续项记 `skipped`（FR-MUT）。
- 破坏性操作（edit/move/archive/trash/permanentDelete）开事务，存 `.mile-kb/transactions/<id>/`（FR-MUT；格式见 数据库设计.md）。

#### M31 `auditList(vaultId?, planId?, limit?)`

- `limit` ≤1000（validate.ts:385）。出参 `AuditListResult{records: ActionRecord[], transactions: [{id, planId, createdAt, status, entryCount}]}`（ipc.ts:295-304）。`ActionRecord` 字段见 mutation.ts:62-78。

#### M32/M33 `rewindPrepare` / `rewindCommit`

- `transactionId` 收紧 `[A-Za-z0-9_-]+` ≤128（参与路径拼接，validate.ts:388-394）。
- `rewindPrepare` 出参 `RestorePreview{transactionId, vaultId, entries: [{path, kind, restorable, note?}]}`；`restorable=false` 表示外部又改过（mutation.ts:96-106）。
- `rewindCommit` 出参 `RestoreResult{transactionId, restored, conflicted, touchedDirectories}`（mutation.ts:108-113）；外部修改过的文件标 `conflicted` 不覆盖（FR-MUT）。

#### M34/M35 `provenanceList` / `provenanceState`

- `provenanceList` 出参 `ProvenanceEntryShape[]`：`{copyPath, sourcePath, baseRevision, baseHash, copiedAt, assessment}`（ipc.ts:384-391）；`assessment{state, canSyncFromSource, reason}`，state ∈ `up-to-date|source-changed|copy-modified|both-changed|source-missing|copy-missing|unknown`（provenance.ts:25-47）。只有 `source-changed` 时 `canSyncFromSource=true`。
- `provenanceState` 出参 `{entry: ProvenanceEntryShape | null}`，`null` 表示该文件无来源关系（ipc.ts:379-381）。

### 4.5 RenderService（Markdown 渲染）

#### M36 `renderDocument(vaultId, path, revision)` — 阅读模式整篇渲染

- 出参 `RenderResult{html?: string, links?: LinkTarget[], failure?}`；`html` 是 bluemonday 白名单清理后的安全 HTML（技术设计 §4.5），`links` 是该篇的链接分类（供前端事件委托）。按 `(path, revision)` 缓存；`revision` 与磁盘不符返回 `revision-mismatch` 提示前端先重读。
- `>1 MiB` 拒绝渲染（`too-large`，FR-DOC）；frontmatter 当正文渲染、原始 HTML 转义、标题不生 id（D-006/D-007，与旧版一致）。

#### M37 `renderBlock(text, kind)` — 对话增量块渲染

- 入参：`text` 是当前正在写的块（≤64 000 字符），`kind ∈ 'message'\|'thought'\|'user'`。
- 出参 `BlockResult{html: string}`。Go 按块边界缓存已封口块，只重渲染末尾未封口块（技术设计 §4.5；FR-AI-CHAT「增量渲染」）。末尾未完成的表格/链接由渲染层先收起。

```go
type BlockResult struct {
    HTML string `json:"html"`
}
```

### 4.6 AgentService（4 个 Agent 与共享上下文）

#### M38 `list(agent, vaultId)` — 取会话信息（懒创建）

- 入参 `agent ∈ curator|scout|distiller|general`。
- 出参 `AgentListResult{session: AgentSessionInfo, engine: EngineStatus}`；`AgentSessionInfo{agent, sessionId, vaultId, model, effort, approval, availableModels, availableEfforts, busy}`（agent.ts:97-115）。`availableEfforts` 恒为空数组（自研引擎不声明档位，D-002）→ 前端强度控件据此禁用。
- `EngineStatus`（M38 与 E10 共用同一定义）：`{phase:'starting'\|'ready'\|'offline', message?, capabilities?}`。
  - 枚举沿用旧版 `SidecarPhase`（`starting|ready|offline|restarting`，agent.ts:64）去掉 `restarting`：它是子进程崩溃重启的中间态，新版引擎在 Go 进程内运行、没有进程级重启（技术设计 §4.6），故不保留。
  - `phase='offline'` 时 `message` 必填，如实说明原因，覆盖 PRD §7 的「未配置」与「出错」：未配置供应商（旧版在回合内报错「还没有配置模型服务」nativeProvider.ts:36，新版升级为引擎状态显示）、初始化失败、引擎内部错误。
  - `capabilities`：旧版 `RuntimeCapabilities`（agent.ts:71-86，protocolVersion 等字段）是 sidecar `initialize` 握手协商结果；新版无 RPC 握手，改为固定能力集 `{promptImage:false, promptAudio:false, promptEmbeddedContext:false, extensionSurface:false, steerMethod:null, reloadExtensionsMethod:null}`，表示 vendor 扩展能力均不支持；「关于与诊断」页据此如实展示。
  - 旧版 `restarts` 字段删除：进程内引擎无重启计数。

#### M39 `prompt(agent, vaultId, text, ch)` — 发起回合（Channel 流式）

- 入参：`text` ≤32 000 字符**超长拒绝不截断**（agent.ts:270；validate.ts:148-151）。`ch` 是前端创建的 `Channel<StreamFrame>`（见 §6）。
- Go 侧：用户消息先落历史（Go 记，前端无法伪造，register.ts:853-854）；激活历史对话时的回顾块由 Go 附带、前端传不进（ipc.ts:240-246）。
- 出参 `PromptResult{stopReason: 'end_turn'\|'cancelled'\|'max_turn_requests'\|'max_tokens'\|'refusal'}`（agent.ts:49-56）。Channel 在方法返回时关闭。
- 取消：前端 `channel.close()` 或调用 `cancel(agent)`；取消后剩余工具调用回填「已取消，未执行」（agnes-engine §3）。
- 失败：`invalid-input`/`engine-unavailable`/`too-large`。流式过程中的供应商错误以 `StreamFrame{kind:'error'}` 推送，不走返回值。

#### M40-M43 `cancel / approve / reject / reset`

- `cancel(agent)`：取消该 Agent 当前回合；无回合时静默成功。
- `approve(requestId, optionId)`：`requestId` `[A-Za-z0-9_-]+` ≤256（validate.ts:131-134）；`optionId` 非空 ≤256 不限字符集（透传，validate.ts:144-146）。卡已失效 → `approval-expired`。
- `reject(requestId)`：同上。
- `reset(vaultId)`：关掉 4 个 Agent 的引擎会话（新对话时用）；正在跑的回合以取消结束，单个失败不阻断其余（register.ts:831-845）。

#### M44 `routePreview(text)` — 路由预览

- 出参 `RoutePreviewResult{agent: AgentId, reason: string, matchedTerms: string[]}`；对应「发给 X + 理由」（FR-AI-ROUTE）。词表与优先级 `scout>general>distiller>curator` 原样迁移（agnes-engine §10）。打字只更新预览、不切换归属。

#### M45-M49 共享上下文

- `contextSnapshot(vaultId)` → `SharedContext{snapshotId, sequence, vault, activeDocument?, pinnedReferences, taskBoard, decisionLog, actionLedger, retrievalBundle}`（context.ts:97-118）。
- `contextPin(vaultId, path, note?)`：`note` ≤500 字符，超长**截断不拒绝**（validate.ts:163-167 走 assertString 的默认 onOverflow）；pin 的 id 不透明、不编码路径（register.ts:906-909）。
- `contextSetTaskStatus(taskId, status)`：`status ∈ queued|running|blocked|done|cancelled`（context.ts:21-26）。
- `contextReportDocument(vaultId, path?, revision, dirty, selection?)`：`path` 为 `null` 表示无打开文档；`revision` 必须是非负整数、`dirty` 必须严格布尔，否则 `invalid-input`（register.ts:930-932）；`selection` ≤200 字符，超长截断。这是唯一由前端主动推给 ContextHub 的派生信息（ipc.ts:416-423）。

子结构（字段取自旧版 `shared/context.ts:20-118`）：

```go
type VaultContext struct {
    ID            string `json:"id"`
    RootLabel     string `json:"rootLabel"` // 只放显示名，绝对路径不进上下文
    Revision      int64  `json:"revision"`
    IndexRevision int64  `json:"indexRevision"`
}

type ActiveDocumentContext struct {
    Path      string `json:"path"`
    Revision  int64  `json:"revision"`
    Dirty     bool   `json:"dirty"`
    Selection string `json:"selection,omitempty"` // ≤200 字符
}

type Reference struct {
    ID       string `json:"id"` // 不透明，不编码路径
    Path     string `json:"path"`
    Revision int64  `json:"revision"`
    Note     string `json:"note,omitempty"` // ≤500 字符
    PinnedAt string `json:"pinnedAt"`
}

type TaskItem struct {
    ID          string   `json:"id"`
    Owner       string   `json:"owner"`
    Status      string   `json:"status"` // queued|running|blocked|done|cancelled
    Goal        string   `json:"goal"`
    Inputs      []string `json:"inputs"`
    Constraints []string `json:"constraints"`
    Outputs     []string `json:"outputs"`
    CreatedBy   string   `json:"createdBy"`
    CreatedAt   string   `json:"createdAt"`
    UpdatedAt   string   `json:"updatedAt"`
}

type Decision struct {
    ID        string   `json:"id"`
    Summary   string   `json:"summary"`
    DecidedBy string   `json:"decidedBy"` // agent id 或 "user"
    DecidedAt string   `json:"decidedAt"`
    Paths     []string `json:"paths,omitempty"`
}

type ContextActionSummary struct {
    ID     string `json:"id"`
    Kind   string `json:"kind"`
    Source string `json:"source,omitempty"`
    Target string `json:"target,omitempty"`
    Status string `json:"status"`
    At     string `json:"at"`
}

type RetrievalItem struct {
    ID       string  `json:"id"`
    Path     string  `json:"path"`
    Revision int64   `json:"revision"`
    Excerpt  string  `json:"excerpt"`
    Score    float64 `json:"score"`
    Reason   string  `json:"reason"`
    Area     string  `json:"area"`
    Stale    bool    `json:"stale,omitempty"` // 片段 revision 落后于磁盘
}
```

- `selection` 字段本来就存在于旧版类型里（`shared/context.ts:82`，`contextTail.ts:88-89` 会消费并截到 200 字符），但旧版从未填入：唯一写入方 `ipc/register.ts:922-935` 只收 path/revision/dirty。新版按 PRD FR-AI-CTX 给 `contextReportDocument` **增加可选入参** `selection`，未传时 `activeDocument.selection` 缺省。新增的是入参，不是结构字段。

### 4.7 ConversationService（对话历史）

| 方法 | 关键校验 | 出参要点 | 来源 |
| --- | --- | --- | --- |
| `list` M50 | — | `ConversationIndex{activeId, conversations: ConversationMeta[]}` | conversation.ts:164-169 |
| `get` M51 | `id` 匹配 `^[A-Za-z0-9_-]{1,64}$` | `{meta, events, skipped, truncated}`；一次最多读 5000 事件，`truncated` 标注 | conversation.ts:15, 29；ipc.ts:667-672 |
| `create` M52 | — | `ConversationMeta{id, title, createdAt, updatedAt, turnCount, agents, pinned}` | conversation.ts:152-162 |
| `rename` M53 | title ≤80 字符去控制字符 | 同上 | conversation.ts:229-237 |
| `pin` M54 | `pinned` 严格布尔 | 同上 | — |
| `remove` M55 | — | 无（两步确认在前端） | FR-AI-PERM |
| `activate` M56 | Agent 运行中拒绝 `invalid-input` | 同 `get`；副作用：备好回顾块供下一条 prompt 使用 | register.ts:639-648 |
| `search` M57 | query ≤200 字符 | `{hits: [{conversationId, title, excerpt, matchedAt}]}`；只匹配 user/message，最多 50 条，片段前后各 30 字符 | FR-AI-HIST |
| `export` M58 | — | `{saved, name}`；弹系统保存对话框，`saved=false` 表示用户取消 | register.ts:655-673 |
| `clear` M59 | — | `{removed: number}`；一并删除残留文件 | FR-AI-HIST |

上限：每库 200 对话、单对话 4 MiB（下一轮开新对话）、单事件正文 64 000 字符截断（conversation.ts:17-30）。落盘事件类型 `user|message|thought|tool|turn-end|error`，权限卡与用量不落盘（FR-AI-HIST；agnes-engine §12）。格式见 数据库设计.md。

### 4.8 SettingsService（设置、供应商、key、最近文档）

| 方法 | 关键校验与规则 | 出参 | 来源 |
| --- | --- | --- | --- |
| `get` M60 | — | `{settings: AppSettings, resolvedTheme, platform}` | ipc.ts:522-533 |
| `patch` M61 | `closeBehavior ∈ ask\|tray\|quit`；`theme ∈ light\|dark\|system`；layout 各宽度/高度**夹到边界不拒绝**（sidebarWidth 192–320、docWidth 320–800、composerHeight 72–400）；`lastVaultId` 可为 `null` | 同 `get` | validate.ts:428-503；settings.ts:14-39 |
| `appConfigGet` M62 | — | 扁平 `AppConfigResult{vaultId, aliases, categories, agents}`；`categories` 为按数据库设计 3.2 解析后的数组（非法行丢弃，不原样透传）；`agents` 固定 4 个键。旧版是 `{config: AppConfig{...}}` 包一层，新版从 M1 起就是扁平的 | appConfig.ts:497-508 |
| `appConfigPatchAgent` M63 | patch 只处理提供的键：model ≤128 可空、effort ∈ 5 档、approval ∈ ask\|auto\|yolo、steering ≤4000 **截断不拒绝**、skillPaths ≤500 条各过路径校验（非法路径 `path-escape`） | 同 M62；写盘后回新配置 | validate.ts:513-560 |
| `appConfigResetSteering` M64 | — | 把该 Agent 的 steering 与 `steeringBaseVersion` 恢复为内置默认并写盘，回新配置 | — |
| `appConfigSetCategories` M65 | categories ≤200 条；非法行**丢弃不抛错**；「进检索」变化后由 Go 后台重标记索引并推 `index-status`（E02），不重建 FTS（D-013） | 同上 | validate.ts:567-577；appConfig.ts:438-486 |
| `appConfigPatchAliases` M66 | aliases ≤2000 条；key 是合法相对路径，value `null` 或空串删除；非法 key 静默跳过 | 同上 | validate.ts:579-616 |
| `providers` M67 | — | `{providers: ProviderDraft[], credentials: [{envName, isSet}]}` —— **绝不含 key 值，连掩码都没有**。新版没有凭据文件（key 在 OS 凭据管理器），旧版的 `credentialsPath` 字段删除 | ipc.ts:611-620；appConfig.ts:116-119；D-011 |
| `saveProviders` M68 | providers ≤32（保存路径上限，旧版 `validate.ts:618 MAX_PROVIDERS=32`；读取 providers.json 时另按 `ProviderStore.ts:36` 最多保留 50 条——两个数字都是旧版代码，Q1 已关闭）；每个过 `normalizeProviderDraft`：name 以字母数字开头 ≤64、kind ∈ openai\|anthropic、baseUrl http(s) host 非空 ≤512、models ≤32 各 ≤128 至少 1 个、apiKeyEnv 匹配 `^[A-Za-z_][A-Za-z0-9_]*$` ≤96 可空、contextWindow 正整数可选；名称忽略大小写去重 | 同上 | validate.ts:622-644；appConfig.ts:103-286 |
| `setKey` M69 | envName ≤96 合法变量名；value ≤2560 字节（mygo `MaxSecretSize`）、**不含换行、不截断**（截断=存进永远失败的凭据） | 同 `providers` | validate.ts:646-669；secrets.go:58 |
| `fetchModels` M70 | kind/baseUrl/apiKeyEnv 同 saveProviders 校验；`apiKey` 可选（刚粘贴未保存的 key，≤4096 字节且不含换行，否则 `invalid-input`）；10s 超时、响应 ≤2 MiB、最多 500 条排序返回 | `{models: string[]}` | validate.ts:675-699；FR-AI-PROV |
| `skillCatalog` M71 | — | `{skills: [{path, category, name}]}`；由 Go 读取索引（`skills/…/SKILL.md`，深度 ≥3，分类取第二段，名字优先索引标题、退回目录名，按分类与名字排序）；索引未打开时返回空数组 | ipc.ts:622-629 |
| `recentDocuments` M72 | — | `{vaultId, documents: RecentDocument[]}`；已过滤掉暂时不存在的，但不回写删除 | ipc.ts:697-701；FR-OVW |
| `noteDocumentOpened` M73 | 文件必须真实存在（防塞任意字符串） | 同上 | register.ts:589-593 |
| `aboutInfo` M74 | — | `{version, techStack, engine: EngineStatus, capabilities: EngineCapabilities, restartCount, storage: [{kind, label, display}]}`；`display` 一律脱敏（库根 → `<vault>`、主目录 → `<home>`），不含绝对路径；`kind ∈ vault-root\|vault-settings\|providers\|app-data\|logs\|credentials`；`credentials` 的 `display` 是固定文字「Windows 凭据管理器」 | FR-SET；D-011、D-013 |
| `copyStoragePath` M75 | `kind` 同上；`credentials` 返回 `unsupported` | `{ok: true}`；由 Go 把真实路径写入系统剪贴板（经可替换接口，测试不依赖真实剪贴板） | D-013 |

`patch` 副作用：字段全部校验完再组装，抛错不留半个 patch（validate.ts:428-503）；`theme` 变化时先持久化成功再 `Theme.SetSource` + 推 `theme-changed`（mygo-web-app skill desktop-integration §3）。

`setKey`：value 只在出站请求头出现，不进日志/错误/上下文/界面；`Secrets.Set` 返回 `ErrUnsupported` → `secret-unavailable` 拒绝保存不退回明文（FR-AI-PROV）。

## 5. 自定义图片协议（替代 `vault:read-image`；M19 上传也走这里）

- 协议名（已定，用户 2026-11-18 确认）：`kbimg`。
- URL 形式（D-009 第 6 条，与 M0 实现一致）：Windows WebView2 上页面里写 `http://kbimg.localhost/<库内相对路径>?vault=<vaultId>`；Go 侧统一按 `kbimg://localhost/<路径>` 处理（mygo `protocol.go:204` 的 `schemeHosts` 机制，`internal/schemehost/schemehost.go:5-7`）。不做一次性 token：路径与 vault 参数都只是「请校验」的输入，是否放行由每次请求的校验决定。
- 读图：`GET http://kbimg.localhost/<路径>?vault=<id>`。
- 上传（M19）：`POST http://kbimg.localhost/__upload?vault=<id>`，请求体是图片原始字节；响应 `application/json`，成功 `{path, existed}`，失败 `{failure:{code,message}}`。上传处理器独立实现，不在读图分支上加方法判断。
- 注册：建窗前 `mygo.Protocol.Handle("kbimg", handler)`（protocol.go:53）。
- 授权规则（mygo-web-app skill desktop-integration §7，已核 v0.10.0；读图与上传都必须满足）：
  1. 每次请求重新解析并 `Lstat` 校验路径仍在授权范围内（防渲染后换成目录联接或指向库外的联接点）；
  2. `CallerWindow(r.Context())` 必须非空——只服务本应用窗口，不是本应用页面一律 404（`ipc.go:461-479`）；
  3. 读图只放行 png/jpg/gif/webp/bmp/avif（按文件头），SVG/ICO 一律 403 不回传路径；
  4. 读图单张 ≤8 MiB，超限 403；上传请求体 ≤8 MiB，超限返回 `too-large`；
  5. `Cache-Control: no-store`；`Content-Type` 按魔数判定。
- 上传附加规则：只接受 POST（其他方法 405）；写入位置固定为 `raws/attachments/`，不接受调用方指定路径；文件名由内容哈希决定（M19）。
- 错误响应：越权/非法格式/超限 → `403`（不带路径）；文件不存在 → `404`。不回传任何绝对路径或内部原因。
- 库内相对路径图片在 `RenderService` 输出时即改写成该协议 URL；`data:image/*` 直接显示、https 远程图默认占位点击才加载、http 拦截（FR-DOC，前端/renderer 职责）。
- 前端 CSP 需放行 `img-src` 的 `http://kbimg.localhost` 与 `data:`。

## 6. Channel：引擎流式帧（`StreamFrame`）

`Prompt` 的 `*mygo.Channel[StreamFrame]` 参数（mygo `channel.go:61-82`）。对应旧版 `agent:event` 广播（agent.ts:165-230）+ transcript 事件（conversation.ts:47-60），全部合并为一个判别联合，用 `kind` 字段收窄。**前端用 `for await` 迭代或 `onmessage` 回调**。

```go
// internal/contract
type StreamFrame struct {
    Kind      string `json:"kind"`      // 判别字段，见下表
    Seq       int    `json:"seq"`       // 本回合内单调递增，前端据此丢弃乱序/判重
    Agent     string `json:"agent"`
    SessionID string `json:"sessionId"`

    // 按 kind 选用的字段（判别联合单结构体，TS 侧 switch kind 收窄）
    Text        string         `json:"text,omitempty"`        // delta/thought：增量文本
    ToolCall    *ToolCallFrame `json:"toolCall,omitempty"`    // tool：一次工具调用状态
    Permission  *PermFrame     `json:"permission,omitempty"`  // permission：审批卡
    Status      *StatusFrame   `json:"status,omitempty"`      // status：重试等状态
    Usage       *UsageFrame    `json:"usage,omitempty"`       // usage：用量
    StopReason  string         `json:"stopReason,omitempty"`  // turn-end
    Message     string         `json:"message,omitempty"`     // error：已脱敏
    Finish      bool           `json:"finish,omitempty"`      // 最后一帧标记（见下「帧终结语义」）
}

// PermFrame：审批卡生命周期帧。status 字段是新版显式化的状态机（旧版
// PermissionRequest 无该字段，卡片移除靠前端在 IPC 成功后自行删除——
// agentStore.ts:368-389 的注释说明了这个「先移除会卡死」的坑）。
type PermFrame struct {
    RequestID  string   `json:"requestId"`
    ToolName   string   `json:"toolName"`
    Detail     string   `json:"detail"`
    TargetPath string   `json:"targetPath,omitempty"`
    Preview    *WritePreview `json:"preview,omitempty"`
    Approval   string   `json:"approval"`        // 发卡时 Main 侧登记的执行模式
    Options    []PermissionOption `json:"options"` // 恒为 [允许,拒绝]
    // 状态机：pending（出现）→ approved/rejected（用户已回应，前端收到确认帧后移除卡片）
    //        → expired（回合取消/引擎重启/对话切换，卡片显示「已失效」后由前端移除）
    Status     string   `json:"status"`          // pending | approved | rejected | expired
}
```

```go
// ---- 帧载荷（§6 判别联合的成员） ----

// ToolCallFrame：一次工具调用的状态，同 id 原地更新
// （旧版 AgentToolCall，shared/agent.ts:122-129）。
type ToolCallFrame struct {
    ID     string `json:"id"`
    Title  string `json:"title"`
    Kind   string `json:"kind,omitempty"`
    Status string `json:"status"` // pending | in-progress | completed | failed
    Detail string `json:"detail,omitempty"` // 参数摘要，已截断，不含完整正文
}

// StatusFrame：回合阶段与重试提示（旧版 AgentEvent 'status'，shared/agent.ts:183-193）。
type StatusFrame struct {
    State  string `json:"state"`
    Phase  string `json:"phase,omitempty"`
    Model  string `json:"model,omitempty"`
    Effort string `json:"effort,omitempty"`
    Detail string `json:"detail,omitempty"` // 只含分类与次数，不含响应体或 URL
}

// UsageFrame / Usage：字段名逐字取自旧版 shared/usage.ts:22-50（源头是
// Reasonix internal/acp/status_usage.go:6-29）。「不知道」不等于「零」：
// 费用与命中率缺失时用指针/可空表示，前端显示「—」。
type UsageFrame struct {
    Turn       Usage `json:"turn"`
    Cumulative Usage `json:"cumulative"`
}

type Usage struct {
    TotalTokens      int64    `json:"totalTokens"`
    PromptTokens     int64    `json:"promptTokens"`
    CompletionTokens int64    `json:"completionTokens"`
    ReasoningTokens  int64    `json:"reasoningTokens"` // 推理 token 必须单列
    CacheHitTokens   int64    `json:"cacheHitTokens"`
    CacheMissTokens  int64    `json:"cacheMissTokens"`
    Estimated        bool     `json:"estimated,omitempty"`
    CacheHitRatio    *float64 `json:"cacheHitRatio,omitempty"` // 缺失时由 hit/(hit+miss) 推导
    EstimatedCost    *float64 `json:"estimatedCost,omitempty"` // 缺省 = 未提供，绝不当 0
    Currency         string   `json:"currency,omitempty"`
    CostComplete     *bool    `json:"costComplete,omitempty"` // false = 费用尚未结算完整
    DisplayStatus    string   `json:"displayStatus,omitempty"`
    UsageSource      string   `json:"usageSource,omitempty"`
}

// WritePreview：审批卡的写入预览（旧版 shared/lineDiff.ts:101-122）。三类用
// Kind 区分，避免 TS 侧出现联合类型无法映射到 Go 的问题。
type WritePreview struct {
    Kind         string     `json:"kind"` // created | modified | summary
    Lines        []DiffLine `json:"lines,omitempty"`
    OldLineCount int        `json:"oldLineCount,omitempty"`
    NewLineCount int        `json:"newLineCount,omitempty"`
    OldBytes     int64      `json:"oldBytes"`
    NewBytes     int64      `json:"newBytes"`
    Reason       string     `json:"reason,omitempty"` // too-large | not-text | unchanged
}

type DiffLine struct {
    Kind string `json:"kind"` // same | added | removed
    Text string `json:"text"`
}

// PermissionOption：恒为 [允许, 拒绝]（旧版 shared/agent.ts:131-136）。
type PermissionOption struct {
    ID    string `json:"id"`
    Label string `json:"label"`
}
```

| kind | 载荷字段 | 含义 | 对应旧版 AgentEvent |
| --- | --- | --- | --- |
| `delta` | `text` | 正文增量（已按 ~16ms 合并成帧） | `message` |
| `thought` | `text` | 思考内容增量 | `thought` |
| `tool` | `toolCall{id,title,kind?,status,detail?}` | 工具调用行，status ∈ pending\|in-progress\|completed\|failed；同 id 原地更新 | `tool` |
| `permission` | `permission{requestId,toolName,detail,targetPath?,preview?,approval,options,status}` | 审批卡生命周期帧；status ∈ pending\|approved\|rejected\|expired；options 恒为 [允许,拒绝] | `permission` |
| `status` | `status{state,phase?,model?,effort?,detail?}` | 状态/重试提示（「第 n/m 次重试」） | `status` |
| `usage` | `usage{turn,cumulative}` | 用量收据（字段见 usage.ts:22-50） | `usage` |
| `turn-end` | `stopReason` | 回合正常结束 | `turn-end` |
| `error` | `message` | 回合级错误（已脱敏） | `error` |

规则（来源：PRD FR-AI-CHAT/ENG/PERM、agnes-engine §13、mygo-web-app bindings-and-channels §4）：

- **按帧合并**：Go 把 SSE `delta.content` 累积到一帧（约 16ms 或满 N 字符）再 `Send`，不是每 token 一次跨进程；意义主要是语义（整段交付、思考与正文分帧），R-4 实测 10k 字符下 token 级与合并渲染无差异，但保留以降低消息数。
- **背压**：`Send` 在页面未消费超过 ~1 MiB 时阻塞（channel.go:55-57），页面快于 Go 时不堆积；主线程事件监听里不阻塞。
- **取消**：`ch.close()`（前端）→ Go `Send` 返回 `ErrChannelClosed` 且调用 ctx 被取消 → 回合取消、HTTP 连接断开、剩余工具调用回填「已取消」。关窗/页面导航同效。
- **审批卡以 Go 为准（生命周期，D）**：`permission` 帧 `status=pending` 发卡；前端 `approve/reject` 调用成功后**不移除卡片**，等 Go 回推同一 `requestId` 的 `status=approved|rejected` 帧才移除（FR-AI-PERM 新增约束，与旧版「IPC 成功才移除」同语义但更明确）。回合取消、引擎重启、切换历史对话时，Go 对所有未决卡推 `status=expired` 帧，前端收到后移除；此后 `approve/reject` 该 `requestId` 返回 `approval-expired`。
- **批准凭证的键（E，与旧版一致）**：审批卡按 `requestId` 登记（`EngineHost.writeApprovals` Map 键 = requestId，EngineHost.ts:207-215）；用户点允许后产生两张凭证——`bridge.allowWriteOnce(sessionId, path)` 放行该会话该路径的一次写（写/编辑走桥接层逐路径消费），`mutationApprovals.set(sessionId, {path, expiresAt: now+60s})` 记结构变更批准（EngineHost.ts:221-225, 724-728）。`permanentDelete` 执行前 `consumeFreshMutationApproval(sessionId, path)`：要求 60 秒内、目标路径一致、且发卡时登记的执行模式不是 `yolo` 的消费型批准，用过即删（EngineHost.ts:733-743）。两种口径：`requestId` 键只管「这张卡批没批」；`sessionId+path` 键只管「这次写/这次结构变更放不放行」，二者不能互相替代。
- **帧终结语义（H）**：`Prompt` 返回值（`PromptResult.stopReason`）与 Channel 的 `onclose` 是回合结束信号；`StreamFrame.finish` 只是 Go 标记「这是它主动发送的最后一帧」的提示位，前端不得据 `finish` 判断回合结束——`finish=true` 之后不会再有业务帧，但真正的结束以 Channel 关闭为准（背压/取消路径下可能无 `finish` 帧）。`turn-end` 帧与 `finish` 的关系：`turn-end` 携带 `stopReason` 是语义上的回合结果，`finish` 是通道层面的末帧标记，通常同帧或紧邻出现。
- **顺序与完整性**：值按序到达，全部在 `Prompt` promise settle 之前；方法返回即 Channel 关闭，`onclose` 触发。
- **历史落盘**：只有 `delta/thought/tool(终态)/turn-end/error` 落 `.jsonl`；`permission/usage/status` 不落盘（FR-AI-HIST；agnes-engine §12）。引擎整体状态已移到 E10 `engine-status`，不在本 Channel 内。

## 7. 事件清单（定向 Event）

`mygo.NewEvent[T](name)`，`EmitPage(win.Page(), payload)` 定向发（单窗口也建议 EmitPage，不依赖 Broadcast；mygo-web-app bindings-and-channels §5）。**就绪握手**：页面挂载并订阅完全部事件后调 `WindowService.ready()`，Go 此前产生的通知不入队（mygo 文档确认：DOM 就绪前发出的事件会在就绪后送达，但业务层仍用 ready 握手保证「订阅齐再开始推」，避免竞态）。

| 事件名 | 载荷 | 触发时机 | 就绪前处理 | 前端动作 |
| --- | --- | --- | --- | --- |
| `vault-changed` | `{vaultId, directories: string[], sequence}` | 文件监听合并去抖后（~50ms）按目录聚合推一次 | ready 前不推 | 按 directories 刷新树对应层；外部修改检测见下注 |
| `index-status` | `IndexStatus` | 索引 phase 变化/扫描进度 | 同上 | 更新索引进度与「索引未完成」标注 |
| `audit-changed` | `{vaultId?, sequence}` | 新操作记录落日志 | 同上 | 刷新操作日志面板 |
| `settings-changed` | `AppSettings` | 设置持久化成功 | 同上 | 应用新设置（主题除外，见下） |
| `theme-changed` | `{mode, resolved}` | 主题切换生效 | 同上 | 写 `data-theme`，那一帧关过渡 |
| `window-state` | `{maximized, focused}` | 最大化/焦点变化 | 同上 | 切换最大化按钮图标 |
| `conversation-changed` | `{vaultId}` | 对话历史变化 | 同上 | 刷新历史列表/标题 |
| `app-config-changed` | `{vaultId}` | Agent 经工具改了 AppConfig | 同上 | re-hydrate 设置页数据 |
| `context-changed` | `SharedContext` | 上下文快照变化 | 同上 | 按 snapshotId upsert |
| `engine-status` | `EngineStatus`（`{phase:'starting'\|'ready'\|'offline', message?, capabilities?}`，定义见 M38） | 引擎未配置/就绪/出错切换 | 同上 | 更新引擎状态胶囊与可用性 |
| `app-notice` | `{text, level:'info'\|'warn'\|'error'}` | 不阻断提示（旧版导入失败、监听降级 HintsOnly、设置损坏备份 `.corrupt`） | 同上 | 行内 `role=status` 提示卡（无全局 Toast，PRD §8） |
| `skills-changed` | `{sequence}` | 后台扫描完成或技能操作完成 | 同上 | 刷新浮层两页与角标；进行中的视图不打断 |

`vault-changed` 的外部修改检测链路（F，与旧版等价）：前端收到事件后刷新 `directories` 列出的目录层；随后对每个**已打开标签**，若其父目录在 `directories` 中，则调 `DocService.read` 重读，用返回的 `revision` 与当前 `baseRevision` 比对——干净文档直接跟随磁盘采用新快照，有未保存修改的文档只缓存磁盘版本并在保存时（或立即）提示「磁盘有新版本」（旧版 `vaultStore.ts:340-361` `refreshDirectories` + `documentStore.ts:185-196` `noteExternalChange`）。选择「前端逐个比对」而不是在事件载荷里附受影响文档列表：与旧版行为完全等价（旧版也是广播目录 → 前端判断重读），且不引入「事件载荷必须知道哪些文档打开着」的前端状态反向依赖。

## 8. Mock、联调与验证

- mock 与契约同源：`src/mocks/` 的假 Transport 实现本文件 §2 目录的全部方法，用相同的 `Failure.Code`、判别联合与上限值；**生产构建不引用**（构建后 grep `dist/` 确认，frontend-transport §7）。
- 必须覆盖的错误分支（每个写接口至少）：`invalid-input` 各类（超长、非法字符、非法枚举、超数量）、`path-escape`（`..`、绝对路径、UNC、点号目录、8.3 短名、junction）、`not-found`、`too-large`、`revision-mismatch`/`conflict`、`permission-denied`（系统区禁写、永久删除无 freshApproval、全自动下永久删除）、`vault-unavailable`、`unsupported-encoding`、`engine-unavailable`、`provider-error`（401/429/5xx/超时/半截流）、`approval-expired`、`secret-unavailable`。
- 联调：Go 侧用 `httptest` 假供应商复刻 SSE 场景（agnes-engine §14）；前端单测注入假 Transport 覆盖迟到结果丢弃、stale 保留、流式只更新进行中消息、审批卡按 Go 确认移除（frontend-transport §7）。
- 验证层级：单接口（`go test`）→ 绑定层（契约测试断言 Bind 集合 = 前端调用集合）→ 端到端（`plugins/automation` 真窗口）。`mygo generate` 后 `git diff src/mygo.ts` 必须核对。

## 9. 未决问题

| # | 问题 | 选项/依据 | 结论 | 阻断 |
| --- | --- | --- | --- | --- |
| Q1 | 供应商数量上限 | ~~PRD 写「最多 50 个」；validate.ts:618 是 32~~ 更正（2026-11-18 复核）：两个数字都来自旧版代码——`validate.ts:618 MAX_PROVIDERS=32` 是界面保存路径的上限，`ProviderStore.ts:36 MAX_PROVIDERS=50` 是读取 providers.json 时的截断上限（`list.slice(0,50)`） | 已确认：保存时最多 32 个；读取文件时最多保留 50 个（兼容规则）。M68 行已改 | 已关闭 |
| Q2 | 图片协议名 | `kbimg` / `mile-img` / `kb-image` | 已确认（2026-11-18）：`kbimg` | 已关闭 |
| Q3 | `window-state` 事件是否保留 | 旧版广播 maximized/focused；mygo 窗口状态可由前端经绑定方法查询，但自绘最大化按钮需要实时状态 | 保留事件（旧版行为一致） | 否 |
| Q4 | `sidecar` 帧是否并入 StreamFrame | 旧版 `sidecar` 是独立广播事件（跨回合生命周期）；并入 Channel 会随回合结束而断 | 拆出为 `engine-status` 事件（E10），不并入 Channel——引擎状态与单个回合无关 | 否 |
| Q5 | 审批卡的「失效」如何回传 | `approval-expired` 作为 `approve/reject` 返回值；另外需要回合取消/换档时主动让卡消失 | 已确认：`PermFrame.status` 显式状态机 pending/approved/rejected/expired，失效推 `status=expired` 帧（§6），不另立码 | 已关闭 |

## 10. 范围与状态声明

- 本文覆盖 PRD v1.9 全部 P0 功能对前端暴露的接口面；Agent 知识库工具（8 个 OpenAI function calling 工具）是给模型的接口，不是前端接口，见 `kb-tool-schemas.json` 与 agnes-engine §6。
- `.mile-kb/` 文件格式、SQLite 索引结构、应用数据文件格式不在本文，见 数据库设计.md。
- M0–M2 的接口已实现并通过门禁（见 docs/PLAN-M0/M1/M2.md 的账本）；M74、M75 与 M65 的进检索副作用为「已设计、未实现」，随 M3 的 T02、T07 实现并验证（docs/PLAN-M3.md）。

## 11. R2 增补：主题色（M3.5）

> 状态：用户 2026-11-20 审核通过；已设计、未实现。需求见 PRD 第 16 节（FR-ACCENT），存储格式见数据库设计 4.1.1，首帧方案见技术设计第 14 节。

不新增方法，也不新增事件：只扩展 M60 `get` 与 M61 `patch` 的 `settings`。应用只有一个窗口，`patch` 的返回值就是新设置，前端据此换色，不需要像 `theme-changed`（E05）那样另推事件（E05 是因为系统主题会在应用外变化）。

### 11.1 类型

```ts
type AccentPreset = 'orange' | 'indigo' | 'blue' | 'green' | 'teal' | 'violet';
type AccentTokens = {            // 10 个令牌，DESIGN.md 12.1；值一律 #rrggbb 小写
  accent, hover, active, soft, text, btn, btnHover, btnActive, onAccent, focus: string;
};
type AccentSetting =
  | { preset: AccentPreset }
  | { preset: 'custom'; hex: string; cache?: { light: AccentTokens; dark: AccentTokens } };
```

`AppSettings` 增加 `accent: AccentSetting`；`SettingsPatch` 增加可选 `accent?: AccentSetting`（整体替换，不做字段级合并）。

### 11.2 M61 `patch` 的校验

| 输入 | 处理 |
| --- | --- |
| `preset` 是 6 个预设之一 | 接受；`hex`、`cache` 即使传了也丢弃 |
| `preset = 'custom'` | `hex` 必填，匹配 `^#[0-9a-fA-F]{6}$`，存成小写；不匹配 → `invalid-input` |
| `cache` | 可选，只在 `custom` 下有意义；`light`、`dark` 两组都要有，每组 10 个键都匹配 `^#[0-9a-fA-F]{6}$`；有任何一项不合法 → `invalid-input` |
| 其他 `preset` 值、多余字段 | `invalid-input` |

沿用 §4.8 的规则：所有字段校验完才组装，抛错时不留半个 patch。

`cache` 是前端按 DESIGN.md 12.4 算好的结果。Go 不自己算颜色，只检查格式，原样保存，供下次启动首帧使用（技术设计 14.2）。它只是颜色值，不涉及权限，所以不需要 Go 复算校验。

### 11.3 M60 `get` 的输出

返回规范化后的 `accent`：文件里缺字段、预设名未知、`hex` 不合法时，一律返回 `{preset: 'orange'}`；`cache` 不合法时去掉 `cache`，保留 `preset` 与 `hex`。规则见数据库设计 4.1.1。

### 11.4 验证

- Go 单元测试：上表每一行各一条，再加「非法 patch 不改动已保存的值」；
- 前端：`src/lib/accent.ts` 的 12 套预设快照（DESIGN.md 12.3）与自定义色边界用例；
- e2e：AC-ACC-1、2、5（PRD 16.7）。

## 12. R2 增补：技能管理（M4）

> 状态：用户 2026-11-20 审核通过（随 PLAN-M4 v0.1 批准）。需求见 PRD 第 15 节（M4 范围）；实现见技术设计第 15 节；存储见数据库设计 4.6；界面见 docs/UI设计稿.md 第 9 节与 DESIGN.md 第 11 节。

### 12.1 类型

```ts
type SkillToolStatus = 'detected' | 'leftover' | 'missing';

type SkillTool = {
  id: string;                 // 稳定 id：agon 约定，如 'claude-code'、'deepseek-harness'、'agents'
  name: string;
  iconKey: string;            // 品牌图标文件名；无图标用首字母，通用目录用 'globe'
  installDir?: string;        // 安装目录（脱敏，~ 开头），只用于显示与目标校验的展示
  scanDirs: string[];         // 只扫描目录（脱敏）
  readers: string[];          // 会读通用目录的工具 id（技术设计 13.1）
  status: SkillToolStatus;    // 已检测到 / 未检测到但有目录 / 未检测到
  official: boolean;          // 官方未核实的工具为 false（Trae、ZCode）
  enabled: boolean;           // MiniMax Code 默认 false
};

type SkillOrigin = 'linked' | 'link-broken' | 'copied' | 'independent' | 'builtin' | 'external-link';
type SkillSync = 'in-sync' | 'vault-updated' | 'tool-updated' | 'both-updated' | 'missing';

type KbSkill = {
  name: string; description: string; path: string; category: string;
  distributed?: { toolId: string; mode: 'link' | 'copy'; sync?: SkillSync }[]; // 卡片上的已分发工具
};
type ToolSkill = {
  name: string; description: string; path: string; toolId: string;
  origin: SkillOrigin; sync?: SkillSync;
  target?: string;            // 链接到知识库 / 外部时的目标（脱敏）
  builtin?: boolean;          // 内置技能（不可卸载/覆盖）
};

type SkillScan = {
  cachedAt: string;           // 缓存结果时间；浮层先显示它
  scanning: boolean;          // 后台扫描进行中
  kb: { category: string; skills: KbSkill[] }[];
  tools: { toolId: string; skills: ToolSkill[] }[];
  warnings?: { toolId?: string; text: string }[];   // 扫描降级、目录不可读
};
```

### 12.2 方法

| 方法 | 入参 | 返回 | 语义 |
| --- | --- | --- | --- |
| M76 `registry` | — | `{tools: SkillTool[]}` | 内置注册表加探测结果；不写盘 |
| M77 `scan` | `vaultId`（可为空串） | `SkillScan` | 触发后台扫描；先返回缓存并置 `scanning=true`，完成后发 E12。无活动库时 `kb=[]` |
| M78 `distributePreview` | `vaultId`、`skillPath`、`targets[]`、`mode` | `{items: SkillPreviewItem[]}` | 校验目标、冲突、重复加载、覆盖备份位置；不写盘 |
| M79 `distribute` | 同上 + 每目标的 `allowOverwrite` | `{items: SkillOpItem[]}` | 执行；逐项结果，部分失败不整体回滚 |
| M80 `uninstallPreview` | `toolId`、`path` | `{kind, target?, backupPath?, affectedTools[]}` | `kind = link/copy/independent`；通用目录卸载时 `affectedTools` 列出读者 |
| M81 `uninstall` | `toolId`、`path` | `{items: SkillOpItem[]}` | 链接只删链接；拷贝/独立先移备份；内置拒绝 |
| M82 `adopt` | `vaultId`、`toolId`、`path`、`category`、`linkBack`、`extras[]` | `{vaultPath, linked, backupIds[], warnings[]}` | 复制进知识库；`linkBack` 时原目录备份后换链；`extras` 是同内容的其他工具副本 |
| M83 `restore` | `backupId` | `{restoredTo: string}` | 恢复到原位置；原位置被占用返回 `skill-conflict` |
| M84 `log` | `limit`（默认 50，≤200） | `{records: SkillLogRecord[]}` | 最近操作记录，按时间倒序 |
| M85 `backups` | — | `{items: SkillBackup[]}` | 备份列表（大小、时间、是否可恢复） |
| M86 `cleanBackups` | `ids[]` | `{items: SkillBackup[]}` | 删除选中的备份；对应记录改为不可恢复 |

```ts
type SkillPreviewItem = {
  targetId: string; toolId: string; name: string;
  action: 'create' | 'exists' | 'overwrite' | 'conflict' | 'duplicate' | 'protected';
  targetPath: string;         // 脱敏展示
  backupPath?: string;        // overwrite 时的备份位置
  detail?: string;            // 例如另一处同名技能的位置
};
type SkillOpItem = {
  targetId: string; toolId: string;
  result: 'linked' | 'copied' | 'skipped' | 'failed' | 'removed';
  detail?: string; backupId?: string;
};
type SkillLogRecord = {
  id: string; at: string;
  op: 'distribute' | 'uninstall' | 'overwrite' | 'adopt' | 'restore' | 'clean';
  skill: string; toolId?: string; location: string;
  mode?: 'link' | 'copy' | 'copy-adopt';
  result: 'ok' | 'partial' | 'failed';
  backupId?: string; restorable: boolean; error?: string;
};
type SkillBackup = { id, toolId, originalPath, kind: 'uninstall'|'overwrite'|'adopt', createdAt, size, files, restorable };
type SkillToolRef = { toolId: string; path: string };            // M82 extras：内容相同的其他工具副本
```

### 12.3 校验与错误

- `toolId`：注册表内的 id；`path`：工具目录内的相对路径，1–2 段，单段规则同 FR-SKILL（≤80、不以点开头、无保留名、无 `..`）；`skillPath`：`skills/<分类>/<技能>`，分类与技能各一段；`category`：单段。
- 所有预览与执行都在服务端重新解析路径到登记目录内部（技术设计 15.3）；前端传绝对路径一律 `invalid-input`。
- 目标已存在且未选覆盖：跳过而不是失败（`skipped`，detail 写原因）；选了覆盖但目标是内置：`skill-protected`。
- 名称冲突（知识库内跨分类同名）：预览返回 `conflict` 并带另一处位置；执行返回 `skill-conflict`。
- 恢复时原位置被占用：`skill-conflict`，不覆盖。
- 备份已清理：`backup-missing`。
- 部分失败：执行方法返回成功结果列表，逐项 `failed` 带原因；不抛整体错误（与分发/卸载的多目标任务一致）。

### 12.4 事件与前端动作

E12 `skills-changed` 在扫描完成与每次操作完成后发；前端重新拉 M77（缓存新鲜时不重复扫）并刷新角标。浮层打开时若缓存超过 10 分钟，后台再扫一次。

### 12.5 验证

- Go 单元测试：路径守卫（`..`、绝对路径、UNC、点号、保留名、链接父目录）、同名冲突、联接失败退回拷贝、删链接不跟随（目标哈希不变）、恢复冲突、备份清理后 `restorable=false`、M76 探测注入表。
- e2e：AC-SKM-1–11、19（假用户目录/假 PATH/假注册表，不碰真实工具目录）。
- 未验证：真实工具目录对联接的兼容性（技术设计 13.5 第 3 条），M4 人工项，执行前单独征得用户同意。
