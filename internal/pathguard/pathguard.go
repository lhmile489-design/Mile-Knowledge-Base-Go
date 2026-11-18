// Package pathguard validates every path that crosses into the filesystem.
//
// Two layers, both required (PRD 10.2):
//  1. spelling: relative, no NUL/UNC/absolute/drive, no "." or ".." segments,
//     no characters Windows forbids, no trailing dot or space, no DOS device
//     names, at most 1024 characters;
//  2. reality: the deepest existing ancestor is resolved and must sit inside
//     the vault root, and no segment may be a symlink, junction or other
//     reparse point.
package pathguard

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// MaxRelLen is the spelling-layer length limit (API.md §1.4).
const MaxRelLen = 1024

var reservedNames = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

var forbidden = "<>:\"|?*"

func failf(code, msg string) *contract.Failure { return contract.Fail(code, msg) }

// ValidateRel normalises a vault-relative path to "/" separators and checks
// the spelling layer. An empty string means the vault root and is valid.
func ValidateRel(p string) (string, *contract.Failure) {
	if len(p) > MaxRelLen {
		return "", failf(contract.CodeInvalidInput, "路径过长").WithLimit(int64(len(p)), MaxRelLen)
	}
	if strings.ContainsRune(p, 0) {
		return "", failf(contract.CodePathEscape, "路径含非法字符")
	}
	if p == "" {
		return "", nil
	}
	if strings.HasPrefix(p, "//") || strings.HasPrefix(p, `\\`) {
		return "", failf(contract.CodePathEscape, "不支持 UNC 路径")
	}
	if strings.HasPrefix(p, "/") || strings.HasPrefix(p, `\`) {
		return "", failf(contract.CodePathEscape, "必须是库内相对路径")
	}
	if len(p) >= 2 && p[1] == ':' {
		return "", failf(contract.CodePathEscape, "不支持盘符路径")
	}

	segments := strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' })
	// Rebuild first so that "a///b" normalises to "a/b".
	clean := make([]string, 0, len(segments))
	for _, seg := range segments {
		if seg == "." || seg == ".." {
			return "", failf(contract.CodePathEscape, "路径不能包含 . 或 ..")
		}
		if strings.TrimSpace(seg) != seg {
			return "", failf(contract.CodeInvalidInput, "路径片段首尾不能是空格")
		}
		if strings.HasSuffix(seg, ".") {
			return "", failf(contract.CodeInvalidInput, "路径片段不能以点结尾")
		}
		for _, r := range seg {
			if r < 0x20 || r == 0x7f {
				return "", failf(contract.CodeInvalidInput, "路径含控制字符")
			}
			if strings.ContainsRune(forbidden, r) {
				return "", failf(contract.CodeInvalidInput, "路径含 Windows 保留字符")
			}
		}
		base := seg
		if i := strings.IndexByte(seg, '.'); i >= 0 {
			base = seg[:i]
		}
		if reservedNames[strings.ToLower(base)] {
			return "", failf(contract.CodeInvalidInput, "路径使用了系统保留名")
		}
		clean = append(clean, seg)
	}
	if len(clean) == 0 {
		return "", nil
	}
	return strings.Join(clean, "/"), nil
}

// ValidateName checks a single new/renamed entry name (FR-TREE).
func ValidateName(name string) *contract.Failure {
	if name == "" {
		return failf(contract.CodeInvalidInput, "名称不能为空")
	}
	if len([]rune(name)) > 200 {
		return failf(contract.CodeInvalidInput, "名称过长")
	}
	if name == "." || name == ".." {
		return failf(contract.CodeInvalidInput, "名称不能是 . 或 ..")
	}
	if strings.ContainsAny(name, `/\`) {
		return failf(contract.CodeInvalidInput, "名称不能包含路径分隔符")
	}
	if err := ValidateNameChars(name); err != nil {
		return err
	}
	return nil
}

// ValidateNameChars is ValidateName without the length/shape rules, for names
// carved out of a path.
func ValidateNameChars(name string) *contract.Failure {
	if strings.TrimSpace(name) != name || strings.HasSuffix(name, ".") {
		return failf(contract.CodeInvalidInput, "名称首尾不能是空格，且不能以点结尾")
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return failf(contract.CodeInvalidInput, "名称含控制字符")
		}
		if strings.ContainsRune(forbidden, r) {
			return failf(contract.CodeInvalidInput, "名称含 Windows 保留字符")
		}
	}
	base := name
	if i := strings.IndexByte(name, '.'); i >= 0 {
		base = name[:i]
	}
	if reservedNames[strings.ToLower(base)] {
		return failf(contract.CodeInvalidInput, "名称是系统保留名")
	}
	return nil
}

// Canon turns an absolute path into a canonical, symlink-free path and rejects
// UNC and device paths. Used for vault roots and their identity.
func Canon(p string) (string, error) {
	if p == "" || strings.ContainsRune(p, 0) {
		return "", fs.ErrInvalid
	}
	if strings.HasPrefix(p, `\\`) || strings.HasPrefix(p, "//") {
		return "", errors.New("UNC paths are not supported")
	}
	abs, err := filepath.Abs(filepath.Clean(p))
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(real, `\\`) {
		return "", errors.New("UNC paths are not supported")
	}
	return real, nil
}

// Resolve maps a vault-relative path to a filesystem path inside root. It
// re-checks the spelling layer and then every existing segment for reparse
// points, so a junction cannot smuggle a path outside the vault (FR-06.6).
func Resolve(root, rel string) (string, *contract.Failure) {
	norm, f := ValidateRel(rel)
	if f != nil {
		return "", f
	}
	realRoot, err := Canon(root)
	if err != nil {
		return "", failf(contract.CodeVaultUnavailable, "知识库根目录不可用")
	}
	if norm == "" {
		return realRoot, nil
	}

	current := realRoot
	segments := strings.Split(norm, "/")
	for i, seg := range segments {
		current = filepath.Join(current, seg)
		info, statErr := os.Lstat(current)
		if statErr != nil {
			if errors.Is(statErr, fs.ErrNotExist) {
				// The tail does not exist yet: that is fine for a create, and
				// nothing below can be a reparse point.
				return filepath.Join(realRoot, filepath.FromSlash(norm)), nil
			}
			return "", failf(contract.CodeIOFailure, "无法读取路径")
		}
		if info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeIrregular != 0 {
			// Go 1.23+ reports Windows junctions as ModeIrregular.
			return "", failf(contract.CodePathEscape, "路径经过符号链接或联接")
		}
		if i < len(segments)-1 && !info.IsDir() {
			return "", failf(contract.CodeNotADirectory, "路径中间项不是目录")
		}
	}
	return current, nil
}

// CanonicalRel returns the on-disk spelling of rel under root. Every existing
// segment is resolved to its real name, so a Windows 8.3 short name
// (MILE-K~1) or a case alias comes back as the long name; a missing tail keeps
// its spelling because nothing on disk exists to alias yet. This is the second
// policy gate of FR-AI-PERM: the spelling check cannot see that MILE-K~1 means
// .mile-kb. It mirrors legacy resolveWithinRoot (pathContainment.ts:154-189).
func CanonicalRel(root, rel string) (string, *contract.Failure) {
	abs, f := Resolve(root, rel)
	if f != nil {
		return "", f
	}
	realRoot, err := Canon(root)
	if err != nil {
		return "", failf(contract.CodeVaultUnavailable, "知识库根目录不可用")
	}
	if abs == realRoot {
		return "", nil
	}

	current := abs
	var missing []string
	for {
		real, evalErr := filepath.EvalSymlinks(current)
		if evalErr == nil {
			if !Within(realRoot, real) {
				return "", failf(contract.CodePathEscape, "路径逃逸知识库范围")
			}
			for i := len(missing) - 1; i >= 0; i-- {
				real = filepath.Join(real, missing[i])
			}
			canonical, relErr := filepath.Rel(realRoot, real)
			if relErr != nil {
				return "", failf(contract.CodeIOFailure, "无法解析路径")
			}
			return filepath.ToSlash(canonical), nil
		}
		if !errors.Is(evalErr, fs.ErrNotExist) && !errors.Is(evalErr, syscall.ENOTDIR) {
			return "", failf(contract.CodeIOFailure, "无法解析路径")
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", failf(contract.CodeIOFailure, "无法解析路径")
		}
		missing = append(missing, filepath.Base(current))
		current = parent
		if !Within(realRoot, current) {
			return "", failf(contract.CodePathEscape, "路径逃逸知识库范围")
		}
	}
}

// Within reports whether target lies inside root (case-insensitive on Windows).
func Within(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if filepath.IsAbs(rel) {
		return false
	}
	for _, seg := range strings.Split(filepath.ToSlash(rel), "/") {
		if seg == ".." {
			return false
		}
	}
	// Guard against drive tricks such as C: vs c:.
	return strings.EqualFold(filepath.VolumeName(root), filepath.VolumeName(target))
}
