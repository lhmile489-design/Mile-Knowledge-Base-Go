package skills

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/vault"
)

// M4 T01: reparse points, path guard and the safety regressions the milestone
// depends on (PRD 15.4; 技术设计 15.3; R-9 C6–C8).

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustJunction(t *testing.T, link, target string) {
	t.Helper()
	if err := CreateJunction(link, target); err != nil {
		t.Fatalf("CreateJunction: %v", err)
	}
}

func TestJunctionClassifyAndReadThrough(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "知识库技能")
	writeFile(t, filepath.Join(target, "SKILL.md"), "# vue\n")
	link := filepath.Join(root, "工具目录", "vue")

	mustJunction(t, link, target)

	kind, got, err := Classify(link)
	if err != nil || kind != KindJunction {
		t.Fatalf("Classify = %q, %v; want junction", kind, err)
	}
	if !strings.EqualFold(filepath.Clean(got), filepath.Clean(target)) {
		t.Fatalf("target = %q, want %q", got, target)
	}
	body, err := os.ReadFile(filepath.Join(link, "SKILL.md"))
	if err != nil || string(body) != "# vue\n" {
		t.Fatalf("read through link = %q, %v", body, err)
	}
}

func TestRemoveLinkKeepsTarget(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	writeFile(t, filepath.Join(target, "a.md"), "one")
	writeFile(t, filepath.Join(target, "nested", "b.md"), "two")
	link := filepath.Join(root, "link")
	mustJunction(t, link, target)

	before, err := TreeHash(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := RemoveLink(link); err != nil {
		t.Fatalf("RemoveLink: %v", err)
	}
	if kind, _, _ := Classify(link); kind != KindMissing {
		t.Fatalf("link still classified as %q", kind)
	}
	after, err := TreeHash(target)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("target changed: %s -> %s", before, after)
	}

	realDir := filepath.Join(root, "real")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := RemoveLink(realDir); err == nil {
		t.Fatal("RemoveLink accepted a real directory")
	}
	if _, err := os.Stat(realDir); err != nil {
		t.Fatalf("real directory was removed: %v", err)
	}
}

// R-9 C7/C8 regression: os.RemoveAll must not follow a junction.
func TestRemoveAllDoesNotFollowJunction(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "vault-skill")
	writeFile(t, filepath.Join(target, "SKILL.md"), "# keep me\n")
	parent := filepath.Join(root, "tool")
	writeFile(t, filepath.Join(parent, "own.md"), "own")
	link := filepath.Join(parent, "vue")
	mustJunction(t, link, target)

	before, err := TreeHash(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(parent); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}
	after, err := TreeHash(target)
	if err != nil {
		t.Fatalf("target gone after RemoveAll: %v", err)
	}
	if before != after {
		t.Fatalf("target changed: %s -> %s", before, after)
	}
}

func TestSafeWalkSkipsLinks(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "outside")
	writeFile(t, filepath.Join(target, "secret.md"), "no")
	tool := filepath.Join(root, "tool")
	writeFile(t, filepath.Join(tool, "a.md"), "a")
	writeFile(t, filepath.Join(tool, "sub", "b.md"), "b")
	mustJunction(t, filepath.Join(tool, "linked"), target)

	files, skipped, err := SafeWalk(tool)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("files = %v, want 2 regular files", files)
	}
	if len(skipped) != 1 || !strings.HasSuffix(skipped[0], "linked") {
		t.Fatalf("skipped = %v, want the junction", skipped)
	}
}

func TestCreateJunctionRefusesBadTargets(t *testing.T) {
	root := t.TempDir()
	if err := CreateJunction(filepath.Join(root, "l"), `\\server\share`); !errors.Is(err, ErrUNCTarget) {
		t.Fatalf("UNC target: %v, want ErrUNCTarget", err)
	}
	if err := CreateJunction(filepath.Join(root, "m"), filepath.Join(root, "missing")); err == nil {
		t.Fatal("missing target accepted")
	}
	file := filepath.Join(root, "f.md")
	writeFile(t, file, "x")
	if err := CreateJunction(filepath.Join(root, "n"), file); err == nil {
		t.Fatal("file target accepted")
	}
}

func TestUnderRoot(t *testing.T) {
	base := `C:\Users\x\vault`
	cases := []struct {
		target string
		want   bool
	}{
		{base, true},
		{base + `\skills\a`, true},
		{base + `2\skills\a`, false},
		{`\\?\` + base + `\skills`, true},
		{`C:\Users\x\vaulto`, false},
	}
	for _, c := range cases {
		if got := UnderRoot(c.target, base); got != c.want {
			t.Fatalf("UnderRoot(%q) = %v, want %v", c.target, got, c.want)
		}
	}
}

func TestResolveUnder(t *testing.T) {
	base := t.TempDir()
	writeFile(t, filepath.Join(base, "a", "b.md"), "x")
	if _, err := ResolveUnder(base, "a/b.md"); err != nil {
		t.Fatalf("valid path rejected: %v", err)
	}
	for _, bad := range []string{"", ".", "..", "../x", `a\..\..\x`, `C:\Windows`, `\\server\share`, "/abs"} {
		if _, err := ResolveUnder(base, bad); err == nil {
			t.Fatalf("ResolveUnder accepted %q", bad)
		}
	}
	if _, err := ResolveUnder(base, "a"); err != nil {
		t.Fatalf("single segment rejected: %v", err)
	}
	// A junction used as a parent must be refused (it could redirect writes).
	outside := t.TempDir()
	mustJunction(t, filepath.Join(base, "j"), outside)
	if _, err := ResolveUnder(base, "j/x"); !errors.Is(err, ErrPathEscape) {
		t.Fatalf("junction parent: %v, want ErrPathEscape", err)
	}
	// The final segment itself may be a link; callers inspect it.
	if _, err := ResolveUnder(base, "j"); err != nil {
		t.Fatalf("final link rejected: %v", err)
	}
}

func TestIsValidSegmentParity(t *testing.T) {
	cases := []string{
		"vue", "前端", "a-b_c", "", ".hidden", "a/b", `a\b`, "..", "CON", "a:b",
		strings.Repeat("长", 80), strings.Repeat("长", 81), "trailing.",
	}
	for _, name := range cases {
		if got, want := IsValidSegment(name), vault.IsSingleSegmentName(name); got != want {
			t.Fatalf("IsValidSegment(%q) = %v, vault says %v", name, got, want)
		}
	}
}

func TestTreeHashStableAndIgnoresLinks(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.md"), "one")
	writeFile(t, filepath.Join(root, "sub", "b.md"), "two")
	first, err := TreeHash(root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := TreeHash(root)
	if err != nil || first != second {
		t.Fatalf("hashes differ: %s vs %s (%v)", first, second, err)
	}
	writeFile(t, filepath.Join(root, "c.md"), "three")
	third, err := TreeHash(root)
	if err != nil || third == first {
		t.Fatalf("hash did not change after adding a file: %s", third)
	}
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "x.md"), "outside")
	mustJunction(t, filepath.Join(root, "linked"), outside)
	after, err := TreeHash(root)
	if err != nil || after != third {
		t.Fatalf("junction changed the tree hash: %s -> %s", third, after)
	}
}
