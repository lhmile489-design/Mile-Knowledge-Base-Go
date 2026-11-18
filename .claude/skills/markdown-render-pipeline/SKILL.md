---
name: markdown-render-pipeline
description: Mile Knowledge Base（Go 重写版）的 Markdown 渲染管线：Go 侧 goldmark（CommonMark + GFM）解析、goldmark/wikilink、goldmark/frontmatter、bluemonday 白名单清理，产出带 data-* 属性的 HTML；前端只做注入与事件委托（链接三分类、wikilink 三态、代码块语言标签与复制、宽表横滚、任务列表只读、阅读/对话两变体的图片规则）。对话流式按块增量渲染、按帧合并推送（PRD P-3、R-4）；阅读模式按 revision 缓存，>1 MiB 不渲染。只要在写或改渲染、链接/图片处理、对话增量渲染、阅读模式，或排查「某段渲染不对 / 与旧版不一致 / 有安全问题」，都应加载本 skill。
---

# markdown-render-pipeline

需求依据：PRD v1.1 的 FR-DOC（阅读渲染、阈值）、FR-LINK（链接规则）、FR-AI-CHAT（对话 Markdown 与流式）、10.2（安全）、10.1 P-3/P-5（流式性能）；技术设计 §2.3（选型）与 §4（关键链路）；decisions.md D-004（Go 侧 goldmark + bluemonday）。
视觉规格（代码块工具条、`.md-table-wrap`、`.img-label`、`.stream-pending`、`.pill` 等）以旧版 `DESIGN.md` §八/§九 与本项目 `design-tokens` skill 为准；本 skill 只管结构与行为。

## 1. 何时加载 / 何时不加载

- 加载：实现或修改 `internal/render`（goldmark、wikilink、frontmatter、bluemonday）、前端注入已渲染 HTML 的组件、对话流式渲染、阅读模式缓存与降级、链接/图片点击行为；排查「渲染与旧版不一致」「注入/链接点击不对」「疑似 XSS」。
- 不加载：窗口/绑定/事件通道（`mygo-web-app`）、路径授权与文件读写本身（`go-service-engineering`）、颜色字号令牌（`design-tokens`）、检索打分（FR-SRCH）。

## 2. 唯一行为基线：旧版

「与旧版一致」是本项目的验收口径（PRD G2）。渲染行为的权威来源是旧版源码与单测，不是「Markdown 通常怎么渲染」。逐条旧版依据放在 `references/old-behavior.md`，写任何规则前先查它。

最关键的基线结论（都已核对到 file:line）：

- **原始 HTML 转义成纯文本显示，不执行、也不丢弃**。R-3 实测（spikes/r3-render/RESULT.md §5）：旧版 react-markdown 10.1.0 未挂 rehype-raw 时，把 `<div>`/`<script>`/`<b>`/注释/`<img>` **转义为可见文字**（块级 HTML 不包 `<p>`，输出形如 `&lt;div class=&quot;x&quot;&gt;块内文字&lt;/div&gt;`，以 `legacy-out/reader/html-*.html` 为准）。**用户 2026-11-18 已裁决：新版复刻这一行为**——转义成文字显示，仍然不执行。不是「静默丢弃」，也不是把 HTML 喂给 DOM。
- **没有语法高亮、公式、图表**。旧版阅读与对话都没有（技术设计 2.3「代码高亮：不引入」）。代码块只显示语言标签与复制按钮（`CodeBlock.tsx`）。chroma、KaTeX、Mermaid 一律不引入。
- **reader 与 chat 是两个变体**，同一套解析、不同的输出覆盖。差异集中在：图片（reader 按 source 分类显示/占位，chat 一律 `.img-label` 不加载）、表格（chat 必须 `.md-table-wrap` 横滚；reader 全宽表格）、代码块工具条（chat 有 `.codeblock` 包装；reader 无右上角工具条）、引用标签（chat 把库内路径/wikilink 渲染成 `.chat-ref` 可点标签）。

## 3. Go 侧管线

```text
文档文本(已过编码/换行层)
  -> goldmark 解析（GFM + wikilink + frontmatter）
  -> 自定义 NodeRenderer：链接分类、wikilink 三态、代码块语言、图片分类、任务列表只读、块级结构
  -> bluemonday 白名单清理
  -> 返回 HTML + 元信息（hasTable/图片清单/截断标记…）
前端：dangerouslySetInnerHTML 注入（HTML 已经过白名单）+ 事件委托接 data-*
```

原则：前端**不再解析 Markdown**。交互（点击链接、复制、打开图片）全部通过 `data-*` 属性 + 事件委托落到 Go 侧的授权方法，不在前端复制判定逻辑。

### 3.1 依赖（已下载到模块缓存核对过）

| 用途 | 包 | 版本 |
| --- | --- | --- |
| 解析 | `github.com/yuin/goldmark` | v1.8.7 |
| GFM | goldmark 自带 `extension.GFM` | — |
| wikilink | `go.abhg.dev/goldmark/wikilink` | v0.6.0 |
| frontmatter | `go.abhg.dev/goldmark/frontmatter` | v0.3.0 |
| HTML 清理 | `github.com/microcosm-cc/bluemonday` | v1.0.27 |

### 3.2 解析器配置

```go
md := goldmark.New(
    goldmark.WithExtensions(
        extension.GFM,
        // 不挂 frontmatter.Extender：阅读模式要把 frontmatter 当正文渲染（PRD v1.3）
        &wikilink.Extender{Resolver: vaultResolver{}}, // Resolver 由本项目实现
    ),
    // 不挂 parser.WithAutoHeadingID()：旧版标题不产 id（用户 2026-11-18 裁决，对齐旧版）
    goldmark.WithRendererOptions(
        renderer.WithNodeRenderers(util.Prioritized(&kbRenderer{variant: v}, 100)),
        // 不加 html.WithUnsafe() —— 原始 HTML 不渲染；RawHTML/HTMLBlock 由自定义渲染器转义成文字
    ),
)
```

- 解析前把 CRLF 统一成 LF；BOM 已在读取层去掉。
- **阅读模式不剥离 frontmatter**：旧版把文档开头的 YAML 当正文渲染（PRD v1.3 FR-DOC，D-006 原样复刻）。所以渲染管线**不要**挂 `frontmatter.Extender`（它会把 frontmatter 从正文里拿掉）；frontmatter 解析只在索引侧单独做（kb-index-sqlite）。
- 库内图片与相对链接的路径**以知识库根为基准**，不以文档目录为基准（PRD v1.3）。
- frontmatter 只用来取 `title` 和 `tags`（FR-SRCH）；数据经 `frontmatter.Get(ctx).Decode(&meta)` 取。旧版只解析「扁平 `key: value` + tags（列表 / `[a,b]` / `a,b` 三种写法）」，扩展不解析嵌套结构。
- **标题不生成 id**（旧版不产 id，用户 2026-11-18 裁决对齐旧版）：不挂 `parser.WithAutoHeadingID()`，也不要写 CJK id 生成器。`#xx` 锚点链接保持旧版行为：渲染为 `.link-anchor`（`data-kind="anchor"`），**点击什么都不发生**（旧版 `readerLinks.ts:31` 注释「阅读模式不做滚动定位，点它什么都不发生」）。

### 3.3 wikilink：`[[目标]]` / `[[目标|别名]]`

goldmark/wikilink 已核对的 API（`wikilink@v0.6.0`）：

- `Parser`（`parser.go:53`）：inline 解析，`[[target]]`、`[[target|label]]`、`![[embed]]`；label 缺省等于 target；目标可含 `#fragment`（`parser.go:82` 拆 `Target` 与 `Fragment`）。必须 199 优先级（`parser.go:20`）以先于普通链接解析。
- `Node`（`ast.go:24`）：`Target []byte`、`Fragment []byte`、`Embed bool`。
- `Resolver` 接口（`resolver.go:21`）：`ResolveWikilink(*Node) (destination []byte, err error)`。返回 `nil, nil` 时渲染层「omit the link，渲染成普通文字」（`resolver.go:30`、`renderer.go:89`）。
- `Renderer`（`renderer.go:68`）：enter 时调 Resolver；有 destination 才输出 `<a href>`；`![[..]]` 且扩展名是图片时输出 `<img>`（`resolveAsImage`，`renderer.go:126`）。

本项目的 `vaultResolver` 必须按 FR-LINK 的三层解析实现：精确相对路径（可省略扩展名）→ 文件名 → 标题，**每层只有唯一命中才算解析**；多候选报「候选 N 个」，无候选报断链。这是业务规则，goldmark/wikilink 只负责语法。对应 UI 三态（旧版 `MarkdownCore.tsx:75-93`）：

- 解析成功 → 可点，标题 `[[目标]]`；
- 无候选 → 灰虚线下划线 + 后缀「（未创建）」（`wiki-missing`）；
- 多候选 → 灰虚线下划线 + 后缀「（有多个同名）」（`data-status="ambiguous"`）。

goldmark/wikilink 与旧版 `splitWikilinks` 的两处差异（R-3 §9/§10，用户 2026-11-18 裁决对齐旧版），在 `vaultResolver` / 渲染层回退处理：

- **空目标**：goldmark/wikilink 会把 `[[ ]]` 解析为 wikilink（`Target`=空白）；旧版 `splitWikilinks` 空目标当普通文字。渲染器对 `TrimSpace(Target)==""` 回退普通文字，不转 wikilink。
- **`#fragment`**：goldmark/wikilink 把 `[[a#frag]]` 拆 `Target=a` + `Fragment=frag`；旧版 `splitWikilinks` 不拆，`target` 整体是 `a#frag`。渲染器把 `Fragment` 拼回（`Target + "#" + Fragment`）再交给三层解析，保持旧版「整个 `a#frag` 当一个目标」的语义——`a#frag` 通常解析不到、走「未创建」，与旧版一致。

`![[embed]]` 旧版没有语义；goldmark/wikilink `Embed=true` 时按图片处理（`resolveAsImage`），走 §3.6 的同一条 src 分类（vault→图片，其它→占位/拦截）。

### 3.4 链接 href 三分类（旧版 `readerLinks.ts:36` `classifyHref`）

| href | 分类 | 点击行为 |
| --- | --- | --- |
| `#xx` | `anchor` | 页内锚点滚动 |
| `http(s)://` | `external` | 交 `Shell.OpenExternal` 系统浏览器打开 |
| 库内相对路径（去掉 `?`/`#` 后解码，不含 `..`/盘符/前导斜杠/UNC） | `internal-path` | 在应用内打开（仍走 pathguard 二次校验） |
| 其它一切：`javascript:` `file:` `mailto:` `data:` `..` 绝对路径 盘符 协议相对 `//x` | `invalid` | 渲染成普通文字，不可点（`link-plain`） |

输出形态：渲染器给 `<a>` 加 `data-kind` 与归一化后的 `data-target`，`invalid` 不输出 `href`。点击时前端把 `data-target` 发回 Go 侧 `ResolveLink`/打开文档，**不在前端再判一次路径**。

### 3.5 代码块、宽表、任务列表

- 围栏代码块：`<div class="codeblock" data-lang="js"><div class="tools"><span class="lang">js</span><button class="copy" data-code="...">复制</button></div><pre><code>…</code></pre></div>`。**复制的是代码原文**（`CodeBlock.tsx:6-8`），不是 DOM 文本——把原文放进 `data-code`（或前端从 `<code>` 取，二选一保持一致）。无语言时不显示 `.lang`，复制按钮仍在。reader 变体不需要 `.codeblock` 工具条（旧版 `BASE_COMPONENTS` 里没有 `pre` override），只保留 `<pre><code>`。
- 表格：chat 变体必须包 `.md-table-wrap`（`TableWrap.tsx`，`overflow-x` 横滚，不压缩列）。reader 变体全宽表格即可。
- 任务列表：`<input type="checkbox" disabled>`（`MarkdownCore.tsx:110`），只读，点击不改文件。**goldmark 不产旧版 task-list 的 class**：旧版（remark-gfm）输出 `<ul class="contains-task-list"><li class="task-list-item">`（以 `legacy-out/reader/gfm-task.html` 为准），goldmark 默认不产。R-3 §9：需自定义 `TaskListItem`/`List` 渲染补这两个 class，或前端 CSS 改用 `:has(> input[type=checkbox])` 选择器——实施时对照旧版 `tokens.css`/组件样式确认哪条更省。
- 行内代码：中性 `<code>`。chat 变体里「整段就是一个库内路径」的行内代码（`chatComponents.tsx:42`，`chatReferences.asPathReference`）要渲染成 `.chat-ref` 标签——这是 Go 侧在渲染行内 code 节点时做的判定，规则与 §4.3 同一条。

### 3.6 图片 src 分类（旧版 `imageSource.ts:91` `classifyImageSource`）

| src | 分类 | reader 行为 | chat 行为 |
| --- | --- | --- | --- |
| 库内相对路径（白名单扩展名 png/jpg/jpeg/gif/webp/svg/avif/bmp/ico） | `vault` | 经自定义协议加载显示 | 只显示 `.img-label`，可点击打开该图 |
| `data:image/<允许MIME>` | `data` | 直接显示 | 不加载，`.img-label` |
| `https://` 远程 | `remote` | 占位卡（host + 「加载这张图片」），点击后才拉 | 不加载，`.img-label` |
| `http://` 明文 | `remote` insecure | 拦截占位「不支持明文 HTTP」 | 同左 |
| 其它（`javascript:`/`file:`/非图片 data/`..`/绝对路径/UNC/超 12 MiB src/扩展名不在白名单） | `invalid` | 占位卡 + 中文原因（`describeImageReject`） | 同左 |

- **src 是语法判定不是安全边界**：真实读取仍走 pathguard 的 realpath containment（`imageSource.ts:159` 注释）。chat 里库内图片标签点击后也是一次打开文件，不是显示图。
- reader 的远程图片点击放行集合在前端维护（旧版 `imageStore.allowedRemote`），占位卡由前端根据 `data-image-*` 渲染，不是 Go 输出 `<img>`。
- **图片不再走 base64**：经 mygo `Protocol.Handle` 自定义协议按授权路径直接读字节（§5）。

### 3.7 渲染输出协议（Go → 前端）

返回结构至少包含：`html`（已清理）、`meta{ hasTable, hasWiki, images[] }`、阅读模式的 `revision`、流式块的 `pendingKind`。HTML 里约定俗成的 `data-*` 属性清单放 `references/html-contract.md`，前端事件委托只认这些属性。

### 3.8 bluemonday 白名单（唯一出口）

`references/whitelist.md` 有完整清单。要点：元素只放 `p h1-h6 a strong em del code pre blockquote ul ol li table thead tbody tr th td hr br img span div details summary input`；属性只放约定的 `data-*`、`id`、`class`、`href`、`src`、`alt`、`title`、`type/checked/disabled`、`align`、`start`；URL 协议只放 `http`、`https`、自定义图片协议、相对地址与 `#`；**禁止一切 `on*`、`style`、`iframe`、`object`、`embed`、`svg`（inline）、`script`**。白名单外的东西在到达前端前就消失——单测断言「渲染器原始输出过 bluemonday 前后一致」，不一致说明渲染器产出了白名单外的东西。

## 4. 对话流式增量渲染（P-3、R-4）

旧版每一帧对整条回答做一次完整 react-markdown 解析（P-3）。新版把成本挪到 Go 侧按块算：

- **按块切分**：工具调用、审批卡、回合结束处「封口」（旧版 `transcript.ts` 的合并语义）。已经封口的块只渲染一次并缓存，只有**末尾正在写的那一块**每帧重渲染。
- **末尾未完成结构先收起**：复刻 `chatStreaming.trimStreamingText` 的语义（旧版 `chatStreaming.ts:61`）——末尾连续 `|` 开头且还没有 `| --- |` 分隔行的块整体裁掉（除非它在未闭合代码围栏内），替换为 `.stream-pending` 占位「正在生成表格…」；末尾未闭合的 `[文字](` 保留文字部分、裁掉 `[` 前缀，占位「正在生成链接…」。未闭合围栏不用处理（CommonMark 自然延伸到文末）。已完成回合（streaming=false）直通不裁。
- **按帧合并推送**：token 在 Go 侧按帧合并后再推给前端（P-5），前端只更新正在输出的那一条消息，已完成消息不重解析、不重分组。

## 5. 图片协议：mygo `Protocol.Handle`

已核对 `mygo@v0.10.0/protocol.go`：

- `Protocol.Handle(scheme string, handler http.Handler)`（`protocol.go:53`）：注册自定义 scheme，返回 error；保留 scheme（http/https/file/data/blob/javascript/ws/wss/ftp/mailto/webkit）不能注册（`protocol.go:34`）。**必须在用到它的窗口创建之前注册**（`protocol.go:46`）。
- handler 是普通 `http.Handler`，在自己的 goroutine 上跑，响应流式回页面（`protocol.go:48`）。`req.Context()` 里能拿到 `CallerPage`/`CallerWindow`（`protocol.go:49`）。
- **Windows（WebView2）上的 URL 形式**：自定义 scheme 在页面里是 `http://<scheme>.localhost/...`（`protocol.go:204` `schemeHosts` 注释），不是 `<scheme>://...`。前端用的图片 URL 前缀要按这个形态写。
- 图片 handler：从 URL 取出路径/token → pathguard 校验属于本知识库且扩展名在图片白名单 → `http.ServeFile` / 手写 `ServeContent`，带正确 `Content-Type` 与 `Cache-Control`。不要用 `FileServer(os.DirFS(vaultRoot))` 直接映射整个库根——那会绕过逐文件的授权与扩展名白名单。
- scheme 建议用一个短名（如 `kbimg`）。`http:`/`https:` 不能注册，所以远程图片永远不走这个协议。

## 6. 阅读模式缓存与阈值

- 阅读模式整篇渲染一次，**按文档 revision 缓存**（FR-DOC 的 revision = 大小 + mtime + SHA-256）；revision 变才重渲染。
- 阈值（FR-DOC）：>1 MiB 不进阅读模式（阅读按钮禁用并说明原因）；>2 MiB 只读预览。渲染入口先查阈值再决定走不走管线。
- 渲染超时与取消：goldmark 本身不接受 ctx，把渲染放进 goroutine 用 `select` 等结果或 ctx 超时；病态输入（深嵌套、超长表）写进模糊测试。

## 7. 安全（PRD 10.2）

- 不执行文档里的 HTML 与脚本：渲染器把原始 HTML 转义成纯文本（不是丢弃、不是注入 DOM），bluemonday 白名单也没有 `script`/`on*`/`iframe`/`object`/`embed`/inline `svg`。
- URL 只放 `http`、`https`、自定义图片协议、相对路径、`#`；`javascript:` `file:` `data:`（图片 `data:image/*` 除外）一律拦截——`data:image` 只允许 `data:image/<白名单MIME>`（`imageSource.ts:129` `classifyDataUri`）。
- 模糊测试：`go test -fuzz` 对渲染入口断言不 panic、输出不含 `<script`、`on\w+=`、`javascript:`。恶意样本放 `testdata/malicious/`。

## 8. 一致性验证（R-3）

方法：从旧版 `tests/unit/` 抽 Markdown 输入样本建对照集（`readerLinks.test.ts`、`remarkWikilinks.test.ts`、`chatStreaming.test.ts`、`chatReferences.test.ts`、`links.test.ts`、`imageSource.test.ts`、`imagePresentation.test.ts`），喂给 Go 侧管线，比对**结构等价**（表格结构、任务列表只读、wikilink 三态、链接三分类、图片分类、流式裁剪结果），不是逐字节 HTML 一致。差异逐条记入 PLAN 并获用户确认。

## 9. 交付前自检

- [ ] 原始 HTML 转义成纯文本显示、不执行（与旧版一致，用户 2026-11-18 裁决）
- [ ] 标题无 id；`#xx` 锚点渲染为 `.link-anchor` 且点击不动作
- [ ] `[[ ]]` 空目标回退普通文字；`[[a#frag]]` 的 `a#frag` 整体当一个目标解析
- [ ] 无语法高亮 / 无公式 / 无图表，代码块只有语言标签 + 复制（复制的是原文）
- [ ] 链接三分类与 `classifyHref` 一致，invalid 渲染成普通文字
- [ ] wikilink 三态（解析/未创建/多同名）与 FR-LINK 的三层解析一致
- [ ] 图片分类与 `classifyImageSource` 一致；chat 一律 `.img-label` 不加载
- [ ] reader / chat 两变体差异齐全（表格横滚、codeblock 工具条、引用标签）
- [ ] bluemonday 白名单单测「清理前后一致」；无 `on*`/`script`/`javascript:`/inline `svg`
- [ ] 流式：封口块不重渲染、末尾未完成表格/链接收起、按帧合并推送
- [ ] 阅读模式按 revision 缓存；>1 MiB 不渲染并说明
- [ ] 图片走 `Protocol.Handle` 自定义协议（Windows 上是 `http://<scheme>.localhost`），不再 base64
