package mutation

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/pathguard"
)

// Executor applies a confirmed plan (API.md M30). Every write goes through the
// vault root and, for edit, through the same revision-checked writer as a
// normal save.
type Executor struct {
	Root    string
	VaultID string
	Manager TxManager
	// Trash moves a path to the system recycle bin (injected: mygo in prod).
	Trash func(abs string) error
	// WriteText saves text with the normal BOM/EOL rules and returns the new
	// hash; a non-nil failure aborts the action.
	WriteText func(rel string, expectedRevision int64, text string) (string, *contract.Failure)
	// RecordProvenance is called after a successful copy into projects/.
	RecordProvenance func(copyRel, sourceRel string)
	Now              func() time.Time
}

func (e Executor) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

// Execute runs the plan and reports completed, failed and skipped actions.
func (e Executor) Execute(plan contract.MutationPlan) contract.MutationReport {
	report := contract.MutationReport{
		PlanID: plan.ID, Completed: []contract.ActionRecord{}, Failed: []contract.ActionRecord{},
		Remaining: []contract.ActionRecord{}, TouchedDirectories: []string{},
	}
	if len(plan.Actions) == 0 {
		report.Status = "failed"
		report.Failure = contract.Fail(contract.CodeInvalidInput, "计划不能为空")
		return report
	}
	if len(plan.Actions) > MaxPlanActions {
		report.Status = "failed"
		report.Failure = contract.Fail(contract.CodeInvalidInput, "单个计划最多 500 项")
		return report
	}
	// Fresh approval is a precondition of the whole run: a plan containing a
	// permanent delete without it never opens a transaction (legacy
	// sanitizeMutationPlan rejects it before execution).
	if hasPermanentDelete(plan.Actions) && !plan.FreshApproval {
		report.Status = "failed"
		report.Failure = contract.Fail(contract.CodePermissionDenied, "永久删除需要单独确认")
		return report
	}

	var tx *Tx
	if needsTransaction(plan.Actions) {
		opened, err := e.Manager.Begin(plan.VaultID, plan.ID)
		if err != nil {
			report.Status = "failed"
			report.Failure = contract.Fail(contract.CodeIOFailure, "无法创建事务："+err.Error())
			return report
		}
		tx = opened
	}

	touched := map[string]bool{}
	stopped := false
	for _, action := range plan.Actions {
		if stopped {
			report.Remaining = append(report.Remaining, e.record(plan, action, "skipped", tx, actionOutcome{}))
			continue
		}
		outcome, err := e.runAction(plan, action, tx)
		if err != nil {
			record := e.record(plan, action, "failed", tx, actionOutcome{})
			record.ErrorCode = failureCode(err)
			record.ErrorMessage = err.Error()
			record.CompletedAt = e.now().UTC().Format(time.RFC3339Nano)
			report.Failed = append(report.Failed, record)
			stopped = true
			continue
		}
		for _, dir := range outcome.touched {
			touched[dir] = true
		}
		report.Completed = append(report.Completed, e.record(plan, action, "completed", tx, outcome))
	}

	if tx != nil {
		if len(report.Failed) > 0 {
			_ = e.Manager.MarkPartial(tx, fmt.Sprintf("%d 项失败，%d 项已完成", len(report.Failed), len(report.Completed)))
		} else if err := e.Manager.CommitWithAfterImages(tx); err != nil {
			report.Status = "partial"
			report.Failure = contract.Fail(contract.CodeIOFailure, "事务提交失败："+err.Error())
			report.TransactionID = tx.ID
			report.TouchedDirectories = sortedKeys(touched)
			return report
		}
		report.TransactionID = tx.ID
	}

	switch {
	case len(report.Failed) == 0:
		report.Status = "completed"
	case len(report.Completed) == 0:
		report.Status = "failed"
	default:
		report.Status = "partial"
	}
	report.TouchedDirectories = sortedKeys(touched)
	return report
}

type actionOutcome struct {
	target     string
	beforeHash string
	afterHash  string
	touched    []string
}

func (e Executor) record(plan contract.MutationPlan, action contract.MutationAction, status string, tx *Tx, outcome actionOutcome) contract.ActionRecord {
	// A record id must be unique across plans: M20–M23 reuse "a1", and the
	// ledger upserts by id, so planID:actionID alone would overwrite history.
	recordID := plan.ID + ":" + action.ID + ":" + e.Manager.randomHex(6)
	record := contract.ActionRecord{
		ID: recordID, PlanID: plan.ID, VaultID: plan.VaultID, Origin: plan.Origin,
		Kind: action.Kind, Source: action.Source, Target: action.Target, Status: status,
		CreatedAt: e.now().UTC().Format(time.RFC3339Nano),
	}
	if outcome.target != "" {
		record.Target = outcome.target
	}
	record.BeforeHash = outcome.beforeHash
	record.AfterHash = outcome.afterHash
	if tx != nil {
		record.TransactionID = tx.ID
	}
	if status == "completed" {
		record.CompletedAt = e.now().UTC().Format(time.RFC3339Nano)
	}
	return record
}

func (e Executor) runAction(plan contract.MutationPlan, action contract.MutationAction, tx *Tx) (actionOutcome, error) {
	switch action.Kind {
	case contract.MutCreate:
		return e.doCreate(action, false)
	case contract.MutCreateDirectory:
		return e.doCreate(action, true)
	case contract.MutEdit:
		return e.doEdit(action, tx)
	case contract.MutMove:
		return e.doMoveOrCopy(action, tx, true)
	case contract.MutCopy:
		return e.doMoveOrCopy(action, tx, false)
	case contract.MutArchive:
		return e.doArchive(action, tx)
	case contract.MutTrash:
		return e.doTrash(action, tx)
	case contract.MutPermanentDelete:
		return e.doPermanentDelete(plan, action, tx)
	}
	return actionOutcome{}, contractError{code: contract.CodeUnsupported, message: "不支持的操作类型"}
}

// abs resolves a plan path through pathguard, so every existing segment is
// checked for junctions and reparse points at execution time (the plan layer's
// check is not enough: M30 receives the plan from the renderer).
func (e Executor) abs(rel string) (string, error) {
	if f := assertSafeRelative(rel); f != nil {
		return "", contractError{code: f.Code, message: f.Message}
	}
	abs, f := pathguard.Resolve(e.Root, rel)
	if f != nil {
		return "", contractError{code: f.Code, message: f.Message}
	}
	return abs, nil
}

func (e Executor) doCreate(action contract.MutationAction, directory bool) (actionOutcome, error) {
	abs, err := e.abs(action.Target)
	if err != nil {
		return actionOutcome{}, err
	}
	if _, statErr := os.Lstat(abs); statErr == nil {
		return actionOutcome{}, contractError{code: contract.CodeInvalidInput, message: "目标已存在"}
	}
	if directory {
		if err := os.MkdirAll(abs, 0o755); err != nil {
			return actionOutcome{}, err
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return actionOutcome{}, err
		}
		text := ""
		if action.Text != nil {
			text = *action.Text
		}
		hash, f := e.writeText(action.Target, 0, text)
		if f != nil {
			return actionOutcome{}, contractError{code: f.Code, message: f.Message}
		}
		return actionOutcome{touched: []string{parentOf(action.Target)}, afterHash: hash}, nil
	}
	return actionOutcome{touched: []string{parentOf(action.Target)}}, nil
}

func (e Executor) doEdit(action contract.MutationAction, tx *Tx) (actionOutcome, error) {
	abs, err := e.abs(action.Target)
	if err != nil {
		return actionOutcome{}, err
	}
	before, _ := hashFile(abs)
	if tx != nil {
		if err := e.Manager.Capture(tx, action.Target, abs); err != nil {
			return actionOutcome{}, err
		}
	}
	text := ""
	if action.Text != nil {
		text = *action.Text
	}
	expected := int64(0)
	if action.ExpectedTargetRevision != nil {
		expected = *action.ExpectedTargetRevision
	}
	hash, f := e.writeText(action.Target, expected, text)
	if f != nil {
		return actionOutcome{}, contractError{code: f.Code, message: f.Message}
	}
	return actionOutcome{touched: []string{parentOf(action.Target)}, beforeHash: before, afterHash: hash}, nil
}

func (e Executor) writeText(rel string, expectedRevision int64, text string) (string, *contract.Failure) {
	if e.WriteText == nil {
		return "", contract.Fail(contract.CodeIOFailure, "写入器未注入")
	}
	return e.WriteText(rel, expectedRevision, text)
}

func (e Executor) doMoveOrCopy(action contract.MutationAction, tx *Tx, isMove bool) (actionOutcome, error) {
	sourceAbs, err := e.abs(action.Source)
	if err != nil {
		return actionOutcome{}, err
	}
	targetAbs, err := e.abs(action.Target)
	if err != nil {
		return actionOutcome{}, err
	}
	info, statErr := os.Lstat(sourceAbs)
	if statErr != nil {
		return actionOutcome{}, contractError{code: contract.CodeNotFound, message: "源不存在"}
	}
	// A case-only rename is legal: on Windows it is the same file.
	caseOnly := isMove && strings.EqualFold(action.Source, action.Target)
	if !caseOnly {
		if _, err := os.Lstat(targetAbs); err == nil {
			return actionOutcome{}, contractError{code: contract.CodeInvalidInput, message: "目标已存在"}
		}
	}
	before, _ := hashFile(sourceAbs)
	if tx != nil {
		if err := e.Manager.Capture(tx, action.Source, sourceAbs); err != nil {
			return actionOutcome{}, err
		}
		if err := e.Manager.Capture(tx, action.Target, targetAbs); err != nil {
			return actionOutcome{}, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(targetAbs), 0o755); err != nil {
		return actionOutcome{}, err
	}
	if isMove {
		if caseOnly {
			// Windows treats both names as the same file; a direct rename does
			// not change the letter case, so hop through a temporary name.
			tmp := sourceAbs + ".case-rename"
			if err := os.Rename(sourceAbs, tmp); err != nil {
				return actionOutcome{}, err
			}
			if err := os.Rename(tmp, targetAbs); err != nil {
				return actionOutcome{}, err
			}
		} else if err := os.Rename(sourceAbs, targetAbs); err != nil {
			return actionOutcome{}, err
		}
	} else if info.IsDir() {
		if err := copyDirectory(sourceAbs, targetAbs); err != nil {
			return actionOutcome{}, err
		}
	} else {
		if err := copyFileVerified(sourceAbs, targetAbs); err != nil {
			return actionOutcome{}, err
		}
	}
	if !isMove && info.Mode().IsRegular() && ShouldTrackProvenance(action.Target) && e.RecordProvenance != nil {
		// Only files: a directory copy has no single baseline hash.
		e.RecordProvenance(action.Target, action.Source)
	}
	after, _ := hashFile(targetAbs)
	return actionOutcome{
		target: action.Target, beforeHash: before, afterHash: after,
		touched: []string{parentOf(action.Source), parentOf(action.Target)},
	}, nil
}

func (e Executor) doArchive(action contract.MutationAction, tx *Tx) (actionOutcome, error) {
	sourceAbs, err := e.abs(action.Source)
	if err != nil {
		return actionOutcome{}, err
	}
	info, statErr := os.Lstat(sourceAbs)
	if statErr != nil {
		return actionOutcome{}, contractError{code: contract.CodeNotFound, message: "源不存在"}
	}
	stamp := e.now().UTC().Format("2006-01-02T15:04:05.000Z")
	stamp = strings.NewReplacer(":", "-", ".", "-").Replace(stamp)
	base := "_archives/" + action.Source
	targetRel := base
	if _, err := os.Lstat(filepath.Join(e.Root, filepath.FromSlash(base))); err == nil {
		dot := strings.LastIndex(base, ".")
		slash := strings.LastIndex(base, "/")
		if dot > slash {
			targetRel = base[:dot] + "." + stamp + base[dot:]
		} else {
			targetRel = base + "." + stamp
		}
	}
	targetAbs, err := e.abs(targetRel)
	if err != nil {
		return actionOutcome{}, err
	}
	before, _ := hashFile(sourceAbs)
	if tx != nil {
		if err := e.Manager.Capture(tx, action.Source, sourceAbs); err != nil {
			return actionOutcome{}, err
		}
		if err := e.Manager.Capture(tx, targetRel, targetAbs); err != nil {
			return actionOutcome{}, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(targetAbs), 0o755); err != nil {
		return actionOutcome{}, err
	}
	if err := os.Rename(sourceAbs, targetAbs); err != nil {
		return actionOutcome{}, err
	}
	_ = info
	return actionOutcome{
		target: targetRel, beforeHash: before,
		touched: []string{parentOf(action.Source), parentOf(targetRel)},
	}, nil
}

func (e Executor) doTrash(action contract.MutationAction, tx *Tx) (actionOutcome, error) {
	sourceAbs, err := e.abs(action.Source)
	if err != nil {
		return actionOutcome{}, err
	}
	info, statErr := os.Lstat(sourceAbs)
	if statErr != nil {
		return actionOutcome{}, contractError{code: contract.CodeNotFound, message: "源不存在"}
	}
	before, _ := hashFile(sourceAbs)
	if tx != nil {
		if err := e.Manager.Capture(tx, action.Source, sourceAbs); err != nil {
			return actionOutcome{}, err
		}
	}
	if e.Trash == nil {
		return actionOutcome{}, contractError{code: contract.CodeIOFailure, message: "回收站不可用"}
	}
	if err := e.Trash(sourceAbs); err != nil {
		return actionOutcome{}, err
	}
	_ = info
	return actionOutcome{touched: []string{parentOf(action.Source)}, beforeHash: before}, nil
}

func (e Executor) doPermanentDelete(plan contract.MutationPlan, action contract.MutationAction, tx *Tx) (actionOutcome, error) {
	if !plan.FreshApproval {
		return actionOutcome{}, contractError{code: contract.CodePermissionDenied, message: "永久删除需要单独确认"}
	}
	sourceAbs, err := e.abs(action.Source)
	if err != nil {
		return actionOutcome{}, err
	}
	if _, statErr := os.Lstat(sourceAbs); statErr != nil {
		return actionOutcome{}, contractError{code: contract.CodeNotFound, message: "源不存在"}
	}
	before, _ := hashFile(sourceAbs)
	if tx != nil {
		if err := e.Manager.Capture(tx, action.Source, sourceAbs); err != nil {
			return actionOutcome{}, err
		}
	}
	if err := os.RemoveAll(sourceAbs); err != nil {
		return actionOutcome{}, err
	}
	return actionOutcome{touched: []string{parentOf(action.Source)}, beforeHash: before}, nil
}

func needsTransaction(actions []contract.MutationAction) bool {
	for _, action := range actions {
		if destructiveKinds[action.Kind] {
			return true
		}
	}
	return false
}

func hasPermanentDelete(actions []contract.MutationAction) bool {
	for _, action := range actions {
		if action.Kind == contract.MutPermanentDelete {
			return true
		}
	}
	return false
}

func copyFileVerified(source, target string) error {
	sourceHash, err := hashFile(source)
	if err != nil {
		return err
	}
	if err := copyFile(source, target); err != nil {
		return err
	}
	targetHash, err := hashFile(target)
	if err != nil {
		return err
	}
	if sourceHash != targetHash {
		return contractError{code: contract.CodeIOFailure, message: "复制后内容校验不一致"}
	}
	return nil
}

func copyFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// copyDirectory copies recursively and verifies every file's hash.
func copyDirectory(source, target string) error {
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(source, entry.Name())
		dstPath := filepath.Join(target, entry.Name())
		if entry.IsDir() {
			if err := copyDirectory(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		if err := copyFileVerified(srcPath, dstPath); err != nil {
			return err
		}
	}
	return nil
}

// contractError carries a failure code through the error channel.
type contractError struct {
	code    string
	message string
}

func (e contractError) Error() string { return e.message }

func failureCode(err error) string {
	if ce, ok := err.(contractError); ok {
		return ce.code
	}
	return contract.CodeIOFailure
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
