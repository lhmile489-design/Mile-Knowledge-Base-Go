package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/ai"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/policy"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/vault"
)

// ApprovalGate decides whether one write or structural change may proceed.
// T06 supplies the real gate (approval cards, 60-second grants); T05 tests
// inject a fake.
// A nil return allows the operation. A returned context error must reach the
// turn loop so cancellation stops the turn; other errors become tool text.
type ApprovalGate interface {
	AllowWrite(ctx context.Context, sessionID, path string) error
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
}

// NewAgentToolRunner builds a runner for one agent session.
func NewAgentToolRunner(app *App, vaultID, agent, sessionID string, gate ApprovalGate) *AgentToolRunner {
	return &AgentToolRunner{
		app: app, vaultID: vaultID, agent: agent, sessionID: sessionID, gate: gate,
		mutation: NewMutationService(app), index: NewIndexService(app),
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
		return "交接尚未就绪：该工具在共享上下文任务中提供。", nil
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
	if decision := policy.CanWrite(r.agent, path); !decision.Allowed {
		return "被拒绝：" + decision.Reason, nil
	}
	if err := r.gate.AllowWrite(ctx, r.sessionID, path); err != nil {
		return gateError(err)
	}
	root, errText := r.root()
	if errText != "" {
		return errText, nil
	}
	revision := int64(0)
	if snapshot, f := vault.Read(root, path); f == nil {
		revision = snapshot.Revision
	} else if f.Code != contract.CodeNotFound {
		return "读取失败：" + f.Message, nil
	}
	outcome, f := vault.Write(root, path, revision, args.Content)
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
	if decision := policy.CanWrite(r.agent, path); !decision.Allowed {
		return "被拒绝：" + decision.Reason, nil
	}
	root, errText := r.root()
	if errText != "" {
		return errText, nil
	}
	snapshot, f := vault.Read(root, path)
	if f != nil {
		return "读取失败：" + f.Message, nil
	}
	switch strings.Count(snapshot.Text, args.OldText) {
	case 0:
		return "old_text 在文件中不存在，未做修改", nil
	case 1:
	default:
		return "old_text 在文件中出现了多次，未做修改；请提供更精确的片段", nil
	}
	if err := r.gate.AllowWrite(ctx, r.sessionID, path); err != nil {
		return gateError(err)
	}
	next := strings.Replace(snapshot.Text, args.OldText, args.NewText, 1)
	outcome, f := vault.Write(root, path, snapshot.Revision, next)
	if f != nil {
		return "写入失败：" + f.Message, nil
	}
	if outcome.Status == "conflict" {
		return "写入冲突：文件在读取后又被修改", nil
	}
	return "已更新 " + path, nil
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
	kind := args.Kind
	switch kind {
	case policy.KindCreate, policy.KindCreateDirectory, policy.KindEdit,
		policy.KindMove, policy.KindCopy, policy.KindArchive, policy.KindTrash, policy.KindPermanentDelete:
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
