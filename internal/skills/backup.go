package skills

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// The backup area (数据库设计 4.6, Q-SKM-3): uninstall and overwrite move the
// original directory here before it disappears. T05 only creates backups;
// listing, restoring and cleaning arrive with T06.

const backupVersion = 1

// BackupMeta is backups/<id>/meta.json.
type BackupMeta struct {
	Version      int    `json:"version"`
	ID           string `json:"id"`
	ToolID       string `json:"toolId"`
	OriginalPath string `json:"originalPath"`
	Kind         string `json:"kind"` // uninstall | overwrite | adopt
	CreatedAt    string `json:"createdAt"`
	Size         int64  `json:"size"`
	Files        int    `json:"files"`
}

// BackupStore owns the skills/backups directory.
type BackupStore struct {
	Dir string
}

func backupID() string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "b-000000000000"
	}
	return "b-" + hex.EncodeToString(buf)
}

// Create moves originalPath into the backup area and writes meta.json. The
// move keeps reparse points as links (R-9 C10) and never follows them.
func (b *BackupStore) Create(toolID, originalPath, kind string, now time.Time) (BackupMeta, string, error) {
	id := backupID()
	dir := filepath.Join(b.Dir, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return BackupMeta{}, "", err
	}
	files, _, err := SafeWalk(originalPath)
	if err != nil {
		return BackupMeta{}, "", err
	}
	var size int64
	for _, rel := range files {
		if info, err := os.Stat(filepath.Join(originalPath, rel)); err == nil {
			size += info.Size()
		}
	}
	meta := BackupMeta{
		Version: backupVersion, ID: id, ToolID: toolID,
		OriginalPath: originalPath, Kind: kind,
		CreatedAt: now.UTC().Format(time.RFC3339),
		Size:      size, Files: len(files),
	}
	target := filepath.Join(dir, "files")
	if err := os.Rename(originalPath, target); err != nil {
		_ = os.RemoveAll(dir)
		return BackupMeta{}, "", err
	}
	body, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return meta, dir, err
	}
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), append(body, '\n'), 0o644); err != nil {
		return meta, dir, err
	}
	return meta, dir, nil
}
