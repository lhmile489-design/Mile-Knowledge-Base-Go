---
name: agnes-engine
description: 在 Go 里实现 Mile Knowledge Base 的 Agnes 引擎（4 个 Agent：curator / scout / distiller / general）与权限门，行为对齐旧版 Electron 实现。覆盖 OpenAI 兼容流式协议与 SSE 组装、重试退避、出站地址与 netproxy、16 步工具循环、上下文预算、8 个知识库工具、两道权限判定、60 秒消费型批准、执行模式三档与 D-002 原样复刻、路由词表与优先级、steering 尾部块、共享上下文 2400 字符预算、用量收据「未知不等于零」、对话记录落盘。本项目 Mile-Knowledge-Base-Go 写或改 internal/ 下 AI 引擎、Agent、权限、对话历史相关 Go 代码时必须加载；只改 UI、检索、渲染、知识库文件读写时不加载。
---

# agnes-engine（Mile Knowledge Base Go 版）

用 Go 在应用进程内实现 Agnes 引擎。行为规格以旧版 `E:\WebApp\Mile-Project\Mile-Knowledge-Base-master`（只读）与 `docs/PRD.md` v1.1 为准；两者冲突时停下问用户，不自行裁决。

## 1. 何时加载 / 何时不加载

- 加载：`internal/` 下写 AI 引擎、Agent 会话、工具循环、权限门、路由、steering、共享上下文、对话历史、用量收据、供应商出站请求的 Go 代码；为这些代码写测试。
- 不加载：mygo 窗口 / 绑定 / Channel 装配（用 `mygo-web-app`）；Markdown 渲染管线（用 `markdown-render-pipeline`）；路径规范化与授权（用 `go-service-engineering`）；纯 UI 令牌（用 `design-tokens`）。

## 2. 铁律

1. **旧版目录只读。** 查行为可以读，任何任务都不修改它。
2. **模型输出与工具参数一律不可信。** 逐字段校验；参数不合法把错误文字回给模型，不中断回合（旧版 `kbTools.ts:13-15, 285-296`）。
3. **单回合 panic / 错误不能拖垮应用。** 引擎在 Go 进程内运行（PRD FR-AI-ENG「新增约束」），每个回合 recover 并转成 error 事件；没有独立引擎进程，不实现旧版 EngineHost 的进程重启逻辑。
4. **key 只从 `mygo.Secrets` 取**，只在出站请求头里出现，不进日志、错误消息、上下文、界面（PRD 10.2）。
5. **权限两道判定在 Go 侧执行**，不依赖前端隐藏按钮，不依赖提示词（PRD 第 3 节）。
6. **D-002 原样复刻**：manual(ask) 与 auto 都逐条弹审批卡（区别只在界面文案）；推理强度控件始终禁用（自研引擎不声明 availableEfforts）；4 个 Agent 的 system 提示回调存在但没有接上，差异只来自 steering。这三处不得「顺手修复」。
7. **业务数值引用 PRD 编号**；发现 PRD 与旧版代码不一致时记录并回报，不自行改 PRD。

## 3. 引擎骨架（对应旧版 NativeEngine）

- 4 个 Agent 各一条会话（`Map[AgentID]*Session`），会话互不共享 transcript；`createSession` 同 agent 同 vault 幂等复用，换 vault 才重开（旧版 `NativeEngine.ts:177-199`）。
- 回合流程：用户消息先进历史（尾部块拼接在后面）→ `runLoop` 最多 16 步（`DEFAULT_MAX_STEPS`，旧版 `NativeEngine.ts:70`）→ 每步先 `maintainContext` 再采样 → 只有干净结束的采样才写 assistant 历史（commit boundary，`NativeEngine.ts:350-366`）→ 有 tool_calls 就顺序执行并逐条回填 tool 消息 → 无调用返回 `end_turn`，超限返回 `max_turn_requests`。
- 取消用 `context.Context` 一路贯穿到 HTTP 连接；取消后剩余工具调用各回填「已取消，未执行」（`NativeEngine.ts:374-383`）。**历史里不能留没有配对结果的 tool_calls**，否则下一轮请求被供应商拒绝。
- 带工具调用的 assistant 消息必须总带 `reasoning_content` 键（空串也带）：DeepSeek 思考模式缺这个键返回 400（`NativeEngine.ts:351-352`）。
- 停止原因词表与旧版一致：`end_turn / cancelled / max_turn_requests / max_tokens / refusal`（`shared/agent.ts:49-56`）。

## 4. OpenAI 兼容流式协议与 SSE 组装

协议：`POST {baseUrl}/chat/completions`，body 含 `stream: true` 与 `stream_options.include_usage`（`openaiClient.ts:105-112`）。细节（逐行规则、think 拆分、tool_calls 累积、usage 归一）见 `references/protocol-and-retry.md`。

要点：

- 只认 `data:` 行；`[DONE]` 或合法 `finish_reason` 才算完整，半截的流报 `StreamIncompleteError`（`streamAssembler.ts:111-114, 183-186`）。
- `delta.content` 是正文；`delta.reasoning_content` 优先、其次 `reasoning`；正文开头的 `<think>…</think>` 也拆成思考（`thinkSplitter.ts` 状态机：probe → inside → passthrough，只在回合最开头认标签）。
- `tool_calls` 按 index 累积 name/arguments；name 一出现就发 `tool-call-start`；没有 id 补 `call_<index>`；完整调用在流结束时按 index 顺序发出（`streamAssembler.ts:160-178, 189-204`）。
- 流式中 300 秒没收到任何字节就中止（`DEFAULT_IDLE_TIMEOUT_MS`，`openaiClient.ts:22, 116-122`）。
- usage 归一：hit 缺省用 `prompt_tokens_details.cached_tokens`，miss 缺省用 `prompt − hit` 推算（`streamAssembler.ts:63-79`）。

## 5. 重试、超时与出站

只重试拿到响应头之前；开始流式后中途断开不重放（`retry.ts:12-13`）。

- 可重试：408、429、5xx；401/403 立即报鉴权错误（`ProviderAuthError`）；其余 4xx 立即失败。
- `MAX_RETRIES = 10`；退避 `500ms × 2^(n−1)`，上限 15s，加 0–250ms 抖动；`Retry-After`（秒数或 HTTP-date）优先，上限 60s（`retry.ts:15-18, 42-61`）。
- 重试时发 status 事件：「第 n/m 次重试，x 秒后自动继续」，不带响应体或 URL（`NativeEngine.ts:298-307`）。
- 出站地址：https 任意；http 只允许回环（localhost、*.localhost、127.0.0.0/8、::1）；URL 不能带用户名密码（`endpoint.ts:21-42`）。
- 代理：用 `github.com/egoist/mygo/netproxy` 的 `netproxy.Transport()` 作 `http.Client.Transport`。它自带回环直连、环境变量优先、Windows 走 WinHTTP（含 PAC/WPAD）、5 秒系统答案缓存、SOCKS5 支持（`netproxy.go:1-34, 66-88, 137-144`）。这与 PRD FR-AI-ENG「跟随系统代理、回环直连」一致；旧版「SOCKS 回落直连」在 mygo 下自然变成 SOCKS5 可用，属改善不属偏差。
- 错误消息分类：`StreamStalledError`（供应商 300 秒没有返回任何数据）、`StreamIncompleteError`（响应在完成前就结束了）、`ProviderStreamError`（流式途中服务端报错）。

## 6. 工具循环与 8 个工具

工具定义（OpenAI function calling，中文描述）原样迁移，见 `references/kb-tool-schemas.json`（从旧版 `kbTools.ts:86-234` 逐字迁移）。规则：

- 改写类工具只有 `write_note / edit_note / mutate_notes` 需要审批（`kbTools.ts:241-243`）。
- 单个工具结果回给模型截断到 20 000 字符（`TOOL_RESULT_MAX_CHARS`，`kbTools.ts:28`），超长加「…（内容过长，已截断，共 N 字符）」。
- `read_note` 分段：单次最多 20 000 字符；分段输出带「［path：第 a-b 个字符，共 N 个，还有更多 offset=…］」；offset 越界明说「offset N 已超出笔记末尾，该笔记共 M 个字符」（`kbTools.ts:454-473`）。
- `search_notes`：query 非空，limit 默认 8、最大 20（`kbTools.ts:29-30, 412-415`）。
- `edit_note`：old_text 必须恰好出现 1 次；0 次 / 多次 / 空 / 与 new_text 相同各有固定报错文案（`VaultFsBridge.ts:245-259`）。
- `mutate_notes` kind 白名单：`createDirectory / move / copy / archive / trash / permanentDelete`；走变更事务，复用 MutationService 而不是自己写文件（`kbTools.ts:245-252, 523-532`）。
- `handoff_task`：owner 必须是已知 Agent 且不是自己；goal ≤4000 字符；inputs、constraints 各最多 50 条、每条 ≤4000（`EngineHost.ts:917-918, 882-895`）。**发起者由 sessionId 反查，模型参数里自称哪个 Agent 都不算**。
- `set_aliases`：aliases 对象，value 为 null 表示删除；最多 2000 条；路径形态校验（`EngineHost.ts:896-913`）。
- 工具异常文案按 `errorText` 分类：ENOENT「文件或目录不存在」、EACCES/EPERM「没有访问权限」、AbortError「操作超时，已中止」等，不透传 Node 底层错误（`kbTools.ts:380-400`）。Go 里对应 `errors.Is`/`errors.As` 分类。

## 7. 权限门（两道判定 + 系统区禁写）

两道判定缺一不可（`VaultFsBridge.ts:133-179`）：

1. 拼写判定（`canAgentWrite` / `canAgentMutate`）：在任何磁盘访问之前做，拒绝时连 stat 都不碰，否则越权 Agent 能靠报错形状探测路径存在。
2. 真实路径判定：按 realpath 反推的规范路径再判一次，挡 8.3 短名、目录联接、符号链接（`VaultFsBridge.ts:161-172`）。同一张批准覆盖规范路径。

系统区禁写（`GLOBAL_WRITE_DENY_PREFIXES`，`agentPolicy.ts:25-44`，任何模式都不得通过）：

- `.mile-kb/config.json`、`.mile-kb/transactions/`、`.mile-kb/index/`、`.mile-kb/provenance.json`、`.mile-kb/settings/`、`.mile-kb/drafts/`；
- 所有点号目录（`area === 'system'` 永不通过白名单）；
- 路径归一：拒绝绝对路径、UNC、空串、任何 `.`/`..` 片段（`agentPolicy.ts:116-129`）。

`permanentDelete` 需要**新鲜审批**：一张 Main 记下的、发卡时不是 yolo、目标一致、60 秒内的消费型批准，用过即焚（`agentPolicy.ts:252-255`，`EngineHost.ts:221-225, 735-741, 815-827`）。yolo 下引擎不弹卡，所以 yolo 下 permanentDelete 必然拿不到批准——这是设计，不是缺陷（PRD AC-AI-2）。

## 8. 执行模式、审批卡与 60 秒批准

- 三档：manual(ask)、auto(auto)、full(yolo)；映射只在 `EXECUTION_MODE_TO_APPROVAL` 一处（`executionMode.ts:19-34`）。配置值不合法降级 manual（`normalizeExecutionMode`，`executionMode.ts:79-81`）。
- manual 与 auto 都逐条弹审批卡（D-002）；只有 full 不弹。
- 审批卡只有两个选项：「允许」`allow_once` /「拒绝」`reject_once`，没有「本会话始终允许」（`NativeEngine.ts:534-538`）。
- 卡记录发出时的执行模式，之后换档不影响已发出的卡（`NativeEngine.ts:509-511`；`EngineHost.ts:707-714` 以 Main 侧登记为准，引擎自称不算）。
- **审批卡状态以 Go 侧为准**：界面先发回应、Go 收到确认后才移除卡片，避免回合永远停在「进行中」（PRD FR-AI-PERM 新增约束）。
- `allowWriteOnce` 批准：同一会话、同一路径、60 秒内有效、用过即焚（`GRANT_TTL_MS = 60_000`，`VaultFsBridge.ts:57, 72-95`）。批准不在这里消耗，落盘前才 `consume()`：edit 因 old_text 不匹配失败时不该消耗批准（`VaultFsBridge.ts:126-132, 210`）。
- 一定会被拒的写不弹审批卡（`precheckWrite` 提前判），免得用户点「允许」也写不进去（`NativeEngine.ts:461-467`）。
- write/edit 审批卡带行级 diff 预览：旧文由 Go 自己读盘（不是模型声称的），新内容截到 64 KiB；LCS 上限 4 000 000 单元，预览最多 60 行、单行 200 字符，超限整体降级为摘要不给半张 diff（`lineDiff.ts:16, 86-89, 125-172`；`EngineHost.ts:648-672`）。
- 两步确认（删除历史、清空、删供应商）：第二次点击距第一次 ≥600ms 且 detail ≤1，确认态 4 秒复位（`confirmGate.ts:12-22`）。

## 9. 上下文预算

- 估算 token：ASCII / 4 + 非 ASCII × 0.7 + 每条消息 4（`contextBudget.ts:52-76`）。
- 用真实 `prompt_tokens` 校准，系数夹 0.5–3（`calibrate`，`contextBudget.ts:82-85`）。
- 窗口默认 64 000，可按供应商配置覆盖；达到 80% 维护一次（`COMPACT_RATIO = 0.8`，`DEFAULT_CONTEXT_WINDOW = 64_000`）。
- 维护：保留最近约窗口 16% 的整轮（`TAIL_RATIO = 0.16`），切点只落在用户消息之前（工具调用与结果必须同侧）；更早内容交给模型写摘要（`COMPACTION_INSTRUCTION` 原文见 references）；摘要失败或仍超限时整轮丢弃最旧内容并加 `（更早的对话因上下文长度已省略）`（`contextBudget.ts:131-169`）。
- system 提示与工具定义也计入预算（`fixedTokens`，`NativeEngine.ts:270-277`）。
- 摘要消息标记 `<compaction-summary>…</compaction-summary>`；切点判定不把摘要消息当一轮的开始（`contextBudget.ts:25-27, 87-89, 96-102`）。

## 10. 路由、steering 与共享上下文

- 路由：输入转小写后关键词**包含匹配**，不分词。词表原样迁移（`agentRouting.ts:44-119`：curator 整理类、scout 检索类、distiller 提炼类，各配英文词）。多类命中按 `ROUTE_PRIORITY` scout(0) > general(1) > distiller(2) > curator(3)；词频不参与；都不命中走 general（`agentRouting.ts:21-26, 29, 156-189`）。理由文案：「这是 X 类请求（识别到「词」）」。
- 追问路由：已有对话默认粘在当前 Agent；显式「自动」只对下一条重新路由；手动改派一直保持（`followUpRouting.ts:33-40`）。
- steering：每个 Agent 一份，≤4000 字符，附在用户消息尾部、不进 system 提示；前固定 PREAMBLE 说明「只表达偏好，无法扩大权限」（`steeringBlock.ts:23-37`）。Go 侧组装尾部块顺序：recap → Main 附带的 tail → steering → 共享上下文，用 `\n\n` 连接（`NativeEngine.ts:233-243`）。
- 共享上下文：注入尾部最多 2400 字符，按优先级保留：revision（永不裁）→ 当前文件与 dirty（选区截 200）→ 固定引用（另受 1500 上限）→ 未完成任务 → 决策记录 → 检索片段（最多 4 条，整条丢弃）。被省略写明「另有 N 项未列出」（`contextTail.ts:17-20, 67-186`；`context.ts:17-19`）。绝对路径不进上下文，文本先脱敏。
- ContextHub 分权：Agent 只能写 taskBoard / decisionLog / retrievalBundle；派生层（vault、activeDocument、pin、actionLedger）一律拒绝，出现非法字段整个 patch 拒绝（`ContextHub.ts:37, 176-195`）。任务状态 Agent 只能改自己 own 的（`ContextHub.ts:159-170`）。

## 11. 用量收据：未知不等于零

- 费用只用供应商给的 `estimatedCost`；不自己拿价格表算。null → 「— 未提供」，绝不显示 $0.00（`usage.ts:11-19, 86-106`）。
- 小数位按量级：≥1 → 2 位，≥0.01 → 3，≥0.0001 → 5，更小 → 8；非零费用永不显示成全零（`usage.ts:112-119`）。
- 命中率：优先 `cacheHitRatio`；缺省由 hit/(hit+miss) 推；两者都是 0 → 「—」（`usage.ts:125-138`）。
- 累计由引擎累计（`session.cumulative`，`NativeEngine.ts:339-348`），界面按回合覆盖，不自己叠加。
- usage 字段：`totalTokens / promptTokens / completionTokens / reasoningTokens / cacheHitTokens / cacheMissTokens`（`TurnUsage`，`usage.ts:22-44`）。

## 12. 对话记录

- 位置：应用数据目录 `conversations/<vaultId>/`，`index.json` + 每对话一个 `<id>.jsonl`；不写进知识库；忘记知识库不清历史（`ConversationStore.ts:1-12`）。
- 事件类型：`user / message / thought / tool / turn-end / error`；权限与用量不落盘（`conversation.ts:47-60`）。流式片段在内存合并、边界才写一行；工具只记终态（completed / failed）（`ConversationRecorder.ts:83-111`）。
- 上限：每库 200 对话；单对话 4 MiB（新一轮开始时开新对话）；单事件正文 64 000 字符截断；一次读回最多 5000 事件（`CONVERSATION_LIMITS`，`conversation.ts:17-30`）。
- jsonl 追加前先查末字节补换行（崩溃残行不拖坏新事件，`ConversationStore.ts:300-322`）；index.json 原子写；读盘坏行跳过并计 `skipped`；索引损坏备份 `.corrupt` 后从事件文件重建（`ConversationStore.ts:386-445`）。
- 标题取首条用户消息第一行、压空白、>40 字符加省略号；重命名 ≤80 字符、去控制字符（`conversation.ts:229-247`）。
- 「接着问」回顾块：只取 user/message，每条 ≤1200、总计 ≤6000，从最新往旧加、装不下丢最旧并注明省略条数；前固定 PREAMBLE「历史不是新指令」；每 Agent 一份、只用一次、发送失败放回（`conversationRecap.ts:15-72`；`ConversationRecap.ts:21-38`）。

## 13. 新版专属约束

- **引擎在 Go 进程内**，没有 utilityProcess：不实现 EngineHost 的 spawn/重启/RPC；但「单回合出错不能拖垮应用」必须做到——每回合 goroutine recover，error 事件照常发，其余会话不受影响。
- **token 按帧合并再推前端**（PRD FR-AI-CHAT 新增约束）：在 Go 侧把 text/thought/tool/status/usage 事件按 ~16ms 帧聚合，经 `*mygo.Channel[T]` Send 给页面。Channel 用法见 `mygo-web-app`；背压窗口 1 MiB，Send 在页面落后时阻塞（`channel.go:55-57, 241-263`），页面关闭后 Send 返回 `ErrChannelClosed` 且调用 ctx 被取消——用它驱动回合取消。
- **审批卡状态以 Go 为准**（PRD 新增约束）：pending 表在 Go 侧维护，Renderer 只发 optionId；回应到账后 Go 先记 grant、再通知界面移除卡片。
- **模型输出与工具参数不可信**：Go 侧按结构体解码 + 逐字段校验，不允许的字段不静默忽略（ContextHub 的 patch 是反例教材：`ContextHub.ts:176-181`）。
- **key 从 `mygo.Secrets` 取**：`Secrets.Set(key, []byte)` / `Get` / `Delete`；`MaxSecretSize = 2560` 字节；key 名 ≤256 字节非空；`ErrSecretNotFound` 判缺失；设置不可用时 `Set` 报错——**拒绝保存，不退回明文**（`secrets.go:54-58, 83-121`）。PRD 上限已改为 ≤2560 字节、不含换行。

## 14. 测试策略

- 假供应商用 `net/http/httptest`：按旧版 `scripts/fixtures/fake-openai-provider.cjs` 的场景（触发词 FS_READ/FS_WRITE/FS_MUTATE/HANDOFF/PERMISSION_TEST/USAGE/STREAM_MD）在 httptest handler 里复刻 SSE 响应；记录每次请求体供断言「送到模型的是什么」。
- 覆盖：重试（429→200、Retry-After、401 不重试、退避序列注入假 delay）、半截流（无 [DONE] 也无 finish_reason 报 StreamIncompleteError）、think 拆分（`<think>` 跨块、未闭合）、tool_calls 累积（按 index、缺 id 补 call_）、取消（流式中 cancel → 剩余调用回填「已取消」）、权限拒绝（系统区路径写被拒且不碰盘）、permanentDelete 新鲜审批。
- 出站请求计数：包装 `http.RoundTripper` 记录调用次数，断言重试次数与代理路径（做法参照 `go-service-engineering` 的 RoundTripper 计数法）。
- **最值得移植的旧版单测**（`tests/unit/`）：`openaiClient.test.ts`、`nativeEngine.test.ts`、`contextBudget.test.ts`、`contextTail.test.ts`、`steeringBlock.test.ts`、`agentRouting.test.ts`、`agentPolicy.test.ts`、`permissionRouter.test.ts`、`pathAliasBypass.test.ts`、`confirmGate.test.ts`、`conversationRecorder.test.ts`、`conversationStore.test.ts`、`usageFormat.test.ts`、`kbToolsError.test.ts`、`lineDiff.test.ts`、`engineHost.test.ts`（只取权限/审批断言部分）。

## 15. 交付前自检

- [ ] 所有写操作过了两道权限判定（拼写 → realpath），且第一道在任何磁盘访问之前。
- [ ] 系统区禁写清单与 `GLOBAL_WRITE_DENY_PREFIXES` + `.mile-kb/drafts/` + 点号目录一致；permanentDelete 需要新鲜审批。
- [ ] 重试只覆盖响应头之前；408/429/5xx、退避 500×2^(n−1) ≤15s +0-250ms、Retry-After ≤60s、MAX_RETRIES=10；300s 空闲中止。
- [ ] 半截流报错；[DONE]/finish_reason 才算完成；assistant 带 tool_calls 时总带 reasoning_content。
- [ ] 工具循环 ≤16 步；取消后剩余调用回填「已取消，未执行」；每个 tool_call 都有配对 tool 消息。
- [ ] 上下文预算：估算公式、80%/16% 阈值、校准夹 0.5–3、切点只在用户消息前、fixedTokens 计入。
- [ ] 路由词表与优先级 scout>general>distiller>curator 原样；steering 尾部块带 PREAMBLE；共享上下文 ≤2400 字符按 6 级优先级。
- [ ] 用量：费用只取供应商值、未知显示「— 未提供」、命中率 0/0 显示「—」、累计由引擎累计。
- [ ] 对话记录：5 种事件、流式合并、工具只记终态、上限 200/4MiB/64000/5000、坏行跳过、残行补换行。
- [ ] manual/auto 都弹审批卡、强度控件禁用、system 提示不注入（D-002）；审批卡只有允许/拒绝、记录发卡时模式、60 秒消费型批准。
- [ ] key 只经 `mygo.Secrets`、不出现在日志/错误/上下文/界面；出站 https 任意、http 只回环、无 userinfo。
- [ ] 引擎代码不 import mygo（除 Secrets 与 Channel 类型）；单回合 recover；token 按帧合并经 Channel 推送。
