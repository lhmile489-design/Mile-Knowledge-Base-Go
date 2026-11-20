package skills

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// M4 T06: link-only removal, backup-before-delete, restore conflicts and
// backup upkeep (PRD FR-SKM-UNINST, AC-SKM-7/8).

func TestUninstallLinkKeepsTarget(t *testing.T) {
	root := t.TempDir()
	kb := filepath.Join(root, "vault", "skills", "前端", "vue")
	writeFile(t, filepath.Join(kb, "SKILL.md"), "# vue\n")
	link := filepath.Join(root, "tool", "vue")
	mustJunction(t, link, kb)
	before := mustHash(t, kb)

	preview, err := PreviewUninstall(&BackupStore{Dir: filepath.Join(root, "backups")}, link)
	if err != nil || preview.Kind != "link" {
		t.Fatalf("preview = %+v %v", preview, err)
	}
	if _, err := ExecuteUninstall(&BackupStore{Dir: filepath.Join(root, "backups")}, "t", link, time.Unix(0, 0)); err != nil {
		t.Fatal(err)
	}
	if kind, _, _ := Classify(link); kind != KindMissing {
		t.Fatalf("link still present: %q", kind)
	}
	if after := mustHash(t, kb); after != before {
		t.Fatalf("target changed: %s -> %s", before, after)
	}
}

func TestUninstallCopyMovesToBackupAndRestores(t *testing.T) {
	root := t.TempDir()
	store := &BackupStore{Dir: filepath.Join(root, "backups")}
	copyDir := filepath.Join(root, "tool", "vue")
	writeFile(t, filepath.Join(copyDir, "SKILL.md"), "# copied\n")

	preview, err := PreviewUninstall(store, copyDir)
	if err != nil || preview.Kind != "independent" {
		t.Fatalf("preview = %+v %v", preview, err)
	}
	backupID, err := ExecuteUninstall(store, "t", copyDir, time.Unix(0, 0))
	if err != nil || backupID == "" {
		t.Fatalf("uninstall: %q %v", backupID, err)
	}
	if _, err := os.Stat(copyDir); !os.IsNotExist(err) {
		t.Fatal("original still exists")
	}
	if body, err := os.ReadFile(filepath.Join(store.Dir, backupID, "files", "SKILL.md")); err != nil || string(body) != "# copied\n" {
		t.Fatalf("backup content: %q %v", body, err)
	}
	if list := ListBackups(store); len(list) != 1 || list[0].ID != backupID {
		t.Fatalf("list = %+v", list)
	}

	restored, err := RestoreBackup(store, backupID)
	if err != nil || restored != copyDir {
		t.Fatalf("restore = %q %v", restored, err)
	}
	if body, err := os.ReadFile(filepath.Join(copyDir, "SKILL.md")); err != nil || string(body) != "# copied\n" {
		t.Fatalf("restored content: %q %v", body, err)
	}
	if list := ListBackups(store); len(list) != 0 {
		t.Fatalf("backup not removed after restore: %+v", list)
	}
}

func TestRestoreConflictAndMissing(t *testing.T) {
	root := t.TempDir()
	store := &BackupStore{Dir: filepath.Join(root, "backups")}
	copyDir := filepath.Join(root, "tool", "vue")
	writeFile(t, filepath.Join(copyDir, "SKILL.md"), "# copied\n")
	backupID, err := ExecuteUninstall(store, "t", copyDir, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(copyDir, "SKILL.md"), "# occupied\n")
	if _, err := RestoreBackup(store, backupID); !errors.Is(err, ErrRestoreConflict) {
		t.Fatalf("restore = %v, want conflict", err)
	}
	if body, _ := os.ReadFile(filepath.Join(copyDir, "SKILL.md")); string(body) != "# occupied\n" {
		t.Fatal("conflicting content was overwritten")
	}
	if _, err := RestoreBackup(store, "b-000000000000"); !errors.Is(err, ErrBackupMissing) {
		t.Fatalf("unknown backup = %v", err)
	}
	if _, err := RestoreBackup(store, "not-an-id"); !errors.Is(err, ErrBackupMissing) {
		t.Fatalf("bad id = %v", err)
	}
}

func TestCleanBackupsMakesRestoreImpossible(t *testing.T) {
	root := t.TempDir()
	store := &BackupStore{Dir: filepath.Join(root, "backups")}
	copyDir := filepath.Join(root, "tool", "vue")
	writeFile(t, filepath.Join(copyDir, "SKILL.md"), "# copied\n")
	backupID, err := ExecuteUninstall(store, "t", copyDir, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err := CleanBackups(store, []string{backupID, "b-ffffffffffff"}); err != nil {
		t.Fatal(err)
	}
	if list := ListBackups(store); len(list) != 0 {
		t.Fatalf("list after clean = %+v", list)
	}
	if _, err := RestoreBackup(store, backupID); !errors.Is(err, ErrBackupMissing) {
		t.Fatalf("restore after clean = %v, want missing", err)
	}
}
