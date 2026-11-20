package skills

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// M4 T07: adopt copies into the knowledge base and optionally swaps the tool
// copy for a link, restoring it when the swap fails (PRD FR-SKM-ADOPT).

func TestAdoptCopiesWithoutLink(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "tool", "foo")
	writeFile(t, filepath.Join(source, "SKILL.md"), "# foo\n")
	target := filepath.Join(root, "vault", "skills", "分类", "foo")

	result, err := Adopt(AdoptOptions{ToolID: "t", SourceDir: source, TargetDir: target})
	if err != nil || result.Linked {
		t.Fatalf("adopt = %+v %v", result, err)
	}
	if body, err := os.ReadFile(filepath.Join(target, "SKILL.md")); err != nil || string(body) != "# foo\n" {
		t.Fatalf("vault copy: %q %v", body, err)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("source should stay: %v", err)
	}
	if _, err := Adopt(AdoptOptions{ToolID: "t", SourceDir: source, TargetDir: target}); !errors.Is(err, ErrAdoptExists) {
		t.Fatalf("second adopt = %v, want ErrAdoptExists", err)
	}
}

func TestAdoptWithLinkBack(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "tool", "foo")
	writeFile(t, filepath.Join(source, "SKILL.md"), "# foo\n")
	target := filepath.Join(root, "vault", "skills", "分类", "foo")
	store := &BackupStore{Dir: filepath.Join(root, "backups")}

	result, err := Adopt(AdoptOptions{
		ToolID: "t", SourceDir: source, TargetDir: target, LinkBack: true,
		Backups: store, Now: func() time.Time { return time.Unix(0, 0) },
	})
	if err != nil || !result.Linked || result.BackupID == "" {
		t.Fatalf("adopt = %+v %v", result, err)
	}
	kind, linkedTo, err := Classify(source)
	if err != nil || kind != KindJunction || linkedTo != target {
		t.Fatalf("tool copy = %q %q %v", kind, linkedTo, err)
	}
	if body, err := os.ReadFile(filepath.Join(source, "SKILL.md")); err != nil || string(body) != "# foo\n" {
		t.Fatalf("read through link: %q %v", body, err)
	}
	if body, err := os.ReadFile(filepath.Join(store.Dir, result.BackupID, "files", "SKILL.md")); err != nil || string(body) != "# foo\n" {
		t.Fatalf("backup: %q %v", body, err)
	}
}

func TestAdoptLinkFailureRestoresSource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "tool", "foo")
	writeFile(t, filepath.Join(source, "SKILL.md"), "# foo\n")
	target := filepath.Join(root, "vault", "skills", "分类", "foo")
	store := &BackupStore{Dir: filepath.Join(root, "backups")}
	_, err := Adopt(AdoptOptions{
		ToolID: "t", SourceDir: source, TargetDir: target, LinkBack: true, Backups: store,
		CreateJunction: func(link, target string) error { return errors.New("simulated") },
	})
	if err == nil {
		t.Fatal("adopt should fail")
	}
	if kind, _, _ := Classify(source); kind != KindDir {
		t.Fatalf("source = %q, want restored directory", kind)
	}
	if body, err := os.ReadFile(filepath.Join(source, "SKILL.md")); err != nil || string(body) != "# foo\n" {
		t.Fatalf("restored source: %q %v", body, err)
	}
	if body, err := os.ReadFile(filepath.Join(target, "SKILL.md")); err != nil || string(body) != "# foo\n" {
		t.Fatalf("vault copy should stay: %q %v", body, err)
	}
	if list := ListBackups(store); len(list) != 0 {
		t.Fatalf("backup left behind after restore: %+v", list)
	}
}
