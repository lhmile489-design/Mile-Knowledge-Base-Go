package ai

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

type approvalHarness struct {
	manager *ApprovalManager
	mu      sync.Mutex
	events  []PermEvent
	modes   map[string]string
	now     time.Time
}

func newHarness(t *testing.T, modes map[string]string) *approvalHarness {
	t.Helper()
	h := &approvalHarness{modes: modes, now: time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)}
	h.manager = NewApprovalManager(
		func(sessionID string) string { return h.modes[sessionID] },
		func(event PermEvent) {
			h.mu.Lock()
			h.events = append(h.events, event)
			h.mu.Unlock()
		},
	)
	h.manager.now = func() time.Time { return h.now }
	return h
}

func (h *approvalHarness) last() (PermEvent, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.events) == 0 {
		return PermEvent{}, false
	}
	return h.events[len(h.events)-1], true
}

func (h *approvalHarness) waitPending(t *testing.T) PermEvent {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		h.mu.Lock()
		for _, event := range h.events {
			if event.Status == "pending" {
				h.mu.Unlock()
				return event
			}
		}
		h.mu.Unlock()
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("no pending event")
	return PermEvent{}
}

func TestApprovalAllowWrite(t *testing.T) {
	h := newHarness(t, map[string]string{"s1": ApprovalAsk})
	done := make(chan error, 1)
	go func() { done <- h.manager.AllowWrite(context.Background(), "s1", "raws/a.md", nil) }()
	pending := h.waitPending(t)
	if pending.Approval != ApprovalAsk || pending.TargetPath != "raws/a.md" || len(pending.Options) != 2 {
		t.Fatalf("pending: %+v", pending)
	}
	if pending.Options[0].ID != "allow_once" || pending.Options[1].ID != "reject_once" {
		t.Fatalf("options: %+v", pending.Options)
	}
	if f := h.manager.Approve(pending.RequestID, "allow_once"); f != nil {
		t.Fatal(f)
	}
	if err := <-done; err != nil {
		t.Fatalf("allow: %v", err)
	}
	if last, _ := h.last(); last.Status != "approved" || last.RequestID != pending.RequestID {
		t.Fatalf("approved event: %+v", last)
	}
}

func TestApprovalReject(t *testing.T) {
	h := newHarness(t, map[string]string{"s1": ApprovalAuto})
	done := make(chan error, 1)
	go func() { done <- h.manager.AllowWrite(context.Background(), "s1", "raws/a.md", nil) }()
	pending := h.waitPending(t)
	if pending.Approval != ApprovalAuto {
		t.Fatalf("auto should also ask: %+v", pending)
	}
	if f := h.manager.Reject(pending.RequestID); f != nil {
		t.Fatal(f)
	}
	err := <-done
	var engineErr *EngineError
	if !errors.As(err, &engineErr) || engineErr.Code != contract.CodePermissionDenied {
		t.Fatalf("reject: %v", err)
	}
}

func TestApprovalCancelExpires(t *testing.T) {
	h := newHarness(t, map[string]string{"s1": ApprovalAsk})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- h.manager.AllowWrite(ctx, "s1", "raws/a.md", nil) }()
	pending := h.waitPending(t)
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	if last, _ := h.last(); last.Status != "expired" || last.RequestID != pending.RequestID {
		t.Fatalf("expired event: %+v", last)
	}
	if f := h.manager.Approve(pending.RequestID, "allow_once"); f == nil || f.Code != contract.CodeApprovalExpired {
		t.Fatalf("approve after cancel: %+v", f)
	}
}

func TestApprovalYoloSkipsCard(t *testing.T) {
	h := newHarness(t, map[string]string{"s1": ApprovalYolo})
	if err := h.manager.AllowWrite(context.Background(), "s1", "raws/a.md", nil); err != nil {
		t.Fatal(err)
	}
	if err := h.manager.AllowMutation(context.Background(), "s1", "raws/a.md"); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.last(); ok {
		t.Fatalf("yolo must not emit cards: %+v", h.events)
	}
	// D-005: no card in yolo means no fresh grant, so the delete is refused.
	if err := h.manager.ConsumeFreshMutation(context.Background(), "s1", "raws/a.md"); err == nil {
		t.Fatal("yolo permanent delete should be refused")
	}
}

func TestWriteApprovalDoesNotGrantFreshMutation(t *testing.T) {
	h := newHarness(t, map[string]string{"s1": ApprovalAsk})
	done := make(chan error, 1)
	go func() { done <- h.manager.AllowWrite(context.Background(), "s1", "raws/a.md", nil) }()
	pending := h.waitPending(t)
	if f := h.manager.Approve(pending.RequestID, "allow_once"); f != nil {
		t.Fatal(f)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	// D-012: an approved write card never stands in for the mutation card a
	// permanent delete needs.
	if err := h.manager.ConsumeFreshMutation(context.Background(), "s1", "raws/a.md"); err == nil {
		t.Fatal("a write approval satisfied a fresh mutation")
	}
}

func TestFreshMutationGrant(t *testing.T) {
	h := newHarness(t, map[string]string{"s1": ApprovalAsk})
	done := make(chan error, 1)
	go func() { done <- h.manager.AllowMutation(context.Background(), "s1", "raws/a.md") }()
	pending := h.waitPending(t)
	if f := h.manager.Approve(pending.RequestID, "allow_once"); f != nil {
		t.Fatal(f)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := h.manager.ConsumeFreshMutation(context.Background(), "s1", "raws/a.md"); err != nil {
		t.Fatalf("first consume: %v", err)
	}
	if err := h.manager.ConsumeFreshMutation(context.Background(), "s1", "raws/a.md"); err == nil {
		t.Fatal("grant must be consumed once")
	}
}

func TestFreshMutationGrantExpires(t *testing.T) {
	h := newHarness(t, map[string]string{"s1": ApprovalAsk})
	done := make(chan error, 1)
	go func() { done <- h.manager.AllowMutation(context.Background(), "s1", "raws/a.md") }()
	pending := h.waitPending(t)
	_ = h.manager.Approve(pending.RequestID, "allow_once")
	<-done
	h.now = h.now.Add(61 * time.Second)
	if err := h.manager.ConsumeFreshMutation(context.Background(), "s1", "raws/a.md"); err == nil {
		t.Fatal("expired grant accepted")
	}
}

func TestApproveUnknownCard(t *testing.T) {
	h := newHarness(t, map[string]string{})
	if f := h.manager.Approve("perm_missing", "allow_once"); f == nil || f.Code != contract.CodeApprovalExpired {
		t.Fatalf("unknown card: %+v", f)
	}
	if f := h.manager.Reject("perm_missing"); f == nil || f.Code != contract.CodeApprovalExpired {
		t.Fatalf("unknown reject: %+v", f)
	}
}

func TestApproveBadOption(t *testing.T) {
	h := newHarness(t, map[string]string{"s1": ApprovalAsk})
	go func() { _ = h.manager.AllowWrite(context.Background(), "s1", "raws/a.md", nil) }()
	pending := h.waitPending(t)
	if f := h.manager.Approve(pending.RequestID, "always_allow"); f == nil || f.Code != contract.CodeInvalidInput {
		t.Fatalf("bad option: %+v", f)
	}
	_ = h.manager.Reject(pending.RequestID)
}

func TestExpireSessionCards(t *testing.T) {
	h := newHarness(t, map[string]string{"s1": ApprovalAsk})
	done := make(chan error, 1)
	go func() { done <- h.manager.AllowWrite(context.Background(), "s1", "raws/a.md", nil) }()
	pending := h.waitPending(t)
	h.manager.ExpireSessionCards("s1")
	if err := <-done; err == nil {
		t.Fatal("expired card allowed the write")
	}
	if last, _ := h.last(); last.Status != "expired" {
		t.Fatalf("expired event: %+v", last)
	}
	if f := h.manager.Approve(pending.RequestID, "allow_once"); f == nil {
		t.Fatal("expired card approved")
	}
}

func TestNormalizeApproval(t *testing.T) {
	if NormalizeApproval("yolo") != ApprovalYolo || NormalizeApproval("ask") != ApprovalAsk {
		t.Fatal("valid modes changed")
	}
	if NormalizeApproval("bogus") != ApprovalAsk || NormalizeApproval("") != ApprovalAsk {
		t.Fatal("invalid modes must degrade to ask")
	}
}

func TestApprovalEventsCarryCardDetails(t *testing.T) {
	h := newHarness(t, map[string]string{"s1": ApprovalAsk})
	preview := &contract.WritePreview{Kind: "created", NewLineCount: 1}
	done := make(chan error, 1)
	go func() { done <- h.manager.AllowWrite(context.Background(), "s1", "raws/a.md", preview) }()
	pending := h.waitPending(t)
	if pending.ToolName != "write_note" || pending.Detail != "写入 raws/a.md" ||
		pending.Preview == nil || pending.Preview.Kind != "created" {
		t.Fatalf("pending card: %+v", pending)
	}
	if f := h.manager.Approve(pending.RequestID, "allow_once"); f != nil {
		t.Fatal(f)
	}
	<-done
	last, _ := h.last()
	if last.Status != "approved" || last.ToolName != "write_note" ||
		last.Detail != "写入 raws/a.md" || last.Preview == nil || len(last.Options) != 2 {
		t.Fatalf("approved event: %+v", last)
	}
}

func TestApprovalConcurrentDecision(t *testing.T) {
	h := newHarness(t, map[string]string{"s1": ApprovalAsk})
	done := make(chan error, 1)
	go func() { done <- h.manager.AllowWrite(context.Background(), "s1", "raws/a.md", nil) }()
	pending := h.waitPending(t)

	var wg sync.WaitGroup
	var approveF, rejectF *contract.Failure
	wg.Add(2)
	go func() { defer wg.Done(); approveF = h.manager.Approve(pending.RequestID, "allow_once") }()
	go func() { defer wg.Done(); rejectF = h.manager.Reject(pending.RequestID) }()
	wg.Wait()
	if (approveF == nil) == (rejectF == nil) {
		t.Fatalf("exactly one decision must win: approve=%+v reject=%+v", approveF, rejectF)
	}
	if approveF != nil && approveF.Code != contract.CodeApprovalExpired {
		t.Fatalf("losing approve: %+v", approveF)
	}
	if rejectF != nil && rejectF.Code != contract.CodeApprovalExpired {
		t.Fatalf("losing reject: %+v", rejectF)
	}
	err := <-done
	if approveF == nil && err != nil {
		t.Fatalf("approve won but the waiter got %v", err)
	}
	if rejectF == nil && err == nil {
		t.Fatal("reject won but the waiter was allowed")
	}
}

func TestIssuedCardKeepsItsMode(t *testing.T) {
	h := newHarness(t, map[string]string{"s1": ApprovalAsk})
	done := make(chan error, 1)
	go func() { done <- h.manager.AllowWrite(context.Background(), "s1", "raws/a.md", nil) }()
	pending := h.waitPending(t)
	h.modes["s1"] = ApprovalYolo // switching the mode must not touch an issued card
	if f := h.manager.Approve(pending.RequestID, "allow_once"); f != nil {
		t.Fatal(f)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if last, _ := h.last(); last.Approval != ApprovalAsk {
		t.Fatalf("issued card mode changed: %+v", last)
	}
}
