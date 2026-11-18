# golden 对照的有意差异清单

本清单登记 `golden_test.go` 中被允许与 `spikes/r3-render/legacy-out/` 不一致的用例，每条给出依据。
判定口径（docs/PLAN-M0.md T09）：除本清单外，其余用例必须与旧版一致；本清单内的用例在 `golden_test.go`
中计入 `intentional` 而不是 `mismatch`。

## D-004 / D-007 决定的契约差异（前端事件委托替代 href）

- `link-https`、`link-http`、`link-anchor`、`gfm-autolink`
  Go 输出 `data-kind` + `data-target`，不输出 `href`/`target`/`rel`；外链打开走 `Shell.OpenExternal`。
  依据：docs/decisions.md D-004、html-contract.md §2。

- `link-internal`、`link-dotdot`、`link-absolute`、`link-drive`、`link-unc`、
  `link-javascript`、`link-mailto`、`link-file`、`link-protocol-relative`
  chat 变体的 `<svg class="ext">` 外链图标不再随 HTML 输出——旧版在 ChatAnchor 里挂 svg，
  新版由前端按 `data-kind` 自绘。依据：html-contract.md §9 事件委托约定；spikes/r3-render/RESULT.md §4。

## 标题无 id（D-007，与旧版一致）

- `cm-heading-cn`、`cm-heading-cn-anchor`
  新版不生成 heading id，`#xx` 锚点点击无动作（旧版 `readerLinks.ts:31`）。
  spike 参考实现加了 CJK id 属「新增行为」，本包按裁决不生成。依据：D-007、PRD v1.4 FR-DOC。

## vault 图片的输出形态（契约：分类标记 + kbimg 协议）

- `img-vault`、`img-svg`
  旧版静态渲染只能输出 loading 卡 `<span data-state="loading">`；新版直接输出
  `<img data-img-kind="vault" data-vault-path src="http://kbimg.localhost/...">`，
  由 `Protocol.Handle("kbimg")` 供字节。依据：API.md §5、html-contract.md §7。

## internal-path 存在性探测的 stub 差异

- `link-internal`、`link-internal-dot`、`link-internal-frag`、`link-internal-encoded`
  旧版 spike 用 `vault.files` 探测存在性；本包把探测交给 `Options.Resolver`，
  golden 里的 stub resolver 与 spike 数据源不完全等价，`data-status` 随 stub 变。
  真实环境由 service 层 `ResolveLink` 统一判定，非渲染差异。依据：spikes/r3-render/RESULT.md §4。

## 任务列表 class

- `gfm-task`
  旧版 remark-gfm 输出 `<ul class="contains-task-list"><li class="task-list-item">`；
  goldmark 不产这两个 class，本包按 R-3 §9 建议不改渲染器、交由前端 CSS 用
  `:has(> input[type=checkbox])` 选择器或后续补充。normalise 时剥掉旧版 class 后结构一致。
  依据：spikes/r3-render/RESULT.md §4、§10.3。

## 说明

- `gfm-table-align`：旧版 `style="text-align:X"`，新版 `align="X"`——normalise 已把旧版映射为
  `align`，输出逐字节一致，不计入差异。
- `html-*`（原始 HTML）：本包按 D-007/PRD v1.4 把块级与行内原始 HTML 转义为文字显示，与旧版一致。
  spike 参考实现当时是「丢弃」，本包与之不同是有意的（裁决结果就是转义）。
- `cm-setext`、`fm-basic`、`fm-title-dash`：spike 里因 heading id 被列差异；本包不产 id，与旧版一致。
