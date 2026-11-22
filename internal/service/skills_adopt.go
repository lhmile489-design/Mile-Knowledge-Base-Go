package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/skills"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/vault"
)

// M82 (API.md §12): adopt a tool skill into the knowledge base, optionally
// swapping the tool copy (and identical copies in other tools) for links.

// Adopt is M82.
func (s *SkillService) Adopt(ctx context.Context, vaultID, toolID, rel, category string, linkBack bool, extras []contract.SkillToolRef) contract.SkillAdoptResult {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if f := validateID(vaultID); f != nil {
		return contract.SkillAdoptResult{Failure: f}
	}
	if !skills.IsValidSegment(category) {
		return contract.SkillAdoptResult{Failure: contract.Fail(contract.CodeInvalidInput, "分类不合法")}
	}
	if len(extras) > 20 {
		return contract.SkillAdoptResult{Failure: contract.Fail(contract.CodeInvalidInput, "工具数量过多")}
	}
	root, f := s.app.Vaults.Root(vaultID)
	if f != nil {
		return contract.SkillAdoptResult{Failure: contract.Fail(contract.CodeVaultUnavailable, "知识库不可用")}
	}
	tool, sourceDir, f := s.resolveAnyToolSkill(ctx, toolID, rel)
	if f != nil {
		return contract.SkillAdoptResult{Failure: f}
	}
	info, err := os.Stat(sourceDir)
	if err != nil || !info.IsDir() {
		return contract.SkillAdoptResult{Failure: contract.Fail(contract.CodeNotFound, "技能不存在")}
	}
	name := filepath.Base(sourceDir)
	if !skills.IsValidSegment(name) {
		return contract.SkillAdoptResult{Failure: contract.Fail(contract.CodeInvalidInput, "技能名不合法")}
	}
	if files, _, err := skills.SafeWalk(sourceDir); err == nil {
		if len(files) > vault.MaxSkillFiles {
			return contract.SkillAdoptResult{Failure: contract.Fail(contract.CodeTooLarge, "技能文件过多")}
		}
		var size int64
		for _, file := range files {
			if info, err := os.Stat(filepath.Join(sourceDir, file)); err == nil {
				size += info.Size()
			}
		}
		if size > vault.MaxSkillBytes {
			return contract.SkillAdoptResult{Failure: contract.Fail(contract.CodeTooLarge, "技能超过大小上限")}
		}
	}
	targetDir := filepath.Join(root, "skills", category, name)
	if _, err := os.Stat(targetDir); err == nil {
		return contract.SkillAdoptResult{Failure: contract.Fail(contract.CodeSkillConflict, "知识库已有同名技能")}
	}
	// Extras must be byte-identical copies of the same skill.
	sourceHash, err := skills.TreeHash(sourceDir)
	if err != nil {
		return contract.SkillAdoptResult{Failure: contract.Fail(contract.CodeIOFailure, "读取技能失败")}
	}
	type extra struct {
		toolID string
		dir    string
	}
	var skipped []string
	resolvedExtras := make([]extra, 0, len(extras))
	for _, ref := range extras {
		otherTool, otherDir, f := s.resolveAnyToolSkill(ctx, ref.ToolID, ref.Path)
		if f != nil {
			return contract.SkillAdoptResult{Failure: f}
		}
		if otherTool.ID == tool.ID {
			continue
		}
		if hash, err := skills.TreeHash(otherDir); err != nil || hash != sourceHash {
			// Different content is skipped, not merged (PRD: 内容不同则分开).
			skipped = append(skipped, otherTool.ID+" 内容不一致，未合并")
			continue
		}
		resolvedExtras = append(resolvedExtras, extra{toolID: otherTool.ID, dir: otherDir})
	}

	result, err := skills.Adopt(skills.AdoptOptions{
		ToolID: tool.ID, SourceDir: sourceDir, TargetDir: targetDir,
		LinkBack: linkBack, Backups: s.backupStore(), Now: s.now,
	})
	switch {
	case errors.Is(err, skills.ErrAdoptExists):
		return contract.SkillAdoptResult{Failure: contract.Fail(contract.CodeSkillConflict, "知识库已有同名技能")}
	case err != nil:
		return contract.SkillAdoptResult{Failure: contract.Fail(contract.CodeIOFailure, "收编失败："+err.Error())}
	}
	out := contract.SkillAdoptResult{VaultPath: "skills/" + category + "/" + name, Linked: result.Linked}
	out.Warnings = append(out.Warnings, skipped...)
	if result.BackupID != "" {
		out.BackupIDs = append(out.BackupIDs, result.BackupID)
	}
	linkedTargets := make([]struct{ toolID, dir string }, 0, len(resolvedExtras)+1)
	if result.Linked {
		linkedTargets = append(linkedTargets, struct{ toolID, dir string }{tool.ID, sourceDir})
	}
	for _, other := range resolvedExtras {
		if !linkBack {
			continue
		}
		meta, _, err := s.backupStore().Create(other.toolID, other.dir, "adopt", s.now())
		if err != nil {
			out.Warnings = append(out.Warnings, other.toolID+" 换链前备份失败")
			continue
		}
		if err := skills.CreateJunction(other.dir, targetDir); err != nil {
			_ = os.Rename(filepath.Join(s.backupStore().Dir, meta.ID, "files"), other.dir)
			out.Warnings = append(out.Warnings, other.toolID+" 换链失败")
			continue
		}
		out.BackupIDs = append(out.BackupIDs, meta.ID)
		linkedTargets = append(linkedTargets, struct{ toolID, dir string }{other.toolID, other.dir})
	}
	if len(linkedTargets) > 0 {
		skillPath := "skills/" + category + "/" + name
		now := s.now()
		s.updateManifest(vaultID, func(m *skills.Manifest) {
			for _, linked := range linkedTargets {
				target, ok := s.manifestTarget(linked.toolID, linked.dir)
				if !ok {
					continue
				}
				m.Upsert(manifestKBItem(target, name, skillPath, "link", now))
			}
		})
	}
	_ = os.Remove(s.cachePath())
	s.emitChanged()
	mode := "copy"
	if linkBack {
		mode = "copy-adopt"
	}
	backupID := ""
	if len(out.BackupIDs) > 0 {
		backupID = out.BackupIDs[0]
	}
	s.logOperation("adopt", name, tool.ID, out.VaultPath, mode, "ok", backupID, "")
	return out
}
