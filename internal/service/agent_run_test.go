package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/ai"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// fakeSink records every frame; pending card frames are pushed to a channel so
// the test can answer them while the turn is blocked on the gate.
type fakeSink struct {
	mu     sync.Mutex
	frames []contract.StreamFrame
	cards  chan contract.PermFrame
}

func newFakeSink() *fakeSink { return &fakeSink{cards: make(chan contract.PermFrame, 4)} }

func (s *fakeSink) Send(frame contract.StreamFrame) error {
	s.mu.Lock()
	s.frames = append(s.frames, frame)
	s.mu.Unlock()
	if frame.Kind == "permission" && frame.Permission != nil && frame.Permission.Status == "pending" {
		select {
		case s.cards <- *frame.Permission:
		default:
		}
	}
	return nil
}

func (s *fakeSink) snapshot() []contract.StreamFrame {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]contract.StreamFrame{}, s.frames...)
}

func intPtr(value int) *int { return &value }

// promptServer answers the first request with a write_note tool call and every
// later request with a plain answer.
func promptServer(t *testing.T) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		call := calls
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		flush := func(chunk string) { fmt.Fprintf(w, "data: %s\n\n", chunk) }
		if call == 1 {
			arguments := `{"path":"raws/a.md","content":"hi"}`
			flush(fmt.Sprintf(`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"write_note","arguments":%q}}]}}]}`, arguments))
			flush(`{"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`)
		} else {
			flush(`{"choices":[{"delta":{"content":"完成"}}]}`)
			flush(`{"choices":[{"delta":{},"finish_reason":"stop"}]}`)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(server.Close)
	return server
}

func promptApp(t *testing.T, server *httptest.Server) (*App, string, string) {
	t.Helper()
	app, vaultID, root := mutationTestApp(t)
	t.Cleanup(app.Conversations.Close)
	providers := []ai.Provider{{
		Name: "fake", Kind: "openai", BaseURL: server.URL,
		Models: []string{"fake-model"}, DefaultModel: "fake-model",
	}}
	if f := ai.WriteProviders(root, providers); f != nil {
		t.Fatalf("providers: %+v", f)
	}
	app.AgentStream = ai.StreamOptions{Client: server.Client(), MaxRetries: intPtr(0)}
	return app, vaultID, root
}

func waitCard(t *testing.T, sink *fakeSink) contract.PermFrame {
	t.Helper()
	select {
	case card := <-sink.cards:
		return card
	case <-time.After(5 * time.Second):
		t.Fatal("no approval card")
		return contract.PermFrame{}
	}
}

func TestAgentListEngineStatus(t *testing.T) {
	app, vaultID, root := mutationTestApp(t)
	svc := NewAgentService(app)

	offline := svc.List("curator", vaultID)
	if offline.Failure != nil || offline.Engine == nil || offline.Engine.Phase != "offline" || offline.Engine.Message == "" {
		t.Fatalf("offline: %+v", offline)
	}
	if offline.Session == nil || offline.Session.AvailableEfforts == nil || len(offline.Session.AvailableEfforts) != 0 {
		t.Fatalf("session: %+v", offline.Session)
	}

	if f := ai.WriteProviders(root, []ai.Provider{{
		Name: "fake", Kind: "openai", BaseURL: "https://example.com",
		Models: []string{"m1", "m2"}, DefaultModel: "m1",
	}}); f != nil {
		t.Fatal(f)
	}
	ready := svc.List("curator", vaultID)
	if ready.Engine == nil || ready.Engine.Phase != "ready" || ready.Engine.Capabilities == nil {
		t.Fatalf("ready: %+v", ready)
	}
	if len(ready.Session.AvailableModels) != 2 || ready.Session.Model != "m1" {
		t.Fatalf("models: %+v", ready.Session)
	}

	if bad := svc.List("nobody", vaultID); bad.Failure == nil || bad.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("unknown agent: %+v", bad)
	}
}

func TestRoutePreviewBinding(t *testing.T) {
	app, _, _ := mutationTestApp(t)
	svc := NewAgentService(app)
	preview := svc.RoutePreview("帮我把这些笔记整理一下")
	if preview.Agent != "curator" || !strings.Contains(preview.Reason, "整理归档") || len(preview.MatchedTerms) == 0 {
		t.Fatalf("preview: %+v", preview)
	}
	fallback := svc.RoutePreview("在吗")
	if fallback.Agent != "general" || !strings.Contains(fallback.Reason, "自由问答") || fallback.MatchedTerms == nil {
		t.Fatalf("fallback: %+v", fallback)
	}
}

func TestPromptValidation(t *testing.T) {
	app, vaultID, _ := mutationTestApp(t)
	svc := NewAgentService(app)
	sink := newFakeSink()
	if result, err := svc.prompt(context.Background(), "nobody", vaultID, "x", sink); err != nil || result.Failure == nil {
		t.Fatalf("unknown agent: %+v %v", result, err)
	}
	if result, _ := svc.prompt(context.Background(), "curator", vaultID, "   ", sink); result.Failure == nil || result.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("empty text: %+v", result)
	}
	tooLong := strings.Repeat("字", maxPromptChars+1)
	if result, _ := svc.prompt(context.Background(), "curator", vaultID, tooLong, sink); result.Failure == nil || result.Failure.Code != contract.CodeTooLarge {
		t.Fatalf("too long: %+v", result)
	}
}

func TestPromptStreamsApprovalAndRecords(t *testing.T) {
	server := promptServer(t)
	app, vaultID, root := promptApp(t, server)
	svc := NewAgentService(app)
	sink := newFakeSink()

	done := make(chan contract.PromptResult, 1)
	errs := make(chan error, 1)
	go func() {
		result, err := svc.prompt(context.Background(), "curator", vaultID, "写入一篇笔记", sink)
		done <- result
		errs <- err
	}()

	card := waitCard(t, sink)
	if card.ToolName != "write_note" || card.TargetPath != "raws/a.md" || card.Preview == nil {
		t.Fatalf("card: %+v", card)
	}
	if failure := svc.Approve(card.RequestID, "allow_once"); failure.Code != "" {
		t.Fatalf("approve: %+v", failure)
	}
	select {
	case result := <-done:
		if err := <-errs; err != nil {
			t.Fatalf("prompt: %v", err)
		}
		if result.StopReason != "end_turn" {
			t.Fatalf("stop: %+v", result)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("prompt did not finish")
	}

	body, err := os.ReadFile(filepath.Join(root, "raws", "a.md"))
	if err != nil || string(body) != "hi" {
		t.Fatalf("file: %q %v", body, err)
	}

	frames := sink.snapshot()
	var sawApproved, sawTool, sawDelta, sawFinish bool
	for _, frame := range frames {
		switch {
		case frame.Kind == "permission" && frame.Permission != nil && frame.Permission.Status == "approved":
			sawApproved = true
		case frame.Kind == "tool" && frame.ToolCall != nil && frame.ToolCall.Status == "completed":
			sawTool = true
		case frame.Kind == "delta" && strings.Contains(frame.Text, "完成"):
			sawDelta = true
		case frame.Kind == "turn-end" && frame.Finish:
			sawFinish = true
		}
	}
	if !sawApproved || !sawTool || !sawDelta || !sawFinish {
		t.Fatalf("frames approved=%v tool=%v delta=%v finish=%v: %+v", sawApproved, sawTool, sawDelta, sawFinish, frames)
	}

	list := app.Conversations.List(vaultID)
	if list.Index == nil || list.Index.ActiveID == nil {
		t.Fatalf("history: %+v", list)
	}
	read := app.Conversations.Get(vaultID, *list.Index.ActiveID)
	if read.Failure != nil {
		t.Fatalf("read: %+v", read.Failure)
	}
	types := []string{}
	for _, event := range read.Events {
		types = append(types, event.Type)
	}
	if strings.Join(types, ",") != "user,tool,message,turn-end" {
		t.Fatalf("events: %v", types)
	}
	if text := derefString(read.Events[0].Text); text != "写入一篇笔记" {
		t.Fatalf("user event: %q", text)
	}
}

func TestPromptCancelExpiresCard(t *testing.T) {
	server := promptServer(t)
	app, vaultID, root := promptApp(t, server)
	svc := NewAgentService(app)
	sink := newFakeSink()

	done := make(chan contract.PromptResult, 1)
	go func() {
		result, _ := svc.prompt(context.Background(), "curator", vaultID, "写入一篇笔记", sink)
		done <- result
	}()
	waitCard(t, sink)
	if err := svc.Cancel("curator"); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-done:
		if result.StopReason != "cancelled" {
			t.Fatalf("stop: %+v", result)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("prompt did not stop")
	}
	if _, err := os.Stat(filepath.Join(root, "raws", "a.md")); !os.IsNotExist(err) {
		t.Fatalf("cancelled turn wrote the file")
	}
	expired := false
	for _, frame := range sink.snapshot() {
		if frame.Kind == "permission" && frame.Permission != nil && frame.Permission.Status == "expired" {
			expired = true
		}
	}
	if !expired {
		t.Fatalf("no expired frame: %+v", sink.snapshot())
	}
}

func TestResetDropsSessions(t *testing.T) {
	app, vaultID, _ := mutationTestApp(t)
	svc := NewAgentService(app)
	first := svc.List("curator", vaultID)
	if first.Session == nil {
		t.Fatal("list")
	}
	if err := svc.Reset(context.Background(), vaultID); err != nil {
		t.Fatal(err)
	}
	second := svc.List("curator", vaultID)
	if second.Session == nil || second.Session.SessionID == first.Session.SessionID {
		t.Fatalf("session not reset: %+v -> %+v", first.Session, second.Session)
	}
	if err := svc.Cancel("nobody"); err == nil {
		t.Fatal("cancel unknown agent accepted")
	}
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func TestAgentBusyWiredAndBlocksActivate(t *testing.T) {
	server := promptServer(t)
	app, vaultID, _ := promptApp(t, server)
	if app.AgentBusy == nil {
		t.Fatal("AgentBusy is not wired")
	}
	svc := NewAgentService(app)
	sink := newFakeSink()
	done := make(chan struct{})
	go func() {
		_, _ = svc.prompt(context.Background(), "curator", vaultID, "写入一篇笔记", sink)
		close(done)
	}()
	waitCard(t, sink)
	if !app.AgentBusy(vaultID) {
		t.Fatal("a running turn was not reported busy")
	}
	conversations := NewConversationService(app)
	created := conversations.Create(vaultID)
	if created.Failure != nil {
		t.Fatalf("create: %+v", created.Failure)
	}
	result := conversations.Activate(vaultID, created.Conversation.ID)
	if result.Failure == nil || result.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("activate during a turn: %+v", result)
	}
	if err := svc.Cancel("curator"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("prompt did not stop")
	}
}
