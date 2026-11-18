package index

import (
	"strings"
)

// LinkKind mirrors the legacy categories (shared/links.ts).
const (
	LinkWiki     = "wiki"
	LinkInternal = "internal"
	LinkExternal = "external"
	LinkOther    = "other"
)

// LinkRef is one extracted link. Line is 0-based; Column is the rune index of
// the match start, kept for editor decorations later.
type LinkRef struct {
	Kind   string
	Target string
	Text   string
	Line   int
	Column int
}

// LinkTarget is a document the resolver may point at.
type LinkTarget struct {
	Path  string
	Title string
}

// Resolver resolves link targets with the legacy three layers, each requiring
// a unique hit: exact relative path (extension optional), file name, title.
type Resolver struct {
	byPath  map[string][]string
	byNoExt map[string][]string
	byName  map[string][]string
	byTitle map[string][]string
	count   int
}

// NewResolver builds the hash maps. Ignored (system area) documents are not
// candidates: the link graph excludes the system area (PRD FR-LINK).
func NewResolver(targets []LinkTarget) *Resolver {
	r := &Resolver{
		byPath:  map[string][]string{},
		byNoExt: map[string][]string{},
		byName:  map[string][]string{},
		byTitle: map[string][]string{},
	}
	for _, t := range targets {
		if t.Path == "" {
			continue
		}
		r.count++
		lower := strings.ToLower(t.Path)
		r.byPath[lower] = append(r.byPath[lower], t.Path)
		r.byNoExt[stripExt(lower)] = append(r.byNoExt[stripExt(lower)], t.Path)
		base := lower
		if at := strings.LastIndex(base, "/"); at >= 0 {
			base = base[at+1:]
		}
		r.byName[stripExt(base)] = append(r.byName[stripExt(base)], t.Path)
		title := strings.ToLower(strings.TrimSpace(t.Title))
		if title != "" {
			r.byTitle[title] = append(r.byTitle[title], t.Path)
		}
	}
	return r
}

// Size reports how many documents are candidates (for cache keys and tests).
func (r *Resolver) Size() int { return r.count }

// Resolve returns the resolved path when exactly one candidate matched, and the
// candidate count otherwise: 0 = no candidate, >1 = ambiguous (never pick one).
func (r *Resolver) Resolve(target string) (string, int) {
	needle := strings.ToLower(strings.TrimSpace(target))
	if needle == "" {
		return "", 0
	}
	if path, n := unique(union(r.byPath[needle], r.byNoExt[stripExt(needle)])); n != 0 {
		return path, n
	}
	if path, n := unique(r.byName[stripExt(needle)]); n != 0 {
		return path, n
	}
	return unique(r.byTitle[needle])
}

func union(a, b []string) []string {
	if len(b) == 0 {
		return a
	}
	out := make([]string, 0, len(a)+len(b))
	out = append(out, a...)
	out = append(out, b...)
	return out
}

// unique collapses duplicates and reports the count.
func unique(paths []string) (string, int) {
	if len(paths) == 0 {
		return "", 0
	}
	seen := make(map[string]bool, len(paths))
	out := paths[:0:0]
	for _, p := range paths {
		if seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	if len(out) == 1 {
		return out[0], 1
	}
	return "", len(out)
}

func stripExt(name string) string {
	dot := strings.LastIndex(name, ".")
	slash := strings.LastIndex(name, "/")
	if dot > slash && dot > 0 {
		return name[:dot]
	}
	return name
}

// ExtractLinks scans Markdown for wikilinks and inline Markdown links. Code
// fences and inline code are skipped, images are not references, and empty
// targets are dropped (shared/links.ts).
func ExtractLinks(content string) []LinkRef {
	lines := strings.Split(content, "\n")
	links := make([]LinkRef, 0, 16)
	inFence := false
	for lineNumber, raw := range lines {
		raw = strings.TrimSuffix(raw, "\r")
		if isFence(raw) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		line := blankInlineCode(raw)
		links = collectWikiLinks(line, lineNumber, links)
		links = collectMarkdownLinks(line, lineNumber, links)
	}
	return links
}

// blankInlineCode replaces backtick spans with spaces of the same length so
// columns keep pointing at the original text.
func blankInlineCode(line string) string {
	var b strings.Builder
	b.Grow(len(line))
	inCode := false
	for _, r := range line {
		if r == '`' {
			inCode = !inCode
			b.WriteRune(' ')
			continue
		}
		if inCode {
			b.WriteRune(' ')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func collectWikiLinks(line string, lineNumber int, out []LinkRef) []LinkRef {
	for i := 0; i+3 < len(line); {
		if line[i] != '[' || line[i+1] != '[' {
			i++
			continue
		}
		end := strings.Index(line[i+2:], "]]")
		if end < 0 {
			break
		}
		innerStart := i + 2
		inner := line[innerStart : innerStart+end]
		if strings.ContainsAny(inner, "[]") {
			i++
			continue
		}
		target, text := inner, inner
		if pipe := strings.Index(inner, "|"); pipe >= 0 {
			target = inner[:pipe]
			text = inner[pipe+1:]
		}
		target = strings.TrimSpace(target)
		text = strings.TrimSpace(text)
		if target != "" {
			if text == "" {
				text = target
			}
			out = append(out, LinkRef{
				Kind: LinkWiki, Target: target, Text: text,
				Line: lineNumber, Column: runeIndex(line, i),
			})
		}
		i = innerStart + end + 2
	}
	return out
}

// collectMarkdownLinks handles `[text](target)` with balanced parentheses in
// the target and skips images (`![...](...)`).
func collectMarkdownLinks(line string, lineNumber int, out []LinkRef) []LinkRef {
	for i := 0; i+1 < len(line); i++ {
		if line[i] != ']' || line[i+1] != '(' {
			continue
		}
		open := strings.LastIndexByte(line[:i], '[')
		if open < 0 || strings.ContainsAny(line[open+1:i], "[]") {
			continue
		}
		if open > 0 && line[open-1] == '!' {
			continue
		}
		target, end, ok := scanMarkdownTarget(line, i+2)
		if !ok {
			continue
		}
		text := line[open+1 : i]
		if target != "" {
			out = append(out, LinkRef{
				Kind: classifyTarget(target), Target: target, Text: text,
				Line: lineNumber, Column: runeIndex(line, open),
			})
		}
		i = end
	}
	return out
}

// scanMarkdownTarget reads until the parenthesis that closes the link, keeping
// balanced pairs inside the target (`Foo_(bar)`, `javascript:alert(1)`).
func scanMarkdownTarget(line string, start int) (string, int, bool) {
	depth := 0
	var b strings.Builder
	for j := start; j < len(line); j++ {
		c := line[j]
		switch c {
		case ' ', '\t':
			return "", 0, false
		case '(':
			depth++
			b.WriteByte(c)
		case ')':
			if depth == 0 {
				return strings.TrimSpace(b.String()), j, true
			}
			depth--
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	return "", 0, false
}

func classifyTarget(target string) string {
	lower := strings.ToLower(target)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return LinkExternal
	}
	if isScheme(lower) {
		return LinkOther
	}
	return LinkInternal
}

// isScheme mirrors /^[a-z][a-z0-9+.-]*:/.
func isScheme(s string) bool {
	if s == "" || s[0] < 'a' || s[0] > 'z' {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '+' || c == '.' || c == '-' {
			continue
		}
		return c == ':'
	}
	return false
}

func runeIndex(s string, byteAt int) int {
	count := 0
	for i := range s {
		if i >= byteAt {
			return count
		}
		count++
	}
	return count
}
