# 绑定、契约与 Channel（Mile Knowledge Base，mygo v0.10.0）

签名已对照 v0.10.0 源码。升级版本后先在源码里确认再用。

## 目录

1. 绑定服务清单
2. 契约类型与错误码
3. 联合类型
4. Channel：引擎流式 token 按帧合并
5. 事件清单与定向发送
6. 取消与迟到结果
7. 常见错误

## 1. 绑定服务清单

签名：`func Bind(services ...any)`（ipc.go:106）、`func CallerWindow(ctx context.Context) *Window`（ipc.go:466）、`func CallerPage(ctx context.Context) *Page`（ipc.go:479）。

`service` 包是系统里唯一 import mygo 的业务层（技术设计 §3）。建议按模块拆服务，全部 `mygo.Bind(...)` 注册：

| 服务（建议） | 方法（建议） | 说明 |
| --- | --- | --- |
| `WindowService` | `Ready()`、`Minimize()`、`ToggleMaximize()`、`Close()`、`PickVaultDir()`、`OpenExternal(url)`、`ShowItemInFolder(path)` | 标题栏按钮、选库对话框、外链/在文件夹显示；`CallerWindow(ctx)` 定位窗口 |
| `VaultService` | `List()`、`Open(id)`、`PickAndOpen()`、`Remove(id)`、`Retry(id)`、`Scaffold(dir, confirmed)`、`Recent()` | 知识库登记/打开/切换/断开/新建向导（FR-VLT） |
| `TreeService` | `ListDir(rel)`、`Rename`、`SetAlias`、`NewFile`、`NewDir`、`Move` | 文件树懒加载与操作（FR-TREE）；操作返回变更计划 |
| `DocService` | `Load(rel)`、`Save(rel, content, revision)`、`Close(rel)` | 文档读写（FR-DOC），revision 冲突走返回值 |
| `SearchService` | `Query(req)`、`Reindex()` | 检索（FR-SRCH） |
| `MutationService` | `Plan(req)`、`Execute(planId)`、`Restore(txId)`、`Audit(limit)` | 变更计划/事务/操作日志（FR-MUT） |
| `EngineService` | `Send(req, ch)`、`Cancel(runId)`、`ListModels(providerId)`、`TestProvider(id)` | AI 引擎流式与供应商（FR-AI-ENG/PROV） |
| `SettingsService` | `Get()`、`Save(draft)`、`SetCloseAction(v)`、`SetTheme(v)`、`SecretStatus(id)`、`SetSecret(id, value)`、`DeleteSecret(id)`、`ImportLegacy()` | 设置与 API key（FR-SET/AI-PROV） |
| `ClipboardService` | `TrashItem(rel)` | 移到回收站走 `Shell.TrashItem` |

规则：

- 方法第一个参数是 `context.Context`（TS 侧不出现）。
- **前端只传相对路径、知识库 id 或一次性 token，不传绝对路径。** 唯一的绝对路径入口是系统目录对话框（`PickVaultDir`）和 Go 自己保存的最近记录。
- 服务结构体不放状态；状态在 `internal` 各包（vault/index/mutation/settings），`service` 只做接线与 `CallerWindow` 定位。
- 改了导出方法、参数结构体、事件后运行 `mygo generate`，`git diff src/mygo.ts` 确认改动。

## 2. 契约类型与错误码

前端可见类型集中在 `internal/contract`。

v0.10.0 的 `mygo-runtime` 里 `CallError` 只有 `name`/`method`/`message`（`packages/runtime/src/index.ts:198`），带不了结构化错误码。所以分两类：

- **可预期的失败**（文件不存在、无权限、超限、编码不支持、越界、重名、revision 冲突、计划被拒、key 不可用）放进返回值，类型安全：

```go
type Failure struct {
    Code   string `json:"code"`   // "not_found"|"permission"|"too_large"|"encoding"|"out_of_scope"|"name_conflict"|"revision_conflict"|"plan_rejected"|"secret_unavailable" ...
    Detail string `json:"detail"` // 给用户看的一句话，已脱敏，不含正文/完整路径/key
    Size   int64  `json:"size,omitempty"`
    Limit  int64  `json:"limit,omitempty"`
}

type LoadResult struct {
    Doc     *DocPayload `json:"doc,omitempty"`
    Failure *Failure    `json:"failure,omitempty"`
}
```

- **意外故障**（程序缺陷、内部状态丢失）才返回 Go `error`，前端收到 `CallError` 显示通用文案并写日志。
- 错误码清单写进 `docs/API.md`（阶段 08），前端只映射清单内的码。
- 大数 ID、时间戳、字节数用 `int64`/`int` 或字符串；时间用 RFC 3339 字符串。

## 3. 联合类型

判别字段 + 可选字段的单结构体，生成的 TS 用 `switch (x.kind)` 收窄。不要用 `any`（生成 `unknown`，字段全丢）。

```go
// 渲染后的链接分类，供前端事件委托（FR-LINK / FR-DOC 图片）
type LinkTarget struct {
    Kind    string `json:"kind"`    // "anchor"|"vault"|"external"|"blocked"
    Anchor  string `json:"anchor,omitempty"`
    Rel     string `json:"rel,omitempty"`     // 库内相对路径
    URL     string `json:"url,omitempty"`     // external
    Reason  string `json:"reason,omitempty"`  // blocked
}
```

## 4. Channel：引擎流式 token 按帧合并

签名：`type Channel[T any]`（channel.go:61），`func (ch *Channel[T]) Send(v T) error`（:69），`func (ch *Channel[T]) Close()`（:82）。关闭后 `Send` 返回 `ErrChannelClosed`；`Send` 在未消费超过阈值时阻塞（背压）。`*mygo.Channel[T]` 只能作为绑定方法参数。

本项目 Channel 的主要用途是**引擎流式输出**（FR-AI-ENG/CHAT，解 P-3/P-5）：

```go
type StreamFrame struct {
    Kind     string `json:"kind"`              // "delta"|"think"|"tool_call"|"usage"|"done"|"error"
    Text     string `json:"text,omitempty"`    // 本帧合并后的正文增量
    Seq      int    `json:"seq"`
    Finish   bool   `json:"finish,omitempty"`
}

func (EngineService) Send(ctx context.Context, req contract.SendReq, ch *mygo.Channel[contract.StreamFrame]) error {
    runID := engine.Start(ctx, req, ch) // 引擎在 Go 进程内运行
    defer engine.Forget(runID)
    return engine.Run(ctx, runID) // 阻塞到回合结束；ctx 取消即断流
}
```

- **按帧合并**：把供应商 SSE 的 `delta.content` 累积到一帧（建议 ~16ms 一帧或满 N 字符）再一次 `Send`，不要每个 token 一次跨进程序列化。思考内容（`reasoning_content`/`<think>`）与正文分帧或分字段，对应 FR-AI-CHAT 的折叠显示。R-4 §3 实测（10k 字符 + 50 条历史）：token 级推送与 16ms 合并在渲染端没有可分辨的帧/延迟差异，Channel 序列化在该规模下不构成瓶颈——**按帧合并的意义主要是语义（按帧交付完整片段、思考与正文分帧），不是性能**；建议保留但不必为性能而做。
- `Send` 返回错误说明页面不再读，立即返回并停掉该回合。
- 工具循环每轮最多 16 步（FR-AI-ENG）；`tool_call`/`usage`/`done`/`error` 作为不同 `Kind` 的帧。
- 一次性结果（读一个文档、跑一次检索）不用 Channel，直接返回。

## 5. 事件清单与定向发送

```go
var FileChanged  = mygo.NewEvent[contract.FileChanged]("file-changed")   // 已打开文档外部变更
var TreeChanged  = mygo.NewEvent[contract.TreeChanged]("tree-changed")   // 目录结构变化
var IndexStatus  = mygo.NewEvent[contract.IndexStatus]("index-status")   // 索引进度/未完成标注
var ApprovalCard = mygo.NewEvent[contract.ApprovalCard]("approval-card") // 审批卡出现/失效
var ThemeChanged = mygo.NewEvent[contract.ThemeChange]("theme-changed")  // 外观切换
var SettingsChanged = mygo.NewEvent[contract.SettingsChange]("settings-changed")
var AppNotice    = mygo.NewEvent[contract.Notice]("app-notice")          // 不阻断的提示
```

- 签名：`func (e *Event[T]) Emit(w *Window, payload T) error`、`EmitPage(p *Page, payload T) error`、`Broadcast(payload T) error`（ipc.go:526/551/565）。
- 单窗口也建议 `EmitPage(win.Page(), ...)` 定向发，不依赖 `Broadcast`；`Emit` 返回错误说明窗口已关，停止发送。
- 事件名用纯字母数字，不以 `mygo:` 开头。

## 6. 取消与迟到结果

- 被动取消：页面导航、`channel.close()`、关窗都会取消绑定方法 ctx；检索、读取、渲染、引擎 HTTP 全部从它派生。
- 停止生成：前端 `channel.close()` 断流即可；需要 Go 侧先落「已中止/不写历史」时用 `runId`：`map[runId]cancel` 登记，另开 `Cancel(runId)` 方法。
- 迟到结果：`Load`/`Query` 请求带前端自增 `requestId`，Go 原样带回；前端只接受最新 `requestId`。Go 侧也可记录最新 `requestId`，旧请求在重活前直接放弃。
- 切换文档时若有未保存修改先确认（FR-DOC）；运行中的回合不允许切换历史对话（FR-AI-HIST）。

## 7. 常见错误

| 现象 | 原因 | 处理 |
| --- | --- | --- |
| 每个 token 都让界面卡 | 每 token 一次 Send / 重新解析整条回答 | Go 侧按帧合并；前端只更新进行中的那条 |
| 生成的 TS 字段是 `unknown` | Go 用了 `any` | 改判别联合结构体 |
| generate 报找不到 mygo 包 | go.mod 缺 require | `go mod tidy` |
| 关窗后流仍在跑/计费 | 用了 `context.Background()` | 从调用 ctx 派生 |
| 快速切换后显示旧文档/旧检索 | 没有 `requestId` 守卫 | 前端丢弃非最新结果 |
| 错误文案里出现完整路径/key | error/Failure 未脱敏 | 在 service 出口统一脱敏（PRD 10.2） |
