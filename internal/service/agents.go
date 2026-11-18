package service

import (
	"context"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/ai"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// AppConfigPatchAgent merges one agent's patch and writes agents.json (M63).
func (s *SettingsService) AppConfigPatchAgent(ctx context.Context, vaultID, agent string, patch contract.AgentPatch) contract.AppConfigResult {
	result := s.appConfigResult(ctx, vaultID)
	if result.Failure != nil {
		return result
	}
	agents, f := ai.ReadAgents(s.vaultRootOrEmpty(vaultID))
	if f != nil {
		result.Failure = f
		return result
	}
	next, f := ai.ApplyAgentPatch(agents, agent, patch)
	if f != nil {
		result.Failure = f
		return result
	}
	if f := ai.WriteAgents(s.vaultRootOrEmpty(vaultID), next); f != nil {
		result.Failure = f
		return result
	}
	result.Agents = toContractAgents(next)
	return result
}

// AppConfigResetSteering restores one agent's default steering (M64). The
// response carries the defaults; nothing is written until the user saves.
func (s *SettingsService) AppConfigResetSteering(ctx context.Context, vaultID, agent string) contract.AppConfigResult {
	result := s.appConfigResult(ctx, vaultID)
	if result.Failure != nil {
		return result
	}
	agents, f := ai.ReadAgents(s.vaultRootOrEmpty(vaultID))
	if f != nil {
		result.Failure = f
		return result
	}
	next, f := ai.ResetAgentSteering(agents, agent)
	if f != nil {
		result.Failure = f
		return result
	}
	if f := ai.WriteAgents(s.vaultRootOrEmpty(vaultID), next); f != nil {
		result.Failure = f
		return result
	}
	result.Agents = toContractAgents(next)
	return result
}

// SkillCatalog returns the skills the settings page can assign (M71). It is
// derived from the index; before the index is open the list is empty.
func (s *SettingsService) SkillCatalog(ctx context.Context, vaultID string) contract.SkillCatalogResult {
	result := contract.SkillCatalogResult{Skills: []contract.SkillEntry{}}
	if f := validateID(vaultID); f != nil {
		result.Failure = f
		return result
	}
	if _, f := s.app.Vaults.Root(vaultID); f != nil {
		result.Failure = contract.Fail(contract.CodeVaultUnavailable, "知识库不可用")
		return result
	}
	handle := s.app.indexHandle(vaultID)
	if handle == nil {
		return result
	}
	handle.mu.Lock()
	store := handle.store
	handle.mu.Unlock()
	if store == nil {
		return result
	}
	docs, err := store.SkillDocuments(ctx)
	if err != nil {
		result.Failure = contract.Fail(contract.CodeIOFailure, "读取技能索引失败")
		return result
	}
	list := make([]ai.SkillDoc, 0, len(docs))
	for _, doc := range docs {
		list = append(list, ai.SkillDoc{Path: doc.Path, Title: doc.Title})
	}
	for _, entry := range ai.BuildSkillCatalog(list) {
		result.Skills = append(result.Skills, contract.SkillEntry{Path: entry.Path, Category: entry.Category, Name: entry.Name})
	}
	return result
}

func (s *SettingsService) appConfigResult(ctx context.Context, vaultID string) contract.AppConfigResult {
	result := s.AppConfigGet(ctx, vaultID)
	if result.Failure != nil {
		return result
	}
	root, f := s.app.Vaults.Root(vaultID)
	if f != nil {
		result.Failure = f
		return result
	}
	agents, _ := ai.ReadAgents(root)
	result.Agents = toContractAgents(agents)
	return result
}

func (s *SettingsService) vaultRootOrEmpty(vaultID string) string {
	root, _ := s.app.Vaults.Root(vaultID)
	return root
}

func toContractAgents(agents map[string]ai.AgentConfig) map[string]contract.AgentConfig {
	out := make(map[string]contract.AgentConfig, len(agents))
	for id, config := range agents {
		out[id] = contract.AgentConfig{
			Model: config.Model, Effort: config.Effort, Approval: config.Approval,
			SkillPaths: config.SkillPaths, Steering: config.Steering,
			SteeringBaseVersion: config.SteeringBaseVersion,
		}
	}
	return out
}
