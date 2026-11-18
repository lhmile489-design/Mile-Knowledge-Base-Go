# R-3 技术风险实测：goldmark 与旧版 react-markdown 渲染一致性

> 阶段 07 / R-3。结论先行：**goldmark + 自定义 NodeRenderer + bluemonday 可以达到与旧版结构一致**；106 个输出用例中 76 例规范化后逐字节一致、30 例有意差异（契约/stub/新增 CJK id/原始 HTML 口径）、0 例未解释结构差异。原始 HTML 的「丢弃 vs 转义」需要用户裁决。

## 1. 复现命令

```bash
# 1) 旧版输出（只读旧版 node_modules，不 npm install）
node spikes/r3-render/legacy/render-legacy.cjs        # 写 legacy-out/

# 2) 生成 cases.json
node -e "const {cases}=require('./spikes/r3-render/cases/cases.cjs');require('fs').writeFileSync('./spikes/r3-render/cases/cases.json',JSON.stringify(cases,null,2))"

# 3) 新版输出
cd spikes/r3-render/goimpl && go mod tidy && go build -o r3render.exe . && ./r3render.exe   # 写 go-out/

# 4) 对比
node spikes/r3-render/compare/compare.cjs             # 打印对照表 + 写 compare/report.json

# 5) 性能
cd spikes/r3-render/goimpl && go test -run TestPerf -v

# 6) 安全 fuzz
cd spikes/r3-render/goimpl && go test -fuzz=FuzzRender -fuzztime=120s .
```

## 2. 对照集与方法

- `cases/cases.cjs`：73 条输入，reader/chat 两变体共 106 份输出，覆盖 CommonMark 基础、GFM（表格/对齐/任务/删除线/自动链接）、嵌套列表、代码块、行内代码、引用、原始 HTML 块与行内、frontmatter、中文标题、wikilink 三态与别名、href 全分类（http/https/#锚点/库内相对/./前缀/带锚带查询/%编码/../ /绝对/盘符/UNC/javascript:/file:/mailto:/协议相对）、图片全分类（库内/data:image/https/http/svg/../ 逃逸/坏扩展名）、chat 引用标签、流式 7 种形态。
- legacy 脚本（`legacy/render-legacy.cjs`）通过**绝对路径 require 旧版 node_modules**，内联 `readerLinks.splitWikilinks`/`remarkWikilinks`、`remarkChatReferences.findReferences`、`imageSource.classifyImageSource`、`chatStreaming.trimStreamingText`（来源行号见文件头注释），组件结构对齐 `MarkdownCore`/`ChatAnchor`/`CodeBlock`/`TableWrap`/`ImageLabel`/`PreviewImage`/`ChatReference`。
- wikilink/链接存在性探测用 stub（`case.vault.files/resolve/ambiguous`），两版同一份输入。
- 对比脚本规范化：剥 React 布尔属性/自闭合、属性排序、`style=text-align`→`align`、旧版 `contains-task-list`/`task-list-item` class、旧版 `<a href>` 映射到 `data-kind/data-target`、剥 chat `<svg class="ext">`、剥 `data-code`/`data-ext`/`data-state`。

## 3. 总体结果

| 类别 | 数量 | 占比 |
| --- | --- | --- |
| 一致（规范化后逐字节相同） | 76 | 71.7% |
| 有意差异（有依据） | 30 | 28.3% |
| 结构差异（未解释） | 0 | 0% |
| 缺输出 | 0 | — |

逐例见 `compare/report.json` 与命令输出。

## 4. 有意差异清单（30 例）

| 差异 | 例数 | 说明 |
| --- | --- | --- |
| Go 输出 `data-target` + 事件委托，不输出 `href` | 5 | D-004 契约：`html-contract.md` §2，前端 click 分发替代 href |
| chat 变体 `<svg class="ext">` 外链图标 | 8 | 旧版 `ChatAnchor.tsx:47` 在 a 内挂 svg；Go 契约交给前端按 `data-kind` 渲染图标 |
| 新增 heading id（含 CJK） | 6 | Go 生成 `id="请求层设计"`；旧版 react-markdown 不产 id，锚点本就不可达——**新增行为**，为 `#中文标题` 锚点提供落点 |
| 原始 HTML 处理 | 9 | 见 §5 重点核对 |
| `contains-task-list`/`task-list-item` class | 1（×2） | remark-gfm 产 class，goldmark 不产；旧版 CSS 可能依赖，需前端补选择器或自定义 TaskListItem 渲染 |
| 表格 `style="text-align:X"` vs `align="X"` | 1 | goldmark 输出 `align` 属性，白名单已放行；语义等价 |
| 库内图片 `<img data-img-kind="vault">` vs PreviewImage loading 卡 | 2 | 契约：Go 输出分类标记，前端经自定义协议加载；legacy 静态渲染只能输出 loading 卡 |
| internal-path 存在性探测 stub | 4 | 两边 stub 数据源不同（`vault.files`），`missing/ok` 随 stub 变；真实环境由 Go 侧 `ResolveLink` 统一判定，不是渲染差异 |

## 5. 重点核对结论

| 核对项 | 结果 |
| --- | --- |
| 原始 HTML | **旧版实际把 `<div>`/`<script>`/`<b>`/注释/`<img>` 转义为文本显示**（react-markdown 10.1.0 默认行为，实测 `legacy/render-legacy.cjs` 输出），与 `old-behavior.md` §1「被丢弃不产出」**不一致**；Go 侧按 skill 契约实现为丢弃。两者都满足「不渲染」。需用户裁决：复刻「转义显示」还是接受「丢弃」。 |
| frontmatter | 一致：两边都把 `---\ntitle: x\n---` 当正文渲染成 `<hr> + <h2>title: x</h2>`（D-006）。Go 额外给 h2 加 CJK id（有意新增）。 |
| 表格 / 任务列表 / 代码块 | 结构一致；任务复选框 `disabled`、代码块 `language-x` + chat `.codeblock` 工具条、chat `.md-table-wrap` 均对齐。 |
| 链接三分类 | 一致：external/anchor/internal-path 输出对应 `data-kind`，invalid 全部 `<span class="link-plain">`，javascript:/file:/mailto:/../ /绝对/盘符/UNC/协议相对全部被拦截为普通文字。 |
| wikilink 三态 | 一致：resolved/missing/ambiguous 三态、`（未创建）`/`（有多个同名）` note、`data-wiki`/`data-status`；chat 变体渲染成 `chat-ref` button（旧版 ChatAnchor 行为）。goldmark/wikilink 会解析 `[[ ]]` 空目标而旧版不转——已在渲染器按空目标回退普通文字处理（`render.go renderWikilink`）。 |
| 危险 href | 一致拦截：`javascript:`/`file:`/`mailto:`/`data:`（链接里）/`..`/绝对路径/盘符/UNC/`//` 全部 `link-plain`。 |
| chat 引用标签 | 一致：`skills/x.md` 文本与 `` `docs/plan.md` `` 行内代码整段路径都渲染成 `<button class="chat-ref" data-ref-kind>`，`data-status` 由存在性探测。 |
| 流式裁剪 | 一致：7 个流式形态（表头/表行无分隔/完整表/分隔到但末行不完整/未闭合链接/未闭合围栏/围栏内表格）`trimStreamingText` 与 `pendingKind` 全对齐。 |

## 6. 性能数字（本机，go1.27.3 windows/amd64）

| 场景 | 耗时 |
| --- | --- |
| 渲染 100 KB 文档（reader） | best 19.4ms / avg 24.4ms |
| 渲染 1 MiB 文档（reader） | best 200.2ms / avg 202.9ms（>1 MiB 不进阅读模式，仅参照上限） |
| 流式末尾块 800 B 单帧（chat，只重渲染末尾块） | best ~0 / avg 350µs（50 次） |
| 整篇 10 KB chat 单帧 | best 1.0ms / avg 3.5ms |

结论：阅读模式按 revision 缓存，100 KB 单次 ~20ms 可接受；流式「只重渲染末尾块」单帧 ~350µs，对 60fps（16ms）预算充裕，且远优于旧版每帧 6.7–9.9ms 的全文解析（P-3 预期收益兑现）。

## 7. 安全 fuzz

`go test -fuzz=FuzzRender -fuzztime=120s`：**2 分钟 / 1,051,438 次执行 / 0 panic / 0 泄漏**（897 个语料）。断言：输出不含 `<script`、标签内 `on\w+=`、可执行位置（href/src/data-target/data-src/data-vault*）的 `javascript:`。文本位置出现 "javascript:"/"on0=" 这类不可点字符串不算泄漏。

## 8. 需要新增的自定义渲染器（实施清单）

在 `internal/render` 需要以下自定义 `NodeRenderer`（本 spike 已验证全部可行）：

1. `ast.KindLink` — `classifyHref` 三分类 + `data-kind/data-target` + invalid → `link-plain` + chat `data-ref-*` 引用按钮。
2. `wikilink.Kind` — `vaultResolver` 三层解析（精确/文件名/标题）+ 三态 + 空目标回退普通文字 + chat `chat-ref`。
3. `ast.KindHeading` — CJK heading id 生成器（保留 Unicode 字母数字、空白/`-`/`_`→`-`、重复加 `-N`）。
4. `ast.KindAutoLink` — 同 link 分类。
5. `ast.KindImage` — `classifyImageSource` 五分类（vault/data/remote-secure/remote-insecure/invalid），reader `data-img-kind` 标记、chat `img-label`。
6. `extast.KindTaskCheckBox` — `<input type="checkbox" disabled>`。
7. `ast.KindFencedCodeBlock`/`CodeBlock` — `language-x` + chat `.codeblock` 工具条 + `data-code` 原文。
8. `extast.KindTable` — chat `.md-table-wrap`。
9. `ast.KindCodeSpan` — chat 整段路径 → `chat-ref`。
10. `ast.KindRawHTML`/`HTMLBlock` — 丢弃或转义（§5 裁决后定）。
11. `transformChatRefs` AST 变换 — text 节点里的库内路径切 link 节点（`data-ref-kind/target`），跳过 `CodeSpan`。
12. `trimStreamingText` — 流式末尾表格/链接裁剪（原样移植，已验证逐行一致）。

## 9. markdown-render-pipeline skill 需修正项（不直接改 skill）

| 位置 | 现状 | 实测结论 | 建议 |
| --- | --- | --- | --- |
| `references/old-behavior.md` §1 | 「原始 HTML 被静默丢弃，不是转义展示」 | react-markdown 10.1.0 **转义为文本显示**（`<div>`→`&lt;div&gt;`）；不是丢弃 | 改为「转义为纯文本展示，不执行」；同时裁决 Go 侧是转义还是丢弃 |
| `SKILL.md` §2 / §7 | 同上说「丢弃」 | 同上 | 同步修正 |
| `SKILL.md` §3.2 `parser.WithAutoHeadingID()` | goldmark 默认 ID 丢弃非 ASCII | 实测 `## 请求层设计` → `id="heading"`；且**旧版根本不产 heading id** | 写明：不挂 `WithAutoHeadingID`（对齐旧版无 id），或挂自定义 CJK IDs 需 `parser.WithContext`/渲染层自定义（本 spike 走渲染层自定义） |
| `SKILL.md` §3.3 | 未提空目标 | goldmark/wikilink 把 `[[ ]]` 解析为 wikilink（Target=空格）；旧版 `splitWikilinks` 不转空目标 | 补：渲染器对 `TrimSpace(Target)==""` 回退普通文字 |
| `SKILL.md` §3.5 / `whitelist.md` | task list 只写 `<input disabled>` | 旧版 `<ul class="contains-task-list"><li class="task-list-item">`，goldmark 无此 class | 补：需自定义 `TaskListItem`/`List` 渲染补 class，或前端 CSS 改选择器 |
| `whitelist.md` | 无 `button`、`article`、`data-state`、`aria-label`、`type`（on a/button） | chat-ref/img-label 是 `<button>`；`data-state`/`aria-label`/`type` 是旧版结构必需 | 补 `button`、`article`、`data-state`、`aria-label`、`type`；`data:` scheme 需在白名单加 `data`（img src 是 `data:image/*`），否则 src 被剥 |
| `SKILL.md` §4 / 流式 | 无 issue | `trimStreamingText` 原样移植可行，逐例一致 | 无 |
| `references/old-behavior.md` §3 | `classifyHref` 分类 | 一致 | 无 |

## 10. 剩余风险与待用户裁决

1. **原始 HTML 口径（裁决点）**：旧版实测「转义为文本显示」，skill 文档与 Go 实现均为「丢弃」。两者都安全，但用户看到的界面不同（旧版能看到 `<div class="x">` 文字，新版直接没有）。需用户裁决：原样复刻转义，还是接受丢弃（更干净）。
2. **中文锚点**：旧版不产 heading id，`#中文标题` 链接在旧版本就不可达；Go 生成 CJK id 后变为可达——行为更优但**是新增**。若严格对齐应关掉 id；建议保留并在 PRD/PLAN 记为 P1 增强。
3. **task-list class**：goldmark 不产 `contains-task-list`/`task-list-item`，旧版 CSS 若按这两个类选择子项间距，前端需补自定义渲染或改 CSS 选择器（实施时对照旧版 `tokens.css`/组件样式确认）。
4. **`data:` 图片白名单**：bluemonday `AllowURLSchemes` 需含 `data` 才能保留 `data:image/*` src——放行的是整个 `data:` scheme；安全边界依赖渲染器只输出 `data:image/<白名单MIME>`（已验证），但属「白名单比语义宽」的折衷，写进决策。
5. **`![[embed]]`**：旧版无语义；goldmark/wikilink `Embed=true` 时按图片处理（`resolveAsImage`），本 spike 未覆盖 embed 用例，实施时补一条。
6. **wikilink `#fragment`**：goldmark/wikilink 会把 `[[a#frag]]` 拆 Target/Fragment；旧版 `splitWikilinks` 不拆，`[[a#frag]]` 的 target 整体是 `a#frag`。差异待实施时裁决（本对照集未含此例）。
