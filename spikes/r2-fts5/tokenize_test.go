// tokenize_test.go — 移植旧版 tests/unit/tokenize.test.ts 与 searchHighlight.test.ts 的全部用例。
// 表驱动，每条注明对应的旧版 it() 名称，保证口径一致可回查。
package main

import (
	"reflect"
	"strings"
	"testing"
)

func contains(ts []string, s string) bool {
	for _, t := range ts {
		if t == s {
			return true
		}
	}
	return false
}

// ---- 旧版 describe('tokenize') ----

// 'splits english on word boundaries and lowercases'
func TestTokenizeEnglishLowercases(t *testing.T) {
	ts := tokenize("Hello World")
	if !contains(ts, "hello") || !contains(ts, "world") {
		t.Fatalf("missing tokens: %v", ts)
	}
	if contains(ts, "Hello") {
		t.Fatalf("should be lowercase: %v", ts)
	}
}

// 'keeps digits and version-like tokens together'
func TestTokenizeVersionTokens(t *testing.T) {
	ts := tokenize("deepseek v4.1 release")
	for _, want := range []string{"deepseek", "v4.1", "release"} {
		if !contains(ts, want) {
			t.Fatalf("missing %q in %v", want, ts)
		}
	}
}

// 'does not split camel case into separate words'
func TestTokenizeCamelCase(t *testing.T) {
	got := tokenize("getUserName")
	want := []string{"getusername"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

// 'emits cjk bigrams so partial chinese phrases still match'
func TestTokenizeCjkBigrams(t *testing.T) {
	ts := tokenize("知识库管理")
	for _, want := range []string{"知识", "识库", "库管", "管理"} {
		if !contains(ts, want) {
			t.Fatalf("missing %q in %v", want, ts)
		}
	}
}

// 'emits a single-character token for a lone cjk character'
func TestTokenizeLoneCjk(t *testing.T) {
	got := tokenize("中")
	want := []string{"中"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

// 'handles mixed chinese and english in one string'
func TestTokenizeMixed(t *testing.T) {
	ts := tokenize("React 状态管理 best practice")
	for _, want := range []string{"react", "状态", "态管", "best", "practice"} {
		if !contains(ts, want) {
			t.Fatalf("missing %q in %v", want, ts)
		}
	}
}

// 'treats punctuation and whitespace as separators'
func TestTokenizeSeparators(t *testing.T) {
	got := tokenize("alpha, beta; gamma\n\tdelta")
	want := []string{"alpha", "beta", "gamma", "delta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

// 'does not bridge a bigram across a punctuation boundary'
func TestTokenizeNoCrossPunctuation(t *testing.T) {
	if contains(tokenize("知识库。管理"), "库管") {
		t.Fatal("bigram crossed punctuation boundary")
	}
}

// 'returns an empty list for empty or symbol-only input'
func TestTokenizeEmpty(t *testing.T) {
	if got := tokenize(""); len(got) != 0 {
		t.Fatalf("empty input got %v", got)
	}
	if got := tokenize("!!! ---"); len(got) != 0 {
		t.Fatalf("symbol-only input got %v", got)
	}
}

// 'deduplicates repeated tokens'
func TestTokenizeDedup(t *testing.T) {
	got := tokenize("test test test")
	count := 0
	for _, tok := range got {
		if tok == "test" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected 1 'test', got %d in %v", count, got)
	}
}

// 'truncates absurdly long input instead of hanging'
func TestTokenizeTruncates(t *testing.T) {
	got := tokenize(strings.Repeat("a", 2_000_000))
	if len(got) == 0 {
		t.Fatal("got no tokens")
	}
	if len(got) >= 500_000 {
		t.Fatalf("too many tokens: %d", len(got))
	}
}

// ---- 旧版 describe('parseDocument') ----

// 'extracts a frontmatter title and tags'
func TestParseFrontmatterTitleTags(t *testing.T) {
	doc := parseDocument("skills/react/SKILL.md",
		strings.Join([]string{"---", "title: React 状态管理", "tags: [react, state]", "---", "", "body text"}, "\n"))
	if doc.title != "React 状态管理" {
		t.Fatalf("title=%q", doc.title)
	}
	if !reflect.DeepEqual(doc.tags, []string{"react", "state"}) {
		t.Fatalf("tags=%v", doc.tags)
	}
}

// 'supports a yaml list form for tags'
func TestParseYamlListTags(t *testing.T) {
	doc := parseDocument("a.md",
		strings.Join([]string{"---", "tags:", "  - alpha", "  - beta", "---", "body"}, "\n"))
	if !reflect.DeepEqual(doc.tags, []string{"alpha", "beta"}) {
		t.Fatalf("tags=%v", doc.tags)
	}
}

// 'falls back to the first markdown heading when frontmatter has no title'
func TestParseTitleFallbackHeading(t *testing.T) {
	doc := parseDocument("a.md", "# Heading One\n\nbody")
	if doc.title != "Heading One" {
		t.Fatalf("title=%q", doc.title)
	}
}

// 'falls back to the file name when there is neither frontmatter nor heading'
func TestParseTitleFallbackFilename(t *testing.T) {
	doc := parseDocument("skills/react/SKILL.md", "just body text")
	if doc.title != "SKILL.md" {
		t.Fatalf("title=%q", doc.title)
	}
}

// 'collects all heading levels for weighting'
func TestParseHeadings(t *testing.T) {
	doc := parseDocument("a.md", "# One\n\n## Two\n\n### Three\n\nbody")
	if !reflect.DeepEqual(doc.headings, []string{"One", "Two", "Three"}) {
		t.Fatalf("headings=%v", doc.headings)
	}
}

// 'does not treat a hash inside a code fence as a heading'
func TestParseNoFenceHeading(t *testing.T) {
	doc := parseDocument("a.md", "```sh\n# not a heading\n```\n\n# Real Heading")
	if !reflect.DeepEqual(doc.headings, []string{"Real Heading"}) {
		t.Fatalf("headings=%v", doc.headings)
	}
}

// 'still indexes code block contents as body text'
func TestParseBodyHasCode(t *testing.T) {
	doc := parseDocument("a.md", "```ts\nconst answer = 42\n```")
	if !strings.Contains(doc.body, "answer") {
		t.Fatalf("body=%q", doc.body)
	}
}

// 'excludes frontmatter from the body so yaml keys do not pollute results'
func TestParseBodyExcludesFrontmatter(t *testing.T) {
	doc := parseDocument("a.md", "---\ntitle: T\ncustomkey: v\n---\nreal body")
	if !strings.Contains(doc.body, "real body") {
		t.Fatalf("body=%q", doc.body)
	}
	if strings.Contains(doc.body, "customkey") {
		t.Fatalf("frontmatter leaked into body: %q", doc.body)
	}
}

// 'tolerates an unterminated frontmatter fence'
func TestParseUnterminatedFence(t *testing.T) {
	doc := parseDocument("a.md", "---\ntitle: T\nno closing fence")
	if len(doc.title) == 0 {
		t.Fatal("empty title")
	}
	_ = parseDocument("a.md", "---") // 不 panic
}

// 'reports tokens for the path itself so file names are searchable'
func TestParsePathTokens(t *testing.T) {
	doc := parseDocument("skills/react/SKILL.md", "body")
	for _, want := range []string{"skills", "react", "skill.md"} {
		if !contains(doc.pathTokens, want) {
			t.Fatalf("missing %q in %v", want, doc.pathTokens)
		}
	}
}

// ---- 旧版 describe('highlightRanges') ----

// 'finds single and multiple token matches case-insensitively'
func TestHighlightBasic(t *testing.T) {
	got := highlightRanges("Hello World", []string{"hello"})
	want := []highlightRange{{0, 5}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	got = highlightRanges("Hello hello world", []string{"hello"})
	want = []highlightRange{{0, 5}, {6, 11}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

// 'merges overlapping ranges from different tokens'
func TestHighlightMergeBigrams(t *testing.T) {
	tokens := tokenize("知识库")
	ranges := highlightRanges("这是一段知识库文本", tokens)
	if len(ranges) != 1 {
		t.Fatalf("expected 1 merged range, got %v", ranges)
	}
	if ranges[0].start != 4 || ranges[0].end != 7 {
		t.Fatalf("range=%v (rune 下标)", ranges[0])
	}
}

// 'returns ranges in order without gaps or inversions'
func TestHighlightOrder(t *testing.T) {
	text := "abc def abc"
	ranges := highlightRanges(text, []string{"abc", "def"})
	for i := 1; i < len(ranges); i++ {
		if ranges[i].start < ranges[i-1].end {
			t.Fatalf("ranges not ordered: %v", ranges)
		}
	}
	runes := []rune(text)
	var sliced []string
	for _, r := range ranges {
		sliced = append(sliced, string(runes[r.start:r.end]))
	}
	if !reflect.DeepEqual(sliced, []string{"abc", "def", "abc"}) {
		t.Fatalf("sliced=%v", sliced)
	}
}

// 'returns empty for no match or empty input'
func TestHighlightEmpty(t *testing.T) {
	if got := highlightRanges("nothing here", []string{"zzz"}); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
	if got := highlightRanges("", []string{"a"}); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
	if got := highlightRanges("text", nil); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

// 'never treats the query as a regex'
func TestHighlightLiteral(t *testing.T) {
	got := highlightRanges("the (a+)+$ pattern", []string{"(a+)+$"})
	want := []highlightRange{{4, 10}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

// 'uses the same tokenizer as the index so bigram tokens align'
func TestHighlightBigramAlign(t *testing.T) {
	text := "受控组件与状态提升"
	tokens := tokenize("状态")
	ranges := highlightRanges(text, tokens)
	if len(ranges) == 0 {
		t.Fatal("no ranges")
	}
	runes := []rune(text)
	for _, r := range ranges {
		if string(runes[r.start:r.end]) != "状态" {
			t.Fatalf("range %v -> %q", r, string(runes[r.start:r.end]))
		}
	}
}

// ---- 旧版 tokenizePath 行为（路径三形态）----

func TestTokenizePathThreeForms(t *testing.T) {
	got := tokenizePath("skills/react/huge.md")
	for _, want := range []string{"skills", "react", "huge.md", "huge", "md"} {
		if !contains(got, want) {
			t.Fatalf("missing %q in %v", want, got)
		}
	}
}
