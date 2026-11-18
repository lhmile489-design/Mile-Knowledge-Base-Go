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

	// Categories pass through untouched (M62 read side).
	catDir := filepath.Join(root, ".mile-kb", "settings")
	if err := os.MkdirAll(catDir, 0o755); err != nil {
		t.Fatal(err)
	}
	categories := `[{"directory":"raws","alias":"","area":"raws","indexed":true,"order":0}]`
	if err := os.WriteFile(filepath.Join(catDir, "categories.json"), []byte(categories), 0o644); err != nil {
		t.Fatal(err)
	}
	got := svc.AppConfigGet(ctx, vaultID)
	if string(got.Categories) != categories {
		t.Errorf("categories = %s", got.Categories)
	}
	if got.Failure != nil {
		t.Errorf("get failure: %+v", got.Failure)
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
