package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/vault"
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

func TestMutationServiceExecuteEditAndMove(t *testing.T) {
	app, vaultID, root := mutationTestApp(t)
	svc := NewMutationService(app)
	ctx := context.Background()

	// A BOM + CRLF file must survive an edit plan unchanged in encoding.
	target := filepath.Join(root, "raws", "编码.md")
	bomOld := string(rune(0xFEFF)) + "# 旧\r\n"
	bomNew := string(rune(0xFEFF)) + "# 新\r\n"
	if err := os.WriteFile(target, []byte(bomOld), 0o644); err != nil {
		t.Fatal(err)
	}

	rev, rf := vaultReadRevision(t, root, "raws/编码.md")
	if rf != nil {
		t.Fatal(rf)
	}
	plan := svc.Plan(ctx, contract.PlanRequest{VaultID: vaultID, Origin: "user",
		Actions: []contract.MutationAction{{
			ID: "a", Kind: contract.MutEdit, Target: "raws/编码.md", Text: ptrText("# 新\n"),
			ExpectedTargetRevision: &rev,
		}}})
	if plan.Failure != nil || plan.Plan == nil {
		t.Fatalf("plan: %+v", plan.Failure)
	}
	report := svc.Execute(ctx, *plan.Plan)
	if report.Status != "completed" {
		t.Fatalf("report = %+v", report)
	}
	body, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != bomNew {
		t.Errorf("encoding lost: %q", body)
	}
	if report.TransactionID == "" {
		t.Error("edit must open a transaction")
	}
	if _, err := os.Stat(filepath.Join(root, ".mile-kb", "transactions", report.TransactionID, "manifest.json")); err != nil {
		t.Errorf("manifest missing: %v", err)
	}

	// A stale revision must fail the whole action and leave the file alone.
	bad := svc.Plan(ctx, contract.PlanRequest{VaultID: vaultID, Origin: "user",
		Actions: []contract.MutationAction{{
			ID: "a", Kind: contract.MutEdit, Target: "raws/编码.md", Text: ptrText("坏\n"),
			ExpectedTargetRevision: ptrInt(99),
		}}})
	if bad.Failure != nil {
		t.Fatalf("plan: %+v", bad.Failure)
	}
	badReport := svc.Execute(ctx, *bad.Plan)
	if badReport.Status != "failed" || badReport.Failed[0].ErrorCode != contract.CodeRevisionMismatch {
		t.Fatalf("stale revision report = %+v", badReport)
	}
	if body, _ := os.ReadFile(target); string(body) != bomNew {
		t.Errorf("stale write changed the file: %q", body)
	}

	// A rename through a plan keeps the transaction.
	move := svc.Rename(ctx, vaultID, "raws/旧名.md", "新名.md")
	if move.Failure != nil {
		t.Fatalf("rename plan: %+v", move.Failure)
	}
	moveReport := svc.Execute(ctx, *move.Plan)
	if moveReport.Status != "completed" {
		t.Fatalf("move report = %+v", moveReport)
	}
	if _, err := os.Stat(filepath.Join(root, "raws", "新名.md")); err != nil {
		t.Errorf("moved file missing: %v", err)
	}
}

// vaultReadRevision reads the current revision through the same package the
// executor writes with.
func vaultReadRevision(t *testing.T, root, rel string) (int64, *contract.Failure) {
	t.Helper()
	snap, f := vault.Read(root, rel)
	return snap.Revision, f
}

func ptrText(v string) *string { return &v }
func ptrInt(v int64) *int64    { return &v }
