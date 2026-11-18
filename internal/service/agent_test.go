package service

import (
	"strings"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/ai"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

func aiHandoffForTest() ai.HandoffRequest {
	return ai.HandoffRequest{Owner: "curator", Goal: "整理 raws", Inputs: []string{"raws/a.md"}}
}

func TestContextSnapshotAndSync(t *testing.T) {
	app, vaultID, _ := mutationTestApp(t)
	svc := NewAgentService(app)

	result := svc.ContextSnapshot(vaultID)
	if result.Failure != nil || result.Context == nil {
		t.Fatalf("snapshot: %+v", result)
	}
	if result.Context.Vault.ID != vaultID || result.Context.Vault.RootLabel == "" {
		t.Fatalf("vault context: %+v", result.Context.Vault)
	}
	if got := svc.ContextSnapshot("../bad"); got.Failure == nil || got.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("bad vault id: %+v", got)
	}
}

func TestContextPinTruncatesNote(t *testing.T) {
	app, vaultID, root := mutationTestApp(t)
	svc := NewAgentService(app)
	writeServiceFile(t, root, "raws/pin.md", "x")

	result := svc.ContextPin(vaultID, "raws/pin.md", strings.Repeat("注", 600))
	if result.Failure != nil || result.Context == nil || len(result.Context.PinnedReferences) != 1 {
		t.Fatalf("pin: %+v", result)
	}
	reference := result.Context.PinnedReferences[0]
	if reference.ID == "" || strings.Contains(reference.ID, "raws") {
		t.Fatalf("pin id must be opaque: %s", reference.ID)
	}
	if len([]rune(reference.Note)) != 500 {
		t.Fatalf("note not truncated: %d runes", len([]rune(reference.Note)))
	}
	if reference.Revision == 0 || reference.PinnedAt == "" {
		t.Fatalf("pin meta: %+v", reference)
	}

	// The same path pins idempotently with the same id.
	again := svc.ContextPin(vaultID, "raws/pin.md", "")
	if len(again.Context.PinnedReferences) != 1 || again.Context.PinnedReferences[0].ID != reference.ID {
		t.Fatalf("re-pin: %+v", again.Context.PinnedReferences)
	}
	if again.Context.PinnedReferences[0].Note != "" {
		t.Fatalf("note should be empty: %+v", again.Context.PinnedReferences[0])
	}

	if got := svc.ContextPin(vaultID, "../x", ""); got.Failure == nil {
		t.Fatal("traversal path pinned")
	}
	if got := svc.ContextPin(vaultID, "", ""); got.Failure == nil || got.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("empty path: %+v", got)
	}
}

func TestContextUnpinValidation(t *testing.T) {
	app, _, _ := mutationTestApp(t)
	svc := NewAgentService(app)
	if got := svc.ContextUnpin(""); got.Failure == nil || got.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("empty id: %+v", got)
	}
	if got := svc.ContextUnpin("bad id!"); got.Failure == nil || got.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("bad characters: %+v", got)
	}
	if got := svc.ContextUnpin("pin-missing"); got.Failure != nil {
		t.Fatalf("missing id must be a silent no-op: %+v", got)
	}
}

func TestContextTaskStatus(t *testing.T) {
	app, _, _ := mutationTestApp(t)
	svc := NewAgentService(app)
	task, f := app.Context.CreateHandoff("distiller", aiHandoffForTest())
	if f != nil {
		t.Fatal(f)
	}
	result := svc.ContextSetTaskStatus(task.ID, contract.TaskDone)
	if result.Failure != nil || len(result.Context.TaskBoard) != 1 || result.Context.TaskBoard[0].Status != contract.TaskDone {
		t.Fatalf("status change: %+v", result)
	}
	if got := svc.ContextSetTaskStatus(task.ID, "bogus"); got.Failure == nil || got.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("bad status: %+v", got)
	}
	if got := svc.ContextSetTaskStatus("task-missing", contract.TaskDone); got.Failure == nil || got.Failure.Code != contract.CodeNotFound {
		t.Fatalf("missing task: %+v", got)
	}
	if got := svc.ContextSetTaskStatus("bad id!", contract.TaskDone); got.Failure == nil || got.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("bad id: %+v", got)
	}
}

func TestContextReportDocument(t *testing.T) {
	app, vaultID, _ := mutationTestApp(t)
	svc := NewAgentService(app)

	if f := svc.ContextReportDocument(vaultID, "raws/a.md", 3, true, strings.Repeat("选", 250)); f.Code != "" {
		t.Fatalf("report: %+v", f)
	}
	document := app.Context.Snapshot().ActiveDocument
	if document == nil || document.Path != "raws/a.md" || document.Revision != 3 || !document.Dirty {
		t.Fatalf("document: %+v", document)
	}
	if len([]rune(document.Selection)) != 200 {
		t.Fatalf("selection not truncated: %d", len([]rune(document.Selection)))
	}

	if f := svc.ContextReportDocument(vaultID, "", 0, false, ""); f.Code != "" {
		t.Fatalf("clear: %+v", f)
	}
	if app.Context.Snapshot().ActiveDocument != nil {
		t.Fatal("active document not cleared")
	}

	if f := svc.ContextReportDocument(vaultID, "raws/a.md", -1, false, ""); f.Code != contract.CodeInvalidInput {
		t.Fatalf("negative revision: %+v", f)
	}
	if f := svc.ContextReportDocument("../bad", "raws/a.md", 1, false, ""); f.Code != contract.CodeInvalidInput {
		t.Fatalf("bad vault: %+v", f)
	}
	if f := svc.ContextReportDocument(vaultID, "../x", 1, false, ""); f.Code == "" {
		t.Fatal("traversal path accepted")
	}
}
