//go:build e2e

package skills

import (
	"os"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// fakeGithubBase is the local fake GitHub the acceptance scripts serve (技术设计
// 16.8). Only e2e builds read it; production always talks to GitHub.
func fakeGithubBase() string {
	return os.Getenv(contract.EnvSkillsFakeGithub)
}
