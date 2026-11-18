package testkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildTypical(t *testing.T) {
	root := t.TempDir()
	if err := BuildTypical(root); err != nil {
		t.Fatalf("BuildTypical: %v", err)
	}

	for _, rel := range []string{
		"skills/react/SKILL.md",
		"raws/富文本样例.md",
		"raws/attachments/sample.png",
		"raws/big-1m.md",
		"raws/huge-2m.md",
		".mile-kb/settings/aliases.json",
		"my-notes/同名文档.md",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Errorf("%s missing: %v", rel, err)
		}
	}

	crlf, err := os.ReadFile(filepath.Join(root, "my-notes", "CRLF与BOM.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(crlf), "\ufeff") {
		t.Error("CRLF sample lost its BOM")
	}
	if !strings.Contains(string(crlf), "\r\n") {
		t.Error("CRLF sample lost its CRLF endings")
	}

	big, err := os.Stat(filepath.Join(root, "raws", "big-1m.md"))
	if err != nil {
		t.Fatal(err)
	}
	if big.Size() <= 1<<20 || big.Size() >= 2<<20 {
		t.Errorf("big-1m.md size = %d, want (1 MiB, 2 MiB)", big.Size())
	}
	huge, err := os.Stat(filepath.Join(root, "raws", "huge-2m.md"))
	if err != nil {
		t.Fatal(err)
	}
	if huge.Size() <= 2<<20 {
		t.Errorf("huge-2m.md size = %d, want > 2 MiB", huge.Size())
	}
}

func TestBuildBigCounts(t *testing.T) {
	if testing.Short() {
		t.Skip("1 万文件样本太慢，-short 跳过")
	}
	root := t.TempDir()
	if err := BuildBig(root); err != nil {
		t.Fatalf("BuildBig: %v", err)
	}
	var md, total int
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		total++
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			md++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if md != 10000 {
		t.Errorf("markdown files = %d, want 10000", md)
	}
	if total < 15000 {
		t.Errorf("entries = %d, want >= 15000", total)
	}
}
