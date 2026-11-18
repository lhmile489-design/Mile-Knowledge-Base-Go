# M2 实施计划：AI 引擎、Agent、对话与设置（模型服务、Agents）

> 状态：草案 v0.2（2026-11-18，已按独立审查修订），待用户审批。审批前不执行第 4 节的任何任务。
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
| mygo v0.10.0 能力 | Secrets、netproxy、Channel、Event、Clipboard 已在 M0/M1 核对 | **Secrets 的实际存储后端、netproxy 的系统代理/PAC 行为未在本项目实测**（技术设计 R-5 标注待验证） | 阻断 T01/T03，各自第一步先验证 |
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
| T00 | 契约补齐与实测前置 | docs/API.md §6/§4.6–4.8、README | M1 | 帧载荷类型、M45–M49 字段、错误码核对；Secrets 与代理的验证脚本 | API 定义完整且与实现字段一一对应；Secrets 存取、netproxy 系统代理/PAC 各有「能/不能」结论并记录 | 待做 |
| T01 | 供应商与 key（M67–M70） | `internal/ai/providers.go`、`internal/service/settings.go`（M67–M70）、`internal/ai/providers_test.go` | T00 | providers.json 读写、reasonix.toml 只读迁移、DeepSeek 预设、fetchModels、key 存取 | V-PROV 全部通过 | 待做 |
| T02 | Agent 配置与 steering（M63、M64、M71） | `internal/ai/agents.go`、`internal/service/settings.go`、`internal/index`（skillCatalog 只读） | T01 | agents.json 字节兼容、默认配置、steering clamp/基线、skillCatalog | V-AGT 全部通过 | 待做 |
| T03 | 引擎骨架与 SSE 流式 | `internal/ai/{engine.go,sse.go,retry.go,usage.go}` | T01、T00 | OpenAI 兼容客户端、SSE 组装、重试/退避/Retry-After、空闲超时、出站校验、netproxy | V-ENG 全部通过（httptest 假供应商） | 待做 |
| T04 | 上下文预算与压缩 | `internal/ai/budget.go`、`compress.go` | T03 | token 估算、真实用量校准、80% 压缩、摘要与整轮丢弃 | V-BUDGET 全部通过 | 待做 |
| T05 | 工具、权限门与 Agent 写路径 | `internal/ai/tools/*`、`internal/ai/{policy.go,bridge.go}`、`internal/mutation/{plan.go,execute.go}`（接受 agent origin）、`internal/service/mutation.go` | T03、M1 | 工具 schema、两条写路径（直写桥 / 变更计划）、两道判定、系统区禁写、60 秒批准、D-005；M29/M30 支持 agent origin 并挂权限矩阵 | V-TOOL、V-PERM 全部通过；**权限专项测试**（提示词无法扩大权限） | 待做 |
| T06 | 执行模式与审批卡（M41、M42） | `internal/ai/approval.go`、`internal/service/agent.go`（M39–M43） | T05 | 3 档模式、perm 帧状态机、expired、差异预览 | V-APPROVAL 全部通过 | 待做 |
| T07 | 路由与改派（M44） | `internal/ai/{routing.go,priority.go}` | T03 | 词表原样迁移、优先级、理由文本 | V-ROUTE 全部通过 | 待做 |
| T08 | 共享上下文（M45–M49） | `internal/ai/context.go`、`internal/service/agent.go` | T03 | 快照、pin/unpin、任务状态、当前文档上报、contextTail 2400 预算与脱敏 | V-CTX 全部通过 | 待做 |
| T09 | 对话历史与搜索（M50–M59） | `internal/conversation/{store.go,events.go,search.go}`、`internal/service/conversation.go` | T03、T06 | index.json/jsonl、上限、损坏恢复、search.db FTS5、导出、接着问 | V-HIST 全部通过 | 待做 |
| T10 | 绑定层与流式通道 | `internal/service/agent.go`（M38–M43 全部）、`src/lib/{stream.ts,agentApi}` | T03–T09 | StreamFrame TS、Channel 前端封装、E07/E08/E09/E10 | 契约测试：Bind 集合 = 前端调用集合 | 待做 |
| T11 | 对话界面骨架 | `src/components/chat/{ChatPane,MessageList,AgentMessage,UserMessage,Header}`、`src/stores/agentStore.ts` | T10 | 头部 44px、消息布局、过程流水线、思考折叠、呼吸动画 | V-CHAT 全部通过；截图 37、38、39 | 待做 |
| T12 | Markdown 流式与引用 | `internal/render`（M37 按块增量，新增）、`src/components/chat/{MarkdownBlock,EvidenceRadar}` | T00（M37 契约）、T11 | 末尾未封口块收起、宽表横向滚动、代码复制、引用标签、证据雷达 | V-CHAT-MD 全部通过 | 待做 |
| T13 | 输入框、运行参数条、路由/改派 | `src/components/chat/{Composer,RunParams,RouteLine,AgentPicker}`、`src/stores/composerStore.ts` | T11、T07 | Enter 发送、高度持久化、32 000 上限、参数条、改派菜单 | V-COMPOSER 全部通过；截图 34、35、36 | 待做 |
| T14 | 审批卡与用量收据 | `src/components/chat/{PermissionCard,UsageReceipt}` | T06、T13 | 差异预览（60 行）、允许/拒绝、Go 确认后移除、收据未知显示「—」 | V-CARD-AI 全部通过；截图 42、43 | 待做 |
| T15 | 历史栏与恢复 | `src/components/chat/HistoryRail`、`src/stores/historyStore.ts` | T09、T11 | 分组、搜索（250ms）、重命名/置顶/删除/清空、导出、接着问、恢复提示 | V-HIST-UI 全部通过；截图 45、46 | 待做 |
| T16 | 会话信息浮层与引擎状态 | `src/components/chat/SessionInfo`、`EngineBadge` | T08、T10 | 引擎状态、模型诊断、读写边界、权限摘要、共享上下文 | V-INFO 全部通过；截图 40、44 | 待做 |
| T17 | 设置页「模型服务」 | `src/components/settings/{ProvidersSection,ProviderForm}` | T01、T00 | 供应商列表/表单、拉模型、key 状态、空状态 | V-SET-PROV 全部通过；截图 41、49、50 | 待做 |
| T18 | 设置页「Agents」 | `src/components/settings/{AgentsSection,SteeringEditor}` | T02 | 模型/强度/模式/Skills 即时生效、steering 草稿与保存条、对比与恢复默认 | V-SET-AGT 全部通过；截图 51 | 待做 |
| T19 | 假供应商 e2e 与闭环验收 | `internal/ai/fake_provider_test.go`、`scripts/e2e/ac-m2.sh` | T03–T18 | 假 OpenAI 供应商、AC-M2 全流程、重启恢复 | AC-M2 全部通过 | 待做 |
| T20 | 性能复核与独立审查 | `scripts/e2e/perf-ai.sh`、本计划第 6 节 | T19 | 流式帧率、输入延迟 p90、历史首帧、消息虚拟化 | 指标有实测；**整体独立审查**（权限门重点） | 待做 |
| T21 | 收尾 | README、CLAUDE.md、本计划 | T20 | 文档同步；推送前确认 | 用户确认后推送 | 待做 |

依赖顺序 T00 → T01 → T02 → T03 → T04/T05/T06/T07/T08 各自可依序进行（按「一次一个」串行）→ T09 → T10 → T11–T18 → T19 → T20 → T21。

## 5. 单任务精确交接

> 通用纪律：Go 业务逻辑先写失败测试；`.mile-kb/` 下 agents.json/providers.json 按数据库设计 3.1/3.6 + 3.0.1 字节兼容；对话历史按 4.4；**Agent 写知识库只有两条既定路径**——`write_note`/`edit_note` 走权限桥直写（与旧版 `VaultFsBridge.ts:205-240` 一致，不开事务、不记操作日志），`mutate_notes` 走 MutationService 的变更计划与执行；错误码按 API §1.3；中文 UI、无 emoji；提交按 M0 检查清单。

### T00 契约补齐与实测前置

- API.md §6：补 `ToolCallFrame{id,title,kind?,status,detail?}`、`StatusFrame{state,phase?,model?,effort?,detail?}`、`UsageFrame{turn,cumulative}` 与 `Usage` 字段（对齐旧版 `usage.ts` 的 token/费用/缓存结构，未知可空）、`WritePreview`（沿用 T10 的 lineDiff 形状，Go 侧新增实现）、`PermissionOption{id,label}`。
- API.md §4.6：补 M45–M49 的入参校验与出参完整字段；核对 M38 的 `capabilities` 固定值。
- 两个实测（各写一次性 Go 测试或 spike，结论写进第 9 节）：
  1. `mygo.Secrets`：Set/Get/Delete 在 Windows 上可用；超过 2560 字节拒绝；不可用时返回 `ErrUnsupported`。
  2. `mygo` netproxy：系统代理（含 PAC）读取方式；回环地址是否绕过；SOCKS 是否回落直连。若能力不足，记录降级方案（直连 + 明确提示），不静默。
- README 增加 M2 会用到的命令占位。

### T01 供应商与 key

- providers.json 读写按 3.6：整文件 `SetEscapeHTML(false)`、固定键序、未知字段保留；读取截 50、保存校验 32；同名忽略大小写去重；`reasonix.toml` 托管区只读迁移（不存在时才读，结果为空不写文件）。
- DeepSeek 预设常量；`fetchModels`：openai 类型 `GET {baseUrl}/models` + Bearer；anthropic 类型按旧版 `modelCatalog.ts:55-59`（`GET {root}/v1/models` + `x-api-key` + `anthropic-version`，仍是只存不调聊天）；10 秒超时、响应 ≤2 MiB、解析 `data[].id`、去 `models/` 前缀、≤500 条排序；错误信息先替换 key 再脱敏。
- key：`Secrets.Set(envName, value)`，≤2560 字节、不含换行；界面状态由「能否读到」决定，不返回明文。
- **未保存草稿的 key**：`fetchModels` 的 `apiKey?` 参数承接编辑器里刚粘贴、尚未保存的明文，只在该请求内使用，不落盘、不进日志；T17 的「测试连接/拉模型」用它。
- 测试：移植 providerForm、modelCatalog、credentialStore 用例；新增伪造 key 不落日志。

### T02 Agent 配置与 steering

- agents.json 按 3.1：4 个键固定顺序、未知顶层键丢弃、未知字段保留；**两层默认要分清**：①写入 agents.json 的落盘默认按 `shared/appConfig.ts:330-374`（model=''、effort='auto'、approval='ask'，4 个 Agent 全 ask）；②引擎在无配置时的展示/兜底默认按 `shared/agent.ts:293-296`（effort='high'、scout='auto' 其余 'ask'）。PRD 的「默认配置」一段指后者，实现时两层各按其来源，不混用；非法 effort 归 auto、非法 approval 降级 ask。
- steering：≤4000，超出截断并标注；`steeringBaseVersion` 保留；「与默认对比」「恢复默认只改草稿」由前端状态实现，后端提供默认值接口（M64 reset 返回默认，不落盘）。
- skillCatalog（M71）：由索引文档派生（skillPaths 指向的目录）；只读。
- 测试：移植 appConfig/agents 相关用例 + 字节往返。

### T03 引擎骨架与 SSE

- 出站校验：https 任意、http 只回环、URL 禁凭据；用 netproxy；按 T00 结论处理 SOCKS/PAC。
- SSE：`delta.content`/`reasoning_content`/`reasoning`、`<think>` 拆分、tool_calls 按 index 累积（无 id 补 `call_<index>`）、`[DONE]`/finish_reason 完整性、半截流报错。
- 重试：仅响应头前；408/429/5xx；401/403 立即鉴权错误；其他 4xx 立即失败；最多 10 次，500ms×2^(n-1) 上限 15s + 0–250ms 抖动；遵守 Retry-After ≤60s；状态帧文案「第 n/m 次重试，x 秒后自动继续」。
- 空闲：300 秒无字节中止（超时值可注入，便于测试缩短）；取消贯穿请求（ctx 取消 → 关连接）。
- 测试：httptest 假供应商覆盖 401/429(Retry-After)/5xx/半截流/超时/取消；**R-4 复测**：真实引擎 + 真实 Markdown，记录流式性能。

### T04 上下文预算

- 估算：ASCII/4 + 非 ASCII×0.7 + 每消息 4；用真实 prompt_tokens 校准系数 0.5–3；窗口默认 64 000，供应商可覆盖。
- 80% 触发压缩：保留最近 ~16%，切点只在 user 消息前；旧内容由模型摘要；失败或仍超限 → 整轮丢最旧 + 截断说明；system 与工具定义计入。
- 测试：移植 contextBudget 全部用例 + 摘要失败路径。

### T05 工具与权限

- 8 个工具 schema 用 agnes-engine skill 的 kb-tool-schemas，并逐字段对照旧版 `engine/kbTools.ts:86-234`；描述中文；参数校验失败以错误文字回模型，不中断回合；结果截断 20 000。
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
- 差异预览：Go 侧 lineDiff（LCS 4M 格、60 行上限、单行 200 字符），读取原文失败显示摘要。
- 测试：移植 permissionRouter/executionMode 用例 + 卡片状态机 + 批准过期 + 消费一次。

### T07 路由

- 词表原样迁移（`shared/agentRouting.ts:44-119`，curator 21 / scout 18 / distiller 17 共 56 词）；小写包含匹配、不分词；优先级 scout > general > distiller > curator（21-26）；理由文本 `explainRoute` 同款；路由预览纯函数。
- 测试：移植 agentRouting 用例 + 边界（多类命中、空输入）。

### T08 共享上下文

- 快照字段与 M45 一致（`SharedContext{snapshotId,sequence,vault,activeDocument?,pinnedReferences,taskBoard,decisionLog,actionLedger,retrievalBundle}`）；任务板/决策/检索片段由 Agent 写，任务状态只能改自己那条。
- **M49 需完整实现**：`AgentService.ContextReportDocument(vaultID,path?,revision,dirty)` 写入 ContextHub 的 activeDocument（含 revision 与可选 selection）；M0 的 `WindowService.ReportDocument` 保留为关窗守卫的专用入口，两者职责在契约里写明，不互相替代。
- contextTail：2400 字符预算，优先级 revision → 当前文件/未保存/选区≤200 → 固定引用（≤1500）→ 未完成任务 → 决策 → 检索片段（≤4，放不下整条丢）；省略时「另有 N 项未列出」；绝对路径脱敏（旧版 `agent/contextTail.ts:67-186`）。
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
- `scripts/e2e/ac-m2.sh`：真实窗口 + 假供应商配置 → 发消息 → 路由 → 流式 → 工具审批 → 允许 → 收据 → 重启恢复 → 历史搜索。
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
| V-PROV-4 | 无 key 环境 | 发请求 | 引擎 offline，原因如实；编辑器检索照常 | e2e |
| V-AGT-1 | 修改 steering 4001 字符 | 保存 | 截断并标「已截断」 | Go |
| V-AGT-2 | 非法 effort/approval | 读取 | 归 auto / 降级 ask | Go |
| V-AGT-3 | agents.json 含未知字段 | 读写往返 | 未知字段保留、键序稳定 | Go |
| V-ENG-1 | 假供应商 401 | 发消息 | 立即鉴权错误，不重试 | httptest |
| V-ENG-2 | 429 + Retry-After | 发消息 | 按 Retry-After 等待并重试，状态帧可见 | httptest |
| V-ENG-3 | 半截流 | 发消息 | 回合报错，不写入半截回答 | httptest |
| V-ENG-4 | 300 秒无字节 | 发消息 | 中止并报超时（用缩短配置测试） | httptest |
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
- 写知识库只经 MutationService；key 只进 Secrets；日志不记正文与 key。
- 提交：每个任务一次，不推送；M2 末询问。

## 8. 来源基线与批准记录

| 资料 | 版本 / 日期 | 适用范围 |
| --- | --- | --- |
| PRD | v1.6 | FR-AI-*、FR-SRCH/LINK/MUT/AREA 的复用 |
| API | v1.0 + T00 补齐 | M38–M49、M63/M64、M67–M71、§6 |
| 数据库设计 | v1.0 | 3.1、3.6、4.4、§6–8 |
| 技术设计 | v1.0 | 第 4.6、第 7 节 R-4/R-5、第 10 节门禁 |
| decisions | D-001–D-009 | D-002、D-005 |
| 旧版代码 | v2.7.0，只读 | 2.1 盘点 |
| Skills | agnes-engine 等 | 第 2 节 |

| 批准标识 | 内容 | 依据和时间 | 状态 |
| --- | --- | --- | --- |
| B-M2-01 | M2 范围 = C6 + C7 + 设置两节 | — | 待确认 |
| B-M2-02 | 第 1.3 节非目标、第 4 节任务与顺序、第 7 节 TDD 例外与审查安排 | — | 待确认 |
| B-M2-03 | 执行节奏沿用「一次一个任务」 | — | 待确认 |
| B-M2-04 | 代理能力不足时降级直连并提示（T00 结论触发） | — | 待确认 |
| B-M2-05 | Agent 写路径按旧版：`write_note`/`edit_note` 走权限桥直写（不开事务、不记日志），`mutate_notes` 走 M29/M30；同时扩展 M29/M30 接受 agent origin | — | 待确认 |

## 9. 恢复入口与执行账本

### 下次会话先读

- 当前阶段：M2 计划草案，待审批。
- 下一个任务：审批后 T00。
- 工作区：`main` 与 `origin/main` 对齐于 `cdc7a21`，工作区干净。
- 待决事项：B-M2-01..04；13 项手动确认；旧版性能基线。

| 时间 | 任务与状态变化 | 文件 / 版本范围 | 验证与证据 | 待决事项 | 下一任务 |
| --- | --- | --- | --- | --- | --- |
| 2026-11-18 | 独立审查（task-reviewer，只读）1 阻断 8 需改 4 建议：**阻断** Agent 写路径语义错误——旧版 write/edit 走权限桥直写、不经变更计划，计划却写「一律经 MutationService」，且 M29 目前拒绝非 user origin → 已拆成两条既定路径，并把「扩展 M29/M30 接受 agent origin + 权限矩阵」并入 T05，新增 B-M2-05 请用户确认；**需改** M49 只有关窗弱化版（补完整语义并区分 WindowService.ReportDocument）、路由词表计数改为 56、agents.json 落盘默认与引擎兜底默认两层分开写、M37 是新建绑定（依赖 T00）、fetchModels 草稿 key 链路、anthropic 拉模型按旧版保留、性能测量口径写死、AC-AI-2 补「切 ask 后弹卡」；**建议** 权限审查点合并 T05+T06、idle timeout 可注入、search.db 版本迁移、Z1 列入 B-M2-05。均已落进计划 | docs/PLAN-M2.md v0.2 | 自查见第 10 节；旧版盘点 20 处 file:line 抽查 18 处属实（2 处已修正） | B-M2-01..05 | T00 |
| 2026-11-18 | 计划草案完成 | docs/PLAN-M2.md v0.1 | 自查见第 10 节；API §6 帧载荷缺失已确认 | B-M2-01..04 | T00 |

## 10. 自查

- 覆盖：第 1.2 节需求 → 任务/验收映射；PRD 第 12 节 AC-AI-1..6 全部映射到第 6 节（AC-AI-2 两段、AC-AI-5 双击、AC-AI-6 2400 截断均有对应 V 项），AC-SRCH/LINK/MUT 复用 M1 的验收。
- 接口一致：M38–M49、M63/M64、M67–M71 来自 API 2.2；帧载荷缺口在 T00 补齐；存储格式只引用数据库设计小节。
- 未定值：系统代理/PAC、Secrets 可用性、旧版词表与 steering 默认文本的精确内容，都在 T00/盘点中落实，不在此臆造。
- 已知风险：权限门与 key 是安全核心（专项审查）；R-4 需用真实引擎复测；D-002 三处行为需按旧版判定；M2 规模最大，任务数 22。
