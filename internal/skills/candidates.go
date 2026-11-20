package skills

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/vault"
)

const (
	// CandidateDepth is how far below a repository root skills are looked for
	// (PRD FR-SKM-SRC: skills/<name>/SKILL.md and one grouping level).
	CandidateDepth = 3
	// MaxCandidates bounds the pick list (16.4).
	MaxCandidates = 50
)

// CandidateOptions tunes the search; zero values use the defaults.
type CandidateOptions struct {
	MaxDepth      int
	MaxCandidates int
	MaxFiles      int
	MaxBytes      int64
	// Existing reports where a skill of this name already lives (a redacted
	// path), used for the "exists" state; nil skips the check.
	Existing func(name string) (string, bool)
}

// CandidateResult is the pick list plus whether the cap truncated it.
type CandidateResult struct {
	Candidates []contract.SkillStageCandidate
	Truncated  bool
}

// FindCandidates locates installable skills inside a staged download
// (FR-SKM-SRC). The root itself with SKILL.md is one skill; otherwise the
// search goes CandidateDepth levels down, skipping dot entries, node_modules
// and reparse points, and never descends into a skill directory.
func FindCandidates(root, fallbackName string, options CandidateOptions) (CandidateResult, *contract.Failure) {
	if options.MaxDepth <= 0 {
		options.MaxDepth = CandidateDepth
	}
	if options.MaxCandidates <= 0 {
		options.MaxCandidates = MaxCandidates
	}
	if options.MaxFiles <= 0 {
		options.MaxFiles = vault.MaxSkillFiles
	}
	if options.MaxBytes <= 0 {
		options.MaxBytes = vault.MaxSkillBytes
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return CandidateResult{}, contract.Fail(contract.CodeNotFound, "暂存内容不存在")
	}

	type found struct {
		rel  string
		name string
	}
	var matches []found
	if hasSkillFile(root) {
		matches = append(matches, found{rel: "", name: rootSkillName(root, fallbackName)})
	} else {
		var walk func(dir, rel string, depth int)
		walk = func(dir, rel string, depth int) {
			entries, err := os.ReadDir(dir)
			if err != nil {
				return
			}
			for _, entry := range entries {
				name := entry.Name()
				if skipEntry(name) {
					continue
				}
				childRel := name
				if rel != "" {
					childRel = rel + "/" + name
				}
				child := filepath.Join(dir, name)
				kind, _, err := Classify(child)
				if err != nil || kind != KindDir {
					continue
				}
				if hasSkillFile(child) {
					matches = append(matches, found{rel: childRel, name: name})
					continue
				}
				if depth < options.MaxDepth {
					walk(child, childRel, depth+1)
				}
			}
		}
		walk(root, "", 1)
	}

	sort.Slice(matches, func(i, j int) bool { return matches[i].rel < matches[j].rel })
	result := CandidateResult{Candidates: make([]contract.SkillStageCandidate, 0, len(matches))}
	seen := map[string]bool{}
	for _, match := range matches {
		candidate := contract.SkillStageCandidate{
			ID:          match.rel,
			Name:        match.name,
			Description: SkillDescription(filepath.Join(root, filepath.FromSlash(match.rel))),
		}
		abs := filepath.Join(root, filepath.FromSlash(match.rel))
		files, bytes, measureFailure := measureSkill(abs, options.MaxFiles, options.MaxBytes)
		candidate.Files, candidate.Bytes = files, bytes
		switch {
		case measureFailure == errSkillTooLarge:
			candidate.State = "too-large"
		case !IsValidSegment(match.name):
			candidate.State = "invalid-name"
		case seen[strings.ToLower(match.name)]:
			candidate.State = "duplicate"
		default:
			if options.Existing != nil {
				if existing, ok := options.Existing(match.name); ok {
					candidate.State = "exists"
					candidate.Existing = existing
				}
			}
			if candidate.State == "" {
				candidate.State = "ok"
			}
			seen[strings.ToLower(match.name)] = true
		}
		if len(result.Candidates) >= options.MaxCandidates {
			result.Truncated = true
			break
		}
		result.Candidates = append(result.Candidates, candidate)
	}
	return result, nil
}

var errSkillTooLarge = os.ErrInvalid

// measureSkill counts regular files and bytes, stopping at the FR-SKILL caps.
func measureSkill(dir string, maxFiles int, maxBytes int64) (int, int64, error) {
	var files int
	var bytes int64
	var walk func(rel string) error
	walk = func(rel string) error {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		entries, err := os.ReadDir(full)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if skipEntry(entry.Name()) {
				continue
			}
			childRel := entry.Name()
			if rel != "" {
				childRel = rel + "/" + entry.Name()
			}
			child := filepath.Join(dir, filepath.FromSlash(childRel))
			kind, _, err := Classify(child)
			if err != nil || kind == KindJunction || kind == KindSymlink {
				continue
			}
			if kind == KindDir {
				if err := walk(childRel); err != nil {
					return err
				}
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			files++
			bytes += info.Size()
			if files > maxFiles || bytes > maxBytes {
				return errSkillTooLarge
			}
		}
		return nil
	}
	if err := walk(""); err != nil {
		if err == errSkillTooLarge {
			return files, bytes, errSkillTooLarge
		}
		return 0, 0, err
	}
	return files, bytes, nil
}

// rootSkillName prefers the SKILL.md frontmatter name, then the caller's
// fallback (repository name or the URL's last segment).
func rootSkillName(dir, fallback string) string {
	if name := frontmatterName(filepath.Join(dir, "SKILL.md")); name != "" && IsValidSegment(name) {
		return name
	}
	if IsValidSegment(fallback) {
		return fallback
	}
	return fallback
}

// frontmatterName reads the first 4 KiB of SKILL.md for `name:` (FR-SKILL).
func frontmatterName(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	buffer := make([]byte, 4096)
	read, _ := file.Read(buffer)
	body := string(buffer[:read])
	lines := strings.Split(body, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return ""
	}
	for _, line := range lines[1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			break
		}
		if value, ok := strings.CutPrefix(trimmed, "name:"); ok {
			return strings.Trim(strings.TrimSpace(value), `"'`)
		}
	}
	return ""
}
