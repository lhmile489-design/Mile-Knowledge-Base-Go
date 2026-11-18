package ai

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// Agent settings limits (数据库设计 3.1, validate.ts:620).
const (
	MaxSteeringChars     = 4000
	MaxSkillPaths        = 500
	MaxAgentModelLen     = 128
	maxAgentSkillPathLen = 1024
)

// AgentIDs is the fixed declaration order (agent.ts:16).
var AgentIDs = []string{"curator", "scout", "distiller", "general"}

var effortLevels = map[string]bool{"auto": true, "disabled": true, "low": true, "high": true, "max": true}
var approvalModes = map[string]bool{"ask": true, "auto": true, "yolo": true}

// AgentConfig is one agents.json entry. Extra keeps unknown fields so a newer
// writer's keys survive (数据库设计 3.0.1).
type AgentConfig struct {
	Model               string
	Effort              string
	Approval            string
	SkillPaths          []string
	Steering            string
	SteeringBaseVersion int
	Extra               map[string]json.RawMessage
}

var agentKnownFields = map[string]bool{
	"model": true, "effort": true, "approval": true, "skillPaths": true,
	"steering": true, "steeringBaseVersion": true,
}

// MarshalJSON writes the fixed key order, then unknown fields sorted.
func (c AgentConfig) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	first := true
	field := func(key string, value any) error {
		if !first {
			buf.WriteByte(',')
		}
		first = false
		k, err := encodeValue(key)
		if err != nil {
			return err
		}
		buf.Write(k)
		buf.WriteByte(':')
		v, err := encodeValue(value)
		if err != nil {
			return err
		}
		buf.Write(v)
		return nil
	}
	steps := []struct {
		key   string
		value any
	}{
		{"model", c.Model}, {"effort", c.Effort}, {"approval", c.Approval},
		{"skillPaths", c.SkillPaths}, {"steering", c.Steering},
		{"steeringBaseVersion", c.SteeringBaseVersion},
	}
	for _, step := range steps {
		if err := field(step.key, step.value); err != nil {
			return nil, err
		}
	}
	keys := make([]string, 0, len(c.Extra))
	for key := range c.Extra {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := field(key, c.Extra[key]); err != nil {
			return nil, err
		}
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// AgentsPath is where agents.json lives inside a vault.
func AgentsPath(vaultRoot string) string {
	return filepath.Join(vaultRoot, ".mile-kb", "settings", "agents.json")
}

func agentsDir(vaultRoot string) string { return filepath.Dir(AgentsPath(vaultRoot)) }

// DefaultAgentConfigs follows D-010 (PRD wording): flash for three agents,
// pro for distiller, effort high, scout auto-approval, everyone else ask.
func DefaultAgentConfigs() map[string]AgentConfig {
	steering := map[string]string{
		"curator": "先预览后执行：任何移动、归档或批量改动都先给出清单，等我确认。\n" +
			"保持目录规范：新文件放进既有分类，不要自造平行结构。\n" +
			"默认归档而不是永久删除。",
		"scout": "来源优先：给结论时附上文件路径，让我能自己核对。\n" +
			"区分「知识库里已有的内容」与「你的推断」，不要混在一起说。\n" +
			"没有命中就直说没有，不要用相关内容凑一个答案。",
		"distiller": "保留来源与不确定性：提炼时标明每条结论出自哪里，存疑的地方明确写出存疑。\n" +
			"事实不自动升级为约束：读到「某项目这样做」不等于「以后都要这样做」。\n" +
			"产出先进草稿区，需要正式化时交接给整理归档。",
		"general": "不假装专门流程已经跑过：没检索就说没检索，没读文件就说没读。\n" +
			"需要整理、检索或提炼时，建议我改派给对应的 Agent，或直接发起交接。",
	}
	models := map[string]string{
		"curator": "deepseek-v4-flash", "scout": "deepseek-v4-flash",
		"distiller": "deepseek-v4-pro", "general": "deepseek-v4-flash",
	}
	out := make(map[string]AgentConfig, len(AgentIDs))
	for _, id := range AgentIDs {
		approval := "ask"
		if id == "scout" {
			approval = "auto"
		}
		out[id] = AgentConfig{
			Model: models[id], Effort: "high", Approval: approval,
			SkillPaths: []string{}, Steering: steering[id], SteeringBaseVersion: 1,
		}
	}
	return out
}

func isKnownAgent(agent string) bool {
	for _, id := range AgentIDs {
		if id == agent {
			return true
		}
	}
	return false
}

// NormalizeAgent coerces one agent entry (AppConfigStore.ts:65-100) with the
// D-010 defaults for missing keys.
func NormalizeAgent(agent string, raw any) AgentConfig {
	defaults := DefaultAgentConfigs()
	fallback, ok := defaults[agent]
	if !ok {
		fallback = defaults["curator"]
	}
	source, ok := asMap(raw)
	if !ok {
		return fallback
	}

	config := fallback
	if value, ok := source["model"].(string); ok {
		config.Model = truncateRunes(value, MaxAgentModelLen)
	}
	if value, exists := source["effort"]; exists {
		if text, ok := value.(string); ok && effortLevels[text] {
			config.Effort = text
		} else {
			// An illegal stored value degrades to auto (agent.ts:43-45), while a
			// missing key keeps the default.
			config.Effort = "auto"
		}
	}
	if value, exists := source["approval"]; exists {
		if text, ok := value.(string); ok && approvalModes[text] {
			config.Approval = text
		} else {
			// Bad approval must degrade to the strictest mode, never to auto/yolo.
			config.Approval = "ask"
		}
	}
	if value, ok := source["skillPaths"].([]any); ok {
		paths := make([]string, 0, len(value))
		for _, item := range value {
			text, ok := item.(string)
			if !ok {
				continue
			}
			clean, ok := cleanSkillPath(text)
			if !ok {
				continue
			}
			if !contains(paths, clean) {
				paths = append(paths, clean)
			}
			if len(paths) >= MaxSkillPaths {
				break
			}
		}
		config.SkillPaths = paths
	}
	if value, ok := source["steering"].(string); ok {
		config.Steering = truncateRunes(value, MaxSteeringChars)
	}
	if value, ok := source["steeringBaseVersion"].(float64); ok && value == float64(int(value)) {
		config.SteeringBaseVersion = int(value)
	}

	extra := map[string]json.RawMessage{}
	for key, value := range source {
		if agentKnownFields[key] {
			continue
		}
		encoded, err := encodeValue(value)
		if err != nil {
			continue
		}
		extra[key] = json.RawMessage(encoded)
	}
	if len(extra) == 0 {
		extra = nil
	}
	config.Extra = extra
	return config
}

func truncateRunes(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit])
}

// cleanSkillPath normalizes separators, drops dot segments and refuses
// absolute paths (validate.ts:544-556, AppConfigStore.ts:56-64).
func cleanSkillPath(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || len(trimmed) > maxAgentSkillPathLen {
		return "", false
	}
	if strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, `\`) {
		return "", false
	}
	if len(trimmed) >= 2 && trimmed[1] == ':' {
		return "", false
	}
	parts := strings.FieldsFunc(trimmed, func(r rune) bool { return r == '/' || r == '\\' })
	if len(parts) == 0 {
		return "", false
	}
	for _, part := range parts {
		if part == "." || part == ".." {
			return "", false
		}
	}
	return strings.Join(parts, "/"), true
}

// CoerceAgentMap keeps only the four known ids and fills the rest with the
// defaults (AppConfigStore.ts:102-109).
func CoerceAgentMap(raw any) map[string]AgentConfig {
	out := DefaultAgentConfigs()
	source, ok := asMap(raw)
	if !ok {
		return out
	}
	for _, id := range AgentIDs {
		if value, exists := source[id]; exists {
			out[id] = NormalizeAgent(id, value)
		}
	}
	return out
}

// ReadAgents reads agents.json; a missing or unreadable file yields the
// defaults (bad JSON degrades to defaults so the settings page still opens).
func ReadAgents(vaultRoot string) (map[string]AgentConfig, *contract.Failure) {
	raw, err := os.ReadFile(AgentsPath(vaultRoot))
	if err != nil {
		return DefaultAgentConfigs(), nil
	}
	var top any
	if json.Unmarshal(raw, &top) != nil {
		return DefaultAgentConfigs(), nil
	}
	return CoerceAgentMap(top), nil
}

// WriteAgents writes all four agents in declaration order, atomically, with
// no HTML escaping and a trailing newline (AppConfigStore.ts:214-227).
func WriteAgents(vaultRoot string, agents map[string]AgentConfig) *contract.Failure {
	if err := os.MkdirAll(agentsDir(vaultRoot), 0o755); err != nil {
		return contract.Fail(contract.CodeIOFailure, "无法创建设置目录")
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	ordered := make(map[string]AgentConfig, len(AgentIDs))
	for _, id := range AgentIDs {
		ordered[id] = agents[id]
	}
	if err := enc.Encode(orderedAgents(ordered)); err != nil {
		return contract.Fail(contract.CodeIOFailure, "无法序列化 Agent 配置")
	}
	target := AgentsPath(vaultRoot)
	tmp := target + "." + randomHex(4) + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return contract.Fail(contract.CodeIOFailure, "无法写入 Agent 配置")
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return contract.Fail(contract.CodeIOFailure, "无法写入 Agent 配置")
	}
	return nil
}

// orderedAgents is a JSON object whose MarshalJSON keeps AgentIDs order;
// encoding/json sorts map keys, so the order cannot come from the map alone.
type orderedAgents map[string]AgentConfig

func (o orderedAgents) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for index, id := range AgentIDs {
		if index > 0 {
			buf.WriteByte(',')
		}
		key, err := encodeValue(id)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteByte(':')
		value, err := encodeValue(o[id])
		if err != nil {
			return nil, err
		}
		buf.Write(value)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// ClampSteering is the injection-time truncation: it appends a note so a half
// sentence is never mistaken for a complete instruction (appConfig.ts:377-381).
func ClampSteering(text string) string {
	if len([]rune(text)) <= MaxSteeringChars {
		return text
	}
	return truncateRunes(text, MaxSteeringChars) + "\n[已截断：steering 超过 4000 字符]"
}

// ApplyAgentPatch merges only the provided keys (M63, validate.ts:513-565).
func ApplyAgentPatch(agents map[string]AgentConfig, agent string, patch contract.AgentPatch) (map[string]AgentConfig, *contract.Failure) {
	if !isKnownAgent(agent) {
		return nil, invalid("未知的 Agent")
	}
	current := agents[agent]
	if patch.Model != nil {
		current.Model = truncateRunes(*patch.Model, MaxAgentModelLen)
	}
	if patch.Effort != nil {
		if !effortLevels[*patch.Effort] {
			return nil, invalid("effort 取值不合法")
		}
		current.Effort = *patch.Effort
	}
	if patch.Approval != nil {
		if !approvalModes[*patch.Approval] {
			return nil, invalid("approval 只接受 ask / auto / yolo")
		}
		current.Approval = *patch.Approval
	}
	if patch.Steering != nil {
		current.Steering = truncateRunes(*patch.Steering, MaxSteeringChars)
	}
	if patch.SkillPaths != nil {
		if len(*patch.SkillPaths) > MaxSkillPaths {
			return nil, invalid("skillPaths 过多")
		}
		paths := make([]string, 0, len(*patch.SkillPaths))
		for _, item := range *patch.SkillPaths {
			clean, ok := cleanSkillPath(item)
			if !ok {
				return nil, contract.Fail(contract.CodePathEscape, "skillPaths 必须是库内相对路径")
			}
			if !contains(paths, clean) {
				paths = append(paths, clean)
			}
		}
		current.SkillPaths = paths
	}
	next := make(map[string]AgentConfig, len(agents))
	for id, config := range agents {
		next[id] = config
	}
	next[agent] = current
	return next, nil
}

// ResetAgentSteering restores one agent's default steering (M64).
func ResetAgentSteering(agents map[string]AgentConfig, agent string) (map[string]AgentConfig, *contract.Failure) {
	if !isKnownAgent(agent) {
		return nil, invalid("未知的 Agent")
	}
	current := agents[agent]
	fallback := DefaultAgentConfigs()[agent]
	current.Steering = fallback.Steering
	current.SteeringBaseVersion = fallback.SteeringBaseVersion
	next := make(map[string]AgentConfig, len(agents))
	for id, config := range agents {
		next[id] = config
	}
	next[agent] = current
	return next, nil
}

// SkillDoc is the index row the catalog needs (path + title).
type SkillDoc struct {
	Path  string
	Title string
}

// SkillEntry is one assignable skill (M71).
type SkillEntry struct {
	Path     string `json:"path"`
	Category string `json:"category"`
	Name     string `json:"name"`
}

// BuildSkillCatalog keeps skills/<…>/SKILL.md at depth ≥3, takes the category
// from the second segment and the title from the index (falling back to the
// directory name), then sorts by category and name (IndexService.ts:278-300).
func BuildSkillCatalog(docs []SkillDoc) []SkillEntry {
	out := make([]SkillEntry, 0, len(docs))
	for _, doc := range docs {
		path := doc.Path
		if !strings.HasPrefix(path, "skills/") || !strings.HasSuffix(strings.ToLower(path), "/skill.md") {
			continue
		}
		segments := strings.Split(path, "/")
		if len(segments) < 3 {
			continue
		}
		name := strings.TrimSpace(doc.Title)
		if name == "" {
			name = segments[len(segments)-2]
		}
		out = append(out, SkillEntry{Path: path, Category: segments[1], Name: name})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		return out[i].Name < out[j].Name
	})
	return out
}
