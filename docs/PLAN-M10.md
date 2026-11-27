# M10 实施计划：R2 全局材质与动效（技能页风格推广到全应用，档位可调）

> 状态：v0.1 已批准（2026-11-27，用户按建议批准，含 B-M10-00–05、Q-M10-1–3）；B-M10-00 与 T00 已放行，其余任务一次一个、逐项放行。
> 本次目标：交付 PRD 第 19 节 FR-UI-MATERIAL（v2.9）。闭环为「升级到 1.8.0 后全应用是新风格，文字密集区仍是不透明底 → 在设置 → 常规 → 外观把模糊、动画分别切到关 / 强，全应用即时变化 → 重启后首帧就是所选档位 → 默认档下流式输出、打字与滚动指标不退化，掉帧时自动关闭模糊并提示」。
> 上一里程碑：docs/PLAN-M9.md（已完成并推送，`7307aad`，1.7.0）。下一里程碑：未规划。

## 1. 目标、范围与授权

目标：

1. 设置字段 `effects`（动画 / 模糊各四档，默认标准），首帧注入，即时生效（技术设计 21.2）。
2. 全局令牌与材质：`effects.css`、三档材质与不透明内衬、窗口级背景色场；技能页改用全局令牌（DESIGN 第 15 节、技术设计 21.3–21.5）。
3. 全应用改造：标题栏、工作台三栏、总览、运行参数与输入框、浮层与对话框、设置页、欢迎页与引导（UI 稿第 13 节）。
4. 全局自动降级与系统减少动画的提示（技术设计 21.6、21.7）。
5. 验收：AC-MAT-1–10；对比度；真窗口视觉；性能门禁（含新增 `kind=m10`）。

不做：任何布局、尺寸、交互逻辑、快捷键与功能规则的改变；标题栏快捷入口（Q-MAT-5）；新增字体与依赖；编辑器内部、虚拟列表、流式消息的动画。

授权边界：测试、e2e 与截图只用临时 APPDATA、临时知识库与假 HOME；不读写真实知识库与真实应用数据；Agent 权限不变。

## 2. 来源基线与决策点

| 项 | 内容 |
| --- | --- |
| 需求基线 | PRD 第 19 节 FR-UI-MATERIAL、AC-MAT-1–10、Q-MAT-1–7（v2.9，用户 2026-11-27 审核通过）；D-021 |
| 设计基线 | docs/UI设计稿.md 第 13 节（2026-11-27 审核通过）；原型 `design/r2/global-material.html` 与 `design/r2/shots-m10/` 19 张；DESIGN.md 第 15 节与 §2 注记（草稿，随本计划审核） |
| 技术基线 | 技术设计第 21 节；API.md 第 17 节；数据库设计 4.1 的 `effects.motion` / `effects.blur`（草稿，随本计划审核） |
| 复用的既有实现 | 主题色首帧链路（`main.go` `PreloadScript` + `public/theme-init.js`）；技能页试点令牌、材质与动效（`skills-page.css`）；`lib/skillsGlass.ts` 的分位数函数；`accent-contrast.js`；`perf.sh` / `perf-ai.sh` / `perf-m8.sh` |
| 代码基线 | `origin/main` `7307aad`（1.7.0）+ 本地未提交的 M9 收尾文档修正与 M10 规划资料（开工前先提交，见 B-M10-00） |
| 已定 | Q-MAT-1–7 按建议 (a)（用户 2026-11-27）；M10 发 1.8.0 |

以下待用户审批时确认（建议列在最后一列）：

| 编号 | 问题 | 选项 | 建议 |
| --- | --- | --- | --- |
| B-M10-00 | 开工前的工作区 | (a) 先把 M9 收尾文档修正、M10 规划资料（含原型、截图、`capture-proto-m10.py`）提交到本地（不推送）；(b) 带着未提交改动开工 | (a) |
| Q-M10-1 | 设置字段形状 | (a) 顶层对象 `effects: {motion, blur}`；(b) 两个顶层字符串 | (a)：一处读写，降级到 1.7.x 时整体保留 |
| Q-M10-2 | 三档模糊在真窗口拉不开时 | (a) T00 实测后按结果调 DESIGN 15.2 数值，先报告再改；(b) 沿用原型数值 | (a) |
| Q-M10-3 | 新门禁 `idleCpuRatio`（最小化时 CPU ≤ 1.7.0 基线 × 1.2） | (a) T00 测基线后定阈值，经用户确认接入；(b) 不设门禁，只记录 | (a) |
| B-M10-02 | 令牌检查改写（`check-skill-tokens.mjs` → `check-effect-tokens.mjs`，技术设计 21.3） | (a) 按 21.3 改写并做正反对照；(b) 保留旧检查 | (a)：旧检查与全局令牌冲突，改写后规则更严（禁止写死模糊值与长时长） |
| B-M10-03 | perf-m8 的 R 取法改为「空闲与滚动中位数的较小值 + 33.4 ms 绝对上限」（补 M9 复核发现的盲点） | (a) 放进 T09，改动前列影响与正反对照；(b) 不改 | (a) |
| B-M10-01 | 版本与安装 | (a) 完成后发 **1.8.0**，打本地安装包并做 1.7.0 → 1.8.0 升级冒烟（安装前单独确认）；(b) 只构建不安装 | (a) |
| B-M10-04 | 真窗口人工核对 | (a) T10 截图后请用户亲测各档位与工作台高频操作；(b) 只看截图 | (a) |
| B-M10-05 | 独立审查 | (a) T11 用 task-reviewer 审查 M10 全部改动（重点：首帧、档位令牌覆盖面、降级判定、对比度、性能）；(b) 自审 | (a) |

## 3. 任务清单

状态取值：待做、进行中、阻塞、待验证、已验证。

| 编号 | 目标 | 修改范围 | 依赖 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- |
| T00 | 基线与真窗口 spike | `spikes/m10-effects/`（新，不进产品代码） | B-M10-00 | 1.7.0 最小化 5 分钟 CPU 基线；真窗口里三档模糊的观感与玻璃底对比度取样；`kind=typical` 在 1.7.0 上的实测基线；结论写进 RESULT.md 并回报（Q-M10-2、Q-M10-3） | 已验证（2026-11-27；Q-M10-2 采用候选 E、Q-M10-3 按建议接入，见账本） |
| T01 | 设置字段与首帧 | `internal/contract`、`internal/settings`、`main.go`、`public/theme-init.js`、`appStore`、`src/mygo.ts`（生成） | T00 | Go 测试先红后绿（默认、非法回落、`patch` 拒绝、顶层保留）；`theme-init` 三种输入的单测；`check:bindings` 通过 | 已验证（2026-11-27） |
| T02 | 全局令牌与检查改写 | `src/styles/effects.css`（新）、`main.css` 引入、`skills-page.css` 去掉自有令牌、`scripts/check-effect-tokens.mjs`（替换旧检查，B-M10-02） | T01 | 检查正反对照；技能页在 `std` 档下与 1.7.0 截图逐张对照无差异（色场位置除外）；ac-m8 / ac-m9 通过 | 已验证（2026-11-27；同环境计算样式与像素对拍一致，见账本） |
| T03 | 色场、降级与系统减少动画 | `BackgroundField`（新）、`useEffectsGuard`（新，替代 `useSkillsGlass`）、`appStore` 状态 | T02 | 判定函数单测（比值、绝对上限、`off` 不判定）；不可见时暂停；技能页改用全局结果 | 已验证（跳过亲测） |
| T04 | 设置页两行与提示 | `GeneralSection.tsx`、样式 | T01、T03 | 两组四段即时生效；两种提示；组件测试 | 已验证（待亲测） |
| T05 | 标题栏与工作台三栏 | `TitleBar`、`main.css`、`sidebar.css`、`chat.css`、`editor.css` | T02 | 外壳玻璃、文字密集区 `.sheet`；布局与尺寸不变（ac-m0 / ac-m1 通过）；截图对照 | 已验证（亲测延后） |
| T06 | 卡片、浮层与对话框 | `overview.css`、运行参数与输入框、`tokens.css` 原 `--glass` 处、`mutation.css`、`health.css`、`search.css`、确认卡 | T05 | 旧 `--glass` 令牌删除且零引用；ac-m1 / ac-m2 / ac-m3 通过 | 待做 |
| T07 | 设置页、欢迎页与引导 | `settings.css`、`welcome.css` | T04、T06 | ac-m3 / ac-m35 通过；截图对照 | 待做 |
| T08 | e2e | `scripts/e2e/ac-m10.sh`（新）；受外观影响的既有断言 | T03–T07 | AC-MAT-1–4、8 自动化；全量 e2e EXIT=0（AC-MAT-10）；改动的既有断言逐条说明原因 | 待做 |
| T09 | 性能 | `perf.sh` / `perf-ai.sh` 加档位参数、`perf-m10.sh`（新）、`perf-gate.py`（`kind=m10`）、`perf-m8.sh`（B-M10-03，改前确认） | T08 | `typical` 在 `std` / `high` 达标；切档 ≤ 100 ms；最小化 CPU 达标（Q-M10-3 定阈值后）；技能页 m8 / m9 不退化 | 待做 |
| T10 | 对比度、视觉与人工核对 | `accent-contrast.js` 扩展、`scripts/visual/capture-m10.py`（新）、重拍 m8 / m9 基线、UI 稿 13.x 差异 | T09 | 6 套主题色 × 浅 / 深 × 模糊三档 ≥ 4.5；真窗口截图与原型对照；按 B-M10-04 请用户亲测 | 待做 |
| T11 | 独立审查 | 审查报告、本计划、受影响代码与测试 | T10 | 阻断 0，需改项已修或经用户接受 | 待做 |
| T12 | 收尾 | 版本 1.8.0、README、CLAUDE.md、路线、发布清单（含「降级到 1.7.x 时 `effects` 保留」说明）、本计划 | T11 | 安装包与 1.7.0 → 1.8.0 升级冒烟（安装前确认）；文档同步；列提交；推送前询问 | 待做 |

## 4. 单任务精确交接（摘要）

- T01：`contract.EffectsSetting{Motion, Blur}`；`AppSettings.Effects`、`SettingsPatch.Effects *EffectsSetting`；`settings.NormalizeEffects`；`main.go` 预载脚本追加 `window.__MKB_EFFECTS__ = <json>;`；`theme-init.js` 只接受四个值，写 `document.documentElement.dataset.blur / .motion`。
- T02：`effects.css` 的选择器只有 `:root`、`[data-theme='dark']`、`[data-blur=…]`、`[data-motion=…]`、`[data-blur-auto='off']` 与 `@media (prefers-reduced-motion: reduce)`；工具类 `.glass-panel`、`.glass-card`、`.glass-strong`、`.sheet`、`.reveal`。
- T03：`useEffectsGuard()` 返回 `void`，副作用是写 `data-blur-auto` 与 `appStore.effectsDegraded`；纯函数 `shouldDegradeGlobal(gaps, idleMedian)`。
- T04：设置行 `data-testid="effects-motion"` / `effects-blur"`，提示 `effects-motion-note` / `effects-blur-note`。

## 5. 验证与验收

| 验收 | 方式 | 任务 |
| --- | --- | --- |
| AC-MAT-1（材质分布） | ac-m10：关键元素的计算样式（`backdrop-filter`、背景透明度）断言 + 截图 | T05–T08 |
| AC-MAT-2 / 3（档位即时生效、重启首帧） | ac-m10：切档后读计算样式；重启后在第一帧读 `<html>` 属性 | T01、T04、T08 |
| AC-MAT-4（系统减少动画） | ac-m10：`--force-prefers-reduced-motion` | T03、T08 |
| AC-MAT-5（对比度） | `accent-contrast.js` 扩展 | T10 |
| AC-MAT-6 / 7（性能） | `perf.sh` / `perf-ai.sh` 两档 | T09 |
| AC-MAT-8（自动降级） | ac-m10：注入慢帧（e2e 专用钩子）后断言 `data-blur-auto` 与设置提示 | T03、T08 |
| AC-MAT-9（最小化资源） | `perf-m10.sh` | T09 |
| AC-MAT-10（既有验收） | 全量 e2e | T08 |

每个任务结束：`tsc`、`test:web`（Node ≥ 22.19）、`test:go`、`lint:go`、`gofmt`；涉及界面的任务跑受影响的 ac 脚本；T08 起每个任务跑全量 e2e。

## 6. 性能目标

见技术设计 21.8：`typical` 现有阈值在 `std` 与 `high` 两档不变；`m8` / `m9` 不退化；新增 `kind=m10`（`effectsSwitchMs` ≤ 100、`idleCpuRatio` 阈值 T00 后定）。超目标先报告再决定，不降阈值。

## 7. 协作、审查与执行方式

- 一次一个任务，汇报后停下；整份计划的批准不等于各任务放行。
- 不用子代理或并行会话执行编号任务；T11 的审查方式按 B-M10-05。
- 每个任务结束提交到本地（不推送）；推送只在 T12 列清单后由用户决定。
- 安装包安装（B-M10-01）、门禁改写（B-M10-02、B-M10-03、Q-M10-3）、人工亲测分别单独确认。
- 全量 e2e 必须带 `-tags=e2e` 的构建；正式包用不带标签的 `bun run build` 重新打。跑 e2e / perf / 截图前确认没有运行中的应用实例。
- Vitest 需要 Node ≥ 22.19（本机 nvm 当前是 20.19.0，只在命令内临时用 22.23.4，不改全局）。

## 8. 恢复入口与执行账本

新会话恢复时：读 CLAUDE.md 第 3 节、本计划第 3、8 节、PRD 第 19 节、技术设计第 21 节、DESIGN 第 15 节；核对 `git log` 与工作区；不重跑已验证的任务，不凭聊天记忆补进度。

| 日期 | 任务 | 结果 | 提交 | 验证 | 下一项与放行状态 |
| --- | --- | --- | --- | --- | --- |
| 2026-11-27 | 计划 | v0.1 草案；PRD 第 19 节 v2.9 与 UI 稿第 13 节已审核通过；DESIGN 第 15 节与 §2 注记、技术设计第 21 节、API 第 17 节、数据库设计 4.1 两行为草稿，随本计划审核；路线 3.9.3 | 未提交 | 现状盘点（技术设计 21.1，逐项附文件与行号）；文档 UTF-8 无 BOM、无乱码 | 待用户审批（含 B-M10-00–05、Q-M10-1–3） |
| 2026-11-27 | 审批 | 用户按建议批准 PLAN-M10 v0.1 与配套资料（DESIGN 第 15 节与 §2 注记、技术设计第 21 节、API 第 17 节、数据库设计 4.1 两行），含 B-M10-00–05、Q-M10-1–3 均按建议 (a)；放行 B-M10-00 与 T00；各文档状态行已回写 | 随 B-M10-00 第 2 次提交 | 状态行 grep 核对 | B-M10-00 → T00 |
| 2026-11-27 | B-M10-00 | 审批结论回写：PLAN-M10 状态行与账本、DESIGN §2 注记与 §15、技术设计 §21、API §17、数据库设计 4.1 两行、decisions D-021、路线 3.9.3、CLAUDE.md 第 3 节；分两次本地提交：`7a5973d`（M9 收尾文档修正：PLAN-M9 头部、推送事实与授权记录）、`cfeee6f`（M10 规划资料 + 原型 + 19 张截图 + 截图脚本，32 个文件）。3 个非本任务文件（`docs/agent-skills/`、`docs/icon/`、`docs/内置Agent技能.md`）未提交，仍待用户决定 | `7a5973d`、`cfeee6f` | 提交前逐个看 git diff，无凭据与个人信息；python 回读 12 个文档 UTF-8 无 BOM、无 U+FFFD；`git diff --check` 无输出；`shots-m10` 19 张；提交后工作区只剩 3 个用户文件 | T00（已放行；需先关闭运行中的应用实例） |
| 2026-11-27 | T00 | 新增 `spikes/m10-effects/`（不进产品代码）。**CPU 基线**（生产 1.7.0、临时 APPDATA / LOCALAPPDATA / USERPROFILE / HOME、临时 typical 库、300 s 最小化）两轮 **0.594 / 0.625 s**（单核 0.20%，7 个进程 = 应用 + 6 个 msedgewebview2），真实 `%APPDATA%\Mile Knowledge Base` 文件指纹前后完全一致；**typical 基线**（e2e 构建）：`perf.sh` 冷启动 870 ms、`perf-m1.sh` 检索 p95 1.2 ms、`perf-ai.sh` 历史首帧 66.1 ms / 流式慢帧 0（551 帧）/ 按键 p90 8.1 ms，`perf:gate --kind typical` **exit 0**；**真窗口模糊 spike**：已审核原型 + `tokens.css` 在 mygo + WebView2 打开（1440×900，dpr 1.751，motion=off），2 状态 × 2 主题 × 4 档共 16 组截图，文字透明化后按包围盒取中位背景算 WCAG，最低 **4.53**（深色菜单强调色，与 M9 一致）。**Q-M10-2 结论**：当前「标准」与「强」在真窗口不可区分（std→high MAD 0.16–1.70、差异 >8 的像素 **0.00%**；根因：原型把 `--field-k` 直接用作 `.field{opacity}`，1.3 被钳到 1，属空操作；模糊半径在柔和色场上无感）。实测候选 B–E，推荐 **E**（色块浅 60/40/28、深 36/32/17；面板浅 34/28、强 70，深 30/24、强 68；40px、饱和 2.0）：work-doc 浅 MAD **1.82 / 6.15%** 像素（≈ off→std 的可见度），菜单 5.95 / 25.14%，对比度最低仍 4.53。DESIGN 15.2/15.3 按 Q-M10-2「先报告再改」**待用户确认**。**Q-M10-3 建议**：`idleCpuRatio = idleCpuSeconds / 0.625 ≤ 1.2`（300 s 内 ≤ 0.75 s），待确认后接入 T09。另：清理了 PowerShell 5.1 在仓库根生成的 `Microsoft/` 缓存目录（20K，非项目内容）。 | 本地（随 T00 提交） | CPU 两轮 JSON 与真实 APPDATA 指纹；gate exit 0；16 张当前档位 + 4 张候选 E 截图、`result.json` / `tune-result.json`、三份 perf 原始输出（`.txt`，`*.log` 被忽略）；19 个文本文件 UTF-8 无 BOM、无 U+FFFD；e2e 构建覆盖 `build/windows-amd64`（正式包 T12 重打） | T01 未放行；Q-M10-2 数值与 Q-M10-3 门槛待用户确认 |
| 2026-11-27 | 裁决（T00 跟进） | 用户 2026-11-27 答复：**Q-M10-2 采用候选 E**（DESIGN 15.2/15.3、UI 稿 13.3/13.5、原型高强档与 2 张 high 截图同步）；**Q-M10-3 按建议接入**（基线 0.625 s、≤1.2 倍，T09 写入门禁）；**T00 亲测跳过**，放行 T01 | 随本提交 | `capture-proto-m10.py` 重拍 19 张，仅 work-doc-blur-high 浅 / 深变化（其余逐字节一致）；DESIGN / UI 稿 UTF-8 无 BOM | T01（已放行） |
| 2026-11-27 | T01 | `contract.EffectsSetting{Motion,Blur}`（`AppSettings.Effects`、`SettingsPatch.Effects`、`HasUnknownEffectsFields`）；`settings.Defaults` = std/std、`NormalizeEffects`（缺失 / 非法 → std）、`Patch` 校验（四值之一、两个字段都要给、`effects` 内多余键 invalid-input、校验完才组装）、`load` 对 `effects` 宽松解析（类型不对不判文件损坏，嵌套未知键按 4.1 丢弃）、`saveLocked` 写 `effects`；`main.go` 在 accent 之后追加 `window.__MKB_EFFECTS__`（同一 PreloadScript，改名 `appearancePreload`）；`theme-init.js` 只写四个合法值、非法与缺失不写；`appStore.setEffects`（先改 `<html>`、串行队列、失败回滚并提示）、启动与 settings-changed 时同步属性；新增 `src/lib/effects.ts`；`mygo generate` 更新绑定。测试：Go 新增 14 项（含子测试；`effects_test.go`：默认、五种加载回退、patch 校验含 4 个子用例、顶层未知保留、嵌套未知丢弃）；Vitest 新增 12 项（theme-init 5、effects 3、appStore 4）；6 个既有设置夹具补 `effects` 字段（类型必填，断言未改） | 本地（随 T01 提交） | 先红：`effects_test.go` 因 `Effects` / `EffectsSetting` 不存在编译失败；后绿：Go **742** 项（基线 728 + 14）、Vitest **56 文件 442 项**（基线 430 + 12）、`tsc` 净、`go vet` / `gofmt -l .` 无输出、`check:bindings` 通过。未跑 e2e：本任务不改界面，按计划 T08 起每任务全量 | T02 未放行 |
| 2026-11-27 | T02 | 新增 `src/styles/effects.css`：DESIGN 15.2 档位令牌（std = 技能页现值；low / high / off 与深色覆盖；`--field-k` 上限 1，high 用候选 E 色块）、动画档令牌与 `[data-motion='off']` / `prefers-reduced-motion`（保留 `.kb-spin` 转圈）、材质工具类 `.glass-panel` / `.glass-card` / `.glass-strong` / `.sheet` / `.reveal`、窗口级色场样式（T03 接入）；`main.css` 在 tokens 之后引入。技能页删除自有令牌定义与深色覆盖、改用全局令牌；写死的 saturate / blur / 时长改走令牌（22px、24px、8px、6px 各自成令牌；260 / 420ms → `--dur-hover` / `--dur-slide`；`skills-fade` → `--dur-enter`）；`skills-shared.css` 与 `page-switch.css` 同步；`tokens.css` 的 `.popover` 提前改用强档令牌（原属 T06，为新检查通过的前置）。`scripts/check-effect-tokens.mjs` 替换旧检查（① 档位令牌只能在 effects.css 定义；② 禁写死 `backdrop-filter: blur(<数值>)` 与 >240ms 时长；`@supports` 与注释豁免；`--self-test` 11 例），旧脚本删除，`package.json` 与 README 同步。**正反对照**：树内 ok（118 定义、133 引用）；临时反例文件三规则全部报错（已删）；self-test 11/11。**技能页 std 一致性**：1.7.0 与当前构建在同环境（1.75 DPI）各跑一次 `spikes/m10-effects/parity/`：计算样式 JSON 唯一差异是新增 `data-blur` / `data-motion` = std；冻结动画并归零滚动后截图 SHA256 **完全相同**（`97a745d3d6378c73a3282943fc29b528069ace24c9593ce19f1a02982406db04`）。仓库里的 1.7.0 截图基线是 1x DPI、当前机器 1.75x，且 `capture-m9.py` 自身重拍两次在滚动 / 深色状态就差 20–33%，跨 DPI / 跨次像素对照不可用，故采用同环境对拍，如实记录。 | 本地（随 T02 提交） | e2e 构建通过；**ac-m8 58/58、ac-m9 107/107**；`tsc` / `vet` / `gofmt` 净；Vitest 56 文件 442 项；`check:effect-tokens` 与 `check:bindings` 通过。未跑全量 e2e（按计划 T08 起每任务全量） | T03 未放行 |
| 2026-11-27 | T03 | **新增**：`src/lib/effectsGuard.ts`（`shouldDegradeGlobal(scrollGaps, idleMedian)`：R = min(空闲中位, 滚动中位)，p95 > 2R 且慢帧占比 > 5% 或 p95 > 33.4 ms 绝对线即降级）；`src/lib/useEffectsGuard.ts`（`useEffectsGuard()` 空闲 30 帧 → 首次滚动 120 帧，`blur` 为 `off` 或已降级时不判定，页面隐藏丢弃半截样本，降级写 `<html data-blur-auto="off">` 与 `appStore.effectsDegraded`，本次运行只判一次；`useSystemReducedMotion()` 把 `prefers-reduced-motion` 写入 `appStore.systemReducedMotion`）；`src/components/BackgroundField.tsx`（窗口级色场，`blur` 为 `off` 或已降级时不渲染，随 `visibilityState` 在 `<html>` 上维护 `data-field-paused`）；`appStore` 新增 `effectsDegraded`、`systemReducedMotion` 与两个 setter；`App.tsx` 挂载三者，`<BackgroundField />` 为 `.app-shell` 首个子节点。**技能页收口**：`SkillsPage` 去掉 `useSkillsGlass`、`data-glass`、页内 `.skills-field`；删除 `useSkillsGlass.ts`、`skillsGlass.ts`、`skillsGlass.test.ts`（旧边界用例已在 `effectsGuard.test.ts` 重写覆盖）；`skills-page.css` 面板底改 `--glass-panel`（**面板自身不加 backdrop-filter**：它会成为 backdrop root，使里面的卡片看不到背后色场），删除页内色场与三组漂移关键帧，`[data-glass='off']` 全部改为 `:is([data-blur='off'], [data-blur-auto='off']) .skills-page …`（含面板自身的内顶边高光，真窗口探测发现降级时仍残留，已补）。**分层**（让色场落在窗口级而技能页透明）：`.app-shell` 加 `isolation: isolate`；色场 `position:absolute; top:var(--titlebar-h); z-index:-1`；技能 host 去掉不透明底；隐藏的工作台由「被不透明页盖住」改为 `opacity:0; pointer-events:none`（仍保持布局与滚动状态）。`effects.css`：motion 为 `low` / `off` 与系统减少动画时色场漂移停止。**TDD**：先写 `effectsGuard` / `useEffectsGuard` / `BackgroundField` / `App`「Global colour field」4 例，红灯原因均为目标模块与行为缺失，再最小实现转绿；CSS 分层与样式迁移属纯样式工作，按计划例外用真窗口计算样式与截图验证，不套 TDD。**改动的既有断言（逐条）**：① `SkillsPage.test.tsx`「keeps the glass on by default and mounts the colour field」→「leaves the colour field and the degrade decision to the app shell」：原因是色场与降级判定上移到应用壳，替代断言为页面内无 `data-glass`、无 `.skills-field`、无 `.background-field`（页面不再自带，全局色场由 App 测试覆盖）；② `ac-m8.sh`「page covers the workbench opaquely」→ 两条：「hidden workbench paints nothing and takes no input」（`opacity 0`、`pointer-events none`、屏幕中心命中不在工作台内）与「skills host lets the window colour field through」（host 背景透明、`[data-testid=background-field]` 的 `z-index -1` 且顶边 ≥ 40）：原因是技能 host 按 M10 设计改为透明，原「host 背景不透明」断言与新架构矛盾，「点不到工作台」的意图保留并加强；③ `ac-m8.sh`「field stops drifting」选择器 `.skills-field span` → `.background-field span`，断言内容不变。**真窗口探测**（`spikes/m10-effects/t03/`，临时 APPDATA / 库 / HOME；`state-*.json` + `shot-*.png`）：浅 / 深色 std 下色场矩形 top=40、`z-index -1`、`opacity 1`，面板 `rgba(255,255,255,0.58)` 且 `backdrop-filter: none`，卡片 `blur(18px) saturate(1.5)`，隐藏工作台 `opacity 0 / pointer-events none`，屏幕中心命中 `kb-card-top`；设 `data-blur-auto=off` 后色场 `display:none`、面板与卡片变不透明、无 blur、无阴影。真实慢帧触发自动降级与设置页提示由 T08（AC-MAT-8）验证，本任务只验证判定函数、钩子与降级后的呈现。 | 本地（随 T03 提交） | `tsc` 净；`bun run test:web`：check-bindings、check-effect-tokens（118 定义 / 132 引用，self-test 11/11）、**Vitest 58 文件 471 项**全过；`go vet` 净、`gofmt -l` 无输出、Go 742 项通过；e2e 构建后 **ac-m8 59/59、ac-m9 107/107、ac-m0 15/15、ac-m1 16/16**（证据在 `spikes/m10-effects/t03/ac/`，CSS 补丁后 ac-m8 重跑 59/59）。**未验证**：全量 e2e（按计划 T08 起每任务全量）；真实慢帧降级（T08）；`perf-m8.sh` / `perf-m9.sh` 里仍有一行只读 `dataset.glass` 的信息性打印（已无此属性，输出为 null，不参与门禁），随 T09 一并处理，本任务不碰性能脚本；工作台三栏与标题栏仍不透明，色场只在技能页、窗口缝隙与标题栏下方可见，属 T05 范围。**亲测**：用户 2026-11-27 明确答复「不需要」并放行 T04，记为**跳过亲测** | T04 已放行（用户 2026-11-27） |
| 2026-11-27 | T04 | **设置页**：`GeneralSection` 的「外观」组在「主题色」后新增「动画」「模糊」两行（组件内 `EffectLevelRow`），各为一组四段单选（关 / 弱 / 标准 / 强，`role=radiogroup`，roving tabindex，方向键 / Home / End 与「点关闭按钮时」同一套 `nextOption`），点击或按键即写 `setEffects`，再点已选档位不写；`data-testid`：行 `effects-motion` / `effects-blur`，档位 `effects-<kind>-<off|low|std|high>`，提示 `effects-motion-note` / `effects-blur-note`。**提示**：`systemReducedMotion` 为真时动画行下出现信息色胶囊「系统已关闭动画效果，当前按「关」显示」；`effectsDegraded` 为真时模糊行下出现警示色胶囊「当前设备滚动掉帧，已自动关闭模糊（本次运行）」；两条提示都不让控件变灰，档位仍显示用户的选择、仍可修改（UI 稿 13.4）。`SettingRow` 新增可选 `note` 属性（复用既有 `.pill`，`role=status`），`tokens.css` 新增 `.setting-row-note` 上边距 6px；外观组说明文字补「动画与模糊改完立即生效，所有页面一起变」。**顺带修复（T01 遗留）**：`setEffects` 此前只处理 `result.failure`，传输层抛错（如 `TransportError`）会变成未处理的 Promise 拒绝且不回滚；现与 `setAccent` 一致，`try/catch` 回滚到原档位并弹出错误提示。这是 T04 首次从界面调用 `setEffects` 才暴露的缺口，故在本任务内先写红灯用例再修。**TDD**：先写 `GeneralSection.test.tsx` 新增 8 例与 `appStore.effects.test.ts` 新增 1 例，红灯原因分别是界面行缺失和传输层异常直接抛出；再最小实现转绿。**既有测试改动**：`GeneralSection.test.tsx` 的 `settingsWith` 增加可选 `effects` 参数（默认仍为 std / std，原 3 例断言不变）；`appStore.effects.test.ts` 的传输层 mock `TransportError` 改为与真实类一致的 `(code, message)` 构造签名，原 3 例断言不变。**真窗口探测**（`spikes/m10-effects/t04/run-t04.sh`，临时 APPDATA / 知识库 / HOME，读写落在临时目录）：默认 std / std；点「动画 关」后 `<html data-motion>` = off，临时 `settings.json` 的 `effects` = `{motion: off, blur: std}`；再点「模糊 弱」= `{off, low}`；改回标准 = `{std, std}`；注入两个状态后两条提示文字与上面一致，档位仍是「标准」；点「模糊 关」后 `.background-field` 消失、落盘 `{std, off}`。浅色与深色截图在同目录。探测中的 Python 解析问题是我脚本的编码问题（`python -I` 忽略 `PYTHONUTF8`，改用 `-X utf8`），与产品无关。 | 本地（随 T04 提交） | `tsc` 净；`bun run test:web`：check-bindings、check-effect-tokens（118 定义 / 132 引用，self-test 11/11）、**Vitest 58 文件 480 项**全过；`go vet` 净、`gofmt -l` 无输出、Go 742 项通过；e2e 构建后 **ac-m3 47/47、ac-m35 37/37**（设置页相关，证据在 `spikes/m10-effects/t04/ac/`）。**未验证**：全量 e2e（按计划 T08 起每任务全量）；真实慢帧与系统「减少动画」开关触发的提示（提示由 store 状态驱动，本任务用注入状态验证了呈现；触发链路由 T03 单测和 T08 的 AC-MAT-4 / 8 覆盖）；设置页自身仍是不透明实底，毛玻璃改造在 T07；提示胶囊的对比度在 T10 统一核对。**亲测**：用户 2026-11-27 答复「等全部执行完毕之后再做亲测」，记为**亲测延后**（不是跳过），全部任务完成后与 T05–T11 一起亲测 | T05 已放行（用户 2026-11-27） |
| 2026-11-27 | T05 | **标题栏与工作台三栏**（纯样式 + 4 处 class 追加，**TDD 例外**：无行为变化，按计划用真窗口计算样式、截图与既有 ac 验证）。`TitleBar` 去掉内联 `--raw-canvas` 底，改 `.titlebar`：`--glass-panel` 底 + 底部 1px 线 + 顶部内高光；三栏外壳与文档折叠竖轨 （`[data-panel=sidebar|agnes|editor|doc-rail]`）改 `--glass-panel` 底、70% 发丝线、内高光，**不再有外投影**；设置页的 `settings-nav` / `settings-content` 不动（T07）。**文字密集区追加 `.sheet`**（不透明内衬）：`VaultSidebar` 的 `.tree-scroll`（保持侧栏底色 `--raw-panel-sidebar`）、`HistoryRail` 的 `.history-list`、`MessageList` 的 `.chat-body`、`EditorPane` 的 `.editor-body`，各配 8px 左右的外边距与 6px 内边距让内衬在外壳内成块（边距集中在 `sidebar.css` / `chat.css` / `editor.css`）。**模糊「关」/ 自动降级**：标题栏回 `--raw-canvas`、无线无高光，三栏回各自的 1.7.0 底色（`--panel-bg`）与 `--shadow-lift-lg`，真窗口实测颜色与 1.7.0 一致（标题栏 236,238,241；侧栏 244,245,247；对话与文档 255）。**与 DESIGN 15.1 表格的差异，请确认**：外壳（标题栏、三栏）只有半透明底、线与内高光，**没有自己的 `backdrop-filter`**。原因两条：① 它们背后只有已经 48px 柔化过的色场，再叠 18px 模糊在画面上看不出差别；② 外壳一旦有 `backdrop-filter` 就成为 backdrop root，壳内的菜单与卡片自己的模糊只能看到壳内部，看不到色场（T03 在技能页已踩到，见上一行）。模糊档位仍然作用在卡片、浮层、对话框这些内含文字、背后有内容的层（T06）。若你希望外壳也严格按 15.1 表格带 `--glass-blur`，需要先决定卡片模糊如何处理 backdrop root，请指示。**修正 T03 的一处取值**：窗口色场 `top` 原按「标题栏以下」取 `var(--titlebar-h)`，与原型（`.field { inset: 0 }`、标题栏本身是玻璃）和 DESIGN 15.3 的 `inset: 0` 不符，现改为 `inset: 0`（仍在标题栏之下的层叠位置，标题栏透出色场）；ac-m8 的对应断言 `top >= 40` 随之改为 `|top| < 1`（原因：色场现在覆盖整个窗口；用 1px 容差是因为 dpr 1.75 下 `getBoundingClientRect` 返回浮点，不是削弱断言）。**改动的既有断言**：仅此一条。**真窗口探测**（`spikes/m10-effects/t05/`，临时 APPDATA / 知识库 / HOME，另放一篇样例文档）：三栏与标题栏矩形改前改后完全一致（标题栏 1427×40@0,0；侧栏 232×846@8,40；对话 603×846@248,40；文档 560×846@859,40）；`tree-scroll` / `history-list` / `chat-body` / `editor-body` 的底分别是 244,245,247 / 255 / 255 / 255，1px 线、12px 圆角；浅色、深色、模糊关三张截图已逐张看过，色场透过标题栏与三栏外壳，内衬清楚，关闭后回到 1.7.0 观感。 | 本地（随 T05 提交） | `tsc` 净；`bun run test:web`：check-bindings、check-effect-tokens（118 定义 / 136 引用）通过，**Vitest 58 文件 480 项**；**注意**：第一次整套跑出过 1 例 `InstallView` 的 Esc 用例失败（M9 记录过的 jsdom 并发抖动，本任务没碰该组件），单独跑 21/21、整套重跑 480/480，如实记录；`go vet` 净、`gofmt -l` 无输出、Go 742 项通过；e2e 构建后 **ac-m0 15/15、ac-m1 16/16、ac-m2 22/22、ac-m3 47/47、ac-m8 59/59、ac-m9 107/107**（证据 `spikes/m10-effects/t05/ac/`）。**未验证**：全量 e2e（按计划 T08 起每任务全量，本任务跑了布局与技能页相关六个）；有真实对话消息、源码 / 分栏模式、搜索面板与 `.backlinks` 底栏时的外观（消息流与分栏的内衬在 T10 视觉对照里补拍；`.backlinks` 与搜索面板属 T06）；对比度（T10 取样复测）。顾问建议先看 low / high 档：已补拍 `after-blur-low|high-light|dark.png` 四张逐张看过，强档下标题栏、对话头、文档标签压在色场上文字仍清楚，没有发现可读性问题（数值对比度仍由 T10 取样）。**亲测**：延后到全部任务完成后 | T06（卡片、浮层与对话框）未放行 |

## 9. 自查

- FR 覆盖：设置与首帧 → T01、T04；令牌与色场 → T02、T03；全应用改造 → T05–T07；降级与系统减少动画 → T03、T04；AC-MAT-1–10 → T08–T10。
- 契约前置：唯一的契约变化（`effects`）在 T01 先落盘。
- 风险前置：真窗口观感、对比度与性能基线放在 T00；令牌检查改写与 perf 测法调整都需用户确认后才动。
- 授权：安装、推送、人工亲测、门禁调整分别单独确认。
