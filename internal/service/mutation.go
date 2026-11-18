package service

import (
	"context"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/mutation"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/pathguard"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/vault"
)

// MutationService turns user actions into confirmed change plans (API.md M20–
// M23, M29). Planning never writes to the vault.
type MutationService struct{ app *App }

// NewMutationService binds one app.
func NewMutationService(app *App) *MutationService { return &MutationService{app: app} }

// Execute applies a confirmed plan (M30). Edits go through the same
// revision-checked writer as a normal save; trash uses the system recycle bin.
func (s *MutationService) Execute(ctx context.Context, plan contract.MutationPlan) contract.MutationReport {
	report := contract.MutationReport{PlanID: plan.ID, Status: "failed",
		Completed: []contract.ActionRecord{}, Failed: []contract.ActionRecord{},
		Remaining: []contract.ActionRecord{}, TouchedDirectories: []string{}}
	if f := validateID(plan.VaultID); f != nil {
		report.Failure = f
		return report
	}
	root, f := s.app.Vaults.Root(plan.VaultID)
	if f != nil {
		report.Failure = f
		return report
	}
	executor := mutation.Executor{
		Root:    root,
		VaultID: plan.VaultID,
		Manager: mutation.TxManager{Root: root},
		Trash: func(abs string) error {
			return mygo.Shell.TrashItem(abs)
		},
		RecordProvenance: func(copyRel, sourceRel string) {
			snap, rf := vault.Read(root, sourceRel)
			if rf != nil {
				return // no baseline: a record without baseHash would be unknown forever
			}
			store, err := mutation.OpenProvenance(root)
			if err != nil {
				return
			}
			_ = store.Record(copyRel, sourceRel, snap.Revision, snap.Hash)
		},
		WriteText: func(rel string, expectedRevision int64, text string) (string, *contract.Failure) {
			outcome, wf := vault.Write(root, rel, expectedRevision, text)
			if wf != nil {
				return "", wf
			}
			if outcome.Status == "conflict" {
				return "", contract.Fail(contract.CodeRevisionMismatch, "目标已被外部修改")
			}
			return outcome.Hash, nil
		},
	}
	report = executor.Execute(plan)
	if a := s.app; a.Audit != nil {
		records := make([]contract.ActionRecord, 0, len(report.Completed)+len(report.Failed)+len(report.Remaining))
		records = append(records, report.Completed...)
		records = append(records, report.Failed...)
		records = append(records, report.Remaining...)
		if err := a.Audit.Append(records...); err != nil {
			_ = AppNotice.Emit(a.Window(), contract.AppNoticeEvent{Text: "操作日志写入失败：" + err.Error(), Level: "warn"})
		} else if len(records) > 0 {
			a.emitAudit(plan.VaultID)
		}
	}
	// A successful edit of a copy whose new content equals its source is the
	// provenance sync: the baseline moves to the source's current version
	// (D-009; a manual edit that happens to match is equally "in sync").
	if store, err := mutation.OpenProvenance(root); err == nil {
		for _, record := range report.Completed {
			if record.Kind != contract.MutEdit || record.Target == "" {
				continue
			}
			entry, ok := store.Get(record.Target)
			if !ok {
				continue
			}
			source, sf := vault.Read(root, entry.SourcePath)
			if sf != nil || source.Hash != record.AfterHash {
				continue
			}
			_ = store.UpdateBaseline(record.Target, source.Revision, source.Hash)
		}
	}

	// Tell the index what changed instead of waiting for the watcher debounce
	// (legacy register.ts updated the index right after execute).
	if len(report.TouchedDirectories) > 0 {
		s.app.enqueueIndex(plan.VaultID, report.TouchedDirectories)
	}
	// Action ledger: the index update is queued first, then the completed
	// actions enter the shared context (legacy register.ts:462-475 order).
	if s.app.Context != nil && len(report.Completed) > 0 {
		summaries := make([]contract.ContextActionSummary, 0, len(report.Completed))
		for _, record := range report.Completed {
			at := record.CompletedAt
			if at == "" {
				at = record.CreatedAt
			}
			summaries = append(summaries, contract.ContextActionSummary{
				ID: record.ID, Kind: record.Kind, Source: record.Source, Target: record.Target,
				Status: record.Status, At: at,
			})
		}
		s.app.Context.RecordActions(summaries)
	}
	return report
}

// AuditList returns operation records and the vault's transactions (M31).
func (s *MutationService) AuditList(ctx context.Context, vaultID, planID string, limit int) contract.AuditListResult {
	result := contract.AuditListResult{Records: []contract.ActionRecord{}, Transactions: []contract.TransactionSummary{}}
	if vaultID != "" {
		if f := validateID(vaultID); f != nil {
			result.Failure = f
			return result
		}
	}
	if s.app.Audit == nil {
		result.Failure = contract.Fail(contract.CodeIOFailure, "操作日志不可用")
		return result
	}
	result.Records = s.app.Audit.List(vaultID, planID, limit)
	if vaultID != "" {
		if root, f := s.app.Vaults.Root(vaultID); f == nil {
			manager := mutation.TxManager{Root: root}
			for _, manifest := range manager.ListTx() {
				result.Transactions = append(result.Transactions, contract.TransactionSummary{
					ID: manifest.ID, PlanID: manifest.PlanID, CreatedAt: manifest.CreatedAt,
					Status: manifest.Status, EntryCount: len(manifest.Entries),
				})
			}
		}
	}
	return result
}

// RewindPrepare reports which entries of a transaction can still be restored (M32).
func (s *MutationService) RewindPrepare(ctx context.Context, vaultID, transactionID string) contract.RestorePreview {
	result := contract.RestorePreview{TransactionID: transactionID, VaultID: vaultID, Entries: []contract.RestorePreviewEntry{}}
	if f := validateID(vaultID); f != nil {
		result.Failure = f
		return result
	}
	root, f := s.app.Vaults.Root(vaultID)
	if f != nil {
		result.Failure = f
		return result
	}
	manager := mutation.TxManager{Root: root}
	entries, pf := manager.Preview(transactionID)
	if pf != nil {
		result.Failure = pf
		return result
	}
	result.Entries = entries
	return result
}

// RewindCommit restores the transaction and marks the matching audit records
// (M33). Entries changed after the transaction are reported, not overwritten.
func (s *MutationService) RewindCommit(ctx context.Context, vaultID, transactionID string) contract.RestoreResult {
	result := contract.RestoreResult{TransactionID: transactionID, Restored: []string{},
		Conflicted: []string{}, TouchedDirectories: []string{}}
	if f := validateID(vaultID); f != nil {
		result.Failure = f
		return result
	}
	root, f := s.app.Vaults.Root(vaultID)
	if f != nil {
		result.Failure = f
		return result
	}
	manager := mutation.TxManager{Root: root}
	restored, conflicted, touched, rf := manager.Restore(transactionID)
	if rf != nil {
		result.Failure = rf
		return result
	}
	result.Restored = restored
	result.Conflicted = conflicted
	result.TouchedDirectories = touched

	if a := s.app; a.Audit != nil {
		var revertedIDs, conflictedIDs []string
		for _, record := range a.Audit.List(vaultID, "", 1000) {
			if record.TransactionID != transactionID {
				continue
			}
			switch {
			case containsPath(conflicted, record.Target), containsPath(conflicted, record.Source):
				conflictedIDs = append(conflictedIDs, record.ID)
			case containsPath(restored, record.Target), containsPath(restored, record.Source):
				revertedIDs = append(revertedIDs, record.ID)
			}
		}
		_, _ = a.Audit.UpdateStatus(revertedIDs, "reverted")
		_, _ = a.Audit.UpdateStatus(conflictedIDs, "conflicted")
		if len(revertedIDs)+len(conflictedIDs) > 0 {
			a.emitAudit(vaultID)
		}
	}
	return result
}

// ProvenanceList returns every copy -> source relationship with its current
// state (M34).
func (s *MutationService) ProvenanceList(ctx context.Context, vaultID string) contract.ProvenanceListResult {
	result := contract.ProvenanceListResult{Entries: []contract.ProvenanceRecordShape{}}
	if f := validateID(vaultID); f != nil {
		result.Failure = f
		return result
	}
	root, f := s.app.Vaults.Root(vaultID)
	if f != nil {
		result.Failure = f
		return result
	}
	store, err := mutation.OpenProvenance(root)
	if err != nil {
		result.Failure = contract.Fail(contract.CodeIOFailure, "读取来源记录失败")
		return result
	}
	for _, record := range store.All() {
		result.Entries = append(result.Entries, s.provenanceShape(root, record))
	}
	return result
}

// ProvenanceState returns the relationship for one path (M35); a nil entry
// means the file has no recorded source.
func (s *MutationService) ProvenanceState(ctx context.Context, vaultID, path string) contract.ProvenanceStateResult {
	result := contract.ProvenanceStateResult{}
	if f := validateID(vaultID); f != nil {
		result.Failure = f
		return result
	}
	norm, f := pathguard.ValidateRel(path)
	if f != nil {
		result.Failure = f
		return result
	}
	root, f := s.app.Vaults.Root(vaultID)
	if f != nil {
		result.Failure = f
		return result
	}
	store, err := mutation.OpenProvenance(root)
	if err != nil {
		result.Failure = contract.Fail(contract.CodeIOFailure, "读取来源记录失败")
		return result
	}
	if record, ok := store.Get(norm); ok {
		shape := s.provenanceShape(root, record)
		result.Entry = &shape
	}
	return result
}

func (s *MutationService) provenanceShape(root string, record mutation.ProvenanceRecord) contract.ProvenanceRecordShape {
	var sourceHash, copyHash *string
	if snap, f := vault.Read(root, record.SourcePath); f == nil {
		hash := snap.Hash
		sourceHash = &hash
	}
	if snap, f := vault.Read(root, record.CopyPath); f == nil {
		hash := snap.Hash
		copyHash = &hash
	}
	state, canSync, reason := mutation.Assess(record, sourceHash, copyHash)
	return contract.ProvenanceRecordShape{
		CopyPath: record.CopyPath, SourcePath: record.SourcePath,
		BaseRevision: record.BaseRevision, BaseHash: record.BaseHash, CopiedAt: record.CopiedAt,
		Assessment: contract.ProvenanceAssessment{State: state, CanSyncFromSource: canSync, Reason: reason},
	}
}

func containsPath(list []string, path string) bool {
	if path == "" {
		return false
	}
	for _, item := range list {
		if item == path {
			return true
		}
	}
	return false
}

// emitAudit pushes E03 with a monotonically increasing sequence.
func (a *App) emitAudit(vaultID string) {
	a.mu.Lock()
	a.auditSeq++
	seq := a.auditSeq
	win := a.win
	a.mu.Unlock()
	if win != nil {
		_ = AuditChanged.Emit(win, contract.AuditChangedEvent{VaultID: vaultID, Sequence: seq})
	}
}

// Plan validates a change plan (M29).
func (s *MutationService) Plan(ctx context.Context, req contract.PlanRequest) contract.PlanResult {
	return s.build(req)
}

// Rename plans a move that only changes the entry name (M20).
func (s *MutationService) Rename(ctx context.Context, vaultID, path, newName string) contract.PlanResult {
	return s.build(contract.PlanRequest{
		VaultID: vaultID,
		Origin:  "user",
		Actions: []contract.MutationAction{{
			ID: "a1", Kind: contract.MutMove, Source: path,
			Target: joinRel(parentOf(path), newName), Reason: "用户重命名",
		}},
	})
}

// CreateFile plans a new file inside dirPath (M21).
func (s *MutationService) CreateFile(ctx context.Context, vaultID, dirPath, name string) contract.PlanResult {
	return s.build(contract.PlanRequest{
		VaultID: vaultID,
		Origin:  "user",
		Actions: []contract.MutationAction{{
			ID: "a1", Kind: contract.MutCreate,
			Target: joinRel(dirPath, name), Reason: "用户新建文件",
		}},
	})
}

// CreateDir plans a new directory inside dirPath (M22).
func (s *MutationService) CreateDir(ctx context.Context, vaultID, dirPath, name string) contract.PlanResult {
	return s.build(contract.PlanRequest{
		VaultID: vaultID,
		Origin:  "user",
		Actions: []contract.MutationAction{{
			ID: "a1", Kind: contract.MutCreateDirectory,
			Target: joinRel(dirPath, name), Reason: "用户新建目录",
		}},
	})
}

// PlanMove plans moving several entries into targetDir (M23).
func (s *MutationService) PlanMove(ctx context.Context, vaultID string, sources []string, targetDir string) contract.PlanResult {
	actions := make([]contract.MutationAction, 0, len(sources))
	for index, source := range sources {
		actions = append(actions, contract.MutationAction{
			ID: "a" + strconv.Itoa(index+1), Kind: contract.MutMove, Source: source,
			Target: joinRel(targetDir, baseName(source)), Reason: "用户移动",
		})
	}
	return s.build(contract.PlanRequest{VaultID: vaultID, Origin: "user", Actions: actions})
}

func (s *MutationService) build(req contract.PlanRequest) contract.PlanResult {
	if f := validateID(req.VaultID); f != nil {
		return contract.PlanResult{Failure: f}
	}
	root, f := s.app.Vaults.Root(req.VaultID)
	if f != nil {
		return contract.PlanResult{Failure: f}
	}
	builder := mutation.Builder{Root: root}
	plan, failure := builder.Plan(req)
	if failure != nil {
		return contract.PlanResult{Failure: failure}
	}
	return contract.PlanResult{Plan: &plan}
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
