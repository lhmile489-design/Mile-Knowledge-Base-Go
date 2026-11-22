package skills

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/pathguard"
)

// targets.json (数据库设计 4.6 M6 增补): the custom tools and projects the user
// registered (FR-SKM-CUSTOM/PROJ, 技术设计 17.3). The file lives in the app
// data directory (B zone), never in a vault.

const (
	targetsVersion  = 1
	targetsFileName = "targets.json"
	targetsMax      = 100
	targetNameMax   = 40
)

// Target kinds (API §14.1).
const (
	TargetCustom  = "custom"
	TargetProject = "project"
)

// Target is one registered custom tool or project.
type Target struct {
	ID         string `json:"id"`                   // t-<12 hex>
	Kind       string `json:"kind"`                 // custom | project
	Name       string `json:"name"`                 // 1–40 chars, single segment
	Path       string `json:"path"`                 // absolute local directory, exists at registration
	InstallRel string `json:"installRel,omitempty"` // project only: one of ProjectSkillDirs()
	CreatedAt  string `json:"createdAt"`            // RFC3339 UTC
}

// Targets is the on-disk shape of skills/targets.json.
type Targets struct {
	Version int      `json:"version"`
	Targets []Target `json:"targets"`
}

// TargetsLoad is LoadTargets' answer: valid records plus what was dropped.
type TargetsLoad struct {
	Targets  []Target
	Warnings []string
}

// TargetsPath returns the file location under the app data directory.
func TargetsPath(dataDir string) string {
	return filepath.Join(dataDir, "skills", targetsFileName)
}

// LoadTargets reads targets.json. A corrupt or version-mismatched file is
// renamed .corrupt-<time> and read as empty; bad records are dropped with a
// warning. Registered directories are not required to still exist.
func LoadTargets(path string) TargetsLoad {
	result := TargetsLoad{Targets: []Target{}}
	body, err := os.ReadFile(path)
	if err != nil {
		return result
	}
	var file Targets
	if err := json.Unmarshal(body, &file); err != nil || file.Version != targetsVersion {
		_ = os.Rename(path, path+".corrupt-"+time.Now().UTC().Format("20060102T150405"))
		return result
	}
	for _, record := range file.Targets {
		if !validTargetRecord(record) {
			result.Warnings = append(result.Warnings, "丢弃一条不合法的登记："+record.ID)
			continue
		}
		if conflictsTargets(record, result.Targets) {
			result.Warnings = append(result.Warnings, "丢弃一条与已有登记重叠的记录："+record.ID)
			continue
		}
		result.Targets = append(result.Targets, record)
	}
	return result
}

// SaveTargets writes targets.json atomically.
func SaveTargets(path string, targets []Target) error {
	file := Targets{Version: targetsVersion, Targets: targets}
	if file.Targets == nil {
		file.Targets = []Target{}
	}
	body, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(body, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// NewTargetID returns t-<12 hex>.
func NewTargetID() (string, error) {
	buffer := make([]byte, 6)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return "t-" + hex.EncodeToString(buffer), nil
}

// IsTargetID accepts t-<12 hex>.
func IsTargetID(id string) bool {
	if len(id) != 14 || !strings.HasPrefix(id, "t-") {
		return false
	}
	for _, r := range id[2:] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

func validTargetRecord(record Target) bool {
	if !IsTargetID(record.ID) {
		return false
	}
	switch record.Kind {
	case TargetCustom:
		if record.InstallRel != "" {
			return false
		}
	case TargetProject:
		if !validInstallRel(record.InstallRel) {
			return false
		}
	default:
		return false
	}
	if validateTargetNameShape(record.Name) != nil {
		return false
	}
	if _, err := pathguard.Canon(record.Path); err != nil {
		// Canon needs the directory to exist; a registered directory that was
		// removed or renamed keeps its record (读取时不要求仍存在), so fall
		// back to a plain absolute check.
		if !localAbsolutePath(record.Path) {
			return false
		}
	}
	_, err := time.Parse(time.RFC3339, record.CreatedAt)
	return err == nil
}

// conflictsTargets reports whether record overlaps a record already kept.
func conflictsTargets(record Target, kept []Target) bool {
	candidate := targetDirs(record)
	for _, existing := range kept {
		for _, dir := range candidate {
			for _, other := range targetDirs(existing) {
				if pathsOverlap(dir, other) {
					return true
				}
			}
		}
	}
	return false
}

// targetDirs is the set of directories a target reads or writes: the custom
// directory itself, or a project's root plus its conventional subdirectories
// (技术设计 17.3 rule 5).
func targetDirs(record Target) []string {
	if record.Kind != TargetProject {
		return []string{record.Path}
	}
	dirs := []string{record.Path}
	for _, rel := range projectDirsForCheck(record) {
		dirs = append(dirs, filepath.Join(record.Path, filepath.FromSlash(rel)))
	}
	return dirs
}

// ProjectTargetDirs is the ordered directory list of a project registration:
// ProjectSkillDirs plus installRel (M97's dirs, 技术设计 17.4).
func ProjectTargetDirs(installRel string) []string {
	return projectDirsForCheck(Target{Kind: TargetProject, InstallRel: installRel})
}

// projectDirsForCheck is ProjectSkillDirs plus the record's installRel.
func projectDirsForCheck(record Target) []string {
	dirs := ProjectSkillDirs()
	if record.InstallRel != "" && !containsString(dirs, record.InstallRel) {
		dirs = append(dirs, record.InstallRel)
	}
	return dirs
}

// TargetEnv is what registration validation needs from the rest of the app.
type TargetEnv struct {
	Home         string   // expands "~" in registry templates
	LocalAppData string   // expands "%LOCALAPPDATA%"
	VaultRoots   []string // every registered vault root
	Targets      []Target // currently registered targets
}

// ValidateTargetName checks the display name (技术设计 17.3).
func ValidateTargetName(name string, existing []Target, exceptID string) *contract.Failure {
	if f := validateTargetNameShape(name); f != nil {
		return f
	}
	lower := strings.ToLower(name)
	for _, tool := range Registry {
		if strings.ToLower(tool.Name) == lower {
			return contract.Fail(contract.CodeInvalidInput, "名称不能和内置工具重名")
		}
	}
	for _, record := range existing {
		if record.ID != exceptID && strings.ToLower(record.Name) == lower {
			return contract.Fail(contract.CodeInvalidInput, "名称已被「"+record.Name+"」使用")
		}
	}
	return nil
}

func validateTargetNameShape(name string) *contract.Failure {
	if len([]rune(name)) > targetNameMax {
		return contract.Fail(contract.CodeInvalidInput, "名称最多 40 个字符")
	}
	if strings.HasPrefix(name, ".") {
		return contract.Fail(contract.CodeInvalidInput, "名称不能以点开头")
	}
	return pathguard.ValidateName(name)
}

// ValidateTargetPath checks the directory rules of 技术设计 17.3 for one
// candidate target. exceptID is the record being edited ("" on create), so the
// 100-target cap does not block edits.
func ValidateTargetPath(env TargetEnv, kind, path, installRel, exceptID string) *contract.Failure {
	count := 0
	for _, record := range env.Targets {
		if record.ID != exceptID {
			count++
		}
	}
	if count >= targetsMax {
		return contract.Fail(contract.CodeInvalidInput, "最多登记 100 个目录")
	}
	if kind == TargetProject && !validInstallRel(installRel) {
		return contract.Fail(contract.CodeInvalidInput, "分发安装子目录不合法")
	}

	dir, f := checkLocalDir(path)
	if f != nil {
		return f
	}
	real, err := pathguard.Canon(dir)
	if err != nil {
		return contract.Fail(contract.CodeInvalidInput, "目录不存在或不可访问")
	}

	// Rule 3: the read/write targets must not overlap a vault root. A project
	// root itself is allowed to contain a vault; its subdirectories are not.
	subdirs := []string{}
	if kind == TargetProject {
		for _, rel := range projectDirsForCheck(Target{Kind: TargetProject, Path: real, InstallRel: installRel}) {
			subdirs = append(subdirs, filepath.Join(real, filepath.FromSlash(rel)))
		}
	}
	vaultDirs := []string{real}
	if kind == TargetProject {
		vaultDirs = subdirs
	}
	for _, vaultRoot := range env.VaultRoots {
		for _, dir := range vaultDirs {
			if pathsOverlap(dir, vaultRoot) {
				return contract.Fail(contract.CodePathEscape, "目录与知识库重叠，不能登记")
			}
		}
	}

	// Rules 4 and 5 look at the project root too: a whole home directory as a
	// project would otherwise overlap every built-in directory.
	allDirs := append([]string{real}, subdirs...)
	for _, entry := range builtinDirs(env) {
		for _, dir := range allDirs {
			if pathsOverlap(dir, entry.dir) {
				return contract.Fail(contract.CodeSkillConflict, "与内置工具「"+entry.tool+"」的目录重叠")
			}
		}
	}

	// Rule 5: other registered targets (project roots included). The record
	// being edited is skipped so an edit can keep its own directory.
	for _, record := range env.Targets {
		if exceptID != "" && record.ID == exceptID {
			continue
		}
		for _, dir := range allDirs {
			for _, other := range targetDirs(record) {
				if pathsOverlap(dir, other) {
					return contract.Fail(contract.CodeSkillConflict, "与已登记的「"+record.Name+"」重叠")
				}
			}
		}
	}
	return nil
}

// checkLocalDir enforces rule 1 and 2: an absolute local path naming an
// existing plain directory (a reparse point itself is rejected, Q-M6-4).
func checkLocalDir(path string) (string, *contract.Failure) {
	if strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, "//") || strings.Contains(path, "\x00") {
		return "", contract.Fail(contract.CodeInvalidInput, "只支持本机磁盘路径，不支持网络路径")
	}
	if !filepath.IsAbs(path) {
		return "", contract.Fail(contract.CodeInvalidInput, "路径必须是绝对路径")
	}
	volume := filepath.VolumeName(path)
	if len(volume) != 2 || volume[1] != ':' {
		return "", contract.Fail(contract.CodeInvalidInput, "只支持本机磁盘路径，不支持网络路径")
	}
	clean := filepath.Clean(path)
	kind, _, err := Classify(clean)
	if err != nil {
		return "", contract.Fail(contract.CodeInvalidInput, "无法读取目录")
	}
	switch kind {
	case KindMissing:
		return "", contract.Fail(contract.CodeInvalidInput, "目录不存在")
	case KindFile:
		return "", contract.Fail(contract.CodeInvalidInput, "路径不是目录")
	case KindJunction, KindSymlink, KindOtherReparse:
		return "", contract.Fail(contract.CodeInvalidInput, "登记的目录本身不能是联接或符号链接")
	}
	return clean, nil
}

func localAbsolutePath(path string) bool {
	if strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, "//") || !filepath.IsAbs(path) {
		return false
	}
	volume := filepath.VolumeName(path)
	return len(volume) == 2 && volume[1] == ':'
}

func validInstallRel(rel string) bool {
	return rel != "" && containsString(ProjectSkillDirs(), rel)
}

// DefaultInstallRel returns the install subdirectory Q-M6-1 picks for a new
// project: the first conventional directory that already exists under root,
// else .claude/skills.
func DefaultInstallRel(root string) string {
	for _, rel := range ProjectSkillDirs() {
		if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err == nil && info.IsDir() {
			return rel
		}
	}
	return ".claude/skills"
}

type builtinDir struct {
	tool string // display name for the conflict message
	dir  string
}

// builtinDirs expands the registry's user-level install/scan/builtin
// directories. Templates with a "*" (AppBuiltinRelative, under the program
// files) are skipped: they are not candidates for registration and the
// wildcard cannot be compared literally.
func builtinDirs(env TargetEnv) []builtinDir {
	home := env.Home
	localAppData := env.LocalAppData
	expand := func(template string) string {
		out := template
		if out == "~" || strings.HasPrefix(out, "~/") || strings.HasPrefix(out, `~\`) {
			out = filepath.Join(home, strings.TrimLeft(out[1:], `/\`))
		}
		out = strings.ReplaceAll(out, "%LOCALAPPDATA%", localAppData)
		return filepath.FromSlash(out)
	}
	var out []builtinDir
	for _, tool := range Registry {
		if tool.InstallDir != "" {
			out = append(out, builtinDir{tool.Name, expand(tool.InstallDir)})
		}
		for _, dir := range tool.ScanDirs {
			out = append(out, builtinDir{tool.Name, expand(dir)})
		}
		for _, dir := range tool.BuiltinDirs {
			out = append(out, builtinDir{tool.Name, expand(dir)})
		}
	}
	return out
}

// pathsOverlap reports whether a and b are the same path or one contains the
// other, case-insensitively (Windows), comparing real paths when they exist.
func pathsOverlap(a, b string) bool {
	left := comparablePath(a)
	right := comparablePath(b)
	return containsPath(left, right) || containsPath(right, left)
}

func comparablePath(path string) string {
	if real, err := pathguard.Canon(path); err == nil {
		return strings.ToLower(real)
	}
	return strings.ToLower(filepath.Clean(path))
}

// containsPath reports whether child equals parent or sits below it.
func containsPath(child, parent string) bool {
	if child == parent {
		return true
	}
	if !strings.HasSuffix(parent, string(filepath.Separator)) {
		parent += string(filepath.Separator)
	}
	return strings.HasPrefix(child, parent)
}

func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

// MergeTargets appends the registered custom tools and projects to the
// detected built-ins (FR-SKM-CUSTOM/PROJ, 技术设计 17.4). Custom tools come
// before projects; each kind keeps creation order. A registered directory
// that is gone stays listed with Missing=true and yields an empty group.
func MergeTargets(tools []DetectedTool, targets []Target) []DetectedTool {
	ordered := make([]Target, len(targets))
	copy(ordered, targets)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Kind != ordered[j].Kind {
			return ordered[i].Kind == TargetCustom
		}
		if ordered[i].CreatedAt != ordered[j].CreatedAt {
			return ordered[i].CreatedAt < ordered[j].CreatedAt
		}
		return ordered[i].ID < ordered[j].ID
	})
	out := make([]DetectedTool, len(tools), len(tools)+len(ordered))
	copy(out, tools)
	for _, target := range ordered {
		out = append(out, targetTool(target))
	}
	return out
}

func targetTool(target Target) DetectedTool {
	entry := DetectedTool{
		ID: target.ID, Name: target.Name, Status: StatusDetected,
		Kind: target.Kind, Official: true, Enabled: true,
	}
	if target.Kind == TargetProject {
		entry.IconKey = "folder"
		entry.Missing = !isDirectory(target.Path)
		entry.ProjectRoot = target.Path
		entry.InstallRel = target.InstallRel
		entry.InstallDir = filepath.Join(target.Path, filepath.FromSlash(target.InstallRel))
		for _, rel := range projectDirsForCheck(target) {
			dir := filepath.Join(target.Path, filepath.FromSlash(rel))
			if isDirectory(dir) {
				entry.ProjectDirs = append(entry.ProjectDirs, ProjectDir{Rel: rel, Dir: dir})
			}
		}
		return entry
	}
	entry.Missing = !isDirectory(target.Path)
	entry.InstallDir = target.Path
	return entry
}

// CountSkills is the quick per-directory count behind M97's skillCount: an
// immediate subdirectory counts when it holds SKILL.md or AGENTS.md. The file
// content is never read, and dot entries are skipped like the scan does.
func CountSkills(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() || skipEntry(entry.Name()) {
			continue
		}
		if hasSkillFile(filepath.Join(dir, entry.Name())) {
			count++
		}
	}
	return count
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
