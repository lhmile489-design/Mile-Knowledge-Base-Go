package service

import (
	"context"
	"strings"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/ai"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// Providers returns the vault's provider definitions plus whether each key is
// configured (M67). The key value never leaves the secret store.
func (s *SettingsService) Providers(ctx context.Context, vaultID string) contract.ProvidersResult {
	result := contract.ProvidersResult{}
	root, f := s.vaultRoot(vaultID, &result.Failure)
	if f != nil {
		return result
	}
	providers, f := ai.ReadProviders(root)
	if f != nil {
		result.Failure = f
		return result
	}
	result.Providers = toProviderDrafts(providers)
	result.Credentials = s.credentialStates(providers)
	return result
}

// SaveProviders normalizes and validates the whole list, then writes it
// atomically (M68). One invalid entry fails the save; nothing is written.
func (s *SettingsService) SaveProviders(ctx context.Context, vaultID string, drafts []contract.ProviderDraft) contract.ProvidersResult {
	result := contract.ProvidersResult{}
	root, f := s.vaultRoot(vaultID, &result.Failure)
	if f != nil {
		return result
	}
	clean, f := ai.SanitizeProviders(fromProviderDrafts(drafts))
	if f != nil {
		result.Failure = f
		return result
	}
	if f := ai.WriteProviders(root, clean); f != nil {
		result.Failure = f
		return result
	}
	result.Providers = toProviderDrafts(clean)
	result.Credentials = s.credentialStates(clean)
	return result
}

// SetKey stores one API key (M69). Like the legacy handler it refreshes the
// snapshot for the active vault; without an active vault it only reports the
// one credential it touched.
func (s *SettingsService) SetKey(ctx context.Context, envName, value string) contract.ProvidersResult {
	result := contract.ProvidersResult{}
	if f := s.app.Keys.Set(envName, value); f != nil {
		result.Failure = f
		return result
	}
	active := s.app.Vaults.Active()
	if active == "" {
		result.Credentials = []contract.ProviderCredentialState{{EnvName: envName, IsSet: true}}
		return result
	}
	if root, f := s.app.Vaults.Root(active); f == nil {
		providers, _ := ai.ReadProviders(root)
		result.Providers = toProviderDrafts(providers)
		result.Credentials = s.credentialStates(providers)
		return result
	}
	result.Credentials = []contract.ProviderCredentialState{{EnvName: envName, IsSet: true}}
	return result
}

// FetchModels asks a provider for its model list (M70). A draft key wins over
// the stored one, matching the legacy handler.
func (s *SettingsService) FetchModels(ctx context.Context, kind, baseURL, apiKeyEnv, apiKey string) contract.FetchModelsResult {
	result := contract.FetchModelsResult{}
	key := strings.TrimSpace(apiKey)
	if key == "" && apiKeyEnv != "" {
		if stored, f := s.app.Keys.Get(apiKeyEnv); f == nil {
			key = stored
		}
	}
	models, f := ai.FetchModels(ctx, ai.FetchModelsInput{Kind: kind, BaseURL: baseURL, APIKey: key})
	if f != nil {
		result.Failure = f
		return result
	}
	result.Models = models
	return result
}

func (s *SettingsService) credentialStates(providers []ai.Provider) []contract.ProviderCredentialState {
	out := make([]contract.ProviderCredentialState, 0, len(providers))
	for _, provider := range providers {
		out = append(out, contract.ProviderCredentialState{
			EnvName: provider.APIKeyEnv,
			IsSet:   provider.APIKeyEnv != "" && s.app.Keys.Has(provider.APIKeyEnv),
		})
	}
	return out
}

func toProviderDrafts(providers []ai.Provider) []contract.ProviderDraft {
	out := make([]contract.ProviderDraft, 0, len(providers))
	for _, provider := range providers {
		out = append(out, contract.ProviderDraft{
			Name: provider.Name, Kind: provider.Kind, BaseURL: provider.BaseURL,
			Models: provider.Models, DefaultModel: provider.DefaultModel,
			APIKeyEnv: provider.APIKeyEnv, ContextWindow: provider.ContextWindow,
		})
	}
	return out
}

func fromProviderDrafts(drafts []contract.ProviderDraft) []ai.Provider {
	out := make([]ai.Provider, 0, len(drafts))
	for _, draft := range drafts {
		out = append(out, ai.Provider{
			Name: draft.Name, Kind: draft.Kind, BaseURL: draft.BaseURL,
			Models: draft.Models, DefaultModel: draft.DefaultModel,
			APIKeyEnv: draft.APIKeyEnv, ContextWindow: draft.ContextWindow,
		})
	}
	return out
}
