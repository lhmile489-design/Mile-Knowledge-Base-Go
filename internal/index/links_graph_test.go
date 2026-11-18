package index

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// Ported from tests/unit/linkGraph.test.ts (legacy v2.7.0, 12 cases), rebuilt on
// the real index instead of an in-memory projection.

func linkVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("raws/A.md", "# 甲\n\n[[B]]、[外部](https://example.com)、[[不存在]]、[[dup]]\n")
	write("raws/B.md", "# 乙\n\n[[A]]\n")
	write("raws/自我.md", "# 自我\n\n[[自我]]\n")
	write("raws/dup.md", "# 重复一\n")
	write("other/dup.md", "# 重复二\n")
	write("raws/fence.md", "```\n[[A]]\n```\n")
	write(".claude/system.md", "# 系统\n\n[[A]]\n")
	return root
}

func buildLinkStore(t *testing.T, root string) *Store {
	t.Helper()
	store := openTestStore(t)
	if err := Build(context.Background(), store, root, nil); err != nil {
		t.Fatal(err)
	}
	return store
}

func graph(t *testing.T, store *Store, rel string) LinkGraph {
	t.Helper()
	out, ok, err := Links(context.Background(), store, rel)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("%s not in index", rel)
	}
	return out
}

func outgoing(g LinkGraph, target string) *ResolvedLink {
	for i := range g.Outgoing {
		if g.Outgoing[i].Target == target {
			return &g.Outgoing[i]
		}
	}
	return nil
}

func TestGraphBacklinkDirection(t *testing.T) {
	store := buildLinkStore(t, linkVault(t))
	if !contains(graph(t, store, "raws/B.md").Backlinks, "raws/A.md") {
		t.Fatalf("B backlinks = %v", graph(t, store, "raws/B.md").Backlinks)
	}
}

func TestGraphOutgoingCoversEveryKind(t *testing.T) {
	store := buildLinkStore(t, linkVault(t))
	g := graph(t, store, "raws/A.md")
	if link := outgoing(g, "B"); link == nil || link.Path == nil || *link.Path != "raws/B.md" || link.Candidates != 1 {
		t.Errorf("wiki edge = %+v", link)
	}
	if link := outgoing(g, "https://example.com"); link == nil || link.Kind != LinkExternal || link.Path != nil {
		t.Errorf("external edge = %+v", link)
	}
	if link := outgoing(g, "不存在"); link == nil || link.Path != nil || link.Candidates != 0 {
		t.Errorf("missing edge = %+v", link)
	}
	if link := outgoing(g, "dup"); link == nil || link.Path != nil || link.Candidates != 2 {
		t.Errorf("ambiguous edge = %+v", link)
	}
}

func TestGraphSelfReferenceIsNotABacklink(t *testing.T) {
	store := buildLinkStore(t, linkVault(t))
	g := graph(t, store, "raws/自我.md")
	if contains(g.Backlinks, "raws/自我.md") {
		t.Errorf("self reference counted: %v", g.Backlinks)
	}
	if link := outgoing(g, "自我"); link == nil || link.Path == nil || *link.Path != "raws/自我.md" {
		t.Errorf("self edge = %+v", link)
	}
}

func TestGraphUnresolvedEdgeHasNoBacklink(t *testing.T) {
	store := buildLinkStore(t, linkVault(t))
	g := graph(t, store, "raws/A.md")
	if link := outgoing(g, "不存在"); link == nil || link.Path != nil {
		t.Errorf("missing edge = %+v", link)
	}
}

func TestGraphAmbiguousEdgeHasNoBacklink(t *testing.T) {
	store := buildLinkStore(t, linkVault(t))
	for _, path := range []string{"raws/dup.md", "other/dup.md"} {
		if contains(graph(t, store, path).Backlinks, "raws/A.md") {
			t.Errorf("%s received a backlink from an ambiguous edge", path)
		}
	}
}

func TestGraphExcludesSystemArea(t *testing.T) {
	store := buildLinkStore(t, linkVault(t))
	if contains(graph(t, store, "raws/A.md").Backlinks, ".claude/system.md") {
		t.Errorf("system document entered the graph")
	}
}

func TestGraphIgnoresFencedLinks(t *testing.T) {
	store := buildLinkStore(t, linkVault(t))
	if contains(graph(t, store, "raws/A.md").Backlinks, "raws/fence.md") {
		t.Errorf("fenced link entered the graph: %v", graph(t, store, "raws/A.md").Backlinks)
	}
}

func TestGraphRemovingALinkLeavesNoGhostBacklink(t *testing.T) {
	ctx := context.Background()
	root := linkVault(t)
	store := buildLinkStore(t, root)
	if err := os.WriteFile(filepath.Join(root, "raws", "A.md"), []byte("# 甲\n\n没有链接了。\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Update(ctx, store, root, []string{"raws"}, nil); err != nil {
		t.Fatal(err)
	}
	if got := graph(t, store, "raws/B.md").Backlinks; len(got) != 1 || got[0] != "raws/自我.md" {
		// B is still referenced by nothing else; 自我 links to itself only.
		if contains(got, "raws/A.md") {
			t.Errorf("ghost backlink survived: %v", got)
		}
	}
}

func TestGraphRetargetingRemovesOldBacklink(t *testing.T) {
	ctx := context.Background()
	root := linkVault(t)
	store := buildLinkStore(t, root)
	if err := os.WriteFile(filepath.Join(root, "raws", "A.md"), []byte("# 甲\n\n[[自我]]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Update(ctx, store, root, []string{"raws"}, nil); err != nil {
		t.Fatal(err)
	}
	if contains(graph(t, store, "raws/B.md").Backlinks, "raws/A.md") {
		t.Errorf("old target kept the backlink")
	}
	if !contains(graph(t, store, "raws/自我.md").Backlinks, "raws/A.md") {
		t.Errorf("new target did not receive the backlink: %v", graph(t, store, "raws/自我.md").Backlinks)
	}
}

func TestGraphDeletingSourceCleansBacklinks(t *testing.T) {
	ctx := context.Background()
	root := linkVault(t)
	store := buildLinkStore(t, root)
	if err := os.Remove(filepath.Join(root, "raws", "A.md")); err != nil {
		t.Fatal(err)
	}
	if err := Update(ctx, store, root, []string{"raws"}, nil); err != nil {
		t.Fatal(err)
	}
	if contains(graph(t, store, "raws/B.md").Backlinks, "raws/A.md") {
		t.Errorf("deleted source still links: %v", graph(t, store, "raws/B.md").Backlinks)
	}
}

func TestGraphNewTargetResolvesEarlierLink(t *testing.T) {
	ctx := context.Background()
	root := linkVault(t)
	store := buildLinkStore(t, root)
	if link := outgoing(graph(t, store, "raws/A.md"), "不存在"); link == nil || link.Path != nil {
		t.Fatalf("precondition: edge should be unresolved, got %+v", link)
	}
	if err := os.WriteFile(filepath.Join(root, "raws", "不存在.md"), []byte("# 不存在\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Update(ctx, store, root, []string{"raws"}, nil); err != nil {
		t.Fatal(err)
	}
	link := outgoing(graph(t, store, "raws/A.md"), "不存在")
	if link == nil || link.Path == nil || *link.Path != "raws/不存在.md" {
		t.Fatalf("edge did not resolve: %+v", link)
	}
	if !contains(graph(t, store, "raws/不存在.md").Backlinks, "raws/A.md") {
		t.Errorf("new target has no backlink")
	}
}

func TestGraphUnknownPathReportsMissing(t *testing.T) {
	store := buildLinkStore(t, linkVault(t))
	if _, ok, err := Links(context.Background(), store, "nope.md"); err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}
