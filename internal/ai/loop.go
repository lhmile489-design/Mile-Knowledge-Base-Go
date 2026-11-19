package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// Loop limits.
const (
	DefaultMaxSteps    = 16
	ToolResultMaxChars = 20_000
	CancelledResult    = "已取消，未执行"
)

// Turn event kinds (M39 frames are built from these).
const (
	EventText    = "text"
	EventThought = "thought"
	EventTool    = "tool"
	EventStatus  = "status"
	EventUsage   = "usage"
	EventTurnEnd = "turn-end"
	EventError   = "error"
)

// ToolRunner executes the knowledge-base tools. Run returns the text sent
// back to the model; only cancellation is an error, every other failure is
// turned into text by the runner itself.
type ToolRunner interface {
	Schemas() []json.RawMessage
	Run(ctx context.Context, call CompletedToolCall) (string, error)
}

// TurnUsageFrame is one usage event: this step's usage plus the session total.
type TurnUsageFrame struct {
	Turn       contract.Usage `json:"turn"`
	Cumulative contract.Usage `json:"cumulative"`
}

// TurnEvent is what the loop reports to the transport layer.
type TurnEvent struct {
	Kind       string
	Text       string
	ToolID     string
	ToolTitle  string
	ToolStatus string
	Detail     string
	Usage      *TurnUsageFrame
	StopReason string
	Message    string
}

// TurnDeps is everything the loop needs from its host.
type TurnDeps struct {
	Providers []Provider
	Configs   map[string]AgentConfig
	Keys      *KeyStore
	Tools     ToolRunner
	Stream    StreamOptions
	OnEvent   func(TurnEvent)
	MaxSteps  int
	// TailBlocks are appended to the user message (steering, recap, shared
	// context); they never go into the system prompt.
	TailBlocks []string
	// SystemPrompt stays empty per D-002; it still counts toward the budget.
	SystemPrompt string
}

func (d TurnDeps) emit(event TurnEvent) {
	if d.OnEvent != nil {
		d.OnEvent(event)
	}
}

func (d TurnDeps) maxSteps() int {
	if d.MaxSteps > 0 {
		return d.MaxSteps
	}
	return DefaultMaxSteps
}

// RunTurn runs one user turn: append the question, stream, execute tool calls,
// repeat until no calls remain or the step limit is hit.
func RunTurn(ctx context.Context, session *Session, userText, recap string, deps TurnDeps) (stopReason string, err error) {
	session.mu.Lock()
	if session.Busy {
		session.mu.Unlock()
		return "", &EngineError{Code: contract.CodeInvalidInput, Message: "上一轮还没结束"}
	}
	session.Busy = true
	child, cancel := context.WithCancel(ctx)
	session.cancel = cancel
	session.mu.Unlock()
	defer func() {
		cancel()
		session.mu.Lock()
		session.Busy = false
		session.cancel = nil
		session.mu.Unlock()
	}()

	defer func() {
		if recovered := recover(); recovered != nil {
			// Do not emit here: the panic may have come from the listener itself.
			err = &EngineError{Code: contract.CodeProviderError, Message: "回合内部错误"}
			stopReason = "error"
		}
	}()

	config := deps.Configs[session.Agent]
	provider, model, resolveErr := ResolveProvider(deps.Providers, session.Model, config.Model)
	if resolveErr != nil {
		deps.emit(TurnEvent{Kind: EventError, Message: resolveErr.Message})
		return "error", resolveErr
	}
	session.Model = model
	session.Effort = config.Effort
	session.Approval = config.Approval
	session.ContextWindow = DefaultContextWindow
	if provider.ContextWindow != nil && *provider.ContextWindow > 0 {
		session.ContextWindow = *provider.ContextWindow
	}
	apiKey := ""
	if provider.APIKeyEnv != "" && deps.Keys != nil {
		if stored, f := deps.Keys.Get(provider.APIKeyEnv); f == nil {
			apiKey = stored
		}
	}
	// A provider that needs a key must fail locally, before any request goes
	// out (the legacy engine throws here too, index.ts:467).
	if provider.APIKeyEnv != "" && apiKey == "" {
		name := provider.Name
		if name == "" {
			name = "模型服务"
		}
		message := "服务 " + name + " 还没有 API key：在设置的「模型服务」里填写"
		deps.emit(TurnEvent{Kind: EventError, Message: message})
		return "error", &EngineError{Code: contract.CodeProviderError, Message: message}
	}

	tail := make([]string, 0, len(deps.TailBlocks)+1)
	if strings.TrimSpace(recap) != "" {
		tail = append(tail, recap)
	}
	for _, block := range deps.TailBlocks {
		if strings.TrimSpace(block) != "" {
			tail = append(tail, block)
		}
	}
	userContent := userText
	if len(tail) > 0 {
		userContent = userText + "\n\n" + strings.Join(tail, "\n\n")
	}
	// The question enters history first: even a failed turn should leave the
	// model knowing what the user asked.
	session.pushMessage(ChatMessage{Role: "user", Content: &userContent})

	tools := deps.Tools.Schemas()
	fixed := fixedTokens(deps.SystemPrompt, tools)
	for step := 0; step < deps.maxSteps(); step++ {
		if child.Err() != nil {
			return finish(deps, session, "cancelled")
		}
		if f := deps.maintain(child, session, provider, apiKey, fixed); f != nil {
			if f.Code == contract.CodeProviderError && child.Err() != nil {
				return finish(deps, session, "cancelled")
			}
			deps.emit(TurnEvent{Kind: EventError, Message: f.Message})
			return "error", &EngineError{Code: f.Code, Message: f.Message}
		}

		var text, reasoning strings.Builder
		var calls []CompletedToolCall
		var usage *contract.Usage
		estimated := EstimateTokens(session.Messages(), session.Calibration()) + fixed

		streamErr := StreamChat(child, ChatRequest{
			BaseURL: provider.BaseURL, APIKey: apiKey, Model: model,
			Messages: session.Messages(), Tools: tools,
		}, deps.Stream, func(chunk StreamChunk) error {
			switch chunk.Kind {
			case ChunkText:
				text.WriteString(chunk.Text)
				deps.emit(TurnEvent{Kind: EventText, Text: chunk.Text})
			case ChunkReasoning:
				reasoning.WriteString(chunk.Text)
				deps.emit(TurnEvent{Kind: EventThought, Text: chunk.Text})
			case ChunkToolCallStart:
				deps.emit(TurnEvent{Kind: EventTool, ToolID: chunk.Call.ID, ToolTitle: chunk.Call.Name, ToolStatus: "pending"})
			case ChunkToolCall:
				calls = append(calls, *chunk.Call)
			case ChunkUsage:
				usage = chunk.Usage
			}
			return nil
		})
		if streamErr != nil {
			if child.Err() != nil {
				return finish(deps, session, "cancelled")
			}
			message := "供应商请求失败"
			var engineErr *EngineError
			if asEngineError(streamErr, &engineErr) {
				message = engineErr.Message
			}
			deps.emit(TurnEvent{Kind: EventError, Message: message})
			return "error", streamErr
		}

		if usage != nil {
			session.setCalibration(Calibrate(usage.PromptTokens, estimated))
			cumulative := session.addUsage(*usage)
			deps.emit(TurnEvent{Kind: EventUsage, Usage: &TurnUsageFrame{Turn: *usage, Cumulative: cumulative}})
		}

		// Commit boundary: only a cleanly finished sample enters the history.
		// A tool-carrying assistant message always carries reasoning_content,
		// even when empty (DeepSeek rejects the request without it).
		assistant := ChatMessage{Role: "assistant"}
		if text.Len() > 0 || len(calls) == 0 {
			content := text.String()
			assistant.Content = &content
		}
		if len(calls) > 0 {
			reasoningText := reasoning.String()
			assistant.ReasoningContent = &reasoningText
			assistant.ToolCalls = make([]ChatToolCallWire, 0, len(calls))
			for _, call := range calls {
				assistant.ToolCalls = append(assistant.ToolCalls, ChatToolCallWire{
					ID: call.ID, Type: "function",
					Function: ToolFunction{Name: call.Name, Arguments: call.Arguments},
				})
			}
		}
		session.pushMessage(assistant)

		if len(calls) == 0 {
			return finish(deps, session, "end_turn")
		}

		for index, call := range calls {
			if child.Err() != nil || ctx.Err() != nil {
				for _, rest := range calls[index:] {
					session.pushMessage(toolResult(rest.ID, CancelledResult))
					deps.emit(TurnEvent{Kind: EventTool, ToolID: rest.ID, ToolTitle: rest.Name, ToolStatus: "failed", Detail: CancelledResult})
				}
				return finish(deps, session, "cancelled")
			}
			output, runErr := runTool(child, deps.Tools, call)
			if runErr != nil {
				if child.Err() != nil {
					for _, rest := range calls[index:] {
						session.pushMessage(toolResult(rest.ID, CancelledResult))
						deps.emit(TurnEvent{Kind: EventTool, ToolID: rest.ID, ToolTitle: rest.Name, ToolStatus: "failed", Detail: CancelledResult})
					}
					return finish(deps, session, "cancelled")
				}
				output = runErr.Error()
			}
			if runes := []rune(output); len(runes) > ToolResultMaxChars {
				output = string(runes[:ToolResultMaxChars]) + fmt.Sprintf("…（内容过长，已截断，共 %d 字符）", len(runes))
			}
			session.pushMessage(toolResult(call.ID, output))
			deps.emit(TurnEvent{Kind: EventTool, ToolID: call.ID, ToolTitle: call.Name, ToolStatus: "completed", Detail: summarizeToolOutput(output)})
		}
	}
	return finish(deps, session, "max_turn_requests")
}

func finish(deps TurnDeps, _ *Session, reason string) (string, error) {
	deps.emit(TurnEvent{Kind: EventTurnEnd, StopReason: reason})
	return reason, nil
}

func runTool(ctx context.Context, runner ToolRunner, call CompletedToolCall) (output string, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = &EngineError{Code: contract.CodeProviderError, Message: fmt.Sprintf("工具 %s 执行失败", call.Name)}
		}
	}()
	return runner.Run(ctx, call)
}

func toolResult(callID, content string) ChatMessage {
	return ChatMessage{Role: "tool", Content: &content, ToolCallID: callID}
}

func summarizeToolOutput(output string) string {
	runes := []rune(output)
	if len(runes) > 120 {
		return string(runes[:120]) + "…"
	}
	return output
}

func asEngineError(err error, target **EngineError) bool {
	for err != nil {
		if engineErr, ok := err.(*EngineError); ok {
			*target = engineErr
			return true
		}
		unwrapper, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = unwrapper.Unwrap()
	}
	return false
}

// maintain compacts or truncates the history when it exceeds 80% of the
// window. A summary failure falls back to dropping whole oldest turns.
func (d TurnDeps) maintain(ctx context.Context, session *Session, provider Provider, apiKey string, fixed int64) *contract.Failure {
	options := BudgetOptions{ContextWindow: session.ContextWindow, Calibration: session.Calibration(), FixedTokens: fixed}
	messages := session.Messages()
	if !NeedsMaintenance(messages, options) {
		return nil
	}
	d.emit(TurnEvent{Kind: EventStatus, Text: "compacting"})
	if plan := PlanCompaction(messages, options); plan != nil {
		summary, err := d.summarize(ctx, provider, apiKey, plan.Prefix)
		if err != nil {
			if ctx.Err() != nil {
				return contract.Fail(contract.CodeProviderError, "请求已取消")
			}
			// Summary failed: fall through to truncation, never abort the turn.
		} else if strings.TrimSpace(summary) != "" {
			next := append([]ChatMessage{RenderSummary(summary)}, plan.Tail...)
			session.setMessages(next)
			messages = next
		}
	}
	if NeedsMaintenance(messages, options) {
		session.setMessages(TruncateToFit(messages, options))
	}
	return nil
}

func (d TurnDeps) summarize(ctx context.Context, provider Provider, apiKey string, prefix []ChatMessage) (string, error) {
	instruction := CompactionInstruction
	messages := append(append([]ChatMessage(nil), prefix...), ChatMessage{Role: "user", Content: &instruction})
	var text strings.Builder
	err := StreamChat(ctx, ChatRequest{
		BaseURL: provider.BaseURL, APIKey: apiKey, Model: "", Messages: messages,
	}, d.Stream, func(chunk StreamChunk) error {
		if chunk.Kind == ChunkText {
			text.WriteString(chunk.Text)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return text.String(), nil
}

func (s *Session) pushMessage(message ChatMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, message)
}

func (s *Session) setMessages(messages []ChatMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append([]ChatMessage(nil), messages...)
}

func (s *Session) setCalibration(factor float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calibration = factor
}

func (s *Session) addUsage(usage contract.Usage) contract.Usage {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cumulative = AddUsage(s.cumulative, usage)
	return s.cumulative
}
