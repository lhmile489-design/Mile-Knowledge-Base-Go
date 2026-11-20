//go:build !e2e

package skills

import (
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// Production builds ignore the fake switch entirely (PRD 15.4).
func TestFakeGithubBaseIgnoredInProduction(t *testing.T) {
	t.Setenv(contract.EnvSkillsFakeGithub, "http://127.0.0.1:18765")
	client := NewGithubClientForTest()
	if client.APIBase != defaultGithubAPI || client.CodeloadBase != defaultCodeload {
		t.Fatalf("production bases = %q / %q", client.APIBase, client.CodeloadBase)
	}
}
