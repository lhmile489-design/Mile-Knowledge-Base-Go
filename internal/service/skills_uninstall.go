package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/skills"
)

// M80/M81/M83/M85/M86 (API.md §12): uninstall, restore and backup upkeep.

func (s *SkillService) backupStore() *skills.BackupStore {
	return &skills.BackupStore{Dir: filepath.Join(s.app.DataDir, "skills", "backups")}
}

// parseToolSpec splits the wire path "install:<rel>", "scan:<n>:<rel>",
// "builtin:<n>:<rel>" or a legacy bare "<rel>" (API.md §12.3).
func parseToolSpec(spec string) (kind string, index int, rel string) {
	parts := strings.SplitN(spec, ":", 3)
	if len(parts) >= 2 && (parts[0] == "install" || parts[0] == "scan" || parts[0] == "builtin") {
		if parts[0] == "install" {
			return "install", -1, strings.Join(parts[1:], ":")
		}
		if len(parts) == 3 {
			n, err := strconv.Atoi(parts[1])
			if err != nil {
				return "invalid", -1, ""
			}
			return parts[0], n, parts[2]
		}
		return "invalid", -1, ""
	}
	return "", -1, spec
}

func validRel(rel string) bool {
	segments := strings.Split(rel, "/")
	if len(segments) < 1 || len(segments) > 2 {
		return false
	}
	for _, segment := range segments {
		if !skills.IsValidSegment(segment) {
			return false
		}
	}
	return true
}

// resolveSpec turns a wire path into an absolute path inside one of the
// tool's registered roots. allowBuiltin lets Reveal/Adopt reach builtin dirs;
// removal paths pass false and get skill-protected instead.
func (s *SkillService) resolveSpec(tool skills.DetectedTool, spec string, allowBuiltin bool) (string, *contract.Failure) {
	// Project rows carry "project:<dir>:<rel>" (API §14.3, M6 T05).
	if dir, rel, ok := parseProjectSpec(spec); ok {
		if rel == "" || !validRel(rel) {
			return "", contract.Fail(contract.CodeInvalidInput, "技能路径不合法")
		}
		root, ok := projectRoot(tool, dir, allowBuiltin)
		if !ok {
			return "", contract.Fail(contract.CodeInvalidInput, "技能路径不合法")
		}
		if abs, err := skills.ResolveUnder(root, rel); err == nil {
			return abs, nil
		}
		return "", contract.Fail(contract.CodeInvalidInput, "技能路径不合法")
	}
	kind, index, rel := parseToolSpec(spec)
	if kind == "invalid" || rel == "" || !validRel(rel) {
		return "", contract.Fail(contract.CodeInvalidInput, "技能路径不合法")
	}
	resolve := func(base string) (string, bool) {
		if base == "" {
			return "", false
		}
		abs, err := skills.ResolveUnder(base, rel)
		return abs, err == nil
	}
	switch kind {
	case "install":
		if abs, ok := resolve(tool.InstallDir); ok {
			return abs, nil
		}
		return "", contract.Fail(contract.CodeSkillProtected, "该工具没有安装目录")
	case "scan":
		if index < 0 || index >= len(tool.ScanDirs) {
			return "", contract.Fail(contract.CodeInvalidInput, "技能路径不合法")
		}
		if abs, ok := resolve(tool.ScanDirs[index]); ok {
			return abs, nil
		}
		return "", contract.Fail(contract.CodeInvalidInput, "技能路径不合法")
	case "builtin":
		if index < 0 || index >= len(tool.BuiltinDirs) {
			return "", contract.Fail(contract.CodeInvalidInput, "技能路径不合法")
		}
		abs, ok := resolve(tool.BuiltinDirs[index])
		if !ok {
			return "", contract.Fail(contract.CodeInvalidInput, "技能路径不合法")
		}
		if !allowBuiltin {
			return "", contract.Fail(contract.CodeSkillProtected, "内置技能只读")
		}
		return abs, nil
	default:
		// Legacy bare name: find the root that actually holds it.
		if abs, ok := resolve(tool.InstallDir); ok && pathExists(abs) {
			return abs, nil
		}
		for _, pd := range tool.ProjectDirs {
			if abs, ok := resolve(pd.Dir); ok && pathExists(abs) {
				return abs, nil
			}
		}
		for _, dir := range tool.ScanDirs {
			if abs, ok := resolve(dir); ok && pathExists(abs) {
				return abs, nil
			}
		}
		for _, dir := range tool.BuiltinDirs {
			if abs, ok := resolve(dir); ok && pathExists(abs) {
				if !allowBuiltin {
					return "", contract.Fail(contract.CodeSkillProtected, "内置技能只读")
				}
				return abs, nil
			}
		}
		return "", contract.Fail(contract.CodeNotFound, "技能不存在")
	}
}

// resolveToolSkill validates toolId + wire path for removal paths and rejects
// builtin directories (PRD FR-SKM-UNINST: builtin skills are read-only).
func (s *SkillService) resolveToolSkill(ctx context.Context, toolID, spec string) (skills.DetectedTool, string, *contract.Failure) {
	for _, tool := range s.skillTools() {
		if tool.ID != toolID {
			continue
		}
		if !tool.Enabled {
			return tool, "", contract.Fail(contract.CodeSkillProtected, "该工具未启用")
		}
		abs, f := s.resolveSpec(tool, spec, false)
		return tool, abs, f
	}
	return skills.DetectedTool{}, "", contract.Fail(contract.CodeInvalidInput, "未知的工具")
}

// resolveAnyToolSkill is the read-side variant used by Reveal and Adopt: it
// may point into a builtin directory, and disabled tools are still readable.
func (s *SkillService) resolveAnyToolSkill(ctx context.Context, toolID, spec string) (skills.DetectedTool, string, *contract.Failure) {
	for _, tool := range s.skillTools() {
		if tool.ID != toolID {
			continue
		}
		abs, f := s.resolveSpec(tool, spec, true)
		return tool, abs, f
	}
	return skills.DetectedTool{}, "", contract.Fail(contract.CodeInvalidInput, "未知的工具")
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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
	s.opMu.Lock()
	defer s.opMu.Unlock()
	_, abs, f := s.resolveToolSkill(ctx, toolID, rel)
	if f != nil {
		return contract.SkillOpResult{Failure: f}
	}
	backupID, err := skills.ExecuteUninstall(s.backupStore(), toolID, abs, s.now())
	item := contract.SkillOpItem{TargetID: toolID, ToolID: toolID}
	if err != nil {
		item.Result = "failed"
		item.Detail = err.Error()
		s.logOperation("uninstall", filepath.Base(abs), toolID, abs, "", "failed", "", err.Error())
		return contract.SkillOpResult{Items: []contract.SkillOpItem{item}}
	}
	item.Result = "removed"
	item.BackupID = backupID
	s.logOperation("uninstall", filepath.Base(abs), toolID, abs, "", "ok", backupID, "")
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
	s.opMu.Lock()
	defer s.opMu.Unlock()
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
	s.logOperation("restore", filepath.Base(restored), "", restored, "", "ok", backupID, "")
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
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if len(ids) == 0 || len(ids) > 200 {
		return contract.SkillBackupsResult{Failure: contract.Fail(contract.CodeInvalidInput, "备份数量不合法")}
	}
	if err := skills.CleanBackups(s.backupStore(), ids); err != nil {
		return contract.SkillBackupsResult{Failure: contract.Fail(contract.CodeIOFailure, "清理备份失败")}
	}
	s.logOperation("clean", "", "", fmt.Sprintf("%d 个备份", len(ids)), "", "ok", "", "")
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

// Reveal opens Explorer at a tool skill directory (M87). The path is resolved
// through the same guard as removal; nothing is written.
func (s *SkillService) Reveal(ctx context.Context, toolID, rel string) contract.Failure {
	_, abs, f := s.resolveAnyToolSkill(ctx, toolID, rel)
	if f != nil {
		return *f
	}
	if _, err := os.Stat(abs); err != nil {
		return *contract.Fail(contract.CodeNotFound, "技能不存在")
	}
	if err := exec.Command("explorer.exe", abs).Start(); err != nil {
		return *contract.Fail(contract.CodeIOFailure, "打开目录失败")
	}
	return contract.Failure{}
}
