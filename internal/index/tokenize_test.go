package index

import (
	"reflect"
	"strings"
	"testing"
)

// Ported from tests/unit/tokenize.test.ts (legacy v2.7.0, 21 cases). Each Go
// test corresponds to one legacy `it`, with the same inputs and assertions.

func TestTokenizeEnglishWordBoundariesLowerCase(t *testing.T) {
	got := Tokenize("Hello World")
	if !contains(got, "hello") || !contains(got, "world") {
		t.Fatalf("missing lowercase tokens: %v", got)
	}
	if contains(got, "Hello") {
		t.Fatalf("kept case: %v", got)
	}
}

func TestTokenizeKeepsVersionLikeTokens(t *testing.T) {
	got := Tokenize("deepseek v4.1 release")
	for _, want := range []string{"deepseek", "v4.1", "release"} {
		if !contains(got, want) {
			t.Errorf("missing %q in %v", want, got)
		}
	}
}

func TestTokenizeDoesNotSplitCamelCase(t *testing.T) {
	got := Tokenize("getUserName")
	if !reflect.DeepEqual(got, []string{"getusername"}) {
		t.Fatalf("got %v", got)
	}
}

func TestTokenizeCJKBigrams(t *testing.T) {
	got := Tokenize("知识库管理")
	for _, want := range []string{"知识", "识库", "库管", "管理"} {
		if !contains(got, want) {
			t.Errorf("missing %q in %v", want, got)
		}
	}
}

func TestTokenizeLoneCJKCharacter(t *testing.T) {
	if got := Tokenize("中"); !reflect.DeepEqual(got, []string{"中"}) {
		t.Fatalf("got %v", got)
	}
}

func TestTokenizeMixedChineseEnglish(t *testing.T) {
	got := Tokenize("React 状态管理 best practice")
	for _, want := range []string{"react", "状态", "态管", "best", "practice"} {
		if !contains(got, want) {
			t.Errorf("missing %q in %v", want, got)
		}
	}
}

func TestTokenizePunctuationAndWhitespaceSeparators(t *testing.T) {
	got := Tokenize("alpha, beta; gamma\n\tdelta")
	want := []string{"alpha", "beta", "gamma", "delta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestTokenizeNoBigramAcrossPunctuation(t *testing.T) {
	if contains(Tokenize("知识库。管理"), "库管") {
		t.Fatal("bridged a bigram across punctuation")
	}
}

func TestTokenizeEmptyAndSymbols(t *testing.T) {
	if got := Tokenize(""); len(got) != 0 {
		t.Fatalf("empty: %v", got)
	}
	if got := Tokenize("!!! ---"); len(got) != 0 {
		t.Fatalf("symbols: %v", got)
	}
}

func TestTokenizeDeduplicates(t *testing.T) {
	n := 0
	for _, tok := range Tokenize("test test test") {
		if tok == "test" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("count %d", n)
	}
}

func TestTokenizeTruncatesLongInput(t *testing.T) {
	got := Tokenize(strings.Repeat("a", 2_000_000))
	if len(got) == 0 || len(got) >= 500_000 {
		t.Fatalf("len %d", len(got))
	}
}

func TestParseDocumentFrontmatterTitleAndTags(t *testing.T) {
	doc := ParseDocument("skills/react/SKILL.md", strings.Join([]string{
		"---", "title: React 状态管理", "tags: [react, state]", "---", "", "body text",
	}, "\n"))
	if doc.Title != "React 状态管理" {
		t.Errorf("title %q", doc.Title)
	}
	if !reflect.DeepEqual(doc.Tags, []string{"react", "state"}) {
		t.Errorf("tags %v", doc.Tags)
	}
}

func TestParseDocumentYAMLListTags(t *testing.T) {
	doc := ParseDocument("a.md", strings.Join([]string{
		"---", "tags:", "  - alpha", "  - beta", "---", "body",
	}, "\n"))
	if !reflect.DeepEqual(doc.Tags, []string{"alpha", "beta"}) {
		t.Errorf("tags %v", doc.Tags)
	}
}

func TestParseDocumentFallsBackToFirstHeading(t *testing.T) {
	if doc := ParseDocument("a.md", "# Heading One\n\nbody"); doc.Title != "Heading One" {
		t.Errorf("title %q", doc.Title)
	}
}

func TestParseDocumentFallsBackToFileName(t *testing.T) {
	if doc := ParseDocument("skills/react/SKILL.md", "just body text"); doc.Title != "SKILL.md" {
		t.Errorf("title %q", doc.Title)
	}
}

func TestParseDocumentCollectsAllHeadings(t *testing.T) {
	doc := ParseDocument("a.md", "# One\n\n## Two\n\n### Three\n\nbody")
	if !reflect.DeepEqual(doc.Headings, []string{"One", "Two", "Three"}) {
		t.Errorf("headings %v", doc.Headings)
	}
}

func TestParseDocumentIgnoresHashInFence(t *testing.T) {
	doc := ParseDocument("a.md", "```sh\n# not a heading\n```\n\n# Real Heading")
	if !reflect.DeepEqual(doc.Headings, []string{"Real Heading"}) {
		t.Errorf("headings %v", doc.Headings)
	}
}

func TestParseDocumentKeepsCodeBlockInBody(t *testing.T) {
	if doc := ParseDocument("a.md", "```ts\nconst answer = 42\n```"); !strings.Contains(doc.Body, "answer") {
		t.Errorf("body %q", doc.Body)
	}
}

func TestParseDocumentExcludesFrontmatterFromBody(t *testing.T) {
	doc := ParseDocument("a.md", "---\ntitle: T\ncustomkey: v\n---\nreal body")
	if !strings.Contains(doc.Body, "real body") {
		t.Errorf("body %q", doc.Body)
	}
	if strings.Contains(doc.Body, "customkey") {
		t.Errorf("frontmatter leaked into body: %q", doc.Body)
	}
}

func TestParseDocumentToleratesUnterminatedFrontmatter(t *testing.T) {
	doc := ParseDocument("a.md", "---\ntitle: T\nno closing fence")
	if doc.Title == "" {
		t.Error("empty title")
	}
	_ = ParseDocument("a.md", "---")
}

func TestParseDocumentPathTokens(t *testing.T) {
	doc := ParseDocument("skills/react/SKILL.md", "body")
	for _, want := range []string{"skills", "react", "skill.md"} {
		if !contains(doc.PathTokens, want) {
			t.Errorf("missing %q in %v", want, doc.PathTokens)
		}
	}
}

func TestTokenizePathThreeForms(t *testing.T) {
	// Legacy output confirmed by running src/shared/tokenize.ts on
	// "v4.1-notes.md": one token for the whole segment, then the .-_ split.
	got := TokenizePath("huge/v4.1-notes.md")
	for _, want := range []string{"huge", "v4.1-notes.md", "v4", "1", "notes", "md"} {
		if !contains(got, want) {
			t.Errorf("missing %q in %v", want, got)
		}
	}
}

func TestTokenizeDropsTrailingInnerSymbols(t *testing.T) {
	// Legacy tokenize("C#") == ["c"], tokenize("c++") == ["c"]: an inner symbol
	// only stays when a word character follows it.
	if got := Tokenize("C#"); !reflect.DeepEqual(got, []string{"c"}) {
		t.Errorf("C#: %v", got)
	}
	if got := Tokenize("c++"); !reflect.DeepEqual(got, []string{"c"}) {
		t.Errorf("c++: %v", got)
	}
}

func TestBuildExcerptFindsEarliestToken(t *testing.T) {
	body := strings.Repeat("x", 50) + "知识库" + strings.Repeat("y", 50)
	got := BuildExcerpt(body, []string{"知识", "库"}, 10)
	if !strings.Contains(got, "知识库") {
		t.Fatalf("excerpt %q", got)
	}
	if !strings.HasPrefix(got, "…") || !strings.HasSuffix(got, "…") {
		t.Errorf("missing ellipses: %q", got)
	}
}

func TestBuildExcerptCollapsesWhitespace(t *testing.T) {
	got := BuildExcerpt("alpha\n\nbeta   gamma", []string{"beta"}, 60)
	if got != "alpha beta gamma" {
		t.Fatalf("got %q", got)
	}
}

func TestBuildExcerptNeverFabricates(t *testing.T) {
	if got := BuildExcerpt("alpha beta", []string{"zzz"}, 60); got != "" {
		t.Fatalf("got %q", got)
	}
	if got := BuildExcerpt("", []string{"a"}, 60); got != "" {
		t.Fatalf("empty body: %q", got)
	}
}

func TestBuildExcerptKeepsShortBodyWhole(t *testing.T) {
	if got := BuildExcerpt("short body", []string{"body"}, 60); got != "short body" {
		t.Fatalf("got %q", got)
	}
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}
