# R-10 宿主注入首帧主题色实测

M3.5 的 T00（docs/PLAN-M3.5.md 第 5 节；技术设计 14.3 的 A-1–A-4）。验证在生产 CSP 下，Go 经 mygo `PageOptions.PreloadScript` 注入的主题色能否在页面自己的脚本之前到达，从而让第一帧就是所选颜色。

## 环境

- Windows 11，go1.27.3 windows/amd64，mygo v0.10.0（模块缓存，`GOPROXY=off`）
- 页面：内存 FS（`mygo.SetFrontend`），`index.html` 带与生产完全相同的 CSP meta（`vite.config.ts:8`）
- 每次运行都用 `mktemp -d` 生成的 APPDATA；截图写临时目录；运行后都已删除，没有碰真实应用数据
- 三种模式：`default`（注入 `{"preset":"blue"}`）、`nopreload`（不注入）、`tricky`（注入含 `</script>`、引号与特殊空白的值）

## 运行方式

```bash
export PATH="/c/Program Files/Go/bin:$HOME/go/bin:$PATH"
cd spikes/r10-preload
APPDATA="$(cygpath -w "$(mktemp -d)")" CAPTURE_DIR="$(cygpath -w "$(mktemp -d)")" \
  GOFLAGS=-mod=mod GOPROXY=off go run .          # nopreload / tricky 时另加 MODE=… 
```

页面把观测值经绑定 `Probe.Report` 回传给 Go，Go 打印一行 `PROBE <json>`，随后 `win.CapturePage()` 截图。

## 结果（2026-11-20，三次运行全过）

| 模式 | 注入 | preloadAt | themeInitAt | 内联脚本被 CSP 拦 | 首帧 rAF 时的按钮色 | 截图按钮色 |
| --- | --- | --- | --- | --- | --- | --- |
| default | 是 | 127.3 ms | 133.5 ms | 是 | `rgb(37, 99, 235)` 蓝 | `#2563eb` 蓝（4245 px） |
| nopreload | 否 | — | 142.8 ms | 是 | `rgb(185, 91, 30)` 橙 | `#b95b1e` 橙（4245 px） |
| tricky | 是（含 `</script>`） | 133.4 ms | 137.9 ms | 是 | `rgb(124, 58, 237)` 紫 | — |

三次运行的 `inlineBlocked` 都是 `true`：页面里的内联脚本确实被 CSP 拦了，说明 CSP 生效；而注入脚本每次都执行了，且早于 `theme-init.js`（preloadAt < themeInitAt）。

tricky 模式里页内读到的值原样是 `</script><script>window.__PWNED__=1</script>"quoted"`，`__PWNED__` 未定义——`encoding/json` 编码后没有破坏脚本。

### A-1：生产 CSP 是否拦住宿主注入的脚本

**不拦。** 页面 CSP 是 `script-src 'self'`，内联脚本被拦（`inlineBlocked:true`），但 `AddScriptToExecuteOnDocumentCreated` 注入的脚本照常执行。WebView2 的宿主脚本不属于页面的「内联脚本」。

### A-2：注入脚本是否早于页面 `<head>` 里的同源脚本

**是。** 三次运行 preloadAt 都比 themeInitAt 早 4–7 ms，`theme-init.js` 读到值时 `window.__MKB_ACCENT__` 已存在。

### A-3：首帧是否已经是所选颜色

**是，按可测的代理指标。** WebView2 没有 Paint Timing（三次运行 `paints: []`），无法直接读 first-paint 时间戳。可测证据有两条：

1. `theme-init.js` 是 `<head>` 里的阻塞式同源脚本，在正文布局之前同步设好 `data-accent`；
2. 在 `theme-init` 里注册的第一次 `requestAnimationFrame` 回调（第一帧绘制前解析样式）读到按钮已是注入的颜色；
3. 截图里按钮像素是注入的颜色。

唯一无法直接观测的是「第一帧之前是否存在一帧默认色」。按 1、2 两条，这一帧不存在；窗口背景色由主题的明暗决定（`background` 变量），不随强调色变，所以也没有背景色跳变。

### A-4：没有值时（自定义色无缓存）的表现

**降级为默认橙，不报错。** `nopreload` 模式即此情况：`preloadPresent:false`，页面按默认橙渲染，`theme-init` 不抛错。对应技术设计 14.2 第 2 条「没有 cache 时先不设，等前端包加载后计算」——最差是首帧显示默认橙、随后换成所选颜色。

## 结论

技术设计 14.2 的方案可行，不需要换方案。实现要点：

1. `WindowOptions.Page.PreloadScript` 放一行数据：`window.__MKB_ACCENT__ = <json>;`，JSON 用 `encoding/json` 生成；`window.__PRELOAD_AT` 这类调试量不进生产。
2. 预加载脚本会先于 mygo 的桥接脚本之外的一切页面脚本运行；`theme-init.js` 里要防 `undefined`，缺值时什么都不做（CSS 默认橙）。
3. 不能依赖 Paint Timing 做验收；首帧颜色用「第一次 rAF 时的计算色 + 截图像素」验证（T07 的 e2e 按这个写）。
4. 值只用 JSON 数据，不拼 HTML；即使值里含 `</script>` 也不影响（注入方式不是内联 `<script>`），编码仍然必要（引号、反斜杠、控制字符）。

## 未测与剩余风险

- 打包版（`mygo build`，`-tags mygo_nodownload,mygo_noinspector`）与开发版的注入行为是否一致：本次用 `go run`，打包标签只影响 FFI 下载器与 inspector，预计无差异；T07 在 e2e 构建（`GOFLAGS=-tags=e2e`）下再验一次。
- 多显示器 / 高 DPI 下首帧时序未单独测（本次截图受 DPI 缩放到 504×281，不影响结论）。
- 页面重载（F5 被生产构建禁用；主题切换不重载页面）未测，按注入机制每次文档加载都会重新执行。
- `PreloadAllFrames` 未启用，也不应启用（本应用页面无 iframe）。
