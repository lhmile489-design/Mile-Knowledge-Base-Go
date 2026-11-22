package skills

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// M7 T03: the restore judgment table (技术设计 18.5, V-JUDGE-1).

func restoreFacts() RestoreFacts {
	return RestoreFacts{
		Resolve: func(target ManifestTarget) (RestoreDestination, string) {
			switch target.Kind {
			case "builtin":
				if target.ToolID == "gone" {
					return RestoreDestination{}, "工具在本机未检测到"
				}
				return RestoreDestination{ToolID: target.ToolID, Label: "Claude Code", Dir: "/home/u/.claude/skills"}, ""
			case "custom":
				if target.Name == "Droid" {
					return RestoreDestination{}, "本机没有登记「Droid」"
				}
				return RestoreDestination{ToolID: "t-aaaaaaaaaaaa", Label: target.Name, Dir: "/home/u/droid"}, ""
			}
			return RestoreDestination{
				ToolID: "t-bbbbbbbbbbbb",
				Label:  "项目 " + target.Name + " · " + target.Subdir,
				Dir:    "/home/u/proj/" + target.Subdir,
			}, ""
		},
		SkillDir: func(skillPath string) string {
			if strings.Contains(skillPath, "missing") {
				return ""
			}
			return "/vault/" + skillPath
		},
		DirExists: func(dir string) bool {
			switch dir {
			case filepath.Join("/home/u/.claude/skills", "copied"),
				filepath.Join("/home/u/.claude/skills", "unknown-dir"),
				filepath.Join("/home/u/.claude/skills", "linked"),
				filepath.Join("/home/u/.claude/skills", "tracked"):
				return true
			}
			return false
		},
		LinkTarget: func(dir string) (string, bool) {
			if dir == filepath.Join("/home/u/.claude/skills", "linked") {
				return "/vault/skills/前端/linked", true
			}
			return "", false
		},
		SourceAt: func(dir string) (ToolSource, bool) {
			if dir == filepath.Join("/home/u/.claude/skills", "tracked") {
				return ToolSource{Owner: "acme", Repo: "skills", Path: "skills/tracked"}, true
			}
			return ToolSource{}, false
		},
		Distributions: []Distribution{
			{VaultID: "v1", SkillPath: "skills/前端/copied", ToolID: "claude-code", TargetPath: "/home/u/.claude/skills/copied", Mode: "copy", Name: "copied"},
			{VaultID: "v2", SkillPath: "skills/前端/other", ToolID: "claude-code", TargetPath: "/home/u/.claude/skills/other", Mode: "link", Name: "other"},
		},
		SourceRecords: []ToolSource{
			{ToolID: "claude-code", TargetPath: "/home/u/.claude/skills/tracked", Owner: "acme", Repo: "skills", Path: "skills/tracked"},
		},
		VaultID: "v1",
		IsBuiltinDir: func(toolID, dir string) bool {
			return toolID == "claude-code" && strings.HasPrefix(dir, filepath.Join("/home/u/.claude", "builtin_skills"))
		},
		LabelFor: func(toolID string) string {
			if toolID == "claude-code" {
				return "Claude Code"
			}
			return toolID
		},
	}
}

func kbItem(name string) ManifestItem {
	return ManifestItem{
		Source: "kb", Target: ManifestTarget{Kind: "builtin", ToolID: "claude-code"}, Name: name,
		SkillPath: "skills/前端/" + name, Mode: "link", UpdatedAt: "2026-11-22T09:00:00Z",
	}
}

func githubItem(name string) ManifestItem {
	return ManifestItem{
		Source: "github", Target: ManifestTarget{Kind: "builtin", ToolID: "claude-code"}, Name: name,
		Owner: "acme", Repo: "skills", Ref: "main", Path: "skills/" + name,
		Commit: strings.Repeat("a", 40), PathCommit: strings.Repeat("b", 40),
		UpdatedAt: "2026-11-22T09:00:00Z",
	}
}

func judge(items ...ManifestItem) RestoreJudgment {
	return JudgeManifest(Manifest{Status: ManifestOK, Items: items}, restoreFacts())
}

func TestJudgeManifestCreateAndNetwork(t *testing.T) {
	out := judge(kbItem("fresh")).Items
	if len(out) != 1 {
		t.Fatalf("items = %d", len(out))
	}
	item := out[0]
	if item.State != "create" || !item.Selectable || !item.DefaultSelected || item.NeedsNetwork {
		t.Fatalf("item = %+v", item)
	}
	if item.TargetLabel != "Claude Code" || item.TargetID != "claude-code" {
		t.Fatalf("target = %+v", item)
	}
	if !regexp.MustCompile(`^r-[0-9a-f]{12}$`).MatchString(item.ID) {
		t.Fatalf("id = %q", item.ID)
	}

	out = judge(githubItem("pdf")).Items
	if len(out) != 1 || !out[0].NeedsNetwork || out[0].Origin == nil || out[0].Origin.Commit == "" {
		t.Fatalf("github create = %+v", out)
	}
	if out[0].Origin.Path != "skills/pdf" {
		t.Fatalf("origin = %+v", out[0].Origin)
	}
}

func TestJudgeManifestExists(t *testing.T) {
	out := judge(kbItem("copied")).Items
	if len(out) != 1 || out[0].State != "exists" || out[0].Selectable {
		t.Fatalf("copy exists = %+v", out)
	}
	if !strings.Contains(out[0].Reason, "拷贝") {
		t.Fatalf("reason = %q", out[0].Reason)
	}

	// A junction to the vault skill counts as distributed, whatever the mode.
	item := kbItem("linked")
	out = judge(item).Items
	if len(out) != 1 || out[0].State != "exists" {
		t.Fatalf("link exists = %+v", out)
	}

	out = judge(githubItem("tracked")).Items
	if len(out) != 1 || out[0].State != "exists" {
		t.Fatalf("github exists = %+v", out)
	}
}

func TestJudgeManifestConflictAndSkip(t *testing.T) {
	out := judge(kbItem("unknown-dir")).Items
	if len(out) != 1 || out[0].State != "conflict" || !out[0].Selectable || out[0].DefaultSelected {
		t.Fatalf("conflict = %+v", out)
	}

	item := kbItem("x")
	item.Target = ManifestTarget{Kind: "builtin", ToolID: "gone"}
	out = judge(item).Items
	if len(out) != 1 || out[0].State != "skip" || !strings.Contains(out[0].Reason, "未检测到") {
		t.Fatalf("skip tool = %+v", out)
	}

	item = kbItem("x")
	item.Target = ManifestTarget{Kind: "custom", Name: "Droid"}
	out = judge(item).Items
	if len(out) != 1 || out[0].State != "skip" || !strings.Contains(out[0].Reason, "没有登记") {
		t.Fatalf("skip registration = %+v", out)
	}

	out = judge(kbItem("missing-skill")).Items
	if len(out) != 1 || out[0].State != "skip" || !strings.Contains(out[0].Reason, "知识库里没有") {
		t.Fatalf("skip skill = %+v", out)
	}

	facts := restoreFacts()
	facts.Resolve = func(target ManifestTarget) (RestoreDestination, string) {
		return RestoreDestination{ToolID: "claude-code", Label: "Claude Code", Dir: "/home/u/.claude/builtin_skills"}, ""
	}
	facts.DirExists = func(dir string) bool { return true }
	out = JudgeManifest(Manifest{Status: ManifestOK, Items: []ManifestItem{kbItem("thing")}}, facts).Items
	if len(out) != 1 || out[0].State != "skip" || !strings.Contains(out[0].Reason, "内置") {
		t.Fatalf("skip builtin = %+v", out)
	}
}

func TestJudgeManifestLocalOnly(t *testing.T) {
	judgment := judge(kbItem("fresh"))
	local := judgment.LocalOnly
	// "copied" (same vault) and the tool source "tracked" are local but not
	// in the manifest; "other" belongs to another vault and is ignored.
	if len(local) != 2 {
		t.Fatalf("localOnly = %+v", local)
	}
	for _, entry := range local {
		if entry.Name == "fresh" {
			t.Fatalf("manifest-covered entry leaked: %+v", local)
		}
	}

	// A manifest item covering the same directory hides the local record.
	judgment = judge(kbItem("copied"))
	for _, entry := range judgment.LocalOnly {
		if entry.Name == "copied" {
			t.Fatalf("covered distribution kept: %+v", judgment.LocalOnly)
		}
	}
}

func TestJudgeManifestStableIDs(t *testing.T) {
	first := judge(kbItem("fresh")).Items[0].ID
	second := judge(kbItem("fresh")).Items[0].ID
	if first != second {
		t.Fatalf("ids differ: %q vs %q", first, second)
	}
	other := judge(kbItem("other")).Items[0].ID
	if first == other {
		t.Fatal("different items share an id")
	}
}
