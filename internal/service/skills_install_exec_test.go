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

func writeInstallFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func stagedForInstall(t *testing.T, svc *SkillService, vaultID string) contract.SkillStage {
	t.Helper()
	archive := tarGzFixture(t, map[string]string{
		"repo/skills/pdf/SKILL.md":       "---\nname: pdf\ndescription: PDF\n---\n",
		"repo/skills/pdf/reference/a.md": "a",
	})
	server := fakeGithubServer(t, archive)
	svc.githubClient = func(string) *skills.GithubClient {
		client := skills.NewGithubClientForTest()
		client.APIBase = server.URL
		client.CodeloadBase = server.URL
		client.SetHTTPForTest(server.Client().Transport)
		return client
	}
	result := svc.ReadGithub(context.Background(), vaultID, "acme/skills")
	if result.Stage == nil {
		t.Fatalf("stage = %+v", result)
	}
	return *result.Stage
}

func TestSkillServiceInstallIntoVault(t *testing.T) {
	svc, vaultID := skillServiceWithHome(t, t.TempDir())
	stage := stagedForInstall(t, svc, vaultID)
	root, _ := svc.app.Vaults.Root(vaultID)

	preview := svc.InstallPreview(context.Background(), vaultID, stage.StageID, []string{"skills/pdf"}, contract.SkillInstallTarget{Kind: "kb", Category: "文档"})
	if preview.Failure != nil || len(preview.Items) != 1 || preview.Items[0].Action != "create" {
		t.Fatalf("preview = %+v", preview)
	}
	result := svc.Install(context.Background(), vaultID, stage.StageID, []string{"skills/pdf"}, contract.SkillInstallTarget{Kind: "kb", Category: "文档"})
	if result.Failure != nil || len(result.Items) != 1 || result.Items[0].Result != "copied" {
		t.Fatalf("install = %+v", result)
	}
	installed := filepath.Join(root, "skills", "文档", "pdf", "SKILL.md")
	if _, err := os.Stat(installed); err != nil {
		t.Fatalf("installed file missing: %v", err)
	}
	records := skills.LoadKbSources(root)
	if len(records.Records) != 1 {
		t.Fatalf("source records = %+v", records)
	}
	record := records.Records[0]
	if record.SkillPath != "skills/文档/pdf" || record.Owner != "acme" || record.Commit != installTestSHA ||
		record.Path != "skills/pdf" || record.PathCommit != installTestSHA || record.BaseHash == "" {
		t.Fatalf("source record = %+v", record)
	}
	if _, err := os.Stat(filepath.Join(svc.app.DataDir, "skills", "tmp", stage.StageID)); !os.IsNotExist(err) {
		t.Fatalf("stage should be discarded after a successful install")
	}
	logged := svc.Log(context.Background(), 10)
	found := false
	for _, entry := range logged.Records {
		if entry.Op == "install" && entry.Skill == "pdf" {
			found = true
		}
	}
	if !found {
		t.Fatalf("install not logged: %+v", logged.Records)
	}
}

func TestSkillServiceInstallIntoVaultSkipsExisting(t *testing.T) {
	svc, vaultID := skillServiceWithHome(t, t.TempDir())
	stage := stagedForInstall(t, svc, vaultID)
	root, _ := svc.app.Vaults.Root(vaultID)
	target := filepath.Join(root, "skills", "文档", "pdf")
	writeInstallFile(t, filepath.Join(target, "SKILL.md"), "# existing\n")

	preview := svc.InstallPreview(context.Background(), vaultID, stage.StageID, []string{"skills/pdf"}, contract.SkillInstallTarget{Kind: "kb", Category: "文档"})
	if preview.Items[0].Action != "exists" {
		t.Fatalf("preview = %+v", preview)
	}
	result := svc.Install(context.Background(), vaultID, stage.StageID, []string{"skills/pdf"}, contract.SkillInstallTarget{Kind: "kb", Category: "文档"})
	if len(result.Items) != 1 || result.Items[0].Result != "skipped" {
		t.Fatalf("install = %+v", result)
	}
	body, _ := os.ReadFile(filepath.Join(target, "SKILL.md"))
	if string(body) != "# existing\n" {
		t.Fatalf("existing skill was overwritten: %q", body)
	}
	if records := skills.LoadKbSources(root); len(records.Records) != 0 {
		t.Fatalf("skipped install wrote a source record: %+v", records)
	}
	if _, err := os.Stat(filepath.Join(svc.app.DataDir, "skills", "tmp", stage.StageID)); err != nil {
		t.Fatalf("stage should stay for a retry: %v", err)
	}
	_ = svc.DiscardStage(context.Background(), stage.StageID)
}

func TestSkillServiceInstallIntoTool(t *testing.T) {
	home := t.TempDir()
	writeInstallFile(t, filepath.Join(home, ".local", "bin", "claude.cmd"), "cmd")
	svc, vaultID := skillServiceWithHome(t, home)
	stage := stagedForInstall(t, svc, vaultID)

	target := contract.SkillInstallTarget{Kind: "tools", Targets: []contract.SkillDistributeTarget{{TargetID: "claude-code"}}}
	preview := svc.InstallPreview(context.Background(), vaultID, stage.StageID, []string{"skills/pdf"}, target)
	if preview.Failure != nil || len(preview.Items) != 1 || preview.Items[0].Action != "create" {
		t.Fatalf("preview = %+v", preview)
	}
	result := svc.Install(context.Background(), vaultID, stage.StageID, []string{"skills/pdf"}, target)
	if result.Failure != nil || len(result.Items) != 1 || result.Items[0].Result != "copied" {
		t.Fatalf("install = %+v", result)
	}
	installed := filepath.Join(home, ".claude", "skills", "pdf", "SKILL.md")
	if _, err := os.Stat(installed); err != nil {
		t.Fatalf("tool copy missing: %v", err)
	}
	sources := skills.LoadToolSources(filepath.Join(svc.app.DataDir, "skills"))
	if len(sources.Records) != 1 {
		t.Fatalf("tool sources = %+v", sources)
	}
	if sources.Records[0].ToolID != "claude-code" || sources.Records[0].BaseHash == "" {
		t.Fatalf("tool source = %+v", sources.Records[0])
	}
	if _, err := os.Stat(filepath.Join(svc.app.DataDir, "skills", "tmp", stage.StageID)); !os.IsNotExist(err) {
		t.Fatalf("stage should be discarded")
	}
}

func TestSkillServiceInstallOverwritesWithBackup(t *testing.T) {
	home := t.TempDir()
	writeInstallFile(t, filepath.Join(home, ".local", "bin", "claude.cmd"), "cmd")
	existing := filepath.Join(home, ".claude", "skills", "pdf")
	writeInstallFile(t, filepath.Join(existing, "SKILL.md"), "# old\n")
	svc, vaultID := skillServiceWithHome(t, home)
	stage := stagedForInstall(t, svc, vaultID)

	target := contract.SkillInstallTarget{Kind: "tools", Targets: []contract.SkillDistributeTarget{{TargetID: "claude-code", AllowOverwrite: true}}}
	preview := svc.InstallPreview(context.Background(), vaultID, stage.StageID, []string{"skills/pdf"}, target)
	if preview.Items[0].Action != "overwrite" || preview.Items[0].BackupPath == "" {
		t.Fatalf("preview = %+v", preview)
	}
	result := svc.Install(context.Background(), vaultID, stage.StageID, []string{"skills/pdf"}, target)
	if result.Items[0].Result != "copied" || result.Items[0].BackupID == "" {
		t.Fatalf("install = %+v", result)
	}
	body, _ := os.ReadFile(filepath.Join(existing, "SKILL.md"))
	if strings.Contains(string(body), "# old") {
		t.Fatalf("old content survived: %q", body)
	}
	backups := svc.Backups(context.Background())
	if len(backups.Items) != 1 || backups.Items[0].Kind != "overwrite" {
		t.Fatalf("backups = %+v", backups.Items)
	}
}

func TestPickInstallCandidatesRequiresOK(t *testing.T) {
	candidates := []contract.SkillStageCandidate{
		{ID: "skills/a", Name: "a", State: "ok"},
		{ID: "skills/b", Name: "b", State: "exists"},
		{ID: "skills/c", Name: "c", State: "too-large"},
	}
	picked, f := pickInstallCandidates(candidates, []string{"skills/a"})
	if f != nil || len(picked) != 1 || picked[0].ID != "skills/a" {
		t.Fatalf("ok candidate = %+v %+v", picked, f)
	}
	for _, id := range []string{"skills/b", "skills/c"} {
		if _, f := pickInstallCandidates(candidates, []string{id}); f == nil || f.Code != contract.CodeInvalidInput {
			t.Fatalf("%s = %+v", id, f)
		}
	}
}

func TestSkillServiceInstallRejectsBadSelection(t *testing.T) {
	svc, vaultID := skillServiceWithHome(t, t.TempDir())
	stage := stagedForInstall(t, svc, vaultID)
	result := svc.Install(context.Background(), vaultID, stage.StageID, []string{"nope"}, contract.SkillInstallTarget{Kind: "kb", Category: "文档"})
	if result.Failure == nil || result.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("bad id = %+v", result)
	}
	if result := svc.InstallPreview(context.Background(), vaultID, "s-0123456789ab", []string{"skills/pdf"}, contract.SkillInstallTarget{Kind: "kb", Category: "文档"}); result.Failure == nil || result.Failure.Code != contract.CodeNotFound {
		t.Fatalf("missing stage = %+v", result)
	}
	target := contract.SkillInstallTarget{Kind: "tools", Targets: []contract.SkillDistributeTarget{{TargetID: "nope"}}}
	if result := svc.Install(context.Background(), vaultID, stage.StageID, []string{"skills/pdf"}, target); result.Failure == nil || result.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("unknown tool = %+v", result)
	}
	_ = svc.DiscardStage(context.Background(), stage.StageID)
}
