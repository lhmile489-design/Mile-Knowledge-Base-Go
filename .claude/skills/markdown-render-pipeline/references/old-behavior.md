# 旧版渲染行为逐条依据

每条都给旧版 file:line。写 Go 侧规则时以这里为准；本文件与 PRD/技术设计冲突时，以 PRD 为准并在 PLAN 标注。

旧版根：`E:\WebApp\Mile-Project\Mile-Knowledge-Base-master`（只读，不修改）。
相对路径默认相对 `src/`。

## 1. 渲染底座与原始 HTML

- `renderer/src/components/markdown/MarkdownCore.tsx:15`：注释「输出 React 元素，不挂 rehype-raw（模型与外部正文都是不可信输入）」。
- `renderer/src/components/workbench/MarkdownReader.tsx:8`：阅读模式同样「不挂 rehype-raw」。
- `renderer/src/components/agnes/ChatMarkdown.tsx:24-26`：对话区「绝不用 dangerouslySetInnerHTML」。
- **R-3 实测修正（2026-11-18 裁决）**：react-markdown 10.1.0 未挂 rehype-raw 时，原始 HTML 的实际行为是**转义为纯文本显示**，不是丢弃。实测输出（`spikes/r3-render/legacy-out/reader/html-*.html`）：`<div class="x">块内文字</div>` → `&lt;div class=&quot;x&quot;&gt;块内文字&lt;/div&gt;`；`<script>alert(1)</script>` → `&lt;script&gt;alert(1)&lt;/script&gt;`；行内 `<b>`/`<em>`、HTML 注释、`<img onerror>` 同样转义为文字。**块级 HTML 不包 `<p>`**（独立成行）；行内 HTML 在所在段落内转义。
- 结论：新版 Go 侧渲染把文档里的原始 HTML **转义成纯文本显示、不执行**（块级与行内、注释都算），复刻旧版。不渲染、不注入 DOM、也不静默丢弃。这与「不渲染原始 HTML」（FR-DOC）一致——显示的是转义文字不是元素。

## 2. 共用与变体

- `MarkdownCore.tsx:30` `MarkdownVariant = 'reader' | 'chat'`；`:127` `PLUGINS = [remarkGfm, remarkWikilinks]`；`:150` chat 追加 `remarkChatReferences`。
- `BASE_COMPONENTS`（`:99`）：`a` → CoreAnchor；`input` → checkbox 只读。
- `READER_COMPONENTS`（`:120`）：`img` → PreviewImage。
- `CHAT_COMPONENTS`（`chatComponents.tsx:22`）：`a`→ChatAnchor、`code`→整段路径变 ChatReference 否则 `.chat-code`、`img`→ImageLabel、`table`→TableWrap、`pre`→CodeBlock。
- 差异汇总：reader 图片走 source 分类显示/占位；chat 图片一律 `.img-label` 不加载。chat 表格 `.md-table-wrap` 横滚；reader 全宽。chat 代码块 `.codeblock` 工具条；reader 纯 `<pre><code>`。chat 行内代码整段是库内路径时变 `.chat-ref` 标签。
- **标题无 id**：旧版 react-markdown 不给标题生成 `id`（实测 `legacy-out/reader/cm-headings.html`、`cm-heading-cn.html` 均为裸 `<h1>`/`<h2>`）。R-3 裁决（用户 2026-11-18）：新版同样不生成 id，不挂 `parser.WithAutoHeadingID()`，不写 CJK id 生成器。

## 3. 链接三分类（`shared/readerLinks.ts`）

- `classifyHref`（`:36`）：`#`→anchor；`http(s)`→external；其它带 scheme→invalid；去 `?`/`#` 解码后含前导 `/`、盘符 `X:\`、`..` 段→invalid；否则 internal-path（去掉 `./` 前缀）。`javascript:`/`file:`/`mailto:`/`data:`/协议相对 `//` 均 invalid → `link-plain` 普通文字（`MarkdownCore.tsx:70`）。
- **anchor 点击无动作**：`readerLinks.ts:31` 注释「文档内锚点（`#xx`），阅读模式不做滚动定位，点它什么都不发生」。`#xx` 渲染为 `.link-anchor`，点击不滚动（标题无 id，本就无落点）。
- `splitWikilinks`（`:77`）：`[[目标]]`/`[[目标|别名]]`，目标禁 `]` `|` 换行，空目标当普通文字。
- **wikilink 不拆 `#`**：`splitWikilinks` 的 target 是 `[[...]]` 整体 inner text，`[[a#frag]]` 的 target 就是 `a#frag`（不拆 Target/Fragment）。与 goldmark/wikilink 默认拆 `#` 不同——渲染器要把 `Fragment` 拼回 `Target` 再解析（R-3 §10.6）。
- 多候选：`MarkdownCore.tsx:75-93` `ambiguous`/`missing` → `wiki-missing` 灰虚线 + `（有多个同名）`/`（未创建）`。
- wikilink 目标解析顺序（`shared/links.ts:188` `resolveLink`）：精确相对路径(可省略扩展名)→文件名→标题；每层唯一命中才解析，多候选报 `candidates` 数。FR-LINK 与此一致。

## 4. 链接图抽取（`shared/links.ts`）

- `extractLinks`（`:79`）：按行扫 + 围栏状态，行内 code 用等长空白剔除（`:106` `blankOutInlineCode`），代码块与行内代码里的 `[[x]]`/`](..)` 不抽取。
- `collectMarkdownLinks`（`:154`）：图片（前导 `!`）不算链接；空目标跳过；target 允许成对括号。
- `classifyUrl`（`:175`）：http(s)→external；其它 scheme→other（不可打开）；否则 internal。

## 5. 代码块 / 表格 / 任务列表 / 图片

- `CodeBlock.tsx`：`.codeblock` 包装 + `.tools`（`.lang` 语言名 + `.copy` 复制）；**复制代码原文不是 DOM 文本**（`:6-8`）；`copied` 2 秒复位；无语言不显示 `.lang` 但复制按钮仍在。
- `TableWrap.tsx`：chat 表格 `.md-table-wrap`，`overflow-x` 横滚不压缩列；`data-scrollable` 标记溢出（量 `table.scrollWidth` 不是 wrap）。
- 任务列表：`MarkdownCore.tsx:110` `<input type=checkbox disabled readOnly>`，只读。remark-gfm 额外给容器加 class：`<ul class="contains-task-list"><li class="task-list-item">`（实测 `legacy-out/reader/gfm-task.html`）；goldmark 默认不产这两个 class，需自定义 List/TaskListItem 渲染补回或前端改 CSS 选择器（R-3 §9）。
- `imageSource.ts:91` `classifyImageSource`：先剥 `<>` 包裹与 title（`:96`、`:123` `stripTitle`）；`data:`→只放 `data:image/<白名单MIME>`；scheme 白名单只 http/https；`//x`→remote(https)；相对路径逐段 percent-decode 后查 `.`/`..`/盘符/UNC/前导斜杠；扩展名白名单 png/jpg/jpeg/gif/webp/svg/avif/bmp/ico；src 超 12 MiB→invalid。
- `imagePresentation.ts` `presentImage`：vault→读字节显示/loading/blocked；data→直接显示；remote insecure(http)→blocked「不支持明文 HTTP」；remote https 未放行→remote-gate（host + 「加载这张图片」+ 「默认不加载：请求会让对方服务器知道你此刻读了这篇笔记」），已放行→show。
- `ImageLabel.tsx`（chat）：库内相对路径 → 可点 `.img-label vault`（图标 + `图片：<alt>` + `打开 <path>`）；其它 src → 不可点 `.img-label` + `（远程图片不加载）`。判定先排除 scheme 与 `//`（`:32` `asVaultPath`），再 `isPlausibleRelativePath`。
- `markdownUrlTransform.ts`：reader 只放行 `data:image/`，其余走默认过滤；chat 连 `data:` 也不放行（`chatUrlTransform`）。

## 6. 对话流式（`shared/chatStreaming.ts`）

- `trimStreamingText`（`:61`）：仅 streaming=true 生效。
  - 末尾连续 `|` 开头行块（可穿过空行），先确认不在未闭合围栏内（`lastOpenFence` `:35`）：
    - 无 `| --- |` 分隔行 → 整块裁掉，`pendingKind='table'`；
    - 有分隔行但最后一行不以 `|` 收尾 → 只裁最后一行；
    - 完整表格直通。
  - 末尾 `\[xxx]\(` 未闭合（不在围栏内）→ 裁掉 `[` 前缀与 `(`，保留文字，`pendingKind='link'`。
  - 未闭合围栏、行内 code、粗体不处理（CommonMark 自然延伸）。
- `streamPendingLabel`（`:128`）：table→「正在生成表格…」；link→「正在生成链接…」。占位是 `.stream-pending` + `role="status"`（`ChatMarkdown.tsx:60`）。

## 7. 引用标签（`shared/chatReferences.ts`、`remarkChatReferences.ts`）

- `findReferences`（`:53`）：`[[wikilink]]` 与库内相对路径（扩展名只 md/markdown/txt）→ `path`/`wiki` 两类 `RefMatch`；wikilink 内的路径不重复认。
- `asPathReference`（`:44`）：整段文本就是一个路径才认（行内代码用）。
- `remarkChatReferences.ts`：只处理 text 节点，link 子树不转；产出 `data-ref-kind`/`data-ref-target` link 节点。
- `ChatAnchor.tsx`：`refKind/target` 成对出现 → ChatReference；`data-wiki` → ChatReference；否则 CoreAnchor + 外链图标。
- 旧版证据雷达（`EvidenceRadar`）：汇总引用次数与首次序号，不在本 skill 范围（FR-AI-CHAT 引用节）。

## 8. 阈值与安全（FR-DOC、PRD 10.2）

- >1 MiB 不进阅读模式；>2 MiB 只读预览；单次写上限 2 MiB。
- 图片按文件头识别 png/jpg/gif/webp/bmp/avif（不收 SVG/ICO —— 注意 `imageSource.ts` 的扩展名白名单含 svg/ico 是给「显示已有引用」的，**保存粘贴图片**时 `saveImage`/attachments 的白名单才是不收 svg/ico；两条规则分开）。
- 粘贴图片保存到 `raws/attachments/pasted-<sha256前12>.<ext>`，上限 8 MiB。
- `markdownUrlTransform` 的 `data:image` 只允许图片 MIME。
