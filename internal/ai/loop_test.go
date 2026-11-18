package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeRunner struct {
	schemas []json.RawMessage
	run     func(ctx context.Context, call CompletedToolCall) (string, error)
	mu      sync.Mutex
	calls   []CompletedToolCall
}

func (f *fakeRunner) Schemas() []json.RawMessage {
	if f.schemas == nil {
		return []json.RawMessage{json.RawMessage(`{"type":"function","function":{"name":"search_notes"}}`)}
	}
	return f.schemas
}

func (f *fakeRunner) Run(ctx context.Context, call CompletedToolCall) (string, error) {
	f.mu.Lock()
	f.calls = append(f.calls, call)
	f.mu.Unlock()
	if f.run != nil {
		return f.run(ctx, call)
	}
	return "ok", nil
}

func (f *fakeRunner) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func loopDeps(server *httptest.Server, tools ToolRunner, events *[]TurnEvent) TurnDeps {
	noRetry := 0
	return TurnDeps{
		Providers: []Provider{{Name: "p", Kind: "openai", BaseURL: server.URL, Models: []string{"m"}, DefaultModel: "m"}},
		Configs:   DefaultAgentConfigs(),
		Keys:      NewMemoryKeyStore(),
		Tools:     tools,
		Stream:    StreamOptions{Client: server.Client(), MaxRetries: &noRetry},
		OnEvent: func(event TurnEvent) {
			if events != nil {
				*events = append(*events, event)
			}
		},
	}
}

func kinds(events []TurnEvent) []string {
	out := make([]string, 0, len(events))
	for _, event := range events {
		out = append(out, event.Kind)
	}
	return out
}

func TestRunTurnPlainAnswer(t *testing.T) {
	server, _ := sseServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
		writeSSE(w,
			`data: {"choices":[{"delta":{"content":"你好"}}]}`,
			`data: {"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`,
			`data: [DONE]`)
	})
	var events []TurnEvent
	session := NewRegistry().Get("scout", "v1")
	reason, err := RunTurn(context.Background(), session, "打个招呼", "", loopDeps(server, &fakeRunner{}, &events))
	if err != nil || reason != "end_turn" {
		t.Fatalf("reason=%q err=%v", reason, err)
	}
	messages := session.Messages()
	if len(messages) != 2 || messages[0].Role != "user" || messages[1].Role != "assistant" {
		t.Fatalf("history: %+v", messages)
	}
	if *messages[1].Content != "你好" || messages[1].ToolCalls != nil {
		t.Fatalf("assistant: %+v", messages[1])
	}
	var usage *TurnUsageFrame
	for _, event := range events {
		if event.Kind == EventUsage {
			usage = event.Usage
		}
	}
	if usage == nil || usage.Turn.TotalTokens != 7 || usage.Cumulative.TotalTokens != 7 {
		t.Fatalf("usage: %+v", usage)
	}
	if events[len(events)-1].Kind != EventTurnEnd || events[len(events)-1].StopReason != "end_turn" {
		t.Fatalf("last event: %+v", events[len(events)-1])
	}
}

func TestRunTurnToolLoop(t *testing.T) {
	server, calls := sseServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		body, _ := io.ReadAll(r.Body)
		var payload struct {
			Messages []struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
		}
		_ = json.Unmarshal(body, &payload)
		last := payload.Messages[len(payload.Messages)-1]
		if last.Role == "user" {
			writeSSE(w,
				`data: {"choices":[{"delta":{"reasoning_content":"先搜"}}]}`,
				`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"name":"search_notes","arguments":"{\"query\":"}}]}}]}`,
				`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"x\"}"}}]}}]}`,
				`data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`,
				`data: [DONE]`)
			return
		}
		writeSSE(w,
			`data: {"choices":[{"delta":{"content":"找到了"}}]}`,
			`data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`,
			`data: [DONE]`)
	})
	runner := &fakeRunner{}
	var events []TurnEvent
	session := NewRegistry().Get("scout", "v1")
	reason, err := RunTurn(context.Background(), session, "搜索 x", "", loopDeps(server, runner, &events))
	if err != nil || reason != "end_turn" {
		t.Fatalf("reason=%q err=%v", reason, err)
	}
	if atomicCalls := *calls; atomicCalls != 2 {
		t.Fatalf("provider calls: %d", atomicCalls)
	}
	if runner.count() != 1 {
		t.Fatalf("tool calls: %d", runner.count())
	}
	got := runner.calls[0]
	if got.ID != "call_0" || got.Name != "search_notes" || got.Arguments != `{"query":"x"}` {
		t.Fatalf("tool call: %+v", got)
	}
	messages := session.Messages()
	if len(messages) != 4 {
		t.Fatalf("history: %+v", messages)
	}
	assistant := messages[1]
	if assistant.ToolCalls == nil || len(assistant.ToolCalls) != 1 || assistant.ReasoningContent == nil || *assistant.ReasoningContent != "先搜" {
		t.Fatalf("assistant with calls: %+v", assistant)
	}
	if assistant.Content != nil {
		t.Fatalf("tool-only assistant content must be null: %+v", assistant.Content)
	}
	if messages[2].Role != "tool" || messages[2].ToolCallID != "call_0" || *messages[2].Content != "ok" {
		t.Fatalf("tool result: %+v", messages[2])
	}
	seenPending, seenCompleted := false, false
	for _, event := range events {
		if event.Kind == EventTool && event.ToolStatus == "pending" {
			seenPending = true
		}
		if event.Kind == EventTool && event.ToolStatus == "completed" {
			seenCompleted = true
		}
	}
	if !seenPending || !seenCompleted {
		t.Fatalf("tool events: %v", kinds(events))
	}
}

func TestRunTurnStepLimit(t *testing.T) {
	server, _ := sseServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
		writeSSE(w,
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"name":"search_notes","arguments":"{}"}}]}}]}`,
			`data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`,
			`data: [DONE]`)
	})
	deps := loopDeps(server, &fakeRunner{}, nil)
	deps.MaxSteps = 2
	session := NewRegistry().Get("scout", "v1")
	reason, err := RunTurn(context.Background(), session, "循环", "", deps)
	if err != nil || reason != "max_turn_requests" {
		t.Fatalf("reason=%q err=%v", reason, err)
	}
	if len(session.Messages()) != 5 {
		t.Fatalf("history: %+v", session.Messages())
	}
}

func TestRunTurnCancelBackfillsResults(t *testing.T) {
	server, _ := sseServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
		writeSSE(w,
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"name":"search_notes","arguments":"{}"}}]}}]}`,
			`data: {"choices":[{"delta":{"tool_calls":[{"index":1,"function":{"name":"read_note","arguments":"{}"}}]}}]}`,
			`data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`,
			`data: [DONE]`)
	})
	started := make(chan struct{}, 1)
	runner := &fakeRunner{run: func(ctx context.Context, _ CompletedToolCall) (string, error) {
		started <- struct{}{}
		<-ctx.Done()
		return "", ctx.Err()
	}}
	session := NewRegistry().Get("scout", "v1")
	done := make(chan struct {
		reason string
		err    error
	}, 1)
	go func() {
		reason, err := RunTurn(context.Background(), session, "取消", "", loopDeps(server, runner, nil))
		done <- struct {
			reason string
			err    error
		}{reason, err}
	}()
	<-started
	session.Cancel()
	select {
	case got := <-done:
		if got.err != nil || got.reason != "cancelled" {
			t.Fatalf("reason=%q err=%v", got.reason, got.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("turn did not stop after cancel")
	}
	messages := session.Messages()
	var results []string
	for _, message := range messages {
		if message.Role == "tool" {
			results = append(results, *message.Content)
		}
	}
	if len(results) != 2 || results[0] != CancelledResult || results[1] != CancelledResult {
		t.Fatalf("cancel backfill: %v", results)
	}
}

func TestRunTurnToolErrorIsText(t *testing.T) {
	server, _ := sseServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), `"role":"tool"`) {
			writeSSE(w, `data: {"choices":[{"delta":{"content":"继续"},"finish_reason":"stop"}]}`, `data: [DONE]`)
			return
		}
		writeSSE(w,
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"name":"search_notes","arguments":"{}"}}]}}]}`,
			`data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`,
			`data: [DONE]`)
	})
	runner := &fakeRunner{run: func(context.Context, CompletedToolCall) (string, error) {
		return "参数不合法：query 必填", nil
	}}
	session := NewRegistry().Get("scout", "v1")
	reason, err := RunTurn(context.Background(), session, "错参数", "", loopDeps(server, runner, nil))
	if err != nil || reason != "end_turn" {
		t.Fatalf("reason=%q err=%v", reason, err)
	}
	messages := session.Messages()
	if *messages[2].Content != "参数不合法：query 必填" {
		t.Fatalf("tool text: %q", *messages[2].Content)
	}
}

func TestRunTurnStreamErrorKeepsQuestionOnly(t *testing.T) {
	server, _ := sseServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
		w.WriteHeader(500)
		_, _ = io.WriteString(w, "boom")
	})
	var events []TurnEvent
	session := NewRegistry().Get("scout", "v1")
	reason, err := RunTurn(context.Background(), session, "失败轮", "", loopDeps(server, &fakeRunner{}, &events))
	if err == nil || reason != "error" {
		t.Fatalf("reason=%q err=%v", reason, err)
	}
	messages := session.Messages()
	if len(messages) != 1 || messages[0].Role != "user" {
		t.Fatalf("failed turn must keep only the question: %+v", messages)
	}
	found := false
	for _, event := range events {
		if event.Kind == EventError {
			found = true
		}
	}
	if !found {
		t.Fatalf("no error event: %v", kinds(events))
	}
}

func TestRunTurnPanicIsolation(t *testing.T) {
	server, _ := sseServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
		writeSSE(w, `data: {"choices":[{"delta":{"content":"ok"},"finish_reason":"stop"}]}`, `data: [DONE]`)
	})
	session := NewRegistry().Get("scout", "v1")
	deps := loopDeps(server, &fakeRunner{}, nil)
	deps.OnEvent = func(TurnEvent) { panic("listener blew up") }
	reason, err := RunTurn(context.Background(), session, "触发", "", deps)
	if err == nil || reason != "error" {
		t.Fatalf("reason=%q err=%v", reason, err)
	}
	if session.Busy {
		t.Fatal("session left busy after panic")
	}

	second := loopDeps(server, &fakeRunner{}, nil)
	if reason, err := RunTurn(context.Background(), session, "再来", "", second); err != nil || reason != "end_turn" {
		t.Fatalf("session unusable after panic: %q %v", reason, err)
	}
}

func TestRunTurnCompactsLongHistory(t *testing.T) {
	server, _ := sseServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "<compaction-summary>") || strings.Contains(string(body), "整理成一份续接用的摘要") {
			writeSSE(w, `data: {"choices":[{"delta":{"content":"摘要正文"},"finish_reason":"stop"}]}`, `data: [DONE]`)
			return
		}
		writeSSE(w, `data: {"choices":[{"delta":{"content":"回答"},"finish_reason":"stop"}]}`, `data: [DONE]`)
	})
	window := 400
	deps := loopDeps(server, &fakeRunner{}, nil)
	deps.Providers = []Provider{{Name: "p", Kind: "openai", BaseURL: server.URL, Models: []string{"m"}, DefaultModel: "m", ContextWindow: &window}}
	session := NewRegistry().Get("scout", "v1")
	long := strings.Repeat("字", 300)
	session.setMessages([]ChatMessage{
		{Role: "user", Content: ptr(long)}, {Role: "assistant", Content: ptr(long)},
		{Role: "user", Content: ptr(long)}, {Role: "assistant", Content: ptr(long)},
	})
	reason, err := RunTurn(context.Background(), session, "继续", "", deps)
	if err != nil || reason != "end_turn" {
		t.Fatalf("reason=%q err=%v", reason, err)
	}
	messages := session.Messages()
	if !IsSummaryMessage(messages[0]) || !strings.Contains(*messages[0].Content, "摘要正文") {
		t.Fatalf("expected a summary at the front: %+v", messages[0])
	}
}

func TestDefaultLimits(t *testing.T) {
	if DefaultMaxSteps != 16 {
		t.Fatalf("step limit: %d", DefaultMaxSteps)
	}
	if ToolResultMaxChars != 20_000 {
		t.Fatalf("tool result cap: %d", ToolResultMaxChars)
	}
}
