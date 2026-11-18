package index

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

func buildSearchFixture(t *testing.T) *Store {
	t.Helper()
	root := newTestVault(t)
	write := func(rel, body string) {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Title match vs body-only match for the same query.
	write("raws/title-match.md", "# 状态管理\n\n无关正文。\n")
	write("raws/body-match.md", "# 普通标题\n\n这里是状态管理的内容。\n")
	// Two-token query where only one document has both tokens.
	write("raws/one-token.md", "# 只有知识\n\n知识。\n")
	write("raws/both-tokens.md", "# 知识与 管理\n\n知识 管理。\n")
	// A path-only match: the query token lives in the file name only.
	write("raws/独特路径词.md", "# 别的标题\n\n没有那个词。\n")

	store := openTestStore(t)
	if err := Build(context.Background(), store, root, nil); err != nil {
		t.Fatal(err)
	}
	return store
}

func runSearch(t *testing.T, store *Store, query contract.SearchQuery) contract.SearchResult {
	t.Helper()
	result, err := Search(context.Background(), store, query)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestSearchChineseBigrams(t *testing.T) {
	store := buildSearchFixture(t)
	result := runSearch(t, store, contract.SearchQuery{Text: "知识库"})
	if result.Total == 0 {
		t.Fatal("no hits")
	}
	var hit *contract.SearchHit
	for i := range result.Hits {
		if result.Hits[i].Path == "raws/首页.md" {
			hit = &result.Hits[i]
		}
	}
	if hit == nil {
		t.Fatalf("首页.md not found in %v", result.Hits)
	}
	if !contains(hit.MatchedFields, "body") {
		t.Errorf("matchedFields = %v", hit.MatchedFields)
	}
	if !strings.Contains(hit.Excerpt, "知识库") {
		t.Errorf("excerpt = %q", hit.Excerpt)
	}
}

func TestSearchTitleOutranksBodyOnly(t *testing.T) {
	store := buildSearchFixture(t)
	result := runSearch(t, store, contract.SearchQuery{Text: "状态管理"})
	titleAt, bodyAt := -1, -1
	for i, hit := range result.Hits {
		switch hit.Path {
		case "raws/title-match.md":
			titleAt = i
		case "raws/body-match.md":
			bodyAt = i
		}
	}
	if titleAt == -1 || bodyAt == -1 {
		t.Fatalf("missing hits: %+v", result.Hits)
	}
	if titleAt > bodyAt {
		t.Errorf("title match ranked below body-only: %d vs %d", titleAt, bodyAt)
	}
}

func TestSearchFullTokenMatchRanksFirst(t *testing.T) {
	store := buildSearchFixture(t)
	result := runSearch(t, store, contract.SearchQuery{Text: "知识 管理"})
	if len(result.Hits) < 2 {
		t.Fatalf("hits = %+v", result.Hits)
	}
	if result.Hits[0].Path != "raws/both-tokens.md" {
		t.Errorf("first hit = %s, want both-tokens.md", result.Hits[0].Path)
	}
}

func TestSearchSystemAreaFilter(t *testing.T) {
	store := buildSearchFixture(t)
	if got := runSearch(t, store, contract.SearchQuery{Text: "系统区笔记"}); got.Total != 0 {
		t.Errorf("system doc leaked into default results: %+v", got.Hits)
	}
	got := runSearch(t, store, contract.SearchQuery{
		Text:    "系统区笔记",
		Filters: &contract.SearchFilters{IncludeSystem: true},
	})
	if got.Total != 1 || !got.Hits[0].Ignored {
		t.Fatalf("includeSystem result = %+v", got.Hits)
	}
}

func TestSearchExtensionAndAreaFilters(t *testing.T) {
	store := buildSearchFixture(t)
	if got := runSearch(t, store, contract.SearchQuery{
		Text: "answer", Filters: &contract.SearchFilters{Extensions: []string{".MD"}},
	}); got.Total != 0 {
		t.Errorf("extension filter ignored: %+v", got.Hits)
	}
	if got := runSearch(t, store, contract.SearchQuery{
		Text: "answer", Filters: &contract.SearchFilters{Extensions: []string{"txt"}},
	}); got.Total != 1 {
		t.Errorf("txt filter result = %+v", got.Hits)
	}
	if got := runSearch(t, store, contract.SearchQuery{
		Text: "知识库", Filters: &contract.SearchFilters{Areas: []string{"skills"}},
	}); got.Total != 0 {
		t.Errorf("area filter ignored: %+v", got.Hits)
	}
}

func TestSearchTagFilter(t *testing.T) {
	store := buildSearchFixture(t)
	if got := runSearch(t, store, contract.SearchQuery{
		Text: "知识库", Filters: &contract.SearchFilters{Tags: []string{"DEMO"}},
	}); got.Total != 1 || got.Hits[0].Path != "raws/首页.md" {
		t.Fatalf("tag filter = %+v", got.Hits)
	}
	if got := runSearch(t, store, contract.SearchQuery{
		Text: "知识库", Filters: &contract.SearchFilters{Tags: []string{"nope"}},
	}); got.Total != 0 {
		t.Errorf("unknown tag returned hits: %+v", got.Hits)
	}
}

func TestSearchUpdatedAfter(t *testing.T) {
	store := buildSearchFixture(t)
	future := runSearch(t, store, contract.SearchQuery{
		Text: "知识库", Filters: &contract.SearchFilters{UpdatedAfter: 4_000_000_000_000},
	})
	if future.Total != 0 {
		t.Errorf("future updatedAfter returned %+v", future.Hits)
	}
}

func TestSearchPathOnlyMatchHasNoExcerpt(t *testing.T) {
	store := buildSearchFixture(t)
	got := runSearch(t, store, contract.SearchQuery{Text: "独特"})
	if got.Total != 1 {
		t.Fatalf("hits = %+v", got.Hits)
	}
	hit := got.Hits[0]
	if !contains(hit.MatchedFields, "path") {
		t.Errorf("matchedFields = %v", hit.MatchedFields)
	}
	if hit.Excerpt != "" {
		t.Errorf("excerpt fabricated for a path-only match: %q", hit.Excerpt)
	}
}

func TestSearchEmptyAndLimits(t *testing.T) {
	store := buildSearchFixture(t)
	if got := runSearch(t, store, contract.SearchQuery{Text: "   "}); got.Total != 0 || len(got.Hits) != 0 {
		t.Errorf("empty query returned %+v", got.Hits)
	}
	got := runSearch(t, store, contract.SearchQuery{Text: "知识库", Limit: 1})
	if len(got.Hits) > 1 {
		t.Errorf("limit ignored: %d hits", len(got.Hits))
	}
	long := strings.Repeat("知", 500)
	if got := runSearch(t, store, contract.SearchQuery{Text: long}); len([]rune(got.Query)) > MaxQueryLength {
		t.Errorf("query not truncated: %d runes", len([]rune(got.Query)))
	}
}
