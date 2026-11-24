package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// M9 T00: the skills page nav remembers two collapsed groups in layout
// (PRD Q-UX-3, API.md §16, 数据库设计 4.1).

func TestSkillsNavCollapseDefaultsToExpanded(t *testing.T) {
	got := Defaults().Layout
	if got.SkillsKbCollapsed || got.SkillsToolsCollapsed {
		t.Fatalf("defaults must be expanded: %+v", got)
	}

	// A 1.6.x settings.json has neither key: both read as false.
	dir := t.TempDir()
	raw := `{
  "version": 1,
  "theme": "light",
  "closeBehavior": "tray",
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
	layout := s.Get().Layout
	if layout.SkillsKbCollapsed || layout.SkillsToolsCollapsed {
		t.Fatalf("missing keys must read as false: %+v", layout)
	}
	if layout.SidebarWidth != 240 || layout.AgnesComposerHeight != 120 {
		t.Fatalf("existing layout fields changed: %+v", layout)
	}
}

func TestSkillsNavCollapsePatchRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, f := Open(dir, "")
	if f != nil {
		t.Fatal(f)
	}
	layout := s.Get().Layout
	layout.SkillsKbCollapsed = true
	got, f := s.Patch(contract.SettingsPatch{Layout: &layout})
	if f != nil {
		t.Fatal(f)
	}
	if !got.Layout.SkillsKbCollapsed || got.Layout.SkillsToolsCollapsed {
		t.Fatalf("patch result: %+v", got.Layout)
	}

	// The file on disk carries the keys and a fresh store reads them back.
	b, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Layout map[string]any `json:"layout"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Layout["skillsKbCollapsed"] != true || doc.Layout["skillsToolsCollapsed"] != false {
		t.Fatalf("persisted layout: %v", doc.Layout)
	}
	reopened, f := Open(dir, "")
	if f != nil {
		t.Fatal(f)
	}
	again := reopened.Get().Layout
	if !again.SkillsKbCollapsed || again.SkillsToolsCollapsed {
		t.Fatalf("reopened layout: %+v", again)
	}

	// Collapsing the other group keeps the first one and the geometry.
	layout = reopened.Get().Layout
	layout.SkillsToolsCollapsed = true
	if _, f := reopened.Patch(contract.SettingsPatch{Layout: &layout}); f != nil {
		t.Fatal(f)
	}
	final := reopened.Get().Layout
	if !final.SkillsKbCollapsed || !final.SkillsToolsCollapsed || final.SidebarWidth != SidebarDef {
		t.Fatalf("second patch: %+v", final)
	}

	// Expanding writes false back, not a missing key.
	layout = final
	layout.SkillsKbCollapsed = false
	layout.SkillsToolsCollapsed = false
	if _, f := reopened.Patch(contract.SettingsPatch{Layout: &layout}); f != nil {
		t.Fatal(f)
	}
	if l := reopened.Get().Layout; l.SkillsKbCollapsed || l.SkillsToolsCollapsed {
		t.Fatalf("expand did not stick: %+v", l)
	}
}

func TestSkillsNavCollapseKeepsTopLevelUnknownFields(t *testing.T) {
	dir := t.TempDir()
	raw := `{
  "version": 1,
  "theme": "light",
  "closeBehavior": "tray",
  "futureField": {"a": 1},
  "layout": {"sidebarWidth": 240, "docWidth": 500, "docCollapsed": false, "sidebarCollapsed": false,
    "agnesControlsCollapsed": false, "agnesHistoryOpen": true, "agnesComposerHeight": 120,
    "skillsKbCollapsed": true},
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
	if !s.Get().Layout.SkillsKbCollapsed {
		t.Fatalf("stored flag not loaded: %+v", s.Get().Layout)
	}
	layout := s.Get().Layout
	layout.SkillsToolsCollapsed = true
	if _, f := s.Patch(contract.SettingsPatch{Layout: &layout}); f != nil {
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
		t.Error("top-level unknown field was dropped")
	}
	l, _ := doc["layout"].(map[string]any)
	if l["skillsKbCollapsed"] != true || l["skillsToolsCollapsed"] != true {
		t.Errorf("layout after patch: %v", l)
	}
}
