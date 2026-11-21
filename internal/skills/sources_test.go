package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

const sourceCommit = "0123456789abcdef0123456789abcdef01234567"

const sampleBaseHash = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func sampleSource(skillPath string) KbSource {
	return KbSource{
		SkillPath:   skillPath,
		Owner:       "anthropics",
		Repo:        "skills",
		Ref:         "main",
		Path:        "skills/pdf",
		Commit:      sourceCommit,
		PathCommit:  sourceCommit,
		BaseHash:    sampleBaseHash,
		InstalledAt: "2026-11-21T09:00:00Z",
	}
}

func TestKbSourcesStableSerialization(t *testing.T) {
	vault := t.TempDir()
	// Write out of order; the file must come out sorted by skillPath.
	if f := SaveKbSource(vault, sampleSource("skills/前端/vue")); f != nil {
		t.Fatalf("save: %+v", f)
	}
	if f := SaveKbSource(vault, sampleSource("skills/后端/go")); f != nil {
		t.Fatalf("save: %+v", f)
	}
	body, err := os.ReadFile(filepath.Join(vault, ".mile-kb", "skill-sources.json"))
	if err != nil {
		t.Fatal(err)
	}
	expected := `{
  "version": 1,
  "records": [
    {
      "skillPath": "skills/前端/vue",
      "owner": "anthropics",
      "repo": "skills",
      "ref": "main",
      "path": "skills/pdf",
      "commit": "` + sourceCommit + `",
      "pathCommit": "` + sourceCommit + `",
      "baseHash": "` + sampleBaseHash + `",
      "installedAt": "2026-11-21T09:00:00Z"
    },
    {
      "skillPath": "skills/后端/go",
      "owner": "anthropics",
      "repo": "skills",
      "ref": "main",
      "path": "skills/pdf",
      "commit": "` + sourceCommit + `",
      "pathCommit": "` + sourceCommit + `",
      "baseHash": "` + sampleBaseHash + `",
      "installedAt": "2026-11-21T09:00:00Z"
    }
  ]
}
`
	if string(body) != expected {
		t.Fatalf("serialization mismatch:\n--- got ---\n%s\n--- want ---\n%s", body, expected)
	}
	if strings.Contains(string(body), `\u`) {
		t.Fatalf("HTML escaping leaked into the file: %s", body)
	}
}

func TestKbSourcesUnknownVersionAndInvalidRecords(t *testing.T) {
	vault := t.TempDir()
	dir := filepath.Join(vault, ".mile-kb")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "skill-sources.json")
	if err := os.WriteFile(path, []byte(`{"version":2,"records":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded := LoadKbSources(vault)
	if !loaded.VersionUnknown {
		t.Fatalf("version 2 should be unknown: %+v", loaded)
	}
	if f := SaveKbSource(vault, sampleSource("skills/前端/vue")); f == nil || f.Code != contract.CodeUnsupported {
		t.Fatalf("save over unknown version = %+v", f)
	}
	if body, _ := os.ReadFile(path); !strings.Contains(string(body), `"version":2`) {
		t.Fatalf("unknown-version file was rewritten: %s", body)
	}

	broken := `{"version":1,"records":[{"skillPath":"skills/x","owner":"a","repo":"b","ref":"main","commit":"` + sourceCommit + `","pathCommit":"` + sourceCommit + `"},{"skillPath":"skills/前端/vue","owner":"anthropics","repo":"skills","ref":"main","path":"","commit":"` + sourceCommit + `","pathCommit":"` + sourceCommit + `","baseHash":"` + sampleBaseHash + `","installedAt":"2026-11-21T09:00:00Z"}]}`
	if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded = LoadKbSources(vault)
	if len(loaded.Records) != 1 || len(loaded.Warnings) == 0 {
		t.Fatalf("invalid records = %+v", loaded)
	}
	if f := RemoveKbSource(vault, "skills/前端/vue"); f != nil {
		t.Fatalf("remove: %+v", f)
	}
	if loaded := LoadKbSources(vault); len(loaded.Records) != 0 {
		t.Fatalf("record not removed: %+v", loaded)
	}
}

func TestSourceRecordsRequireBaseHashAndTime(t *testing.T) {
	vault := t.TempDir()
	badHash := sampleSource("skills/前端/vue")
	badHash.BaseHash = "sha256:abc"
	if f := SaveKbSource(vault, badHash); f == nil || f.Code != contract.CodeInvalidInput {
		t.Fatalf("bad hash = %+v", f)
	}
	badTime := sampleSource("skills/前端/vue")
	badTime.InstalledAt = ""
	if f := SaveKbSource(vault, badTime); f == nil || f.Code != contract.CodeInvalidInput {
		t.Fatalf("bad time = %+v", f)
	}
	empty := sampleSource("skills/前端/vue")
	empty.BaseHash = ""
	if f := SaveKbSource(vault, empty); f == nil || f.Code != contract.CodeInvalidInput {
		t.Fatalf("empty hash = %+v", f)
	}

	// A hand-edited record without baseHash is dropped instead of making the
	// update check treat the skill as never modified.
	raw := `{"version":1,"records":[{"skillPath":"skills/前端/vue","owner":"anthropics","repo":"skills","ref":"main","path":"","commit":"` + sourceCommit + `","pathCommit":"` + sourceCommit + `","installedAt":"2026-11-21T09:00:00Z"}]}`
	dir := filepath.Join(vault, ".mile-kb")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skill-sources.json"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded := LoadKbSources(vault)
	if len(loaded.Records) != 0 || len(loaded.Warnings) == 0 {
		t.Fatalf("record without baseHash = %+v", loaded)
	}
}

func TestToolSources(t *testing.T) {
	skillsDir := t.TempDir()
	sample := sampleSource("skills/文档/pdf")
	record := ToolSource{
		ToolID: "claude-code", TargetPath: filepath.Join("C:", "Users", "x", ".claude", "skills", "pdf"),
		Owner: sample.Owner, Repo: sample.Repo, Ref: sample.Ref, Path: sample.Path,
		Commit: sample.Commit, PathCommit: sample.PathCommit, BaseHash: sample.BaseHash, InstalledAt: sample.InstalledAt,
	}
	if f := SaveToolSource(skillsDir, record); f != nil {
		t.Fatalf("save: %+v", f)
	}
	loaded := LoadToolSources(skillsDir)
	if len(loaded.Records) != 1 || !IsToolSourceID(loaded.Records[0].ID) {
		t.Fatalf("loaded = %+v", loaded)
	}
	if f := RemoveToolSource(skillsDir, record.TargetPath); f != nil {
		t.Fatalf("remove: %+v", f)
	}
	if loaded := LoadToolSources(skillsDir); len(loaded.Records) != 0 {
		t.Fatalf("record not removed: %+v", loaded)
	}

	if err := os.WriteFile(filepath.Join(skillsDir, "sources.json"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	LoadToolSources(skillsDir)
	matches, _ := filepath.Glob(filepath.Join(skillsDir, "sources.json.corrupt-*"))
	if len(matches) != 1 {
		t.Fatalf("corrupt file was not renamed: %v", matches)
	}
}

func TestAttachKbRemotes(t *testing.T) {
	vault := t.TempDir()
	dir := filepath.Join(vault, "skills", "文档", "pdf")
	writeFile(t, filepath.Join(dir, "SKILL.md"), "# pdf\n")
	record := sampleSource("skills/文档/pdf")
	if f := SaveKbSource(vault, record); f != nil {
		t.Fatalf("save: %+v", f)
	}
	settings, err := TreeHash(dir)
	if err != nil {
		t.Fatal(err)
	}
	record.BaseHash = settings
	if f := SaveKbSource(vault, record); f != nil {
		t.Fatalf("save: %+v", f)
	}

	kbs := []contract.KbSkill{{Name: "pdf", Path: "skills/文档/pdf"}}
	AttachKbRemotes(vault, kbs, LoadKbSources(vault), nil)
	if kbs[0].Remote == nil || kbs[0].Remote.UpdateState != "unchecked" || kbs[0].Remote.Owner != "anthropics" {
		t.Fatalf("remote = %+v", kbs[0].Remote)
	}

	writeFile(t, filepath.Join(dir, "SKILL.md"), "# pdf changed\n")
	AttachKbRemotes(vault, kbs, LoadKbSources(vault), nil)
	if kbs[0].Remote == nil || kbs[0].Remote.UpdateState != "local-modified" {
		t.Fatalf("local change = %+v", kbs[0].Remote)
	}
}

func TestScanAttachesToolRemote(t *testing.T) {
	root := t.TempDir()
	install := filepath.Join(root, "skills")
	dir := filepath.Join(install, "pdf")
	writeFile(t, filepath.Join(dir, "SKILL.md"), "# pdf\n")
	hash, err := TreeHash(dir)
	if err != nil {
		t.Fatal(err)
	}
	sample := sampleSource("skills/文档/pdf")
	record := ToolSource{
		ToolID: "claude-code", TargetPath: dir, Owner: sample.Owner, Repo: sample.Repo, Ref: sample.Ref,
		Path: sample.Path, Commit: sample.Commit, PathCommit: sample.PathCommit, BaseHash: sample.BaseHash,
		InstalledAt: sample.InstalledAt,
	}
	record.BaseHash = hash
	record.Commit = sourceCommit
	record.PathCommit = sourceCommit
	groups, _ := ScanTools(ScanOptions{
		Tools:       []DetectedTool{{ID: "claude-code", Status: StatusDetected, InstallDir: install}},
		ToolSources: []ToolSource{record},
	})
	var remote *contract.SkillRemote
	for _, skill := range groups[0].Skills {
		if skill.Name == "pdf" {
			remote = skill.Remote
		}
	}
	if remote == nil || remote.UpdateState != "unchecked" {
		t.Fatalf("tool remote = %+v", remote)
	}
}
