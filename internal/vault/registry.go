// Package vault owns the registered knowledge bases: identity, the canonical
// root, the directory scaffold and the lazily loaded tree.
package vault

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/pathguard"
)

// Prefs persists the registered vault list (implemented by settings.Store).
type Prefs interface {
	RecentVaults() []contract.Vault
	LastVaultID() string
	SaveVaults(vaults []contract.Vault, activeID string) *contract.Failure
}

// ID derives the vault identity exactly like the legacy app: the lowercase
// canonical path hashed with SHA-256, first 16 hex characters. Imported recent
// documents key on this value, so it must not change.
func ID(canonicalPath string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(canonicalPath)))
	return hex.EncodeToString(sum[:])[:16]
}

// Registry is the in-memory vault list. It is safe for concurrent use.
type Registry struct {
	prefs Prefs

	mu          sync.RWMutex
	vaults      []contract.Vault
	unavailable []contract.UnavailableVault
	active      string
	roots       map[string]string
	aliases     map[string]map[string]string
}

// New creates an empty registry.
func New(prefs Prefs) *Registry {
	return &Registry{prefs: prefs, roots: map[string]string{}, aliases: map[string]map[string]string{}}
}

// Load registers every persisted vault, moving the ones whose directory is gone
// or unreadable into the unavailable list (legacy VaultService.restore).
func (r *Registry) Load() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.vaults = nil
	r.unavailable = nil
	r.roots = map[string]string{}

	var stored []contract.Vault
	if r.prefs != nil {
		stored = r.prefs.RecentVaults()
	}
	seen := map[string]bool{}
	for _, v := range stored {
		canon, err := pathguard.Canon(v.Root)
		if err != nil {
			r.unavailable = append(r.unavailable, contract.UnavailableVault{
				ID: v.ID, Root: v.Root, DisplayName: v.DisplayName, Reason: reasonOf(err),
			})
			continue
		}
		info, statErr := os.Stat(canon)
		if statErr != nil || !info.IsDir() {
			reason := "missing"
			if statErr != nil && errors.Is(statErr, fs.ErrPermission) {
				reason = "no-permission"
			}
			r.unavailable = append(r.unavailable, contract.UnavailableVault{
				ID: v.ID, Root: v.Root, DisplayName: v.DisplayName, Reason: reason,
			})
			continue
		}
		id := ID(canon)
		if seen[id] {
			continue
		}
		seen[id] = true
		v.ID = id
		v.Root = canon
		if v.DisplayName == "" {
			v.DisplayName = filepath.Base(canon)
		}
		r.vaults = append(r.vaults, v)
		r.roots[id] = canon
	}
	// Reopen the vault that was active last time, if it is still usable.
	if r.prefs != nil {
		last := r.prefs.LastVaultID()
		if last != "" {
			if _, ok := r.roots[last]; ok {
				r.active = last
			}
		}
	}
	if r.active != "" && r.roots[r.active] == "" {
		r.active = ""
	}
}

func reasonOf(err error) string {
	if errors.Is(err, fs.ErrPermission) {
		return "no-permission"
	}
	return "missing"
}

// List returns the current view for the UI.
func (r *Registry) List() contract.VaultListResult {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return contract.VaultListResult{
		Vaults:        append([]contract.Vault{}, r.vaults...),
		ActiveVaultID: r.active,
		Unavailable:   append([]contract.UnavailableVault{}, r.unavailable...),
	}
}

// Register adds (or refreshes) a vault rooted at root and makes it active.
func (r *Registry) Register(root string) (contract.Vault, *contract.Failure) {
	canon, err := pathguard.Canon(root)
	if err != nil {
		return contract.Vault{}, contract.Fail(contract.CodeVaultUnavailable, "目录不可用")
	}
	info, statErr := os.Stat(canon)
	if statErr != nil || !info.IsDir() {
		return contract.Vault{}, contract.Fail(contract.CodeVaultUnavailable, "目录不可用")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	id := ID(canon)
	now := time.Now().UTC().Format(time.RFC3339)
	found := false
	for i := range r.vaults {
		if r.vaults[i].ID == id {
			r.vaults[i].Root = canon
			r.vaults[i].LastOpenedAt = now
			found = true
			break
		}
	}
	var v contract.Vault
	if !found {
		v = contract.Vault{
			ID: id, Root: canon, DisplayName: filepath.Base(canon),
			CreatedAt: now, LastOpenedAt: now,
		}
		r.vaults = append([]contract.Vault{v}, r.vaults...)
	} else {
		for _, item := range r.vaults {
			if item.ID == id {
				v = item
			}
		}
	}
	// Drop it from unavailable if it came back.
	filtered := r.unavailable[:0]
	for _, u := range r.unavailable {
		if u.ID != id {
			filtered = append(filtered, u)
		}
	}
	r.unavailable = filtered
	r.roots[id] = canon
	r.active = id
	r.persistLocked()
	return v, nil
}

// Open makes an existing vault active (M08).
func (r *Registry) Open(id string) *contract.Failure {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.vaults {
		if r.vaults[i].ID == id {
			r.vaults[i].LastOpenedAt = time.Now().UTC().Format(time.RFC3339)
			r.active = id
			r.persistLocked()
			return nil
		}
	}
	for _, u := range r.unavailable {
		if u.ID == id {
			return contract.Fail(contract.CodeVaultUnavailable, "知识库不可用")
		}
	}
	return contract.Fail(contract.CodeNotFound, "知识库未登记")
}

// Forget removes a vault from the list (M10). Recent documents are cleared by
// the caller; the recent-document store is owned by settings.
func (r *Registry) Forget(id string) *contract.Failure {
	r.mu.Lock()
	defer r.mu.Unlock()
	kept := r.vaults[:0]
	for _, v := range r.vaults {
		if v.ID != id {
			kept = append(kept, v)
		}
	}
	r.vaults = kept
	un := r.unavailable[:0]
	for _, u := range r.unavailable {
		if u.ID != id {
			un = append(un, u)
		}
	}
	r.unavailable = un
	delete(r.roots, id)
	delete(r.aliases, id)
	if r.active == id {
		r.active = ""
	}
	r.persistLocked()
	return nil
}

// Retry re-checks an unavailable vault (M11). It stays unavailable when the
// directory is still unusable; that is not an error for the caller.
func (r *Registry) Retry(id string) {
	r.mu.Lock()
	var root string
	var found bool
	for _, u := range r.unavailable {
		if u.ID == id {
			root, found = u.Root, true
			break
		}
	}
	r.mu.Unlock()
	if !found {
		return
	}
	if _, f := r.Register(root); f != nil {
		return
	}
}

// Active returns the active vault id ("" when none).
func (r *Registry) Active() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.active
}

// Root returns the canonical root for a vault id.
func (r *Registry) Root(id string) (string, *contract.Failure) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	root, ok := r.roots[id]
	if !ok {
		return "", contract.Fail(contract.CodeVaultUnavailable, "知识库不可用")
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return "", contract.Fail(contract.CodeVaultUnavailable, "知识库不可用")
	}
	return root, nil
}

// Vault returns a registered vault by id.
func (r *Registry) Vault(id string) (contract.Vault, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, v := range r.vaults {
		if v.ID == id {
			return v, true
		}
	}
	return contract.Vault{}, false
}

// Aliases returns the directory alias map (path -> alias) for a vault.
func (r *Registry) Aliases(id string) map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]string, len(r.aliases))
	for k, v := range r.aliases[id] {
		out[k] = v
	}
	return out
}

// SetAliases replaces the alias map for a vault (loaded from .mile-kb).
func (r *Registry) SetAliases(id string, aliases map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.aliases[id] = aliases
}

func (r *Registry) persistLocked() {
	if r.prefs == nil {
		return
	}
	list := make([]contract.Vault, len(r.vaults))
	copy(list, r.vaults)
	r.prefs.SaveVaults(list, r.active)
}
