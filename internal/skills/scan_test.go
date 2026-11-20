package skills

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// M4 T03: two-level scanning, the six origin states, copy sync states and the
// scan cache (PRD FR-SKM-SCAN/DIST; 技术设计 15.4; 数据库设计 4.6).

func mustHash(t *testing.T, dir string) string {
	t.Helper()
	hash, err := TreeHash(dir)
	if err != nil {
		t.Fatal(err)
	}
	return hash
}

func originOf(skills []contract.ToolSkill, name string) contract.ToolSkill {
	for _, skill := range skills {
		if skill.Name == name {
			return skill
		}
	}
	return contract.ToolSkill{}
}

func TestScanSixOrigins(t *testing.T) {
	root := t.TempDir()
	vault := filepath.Join(root, "vault")
	kbSkill := filepath.Join(vault, "skills", "前端", "vue")
	writeFile(t, filepath.Join(kbSkill, "SKILL.md"), "---\nname: vue\ndescription: 组合式 API\n---\n\n# vue\n")
	external := filepath.Join(root, "external", "magpie-skill")
	writeFile(t, filepath.Join(external, "SKILL.md"), "# magpie\n")
	tool := filepath.Join(root, "tool")
	writeFile(t, filepath.Join(tool, "copied", "SKILL.md"), "# copied\n")
	writeFile(t, filepath.Join(tool, "independent", "SKILL.md"), "# independent\n")
	builtinRoot := filepath.Join(root, "builtin")
	writeFile(t, filepath.Join(builtinRoot, "official", "SKILL.md"), "# builtin\n")
	mustJunction(t, filepath.Join(tool, "linked"), kbSkill)
	mustJunction(t, filepath.Join(tool, "external"), external)
	brokenTarget := filepath.Join(root, "gone")
	writeFile(t, filepath.Join(brokenTarget, "SKILL.md"), "# gone\n")
	mustJunction(t, filepath.Join(tool, "broken"), brokenTarget)
	if err := os.RemoveAll(brokenTarget); err != nil {
		t.Fatal(err)
	}

	groups, warnings := ScanTools(ScanOptions{
		VaultID:   "v1",
		VaultRoot: vault,
		Tools: []DetectedTool{{
			ID: "demo", Status: StatusDetected,
			ScanDirs: []string{tool}, BuiltinDirs: []string{builtinRoot},
		}},
		Distributions: []Distribution{{
			ID: "d1", VaultID: "v1", ToolID: "demo",
			SkillPath: "skills/前端/vue", TargetPath: filepath.Join(tool, "copied"),
			Mode: "copy", Name: "vue", BaseHash: mustHash(t, filepath.Join(tool, "copied")),
		}},
	})
	if len(warnings) != 0 {
		t.Fatalf("warnings = %+v", warnings)
	}
	if len(groups) != 1 {
		t.Fatalf("groups = %d", len(groups))
	}
	skills := groups[0].Skills
	cases := map[string]string{
		"linked":      "linked",
		"broken":      "link-broken",
		"copied":      "copied",
		"independent": "independent",
		"official":    "builtin",
		"external":    "external-link",
	}
	for name, want := range cases {
		if got := originOf(skills, name).Origin; got != want {
			t.Fatalf("%s origin = %q, want %q (all: %+v)", name, got, want, skills)
		}
	}
	if got := originOf(skills, "linked").Description; got != "组合式 API" {
		t.Fatalf("frontmatter description = %q", got)
	}
	if !originOf(skills, "official").Builtin {
		t.Fatal("builtin flag not set")
	}
}

func TestCopySyncStates(t *testing.T) {
	root := t.TempDir()
	vault := filepath.Join(root, "vault")
	tool := filepath.Join(root, "tool")
	makePair := func(name, kvBody, copyBody string) (skillPath, target, base string) {
		skillPath = "skills/分类/" + name
		kb := filepath.Join(vault, filepath.FromSlash(skillPath))
		writeFile(t, filepath.Join(kb, "SKILL.md"), kvBody)
		target = filepath.Join(tool, name)
		writeFile(t, filepath.Join(target, "SKILL.md"), copyBody)
		return skillPath, target, mustHash(t, target)
	}
	pathInSync, targetInSync, baseInSync := makePair("in-sync", "same\n", "same\n")
	pathVault, targetVault, baseVault := makePair("vault-updated", "baseline\n", "baseline\n")
	writeFile(t, filepath.Join(vault, "skills", "分类", "vault-updated", "SKILL.md"), "changed\n")
	pathTool, targetTool, baseTool := makePair("tool-updated", "baseline\n", "baseline\n")
	writeFile(t, filepath.Join(targetTool, "SKILL.md"), "changed in tool\n")
	pathBoth, targetBoth, baseBoth := makePair("both-updated", "baseline\n", "baseline\n")
	writeFile(t, filepath.Join(vault, "skills", "分类", "both-updated", "SKILL.md"), "vault changed\n")
	writeFile(t, filepath.Join(targetBoth, "SKILL.md"), "tool changed\n")

	dists := []Distribution{
		{ID: "1", VaultID: "v1", ToolID: "demo", SkillPath: pathInSync, TargetPath: targetInSync, BaseHash: baseInSync},
		{ID: "2", VaultID: "v1", ToolID: "demo", SkillPath: pathVault, TargetPath: targetVault, BaseHash: baseVault},
		{ID: "3", VaultID: "v1", ToolID: "demo", SkillPath: pathTool, TargetPath: targetTool, BaseHash: baseTool},
		{ID: "4", VaultID: "v1", ToolID: "demo", SkillPath: pathBoth, TargetPath: targetBoth, BaseHash: baseBoth},
	}
	groups, _ := ScanTools(ScanOptions{
		VaultID: "v1", VaultRoot: vault,
		Tools:         []DetectedTool{{ID: "demo", Status: StatusDetected, ScanDirs: []string{tool}}},
		Distributions: dists,
	})
	want := map[string]string{
		"in-sync": "in-sync", "vault-updated": "vault-updated",
		"tool-updated": "tool-updated", "both-updated": "both-updated",
	}
	for name, state := range want {
		skill := originOf(groups[0].Skills, name)
		if skill.Origin != "copied" || skill.Sync != state {
			t.Fatalf("%s = origin %q sync %q, want copied/%s", name, skill.Origin, skill.Sync, state)
		}
	}
}

func TestScanTwoLevelsAndSkips(t *testing.T) {
	root := t.TempDir()
	tool := filepath.Join(root, "tool")
	writeFile(t, filepath.Join(tool, "group", "nested", "SKILL.md"), "# nested\n")
	writeFile(t, filepath.Join(tool, ".hidden", "SKILL.md"), "# hidden\n")
	writeFile(t, filepath.Join(tool, "node_modules", "pkg", "SKILL.md"), "# pkg\n")
	writeFile(t, filepath.Join(tool, "a", "b", "c", "SKILL.md"), "# too deep\n")
	groups, _ := ScanTools(ScanOptions{
		Tools: []DetectedTool{{ID: "demo", Status: StatusDetected, ScanDirs: []string{tool}}},
	})
	names := map[string]bool{}
	for _, skill := range groups[0].Skills {
		names[skill.Name] = true
	}
	if !names["nested"] {
		t.Fatalf("two-level skill missing: %+v", groups[0].Skills)
	}
	for _, skipped := range []string{"hidden", "pkg", "c"} {
		if names[skipped] {
			t.Fatalf("%s should have been skipped", skipped)
		}
	}
}

func TestExpandWildcard(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"alpha", "beta"} {
		writeFile(t, filepath.Join(root, "packages", name, "skills", "SKILL.md"), "# s\n")
	}
	dirs := ExpandWildcard(filepath.Join(root, "packages", "*", "skills"))
	if len(dirs) != 2 {
		t.Fatalf("expanded %d dirs: %v", len(dirs), dirs)
	}
}

func TestScanCacheRoundTripAndCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skills", "scan-cache.json")
	scan := contract.SkillScan{
		Kb: []contract.SkillCategory{{Category: "前端", Skills: []contract.KbSkill{{Name: "vue", Path: "skills/前端/vue"}}}},
		Tools: []contract.SkillToolScan{{ToolID: "demo", Skills: []contract.ToolSkill{{
			Name: "vue", Path: `C:\t\vue`, ToolID: "demo", Origin: "linked",
		}}}},
	}
	if err := SaveScanCache(path, scan, "2026-11-20T09:00:00Z"); err != nil {
		t.Fatal(err)
	}
	cache := LoadScanCache(path)
	if cache == nil || cache.CachedAt != "2026-11-20T09:00:00Z" || len(cache.Tools[0].Skills) != 1 {
		t.Fatalf("cache = %+v", cache)
	}
	if got := cache.ToScan(true); !got.Scanning || len(got.Kb) != 1 {
		t.Fatalf("ToScan = %+v", got)
	}

	writeFile(t, path, "{not json")
	if LoadScanCache(path) != nil {
		t.Fatal("corrupt cache was accepted")
	}
	writeFile(t, path, `{"version": 99, "cachedAt": "x"}`)
	if LoadScanCache(path) != nil {
		t.Fatal("version-mismatched cache was accepted")
	}
}

func TestScanCacheTrim(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scan-cache.json")
	skills := make([]contract.ToolSkill, scanCacheMaxEntries+1)
	for i := range skills {
		skills[i] = contract.ToolSkill{Name: "s", Path: "p", ToolID: "demo", Origin: "independent"}
	}
	scan := contract.SkillScan{Tools: []contract.SkillToolScan{{ToolID: "demo", Skills: skills}}}
	if err := SaveScanCache(path, scan, "2026-11-20T09:00:00Z"); err != nil {
		t.Fatal(err)
	}
	cache := LoadScanCache(path)
	if cache == nil || len(cache.Tools[0].Skills) != scanCacheMaxEntries {
		t.Fatalf("trimmed cache = %+v", cache)
	}
	if len(cache.Warnings) == 0 {
		t.Fatal("trim warning missing")
	}
}

// T17 审查：builtin 目录里的链接同样是内置技能；两层技能带可解析的 base/rel。
func TestBuiltinLinksAndTwoLevelRel(t *testing.T) {
	root := t.TempDir()
	builtin := filepath.Join(root, "builtin")
	writeFile(t, filepath.Join(builtin, "official", "SKILL.md"), "# builtin\n")
	outside := filepath.Join(root, "outside")
	writeFile(t, filepath.Join(outside, "SKILL.md"), "# outside\n")
	mustJunction(t, filepath.Join(builtin, "linked-builtin"), outside)
	tool := filepath.Join(root, "tool")
	writeFile(t, filepath.Join(tool, "group", "nested", "SKILL.md"), "# nested\n")
	groups, _ := ScanTools(ScanOptions{
		Tools: []DetectedTool{{ID: "demo", Status: StatusDetected, ScanDirs: []string{tool}, BuiltinDirs: []string{builtin}}},
	})
	byName := map[string]contract.ToolSkill{}
	for _, skill := range groups[0].Skills {
		byName[skill.Name] = skill
	}
	if skill := byName["linked-builtin"]; skill.Origin != "builtin" || !skill.Builtin {
		t.Fatalf("builtin link = %+v, want builtin origin", skill)
	}
	if skill := byName["nested"]; skill.Rel != "group/nested" || skill.Base != "scan:0" {
		t.Fatalf("nested rel/base = %q/%q", skill.Rel, skill.Base)
	}
	if skill := byName["official"]; skill.Base != "builtin:0" || skill.Rel != "official" {
		t.Fatalf("builtin rel/base = %q/%q", skill.Rel, skill.Base)
	}
}

// T17 审查：知识库技能被删或改名后，拷贝不应再显示为「知识库有更新」。
func TestCopySyncVaultMissing(t *testing.T) {
	root := t.TempDir()
	vault := filepath.Join(root, "vault")
	toolDir := filepath.Join(root, "tool", "copied")
	writeFile(t, filepath.Join(toolDir, "SKILL.md"), "# copied\n")
	dist := Distribution{SkillPath: "skills/分类/gone", TargetPath: toolDir, BaseHash: mustHash(t, toolDir)}
	if got := CopySyncState(vault, dist, toolDir); got != "vault-missing" {
		t.Fatalf("sync = %q, want vault-missing", got)
	}
	if got := CopySyncState("", dist, toolDir); got != "" {
		t.Fatalf("sync without vault = %q, want empty", got)
	}
}
