package skills

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

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

// MaxUpdateTextBytes is M18's write limit; a larger file goes through
// importFile instead of create/edit (16.6).
const MaxUpdateTextBytes = 2 << 20

// UpdateDiff is one file that differs between an installed skill and the
// staged new version.
type UpdateDiff struct {
	Path        string // slash-separated, relative to the skill directory
	Change      string // added | modified | removed
	Bytes       int64
	Binary      bool
	Text        string // staged text for added/modified when TextOK
	TextOK      bool   // the staged file can be written through create/edit
	LocalTextOK bool   // modified only: the local file is also plain text
	Hash        string // staged file hash for added/modified
}

// DiffSkillTrees compares the installed directory with the staged one, file by
// file (16.6). Links are skipped by SafeWalk on both sides.
func DiffSkillTrees(localDir, stagedDir string) ([]UpdateDiff, error) {
	localFiles, _, err := SafeWalk(localDir)
	if err != nil {
		return nil, err
	}
	stagedFiles, _, err := SafeWalk(stagedDir)
	if err != nil {
		return nil, err
	}
	localSet := map[string]bool{}
	for _, rel := range localFiles {
		localSet[filepath.ToSlash(rel)] = true
	}
	stagedSet := map[string]bool{}
	for _, rel := range stagedFiles {
		stagedSet[filepath.ToSlash(rel)] = true
	}
	all := make([]string, 0, len(localSet)+len(stagedSet))
	for rel := range stagedSet {
		all = append(all, rel)
	}
	for rel := range localSet {
		if !stagedSet[rel] {
			all = append(all, rel)
		}
	}
	sort.Strings(all)

	diffs := make([]UpdateDiff, 0, len(all))
	for _, rel := range all {
		localPath := filepath.Join(localDir, rel)
		stagedPath := filepath.Join(stagedDir, rel)
		switch {
		case !localSet[rel]:
			diff, err := describeStaged(stagedPath, rel, "added")
			if err != nil {
				return nil, err
			}
			diffs = append(diffs, diff)
		case !stagedSet[rel]:
			info, err := os.Stat(localPath)
			if err != nil {
				return nil, err
			}
			diff := UpdateDiff{Path: rel, Change: "removed", Bytes: info.Size(), Binary: !plainFile(localPath)}
			diffs = append(diffs, diff)
		default:
			stagedHash, err := fileSHA(stagedPath)
			if err != nil {
				return nil, err
			}
			localHash, err := fileSHA(localPath)
			if err != nil {
				return nil, err
			}
			if stagedHash == localHash {
				continue
			}
			diff, err := describeStaged(stagedPath, rel, "modified")
			if err != nil {
				return nil, err
			}
			diff.LocalTextOK = plainFile(localPath)
			diffs = append(diffs, diff)
		}
	}
	return diffs, nil
}

func describeStaged(path, rel, change string) (UpdateDiff, error) {
	info, err := os.Stat(path)
	if err != nil {
		return UpdateDiff{}, err
	}
	hash, err := fileSHA(path)
	if err != nil {
		return UpdateDiff{}, err
	}
	diff := UpdateDiff{Path: rel, Change: change, Bytes: info.Size(), Hash: hash}
	if text, ok := plainTextFile(path, info.Size()); ok {
		diff.Text = text
		diff.TextOK = true
		return diff, nil
	}
	diff.Binary = true
	return diff, nil
}

// plainTextFile reports whether a file can go through create/edit without
// changing its bytes: valid UTF-8, no BOM, LF endings and within the write
// limit. Everything else must use importFile so the updated tree stays
// byte-identical to the staged version (M95 compares tree hashes).
func plainTextFile(path string, size int64) (string, bool) {
	if size > MaxUpdateTextBytes {
		return "", false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	if bytes.HasPrefix(raw, []byte{0xEF, 0xBB, 0xBF}) || !utf8.Valid(raw) || bytes.ContainsRune(raw, '\r') {
		return "", false
	}
	return string(raw), true
}

func plainFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	_, ok := plainTextFile(path, info.Size())
	return ok
}

// FileSHA is the sha256 of one file, formatted like TreeHash. importFile pins
// this value (API §13.4).
func FileSHA(path string) (string, error) { return fileSHA(path) }

func fileSHA(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(digest.Sum(nil)), nil
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
