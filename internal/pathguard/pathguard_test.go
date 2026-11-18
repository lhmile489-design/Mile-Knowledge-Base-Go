package pathguard

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

func TestValidateRel(t *testing.T) {
	good := map[string]string{
		"":                "",
		"a.md":            "a.md",
		"a/b/c.md":        "a/b/c.md",
		`a\b\c.md`:        "a/b/c.md",
		"a//b.md":         "a/b.md",
		"skills/react.md": "skills/react.md",
		"中文/笔记.md":        "中文/笔记.md",
		"a b/c d.md":      "a b/c d.md",
		"README.md":       "README.md",
		// ".." inside a name is legal; only the segments "." and ".." are not
		// (FR-TREE). Found by FuzzValidateRel in T00.
		"..0":        "..0",
		"a..b.md":    "a..b.md",
		"notes..txt": "notes..txt",
	}
	for in, want := range good {
		got, f := ValidateRel(in)
		if f != nil {
			t.Errorf("ValidateRel(%q) failed: %+v", in, f)
			continue
		}
		if got != want {
			t.Errorf("ValidateRel(%q) = %q, want %q", in, got, want)
		}
	}

	bad := []string{
		"..",
		"../x",
		"a/../../b",
		"/abs/path",
		`\\server\share`,
		"//server/share",
		`C:\x`,
		"C:x",
		"a\x00b",
		"a<b",
		"a>b",
		"a:b",
		`a"b`,
		"a|b",
		"a?b",
		"a*b",
		"a\x01b",
		"a/b.",
		"a/ b",
		"a/con",
		"a/NUL.txt",
		"a/lpt9.log",
		strings.Repeat("a", MaxRelLen+1),
	}
	for _, in := range bad {
		if _, f := ValidateRel(in); f == nil {
			t.Errorf("ValidateRel(%q) accepted a bad path", in)
		}
	}
}

func TestValidateName(t *testing.T) {
	for _, ok := range []string{"笔记.md", "a b.md", "readme", "a.b.c.md"} {
		if f := ValidateName(ok); f != nil {
			t.Errorf("ValidateName(%q) = %+v", ok, f)
		}
	}
	for _, bad := range []string{"", ".", "..", "a/b", `a\b`, "con", "aux.md", "a.", " a", "a<b", strings.Repeat("名", 201)} {
		if f := ValidateName(bad); f == nil {
			t.Errorf("ValidateName(%q) accepted", bad)
		}
	}
}

func TestResolveContainment(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "skills", "react"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "skills", "react", "SKILL.md"), []byte("# x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, f := Resolve(root, "skills/react/SKILL.md")
	if f != nil {
		t.Fatalf("resolve existing: %+v", f)
	}
	if !Within(root, got) {
		t.Errorf("resolved path %q escapes %q", got, root)
	}

	// A create target that does not exist yet still resolves inside the root.
	got, f = Resolve(root, "skills/new.md")
	if f != nil {
		t.Fatalf("resolve new: %+v", f)
	}
	if !Within(root, got) {
		t.Errorf("new path %q escapes", got)
	}

	if _, f := Resolve(root, "../outside.md"); f == nil || f.Code != contract.CodePathEscape {
		t.Errorf("traversal not rejected: %+v", f)
	}
}

func TestResolveRejectsJunction(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	cmd := exec.Command("cmd", "/c", "mklink", "/J", link, outside)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("cannot create junction here: %v %s", err, out)
	}
	if _, f := Resolve(root, "link/secret.md"); f == nil {
		t.Fatal("junction inside the vault was not rejected")
	}
}

func TestWithin(t *testing.T) {
	root := `C:\vault`
	cases := map[string]bool{
		`C:\vault\a.md`:    true,
		`C:\vault`:         true,
		`c:\vault\a.md`:    true,
		`C:\vault2\a.md`:   true, // spelling-level: Rel gives ..\vault2; check below
		`C:\other\a.md`:    false,
		`C:\vault\..\x.md`: false,
	}
	for target, want := range cases {
		got := Within(root, target)
		if target == `C:\vault2\a.md` {
			// Rel(C:\vault, C:\vault2\a.md) = ..\vault2\a.md, so false is correct.
			want = false
		}
		if got != want {
			t.Errorf("Within(%q,%q) = %v, want %v", root, target, got, want)
		}
	}
}

func FuzzValidateRel(f *testing.F) {
	for _, seed := range []string{"a.md", "../x", "a/b", `C:\x`, "a\x00", "con", "中文/笔记.md", "..0"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, p string) {
		norm, fail := ValidateRel(p)
		if fail != nil {
			return
		}
		if strings.HasPrefix(norm, "/") || strings.ContainsRune(norm, 0) {
			t.Fatalf("ValidateRel accepted %q as %q", p, norm)
		}
		// The real invariant: no segment is "." or "..", so the result cannot
		// climb out of the vault. A name that merely contains ".." ("..0",
		// "a..b.md") is legal (FR-TREE).
		for _, seg := range strings.Split(norm, "/") {
			if seg == "." || seg == ".." {
				t.Fatalf("ValidateRel accepted %q as %q (segment %q)", p, norm, seg)
			}
		}
	})
}

func TestCanonicalRelExpandsCaseAndMissingTail(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "raws"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "raws", "a.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, f := CanonicalRel(root, "RAWS/A.MD")
	if f != nil {
		t.Fatalf("case alias: %+v", f)
	}
	if got != "raws/a.md" {
		t.Fatalf("case alias = %q", got)
	}

	got, f = CanonicalRel(root, "raws/new.md")
	if f != nil || got != "raws/new.md" {
		t.Fatalf("missing tail = %q %+v", got, f)
	}

	if _, f := CanonicalRel(root, "../x"); f == nil {
		t.Fatal("traversal accepted")
	}
}

func TestCanonicalRelExpandsShortName(t *testing.T) {
	root := t.TempDir()
	long := "verylongdirectoryname123456"
	if err := os.MkdirAll(filepath.Join(root, long, "settings"), 0o755); err != nil {
		t.Fatal(err)
	}
	short := shortDirName(t, root, long)
	if short == "" {
		t.Skip("this volume has no 8.3 short names")
	}
	got, f := CanonicalRel(root, short+"/settings/x.json")
	if f != nil {
		t.Fatalf("short name: %+v", f)
	}
	if got != long+"/settings/x.json" {
		t.Fatalf("short name = %q", got)
	}
}

// shortDirName reads the 8.3 alias of one root entry from dir /x.
func shortDirName(t *testing.T, root, long string) string {
	t.Helper()
	out, err := exec.Command("cmd", "/c", "dir", "/x", "/a", root).Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		hasLong := false
		for _, field := range fields {
			if strings.EqualFold(field, long) {
				hasLong = true
			}
		}
		if !hasLong {
			continue
		}
		for _, field := range fields {
			if strings.Contains(field, "~") && !strings.EqualFold(field, long) {
				return field
			}
		}
	}
	return ""
}
