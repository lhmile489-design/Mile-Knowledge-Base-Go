package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/ai"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

func draft(name string) contract.ProviderDraft {
	return contract.ProviderDraft{
		Name: name, Kind: "openai", BaseURL: "https://api.deepseek.com",
		Models: []string{"deepseek-v4-flash", "deepseek-v4-pro"}, DefaultModel: "deepseek-v4-flash",
		APIKeyEnv: "DEEPSEEK_API_KEY",
	}
}

func TestProvidersEmptyAndSave(t *testing.T) {
	app, vaultID, root := mutationTestApp(t)
	svc := NewSettingsService(app)
	ctx := context.Background()

	empty := svc.Providers(ctx, vaultID)
	if empty.Failure != nil || len(empty.Providers) != 0 {
		t.Fatalf("empty vault: %+v", empty)
	}

	saved := svc.SaveProviders(ctx, vaultID, []contract.ProviderDraft{draft("deepseek")})
	if saved.Failure != nil || len(saved.Providers) != 1 {
		t.Fatalf("save: %+v", saved)
	}
	if _, err := os.Stat(ai.ProviderPath(root)); err != nil {
		t.Fatalf("file missing: %v", err)
	}
	if len(saved.Credentials) != 1 || saved.Credentials[0].IsSet {
		t.Fatalf("credentials before set: %+v", saved.Credentials)
	}

	listed := svc.Providers(ctx, vaultID)
	if len(listed.Providers) != 1 || listed.Providers[0].Name != "deepseek" {
		t.Fatalf("listed: %+v", listed)
	}

	tooMany := make([]contract.ProviderDraft, 33)
	for i := range tooMany {
		tooMany[i] = draft("p" + string(rune('a'+i%26)) + string(rune('a'+i/26)))
	}
	if f := svc.SaveProviders(ctx, vaultID, tooMany); f.Failure == nil || f.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("33 providers: %+v", f)
	}

	dup := svc.SaveProviders(ctx, vaultID, []contract.ProviderDraft{draft("Same"), draft("same")})
	if dup.Failure == nil || dup.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("duplicate: %+v", dup)
	}

	bad := []contract.ProviderDraft{draft("ok"), {Name: "-bad", Kind: "openai", BaseURL: "https://x.test", Models: []string{"m"}}}
	before, _ := os.ReadFile(ai.ProviderPath(root))
	if f := svc.SaveProviders(ctx, vaultID, bad); f.Failure == nil {
		t.Fatalf("invalid entry accepted")
	}
	after, _ := os.ReadFile(ai.ProviderPath(root))
	if string(before) != string(after) {
		t.Fatalf("failed save must not write")
	}
}

func TestSetKey(t *testing.T) {
	app, vaultID, _ := mutationTestApp(t)
	svc := NewSettingsService(app)
	ctx := context.Background()
	if f := svc.SaveProviders(ctx, vaultID, []contract.ProviderDraft{draft("deepseek")}); f.Failure != nil {
		t.Fatal(f.Failure)
	}

	got := svc.SetKey(ctx, "DEEPSEEK_API_KEY", "sk-test-value")
	if got.Failure != nil {
		t.Fatal(got.Failure)
	}
	if len(got.Credentials) != 1 || !got.Credentials[0].IsSet {
		t.Fatalf("credentials: %+v", got.Credentials)
	}
	if !app.Keys.Has("DEEPSEEK_API_KEY") {
		t.Fatal("key not stored")
	}

	if bad := svc.SetKey(ctx, "not a name", "x"); bad.Failure == nil || bad.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("bad env name: %+v", bad)
	}

	// Without an active vault the result only reports the touched credential.
	fresh, err := New(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fresh.Keys = ai.NewMemoryKeyStore()
	only := NewSettingsService(fresh).SetKey(ctx, "API_KEY", "x")
	if only.Failure != nil || len(only.Providers) != 0 || len(only.Credentials) != 1 {
		t.Fatalf("no active vault: %+v", only)
	}
}

func TestFetchModelsUsesStoredKey(t *testing.T) {
	var seenAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"data":[{"id":"m1"}]}`))
	}))
	defer server.Close()

	app, _, _ := mutationTestApp(t)
	svc := NewSettingsService(app)
	ctx := context.Background()
	if f := app.Keys.Set("DEEPSEEK_API_KEY", "sk-stored"); f != nil {
		t.Fatal(f)
	}

	got := svc.FetchModels(ctx, "openai", server.URL, "DEEPSEEK_API_KEY", "")
	if got.Failure != nil || len(got.Models) != 1 {
		t.Fatalf("fetch: %+v", got)
	}
	if seenAuth != "Bearer sk-stored" {
		t.Fatalf("auth header = %q", seenAuth)
	}

	draftKey := svc.FetchModels(ctx, "openai", server.URL, "DEEPSEEK_API_KEY", "sk-draft")
	if draftKey.Failure != nil || seenAuth != "Bearer sk-draft" {
		t.Fatalf("draft key should win: %q %+v", seenAuth, draftKey.Failure)
	}

	denied := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`bad`))
	}))
	defer denied.Close()
	if f := svc.FetchModels(ctx, "openai", denied.URL, "", ""); f.Failure == nil || f.Failure.Code != contract.CodePermissionDenied {
		t.Fatalf("401: %+v", f)
	}
}

func TestFetchModelsRejectsMalformedKey(t *testing.T) {
	app, _, _ := mutationTestApp(t)
	svc := NewSettingsService(app)
	if r := svc.FetchModels(context.Background(), "openai", "https://x.test", "", "a\nb"); r.Failure == nil || r.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("newline key: %+v", r)
	}
	if r := svc.FetchModels(context.Background(), "openai", "https://x.test", "", strings.Repeat("a", 4097)); r.Failure == nil || r.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("oversized key: %+v", r)
	}
}
