package skills

import (
	"path/filepath"
	"testing"
)

// M6 T03: custom tools and projects join detection and scanning (技术设计
// 17.4, API §14.1; V-SCAN-1/2).

func scanTargetRecord(id, kind, name, path, installRel, createdAt string) Target {
	return Target{ID: id, Kind: kind, Name: name, Path: path, InstallRel: installRel, CreatedAt: createdAt}
}

func TestMergeTargets(t *testing.T) {
	root := t.TempDir()
	custom := newDir(t, filepath.Join(root, "custom"))
	project := newDir(t, filepath.Join(root, "project"))
	newDir(t, filepath.Join(project, ".claude", "skills"))
	newDir(t, filepath.Join(project, ".kiro", "skills"))
	gone := filepath.Join(root, "gone")

	targets := []Target{
		scanTargetRecord("t-222222222222", TargetProject, "Proj", project, ".kiro/skills", "2026-11-22T10:00:00Z"),
		scanTargetRecord("t-111111111111", TargetCustom, "Droid", custom, "", "2026-11-22T09:00:00Z"),
		scanTargetRecord("t-333333333333", TargetCustom, "Gone", gone, "", "2026-11-21T09:00:00Z"),
	}
	builtin := []DetectedTool{{ID: "claude-code", Kind: "builtin", Status: StatusDetected}}
	merged := MergeTargets(builtin, targets)

	if len(merged) != 4 || merged[0].ID != "claude-code" {
		t.Fatalf("merged = %+v", merged)
	}
	// Custom tools first (creation order), then projects.
	if merged[1].ID != "t-333333333333" || merged[2].ID != "t-111111111111" || merged[3].ID != "t-222222222222" {
		t.Fatalf("order = %s, %s, %s", merged[1].ID, merged[2].ID, merged[3].ID)
	}
	if len(builtin) != 1 || builtin[0].ProjectDirs != nil {
		t.Fatal("MergeTargets must not mutate its input")
	}

	droid := merged[2]
	if droid.Kind != TargetCustom || droid.InstallDir != custom || droid.Missing || droid.Status != StatusDetected {
		t.Fatalf("custom entry = %+v", droid)
	}
	if merged[1].Missing != true {
		t.Fatal("a gone custom dir must be marked missing")
	}

	proj := merged[3]
	if proj.Kind != TargetProject || proj.IconKey != "folder" || proj.Missing {
		t.Fatalf("project entry = %+v", proj)
	}
	if proj.InstallDir != filepath.Join(project, ".kiro", "skills") {
		t.Fatalf("project install dir = %q", proj.InstallDir)
	}
	rels := []string{}
	for _, pd := range proj.ProjectDirs {
		rels = append(rels, pd.Rel)
	}
	wantRels := []string{".claude/skills", ".kiro/skills"}
	if len(rels) != 2 || rels[0] != wantRels[0] || rels[1] != wantRels[1] {
		t.Fatalf("project dirs = %v, want %v (registry order, existing only)", rels, wantRels)
	}
}

func TestScanProjectAndCustomGroups(t *testing.T) {
	root := t.TempDir()
	project := newDir(t, filepath.Join(root, "project"))
	writeFile(t, filepath.Join(project, ".claude", "skills", "x", "SKILL.md"), "---\nname: x\ndescription: X skill\n---\n")
	writeFile(t, filepath.Join(project, ".agents", "skills", "y", "SKILL.md"), "# y\n")
	copied := filepath.Join(project, ".claude", "skills", "w")
	writeFile(t, filepath.Join(copied, "SKILL.md"), "# w\n")
	newDir(t, filepath.Join(project, "docs", "nested")) // not a conventional dir
	writeFile(t, filepath.Join(project, "docs", "nested", "SKILL.md"), "# not scanned\n")
	custom := newDir(t, filepath.Join(root, "custom"))
	writeFile(t, filepath.Join(custom, "z", "SKILL.md"), "# z\n")

	projectTarget := scanTargetRecord("t-222222222222", TargetProject, "Proj", project, ".claude/skills", "2026-11-22T10:00:00Z")
	customTarget := scanTargetRecord("t-111111111111", TargetCustom, "Droid", custom, "", "2026-11-22T09:00:00Z")
	tools := MergeTargets(nil, []Target{projectTarget, customTarget})

	kv := KbSkillRef{Category: "前端", Name: "vue", Path: "skills/前端/vue"}
	groups, warnings := ScanTools(ScanOptions{
		Tools:  tools,
		KbRefs: []KbSkillRef{kv},
		Distributions: []Distribution{{
			ID: "d1", ToolID: projectTarget.ID, SkillPath: "skills/前端/vue",
			TargetPath: copied, Mode: "copy", Name: "vue",
			BaseHash: mustHash(t, copied),
		}},
	})
	if len(warnings) != 0 {
		t.Fatalf("warnings = %+v", warnings)
	}
	if len(groups) != 2 {
		t.Fatalf("groups = %d", len(groups))
	}
	if groups[0].ToolID != customTarget.ID || groups[1].ToolID != projectTarget.ID {
		t.Fatalf("group order = %s, %s", groups[0].ToolID, groups[1].ToolID)
	}

	if len(groups[0].Skills) != 1 || groups[0].Skills[0].Name != "z" || groups[0].Skills[0].Base != "install" {
		t.Fatalf("custom group = %+v", groups[0].Skills)
	}
	if groups[0].Skills[0].Subdir != "" || groups[0].Skills[0].SourceToolID != "" {
		t.Fatal("custom rows must not carry project fields")
	}

	projSkills := groups[1].Skills
	if len(projSkills) != 3 {
		t.Fatalf("project skills = %+v", projSkills)
	}
	byName := map[string]int{}
	for i, skill := range projSkills {
		byName[skill.Name] = i
	}
	for name, want := range map[string]struct{ source, subdir, base string }{
		"x": {"claude-code", ".claude/skills", "project:.claude/skills"},
		"y": {"agents", ".agents/skills", "project:.agents/skills"},
		"w": {"claude-code", ".claude/skills", "project:.claude/skills"},
	} {
		skill := projSkills[byName[name]]
		if skill.SourceToolID != want.source || skill.Subdir != want.subdir || skill.Base != want.base {
			t.Fatalf("%s = source %q subdir %q base %q", name, skill.SourceToolID, skill.Subdir, skill.Base)
		}
		if skill.ToolID != projectTarget.ID {
			t.Fatalf("%s tool = %q", name, skill.ToolID)
		}
	}
	if got := projSkills[byName["w"]]; got.Origin != "copied" {
		t.Fatalf("distribution by target id: origin = %q", got.Origin)
	}
	if _, ok := byName["nested"]; ok {
		t.Fatal("non-conventional project directories must not be scanned")
	}
}

func TestScanMissingTargetGroups(t *testing.T) {
	root := t.TempDir()
	targets := []Target{
		scanTargetRecord("t-111111111111", TargetCustom, "Gone", filepath.Join(root, "gone-custom"), "", "2026-11-22T09:00:00Z"),
		scanTargetRecord("t-222222222222", TargetProject, "GoneProj", filepath.Join(root, "gone-project"), ".claude/skills", "2026-11-22T10:00:00Z"),
	}
	groups, warnings := ScanTools(ScanOptions{Tools: MergeTargets(nil, targets)})
	if len(warnings) != 0 {
		t.Fatalf("warnings = %+v", warnings)
	}
	if len(groups) != 2 || len(groups[0].Skills) != 0 || len(groups[1].Skills) != 0 {
		t.Fatalf("missing targets produced groups %+v", groups)
	}
}
