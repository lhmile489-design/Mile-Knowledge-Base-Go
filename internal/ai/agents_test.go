package ai

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

func TestDefaultAgentConfigs(t *testing.T) {
	defs := DefaultAgentConfigs()
	if len(defs) != 4 {
		t.Fatalf("agents: %d", len(defs))
	}
	for _, id := range AgentIDs {
		if _, ok := defs[id]; !ok {
			t.Fatalf("missing %s", id)
		}
	}
	if defs["curator"].Model != "deepseek-v4-flash" || defs["distiller"].Model != "deepseek-v4-pro" {
		t.Errorf("models: %+v", defs)
	}
	if defs["curator"].Effort != "high" {
		t.Errorf("effort: %s", defs["curator"].Effort)
	}
	if defs["scout"].Approval != "auto" || defs["curator"].Approval != "ask" || defs["distiller"].Approval != "ask" || defs["general"].Approval != "ask" {
		t.Errorf("approvals: %+v", defs)
	}
	if !strings.Contains(defs["curator"].Steering, "先预览后执行") || !strings.Contains(defs["general"].Steering, "不假装专门流程") {
		t.Errorf("steering text changed: %q", defs["curator"].Steering)
	}
	if defs["scout"].SteeringBaseVersion != 1 {
		t.Errorf("base version: %d", defs["scout"].SteeringBaseVersion)
	}
}

func TestReadAgentsDefaultsAndFileWins(t *testing.T) {
	dir := t.TempDir()
	got, f := ReadAgents(dir)
	if f != nil || got["scout"].Approval != "auto" {
		t.Fatalf("missing file: %+v %+v", got, f)
	}

	if err := os.MkdirAll(agentsDir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"curator":{"model":"custom","effort":"low","approval":"yolo","skillPaths":[],"steering":"mine","steeringBaseVersion":1},"unknown":{}}`
	if err := os.WriteFile(AgentsPath(dir), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, f = ReadAgents(dir)
	if f != nil {
		t.Fatal(f)
	}
	if got["curator"].Model != "custom" || got["curator"].Effort != "low" || got["curator"].Approval != "yolo" || got["curator"].Steering != "mine" {
		t.Fatalf("file values lost: %+v", got["curator"])
	}
	if got["scout"].Model != "deepseek-v4-flash" {
		t.Fatalf("missing agent should use the D-010 default: %+v", got["scout"])
	}
}

func TestNormalizeAgentFallbacks(t *testing.T) {
	got := NormalizeAgent("scout", map[string]any{
		"model": strings.Repeat("m", 200), "effort": "medium", "approval": "sometimes",
		"skillPaths": []any{"skills/a/SKILL.md", "/abs/x.md", "../up.md", "skills//a/SKILL.md", "skills/a/SKILL.md"},
		"steering":   strings.Repeat("字", 4001),
	})
	if len(got.Model) != 128 {
		t.Errorf("model truncation: %d", len(got.Model))
	}
	if got.Effort != "auto" {
		t.Errorf("invalid effort should normalize to auto: %s", got.Effort)
	}
	if got.Approval != "ask" {
		t.Errorf("invalid approval must fall back to the strictest value: %s", got.Approval)
	}
	if len(got.SkillPaths) != 1 || got.SkillPaths[0] != "skills/a/SKILL.md" {
		t.Errorf("skill paths: %v", got.SkillPaths)
	}
	if len([]rune(got.Steering)) != 4000 {
		t.Errorf("steering truncation: %d runes", len([]rune(got.Steering)))
	}

	// Missing keys use the D-010 defaults, not the normalization fallbacks.
	missing := NormalizeAgent("scout", map[string]any{})
	if missing.Effort != "high" || missing.Approval != "auto" || missing.Model != "deepseek-v4-flash" {
		t.Errorf("missing keys: %+v", missing)
	}
}

func TestAgentsFileBytesAndExtras(t *testing.T) {
	dir := t.TempDir()
	agents := DefaultAgentConfigs()
	curator := agents["curator"]
	curator.Extra = map[string]json.RawMessage{}
	raw, _ := encodeValue("keep-me")
	curator.Extra["futureField"] = raw
	agents["curator"] = curator
	if f := WriteAgents(dir, agents); f != nil {
		t.Fatal(f)
	}
	body, err := os.ReadFile(AgentsPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.HasPrefix(text, "{\n  \"curator\": {\n    \"model\":") {
		t.Fatalf("key order/format:\n%s", text)
	}
	if !strings.Contains(text, `"futureField": "keep-me"`) {
		t.Fatalf("extra field lost:\n%s", text)
	}
	if !strings.HasSuffix(text, "}\n") {
		t.Fatalf("missing trailing newline")
	}
	for _, id := range AgentIDs[1:] {
		if !strings.Contains(text, `"`+id+`": {`) {
			t.Fatalf("missing %s:\n%s", id, text)
		}
	}
}

func TestApplyAgentPatch(t *testing.T) {
	agents := DefaultAgentConfigs()
	steering := "new text"
	model := ""
	patched, f := ApplyAgentPatch(agents, "curator", contract.AgentPatch{Model: &model, Steering: &steering})
	if f != nil {
		t.Fatal(f)
	}
	if patched["curator"].Model != "" || patched["curator"].Steering != "new text" {
		t.Fatalf("patch: %+v", patched["curator"])
	}
	if patched["curator"].Effort != "high" || patched["curator"].Approval != "ask" {
		t.Fatalf("unprovided keys changed: %+v", patched["curator"])
	}

	bad := "medium"
	if _, f := ApplyAgentPatch(agents, "curator", contract.AgentPatch{Effort: &bad}); f == nil || f.Code != contract.CodeInvalidInput {
		t.Fatalf("invalid effort: %+v", f)
	}
	badApproval := "always"
	if _, f := ApplyAgentPatch(agents, "curator", contract.AgentPatch{Approval: &badApproval}); f == nil {
		t.Fatalf("invalid approval accepted")
	}
	if _, f := ApplyAgentPatch(agents, "nobody", contract.AgentPatch{Model: &model}); f == nil {
		t.Fatalf("unknown agent accepted")
	}

	paths := []string{"skills/a/SKILL.md"}
	withPaths, f := ApplyAgentPatch(agents, "scout", contract.AgentPatch{SkillPaths: &paths})
	if f != nil || len(withPaths["scout"].SkillPaths) != 1 {
		t.Fatalf("skill paths: %+v %+v", withPaths["scout"], f)
	}
	escape := []string{"../outside.md"}
	if _, f := ApplyAgentPatch(agents, "scout", contract.AgentPatch{SkillPaths: &escape}); f == nil {
		t.Fatalf("escaping skill path accepted")
	}
}

func TestResetSteering(t *testing.T) {
	agents := DefaultAgentConfigs()
	curator := agents["curator"]
	curator.Steering = "custom"
	agents["curator"] = curator
	reset, f := ResetAgentSteering(agents, "curator")
	if f != nil {
		t.Fatal(f)
	}
	if reset["curator"].Steering != DefaultAgentConfigs()["curator"].Steering {
		t.Fatalf("reset did not restore the default")
	}
}

func TestClampSteering(t *testing.T) {
	short := "ok"
	if ClampSteering(short) != short {
		t.Fatal("short steering changed")
	}
	long := strings.Repeat("字", 4001)
	clamped := ClampSteering(long)
	if !strings.Contains(clamped, "[已截断：steering 超过 4000 字符]") {
		t.Fatalf("missing truncation note: %q", clamped[len(clamped)-60:])
	}
}

func TestSkillCatalogFromDocs(t *testing.T) {
	docs := []SkillDoc{
		{Path: "skills/go/SKILL.md", Title: "Go 规则"},
		{Path: "skills/Go-tools/x/SKILL.md", Title: ""},
		{Path: "skills/go/other.md", Title: "not a skill"},
		{Path: "skills/README.md", Title: "readme"},
		{Path: "raws/SKILL.md", Title: "outside"},
	}
	got := BuildSkillCatalog(docs)
	if len(got) != 2 {
		t.Fatalf("catalog: %+v", got)
	}
	if got[0].Category != "Go-tools" || got[0].Name != "x" {
		t.Errorf("fallback name should be the directory: %+v", got[0])
	}
	if got[1].Path != "skills/go/SKILL.md" || got[1].Name != "Go 规则" {
		t.Errorf("title should win: %+v", got[1])
	}
}
