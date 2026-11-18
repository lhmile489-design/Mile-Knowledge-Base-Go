// Package policy is the agent write boundary: which vault paths an agent may
// write or mutate. It is pure logic, shared by the planner and the executor so
// a tampered plan cannot skip the check (shared/agentPolicy.ts:1-256).
package policy

import (
	"strings"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/vault"
)

// DraftsPrefix stays inside the system area: agents never write it directly.
const DraftsPrefix = ".mile-kb/drafts/"

// GlobalWriteDenyPrefixes apply to every agent in every approval mode.
var GlobalWriteDenyPrefixes = []string{
	".mile-kb/config.json",
	".mile-kb/transactions/",
	".mile-kb/index/",
	".mile-kb/provenance.json",
	".mile-kb/settings/",
}

// ContentAreas are writable by agents; the system area is not.
var ContentAreas = []string{
	vault.AreaSkills, vault.AreaRaws, vault.AreaArchives, vault.AreaProjects,
	vault.AreaTools, vault.AreaWiki, vault.AreaConstraints, vault.AreaOther,
}

// AgentIDs is the fixed agent list.
var AgentIDs = []string{"curator", "scout", "distiller", "general"}

// Mutation kinds accepted from an agent.
const (
	KindCreate          = "create"
	KindCreateDirectory = "createDirectory"
	KindEdit            = "edit"
	KindMove            = "move"
	KindCopy            = "copy"
	KindArchive         = "archive"
	KindTrash           = "trash"
	KindPermanentDelete = "permanentDelete"
)

// Decision is the outcome of one check.
type Decision struct {
	Allowed               bool
	RequiresFreshApproval bool
	Code                  string
	Reason                string
}

func allow() Decision                   { return Decision{Allowed: true} }
func deny(code, reason string) Decision { return Decision{Code: code, Reason: reason} }
func allowFresh() Decision {
	out := allow()
	out.RequiresFreshApproval = true
	return out
}

// IsAgent reports whether id names one of the four agents.
func IsAgent(id string) bool {
	for _, agent := range AgentIDs {
		if agent == id {
			return true
		}
	}
	return false
}

// Normalize turns a path into a vault-relative forward-slash path. Absolute
// paths, UNC, drive letters, empty paths and any dot segment are refused.
func Normalize(path string) (string, bool) {
	if path == "" || strings.HasPrefix(path, "//") || strings.HasPrefix(path, `\\`) {
		return "", false
	}
	if strings.HasPrefix(path, "/") || strings.HasPrefix(path, `\`) {
		return "", false
	}
	if len(path) >= 2 && path[1] == ':' {
		return "", false
	}
	segments := strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' })
	if len(segments) == 0 {
		return "", false
	}
	for _, segment := range segments {
		if segment == "." || segment == ".." {
			return "", false
		}
	}
	return strings.Join(segments, "/"), true
}

func hasDeniedPrefix(relative string) bool {
	lower := strings.ToLower(relative)
	for _, prefix := range GlobalWriteDenyPrefixes {
		p := strings.ToLower(prefix)
		if strings.HasSuffix(p, "/") {
			if strings.HasPrefix(lower, p) {
				return true
			}
		} else if lower == p {
			return true
		}
	}
	return false
}

func areaOf(relative string) string {
	return vault.AreaFor(strings.Split(relative, "/")[0])
}

func isContentArea(area string) bool {
	for _, candidate := range ContentAreas {
		if candidate == area {
			return true
		}
	}
	return false
}

// CanWrite reports whether the agent may write this vault-relative path.
func CanWrite(agent, path string) Decision {
	if !IsAgent(agent) {
		return deny("invalid-path", "未知 Agent")
	}
	relative, ok := Normalize(path)
	if !ok {
		return deny("invalid-path", "路径必须是知识库内的相对路径，且不能包含 . 或 .. 片段")
	}
	// The global deny list is checked first and independently of the area
	// check, so a too-wide prefix can never bypass it.
	if hasDeniedPrefix(relative) {
		return deny("system-path-protected", "该位置由应用维护（配置、事务恢复记录或索引），任何 Agent 都不能写入")
	}
	area := areaOf(relative)
	if area == vault.AreaSystem {
		return deny("system-path-protected", "系统配置目录不接受 Agent 写入")
	}
	if isContentArea(area) {
		return allow()
	}
	return deny("area-not-writable", agent+" 不能写入该区域")
}

// CanMutate reports whether the agent may perform kind on these paths. A
// permanent delete still needs the caller's fresh approval.
func CanMutate(agent, kind, source, target string) Decision {
	if !IsAgent(agent) {
		return deny("invalid-path", "未知 Agent")
	}
	switch kind {
	case KindCreate, KindCreateDirectory, KindEdit:
		path := source
		if path == "" {
			path = target
		}
		return CanWrite(agent, path)
	case KindMove, KindCopy, KindArchive, KindTrash, KindPermanentDelete:
	default:
		return deny("mutation-not-allowed", "不支持的结构变更类型")
	}
	if decision := CanWrite(agent, source); !decision.Allowed {
		return decision
	}
	// An empty target is the vault root, which is always a legal destination.
	if target != "" {
		if decision := CanWrite(agent, target); !decision.Allowed {
			return decision
		}
	}
	if kind == KindPermanentDelete {
		return allowFresh()
	}
	return allow()
}
