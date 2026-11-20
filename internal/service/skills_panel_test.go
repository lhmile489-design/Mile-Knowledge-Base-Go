package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/skills"
)

// M4 T04: M76/M77 wire-up, redaction and cached-first scanning (API.md §12).

func skillServiceWithHome(t *testing.T, home string) (*SkillService, string) {
	t.Helper()
	app, vaultID, _ := mutationTestApp(t)
	svc := NewSkillService(app)
	svc.detector = func() *skills.Detector {
		return &skills.Detector{
			Home:             home,
			LocalAppData:     filepath.Join(home, "AppData", "Local"),
			UserProgramFiles: filepath.Join(home, "Programs"),
			CLIDirs:          []string{filepath.Join(home, ".local", "bin")},
		}
	}
	return svc, vaultID
}

func TestSkillServiceRegistryRedactsPaths(t *testing.T) {
	home := t.TempDir()
	svc, _ := skillServiceWithHome(t, home)
	result := svc.Registry(context.Background())
	if len(result.Tools) != 25 {
		t.Fatalf("tools = %d, want 25", len(result.Tools))
	}
	var claude contract.SkillTool
	for _, tool := range result.Tools {
		if tool.ID == "claude-code" {
			claude = tool
		}
	}
	if !strings.HasPrefix(claude.InstallDir, "<home>") || !strings.HasSuffix(claude.InstallDir, "skills") {
		t.Fatalf("installDir = %q, want redacted", claude.InstallDir)
	}
}

func TestSkillServiceScanIsCachedFirst(t *testing.T) {
	home := t.TempDir()
	writeSkill := func(rel string) {
		t.Helper()
		full := filepath.Join(home, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("# vue\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeSkill(".local/bin/claude.cmd")
	writeSkill(".claude/skills/vue/SKILL.md")
	svc, vaultID := skillServiceWithHome(t, home)

	first := svc.Scan(context.Background(), vaultID)
	if first.Failure != nil {
		t.Fatalf("scan: %+v", first.Failure)
	}
	if first.Scan.Scanning {
		t.Fatal("first scan should be synchronous")
	}
	var found bool
	for _, group := range first.Scan.Tools {
		if group.ToolID != "claude-code" {
			continue
		}
		for _, skill := range group.Skills {
			if skill.Name == "vue" && skill.Origin == "independent" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("claude-code vu not found: %+v", first.Scan.Tools)
	}

	second := svc.Scan(context.Background(), vaultID)
	if second.Failure != nil {
		t.Fatalf("second scan: %+v", second.Failure)
	}
	if !second.Scan.Scanning {
		t.Fatal("second scan should serve the cache with scanning=true")
	}
	if second.Scan.CachedAt == "" {
		t.Fatal("cached scan has no timestamp")
	}
}

func TestSkillServiceScanRejectsBadVault(t *testing.T) {
	svc, _ := skillServiceWithHome(t, t.TempDir())
	result := svc.Scan(context.Background(), "bad id with spaces")
	if result.Failure == nil || result.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("failure = %+v", result.Failure)
	}
	if result.Failure.Message == "" {
		t.Fatal("failure message is empty")
	}
}

func TestSkillServiceDistributeWritesRecord(t *testing.T) {
	app, vaultID, root := mutationTestApp(t)
	home := t.TempDir()
	writeFile := func(rel, body string) {
		t.Helper()
		full := filepath.Join(home, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(".local/bin/claude.cmd", "@echo off")
	skillDir := filepath.Join(root, "skills", "前端", "vue")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# vue\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := NewSkillService(app)
	svc.detector = func() *skills.Detector {
		return &skills.Detector{
			Home: home, LocalAppData: filepath.Join(home, "AppData", "Local"),
			UserProgramFiles: filepath.Join(home, "Programs"),
			CLIDirs:          []string{filepath.Join(home, ".local", "bin")},
		}
	}
	targets := []contract.SkillDistributeTarget{{TargetID: "claude-code"}}
	preview := svc.DistributePreview(context.Background(), vaultID, "skills/前端/vue", targets, "link")
	if preview.Failure != nil || preview.Items[0].Action != "create" {
		t.Fatalf("preview = %+v", preview)
	}
	result := svc.Distribute(context.Background(), vaultID, "skills/前端/vue", targets, "link")
	if result.Failure != nil || result.Items[0].Result != "linked" {
		t.Fatalf("distribute = %+v", result)
	}
	if kind, _, err := skills.Classify(filepath.Join(home, ".claude", "skills", "vue")); err != nil || kind != skills.KindJunction {
		t.Fatalf("target kind = %q %v", kind, err)
	}
	state := skills.LoadState(filepath.Join(app.DataDir, "skills", "state.json"))
	if len(state.Distributions) != 1 || state.Distributions[0].VaultID != vaultID ||
		state.Distributions[0].SkillPath != "skills/前端/vue" {
		t.Fatalf("state = %+v", state.Distributions)
	}
	again := svc.DistributePreview(context.Background(), vaultID, "skills/前端/vue", targets, "link")
	if again.Items[0].Action != "exists" {
		t.Fatalf("second preview = %+v", again.Items[0])
	}
}

func TestSkillServiceUninstallAndBackups(t *testing.T) {
	app, _, _ := mutationTestApp(t)
	home := t.TempDir()
	writeFile := func(rel, body string) {
		t.Helper()
		full := filepath.Join(home, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(".local/bin/claude.cmd", "@echo off")
	writeFile(".claude/skills/copied/SKILL.md", "# copied\n")
	// Builtin dir of Trae, detected through the uninstall registry.
	writeFile(".trae/builtin_skills/official/SKILL.md", "# builtin\n")
	svc := NewSkillService(app)
	svc.detector = func() *skills.Detector {
		return &skills.Detector{
			Home: home, LocalAppData: filepath.Join(home, "AppData", "Local"),
			UserProgramFiles: filepath.Join(home, "Programs"),
			CLIDirs:          []string{filepath.Join(home, ".local", "bin")},
			Uninstall:        []skills.UninstallEntry{{DisplayName: "TRAE 2.0"}},
		}
	}

	preview := svc.UninstallPreview(context.Background(), "claude-code", "copied")
	if preview.Failure != nil || preview.Preview.Kind != "independent" {
		t.Fatalf("preview = %+v", preview)
	}
	result := svc.Uninstall(context.Background(), "claude-code", "copied")
	if result.Failure != nil || result.Items[0].Result != "removed" || result.Items[0].BackupID == "" {
		t.Fatalf("uninstall = %+v", result)
	}
	backups := svc.Backups(context.Background())
	if backups.Failure != nil || len(backups.Items) != 1 || !backups.Items[0].Restorable {
		t.Fatalf("backups = %+v", backups)
	}
	if !strings.HasPrefix(backups.Items[0].OriginalPath, "<home>") {
		t.Fatalf("originalPath not redacted: %q", backups.Items[0].OriginalPath)
	}
	restore := svc.Restore(context.Background(), backups.Items[0].ID)
	if restore.Failure != nil || !strings.HasPrefix(restore.RestoredTo, "<home>") {
		t.Fatalf("restore = %+v", restore)
	}
	if body, err := os.ReadFile(filepath.Join(home, ".claude", "skills", "copied", "SKILL.md")); err != nil || string(body) != "# copied\n" {
		t.Fatalf("restored: %q %v", body, err)
	}

	builtin := svc.UninstallPreview(context.Background(), "trae", "official")
	if builtin.Failure == nil || builtin.Failure.Code != contract.CodeSkillProtected {
		t.Fatalf("builtin preview = %+v", builtin)
	}
}

func TestSkillServiceAdoptMergesIdenticalTools(t *testing.T) {
	app, vaultID, root := mutationTestApp(t)
	home := t.TempDir()
	writeFile := func(rel, body string) {
		t.Helper()
		full := filepath.Join(home, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(".local/bin/claude.cmd", "@echo off")
	writeFile(".local/bin/cursor.cmd", "@echo off")
	writeFile(".claude/skills/foo/SKILL.md", "# foo\n")
	writeFile(".cursor/skills/foo/SKILL.md", "# foo\n")
	svc := NewSkillService(app)
	svc.detector = func() *skills.Detector {
		return &skills.Detector{
			Home: home, LocalAppData: filepath.Join(home, "AppData", "Local"),
			UserProgramFiles: filepath.Join(home, "Programs"),
			CLIDirs:          []string{filepath.Join(home, ".local", "bin")},
		}
	}
	result := svc.Adopt(context.Background(), vaultID, "claude-code", "foo", "分类", true,
		[]contract.SkillToolRef{{ToolID: "cursor", Path: "foo"}})
	if result.Failure != nil || !result.Linked || len(result.Warnings) != 0 || len(result.BackupIDs) != 2 {
		t.Fatalf("adopt = %+v", result)
	}
	target := filepath.Join(root, "skills", "分类", "foo")
	if body, err := os.ReadFile(filepath.Join(target, "SKILL.md")); err != nil || string(body) != "# foo\n" {
		t.Fatalf("vault copy: %q %v", body, err)
	}
	for _, toolDir := range []string{".claude/skills/foo", ".cursor/skills/foo"} {
		full := filepath.Join(home, filepath.FromSlash(toolDir))
		kind, linkedTo, err := skills.Classify(full)
		if err != nil || kind != skills.KindJunction || linkedTo != target {
			t.Fatalf("%s = %q %q %v", toolDir, kind, linkedTo, err)
		}
	}
	mismatch := svc.Adopt(context.Background(), vaultID, "claude-code", "foo", "分类", true,
		[]contract.SkillToolRef{{ToolID: "cursor", Path: "foo"}})
	if mismatch.Failure == nil || mismatch.Failure.Code != contract.CodeSkillConflict {
		t.Fatalf("second adopt = %+v", mismatch)
	}
	// The tool copies are links now, so a second adopt must not see a
	// mismatch either way; the conflict above is about the vault target.
}

func TestSkillServiceLogsOperations(t *testing.T) {
	app, vaultID, root := mutationTestApp(t)
	home := t.TempDir()
	writeFile := func(rel, body string) {
		t.Helper()
		full := filepath.Join(home, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(".local/bin/claude.cmd", "@echo off")
	writeFile(".claude/skills/copied/SKILL.md", "# copied\n")
	svc := NewSkillService(app)
	svc.detector = func() *skills.Detector {
		return &skills.Detector{
			Home: home, LocalAppData: filepath.Join(home, "AppData", "Local"),
			UserProgramFiles: filepath.Join(home, "Programs"),
			CLIDirs:          []string{filepath.Join(home, ".local", "bin")},
		}
	}
	skillDir := filepath.Join(root, "skills", "前端", "vue")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# vue\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	targets := []contract.SkillDistributeTarget{{TargetID: "claude-code"}}
	if result := svc.Distribute(context.Background(), vaultID, "skills/前端/vue", targets, "link"); result.Failure != nil {
		t.Fatalf("distribute: %+v", result)
	}
	log := svc.Log(context.Background(), 10)
	if log.Failure != nil || len(log.Records) != 1 {
		t.Fatalf("log = %+v", log)
	}
	if log.Records[0].Op != "distribute" || log.Records[0].Skill != "vue" || log.Records[0].Mode != "link" {
		t.Fatalf("record = %+v", log.Records[0])
	}
	if !strings.HasPrefix(log.Records[0].Location, "<home>") {
		t.Fatalf("location not redacted: %q", log.Records[0].Location)
	}

	if result := svc.Uninstall(context.Background(), "claude-code", "copied"); result.Failure != nil || result.Items[0].Result != "removed" {
		t.Fatalf("uninstall: %+v", result)
	}
	log = svc.Log(context.Background(), 10)
	if log.Records[0].Op != "uninstall" || log.Records[0].BackupID == "" || !log.Records[0].Restorable {
		t.Fatalf("uninstall record = %+v", log.Records[0])
	}
	backupID := log.Records[0].BackupID
	if restore := svc.Restore(context.Background(), backupID); restore.Failure != nil {
		t.Fatalf("restore: %+v", restore)
	}
	log = svc.Log(context.Background(), 10)
	if log.Records[0].Op != "restore" || log.Records[0].Skill != "copied" {
		t.Fatalf("restore record = %+v", log.Records[0])
	}
	// The backup was consumed by the restore, so the old record is no longer
	// restorable.
	for _, record := range log.Records {
		if record.Op == "uninstall" && record.Restorable {
			t.Fatalf("uninstall record still restorable: %+v", record)
		}
	}
}
