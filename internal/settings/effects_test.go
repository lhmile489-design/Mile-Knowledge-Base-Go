package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

func TestEffectsDefaults(t *testing.T) {
	s := open(t, "")
	got := s.Get().Effects
	if got.Motion != "std" || got.Blur != "std" {
		t.Fatalf("defaults = %+v, want std/std", got)
	}
	if d := Defaults().Effects; d.Motion != "std" || d.Blur != "std" {
		t.Fatalf("Defaults().Effects = %+v", d)
	}
}

func TestEffectsLoadFallbacks(t *testing.T) {
	cases := []struct {
		name string
		body string
		want contract.EffectsSetting
	}{
		{"valid", `"effects": {"motion": "high", "blur": "off"}`, contract.EffectsSetting{Motion: "high", Blur: "off"}},
		{"missing object", `"theme": "light"`, contract.EffectsSetting{Motion: "std", Blur: "std"}},
		{"invalid values", `"effects": {"motion": "turbo", "blur": "loud"}`, contract.EffectsSetting{Motion: "std", Blur: "std"}},
		{"missing one field", `"effects": {"motion": "low"}`, contract.EffectsSetting{Motion: "low", Blur: "std"}},
		{"wrong type", `"effects": "high"`, contract.EffectsSetting{Motion: "std", Blur: "std"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			raw := "{\n  \"version\": 1,\n  \"theme\": \"light\",\n  " + tc.body + "\n}\n"
			if err := os.WriteFile(filepath.Join(dir, FileName), []byte(raw), 0o644); err != nil {
				t.Fatal(err)
			}
			s, f := Open(dir, "")
			if f != nil {
				t.Fatalf("Open: %+v", f)
			}
			if got := s.Get().Effects; got != tc.want {
				t.Fatalf("effects = %+v, want %+v", got, tc.want)
			}
			// A wrong-typed object must not be treated as a corrupt file.
			if _, err := os.Stat(filepath.Join(dir, FileName) + ".corrupt"); err == nil {
				t.Fatalf("file was backed up as corrupt")
			}
			matches, _ := filepath.Glob(filepath.Join(dir, FileName+".*.corrupt"))
			if len(matches) > 0 {
				t.Fatalf("corrupt backup written: %v", matches)
			}
		})
	}
}

func TestEffectsPatchValidation(t *testing.T) {
	dir := t.TempDir()
	s, f := Open(dir, "")
	if f != nil {
		t.Fatal(f)
	}

	valid := contract.EffectsSetting{Motion: "low", Blur: "off"}
	if _, f := s.Patch(contract.SettingsPatch{Effects: &valid}); f != nil {
		t.Fatalf("valid patch rejected: %+v", f)
	}
	if got := s.Get().Effects; got != valid {
		t.Fatalf("effects = %+v after patch", got)
	}

	reopened, f := Open(dir, "")
	if f != nil {
		t.Fatal(f)
	}
	if got := reopened.Get().Effects; got != valid {
		t.Fatalf("effects = %+v after reopen, want %+v", got, valid)
	}

	bad := []struct {
		name  string
		value contract.EffectsSetting
	}{
		{"unknown motion", contract.EffectsSetting{Motion: "turbo", Blur: "std"}},
		{"unknown blur", contract.EffectsSetting{Motion: "std", Blur: "loud"}},
		{"missing motion", contract.EffectsSetting{Motion: "", Blur: "std"}},
		{"missing blur", contract.EffectsSetting{Motion: "high", Blur: ""}},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			p := tc.value
			_, f := s.Patch(contract.SettingsPatch{Effects: &p})
			if f == nil || f.Code != contract.CodeInvalidInput {
				t.Fatalf("failure = %+v, want invalid-input", f)
			}
			if got := s.Get().Effects; got != valid {
				t.Fatalf("failed patch changed effects to %+v", got)
			}
		})
	}

	// Extra keys inside the effects object are an invalid-input on patch
	// (API.md §17), not silently dropped.
	var patch contract.SettingsPatch
	if err := json.Unmarshal([]byte(`{"effects": {"motion": "high", "blur": "std", "zoom": "max"}}`), &patch); err != nil {
		t.Fatal(err)
	}
	if !patch.HasUnknownEffectsFields() {
		t.Fatal("unknown effects key not recorded")
	}
	if _, f := s.Patch(patch); f == nil || f.Code != contract.CodeInvalidInput {
		t.Fatalf("failure = %+v, want invalid-input", f)
	}

	// A patch without effects leaves the stored value alone.
	theme := "dark"
	if _, f := s.Patch(contract.SettingsPatch{Theme: &theme}); f != nil {
		t.Fatal(f)
	}
	if got := s.Get().Effects; got != valid {
		t.Fatalf("effects = %+v after unrelated patch", got)
	}
}

func TestEffectsUnknownTopLevelPreserved(t *testing.T) {
	dir := t.TempDir()
	raw := `{
  "version": 1,
  "theme": "light",
  "futureField": {"a": 1},
  "effects": {"motion": "std", "blur": "std"}
}
`
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	s, f := Open(dir, "")
	if f != nil {
		t.Fatal(f)
	}
	next := contract.EffectsSetting{Motion: "high", Blur: "low"}
	if _, f := s.Patch(contract.SettingsPatch{Effects: &next}); f != nil {
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
		t.Fatalf("top-level unknown field lost: %s", string(b))
	}
	effects, _ := doc["effects"].(map[string]any)
	if effects["motion"] != "high" || effects["blur"] != "low" {
		t.Fatalf("written effects = %v", doc["effects"])
	}
}

func TestEffectsNestedUnknownDropped(t *testing.T) {
	// 数据库设计 4.1: only top-level unknown fields are preserved; keys inside
	// effects are not. This test pins that documented behaviour.
	dir := t.TempDir()
	raw := `{
  "version": 1,
  "theme": "light",
  "effects": {"motion": "low", "blur": "std", "future": 1}
}
`
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	s, f := Open(dir, "")
	if f != nil {
		t.Fatal(f)
	}
	if got := s.Get().Effects; got.Motion != "low" || got.Blur != "std" {
		t.Fatalf("effects = %+v", got)
	}
	theme := "dark"
	if _, f := s.Patch(contract.SettingsPatch{Theme: &theme}); f != nil {
		t.Fatal(f)
	}
	b, _ := os.ReadFile(filepath.Join(dir, FileName))
	var doc struct {
		Effects map[string]any `json:"effects"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if _, ok := doc.Effects["future"]; ok {
		t.Fatalf("nested unknown key was preserved: %s", string(b))
	}
}
