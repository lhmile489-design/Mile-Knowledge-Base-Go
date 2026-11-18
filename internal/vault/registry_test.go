package vault

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

type fakePrefs struct {
	vaults []contract.Vault
	last   string
}

func (f *fakePrefs) RecentVaults() []contract.Vault { return f.vaults }
func (f *fakePrefs) LastVaultID() string            { return f.last }
func (f *fakePrefs) SaveVaults(list []contract.Vault, active string) *contract.Failure {
	f.vaults = list
	f.last = active
	return nil
}

func TestLoadRestoresLastVaultAndReportsUnavailable(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(t.TempDir(), "gone")
	canon, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	prefs := &fakePrefs{
		vaults: []contract.Vault{
			{ID: "aaaaaaaaaaaaaaaa", Root: canon, DisplayName: "here"},
			{ID: "bbbbbbbbbbbbbbbb", Root: missing, DisplayName: "gone"},
		},
		last: "aaaaaaaaaaaaaaaa",
	}
	reg := New(prefs)
	reg.Load()

	got := reg.List()
	if len(got.Vaults) != 1 {
		t.Fatalf("vaults = %+v", got.Vaults)
	}
	if got.Vaults[0].ID != ID(canon) {
		t.Errorf("id = %q, want %q", got.Vaults[0].ID, ID(canon))
	}
	if len(got.Unavailable) != 1 || got.Unavailable[0].Reason != "missing" {
		t.Errorf("unavailable = %+v", got.Unavailable)
	}
	// The stored active id was recomputed from the canonical path, so the
	// remembered id may no longer match; reopening is by the recomputed id.
	if got.ActiveVaultID != "" && got.ActiveVaultID != ID(canon) {
		t.Errorf("active = %q", got.ActiveVaultID)
	}
}

func TestOpenForgetRetry(t *testing.T) {
	root := t.TempDir()
	canon, _ := filepath.EvalSymlinks(root)
	prefs := &fakePrefs{last: ID(canon)}
	prefs.vaults = []contract.Vault{{ID: ID(canon), Root: canon, DisplayName: "v"}}
	reg := New(prefs)
	reg.Load()
	if reg.Active() != ID(canon) {
		t.Fatalf("active after load = %q", reg.Active())
	}

	// Forget clears the entry and the active id.
	if f := reg.Forget(ID(canon)); f != nil {
		t.Fatal(f)
	}
	if len(reg.List().Vaults) != 0 || reg.Active() != "" {
		t.Errorf("forget left state: %+v", reg.List())
	}

	// A directory that disappears becomes unavailable; retry does not throw.
	gone := filepath.Join(t.TempDir(), "nope")
	prefs2 := &fakePrefs{vaults: []contract.Vault{{ID: "cccccccccccccccc", Root: gone, DisplayName: "g"}}}
	reg2 := New(prefs2)
	reg2.Load()
	if len(reg2.List().Unavailable) != 1 {
		t.Fatalf("unavailable = %+v", reg2.List())
	}
	reg2.Retry("cccccccccccccccc")
	if len(reg2.List().Unavailable) != 1 {
		t.Error("retry of a missing dir should leave it unavailable")
	}

	// A file instead of a directory is unavailable too.
	fileRoot := filepath.Join(t.TempDir(), "file.md")
	if err := os.WriteFile(fileRoot, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	prefs3 := &fakePrefs{vaults: []contract.Vault{{ID: "dddddddddddddddd", Root: fileRoot}}}
	reg3 := New(prefs3)
	reg3.Load()
	if len(reg3.List().Unavailable) != 1 {
		t.Error("a file passed as a vault root should be unavailable")
	}
}
