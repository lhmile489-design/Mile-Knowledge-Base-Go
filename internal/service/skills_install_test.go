package service

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/skills"
)

const installTestSHA = "0123456789abcdef0123456789abcdef01234567"

func tarGzFixture(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(gz)
	for name, body := range files {
		header := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func fakeGithubServer(t *testing.T, archive []byte) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/acme/skills":
			_ = json.NewEncoder(w).Encode(map[string]string{"default_branch": "main"})
		case r.URL.Path == "/repos/acme/skills/commits/main":
			_, _ = w.Write([]byte(installTestSHA))
		case r.URL.Path == "/acme/skills/tar.gz/"+installTestSHA:
			_, _ = w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func githubStagedService(t *testing.T, server *httptest.Server) (*SkillService, string) {
	t.Helper()
	svc, vaultID := skillServiceWithHome(t, t.TempDir())
	svc.githubClient = func(string) *skills.GithubClient {
		client := skills.NewGithubClientForTest()
		client.APIBase = server.URL
		client.CodeloadBase = server.URL
		client.SetHTTPForTest(server.Client().Transport)
		return client
	}
	return svc, vaultID
}

func TestSkillServiceReadGithubStages(t *testing.T) {
	archive := tarGzFixture(t, map[string]string{
		"skills-0123/skills/pdf/SKILL.md":       "---\nname: pdf\ndescription: PDF tools\n---\n",
		"skills-0123/skills/pdf/reference/a.md": "a",
		"skills-0123/skills/docx/SKILL.md":      "# docx\n",
		"skills-0123/README.md":                 "readme",
	})
	server := fakeGithubServer(t, archive)
	svc, vaultID := githubStagedService(t, server)

	result := svc.ReadGithub(context.Background(), vaultID, "acme/skills")
	if result.Failure != nil {
		t.Fatalf("read github: %+v", result.Failure)
	}
	if result.Stage == nil || result.Stage.Kind != "github" || result.Stage.Origin == nil {
		t.Fatalf("stage = %+v", result.Stage)
	}
	if result.Stage.Origin.Commit != installTestSHA || result.Stage.Origin.Ref != "main" {
		t.Fatalf("origin = %+v", result.Stage.Origin)
	}
	states := map[string]string{}
	for _, candidate := range result.Stage.Candidates {
		states[candidate.Name] = candidate.State
	}
	if states["pdf"] != "ok" || states["docx"] != "ok" {
		t.Fatalf("candidates = %+v", result.Stage.Candidates)
	}

	stageDir := filepath.Join(svc.app.DataDir, "skills", "tmp", result.Stage.StageID)
	if _, err := os.Stat(stageDir); err != nil {
		t.Fatalf("stage dir missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(stageDir, "archive.tar.gz")); !os.IsNotExist(err) {
		t.Fatalf("archive should be removed after unpacking")
	}
	metaBody, err := os.ReadFile(filepath.Join(stageDir, "meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	var meta skills.StageMeta
	if err := json.Unmarshal(metaBody, &meta); err != nil {
		t.Fatal(err)
	}
	if meta.PathCommit != installTestSHA || len(meta.TreeHashes) == 0 {
		t.Fatalf("meta = %+v", meta)
	}

	if f := svc.DiscardStage(context.Background(), result.Stage.StageID); f.Code != "" {
		t.Fatalf("discard: %+v", f)
	}
	if _, err := os.Stat(stageDir); !os.IsNotExist(err) {
		t.Fatalf("discarded stage still exists")
	}
}

func TestSkillServiceReadGithubRejectsBadInput(t *testing.T) {
	svc, vaultID := skillServiceWithHome(t, t.TempDir())
	result := svc.ReadGithub(context.Background(), vaultID, "https://gitlab.com/a/b")
	if result.Failure == nil || result.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("bad input = %+v", result)
	}
}

func TestSkillServiceReadGithubCancelled(t *testing.T) {
	server := fakeGithubServer(t, tarGzFixture(t, map[string]string{"repo/SKILL.md": "# x\n"}))
	svc, vaultID := githubStagedService(t, server)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := svc.ReadGithub(ctx, vaultID, "acme/skills")
	if !result.Cancelled {
		t.Fatalf("cancel = %+v", result)
	}
	entries, _ := os.ReadDir(filepath.Join(svc.app.DataDir, "skills", "tmp"))
	for _, entry := range entries {
		t.Fatalf("cancelled read left a stage: %s", entry.Name())
	}
}

func TestSkillServiceReadGithubNotFound(t *testing.T) {
	server := fakeGithubServer(t, nil)
	svc, vaultID := githubStagedService(t, server)
	result := svc.ReadGithub(context.Background(), vaultID, "acme/missing")
	if result.Failure == nil || result.Failure.Code != contract.CodeNotFound {
		t.Fatalf("not found = %+v", result)
	}
}

func TestSkillServiceDiscardUnknownStage(t *testing.T) {
	svc, _ := skillServiceWithHome(t, t.TempDir())
	if f := svc.DiscardStage(context.Background(), "s-0123456789ab"); f.Code != "" {
		t.Fatalf("discard unknown = %+v", f)
	}
	if f := svc.DiscardStage(context.Background(), "bad"); f.Code != contract.CodeInvalidInput {
		t.Fatalf("discard bad id = %+v", f)
	}
}

func TestSkillServiceReadGithubProgressPhase(t *testing.T) {
	archive := tarGzFixture(t, map[string]string{"repo/SKILL.md": strings.Repeat("# x\n", 10)})
	server := fakeGithubServer(t, archive)
	svc, vaultID := githubStagedService(t, server)
	svc.now = func() time.Time { return time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC) }
	result := svc.ReadGithub(context.Background(), vaultID, "acme/skills")
	if result.Stage == nil {
		t.Fatalf("stage = %+v", result)
	}
	_ = svc.DiscardStage(context.Background(), result.Stage.StageID)
}

func writeZipFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "company-skills.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for name, body := range map[string]string{
		"skills/pdf/SKILL.md":  "# pdf\n",
		"skills/docx/SKILL.md": "# docx\n",
	} {
		part, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSkillServicePickZipStages(t *testing.T) {
	path := writeZipFixture(t)
	svc, vaultID := skillServiceWithHome(t, t.TempDir())
	svc.openDialog = func(context.Context, string, []string) ([]string, error) {
		return []string{path}, nil
	}
	result := svc.PickZip(context.Background(), vaultID)
	if result.Failure != nil || result.Stage == nil {
		t.Fatalf("pick zip = %+v", result)
	}
	if result.Stage.Kind != "zip" || result.Stage.Label != "company-skills.zip" {
		t.Fatalf("stage = %+v", result.Stage)
	}
	names := map[string]bool{}
	for _, candidate := range result.Stage.Candidates {
		names[candidate.Name] = candidate.State == "ok"
	}
	if !names["pdf"] || !names["docx"] {
		t.Fatalf("candidates = %+v", result.Stage.Candidates)
	}
	_ = svc.DiscardStage(context.Background(), result.Stage.StageID)
}

func TestSkillServicePickZipCancelled(t *testing.T) {
	svc, vaultID := skillServiceWithHome(t, t.TempDir())
	svc.openDialog = func(context.Context, string, []string) ([]string, error) { return nil, nil }
	if result := svc.PickZip(context.Background(), vaultID); !result.Cancelled {
		t.Fatalf("cancel = %+v", result)
	}
}
