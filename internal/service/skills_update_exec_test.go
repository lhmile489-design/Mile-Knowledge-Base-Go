package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/mutation"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/skills"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/vault"
)

// M5 T09: M94 update preview, M95 confirm, M96 tool update (技术设计 16.6).

const updateTestPathSHA = "fedcba9876543210fedcba9876543210fedcba98"

func updateExecServer(t *testing.T, archive []byte, head, pathCommit string, requests *int64) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(requests, 1)
		switch {
		case r.URL.Path == "/repos/acme/skills/commits/main":
			_, _ = w.Write([]byte(head))
		case r.URL.Path == "/repos/acme/skills/commits":
			if r.URL.Query().Get("path") != "skills/pdf" || r.URL.Query().Get("sha") != "main" {
				t.Errorf("path commit query = %s", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode([]map[string]string{{"sha": pathCommit}})
		case r.URL.Path == "/acme/skills/tar.gz/"+head:
			_, _ = w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func updateArchive(t *testing.T) []byte {
	t.Helper()
	return tarGzFixture(t, map[string]string{
		"repo/skills/pdf/SKILL.md":   "# pdf v2\n",
		"repo/skills/pdf/keep.md":    "same",
		"repo/skills/pdf/notes.md":   "notes",
		"repo/skills/pdf/.gitignore": "node_modules\n",
		"repo/skills/pdf/sub/new.md": "new",
		"repo/skills/pdf/img/a.bin":  string([]byte{0x01, 0xff}),
	})
}

func kbUpdateFixture(t *testing.T, svc *SkillService, vaultID string) (string, string) {
	t.Helper()
	root, f := svc.app.Vaults.Root(vaultID)
	if f != nil {
		t.Fatal(f)
	}
	dir := filepath.Join(root, "skills", "文档", "pdf")
	writeInstallFile(t, filepath.Join(dir, "SKILL.md"), "# old\n")
	writeInstallFile(t, filepath.Join(dir, "keep.md"), "same")
	writeInstallFile(t, filepath.Join(dir, "gone.md"), "gone")
	if err := os.MkdirAll(filepath.Join(dir, "img"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "img", "a.bin"), []byte{0x00, 0xff}, 0o644); err != nil {
		t.Fatal(err)
	}
	hash, err := skills.TreeHash(dir)
	if err != nil {
		t.Fatal(err)
	}
	record := skills.KbSource{
		SkillPath: "skills/文档/pdf", Owner: "acme", Repo: "skills", Ref: "main", Path: "skills/pdf",
		Commit: updateTestSHA, PathCommit: updateTestSHA, BaseHash: hash, InstalledAt: "2026-11-21T09:00:00Z",
	}
	if f := skills.SaveKbSource(root, record); f != nil {
		t.Fatal(f)
	}
	return root, dir
}

func executeUpdatePlan(t *testing.T, svc *SkillService, root, vaultID string, plan contract.MutationPlan) contract.MutationReport {
	t.Helper()
	executor := mutation.Executor{
		Root: root, VaultID: vaultID,
		Manager:   mutation.TxManager{Root: root},
		Trash:     func(abs string) error { return os.RemoveAll(abs) },
		StageRoot: filepath.Join(svc.app.DataDir, "skills", "tmp"),
		StatRevision: func(rel string) (int64, *contract.Failure) {
			revision, _, f := vault.StatRevision(root, rel)
			if f != nil {
				return 0, f
			}
			return revision, nil
		},
		WriteText: func(rel string, expected int64, text string) (string, *contract.Failure) {
			outcome, f := vault.Write(root, rel, expected, text)
			if f != nil {
				return "", f
			}
			if outcome.Status == "conflict" {
				return "", contract.Fail(contract.CodeRevisionMismatch, "目标已被外部修改")
			}
			return outcome.Hash, nil
		},
		Now: svc.now,
	}
	return executor.Execute(plan)
}

func actionKinds(plan contract.MutationPlan) map[string]int {
	kinds := map[string]int{}
	for _, action := range plan.Actions {
		kinds[action.Kind]++
	}
	return kinds
}

func TestSkillServiceKbUpdatePreviewAndConfirm(t *testing.T) {
	svc, vaultID := updateTestService(t)
	var requests int64
	server := updateExecServer(t, updateArchive(t), updateTestNewSHA, updateTestPathSHA, &requests)
	injectUpdateClient(svc, server)
	root, dir := kbUpdateFixture(t, svc, vaultID)

	preview := svc.UpdatePreview(context.Background(), vaultID, contract.SkillUpdateTarget{Kind: "kb", SkillPath: "skills/文档/pdf"})
	if preview.Failure != nil || preview.StageID == "" || preview.Plan == nil {
		t.Fatalf("preview = %+v", preview)
	}
	if preview.From != updateTestSHA || preview.To != updateTestPathSHA || len(preview.Files) != 6 {
		t.Fatalf("preview meta = from %q to %q files %d", preview.From, preview.To, len(preview.Files))
	}
	changes := map[string]string{}
	for _, file := range preview.Files {
		changes[file.Path] = file.Change
	}
	if changes["SKILL.md"] != "modified" || changes["img/a.bin"] != "modified" ||
		changes["gone.md"] != "removed" || changes["notes.md"] != "added" ||
		changes[".gitignore"] != "added" || changes["sub/new.md"] != "added" {
		t.Fatalf("changes = %+v", changes)
	}
	kinds := actionKinds(*preview.Plan)
	if kinds[contract.MutEdit] != 1 || kinds[contract.MutTrash] != 1 || kinds[contract.MutCreate] != 1 || kinds[contract.MutImportFile] != 3 {
		t.Fatalf("actions = %+v", kinds)
	}
	for _, action := range preview.Plan.Actions {
		switch action.Target {
		case "skills/文档/pdf/notes.md":
			if action.Kind != contract.MutCreate || action.Text == nil || *action.Text != "notes" {
				t.Fatalf("notes action = %+v", action)
			}
		case "skills/文档/pdf/sub/new.md":
			if action.Kind != contract.MutImportFile || action.StagePath != "skills/pdf/sub/new.md" {
				t.Fatalf("sub action = %+v", action)
			}
		case "skills/文档/pdf/.gitignore":
			if action.Kind != contract.MutImportFile || action.StagePath != "skills/pdf/.gitignore" {
				t.Fatalf("dotfile action = %+v", action)
			}
		case "skills/文档/pdf/img/a.bin":
			if action.Kind != contract.MutImportFile || action.ExpectedTargetRevision == nil {
				t.Fatalf("binary action = %+v", action)
			}
		}
	}

	report := executeUpdatePlan(t, svc, root, vaultID, *preview.Plan)
	if report.Status != "completed" || len(report.Failed) != 0 {
		t.Fatalf("execute = %+v", report)
	}
	if body, _ := os.ReadFile(filepath.Join(dir, "SKILL.md")); string(body) != "# pdf v2\n" {
		t.Fatalf("SKILL.md = %q", body)
	}
	if body, _ := os.ReadFile(filepath.Join(dir, "img", "a.bin")); len(body) != 2 || body[0] != 0x01 || body[1] != 0xff {
		t.Fatalf("img = %v", body)
	}
	if _, err := os.Stat(filepath.Join(dir, ".gitignore")); err != nil {
		t.Fatalf(".gitignore missing: %v", err)
	}
	if body, _ := os.ReadFile(filepath.Join(dir, "sub", "new.md")); string(body) != "new" {
		t.Fatalf("sub/new.md = %q", body)
	}
	if _, err := os.Stat(filepath.Join(dir, "gone.md")); !os.IsNotExist(err) {
		t.Fatalf("gone.md still there")
	}

	stage, f := svc.stageStore().Get(preview.StageID)
	if f != nil {
		t.Fatalf("stage = %+v", f)
	}
	stagedHash, err := skills.TreeHash(filepath.Join(stage.Root(), filepath.FromSlash("skills/pdf")))
	if err != nil || stagedHash == "" {
		t.Fatalf("staged hash = %q %v", stagedHash, err)
	}
	if localHash, _ := skills.TreeHash(dir); localHash != stagedHash {
		t.Fatalf("local %q != staged %q", localHash, stagedHash)
	}

	confirm := svc.ConfirmKbUpdate(context.Background(), vaultID, preview.StageID)
	if confirm.Failure != nil || !confirm.Updated {
		t.Fatalf("confirm = %+v", confirm)
	}
	records := skills.LoadKbSources(root)
	if len(records.Records) != 1 || records.Records[0].Commit != updateTestNewSHA ||
		records.Records[0].PathCommit != updateTestPathSHA || records.Records[0].BaseHash != stagedHash {
		t.Fatalf("record = %+v", records.Records)
	}
	if _, f := svc.stageStore().Get(preview.StageID); f == nil {
		t.Fatalf("stage should be discarded after confirm")
	}
}

func TestSkillServiceKbUpdateConflictAndMismatch(t *testing.T) {
	svc, vaultID := updateTestService(t)
	var requests int64
	server := updateExecServer(t, updateArchive(t), updateTestNewSHA, updateTestPathSHA, &requests)
	injectUpdateClient(svc, server)
	_, dir := kbUpdateFixture(t, svc, vaultID)

	// A local edit blocks the preview before any network request.
	writeInstallFile(t, filepath.Join(dir, "SKILL.md"), "# local edit\n")
	preview := svc.UpdatePreview(context.Background(), vaultID, contract.SkillUpdateTarget{Kind: "kb", SkillPath: "skills/文档/pdf"})
	if preview.Failure == nil || preview.Failure.Code != contract.CodeSkillConflict {
		t.Fatalf("local edit = %+v", preview)
	}
	if atomic.LoadInt64(&requests) != 0 {
		t.Fatalf("conflict still asked GitHub")
	}

	// Restore the baseline, preview again, and confirm without executing: the
	// content still differs from the staged version.
	writeInstallFile(t, filepath.Join(dir, "SKILL.md"), "# old\n")
	preview = svc.UpdatePreview(context.Background(), vaultID, contract.SkillUpdateTarget{Kind: "kb", SkillPath: "skills/文档/pdf"})
	if preview.Failure != nil || preview.Plan == nil {
		t.Fatalf("preview = %+v", preview)
	}
	confirm := svc.ConfirmKbUpdate(context.Background(), vaultID, preview.StageID)
	if confirm.Updated || confirm.Failure == nil || confirm.Failure.Code != contract.CodeRevisionMismatch {
		t.Fatalf("mismatch = %+v", confirm)
	}
	if records := skills.LoadKbSources(vaultRootForTest(t, svc, vaultID)); records.Records[0].PathCommit != updateTestSHA {
		t.Fatalf("record moved without an update: %+v", records.Records[0])
	}
}

func TestSkillServiceUpdatePreviewUpToDate(t *testing.T) {
	svc, vaultID := updateTestService(t)
	var requests int64
	server := updateExecServer(t, updateArchive(t), updateTestNewSHA, updateTestSHA, &requests)
	injectUpdateClient(svc, server)
	kbUpdateFixture(t, svc, vaultID)

	preview := svc.UpdatePreview(context.Background(), vaultID, contract.SkillUpdateTarget{Kind: "kb", SkillPath: "skills/文档/pdf"})
	if preview.Failure != nil || preview.StageID != "" || preview.Plan != nil || len(preview.Files) != 0 {
		t.Fatalf("up to date = %+v", preview)
	}
}

func TestSkillServiceToolUpdate(t *testing.T) {
	home := t.TempDir()
	writeInstallFile(t, filepath.Join(home, ".local", "bin", "claude.cmd"), "cmd")
	svc, _ := skillServiceWithHome(t, home)
	var requests int64
	server := updateExecServer(t, updateArchive(t), updateTestNewSHA, updateTestPathSHA, &requests)
	injectUpdateClient(svc, server)

	dir := filepath.Join(home, ".claude", "skills", "pdf")
	writeInstallFile(t, filepath.Join(dir, "SKILL.md"), "# old\n")
	writeInstallFile(t, filepath.Join(dir, "keep.md"), "same")
	writeInstallFile(t, filepath.Join(dir, "gone.md"), "gone")
	if err := os.MkdirAll(filepath.Join(dir, "img"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "img", "a.bin"), []byte{0x00, 0xff}, 0o644); err != nil {
		t.Fatal(err)
	}
	hash, err := skills.TreeHash(dir)
	if err != nil {
		t.Fatal(err)
	}
	skillsDir := filepath.Join(svc.app.DataDir, "skills")
	record := skills.ToolSource{
		ToolID: "claude-code", TargetPath: dir, Owner: "acme", Repo: "skills", Ref: "main", Path: "skills/pdf",
		Commit: updateTestSHA, PathCommit: updateTestSHA, BaseHash: hash, InstalledAt: "2026-11-21T09:00:00Z",
	}
	if f := skills.SaveToolSource(skillsDir, record); f != nil {
		t.Fatal(f)
	}

	target := contract.SkillUpdateTarget{Kind: "tool", ToolID: "claude-code", Path: "install:pdf"}
	preview := svc.UpdatePreview(context.Background(), "", target)
	if preview.Failure != nil || preview.StageID == "" || preview.Plan != nil || preview.BackupPath == "" || len(preview.Files) != 6 {
		t.Fatalf("tool preview = %+v", preview)
	}
	result := svc.UpdateTool(context.Background(), preview.StageID)
	if result.Failure != nil || len(result.Items) != 1 || result.Items[0].Result != "copied" || result.Items[0].BackupID == "" {
		t.Fatalf("tool update = %+v", result)
	}
	if body, _ := os.ReadFile(filepath.Join(dir, "SKILL.md")); string(body) != "# pdf v2\n" {
		t.Fatalf("SKILL.md = %q", body)
	}
	if _, err := os.Stat(filepath.Join(dir, "gone.md")); !os.IsNotExist(err) {
		t.Fatalf("gone.md survived the replacement")
	}
	backups := svc.Backups(context.Background())
	found := false
	for _, backup := range backups.Items {
		if backup.ID == result.Items[0].BackupID && backup.Kind == "update" {
			found = true
		}
	}
	if !found {
		t.Fatalf("update backup missing: %+v", backups.Items)
	}
	sources := skills.LoadToolSources(skillsDir)
	if len(sources.Records) != 1 || sources.Records[0].Commit != updateTestNewSHA ||
		sources.Records[0].PathCommit != updateTestPathSHA || sources.Records[0].BaseHash == hash {
		t.Fatalf("tool source = %+v", sources.Records)
	}
	if _, f := svc.stageStore().Get(preview.StageID); f == nil {
		t.Fatalf("stage should be discarded after update")
	}
}

func TestSkillServiceToolUpdateRefusesLocalChanges(t *testing.T) {
	home := t.TempDir()
	writeInstallFile(t, filepath.Join(home, ".local", "bin", "claude.cmd"), "cmd")
	svc, _ := skillServiceWithHome(t, home)
	var requests int64
	server := updateExecServer(t, updateArchive(t), updateTestNewSHA, updateTestPathSHA, &requests)
	injectUpdateClient(svc, server)

	dir := filepath.Join(home, ".claude", "skills", "pdf")
	writeInstallFile(t, filepath.Join(dir, "SKILL.md"), "# old\n")
	hash, err := skills.TreeHash(dir)
	if err != nil {
		t.Fatal(err)
	}
	skillsDir := filepath.Join(svc.app.DataDir, "skills")
	record := skills.ToolSource{
		ToolID: "claude-code", TargetPath: dir, Owner: "acme", Repo: "skills", Ref: "main", Path: "skills/pdf",
		Commit: updateTestSHA, PathCommit: updateTestSHA, BaseHash: hash, InstalledAt: "2026-11-21T09:00:00Z",
	}
	if f := skills.SaveToolSource(skillsDir, record); f != nil {
		t.Fatal(f)
	}
	target := contract.SkillUpdateTarget{Kind: "tool", ToolID: "claude-code", Path: "install:pdf"}
	preview := svc.UpdatePreview(context.Background(), "", target)
	if preview.Failure != nil || preview.StageID == "" {
		t.Fatalf("preview = %+v", preview)
	}
	writeInstallFile(t, filepath.Join(dir, "SKILL.md"), "# local edit\n")
	result := svc.UpdateTool(context.Background(), preview.StageID)
	if result.Failure == nil || result.Failure.Code != contract.CodeSkillConflict {
		t.Fatalf("local edit = %+v", result)
	}
	if body, _ := os.ReadFile(filepath.Join(dir, "SKILL.md")); string(body) != "# local edit\n" {
		t.Fatalf("local content changed: %q", body)
	}
}

func vaultRootForTest(t *testing.T, svc *SkillService, vaultID string) string {
	t.Helper()
	root, f := svc.app.Vaults.Root(vaultID)
	if f != nil {
		t.Fatal(f)
	}
	return root
}
