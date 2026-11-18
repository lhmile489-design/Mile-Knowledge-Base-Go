package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

func open(t *testing.T, legacy string) *Store {
	t.Helper()
	s, f := Open(t.TempDir(), legacy)
	if f != nil {
		t.Fatalf("Open: %+v", f)
	}
	return s
}

func TestDefaultsAndPatch(t *testing.T) {
	dir := t.TempDir()
	s, f := Open(dir, "")
	if f != nil {
		t.Fatal(f)
	}
	got := s.Get()
	if got.Theme != "system" || got.CloseBehavior != "ask" || got.Layout.SidebarWidth != 232 {
		t.Fatalf("unexpected defaults: %+v", got)
	}

	theme := "dark"
	layout := got.Layout
	layout.SidebarWidth = 9999
	layout.DocWidth = 1
	if _, f := s.Patch(contract.SettingsPatch{Theme: &theme, Layout: &layout}); f != nil {
		t.Fatal(f)
	}
	got = s.Get()
	if got.Theme != "dark" {
		t.Errorf("theme = %q", got.Theme)
	}
	if got.Layout.SidebarWidth != SidebarMax || got.Layout.DocWidth != DocMin {
		t.Errorf("layout not clamped: %+v", got.Layout)
	}

	if _, f := s.Patch(contract.SettingsPatch{Theme: strPtr("purple")}); f == nil {
		t.Error("invalid theme accepted")
	}
}

func strPtr(s string) *string { return &s }

func TestUnknownFieldsPreserved(t *testing.T) {
	dir := t.TempDir()
	raw := `{
  "version": 1,
  "theme": "light",
  "closeBehavior": "tray",
  "futureField": {"a": 1},
  "layout": {"sidebarWidth": 240, "docWidth": 500, "docCollapsed": false, "sidebarCollapsed": false,
    "agnesControlsCollapsed": false, "agnesHistoryOpen": true, "agnesComposerHeight": 120},
  "recentVaults": [],
  "lastVaultId": null
}
`
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	s, f := Open(dir, "")
	if f != nil {
		t.Fatal(f)
	}
	theme := "dark"
	if _, f := s.Patch(contract.SettingsPatch{Theme: &theme}); f != nil {
		t.Fatal(f)
	}
	b, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if _, ok := doc["futureField"]; !ok {
		t.Error("unknown field was dropped on save")
	}
	if doc["theme"] != "dark" {
		t.Errorf("theme not saved: %v", doc["theme"])
	}
}

func TestCorruptFileIsBackedUp(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, f := Open(dir, "")
	if f != nil {
		t.Fatal(f)
	}
	if s.Get().Theme != "system" {
		t.Error("corrupt file did not fall back to defaults")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".corrupt" || filepath.Ext(e.Name()) == ".corrupt"[:0]+".corrupt" {
			found = true
		}
		if len(e.Name()) > len(FileName)+7 && e.Name()[len(FileName):len(FileName)+2] == ".2" {
			found = true
		}
	}
	if !found {
		t.Errorf("no .corrupt backup written: %v", entries)
	}
}

func TestRecentDocuments(t *testing.T) {
	dir := t.TempDir()
	s, f := Open(dir, "")
	if f != nil {
		t.Fatal(f)
	}
	for i := 0; i < 15; i++ {
		if _, f := s.NoteDocumentOpened("v1", filepath.ToSlash(filepath.Join("d", string(rune('a'+i))+".md")), "2026-11-18T00:00:00Z"); f != nil {
			t.Fatal(f)
		}
	}
	list := s.RecentDocuments("v1")
	if len(list) != MaxRecentDocs {
		t.Fatalf("len = %d, want %d", len(list), MaxRecentDocs)
	}
	// The newest entry is first.
	if list[0].Path != "d/o.md" {
		t.Errorf("first = %q", list[0].Path)
	}
	// Re-opening moves it to the front without duplicating.
	if _, f := s.NoteDocumentOpened("v1", "d/a.md", "2026-11-18T01:00:00Z"); f != nil {
		t.Fatal(f)
	}
	list = s.RecentDocuments("v1")
	if list[0].Path != "d/a.md" {
		t.Errorf("after reopen first = %q", list[0].Path)
	}
	seen := map[string]int{}
	for _, item := range list {
		seen[item.Path]++
	}
	if seen["d/a.md"] != 1 {
		t.Error("duplicate entry after reopen")
	}
	if f := s.ForgetVault("v1"); f != nil {
		t.Fatal(f)
	}
	if len(s.RecentDocuments("v1")) != 0 {
		t.Error("forget did not clear recent documents")
	}
}

func TestImportLegacyOnce(t *testing.T) {
	legacy := t.TempDir()
	dir := t.TempDir()
	legacySettings := map[string]any{
		"theme":         "dark",
		"closeBehavior": "tray",
		"layout": map[string]any{
			"sidebarWidth": 260, "docWidth": 620, "docCollapsed": false, "sidebarCollapsed": false,
			"agnesControlsCollapsed": false, "agnesHistoryOpen": true, "agnesComposerHeight": 140,
		},
		"recentVaults": []map[string]any{
			{"id": "aaaaaaaaaaaaaaaa", "root": `E:\vault-a`, "displayName": "vault-a", "lastOpenedAt": "2026-11-17T10:00:00Z"},
			{"id": "bbbbbbbbbbbbbbbb", "root": `E:\vault-b`, "displayName": "vault-b", "lastOpenedAt": "2026-11-16T10:00:00Z"},
		},
		"lastVaultId": "aaaaaaaaaaaaaaaa",
	}
	b, _ := json.Marshal(legacySettings)
	if err := os.WriteFile(filepath.Join(legacy, FileName), b, 0o644); err != nil {
		t.Fatal(err)
	}
	recent := map[string]any{
		"aaaaaaaaaaaaaaaa": []map[string]string{{"path": "raws/a.md", "openedAt": "2026-11-17T11:00:00Z"}},
		"cccccccccccccccc": []map[string]string{{"path": "raws/c.md", "openedAt": "2026-11-17T11:00:00Z"}},
	}
	b, _ = json.Marshal(recent)
	if err := os.WriteFile(filepath.Join(legacy, RecentFileName), b, 0o644); err != nil {
		t.Fatal(err)
	}

	s, f := Open(dir, legacy)
	if f != nil {
		t.Fatal(f)
	}
	res := s.ImportLegacy()
	if !res.Ran || !res.Imported || res.Vaults != 2 {
		t.Fatalf("import result: %+v", res)
	}
	got := s.Get()
	if got.Theme != "dark" || got.CloseBehavior != "tray" || got.LastVaultID != "aaaaaaaaaaaaaaaa" {
		t.Errorf("imported settings wrong: %+v", got)
	}
	if got.Layout.SidebarWidth != 260 || got.Layout.AgnesComposerHeight != 140 {
		t.Errorf("imported layout wrong: %+v", got.Layout)
	}
	if n := len(s.RecentDocuments("aaaaaaaaaaaaaaaa")); n != 1 {
		t.Errorf("imported recent docs = %d", n)
	}
	if n := len(s.RecentDocuments("cccccccccccccccc")); n != 0 {
		t.Error("recent docs of an unknown vault were imported")
	}

	// Second run does not import again.
	if res := s.ImportLegacy(); res.Ran {
		t.Error("import ran twice")
	}
	if _, err := os.Stat(filepath.Join(dir, ImportMarker)); err != nil {
		t.Errorf("marker missing: %v", err)
	}
}

func TestImportLegacyMissingSource(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(t.TempDir(), "does-not-exist")
	s, f := Open(dir, legacy)
	if f != nil {
		t.Fatal(f)
	}
	res := s.ImportLegacy()
	if !res.Ran {
		t.Fatal("import should run once even without a legacy install")
	}
	if s.Get().Theme != "system" {
		t.Error("missing source changed settings")
	}
	if _, err := os.Stat(filepath.Join(dir, ImportMarker)); err != nil {
		t.Error("marker not written for a missing source")
	}
}
