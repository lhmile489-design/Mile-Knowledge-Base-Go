package skills

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

func TestUpdateCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)
	stamp := now.Format(time.RFC3339)
	results := []UpdateResult{
		{Owner: "anthropics", Repo: "skills", Ref: "main", Path: "skills/pdf", LatestPathCommit: sourceCommit, CheckedAt: stamp},
		{Owner: "acme", Repo: "tools", Ref: "main", Error: "连不上 GitHub", CheckedAt: stamp},
	}
	if f := SaveUpdateCache(dir, results, now); f != nil {
		t.Fatalf("save: %+v", f)
	}
	cache := LoadUpdateCache(dir)
	if cache == nil || len(cache.Results) != 2 || cache.CheckedAt != stamp {
		t.Fatalf("cache = %+v", cache)
	}
	if !cache.Fresh(now.Add(59 * time.Minute)) {
		t.Fatalf("59 minutes should be fresh")
	}
	if cache.Fresh(now.Add(61 * time.Minute)) {
		t.Fatalf("61 minutes should be stale")
	}
	entry, ok := cache.Lookup("anthropics", "skills", "main", "skills/pdf")
	if !ok || entry.LatestPathCommit != sourceCommit {
		t.Fatalf("lookup = %+v %v", entry, ok)
	}
	var groups []UpdateGroup
	groups = append(groups, UpdateGroup{Owner: "anthropics", Repo: "skills", Ref: "main", Path: "skills/pdf"})
	groups = append(groups, UpdateGroup{Owner: "acme", Repo: "tools", Ref: "main"})
	groups = append(groups, UpdateGroup{Owner: "acme", Repo: "more", Ref: "main", Path: "skills/x"})
	if got := cache.CountUnchecked(groups); got != 2 {
		t.Fatalf("unchecked = %d, want 2", got)
	}

	if err := os.WriteFile(filepath.Join(dir, updateCacheName), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if LoadUpdateCache(dir) != nil {
		t.Fatalf("damaged cache was not dropped")
	}
	if _, err := os.Stat(filepath.Join(dir, updateCacheName)); !os.IsNotExist(err) {
		t.Fatalf("damaged cache was not deleted")
	}

	unknown := []byte(`{"version":2,"checkedAt":"2026-11-21T10:00:00Z","results":[]}`)
	if err := os.WriteFile(filepath.Join(dir, updateCacheName), unknown, 0o644); err != nil {
		t.Fatal(err)
	}
	if LoadUpdateCache(dir) != nil {
		t.Fatalf("unknown version was not dropped")
	}
	if _, err := os.Stat(filepath.Join(dir, updateCacheName)); !os.IsNotExist(err) {
		t.Fatalf("unknown version was not deleted")
	}
}

func TestCollectUpdateGroups(t *testing.T) {
	kb := []KbSource{
		sampleSource("skills/文档/pdf"),
		{SkillPath: "skills/文档/other", Owner: "acme", Repo: "tools", Ref: "main", Path: "skills/other"},
	}
	tool := []ToolSource{{ToolID: "claude-code", TargetPath: "x", Owner: "anthropics", Repo: "skills", Ref: "main", Path: "skills/pdf"}}
	groups := CollectUpdateGroups(kb, tool)
	if len(groups) != 2 {
		t.Fatalf("groups = %+v", groups)
	}
	// Sorted by the raw group key: "acme" < "anthropics".
	if groups[0].Owner != "acme" || groups[0].Path != "skills/other" || groups[1].Owner != "anthropics" {
		t.Fatalf("order = %+v", groups)
	}
	if got := UpdateGroupKey("a", "b", "c", "d"); got == UpdateGroupKey("a", "b", "c", "e") {
		t.Fatalf("group key does not separate paths")
	}
}

func TestRemoteUpdateState(t *testing.T) {
	const other = "89abcdef0123456789abcdef0123456789abcdef"
	stamp := "2026-11-21T10:00:00Z"
	cases := []struct {
		name      string
		latest    string
		pollError string
		base      string
		local     string
		checked   bool
		want      string
	}{
		{"up to date", sourceCommit, "", "sha256:a", "sha256:a", true, "up-to-date"},
		{"remote changed", other, "", "sha256:a", "sha256:a", true, "update-available"},
		{"both changed", other, "", "sha256:a", "sha256:b", true, "both-changed"},
		{"local changed", sourceCommit, "", "sha256:a", "sha256:b", true, "local-modified"},
		{"never checked", "", "", "sha256:a", "sha256:a", false, "unchecked"},
		{"unchecked but local", "", "", "sha256:a", "sha256:b", false, "local-modified"},
		{"check failed", "", "连不上 GitHub", "sha256:a", "sha256:a", true, "error"},
	}
	for _, test := range cases {
		result := UpdateResult{LatestPathCommit: test.latest, Error: test.pollError, CheckedAt: stamp}
		state, latest, checkedAt, errText := RemoteUpdateState(sourceCommit, test.base, test.local, result, test.checked)
		if state != test.want {
			t.Fatalf("%s = %q, want %q", test.name, state, test.want)
		}
		if test.checked && test.pollError == "" {
			if latest != test.latest || checkedAt != stamp {
				t.Fatalf("%s latest = %q %q", test.name, latest, checkedAt)
			}
		}
		if test.want == "error" && errText != test.pollError {
			t.Fatalf("%s error = %q", test.name, errText)
		}
	}
}

func TestAttachKbRemotesWithCache(t *testing.T) {
	vault := t.TempDir()
	dir := filepath.Join(vault, "skills", "文档", "pdf")
	writeFile(t, filepath.Join(dir, "SKILL.md"), "# pdf\n")
	base, err := TreeHash(dir)
	if err != nil {
		t.Fatal(err)
	}
	record := sampleSource("skills/文档/pdf")
	record.BaseHash = base
	if f := SaveKbSource(vault, record); f != nil {
		t.Fatalf("save: %+v", f)
	}
	kbs := []contract.KbSkill{{Name: "pdf", Path: "skills/文档/pdf"}}

	// remote changed → update-available
	now := time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)
	other := "89abcdef0123456789abcdef0123456789abcdef"
	cacheDir := t.TempDir()
	if f := SaveUpdateCache(cacheDir, []UpdateResult{{Owner: "anthropics", Repo: "skills", Ref: "main", Path: "skills/pdf", LatestPathCommit: other, CheckedAt: now.Format(time.RFC3339)}}, now); f != nil {
		t.Fatalf("save cache: %+v", f)
	}
	AttachKbRemotes(vault, kbs, LoadKbSources(vault), LoadUpdateCache(cacheDir))
	if kbs[0].Remote == nil || kbs[0].Remote.UpdateState != "update-available" ||
		kbs[0].Remote.LatestPathCommit != other || kbs[0].Remote.CheckedAt == "" {
		t.Fatalf("remote changed = %+v", kbs[0].Remote)
	}

	// local changed on top → both-changed
	writeFile(t, filepath.Join(dir, "SKILL.md"), "# pdf changed\n")
	AttachKbRemotes(vault, kbs, LoadKbSources(vault), LoadUpdateCache(cacheDir))
	if kbs[0].Remote == nil || kbs[0].Remote.UpdateState != "both-changed" {
		t.Fatalf("both changed = %+v", kbs[0].Remote)
	}

	// same remote commit → up-to-date for the fresh copy
	freshVault := t.TempDir()
	freshDir := filepath.Join(freshVault, "skills", "文档", "pdf")
	writeFile(t, filepath.Join(freshDir, "SKILL.md"), "# pdf\n")
	freshRecord := sampleSource("skills/文档/pdf")
	freshRecord.BaseHash = base
	if f := SaveKbSource(freshVault, freshRecord); f != nil {
		t.Fatal(f)
	}
	if f := SaveUpdateCache(cacheDir, []UpdateResult{{Owner: "anthropics", Repo: "skills", Ref: "main", Path: "skills/pdf", LatestPathCommit: sourceCommit, CheckedAt: now.Format(time.RFC3339)}}, now); f != nil {
		t.Fatal(f)
	}
	freshKbs := []contract.KbSkill{{Name: "pdf", Path: "skills/文档/pdf"}}
	AttachKbRemotes(freshVault, freshKbs, LoadKbSources(freshVault), LoadUpdateCache(cacheDir))
	if freshKbs[0].Remote == nil || freshKbs[0].Remote.UpdateState != "up-to-date" {
		t.Fatalf("up to date = %+v", freshKbs[0].Remote)
	}

	// a failed check shows the reason
	if f := SaveUpdateCache(cacheDir, []UpdateResult{{Owner: "anthropics", Repo: "skills", Ref: "main", Path: "skills/pdf", Error: "连不上 GitHub", CheckedAt: now.Format(time.RFC3339)}}, now); f != nil {
		t.Fatal(f)
	}
	AttachKbRemotes(freshVault, freshKbs, LoadKbSources(freshVault), LoadUpdateCache(cacheDir))
	if freshKbs[0].Remote == nil || freshKbs[0].Remote.UpdateState != "error" || freshKbs[0].Remote.Error != "连不上 GitHub" {
		t.Fatalf("error state = %+v", freshKbs[0].Remote)
	}
}

func TestScanAttachesToolRemoteWithCache(t *testing.T) {
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
		Path: sample.Path, Commit: sample.Commit, PathCommit: sourceCommit, BaseHash: hash,
		InstalledAt: sample.InstalledAt,
	}
	other := "89abcdef0123456789abcdef0123456789abcdef"
	cacheDir := t.TempDir()
	now := time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)
	if f := SaveUpdateCache(cacheDir, []UpdateResult{{Owner: sample.Owner, Repo: sample.Repo, Ref: sample.Ref, Path: sample.Path, LatestPathCommit: other, CheckedAt: now.Format(time.RFC3339)}}, now); f != nil {
		t.Fatal(f)
	}
	groups, _ := ScanTools(ScanOptions{
		Tools:       []DetectedTool{{ID: "claude-code", Status: StatusDetected, InstallDir: install}},
		ToolSources: []ToolSource{record},
		UpdateCache: LoadUpdateCache(cacheDir),
	})
	var remote *contract.SkillRemote
	for _, skill := range groups[0].Skills {
		if skill.Name == "pdf" {
			remote = skill.Remote
		}
	}
	if remote == nil || remote.UpdateState != "update-available" || remote.LatestPathCommit != other {
		t.Fatalf("tool remote = %+v", remote)
	}
}
