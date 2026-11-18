package service

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/ai"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

func TestAppConfigPatchAgent(t *testing.T) {
	app, vaultID, root := mutationTestApp(t)
	svc := NewSettingsService(app)
	ctx := context.Background()

	initial := svc.AppConfigGet(ctx, vaultID)
	if initial.Failure != nil || len(initial.Agents) != 4 {
		t.Fatalf("initial: %+v", initial)
	}
	if initial.Agents["scout"].Approval != "auto" || initial.Agents["curator"].Effort != "high" {
		t.Fatalf("D-010 defaults: %+v", initial.Agents)
	}

	steering := "只改这一项"
	patched := svc.AppConfigPatchAgent(ctx, vaultID, "curator", contract.AgentPatch{Steering: &steering})
	if patched.Failure != nil {
		t.Fatal(patched.Failure)
	}
	if patched.Agents["curator"].Steering != steering {
		t.Fatalf("steering: %+v", patched.Agents["curator"])
	}
	if patched.Agents["curator"].Model != "deepseek-v4-flash" || patched.Agents["curator"].Approval != "ask" {
		t.Fatalf("unprovided keys changed: %+v", patched.Agents["curator"])
	}
	body, err := os.ReadFile(ai.AgentsPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"curator"`) || !strings.Contains(string(body), steering) {
		t.Fatalf("file not written:\n%s", body)
	}

	bad := "medium"
	if got := svc.AppConfigPatchAgent(ctx, vaultID, "curator", contract.AgentPatch{Effort: &bad}); got.Failure == nil || got.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("bad effort: %+v", got)
	}
	if got := svc.AppConfigPatchAgent(ctx, vaultID, "nobody", contract.AgentPatch{}); got.Failure == nil {
		t.Fatalf("unknown agent accepted")
	}

	reloaded := svc.AppConfigGet(ctx, vaultID)
	if reloaded.Agents["curator"].Steering != steering {
		t.Fatalf("reload: %+v", reloaded.Agents["curator"])
	}
}

func TestAppConfigResetSteering(t *testing.T) {
	app, vaultID, _ := mutationTestApp(t)
	svc := NewSettingsService(app)
	ctx := context.Background()
	steering := "custom"
	if got := svc.AppConfigPatchAgent(ctx, vaultID, "scout", contract.AgentPatch{Steering: &steering}); got.Failure != nil {
		t.Fatal(got.Failure)
	}
	reset := svc.AppConfigResetSteering(ctx, vaultID, "scout")
	if reset.Failure != nil {
		t.Fatal(reset.Failure)
	}
	if !strings.Contains(reset.Agents["scout"].Steering, "来源优先") {
		t.Fatalf("reset: %q", reset.Agents["scout"].Steering)
	}
	again := svc.AppConfigGet(ctx, vaultID)
	if !strings.Contains(again.Agents["scout"].Steering, "来源优先") {
		t.Fatalf("reset not persisted: %q", again.Agents["scout"].Steering)
	}
}

func TestSkillCatalogEmptyAndUnknownVault(t *testing.T) {
	app, vaultID, _ := mutationTestApp(t)
	svc := NewSettingsService(app)
	ctx := context.Background()

	empty := svc.SkillCatalog(ctx, vaultID)
	if empty.Failure != nil || len(empty.Skills) != 0 {
		t.Fatalf("no index yet: %+v", empty)
	}
	unknown := svc.SkillCatalog(ctx, "nope")
	if unknown.Failure == nil || unknown.Failure.Code != contract.CodeVaultUnavailable {
		t.Fatalf("unknown vault: %+v", unknown)
	}
}
