//go:build e2e

package service

import (
	"context"
	"os"

	"github.com/egoist/mygo"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// autoOpenDialog lets the acceptance and screenshot scripts hand pickZip a
// fixed file through SKILLS_FAKE_ZIP; production builds always show the real
// dialog (skills_dialog_prod.go).
func autoOpenDialog(ctx context.Context, title string, extensions []string) ([]string, error) {
	if fixed := os.Getenv(contract.EnvSkillsFakeZip); fixed != "" {
		return []string{fixed}, nil
	}
	return mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent:    mygo.CallerWindow(ctx),
		Title:     title,
		Directory: false,
		Filters:   []mygo.FileFilter{{Name: "zip 文件", Extensions: extensions}},
	})
}
