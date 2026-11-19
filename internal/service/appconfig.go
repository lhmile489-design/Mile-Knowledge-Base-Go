package service

import (
	"context"
	"sort"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/ai"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/vault"
)

// AppConfigGet returns the vault-level settings the UI reads (M62): display
// aliases, the parsed category table and the four agents.
func (s *SettingsService) AppConfigGet(ctx context.Context, vaultID string) contract.AppConfigResult {
	result := contract.AppConfigResult{Aliases: map[string]string{}, Categories: []contract.CategoryConfig{}}
	root, f := s.vaultRoot(vaultID, &result.Failure)
	if f != nil {
		return result
	}
	aliases := vault.ReadAliases(vault.AliasPath(root))
	result.Aliases = aliases.Map()
	result.Categories = toContractCategories(vault.ReadCategories(root))
	agents, _ := ai.ReadAgents(root)
	result.Agents = toContractAgents(agents)
	return result
}

// AppConfigSetCategories writes the category table (M65). Invalid rows are
// dropped and rows equal to the defaults are not persisted, matching the
// legacy writer (数据库设计 3.2, D-013).
func (s *SettingsService) AppConfigSetCategories(ctx context.Context, vaultID string, categories []contract.CategoryConfig) contract.AppConfigResult {
	result := s.appConfigResult(ctx, vaultID)
	if result.Failure != nil {
		return result
	}
	root := s.vaultRootOrEmpty(vaultID)
	list := make([]vault.Category, 0, len(categories))
	for _, item := range categories {
		list = append(list, vault.Category{
			Directory: item.Directory, Alias: item.Alias,
			Area: item.Area, Indexed: item.Indexed, Order: item.Order,
		})
	}
	if err := vault.WriteCategories(root, list); err != nil {
		result.Failure = contract.Fail(contract.CodeIOFailure, "写入分类设置失败")
		return result
	}
	result.Categories = toContractCategories(vault.ReadCategories(root))
	return result
}

func toContractCategories(list []vault.Category) []contract.CategoryConfig {
	out := make([]contract.CategoryConfig, 0, len(list))
	for _, item := range list {
		out = append(out, contract.CategoryConfig{
			Directory: item.Directory, Alias: item.Alias,
			Area: item.Area, Indexed: item.Indexed, Order: item.Order,
		})
	}
	return out
}

// AppConfigPatchAliases applies alias edits (M66). A nil or empty value deletes
// the entry; the file keeps byte compatibility with the legacy writer.
func (s *SettingsService) AppConfigPatchAliases(ctx context.Context, vaultID string, patches map[string]*string) contract.AppConfigResult {
	result := contract.AppConfigResult{Aliases: map[string]string{}, Categories: []contract.CategoryConfig{}}
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
	result.Categories = toContractCategories(vault.ReadCategories(root))
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
