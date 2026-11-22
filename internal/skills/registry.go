package skills

// Tool is one built-in registry entry (技术设计 13.2, FR-SKM-TOOL). Path
// templates use "~" (the user profile) and "%LOCALAPPDATA%"; ScanRelative
// entries are relative to the tool's skill install directory, while
// AppBuiltinRelative entries are relative to the tool's application install
// location (ZCode, MiniMax) and may contain a "*" segment to expand. Builtin
// dirs hold read-only skills shipped with the tool (PRD FR-SKM-SCAN).
type Tool struct {
	ID                 string
	Name               string
	IconKey            string
	Shared             bool // the shared ~/.agents/skills entry
	InstallDir         string
	ScanDirs           []string
	ScanRelative       []string
	BuiltinDirs        []string
	AppBuiltinRelative []string
	Readers            []string // shared entry: tools that read ~/.agents/skills
	// ProjectDirs are project-relative skill directories this tool reads
	// (FR-SKM-PROJ, 技术设计 13.6). Slash-separated, no leading "~" or "."
	// implied by the tool's user-level path; only documented conventions are
	// listed.
	ProjectDirs []string
	Official    bool
	Enabled     bool
	CLIs        []string
	Uninstall   []string // DisplayName prefixes in the uninstall registry
	KnownExes   []string // relative paths under UserProgramFiles
}

// sharedReaders are the tools whose official docs read ~/.agents/skills
// (技术设计 13.1, 15 tools).
var sharedReaders = []string{
	"codex", "cursor", "grok", "opencode", "gemini-cli", "antigravity",
	"windsurf", "augment", "pi", "hermes", "openclaw", "warp", "kimi",
	"deepseek-harness", "trae-cn",
}

// Registry is the built-in tool table, in display order: the shared directory
// first, then the 24 tools (PRD FR-SKM-TOOL; 技术设计 13.2).
var Registry = []Tool{
	{
		ID: "agents", Name: "通用目录", IconKey: "globe", Shared: true,
		InstallDir: "~/.agents/skills", Readers: sharedReaders, Official: true, Enabled: true,
		ProjectDirs: []string{".agents/skills"}, // Codex/OpenCode/Gemini/Augment/Pi/Kimi/DeepSeek/Warp/Trae/Cursor 等
	},
	{
		ID: "claude-code", Name: "Claude Code", IconKey: "claude-code",
		InstallDir: "~/.claude/skills", Official: true, Enabled: true,
		ProjectDirs: []string{".claude/skills"}, // 项目级 skills 官方写明，含嵌套子目录
		CLIs:        []string{"claude"}, Uninstall: []string{"Claude Code"},
	},
	{
		ID: "codex", Name: "Codex", IconKey: "codex", Shared: false,
		ScanDirs: []string{"~/.codex/skills"}, Official: true, Enabled: true,
		ProjectDirs: []string{".codex/skills"}, // 推导：Codex 官方只写 .agents/skills；Cursor/Kimi/Warp 作为兼容目录读取
		CLIs:        []string{"codex"},
	},
	{
		ID: "cursor", Name: "Cursor", IconKey: "cursor",
		InstallDir: "~/.cursor/skills", ScanDirs: []string{"~/.cursor/skills-cursor"},
		Official: true, Enabled: true,
		ProjectDirs: []string{".cursor/skills"},
		CLIs:        []string{"cursor"}, Uninstall: []string{"Cursor"},
	},
	{
		ID: "trae", Name: "Trae", IconKey: "trae",
		InstallDir: "~/.trae/skills", BuiltinDirs: []string{"~/.trae/builtin_skills"},
		ProjectDirs: []string{".trae/skills"}, // Trae CN 官方文档写项目内 .trae/skills；国际版同一约定
		Official:    false, Enabled: true,     // 官方页面未读到，按本机证据启用
		Uninstall: []string{"TRAE"},
	},
	{
		ID: "trae-cn", Name: "Trae CN", IconKey: "trae-cn",
		InstallDir: "~/.trae-cn/skills", Official: true, Enabled: true,
		Uninstall: []string{"Trae CN"},
	},
	{
		ID: "workbuddy", Name: "WorkBuddy", IconKey: "workbuddy",
		InstallDir: "~/.workbuddy/skills", Official: false, Enabled: true, // Q-M4-1
		Uninstall: []string{"WorkBuddy"},
	},
	{
		ID: "qoder", Name: "Qoder", IconKey: "qoder",
		InstallDir: "~/.qoder/skills", Official: true, Enabled: true,
		ProjectDirs: []string{".qoder/skills"},
		CLIs:        []string{"qoder", "qodercli"}, Uninstall: []string{"Qoder"},
	},
	{
		ID: "grok", Name: "Grok Build", IconKey: "grok",
		InstallDir: "~/.grok/skills", BuiltinDirs: []string{"~/.grok/bundled/skills"},
		ProjectDirs: []string{".grok/skills"},
		Official:    true, Enabled: true,
		CLIs: []string{"grok"},
	},
	{
		ID: "opencode", Name: "OpenCode", IconKey: "opencode",
		InstallDir: "~/.config/opencode/skills", ScanDirs: []string{"~/.opencode/skills"},
		ProjectDirs: []string{".opencode/skills"},
		Official:    true, Enabled: true,
		Uninstall: []string{"OpenCode"},
	},
	{
		ID: "gemini-cli", Name: "Gemini CLI", IconKey: "gemini",
		InstallDir: "~/.gemini/skills", Official: true, Enabled: true,
		ProjectDirs: []string{".gemini/skills"},
		CLIs:        []string{"gemini"},
	},
	{
		ID: "antigravity", Name: "Antigravity", IconKey: "antigravity",
		InstallDir:  "~/.gemini/config/skills",
		ScanDirs:    []string{"~/.gemini/antigravity/skills"},
		BuiltinDirs: []string{"~/.gemini/antigravity/builtin/skills"},
		Official:    true, Enabled: true,
		Uninstall: []string{"Antigravity"},
	},
	{
		ID: "windsurf", Name: "Windsurf", IconKey: "windsurf",
		InstallDir: "~/.codeium/windsurf/skills", Official: true, Enabled: true,
		ProjectDirs: []string{".windsurf/skills"}, // 官方文档已迁到 docs.devin.ai，写作旧版读取；新主路径 .devin/skills 不在本工具登记内（技术设计 13.6）
		Uninstall:   []string{"Windsurf"},
	},
	{
		ID: "augment", Name: "Augment", IconKey: "augment",
		InstallDir: "~/.augment/skills", Official: true, Enabled: true,
		ProjectDirs: []string{".augment/skills"},
		CLIs:        []string{"augment"},
	},
	{
		ID: "codebuddy", Name: "CodeBuddy", IconKey: "codebuddy",
		InstallDir: "~/.codebuddy/skills", Official: true, Enabled: true,
		ProjectDirs: []string{".codebuddy/skills"},
		CLIs:        []string{"codebuddy"}, Uninstall: []string{"CodeBuddy"},
	},
	{
		ID: "pi", Name: "Pi", IconKey: "pi",
		InstallDir: "~/.pi/agent/skills", Official: true, Enabled: true,
		CLIs: []string{"pi"},
	},
	{
		ID: "kiro", Name: "Kiro", IconKey: "kiro",
		InstallDir: "~/.kiro/skills", Official: true, Enabled: true,
		ProjectDirs: []string{".kiro/skills"},
		CLIs:        []string{"kiro", "kiro-cli"}, Uninstall: []string{"Kiro"},
	},
	{
		ID: "hermes", Name: "Hermes", IconKey: "hermes",
		InstallDir: "~/.hermes/skills", ScanDirs: []string{"%LOCALAPPDATA%/hermes/skills"},
		Official: true, Enabled: true,
		CLIs: []string{"hermes"}, Uninstall: []string{"Hermes"},
	},
	{
		ID: "openclaw", Name: "OpenClaw", IconKey: "openclaw",
		InstallDir:  "~/.openclaw/skills",
		ScanDirs:    []string{"~/.clawdbot/skills", "~/.moltbot/skills"},
		ProjectDirs: []string{"skills"}, // 工作区根的 skills/（OpenClaw 文档）
		Official:    true, Enabled: true,
		CLIs: []string{"openclaw"},
	},
	{
		ID: "cline", Name: "Cline", IconKey: "cline",
		InstallDir: "~/.cline/skills", Official: true, Enabled: true,
		ProjectDirs: []string{".cline/skills", ".clinerules/skills"},
		CLIs:        []string{"cline"},
	},
	{
		ID: "warp", Name: "Warp", IconKey: "warp",
		ScanDirs: []string{"~/.warp/skills"}, Official: true, Enabled: true,
		ProjectDirs: []string{".warp/skills"},
		Uninstall:   []string{"Warp"},
	},
	{
		ID: "kimi", Name: "Kimi Code CLI", IconKey: "kimi",
		ScanDirs: []string{"~/.kimi/skills"}, Official: true, Enabled: true,
		ProjectDirs: []string{".kimi/skills"},
		CLIs:        []string{"kimi"}, Uninstall: []string{"Kimi"},
	},
	{
		ID: "zcode", Name: "ZCode", IconKey: "zcode",
		InstallDir: "~/.zcode/skills", AppBuiltinRelative: []string{`resources\glm\packages\*\skills`},
		Official: false, Enabled: true, // 官方文档未查到，本机证据
		Uninstall: []string{"ZCode"}, KnownExes: []string{`ZCode\ZCode.exe`},
	},
	{
		ID: "deepseek-harness", Name: "DeepSeek Harness", IconKey: "deepseek-harness",
		InstallDir: "~/.dsh/skills", Official: true, Enabled: true,
		ProjectDirs: []string{".dsh/skills"},
		Uninstall:   []string{"DeepSeek"}, KnownExes: []string{`DeepSeek Harness\DeepSeek Harness.exe`},
	},
	{
		ID: "minimax-code", Name: "MiniMax Code", IconKey: "", // letter placeholder
		AppBuiltinRelative: []string{`resources\resources\daemon\skills`},
		Official:           false, Enabled: false, // 官方未核实，默认不启用
		KnownExes: []string{`MiniMax Code\MiniMax Code.exe`},
	},
}

// ToolByID returns the registry entry with this id.
func ToolByID(id string) (Tool, bool) {
	for _, tool := range Registry {
		if tool.ID == id {
			return tool, true
		}
	}
	return Tool{}, false
}

// ProjectSkillDirs returns the deduplicated project-relative skill directories
// of the whole registry, in registry order; the first tool that lists a
// directory owns it (FR-SKM-PROJ, 技术设计 13.6).
func ProjectSkillDirs() []string {
	seen := make(map[string]bool, 20)
	out := make([]string, 0, 20)
	for _, tool := range Registry {
		for _, dir := range tool.ProjectDirs {
			if seen[dir] {
				continue
			}
			seen[dir] = true
			out = append(out, dir)
		}
	}
	return out
}

// ProjectDirTool reports which registry tool a project-relative skill
// directory belongs to (first in registry order). Project scan rows use it for
// sourceToolId (API §14.1).
func ProjectDirTool(rel string) (string, bool) {
	for _, tool := range Registry {
		for _, dir := range tool.ProjectDirs {
			if dir == rel {
				return tool.ID, true
			}
		}
	}
	return "", false
}
