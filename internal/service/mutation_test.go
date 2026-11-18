package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

func mutationTestApp(t *testing.T) (*App, string, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "raws"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "raws", "旧名.md"), []byte("# 旧名\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	app, err := New(t.TempDir(), filepath.Join(t.TempDir(), "legacy"))
	if err != nil {
		t.Fatal(err)
	}
	v, f := app.Vaults.Register(root)
	if f != nil {
		t.Fatalf("register: %+v", f)
	}
	return app, v.ID, root
}

func TestMutationServiceBuildsPlans(t *testing.T) {
	app, vaultID, _ := mutationTestApp(t)
	svc := NewMutationService(app)
	ctx := context.Background()

	rename := svc.Rename(ctx, vaultID, "raws/旧名.md", "新名.md")
	if rename.Failure != nil || rename.Plan == nil || rename.Plan.Actions[0].Target != "raws/新名.md" {
		t.Fatalf("rename: %+v %+v", rename.Plan, rename.Failure)
	}
	if rename.Plan.Origin != "user" || rename.Plan.Risk != "medium" {
		t.Errorf("plan meta: %+v", rename.Plan)
	}

	create := svc.CreateFile(ctx, vaultID, "raws", "笔记")
	if create.Failure != nil || create.Plan.Actions[0].Target != "raws/笔记.md" {
		t.Fatalf("create file: %+v %+v", create.Plan, create.Failure)
	}
	dir := svc.CreateDir(ctx, vaultID, "raws", "新目录")
	if dir.Failure != nil || dir.Plan.Actions[0].Kind != contract.MutCreateDirectory {
		t.Fatalf("create dir: %+v %+v", dir.Plan, dir.Failure)
	}

	move := svc.PlanMove(ctx, vaultID, []string{"raws/旧名.md"}, "projects")
	if move.Failure != nil || move.Plan.Actions[0].Target != "projects/旧名.md" {
		t.Fatalf("plan move: %+v %+v", move.Plan, move.Failure)
	}

	agent := svc.Plan(ctx, contract.PlanRequest{VaultID: vaultID, Origin: "curator",
		Actions: []contract.MutationAction{{ID: "a", Kind: contract.MutCreate, Target: "raws/x.md"}}})
	if agent.Failure == nil {
		t.Error("agent origin accepted in M1")
	}
}
