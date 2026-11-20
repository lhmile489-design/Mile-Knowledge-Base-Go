package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/skills"
)

// M78/M79 (API.md §12): distribute a knowledge-base skill to tool
// directories. The service resolves the targets and the cross-category name
// rule; the skills package does the file work (技术设计 15.5).

func (s *SkillService) statePath() string {
	return filepath.Join(s.app.DataDir, "skills", "state.json")
}

// DistributePreview is M78.
func (s *SkillService) DistributePreview(ctx context.Context, vaultID, skillPath string, targets []contract.SkillDistributeTarget, mode string) contract.SkillPreviewResult {
	opts, f := s.distributeOptions(ctx, vaultID, skillPath, targets, mode)
	if f != nil {
		return contract.SkillPreviewResult{Failure: f}
	}
	return contract.SkillPreviewResult{Items: skills.PreviewDistribute(opts)}
}

// Distribute is M79.
func (s *SkillService) Distribute(ctx context.Context, vaultID, skillPath string, targets []contract.SkillDistributeTarget, mode string) contract.SkillOpResult {
	opts, f := s.distributeOptions(ctx, vaultID, skillPath, targets, mode)
	if f != nil {
		return contract.SkillOpResult{Failure: f}
	}
	items, records := skills.ExecuteDistribute(opts)
	locations := map[string]string{}
	for _, target := range opts.Targets {
		locations[target.TargetID] = filepath.Join(target.InstallDir, target.Name)
	}
	s.logDistributeRecords(opts.Targets[0].Name, items, locations)
	if len(records) > 0 {
		state := skills.LoadState(s.statePath())
		for _, record := range records {
			state.AddDistribution(record)
		}
		if err := skills.SaveState(s.statePath(), *state); err != nil {
			return contract.SkillOpResult{Items: items, Failure: contract.Fail(contract.CodeIOFailure, "分发记录写入失败")}
		}
		// The next scan must see the new state instead of a stale cache.
		_ = os.Remove(s.cachePath())
		s.emitChanged()
	}
	return contract.SkillOpResult{Items: items}
}

func (s *SkillService) emitChanged() {
	s.mu.Lock()
	s.seq++
	seq := s.seq
	s.mu.Unlock()
	if window := s.app.Window(); window != nil {
		_ = SkillsChanged.Emit(window, contract.SkillsChangedEvent{Sequence: seq})
	}
}

// distributeOptions validates the request and resolves registry targets.
func (s *SkillService) distributeOptions(ctx context.Context, vaultID, skillPath string, targets []contract.SkillDistributeTarget, mode string) (skills.DistributeOptions, *contract.Failure) {
	if mode != "link" && mode != "copy" {
		return skills.DistributeOptions{}, contract.Fail(contract.CodeInvalidInput, "分发方式不合法")
	}
	if len(targets) == 0 || len(targets) > 25 {
		return skills.DistributeOptions{}, contract.Fail(contract.CodeInvalidInput, "目标数量不合法")
	}
	if f := validateID(vaultID); f != nil {
		return skills.DistributeOptions{}, f
	}
	root, f := s.app.Vaults.Root(vaultID)
	if f != nil {
		return skills.DistributeOptions{}, contract.Fail(contract.CodeVaultUnavailable, "知识库不可用")
	}
	parts := strings.Split(skillPath, "/")
	if len(parts) != 3 || parts[0] != "skills" || !skills.IsValidSegment(parts[1]) || !skills.IsValidSegment(parts[2]) {
		return skills.DistributeOptions{}, contract.Fail(contract.CodeInvalidInput, "技能路径不合法")
	}
	name := parts[2]
	sourceDir := filepath.Join(root, filepath.FromSlash(skillPath))
	if info, err := os.Stat(sourceDir); err != nil || !info.IsDir() {
		return skills.DistributeOptions{}, contract.Fail(contract.CodeNotFound, "技能不存在")
	}
	detector := s.detector()
	tools := detector.Detect()
	byID := map[string]skills.DetectedTool{}
	readers := map[string]bool{}
	for _, tool := range tools {
		byID[tool.ID] = tool
		if tool.Shared {
			for _, reader := range tool.Readers {
				readers[reader] = true
			}
		}
	}
	state := skills.LoadState(s.statePath())
	_, refs, _ := s.kbSkills(ctx, vaultID)
	distributedTo := func(otherPath, toolID string) bool {
		for _, dist := range state.Distributions {
			if dist.VaultID == vaultID && dist.SkillPath == otherPath && dist.ToolID == toolID {
				return true
			}
		}
		return false
	}
	resolved := make([]skills.DistributeTarget, 0, len(targets))
	sharedTargeted := false
	for _, input := range targets {
		tool, ok := byID[input.TargetID]
		if !ok {
			return skills.DistributeOptions{}, contract.Fail(contract.CodeInvalidInput, "未知的工具")
		}
		if !tool.Enabled {
			return skills.DistributeOptions{}, contract.Fail(contract.CodeSkillProtected, "该工具未启用")
		}
		if !tool.Shared && tool.Status != skills.StatusDetected {
			return skills.DistributeOptions{}, contract.Fail(contract.CodeSkillProtected, "该工具未检测到")
		}
		if tool.InstallDir == "" {
			return skills.DistributeOptions{}, contract.Fail(contract.CodeSkillProtected, "该工具没有安装目录")
		}
		if tool.Shared {
			sharedTargeted = true
		}
		conflict := ""
		for _, ref := range refs {
			if ref.Name != name || ref.Path == skillPath {
				continue
			}
			if distributedTo(ref.Path, input.TargetID) {
				conflict = "同名技能已在 " + ref.Path
				break
			}
		}
		resolved = append(resolved, skills.DistributeTarget{
			TargetID:       input.TargetID,
			ToolID:         input.TargetID,
			Name:           name,
			InstallDir:     tool.InstallDir,
			ReadsShared:    readers[input.TargetID],
			AllowOverwrite: input.AllowOverwrite,
			Conflict:       conflict,
		})
	}
	return skills.DistributeOptions{
		VaultID:        vaultID,
		SkillPath:      skillPath,
		SourceDir:      sourceDir,
		Mode:           mode,
		Targets:        resolved,
		Backups:        &skills.BackupStore{Dir: filepath.Join(s.app.DataDir, "skills", "backups")},
		Now:            s.now,
		SharedTargeted: sharedTargeted,
		Distributions:  state.Distributions,
	}, nil
}
