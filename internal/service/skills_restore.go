package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/skills"
)

// M7 T03: M102 restorePreview judges the vault manifest against this machine
// (技术设计 18.4/18.5). Read-only: no network, no writes.

// RestorePreview is M102.
func (s *SkillService) RestorePreview(ctx context.Context, vaultID string) contract.SkillRestorePreviewResult {
	if f := validateID(vaultID); f != nil {
		return contract.SkillRestorePreviewResult{Failure: f}
	}
	root, f := s.app.Vaults.Root(vaultID)
	if f != nil {
		return contract.SkillRestorePreviewResult{Failure: contract.Fail(contract.CodeVaultUnavailable, "知识库不可用")}
	}
	manifest := skills.LoadManifest(root)
	preview := &contract.SkillRestorePreview{
		Status:     manifest.Status,
		UpdatedAt:  manifest.UpdatedAt,
		Items:      []contract.SkillRestoreItem{},
		LocalOnly:  []contract.SkillRestoreLocal{},
		Unmatched:  []string{},
		Warnings:   append([]string{}, manifest.Warnings...),
		CanRebuild: manifest.Status != skills.ManifestVersionUnknown,
	}
	if s.manifestStaleFlag() {
		preview.Warnings = append(preview.Warnings, "分发清单未能更新")
	}
	if preview.Status != skills.ManifestOK {
		return contract.SkillRestorePreviewResult{Preview: preview}
	}

	facts, unmatched := s.restoreFacts(root, vaultID)
	judgment := skills.JudgeManifest(manifest, facts)
	preview.Items = judgment.Items
	preview.LocalOnly = judgment.LocalOnly
	preview.Unmatched = *unmatched
	for _, item := range judgment.Items {
		if item.State == "create" || item.State == "conflict" {
			preview.Pending++
		}
	}
	return contract.SkillRestorePreviewResult{Preview: preview}
}

// restoreFacts assembles what the judge needs (技术设计 18.4). It returns the
// registration names that could not be matched, deduplicated.
func (s *SkillService) restoreFacts(root, vaultID string) (skills.RestoreFacts, *[]string) {
	tools := s.skillTools()
	byID := make(map[string]skills.DetectedTool, len(tools))
	for _, tool := range tools {
		byID[tool.ID] = tool
	}
	registrations := s.loadTargets().Targets
	distributions := skills.LoadState(s.statePath()).Distributions
	sources := skills.LoadToolSources(filepath.Join(s.app.DataDir, "skills"))

	builtinDirs := map[string][]string{}
	for _, tool := range s.detector().Detect() {
		if len(tool.BuiltinDirs) > 0 {
			builtinDirs[tool.ID] = tool.BuiltinDirs
		}
	}
	sourceAt := make(map[string]skills.ToolSource, len(sources.Records))
	for _, record := range sources.Records {
		sourceAt[restoreComparable(record.TargetPath)] = record
	}

	unmatched := []string{}
	unmatchedSeen := map[string]bool{}
	resolve := func(target skills.ManifestTarget) (skills.RestoreDestination, string) {
		switch target.Kind {
		case "builtin":
			tool, ok := byID[target.ToolID]
			if !ok {
				return skills.RestoreDestination{}, "工具不在本机注册表"
			}
			if !tool.Enabled {
				return skills.RestoreDestination{}, "工具未启用"
			}
			if !tool.Shared && tool.Status != skills.StatusDetected {
				return skills.RestoreDestination{}, "工具在本机未检测到"
			}
			if tool.Missing {
				return skills.RestoreDestination{}, "登记目录不存在"
			}
			if tool.InstallDir == "" {
				return skills.RestoreDestination{}, "该工具没有安装目录"
			}
			return skills.RestoreDestination{ToolID: tool.ID, Label: tool.Name, Dir: tool.InstallDir}, ""
		case "custom", "project":
			for _, record := range registrations {
				if record.Kind != target.Kind || !strings.EqualFold(record.Name, target.Name) {
					continue
				}
				if !isDirectoryQuick(record.Path) {
					return skills.RestoreDestination{}, "登记目录不存在"
				}
				if record.Kind == skills.TargetProject {
					if !projectSubdirKnown(target.Subdir) {
						return skills.RestoreDestination{}, "子目录不在项目的约定目录里"
					}
					return skills.RestoreDestination{
						ToolID: record.ID,
						Label:  "项目 " + record.Name + " · " + target.Subdir,
						Dir:    filepath.Join(record.Path, filepath.FromSlash(target.Subdir)),
					}, ""
				}
				return skills.RestoreDestination{ToolID: record.ID, Label: record.Name, Dir: record.Path}, ""
			}
			if !unmatchedSeen[target.Name] {
				unmatchedSeen[target.Name] = true
				unmatched = append(unmatched, target.Name)
			}
			return skills.RestoreDestination{}, "本机没有登记「" + target.Name + "」"
		}
		return skills.RestoreDestination{}, "目标类型不认识"
	}

	facts := skills.RestoreFacts{
		Resolve:    resolve,
		SkillDir:   func(skillPath string) string { return vaultSkillDir(root, skillPath) },
		DirExists:  isDirectoryQuick,
		LinkTarget: restoreLinkTarget,
		SourceAt: func(dir string) (skills.ToolSource, bool) {
			record, ok := sourceAt[restoreComparable(dir)]
			return record, ok
		},
		SourceRecords: sources.Records,
		Distributions: distributions,
		VaultID:       vaultID,
		IsBuiltinDir: func(toolID, dir string) bool {
			for _, base := range builtinDirs[toolID] {
				if dirWithin(dir, base) {
					return true
				}
			}
			return false
		},
		LabelFor: func(toolID string) string {
			if tool, ok := byID[toolID]; ok {
				return tool.Name
			}
			for _, record := range registrations {
				if record.ID == toolID {
					return record.Name
				}
			}
			return ""
		},
	}
	return facts, &unmatched
}

func restoreComparable(path string) string {
	return strings.ToLower(filepath.Clean(path))
}

func dirWithin(path, base string) bool {
	path = restoreComparable(path)
	base = restoreComparable(base)
	return path == base || strings.HasPrefix(path, base+string(filepath.Separator))
}

func vaultSkillDir(root, skillPath string) string {
	dir := filepath.Join(root, filepath.FromSlash(skillPath))
	for _, name := range []string{"SKILL.md", "AGENTS.md"} {
		if info, err := os.Stat(filepath.Join(dir, name)); err == nil && !info.IsDir() {
			return dir
		}
	}
	return ""
}

func restoreLinkTarget(dir string) (string, bool) {
	kind, linkedTo, err := skills.Classify(dir)
	if err != nil || (kind != skills.KindJunction && kind != skills.KindSymlink) {
		return "", false
	}
	return linkedTo, true
}

func projectSubdirKnown(subdir string) bool {
	if subdir == "" {
		return false
	}
	for _, rel := range skills.ProjectSkillDirs() {
		if rel == subdir {
			return true
		}
	}
	return false
}
