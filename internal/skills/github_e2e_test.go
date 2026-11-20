//go:build e2e

package skills

import (
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// The e2e build points the client at the local fake GitHub (技术设计 16.8).
func TestFakeGithubBaseFromEnv(t *testing.T) {
	t.Setenv(contract.EnvSkillsFakeGithub, "http://127.0.0.1:18765/")
	client := NewGithubClientForTest()
	if client.APIBase != "http://127.0.0.1:18765" || client.CodeloadBase != "http://127.0.0.1:18765" {
		t.Fatalf("fake bases = %q / %q", client.APIBase, client.CodeloadBase)
	}
}
