// Package render turns Markdown into sanitized HTML for the two display
// variants of the app: "reader" (阅读模式) and "chat" (对话消息).
//
// The behaviour contract is the legacy Electron implementation; the
// authoritative references are
//
//	.claude/skills/markdown-render-pipeline/  (SKILL.md + references/)
//	spikes/r3-render/RESULT.md                (R-3 parity measurements)
//	docs/decisions.md D-004, D-006, D-007
//
// Key decisions baked in here:
//   - raw HTML (block, inline, comments) is escaped to visible text, never
//     executed and never dropped (D-007 / PRD v1.4);
//   - YAML frontmatter renders as normal body text (D-006);
//   - headings get no id; "#xx" anchor links carry data-kind="anchor" and
//     perform no scrolling when clicked;
//   - links are classified external / anchor / vault-relative / invalid;
//   - vault images are rewritten to http://kbimg.localhost/<escaped path>
//     via Options.ImageURL; remote https shows a placeholder card, plain
//     http is blocked, data:image/* passes through.
package render

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

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

// Variant selects which output overrides apply.
type Variant string

const (
	// VariantReader is the reading-mode variant (阅读模式).
	VariantReader Variant = "reader"
	// VariantChat is the conversation variant (对话消息).
	VariantChat Variant = "chat"
)

// Link kinds reported through data-kind on anchors.
const (
	KindExternal = "external"
	KindAnchor   = "anchor"
	KindDoc      = "doc"
	KindWiki     = "wiki"
)

// Link states reported through data-status.
const (
	StatusOK        = "ok"
	StatusMissing   = "missing"
	StatusAmbiguous = "ambiguous"
)

// LinkResolver maps a Markdown target to a canonical in-vault path.
//
// The renderer passes the target through unchanged; deciding what counts as
// a hit (exact relative path, file name, title — in that order) is the
// resolver's job. A false ok means the renderer degrades the link:
// for images the relevant placeholder is emitted; for links the resolver's
// kind decides between "missing" (not yet created) and "ambiguous"
// (multiple same-name candidates).
type LinkResolver interface {
	// ResolveLink handles [text](target) and [[wikilink]] targets.
	// kind is "internal", "missing" or "ambiguous".
	ResolveLink(target string) (path string, kind string, ok bool)
	// ResolveImage handles ![alt](src) targets that already passed the
	// syntactic vault-path checks. ok=false marks the file unreadable.
	ResolveImage(src string) (path string, ok bool)
}

// Options controls one Render call.
type Options struct {
	// Variant picks the output shape. Empty or unknown values render as reader.
	Variant Variant
	// Resolver resolves in-vault relative links and images. nil renders every
	// vault-relative link as missing and every vault image as broken.
	Resolver LinkResolver
	// ImageURL builds the URL for a resolved vault image path. nil defaults
	// to the M0 form: "http://kbimg.localhost/" + url.PathEscape(path).
	ImageURL func(path string) string
}

// Result is one rendered document or block.
type Result struct {
	// HTML is sanitized against the bluemonday whitelist; safe to inject.
	HTML string
	// HasImages reports whether the source contained any image syntax.
	HasImages bool
}

// Render parses md as CommonMark + GFM + wikilinks and returns sanitized
// HTML. CRLF is normalised to LF first. The call is CPU-bound and carries
// no timeout; callers that need one run it in a goroutine.
func Render(md []byte, opts Options) (Result, error) {
	src := bytes.ReplaceAll(md, []byte("\r\n"), []byte("\n"))
	src = bytes.ReplaceAll(src, []byte("\r"), []byte("\n"))

	r := &kbRenderer{opts: opts}
	markdown := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			// No frontmatter.Extender: frontmatter stays in the body (D-006).
			&wikilink.Extender{Resolver: wikilinkResolver{r: r}},
		),
		// No parser.WithAutoHeadingID: headings carry no id (D-007).
		goldmark.WithRendererOptions(
			// No html.WithUnsafe: raw HTML is escaped to text instead.
			renderer.WithNodeRenderers(util.Prioritized(r, 100)),
		),
	)

	doc := markdown.Parser().Parse(text.NewReader(src))
	if opts.Variant == VariantChat {
		transformChatRefs(doc, src, r)
	}
	var buf bytes.Buffer
	if err := markdown.Renderer().Render(&buf, src, doc); err != nil {
		return Result{}, fmt.Errorf("render: %w", err)
	}
	return Result{HTML: normalizeEntities(sanitize(buf.String())), HasImages: r.hasImages}, nil
}

func defaultImageURL(path string) string {
	return "http://kbimg.localhost/" + url.PathEscape(path)
}

// sanitize applies the whitelist from
// .claude/skills/markdown-render-pipeline/references/whitelist.md. The
// renderer only emits whitelisted markup; this is the single exit gate.
func sanitize(html string) string {
	return newPolicy().Sanitize(html)
}

func newPolicy() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements("p", "h1", "h2", "h3", "h4", "h5", "h6", "a", "strong", "em", "del", "code",
		"pre", "blockquote", "ul", "ol", "li", "table", "thead", "tbody", "tr", "th", "td",
		"hr", "br", "img", "span", "div", "details", "summary", "input", "button", "article")
	p.AllowAttrs("href", "data-kind", "data-target", "data-wiki", "data-status", "title", "class",
		"data-ref-kind", "data-ref-target", "aria-label", "type").OnElements("a")
	p.AllowAttrs("src", "alt", "title", "class",
		"data-img-kind", "data-vault-path", "data-vault", "data-src", "data-host",
		"data-secure", "data-state").OnElements("img")
	p.AllowAttrs("id", "class").OnElements("h1", "h2", "h3", "h4", "h5", "h6")
	p.AllowAttrs("class", "data-lang", "data-display", "data-scrollable", "data-kind", "data-status",
		"data-vault", "data-img-kind", "data-src", "data-host", "data-secure", "data-reason", "data-state",
		"data-ref-kind", "data-ref-target", "data-target", "data-doc-path", "data-msg-id",
		"aria-label", "role", "title", "type", "data-wiki", "data-code", "data-ext",
	).OnElements("span", "div", "button", "code")
	p.AllowAttrs("class").OnElements("pre", "code", "table", "details", "summary", "article")
	p.AllowAttrs("type", "disabled", "checked").OnElements("input")
	p.AllowAttrs("align").OnElements("th", "td")
	p.AllowAttrs("start").OnElements("ol")
	// "data" stays only for <img src="data:image/...">; the renderer rejects
	// every other data: URI before it reaches here (whitelist.md hard rule).
	p.AllowURLSchemes("http", "https", "kbimg", "data")
	p.AllowRelativeURLs(true)
	return p
}

var htmlEscaper = strings.NewReplacer(
	"&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;",
)

func htmlEscape(s string) string { return htmlEscaper.Replace(s) }

// normalizeEntities rewrites bluemonday's numeric character references to
// the named entities the legacy React renderer emitted, so sanitized text
// compares byte-for-byte. Only &#34; (") and &#39; (') appear; bluemonday
// already leaves &amp; &lt; &gt; &quot; alone.
var entityReplacer = strings.NewReplacer("&#34;", "&quot;", "&#39;", "&#39;")

func normalizeEntities(s string) string {
	if !strings.Contains(s, "&#") {
		return s
	}
	return entityReplacer.Replace(s)
}

// ---------------------------------------------------------------------------
// Link classification (legacy readerLinks.ts classifyHref).
// ---------------------------------------------------------------------------

var (
	schemeRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:`)
	httpRe   = regexp.MustCompile(`^https?://`)
	driveRe  = regexp.MustCompile(`^[a-zA-Z]:[\\/]`)
)

type linkClass int

const (
	lcInvalid linkClass = iota
	lcExternal
	lcAnchor
	lcInternal
)

// classifyHref mirrors legacy classifyHref: "#" → anchor, http(s) →
// external, any other scheme → invalid; the remainder must be a clean
// vault-relative path (no leading slash, drive letter, ".." or UNC).
func classifyHref(raw string) (linkClass, string) {
	href := strings.TrimSpace(raw)
	if href == "" {
		return lcInvalid, ""
	}
	if strings.HasPrefix(href, "#") {
		return lcAnchor, href[1:]
	}
	if httpRe.MatchString(href) {
		return lcExternal, href
	}
	if schemeRe.MatchString(href) {
		return lcInvalid, href
	}
	withoutFragment := strings.SplitN(strings.SplitN(href, "#", 2)[0], "?", 2)[0]
	decoded := safeDecode(withoutFragment)
	decoded = strings.ReplaceAll(decoded, "\\", "/")
	if decoded == "" {
		return lcAnchor, href
	}
	if strings.HasPrefix(decoded, "/") || driveRe.MatchString(decoded) {
		return lcInvalid, href
	}
	for _, seg := range strings.Split(decoded, "/") {
		if seg == ".." {
			return lcInvalid, href
		}
	}
	target := decoded
	if strings.HasPrefix(target, "./") {
		target = target[2:]
	}
	return lcInternal, target
}

func safeDecode(s string) string {
	d, err := url.PathUnescape(s)
	if err != nil {
		return s
	}
	return d
}

// ---------------------------------------------------------------------------
// Image source classification (legacy imageSource.ts classifyImageSource).
// ---------------------------------------------------------------------------

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
	kind   imgKind
	path   string
	src    string
	host   string
	secure bool
	reason string
}

var imgTitleRe = regexp.MustCompile(`^(\S*)\s+(?:"[^"]*"|'[^']*'|\([^)]*\))$`)

// classifyImageSource mirrors legacy classifyImageSource: strip "<...>"
// wrappers and titles, then bucket into data / remote / vault / invalid.
func classifyImageSource(raw string) imgSource {
	if len(raw) > maxSrcLength {
		return imgSource{kind: imgInvalid, reason: "too-long"}
	}
	src := strings.TrimSpace(raw)
	if strings.HasPrefix(src, "<") && strings.HasSuffix(src, ">") {
		src = src[1 : len(src)-1]
	}
	if m := imgTitleRe.FindStringSubmatch(src); m != nil {
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
	// vault relative path
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

// decodePathSegments percent-decodes each segment; a malformed escape is
// treated as an escape attempt and rejected.
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
	"empty":                 "图片地址为空",
	"too-long":              "图片地址过长",
	"unsupported-scheme":    "不支持这种地址协议",
	"not-an-image":          "内联数据不是图片",
	"path-escape":           "路径超出知识库范围",
	"unsupported-extension": "不是支持的图片格式",
	"missing":               "图片文件不存在",
}

// ---------------------------------------------------------------------------
// Custom node renderer.
// ---------------------------------------------------------------------------

type kbRenderer struct {
	opts      Options
	hasImages bool
	// per-link rendering state, stashed on the node between enter/leave.
}

func (r *kbRenderer) variant() Variant {
	if r.opts.Variant == VariantChat {
		return VariantChat
	}
	return VariantReader
}

func (r *kbRenderer) imageURL(path string) string {
	if r.opts.ImageURL != nil {
		return r.opts.ImageURL(path)
	}
	return defaultImageURL(path)
}

// resolveLink runs the resolver and normalises its answer into a status.
// Nil resolver → every vault-relative link is missing.
func (r *kbRenderer) resolveLink(target string) (path, status string) {
	if r.opts.Resolver == nil {
		return target, StatusMissing
	}
	p, kind, ok := r.opts.Resolver.ResolveLink(target)
	if !ok {
		if kind == StatusAmbiguous {
			return target, StatusAmbiguous
		}
		return target, StatusMissing
	}
	if p == "" {
		p = target
	}
	return p, StatusOK
}

// resolveImage checks a syntactically-vault image against the resolver.
func (r *kbRenderer) resolveImage(src string) (path string, ok bool) {
	if r.opts.Resolver == nil {
		return "", false
	}
	return r.opts.Resolver.ResolveImage(src)
}

func (r *kbRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindLink, r.renderLink)
	reg.Register(ast.KindImage, r.renderImage)
	reg.Register(wikilink.Kind, r.renderWikilink)
	reg.Register(extast.KindTaskCheckBox, r.renderTaskCheckBox)
	reg.Register(ast.KindFencedCodeBlock, r.renderFencedCode)
	reg.Register(ast.KindCodeBlock, r.renderFencedCode)
	reg.Register(extast.KindTable, r.renderTable)
	reg.Register(extast.KindTableHeader, r.renderTableHeader)
	reg.Register(extast.KindTableRow, r.renderTableRow)
	reg.Register(extast.KindTableCell, r.renderTableCell)
	reg.Register(ast.KindCodeSpan, r.renderCodeSpan)
	reg.Register(ast.KindAutoLink, r.renderAutoLink)
	reg.Register(ast.KindRawHTML, r.renderRawHTML)
	reg.Register(ast.KindHTMLBlock, r.renderHTMLBlock)
}

// ---- raw HTML: escape to visible text (D-007 / PRD v1.4) ----

// renderHTMLBlock escapes each line of a block-level raw HTML node. The
// block stays unwrapped in <p>, matching the legacy output.
func (r *kbRenderer) renderHTMLBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.HTMLBlock)
	for i := 0; i < n.Lines().Len(); i++ {
		seg := n.Lines().At(i)
		line := seg.Value(source)
		if i == n.Lines().Len()-1 {
			line = bytes.TrimRight(line, "\n")
		}
		fmt.Fprint(w, htmlEscape(string(line)))
	}
	fmt.Fprint(w, "\n")
	return ast.WalkSkipChildren, nil
}

// renderRawHTML escapes inline raw HTML and comments inside a paragraph.
func (r *kbRenderer) renderRawHTML(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.RawHTML)
	for i := 0; i < n.Segments.Len(); i++ {
		seg := n.Segments.At(i)
		fmt.Fprint(w, htmlEscape(string(seg.Value(source))))
	}
	return ast.WalkSkipChildren, nil
}

// ---- links ----

func (r *kbRenderer) renderAutoLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.AutoLink)
	if !entering {
		return ast.WalkContinue, nil
	}
	raw := string(n.URL(source))
	kind, target := classifyHref(raw)
	label := raw
	switch kind {
	case lcExternal:
		fmt.Fprintf(w, `<a class="link-external" data-kind="external" data-target=%q>%s</a>`, target, htmlEscape(label))
	case lcAnchor:
		fmt.Fprintf(w, `<a class="link-anchor" data-kind="anchor" data-target=%q>%s</a>`, target, htmlEscape(label))
	case lcInternal:
		path, status := r.resolveLink(target)
		if status == StatusOK {
			fmt.Fprintf(w, `<a class="wiki" data-kind="doc" data-status="ok" data-target=%q title=%q>%s</a>`, path, path, htmlEscape(label))
		} else {
			fmt.Fprintf(w, `<a class="wiki wiki-missing" data-kind="doc" data-status=%q data-target=%q title=%q>%s<span class="wiki-note">（未创建）</span></a>`, status, path, path, htmlEscape(label))
		}
	default:
		fmt.Fprintf(w, `<span class="link-plain">%s</span>`, htmlEscape(label))
	}
	return ast.WalkSkipChildren, nil
}

func (r *kbRenderer) renderLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Link)
	if entering {
		// chat 引用标签：transformChatRefs stamped data-ref-* on this node.
		refKind, _ := n.AttributeString("data-ref-kind")
		refTarget, _ := n.AttributeString("data-ref-target")
		if ks, ok := refKind.(string); ok && (ks == "path" || ks == "wiki") {
			if ts, ok2 := refTarget.(string); ok2 {
				status := StatusMissing
				if r.opts.Resolver != nil {
					if _, k, ok3 := r.opts.Resolver.ResolveLink(ts); ok3 && k != StatusAmbiguous {
						status = StatusOK
					}
				}
				n.SetAttributeString("data-kb-ref", status)
				fmt.Fprintf(w, `<button type="button" class="chat-ref" data-ref-kind=%q data-ref-target=%q data-status=%q title=%q aria-label="打开 %s">`, ks, ts, status, ts, htmlEscape(ts))
				return ast.WalkContinue, nil
			}
		}
		raw := string(n.Destination)
		kind, target := classifyHref(raw)
		switch kind {
		case lcExternal:
			fmt.Fprintf(w, `<a class="link-external" data-kind="external" data-target=%q>`, target)
		case lcAnchor:
			fmt.Fprintf(w, `<a class="link-anchor" data-kind="anchor" data-target=%q>`, target)
		case lcInternal:
			path, status := r.resolveLink(target)
			switch status {
			case StatusOK:
				fmt.Fprintf(w, `<a class="wiki" data-kind="doc" data-status="ok" data-target=%q title=%q>`, path, path)
			case StatusAmbiguous:
				fmt.Fprintf(w, `<a class="wiki wiki-missing" data-kind="doc" data-status="ambiguous" data-target=%q title="有多个同名文件">`, path)
			default:
				fmt.Fprintf(w, `<a class="wiki wiki-missing" data-kind="doc" data-status="missing" data-target=%q title=%q>`, path, path)
			}
			n.SetAttributeString("data-kb-status", status)
		default:
			fmt.Fprintf(w, `<span class="link-plain">`)
		}
		n.SetAttributeString("data-kb-kind", int64(kind))
		return ast.WalkContinue, nil
	}
	// leaving
	if s, ok := n.AttributeString("data-kb-ref"); ok {
		if st, _ := s.(string); st != StatusOK {
			fmt.Fprint(w, `<span class="chat-ref-note">找不到</span>`)
		}
		fmt.Fprint(w, `</button>`)
		return ast.WalkContinue, nil
	}
	kindv, _ := n.AttributeString("data-kb-kind")
	kind := linkClass(0)
	if k, ok := kindv.(int64); ok {
		kind = linkClass(k)
	}
	if kind == lcInternal {
		st, _ := n.AttributeString("data-kb-status")
		if st == StatusAmbiguous {
			fmt.Fprint(w, `<span class="wiki-note">（有多个同名）</span>`)
		} else if st == StatusMissing {
			fmt.Fprint(w, `<span class="wiki-note">（未创建）</span>`)
		}
	}
	if kind == lcInvalid {
		fmt.Fprint(w, `</span>`)
	} else {
		fmt.Fprint(w, `</a>`)
	}
	return ast.WalkContinue, nil
}

// ---- wikilinks ----

// wikilinkResolver adapts our Options.Resolver to goldmark/wikilink. The
// library only needs the destination when resolution succeeds; the custom
// node renderer below re-runs resolution itself for the three states, so
// this adapter mainly drives ![[embed]] handling.
type wikilinkResolver struct{ r *kbRenderer }

func (w wikilinkResolver) ResolveWikilink(n *wikilink.Node) ([]byte, error) {
	target := wikiTarget(n)
	if strings.TrimSpace(target) == "" {
		return nil, nil
	}
	path, status := w.r.resolveLink(target)
	if status == StatusOK {
		return []byte(path), nil
	}
	if status == StatusAmbiguous {
		return nil, errAmbiguous
	}
	return nil, errMissing
}

var (
	errAmbiguous = errors.New("ambiguous")
	errMissing   = errors.New("missing")
)

// wikiTarget rejoins Target + "#" + Fragment: legacy splitWikilinks treats
// the whole "a#frag" as one lookup target (old-behavior.md §3).
func wikiTarget(n *wikilink.Node) string {
	if len(n.Fragment) == 0 {
		return string(n.Target)
	}
	return string(n.Target) + "#" + string(n.Fragment)
}

func (r *kbRenderer) renderWikilink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*wikilink.Node)
	target := wikiTarget(n)

	// Empty target falls back to literal text, like splitWikilinks.
	if strings.TrimSpace(target) == "" {
		fmt.Fprint(w, htmlEscape("[["))
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			if t, ok := c.(*ast.Text); ok {
				fmt.Fprint(w, htmlEscape(string(t.Segment.Value(source))))
			}
		}
		fmt.Fprint(w, htmlEscape("]]"))
		return ast.WalkSkipChildren, nil
	}

	// ![[embed]]: no legacy semantics; route through image classification.
	if n.Embed {
		r.renderImageSrc(w, string(n.Target), target)
		return ast.WalkSkipChildren, nil
	}

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

	path, status := r.resolveLink(target)
	if r.variant() == VariantChat {
		// chat: wikilinks become chat-ref buttons (legacy ChatAnchor data-wiki).
		fmt.Fprintf(w, `<button type="button" class="chat-ref" data-ref-kind="wiki" data-ref-target=%q data-status=%q title="[[%s]]" aria-label="打开 %s">%s`,
			target, status, htmlEscape(target), htmlEscape(target), htmlEscape(label))
		if status == StatusAmbiguous {
			fmt.Fprint(w, `<span class="chat-ref-note">有多个同名</span>`)
		} else if status == StatusMissing {
			fmt.Fprint(w, `<span class="chat-ref-note">找不到</span>`)
		}
		fmt.Fprint(w, `</button>`)
		return ast.WalkSkipChildren, nil
	}
	switch status {
	case StatusAmbiguous:
		fmt.Fprintf(w, `<a class="wiki wiki-missing" data-kind="wiki" data-status="ambiguous" data-target=%q data-wiki=%q title="有多个同名文件">%s<span class="wiki-note">（有多个同名）</span></a>`,
			path, target, htmlEscape(label))
	case StatusMissing:
		fmt.Fprintf(w, `<a class="wiki wiki-missing" data-kind="wiki" data-status="missing" data-target=%q data-wiki=%q title="[[%s]]">%s<span class="wiki-note">（未创建）</span></a>`,
			path, target, target, htmlEscape(label))
	default:
		fmt.Fprintf(w, `<a class="wiki" data-kind="wiki" data-status="ok" data-target=%q data-wiki=%q title="[[%s]]">%s</a>`,
			path, target, target, htmlEscape(label))
	}
	return ast.WalkSkipChildren, nil
}

// ---- task list checkbox ----

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

// ---- code blocks ----

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
	if r.variant() == VariantChat {
		fmt.Fprintf(w, `<div class="codeblock" data-lang=%q><div class="tools">`, lang)
		if lang != "" {
			fmt.Fprintf(w, `<span class="lang">%s</span>`, htmlEscape(lang))
		}
		fmt.Fprintf(w, `<button type="button" class="copy" data-act="copy" aria-label="复制代码" data-code="%s">复制</button></div><pre><code`, htmlEscape(code))
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

// ---- tables ----

func (r *kbRenderer) renderTable(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if r.variant() == VariantChat {
		if entering {
			fmt.Fprint(w, `<div class="md-table-wrap"><table>`)
		} else {
			fmt.Fprint(w, `</table></div>`)
		}
		return ast.WalkContinue, nil
	}
	if entering {
		fmt.Fprint(w, `<table>`)
	} else {
		fmt.Fprint(w, `</table>`)
	}
	return ast.WalkContinue, nil
}

// Table substructure mirrors legacy remark-gfm output: no whitespace
// between tags and align attributes on th/td (goldmark's defaults differ).

func (r *kbRenderer) renderTableHeader(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		fmt.Fprint(w, `<thead><tr>`)
	} else {
		fmt.Fprint(w, `</tr></thead>`)
	}
	return ast.WalkContinue, nil
}

func (r *kbRenderer) renderTableRow(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		if node.PreviousSibling() != nil && node.PreviousSibling().Kind() == extast.KindTableHeader {
			fmt.Fprint(w, `<tbody>`)
		}
		fmt.Fprint(w, `<tr>`)
	} else {
		fmt.Fprint(w, `</tr>`)
		if node.NextSibling() == nil {
			fmt.Fprint(w, `</tbody>`)
		}
	}
	return ast.WalkContinue, nil
}

func (r *kbRenderer) renderTableCell(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*extast.TableCell)
	tag := "td"
	if n.Parent() != nil && n.Parent().Kind() == extast.KindTableHeader {
		tag = "th"
	}
	if entering {
		switch n.Alignment {
		case extast.AlignLeft:
			fmt.Fprintf(w, `<%s align="left">`, tag)
		case extast.AlignCenter:
			fmt.Fprintf(w, `<%s align="center">`, tag)
		case extast.AlignRight:
			fmt.Fprintf(w, `<%s align="right">`, tag)
		default:
			fmt.Fprintf(w, `<%s>`, tag)
		}
	} else {
		fmt.Fprintf(w, `</%s>`, tag)
	}
	return ast.WalkContinue, nil
}

// ---- inline code ----

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

// asPathReference mirrors chatReferences.asPathReference: the whole code
// span must be one vault-relative .md/.markdown/.txt path.
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
	codeText := buf.String()
	if r.variant() == VariantChat {
		if p := asPathReference(codeText); p != "" {
			status := StatusMissing
			if r.opts.Resolver != nil {
				if _, k, ok := r.opts.Resolver.ResolveLink(p); ok && k != StatusAmbiguous {
					status = StatusOK
				}
			}
			fmt.Fprintf(w, `<button type="button" class="chat-ref" data-ref-kind="path" data-ref-target=%q data-status=%q title=%q aria-label="打开 %s">%s`,
				p, status, p, htmlEscape(p), htmlEscape(p))
			if status != StatusOK {
				fmt.Fprint(w, `<span class="chat-ref-note">找不到</span>`)
			}
			fmt.Fprint(w, `</button>`)
			return ast.WalkSkipChildren, nil
		}
		fmt.Fprintf(w, `<code class="chat-code">%s</code>`, htmlEscape(codeText))
		return ast.WalkSkipChildren, nil
	}
	fmt.Fprintf(w, `<code>%s</code>`, htmlEscape(codeText))
	return ast.WalkSkipChildren, nil
}

// ---- images ----

func (r *kbRenderer) renderImage(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.Image)
	alt := extractText(n, source)
	r.renderImageSrc(w, string(n.Destination), alt)
	return ast.WalkSkipChildren, nil
}

// renderImageSrc is shared by ![alt](src) and ![[embed]].
func (r *kbRenderer) renderImageSrc(w util.BufWriter, rawDest, alt string) {
	r.hasImages = true
	src := classifyImageSource(rawDest)
	if r.variant() == VariantChat {
		caption := alt
		if strings.TrimSpace(caption) == "" {
			caption = "未命名图片"
		}
		// asVaultPath: rule out schemes and "//", then plausible relative path.
		vault := ""
		if !schemeRe.MatchString(rawDest) && !strings.HasPrefix(rawDest, "//") && isPlausibleRelativePath(rawDest) {
			vault = rawDest
		}
		if vault == "" {
			fmt.Fprintf(w, `<span class="img-label">图片：%s<span class="img-note">（远程图片不加载）</span></span>`, htmlEscape(caption))
		} else {
			fmt.Fprintf(w, `<button type="button" class="img-label vault" data-vault=%q title=%q aria-label="打开 %s">图片：%s<span class="img-open">打开 %s</span></button>`,
				vault, vault, htmlEscape(vault), htmlEscape(caption), htmlEscape(vault))
		}
		return
	}
	// reader: emit classification markers; the frontend renders each kind.
	switch src.kind {
	case imgData:
		fmt.Fprintf(w, `<img class="mile-preview-image" data-img-kind="data" data-state="show" alt=%q src=%q>`, alt, src.src)
	case imgVault:
		path, ok := r.resolveImage(src.path)
		if !ok {
			fmt.Fprintf(w, `<span class="mile-preview-image-card" data-img-kind="invalid" data-state="blocked" data-reason=%q><strong>%s</strong><span>%s</span></span>`,
				rejectReasonText["missing"], htmlEscape(captionOf(alt)), htmlEscape(rejectReasonText["missing"]))
			return
		}
		fmt.Fprintf(w, `<img data-img-kind="vault" data-vault-path=%q src=%q alt=%q>`, path, r.imageURL(path), alt)
	case imgRemote:
		if !src.secure {
			fmt.Fprintf(w, `<span class="mile-preview-image-card" data-img-kind="remote" data-state="blocked" data-secure="false"><strong>%s</strong><span>不支持明文 HTTP 图片（可能被中间人替换），请改用 https 链接</span></span>`,
				htmlEscape(captionOf(alt)))
		} else {
			fmt.Fprintf(w, `<span class="mile-preview-image-card" data-img-kind="remote" data-state="remote-gate" data-src=%q data-host=%q data-secure="true"><strong>%s</strong><span>默认不加载：请求会让对方服务器知道你此刻读了这篇笔记</span><code>%s</code><button type="button" class="btn-quiet">加载这张图片</button></span>`,
				src.src, src.host, htmlEscape(captionOf(alt)), htmlEscape(src.host))
		}
	default:
		reason := src.reason
		if reason == "" {
			reason = "path-escape"
		}
		fmt.Fprintf(w, `<span class="mile-preview-image-card" data-img-kind="invalid" data-state="blocked" data-reason=%q><strong>%s</strong><span>%s</span></span>`,
			rejectReasonText[reason], htmlEscape(captionOf(alt)), htmlEscape(rejectReasonText[reason]))
	}
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

// ---------------------------------------------------------------------------
// chat reference transform (remarkChatReferences):
// text inside a chat message that is a vault path becomes a ref button.
// ---------------------------------------------------------------------------

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
		// start may not be preceded by word/./\::@/-; end may not be followed by word//-
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
	for i := 0; i < len(found); i++ {
		for j := i + 1; j < len(found); j++ {
			if found[j].start < found[i].start {
				found[i], found[j] = found[j], found[i]
			}
		}
	}
	return found
}

// transformChatRefs splits Text nodes containing vault paths or [[wiki]]
// into Link nodes stamped with data-ref-*. Links and CodeSpans are skipped
// (legacy remarkChatReferences only touches text nodes outside links).
func transformChatRefs(doc ast.Node, source []byte, r *kbRenderer) {
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

// ---------------------------------------------------------------------------
// Streaming trim (chatStreaming.trimStreamingText): fold an unfinished
// trailing table or link into a pending marker while streaming.
// ---------------------------------------------------------------------------

var fenceRe = regexp.MustCompile("^(?:```|~~~)")

func lastOpenFence(lines []string, beforeIndex int) int {
	open := -1
	for i := 0; i < beforeIndex && i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if !fenceRe.MatchString(t) {
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

// TrimResult is what TrimStreamingText cut out of the tail.
type TrimResult struct {
	Text        string
	Pending     bool
	PendingKind string // "table" | "link"
}

var (
	tableSepRe = regexp.MustCompile(`^\s*\|[\s\-:|]+\|\s*$`)
	linkTailRe = regexp.MustCompile(`\[([^\]]+)\]\([^)]*$`)
)

// TrimStreamingText mirrors legacy trimStreamingText: only when streaming,
// an unfinished trailing table block (no "| --- |" separator) or an
// unclosed "[text](" link is folded away so the renderer never sees it.
func TrimStreamingText(s string, streaming bool) TrimResult {
	if !streaming || s == "" {
		return TrimResult{Text: s}
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
			return TrimResult{Text: s}
		}
		block := lines[tableStart : tableEnd+1]
		sepIndex := -1
		for i, l := range block {
			if tableSepRe.MatchString(l) {
				sepIndex = i
				break
			}
		}
		if sepIndex == -1 {
			kept := strings.TrimRight(strings.Join(lines[:tableStart], "\n"), "\n")
			return TrimResult{Text: kept, Pending: true, PendingKind: "table"}
		}
		lastLine := strings.TrimSpace(lines[tableEnd])
		if !strings.HasSuffix(lastLine, "|") {
			kept := strings.TrimRight(strings.Join(lines[:tableEnd], "\n"), "\n")
			return TrimResult{Text: kept, Pending: true, PendingKind: "table"}
		}
	}
	tail := s
	if len(tail) > 80 {
		tail = tail[len(tail)-80:]
	}
	if m := linkTailRe.FindStringSubmatch(tail); m != nil {
		idx := strings.LastIndex(s, m[0])
		before := s[:idx]
		lineNo := strings.Count(before, "\n")
		if lastOpenFence(strings.Split(s, "\n"), lineNo) == -1 {
			kept := s[:idx] + m[1]
			return TrimResult{Text: kept, Pending: true, PendingKind: "link"}
		}
	}
	return TrimResult{Text: s}
}
