package skills

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/pathguard"
)

// MaxSegmentLength caps one category or skill directory name (FR-SKILL).
const MaxSegmentLength = 80

// ErrPathEscape is returned when a relative path leaves its base directory or
// crosses a reparse point on the way down.
var ErrPathEscape = errors.New("path escapes its base directory")

// IsValidSegment mirrors vault.IsSingleSegmentName for tool-side skill names:
// one segment, no leading dot, within the length limit, no reserved name.
// The rules must stay identical to the knowledge-base side (PRD 15.4); a
// parity test pins both implementations together.
func IsValidSegment(name string) bool {
	if name == "" || len([]rune(name)) > MaxSegmentLength {
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

// ResolveUnder joins rel onto base and guarantees the result stays inside
// base, without following any reparse point on the way down. The final
// segment may itself be a link (callers need to inspect it); every parent
// segment must be a real directory.
func ResolveUnder(base, rel string) (string, error) {
	if rel == "" {
		return "", ErrPathEscape
	}
	if filepath.IsAbs(rel) || strings.HasPrefix(rel, `\\`) || strings.HasPrefix(rel, `/`) ||
		strings.HasPrefix(rel, `\`) || filepath.VolumeName(rel) != "" {
		return "", ErrPathEscape
	}
	clean := filepath.Clean(rel)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", ErrPathEscape
	}
	joined := filepath.Join(base, clean)
	if !UnderRoot(joined, base) {
		return "", ErrPathEscape
	}
	// Walk the parents below base; a junction anywhere on the way down could
	// redirect the write outside the registered directory.
	parts := strings.Split(clean, string(filepath.Separator))
	cur := base
	for _, part := range parts[:len(parts)-1] {
		cur = filepath.Join(cur, part)
		kind, _, err := Classify(cur)
		if err != nil {
			return "", err
		}
		if kind != KindDir {
			return "", ErrPathEscape
		}
	}
	return joined, nil
}

// TreeHash is the sha256 over the relative paths and contents of the regular
// files under root, skipping links (SafeWalk never descends into them). Used
// for copy-sync baselines and for the safety tests that prove a link removal
// leaves its target untouched.
func TreeHash(root string) (string, error) {
	files, _, err := SafeWalk(root)
	if err != nil {
		return "", err
	}
	sortStrings(files)
	h := sha256.New()
	for _, rel := range files {
		h.Write([]byte(rel))
		h.Write([]byte{0})
		f, err := os.Open(filepath.Join(root, rel))
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(h, f); err != nil {
			_ = f.Close()
			return "", err
		}
		_ = f.Close()
		h.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}
