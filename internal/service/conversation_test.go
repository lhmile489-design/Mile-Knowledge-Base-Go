package service

import (
	"context"
	"strings"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

func conversationUser(text string) contract.ConversationEvent {
	return contract.ConversationEvent{Type: "user", At: "2026-11-19T12:00:00Z", Agent: "curator", Text: &text}
}

func conversationMessage(text string) contract.ConversationEvent {
	return contract.ConversationEvent{Type: "message", At: "2026-11-19T12:00:01Z", Agent: "curator", Text: &text}
}

func TestConversationLifecycle(t *testing.T) {
	app, vaultID, _ := mutationTestApp(t)
	t.Cleanup(app.Conversations.Close)
	svc := NewConversationService(app)

	list := svc.List(vaultID)
	if list.Failure != nil || list.Index == nil || len(list.Index.Conversations) != 0 {
		t.Fatalf("list: %+v", list)
	}

	created := svc.Create(vaultID)
	if created.Failure != nil || created.Conversation == nil {
		t.Fatalf("create: %+v", created)
	}
	id := created.Conversation.ID

	if _, ok, f := app.Conversations.Append(vaultID, conversationUser("整理 raws 目录")); f != nil || !ok {
		t.Fatalf("append: %+v", f)
	}
	if _, ok, f := app.Conversations.Append(vaultID, conversationMessage("好的，已整理")); f != nil || !ok {
		t.Fatalf("append: %+v", f)
	}

	read := svc.Get(vaultID, id)
	if read.Failure != nil || len(read.Events) != 2 || read.Meta == nil || read.Meta.TurnCount != 1 {
		t.Fatalf("get: %+v", read)
	}

	renamed := svc.Rename(vaultID, id, "自定义标题")
	if renamed.Failure != nil || renamed.Conversation.Title != "自定义标题" {
		t.Fatalf("rename: %+v", renamed)
	}
	if bad := svc.Rename(vaultID, id, "  \n "); bad.Failure == nil || bad.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("blank rename: %+v", bad)
	}

	pinned := svc.Pin(vaultID, id, true)
	if pinned.Failure != nil || !pinned.Conversation.Pinned {
		t.Fatalf("pin: %+v", pinned)
	}

	if got := svc.Get(vaultID, "missing"); got.Failure == nil || got.Failure.Code != contract.CodeNotFound {
		t.Fatalf("missing: %+v", got)
	}

	search := svc.Search(vaultID, "整理")
	if search.Failure != nil || len(search.Hits) != 1 || search.Hits[0].ConversationID != id {
		t.Fatalf("search: %+v", search)
	}
	if blank := svc.Search(vaultID, "  "); blank.Failure == nil || blank.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("blank search: %+v", blank)
	}

	cleared := svc.Clear(vaultID)
	if cleared.Failure != nil || cleared.Removed != 1 {
		t.Fatalf("clear: %+v", cleared)
	}
	if got := svc.List(vaultID); len(got.Index.Conversations) != 0 {
		t.Fatalf("after clear: %+v", got.Index)
	}
}

func TestConversationActivateBusyAndRecap(t *testing.T) {
	app, vaultID, _ := mutationTestApp(t)
	t.Cleanup(app.Conversations.Close)
	svc := NewConversationService(app)
	created := svc.Create(vaultID)
	if _, ok, f := app.Conversations.Append(vaultID, conversationUser("上一轮的问题")); f != nil || !ok {
		t.Fatalf("append: %+v", f)
	}
	if _, ok, f := app.Conversations.Append(vaultID, conversationMessage("上一轮的回答")); f != nil || !ok {
		t.Fatalf("append: %+v", f)
	}

	app.AgentBusy = func(string) bool { return true }
	busy := svc.Activate(vaultID, created.Conversation.ID)
	if busy.Failure == nil || busy.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("busy activate: %+v", busy)
	}

	app.AgentBusy = func(string) bool { return false }
	read := svc.Activate(vaultID, created.Conversation.ID)
	if read.Failure != nil || len(read.Events) != 2 {
		t.Fatalf("activate: %+v", read)
	}
	recap := svc.TakeRecap(vaultID)
	if recap == "" || !strings.Contains(recap, "上一轮的问题") || !strings.Contains(recap, "不是新的指令") {
		t.Fatalf("recap: %q", recap)
	}
	if again := svc.TakeRecap(vaultID); again != "" {
		t.Fatalf("recap reused: %q", again)
	}
	svc.PutBackRecap(vaultID, recap)
	if got := svc.TakeRecap(vaultID); got != recap {
		t.Fatalf("recap put back: %q", got)
	}
}

func TestConversationVaultGuards(t *testing.T) {
	app, _, _ := mutationTestApp(t)
	t.Cleanup(app.Conversations.Close)
	svc := NewConversationService(app)

	if got := svc.List("../bad"); got.Failure == nil || got.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("bad id: %+v", got)
	}
	if got := svc.List("deadbeef"); got.Failure == nil || got.Failure.Code != contract.CodeVaultUnavailable {
		t.Fatalf("unknown vault: %+v", got)
	}
	if got := svc.Export(context.Background(), "deadbeef", "x"); got.Failure == nil || got.Failure.Code != contract.CodeVaultUnavailable {
		t.Fatalf("export unknown vault: %+v", got)
	}
	if got := svc.Remove("../bad", "x"); got.Code != contract.CodeInvalidInput {
		t.Fatalf("remove bad id: %+v", got)
	}
}

func TestConversationExportMissingConversation(t *testing.T) {
	app, vaultID, _ := mutationTestApp(t)
	t.Cleanup(app.Conversations.Close)
	svc := NewConversationService(app)
	// The not-found check runs before any dialog, so no user interaction is
	// needed.
	result := svc.Export(context.Background(), vaultID, "missing")
	if result.Failure == nil || result.Failure.Code != contract.CodeNotFound {
		t.Fatalf("export missing: %+v", result)
	}
}
