package service

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/pathguard"
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

// restoreIDPattern matches the r-<12 hex> ids RestorePreview hands out.
var restoreIDPattern = regexp.MustCompile(`^r-[0-9a-f]{12}$`)

// Restore is M103: execute the selected manifest items (技术设计 18.8). The
// manifest is re-read and every item is judged again; the manifest itself is
// never modified here.
func (s *SkillService) RestoreManifest(ctx context.Context, vaultID string, selections []contract.SkillRestoreSelection, requestID string) contract.SkillManifestRestoreResult {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if f := validateID(vaultID); f != nil {
		return contract.SkillManifestRestoreResult{Failure: f}
	}
	if len(selections) == 0 || len(selections) > 500 {
		return contract.SkillManifestRestoreResult{Failure: contract.Fail(contract.CodeInvalidInput, "选择项数量不合法")}
	}
	overwrite := map[string]bool{}
	order := make([]string, 0, len(selections))
	for _, selection := range selections {
		if !restoreIDPattern.MatchString(selection.ID) {
			return contract.SkillManifestRestoreResult{Failure: contract.Fail(contract.CodeInvalidInput, "选择项不合法")}
		}
		if _, seen := overwrite[selection.ID]; !seen {
			order = append(order, selection.ID)
		}
		overwrite[selection.ID] = overwrite[selection.ID] || selection.Overwrite
	}
	root, f := s.app.Vaults.Root(vaultID)
	if f != nil {
		return contract.SkillManifestRestoreResult{Failure: contract.Fail(contract.CodeVaultUnavailable, "知识库不可用")}
	}
	manifest := skills.LoadManifest(root)
	if manifest.Status != skills.ManifestOK {
		return contract.SkillManifestRestoreResult{Failure: contract.Fail(contract.CodeUnsupported, "分发清单无法读取")}
	}
	facts, _ := s.restoreFacts(root, vaultID)
	judgment := skills.JudgeManifest(manifest, facts)
	judged := make(map[string]contract.SkillRestoreItem, len(judgment.Items))
	manifests := make(map[string]skills.ManifestItem, len(manifest.Items))
	for i, item := range judgment.Items {
		judged[item.ID] = item
		manifests[item.ID] = manifest.Items[i]
	}

	now := s.now()
	state := skills.LoadState(s.statePath())
	stateChanged := false
	stages := map[githubRestoreKey]*githubRestoreStage{}
	defer func() {
		for _, stage := range stages {
			_ = s.stageStore().Discard(stage.stage.Meta.ID)
		}
	}()

	items := make([]contract.SkillRestoreOpItem, 0, len(order))
	canceled := false
	for _, id := range order {
		if ctx.Err() != nil {
			canceled = true
		}
		item, ok := judged[id]
		if !ok {
			items = append(items, contract.SkillRestoreOpItem{ID: id, Result: "skipped", Detail: "清单已变化"})
			continue
		}
		out := contract.SkillRestoreOpItem{ID: id, Name: item.Name, TargetLabel: item.TargetLabel}
		if canceled {
			out.Result, out.Detail = "skipped", "已取消"
			items = append(items, out)
			continue
		}
		switch item.State {
		case "exists":
			out.Result, out.Detail = "skipped", "已一致"
		case "skip":
			out.Result, out.Detail = "skipped", item.Reason
		case "conflict":
			if !overwrite[id] {
				out.Result, out.Detail = "skipped", "目标已存在，未勾选覆盖"
			} else {
				s.restoreItem(ctx, vaultID, item, manifests[id], overwrite[id], now, state, &stateChanged, stages, requestID, &out)
			}
		default:
			s.restoreItem(ctx, vaultID, item, manifests[id], overwrite[id], now, state, &stateChanged, stages, requestID, &out)
		}
		if ctx.Err() != nil && out.Result == "failed" {
			out.Result, out.Detail = "skipped", "已取消"
		}
		items = append(items, out)
	}

	if stateChanged {
		if err := skills.SaveState(s.statePath(), *state); err != nil {
			return contract.SkillManifestRestoreResult{Items: items, Failure: contract.Fail(contract.CodeIOFailure, "分发记录写入失败")}
		}
	}
	changed := false
	for _, item := range items {
		if item.Result == "linked" || item.Result == "copied" || item.Result == "installed" {
			changed = true
			break
		}
	}
	if changed {
		_ = os.Remove(s.cachePath())
		s.emitChanged()
	}
	return contract.SkillManifestRestoreResult{Items: items}
}

type githubRestoreKey struct {
	owner, repo, ref, commit string
}

type githubRestoreStage struct {
	stage skills.Stage
	fail  string
}

// restoreItem executes one judged item. The judgment was made under the same
// lock, moments ago (技术设计 18.8 step 1).
func (s *SkillService) restoreItem(
	ctx context.Context, vaultID string,
	item contract.SkillRestoreItem, manifest skills.ManifestItem, overwrite bool,
	now time.Time, state *skills.State, stateChanged *bool,
	stages map[githubRestoreKey]*githubRestoreStage, requestID string,
	out *contract.SkillRestoreOpItem,
) {
	if item.Source == "kb" {
		s.restoreKBItem(vaultID, item, overwrite, now, state, stateChanged, out)
		return
	}
	s.restoreGitHubItem(ctx, vaultID, item, manifest, overwrite, now, stages, requestID, out)
}

// restoreKBItem distributes one vault skill according to the manifest
// (技术设计 18.8 step 2).
func (s *SkillService) restoreKBItem(
	vaultID string, item contract.SkillRestoreItem, overwrite bool,
	now time.Time, state *skills.State, stateChanged *bool,
	out *contract.SkillRestoreOpItem,
) {
	root, f := s.app.Vaults.Root(vaultID)
	if f != nil {
		out.Result, out.Detail = "failed", "知识库不可用"
		return
	}
	facts, _ := s.restoreFacts(root, vaultID)
	dest, reason := facts.Resolve(skills.ManifestTarget{
		Kind: item.Target.Kind, ToolID: item.Target.ToolID, Name: item.Target.Name, Subdir: item.Target.Subdir,
	})
	if reason != "" {
		out.Result, out.Detail = "skipped", reason
		return
	}
	sourceDir := vaultSkillDir(root, item.SkillPath)
	if sourceDir == "" {
		out.Result, out.Detail = "failed", "知识库里没有这个技能"
		return
	}
	mode := item.Mode
	if mode != "link" && mode != "copy" {
		mode = "link"
	}
	results, records := skills.ExecuteDistribute(skills.DistributeOptions{
		VaultID:   vaultID,
		SkillPath: item.SkillPath,
		SourceDir: sourceDir,
		Mode:      mode,
		Targets: []skills.DistributeTarget{{
			TargetID: dest.ToolID, ToolID: dest.ToolID, Name: item.Name,
			InstallDir: dest.Dir, AllowOverwrite: overwrite,
		}},
		Backups: s.backupStore(),
		Now:     s.now,
	})
	result := results[0]
	out.Result, out.Detail, out.BackupID = result.Result, result.Detail, result.BackupID
	if result.Result != "linked" && result.Result != "copied" {
		return
	}
	for _, record := range records {
		state.AddDistribution(record)
	}
	*stateChanged = true
	targetPath := filepath.Join(dest.Dir, item.Name)
	s.logOperation("distribute", item.Name, dest.ToolID, targetPath, result.Result, "ok", result.BackupID, "从清单还原")
}

// restoreGitHubItem installs one GitHub skill from the recorded commit
// (技术设计 18.8 step 2). All items of one owner/repo/ref/commit share a
// single download.
func (s *SkillService) restoreGitHubItem(
	ctx context.Context, vaultID string, item contract.SkillRestoreItem, manifest skills.ManifestItem, overwrite bool,
	now time.Time, stages map[githubRestoreKey]*githubRestoreStage, requestID string,
	out *contract.SkillRestoreOpItem,
) {
	if item.Origin == nil {
		out.Result, out.Detail = "failed", "来源信息不完整"
		return
	}
	key := githubRestoreKey{owner: item.Origin.Owner, repo: item.Origin.Repo, ref: item.Origin.Ref, commit: item.Origin.Commit}
	group, ok := stages[key]
	if !ok {
		stage, sf := s.stageStore().Create("github", item.Origin.Owner+"/"+item.Origin.Repo, item.Origin)
		if sf != nil {
			out.Result, out.Detail = "failed", sf.Message
			return
		}
		group = &githubRestoreStage{stage: stage}
		stages[key] = group
		group.fail = s.downloadRestoreArchive(ctx, stage, item.Origin, requestID)
	}
	if group.fail != "" {
		out.Result, out.Detail = "failed", group.fail
		return
	}
	candidate := filepath.Join(group.stage.Root(), filepath.FromSlash(item.Origin.Path))
	if !hasSkillMarker(candidate) {
		out.Result, out.Detail = "failed", "压缩包里没有这个技能"
		return
	}
	root, f := s.app.Vaults.Root(vaultID)
	if f != nil {
		out.Result, out.Detail = "failed", "知识库不可用"
		return
	}
	facts, _ := s.restoreFacts(root, vaultID)
	targetDir, reason := facts.Resolve(skills.ManifestTarget{
		Kind: item.Target.Kind, ToolID: item.Target.ToolID, Name: item.Target.Name, Subdir: item.Target.Subdir,
	})
	if reason != "" {
		out.Result, out.Detail = "skipped", reason
		return
	}
	results, _ := skills.ExecuteDistribute(skills.DistributeOptions{
		SourceDir: candidate,
		Mode:      "copy",
		Targets: []skills.DistributeTarget{{
			TargetID: targetDir.ToolID, ToolID: targetDir.ToolID, Name: item.Name,
			InstallDir: targetDir.Dir, AllowOverwrite: overwrite,
		}},
		Backups: s.backupStore(),
		Now:     s.now,
	})
	result := results[0]
	out.Result, out.Detail, out.BackupID = result.Result, result.Detail, result.BackupID
	if result.Result != "copied" {
		return
	}
	targetPath := filepath.Join(targetDir.Dir, item.Name)
	hash, _ := skills.TreeHash(targetPath)
	record := skills.ToolSource{
		ToolID: targetDir.ToolID, TargetPath: targetPath,
		Owner: item.Origin.Owner, Repo: item.Origin.Repo, Ref: item.Origin.Ref, Path: item.Origin.Path,
		Commit: item.Origin.Commit, PathCommit: manifest.PathCommit,
		BaseHash: hash, InstalledAt: now.UTC().Format(time.RFC3339),
	}
	if f := skills.SaveToolSource(filepath.Join(s.app.DataDir, "skills"), record); f != nil {
		out.Detail = "已安装，但来源记录写入失败：" + f.Message
	}
	out.Result = "installed"
	s.logOperation("install", item.Name, targetDir.ToolID, targetPath, "copy", "ok", result.BackupID, "从清单还原")
}

// hasSkillMarker reports whether dir is a skill directory.
func hasSkillMarker(dir string) bool {
	for _, name := range []string{"SKILL.md", "AGENTS.md"} {
		if info, err := os.Stat(filepath.Join(dir, name)); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

// downloadRestoreArchive fetches the recorded commit into the stage and
// unpacks it. The empty string means success.
func (s *SkillService) downloadRestoreArchive(ctx context.Context, stage skills.Stage, origin *contract.SkillOrigin, requestID string) string {
	client := s.githubClient(s.app.Version)
	ref := skills.GithubRef{Owner: origin.Owner, Repo: origin.Repo, Ref: origin.Ref, Path: origin.Path}
	archivePath := filepath.Join(stage.Dir, "archive.tar.gz")
	archive, err := os.Create(archivePath)
	if err != nil {
		return "无法写入暂存区"
	}
	var writer io.Writer = archive
	emit := s.progressEmitter(requestID)
	if requestID != "" {
		emit("downloading", 0, true)
		writer = &progressWriter{writer: archive, emit: func(received int64) { emit("downloading", received, false) }}
	}
	_, f := client.DownloadTarball(ctx, ref, origin.Commit, writer)
	closeErr := archive.Close()
	if f != nil {
		return f.Message
	}
	if closeErr != nil {
		return "无法写入暂存区"
	}
	emit("unpacking", 0, true)
	file, err := os.Open(archivePath)
	if err != nil {
		return "无法读取暂存区"
	}
	_, f = skills.ExtractTarGz(file, stage.Root(), skills.ExtractOptions{})
	file.Close()
	_ = os.Remove(archivePath)
	if f != nil {
		return f.Message
	}
	return ""
}

// RebuildManifest is M104: rewrite the vault manifest from this machine's
// records (技术设计 18.6). The old file is kept as a .corrupt copy first.
func (s *SkillService) RebuildManifest(ctx context.Context, vaultID string) contract.SkillRebuildResult {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if f := validateID(vaultID); f != nil {
		return contract.SkillRebuildResult{Failure: f}
	}
	root, f := s.app.Vaults.Root(vaultID)
	if f != nil {
		return contract.SkillRebuildResult{Failure: contract.Fail(contract.CodeVaultUnavailable, "知识库不可用")}
	}
	current := skills.LoadManifest(root)
	if current.Status == skills.ManifestVersionUnknown {
		return contract.SkillRebuildResult{Failure: contract.Fail(contract.CodeUnsupported, "分发清单来自更新版本的应用，请升级后再重建")}
	}

	now := s.now()
	rebuilt := skills.Manifest{Items: []skills.ManifestItem{}}
	skipped := 0
	// Tool sources first, distributions second: a knowledge-base distribution
	// is authoritative when both describe the same target.
	for _, source := range skills.LoadToolSources(filepath.Join(s.app.DataDir, "skills")).Records {
		if !isDirectoryQuick(source.TargetPath) {
			skipped++
			continue
		}
		target, ok := s.manifestTarget(source.ToolID, source.TargetPath)
		if !ok {
			skipped++
			continue
		}
		rebuilt.Upsert(manifestGitHubItem(target, filepath.Base(source.TargetPath),
			source.Owner, source.Repo, source.Ref, source.Path,
			source.Commit, source.PathCommit, now))
	}
	state := skills.LoadState(s.statePath())
	for _, dist := range state.Distributions {
		if dist.VaultID != vaultID {
			continue
		}
		target, ok := s.manifestTarget(dist.ToolID, dist.TargetPath)
		if !ok {
			skipped++
			continue
		}
		rebuilt.Upsert(manifestKBItem(target, dist.Name, dist.SkillPath, dist.Mode, now))
	}

	corruptCopy := ""
	if current.Status != skills.ManifestMissing {
		path, pf := pathguard.Resolve(root, ".mile-kb/skill-distributions.json")
		if pf != nil {
			return contract.SkillRebuildResult{Failure: contract.Fail(contract.CodeIOFailure, "分发清单路径不可用")}
		}
		name := "skill-distributions.json.corrupt-" + now.UTC().Format("20060102T150405Z")
		if err := os.Rename(path, filepath.Join(filepath.Dir(path), name)); err != nil {
			return contract.SkillRebuildResult{Failure: contract.Fail(contract.CodeIOFailure, "无法保留原分发清单副本")}
		}
		corruptCopy = name
	}
	written := len(rebuilt.Items)
	if f := skills.SaveManifest(root, &rebuilt); f != nil {
		return contract.SkillRebuildResult{Failure: f, CorruptCopy: corruptCopy}
	}
	s.clearManifestStale()
	return contract.SkillRebuildResult{Written: written, Skipped: skipped, CorruptCopy: corruptCopy}
}
