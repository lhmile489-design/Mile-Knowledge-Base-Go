package skills

import (
	"reflect"
	"strings"
	"testing"
)

// M6 T01: project-relative skill directories (技术设计 13.6, FR-SKM-PROJ).

func TestProjectDirsShape(t *testing.T) {
	for _, tool := range Registry {
		for _, dir := range tool.ProjectDirs {
			if dir == "" || strings.HasPrefix(dir, "/") || strings.HasSuffix(dir, "/") {
				t.Fatalf("%s: bad project dir %q", tool.ID, dir)
			}
			if strings.ContainsAny(dir, `\:~`) {
				t.Fatalf("%s: project dir %q must be a plain slash-relative path", tool.ID, dir)
			}
			for _, segment := range strings.Split(dir, "/") {
				if segment == "" || segment == "." || segment == ".." {
					t.Fatalf("%s: bad segment in %q", tool.ID, dir)
				}
			}
		}
	}
}

func TestProjectSkillDirsOrder(t *testing.T) {
	want := []string{
		".agents/skills",
		".claude/skills",
		".codex/skills",
		".cursor/skills",
		".trae/skills",
		".qoder/skills",
		".grok/skills",
		".opencode/skills",
		".gemini/skills",
		".windsurf/skills",
		".augment/skills",
		".codebuddy/skills",
		".kiro/skills",
		"skills",
		".cline/skills",
		".clinerules/skills",
		".warp/skills",
		".kimi/skills",
		".dsh/skills",
	}
	got := ProjectSkillDirs()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("project dirs = %v, want %v", got, want)
	}
	seen := map[string]bool{}
	for _, dir := range got {
		if seen[dir] {
			t.Fatalf("duplicate project dir %q", dir)
		}
		seen[dir] = true
	}
}

func TestProjectDirTool(t *testing.T) {
	cases := map[string]string{
		".agents/skills":     "agents",
		".claude/skills":     "claude-code",
		".trae/skills":       "trae",
		".cline/skills":      "cline",
		".clinerules/skills": "cline",
		"skills":             "openclaw",
		".dsh/skills":        "deepseek-harness",
	}
	for dir, want := range cases {
		if got, ok := ProjectDirTool(dir); !ok || got != want {
			t.Fatalf("ProjectDirTool(%q) = %q, %v; want %q", dir, got, ok, want)
		}
	}
	if _, ok := ProjectDirTool(".no-such-tool/skills"); ok {
		t.Fatal("unknown dir must not resolve")
	}
}

func TestProjectDirsUnverifiedToolsStayEmpty(t *testing.T) {
	// 官方没写项目级目录的工具不列（技术设计 13.6）：Pi、Hermes、WorkBuddy、
	// ZCode、Antigravity、MiniMax Code、Trae CN（与 Trae 共用 .trae/skills）。
	for _, id := range []string{"pi", "hermes", "workbuddy", "zcode", "antigravity", "minimax-code", "trae-cn"} {
		tool, ok := ToolByID(id)
		if !ok {
			t.Fatalf("missing tool %q", id)
		}
		if len(tool.ProjectDirs) != 0 {
			t.Fatalf("%s should have no project dir, got %v", id, tool.ProjectDirs)
		}
	}
	// Codex 的 .codex/skills 是推导项（兼容读取），保留但只有一条。
	codex, _ := ToolByID("codex")
	if !reflect.DeepEqual(codex.ProjectDirs, []string{".codex/skills"}) {
		t.Fatalf("codex project dirs = %v", codex.ProjectDirs)
	}
}
