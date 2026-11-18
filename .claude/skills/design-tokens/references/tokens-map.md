# 令牌速查（旧版 tokens.css 实测）

来源：`E:\WebApp\Mile-Project\Mile-Knowledge-Base-master\src\renderer\src\assets\tokens.css`（4913 行）+ 根 `DESIGN.md`。阶段 06 拆分时的分组依据；改动先看本表确认令牌归属。

## 文件结构（4913 行的分块）

1. 文件头注释（`:1-13`）：不用 `@layer` 的 5 条手写规则（transform 过渡要列颜色属性、`:active` 在 `:hover` 后、reduced-motion 块放最末且 `:not()` 对齐、`color-mix` 写 `in srgb`、`--glow` 消费者给兜底）。
2. 原始层浅色 `:root,[data-preview='light']`（`:26`）+ 深色 `[data-theme='dark'],[data-preview='dark']`（`:180`）。
3. 语义层 `@theme inline`（`:221`）+ `@custom-variant dark`（`:242`）。
4. 基础（reset、`body`、`font-variant-numeric`）、`.text-*` 字号类、`.theme-switching`、`.drag-region`/`.no-drag`。
5. 之后是数十个组件/面板原语块（`.field`、`.btn`、`.pill`、`.surface-card`、`.popover`、`.codeblock`、`.prose`、`.thinking-*`、`.restore-note`…），按功能分节注释。
6. 文件最末：`@media (prefers-reduced-motion: reduce)` 两个块（`:4823` thinking 降级、`:4901` 浮层/弹性归零）。

## 三层

`--raw-*`（真实值，两主题各一套）→ `@theme inline` 的 `--color-*` / `--font-*`（语义入口）→ 组件 `var(--color-*)` / `var(--raw-*)` / `.text-*`。`--raw-*` 也直接被组件引用（面板层级、状态色）。

## 原始层全表

浅色（`:root`）：canvas `#eceef1`、card `#ffffff`、raised `#eceef1`、sunken `#f2f3f5`、line `#e1e3e7`、line-strong `#c9cdd4`、strong `#15171c`（ink=别名）、text `#24272e`、muted `#4b505a`、faint `#5c616b`、live `#15803d`、info `#4338ca`、warn `#92400e`、fault `#b91c1c`、idle `#6b7280`、on-ink `#ffffff`、accent `#4f46e5`、accent-hover `#4338ca`、accent-active `#3730a3`、accent-soft `#ebeafd`、on-accent `#ffffff`、focus `#4f46e5`、shadow-panel `0 4px 14px rgb(20 22 28/.06)`、shadow-float `0 16px 40px rgb(20 22 28/.16)`、glass `rgb(255 255 255/.72)`、glass-line `rgb(255 255 255/.6)`、panel-sidebar `#f4f5f7`、panel-editor `#ffffff`、panel-agnes `#ffffff`。

深色（`[data-theme='dark']`）：canvas `#0e0f11`、card `#1c1e22`、raised `#26282d`、sunken `#16171a`、line `#2b2e34`、line-strong `#3b3f47`、strong `#f3f4f6`、text `#e6e7ea`、muted `#a6aab3`、faint `#9a9ea8`、live `#4ade80`、info `#8b8ff8`、warn `#fbbf24`、fault `#f87171`、idle `#8b8f98`、on-ink `#0e0f11`、accent `#8b8ff8`、accent-hover `#a2a5fa`、accent-active `#7579ec`、accent-soft `#2a2b45`、on-accent `#111214`、focus `#8b8ff8`、shadow-panel `none`、shadow-float `0 16px 40px rgb(0 0 0/.5)`、glass `rgb(28 30 34/.72)`、glass-line `rgb(255 255 255/.08)`、panel-sidebar `#141518`、panel-editor `#18191d`、panel-agnes `#1c1e22`。

非颜色原始令牌（`:root`）：font-ui（-apple-system,BlinkMacSystemFont,Segoe UI Variable Display,Segoe UI,Microsoft YaHei UI,system-ui,sans-serif）、font-mono（Cascadia Code,SF Mono,Menlo,Consolas,DejaVu Sans Mono,monospace）、titlebar-h 40、agnes-rail-w 40、gap-panel 8、gap-tile 12、pad-panel 8、pad-tile 16、radius-panel/card/control/chip/pill 16/12/10/999/999、ease `cubic-bezier(0.22,1,0.36,1)`、ease-spring `cubic-bezier(0.34,1.56,0.64,1)`、dur 150ms、dur-spring 220ms、text-caption/meta/body-sm/body/read/title/display 11/12/13/14/15/18/24。

## 面板相对层级（局部变量，每个 `[data-panel]` 定义）

`--panel-bg`（本面板底色）、`--panel-card`（浅=白 / 深=`--raw-raised`，`.surface-card`/`.turn`/`.runtime-controls`）、`--panel-sunken`（两主题都 `--raw-sunken`，凹区内卡）。`.surface-card` 兜底 `var(--panel-card, var(--raw-card))`。

## 已知旧值漂移（不要搬）

- `ThemeService.ts:13` 窗口 `#F6F7F9`/`#0B0F14` ≠ `--raw-canvas`；新版以令牌为准（PRD Q6）。
- `--syntax-*` 已随 live 模式下线（`tokens.css:55`），不存在。
- 字号别名：caption-lg=caption、meta-lg=meta、body-md=body-sm、body-lg=read。
- `--ok` 是 `--live` 的别名；`--shadow-lift-lg`=`--shadow-panel`；`--text-on-accent`=`--raw-on-accent`。
