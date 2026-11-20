package skills

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

type tarEntry struct {
	name     string
	body     string
	typeflag byte
	linkname string
}

func buildTarGz(t *testing.T, entries []tarEntry) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(gz)
	for _, item := range entries {
		flag := item.typeflag
		if flag == 0 {
			flag = tar.TypeReg
		}
		header := &tar.Header{Name: item.name, Mode: 0o644, Typeflag: flag, Linkname: item.linkname}
		if flag == tar.TypeReg {
			header.Size = int64(len(item.body))
		}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if flag == tar.TypeReg {
			if _, err := writer.Write([]byte(item.body)); err != nil {
				t.Fatal(err)
			}
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

func extractTar(t *testing.T, entries []tarEntry, options ExtractOptions) (ExtractResult, *contract.Failure, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	result, f := ExtractTarGz(bytes.NewReader(buildTarGz(t, entries)), root, options)
	return result, f, root
}

func TestExtractTarGzValid(t *testing.T) {
	result, f, root := extractTar(t, []tarEntry{
		{name: "skills-0123/", typeflag: tar.TypeDir},
		{name: "skills-0123/SKILL.md", body: "# skill\n"},
		{name: "skills-0123/reference/a.md", body: "a"},
		{name: "skills-0123/link", typeflag: tar.TypeSymlink, linkname: "/etc/passwd"},
	}, ExtractOptions{})
	if f != nil {
		t.Fatalf("extract: %+v", f)
	}
	body, err := os.ReadFile(filepath.Join(root, "SKILL.md"))
	if err != nil || string(body) != "# skill\n" {
		t.Fatalf("SKILL.md = %q err=%v", body, err)
	}
	if _, err := os.Stat(filepath.Join(root, "reference", "a.md")); err != nil {
		t.Fatalf("nested file missing: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "link")); !os.IsNotExist(err) {
		t.Fatalf("symlink should not be created")
	}
	if result.Files != 2 || len(result.Skipped) != 1 || result.Skipped[0].Reason != "symlink" {
		t.Fatalf("result = %+v", result)
	}
}

func TestExtractTarGzRejectsUnsafe(t *testing.T) {
	cases := []struct {
		name    string
		entries []tarEntry
	}{
		{"parent", []tarEntry{{name: "repo/../evil.md", body: "x"}}},
		{"absolute", []tarEntry{{name: "/evil.md", body: "x"}}},
		{"drive", []tarEntry{{name: "repo/C:/evil.md", body: "x"}}},
		{"reserved", []tarEntry{{name: "repo/CON/x.md", body: "x"}}},
		{"reserved-with-ext", []tarEntry{{name: "repo/nul.txt", body: "x"}}},
		{"trailing-dot", []tarEntry{{name: "repo/a./x.md", body: "x"}}},
		{"trailing-space", []tarEntry{{name: "repo/a /x.md", body: "x"}}},
		{"case-dup", []tarEntry{{name: "repo/A/x.md", body: "x"}, {name: "repo/a/x.md", body: "y"}}},
		{"dot-segment", []tarEntry{{name: "repo/./x.md", body: "x"}}},
	}
	for _, item := range cases {
		_, f, root := extractTar(t, item.entries, ExtractOptions{})
		if f == nil || f.Code != contract.CodeUnsafeArchive {
			t.Errorf("%s: got %+v, want unsafe-archive", item.name, f)
			continue
		}
		if _, err := os.Stat(root); !os.IsNotExist(err) {
			t.Errorf("%s: unsafe archive left the root behind", item.name)
		}
	}
}

func TestExtractTarGzSizeAndEntryCaps(t *testing.T) {
	_, f, root := extractTar(t, []tarEntry{{name: "repo/big.md", body: strings.Repeat("x", 50)}}, ExtractOptions{MaxBytes: 20})
	if f == nil || f.Code != contract.CodeTooLarge {
		t.Fatalf("size cap = %+v", f)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("size cap left the root behind")
	}

	_, f, _ = extractTar(t, []tarEntry{{name: "repo/a.md", body: "a"}, {name: "repo/b.md", body: "b"}}, ExtractOptions{MaxEntries: 1})
	if f == nil || f.Code != contract.CodeTooLarge {
		t.Fatalf("entry cap = %+v", f)
	}
}

// writeFile counts the bytes it actually copies, not the declared size.
func TestWriteFileCountsRealBytes(t *testing.T) {
	root := t.TempDir()
	state := &extractState{root: root, options: ExtractOptions{MaxBytes: 20}, seen: map[string]string{}}
	target, err := ResolveUnder(root, "a.md")
	if err != nil {
		t.Fatal(err)
	}
	if f := state.writeFile(target, strings.NewReader(strings.Repeat("x", 50)), 5); f == nil || f.Code != contract.CodeTooLarge {
		t.Fatalf("declared size hid the real one: %+v", f)
	}
}

type zipEntry struct {
	name string
	body string
	mode os.FileMode
}

func buildZip(t *testing.T, entries []zipEntry) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "archive.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for _, item := range entries {
		header := &zip.FileHeader{Name: item.name, Method: zip.Store}
		header.SetMode(item.mode)
		part, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte(item.body)); err != nil {
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

func TestExtractZipValidAndSkipSymlink(t *testing.T) {
	path := buildZip(t, []zipEntry{
		{"repo/SKILL.md", "# skill\n", 0o644},
		{"repo/ref/a.md", "a", 0o644},
		{"repo/link", "/etc/passwd", os.ModeSymlink | 0o777},
	})
	root := filepath.Join(t.TempDir(), "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	result, f := ExtractZip(path, root, ExtractOptions{})
	if f != nil {
		t.Fatalf("extract: %+v", f)
	}
	if body, err := os.ReadFile(filepath.Join(root, "SKILL.md")); err != nil || string(body) != "# skill\n" {
		t.Fatalf("SKILL.md = %q err=%v", body, err)
	}
	if _, err := os.Lstat(filepath.Join(root, "link")); !os.IsNotExist(err) {
		t.Fatalf("symlink should not be created")
	}
	if len(result.Skipped) != 1 || result.Skipped[0].Reason != "symlink" {
		t.Fatalf("skipped = %+v", result.Skipped)
	}
}

func TestExtractZipRejectsBeforeWriting(t *testing.T) {
	path := buildZip(t, []zipEntry{
		{"repo/good.md", "good", 0o644},
		{"repo/../evil.md", "evil", 0o644},
	})
	root := filepath.Join(t.TempDir(), "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	_, f := ExtractZip(path, root, ExtractOptions{})
	if f == nil || f.Code != contract.CodeUnsafeArchive {
		t.Fatalf("zip unsafe = %+v", f)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("zip failure left the root behind")
	}
}

func TestExtractZipSizeCap(t *testing.T) {
	path := buildZip(t, []zipEntry{{"repo/big.md", strings.Repeat("x", 40), 0o644}})
	root := filepath.Join(t.TempDir(), "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	_, f := ExtractZip(path, root, ExtractOptions{MaxBytes: 20})
	if f == nil || f.Code != contract.CodeTooLarge {
		t.Fatalf("zip size cap = %+v", f)
	}
}

func TestStageStore(t *testing.T) {
	now := time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC)
	store := NewStageStore(t.TempDir(), func() time.Time { return now })
	first, f := store.Create("github", "acme/skills", &contract.SkillOrigin{Owner: "acme", Repo: "skills", Ref: "main", Path: "skills/pdf", Commit: strings.Repeat("a", 40)})
	if f != nil {
		t.Fatalf("create: %+v", f)
	}
	second, f := store.Create("zip", "skills.zip", nil)
	if f != nil {
		t.Fatalf("create: %+v", f)
	}
	third, f := store.Create("zip", "second.zip", nil)
	if f != nil {
		t.Fatalf("create: %+v", f)
	}
	fourth, f := store.Create("zip", "third.zip", nil)
	if f != nil {
		t.Fatalf("create: %+v", f)
	}
	if _, f := store.Get(first.Meta.ID); f == nil || f.Code != contract.CodeNotFound {
		t.Fatalf("oldest stage should have been dropped: %+v", f)
	}
	for _, stage := range []Stage{second, third, fourth} {
		if _, f := store.Get(stage.Meta.ID); f != nil {
			t.Fatalf("stage %s missing: %+v", stage.Meta.ID, f)
		}
	}
	if f := fourth.SetTreeHashes(map[string]string{"skills/pdf": "sha256:abc"}); f != nil {
		t.Fatalf("hashes: %+v", f)
	}
	reloaded, f := store.Get(fourth.Meta.ID)
	if f != nil || reloaded.Meta.TreeHashes["skills/pdf"] != "sha256:abc" {
		t.Fatalf("hashes not persisted: %+v %v", f, reloaded.Meta.TreeHashes)
	}

	now = now.Add(StageTTL + time.Minute)
	if _, f := store.Get(third.Meta.ID); f == nil || f.Code != contract.CodeNotFound {
		t.Fatalf("expired stage = %+v", f)
	}
	if _, err := os.Stat(third.Dir); !os.IsNotExist(err) {
		t.Fatalf("expired stage directory still exists")
	}

	if f := store.Discard(fourth.Meta.ID); f != nil {
		t.Fatalf("discard: %+v", f)
	}
	if _, err := os.Stat(fourth.Dir); !os.IsNotExist(err) {
		t.Fatalf("discarded stage directory still exists")
	}
	if f := store.Discard("bad"); f == nil || f.Code != contract.CodeInvalidInput {
		t.Fatalf("bad id = %+v", f)
	}
	if !IsStageID("s-0123456789ab") || IsStageID("s-xyz") || IsStageID("x-0123456789ab") {
		t.Fatalf("IsStageID mismatch")
	}
}
