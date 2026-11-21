//go:build e2e

package service

import (
	"context"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

func TestAutoOpenDialogUsesFakeZip(t *testing.T) {
	t.Setenv(contract.EnvSkillsFakeZip, `C:\fixture\demo.zip`)
	paths, err := autoOpenDialog(context.Background(), "选择技能 zip 文件", []string{"zip"})
	if err != nil || len(paths) != 1 || paths[0] != `C:\fixture\demo.zip` {
		t.Fatalf("fake zip = %v %v", paths, err)
	}
}
