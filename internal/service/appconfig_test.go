package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/vault"
)

func TestAliasPatchWritesLegacyBytesAndReachesTree(t *testing.T) {
	app, vaultID, root := mutationTestApp(t)
	if err := os.MkdirAll(filepath.Join(root, "raws", "子目录"), 0o755); err != nil {
		t.Fatal(err)
	}
	svc := NewSettingsService(app)
	ctx := context.Background()

	a := "暂存区"
	b := "子区"
	patched := svc.AppConfigPatchAliases(ctx, vaultID, map[string]*string{
		"raws":     &a,
		"raws/子目录": &b,
	})
	if patched.Failure != nil {
		t.Fatalf("patch: %+v", patched.Failure)
	}
	if patched.Aliases["raws"] != "暂存区" || patched.Aliases["raws/子目录"] != "子区" {
		t.Errorf("aliases = %+v", patched.Aliases)
	}

	body, err := os.ReadFile(vault.AliasPath(root))
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"raws\": \"暂存区\",\n  \"raws/子目录\": \"子区\"\n}\n"
	if string(body) != want {
		t.Fatalf("bytes = %q want %q", body, want)
	}

	rootTree, f := vault.ListDir(root, "", patched.Aliases)
	if f != nil {
		t.Fatal(f)
	}
	found := false
	for _, entry := range rootTree.Entries {
		if entry.Path == "raws" {
			found = entry.Alias == "暂存区"
		}
	}
	if !found {
		t.Errorf("root tree missing alias: %+v", rootTree.Entries)
	}
	subTree, f := vault.ListDir(root, "raws", patched.Aliases)
	if f != nil {
		t.Fatal(f)
	}
	subFound := false
	for _, entry := range subTree.Entries {
		if entry.Path == "raws/子目录" {
			subFound = entry.Alias == "子区"
		}
	}
	if !subFound {
		t.Errorf("nested alias missing: %+v", subTree.Entries)
	}

	// A null value removes the entry and the file keeps its order.
	removed := svc.AppConfigPatchAliases(ctx, vaultID, map[string]*string{"raws": nil})
	if removed.Failure != nil {
		t.Fatalf("remove: %+v", removed.Failure)
	}
	body, _ = os.ReadFile(vault.AliasPath(root))
	if string(body) != "{\n  \"raws/子目录\": \"子区\"\n}\n" {
		t.Fatalf("after delete = %q", body)
	}

	// Categories come back parsed (M62 read side, T01).
	catDir := filepath.Join(root, ".mile-kb", "settings")
	if err := os.MkdirAll(catDir, 0o755); err != nil {
		t.Fatal(err)
	}
	categories := `[{"directory":"raws","alias":"暂存","area":"raws","indexed":true,"order":0}]`
	if err := os.WriteFile(filepath.Join(catDir, "categories.json"), []byte(categories), 0o644); err != nil {
		t.Fatal(err)
	}
	got := svc.AppConfigGet(ctx, vaultID)
	if got.Failure != nil {
		t.Errorf("get failure: %+v", got.Failure)
	}
	if len(got.Categories) != 1 {
		t.Fatalf("categories = %+v", got.Categories)
	}
	entry := got.Categories[0]
	if entry.Directory != "raws" || entry.Alias != "暂存" || entry.Area == nil || *entry.Area != "raws" || !entry.Indexed {
		t.Errorf("category = %+v", entry)
	}
}

func TestAppConfigSetCategoriesWritesAndDropsInvalid(t *testing.T) {
	app, vaultID, root := mutationTestApp(t)
	svc := NewSettingsService(app)
	ctx := context.Background()
	area := "raws"
	bad := "bogus"
	got := svc.AppConfigSetCategories(ctx, vaultID, []contract.CategoryConfig{
		{Directory: "wiki", Alias: "", Indexed: true},              // 与默认无异
		{Directory: "projects", Alias: " 项目 ", Indexed: true},      // 有效
		{Directory: "a/b", Alias: "越界", Indexed: true},             // 非法目录名
		{Directory: "raws", Alias: "", Area: &area, Indexed: true}, // 有效 area
		{Directory: "tools", Alias: "", Area: &bad, Indexed: true}, // 非法 area → null，与默认无异被丢弃
	})
	if got.Failure != nil {
		t.Fatalf("set: %+v", got.Failure)
	}
	if len(got.Categories) != 2 {
		t.Fatalf("categories = %+v", got.Categories)
	}
	if got.Categories[0].Directory != "projects" || got.Categories[0].Alias != "项目" {
		t.Errorf("projects = %+v", got.Categories[0])
	}
	if got.Categories[1].Directory != "raws" || got.Categories[1].Area == nil || *got.Categories[1].Area != "raws" {
		t.Errorf("raws = %+v", got.Categories[1])
	}

	body, err := os.ReadFile(vault.CategoriesPath(root))
	if err != nil {
		t.Fatal(err)
	}
	want := `[
  {
    "directory": "projects",
    "alias": "项目",
    "area": null,
    "indexed": true,
    "order": 0
  },
  {
    "directory": "raws",
    "alias": "",
    "area": "raws",
    "indexed": true,
    "order": 0
  }
]
`
	if string(body) != want {
		t.Fatalf("bytes = %q, want %q", body, want)
	}

	// A second read returns the same table.
	again := svc.AppConfigGet(ctx, vaultID)
	if len(again.Categories) != 2 {
		t.Fatalf("reload = %+v", again.Categories)
	}

	missing := svc.AppConfigSetCategories(ctx, "no-such-vault", nil)
	if missing.Failure == nil || missing.Failure.Code != contract.CodeVaultUnavailable && missing.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("missing vault = %+v", missing.Failure)
	}
}

func TestAliasPatchRejectsBadKeys(t *testing.T) {
	app, vaultID, _ := mutationTestApp(t)
	svc := NewSettingsService(app)
	value := "x"
	got := svc.AppConfigPatchAliases(context.Background(), vaultID, map[string]*string{"../escape": &value})
	if got.Failure == nil || got.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("failure = %+v", got.Failure)
	}
}

func TestCategoriesToggleReachesTheIndex(t *testing.T) {
	root := t.TempDir()
	for _, item := range []struct{ rel, body string }{
		{"projects/p.md", "# 项目\n\n项目专用词。\n"},
		{"raws/r.md", "# 原始\n\n项目专用词。\n"},
	} {
		full := filepath.Join(root, filepath.FromSlash(item.rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(item.body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	app, err := New(t.TempDir(), filepath.Join(t.TempDir(), "legacy"))
	if err != nil {
		t.Fatal(err)
	}
	v, f := app.Vaults.Register(root)
	if f != nil {
		t.Fatalf("register: %+v", f)
	}
	app.startIndex(v.ID)
	t.Cleanup(func() { app.indexStop(v.ID, false) })
	waitPhase(t, app, v.ID, "ready")
	indexSvc := NewIndexService(app)
	settings := NewSettingsService(app)
	ctx := context.Background()

	search := func() int {
		result := indexSvc.Search(ctx, contract.SearchQuery{VaultID: v.ID, Text: "项目专用词", Limit: 10})
		if result.Failure != nil {
			t.Fatalf("search: %+v", result.Failure)
		}
		return len(result.Hits)
	}
	if hits := search(); hits != 2 {
		t.Fatalf("before = %d, want 2", hits)
	}

	off := settings.AppConfigSetCategories(ctx, v.ID, []contract.CategoryConfig{
		{Directory: "projects", Indexed: false},
	})
	if off.Failure != nil {
		t.Fatalf("set: %+v", off.Failure)
	}
	if hits := search(); hits != 1 {
		t.Fatalf("after off = %d, want 1", hits)
	}

	// 关闭/重开索引后仍然生效（构建时读 categories.json）。
	app.indexStop(v.ID, false)
	app.startIndex(v.ID)
	waitPhase(t, app, v.ID, "ready")
	if hits := search(); hits != 1 {
		t.Fatalf("after restart = %d, want 1", hits)
	}

	on := settings.AppConfigSetCategories(ctx, v.ID, []contract.CategoryConfig{
		{Directory: "projects", Indexed: true},
	})
	if on.Failure != nil {
		t.Fatalf("set on: %+v", on.Failure)
	}
	if hits := search(); hits != 2 {
		t.Fatalf("after on = %d, want 2", hits)
	}
}
