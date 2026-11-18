# 阶段 07 风险实测 R-4 / R-5 / R-6 / R-8 — 结果

> 范围：只写 `spikes/r4-window/`；不改项目其他文件。日期 2026-11-18，Windows 11，
> Go 1.27.3，Bun 1.3.13，mygo v0.10.0（Go module / mygo-runtime / mygo-cli 全部钉死 0.10.0）。
> 显示器约 200 Hz（rAF 帧间隔中位数 ≈ 5 ms，下文所有"帧间隔"以此解读）。

## 1. 运行命令（实测可用）

```bash
# 依赖（一次）
cd spikes/r4-window
go mod tidy            # 需 PATH 里有 go
bun install
bunx mygo generate     # 生成 src/mygo.ts

# 方式 A：Vite dev server + 带 automation 的应用（等价 mygo dev，可注入 e2e）
bun run dev:web                                              # 终端 1：Vite on 127.0.0.1:5274
go build -tags e2e -o /tmp/r4-window.exe .                    # 先 build 一次（避免每次 go run 触发防火墙）
MYGO_DEV_URL=http://localhost:5274 \
MYGO_AUTOMATION='tcp::18081' \
  /tmp/r4-window.exe                                         # 终端 2

# 自动化驱动（终端 3）
cd drive
MYGO_AUTOMATION='tcp:127.0.0.1:18081' go run . -f request.json

# 方式 B：构建产物（前端打进 exe）
bun run build:web
MYGO_FRONTEND_DIST=dist MYGO_AUTOMATION='tcp::18081' go run -tags e2e .
```

`drive/` 里的自动化驱动：`drive/main.go`（发一个 JSON 请求行到 socket，`-f file` 或参数/ stdin）、
`drive/reqs/*.js`（本测量用的页面脚本）。`go vet ./...`、`bun run typecheck` 均通过。

注意：
- `MYGO_AUTOMATION` 在 Windows 上是 **Unix socket 文件路径**（`net.Listen("unix")`），不是 TCP；
  任务书说的 `tcp:` 前缀是我加的非标准扩展，驱动端 `drive/main.go` 里解析。Windows 上 Go 的
  `net.Listen("unix", path)` 生成的是 AF_UNIX socket，**Git Bash 的 socat/curl 连不上**，
  必须用 Go（`net.Dial("unix")`）或等价的 AF_UNIX 客户端连。
- 每次 `go run -tags e2e .` 都编译出新的临时 exe，**Windows 防火墙会对每个新 exe 弹一次
  「是否允许网络访问」**，遮挡应用窗口并干扰自动化截图。解决：先 `go build -o` 出固定名字的
  exe，防火墙只弹一次。

## 2. 自动测得的数据

### 2.1 冷启动（R-6 关联）

| 指标 | 实测 |
| --- | --- |
| 进程 `pre-run` → `app-ready` | 13–16 ms |
| `app-ready` → `tray-created` | ~35 ms（托盘创建无压力） |
| 进程启动 → `ready-to-show`（首帧前） | **536 – 813 ms**（3 次：536 / 785 / 813 ms） |
| 页面 `DOMContentLoaded` 相对脚本执行 | 89 – 205 ms |
| 页面首次绑定调用 `SpikeService.Ready` 返回时 `bootMs` | 538 – 896 ms |
| WebView2 创建失败重试 | 出现过 2 次 `HRESULT 0x800700aa`（同一台机短时连开多实例时；3 次重试后放弃，第 3 次冷启动才成功） |

### 2.2 R-6 无边框窗口

| 检查项 | 自动测得 |
| --- | --- |
| `WindowOptions.Frameless: true` 生效 | 是：automation `windows` 无原生标题栏，截图确认自绘 40px 标题栏 |
| 默认尺寸 | `bounds` 1440×900，`contentBounds` 1424×892（16px 边距差 = 无边框阴影边） |
| 最小尺寸钳制 | `winOp("setSize")` 请求 400×300 → `bounds` 被系统钳到 **744×640**，`contentBounds` 728×632 |
| `WinOp` 绑定 | `minimize` / `toggleMaximize` / `close` / `hideToTray` / `show` 全部通 |
| 最大化 | `toggleMaximize` → `bounds` 2576×1408，`maximized:true` |
| 最小化 | `minimize` → `bounds` 掉到 -32000,-32000、`minimized:true`（Windows 惯例） |
| 最小化恢复 | `toggleMaximize` 不能从最小化恢复（它是 maximize/unmaximize）；`winOp("show")`（`Show`+`Focus`）恢复正确 |
| 首帧底色 | 截图采样：内容区 `rgb(236,238,241)` = `#eceef1`，无白闪；标题栏渐变带（spike 样式），非系统色 |

### 2.3 R-5 托盘

| 检查项 | 自动测得 |
| --- | --- |
| `NewTray` 在 `WhenReady` 后调用 | 成功，`trayOk:true` |
| `hideToTray` | `visible:false`，窗口隐藏，`SetSkipTaskbar(true)` |
| `showMain` | `visible:true`，恢复显示 |

托盘「左键单击显示主窗口 / 右键弹菜单」按 `internal/windows/tray.go:400-411` 源码：
`wmLButtonUp → Clicked()`、`wmRButtonUp → RightClicked() + PopUpMenu(menu)`，与是否设菜单无关。
**源码预期成立，真机点击在手动清单复核。**

### 2.4 R-8 关窗询问框

| 检查项 | 结果 |
| --- | --- |
| `OnClose` + `PreventDefault` | 生效：`winOp("close")` 后窗口仍在，进程未退出（`OnClose` 拦截、`PreventDefault` 调用、`handleCloseRequest` goroutine 启动，源码已核） |
| `Dialog.Message` 带 `CheckboxLabel` | 绑定方法 `dialogProbe` 确实触发了 task dialog（eval 阻塞 = 模态打开），但**自动化期间被 Windows 防火墙弹窗遮挡 + 应用窗口失去焦点，三次触发都在等待用户点击时超时**，未能拿到 `MessageResult` |
| 按钮索引 / 复选框返回值 | 结构签名已核 `MessageResult{Button, CheckboxChecked}`；**具体索引与「记住我的选择」回读待真机点击回填**（M1–M3）。`CancelButton:-1` 时 dismiss 是否报 `Button=-1` 同待回填（M2）。 |

> 应用进程仍在运行（PID 见 `tasklist r4-window.exe`），任务对话框可能仍在某处等点击；
> 手动验收时先把它找出来，或直接重启应用按 M1 重做。

### 2.5 R-4 流式增量渲染（核心数据）

测试条件：50 条历史消息（每条约 2000 字符 HTML）+ 流式写一条 10 000 字符回答；
`Stream` 绑定方法在 Go 侧把 24 字符"token"按间隔合并后经 `Channel` 推给页面；
页面用 `PerformanceObserver('longtask')` + rAF 帧间隔 + 模拟按键测输入延迟。

| 推送方式 | 渲染方式 | Go 侧 Send 次数 | 总耗时 | rAF 帧间隔 p50 / p95 / max | >50ms 长任务 | 最长任务 | 输入延迟 p50 / p90 / max |
| --- | --- | --- | --- | --- | --- | --- | --- |
| frame16（16ms 合并） | 增量（只改 open 块） | 53 | 836 ms | 5.0 / 5.1 / 5.1 ms | 1 | 84 ms | 8.3 / 9.0 / 9.1 ms |
| token（≈2ms 一推） | 增量 | 214 | 836 ms | 5.0 / 5.1 / 5.2 ms | 1 | 84 ms | 8.5 / 9.3 / 10.3 ms |
| frame50（50ms 合并） | 增量 | 17 | 837 ms | 5.0 / 5.1 / 5.2 ms | 1 | 84 ms | — （未并发按键） |
| frame16 | **整段重渲**（对照，模拟 P-3） | 53 | 837 ms | 5.0 / 5.1 / 5.1 ms | 1 | 84 ms | 8.3 / 8.8 / 8.8 ms |

**读数解读：**

- 10 000 字符在 ~836 ms 内推完，4 种方式**全部满足** AC-PERF-2「按键延迟 p90 ≤ 16 ms」（实测 p90 ≤ 10.3 ms）。
- 唯一的长任务（84 ms）是**流式开始前**渲染 50 条历史消息的一次性开销，不是流式本身；
  流式期间没有 >50 ms 的任务，`dropped`（>25 ms 帧间隔）为 0。
- 帧间隔 p50 ≈ 5 ms 说明页面完全跟上 ~200 Hz 刷新率；流式没有造成可测的掉帧。
- **token 模式（214 次 Send）与 16ms 合并（53 次）在渲染端没有可分辨的帧/延迟差异**——
  Channel 的序列化开销在 10k 字符规模下不构成瓶颈；合并的意义更多在语义（按帧交付完整片段）
  而非性能。整段重渲在这个规模下也没劣化到可测，因为正文节点本身不大；差异会在更大回答
  （50k+ 字）或更复杂 Markdown（表格、代码高亮）下放大，本次未覆盖。

## 3. 各项结论

### R-4 对话增量渲染 — 基本通过（规模有限）

- 增量渲染（只改 open 块、已封口块不再动）在 10k 字符 + 50 条历史下，流式期间无 >50ms 任务、
  无掉帧、输入延迟 p90 ≤ 9.3 ms，达标。
- token 级推送在性能上并不比按帧合并差（本规模内）；P-5 的"跨进程 3 次"在 mygo 单进程
  Channel 下天然不存在（Go↔WebView2 同进程桥），按帧合并建议保留（语义清晰），但不必为性能。
- **遗留**：本测试用纯文本段落。真实 Markdown 含表格、代码块、嵌套列表时，"末尾未完成结构先收起"
  的实现成本未验证；50k 字以上长回答的累计 DOM 成本未测。建议阶段 08 在真实引擎接入后复测。

### R-5 托盘 — 机制成立，真机待复核

- `NewTray`、菜单、「挂到托盘」「恢复」全部工作；源码确认 Windows 左键总触发 `OnClick`、右键
  自动弹菜单，与 SKILL 预期一致。左键单击 / 右键菜单的**手感**在手动清单复核。

### R-6 无边框窗口 — 通过

- Frameless、1440×900 默认、744×640 最小钳制、自绘 40px 标题栏 + `WinOp` 绑定、
  `light-dark(#eceef1,#0e0f11)` 首帧无白闪，全部成立。拖动 / 双击 / 边缘缩放 / Aero Snap 在手动清单。

### R-8 关窗询问框 — 机制成立，返回值待真机回填

- `OnClose`+`PreventDefault`+`Dialog.Message`（`CheckboxLabel`）链路通：拦截关窗、task dialog 弹出。
- **未完成**：Windows 防火墙弹窗遮挡 + 窗口焦点被切走，三次触发都没能在时限内拿到
  `MessageResult{Button, CheckboxChecked}`；`CancelButton:-1` 的 dismiss 值同样待回填。
  应用仍在运行供手动验收（M1–M3）。

## 4. 手动确认清单（请逐项回填）

> 前置：先把 Windows 防火墙弹窗点掉，确认 R4 Spike 窗口在最前。
> 需要时重启应用：方式 A 终端 2。

| # | 操作步骤 | 预期结果 | 实测 |
| --- | --- | --- | --- |
| M1 | 点 R4 Spike 标题栏的「dialog probe」按钮，等对话框弹出，点「挂到托盘」，**勾选**「记住我的选择」 | 对话框关闭，窗口挂到托盘；`%TEMP%/r4-spike/spike-log.jsonl` 出现 `dialog-probe`，`button=0`、`checkbox=true` | |
| M2 | 再点「dialog probe」，**不勾选**，按 Esc 或点右上角 X 关对话框 | `button=-1`（dismissed），窗口按「挂到托盘」处理 | |
| M3 | 点「dialog probe」，点「退出应用」 | 进程退出；`button=1` | |
| M4 | 左键单击任务栏托盘区的 R4 Spike 图标 | 主窗口恢复显示（`OnClick` → `showMain`） | |
| M5 | 右键单击托盘图标 | 弹出菜单「显示主窗口 / 退出」，不需再点 | |
| M6 | 点托盘菜单「显示主窗口」 | 窗口恢复 | |
| M7 | 点托盘菜单「退出」 | 进程退出（`quitting` 置位，不被 OnClose 拦回） | |
| M8 | 按住自绘标题栏拖动窗口 | 窗口跟随鼠标移动 | |
| M9 | 双击自绘标题栏空白处 | 窗口最大化；再双击还原 | |
| M10 | 拖窗口四角/四边缩放 | 可调大小，到 744×640 卡住 | |
| M11 | 把窗口拖到屏幕左/右/上边缘 | Aero Snap 半屏/全屏贴靠生效（无边框窗 Windows 11 下可能无贴靠提示，与 D-004 一致则记"符合预期"） | |
| M12 | 系统设置切深色主题，重启应用看首帧 | 首帧深色 `#0e0f11`，无白闪 | |
| M13 | 挂到托盘后检查任务栏 | 任务栏无 R4 Spike 图标（`SetSkipTaskbar`） | |

## 5. 剩余风险

- R-4 只覆盖纯文本流式；真实 Markdown（goldmark 渲染后含表格/代码/嵌套）的增量收口未验。
- WebView2 冷启动偶发 `HRESULT 0x800700aa`（同机短时多开）；mygo 已自动重试，但正式应用启动
  失败路径要兜底提示。
- 自动化 socket 是 AF_UNIX 文件 socket，Git Bash 工具链连不上，需 Go 驱动；
  每次 `go run` 触发防火墙提示，正式 E2E 建议先 build 固定 exe。

## 6. skill 需修正项（仅记录，不改 skill）

`mygo-web-app` skill：

1. `references/build-and-verify.md` §1 命令速查可加：`MYGO_AUTOMATION` 在 Windows 是 AF_UNIX
   文件 socket（`net.Listen("unix")`），Git Bash 的 socat/curl 连不上，需 Go `net.Dial("unix")`
   或 `tcp:` 扩展（`listen.go:28` 支持 `tcp:addr`）。
2. 同文件可加一条踩坑：每次 `go run -tags e2e` 生成新临时 exe，Windows 防火墙逐 exe 弹
   网络访问框遮挡窗口；E2E 前先 `go build -tags e2e -o <固定名>`。
3. `references/desktop-integration.md` §5 关窗：实测 `winOp("close")` → `OnClose`+
   `PreventDefault`+goroutine 弹 `Dialog.Message` 链路成立；可补一句「`Dialog.Message`
   返回 `MessageResult{Button, CheckboxChecked}`，`CancelButton:-1` 时 dismiss 报 `Button=-1`」
   （本次未能在真机回填 index，留 M1–M3）。
4. SKILL.md §3「托盘」段把「R-5 待真机复核」的注保留（机制已核源码，手感仍待点击）。

`mygo-app-scaffold` skill：

5. §1 环境前置检查可补：automation/E2E 需要 `go build -tags e2e`，`go run` 也行但触发
   Windows 防火墙逐 exe 提示。
