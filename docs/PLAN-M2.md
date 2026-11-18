# M2 实施计划：AI 引擎、Agent、对话与设置（模型服务、Agents）

> 状态：草案 v0.3（2026-11-18，已按独立审查与复审修订），待用户审批。审批前不执行第 4 节的任何任务。
> 本次目标：交付技术设计第 9 节能力 C6（AI 引擎与权限）、C7（对话、历史、用量）与设置页的「模型服务」「Agents」两节，形成闭环：配置 DeepSeek 并填 key → 发消息 → 自动路由 → 流式回答 → 工具调用弹审批 → 用量收据 → 重启恢复对话 → 历史搜索。
> 上一里程碑：docs/PLAN-M1.md（已完成并推送，`cdc7a21`）。

## 1. 目标、范围与授权

### 1.1 交付的可观察结果

1. 设置页「模型服务」可新增/编辑供应商（最多 32 个保存，读取保留 50 个），支持 DeepSeek 预设、拉取模型目录、配置 key；key 存系统加密，界面只显示「已配置 / 未配置」。
2. 设置页「Agents」可配置 4 个 Agent 的模型、推理强度、执行模式、可用 Skills 与 steering（草稿 + 保存条 + 与默认对比 + 恢复默认）。
3. 输入框发消息前显示「发给 X」及理由；手动改派保持到取消；「自动」只对下一条生效。
4. OpenAI 兼容流式回答：正文与思考分开、按帧合并推送、重试状态可见、300 秒空闲中止、可取消。
5. 8 个知识库工具可用；写操作与结构操作按执行模式弹审批卡（含行差异预览）；系统区任何模式都禁写；永久删除按 D-005 直接拒绝。
6. 回合结束后显示用量收据（本轮 / 累计）；供应商未给费用时显示「— 未提供」。
7. 对话历史按 `conversations/<vaultId>/index.json + <id>.jsonl` 落盘；重启恢复上次对话；「接着问」生成一次性回顾块；历史搜索命中用户消息与回答正文。
8. 会话信息浮层显示引擎状态、模型与诊断、读写边界、4 个 Agent 的权限摘要、共享上下文。
9. 未配置或引擎出错时如实显示原因，编辑器和检索不受影响。

### 1.2 关联需求

FR-AI-ENG、FR-AI-AGT、FR-AI-ROUTE、FR-AI-PERM、FR-AI-CHAT、FR-AI-HIST、FR-AI-PROV（模型服务与 key）、FR-AI-USAGE、FR-AI-STEER、FR-AI-CTX；FR-SRCH / FR-LINK / FR-MUT / FR-AREA（工具与上下文的读侧复用）；NFR-PERF（流式 60fps、输入延迟、历史查看器、消息虚拟化）。

### 1.3 明确不做（M2 非目标）

| 不做 | 原因 / 去向 |
| --- | --- |
| 设置页「常规」「分类与检索」「关于与诊断」 | M3。诊断所需数据（引擎状态、能力、key 存放说明）M2 已具备，浮层先用 |
| 技能导入向导（M14、M15）、总览磁贴、健康入口 | M3 |
| P1 修复三项：手动/自动差异、强度控件可用、system 提示接入 | D-002 原样复刻；修复需单独提案 |
| anthropic 类型供应商的 chat/completions 调用 | PRD：只保存不调用；模型目录拉取按旧版 `modelCatalog.ts:55-59` 保留（GET /v1/models + x-api-key + anthropic-version），T01 实现 |
| 旧版 safeStorage 加密的 key 解密 | Q2：读不出来，用户重填 |
| 旧版对话历史、providers.json 之外的旧数据导入 | Q2/D-003：不导入对话历史 |
| 结构化输出、图片/音频输入、vendor 扩展 | 旧版 capability 全为 false，原样保留 |

### 1.4 审批记录

见第 8 节。

## 2. 前置资料与现状

| 输入 | 版本 / 实际情况 | 缺口 | 是否阻断 |
| --- | --- | --- | --- |
| PRD | v1.6（含 FR-PROV 同步新增） | 无 | 否 |
| API.md | v1.0；M38–M49、M63、M64、M67–M71 已定义；§6 StreamFrame 主结构已定义 | **§6 引用的 ToolCallFrame / StatusFrame / UsageFrame / WritePreview / PermissionOption 没有定义**；M45–M49 的详细字段需在 T00 补齐 | 阻断 T03，T00 先补 |
| 数据库设计 | v1.0；3.1 agents.json、3.6 providers.json、4.4 对话历史与 search.db 已定稿；§6-8 有并发与恢复纪律 | 无 | 否 |
| M1 代码 | index（M24/M25/M27）、mutation（M29/M30/M31–M33）、vault 别名、render（M36）、service 绑定 | **M49 只有关窗用的弱化版**：M0 的 `WindowService.ReportDocument` 仅记录 dirty；`AgentService.ContextReportDocument` 与 ContextHub 均未实现 | 否，T08 处理 |
| mygo v0.10.0 能力 | 选型已在技术设计 2.1 确认：`mygo/netproxy`（WinHTTP，含 PAC / WPAD，回环直连，SOCKS5 可用）与 `mygo.Secrets`（Windows 凭据管理器，`MaxSecretSize`=2560，`secrets.go:58`）；Channel、Event、Clipboard 已在 M0/M1 使用 | 两者都没在本机实际调用过；Secrets 写的是真实凭据管理器，测试必须隔离（见 T01） | 否；T00 做本机冒烟验证 |
| 旧版实现 | 盘点见 2.1（只读参考） | — | 否 |
| Skills | agnes-engine（4 份 references，含工具 schema）、mygo-web-app、design-tokens、react-best-practices、kb-index-sqlite | 实际调用情况未验证 | 否 |
| D-002 / D-005 | 已确认 | 验收要按旧版行为判定 | 否 |

### 2.1 旧版实现盘点（只读；路径相对 `Mile-Knowledge-Base-master/`）

引擎与宿主（`src/main/`）：

| 能力 | 文件（行数） | 关键位置 |
| --- | --- | --- |
| 引擎接口 | `agent/AgentEngine.ts`（53） | initialize/createSession/prompt/cancel/respondToPermission/rejectPermission/setConfigOption/closeSession/ownerOf/onEvent/dispose |
| 宿主与审批 | `engine/EngineHost.ts`（928） | 审批登记 EngineHost.ts:679-715；差异预览 607-673；永久删除兑现 816-827；交接发起者反查 754-764 |
| 工具循环 | `engine/NativeEngine.ts`（631） | 步数上限 16（`DEFAULT_MAX_STEPS`）；工具前后检查取消 282/376；yolo 不弹卡 468-477；压缩 398-433 |
| OpenAI 客户端 | `engine/openaiClient.ts`（200） | 空闲超时 22；取消 100-103；不完整流 191 |
| SSE 组装 | `engine/streamAssembler.ts`（205） | 组装 107-204；usage 归一 63-79 |
| think 拆分 | `engine/thinkSplitter.ts`（81） | push/flush |
| 重试 | `engine/retry.ts`（146） | 15/38-40/42-50/53-61/138-140 |
| 出站与代理 | `engine/endpoint.ts`（42）、`engine/proxy.ts`（103） | 仅 chat/completions；ProxyAgent |
| provider 选择 | `engine/nativeProvider.ts`（51） | 会话选 → 保存 → 第一个 openai |
| 工具实现 | `engine/kbTools.ts`（576） | schema 86-234；执行 402-576；截断 20 000；`precheckWrite` 313-326 |
| 权限纯规则 | `shared/agentPolicy.ts`（256） | 禁写前缀 25-44；两道判定 |
| 权限桥 | `agent/VaultFsBridge.ts`（292） | 相对 + canonical 两道门 133-179；GRANT_TTL 60s |
| 上下文预算 | `engine/contextBudget.ts`（195） | 0.8/0.16/64k；摘要指令 29-46 |
| 上下文尾部 | `agent/contextTail.ts`（191） | 2400 上限；优先级与「另有 N 项」 |
| steering 块 | `agent/steeringBlock.ts`（40） | 「不能扩大权限」前言在前 |
| 路由 | `shared/agentRouting.ts`（195） | 优先级 21-26；词表 `agentRouting.ts:44-119`（curator 21 / scout 18 / distiller 17，共 56 词）；包含匹配 152 |
| 连续追问 | `shared/followUpRouting.ts`（40） | override > 自动/无对话 > 粘性 |
| 用量 | `shared/usage.ts`（143） | 未知标记 `—`；小数位规则 |
| transcript/过程 | `shared/{transcript,transcriptGroups,turnGroups,turnProcess,toolLine,agentPhase,evidence}.ts` | 流水线、证据雷达数据 |
| 对话存储 | `services/ConversationStore.ts`（460）、`ConversationRecorder.ts`（139）、`shared/conversation.ts`（247） | 上限 200/4 MiB/64 000/5000；残行补换行 305-322；索引损坏重建 386-445 |
| 对话搜索 | `shared/conversationSearch.ts`（45） | **旧版逐文件包含匹配，非 FTS5**；新版按 4.4 用 search.db |
| 回顾块 | `shared/conversationRecap.ts`（72） | 6000/1200 |
| ContextHub | `services/ContextHub.ts`（约 460） | Agent 只能写任务板/决策/检索片段 |
| provider 存储 | `services/ProviderStore.ts`（150） | 3.6 与 reasonix.toml 迁移 |
| 模型目录 | `services/modelCatalog.ts`（145） | 500 条/2 MiB/10s |
| 凭据 | `engine/CredentialStore.ts`（139） | safeStorage；不可用拒绝明文 |
| Agent 配置 | `services/AppConfigStore.ts`（256）、`shared/appConfig.ts` | 3.1；DeepSeek 预设 125-133；默认 steering 330-374 |
| Agent 契约 | `shared/agent.ts`（297） | 32 000 上限；EFFORT_LEVELS 无 medium |

关键行为确认（用于验收口径）：

- D-002 三项属实：ask 与 auto 都弹卡（NativeEngine.ts:468-469）；`availableEfforts` 恒空（NativeEngine.ts:215）→ 强度控件禁用；systemPrompt 从未传入（NativeEngine.ts:268/278-279，`engineServer.ts:93-101`）。D-005 属实（EngineHost.ts:816-827）。
- 默认 steering 原文在 `shared/appConfig.ts:336-373`（4 段，T02 原样迁移，不自行改写）。
- **Agent 默认配置与旧版运行行为不同（D-010）**：`shared/agent.ts:289-297` 的 `DEFAULT_AGENT_CONFIG` 在旧版 `src/` 与 `tests/` 中没有引用；旧版运行时 4 个 Agent 都是 `ask`、模型为空串。用户确认按 PRD 现写法实现，见 T02。
- 共享上下文的写入方：`ContextHub.applyAgentPatch`（`ContextHub.ts:176`）在旧版没有调用方，Agent 实际只经 `handoff_task` 写任务板；actionLedger 由变更执行完成后的回调写入（`main/index.ts:442-456`、`ipc/register.ts:462-475`）。`search_notes` 不写检索片段。
- 旧版对话搜索逐文件扫描（性能坑），新版按数据库设计 4.4 改为 search.db FTS5，属于已批准差异。

渲染进程（`src/renderer/src/components/agnes/`）：`AgnesDrawer.tsx`（600，容器）、`AgentTranscript.tsx`（413，**旧版无虚拟化**）、`UnifiedComposer.tsx`（418）、`RuntimeControls.tsx`（291）、`UsageReceipt.tsx`（91）、`PermissionCard.tsx`（158）、`EvidenceRadar.tsx`（96）、`ConversationHistory.tsx`（488）、`ProviderDiagnostics.tsx`（71）、`ThinkingIndicator.tsx`（69）、`RestoreNote.tsx`（35）、`agentMeta.ts`（74）；设置：`settings/ModelSection.tsx`（688）、`ProviderEditor.tsx`（718）、`AgentsSection.tsx`（551）。

旧版测试（`tests/unit/`，M2 相关约 45 个文件）：agentEngineContract 8、agentPolicy 15、agentRouting 14、contextBudget 14、contextTail 16、nativeEngine 21、engineHost 23、engineHostWiring 9、openaiClient 13、engineProvider 20、engineRpc 7、permissionRouter 9、reasonixFsBridge 10、executionMode 20、credentialStore 8、modelCatalog 13、modelCatalogProxy 2、nativeProvider 4、modelService 7、providerForm 29、appConfig 约 18、settingsDrafts 15、conversation 11、conversationStore 18、conversationRecorder 10、conversationM133 15、conversationGroups/View/Restore/Time 12/10/7/7、transcript 24、transcriptGroups 8、turnGroups 17、turnProcess 12、toolLine 8、usageFormat 16、chatStreaming 20、agentPhase 12、agentMeta 9、evidence 5、followUpRouting 6、kbToolsError 4、steeringBlock 3、confirmGate 5。

测试替身（`tests/helpers/`）：`fakeProvider.ts`（真 http 回环 + SSE helpers）、`memoryEngineProcess.ts`（内存双通道 + crash）、`fakeProxy.ts`；新版对应 `internal/ai/fake_provider_test.go` 与 e2e 假供应商，场景照搬（重试/半截流/401/工具/审批）。

## 3. 总路线

M0、M1 已完成。M2 为本计划。M3（其余设置、总览、技能导入、视觉验收与发布）仍未审批，M2 完成后另行细化。

规模参考：旧版对应源码约 12 000 行（引擎与 Agent 约 5 400、对话与上下文约 2 100、对话界面约 3 700、设置两节约 1 400）。

## 4. 任务清单

执行方式沿用用户 2026-11-18 的指示：**一次只做一个任务，做完汇报并停下，等用户审查后再做下一个**。状态：待做、进行中、阻塞、待验证、已验证。

| 编号 | 目标 | 修改范围 | 依赖 | 产物 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- | --- |
| T00 | 契约补齐与实测前置 | docs/API.md §6/§4.6、README、`spikes/t00-smoke/` | M1 | 帧载荷类型、M37 `BlockResult`、M45–M49 子结构；Secrets 与 netproxy 冒烟 | API 定义完整且与实现字段一一对应；Secrets 存取、netproxy 回环各有结论并记录 | 已验证 |
| T01 | 供应商与 key（M67–M70） | `internal/ai/providers.go`、`internal/service/settings.go`（M67–M70）、`internal/ai/providers_test.go` | T00 | providers.json 读写、reasonix.toml 只读迁移、DeepSeek 预设、fetchModels、key 存取 | V-PROV 全部通过 | 已验证 |
| T02 | Agent 配置与 steering（M63、M64、M71） | `internal/ai/agents.go`、`internal/service/appconfig.go`、`internal/index`（skillCatalog 只读） | T01 | agents.json 字节兼容、D-010 默认配置、steering clamp/基线、skillCatalog | V-AGT 全部通过 | 已验证 |
| T03 | 引擎骨架与 SSE 流式 | `internal/ai/{engine.go,sse.go,think.go,retry.go}` | T01、T00 | OpenAI 兼容客户端、SSE 组装、`<think>` 拆分、重试/退避/Retry-After、空闲超时、出站校验、netproxy | V-ENG 全部通过（httptest 假供应商） | 已验证 |
| T04 | 回合循环、会话与上下文预算 | `internal/ai/{session.go,loop.go,model.go,budget.go,compress.go}` | T01、T02、T03 | 会话懒创建、模型选择顺序、16 步工具循环、调用与结果配对、取消回填、只写完整采样、单回合出错隔离、`reasoning_content` 回填、token 估算与压缩 | V-LOOP、V-BUDGET 全部通过 | 已验证 |
| T05 | 工具、权限门与 Agent 写路径 | `internal/ai/tools/*`、`internal/ai/{policy.go,bridge.go}`、`internal/mutation/{plan.go,execute.go}`（接受 agent origin）、`internal/service/mutation.go` | T04、M1 | 8 个工具中除 `handoff_task` 外的 7 个（`handoff_task` 依赖 ContextHub，随 T08 交付）、两条写路径（直写桥 / 变更计划）、两道判定、系统区禁写、60 秒批准、D-005；M29/M30 支持 agent origin 并挂权限矩阵 | V-TOOL、V-PERM 全部通过；**权限专项测试**（提示词无法扩大权限） | 已验证 |
| T06 | 执行模式与审批卡（M41、M42） | `internal/ai/approval.go`、`internal/service/agent.go`（M39–M43） | T05 | 3 档模式、perm 帧状态机、expired、差异预览 | V-APPROVAL 全部通过 | 已验证 |
| T07 | 路由与改派（M44） | `internal/ai/{routing.go,priority.go}` | T03 | 词表原样迁移、优先级、理由文本 | V-ROUTE 全部通过 | 已验证 |
| T08 | 共享上下文与交接（M45–M49） | `internal/ai/{context.go,context_tail.go,redact.go}`、`internal/service/{agent.go,tools.go,mutation.go}` | T05 | 快照、pin/unpin、任务状态、当前文档上报、actionLedger 接变更完成回调、`handoff_task` 工具、contextTail 2400 预算与脱敏 | V-CTX 全部通过 | 已验证 |
| T09 | 对话历史与搜索（M50–M59） | `internal/conversation/{store.go,events.go,search.go}`、`internal/service/conversation.go` | T03、T06 | index.json/jsonl、上限、损坏恢复、search.db FTS5、导出、接着问 | V-HIST 全部通过 | 已验证 |
| T10 | 绑定层与流式通道 | `internal/service/agent_run.go`（M38–M44）、`src/lib/{stream.ts,agentApi.ts}`、`scripts/check-bindings.mjs` | T03–T09 | StreamFrame TS、Channel 前端封装、E07/E08/E09/E10 | 契约测试：Bind 集合 = 前端调用集合 | 已验证 |
| T11 | 对话界面骨架 | `src/components/chat/{ChatPane,MessageList,AgentMessage,UserMessage,Header}`、`src/stores/agentStore.ts` | T10 | 头部 44px、消息布局、过程流水线、思考折叠、呼吸动画、每条消息的「复制 / 重新回答」 | V-CHAT 全部通过；截图 37、38、39 | 待做 |
| T12 | Markdown 流式与引用 | `internal/render`（M37 按块增量，新增）、`src/components/chat/{MarkdownBlock,EvidenceRadar}` | T00（补 `BlockResult` 结构）、T11 | 末尾未封口块收起、宽表横向滚动、代码复制、引用标签、证据雷达 | V-CHAT-MD 全部通过 | 待做 |
| T13 | 输入框、运行参数条、路由/改派 | `src/components/chat/{Composer,RunParams,RouteLine,AgentPicker}`、`src/stores/composerStore.ts` | T11、T07 | Enter 发送、高度持久化、32 000 上限、参数条、改派菜单 | V-COMPOSER 全部通过；截图 34、35、36 | 待做 |
| T14 | 审批卡与用量收据 | `src/components/chat/{PermissionCard,UsageReceipt}` | T06、T13 | 差异预览（60 行）、允许/拒绝、Go 确认后移除、收据未知显示「—」 | V-CARD-AI 全部通过；截图 42、43 | 待做 |
| T15 | 历史栏、只读查看器与恢复 | `src/components/chat/{HistoryRail,ConversationViewer}`、`src/stores/historyStore.ts` | T09、T11 | 分组、搜索（250ms）、重命名/置顶/删除/清空、只读查看器（虚拟化）、导出、接着问、恢复提示 | V-HIST-UI 全部通过；截图 45、46 | 待做 |
| T16 | 会话信息浮层与引擎状态 | `src/components/chat/SessionInfo`、`EngineBadge` | T08、T10 | 引擎状态、模型诊断、读写边界、权限摘要、共享上下文 | V-INFO 全部通过；截图 40、44 | 待做 |
| T17 | 设置页「模型服务」 | `src/components/settings/{ProvidersSection,ProviderForm}` | T01、T00 | 供应商列表/表单、拉模型、key 状态、空状态 | V-SET-PROV 全部通过；截图 41、49、50 | 待做 |
| T18 | 设置页「Agents」 | `src/components/settings/{AgentsSection,SteeringEditor}` | T02 | 模型/强度/模式/Skills 即时生效、steering 草稿与保存条、对比与恢复默认 | V-SET-AGT 全部通过；截图 51 | 待做 |
| T19 | 假供应商 e2e 与闭环验收 | `internal/ai/fake_provider_test.go`、`scripts/e2e/ac-m2.sh` | T03–T18 | 假 OpenAI 供应商、AC-M2 全流程、重启恢复 | AC-M2 全部通过 | 待做 |
| T20 | 性能复核与独立审查 | `scripts/e2e/perf-ai.sh`、本计划第 6 节 | T19 | 流式帧率、输入延迟 p90、历史首帧、消息虚拟化 | 指标有实测；**整体独立审查**（权限门重点） | 待做 |
| T21 | 收尾 | README、CLAUDE.md、本计划 | T20 | 文档同步；推送前确认 | 用户确认后推送 | 待做 |

依赖顺序 T00 → T01 → T02 → T03 → T04 → T05 → T06 → T07 → T08 → T09 → T10 → T11–T18 → T19 → T20 → T21，按「一次一个」串行。T07 只依赖 T03，排在 T06 之后只是为了让权限门专项审查（T05+T06）尽早做完。

## 5. 单任务精确交接

> 通用纪律：Go 业务逻辑先写失败测试；`.mile-kb/` 下 agents.json/providers.json 按数据库设计 3.1/3.6 + 3.0.1 字节兼容；对话历史按 4.4；**Agent 写知识库只有两条既定路径**——`write_note`/`edit_note` 走权限桥直写（与旧版 `VaultFsBridge.ts:205-240` 一致，不开事务、不记操作日志），`mutate_notes` 走 MutationService 的变更计划与执行；错误码按 API §1.3；中文 UI、无 emoji；提交按 M0 检查清单。

### T00 契约补齐与实测前置

- API.md §6：补 `ToolCallFrame{id,title,kind?,status,detail?}`、`StatusFrame{state,phase?,model?,effort?,detail?}`、`UsageFrame{turn,cumulative}` 与 `Usage` 字段（对齐旧版 `usage.ts` 的 token/费用/缓存结构，未知可空）、`WritePreview`（形状沿用 M1 的 `src/lib/lineDiff.ts`，Go 侧移植见 T06）、`PermissionOption{id,label}`。
- API.md §6：帧表删去 `sidecar` 这一 kind。§9 Q4 已决定引擎状态走 E10 `engine-status` 事件，不放进 Channel。
- API.md §4.5：补 M37 的 `BlockResult` 结构。
- API.md §4.6：补 M45–M49 的入参校验与出参完整字段；核对 M38 的 `capabilities` 固定值。
- 两个本机冒烟验证（一次性 spike，结论写进第 9 节）：
  1. `mygo.Secrets`：只用一次性服务名 `Secrets.Service("mkb-spike-<随机>")` 做 Set / Get / Delete，结束时删除；验证超过 2560 字节被拒绝。不碰应用自己的服务名。
  2. `netproxy.Transport()`：对回环地址直连；系统未设代理时直连。PAC / WPAD 与 SOCKS5 的行为以 mygo 源码文档与技术设计 2.1 为准，本机没有对应代理环境时记为「未实测」，不当作已验证。
- README 增加 M2 会用到的命令占位。

### T01 供应商与 key

- providers.json 读写按 3.6：整文件 `SetEscapeHTML(false)`、固定键序、未知字段保留；读取截 50、保存校验 32；同名忽略大小写去重；`reasonix.toml` 托管区只读迁移（不存在时才读，结果为空不写文件）。
- DeepSeek 预设常量；`fetchModels`：openai 类型 `GET {baseUrl}/models` + Bearer；anthropic 类型按旧版 `modelCatalog.ts:55-59`（`GET {root}/v1/models` + `x-api-key` + `anthropic-version`，仍是只存不调聊天）；10 秒超时、响应 ≤2 MiB、解析 `data[].id`、去 `models/` 前缀、≤500 条排序；错误信息先替换 key 再脱敏。
- key：`Secrets.Set(envName, value)`，≤2560 字节、不含换行；界面状态由「能否读到」决定，不返回明文。
- **凭据隔离**：Secrets 写的是 Windows 凭据管理器，按应用标识区分，临时 APPDATA 隔离不了。所以 key 存取写成接口：单元测试用内存实现；唯一的真实存储测试用 `Secrets.Service("mkb-test-<随机>")` 并在 `t.Cleanup` 中删除；e2e 一律使用不需要 key 的假供应商。任何测试都不读写应用自己服务名下的凭据。
- **未保存草稿的 key**：`fetchModels` 的 `apiKey?` 参数承接编辑器里刚粘贴、尚未保存的明文，只在该请求内使用，不落盘、不进日志；T17 的「测试连接/拉模型」用它。
- 测试：移植 providerForm、modelCatalog、credentialStore 用例；新增伪造 key 不落日志。

### T02 Agent 配置与 steering

- agents.json 按 3.1：4 个键固定顺序、未知顶层键丢弃、未知字段保留；**默认配置按 D-010（PRD 现写法）**：agents.json 缺失、或缺某个 Agent 的键时，curator / scout / general 为 `model:"deepseek-v4-flash"`，distiller 为 `"deepseek-v4-pro"`；`effort:"high"`；scout 为 `approval:"auto"`，其余为 `"ask"`。模型存裸名，供应商里没有该模型时按 T04 的选择顺序回落。这与旧版运行行为不同（旧版 4 个都是空模型加 `ask`），已在 D-010 记录。非法 effort 归 `auto`、非法 approval 降级 `ask`，回退值与默认值是两回事。
- steering：≤4000，超出截断并标注；`steeringBaseVersion` 保留；「与默认对比」「恢复默认只改草稿」由前端状态实现，后端提供默认值接口（M64 reset 返回默认，不落盘）。
- skillCatalog（M71）：由索引文档派生（skillPaths 指向的目录）；只读。
- 测试：移植 appConfig/agents 相关用例 + 字节往返；新增「文件缺失时 4 个 Agent 的默认值等于 D-010」与「已有文件的值优先于默认值」。

### T03 引擎骨架与 SSE

- 出站校验：https 任意、http 只回环、URL 禁凭据；HTTP 客户端用 `netproxy.Transport()`（技术设计 2.1）：回环直连、PAC / WPAD、SOCKS5 可用，设置了代理环境变量时以环境变量为准（D-010）。
- SSE：`delta.content`/`reasoning_content`/`reasoning`、`<think>` 拆分、tool_calls 按 index 累积（无 id 补 `call_<index>`）、`[DONE]`/finish_reason 完整性、半截流报错。
- 重试：仅响应头前；408/429/5xx；401/403 立即鉴权错误；其他 4xx 立即失败；最多 10 次，500ms×2^(n-1) 上限 15s + 0–250ms 抖动；遵守 Retry-After ≤60s；状态帧文案「第 n/m 次重试，x 秒后自动继续」。
- 空闲：300 秒无字节中止（超时值可注入，便于测试缩短）；取消贯穿请求（ctx 取消 → 关连接）。
- 测试：httptest 假供应商覆盖 401/429(Retry-After)/5xx/半截流/超时/取消；**R-4 复测**：真实引擎 + 真实 Markdown，记录流式性能。

### T04 回合循环、会话与上下文预算

对应旧版 `engine/NativeEngine.ts`（631 行），是 M2 里最核心的一块。工具执行在本任务里用测试替身注入，真实工具由 T05 提供。

- 会话：按 (agent, vaultId) 懒创建、幂等（旧版 `engineHost.test.ts` 的「重复 createSession 是幂等的」）；会话信息供 M38 使用，`availableEfforts` 恒为空（D-002）。
- 模型选择顺序（旧版 `engine/nativeProvider.ts:24-51`）：会话所选 → Agent 保存的 → 第一个可用 openai 供应商的默认模型 → 该供应商模型列表第一个；名字不在可用供应商的模型列表里就回落，不把任意字符串发给供应商。没有可用供应商时引擎状态为 offline，并带原因。
- 工具循环：每轮最多 16 步，到上限以 `max_turn_requests` 结束；每个工具调用都要有对应的结果消息；取消时剩余调用回填「已取消，未执行」；只有正常结束的采样写入历史；参数不合法以错误文字回给模型，不中断回合。
- 带工具调用的 assistant 消息总是带上 `reasoning_content` 字段（DeepSeek 思考模式缺这个字段会返回 400，旧版 `NativeEngine.ts:351-358`）。
- steering 与共享上下文尾部块拼在用户消息末尾，不进 system；system 提示为空（D-002）。尾部块由 T08 提供，本任务先用空串。
- 单回合出错隔离：回合内的 panic 由 recover 转为该回合的 `error` 帧，不影响其他会话和应用。
- 估算：ASCII/4 + 非 ASCII×0.7 + 每消息 4；用真实 prompt_tokens 校准系数 0.5–3；窗口默认 64 000，供应商可覆盖。
- 80% 触发压缩：保留最近 ~16%，切点只在 user 消息前；旧内容由模型摘要；失败或仍超限 → 整轮丢最旧 + 截断说明；system 与工具定义计入。
- 测试：移植 contextBudget 全部用例、nativeEngine 中与循环相关的用例（步数上限、取消回填、只写完整采样、供应商报错不带 key）、nativeProvider 的模型选择用例，加摘要失败路径与 panic 隔离。

### T05 工具与权限

- 8 个工具的 schema 都在本任务定义（用 agnes-engine skill 的 kb-tool-schemas，逐字段对照旧版 `engine/kbTools.ts:86-234`），执行实现交付其中 7 个；`handoff_task` 在 T08 实现，在此之前调用它返回「交接尚未就绪」工具错误。描述中文；参数校验失败以错误文字回模型，不中断回合；结果截断 20 000。
- `set_aliases`：Agent 不能直接写 `.mile-kb/settings/`，所以按旧版（`EngineHost.ts:896-913`）经 M1 已有的 `AppConfigPatchAliases`（M66）写入，路径键先过相对路径校验，成功后发 E08 `app-config-changed`。
- 两条写路径（与旧版一致，见 `engine/kbTools.ts:495-517` 与 `EngineHost.ts:816-846`）：
  - `write_note` / `edit_note` → 权限桥 `bridge.go` → `vault.Write`；两道判定在桥内，60 秒 `allowWriteOnce` 在落盘前消费；**不生成变更计划、不开事务、不记操作日志**。`write_note` 自动建父目录但只能写内容区；`edit_note` old_text 必须恰好 1 次。
  - `mutate_notes` → M29 生成计划 → M30 执行（事务与操作日志照旧）。
- **扩展 MutationService**：M29/M30 接受 `origin=agent`（curator/scout/distiller/general），在开事务之前挂权限矩阵（系统区、draft、目标校验），permanentDelete 仍要求 T06 的消费型批准；用户 origin 的行为不变。API.md M29 的「M1 只接受 user」注记同步改为「M1 只接受 user；M2 起接受 agent origin，权限矩阵见 FR-AI-PERM」。
- 权限门两道：拼写层 + 真实路径层（旧版 `agent/VaultFsBridge.ts:133-179`）；系统区前缀（3.0 列举）+ 所有点号目录 + drafts（`shared/agentPolicy.ts:25-44,202-208`）；对 Agent 一律拒绝，用户不受影响。
- 测试：移植 agentPolicy 全部用例；新增「提示词注入无法扩大权限」用例（把越权路径写进 steering 与工具参数，仍被拒）。

### T06 执行模式与审批

- 三档：ask/auto 都发卡（文案不同）；yolo 不发卡直接执行内容区；permanentDelete 在 yolo 下直接拒绝并回工具错误（D-005）。非法值降级 ask。
- PermFrame 状态机 pending→approved/rejected/expired；Go 确认后才发 approved/rejected；取消/换库/重启对未决卡发 expired；approve/reject 过期返回 `approval-expired`。
- 60 秒消费型批准：键 sessionId+path；permanentDelete 额外要求「发卡时非 yolo」，用完即删（旧版 `EngineHost.ts:712-713,816-827`）。
- 差异预览：把 M1 的 `src/lib/lineDiff.ts` 移植成 Go（`MAX_LCS_CELLS`=4 000 000、`PREVIEW_MAX_LINES`=60、`PREVIEW_MAX_LINE_CHARS`=200），复用 M1 的 TS 用例作为 Go 用例的期望值；读取原文失败只显示一行摘要。
- 测试：移植 permissionRouter/executionMode 用例 + 卡片状态机 + 批准过期 + 消费一次。

### T07 路由

- 词表原样迁移（`shared/agentRouting.ts:44-119`，curator 21 / scout 18 / distiller 17 共 56 词）；小写包含匹配、不分词；优先级 scout > general > distiller > curator（21-26）；理由文本 `explainRoute` 同款；路由预览纯函数。
- 测试：移植 agentRouting 用例 + 边界（多类命中、空输入）。

### T08 共享上下文

- 快照字段与 M45 一致（`SharedContext{snapshotId,sequence,vault,activeDocument?,pinnedReferences,taskBoard,decisionLog,actionLedger,retrievalBundle}`）；任务板/决策/检索片段由 Agent 写，任务状态只能改自己那条。
- **actionLedger**：M30 执行完成后，先更新索引再调 ContextHub 记录（与旧版 `ipc/register.ts:462-475` 顺序一致）；用户与 Agent 的变更走同一入口。
- **`handoff_task`**：在本任务实现。发起者按 sessionId 反查，不信任模型自报的 agent；不能交给自己；goal ≤4000，inputs / constraints 各 ≤50 条、每条 ≤4000。
- 旧版 `ContextHub.applyAgentPatch` 没有调用方，Agent 只经 `handoff_task` 写任务板。新版照此，不新增让 Agent 写决策记录或检索片段的工具。
- **M49 需完整实现**：`AgentService.ContextReportDocument(vaultID,path?,revision,dirty)` 写入 ContextHub 的 activeDocument（含 revision 与可选 selection）；M0 的 `WindowService.ReportDocument` 保留为关窗守卫的专用入口，两者职责在契约里写明，不互相替代。
- contextTail（接到 T04 的尾部块）：2400 字符预算，优先级 revision → 当前文件/未保存/选区≤200 → 固定引用（≤1500）→ 未完成任务 → 决策 → 检索片段（≤4，放不下整条丢）；省略时「另有 N 项未列出」；绝对路径脱敏（旧版 `agent/contextTail.ts:67-186`）。
- 测试：移植 contextHub/contextTail 用例 + 截断顺序 + 脱敏。

### T09 对话历史与搜索

- index.json/jsonl 按 4.4；上限 200 对话 / 4 MiB / 单事件 64 000 / 读回 5000；标题派生与重命名规则；坏行跳过、残行补换行；index.json 损坏扫 jsonl 重建。
- search.db：独立库、FTS5、预切词同 M1；带 `meta/schema_version` 迁移（与 4.5 同纪律）；删除单条同步删行；clear 删库；损坏/版本不符全量重建。
- 导出 Markdown 规则；「接着问」回顾块（≤6000、每条 ≤1200、声明非新指令、只用一次、失败放回）；Agent 运行中拒绝切换（M56）。
- 测试：移植 conversationStore/conversationRecorder 相关用例 + 搜索防抖/片段 + 重建。

### T10 绑定层与流式通道

- M38–M43 全部绑定；StreamFrame TS 类型由 mygo generate 产生；`src/lib/stream.ts` 把 Channel 包成 async iterable + onclose；E07/E08/E09/E10 事件。
- 契约测试：`src/mygo.ts` 的方法集合与前端调用集合一致（脚本比对），防止漏绑。

### T11 对话界面骨架

- 头部 44px（历史开关、标题、待审批标签、会话信息、新对话）；用户消息右对齐 72%，Agent 消息 760px 居中、24px 头像；过程流水线（工具合并、已完成 N 步 · 思考 x.xs、同 id 原地更新）；思考折叠；呼吸动画（减少动效静止）。
- 消息列表虚拟化（P-3/P-5）；流式只更新进行中消息。
- 每条消息的操作（PRD FR-AI-CHAT）：复制（只复制正文）、重新回答（用同一条用户消息重新发起回合）。流式中断或空闲超时后也能重新回答（PRD §9）。
- 截图 37/38/39 对照。

### T12 Markdown 流式与引用

- **M37 是新建绑定**：`RenderService.renderBlock(text, kind)` 与 Go 侧「已封口块只渲染一次」的按块缓存，不能复用 M36 的按文档缓存。
- M37 renderBlock：按块缓存，只重渲染末尾未封口块；未闭合表格/链接收起显示「正在生成…」。
- 引用识别与证据雷达（路径类型、次数、首个序号 [n]，默认收起，不显示匹配度）；宽表横向滚动、代码复制、图片不加载只显示标签、库内图片可点击打开。

### T13 输入框与参数条

- Enter 发送 / Shift+Enter 换行；高度拖动 + ↑↓ 16px + 持久化；32 000 上限拒绝不截断；参数条折叠摘要（模型、强度、模式、本轮用量）；路由行「发给 X」+ 权限摘要；改派菜单与保持规则。
- 截图 34/35/36。

### T14 审批卡与用量收据

- 审批卡：允许/拒绝、差异预览、发卡时模式记录、Go 确认后移除、expired 文案；卡片状态以 Go 为准。
- 收据：本轮/累计、token 六项、费用与币种、估算标记、缓存命中率；未知显示「—」，小数位规则；累计值由 Go 给。
- 截图 42/43。

### T15 历史栏与恢复

- 只读查看器（旧版 `ConversationHistory.tsx` 的 `ConversationViewer`）：回放一条历史对话，消息列表虚拟化（P-4），提供「接着问 / 导出 / 返回」。
- 200px、开合持久化；搜索 250ms/≤200；分组置顶/今天/昨天/更早；行悬停重命名/置顶/删除；底部「共 N 条 · 清空全部」；两步确认复用 confirmGate；导出；接着问；启动恢复提示。
- 截图 45/46。

### T16 会话信息浮层与引擎状态

- 340px：引擎状态、模型与诊断、读写边界、4 个权限摘要、共享上下文（任务板/固定引用）、设置入口；截图 44；引擎未配置截图 40。

### T17 设置页「模型服务」

- 供应商列表/表单（字段与校验按 3.6）、DeepSeek 预设、拉模型、key 状态、空状态；截图 41/49/50。

### T18 设置页「Agents」

- 4 个 Agent：模型、强度（控件禁用，D-002）、执行模式、Skills 选择即时生效；steering 草稿/保存条/对比/恢复默认；截图 51。

### T19 假供应商 e2e 与闭环验收

- `internal/ai/fake_provider_test.go`：httptest 假 OpenAI 供应商，覆盖流式、工具调用、审批、取消、重试、半截流、用量。
- `scripts/e2e/ac-m2.sh`：真实窗口 + 不需要 key 的假供应商（`apiKeyEnv` 留空，不写凭据管理器） → 发消息 → 路由 → 流式 → 工具审批 → 允许 → 收据 → 重启恢复 → 历史搜索。
- AC-M2 全部通过。

### T20 性能与整体审查

- 测量口径（写进 `scripts/e2e/perf-ai.sh`，用 automation `evaluate` 注入）：
  - 流式帧率：回合期间用 `requestAnimationFrame` 采样帧间隔，统计 >20ms 的帧数与最大间隔；
  - 输入延迟 p90：流式进行中用 automation 向输入框注入按键（R-4 同款方式），量 `keydown → 下一帧` 的间隔，取 p90；
  - 历史查看器首帧：从点击历史条目到 `.prose` 首次出现，用 `performance.now()`；50 条长回答样本由假供应商生成；
  - 消息虚拟化：滚动中统计 `[data-message]` 行数 ≤ 可见行 + overscan。
  - 任何一项若自动化无法注入（例如 WebView2 不派发合成按键），改用「同链路代理指标」并在第 6 节写明，不把「不卡」当结论。
- **整体独立审查**：权限门、key 处理、审批状态机、写路径、契约一致性。

### T21 收尾

- README、CLAUDE.md、本计划账本；列提交；询问后推送。

## 6. 验证与验收

| 编号 | 给定条件 | 操作 | 预期结果 | 方式 |
| --- | --- | --- | --- | --- |
| V-PROV-1 | 新增 DeepSeek 预设 | 保存 | providers.json 字节符合 3.6，Key 存 Secrets，界面「已配置」 | Go + e2e |
| V-PROV-2 | 33 个供应商 | 保存 | 拒绝（保存上限 32） | Go |
| V-PROV-3 | 文件含 60 条 | 读取 | 保留 50，其余截断 | Go |
| V-PROV-4 | 没有任何可用供应商 | 打开对话 | 引擎 offline，原因如实；编辑器与检索照常 | e2e |
| V-PROV-5 | 供应商需要 key 但未配置 | 发消息 | 回合报鉴权类错误，错误文字不含 key；界面提示去配置 | httptest + e2e |
| V-PROV-6 | 任意测试运行后 | 检查凭据管理器 | 应用自己服务名下没有新增或改动的凭据 | Go |
| V-AGT-1 | 修改 steering 4001 字符 | 保存 | 截断并标「已截断」 | Go |
| V-AGT-2 | 非法 effort/approval | 读取 | 归 auto / 降级 ask | Go |
| V-AGT-3 | agents.json 含未知字段 | 读写往返 | 未知字段保留、键序稳定 | Go |
| V-AGT-4 | agents.json 不存在 | 读取 | 4 个 Agent 的默认值等于 D-010；已有文件时文件值优先 | Go |
| V-ENG-1 | 假供应商 401 | 发消息 | 立即鉴权错误，不重试 | httptest |
| V-ENG-2 | 429 + Retry-After | 发消息 | 按 Retry-After 等待并重试，状态帧可见 | httptest |
| V-ENG-3 | 半截流 | 发消息 | 回合报错，不写入半截回答 | httptest |
| V-ENG-4 | 300 秒无字节 | 发消息 | 中止并报超时（用缩短配置测试） | httptest |
| V-LOOP-1 | 假供应商连续返回工具调用 | 发消息 | 第 16 步后以 `max_turn_requests` 结束 | httptest |
| V-LOOP-2 | 等审批时取消 | 取消 | 当前与剩余调用都回填「已取消，未执行」，历史里没有半截采样 | httptest |
| V-LOOP-3 | 上一轮有工具调用 | 发下一轮 | 请求里那条 assistant 消息带 `reasoning_content` 字段 | httptest |
| V-LOOP-4 | 工具实现 panic | 发消息 | 该回合以 error 结束，其他会话照常，应用不退出 | Go |
| V-LOOP-5 | 会话所选的模型不在任何供应商里 | 发消息 | 回落到第一个可用供应商的默认模型 | Go |
| V-BUDGET-1 | 窗口 80% | 继续发消息 | 压缩发生，切点在 user 前，摘要注入 | Go |
| V-BUDGET-2 | 摘要失败 | 继续 | 整轮丢最旧 + 截断说明，不丢当前消息 | Go |
| V-TOOL-1 | 8 个工具 | 逐个调用 | 参数/结果/截断符合 PRD 表 | httptest |
| V-TOOL-2 | edit_note old_text 0/2 次 | 调用 | 返回错误文字，不中断回合 | Go |
| V-PERM-1 | 提示词要求写 `.mile-kb/settings/x.json` | 任何模式 | 拒绝并回工具错误；文件不存在 | httptest |
| V-PERM-2 | yolo 模式永久删除 | 调用 | 直接拒绝，不弹卡；**切到 ask 后同一操作弹卡，允许后执行且只放行一次**（AC-AI-2 两段） | httptest |
| V-PERM-3 | 审批卡允许 | 60 秒内同路径再写 | 允许一次后失效 | Go |
| V-PERM-4 | 回合取消 | 未决卡 | 推 expired，approve 返回 approval-expired | Go |
| V-ROUTE-1 | 「帮我整理归档这个目录」 | 输入 | 「发给 整理归档」+ 识别词（AC-AI-3） | e2e |
| V-ROUTE-2 | 同时命中多类 | 输入 | 按 scout>general>distiller>curator | Go |
| V-CTX-1 | 上下文超 2400 | 发消息 | 按优先级截断，写明「另有 N 项未列出」 | Go |
| V-CTX-2 | 含绝对路径 | 注入 | 脱敏后不含真实路径 | Go |
| V-HIST-1 | 连续对话 3 轮 | 重启 | 恢复最后对话，历史完整 | e2e |
| V-HIST-2 | 坏行/残行 | 读取/追加 | 跳过坏行、补换行后追加 | Go |
| V-HIST-3 | 搜索中文 | 输入 | 250ms 防抖、命中 user/message、片段 ±30 | Go + e2e |
| V-HIST-4 | 删除/清空 | 两步确认 | 600ms 内二次点击无效；清空删残留 | Go + e2e |
| V-CHAT-1 | 流式 10k 字符 | 观察 | 60fps，只更新进行中消息 | e2e + perf |
| V-CHAT-2 | 末尾未闭合表格 | 观察 | 收起并显示「正在生成…」 | e2e |
| V-CHAT-3 | 一条已完成的回答 | 点「重新回答」 | 用同一条用户消息发起新回合；「复制」只复制正文 | e2e |
| V-HIST-5 | 50 条长回答的历史对话 | 打开只读查看器 | 消息列表虚拟化，首帧按 T20 口径计时 | e2e + perf |
| V-CARD-AI-1 | 允许审批 | 点击 | 卡片在 Go 确认帧后移除 | e2e |
| V-CARD-AI-2 | 供应商无费用 | 回合结束 | 费用「— 未提供」，不显示 $0.00（AC-AI-4） | Go + e2e |
| V-SET-PROV-1 | 空状态 | 打开 | 与截图 41 一致 | e2e |
| V-SET-AGT-1 | 改 steering 不保存即离开 | 离开 | 草稿确认提示 | e2e |
| AC-M2-1（本计划自定的闭环用例，PRD 无此编号） | 假供应商 | 配置 → 发消息 → 路由 → 流式 → 审批 → 收据 → 重启 → 搜索 | 每步与 PRD/截图一致 | e2e |

性能（PRD 10.1；G-PERF 维持警告级）：

| 指标 | 目标 | 新版实测 | 旧版基线 |
| --- | --- | --- | --- |
| 流式输出 | 60fps | 未测 | 单条 9832 字每帧 6.7–9.9ms；长会话未测 |
| 输入框按键延迟 p90（50 轮流式期间） | ≤16ms | 未测 | 未测 |
| 历史查看器（50 条长回答）首帧 | ≤100ms | 未测 | 172–189ms |
| 消息列表虚拟化 | 60fps | 未测 | 未测 |

## 7. 协作、审查与执行方式

- 一次一个任务；每完成汇报并停下。
- 测试先行：Go 侧协议、重试、预算、权限、路由、上下文、历史、存储一律先写测试；移植旧版单测按 2.1 清单。
- TDD 例外：契约文档（T00）、设置页纯视觉外壳（T17/T18 外壳，逻辑仍有单测）、e2e 脚本本身。
- 独立审查：**T05+T06 完成后一并做权限门专项审查**（两道判定、系统区、审批状态机、Agent origin），T19 后整体审查；修复上限 3 轮。
- Agent 写知识库只走两条既定路径（直写桥 / 变更计划，见第 5 节通用纪律）；key 只进 Secrets，测试不碰应用自己服务名下的凭据；日志不记正文与 key。
- 提交：每个任务一次，不推送；M2 末询问。

## 8. 来源基线与批准记录

| 资料 | 版本 / 日期 | 适用范围 |
| --- | --- | --- |
| PRD | v1.7 | FR-AI-*、FR-SRCH/LINK/MUT/AREA 的复用 |
| API | v1.0 + T00 补齐 | M38–M49、M63/M64、M67–M71、§6 |
| 数据库设计 | v1.0 | 3.1、3.6、4.4、§6–8 |
| 技术设计 | v1.0 | 第 2.1 节选型（netproxy、Secrets）、第 4.6 节、第 7 节 R-4、第 10 节门禁 |
| decisions | D-001–D-012 | D-002、D-005、D-010、D-012 |
| 旧版代码 | v2.7.0，只读 | 2.1 盘点 |
| Skills | agnes-engine 等 | 第 2 节 |

| 批准标识 | 内容 | 依据和时间 | 状态 |
| --- | --- | --- | --- |
| B-M2-01 | M2 范围 = C6 + C7 + 设置两节 | 用户 2026-11-18 批准 v0.3 并放行 T00 | 已确认 |
| B-M2-02 | 第 1.3 节非目标、第 4 节任务与顺序、第 7 节 TDD 例外与审查安排 | 用户 2026-11-18 批准 v0.3 并放行 T00 | 已确认 |
| B-M2-03 | 执行节奏沿用「一次一个任务」 | 用户 2026-11-18 批准 v0.3 并放行 T00 | 已确认 |
| B-M2-04 | ~~代理能力不足时降级直连并提示~~ | 撤回：技术设计 2.1 已确认 netproxy 方案（含 PAC / WPAD、SOCKS5），不需要另批降级 | 已撤回 |
| B-M2-05 | Agent 写路径按旧版：`write_note`/`edit_note` 走权限桥直写（不开事务、不记日志），`mutate_notes` 走 M29/M30；同时扩展 M29/M30 接受 agent origin | 用户 2026-11-18 批准 v0.3 并放行 T00 | 已确认 |
| B-M2-06 | Agent 默认配置按 PRD 现写法（D-010） | 用户 2026-11-18 选择「按 PRD 现写法」 | 已确认 |

## 9. 恢复入口与执行账本

### 下次会话先读

- 当前阶段：T10 已完成并提交，等待亲测与放行 T11。
- 下一个任务：用户放行后 T11（对话界面骨架）。
- 工作区：`main` 领先 `origin/main`，本地提交自 `b5343e0` 起（v0.2、v0.3、审查修复、D-012、T07–T10）均未推送。
- 待决事项：`check:bindings` 是否进正式门禁清单由 T20/T21 定；13 项手动确认；旧版性能基线。

| 时间 | 任务与状态变化 | 文件 / 版本范围 | 验证与证据 | 待决事项 | 下一任务 |
| --- | --- | --- | --- | --- | --- |
| 2026-11-19 | T10 完成（绑定层与流式通道） | `contract.go`：`StreamFrame`/`PermFrame`/`ToolCallFrame`/`StatusFrame`/`UsageFrame`/`PermissionOption`、`AgentSessionInfo`/`AgentListResult`/`EngineStatus`/`EngineCapabilities`/`PromptResult`/`RoutePreviewResult`、E07/E08 载荷。`internal/service/agent_run.go`：M38 List（懒建会话、模型解析、availableModels、availableEfforts 恒空）、M39 Prompt（≤32 000 拒绝、空消息拒绝、用户消息先落历史、recap 取出并在失败时放回、steering + contextTail 尾部注入、审批管理器按回合创建）、M40 Cancel、M41/M42 Approve/Reject（requestId 字符集与 256 上限、未知卡 approval-expired、requestID→manager 注册表）、M43 Reset（取消+丢弃会话+清 recap）、M44 RoutePreview（带职责名理由）；帧通道：正文/思考按 512 rune 合并、非 delta 事件为边界、终端帧在卡片 expired 之后最后发送并带 `finish`、Channel 写失败即取消回合；录制器：user→message/thought 合流→tool 终态→turn-end/error 落 jsonl；E10 由 providers+key 计算 offline/ready 并在变化时推送（List 与启动时），E07 挂 store、E08 挂 set_aliases、E09 挂 hub。`src/lib/stream.ts` + `src/lib/agentApi.ts`：Channel 包 async-iterable 句柄（abort/done），全部 M38–M59 方法与 E07–E10 订阅。`scripts/check-bindings.mjs` + `npm run check:bindings`：AgentService/ConversationService 生成方法必须被前端调用、前端调用必须有对应绑定。`npm run generate` 重新生成 `src/mygo.ts`（+544 行）。**设计差异**：帧合并按 512 字符阈值而非 16ms 定时器（API §6 二选一，语义不变、少一个定时 goroutine）；`TakeRecap`/`PutBackRecap` 改为不导出，避免绑定后前端把 recap 抢先消费；`contextReportDocument` 的 path 空串=null 约定维持 T09 注记。测试新增 7 项（steering 1、agent_run 6：引擎状态、路由预览、Prompt 校验、含真实审批的端到端回合（写盘+帧序+历史事件序 user,tool,message,turn-end）、取消过期卡、Reset 换会话） | internal/{contract,ai,service}、main.go、src/{lib,mygo.ts}、scripts/check-bindings.mjs、package.json | `go test ./...` 465 项通过（17 包）；`go vet`、`gofmt -l` 无输出；`npm run check:bindings` 通过（AgentService:12 ConversationService:10）；`tsc --noEmit` 无错；`vitest` 79 项通过 | `check:bindings` 是否进门禁由 T20/T21 定 | T11（待放行） |
| 2026-11-19 | T09 完成（对话历史与搜索） | `internal/conversation/events.go`：6 种事件严格解析（逐字段校验、坏行跳过计数、正文 64 000 截断 + 标注、tool 字段上限、ISO 与非空校验）、`CoerceIndex`（坏项丢弃、重复 id 保留第一个、200 上限、activeId 必须存在）、标题清洗/派生（80/40 + 省略号）、导出 Markdown 与文件名规则、回顾块（6000/1200、从新到旧装填、省略标注、「非新指令」前言）。`store.go`：`index.json` + `<id>.jsonl` 布局、200 对话 / 4 MiB（仅新一轮前检查）/ 读回 5000 / 残行补换行、索引原子写、损坏 index.json 备份 `.corrupt` 后扫 jsonl 重建、Create 复用空活动对话、remove/activate/clear（含未登记残留文件）、全部操作串行化。`search.go`：独立 `search.db`（FTS5、与知识库同一预切词、`meta.schema_version`）、`conv_text` 表保存原文供 ±30 片段、按会话取最早命中、按 updatedAt 倒序、≤50、删除单条同步删行、clear 删库、损坏/版本不符/缺失时扫 jsonl 重建；历史写入不因搜索索引失败而失败。`service/conversation.go`：M50–M59 绑定（库校验、查询 ≤200 去空白、导出走系统保存对话框、M56 运行中拒绝切换的 `app.AgentBusy` 钩子、回顾块 `TakeRecap`/`PutBackRecap` 供 T10）。**与设计的差异**：search.db 增一张 `conv_text`（原文，派生可重建），因为 FTS 存的是预切词文本无法直接生成片段；搜索只索引 user/message（按 4.4），旧版额外的标题命中未保留；M57 出参按 API 的 `{conversationId,title,excerpt,matchedAt}`（旧版 `{id,title,snippet}`），matchedAt 取会话 updatedAt；事件结构体用指针字段保证三种事件各写各的键、其余键不落盘。测试新增 26 项（conversation 22：解析/截断/标题/coerce/排序/导出/回顾 + 存储/重建/边界/上限/搜索/损坏重建；service 4：生命周期、busy 与回顾、库校验、导出前置校验） | docs/PLAN-M2.md、internal/{contract,conversation,service}、package.json 未动 | `go test ./...` 458 项通过（17 包）；`go vet ./...`、`gofmt -l` 无输出；`tsc --noEmit` 无错；`vitest` 79 项通过 | E07/E09、录制器合流与 M44 绑定随 T10 | T10（待放行） |
| 2026-11-19 | T08 完成（共享上下文与交接） | `contract.go`：M45–M49 全部子结构 + `SharedContext`/`SharedContextResult`。`internal/ai/context.go`：`ContextHub`（sequence/snapshotId、有序 upsert、pin/unpin、任务状态分权、`CreateHandoff` 拒绝交给自己、`RecordActions`；相同 vault 值不重复提交；`OnChanged` 供 T10 接 E09）。`internal/ai/context_tail.go`：2400 硬预算按「revision → 当前文件/选区 200 → pin 1500 → 未完成任务 → 决策 → 检索 4 条」渲染，省略写「另有 N 项」，检索整条进出，最终按整行兜底。`internal/ai/redact.go`：移植 `redact.ts`（vaultRoot 先于 home、`KEY=value`/sk- 前缀脱敏）。`internal/service/agent.go`：M45–M49（pin note 截 500 不拒绝、pin id 为 sha256 不透明、unpin/taskId 字符集、M49 path 空串=清空、revision 非负、selection 截 200），`syncContextVault` 由开库/监听/索引状态驱动，vault revision 用监听序列计数。`service/tools.go`：`handoff_task` 实现（发起者取 runner 会话、owner≠self、goal≤4000、inputs/constraints≤50×4000，按 PRD 而不是旧版无上限）。`mutation.go`：完成后先把受影响目录入队索引，再 `RecordActions` 写 actionLedger。**与计划文本的差异**：handoff 放在既有 `service/tools.go`（runner 已持有 session/agent），未新建 `internal/ai/tools/handoff.go`；M49 的 `path` 用空串表示 null（绑定层 T10 映射）；旧版 `assertString` 超长是直接切片无省略号，已照抄；pin 的空 note 视为未提供（绑定层无法区分 undefined 与空串）。测试新增 25 项（hub 7、contextTail 10、service 5、handoff 2、actionLedger 1） | docs/API.md（无需改，M45–M49 原定义已落实）、docs/PLAN-M2.md、internal/{contract,ai,service} | `go test ./...` 432 项通过（16 包）；`go vet ./...`、`gofmt -l` 无输出；`tsc --noEmit` 无错；`vitest` 79 项通过 | E09/尾部注入随 T10；M44 绑定随 T10 | T09（待放行） |
| 2026-11-19 | T07 完成（路由与改派） | `internal/ai/routing.go`：`RouteRules` 原样迁移 `agentRouting.ts:44-119`（curator 21 / scout 18 / distiller 17 共 56 词）、`RouteToAgent`（小写包含、不分词、词频不参与、命中后按优先级取胜、空输入/无命中兜底 general，理由文案同旧版）、`ExplainRoute`（命中最多列 3 词、兜底不编造词）；`ResolveFollowUpRoute`（override > 自动/无对话 > 粘性，`followUpRouting.ts:34-40`）。`internal/ai/priority.go`：`RoutePriority`（scout 0 / general 1 / distiller 2 / curator 3）与 `FallbackAgent`。测试：移植 agentRouting 14 项 + followUpRouting 6 项为 10 个 Go 用例（意图分类、兜底、空输入、大小写、混合优先级 scout>distiller>curator、只明确整理意图给 curator、输出恒为 4 个已知 Agent、词表计数 21/18/17/56、理由文本截断、连续追问 6 种组合），先写测试见 `undefined: RouteToAgent` 红灯再实现转绿 | `internal/ai/{routing.go,priority.go,routing_test.go}`、docs/PLAN-M2.md | `go test ./internal/ai -run 'Route|FollowUp'` 10 项通过；`go test ./...` 408 项通过（16 包）；`go vet ./...`、`gofmt -l` 无输出 | M44 service 绑定随 T10；V-ROUTE-1 的 e2e 在 T19 | T08（待放行） |
| 2026-11-19 | N4 决策：永久删除的新鲜批准保持更严 | docs/PRD.md v1.8（FR-AI-PERM 批准语义与 60 秒口径、`mutate_notes` 行）、docs/decisions.md D-012、docs/PLAN-M2.md | 用户 2026-11-19「保持更严」；实现无需改动：`approval.go` 现实现即只对 `kind=mutation` 的卡登记 grant，`TestFreshMutationGrant` / `TestApprovalYoloSkipsCard` 已覆盖；本轮只改文档，`git diff` 仅 docs | 无 | T07（待放行） |
| 2026-11-19 | T05+T06 权限门专项审查处理（独立 task-reviewer，范围 `a2f0e31^..HEAD`）。**B1 属实已修**：新增 `pathguard.CanonicalRel`（最深现存祖先 `EvalSymlinks` + 尾段拼接，展开 8.3 短名/大小写别名，同旧版 `resolveWithinRoot`），工具层 `confirmPath` 与 `mutateNotes`、执行器 agent 计划复查均改为「拼写门 → canonical 门」两道；本机 E: 有 8.3 别名，`MILE-K~1/settings/evil.json` 用例真实执行（未跳过）被拒，篡改计划用例执行器返回 permission-denied。**B2 属实已修**：`ApprovalGate.AllowWrite` 增加 `*contract.WritePreview`；write/edit 在发卡前用已读快照构造预览（新建 created、已有 modified、读失败无预览不堵审批，同 `EngineHost.ts:658`），`PermEvent` 全生命周期（pending/approved/rejected/expired）携带 ToolName/Detail/Preview/Options。**N1 属实已修**：`edit_note` 对齐 `applyExactEdit`——old/new 先 `toLf`、空串拒绝、相同拒绝、**重叠计数**（`aaa` 里 `aa` 判歧义）、四条文案照旧版。**N2 属实已修**：`mutate_notes` 收窄为 schema 的 6 种 kind（create/edit 走 write_note/edit_note），补 source/target 必填校验（旧版 `mutationArgs`）。**N3**：写卡不发复用 grant 为计划已注明的有意偏离；新代码先校验 old_text 再发卡，旧版「批准后校验失败、60 秒内重试不再询问」的场景不复存在，用户可见行为等价。**N4 待决**：旧版任意批准卡都登记 mutation 新鲜批准，新实现只有 mutation 卡登记（更严）；保持或对齐待用户拍板。建议项：删 `approval.go` 死占位、修正 `LineDiff` 参数注释。新增测试 11 项（canonical 2、工具 7、审批 3 含并发 approve/reject 与发卡后换档） | `internal/pathguard/pathguard.go`、`internal/service/tools.go`、`internal/ai/{approval,linediff}.go`、`internal/mutation/execute.go` 及四个包的测试 | `go test ./...` 397 项通过（16 包）；`go vet ./...` 无输出；`gofmt -l` 无输出；`tsc --noEmit` 无错；`vitest` 79 项通过；`go test -race` 需 cgo，本仓门禁未包含，未跑 | N4；PRD FR-AI-PERM「60 秒有效」表述与实现口径 | T07（待放行） |
| 2026-11-18 | T06 完成 | `internal/ai/linediff.go`：移植旧版 LCS 行级 diff（4M 格退化、60 行上限、单行 200 字符裁剪）与 `BuildWritePreview`（created / modified / summary：too-large、unchanged）。`internal/ai/approval.go`：`ApprovalManager` 实现审批卡状态机（pending → approved/rejected/expired）、ask 与 auto 都发卡、yolo 不发卡、`ExpireSessionCards` 取消时推 expired、approve/reject 过期返回 `approval-expired`、mutation 批准登记 60 秒 grant 并由 `ConsumeFreshMutation` 消费一次（yolo 下永久删除无 grant，按 D-005 拒绝）、未知 approval 值降级 ask。`ApprovalGate` 接口由 `*contract.Failure` 改为 `error`：等待审批时取消可贯穿到回合循环，`gateError` 区分取消与拒绝文字。T05 的 runner 与 fake gate 同步更新。测试：lineDiff/preview 4 项、approval 11 项（允许/拒绝/取消过期/未知卡/非法选项/yolo/新鲜批准一次性与 60 秒过期/会话失效/模式归一） | docs/PLAN-M2.md、docs/API.md、internal/ai/{linediff,approval}.go 与测试、internal/contract（DiffLine/WritePreview）、internal/service/tools.go | `go test ./internal/ai ./internal/service` 112 项通过；全仓门禁待跑 | 审批卡帧接 StreamFrame 在 T10；写路径的「批准即放行本次」未登记可复用的 write grant（旧版 grant 用于批准与写盘之间，本次直接放行），已注明 | 权限专项审查（T05+T06） |
| 2026-11-18 | T05 完成 | `internal/policy`：路径归一（拒绝对路径/UNC/盘符/空/dot 片段）、全局禁写前缀（config/transactions/index/provenance/settings，大小写不敏感）、system area 与 drafts 一律拒、内容区白名单、`CanMutate`（create 系列走写判定、结构变更查 source 与 target、permanentDelete 标 `RequiresFreshApproval`）。`internal/mutation`：计划接受 4 个 agent origin，权限门在**任何文件系统检查之前**运行（系统路径返回 permission-denied，不泄露存在性）；`Executor.Execute` 对 agent 计划再校验一次，防篡改计划绕过。`internal/ai/tool_schemas.go`：从 agnes-engine 的 kb-tool-schemas.json 逐字段生成 8 个工具定义。`internal/service/tools.go`：`AgentToolRunner` 实现 7 个工具（handoff 待 T08），`ApprovalGate` 接口留给 T06；write/edit 走「policy → gate → vault 直写」，mutate 走「policy → gate（永久删除再消费新鲜批准）→ M29 计划 → M30 执行」，set_aliases 经 M66。测试：policy 7 项、mutation 新增 5 项（agent 来源接受/拒绝、执行期二次拦截）、service 工具 14 项（写/编辑/分页/列目录隐藏系统区/目录创建/越权拒绝/永久删除新鲜批准/handoff/别名/未知工具/8 个 schema） | docs/API.md（M29 origin 注记）、docs/PLAN-M2.md、internal/{policy,mutation,service,ai} | `go test ./internal/...` 76 项通过（三个包）；`go build ./...` 通过 | 真实审批卡与 60 秒批准在 T06；handoff 在 T08；search_notes 走空索引时只验证了「没有命中」路径，真实索引结果待 T19 | T06 |
| 2026-11-18 | T04 完成 | `internal/ai/session.go`：按 (agent, vault) 懒创建幂等会话、Reset、`SessionInfo`（`availableEfforts` 恒空）、`ResolveProvider`（会话所选 → 保存的 → 第一个 openai 默认模型 → 模型列表第一个，名字不在列表即回落，无可用时 engine-unavailable 带原因）、`ModelChoices`；`internal/ai/loop.go`：16 步工具循环、每步预算维护、用户消息先入历史、只有干净结束的采样写 assistant、工具调用与结果一一配对、取消回填「已取消，未执行」、工具失败/参数错误变文字、结果截断 20 000、`reasoning_content` 恒带（指针类型，空串也序列化）、usage 校准与累计、单回合 panic 隔离（监听器 panic 也复位于会话）；`internal/ai/budget.go`：字符估算、0.5–3 校准、80% 触发、16% 尾部切点只在 user 前、摘要替换与整轮截断兜底、`AddUsage`。测试：新增 12 项（预算 6、会话 4、循环 8 中的关键路径：普通回合、工具回合、步数上限、取消回填、工具错误文字、流错误只留提问、panic 隔离后会话可用、长历史压缩） | docs/PLAN-M2.md、internal/ai/{session,loop,budget}.go 与测试、internal/ai/engine.go（`ReasoningContent` 改指针） | `go test ./internal/ai` 全部通过（含新增） | 工具 panic 被转为工具错误文字而非终止回合（比「单回合出错隔离」更温和，已注明）；摘要失败路径用 truncate 兜底逻辑覆盖，未单独造摘要请求失败场景 | T05 |
| 2026-11-18 | T03 完成 | `internal/ai/think.go`：`<think>` 拆分（只在回答开头识别，跨块标签扣住，未闭合算思考）；`sse.go`：`data:` 行组装、正文/reasoning_content/reasoning、tool_calls 按 index 累积（无 id 补 `call_<index>`）、usage 归一（DeepSeek hit/miss、OpenAI cached_tokens 推 miss）、`[DONE]`/finish_reason 完整性、流内 error 与解码错误；`retry.go`：408/429/5xx 可重试、401/403 立即鉴权错误、500ms×2^(n-1) 上限 15s + 0–250ms 抖动、Retry-After 秒/HTTP-date 上限 60s、只重试拿到响应头之前；`engine.go`：`/chat/completions`、Bearer、`stream`+`include_usage`、事务性 `reasoning_content` 回填、300 秒空闲超时（可注入）、取消贯穿、`netproxy.Transport()`、出站校验。测试：新增 25 项（think 5、SSE 7、retry 4、engine 9：正常流、401 不重试、429+Retry-After、500 耗尽、半截流、空闲超时、取消、外部 http 拒绝、请求形状） | docs/PLAN-M2.md、internal/ai/{think,sse,retry,engine}.go 与测试、internal/contract/contract.go（`Usage`） | `go test ./internal/ai` 48 项通过 | R-4 的真实引擎复测移至 T20（需要前端流式与假供应商联调；T03 只有 httptest 层）；PAC/WPAD/SOCKS 仍未实测 | T04 |
| 2026-11-18 | T02 完成 | `internal/ai/agents.go`：4 个 Agent 固定顺序读写、D-010 默认值（flash/pro、high、scout auto）、缺键用默认而非法值归一（effort→auto、approval→ask，含 scout 也只往严格方向降级）、steering 存储截断 4000 字符与注入时 `clampSteering` 标注、skillPaths 归一/去重/500 上限、未知 agent 内字段保留；`ReadAgents`/`WriteAgents` 原子写 + 结尾换行 + 不转义 HTML。`ApplyAgentPatch`（M63，只改提供键、非法枚举拒绝、非法 skillPaths 返回 `path-escape`）、`ResetAgentSteering`（M64）。`BuildSkillCatalog`（M71）+ `Store.SkillDocuments`。`internal/service/agents.go` 三个绑定，`AppConfigGet` 起返回 `agents`。契约：`AppConfigResult` 增加 `agents`，新增 `AgentConfig`/`AgentPatch`/`SkillEntry`/`SkillCatalogResult`；API M62 扁平形状、M63 path-escape、M64 写盘、M71 索引规则同步。测试：ai 包 8 项（默认值、文件优先、缺键/非法值、字节与未知字段、patch、reset、clamp、catalog）+ service 3 项（patch 写盘与重读、reset 持久化、catalog 空/未知库） | docs/API.md、docs/数据库设计.md、internal/ai/{agents.go,agents_test.go}、internal/index/store.go、internal/service/{agents.go,appconfig.go,agents_test.go}、internal/contract/contract.go | `go test ./internal/ai ./internal/service` 通过（ai 23 项） | skillCatalog 的索引集成未单测（空 handle 路径已测）；T19 e2e 覆盖真实索引 | T03 |
| 2026-11-18 | T01 完成 | `internal/ai/{providers,models,keys}.go`：providers.json 读写（固定键序、`SetEscapeHTML(false)`、未知字段按排序保留、读取 50/保存 32、读取坏条目跳过而保存整单失败）、reasonix.toml 托管区正则迁移（空结果不写）、DeepSeek 预设；`fetchModels` 两种 kind（openai `/models`、anthropic 去尾 `/v1` 再拼 `/v1/models` + `x-api-key` + `anthropic-version`）、10 秒超时、2 MiB、去 `models/` 前缀、去重排序 500 条、错误先替换本次 key 再脱敏；`KeyStore`（2560 字节、禁换行、内存与 OS 凭据两种后端）。`internal/service/providers.go`：M67 Providers、M68 SaveProviders、M69 SetKey（活动库刷新）、M70 FetchModels（草稿 key 优先于已保存 key）。契约：新增 `ProviderDraft`/`ProviderCredentialState`/`ProvidersResult`/`FetchModelsResult`；删除 `credentialsPath`；M70 错误码按旧版；D-011 记录 4 项收紧；数据库设计 3.6 重名规则改正。测试：15 项 ai 包用例（含真实 OS 凭据一次性服务名）+ 3 项 service 用例（33 条拒绝、重名拒绝、失败不写盘、草稿 key 优先、401 映射） | docs/API.md、docs/数据库设计.md、docs/decisions.md、internal/ai/*、internal/service/{providers.go,service.go,providers_test.go,mutation_test.go}、internal/contract/contract.go | `go test ./internal/ai ./internal/service` 通过；`go build ./...` 通过 | PAC/SOCKS 仍未实测（T00 记录）；凭据文件路径字段删除需 UI 在 T17 跟进 | T02 |
| 2026-11-18 | T00 契约核对（Workflow：3 个只读视角 + 每条发现 1 个反驳者，10 个 agent）7 条发现、3 条确认，全为 minor，均已修：①M46 `note` 补「超长截断不拒绝」（旧版 `validate.ts:163-167` 默认 onOverflow）；②`selection` 表述改为「字段旧版已有（`context.ts:82`），新版新增的是入参」，纠正「新字段」的说法；③M49 补 `revision` 非负整数、`dirty` 严格布尔，否则 `invalid-input`（`register.ts:930-932`）。另修掉 §6「历史落盘」残留的 `sidecar`。 | docs/API.md、docs/PLAN-M2.md | Workflow `t00-contract-check`：checked=frames/context/consistency，raw=7，confirmed=3；`go build ./...` 与 `go vet ./spikes/...` 通过 | T01 待放行 | T01 |
| 2026-11-18 | T00 开始：用户批准计划并放行 T00（B-M2-01/02/03/05 确认，B-M2-06 早前确认，B-M2-04 撤回）。已补 docs/API.md §6 的 `ToolCallFrame`、`StatusFrame`、`UsageFrame`/`Usage`、`WritePreview`/`DiffLine`、`PermissionOption`，删去 `sidecar` kind；补 M37 `BlockResult`；补 M45–M49 的 7 个子结构与 `contextReportDocument` 的可选 `selection`（PRD 要求，旧版 `ipc/register.ts:922-935` 不传）。新增 `spikes/t00-smoke`：Secrets 用一次性服务名 `mkb-spike-<随机>` 验证 Set/Get/Delete 与 2561 字节拒绝，全部通过；netproxy 对 127.0.0.1 与 localhost 直连 200。PAC/WPAD 与 SOCKS5 本机无环境，记为未实测。README 加 M2 三条命令。 | docs/API.md、docs/PLAN-M2.md、README.md、spikes/t00-smoke/main.go | `go run ./spikes/t00-smoke`：secrets.set/get ok、oversize rejected、afterDelete gone；netproxy loopback ok 200、localhost ok 200、ok=true | 契约核对 workflow 结果待回填；PAC/SOCKS 未实测 | T01 |
| 2026-11-18 | 复审（主会话自审，用户要求）2 阻断 7 需改 4 小问题：**阻断** ①PRD 的 Agent 默认配置来自旧版死代码 `DEFAULT_AGENT_CONFIG`（`shared/agent.ts:289-297`，src 与 tests 零引用），旧版运行时 4 个都是 ask 加空模型；用户选择按 PRD 现写法，记为 D-010，PRD 升 v1.7、数据库设计 3.1 同步，模型存裸名；②旧版 NativeEngine 的回合循环没有任务负责 → T04 扩为「回合循环、会话与上下文预算」，新增 V-LOOP-1..5。**需改** `handoff_task` 依赖 ContextHub → 移到 T08，并补 actionLedger 回调；`set_aliases` 经 M66；§7 写路径表述与两条路径统一；B-M2-04 撤回（技术设计 2.1 已确认 netproxy），PRD 的 SOCKS 与环境变量口径改为与之一致；Secrets 测试隔离；每条消息的「复制 / 重新回答」进 T11；只读查看器进 T15；V-PROV-4 改写并新增 V-PROV-5、6。**小问题** WritePreview 的 lineDiff 指 M1 的 TS 版并移植为 Go；T12 依赖改为补 `BlockResult`；帧表删 `sidecar`；恢复入口与待决事项更新 | docs/PLAN-M2.md v0.3、docs/decisions.md D-010、docs/PRD.md v1.7、docs/数据库设计.md 3.1 | 旧版核对：`agent.ts:289` 全仓库只有定义；`NativeEngine.ts:189` 会话默认 ask；`appConfig.ts:330-367` 4 个 Agent 均为空模型 + auto + ask；`nativeProvider.ts:41-50` 精确匹配模型名；`ContextHub.ts:176` 无调用方；mygo `netproxy.go` 文档与 `secrets.go:58` | B-M2-01、02、03、05 | T00 |
| 2026-11-18 | 独立审查（task-reviewer，只读）1 阻断 8 需改 4 建议：**阻断** Agent 写路径语义错误——旧版 write/edit 走权限桥直写、不经变更计划，计划却写「一律经 MutationService」，且 M29 目前拒绝非 user origin → 已拆成两条既定路径，并把「扩展 M29/M30 接受 agent origin + 权限矩阵」并入 T05，新增 B-M2-05 请用户确认；**需改** M49 只有关窗弱化版（补完整语义并区分 WindowService.ReportDocument）、路由词表计数改为 56、agents.json 落盘默认与引擎兜底默认两层分开写、M37 是新建绑定（依赖 T00）、fetchModels 草稿 key 链路、anthropic 拉模型按旧版保留、性能测量口径写死、AC-AI-2 补「切 ask 后弹卡」；**建议** 权限审查点合并 T05+T06、idle timeout 可注入、search.db 版本迁移、Z1 列入 B-M2-05。均已落进计划 | docs/PLAN-M2.md v0.2 | 自查见第 10 节；旧版盘点 20 处 file:line 抽查 18 处属实（2 处已修正） | B-M2-01..05 | T00 |
| 2026-11-18 | 计划草案完成 | docs/PLAN-M2.md v0.1 | 自查见第 10 节；API §6 帧载荷缺失已确认 | B-M2-01..04 | T00 |

## 10. 自查

- 覆盖：第 1.2 节需求 → 任务/验收映射；PRD 第 12 节 AC-AI-1..6 全部映射到第 6 节（AC-AI-2 两段、AC-AI-5 双击、AC-AI-6 2400 截断均有对应 V 项），AC-SRCH/LINK/MUT 复用 M1 的验收。
- 接口一致：M38–M49、M63/M64、M67–M71 来自 API 2.2；帧载荷缺口在 T00 补齐；存储格式只引用数据库设计小节。
- 未定值：系统代理/PAC、Secrets 可用性、旧版词表与 steering 默认文本的精确内容，都在 T00/盘点中落实，不在此臆造。
- 已知风险：权限门与 key 是安全核心（专项审查）；R-4 需用真实引擎复测；D-002 三处行为按旧版判定，D-010 默认配置按 PRD 判定；T04 回合循环是最复杂的单项，单独有 V-LOOP；M2 规模最大，任务数 22。
