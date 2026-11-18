package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func waitPhase(t *testing.T, app *App, vaultID, want string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if app.indexStatus(vaultID).Phase == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("index never reached %q, last %+v", want, app.indexStatus(vaultID))
}

func TestIndexServiceBuildsAndRebuilds(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte("# 甲\n\n知识库。\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
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
	waitPhase(t, app, v.ID, "ready")

	status := app.indexStatus(v.ID)
	if status.DocumentCount != 2 {
		t.Errorf("documentCount = %d", status.DocumentCount)
	}
	if _, err := os.Stat(indexPath(app.DataDir, v.ID)); err != nil {
		t.Errorf("index file missing: %v", err)
	}

	// Rebuild starts from scratch: the file is recreated and the scan runs.
	app.indexRebuild(v.ID)
	waitPhase(t, app, v.ID, "ready")
	if got := app.indexStatus(v.ID); got.DocumentCount != 2 {
		t.Errorf("after rebuild documentCount = %d", got.DocumentCount)
	}

	// Stopping with removal must leave no index files behind (vault removal).
	app.indexStop(v.ID, true)
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if _, err := os.Stat(indexPath(app.DataDir, v.ID) + suffix); !os.IsNotExist(err) {
			t.Errorf("index file %q survived removal", suffix)
		}
	}
}
