package service

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/skills"
)

// Update detection (M93, 技术设计 16.5, Q-SKM-2).

func (s *SkillService) updateCacheDir() string {
	return filepath.Join(s.app.DataDir, "skills")
}

func (s *SkillService) updateCache() *skills.UpdateCache {
	return skills.LoadUpdateCache(s.updateCacheDir())
}

// CheckUpdates is M93: fresh cache within 60 minutes is answered directly,
// otherwise one grouped round runs in the background and ends with E12.
func (s *SkillService) CheckUpdates(ctx context.Context, vaultID string, force bool) contract.SkillUpdateCheckResult {
	groups := s.updateGroups(vaultID)
	cache := s.updateCache()
	result := contract.SkillUpdateCheckResult{Groups: len(groups)}
	if cache != nil {
		result.CheckedAt = cache.CheckedAt
		result.Unchecked = cache.CountUnchecked(groups)
	}
	// Read and set the flag under one lock: two callers (panel open plus a
	// quick "check now") must never start two rounds.
	s.mu.Lock()
	result.RateLimitResetAt = s.rateLimitResetAt
	if s.checking {
		s.mu.Unlock()
		result.Checking = true
		return result
	}
	if (!force && cache.Fresh(s.now())) || len(groups) == 0 {
		s.mu.Unlock()
		return result
	}
	s.checking = true
	s.mu.Unlock()
	result.Checking = true
	go s.runUpdateCheck(groups)
	return result
}

// updateGroups collects the deduped (owner, repo, ref, path) groups from both
// source zones (16.2).
func (s *SkillService) updateGroups(vaultID string) []skills.UpdateGroup {
	var kbRecords []skills.KbSource
	if vaultID != "" {
		if root, f := s.app.Vaults.Root(vaultID); f == nil {
			kbRecords = skills.LoadKbSources(root).Records
		}
	}
	toolRecords := skills.LoadToolSources(s.updateCacheDir()).Records
	return skills.CollectUpdateGroups(kbRecords, toolRecords)
}

// runUpdateCheck asks GitHub about every group, one request each, and stops at
// the 40-group cap or an exhausted quota (16.2).
func (s *SkillService) runUpdateCheck(groups []skills.UpdateGroup) {
	defer func() {
		s.mu.Lock()
		s.checking = false
		s.mu.Unlock()
	}()
	client := s.githubClient(s.app.Version)
	results := make([]skills.UpdateResult, 0, len(groups))
	rateLimitReset := ""
	for index, group := range groups {
		if index >= skills.MaxUpdateGroups {
			break
		}
		entry := skills.UpdateResult{
			Owner: group.Owner, Repo: group.Repo, Ref: group.Ref, Path: group.Path,
			CheckedAt: s.now().UTC().Format(time.RFC3339),
		}
		commit, f := client.LatestPathCommit(context.Background(), skills.GithubRef{
			Owner: group.Owner, Repo: group.Repo, Ref: group.Ref, Path: group.Path,
		})
		if f != nil {
			entry.Error = f.Message
			if f.Detail != "" {
				entry.Error += "；" + f.Detail
			}
			results = append(results, entry)
			if f.Code == contract.CodeRateLimited {
				if reset := client.RateLimitResetAt(); !reset.IsZero() {
					rateLimitReset = reset.UTC().Format(time.RFC3339)
				}
				break
			}
			continue
		}
		entry.LatestPathCommit = commit
		results = append(results, entry)
	}
	if f := skills.SaveUpdateCache(s.updateCacheDir(), results, s.now()); f != nil {
		s.logOperation("update-check", "", "", "", "", "failed", "", f.Message)
	}
	s.mu.Lock()
	s.rateLimitResetAt = rateLimitReset
	s.mu.Unlock()
	// The scan payload carries the states, so the next scan must rebuild it.
	_ = os.Remove(s.cachePath())
	s.emitChanged()
}
