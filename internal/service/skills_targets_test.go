package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/skills"
)

// M6 T04: M97–M101 (API §14, 技术设计 17).

func writeSkillDir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSkillTargetsSaveList(t *testing.T) {
	home := t.TempDir()
	svc, _ := skillServiceWithHome(t, home)
	ctx := context.Background()

	droid := filepath.Join(home, "droid-skills")
	writeSkillDir(t, filepath.Join(droid, "x"))
	newDir := filepath.Join(home, "another")
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatal(err)
	}

	res := svc.SaveTarget(ctx, contract.SkillSaveTargetInput{Kind: "custom", Name: "Droid", Token: svc.newTargetPick(droid)})
	if res.Failure != nil || res.Target == nil {
		t.Fatalf("save = %+v", res)
	}
	saved := *res.Target
	if !skills.IsTargetID(saved.ID) || saved.Kind != "custom" || saved.SkillCount != 1 || !saved.Exists {
		t.Fatalf("target = %+v", saved)
	}
	if !strings.HasPrefix(saved.Path, "<home>") {
		t.Fatalf("path not masked: %q", saved.Path)
	}

	list := svc.ListTargets(ctx)
	if list.Failure != nil || len(list.Items) != 1 || list.Items[0].ID != saved.ID {
		t.Fatalf("list = %+v", list)
	}

	registry := svc.Registry(ctx)
	var custom *contract.SkillTool
	for i := range registry.Tools {
		if registry.Tools[i].ID == saved.ID {
			custom = &registry.Tools[i]
		}
	}
	if custom == nil || custom.Kind != "custom" || custom.Missing || !strings.HasPrefix(custom.InstallDir, "<home>") {
		t.Fatalf("registry entry = %+v", custom)
	}
	if total := len(registry.Tools); total != len(skills.Registry)+1 {
		t.Fatalf("registry size = %d", total)
	}

	// Duplicate name (case-insensitive) is refused.
	dupName := svc.SaveTarget(ctx, contract.SkillSaveTargetInput{Kind: "custom", Name: "droid", Token: svc.newTargetPick(newDir)})
	if dupName.Failure == nil || dupName.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("duplicate name = %+v", dupName)
	}
	// Overlapping directory is refused.
	inside := filepath.Join(droid, "x")
	overlap := svc.SaveTarget(ctx, contract.SkillSaveTargetInput{Kind: "custom", Name: "Inner", Token: svc.newTargetPick(inside)})
	if overlap.Failure == nil || overlap.Failure.Code != contract.CodeSkillConflict {
		t.Fatalf("overlap = %+v", overlap)
	}
	// A directory inside the registered vault is refused.
	vaultRoot := svc.app.Vaults.List().Vaults[0].Root
	inVault := filepath.Join(vaultRoot, "raws")
	inVaultRes := svc.SaveTarget(ctx, contract.SkillSaveTargetInput{Kind: "custom", Name: "InVault", Token: svc.newTargetPick(inVault)})
	if inVaultRes.Failure == nil || inVaultRes.Failure.Code != contract.CodePathEscape {
		t.Fatalf("vault overlap = %+v", inVaultRes)
	}
	// Editing only the name keeps createdAt and the directory.
	edit := svc.SaveTarget(ctx, contract.SkillSaveTargetInput{ID: saved.ID, Name: "Droid Two"})
	if edit.Failure != nil || edit.Target == nil || edit.Target.Name != "Droid Two" || edit.Target.CreatedAt != saved.CreatedAt {
		t.Fatalf("edit = %+v", edit)
	}
	if !strings.HasPrefix(edit.Target.Path, "<home>") || edit.Target.ID != saved.ID {
		t.Fatalf("edit target = %+v", edit.Target)
	}
	// Unknown id is not found.
	missing := svc.SaveTarget(ctx, contract.SkillSaveTargetInput{ID: "t-ffffffffffff", Name: "x"})
	if missing.Failure == nil || missing.Failure.Code != contract.CodeNotFound {
		t.Fatalf("unknown id = %+v", missing)
	}
}

func TestSkillTargetsProjectAndTokenExpiry(t *testing.T) {
	home := t.TempDir()
	svc, _ := skillServiceWithHome(t, home)
	ctx := context.Background()

	project := filepath.Join(home, "proj")
	writeSkillDir(t, filepath.Join(project, ".claude", "skills", "vue"))
	writeSkillDir(t, filepath.Join(project, ".kiro", "skills", "k"))

	res := svc.SaveTarget(ctx, contract.SkillSaveTargetInput{Kind: "project", Name: "Proj", Token: svc.newTargetPick(project)})
	if res.Failure != nil || res.Target == nil {
		t.Fatalf("project save = %+v", res)
	}
	if res.Target.InstallDir != ".claude/skills" {
		t.Fatalf("default install dir = %q", res.Target.InstallDir)
	}
	// .agents/skills ranks before .claude/skills but does not exist here.
	if len(res.Target.Dirs) != 2 || res.Target.Dirs[0].Rel != ".claude/skills" || res.Target.Dirs[0].Skills != 1 {
		t.Fatalf("dirs = %+v", res.Target.Dirs)
	}
	if res.Target.SkillCount != 2 {
		t.Fatalf("skillCount = %d", res.Target.SkillCount)
	}

	// The picker token survives a validation failure and expires after 10m.
	token := svc.newTargetPick(project)
	bad := svc.SaveTarget(ctx, contract.SkillSaveTargetInput{Kind: "project", Name: "Bad", Token: token, InstallDir: "nope/skills"})
	if bad.Failure == nil || bad.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("bad install dir = %+v", bad)
	}
	svc.now = func() time.Time { return time.Now().Add(11 * time.Minute) }
	expired := svc.SaveTarget(ctx, contract.SkillSaveTargetInput{Kind: "project", Name: "Late", Token: token})
	if expired.Failure == nil || expired.Failure.Code != contract.CodeNotFound {
		t.Fatalf("expired token = %+v", expired)
	}
}

func TestSkillTargetsRemoveCleansRecords(t *testing.T) {
	home := t.TempDir()
	svc, _ := skillServiceWithHome(t, home)
	ctx := context.Background()

	custom := filepath.Join(home, "droid")
	writeSkillDir(t, filepath.Join(custom, "x"))
	saved := svc.SaveTarget(ctx, contract.SkillSaveTargetInput{Kind: "custom", Name: "Droid", Token: svc.newTargetPick(custom)})
	if saved.Failure != nil {
		t.Fatalf("save = %+v", saved)
	}
	id := saved.Target.ID

	statePath := filepath.Join(svc.app.DataDir, "skills", "state.json")
	state := skills.LoadState(statePath)
	state.Distributions = append(state.Distributions, skills.Distribution{
		ID: "d-aaaaaaaaaaaa", VaultID: "v1", SkillPath: "skills/通用/x", ToolID: id,
		TargetPath: filepath.Join(custom, "x"), Mode: "copy", Name: "x", BaseHash: "sha256:" + strings.Repeat("a", 64),
	})
	if err := skills.SaveState(statePath, *state); err != nil {
		t.Fatal(err)
	}
	record := skills.ToolSource{
		ToolID: id, TargetPath: filepath.Join(custom, "x"), Owner: "acme", Repo: "skills", Ref: "main", Path: "skills/x",
		Commit: strings.Repeat("1", 40), PathCommit: strings.Repeat("1", 40),
		BaseHash: "sha256:" + strings.Repeat("b", 64), InstalledAt: "2026-11-22T09:00:00Z",
	}
	if f := skills.SaveToolSource(filepath.Join(svc.app.DataDir, "skills"), record); f != nil {
		t.Fatal(f)
	}

	before, err := skills.TreeHash(custom)
	if err != nil {
		t.Fatal(err)
	}
	res := svc.RemoveTarget(ctx, id)
	if res.Failure != nil || !res.Removed || res.DistributionsRemoved != 1 || res.SourcesRemoved != 1 {
		t.Fatalf("remove = %+v", res)
	}
	after, err := skills.TreeHash(custom)
	if err != nil || after != before {
		t.Fatalf("directory bytes changed: %s -> %s (%v)", before, after, err)
	}
	if got := skills.LoadState(statePath); len(got.Distributions) != 0 {
		t.Fatalf("distributions left: %+v", got.Distributions)
	}
	if got := skills.LoadToolSources(filepath.Join(svc.app.DataDir, "skills")); len(got.Records) != 0 {
		t.Fatalf("sources left: %+v", got.Records)
	}
	if list := svc.ListTargets(ctx); len(list.Items) != 0 {
		t.Fatalf("targets left: %+v", list.Items)
	}
	if _, err := os.Stat(filepath.Join(custom, "x", "SKILL.md")); err != nil {
		t.Fatal("files must stay after removing the registration")
	}
	log := svc.Log(ctx, 10)
	found := false
	for _, rec := range log.Records {
		if rec.Op == "unregister" && rec.ToolID == id {
			found = true
		}
	}
	if !found {
		t.Fatalf("no unregister log: %+v", log.Records)
	}
	if again := svc.RemoveTarget(ctx, id); again.Failure == nil || again.Failure.Code != contract.CodeNotFound {
		t.Fatalf("second remove = %+v", again)
	}
}

func TestReregisteredTargetIsScannedAgain(t *testing.T) {
	home := t.TempDir()
	svc, vaultID := skillServiceWithHome(t, home)
	ctx := context.Background()
	seedVaultSkill(t, svc.app.Vaults.List().Vaults[0].Root, "skills/前端/demo")

	custom := filepath.Join(home, "droid")
	if err := os.MkdirAll(custom, 0o755); err != nil {
		t.Fatal(err)
	}
	saved := svc.SaveTarget(ctx, contract.SkillSaveTargetInput{Kind: "custom", Name: "Droid", Token: svc.newTargetPick(custom)})
	if saved.Failure != nil {
		t.Fatalf("save = %+v", saved)
	}
	if op := svc.Distribute(ctx, vaultID, "skills/前端/demo", []contract.SkillDistributeTarget{{TargetID: saved.Target.ID}}, "copy"); op.Failure != nil {
		t.Fatalf("distribute = %+v", op)
	}
	if res := svc.RemoveTarget(ctx, saved.Target.ID); res.Failure != nil {
		t.Fatalf("remove = %+v", res)
	}
	// The copy stays on disk; re-registering the same directory makes the scan
	// see it again — as an independent skill, because its record is gone.
	again := svc.SaveTarget(ctx, contract.SkillSaveTargetInput{Kind: "custom", Name: "Droid Again", Token: svc.newTargetPick(custom)})
	if again.Failure != nil {
		t.Fatalf("re-register = %+v", again)
	}
	scan := svc.Scan(ctx, vaultID)
	if scan.Failure != nil {
		t.Fatalf("scan = %+v", scan)
	}
	found := false
	for _, group := range scan.Scan.Tools {
		if group.ToolID != again.Target.ID {
			continue
		}
		for _, skill := range group.Skills {
			if skill.Name == "demo" {
				found = true
				if skill.Origin != "independent" {
					t.Fatalf("origin after re-register = %q, want independent", skill.Origin)
				}
			}
		}
	}
	if !found {
		t.Fatalf("leftover copy not scanned: %+v", scan.Scan.Tools)
	}
}

func TestMaskPathShortForms(t *testing.T) {
	detector := &skills.Detector{Home: `C:\Users\someone`, LocalAppData: `C:\Users\someone\AppData\Local`}
	cases := map[string]string{
		`D:\site`:                          `D:\…\site`,
		`D:\`:                              `D:\…\`,
		`C:\Users\someone\x`:               `<home>\x`,
		`C:\Users\someone\AppData\Local\y`: `<local-app-data>\y`,
	}
	for input, want := range cases {
		if got := maskPath(input, detector); got != want {
			t.Fatalf("maskPath(%q) = %q, want %q", input, got, want)
		}
	}
}
