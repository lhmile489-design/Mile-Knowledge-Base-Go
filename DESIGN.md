# 全局 UI 设计契约

> 状态：阶段 06 定稿 v1.0（用户 2026-11-18 审核通过）。
> 适用迭代：PRD v1.3（Go 重写版，G2 与旧版 UI/UX 一致）。
> 本文件是「规范与语义」的单一契约；数值的单一实现源是阶段 10 建立的 `src/styles/`（由旧版 `tokens.css` 原样移植并拆分）。本文件与 `src/styles/` 数值冲突时先澄清，不自行二选一。
> 旧版 DESIGN.md 中的 verify-ui-* 断言是本仓不沿用的验证机制（旧版专有），本文件只迁移规范本身，不迁移断言编号。

## 1. 设计来源与范围

| 项目 | 内容 |
| --- | --- |
| 需求依据 | docs/PRD.md v1.3（FR-WIN、FR-THEME、FR-DOC、FR-SET、FR-AI-*、10.3） |
| 视觉基线 | 旧版 v2.7.0 运行截图 `design/baseline/`（manifest.json + shots/*.png，用户审核通过）；页面与状态清单见 docs/UI设计稿.md |
| 规范基线 | 旧版 `DESIGN.md`（E:\WebApp\Mile-Project\Mile-Knowledge-Base-master\DESIGN.md，下称「旧版规范」） |
| 数值基线 | 旧版 `src/renderer/src/assets/tokens.css`（4913 行，下称「旧版 tokens.css」，文中来源标注其行号） |
| 平台 | 仅 Windows 11（PRD Q1）；webview 为系统 WebView2 |
| 有意差异 | 仅一处（D-006）：所有已登记知识库都断开时显示欢迎页（旧版进工作台空壳）；其余与旧版一致 |
| 不在范围 | 业务规则、接口字段（PRD / API.md）；逐页像素（看截图编号） |

## 2. 设计方向

极简、冷调、编辑器优先。信息密度靠排版与留白节奏承载，不靠装饰（旧版规范 §一）。核心原则：**每个状态变化只动一个属性，且动得极少**。

允许的全部视觉手段（旧版规范 §一 原表迁移）：

| 手段 | 幅度 | 用在哪 |
| --- | --- | --- |
| 边框换色 | 1px，仅 border-color | 状态表达的唯一手段 |
| 背景微提 | 相邻层级差一档 | 卡片 / 抬升面 |
| 位移 | hover 上浮 2px，按压下沉 1px | 可交互卡片、按钮 |
| 缩放 | 0.96 | 仅小控件按压 |
| 阴影 | 两档：`--shadow-panel`（浅 0.06，深 none）/ `--shadow-float` | 面板与浮层 |
| 光晕 | box-shadow spread，无 blur | 仅「运行中」圆点 |
| 强调块 | 6px 实心方块 | 仅 eyebrow 前缀 |
| 发丝线 | 1px `--raw-line` | 分节、卡内分隔 |
| 强调色 | 单一靛蓝 `--raw-accent` | 仅交互：焦点、选中、每视图唯一主操作 |
| 浮动面板 | 圆角 16，与画布间 8px 缝 | 工作台三块面板 + Bento 网格 + 设置两块面板 |
| 面板分色 | 浅：同色相、侧栏低一档；深：纯明度阶梯 | 仅工作台三面板与设置两面板 |
| 毛玻璃 | `--glass` + blur(18px) saturate(1.4) | **只给浮层**：下拉菜单、会话信息浮层、popover |
| 弹性缓动 | `--ease-spring` + 220ms（任何过渡 ≤240ms） | 按压、开关、展开折叠、浮层出现 |
| 状态染底 | `color-mix(tone 16%, card)` 底 + `color-mix(tone 72%, strong)` 字 | **只给 `.pill` 胶囊与小标签**，必须配文字 |
| 圆角四档 | 16 / 12 / 10 / 999 | 全站；具名例外：任务复选框 3、过程步骤节点 2、进行中指示器方块 1 |

**禁止**：渐变、多层阴影、任何 emoji、任何品牌 logo、任何超过 240ms 的过渡、浮层之外的模糊、胶囊之外的染底、四档之外的圆角。唯一已授权的动画例外见 §6 进行中指示器。

新增视觉手段必须属于上表；不在表内的需要先提出并获用户确认，不静默发明。

> 具名例外（用户 2026-11-24 审核通过）：技能页试点毛玻璃卡片、背景色场与较长的非线性动效，只在 `.skills-page` 范围内生效，见第 13 节。其他界面仍按上表与禁止项。

## 3. 设计令牌

三层结构（方向单一）：**组件只用语义令牌 → 语义令牌指向原始变量 `--raw-*` → 原始变量按 `<html data-theme>` 整体翻转**。CSS 只写 `:root` 浅 + `[data-theme='dark']` 深两块；用户显式选择覆盖系统（FR-THEME），不读 `prefers-color-scheme` 定主题。

### 3.1 原始层颜色（旧版 tokens.css :26-172 浅 / :180-218 深）

| 令牌 | 浅色 | 深色 | 用途 |
| --- | --- | --- | --- |
| `--raw-canvas` | `#eceef1` | `#0e0f11` | 最底层画布；**= 窗口底色（PRD Q6）** |
| `--raw-card` | `#ffffff` | `#1c1e22` | 卡片 |
| `--raw-raised` | `#eceef1` | `#26282d` | 输入框、分段槽、用户气泡 |
| `--raw-sunken` | `#f2f3f5` | `#16171a` | 历史栏、代码块、表头 |
| `--raw-line` | `#e1e3e7` | `#2b2e34` | 发丝线 |
| `--raw-line-strong` | `#c9cdd4` | `#3b3f47` | 控件轮廓：输入框、头像框、复选框 |
| `--raw-strong` | `#15171c` | `#f3f4f6` | 标题、用户气泡文字（`--raw-ink` 为别名） |
| `--raw-text` | `#24272e` | `#e6e7ea` | 正文 |
| `--raw-muted` | `#4b505a` | `#a6aab3` | 次级文字 |
| `--raw-faint` | `#5c616b` | `#9a9ea8` | 辅助说明（作文字必须 ≥4.5:1） |
| `--text-on-ink` | `#ffffff` | `#0e0f11` | `--raw-ink` 上的前景 |
| `--live`（`--ok` 别名） | `#15803d` | `#4ade80` | 状态色（只染 1px 边框 / pill，见 §5） |
| `--info` | `#4338ca` | `#8b8ff8` | 同上 |
| `--warn` | `#92400e` | `#fbbf24` | 同上 |
| `--fault` | `#b91c1c` | `#f87171` | 同上 |
| `--idle` | `#6b7280` | `#8b8f98` | 同上 |
| `--raw-accent` | `#e87226` | `#fa8234` | 强调色：只表达交互；其余预设与自定义见 §12 |
| `--raw-accent-hover` | `#d16722` | `#fb914c` | 悬停（不带文字的元素） |
| `--raw-accent-active` | `#ba5b1e` | `#dc722e` | 按压（不带文字的元素） |
| `--raw-accent-soft` | `#fceee5` | `#4d3426` | 选中行底、菜单选中项 |
| --raw-accent-text | `#aa541c` | `#fa8234` | 强调色文字（卡片 / 画布 / 软底上 ≥4.5:1） |
| `--raw-accent-btn` 一族（`-hover` / `-active`） | `#ba5b1e` / `#a7521b` / `#954918` | `#fa8234` / `#fb914c` / `#dc722e` | 带文字的强调底：主按钮、发送按钮、选中勾、开关底 |
| `--raw-on-accent`（`--text-on-accent` 别名） | `#ffffff` | `#111214` | 强调色上的前景 |
| `--focus` | `#e87226` | `#fa8234` | 焦点环 |
| `--shadow-panel` | `0 4px 14px rgb(20 22 28/.06)` | `none` | 面板阴影；`--shadow-lift-lg` 为别名 |
| `--shadow-float` | `0 16px 40px rgb(20 22 28/.16)` | `0 16px 40px rgb(0 0 0/.5)` | 浮层阴影 |
| `--glass` | `rgb(255 255 255/.72)` | `rgb(28 30 34/.72)` | 浮层毛玻璃底 |
| `--glass-line` | `rgb(255 255 255/.6)` | `rgb(255 255 255/.08)` | 浮层描边 |
| `--raw-panel-sidebar` | `#f4f5f7` | `#141518` | 侧栏 / 文档竖轨 / 设置导航 |
| `--raw-panel-editor` | `#ffffff` | `#18191d` | 文档面板 / 设置内容 |
| `--raw-panel-agnes` | `#ffffff` | `#1c1e22` | 对话面板（深色最亮一档） |

强调色**只表达交互**：焦点环、选中态、每个视图最多一个主要操作按钮；永远不表达状态。阅读模式正文链接也用强调色。

**面板相对层级局部变量**（每个 `[data-panel]` 定义，随主题翻转，旧版 tokens.css :414-445）：

| 令牌 | 浅色 | 深色 | 语义 |
| --- | --- | --- | --- |
| `--panel-bg` | 本面板令牌 | 本面板令牌 | 面板底色；面板内「跟面板同色」的容器用它，不写死 `--raw-card` |
| `--panel-card` | `#ffffff` | `var(--raw-raised)` | 比面板亮一档的浮卡色：`.surface-card`、`.turn`、`.runtime-controls` |
| `--panel-sunken` | `var(--raw-sunken)` | `var(--raw-sunken)` | 比浮卡低一档的凹区/内卡色（全自动后果卡、diff 预览区） |

`.surface-card` 底色为 `var(--panel-card, var(--raw-card))`：面板内自动亮一档，面板外回落卡色。

### 3.2 字体与字号（tokens.css :97-163）

| 令牌 | 值 | 用途 |
| --- | --- | --- |
| `--font-ui` | `-apple-system, BlinkMacSystemFont, 'Segoe UI Variable Display', 'Segoe UI', 'Microsoft YaHei UI', system-ui, sans-serif` | 全部界面文字 |
| `--font-mono` | `'Cascadia Code', 'SF Mono', 'Menlo', 'Consolas', 'DejaVu Sans Mono', monospace` | 代码、路径、源码模式 |

字号阶梯（只许用 `.text-*` 类或 `var(--text-*)`，不写一次性字号）：

| 令牌 | 值 | 典型用途 |
| --- | --- | --- |
| `--text-caption` | 11px | 角标、时间戳、胶囊、键帽 |
| `--text-meta` | 12px | 分节标签、元信息、提示文案 |
| `--text-body-sm` | 13px | 紧凑正文、树行、Agent 名、行内小按钮 |
| `--text-body` | 14px | 默认正文、对话气泡、按钮 |
| `--text-read` | 15px | 文档阅读模式正文 |
| `--text-title` | 18px | 页面 / 模块标题 |
| `--text-display` | 24px | 空状态主标题、h1 |

旧名别名：`caption-lg=caption`、`meta-lg=meta`、`body-md=body-sm`、`body-lg=read`（tokens-map.md）。数字与计数加 `font-variant-numeric: tabular-nums`（`.tabular`/`[data-numeric]`，:269-273）。

字体按视图分工：源码 / 分栏左栏用 `--font-mono` 13px（`--text-body-md`）行高 1.65（`.source-view`/`.source-editor` :1866-1974）；阅读正文用 UI 字体 15px 行高 1.75（`.prose` :4620）。

> 漂移注记：旧版规范 §三 写「源码 12.5px」，旧版 tokens.css 实际为 `--text-body-md`=13px。以 tokens.css 为准。

### 3.3 间距、圆角、动效（tokens.css :112-144）

| 令牌 | 值 | 用途 |
| --- | --- | --- |
| `--titlebar-h` | 40px | 自绘标题栏高度 |
| `--agnes-rail-w` | 40px | 文档面板折叠竖轨宽 |
| `--gap-panel` | 8px | 面板缝（= 拖拽条命中区） |
| `--gap-tile` | 12px | 磁贴 / 卡间距 |
| `--pad-panel` | 8px | 面板内边距 |
| `--pad-tile` | 16px | 磁贴内边距 |
| `--radius-panel` | 16px | 工作台三块面板、Bento 模块、设置两块面板 |
| `--radius-card` | 12px | 卡（审批 / 计划 / 冲突 / surface-card）、用户气泡、浮层 |
| `--radius-control` | 10px | 按钮、输入框、树行、代码块、菜单项 |
| `--radius-chip` / `--radius-pill` | 999px | 行内代码、键帽、胶囊、分段控件、发送键、开关 |

圆角具名例外仅：任务清单复选框 3px、过程步骤节点 2px、进行中指示器方块 1px。间距全部落 4px 栅格。

| 令牌 | 值 | 用途 |
| --- | --- | --- |
| `--ease` | `cubic-bezier(0.22,1,0.36,1)` | 颜色 / 透明度等「跟手」过渡 |
| `--ease-spring` | `cubic-bezier(0.34,1.56,0.64,1)` | 按压、开关、展开折叠（caret 旋转）、浮层出现 |
| `--dur` | 150ms | 默认时长 |
| `--dur-spring` | 220ms | 弹性时长 |

任何过渡 ≤240ms；只动 `opacity` / `transform` / `background-position`，不动几何属性。按压物理：按下 60ms，回弹 150ms。

### 3.4 主题机制（FR-THEME）

- 偏好三值 `system` / `light` / `dark` 存应用设置（Go 侧），根元素 `<html data-theme>` 始终带解析后的值。
- 主题切换帧给 `<html>` 加 `.theme-switching`（`*{transition:none!important}`，tokens.css :317），一帧后移除；`color-scheme` 跟 `data-theme` 走。
- 防白闪：`index.html` 引独立 `theme-init.js`（CSP 禁内联脚本）先按 `prefers-color-scheme` 写 `data-theme` + `color-scheme`；Go 建窗前 `WindowOptions.BackgroundColor` = `--raw-canvas`（浅 `#eceef1` / 深 `#0e0f11`，PRD Q6；**不用**旧版 ThemeService 的 `#F6F7F9` / `#0B0F14`——漂移项，以令牌为准）。
- 设置页主题缩略图用 `[data-preview='light'|'dark']`（tokens.css :27,:181）让两张缩略图在任何当前主题下按自己那套令牌上色。
- 深浅两值一一对应：新令牌必须两主题同有值。

## 4. 布局与适配

```text
┌─ 标题栏 40px（画在画布上，无底边线，整条可拖，交互元素不可拖）─┐
│ ╭──────────╮  ╭──────────────────────╮  ╭─────────────────────╮ │
│ │ 侧栏     │⇔│  对话列（永远展开）   │⇔│  文档面板 / 40px 竖轨│ │
│ │ 232px    │  │  弹性列，min 480px   │  │  560px，320–800     │ │
│ │ 192–320  │  │                      │  │  折叠时 40px        │ │
│ ╰──────────╯  ╰──────────────────────╯  ╰─────────────────────╯ │
└──────────────────────────────────────────────────────────────────┘
  ↑ 8px 外边距  ↑ 8px 缝 = 拖拽条       ↑ 8px 缝（折叠时不可拖，占位）
```

| 项 | 值 | 来源 |
| --- | --- | --- |
| 窗口默认 / 最小 | 1440×900 / 744×640 | PRD FR-WIN；最小宽 = 侧栏 192 + 对话 480 + 竖轨 40 + 缝总宽 32（`WORKBENCH_GUTTERS = 4×8`，settings.ts:30） |
| 侧栏宽 | 默认 232，192–320，持久化 | settings.ts:14 / PRD FR-SET |
| 文档面板宽 | 默认 560，320–800，持久化 | settings.ts:18 / PRD FR-SET |
| 文档竖轨宽 | 40px（`--agnes-rail-w`） | tokens.css :113 |
| 对话列最小 | 480px，只有下限 | settings.ts:21 |
| 输入框高 | 默认 96，72–400，持久化 | settings.ts:39 / PRD FR-SET |
| 历史栏宽 | 200px，可开合，收起时列宽 0 且 visibility:hidden | 旧版规范 §八 |

布局规则（旧版规范 §五）：

- 三块 `.workbench-panel`：1px `--raw-line` 边框、16px 圆角、`--shadow-panel`（深 none）、各自 `data-panel` 底色；`overflow:hidden` 既裁圆角也让自动最小尺寸归零。面板间与四周（顶部除外，紧贴标题栏）留 8px 画布缝。
- **缝即拖拽条**：8px 命中区，平时透明；悬停 / 键盘聚焦时 `::after` 显示 2px 强调色竖线（竖线画在 `::after` 上不染本体，tokens.css :1005-1035）；`role="separator"` + `aria-valuenow/min/max`，键盘 `←`/`→` 步进 **8px**（Workbench.tsx:32 `KEYBOARD_STEP=8`；输入框纵向拖拽条 6px 高、键盘步进 16px，UnifiedComposer.tsx:48）。
- 窗口变窄先压文档面板（运行时夹取，不改用户存下的宽度），不压对话列；打开任何文件自动展开文档面板；`Ctrl+\` 开关。
- 折叠时第 5 列仍是 8px 占位缝（不可拖），保持 5 列 grid 结构。
- 标题栏 40px：左「18px 图标 + Mile Knowledge Base + VAULT 小标签（等宽大写）」，中知识库名（截断），右主题分段 + 三个窗口按钮。无底边线（下方面板自带顶边框）。窗口按钮不做位移（贴边位移露画布）；关闭钮悬停 `--fault` 88% 混 `--raw-card`（tokens.css :510-521）。
- **滚动归属**：各栏各自滚动（`.scroll-y`），`body { overflow:hidden }`，整窗无滚动条。面板内滚动的列挂 `.scroll-y`（写在更晚处，`overflow-y:auto` 胜过面板的 `overflow:hidden`）。
- flex 滚动坑（必须记住）：长内容滚不动的根因是 flex item `min-height:auto`，编辑区 `<main>` 必须 `min-height:0`（旧版规范 §五）。面板 `overflow:hidden` 已让自动最小尺寸归零，里层不必重复。
- 覆盖层位置与层级：设置页、技能向导、健康面板等覆盖整个窗口**但留出标题栏**（`top: var(--titlebar-h)`，不透明 `--raw-canvas` 底、`z-index:40`，SettingsView.tsx:326-337）；浮层 popover `z-index:5`（tokens.css :4370-4372）。原生对话框（选目录、关窗询问）用系统对话框，不在这个层级体系内。
- 容器查询断点（不用视口媒体查询，因栏宽由用户拖拽决定）：总览网格与欢迎页容器 `<720px` 退 2 列、`<480px` 退 1 列（`.overview-shell` `container-type: inline-size`，tokens.css :2094-2110、:2291-2310）；设置页 bento `<720px` 退单列（:3974-3988）、供应商/steering 相关 `<560px`（:2893,:3104,:3952）、agent 卡 `<600px`（:2911）、窗口 `<899px` 时 Agnes 面板窄化（:4026）。旧版 tokens.css 中另有 `@media (max-width:899px)`（:4026）——媒体查询仅用于窗口级布局，组件内一律容器查询。
- 长文本：树行、历史行、标签名、路径统一 `text-overflow:ellipsis` 单行截断 + 悬停 title 提示；长代码块 / 宽表格横向滚动不压缩（`.md-table-wrap` `width:max-content;min-width:100%`，溢出右缘发丝线提示）。
- **长列表虚拟化（新增约束，外观与旧版一致）**：单目录 1 万项文件树与 50 轮以上长对话必须虚拟化渲染（PRD 10 节性能、UI设计稿 §6）；只渲染可视行，行高、间距、悬停与选中样式不变，虚拟化本身不产生任何视觉差异。
- 树行 hover 作用域：操作按钮组包在 `.tree-row-scope`（只包本行不包子节点），隐藏时 `pointer-events:none`，悬浮最深子项只亮本行（tokens.css :864-909）。
- 侧栏图标按目录名关键词判定主题（`directoryIcons.ts`），不匹配用普通文件夹图标，不用问号。

## 5. 组件规范

下表为旧版实际用到的共用组件；行末括注为旧版 tokens.css 行号。状态未单列的组件：hover 统一 `background: var(--raw-raised)` 上浮类（`.pressable`），`:active` `scale(0.96) translateY(1px)` 60ms 或 `translateY(1px)`；`:active` 规则必须写在 `:hover` 之后；焦点用 `:focus-visible` 而非 `:focus`。

### 5.1 按钮类

| 组件 | 尺寸 / 结构 | 状态 |
| --- | --- | --- |
| 主按钮 `.btn-primary` | 高按内容（padding 5px 12px），`--raw-ink` 底 + `--text-on-ink` 字，10px 圆角，`--text-body` 600（:524-560） | hover 底向 canvas 混 12%；active 0.96 缩放；disabled 换 `--raw-raised` 底 + 边框，**不压 opacity** |
| 次按钮 `.btn-quiet` | 同尺寸，`--raw-card` 底 + 1px `--raw-line`（:562-589） | 同上 |
| 小按钮 `.btn` | 高 28，padding 0 10，透明底 muted 字，`--text-meta`（:1658-1683） | hover `--raw-raised` 底 + strong 字；`.btn.line` 加 1px 线；`.btn.primary` 墨底变体（:3407）；`.btn.danger-quiet` 悬停 `--fault`（:4351）；`.btn.lg` 加大（:3426） |
| **唯一强调按钮 `.btn-accent`** | `--raw-accent` 底 + `--text-on-accent`，每个视图最多一个（:2451-2486） | hover 向 `--raw-ink` 混 12%；active 0.96；disabled 同 .btn-primary |
| 图标按钮 `.icon-button` | 28×28 网格居中，透明底（:654-677） | hover `--raw-raised`；active 0.96；**必须 `aria-label`**（IconButton 必填 prop） |
| 历史行内小图标钮 `.icon-btn` | 24px，行尾浮出（:4284-4312） | hover 底色；`.danger` hover `--fault`；`aria-pressed` 表达钉选 |
| 窗口按钮 `.window-button` | 标题栏右侧三个 | 只过渡底色，无位移；关闭钮 hover `--fault` 88% 混 card + 白字（:510-521） |
| 发送键 `.composer-send` | 30×30 圆形 999，`--raw-accent` 底（:1095-1126） | hover 向 ink 混 15%；active 0.94；disabled 换 raised 底 faint 字 |

### 5.2 选择类控件

| 组件 | 尺寸 / 结构 | 状态 |
| --- | --- | --- |
| 分段控件 `.segmented` | 槽 `padding:2px` `--raw-raised` 底、999 圆角；项 `padding 4px 10px`、999 圆角、`--text-body-sm` 600（:592-643） | 选中项 `--raw-card` 底 + `--raw-accent` 字（aria-selected 与 aria-checked 同视觉）；`:active` 下沉 1px；`.segmented-fill` 等分铺满 |
| 开关 `.switch` | 34×20，999 圆角，`--raw-raised` 底 + 1px `--raw-line-strong`；滑块 14×14 白底（:3363-3404） | `aria-checked` 时 `--raw-accent` 底 + 滑块 `translateX(14px)`（`--dur-spring`）；disabled `opacity:.5` |
| 复选框 `.field-check` | 13×13，原生 `accent-color: var(--raw-ink)`，不自绘（:2059-2065） | 原生状态 |
| 下拉 `.agent-model-select` / `.field` 系 | 见 5.3 | — |

分段控件是 `radiogroup`+`radio` 或 `tablist`+`tab`，同时只有一个 `aria-checked`/`aria-selected`。

### 5.3 输入类

| 组件 | 尺寸 / 结构 | 状态 |
| --- | --- | --- |
| 输入框 `.field` | `padding ~5.4px 8px`，1px `--raw-line`，`--raw-raised` 底，`--text-body-sm`（:1988-2023） | hover 边向 ink 混 18%；`:focus-visible` 2px `--raw-accent` outline（offset -1）+ 边换 accent；disabled faint 字 |
| 等宽输入 `.field-mono` | 同 `.field`，`--font-mono` 12px | 同上 |
| 文本框 `.field-textarea` | `min-height:4.5rem`，可纵向拉伸，行高 1.6（:2031-2035） | 同上 |
| 对话输入卡 `.composer-card` | `--raw-card` 底 + 1px 线 + 12px 圆角，padding 10/12/9（:1064-1074） | `:focus-within` 边向 `--raw-accent` 混 45%；内部输入框去边（`.composer-input`） |
| 表单行 `.field-row` / `.field-label` / `.field-hint` | 标签上控件下；label `--text-meta` 600 muted；hint `--text-caption` faint（:2038-2056） | — |

### 5.4 列表 / 导航行

| 组件 | 尺寸 / 结构 | 状态 |
| --- | --- | --- |
| 树行 `.tree-row` | `min-height:28`，padding 4/8，10px 圆角，`--text-body`（:707-759） | hover `--raw-raised`；active 下沉 1px；选中（`aria-selected`）`--raw-accent-soft` 底 + 左侧 2px `--raw-accent` 指示条 + 字重 600 |
| 标签页 `.tab-item`（标签 30px 高） | 复用 `.tree-row` 排版 | 选中不填色块，底部 2px `--raw-accent` 下划线；文字 strong 600（:765-776、旧版规范 §九） |
| 菜单项 `.menu-item` | `padding 7px 8px`，10px 圆角（:4438-4486） | hover `--raw-strong` 7% 混透明；选中 `--raw-accent-soft` 底；`menuitemradio`+`aria-checked` |
| 侧栏搜索入口 `.sidebar-search` | 高 28，`--raw-raised` 底 + 1px 线，`.sidebar-kbd` 键帽（16px 高、等宽、`--radius-chip`）（:783-815） | hover 边向 ink 混 18% |
| 设置导航项 `.settings-nav-item` | 与树行选中态一致（:3989-4011） | 选中 `--raw-accent-soft` + 字重 600 |
| 历史行 `.history-item` | 标题 `--text-meta` + 下行 Agent 名与时间 `--text-caption` faint（:4195-4260） | hover / `:focus-within` `--raw-raised`；当前行 `--raw-card` 底 + 1px 内描边 + 标题 600；行尾悬停浮出 24px 图标钮（重命名 / 置顶 / 删除） |
| Agent 卡 `.agent-tab` | 设置页 4 张切换（:2905-3010） | hover 抬升；选中描边 + 强调色细节（`aria-selected`） |
| 分节标题 `.eyebrow` | `--text-meta` 600、`letter-spacing:0.12em`、大写、faint；前缀 6px 方块 + 后缀 1px 发丝线（`.eyebrow-tight` 去后缀线）（:331-360） | 静态 |

### 5.5 卡片与提示

| 组件 | 尺寸 / 结构 | 状态 |
| --- | --- | --- |
| 浮卡 `.surface-card` | `var(--panel-card, var(--raw-card))` 底 + 1px `--raw-line` + 12px 圆角（:368-377） | `data-tone` 状态：`info/ok/warn` 边 `color-mix(tone 40%, line)`、`fault` 45%（:453-464）；`data-interactive` hover 上浮 2px + `--shadow-panel` + 边 `--glow` 55% |
| 工作台面板 `.workbench-panel` | 1px 线 + 16px 圆角 + `--shadow-panel`；`data-panel` 定底色（:388-445） | 静态 |
| 状态卡 `.status-card`（审批 / 失败 / 恢复失败） | `--raw-card` 底 + 1px 线 + 12px 圆角，padding 10/12（:1686-1725） | `data-tone` 边直接染 `var(--warn)`/`var(--fault)`/`var(--info)`（注意：与 `.surface-card` 的 40% 混色不同，以 tokens.css :1697-1705 为准）；失败卡 `role="alert"` 带「重试」 |
| 计划卡 / 报告卡 / 审批卡 | `.surface-card` 或 `.status-card` 承载，12px 圆角；差异预览区用 `--panel-sunken` 凹一档 | 永久删除计划卡加额外警示块与「我了解风险」预确认（截图 29/30/31/42） |
| 欢迎警示条 `.welcome-alert` | 描边条，不套 surface-card（:2339-2356） | `data-tone` warn / fault 染边 |
| 状态胶囊 `.pill` | 高 20，padding 0 8，999 圆角，`--text-caption` 600；底 `color-mix(tone 16%, card)`、字 `color-mix(tone 72%, strong)`；`--tone` ∈ live/info/warn/fault/idle；图标 11px（:1228-1259） | **必须配文字**；只用于小元素（Agent 状态、引擎状态、待审批计数） |
| 状态点 `.status-dot` | 6×6 圆点，`--glow` 底；`data-halo` 加 3px spread 光晕（无 blur）（:680-704） | 「运行中」用语义色光晕 |
| 对话状态胶囊 `.status-pill` | 高由 padding 定，1px `--glow` 35% 边 + 8% 底，`--text-meta` 600（:693-704） | 静态 |
| 恢复提示条 `.restore-note` | 两侧发丝线夹一行 `--text-meta` faint 文案 + 强调色文字钮（:4867-4894） | 按钮 hover 下划线 |
| 磁贴 `.overview-tile` | `.surface-card` 承载，4 列网格、行高 120、`--gap-tile` 间距（:2104-2210） | 容器 <720 退 2 列、<480 退 1 列；跨列卡 `.overview-w2`/`.overview-h2` 同退；每卡必有加载 / 空 / 出错三文案 |
| 引用标签 `.chat-ref` | `--raw-accent-soft` 底 + `--raw-accent` 字，等宽 0.92em（:830-862） | hover 下划线；missing / ambiguous 变体虚线下划线 + 旁注文字（无过渡无位移） |
| 键帽 `.sidebar-kbd` | 高 16，1px 线，等宽 `--text-caption`，999 圆角（:805-815） | 静态 |

### 5.6 浮层与覆盖层

| 组件 | 尺寸 / 结构 | 状态 |
| --- | --- | --- |
| 浮层 `.popover` | `--glass` 底 + `blur(18px) saturate(1.4)` + 1px `--raw-line` + 12px 圆角 + `--shadow-float`，padding 6，min-width 240，z-index 5（:4370-4381） | 出现：`opacity` 走 `--dur`+`--ease`，`translateY(6px) scale(0.97)→none` 走 `--dur-spring`+`--ease-spring`；Esc 关并还焦点 |
| 会话信息浮层 `.session-info` | 宽 340，`top:48px right:12px`，自按钮右下方展开，`role="dialog"`（:4488-4610） | 点外部 / Esc 关闭并还焦点；分节用 `--text-caption` 600 faint 标题 + 1px 发丝线 |
| 路由菜单 `.route-menu` | 宽 280，`bottom:calc(100%+8px)`（:4424-4429） | 同 popover 出现动画 |
| 覆盖层（设置页 / 技能向导 / 健康面板） | `fixed`，`top:var(--titlebar-h)`，不透明 `--raw-canvas` 底，z-index 40；内部两块浮动面板 + 8px 缝 | `role="dialog"`+`aria-modal`（健康面板 `aria-modal="false"`）+`aria-labelledby`；Esc 关；有未保存草稿先确认 |
| 设置内确认条 `.settings-confirm` | 底部停靠（:3214-3243） | 行内确认，非模态 |
| 保存条 `.savebar` | 底部停靠，`--panel-card` 底（:3170-3214） | 草稿超长时给出原因并禁用保存 |

毛玻璃只给浮层；主面板、滚动容器、对话消息一律不用（滚动掉帧）。

### 5.7 对话区组件（旧版规范 §八，tokens.css :1057-1846）

| 组件 | 规格 |
| --- | --- |
| 用户气泡 `.msg-user` | 右对齐，气泡最大宽 72%；`--raw-raised` 底、`--radius-card` 圆角（右上角 `--radius-chip`）、内边距 9/13；标签「你 · 时间」`--text-caption` faint；纯文本不解析 Markdown |
| Agent 消息 `.msg-agent` | 左对齐占满阅读栏，正文缩进 32px（头像 24 + 间距 8）；无底色；24px 头像框（1px `--raw-line-strong`、10px 圆角、`--raw-card` 底）+ 中文名 `--text-body-sm` 600 strong + 英文名 `--text-meta` faint + 时间 + 状态胶囊；Markdown 渲染 |
| 阅读栏 `.conversation-column` | 最大 760px 居中，消息间距 20px，滚动区内边距 16/20/24；总览用 `.conversation-wide` 最大 860px |
| 过程摘要 `.process` | 一轮最多一个，30px 高 `--text-meta` muted，「已完成 N 步 · 思考 X.Xs」+ 右侧「展开过程」；展开后 `.steps` 时间线：1px 竖线、7px 方块节点（圆角 2）、步骤名 600 strong、说明 faint；回合结束自动收起（用户手动展开过的不收） |
| 引用证据 `.evidence` | 「引用 N 个来源」默认收起，展开为 `.chip` 列表（等宽、`--text-caption`、`--raw-sunken` 底，hover 描边 `--raw-accent`） |
| 消息操作 `.msg-actions` | 「复制」「重新回答」`.btn` 28px；回合进行中禁用 |
| 输入卡 `.composer-card` | 见 5.3；顶部 `.composer-route` 路由触发器「发给 X」（`.route-who` strong；`.route-reason` `--text-meta` faint 截断） |
| 运行参数条 `.runtime-controls` | `--panel-card` 底、12px 圆角，输入框正上方同宽（最大 760px）；收起时 30px 摘要行（模型等宽 / 强度 / **执行模式 600 strong** / 右侧本轮用量等宽 caption）；展开为模型下拉 + 强度与执行模式两个胶囊分段 + 一行后果说明；**折叠时卸载内容不 display:none**（隐藏元素仍可 Tab 聚焦），收起态持久化 |
| 用量收据 | 等宽 caption：本轮 / 累计 `N in · N out · N 推理` + `百分比 — 费用`；未知显示「— 未提供」，不显示 0（PRD FR-AI-USAGE） |
| 代码块 `.codeblock` | `--raw-sunken` 底 + 1px 线 + `--radius-control`，右上角 `.tools` 浮层（语言标签 + 「复制」，复制原始 Markdown） |
| 图片标签 `.img-label` | 对话中图片一律不加载（安全决策，旧版规范 §八）：虚线框 + alt 文字；库内 `.vault` 变体可点开 |
| `.stream-pending` | 流式中末尾未完成的表格 / 链接的占位行「正在生成表格…」 |
| `.restore-note` | 见 5.5 |

### 5.8 文档面板

| 组件 | 规格 |
| --- | --- |
| 工具栏 `.doc-toolbar` | 高 38：路径 `--text-meta` muted 截断 + 只读标记（锁图标 + `--text-caption` faint）+ 模式分段控件（阅读 / 源码 / 分栏，胶囊形，选中项 `--raw-card` 底 + 1px 描边 + 600） |
| 标签栏 `.tab` | 高 30；选中文字 strong 600 + 底部 2px `--raw-accent`；脏标记为文件名旁的圆点 |
| 阅读区 `.reader` | padding 24/28/48；`.prose` 最大 70ch 居中（:4614-4821，逐元素规格见下表） |
| 源码视图 `.source-view` / `.source-editor` | `--font-mono` 13px、行高 1.65；**无语法着色**；分栏模式左源码右阅读用同一渲染器 |
| 反向链接 / 来源追踪小节 | `.prose` 同族排版，分节标题 + 列表行（截图 20/21） |
| 大文件 | >1MiB 阅读模式禁用并说明原因；>2MiB 只读（截图 24/25，PRD FR-DOC） |
| 冲突卡 | `.status-card`/`surface-card` 承载，3 个选项（截图 23） |

阅读排版（`.prose`，tokens.css :4620-4821）：正文 `--text-read` 15px / 1.75 `--raw-text`、段距 12px；h1 `--text-display` 650 strong；h2 `--text-title` 650、上 28 下 10 + 底部发丝线；h3 16px 650；列表左 padding 24、项距 3；任务清单复选框 14px、1px `--raw-line-strong`、**3px 圆角（具名例外）**、已完成 `--raw-accent` 填充 + faint 删除线、只读；引用 3px `--raw-line-strong` 左竖条 + muted；行内代码 0.86em 等宽、`--raw-sunken` 底 + 1px 线 + `--radius-chip` + strong 字；代码块 `--raw-sunken` + 1px 线 + `--radius-control` + 13px / 1.6、横向可滚；表格全宽 `--text-body-sm`、单元格 1px 线 + padding 7/10、表头 `--raw-sunken` 600；链接 `--raw-accent` + 1px 下划线（偏移 3px）、hover `--raw-accent-hover`；未解析 wikilink muted + 虚线下划线 + 「（未创建）」faint 后缀、点击不新建；分隔线 1px 发丝线上下 20；图片经 containment 读取、说明文字 `--text-meta` faint 居中。**frontmatter 当正文渲染（D-006 原样复刻）**。

## 6. 反馈规范

| 场景 | 表现 | 规则 |
| --- | --- | --- |
| 加载 / 读取中 | 行内淡色文字（faint/muted），**无转圈**；树与历史等列表就地文字提示 | 异步数据带取消标志或请求序号，切库后旧响应不得覆盖新库 |
| AI 进行中 | `.thinking-row`：三个 5px 方块 `opacity`+2px 起伏、1.2s 循环、相位错开 0.16s；文字扫光 `background-position` 2s 循环（`.thinking-label[data-shimmer]`，带 `color` 与 `-webkit-text-fill-color` 双兜底）（:2530-2604） | **唯一授权的循环动画**：只对 `.thinking-*` 生效；只动 GPU 安全属性；`role="status"`+`aria-live="polite"`；旁必有文字（「正在思考」「等待你确认」）；**等待用户类阶段不动**（`data-animate='false'` 停 `opacity:.65`）；reduced-motion 降级静态（:4823-4864） |
| 重试中 | 行内文字「第 n/m 次重试，x 秒后自动继续」（PRD FR-AI-ENG） | — |
| 空态 | 对话空态 = 总览磁贴（`.overview-*`）；空列表行内文字 + 必要时一个操作；模型服务空态 `.svc-empty` 大空块（:3433-3474） | 每卡必有加载 / 空 / 错三文案；错误只显示脱敏 message |
| 错误 / 警示 / 信息 | 行内卡：`.status-card` / `.surface-card[data-tone]` / `.welcome-alert`，状态色只染 1px 边框 + 图标 + 文字 | **没有全局 Toast**；失败卡 `role="alert"` + 「重试」 |
| 确认 | 行内警示卡为主（变更计划卡、冲突卡、`.settings-confirm` 底部停靠条）；永久删除计划卡额外警示 + 「我了解风险，继续」预确认 | 两步确认 + **600ms 时间门**（`confirmGate.ts`）：删除历史 / 清空全部 / 删除供应商，点一次变「确认删除」`--fault` 600 字，双击不得连过两步；关窗用原生对话框（FR-WIN） |
| 未保存 | 关标签、切文档、离开设置、关窗前先确认（行内卡或原生框） | 设置文本类草稿统一走底部 `.savebar` |
| 成功 | 无全局 Toast；行内 status 卡 / 状态变化表达；设置保存成功提示 **2 秒**自动消失（PRD FR-SET） | 恢复入口在操作日志每条已完成事务的「恢复」 |
| 禁用 | 弱化背景 + 保留边框（`.btn-primary:disabled` 换 `--raw-raised` 底）；**不压整块 opacity**（会把文字拖到 1.26:1） | `cursor:not-allowed`；switch 例外 `opacity:.5` |

## 7. 交互与可访问性

### 7.1 快捷键总表（旧版实现 + PRD 汇总）

| 键 | 作用 | 来源 |
| --- | --- | --- |
| Ctrl+P | 打开搜索面板（文档面板顶部，最大高 420） | Workbench.tsx:117 / PRD FR-SRCH |
| Ctrl+\ | 开关文档面板 | Workbench.tsx:112 |
| Ctrl+S | 编辑器保存 / 设置页草稿保存 | PRD FR-DOC、FR-SET |
| Ctrl+F | 编辑器内查找（面板在顶部） | PRD FR-DOC |
| Ctrl / Cmd + 点击 wikilink | 跳转或新建 | PRD FR-DOC |
| Tab（编辑器内） | 缩进 | PRD FR-DOC |
| Enter | 对话输入发送（Shift+Enter 换行）；搜索结果打开 | UnifiedComposer.tsx:386；SearchPanel.tsx:122 |
| ↑ / ↓ | 搜索结果选择；设置页分节切换 | SearchPanel.tsx:116；PRD FR-SET |
| ← / → | 栏宽拖拽条步进 8px；输入框高拖拽条步进 16px | Workbench.tsx:32；UnifiedComposer.tsx:48 |
| Esc | 关闭搜索面板 / 浮层 / 设置页 / 向导 / 健康面板 / 编辑中的改派或输入撤销 | SearchPanel:113、SettingsView:285,302、SkillImportWizard:61、HealthPanel:55、ModelSection:597 等 |
| Enter / Esc（内联改名 / 新建） | 确认 / 取消 | InlineRename:65、InlineCreate:54、InlineAliasEdit:67 |

### 7.2 可访问性清单（旧版规范 §十 + PRD 10.3，逐项可验）

- 图标按钮必须有 `aria-label`（tooltip 对键盘与读屏不存在）。
- 表单控件有可访问名（`<label>` 包裹或 `aria-label`）。
- 分段控件是 `radiogroup`+`radio`（或 `tablist`+`tab`），同时只有一个 `aria-checked`/`aria-selected`。
- 模态是 `role="dialog"` + `aria-modal` + `aria-labelledby`，Esc 可关；非模态面板（健康面板）`aria-modal="false"`。
- 进行中指示器 `role="status"` + `aria-live="polite"`。
- 装饰性图片 `alt=""` + `aria-hidden`。
- 焦点：键盘焦点环可见（`:focus-visible`，2px `--raw-accent`）；浮层 / 覆盖层关闭后焦点归还触发按钮；拖拽条焦点由 2px 竖线表达（关 outline）；`.resizer` 带 `role="separator"` + `aria-orientation` + `aria-valuenow/min/max`。
- 折叠区必须**卸载**而非 `display:none`（隐藏控件仍可 Tab 聚焦是缺陷）。
- 颜色不能是唯一信息载体：状态必配文字或图标；用户 / Agent 角色靠对齐 + 文字标签 + 表面三重区分；进行中动画旁必有文字。
- 对比度：正文 ≥4.5:1；非文字 UI（图标、控件轮廓、指示条）≥3:1；faint/muted 在 canvas、card、三块面板底色上的组合都按 4.5:1 算；强调色文字在卡片 / 画布 / 抬升面 / 淡底上 ≥4.5:1；强调色主按钮前景 `--text-on-accent` ≥4.5:1。改任何文字或交互色必须重算（脚本见 `.claude/skills/design-tokens/SKILL.md` §6）。
- `prefers-reduced-motion`：循环动画停（`.thinking-*` 停 `opacity:.65` 并归还文字颜色）；浮层 / 弹性过渡归零（tokens.css 末尾两个覆盖块 :4823、:4901，`:not()` 与外面对齐）。
- 1280 / 1440 / 1920 三宽度下无横向溢出。
- 主题切换一帧无过渡（`.theme-switching`），无白闪（§3.4）。

## 8. 实现边界

- 组件里**不写** hex/rgb/hsl 与令牌外数值；只用 `var(--…)` 或 `.text-*` 类；深浅两值一一对应。改令牌流程：先改本文件两列值（浅 / 深）→ 跑对比度 → 同步 `src/styles/` CSS → 改 `--raw-canvas` 时同步 `WindowOptions.BackgroundColor`。
- 状态色只染 1px `border-color`（`color-mix(in srgb, var(--tone) 40%, var(--color-line))`，fault 45%），卡片 / 面板 / 整行背景不染状态色；唯一例外 `.pill`（16% 底 / 72% 字，必须配文字，图标 11px）。`.status-card` 的 tone 边是直染 `var(--warn/fault/info)`（tokens.css :1697-1705，与 `.surface-card` 的 40% 混色不同，两者都以代码为准）。
- 强调色只表达交互，永不表达状态；每个视图最多一个强调色主操作。
- 图标一律 `@phosphor-icons/react` **深导入**（`import { XxxIcon } from '@phosphor-icons/react/X'`），集中出口 `src/lib/icons.ts`（不用桶文件）；三档字重：导航选中 `fill`、未选中 `regular`、强调操作 `bold`；实测常用尺寸 11–16px（胶囊 11、行内 13–14、头像框外 15–16）；不用 emoji；目录图标按关键词判定、缺省普通文件夹。
- 等宽字体栈只在 `src/styles/` 定义一处，组件一律 `var(--font-mono)`，不另写字体名。
- **Markdown 渲染由 Go 输出 HTML**（D-004：goldmark + bluemonday）：Go 端输出带 data 属性的 HTML，前端只做事件委托与样式；前端类名必须与旧版 `.prose` / `.reader` / `.answer` / `.codeblock` / `.md-table-wrap` / `.img-label` / `.chat-ref` 等一致，保证 tokens.css 移植后原样生效。文档内不执行 HTML / 脚本；对话中图片不加载、渲染成 `.img-label`。
- 设计缺失的场景：先按 §2 允许的手段表提建议并获用户确认，不静默发明新视觉手段；新组件优先复用本节原语组合。
- 不用 `@layer` 包裹组件原语（Tailwind 4 层级顺序会让工具类反覆盖组件类，tokens.css :4-13 的五条手写纪律沿用：transform 过渡列颜色属性、`:active` 在 `:hover` 后、reduced-motion 块放最末且 `:not()` 对齐、`color-mix` 写 `in srgb`、`--glow` 消费者给兜底）。
- Tailwind 4：`main.css` 只 `@import 'tailwindcss'` + `./tokens.css`；`dark:` 已由 `@custom-variant` 绑到 `data-theme`，不用 `@media (prefers-color-scheme)` 写颜色。

## 9. 视觉验收

方法：同样本知识库、同窗口尺寸、同主题下，新版窗口与 `design/baseline/shots/` 对应截图并排对比；G2 要求布局结构、栏宽、令牌、交互状态一致，**像素级差异只允许来自字体渲染**（WebView2 与 Electron 的字体光栅化差异）。

代表性对照清单（编号 = manifest.json id）：

| 检查项 | 截图 |
| --- | --- |
| 欢迎页浅 / 深、已断开故障卡、引导两步 | 00、03、01、02 |
| 工作台默认（文档折叠 / 展开）、最小窗口 744×640、深色 | 12、10、56、57、11、17、39、48、50 |
| 侧栏与文件树（展开、hover 操作、内联改名 / 新建） | 13、14、15、15b |
| 文档阅读 / 源码 / 分栏 / 标签脏标记 / 冲突卡 / 大文件 / 反链 / 来源 | 16、17、18、19、22、23、24、25、20、21 |
| 搜索（结果 / 空 / 筛选） | 26、27、28 |
| 变更计划卡、报告卡、永久删除卡、操作日志、健康面板、总览 | 29、30、31、32、33、10、11 |
| 对话（路由预览 / 改派 / 流式 / 完成 / 审批卡 / 用量 / 参数条 / 会话信息 / 历史栏 / 无供应商） | 34、35、37、38、39、42、43、36、44、45、46、40 |
| 设置五节 + 保存条 + 导入向导 | 47、48、49、50、51、52、53、54、55 |
| 原生对话框（关窗询问、选目录） | 58、59（无法截取，按 FR-WIN 文字验收） |

逐版检查：布局栅格与栏宽、面板分色与缝、选中 / 悬停 / 焦点 / 按下 / 禁用各态、深浅两主题、状态色只染边、胶囊染底、对比度、Esc / 焦点归还、reduced-motion 降级、长列表虚拟化后的视觉一致、虚拟化与容器查询断点下无横向溢出。允许差异：仅字体渲染；有意差异：D-006 欢迎页（全断开时新版显示欢迎页而非工作台空壳）。截图不能替代真实业务链路验证，视觉验收用 AC-* 与真实库数据做终验。

## 10. 未决问题与完成条件

| 问题 | 建议或选项 | 影响 | 是否阻断实现 | 需要谁确认 |
| --- | --- | --- | --- | --- |
| 无 | — | — | — | — |

完成条件：本文件 9 项齐全，语义令牌两主题数值与旧版 tokens.css 一致（漂移项已标注），关键状态与适配规则可执行可验收；阶段 10 建立的 `src/styles/` 与本文件数值冲突时先澄清再改。


## 11. R2 增补：技能浮层

> 状态：用户 2026-11-20 审核通过（含毛玻璃修订）。页面结构与状态见 docs/UI设计稿.md 第 9 节，视觉稿 `design/r2/skills-panel.html`。本节只记新增的共用规则；能用已有原语的一律复用，下表列出的是新增组合。

### 11.1 复用与新增

| 部分 | 做法 |
| --- | --- |
| 标题栏「技能」按钮 | 新增：胶囊形，高 28，`--raw-raised` 底、`--text-body-sm` 600；打开时 `--raw-accent-soft` 底 + `--raw-accent` 字（`aria-expanded`）；角标 18px 胶囊，配色同 `.pill[data-tone='warn']`。必须带 `.no-drag` |
| 浮层外壳 | 新增，**毛玻璃**（用户 2026-11-20 要求）：底色 `--raw-card` 62% 混透明，`backdrop-filter: blur(24px) saturate(1.5)`，1px `--raw-line` 75% 混透明、`--radius-panel`（16）、`--shadow-float` 加一道 `--glass-line` 顶部内高光，z-index 5。模糊只挂在**不滚动的外壳**上，滚动的是里面的主体，所以不违反 §5.6「滚动容器不用 blur」。浏览器不支持 `backdrop-filter` 时退回不透明 `--raw-card`。与现有 `.popover` 一样不读系统的「减少透明度」设置（本机就关着透明效果，读了用户就看不到）。结构是顶栏 / 滚动主体 / 底栏，顶栏与底栏 1px 发丝线（`--raw-line` 75% 混透明） |
| 浮层内表面 | 卡片、工具行悬停、目标格、chip、选择区用局部变量 `--skm-surface`：浅色 `--raw-card` 70% 混透明，深色 `--raw-raised` 66% 混透明，让玻璃层次透出来；输入框、确认卡、确认卡内的条目保持不透明，保证读写区域稳定 |
| 两页切换、显示方式、分发方式 | 复用 `.segmented`（分发方式用 `role="radiogroup"` + `aria-checked`） |
| 技能卡片 | 新增 `.skm-card`：`--raw-sunken` 底、1px 线、`--radius-card`；悬停只换边框色到 `--raw-line-strong`，不抬起、不位移；选中是强调色边框 + 混 `--raw-accent-soft` 的底；「知识库有更新」染 warn 边（§8 状态色只染边） |
| 工具筛选 chip | 新增 `.skm-chip`：高 28 胶囊，不用 `.chip`（那是等宽小字的证据 chip）；按下态 `aria-pressed`，强调色边 + `--raw-accent-soft` 底 |
| 工具技能行 | 新增 `.skm-row`：高 40，5 列（图标 20 / 名称 / 路径 / 状态 150 / 操作 88）；行间发丝线，悬停行 `--raw-sunken` 底；操作按钮复用 `.icon-button`，平时隐藏、悬停或键盘聚焦本行时显示 |
| 来源状态 | 复用 `.pill`：链接到知识库 = live；拷贝且知识库有更新 = warn；链接失效 = fault；链接到外部、独立、内置 = idle（内置带锁图标）。胶囊必须有文字，颜色不是唯一线索 |
| 分发目标格 | 新增 `.skm-target`：`role="checkbox"`，勾选态强调色边 + `--raw-accent-soft` 底 + 右上角实心勾；禁用态 `--raw-sunken` 底、`aria-disabled`，说明文字写原因 |
| 提示 | 警示 / 信息用行内卡片（warn / info 混色边），不用 Toast，与 §6 一致 |
| 卸载确认 | 浮层内二级层：浮层主体上盖 40% `--raw-canvas` 遮罩加 `blur(6px)`，居中 540 宽不透明确认卡；危险按钮新增 `.btn-danger`（fault 45% 混线的边、10% 混卡片底、fault 字），按钮文字写数量 |
| 主操作 | 复用 `.btn-primary`（每个视图最多一个），次要用 `.btn-quiet`，行内小按钮新增 `.btn-xs`（高 24） |

### 11.2 品牌图标

- 工具图标是各厂商的品牌 SVG，来自 skillkit（MIT），放在 `design/r2/agents/`，实施时移到 `src/assets/agents/`，同目录附 NOTICE（商标归各自所有者，D-014 第 4 条）。这是 §8「图标一律 Phosphor」的唯一例外，只用在表示「某个工具」的位置；其余图标（搜索、关闭、卸载、文件夹等）照旧用 Phosphor（原型里手绘的线条图标只是占位，实施时换成 Phosphor 对应项：Stack、MagnifyingGlass、SquaresFour、ListBullets、Plus、X、DotsThree、ArrowRight、Folder、Link、Trash、Tray、Check、Warning、Info、Globe、CaretLeft、Lock、ArrowCounterClockwise、GithubLogo、FileZip）。
- 品牌图标只按 16 / 18 / 20px 显示，圆角 4px，不重新着色。深色主题下给 `img` 垫一块 `#e6e7ea` 浅底加 1px 内边距：Cursor、Grok、OpenCode 等是黑色单色图标，直接放在深底上看不见。这个浅底是例外，只用于第三方图标。
- 没有图标的工具（MiniMax Code）用首字母占位：`--raw-raised` 底、`--raw-muted` 字。通用目录用 Phosphor Globe。

### 11.3 交互与可访问性

- 按钮 `aria-haspopup="dialog"` + `aria-expanded` + `aria-controls`；浮层 `role="dialog"` + `aria-label="技能"`，打开时焦点进入顶栏第一个控件，Tab 在浮层内循环，关闭后焦点回到按钮。
- 卸载确认卡 `role="alertdialog"`，打开时焦点在「取消」上（危险操作不预选危险按钮）。
- 两页切换支持 `←`/`→`；工具技能行的操作在键盘聚焦本行时可见，不能只靠悬停。
- 角标有 `aria-label`（「N 项需要处理」）；状态胶囊有文字；分发目标格的状态写在格内说明文字里。
- `prefers-reduced-motion` 时浮层展开不做位移与缩放，只做透明度。
- 玻璃上的文字对比度（2026-11-20 按截图实测）：最弱的 `--raw-faint` 在 5 个视图 × 浅 / 深下最低 5.12:1，`--raw-muted` 最低 5.90:1，都高于 4.5:1。原型背后只有总览磁贴和正文；背后是深色大图之类的内容时没测过，实施时用真实工作台截图复测。
- 浮层展开动效复用 §5.6 popover 的出现动画（透明度 `--dur`+`--ease`，`translateY(6px) scale(0.97)` → 无，走 `--dur-spring`+`--ease-spring`）。

### 11.4 视觉验收

R2 的视觉验收以 `design/r2/shots/` 的 18 张截图为基准，方法同 §9；允许差异：字体渲染、真实数据造成的数量与文字差异、原型中占位图标换成 Phosphor 后的形状差异。


### 11.5 M5 增补：安装来源与更新（2026-11-21 审核通过）

页面结构与状态见 docs/UI设计稿.md 9.4。新增的组合都在浮层内，沿用 11.1 的表面与线条规则：

| 部分 | 做法 |
| --- | --- |
| 地址输入行 | 新增 `.skm-urlrow`：输入框（等宽字、`--text-meta`）+ 次要按钮「读取」，两列网格，间距 8 |
| 来源条 | 新增 `.skm-origin`：`--skm-surface` 底、1px 线、`--radius-card`；GitHub 图标 + 粗体仓库名 + 分支 + 等宽短 SHA（`--raw-faint`）+ 说明 |
| 候选列表 | 新增 `.skm-cand`：4 列（复选框 18 / 名称 150 / 路径与描述 / 大小或原因）；行高 ≥ 40；行间发丝线；整行 `role="checkbox"`；禁用行名称降为 `--raw-muted`，右侧用 `.pill` 写原因（已存在 = idle，过大 = warn） |
| 复选框 | 复用 11.1 的 `.skm-box`；未选是卡片底 + `--raw-line-strong` 边，禁用是 `--raw-sunken` 底 |
| 进度条 | 新增 `.skm-progress`：高 4 胶囊，轨道 `--raw-sunken`，填充 `--raw-accent-btn`；`prefers-reduced-motion` 时不做宽度过渡 |
| 错误卡 | `.skm-note` 增加 `data-tone="fault"`：fault 40% 混线的边、6% 混卡片底、fault 图标；正文第一句加粗写结论 |
| 文件差异 | 新增 `.skm-diff`：3 列（变动胶囊 52 / 等宽路径 / 大小）；修改 = idle，新增 = live，删除 = fault |
| GitHub 状态胶囊 | 复用 `.pill`，带 Phosphor GithubLogo：有更新、两边都改了 = warn；最新、本地改过 = idle；检查失败 = fault。卡片染边规则同「知识库有更新」 |
| 卡片来源 | `.skm-card .src`：`--text-caption`、`--raw-faint`，GitHub 图标 + `owner/repo`，只在没有状态胶囊时显示 |

对比度：新增文字都使用 11.3 已实测的 `--raw-faint` / `--raw-muted` / 胶囊配色，没有新的前景色；实施后按 11.4 用真实截图复测。

### 11.6 M6 增补：管理目录与项目分组（2026-11-22 审核通过）

页面结构与状态见 docs/UI设计稿.md 9.5。全部在技能浮层内，沿用 11.1 的表面与线条规则：

| 部分 | 做法 |
| --- | --- |
| 管理目录子视图头 | 同 11.1 的 `.skm-sub`：返回图标 + 「工具技能 / 管理目录」面包屑；右侧「新增自定义工具」「登记项目」用 `.btn-quiet`（次要按钮，不是主按钮），再加关闭图标 |
| 登记行 | 新增 `.skm-tgt-row`：3 列（类型徽标 56 / 名称 + 脱敏路径 / 统计 + 操作），行高 ≥ 44，行间发丝线 `--skm-line`；悬停显示编辑 / 移除图标（`.icon-button`，26×26，同现有行）；项目行的「N 个子目录」用 `--text-caption`、`--raw-faint` |
| 类型徽标 | 复用 `.pill`：自定义 = `data-tone="idle"`，项目 = `data-tone="info"`；文字「自定义」「项目」，不另造颜色 |
| 脱敏路径 | 新增 `.skm-path`：等宽字、`--text-caption`、`--raw-faint`、单行省略；平台前缀替换成 `~`，用户目录外的路径写成 `<盘符>:\…\<最后 2 段>`（API §14.1） |
| 目录不存在 | 行内文字用 warn 文字的既有口径（`color-mix(warn 70%, --raw-strong)`），另加 11.1 的 `.pill[data-tone="warn"]`「目录不存在」；不给整行染边（这一行不是错误，是事实提示） |
| 表单 | 名称与「选择文件夹」都用设置页的 `.field`（1.3.2 的教训，不再用未定义的类名）；字段标签用 `.skm-label`；字段下面一行说明用 `--text-caption`、`--raw-faint` |
| 校验错误 | 新增 `.skm-errline`：`--text-caption`、fault 文字色，位于字段下方 4px；同一表单只显示第一条；不弹 toast |
| 项目子目录 chips | 复用 `.skm-chip` 的静态形态（`<span>` 不是按钮）：图标用 `folder`、后面跟 `n` 计数；只列出实际存在的约定子目录 |
| 安装子目录单选 | 复用 `.segmented` 的单选形态；超过 3 个候选时换成纵向单选列表（每行 32 高、点号 + 名称 + 计数），避免横向挤压 |
| 移除确认 | 复用 `.skm-confirm` 与 `.btn-danger`；正文第二句加粗写「目录里的文件不会删除」，字号 `--text-body-sm`；按钮「移除登记」 |
| 工具页项目分组 | 分组标题 `.skm-group` 写「项目 · <名称>」+ 技能数；行首图标换成 `sourceToolId` 的工具图标（16×16，title 写「Claude Code · .claude/skills」）；行结构与现有 `.skm-row` 完全一致，不加新列 |
| 分发 / 安装目标格 | 复用 `.skm-target`：项目格第二行写「写入 .claude/skills · N 个技能」；目录不存在时 `aria-disabled` 并写「目录不存在」；自定义格第二行写脱敏路径的最后一段 |

对比度：没有新的前景色，全部复用 11.3 已实测的 `--raw-faint` / `--raw-muted` / fault / 胶囊配色；T10 真实截图复测（2026-11-22）：路径 6.22、统计文字 8.09 / 7.17、缺失胶囊 7.47 / 7.77、表单错误 6.47 / 6.03（浅 / 深），全部 ≥ 4.5。

### 11.7 M7 增补：还原横幅与还原预览（草稿，随 PLAN-M7 审核）

页面结构与状态见 docs/UI设计稿.md 9.6。全部在技能浮层的「工具技能」页内；类名用真实实现的前缀（M6 实现时 `.skm-*` 原型名已落为 `.targets-*` / `.tool-*` / `.skills-*`，本节直接用实现口径）：

| 部分 | 做法 |
| --- | --- |
| 待还原横幅 | 新增 `.restore-banner`：一行，高 ≥ 40，左侧 16×16 信息图标，文字 `--text-body-sm`、`--raw-strong`；底色与描边用 `.pill[data-tone="info"]` 那套 info 浅底，不用 `--raw-accent-soft`（那是选中行的底色，会混淆）；右侧按钮 `.btn-quiet`「查看并还原」 |
| 清单无法读取横幅 | 同一结构，`data-tone="warn"`：warn 浅底 + warn 文字口径（`color-mix(warn 70%, --raw-strong)`，同 11.6「目录不存在」）；按钮「用本机记录重建」仍是 `.btn-quiet`，不是危险按钮（重建保留原文件副本） |
| 还原子视图 | 复用 `.skills-subview` / `.skills-subview-head` / `.skills-subview-foot`；面包屑「工具技能 / 从清单还原」 |
| 分组 | 复用 `.skills-group-title`：「新建 3」「冲突 1」「已一致 5」「跳过 2」；组顺序固定，空组不显示 |
| 还原行 | 新增 `.restore-row`：4 列（勾选 28 / 名称 + 目标 / 来源说明 / 原因），行高 ≥ 44，发丝线分隔，同 `.tool-row`；不可勾选的行勾选框位置留空，不放禁用勾选框；目标图标复用 `.tool-icon`（项目用 `sourceToolId` 的图标，自定义用 `.tool-icon-folder`） |
| 「需要联网」 | `.pill[data-tone="info"]` 小胶囊，放在来源说明后面；不用颜色单独表达，文字始终在 |
| 冲突说明 | 冲突组标题下一行 `--text-caption`、`--raw-muted`：「勾选的项会先备份原目录再覆盖」 |
| 原因文字 | `--text-caption`、`--raw-faint`，单行省略，`title` 放全文 |
| 本机有、清单没有 | 可折叠段落（`<details>`），标题 `--text-caption`、`--raw-muted`；内容只读列表，不带勾选 |
| 进度 | 复用 M5 的 `.install-progress` / `.install-progress-line` |
| 结果 | 复用 `.skills-result`；「未匹配」一行用 `--raw-muted`，不用 fault 色（不算失败）；失败项原因用 fault 文字色 |
| 重建确认 | 复用现有确认卡与 `.btn-accent`「重建清单」（不是 `.btn-danger`：不会丢失数据） |

对比度：没有新的前景色，info / warn 胶囊与 `--raw-faint` / `--raw-muted` 都沿用 11.3、11.6 已实测的配色；T00 原型截图与实现后真实截图各复测一次，门槛同 12.3。

T08 真实截图复测（2026-11-23，`capture-m7.py`）：横幅 15.75 / 13.47、目标行 8.09 / 7.17、原因与来源 6.22 / 6.22、info 胶囊 7.97 / 6.08、warn 提示 16.1 / 13.16（浅 / 深），最低 6.08，全部高于门槛。实现时修正一处：还原子视图最初误用模态外壳 `.skills-subview`，改为与 `.targets-view` 相同的整页结构（头 / 可滚主体 / 固定底栏）。

## 12. R2 增补：主题色

> 状态：用户 2026-11-20 审核通过。需求见 PRD 第 16 节（FR-ACCENT），入口见 docs/UI设计稿.md 第 10 节。本节修改 §3.1 的强调色定义：强调色从「单一靛蓝」变为「可选，默认橙色」，其余原则不变。

### 12.1 令牌模型变化

| 令牌 | 现在 | 改为 |
| --- | --- | --- |
| `--raw-accent` | 填充与文字共用 | 只用于**不带文字的强调色 UI**：选中描边、焦点环、拖拽条竖线、开关轨道、选中树行左侧竖条、标签页未保存圆点、输入框焦点描边 |
| `--raw-accent-btn` / `-btn-hover` / `-btn-active`（新增） | — | **带文字或图标的强调色底**：`.btn-accent` 主按钮、`.composer-send` 发送按钮、复选框选中底、色板色块。浅色下比 `--raw-accent` 深，好让白字达标；深色下与 `--raw-accent` 一族相同（用户 2026-11-20 要求按钮用白字，D-015 第 6 条） |
| `--raw-accent-text`（新增） | — | 强调色**文字**：选中的分段项、链接、文字按钮、选中行文字。29 处 `color:` 用法改用它 |
| `--raw-accent-hover` / `-active` | 悬停 / 按下 | 只给不带文字的强调色元素用；按钮的悬停与按下改用 `--raw-accent-btn-hover` / `-active` |
| `--raw-accent-soft` | 选中行底 | 不变 |
| `--raw-on-accent` | 浅白深近黑 | 仍是浅色白字、深色近黑字，6 个预设都一样；它只搭配 `--raw-accent-btn` 一族使用。`.composer-send` 现在写的是 `color: var(--raw-card)`，改成它 |
| `--focus` | 等于强调色 | 跟随 `--raw-accent` |
| `--info` | 靛蓝 | **不变**，它是状态色 |

实现：每套配色写成 `[data-accent='orange']`（浅）与 `[data-theme='dark'][data-accent='orange']`（深）两段，只覆盖上表 10 个令牌；默认值只写在 `:root` 与 `[data-theme='dark']` 上，`[data-preview]` 的主题缩略图不覆盖强调色，跟随当前选择；自定义色由前端算好后写到 `<html>` 的内联样式变量上。`:root` 的默认值改为橙色。

### 12.2 不随主题色变的

画布、面板、卡片、中性文字、状态色、阴影、圆角、字号、毛玻璃、应用与工具图标（PRD 16.3）。所以对比度只需要检查强调色一族与这些固定底色之间的组合。

### 12.3 预设配色（2026-11-20 计算并验证）

验证标准沿用 §7.2：强调色文字在卡片、画布、抬升面、淡底上 ≥4.5:1；`--raw-on-accent` 在 btn、btn-hover、btn-active 三态上都 ≥4.5:1；accent 与卡片 ≥3:1（开关滑块是卡片色，也由这一条保证）。12 套全部通过；「靛」两套与现在的令牌逐值相同。不带文字元素的悬停与按下（`--raw-accent-hover` / `-active`）只有非文字要求，数值在 `src/lib/accent.ts` 的测试快照里，不在表中重复。

| 预设（基色） | 主题 | accent | soft | accent-text | btn | btn-hover | btn-active | on-accent |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 橙（默认） `#fa7b29` | 浅 | `#e87226` | `#fceee5` | `#aa541c` | `#ba5b1e` | `#a7521b` | `#954918` | `#ffffff` |
| 橙（默认） `#fa7b29` | 深 | `#fa8234` | `#4d3426` | `#fa8234` | `#fa8234` | `#fb914c` | `#dc722e` | `#111214` |
| 靛 `#4f46e5` | 浅 | `#4f46e5` | `#ebeafd` | `#4f46e5` | `#4f46e5` | `#4338ca` | `#3730a3` | `#ffffff` |
| 靛 `#4f46e5` | 深 | `#8b8ff8` | `#2a2b45` | `#8b8ff8` | `#8b8ff8` | `#a2a5fa` | `#7579ec` | `#111214` |
| 蓝 `#2563eb` | 浅 | `#2563eb` | `#e5ecfd` | `#2461e6` | `#2563eb` | `#2159d4` | `#1e4fbc` | `#ffffff` |
| 蓝 `#2563eb` | 深 | `#81a5f3` | `#323c50` | `#81a5f3` | `#81a5f3` | `#90b0f4` | `#7291d6` | `#111214` |
| 绿 `#16a34a` | 浅 | `#16a34a` | `#e3f4e9` | `#117c38` | `#12873d` | `#107a37` | `#0e6c31` | `#ffffff` |
| 绿 `#16a34a` | 深 | `#4cb874` | `#274034` | `#4cb874` | `#4cb874` | `#61c185` | `#43a266` | `#111214` |
| 青 `#0d9488` | 浅 | `#0d9488` | `#e2f2f1` | `#0b7970` | `#0c857a` | `#0b786e` | `#0a6a62` | `#ffffff` |
| 青 `#0d9488` | 深 | `#56b4ac` | `#293f40` | `#56b4ac` | `#56b4ac` | `#6abdb6` | `#4c9e97` | `#111214` |
| 紫 `#7c3aed` | 浅 | `#7c3aed` | `#efe7fd` | `#7c3aed` | `#7c3aed` | `#7034d5` | `#632ebe` | `#ffffff` |
| 紫 `#7c3aed` | 深 | `#b793f5` | `#3e3850` | `#b793f5` | `#b793f5` | `#c0a0f6` | `#a181d8` | `#111214` |

说明：
- 橙色在浅色下：基色 `#fa7b29` 上放白字只有 2.65:1。所以开关、焦点环等不带文字的元素用亮橙 `#e87226`（与白卡 3.06:1），按钮底从 accent 再加深到 `#ba5b1e`（白字 4.56:1），强调色文字用 `#aa541c`。绿、青同理，按钮分别是 `#12873d`、`#0c857a`。
- 浅色下 6 个预设的按钮都是白字，悬停与按下一律往深处走。
- 深色下各预设都提亮到与深色面板 ≥4.5:1，按钮用近黑字（用户 2026-11-20 选择保持现状）：深色画布上亮按钮更醒目；若改成加深底加白字，靛、紫的按钮与深色卡片不到 3:1。
- 对比图：`design/r2/shots-accent/onaccent-compare.png`（现状深色字 / 加深加白字 / 亮色加白字三列）。

### 12.4 自定义色的生成规则

输入 `#RRGGBB`（基色 b），浅色与深色各算一套。函数放在 `src/lib/accent.ts`，单元测试覆盖边界色（纯黄、纯黑、纯白、灰）：

- 浅色：
  - accent = b；若与白卡 <3:1，向黑色混合，直到 ≥3:1；
  - btn = accent 继续向黑色混合，直到白字在上面 ≥4.5:1；btn-hover / btn-active = btn 混 10% / 20% 黑；on-accent = 白字；
  - hover / active（不带文字元素用）= accent 混 10% / 20% 黑；
  - soft = accent 12% 混白卡；
  - accent-text = b 向黑色混合，直到在卡片、画布、抬升面、soft 上都 ≥4.5:1。
- 深色：
  - accent = accent-text = b 向白色混合，直到在深色卡片、画布、抬升面以及自己的 soft 上都 ≥4.5:1；
  - btn 一族 = accent 一族；on-accent = 近黑字（若 ≥4.5:1）否则白字；
  - hover = 混 12% 白，active = 混 12% 黑；soft = accent 22% 混深色卡片。
- 结果与输入不同时，设置页与色板显示「实际使用」的颜色。接近灰色的输入（饱和度很低）照样可用，但提示「颜色太接近灰色，选中态可能不明显」。

上面的步骤与生成 12.3 的脚本是同一套；实施时把脚本移成测试，断言 12 套预设的数值不漂移。

### 12.5 撞色

橙色与 warn 色相只差 1°，绿色与 live 相同，靛、蓝、紫与 info 接近。处理：状态色不改，状态一律用带文字的胶囊或带文字的卡片表达，不能只靠颜色（§7.2 已有要求）。默认橙色下，主按钮与警示边框会更像，这是选橙色作默认的已知代价（D-015）。

### 12.6 视觉验收

R1 的 53 张基线对照改在「靛」预设下做（数值未变）；R2 技能浮层的 18 张截图在实施时按默认橙色重拍一次。主题色本身的验收：6 个预设 × 浅 / 深，各截标题栏色板打开态与一个含主按钮、分段控件、链接、选中行的页面。

## 13. R2 增补：技能页（独立页面与材质试点）

> 状态：用户 2026-11-24 审核通过；随 M8 实施。需求见 PRD 第 17 节，页面结构见 docs/UI设计稿.md 第 11 节，原型 `design/r2/skills-page.html`。审核通过后，本节是 §2 禁止项在技能页内的具名例外；第 11 节（技能浮层）中关于外壳与尺寸的规则由本节替代，组件规则继续有效。

### 13.1 试点范围

- 所有新增手段只在技能页根元素 `.skills-page` 之内生效；令牌也只在这个范围里定义，不进 `:root`。
- 不在范围内的界面（工作台、设置页、对话、文档、浮层）不允许引用本节令牌。推广到其他界面要另提议并经用户确认。

### 13.2 新增令牌（只在 `.skills-page` 内）

| 令牌 | 浅色 | 深色 | 用途 |
| --- | --- | --- | --- |
| `--glass-card` | `--raw-card` 50% 混透明 | `--raw-raised` 46% 混透明 | 卡片、列表、方式卡、目标格的底 |
| `--glass-card-hover` | `--raw-card` 66% | `--raw-raised` 62% | 卡片悬停的底 |
| `--glass-strong` | `--raw-card` 78% | `--raw-raised` 80% | 横幅、底部操作条、确认卡、返回按钮 |
| `--glass-edge` | `rgb(255 255 255 / .7)` | `rgb(255 255 255 / .09)` | 顶部 1px 内高光（`inset 0 1px 0`） |
| `--glass-blur` | 18px | 18px | 卡片模糊半径；底部操作条 22px，确认卡 24px |
| `--field-a` / `-b` / `-c` | 强调色 36%、`--info` 24%、`--live` 16% | 22%、20%、10% | 背景色场三团的颜色 |
| `--ease-out-expo` | `cubic-bezier(0.16, 1, 0.3, 1)` | 同左 | 入场：快起慢收 |
| `--ease-spring-soft` | `cubic-bezier(0.34, 1.36, 0.64, 1)` | 同左 | 悬停、滑块、按钮内图标：轻微过冲 |
| `--dur-enter` / `--dur-slide` / `--dur-hover` | 520 / 460 / 320ms | 同左 | 入场 / 滑块与图标 / 悬停 |
| `--stagger` | 38ms | 同左 | 入场逐张错开 |

### 13.3 毛玻璃

- 只用在不透明度足够的元素上：卡片、工具列表、分发方式卡与目标格、还原横幅、底部操作条、确认卡、空状态图标块。滚动区域本身（`.content`）不加模糊。
- 模糊必须配半透明底与顶部内高光，边框用 `--raw-line` 70% 混透明；选中态边框仍是 `--raw-accent`，状态只用边框与胶囊表达（§2 规则不变）。
- 文字对比度按「模糊后实际底色」验：浅色 `--raw-muted` 在 `--glass-card` 上 ≥4.5:1，深色同理；实现时用真实截图取样复测（DESIGN §9 的验收方法）。
- 降级：`@supports not (backdrop-filter)` 或性能不达标（PRD Q-PAGE-5）时，上述元素改用不透明 `--raw-card`，色场关闭。

### 13.4 背景色场

- 页面面板底层一个 `position:absolute; inset:0; z-index:-1; pointer-events:none` 的层，里面三团径向渐变色块，各自 `filter: blur(40px)`，26–32 秒往返漂动（只动 `transform`）。
- 色场固定在面板上，不随内容滚动；不放在滚动容器里。
- 这是 §2「禁止渐变」在技能页内的唯一例外；卡片、按钮、文字不用渐变（卡片悬停光斑是 `radial-gradient` 的高光，算在色场例外里，只在悬停时出现）。

### 13.5 动效

- 允许超过 §3.3 的 240ms 上限，最长 520ms（入场、操作条弹入、确认卡弹入）；色场漂动、空状态浮动、骨架流光是循环动画，属于具名例外。
- 只动 `transform`、`opacity`、`filter`（入场模糊 6px→0）、`background-position`；不动宽高、位置等几何属性。`will-change` 只给色场。
- 入场错开只用在一次进入的列表；同一屏最多错开 24 项，其余直接显示，避免长列表等待。
- `prefers-reduced-motion: reduce` 时：所有动画与过渡 1ms，循环动画停止，入场元素直接可见。

### 13.6 布局尺寸

| 项 | 值 |
| --- | --- |
| 页面面板 | 标题栏以下，四周 8px 画布缝（顶部紧贴标题栏），圆角 16、1px `--raw-line` |
| 左侧导航 | 248px；窗口宽 < 1024px 时 56px 图标栏 |
| 主区内边距 | 左右 24px；工具栏上 16 下 12 |
| 卡片网格 | `repeat(auto-fill, minmax(220px, 1fr))`，间距 12 |
| 目标网格 | `repeat(auto-fill, minmax(190px, 1fr))`，间距 10；通用目录占 2 格 |
| 卡片 | 最小高 148，内边距 14，圆角 12；图标块 32×32 圆角 10 |
| 底部操作条 | 距主区左右 24、底 16，圆角 12；出现时内容底部留 88px |
| 确认卡 | 宽 540，窗口窄时 `100% - 48px` |

### 13.7 视觉验收

- 原型截图 18 张（`design/r2/shots-page/`）作为审核依据；实施后按同一组状态在真窗口重拍并对照，差异写进 UI 稿。
- 性能：真窗口里 200 张卡片滚动的帧间隔 p95，门槛在计划 spike 后定（PRD AC-PAGE-7）。
- 对比度：12 套主题色 × 毛玻璃底，强调色文字、次级文字、按钮文字按真实截图取样复测。
- T11 实测（2026-11-24）：真窗口 8 个状态 × 浅 / 深 + 窄窗 2 张见 `design/r2/shots-m8/`；默认橙的卡片描述 6.78 / 5.64、分类 5.21 / 4.89、工具路径 8.09 / 7.52、技能名 12.53 / 10.61；6 套主题色 × 2 主题共 12 组最低 4.53（导航当前项强调色文字）/ 5.64（卡片描述），全部 ≥ 4.5。差异见 UI 稿 11.5。
- T12 性能实测（2026-11-24）：200 个技能、毛玻璃开——进入 p95 138.3 ms、滚动 p95 0.34 个刷新间隔、慢帧 0、切回工作台 p95 37.3 ms，未触发自动降级；M5–M7 基准无退化。
