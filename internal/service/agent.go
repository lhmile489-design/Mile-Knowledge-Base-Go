package service

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"regexp"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/pathguard"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/vault"
)

// AgentService binds the shared context hub (API.md M45–M49). The turn-loop
// binding (M38–M44) arrives with T10.
type AgentService struct{ app *App }

// NewAgentService binds one app.
func NewAgentService(app *App) *AgentService { return &AgentService{app: app} }

var (
	// referenceId / taskId character sets come from validate.ts:169-184.
	referenceIDPattern = regexp.MustCompile(`^[A-Za-z0-9_:-]+$`)
	taskIDPattern      = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
)

func nowISO() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
}

// truncateRunes slices to max runes without an ellipsis, mirroring the legacy
// assertString overflow behaviour (validate.ts:74-83).
func truncateRunes(text string, max int) string {
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return string(runes[:max])
}

// pinIDFor is the opaque pin identity: same vault and path always produce the
// same id, and the id never encodes path characters (register.ts:150-152).
func pinIDFor(vaultID, path string) string {
	sum := sha256.Sum256([]byte(vaultID + "\x00" + path))
	return "pin-" + hex.EncodeToString(sum[:])[:24]
}

func (s *AgentService) snapshotResult() contract.SharedContextResult {
	snapshot := s.app.Context.Snapshot()
	return contract.SharedContextResult{Context: &snapshot}
}

// ContextSnapshot is M45.
func (s *AgentService) ContextSnapshot(vaultID string) contract.SharedContextResult {
	if f := validateID(vaultID); f != nil {
		return contract.SharedContextResult{Failure: f}
	}
	s.app.syncContextVault(vaultID)
	return s.snapshotResult()
}

// ContextPin is M46: pin one file. The note is truncated at 500 characters,
// never rejected (API.md M45–M49).
func (s *AgentService) ContextPin(vaultID, path, note string) contract.SharedContextResult {
	if f := validateID(vaultID); f != nil {
		return contract.SharedContextResult{Failure: f}
	}
	root, f := s.app.Vaults.Root(vaultID)
	if f != nil {
		return contract.SharedContextResult{Failure: f}
	}
	normalized, f := pathguard.ValidateRel(path)
	if f != nil {
		return contract.SharedContextResult{Failure: f}
	}
	if normalized == "" {
		return contract.SharedContextResult{Failure: contract.Fail(contract.CodeInvalidInput, "path 不能为空")}
	}
	snapshot, f := vault.Read(root, normalized)
	if f != nil {
		return contract.SharedContextResult{Failure: f}
	}
	reference := contract.Reference{
		ID: pinIDFor(vaultID, normalized), Path: normalized,
		Revision: snapshot.Revision, PinnedAt: nowISO(),
	}
	if note != "" {
		reference.Note = truncateRunes(note, 500)
	}
	s.app.Context.Pin(reference)
	return s.snapshotResult()
}

// ContextUnpin is M47: remove one pinned reference.
func (s *AgentService) ContextUnpin(referenceID string) contract.SharedContextResult {
	id := truncateRunes(referenceID, 128)
	if id == "" {
		return contract.SharedContextResult{Failure: contract.Fail(contract.CodeInvalidInput, "referenceId 不能为空")}
	}
	if !referenceIDPattern.MatchString(id) {
		return contract.SharedContextResult{Failure: contract.Fail(contract.CodeInvalidInput, "referenceId 含非法字符")}
	}
	s.app.Context.Unpin(id)
	return s.snapshotResult()
}

// ContextSetTaskStatus is M48: the user may move any task.
func (s *AgentService) ContextSetTaskStatus(taskID, status string) contract.SharedContextResult {
	id := truncateRunes(taskID, 128)
	if id == "" {
		return contract.SharedContextResult{Failure: contract.Fail(contract.CodeInvalidInput, "taskId 不能为空")}
	}
	if !taskIDPattern.MatchString(id) {
		return contract.SharedContextResult{Failure: contract.Fail(contract.CodeInvalidInput, "taskId 含非法字符")}
	}
	if f := s.app.Context.SetTaskStatusByUser(id, status); f != nil {
		return contract.SharedContextResult{Failure: f}
	}
	return s.snapshotResult()
}

// ContextReportDocument is M49: the only derived state the frontend pushes.
// An empty path clears the active document (the legacy path=null case); the
// revision must be non-negative and selection is truncated at 200 characters.
// WindowService.ReportDocument stays the close-guard entry and does not
// replace this one.
func (s *AgentService) ContextReportDocument(vaultID, path string, revision int64, dirty bool, selection string) contract.Failure {
	if f := validateID(vaultID); f != nil {
		return *f
	}
	if path == "" {
		s.app.Context.SetActiveDocument(nil)
		return contract.Failure{}
	}
	normalized, f := pathguard.ValidateRel(path)
	if f != nil {
		return *f
	}
	if normalized == "" {
		return *contract.Fail(contract.CodeInvalidInput, "path 不能为空")
	}
	if revision < 0 {
		return *contract.Fail(contract.CodeInvalidInput, "revision 必须是非负整数")
	}
	document := &contract.ActiveDocumentContext{Path: normalized, Revision: revision, Dirty: dirty}
	if selection != "" {
		document.Selection = truncateRunes(selection, 200)
	}
	s.app.Context.SetActiveDocument(document)
	return contract.Failure{}
}

// syncContextVault refreshes the derived vault fields in the shared context.
// It is called from vault switches, watcher events and index updates; the hub
// only bumps its sequence when the values actually change.
func (a *App) syncContextVault(vaultID string) {
	if a.Context == nil || validateID(vaultID) != nil {
		return
	}
	root, f := a.Vaults.Root(vaultID)
	if f != nil {
		return
	}
	label := filepath.Base(root)
	for _, entry := range a.Vaults.List().Vaults {
		if entry.ID == vaultID && entry.DisplayName != "" {
			label = entry.DisplayName
			break
		}
	}
	a.mu.Lock()
	revision := a.vaultRevisions[vaultID]
	a.mu.Unlock()
	a.Context.SetVault(contract.VaultContext{
		ID: vaultID, RootLabel: label, Revision: revision,
		IndexRevision: a.indexStatus(vaultID).Revision,
	})
}

// bumpVaultRevision advances the per-vault change counter that feeds the
// context's vault revision (the legacy vault-changed sequence).
func (a *App) bumpVaultRevision(vaultID string) {
	a.mu.Lock()
	if a.vaultRevisions == nil {
		a.vaultRevisions = map[string]int64{}
	}
	a.vaultRevisions[vaultID]++
	a.mu.Unlock()
}
