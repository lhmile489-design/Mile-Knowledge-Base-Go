package service

import (
	"net/http/httptest"
	"testing"
)

func TestVaultRootRequiresExplicitVault(t *testing.T) {
	app, vaultID, _ := mutationTestApp(t)

	// No vault parameter: refused even though a vault is registered.
	req := httptest.NewRequest("GET", "http://kbimg.localhost/raws/x.png", nil)
	if _, _, ok := vaultRootForRequest(app, req); ok {
		t.Fatal("missing vault parameter accepted")
	}

	// Unknown vault id: refused.
	req = httptest.NewRequest("GET", "http://kbimg.localhost/raws/x.png?vault=nope", nil)
	if _, _, ok := vaultRootForRequest(app, req); ok {
		t.Fatal("unknown vault accepted")
	}

	// The registered vault works.
	req = httptest.NewRequest("GET", "http://kbimg.localhost/raws/x.png?vault="+vaultID, nil)
	if root, id, ok := vaultRootForRequest(app, req); !ok || root == "" || id != vaultID {
		t.Fatalf("registered vault rejected: %v %v", id, ok)
	}
}
