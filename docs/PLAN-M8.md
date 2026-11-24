# M8 实施计划：R2 技能页（独立页面与材质试点）

> 状态：v0.1 已批准（用户 2026-11-24 按建议批准，含 Q-M8-1–3、B-M8-01–03）；按「一次一个任务」执行，T00–T14 已完成，推送待用户确认。
> 本次目标：交付 PRD 第 17 节 FR-SKM-PAGE（v2.7）。闭环为「在标题栏点『技能』进入独立技能页 → 在左侧导航切换知识库与工具目录 → 完成一次分发与一次卸载 → 点『工作台』返回，对话、文档、草稿与滚动位置都和离开前一样」；技能页里的毛玻璃卡片、背景色场与非线性动效按 DESIGN 第 13 节实现，性能不达标时自动降级。
> 上一里程碑：docs/PLAN-M7.md（已完成并推送，`813e9ec`；推送记录 `8bacb3e`，版本 1.5.0）。下一里程碑：未规划。

## 1. 目标、范围与授权

目标：

1. 页面外壳：标题栏提到根组件，「工作台 / 技能」切换胶囊替代「技能」按钮；工作台在技能页期间保持挂载并隐藏；设置页打开时先走离开确认（技术设计 19.2）。
2. 技能页：左侧导航（知识库分类、工具目录、管理目录、分发进度），主区工具栏与卡片网格 / 工具列表；窄窗口图标栏（UI 稿 11.1–11.2）。
3. 子视图：分发、收编、工具更新改为整页子视图；安装、更新、还原、管理目录换整页外壳；卸载与覆盖保留居中确认卡（技术设计 19.3）。
4. 材质与动效：毛玻璃、色场、入场与悬停动效、降级开关与自动降级、关闭动画效果时的行为（DESIGN 第 13 节，技术设计 19.4）。
5. 迁移：原浮层的入口（总览磁贴、设置页「管理目录」、打开文档 / 定位）改到技能页；e2e 与截图脚本改写入口；删除浮层外壳与旧名字。
6. 验收：AC-PAGE-1–8 自动化；真窗口视觉对照；性能门禁。

不做：改任何技能管理的行为、接口、存储与写入规则；设置页的保持挂载（Q-M8-1）；把材质推广到技能页以外（DESIGN 13.1）；新增依赖。

授权边界：测试与 e2e 只用临时目录、假 HOME 与本地假 GitHub，不读写真实工具目录、真实知识库与真实应用数据，不访问真实 GitHub；Agent 权限不变（PRD 15.4）。

## 2. 来源基线与决策点

| 项 | 内容 |
| --- | --- |
| 需求基线 | PRD 第 17 节 FR-SKM-PAGE、AC-PAGE-1–8、Q-PAGE-1–5（v2.7，用户 2026-11-24 审核通过）；D-019 |
| 设计基线 | docs/UI设计稿.md 第 11 节、DESIGN.md 第 13 节（用户 2026-11-24 审核通过）；原型 `design/r2/skills-page.html` 与 `design/r2/shots-page/` 18 张 |
| 技术基线 | 技术设计第 19 节（页面外壳、组件划分、材质与降级、性能、测试、契约影响；用户 2026-11-24 随本计划审核通过） |
| 复用的既有实现 | `KbSkillsPage`、`ToolSkillsPage`、`InstallView`、`UpdateView`、`RestoreView`、`TargetsView`、各对话框的数据与提交逻辑；`SkillsPanel` 里的扫描刷新、更新检查、结果卡与变更计划联动；`perf-ai.sh` 的帧间隔采样 |
| 代码基线 | `origin/main` `8bacb3e`（1.5.0） |
| 已定 | Q-PAGE-1 设置页先关再进；Q-PAGE-2 启动总在工作台；Q-PAGE-3 不加快捷键；Q-PAGE-4 M8 发 1.6.0；Q-PAGE-5 性能不达标退成不模糊卡片（用户 2026-11-24） |

以下各项用户 2026-11-24 批准计划时全部按建议执行：

| 编号 | 问题 | 选项 | 建议 |
| --- | --- | --- | --- |
| Q-M8-1 | 设置页现在打开时会卸载工作台，要不要顺便改成保持挂载 | (a) 不改，只把标题栏提到根组件；(b) 一起改 | (a)：PRD 没要求，改了扩大回归面；技能页验证过做法后再单独提议 |
| Q-M8-2 | `openSkillsPanel` 等旧名字保留多久 | (a) M8 实施期间保留，T13 删除并改完调用方；(b) 立即删除 | (a)：每个任务都能独立跑通全量 e2e |
| Q-M8-3 | 自动降级后要不要提示用户 | (a) 不提示，只写前端日志；(b) 页面角落提示一次 | (a)：降级后观感完整，提示反而困惑 |
| B-M8-01 | 版本与安装 | (a) 完成后发 **1.6.0**，打本地安装包并做 1.5.0 → 1.6.0 升级冒烟（安装前单独确认）；(b) 只构建不安装 | (a) |
| B-M8-02 | 性能 spike 结果与门槛 | (a) T00 实测后按技术设计 19.5 校准门槛，超出先报告再决定；(b) 门槛照草稿固定不调 | (a) |
| B-M8-03 | 整体审查方式 | (a) task-reviewer 独立审查，重点看工作台保持挂载、焦点与 Esc、降级判定；(b) 自审 | (a) |

## 3. 任务清单

执行方式：**一次只做一个任务，做完汇报并停下，等用户放行后再做下一个**。状态：待做、进行中、阻塞、待验证、已验证。

| 编号 | 目标 | 修改范围 | 依赖 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- |
| T00 | 毛玻璃滚动性能 spike | `spikes/m8-glass/`（新，独立页面，不进业务代码）、`spikes/m8-glass/RESULT.md` | 用户批准本计划 | 真窗口（mygo + WebView2，e2e 构建）里 200 张毛玻璃卡片滚动的帧间隔 p95 与慢帧比例，毛玻璃开 / 关各测 3 轮；核实 WebView2 是否跟随 Windows「动画效果」开关；给出校准后的门槛建议 | 已验证（2 尺寸 × 3 轮 × 开 / 关：p95 均 5.1 ms、慢帧 0，两组无可测差异；门槛已改为刷新间隔倍数；动画设置「开」一侧一致，「关」一侧留给 T11 请用户验证） |
| T01 | 页面状态与外壳 | `appStore.ts`（`page`、`showPage`、旧名字别名）、`App.tsx`（标题栏提到根、工作台保持挂载并 `hidden` + `inert`）、`TitleBar.tsx`、`Workbench.tsx` / `SettingsPage.tsx` / `WelcomeView.tsx` 去掉各自的标题栏 | T00 | Vitest：`showPage` 的设置页草稿取消 / 确认、启动默认工作台、隐藏的工作台带 `hidden` 与 `inert`；既有 e2e 全部通过 | 已验证（`page` / `pendingPage` / `showPage` / `commitPendingPage` / `cancelPendingPage`；标题栏提到根组件，三个页面不再各画一份；工作台在技能页期间挂载并带 `hidden` + `inert`；设置页草稿走同一离开确认；新测试 10 条，全量 Vitest 291、Go 全过、全量 e2e 365） |
| T02 | 切换胶囊 | `PageSwitch.tsx`（新，替代 `SkillButton`）、标题栏样式 | T01 | 角标规则不变；`aria-current`、键盘可达；滑块弹性滑动与关闭动画时直接跳；组件测试 | 已验证（`PageSwitch` 两段胶囊：滑块按按钮实测位置定位、460ms 轻过冲、`prefers-reduced-motion: reduce` 直接跳；`aria-current="page"`；原生按钮键盘可达；角标沿用 `badgeCount`，`aria-label` 不变；过渡期设置页打开时仍打开旧浮层，T09 改走离开确认；新测试 4 条，全量 Vitest 295、Go 全过、全量 e2e 365） |
| T03 | 技能页骨架与导航 | `SkillsPage.tsx`、`SkillsNav.tsx`（新）、`skills-page.css`（新，含试点令牌）、样式范围检查脚本（接入 `test:web`） | T02 | 两组导航、选中、需要处理的小圆点、分发进度；窗口 < 1024px 图标栏；进入页面焦点在导航当前项；试点令牌只在 `.skills-page` 内（检查脚本正反对照） | 已验证（知识库 / 工具目录两组导航与计数、当前项 `aria-current`、需要处理的工具小圆点、已分发 X / Y 与进度条；无库时提示打开知识库；< 1024px 图标栏由媒体查询实现，真窗口留 T10/T11 验证；进入页面焦点在当前导航项；试点令牌只在 `.skills-page` 内，`check:skill-tokens` 接入 `test:web`，正反例均验证；新测试 10 条，全量 Vitest 305、Go 全过、全量 e2e 365） |
| T04 | 知识库视图 | `KbSkillsPage.tsx`（卡片结构与类名）、`skills-page.css` | T03 | 卡片网格 2–4 列、分组、悬停操作与 ⋯ 菜单、批量选择与底部操作条、安装卡、还原横幅位置；打开 `SKILL.md` / 在侧栏定位改为先回工作台；组件测试更新 | 已验证（知识库视图接入技能页主区：工具栏标题与数量、搜索 Ctrl K、网格 / 列表、安装；分组带计数与全选本组；卡片有分类图标块、两行描述、叠放工具或「未分发」、状态胶囊、悬停分发 / ⋯ 菜单；虚线安装卡；批量选择由卡片菜单进入，底部浮出操作条，可全选本组、取消、分发；还原横幅在页面内容顶部；打开 SKILL.md 与在侧栏定位都先回工作台；测试 10 + 6 条，全量 Vitest 313、Go 全过、全量 e2e 365） |
| T05 | 工具视图 | `ToolSkillsPage.tsx`、`skills-page.css` | T03 | 去掉 chip 行，按导航选中的工具过滤；按工具分组的列表；行操作；组件测试更新 | 已验证（技能页工具视图按导航选中的工具过滤；每组一个列表，组头是工具图标、名称与数量；行仍是图标、名称、路径、状态胶囊与悬停操作；chip 行、隐藏工具折叠、管理目录行与旧还原横幅只在旧浮层保留（页面样式隐藏，T09/T14 随浮层删除）；新增分组测试与导航过滤测试，全量 Vitest 315、Go 全过、全量 e2e 365） |
| T06 | 整页子视图（分发、收编、工具更新） | `DistributeDialog.tsx`、`AdoptDialog.tsx`、`ToolUpdateDialog.tsx` 的外壳；子视图路由 | T04、T05 | 面包屑 + 返回 + 底部操作条；Esc 逐级返回；提交逻辑与结果卡不变；组件测试更新 | 已验证（新增 `SkillsSubview` 外壳：面包屑（当前项加粗）、返回按钮、可滚正文、固定底部操作条；分发 / 收编 / 工具更新三个子视图换成整页外壳，提交逻辑、结果回调与 `data-testid` 不变；Esc 仍是先回列表；分发在页面主区替换列表，收编与工具更新覆盖主区；组件测试 319 通过，全量 e2e 365 全过） |
| T07 | 其余子视图与确认卡 | `InstallView`、`UpdateView`、`RestoreView`、`TargetsView` 的外壳；`UninstallDialog` 与覆盖确认的确认卡层 | T06 | 内容不变，外壳统一；确认卡覆盖整个技能页、Esc 等于取消；设置页「管理目录」入口直接进管理目录子视图 | 已验证（安装 / 更新 / 还原 / 管理目录四个视图换成整页面包屑外壳：共享 `SkillsCrumbs`、粘贴页内的顶栏与固定操作条、滚动正文；安装 Esc 逐级（候选 → 来源 → 列表），更新 Esc 取消；卸载、还原重建、移除登记三张确认卡盖住整个技能页（导航 + 主区），Esc 等于取消；设置页「管理目录」改为先切技能页再进管理目录子视图（经设置页离开确认），请求被消费不会影响下次进入；组件测试 322 通过、Go 全过、全量 e2e 365 全过） |
| T08 | 材质、动效与降级 | `skills-page.css`、降级判定（纯函数 + 采样钩子）、`prefers-reduced-motion` 规则 | T07 | DESIGN 13.3–13.5 落地；`data-glass` 开关；自动降级判定函数的单测；关闭动画效果时内容立即可见 | 已验证（毛玻璃加在卡片、工具列表、目标格、横幅、批量条与确认卡上，滚动容器不加；三团色场在页面底层 26–32 秒漂动；卡片入场逐张错开 38ms（最多 24 项）、悬停上浮 / 图标旋转 / 工具图标散开 / 操作滑入 / 指针光斑；`data-glass=on/off` 默认开，关闭时退成不透明底、色场隐藏、动效保留；自动降级判定是纯函数（p95 > 2R 且慢帧 > 5%），采样钩子空闲测 R、首次滚动采 120 帧，本次运行内保持并写前端日志；`prefers-reduced-motion` 下动画 / 过渡 1ms、色场停止、卡片直接可见；判定函数单测覆盖边界，Vitest 325、Go 全过、全量 e2e 365 全过；真窗口材质、对比度与系统关闭动画留 T11/T12） |
| T09 | 入口迁移与旧外壳删除准备 | 总览磁贴、设置页入口、`openTargetsManager` 改到技能页；e2e 与截图脚本的入口改写（`ac-m4`–`ac-m7`、`capture-m5/6/7`、`capture-new.py`） | T08 | 全量 e2e 通过；AC-SKM-11 断言按计划删去并记录（由 AC-PAGE-2 取代） | 已验证（总览磁贴与设置页入口都改到技能页；「技能 / 工作台」胶囊不再打开旧浮层；e2e 的 ac-m4–m7 全部改用 `showPage('skills')`、导航按钮与页面安装入口，逐个脚本通过后再跑全量：359 项 EXIT=0；AC-SKM-11 的 6 条断言按 PRD AC-PAGE-8 删去并由 T10 的 AC-PAGE-2 取代；截图脚本入口同步改写（本轮未执行）；Vitest 325、Go 全过） |
| T10 | M8 验收自动化 | `scripts/e2e/ac-m8.sh`（新） | T09 | AC-PAGE-1–6 全部断言通过；AC-PAGE-8 = 全量 e2e 通过 | 已验证（`scripts/e2e/ac-m8.sh`：AC-PAGE-1 流式续写 + 草稿 + 脏文档 + 滚动位置，AC-PAGE-2 切换 / 子视图 / 两次 Esc / 回工作台与焦点，AC-PAGE-3 设置草稿的离开确认，AC-PAGE-4 无库时工具目录可用且能卸载，AC-PAGE-5 744×640 图标栏与子视图完整可见，AC-PAGE-6 减少动效；AC-PAGE-8 = 全量 e2e 405 项通过。为满足 AC-PAGE-1 的滚动位置，隐藏工作台不再用 `display:none`，改为保持布局并被技能页覆盖 + `inert`——与技术设计 19.2 原写法不同；用户 2026-11-24 放行 T11 时确认，19.2 已同步） |
| T11 | 视觉对照 | `scripts/visual/capture-m8.py`（新）、`design/r2/shots-m8/`、UI 稿 11.5（真实截图差异）、DESIGN 13.7 实测 | T10 | 8 状态 × 浅 / 深 + 窄窗口 2 张真实截图；与原型逐张对照，差异写进 UI 稿；毛玻璃底上的对比度取样复测达标 | 已验证（`capture-m8.py` 在真窗口拍 18 张：8 状态 × 浅 / 深 + 两张窄窗；UI 稿 11.5 记录 7 条差异（工具目录完整列出、扫描状态行、状态胶囊、无 Ctrl K 提示、精简空状态、分发目标受真实探测限制、窄窗隐藏应用名）；对比度取样：默认橙描述 6.78 / 5.64、分类 5.21 / 4.89、路径 8.09 / 7.52、名称 12.53 / 10.61，6 套主题色 × 2 主题最低 4.53 / 5.64，全部 ≥ 4.5；Q-M8-4 按用户选择跳过系统开关，AC-PAGE-6 用 Chromium 参数验证） |
| T12 | 性能 | `scripts/e2e/perf-m8.sh`（新）、`perf-gate.py`（`kind=m8`） | T11 | 按 T00 校准后的门槛实测通过（AC-PAGE-7）；`kind=skills/m5/m6/m7` 不退化 | 已验证（`perf-m8.sh` 200 个技能真窗口：进入 p95 138.3ms、滚动 p95 0.34 个刷新间隔、慢帧 0、切回 p95 37.3ms，`kind=m8` 通过；M5–M7 与 `skills` 基准复跑全部通过且未退化） |
| T13 | 整体审查（按 B-M8-03） | 审查报告、本计划、受影响代码与测试 | T12 | 阻断 0，需改项已修或经用户接受 | 已验证（task-reviewer 独立审查 8bacb3e..HEAD：阻断 0、需改 1（技能页缺「检查更新」按钮与未检查 / 额度底栏状态，违反 PRD FR-SKM-UPD）已修并补测试；其余为建议与验证缺口：新增工作台被覆盖 / inert 的 e2e 断言，自动降级真实触发与旧浮层死代码留作已知风险，T14 清理；Vitest 327、Go 全过、全量 e2e 409） |
| T14 | 收尾（按 B-M8-01） | 删除浮层外壳与旧名字（Q-M8-2）、`skills-panel.css` 浮层规则；README、CLAUDE.md、路线、发布清单、本计划、`mygo.config.ts` | T13 | 旧名字零引用；1.6.0 安装包与 1.5.0 → 1.6.0 升级冒烟；文档同步；列提交；推送前询问 | 已验证（删除 `SkillsPanel` / `SkillButton` 与测试、`appStore` 三处旧状态与别名、浮层专属样式；`skills-panel.css` 更名为 `skills-shared.css`；旧名字在 `src/` 与 `scripts/` 零引用；版本 1.6.0；Vitest 319、Go 719、全量 e2e 409；安装包 1.6.0 与本机 1.5.0 → 1.6.0 覆盖安装冒烟完成，真实设置文件前后一致；提交本地完成，推送待用户确认） |

依赖顺序 T00 → T14，串行。

## 4. 单任务精确交接（摘要）

> 通用纪律：组件行为测试先行；测试只用临时目录、假 HOME 与本地假 GitHub；中文 UI、无 emoji；提交前扫描密钥与真实路径；每个任务结束时全量 e2e 通过（带 `-tags=e2e` 构建）。

- **T00**：spike 是独立的 mygo 小程序或 e2e 构建下的隐藏调试路由，不改业务组件。复用 `perf-ai.sh` 的 `requestAnimationFrame` 帧间隔采样；滚动用脚本驱动 `scrollTop`，每轮 5 秒。分别测 1440×900 与 960×720。结果写 `RESULT.md`，结论给出 19.5 与 19.4 门槛的校准值。
- **T01**：工作台保持挂载是本里程碑最大的风险点。隐藏用 `hidden` 属性加 `inert`，不用 `display:none` 的类名切换以外的手段，也不用 `visibility`；检查隐藏期间 `Workbench` 内部的 `ResizeObserver` / 尺寸计算不会因 0 尺寸写坏持久化的布局宽度（目前没有 `ResizeObserver`，但侧栏与文档宽度的夹取要在可见时才算）。设置页仍按现状卸载工作台（Q-M8-1）。
- **T02**：`PageSwitch` 放在原 `SkillButton` 的位置；角标数量函数 `badgeCount` 复用。
- **T03**：试点令牌放在 `skills-page.css` 的 `.skills-page` 选择器里；检查脚本扫描 `src/**/*.css` 与 `src/**/*.tsx` 的内联样式，`skills-page.css` 以外出现试点令牌名即失败。
- **T04 / T05**：只改结构与类名，数据流与回调签名不动，方便对照既有测试。
- **T06**：三个对话框从「浮层二级层」改为「主区子视图」时，保留它们各自的提交函数与结果回调；底部操作条承载原底栏按钮。
- **T07**：`openTargetsManager` 的 `targetsRequest` 计数机制保留，打开时先 `showPage("skills")`。
- **T08**：自动降级判定写成纯函数 `shouldDegrade(frameGaps, threshold)`，采样钩子只负责收集；判定结果本次运行内保持。
- **T09**：e2e 里 `openSkillsPanel()` 先保留别名可用，再逐个脚本改成 `showPage("skills")` 或点 `PageSwitch`；改完一个脚本跑一次该脚本。
- **T10**：AC-PAGE-1 用 M2 的假供应商开一段慢速流，切到技能页 5 秒后切回，断言流式文本继续增长、输入框文字与文档未保存标记不变、对话列表 `scrollTop` 不变。
- **T14**：删除 `SkillsPanel` 外壳、`SkillButton`、浮层专属样式与旧名字后，`rg "openSkillsPanel|skillsPanelOpen|closeSkillsPanel|skills-layer"` 在 `src/` 与 `scripts/` 应无结果。

## 5. 验证与验收

| 编号 | 给定条件 | 操作 | 预期结果 | 方式 |
| --- | --- | --- | --- | --- |
| V-SHELL-1 | 设置页打开、有未保存草稿 | `showPage("skills")`，取消 / 确认各一次 | 取消留在设置页、草稿还在；确认后进入技能页 | Vitest + e2e |
| V-SHELL-2 | 技能页显示中 | 查 DOM | 工作台节点仍在、带 `hidden` 与 `inert`；Tab 不进入工作台 | Vitest + e2e |
| V-SHELL-3 | 启动 | 打开应用 | 显示工作台，`page = "work"` | Vitest |
| V-NAV-1 | 窗口 960 | 进入技能页 | 导航是 56px 图标栏；无横向滚动 | e2e |
| V-ESC-1 | 分发子视图 | 按两次 Esc | 第一次回列表，第二次不离开页面 | Vitest + e2e |
| V-GLASS-1 | 采样帧间隔超过门槛 | 调用判定函数 | 返回降级；`data-glass` 变 `off` | Vitest |
| V-SCOPE-1 | 在 `skills-page.css` 以外写入 `--glass-card` | 跑样式检查 | 检查失败；删掉后通过 | 脚本正反对照 |
| V-PAGE-AC | AC-PAGE-1–8 | 见 PRD 17.6 | 全部通过 | e2e + 性能 |

门禁按技术设计第 10、19 节；实际命令按 README 第 5 节；每个任务的实际结果记在第 8 节账本。

## 6. 性能目标

| 指标 | 目标（技术设计 19.5 草稿，T00 后校准） | 依据 |
| --- | --- | --- |
| 200 张卡片滚动，毛玻璃开 | `scrollP95Frames = p95 / R ≤ 1.5`，`slowFrameRatio(> 2R) ≤ 5%`（R = 刷新间隔中位数；T00 实测本机 R=5.0 ms，开 / 关均 p95 5.1 ms） | T00 校准，按刷新率换算 |
| 进入技能页到首屏卡片可见 | p95 ≤ 300 ms | M4 扫描 p95 123 ms + 渲染 |
| 切回工作台（有流式回答） | 第一帧 ≤ 100 ms，流不中断 | AC-PAGE-1 |
| M4–M7 基准（`kind=skills/m5/m6/m7`） | 不退化 | G-PERF |

实测后按 G-PERF 阻断口径校准；超目标先报告再决定（B-M8-02）。

## 7. 协作、审查与执行方式

- 一次一个任务，汇报后停下；整份计划的批准不等于各任务放行。
- 不用子代理或并行会话执行编号任务；T13 的审查方式按 B-M8-03。
- 每个任务结束提交到本地（不推送）；推送只在 T14 列清单后由用户决定。
- 安装包安装（B-M8-01）执行前单独征得同意；安装前确认没有运行中的实例（托盘里的也要退出）。冒烟用 bash 继承的 `APPDATA` 环境变量启动，并在启动前后核对真实 `settings.json`（发布清单第 13 节的教训）。
- 全量 e2e 必须带 `-tags=e2e` 的构建；不要在打过正式包之后用 `E2E_SKIP_BUILD=1` 复用产物。跑全量前确认没有残留的应用实例（M7 T11 的教训：残留实例会让 ac-m4 焦点断言失败）。

## 8. 恢复入口与执行账本

新会话恢复时：读 CLAUDE.md 第 3 节、本计划第 3、8 节、相关契约；核对 `git log` 与工作区；不重跑已验证的任务，不凭聊天记忆补进度。

| 日期 | 任务 | 结果 | 提交 | 验证 | 下一项与放行状态 |
| --- | --- | --- | --- | --- | --- |
| 2026-11-24 | 计划 | v0.1；PRD 第 17 节 v2.7、UI 稿第 11 节、DESIGN 第 13 节已审核通过；技术设计第 19 节；路线 3.9.1 | 计划记录提交（本地，未推送） | 现状盘点（`App.tsx` 三选一渲染、三处 `TitleBar`、浮层定位与焦点、子视图两种承载方式、e2e 与截图脚本的入口引用数）；文档 UTF-8 无 BOM | 用户 2026-11-24 按建议批准本计划与技术设计第 19 节（Q-M8-1–3、B-M8-01–03 均按建议）；T00 待放行 |
| 2026-11-24 | T00 | `spikes/m8-glass/`（新）：独立 mygo spike 页面复刻技能页材质（200 张毛玻璃卡片 + 色场），automation eval 驱动 6 秒滚动并采帧间隔；两尺寸 × 3 轮 × 开 / 关共 12 组：p95 全为 5.1 ms（= 1 个刷新间隔）、慢帧 0，毛玻璃与不透明无可测差异；`loadEventEnd` 239–240 ms；系统「动画效果」= 开，页面 `prefers-reduced-motion` = no-preference（一致）；门槛改为 `scrollP95Frames ≤ 1.5`、`slowFrameRatio ≤ 5%`，自动降级改为「前 120 帧 p95 > 2R 且慢帧 > 5%」；RESULT 记录方法、原始数据与局限（脚本驱动滚动、后台窗口、未覆盖 4K / 低端集显） | 待提交（本地） | `bash spikes/m8-glass/run.sh` 两尺寸各 6 组全部 `done:true`；数据见 spikes/m8-glass/RESULT.md；期间修了三处驱动问题（结果值是 JSON 字符串导致轮询匹配失败、Git Bash 的 `$!` 不是 Windows PID 导致僵尸实例占端口、第二次运行先等到端口释放） | 待放行 T01 |
| 2026-11-24 | T01 | `appStore` 增加 `page`（默认 work，不持久化）、`pendingPage`、`showPage` / `commitPendingPage` / `cancelPendingPage`；设置页打开时 `showPage("skills")` 进入等待，由 SettingsPage 的既有离开确认（requestLeave）决定：无草稿直接关闭并跳转，有草稿弹确认，放弃后关闭并跳转、继续编辑则清掉等待；`App.tsx` 把 `TitleBar` 提到根组件并在三个页面去掉各自的标题栏，工作台改为 `page-host` 包裹、`page !== "work"` 时 `hidden` + `inert`（CSS 加 `.page-host[hidden]{display:none}` 防止被 grid 覆盖）；`Workbench` / `SettingsPage` / `WelcomeView` 的根元素收进父级网格；旧名字 `openSkillsPanel` 等本期保持不变，T09 再接别名。测试：`appStore.page.test.ts` 3 条、`App.test.tsx` 4 条（含工作台隐藏 / inert、设置页替换、欢迎页）、`SettingsPage.pending.test.tsx` 3 条（无草稿、放弃、继续编辑） | 本地 `23ad0e7` | `tsc` 通过；Vitest 44 文件 / 291 项全过；Go 全过；全量 e2e 365 项（15/16/22/47/37/60/54/46/68）EXIT=0 | 待放行 T02 |

| 2026-11-24 | T02 | `PageSwitch.tsx`（新）：标题栏「工作台 / 技能」两段胶囊，替代 `SkillButton`；滑块位置由按钮实测 `offsetLeft` / `offsetWidth` 得出，`ResizeObserver` 跟随宽度变化，460ms 轻过冲（`cubic-bezier(0.34, 1.36, 0.64, 1)`，首次定位不加过渡，`prefers-reduced-motion: reduce` 时直接跳）；`aria-current="page"` 标当前段，两段都是原生按钮、键盘可达；角标沿用 `badgeCount`，`aria-label` 仍是「技能，N 项需要处理」；过渡期兼容：设置页打开时点「技能」仍打开旧浮层（`data-testid="open-skills"` 落在技能段，AC-SKM-11 依赖），T09 换成 `showPage` 的离开确认；`.skill-badge` 样式从 `skills-panel.css` 迁到 `page-switch.css`（两处共用，只留一份）；新样式在 `main.css` 引入。测试：`PageSwitch.test.tsx` 4 条（`aria-current` 与切换、角标与可读标签、原生按钮可聚焦、设置页打开时的过渡行为） | 本地（随本任务提交） | `tsc` 通过；`check:bindings` 通过；全量 Vitest 45 文件 / 295 项全过；Go 全过；全量 e2e 365 项（15/16/22/47/37/60/54/46/68）EXIT=0。首轮全量 e2e 的 AC-SKM-11「focus back on pill」失败一次；单独重跑 ac-m4 60/60、紧接着全量重跑 365 全过，判为后台窗口焦点时序抖动（该断言在 T09 由 AC-PAGE-2 取代），未改测试 | 待放行 T03 |
| 2026-11-24 | T03 | `SkillsPage.tsx` / `SkillsNav.tsx`（新）：技能页面板（标题栏以下、左右下 8px 缝、16 圆角）与左侧导航；知识库一组为「全部技能」+ 扫描分类与计数，工具目录一组为「全部工具」+ 各工具计数与「管理目录…」，需要处理的工具带小圆点；底部「已分发 X / Y」、进度条与「N 个有更新 · N 个链接失效」，无库时只留「全部技能」并提示打开知识库后显示分发进度；窗口 < 1024px 由媒体查询收成 56px 图标栏（标题、计数与文字隐藏，只留图标、进度条，小圆点移到右上）。`SkillsPage` 进入时刷新扫描、把焦点放到当前导航项；选中的分类或工具在扫描里消失时回退到全部；主区暂为空壳，T04/T05 接入。`skillsView` 增加 `toolNeedsAttention`、`attentionCounts`、`distributionProgress`（`badgeCount` 改为复用 `attentionCounts`）。`skills-page.css`（新）：试点令牌按 DESIGN 13.2 定义在 `.skills-page` 与深色覆盖里，所有选择器以 `.skills-page` 开头。`scripts/check-skill-tokens.mjs`（新）扫描 `src/**/*.{css,tsx}`：令牌只能出现在 `src/styles/skills-page.css`，且必须在 `.skills-page` 块内；接入 `test:web`。`App.tsx` 在 `page === "skills"` 时渲染技能页 host，工作台保持挂载并 `hidden` + `inert`。测试：`skillsView.test.ts` 3 条、`SkillsNav.test.tsx` 4 条、`SkillsPage.test.tsx` 2 条、`App.test.tsx` 增 1 条 | 本地（随本任务提交） | `tsc` 通过；`check:skill-tokens` 正常通过，两处反例（其他样式文件引用令牌、`skills-page.css` 的 `.skills-page` 之外引用令牌）分别报错，删除后恢复；全量 Vitest 48 文件 / 305 项全过；Go 全过；全量 e2e 365 项（15/16/22/47/37/60/54/46/68）EXIT=0（最终树复跑）。窄窗口布局与真实焦点行为留 T10/T11 真窗口验证 | 待放行 T04 |
| 2026-11-24 | T04 | `KbSkillsPage.tsx` 重构为页面结构（UI 稿 11.2）：工具栏（标题 + 「N 个 · N 个分类」/ 搜索时「N / M 个」、搜索框、网格 / 列表、安装）、分组头（分类 + 计数 + 全选本组）、卡片（分类图标块、名称、分类、两行描述、叠放工具图标或「未分发」、状态与 GitHub 胶囊、悬停「分发」/「更新」、⋯ 菜单含打开 / 定位 / 批量选择 / 更新）、分组末尾虚线安装卡、底部批量操作条（已选 N / 取消 / 分发到…）、Ctrl K 聚焦搜索；原工具栏「批量选择」入口移进卡片菜单，`data-testid` 保留。`SkillsPage.tsx` 在主区接入知识库视图并按导航分类过滤；还原横幅从工具页移到页面内容顶部（M103 预览）；打开 SKILL.md / 在侧栏定位先 `showPage("work")` 再执行；分发、安装、更新、还原复用现有组件（外壳未改，T06/T07 换成整页子视图），更新提出的变更计划会提示回工作台确认。`skills-page.css` 增加知识库视图样式（`.skills-page` 前缀，宽屏最多 4 列）。测试：`KbSkillsPage.test.tsx` 重写为 10 条（标题 / 计数、分类过滤、搜索计数、Ctrl K、网格切换、批量、全选与取消、菜单、安装入口、更新状态），`SkillsPage.test.tsx` 扩为 6 条（焦点、分类过滤、先回工作台再打开文档、还原横幅位置、分发对话框、扫描刷新后选中不丢） | 本地（随本任务提交） | `tsc` 通过；`check:skill-tokens` 通过；全量 Vitest 48 文件 / 313 项全过；Go 全过；全量 e2e 365 项（15/16/22/47/37/60/54/46/68）EXIT=0。真窗口视觉留 T11；工具视图与子视图外壳分别留给 T05、T06/T07；旧浮层仍渲染同一组件，页面专属样式不作用于浮层（浮层保留原有样式，T14 删除） | 待放行 T05 |
| 2026-11-24 | T05 | `ToolSkillsPage.tsx`：技能行改为按工具分组渲染（组头 = 工具图标 + 名称 + 数量，组内按名称排序，每组一个列表容器）；行结构、`data-testid` 与卸载 / 收编 / 更新 / 重新分发 / 打开目录操作不变。为保持旧浮层与既有 e2e 可用，chip 行、未检测工具折叠、管理目录行与组件内的还原横幅仍保留在组件里，技能页用 `.skills-page` 作用域样式隐藏它们（T09 迁移入口、T14 随浮层删除）。`SkillsPage.tsx` 在导航选择工具目录时渲染工具视图，按选中的工具过滤扫描分组（「全部工具」显示全部）；`onChanged` 刷新扫描，结果进页面横幅。`skills-page.css` 增加工具视图样式（组头、列表、行网格、路径等宽截断、操作悬停出现）。测试：`ToolSkillsPage.test.tsx` 增分组断言，`SkillsPage.test.tsx` 增「导航选中的工具只显示该组、其他组不渲染」；全量 Vitest 48 文件 / 315 项 | 本地（随本任务提交） | `tsc` 通过；`check:skill-tokens` 通过；全量 Vitest 315 全过；Go 全过；全量 e2e 365 项 EXIT=0（首轮仅 AC-SKM-11「focus back on pill」偶发失败，整轮复跑全过；该断言与 T02 同一已知后台焦点抖动，T09 由 AC-PAGE-2 取代，未改测试）。真窗口视觉留 T11；子视图外壳 T06/T07 | 待放行 T06 |
| 2026-11-24 | T06 | `SkillsSubview.tsx`（新）：整页子视图外壳，面包屑 + 返回（可禁用）+ 滚动正文 + 固定操作条。`DistributeDialog`、`AdoptDialog`、`ToolUpdateDialog` 去掉浮层卡片，改用外壳：面包屑分别为「全部技能 / 技能 / 分发到…」「工具技能 / 技能 / 收进知识库」「工具技能 / 工具 / 更新 技能」；提交逻辑、结果回调、Esc 行为与 `data-testid` 全部不变。`SkillsPage` 把分发子视图放进主区路由（打开时替换列表，关闭回列表）；收编 / 工具更新仍由 `ToolSkillsPage` 内部路由，外壳改为覆盖主区。`skills-panel.css` 调整子视图基座（三行网格、正文滚动、返回与面包屑样式，保留确认卡用的居中卡片），`skills-page.css` 增加页面内的子视图样式（不透明底、24px 内边距、固定操作条、目标网格）。**e2e 附带修复（经用户选择）**：AC-SKM-11 的焦点断言在后台窗口下偶发失败，根因是 Windows/WebView2 不向非前台窗口派发 focus 事件；`ac-m4.sh` 增加 `foreground_app`（PowerShell ShowWindow / SetForegroundWindow / AppActivate），在断言前把测试窗口切到前台，不改产品代码、不放宽断言。测试：新增 `SkillsSubview.test.tsx` 2 条、`DistributeDialog.test.tsx` +1（面包屑与返回 / Esc）、`SkillsPage.test.tsx` +1（Esc 回列表） | 本地（随本任务提交） | `tsc` 通过；`check:skill-tokens` 通过；全量 Vitest 49 文件 / 319 项全过；Go 全过；全量 e2e 365 项（15/16/22/47/37/60/54/46/68）EXIT=0。前台化只出现在 ac-m4 的这一条断言前，会短暂抢占一次前台焦点 | 待放行 T07 |
| 2026-11-24 | T07 | `SkillsSubview.tsx` 抽出共享 `SkillsCrumbs`；`InstallView`、`UpdateView`、`RestoreView`、`TargetsView` 的标题改为面包屑（全部技能 / 安装、知识库技能 / 更新…、工具技能 / 还原、工具技能 / 管理目录…），页面作用域样式把它们统一成整页外壳（不透明底、24px 内边距、吸顶标题、底部操作条吸附），内容与 `data-testid` 不变；`InstallView` 补 Esc（候选 → 来源 → 列表），`UpdateView` 补 Esc（取消）。`UninstallDialog`、`RestoreView` 的重建确认、`TargetsView` 的移除确认加 `skills-confirm-layer`，页面内固定覆盖标题栏以下的整个技能页，Esc 仍先取消确认。`appStore.openTargetsManager` 改为先 `showPage("skills")`（设置页打开时走 `pendingPage` 离开确认）再累加 `targetsRequest`；新增 `consumeTargetsRequest`，取消离开时一并清零；`SkillsPage` 消费请求后直接打开管理目录子视图，返回回到工具列表。测试：`UpdateView.test.tsx` +1（Esc）、`SkillsPage.test.tsx` +1（请求打开管理目录并消费）、`appStore.page.test.ts` +1（入口路由与取消清零）、`AboutSection.test.tsx` 断言改为进入技能页 | 本地（随本任务提交） | `tsc` 通过；`check:skill-tokens` 通过；全量 Vitest 49 文件 / 322 项全过；Go 全过；全量 e2e 365 项（15/16/22/47/37/60/54/46/68）EXIT=0（含 ac-m6 的设置入口断言）。真窗口视觉留 T11 | 待放行 T08 |
| 2026-11-24 | T08 | `skillsGlass.ts`（新）：`percentile` / `median` / `shouldDegrade(frameGaps, refreshMs)`，门槛按 T00 校准（p95 > 2R 且 > 2R 的帧占比 > 5%）。`useSkillsGlass.ts`（新）：进入页面先采 30 帧空闲间隔取中位数 R，第一次滚动采 120 帧判定；判定结果本次运行内保持（模块级），降级时写一条不含路径的前端日志。`SkillsPage` 根加 `data-glass` 与三团色场层；`KbSkillsPage` 卡片带 `--i` 错开序号与指针光斑坐标。`skills-page.css`：毛玻璃（卡片 / 列表 / 目标格用 `--glass-card`，横幅 / 操作条 / 确认卡用 `--glass-strong`）、`@supports not (backdrop-filter)` 与 `data-glass=off` 退不透明、色场漂动、卡片入场（`backwards` 填充，结束后悬停位移生效）、悬停 / 按压 / 图标 / 叠放 / 操作条动效，`prefers-reduced-motion` 全部降级。测试：`skillsGlass.test.ts` 2 条（分位数 / 中位数、降级边界），`SkillsPage.test.tsx` +1（默认 `data-glass=on` 与色场存在） | 本地（随本任务提交） | `tsc` 通过；`check:skill-tokens` 通过（51 处，全在 `.skills-page` 内）；全量 Vitest 50 文件 / 325 项全过；Go 全过；全量 e2e 365 项 EXIT=0。真窗口材质、模糊底对比度、系统关闭动画效果后的表现与性能复测留 T11/T12（Q-M8-4） | 待放行 T09 |
| 2026-11-24 | T09 | 入口迁移：`PageSwitch` 去掉「设置页打开时仍开旧浮层」的过渡分支，统一走 `showPage('skills')`（设置页仍由离开确认把关）；总览磁贴改为 `showPage('skills')`；设置页「管理目录」保持 T07 的直达子视图。技能页补两处入口缺口：导航切换时清掉分发 / 安装 / 更新 / 还原子视图，工具视图加「安装」按钮（`kb-install-top`，与知识库视图一致），导航底部加 `skills-status`（后台扫描中 / 上次扫描）。e2e：ac-m4–m7 的 `open_panel` 改成 `showPage('skills')` 等 `skills-page`，`open_kb` / `open_tools` 改成点导航「全部技能 / 全部工具」，安装入口改成 `kb-install-top`，管理目录改成导航「管理目录…」，关闭改成 `showPage('work')`；ac-m4 删除 AC-SKM-11 的浮层断言（保留侧栏「导入技能」断言），按 PRD AC-PAGE-8 由 T10 的 AC-PAGE-2 取代；`foreground_app` 助手随断言一起删除。截图脚本 capture-m5/6/7、capture-new 的入口与 tab 选择器同步改写（未运行，T11 用新脚本复拍） | 本地（随本任务提交） | `tsc` 通过；`check:skill-tokens` 通过；全量 Vitest 50 文件 / 325 项全过；Go 全过；ac-m4 54/54、ac-m5 54/54、ac-m6 46/46、ac-m7 68/68，全量 e2e 359 项（15/16/22/47/37/54/54/46/68）EXIT=0；总数比 T08 少 6 条，正是删掉的 AC-SKM-11 | 待放行 T10 |
| 2026-11-24 | T10 | `scripts/e2e/ac-m8.sh`（新）：临时 APPDATA / 假 HOME / 假供应商，覆盖 AC-PAGE-1–6。AC-PAGE-1 用假供应商的长回答：切页 5 秒后断言回答长度继续增长、草稿文本、文档脏标记与聊天滚动位置不变；AC-PAGE-2 经 DOM 点击切页（自动化后端拒绝 WebView2 的指针输入，只能 eval），打开分发子视图后两次 Esc，第二次仍留在技能页，回工作台后显式执行真实点击会带来的 focus 并断言焦点在「工作台」段；AC-PAGE-3 在设置里改 steering 制造草稿，取消 / 放弃两条路径；AC-PAGE-4 用空 APPDATA 启动，无库时知识库空状态、工具目录可看可卸载；AC-PAGE-5 用 Win32 `SetWindowPos` 把窗口缩到 744×640，断言 56px 图标栏、无横向滚动、分发子视图与卸载确认卡完整可见；AC-PAGE-6 以 `WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS=--force-prefers-reduced-motion` 重启，断言媒体查询生效、卡片无入场动画、色场不动、悬停位移规则为 none（自动化无法真实悬停，规则存在性 + 当前 transform 两条一起验，真实观感留 T11）。**外壳实现调整（待用户确认）**：Tailwind preflight 用 `@layer base` 的 `!important` 隐藏 `[hidden]`，`display:none` 会丢滚动位置与虚拟行；改为同一层覆盖 `display:grid !important`，让工作台保持布局、由不透明的技能页宿主覆盖，`inert` 保证不可聚焦，两个 `.page-host` 共用同一网格单元。测试：`main.css` 的宿主规则；全量 Vitest 50 文件 / 325 项 | 本地（随本任务提交） | `tsc` 通过；`check:skill-tokens` 通过；全量 Vitest 325 全过；Go 全过；ac-m8 46/46；全量 e2e 405 项（15/16/22/47/37/54/54/46/68/46）EXIT=0 | 待放行 T11 |
| 2026-11-24 | T11 | `scripts/visual/capture-m8.py`（新）：临时 APPDATA / 假 HOME / 真窗口，拍 8 个状态 × 浅 / 深（kb、kb-select、tools、distribute、uninstall、search-empty、scanning、novault）加 `kb-narrow-light`、`uninstall-narrow-light` 共 18 张，存 `design/r2/shots-m8/`；拍摄时注入页面级动效关闭，与原型一致；同时按真实像素取样卡片描述、分类名、工具路径与技能名，并扫 6 套主题色 × 2 主题的导航当前项强调色文字。视觉核对修了 5 处实现问题：工具视图缺标题 / 数量 / 安装工具栏且分组被拉伸、窄窗口标题栏没有隐藏应用名导致换行、窄窗工具栏安装按钮挤成竖排、子视图面包屑被推到最右、确认卡在固定层里被裁切。文档：UI 稿 11.5 差异与对比度、DESIGN 13.7 实测、技术设计 19.2 隐藏方式同步与 Q-M8-4 结论（用户选择跳过系统开关，AC-PAGE-6 用 `--force-prefers-reduced-motion` 验证） | 本地（随本任务提交） | `tsc` 通过；`check:skill-tokens` 通过；`capture-m8.py` 18 张全部生成；对比度最低 4.53（导航当前项）、描述 5.64，均 ≥ 4.5；全量 Vitest 50 文件 / 326 项全过；Go 全过；全量 e2e 405 项（15/16/22/47/37/54/54/46/68/46）EXIT=0 | 待放行 T12 |
| 2026-11-24 | T12 | `scripts/e2e/perf-m8.sh`（新）：200 个技能、毛玻璃开、真窗口。进入技能页测 5 轮（切回工作台再进，rAF 到首张卡片可见）；空闲 40 帧取刷新间隔 R；卡片网格滚动 3 轮 × 6 秒（rAF 驱动 `scrollTop`）记录 p50 / p95 / max 与 > 2R 的慢帧比例；切回工作台测 5 轮首帧。指标：`enterP95Ms` 138.3、`scrollP95Frames` 0.34、`slowFrameRatio` 0、`switchBackMs` 37.3，未触发自动降级。`perf-gate.py` 增加 `kind=m8`（1.5 / 0.05 / 300 / 100）。回归：`perf-m4`（skills）106.6 / 31.8，`perf-m5` 161.2 / 60.6，`perf-m6` 114.1 / 238.4 / 27.7 / 14.9，`perf-m7` 53.2 / 15.5 / 72.7，全部通过且不高于既有记录。README、技术设计 19.5、DESIGN 13.7 记录实测值 | 本地（随本任务提交） | `bash scripts/e2e/perf-m8.sh` EXIT=0（窗口在后台，R 约 24.7ms；比值口径按 T00）；四个既有性能脚本逐个 `perf-gate` 通过 | 待放行 T13 |
| 2026-11-24 | T13 | task-reviewer 独立审查 M8 全部改动（重点：工作台保持挂载、焦点与 Esc、降级判定、入口迁移、契约与测试、安全边界）。结论：阻断 0；需改 1——技能页丢了 PRD FR-SKM-UPD 的手动「检查更新」与「未检查来源 / 额度用完」底栏状态。修复：`SkillsPage` 保存 M93 结果并在打开与事件后刷新，`SkillsNav` 底栏恢复「检查更新」按钮、上次检查时间、未检查来源与额度提示（`skills-checked` / `skills-check-updates` / `skills-unchecked` / `skills-rate-limit` 沿用旧 testid），样式加 `.skills-nav-updates`（窄窗隐藏），组件测试加手动检查与三类状态断言。审查其余项处理：ac-m8 增加「工作台被覆盖时 inert」「技能页宿主不透明覆盖、中心点不在工作台」「手动检查按钮可用」三条 e2e；无 GitHub 来源的夹具里 M93 不会产生检查时间，e2e 只验按钮可用，强制检查由组件测试覆盖。保留为已知风险/记录：自动降级链在真窗口未实际触发（T12 余量很大）、设置页打开时胶囊仍高亮「工作台」的过渡中间态、旧浮层与 `SkillButton` 死代码按计划 T14 删除、真实系统动画开关按用户选择跳过。T11 截图按新底栏重拍 18 张，UI 稿 11.5 第 2 条同步 | 本地（随本任务提交） | task-reviewer 报告（阻断 0、需改 1、建议 8）；修复后 `tsc` 通过、`check:skill-tokens` 通过；全量 Vitest 50 文件 / 327 项；Go 全过；全量 e2e 409 项（15/16/22/47/37/54/54/46/68/50）EXIT=0；ac-m8 50/50 | 待放行 T14 |
| 2026-11-24 | 图标替换（用户请求，非编号任务） | 用户提供 `docs/icon/Glossy Blue Toolbox Cube Icon.png`（1254×1254、黑色圆角底）。处理：黑底转透明并做圆角边缘渐变、裁到内容边界加 2% 留白，生成 `resources/icon.png`（512）与 `resources/icon-tray.png`（32）。`bun run build` 重新生成 exe 与 1.5.0 安装包；从 exe 提取的 32px 图标与 Vite 构建后的 `dist/assets/icon-*.png` 哈希均与新图一致 | 本地（随本任务提交） | 构建成功；exe 提取图标与产物资源核对通过。Web/Go 测试与图标无关，未重跑全量 | 待放行 T14 |
| 2026-11-24 | T14 | 收尾：删除 `SkillsPanel.tsx`、`SkillButton.tsx` 与 `SkillsPanel.test.tsx`；`App.tsx` 不再挂载浮层；`appStore` 删除 `skillsPanelOpen` / `openSkillsPanel` / `closeSkillsPanel`；浮层专属样式（`.skills-layer`、`.skills-backdrop`、`.skills-panel`、`.skills-head`、`.skills-tabs`、`.skills-count`、`.skills-install`、`.skills-body`、`.skills-groups`、`.skills-foot`、`.skill-button` 等）删除，共享样式文件更名为 `skills-shared.css`；`capture-m5/6/7/new` 的旧入口改为 `showPage`；`mygo.config.ts` 版本 1.6.0。文档：README、CLAUDE.md、PLAN-M0（M8 行与 3.9.1）、技术设计 19.2（别名已删）、发布清单第 15 节、本计划。安装冒烟经用户授权由本会话执行：静默 `/S` 退出码 0，安装目录 1.6.0，真实 `settings.json` 前后 SHA256 一致，应用启动 10 秒存活、标题正确；关闭询问导致 `CloseMainWindow` 未退出，由本会话停止 | 本地（随本任务提交） | `tsc` 通过；`check:skill-tokens` 通过；Vitest 49 文件 / 319 项；Go 719；全量 e2e 409 项 EXIT=0；`rg` 旧名字零引用；1.6.0 安装包与升级冒烟见发布清单第 15 节。提交清单见本轮汇报，推送待用户确认 | 待用户确认推送 |

## 9. 自查

- FR 覆盖：FR-SKM-PAGE 的入口与外壳 → T01、T02；导航与主区 → T03–T05；子视图 → T06、T07；材质试点 → T08；其他入口 → T09；AC-PAGE-1–8 → T10、T12（AC-PAGE-7）、T09（AC-PAGE-8）。
- 契约前置：不改 API 与存储（技术设计 19.8）；界面契约已审核；技术设计第 19 节与本计划一起审。
- 风险前置：毛玻璃性能放在 T00，结论不理想时先报告再决定是否调整 T08；工作台保持挂载放在 T01，后面每个任务都在它之上验证。
- 授权：安装、推送、人工亲测分别单独确认（第 7 节）。
