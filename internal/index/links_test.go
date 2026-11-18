package index

import (
	"strings"
	"testing"
)

// Ported from tests/unit/links.test.ts (legacy v2.7.0, 27 cases).

func extractOne(t *testing.T, md string) LinkRef {
	t.Helper()
	links := ExtractLinks(md)
	if len(links) != 1 {
		t.Fatalf("expected one link, got %+v", links)
	}
	return links[0]
}

func TestExtractSimpleWikilink(t *testing.T) {
	link := extractOne(t, "前文 [[目标]] 后文")
	if link.Kind != LinkWiki || link.Target != "目标" || link.Text != "目标" {
		t.Fatalf("link = %+v", link)
	}
}

func TestExtractAliasedWikilink(t *testing.T) {
	link := extractOne(t, "[[目标|显示文本]]")
	if link.Target != "目标" || link.Text != "显示文本" {
		t.Fatalf("link = %+v", link)
	}
}

func TestExtractPathWikilink(t *testing.T) {
	link := extractOne(t, "[[raws/笔记]]")
	if link.Target != "raws/笔记" {
		t.Fatalf("link = %+v", link)
	}
}

func TestExtractMultipleWikilinksOnOneLine(t *testing.T) {
	links := ExtractLinks("[[甲]] 与 [[乙]]")
	if len(links) != 2 || links[0].Target != "甲" || links[1].Target != "乙" {
		t.Fatalf("links = %+v", links)
	}
}

func TestExtractWikilinksAcrossLines(t *testing.T) {
	links := ExtractLinks("[[甲]]\n正文\n[[乙]]")
	if len(links) != 2 || links[0].Line != 0 || links[1].Line != 2 {
		t.Fatalf("links = %+v", links)
	}
}

func TestExtractIgnoresBacktickFence(t *testing.T) {
	md := "```\n[[甲]]\n```\n[[乙]]"
	links := ExtractLinks(md)
	if len(links) != 1 || links[0].Target != "乙" {
		t.Fatalf("links = %+v", links)
	}
}

func TestExtractIgnoresTildeFence(t *testing.T) {
	md := "~~~\n[[甲]]\n~~~"
	if links := ExtractLinks(md); len(links) != 0 {
		t.Fatalf("links = %+v", links)
	}
}

func TestExtractIgnoresInlineCode(t *testing.T) {
	if links := ExtractLinks("讲语法：`[[甲]]`"); len(links) != 0 {
		t.Fatalf("links = %+v", links)
	}
}

func TestExtractIgnoresEmptyWikilinks(t *testing.T) {
	if links := ExtractLinks("[[]] 与 [[   ]]"); len(links) != 0 {
		t.Fatalf("links = %+v", links)
	}
}

func TestExtractIgnoresUnclosedWikilink(t *testing.T) {
	if links := ExtractLinks("[[未闭合"); len(links) != 0 {
		t.Fatalf("links = %+v", links)
	}
}

func TestClassifyRelativeAsInternal(t *testing.T) {
	link := extractOne(t, "[文字](raws/笔记.md)")
	if link.Kind != LinkInternal || link.Target != "raws/笔记.md" || link.Text != "文字" {
		t.Fatalf("link = %+v", link)
	}
}

func TestClassifyHTTPAsExternal(t *testing.T) {
	for _, url := range []string{"http://example.com", "https://example.com/a?b=1"} {
		if link := extractOne(t, "[x]("+url+")"); link.Kind != LinkExternal {
			t.Errorf("%s -> %s", url, link.Kind)
		}
	}
}

func TestClassifyOtherSchemesAsOther(t *testing.T) {
	for _, url := range []string{"mailto:a@b.c", "javascript:alert(1)", "file:///c:/x"} {
		if link := extractOne(t, "[x]("+url+")"); link.Kind != LinkOther {
			t.Errorf("%s -> %s", url, link.Kind)
		}
	}
}

func TestExtractIgnoresMarkdownLinksInFence(t *testing.T) {
	md := "```\n[甲](a.md)\n```"
	if links := ExtractLinks(md); len(links) != 0 {
		t.Fatalf("links = %+v", links)
	}
}

func TestExtractImagesAreNotLinks(t *testing.T) {
	if links := ExtractLinks("![图](pic.png)"); len(links) != 0 {
		t.Fatalf("links = %+v", links)
	}
}

func TestExtractIgnoresEmptyTargets(t *testing.T) {
	if links := ExtractLinks("[文本]()"); len(links) != 0 {
		t.Fatalf("links = %+v", links)
	}
}

func TestExtractKeepsBalancedParentheses(t *testing.T) {
	link := extractOne(t, "[维基](https://en.wikipedia.org/wiki/Foo_(bar))")
	if link.Target != "https://en.wikipedia.org/wiki/Foo_(bar)" {
		t.Fatalf("target = %q", link.Target)
	}
}

func TestExtractLinkInsideParentheses(t *testing.T) {
	link := extractOne(t, "（见 [文档](raws/a.md)）")
	if link.Target != "raws/a.md" {
		t.Fatalf("target = %q", link.Target)
	}
}

func targetsFixture() []LinkTarget {
	return []LinkTarget{
		{Path: "raws/a.md", Title: "甲"},
		{Path: "raws/b.md", Title: "乙"},
		{Path: "skills/react/SKILL.md", Title: "React 技能"},
		{Path: "raws/dup.md", Title: "重复一"},
		{Path: "other/dup.md", Title: "重复二"},
	}
}

func TestResolveExactRelativePath(t *testing.T) {
	if path, n := NewResolver(targetsFixture()).Resolve("raws/a.md"); path != "raws/a.md" || n != 1 {
		t.Fatalf("resolved %q %d", path, n)
	}
}

func TestResolveWithoutExtension(t *testing.T) {
	if path, n := NewResolver(targetsFixture()).Resolve("raws/b"); path != "raws/b.md" || n != 1 {
		t.Fatalf("resolved %q %d", path, n)
	}
}

func TestResolveCaseInsensitive(t *testing.T) {
	if path, n := NewResolver(targetsFixture()).Resolve("SKILLS/REACT/skill"); path != "skills/react/SKILL.md" || n != 1 {
		t.Fatalf("resolved %q %d", path, n)
	}
}

func TestResolveUniqueFileName(t *testing.T) {
	if path, n := NewResolver(targetsFixture()).Resolve("SKILL"); path != "skills/react/SKILL.md" || n != 1 {
		t.Fatalf("resolved %q %d", path, n)
	}
}

func TestResolveUniqueTitle(t *testing.T) {
	if path, n := NewResolver(targetsFixture()).Resolve("React 技能"); path != "skills/react/SKILL.md" || n != 1 {
		t.Fatalf("resolved %q %d", path, n)
	}
}

func TestResolveAmbiguousReportsCount(t *testing.T) {
	path, n := NewResolver(targetsFixture()).Resolve("dup")
	if path != "" || n != 2 {
		t.Fatalf("resolved %q %d", path, n)
	}
}

func TestResolveNoneReportsZero(t *testing.T) {
	if path, n := NewResolver(targetsFixture()).Resolve("不存在"); path != "" || n != 0 {
		t.Fatalf("resolved %q %d", path, n)
	}
}

func TestResolveExactPathWinsOverDuplicateName(t *testing.T) {
	// "raws/dup" is an exact path even though "dup" alone is ambiguous.
	if path, n := NewResolver(targetsFixture()).Resolve("raws/dup"); path != "raws/dup.md" || n != 1 {
		t.Fatalf("resolved %q %d", path, n)
	}
}

func TestResolveEmptyTargetIsUnresolved(t *testing.T) {
	for _, target := range []string{"", "   "} {
		if path, n := NewResolver(targetsFixture()).Resolve(target); path != "" || n != 0 {
			t.Fatalf("%q resolved %q %d", target, path, n)
		}
	}
}

func TestExtractColumnPointsAtMatch(t *testing.T) {
	line := "前缀 [[目标]]"
	link := extractOne(t, line)
	if !strings.HasPrefix(line[byteIndexForRune(line, link.Column):], "[[") {
		t.Fatalf("column %d does not point at the link: %q", link.Column, line)
	}
}

func byteIndexForRune(s string, runeAt int) int {
	count := 0
	for i := range s {
		if count == runeAt {
			return i
		}
		count++
	}
	return len(s)
}
