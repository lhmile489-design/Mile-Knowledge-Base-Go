package skills

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// FR-SKM-UNINST (PRD §15.5; 技术设计 15.5): links are deleted as links, real
// directories move into the backup area first, and restores never overwrite.

// ErrBackupMissing means the backup was cleaned or never existed.
var ErrBackupMissing = errors.New("backup missing")

// ErrRestoreConflict means the original path is occupied again.
var ErrRestoreConflict = errors.New("restore target exists")

// UninstallPreview is the read-only answer of M80.
type UninstallPreview struct {
	Kind       string // link | copy | independent
	Target     string // link target for display
	BackupPath string
}

// PreviewUninstall inspects a tool skill before removal.
func PreviewUninstall(store *BackupStore, path string) (UninstallPreview, error) {
	kind, target, err := Classify(path)
	if err != nil {
		return UninstallPreview{}, err
	}
	switch kind {
	case KindMissing:
		return UninstallPreview{}, os.ErrNotExist
	case KindJunction, KindSymlink:
		return UninstallPreview{Kind: "link", Target: target}, nil
	case KindDir:
		preview := UninstallPreview{Kind: "independent"}
		if store != nil {
			preview.BackupPath = filepath.Join(store.Dir, "<new>", "files")
		}
		return preview, nil
	default:
		return UninstallPreview{}, errors.New("not a skill directory")
	}
}

// ExecuteUninstall removes one skill. Links are removed without a backup; a
// real directory is moved into the backup area and its id returned.
func ExecuteUninstall(store *BackupStore, toolID, path string, now time.Time) (backupID string, err error) {
	kind, _, err := Classify(path)
	if err != nil {
		return "", err
	}
	switch kind {
	case KindJunction, KindSymlink:
		return "", RemoveLink(path)
	case KindDir:
		if store == nil {
			return "", errors.New("backup store unavailable")
		}
		meta, _, err := store.Create(toolID, path, "uninstall", now)
		if err != nil {
			return "", err
		}
		return meta.ID, nil
	default:
		return "", os.ErrNotExist
	}
}

// RestoreBackup moves a backup back to its original path and deletes the
// backup entry. A path already in use is a conflict (never overwritten).
func RestoreBackup(store *BackupStore, id string) (string, error) {
	meta, dir, err := loadBackupMeta(store, id)
	if err != nil {
		return "", err
	}
	if pathExists(meta.OriginalPath) {
		return "", ErrRestoreConflict
	}
	if err := os.MkdirAll(filepath.Dir(meta.OriginalPath), 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(filepath.Join(dir, "files"), meta.OriginalPath); err != nil {
		return "", err
	}
	_ = os.Remove(filepath.Join(dir, "meta.json"))
	_ = os.Remove(dir)
	return meta.OriginalPath, nil
}

func loadBackupMeta(store *BackupStore, id string) (BackupMeta, string, error) {
	if store == nil || !validBackupID(id) {
		return BackupMeta{}, "", ErrBackupMissing
	}
	dir := filepath.Join(store.Dir, id)
	body, err := os.ReadFile(filepath.Join(dir, "meta.json"))
	if err != nil {
		return BackupMeta{}, "", ErrBackupMissing
	}
	var meta BackupMeta
	if err := json.Unmarshal(body, &meta); err != nil || meta.Version != backupVersion || meta.ID != id {
		return BackupMeta{}, "", ErrBackupMissing
	}
	return meta, dir, nil
}

func validBackupID(id string) bool {
	if !strings.HasPrefix(id, "b-") || len(id) != 14 {
		return false
	}
	for _, r := range id[2:] {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}

// ListBackups returns every readable backup, newest first.
func ListBackups(store *BackupStore) []BackupMeta {
	if store == nil {
		return nil
	}
	entries, err := os.ReadDir(store.Dir)
	if err != nil {
		return nil
	}
	var out []BackupMeta
	for _, entry := range entries {
		if !entry.IsDir() || !validBackupID(entry.Name()) {
			continue
		}
		meta, _, err := loadBackupMeta(store, entry.Name())
		if err != nil {
			continue
		}
		out = append(out, meta)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

// CleanBackups deletes the selected backups; their log records become
// unrestorable (FR-SKM-UNINST, Q-SKM-3). Unknown ids are ignored.
func CleanBackups(store *BackupStore, ids []string) error {
	if store == nil {
		return errors.New("backup store unavailable")
	}
	for _, id := range ids {
		if !validBackupID(id) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(store.Dir, id)); err != nil {
			return err
		}
	}
	return nil
}
