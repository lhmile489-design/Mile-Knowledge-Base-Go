package ai

import (
	"fmt"
	"strings"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// ContextTailOptions overrides the budget (tests and degraded scenarios only);
// the default is ContextTailMaxChars.
type ContextTailOptions struct {
	MaxChars int
	Redact   RedactContext
}

// tailBudget accumulates whole lines: a line either fits or it does not, so
// the model never reads half a record (contextTail.ts:36-65).
type tailBudget struct {
	lines []string
	used  int
	limit int
}

func (b *tailBudget) add(line string) bool {
	cost := len([]rune(line)) + 1 // newline
	if b.used+cost > b.limit {
		return false
	}
	b.lines = append(b.lines, line)
	b.used += cost
	return true
}

// force only serves content that is never trimmed (the vault revisions).
func (b *tailBudget) force(line string) {
	b.lines = append(b.lines, line)
	b.used += len([]rune(line)) + 1
}

func (b *tailBudget) remaining() int { return b.limit - b.used }
func (b *tailBudget) render() string { return strings.Join(b.lines, "\n") }

// RenderContextTail renders the shared-context tail block appended to the
// user message (contextTail.ts:67-186). Pure: the budget order is
//  1. vault / index revisions (never trimmed),
//  2. active document and dirty state,
//  3. pinned references (own 1500-character cap),
//  4. open tasks (the handoff carrier),
//  5. decisions,
//  6. retrieval snippets (at most 4, dropped whole when they do not fit).
//
// Everything omitted leaves an "N items not listed" marker: silently dropping
// entries would let the model believe it saw the whole picture.
func RenderContextTail(context contract.SharedContext, options ContextTailOptions) string {
	limit := options.MaxChars
	if limit <= 0 {
		limit = ContextTailMaxChars
	}
	scrub := func(text string) string { return Redact(text, options.Redact) }
	budget := &tailBudget{limit: limit}

	// 1. Vault / index revisions: the freshness baseline, never trimmed. Only
	// the display label goes in; absolute paths stay out of the context.
	budget.force("# 知识库状态（由应用维护，不可修改）")
	budget.force(fmt.Sprintf("知识库：%s｜内容版本 %d｜索引版本 %d",
		scrub(context.Vault.RootLabel), context.Vault.Revision, context.Vault.IndexRevision))

	// 2. Active document and unsaved state.
	if context.ActiveDocument != nil {
		doc := context.ActiveDocument
		dirtyMark := "已保存"
		if doc.Dirty {
			dirtyMark = "有未保存修改（磁盘内容与用户所见不同）"
		}
		budget.add(fmt.Sprintf("当前文件：%s（版本 %d，%s）", doc.Path, doc.Revision, dirtyMark))
		if doc.Selection != "" {
			budget.add("选中片段：" + scrub(truncateTail(doc.Selection, 200)))
		}
	}

	// 3. Pinned references: additionally capped at 1500 characters.
	if len(context.PinnedReferences) > 0 {
		pinBudget := ContextMaxPinnedChars
		if remaining := budget.remaining(); remaining < pinBudget {
			pinBudget = remaining
		}
		if pinBudget < 0 {
			pinBudget = 0
		}
		pinUsed, pinShown := 0, 0
		pinLines := make([]string, 0, len(context.PinnedReferences))
		for _, pin := range context.PinnedReferences {
			note := ""
			if pin.Note != "" {
				note = "：" + scrub(truncateTail(pin.Note, 120))
			}
			line := fmt.Sprintf("- %s（版本 %d）%s", pin.Path, pin.Revision, note)
			cost := len([]rune(line)) + 1
			if pinUsed+cost > pinBudget {
				break
			}
			pinLines = append(pinLines, line)
			pinUsed += cost
			pinShown++
		}
		if len(pinLines) > 0 && budget.add("## 用户固定引用") {
			for _, line := range pinLines {
				budget.add(line)
			}
		}
		if omitted := len(context.PinnedReferences) - pinShown; omitted > 0 {
			budget.add(fmt.Sprintf("（另有 %d 条固定引用未列出，已超出上下文预算）", omitted))
		}
	}

	// 4. Open tasks: the handoff carrier.
	var openTasks []contract.TaskItem
	for _, task := range context.TaskBoard {
		if IsOpenTask(task.Status) {
			openTasks = append(openTasks, task)
		}
	}
	if len(openTasks) > 0 {
		shown := 0
		if budget.add("## 任务板（未完成）") {
			for _, item := range openTasks {
				from := ""
				if item.CreatedBy != item.Owner {
					from = "，来自 " + item.CreatedBy
				}
				head := fmt.Sprintf("- [%s] %s%s：%s", item.Status, item.Owner, from, scrub(truncateTail(item.Goal, 160)))
				if !budget.add(head) {
					break
				}
				shown++
				if len(item.Inputs) > 0 {
					inputs := item.Inputs
					if len(inputs) > 3 {
						inputs = inputs[:3]
					}
					budget.add("  输入：" + strings.Join(inputs, "、"))
				}
				if len(item.Constraints) > 0 {
					budget.add("  约束：" + scrub(truncateTail(strings.Join(item.Constraints, "；"), 160)))
				}
			}
		}
		if omitted := len(openTasks) - shown; omitted > 0 {
			budget.add(fmt.Sprintf("（另有 %d 项未完成任务未列出）", omitted))
		}
	}

	// 5. Decisions, most recent first.
	if len(context.DecisionLog) > 0 {
		shown := 0
		if budget.add("## 已确认的决策") {
			for index := len(context.DecisionLog) - 1; index >= 0; index-- {
				decision := context.DecisionLog[index]
				line := fmt.Sprintf("- %s：%s", decision.DecidedBy, scrub(truncateTail(decision.Summary, 160)))
				if !budget.add(line) {
					break
				}
				shown++
			}
		}
		if omitted := len(context.DecisionLog) - shown; omitted > 0 {
			budget.add(fmt.Sprintf("（另有 %d 条决策未列出）", omitted))
		}
	}

	// 6. Retrieval snippets: at most four, either complete or absent.
	if len(context.RetrievalBundle) > 0 {
		candidates := context.RetrievalBundle
		if len(candidates) > ContextMaxRetrievalItems {
			candidates = candidates[:ContextMaxRetrievalItems]
		}
		shown := 0
		if budget.add("## 检索到的片段（来自知识库，非模型推断）") {
			for _, item := range candidates {
				staleMark := ""
				if item.Stale {
					staleMark = "，可能过期"
				}
				head := fmt.Sprintf("- %s（版本 %d｜相关度 %.2f%s）", item.Path, item.Revision, item.Score, staleMark)
				why := "  理由：" + scrub(truncateTail(item.Reason, 100))
				body := "  片段：" + scrub(truncateTail(item.Excerpt, 240))
				cost := len([]rune(head)) + len([]rune(why)) + len([]rune(body)) + 3
				if cost > budget.remaining() {
					break
				}
				budget.force(head)
				budget.force(why)
				budget.force(body)
				shown++
			}
		}
		if omitted := len(context.RetrievalBundle) - shown; omitted > 0 {
			budget.add(fmt.Sprintf("（另有 %d 条检索结果未列出）", omitted))
		}
	}

	rendered := budget.render()
	// Forced blocks can overshoot slightly (vault lines plus one snippet); the
	// final guard keeps whole lines only.
	if len([]rune(rendered)) <= limit {
		return rendered
	}
	lines := strings.Split(rendered, "\n")
	kept := make([]string, 0, len(lines))
	used := 0
	for _, line := range lines {
		cost := len([]rune(line)) + 1
		if used+cost > limit {
			break
		}
		kept = append(kept, line)
		used += cost
	}
	return strings.Join(kept, "\n")
}

// truncateTail collapses whitespace runs and trims to max runes with an
// ellipsis (contextTail.ts:188-191).
func truncateTail(text string, max int) string {
	normalized := strings.Join(strings.Fields(text), " ")
	runes := []rune(normalized)
	if len(runes) <= max {
		return normalized
	}
	return string(runes[:max-1]) + "…"
}
