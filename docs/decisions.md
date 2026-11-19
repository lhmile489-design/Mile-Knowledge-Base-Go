# 重要决策记录

> 状态：已有 8 条采纳决策（D-001 至 D-008），2026-11-18。
> 仅记录以后需要解释“为什么这样选”的重要取舍，不追求记录数量。

少量决策可直接写在 PRD 或技术设计的相应章节；无需为了本文件存在而虚构决策。

## 使用规则

- 提议、已采纳、已否决、已替代分开，不把草案自动升级为结论。
- 采纳后更新相关权威正文，再记录原因、代价和影响。
- 被替代的记录保留历史并指向替代项，不继续作为隐藏的现行规则。
- 不保存完整对话、真实密钥或无关实现流水。
- 本文不是 agent 自主裁决的授权。记录为“建议”或“裁决”不能替代用户确认，也不能自行接受阻断风险。
- 质量门禁例外统一在技术设计维护批准、范围、责任人与到期条件；此处只引用原因，避免重复维护批准状态。

## D-001：用 mygo + React 重写旧版，功能与 UI 对齐

- 日期与状态：2026-11-18，已采纳（用户明确指示）。
- 背景：旧版 Mile Knowledge Base（Electron 39 + React 19，v2.7.0）功能完整，但经常卡死、性能低。
- 结论：后端改用 Go + egoist/mygo（系统 WebView2），前端用 React；功能、UI/UX、布局与旧版一致。
- 原因：用户决定。代码层根因分析（docs/PRD.md 1.2）显示，主要卡顿来自主进程里串行的全量 IO、序列化和链接重算，以及前端的全量重渲染。换栈本身不能自动消除这些问题，所以 PRD 第 10 节另立了性能约束。
- 代价：旧版的 Electron 专属能力（safeStorage、utilityProcess、session 代理解析、托盘）都要在 mygo 上重新实现或找替代，阶段 07 逐项验证。
- 受影响：PRODUCT.md、docs/PRD.md、docs/技术设计.md（阶段 03）、Skills（阶段 04）。
- 未决：具体版本、目标平台、数据互通方式见 PRD 第 13 节。

## D-002：旧版的疑似缺陷本期原样复刻

- 日期与状态：2026-11-18，已采纳（用户确认 PRD Q3 / Q4）。
- 背景：盘点旧版代码时发现 3 处行为与界面表达不符。
  - 手动和自动执行模式都逐条弹审批卡，只有界面文案不同（`shared/executionMode.ts`）。
  - 推理强度控件始终禁用，因为自研引擎从不声明可用档位。
  - 4 个 Agent 的 system 提示回调存在但没有接上，差异只来自 steering（`main/engine/NativeEngine.ts:107`）。
- 备选：本期顺手修复；或原样复刻、修复另批。
- 结论：原样复刻。这 3 项在 PRD 中记为旧版行为，修复列为 P1，需要单独提出需求并获批准。
- 原因：重写的首要目标是对齐和性能；同时改行为会让「与旧版一致」失去对照基准。
- 代价：新版首发时仍带着这 3 处用户可感知的不一致。
- 受影响：docs/PRD.md 的 FR-AI-PERM、FR-AI-AGT、FR-AI-PROV。
- 未决：P1 修复的具体规则待日后提出。

## D-003：只做 Windows 11；库内数据兼容，应用数据部分导入

- 日期与状态：2026-11-18，已采纳（用户确认 PRD Q1 / Q2）。
- 结论：
  - 目标平台只有 Windows 11。
  - 知识库内 `.mile-kb/` 的格式与旧版兼容，新旧版可以轮流打开同一个库。
  - 首次启动时一次性导入旧版应用设置和最近记录。
  - 对话历史不导入；API key 由用户重新填写。
- 原因：
  - 旧版只在 Windows 上实测过。
  - 旧版 key 用 Electron safeStorage 加密，新版无法可靠解密。
  - 不导入历史可以省掉旧格式的长期兼容负担。
- 代价：换版本后旧对话在新版里看不到（旧版仍可查看）；需要重新配置 key。
- 受影响：docs/PRD.md 第 4.3、10.3 节，阶段 08 的数据设计。

## D-004：阶段 03 的 4 项关键选型

- 日期与状态：2026-11-18，已采纳（用户选择）。
- 标题栏：无边框窗口（mygo `Frameless`），自绘最小化、最大化、关闭按钮，与旧版一致。代价是没有 Windows 11 悬停最大化按钮时弹出的贴靠布局。
- Markdown：在 Go 侧用 goldmark 渲染 HTML，并用 bluemonday 清理。前端不再用 react-markdown。
  - 原因：解析移出 UI 线程；对话可以按块增量渲染，从根上解决 PRD P-3。
  - 代价：旧版的 React 渲染组件（wikilink 跳转、引用标签、图片标签、代码复制、宽表格）要改成「Go 输出带 data 属性的 HTML + 前端事件委托」，与旧版的一致性由 R-3 验证。
  - 建议方案原为「前端 react-markdown」，用户选择 Go 侧。
- 检索：用 SQLite FTS5（modernc.org/sqlite，纯 Go），排序不要求与旧版一致，以性能为先。
  - 用户原话：「不一定要和原版一致，尽量使用 go 语言，提高性能」。
  - 代价：旧版的打分公式不再逐条复刻；中文要先在 Go 侧切成二元组再写入。
- API key：存在 `mygo.Secrets`（Windows 凭据管理器），上限从 ≤4096 字符改为 ≤2560 字节。
- 受影响：PRD v1.1（FR-SRCH、FR-AI-PROV），docs/技术设计.md 第 2 节；`markdown-render-pipeline` skill 改写时以本条为准。

## D-005：全自动模式下 Agent 的永久删除直接拒绝

- 日期与状态：2026-11-18，已采纳（用户确认）。
- 背景：PRD v1.1 写的是「全自动下永久删除仍弹出单独确认」。阶段 04 核对旧版代码时发现，旧版并不弹卡。`EngineHost.ts:731-741, 815-827` 要求一张「发卡时不是全自动」的消费型批准，而全自动模式下引擎从不发卡，所以永久删除必然被拒绝，并以「永久删除需要本次单独确认」回给模型。
- 结论：按旧版代码执行。PRD 升为 v1.2，修改 FR-AI-PERM、第 3 节角色表、AC-AI-2；PRODUCT.md 的原则表述同步修改。
- 原因：与「功能和旧版一致」的目标一致，也更严格。
- 代价：全自动模式下用户想让 Agent 永久删除，需要先切到手动或自动模式。

## D-006：阶段 05 截图核对后的 3 项行为决定

- 日期与状态：2026-11-18，已采纳（用户确认）。
- 背景：在沙箱中运行旧版截图（`design/baseline/`），发现 3 处与 PRD 描述不同。
- 结论：
  - 所有已登记知识库都断开时：**新版改为显示欢迎页**，已断开的知识库以故障卡显示。旧版此时进入工作台的「知识库不可用」空壳（旧版 `App.tsx:101`）。这是本期唯一一处有意与旧版不同的界面行为，由用户直接决定，纳入本期范围。
  - 库内图片与相对链接以**知识库根目录**为基准解析，与旧版一致（旧版 `imageSource.ts:152-180`）。粘贴图片插入的 `raws/attachments/...` 路径也是相对库根，所以旧版内部是自洽的。
  - 阅读模式把文档开头的 YAML frontmatter 当正文渲染：**原样复刻**（与 D-002 同一口径），修复列为 P1 另批。
- 受影响：PRD v1.3（5.1、FR-VLT、FR-DOC、FR-LINK），docs/UI设计稿.md 第 7 节，markdown-render-pipeline skill（渲染管线不挂 frontmatter 扩展）。

## D-007：阶段 07 风险实测后的技术结论

- 日期与状态：2026-11-18，已采纳（实测结论；其中渲染两项由用户确认）。证据在 `spikes/` 各目录的 RESULT.md。
- 文件监听（R-1）：采用 `mygo.WatchFiles` 普通模式，加 `Ignore(node_modules/.git)`，`MaxEntries` 设为 65536。实测 15001 个文件的样本：外部改 1000 个文件，1.87 秒内全部识别、零丢失；首次快照 7.7 秒，在后台进行。备选 fsnotify v1.10.1 的公开 API 不支持递归，放弃。
- 检索（R-2）：采用预切词加 unicode61，FTS5 不挂 external content。1 万篇样本 p95 为 10.6ms，首建 1.33 秒，增量 1000 篇 204ms。片段、高亮与匹配字段在 Go 侧计算，因为 modernc 没有编译 snippet、highlight、offsets。链接解析必须用哈希索引；spike 里线性扫描花了 23.8 秒，不作为基线。
- 渲染（R-3）：goldmark 方案与旧版输出对照，一致 76 例，有意差异 30 例，结构差异 0 例。用户决定：
  - 原始 HTML 转义成文字显示，与旧版一致。之前文档写的「丢弃」是错的，PRD 改为 v1.4。
  - 标题不生成 id，与旧版一致。
- 流式与窗口（R-4、R-5、R-6、R-8）：
  - 50 条历史加 1 万字流式，输入延迟 p90 ≤ 10.3ms，没有超过 50ms 的长任务。
  - 无边框窗口的默认与最小尺寸、自绘按钮、首帧底色都通过。
  - 托盘与关窗询问框的机制成立；按钮索引与复选框返回值等用户手动确认（spikes/r4-window/RESULT.md 第 4 节）。
- 受影响：docs/技术设计.md 第 2–10 节、PRD v1.4、markdown-render-pipeline / kb-index-sqlite / mygo-web-app / mygo-app-scaffold 四个 skill。

## D-008：设计交接检查后的契约修订（用户 3 项决定 + 9 项规则澄清）

- 日期与状态：2026-11-18，已采纳（用户确认）。
- 背景：阶段 08 设计交接检查发现 API.md / PRD / 数据库设计 之间有 9 处不一致或语义含糊（A–I），连同 3 个未决问题一起交用户裁决。
- 用户决定（3 项）：
  - 供应商数量：保存时最多 32 个（旧版 `validate.ts:618`），读取 providers.json 时最多保留 50 个（旧版 `ProviderStore.ts:36`）。两个数字都来自旧版代码，32 是界面保存实际生效的上限。PRD FR-AI-PROV、API.md §9 Q1、M68 行按此改正。
  - 对话历史搜索：对话正文写入 SQLite FTS5，与知识库索引同一套预切词 + unicode61，全文检索、不读对话文件。放独立的 `conversations/<vaultId>/search.db`（索引库可删重建，但对话 FTS 的源是 jsonl 不是索引库，故不并入 `index/<vaultId>.db`；损坏时扫 jsonl 重建）。数据库设计 §4.4、Q-A 按此关闭；PRD FR-AI-HIST 补「搜索使用 FTS5」。
  - 图片协议名：`kbimg`（Windows 页面中为 `http://kbimg.localhost/...`）。API.md §5、§9 Q2、技术设计 §5 按此关闭。
- 交接检查引起的规则澄清（逐条核实旧版代码后写入契约）：
  - 引擎状态：枚举统一为 `starting | ready | offline`（旧版 `SidecarPhase` agent.ts:64 去掉 `restarting`——进程内引擎无进程级重启）；`offline` 带 `message` 覆盖「未配置」与「出错」（旧版未配置供应商是回合内报错 nativeProvider.ts:36，新版升级为引擎状态）；`capabilities` 字段旧版真实存在（agent.ts:71-86），新版为无握手的固定能力集；`restarts` 字段删除。API.md M38 与 E10 共用同一定义，PRD §7 引擎行同词。
  - 操作日志：单文件跨全部知识库（AuditLedger.ts `audit.json` 在 userData 根），按记录内 `vaultId` 过滤（:124）；移除知识库不清审计（register.ts:387-394 不触碰 AuditLedger）。数据库设计 §4.3、§7 写明。
  - 变更计划「待确认/执行中」是前端状态（mutationStore `pending`/`running`），不经 API 暴露；执行中再次提交靠前端禁用 + `expectedSourceRevision` 复核；执行中关窗按部分失败同路径恢复。
  - 审批卡：`PermFrame.status` 显式状态机 pending/approved/rejected/expired，前端收到 Go 的确认帧后才移除卡片（与旧版 agentStore.ts:368-389「IPC 成功才移除」同语义）；失效推 `status=expired` 帧。
  - 批准凭证的键：审批卡按 `requestId` 发卡（EngineHost.ts:207-215 writeApprovals）；批准后 `bridge.allowWriteOnce(sessionId, path)` 放行一次写，`mutationApprovals` 按 `sessionId+path` 记 60 秒批准（:221-225, :724-728）；`permanentDelete` 要求发卡时非 yolo 且目标一致的消费型批准（:733-743）。
  - 外部修改提示：与旧版等价——`vault-changed` 广播目录列表，前端对已打开标签按父目录匹配后重读、比对 revision（旧版 vaultStore.ts:340-361 + documentStore.ts:185-196），不在事件载荷里附文档列表。
  - 旧版设置导入：recentVault 目录不存在时仍导入，登记为「已断开」（旧版 `restore()` VaultService.ts:82-90），`import-legacy.done` 照写。
  - 小项：API.md 服务数 8 改 9；M38 的 §6.3 引用改为 M38 内联的 capabilities 定义；`StreamFrame.finish` 仅为末帧提示位、回合结束以 Channel 关闭/Prompt 返回为准；数据库设计 Q-C 正文副本定为独立 `doc_body` 表；PRD §12 新增 AC-OVW-1（待审批计数磁贴）与 AC-AI-6（共享上下文 2400 字符上限与「另有 N 项未列出」）；UI设计稿 §3.7 补「头部待审批标签」行（截图 42）。
- 受影响：docs/API.md、docs/数据库设计.md、docs/PRD.md（v1.5）、docs/技术设计.md（一句）、docs/UI设计稿.md（一行）。

## D-009：M1 计划阶段的 6 项决定（用户 3 项选择 + 3 项契约修订）

- 日期与状态：2026-11-18，已确认（D-009.1–D-009.4 用户选择；D-009.5–D-009.6 由用户在「使用你推荐的即可」中确认）。
- 背景：编写 docs/PLAN-M1.md 时发现 4 处契约与现实或旧版不一致，需要在动手前定口径。
- 结论：
  1. **图片保存走协议上传**：粘贴/拖入的图片由页面 `POST` 到 `http://kbimg.localhost/__upload?vault=<vaultId>`，请求体是图片原始字节；不再走绑定方法 `saveImage`，也不走 base64。理由：PRD FR-DOC 已有「图片不经 base64 跨进程传输」的约束，而 API.md 原 M19 用 base64，两者冲突；协议上传同时满足 PRD 与性能要求。
  2. **「来源改了」提供真正的一键同步**：旧版按钮叫「查看来源更新」，只是打开来源文档（`ProvenancePanel.tsx:109-118`）。新版改为生成一条 `edit` 变更计划，走计划卡确认后执行，可从操作日志恢复；成功后把该记录的基线更新为来源当前值。这是对齐后的有意改进，旧版没有。
  3. **健康面板 M1 交付，用户入口随 M3**：UI 稿里健康面板只能从总览磁贴打开，而总览在 M3。M1 做 M28 与面板，用 e2e 直接打开验证，不新增旧版没有的临时入口。
  4. **侧栏按钮只放已可用的功能**：M1 加顶部「新建文件 / 新建目录 / 搜索 Ctrl P」与底部「操作日志」；「导入技能」随 M3、「设置」随 M2 再加。
  5. **「进检索」开关 M1 不读**：PRD FR-AREA 写「即时生效」，但旧版索引根本不读 categories.json（`IndexService.ts` 无 categories 引用，`ignored` 只由系统区判定决定，L464）。M1 与旧版一致：索引只按系统区过滤；开关怎么生效在 M3 做设置页时再定，届时同步修正 PRD 措辞。
  6. **图片协议地址沿用 M0 已实现的形式**：`http://kbimg.localhost/<库内相对路径>?vault=<vaultId>`，不做 API.md 原写的「一次性 token」。安全靠 4 条校验保证：每次请求重新解析并 `Lstat`；`CallerWindow` 必须非空；按文件头判格式（SVG/ICO 拒绝）；单张 ≤8 MiB。M0 的实现缺后 3 条，M1 的 T12 补齐。
- 原因：前 4 项是产品口径选择；后 2 项是契约与现状/旧版的偏差，按「契约文档与代码对齐」原则处理，不让代码单方面漂移，也不让文档保留无法实现的承诺。
- 代价与风险：一键同步是新行为，恢复后基线不回退会让状态显示为「副本改了」（已作为 B-M1-05 记录并在计划中标注）；「进检索」开关在 M1–M2 期间是设置项里可见但无效的状态，PRD 已注明，M3 必须处理。
- 受影响：docs/API.md（M19、§3 映射、§5、方法总数）、docs/PRD.md（FR-AREA、FR-DOC、FR-PROV，版本 v1.6）、docs/PLAN-M1.md（T00、T09、T12、B-M1-01..08）、internal/service/image.go（T12 补齐校验）、vite.config.ts（CSP connect-src，T12）。
- 正文同步情况：本决策已回写上述文档；D-009 之前的 M0 阶段文档不改写。

## D-010：M2 计划复审的 Agent 默认配置与代理口径

- 日期与状态：2026-11-18，已确认（用户选择「按 PRD 现写法」）。
- 背景：复审 docs/PLAN-M2.md 时发现，PRD FR-AI-PROV 写的 Agent 默认配置（模型 flash、distiller 用 pro、推理强度高、scout 自动审批）来自旧版 `shared/agent.ts:289-297` 的 `DEFAULT_AGENT_CONFIG`。这个常量在旧版 `src/` 与 `tests/` 中没有任何引用。旧版实际运行时：会话建立时审批为 `ask`（`NativeEngine.ts:189`），随后被 agents.json 覆盖；agents.json 的默认值（`shared/appConfig.ts:330-367`）4 个 Agent 都是 `model=''`、`effort='auto'`、`approval='ask'`，`model` 为空时用供应商默认模型。
- 备选：①按旧版实际运行行为（4 个都逐条审批、都用默认模型）；②按 PRD 现写法。
- 结论：
  1. **按 PRD 现写法**。agents.json 缺失、或缺某个 Agent 的键时，默认值为：curator / scout / general 的模型 `deepseek-v4-flash`，distiller 为 `deepseek-v4-pro`；推理强度 `high`；审批 scout 为 `auto`，其余为 `ask`。steering 默认文本不变（`shared/appConfig.ts:336-373`）。
  2. **模型名落盘用裸名**。PRD 写的 `deepseek/deepseek-v4-flash` 中 `deepseek/` 表示供应商，不写进 agents.json：模型解析按供应商模型列表精确匹配（旧版 `nativeProvider.ts:41-46`），DeepSeek 预设的模型列表是裸名（`appConfig.ts:125-133`），带前缀的值永远匹配不上。用户的供应商里没有该模型时，按既有规则回落到第一个可用供应商的默认模型。
  3. **非法值的回退规则不变**：非法 effort 归 `auto`，非法 approval 降级 `ask`（降级只往严格方向走）。默认值与回退值是两回事。
  4. **代理口径以技术设计为准**：技术设计第 2.1 节已确认用 `mygo/netproxy`（WinHTTP，含 PAC / WPAD，回环直连，SOCKS5 可用）。PRD「SOCKS 代理回落为直连」改为与之一致；另记录一处差异：netproxy 在设置了 `HTTP_PROXY` / `HTTPS_PROXY` / `ALL_PROXY` / `NO_PROXY` 时以环境变量为准，旧版只问系统代理。
- 原因：用户选择 PRD 的产品意图（scout 只读检索免审批、distiller 用更强模型）；第 2 条是让该意图在既有解析规则下真正生效所必需的解释，不是新行为。
- 代价与风险：与旧版运行行为不一致的地方有 4 处：scout 默认免审批，distiller 默认 pro，强度默认 high，模型默认值非空。已有 agents.json 的知识库不受影响，文件里的值优先。强度控件在 D-002 下始终禁用，`high` 只是存储值，不改变请求。
- 受影响：docs/PRD.md（FR-AI-PROV 默认配置、FR-AI-ENG 代理，v1.7）、docs/数据库设计.md（3.1 默认值）、docs/PLAN-M2.md（T00、T02、T04、v0.3）。
- 正文同步情况：本决策已回写上述文档。

## D-011：M2 供应商与凭据的 4 项契约收紧

- 日期与状态：2026-11-18，已确认（T01 实施时按旧版与 PRD 核对后确认，随任务汇报给用户）。
- 背景：T01 实现 M67–M70 时发现旧版与 API/数据库设计有几处不一致，需要定口径。
- 结论：
  1. **删除 `credentialsPath`**：旧版回出 `engine-credentials.json` 的路径；新版 key 存 Windows 凭据管理器，没有文件可报。M67 出参只保留 `providers` 与 `credentials`。
  2. **M69 用活动知识库刷新快照**：方法签名没有 vaultId（旧版 `register.ts:773-787` 就是这样），成功后读活动库的 provider 列表；没有活动库时只回本次 envName 的凭据状态。这与图片协议必须显式传库不同：设置页的操作主体是活动库。
  3. **M70 错误码按旧版**：401/403 `permission-denied`、404/405 `unsupported`、其余 `io-failure`、响应 >2 MiB `too-large`；不使用 §1.3 的 `provider-error`（那是回合内引擎错误的码）。API M70 行同步修正。
  4. **重名规则按旧版非对称**：读取区分大小写、只留第一个；保存忽略大小写、整单拒绝。数据库设计 3.6 原文写「读取忽略大小写」与旧版不符，已改正。
- 原因：第 1、2 条是删掉新版无法提供的字段/参数；第 3、4 条以旧版行为为准，且都更严格或等价。
- 代价与风险：前端在 M67 里不再显示「凭据文件位置」；设置页「key 存哪里」改为说明「Windows 凭据管理器」。
- 受影响：docs/API.md（M67、M69、M70）、docs/数据库设计.md 3.6、docs/PLAN-M2.md T01 账本、internal/contract（ProviderDraft/ProvidersResult/FetchModelsResult）、internal/ai（providers/models/keys）、internal/service/providers.go。
- 正文同步情况：已回写上述文档。

## D-012：永久删除的新鲜批准只认变更审批卡（比旧版更严）

- 日期与状态：2026-11-19，已确认（用户选择「保持更严」）。
- 背景：T05+T06 权限门专项审查发现，旧版 `EngineHost.grantIfAllowed`（`EngineHost.ts:717-729`）在用户允许**任何**审批卡（含 write_note / edit_note 的写卡）时，都会同时登记 `mutationApprovals`（60 秒、路径一致、发卡时非 yolo）；新实现只有 `kind=mutation` 的卡登记 grant。审查请用户确认保持新实现还是对齐旧版。
- 旧版行为：写卡批准后 60 秒内，对同一路径发起 permanentDelete 可以不再弹卡，直接消费那条批准。
- 备选：①保持新实现，只认变更审批卡；②对齐旧版，任意批准卡都登记。
- 结论：**保持更严**。永久删除只能由专门的变更审批卡（目标一致、发卡时非全自动）在 60 秒内一次性放行；普通写入卡不顶替。
- 原因：与 D-005 和 PRD「永久删除需要本次单独确认」的意图一致——写卡批准的是内容写入，不是删除授权；旧版把两者混在同一张表里是实现疏忽，其行为是降低门槛。
- 代价与风险：与旧版存在一处用户可见差异：先批准一次写入、再在同一路径发起永久删除时，新版仍会弹卡。删除路径比旧版多一次确认，方向更安全。
- 受影响：docs/PRD.md（FR-AI-PERM，v1.8 与 `mutate_notes` 行）、docs/PLAN-M2.md 账本、internal/ai/approval.go（现实现无需改动，`kind == "mutation"` 才登记 grant）。
- 正文同步情况：已回写 PRD 与 PLAN。
- 附录（2026-11-19，T20 整体审查修复）：grant 进一步按变更 kind 绑定——`AllowMutation` 带 kind，`ConsumeFreshMutation` 只接受 `kind=permanentDelete` 的卡，move/copy 等其它结构变更的批准也不能顶替永久删除。D-012 的「只认变更审批卡」据此收窄为「只认永久删除的变更审批卡」。受影响：internal/ai/approval.go、internal/service/tools.go、internal/ai/approval_test.go（`TestMutationGrantIsKindBound`）。

## 单条模板

### 待编号：决策标题

- 日期与状态：待填写。
- 背景与需要解决的问题：待填写。
- 已知事实、约束与不确定性：待填写。
- 备选方案及主要利弊：待填写。
- 结论与确认依据：待填写；未确认时保持提议状态。
- 原因、代价、适用条件与后续风险：待填写。
- 受影响的文档、接口、数据、代码、Skills 或计划：待填写。
- 正文同步情况、替代关系：待填写。
