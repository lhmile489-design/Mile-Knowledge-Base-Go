package service

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/skills"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/vault"
)

// InstallPreview is M90: what M91 would write for the picked candidates.
func (s *SkillService) InstallPreview(ctx context.Context, vaultID, stageID string, ids []string, target contract.SkillInstallTarget) contract.SkillPreviewResult {
	stage, picked, f := s.stageSelection(ctx, vaultID, stageID, ids)
	if f != nil {
		return contract.SkillPreviewResult{Failure: f}
	}
	if target.Kind == "kb" {
		if !skills.IsValidSegment(target.Category) {
			return contract.SkillPreviewResult{Failure: contract.Fail(contract.CodeInvalidInput, "分类名不合法")}
		}
		root, f := s.app.Vaults.Root(vaultID)
		if f != nil {
			return contract.SkillPreviewResult{Failure: contract.Fail(contract.CodeVaultUnavailable, "知识库不可用")}
		}
		items := make([]contract.SkillPreviewItem, 0, len(picked))
		for _, candidate := range picked {
			targetPath := "skills/" + target.Category + "/" + candidate.Name
			action := "create"
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(targetPath))); err == nil {
				action = "exists"
			}
			items = append(items, contract.SkillPreviewItem{
				TargetID: candidate.ID, ToolID: "kb", Name: candidate.Name,
				Action: action, TargetPath: targetPath,
			})
		}
		return contract.SkillPreviewResult{Items: items}
	}
	if target.Kind != "tools" {
		return contract.SkillPreviewResult{Failure: contract.Fail(contract.CodeInvalidInput, "安装目标不合法")}
	}
	targets, f := s.resolveInstallTargets(target.Targets)
	if f != nil {
		return contract.SkillPreviewResult{Failure: f}
	}
	items := make([]contract.SkillPreviewItem, 0, len(picked)*len(targets))
	for _, candidate := range picked {
		sourceDir := filepath.Join(stage.Root(), filepath.FromSlash(candidate.ID))
		perTarget := skills.PreviewDistribute(skills.DistributeOptions{
			SourceDir: sourceDir,
			Mode:      "copy",
			Targets:   withName(targets, candidate.Name),
			Backups:   s.backupStore(),
			Now:       s.now,
		})
		for index := range perTarget {
			perTarget[index].Name = candidate.Name
		}
		items = append(items, perTarget...)
	}
	return contract.SkillPreviewResult{Items: items}
}

// Install is M91.
func (s *SkillService) Install(ctx context.Context, vaultID, stageID string, ids []string, target contract.SkillInstallTarget) contract.SkillOpResult {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	stage, picked, f := s.stageSelection(ctx, vaultID, stageID, ids)
	if f != nil {
		return contract.SkillOpResult{Failure: f}
	}
	store := s.stageStore()
	var items []contract.SkillOpItem
	success := false

	switch target.Kind {
	case "kb":
		if !skills.IsValidSegment(target.Category) {
			return contract.SkillOpResult{Failure: contract.Fail(contract.CodeInvalidInput, "分类名不合法")}
		}
		root, f := s.app.Vaults.Root(vaultID)
		if f != nil {
			return contract.SkillOpResult{Failure: contract.Fail(contract.CodeVaultUnavailable, "知识库不可用")}
		}
		preview := &vault.SkillSourcePreview{Dirs: map[string]string{}}
		names := make([]string, 0, len(picked))
		for _, candidate := range picked {
			preview.Dirs[candidate.Name] = candidate.ID
			names = append(names, candidate.Name)
		}
		outcome, f := vault.ImportSkills(root, stage.Root(), preview, target.Category, names)
		if f != nil {
			return contract.SkillOpResult{Failure: f}
		}
		imported := map[string]bool{}
		for _, name := range outcome.Imported {
			imported[name] = true
			success = true
		}
		for _, candidate := range picked {
			item := contract.SkillOpItem{TargetID: candidate.ID, ToolID: "kb", Result: "copied"}
			if !imported[candidate.Name] {
				item.Result = "skipped"
				item.Detail = "目标已存在或复制失败"
				for _, skip := range outcome.Skipped {
					if skip.Name == candidate.Name {
						item.Detail = "跳过：" + skip.Reason
					}
				}
				items = append(items, item)
				continue
			}
			skillPath := "skills/" + target.Category + "/" + candidate.Name
			targetDir := filepath.Join(root, filepath.FromSlash(skillPath))
			hash, hashErr := skills.TreeHash(targetDir)
			if hashErr != nil || hash == "" {
				item.Detail = "已安装，但无法计算内容哈希，未写来源记录"
			} else if stage.Meta.Origin != nil {
				repoPath := joinRepoPath(stage.Meta.Origin.Path, candidate.ID)
				record := skills.KbSource{
					SkillPath: skillPath, Owner: stage.Meta.Origin.Owner, Repo: stage.Meta.Origin.Repo,
					Ref: stage.Meta.Origin.Ref, Path: repoPath, Commit: stage.Meta.Origin.Commit,
					PathCommit: stage.Meta.PathCommit, BaseHash: hash,
					InstalledAt: s.now().UTC().Format("2006-01-02T15:04:05Z"),
				}
				if f := skills.SaveKbSource(root, record); f != nil {
					item.Detail = "已安装，但来源记录写入失败：" + f.Message
				}
			}
			s.logOperation("install", candidate.Name, "kb", skillPath, "copy", "ok", "", "")
			items = append(items, item)
		}
	case "tools":
		targets, f := s.resolveInstallTargets(target.Targets)
		if f != nil {
			return contract.SkillOpResult{Failure: f}
		}
		detector := s.detector()
		for _, candidate := range picked {
			sourceDir := filepath.Join(stage.Root(), filepath.FromSlash(candidate.ID))
			hash, _ := skills.TreeHash(sourceDir)
			perTarget, _ := skills.ExecuteDistribute(skills.DistributeOptions{
				SourceDir: sourceDir,
				Mode:      "copy",
				Targets:   withName(targets, candidate.Name),
				Backups:   s.backupStore(),
				Now:       s.now,
			})
			for index, item := range perTarget {
				item.Detail = strings.TrimSpace(item.Detail)
				items = append(items, item)
				location := redactToolPath(filepath.Join(targets[index].InstallDir, candidate.Name), detector)
				result := "ok"
				if item.Result == "failed" {
					result = "failed"
				}
				if item.Result == "copied" || item.Result == "failed" {
					s.logOperation("install", candidate.Name, item.ToolID, location, "copy", result, item.BackupID, item.Detail)
				}
				if item.Result != "copied" {
					continue
				}
				success = true
				if stage.Meta.Origin != nil {
					targetPath := filepath.Join(targets[index].InstallDir, candidate.Name)
					targetHash, err := skills.TreeHash(targetPath)
					if err != nil || targetHash == "" {
						targetHash = hash
					}
					record := skills.ToolSource{
						ToolID: item.ToolID, TargetPath: targetPath,
						Owner: stage.Meta.Origin.Owner, Repo: stage.Meta.Origin.Repo, Ref: stage.Meta.Origin.Ref,
						Path:   joinRepoPath(stage.Meta.Origin.Path, candidate.ID),
						Commit: stage.Meta.Origin.Commit, PathCommit: stage.Meta.PathCommit,
						BaseHash: targetHash, InstalledAt: s.now().UTC().Format("2006-01-02T15:04:05Z"),
					}
					if f := skills.SaveToolSource(filepath.Join(s.app.DataDir, "skills"), record); f != nil {
						item.Detail = "已安装，但来源记录写入失败：" + f.Message
					}
				}
			}
		}
	default:
		return contract.SkillOpResult{Failure: contract.Fail(contract.CodeInvalidInput, "安装目标不合法")}
	}

	if success {
		_ = os.Remove(s.cachePath())
		s.emitChanged()
		_ = store.Discard(stageID)
	}
	return contract.SkillOpResult{Items: items}
}

// stageSelection loads the stage and keeps the candidates the user picked.
func (s *SkillService) stageSelection(ctx context.Context, vaultID, stageID string, ids []string) (skills.Stage, []contract.SkillStageCandidate, *contract.Failure) {
	if len(ids) == 0 || len(ids) > 50 {
		return skills.Stage{}, nil, contract.Fail(contract.CodeInvalidInput, "请选择 1–50 个技能")
	}
	stage, f := s.stageStore().Get(stageID)
	if f != nil {
		return skills.Stage{}, nil, f
	}
	existing := s.existingSkillNames(ctx, vaultID)
	result, f := skills.FindCandidates(stage.Root(), stage.Meta.Label, skills.CandidateOptions{
		Existing: func(name string) (string, bool) {
			location, ok := existing[strings.ToLower(name)]
			return location, ok
		},
	})
	if f != nil {
		return skills.Stage{}, nil, f
	}
	picked, f := pickInstallCandidates(result.Candidates, ids)
	if f != nil {
		return skills.Stage{}, nil, f
	}
	return stage, picked, nil
}

// pickInstallCandidates enforces API §13.3: only state=ok may be installed. A
// target that already exists in the vault is refused here even if a caller
// bypasses the disabled checkbox.
func pickInstallCandidates(candidates []contract.SkillStageCandidate, ids []string) ([]contract.SkillStageCandidate, *contract.Failure) {
	byID := map[string]contract.SkillStageCandidate{}
	for _, candidate := range candidates {
		if candidate.State == "ok" {
			byID[candidate.ID] = candidate
		}
	}
	picked := make([]contract.SkillStageCandidate, 0, len(ids))
	for _, id := range ids {
		candidate, ok := byID[id]
		if !ok {
			return nil, contract.Fail(contract.CodeInvalidInput, "候选不存在或不能安装")
		}
		picked = append(picked, candidate)
	}
	return picked, nil
}

// resolveInstallTargets validates the tool targets like M78 does.
func (s *SkillService) resolveInstallTargets(inputs []contract.SkillDistributeTarget) ([]skills.DistributeTarget, *contract.Failure) {
	if len(inputs) == 0 || len(inputs) > 25 {
		return nil, contract.Fail(contract.CodeInvalidInput, "目标数量不合法")
	}
	byID := map[string]skills.DetectedTool{}
	readers := map[string]bool{}
	for _, tool := range s.skillTools() {
		byID[tool.ID] = tool
		if tool.Shared {
			for _, reader := range tool.Readers {
				readers[reader] = true
			}
		}
	}
	resolved := make([]skills.DistributeTarget, 0, len(inputs))
	for _, input := range inputs {
		tool, ok := byID[input.TargetID]
		if !ok {
			return nil, contract.Fail(contract.CodeInvalidInput, "未知的工具")
		}
		if !tool.Enabled {
			return nil, contract.Fail(contract.CodeSkillProtected, "该工具未启用")
		}
		if !tool.Shared && tool.Status != skills.StatusDetected {
			return nil, contract.Fail(contract.CodeSkillProtected, "该工具未检测到")
		}
		if tool.Missing {
			return nil, contract.Fail(contract.CodeNotFound, "登记目录不存在")
		}
		if tool.InstallDir == "" {
			return nil, contract.Fail(contract.CodeSkillProtected, "该工具没有安装目录")
		}
		resolved = append(resolved, skills.DistributeTarget{
			TargetID: input.TargetID, ToolID: input.TargetID, InstallDir: tool.InstallDir,
			ReadsShared: readers[input.TargetID], AllowOverwrite: input.AllowOverwrite,
		})
	}
	return resolved, nil
}

func withName(targets []skills.DistributeTarget, name string) []skills.DistributeTarget {
	out := make([]skills.DistributeTarget, len(targets))
	for index, target := range targets {
		out[index] = target
		out[index].Name = name
	}
	return out
}

// joinRepoPath combines the address sub-path with the candidate's staged path.
func joinRepoPath(base, candidate string) string {
	parts := []string{}
	for _, part := range []string{base, candidate} {
		part = strings.Trim(part, "/")
		if part != "" {
			parts = append(parts, part)
		}
	}
	return path.Join(parts...)
}
