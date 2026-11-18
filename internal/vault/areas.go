package vault

import "strings"

// Area ids (PRD FR-AREA). "other" is the catch-all for unmapped directories and
// is never hidden; "system" holds dot directories and is excluded from search.
const (
	AreaSkills      = "skills"
	AreaRaws        = "raws"
	AreaArchives    = "archives"
	AreaProjects    = "projects"
	AreaTools       = "tools"
	AreaWiki        = "wiki"
	AreaConstraints = "constraints"
	AreaSystem      = "system"
	AreaOther       = "other"
)

// AreaOrder is the sidebar section order.
var AreaOrder = []string{
	AreaSkills, AreaRaws, AreaProjects, AreaTools, AreaWiki, AreaConstraints, AreaArchives, AreaOther, AreaSystem,
}

var areaDirectories = map[string][]string{
	AreaSkills:      {"skills"},
	AreaRaws:        {"raws", "clippings"},
	AreaArchives:    {"_archives"},
	AreaProjects:    {"projects"},
	AreaTools:       {"tools"},
	AreaWiki:        {"ai knowledge base"},
	AreaConstraints: {"dev-constraints"},
}

// AreaFor maps a top-level directory name to its functional area.
func AreaFor(name string) string {
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, ".") {
		return AreaSystem
	}
	for area, names := range areaDirectories {
		for _, n := range names {
			if lower == n {
				return area
			}
		}
	}
	return AreaOther
}

// IconFor picks the semantic icon key for a directory, matching the legacy
// keyword order (shared/directoryIcons.ts). The UI maps the key to a component.
func IconFor(name, area string) string {
	if area == AreaSystem {
		return "system"
	}
	if area != AreaOther {
		return area
	}
	lower := strings.ToLower(name)
	type rule struct {
		icon  string
		words []string
	}
	rules := []rule{
		{"react-native", []string{"react-native", "reactnative"}},
		{"javascript", []string{"javascript", "typescript", "node", "nodejs", "js", "ts"}},
		{"ai", []string{"ai", "llm", "prompt", "agent"}},
		{"frontend", []string{"front", "ui", "web", "react", "vue"}},
		{"backend", []string{"back", "server", "api", "service"}},
		{"mobile", []string{"mobile", "android", "ios", "app"}},
		{"miniprogram", []string{"miniprogram", "wechat", "小程序"}},
		{"desktop", []string{"desktop", "electron", "mygo", "tauri"}},
		{"database", []string{"database", "db", "sql", "mysql", "redis", "数据库"}},
		{"design", []string{"design", "ui-ux", "figma", "design-tokens", "设计"}},
		{"testing", []string{"test", "testing", "qa", "测试"}},
		{"deployment", []string{"deploy", "devops", "ci", "docker", "部署"}},
		{"copywriting", []string{"copy", "writing", "文案"}},
		{"docs", []string{"doc", "docs", "readme", "notes", "笔记", "文档"}},
		{"maps", []string{"map", "geo", "地图"}},
	}
	for _, r := range rules {
		for _, w := range r.words {
			if strings.Contains(lower, w) {
				return r.icon
			}
		}
	}
	switch lower {
	case "go", "c", "cpp", "r", "lua", "perl", "ruby", "swift", "rust", "java", "python", "kotlin":
		return "language"
	}
	return "folder"
}
