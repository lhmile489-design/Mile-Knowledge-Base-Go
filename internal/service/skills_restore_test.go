package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/skills"
)

// M7 T03: M102 judges the manifest against this machine (技术设计 18.5).

func TestRestorePreviewJudgesAndSkips(t *testing.T) {
	home := t.TempDir()
	writeInstallFile(t, filepath.Join(home, ".local", "bin", "claude.cmd"), "cmd")
	svc, vaultID := skillServiceWithHome(t, home)
	root := svc.app.Vaults.List().Vaults[0].Root
	seedVaultSkill(t, root, "skills/前端/demo")

	manifest := skills.Manifest{Items: []skills.ManifestItem{
		{
			Source: "kb", Target: skills.ManifestTarget{Kind: "builtin", ToolID: "claude-code"},
			Name: "demo", SkillPath: "skills/前端/demo", Mode: "link", UpdatedAt: "2026-11-22T09:00:00Z",
		},
		{
			Source: "kb", Target: skills.ManifestTarget{Kind: "custom", Name: "Droid"},
			Name: "x", SkillPath: "skills/前端/demo", Mode: "copy", UpdatedAt: "2026-11-22T09:00:00Z",
		},
	}}
	if f := skills.SaveManifest(root, &manifest); f != nil {
		t.Fatal(f)
	}

	result := svc.RestorePreview(context.Background(), vaultID)
	if result.Failure != nil || result.Preview == nil {
		t.Fatalf("preview = %+v", result)
	}
	preview := result.Preview
	if preview.Status != skills.ManifestOK || len(preview.Items) != 2 || preview.Pending != 1 {
		t.Fatalf("preview = %+v", preview)
	}
	first := preview.Items[0]
	if first.State != "create" || first.TargetLabel != "Claude Code" || !first.DefaultSelected {
		t.Fatalf("first = %+v", first)
	}
	second := preview.Items[1]
	if second.State != "skip" || !strings.Contains(second.Reason, "没有登记") {
		t.Fatalf("second = %+v", second)
	}
	if len(preview.Unmatched) != 1 || preview.Unmatched[0] != "Droid" {
		t.Fatalf("unmatched = %+v", preview.Unmatched)
	}
	// No absolute path may leak into the payload.
	blob := first.TargetLabel + first.Reason + second.TargetLabel + second.Reason
	for _, forbidden := range []string{root, home} {
		if strings.Contains(blob, forbidden) {
			t.Fatalf("payload leaks %q: %s", forbidden, blob)
		}
	}
}

func TestRestorePreviewStates(t *testing.T) {
	svc, vaultID := skillServiceWithHome(t, t.TempDir())
	root := svc.app.Vaults.List().Vaults[0].Root

	missing := svc.RestorePreview(context.Background(), vaultID)
	if missing.Failure != nil || missing.Preview == nil || missing.Preview.Status != skills.ManifestMissing ||
		len(missing.Preview.Items) != 0 || !missing.Preview.CanRebuild {
		t.Fatalf("missing = %+v", missing.Preview)
	}

	path := filepath.Join(root, ".mile-kb", "skill-distributions.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("<<<<<<< HEAD\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	broken := svc.RestorePreview(context.Background(), vaultID)
	if broken.Failure != nil || broken.Preview.Status != skills.ManifestUnreadable || !broken.Preview.CanRebuild {
		t.Fatalf("broken = %+v", broken.Preview)
	}

	if err := os.WriteFile(path, []byte(`{"version":2,"items":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	future := svc.RestorePreview(context.Background(), vaultID)
	if future.Preview == nil || future.Preview.Status != skills.ManifestVersionUnknown || future.Preview.CanRebuild {
		t.Fatalf("future = %+v", future.Preview)
	}
}
