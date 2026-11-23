package service

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/egoist/mygo"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/ai"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/policy"
)

// maxPromptChars rejects oversized prompts instead of truncating (API M39).
const maxPromptChars = 32_000

var agentDisplayNames = map[string]string{
	"curator": "整理归档", "scout": "推荐检索", "distiller": "蒸馏学习", "general": "自由问答",
}

var approvalRequestPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func engineCapabilities() *contract.EngineCapabilities { return &contract.EngineCapabilities{} }

// computeEngineStatus derives E10 from the vault's provider configuration and
// key availability; the in-process engine has no process to probe.
func (a *App) computeEngineStatus(vaultID string) contract.EngineStatus {
	offline := func(message string) contract.EngineStatus {
		return contract.EngineStatus{Phase: "offline", Message: message}
	}
	root, f := a.Vaults.Root(vaultID)
	if f != nil {
		return offline("知识库不可用")
	}
	providers, pf := ai.ReadProviders(root)
	if pf != nil {
		return offline("无法读取模型服务配置")
	}
	provider, _, resolveErr := ai.ResolveProvider(providers, "", "")
	if resolveErr != nil {
		return offline(resolveErr.Message)
	}
	if !ai.HasKey(a.Keys, provider) {
		name := provider.Name
		if name == "" {
			name = provider.APIKeyEnv
		}
		return offline("服务 " + name + " 还没有 API key：在设置的「模型服务」里填写")
	}
	return contract.EngineStatus{Phase: "ready", Capabilities: engineCapabilities()}
}

// emitEngineStatus broadcasts E10 only when the phase or message changed.
func (a *App) emitEngineStatus(vaultID string) contract.EngineStatus {
	status := a.computeEngineStatus(vaultID)
	key := status.Phase + "\x00" + status.Message
	a.mu.Lock()
	if a.engineStatus == nil {
		a.engineStatus = map[string]string{}
	}
	changed := a.engineStatus[vaultID] != key
	a.engineStatus[vaultID] = key
	a.mu.Unlock()
	if changed {
		if window := a.Window(); window != nil {
			_ = EngineStatusEvent.Emit(window, status)
		}
	}
	return status
}

// SetRecap stores the recap block prepared for the next prompt of one vault.
func (a *App) SetRecap(vaultID, recap string) {
	a.mu.Lock()
	if a.recaps == nil {
		a.recaps = map[string]string{}
	}
	if recap == "" {
		delete(a.recaps, vaultID)
	} else {
		a.recaps[vaultID] = recap
	}
	a.mu.Unlock()
}

// TakeRecap consumes the stored recap block (used exactly once).
func (a *App) TakeRecap(vaultID string) string {
	a.mu.Lock()
	recap := a.recaps[vaultID]
	delete(a.recaps, vaultID)
	a.mu.Unlock()
	return recap
}

// PutBackRecap restores a recap whose prompt never ran.
func (a *App) PutBackRecap(vaultID, recap string) { a.SetRecap(vaultID, recap) }

// ---- M38–M44 ----

// List is M38: the session for one (agent, vault), lazily created.
func (s *AgentService) List(agent, vaultID string) contract.AgentListResult {
	if !policy.IsAgent(agent) {
		return contract.AgentListResult{Failure: contract.Fail(contract.CodeInvalidInput, "未知 Agent")}
	}
	root, f := s.app.Vaults.Root(vaultID)
	if f != nil {
		return contract.AgentListResult{Failure: f}
	}
	providers, _ := ai.ReadProviders(root)
	configs, _ := ai.ReadAgents(root)
	config := configs[agent]
	session := s.app.Agents.Get(agent, vaultID)
	// The saved agent config wins; session.Model is only the last resolved
	// cache, so a model picked in the UI takes effect on the next List.
	if _, model, resolveErr := ai.ResolveProvider(providers, config.Model, session.Model); resolveErr == nil {
		session.Model = model
	}
	session.Effort = config.Effort
	session.Approval = config.Approval
	info := session.Info(ai.ModelChoices(providers))
	models := make([]contract.AgentModelOption, 0, len(info.AvailableModels))
	for _, model := range info.AvailableModels {
		models = append(models, contract.AgentModelOption{ModelID: model, Name: model})
	}
	engine := s.app.emitEngineStatus(vaultID)
	return contract.AgentListResult{
		Session: &contract.AgentSessionInfo{
			Agent: info.Agent, SessionID: info.SessionID, VaultID: info.VaultID,
			Model: info.Model, Effort: info.Effort, Approval: info.Approval,
			AvailableModels: models, AvailableEfforts: []string{}, Busy: info.Busy,
		},
		Engine: &engine,
	}
}

// frameSender is the channel surface the turn loop needs; tests substitute a
// recording sink for mygo.Channel.
type frameSender interface {
	Send(contract.StreamFrame) error
}

type channelSender struct {
	ch *mygo.Channel[contract.StreamFrame]
}

func (c channelSender) Send(frame contract.StreamFrame) error { return c.ch.Send(frame) }

// Prompt is M39: run one turn and stream its frames. The user message is
// recorded before the turn so a failed answer never erases the question.
func (s *AgentService) Prompt(ctx context.Context, agent, vaultID, text string, ch *mygo.Channel[contract.StreamFrame]) (contract.PromptResult, error) {
	return s.prompt(ctx, agent, vaultID, text, channelSender{ch})
}

func (s *AgentService) prompt(ctx context.Context, agent, vaultID, text string, sink frameSender) (contract.PromptResult, error) {
	if !policy.IsAgent(agent) {
		return contract.PromptResult{Failure: contract.Fail(contract.CodeInvalidInput, "未知 Agent")}, nil
	}
	if f := validateID(vaultID); f != nil {
		return contract.PromptResult{Failure: f}, nil
	}
	root, f := s.app.Vaults.Root(vaultID)
	if f != nil {
		return contract.PromptResult{Failure: f}, nil
	}
	if strings.TrimSpace(text) == "" {
		return contract.PromptResult{Failure: contract.Fail(contract.CodeInvalidInput, "消息不能为空")}, nil
	}
	if count := len([]rune(text)); count > maxPromptChars {
		return contract.PromptResult{Failure: contract.Fail(contract.CodeTooLarge, "消息超过 32000 字符").WithLimit(int64(count), maxPromptChars)}, nil
	}
	session := s.app.Agents.Get(agent, vaultID)
	if session.Busy {
		return contract.PromptResult{Failure: contract.Fail(contract.CodeInvalidInput, "上一轮还没结束")}, nil
	}

	configs, _ := ai.ReadAgents(root)
	config := configs[agent]
	providers, _ := ai.ReadProviders(root)
	session.Approval = config.Approval

	userText := text
	_, _, _ = s.app.Conversations.Append(vaultID, contract.ConversationEvent{
		Type: "user", At: nowISO(), Agent: agent, Text: &userText,
	})

	rt := &turnRuntime{
		app: s.app, service: s, channel: sink, session: session,
		agent: agent, vaultID: vaultID, sessionID: agent,
		recorder: &turnRecorder{app: s.app, vaultID: vaultID, agent: agent},
	}
	manager := ai.NewApprovalManager(
		func(string) string { return config.Approval },
		rt.onPermission,
	)
	rt.manager = manager
	runner := NewAgentToolRunner(s.app, vaultID, agent, agent, manager)
	recap := s.app.TakeRecap(vaultID)
	stream := s.app.AgentStream
	previousRetry := stream.OnRetry
	stream.OnRetry = func(info ai.RetryInfo) {
		if previousRetry != nil {
			previousRetry(info)
		}
		rt.flush()
		rt.send("status", func(frame *contract.StreamFrame) {
			frame.Status = &contract.StatusFrame{
				State:  "retry",
				Detail: fmt.Sprintf("第 %d/%d 次重试", info.Attempt, info.Max),
			}
		})
	}
	deps := ai.TurnDeps{
		Providers: providers, Configs: configs, Keys: s.app.Keys,
		Tools: runner, Stream: stream, OnEvent: rt.onEvent,
		TailBlocks: rt.tailBlocks(root, config.Steering),
	}
	stopReason, runErr := ai.RunTurn(ctx, session, text, recap, deps)
	rt.flushAll()
	manager.ExpireSessionCards(agent)
	rt.finish(stopReason, runErr, root)
	if (runErr != nil || stopReason == "error") && recap != "" {
		s.app.PutBackRecap(vaultID, recap)
	}
	if runErr != nil {
		return contract.PromptResult{}, runErr
	}
	return contract.PromptResult{StopReason: stopReason}, nil
}

// Cancel is M40.
func (s *AgentService) Cancel(agent string) error {
	if !policy.IsAgent(agent) {
		return errFromFailure(*contract.Fail(contract.CodeInvalidInput, "未知 Agent"))
	}
	s.app.Agents.CancelAgent(agent)
	return nil
}

// Reset is M43: cancel the vault's turns and drop its four sessions.
func (s *AgentService) Reset(ctx context.Context, vaultID string) error {
	if f := validateID(vaultID); f != nil {
		return errFromFailure(*f)
	}
	s.app.Agents.CancelVault(vaultID)
	s.app.Agents.Reset(vaultID)
	s.app.SetRecap(vaultID, "")
	return nil
}

// Approve is M41.
func (s *AgentService) Approve(requestID, optionID string) contract.Failure {
	if !approvalRequestPattern.MatchString(requestID) || len(requestID) > 256 {
		return *contract.Fail(contract.CodeInvalidInput, "requestId 不合法")
	}
	if strings.TrimSpace(optionID) == "" || len(optionID) > 256 {
		return *contract.Fail(contract.CodeInvalidInput, "optionId 不合法")
	}
	manager := s.managerFor(requestID)
	if manager == nil {
		return *contract.Fail(contract.CodeApprovalExpired, "审批已失效")
	}
	if f := manager.Approve(requestID, optionID); f != nil {
		return *f
	}
	return contract.Failure{}
}

// Reject is M42.
func (s *AgentService) Reject(requestID string) contract.Failure {
	if !approvalRequestPattern.MatchString(requestID) || len(requestID) > 256 {
		return *contract.Fail(contract.CodeInvalidInput, "requestId 不合法")
	}
	manager := s.managerFor(requestID)
	if manager == nil {
		return *contract.Fail(contract.CodeApprovalExpired, "审批已失效")
	}
	if f := manager.Reject(requestID); f != nil {
		return *f
	}
	return contract.Failure{}
}

// RoutePreview is M44.
func (s *AgentService) RoutePreview(text string) contract.RoutePreviewResult {
	decision := ai.RouteToAgent(text)
	matched := decision.Matched
	if matched == nil {
		matched = []string{}
	}
	return contract.RoutePreviewResult{
		Agent:        decision.Agent,
		Reason:       ai.ExplainRoute(decision, agentDisplayNames[decision.Agent]),
		MatchedTerms: matched,
	}
}

// ---- approval card registry ----

func (s *AgentService) rememberCard(requestID string, manager *ai.ApprovalManager) {
	s.mu.Lock()
	if s.managers == nil {
		s.managers = map[string]*ai.ApprovalManager{}
	}
	s.managers[requestID] = manager
	s.mu.Unlock()
}

func (s *AgentService) forgetCard(requestID string) {
	s.mu.Lock()
	delete(s.managers, requestID)
	s.mu.Unlock()
}

func (s *AgentService) managerFor(requestID string) *ai.ApprovalManager {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.managers[requestID]
}

// ---- turn runtime: frames and recording ----

// turnRuntime bridges the loop's events to the StreamFrame channel and the
// conversation recorder: text deltas are coalesced per kind (about 512 runes,
// a frame instead of a token), every non-delta event is a boundary, and the
// terminal frame carries Finish=true only after pending cards expired.
type turnRuntime struct {
	app     *App
	service *AgentService
	channel frameSender
	session *ai.Session

	agent     string
	vaultID   string
	sessionID string

	mu         sync.Mutex
	seq        int
	closed     bool
	bufferKind string
	buffer     strings.Builder
	buffered   int

	recorder *turnRecorder
	manager  *ai.ApprovalManager
	terminal *contract.StreamFrame
}

func (rt *turnRuntime) sanitize(message string) string {
	home, _ := os.UserHomeDir()
	root, _ := rt.app.Vaults.Root(rt.vaultID)
	return ai.Redact(message, ai.RedactContext{Home: home, VaultRoot: root})
}

// send sends one frame. A closed channel means the page stopped reading: the
// turn is cancelled and no further frames are attempted.
func (rt *turnRuntime) send(kind string, mutate func(*contract.StreamFrame)) bool {
	rt.mu.Lock()
	if rt.closed {
		rt.mu.Unlock()
		return false
	}
	rt.seq++
	frame := contract.StreamFrame{Kind: kind, Seq: rt.seq, Agent: rt.agent, SessionID: rt.sessionID}
	if mutate != nil {
		mutate(&frame)
	}
	rt.mu.Unlock()
	if err := rt.channel.Send(frame); err != nil {
		rt.mu.Lock()
		rt.closed = true
		rt.mu.Unlock()
		rt.session.Cancel()
		return false
	}
	return true
}

func (rt *turnRuntime) delta(kind, text string) {
	rt.mu.Lock()
	if rt.bufferKind != kind {
		pendingKind, pendingText := rt.takeBufferLocked()
		rt.mu.Unlock()
		rt.sendText(pendingKind, pendingText)
		rt.mu.Lock()
		rt.bufferKind = kind
	}
	rt.buffer.WriteString(text)
	rt.buffered += len([]rune(text))
	flush := rt.buffered >= 512
	if flush {
		kindNow, textNow := rt.takeBufferLocked()
		rt.mu.Unlock()
		rt.sendText(kindNow, textNow)
		return
	}
	rt.mu.Unlock()
}

func (rt *turnRuntime) takeBufferLocked() (string, string) {
	kind := rt.bufferKind
	text := rt.buffer.String()
	rt.buffer.Reset()
	rt.bufferKind = ""
	rt.buffered = 0
	return kind, text
}

func (rt *turnRuntime) sendText(kind, text string) {
	if text == "" {
		return
	}
	frameKind := "delta"
	if kind == "thought" {
		frameKind = "thought"
	}
	rt.send(frameKind, func(frame *contract.StreamFrame) { frame.Text = text })
}

// flush sends the buffered delta if any; it is the boundary before every
// non-delta frame. Callers outside onEvent use flushAll to include the
// recorder buffer.
func (rt *turnRuntime) flush() {
	rt.mu.Lock()
	kind, text := rt.takeBufferLocked()
	rt.mu.Unlock()
	rt.sendText(kind, text)
}

func (rt *turnRuntime) flushAll() {
	rt.flush()
	rt.recorder.flush()
}

func (rt *turnRuntime) onEvent(event ai.TurnEvent) {
	switch event.Kind {
	case ai.EventText:
		rt.recorder.chunk("message", event.Text)
		rt.delta("delta", event.Text)
	case ai.EventThought:
		rt.recorder.chunk("thought", event.Text)
		rt.delta("thought", event.Text)
	case ai.EventTool:
		rt.flushAll()
		if event.ToolStatus == "completed" || event.ToolStatus == "failed" {
			rt.recorder.tool(event)
		}
		rt.send("tool", func(frame *contract.StreamFrame) {
			frame.ToolCall = &contract.ToolCallFrame{
				ID: event.ToolID, Title: event.ToolTitle, Status: event.ToolStatus, Detail: event.Detail,
			}
		})
	case ai.EventStatus:
		rt.flushAll()
		rt.send("status", func(frame *contract.StreamFrame) {
			frame.Status = &contract.StatusFrame{State: event.Text}
		})
	case ai.EventUsage:
		rt.flushAll()
		if event.Usage != nil {
			usage := event.Usage
			rt.send("usage", func(frame *contract.StreamFrame) {
				frame.Usage = &contract.UsageFrame{Turn: usage.Turn, Cumulative: usage.Cumulative}
			})
		}
	case ai.EventTurnEnd:
		rt.flushAll()
		rt.recorder.turnEnd(event.StopReason)
		stop := event.StopReason
		rt.terminal = &contract.StreamFrame{Kind: "turn-end", StopReason: stop}
	case ai.EventError:
		rt.flushAll()
		message := rt.sanitize(event.Message)
		rt.recorder.failure(message)
		rt.terminal = &contract.StreamFrame{Kind: "error", Message: message}
	}
}

// onPermission turns the approval manager's events into permission frames and
// keeps requestID → manager visible to approve/reject while the card is open.
func (rt *turnRuntime) onPermission(event ai.PermEvent) {
	rt.flushAll()
	options := make([]contract.PermissionOption, 0, len(event.Options))
	for _, option := range event.Options {
		options = append(options, contract.PermissionOption{ID: option.ID, Label: option.Label})
	}
	delivered := rt.send("permission", func(frame *contract.StreamFrame) {
		frame.Permission = &contract.PermFrame{
			RequestID: event.RequestID, ToolName: event.ToolName, Detail: event.Detail,
			TargetPath: event.TargetPath, Preview: event.Preview, Approval: event.Approval,
			Options: options, Status: event.Status,
		}
	})
	if event.Status == "pending" {
		// A card the frontend never received must not stay approvable.
		if delivered {
			rt.service.rememberCard(event.RequestID, rt.manager)
		}
	} else {
		rt.service.forgetCard(event.RequestID)
	}
}

// finish sends the terminal frame last, after pending cards expired, so the
// card lifecycle never follows the end marker.
func (rt *turnRuntime) finish(stopReason string, runErr error, _ string) {
	rt.flushAll()
	terminal := rt.terminal
	if terminal == nil {
		if runErr != nil {
			terminal = &contract.StreamFrame{Kind: "error", Message: rt.sanitize(runErr.Error())}
		} else {
			terminal = &contract.StreamFrame{Kind: "turn-end", StopReason: stopReason}
		}
	}
	rt.send(terminal.Kind, func(frame *contract.StreamFrame) {
		*frame = *terminal
		frame.Kind = terminal.Kind
		frame.Finish = true
	})
}

// tailBlocks are the steering block and the shared context tail, injected
// after the recap and never into the system prompt.
func (rt *turnRuntime) tailBlocks(root, steering string) []string {
	snapshot := rt.app.Context.Snapshot()
	home, _ := os.UserHomeDir()
	contextTail := ai.RenderContextTail(snapshot, ai.ContextTailOptions{
		Redact: ai.RedactContext{Home: home, VaultRoot: root},
	})
	return []string{ai.RenderSteeringBlock(steering), contextTail}
}

// turnRecorder merges streamed chunks into message/thought lines and records
// the terminal states (tool final, turn-end, error) on boundaries.
type turnRecorder struct {
	app     *App
	vaultID string
	agent   string
	kind    string
	buffer  strings.Builder
}

func (r *turnRecorder) write(event contract.ConversationEvent) {
	_, _, _ = r.app.Conversations.Append(r.vaultID, event)
}

func (r *turnRecorder) chunk(kind, text string) {
	if r.kind != "" && r.kind != kind {
		r.flush()
	}
	r.kind = kind
	r.buffer.WriteString(text)
}

func (r *turnRecorder) flush() {
	if r.buffer.Len() == 0 {
		r.kind = ""
		return
	}
	text := r.buffer.String()
	kind := r.kind
	r.buffer.Reset()
	r.kind = ""
	event := contract.ConversationEvent{At: nowISO(), Agent: r.agent, Text: &text}
	if kind == "thought" {
		event.Type = "thought"
	} else {
		event.Type = "message"
	}
	r.write(event)
}

func (r *turnRecorder) tool(event ai.TurnEvent) {
	id, title, status := event.ToolID, event.ToolTitle, event.ToolStatus
	r.write(contract.ConversationEvent{
		Type: "tool", At: nowISO(), Agent: r.agent,
		ID: &id, Title: &title, Status: &status, Detail: event.Detail,
	})
}

func (r *turnRecorder) turnEnd(stopReason string) {
	r.flush()
	stop := stopReason
	r.write(contract.ConversationEvent{Type: "turn-end", At: nowISO(), Agent: r.agent, StopReason: &stop})
}

func (r *turnRecorder) failure(message string) {
	r.flush()
	r.write(contract.ConversationEvent{Type: "error", At: nowISO(), Agent: r.agent, Message: &message})
}
