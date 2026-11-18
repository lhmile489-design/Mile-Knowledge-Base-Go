package vault

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/pathguard"
)

// scafoldDirectories are created for a brand-new vault (PRD FR-VLT). skills/ is
// deliberately absent: categorisation differs per user.
var scaffoldDirectories = []string{
	"raws", "_archives", "projects", "tools", "AI Knowledge base", "dev-constraints",
}

var scaffoldReadme = map[string]string{
	"raws":              "# 暂存\n\n原始材料、剪藏与草稿放在这里。LLM 可以直接写入本目录。\n",
	"_archives":         "# 归档\n\n被替换或不再活跃的内容移到这里，保留可追溯性。\n",
	"projects":          "# 项目\n\n每个项目一个子目录。从 skills 复制来的技能副本会记录来源。\n",
	"tools":             "# 工具\n\n脚本、小工具与其说明。\n",
	"AI Knowledge base": "# 独立知识库\n\n成体系的 AI 知识条目。\n",
	"dev-constraints":   "# 约束\n\n工程约定与不可违反的规则。\n",
}

// NeedsScaffold reports whether none of the canonical directories exist yet.
func NeedsScaffold(root string) bool {
	for _, dir := range scaffoldDirectories {
		if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(dir))); err == nil && info.IsDir() {
			return false
		}
	}
	return true
}

// CreateScaffold creates the missing canonical directories with their README.
// It is idempotent and returns the directories it created.
func CreateScaffold(root string) ([]string, error) {
	var created []string
	for _, dir := range scaffoldDirectories {
		full := filepath.Join(root, filepath.FromSlash(dir))
		if info, err := os.Stat(full); err == nil && info.IsDir() {
			continue
		}
		if err := os.MkdirAll(full, 0o755); err != nil {
			return created, err
		}
		readme := filepath.Join(full, "README.md")
		if _, err := os.Stat(readme); err != nil {
			if err := os.WriteFile(readme, []byte(scaffoldReadme[dir]), 0o644); err != nil {
				return created, err
			}
		}
		created = append(created, dir)
	}
	return created, nil
}

// ListDir returns one directory level of a vault. Directories come first, then
// files, each group sorted by name case-insensitively (PRD FR-TREE).
func ListDir(root, rel string, aliases map[string]string) (contract.TreeResult, *contract.Failure) {
	abs, f := pathguard.Resolve(root, rel)
	if f != nil {
		return contract.TreeResult{}, f
	}
	info, err := os.Stat(abs)
	if err != nil {
		return contract.TreeResult{}, contract.Fail(contract.CodeNotFound, "目录不存在")
	}
	if !info.IsDir() {
		return contract.TreeResult{}, contract.Fail(contract.CodeNotADirectory, "不是目录")
	}

	dirEntries, err := os.ReadDir(abs)
	if err != nil {
		return contract.TreeResult{}, contract.Fail(contract.CodeIOFailure, "无法读取目录")
	}

	normRel, _ := pathguard.ValidateRel(rel)
	entries := make([]contract.FileEntry, 0, len(dirEntries))
	for _, de := range dirEntries {
		name := de.Name()
		childRel := name
		if normRel != "" {
			childRel = normRel + "/" + name
		}
		if f := pathguard.ValidateNameChars(name); f != nil {
			continue // names Windows itself would not create; never surfaced
		}
		entry := contract.FileEntry{VaultID: "", Path: childRel, Name: name}
		info, statErr := os.Lstat(filepath.Join(abs, name))
		if statErr != nil {
			continue
		}
		top := normRel == ""
		area := AreaOther
		if de.IsDir() {
			entry.Kind = "directory"
			if top {
				area = AreaFor(name)
			} else {
				area = AreaOther
			}
			if count, err := countChildren(filepath.Join(abs, name)); err == nil {
				entry.ChildCount = &count
			}
			entry.Icon = IconFor(name, area)
			entry.Alias = aliasFor(top, name, aliases)
		} else {
			entry.Kind = "file"
			entry.Size = info.Size()
			entry.MtimeMs = info.ModTime().UnixMilli()
			entry.Icon = "file"
			if top {
				area = AreaFor(name)
			}
		}
		entry.Area = area
		entry.Ignored = area == AreaSystem
		entries = append(entries, entry)
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if (entries[i].Kind == "directory") != (entries[j].Kind == "directory") {
			return entries[i].Kind == "directory"
		}
		a, b := strings.ToLower(entries[i].Name), strings.ToLower(entries[j].Name)
		if a == b {
			return entries[i].Name < entries[j].Name
		}
		return a < b
	})

	result := contract.TreeResult{Path: normRel, Entries: entries}
	if normRel == "" {
		result.SkillCategoryCount = countSkillCategories(abs)
	}
	return result, nil
}

func aliasFor(top bool, name string, aliases map[string]string) string {
	if !top || len(aliases) == 0 {
		return ""
	}
	return aliases[name]
}

func countChildren(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	return len(entries), nil
}

func countSkillCategories(root string) int {
	entries, err := os.ReadDir(filepath.Join(root, "skills"))
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			n++
		}
	}
	return n
}
