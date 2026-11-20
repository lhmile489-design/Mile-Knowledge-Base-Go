# M3.5 实施计划：主题色

> 状态：已完成（2026-11-20）。计划 v0.1 经用户批准后按「一次一个任务」执行完毕，T00–T11 全部交付；验收与门禁证据见第 6、9 节；提交在本地，待用户确认推送。
> 本次目标：交付 PRD 第 16 节 FR-ACCENT。闭环为「标题栏色点或设置页选一个预设 / 输入自定义色 → 全界面立即换色 → 重启后第一帧就是所选颜色」，6 个预设在浅、深两种主题下的对比度由自动测试兜底。
> 上一里程碑：docs/PLAN-M3.md（已完成并推送，`765f908`；之后 R2 的需求与设计提交在本地，未推送）。下一里程碑：M4（依赖本里程碑，技能浮层按新令牌实现）。

## 1. 目标、范围与授权

### 1.1 交付的可观察结果

1. 标题栏「技能」按钮与主题切换之间有一个当前主题色的色点；点开是色板：6 个预设（橙、靛、蓝、绿、青、紫）加自定义 `#RRGGBB`。选中即生效并写入设置，Esc 或点外面关闭，焦点回到色点。
2. 设置 → 常规 → 外观，「主题」下面有「主题色」一行，内容与色板相同，两处同步。
3. 换色后，强调色一族全部跟着变：主按钮与发送按钮（浅色下白字）、选中的分段项、链接、选中树行、焦点环、开关、复选框、拖拽条、输入框焦点描边。状态色、画布与面板、中性文字不变。
4. 默认是橙色；1.0.0 升级上来的设置没有 `accent`，按橙色显示。选「靛」时界面与 R1 逐像素一致（字体渲染除外）。
5. 冷启动第一帧就是所选颜色（预设与带缓存的自定义色）。
6. 自定义色对比度不够时自动调整，色板与设置页显示「实际使用」的颜色；格式不对时提示、不保存。

### 1.2 关联需求与契约

| 来源 | 位置 |
| --- | --- |
| 需求 | PRD 第 16 节（v2.3）：FR-ACCENT、AC-ACC-1–6 |
| 视觉 | DESIGN.md 第 12 节：令牌模型（12.1）、12 套预设（12.3）、自定义规则（12.4）；docs/UI设计稿.md 第 10 节；原型 `design/r2/accent-picker.html` |
| 接口 | API.md 第 11 节：M60 / M61 扩展 `accent` |
| 数据 | 数据库设计 4.1.1：`settings.json` 的 `accent` 字段、回落规则、`version` 不升 |
| 技术 | 技术设计第 14 节：PreloadScript 首帧方案、待验证项 A-1–A-4 |
| 决定 | decisions.md D-015 第 1–6 条 |

### 1.3 明确不做

| 不做 | 原因 / 去向 |
| --- | --- |
| 换字体 | D-015 第 4 条：「字体」指文字颜色 |
| 换画布、面板、状态色 | PRD 16.3 |
| 技能浮层（M4）的实现 | M4 的范围；本里程碑只把 `design/r2/skills-panel.html` 的截图按新默认色重拍 |
| 深色主题按钮改白字 | D-015 第 6 条：深色保持亮底近黑字 |
| GitHub Release、代码签名、自动更新 | 沿用 M3 |

### 1.4 审批记录

见第 8 节。

## 2. 前置资料与现状

| 输入 | 状态 | 缺口 | 是否阻断 |
| --- | --- | --- | --- |
| PRD 第 16 节 | 已确认 v2.3 | 无 | 否 |
| DESIGN.md 第 12 节、UI 稿第 10 节、原型 | 已审核 | 无 | 否 |
| API 第 11 节、数据库设计 4.1.1、技术设计第 14 节 | 已审核；已设计、未实现 | A-1–A-4 未验证（T00） | T00 阻断其后任务 |
| 本计划待定事项 | B-M35-01 版本与安装包、B-M35-02 审查方式 | 见第 8 节 | 审批时一并定 |

现有代码盘点（2026-11-20，本地 `ab36fc5`）：

| 能力 | 现状 | M3.5 要做 |
| --- | --- | --- |
| 强调色令牌 | `src/styles/tokens.css:68-75`（浅）、`:205-211`（深），只有一套靛蓝；TS / TSX 里没有直接写强调色 | T03 拆分并加 12 套 |
| 强调色文字用法 | `color: var(--raw-accent)` / `var(--color-accent)` 共 29 处：tokens.css 16、chat.css 4、mutation.css 3，editor / health / overview / search / settings / sidebar 各 1 | T03 全部改用 `--raw-accent-text` |
| 带文字的强调色底 | `.btn-accent`（`tokens.css:2451`）用 `--text-on-accent`；`.composer-send`（`:1095`）前景是 `var(--raw-card)`；`accent-color` 两处（`:3809`、`:4708`） | T03 改用按钮令牌与 `--raw-on-accent` |
| 设置 | `internal/settings/settings.go`：已知字段表 `:327` 不含 `accent`，未知字段原样保留；`internal/contract/contract.go:234-252` 的 `AppSettings` / `SettingsPatch` | T01 加字段、规范化、校验 |
| 首帧 | `public/theme-init.js` 只设明暗；`main.go:88` 的 `NewWindow` 没设 `Page`；生产 CSP 是 meta 标签 `script-src 'self'`（`vite.config.ts:8`） | T00 验证、T04 实现 |
| PreloadScript | mygo v0.10.0：`WindowOptions.Page`（`window.go:211`）→ `PageOptions.PreloadScript`（`page.go:299`）→ WebView2 `AddScriptToExecuteOnDocumentCreated`（`internal/windows/webview.go:796`） | T00、T04 |
| 标题栏、设置常规 | `src/components/TitleBar.tsx`、`src/components/settings/GeneralSection.tsx` | T05、T06 |
| 视觉对照 | `scripts/visual/capture-new.py`（49 张，生成物在 `design/compare/`） | T08 加 `--accent` 参数，在靛下重跑 |
| e2e | `scripts/e2e/ac-m0..m3.sh`，`bun run test:e2e` 100 项 | T07 新增 `ac-m35.sh` 并全量回归 |

## 3. 总路线

规模：约为 M3 的四分之一。主要工作在 T03 的令牌拆分，其余任务都小。

主要风险：
- **A-1 不成立**（宿主注入的脚本被页面 CSP 拦下）：首帧方案要换成绑定方法同步取值，代价是首帧先显示默认橙色。T00 先验证，结果出来再继续。
- **29 处文字用法漏改**：换成橙、绿、青时那处文字对比度不达标。T07 的对比度检查对真实组件取计算后的颜色，不只检查令牌。
- **R1 外观被意外改动**：T03 用「靛预设下 49 张截图逐像素比对」兜底。

## 4. 任务清单

执行方式沿用：**一次只做一个任务，做完汇报并停下，等用户放行后再做下一个**。状态：待做、进行中、阻塞、待验证、已验证。

| 编号 | 目标 | 修改范围 | 依赖 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- |
| T00 | 首帧方案验证（A-1–A-4） | `spikes/r10-preload/`（新） | 用户批准本计划 | RESULT.md 给出 A-1–A-4 的实测结论；若 A-1 不成立，提出替代方案并停下等用户决定 | 待验证（证据齐，待用户验收） |
| T01 | Go：`accent` 字段、规范化与校验 | `internal/settings/{settings,accent}.go`、`internal/contract/contract.go`、生成的绑定 | T00 | V-ACC-GO 全部通过；`check:bindings` 通过 | 待验证（门禁已过，待用户验收） |
| T02 | 前端配色算法 | `src/lib/accent.ts`（新）、`src/lib/accent.test.ts`（新） | T00 | 12 套预设快照与 DESIGN.md 12.3 逐值相同；V-ACC-ALG 通过 | 待验证（门禁已过，待用户验收） |
| T03 | 令牌拆分与 12 套配色 | `src/styles/*.css`、`scripts/visual/capture-new.py` | T02 | 29 处改名、按钮与发送按钮改按钮令牌；默认橙；V-ACC-TOK 通过（含靛下逐像素比对） | 待验证（门禁已过，待用户验收） |
| T04 | 首帧链路 | `main.go`、`public/theme-init.js`、`src/stores/appStore.ts`、`src/lib/accent.ts` | T00、T01、T02、T03 | V-ACC-BOOT 通过 | 待验证（门禁已过，待用户验收） |
| T05 | 标题栏色点与色板 | `src/components/TitleBar.tsx`、`src/components/AccentPicker.tsx`（新）、`src/styles/accent-picker.css`（新） | T04 | 交互与可访问性按 UI 稿 10.1、DESIGN.md 12；单测与构建通过 | 待验证（门禁已过，待用户验收） |
| T06 | 设置页「主题色」一行 | `src/components/settings/GeneralSection.tsx`、`src/components/AccentOptions.tsx`（新）、`src/lib/useResolvedTheme.ts`（新）、`src/styles/accent-picker.css` | T05 | 与色板同步；单测与构建通过 | 待验证（门禁已过，待用户验收） |
| T07 | 验收与全量回归 | `scripts/e2e/ac-m35.sh`（新）、`scripts/e2e/accent-contrast.js`（新） | T06 | AC-ACC-1–6 与真实组件对比度检查通过；`bun run test:e2e` 全过；冷启动性能不劣于 M3 | 待验证（门禁已过，待用户验收） |
| T08 | 视觉重拍 | `scripts/visual/capture-new.py`、`design/r2/shots/`、`design/r2/shots-accent/` | T07 | 靛下 R1 对照无回归；6 预设 × 浅 / 深真实界面截图；技能浮层 18 张按橙重拍 | 待验证（门禁已过，待用户验收） |
| T09 | 版本与安装包（按 B-M35-01） | `mygo.config.ts`、docs/发布清单.md | T08 | 见 B-M35-01；安装动作执行前再单独确认 | 待验证（门禁已过，待用户验收） |
| T10 | 整体审查（按 B-M35-02） | 审查报告、本计划第 6 节 | T09 | 阻断项为 0，需改项已修或经用户接受 | 待验证（门禁已过，待用户验收） |
| T11 | 收尾 | README、CLAUDE.md、本计划、docs/PLAN-M0.md 路线 | T10 | 文档同步；列提交；推送前询问用户 | 待验证（文档已同步，推送待用户确认） |

依赖顺序 T00 → T11，串行。T01 与 T02 互不依赖，但仍按「一次一个」分开放行。

T01 实现中补充的两处（已在契约范围内，未改契约）：
- 读入时 `accent` 用单独宽松解析：字段类型不对只回落成默认色，其余字段坏了才按「文件损坏」备份（数据库设计 4.1.1 的「不报错、只回落」）。
- `Store.Patch` 改为先把所有字段校验完再组装，任一不合法都不留下半个 patch（API §4.8 的要求；此前 theme 合法而后续字段非法时，内存里会先改掉 theme）。

## 5. 单任务精确交接

> 通用纪律：Go 与 TS 的业务逻辑测试先行（先写能因功能缺失而失败的测试）；样式与视觉按第 6 节的截图和对比度检查验证，不机械套 TDD；测试和脚本只用临时目录与临时 APPDATA，不读写真实应用数据；中文 UI、无 emoji；提交前扫描密钥与真实路径。

### T00 首帧方案验证（已完成）

> 结果（2026-11-20）：A-1 不受 CSP 限制、A-2 早于页面脚本、A-3 首帧即所选色、A-4 缺值时降级为默认橙不报错，另测了含 `</script>` 的值；证据 `spikes/r10-preload/RESULT.md`，技术设计 14.3 已回写。**结论：方案成立，T04 按原方案实现。**

- 在 `spikes/r10-preload/` 写一个最小 mygo 程序：窗口 `Page.PreloadScript` 注入 `window.__MKB_ACCENT__ = {"preset":"blue"}`；页面用与生产相同的 CSP meta 标签（`vite.config.ts:8` 的值），`<head>` 里同源加载一个仿 `theme-init.js` 的脚本，读到值后设 `data-accent`，并把读取结果、执行顺序记录下来。
- 实测：
  - A-1：CSP 下注入脚本能否执行；
  - A-2：注入脚本是否早于 `<head>` 里的同源脚本；
  - A-3：用 automation 的 `capture` 截第一帧，检查背景色块是否已是所选颜色；
  - A-4：不注入时的行为（模拟自定义色无缓存），记录是否有一帧默认色。
- JSON 用 `encoding/json` 编码，并测一个含 `</script>` 与引号的值，确认不会破坏脚本。
- 产物：`spikes/r10-preload/RESULT.md`。A-1 或 A-2 不成立时，在 RESULT.md 写替代方案（如绑定方法同步取值）与代价，停下等用户决定，不自行改契约。

### T01 Go：`accent` 字段

- `contract.AppSettings` 加 `Accent AccentSetting`，`SettingsPatch` 加 `Accent *AccentSetting`；类型与 API 11.1 一致（`AccentTokens` 10 个键）。
- `settings.go`：
  - 已知字段表加 `accent`；
  - 读入按数据库设计 4.1.1 规范化：缺省 / 非法 → `{preset:"orange"}`；`custom` 的 `hex` 非法 → 橙；`cache` 非法 → 只丢 `cache`；
  - 规范化结果不单独写盘。
- `Patch`：按 API 11.2 校验，整体替换；全部字段校验完再组装，非法时 `invalid-input` 且不改动已保存的值。
- 测试（先写）：API 11.2 表每行一条；数据库设计 4.1.1 每条回落规则一条；1.0.0 格式的种子文件（无 `accent`）读出为橙；带 `accent` 的文件经旧代码路径往返（未知字段保留）不丢值。
- 重新生成绑定，`check:bindings` 通过。

### T02 前端配色算法

- `src/lib/accent.ts`：导出 6 个预设的浅 / 深配色（常量，数值抄 DESIGN.md 12.3），以及 `deriveAccent(hex)` 返回 `{light, dark}`，规则按 DESIGN.md 12.4（与 `design/r2/accent-picker.html` 里的实现一致）。
- 测试：
  - 6 个基色走 `deriveAccent` 的结果，除「靛」外与常量逐值相同（靛是锁定的旧值）；
  - 12 套配色逐条断言 DESIGN.md 12.3 的对比度标准；
  - 边界色：`#ffff00`、`#000000`、`#ffffff`、`#808080`，结果都满足对比度标准，`#808080` 标记「接近灰色」；
  - 非法输入（`#fff`、`ff7b29`、`#gggggg`）返回错误。
- 这里的对比度函数是唯一实现，T07 复用。

### T03 令牌拆分与 12 套配色

- 新增 `--raw-accent-text`、`--raw-accent-btn`、`--raw-accent-btn-hover`、`--raw-accent-btn-active`；`:root` 与 `[data-theme='dark']` 的默认值改为橙色那两套。
- 12 段 `[data-accent='…']` / `[data-theme='dark'][data-accent='…']`，各只覆盖 DESIGN.md 12.1 的 10 个令牌；外加 `custom` 两段，从 `--accent-light-*` / `--accent-dark-*` 映射（技术设计 14.2）。
- 29 处 `color:` 改用 `--raw-accent-text`；`.btn-accent`、`.composer-send` 及其悬停 / 按下改用按钮令牌，`.composer-send` 前景改为 `--raw-on-accent`；两处 `accent-color` 改用 `--raw-accent-btn`。逐处在交付说明里列出。
- 其余背景、描边类用法保持 `--raw-accent`。
- 验证：见 V-ACC-TOK。

### T03 补充说明（2026-11-20 实测）

- 「靛下 49 张逐像素比对」实际做了 50 张（M3 的清单里 60 号也在内），对照的是**同一份代码改动前构建**的截图，因为 M3 留下的那套截图分辨率不同（2498×1565 对 1424×892），不能直接比。
- 差异判定标准：橙色像素 0 且「一侧橙、一侧靛」的互换像素 0；其余差异逐张看过，都是时间文本（「刚刚」/「N 分钟前」、搜索耗时）或运行态（强度选择、最近文档顺序、流式文本）。
- 两处修正：
  1. 主题色默认值原先与 `[data-preview='light'|'dark']` 共用选择器，导致设置页主题缩略图不跟随所选颜色；已拆成独立的 `:root` / `[data-theme='dark']` 规则。
  2. `capture-new.py` 注入 `data-accent` 后立刻截图会截到换色前的帧；已加 0.3 秒等待。`CAPTURE_ACCENT` 环境变量在 T08 正式用于重拍。

### T04 首帧链路

- `main.go`：在 `NewWindow` 之前读设置，`Page.PreloadScript` 设为 `window.__MKB_ACCENT__ = <json>;`（`encoding/json`）。只设 `PreloadScript`，`Page` 的其他字段保持默认值，不改变现有 DevTools 行为。
- `public/theme-init.js`：读 `window.__MKB_ACCENT__`；预设设 `data-accent`；自定义色有缓存时设 `data-accent="custom"` 和 20 个内联变量；值不合法时什么都不做（CSS 默认是橙）。
- `appStore`：设置载入后与切换时调用同一个 `applyAccent`，与 `theme-init.js` 的结果一致；切换时先在页面上换色，再调 M61（自定义色时带缓存）；M61 失败则回到切换前的颜色并提示。
- 若 T00 结论是替代方案，按用户批准的方案实现，本节随之修订。

### T05 标题栏色点与色板

- 色点：28×28 `.icon-button` 内一个 14px 圆点，`aria-label="主题色：<名称>"`、`aria-haspopup="dialog"`、`aria-expanded`，带 `.no-drag`。
- 色板：`.popover`（毛玻璃）约 272 宽；6 个色块 `role="radiogroup"`，`←`/`→` 移动、Enter 选中；色块底用按钮令牌，选中项外圈加勾；自定义输入框、预览圆点、「实际使用」与错误提示按原型。
- 关闭：Esc、点外面、再点色点；焦点回到色点。`prefers-reduced-motion` 时只做透明度。
- 单测：输入校验与「实际使用」文案的纯函数部分。

### T06 设置页「主题色」一行

- GeneralSection 的「主题」下加一行，复用 T05 的色块与自定义组件；选择即生效、不进保存条（与主题切换一致）；与标题栏色板读同一个 store。
- 完成后：色块与自定义输入抽成 `AccentOptions`，标题栏浮层与设置页共用，`testIdPrefix` 区分两处的测试钩子；解析后的深 / 浅色统一走 `useResolvedTheme`，系统切换时色块勾选的前景色跟着变。设置页这一行用 `variant="inline"`，宽 336px，窄内容区随 `setting-row` 的容器查询换到标签下方。

### T07 验收与全量回归

- `scripts/e2e/ac-m35.sh`：临时 APPDATA；AC-ACC-1、2、4、5 自动化；AC-ACC-6 交给 T08。
- `scripts/e2e/accent-contrast.js`：经 automation `eval` 在真实窗口里，对 6 预设 × 浅 / 深逐套切换，取主按钮、发送按钮、选中分段项、链接、选中树行、复选框的计算后颜色，按 DESIGN.md 12.3 的标准断言（AC-ACC-3）。
- 全量：`bun run test:e2e`（ac-m0..m3 加 ac-m35）退出码 0；`bash scripts/e2e/perf.sh` 冷启动与 M3 实测比较，结果走 `perf:gate`。
- 实现口径（完成后补记）：
  - AC-ACC-1 在全新 APPDATA（无设置文件、无知识库）上读默认橙：`data-accent`、`--raw-accent` / `--raw-accent-btn` / `--focus` 与欢迎页真实 `.btn-accent` 的底色和前景（按解析后的浅 / 深取 DESIGN 12.3 的值）。第一帧的直接证据沿用 T00 的 A-3（第一次 rAF 已是注入色 + 截图像素），e2e 只验证启动后的取值。
  - AC-ACC-2 从色板选蓝，查「立即」生效与设置页同步，重启后再查；设置文件里是 `{preset:"blue"}`。
  - AC-ACC-3 的页面脚本先在工作台采样（选中分段项、选中树行文字、正文链接、发送按钮、真实样式类 `.btn-accent`），再在设置 → 分类与检索采样打开状态的开关滑块与强调底；每套切换后等 150ms 过渡结束再读计算色。树行 2px 指示条不采样：选中态同时有背景、字重与 `aria-selected`，指示条是装饰性强化，不适用 12.3 的「填充与卡片」口径。
  - AC-ACC-4 在设置页输入 `#ffff00`：提示「实际使用 #999900（浅色）」，文件保留输入值与缓存，重启后仍为首帧自定义色。
  - AC-ACC-5 把设置文件的 `accent` 改成乱码字符串：启动回落为橙、无 fault 提示，设置页仍能重选绿色并写回。
  - 脚本启动前检查是否已有应用实例在运行，只结束自己启动的进程；不触碰真实 APPDATA。
- 实测（2026-11-20）：`ac-m35.sh` 37/37（全新安装 5 项、AC-ACC-2 8 项、AC-ACC-3、AC-ACC-4 6 项、AC-ACC-5 6 项）；`accent-contrast.js` 72 个样本，最低值 文字 5.26、按钮 4.51、填充 3.06，全部达标；全量 `bun run test:e2e` 137 项断言（ac-m0 15、ac-m1 16、ac-m2 22、ac-m3 47、ac-m35 37）全部 fail=0；`perf.sh` 典型冷启动 918ms、大库 771ms（M1/M3 基线 910–1101ms / 1093ms），`perf:gate --kind typical` 通过；scroll 与 1000 文件外部改动长任务均为 0。`typical_reader_ms` 沿用脚本既有空值（M0 已记「读模式单独计时未做」）。
- 首轮全量 e2e 在 AC-ACC-2 点蓝前偶发一次 `missing`：popover 容器已出现、色块尚未提交；脚本改为等 6 个 radio 就绪再点击，复跑通过。

### T08 视觉重拍

- `capture-new.py` 加 `--accent <preset>`：在靛下重跑 49 张，与 M3 的截图逐张比较；有差异的列清单逐项说明。
- 真实界面 6 预设 × 浅 / 深各一张总览加一张设置常规页。
- 技能浮层原型改用新令牌（文字用 `--raw-accent-text`、按钮用按钮令牌），按默认橙重拍 18 张。
- 实测（2026-11-20）：`--accent` 参数已加（保留 `CAPTURE_ACCENT` 环境变量兼容），靛下重跑 50 个状态全部成功；截图前的换色改走 `setAccent`（store → CSS），设置页色块勾选也正确显示靛。比较结论：真实差异只有标题栏色点（T05）与设置常规页的「主题色」行（T06），其余为文本抗锯齿、运行态内容与 175% 显示缩放的重采样差异；50 张里没有橙色强调色的面（仅品牌图标与少量抗锯齿像素命中原橙色值）。
- 真实界面截图 13 张（`real-overview-<预设>-<浅/深>` 12 张 + `real-settings-light`）已写入 `design/r2/shots-accent/`，走真实 `setAccent` / `setTheme`。
- 技能浮层原型：文字改用文字令牌、勾选/复选框填充改用按钮令牌，边框与淡底保留强调色；18 张已在默认橙下重拍。

### T09–T11

按 B-M35-01、B-M35-02 的批准结果执行；T11 列出全部提交，推送前询问用户。

- T09 实测（2026-11-20）：版本 1.1.0；`bun run build` 产出 setup-1.1.0（6.7MB）与 exe（19.6MB），版本信息核对为 1.1.0。经用户确认后，`/S` 静默覆盖安装到 `%LOCALAPPDATA%\Programs`，安装目录 exe 变为 1.1.0。升级冒烟用临时 APPDATA：1.0.0 生成的设置改为 `theme: dark`、`sidebarWidth: 260`，升级后深色与 260 保留、默认橙生效，临时设置新增 `accent`（记录与细节见 docs/发布清单.md 第 8 节）。
- T09 插曲：冒烟期间出现一个不是本会话启动的 1.1.0 实例，写入了真实 `settings.json`（新增默认 `accent`）并刷新会话索引；设置项与库完整，会话与检索数据未变；该实例被本会话误结束，已在发布清单记录并向用户说明。

### T10 整体独立审查（task-reviewer，2026-11-20）

- 范围：`origin/main..HEAD` 17 个提交（T00–T09 + 文档 / spike）；审查代理只读、未修改仓库。它本地复跑 `go test`（526）、Vitest（216）、`tsc`、`check:bindings`、gofmt / vet，并逐值比对 12 套预设与 DESIGN 12.3。
- 结论：阻断 0，需改 4，建议 3。需改项全部修复，建议项保留并记录理由。
- 需改 1（近灰提示未接入 UI，DESIGN 12.4）：`AccentOptions.hintFor` 调用 `isNearGray`，灰色输入显示「颜色太接近灰色，选中态可能不明显。」（`data-tone='gray'`，警示色）；新增 1 项组件测试。
- 需改 2（accent 对象内未知键丢失）：`Store` 读入时收集 accent 里不认识的键，保存时与规范值合并写回；新增 `TestAccentUnknownFieldsPreserved`。
- 需改 3（M61 多余字段未拒绝，API 11.2 漂移）：`contract.SettingsPatch` 自定义 `UnmarshalJSON` 标记 accent / cache 内的未知键，`Store.Patch` 返回 `invalid-input`；新增 `TestAccentPatchRejectsUnknownFields`。
- 需改 4（DESIGN §3.1 强调色表仍是靛蓝）：改为橙色默认，补文字令牌与按钮令牌一族，并指向 §12。
- 建议项保留：`tokensFor` / `accentCssVars` 仅测试使用（保留作公共 API）；`--raw-accent-btn-active` 暂无消费者（沿用 M3 前按下态只做 transform）；自定义色失焦不提交（与原型一致）。三条均不改。
- 审查无法复现的部分：真实窗口的 e2e / 性能数字与 T09 安装包；修复后由本会话重跑全量 e2e 复核：137 项断言全过（ac-m0 15、ac-m1 16、ac-m2 22、ac-m3 47、ac-m35 37），EXIT=0；生产 1.1.0 构建随后重建（e2e 构建会覆盖同目录产物）。

## 6. 验证与验收

| 编号 | 给定条件 | 操作 | 预期结果 | 方式 |
| --- | --- | --- | --- | --- |
| V-ACC-GO-1 | 1.0.0 的设置文件（无 `accent`） | 读取 | `{preset:"orange"}`，文件不被改写 | Go |
| V-ACC-GO-2 | `accent` 是 `"blue"`（字符串）、`{preset:"pink"}`、`{preset:"custom",hex:"#fff"}` | 读取 | 都回落为橙，不报错 | Go |
| V-ACC-GO-3 | `custom` 的 `cache.dark` 缺一个键 | 读取 | 保留 `preset` 与 `hex`，丢掉 `cache` | Go |
| V-ACC-GO-4 | patch `{preset:"teal",hex:"#123456"}` | 保存 | 存 `{preset:"teal"}` | Go |
| V-ACC-GO-5 | patch `{preset:"custom",hex:"#FFFF00"}` | 保存 | 存小写 `#ffff00` | Go |
| V-ACC-GO-6 | patch 非法（未知预设、`hex` 格式错、`cache` 值非法、accent / cache 内的多余字段） | 保存 | `invalid-input`，已保存的值不变，同一 patch 里的 `theme` 也不生效 | Go |
| V-ACC-GO-7 | 带 `accent` 的文件（含 accent 内的未知键） | 用不认识 `accent` 的代码路径读写（模拟 1.0.0） | 写回后 `accent` 与其中的未知键原样保留 | Go |
| V-ACC-ALG-1 | 6 个预设 | 快照 | 与 DESIGN.md 12.3 逐值相同 | vitest |
| V-ACC-ALG-2 | 12 套配色 | 对比度 | 全部满足 DESIGN.md 12.3 的标准 | vitest |
| V-ACC-ALG-3 | `#ffff00`、`#000000`、`#ffffff`、`#808080` | `deriveAccent` 与色板 | 满足对比度标准；灰色有 `isNearGray` 标记，色板提示「颜色太接近灰色，选中态可能不明显」 | vitest |
| V-ACC-TOK-1 | 选「靛」 | 49 张 R1 截图 | 与 M3 截图逐像素一致；不一致的逐张说明 | 脚本 + 人工 |
| V-ACC-TOK-2 | 代码库 | 搜 `color:\s*var\(--(raw|color)-accent\)` | 0 处 | 脚本 |
| V-ACC-BOOT-1 | 设置为蓝 | 冷启动截第一帧 | 主按钮已是蓝 | e2e（AC-ACC-1 / 2） |
| V-ACC-BOOT-2 | 设置为自定义且有缓存 | 冷启动 | 第一帧是缓存的颜色 | e2e |
| V-ACC-BOOT-3 | M61 失败（注入） | 切换颜色 | 回到切换前的颜色并提示 | vitest |
| V-ACC-UI-1 | 色板 | 只用键盘选色、Esc 关闭 | 焦点回到色点；色块角色正确 | e2e + 人工 |
| V-ACC-UI-2 | 标题栏选绿 | 打开设置常规 | 「主题色」一行显示绿 | e2e |
| V-ACC-AC | AC-ACC-1–6 | 见 PRD 16.7 | 全部通过 | e2e + 脚本 + 人工 |
| V-ACC-REG | 全部 | `bun run test:e2e`、`test:go`、`test:web`、`typecheck`、`lint:go`、`gofmt -l .` | 全部通过 | 脚本 |
| V-ACC-PERF | 冷启动 | `perf.sh` 与 `perf:gate` | 不劣于 M3 实测 | 脚本 |

门禁按技术设计第 10 节；实际命令按 README 第 5 节；每个任务的实际结果记在第 9 节账本。

## 7. 协作、审查与执行方式

- 一次一个任务，汇报后停下；整份计划的批准不等于各任务放行。
- 不用子代理或并行会话执行编号任务；T10 的审查方式按 B-M35-02。
- 每个任务结束提交到本地（不推送）；推送只在 T11 列清单后由用户决定。

## 8. 来源基线与批准记录

| 项 | 内容 |
| --- | --- |
| 需求与设计基线 | PRD v2.3、DESIGN.md 第 12 节、UI 稿第 10 节、API 第 11 节、数据库设计 4.1.1、技术设计第 14 节，均为用户 2026-11-20 审核通过 |
| 代码基线 | 本地 `main` `ab36fc5`（`origin/main` 为 `9180180`，本地多 6 个文档与 spike 提交） |
| 本计划 | v0.1，用户 2026-11-20 批准（B-M35-01 选 a、B-M35-02 选 a） |

待用户在审批时决定：

| 编号 | 问题 | 选项 | 建议 |
| --- | --- | --- | --- |
| B-M35-01 | 版本号与安装包 | (a) 版本改 1.1.0，打本地安装包，在本机做「1.0.0 → 1.1.0 升级」冒烟（验证默认色从靛变橙、设置保留）；(b) 不改版本、不打包，留到 M4 结束 | **已确认（2026-11-20）：(a)**；安装动作执行前再单独确认 |
| B-M35-02 | T10 的审查方式 | (a) 用 task-reviewer 子代理做一次独立审查（只报告不修复）；(b) 自审，在报告里写明是自审 | **已确认（2026-11-20）：(a)**；这是本计划唯一使用子代理的地方 |

| 日期 | 事项 | 结论 |
| --- | --- | --- |
| 2026-11-20 | 计划 v0.1 获批 | 用户批准，B-M35-01 选 (a)、B-M35-02 选 (a)，放行 T00 |

## 9. 恢复入口与执行账本

新会话恢复时：读 CLAUDE.md 第 3 节、本计划第 4 节与本节、相关契约；核对 `git log` 与工作区；不重跑已验证的任务，不凭聊天记忆补进度。

| 日期 | 任务 | 结果 | 提交 | 验证 | 下一项与放行状态 |
| --- | --- | --- | --- | --- | --- |
| 2026-11-20 | 计划 | v0.1 获批 | — | 自查见第 10 节 | T00 已放行 |
| 2026-11-20 | T00 | 完成：方案成立，A-1–A-4 全过 | 见下 | RESULT.md；技术设计 14.3 | 用户已验收并放行 T01 |
| 2026-11-20 | T11 | 完成：README 状态行、文档导航、命令表（前端 217、Go 528、e2e 137、M3.5 验收与视觉两行、1.1.0 安装包）更新；CLAUDE.md 第 3 节补 M3.5 完成状态；docs/PLAN-M0.md 路线表 M3.5 改「已完成」并补状态行；本计划状态与账本收尾。**无代码改动** | 见下 | docs-only：UTF-8 无 BOM、LF 回读核对；提交列表见下 | 推送决定待用户 |
| 2026-11-20 | T10 | 完成：task-reviewer 独立只读审查（17 个提交）→ 阻断 0、需改 4、建议 3。4 项需改全部修复：近灰提示接入色板（DESIGN 12.4）、accent 内未知键读写保留、M61 拒绝 accent / cache 多余字段（API 11.2）、DESIGN §3.1 强调色表改橙色默认。修复后 Go 528、Vitest 217、tsc / bindings 通过；全量 e2e 137 项断言全过（15/16/22/47/37）；生产 1.1.0 构建已恢复 | 见下 | 审查报告、Go / Vitest / tsc、全量 e2e | 等待放行 T11 |
| 2026-11-20 | T09 | 完成：版本 1.1.0，生产构建产出 setup-1.1.0（6.7MB）与 exe（19.6MB）；本机 1.0.0 → 1.1.0 静默覆盖安装，安装目录 exe 版本核对为 1.1.0；升级冒烟（临时 APPDATA）保留深色主题与左栏 260、默认橙生效。冒烟期间一个非本会话启动的实例写入真实设置（新增默认 accent），数据完整；该实例被误结束，已在发布清单与本节记录 | 见下 | 安装包构建与版本核对、升级截图、临时设置文件 | 等待放行 T10 |
| 2026-11-20 | T08 | 完成：`capture-new.py` 加 `--accent`（兼容旧环境变量），靛下重跑 50 个状态全部成功；与 M3 截图比较，真实差异只有标题栏色点与设置常规页「主题色」行，其余为抗锯齿、运行态与 175% 缩放的重采样差异。真实界面 6 预设 × 浅 / 深总览 12 张 + 设置常规 1 张；技能浮层原型改用文字 / 按钮令牌，按默认橙重拍 18 张 | 见下 | `capture-new.py --accents`、`--accent indigo`、Playwright 重拍、逐张像素比较 | 等待放行 T09 |
| 2026-11-20 | T07 | 完成：新增 `scripts/e2e/ac-m35.sh`（全新 APPDATA 的默认橙与欢迎页主按钮；色板选蓝立即生效、设置页同步、重启保留；自定义 `#ffff00` 保存输入值与缓存并重启生效；`accent` 乱码回落橙、无报错、可重选绿）与 `scripts/e2e/accent-contrast.js`（6 预设 × 浅 / 深，72 个真实组件样本）。全量 `bun run test:e2e`：ac-m0 15、ac-m1 16、ac-m2 22、ac-m3 47、ac-m35 37，全部 fail=0；对比度最低 文字 5.26、按钮 4.51、填充 3.06；冷启动典型 918ms、大库 771ms（基线 910–1101 / 1093），`perf:gate` 通过。首轮全量在点蓝前偶发一次 `missing`，等 6 个色块就绪后复跑通过 | 见下 | 两轮全量 e2e、单跑 ac-m35、perf.sh + perf:gate | 等待放行 T08 |
| 2026-11-20 | T06 | 完成：常规「外观」新增「主题色」一行，与标题栏色板共用抽出的 `AccentOptions`（6 色块、自定义输入、「实际使用」提示），两处读同一个 store、选择即生效、不进保存条。3 项新测试（行内 6 色块与选中态、标题栏色点同步换色、自定义带缓存），全套 216 项通过；真机核对浅色 / 深色 / 自定义三态与色板回归，设置文件存下自定义色及其缓存 | 见下 | `tsc`、Vitest 216 项、e2e 构建 + 真机截图 | 等待放行 T07 |
| 2026-11-20 | T05 | 完成：标题栏色点（含 `aria-label`/`aria-expanded`/`aria-haspopup`）与色板浮层；6 个色块 radiogroup、方向键移动、Enter 选择、勾选色块用各自前景色；自定义输入带校验、「实际使用」提示与预览点；Esc/点外面关闭并还焦点。5 项组件测试、全套 213 项通过；真机截图核对浅色/深色/自定义三态，修了自定义行被挤成一列的问题 | 见下 | `tsc`、Vitest 213 项、e2e 构建 + 截图 | 等待放行 T06 |
| 2026-11-20 | T04 | 完成：Go 经 `Page.PreloadScript` 注入设置里的主题色；`theme-init.js` 首帧读值（预设设 `data-accent`，自定义写 20 个内联变量）；`appStore` 载入 / 设置变化时应用，切换走 `setAccent`：先换色、再 patch（自定义补算缓存），失败回滚并提示。实测：预设蓝注入 `blue`、计算值 `#2563eb`、截图 423 个蓝色像素；自定义黄 `data-accent=custom`、内联变量 `#999900`、按钮 `#7a7a00` | 见下 | `tsc`、Vitest 208 项；Go build/vet | 等待放行 T05 |
| 2026-11-20 | T03 | 完成：新增 accent.css（12 预设 + custom 映射）、令牌拆分、27 处文字改名、按钮组；靛下与「改动前同一份构建」逐像素比对：50 张里 26 张完全相同、橙色像素 0、橙靛互换 0，其余差异为时间文本与运行态（强度选择、最近文档顺序）。过程中修了两处：主题缩略图不再覆盖强调色；截图脚本注入后等一次重绘 | 见下 | 两次 e2e 构建 + 50 张对照；`build:web` | 等待放行 T04 |
| 2026-11-20 | T02 | 完成：`src/lib/accent.ts`（6 预设常量、`deriveAccent`、对比度函数、`tokensFor`、CSS 变量映射）+ 12 个单测；DESIGN.md 12.3 橙色浅色按钮三值按算法回写（`#b95b1e`→`#ba5b1e` 等，因为规则是「从 accent 继续加深」） | 见下 | `tsc`、Vitest 199 项（含新增 12 项） | 等待放行 T03 |
| 2026-11-20 | T01 | 完成：`accent` 类型、读入回落、Patch 严格校验；坏 accent 不再让整个设置文件被判损坏；Patch 改为「全部校验后再组装」 | 见下 | Go 526 项、vet/gofmt、绑定对拍、`tsc`、Vitest 187 项 | 等待放行 T02 |

## 10. 自查

- 每个任务都有修改范围、依赖和可验证的完成条件；T00 的结论可能改变 T04，已写明停下等决定。
- AC-ACC-1–6 都对应到了第 6 节的验证项（1、2→BOOT；3→T07 对比度脚本；4→ALG-3 与 e2e；5→GO-2 与 e2e；6→TOK-1）。
- 契约里每个新字段、每条回落规则都有对应测试（GO-1–7）。
- 涉及安装、推送、独立审查的动作都留了单独的确认点。
- 未覆盖：干净机器上的首帧表现（与 M3 相同，开发机测不出来）；`#808080` 这类接近灰色的自定义色只做提示、不拦截，体验由用户判断。
