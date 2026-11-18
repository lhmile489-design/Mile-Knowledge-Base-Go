# bluemonday 白名单

只为本项目渲染器输出设计，不用 `UGCPolicy`（比需要的宽）。任何白名单外的元素/属性/协议在到达前端前就消失。

```go
p := bluemonday.NewPolicy()

// 元素
p.AllowElements("p","h1","h2","h3","h4","h5","h6","a","strong","em","del","code",
  "pre","blockquote","ul","ol","li","table","thead","tbody","tr","th","td",
  "hr","br","img","span","div","details","summary","input","button","article")

// 属性（按元素绑定）
p.AllowAttrs("href","data-kind","data-target","data-wiki","data-status","title","class",
  "data-ref-kind","data-ref-target").OnElements("a")
p.AllowAttrs("src","alt","title","class","data-state",
  "data-img-kind","data-vault-path","data-vault","data-src","data-host","data-secure").OnElements("img")
p.AllowAttrs("class").OnElements("h1","h2","h3","h4","h5","h6") // 标题无 id（R-3，用户 2026-11-18）
p.AllowAttrs("class","data-lang","data-display","data-scrollable","data-kind","data-status",
  "data-vault","data-img-kind","data-src","data-host","data-secure","data-reason","data-state",
  "data-ref-kind","data-ref-target","data-target","data-doc-path","data-msg-id",
  "aria-label","role","title").OnElements("span","div","article")
p.AllowAttrs("class").OnElements("pre","code","table","details","summary")
p.AllowAttrs("type","disabled","checked").OnElements("input")
p.AllowAttrs("type","class","data-ref-kind","data-ref-target","data-status","data-vault",
  "aria-label","title").OnElements("button") // chat-ref / img-label / 代码块复制按钮
p.AllowAttrs("align").OnElements("th","td")
p.AllowAttrs("start").OnElements("ol")

// URL 协议：http(s) + 自定义图片协议 + data:image/* + 相对路径 + #
p.AllowURLSchemes("http","https","kbimg","data") // data 仅 img src 用，见硬约束
p.AllowRelativeURLs(true)
```

## 硬约束

- `AllowURLSchemes` 里有 `data` 但**仅限 `img src`**（R-3 §9：`data:image/*` 的 src 会被 bluemonday 按 scheme 剥掉，必须放行 `data`）。`a`/`button` 上没有 `src`/`href` 之外的 URL 属性，`href` 不放 `data:`——安全边界靠渲染器只输出 `data:image/<白名单MIME>`（`imageSource.ts:129` `classifyDataUri`），`data:text/html` 等一律在渲染器侧就拦截。这是「白名单比语义宽」的折衷，已记录（R-3 §10.4）。
- `AllowURLSchemes` 里**没有** `javascript`、`file`、`vbscript`、`mailto`。
- 没有 `AllowStyles()`、`AllowAttrs(...).Globally()`、`Matching(regexp.MustCompile(".*"))` 这类放宽；`on*`、`style`、`iframe`、`object`、`embed`、inline `svg`、`script` 一律不在名单里。
- `<input>` 只允许 `type=checkbox disabled`（任务列表只读），没有别的 input。
- 图片协议名（示例 `kbimg`）写进白名单；协议名与 `Protocol.Handle` 注册的 scheme 一致。
- bluemonday 的 `AllowElements`/`AllowAttrs().OnElements()`/`AllowURLSchemes`/`AllowRelativeURLs` API 已核对 `bluemonday@v1.0.27/policy.go`。

## 自检单测

渲染器原始输出过 bluemonday 前后必须一致（输出本身就该只含白名单内容）；不一致说明渲染器产出了白名单外的东西，改渲染器不是改白名单。
