package mutation

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

func restoreManager(t *testing.T, root string) TxManager {
	t.Helper()
	return TxManager{Root: root, Now: func() time.Time {
		return time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC)
	}, Rand: func() string { return "restore1" }}
}

func TestRestoreReturnsOriginalBytes(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "raws/a.md", "原始内容\n")
	manager := restoreManager(t, root)

	tx, err := manager.Begin("v", "p")
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Capture(tx, "raws/a.md", filepath.Join(root, "raws", "a.md")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "raws/a.md", "改过的内容\n")
	if err := manager.CommitWithAfterImages(tx); err != nil {
		t.Fatal(err)
	}

	restored, conflicted, touched, f := manager.Restore(tx.ID)
	if f != nil {
		t.Fatal(f)
	}
	if len(restored) != 1 || len(conflicted) != 0 {
		t.Fatalf("restored=%v conflicted=%v", restored, conflicted)
	}
	body, _ := os.ReadFile(filepath.Join(root, "raws", "a.md"))
	if string(body) != "原始内容\n" {
		t.Errorf("body = %q", body)
	}
	if len(touched) != 1 || touched[0] != "raws" {
		t.Errorf("touched = %v", touched)
	}
}

func TestRestoreConflictsWhenChangedAfterTransaction(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "raws/a.md", "原始\n")
	manager := restoreManager(t, root)

	tx, _ := manager.Begin("v", "p")
	_ = manager.Capture(tx, "raws/a.md", filepath.Join(root, "raws", "a.md"))
	writeFile(t, root, "raws/a.md", "事务后\n")
	_ = manager.CommitWithAfterImages(tx)
	// External edit after the transaction committed.
	writeFile(t, root, "raws/a.md", "外部又改了\n")

	preview, f := manager.Preview(tx.ID)
	if f != nil {
		t.Fatal(f)
	}
	if len(preview) != 1 || preview[0].Restorable {
		t.Fatalf("preview = %+v", preview)
	}
	if preview[0].Note == "" {
		t.Error("conflict needs a note")
	}

	restored, conflicted, _, f := manager.Restore(tx.ID)
	if f != nil {
		t.Fatal(f)
	}
	if len(restored) != 0 || len(conflicted) != 1 {
		t.Fatalf("restored=%v conflicted=%v", restored, conflicted)
	}
	body, _ := os.ReadFile(filepath.Join(root, "raws", "a.md"))
	if string(body) != "外部又改了\n" {
		t.Errorf("conflict overwrote the file: %q", body)
	}
}

func TestRestoreAbsentDeletesAndDirectoryRecreates(t *testing.T) {
	root := t.TempDir()
	manager := restoreManager(t, root)

	tx, _ := manager.Begin("v", "p")
	// A file that did not exist before the plan: restoring removes it.
	_ = manager.Capture(tx, "raws/new.md", filepath.Join(root, "raws", "new.md"))
	writeFile(t, root, "raws/new.md", "新建\n")
	// A directory that existed: restoring recreates it.
	writeFile(t, root, "raws/dir/keep.md", "keep\n")
	_ = manager.Capture(tx, "raws/dir", filepath.Join(root, "raws", "dir"))
	_ = os.RemoveAll(filepath.Join(root, "raws", "dir"))
	_ = manager.CommitWithAfterImages(tx)

	restored, conflicted, _, f := manager.Restore(tx.ID)
	if f != nil {
		t.Fatal(f)
	}
	if len(conflicted) != 0 || len(restored) != 2 {
		t.Fatalf("restored=%v conflicted=%v", restored, conflicted)
	}
	if _, err := os.Lstat(filepath.Join(root, "raws", "new.md")); !os.IsNotExist(err) {
		t.Error("absent entry was not removed")
	}
	if info, err := os.Stat(filepath.Join(root, "raws", "dir")); err != nil || !info.IsDir() {
		t.Error("directory was not recreated")
	}
}

func TestRestoreRejectsUnsafeIDs(t *testing.T) {
	root := t.TempDir()
	manager := restoreManager(t, root)
	for _, id := range []string{"", "../x", "a/b", "a b", ".."} {
		if _, f := manager.ReadTx(id); f == nil || f.Code != contract.CodeInvalidInput {
			t.Errorf("%q accepted: %+v", id, f)
		}
	}
	if _, f := manager.Preview("does-not-exist"); f == nil || f.Code != contract.CodeNotFound {
		t.Errorf("missing tx = %+v", f)
	}
}

func TestListTransactionsNewestFirst(t *testing.T) {
	root := t.TempDir()
	manager := restoreManager(t, root)
	first, _ := manager.Begin("v", "p1")
	manager.Now = func() time.Time { return time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC) }
	second, _ := manager.Begin("v", "p2")
	list := manager.ListTx()
	if len(list) != 2 || list[0].ID != second.ID || list[1].ID != first.ID {
		t.Fatalf("list = %+v", list)
	}
}
