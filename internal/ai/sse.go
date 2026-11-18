package ai

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// Chunk kinds (StreamFrame kinds are built from these in M2 T04/T10).
const (
	ChunkText          = "text"
	ChunkReasoning     = "reasoning"
	ChunkToolCallStart = "tool-call-start"
	ChunkToolCall      = "tool-call"
	ChunkUsage         = "usage"
	ChunkFinish        = "finish"
)

// CompletedToolCall is one fully assembled function call.
type CompletedToolCall struct {
	ID        string
	Name      string
	Arguments string
}

// StreamChunk is one piece the client emits.
type StreamChunk struct {
	Kind   string
	Text   string
	Call   *CompletedToolCall
	Usage  *contract.Usage
	Reason string
}

// StreamDecodeError means the provider sent something that is not valid SSE
// JSON (streamAssembler.ts:37-43).
type StreamDecodeError struct{ Detail string }

func (e *StreamDecodeError) Error() string {
	return "供应商返回了无法解析的流式数据：" + e.Detail
}

// ProviderStreamError means the provider reported an error after streaming
// had already started.
type ProviderStreamError struct{ Message string }

func (e *ProviderStreamError) Error() string { return "供应商在输出途中报错：" + e.Message }

type wireUsage struct {
	PromptTokens        float64 `json:"prompt_tokens"`
	CompletionTokens    float64 `json:"completion_tokens"`
	TotalTokens         float64 `json:"total_tokens"`
	PromptCacheHit      float64 `json:"prompt_cache_hit_tokens"`
	PromptCacheMiss     float64 `json:"prompt_cache_miss_tokens"`
	PromptTokensDetails *struct {
		CachedTokens float64 `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
	CompletionTokensDetails *struct {
		ReasoningTokens float64 `json:"reasoning_tokens"`
	} `json:"completion_tokens_details"`
}

func nonNegative(value float64) int64 {
	if value > 0 {
		return int64(value)
	}
	return 0
}

// normalizeUsage follows Reasonix: DeepSeek reports hit/miss directly, OpenAI
// only reports cached_tokens and the miss count is derived (streamAssembler.ts:63-79).
func normalizeUsage(raw wireUsage) *contract.Usage {
	prompt := nonNegative(raw.PromptTokens)
	completion := nonNegative(raw.CompletionTokens)
	total := nonNegative(raw.TotalTokens)
	if total == 0 {
		total = prompt + completion
	}
	hit := nonNegative(raw.PromptCacheHit)
	if hit == 0 && raw.PromptTokensDetails != nil {
		hit = nonNegative(raw.PromptTokensDetails.CachedTokens)
	}
	miss := nonNegative(raw.PromptCacheMiss)
	if miss == 0 && hit > 0 && prompt > hit {
		miss = prompt - hit
	}
	var reasoning int64
	if raw.CompletionTokensDetails != nil {
		reasoning = nonNegative(raw.CompletionTokensDetails.ReasoningTokens)
	}
	return &contract.Usage{
		TotalTokens: total, PromptTokens: prompt, CompletionTokens: completion,
		ReasoningTokens: reasoning, CacheHitTokens: hit, CacheMissTokens: miss,
	}
}

type partialCall struct {
	id        string
	name      string
	arguments strings.Builder
	started   bool
}

// Assembler turns `data:` lines into chunks (streamAssembler.ts:100-205).
type Assembler struct {
	think        ThinkSplitter
	calls        map[int]*partialCall
	finishReason string
	sawDone      bool
}

type wireToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type wireToolDelta struct {
	Index    *int             `json:"index"`
	ID       string           `json:"id"`
	Function wireToolFunction `json:"function"`
}

type wirePayload struct {
	Choices []struct {
		Delta struct {
			Content          *string         `json:"content"`
			ReasoningContent string          `json:"reasoning_content"`
			Reasoning        string          `json:"reasoning"`
			ToolCalls        []wireToolDelta `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *wireUsage `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Push consumes one trimmed line. Non-data lines are ignored.
func (a *Assembler) Push(line string) ([]StreamChunk, bool, error) {
	if !strings.HasPrefix(line, "data:") {
		return nil, false, nil
	}
	data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	if data == "" {
		return nil, false, nil
	}
	if data == "[DONE]" {
		a.sawDone = true
		return nil, true, nil
	}

	var payload wirePayload
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		return nil, false, &StreamDecodeError{Detail: err.Error()}
	}
	if payload.Error != nil {
		message := payload.Error.Message
		if message == "" {
			message = "未知错误"
		}
		return nil, false, &ProviderStreamError{Message: message}
	}

	var chunks []StreamChunk
	if payload.Usage != nil {
		chunks = append(chunks, StreamChunk{Kind: ChunkUsage, Usage: normalizeUsage(*payload.Usage)})
	}
	if len(payload.Choices) == 0 {
		return chunks, false, nil
	}
	choice := payload.Choices[0]
	if choice.FinishReason != "" {
		a.finishReason = choice.FinishReason
	}

	if reasoning := choice.Delta.ReasoningContent; reasoning != "" {
		chunks = append(chunks, StreamChunk{Kind: ChunkReasoning, Text: reasoning})
	} else if reasoning := choice.Delta.Reasoning; reasoning != "" {
		chunks = append(chunks, StreamChunk{Kind: ChunkReasoning, Text: reasoning})
	}

	if choice.Delta.Content != nil && *choice.Delta.Content != "" {
		piece := a.think.Push(*choice.Delta.Content)
		if piece.Reasoning != "" {
			chunks = append(chunks, StreamChunk{Kind: ChunkReasoning, Text: piece.Reasoning})
		}
		if piece.Text != "" {
			chunks = append(chunks, StreamChunk{Kind: ChunkText, Text: piece.Text})
		}
	}

	for _, raw := range choice.Delta.ToolCalls {
		index := 0
		if raw.Index != nil {
			index = *raw.Index
		}
		if a.calls == nil {
			a.calls = map[int]*partialCall{}
		}
		call := a.calls[index]
		if call == nil {
			call = &partialCall{}
			a.calls[index] = call
		}
		if raw.ID != "" {
			call.id = raw.ID
		}
		if raw.Function.Name != "" {
			call.name = raw.Function.Name
		}
		call.arguments.WriteString(raw.Function.Arguments)
		if !call.started && call.name != "" {
			call.started = true
			chunks = append(chunks, StreamChunk{
				Kind: ChunkToolCallStart,
				Call: &CompletedToolCall{ID: callID(call, index), Name: call.name},
			})
		}
	}
	return chunks, false, nil
}

func callID(call *partialCall, index int) string {
	if call.id != "" {
		return call.id
	}
	return fmt.Sprintf("call_%d", index)
}

// Complete reports whether the stream ended the legal way: [DONE] or a
// non-empty finish_reason.
func (a *Assembler) Complete() bool { return a.sawDone || a.finishReason != "" }

// Finish flushes held text, emits the complete tool calls in index order and
// one finish chunk.
func (a *Assembler) Finish() []StreamChunk {
	var chunks []StreamChunk
	piece := a.think.Flush()
	if piece.Reasoning != "" {
		chunks = append(chunks, StreamChunk{Kind: ChunkReasoning, Text: piece.Reasoning})
	}
	if piece.Text != "" {
		chunks = append(chunks, StreamChunk{Kind: ChunkText, Text: piece.Text})
	}
	indexes := make([]int, 0, len(a.calls))
	for index := range a.calls {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	for _, index := range indexes {
		call := a.calls[index]
		if call.name == "" {
			continue
		}
		chunks = append(chunks, StreamChunk{Kind: ChunkToolCall, Call: &CompletedToolCall{
			ID: callID(call, index), Name: call.name, Arguments: call.arguments.String(),
		}})
	}
	chunks = append(chunks, StreamChunk{Kind: ChunkFinish, Reason: a.finishReason})
	return chunks
}
