package mutation

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/pathguard"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/vault"
)

// MaxWriteBytes is the M18 text limit; plans validate it up front so the user
// is not told after confirming.
const MaxWriteBytes = 2 << 20

// Builder validates plans against a vault root. It only reads.
type Builder struct {
	Root string
	// NewID is injectable for tests; defaults to a random plan id.
	NewID func() string
}

// Plan validates the request and returns the plan the user will confirm
// (API.md M29). Nothing is written: every check is a read.
func (b Builder) Plan(req contract.PlanRequest) (contract.MutationPlan, *contract.Failure) {
	plan := contract.MutationPlan{VaultID: req.VaultID, Origin: req.Origin}
	if req.Origin != "user" {
		// Agents arrive with M2, together with their permission gate.
		return plan, contract.Fail(contract.CodeInvalidInput, "本期只接受用户发起的计划")
	}
	if len(req.Actions) == 0 {
		return plan, contract.Fail(contract.CodeInvalidInput, "计划不能为空")
	}
	if len(req.Actions) > MaxPlanActions {
		return plan, contract.Fail(contract.CodeInvalidInput, "单个计划最多 500 项")
	}

	actions := make([]contract.MutationAction, 0, len(req.Actions))
	for index, raw := range req.Actions {
		action, f := b.validateAction(index, raw)
		if f != nil {
			return plan, f
		}
		actions = append(actions, action)
	}

	plan.ID = b.newID()
	plan.Actions = actions
	plan.Risk = AssessRisk(actions)
	plan.FreshApproval = req.FreshApproval
	return plan, nil
}

func (b Builder) newID() string {
	if b.NewID != nil {
		return b.NewID()
	}
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "plan-0000000000000000"
	}
	return "plan-" + hex.EncodeToString(buf)
}

func (b Builder) validateAction(index int, raw contract.MutationAction) (contract.MutationAction, *contract.Failure) {
	action := raw
	if strings.TrimSpace(action.ID) == "" || len(action.ID) > 128 {
		return action, contract.Fail(contract.CodeInvalidInput, "动作 id 不合法")
	}
	if !isMutationKind(action.Kind) {
		return action, contract.Fail(contract.CodeInvalidInput, "不支持的动作类型")
	}
	if action.Reason == "" {
		action.Reason = "用户操作"
	}
	if len([]rune(action.Reason)) > 500 {
		return action, contract.Fail(contract.CodeInvalidInput, "原因过长")
	}
	for _, rev := range []*int64{action.ExpectedSourceRevision, action.ExpectedTargetRevision} {
		if rev != nil && *rev < 0 {
			return action, contract.Fail(contract.CodeInvalidInput, "revision 不能为负数")
		}
	}
	if action.Text != nil && len([]byte(*action.Text)) > MaxWriteBytes {
		return action, contract.Fail(contract.CodeTooLarge, "正文超过 2 MiB")
	}

	var f *contract.Failure
	if action.Source, f = b.normalise("source", action.Source); f != nil {
		return action, f
	}
	if action.Target, f = b.normalise("target", action.Target); f != nil {
		return action, f
	}

	switch action.Kind {
	case contract.MutCreate:
		return b.validateCreate(action, false)
	case contract.MutCreateDirectory:
		return b.validateCreate(action, true)
	case contract.MutEdit:
		return b.validateEdit(action)
	case contract.MutMove:
		return b.validateMove(action)
	case contract.MutCopy:
		return b.validateCopy(action)
	case contract.MutArchive, contract.MutTrash, contract.MutPermanentDelete:
		if action.Source == "" {
			return action, contract.Fail(contract.CodeInvalidInput, "缺少源路径")
		}
		if _, f := b.lstat(action.Source); f != nil {
			return action, f
		}
		return action, nil
	}
	return action, contract.Fail(contract.CodeInvalidInput, "不支持的动作类型")
}

func (b Builder) normalise(field, rel string) (string, *contract.Failure) {
	if rel == "" {
		return "", nil
	}
	norm, f := pathguard.ValidateRel(rel)
	if f != nil {
		return "", f
	}
	return norm, nil
}

// lstat resolves a path inside the vault and reports whether it exists.
func (b Builder) lstat(rel string) (os.FileInfo, *contract.Failure) {
	abs, f := pathguard.Resolve(b.Root, rel)
	if f != nil {
		return nil, f
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return nil, contract.Fail(contract.CodeNotFound, "路径不存在")
	}
	return info, nil
}

func (b Builder) exists(rel string) bool {
	abs, f := pathguard.Resolve(b.Root, rel)
	if f != nil {
		return false
	}
	_, err := os.Lstat(abs)
	return err == nil
}

// siblings lists the names in the parent directory of rel.
func (b Builder) siblings(rel string) ([]string, *contract.Failure) {
	parent := parentOf(rel)
	abs, f := pathguard.Resolve(b.Root, parent)
	if f != nil {
		return nil, f
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, contract.Fail(contract.CodeNotADirectory, "目录不存在")
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names, nil
}

// validateCreate checks a new file or directory.
func (b Builder) validateCreate(action contract.MutationAction, directory bool) (contract.MutationAction, *contract.Failure) {
	if action.Target == "" {
		return action, contract.Fail(contract.CodeInvalidInput, "缺少目标路径")
	}
	parent := parentOf(action.Target)
	if parent != "" {
		info, f := b.lstat(parent)
		if f != nil {
			return action, contract.Fail(contract.CodeNotADirectory, "目标目录不存在")
		}
		if !info.IsDir() {
			return action, contract.Fail(contract.CodeNotADirectory, "目标目录不存在")
		}
	}
	if b.exists(action.Target) {
		return action, contract.Fail(contract.CodeNameConflict, "已存在同名项")
	}
	name := baseName(action.Target)
	siblings, f := b.siblings(action.Target)
	if f != nil {
		return action, f
	}
	checked, f := CheckEntryName(name, siblings)
	if f != nil {
		return action, f
	}
	if !directory {
		checked = WithDefaultExtension(checked)
	}
	action.Target = joinRel(parent, checked)
	return action, nil
}

// validateEdit checks an in-place text write. Like the legacy service, edit
// carries the file in target (source is ignored) and writes through the same
// revision-checked path as a normal save.
func (b Builder) validateEdit(action contract.MutationAction) (contract.MutationAction, *contract.Failure) {
	if action.Target == "" || action.Text == nil {
		return action, contract.Fail(contract.CodeInvalidInput, "编辑需要目标路径与正文")
	}
	info, f := b.lstat(action.Target)
	if f != nil {
		return action, f
	}
	if info.IsDir() {
		return action, contract.Fail(contract.CodeNotAFile, "目标不是文件")
	}
	return action, nil
}

func (b Builder) validateMove(action contract.MutationAction) (contract.MutationAction, *contract.Failure) {
	if action.Source == "" || action.Target == "" {
		return action, contract.Fail(contract.CodeInvalidInput, "移动需要源与目标")
	}
	if action.Source == action.Target {
		return action, contract.Fail(contract.CodeInvalidInput, "源与目标相同")
	}
	info, f := b.lstat(action.Source)
	if f != nil {
		return action, f
	}
	// A directory cannot be moved into itself or its own subtree.
	if info.IsDir() && strings.HasPrefix(action.Target, action.Source+"/") {
		return action, contract.Fail(contract.CodeInvalidInput, "不能移动到自身或子目录")
	}
	// Missing target parents are fine: execution creates them (legacy
	// doMoveOrCopy mkdirs recursively), so the plan only checks the leaf.
	parent := parentOf(action.Target)
	if b.exists(action.Target) {
		// The single allowed collision: renaming only the letter case.
		caseOnly := strings.EqualFold(action.Source, action.Target) &&
			parentOf(action.Source) == parentOf(action.Target)
		if !caseOnly {
			return action, contract.Fail(contract.CodeNameConflict, "目标已存在")
		}
	}
	name := baseName(action.Target)
	var siblings []string
	if parent == "" || b.exists(parent) {
		// A missing target parent has no siblings to collide with; execution
		// creates the directory (legacy doMoveOrCopy mkdirs recursively).
		var f *contract.Failure
		siblings, f = b.siblings(action.Target)
		if f != nil {
			return action, f
		}
	}
	// The source itself is not a collision for a rename in place.
	filtered := make([]string, 0, len(siblings))
	for _, sibling := range siblings {
		if strings.EqualFold(joinRel(parentOf(action.Source), sibling), action.Source) &&
			parentOf(action.Source) == parentOf(action.Target) {
			continue
		}
		filtered = append(filtered, sibling)
	}
	checked, f := CheckEntryName(name, filtered)
	if f != nil {
		return action, f
	}
	action.Target = joinRel(parent, checked)
	return action, nil
}

func (b Builder) validateCopy(action contract.MutationAction) (contract.MutationAction, *contract.Failure) {
	if action.Source == "" || action.Target == "" {
		return action, contract.Fail(contract.CodeInvalidInput, "复制需要源与目标")
	}
	if _, f := b.lstat(action.Source); f != nil {
		return action, f
	}
	if strings.HasPrefix(action.Target, action.Source+"/") {
		return action, contract.Fail(contract.CodeInvalidInput, "不能复制到自身或子目录")
	}
	if b.exists(action.Target) {
		return action, contract.Fail(contract.CodeNameConflict, "目标已存在")
	}
	// As with move, execution creates missing target parents.
	return action, nil
}

func parentOf(rel string) string {
	if at := strings.LastIndex(rel, "/"); at >= 0 {
		return rel[:at]
	}
	return ""
}

func baseName(rel string) string {
	if at := strings.LastIndex(rel, "/"); at >= 0 {
		return rel[at+1:]
	}
	return rel
}

func joinRel(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + "/" + name
}

// RootFor reports the filesystem root used by the builder (for callers that
// need to re-check state at execution time).
func (b Builder) RootFor() string { return filepath.Clean(b.Root) }

var _ = vault.AreaOther
