package ai

import (
	"errors"
	"strings"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

func TestMemoryKeys(t *testing.T) {
	store := NewMemoryKeyStore()
	if f := store.Set("DEEPSEEK_API_KEY", "sk-a"); f != nil {
		t.Fatal(f)
	}
	if !store.Has("DEEPSEEK_API_KEY") {
		t.Error("has after set")
	}
	if v, f := store.Get("DEEPSEEK_API_KEY"); f != nil || v != "sk-a" {
		t.Fatalf("get: %q %+v", v, f)
	}
	if f := store.Delete("DEEPSEEK_API_KEY"); f != nil {
		t.Fatal(f)
	}
	if store.Has("DEEPSEEK_API_KEY") {
		t.Error("still present after delete")
	}
	if _, f := store.Get("DEEPSEEK_API_KEY"); f == nil || f.Code != contract.CodeNotFound {
		t.Fatalf("get missing: %+v", f)
	}
}

func TestKeyValidation(t *testing.T) {
	store := NewMemoryKeyStore()
	cases := []struct {
		name  string
		env   string
		value string
		ok    bool
	}{
		{"valid", "API_KEY", "sk-x", true},
		{"underscore start", "_KEY", "x", true},
		{"empty env", "", "x", false},
		{"bad env", "1KEY", "x", false},
		{"model name env", "deepseek-v4-flash", "x", false},
		{"empty value", "API_KEY", "", false},
		{"newline", "API_KEY", "a\nb", false},
		{"carriage return", "API_KEY", "a\rb", false},
		{"too long", "API_KEY", strings.Repeat("x", MaxKeyBytes+1), false},
		{"max size", "API_KEY", strings.Repeat("x", MaxKeyBytes), true},
	}
	for _, c := range cases {
		f := store.Set(c.env, c.value)
		if (f == nil) != c.ok {
			t.Errorf("%s: failure=%+v", c.name, f)
		}
		if f != nil && f.Code != contract.CodeInvalidInput && f.Code != contract.CodeTooLarge {
			t.Errorf("%s: code=%s", c.name, f.Code)
		}
	}
}

// failingBackend simulates a machine without OS secret storage.
type failingBackend struct{}

func (failingBackend) Set(string, string) error { return errors.ErrUnsupported }
func (failingBackend) Get(string) (string, error) {
	return "", errors.ErrUnsupported
}
func (failingBackend) Delete(string) error { return errors.ErrUnsupported }

func TestKeyStoreUnavailable(t *testing.T) {
	store := NewKeyStore(failingBackend{})
	if f := store.Set("API_KEY", "x"); f == nil || f.Code != contract.CodeSecretUnavailable {
		t.Fatalf("set: %+v", f)
	}
	if _, f := store.Get("API_KEY"); f == nil || f.Code != contract.CodeSecretUnavailable {
		t.Fatalf("get: %+v", f)
	}
}

// TestSecretsBackendReal uses a throwaway service name and removes it. It never
// touches the application's own secrets.
func TestSecretsBackendReal(t *testing.T) {
	backend := NewSecretsBackend("mkb-test-" + randomHex(8))
	t.Cleanup(func() { _ = backend.Delete("probe") })
	store := NewKeyStore(backend)
	if f := store.Set("probe", "hello"); f != nil {
		t.Skipf("OS secret storage unavailable here: %+v", f)
	}
	if v, f := store.Get("probe"); f != nil || v != "hello" {
		t.Fatalf("get: %q %+v", v, f)
	}
	if !store.Has("probe") {
		t.Error("has")
	}
	if f := store.Delete("probe"); f != nil {
		t.Fatal(f)
	}
}
