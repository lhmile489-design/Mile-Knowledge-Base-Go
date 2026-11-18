---
name: mygo-web-app
description: Mile Knowledge Base（Go 重构版）用 egoist/mygo v0.10.0（Go + WebView2 + React 19）开发的工作法：单窗口 Frameless 无边框 + 自绘标题栏、托盘与关窗行为、深浅主题防白闪、绑定方法与 Channel 流式、定向事件、文件/消息对话框、库内图片自定义协议、导航与外链限制、生产关 DevTools、文件监听、Secrets 存 API key、netproxy 代理、前端 transport 封装与 Zustand 细粒度订阅、构建与真实窗口端到端验证（plugins/automation）。只要在写或改本项目的 Go 绑定、窗口/托盘/关窗、事件、原生行为、流式推送、前端调 Go、mygo dev/generate/build，或提到 mygo.Bind、Channel、Emit、Protocol.Handle、NewTray、WatchFiles、Secrets、mygo-runtime、src/mygo.ts，都应加载本 skill。
---

# mygo Web 应用开发（Mile Knowledge Base 版）

本 skill 以 `docs/PRD.md`（v1.1）与 `docs/技术设计.md`（v1.0）为准，API 已对照 **mygo v0.10.0** 源码核对（路径 `C:\Users\LHmile\go\pkg\mod\github.com\egoist\mygo@v0.10.0`）。本项目是**单窗口**应用，区别于多窗口的 Mile Typora；托盘、关窗、流式、单实例来自通用版 mygo-web-app。

## 0. 动手前先做的三件事

1. **以锁定版本源码为准。** `go list -m -f '{{.Dir}}' github.com/egoist/mygo` 定位源码，读 `docs/app.md`、`docs/windows.md`、`docs/bindings.md`、`docs/frontend.md`、`docs/native.md`。本 skill 里的签名对应 v0.10.0；升级版本后先对比源码再用。
2. **看项目已有约定。** `mygo.config.ts`、`vite.config.ts` 端口、`package.json` 的 `mygo-runtime`/`mygo-cli` 由脚手架生成并钉成精确 0.10.0；`src/mygo.ts` 是 `mygo generate` 的生成物，不手改。
3. **确认任务落在哪一层**，只读需要的参考文档：

| 任务 | 读 |
| --- | --- |
| 绑定方法、契约类型、错误、Channel 流式（token 按帧合并）、取消 | `references/bindings-and-channels.md` |
| Frameless 窗口与自绘标题栏、托盘、关窗行为、对话框、图片协议、导航/外链、DevTools、主题、Secrets、netproxy、WatchFiles、单实例 | `references/desktop-integration.md` |
| 前端 transport、Zustand 细粒度订阅、迟到结果过滤、事件订阅与就绪、mock | `references/frontend-transport.md` |
| `mygo dev`/`generate`/`build` 报错、发版、真实窗口端到端验收（plugins/automation）、踩坑 | `references/build-and-verify.md` |

## 何时加载 / 何时不加载

| 加载 | 不加载 |
| --- | --- |
| 写/改 Go 绑定、Channel 流式、事件、托盘、关窗、对话框、图片协议、导航/外链、主题、Secrets、代理、文件监听、前端调 Go、端到端验证 | 纯 Go 内部逻辑（路径校验、索引、渲染、变更事务）→ `go-service-engineering` / `markdown-render-pipeline` |
| 排查 mygo dev/generate/build、WebView2、原生行为 | 新建工程、配 mygo.config.ts/vite/package.json → `mygo-app-scaffold` |
| 涉及 FR-WIN / FR-THEME / FR-AI-ENG 流式 / FR-DOC 图片 / R-1..R-8 原生验证 | UI 视觉与令牌 → `design-tokens` |

## 1. 心智模型（单窗口）

```text
Go 进程（一个）                              一个 Frameless 窗口 + 一张 React 页面
  main: 单实例锁 -> Protocol.Handle -> Theme.SetSource
        -> Bind -> WhenReady(建窗+托盘) -> Run
  service 层（唯一 import mygo 的业务层）
     |                                            |
  绑定方法 --(Promise)------ 调用：读文档、检索、设置、变更计划
  Channel[T] ---- 流式 ------ 引擎 token 按帧合并 -> for await
  Event.Emit ---- 推送 ------ 文件变更、树变更、审批卡、外观切换
  Protocol "kbimg" -- 图片 -- 授权后的库内图片 -> <img>
```

三条通道：

- **调用**：一次性请求/响应。返回 error 在前端是 `CallError`（只有 `name`/`method`/`message`）。
- **Channel**：一次调用内的连续数据，本项目主要是**引擎流式 token**（Go 侧按帧合并后推送，FR-AI-CHAT 新增约束，解 P-5）。
- **Event**：Go 主动推送，与某次调用无关（文件变更、树变更、审批卡出现/失效、外观切换）。单窗口也建议定向 `EmitPage(win.Page(), ...)`，不依赖 `Broadcast`，便于以后扩展与记录失败。

## 2. 线程与生命周期规则（v0.10.0 源码）

- 绑定方法每次调用跑在独立 goroutine，可阻塞。服务结构体按值绑定、不放可变状态；共享状态放包级变量加锁或指针成员。
- 窗口回调（`OnClose`/`OnClosed`/`OnFocus`、托盘点击、菜单点击）跑在主线程，只做短动作，慢活起 goroutine。
- mygo API 可在任意 goroutine 调用，会转到主线程。需要"判断 + 原生调用"不可分割时用 `mygo.RunOnMain`。
- 绑定方法的 ctx 在页面导航或窗口关闭时取消。所有外部 IO 从它派生，**不要 `context.Background()`**；窗口级长任务（文件监听）挂在自己的 ctx，关窗时取消。
- `mygo.CallerWindow(ctx)` / `mygo.CallerPage(ctx)` 拿调用方窗口/页面：对话框 `Parent`、定位上下文都靠它；**不信任前端传的窗口 ID 或绝对路径**。
- `NewTray` **必须在 `App.WhenReady` 之后调用**（否则 panic，`modules.go:922`）。
- 单窗口应用：最后一个窗口关闭时应用默认退出；但本有关窗行为（挂到托盘），`OnClose` 里 `PreventDefault` 拦截，见 desktop-integration。

## 3. 本项目特有的几条（对应 PRD）

- **单窗口 Frameless**：`WindowOptions{Frameless: true}`，自绘 40px 标题栏，整条 `--app-region: drag`，按钮 `no-drag`。默认 1440×900，最小 744×640（FR-WIN）。无边框窗口在 Windows 缺少系统圆角/贴靠布局，与 D-004 一致（原样接受）。
- **防白闪**：`BackgroundColor` 与页面画布色一致，用 `light-dark(#eceef1, #0e0f11)`（FR-THEME，Q6 以令牌 `--raw-canvas` 为准）；`Theme.SetSource` 在建窗前；`Hidden:true` + `OnReadyToShow(win.Show)` 彻底避免加载期闪。
- **托盘**：`NewTray` 菜单「显示主窗口 / 退出」；Windows 实现里左键 `wmLButtonUp` 触发 `Clicked()`（即有菜单时单击仍会 `OnClick`），右键 `wmRButtonUp` 触发 `RightClicked()` 且自动 `PopUpMenu`（`internal/windows/tray.go:400-411` 源码已核，R-4/R-5 实测确认机制成立）。所以「单击显示主窗口」用 `OnClick`、菜单交给右键自动弹出即可。
- **关窗行为**（FR-WIN）：`OnClose` + `PreventDefault`；先查未保存文档（「继续编辑/丢弃并关闭」），再按设置 ask/tray/quit；询问框用 `Dialog.Message` 带 `CheckboxLabel`（「记住我的选择」）；托盘创建失败时「挂到托盘」按退出处理。
- **Secrets**：`mygo.Secrets` 存 API key（Windows 凭据管理器），上限 `mygo.MaxSecretSize` = 2560 字节（FR-AI-PROV）；系统加密不可用（`errors.Is(err, errors.ErrUnsupported)`）时拒绝保存，不退回明文。
- **代理**：`netproxy.Transport()` 给模型供应商 HTTP 出站（FR-AI-ENG）；回环直连，跟随 WinHTTP/PAC。
- **WatchFiles**：知识库文件监听。**R-1 已实测（阶段 07，spikes/r1-watch/RESULT.md），选定方案 A**：普通模式（`Recursive:true`）+ `Ignore` 排除 `node_modules`/`.git` + `MaxEntries=65536`（默认 8192 会报 `ErrWatchLimit`；65536 对 1.5 万条目约 4 倍余量）。Windows 整树单句柄覆盖（`Subtree`），首次全量快照 ~7.7s 放后台 goroutine。HintsOnly 与 fsnotify 方案见 desktop-integration §12（均为备选/兜底）。
- **图片**：`Protocol.Handle` 注册自定义 scheme 提供库内图片，替代旧版 base64 IPC（FR-DOC 新增约束）。Windows 上地址是 `http://<scheme>.localhost/...`。
- **生产关 DevTools**：`PageOptions.DevTools = DevToolsDisabled`，同时关掉 WebView2 自带 F5/Ctrl+R/Ctrl+P/Ctrl+F/缩放等按键（`internal/windows/webview.go`）；应用快捷键由前端实现。

## 4. 流式与取消（FR-AI-ENG / FR-AI-CHAT）

- token 在 Go 侧**按帧合并**后推送：把供应商 SSE 的 delta 累积到一帧（如每 ~16ms 或每 N 字符），再一次 `ch.Send` 或定向事件给前端，解 P-3/P-5。前端只更新正在输出的那条消息，不重新解析已完成消息。
- 取消贯穿：用户停止 → 前端 `channel.close()`，Go 侧绑定方法 ctx 取消 → 引擎 HTTP 请求 ctx 取消。需要 Go 侧先落"已中止"再停时用 `runId` + `map[runId]cancel` 另开 `Cancel(runId)` 方法。
- 引擎在 Go 进程内运行（技术设计），单回合出错不能拖垮应用；取消/崩溃恢复按 FR-AI-ENG。

## 5. 安全边界（PRD 10.2）

1. 页面不离开应用：`page.OnWillNavigate` 只放行应用自身来源（开发期 `devUrl`、构建后 `http://mygo.localhost`），其余 `PreventDefault`；http(s) 外链经绑定方法 `Shell.OpenExternal` 交系统浏览器，其余协议一律拒绝。
2. 库内图片经自定义协议 + 授权 token；`data:image/*` 直接显示；https 远程图默认占位点击才加载；http 拦截（FR-DOC 图片）。
3. 生产 `DevToolsDisabled`；`page.OnContextMenu` 移除/置空 webview 菜单。
4. 所有用户/Agent 路径在 Go 侧 `pathguard` 两道校验（拼写层 + 真实路径层），不信前端传入。

## 6. 构建、发版与验证

- `mygo dev` 起 Vite + 窗口；`dev` 与 `devUrl` 端口同改，`strictPort: true`。
- `mygo build` 出安装包（NSIS 首联网下载）；`-skip-installer` 只出 exe 且清空输出目录。
- 端到端：`plugins/automation` + build tag `e2e`，能驱动真实窗口、执行页面脚本、截图，替代旧版 32 个 Electron 真窗口脚本；发布构建不含（`mygo_noinspector`）。
- **构建通过不等于原生验收通过。** 托盘、关窗、对话框、DPI、深浅主题、Frameless 拖动/缩放都要在真实 Windows 11 窗口里过，汇报分开写「自动验证」与「原生验收」。

## 7. 交付前自检

- [ ] 用到的 API 在 v0.10.0 源码里核对过签名
- [ ] 事件定向发送；绑定方法用 `CallerWindow(ctx)` 定位，不信前端窗口 ID/绝对路径
- [ ] 流式 token 在 Go 侧按帧合并；取消从绑定 ctx 一路传到引擎 HTTP
- [ ] 关窗走 `OnClose`+`PreventDefault`，未保存确认与 ask/tray/quit 三分支齐全
- [ ] `NewTray` 在 `WhenReady` 后调用；托盘失败「挂到托盘」降级为退出
- [ ] `BackgroundColor`/`Theme.SetSource` 在建窗前；首帧无白闪
- [ ] API key 只走 `mygo.Secrets`，≤2560 字节，不可用即拒存
- [ ] 图片只经自定义协议并做授权校验；外链只放行 http(s) 交 `Shell.OpenExternal`
- [ ] 生产构建 `DevToolsDisabled`，导航被限制在应用内
- [ ] `src/mygo.ts` 由 `mygo generate` 生成；生产构建不引用 `src/mocks/`
- [ ] 汇报分清自动验证与原生验收
