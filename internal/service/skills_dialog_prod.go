//go:build !e2e

package service

import (
	"context"

	"github.com/egoist/mygo"
)

// autoOpenDialog shows the real file dialog; the e2e build can skip it with
// SKILLS_FAKE_ZIP (skills_dialog_e2e.go).
func autoOpenDialog(ctx context.Context, title string, extensions []string) ([]string, error) {
	return mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent:    mygo.CallerWindow(ctx),
		Title:     title,
		Directory: false,
		Filters:   []mygo.FileFilter{{Name: "zip 文件", Extensions: extensions}},
	})
}
