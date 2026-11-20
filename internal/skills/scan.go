package skills

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// FR-SKM-SCAN (PRD §15.5; 技术设计 15.4): scan both skill sets and classify
// every tool skill by origin. Scanning is read-only; links are inspected but
// never followed for writing, and only one level below a link is ever read
// (through os.Stat/ReadFile for detection and frontmatter).

// KbSkillRef is one knowledge-base skill the scanner needs for link and copy
// comparison. Path is relative to the vault, e.g. "skills/前端/vue".
type KbSkillRef struct {
	Category string
	Name     string
	Path     string
}

// Distribution mirrors one record of skills/state.json (数据库设计 4.6).
type Distribution struct {
	ID         string
	VaultID    string
	SkillPath  string
	ToolID     string
	TargetPath string
	Mode       string
	Name       string
	BaseHash   string
}

// ScanOptions carries everything a scan needs; nothing is read from globals.
type ScanOptions struct {
	VaultID       string
	VaultRoot     string
	Kb            []contract.SkillCategory // grouped knowledge-base skills for the payload
	KbRefs        []KbSkillRef
	Tools         []DetectedTool
	Distributions []Distribution
	Now           func() time.Time
}

// ScanTools walks every detected tool's directories and returns one group per
// tool (registry order) plus warnings for directories that could not be read.
func ScanTools(opts ScanOptions) ([]contract.SkillToolScan, []contract.SkillScanWarning) {
	var groups []contract.SkillToolScan
	var warnings []contract.SkillScanWarning
	kbSkillsRoot := filepath.Join(opts.VaultRoot, "skills")
	byTool := map[string][]Distribution{}
	for _, dist := range opts.Distributions {
		if dist.VaultID != "" && opts.VaultID != "" && dist.VaultID != opts.VaultID {
			continue
		}
		byTool[dist.ToolID] = append(byTool[dist.ToolID], dist)
	}
	for _, tool := range opts.Tools {
		if tool.Status == StatusMissing && !tool.Shared {
			groups = append(groups, contract.SkillToolScan{ToolID: tool.ID})
			continue
		}
		group := contract.SkillToolScan{ToolID: tool.ID}
		seen := map[string]bool{}
		targets := make([]struct {
			dir     string
			builtin bool
		}, 0, len(tool.ScanDirs)+len(tool.BuiltinDirs))
		if tool.InstallDir != "" {
			targets = append(targets, struct {
				dir     string
				builtin bool
			}{tool.InstallDir, false})
		}
		for _, dir := range tool.ScanDirs {
			targets = append(targets, struct {
				dir     string
				builtin bool
			}{dir, false})
		}
		for _, dir := range tool.BuiltinDirs {
			targets = append(targets, struct {
				dir     string
				builtin bool
			}{dir, true})
		}
		for _, target := range targets {
			for _, root := range expandWildcard(target.dir) {
				found, warns := scanRoot(opts, tool, root, target.builtin, byTool[tool.ID], kbSkillsRoot)
				warnings = append(warnings, warns...)
				for _, skill := range found {
					key := strings.ToLower(skill.Path)
					if seen[key] {
						continue
					}
					seen[key] = true
					group.Skills = append(group.Skills, skill)
				}
			}
		}
		sort.Slice(group.Skills, func(i, j int) bool {
			return strings.ToLower(group.Skills[i].Name) < strings.ToLower(group.Skills[j].Name)
		})
		groups = append(groups, group)
	}
	return groups, warnings
}

// expandWildcard expands one "*" path segment, e.g. ZCode's
// resources\glm\packages\*\skills. Without a "*" it returns the path itself.
func expandWildcard(dir string) []string {
	parts := strings.Split(dir, string(filepath.Separator))
	index := -1
	for i, part := range parts {
		if part == "*" {
			index = i
			break
		}
	}
	if index < 0 {
		return []string{dir}
	}
	parent := strings.Join(parts[:index], string(filepath.Separator))
	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil
	}
	var out []string
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		child := filepath.Join(parent, entry.Name())
		if kind, _, err := Classify(child); err != nil || kind != KindDir {
			continue
		}
		out = append(out, filepath.Join(append([]string{child}, parts[index+1:]...)...))
	}
	return out
}

func scanRoot(opts ScanOptions, tool DetectedTool, root string, builtin bool, dists []Distribution, kbSkillsRoot string) ([]contract.ToolSkill, []contract.SkillScanWarning) {
	var skills []contract.ToolSkill
	var warnings []contract.SkillScanWarning
	if root == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if !os.IsNotExist(err) {
			warnings = append(warnings, contract.SkillScanWarning{ToolID: tool.ID, Text: "目录无法读取：" + filepath.Base(root)})
		}
		return nil, warnings
	}
	var handle func(dir string, depth int)
	handle = func(dir string, depth int) {
		kind, target, err := Classify(dir)
		if err != nil {
			return
		}
		name := filepath.Base(dir)
		switch kind {
		case KindJunction, KindSymlink:
			// A link is listed as a skill (its target is read-only); it is
			// never descended into.
			exists := pathExists(dir)
			skill := contract.ToolSkill{Name: name, Path: dir, ToolID: tool.ID, Target: target}
			switch {
			case !exists:
				skill.Origin = "link-broken"
			case kbSkillsRoot != "" && UnderRoot(target, kbSkillsRoot):
				skill.Origin = "linked"
			default:
				skill.Origin = "external-link"
			}
			skill.Description = SkillDescription(dir)
			skills = append(skills, skill)
			return
		case KindDir:
			if hasSkillFile(dir) {
				skill := contract.ToolSkill{Name: name, Path: dir, ToolID: tool.ID, Builtin: builtin}
				switch {
				case builtin:
					skill.Origin = "builtin"
				default:
					if dist, ok := matchDistribution(dists, dir); ok {
						skill.Origin = "copied"
						skill.Sync = copySyncState(opts, dist, dir)
					} else {
						skill.Origin = "independent"
					}
				}
				skill.Description = SkillDescription(dir)
				skills = append(skills, skill)
			}
		default:
			return
		}
		if depth >= 2 {
			return
		}
		children, err := os.ReadDir(dir)
		if err != nil {
			warnings = append(warnings, contract.SkillScanWarning{ToolID: tool.ID, Text: "目录无法读取：" + filepath.Base(dir)})
			return
		}
		for _, child := range children {
			if skipEntry(child.Name()) {
				continue
			}
			handle(filepath.Join(dir, child.Name()), depth+1)
		}
	}
	for _, entry := range entries {
		if skipEntry(entry.Name()) {
			continue
		}
		handle(filepath.Join(root, entry.Name()), 1)
	}
	return skills, warnings
}

func skipEntry(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasPrefix(lower, ".") || lower == "node_modules"
}

func hasSkillFile(dir string) bool {
	return pathExists(filepath.Join(dir, "SKILL.md")) || pathExists(filepath.Join(dir, "AGENTS.md"))
}

func matchDistribution(dists []Distribution, dir string) (Distribution, bool) {
	for _, dist := range dists {
		if strings.EqualFold(filepath.Clean(dist.TargetPath), filepath.Clean(dir)) {
			return dist, true
		}
	}
	return Distribution{}, false
}

// copySyncState compares the copy against its recorded baseline and the
// knowledge base (FR-SKM-DIST; R-9 C16).
func copySyncState(opts ScanOptions, dist Distribution, dir string) string {
	toolHash, err := TreeHash(dir)
	if err != nil {
		return "missing"
	}
	kbHash := ""
	if opts.VaultRoot != "" && dist.SkillPath != "" {
		if hash, err := TreeHash(filepath.Join(opts.VaultRoot, filepath.FromSlash(dist.SkillPath))); err == nil {
			kbHash = hash
		}
	}
	base := dist.BaseHash
	switch {
	case kbHash == "":
		return "vault-updated"
	case toolHash == kbHash:
		return "in-sync"
	case toolHash == base && kbHash != base:
		return "vault-updated"
	case kbHash == base && toolHash != base:
		return "tool-updated"
	default:
		return "both-updated"
	}
}

// SkillDescription reads the frontmatter description of SKILL.md /
// AGENTS.md (first 4 KiB, 技术设计 15.3).
func SkillDescription(dir string) string {
	for _, name := range []string{"SKILL.md", "AGENTS.md"} {
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		text := string(body)
		if len(text) > 4096 {
			text = text[:4096]
		}
		if !strings.HasPrefix(text, "---") {
			return ""
		}
		lines := strings.Split(text, "\n")
		for i := 1; i < len(lines); i++ {
			line := strings.TrimSpace(lines[i])
			if line == "---" || line == "..." {
				break
			}
			if value, ok := strings.CutPrefix(line, "description:"); ok {
				return strings.Trim(strings.TrimSpace(value), `"'`)
			}
		}
		return ""
	}
	return ""
}
