package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeSkillFile(t *testing.T, root, rel, text string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A skill written into the vault shows up in the scan only after the index has
// seen it; the skills page learns about that through OnSkillsIndexed (M9 T06,
// AC-UX-5).
func TestIndexWorkerReportsSkillBatches(t *testing.T) {
	root := t.TempDir()
	writeSkillFile(t, root, "skills/前端/vue/SKILL.md", "# vue\n")
	writeSkillFile(t, root, "notes/a.md", "# a\n")
	app, err := New(t.TempDir(), filepath.Join(t.TempDir(), "legacy"))
	if err != nil {
		t.Fatal(err)
	}
	got := make(chan string, 8)
	app.OnSkillsIndexed = func(vaultID string) { got <- vaultID }
	v, f := app.Vaults.Register(root)
	if f != nil {
		t.Fatalf("register: %+v", f)
	}
	app.startIndex(v.ID)
	t.Cleanup(func() { app.indexStop(v.ID, false) })
	waitPhase(t, app, v.ID, "ready")

	// A batch that only touches notes is not a skills change.
	writeSkillFile(t, root, "notes/b.md", "# b\n")
	app.enqueueIndex(v.ID, []string{"notes"})
	select {
	case id := <-got:
		t.Fatalf("notes batch reported as skills change for %s", id)
	case <-time.After(700 * time.Millisecond):
	}

	// A batch under skills/ is reported once the index holds the new file.
	writeSkillFile(t, root, "skills/文档/pdf/SKILL.md", "# pdf\n")
	app.enqueueIndex(v.ID, []string{"skills/文档"})
	select {
	case id := <-got:
		if id != v.ID {
			t.Fatalf("reported vault %q, want %q", id, v.ID)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("skills batch was never reported")
	}
	handle := app.indexHandle(v.ID)
	handle.mu.Lock()
	store := handle.store
	handle.mu.Unlock()
	docs, err := store.SkillDocuments(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, doc := range docs {
		if doc.Path == "skills/文档/pdf/SKILL.md" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the hook ran before the index had the new skill: %+v", docs)
	}

	// A full pass (the overflow path) covers skills too.
	app.enqueueIndex(v.ID, []string{""})
	select {
	case <-got:
	case <-time.After(10 * time.Second):
		t.Fatal("full pass was never reported")
	}
}

func TestTouchesSkills(t *testing.T) {
	cases := []struct {
		dirs []string
		want bool
	}{
		{nil, false},
		{[]string{"notes"}, false},
		{[]string{"skillset"}, false},
		{[]string{"skills"}, true},
		{[]string{"skills/前端"}, true},
		{[]string{"notes", "skills/x"}, true},
		{[]string{""}, true},
	}
	for _, tc := range cases {
		if got := touchesSkills(tc.dirs); got != tc.want {
			t.Errorf("touchesSkills(%q) = %v, want %v", tc.dirs, got, tc.want)
		}
	}
}

// The scan cache would answer for two more seconds; the index change has to
// drop it so the next scan reads the fresh index.
func TestSkillServiceIndexUpdatedDropsTheScanCache(t *testing.T) {
	app, err := New(t.TempDir(), filepath.Join(t.TempDir(), "legacy"))
	if err != nil {
		t.Fatal(err)
	}
	svc := NewSkillService(app)
	if app.OnSkillsIndexed == nil {
		t.Fatal("NewSkillService did not register the index hook")
	}
	cache := svc.cachePath()
	if err := os.MkdirAll(filepath.Dir(cache), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cache, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := svc.seq
	app.OnSkillsIndexed("v1")
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Errorf("scan cache survived: %v", err)
	}
	if svc.seq != before+1 {
		t.Errorf("E12 sequence %d, want %d", svc.seq, before+1)
	}
}

// A scan that started before a change must not write its older result into the
// cache after the change dropped it: the next load would answer from it for two
// seconds and the page would keep showing the old list.
func TestScanStartedBeforeInvalidationDoesNotWriteTheCache(t *testing.T) {
	app, err := New(t.TempDir(), filepath.Join(t.TempDir(), "legacy"))
	if err != nil {
		t.Fatal(err)
	}
	svc := NewSkillService(app)
	svc.beforeSaveScan = func() { svc.dropScanCache() }
	svc.runScan(t.Context(), "")
	if _, err := os.Stat(svc.cachePath()); !os.IsNotExist(err) {
		t.Fatalf("a stale scan wrote the cache: %v", err)
	}

	// Without an invalidation in between, the scan is cached as before.
	svc.beforeSaveScan = nil
	svc.runScan(t.Context(), "")
	if _, err := os.Stat(svc.cachePath()); err != nil {
		t.Fatalf("a current scan did not write the cache: %v", err)
	}
}
