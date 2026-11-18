// 对照集：每条给 id / 说明 / Markdown 输入 / variant / streaming。
// variant: 'reader' | 'chat' | 'both'（both 表示两个变体都跑）。
const cases = [
  // ---- CommonMark 基础 ----
  { id: 'cm-para', variant: 'both', note: '普通段落与软换行', md: '第一段。\n第二行软换行。\n\n第二段。' },
  { id: 'cm-headings', variant: 'both', note: '1-6 级标题', md: '# H1\n\n## H2\n\n### H3\n\n#### H4\n\n##### H5\n\n###### H6' },
  { id: 'cm-setext', variant: 'reader', note: 'Setext 标题（== 与 --）', md: 'Setext H1\n==\n\nSetext H2\n--' },
  { id: 'cm-heading-cn', variant: 'both', note: '中文标题锚点', md: '## 请求层设计\n\n跳到 [锚](#请求层设计)。' },
  { id: 'cm-heading-cn-anchor', variant: 'reader', note: '中文标题锚点链接', md: '## 部署步骤\n\n见 [部署](#部署步骤)。' },
  { id: 'cm-em-strong', variant: 'both', note: '粗斜与嵌套', md: '普通 **粗体** *斜体* ***粗斜*** __下划线粗__ _下划线斜_。' },
  { id: 'cm-inline-code', variant: 'both', note: '行内代码', md: '用 `npm run dev` 启动。`[[不是wikilink]]` 也是代码。' },
  { id: 'cm-code-fence-lang', variant: 'both', note: '围栏代码块带语言', md: '```js\nconst a = 1;\nconsole.log(a);\n```' },
  { id: 'cm-code-fence-nolang', variant: 'both', note: '围栏代码块无语言', md: '```\nplain code\n```' },
  { id: 'cm-code-indent', variant: 'reader', note: '缩进代码块', md: '    indented code\n    second line' },
  { id: 'cm-blockquote', variant: 'both', note: '引用块与嵌套', md: '> 引用第一行\n> 第二行\n>\n> - 列表\n> - 项二' },
  { id: 'cm-hr', variant: 'both', note: '水平线', md: '上文\n\n---\n\n***\n\n___\n\n下文' },
  { id: 'cm-ul', variant: 'both', note: '无序列表', md: '- 甲\n- 乙\n  - 乙一\n  - 乙二\n- 丙' },
  { id: 'cm-ol', variant: 'both', note: '有序列表与 start', md: '3. 第三\n4. 第四\n5. 第五' },
  { id: 'cm-list-nested', variant: 'reader', note: '列表嵌套段落与代码', md: '- 第一项\n\n  嵌套段落\n\n  ```\n  code\n  ```\n\n- 第二项' },
  { id: 'cm-hardbreak', variant: 'reader', note: '行尾两空格硬换行', md: '第一行  \n第二行' },
  { id: 'cm-escape', variant: 'reader', note: '反斜杠转义', md: '\\*不是斜体\\* \\`不是代码\\` \\[不是链接\\]' },

  // ---- GFM ----
  { id: 'gfm-table', variant: 'both', note: '基础表格', md: '| 名称 | 路径 |\n| --- | --- |\n| A | a.md |\n| B | b.md |' },
  { id: 'gfm-table-align', variant: 'reader', note: '表格对齐', md: '| 左 | 中 | 右 |\n| :-- | :-: | --: |\n| 1 | 2 | 3 |' },
  { id: 'gfm-table-wide', variant: 'chat', note: '超宽表格横滚', md: '| c1 | c2 | c3 | c4 | c5 | c6 |\n| -- | -- | -- | -- | -- | -- |\n| 很长很长很长的单元格内容一 | 很长很长很长的单元格内容二 | 很长很长很长的单元格内容三 | 很长很长很长的单元格内容四 | 很长很长很长的单元格内容五 | 很长很长很长的单元格内容六 |' },
  { id: 'gfm-task', variant: 'both', note: '任务列表', md: '- [x] 已完成\n- [ ] 未完成\n- [X] 大写也完成' },
  { id: 'gfm-strike', variant: 'both', note: '删除线', md: '这是 ~~删除线~~ 文字。' },
  { id: 'gfm-autolink', variant: 'both', note: '自动链接 www 与裸 URL', md: '访问 https://example.com/a?b=1 与 www.example.com。' },

  // ---- 原始 HTML（D-004：丢弃不转义）----
  { id: 'html-block-div', variant: 'both', note: '原始 HTML 块丢弃', md: '前文。\n\n<div class="x">块内文字</div>\n\n后文。' },
  { id: 'html-block-script', variant: 'both', note: 'script 块丢弃', md: '前文。\n\n<script>alert(1)</script>\n\n后文。' },
  { id: 'html-inline', variant: 'both', note: '行内 HTML 丢弃', md: '这是 <b>粗</b> 与 <em>斜</em> 行内标签。' },
  { id: 'html-comment', variant: 'reader', note: 'HTML 注释丢弃', md: '前 <!-- 注释 --> 后。' },
  { id: 'html-img-tag', variant: 'reader', note: '原始 <img> 标签丢弃', md: '正文 <img src="x.png" onerror="alert(1)"> 结束。' },

  // ---- frontmatter（D-006 原样复刻：当正文渲染）----
  { id: 'fm-basic', variant: 'both', note: 'frontmatter 当正文', md: '---\ntitle: 测试文档\ntags: [a, b]\n---\n\n正文第一段。' },
  { id: 'fm-title-dash', variant: 'reader', note: 'frontmatter 中 title 行 + --- 渲染成二级标题', md: '---\ntitle: 我的标题\n---\n\n正文。' },

  // ---- wikilink 三态与别名 ----
  { id: 'wiki-plain', variant: 'both', note: 'wikilink 基本形态（存在性由 stub 数据定）', md: '见 [[note-a]] 一节。', vault: { files: ['note-a.md'], resolve: { 'note-a': 'note-a.md' } } },
  { id: 'wiki-alias', variant: 'both', note: 'wikilink 别名', md: '见 [[note-a|别名文字]]。', vault: { files: ['note-a.md'], resolve: { 'note-a': 'note-a.md' } } },
  { id: 'wiki-missing', variant: 'both', note: 'wikilink 未创建', md: '见 [[不存在]]。', vault: { files: [] } },
  { id: 'wiki-ambiguous', variant: 'both', note: 'wikilink 多同名', md: '见 [[dup]]。', vault: { files: ['a/dup.md', 'b/dup.md'], ambiguous: ['dup'] } },
  { id: 'wiki-in-code', variant: 'both', note: '代码块内 wikilink 不转', md: '```\n[[not-a-link]]\n```\n\n`[[inline]]` 也不是。' },
  { id: 'wiki-unclosed', variant: 'reader', note: '未闭合 wikilink 当文字', md: '这是 [[未闭合 的文字。' },
  { id: 'wiki-empty', variant: 'reader', note: '空目标 wikilink 当文字', md: '空目标 [[ ]] 保留。' },

  // ---- 链接三分类 ----
  { id: 'link-https', variant: 'both', note: 'https 外链', md: '[官网](https://example.com/a?b=1#c)。' },
  { id: 'link-http', variant: 'reader', note: 'http 外链', md: '[旧站](http://example.com)。' },
  { id: 'link-anchor', variant: 'both', note: '#锚点', md: '跳 [到顶部](#请求层)。' },
  { id: 'link-internal', variant: 'both', note: '库内相对路径链接', md: '打开 [笔记](skills/react/a.md)。', vault: { files: ['skills/react/a.md'] } },
  { id: 'link-internal-dot', variant: 'reader', note: './前缀相对路径', md: '打开 [笔记](./raws/b.md)。' },
  { id: 'link-internal-frag', variant: 'reader', note: '库内路径带锚点', md: '打开 [笔记](a/b.md#标题)。' },
  { id: 'link-internal-encoded', variant: 'reader', note: '百分号编码路径', md: '打开 [笔记](raws/my%20note.md)。' },
  { id: 'link-dotdot', variant: 'both', note: '.. 上跳拒绝', md: '坏 [链接](../../outside.md)。' },
  { id: 'link-absolute', variant: 'both', note: '绝对路径拒绝', md: '坏 [链接](/etc/passwd)。' },
  { id: 'link-drive', variant: 'both', note: '盘符路径拒绝', md: '坏 [链接](C:/Windows/win.ini)。' },
  { id: 'link-unc', variant: 'both', note: 'UNC 路径拒绝', md: '坏 [链接](\\\\server\\share\\a.md)。' },
  { id: 'link-javascript', variant: 'both', note: 'javascript: 拒绝', md: '坏 [链接](javascript:alert(1))。' },
  { id: 'link-file', variant: 'reader', note: 'file: 拒绝', md: '坏 [链接](file:///C:/x)。' },
  { id: 'link-mailto', variant: 'both', note: 'mailto: 拒绝', md: '坏 [链接](mailto:a@b.c)。' },
  { id: 'link-protocol-relative', variant: 'reader', note: '// 协议相对拒绝', md: '坏 [链接](//evil.com/x)。' },

  // ---- 图片分类 ----
  { id: 'img-vault', variant: 'reader', note: '库内图片（reader 显示）', md: '![截图](raws/shots/a.png)。', vault: { files: ['raws/shots/a.png'] } },
  { id: 'img-vault-chat', variant: 'chat', note: '库内图片（chat 只显示标签可点）', md: '![截图](raws/shots/a.png)。' },
  { id: 'img-data', variant: 'reader', note: 'data:image 直接显示', md: '![内联](data:image/png;base64,iVBORw0KGgo=)。' },
  { id: 'img-data-chat', variant: 'chat', note: 'data:image 在 chat 不加载', md: '![内联](data:image/png;base64,iVBORw0KGgo=)。' },
  { id: 'img-https', variant: 'reader', note: 'https 远程图占位卡', md: '![远程](https://cdn.example.com/a.png)。' },
  { id: 'img-https-chat', variant: 'chat', note: 'https 远程图 chat 不加载', md: '![远程](https://cdn.example.com/a.png)。' },
  { id: 'img-http', variant: 'reader', note: 'http 明文图拦截', md: '![明文](http://example.com/a.png)。' },
  { id: 'img-dotdot', variant: 'reader', note: '图片 .. 逃逸拒绝', md: '![逃逸](../../secret.png)。' },
  { id: 'img-badext', variant: 'reader', note: '扩展名不在白名单', md: '![文本](a.txt)。' },
  { id: 'img-svg', variant: 'reader', note: 'svg 白名单内（显示白名单含 svg）', md: '![图标](icon.svg)。' },

  // ---- chat 变体引用标签 ----
  { id: 'chat-ref-path', variant: 'chat', note: 'chat 文本里的库内路径变引用标签', md: '建议看 skills/react/SKILL.md 与 raws/note.txt。' },
  { id: 'chat-ref-code', variant: 'chat', note: 'chat 行内代码整段是路径变标签', md: '请看 `docs/plan.md` 文件。' },
  { id: 'chat-ref-wiki', variant: 'chat', note: 'chat 里 [[x]] 变引用标签', md: '相关：[[note-a]]。', vault: { files: ['note-a.md'] } },

  // ---- 流式（chat streaming）----
  { id: 'stream-table-header', variant: 'chat', streaming: true, note: '流式：末尾只有表头行', md: '前文。\n\n| 分组 | Skill | 功能 |' },
  { id: 'stream-table-rows', variant: 'chat', streaming: true, note: '流式：表头+数据行无分隔行', md: '前文。\n\n| a | b |\n| 1 | 2 |\n| 3 | 4 |' },
  { id: 'stream-table-done', variant: 'chat', streaming: true, note: '流式：完整表格直通', md: '前文。\n\n| a | b |\n| --- | --- |\n| 1 | 2 |' },
  { id: 'stream-table-partial', variant: 'chat', streaming: true, note: '流式：分隔行到但末行不完整', md: '前文。\n\n| a | b |\n| --- | --- |\n| 1 | 2' },
  { id: 'stream-link-open', variant: 'chat', streaming: true, note: '流式：末尾未闭合链接', md: '前面 OK [示例](https://exa' },
  { id: 'stream-link-empty', variant: 'chat', streaming: true, note: '流式：[文字]( 无目标', md: '看 [链接](' },
  { id: 'stream-fence-open', variant: 'chat', streaming: true, note: '流式：未闭合围栏直通', md: '前文。\n\n```ts\nconst x = 1' },
  { id: 'stream-table-in-fence', variant: 'chat', streaming: true, note: '流式：围栏内的 | 不裁', md: '```\n| a | b |\n```' },
]


function casesFor(variant) {
  return cases.filter((c) => c.variant === variant || c.variant === 'both')
}

module.exports = { cases, casesFor }
