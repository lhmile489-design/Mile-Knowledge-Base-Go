package skills

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/pathguard"
)

// Source file locations (数据库设计 3.8 and 4.6).
const (
	kbSourcesRel   = ".mile-kb/skill-sources.json"
	toolSourcesRel = "sources.json"
	sourcesVersion = 1
)

// KbSource is one GitHub record for a knowledge-base skill (A zone).
type KbSource struct {
	SkillPath   string `json:"skillPath"`
	Owner       string `json:"owner"`
	Repo        string `json:"repo"`
	Ref         string `json:"ref"`
	Path        string `json:"path"`
	Commit      string `json:"commit"`
	PathCommit  string `json:"pathCommit"`
	BaseHash    string `json:"baseHash"`
	InstalledAt string `json:"installedAt"`
}

// KbSources is the parsed A-zone file.
type KbSources struct {
	Records        []KbSource
	VersionUnknown bool
	Warnings       []string
}

// ToolSource is one GitHub record for a skill installed straight into a tool
// (数据库设计 4.6 M5 增补).
type ToolSource struct {
	ID          string `json:"id"`
	ToolID      string `json:"toolId"`
	TargetPath  string `json:"targetPath"`
	Owner       string `json:"owner"`
	Repo        string `json:"repo"`
	Ref         string `json:"ref"`
	Path        string `json:"path"`
	Commit      string `json:"commit"`
	PathCommit  string `json:"pathCommit"`
	BaseHash    string `json:"baseHash"`
	InstalledAt string `json:"installedAt"`
}

// ToolSources is the parsed B-zone file.
type ToolSources struct {
	Records []ToolSource
}

type kbSourcesFile struct {
	Version int        `json:"version"`
	Records []KbSource `json:"records"`
}

type toolSourcesFile struct {
	Version int          `json:"version"`
	Records []ToolSource `json:"records"`
}

// LoadKbSources reads the knowledge-base source file. Unknown versions are
// reported and left untouched; malformed records are dropped with a warning
// (3.8).
func LoadKbSources(vaultRoot string) KbSources {
	result := KbSources{Records: []KbSource{}}
	path, f := pathguard.Resolve(vaultRoot, kbSourcesRel)
	if f != nil {
		result.Warnings = append(result.Warnings, "来源记录路径不可用")
		return result
	}
	body, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			result.Warnings = append(result.Warnings, "来源记录无法读取")
		}
		return result
	}
	var file kbSourcesFile
	if err := json.Unmarshal(body, &file); err != nil {
		result.Warnings = append(result.Warnings, "来源记录已损坏，本次忽略")
		return result
	}
	if file.Version != sourcesVersion {
		result.VersionUnknown = true
		result.Warnings = append(result.Warnings, "来源记录版本不认识，本次忽略")
		return result
	}
	for _, record := range file.Records {
		if !validKbSkillPath(record.SkillPath) || !isHexSHA(record.Commit) || !isHexSHA(record.PathCommit) {
			result.Warnings = append(result.Warnings, "来源记录里有无效条目，已跳过")
			continue
		}
		result.Records = append(result.Records, record)
	}
	return result
}

// SaveKbSource merges one record by skillPath and writes the file with the
// stable serialization from 数据库设计 3.4/3.8.
func SaveKbSource(vaultRoot string, record KbSource) *contract.Failure {
	if !validKbSkillPath(record.SkillPath) || !validOrigin(record.Owner, record.Repo, record.Ref, record.Path) ||
		!isHexSHA(record.Commit) || !isHexSHA(record.PathCommit) {
		return contract.Fail(contract.CodeInvalidInput, "来源记录字段不合法")
	}
	existing := LoadKbSources(vaultRoot)
	if existing.VersionUnknown {
		return contract.Fail(contract.CodeUnsupported, "来源记录版本不认识，未写入")
	}
	merged := make([]KbSource, 0, len(existing.Records)+1)
	replaced := false
	for _, item := range existing.Records {
		if item.SkillPath == record.SkillPath {
			merged = append(merged, record)
			replaced = true
			continue
		}
		merged = append(merged, item)
	}
	if !replaced {
		merged = append(merged, record)
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].SkillPath < merged[j].SkillPath })
	return writeKbSources(vaultRoot, merged)
}

// RemoveKbSource drops one record; a missing record is fine.
func RemoveKbSource(vaultRoot, skillPath string) *contract.Failure {
	existing := LoadKbSources(vaultRoot)
	if existing.VersionUnknown {
		return contract.Fail(contract.CodeUnsupported, "来源记录版本不认识，未写入")
	}
	merged := make([]KbSource, 0, len(existing.Records))
	for _, item := range existing.Records {
		if item.SkillPath != skillPath {
			merged = append(merged, item)
		}
	}
	return writeKbSources(vaultRoot, merged)
}

func writeKbSources(vaultRoot string, records []KbSource) *contract.Failure {
	path, f := pathguard.Resolve(vaultRoot, kbSourcesRel)
	if f != nil {
		return contract.Fail(contract.CodeIOFailure, "来源记录路径不可用")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return contract.Fail(contract.CodeIOFailure, "无法创建来源记录目录")
	}
	body := encodeSources(kbSourcesFile{Version: sourcesVersion, Records: records})
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, body, 0o644); err != nil {
		return contract.Fail(contract.CodeIOFailure, "无法写入来源记录")
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return contract.Fail(contract.CodeIOFailure, "无法写入来源记录")
	}
	return nil
}

// encodeSources matches the legacy JSON style: two-space indent, no HTML
// escaping, trailing newline (数据库设计 3.0.1).
func encodeSources(value any) []byte {
	var builder strings.Builder
	encoder := json.NewEncoder(&builder)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return nil
	}
	return []byte(builder.String())
}

// LoadToolSources reads the B-zone file; a damaged file starts empty (4.6).
func LoadToolSources(skillsDir string) ToolSources {
	result := ToolSources{Records: []ToolSource{}}
	path := filepath.Join(skillsDir, toolSourcesRel)
	body, err := os.ReadFile(path)
	if err != nil {
		return result
	}
	var file toolSourcesFile
	if err := json.Unmarshal(body, &file); err != nil || file.Version != sourcesVersion {
		_ = os.Rename(path, path+".corrupt-"+time.Now().UTC().Format("20060102T150405"))
		return result
	}
	for _, record := range file.Records {
		if !IsToolSourceID(record.ID) || record.ToolID == "" || record.TargetPath == "" ||
			!validOrigin(record.Owner, record.Repo, record.Ref, record.Path) ||
			!isHexSHA(record.Commit) || !isHexSHA(record.PathCommit) {
			continue
		}
		result.Records = append(result.Records, record)
	}
	return result
}

// SaveToolSource merges by targetPath and writes atomically.
func SaveToolSource(skillsDir string, record ToolSource) *contract.Failure {
	if record.ID == "" {
		id, err := newToolSourceID()
		if err != nil {
			return contract.Fail(contract.CodeIOFailure, "无法生成来源记录编号")
		}
		record.ID = id
	}
	if !IsToolSourceID(record.ID) || record.ToolID == "" || record.TargetPath == "" ||
		!validOrigin(record.Owner, record.Repo, record.Ref, record.Path) ||
		!isHexSHA(record.Commit) || !isHexSHA(record.PathCommit) {
		return contract.Fail(contract.CodeInvalidInput, "来源记录字段不合法")
	}
	existing := LoadToolSources(skillsDir)
	merged := make([]ToolSource, 0, len(existing.Records)+1)
	replaced := false
	for _, item := range existing.Records {
		if strings.EqualFold(item.TargetPath, record.TargetPath) {
			merged = append(merged, record)
			replaced = true
			continue
		}
		merged = append(merged, item)
	}
	if !replaced {
		merged = append(merged, record)
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].TargetPath < merged[j].TargetPath })
	return writeToolSources(skillsDir, merged)
}

// RemoveToolSource drops the record for one target path.
func RemoveToolSource(skillsDir, targetPath string) *contract.Failure {
	existing := LoadToolSources(skillsDir)
	merged := make([]ToolSource, 0, len(existing.Records))
	for _, item := range existing.Records {
		if !strings.EqualFold(item.TargetPath, targetPath) {
			merged = append(merged, item)
		}
	}
	return writeToolSources(skillsDir, merged)
}

func writeToolSources(skillsDir string, records []ToolSource) *contract.Failure {
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		return contract.Fail(contract.CodeIOFailure, "无法创建应用数据目录")
	}
	path := filepath.Join(skillsDir, toolSourcesRel)
	body := encodeSources(toolSourcesFile{Version: sourcesVersion, Records: records})
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, body, 0o644); err != nil {
		return contract.Fail(contract.CodeIOFailure, "无法写入来源记录")
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return contract.Fail(contract.CodeIOFailure, "无法写入来源记录")
	}
	return nil
}

// AttachKbRemotes fills KbSkill.Remote from the A-zone records, the local hash
// and the latest update check (16.5).
func AttachKbRemotes(vaultRoot string, kbs []contract.KbSkill, sources KbSources, cache *UpdateCache) {
	if len(sources.Records) == 0 {
		return
	}
	byPath := make(map[string]KbSource, len(sources.Records))
	for _, record := range sources.Records {
		byPath[record.SkillPath] = record
	}
	for index := range kbs {
		record, ok := byPath[kbs[index].Path]
		if !ok {
			continue
		}
		kbs[index].Remote = remoteFor(vaultRoot, kbs[index].Path, record, cache)
	}
}

// remoteFor builds the SkillRemote shown in the UI.
func remoteFor(root, dir string, record KbSource, cache *UpdateCache) *contract.SkillRemote {
	localHash := ""
	if hash, err := TreeHash(filepath.Join(root, filepath.FromSlash(dir))); err == nil {
		localHash = hash
	}
	result, checked := cache.Lookup(record.Owner, record.Repo, record.Ref, record.Path)
	state, latest, checkedAt, errText := RemoteUpdateState(record.PathCommit, record.BaseHash, localHash, result, checked)
	return &contract.SkillRemote{
		SkillOrigin: contract.SkillOrigin{
			Owner: record.Owner, Repo: record.Repo, Ref: record.Ref, Path: record.Path, Commit: record.Commit,
		},
		PathCommit:       record.PathCommit,
		InstalledAt:      record.InstalledAt,
		UpdateState:      state,
		LatestPathCommit: latest,
		CheckedAt:        checkedAt,
		Error:            errText,
	}
}

// matchToolSource finds the B-zone record for a skill directory.
func matchToolSource(records []ToolSource, dir string) (ToolSource, bool) {
	for _, record := range records {
		if strings.EqualFold(filepath.Clean(record.TargetPath), filepath.Clean(dir)) {
			return record, true
		}
	}
	return ToolSource{}, false
}

// FindToolSource exposes the B-zone lookup to the update flow (M94/M96).
func FindToolSource(records []ToolSource, dir string) (ToolSource, bool) {
	return matchToolSource(records, dir)
}

// IsValidKbSkillPath reports whether value is skills/<category>/<name>.
func IsValidKbSkillPath(value string) bool { return validKbSkillPath(value) }

// toolRemoteFor fills ToolSkill.Remote for a skill installed straight into a
// tool (B zone).
func toolRemoteFor(dir string, record ToolSource, cache *UpdateCache) *contract.SkillRemote {
	source := KbSource{
		Owner: record.Owner, Repo: record.Repo, Ref: record.Ref,
		Path: record.Path, Commit: record.Commit, PathCommit: record.PathCommit,
		BaseHash: record.BaseHash, InstalledAt: record.InstalledAt,
	}
	return remoteFor(filepath.Dir(dir), filepath.Base(dir), source, cache)
}

func validKbSkillPath(value string) bool {
	segments := strings.Split(value, "/")
	return len(segments) == 3 && segments[0] == "skills" &&
		IsValidSegment(segments[1]) && IsValidSegment(segments[2])
}

func validOrigin(owner, repo, ref, path string) bool {
	if owner == "" || repo == "" || ref == "" {
		return false
	}
	for _, r := range owner + repo {
		if !isGithubNameRune(r) {
			return false
		}
	}
	if !IsValidSegment(ref) {
		return false
	}
	if path == "" {
		return true
	}
	for _, segment := range strings.Split(path, "/") {
		if !IsValidSegment(segment) {
			return false
		}
	}
	return true
}

// IsToolSourceID accepts g-<12 hex>.
func IsToolSourceID(id string) bool {
	if len(id) != 14 || !strings.HasPrefix(id, "g-") {
		return false
	}
	for _, r := range id[2:] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

func newToolSourceID() (string, error) {
	buffer := make([]byte, 6)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return "g-" + hex.EncodeToString(buffer), nil
}
