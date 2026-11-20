package skills

import (
	"encoding/json"
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

// M4 T02: the built-in registry and the three detection rules (技术设计 13.2/13.3).

func TestRegistryShape(t *testing.T) {
	if len(Registry) != 25 {
		t.Fatalf("registry has %d tools, want 25", len(Registry))
	}
	seen := map[string]bool{}
	for _, tool := range Registry {
		if tool.ID == "" || seen[tool.ID] {
			t.Fatalf("duplicate or empty id %q", tool.ID)
		}
		seen[tool.ID] = true
		if tool.Name == "" {
			t.Fatalf("%s has no name", tool.ID)
		}
		if !tool.Shared && tool.InstallDir == "" && len(tool.ScanDirs) == 0 &&
			len(tool.ScanRelative) == 0 && len(tool.BuiltinDirs) == 0 && len(tool.AppBuiltinRelative) == 0 {
			t.Fatalf("%s has no directory at all", tool.ID)
		}
		for _, reader := range tool.Readers {
			if _, ok := ToolByID(reader); !ok {
				t.Fatalf("%s lists unknown reader %q", tool.ID, reader)
			}
		}
	}
	shared, ok := ToolByID("agents")
	if !ok || !shared.Shared || len(shared.Readers) != 15 {
		t.Fatalf("shared tool readers = %d, want 15", len(shared.Readers))
	}
	if trae, _ := ToolByID("trae"); trae.Official {
		t.Fatal("trae should be marked unofficial")
	}
	if zcode, _ := ToolByID("zcode"); zcode.Official {
		t.Fatal("zcode should be marked unofficial")
	}
	if minimax, _ := ToolByID("minimax-code"); minimax.Enabled {
		t.Fatal("minimax-code must default to disabled")
	}
}

func detectByID(tools []DetectedTool, id string) DetectedTool {
	for _, tool := range tools {
		if tool.ID == id {
			return tool
		}
	}
	return DetectedTool{}
}

func TestDetectEvidenceAndStates(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	programFiles := filepath.Join(root, "Programs")
	writeFile(t, filepath.Join(home, ".local", "bin", "claude.cmd"), "@echo off")
	writeFile(t, filepath.Join(home, ".trae", "skills", "x", "SKILL.md"), "# x")
	writeFile(t, filepath.Join(programFiles, "DeepSeek Harness", "DeepSeek Harness.exe"), "MZ")
	writeFile(t, filepath.Join(home, ".zcode", "skills", "y", "SKILL.md"), "# y")
	writeFile(t, filepath.Join(programFiles, "ZCode", "ZCode.exe"), "MZ")
	writeFile(t, filepath.Join(programFiles, "ZCode", "resources", "glm", "packages", "pkg", "skills", "SKILL.md"), "# builtin")

	detector := &Detector{
		Home: home, LocalAppData: filepath.Join(home, "AppData", "Local"),
		UserProgramFiles: programFiles,
		CLIDirs:          []string{filepath.Join(home, ".local", "bin")},
		PathDirs:         []string{filepath.Join(home, "bin")},
		Uninstall:        []UninstallEntry{{DisplayName: "Kiro 0.9.1", InstallLocation: `D:\Kiro`}},
	}
	tools := detector.Detect()

	if got := detectByID(tools, "claude-code").Status; got != StatusDetected {
		t.Fatalf("claude-code = %q, want detected via CLI", got)
	}
	if got := detectByID(tools, "kiro").Status; got != StatusDetected {
		t.Fatalf("kiro = %q, want detected via uninstall entry", got)
	}
	if got := detectByID(tools, "deepseek-harness").Status; got != StatusDetected {
		t.Fatalf("deepseek = %q, want detected via known exe", got)
	}
	if got := detectByID(tools, "trae").Status; got != StatusLeftover {
		t.Fatalf("trae = %q, want leftover (skills dir only)", got)
	}
	if got := detectByID(tools, "windsurf").Status; got != StatusMissing {
		t.Fatalf("windsurf = %q, want missing", got)
	}
	if got := detectByID(tools, "agents").Status; got != StatusDetected {
		t.Fatalf("shared = %q, want detected", got)
	}
	zcode := detectByID(tools, "zcode")
	if zcode.Status != StatusDetected || len(zcode.BuiltinDirs) == 0 {
		t.Fatalf("zcode = %+v, want detected with builtin dirs", zcode)
	}
	found := false
	for _, dir := range zcode.BuiltinDirs {
		if strings.Contains(dir, filepath.Join("packages", "pkg", "skills")) {
			found = true
		}
	}
	if !found {
		t.Fatalf("zcode builtin dirs = %v", zcode.BuiltinDirs)
	}
}

func TestDetectCLIExtensions(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"claude.exe", "codex.cmd", "gemini.bat", "qoder.ps1", "pi"} {
		writeFile(t, filepath.Join(root, name), "x")
	}
	detector := &Detector{Home: root, CLIDirs: []string{root}}
	tools := detector.Detect()
	for _, id := range []string{"claude-code", "codex", "gemini-cli", "qoder", "pi"} {
		if got := detectByID(tools, id).Status; got != StatusDetected {
			t.Fatalf("%s = %q, want detected", id, got)
		}
	}
}

func TestExpandPath(t *testing.T) {
	detector := &Detector{Home: `C:\Users\me`, LocalAppData: `C:\Users\me\AppData\Local`}
	cases := map[string]string{
		"~/.claude/skills":             `C:\Users\me\.claude\skills`,
		"~/.agents/skills":             `C:\Users\me\.agents\skills`,
		"%LOCALAPPDATA%/hermes/skills": `C:\Users\me\AppData\Local\hermes\skills`,
		"":                             "",
	}
	for in, want := range cases {
		if got := detector.ExpandPath(in); got != want {
			t.Fatalf("ExpandPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFakeDetectorFromEnv(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	writeFile(t, filepath.Join(home, ".local", "bin", "claude.cmd"), "x")
	registryFile := filepath.Join(root, "registry.json")
	body, err := json.Marshal(map[string]any{
		"userProgramFiles": filepath.Join(root, "Programs"),
		"uninstall":        []UninstallEntry{{DisplayName: "Cursor 1.0"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, registryFile, string(body))
	t.Setenv("SKILLS_FAKE_HOME", home)
	t.Setenv("SKILLS_FAKE_PATH", filepath.Join(home, "bin"))
	t.Setenv("SKILLS_FAKE_REGISTRY", registryFile)

	detector := newFakeDetector()
	if detector.Home != home {
		t.Fatalf("home = %q", detector.Home)
	}
	if got := detectByID(detector.Detect(), "cursor").Status; got != StatusDetected {
		t.Fatalf("cursor = %q, want detected via fake registry", got)
	}
	if got := detectByID(detector.Detect(), "claude-code").Status; got != StatusDetected {
		t.Fatalf("claude = %q, want detected via fake CLI dir", got)
	}
}
