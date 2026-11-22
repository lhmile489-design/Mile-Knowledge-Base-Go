package skills

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// M6 T02: targets.json and the registration rules (技术设计 17.3, V-TGT-1–4).

func targetEnv(t *testing.T) (TargetEnv, string) {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(filepath.Join(home, ".claude", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	return TargetEnv{Home: home, LocalAppData: filepath.Join(root, "Local")}, root
}

func newDir(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func wantCode(t *testing.T, f *contract.Failure, code string) {
	t.Helper()
	if f == nil || f.Code != code {
		t.Fatalf("failure = %+v, want code %q", f, code)
	}
}

func targetRecord(id, kind, name, path, installRel string) Target {
	return Target{
		ID: id, Kind: kind, Name: name, Path: path, InstallRel: installRel,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

func TestTargetNameRules(t *testing.T) {
	_, root := targetEnv(t)
	custom := targetRecord("t-aaaaaaaaaaaa", TargetCustom, "Droid", newDir(t, filepath.Join(root, "droid")), "")

	cases := []struct {
		name string
		want string // "" = ok
	}{
		{"", "invalid-input"},
		{"a/b", "invalid-input"},
		{".hidden", "invalid-input"},
		{"CON", "invalid-input"},
		{"trailing ", "invalid-input"},
		{"name.", "invalid-input"},
		{"Claude Code", "invalid-input"}, // built-in display name
		{"droid", "invalid-input"},       // case-insensitive duplicate
		{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "invalid-input"},
		{"我的工具", ""},
		{"Droid Two", ""},
	}
	for _, test := range cases {
		f := ValidateTargetName(test.name, []Target{custom}, "")
		if test.want == "" {
			if f != nil {
				t.Fatalf("name %q: unexpected failure %+v", test.name, f)
			}
			continue
		}
		wantCode(t, f, test.want)
	}
	// Editing keeps its own name.
	if f := ValidateTargetName("droid", []Target{custom}, custom.ID); f != nil {
		t.Fatalf("edit kept name: %+v", f)
	}
}

func TestTargetPathRules(t *testing.T) {
	env, root := targetEnv(t)
	customDir := newDir(t, filepath.Join(root, "droid"))

	if f := ValidateTargetPath(env, TargetCustom, "", "", ""); f == nil || f.Code != contract.CodeInvalidInput {
		t.Fatalf("empty path: %+v", f)
	}
	wantCode(t, ValidateTargetPath(env, TargetCustom, `\\server\share\skills`, "", ""), contract.CodeInvalidInput)
	wantCode(t, ValidateTargetPath(env, TargetCustom, `\\?\C:\skills`, "", ""), contract.CodeInvalidInput)
	wantCode(t, ValidateTargetPath(env, TargetCustom, `relative\dir`, "", ""), contract.CodeInvalidInput)
	wantCode(t, ValidateTargetPath(env, TargetCustom, filepath.Join(root, "missing"), "", ""), contract.CodeInvalidInput)

	file := filepath.Join(root, "file.txt")
	writeFile(t, file, "x")
	wantCode(t, ValidateTargetPath(env, TargetCustom, file, "", ""), contract.CodeInvalidInput)

	link := filepath.Join(root, "linked")
	mustJunction(t, link, customDir)
	wantCode(t, ValidateTargetPath(env, TargetCustom, link, "", ""), contract.CodeInvalidInput)

	if f := ValidateTargetPath(env, TargetCustom, customDir, "", ""); f != nil {
		t.Fatalf("plain dir rejected: %+v", f)
	}
}

func TestTargetPathOverlaps(t *testing.T) {
	env, root := targetEnv(t)
	vault := newDir(t, filepath.Join(root, "vault"))
	env.VaultRoots = []string{vault}

	// Rule 3: inside the vault, or containing it, is path-escape.
	wantCode(t, ValidateTargetPath(env, TargetCustom, newDir(t, filepath.Join(vault, "skills")), "", ""), contract.CodePathEscape)
	wantCode(t, ValidateTargetPath(env, TargetCustom, filepath.Join(vault, "skills"), "", ""), contract.CodePathEscape)
	wantCode(t, ValidateTargetPath(env, TargetCustom, root, "", ""), contract.CodePathEscape)

	// A project may contain a vault, but not scan into it.
	project := newDir(t, filepath.Join(root, "project"))
	newDir(t, filepath.Join(project, ".claude", "skills"))
	nestedVault := newDir(t, filepath.Join(project, ".agents", "skills"))
	env.VaultRoots = []string{nestedVault}
	wantCode(t, ValidateTargetPath(env, TargetProject, project, ".claude/skills", ""), contract.CodePathEscape)
	env.VaultRoots = nil
	if f := ValidateTargetPath(env, TargetProject, project, ".claude/skills", ""); f != nil {
		t.Fatalf("project with own subdirs rejected: %+v", f)
	}

	// rule 1 of installRel: must be a conventional directory
	wantCode(t, ValidateTargetPath(env, TargetProject, project, "elsewhere/skills", ""), contract.CodeInvalidInput)
	wantCode(t, ValidateTargetPath(env, TargetProject, project, "", ""), contract.CodeInvalidInput)

	// Rule 4: built-in tool directories (expanded through the fake home).
	insideBuiltin := newDir(t, filepath.Join(env.Home, ".claude", "skills", "x"))
	env2 := env
	wantCode(t, ValidateTargetPath(env2, TargetCustom, insideBuiltin, "", ""), contract.CodeSkillConflict)
	wantCode(t, ValidateTargetPath(env2, TargetCustom, filepath.Join(env.Home, ".claude"), "", ""), contract.CodeSkillConflict)
	wantCode(t, ValidateTargetPath(env2, TargetCustom, env.Home, "", ""), contract.CodeSkillConflict)

	// Rule 5: another target.
	custom := targetRecord("t-bbbbbbbbbbbb", TargetCustom, "Droid", newDir(t, filepath.Join(project, "vendor", "skills")), "")
	env3 := env
	env3.Targets = []Target{custom}
	wantCode(t, ValidateTargetPath(env3, TargetCustom, newDir(t, filepath.Join(project, "vendor", "skills", "inner")), "", ""), contract.CodeSkillConflict)
	wantCode(t, ValidateTargetPath(env3, TargetProject, project, ".claude/skills", ""), contract.CodeSkillConflict) // project root contains the custom dir
	// A project's own subdirectory cannot be registered on its own either.
	env4 := env
	env4.Targets = []Target{targetRecord("t-cccccccccccc", TargetProject, "Proj", project, ".claude/skills")}
	wantCode(t, ValidateTargetPath(env4, TargetCustom, filepath.Join(project, ".claude", "skills"), "", ""), contract.CodeSkillConflict)
	wantCode(t, ValidateTargetPath(env4, TargetCustom, filepath.Join(project, "vendor", "skills"), "", ""), contract.CodeSkillConflict)

	// Editing a record skips itself.
	env5 := env
	env5.Targets = []Target{custom}
	if f := ValidateTargetPath(env5, TargetCustom, custom.Path, "", custom.ID); f != nil {
		t.Fatalf("edit keeping its own dir: %+v", f)
	}
	// The cap counts other records only.
	many := make([]Target, 0, targetsMax)
	for i := 0; i < targetsMax; i++ {
		many = append(many, targetRecord("t-dddddddddddd", TargetCustom, "x", custom.Path, ""))
	}
	env6 := env
	env6.Targets = many
	wantCode(t, ValidateTargetPath(env6, TargetCustom, custom.Path, "", ""), contract.CodeInvalidInput)
	if f := ValidateTargetPath(env6, TargetCustom, custom.Path, "", "t-dddddddddddd"); f != nil {
		t.Fatalf("edit at the cap: %+v", f)
	}
}

func TestDefaultInstallRel(t *testing.T) {
	root := t.TempDir()
	if got := DefaultInstallRel(root); got != ".claude/skills" {
		t.Fatalf("empty project default = %q", got)
	}
	newDir(t, filepath.Join(root, ".kiro", "skills"))
	newDir(t, filepath.Join(root, ".agents", "skills"))
	// Registry order decides: .agents/skills comes before .kiro/skills.
	if got := DefaultInstallRel(root); got != ".agents/skills" {
		t.Fatalf("default = %q, want .agents/skills", got)
	}
}

func TestTargetsStoreRoundTrip(t *testing.T) {
	path := TargetsPath(t.TempDir())
	root := t.TempDir()
	records := []Target{
		targetRecord("t-111111111111", TargetCustom, "Droid", newDir(t, filepath.Join(root, "droid")), ""),
		targetRecord("t-222222222222", TargetProject, "Proj", newDir(t, filepath.Join(root, "proj")), ".claude/skills"),
	}
	if err := SaveTargets(path, records); err != nil {
		t.Fatal(err)
	}
	got := LoadTargets(path)
	if len(got.Warnings) != 0 || len(got.Targets) != 2 {
		t.Fatalf("load = %+v", got)
	}
	if got.Targets[1].InstallRel != ".claude/skills" || got.Targets[0].Name != "Droid" {
		t.Fatalf("records = %+v", got.Targets)
	}
}

func TestTargetsLoadDropsBadRecords(t *testing.T) {
	path := TargetsPath(t.TempDir())
	root := t.TempDir()
	good := targetRecord("t-333333333333", TargetCustom, "Droid", newDir(t, filepath.Join(root, "droid")), "")
	overlap := targetRecord("t-444444444444", TargetCustom, "Inner", newDir(t, filepath.Join(root, "droid", "inner")), "")
	badID := targetRecord("x-555555555555", TargetCustom, "Bad", newDir(t, filepath.Join(root, "bad1")), "")
	badKind := targetRecord("t-666666666666", "tool", "Bad", newDir(t, filepath.Join(root, "bad2")), "")
	badName := targetRecord("t-777777777777", TargetCustom, "", newDir(t, filepath.Join(root, "bad3")), "")
	badRel := targetRecord("t-888888888888", TargetProject, "Bad", newDir(t, filepath.Join(root, "bad4")), ".nope/skills")
	missingDir := targetRecord("t-999999999999", TargetCustom, "Gone", filepath.Join(root, "gradle"), "")

	body := `{"version":1,"targets":[` +
		jsonRecord(good) + `,` + jsonRecord(overlap) + `,` + jsonRecord(badID) + `,` +
		jsonRecord(badKind) + `,` + jsonRecord(badName) + `,` + jsonRecord(badRel) + `,` +
		jsonRecord(missingDir) + `]}`
	writeFile(t, path, body)
	got := LoadTargets(path)
	if len(got.Targets) != 2 {
		t.Fatalf("kept %d records, want 2: %+v", len(got.Targets), got)
	}
	if len(got.Warnings) != 5 {
		t.Fatalf("warnings = %v, want 5", got.Warnings)
	}
	// A registered directory that no longer exists keeps its record.
	if got.Targets[1].Name != "Gone" {
		t.Fatalf("missing-dir record dropped: %+v", got.Targets)
	}
}

func TestTargetsLoadRenamesCorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "targets.json")
	writeFile(t, path, "{not json")
	got := LoadTargets(path)
	if len(got.Targets) != 0 {
		t.Fatalf("corrupt file produced records: %+v", got)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range entries {
		if len(entry.Name()) > len("targets.json.corrupt-") && entry.Name()[:len("targets.json.corrupt-")] == "targets.json.corrupt-" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no .corrupt- backup in %v", entries)
	}

	// Unknown version is treated the same way.
	path2 := filepath.Join(t.TempDir(), "targets.json")
	writeFile(t, path2, `{"version":99,"targets":[]}`)
	if got := LoadTargets(path2); len(got.Targets) != 0 {
		t.Fatalf("version 99 produced records")
	}
}

func jsonRecord(record Target) string {
	body, err := json.Marshal(record)
	if err != nil {
		panic(err)
	}
	return string(body)
}

func TestTargetPathDetectedBuiltinDirs(t *testing.T) {
	env, root := targetEnv(t)
	appDir := newDir(t, filepath.Join(root, "Programs", "ZCode", "resources", "glm", "packages", "1", "skills"))
	env.BuiltinDirs = []BuiltinDirRef{{Tool: "ZCode", Dir: appDir}}
	inside := newDir(t, filepath.Join(appDir, "official"))
	wantCode(t, ValidateTargetPath(env, TargetCustom, inside, "", ""), contract.CodeSkillConflict)
	if f := ValidateTargetPath(env, TargetCustom, newDir(t, filepath.Join(root, "plain")), "", ""); f != nil {
		t.Fatalf("plain dir rejected: %+v", f)
	}
}
