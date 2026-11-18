package ai

import (
	"strings"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

func TestEstimateTextTokens(t *testing.T) {
	cases := []struct {
		text string
		want int64
	}{
		{"", 0},
		{"abcd", 1},
		{"abcdefgh", 2},
		{"你好世界", 3},
		{"a你b好", 2},
	}
	for _, c := range cases {
		if got := EstimateTextTokens(c.text); got != c.want {
			t.Errorf("%q: %d want %d", c.text, got, c.want)
		}
	}
}

func TestCalibrate(t *testing.T) {
	if got := Calibrate(100, 100); got != 1 {
		t.Errorf("same: %v", got)
	}
	if got := Calibrate(200, 100); got != 2 {
		t.Errorf("double: %v", got)
	}
	if got := Calibrate(1000, 100); got != 3 {
		t.Errorf("clamp high: %v", got)
	}
	if got := Calibrate(1, 100); got != 0.5 {
		t.Errorf("clamp low: %v", got)
	}
	if got := Calibrate(0, 100); got != 1 {
		t.Errorf("no data: %v", got)
	}
}

func TestNeedsMaintenance(t *testing.T) {
	messages := []ChatMessage{{Role: "user", Content: ptr(strings.Repeat("a", 3200))}} // ~800 tokens
	options := BudgetOptions{ContextWindow: 1000}
	if !NeedsMaintenance(messages, options) {
		t.Fatal("800 > 80% of 1000 should need maintenance")
	}
	small := []ChatMessage{{Role: "user", Content: ptr(strings.Repeat("a", 2000))}} // ~500
	if NeedsMaintenance(small, options) {
		t.Fatal("500 < 80% should not need maintenance")
	}
	if !NeedsMaintenance(small, BudgetOptions{ContextWindow: 1000, FixedTokens: 400}) {
		t.Fatal("fixed tokens count toward the budget")
	}
}

func turn(user string, assistant string) []ChatMessage {
	return []ChatMessage{
		{Role: "user", Content: ptr(user)},
		{Role: "assistant", Content: ptr(assistant)},
	}
}

func TestPlanCompaction(t *testing.T) {
	messages := append(turn("一轮", strings.Repeat("a", 40)), turn("二轮", strings.Repeat("a", 400))...)
	messages = append(messages, turn("当前", "回答")...)
	plan := PlanCompaction(messages, BudgetOptions{ContextWindow: 600})
	if plan == nil {
		t.Fatal("expected a plan")
	}
	if plan.Tail[0].Role != "user" || *plan.Tail[0].Content != "当前" {
		t.Fatalf("cut must be at a user message: %+v", plan.Tail[0])
	}
	if len(plan.Prefix) != 4 {
		t.Fatalf("prefix: %d", len(plan.Prefix))
	}

	// A prefix with fewer than two messages is not worth summarizing.
	only := turn("当前", "回答")
	if PlanCompaction(only, BudgetOptions{ContextWindow: 600}) != nil {
		t.Fatal("single turn should not produce a plan")
	}
}

func TestTruncateToFit(t *testing.T) {
	messages := append(turn("旧", strings.Repeat("a", 400)), turn("新", "回答")...)
	got := TruncateToFit(messages, BudgetOptions{ContextWindow: 120})
	if len(got) == 0 || got[0].Content == nil || *got[0].Content != TruncationNote {
		t.Fatalf("expected a truncation note: %+v", got)
	}
	last := got[len(got)-1]
	if *last.Content != "回答" {
		t.Fatalf("current turn must stay: %+v", got)
	}

	short := turn("问", "答")
	if out := TruncateToFit(short, BudgetOptions{ContextWindow: 64_000}); len(out) != 2 {
		t.Fatalf("under limit should be untouched: %+v", out)
	}
}

func TestSummaryMessage(t *testing.T) {
	message := RenderSummary(" 摘要内容 ")
	if !IsSummaryMessage(message) || !strings.HasPrefix(*message.Content, SummaryOpen) {
		t.Fatalf("summary: %+v", message)
	}
	if IsSummaryMessage(ChatMessage{Role: "user", Content: ptr("普通")}) {
		t.Fatal("plain user message treated as summary")
	}
	if starts := TurnStarts([]ChatMessage{message, {Role: "user", Content: ptr("问")}}); len(starts) != 1 || starts[0] != 1 {
		t.Fatalf("summary must not start a turn: %v", starts)
	}
}

func TestAddUsage(t *testing.T) {
	got := AddUsage(contract.Usage{TotalTokens: 1, PromptTokens: 2}, contract.Usage{TotalTokens: 3, PromptTokens: 4})
	if got.TotalTokens != 4 || got.PromptTokens != 6 || got.EstimatedCost != nil {
		t.Fatalf("add: %+v", got)
	}
}
