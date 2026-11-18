package index

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func healthVault(t *testing.T) string {
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
	write("raws/断链.md", "# 断链\n\n[[不存在]] 与 [[同名]]\n")
	write("a/同名.md", "# 同名\n")
	write("b/同名.md", "# 同名\n")
	write("raws/孤立.md", "# 孤立\n")
	write("README.md", "# 根说明\n")
	write("skills/react/SKILL.md", "# 技能\n")
	write("raws/缺标题.md", "没有标题的正文\n")
	write(".claude/系统.md", "# 系统\n\n[[不存在]]\n")
	return root
}

func TestHealthFourLists(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	if err := Build(ctx, store, healthVault(t), nil); err != nil {
		t.Fatal(err)
	}
	unresolved, orphans, duplicates, untitled, revision, err := Health(ctx, store)
	if err != nil {
		t.Fatal(err)
	}
	if revision == 0 {
		t.Error("revision not reported")
	}

	if len(unresolved) != 2 {
		t.Fatalf("unresolved = %+v", unresolved)
	}
	byTarget := map[string]int{}
	for _, item := range unresolved {
		byTarget[item.Target] = item.Candidates
	}
	if byTarget["不存在"] != 0 || byTarget["同名"] != 2 {
		t.Errorf("candidates = %v", byTarget)
	}

	if len(duplicates) != 1 || duplicates[0].Name != "同名.md" || len(duplicates[0].Paths) != 2 {
		t.Errorf("duplicates = %+v", duplicates)
	}

	if len(untitled) != 1 || untitled[0].Path != "raws/缺标题.md" {
		t.Errorf("untitled = %+v", untitled)
	}

	// Legacy rule: outgoing empty and no resolved incoming edge. The two
	// same-named files are only referenced by an ambiguous link, which is not
	// a resolved edge, so they count as orphans too (same as the old app).
	paths := map[string]bool{}
	for _, item := range orphans {
		paths[item.Path] = true
	}
	for _, want := range []string{"a/同名.md", "b/同名.md", "raws/孤立.md", "raws/缺标题.md"} {
		if !paths[want] {
			t.Errorf("missing orphan %s in %+v", want, orphans)
		}
	}
	if paths["README.md"] || paths["skills/react/SKILL.md"] || paths[".claude/系统.md"] {
		t.Errorf("entry points or system docs counted as orphans: %+v", orphans)
	}
}

func TestHealthCapsLists(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	// 205 isolated documents: every list stays at the 200 cap.
	for i := 0; i < 205; i++ {
		rel := filepath.Join(root, "raws", "孤儿-"+fmt.Sprintf("%03d", i)+".md")
		if err := os.MkdirAll(filepath.Dir(rel), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(rel, []byte("body\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	store := openTestStore(t)
	if err := Build(ctx, store, root, nil); err != nil {
		t.Fatal(err)
	}
	_, orphans, _, untitled, _, err := Health(ctx, store)
	if err != nil {
		t.Fatal(err)
	}
	if len(orphans) != HealthMaxItems || len(untitled) != HealthMaxItems {
		t.Fatalf("caps not applied: orphans=%d untitled=%d", len(orphans), len(untitled))
	}
}
