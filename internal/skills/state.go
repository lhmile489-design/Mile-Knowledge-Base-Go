package skills

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// scan-cache.json (数据库设计 4.6): the last scan, shown while the next one
// runs. It is derived data — a broken or old file is dropped and rebuilt.

const (
	scanCacheVersion    = 1
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
