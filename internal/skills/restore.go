package skills

import (
	"crypto/sha1"
	"encoding/hex"
	"path/filepath"
	"strings"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// M7 T03: judge every manifest entry against this machine (技术设计 18.5).
// The judgment is a pure function over the facts the service provides; it
// never touches the network and never writes anything.

// RestoreDestination is a matched local target.
type RestoreDestination struct {
	ToolID string // builtin registry id or registration id
	Label  string // display label
	Dir    string // absolute directory the skill lands in
}

// RestoreFacts is what the judge knows about this machine. The service fills
// it from the detector, the registrations, state.json and skills/sources.json.
type RestoreFacts struct {
	Resolve       func(target ManifestTarget) (RestoreDestination, string)
	SkillDir      func(skillPath string) string // "" when the vault skill is missing
	DirExists     func(dir string) bool
	LinkTarget    func(dir string) (string, bool) // dir is a junction/symlink and points here
	SourceAt      func(dir string) (ToolSource, bool)
	SourceRecords []ToolSource
	Distributions []Distribution
	VaultID       string
	IsBuiltinDir  func(toolID, dir string) bool
	LabelFor      func(toolID string) string
}

// RestoreJudgment is the judged preview payload.
type RestoreJudgment struct {
	Items     []contract.SkillRestoreItem
	LocalOnly []contract.SkillRestoreLocal
}

// JudgeManifest walks the manifest in its stored order.
func JudgeManifest(manifest Manifest, facts RestoreFacts) RestoreJudgment {
	judgment := RestoreJudgment{
		Items:     make([]contract.SkillRestoreItem, 0, len(manifest.Items)),
		LocalOnly: []contract.SkillRestoreLocal{},
	}
	covered := map[string]bool{}
	for _, item := range manifest.Items {
		judged, dir := judgeItem(item, facts)
		judgment.Items = append(judgment.Items, judged)
		if dir != "" {
			covered[comparablePath(dir)] = true
		}
	}
	for _, dist := range facts.Distributions {
		if facts.VaultID != "" && dist.VaultID != facts.VaultID {
			continue
		}
		if covered[comparablePath(dist.TargetPath)] {
			continue
		}
		judgment.LocalOnly = append(judgment.LocalOnly, contract.SkillRestoreLocal{
			Name:        dist.Name,
			TargetLabel: restoreLabel(facts.LabelFor, dist.ToolID),
		})
	}
	for _, source := range facts.sources() {
		if covered[comparablePath(source.TargetPath)] {
			continue
		}
		judgment.LocalOnly = append(judgment.LocalOnly, contract.SkillRestoreLocal{
			Name:        filepath.Base(source.TargetPath),
			TargetLabel: restoreLabel(facts.LabelFor, source.ToolID),
		})
	}
	return judgment
}

// sources returns the tool source records the facts carry through SourceAt.
// The service passes a snapshot list; tests may leave it empty.
func (f RestoreFacts) sources() []ToolSource { return f.SourceRecords }

func judgeItem(item ManifestItem, facts RestoreFacts) (contract.SkillRestoreItem, string) {
	judged := contract.SkillRestoreItem{
		ID:          restoreID(manifestSortKey(item)),
		Source:      item.Source,
		Name:        item.Name,
		SkillPath:   item.SkillPath,
		Mode:        item.Mode,
		Target:      contract.SkillManifestTarget{Kind: item.Target.Kind, ToolID: item.Target.ToolID, Name: item.Target.Name, Subdir: item.Target.Subdir},
		TargetLabel: manifestTargetLabel(item.Target),
	}
	if item.Source == "github" {
		judged.Origin = &contract.SkillOrigin{
			Owner: item.Owner, Repo: item.Repo, Ref: item.Ref, Path: item.Path, Commit: item.Commit,
		}
	}

	dest, reason := RestoreDestination{}, ""
	if facts.Resolve != nil {
		dest, reason = facts.Resolve(item.Target)
	}
	if reason != "" {
		judged.State = "skip"
		judged.Reason = reason
		return judged, ""
	}
	judged.TargetID = dest.ToolID
	if dest.Label != "" {
		judged.TargetLabel = dest.Label
	}
	dir := filepath.Join(dest.Dir, item.Name)

	if item.Source == "kb" {
		skillDir := ""
		if facts.SkillDir != nil {
			skillDir = facts.SkillDir(item.SkillPath)
		}
		if skillDir == "" {
			judged.State = "skip"
			judged.Reason = "知识库里没有这个技能"
			return judged, ""
		}
		if facts.IsBuiltinDir != nil && facts.IsBuiltinDir(dest.ToolID, dir) {
			judged.State = "skip"
			judged.Reason = "内置技能不能覆盖"
			return judged, ""
		}
		exists := facts.DirExists != nil && facts.DirExists(dir)
		if !exists {
			judged.State = "create"
			judged.Selectable = true
			judged.DefaultSelected = true
			return judged, dir
		}
		if facts.LinkTarget != nil {
			if linked, ok := facts.LinkTarget(dir); ok && strings.EqualFold(filepath.Clean(linked), filepath.Clean(skillDir)) {
				judged.State = "exists"
				judged.Reason = "已分发（链接）"
				return judged, dir
			}
		}
		for _, dist := range facts.Distributions {
			if dist.ToolID == dest.ToolID && strings.EqualFold(filepath.Clean(dist.TargetPath), filepath.Clean(dir)) {
				judged.State = "exists"
				if dist.Mode == item.Mode {
					judged.Reason = "已分发"
				} else if dist.Mode == "copy" {
					judged.Reason = "本机为拷贝"
				} else {
					judged.Reason = "本机为链接"
				}
				return judged, dir
			}
		}
		judged.State = "conflict"
		judged.Reason = "目标位置已有同名目录"
		judged.Selectable = true
		return judged, dir
	}

	// source == github
	if facts.IsBuiltinDir != nil && facts.IsBuiltinDir(dest.ToolID, dir) {
		judged.State = "skip"
		judged.Reason = "内置技能不能覆盖"
		return judged, ""
	}
	exists := facts.DirExists != nil && facts.DirExists(dir)
	if !exists {
		judged.State = "create"
		judged.NeedsNetwork = true
		judged.Selectable = true
		judged.DefaultSelected = true
		return judged, dir
	}
	judged.NeedsNetwork = true
	if facts.SourceAt != nil {
		if source, ok := facts.SourceAt(dir); ok &&
			strings.EqualFold(source.Owner, item.Owner) &&
			strings.EqualFold(source.Repo, item.Repo) &&
			strings.EqualFold(source.Path, item.Path) {
			judged.State = "exists"
			judged.Reason = "已安装（GitHub）"
			return judged, dir
		}
	}
	judged.State = "conflict"
	judged.Reason = "目标位置已有同名目录"
	judged.Selectable = true
	return judged, dir
}

func manifestTargetLabel(target ManifestTarget) string {
	switch target.Kind {
	case "builtin":
		if tool, ok := ToolByID(target.ToolID); ok {
			return tool.Name
		}
		return target.ToolID
	case "custom":
		return target.Name
	case "project":
		return "项目 " + target.Name + " · " + target.Subdir
	}
	return target.Kind
}

func restoreLabel(labelFor func(string) string, toolID string) string {
	if labelFor != nil {
		if label := labelFor(toolID); label != "" {
			return label
		}
	}
	return toolID
}

func restoreID(key string) string {
	sum := sha1.Sum([]byte(key))
	return "r-" + hex.EncodeToString(sum[:])[:12]
}
