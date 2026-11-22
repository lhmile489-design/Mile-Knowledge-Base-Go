//go:build e2e

package service

import (
	"context"
	"os"

	"github.com/egoist/mygo"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// pickFolderDialog is the folder picker of M98. The e2e build can answer it
// with SKILLS_FAKE_PICK_DIR; production always shows the real dialog
// (skills_dialog_prod.go).
func pickFolderDialog(ctx context.Context, title string) ([]string, error) {
	if fixed := os.Getenv(contract.EnvSkillsFakePickDir); fixed != "" {
		return []string{fixed}, nil
	}
	return mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent:    mygo.CallerWindow(ctx),
		Title:     title,
		Directory: true,
	})
}

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
