# M0 实施计划：基础工程 + 首个业务闭环

> 状态：M0 已完成（2026-11-18）：T00–T11 全部交付，AC 15/15，性能见 §6；已推送到 `origin/main`（`39041a0`）。
> 本次目标：完成工作流阶段 10（基础工程）与阶段 11（首个业务闭环）。闭环为「打开知识库 → 浏览文件树 → 阅读 / 编辑 / 保存文档 → 外部修改同步」，对应技术设计第 9 节能力 C1 + C2 + C3。
> 计划入口：本文件是 M0 计划；当前里程碑入口为 docs/PLAN-M2.md。

## 1. 目标、范围与授权

### 1.1 交付的可观察结果

1. `bun run dev` 弹出无边框窗口：40px 自绘标题栏、三栏工作台，深浅主题切换正常，没有白闪。
2. 托盘和关窗行为符合 FR-WIN（询问 / 挂到托盘 / 退出、记住选择、未保存确认）。
3. 首次启动自动导入旧版的应用设置和最近记录（只读旧文件）。
4. 能用欢迎页和新手引导登记新知识库；所有已登记知识库都断开时显示欢迎页（D-006）。
5. 打开知识库后，侧栏按功能区分组显示文件树，带语义图标和别名，懒加载并虚拟化。
6. 文档有标签页，支持阅读 / 源码 / 分栏三种模式；保存时保留编码、BOM 和换行风格；出现冲突时显示冲突卡；大文件按阈值降级。
7. 外部修改文件后 1 秒级同步到树和已打开的文档。
8. 阅读模式由 Go 渲染，渲染结果与旧版对照集一致；库内图片经 `kbimg` 协议显示。

### 1.2 关联需求

FR-WIN、FR-THEME、FR-VLT、FR-AREA（读取侧）、FR-TREE（浏览，不含变更操作）、FR-DOC、FR-LINK（阅读模式 href 分类；wikilink 只解析到「路径 / 文件名」两层，见 1.3）、FR-SET（应用设置、旧版导入；设置页不做）、NFR-PERF（冷启动、大知识库首屏、外部改动同步、长列表），以及 PRD 10.2 安全要求中与本范围相关的条目。

### 1.3 明确不做（M0 非目标）

| 不做 | 原因 / 去向 |
| --- | --- |
| 检索、链接图、反向链接、健康检查（C4） | 留到 M1。wikilink 的「按标题解析」依赖索引，M0 只做「相对路径 → 文件名」两层；有多个候选、无候选的显示与旧版一致 |
| 树的新建、重命名、别名编辑、归档、删除、拖放，变更计划，事务，操作日志，来源追踪（C5） | 留到 M1。M0 的树行不显示悬停操作按钮 |
| 图片粘贴保存（FR-DOC 中的 `saveImage`） | 写盘要走变更与事务，随 C5 进 M1 |
| AI、对话、历史、总览磁贴、设置页、技能导入（C6–C8） | M2 及以后。M0 中间栏只显示对话面板外框和一行占位文字，不做总览 |
| 安装包与发布 | M0 只做开发构建和本机 `mygo build` 冒烟，不出正式安装包 |

### 1.4 审批记录

见第 8 节。用户确认前不执行第 4 节的任何任务。

## 2. 前置资料与现状

| 输入 | 版本或实际情况 | 缺口 | 是否阻断 |
| --- | --- | --- | --- |
| PRODUCT / PRD | PRODUCT 已确认；PRD v1.5 已确认 | 无 | 否 |
| 设计稿 / DESIGN | design/baseline 共 53 张截图 + docs/UI设计稿.md 已审核；DESIGN.md v1.0 已审核 | 无 | 否 |
| 技术设计 / API / 数据模型 | 技术设计 v1.0、API.md v1.0、数据库设计.md v1.0 均已审核 | 无 | 否 |
| 现有代码 | 没有正式工程；spikes/ 下有 4 个一次性验证程序，可参考，不直接复用为正式代码 | — | 否 |
| Skills | `.claude/skills/` 下 9 个，阶段 07 已按实测修正 | 实际调用情况未验证 | 否 |
| 环境 | Go 1.27.3、Bun 1.3.13、WebView2；mygo v0.10.0 源码在模块缓存中 | 全局 mygo CLI 是 0.8.3，不使用 | 否 |
| 版本控制 | 本目录不是 git 仓库；远端 `https://github.com/lhmile489-design/Mile-Knowledge-Base-Go.git` 为公开空仓库 | 待 T00 初始化 | 否 |
| 手动确认 | spikes/r4-window/RESULT.md 第 4 节 13 项尚未回填 | T04 完成后在正式工程里再确认一次 | 阻断 T04 完成，不阻断开工 |

## 3. 总路线

> 粗粒度路线图，2026-11-18 应用户要求补充。M0、M1、M2 均已单独审批并完成；M3 未审批、不可执行。每个里程碑在前一个完成后，另写 docs/PLAN-M编号.md 细化，并单独审批。届时会根据前一个里程碑的实际发现调整，这里的划分和规模都可能变化。

| 里程碑 | 结果 | 状态 |
| --- | --- | --- |
| M0 | 基础工程 + C1 窗口骨架 + C2 知识库与文档读写 + C3 阅读渲染 | 已完成并推送（39041a0） |
| M1 | C4 检索、链接、健康 + C5 变更、事务、审计、来源 | 已完成并推送（59a1076） |
| M2 | C6 AI 引擎与权限 + C7 对话、历史、用量 + 设置页的「模型服务」「Agents」两节 | 已完成并推送（f6d663a） |
| M3 | C8 设置页其余各节、技能导入、总览 + 全量视觉验收 + 发布 | 已完成并推送（765f908） |
| M3.5 | R2 主题色：6 个预设加自定义，默认橙色，标题栏色点与设置页入口（PRD 第 16 节） | 已完成并推送（2026-11-20，T00–T11；`a566615`；见 docs/PLAN-M3.5.md）；M4 依赖其令牌与截图基线 |
| M4 | R2 技能管理核心：工具注册表、探测、扫描、分发、卸载、收编、操作记录、「技能」浮层 | 已完成并推送（2026-11-20，T00–T18；`d9c854b`；见 docs/PLAN-M4.md） |
| M5 | R2 安装来源：GitHub / zip 安装、多技能仓库勾选、更新检测 | 资料已审核通过（2026-11-21），T00 已放行；见 docs/PLAN-M5.md |
| M6 | R2 扩展：自定义工具目录、项目源、Git 跨机同步 | 需求已确认，Git 同步范围待 Q-SKM-4；未规划、不可执行 |

与原先一行版的差别：设置页的「模型服务」和「Agents」两节从 M3 提到 M2。原因是不配置供应商和 key，AI 无法使用，M2 的闭环验收就跑不起来。

### 3.1 规模参考

用旧版对应功能的源码行数（TypeScript，含界面）粗略衡量相对工作量。旧版行数和新版工作量不成比例：Go 与 TS 的写法不同；新版有些部分更重，比如要和旧版做字节级兼容、要做性能约束；也有些部分更轻，比如没有进程间通信。所以这一列只用来比较几个里程碑谁大谁小，不是工时估算。

| 里程碑 | 旧版对应源码 | 相对规模 |
| --- | --- | --- |
| M0 | 约 7 600 行（主进程窗口与知识库服务、安全、文本读写、设置、工作台骨架、树、标签、编辑器、阅读渲染） | 基准 |
| M1 | 约 5 000 行（索引、链接、健康约 1 450；变更、事务、审计、来源约 1 590；对应面板与计划卡约 1 700） | 略小于 M0，但写用户文件，风险最高 |
| M2 | 约 12 000 行（引擎与 Agent 约 5 400；对话与上下文约 2 100；对话界面约 3 700；设置中模型与 Agents 两节约 1 400） | 最大，约为 M0 的 1.5 倍 |
| M3 | 约 3 500 行（设置其余各节约 1 900；总览约 600；技能导入约 450；引导等零散部分）+ 全量视觉验收 + 发布清单 | 代码最少，验收和发布的工作较多 |

### 3.2 M1：检索、链接、健康 + 变更与恢复

- **目标闭环**：在搜索面板搜中文关键词，打开结果；看反向链接；在树里新建、重命名、归档一篇文档；去操作日志里恢复。
- **主要任务**：
  1. SQLite 索引：按 kb-index-sqlite skill 和数据库设计 4.5 建库与迁移，预切词，后台首建，变更批次增量写入；
  2. 检索（M24–M26）与搜索面板；
  3. 链接图：哈希索引，三层解析补上标题层（M0 推迟的部分），反向链接面板（M27）；
  4. 健康检查（M28）与健康面板；
  5. 变更计划（M29、M30）与计划卡、报告卡；
  6. 事务与恢复：`.mile-kb/transactions/`，字节级兼容（M32、M33）；
  7. 操作日志：audit.jsonl（M31）与日志面板；
  8. 来源追踪（M34、M35）与来源面板；
  9. 树的新建、重命名、别名编辑、拖放（M20–M23、M66），以及图片粘贴（M19）；
  10. 闭环验收、`.mile-kb/` 新旧版轮流打开的兼容测试。
- **依赖**：M0 的路径、读写、监听、渲染；R-2 结论。
- **主要风险**：
  - 事务与恢复会写用户文件，所以要独立审查，并用「新旧版轮流打开同一个库」的往返测试来验证；
  - 链接全量解析的耗时要在哈希实现上重测，spike 的 23.8 秒不作为基线；
  - 外部批量修改时，索引增量更新不能阻塞界面。
- **移植的旧版测试（参考）**：indexService、tokenize、searchHighlight、links、linkGraph、mutationService、transactionService、auditLedger、provenance、provenanceService、lineDiff、saveImage 等。

### 3.3 M2：AI

- **目标闭环**：在设置里配置 DeepSeek 并填 key → 在对话框输入「总结这篇文章」→ 自动路由到蒸馏学习 → 流式回答 → Agent 写草稿时弹审批卡 → 允许 → 用量收据 → 重启后恢复对话 → 在历史里搜索。
- **主要任务**：
  1. 供应商与 key：providers.json、`mygo.Secrets`、模型目录拉取（M67–M70）；设置页「模型服务」一节；
  2. Agent 配置、steering 与可用 Skills（M63、M64、M71）；Agent 列表与重置（M38、M43）；设置页「Agents」一节；
  3. 引擎：OpenAI 兼容流式协议、SSE 组装、重试、空闲超时、代理、上下文预算（按 agnes-engine skill）；
  4. 8 个知识库工具与权限门：两道判定、系统区禁写、60 秒消费型批准、永久删除规则（D-005）；
  5. 路由与改派（M44）；
  6. 流式通道（M39 + StreamFrame）、取消（M40）、审批（M41、M42）；
  7. 对话界面：消息虚拟化、按块增量渲染（M37）、过程流水线、证据雷达、输入框、运行参数条、用量收据；
  8. 共享上下文完整实现（M45–M49）与会话信息浮层；
  9. 对话历史：jsonl 落盘、FTS5 搜索（数据库设计 4.4）、导出、接着问、删除与清空（M50–M59）；
  10. 闭环验收，并用假供应商做 e2e（参考旧版 fake-openai-provider 的场景）。
- **依赖**：M1 的变更与事务，因为 Agent 的结构操作要走 MutationService；M0 的渲染。
- **主要风险**：
  - 权限门是安全核心，要独立审查；需要用测试证明「提示词无法扩大权限」；
  - R-4 当时用的是预生成的 HTML，要用真实引擎和真实 Markdown 复测流式性能；
  - D-002 规定 3 处行为原样复刻，验收要按旧版行为判定。
- **移植的旧版测试（参考）**：nativeEngine、openaiClient、engineProvider、contextBudget、agentPolicy、agentRouting、permissionRouter、executionMode、usageFormat、transcript、turnGroups、conversationStore、conversationRecorder、contextHub、contextTail、providerForm、modelCatalog、credentialStore 等。

### 3.4 M3：其余设置、导入、总览、验收与发布

- **目标闭环**：首次使用的完整路径与旧版一致；全部 53 张基线截图逐张对照通过；能打出 Windows 安装包并装好使用。
- **主要任务**：
  1. 设置页其余各节：常规、分类与检索、关于与诊断（M65），以及保存条与草稿；
  2. 技能导入向导（M14、M15）；
  3. 总览磁贴（M72、M73 与各状态汇总）；
  4. 全量视觉验收：对照 design/baseline 全部截图，并补做截图 60 的状态；
  5. 全量性能验收：PRD 10.1 指标与旧版基线对比，G-PERF 从警告转为阻断（PRD Q7）；
  6. 发布：完善 docs/发布清单.md，`mygo build` 打安装包，装好后冒烟，验证旧版设置导入；
  7. 文档收尾。
- **依赖**：M0–M2 全部完成。
- **主要风险**：
  - 安装包和 WebView2 运行时在干净机器上的表现，开发机上测不出来；
  - 视觉验收里可能会冒出 M0–M2 漏掉的细节，需要预留修复时间。

### 3.5 M3.5：R2 主题色

- **状态**：已完成（2026-11-20），T00–T11 按「一次一个任务」交付；验收、门禁、视觉对照与 1.1.0 升级冒烟见 docs/PLAN-M3.5.md 第 6、9 节。
- **目标闭环**：在标题栏色点或设置页选一个预设或输入自定义色，界面立即换色、重启后保持，第一帧就是所选颜色；6 个预设在浅、深两种主题下的对比度都有自动测试兜底。
- **开工前补齐**（各自审核）：
  1. PRD 第 16 节、DESIGN.md 第 12 节、docs/UI设计稿.md 第 10 节审核通过（已满足，2026-11-20）；
  2. 色板原型与截图（`design/r2/accent-picker.html`、`design/r2/shots-accent/` 20 张，用户 2026-11-20 审核通过）；
  3. API（设置补丁新增 `accent`）与数据库设计（`settings.json` 新字段及默认值）增补（已起草：API.md 第 11 节、数据库设计 4.1.1、技术设计第 14 节，用户 2026-11-20 审核通过）；
  4. 写 docs/PLAN-M3.5.md 并审批（v0.1 草案已写，2026-11-20，待审批；T00–T11 共 12 个任务，B-M35-01 版本与安装包、B-M35-02 审查方式待定）。
- **主要任务（粗粒度）**：令牌拆分（新增 `--raw-accent-text`，29 处文字用法改名，靛蓝下零视觉变化）→ 12 套预设 CSS 与默认改橙 → 自定义色生成函数与测试 → 设置字段、Go 校验与首帧镜像 → 标题栏色点与色板 → 设置页一行 → R1 视觉对照改在「靛」下跑、技能浮层截图按橙色重拍 → 验收 AC-ACC-1–6。
- **依赖**：M3 完成（已满足）。M4 依赖本里程碑：技能浮层按新令牌实现。
- **主要风险**：29 处文字用法里若有漏改，换成橙、绿、青时那处文字对比度会不达标，所以对比度测试要覆盖真实组件而不只是令牌；已有安装升级后默认色变化（PRD 16.4）。

### 3.6 M4：R2 技能管理核心

- **目标闭环**：在浮层里看清本机各 AI 工具装了哪些技能；把知识库技能分发到选中的工具（链接优先、退回拷贝），改知识库一处、工具立刻生效；卸载与覆盖都能恢复；工具里的独立技能可以收进知识库。
- **开工前补齐**（按 WORKFLOW 阶段顺序，各自审核）：
  1. PRD 第 15 节审核通过、Q-SKM-3 有结论（均已满足，2026-11-20）；
  2. 各工具在 Windows 上的技能目录与探测方式逐个核实（本机实测 + 官方文档），写进技术设计（已完成：技术设计第 13 节，用户 2026-11-20 审核通过）；
  3. 目录联接的创建、识别、删除不跟随，在 spike 里实测（含网络盘、跨盘、权限不足的退回路径）（已完成：`spikes/r9-junction`，26/26 通过，用户 2026-11-20 审核通过；权限不足未测，见其 RESULT.md）；
  4. 浮层的 UI 稿与 DESIGN.md 增补（入口改到标题栏；`design/r2/` 原型与 18 张截图、docs/UI设计稿.md 第 9 节、DESIGN.md 第 11 节，用户 2026-11-20 审核通过）；
  5. 技术设计、API（新增绑定与事件）、数据库设计（分发记录、操作记录、备份区）增补（已完成：技术设计第 15 节、API 第 12 节、数据库设计 4.6，用户 2026-11-20 审核通过）；
  6. 写 docs/PLAN-M4.md 并审批（v0.1 已批准，2026-11-20，T00–T18，B-M4-01–03 按建议执行）。
- **主要任务（粗粒度）**：注册表与图标资源（含 NOTICE）→ 探测 → 两套扫描与来源状态 → 分发（联接 / 拷贝、同名冲突、拷贝同步状态）→ 卸载与备份恢复 → 收编 → 操作记录 → 浮层两页与安装视图（本地文件夹）→ 验收脚本 AC-SKM-1–11、19 → 整体独立审查。
- **依赖**：M3 完成（已满足）；M3.5 完成（技能浮层按新令牌实现）。
- **主要风险**：知识库外删除的安全性（删链接误跟随会删掉知识库内容）；工具目录约定可能随工具版本变化；e2e 不能碰真实用户目录，需要把用户主目录换成临时目录的测试注入。

### 3.7 M5：R2 安装来源

- **状态：已完成（2026-11-21）**。T00–T16 全部交付；版本 1.3.0；AC-SKM-12–16 自动化、全量 e2e 250 项、性能门禁与真实 GitHub 冒烟均通过；已推送到 `origin/main`（`aaaadef`）。详见 docs/PLAN-M5.md。
- **目标闭环**：粘贴 GitHub 地址或 `owner/repo`，或选一个 zip，列出候选技能，勾选后装进知识库或直接装进工具；GitHub 来源的技能能看到「有更新」并安全更新。
- **开工前补齐**：Q-SKM-1、2、5 有结论（2026-11-21 已定，D-016）；技术设计加入 GitHub 端点（第 16.2 节；外连都在 Go 侧，CSP 不变）；来源记录格式写进数据库设计（3.8、4.6）；docs/PLAN-M5.md 审批（v0.1 已批准）。
- **主要任务（粗粒度）**：地址解析 → 下载与安全解包 → 候选定位与勾选 → 安装目标二选一 → 来源记录 → 更新检测与更新（知识库走变更计划）→ 验收 AC-SKM-12–16（离线用本地假 GitHub 服务）。
- **依赖**：M4 完成。
- **主要风险**：解包安全（路径穿越、超大包）；GitHub 未登录额度；e2e 不能依赖真实网络。

### 3.8 M6：R2 扩展

- **目标闭环**：用户登记的自定义工具目录与项目目录，与内置工具享有同样的扫描、分发、卸载、收编；Git 同步按 Q-SKM-4 的结论交付。
- **开工前补齐**：Q-SKM-4 有结论（M5 完成后再议）；docs/PLAN-M6.md 审批。
- **依赖**：M4 完成；Git 同步还依赖 M5 的来源记录。
- **主要风险**：Git 同步体量大，可能要单独再拆一个里程碑。

### 3.9 跨里程碑的约定

- 每个里程碑结束时：做一次独立整体审查，列出提交并询问用户后再推送；若有新的设计决定，回写 PRD 和契约。
- 13 项手动确认在 M0 的 T04 完成；之后只在窗口相关代码变化时复测。
- M1 结束后补一次「旧版性能基线」测量，用的是 M1 才具备的检索和批量变更场景。

## 4. 任务清单

状态取值：待做、进行中、阻塞、待验证、已验证。

| 编号 | 目标 | 修改范围 | 依赖 | 产物 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- | --- |
| T00 | 初始化版本控制 | `.git/`、`.gitignore`、`.gitattributes` | — | 本地仓库，`origin` 指向用户提供的地址；首个提交只含已有文档、skills、spikes、design | `git status` 干净；通过第 5 节 T00 的提交前检查清单 | 待做 |
| T01 | 建工程骨架 | 根目录的 mygo 模板文件、`go.mod`、`package.json`、`bun.lock`、`vite.config.ts`、`tsconfig.json`、`mygo.config.ts`、`main.go`、`src/` | T00 | 能运行的空应用 | `bun run dev` 弹窗并显示 React 根组件；`bunx mygo doctor` 必需项全绿 | 待做 |
| T02 | 接入质量门禁和测试数据 | `scripts/`、`package.json` scripts、README 命令表、`internal/testkit/`、`cmd/genvault/` | T01 | 门禁命令；样本知识库生成器 | 每个门禁都完成「违规对照失败、正常对照通过」的自验证（技术设计 10） | 待做 |
| T03 | 移植设计令牌和样式 | `src/styles/`、`public/theme-init.js`、`index.html`、`src/lib/icons.ts` | T01 | 旧版 tokens.css 按 DESIGN.md 移植并拆分 | 关键令牌值与旧版一致（脚本比对）；深浅主题都能切换 | 待做 |
| T04 | C1 窗口骨架 | `main.go`、`internal/service/window.go`、`internal/agent/context.go`（仅当前文档与 M49）、`src/app/`、`src/components/workbench/{TitleBar,Workbench}` | T01、T03、T05 | M01–M04、M06；E05、E06；托盘；关窗流程 | 第 5 节 V-C1 全部通过；13 项手动确认回填 | 待做 |
| T05 | 应用设置与旧版导入 | `internal/settings/`、`internal/service/settings.go`（M60、M61） | T01、T02 | `settings.json`（数据库设计 4.1）、`recent-documents.json`（4.2）、旧版导入（第 5 节）；E04、E11 | V-SET 全部通过 | 待做 |
| T06 | 路径安全与知识库登记 | `internal/pathguard/`、`internal/vault/registry.go`、`internal/service/vault.go`（M07–M13）、`src/components/{WelcomeView,OnboardingGuide}` | T02、T05 | 知识库 id 与旧版一致；欢迎页、新手引导、脚手架、已断开处理 | V-VLT 全部通过 | 待做 |
| T07a | 文件树 | `internal/vault/tree.go`、`internal/service/doc.go`（M16）、`internal/settings/appconfig_read.go`（M62 只读：categories、aliases）、`src/components/workbench/{VaultSidebar,TreeNode}` | T06 | 按功能区分组、带图标与别名、懒加载并虚拟化的树 | V-TREE 全部通过 | 待做 |
| T07b | 标签页与文档读写 | `internal/vault/{textfile,revision}.go`、`internal/service/doc.go`（M17、M18）、`src/components/workbench/{TabStrip,EditorPane,ConflictCard}`、`src/editor/` | T07a | 标签页、源码模式编辑与保存、冲突卡、大文件降级 | V-DOC 全部通过；**独立审查**（第 7 节） | 待做 |
| T08 | 文件监听与外部修改 | `internal/vault/watch.go`（服务层注入 `mygo.WatchFiles`）、E01、前端已打开标签比对 | T07b | 技术设计 4.2 的链路；超过上限时降级为 HintsOnly | V-WATCH 全部通过 | 待做 |
| T09 | C3 阅读渲染与图片协议 | `internal/render/`、`internal/service/render.go`（M36）、`kbimg` 协议处理器、`src/components/markdown/`（注入与事件委托） | T07b | 阅读模式与分栏；链接与图片分类；对照集回归 | V-RENDER 全部通过 | 待做 |
| T10 | 闭环验收与基线测量 | `e2e/`（automation 客户端）、`bench/`、本计划第 6 节记录 | T04–T09 | 端到端用例、性能数据、视觉对比记录、旧版基线 | 第 6 节验收全部有实际证据；**整体独立审查** | 待做 |
| T11 | 收尾 | README、CLAUDE.md、本计划 | T10 | 文档同步；推送前确认 | 用户确认后推送到远端 | 待做 |

## 5. 单任务精确交接

### T00 版本控制

- 默认分支 `main`。`.gitignore` 包含：`node_modules/`、`dist/`、`build/`、`src/mygo.ts`（生成物，是否提交按 mygo 模板的 gitignore 为准）、`*.exe`、`.env*`、`*.db`、`*.db-wal`、`*.db-shm`、`spikes/**/node_modules/`、`spikes/**/*.exe`。
- 提交信息遵循 Conventional Commits，结尾带 `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`；含中文的提交信息用 `git commit -F <文件>`。
- 每个任务验证通过后提交一次，只暂存本任务的文件。**不推送**，推送在 T11 询问用户后进行。
- 提交前检查清单（每次提交都要执行，因为仓库是公开的）：
  1. `git diff --cached --name-only` 中不得出现 `.env*`、`*.db*`、`*.exe`、`node_modules/`、`build/`、`dist/`；
  2. 在暂存内容里搜索密钥特征：`sk-`、`ghp_`、`gho_`、`xoxb-`、`xoxp-`、`Bearer `、`api_key`，以及带值的 `apiKey`；命中任何一项就停下检查；
  3. 不得包含用户的真实数据：`%APPDATA%\mile-knowledge-base` 下任何文件的内容，以及 `C:\Users\LHmile\AppData` 下的真实数据文件；
  4. design/baseline 的截图只能来自沙箱样本知识库（由 capture.cjs 生成）；首次提交前人工抽看截图 00、10、16、38、49，确认里面没有真实知识库内容或 key；
  5. 检查结果记入第 9 节账本。

### T01 工程骨架

- 生成：在系统临时目录运行 `go run github.com/egoist/mygo/cmd/mygo@v0.10.0 init -name "Mile Knowledge Base" -module github.com/lhmile489-design/Mile-Knowledge-Base-Go <临时目录>`，逐个复制进仓库根目录，不覆盖已有文件，`.gitignore` 合并。
- 钉版本：`go get github.com/egoist/mygo@v0.10.0`；`package.json` 中 `mygo-runtime`、`mygo-cli` 写精确版本 `0.10.0`。
- `mygo.config.ts`：`identifier: "dev.mile.kb"`，`devUrl: "http://127.0.0.1:5274"`；`vite.config.ts` 设 `server.port: 5274`、`strictPort: true`。
- M0 需要的前端依赖用精确版本（技术设计 2.2）：react / react-dom 19.3.1、zustand 5.1.0、@phosphor-icons/react 2.1.10、@tanstack/react-virtual 3.14.26、tailwindcss 与 @tailwindcss/vite 4.3.5、CodeMirror（view 6.43.15、state 6.7.6、commands 6.11.1、search 6.7.2、language 6.13.1、lang-markdown 6.5.2）；开发依赖 typescript 7.0.2、vite 8.3.8、@vitejs/plugin-react 6.3.0、vitest 5.1.0、jsdom、@testing-library/react。
- Go 依赖在用到它的任务里再 `go get`（T09 加 goldmark v1.8.7、wikilink v0.6.0、bluemonday v1.0.27），避免 `go mod tidy` 删掉暂时没用到的依赖。modernc.org/sqlite 不在 M0 引入。
- 产出接口：`src/lib/transport.ts` 作为前端调用 Go 的唯一入口，组件不直接 import `src/mygo.ts`（mygo-web-app skill）。

### T02 质量门禁与测试数据

在 README 的命令表登记以下命令标识，并给出实际命令：

| 门禁 | 命令标识 | 预期 |
| --- | --- | --- |
| G-GO-TEST | `test:go` | `go test ./...` 全部通过 |
| G-GO-RACE | `test:go-race` | `go test -race ./internal/...` 通过 |
| G-GO-VET | `lint:go` | `go vet ./...` 无输出 |
| G-WEB-TYPE | `typecheck` | `tsc --noEmit` 无报错 |
| G-WEB-TEST | `test:web` | `bunx vitest run` 全部通过 |
| G-FUZZ | `test:fuzz` | 对 pathguard 和 render 的模糊测试，固定时长内 0 失败；时长在 T02 测出基线后填写 |
| G-E2E | `test:e2e` | `go build -tags e2e -o build/e2e/mkb-e2e.exe`，再运行 Go 写的 automation 客户端 |
| G-PERF | `bench` | 输出第 6 节的指标；M0 只警告，不阻断 |
| G-VISUAL | 人工 | 对照 design/baseline 截图编号 |

- 门禁自验证：每个门禁各放一个故意失败的对照（例如一个必然失败的测试、一个类型错误），确认命令返回非零；验证完删除对照，结果记入本计划第 9 节。
- 样本知识库：`cmd/genvault` 生成两种样本到临时目录。
  - 「典型」：结构参照 design/baseline/capture.cjs 的样本，含各功能区、中文名、长文件名、BOM / CRLF 文件、1 MiB 与 2 MiB 文件、图片、wikilink；
  - 「大」：1 万篇 md 加 5000 个文件的代码目录（参照 spikes/r1-watch）。
  - 测试只使用生成的样本，不读用户真实数据。

### T03 令牌与样式

- 把旧版 `src/renderer/src/assets/tokens.css` 按 DESIGN.md 第 3 节拆到 `src/styles/`：令牌、组件原语、面板。`main.css` 只 `@import 'tailwindcss'` 加令牌文件。
- `public/theme-init.js` 在挂载前写 `data-theme` 与 `color-scheme`（CSP 禁止内联脚本）。
- 验证：写一个脚本，从旧版 tokens.css 和新版 `src/styles/` 中提取 `--raw-*` 与语义令牌，逐个比对数值，差异必须为 0；有意差异（如果有）列入本计划并获批准。

### T04 C1 窗口骨架

- 启动顺序（技术设计 4.1）：单实例锁 → 读设置（T05）→ `Theme.SetSource` → 注册 `kbimg` 协议（T09 填充处理器，T04 先注册空处理器）→ `NewWindow{Frameless, 1440×900, Min 744×640, BackgroundColor 按主题取 #eceef1 / #0e0f11}` → `OnReadyToShow` 显示 → `App.WhenReady` 后 `NewTray`。
- 顺序约束：
  - 首帧不闪靠三者配合：T03 的 `theme-init.js` 在 React 挂载前、`OnReadyToShow` 之前生效，加上 `Theme.SetSource` 和 `BackgroundColor`；
  - T05 的导入提示（E11）在页面调用 M04 `ready` 之后才推送，未就绪时先缓存。T05 本身不依赖窗口。
- 绑定：M01 minimize、M02 toggleMaximize、M03 close、M04 ready（就绪握手，幂等）、M06 pickVaultDir。最小化后恢复用 Show 加 Focus（R-6 实测）。
- 关窗：`OnClose` 中 `PreventDefault`；有未保存文档时先确认。dirty 状态来自前端调用 M49 `AgentService.contextReportDocument` 上报的当前文档状态，与旧版一致（旧版关窗守卫读 ContextHub.activeDocument，`windowCloseGuard.ts`）。M0 在 `internal/agent/context.go` 只实现 ContextHub 的「当前文档」部分和 M49，其余共享上下文能力留到 M2；按 `closeBehavior` 走询问、挂到托盘或退出；询问框使用 `Dialog.Message`，按钮为「挂到托盘」「退出应用」，`CheckboxLabel` 为「记住我的选择」；托盘创建失败时按退出处理。
- 前端：TitleBar（图标、名称、VAULT 标签、知识库名、主题分段控件、窗口按钮）；Workbench 五列网格，8px 缝可拖动，键盘每次 ±8px，栏宽写回设置。
- 测试先行：关窗决策写成纯函数（输入：closeBehavior、dirty、托盘是否可用、对话框结果；输出：动作），移植旧版 `closeBehavior.test.ts`、`windowCloseGuard.test.ts` 的用例。
- 手动确认：在正式工程里重新执行 spikes/r4-window/RESULT.md 第 4 节的 13 项，结果记入第 9 节。

### T05 设置与旧版导入

- 格式按数据库设计 4.1、4.2：原子写入；保留未知字段；损坏时备份为 `.corrupt` 后用默认值；`version: 1`。
- 旧版导入按数据库设计第 5 节：只读 `%APPDATA%\mile-knowledge-base\settings.json` 和 `recent-documents.json`；写 `import-legacy.done`；目录已断开的库仍导入；失败时推 E11 提示。
- 测试先行：
  - 移植旧版 `configStore.test.ts`、`recentDocuments.test.ts`；
  - 新写导入测试，用 `verify-ui-m0.cjs` 同款的种子文件作样本；
  - 覆盖：导入一次后再启动不重复导入；旧文件损坏；字段越界时夹到范围内；已断开的库。
  - 测试里通过注入路径指向临时目录，**不读真实 %APPDATA%**。

### T06 路径与知识库登记

- `pathguard`：拼写层加真实路径层两道校验（go-service-engineering skill）。移植旧版 `pathContainment.test.ts`、`pathAliasBypass.test.ts`，并加入模糊测试（G-FUZZ）。
- 知识库 id：`hex(sha256(lower(realpath)))[:16]`。测试先行：用旧版算法对同一路径算出的值作为期望值（在测试里固定几组样例，包括中文路径和盘符大小写不同的情况）。
- VaultService M07–M13：已断开知识库的处理；所有知识库都断开时显示欢迎页（D-006）；新建向导与脚手架（6 个目录 + README，非空目录必须确认，可重复执行）。移植 `vaultService.test.ts`、`vaultUnavailable.test.ts` 中适用的用例。
- 前端：WelcomeView、OnboardingGuide，对照截图 00–03。

### T07a 文件树

- 树（M16）：懒加载，每次一层；目录在前；排序大小写不敏感；一级目录映射到功能区；目录语义图标；别名（读 `.mile-kb/settings/aliases.json`）；「未索引」标注只读取 categories 配置。移植 `treeOrder.test.ts`、`directoryIcons.test.ts`。前端用 react-virtual 虚拟化。
- 性能：单目录 10 000 项时滚动保持 60fps（V-TREE-2）。

### T07b 标签页与文档读写

- 读写（M17、M18）：
  - 严格 UTF-8；记录 BOM 与主导换行风格，写回时还原；
  - 原子写入（同目录临时文件 + rename）；
  - revision 由大小、修改时间、SHA-256 组成；
  - 阈值：1 MiB 不进阅读模式，2 MiB 只读，单次写入上限 2 MiB；
  - 冲突返回 `conflict` 形态（API.md M18）。
  - 移植 `textFile.test.ts`、`documentWrite.test.ts`、`revisionRegistry.test.ts`。
- 只读保证测试：扫描和读取前后，对样本里所有文件算 SHA-256，断言完全一致，且没有新增文件。
- 前端：TabStrip（拖拽重排、双击置顶、脏点、关闭确认，移植 `tabsStore.test.ts`）；EditorPane 三模式工具栏；CodeMirror 源码模式（外部修改用差异更新，不整篇替换）；ConflictCard 的 3 个选项。
- 性能约束：树和读取都不阻塞界面；IO 有并发上限（技术设计 4.1、P-7）。

### T08 文件监听

- `mygo.WatchFiles` 使用普通模式、Recursive、忽略 `node_modules` 与 `.git`、`MaxEntries` 65536、Debounce 50ms、MaxDelay 500ms；遇到 `ErrWatchLimit` 降级为 HintsOnly 加按目录比对，并推 E11 提示（R-1）。
- E01 载荷为 `{vaultId, directories[], sequence}`。前端按父目录匹配已打开的标签，重读并比对 revision（API.md 中 F 项的做法）：
  - 标签没有未保存修改：直接显示新内容；
  - 有未保存修改：显示「磁盘有新版本」信息卡。
- 过滤自己的写入：保存成功后记下新的 revision，监听报回同一 revision 时忽略。
- 测试：用 R-1 的场景 S1–S4 写成 Go 集成测试（需要应用运行时就放到 e2e）。

### T09 阅读渲染

- `internal/render` 按 markdown-render-pipeline skill 实现：
  - 原始 HTML 转义显示；frontmatter 当正文；标题不生成 id；`#xx` 锚点点击无动作；
  - 链接分 3 类；图片按规则分类，库内路径相对知识库根；
  - bluemonday 白名单。
- wikilink 的 M0 解析器只查「规范路径 → 文件名」两层（1.3）。标题层留到 M1，接口上预留。
- 回归：把 spikes/r3-render 的 73 条输入和旧版输出做成 golden 测试；对照结论必须与 R-3 一致（一致 76 例 / 有意差异 30 例 / 结构差异 0 例）。判定口径：旧版要到第三层（按标题）才命中的用例，统一列入「M1 清单」，M0 不比对这些用例；其余用例只比对路径层和文件名层的结果，必须与 R-3 一致。M1 清单在 T09 开始时从对照集机械筛出（按旧版规则算出每个用例的命中层级），写进本计划第 9 节，执行中不临时判断。
- `kbimg` 协议：只返回授权路径下、白名单格式的图片；地址形式 `http://kbimg.localhost/...`。测试覆盖越界、编码过的 `../`、SVG 拒绝。
- 前端：注入 HTML 后用事件委托处理链接点击（外链经 M05 `openExternal`）和代码块复制；类名与旧版 `.prose`、`.reader` 等一致。
- 模糊测试：渲染入口纳入 G-FUZZ。

### T10 闭环验收

- 端到端（G-E2E，automation 驱动真实窗口）：跑第 6 节的 AC-M0 用例。
- 性能（G-PERF）：在「典型」与「大」样本上测第 6 节指标。同样的样本也在旧版上测冷启动和大知识库首屏，作为 PRD Q7 的基线；旧版用 design/baseline/capture.cjs 的沙箱方式运行，不改旧版文件。
- 视觉（G-VISUAL）：同样本、同窗口尺寸、同主题，对照截图 00、01、02、03、13、16、17、18、19、22、23、24、25、57（10、12、56 只比对三栏几何，因为中间栏的总览不在 M0 范围）。

### T11 收尾

- 更新 README（真实命令）、CLAUDE.md（当前阶段、模块边界）、本计划状态与账本。
- 列出要推送的提交，询问用户；用户同意后推送到 `origin main`。

## 6. 验证与验收

| 编号 | 给定条件 | 操作 | 预期结果 | 方式 |
| --- | --- | --- | --- | --- |
| V-C1-1 | 系统深色，设置为跟随系统 | 冷启动 | 首帧即深色，没有白闪 | e2e 截图取像素 |
| V-C1-2 | 关窗行为为询问 | 点关闭 | 弹出询问框；勾选「记住我的选择」并选「挂到托盘」后，再次关窗直接隐藏 | 手动 M1–M3 |
| V-C1-3 | 有未保存文档 | 点关闭 | 先弹「继续编辑 / 丢弃并关闭」 | e2e + 手动 |
| V-C1-4 | 窗口 | 缩到最小 | 被限制在 744×640 | e2e |
| V-SET-1 | 临时目录中放旧版种子文件 | 首次启动 | 主题、布局、最近知识库按旧值导入；再启动不重复导入 | Go 测试 |
| V-SET-2 | 所有测试、e2e、基准与截图脚本 | 运行 | 都不读取真实的 `%APPDATA%\mile-knowledge-base`。旧版数据目录由调用方注入，只有生产启动路径会传入真实目录；用一个测试断言测试进程打开过的文件都不在真实目录下 | Go 测试 + 代码审查 |
| V-VLT-1 | 非空目录 | 新建向导 | 不勾选确认就不建；勾选后只建缺少的目录 | Go 测试 + e2e |
| V-VLT-2 | 唯一的知识库被改名 | 启动 | 显示欢迎页，该库为已断开故障卡 | e2e |
| V-VLT-3 | 同一路径 | 计算知识库 id | 与旧版算法结果一致 | Go 测试 |
| V-TREE-1 | 「典型」样本 | 打开知识库 | 分区顺序、图标、别名、排序都与截图 13 一致 | e2e + 人工对照 |
| V-TREE-2 | 单目录 10 000 项 | 滚动文件树 | 保持 60fps，DOM 中只有可见行 | e2e + bench |
| V-DOC-1 | CRLF + BOM 的文件 | 改一个字后保存 | 只有这个字的字节变化 | Go 测试 + e2e |
| V-DOC-2 | 外部修改了一个有未保存修改的文件 | Ctrl+S | 不写盘，显示冲突卡，3 个选项都能用 | e2e |
| V-DOC-3 | 1.5 MiB 文件 / 2.5 MiB 文件 | 打开 | 阅读模式禁用并说明原因 / 只读预览 | e2e |
| V-DOC-4 | 同级已有 Readme.md | — | M0 不含新建，不验证 | — |
| V-WATCH-1 | 「大」样本 | 外部脚本一次改 1000 个文件 | 界面保持可操作；树与已打开文档在 1 秒级内同步；主线程没有超过 200ms 的任务 | e2e + bench |
| V-RENDER-1 | R-3 对照集 | 渲染 | 与 golden 一致（M1 解析的用例除外） | Go 测试 |
| V-RENDER-2 | 文档含 `../` 图片、SVG 图片、远程图片 | 阅读模式 | 分别为越界占位、拒绝、点击后才加载 | Go 测试 + e2e |
| AC-M0-1 | 首次启动、无知识库 | 引导建库 → 打开 → 展开 skills → 打开文档 → 切源码 → 编辑 → Ctrl+S → 外部改另一篇已打开文档 | 每步与截图和 PRD 一致；重启后最近文档和布局都保留 | e2e |

性能指标（PRD 10.1）。测量方法：`scripts/e2e/perf.sh`（e2e 构建 + automation，样本由 `cmd/genvault` 生成；进程启动到树第一行出现，含 WebView 启动）。

| 指标 | 目标 | 新版实测（2026-11-18） | 旧版基线 |
| --- | --- | --- | --- |
| 冷启动到树首屏（典型） | ≤ 1.5s | 1101ms（进程启动起算） | 未测 |
| 冷启动到树首屏（大：1 万 md + 5 千代码文件） | — | 908ms | 未测 |
| 打开一个含 5 000 项的单目录 | 首屏 ≤ 300ms | 目录数据 320ms 到位；DOM 中只有 40 行（虚拟化生效） | 未虚拟化 |
| 树 5 000 项滚动 3 秒 | 60fps | 长任务 0 个（>50ms 为 0） | 未测 |
| 外部一次改 1 000 个文件期间主线程 | 无 >200ms 任务 | 长任务 0 个（观察 4 秒） | 机制性卡顿（P-1） |
| 渲染 100 KB 文档 | — | R-3 基准约 24ms | 未测 |
| 读模式端到端 | — | 已验证渲染与图片；单独计时未做 | — |

旧版基线（PRD Q7）尚未测量：需要把同一套样本交给旧版 Electron 应用跑一遍。M0 未做，记录为待办；G-PERF 维持警告级。

### 6.1 闭环验收记录（AC-M0-1，2026-11-18）

`scripts/e2e/ac-m0.sh` 在临时 APPDATA + 样本知识库上按用户路径驱动真实窗口，15 项断言全部通过：

```text
workbench、9 个分区、展开 skills、子项可见、点开 raws、点开 raws/首页.md、
阅读模式渲染、切源码、源码编辑器可见、编辑后脏点出现、保存后脏点消失、
磁盘文件确有「AC 追加。」、外部修改被自动采用、生产页面含 CSP meta
RESULT pass=15 fail=0
```

## 7. 协作、审查与执行方式

- 执行者：主会话。可以用子代理并行做互不重叠的任务，例如 T03 与 T05 可以并行。每个子任务必须给出文件范围，两个写入者不碰同一批文件。
- 汇合点：T04 依赖 T03 和 T05；T07b 之后才能做 T08 和 T09。
- 共享产物负责人：`internal/service` 的绑定签名以 API.md 为准，由主会话维护；`src/mygo.ts` 只通过 `mygo generate` 生成。
- 测试先行：Go 的业务逻辑（关窗决策、设置、导入、pathguard、知识库 id、树排序与图标、文本读写、revision、监听合并、渲染）一律先写失败测试再实现。
- TDD 例外（需批准）：脚手架与配置（T00、T01）、令牌移植（T03，用脚本比对代替）、纯视觉组件（用 e2e 截图与人工对照代替）。
- 独立审查：
  - T07b 完成后审查一次，因为它涉及写用户文件，风险高；
  - T10 完成后做整体审查。
  - 审查者是新开的子代理，输入为批准的契约、任务范围和完整 diff，不附作者结论。
  - 修复和复审最多 3 轮，仍有阻断问题就暂停并向用户报告。
- 执行方式：用户批准后，在第 4 节范围内连续执行。每个任务完成就简要汇报并更新第 9 节；遇到新的决策点或审核节点就暂停。
- 提交：每个已验证的任务提交一次，不推送（T11 询问后推送）。

## 8. 来源基线与批准记录

| 资料 | 版本 / 日期 | 适用范围 |
| --- | --- | --- |
| docs/PRD.md | v1.5，2026-11-18 | 第 1.2 节所列功能 |
| DESIGN.md | v1.0，2026-11-18 | T03、T04、T06、T07、T09 |
| docs/UI设计稿.md + design/baseline | v1.0，2026-11-18 | 视觉验收 |
| docs/技术设计.md | v1.0，2026-11-18（第 10 节门禁表已批准） | 全部 |
| docs/API.md | v1.0，2026-11-18 | M01–M04、M05、M06、M07–M13、M16–M18、M36、M49（dirty 上报）、M60–M62；E01、E04、E05、E06、E11 |
| docs/数据库设计.md | v1.0，2026-11-18 | 4.1、4.2、第 5 节、3.2–3.3（只读） |
| docs/decisions.md | D-001 至 D-008 | 全部 |

| 批准标识 | 内容 | 依据和时间 | 状态 |
| --- | --- | --- | --- |
| B-01 | identifier `dev.mile.kb`，开发端口 5274 | 用户 2026-11-18 选择 | 已确认 |
| B-02 | 技术设计第 10 节质量门禁表按提议生效 | 用户 2026-11-18 选择 | 已确认 |
| B-03 | 版本控制：git init，远端 `lhmile489-design/Mile-Knowledge-Base-Go`（公开），每个已验证任务小步提交，里程碑末询问后再推送；spikes 和 design/baseline 都进仓库 | 用户 2026-11-18 选择 | 已确认 |
| B-04 | M0 范围为工程加 C1–C3 闭环 | 用户 2026-11-18 选择 | 已确认 |
| B-05 | 本计划第 1.3 节非目标、第 4 节任务、第 7 节的 TDD 例外与审查安排 | 用户 2026-11-18 指示「开始执行任务」 | 已确认 |

## 9. 恢复入口与执行账本

### 下次会话先读

- 当前阶段：M0 完成；M1 计划未制定。
- 下一个任务：M1 计划（用户将更换模型后编写）。
- 工作区：没有 git；没有正式工程代码；spikes/r4-window 的验证应用可能还在运行（PID 17392），不是本计划启动的。
- 最近验证：阶段 07 的 spike 结果，见技术设计第 7 节。
- 待决事项：B-05；13 项手动确认（T04）。

| 时间 | 任务与状态变化 | 文件 / 版本范围 | 验证与证据 | 待决事项 | 下一任务 |
| --- | --- | --- | --- | --- | --- |
| 2026-11-18 | 计划草案完成 | docs/PLAN-M0.md v0.1 | 自查见第 10 节 | B-05 | T00 |
| 2026-11-18 | 应用户要求在第 3 节补粗粒度路线图（M1–M3 未审批）；设置页「模型服务」「Agents」两节从 M3 提到 M2 | docs/PLAN-M0.md v0.3 | 73 个绑定方法已核对全部归入 M0–M3 | B-05 | T00 |
| 2026-11-18 | T00 完成 | .gitignore/.gitattributes + 463 文件首提 | 提交前检查通过（无密钥、无大文件、截图 00 重截为中性路径） | — | T01 |
| 2026-11-18 | T01 完成 | mygo 模板 + package/tsconfig/vite + 最小窗口 | doctor 必需项全绿；tsc 通过；vite build 通过；dev 启动 75s 正常 | — | T02 |
| 2026-11-18 | T02 完成 | internal/testkit、cmd/genvault、vitest 配置 | 门禁负向对照：4 个门禁在故意破坏时均失败，移除后恢复通过 | — | T03 |
| 2026-11-18 | T03 完成 | src/styles/tokens.css（4913 行原样移植）、theme-init.js、compare-tokens | 80 个令牌声明零差异 | 未按 DESIGN §3 拆分文件，M0 保持单文件，拆分留到 M3 | T05 |
| 2026-11-18 | T11 完成 | README 命令表与状态、CLAUDE.md 当前阶段、本计划状态 | 提交 12 个，未推送；推送需用户确认 | 无（推送待确认） | M1 |
| 2026-11-18 | T10 完成 | 生产构建注入 CSP（Vite 插件，仅 build）；`scripts/e2e/{ac-m0,perf}.sh` 入库；样本生成器改为一半嵌套一半平铺 | AC 15/15 通过；性能：典型冷启动 1101ms、大库 908ms、5000 项目录 320ms、滚动与 1000 文件外部改动长任务均为 0；CSP meta 存在于 dist 与页面 | 旧版性能基线未测（PRD Q7 待办）；读模式单独计时未做 | T11 |
| 2026-11-18 | T09 完成 | ReaderView（Go HTML 注入 + 事件委托）、阅读/源码/分栏三模式、kbimg 图片、代码复制、wikilink 导航；EditorPane 模式切换与大文件禁用 | e2e：`.prose` 渲染、任务复选框、原始 HTML 以文字显示（无 div.x）、图片经 `http://kbimg.localhost/...` 加载（naturalWidth=1）、点 wikilink 打开目标文档（2 个标签、active=共享笔记.md）、点未创建链接不动作；分栏下 CM 与 prose 并存；截图 design/m0/t09-reader-1440x900.png | 表格/代码块由 render golden 覆盖，样本文档里没有，未在真实窗口截图；CSP 未接入 | T10 |
| 2026-11-18 | T07b 审查修复 | 阻断：dirty 只上报当前标签 → 改为聚合全部标签；需要修改：稍后处理死按钮（新增 dismissConflict）、采用磁盘版本两步确认（600ms 门 + 4s 复位，纯函数 + 4 个单测）、新建文件权限 0600→0644、forget 清理 revision、外部变更按 hash 判定、冲突载荷带 diskSize；补 Go 回归测试 5 项（同大小同 mtime 不同内容、expectedRevision=0 冲突、revision 稳定、无 tmp 残留、负数 revision） | vault 测试 15 项通过；vitest 12 项；tsc 通过；API.md 补 diskSize 字段 | 自写回环断言留 T10 | T09 |
| 2026-11-18 | T08 完成 | E01 订阅 → 打开的文档重读；干净文档自动采用磁盘版本，脏文档只在卡上提示不覆盖；通知栈（监听降级/导入提示）；lib/docUtils 抽出并加单测 | e2e 实测：外部追加 → 文档自动跟进（adopted=true, dirty=false）；本地有草稿时外部改动 → 保留本地、externalRevision=3、卡片出现；截图 design/m0/t08-external-change-1440x900.png；vitest 8 项通过 | 自写回环未单独断言（保存后重读同内容，无可见影响，T10 补） | T09 |
| 2026-11-18 | T07b 完成（自动部分） | docStore（打开/脏标记/保存/冲突/置顶/关闭确认）、TabStrip、EditorPane、CodeMirror 源码编辑器（最小差分更新）、ConflictCard、大小与编码信息、dirty 上报关窗守卫；editor.css | e2e 实测：打开 raws/首页.md（rev 1, LF, 52 B）→ 编辑后 dirty=true → 保存 rev 2 / 68 B，磁盘文件确有追加；外部改盘后保存出现冲突卡，三个按钮齐全；截图 design/m0/t07b-editor-conflict-1440x900.png | 阅读/分栏模式待 T09；独立审查已完成，问题见下条 | T08 |
| 2026-11-18 | T07a 完成 | VaultSidebar（虚拟化树、功能区分节、图标、别名、子项计数、未索引标注）、treeStore（懒加载/展开/失效重载）、treeIcons、sidebar.css | e2e 实测：12 行根节点、9 个分区、展开 skills 后 16 行；截图 design/m0/t07a-sidebar-tree-1440x900.png；tsc 通过 | 单目录 1 万项的滚动帧率未实测（V-TREE-2 留 T10）；文件点击只选中，打开文档在 T07b | T07b |
| 2026-11-18 | T06 完成（自动部分） | WelcomeView（主卡/最近卡/故障卡）、OnboardingGuide 两步、App 路由按 activeVaultId 分流、welcome.css、启动时恢复上次知识库（Prefs.LastVaultID） | 截图 design/m0/t06-welcome-recent-and-fault-1440x900.png 与 t06-onboarding-step1-1440x900.png；vault 测试 10 项通过（含恢复上次库、断开库、文件冒充目录） | 向导第 2 步依赖原生选目录框，无法自动截图；CreateVault 的非空确认只有代码路径，未做端到端 | T07a |
| 2026-11-18 | T04 完成（自动部分） | 前端 TitleBar、Workbench 三栏网格、可拖拽/键盘分隔条、文档面板折叠轨、窗口按钮、标题栏事件；vite 补 Tailwind 插件 | e2e 构建 + automation 实测：标题栏 40px、三栏面板 3 个、文档轨 40px、分隔条拖拽与 Ctrl+\ 折叠可用；截图 design/m0/t04-workbench-shell-1440x900.png | 13 项手动确认待用户回填 | T06 |
| 2026-11-18 | T05/T06/T07a/T07b/T08/T09 后端部分完成 | pathguard、settings、vault（registry/tree/textfile）、render、watch、service 绑定、kbimg、关窗决策 | go test 全绿（pathguard 13、settings 6、vault 8、render 78、watch 18、testkit 2）；golden 94 一致 / 12 有意差异；tsc 通过 | 各任务的界面部分未做 | T04 |
| 2026-11-18 | 独立审查（子代理，只读）发现阻断 2 项、需修改 3 项、建议 4 项，已修订为 v0.2：T00 提交前清单、V-SET-2、T07 拆为 T07a / T07b、T09 判定口径、启动顺序约束。审查同时核实 14 个待移植的旧版测试文件，以及计划引用的全部接口和截图编号都存在 | docs/PLAN-M0.md v0.2 | 审查报告 | B-05 | T00 |

## 10. 自查

- 覆盖：第 1.2 节每个功能都有对应任务和验收；非目标写明了去向。
- 接口一致：任务里引用的方法和事件编号都来自 API.md 2.2、2.3 节；存储格式引用数据库设计.md 的小节，本计划不另定义字段。
- 未定值：G-FUZZ 时长、性能的旧版基线标为「T02 / T10 测出后填写」，不作为已批准的可执行数值；其余数值都来自已批准文档。
- 已知风险：
  - 中间栏在 M0 是占位，截图 10、12、56 只比对几何；
  - wikilink 标题层解析推迟到 M1；
  - mygo v0.10.0 发布不久，遇到框架缺陷时按 WORKFLOW 5.4 调试，连续 3 次修不好就升级给用户。
