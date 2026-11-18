// Package index holds the search-side pure functions: tokenising, document
// parsing, excerpts and (from T04) link extraction and the SQLite store.
//
// The behaviour is ported from the legacy Electron app
// (src/shared/tokenize.ts and src/main/services/tokenize.ts); the legacy unit
// tests are ported alongside so the two implementations stay aligned.
package index

import (
	"strings"
	"unicode"
)

// MaxTokenizeLength mirrors the legacy cap: absurd input is truncated rather
// than allowed to hang the app. The legacy code counts UTF-16 units; this
// counts runes, which differ only for astral characters at the boundary.
const MaxTokenizeLength = 200_000

// Parsed is what the indexer keeps per document.
type Parsed struct {
	Title      string
	Headings   []string
	Tags       []string
	Body       string
	PathTokens []string
}

// isCJK mirrors the legacy character class
// /[぀-ヿ㐀-䶿一-鿿豈-﫿]/: kana, CJK extension A, and U+4E00..U+FAFF (the
// legacy 豈 is U+8C48, which sits inside the unified block, so the last two
// ranges merge).
//
// The legacy regex runs on UTF-16 code units without the u flag, so its last
// range also swallows the surrogate area D800..DFFF and the legacy tokenizer
// produced junk tokens built from half an emoji. Go works on code points, where
// a surrogate never appears; the renderer port skips surrogate code units
// explicitly so both sides agree (intentional difference, see PLAN-M1 T01).
func isCJK(r rune) bool {
	return (r >= 0x3040 && r <= 0x30FF) ||
		(r >= 0x3400 && r <= 0x4DBF) ||
		(r >= 0x4E00 && r <= 0xFAFF)
}

// isWordChar mirrors /[A-Za-z0-9_]/.
func isWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}

// isInnerWordChar mirrors the set that may stay inside a word when followed by
// a word character, so `v4.1` and `x-ray` survive but a trailing period does
// not.
func isInnerWordChar(r rune) bool {
	return r == '.' || r == '-' || r == '+' || r == '#'
}

// isJSSpace reports the characters ECMAScript treats as whitespace, used by
// \s and by String.prototype.trim. Go's unicode.IsSpace differs on U+0085 and
// U+FEFF, so the legacy set is spelled out to keep outputs identical.
func isJSSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x00A0, 0x1680, 0x2028, 0x2029,
		0x202F, 0x205F, 0x3000, 0xFEFF:
		return true
	}
	return r >= 0x2000 && r <= 0x200A
}

func trimJS(s string) string {
	return strings.TrimFunc(s, isJSSpace)
}

func trimLeftJS(s string) string {
	return strings.TrimLeftFunc(s, isJSSpace)
}

func trimRightJS(s string) string {
	return strings.TrimRightFunc(s, isJSSpace)
}

// Tokenize splits text into deduplicated lowercase tokens, keeping the order of
// first appearance.
//
// CJK runs become bigrams (a lone character stays a single token) and never
// bridge a punctuation boundary; ASCII words keep `.`, `-`, `+` and `#` inside
// them when a word character follows.
func Tokenize(input string) []string {
	if input == "" {
		return nil
	}
	runes := []rune(input)
	if len(runes) > MaxTokenizeLength {
		runes = runes[:MaxTokenizeLength]
	}

	out := make([]string, 0, 32)
	seen := make(map[string]struct{}, 32)
	add := func(tok string) {
		if tok == "" {
			return
		}
		if _, ok := seen[tok]; ok {
			return
		}
		seen[tok] = struct{}{}
		out = append(out, tok)
	}

	for i := 0; i < len(runes); {
		r := runes[i]
		if isCJK(r) {
			j := i
			for j < len(runes) && isCJK(runes[j]) {
				j++
			}
			run := runes[i:j]
			if len(run) == 1 {
				add(string(run))
			} else {
				for k := 0; k+1 < len(run); k++ {
					add(string(run[k : k+2]))
				}
			}
			i = j
			continue
		}
		if isWordChar(r) {
			j := i
			for j < len(runes) {
				if isWordChar(runes[j]) {
					j++
					continue
				}
				if isInnerWordChar(runes[j]) && j+1 < len(runes) && isWordChar(runes[j+1]) {
					j++
					continue
				}
				break
			}
			add(strings.ToLower(string(runes[i:j])))
			i = j
			continue
		}
		i++
	}
	return out
}

// TokenizePath emits, per path segment, the whole segment plus its tokens plus
// a split on `.`, `-` and `_`, so "huge" finds "huge.md" even though the
// tokenizer keeps the dot inside a word.
func TokenizePath(path string) []string {
	out := make([]string, 0, 16)
	seen := make(map[string]struct{}, 16)
	add := func(tok string) {
		if tok == "" {
			return
		}
		if _, ok := seen[tok]; ok {
			return
		}
		seen[tok] = struct{}{}
		out = append(out, tok)
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "" {
			continue
		}
		add(strings.ToLower(segment))
		for _, token := range Tokenize(segment) {
			add(token)
			for _, part := range splitInnerPunct(token) {
				add(part)
			}
		}
	}
	return out
}

// splitInnerPunct mirrors token.split(/[.\-_]+/).
func splitInnerPunct(token string) []string {
	return strings.FieldsFunc(token, func(r rune) bool {
		return r == '.' || r == '-' || r == '_'
	})
}

// splitLines mirrors content.split(/\r?\n/): a trailing newline yields a final
// empty line, and a lone \r is kept.
func splitLines(content string) []string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSuffix(line, "\r")
	}
	return lines
}

type frontmatter struct {
	fields    map[string]string
	tags      []string
	bodyStart int
}

// parseFrontmatter deliberately understands only flat `key: value` pairs and
// the two tag forms; an unterminated fence is tolerated rather than an error.
func parseFrontmatter(lines []string) frontmatter {
	front := frontmatter{fields: map[string]string{}}
	if len(lines) == 0 || trimJS(lines[0]) != "---" {
		return front
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if trimJS(lines[i]) == "---" {
			end = i
			break
		}
	}
	limit := len(lines)
	if end != -1 {
		limit = end
	}

	collectingTags := false
	for i := 1; i < limit; i++ {
		line := lines[i]

		if collectingTags {
			if item, ok := yamlListItem(line); ok {
				front.tags = append(front.tags, stripQuotes(item))
				continue
			}
		}

		key, value, ok := yamlField(line)
		if !ok {
			continue
		}
		collectingTags = key == "tags" && value == ""

		if key == "tags" && value != "" {
			inner := value
			if strings.HasPrefix(inner, "[") && strings.HasSuffix(inner, "]") {
				inner = inner[1 : len(inner)-1]
			}
			for _, part := range strings.Split(inner, ",") {
				if tag := stripQuotes(trimJS(part)); tag != "" {
					front.tags = append(front.tags, tag)
				}
			}
			continue
		}

		if value != "" {
			front.fields[key] = stripQuotes(value)
		}
	}

	front.bodyStart = len(lines)
	if end != -1 {
		front.bodyStart = end + 1
	}
	return front
}

// yamlListItem matches /^\s*-\s+(.+)$/ and returns the trimmed item.
func yamlListItem(line string) (string, bool) {
	i := 0
	for i < len(line) && isJSSpace(rune(line[i])) {
		i++
	}
	if i >= len(line) || line[i] != '-' {
		return "", false
	}
	i++
	spaces := 0
	for i < len(line) && isJSSpace(rune(line[i])) {
		i++
		spaces++
	}
	if spaces == 0 || i >= len(line) {
		return "", false
	}
	return trimJS(line[i:]), true
}

// yamlField matches /^([A-Za-z_][A-Za-z0-9_-]*)\s*:\s*(.*)$/ and returns the
// lowercased key and the trimmed value.
func yamlField(line string) (string, string, bool) {
	i := 0
	if i >= len(line) {
		return "", "", false
	}
	first := rune(line[i])
	if !(first == '_' || (first >= 'A' && first <= 'Z') || (first >= 'a' && first <= 'z')) {
		return "", "", false
	}
	i++
	for i < len(line) {
		r := rune(line[i])
		if r == '_' || r == '-' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			i++
			continue
		}
		break
	}
	key := strings.ToLower(line[:i])
	for i < len(line) && isJSSpace(rune(line[i])) {
		i++
	}
	if i >= len(line) || line[i] != ':' {
		return "", "", false
	}
	i++
	for i < len(line) && isJSSpace(rune(line[i])) {
		i++
	}
	return key, trimJS(line[i:]), true
}

func stripQuotes(value string) string {
	if len(value) >= 2 {
		first, last := value[0], value[len(value)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}

// ParseDocument extracts the fields the index keeps: title, headings, tags,
// body and path tokens.
func ParseDocument(path, content string) Parsed {
	lines := splitLines(content)
	front := parseFrontmatter(lines)

	headings := make([]string, 0, 8)
	inFence := false
	for i := front.bodyStart; i < len(lines); i++ {
		line := lines[i]
		if isFence(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if heading, ok := parseHeading(line); ok {
			headings = append(headings, heading)
		}
	}

	fileName := path
	if at := strings.LastIndex(path, "/"); at >= 0 {
		fileName = path[at+1:]
	}
	title := fileName
	if len(headings) > 0 {
		title = headings[0]
	}
	if frontTitle, ok := front.fields["title"]; ok {
		title = frontTitle
	}

	tags := make([]string, len(front.tags))
	for i, tag := range front.tags {
		tags[i] = strings.ToLower(tag)
	}

	return Parsed{
		Title:      title,
		Headings:   headings,
		Tags:       tags,
		Body:       strings.Join(lines[front.bodyStart:], "\n"),
		PathTokens: TokenizePath(path),
	}
}

// isFence mirrors /^\s*(```|~~~)/.
func isFence(line string) bool {
	trimmed := trimLeftJS(line)
	return strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")
}

// parseHeading mirrors /^(#{1,6})\s+(.+?)\s*#*\s*$/ and returns the trimmed
// heading text.
func parseHeading(line string) (string, bool) {
	i := 0
	for i < len(line) && line[i] == '#' {
		i++
	}
	if i == 0 || i > 6 {
		return "", false
	}
	spaces := 0
	for i < len(line) && isJSSpace(rune(line[i])) {
		i++
		spaces++
	}
	if spaces == 0 || i >= len(line) {
		return "", false
	}
	rest := trimRightJS(line[i:])
	rest = strings.TrimRight(rest, "#")
	return trimJS(rest), true
}

// BuildExcerpt returns a window around the earliest query token, or "" when no
// token occurs — it never fabricates a lead-in that does not contain the hit.
// Positions are counted in runes.
func BuildExcerpt(body string, queryTokens []string, radius int) string {
	if body == "" || len(queryTokens) == 0 {
		return ""
	}
	original := []rune(body)
	haystack := make([]rune, len(original))
	for i, r := range original {
		haystack[i] = unicode.ToLower(r)
	}

	best := -1
	for _, token := range queryTokens {
		needle := []rune(strings.ToLower(token))
		if len(needle) == 0 {
			continue
		}
		if at := indexRunes(haystack, needle); at != -1 && (best == -1 || at < best) {
			best = at
		}
	}
	if best == -1 {
		return ""
	}

	start := best - radius
	if start < 0 {
		start = 0
	}
	end := best + radius
	if end > len(original) {
		end = len(original)
	}
	prefix := ""
	if start > 0 {
		prefix = "…"
	}
	suffix := ""
	if end < len(original) {
		suffix = "…"
	}
	return prefix + collapseSpace(original[start:end]) + suffix
}

func indexRunes(haystack, needle []rune) int {
	if len(needle) == 0 || len(needle) > len(haystack) {
		return -1
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for k, r := range needle {
			if haystack[i+k] != r {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// collapseSpace mirrors .replace(/\s+/g, ' ').trim().
func collapseSpace(runes []rune) string {
	var b strings.Builder
	pendingSpace := false
	for _, r := range runes {
		if isJSSpace(r) {
			pendingSpace = b.Len() > 0
			continue
		}
		if pendingSpace {
			b.WriteByte(' ')
			pendingSpace = false
		}
		b.WriteRune(r)
	}
	return b.String()
}
