package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/ai"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/pathguard"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/policy"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/vault"
)

// ApprovalGate decides whether one write or structural change may proceed.
// T06 supplies the real gate (approval cards, 60-second grants); T05 tests
// inject a fake.
// A nil return allows the operation. A returned context error must reach the
// turn loop so cancellation stops the turn; other errors become tool text.
type ApprovalGate interface {
	AllowWrite(ctx context.Context, sessionID, path string, preview *contract.WritePreview) error
	AllowMutation(ctx context.Context, sessionID, path string) error
	ConsumeFreshMutation(ctx context.Context, sessionID, path string) error
}

// AgentToolRunner executes the seven tools that are available in M2 (handoff
// arrives with the shared context in T08).
type AgentToolRunner struct {
	app       *App
	vaultID   string
	agent     string
	sessionID string
	gate      ApprovalGate
	mutation  *MutationService
	index     *IndexService
	context   *ai.ContextHub
}

// NewAgentToolRunner builds a runner for one agent session.
func NewAgentToolRunner(app *App, vaultID, agent, sessionID string, gate ApprovalGate) *AgentToolRunner {
	return &AgentToolRunner{
		app: app, vaultID: vaultID, agent: agent, sessionID: sessionID, gate: gate,
		mutation: NewMutationService(app), index: NewIndexService(app), context: app.Context,
	}
}

// Schemas returns the eight tool definitions.
func (r *AgentToolRunner) Schemas() []json.RawMessage { return ai.ToolSchemas }

// Run dispatches one call. Only cancellation is an error; everything else is
// text for the model.
func (r *AgentToolRunner) Run(ctx context.Context, call ai.CompletedToolCall) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	switch call.Name {
	case "search_notes":
		return r.searchNotes(ctx, call.Arguments), nil
	case "read_note":
		return r.readNote(call.Arguments), nil
	case "list_dir":
		return r.listDir(call.Arguments), nil
	case "write_note":
		return r.writeNote(ctx, call.Arguments)
	case "edit_note":
		return r.editNote(ctx, call.Arguments)
	case "mutate_notes":
		return r.mutateNotes(ctx, call.Arguments)
	case "set_aliases":
		return r.setAliases(ctx, call.Arguments), nil
	case "handoff_task":
		return r.handoff(call.Arguments), nil
	default:
		return fmt.Sprintf("未知工具：%s。可用的工具只有 %s。", call.Name, toolNames()), nil
	}
}

func toolNames() string {
	return "search_notes、read_note、list_dir、write_note、edit_note、mutate_notes、handoff_task、set_aliases"
}

func gateError(err error) (string, error) {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "", err
	}
	return "被拒绝：" + err.Error(), nil
}

func (r *AgentToolRunner) root() (string, string) {
	root, f := r.app.Vaults.Root(r.vaultID)
	if f != nil {
		return "", "知识库不可用"
	}
	return root, ""
}

func (r *AgentToolRunner) searchNotes(ctx context.Context, raw string) string {
	var args struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(rawOrDefault(raw, "{}")), &args); err != nil {
		return "参数不合法：无法解析为 JSON"
	}
	query := strings.TrimSpace(args.Query)
	if query == "" {
		return "参数不合法：query 不能为空"
	}
	limit := args.Limit
	if limit <= 0 {
		limit = 8
	}
	if limit > 20 {
		limit = 20
	}
	result := r.index.Search(ctx, contract.SearchQuery{VaultID: r.vaultID, Text: query, Limit: limit})
	if result.Failure != nil {
		return "检索失败：" + result.Failure.Message
	}
	if len(result.Hits) == 0 {
		return "没有命中。"
	}
	lines := make([]string, 0, len(result.Hits))
	for _, hit := range result.Hits {
		line := hit.Path + " — " + hit.Title
		if len(hit.MatchedFields) > 0 {
			line += "（命中：" + strings.Join(hit.MatchedFields, "、") + "）"
		}
		if hit.Excerpt != "" {
			line += "\n  " + hit.Excerpt
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func rawOrDefault(raw, fallback string) string {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	return raw
}

func (r *AgentToolRunner) readNote(raw string) string {
	var args struct {
		Path   string `json:"path"`
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(rawOrDefault(raw, "{}")), &args); err != nil {
		return "参数不合法：无法解析为 JSON"
	}
	root, errText := r.root()
	if errText != "" {
		return errText
	}
	snapshot, f := vault.Read(root, args.Path)
	if f != nil {
		return "读取失败：" + f.Message
	}
	runes := []rune(snapshot.Text)
	total := len(runes)
	offset := args.Offset
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		return fmt.Sprintf("offset 越界：共 %d 个字符", total)
	}
	limit := args.Limit
	if limit <= 0 || limit > 20000 {
		limit = 20000
	}
	end := offset + limit
	if end > total {
		end = total
	}
	header := fmt.Sprintf("第 %d-%d 个字符，共 %d 个", offset+1, end, total)
	if offset == 0 && end == 0 {
		header = "共 0 个字符"
	}
	if end < total {
		header += fmt.Sprintf("，还有更多：offset=%d", end)
	}
	return header + "\n" + string(runes[offset:end])
}

func (r *AgentToolRunner) listDir(raw string) string {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(rawOrDefault(raw, "{}")), &args); err != nil {
		return "参数不合法：无法解析为 JSON"
	}
	path := strings.TrimSpace(args.Path)
	if path != "" {
		normalized, ok := policy.Normalize(path)
		if !ok {
			return "参数不合法：路径必须是知识库内的相对路径"
		}
		path = normalized
	}
	root, errText := r.root()
	if errText != "" {
		return errText
	}
	tree, f := vault.ListDir(root, path, nil)
	if f != nil {
		return "读取失败：" + f.Message
	}
	lines := make([]string, 0, len(tree.Entries))
	for _, entry := range tree.Entries {
		if entry.Area == vault.AreaSystem {
			continue
		}
		kind := "文件"
		if entry.Kind == "directory" {
			kind = "目录"
		}
		lines = append(lines, "["+kind+"] "+entry.Name)
	}
	if len(lines) == 0 {
		return "（空目录）"
	}
	return strings.Join(lines, "\n")
}

// confirmPath runs the two policy gates of FR-AI-PERM: the spelling check
// first (no disk access, so an invalid path never touches the disk), then the
// canonical check on the resolved on-disk spelling, which expands Windows 8.3
// short names and case aliases (legacy VaultFsBridge.ts:161-172). The
// canonical path is what the caller must read and write.
func (r *AgentToolRunner) confirmPath(root, path string) (string, string) {
	if decision := policy.CanWrite(r.agent, path); !decision.Allowed {
		return "", "被拒绝：" + decision.Reason
	}
	canonical, f := pathguard.CanonicalRel(root, path)
	if f != nil {
		return "", "被拒绝：" + f.Message
	}
	if decision := policy.CanWrite(r.agent, canonical); !decision.Allowed {
		return "", "被拒绝：" + decision.Reason
	}
	return canonical, ""
}

func (r *AgentToolRunner) writeNote(ctx context.Context, raw string) (string, error) {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(rawOrDefault(raw, "{}")), &args); err != nil {
		return "参数不合法：无法解析为 JSON", nil
	}
	path, ok := policy.Normalize(args.Path)
	if !ok {
		return "参数不合法：path 必须是知识库内的相对路径", nil
	}
	root, errText := r.root()
	if errText != "" {
		return errText, nil
	}
	canonical, errText := r.confirmPath(root, path)
	if errText != "" {
		return errText, nil
	}
	revision := int64(0)
	var preview *contract.WritePreview
	if snapshot, f := vault.Read(root, canonical); f == nil {
		revision = snapshot.Revision
		old := snapshot.Text
		preview = ai.BuildWritePreview(args.Content, &old, int64(len(args.Content)))
	} else if f.Code == contract.CodeNotFound {
		preview = ai.BuildWritePreview(args.Content, nil, int64(len(args.Content)))
	}
	// A failed old-text read only drops the preview; the approval still shows
	// its one-line summary and the write reports the real error (EngineHost.ts:658).
	if err := r.gate.AllowWrite(ctx, r.sessionID, path, preview); err != nil {
		return gateError(err)
	}
	outcome, f := vault.Write(root, canonical, revision, args.Content)
	if f != nil {
		return "写入失败：" + f.Message, nil
	}
	if outcome.Status == "conflict" {
		return "写入冲突：磁盘上的内容在本次读取后又被修改", nil
	}
	return fmt.Sprintf("已写入 %s（%d 字节）", path, outcome.Size), nil
}

func (r *AgentToolRunner) editNote(ctx context.Context, raw string) (string, error) {
	var args struct {
		Path    string `json:"path"`
		OldText string `json:"old_text"`
		NewText string `json:"new_text"`
	}
	if err := json.Unmarshal([]byte(rawOrDefault(raw, "{}")), &args); err != nil {
		return "参数不合法：无法解析为 JSON", nil
	}
	path, ok := policy.Normalize(args.Path)
	if !ok {
		return "参数不合法：path 必须是知识库内的相对路径", nil
	}
	root, errText := r.root()
	if errText != "" {
		return errText, nil
	}
	canonical, errText := r.confirmPath(root, path)
	if errText != "" {
		return errText, nil
	}
	snapshot, f := vault.Read(root, canonical)
	if f != nil {
		return "读取失败：" + f.Message, nil
	}
	from, to := toLf(args.OldText), toLf(args.NewText)
	switch {
	case from == "":
		return "old_text 不能为空", nil
	case from == to:
		return "old_text 与 new_text 相同，没有可修改的内容", nil
	}
	count := countOccurrences(snapshot.Text, from)
	switch {
	case count == 0:
		return "old_text 在笔记中找不到，请先 read_note 核对原文（空白与换行需完全一致）", nil
	case count > 1:
		return fmt.Sprintf("old_text 在笔记中出现了 %d 处，位置不唯一；请带上更多上下文使其只匹配一处", count), nil
	}
	index := strings.Index(snapshot.Text, from)
	next := snapshot.Text[:index] + to + snapshot.Text[index+len(from):]
	old := snapshot.Text
	preview := ai.BuildWritePreview(next, &old, int64(len(next)))
	if err := r.gate.AllowWrite(ctx, r.sessionID, path, preview); err != nil {
		return gateError(err)
	}
	outcome, f := vault.Write(root, canonical, snapshot.Revision, next)
	if f != nil {
		return "写入失败：" + f.Message, nil
	}
	if outcome.Status == "conflict" {
		return "写入冲突：文件在读取后又被修改", nil
	}
	return "已更新 " + path, nil
}

// toLf normalises CRLF to LF: the model may paste an old_text copied with
// Windows line endings (noteText.ts:64).
func toLf(value string) string { return strings.ReplaceAll(value, "\r\n", "\n") }

// countOccurrences counts overlapping matches: "aa" occurs twice in "aaa",
// which is ambiguous for an exact edit (noteText.ts:52-62).
func countOccurrences(text, needle string) int {
	if needle == "" {
		return 0
	}
	count := 0
	for from := 0; from < len(text); {
		index := strings.Index(text[from:], needle)
		if index < 0 {
			return count
		}
		count++
		from += index + 1
	}
	return count
}

func (r *AgentToolRunner) mutateNotes(ctx context.Context, raw string) (string, error) {
	var args struct {
		Kind   string `json:"kind"`
		Source string `json:"source"`
		Target string `json:"target"`
	}
	if err := json.Unmarshal([]byte(rawOrDefault(raw, "{}")), &args); err != nil {
		return "参数不合法：无法解析为 JSON", nil
	}
	// The six kinds of the tool schema; a note is created with write_note and
	// edited with edit_note, so create/edit are not mutations here
	// (kbTools.ts:245-266).
	kind := args.Kind
	switch kind {
	case policy.KindCreateDirectory, policy.KindMove, policy.KindCopy,
		policy.KindArchive, policy.KindTrash, policy.KindPermanentDelete:
	default:
		return "参数不合法：kind 只接受 createDirectory / move / copy / archive / trash / permanentDelete", nil
	}
	source, target := args.Source, args.Target
	if source != "" {
		normalized, ok := policy.Normalize(source)
		if !ok {
			return "参数不合法：source 必须是知识库内的相对路径", nil
		}
		source = normalized
	}
	if target != "" {
		normalized, ok := policy.Normalize(target)
		if !ok {
			return "参数不合法：target 必须是知识库内的相对路径", nil
		}
		target = normalized
	}
	if kind != policy.KindCreateDirectory && source == "" {
		return "参数不合法：mutate_notes 的 kind 或路径不合法", nil
	}
	if (kind == policy.KindCreateDirectory || kind == policy.KindMove || kind == policy.KindCopy) && target == "" {
		return "参数不合法：mutate_notes 的 kind 或路径不合法", nil
	}
	root, errText := r.root()
	if errText != "" {
		return errText, nil
	}
	// The second gate runs on the canonical spelling of both sides: a short
	// name such as MILE-K~1 must not smuggle a path into the system area.
	if source != "" {
		canonical, f := pathguard.CanonicalRel(root, source)
		if f != nil {
			return "被拒绝：" + f.Message, nil
		}
		source = canonical
	}
	if target != "" {
		canonical, f := pathguard.CanonicalRel(root, target)
		if f != nil {
			return "被拒绝：" + f.Message, nil
		}
		target = canonical
	}
	if decision := policy.CanMutate(r.agent, kind, source, target); !decision.Allowed {
		return "被拒绝：" + decision.Reason, nil
	}
	guardPath := source
	if guardPath == "" {
		guardPath = target
	}
	if err := r.gate.AllowMutation(ctx, r.sessionID, guardPath); err != nil {
		return gateError(err)
	}
	fresh := false
	if kind == policy.KindPermanentDelete {
		if err := r.gate.ConsumeFreshMutation(ctx, r.sessionID, guardPath); err != nil {
			return gateError(err)
		}
		fresh = true
	}
	action := contract.MutationAction{ID: "a1", Kind: kind, Source: source, Target: target, Reason: "Agent 操作"}
	planned := r.mutation.Plan(ctx, contract.PlanRequest{
		VaultID: r.vaultID, Origin: r.agent, Actions: []contract.MutationAction{action}, FreshApproval: fresh,
	})
	if planned.Failure != nil {
		return "无法生成变更计划：" + planned.Failure.Message, nil
	}
	if planned.Plan == nil {
		return "无法生成变更计划", nil
	}
	report := r.mutation.Execute(ctx, *planned.Plan)
	if report.Failure != nil {
		return "执行失败：" + report.Failure.Message, nil
	}
	return fmt.Sprintf("完成 %d 项，失败 %d 项", len(report.Completed), len(report.Failed)), nil
}

// handoff creates one task on the shared board. The initiator is the runner's
// session agent, never the model's claim (kbTools.ts:533-553); the receiver
// cannot be the sender. PRD FR-AI-AI limits: goal ≤4000, inputs and
// constraints ≤50 entries of ≤4000 characters each.
func (r *AgentToolRunner) handoff(raw string) string {
	var args struct {
		Owner       string   `json:"owner"`
		Goal        string   `json:"goal"`
		Inputs      []string `json:"inputs"`
		Constraints []string `json:"constraints"`
	}
	if err := json.Unmarshal([]byte(rawOrDefault(raw, "{}")), &args); err != nil {
		return "参数不合法：无法解析为 JSON"
	}
	owner := strings.TrimSpace(args.Owner)
	goal := strings.TrimSpace(args.Goal)
	if owner == "" {
		return "owner 不能为空"
	}
	if goal == "" {
		return "goal 不能为空：交接任务必须写明目标"
	}
	if len([]rune(goal)) > 4000 {
		return "参数不合法：goal 不能超过 4000 字符"
	}
	if len(args.Inputs) > 50 {
		return "参数不合法：inputs 最多 50 条"
	}
	if len(args.Constraints) > 50 {
		return "参数不合法：constraints 最多 50 条"
	}
	for _, input := range args.Inputs {
		if len([]rune(input)) > 4000 {
			return "参数不合法：inputs 的每条不能超过 4000 字符"
		}
	}
	for _, constraint := range args.Constraints {
		if len([]rune(constraint)) > 4000 {
			return "参数不合法：constraints 的每条不能超过 4000 字符"
		}
	}
	if r.context == nil {
		return "交接失败：共享上下文不可用"
	}
	task, f := r.context.CreateHandoff(r.agent, ai.HandoffRequest{
		Owner: owner, Goal: goal, Inputs: args.Inputs, Constraints: args.Constraints,
	})
	if f != nil {
		return f.Message
	}
	return "已创建交接任务 " + task.ID + "，接收者 " + owner
}

func (r *AgentToolRunner) setAliases(ctx context.Context, raw string) string {
	var args struct {
		Aliases map[string]*string `json:"aliases"`
	}
	if err := json.Unmarshal([]byte(rawOrDefault(raw, "{}")), &args); err != nil {
		return "参数不合法：无法解析为 JSON"
	}
	if len(args.Aliases) == 0 {
		return "参数不合法：aliases 不能为空"
	}
	result := NewSettingsService(r.app).AppConfigPatchAliases(ctx, r.vaultID, args.Aliases)
	if result.Failure != nil {
		return "设置别名失败：" + result.Failure.Message
	}
	return fmt.Sprintf("已更新 %d 条别名", len(result.Aliases))
}
