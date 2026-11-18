---
name: design-tokens
description: Mile Knowledge Base（Go 重写版）的设计令牌落地与维护：唯一视觉基线是旧版 DESIGN.md + tokens.css（4913 行），三层令牌结构（--raw-* 原始值 → 语义令牌 → [data-theme='dark'] 深色同名覆盖），窗口 BackgroundColor 与 --raw-canvas 一致，Tailwind 4 语义层 @theme inline，phosphor 深导入图标，胶囊状态色、状态色只染 1px 边框等硬规则，防白闪与 .theme-switching，prefers-reduced-motion，WCAG 对比度校验。只要在写或改界面样式、颜色、字号、间距、主题、深色模式、图标，或要新增/修改令牌，都应加载本 skill。
---

# design-tokens

唯一视觉基线：旧版 `E:\WebApp\Mile-Project\Mile-Knowledge-Base-master\DESIGN.md`（45K，通读）+ `src/renderer/src/assets/tokens.css`（4913 行）。本项目根目录 `DESIGN.md` 目前是空模板，**阶段 06 才填**——在那之前一切以旧版 DESIGN.md 与 tokens.css 为准；阶段 06 把规范转写进根 DESIGN.md 并拆分 tokens.css。

依据：PRD v1.1 FR-THEME（三档、防白闪）、FR-DOC（阅读排版）、FR-AI-CHAT（气泡/胶囊）、8 节（状态胶囊规格）、10.3（对比度 ≥4.5:1、focus 可见、reduced-motion）；技术设计 §2.2（Tailwind 4.3.5 + tokens.css 迁移）。

## 1. 何时加载 / 何时不加载

- 加载：写或改任何 CSS / 组件样式 / Tailwind 类；改颜色、字号、间距、圆角、阴影、动效、图标；接入或切换主题；新增令牌；排查「样式与旧版不一致」「深色下不对」「对比度不够」。
- 不加载：Go 后端逻辑（`go-service-engineering`）、Markdown 结构渲染（`markdown-render-pipeline`）、窗口/绑定脚手架（`mygo-app-scaffold` / `mygo-web-app`）。

## 2. 单一来源与迁移策略

```text
阶段 06 前：旧版 DESIGN.md（规范）+ 旧版 tokens.css（数值）是唯一权威
阶段 06：  规范转写进根 DESIGN.md（契约，人和 AI 读）
            tokens.css 直接移植并拆分（按令牌 / 组件原语 / 面板分文件）
之后：     根 DESIGN.md（契约）-> tokens/*.css（唯一实现）-> 组件只引用令牌
           -> main.go WindowOptions.BackgroundColor（= --raw-canvas）
```

**改令牌流程**：先改根 DESIGN.md 的两列值（浅/深），跑对比度（§6），再同步 CSS 文件，再确认 `BackgroundColor`（改 `--raw-canvas` 时）。不允许在组件里「临时」写色值。

## 3. 三层令牌结构

方向单一：**组件只用语义令牌 → 语义令牌指向原始变量 → 原始变量按 `[data-theme]` 整体翻转**。

- **原始层**（`tokens.css:26` `:root` 浅 / `:180` `[data-theme='dark']` 深）：`--raw-*` 存真实色值。深色全部表面落在同一色相（220°-228°），按明度分层。
- **语义层**（`tokens.css:221` `@theme inline`）：`--color-*` 与 `--font-*` 指向 `--raw-*`，是 Tailwind 4 与组件用的入口；`@custom-variant dark`（`:242`）把 `dark:` 变体绑到 `data-theme`。
- **关键纪律**：`@theme inline` 不是 `@theme`——Tailwind 4 只输出被引用的 `@theme` 变量，`inline` 保证语义令牌指向 var() 而不是被内联成定值（`tokens.css:128` 注释）。

## 4. 主要语义令牌（旧版 tokens.css 实测值）

| 组 | 令牌 | 浅色 | 深色 | 用途 |
| --- | --- | --- | --- | --- |
| 画布/表面 | `--raw-canvas` | `#eceef1` | `#0e0f11` | 最底层画布；**= 窗口 `BackgroundColor`（PRD Q6：以令牌为准，不用旧版 ThemeService 里的 `#F6F7F9`/`#0B0F14`）** |
| | `--raw-card` | `#ffffff` | `#1c1e22` | 卡片 |
| | `--raw-raised` | `#eceef1` | `#26282d` | 输入框/分段槽/用户气泡 |
| | `--raw-sunken` | `#f2f3f5` | `#16171a` | 历史栏/代码块/表头 |
| 线 | `--raw-line` | `#e1e3e7` | `#2b2e34` | 发丝线 |
| | `--raw-line-strong` | `#c9cdd4` | `#3b3f47` | 控件轮廓 |
| 文字 | `--raw-strong` | `#15171c` | `#f3f4f6` | 标题/气泡文字（`--raw-ink` 是别名） |
| | `--raw-text` | `#24272e` | `#e6e7ea` | 正文 |
| | `--raw-muted` | `#4b505a` | `#a6aab3` | 次级 |
| | `--raw-faint` | `#5c616b` | `#9a9ea8` | 辅助（作文字也须 ≥4.5:1） |
| 状态 | `--live`/`--ok` | `#15803d` | `#4ade80` | 只染 1px 边框 + 文字（见 §7） |
| | `--info` | `#4338ca` | `#8b8ff8` | |
| | `--warn` | `#92400e` | `#fbbf24` | |
| | `--fault` | `#b91c1c` | `#f87171` | |
| | `--idle` | `#6b7280` | `#8b8f98` | |
| 强调 | `--raw-accent` | `#4f46e5` | `#8b8ff8` | 只表达交互（焦点/选中/每个视图一个主操作） |
| | `--raw-accent-soft` | `#ebeafd` | `#2a2b45` | 选中行底 |
| | `--raw-on-accent` | `#ffffff` | `#111214` | 强调色上的前景（分主题） |
| | `--focus` | `#4f46e5` | `#8b8ff8` | 焦点环 |
| 面板 | `--raw-panel-sidebar` | `#f4f5f7` | `#141518` | 侧栏低一档 |
| | `--raw-panel-editor` | `#ffffff` | `#18191d` | 文档面板 |
| | `--raw-panel-agnes` | `#ffffff` | `#1c1e22` | 对话面板（深色最亮） |
| 浮层 | `--glass` / `--glass-line` / `--shadow-float` | 见 tokens.css | | 毛玻璃只给浮层 |

面板内三级相对层级（不写死 `--raw-card`）：`--panel-bg`（跟面板同色）、`--panel-card`（比面板亮一档，`.surface-card`/`.turn`）、`--panel-sunken`（比浮卡低一档）。`[data-panel]` 各自定义，随主题翻转（旧版 DESIGN.md §七）。

## 5. 字号、圆角、动效、间距、阴影

- 字号阶梯（`tokens.css:147`）：`--text-caption` 11 / `--text-meta` 12 / `--text-body-sm` 13 / `--text-body` 14 / `--text-read` 15（阅读正文）/ `--text-title` 18 / `--text-display` 24。组件只用 `.text-*` 类或 `var(--text-*)`。
- 圆角四档：`--radius-panel` 16 / `--radius-card` 12 / `--radius-control` 10 / `--radius-chip`=`--radius-pill` 999。具名例外仅：任务复选框 3px、过程步骤节点 2px、进行中指示器方块 1px。
- 动效：`--ease` `cubic-bezier(0.22,1,0.36,1)` + `--dur` 150ms（颜色类）；`--ease-spring` `cubic-bezier(0.34,1.56,0.64,1)` + `--dur-spring` 220ms（按压/开关/展开折叠/浮层）。**任何过渡 ≤240ms**，只动 `opacity`/`transform`/`background-position`。
- 间距：`--gap-panel` 8（面板缝=拖拽条）、`--gap-tile` 12、`--pad-panel` 8、`--pad-tile` 16；4px 栅格。
- 阴影两档：`--shadow-panel`（浅 `0 4px 14px rgb(20 22 28/.06)`，**深色 none**）、`--shadow-float`。不叠加 border + shadow。
- 标题栏 40px `--titlebar-h`、文档竖轨 40px `--agnes-rail-w`。

## 6. 对比度校验

改任何涉及文字或可交互元素的颜色，用脚本算对比度并写回 DESIGN.md（正文 ≥4.5，非文字界面元素 ≥3）：

```bash
python -I -c "
def L(h):
    h=h.lstrip('#'); c=[int(h[i:i+2],16)/255 for i in (0,2,4)]
    c=[x/12.92 if x<=0.03928 else ((x+0.055)/1.055)**2.4 for x in c]
    return 0.2126*c[0]+0.7152*c[1]+0.0722*c[2]
a,b=sorted([L('#5c616b'),L('#eceef1')],reverse=True); print(round((a+0.05)/(b+0.05),2))"
```

faint/muted 在 canvas、card、三块面板底色上的组合都要按 4.5:1 算（旧版 `#8a929e` 在浅画布 2.93:1 是回归项）。

## 7. 硬规则（命中即违规）

- 组件里无 hex/rgb/hsl；只用 `var(--…)` 或 `.text-*`；深浅两值一一对应，不允许某令牌只在一个主题有值。
- **状态色只染 1px `border-color`**（`color-mix(in srgb, var(--tone) 40%, var(--color-line))`，fault 用 45%），卡片/面板/整行背景不染状态色。唯一例外 `.pill`：20px 高、999 圆角、`--text-caption` 600，底 `color-mix(--tone 16%, --raw-card)`、字 `color-mix(--tone 72%, --raw-strong)`，必须配文字，图标 11px（PRD 8 节）。
- 强调色只表达交互，永不表达状态；每个视图最多一个强调色主操作。
- 禁：渐变、多层阴影、emoji、四档外圆角、>240ms 过渡、浮层外的模糊、胶囊外的染底。
- 图标一律 `@phosphor-icons/react` **深导入**（`src/lib/icons.ts` 集中出口，`import { XxxIcon } from '@phosphor-icons/react/X'`）；三档字重：导航选中 `fill`、未选中 `regular`、强调操作 `bold`；常用尺寸 11-16px（胶囊 11、行内 13-14、头像框外 15-16）；图标按钮必须有 `aria-label`；不用 emoji。
- `prefers-reduced-motion`：循环动画降级为静态（`.thinking-*` 停 `opacity:.65`），浮层/弹性过渡归零；覆盖块放文件末尾且 `:not()` 与外面对齐（`tokens.css:4823/4901`）。
- Tailwind 4.3.5：`main.css` 只 `@import 'tailwindcss'` + `./tokens.css`；`dark:` 已由 `@custom-variant` 绑到 `data-theme`，**不用 `@media (prefers-color-scheme)` 写颜色**。

## 8. 主题机制（FR-THEME）

- 偏好三值 `system`/`light`/`dark` 存应用设置（Go 侧）；根元素 `<html data-theme>` 始终带**解析后**的值，CSS 只写 `:root` 浅 + `[data-theme='dark']` 深两块。
- `data-theme` 写在 `<html>`；用户显式选择覆盖系统——不读 `prefers-color-scheme` 定主题。
- 防白闪：`index.html` 引独立 `theme-init.js`（CSP 禁内联脚本）按 `prefers-color-scheme` 先写 `data-theme` + `color-scheme`；Go 建窗前 `Theme.SetSource` + `WindowOptions.BackgroundColor` = `--raw-canvas`（浅 `#eceef1` / 深 `#0e0f11`）。
- 切换帧给 `<html>` 加 `.theme-switching`（`*{transition:none!important}`，`tokens.css:317`），一帧后移除。
- 主题缩略图：`[data-preview='light'|'dark']`（`tokens.css:27,181`）让设置页两张缩略图在任何当前主题下按自己那套令牌上色。
- `color-scheme` 跟 `data-theme` 切，滚动条与原生控件随主题变。

## 9. 交付前自检

- [ ] 数值与旧版 tokens.css 一致（阶段 06 前）；之后三处一致：根 DESIGN.md、CSS 文件、组件
- [ ] 新增/改的颜色在两套主题下算过对比度并写回 DESIGN.md
- [ ] `WindowOptions.BackgroundColor` == `--raw-canvas`（不用旧版 `#F6F7F9`/`#0B0F14`）
- [ ] 无硬编码色值/字号/圆角/阴影/魔法时长；状态色只染 1px 边框或 `.pill`
- [ ] 图标深导入、有 `aria-label`、字重/尺寸符合规则、无 emoji
- [ ] 主题切换无白闪、`.theme-switching` 生效、`data-theme` 在 `<html>`、`dark:` 变体可用
- [ ] reduced-motion 下循环动画与弹性过渡已降级
- [ ] 改动令牌走了「DESIGN.md → 对比度 → CSS → BackgroundColor」流程
