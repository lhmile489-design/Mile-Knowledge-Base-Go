package mutation

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

func agentBuilder(t *testing.T) (Builder, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "raws"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "raws", "a.md"), []byte("# a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return Builder{Root: root}, root
}

func TestAgentOriginAccepted(t *testing.T) {
	builder, _ := agentBuilder(t)
	plan, f := builder.Plan(contract.PlanRequest{
		VaultID: "v1", Origin: "curator",
		Actions: []contract.MutationAction{{ID: "a1", Kind: contract.MutMove, Source: "raws/a.md", Target: "raws/b.md"}},
	})
	if f != nil {
		t.Fatalf("agent move rejected: %+v", f)
	}
	if plan.Origin != "curator" || len(plan.Actions) != 1 {
		t.Fatalf("plan: %+v", plan)
	}
}

func TestAgentOriginPolicyDenies(t *testing.T) {
	builder, _ := agentBuilder(t)
	_, f := builder.Plan(contract.PlanRequest{
		VaultID: "v1", Origin: "scout",
		Actions: []contract.MutationAction{{ID: "a1", Kind: contract.MutMove, Source: "raws/a.md", Target: ".mile-kb/settings/a.md"}},
	})
	if f == nil || f.Code != contract.CodePermissionDenied {
		t.Fatalf("system target: %+v", f)
	}

	_, f = builder.Plan(contract.PlanRequest{
		VaultID: "v1", Origin: "scout",
		Actions: []contract.MutationAction{{ID: "a1", Kind: contract.MutMove, Source: ".mile-kb/config.json", Target: "raws/x.md"}},
	})
	if f == nil || f.Code != contract.CodePermissionDenied {
		t.Fatalf("system source: %+v", f)
	}

	_, f = builder.Plan(contract.PlanRequest{
		VaultID: "v1", Origin: "nobody",
		Actions: []contract.MutationAction{{ID: "a1", Kind: contract.MutMove, Source: "raws/a.md", Target: "raws/b.md"}},
	})
	if f == nil || f.Code != contract.CodeInvalidInput {
		t.Fatalf("unknown origin: %+v", f)
	}
}

func TestUserOriginStillUnrestrictedByPolicy(t *testing.T) {
	builder, _ := agentBuilder(t)
	// A user may move a file into settings through the normal plan flow; the
	// policy applies to agents only.
	if _, f := builder.Plan(contract.PlanRequest{
		VaultID: "v1", Origin: "user",
		Actions: []contract.MutationAction{{ID: "a1", Kind: contract.MutMove, Source: "raws/a.md", Target: ".mile-kb/settings/a.md"}},
	}); f != nil {
		t.Fatalf("user plan rejected: %+v", f)
	}
}

func TestExecutorRechecksAgentPlans(t *testing.T) {
	builder, root := agentBuilder(t)
	plan, f := builder.Plan(contract.PlanRequest{
		VaultID: "v1", Origin: "curator",
		Actions: []contract.MutationAction{{ID: "a1", Kind: contract.MutMove, Source: "raws/a.md", Target: "raws/b.md"}},
	})
	if f != nil {
		t.Fatal(f)
	}
	// Simulate a tampered plan that skipped the builder.
	plan.Actions[0].Target = ".mile-kb/settings/a.md"
	executor := Executor{Root: root, VaultID: "v1", Manager: TxManager{Root: root}}
	report := executor.Execute(plan)
	if report.Failure == nil || report.Failure.Code != contract.CodePermissionDenied {
		t.Fatalf("executor accepted a tampered agent plan: %+v", report)
	}
	if _, err := os.Stat(filepath.Join(root, "raws", "a.md")); err != nil {
		t.Fatalf("source moved anyway: %v", err)
	}
}
