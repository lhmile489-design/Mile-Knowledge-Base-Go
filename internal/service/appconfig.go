package service

import (
	"context"
	"os"
	"path/filepath"
	"sort"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/vault"
)

// AppConfigGet returns the vault-level settings the UI reads (M62): display
// aliases and the raw categories file (the settings page consumes it in M3).
func (s *SettingsService) AppConfigGet(ctx context.Context, vaultID string) contract.AppConfigResult {
	result := contract.AppConfigResult{Aliases: map[string]string{}}
	root, f := s.vaultRoot(vaultID, &result.Failure)
	if f != nil {
		return result
	}
	aliases := vault.ReadAliases(vault.AliasPath(root))
	result.Aliases = aliases.Map()
	if raw, err := os.ReadFile(filepath.Join(root, ".mile-kb", "settings", "categories.json")); err == nil {
		result.Categories = raw
	}
	return result
}

// AppConfigPatchAliases applies alias edits (M66). A nil or empty value deletes
// the entry; the file keeps byte compatibility with the legacy writer.
func (s *SettingsService) AppConfigPatchAliases(ctx context.Context, vaultID string, patches map[string]*string) contract.AppConfigResult {
	result := contract.AppConfigResult{Aliases: map[string]string{}}
	root, f := s.vaultRoot(vaultID, &result.Failure)
	if f != nil {
		return result
	}
	file := vault.ReadAliases(vault.AliasPath(root))
	keys := make([]string, 0, len(patches))
	for key := range patches {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if fail := file.Set(key, patches[key]); fail != nil {
			result.Failure = fail
			return result
		}
	}
	if err := file.Save(); err != nil {
		result.Failure = contract.Fail(contract.CodeIOFailure, "写入别名失败")
		return result
	}
	result.Aliases = file.Map()
	s.app.Vaults.SetAliases(vaultID, result.Aliases)
	return result
}

func (s *SettingsService) vaultRoot(vaultID string, failure **contract.Failure) (string, *contract.Failure) {
	if f := validateID(vaultID); f != nil {
		*failure = f
		return "", f
	}
	root, f := s.app.Vaults.Root(vaultID)
	if f != nil {
		*failure = f
		return "", f
	}
	return root, nil
}
