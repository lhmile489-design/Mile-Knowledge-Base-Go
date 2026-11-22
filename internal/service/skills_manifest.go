package service

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/skills"
)

// M7 T02: keep the vault manifest in sync with the operations that changed
// this machine (技术设计 18.3). The manifest is a convenience for other
// machines: a manifest that cannot be read or written never fails the
// operation itself (Q-SKM-10).

// updateManifest loads the vault manifest, applies mutate and saves it. A
// missing file is created; unreadable, oversized and future-version files are
// left alone and remembered so M102 can report them (API §15.3).
func (s *SkillService) updateManifest(vaultID string, mutate func(*skills.Manifest)) {
	if vaultID == "" {
		return
	}
	root, f := s.app.Vaults.Root(vaultID)
	if f != nil {
		return
	}
	manifest := skills.LoadManifest(root)
	switch manifest.Status {
	case skills.ManifestOK, skills.ManifestMissing:
	default:
		s.markManifestStale()
		return
	}
	mutate(&manifest)
	if f := skills.SaveManifest(root, &manifest); f != nil {
		s.markManifestStale()
		return
	}
	s.clearManifestStale()
}

func (s *SkillService) markManifestStale() {
	s.mu.Lock()
	s.manifestStale = true
	s.mu.Unlock()
}

func (s *SkillService) clearManifestStale() {
	s.mu.Lock()
	s.manifestStale = false
	s.mu.Unlock()
}

// manifestStaleFlag reports whether the last manifest write failed or was
// skipped; M102 surfaces it as a warning.
func (s *SkillService) manifestStaleFlag() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.manifestStale
}

// manifestTarget maps a tool id (registry id or registration id) plus the
// installed skill directory to the machine-independent target (技术设计
// 18.4). Project entries keep the actual install subdir, not the registered
// default.
func (s *SkillService) manifestTarget(toolID, targetPath string) (skills.ManifestTarget, bool) {
	return manifestTargetFrom(s.loadTargets().Targets, toolID, targetPath)
}

// manifestTargetFrom is manifestTarget with a preloaded registration list, so
// hot loops (rebuild over thousands of records) read targets.json once.
func manifestTargetFrom(registrations []skills.Target, toolID, targetPath string) (skills.ManifestTarget, bool) {
	if skills.IsTargetID(toolID) {
		for _, record := range registrations {
			if record.ID != toolID {
				continue
			}
			if record.Kind == skills.TargetCustom {
				return skills.ManifestTarget{Kind: "custom", Name: record.Name}, true
			}
			subdir, ok := projectSubdir(record.Path, targetPath)
			if !ok {
				return skills.ManifestTarget{}, false
			}
			return skills.ManifestTarget{Kind: "project", Name: record.Name, Subdir: subdir}, true
		}
		return skills.ManifestTarget{}, false
	}
	if _, ok := skills.ToolByID(toolID); !ok {
		return skills.ManifestTarget{}, false
	}
	return skills.ManifestTarget{Kind: "builtin", ToolID: toolID}, true
}

// projectSubdir derives the project-relative install subdir from the actual
// installed directory (targetPath is <root>/<subdir>/<name>).
func projectSubdir(projectRoot, targetPath string) (string, bool) {
	installDir := filepath.Dir(targetPath)
	rel, err := filepath.Rel(projectRoot, installDir)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", false
	}
	rel = filepath.ToSlash(rel)
	if _, ok := skills.ProjectDirTool(rel); !ok {
		return "", false
	}
	return rel, true
}

// manifestKBItem builds one distribution entry.
func manifestKBItem(target skills.ManifestTarget, name, skillPath, mode string, now time.Time) skills.ManifestItem {
	return skills.ManifestItem{
		Source:    "kb",
		Target:    target,
		Name:      name,
		SkillPath: skillPath,
		Mode:      mode,
		UpdatedAt: now.UTC().Format(time.RFC3339),
	}
}

// manifestGitHubItem builds one GitHub tool-install entry.
func manifestGitHubItem(target skills.ManifestTarget, name, owner, repo, ref, path, commit, pathCommit string, now time.Time) skills.ManifestItem {
	return skills.ManifestItem{
		Source:     "github",
		Target:     target,
		Name:       name,
		Owner:      owner,
		Repo:       repo,
		Ref:        ref,
		Path:       path,
		Commit:     commit,
		PathCommit: pathCommit,
		UpdatedAt:  now.UTC().Format(time.RFC3339),
	}
}
