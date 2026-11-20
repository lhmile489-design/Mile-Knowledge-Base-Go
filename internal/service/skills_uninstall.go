package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/skills"
)

// M80/M81/M83/M85/M86 (API.md §12): uninstall, restore and backup upkeep.

func (s *SkillService) backupStore() *skills.BackupStore {
	return &skills.BackupStore{Dir: filepath.Join(s.app.DataDir, "skills", "backups")}
}

// resolveToolSkill validates toolId + tool-relative path and rejects builtin
// directories (PRD FR-SKM-UNINST: builtin skills are read-only).
func (s *SkillService) resolveToolSkill(ctx context.Context, toolID, rel string) (skills.DetectedTool, string, *contract.Failure) {
	detector := s.detector()
	for _, tool := range detector.Detect() {
		if tool.ID != toolID {
			continue
		}
		if !tool.Enabled {
			return tool, "", contract.Fail(contract.CodeSkillProtected, "该工具未启用")
		}
		// The path may point into a builtin directory too; those are readable
		// but never removable (PRD FR-SKM-UNINST).
		for _, builtin := range tool.BuiltinDirs {
			if abs, err := skills.ResolveUnder(builtin, rel); err == nil {
				_ = abs
				return tool, "", contract.Fail(contract.CodeSkillProtected, "内置技能只读")
			}
		}
		if tool.InstallDir == "" {
			return tool, "", contract.Fail(contract.CodeSkillProtected, "该工具没有安装目录")
		}
		abs, err := skills.ResolveUnder(tool.InstallDir, rel)
		if err != nil {
			return tool, "", contract.Fail(contract.CodeInvalidInput, "技能路径不合法")
		}
		return tool, abs, nil
	}
	return skills.DetectedTool{}, "", contract.Fail(contract.CodeInvalidInput, "未知的工具")
}

// UninstallPreview is M80.
func (s *SkillService) UninstallPreview(ctx context.Context, toolID, rel string) contract.SkillUninstallResult {
	tool, abs, f := s.resolveToolSkill(ctx, toolID, rel)
	if f != nil {
		return contract.SkillUninstallResult{Failure: f}
	}
	preview, err := skills.PreviewUninstall(s.backupStore(), abs)
	if err != nil {
		return contract.SkillUninstallResult{Failure: contract.Fail(contract.CodeNotFound, "技能不存在")}
	}
	kind := preview.Kind
	if kind == "independent" {
		state := skills.LoadState(s.statePath())
		for _, dist := range state.Distributions {
			if dist.ToolID == tool.ID && strings.EqualFold(filepath.Clean(dist.TargetPath), filepath.Clean(abs)) {
				kind = "copy"
				break
			}
		}
	}
	result := contract.SkillUninstallPreview{
		Kind:       kind,
		Target:     redactToolPath(preview.Target, s.detector()),
		BackupPath: "",
	}
	if preview.BackupPath != "" {
		result.BackupPath = filepath.Join(s.backupStore().Dir, "<new>", "files")
	}
	if tool.Shared {
		for _, reader := range tool.Readers {
			for _, other := range s.detector().Detect() {
				if other.ID == reader && other.Status == skills.StatusDetected {
					result.AffectedTools = append(result.AffectedTools, reader)
					break
				}
			}
		}
	}
	return contract.SkillUninstallResult{Preview: result}
}

// Uninstall is M81.
func (s *SkillService) Uninstall(ctx context.Context, toolID, rel string) contract.SkillOpResult {
	_, abs, f := s.resolveToolSkill(ctx, toolID, rel)
	if f != nil {
		return contract.SkillOpResult{Failure: f}
	}
	backupID, err := skills.ExecuteUninstall(s.backupStore(), toolID, abs, s.now())
	item := contract.SkillOpItem{TargetID: toolID, ToolID: toolID}
	if err != nil {
		item.Result = "failed"
		item.Detail = err.Error()
		return contract.SkillOpResult{Items: []contract.SkillOpItem{item}}
	}
	item.Result = "removed"
	item.BackupID = backupID
	state := skills.LoadState(s.statePath())
	kept := state.Distributions[:0]
	for _, dist := range state.Distributions {
		if dist.ToolID == toolID && strings.EqualFold(filepath.Clean(dist.TargetPath), filepath.Clean(abs)) {
			continue
		}
		kept = append(kept, dist)
	}
	state.Distributions = kept
	if err := skills.SaveState(s.statePath(), *state); err != nil {
		return contract.SkillOpResult{Items: []contract.SkillOpItem{item}, Failure: contract.Fail(contract.CodeIOFailure, "分发记录写入失败")}
	}
	_ = os.Remove(s.cachePath())
	s.emitChanged()
	return contract.SkillOpResult{Items: []contract.SkillOpItem{item}}
}

// Restore is M83.
func (s *SkillService) Restore(ctx context.Context, backupID string) contract.SkillRestoreResult {
	restored, err := skills.RestoreBackup(s.backupStore(), backupID)
	switch {
	case errors.Is(err, skills.ErrRestoreConflict):
		return contract.SkillRestoreResult{Failure: contract.Fail(contract.CodeSkillConflict, "原位置已有同名目录")}
	case errors.Is(err, skills.ErrBackupMissing):
		return contract.SkillRestoreResult{Failure: contract.Fail(contract.CodeBackupMissing, "备份不存在或已清理")}
	case err != nil:
		return contract.SkillRestoreResult{Failure: contract.Fail(contract.CodeIOFailure, "恢复失败")}
	}
	_ = os.Remove(s.cachePath())
	s.emitChanged()
	return contract.SkillRestoreResult{RestoredTo: redactToolPath(restored, s.detector())}
}

// Backups is M85.
func (s *SkillService) Backups(ctx context.Context) contract.SkillBackupsResult {
	items := skills.ListBackups(s.backupStore())
	out := make([]contract.SkillBackup, 0, len(items))
	for _, item := range items {
		out = append(out, backupContract(item, s.detector()))
	}
	return contract.SkillBackupsResult{Items: out}
}

// CleanBackups is M86.
func (s *SkillService) CleanBackups(ctx context.Context, ids []string) contract.SkillBackupsResult {
	if len(ids) == 0 || len(ids) > 200 {
		return contract.SkillBackupsResult{Failure: contract.Fail(contract.CodeInvalidInput, "备份数量不合法")}
	}
	if err := skills.CleanBackups(s.backupStore(), ids); err != nil {
		return contract.SkillBackupsResult{Failure: contract.Fail(contract.CodeIOFailure, "清理备份失败")}
	}
	return s.Backups(ctx)
}

func backupContract(meta skills.BackupMeta, detector *skills.Detector) contract.SkillBackup {
	return contract.SkillBackup{
		ID:           meta.ID,
		ToolID:       meta.ToolID,
		OriginalPath: redactToolPath(meta.OriginalPath, detector),
		Kind:         meta.Kind,
		CreatedAt:    meta.CreatedAt,
		Size:         meta.Size,
		Files:        meta.Files,
		Restorable:   true,
	}
}
