package ai

import (
	"strings"
	"sync"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// Context limits (shared/context.ts:17-19); the tail budget is a hard cap.
const (
	ContextTailMaxChars      = 2400
	ContextMaxRetrievalItems = 4
	ContextMaxPinnedChars    = 1500
)

var openTaskStatuses = map[string]bool{
	contract.TaskQueued: true, contract.TaskRunning: true, contract.TaskBlocked: true,
}

// IsTaskStatus reports whether status is one of the five known states.
func IsTaskStatus(status string) bool {
	switch status {
	case contract.TaskQueued, contract.TaskRunning, contract.TaskBlocked, contract.TaskDone, contract.TaskCancelled:
		return true
	}
	return false
}

// IsOpenTask reports whether the task still occupies the injection budget.
func IsOpenTask(status string) bool { return openTaskStatuses[status] }

// HandoffRequest is one handoff_task call. The initiator is never taken from
// the model; the caller resolves it from the session.
type HandoffRequest struct {
	Owner       string
	Goal        string
	Inputs      []string
	Constraints []string
}

// ContextHub is the shared "current situation" of the four agents
// (services/ContextHub.ts). It stores derived state in memory only: vault
// revision, active document, pins, task board and action ledger. Agent
// transcripts stay per-session and are not stored here.
//
// Every change bumps sequence and replaces snapshotID, so a turn can record
// which snapshot it actually used. Collections are ordered slices of ids plus
// maps: upserting an existing id keeps its original position.
type ContextHub struct {
	mu         sync.Mutex
	sequence   int64
	snapshotID string
	vault      contract.VaultContext
	activeDoc  *contract.ActiveDocumentContext

	pins      map[string]contract.Reference
	pinOrder  []string
	tasks     map[string]contract.TaskItem
	taskOrder []string
	actions   map[string]contract.ContextActionSummary
	actOrder  []string

	listeners []func(contract.SharedContext)
	newID     func() string
	now       func() time.Time
}

// NewContextHub builds an empty hub.
func NewContextHub() *ContextHub {
	return &ContextHub{
		snapshotID: "snap-" + randomHex(12),
		pins:       map[string]contract.Reference{},
		tasks:      map[string]contract.TaskItem{},
		actions:    map[string]contract.ContextActionSummary{},
		newID:      func() string { return "task-" + randomHex(16) },
		now:        time.Now,
	}
}

func (h *ContextHub) nowISO() string {
	return h.now().UTC().Format("2006-01-02T15:04:05.000Z")
}

// Snapshot returns the current context. Arrays are always non-nil.
func (h *ContextHub) Snapshot() contract.SharedContext {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.snapshotLocked()
}

func (h *ContextHub) snapshotLocked() contract.SharedContext {
	out := contract.SharedContext{
		SnapshotID:       h.snapshotID,
		Sequence:         h.sequence,
		Vault:            h.vault,
		PinnedReferences: make([]contract.Reference, 0, len(h.pinOrder)),
		TaskBoard:        make([]contract.TaskItem, 0, len(h.taskOrder)),
		DecisionLog:      []contract.Decision{},
		ActionLedger:     make([]contract.ContextActionSummary, 0, len(h.actOrder)),
		RetrievalBundle:  []contract.RetrievalItem{},
	}
	if h.activeDoc != nil {
		doc := *h.activeDoc
		out.ActiveDocument = &doc
	}
	for _, id := range h.pinOrder {
		out.PinnedReferences = append(out.PinnedReferences, h.pins[id])
	}
	for _, id := range h.taskOrder {
		out.TaskBoard = append(out.TaskBoard, h.tasks[id])
	}
	for _, id := range h.actOrder {
		out.ActionLedger = append(out.ActionLedger, h.actions[id])
	}
	return out
}

// OnChanged subscribes to snapshot changes (E09 wiring happens in T10).
func (h *ContextHub) OnChanged(listener func(contract.SharedContext)) (cancel func()) {
	h.mu.Lock()
	h.listeners = append(h.listeners, listener)
	index := len(h.listeners) - 1
	h.mu.Unlock()
	return func() {
		h.mu.Lock()
		h.listeners[index] = nil
		h.mu.Unlock()
	}
}

// commit bumps the sequence, swaps the snapshot id and notifies listeners
// outside the lock. A panicking listener never blocks the others.
func (h *ContextHub) commit() {
	h.mu.Lock()
	h.sequence++
	h.snapshotID = "snap-" + randomHex(12)
	snapshot := h.snapshotLocked()
	listeners := make([]func(contract.SharedContext), 0, len(h.listeners))
	for _, listener := range h.listeners {
		if listener != nil {
			listeners = append(listeners, listener)
		}
	}
	h.mu.Unlock()
	for _, listener := range listeners {
		func() {
			defer func() { _ = recover() }()
			listener(snapshot)
		}()
	}
}

// SetVault stores the derived vault context. Repeating identical values is not
// a change: the service syncs it on every index update, and a sequence bump
// per sync would make the change marker meaningless.
func (h *ContextHub) SetVault(vault contract.VaultContext) {
	h.mu.Lock()
	changed := h.vault != vault
	if changed {
		h.vault = vault
	}
	h.mu.Unlock()
	if changed {
		h.commit()
	}
}

// SetActiveDocument records what the user is looking at; nil clears it.
func (h *ContextHub) SetActiveDocument(document *contract.ActiveDocumentContext) {
	h.mu.Lock()
	if document == nil {
		h.activeDoc = nil
	} else {
		doc := *document
		h.activeDoc = &doc
	}
	h.mu.Unlock()
	h.commit()
}

// Pin adds or replaces one user-pinned reference.
func (h *ContextHub) Pin(reference contract.Reference) {
	h.mu.Lock()
	if _, ok := h.pins[reference.ID]; !ok {
		h.pinOrder = append(h.pinOrder, reference.ID)
	}
	h.pins[reference.ID] = reference
	h.mu.Unlock()
	h.commit()
}

// Unpin removes one reference; a missing id is a no-op.
func (h *ContextHub) Unpin(referenceID string) {
	h.mu.Lock()
	_, ok := h.pins[referenceID]
	if ok {
		delete(h.pins, referenceID)
		for i, id := range h.pinOrder {
			if id == referenceID {
				h.pinOrder = append(h.pinOrder[:i], h.pinOrder[i+1:]...)
				break
			}
		}
	}
	h.mu.Unlock()
	if ok {
		h.commit()
	}
}

// RecordActions upserts the completed action summaries in one commit.
func (h *ContextHub) RecordActions(summaries []contract.ContextActionSummary) {
	if len(summaries) == 0 {
		return
	}
	h.mu.Lock()
	for _, summary := range summaries {
		if _, ok := h.actions[summary.ID]; !ok {
			h.actOrder = append(h.actOrder, summary.ID)
		}
		h.actions[summary.ID] = summary
	}
	h.mu.Unlock()
	h.commit()
}

// SetTaskStatusByUser lets the user change any task (services/ContextHub.ts:150).
func (h *ContextHub) SetTaskStatusByUser(taskID, status string) *contract.Failure {
	if !IsTaskStatus(status) {
		return contract.Fail(contract.CodeInvalidInput, "未知的任务状态")
	}
	h.mu.Lock()
	task, ok := h.tasks[taskID]
	if !ok {
		h.mu.Unlock()
		return contract.Fail(contract.CodeNotFound, "任务不存在")
	}
	task.Status = status
	task.UpdatedAt = h.nowISO()
	h.tasks[taskID] = task
	h.mu.Unlock()
	h.commit()
	return nil
}

// SetTaskStatusByAgent only allows the agent that owns the task.
func (h *ContextHub) SetTaskStatusByAgent(agent, taskID, status string) *contract.Failure {
	if !IsTaskStatus(status) {
		return contract.Fail(contract.CodeInvalidInput, "未知的任务状态")
	}
	h.mu.Lock()
	task, ok := h.tasks[taskID]
	if !ok {
		h.mu.Unlock()
		return contract.Fail(contract.CodeNotFound, "任务不存在")
	}
	if task.Owner != agent {
		h.mu.Unlock()
		return contract.Fail(contract.CodePermissionDenied, "只能更新分配给自己的任务状态")
	}
	task.Status = status
	task.UpdatedAt = h.nowISO()
	h.tasks[taskID] = task
	h.mu.Unlock()
	h.commit()
	return nil
}

// CreateHandoff adds a queued task handed from one agent to another
// (services/ContextHub.ts:201-225). The receiver cannot be the sender: that
// would turn a handoff into self-authorisation.
func (h *ContextHub) CreateHandoff(from string, request HandoffRequest) (contract.TaskItem, *contract.Failure) {
	if request.Owner == from {
		return contract.TaskItem{}, contract.Fail(contract.CodeInvalidInput, "交接任务的接收者不能是自己")
	}
	if len([]rune(strings.TrimSpace(request.Goal))) == 0 {
		return contract.TaskItem{}, contract.Fail(contract.CodeInvalidInput, "交接任务必须写明目标")
	}
	now := h.nowISO()
	task := contract.TaskItem{
		ID: h.newID(), Owner: request.Owner, Status: contract.TaskQueued,
		Goal:        request.Goal,
		Inputs:      append([]string{}, request.Inputs...),
		Constraints: append([]string{}, request.Constraints...),
		Outputs:     []string{},
		CreatedBy:   from, CreatedAt: now, UpdatedAt: now,
	}
	h.mu.Lock()
	h.taskOrder = append(h.taskOrder, task.ID)
	h.tasks[task.ID] = task
	h.mu.Unlock()
	h.commit()
	return task, nil
}
