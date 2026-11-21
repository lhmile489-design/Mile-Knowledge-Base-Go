package skills

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// Update detection (技术设计 16.5, Q-SKM-2). The cache is derived data
// (数据库设计 4.6) and is safe to delete.
const (
	updateCacheName    = "update-cache.json"
	updateCacheVersion = 1
	UpdateCacheTTL     = 60 * time.Minute
	MaxUpdateGroups    = 40
)

// UpdateResult is one checked (owner, repo, ref, path) group.
type UpdateResult struct {
	Owner            string `json:"owner"`
	Repo             string `json:"repo"`
	Ref              string `json:"ref"`
	Path             string `json:"path"`
	LatestPathCommit string `json:"latestPathCommit,omitempty"`
	Error            string `json:"error,omitempty"`
	CheckedAt        string `json:"checkedAt"`
}

type updateCacheFile struct {
	Version   int            `json:"version"`
	CheckedAt string         `json:"checkedAt"`
	Results   []UpdateResult `json:"results"`
}

// UpdateCache is the parsed skills/update-cache.json (数据库设计 4.6).
type UpdateCache struct {
	CheckedAt string
	Results   []UpdateResult

	checkedAt time.Time
	byGroup   map[string]UpdateResult
}

// UpdateGroup is one unique remote location a check round asks about; several
// installed skills can share it (16.2).
type UpdateGroup struct {
	Owner string
	Repo  string
	Ref   string
	Path  string
}

// UpdateGroupKey is the internal dedupe key; it is never persisted.
func UpdateGroupKey(owner, repo, ref, path string) string {
	return owner + "\x00" + repo + "\x00" + ref + "\x00" + path
}

// CollectUpdateGroups dedupes the knowledge-base and tool source records into
// one sorted list of remote groups.
func CollectUpdateGroups(kbRecords []KbSource, toolRecords []ToolSource) []UpdateGroup {
	seen := map[string]bool{}
	groups := []UpdateGroup{}
	add := func(owner, repo, ref, path string) {
		if owner == "" || repo == "" || ref == "" {
			return
		}
		key := UpdateGroupKey(owner, repo, ref, path)
		if seen[key] {
			return
		}
		seen[key] = true
		groups = append(groups, UpdateGroup{Owner: owner, Repo: repo, Ref: ref, Path: path})
	}
	for _, record := range kbRecords {
		add(record.Owner, record.Repo, record.Ref, record.Path)
	}
	for _, record := range toolRecords {
		add(record.Owner, record.Repo, record.Ref, record.Path)
	}
	sort.Slice(groups, func(i, j int) bool {
		return UpdateGroupKey(groups[i].Owner, groups[i].Repo, groups[i].Ref, groups[i].Path) <
			UpdateGroupKey(groups[j].Owner, groups[j].Repo, groups[j].Ref, groups[j].Path)
	})
	return groups
}

// LoadUpdateCache reads the cache; a damaged or unknown-version file is
// deleted and reported as absent (派生数据).
func LoadUpdateCache(skillsDir string) *UpdateCache {
	path := filepath.Join(skillsDir, updateCacheName)
	body, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var file updateCacheFile
	if err := json.Unmarshal(body, &file); err != nil || file.Version != updateCacheVersion {
		_ = os.Remove(path)
		return nil
	}
	cache := &UpdateCache{
		CheckedAt: file.CheckedAt,
		Results:   []UpdateResult{},
		byGroup:   map[string]UpdateResult{},
	}
	cache.checkedAt, _ = time.Parse(time.RFC3339, file.CheckedAt)
	for _, result := range file.Results {
		if !validOrigin(result.Owner, result.Repo, result.Ref, result.Path) {
			continue
		}
		if result.LatestPathCommit == "" && result.Error == "" {
			continue
		}
		if result.LatestPathCommit != "" && !isHexSHA(result.LatestPathCommit) {
			continue
		}
		key := UpdateGroupKey(result.Owner, result.Repo, result.Ref, result.Path)
		cache.Results = append(cache.Results, result)
		cache.byGroup[key] = result
	}
	return cache
}

// SaveUpdateCache writes the round atomically with the stable encoding.
func SaveUpdateCache(skillsDir string, results []UpdateResult, checkedAt time.Time) *contract.Failure {
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		return contract.Fail(contract.CodeIOFailure, "无法创建应用数据目录")
	}
	body := encodeSources(updateCacheFile{
		Version:   updateCacheVersion,
		CheckedAt: checkedAt.UTC().Format(time.RFC3339),
		Results:   results,
	})
	path := filepath.Join(skillsDir, updateCacheName)
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, body, 0o644); err != nil {
		return contract.Fail(contract.CodeIOFailure, "无法写入更新缓存")
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return contract.Fail(contract.CodeIOFailure, "无法写入更新缓存")
	}
	return nil
}

// Fresh reports whether the cache is younger than 60 minutes (Q-SKM-2).
func (c *UpdateCache) Fresh(now time.Time) bool {
	if c == nil || c.checkedAt.IsZero() {
		return false
	}
	return now.Sub(c.checkedAt) < UpdateCacheTTL
}

// Lookup finds the result for one group.
func (c *UpdateCache) Lookup(owner, repo, ref, path string) (UpdateResult, bool) {
	if c == nil {
		return UpdateResult{}, false
	}
	result, ok := c.byGroup[UpdateGroupKey(owner, repo, ref, path)]
	return result, ok
}

// CountUnchecked counts groups without a usable result this round.
func (c *UpdateCache) CountUnchecked(groups []UpdateGroup) int {
	count := 0
	for _, group := range groups {
		result, ok := c.Lookup(group.Owner, group.Repo, group.Ref, group.Path)
		if !ok || result.Error != "" {
			count++
		}
	}
	return count
}

// RemoteUpdateState decides the five states of 技术设计 16.5 from the local
// hash and the cached remote answer. checked=false means this group was never
// asked (or dropped by the 40-group cap / rate limit).
func RemoteUpdateState(pathCommit, baseHash, localHash string, result UpdateResult, checked bool) (state, latest, checkedAt, errText string) {
	localChanged := baseHash != "" && localHash != "" && !strings.EqualFold(localHash, baseHash)
	if !checked {
		if localChanged {
			return "local-modified", "", "", ""
		}
		return "unchecked", "", "", ""
	}
	if result.Error != "" {
		return "error", "", result.CheckedAt, result.Error
	}
	remoteChanged := result.LatestPathCommit != "" && !strings.EqualFold(result.LatestPathCommit, pathCommit)
	switch {
	case remoteChanged && localChanged:
		return "both-changed", result.LatestPathCommit, result.CheckedAt, ""
	case remoteChanged:
		return "update-available", result.LatestPathCommit, result.CheckedAt, ""
	case localChanged:
		return "local-modified", result.LatestPathCommit, result.CheckedAt, ""
	default:
		return "up-to-date", result.LatestPathCommit, result.CheckedAt, ""
	}
}
