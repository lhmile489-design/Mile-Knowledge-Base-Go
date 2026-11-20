package skills

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)

// FR-SKM-ADOPT (PRD §15.5; 技术设计 15.5): copy a tool skill into the
// knowledge base, optionally replacing the tool copy with a junction. The
// original directory is backed up before the swap.

// ErrAdoptExists means the knowledge-base target already exists; adoption
// never overwrites (same rule as FR-SKILL).
var ErrAdoptExists = errors.New("knowledge-base target exists")

// AdoptOptions carries one adoption.
type AdoptOptions struct {
	ToolID         string
	SourceDir      string // tool skill directory
	TargetDir      string // vault skills/<category>/<name>
	LinkBack       bool
	Backups        *BackupStore
	Now            func() time.Time
	CreateJunction func(link, target string) error
}

// AdoptResult reports what happened.
type AdoptResult struct {
	Linked   bool
	BackupID string
}

// Adopt copies the skill and (optionally) swaps the tool copy for a link.
// A failed swap restores the original directory; the vault copy stays.
func Adopt(opts AdoptOptions) (AdoptResult, error) {
	createLink := opts.CreateJunction
	if createLink == nil {
		createLink = CreateJunction
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	if pathExists(opts.TargetDir) {
		return AdoptResult{}, ErrAdoptExists
	}
	if err := os.MkdirAll(filepath.Dir(opts.TargetDir), 0o755); err != nil {
		return AdoptResult{}, err
	}
	if err := copyTree(opts.SourceDir, opts.TargetDir); err != nil {
		_ = os.RemoveAll(opts.TargetDir)
		return AdoptResult{}, err
	}
	if !opts.LinkBack {
		return AdoptResult{}, nil
	}
	if opts.Backups == nil {
		return AdoptResult{}, errors.New("backup store unavailable")
	}
	meta, dir, err := opts.Backups.Create(opts.ToolID, opts.SourceDir, "adopt", now())
	if err != nil {
		return AdoptResult{}, err
	}
	if err := createLink(opts.SourceDir, opts.TargetDir); err != nil {
		// Put the original back so the tool keeps working; the vault copy
		// remains, as the PRD requires.
		_ = os.Rename(filepath.Join(dir, "files"), opts.SourceDir)
		_ = os.Remove(filepath.Join(dir, "meta.json"))
		_ = os.Remove(dir)
		return AdoptResult{}, err
	}
	return AdoptResult{Linked: true, BackupID: meta.ID}, nil
}
