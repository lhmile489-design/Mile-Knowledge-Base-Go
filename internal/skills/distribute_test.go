package skills

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// M4 T05: junction first, copy fallback, conflicts, overwrite backup and the
// duplicate-load hint (PRD FR-SKM-DIST; 技术设计 15.5).

func sourceSkill(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "vault", "skills", "前端", "vue")
	writeFile(t, filepath.Join(dir, "SKILL.md"), "# vue\n")
	writeFile(t, filepath.Join(dir, "refs", "a.md"), "ref\n")
	return dir
}

func TestDistributePreviewCreateAndJunction(t *testing.T) {
	source := sourceSkill(t)
	install := filepath.Join(t.TempDir(), "tool")
	opts := DistributeOptions{
		VaultID: "v1", SkillPath: "skills/前端/vue", SourceDir: source, Mode: "link",
		Targets: []DistributeTarget{{TargetID: "claude-code", ToolID: "claude-code", Name: "vue", InstallDir: install}},
		Now:     func() time.Time { return time.Unix(0, 0) },
	}
	preview := PreviewDistribute(opts)
	if len(preview) != 1 || preview[0].Action != "create" {
		t.Fatalf("preview = %+v", preview)
	}
	items, records := ExecuteDistribute(opts)
	if len(items) != 1 || items[0].Result != "linked" {
		t.Fatalf("items = %+v", items)
	}
	kind, target, err := Classify(filepath.Join(install, "vue"))
	if err != nil || kind != KindJunction || !strings.EqualFold(filepath.Clean(target), filepath.Clean(source)) {
		t.Fatalf("kind=%q target=%q err=%v", kind, target, err)
	}
	if len(records) != 1 || records[0].Mode != "link" || records[0].BaseHash == "" {
		t.Fatalf("records = %+v", records)
	}
	if body, err := os.ReadFile(filepath.Join(install, "vue", "SKILL.md")); err != nil || string(body) != "# vue\n" {
		t.Fatalf("read through link: %q %v", body, err)
	}
}

func TestDistributeFallsBackToCopy(t *testing.T) {
	source := sourceSkill(t)
	install := filepath.Join(t.TempDir(), "tool")
	opts := DistributeOptions{
		VaultID: "v1", SkillPath: "skills/前端/vue", SourceDir: source, Mode: "link",
		Targets: []DistributeTarget{{TargetID: "t", ToolID: "t", Name: "vue", InstallDir: install}},
		CreateJunction: func(link, target string) error {
			return errors.New("simulated junction failure")
		},
	}
	items, records := ExecuteDistribute(opts)
	if items[0].Result != "copied" || !strings.Contains(items[0].Detail, "拷贝") {
		t.Fatalf("items = %+v", items)
	}
	if len(records) != 1 || records[0].Mode != "copy" {
		t.Fatalf("records = %+v", records)
	}
	if body, err := os.ReadFile(filepath.Join(install, "vue", "SKILL.md")); err != nil || string(body) != "# vue\n" {
		t.Fatalf("copied file: %q %v", body, err)
	}
}

func TestDistributeExistingAndOverwriteBackup(t *testing.T) {
	source := sourceSkill(t)
	install := filepath.Join(t.TempDir(), "tool")
	writeFile(t, filepath.Join(install, "vue", "SKILL.md"), "# old\n")
	backups := &BackupStore{Dir: filepath.Join(t.TempDir(), "backups")}
	opts := DistributeOptions{
		VaultID: "v1", SkillPath: "skills/前端/vue", SourceDir: source, Mode: "copy",
		Targets: []DistributeTarget{{TargetID: "t", ToolID: "t", Name: "vue", InstallDir: install}},
		Backups: backups,
		Now:     func() time.Time { return time.Unix(0, 0) },
	}
	if preview := PreviewDistribute(opts); preview[0].Action != "exists" {
		t.Fatalf("preview without overwrite = %+v", preview)
	}
	if items, _ := ExecuteDistribute(opts); items[0].Result != "skipped" {
		t.Fatalf("execute without overwrite = %+v", items)
	}

	opts.Targets[0].AllowOverwrite = true
	preview := PreviewDistribute(opts)
	if preview[0].Action != "overwrite" || preview[0].BackupPath == "" {
		t.Fatalf("preview overwrite = %+v", preview)
	}
	items, records := ExecuteDistribute(opts)
	if items[0].Result != "copied" || items[0].BackupID == "" {
		t.Fatalf("overwrite items = %+v", items)
	}
	if body, err := os.ReadFile(filepath.Join(install, "vue", "SKILL.md")); err != nil || string(body) != "# vue\n" {
		t.Fatalf("new content: %q %v", body, err)
	}
	if body, err := os.ReadFile(filepath.Join(backups.Dir, items[0].BackupID, "files", "SKILL.md")); err != nil || string(body) != "# old\n" {
		t.Fatalf("backup content: %q %v", body, err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %+v", records)
	}
}

func TestDuplicateLoadHint(t *testing.T) {
	source := sourceSkill(t)
	opts := DistributeOptions{
		VaultID: "v1", SkillPath: "skills/前端/vue", SourceDir: source, Mode: "link",
		SharedTargeted: true,
		Targets: []DistributeTarget{
			{TargetID: "agents", ToolID: "agents", Name: "vue", InstallDir: filepath.Join(t.TempDir(), "agents")},
			{TargetID: "cursor", ToolID: "cursor", Name: "vue", InstallDir: filepath.Join(t.TempDir(), "cursor"), ReadsShared: true},
			{TargetID: "claude-code", ToolID: "claude-code", Name: "vue", InstallDir: filepath.Join(t.TempDir(), "claude")},
		},
	}
	preview := PreviewDistribute(opts)
	if preview[1].Detail != "可能重复加载" {
		t.Fatalf("reader target = %+v", preview[1])
	}
	if preview[2].Detail != "" {
		t.Fatalf("non-reader target = %+v", preview[2])
	}
}

func TestConflictBlocksTarget(t *testing.T) {
	source := sourceSkill(t)
	opts := DistributeOptions{
		VaultID: "v1", SkillPath: "skills/前端/vue", SourceDir: source, Mode: "link",
		Targets: []DistributeTarget{{TargetID: "t", ToolID: "t", Name: "vue", InstallDir: t.TempDir(), Conflict: "同名技能已在 skills/后端/vue"}},
	}
	if preview := PreviewDistribute(opts); preview[0].Action != "conflict" {
		t.Fatalf("preview = %+v", preview)
	}
	if items, _ := ExecuteDistribute(opts); items[0].Result != "failed" {
		t.Fatalf("execute = %+v", items)
	}
}

func TestStateStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skills", "state.json")
	state := LoadState(path)
	state.AddDistribution(Distribution{ID: "d1", VaultID: "v1", SkillPath: "skills/前端/vue", ToolID: "claude-code", TargetPath: `C:\t\vue`, Mode: "link", Name: "vue"})
	state.AddDistribution(Distribution{ID: "d2", VaultID: "v1", SkillPath: "skills/前端/vue", ToolID: "claude-code", TargetPath: `C:\t\vue`, Mode: "copy", Name: "vue"})
	if err := SaveState(path, *state); err != nil {
		t.Fatal(err)
	}
	loaded := LoadState(path)
	if len(loaded.Distributions) != 1 || loaded.Distributions[0].Mode != "copy" {
		t.Fatalf("loaded = %+v", loaded.Distributions)
	}
}
