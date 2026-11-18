package ai

import (
	"encoding/json"
	"math"
	"strings"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// Budget constants (contextBudget.ts:20-24).
const (
	CompactRatio         = 0.8
	TailRatio            = 0.16
	DefaultContextWindow = 64_000
	minPrefixMessages    = 2
	messageOverhead      = 4
	SummaryOpen          = "<compaction-summary>"
	SummaryClose         = "</compaction-summary>"
	TruncationNote       = "（更早的对话因上下文长度已省略）"
)

// CompactionInstruction is sent verbatim to the model (contextBudget.ts:34-56).
const CompactionInstruction = `把上面的对话整理成一份续接用的摘要，按下列标题写，没有内容的标题省略：

## 不变的事实与约束
用户说过、仍然有效的名字、路径、偏好和"不要做什么"，尽量用用户的原话。

## 目标
用户要做什么。

## 已做的决定
做过的选择和理由，免得反复。

## 涉及的笔记
读过或写过的路径，以及其中要紧的事实。

## 未完成与下一步
还在进行的事，以及最具体的下一步。

规则：要简洁，用要点。路径、名字、数字原样保留。合并已有 <compaction-summary> 里仍然有效的内容。不要编造。只输出摘要正文，不要调用工具。`

// EstimateTextTokens is characters/4 for ASCII plus 0.7 per non-ASCII rune.
func EstimateTextTokens(text string) int64 {
	var ascii, other int64
	for _, r := range text {
		if r < 0x80 {
			ascii++
		} else {
			other++
		}
	}
	return int64(math.Ceil(float64(ascii)/4 + float64(other)*0.7))
}

// EstimateMessageTokens adds the per-message overhead and the tool calls.
func EstimateMessageTokens(message ChatMessage) int64 {
	total := int64(messageOverhead)
	if message.Content != nil {
		total += EstimateTextTokens(*message.Content)
	}
	if message.ReasoningContent != nil {
		total += EstimateTextTokens(*message.ReasoningContent)
	}
	for _, call := range message.ToolCalls {
		total += EstimateTextTokens(call.Function.Name) + EstimateTextTokens(call.Function.Arguments)
	}
	return total
}

// EstimateTokens sums the messages and applies the calibration factor.
func EstimateTokens(messages []ChatMessage, calibration float64) int64 {
	if calibration <= 0 {
		calibration = 1
	}
	var total int64
	for _, message := range messages {
		total += EstimateMessageTokens(message)
	}
	return int64(math.Ceil(float64(total) * calibration))
}

// Calibrate derives the factor from the real prompt_tokens, clamped to 0.5–3.
func Calibrate(actualPromptTokens, estimated int64) float64 {
	if actualPromptTokens <= 0 || estimated <= 0 {
		return 1
	}
	factor := float64(actualPromptTokens) / float64(estimated)
	if factor < 0.5 {
		return 0.5
	}
	if factor > 3 {
		return 3
	}
	return factor
}

// IsSummaryMessage reports whether a message is a compaction summary.
func IsSummaryMessage(message ChatMessage) bool {
	return message.Role == "user" && message.Content != nil && strings.HasPrefix(*message.Content, SummaryOpen)
}

// RenderSummary wraps a summary as a user message.
func RenderSummary(text string) ChatMessage {
	content := SummaryOpen + "\n" + strings.TrimSpace(text) + "\n" + SummaryClose
	return ChatMessage{Role: "user", Content: &content}
}

// TurnStarts lists the indices where a turn starts: user messages that are not
// summaries. They are the only legal compaction cut points.
func TurnStarts(messages []ChatMessage) []int {
	var starts []int
	for index, message := range messages {
		if message.Role == "user" && !IsSummaryMessage(message) {
			starts = append(starts, index)
		}
	}
	return starts
}

// BudgetOptions mirrors the legacy shape.
type BudgetOptions struct {
	ContextWindow int
	Calibration   float64
	FixedTokens   int64
}

// CompactTrigger is 80% of the window.
func CompactTrigger(contextWindow int) int64 {
	if contextWindow <= 0 {
		contextWindow = DefaultContextWindow
	}
	trigger := int64(math.Floor(float64(contextWindow) * CompactRatio))
	if trigger < 1 {
		trigger = 1
	}
	return trigger
}

// NeedsMaintenance reports whether the estimated input exceeds 80%.
func NeedsMaintenance(messages []ChatMessage, options BudgetOptions) bool {
	used := EstimateTokens(messages, options.Calibration) + options.FixedTokens
	return used > CompactTrigger(options.ContextWindow)
}

// CompactionPlan splits the history into the part to summarize and the part to
// keep verbatim.
type CompactionPlan struct {
	Prefix []ChatMessage
	Tail   []ChatMessage
}

// PlanCompaction keeps as many whole recent turns as fit in 16% of the window;
// the current turn is always kept. A prefix too short to be worth summarizing
// returns nil.
func PlanCompaction(messages []ChatMessage, options BudgetOptions) *CompactionPlan {
	starts := TurnStarts(messages)
	if len(starts) == 0 {
		return nil
	}
	window := options.ContextWindow
	if window <= 0 {
		window = DefaultContextWindow
	}
	budget := int64(math.Floor(float64(window) * TailRatio))
	if budget < 1 {
		budget = 1
	}
	cut := starts[len(starts)-1]
	for index := len(starts) - 2; index >= 0; index-- {
		candidate := starts[index]
		if EstimateTokens(messages[candidate:], options.Calibration) > budget {
			break
		}
		cut = candidate
	}
	if cut < minPrefixMessages {
		return nil
	}
	return &CompactionPlan{Prefix: messages[:cut], Tail: messages[cut:]}
}

// TruncateToFit drops whole oldest turns until the history fits, always
// keeping the current turn; when anything was dropped it prepends a note.
func TruncateToFit(messages []ChatMessage, options BudgetOptions) []ChatMessage {
	limit := CompactTrigger(options.ContextWindow) - options.FixedTokens
	if EstimateTokens(messages, options.Calibration) <= limit {
		return append([]ChatMessage(nil), messages...)
	}
	starts := TurnStarts(messages)
	if len(starts) == 0 {
		return append([]ChatMessage(nil), messages...)
	}
	noteText := TruncationNote
	note := ChatMessage{Role: "user", Content: &noteText}
	for _, start := range starts[1:] {
		kept := append([]ChatMessage{note}, messages[start:]...)
		if EstimateTokens(kept, options.Calibration) <= limit {
			return kept
		}
	}
	return append([]ChatMessage{note}, messages[starts[len(starts)-1]:]...)
}

// AddUsage accumulates two usage groups; cost stays untouched (the provider
// is the only source of money values).
func AddUsage(a, b contract.Usage) contract.Usage {
	return contract.Usage{
		TotalTokens:      a.TotalTokens + b.TotalTokens,
		PromptTokens:     a.PromptTokens + b.PromptTokens,
		CompletionTokens: a.CompletionTokens + b.CompletionTokens,
		ReasoningTokens:  a.ReasoningTokens + b.ReasoningTokens,
		CacheHitTokens:   a.CacheHitTokens + b.CacheHitTokens,
		CacheMissTokens:  a.CacheMissTokens + b.CacheMissTokens,
	}
}

// fixedTokens counts the system prompt and the tool schemas, which are not in
// the messages but still consume input.
func fixedTokens(system string, tools []json.RawMessage) int64 {
	total := EstimateTextTokens(system)
	for _, schema := range tools {
		total += EstimateTextTokens(string(schema))
	}
	return total
}
