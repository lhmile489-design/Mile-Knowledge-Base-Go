# Go 输出 HTML 的结构契约（data-* 协议）

前端只做「注入 + 事件委托」，不再解析 Markdown、不再判路径。所有交互信号都放在 `data-*` 属性上。下面是 Go 渲染器必须产出的结构与前端对应行为；白名单见 `whitelist.md`，本契约里的每个属性都要在白名单里。

## 1. 顶层包裹

阅读模式：`<div class="reader"><article class="prose" data-doc-path="...">…</article></div>`（外层 `.reader` 负责滚动，`.prose` 管排版宽度，CSS 由 design-tokens 管）。
对话消息：`<div class="answer" data-msg-id="...">…</div>`，流式中末尾追加 `<div class="stream-pending" data-kind="table|link" role="status">正在生成…</div>`。

## 2. 链接 `<a>`

| 场景 | 输出 |
| --- | --- |
| 外链 | `<a class="link-external" data-kind="external" data-target="https://…">文字</a>`，点击 → `Shell.OpenExternal` |
| 锚点 | `<a class="link-anchor" data-kind="anchor" data-target="xx">`，点击 → 页内滚动 |
| 库内路径 | `<a class="wiki" data-kind="internal" data-target="dir/a.md">`，点击 → Go `ResolveLink` 后打开 |
| wikilink 已解析 | `<a class="wiki" data-kind="wiki" data-target="dir/a.md" data-wiki="a" title="[[a]]">` |
| wikilink 未创建 | 同上 + `class="wiki wiki-missing"` `data-status="missing"`，文字后追加 `<span class="wiki-note">（未创建）</span>` |
| wikilink 多同名 | `data-status="ambiguous"`，note 为 `（有多个同名）`，`title="有多个同名文件"` |
| invalid | `<span class="link-plain">文字</span>`（不输出 `<a>`，不可点） |

## 3. 引用标签（chat 变体）

`<a class="chat-ref" data-ref-kind="path|wiki" data-ref-target="…">label</a>`；`refKind`/`target` 必须成对出现才识别，缺一个按普通链接处理（旧版 `ChatAnchor.tsx:36`）。点击 → 探测存在后打开；不存在标「找不到」。

## 4. 代码块

```
<div class="codeblock" data-lang="js">
  <div class="tools"><span class="lang">js</span>
    <button type="button" class="copy" aria-label="复制代码">复制</button></div>
  <pre><code>原文…</code></pre>
</div>
```

- `data-lang` 取 `language-xxx` 的 xxx；无语言不输出 `.lang`，复制按钮仍在。
- 复制内容取 `<code>` 的文本（原文），点击后按钮变「已复制」2 秒。
- reader 变体不套 `.codeblock`，直接 `<pre><code class="language-js">`（保留 language class 便于将来，不显示工具条）。

## 5. 表格

chat：`<div class="md-table-wrap"><table>…</table></div>`；前端在 table 溢出 wrap 时给 wrap 加 `data-scrollable="true"`（量 `table.scrollWidth`，不是 wrap）。reader：`<table>` 全宽，不套 wrap。

## 6. 任务列表

`<input type="checkbox" disabled>`（勾选的再加 `checked`）。只读，点击不改文件。

## 7. 图片

Go 不在 HTML 里决定「显示还是占位」——它输出分类标记，前端按分类渲染：

```
<img data-img-kind="vault" data-vault-path="dir/a.png" alt="…">
<img data-img-kind="data"  src="data:image/png;base64,…" alt="…">
<span data-img-kind="remote" data-src="https://…" data-host="example.com" data-secure="true"  class="img-label-remote">…</span>
<span data-img-kind="blocked" data-reason="路径超出知识库范围" class="img-label-blocked">…</span>
```

- vault → 前端把 `src` 换成 `http://<scheme>.localhost/<token-or-path>`（自定义协议，见 SKILL §5）。
- remote insecure(http) → blocked，原因「不支持明文 HTTP」。
- chat 变体：所有图片一律 `<span class="img-label" data-vault="…(可选)">图片：<alt></span>`，库内路径加 `vault` 变体可点开，其它不可点。

## 8. 标题与锚点

标题 `<h1>` … `<h6>` **不带 `id`**（旧版不产 id，用户 2026-11-18 裁决对齐旧版；不挂 `parser.WithAutoHeadingID()`，也不要自定义 ID 生成器）。

`#xx` 锚点链接输出 `<a class="link-anchor" data-kind="anchor" data-target="xx">`（`data-target` 去掉 `#` 前缀），点击**什么都不发生**——旧版 `readerLinks.ts:31`「阅读模式不做滚动定位」。前端不要为 anchor 实现滚动。

## 9. 事件委托约定

前端在 `.reader` / `.answer` 容器挂一个 `click` 监听：

- `a[data-kind]` → 按 kind 分发（external→OpenExternal、anchor→无动作、internal/wiki→Go 打开）。
- `.chat-ref` → 探测后打开。
- `.codeblock .copy` → 复制 `<code>` 文本，按钮文案 2 秒切换。
- `[data-img-kind="remote"]` 的「加载这张图片」按钮 → 放行该 src 后重渲染为 `<img>`。
- `.img-label.vault` → 打开该库内图片文件。

**不在前端再做路径合法性判断**——所有 data-target 在 Go 输出时已分类，真正的授权在点击时由 Go 侧 pathguard 再做一次。
