// 最小渲染器：goldmark + GFM + wikilink + bluemonday。
// 行为对齐旧版 MarkdownCore / ChatMarkdown 与 markdown-render-pipeline skill。
// 两变体 reader/chat，输出带 data-* 属性的 HTML。
package main

import (
	"bytes"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	wikilink "go.abhg.dev/goldmark/wikilink"
)

// ---------- 案例（与 cases.cjs 同步，用 JSON 读入） ----------

type vaultStub struct {
	Files     []string          `json:"files"`
	Resolve   map[string]string `json:"resolve"`
	Ambiguous []string          `json:"ambiguous"`
}

type testCase struct {
	ID        string     `json:"id"`
	Variant   string     `json:"variant"`
	Note      string     `json:"note"`
	MD        string     `json:"md"`
	Streaming bool       `json:"streaming"`
	Vault     *vaultStub `json:"vault"`
}

// ---------- wikilink resolver（FR-LINK 三层解析的 spike 版）----------

type vaultResolver struct {
	v *vaultStub
}

// ResolveWikilink 返回目标路径；无法解析返回 nil,nil（渲染成普通文字由自定义渲染器处理三态）。
func (r vaultResolver) ResolveWikilink(n *wikilink.Node) ([]byte, error) {
	target := string(n.Target)
	if r.v == nil {
		return nil, errAmbiguous // 无库信息：交自定义逻辑判
	}
	for _, a := range r.v.Ambiguous {
		if a == target {
			return nil, errAmbiguous
		}
	}
	t := target
	if p, ok := r.v.Resolve[target]; ok {
		t = p
	}
	cands := []string{t}
	for _, ext := range []string{".md", ".markdown", ".txt"} {
		if !strings.HasSuffix(strings.ToLower(t), ext) {
			cands = append(cands, t+ext)
		}
	}
	files := map[string]bool{}
	for _, f := range r.v.Files {
		files[f] = true
	}
	for _, c := range cands {
		if files[c] {
			return []byte(c), nil
		}
	}
	return nil, errMissing
}

var (
	errAmbiguous = fmt.Errorf("ambiguous")
	errMissing   = fmt.Errorf("missing")
)

// ---------- 链接三分类（readerLinks.ts:36 classifyHref）----------

var (
	schemeRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:`)
	httpRe   = regexp.MustCompile(`^https?://`)
	driveRe  = regexp.MustCompile(`^[a-zA-Z]:[\\/]`)
)

type linkKind int

const (
	lkInvalid linkKind = iota
	lkExternal
	lkAnchor
	lkInternal
)

func classifyHref(raw string) (linkKind, string) {
	href := strings.TrimSpace(raw)
	if href == "" {
		return lkInvalid, ""
	}
	if strings.HasPrefix(href, "#") {
		return lkAnchor, href[1:]
	}
	if httpRe.MatchString(href) {
		return lkExternal, href
	}
	if schemeRe.MatchString(href) {
		return lkInvalid, href
	}
	withoutFragment := strings.SplitN(strings.SplitN(href, "#", 2)[0], "?", 2)[0]
	decoded := safeDecode(withoutFragment)
	decoded = strings.ReplaceAll(decoded, "\\", "/")
	if decoded == "" {
		return lkAnchor, href
	}
	if strings.HasPrefix(decoded, "/") || driveRe.MatchString(decoded) {
		return lkInvalid, href
	}
	for _, seg := range strings.Split(decoded, "/") {
		if seg == ".." {
			return lkInvalid, href
		}
	}
	target := decoded
	if strings.HasPrefix(target, "./") {
		target = target[2:]
	}
	return lkInternal, target
}

func safeDecode(s string) string {
	d, err := url.PathUnescape(s)
	if err != nil {
		return s
	}
	return d
}

// ---------- 图片 src 分类（imageSource.ts classifyImageSource）----------

var imageExtensions = map[string]string{
	"png": "image/png", "jpg": "image/jpeg", "jpeg": "image/jpeg",
	"gif": "image/gif", "webp": "image/webp", "svg": "image/svg+xml",
	"avif": "image/avif", "bmp": "image/bmp", "ico": "image/x-icon",
}

const maxSrcLength = 12 * 1024 * 1024

type imgKind int

const (
	imgInvalid imgKind = iota
	imgVault
	imgData
	imgRemote
)

type imgSource struct {
	kind    imgKind
	path    string
	src     string
	host    string
	secure  bool
	reason  string
}

var titleRe = regexp.MustCompile(`^(\S*)\s+(?:"[^"]*"|'[^']*'|\([^)]*\))$`)

func classifyImageSource(raw string) imgSource {
	if len(raw) > maxSrcLength {
		return imgSource{kind: imgInvalid, reason: "too-long"}
	}
	src := strings.TrimSpace(raw)
	if strings.HasPrefix(src, "<") && strings.HasSuffix(src, ">") {
		src = src[1 : len(src)-1]
	}
	if m := titleRe.FindStringSubmatch(src); m != nil {
		src = m[1]
	}
	src = strings.TrimSpace(src)
	if src == "" {
		return imgSource{kind: imgInvalid, reason: "empty"}
	}
	if strings.ContainsRune(src, 0) {
		return imgSource{kind: imgInvalid, reason: "path-escape"}
	}
	if strings.HasPrefix(strings.ToLower(src), "data:") {
		m := regexp.MustCompile(`(?i)^data:([^;,]*)[;,]`).FindStringSubmatch(src)
		if m == nil {
			return imgSource{kind: imgInvalid, reason: "not-an-image"}
		}
		mime := strings.ToLower(strings.TrimSpace(m[1]))
		ok := false
		for _, v := range imageExtensions {
			if v == mime {
				ok = true
				break
			}
		}
		if !ok {
			return imgSource{kind: imgInvalid, reason: "not-an-image"}
		}
		return imgSource{kind: imgData, src: src}
	}
	if m := regexp.MustCompile(`^([A-Za-z][A-Za-z0-9+.-]*):`).FindStringSubmatch(src); m != nil {
		proto := strings.ToLower(m[1])
		if proto != "http" && proto != "https" {
			return imgSource{kind: imgInvalid, reason: "unsupported-scheme"}
		}
		return classifyRemote(src, proto == "https")
	}
	if strings.HasPrefix(src, "//") {
		return classifyRemote("https:"+src, true)
	}
	// vault 相对路径
	clean := strings.SplitN(strings.SplitN(src, "#", 2)[0], "?", 2)[0]
	if clean == "" {
		return imgSource{kind: imgInvalid, reason: "empty"}
	}
	if strings.HasPrefix(clean, `\\`) || strings.HasPrefix(clean, "//") {
		return imgSource{kind: imgInvalid, reason: "path-escape"}
	}
	if strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, `\`) {
		return imgSource{kind: imgInvalid, reason: "path-escape"}
	}
	if regexp.MustCompile(`^[A-Za-z]:`).MatchString(clean) {
		return imgSource{kind: imgInvalid, reason: "path-escape"}
	}
	decoded, err := decodePathSegments(clean)
	if err != nil {
		return imgSource{kind: imgInvalid, reason: "path-escape"}
	}
	segs := []string{}
	for _, s := range regexp.MustCompile(`[\\/]+`).Split(decoded, -1) {
		if s != "" {
			segs = append(segs, s)
		}
	}
	if len(segs) == 0 {
		return imgSource{kind: imgInvalid, reason: "empty"}
	}
	for _, s := range segs {
		if s == "." || s == ".." {
			return imgSource{kind: imgInvalid, reason: "path-escape"}
		}
	}
	base := segs[len(segs)-1]
	dot := strings.LastIndex(base, ".")
	if dot <= 0 || dot == len(base)-1 {
		return imgSource{kind: imgInvalid, reason: "unsupported-extension"}
	}
	ext := strings.ToLower(base[dot+1:])
	if _, ok := imageExtensions[ext]; !ok {
		return imgSource{kind: imgInvalid, reason: "unsupported-extension"}
	}
	return imgSource{kind: imgVault, path: strings.Join(segs, "/")}
}

func classifyRemote(src string, secure bool) imgSource {
	u, err := url.Parse(src)
	if err != nil || u.Host == "" {
		return imgSource{kind: imgInvalid, reason: "unsupported-scheme"}
	}
	return imgSource{kind: imgRemote, src: src, host: u.Host, secure: secure}
}

// 逐段 percent-decode；畸形序列视为逃逸。
func decodePathSegments(value string) (string, error) {
	parts := regexp.MustCompile(`([\\/]+)`).Split(value, -1)
	seps := regexp.MustCompile(`[\\/]+`).FindAllString(value, -1)
	var b strings.Builder
	for i, p := range parts {
		if i > 0 {
			b.WriteString(seps[i-1])
		}
		if regexp.MustCompile(`^[\\/]+$`).MatchString(p) {
			continue
		}
		d, err := url.PathUnescape(p)
		if err != nil {
			return "", err
		}
		b.WriteString(d)
	}
	return b.String(), nil
}

var rejectReasonText = map[string]string{
	"empty":                  "图片地址为空",
	"too-long":               "图片地址过长",
	"unsupported-scheme":     "不支持这种地址协议",
	"not-an-image":           "内联数据不是图片",
	"path-escape":            "路径超出知识库范围",
	"unsupported-extension":  "不是支持的图片格式",
}

// ---------- 自定义渲染器 ----------

type kbRenderer struct {
	variant string // reader | chat
	v       *vaultStub
	ids     *cjkIDs
}

func (r *kbRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindLink, r.renderLink)
	reg.Register(ast.KindImage, r.renderImage)
	reg.Register(ast.KindHeading, r.renderHeading)
	reg.Register(wikilink.Kind, r.renderWikilink)
	reg.Register(extast.KindTaskCheckBox, r.renderTaskCheckBox)
	reg.Register(ast.KindFencedCodeBlock, r.renderFencedCode)
	reg.Register(ast.KindCodeBlock, r.renderFencedCode)
	reg.Register(extast.KindTable, r.renderTable)
	reg.Register(ast.KindCodeSpan, r.renderCodeSpan)
	reg.Register(ast.KindAutoLink, r.renderAutoLink)
	reg.Register(ast.KindRawHTML, r.renderRawHTML)
	reg.Register(ast.KindHTMLBlock, r.renderHTMLBlock)
}

// CJK 友好 heading ID：保留 Unicode 字母数字、小写英文、空白与 -_ 转 -，重复加 -N。
// 旧版 react-markdown 不生成 heading id（无锚点）；本实现生成 CJK id 属有意新增，
// 为锚点链接 `#中文标题` 提供落点，差异记入 RESULT.md。
type cjkIDs struct{ used map[string]int }

func (c *cjkIDs) generate(value []byte) string {
	s := strings.ToLower(string(value))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r == ' ' || r == '-' || r == '_' || r == '\t':
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		}
	}
	id := strings.TrimRight(b.String(), "-")
	if id == "" {
		id = "heading"
	}
	if c.used == nil {
		c.used = map[string]int{}
	}
	if n, ok := c.used[id]; ok {
		c.used[id] = n + 1
		id = fmt.Sprintf("%s-%d", id, n+1)
	} else {
		c.used[id] = 0
	}
	return id
}

func (r *kbRenderer) renderHeading(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Heading)
	if entering {
		if r.ids == nil {
			r.ids = &cjkIDs{}
		}
		var text bytes.Buffer
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			if t, ok := c.(*ast.Text); ok {
				text.Write(t.Segment.Value(source))
			}
		}
		id := r.ids.generate(text.Bytes())
		fmt.Fprintf(w, "<h%d id=%q>", n.Level, id)
	} else {
		fmt.Fprintf(w, "</h%d>\n", n.Level)
	}
	return ast.WalkContinue, nil
}

// 原始 HTML：丢弃（对齐 react-markdown 10 行为——实际旧版是转义展示，
// 这里按 skill 契约「丢弃」实现，差异记入 RESULT.md；两种都能满足「不渲染」）。
func (r *kbRenderer) renderHTMLBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	// 丢弃：不输出任何内容。
	return ast.WalkContinue, nil
}

func (r *kbRenderer) renderRawHTML(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	return ast.WalkContinue, nil
}

func (r *kbRenderer) renderAutoLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.AutoLink)
	if !entering {
		return ast.WalkContinue, nil
	}
	raw := string(n.URL(source))
	kind, target := classifyHref(raw)
	label := raw
	switch kind {
	case lkExternal:
		fmt.Fprintf(w, `<a class="link-external" data-kind="external" data-target=%q>%s</a>`, target, htmlEscape(label))
	case lkAnchor:
		fmt.Fprintf(w, `<a class="link-anchor" data-kind="anchor" data-target=%q>%s</a>`, target, htmlEscape(label))
	case lkInternal:
		fmt.Fprintf(w, `<a class="wiki" data-kind="internal" data-target=%q>%s</a>`, target, htmlEscape(label))
	default:
		fmt.Fprintf(w, `<span class="link-plain">%s</span>`, htmlEscape(label))
	}
	return ast.WalkSkipChildren, nil
}

func (r *kbRenderer) renderLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Link)
	// chat 引用标签：transformChatRefs 打的 data-ref-*。
	if entering {
		refKind, _ := n.AttributeString("data-ref-kind")
		refTarget, _ := n.AttributeString("data-ref-target")
		if ks, ok := refKind.(string); ok && (ks == "path" || ks == "wiki") {
			if ts, ok2 := refTarget.(string); ok2 {
				status := "missing"
				if r.v != nil {
					for _, f := range r.v.Files {
						if f == ts {
							status = "ok"
							break
						}
					}
				}
				n.SetAttributeString("data-kb-refkind", ks)
				n.SetAttributeString("data-kb-reftarget", ts)
				n.SetAttributeString("data-kb-refstatus", status)
				fmt.Fprintf(w, `<button type="button" class="chat-ref" data-ref-kind=%q data-ref-target=%q data-status=%q title=%q aria-label="打开 %s">`, ks, ts, status, ts, htmlEscape(ts))
				return ast.WalkContinue, nil
			}
		}
		raw := string(n.Destination)
		kind, target := classifyHref(raw)
		switch kind {
		case lkExternal:
			fmt.Fprintf(w, `<a class="link-external" data-kind="external" data-target=%q>`, target)
		case lkAnchor:
			fmt.Fprintf(w, `<a class="link-anchor" data-kind="anchor" data-target=%q>`, target)
		case lkInternal:
			// 库内路径链接：旧版行为也探测存在性（CoreAnchor 里 exists）。
			status := "ok"
			if r.v != nil {
				found := false
				for _, f := range r.v.Files {
					if f == target {
						found = true
						break
					}
				}
				if !found {
					status = "missing"
				}
			}
			if status == "missing" {
				fmt.Fprintf(w, `<a class="wiki wiki-missing" data-kind="internal" data-status="missing" data-target=%q title=%q>`, target, target)
			} else {
				fmt.Fprintf(w, `<a class="wiki" data-kind="internal" data-status="ok" data-target=%q title=%q>`, target, target)
			}
		default:
			fmt.Fprintf(w, `<span class="link-plain">`)
		}
		n.SetAttributeString("data-kb-kind", int64(kind))
		return ast.WalkContinue, nil
	}
	// leaving
	if _, ok := n.AttributeString("data-kb-refkind"); ok {
		if s, _ := n.AttributeString("data-kb-refstatus"); s != "ok" {
			fmt.Fprint(w, `<span class="chat-ref-note">找不到</span>`)
		}
		fmt.Fprint(w, `</button>`)
		return ast.WalkContinue, nil
	}
	kindv, _ := n.AttributeString("data-kb-kind")
	if k, ok := kindv.(int64); ok && linkKind(k) == lkInvalid {
		fmt.Fprint(w, `</span>`)
	} else {
		fmt.Fprint(w, `</a>`)
	}
	return ast.WalkContinue, nil
}

func (r *kbRenderer) renderWikilink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*wikilink.Node)
	target := string(n.Target)
	// 空目标当普通文字（旧版 splitWikilinks 不转空目标）。
	if strings.TrimSpace(target) == "" {
		fmt.Fprint(w, htmlEscape("[["))
		// 渲染子节点（如 " "），再补 ]]。
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			if t, ok := c.(*ast.Text); ok {
				w.Write(t.Segment.Value(source))
			}
		}
		fmt.Fprint(w, htmlEscape("]]"))
		return ast.WalkSkipChildren, nil
	}
	// label：[[a|别名]] 显示别名；goldmark/wikilink 把 label 放在子节点。
	label := target
	if n.FirstChild() != nil {
		var lb strings.Builder
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			if t, ok := c.(*ast.Text); ok {
				lb.Write(t.Segment.Value(source))
			}
		}
		if s := lb.String(); s != "" {
			label = s
		}
	}
	dest, err := vaultResolver{v: r.v}.ResolveWikilink(n)
	status := "ok"
	resolved := target
	switch {
	case err == errAmbiguous:
		status = "ambiguous"
	case err == errMissing || dest == nil:
		status = "missing"
	case err != nil:
		status = "missing"
	default:
		resolved = string(dest)
	}
	if r.variant == "chat" {
		// chat 变体：wikilink 渲染成 ChatReference button（旧版 ChatAnchor 的 data-wiki 分支）。
		fmt.Fprintf(w, `<button type="button" class="chat-ref" data-ref-kind="wiki" data-ref-target=%q data-status=%q title="[[%s]]" aria-label="打开 %s">%s`,
			target, status, htmlEscape(target), htmlEscape(target), htmlEscape(label))
		if status == "ambiguous" {
			fmt.Fprint(w, `<span class="chat-ref-note">有多个同名</span>`)
		} else if status == "missing" {
			fmt.Fprint(w, `<span class="chat-ref-note">找不到</span>`)
		}
		fmt.Fprint(w, `</button>`)
		return ast.WalkSkipChildren, nil
	}
	switch status {
	case "ambiguous":
		fmt.Fprintf(w, `<a class="wiki wiki-missing" data-kind="wiki" data-status="ambiguous" data-target=%q data-wiki=%q title="有多个同名文件">%s<span class="wiki-note">（有多个同名）</span></a>`,
			resolved, target, htmlEscape(label))
	case "missing":
		fmt.Fprintf(w, `<a class="wiki wiki-missing" data-kind="wiki" data-status="missing" data-target=%q data-wiki=%q title="[[%s]]">%s<span class="wiki-note">（未创建）</span></a>`,
			resolved, target, target, htmlEscape(label))
	default:
		fmt.Fprintf(w, `<a class="wiki" data-kind="wiki" data-status="ok" data-target=%q data-wiki=%q title="[[%s]]">%s</a>`,
			resolved, target, target, htmlEscape(label))
	}
	return ast.WalkSkipChildren, nil
}

func (r *kbRenderer) renderTaskCheckBox(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*extast.TaskCheckBox)
	if n.IsChecked {
		fmt.Fprint(w, `<input type="checkbox" disabled checked> `)
	} else {
		fmt.Fprint(w, `<input type="checkbox" disabled> `)
	}
	return ast.WalkSkipChildren, nil
}

func (r *kbRenderer) renderFencedCode(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	var lang string
	var codeLines bytes.Buffer
	if n, ok := node.(*ast.FencedCodeBlock); ok {
		if n.Language(source) != nil {
			lang = string(n.Language(source))
		}
		for i := 0; i < n.Lines().Len(); i++ {
			seg := n.Lines().At(i)
			codeLines.Write(seg.Value(source))
		}
	} else if n, ok := node.(*ast.CodeBlock); ok {
		for i := 0; i < n.Lines().Len(); i++ {
			seg := n.Lines().At(i)
			codeLines.Write(seg.Value(source))
		}
	}
	code := codeLines.String()
	if r.variant == "chat" {
		fmt.Fprintf(w, `<div class="codeblock" data-lang=%q><div class="tools">`, lang)
		if lang != "" {
			fmt.Fprintf(w, `<span class="lang">%s</span>`, htmlEscape(lang))
		}
		fmt.Fprintf(w, `<button type="button" class="copy" aria-label="复制代码" data-code="%s">复制</button></div><pre><code`, htmlEscape(code))
		if lang != "" {
			fmt.Fprintf(w, ` class="language-%s"`, htmlEscape(lang))
		}
		fmt.Fprintf(w, `>%s</code></pre></div>`+"\n", htmlEscape(code))
	} else {
		fmt.Fprint(w, `<pre><code`)
		if lang != "" {
			fmt.Fprintf(w, ` class="language-%s"`, htmlEscape(lang))
		}
		fmt.Fprintf(w, `>%s</code></pre>`+"\n", htmlEscape(code))
	}
	return ast.WalkSkipChildren, nil
}

func (r *kbRenderer) renderTable(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if r.variant == "chat" {
		if entering {
			fmt.Fprint(w, `<div class="md-table-wrap"><table>`)
		} else {
			fmt.Fprint(w, `</table></div>`+"\n")
		}
		return ast.WalkContinue, nil
	}
	if entering {
		fmt.Fprint(w, `<table>`)
	} else {
		fmt.Fprint(w, `</table>`+"\n")
	}
	return ast.WalkContinue, nil
}

// chat 行内代码整段是库内路径 → chat-ref 标签（chatReferences.asPathReference）。
var pathTokenRe = regexp.MustCompile(`(?i)^(?:[\w\x{4E00}-\x{9FFF}.-]+/)*[\w\x{4E00}-\x{9FFF}.-]+\.(?:md|markdown|txt)$`)

func isPlausibleRelativePath(v string) bool {
	if v == "" || len(v) > 1024 {
		return false
	}
	if strings.HasPrefix(v, "/") || strings.HasPrefix(v, `\`) {
		return false
	}
	if regexp.MustCompile(`^[A-Za-z]:`).MatchString(v) {
		return false
	}
	for _, s := range regexp.MustCompile(`[\\/]+`).Split(v, -1) {
		if s == "." || s == ".." {
			return false
		}
	}
	return true
}

func asPathReference(text string) string {
	candidate := strings.TrimSpace(text)
	if candidate == "" || candidate != text {
		return ""
	}
	if !pathTokenRe.MatchString(candidate) {
		return ""
	}
	if !isPlausibleRelativePath(candidate) {
		return ""
	}
	return candidate
}

func (r *kbRenderer) renderCodeSpan(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.CodeSpan)
	var buf bytes.Buffer
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			buf.Write(t.Segment.Value(source))
		}
	}
	text := buf.String()
	if r.variant == "chat" {
		if p := asPathReference(text); p != "" {
			status := "missing"
			if r.v != nil {
				for _, f := range r.v.Files {
					if f == p {
						status = "ok"
						break
					}
				}
			}
			fmt.Fprintf(w, `<button type="button" class="chat-ref" data-ref-kind="path" data-ref-target=%q data-status=%q title=%q aria-label="打开 %s">%s`,
				p, status, p, htmlEscape(p), htmlEscape(p))
			if status != "ok" {
				fmt.Fprint(w, `<span class="chat-ref-note">找不到</span>`)
			}
			fmt.Fprint(w, `</button>`)
			return ast.WalkSkipChildren, nil
		}
		fmt.Fprintf(w, `<code class="chat-code">%s</code>`, htmlEscape(text))
		return ast.WalkSkipChildren, nil
	}
	fmt.Fprintf(w, `<code>%s</code>`, htmlEscape(text))
	return ast.WalkSkipChildren, nil
}

func (r *kbRenderer) renderImage(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.Image)
	alt := extractText(n, source)
	src := classifyImageSource(string(n.Destination))
	if r.variant == "chat" {
		caption := alt
		if strings.TrimSpace(caption) == "" {
			caption = "未命名图片"
		}
		// asVaultPath: 排除 scheme 与 //，再 isPlausibleRelativePath。
		s := string(n.Destination)
		vault := ""
		if !schemeRe.MatchString(s) && !strings.HasPrefix(s, "//") && isPlausibleRelativePath(s) {
			vault = s
		}
		if vault == "" {
			fmt.Fprintf(w, `<span class="img-label">图片：%s<span class="img-note">（远程图片不加载）</span></span>`, htmlEscape(caption))
		} else {
			fmt.Fprintf(w, `<button type="button" class="img-label vault" data-vault=%q title=%q aria-label="打开 %s">图片：%s<span class="img-open">打开 %s</span></button>`,
				vault, vault, htmlEscape(vault), htmlEscape(caption), htmlEscape(vault))
		}
		return ast.WalkSkipChildren, nil
	}
	// reader：输出分类标记，前端按 data-img-kind 渲染。
	switch src.kind {
	case imgData:
		fmt.Fprintf(w, `<img class="mile-preview-image" data-img-kind="data" data-state="show" alt=%q src=%q>`, alt, src.src)
	case imgVault:
		fmt.Fprintf(w, `<img data-img-kind="vault" data-vault-path=%q alt=%q>`, src.path, alt)
	case imgRemote:
		if !src.secure {
			fmt.Fprintf(w, `<span class="mile-preview-image-card" data-img-kind="remote" data-state="blocked" data-secure="false"><strong>%s</strong><span>不支持明文 HTTP 图片（可能被中间人替换），请改用 https 链接</span></span>`,
				htmlEscape(captionOf(alt)))
		} else {
			fmt.Fprintf(w, `<span class="mile-preview-image-card" data-img-kind="remote" data-state="remote-gate" data-src=%q data-host=%q data-secure="true"><strong>%s</strong><span>默认不加载：请求会让对方服务器知道你此刻读了这篇笔记</span><code>%s</code><button type="button" class="btn-quiet">加载这张图片</button></span>`,
				src.src, src.host, htmlEscape(captionOf(alt)), htmlEscape(src.host))
		}
	default:
		fmt.Fprintf(w, `<span class="mile-preview-image-card" data-img-kind="invalid" data-state="blocked" data-reason=%q><strong>%s</strong><span>%s</span></span>`,
			rejectReasonText[src.reason], htmlEscape(captionOf(alt)), htmlEscape(rejectReasonText[src.reason]))
	}
	return ast.WalkSkipChildren, nil
}

func captionOf(alt string) string {
	t := strings.TrimSpace(alt)
	if t == "" {
		return "未命名图片"
	}
	return t
}

func extractText(n ast.Node, source []byte) string {
	var b strings.Builder
	var walk func(node ast.Node)
	walk = func(node ast.Node) {
		for c := node.FirstChild(); c != nil; c = c.NextSibling() {
			if t, ok := c.(*ast.Text); ok {
				b.Write(t.Segment.Value(source))
			}
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

var htmlEscaper = strings.NewReplacer(
	"&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;",
)

func htmlEscape(s string) string { return htmlEscaper.Replace(s) }

// ---------- chat 引用（remarkChatReferences：text 里的库内路径 → chat-ref 标签）----------
// goldmark 没有对应插件，在渲染后处理阶段做：这里用一个 text-transformer，
// 在 AST 里把 text 节点中的路径切成 link 节点（带 data-ref-*），渲染时走同一条 link 分支。
// 为保持 spike 简单，这里直接对整段 markdown 在解析后做 AST 变换（transformChatRefs）。

var wikiTokenRe = regexp.MustCompile(`\[\[([^[\]|\n]+)(?:\|([^[\]\n]+))?\]\]`)
var refPathRe = regexp.MustCompile(`(?i)(?:[\w\x{4E00}-\x{9FFF}.-]+/)*[\w\x{4E00}-\x{9FFF}.-]+\.(?:md|markdown|txt)`)

type refMatch struct {
	start, end int
	kind       string // path | wiki
	target     string
	label      string
}

func findReferences(text string) []refMatch {
	var found []refMatch
	for _, m := range wikiTokenRe.FindAllStringSubmatchIndex(text, -1) {
		target := strings.TrimSpace(text[m[2]:m[3]])
		if target == "" {
			continue
		}
		label := target
		if m[4] >= 0 {
			if l := strings.TrimSpace(text[m[4]:m[5]]); l != "" {
				label = l
			}
		}
		found = append(found, refMatch{start: m[0], end: m[1], kind: "wiki", target: target, label: label})
	}
	for _, m := range refPathRe.FindAllStringIndex(text, -1) {
		start, end := m[0], m[1]
		// 起点不能紧跟 \w / . \ : @ - 之一；末尾不能紧跟 \w / -
		if start > 0 {
			prev := text[start-1]
			if strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_/.\\:@-", rune(prev)) {
				continue
			}
		}
		if end < len(text) {
			next := text[end]
			if strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_/-", rune(next)) {
				continue
			}
		}
		overlap := false
		for _, h := range found {
			if start < h.end && end > h.start {
				overlap = true
				break
			}
		}
		if overlap {
			continue
		}
		target := text[start:end]
		if !isPlausibleRelativePath(target) {
			continue
		}
		found = append(found, refMatch{start: start, end: end, kind: "path", target: target, label: target})
	}
	// 按 start 排序
	for i := 0; i < len(found); i++ {
		for j := i + 1; j < len(found); j++ {
			if found[j].start < found[i].start {
				found[i], found[j] = found[j], found[i]
			}
		}
	}
	return found
}

// transformChatRefs：把 text 节点里的库内路径切成 link 节点（data-ref-kind/target）。
func transformChatRefs(doc ast.Node, source []byte) {
	var process func(n ast.Node, insideLink bool)
	process = func(n ast.Node, insideLink bool) {
		for c := n.FirstChild(); c != nil; {
			next := c.NextSibling()
			switch v := c.(type) {
			case *ast.Text:
				if insideLink {
					c = next
					continue
				}
				segText := string(v.Segment.Value(source))
				refs := findReferences(segText)
				if len(refs) == 0 {
					c = next
					continue
				}
				parent := v.Parent()
				base := v.Segment.Start
				cursor := 0
				for _, ref := range refs {
					if ref.start > cursor {
						parent.InsertBefore(parent, v,
							ast.NewTextSegment(text.NewSegment(base+cursor, base+ref.start)))
					}
					link := ast.NewLink()
					link.Destination = []byte("#")
					link.SetAttributeString("data-ref-kind", ref.kind)
					link.SetAttributeString("data-ref-target", ref.target)
					link.AppendChild(link, ast.NewString([]byte(ref.label)))
					parent.InsertBefore(parent, v, link)
					cursor = ref.end
				}
				if cursor < len(segText) {
					parent.InsertBefore(parent, v,
						ast.NewTextSegment(text.NewSegment(base+cursor, base+len(segText))))
				}
				parent.RemoveChild(parent, v)
			default:
				// 跳过 CodeSpan：行内代码内容不是正文文本（旧版 inlineCode 不是 text 节点）。
				if _, ok := c.(*ast.CodeSpan); ok {
					c = next
					continue
				}
				isL := insideLink
				if _, ok := c.(*ast.Link); ok {
					isL = true
				}
				process(c, isL)
			}
			c = next
		}
	}
	process(doc, false)
}

// ---------- 链接 close：goldmark 的标准 Link 渲染器会写 </a>，但我们接管了 ----------
// 简化方案：不接管 ast.KindLink，改为在渲染后处理输出？不行——invalid 要去掉 <a>。
// 正确方案：用带状态的 renderLink（上面写了 open）。这里补 leaving 的 close。
// 但 renderLink 在 leaving 分支直接返回了。我们把 renderLink 改成完整版。

// ---------- bluemonday 白名单 ----------

func newPolicy() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements("p", "h1", "h2", "h3", "h4", "h5", "h6", "a", "strong", "em", "del", "code",
		"pre", "blockquote", "ul", "ol", "li", "table", "thead", "tbody", "tr", "th", "td",
		"hr", "br", "img", "span", "div", "details", "summary", "input", "button", "article")
	p.AllowAttrs("href", "data-kind", "data-target", "data-wiki", "data-status", "title", "class",
		"data-ref-kind", "data-ref-target", "aria-label", "type").OnElements("a")
	p.AllowAttrs("src", "alt", "title", "class",
		"data-img-kind", "data-vault-path", "data-vault", "data-src", "data-host", "data-secure", "data-state").OnElements("img")
	p.AllowAttrs("id", "class").OnElements("h1", "h2", "h3", "h4", "h5", "h6")
	p.AllowAttrs("class", "data-lang", "data-display", "data-scrollable", "data-kind", "data-status",
		"data-vault", "data-img-kind", "data-src", "data-host", "data-secure", "data-reason", "data-state",
		"data-ref-kind", "data-ref-target", "data-target", "data-doc-path", "data-msg-id",
		"aria-label", "role", "title", "type", "data-wiki", "data-code", "data-ext").OnElements("span", "div", "button", "code")
	p.AllowAttrs("class").OnElements("pre", "code", "table", "details", "summary", "article")
	p.AllowAttrs("type", "disabled", "checked").OnElements("input")
	p.AllowAttrs("align").OnElements("th", "td")
	p.AllowAttrs("start").OnElements("ol")
	p.AllowURLSchemes("http", "https", "kbimg", "data")
	p.AllowRelativeURLs(true)
	return p
}

// ---------- 渲染入口 ----------

func render(md goldmark.Markdown, source []byte, v *vaultStub, variant string) (string, error) {
	doc := md.Parser().Parse(text.NewReader(source))
	if variant == "chat" {
		transformChatRefs(doc, source)
	}
	var buf bytes.Buffer
	if err := md.Renderer().Render(&buf, source, doc); err != nil {
		return "", err
	}
	return newPolicy().Sanitize(buf.String()), nil
}

func newMarkdown(v *vaultStub, variant string) goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			&wikilink.Extender{Resolver: vaultResolver{v: v}},
		),
		goldmark.WithRendererOptions(
			renderer.WithNodeRenderers(util.Prioritized(&kbRenderer{variant: variant, v: v}, 100)),
		),
	)
}

// ---------- 流式裁剪（chatStreaming.trimStreamingText） ----------

func lastOpenFence(lines []string, beforeIndex int) int {
	open := -1
	for i := 0; i < beforeIndex; i++ {
		t := strings.TrimSpace(lines[i])
		if !regexp.MustCompile(`^(?:` + "```" + `|~~~)`).MatchString(t) {
			continue
		}
		if open == -1 {
			open = i
		} else {
			prev := strings.TrimSpace(lines[open])
			if len(prev) >= 3 && len(t) >= 3 && prev[:3] == t[:3] {
				open = -1
			}
		}
	}
	return open
}

type trimResult struct {
	text        string
	pending     bool
	pendingKind string
}

func trimStreamingText(s string, streaming bool) trimResult {
	if !streaming || s == "" {
		return trimResult{text: s}
	}
	lines := strings.Split(s, "\n")
	tableEnd, tableStart := -1, -1
	for i := len(lines) - 1; i >= 0; i-- {
		t := strings.TrimSpace(lines[i])
		if strings.HasPrefix(t, "|") {
			if tableEnd == -1 {
				tableEnd = i
			}
			tableStart = i
			continue
		}
		if t == "" {
			continue
		}
		break
	}
	if tableEnd != -1 {
		if lastOpenFence(lines, tableStart) != -1 {
			return trimResult{text: s}
		}
		block := lines[tableStart : tableEnd+1]
		sepIndex := -1
		for i, l := range block {
			if regexp.MustCompile(`^\s*\|[\s\-:|]+\|\s*$`).MatchString(l) {
				sepIndex = i
				break
			}
		}
		if sepIndex == -1 {
			kept := strings.TrimRight(strings.Join(lines[:tableStart], "\n"), "\n")
			return trimResult{text: kept, pending: true, pendingKind: "table"}
		}
		lastLine := strings.TrimSpace(lines[tableEnd])
		if !strings.HasSuffix(lastLine, "|") {
			kept := strings.TrimRight(strings.Join(lines[:tableEnd], "\n"), "\n")
			return trimResult{text: kept, pending: true, pendingKind: "table"}
		}
	}
	// 末尾未闭合 [文字](
	tail := s
	if len(tail) > 80 {
		tail = tail[len(tail)-80:]
	}
	m := regexp.MustCompile(`\[([^\]]+)\]\([^)]*$`).FindStringSubmatch(tail)
	if m != nil {
		idx := strings.LastIndex(s, m[0])
		before := s[:idx]
		lineNo := strings.Count(before, "\n")
		if lastOpenFence(strings.Split(s, "\n"), lineNo) == -1 {
			kept := s[:idx] + m[1]
			return trimResult{text: kept, pending: true, pendingKind: "link"}
		}
	}
	return trimResult{text: s}
}
