package skills

// Tool is one built-in registry entry (技术设计 13.2, FR-SKM-TOOL). Path
// templates use "~" (the user profile) and "%LOCALAPPDATA%"; ScanRelative and
// BuiltinRelative entries are relative to the detected install location
// (ZCode, MiniMax). Builtin dirs hold read-only skills shipped with the tool
// (PRD FR-SKM-SCAN). A "*" segment marks one directory level to expand.
type Tool struct {
	ID              string
	Name            string
	IconKey         string
	Shared          bool // the shared ~/.agents/skills entry
	InstallDir      string
	ScanDirs        []string
	ScanRelative    []string
	BuiltinDirs     []string
	BuiltinRelative []string
	Readers         []string // shared entry: tools that read ~/.agents/skills
	Official        bool
	Enabled         bool
	CLIs            []string
	Uninstall       []string // DisplayName prefixes in the uninstall registry
	KnownExes       []string // relative paths under UserProgramFiles
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
	},
	{
		ID: "claude-code", Name: "Claude Code", IconKey: "claude-code",
		InstallDir: "~/.claude/skills", Official: true, Enabled: true,
		CLIs: []string{"claude"}, Uninstall: []string{"Claude Code"},
	},
	{
		ID: "codex", Name: "Codex", IconKey: "codex", Shared: false,
		ScanDirs: []string{"~/.codex/skills"}, Official: true, Enabled: true,
		CLIs: []string{"codex"},
	},
	{
		ID: "cursor", Name: "Cursor", IconKey: "cursor",
		InstallDir: "~/.cursor/skills", ScanDirs: []string{"~/.cursor/skills-cursor"},
		Official: true, Enabled: true,
		CLIs: []string{"cursor"}, Uninstall: []string{"Cursor"},
	},
	{
		ID: "trae", Name: "Trae", IconKey: "trae",
		InstallDir: "~/.trae/skills", BuiltinDirs: []string{"~/.trae/builtin_skills"},
		Official: false, Enabled: true, // 官方页面未读到，按本机证据启用
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
		CLIs: []string{"qoder", "qodercli"}, Uninstall: []string{"Qoder"},
	},
	{
		ID: "grok", Name: "Grok Build", IconKey: "grok",
		InstallDir: "~/.grok/skills", BuiltinDirs: []string{"~/.grok/bundled/skills"},
		Official: true, Enabled: true,
		CLIs: []string{"grok"},
	},
	{
		ID: "opencode", Name: "OpenCode", IconKey: "opencode",
		InstallDir: "~/.config/opencode/skills", ScanDirs: []string{"~/.opencode/skills"},
		Official: true, Enabled: true,
		Uninstall: []string{"OpenCode"},
	},
	{
		ID: "gemini-cli", Name: "Gemini CLI", IconKey: "gemini",
		InstallDir: "~/.gemini/skills", Official: true, Enabled: true,
		CLIs: []string{"gemini"},
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
		Uninstall: []string{"Windsurf"},
	},
	{
		ID: "augment", Name: "Augment", IconKey: "augment",
		InstallDir: "~/.augment/skills", Official: true, Enabled: true,
		CLIs: []string{"augment"},
	},
	{
		ID: "codebuddy", Name: "CodeBuddy", IconKey: "codebuddy",
		InstallDir: "~/.codebuddy/skills", Official: true, Enabled: true,
		CLIs: []string{"codebuddy"}, Uninstall: []string{"CodeBuddy"},
	},
	{
		ID: "pi", Name: "Pi", IconKey: "pi",
		InstallDir: "~/.pi/agent/skills", Official: true, Enabled: true,
		CLIs: []string{"pi"},
	},
	{
		ID: "kiro", Name: "Kiro", IconKey: "kiro",
		InstallDir: "~/.kiro/skills", Official: true, Enabled: true,
		CLIs: []string{"kiro", "kiro-cli"}, Uninstall: []string{"Kiro"},
	},
	{
		ID: "hermes", Name: "Hermes", IconKey: "hermes",
		InstallDir: "~/.hermes/skills", ScanDirs: []string{"%LOCALAPPDATA%/hermes/skills"},
		Official: true, Enabled: true,
		CLIs: []string{"hermes"}, Uninstall: []string{"Hermes"},
	},
	{
		ID: "openclaw", Name: "OpenClaw", IconKey: "openclaw",
		InstallDir: "~/.openclaw/skills",
		ScanDirs:   []string{"~/.clawdbot/skills", "~/.moltbot/skills"},
		Official:   true, Enabled: true,
		CLIs: []string{"openclaw"},
	},
	{
		ID: "cline", Name: "Cline", IconKey: "cline",
		InstallDir: "~/.cline/skills", Official: true, Enabled: true,
		CLIs: []string{"cline"},
	},
	{
		ID: "warp", Name: "Warp", IconKey: "warp",
		ScanDirs: []string{"~/.warp/skills"}, Official: true, Enabled: true,
		Uninstall: []string{"Warp"},
	},
	{
		ID: "kimi", Name: "Kimi Code CLI", IconKey: "kimi",
		ScanDirs: []string{"~/.kimi/skills"}, Official: true, Enabled: true,
		CLIs: []string{"kimi"}, Uninstall: []string{"Kimi"},
	},
	{
		ID: "zcode", Name: "ZCode", IconKey: "zcode",
		InstallDir: "~/.zcode/skills", BuiltinRelative: []string{`resources\glm\packages\*\skills`},
		Official: false, Enabled: true, // 官方文档未查到，本机证据
		Uninstall: []string{"ZCode"}, KnownExes: []string{`ZCode\ZCode.exe`},
	},
	{
		ID: "deepseek-harness", Name: "DeepSeek Harness", IconKey: "deepseek-harness",
		InstallDir: "~/.dsh/skills", Official: true, Enabled: true,
		Uninstall: []string{"DeepSeek"}, KnownExes: []string{`DeepSeek Harness\DeepSeek Harness.exe`},
	},
	{
		ID: "minimax-code", Name: "MiniMax Code", IconKey: "", // letter placeholder
		BuiltinRelative: []string{`resources\resources\daemon\skills`},
		Official:        false, Enabled: false, // 官方未核实，默认不启用
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
