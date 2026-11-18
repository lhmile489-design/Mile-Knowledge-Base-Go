package ai

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

func ptr[T any](v T) *T { return &v }

func TestNormalizeProvider(t *testing.T) {
	cases := []struct {
		name string
		in   map[string]any
		ok   bool
	}{
		{"deepseek preset", map[string]any{"name": "deepseek", "kind": "openai", "baseUrl": "https://api.deepseek.com", "models": []any{"deepseek-v4-flash"}, "defaultModel": "", "apiKeyEnv": "DEEPSEEK_API_KEY", "contextWindow": 1000000}, true},
		{"empty name", map[string]any{"name": "", "kind": "openai", "baseUrl": "https://x.test", "models": []any{"m"}}, false},
		{"name starting with dash", map[string]any{"name": "-x", "kind": "openai", "baseUrl": "https://x.test", "models": []any{"m"}}, false},
		{"bad kind", map[string]any{"name": "x", "kind": "gemini", "baseUrl": "https://x.test", "models": []any{"m"}}, false},
		{"file scheme", map[string]any{"name": "x", "kind": "openai", "baseUrl": "file:///c:/x", "models": []any{"m"}}, false},
		{"no host", map[string]any{"name": "x", "kind": "openai", "baseUrl": "https://?x", "models": []any{"m"}}, false},
		{"model name in env field", map[string]any{"name": "x", "kind": "openai", "baseUrl": "https://x.test", "models": []any{"m"}, "apiKeyEnv": "grok-4.5"}, false},
		{"no models", map[string]any{"name": "x", "kind": "openai", "baseUrl": "https://x.test", "models": []any{"  "}}, false},
		{"too many models", map[string]any{"name": "x", "kind": "openai", "baseUrl": "https://x.test", "models": manyModels(33)}, false},
		{"model too long", map[string]any{"name": "x", "kind": "openai", "baseUrl": "https://x.test", "models": []any{strings.Repeat("m", 129)}}, false},
		{"anthropic is a valid kind", map[string]any{"name": "x", "kind": "anthropic", "baseUrl": "https://x.test", "models": []any{"m"}, "apiKeyEnv": ""}, true},
	}
	for _, c := range cases {
		got := NormalizeProvider(c.in)
		if got.OK != c.ok {
			t.Errorf("%s: ok=%v failure=%+v", c.name, got.OK, got.Failure)
		}
	}
}

func TestNormalizeProviderDefaultsAndDedupe(t *testing.T) {
	got := NormalizeProvider(map[string]any{
		"name": "  deepseek ", "kind": "openai", "baseUrl": " https://api.deepseek.com ",
		"models": []any{"a", " a ", "b"}, "defaultModel": "missing",
		"apiKeyEnv": "", "contextWindow": -1,
	})
	if !got.OK {
		t.Fatalf("failure: %+v", got.Failure)
	}
	if got.Provider.Name != "deepseek" || got.Provider.BaseURL != "https://api.deepseek.com" {
		t.Errorf("trim: %+v", got.Provider)
	}
	if len(got.Provider.Models) != 2 || got.Provider.Models[0] != "a" || got.Provider.Models[1] != "b" {
		t.Errorf("models: %v", got.Provider.Models)
	}
	if got.Provider.DefaultModel != "a" {
		t.Errorf("default: %s", got.Provider.DefaultModel)
	}
	if got.Provider.ContextWindow != nil {
		t.Errorf("negative context window kept: %v", *got.Provider.ContextWindow)
	}
}

func TestCoerceProvidersLimits(t *testing.T) {
	list := make([]any, 0, 60)
	for i := 0; i < 60; i++ {
		list = append(list, map[string]any{"name": "p" + strconv.Itoa(i), "kind": "openai", "baseUrl": "https://x.test", "models": []any{"m"}})
	}
	got := CoerceProviders(map[string]any{"version": 1, "providers": list})
	if len(got) != 50 {
		t.Fatalf("kept %d, want 50", len(got))
	}
	if _, f := ReadProviders(t.TempDir()); f != nil || len(mustProviders(t, t.TempDir())) != 0 {
		t.Fatalf("missing file should be empty, got %+v", f)
	}

	dir := t.TempDir()
	raw, _ := json.Marshal(map[string]any{"version": 1, "providers": []any{
		map[string]any{"name": "Same", "kind": "openai", "baseUrl": "https://x.test", "models": []any{"m"}},
		map[string]any{"name": "same", "kind": "openai", "baseUrl": "https://y.test", "models": []any{"m"}},
	}})
	if err := os.MkdirAll(filepath.Dir(ProviderPath(dir)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ProviderPath(dir), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	got, f := ReadProviders(dir)
	if f != nil {
		t.Fatal(f)
	}
	// Read path keeps case variants: the legacy dedupe is case-sensitive on
	// read (ProviderStore.ts:66-75) and case-insensitive only on save.
	if len(got) != 2 {
		t.Fatalf("read should keep both case variants: %+v", got)
	}
}

func TestProviderFileBytes(t *testing.T) {
	dir := t.TempDir()
	p := Provider{Name: "deepseek", Kind: "openai", BaseURL: "https://api.deepseek.com", Models: []string{"deepseek-v4-flash", "a<b"}, DefaultModel: "deepseek-v4-flash", APIKeyEnv: "DEEPSEEK_API_KEY", ContextWindow: ptr(1000000)}
	if f := WriteProviders(dir, []Provider{p}); f != nil {
		t.Fatal(f)
	}
	body, err := os.ReadFile(ProviderPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n" +
		"  \"version\": 1,\n" +
		"  \"providers\": [\n" +
		"    {\n" +
		"      \"name\": \"deepseek\",\n" +
		"      \"kind\": \"openai\",\n" +
		"      \"baseUrl\": \"https://api.deepseek.com\",\n" +
		"      \"models\": [\n" +
		"        \"deepseek-v4-flash\",\n" +
		"        \"a<b\"\n" +
		"      ],\n" +
		"      \"defaultModel\": \"deepseek-v4-flash\",\n" +
		"      \"apiKeyEnv\": \"DEEPSEEK_API_KEY\",\n" +
		"      \"contextWindow\": 1000000\n" +
		"    }\n" +
		"  ]\n" +
		"}\n"
	if string(body) != want {
		t.Fatalf("bytes:\n%s\nwant:\n%s", body, want)
	}
}

func TestLegacyMigration(t *testing.T) {
	dir := t.TempDir()
	toml := "# >>> mile-knowledge-base managed providers 由应用维护\n" +
		"[[providers]]\n" +
		"name = \"legacy\"\n" +
		"kind = \"openai\"\n" +
		"base_url = \"https://legacy.test\"\n" +
		"models = [\"m1\", \"m2\"]\n" +
		"default = \"m2\"\n" +
		"api_key_env = \"\"\n" +
		"context_window = 32000\n" +
		"# <<< mile-knowledge-base managed providers\n"
	if err := os.WriteFile(filepath.Join(dir, "reasonix.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	got, f := ReadProviders(dir)
	if f != nil {
		t.Fatal(f)
	}
	if len(got) != 1 || got[0].Name != "legacy" || got[0].DefaultModel != "m2" || got[0].ContextWindow == nil || *got[0].ContextWindow != 32000 {
		t.Fatalf("migrated: %+v", got)
	}
	if _, err := os.Stat(ProviderPath(dir)); err != nil {
		t.Fatalf("migration should write providers.json: %v", err)
	}

	// An existing providers.json wins over reasonix.toml.
	if f := WriteProviders(dir, []Provider{{Name: "new", Kind: "openai", BaseURL: "https://n.test", Models: []string{"m"}, DefaultModel: "m"}}); f != nil {
		t.Fatal(f)
	}
	got, _ = ReadProviders(dir)
	if len(got) != 1 || got[0].Name != "new" {
		t.Fatalf("providers.json should win: %+v", got)
	}

	// An empty managed region must not write a file (otherwise migration stops).
	empty := t.TempDir()
	if err := os.WriteFile(filepath.Join(empty, "reasonix.toml"), []byte("# >>> mile-knowledge-base managed providers\n# <<< mile-knowledge-base managed providers\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, f := ReadProviders(empty); f != nil {
		t.Fatal(f)
	}
	if _, err := os.Stat(ProviderPath(empty)); !os.IsNotExist(err) {
		t.Fatalf("empty migration wrote a file")
	}
}

func TestDeepSeekPreset(t *testing.T) {
	p := DeepSeekPreset()
	if p.Name != "deepseek" || p.BaseURL != "https://api.deepseek.com" || p.APIKeyEnv != "DEEPSEEK_API_KEY" || p.DefaultModel != "deepseek-v4-flash" || p.ContextWindow == nil || *p.ContextWindow != 1000000 {
		t.Fatalf("preset: %+v", p)
	}
}

func manyModels(n int) []any {
	out := make([]any, n)
	for i := range out {
		out[i] = "m" + strconv.Itoa(i)
	}
	return out
}

func mustProviders(t *testing.T, dir string) []Provider {
	t.Helper()
	list, f := ReadProviders(dir)
	if f != nil {
		t.Fatalf("read %s: %+v", dir, f)
	}
	return list
}

func TestSanitizeProviders(t *testing.T) {
	base := func(name string) Provider {
		return Provider{Name: name, Kind: "openai", BaseURL: "https://x.test", Models: []string{"m"}, DefaultModel: "m"}
	}
	list := make([]Provider, 0, 33)
	for i := 0; i < 33; i++ {
		list = append(list, base("p"+strconv.Itoa(i)))
	}
	if _, f := SanitizeProviders(list); f == nil || f.Code != contract.CodeInvalidInput {
		t.Fatalf("33 providers: %+v", f)
	}
	if _, f := SanitizeProviders([]Provider{base("Same"), base("same")}); f == nil || f.Code != contract.CodeInvalidInput {
		t.Fatalf("duplicate names on save: %+v", f)
	}
	if _, f := SanitizeProviders([]Provider{base("ok"), {Name: "-bad", Kind: "openai", BaseURL: "https://x.test", Models: []string{"m"}}}); f == nil {
		t.Fatalf("invalid entry should fail the whole save")
	}
	clean, f := SanitizeProviders([]Provider{base("ok")})
	if f != nil || len(clean) != 1 || clean[0].DefaultModel != "m" {
		t.Fatalf("valid save: %+v %+v", clean, f)
	}
}

func TestProviderUnknownFieldsSurviveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(ProviderPath(dir)), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := `{"version":1,"providers":[{"name":"x","kind":"openai","baseUrl":"https://x.test","models":["m"],"defaultModel":"m","apiKeyEnv":"","zeta":"keep","alpha":{"nested":true}}]}`
	if err := os.WriteFile(ProviderPath(dir), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	got, f := ReadProviders(dir)
	if f != nil || len(got) != 1 {
		t.Fatalf("read: %+v %+v", got, f)
	}
	if len(got[0].Extra) != 2 {
		t.Fatalf("extra fields: %+v", got[0].Extra)
	}
	if f := WriteProviders(dir, got); f != nil {
		t.Fatal(f)
	}
	body, _ := os.ReadFile(ProviderPath(dir))
	if !strings.Contains(string(body), `"zeta": "keep"`) || !strings.Contains(string(body), `"alpha": {`) {
		t.Fatalf("unknown fields lost:\n%s", body)
	}
	if strings.Index(string(body), `"alpha"`) > strings.Index(string(body), `"zeta"`) {
		t.Fatalf("extra fields should be sorted:\n%s", body)
	}
}
