package render

import (
	"regexp"
	"strings"
	"testing"
)

// entityStripRe removes HTML entities before scanning for executable
// attributes: `&lt;iframe src=javascript:…&gt;` is escaped text, not a tag.
var entityStripRe = regexp.MustCompile(`&(?:amp|lt|gt|quot|#39|#34|#[0-9]+|#x[0-9a-f]+);`)

// Security: whatever the input, the sanitized output must never contain an
// executable payload. Runs both variants on every seed.
func FuzzRender(f *testing.F) {
	seeds := []string{
		"# t\n\n<script>alert(1)</script>\n\n[x](javascript:alert(1))\n\n![i](javascript:alert(1))\n",
		"| a |\n| - |\n| <img src=x onerror=alert(1)> |\n",
		"[[x]] [[a|b]] [y](../out) ![p](../../etc/passwd.png)\n",
		"<div onclick='x'>text</div> 前文 <em>斜</em>\n",
		"```\n<script>\n```\n",
		"%2e%2e/x.png data:image/svg+xml;base64,AAA //cdn/a.png",
		"![x](data:text/html;base64,PHNjcmlwdD4=)\n",
		"<iframe src=javascript:alert(1)></iframe>\n",
		"[a](vbscript:msgbox(1)) ![b](file:///etc/passwd) [c](//evil)",
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, src []byte) {
		for _, v := range []Variant{VariantReader, VariantChat} {
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("panic variant=%s src=%q: %v", v, src[:minInt(80, len(src))], r)
					}
				}()
				res, err := Render(src, Options{Variant: v})
				if err != nil {
					t.Fatalf("render err: %v", err)
				}
				out := strings.ToLower(res.HTML)
				if strings.Contains(out, "<script") {
					t.Fatalf("script leaked variant=%s src=%q out=%q", v, src[:minInt(80, len(src))], out[:minInt(400, len(out))])
				}
				if regexp.MustCompile(`<[a-z][^>]*\son\w+\s*=`).MatchString(out) {
					t.Fatalf("on* leaked variant=%s out=%q", v, out[:minInt(400, len(out))])
				}
				// "javascript:" only counts inside a real attribute value —
				// i.e. inside a live tag (<tag attr=javascript:…>). When it
				// sits in escaped text (&lt;iframe src=javascript:…&gt;) it
				// is inert and must not count. We therefore first strip all
				// escaped entities, then look for the pattern in tags.
				stripped := entityStripRe.ReplaceAllString(out, "")
				if regexp.MustCompile(`<[a-z][^>]*(?:href|src|data-target|data-src|data-vault|data-vault-path)\s*=\s*["']?\s*javascript:`).MatchString(stripped) {
					t.Fatalf("javascript: leaked variant=%s out=%q", v, out[:minInt(400, len(out))])
				}
			}()
		}
	})
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Table-driven security cases: each input must come out inert in both
// variants (the payload text may remain, but never as markup).
func TestSecurityPayloads(t *testing.T) {
	cases := []struct {
		name string
		md   string
	}{
		{"script block", "前文。\n\n<script>alert(1)</script>\n\n后文。"},
		{"img onerror", `正文 <img src="x.png" onerror="alert(1)"> 结束。`},
		{"javascript link", "坏 [链接](javascript:alert(1))。"},
		{"javascript image", "![i](javascript:alert(1))"},
		{"data html image", "![x](data:text/html;base64,PHNjcmlwdD4=)"},
		{"iframe", "前文。\n\n<iframe src=\"javascript:alert(1)\"></iframe>\n\n后文。"},
		{"inline event attr", `<div onclick="steal()">文字</div>`},
		{"vbscript link", "[a](vbscript:msgbox(1))"},
		{"file link", "[f](file:///C:/Windows/win.ini)"},
		{"protocol-relative link", "[p](//evil.com/x)"},
		{"comment", "前 <!-- <script>alert(1)</script> --> 后。"},
	}
	for _, tc := range cases {
		for _, v := range []Variant{VariantReader, VariantChat} {
			t.Run(tc.name+"/"+string(v), func(t *testing.T) {
				res, err := Render([]byte(tc.md), Options{Variant: v})
				if err != nil {
					t.Fatalf("render: %v", err)
				}
				out := strings.ToLower(res.HTML)
				if strings.Contains(out, "<script") || strings.Contains(out, "<iframe") {
					t.Fatalf("executable tag leaked: %q", res.HTML)
				}
				if regexp.MustCompile(`<[a-z][^>]*\son\w+\s*=`).MatchString(out) {
					t.Fatalf("event attribute leaked: %q", res.HTML)
				}
				if regexp.MustCompile(`(?:href|src|data-target|data-src|data-vault|data-vault-path)\s*=\s*"?\s*(?:javascript|vbscript|file):`).MatchString(out) {
					t.Fatalf("dangerous scheme leaked: %q", res.HTML)
				}
				if strings.Contains(out, `data-img-kind="data"`) && strings.Contains(out, "data:text/html") {
					t.Fatalf("non-image data uri displayed: %q", res.HTML)
				}
			})
		}
	}
}

// Anchors: "#xx" renders as a link-anchor carrying the unescaped target
// and headings get no id (D-007 / legacy readerLinks.ts:31).
func TestAnchorsAndHeadings(t *testing.T) {
	res, err := Render([]byte("## 部署步骤\n\n见 [部署](#部署步骤)。"), Options{Variant: VariantReader})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res.HTML, `id=`) && regexp.MustCompile(`<h[1-6][^>]*\sid=`).MatchString(res.HTML) {
		t.Fatalf("heading id leaked: %q", res.HTML)
	}
	if !strings.Contains(res.HTML, `class="link-anchor"`) {
		t.Fatalf("anchor class missing: %q", res.HTML)
	}
	if !strings.Contains(res.HTML, `data-kind="anchor"`) {
		t.Fatalf("anchor kind missing: %q", res.HTML)
	}
	if !strings.Contains(res.HTML, `data-target="部署步骤"`) {
		t.Fatalf("anchor target should drop the # prefix: %q", res.HTML)
	}
	// No scroll behaviour: the anchor must not carry an href at all.
	if strings.Contains(res.HTML, `href="#`) {
		t.Fatalf("anchor must not emit href: %q", res.HTML)
	}
}

// Link classification table: FR-LINK three buckets + invalid → link-plain.
func TestLinkClassification(t *testing.T) {
	okResolver := stubOKResolver{}
	cases := []struct {
		name     string
		md       string
		want     []string
		notWant  []string
		resolver LinkResolver
	}{
		{"https external", "[a](https://x.com)", []string{`data-kind="external"`, `data-target="https://x.com"`}, nil, nil},
		{"anchor", "[a](#x)", []string{`data-kind="anchor"`, `data-target="x"`}, nil, nil},
		{"vault doc ok", "[a](dir/note.md)", []string{`data-kind="doc"`, `data-status="ok"`, `data-target="dir/note.md"`}, nil, okResolver},
		{"vault doc missing", "[a](dir/note.md)", []string{`data-kind="doc"`, `data-status="missing"`, `（未创建）`}, nil, nil},
		{"javascript invalid", "[a](javascript:x)", []string{`class="link-plain"`}, []string{"<a "}, nil},
		{"mailto invalid", "[a](mailto:a@b.c)", []string{`class="link-plain"`}, []string{"<a "}, nil},
		{"dotdot invalid", "[a](../out.md)", []string{`class="link-plain"`}, []string{"<a "}, nil},
		{"absolute invalid", "[a](/etc/passwd)", []string{`class="link-plain"`}, []string{"<a "}, nil},
		{"drive invalid", "[a](C:/x/y.md)", []string{`class="link-plain"`}, []string{"<a "}, nil},
		{"unc invalid", `[a](\\server\share\a.md)`, []string{`class="link-plain"`}, []string{"<a "}, nil},
		{"data link invalid", "[a](data:image/png;base64,AAA)", []string{`class="link-plain"`}, []string{"<a "}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := Render([]byte(tc.md), Options{Variant: VariantReader, Resolver: tc.resolver})
			if err != nil {
				t.Fatal(err)
			}
			for _, w := range tc.want {
				if !strings.Contains(res.HTML, w) {
					t.Errorf("want %q in %q", w, res.HTML)
				}
			}
			for _, nw := range tc.notWant {
				if strings.Contains(res.HTML, nw) {
					t.Errorf("notWant %q in %q", nw, res.HTML)
				}
			}
		})
	}
}

type stubOKResolver struct{}

func (stubOKResolver) ResolveLink(target string) (string, string, bool) {
	return target, "internal", true
}
func (stubOKResolver) ResolveImage(src string) (string, bool) { return src, true }

// Wikilink three states + empty-target fallback + alias.
func TestWikilinks(t *testing.T) {
	resolver := stubOKResolver{}
	cases := []struct {
		name     string
		md       string
		variant  Variant
		resolver LinkResolver
		want     []string
	}{
		{"resolved reader", "见 [[note-a]]。", VariantReader, resolver,
			[]string{`data-kind="wiki"`, `data-status="ok"`, `data-target="note-a"`, `data-wiki="note-a"`}},
		{"resolved chat", "见 [[note-a]]。", VariantChat, resolver,
			[]string{`class="chat-ref"`, `data-ref-kind="wiki"`, `data-status="ok"`}},
		{"missing reader", "见 [[不存在]]。", VariantReader, nil,
			[]string{`wiki-missing`, `data-status="missing"`, `（未创建）`}},
		{"unresolved chat", "见 [[不存在]]。", VariantChat, nil,
			// Without a resolver the chat variant claims nothing: existence is
			// decided by the click-time read (T12), so no 找不到 note here.
			[]string{`class="chat-ref"`, `data-status="unknown"`}},
		{"alias label", "见 [[note-a|别名]]。", VariantReader, resolver,
			[]string{`>别名</a>`}},
		{"empty target literal", "空 [[ ]] 保留。", VariantReader, resolver,
			[]string{`空 [[ ]] 保留`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := Render([]byte(tc.md), Options{Variant: tc.variant, Resolver: tc.resolver})
			if err != nil {
				t.Fatal(err)
			}
			for _, w := range tc.want {
				if !strings.Contains(res.HTML, w) {
					t.Errorf("want %q in %q", w, res.HTML)
				}
			}
		})
	}
}

// Images: each classified source renders the contract shape.
func TestImages(t *testing.T) {
	cases := []struct {
		name    string
		md      string
		variant Variant
		want    []string
		notWant []string
	}{
		{"vault reader", "![截图](raws/a.png)", VariantReader,
			[]string{`data-img-kind="vault"`, `data-vault-path="raws/a.png"`, `src="http://kbimg.localhost/raws%2Fa.png"`}, nil},
		{"data reader", "![内联](data:image/png;base64,iVBORw0KGgo=)", VariantReader,
			[]string{`data-img-kind="data"`, `src="data:image/png;base64,iVBORw0KGgo="`}, nil},
		{"https reader", "![远程](https://cdn.example.com/a.png)", VariantReader,
			[]string{`data-img-kind="remote"`, `data-state="remote-gate"`, `data-host="cdn.example.com"`, `加载这张图片`}, nil},
		{"http reader", "![明文](http://example.com/a.png)", VariantReader,
			[]string{`data-img-kind="remote"`, `data-state="blocked"`, `不支持明文 HTTP`}, nil},
		{"dotdot reader", "![逃逸](../../secret.png)", VariantReader,
			[]string{`data-img-kind="invalid"`, `data-reason="路径超出知识库范围"`}, nil},
		{"badext reader", "![文本](a.txt)", VariantReader,
			[]string{`data-img-kind="invalid"`, `data-reason="不是支持的图片格式"`}, nil},
		{"vault chat", "![截图](raws/a.png)", VariantChat,
			[]string{`class="img-label vault"`, `data-vault="raws/a.png"`, `图片：截图`},
			[]string{`<img`}},
		{"remote chat", "![远程](https://cdn.example.com/a.png)", VariantChat,
			[]string{`class="img-label"`, `（远程图片不加载）`},
			[]string{`<img`}},
		{"data chat", "![内联](data:image/png;base64,AAA=)", VariantChat,
			[]string{`class="img-label"`, `（远程图片不加载）`},
			[]string{`<img`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := Render([]byte(tc.md), Options{Variant: tc.variant, Resolver: stubOKResolver{}})
			if err != nil {
				t.Fatal(err)
			}
			if !res.HasImages {
				t.Errorf("HasImages should be true")
			}
			for _, w := range tc.want {
				if !strings.Contains(res.HTML, w) {
					t.Errorf("want %q in %q", w, res.HTML)
				}
			}
			for _, nw := range tc.notWant {
				if strings.Contains(res.HTML, nw) {
					t.Errorf("notWant %q in %q", nw, res.HTML)
				}
			}
		})
	}
}

// Custom ImageURL is honoured; nil resolver degrades vault images.
func TestImageURLAndNilResolver(t *testing.T) {
	res, err := Render([]byte("![a](x/y.png)"), Options{
		Variant:  VariantReader,
		Resolver: stubOKResolver{},
		ImageURL: func(p string) string { return "https://img.test/" + p },
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.HTML, `src="https://img.test/x/y.png"`) {
		t.Fatalf("custom ImageURL not used: %q", res.HTML)
	}
	res, err = Render([]byte("![a](x/y.png)"), Options{Variant: VariantReader})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.HTML, `data-img-kind="invalid"`) {
		t.Fatalf("nil resolver should degrade vault image: %q", res.HTML)
	}
}

// Whitelist self-check: renderer output must be invariant under sanitize —
// anything it emits is already inside the whitelist (whitelist.md §自检).
func TestSanitizeInvariant(t *testing.T) {
	inputs := []string{
		"# t\n\n**b** *i* `c` ~~d~~ [x](https://a.b) [[w]]\n\n- [x] task\n\n| a |\n| - |\n| 1 |\n\n```go\nfmt.Println(1)\n```\n\n> quote\n\n![i](a.png)\n\n<div>raw</div>",
	}
	for _, src := range inputs {
		for _, v := range []Variant{VariantReader, VariantChat} {
			res, err := Render([]byte(src), Options{Variant: v, Resolver: stubOKResolver{}})
			if err != nil {
				t.Fatal(err)
			}
			if again := normalizeEntities(sanitize(res.HTML)); again != res.HTML {
				t.Fatalf("variant=%s sanitize not invariant:\nfirst : %q\nsecond: %q", v, res.HTML, again)
			}
		}
	}
}

// CRLF normalisation: a CRLF source renders like LF.
func TestCRLF(t *testing.T) {
	lf, _ := Render([]byte("a\n\nb"), Options{})
	crlf, _ := Render([]byte("a\r\n\r\nb"), Options{})
	if lf.HTML != crlf.HTML {
		t.Fatalf("CRLF differs: %q vs %q", lf.HTML, crlf.HTML)
	}
}

// Frontmatter renders as body text (D-006): an <hr> then an h2 holding
// the "title: ..." lines.
func TestFrontmatterAsBody(t *testing.T) {
	res, err := Render([]byte("---\ntitle: 测试文档\ntags: [a, b]\n---\n\n正文。"), Options{Variant: VariantReader})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.HTML, "<hr>") || !strings.Contains(res.HTML, "title: 测试文档") {
		t.Fatalf("frontmatter not rendered as body: %q", res.HTML)
	}
}

// Streaming trim: the trailing unfinished table/link folds to pending.
func TestTrimStreamingText(t *testing.T) {
	cases := []struct {
		name     string
		md       string
		wantKind string
		wantText string
	}{
		{"table header only", "前文。\n\n| 分组 | Skill |", "table", "前文。"},
		{"table rows no sep", "前文。\n\n| a | b |\n| 1 | 2 |", "table", "前文。"},
		{"table done passthrough", "前文。\n\n| a | b |\n| --- | --- |\n| 1 | 2 |", "", "前文。\n\n| a | b |\n| --- | --- |\n| 1 | 2 |"},
		{"table partial last row", "前文。\n\n| a | b |\n| --- | --- |\n| 1 | 2", "table", "前文。\n\n| a | b |\n| --- | --- |"},
		{"link open", "前面 OK [示例](https://exa", "link", "前面 OK 示例"},
		{"link empty dest", "看 [链接](", "link", "看 链接"},
		{"fence open passthrough", "前文。\n\n```ts\nconst x = 1", "", "前文。\n\n```ts\nconst x = 1"},
		{"table inside fence", "```\n| a | b |\n```", "", "```\n| a | b |\n```"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := TrimStreamingText(tc.md, true)
			if got.PendingKind != tc.wantKind {
				t.Errorf("kind = %q want %q", got.PendingKind, tc.wantKind)
			}
			if got.Text != tc.wantText {
				t.Errorf("text = %q want %q", got.Text, tc.wantText)
			}
		})
	}
	// non-streaming is a passthrough
	if tr := TrimStreamingText("| a |", false); tr.Pending || tr.Text != "| a |" {
		t.Errorf("non-streaming should passthrough: %+v", tr)
	}
}

// Chat refs: a bare vault path in text becomes a chat-ref button.
func TestChatRefs(t *testing.T) {
	res, err := Render([]byte("建议看 skills/react/SKILL.md 文件。"), Options{Variant: VariantChat, Resolver: stubOKResolver{}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.HTML, `class="chat-ref"`) || !strings.Contains(res.HTML, `data-ref-kind="path"`) || !strings.Contains(res.HTML, `data-ref-target="skills/react/SKILL.md"`) {
		t.Fatalf("chat ref missing: %q", res.HTML)
	}
	// reader variant leaves the path as plain text.
	res, err = Render([]byte("建议看 skills/react/SKILL.md 文件。"), Options{Variant: VariantReader})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res.HTML, "chat-ref") {
		t.Fatalf("reader must not emit chat-ref: %q", res.HTML)
	}
	// Inline code whole-path also becomes a ref in chat.
	res, err = Render([]byte("请看 `docs/plan.md` 文件。"), Options{Variant: VariantChat, Resolver: stubOKResolver{}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.HTML, `data-ref-target="docs/plan.md"`) {
		t.Fatalf("inline-code ref missing: %q", res.HTML)
	}
}
