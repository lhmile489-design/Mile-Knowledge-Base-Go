package ai

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// Approval modes (shared/executionMode.ts). manual and auto both ask; only
// full (yolo) skips the card, and a permanent delete is refused there (D-005).
const (
	ApprovalAsk  = "ask"
	ApprovalAuto = "auto"
	ApprovalYolo = "yolo"

	// GrantTTL is how long an approved mutation stays consumable.
	GrantTTL = 60 * time.Second
)

// NormalizeApproval degrades unknown values to the strictest mode.
func NormalizeApproval(value string) string {
	switch value {
	case ApprovalAsk, ApprovalAuto, ApprovalYolo:
		return value
	default:
		return ApprovalAsk
	}
}

// ApprovalOption is one button on the card; it is always allow then reject.
type ApprovalOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// Allow and reject are the only options (no "always allow").
var ApprovalOptions = []ApprovalOption{
	{ID: "allow_once", Label: "允许"},
	{ID: "reject_once", Label: "拒绝"},
}

// PermEvent is the approval-card lifecycle frame. T10 turns it into a
// StreamFrame with kind=permission.
type PermEvent struct {
	RequestID  string
	ToolName   string
	Detail     string
	TargetPath string
	Approval   string
	Status     string // pending | approved | rejected | expired
	Preview    *contract.WritePreview
	Options    []ApprovalOption
}

type pendingApproval struct {
	requestID string
	sessionID string
	path      string
	kind      string
	approval  string
	decided   chan bool
	settled   bool
	expired   bool
}

type grant struct {
	path            string
	kind            string
	approvalAtIssue string
	expiresAt       time.Time
}

// ApprovalManager issues cards, waits for the answer and keeps the 60-second
// grants. One instance serves every session.
type ApprovalManager struct {
	mu      sync.Mutex
	pending map[string]*pendingApproval
	grants  map[string]*grant

	modeOf func(sessionID string) string
	emit   func(PermEvent)
	now    func() time.Time
	newID  func() string
}

// NewApprovalManager wires the mode lookup and the frame sink.
func NewApprovalManager(modeOf func(sessionID string) string, emit func(PermEvent)) *ApprovalManager {
	return &ApprovalManager{
		pending: map[string]*pendingApproval{},
		grants:  map[string]*grant{},
		modeOf:  modeOf,
		emit:    emit,
		now:     time.Now,
		newID:   func() string { return "perm_" + randomHex(8) },
	}
}

func grantKey(sessionID, path string) string { return sessionID + "\x00" + path }

// AllowWrite asks for one file write. yolo proceeds without a card.
func (m *ApprovalManager) AllowWrite(ctx context.Context, sessionID, path string) error {
	return m.request(ctx, sessionID, path, "write", "write_note", "写入 "+path, nil)
}

// AllowMutation asks for one structural change.
func (m *ApprovalManager) AllowMutation(ctx context.Context, sessionID, path string) error {
	return m.request(ctx, sessionID, path, "mutation", "mutate_notes", "变更 "+path, nil)
}

// ConsumeFreshMutation spends the 60-second grant a permanent delete needs.
// The grant must come from a card issued outside yolo mode.
func (m *ApprovalManager) ConsumeFreshMutation(ctx context.Context, sessionID, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := grantKey(sessionID, path)
	found := m.grants[key]
	if found == nil || found.kind != "mutation" || found.approvalAtIssue == ApprovalYolo || m.expired(found) {
		return &EngineError{Code: contract.CodePermissionDenied, Message: "永久删除需要本次单独确认"}
	}
	delete(m.grants, key)
	return nil
}

func (m *ApprovalManager) expired(found *grant) bool {
	return !m.now().Before(found.expiresAt)
}

func (m *ApprovalManager) request(ctx context.Context, sessionID, path, kind, toolName, detail string, preview *contract.WritePreview) error {
	mode := NormalizeApproval(m.modeOf(sessionID))
	if mode == ApprovalYolo {
		// No card in full-auto mode. A permanent delete therefore has no fresh
		// grant and is refused by ConsumeFreshMutation (D-005).
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	request := &pendingApproval{
		requestID: m.newID(), sessionID: sessionID, path: path, kind: kind,
		approval: mode, decided: make(chan bool, 1),
	}
	m.mu.Lock()
	m.pending[request.requestID] = request
	m.mu.Unlock()
	m.emitEvent(PermEvent{
		RequestID: request.requestID, ToolName: toolName, Detail: detail,
		TargetPath: path, Approval: mode, Status: "pending",
		Preview: preview, Options: ApprovalOptions,
	})

	select {
	case approved := <-request.decided:
		m.mu.Lock()
		expired := request.expired
		m.mu.Unlock()
		if expired {
			return &EngineError{Code: contract.CodePermissionDenied, Message: "审批已失效"}
		}
		if !approved {
			return &EngineError{Code: contract.CodePermissionDenied, Message: "用户拒绝了这次操作"}
		}
		if kind == "mutation" {
			m.mu.Lock()
			m.grants[grantKey(sessionID, path)] = &grant{
				path: path, kind: kind, approvalAtIssue: mode,
				expiresAt: m.now().Add(GrantTTL),
			}
			m.mu.Unlock()
		}
		return nil
	case <-ctx.Done():
		m.mu.Lock()
		if !request.settled {
			request.settled = true
			delete(m.pending, request.requestID)
		}
		m.mu.Unlock()
		m.emitEvent(PermEvent{
			RequestID: request.requestID, ToolName: toolName, Detail: detail,
			TargetPath: path, Approval: mode, Status: "expired", Preview: preview,
			Options: ApprovalOptions,
		})
		return ctx.Err()
	}
}

// Approve resolves a pending card. It returns approval-expired when the card
// is gone (cancelled, timed out, or already answered).
func (m *ApprovalManager) Approve(requestID, optionID string) *contract.Failure {
	if optionID != "allow_once" && optionID != "allow" {
		return contract.Fail(contract.CodeInvalidInput, "未知的审批选项")
	}
	request, f := m.take(requestID)
	if f != nil {
		return f
	}
	m.emitEvent(PermEvent{
		RequestID: request.requestID, TargetPath: request.path,
		Approval: request.approval, Status: "approved", Options: ApprovalOptions,
	})
	request.decided <- true
	return nil
}

// Reject resolves a pending card as denied.
func (m *ApprovalManager) Reject(requestID string) *contract.Failure {
	request, f := m.take(requestID)
	if f != nil {
		return f
	}
	m.emitEvent(PermEvent{
		RequestID: request.requestID, TargetPath: request.path,
		Approval: request.approval, Status: "rejected", Options: ApprovalOptions,
	})
	request.decided <- false
	return nil
}

// ExpireSessionCards invalidates every pending card of a session (turn cancel,
// vault switch, reset). The waiting request returns 审批已失效.
func (m *ApprovalManager) ExpireSessionCards(sessionID string) {
	m.mu.Lock()
	var expired []*pendingApproval
	for id, request := range m.pending {
		if request.sessionID == sessionID && !request.settled {
			request.settled = true
			request.expired = true
			expired = append(expired, request)
			delete(m.pending, id)
		}
	}
	m.mu.Unlock()
	for _, request := range expired {
		m.emitEvent(PermEvent{
			RequestID: request.requestID, TargetPath: request.path,
			Approval: request.approval, Status: "expired", Options: ApprovalOptions,
		})
		request.decided <- false
	}
}

func (m *ApprovalManager) take(requestID string) (*pendingApproval, *contract.Failure) {
	m.mu.Lock()
	defer m.mu.Unlock()
	request, ok := m.pending[requestID]
	if !ok || request.settled {
		return nil, contract.Fail(contract.CodeApprovalExpired, "审批已失效")
	}
	request.settled = true
	delete(m.pending, requestID)
	return request, nil
}

func (m *ApprovalManager) emitEvent(event PermEvent) {
	if m.emit != nil {
		m.emit(event)
	}
}

// Compile-time note: ApprovalManager satisfies the tool gate.
var _ interface {
	AllowWrite(context.Context, string, string) error
	AllowMutation(context.Context, string, string) error
	ConsumeFreshMutation(context.Context, string, string) error
} = (*ApprovalManager)(nil)

var _ = errors.Is
var _ = fmt.Sprintf
