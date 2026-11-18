package ai

import (
	"fmt"
	"strings"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

func baseContext() contract.SharedContext {
	return contract.SharedContext{
		SnapshotID: "snap-1",
		Sequence:   1,
		Vault:      contract.VaultContext{ID: "v1", RootLabel: "my-vault", Revision: 42, IndexRevision: 7},
	}
}

func retrievalItem(i, excerptLen int) contract.RetrievalItem {
	excerpt := fmt.Sprintf("片段内容%d", i)
	if runes := []rune(excerpt); len(runes) < excerptLen {
		excerpt += strings.Repeat("。", excerptLen-len(runes))
	}
	return contract.RetrievalItem{
		ID:       fmt.Sprintf("r-%d", i),
		Path:     fmt.Sprintf("skills/topic-%d/SKILL.md", i),
		Revision: int64(10 + i),
		Excerpt:  excerpt,
		Score:    1 - float64(i)*0.1,
		Reason:   fmt.Sprintf("与查询相关 %d", i),
		Area:     "skills",
	}
}

func taskItem(overrides contract.TaskItem) contract.TaskItem {
	task := contract.TaskItem{
		ID: "t-1", Owner: "curator", Status: contract.TaskQueued,
		Goal: "把草稿合并为正式技能", Inputs: []string{".mile-kb/drafts/candidate.md"},
		Constraints: []string{"保留来源", "标注不确定性"}, Outputs: []string{},
		CreatedBy: "distiller", CreatedAt: "2026-08-27T00:00:00.000Z", UpdatedAt: "2026-08-27T00:00:00.000Z",
	}
	if overrides.ID != "" {
		task.ID = overrides.ID
	}
	if overrides.Owner != "" {
		task.Owner = overrides.Owner
	}
	if overrides.Status != "" {
		task.Status = overrides.Status
	}
	if overrides.Goal != "" {
		task.Goal = overrides.Goal
	}
	if overrides.CreatedBy != "" {
		task.CreatedBy = overrides.CreatedBy
	}
	return task
}

func TestContextTailBasics(t *testing.T) {
	text := RenderContextTail(baseContext(), ContextTailOptions{})
	if text == "" || !strings.Contains(text, "42") {
		t.Fatalf("empty context tail: %q", text)
	}
	if !strings.Contains(text, "7") {
		t.Fatalf("index revision missing: %q", text)
	}
	if strings.Contains(text, `C:\`) || strings.Contains(text, "/home/") {
		t.Fatalf("absolute path leaked: %q", text)
	}
}

func TestContextTailActiveDocument(t *testing.T) {
	context := baseContext()
	context.ActiveDocument = &contract.ActiveDocumentContext{Path: "raws/note.md", Revision: 3, Dirty: true}
	text := RenderContextTail(context, ContextTailOptions{})
	if !strings.Contains(text, "raws/note.md") {
		t.Fatalf("document path missing: %q", text)
	}
	if !strings.Contains(text, "未保存") {
		t.Fatalf("dirty mark missing: %q", text)
	}
}

func TestContextTailBudgetNeverExceeded(t *testing.T) {
	context := baseContext()
	for i := 0; i < 50; i++ {
		context.RetrievalBundle = append(context.RetrievalBundle, retrievalItem(i, 2000))
		context.TaskBoard = append(context.TaskBoard, taskItem(contract.TaskItem{ID: fmt.Sprintf("t-%d", i), Goal: strings.Repeat("目标", 500)}))
		context.DecisionLog = append(context.DecisionLog, contract.Decision{
			ID: fmt.Sprintf("d-%d", i), Summary: strings.Repeat("决策", 500), DecidedBy: "user",
			DecidedAt: "2026-08-27T00:00:00.000Z",
		})
		context.PinnedReferences = append(context.PinnedReferences, contract.Reference{
			ID: fmt.Sprintf("p-%d", i), Path: fmt.Sprintf("raws/pin-%d.md", i), Revision: int64(i),
			Note: strings.Repeat("备注", 500), PinnedAt: "2026-08-27T00:00:00.000Z",
		})
	}
	text := RenderContextTail(context, ContextTailOptions{})
	if got := len([]rune(text)); got > ContextTailMaxChars {
		t.Fatalf("tail is %d runes", got)
	}
}

func TestContextTailRetrievalLimit(t *testing.T) {
	context := baseContext()
	for i := 0; i < 9; i++ {
		context.RetrievalBundle = append(context.RetrievalBundle, retrievalItem(i, 40))
	}
	text := RenderContextTail(context, ContextTailOptions{})
	if strings.Contains(text, "skills/topic-4/SKILL.md") {
		t.Fatalf("fifth retrieval item leaked: %q", text)
	}
	if !strings.Contains(text, "skills/topic-0/SKILL.md") {
		t.Fatalf("first retrieval item missing: %q", text)
	}
	if !strings.Contains(text, "未列出") && !strings.Contains(text, "另有") {
		t.Fatalf("omission marker missing: %q", text)
	}
}

func TestContextTailRetrievalWholeItems(t *testing.T) {
	context := baseContext()
	for i := 0; i < 4; i++ {
		context.RetrievalBundle = append(context.RetrievalBundle, retrievalItem(i, 900))
	}
	text := RenderContextTail(context, ContextTailOptions{})
	for i := 0; i < ContextMaxRetrievalItems; i++ {
		path := fmt.Sprintf("skills/topic-%d/SKILL.md", i)
		if strings.Contains(text, path) && !strings.Contains(text, fmt.Sprintf("%d", 10+i)) {
			t.Fatalf("partial item %d: %q", i, text)
		}
	}
	if got := len([]rune(text)); got > ContextTailMaxChars {
		t.Fatalf("tail is %d runes", got)
	}
}

func TestContextTailPinnedBudget(t *testing.T) {
	context := baseContext()
	for i := 0; i < 20; i++ {
		context.PinnedReferences = append(context.PinnedReferences, contract.Reference{
			ID: fmt.Sprintf("p-%d", i), Path: fmt.Sprintf("raws/pinned-%d.md", i), Revision: int64(i),
			Note: strings.Repeat("说明", 100), PinnedAt: "2026-08-27T00:00:00.000Z",
		})
	}
	text := RenderContextTail(context, ContextTailOptions{})
	if got := len([]rune(text)); got > ContextTailMaxChars {
		t.Fatalf("tail is %d runes", got)
	}
	if !strings.Contains(text, "未列出") && !strings.Contains(text, "另有") {
		t.Fatalf("omission marker missing: %q", text)
	}
	if !strings.Contains(text, "42") {
		t.Fatalf("vault revision was trimmed: %q", text)
	}
}

func TestContextTailTinyBudgetKeepsVaultAndDocument(t *testing.T) {
	context := baseContext()
	context.ActiveDocument = &contract.ActiveDocumentContext{Path: "raws/current.md", Revision: 5}
	for i := 0; i < 4; i++ {
		context.RetrievalBundle = append(context.RetrievalBundle, retrievalItem(i, 400))
	}
	text := RenderContextTail(context, ContextTailOptions{MaxChars: 260})
	if !strings.Contains(text, "42") || !strings.Contains(text, "raws/current.md") {
		t.Fatalf("priority broke: %q", text)
	}
	if got := len([]rune(text)); got > 260 {
		t.Fatalf("tail is %d runes", got)
	}
}

func TestContextTailTasks(t *testing.T) {
	context := baseContext()
	context.TaskBoard = []contract.TaskItem{
		taskItem(contract.TaskItem{}),
		taskItem(contract.TaskItem{ID: "t-done", Status: contract.TaskDone, Goal: "已完成的任务目标XYZ"}),
		taskItem(contract.TaskItem{ID: "t-cancelled", Status: contract.TaskCancelled, Goal: "已取消的任务目标ABC"}),
	}
	text := RenderContextTail(context, ContextTailOptions{})
	if !strings.Contains(text, "curator") || !strings.Contains(text, "把草稿合并为正式技能") {
		t.Fatalf("open task missing: %q", text)
	}
	if strings.Contains(text, "已完成的任务目标XYZ") || strings.Contains(text, "已取消的任务目标ABC") {
		t.Fatalf("closed task was injected: %q", text)
	}
	if !strings.Contains(text, "distiller") {
		t.Fatalf("createdBy missing: %q", text)
	}
}

func TestContextTailStaleAndRedaction(t *testing.T) {
	context := baseContext()
	stale := retrievalItem(0, 40)
	stale.Stale = true
	context.RetrievalBundle = append(context.RetrievalBundle, stale)
	context.DecisionLog = append(context.DecisionLog,
		contract.Decision{ID: "d-1", Summary: "用户提供了 sk-ABCDEFGH12345678 作为密钥", DecidedBy: "user", DecidedAt: "2026-08-27T00:00:00.000Z"},
		contract.Decision{ID: "d-2", Summary: `归档 C:\Users\realname\vault\raws\x.md`, DecidedBy: "user", DecidedAt: "2026-08-27T00:00:00.000Z"},
	)
	text := RenderContextTail(context, ContextTailOptions{
		Redact: RedactContext{Home: `C:\Users\realname`, VaultRoot: `C:\Users\realname\vault`},
	})
	if strings.Contains(text, "可能过期") == false {
		t.Fatalf("stale mark missing: %q", text)
	}
	if strings.Contains(text, "sk-ABCDEFGH12345678") {
		t.Fatalf("api key leaked: %q", text)
	}
	if strings.Contains(text, "realname") {
		t.Fatalf("home path leaked: %q", text)
	}
}

func TestContextTailConstants(t *testing.T) {
	if ContextTailMaxChars != 2400 || ContextMaxRetrievalItems != 4 || ContextMaxPinnedChars != 1500 {
		t.Fatalf("constants drifted: %d %d %d", ContextTailMaxChars, ContextMaxRetrievalItems, ContextMaxPinnedChars)
	}
}
