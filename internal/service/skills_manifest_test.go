package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/skills"
)

// M7 T02: every successful write path updates the vault manifest
// (技术设计 18.3, V-HOOK-1/2).

func manifestOf(t *testing.T, svc *SkillService) skills.Manifest {
	t.Helper()
	root := svc.app.Vaults.List().Vaults[0].Root
	return skills.LoadManifest(root)
}

func activateVault(t *testing.T, svc *SkillService, vaultID string) {
	t.Helper()
	if f := svc.app.Vaults.Open(vaultID); f != nil {
		t.Fatalf("open vault: %+v", f)
	}
	if svc.app.Vaults.Active() != vaultID {
		t.Fatalf("active = %q", svc.app.Vaults.Active())
	}
}

func TestDistributeAndUninstallUpdateManifest(t *testing.T) {
	home := t.TempDir()
	writeInstallFile(t, filepath.Join(home, ".local", "bin", "claude.cmd"), "cmd")
	svc, vaultID := skillServiceWithHome(t, home)
	activateVault(t, svc, vaultID)
	ctx := context.Background()
	root := svc.app.Vaults.List().Vaults[0].Root
	seedVaultSkill(t, root, "skills/前端/demo")

	if res := svc.Distribute(ctx, vaultID, "skills/前端/demo",
		[]contract.SkillDistributeTarget{{TargetID: "claude-code"}}, "copy"); res.Failure != nil {
		t.Fatalf("distribute: %+v", res.Failure)
	}
	manifest := manifestOf(t, svc)
	if manifest.Status != skills.ManifestOK || len(manifest.Items) != 1 {
		t.Fatalf("manifest = %+v", manifest)
	}
	item := manifest.Items[0]
	if item.Source != "kb" || item.Name != "demo" || item.SkillPath != "skills/前端/demo" || item.Mode != "copy" {
		t.Fatalf("item = %+v", item)
	}
	if item.Target.Kind != "builtin" || item.Target.ToolID != "claude-code" {
		t.Fatalf("target = %+v", item.Target)
	}

	if res := svc.Uninstall(ctx, "claude-code", "install:demo"); res.Failure != nil {
		t.Fatalf("uninstall: %+v", res.Failure)
	}
	if after := manifestOf(t, svc); len(after.Items) != 0 {
		t.Fatalf("items after uninstall = %+v", after.Items)
	}
}

func TestDistributeProjectWritesSubdir(t *testing.T) {
	home := t.TempDir()
	svc, vaultID := skillServiceWithHome(t, home)
	ctx := context.Background()
	root := svc.app.Vaults.List().Vaults[0].Root
	seedVaultSkill(t, root, "skills/前端/demo")

	project := filepath.Join(home, "proj")
	if err := os.MkdirAll(filepath.Join(project, ".claude", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if res := svc.SaveTarget(ctx, contract.SkillSaveTargetInput{Kind: "project", Name: "Proj", Token: svc.newTargetPick(project)}); res.Failure != nil {
		t.Fatalf("save target: %+v", res.Failure)
	}
	ids := targetIDs(t, svc)
	if res := svc.Distribute(ctx, vaultID, "skills/前端/demo",
		[]contract.SkillDistributeTarget{{TargetID: ids["Proj"]}}, "link"); res.Failure != nil {
		t.Fatalf("distribute: %+v", res.Failure)
	}
	manifest := manifestOf(t, svc)
	if len(manifest.Items) != 1 {
		t.Fatalf("manifest = %+v", manifest)
	}
	target := manifest.Items[0].Target
	if target.Kind != "project" || target.Name != "Proj" || target.Subdir != ".claude/skills" {
		t.Fatalf("target = %+v", target)
	}
	if manifest.Items[0].Mode != "link" {
		t.Fatalf("mode = %q", manifest.Items[0].Mode)
	}
}

func TestInstallIntoTokenWritesGitHubItem(t *testing.T) {
	home := t.TempDir()
	writeInstallFile(t, filepath.Join(home, ".local", "bin", "claude.cmd"), "cmd")
	svc, vaultID := skillServiceWithHome(t, home)
	stage := stagedForInstall(t, svc, vaultID)
	target := contract.SkillInstallTarget{Kind: "tools", Targets: []contract.SkillDistributeTarget{{TargetID: "claude-code"}}}
	result := svc.Install(context.Background(), vaultID, stage.StageID, []string{"skills/pdf"}, target)
	if result.Failure != nil || result.Items[0].Result != "copied" {
		t.Fatalf("install = %+v", result)
	}
	manifest := manifestOf(t, svc)
	if len(manifest.Items) != 1 {
		t.Fatalf("manifest = %+v", manifest)
	}
	item := manifest.Items[0]
	if item.Source != "github" || item.Name != "pdf" || item.Owner != "acme" || item.Repo != "skills" ||
		item.Path != "skills/pdf" || item.Commit != installTestSHA || item.PathCommit != installTestSHA {
		t.Fatalf("item = %+v", item)
	}
	if item.Target.Kind != "builtin" || item.Target.ToolID != "claude-code" {
		t.Fatalf("target = %+v", item.Target)
	}
}

func TestToolUpdateRefreshesManifestCommit(t *testing.T) {
	home := t.TempDir()
	writeInstallFile(t, filepath.Join(home, ".local", "bin", "claude.cmd"), "cmd")
	svc, vaultID := skillServiceWithHome(t, home)
	activateVault(t, svc, vaultID)
	var requests int64
	server := updateExecServer(t, updateArchive(t), updateTestNewSHA, updateTestPathSHA, &requests)
	injectUpdateClient(svc, server)

	dir := filepath.Join(home, ".claude", "skills", "pdf")
	writeInstallFile(t, filepath.Join(dir, "SKILL.md"), "# old\n")
	hash, err := skills.TreeHash(dir)
	if err != nil {
		t.Fatal(err)
	}
	skillsDir := filepath.Join(svc.app.DataDir, "skills")
	record := skills.ToolSource{
		ToolID: "claude-code", TargetPath: dir, Owner: "acme", Repo: "skills", Ref: "main", Path: "skills/pdf",
		Commit: updateTestSHA, PathCommit: updateTestSHA, BaseHash: hash, InstalledAt: "2026-11-21T09:00:00Z",
	}
	if f := skills.SaveToolSource(skillsDir, record); f != nil {
		t.Fatal(f)
	}
	// Seed the manifest with the older commit so the update must refresh it.
	root := svc.app.Vaults.List().Vaults[0].Root
	seed := skills.Manifest{Items: []skills.ManifestItem{{
		Source: "github", Target: skills.ManifestTarget{Kind: "builtin", ToolID: "claude-code"}, Name: "pdf",
		Owner: "acme", Repo: "skills", Ref: "main", Path: "skills/pdf",
		Commit: updateTestSHA, PathCommit: updateTestSHA, UpdatedAt: "2026-11-21T09:00:00Z",
	}}}
	if f := skills.SaveManifest(root, &seed); f != nil {
		t.Fatal(f)
	}

	target := contract.SkillUpdateTarget{Kind: "tool", ToolID: "claude-code", Path: "install:pdf"}
	preview := svc.UpdatePreview(context.Background(), "", target)
	if preview.Failure != nil || preview.StageID == "" {
		t.Fatalf("preview = %+v", preview)
	}
	if result := svc.UpdateTool(context.Background(), preview.StageID); result.Failure != nil {
		t.Fatalf("update = %+v", result.Failure)
	}
	manifest := manifestOf(t, svc)
	if len(manifest.Items) != 1 || manifest.Items[0].Commit != updateTestNewSHA || manifest.Items[0].PathCommit != updateTestPathSHA {
		t.Fatalf("manifest = %+v", manifest.Items)
	}
}

func TestAdoptLinkBackWritesLinkItem(t *testing.T) {
	home := t.TempDir()
	writeInstallFile(t, filepath.Join(home, ".local", "bin", "claude.cmd"), "cmd")
	toolDir := filepath.Join(home, ".claude", "skills", "brainstorm")
	writeInstallFile(t, filepath.Join(toolDir, "SKILL.md"), "# brainstorm\n")
	svc, vaultID := skillServiceWithHome(t, home)

	result := svc.Adopt(context.Background(), vaultID, "claude-code", "install:brainstorm", "写作", true, nil)
	if result.Failure != nil || !result.Linked {
		t.Fatalf("adopt = %+v", result)
	}
	manifest := manifestOf(t, svc)
	if len(manifest.Items) != 1 {
		t.Fatalf("manifest = %+v", manifest)
	}
	item := manifest.Items[0]
	if item.Source != "kb" || item.Mode != "link" || item.SkillPath != "skills/写作/brainstorm" || item.Name != "brainstorm" {
		t.Fatalf("item = %+v", item)
	}
	if item.Target.Kind != "builtin" || item.Target.ToolID != "claude-code" {
		t.Fatalf("target = %+v", item.Target)
	}
}

func TestManifestUnreadableStopsWrites(t *testing.T) {
	home := t.TempDir()
	writeInstallFile(t, filepath.Join(home, ".local", "bin", "claude.cmd"), "cmd")
	svc, vaultID := skillServiceWithHome(t, home)
	root := svc.app.Vaults.List().Vaults[0].Root
	seedVaultSkill(t, root, "skills/前端/demo")
	path := filepath.Join(root, ".mile-kb", "skill-distributions.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	broken := []byte("<<<<<<< HEAD\n{\"version\":1}\n=======\n{\"version\":1}\n>>>>>>> x\n")
	if err := os.WriteFile(path, broken, 0o644); err != nil {
		t.Fatal(err)
	}

	result := svc.Distribute(context.Background(), vaultID, "skills/前端/demo",
		[]contract.SkillDistributeTarget{{TargetID: "claude-code"}}, "copy")
	if result.Failure != nil || result.Items[0].Result != "copied" {
		t.Fatalf("distribute must still succeed: %+v", result)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(broken) {
		t.Fatal("the unreadable manifest must be left byte-identical")
	}
	if !svc.manifestStaleFlag() {
		t.Fatal("M102 should be able to report the stale manifest")
	}
}

func TestManifestVersionUnknownStopsWrites(t *testing.T) {
	svc, vaultID := skillServiceWithHome(t, t.TempDir())
	root := svc.app.Vaults.List().Vaults[0].Root
	seedVaultSkill(t, root, "skills/前端/demo")
	path := filepath.Join(root, ".mile-kb", "skill-distributions.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"version":2,"items":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(t.TempDir(), "proj")
	if err := os.MkdirAll(filepath.Join(project, ".claude", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if res := svc.SaveTarget(context.Background(), contract.SkillSaveTargetInput{Kind: "project", Name: "P", Token: svc.newTargetPick(project)}); res.Failure != nil {
		t.Fatalf("save target: %+v", res.Failure)
	}
	ids := targetIDs(t, svc)
	if res := svc.Distribute(context.Background(), vaultID, "skills/前端/demo",
		[]contract.SkillDistributeTarget{{TargetID: ids["P"]}}, "copy"); res.Failure != nil {
		t.Fatalf("distribute: %+v", res.Failure)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != `{"version":2,"items":[]}` {
		t.Fatal("a future-version manifest must never be rewritten")
	}
}
