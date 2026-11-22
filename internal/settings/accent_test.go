package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// M3.5 T01: the accent field per API.md §11 and 数据库设计 4.1.1.

func writeSettings(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func testTokens() contract.AccentTokens {
	return contract.AccentTokens{
		Accent: "#b95b1e", Hover: "#a6521b", Active: "#944918", Soft: "#fceee5",
		Text: "#aa541c", Btn: "#b95b1e", BtnHover: "#a6521b", BtnActive: "#944918",
		OnAccent: "#ffffff", Focus: "#e87226",
	}
}

// T10 审查需改 2：accent 对象里其他版本写的键，读写往返后仍在。
func TestAccentUnknownFieldsPreserved(t *testing.T) {
	dir := writeSettings(t, `{
  "version": 1,
  "theme": "light",
  "closeBehavior": "ask",
  "accent": {"preset": "blue", "futureX": 1, "future": {"a": true}},
  "layout": {"sidebarWidth": 240, "docWidth": 500, "docCollapsed": false, "sidebarCollapsed": false,
    "agnesControlsCollapsed": false, "agnesHistoryOpen": true, "agnesComposerHeight": 120},
  "recentVaults": []
}
`)
	s, f := Open(dir, "")
	if f != nil {
		t.Fatal(f)
	}
	if got := s.Get().Accent.Preset; got != "blue" {
		t.Fatalf("accent = %q, want blue", got)
	}
	theme := "dark"
	if _, f := s.Patch(contract.SettingsPatch{Theme: &theme}); f != nil {
		t.Fatal(f)
	}
	b, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Accent map[string]json.RawMessage `json:"accent"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if _, ok := doc.Accent["futureX"]; !ok {
		t.Error("unknown accent key futureX was dropped on save")
	}
	if _, ok := doc.Accent["future"]; !ok {
		t.Error("unknown accent key future was dropped on save")
	}
	if string(doc.Accent["preset"]) != `"blue"` {
		t.Errorf("preset = %s, want \"blue\"", doc.Accent["preset"])
	}
}

// T10 审查需改 3：API 11.2 的多余字段必须整条拒绝，不能静默接受。
func TestAccentPatchRejectsUnknownFields(t *testing.T) {
	s := open(t, "")
	var patch contract.SettingsPatch
	if err := json.Unmarshal([]byte(`{"accent":{"preset":"blue","extra":1}}`), &patch); err != nil {
		t.Fatal(err)
	}
	if !patch.HasUnknownAccentFields() {
		t.Fatal("unknown accent field was not flagged")
	}
	if _, f := s.Patch(patch); f == nil || f.Code != contract.CodeInvalidInput {
		t.Fatalf("want invalid-input, got %+v", f)
	}
	if got := s.Get().Accent.Preset; got != "orange" {
		t.Fatalf("accent changed to %q after a rejected patch", got)
	}

	var cachePatch contract.SettingsPatch
	if err := json.Unmarshal([]byte(`{"accent":{"preset":"custom","hex":"#123456","cache":{"light":{},"dark":{},"extra":1}}}`), &cachePatch); err != nil {
		t.Fatal(err)
	}
	if !cachePatch.HasUnknownAccentFields() {
		t.Fatal("unknown cache field was not flagged")
	}

	var okPatch contract.SettingsPatch
	if err := json.Unmarshal([]byte(`{"accent":{"preset":"teal"}}`), &okPatch); err != nil {
		t.Fatal(err)
	}
	if okPatch.HasUnknownAccentFields() {
		t.Fatal("clean patch was flagged")
	}
	if _, f := s.Patch(okPatch); f != nil {
		t.Fatal(f)
	}
	if got := s.Get().Accent.Preset; got != "teal" {
		t.Fatalf("accent = %q, want teal", got)
	}
}

func TestAccentDefaultsWhenOmitted(t *testing.T) {
	s := open(t, "")
	if got := s.Get().Accent; got.Preset != "orange" || got.Hex != "" || got.Cache != nil {
		t.Fatalf("accent = %+v, want orange", got)
	}
}

func TestAccentFallbacks(t *testing.T) {
	cases := []struct {
		name string
		body string
		want contract.AccentSetting
	}{
		{"string value", `{"version":1,"accent":"blue"}`, contract.AccentSetting{Preset: "orange"}},
		{"unknown preset", `{"version":1,"accent":{"preset":"pink"}}`, contract.AccentSetting{Preset: "orange"}},
		{"no preset", `{"version":1,"accent":{}}`, contract.AccentSetting{Preset: "orange"}},
		{"custom without hex", `{"version":1,"accent":{"preset":"custom"}}`, contract.AccentSetting{Preset: "orange"}},
		{"custom short hex", `{"version":1,"accent":{"preset":"custom","hex":"#fff"}}`, contract.AccentSetting{Preset: "orange"}},
		{
			"custom with broken cache keeps hex",
			`{"version":1,"accent":{"preset":"custom","hex":"#2563eb","cache":{"light":{"accent":"#2563eb"}}}}`,
			contract.AccentSetting{Preset: "custom", Hex: "#2563eb"},
		},
		{
			"known preset drops stray hex",
			`{"version":1,"accent":{"preset":"teal","hex":"#123456"}}`,
			contract.AccentSetting{Preset: "teal"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := writeSettings(t, c.body)
			s, f := Open(dir, "")
			if f != nil {
				t.Fatal(f)
			}
			got := s.Get().Accent
			if got != c.want {
				t.Fatalf("accent = %+v, want %+v", got, c.want)
			}
			// 读入时的回落不写盘：文件字节保持不变。
			if raw, err := os.ReadFile(filepath.Join(dir, FileName)); err != nil || string(raw) != c.body {
				t.Fatalf("file rewritten on load: %q %v", string(raw), err)
			}
		})
	}
}

func TestAccentPatchCustomLowercasesAndKeepsCache(t *testing.T) {
	s := open(t, "")
	cache := &contract.AccentCache{Light: testTokens(), Dark: testTokens()}
	p := contract.AccentSetting{Preset: "custom", Hex: "#FFFF00", Cache: cache}
	if _, f := s.Patch(contract.SettingsPatch{Accent: &p}); f != nil {
		t.Fatal(f)
	}
	got := s.Get().Accent
	if got.Preset != "custom" || got.Hex != "#ffff00" || got.Cache == nil {
		t.Fatalf("accent = %+v", got)
	}
	if got.Cache.Light.Btn != "#b95b1e" || got.Cache.Dark.OnAccent != "#ffffff" {
		t.Fatalf("cache lost: %+v", got.Cache)
	}
}

func TestAccentPresetDropsHexAndCache(t *testing.T) {
	s := open(t, "")
	custom := contract.AccentSetting{Preset: "custom", Hex: "#ffff00"}
	if _, f := s.Patch(contract.SettingsPatch{Accent: &custom}); f != nil {
		t.Fatal(f)
	}
	teal := contract.AccentSetting{Preset: "teal", Hex: "#123456", Cache: &contract.AccentCache{Light: testTokens(), Dark: testTokens()}}
	if _, f := s.Patch(contract.SettingsPatch{Accent: &teal}); f != nil {
		t.Fatal(f)
	}
	got := s.Get().Accent
	if got.Preset != "teal" || got.Hex != "" || got.Cache != nil {
		t.Fatalf("accent = %+v, want teal only", got)
	}
}

func TestAccentPatchInvalidChangesNothing(t *testing.T) {
	s := open(t, "")
	theme := "dark"
	bad := []contract.AccentSetting{
		{Preset: "pink"},
		{Preset: "custom", Hex: "#gggggg"},
		{Preset: "custom", Hex: "2563eb"},
		{Preset: "custom", Hex: "#2563eb", Cache: &contract.AccentCache{Light: testTokens()}},
	}
	broken := testTokens()
	broken.OnAccent = "white"
	bad = append(bad, contract.AccentSetting{Preset: "custom", Hex: "#2563eb", Cache: &contract.AccentCache{Light: testTokens(), Dark: broken}})
	for _, p := range bad {
		p := p
		// 同一个 patch 里的 theme 也不能生效。
		if _, f := s.Patch(contract.SettingsPatch{Accent: &p, Theme: &theme}); f == nil || f.Code != contract.CodeInvalidInput {
			t.Fatalf("%+v: failure = %+v, want invalid-input", p, f)
		}
	}
	if got := s.Get(); got.Theme != "system" || got.Accent.Preset != "orange" {
		t.Fatalf("settings changed: theme=%s accent=%+v", got.Theme, got.Accent)
	}
}

func TestAccentSurvivesOtherPatches(t *testing.T) {
	s := open(t, "")
	custom := contract.AccentSetting{Preset: "custom", Hex: "#ffff00"}
	if _, f := s.Patch(contract.SettingsPatch{Accent: &custom}); f != nil {
		t.Fatal(f)
	}
	theme := "dark"
	if _, f := s.Patch(contract.SettingsPatch{Theme: &theme}); f != nil {
		t.Fatal(f)
	}
	raw, err := os.ReadFile(filepath.Join(s.dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"accent"`) || !strings.Contains(string(raw), `"#ffff00"`) {
		t.Fatalf("accent missing from file: %s", raw)
	}
}
