# M3.5 实施计划：主题色

> 状态：v0.1 已获用户批准（2026-11-20），按「一次一个任务」执行；T00 已放行，其余任务待逐项放行。
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
| T03 | 令牌拆分与 12 套配色 | `src/styles/*.css` | T02 | 29 处改名、按钮与发送按钮改按钮令牌；默认橙；V-ACC-TOK 通过（含靛下 49 张逐像素比对） | 待做 |
| T04 | 首帧链路 | `main.go`、`public/theme-init.js`、`src/stores/appStore.ts` | T00、T01、T02、T03 | V-ACC-BOOT 通过 | 待做 |
| T05 | 标题栏色点与色板 | `src/components/TitleBar.tsx`、`src/components/AccentPicker.tsx`（新）、`src/styles/` | T04 | 交互与可访问性按 UI 稿 10.1、DESIGN.md 12；单测与构建通过 | 待做 |
| T06 | 设置页「主题色」一行 | `src/components/settings/GeneralSection.tsx` | T05 | 与色板同步；单测与构建通过 | 待做 |
| T07 | 验收与全量回归 | `scripts/e2e/ac-m35.sh`（新）、`scripts/e2e/accent-contrast.js`（新） | T06 | AC-ACC-1–6 与真实组件对比度检查通过；`bun run test:e2e` 全过；冷启动性能不劣于 M3 | 待做 |
| T08 | 视觉重拍 | `scripts/visual/capture-new.py`、`design/r2/shots/`、`design/r2/shots-accent/` | T07 | 靛下 R1 对照无回归；6 预设 × 浅 / 深真实界面截图；技能浮层 18 张按橙重拍 | 待做 |
| T09 | 版本与安装包（按 B-M35-01） | `mygo.config.ts`、docs/发布清单.md | T08 | 见 B-M35-01；安装动作执行前再单独确认 | 待做 |
| T10 | 整体审查（按 B-M35-02） | 审查报告、本计划第 6 节 | T09 | 阻断项为 0，需改项已修或经用户接受 | 待做 |
| T11 | 收尾 | README、CLAUDE.md、本计划、docs/PLAN-M0.md 路线 | T10 | 文档同步；列提交；推送前询问用户 | 待做 |

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

### T07 验收与全量回归

- `scripts/e2e/ac-m35.sh`：临时 APPDATA；AC-ACC-1、2、4、5 自动化；AC-ACC-6 交给 T08。
- `scripts/e2e/accent-contrast.js`：经 automation `eval` 在真实窗口里，对 6 预设 × 浅 / 深逐套切换，取主按钮、发送按钮、选中分段项、链接、选中树行、复选框的计算后颜色，按 DESIGN.md 12.3 的标准断言（AC-ACC-3）。
- 全量：`bun run test:e2e`（ac-m0..m3 加 ac-m35）退出码 0；`bash scripts/e2e/perf.sh` 冷启动与 M3 实测比较，结果走 `perf:gate`。

### T08 视觉重拍

- `capture-new.py` 加 `--accent <preset>`：在靛下重跑 49 张，与 M3 的截图逐张比较；有差异的列清单逐项说明。
- 真实界面 6 预设 × 浅 / 深各一张总览加一张设置常规页。
- 技能浮层原型改用新令牌（文字用 `--raw-accent-text`、按钮用按钮令牌），按默认橙重拍 18 张。

### T09–T11

按 B-M35-01、B-M35-02 的批准结果执行；T11 列出全部提交，推送前询问用户。

## 6. 验证与验收

| 编号 | 给定条件 | 操作 | 预期结果 | 方式 |
| --- | --- | --- | --- | --- |
| V-ACC-GO-1 | 1.0.0 的设置文件（无 `accent`） | 读取 | `{preset:"orange"}`，文件不被改写 | Go |
| V-ACC-GO-2 | `accent` 是 `"blue"`（字符串）、`{preset:"pink"}`、`{preset:"custom",hex:"#fff"}` | 读取 | 都回落为橙，不报错 | Go |
| V-ACC-GO-3 | `custom` 的 `cache.dark` 缺一个键 | 读取 | 保留 `preset` 与 `hex`，丢掉 `cache` | Go |
| V-ACC-GO-4 | patch `{preset:"teal",hex:"#123456"}` | 保存 | 存 `{preset:"teal"}` | Go |
| V-ACC-GO-5 | patch `{preset:"custom",hex:"#FFFF00"}` | 保存 | 存小写 `#ffff00` | Go |
| V-ACC-GO-6 | patch 非法（未知预设、`hex` 格式错、`cache` 值非法） | 保存 | `invalid-input`，已保存的值不变，同一 patch 里的 `theme` 也不生效 | Go |
| V-ACC-GO-7 | 带 `accent` 的文件 | 用不认识 `accent` 的代码路径读写（模拟 1.0.0） | 写回后 `accent` 原样保留 | Go |
| V-ACC-ALG-1 | 6 个预设 | 快照 | 与 DESIGN.md 12.3 逐值相同 | vitest |
| V-ACC-ALG-2 | 12 套配色 | 对比度 | 全部满足 DESIGN.md 12.3 的标准 | vitest |
| V-ACC-ALG-3 | `#ffff00`、`#000000`、`#ffffff`、`#808080` | `deriveAccent` | 满足对比度标准；灰色有标记 | vitest |
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
| 2026-11-20 | T02 | 完成：`src/lib/accent.ts`（6 预设常量、`deriveAccent`、对比度函数、`tokensFor`、CSS 变量映射）+ 12 个单测；DESIGN.md 12.3 橙色浅色按钮三值按算法回写（`#b95b1e`→`#ba5b1e` 等，因为规则是「从 accent 继续加深」） | 见下 | `tsc`、Vitest 199 项（含新增 12 项） | 等待放行 T03 |
| 2026-11-20 | T01 | 完成：`accent` 类型、读入回落、Patch 严格校验；坏 accent 不再让整个设置文件被判损坏；Patch 改为「全部校验后再组装」 | 见下 | Go 526 项、vet/gofmt、绑定对拍、`tsc`、Vitest 187 项 | 等待放行 T02 |

## 10. 自查

- 每个任务都有修改范围、依赖和可验证的完成条件；T00 的结论可能改变 T04，已写明停下等决定。
- AC-ACC-1–6 都对应到了第 6 节的验证项（1、2→BOOT；3→T07 对比度脚本；4→ALG-3 与 e2e；5→GO-2 与 e2e；6→TOK-1）。
- 契约里每个新字段、每条回落规则都有对应测试（GO-1–7）。
- 涉及安装、推送、独立审查的动作都留了单独的确认点。
- 未覆盖：干净机器上的首帧表现（与 M3 相同，开发机测不出来）；`#808080` 这类接近灰色的自定义色只做提示、不拦截，体验由用户判断。
