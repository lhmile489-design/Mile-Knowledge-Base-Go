package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/skills"
)

// M6 T05: distribute / uninstall / adopt / install / update resolve registered
// targets (技术设计 17.5, V-DIST-1/2).

func seedVaultSkill(t *testing.T, vaultRoot, rel string) {
	t.Helper()
	dir := filepath.Join(vaultRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func targetIDs(t *testing.T, svc *SkillService) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, item := range svc.ListTargets(context.Background()).Items {
		out[item.Name] = item.ID
	}
	return out
}

func TestDistributeToCustomAndProject(t *testing.T) {
	home := t.TempDir()
	svc, vaultID := skillServiceWithHome(t, home)
	ctx := context.Background()
	vaultRoot := svc.app.Vaults.List().Vaults[0].Root
	seedVaultSkill(t, vaultRoot, "skills/前端/demo")

	custom := filepath.Join(home, "droid")
	if err := os.MkdirAll(custom, 0o755); err != nil {
		t.Fatal(err)
	}
	if res := svc.SaveTarget(ctx, contract.SkillSaveTargetInput{Kind: "custom", Name: "Droid", Token: svc.newTargetPick(custom)}); res.Failure != nil {
		t.Fatalf("custom save: %+v", res.Failure)
	}
	project := filepath.Join(home, "proj")
	if err := os.MkdirAll(filepath.Join(project, ".claude", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if res := svc.SaveTarget(ctx, contract.SkillSaveTargetInput{Kind: "project", Name: "Proj", Token: svc.newTargetPick(project)}); res.Failure != nil {
		t.Fatalf("project save: %+v", res.Failure)
	}
	ids := targetIDs(t, svc)

	// Custom tool: copy in, record written under the registration id.
	preview := svc.DistributePreview(ctx, vaultID, "skills/前端/demo", []contract.SkillDistributeTarget{{TargetID: ids["Droid"]}}, "copy")
	if preview.Failure != nil || len(preview.Items) != 1 || preview.Items[0].Action != "create" {
		t.Fatalf("custom preview = %+v", preview)
	}
	op := svc.Distribute(ctx, vaultID, "skills/前端/demo", []contract.SkillDistributeTarget{{TargetID: ids["Droid"]}}, "copy")
	if op.Failure != nil || len(op.Items) != 1 || op.Items[0].Result != "copied" {
		t.Fatalf("custom distribute = %+v", op)
	}
	if _, err := os.Stat(filepath.Join(custom, "demo", "SKILL.md")); err != nil {
		t.Fatalf("custom copy missing: %v", err)
	}
	state := skills.LoadState(filepath.Join(svc.app.DataDir, "skills", "state.json"))
	if len(state.Distributions) != 1 || state.Distributions[0].ToolID != ids["Droid"] {
		t.Fatalf("distribution = %+v", state.Distributions)
	}

	// Project: writes into installRel; the wire spec addresses the row.
	projTarget := contract.SkillDistributeTarget{TargetID: ids["Proj"]}
	if op := svc.Distribute(ctx, vaultID, "skills/前端/demo", []contract.SkillDistributeTarget{projTarget}, "copy"); op.Failure != nil || op.Items[0].Result != "copied" {
		t.Fatalf("project distribute = %+v", op)
	}
	targetPath := filepath.Join(project, ".claude", "skills", "demo")
	if _, err := os.Stat(filepath.Join(targetPath, "SKILL.md")); err != nil {
		t.Fatalf("project copy missing: %v", err)
	}
	unPreview := svc.UninstallPreview(ctx, ids["Proj"], "project:.claude/skills:demo")
	if unPreview.Failure != nil || unPreview.Preview.Kind != "copy" {
		t.Fatalf("project uninstall preview = %+v", unPreview)
	}
	if res := svc.Uninstall(ctx, ids["Proj"], "project:.claude/skills:demo"); res.Failure != nil || res.Items[0].Result != "removed" {
		t.Fatalf("project uninstall = %+v", res)
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("project copy still there: %v", err)
	}
	// A directory outside the project's list is refused.
	bad := svc.UninstallPreview(ctx, ids["Proj"], "project:docs/skills:demo")
	if bad.Failure == nil || bad.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("bad project dir = %+v", bad)
	}
	// Unknown registration id is refused.
	unknown := svc.UninstallPreview(ctx, "t-ffffffffffff", "install:demo")
	if unknown.Failure == nil || unknown.Failure.Code != contract.CodeNotFound {
		t.Fatalf("removed target = %+v", unknown)
	}
}

func TestDistributeRejectsMissingTarget(t *testing.T) {
	home := t.TempDir()
	svc, vaultID := skillServiceWithHome(t, home)
	ctx := context.Background()
	seedVaultSkill(t, svc.app.Vaults.List().Vaults[0].Root, "skills/前端/demo")

	custom := filepath.Join(home, "droid")
	if err := os.MkdirAll(custom, 0o755); err != nil {
		t.Fatal(err)
	}
	res := svc.SaveTarget(ctx, contract.SkillSaveTargetInput{Kind: "custom", Name: "Droid", Token: svc.newTargetPick(custom)})
	if res.Failure != nil {
		t.Fatalf("save: %+v", res.Failure)
	}
	if err := os.RemoveAll(custom); err != nil {
		t.Fatal(err)
	}
	op := svc.Distribute(ctx, vaultID, "skills/前端/demo", []contract.SkillDistributeTarget{{TargetID: res.Target.ID}}, "copy")
	if op.Failure == nil || op.Failure.Code != contract.CodeNotFound {
		t.Fatalf("missing target distribute = %+v", op)
	}
	if _, f := svc.resolveInstallTargets([]contract.SkillDistributeTarget{{TargetID: res.Target.ID}}); f == nil || f.Code != contract.CodeNotFound {
		t.Fatalf("missing target install = %+v", f)
	}
}
