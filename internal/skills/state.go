package skills

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// scan-cache.json (数据库设计 4.6): the last scan, shown while the next one
// runs. It is derived data — a broken or old file is dropped and rebuilt.

const (
	scanCacheVersion    = 2 // v2: SkillTool.kind/missing, ToolSkill.sourceToolId/subdir (M6, API §14.1)
	scanCacheMaxBytes   = 2 << 20
	scanCacheMaxEntries = 2000
)

// ScanCache is the on-disk shape; CachedAt drives the UI's "cached" label.
type ScanCache struct {
	Version  int                         `json:"version"`
	CachedAt string                      `json:"cachedAt"`
	Kb       []contract.SkillCategory    `json:"kb"`
	Tools    []contract.SkillToolScan    `json:"tools"`
	Warnings []contract.SkillScanWarning `json:"warnings,omitempty"`
}

// LoadScanCache returns nil for a missing, unreadable, oversized or
// version-mismatched file; callers simply scan again.
func LoadScanCache(path string) *ScanCache {
	info, err := os.Stat(path)
	if err != nil || info.Size() > scanCacheMaxBytes {
		return nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cache ScanCache
	if err := json.Unmarshal(body, &cache); err != nil || cache.Version != scanCacheVersion {
		return nil
	}
	return &cache
}

// SaveScanCache writes atomically and caps the tool entries at 2000 (a
// warning records the trim, 数据库设计 4.6).
func SaveScanCache(path string, scan contract.SkillScan, cachedAt string) error {
	cache := ScanCache{
		Version:  scanCacheVersion,
		CachedAt: cachedAt,
		Kb:       scan.Kb,
		Tools:    scan.Tools,
		Warnings: scan.Warnings,
	}
	if cache.Kb == nil {
		cache.Kb = []contract.SkillCategory{}
	}
	if cache.Tools == nil {
		cache.Tools = []contract.SkillToolScan{}
	}
	total := 0
	for _, group := range cache.Tools {
		total += len(group.Skills)
	}
	if total > scanCacheMaxEntries {
		trimmed := make([]contract.SkillToolScan, 0, len(cache.Tools))
		left := scanCacheMaxEntries
		for _, group := range cache.Tools {
			if left <= 0 {
				break
			}
			if len(group.Skills) > left {
				group.Skills = group.Skills[:left]
			}
			left -= len(group.Skills)
			trimmed = append(trimmed, group)
		}
		cache.Tools = trimmed
		cache.Warnings = append(cache.Warnings, contract.SkillScanWarning{
			Text: "扫描结果超过 2000 个条目，缓存只保留前 2000 个",
		})
	}
	body, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(body, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ToScan turns a cache row into the M77 payload.
func (c *ScanCache) ToScan(scanning bool) contract.SkillScan {
	return contract.SkillScan{
		CachedAt: c.CachedAt,
		Scanning: scanning,
		Kb:       c.Kb,
		Tools:    c.Tools,
		Warnings: c.Warnings,
	}
}

// Distribution records live in skills/state.json (数据库设计 4.6). They are
// the source of truth for "which knowledge-base skill was distributed to
// which tool, and how" — the scan and later uninstall both read them.

const stateVersion = 1

// State is the on-disk shape of skills/state.json.
type State struct {
	Version       int            `json:"version"`
	Distributions []Distribution `json:"distributions"`
}

// LoadState returns an empty state for a missing or corrupt file; callers do
// not fail a scan because a bookkeeping file is unreadable.
func LoadState(path string) *State {
	body, err := os.ReadFile(path)
	if err != nil {
		return &State{Version: stateVersion, Distributions: []Distribution{}}
	}
	var state State
	if err := json.Unmarshal(body, &state); err != nil || state.Version != stateVersion {
		return &State{Version: stateVersion, Distributions: []Distribution{}}
	}
	if state.Distributions == nil {
		state.Distributions = []Distribution{}
	}
	return &state
}

// SaveState writes state.json atomically.
func SaveState(path string, state State) error {
	state.Version = stateVersion
	if state.Distributions == nil {
		state.Distributions = []Distribution{}
	}
	body, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(body, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// RemoveDistributions drops every record of one tool or registered target
// and reports how many were removed (M6, API §14.2 M100).
func RemoveDistributions(path, toolID string) (int, *contract.Failure) {
	state := LoadState(path)
	kept := make([]Distribution, 0, len(state.Distributions))
	for _, record := range state.Distributions {
		if record.ToolID != toolID {
			kept = append(kept, record)
		}
	}
	removed := len(state.Distributions) - len(kept)
	if removed == 0 {
		return 0, nil
	}
	state.Distributions = kept
	if err := SaveState(path, *state); err != nil {
		return 0, contract.Fail(contract.CodeIOFailure, "无法写入分发记录")
	}
	return removed, nil
}

// AddDistribution appends or replaces the record for the same tool+target.
func (s *State) AddDistribution(record Distribution) {
	for i, existing := range s.Distributions {
		if existing.ToolID == record.ToolID && strings.EqualFold(existing.TargetPath, record.TargetPath) {
			s.Distributions[i] = record
			return
		}
	}
	s.Distributions = append(s.Distributions, record)
}
