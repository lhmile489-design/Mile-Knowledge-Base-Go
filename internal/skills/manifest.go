package skills

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/pathguard"
)

// The distribution manifest lives inside the vault (数据库设计 3.9, 技术设计
// 18.6). It is machine-independent: no absolute paths, no registered target
// ids, only names and registry ids.
const (
	manifestRel      = ".mile-kb/skill-distributions.json"
	manifestVersion  = 1
	manifestMaxItems = 2000
	manifestMaxBytes = 1 << 20
)

// Manifest read states (API.md §15.1 SkillManifestStatus).
const (
	ManifestOK             = "ok"
	ManifestMissing        = "missing"
	ManifestUnreadable     = "unreadable"
	ManifestTooLarge       = "too-large"
	ManifestVersionUnknown = "version-unknown"
)

// ManifestTarget is the machine-independent destination of one item.
type ManifestTarget struct {
	Kind   string `json:"kind"`             // builtin | custom | project
	ToolID string `json:"toolId,omitempty"` // builtin
	Name   string `json:"name,omitempty"`   // custom / project: registered name
	Subdir string `json:"subdir,omitempty"` // project: project-relative install subdir
}

// ManifestItem is one distribution (source=kb) or GitHub tool install
// (source=github). The JSON field order matches 数据库设计 3.9.
type ManifestItem struct {
	Source     string         `json:"source"`
	Target     ManifestTarget `json:"target"`
	Name       string         `json:"name"`
	SkillPath  string         `json:"skillPath,omitempty"`
	Mode       string         `json:"mode,omitempty"`
	Owner      string         `json:"owner,omitempty"`
	Repo       string         `json:"repo,omitempty"`
	Ref        string         `json:"ref,omitempty"`
	Path       string         `json:"path,omitempty"`
	Commit     string         `json:"commit,omitempty"`
	PathCommit string         `json:"pathCommit,omitempty"`
	UpdatedAt  string         `json:"updatedAt"`
}

// Manifest is the parsed vault file. Items are validated and deduplicated;
// Status follows API §15.1.
type Manifest struct {
	Status    string
	UpdatedAt string
	Items     []ManifestItem
	Warnings  []string
}

type manifestFile struct {
	Version   int            `json:"version"`
	UpdatedAt string         `json:"updatedAt"`
	Items     []ManifestItem `json:"items"`
}

// LoadManifest reads the manifest from the vault. A missing file is
// ManifestMissing; broken, oversized and future-version files are reported
// and left untouched. Invalid items are dropped with a warning and the file
// stays ManifestOK (技术设计 18.6).
func LoadManifest(vaultRoot string) Manifest {
	result := Manifest{Status: ManifestMissing, Items: []ManifestItem{}}
	path, f := pathguard.Resolve(vaultRoot, manifestRel)
	if f != nil {
		result.Status = ManifestUnreadable
		result.Warnings = append(result.Warnings, "分发清单路径不可用")
		return result
	}
	info, err := os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			result.Status = ManifestUnreadable
			result.Warnings = append(result.Warnings, "分发清单无法读取")
		}
		return result
	}
	if info.Size() > manifestMaxBytes {
		result.Status = ManifestTooLarge
		result.Warnings = append(result.Warnings, "分发清单超过 1 MiB，已忽略")
		return result
	}
	body, err := os.ReadFile(path)
	if err != nil {
		result.Status = ManifestUnreadable
		result.Warnings = append(result.Warnings, "分发清单无法读取")
		return result
	}
	var header struct {
		Version   int             `json:"version"`
		UpdatedAt string          `json:"updatedAt"`
		Items     json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(body, &header); err != nil {
		result.Status = ManifestUnreadable
		result.Warnings = append(result.Warnings, "分发清单已损坏，本次忽略")
		return result
	}
	if header.Version != manifestVersion {
		result.Status = ManifestVersionUnknown
		result.Warnings = append(result.Warnings, "分发清单版本不认识，本次忽略")
		return result
	}
	trimmed := strings.TrimSpace(string(header.Items))
	if trimmed == "" || trimmed == "null" || !strings.HasPrefix(trimmed, "[") {
		result.Status = ManifestUnreadable
		result.Warnings = append(result.Warnings, "分发清单缺少条目数组，本次忽略")
		return result
	}
	items := []ManifestItem{}
	if err := json.Unmarshal(header.Items, &items); err != nil {
		result.Status = ManifestUnreadable
		result.Warnings = append(result.Warnings, "分发清单条目无法解析，本次忽略")
		return result
	}
	result.Status = ManifestOK
	result.UpdatedAt = header.UpdatedAt
	if result.UpdatedAt != "" && !validManifestTime(result.UpdatedAt) {
		result.Warnings = append(result.Warnings, "分发清单的更新时间不合法，已忽略")
		result.UpdatedAt = ""
	}

	dropped := 0
	seen := map[string]int{}
	kept := make([]ManifestItem, 0, len(items))
	for _, item := range items {
		if !validManifestItem(item) {
			dropped++
			continue
		}
		key := manifestSortKey(item)
		if index, ok := seen[key]; ok {
			kept[index] = item
			dropped++
			continue
		}
		seen[key] = len(kept)
		kept = append(kept, item)
	}
	if len(kept) > manifestMaxItems {
		dropped += len(kept) - manifestMaxItems
		kept = kept[:manifestMaxItems]
	}
	sort.SliceStable(kept, func(i, j int) bool {
		return manifestSortKey(kept[i]) < manifestSortKey(kept[j])
	})
	if dropped > 0 {
		result.Warnings = append(result.Warnings, "分发清单里有无效或重复条目，已跳过")
	}
	result.Items = kept
	return result
}

// SaveManifest validates every item, deduplicates by identity, sorts and
// writes the file atomically (技术设计 18.6). An invalid item refuses the
// whole write so a bug can never leak a bad path into the vault.
func SaveManifest(vaultRoot string, manifest *Manifest) *contract.Failure {
	if manifest == nil {
		return contract.Fail(contract.CodeInvalidInput, "分发清单为空")
	}
	// 数据库设计 3.9: the top-level updatedAt is the last write time, so it is
	// stamped on every save (the caller's value is read-only metadata).
	updatedAt := time.Now().UTC().Format(time.RFC3339)

	byKey := map[string]ManifestItem{}
	order := make([]string, 0, len(manifest.Items))
	for _, item := range manifest.Items {
		if !validManifestItem(item) {
			return contract.Fail(contract.CodeInvalidInput, "分发清单条目不合法")
		}
		key := manifestSortKey(item)
		if _, ok := byKey[key]; !ok {
			order = append(order, key)
		}
		byKey[key] = item
	}
	if len(order) > manifestMaxItems {
		return contract.Fail(contract.CodeTooLarge, "分发清单超过 2000 项").WithLimit(int64(len(order)), manifestMaxItems)
	}
	items := make([]ManifestItem, 0, len(order))
	for _, key := range order {
		items = append(items, byKey[key])
	}
	sort.SliceStable(items, func(i, j int) bool {
		return manifestSortKey(items[i]) < manifestSortKey(items[j])
	})

	path, f := pathguard.Resolve(vaultRoot, manifestRel)
	if f != nil {
		return contract.Fail(contract.CodeIOFailure, "分发清单路径不可用")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return contract.Fail(contract.CodeIOFailure, "无法创建分发清单目录")
	}
	file := manifestFile{Version: manifestVersion, UpdatedAt: updatedAt, Items: items}
	var builder strings.Builder
	encoder := json.NewEncoder(&builder)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(file); err != nil {
		return contract.Fail(contract.CodeIOFailure, "无法编码分发清单")
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, []byte(builder.String()), 0o644); err != nil {
		return contract.Fail(contract.CodeIOFailure, "无法写入分发清单")
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return contract.Fail(contract.CodeIOFailure, "无法写入分发清单")
	}
	manifest.UpdatedAt = updatedAt
	manifest.Status = ManifestOK
	return nil
}

// ManifestKey is the identity of one item: target plus skill name, compared
// case-insensitively (技术设计 18.4).
func ManifestKey(target ManifestTarget, name string) string {
	return manifestSortKey(ManifestItem{Target: target, Name: name})
}

func manifestSortKey(item ManifestItem) string {
	target := item.Target
	second := strings.ToLower(target.ToolID)
	if target.Kind != "builtin" {
		second = strings.ToLower(target.Name)
	}
	return strings.ToLower(target.Kind) + "\x00" + second + "\x00" +
		strings.ToLower(target.Subdir) + "\x00" + strings.ToLower(item.Name)
}

// Upsert adds or replaces the item with the same identity and reports whether
// anything changed.
func (m *Manifest) Upsert(item ManifestItem) bool {
	key := manifestSortKey(item)
	for i, existing := range m.Items {
		if manifestSortKey(existing) == key {
			if existing == item {
				return false
			}
			m.Items[i] = item
			return true
		}
	}
	m.Items = append(m.Items, item)
	return true
}

// Remove drops the item with the same identity and reports whether it existed.
func (m *Manifest) Remove(target ManifestTarget, name string) bool {
	key := ManifestKey(target, name)
	for i, existing := range m.Items {
		if manifestSortKey(existing) == key {
			m.Items = append(m.Items[:i], m.Items[i+1:]...)
			return true
		}
	}
	return false
}

// validManifestItem checks every field of one item (技术设计 18.7).
func validManifestItem(item ManifestItem) bool {
	if !IsValidSegment(item.Name) || !validManifestTime(item.UpdatedAt) || !validManifestTarget(item.Target) {
		return false
	}
	switch item.Source {
	case "kb":
		if !validKbSkillPath(item.SkillPath) || (item.Mode != "link" && item.Mode != "copy") {
			return false
		}
		return item.Owner == "" && item.Repo == "" && item.Ref == "" &&
			item.Path == "" && item.Commit == "" && item.PathCommit == ""
	case "github":
		if item.SkillPath != "" || item.Mode != "" {
			return false
		}
		return validOrigin(item.Owner, item.Repo, item.Ref, item.Path) &&
			isHexSHA(item.Commit) && isHexSHA(item.PathCommit)
	}
	return false
}

// validManifestTarget checks the machine-independent destination.
func validManifestTarget(target ManifestTarget) bool {
	switch target.Kind {
	case "builtin":
		if target.ToolID == "" || target.Name != "" || target.Subdir != "" {
			return false
		}
		_, ok := ToolByID(target.ToolID)
		return ok
	case "custom":
		return target.ToolID == "" && target.Subdir == "" && IsValidSegment(target.Name)
	case "project":
		if target.ToolID != "" || !IsValidSegment(target.Name) || target.Subdir == "" {
			return false
		}
		for _, rel := range ProjectSkillDirs() {
			if rel == target.Subdir {
				return true
			}
		}
	}
	return false
}

func validManifestTime(value string) bool {
	if value == "" {
		return false
	}
	_, err := time.Parse(time.RFC3339, value)
	return err == nil
}
