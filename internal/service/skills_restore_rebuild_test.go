package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/skills"
)

// M7 T05: M104 rebuilds the manifest from local records (技术设计 18.6,
// V-REB-1).

func TestRebuildManifestFromLocalRecords(t *testing.T) {
	home := t.TempDir()
	writeInstallFile(t, filepath.Join(home, ".local", "bin", "claude.cmd"), "cmd")
	svc, vaultID := skillServiceWithHome(t, home)
	root := svc.app.Vaults.List().Vaults[0].Root
	seedVaultSkill(t, root, "skills/前端/demo")

	// One kernel distribution, one GitHub tool source, one distribution whose
	// registration no longer exists.
	state := skills.LoadState(filepath.Join(svc.app.DataDir, "skills", "state.json"))
	state.AddDistribution(skills.Distribution{
		VaultID: vaultID, SkillPath: "skills/前端/demo", ToolID: "claude-code",
		TargetPath: filepath.Join(home, ".claude", "skills", "demo"), Mode: "link", Name: "demo",
	})
	state.AddDistribution(skills.Distribution{
		VaultID: vaultID, SkillPath: "skills/前端/demo", ToolID: "t-ffffffffffff",
		TargetPath: filepath.Join(home, "gone", "demo"), Mode: "copy", Name: "demo",
	})
	if err := skills.SaveState(filepath.Join(svc.app.DataDir, "skills", "state.json"), *state); err != nil {
		t.Fatal(err)
	}
	toolDir := filepath.Join(home, ".claude", "skills", "pdf")
	writeInstallFile(t, filepath.Join(toolDir, "SKILL.md"), "# pdf\n")
	if f := skills.SaveToolSource(filepath.Join(svc.app.DataDir, "skills"), skills.ToolSource{
		ToolID: "claude-code", TargetPath: toolDir, Owner: "acme", Repo: "skills", Ref: "main",
		Path: "skills/pdf", Commit: installTestSHA, PathCommit: installTestSHA,
		BaseHash: "sha256:" + strings.Repeat("a", 64), InstalledAt: "2026-11-22T09:00:00Z",
	}); f != nil {
		t.Fatal(f)
	}

	path := filepath.Join(root, ".mile-kb", "skill-distributions.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("<<<<<<< HEAD\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := svc.RebuildManifest(context.Background(), vaultID)
	if result.Failure != nil || result.Written != 2 || result.Skipped != 1 || result.CorruptCopy == "" {
		t.Fatalf("rebuild = %+v", result)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), result.CorruptCopy)); err != nil {
		t.Fatalf("corrupt copy missing: %v", err)
	}
	manifest := skills.LoadManifest(root)
	if manifest.Status != skills.ManifestOK || len(manifest.Items) != 2 {
		t.Fatalf("manifest = %+v", manifest)
	}
	names := map[string]string{}
	for _, item := range manifest.Items {
		names[item.Name] = item.Source
	}
	if names["demo"] != "kb" || names["pdf"] != "github" {
		t.Fatalf("items = %+v", manifest.Items)
	}
}

func TestRebuildManifestMissingAndFuture(t *testing.T) {
	svc, vaultID := skillServiceWithHome(t, t.TempDir())
	root := svc.app.Vaults.List().Vaults[0].Root

	missing := svc.RebuildManifest(context.Background(), vaultID)
	if missing.Failure != nil || missing.Written != 0 || missing.CorruptCopy != "" {
		t.Fatalf("missing rebuild = %+v", missing)
	}
	if manifest := skills.LoadManifest(root); manifest.Status != skills.ManifestOK {
		t.Fatalf("manifest = %+v", manifest)
	}

	path := filepath.Join(root, ".mile-kb", "skill-distributions.json")
	if err := os.WriteFile(path, []byte(`{"version":2,"items":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	future := svc.RebuildManifest(context.Background(), vaultID)
	if future.Failure == nil || future.Failure.Code != contract.CodeUnsupported {
		t.Fatalf("future rebuild = %+v", future)
	}
	body, err := os.ReadFile(path)
	if err != nil || string(body) != `{"version":2,"items":[]}` {
		t.Fatalf("future manifest rewritten: %q %v", body, err)
	}
}

func TestRebuildManifestSkipsGoneTargets(t *testing.T) {
	home := t.TempDir()
	writeInstallFile(t, filepath.Join(home, ".local", "bin", "claude.cmd"), "cmd")
	svc, vaultID := skillServiceWithHome(t, home)

	// A source record whose directory is gone must not enter the manifest.
	if f := skills.SaveToolSource(filepath.Join(svc.app.DataDir, "skills"), skills.ToolSource{
		ToolID: "claude-code", TargetPath: filepath.Join(home, ".claude", "skills", "gone"),
		Owner: "acme", Repo: "skills", Ref: "main", Path: "skills/gone",
		Commit: installTestSHA, PathCommit: installTestSHA,
		BaseHash: "sha256:" + strings.Repeat("a", 64), InstalledAt: "2026-11-22T09:00:00Z",
	}); f != nil {
		t.Fatal(f)
	}
	result := svc.RebuildManifest(context.Background(), vaultID)
	if result.Failure != nil || result.Written != 0 || result.Skipped != 1 {
		t.Fatalf("rebuild = %+v", result)
	}
}
