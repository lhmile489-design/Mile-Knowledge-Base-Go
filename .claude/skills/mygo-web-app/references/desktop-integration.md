# 原生桌面集成（Mile Knowledge Base，mygo v0.10.0）

签名已对照 v0.10.0 源码（`C:\Users\LHmile\go\pkg\mod\github.com\egoist\mygo@v0.10.0`）。升级版本后先在源码里确认再用。本项目是**单窗口**应用，托盘与关窗来自通用版，多窗口相关内容不适用。

## 目录

1. 启动顺序与单实例
2. Frameless 窗口与自绘标题栏
3. 深浅主题与防白闪
4. 托盘
5. 关窗行为
6. 文件与消息对话框
7. 库内图片：自定义协议
8. 导航、外链与右键菜单
9. DevTools 与快捷键
10. Secrets：API key
11. netproxy：模型供应商出站代理
12. WatchFiles：知识库文件监听
13. 设计原则：可测的纯 Go 包

## 1. 启动顺序与单实例

```go
func main() {
    if !mygo.App.RequestSingleInstanceLock() { // 必须是第一个 mygo 调用（single_instance.go:65）
        return // 第二实例退出；本应用无文件关联/命令行打开需求
    }
    // 应用设置/最近记录/索引库都由一个进程写，必须单实例（SQLite）
    if err := mygo.Protocol.Handle("kbimg", imageHandler()); err != nil {
        log.Fatal(err) // 建窗前注册（protocol.go:53）
    }
    mygo.Theme.SetSource(themeFromSettings()) // 建窗前设置（modules.go:561）
    mygo.Bind(service.BindAll()...)
    mygo.App.WhenReady(func() {
        service.Startup() // 建主窗 + 托盘；自动打开上次知识库（FR-VLT）
    })
    if err := mygo.App.Run(); err != nil { log.Fatal(err) }
}
```

- 设置/最近记录/索引 SQLite 都由一个进程统一写入，单实例必需。
- `NewTray` 必须在 `App.WhenReady` 之后调用，否则 panic（`modules.go:922`）。

## 2. Frameless 窗口与自绘标题栏（FR-WIN）

```go
win := mygo.NewWindow(mygo.WindowOptions{
    Title:           "Mile Knowledge Base",
    Width: 1440, Height: 900,            // 默认 1440x900（FR-WIN）
    MinWidth: 744, MinHeight: 640,       // 最小 744x640（FR-WIN）
    Frameless:       true,               // 无系统标题栏，自绘 40px 标题栏
    Hidden:          true,               // 配合 OnReadyToShow 防白闪
    BackgroundColor: "light-dark(#eceef1, #0e0f11)", // 令牌 --raw-canvas（Q6）
    URL:             "/",
    Page: mygo.PageOptions{DevTools: devToolsMode()},
})
win.OnReadyToShow(win.Show) // window.go:2125
```

- `Frameless: true`（window.go:100）：页面用 CSS `--app-region: drag` 标可拖区，按钮标 `--app-region: no-drag`（标题栏交互元素不可拖）。
- 自绘标题栏按钮调 `win.Minimize()` / `win.Maximize()`/`Unmaximize()`（或按 `IsMaximized()` 切换）/ `win.Close()`（window.go:1089/1095/1098/1112/924）。
- Windows 无边框窗无系统圆角与悬停贴靠布局，与 D-004 一致，本期原样接受（P1 另批）。
- 最小化后恢复用 `win.Restore()`、`win.Focus()`。

## 3. 深浅主题与防白闪（FR-THEME）

- 三档：浅色/深色/跟随系统，默认跟随系统，存应用设置。
- 建窗前 `mygo.Theme.SetSource(mygo.ThemeSystem|ThemeLight|ThemeDark)`（modules.go:561，ThemeSource 常量 modules.go:394-396）。
- `BackgroundColor` 用 `light-dark(#eceef1, #0e0f11)`，与令牌 `--raw-canvas` 一致（Q6；旧版 `#F6F7F9`/`#0B0F14` 与令牌不一致，新版以令牌为准）。
- 页面：`:root{color-scheme:light dark}`，深色用 `@media (prefers-color-scheme:dark)` 或 `data-theme` 覆盖同名令牌；`index.html` 内联脚本按 `prefers-color-scheme` 先写 `data-theme`，防首帧闪。
- 设置里改外观：先持久化成功再 `SetSource` + `Emit` 通知页面；失败不改变当前外观。
- 切换那一帧页面关过渡，避免颜色逐个擦除（FR-THEME）。

## 4. 托盘（FR-WIN）

```go
tray, err := mygo.NewTray(mygo.TrayOptions{Icon: iconPNG, ToolTip: "Mile Knowledge Base"}) // modules.go:922，须在 WhenReady 后
if err != nil {
    trayOK = false // FR-WIN：托盘创建失败时，"挂到托盘"按退出处理
} else {
    trayOK = true
    tray.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
        {Label: "显示主窗口", Click: func(*mygo.MenuItem, *mygo.Window) { mygo.RunOnMain(showMain) }},
        mygo.Separator(),
        {Label: "退出", Click: func(*mygo.MenuItem, *mygo.Window) {
            quitting.Store(true); mygo.App.Quit()
        }},
    }))
    tray.OnClick(func() { showMain() }) // 单击显示主窗口
    // 右键自动弹菜单，不必再 OnRightClick(tray.PopUpMenu)
}
```

- **Windows 单击与菜单（源码已核 + R-4/R-5 实测确认机制成立）**：`internal/windows/tray.go:400-411` 显示，左键 `wmLButtonUp` 触发 `Clicked()`（即 `OnClick`），**与是否设了菜单无关**；右键 `wmRButtonUp` 触发 `RightClicked()` 且**自动 `PopUpMenu(t.menu)`**。所以「单击显示主窗口 + 右键出菜单」直接 `OnClick(showMain)` 即可，不需要手动 `OnRightClick`。注意通用版写的「OnClick 只在无菜单时触发」对应的是 macOS/Linux 语义，Windows 实现不同，**以本段为准**。
- `showMain()`：`win.SetSkipTaskbar(false)`、`win.Show()`、若 `IsMinimized()` 则 `Restore()`、`Focus()`。**最小化后恢复用 `Show()`+`Focus()`（R-4 §2.2 实测）**：`toggleMaximize`/`Maximize` 是 maximize/unmaximize 切换，不能从最小化恢复——恢复用 `Show`+`Focus`（`IsMinimized()` 时先 `Restore()`）。
- 隐藏到托盘时同时 `win.SetSkipTaskbar(true)`（window.go:1520）。
- 托盘图标用 `//go:embed` 嵌入 PNG。

## 5. 关窗行为（FR-WIN）

```go
var quitting atomic.Bool

win.OnClose(func(e *mygo.CloseEvent) { // window.go:2102；主线程，只做短动作
    if quitting.Load() { return }        // 真退出，放行
    e.PreventDefault()
    go handleCloseRequest(win)           // 慢活（对话框）起 goroutine
})
mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { quitting.Store(true) }) // app.go:584
```

`handleCloseRequest` 顺序（对应 FR-WIN 与 AC-WIN-1/2）：

1. 有未保存文档：先 `Dialog.Message`「继续编辑 / 丢弃并关闭」；选继续编辑则中止关窗。
2. 再按设置里的关窗行为：
   - `ask`（默认）：`Dialog.Message`「挂到托盘 / 退出应用」，`CheckboxLabel:"记住我的选择"`；勾选后把选择写回设置。取消/Esc 按「挂到托盘」处理。
   - `tray`：`trayOK` 则 `win.Hide()`+`SetSkipTaskbar(true)`；**`trayOK=false` 时按退出处理**（托盘创建失败不能把窗口藏起来再也找不回，FR-WIN）。
   - `quit`：`quitting.Store(true); mygo.App.Quit()`。

## 6. 文件与消息对话框（Dialog）

- 选知识库目录：`mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: mygo.CallerWindow(ctx), Directory: true})`（dialog.go:141）；取消返回空切片，不报错、不建库、不写最近记录。
- `OpenDialogOptions` 字段：`Parent/Title/DefaultPath/ButtonLabel/Filters/Directory/Multiple/ShowHiddenFiles`（dialog.go:36-57）。
- 关窗询问 / 未保存确认：`mygo.Dialog.Message(opts)`（dialog.go:247）返回 `MessageResult{Button, CheckboxChecked}`；`MessageOptions` 含 `Parent/Type/Title/Message/Detail/Buttons/DefaultButton/CancelButton/DestructiveButtons/CheckboxLabel/CheckboxChecked`（dialog.go:87-114）。Windows 用 task dialog，支持复选框，满足 AC-WIN-1。R-4 §2.4：`OnClose`+`PreventDefault`+goroutine 弹 `Dialog.Message` 链路实测成立；`CancelButton:-1` 时用户按 Esc/关对话框的 dismiss 报 `Button=-1`（具体按钮索引待真机回填 M1–M3）。
- 选目录得到的授权 token 有效期 10 分钟，前端拿不到绝对路径（FR-VLT）：Go 侧把选中路径映射成一次性 token 发给页面，页面凭 token 调后续方法。

## 7. 库内图片：自定义协议（FR-DOC，替代旧版 base64 IPC）

```go
func imageHandler() http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        win := mygo.CallerWindow(r.Context())
        if win == nil { http.NotFound(w, r); return }
        token := strings.TrimPrefix(r.URL.Path, "/")
        res, err := service.ResolveImage(win.ID(), token) // 授权校验+类型白名单+8MiB上限
        if err != nil { http.Error(w, "", http.StatusForbidden); return } // 不回传路径
        w.Header().Set("Content-Type", res.MIME)
        w.Header().Set("Cache-Control", "no-store")
        http.ServeContent(w, r, "", res.ModTime, res.File)
    })
}
```

- 渲染管线把库内相对图片改写成 `kbimg://localhost/<token>`，Windows WebView2 实际地址 `http://kbimg.localhost/<token>`（`protocol.go` 的 `schemeHosts`）。前端 CSP `img-src` 要放行它。
- token 是渲染时登记的随机串 → 「窗口 ID + 已校验真实路径」；每次请求仍重新 `Lstat` 校验（防渲染后换成联接）。
- 只服务 png/jpg/gif/webp/bmp/avif，按文件头判类型，上限 8 MiB（FR-DOC）；SVG/ICO 一律拒。
- `data:image/*` 直接显示；https 远程图默认占位点击才加载；http 拦截（FR-DOC 图片，前端/renderer 层职责）。
- 注册须在建窗前：`Protocol.Handle`（protocol.go:53）或 `HandleFunc`（:68）。

## 8. 导航、外链与右键菜单（PRD 10.2）

```go
page.OnWillNavigate(func(e *mygo.NavigateEvent) { // window.go:2179
    if !isAppURL(e.URL) { e.PreventDefault() } // 页面不离开应用
})
page.OnContextMenu(func(e *mygo.ContextMenuEvent) { // context_menu.go:124
    if e.Menu != nil {
        e.Menu.Remove(mygo.ContextMenuReload, mygo.ContextMenuBack, mygo.ContextMenuForward, mygo.ContextMenuInspect)
    }
    // 或 e.Menu = nil 整体不显示；也可 Append 自定义项
})
```

- 应用来源：开发期 `devUrl`（`http://127.0.0.1:<port>`），构建后 `http://mygo.localhost`；两者放行，其余 `PreventDefault`。
- 外链：前端拦 `<a>` 点击，调 `WindowService.OpenExternal(ctx,url)`；Go 只接受 `http`/`https`，其余协议（`javascript:`/`data:`/`file:`/自定义协议/`mailto:`）直接拒；同意后 `mygo.Shell.OpenExternal(url)`（modules.go:35）。
- 库内相对路径、wikilink、`#` 锚点在应用内处理（FR-LINK），由前端事件委托 + 绑定方法解析。

## 9. DevTools 与快捷键

- 生产构建 `PageOptions.DevTools = mygo.DevToolsDisabled`（window.go:47）。源码显示它同时关 `AreBrowserAcceleratorKeysEnabled`，WebView2 自带 F5/Ctrl+R/Ctrl+P/Ctrl+F/Ctrl+加减缩放全失效；页面 keydown 不受影响，应用快捷键由前端实现。
- 开发构建 `DevToolsAuto`（默认，开发期可检查）。
- 不注册 `GlobalShortcut`（本应用无全局快捷键需求）。

## 10. Secrets：API key（FR-AI-PROV）

- `mygo.Secrets`（secrets.go:48）存 key，Windows 走凭据管理器。`Set(key, value)`/`Get(key)`/`Delete(key)`（:85/97/115）。
- 上限 `mygo.MaxSecretSize` = **2560 字节**（secrets.go:58），PRD 已改为 ≤2560 字节；key 不含换行。
- 系统加密不可用时 `errors.Is(err, errors.ErrUnsupported)` 为真（secrets.go:42），此时**拒绝保存，不退回明文**（FR-AI-PROV）。
- service 名默认取 `identifier`；`mygo dev` 下是 `<identifier>.dev`，与已安装应用的 key 隔离（secrets.go:32）。
- 界面只显示「已配置 / 未配置」，不回显掩码；解密失败提示重填。

## 11. netproxy：模型供应商出站代理（FR-AI-ENG）

- `netproxy.Transport()`（netproxy/netproxy.go:82）返回 `*http.Transport`，跟随 Windows 系统代理（WinHTTP/PAC/WPAD），回环地址直连，SOCKS 也可用，给 `engine` 的 `http.Client` 用。
- 出站地址校验：http 只允许回环（127.0.0.1/localhost），https 任意，URL 不带用户名密码（FR-AI-ENG）。

## 12. WatchFiles：知识库文件监听（R-1 已实测定案）

签名：`func WatchFiles(ctx, path, opts FileWatchOptions) (*FileWatcher, error)`（watch.go:168）；`(w *FileWatcher) Next(ctx)`（:288）返回一批 `FileEvent`，`io.EOF` 表示监听结束或根目录消失。

`FileWatchOptions`（watch.go:58-96）：`Recursive`、`Ignore(path,isDir)`、`Debounce`（默认 50ms）、`MaxDelay`（默认 500ms）、`MaxEntries`、`HintsOnly`。

**选定方案（R-1 实测，见 spikes/r1-watch/RESULT.md，阶段 07）：方案 A —— 普通模式 + `Ignore(node_modules/.git)` + `MaxEntries=65536`。**

```go
mygo.WatchFiles(ctx, vaultRoot, mygo.FileWatchOptions{
    Recursive:  true,
    MaxEntries: 65536,               // 默认 8192 会 ErrWatchLimit；1.5 万条目样本下约 4 倍余量
    Debounce:   50 * time.Millisecond,  // 默认即可
    MaxDelay:   500 * time.Millisecond, // 批量写入期合并突发
    Ignore: func(path string, isDir bool) bool {
        return isDir && (filepath.Base(path) == "node_modules" || filepath.Base(path) == ".git")
    },
})
```

实测读数（1 万 md + 5000 文件代码目录，360 目录、15001 文件）：

- 建立监听耗时 7735ms = mygo 普通模式**首次全量遍历 + stat 建快照**，一次性成本，发生在打开知识库时——**放后台 goroutine 做，不阻塞 UI**（PRD 允许「索引在后台建立」）。Windows 端 `ReadDirectoryChangesW` 根目录 `Subtree=true` 单句柄覆盖整树，不逐目录加句柄。
- S1 原子写延迟 ~52–55ms（含默认 50ms 去抖），S2 burst 1000 文件 1.9s 内全部识别 0 丢失，S3 目录增删改全部收到。满足「外部改动同步 ≤1s」。
- 溢出兜底：raw 通道满 → spill → 全量重扫，「宁可重扫不丢」，与 S2 零丢失一致。
- `MaxEntries` 管的是快照条目数不是句柄数，调大只多内存与首次遍历时间；65536 只校验非负，被接受。

`Ignore` 排除 `node_modules`/`.git`（这两个子树按 PRD 必须忽略，事件直接不入树）；`.mile-kb` 按 FR-AREA 要显示，监听不排除、索引层再过滤。注意 `.mile-kb` 内自身写入（索引落盘）会触发事件回环，需在业务层忽略。

备选（R-1 同时实测，不选但记录）：

- **方案 B `HintsOnly:true`**：只报目录级 `FileHint`、不报具体文件，Go 侧按 hint 目录做增量 stat diff（需自写 ~150 行 diff）。建立几乎零成本（~360 目录 < 8192 上限），适用「a 的首次 7.7s 快照不可接受、或库远大于 1.5 万条目」。本项目未选：方案 A 已达标且精确到文件不需 diff。
- **方案 C 裸 fsnotify**：公开 API **无递归**（`Add(root\...)` 只收直接子项），须逐目录 `Add`（311 句柄）且新建目录要补 `Add`、无去抖、是第二依赖。仅作 a/b 都失败时的兜底。

通用纪律：事件是"再读一次"的提示不是精确操作记录；原子替换保存报为写入；监听挂在知识库自己的 ctx 上，切换/移除知识库时取消；`WatchFiles` 对单文件通过其父目录监听；外部批量改动（git checkout 1000 文件）期间界面保持可操作、索引后台追上（P-1/P-6）。

## 13. 设计原则：可测的纯 Go 包

托盘/关窗分支、授权 token 表、监听聚合、外观切换门限都不 import mygo，原生调用通过函数参数注入，`service`/`main` 只做接线。边界条件（重复就绪、关窗后再发、托盘失败、监听超限、Ask 勾选写回）用 `go test` 覆盖，原生行为只在真实窗口验收。
