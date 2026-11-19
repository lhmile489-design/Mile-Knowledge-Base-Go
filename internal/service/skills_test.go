package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/vault"
)

func skillSource(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, rel := range []string{"vue/SKILL.md", "vue/refs/a.md", "go/SKILL.md"} {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("# 技能\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestImportSkillsServiceUsesTokenAndKeepsIt(t *testing.T) {
	app, vaultID, root := mutationTestApp(t)
	source := skillSource(t)
	preview, err := vault.PreviewSkillSource(source)
	if err != nil {
		t.Fatal(err)
	}
	token := app.newPickToken(source)
	app.mu.Lock()
	app.skillPreviews[token] = preview
	app.mu.Unlock()

	svc := NewVaultService(app)
	ctx := context.Background()
	got := svc.ImportSkills(ctx, vaultID, token, "前端", []string{"vue", "vue", "nope"})
	if got.Failure != nil {
		t.Fatalf("import: %+v", got.Failure)
	}
	if len(got.Imported) != 1 || got.Imported[0] != "vue" {
		t.Fatalf("imported = %v", got.Imported)
	}
	if len(got.Skipped) != 1 || got.Skipped[0].Name != "nope" || got.Skipped[0].Reason != "unknown" {
		t.Fatalf("skipped = %+v", got.Skipped)
	}
	if _, err := os.Stat(filepath.Join(root, "skills", "前端", "vue", "refs", "a.md")); err != nil {
		t.Fatalf("copied file missing: %v", err)
	}

	// 令牌不销毁：换个分类还能再导一次。
	again := svc.ImportSkills(ctx, vaultID, token, "另一个分类", []string{"go"})
	if again.Failure != nil || len(again.Imported) != 1 {
		t.Fatalf("retry: %+v %+v", again, again.Failure)
	}
}

func TestImportSkillsServiceRejectsExpiredTokenAndBadInput(t *testing.T) {
	app, vaultID, _ := mutationTestApp(t)
	svc := NewVaultService(app)
	ctx := context.Background()

	expired := svc.ImportSkills(ctx, vaultID, "0123456789abcdef", "前端", []string{"vue"})
	if expired.Failure == nil || expired.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("expired = %+v", expired.Failure)
	}
	if expired.Failure.Message != "技能来源已过期，请重新选择" {
		t.Errorf("message = %q", expired.Failure.Message)
	}
	if empty := svc.ImportSkills(ctx, vaultID, "0123456789abcdef", "前端", nil); empty.Failure == nil {
		t.Fatal("empty names accepted")
	}
	many := make([]string, 201)
	for i := range many {
		many[i] = "skill"
	}
	if tooMany := svc.ImportSkills(ctx, vaultID, "0123456789abcdef", "前端", many); tooMany.Failure == nil || tooMany.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("too many = %+v", tooMany.Failure)
	}
}
