package mutation

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

func importStage(t *testing.T) (string, string, string) {
	t.Helper()
	staging := t.TempDir()
	stageID := "s-0123456789ab"
	file := filepath.Join(staging, stageID, "root", "docs", "a.bin")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte{0x00, 0x01, 0xff, 'a'}
	if err := os.WriteFile(file, body, 0o644); err != nil {
		t.Fatal(err)
	}
	hash, err := stagedFileHash(file)
	if err != nil {
		t.Fatal(err)
	}
	return staging, stageID, hash
}

func importAction(stageID, hash, target string, expected *int64) contract.MutationAction {
	return contract.MutationAction{
		ID: "imp", Kind: contract.MutImportFile, Target: target,
		StageID: stageID, StagePath: "docs/a.bin", StageHash: hash,
		ExpectedTargetRevision: expected, Reason: "更新技能文件",
	}
}

func TestPlanImportFileRules(t *testing.T) {
	root := t.TempDir()
	staging, stageID, hash := importStage(t)
	builder := Builder{Root: root, StagingRoot: staging}

	plan, f := builder.Plan(contract.PlanRequest{VaultID: "v", Origin: "user", Actions: []contract.MutationAction{importAction(stageID, hash, "skills/文档/pdf/img/a.bin", nil)}})
	if f != nil || plan.Risk != "low" || len(plan.Actions) != 1 {
		t.Fatalf("create plan = %+v %+v", plan, f)
	}

	if _, f := builder.Plan(contract.PlanRequest{VaultID: "v", Origin: "curator", Actions: []contract.MutationAction{importAction(stageID, hash, "x.bin", nil)}}); f == nil || f.Code != contract.CodePermissionDenied {
		t.Fatalf("agent plan = %+v", f)
	}

	traversal := importAction(stageID, hash, "x.bin", nil)
	traversal.StagePath = "../escape.bin"
	if _, f := builder.Plan(contract.PlanRequest{VaultID: "v", Origin: "user", Actions: []contract.MutationAction{traversal}}); f == nil || f.Code != contract.CodeInvalidInput {
		t.Fatalf("traversal = %+v", f)
	}

	wrongHash := importAction(stageID, "sha256:"+string(make([]byte, 64)), "x.bin", nil)
	if _, f := builder.Plan(contract.PlanRequest{VaultID: "v", Origin: "user", Actions: []contract.MutationAction{wrongHash}}); f == nil || f.Code != contract.CodeRevisionMismatch {
		t.Fatalf("hash mismatch = %+v", f)
	}

	if err := os.MkdirAll(filepath.Join(root, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "skills", "a.bin"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, f := builder.Plan(contract.PlanRequest{VaultID: "v", Origin: "user", Actions: []contract.MutationAction{importAction(stageID, hash, "skills/a.bin", nil)}}); f == nil || f.Code != contract.CodeInvalidInput {
		t.Fatalf("replace without revision = %+v", f)
	}
	expected := int64(5)
	replacePlan, f := builder.Plan(contract.PlanRequest{VaultID: "v", Origin: "user", Actions: []contract.MutationAction{importAction(stageID, hash, "skills/a.bin", &expected)}})
	if f != nil || replacePlan.Risk != "medium" {
		t.Fatalf("replace plan = %+v %+v", replacePlan, f)
	}

	builder.StagingRoot = ""
	if _, f := builder.Plan(contract.PlanRequest{VaultID: "v", Origin: "user", Actions: []contract.MutationAction{importAction(stageID, hash, "x.bin", nil)}}); f == nil || f.Code != contract.CodeInvalidInput {
		t.Fatalf("disabled staging = %+v", f)
	}
}

func TestPlanImportFileAllowsDeepDotPaths(t *testing.T) {
	staging := t.TempDir()
	stageID := "s-0123456789ab"
	rel := "skills/pdf/references/deep/.hidden/a.bin"
	file := filepath.Join(staging, stageID, "root", filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte{0x00}, 0o644); err != nil {
		t.Fatal(err)
	}
	hash, err := stagedFileHash(file)
	if err != nil {
		t.Fatal(err)
	}
	builder := Builder{Root: t.TempDir(), StagingRoot: staging}
	action := contract.MutationAction{
		ID: "imp", Kind: contract.MutImportFile, Target: "x.bin",
		StageID: stageID, StagePath: rel, StageHash: hash, Reason: "更新技能",
	}
	plan, f := builder.Plan(contract.PlanRequest{VaultID: "v", Origin: "user", Actions: []contract.MutationAction{action}})
	if f != nil || len(plan.Actions) != 1 {
		t.Fatalf("deep stage path = %+v %+v", plan, f)
	}
}

func TestExecuteImportFileCreatesAndReplaces(t *testing.T) {
	root := t.TempDir()
	staging, stageID, hash := importStage(t)
	executor, _ := fixedExecutor(t, root)
	executor.StageRoot = staging
	revision := int64(7)
	executor.StatRevision = func(rel string) (int64, *contract.Failure) { return revision, nil }

	expected := revision
	action := importAction(stageID, hash, "skills/文档/pdf/img/a.bin", nil)
	report := executor.Execute(contract.MutationPlan{VaultID: "v", Origin: "user", ID: "p1", Actions: []contract.MutationAction{action}})
	if report.Status != "completed" {
		t.Fatalf("create report = %+v", report)
	}
	written, err := os.ReadFile(filepath.Join(root, "skills", "文档", "pdf", "img", "a.bin"))
	if err != nil || len(written) != 4 || written[0] != 0x00 {
		t.Fatalf("written = %v err=%v", written, err)
	}

	replace := importAction(stageID, hash, "skills/文档/pdf/img/a.bin", &expected)
	report = executor.Execute(contract.MutationPlan{VaultID: "v", Origin: "user", ID: "p2", Actions: []contract.MutationAction{replace}})
	if report.Status != "completed" || report.TransactionID == "" {
		t.Fatalf("replace report = %+v", report)
	}

	revision = 8
	stale := importAction(stageID, hash, "skills/文档/pdf/img/a.bin", &expected)
	report = executor.Execute(contract.MutationPlan{VaultID: "v", Origin: "user", ID: "p3", Actions: []contract.MutationAction{stale}})
	if report.Status != "failed" || len(report.Failed) != 1 || report.Failed[0].ErrorCode != contract.CodeRevisionMismatch {
		t.Fatalf("stale report = %+v", report)
	}
}

func TestExecuteImportFileRejectsChangedStage(t *testing.T) {
	root := t.TempDir()
	staging, stageID, hash := importStage(t)
	file := filepath.Join(staging, stageID, "root", "docs", "a.bin")
	if err := os.WriteFile(file, []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	executor, _ := fixedExecutor(t, root)
	executor.StageRoot = staging
	report := executor.Execute(contract.MutationPlan{VaultID: "v", Origin: "user", ID: "p1", Actions: []contract.MutationAction{importAction(stageID, hash, "x.bin", nil)}})
	if report.Status != "failed" || len(report.Failed) != 1 || report.Failed[0].ErrorCode != contract.CodeRevisionMismatch {
		t.Fatalf("changed stage = %+v", report)
	}
	if _, err := os.Stat(filepath.Join(root, "x.bin")); !os.IsNotExist(err) {
		t.Fatalf("failed import wrote the target")
	}
}

func TestExecuteImportFileAgentDenied(t *testing.T) {
	root := t.TempDir()
	staging, stageID, hash := importStage(t)
	executor, _ := fixedExecutor(t, root)
	executor.StageRoot = staging
	report := executor.Execute(contract.MutationPlan{VaultID: "v", Origin: "curator", ID: "p1", Actions: []contract.MutationAction{importAction(stageID, hash, "x.bin", nil)}})
	if report.Status != "failed" || report.Failure == nil || report.Failure.Code != contract.CodePermissionDenied {
		t.Fatalf("agent execute = %+v", report)
	}
}
