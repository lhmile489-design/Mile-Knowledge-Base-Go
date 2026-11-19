package ai

// T19: one programmable fake OpenAI-compatible provider drives complete turns
// through the real loop — streaming, tool calls, approvals, cancellation,
// retries, truncated streams and usage accounting (AC-M2 in Go form).

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeOpenAI records every request body and answers through the reply hook.
type fakeOpenAI struct {
	mu     sync.Mutex
	calls  int
	bodies []map[string]any
	server *httptest.Server
}

func newFakeOpenAI(t *testing.T, reply func(w http.ResponseWriter, r *http.Request, call int, body map[string]any)) *fakeOpenAI {
	t.Helper()
	fake := &fakeOpenAI{}
	fake.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		body := map[string]any{}
		_ = json.Unmarshal(raw, &body)
		fake.mu.Lock()
		fake.calls++
		call := fake.calls
		fake.bodies = append(fake.bodies, body)
		fake.mu.Unlock()
		reply(w, r, call, body)
	}))
	t.Cleanup(fake.server.Close)
	return fake
}

func (f *fakeOpenAI) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *fakeOpenAI) requestBody(index int) map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.bodies[index]
}

// deps builds the loop dependencies against the fake provider.
func (f *fakeOpenAI) deps(tools ToolRunner, log *eventLog) TurnDeps {
	noRetry := 0
	return TurnDeps{
		Providers: []Provider{{
			Name: "fake", Kind: "openai", BaseURL: f.server.URL,
			Models: []string{"fake-model"}, DefaultModel: "fake-model",
		}},
		Configs: DefaultAgentConfigs(),
		Keys:    NewMemoryKeyStore(),
		Tools:   tools,
		Stream:  StreamOptions{Client: f.server.Client(), MaxRetries: &noRetry},
		OnEvent: log.add,
	}
}

// eventLog collects loop events from the turn goroutine.
type eventLog struct {
	mu     sync.Mutex
	events []TurnEvent
}

func (l *eventLog) add(event TurnEvent) {
	l.mu.Lock()
	l.events = append(l.events, event)
	l.mu.Unlock()
}

func (l *eventLog) all() []TurnEvent {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]TurnEvent{}, l.events...)
}

func (l *eventLog) count(kind string) int {
	total := 0
	for _, event := range l.all() {
		if event.Kind == kind {
			total++
		}
	}
	return total
}

func (l *eventLog) first(kind string) (TurnEvent, bool) {
	for _, event := range l.all() {
		if event.Kind == kind {
			return event, true
		}
	}
	return TurnEvent{}, false
}

// sseData writes one SSE frame and flushes it immediately.
func sseData(w http.ResponseWriter, payload string) {
	_, _ = io.WriteString(w, "data: "+payload+"\n\n")
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func sseDone(w http.ResponseWriter) {
	_, _ = io.WriteString(w, "data: [DONE]\n\n")
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func TestFakeProviderStreamingTurn(t *testing.T) {
	fake := newFakeOpenAI(t, func(w http.ResponseWriter, _ *http.Request, call int, body map[string]any) {
		if call != 1 {
			t.Errorf("unexpected request %d", call)
		}
		if body["stream"] != true {
			t.Errorf("stream flag: %v", body["stream"])
		}
		if options, ok := body["stream_options"].(map[string]any); !ok || options["include_usage"] != true {
			t.Errorf("stream_options: %v", body["stream_options"])
		}
		w.Header().Set("Content-Type", "text/event-stream")
		sseData(w, `{"choices":[{"delta":{"reasoning_content":"想一下"}}]}`)
		sseData(w, `{"choices":[{"delta":{"content":"你好，"}}]}`)
		sseData(w, `{"choices":[{"delta":{"content":"世界"}}]}`)
		sseData(w, `{"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":3,"total_tokens":13,"prompt_cache_hit_tokens":6,"prompt_cache_miss_tokens":4}}`)
		sseDone(w)
	})

	var log eventLog
	session := NewRegistry().Get("general", "v1")
	reason, err := RunTurn(context.Background(), session, "打个招呼", "", fake.deps(&fakeRunner{}, &log))
	if err != nil || reason != "end_turn" {
		t.Fatalf("reason=%q err=%v", reason, err)
	}
	if log.count(EventThought) != 1 || log.count(EventText) != 2 || log.count(EventTurnEnd) != 1 {
		t.Fatalf("events: %+v", log.all())
	}
	usageEvent, ok := log.first(EventUsage)
	if !ok || usageEvent.Usage == nil {
		t.Fatalf("no usage event: %+v", log.all())
	}
	turn := usageEvent.Usage.Turn
	if turn.PromptTokens != 10 || turn.CompletionTokens != 3 || turn.TotalTokens != 13 {
		t.Fatalf("turn usage: %+v", turn)
	}
	if turn.CacheHitTokens != 6 || turn.CacheMissTokens != 4 {
		t.Fatalf("cache usage: %+v", turn)
	}
	if usageEvent.Usage.Cumulative.TotalTokens != 13 {
		t.Fatalf("cumulative: %+v", usageEvent.Usage.Cumulative)
	}
	messages := session.Messages()
	last := messages[len(messages)-1]
	if last.Role != "assistant" || last.Content == nil || *last.Content != "你好，世界" {
		t.Fatalf("assistant message: %+v", last)
	}
}

func TestFakeProviderToolCallLoop(t *testing.T) {
	fake := newFakeOpenAI(t, func(w http.ResponseWriter, _ *http.Request, call int, body map[string]any) {
		w.Header().Set("Content-Type", "text/event-stream")
		switch call {
		case 1:
			sseData(w, `{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_a","type":"function","function":{"name":"search_notes","arguments":"{\"query\":\"a\"}"}}]}}]}`)
			sseData(w, `{"choices":[{"delta":{"tool_calls":[{"index":1,"id":"call_b","type":"function","function":{"name":"list_dir","arguments":"{\"path\":\"\"}"}}]}}]}`)
			sseData(w, `{"choices":[{"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`)
		case 2:
			messages, _ := body["messages"].([]any)
			var assistant map[string]any
			tools := 0
			for _, raw := range messages {
				message, _ := raw.(map[string]any)
				if message["role"] == "assistant" {
					assistant = message
				}
				if message["role"] == "tool" {
					tools++
				}
			}
			if assistant == nil {
				t.Fatalf("second request lost the assistant message")
			}
			calls, _ := assistant["tool_calls"].([]any)
			if len(calls) != 2 {
				t.Fatalf("assistant tool_calls: %v", assistant["tool_calls"])
			}
			if _, ok := assistant["reasoning_content"]; !ok {
				t.Fatalf("assistant message has no reasoning_content: %v", assistant)
			}
			if tools != 2 {
				t.Fatalf("tool results: %d", tools)
			}
			sseData(w, `{"choices":[{"delta":{"content":"收尾完成"}}]}`)
			sseData(w, `{"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":9,"completion_tokens":4,"total_tokens":13}}`)
		default:
			t.Fatalf("unexpected request %d", call)
		}
		sseDone(w)
	})

	var log eventLog
	runner := &fakeRunner{}
	session := NewRegistry().Get("general", "v1")
	reason, err := RunTurn(context.Background(), session, "找一下", "", fake.deps(runner, &log))
	if err != nil || reason != "end_turn" {
		t.Fatalf("reason=%q err=%v", reason, err)
	}
	if runner.count() != 2 {
		t.Fatalf("tool runs: %d", runner.count())
	}
	if runner.calls[0].Name != "search_notes" || runner.calls[1].Name != "list_dir" {
		t.Fatalf("tool order: %+v", runner.calls)
	}
	completed := 0
	for _, event := range log.all() {
		if event.Kind == EventTool && event.ToolStatus == "completed" {
			completed++
		}
	}
	if completed != 2 {
		t.Fatalf("tool events: %+v", log.all())
	}
	var lastUsage *TurnUsageFrame
	for _, event := range log.all() {
		if event.Kind == EventUsage {
			lastUsage = event.Usage
		}
	}
	if lastUsage == nil || lastUsage.Cumulative.TotalTokens != 20 {
		t.Fatalf("cumulative usage: %+v", lastUsage)
	}
	messages := session.Messages()
	last := messages[len(messages)-1]
	if last.Role != "assistant" || last.Content == nil || *last.Content != "收尾完成" {
		t.Fatalf("final message: %+v", last)
	}
}

// approvalTurn wires one write_note tool call and an approval manager whose
// pending card is answered by the decision hook. The returned channel closes
// once the decision ran, so the test reads state without a race.
func approvalTurn(t *testing.T, decide func(manager *ApprovalManager, requestID string)) (*fakeOpenAI, *eventLog, *fakeRunner, *int, <-chan struct{}) {
	t.Helper()
	fake := newFakeOpenAI(t, func(w http.ResponseWriter, _ *http.Request, call int, _ map[string]any) {
		w.Header().Set("Content-Type", "text/event-stream")
		switch call {
		case 1:
			sseData(w, `{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_w","type":"function","function":{"name":"write_note","arguments":"{\"path\":\"raws/a.md\",\"content\":\"hi\"}"}}]}}]}`)
			sseData(w, `{"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`)
		case 2:
			sseData(w, `{"choices":[{"delta":{"content":"已经处理"}}]}`)
			sseData(w, `{"choices":[{"delta":{},"finish_reason":"stop"}]}`)
		}
		sseDone(w)
	})
	log := &eventLog{}
	decided := make(chan struct{})
	writes := new(int)
	var manager *ApprovalManager
	manager = NewApprovalManager(
		func(string) string { return ApprovalAsk },
		func(event PermEvent) {
			log.add(TurnEvent{Kind: "perm-" + event.Status, ToolID: event.RequestID})
			if event.Status == "pending" {
				go func() {
					decide(manager, event.RequestID)
					close(decided)
				}()
			}
		},
	)
	runner := &fakeRunner{run: func(ctx context.Context, _ CompletedToolCall) (string, error) {
		if err := manager.AllowWrite(ctx, "s1", "raws/a.md", nil); err != nil {
			return "拒绝：" + err.Error(), nil
		}
		*writes++
		return "written", nil
	}}
	return fake, log, runner, writes, decided
}

func waitDecided(t *testing.T, decided <-chan struct{}) {
	t.Helper()
	select {
	case <-decided:
	case <-time.After(5 * time.Second):
		t.Fatal("the approval decision never ran")
	}
}

func TestFakeProviderApprovalAllow(t *testing.T) {
	fake, log, runner, writes, decided := approvalTurn(t, func(manager *ApprovalManager, requestID string) {
		if failure := manager.Approve(requestID, "allow_once"); failure != nil {
			t.Errorf("approve: %+v", failure)
		}
	})
	session := NewRegistry().Get("general", "v1")
	reason, err := RunTurn(context.Background(), session, "写入", "", fake.deps(runner, log))
	if err != nil || reason != "end_turn" {
		t.Fatalf("reason=%q err=%v", reason, err)
	}
	waitDecided(t, decided)
	if *writes != 1 {
		t.Fatalf("writes: %d", *writes)
	}
	if log.count("perm-pending") != 1 || log.count("perm-approved") != 1 {
		t.Fatalf("approval events: %+v", log.all())
	}
}

func TestFakeProviderApprovalReject(t *testing.T) {
	fake, log, runner, writes, decided := approvalTurn(t, func(manager *ApprovalManager, requestID string) {
		if failure := manager.Reject(requestID); failure != nil {
			t.Errorf("reject: %+v", failure)
		}
	})
	session := NewRegistry().Get("general", "v1")
	reason, err := RunTurn(context.Background(), session, "写入", "", fake.deps(runner, log))
	if err != nil || reason != "end_turn" {
		t.Fatalf("reason=%q err=%v", reason, err)
	}
	waitDecided(t, decided)
	if *writes != 0 {
		t.Fatalf("rejected write still executed")
	}
	if log.count("perm-rejected") != 1 {
		t.Fatalf("approval events: %+v", log.all())
	}
	var output string
	for _, message := range session.Messages() {
		if message.Role == "tool" && message.Content != nil {
			output = *message.Content
		}
	}
	if !strings.Contains(output, "用户拒绝了这次操作") {
		t.Fatalf("tool result: %q", output)
	}
}

func TestFakeProviderCancelDuringToolCall(t *testing.T) {
	fake := newFakeOpenAI(t, func(w http.ResponseWriter, _ *http.Request, call int, _ map[string]any) {
		w.Header().Set("Content-Type", "text/event-stream")
		if call != 1 {
			t.Fatalf("unexpected request %d", call)
		}
		sseData(w, `{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_a","type":"function","function":{"name":"search_notes","arguments":"{}"}}]}}]}`)
		sseData(w, `{"choices":[{"delta":{"tool_calls":[{"index":1,"id":"call_b","type":"function","function":{"name":"list_dir","arguments":"{}"}}]}}]}`)
		sseData(w, `{"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`)
		sseDone(w)
	})
	started := make(chan struct{}, 2)
	runner := &fakeRunner{run: func(ctx context.Context, _ CompletedToolCall) (string, error) {
		started <- struct{}{}
		<-ctx.Done()
		return "", ctx.Err()
	}}
	var log eventLog
	session := NewRegistry().Get("general", "v1")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	var reason string
	var err error
	go func() {
		reason, err = RunTurn(ctx, session, "找一下", "", fake.deps(runner, &log))
		close(done)
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("tool never started")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("turn did not stop")
	}
	if err != nil || reason != "cancelled" {
		t.Fatalf("reason=%q err=%v", reason, err)
	}
	if log.count(EventError) != 0 {
		t.Fatalf("cancel emitted an error: %+v", log.all())
	}
	cancelled := 0
	for _, message := range session.Messages() {
		if message.Role == "tool" && message.Content != nil && *message.Content == CancelledResult {
			cancelled++
		}
	}
	if cancelled != 2 {
		t.Fatalf("cancelled tool results: %d", cancelled)
	}
}

func TestFakeProviderCancelMidStream(t *testing.T) {
	sent := make(chan struct{})
	var once sync.Once
	fake := newFakeOpenAI(t, func(w http.ResponseWriter, r *http.Request, _ int, _ map[string]any) {
		w.Header().Set("Content-Type", "text/event-stream")
		sseData(w, `{"choices":[{"delta":{"content":"半"}}]}`)
		once.Do(func() { close(sent) })
		<-r.Context().Done()
	})
	var log eventLog
	session := NewRegistry().Get("general", "v1")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	var reason string
	var err error
	go func() {
		reason, err = RunTurn(ctx, session, "回答一半", "", fake.deps(&fakeRunner{}, &log))
		close(done)
	}()
	select {
	case <-sent:
	case <-time.After(5 * time.Second):
		t.Fatal("stream never started")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("turn did not stop")
	}
	if err != nil || reason != "cancelled" {
		t.Fatalf("reason=%q err=%v", reason, err)
	}
	if log.count(EventError) != 0 {
		t.Fatalf("cancel emitted an error: %+v", log.all())
	}
	if messages := session.Messages(); len(messages) != 1 || messages[0].Role != "user" {
		t.Fatalf("half answer entered history: %+v", messages)
	}
}

func TestFakeProviderRetryAfter429(t *testing.T) {
	fake := newFakeOpenAI(t, func(w http.ResponseWriter, _ *http.Request, call int, _ map[string]any) {
		if call == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		sseData(w, `{"choices":[{"delta":{"content":"重试成功"}}]}`)
		sseData(w, `{"choices":[{"delta":{},"finish_reason":"stop"}]}`)
		sseDone(w)
	})
	deps := fake.deps(&fakeRunner{}, &eventLog{})
	noRetry := 3
	deps.Stream.MaxRetries = &noRetry
	deps.Stream.Delay = func(int, time.Duration) time.Duration { return 0 }
	var log eventLog
	deps.OnEvent = log.add
	session := NewRegistry().Get("general", "v1")
	reason, err := RunTurn(context.Background(), session, "再说一次", "", deps)
	if err != nil || reason != "end_turn" {
		t.Fatalf("reason=%q err=%v", reason, err)
	}
	if fake.callCount() != 2 {
		t.Fatalf("requests: %d", fake.callCount())
	}
	if log.count(EventError) != 0 {
		t.Fatalf("retry recovered but emitted an error: %+v", log.all())
	}
}

func TestFakeProviderTruncatedStream(t *testing.T) {
	fake := newFakeOpenAI(t, func(w http.ResponseWriter, _ *http.Request, _ int, _ map[string]any) {
		w.Header().Set("Content-Type", "text/event-stream")
		sseData(w, `{"choices":[{"delta":{"content":"半截"}}]}`)
	})
	var log eventLog
	session := NewRegistry().Get("general", "v1")
	reason, err := RunTurn(context.Background(), session, "写一段", "", fake.deps(&fakeRunner{}, &log))
	if err == nil || reason != "error" {
		t.Fatalf("reason=%q err=%v", reason, err)
	}
	errorEvent, ok := log.first(EventError)
	if !ok || !strings.Contains(errorEvent.Message, "完成前就结束") {
		t.Fatalf("error event: %+v", log.all())
	}
	messages := session.Messages()
	if len(messages) != 1 || messages[0].Role != "user" {
		t.Fatalf("half answer entered history: %+v", messages)
	}
}

func TestFakeProviderMissingKeyFailsLocally(t *testing.T) {
	fake := newFakeOpenAI(t, func(w http.ResponseWriter, _ *http.Request, _ int, _ map[string]any) {
		t.Errorf("a request went out without a key")
		sseData(w, `{"choices":[{"delta":{"content":"不该到这里"}}]}`)
		sseDone(w)
	})
	var log eventLog
	deps := fake.deps(&fakeRunner{}, &log)
	deps.Providers[0].APIKeyEnv = "MKB_TEST_KEY"
	deps.Keys = NewMemoryKeyStore()
	session := NewRegistry().Get("general", "v1")
	reason, err := RunTurn(context.Background(), session, "你好", "", deps)
	if err == nil || reason != "error" {
		t.Fatalf("reason=%q err=%v", reason, err)
	}
	if fake.callCount() != 0 {
		t.Fatalf("requests went out: %d", fake.callCount())
	}
	errorEvent, ok := log.first(EventError)
	if !ok || !strings.Contains(errorEvent.Message, "还没有 API key") {
		t.Fatalf("error event: %+v", log.all())
	}
}

func TestFakeProviderTruncatesToolResultWithNote(t *testing.T) {
	long := strings.Repeat("字", ToolResultMaxChars+25)
	fake := newFakeOpenAI(t, func(w http.ResponseWriter, _ *http.Request, call int, _ map[string]any) {
		w.Header().Set("Content-Type", "text/event-stream")
		if call == 1 {
			sseData(w, `{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_t","type":"function","function":{"name":"read_note","arguments":"{}"}}]}}]}`)
			sseData(w, `{"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`)
		} else {
			sseData(w, `{"choices":[{"delta":{"content":"收尾"}}]}`)
			sseData(w, `{"choices":[{"delta":{},"finish_reason":"stop"}]}`)
		}
		sseDone(w)
	})
	var log eventLog
	runner := &fakeRunner{run: func(context.Context, CompletedToolCall) (string, error) { return long, nil }}
	session := NewRegistry().Get("general", "v1")
	if reason, err := RunTurn(context.Background(), session, "读一下", "", fake.deps(runner, &log)); err != nil || reason != "end_turn" {
		t.Fatalf("reason=%q err=%v", reason, err)
	}
	var toolText string
	for _, message := range session.Messages() {
		if message.Role == "tool" && message.Content != nil {
			toolText = *message.Content
		}
	}
	want := fmt.Sprintf("…（内容过长，已截断，共 %d 字符）", ToolResultMaxChars+25)
	if !strings.Contains(toolText, want) {
		t.Fatalf("tool result note missing: %q", toolText)
	}
}
