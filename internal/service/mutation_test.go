package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/ai"
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
	// Tests never touch the real OS secret store.
	app.Keys = ai.NewMemoryKeyStore()
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

	// M2 accepts agent origins, but only through the permission gate.
	agent := svc.Plan(ctx, contract.PlanRequest{VaultID: vaultID, Origin: "curator",
		Actions: []contract.MutationAction{{ID: "a", Kind: contract.MutCreate, Target: "raws/x.md"}}})
	if agent.Failure != nil || agent.Plan == nil {
		t.Errorf("agent origin should be accepted in M2: %+v", agent)
	}
	denied := svc.Plan(ctx, contract.PlanRequest{VaultID: vaultID, Origin: "curator",
		Actions: []contract.MutationAction{{ID: "a", Kind: contract.MutCreate, Target: ".mile-kb/settings/x.md"}}})
	if denied.Failure == nil || denied.Failure.Code != contract.CodePermissionDenied {
		t.Errorf("agent writing settings: %+v", denied)
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

func TestMutationServiceAuditAndRewind(t *testing.T) {
	app, vaultID, root := mutationTestApp(t)
	svc := NewMutationService(app)
	ctx := context.Background()

	target := filepath.Join(root, "raws", "审计.md")
	if err := os.WriteFile(target, []byte("原始"), 0o644); err != nil {
		t.Fatal(err)
	}
	rev, rf := vaultReadRevision(t, root, "raws/审计.md")
	if rf != nil {
		t.Fatal(rf)
	}
	plan := svc.Plan(ctx, contract.PlanRequest{VaultID: vaultID, Origin: "user",
		Actions: []contract.MutationAction{{
			ID: "a1", Kind: contract.MutEdit, Target: "raws/审计.md", Text: ptrText("改过"),
			ExpectedTargetRevision: &rev,
		}}})
	if plan.Failure != nil {
		t.Fatalf("plan: %+v", plan.Failure)
	}
	report := svc.Execute(ctx, *plan.Plan)
	if report.Status != "completed" {
		t.Fatalf("report = %+v", report)
	}

	// The ledger holds the executed record and the transaction summary.
	list := svc.AuditList(ctx, vaultID, "", 50)
	if len(list.Records) != 1 || list.Records[0].Status != "completed" {
		t.Fatalf("records = %+v", list.Records)
	}
	if len(list.Transactions) != 1 || list.Transactions[0].ID != report.TransactionID {
		t.Fatalf("transactions = %+v", list.Transactions)
	}

	preview := svc.RewindPrepare(ctx, vaultID, report.TransactionID)
	if preview.Failure != nil || len(preview.Entries) != 1 || !preview.Entries[0].Restorable {
		t.Fatalf("preview = %+v %+v", preview, preview.Failure)
	}
	restored := svc.RewindCommit(ctx, vaultID, report.TransactionID)
	if restored.Failure != nil || len(restored.Restored) != 1 || len(restored.Conflicted) != 0 {
		t.Fatalf("restore = %+v", restored)
	}
	body, _ := os.ReadFile(target)
	if string(body) != "原始" {
		t.Errorf("body = %q", body)
	}
	list = svc.AuditList(ctx, vaultID, plan.Plan.ID, 50)
	if len(list.Records) != 1 || list.Records[0].Status != "reverted" {
		t.Fatalf("record status = %+v", list.Records)
	}

	// A second round: external change after commit must conflict on rewind.
	rev, _ = vaultReadRevision(t, root, "raws/审计.md")
	again := svc.Plan(ctx, contract.PlanRequest{VaultID: vaultID, Origin: "user",
		Actions: []contract.MutationAction{{
			ID: "a2", Kind: contract.MutEdit, Target: "raws/审计.md", Text: ptrText("第二次"),
			ExpectedTargetRevision: &rev,
		}}})
	second := svc.Execute(ctx, *again.Plan)
	if second.Status != "completed" {
		t.Fatalf("second report = %+v", second)
	}
	if err := os.WriteFile(target, []byte("外部改"), 0o644); err != nil {
		t.Fatal(err)
	}
	conflictPreview := svc.RewindPrepare(ctx, vaultID, second.TransactionID)
	if conflictPreview.Entries[0].Restorable {
		t.Fatalf("preview should be blocked: %+v", conflictPreview.Entries)
	}
	conflictResult := svc.RewindCommit(ctx, vaultID, second.TransactionID)
	if len(conflictResult.Conflicted) != 1 || len(conflictResult.Restored) != 0 {
		t.Fatalf("conflict result = %+v", conflictResult)
	}
	if body, _ := os.ReadFile(target); string(body) != "外部改" {
		t.Errorf("conflict overwrote: %q", body)
	}
}

func TestMutationServiceRejectsBadIDs(t *testing.T) {
	app, _, _ := mutationTestApp(t)
	svc := NewMutationService(app)
	ctx := context.Background()

	if got := svc.AuditList(ctx, "../bad", "", 10); got.Failure == nil || got.Failure.Code != contract.CodeInvalidInput {
		t.Errorf("auditList bad vault accepted: %+v", got.Failure)
	}
	if got := svc.RewindPrepare(ctx, "../bad", "tx"); got.Failure == nil || got.Failure.Code != contract.CodeInvalidInput {
		t.Errorf("rewindPrepare bad vault accepted: %+v", got.Failure)
	}
	if got := svc.RewindCommit(ctx, "../bad", "tx"); got.Failure == nil || got.Failure.Code != contract.CodeInvalidInput {
		t.Errorf("rewindCommit bad vault accepted: %+v", got.Failure)
	}
}

func TestProvenanceRecordedOnCopyAndSyncedByEdit(t *testing.T) {
	app, vaultID, root := mutationTestApp(t)
	svc := NewMutationService(app)
	ctx := context.Background()

	writeServiceFile(t, root, "raws/来源.md", "第一版")
	copyPlan := svc.Plan(ctx, contract.PlanRequest{VaultID: vaultID, Origin: "user",
		Actions: []contract.MutationAction{{
			ID: "a1", Kind: contract.MutCopy, Source: "raws/来源.md", Target: "projects/来源.md",
		}}})
	if copyPlan.Failure != nil {
		t.Fatalf("plan: %+v", copyPlan.Failure)
	}
	if report := svc.Execute(ctx, *copyPlan.Plan); report.Status != "completed" {
		t.Fatalf("copy report = %+v", report)
	}

	list := svc.ProvenanceList(ctx, vaultID)
	if len(list.Entries) != 1 {
		t.Fatalf("entries = %+v", list.Entries)
	}
	entry := list.Entries[0]
	if entry.CopyPath != "projects/来源.md" || entry.SourcePath != "raws/来源.md" {
		t.Errorf("entry = %+v", entry)
	}
	if entry.Assessment.State != "up-to-date" || entry.Assessment.CanSyncFromSource {
		t.Errorf("initial assessment = %+v", entry.Assessment)
	}

	// Source moves on: the copy is untouched, so a sync is offered.
	writeServiceFile(t, root, "raws/来源.md", "第二版")
	list = svc.ProvenanceList(ctx, vaultID)
	if list.Entries[0].Assessment.State != "source-changed" || !list.Entries[0].Assessment.CanSyncFromSource {
		t.Fatalf("assessment = %+v", list.Entries[0].Assessment)
	}

	copyRev, rf := vaultReadRevision(t, root, "projects/来源.md")
	if rf != nil {
		t.Fatal(rf)
	}
	syncPlan := svc.Plan(ctx, contract.PlanRequest{VaultID: vaultID, Origin: "user",
		Actions: []contract.MutationAction{{
			ID: "a1", Kind: contract.MutEdit, Target: "projects/来源.md", Text: ptrText("第二版"),
			ExpectedTargetRevision: &copyRev,
		}}})
	if syncPlan.Failure != nil {
		t.Fatalf("sync plan: %+v", syncPlan.Failure)
	}
	if report := svc.Execute(ctx, *syncPlan.Plan); report.Status != "completed" {
		t.Fatalf("sync report = %+v", report)
	}
	list = svc.ProvenanceList(ctx, vaultID)
	if list.Entries[0].Assessment.State != "up-to-date" {
		t.Fatalf("after sync = %+v", list.Entries[0].Assessment)
	}
	sourceSnap, _ := vault.Read(root, "raws/来源.md")
	if list.Entries[0].BaseHash != sourceSnap.Hash || list.Entries[0].BaseRevision != sourceSnap.Revision {
		t.Errorf("baseline not updated: %+v", list.Entries[0])
	}

	// A copy outside projects/ never gets a relationship.
	outside := svc.Plan(ctx, contract.PlanRequest{VaultID: vaultID, Origin: "user",
		Actions: []contract.MutationAction{{
			ID: "a1", Kind: contract.MutCopy, Source: "raws/来源.md", Target: "raws/副本.md",
		}}})
	if report := svc.Execute(ctx, *outside.Plan); report.Status != "completed" {
		t.Fatalf("plain copy = %+v", report)
	}
	if got := svc.ProvenanceList(ctx, vaultID); len(got.Entries) != 1 {
		t.Errorf("plain copy created a record: %+v", got.Entries)
	}
}

func writeServiceFile(t *testing.T, root, rel, body string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
