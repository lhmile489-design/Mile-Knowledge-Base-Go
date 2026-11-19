package vault

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/pathguard"
)

// Skill import (M14, M15; PRD FR-SKILL). A skill is a directory holding
// SKILL.md. Limits are the legacy ones: skills are small text packages, and a
// bigger source is almost certainly the whole repository by mistake.

const (
	// MaxSkillFiles caps one skill package.
	MaxSkillFiles = 2000
	// MaxSkillBytes caps one skill package.
	MaxSkillBytes = 50 << 20
	// MaxSkillNameLength caps category and skill names.
	MaxSkillNameLength = 80
)

// osJunk are the system files that do not make a folder "non-empty".
var osJunk = map[string]bool{".ds_store": true, "thumbs.db": true, "desktop.ini": true}

// ProbeFolder counts user content. Only OS junk is ignored: a dot directory
// such as .git means the folder is a repository, so it counts.
func ProbeFolder(folder string) (entryCount int, empty bool, err error) {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return 0, false, err
	}
	for _, entry := range entries {
		if osJunk[strings.ToLower(entry.Name())] {
			continue
		}
		entryCount++
	}
	return entryCount, entryCount == 0, nil
}

// IsSingleSegmentName reports whether a category or skill name is safe to join
// into a path: one segment, no leading dot, within the length limit.
func IsSingleSegmentName(name string) bool {
	if name == "" || len([]rune(name)) > MaxSkillNameLength {
		return false
	}
	if strings.HasPrefix(name, ".") {
		return false
	}
	if strings.ContainsAny(name, `/\`) {
		return false
	}
	clean, f := pathguard.ValidateRel(name)
	return f == nil && clean == name
}

// SkillCandidateInfo is one row of the wizard's second step.
type SkillCandidateInfo struct {
	Name      string
	FileCount int
}

// SkillSourcePreview is the cached M14 result. Dirs maps a skill name to its
// path relative to the source root ("" means the root itself is the skill).
type SkillSourcePreview struct {
	Dirs       map[string]string
	Candidates []SkillCandidateInfo
}

// SkillSkip is one entry of the import result's skipped list.
type SkillSkip struct {
	Name   string
	Reason string // exists | unknown | too-large | failed
}

// ImportOutcome is the M15 payload.
type ImportOutcome struct {
	Imported []string
	Skipped  []SkillSkip
}

// copySkillFile is replaceable so tests can force a mid-copy failure.
var copySkillFile = func(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// PreviewSkillSource lists the skills under the picked folder. The folder
// itself with SKILL.md is a single skill; otherwise only direct children with
// SKILL.md count — a skill repository is <source>/<name>/SKILL.md, and
// recursing would treat nested examples as skills too. Candidates over the
// limits are dropped here, before the user can select them.
func PreviewSkillSource(sourceRoot string) (*SkillSourcePreview, error) {
	dirs := map[string]string{}
	if hasSkillMD(sourceRoot) {
		name := filepath.Base(sourceRoot)
		if IsSingleSegmentName(name) {
			dirs[name] = ""
		}
	} else {
		entries, err := os.ReadDir(sourceRoot)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
				continue
			}
			name := entry.Name()
			if !IsSingleSegmentName(name) {
				continue
			}
			if hasSkillMD(filepath.Join(sourceRoot, name)) {
				dirs[name] = name
			}
		}
	}

	names := make([]string, 0, len(dirs))
	for name := range dirs {
		names = append(names, name)
	}
	sort.Strings(names)
	preview := &SkillSourcePreview{Dirs: map[string]string{}, Candidates: []SkillCandidateInfo{}}
	for _, name := range names {
		files, _, err := walkSkill(filepath.Join(sourceRoot, filepath.FromSlash(dirs[name])))
		if err != nil {
			// Over the limits (or unreadable): leave the candidate out instead
			// of failing the whole preview.
			continue
		}
		preview.Dirs[name] = dirs[name]
		preview.Candidates = append(preview.Candidates, SkillCandidateInfo{Name: name, FileCount: len(files)})
	}
	return preview, nil
}

// ImportSkills copies previewed skills into <vaultRoot>/skills/<category>/. It
// never overwrites: an existing target is reported as "exists". A copy that
// fails removes only the directory this run created.
func ImportSkills(vaultRoot, sourceRoot string, preview *SkillSourcePreview, category string, names []string) (ImportOutcome, *contract.Failure) {
	outcome := ImportOutcome{Imported: []string{}, Skipped: []SkillSkip{}}
	if !IsSingleSegmentName(category) {
		return outcome, contract.Fail(contract.CodeInvalidInput, "分类名不合法")
	}
	for _, name := range names {
		rel, ok := preview.Dirs[name]
		// Only names the preview listed are importable: the renderer cannot
		// point the host at a directory it never scanned.
		if !ok || !IsSingleSegmentName(name) {
			outcome.Skipped = append(outcome.Skipped, SkillSkip{Name: name, Reason: "unknown"})
			continue
		}
		target, f := pathguard.Resolve(vaultRoot, "skills/"+category+"/"+name)
		if f != nil {
			return outcome, contract.Fail(contract.CodePathEscape, "导入目标超出知识库范围")
		}
		if _, err := os.Lstat(target); err == nil {
			outcome.Skipped = append(outcome.Skipped, SkillSkip{Name: name, Reason: "exists"})
			continue
		}
		from := sourceRoot
		if rel != "" {
			from = filepath.Join(sourceRoot, filepath.FromSlash(rel))
		}
		files, _, err := walkSkill(from)
		if err != nil {
			reason := "failed"
			if isTooLarge(err) {
				reason = "too-large"
			}
			outcome.Skipped = append(outcome.Skipped, SkillSkip{Name: name, Reason: reason})
			continue
		}
		if err := copySkillTree(from, target, files); err != nil {
			_ = os.RemoveAll(target)
			outcome.Skipped = append(outcome.Skipped, SkillSkip{Name: name, Reason: "failed"})
			continue
		}
		outcome.Imported = append(outcome.Imported, name)
	}
	return outcome, nil
}

func copySkillTree(from, target string, files []string) error {
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	for _, rel := range files {
		dst := filepath.Join(target, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := copySkillFile(filepath.Join(from, filepath.FromSlash(rel)), dst); err != nil {
			return err
		}
	}
	return nil
}

// tooLargeError marks a walk that exceeded the package limits.
type tooLargeError struct{}

func (tooLargeError) Error() string {
	return "技能包过大（文件数或总大小超限），请确认选的不是整个仓库"
}

func isTooLarge(err error) bool {
	_, ok := err.(tooLargeError)
	return ok
}

// walkSkill lists regular files, skipping dot entries and symlinks (they may
// point outside the source or form a loop) and failing past the limits.
func walkSkill(dir string) ([]string, int64, error) {
	files := []string{}
	var bytes int64
	var walk func(rel string) error
	walk = func(rel string) error {
		full := dir
		if rel != "" {
			full = filepath.Join(dir, filepath.FromSlash(rel))
		}
		entries, err := os.ReadDir(full)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			if entry.Type()&os.ModeSymlink != 0 {
				continue
			}
			child := name
			if rel != "" {
				child = rel + "/" + name
			}
			if entry.IsDir() {
				if err := walk(child); err != nil {
					return err
				}
				continue
			}
			if !entry.Type().IsRegular() {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			bytes += info.Size()
			files = append(files, child)
			if len(files) > MaxSkillFiles || bytes > MaxSkillBytes {
				return tooLargeError{}
			}
		}
		return nil
	}
	if err := walk(""); err != nil {
		return nil, 0, err
	}
	return files, bytes, nil
}

func hasSkillMD(dir string) bool {
	info, err := os.Lstat(filepath.Join(dir, "SKILL.md"))
	return err == nil && info.Mode().IsRegular()
}
