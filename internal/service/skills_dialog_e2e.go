//go:build e2e

package service

import (
	"context"
	"os"
	"strings"

	"github.com/egoist/mygo"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// pickFolderDialog is the folder picker of M98. The e2e build can answer it
// with SKILLS_FAKE_PICK_DIR; production always shows the real dialog
// (skills_dialog_prod.go).
func pickFolderDialog(ctx context.Context, title string) ([]string, error) {
	if fixed := os.Getenv(contract.EnvSkillsFakePickDir); fixed != "" {
		// "@<file>" pops the first non-empty line so a script can answer
		// several picks in one run (M6 T09).
		if strings.HasPrefix(fixed, "@") {
			return []string{nextFakePick(strings.TrimPrefix(fixed, "@"))}, nil
		}
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

// nextFakePick removes and returns the first non-empty line of the queue file
// (e2e only, SKILLS_FAKE_PICK_DIR=@<file>).
func nextFakePick(queue string) string {
	body, err := os.ReadFile(queue)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(body), "\n")
	for index, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		rest := strings.Join(append(lines[:index:index], lines[index+1:]...), "\n")
		_ = os.WriteFile(queue, []byte(rest), 0o644)
		return line
	}
	return ""
}
