package service

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

func TestAboutInfoRedactsEveryPath(t *testing.T) {
	app, _, root := mutationTestApp(t)
	app.Version = "1.0.0"
	svc := NewSettingsService(app)
	got := svc.AboutInfo(context.Background())
	if got.Failure != nil {
		t.Fatalf("failure = %+v", got.Failure)
	}
	if got.Version != "1.0.0" {
		t.Errorf("version = %q", got.Version)
	}
	if got.TechStack == "" || !strings.Contains(got.TechStack, "React") {
		t.Errorf("techStack = %q", got.TechStack)
	}
	if got.Capabilities == nil {
		t.Error("capabilities missing")
	}

	kinds := map[string]contract.StorageLocation{}
	for _, item := range got.Storage {
		kinds[item.Kind] = item
	}
	for _, kind := range []string{"vault-root", "vault-settings", "providers", "app-data", "logs", "credentials"} {
		if _, ok := kinds[kind]; !ok {
			t.Errorf("missing storage kind %q: %+v", kind, got.Storage)
		}
	}
	home, _ := os.UserHomeDir()
	for _, item := range got.Storage {
		if strings.Contains(item.Display, root) {
			t.Errorf("%s leaks the vault path: %q", item.Kind, item.Display)
		}
		if home != "" && strings.Contains(strings.ToLower(item.Display), strings.ToLower(home)) {
			t.Errorf("%s leaks the home path: %q", item.Kind, item.Display)
		}
	}
	if kinds["vault-root"].Display != "<vault>" || !kinds["vault-root"].Copyable {
		t.Errorf("vault-root = %+v", kinds["vault-root"])
	}
	if kinds["credentials"].Display != "Windows 凭据管理器" || kinds["credentials"].Copyable {
		t.Errorf("credentials = %+v", kinds["credentials"])
	}
}

func TestCopyStoragePathWritesRealPath(t *testing.T) {
	app, _, root := mutationTestApp(t)
	var copied []string
	original := writeClipboard
	writeClipboard = func(text string) error {
		copied = append(copied, text)
		return nil
	}
	t.Cleanup(func() { writeClipboard = original })
	svc := NewSettingsService(app)
	ctx := context.Background()

	if got := svc.CopyStoragePath(ctx, "vault-root"); got.Failure != nil || !got.OK {
		t.Fatalf("vault-root: %+v", got)
	}
	if len(copied) != 1 || copied[0] != root {
		t.Fatalf("copied = %v, want %q", copied, root)
	}

	if got := svc.CopyStoragePath(ctx, "providers"); got.Failure != nil || !got.OK {
		t.Fatalf("providers: %+v", got)
	}
	if !strings.HasSuffix(copied[1], ".mile-kb/settings/providers.json") &&
		!strings.HasSuffix(copied[1], `.mile-kb\settings\providers.json`) {
		t.Errorf("providers path = %q", copied[1])
	}

	if got := svc.CopyStoragePath(ctx, "credentials"); got.Failure == nil || got.Failure.Code != contract.CodeUnsupported {
		t.Fatalf("credentials = %+v", got.Failure)
	}
	if got := svc.CopyStoragePath(ctx, "bogus"); got.Failure == nil || got.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("bogus = %+v", got.Failure)
	}
	if len(copied) != 2 {
		t.Errorf("rejected kinds wrote to the clipboard: %v", copied)
	}
}

func TestAboutInfoWithoutVaultIsOffline(t *testing.T) {
	app, err := New(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	got := NewSettingsService(app).AboutInfo(context.Background())
	if got.Failure != nil {
		t.Fatalf("failure = %+v", got.Failure)
	}
	if got.Engine.Phase != "offline" {
		t.Errorf("engine = %+v", got.Engine)
	}
	if len(got.Storage) != 6 {
		t.Errorf("storage = %+v", got.Storage)
	}
}
