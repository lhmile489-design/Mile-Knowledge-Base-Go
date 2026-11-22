package service

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/mutation"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/skills"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/vault"
)

// Update execution (M94–M96, 技术设计 16.6). Knowledge-base skills go through a
// change plan the user confirms; tool skills are backed up and replaced in
// place. Both refuse to touch a locally modified skill.

// updateSource is the installed skill an update applies to.
type updateSource struct {
	Kind       string // kb | tool
	SkillPath  string // vault-relative, kb only
	ToolID     string // tool only
	TargetDir  string // absolute directory that holds the installed files
	Owner      string
	Repo       string
	Ref        string
	RepoPath   string // the skill directory inside the repository
	Commit     string
	PathCommit string
	BaseHash   string
}

// UpdatePreview is M94: fetch the new version, compare it and prepare the plan
// (knowledge base) or the replacement stage (tool).
func (s *SkillService) UpdatePreview(ctx context.Context, vaultID string, target contract.SkillUpdateTarget) contract.SkillUpdatePreviewResult {
	src, root, f := s.resolveUpdateSource(ctx, vaultID, target)
	if f != nil {
		return contract.SkillUpdatePreviewResult{Failure: f}
	}
	localHash, err := skills.TreeHash(src.TargetDir)
	if err != nil {
		return contract.SkillUpdatePreviewResult{Failure: contract.Fail(contract.CodeIOFailure, "无法读取技能目录")}
	}
	if src.BaseHash != "" && !strings.EqualFold(localHash, src.BaseHash) {
		return contract.SkillUpdatePreviewResult{Failure: contract.Fail(contract.CodeSkillConflict, "本地改过，不能一键更新；可以重新安装到别的分类")}
	}

	client := s.githubClient(s.app.Version)
	ref := skills.GithubRef{Owner: src.Owner, Repo: src.Repo, Ref: src.Ref, Path: src.RepoPath}
	resolved, f := client.Resolve(ctx, ref)
	if f != nil {
		return contract.SkillUpdatePreviewResult{Failure: f}
	}
	if strings.EqualFold(resolved.PathCommit, src.PathCommit) {
		return contract.SkillUpdatePreviewResult{
			From: src.PathCommit, To: resolved.PathCommit, Files: []contract.SkillUpdateFile{},
		}
	}

	store := s.stageStore()
	origin := &contract.SkillOrigin{
		Owner: src.Owner, Repo: src.Repo, Ref: resolved.Ref, Path: src.RepoPath, Commit: resolved.Commit,
	}
	stage, f := store.Create("github", src.Owner+"/"+src.Repo, origin)
	if f != nil {
		return contract.SkillUpdatePreviewResult{Failure: f}
	}
	stage.Meta.PathCommit = resolved.PathCommit
	if f := stage.SetUpdate(skills.StageUpdate{
		Kind: src.Kind, SkillPath: src.SkillPath, ToolID: src.ToolID,
		TargetPath: src.TargetDir, SourcePath: src.RepoPath,
	}); f != nil {
		_ = store.Discard(stage.Meta.ID)
		return contract.SkillUpdatePreviewResult{Failure: f}
	}
	fail := func(f *contract.Failure) contract.SkillUpdatePreviewResult {
		_ = store.Discard(stage.Meta.ID)
		return contract.SkillUpdatePreviewResult{Failure: f}
	}

	archivePath := filepath.Join(stage.Dir, "archive.tar.gz")
	archive, err := os.Create(archivePath)
	if err != nil {
		return fail(contract.Fail(contract.CodeIOFailure, "无法写入暂存区"))
	}
	_, f = client.DownloadTarball(ctx, ref, resolved.Commit, archive)
	closeErr := archive.Close()
	if f != nil {
		return fail(f)
	}
	if closeErr != nil {
		return fail(contract.Fail(contract.CodeIOFailure, "无法写入暂存区"))
	}
	file, err := os.Open(archivePath)
	if err != nil {
		return fail(contract.Fail(contract.CodeIOFailure, "无法读取暂存区"))
	}
	_, f = skills.ExtractTarGz(file, stage.Root(), skills.ExtractOptions{})
	file.Close()
	_ = os.Remove(archivePath)
	if f != nil {
		return fail(f)
	}

	candidate := filepath.Join(stage.Root(), filepath.FromSlash(src.RepoPath))
	if !pathExists(filepath.Join(candidate, "SKILL.md")) && !pathExists(filepath.Join(candidate, "AGENTS.md")) {
		return fail(contract.Fail(contract.CodeNotFound, "远端不再包含该技能"))
	}
	diff, err := skills.DiffSkillTrees(src.TargetDir, candidate)
	if err != nil {
		return fail(contract.Fail(contract.CodeIOFailure, "无法比较技能内容"))
	}
	if len(diff) > mutation.MaxPlanActions {
		return fail(contract.Fail(contract.CodeTooLarge, "变动超过 500 项，请重新安装该技能").WithLimit(int64(len(diff)), mutation.MaxPlanActions))
	}
	if len(diff) == 0 {
		// A path commit can move without changing the skill's bytes.
		_ = store.Discard(stage.Meta.ID)
		return contract.SkillUpdatePreviewResult{
			From: src.PathCommit, To: resolved.PathCommit, Files: []contract.SkillUpdateFile{},
		}
	}
	files := make([]contract.SkillUpdateFile, 0, len(diff))
	for _, entry := range diff {
		files = append(files, contract.SkillUpdateFile{
			Path: entry.Path, Change: entry.Change, Bytes: entry.Bytes, Binary: entry.Binary,
		})
	}
	if src.Kind == "tool" {
		return contract.SkillUpdatePreviewResult{
			StageID: stage.Meta.ID, From: src.PathCommit, To: resolved.PathCommit,
			Files: files, BackupPath: filepath.Join(s.backupStore().Dir, "<new>", "files"),
		}
	}
	plan, f := s.buildSkillUpdatePlan(root, vaultID, stage, src, diff)
	if f != nil {
		return fail(f)
	}
	return contract.SkillUpdatePreviewResult{
		StageID: stage.Meta.ID, From: src.PathCommit, To: resolved.PathCommit,
		Files: files, Plan: plan,
	}
}

// resolveUpdateSource validates the target and finds its GitHub source record.
func (s *SkillService) resolveUpdateSource(ctx context.Context, vaultID string, target contract.SkillUpdateTarget) (updateSource, string, *contract.Failure) {
	switch target.Kind {
	case "kb":
		if f := validateID(vaultID); f != nil {
			return updateSource{}, "", f
		}
		if !skills.IsValidKbSkillPath(target.SkillPath) {
			return updateSource{}, "", contract.Fail(contract.CodeInvalidInput, "技能路径不合法")
		}
		root, f := s.app.Vaults.Root(vaultID)
		if f != nil {
			return updateSource{}, "", contract.Fail(contract.CodeVaultUnavailable, "知识库不可用")
		}
		var record skills.KbSource
		found := false
		for _, item := range skills.LoadKbSources(root).Records {
			if item.SkillPath == target.SkillPath {
				record, found = item, true
				break
			}
		}
		if !found {
			return updateSource{}, "", contract.Fail(contract.CodeNotFound, "该技能没有 GitHub 来源")
		}
		dir := filepath.Join(root, filepath.FromSlash(target.SkillPath))
		if !pathExists(dir) {
			return updateSource{}, "", contract.Fail(contract.CodeNotFound, "技能不存在")
		}
		return updateSource{
			Kind: "kb", SkillPath: record.SkillPath, TargetDir: dir,
			Owner: record.Owner, Repo: record.Repo, Ref: record.Ref, RepoPath: record.Path,
			Commit: record.Commit, PathCommit: record.PathCommit, BaseHash: record.BaseHash,
		}, root, nil
	case "tool":
		if target.ToolID == "" || target.Path == "" {
			return updateSource{}, "", contract.Fail(contract.CodeInvalidInput, "工具技能需要工具与路径")
		}
		_, abs, f := s.resolveToolSkill(ctx, target.ToolID, target.Path)
		if f != nil {
			return updateSource{}, "", f
		}
		record, ok := skills.FindToolSource(skills.LoadToolSources(s.updateCacheDir()).Records, abs)
		if !ok {
			return updateSource{}, "", contract.Fail(contract.CodeNotFound, "该技能没有 GitHub 来源")
		}
		return updateSource{
			Kind: "tool", ToolID: record.ToolID, TargetDir: abs,
			Owner: record.Owner, Repo: record.Repo, Ref: record.Ref, RepoPath: record.Path,
			Commit: record.Commit, PathCommit: record.PathCommit, BaseHash: record.BaseHash,
		}, "", nil
	}
	return updateSource{}, "", contract.Fail(contract.CodeInvalidInput, "更新目标不合法")
}

// buildSkillUpdatePlan turns the diff into one user plan (16.6). Text uses
// edit/create, everything else uses importFile so the bytes match exactly; a
// file whose parent directory does not exist yet also goes through importFile
// (the plan validator requires existing parents, the executor creates them).
func (s *SkillService) buildSkillUpdatePlan(root, vaultID string, stage skills.Stage, src updateSource, diff []skills.UpdateDiff) (*contract.MutationPlan, *contract.Failure) {
	actions := make([]contract.MutationAction, 0, len(diff))
	add := func(action contract.MutationAction) {
		action.ID = "a" + strconv.Itoa(len(actions)+1)
		action.Reason = "更新技能"
		actions = append(actions, action)
	}
	for _, entry := range diff {
		targetRel := src.SkillPath + "/" + entry.Path
		stagePath := path.Join(src.RepoPath, entry.Path)
		switch entry.Change {
		case "removed":
			add(contract.MutationAction{Kind: contract.MutTrash, Source: targetRel})
		case "added":
			// create appends ".md" to a name without an extension and needs the
			// parent to exist; a skill file must keep its exact name, so
			// everything else is written through importFile.
			if entry.TextOK && hasExtension(entry.Path) &&
				pathExists(filepath.Join(root, filepath.Dir(filepath.FromSlash(targetRel)))) {
				text := entry.Text
				add(contract.MutationAction{Kind: contract.MutCreate, Target: targetRel, Text: &text})
				continue
			}
			add(contract.MutationAction{
				Kind: contract.MutImportFile, Target: targetRel,
				StageID: stage.Meta.ID, StagePath: stagePath, StageHash: entry.Hash,
			})
		case "modified":
			revision, _, revFailure := vault.StatRevision(root, targetRel)
			if revFailure != nil && revFailure.Code != contract.CodeNotFound {
				return nil, revFailure
			}
			exists := revFailure == nil
			if entry.TextOK && entry.LocalTextOK {
				if !exists {
					return nil, contract.Fail(contract.CodeRevisionMismatch, "本地文件已变化，请重新检查更新")
				}
				text := entry.Text
				add(contract.MutationAction{
					Kind: contract.MutEdit, Target: targetRel, Text: &text,
					ExpectedTargetRevision: &revision,
				})
				continue
			}
			action := contract.MutationAction{
				Kind: contract.MutImportFile, Target: targetRel,
				StageID: stage.Meta.ID, StagePath: stagePath, StageHash: entry.Hash,
			}
			if exists {
				action.ExpectedTargetRevision = &revision
			}
			add(action)
		}
	}
	builder := mutation.Builder{Root: root, StagingRoot: filepath.Join(s.app.DataDir, "skills", "tmp")}
	plan, f := builder.Plan(contract.PlanRequest{VaultID: vaultID, Origin: "user", Actions: actions})
	if f != nil {
		return nil, f
	}
	return &plan, nil
}

// hasExtension mirrors mutation.WithDefaultExtension's check: a name with a dot
// after the first character survives create unchanged.
func hasExtension(rel string) bool {
	return strings.LastIndex(path.Base(rel), ".") > 0
}

// ConfirmKbUpdate is M95: only when the skill now equals the staged version
// does the source record move forward.
func (s *SkillService) ConfirmKbUpdate(ctx context.Context, vaultID, stageID string) contract.SkillConfirmUpdateResult {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if f := validateID(vaultID); f != nil {
		return contract.SkillConfirmUpdateResult{Failure: f}
	}
	root, f := s.app.Vaults.Root(vaultID)
	if f != nil {
		return contract.SkillConfirmUpdateResult{Failure: contract.Fail(contract.CodeVaultUnavailable, "知识库不可用")}
	}
	stage, f := s.stageStore().Get(stageID)
	if f != nil {
		return contract.SkillConfirmUpdateResult{Failure: f}
	}
	update := stage.Meta.Update
	if update == nil || update.Kind != "kb" || !skills.IsValidKbSkillPath(update.SkillPath) {
		return contract.SkillConfirmUpdateResult{Failure: contract.Fail(contract.CodeInvalidInput, "该暂存不是知识库更新")}
	}
	stagedHash, err := skills.TreeHash(filepath.Join(stage.Root(), filepath.FromSlash(update.SourcePath)))
	if err != nil {
		return contract.SkillConfirmUpdateResult{Failure: contract.Fail(contract.CodeIOFailure, "无法读取暂存版本")}
	}
	localHash, err := skills.TreeHash(filepath.Join(root, filepath.FromSlash(update.SkillPath)))
	if err != nil || !strings.EqualFold(stagedHash, localHash) {
		return contract.SkillConfirmUpdateResult{
			Updated: false,
			Failure: contract.Fail(contract.CodeRevisionMismatch, "执行后的内容与暂存版本不一致，请重新检查更新"),
		}
	}
	var record skills.KbSource
	found := false
	for _, item := range skills.LoadKbSources(root).Records {
		if item.SkillPath == update.SkillPath {
			record, found = item, true
			break
		}
	}
	if !found {
		return contract.SkillConfirmUpdateResult{Failure: contract.Fail(contract.CodeNotFound, "来源记录已不存在")}
	}
	if stage.Meta.Origin == nil {
		return contract.SkillConfirmUpdateResult{Failure: contract.Fail(contract.CodeInvalidInput, "暂存缺少来源信息")}
	}
	record.Owner = stage.Meta.Origin.Owner
	record.Repo = stage.Meta.Origin.Repo
	record.Ref = stage.Meta.Origin.Ref
	record.Path = stage.Meta.Origin.Path
	record.Commit = stage.Meta.Origin.Commit
	record.PathCommit = stage.Meta.PathCommit
	record.BaseHash = stagedHash
	record.InstalledAt = s.now().UTC().Format(time.RFC3339)
	if f := skills.SaveKbSource(root, record); f != nil {
		return contract.SkillConfirmUpdateResult{Updated: false, Failure: f}
	}
	_ = os.Remove(s.cachePath())
	s.emitChanged()
	_ = s.stageStore().Discard(stageID)
	return contract.SkillConfirmUpdateResult{Updated: true}
}

// UpdateTool is M96: back up the tool skill, copy the staged version in, and
// move the backup back if the copy fails (16.6).
func (s *SkillService) UpdateTool(ctx context.Context, stageID string) contract.SkillOpResult {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	stage, f := s.stageStore().Get(stageID)
	if f != nil {
		return contract.SkillOpResult{Failure: f}
	}
	update := stage.Meta.Update
	if update == nil || update.Kind != "tool" || update.ToolID == "" || update.TargetPath == "" || stage.Meta.Origin == nil {
		return contract.SkillOpResult{Failure: contract.Fail(contract.CodeInvalidInput, "该暂存不是工具更新")}
	}
	skillsDir := s.updateCacheDir()
	record, ok := skills.FindToolSource(skills.LoadToolSources(skillsDir).Records, update.TargetPath)
	if !ok {
		return contract.SkillOpResult{Failure: contract.Fail(contract.CodeNotFound, "来源记录已不存在")}
	}
	localHash, err := skills.TreeHash(update.TargetPath)
	if err != nil {
		return contract.SkillOpResult{Failure: contract.Fail(contract.CodeIOFailure, "无法读取工具技能目录")}
	}
	if record.BaseHash != "" && !strings.EqualFold(localHash, record.BaseHash) {
		return contract.SkillOpResult{Failure: contract.Fail(contract.CodeSkillConflict, "本地改过，不能一键更新")}
	}
	stagedDir := filepath.Join(stage.Root(), filepath.FromSlash(update.SourcePath))
	stagedHash, err := skills.TreeHash(stagedDir)
	if err != nil {
		return contract.SkillOpResult{Failure: contract.Fail(contract.CodeIOFailure, "无法读取暂存版本")}
	}

	backup, backupDir, err := s.backupStore().Create(record.ToolID, update.TargetPath, "update", s.now())
	if err != nil {
		return contract.SkillOpResult{Failure: contract.Fail(contract.CodeIOFailure, "备份失败："+err.Error())}
	}
	name := filepath.Base(update.TargetPath)
	location := redactToolPath(update.TargetPath, s.detector())
	item := contract.SkillOpItem{TargetID: record.ID, ToolID: record.ToolID, Result: "copied", Detail: "已更新", BackupID: backup.ID}
	if err := skills.CopyTree(stagedDir, update.TargetPath); err != nil {
		_ = os.RemoveAll(update.TargetPath)
		detail := "更新失败，已恢复原目录"
		if restoreErr := os.Rename(filepath.Join(backupDir, "files"), update.TargetPath); restoreErr != nil {
			detail = "更新失败，恢复也失败：" + restoreErr.Error()
		}
		s.logOperation("update", name, record.ToolID, location, "copy", "failed", backup.ID, detail)
		return contract.SkillOpResult{
			Items:   []contract.SkillOpItem{{TargetID: record.ID, ToolID: record.ToolID, Result: "failed", Detail: detail, BackupID: backup.ID}},
			Failure: contract.Fail(contract.CodeIOFailure, detail),
		}
	}

	record.Commit = stage.Meta.Origin.Commit
	record.PathCommit = stage.Meta.PathCommit
	record.BaseHash = stagedHash
	record.InstalledAt = s.now().UTC().Format(time.RFC3339)
	if f := skills.SaveToolSource(skillsDir, record); f != nil {
		item.Detail = "已更新，但来源记录写入失败：" + f.Message
	} else if target, ok := s.manifestTarget(record.ToolID, update.TargetPath); ok {
		now := s.now()
		s.updateManifest(s.app.Vaults.Active(), func(m *skills.Manifest) {
			m.Upsert(manifestGitHubItem(target, name, record.Owner, record.Repo, record.Ref,
				record.Path, record.Commit, record.PathCommit, now))
		})
	}
	s.logOperation("update", name, record.ToolID, location, "copy", "ok", backup.ID, "")
	_ = os.Remove(s.cachePath())
	s.emitChanged()
	_ = s.stageStore().Discard(stageID)
	return contract.SkillOpResult{Items: []contract.SkillOpItem{item}}
}
