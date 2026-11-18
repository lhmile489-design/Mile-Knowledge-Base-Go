package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

func sseServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, call int)) (*httptest.Server, *int32) {
	t.Helper()
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := int(atomic.AddInt32(&calls, 1))
		handler(w, r, call)
	}))
	t.Cleanup(server.Close)
	return server, &calls
}

func writeSSE(w http.ResponseWriter, lines ...string) {
	w.Header().Set("Content-Type", "text/event-stream")
	for _, line := range lines {
		_, _ = io.WriteString(w, line+"\n\n")
	}
}

func TestStreamChatHappyPath(t *testing.T) {
	server, _ := sseServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Errorf("auth header: %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		if payload["stream"] != true || payload["model"] != "deepseek-v4-flash" {
			t.Errorf("body: %s", body)
		}
		if opts, ok := payload["stream_options"].(map[string]any); !ok || opts["include_usage"] != true {
			t.Errorf("stream_options: %s", body)
		}
		writeSSE(w,
			`data: {"choices":[{"delta":{"reasoning_content":"想"}}]}`,
			`data: {"choices":[{"delta":{"content":"你好"}}]}`,
			`data: {"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`,
			`data: [DONE]`,
		)
	})

	var text, reasoning string
	var usage *contract.Usage
	err := StreamChat(context.Background(), ChatRequest{
		BaseURL: server.URL, APIKey: "sk-test", Model: "deepseek-v4-flash",
		Messages: []ChatMessage{{Role: "user", Content: ptr("hi")}},
	}, StreamOptions{Client: server.Client()}, func(chunk StreamChunk) error {
		switch chunk.Kind {
		case "text":
			text += chunk.Text
		case "reasoning":
			reasoning += chunk.Text
		case "usage":
			usage = chunk.Usage
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if text != "你好" || reasoning != "想" {
		t.Fatalf("text=%q reasoning=%q", text, reasoning)
	}
	if usage == nil || usage.TotalTokens != 7 {
		t.Fatalf("usage: %+v", usage)
	}
}

func TestStreamChatAuthNoRetry(t *testing.T) {
	server, calls := sseServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
		w.WriteHeader(401)
		_, _ = io.WriteString(w, "bad key")
	})
	err := StreamChat(context.Background(), ChatRequest{BaseURL: server.URL, Model: "m"}, StreamOptions{Client: server.Client()}, func(StreamChunk) error { return nil })
	var engineErr *EngineError
	if !errors.As(err, &engineErr) || engineErr.Code != contract.CodePermissionDenied {
		t.Fatalf("err: %v", err)
	}
	if atomic.LoadInt32(calls) != 1 {
		t.Fatalf("401 retried: %d", *calls)
	}
}

func TestStreamChatRetryAfter(t *testing.T) {
	server, calls := sseServer(t, func(w http.ResponseWriter, _ *http.Request, call int) {
		if call == 1 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(429)
			return
		}
		writeSSE(w, `data: {"choices":[{"delta":{"content":"ok"},"finish_reason":"stop"}]}`, `data: [DONE]`)
	})
	var retry RetryInfo
	err := StreamChat(context.Background(), ChatRequest{BaseURL: server.URL, Model: "m"}, StreamOptions{
		Client:  server.Client(),
		OnRetry: func(info RetryInfo) { retry = info },
		Delay:   func(_ int, retryAfter time.Duration) time.Duration { return retryAfter },
	}, func(StreamChunk) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(calls) != 2 {
		t.Fatalf("calls: %d", *calls)
	}
	if retry.Delay != 2*time.Second || retry.Attempt != 1 {
		t.Fatalf("retry info: %+v", retry)
	}
}

func TestStreamChatExhausted(t *testing.T) {
	server, calls := sseServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
		w.WriteHeader(500)
		_, _ = io.WriteString(w, "boom")
	})
	max := 2
	err := StreamChat(context.Background(), ChatRequest{BaseURL: server.URL, Model: "m"}, StreamOptions{
		Client: server.Client(), MaxRetries: &max,
		Delay: func(int, time.Duration) time.Duration { return 0 },
	}, func(StreamChunk) error { return nil })
	var engineErr *EngineError
	if !errors.As(err, &engineErr) || engineErr.Code != contract.CodeProviderError || !strings.Contains(engineErr.Message, "boom") {
		t.Fatalf("err: %v", err)
	}
	if atomic.LoadInt32(calls) != 3 {
		t.Fatalf("calls: %d", *calls)
	}
}

func TestStreamChatIncomplete(t *testing.T) {
	server, _ := sseServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
		writeSSE(w, `data: {"choices":[{"delta":{"content":"半截"}}]}`)
	})
	var text string
	err := StreamChat(context.Background(), ChatRequest{BaseURL: server.URL, Model: "m"}, StreamOptions{Client: server.Client()}, func(chunk StreamChunk) error {
		if chunk.Kind == "text" {
			text += chunk.Text
		}
		return nil
	})
	var engineErr *EngineError
	if !errors.As(err, &engineErr) || engineErr.Code != contract.CodeProviderError || !strings.Contains(engineErr.Message, "完成前就结束") {
		t.Fatalf("err: %v", err)
	}
	if text != "半截" {
		t.Fatalf("already received text must be kept: %q", text)
	}
}

func TestStreamChatIdleTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, `data: {"choices":[{"delta":{"content":"开始"}}]}`+"\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	err := StreamChat(context.Background(), ChatRequest{BaseURL: server.URL, Model: "m"}, StreamOptions{
		Client: server.Client(), IdleTimeout: 80 * time.Millisecond,
	}, func(StreamChunk) error { return nil })
	var engineErr *EngineError
	if !errors.As(err, &engineErr) || engineErr.Code != contract.CodeProviderError || !strings.Contains(engineErr.Message, "没有返回任何数据") {
		t.Fatalf("err: %v", err)
	}
}

func TestStreamChatCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, `data: {"choices":[{"delta":{"content":"x"}}]}`+"\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(80 * time.Millisecond)
		cancel()
	}()
	err := StreamChat(ctx, ChatRequest{BaseURL: server.URL, Model: "m"}, StreamOptions{Client: server.Client()}, func(StreamChunk) error { return nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
}

func TestStreamChatRejectsExternalPlainHTTP(t *testing.T) {
	err := StreamChat(context.Background(), ChatRequest{BaseURL: "http://example.com/v1", Model: "m"}, StreamOptions{}, func(StreamChunk) error { return nil })
	var engineErr *EngineError
	if !errors.As(err, &engineErr) || engineErr.Code != contract.CodeInvalidInput {
		t.Fatalf("err: %v", err)
	}
}

func TestStreamChatRequestShape(t *testing.T) {
	var seen map[string]any
	server, _ := sseServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &seen)
		writeSSE(w, `data: [DONE]`)
	})
	tools := []json.RawMessage{json.RawMessage(`{"type":"function","function":{"name":"search_notes"}}`)}
	maxTokens := 256
	err := StreamChat(context.Background(), ChatRequest{
		BaseURL: server.URL + "/", Model: "m", MaxTokens: maxTokens, Tools: tools,
		Messages: []ChatMessage{
			{Role: "assistant", Content: nil, ReasoningContent: ptr("why"), ToolCalls: []ChatToolCallWire{{
				ID: "call_1", Type: "function", Function: ToolFunction{Name: "search_notes", Arguments: `{}`},
			}}},
			{Role: "tool", Content: ptr("result"), ToolCallID: "call_1"},
		},
	}, StreamOptions{Client: server.Client()}, func(StreamChunk) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if seen["max_tokens"] != float64(256) {
		t.Fatalf("max_tokens: %v", seen["max_tokens"])
	}
	if tools, ok := seen["tools"].([]any); !ok || len(tools) != 1 {
		t.Fatalf("tools: %v", seen["tools"])
	}
	messages := seen["messages"].([]any)
	assistant := messages[0].(map[string]any)
	if assistant["reasoning_content"] != "why" {
		t.Fatalf("reasoning_content must be sent back: %v", assistant)
	}
	if assistant["content"] != nil {
		t.Fatalf("tool-only assistant content must be null: %v", assistant)
	}
	toolMessage := messages[1].(map[string]any)
	if toolMessage["tool_call_id"] != "call_1" {
		t.Fatalf("tool_call_id: %v", toolMessage)
	}
	_ = fmt.Sprint()
}
