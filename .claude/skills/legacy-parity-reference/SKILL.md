---
name: legacy-parity-reference
description: Mile Knowledge Base（Go 重写版）把旧版 Electron 应用（E:\WebApp\Mile-Project\Mile-Knowledge-Base-master，只读）当作功能与 UI 对照基线的方法。覆盖旧版只读铁律、PRD 功能编号 → 旧版文件与单测的对照表、查旧版行为的顺序（PRD → shared 纯逻辑 → tests/unit → main/services → renderer）、并排对比 UI 的做法（能否本地跑、同一知识库样本与窗口尺寸截图、对比清单）、D-002 原样复刻与 D-004 不复刻的差异清单、旧版数据位置（.mile-kb/ 与 Electron userData）只读规则、性能对比基线的测法（PRD 10.1 / Q7）。本项目任何「对齐旧版」「与旧版一致吗」「旧版是怎么做的」「对比旧版」的任务都应加载；不涉及旧版对照的新功能设计不加载。
---

# legacy-parity-reference（Mile Knowledge Base Go 版）

旧版 = `E:\WebApp\Mile-Project\Mile-Knowledge-Base-master`（Electron 39 + React 19，v2.7.0）。它是功能与 UI 的对照基线；PRD `docs/PRD.md` v1.1 是从它盘点出的契约，decisions.md 记录哪些差异是有意的。

## 1. 何时加载 / 何时不加载

- 加载：实现某个 PRD 功能要核对旧版行为；UI 与旧版并排对比；排查「新版与旧版不一致」；定性能基线要测旧版；导入旧版设置或读旧版数据。
- 不加载：纯 Go / mygo / 前端工程问题（用 `go-service-engineering`、`mygo-web-app`）；与旧版无关的新设计；写代码本身不查旧版时。

## 2. 铁律

1. **旧版目录只读。** 任何任务都不修改、不写入旧版目录（包括 `node_modules` 之外的任何文件）；跑它也只为观察，不往里面写。
2. **PRD 与旧版代码冲突时先问用户**，不自行裁决；PRD 自己的记号是「旧版行为 / 新增约束 / 待确认」。
3. **以代码为准**：PRD 已注明「文档与代码冲突时取代码」（PRD 1.3）；发现 PRD 与旧版代码不一致时记录并回报，不自行改 PRD。
4. **数据也只读**：`.mile-kb/`、Electron userData 里的旧版文件只读取，不修改；导入是一次性读，不写回。

## 3. 功能 → 旧版文件对照

扩充 PRD 第 14 节。路径相对 `Mile-Knowledge-Base-master/src/`；测试在 `tests/unit/`（89 个文件）。AI 相关细节同时见 `agnes-engine` skill。

| PRD 功能 | 旧版主要文件 | 单测 |
| --- | --- | --- |
| FR-WIN 窗口/托盘/关窗 | `main/index.ts`、`main/windowCloseGuard.ts`、`main/services/TrayService.ts`、`ThemeService.ts` | `closeBehavior`、`windowCloseGuard` |
| FR-THEME 主题 | `main/services/ThemeService.ts`、`renderer/src/assets/tokens.css`、根 `DESIGN.md` | `fontStack` |
| FR-VLT 知识库 | `main/services/VaultService.ts`、`vaultScaffold.ts`、`RecentDocumentsStore.ts`、`shared/vault.ts`、`recentDocuments.ts` | `vaultService`、`vaultUnavailable`、`recentDocuments` |
| FR-AREA 功能区/图标 | `shared/vault.ts`（classifyTopLevel）、`shared/directoryIcons.ts` | `classification`、`directoryIcons`、`appIcons` |
| FR-TREE 文件树 | `main/services/VaultService.ts`（tree）、`shared/vault.ts`、`shared/entryName.ts`；渲染 `renderer/src/components/TreeNode.tsx` | `treeOrder`、`entryName` 相关 |
| FR-DOC 文档读写 | `main/services/textFile.ts`、`RevisionRegistry.ts`、`shared/noteText.ts`、`shared/lineDiff.ts`；编辑器 `MarkdownEditor.tsx` | `documentWrite`、`textFile`、`revisionRegistry` |
| FR-LINK 链接 | `shared/links.ts`、`readerLinks.ts`、`wikiNavigation.ts`、`remarkWikilinks.ts` | `links`、`linkGraph`、`readerLinks`、`remarkWikilinks` |
| FR-SRCH 检索 | `main/services/IndexService.ts`、`shared/search.ts`、`shared/tokenize.ts` | `indexService`、`tokenize`、`searchHighlight` |
| FR-MUT 变更/事务/日志 | `main/services/MutationService.ts`、`TransactionService.ts`、`AuditLedger.ts`、`shared/mutation.ts` | `mutationService`、`transactionService`、`auditLedger` |
| FR-PROV 来源追踪 | `main/services/ProvenanceService.ts`、`shared/provenance.ts` | `provenance`、`provenanceService` |
| FR-HLTH 健康检查 | `shared/kbHealth.ts` + IndexService | （并入 indexService 测试） |
| FR-OVW 总览 | renderer `OverviewGrid.tsx` + 各服务 | — |
| FR-SKILL 技能导入 | `main/services/skillImport.ts` | `skillImport` |
| FR-SET 设置 | `main/services/ConfigStore.ts`、`AppConfigStore.ts`、`ProviderStore.ts`、`shared/settings.ts`、`appConfig.ts`、`settingsDrafts.ts`、`providerForm.ts` | `configStore`、`appConfig`、`appConfigStore`、`settingsDrafts`、`providerForm` |
| FR-AI-ENG 引擎 | `main/engine/openaiClient.ts`、`streamAssembler.ts`、`thinkSplitter.ts`、`retry.ts`、`proxy.ts`、`endpoint.ts`、`contextBudget.ts`、`nativeProvider.ts`、`CredentialStore.ts`、`EngineHost.ts`、`NativeEngine.ts`、`engineServer.ts`、`rpc.ts` | `openaiClient`、`nativeEngine`、`contextBudget`、`engineHost`、`engineRpc`、`engineProvider`、`nativeProvider`、`proxy`、`credentialStore`、`modelCatalog` |
| FR-AI-AGT 工具 | `main/engine/kbTools.ts`、`main/agent/VaultFsBridge.ts`、`PermissionRouter.ts`、`shared/agentPolicy.ts` | `kbToolsError`、`agentPolicy`、`permissionRouter`、`pathAliasBypass`、`reasonixFsBridge` |
| FR-AI-ROUTE 路由 | `shared/agentRouting.ts`、`followUpRouting.ts` | `agentRouting`、`followUpRouting` |
| FR-AI-PERM 审批/模式 | `shared/executionMode.ts`、`confirmGate.ts`、`agentPolicy.ts`；审批卡 `shared/lineDiff.ts`、`main/engine/EngineHost.ts` | `executionMode`、`confirmGate`、`lineDiff` |
| FR-AI-CHAT 对话渲染 | `shared/transcript.ts`、`transcriptGroups.ts`、`turnGroups.ts`、`turnProcess.ts`、`toolLine.ts`、`chatStreaming.ts`、`chatReferences.ts`、`remarkChatReferences.ts`、`evidence.ts`；UI `AgentTranscript.tsx`、`ChatMarkdown.tsx` | `transcript`、`transcriptGroups`、`turnGroups`、`turnProcess`、`toolLine`、`chatStreaming`、`chatReferences`、`remarkChatReferences`、`evidence` |
| FR-AI-HIST 历史 | `main/services/ConversationStore.ts`、`ConversationRecorder.ts`、`ConversationRecap.ts`；`shared/conversation*.ts`（含 Groups/Recap/Restore/Search/Time/View） | `conversation*`（7 个）、`conversationRecorder` |
| FR-AI-PROV 供应商/key | `main/services/ProviderStore.ts`、`modelCatalog.ts`、`engine/CredentialStore.ts`；`shared/appConfig.ts`、`modelService.ts` | `modelCatalog`、`modelCatalogProxy`、`modelService`、`credentialStore` |
| FR-AI-USAGE 用量 | `shared/usage.ts`、`main/engine/contextBudget.ts`（addUsage） | `usageFormat` |
| FR-AI-STEER steering | `main/agent/steeringBlock.ts`、设置里 agents.json | `steeringBlock` |
| FR-AI-CTX 共享上下文 | `main/services/ContextHub.ts`、`main/agent/contextTail.ts`、`shared/context.ts` | `contextHub`、`contextTail` |
| 安全 / IPC 校验 | `main/security/`（pathContainment、redact 等）、`main/ipc/validate.ts` | `pathContainment`、`ipcValidate`、`navigationPolicy` |

## 4. 查旧版行为的顺序

1. **先 PRD**：该功能在 `docs/PRD.md` 的 FR-* 节与第 7/9 节状态表；区分「旧版行为」与「新增约束」。
2. **再 shared/ 纯逻辑**：`src/shared/` 是无 IO 的规则层（agentPolicy、agentRouting、executionMode、context、conversation、usage、tokenize、links、search、mutation、provenance、directoryIcons、lineDiff、turn*/transcript*/toolLine），最容易穷举读。
3. **再 tests/unit**：89 个测试文件是行为规格；看断言比看实现更快确认边界。
4. **再 main/services 与 main/engine**：IO 与编排（VaultService、IndexService、MutationService、Conversation*、EngineHost、NativeEngine）。
5. **最后 renderer**：布局与交互细节（`renderer/src/components/**`、tokens.css）。
6. **旧版 DESIGN.md**（根目录，45KB）：视觉权威，与 tokens.css 同地位；UI 对比以它 + tokens.css 为准，不看旧版实现样式。

## 5. 并排对比 UI

- **能否本地跑**：旧版 `package.json` scripts 有 `dev: electron-vite dev`、`start: electron-vite preview`、`test: vitest run`。`.claude/launch.json` 里只有两个 `python -m http.server` 配置，服务的是 `docs/proposals` 静态稿，**不是应用本身**。要跑应用需 `npm install` 后 `npm run dev`（Electron）。前提：本机有 Node 与依赖；只运行不修改——`npm install` 只写 `node_modules`，属于运行必需；不写其它文件。
- **对比条件**：同一知识库样本、同一窗口尺寸（默认 1440×900，最小 744×640）、同一主题档；新版也开到同尺寸再截。
- **对比清单**：布局结构（栏宽：侧栏默认 232、文档面板默认 560）、令牌数值（对照 tokens.css，不按像素猜）、交互状态（hover/禁用/选中）、文案（含空态、状态胶囊文字）、图标（phosphor 同一图标）。像素级差异只允许来自字体渲染（PRD G2）。
- **不对比的**：框架差异带来的实现细节（Electron vs WebView2 的渲染微调）、已决定不复刻的差异（见 §6）。

## 6. 「像缺陷但要复刻」与「不复刻」

**D-002 原样复刻**（PRD Q3/Q4，已确认）：

- manual 与 auto 执行模式都逐条弹审批卡，区别只在界面文案（`shared/executionMode.ts`）。
- 推理强度控件始终禁用：自研引擎 `availableEfforts` 恒空（`NativeEngine.ts:213-217`）。
- 4 个 Agent 的 system 提示回调存在但没有接上，差异只来自 steering（`NativeEngine.ts:107`）。
- 修复属 P1，需单独批准；本期不得顺手改。

**D-004 不复刻 / 有意差异**（已确认）：

- 检索改用 SQLite FTS5 + bm25，排序不要求与旧版逐条一致（PRD FR-SRCH v1.1）。
- Markdown 在 Go 侧 goldmark 渲染，前端不再 react-markdown（行为口径由 R-3 验证）。
- API key 上限 ≤2560 字节（`mygo.MaxSecretSize`），存 `mygo.Secrets`；旧版 safeStorage 的 key 读不出，用户需重填（PRD Q2）。
- 窗口底色以令牌 `--raw-canvas`（#eceef1 / #0e0f11）为准，不用旧版主进程里的 #F6F7F9 / #0B0F14（PRD Q6）。
- 对话历史不导入旧版（PRD Q2）；索引快照 `index-<vaultId>.json` 不读不迁移（PRD FR-SRCH）。
- 引擎在 Go 进程内运行，不实现旧版 EngineHost 的进程重启；单回合出错不拖垮应用（PRD FR-AI-ENG 新增约束）。

## 7. 旧版数据位置（只读）

- 知识库内 `.mile-kb/`：`settings/providers.json`、`settings/agents.json`、`settings/categories.json`、`settings/aliases.json`、`transactions/<id>/`、`provenance.json`、`drafts/`。新版与旧版格式兼容（PRD Q2(a)），但**读取**这些文件只为对照或导入，不修改。
- Electron userData（应用级）：`settings.json`（主题/关窗/布局/最近知识库/上次知识库）、`recent-documents.json`、`conversations/<vaultId>/index.json` + `*.jsonl`、`engine-credentials.json`（safeStorage 加密，新版读不出）。具体路径在阶段 07 R-7 确认。
- 读这些文件用于：一次性导入设置与最近文档（PRD 10.3）、对照行为、取样测试数据。一律只读；导入失败按默认值运行。

## 8. 性能对比基线（PRD 10.1 / Q7）

旧版没有实测基线，新版首个可运行版本在同一样本上测旧版。测法：

- 样本：阶段 07 定义并入库的「典型」（用户真实知识库规模）与「大」（10 000 md + 约 5000 文件代码目录）知识库。
- 冷启动：进程不在内存 → 侧栏首屏可点。
- 大库首屏：打开 → 树首屏显示时间；索引期间界面是否可操作。
- 检索：输入停止 → 结果显示，p95。
- 外部改动：脚本一次改 1000 个文件，量界面可操作性与同步耗时（对应 P-1）。
- 流式：长对话生成期间量帧率与按键延迟（P-3/P-5；旧版已知单条 9832 字每帧 6.7–9.9ms、50 条长回答历史查看器 172–189ms，见 PRD 1.2、10.1）。
- 长列表：树单目录 10 000 项、搜索 200 条、操作日志 200 条的滚动帧率。
- 内存与包体：空闲与打开典型库后的进程总内存、安装包体积。
- 测旧版时与新版用**同机、同样本、同窗口尺寸**；记录方法写进当次 PLAN，结果作为 Q7 校准依据回报，不预设目标达成。

## 9. 交付前自检

- [ ] 没修改旧版目录、`.mile-kb/`、userData 里的任何文件。
- [ ] 行为结论引用了 PRD 编号或旧版 file:line；PRD 与代码冲突已记录并回报而不是自行裁决。
- [ ] UI 对比用了同一样本库、同一窗口尺寸、同一主题；令牌以 tokens.css / DESIGN.md 为准。
- [ ] 没把「像缺陷」的 3 处 D-002 当成新版要修的问题；没把 D-004 的有意差异当成不一致上报。
- [ ] 性能基线在同机同样本下测，方法与数值写进 PLAN；未实测的不标「已达标」。
