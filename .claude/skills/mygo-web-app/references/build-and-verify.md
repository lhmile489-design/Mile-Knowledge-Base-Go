# 构建、发版与验证（Mile Knowledge Base，mygo v0.10.0）

## 目录

1. 命令速查
2. 配置契约
3. 发版流程
4. 真实窗口端到端验证（plugins/automation）
5. 原生验收清单
6. 踩坑速查

## 1. 命令速查

```bash
bunx mygo doctor             # 环境自检（Go、Bun、WebView2）
bun run dev                  # = mygo dev：起 Vite + 开窗，监听 Go 变化重编
mygo dev -background         # 窗口不抢焦点（cmd/mygo/dev.go:67），适合边写边看
bun run generate             # = mygo generate：重新生成 src/mygo.ts
bun run build                # = mygo build：安装包 + exe（NSIS 首次联网下载）
mygo build -skip-installer   # 只出 exe，会清空输出目录
go test ./...
go test -race ./internal/...
go vet ./...
bun run typecheck
bunx vitest run
```

- bash 里找不到 `go`：`export PATH="/c/Program Files/Go/bin:$HOME/go/bin:$PATH"`。
- 端到端（build tag `e2e`）：`go test -tags e2e ./e2e/...`，需先 `go build -tags e2e` 出带 automation 的 exe 并设 `MYGO_AUTOMATION`。
- **`MYGO_AUTOMATION` 在 Windows 上是 AF_UNIX 文件 socket**（`net.Listen("unix")`，R-4 §1/§5 实测）：Git Bash 的 `socat`/`curl` 连不上，必须用 Go `net.Dial("unix")` 或等价的 AF_UNIX 客户端驱动（`listen.go:28` 支持 `tcp:addr` 扩展）。
- **`go run -tags e2e` 每次编译出新临时 exe，Windows 防火墙逐 exe 弹「是否允许网络访问」**，遮挡应用窗口并干扰自动化截图（R-4 §1/§5 实测）。E2E 前先 `go build -tags e2e -o <固定名>.exe`，防火墙只弹一次。

## 2. 配置契约

v0.10.0 web 模板（`cmd/mygo/template/web/`）生成的字段：

| 文件 | 字段 |
| --- | --- |
| `mygo.config.ts` | `name`、`identifier`、`version`、`devUrl`、`devCommand:"bun run dev:web"`、`buildCommand:"bun run build:web"`、`frontendDist:"dist"`、`bindings:"src/mygo.ts"`、`out:"build"` |
| `vite.config.ts` | `server.port`、`strictPort:true`、`watch.ignored:["**/.mygo/**","**/build/**"]` |
| `package.json` | `mygo-runtime`（deps）、`mygo-cli`（devDeps）、`typescript ^7`、`vite ^8`；脚本 `dev/build/generate/dev:web/build:web/typecheck` |

- `mygo.config.ts` 注释里还含 `devUrl:"http://localhost:5173"` 与 ios/android 段——本项目只做 Windows，平台字段不填；端口按阶段 10 定稿值改，建议 `host:"127.0.0.1"`。
- `identifier` 定稿后不改（决定 `PathUserData` 与单实例锁，建议 `dev.mile.kb`）。
- `mygo-runtime`/`mygo-cli` 钉成精确 `0.10.0`。

## 3. 发版流程

1. 改 `mygo.config.ts` 的 `version`；Go 里若另有版本常量（"关于"页 FR-SET）同步改并加比对测试。
2. 跑全：`go test ./...`、`go vet ./...`、`bun run typecheck`、`bunx vitest run`。
3. 构建后 grep `dist/`：没有 `src/mocks/` 内容、没有意外 `http(s)://` CDN 引用（离线）。
4. `mygo build`；需要免安装 exe 时先备份安装包再 `-skip-installer`。
5. 记录产物大小与 SHA-256；包体与内存基线在首个可运行版本测旧版对比（PRD G5）。
6. 图标 `resources/icon.png` 用 1024×1024 带透明通道 PNG；复制二进制用 `cp`/`Copy-Item`。

## 4. 真实窗口端到端验证（plugins/automation）

替代旧版 32 个 Electron 真窗口脚本（PRD 4.3 不迁移旧脚本，新版自己定方式）。

```go
//go:build e2e
package main
import _ "github.com/egoist/mygo/plugins/automation"
```

- 机制（`plugins/automation/doc.go`）：import 后，仅当环境变量 `MYGO_AUTOMATION=<sock路径>` 时才在 Unix socket 上监听 JSON 请求；能列出窗口、读原生 UI 元素树、发指针/滚轮/键/文本输入、点应用菜单、在页面执行脚本、窗口截图 PNG、等待 settle。
- 用 build tag `e2e` 隔离：带 tag 构建出含插件的 exe 供测试驱动；发布构建走 `-tags mygo_noinspector`/`mygo build`，插件被剔除（`//go:build !mygo_noinspector`，automation.go:1），线上不含。
- 命令行客户端 `cmd/mygo-automation` 可驱动；测试里用 socket 发 JSON。
- 自动化走查只在用户明确要求时执行。

## 5. 原生验收清单（对应 PRD 12 / AC）

自动测试覆盖不到原生行为。以下在真实 Windows 11 上用**分发包**逐项验收：

| 项 | 怎么验 | 依据 |
| --- | --- | --- |
| 窗口与标题栏 | 默认 1440×900、最小 744×640；自绘栏拖动、双击最大化、边缘缩放；三按钮行为 | FR-WIN、R-6 |
| 防白闪 | 冷启动首帧即深色/浅色画布；系统切外观、设置三档切换 | AC-THEME-1、R-6 |
| 托盘 | 单击显示主窗、右键出「显示主窗口/退出」；隐藏到托盘后任务栏无图标 | FR-WIN、R-5 |
| 关窗 | ask 弹「挂到托盘/退出」+「记住我的选择」；有未保存先确认；取消按挂到托盘 | AC-WIN-1/2、R-8 |
| 托盘失败 | 模拟创建失败时「挂到托盘」按退出 | FR-WIN |
| 对话框 | 选目录取消不改状态；中文/空格路径 | FR-VLT |
| 图片 | 库内相对图显示、data:image 显示、https 占位点击加载、http 拦截、SVG/越界拒 | FR-DOC |
| 导航/外链 | http(s) 交系统浏览器；其它协议/库外路径不跳转 | PRD 10.2 |
| DevTools/快捷键 | 生产包 F5/Ctrl+P/F12 无效；应用快捷键正常 | PRD 10.2 |
| Secrets | 存取 key、超限拒绝、不可用拒绝、解密失败提示 | FR-AI-PROV |
| 文件监听 | 大样本不超限；外部改 1000 文件 1 秒级同步、界面可操作 | R-1、AC-PERF-1 |
| 流式 | 长回答流式时输入框打字 p90≤16ms、60fps | AC-PERF-2、R-4 |
| 性能指标 | 冷启动、首屏、检索、内存、包体按 PRD 10.1 | NFR-PERF、G4/G5 |

汇报分两栏：「自动验证通过」与「原生验收待补」，不用构建成功代替原生验收。

## 6. 踩坑速查

| 现象 | 原因 | 处理 |
| --- | --- | --- |
| `mygo dev` 白屏 | Vite 换了端口，`devUrl` 仍指旧端口 | `strictPort:true`，两处同改 |
| generate 报找不到 mygo 包 | go.mod 缺 require | `go mod tidy` |
| 生产包 F5 重载整个页面 | `DevTools` 没设 `DevToolsDisabled` | 生产构建关掉 |
| 库内图片全 404 | 协议在建窗后才注册，或 CSP 没放行 `http://<scheme>.localhost` | 建窗前 `Protocol.Handle`；CSP `img-src` 加上 |
| 页面收不到首批事件 | 页面未就绪时就 Emit | 就绪握手：订阅完调 `ready()` |
| 启动白闪 | `BackgroundColor` 与 `--raw-canvas` 不一致 | `light-dark(#eceef1,#0e0f11)` + `Hidden`+`OnReadyToShow` |
| 托盘单击没反应 | 误以为有菜单时 OnClick 不触发 | Windows 实现里左键总会 `Clicked()`，右键自动弹菜单（见 desktop-integration §4） |
| 托盘「退出」退不掉 | `OnClose` 又把它拦回托盘 | `quitting` 标志 + `App.Quit` |
| 「挂到托盘」后窗口找不回 | 托盘创建失败仍隐藏窗口 | `trayOK=false` 时按退出处理 |
| `bun run test` 报怪错 | 命中系统 `test` 命令 | `bunx vitest run` |
| 大库监听报 ErrWatchLimit | 默认 MaxEntries 8192 超限 | R-1 定案：普通模式 + `MaxEntries=65536` + `Ignore(node_modules/.git)`（见 desktop-integration §12） |
| E2E 时防火墙反复弹窗遮挡窗口 | `go run -tags e2e` 每次出新临时 exe | 先 `go build -tags e2e -o <固定名>.exe`，防火墙只弹一次（R-4） |
| Git Bash 连不上 automation socket | Windows 上 `MYGO_AUTOMATION` 是 AF_UNIX 文件 socket | 用 Go `net.Dial("unix")` 驱动，或 `tcp:` 扩展（R-4） |
| 短时连开多实例 WebView2 创建失败 `HRESULT 0x800700aa` | 同机短时多开 WebView2 | 单实例锁（`RequestSingleInstanceLock`）避免；mygo 已自动重试，正式应用启动失败要兜底提示（R-4 §5） |
| 最小化后用 toggleMaximize 恢复不了 | 它是 maximize/unmaximize 切换，不恢复最小化 | 用 `Show()`+`Focus()` 恢复（`IsMinimized()` 时先 `Restore()`，R-4 §2.2） |
| API key 保存失败 | 超 2560 字节 / 凭据管理器不可用 | 上限校验前置；不可用拒存不退明文 |
