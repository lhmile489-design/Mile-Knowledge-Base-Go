package ai

import (
	"strings"
	"testing"
)

func collect(t *testing.T, lines ...string) ([]StreamChunk, bool) {
	t.Helper()
	assembler := &Assembler{}
	var chunks []StreamChunk
	for _, line := range lines {
		got, done, err := assembler.Push(line)
		if err != nil {
			t.Fatalf("push %q: %v", line, err)
		}
		chunks = append(chunks, got...)
		if done {
			return chunks, true
		}
	}
	return chunks, false
}

func TestAssemblerTextAndReasoning(t *testing.T) {
	chunks, _ := collect(t,
		`data: {"choices":[{"delta":{"reasoning_content":"想"}}]}`,
		`data: {"choices":[{"delta":{"reasoning":"再想"}}]}`,
		`data: {"choices":[{"delta":{"content":"答"}}]}`,
		`data: [DONE]`,
	)
	var kinds []string
	for _, chunk := range chunks {
		kinds = append(kinds, chunk.Kind+":"+chunk.Text)
	}
	want := []string{"reasoning:想", "reasoning:再想", "text:答"}
	if strings.Join(kinds, "|") != strings.Join(want, "|") {
		t.Fatalf("chunks: %v", kinds)
	}
}

func TestAssemblerThinkInsideContent(t *testing.T) {
	assembler := &Assembler{}
	chunks, _, _ := assembler.Push(`data: {"choices":[{"delta":{"content":"<think>内联</think>正文"}}]}`)
	if len(chunks) != 2 || chunks[0].Kind != "reasoning" || chunks[0].Text != "内联" || chunks[1].Kind != "text" || chunks[1].Text != "正文" {
		t.Fatalf("inline think: %+v", chunks)
	}
}

func TestAssemblerUsage(t *testing.T) {
	chunks, _ := collect(t,
		`data: {"choices":[{"delta":{}}],"usage":{"prompt_tokens":100,"completion_tokens":20,"total_tokens":120,"prompt_cache_hit_tokens":30,"prompt_cache_miss_tokens":70,"completion_tokens_details":{"reasoning_tokens":5}}}`,
	)
	if len(chunks) != 1 || chunks[0].Kind != "usage" || chunks[0].Usage == nil {
		t.Fatalf("usage chunk: %+v", chunks)
	}
	usage := chunks[0].Usage
	if usage.PromptTokens != 100 || usage.CompletionTokens != 20 || usage.TotalTokens != 120 ||
		usage.CacheHitTokens != 30 || usage.CacheMissTokens != 70 || usage.ReasoningTokens != 5 {
		t.Fatalf("usage: %+v", usage)
	}

	// OpenAI only reports cached_tokens; miss is derived from prompt - hit.
	chunks, _ = collect(t, `data: {"choices":[],"usage":{"prompt_tokens":50,"completion_tokens":1,"prompt_tokens_details":{"cached_tokens":20}}}`)
	usage = chunks[0].Usage
	if usage.CacheHitTokens != 20 || usage.CacheMissTokens != 30 || usage.TotalTokens != 51 {
		t.Fatalf("derived usage: %+v", usage)
	}
}

func TestAssemblerToolCalls(t *testing.T) {
	assembler := &Assembler{}
	var chunks []StreamChunk
	for _, line := range []string{
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"name":"search_notes","arguments":"{\"q"}}]}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":1,"id":"call_b","function":{"name":"read_note","arguments":"{}"}}]}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"uery\":\"x\"}"}}]}}]}`,
		`data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`,
	} {
		got, _, err := assembler.Push(line)
		if err != nil {
			t.Fatal(err)
		}
		chunks = append(chunks, got...)
	}
	chunks = append(chunks, assembler.Finish()...)

	var starts []string
	var calls []CompletedToolCall
	var finish string
	for _, chunk := range chunks {
		switch chunk.Kind {
		case "tool-call-start":
			starts = append(starts, chunk.Call.ID+":"+chunk.Call.Name)
		case "tool-call":
			calls = append(calls, *chunk.Call)
		case "finish":
			finish = chunk.Reason
		}
	}
	if strings.Join(starts, "|") != "call_0:search_notes|call_b:read_note" {
		t.Fatalf("starts: %v", starts)
	}
	if len(calls) != 2 || calls[0].ID != "call_0" || calls[0].Name != "search_notes" || calls[0].Arguments != `{"query":"x"}` {
		t.Fatalf("calls: %+v", calls)
	}
	if calls[1].ID != "call_b" || finish != "tool_calls" {
		t.Fatalf("second call/finish: %+v %q", calls, finish)
	}
}

func TestAssemblerCompleteness(t *testing.T) {
	assembler := &Assembler{}
	if _, _, err := assembler.Push(`data: {"choices":[{"delta":{"content":"半截"}}]}`); err != nil {
		t.Fatal(err)
	}
	if assembler.Complete() {
		t.Fatal("stream without [DONE]/finish_reason must be incomplete")
	}

	done := &Assembler{}
	_, fin, _ := done.Push(`data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`)
	if !done.Complete() || fin {
		t.Fatal("finish_reason should complete the stream")
	}

	literal := &Assembler{}
	_, doneFlag, _ := literal.Push(`data: [DONE]`)
	if !literal.Complete() || !doneFlag {
		t.Fatal("[DONE] should complete the stream")
	}
}

func TestAssemblerErrors(t *testing.T) {
	assembler := &Assembler{}
	if _, _, err := assembler.Push(`data: {not json`); err == nil {
		t.Fatal("bad JSON accepted")
	}
	assembler = &Assembler{}
	if _, _, err := assembler.Push(`data: {"error":{"message":"boom"}}`); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("wire error: %v", err)
	}
	if _, _, err := assembler.Push(`data: [1,2]`); err == nil {
		t.Fatal("non-object accepted")
	}
}

func TestAssemblerIgnoresNonData(t *testing.T) {
	chunks, done := collect(t, `: keep-alive`, `event: ping`, ``)
	if len(chunks) != 0 || done {
		t.Fatalf("non-data lines: %+v %v", chunks, done)
	}
}
