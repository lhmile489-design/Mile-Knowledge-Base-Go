package vault

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

func newRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	return root
}

func writeFile(t *testing.T, root, rel, body string) string {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return full
}

func TestIDMatchesLegacyAlgorithm(t *testing.T) {
	// sha256("e:\\vault")[:16] is stable; the same value must come back for the
	// differently-cased spelling because Windows paths are case-insensitive.
	a := ID(`E:\Vault`)
	b := ID(`e:\vault`)
	if a != b {
		t.Errorf("ID is case sensitive: %q vs %q", a, b)
	}
	if len(a) != 16 {
		t.Errorf("id length = %d", len(a))
	}
}

func TestScaffold(t *testing.T) {
	root := newRoot(t)
	if !NeedsScaffold(root) {
		t.Fatal("empty root should need a scaffold")
	}
	created, err := CreateScaffold(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 6 {
		t.Errorf("created %d dirs, want 6", len(created))
	}
	for _, dir := range scaffoldDirectories {
		if _, err := os.Stat(filepath.Join(root, dir, "README.md")); err != nil {
			t.Errorf("%s/README.md missing", dir)
		}
	}
	if NeedsScaffold(root) {
		t.Error("scaffolded root still reports needing a scaffold")
	}
	created, err = CreateScaffold(root)
	if err != nil || len(created) != 0 {
		t.Errorf("second run created %v (err %v)", created, err)
	}
	// skills/ is deliberately not created.
	if _, err := os.Stat(filepath.Join(root, "skills")); !os.IsNotExist(err) {
		t.Error("scaffold must not create skills/")
	}
}

func TestListDirOrderAreaAndIcon(t *testing.T) {
	root := newRoot(t)
	writeFile(t, root, "raws/b.md", "b")
	writeFile(t, root, "raws/a.md", "a")
	writeFile(t, root, "skills/react/SKILL.md", "")
	writeFile(t, root, "skills/go/SKILL.md", "")
	writeFile(t, root, "notes.txt", "x")
	writeFile(t, root, ".obsidian/app.json", "{}")

	res, f := ListDir(root, "", nil)
	if f != nil {
		t.Fatal(f)
	}
	if res.SkillCategoryCount != 2 {
		t.Errorf("skill categories = %d", res.SkillCategoryCount)
	}
	// Directories first, then files, case-insensitive by name.
	var order []string
	for _, e := range res.Entries {
		order = append(order, e.Kind+":"+e.Name)
	}
	want := []string{"directory:.obsidian", "directory:raws", "directory:skills", "file:notes.txt"}
	if len(order) != len(want) {
		t.Fatalf("order = %v", order)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("order[%d] = %q, want %q", i, order[i], want[i])
		}
	}
	byName := map[string]contract.FileEntry{}
	for _, e := range res.Entries {
		byName[e.Name] = e
	}
	if byName["raws"].Area != AreaRaws {
		t.Errorf("raws area = %q", byName["raws"].Area)
	}
	if byName[".obsidian"].Area != AreaSystem || !byName[".obsidian"].Ignored {
		t.Errorf(".obsidian should be system+ignored: %+v", byName[".obsidian"])
	}
	if byName["skills"].Icon != AreaSkills {
		t.Errorf("skills icon = %q", byName["skills"].Icon)
	}
	if byName["raws"].ChildCount == nil || *byName["raws"].ChildCount != 2 {
		t.Errorf("raws childCount = %v", byName["raws"].ChildCount)
	}
}

func TestListDirNestedAndAlias(t *testing.T) {
	root := newRoot(t)
	writeFile(t, root, "skills/react/hooks/a.md", "x")
	res, f := ListDir(root, "skills", map[string]string{"skills": "技能"})
	if f != nil {
		t.Fatal(f)
	}
	if len(res.Entries) != 1 || res.Entries[0].Name != "react" {
		t.Fatalf("entries = %+v", res.Entries)
	}
	// Aliases only apply to top-level rows.
	if res.Entries[0].Alias != "" {
		t.Errorf("nested alias leaked: %q", res.Entries[0].Alias)
	}
	res, f = ListDir(root, "", map[string]string{"skills": "技能"})
	if f != nil {
		t.Fatal(f)
	}
	if res.Entries[0].Alias != "技能" {
		t.Errorf("top-level alias = %q", res.Entries[0].Alias)
	}
	if _, f := ListDir(root, "nope", nil); f == nil || f.Code != contract.CodeNotFound {
		t.Errorf("missing dir failure = %+v", f)
	}
	if _, f := ListDir(root, "../", nil); f == nil || f.Code != contract.CodePathEscape {
		t.Errorf("escape not rejected: %+v", f)
	}
}

func TestReadWriteRoundTripKeepsBytes(t *testing.T) {
	root := newRoot(t)
	bom := "\ufeff# 标题\r\n\r\n正文\r\n"
	full := writeFile(t, root, "note.md", bom)

	snap, f := Read(root, "note.md")
	if f != nil {
		t.Fatal(f)
	}
	if snap.EOL != "crlf" || !snap.BOM {
		t.Fatalf("metadata = %+v", snap)
	}
	if snap.Text != "# 标题\n\n正文\n" {
		t.Errorf("text = %q", snap.Text)
	}

	out, f := Write(root, "note.md", snap.Revision, "# 标题\n\n新正文\n")
	if f != nil {
		t.Fatal(f)
	}
	if out.Status != "written" {
		t.Fatalf("status = %q", out.Status)
	}
	raw, err := os.ReadFile(full)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "\ufeff# 标题\r\n\r\n新正文\r\n" {
		t.Errorf("bytes changed beyond the edit: %q", raw)
	}
}

func TestWriteConflict(t *testing.T) {
	root := newRoot(t)
	full := writeFile(t, root, "note.md", "v1\n")
	snap, f := Read(root, "note.md")
	if f != nil {
		t.Fatal(f)
	}
	// Someone else saves first.
	if err := os.WriteFile(full, []byte("external\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, f := Write(root, "note.md", snap.Revision, "mine\n")
	if f != nil {
		t.Fatal(f)
	}
	if out.Status != "conflict" {
		t.Fatalf("status = %q", out.Status)
	}
	if out.DiskText != "external\n" {
		t.Errorf("disk text = %q", out.DiskText)
	}
	raw, _ := os.ReadFile(full)
	if string(raw) != "external\n" {
		t.Error("conflict wrote to disk")
	}
}

func TestWriteLimitsAndEncoding(t *testing.T) {
	root := newRoot(t)
	if _, f := Write(root, "big.md", 0, string(make([]byte, MaxWriteBytes+1))); f == nil || f.Code != contract.CodeTooLarge {
		t.Errorf("oversize write = %+v", f)
	}
	bad := writeFile(t, root, "gbk.md", "")
	if err := os.WriteFile(bad, []byte{0xC4, 0xE3, 0xBA, 0xC3}, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, f := Read(root, "gbk.md"); f == nil || f.Code != contract.CodeUnsupportedEncoding {
		t.Errorf("non-utf8 read = %+v", f)
	}
	if _, f := Read(root, "missing.md"); f == nil || f.Code != contract.CodeNotFound {
		t.Errorf("missing read = %+v", f)
	}
}

func TestReadDegradedThresholds(t *testing.T) {
	root := newRoot(t)
	mid := make([]byte, SourceOnlyBytes+1)
	for i := range mid {
		mid[i] = 'a'
	}
	writeFile(t, root, "mid.md", string(mid))
	snap, f := Read(root, "mid.md")
	if f != nil {
		t.Fatal(f)
	}
	if snap.Degraded != "large-source-only" {
		t.Errorf("degraded = %q", snap.Degraded)
	}
}

func TestConflictWhenContentChangesButSizeAndMtimeMatch(t *testing.T) {
	root := newRoot(t)
	full := writeFile(t, root, "same.md", "aaaa\n")
	info, err := os.Stat(full)
	if err != nil {
		t.Fatal(err)
	}
	snap, f := Read(root, "same.md")
	if f != nil {
		t.Fatal(f)
	}
	// Same size, same mtime, different bytes: only the hash can tell.
	if err := os.WriteFile(full, []byte("bbbb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(full, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	out, f := Write(root, "same.md", snap.Revision, "mine\n")
	if f != nil {
		t.Fatal(f)
	}
	if out.Status != "conflict" {
		t.Fatalf("status = %q, want conflict (hash must catch it)", out.Status)
	}
	if out.DiskText != "bbbb\n" {
		t.Errorf("diskText = %q", out.DiskText)
	}
}

func TestExpectedZeroOnExistingFileConflicts(t *testing.T) {
	root := newRoot(t)
	writeFile(t, root, "exists.md", "v1\n")
	out, f := Write(root, "exists.md", 0, "v2\n")
	if f != nil {
		t.Fatal(f)
	}
	if out.Status != "conflict" {
		t.Fatalf("status = %q, want conflict", out.Status)
	}
}

func TestRevisionStableAcrossRereadsAndBumpedByWrites(t *testing.T) {
	root := newRoot(t)
	writeFile(t, root, "r.md", "one\n")
	first, f := Read(root, "r.md")
	if f != nil {
		t.Fatal(f)
	}
	again, f := Read(root, "r.md")
	if f != nil {
		t.Fatal(f)
	}
	if again.Revision != first.Revision {
		t.Errorf("revision changed without a write: %d → %d", first.Revision, again.Revision)
	}
	out, f := Write(root, "r.md", first.Revision, "two\n")
	if f != nil {
		t.Fatal(f)
	}
	if out.Revision <= first.Revision {
		t.Errorf("write did not bump revision: %d → %d", first.Revision, out.Revision)
	}
}

func TestNoTempFilesLeftBehind(t *testing.T) {
	root := newRoot(t)
	writeFile(t, root, "t.md", "v1\n")
	snap, _ := Read(root, "t.md")
	// Successful write.
	if _, f := Write(root, "t.md", snap.Revision, "v2\n"); f != nil {
		t.Fatal(f)
	}
	// Conflict (disk changed under us).
	if err := os.WriteFile(filepath.Join(root, "t.md"), []byte("v3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, f := Write(root, "t.md", snap.Revision, "v4\n"); f != nil || out.Status != "conflict" {
		t.Fatalf("expected conflict, got %+v %v", out, f)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if len(e.Name()) > 5 && e.Name()[0] == '.' && filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

func TestNegativeExpectedRevisionConflicts(t *testing.T) {
	root := newRoot(t)
	writeFile(t, root, "n.md", "v1\n")
	out, f := Write(root, "n.md", -5, "v2\n")
	if f != nil {
		t.Fatal(f)
	}
	if out.Status != "conflict" {
		t.Errorf("status = %q", out.Status)
	}
}
