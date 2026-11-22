package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/skills"
)

// M7 T04: M103 executes a restore (技术设计 18.8, V-EXEC-1..3).

// restoreServer serves one tarball at a pinned commit and counts downloads.
func restoreServer(t *testing.T, archive []byte) (*httptest.Server, *int64) {
	t.Helper()
	var downloads int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/tar.gz/"+installTestSHA) {
			atomic.AddInt64(&downloads, 1)
			_, _ = w.Write(archive)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)
	return server, &downloads
}

func injectRestoreClient(svc *SkillService, server *httptest.Server) {
	svc.githubClient = func(string) *skills.GithubClient {
		client := skills.NewGithubClientForTest()
		client.APIBase = server.URL
		client.CodeloadBase = server.URL
		client.SetHTTPForTest(server.Client().Transport)
		return client
	}
}

func seedManifest(t *testing.T, root string, items ...skills.ManifestItem) {
	t.Helper()
	manifest := skills.Manifest{Items: items}
	if f := skills.SaveManifest(root, &manifest); f != nil {
		t.Fatal(f)
	}
}

func manifestItemIDs(t *testing.T, preview *contract.SkillRestorePreview) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, item := range preview.Items {
		out[item.Name] = item.ID
	}
	return out
}

func TestRestoreKBLinkAndConflict(t *testing.T) {
	home := t.TempDir()
	writeInstallFile(t, filepath.Join(home, ".local", "bin", "claude.cmd"), "cmd")
	svc, vaultID := skillServiceWithHome(t, home)
	ctx := context.Background()
	root := svc.app.Vaults.List().Vaults[0].Root
	seedVaultSkill(t, root, "skills/前端/demo")
	seedManifest(t, root, skills.ManifestItem{
		Source: "kb", Target: skills.ManifestTarget{Kind: "builtin", ToolID: "claude-code"},
		Name: "demo", SkillPath: "skills/前端/demo", Mode: "link", UpdatedAt: "2026-11-22T09:00:00Z",
	})

	preview := svc.RestorePreview(ctx, vaultID).Preview
	ids := manifestItemIDs(t, preview)
	result := svc.RestoreManifest(ctx, vaultID, []contract.SkillRestoreSelection{{ID: ids["demo"]}}, "")
	if result.Failure != nil || len(result.Items) != 1 || result.Items[0].Result != "linked" {
		t.Fatalf("restore = %+v", result)
	}
	installed := filepath.Join(home, ".claude", "skills", "demo")
	kind, linkedTo, err := skills.Classify(installed)
	if err != nil || kind != skills.KindJunction {
		t.Fatalf("junction missing: %v %v %q", kind, err, linkedTo)
	}
	if !strings.EqualFold(filepath.Clean(linkedTo), filepath.Clean(filepath.Join(root, "skills", "前端", "demo"))) {
		t.Fatalf("junction target = %q", linkedTo)
	}
	state := skills.LoadState(filepath.Join(svc.app.DataDir, "skills", "state.json"))
	if len(state.Distributions) != 1 || state.Distributions[0].ToolID != "claude-code" {
		t.Fatalf("state = %+v", state.Distributions)
	}
	// Re-running the same restore reports "已一致" and changes nothing.
	again := svc.RestoreManifest(ctx, vaultID, []contract.SkillRestoreSelection{{ID: ids["demo"]}}, "")
	if again.Items[0].Result != "skipped" || !strings.Contains(again.Items[0].Detail, "已一致") {
		t.Fatalf("second restore = %+v", again.Items)
	}

	// A pre-existing directory without a record is a conflict: first pass
	// skips without overwrite, the second overwrites with a backup.
	other := filepath.Join(home, ".claude", "skills", "taken")
	writeInstallFile(t, filepath.Join(other, "SKILL.md"), "# old\n")
	seedManifest(t, root, skills.ManifestItem{
		Source: "kb", Target: skills.ManifestTarget{Kind: "builtin", ToolID: "claude-code"},
		Name: "taken", SkillPath: "skills/前端/demo", Mode: "copy", UpdatedAt: "2026-11-22T09:00:00Z",
	})
	preview = svc.RestorePreview(ctx, vaultID).Preview
	ids = manifestItemIDs(t, preview)
	skipped := svc.RestoreManifest(ctx, vaultID, []contract.SkillRestoreSelection{{ID: ids["taken"]}}, "")
	if skipped.Items[0].Result != "skipped" {
		t.Fatalf("conflict without overwrite = %+v", skipped.Items)
	}
	if body, _ := os.ReadFile(filepath.Join(other, "SKILL.md")); string(body) != "# old\n" {
		t.Fatal("conflict without overwrite touched the directory")
	}
	over := svc.RestoreManifest(ctx, vaultID, []contract.SkillRestoreSelection{{ID: ids["taken"], Overwrite: true}}, "")
	if over.Items[0].Result != "copied" || over.Items[0].BackupID == "" {
		t.Fatalf("conflict overwrite = %+v", over.Items)
	}
	if body, _ := os.ReadFile(filepath.Join(other, "SKILL.md")); !strings.Contains(string(body), "demo") {
		t.Fatalf("overwrite content = %q", body)
	}
}

func TestRestoreGitHubDeduplicatesDownload(t *testing.T) {
	home := t.TempDir()
	writeInstallFile(t, filepath.Join(home, ".local", "bin", "claude.cmd"), "cmd")
	svc, vaultID := skillServiceWithHome(t, home)
	archive := tarGzFixture(t, map[string]string{
		"repo/skills/pdf/SKILL.md":  "---\nname: pdf\n---\n",
		"repo/skills/docx/SKILL.md": "---\nname: docx\n---\n",
	})
	server, downloads := restoreServer(t, archive)
	injectRestoreClient(svc, server)
	root := svc.app.Vaults.List().Vaults[0].Root
	seedManifest(t, root,
		skills.ManifestItem{
			Source: "github", Target: skills.ManifestTarget{Kind: "builtin", ToolID: "claude-code"}, Name: "pdf",
			Owner: "acme", Repo: "skills", Ref: "main", Path: "skills/pdf",
			Commit: installTestSHA, PathCommit: installTestSHA, UpdatedAt: "2026-11-22T09:00:00Z",
		},
		skills.ManifestItem{
			Source: "github", Target: skills.ManifestTarget{Kind: "builtin", ToolID: "claude-code"}, Name: "docx",
			Owner: "acme", Repo: "skills", Ref: "main", Path: "skills/docx",
			Commit: installTestSHA, PathCommit: installTestSHA, UpdatedAt: "2026-11-22T09:00:00Z",
		},
	)
	preview := svc.RestorePreview(context.Background(), vaultID).Preview
	if preview.Pending != 2 {
		t.Fatalf("pending = %d", preview.Pending)
	}
	ids := manifestItemIDs(t, preview)
	result := svc.RestoreManifest(context.Background(), vaultID, []contract.SkillRestoreSelection{
		{ID: ids["pdf"]}, {ID: ids["docx"]},
	}, "req-1")
	if result.Failure != nil {
		t.Fatalf("restore = %+v", result.Failure)
	}
	for _, item := range result.Items {
		if item.Result != "installed" {
			t.Fatalf("item = %+v", item)
		}
	}
	if got := atomic.LoadInt64(downloads); got != 1 {
		t.Fatalf("downloads = %d, want 1", got)
	}
	for _, name := range []string{"pdf", "docx"} {
		if _, err := os.Stat(filepath.Join(home, ".claude", "skills", name, "SKILL.md")); err != nil {
			t.Fatalf("%s missing: %v", name, err)
		}
	}
	sources := skills.LoadToolSources(filepath.Join(svc.app.DataDir, "skills"))
	if len(sources.Records) != 2 {
		t.Fatalf("sources = %+v", sources.Records)
	}
	for _, record := range sources.Records {
		if record.Commit != installTestSHA || record.PathCommit != installTestSHA || record.BaseHash == "" {
			t.Fatalf("record = %+v", record)
		}
	}
}

func TestRestoreCancelledAndUnknownID(t *testing.T) {
	home := t.TempDir()
	writeInstallFile(t, filepath.Join(home, ".local", "bin", "claude.cmd"), "cmd")
	svc, vaultID := skillServiceWithHome(t, home)
	root := svc.app.Vaults.List().Vaults[0].Root
	seedVaultSkill(t, root, "skills/前端/demo")
	seedManifest(t, root, skills.ManifestItem{
		Source: "kb", Target: skills.ManifestTarget{Kind: "builtin", ToolID: "claude-code"},
		Name: "demo", SkillPath: "skills/前端/demo", Mode: "link", UpdatedAt: "2026-11-22T09:00:00Z",
	})
	preview := svc.RestorePreview(context.Background(), vaultID).Preview
	ids := manifestItemIDs(t, preview)

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	result := svc.RestoreManifest(cancelled, vaultID, []contract.SkillRestoreSelection{{ID: ids["demo"]}}, "")
	if len(result.Items) != 1 || result.Items[0].Result != "skipped" || !strings.Contains(result.Items[0].Detail, "已取消") {
		t.Fatalf("cancelled = %+v", result.Items)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "demo")); !os.IsNotExist(err) {
		t.Fatal("cancelled restore wrote something")
	}

	result = svc.RestoreManifest(context.Background(), vaultID, []contract.SkillRestoreSelection{{ID: "r-000000000000"}}, "")
	if len(result.Items) != 1 || result.Items[0].Result != "skipped" || !strings.Contains(result.Items[0].Detail, "清单已变化") {
		t.Fatalf("unknown id = %+v", result.Items)
	}
}

func TestRestoreRejectsUnreadableManifest(t *testing.T) {
	svc, vaultID := skillServiceWithHome(t, t.TempDir())
	result := svc.RestoreManifest(context.Background(), vaultID, []contract.SkillRestoreSelection{{ID: "r-000000000000"}}, "")
	if result.Failure == nil || result.Failure.Code != contract.CodeUnsupported {
		t.Fatalf("unreadable = %+v", result)
	}
}
