package index

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/vault"
)

// 「进检索」开关（D-013）：只影响全文检索，链接解析与健康检查不受影响。

func writeVaultFile(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func categoryVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeVaultFile(t, root, "projects/p.md", "# 项目\n\n项目专用词在这里。\n\n[[r]]\n")
	writeVaultFile(t, root, "raws/r.md", "# 原始\n\n项目专用词也在这里。\n")
	writeVaultFile(t, root, ".claude/system.md", "# 系统\n\n项目专用词在系统区。\n")
	writeVaultFile(t, root, "note.md", "# 根目录\n\n项目专用词在根目录。\n")
	return root
}

func searchPaths(t *testing.T, store *Store, text string) []string {
	t.Helper()
	result, err := Search(context.Background(), store, contract.SearchQuery{VaultID: "vault", Text: text, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 0, len(result.Hits))
	for _, hit := range result.Hits {
		paths = append(paths, hit.Path)
	}
	sort.Strings(paths)
	return paths
}

func TestSetSearchExclusionsHidesOnlySearch(t *testing.T) {
	ctx := context.Background()
	root := categoryVault(t)
	store := openTestStore(t)
	if err := Build(ctx, store, root, nil); err != nil {
		t.Fatal(err)
	}
	if got := searchPaths(t, store, "项目专用词"); len(got) != 3 {
		t.Fatalf("before = %v, want projects/raws/note", got)
	}
	unresolvedBefore, orphansBefore, dupBefore, untitledBefore, _, err := Health(ctx, store)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.SetSearchExclusions(ctx, []string{"projects"}); err != nil {
		t.Fatal(err)
	}
	if got := searchPaths(t, store, "项目专用词"); len(got) != 2 || got[0] != "note.md" || got[1] != "raws/r.md" {
		t.Fatalf("after = %v, want note.md + raws/r.md", got)
	}

	// 链接解析与健康检查不受影响：projects/p.md 的 [[r]] 仍解析到 raws/r.md。
	graph, ok, err := Links(ctx, store, "projects/p.md")
	if err != nil || !ok {
		t.Fatalf("links: ok=%v err=%v", ok, err)
	}
	resolved := false
	for _, link := range graph.Outgoing {
		if link.Target == "r" && link.Path != nil && *link.Path == "raws/r.md" {
			resolved = true
		}
	}
	if !resolved {
		t.Fatalf("outgoing = %+v, want r -> raws/r.md", graph.Outgoing)
	}
	unresolved, orphans, duplicates, untitled, _, err := Health(ctx, store)
	if err != nil {
		t.Fatal(err)
	}
	if len(unresolved) != 0 {
		t.Errorf("health unresolved = %+v, want none", unresolved)
	}
	// 四类结果与开关前逐项相等：健康不看 search_excluded。
	if len(unresolved) != len(unresolvedBefore) || len(orphans) != len(orphansBefore) ||
		len(duplicates) != len(dupBefore) || len(untitled) != len(untitledBefore) {
		t.Errorf("health changed: unresolved %d→%d, orphans %d→%d, duplicates %d→%d, untitled %d→%d",
			len(unresolvedBefore), len(unresolved), len(orphansBefore), len(orphans),
			len(dupBefore), len(duplicates), len(untitledBefore), len(untitled))
	}

	// 清空排除后恢复命中。
	if err := store.SetSearchExclusions(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if got := searchPaths(t, store, "项目专用词"); len(got) != 3 {
		t.Fatalf("restored = %v", got)
	}
}

func TestBuildReadsCategoryExclusions(t *testing.T) {
	ctx := context.Background()
	root := categoryVault(t)
	// 大小写与磁盘目录不同，按 Windows 的习惯应当同样生效。
	if err := vault.WriteCategories(root, []vault.Category{
		{Directory: "Projects", Indexed: false},
		{Directory: ".claude", Indexed: true}, // 系统区打开也不进检索
	}); err != nil {
		t.Fatal(err)
	}
	store := openTestStore(t)
	if err := Build(ctx, store, root, nil); err != nil {
		t.Fatal(err)
	}
	if got := searchPaths(t, store, "项目专用词"); len(got) != 2 || got[0] != "note.md" || got[1] != "raws/r.md" {
		t.Fatalf("hits = %v, want note.md + raws/r.md", got)
	}
}

func TestUpdateKeepsCategoryExclusions(t *testing.T) {
	ctx := context.Background()
	root := categoryVault(t)
	if err := vault.WriteCategories(root, []vault.Category{{Directory: "projects", Indexed: false}}); err != nil {
		t.Fatal(err)
	}
	store := openTestStore(t)
	if err := Build(ctx, store, root, nil); err != nil {
		t.Fatal(err)
	}
	// 增量更新重新写这一行时，排除标记仍然生效。
	writeVaultFile(t, root, "projects/p.md", "# 项目\n\n项目专用词改过了。\n")
	if err := Update(ctx, store, root, []string{"projects"}, nil); err != nil {
		t.Fatal(err)
	}
	if got := searchPaths(t, store, "项目专用词"); len(got) != 2 {
		t.Fatalf("after update = %v, want projects still hidden", got)
	}
}

func TestSchemaVersionOneIsRebuilt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.db")
	store, err := OpenIndex(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.setMeta("schema_version", "1"); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	rebuilt, err := OpenIndex(path)
	if err != nil {
		t.Fatal(err)
	}
	defer rebuilt.Close()
	if count, _ := rebuilt.DocumentCount(context.Background()); count != 0 {
		t.Fatalf("v1 database was not rebuilt: %d rows", count)
	}
}
