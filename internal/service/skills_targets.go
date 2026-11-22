package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/pathguard"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/skills"
)

// M6 T04: the registration service behind M97–M101 (API §14, 技术设计 17).
// Registrations live in skills/targets.json; the folder picker hands back an
// opaque token, never the absolute path.

const targetPickTTL = 10 * time.Minute

type targetPick struct {
	path      string
	expiresAt time.Time
}

func (s *SkillService) targetsFile() string {
	return skills.TargetsPath(s.app.DataDir)
}

func (s *SkillService) loadTargets() skills.TargetsLoad {
	return skills.LoadTargets(s.targetsFile())
}

// targetEnv assembles what the validation rules need (技术设计 17.3).
func (s *SkillService) targetEnv(targets []skills.Target) skills.TargetEnv {
	detector := s.detector()
	roots := make([]string, 0, len(s.app.Vaults.List().Vaults))
	for _, item := range s.app.Vaults.List().Vaults {
		roots = append(roots, item.Root)
	}
	var builtin []skills.BuiltinDirRef
	for _, tool := range detector.Detect() {
		for _, dir := range tool.BuiltinDirs {
			builtin = append(builtin, skills.BuiltinDirRef{Tool: tool.Name, Dir: dir})
		}
	}
	return skills.TargetEnv{
		Home:         detector.Home,
		LocalAppData: detector.LocalAppData,
		VaultRoots:   roots,
		Targets:      targets,
		BuiltinDirs:  builtin,
	}
}

// maskPath redacts a registered directory for display (API §14.1). Home and
// local-app-data prefixes use the M4 labels; other paths keep the drive and
// the last two segments.
func maskPath(value string, detector *skills.Detector) string {
	if value == "" {
		return ""
	}
	if masked := redactToolPath(value, detector); masked != value {
		return masked
	}
	volume := filepath.VolumeName(value)
	if volume == "" {
		return value
	}
	rest := strings.TrimPrefix(filepath.ToSlash(value), filepath.ToSlash(volume))
	parts := make([]string, 0, 4)
	for _, part := range strings.Split(rest, "/") {
		if part != "" {
			parts = append(parts, part)
		}
	}
	switch len(parts) {
	case 0:
		return volume + `\…\`
	case 1:
		return volume + `\…\` + parts[0]
	default:
		return volume + `\…\` + parts[len(parts)-2] + `\` + parts[len(parts)-1]
	}
}

func maskPaths(values []string, detector *skills.Detector) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, maskPath(value, detector))
	}
	return out
}

func (s *SkillService) targetView(record skills.Target, detector *skills.Detector) contract.SkillTarget {
	view := contract.SkillTarget{
		ID:        record.ID,
		Kind:      record.Kind,
		Name:      record.Name,
		Path:      maskPath(record.Path, detector),
		Exists:    isDirectoryQuick(record.Path),
		CreatedAt: record.CreatedAt,
	}
	if record.Kind != skills.TargetProject {
		view.SkillCount = skills.CountSkills(record.Path)
		return view
	}
	view.InstallDir = record.InstallRel
	// Existing conventional subdirectories plus installDir, which stays listed
	// with exists=false until it is created (API §14.1).
	for _, rel := range skills.ProjectTargetDirs(record.InstallRel) {
		dir := filepath.Join(record.Path, filepath.FromSlash(rel))
		exists := isDirectoryQuick(dir)
		if !exists && rel != record.InstallRel {
			continue
		}
		count := 0
		if exists {
			count = skills.CountSkills(dir)
		}
		view.Dirs = append(view.Dirs, contract.SkillTargetDir{Rel: rel, Exists: exists, Skills: count})
		view.SkillCount += count
	}
	return view
}

func isDirectoryQuick(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// ListTargets is M97: every registration, custom tools before projects.
func (s *SkillService) ListTargets(ctx context.Context) contract.SkillTargetsResult {
	detector := s.detector()
	loaded := s.loadTargets()
	items := make([]contract.SkillTarget, 0, len(loaded.Targets))
	for _, record := range loaded.Targets {
		items = append(items, s.targetView(record, detector))
	}
	return contract.SkillTargetsResult{Items: items}
}

// PickTargetFolder is M98: the native picker behind an opaque token.
func (s *SkillService) PickTargetFolder(ctx context.Context) contract.PickTargetFolderResult {
	paths, err := pickFolderDialog(ctx, "选择技能目录")
	if err != nil || len(paths) == 0 {
		return contract.PickTargetFolderResult{Cancelled: true}
	}
	root, err := pathguard.Canon(paths[0])
	if err != nil {
		return contract.PickTargetFolderResult{Failure: contract.Fail(contract.CodeInvalidInput, "目录不存在或不可用")}
	}
	dirs := []contract.SkillTargetDir{}
	for _, rel := range skills.ProjectSkillDirs() {
		dir := filepath.Join(root, filepath.FromSlash(rel))
		if !isDirectoryQuick(dir) {
			continue
		}
		dirs = append(dirs, contract.SkillTargetDir{Rel: rel, Exists: true, Skills: skills.CountSkills(dir)})
	}
	return contract.PickTargetFolderResult{
		Token:       s.newTargetPick(root),
		DisplayName: filepath.Base(root),
		Path:        maskPath(root, s.detector()),
		Dirs:        dirs,
	}
}

// SaveTarget is M99: create (token required) or edit (empty token keeps the
// registered directory). Everything is validated again on this side.
func (s *SkillService) SaveTarget(ctx context.Context, input contract.SkillSaveTargetInput) contract.SkillSaveTargetResult {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	loaded := s.loadTargets()
	targets := loaded.Targets
	exceptID := input.ID

	kind := input.Kind
	var record skills.Target
	if exceptID != "" {
		found := false
		for _, item := range targets {
			if item.ID == exceptID {
				record = item
				found = true
				break
			}
		}
		if !found {
			return contract.SkillSaveTargetResult{Failure: contract.Fail(contract.CodeNotFound, "登记不存在")}
		}
		if kind != "" && kind != record.Kind {
			return contract.SkillSaveTargetResult{Failure: contract.Fail(contract.CodeInvalidInput, "登记类型不能修改")}
		}
		kind = record.Kind
	} else if kind != skills.TargetCustom && kind != skills.TargetProject {
		return contract.SkillSaveTargetResult{Failure: contract.Fail(contract.CodeInvalidInput, "登记类型不合法")}
	}

	if f := skills.ValidateTargetName(input.Name, targets, exceptID); f != nil {
		return contract.SkillSaveTargetResult{Failure: f}
	}

	path := record.Path
	if input.Token != "" {
		picked, f := s.targetPickPath(input.Token)
		if f != nil {
			return contract.SkillSaveTargetResult{Failure: f}
		}
		path = picked
	} else if exceptID == "" {
		return contract.SkillSaveTargetResult{Failure: contract.Fail(contract.CodeInvalidInput, "请选择目录")}
	}

	installRel := input.InstallDir
	if kind == skills.TargetProject {
		if installRel == "" && exceptID != "" {
			installRel = record.InstallRel
		}
		if installRel == "" {
			installRel = skills.DefaultInstallRel(path)
		}
	} else if input.InstallDir != "" {
		return contract.SkillSaveTargetResult{Failure: contract.Fail(contract.CodeInvalidInput, "自定义工具没有安装子目录")}
	}

	if f := skills.ValidateTargetPath(s.targetEnv(targets), kind, path, installRel, exceptID); f != nil {
		return contract.SkillSaveTargetResult{Failure: f}
	}
	canonical, err := pathguard.Canon(path)
	if err != nil {
		return contract.SkillSaveTargetResult{Failure: contract.Fail(contract.CodeInvalidInput, "目录不存在或不可用")}
	}

	if exceptID == "" {
		id, err := skills.NewTargetID()
		if err != nil {
			return contract.SkillSaveTargetResult{Failure: contract.Fail(contract.CodeIOFailure, "无法生成登记 id")}
		}
		record = skills.Target{
			ID: id, Kind: kind, Name: input.Name, Path: canonical, InstallRel: installRel,
			CreatedAt: s.now().UTC().Format(time.RFC3339),
		}
		targets = append(targets, record)
	} else {
		record.Name = input.Name
		record.Path = canonical
		record.InstallRel = installRel
		for i := range targets {
			if targets[i].ID == exceptID {
				targets[i] = record
			}
		}
	}
	if err := skills.SaveTargets(s.targetsFile(), targets); err != nil {
		return contract.SkillSaveTargetResult{Failure: contract.Fail(contract.CodeIOFailure, "无法写入登记")}
	}
	if input.Token != "" {
		s.clearTargetPick(input.Token)
	}
	_ = os.Remove(s.cachePath())
	s.emitChanged()
	view := s.targetView(record, s.detector())
	return contract.SkillSaveTargetResult{Target: &view}
}

// RemoveTarget is M100: drop the registration and the records that point at
// it. Files inside the directory are never touched (AC-SKM-17).
func (s *SkillService) RemoveTarget(ctx context.Context, id string) contract.SkillRemoveTargetResult {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	loaded := s.loadTargets()
	var record skills.Target
	found := false
	kept := make([]skills.Target, 0, len(loaded.Targets))
	for _, item := range loaded.Targets {
		if item.ID == id {
			record = item
			found = true
			continue
		}
		kept = append(kept, item)
	}
	if !found {
		return contract.SkillRemoveTargetResult{Failure: contract.Fail(contract.CodeNotFound, "登记不存在")}
	}

	removedDistributions, f := skills.RemoveDistributions(filepath.Join(s.app.DataDir, "skills", "state.json"), id)
	if f != nil {
		return contract.SkillRemoveTargetResult{Failure: f}
	}
	removedSources, f := skills.RemoveToolSources(filepath.Join(s.app.DataDir, "skills"), id)
	if f != nil {
		return contract.SkillRemoveTargetResult{Failure: f}
	}

	if err := skills.SaveTargets(s.targetsFile(), kept); err != nil {
		return contract.SkillRemoveTargetResult{Failure: contract.Fail(contract.CodeIOFailure, "无法写入登记")}
	}
	s.logOperation("unregister", record.Name, record.ID, record.Path, "", "ok", "",
		fmt.Sprintf("删除 %d 条分发记录、%d 条来源记录", removedDistributions, removedSources))
	_ = os.Remove(s.cachePath())
	s.emitChanged()
	return contract.SkillRemoveTargetResult{
		Removed:              true,
		DistributionsRemoved: removedDistributions,
		SourcesRemoved:       removedSources,
	}
}

// RevealTarget is M101: open Explorer at the registered directory; read-only.
func (s *SkillService) RevealTarget(ctx context.Context, id string) contract.Failure {
	for _, record := range s.loadTargets().Targets {
		if record.ID != id {
			continue
		}
		if !isDirectoryQuick(record.Path) {
			return *contract.Fail(contract.CodeNotFound, "目录不存在")
		}
		if err := exec.Command("explorer.exe", record.Path).Start(); err != nil {
			return *contract.Fail(contract.CodeIOFailure, "打开目录失败")
		}
		return contract.Failure{}
	}
	return *contract.Fail(contract.CodeNotFound, "登记不存在")
}

// unknownToolFailure separates a removed registration (not-found, API §14.3)
// from an id that never existed.
func unknownToolFailure(toolID string) *contract.Failure {
	if skills.IsTargetID(toolID) {
		return contract.Fail(contract.CodeNotFound, "登记已移除")
	}
	return contract.Fail(contract.CodeInvalidInput, "未知的工具")
}

// skillTools is the merged tool list every write path resolves against
// (M6 T05): built-ins first, then custom tools and projects.
func (s *SkillService) skillTools() []skills.DetectedTool {
	return skills.MergeTargets(s.detector().Detect(), s.loadTargets().Targets)
}

// parseProjectSpec splits "project:<dir>:<rel>" (API §14.3). The directory
// part carries no colon, so the last colon separates it from the skill path.
func parseProjectSpec(spec string) (dir, rel string, ok bool) {
	rest, found := strings.CutPrefix(spec, "project:")
	if !found {
		return "", "", false
	}
	index := strings.LastIndex(rest, ":")
	if index < 0 {
		return "", "", false
	}
	return rest[:index], rest[index+1:], true
}

// projectRoot resolves one project-relative directory. allowInstall lets the
// caller name installRel before that directory exists (distribute/install).
func projectRoot(tool skills.DetectedTool, dir string, allowInstall bool) (string, bool) {
	if tool.Kind != skills.TargetProject {
		return "", false
	}
	for _, pd := range tool.ProjectDirs {
		if pd.Rel == dir {
			return pd.Dir, true
		}
	}
	if allowInstall && tool.InstallRel != "" && dir == tool.InstallRel && tool.ProjectRoot != "" {
		return filepath.Join(tool.ProjectRoot, filepath.FromSlash(dir)), true
	}
	return "", false
}

func (s *SkillService) newTargetPick(path string) string {
	raw := make([]byte, 8)
	_, _ = rand.Read(raw)
	token := hex.EncodeToString(raw)
	s.targetPicksMu.Lock()
	s.targetPicks[token] = targetPick{path: path, expiresAt: s.now().Add(targetPickTTL)}
	s.targetPicksMu.Unlock()
	return token
}

// targetPickPath resolves a token without consuming it: a failed save keeps
// the token alive until it expires.
func (s *SkillService) targetPickPath(token string) (string, *contract.Failure) {
	s.targetPicksMu.Lock()
	defer s.targetPicksMu.Unlock()
	item, ok := s.targetPicks[token]
	if !ok || s.now().After(item.expiresAt) {
		delete(s.targetPicks, token)
		return "", contract.Fail(contract.CodeNotFound, "选择已过期，请重新选择目录")
	}
	return item.path, nil
}

func (s *SkillService) clearTargetPick(token string) {
	s.targetPicksMu.Lock()
	delete(s.targetPicks, token)
	s.targetPicksMu.Unlock()
}
