package ai

import (
	"testing"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

func testHub() *ContextHub {
	hub := NewContextHub()
	hub.now = func() time.Time { return time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC) }
	return hub
}

func TestContextSnapshotStartsEmpty(t *testing.T) {
	hub := testHub()
	snapshot := hub.Snapshot()
	if snapshot.SnapshotID == "" || snapshot.Sequence != 0 {
		t.Fatalf("empty snapshot: %+v", snapshot)
	}
	if snapshot.PinnedReferences == nil || snapshot.TaskBoard == nil ||
		snapshot.DecisionLog == nil || snapshot.ActionLedger == nil || snapshot.RetrievalBundle == nil {
		t.Fatalf("arrays must be empty, not nil: %+v", snapshot)
	}
	if snapshot.ActiveDocument != nil {
		t.Fatalf("active document: %+v", snapshot.ActiveDocument)
	}
}

func TestContextChangesBumpSequence(t *testing.T) {
	hub := testHub()
	first := hub.Snapshot()
	hub.SetVault(contract.VaultContext{ID: "v1", RootLabel: "库", Revision: 3, IndexRevision: 7})
	second := hub.Snapshot()
	if second.Sequence != first.Sequence+1 || second.SnapshotID == first.SnapshotID {
		t.Fatalf("sequence: %+v -> %+v", first, second)
	}
	// The same derived values synced again must not look like a new change.
	hub.SetVault(contract.VaultContext{ID: "v1", RootLabel: "库", Revision: 3, IndexRevision: 7})
	if third := hub.Snapshot(); third.Sequence != second.Sequence {
		t.Fatalf("identical vault synced twice: %+v", third)
	}
}

func TestContextActiveDocument(t *testing.T) {
	hub := testHub()
	hub.SetActiveDocument(&contract.ActiveDocumentContext{Path: "raws/a.md", Revision: 4, Dirty: true})
	if snapshot := hub.Snapshot(); snapshot.ActiveDocument == nil || snapshot.ActiveDocument.Path != "raws/a.md" {
		t.Fatalf("set: %+v", snapshot.ActiveDocument)
	}
	hub.SetActiveDocument(nil)
	if snapshot := hub.Snapshot(); snapshot.ActiveDocument != nil {
		t.Fatalf("clear: %+v", snapshot.ActiveDocument)
	}
}

func TestContextPinUnpin(t *testing.T) {
	hub := testHub()
	hub.Pin(contract.Reference{ID: "pin-1", Path: "raws/a.md", Revision: 2})
	hub.Pin(contract.Reference{ID: "pin-2", Path: "raws/b.md", Revision: 3})
	snapshot := hub.Snapshot()
	if len(snapshot.PinnedReferences) != 2 || snapshot.PinnedReferences[0].ID != "pin-1" {
		t.Fatalf("pins: %+v", snapshot.PinnedReferences)
	}
	// Re-pinning the same id replaces in place, keeping the insertion order.
	hub.Pin(contract.Reference{ID: "pin-1", Path: "raws/a.md", Revision: 9})
	snapshot = hub.Snapshot()
	if len(snapshot.PinnedReferences) != 2 || snapshot.PinnedReferences[0].Revision != 9 {
		t.Fatalf("re-pin: %+v", snapshot.PinnedReferences)
	}
	hub.Unpin("pin-1")
	if snapshot := hub.Snapshot(); len(snapshot.PinnedReferences) != 1 || snapshot.PinnedReferences[0].ID != "pin-2" {
		t.Fatalf("unpin: %+v", snapshot.PinnedReferences)
	}
	before := hub.Snapshot().Sequence
	hub.Unpin("pin-missing")
	if after := hub.Snapshot().Sequence; after != before {
		t.Fatalf("unpin of a missing id must not commit: %d -> %d", before, after)
	}
}

func TestContextTaskStatusRules(t *testing.T) {
	hub := testHub()
	task, f := hub.CreateHandoff("distiller", HandoffRequest{
		Owner: "curator", Goal: "把草稿合并为正式技能",
		Inputs: []string{"raws/a.md"}, Constraints: []string{"保留来源"},
	})
	if f != nil {
		t.Fatal(f)
	}
	if f := hub.SetTaskStatusByAgent("scout", task.ID, contract.TaskRunning); f == nil || f.Code != contract.CodePermissionDenied {
		t.Fatalf("foreign agent status change: %+v", f)
	}
	if f := hub.SetTaskStatusByAgent("curator", task.ID, contract.TaskRunning); f != nil {
		t.Fatal(f)
	}
	if f := hub.SetTaskStatusByUser(task.ID, contract.TaskDone); f != nil {
		t.Fatal(f)
	}
	if got := hub.Snapshot().TaskBoard[0]; got.Status != contract.TaskDone {
		t.Fatalf("status: %+v", got)
	}
	if f := hub.SetTaskStatusByUser(task.ID, "bogus"); f == nil || f.Code != contract.CodeInvalidInput {
		t.Fatalf("bad status: %+v", f)
	}
	if f := hub.SetTaskStatusByUser("task-missing", contract.TaskDone); f == nil || f.Code != contract.CodeNotFound {
		t.Fatalf("missing task: %+v", f)
	}
}

func TestCreateHandoffRules(t *testing.T) {
	hub := testHub()
	if _, f := hub.CreateHandoff("curator", HandoffRequest{Owner: "curator", Goal: "x"}); f == nil || f.Code != contract.CodeInvalidInput {
		t.Fatalf("self handoff: %+v", f)
	}
	if _, f := hub.CreateHandoff("curator", HandoffRequest{Owner: "scout", Goal: "   "}); f == nil || f.Code != contract.CodeInvalidInput {
		t.Fatalf("empty goal: %+v", f)
	}
	task, f := hub.CreateHandoff("distiller", HandoffRequest{
		Owner: "curator", Goal: "整理 raws",
		Inputs: []string{"raws/a.md"}, Constraints: []string{"保留来源", "标注不确定性"},
	})
	if f != nil {
		t.Fatal(f)
	}
	if task.Owner != "curator" || task.CreatedBy != "distiller" || task.Status != contract.TaskQueued {
		t.Fatalf("task: %+v", task)
	}
	if task.ID == "" || task.CreatedAt == "" || task.UpdatedAt == "" {
		t.Fatalf("task meta: %+v", task)
	}
	if task.Outputs == nil {
		t.Fatalf("outputs must be an empty array")
	}
	if len(task.Inputs) != 1 || len(task.Constraints) != 2 {
		t.Fatalf("task lists: %+v", task)
	}
	if board := hub.Snapshot().TaskBoard; len(board) != 1 || board[0].ID != task.ID {
		t.Fatalf("board: %+v", board)
	}
}

func TestContextRecordActionsUpsertsInOrder(t *testing.T) {
	hub := testHub()
	hub.RecordActions([]contract.ContextActionSummary{
		{ID: "a1", Kind: "edit", Target: "raws/a.md", Status: "completed", At: "t1"},
		{ID: "a2", Kind: "move", Source: "raws/a.md", Target: "raws/b.md", Status: "completed", At: "t2"},
	})
	hub.RecordActions([]contract.ContextActionSummary{
		{ID: "a1", Kind: "edit", Target: "raws/a.md", Status: "reverted", At: "t3"},
	})
	ledger := hub.Snapshot().ActionLedger
	if len(ledger) != 2 || ledger[0].ID != "a1" || ledger[1].ID != "a2" {
		t.Fatalf("order: %+v", ledger)
	}
	if ledger[0].Status != "reverted" {
		t.Fatalf("upsert: %+v", ledger[0])
	}
}
