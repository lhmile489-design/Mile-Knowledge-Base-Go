package conversation

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

func userEvent(text string) contract.ConversationEvent {
	return contract.ConversationEvent{Type: "user", At: "2026-11-19T12:00:00.000Z", Agent: "curator", Text: textPtr(text)}
}

func messageEvent(text string) contract.ConversationEvent {
	return contract.ConversationEvent{Type: "message", At: "2026-11-19T12:00:00.000Z", Agent: "curator", Text: textPtr(text)}
}

func TestParseEventRoundTrip(t *testing.T) {
	status := "completed"
	id := "call_1"
	title := "search_notes"
	events := []contract.ConversationEvent{
		userEvent("问"),
		messageEvent("答"),
		{Type: "thought", At: "2026-11-19T12:00:00.000Z", Agent: "scout", Text: textPtr("想")},
		{Type: "tool", At: "2026-11-19T12:00:00.000Z", Agent: "scout", ID: &id, Title: &title, Status: &status, Kind: "read", Detail: "详情"},
		{Type: "turn-end", At: "2026-11-19T12:00:00.000Z", Agent: "scout", StopReason: textPtr("end_turn")},
		{Type: "error", At: "2026-11-19T12:00:00.000Z", Agent: "scout", Message: textPtr("出错")},
	}
	for _, event := range events {
		line := SerializeEvent(event)
		if !strings.HasSuffix(line, "\n") || strings.Count(line, "\n") != 1 {
			t.Fatalf("line: %q", line)
		}
		parsed, ok := ParseEventLine(strings.TrimSuffix(line, "\n"))
		if !ok {
			t.Fatalf("parse %s: %q", event.Type, line)
		}
		original, _ := json.Marshal(event)
		again, _ := json.Marshal(parsed)
		if string(original) != string(again) {
			t.Fatalf("roundtrip %s: %s -> %s", event.Type, original, again)
		}
	}
}

func TestParseEventRejectsBadLines(t *testing.T) {
	bad := []string{
		"",
		"not json",
		`[]`,
		`{"t":"user","at":"nope","agent":"curator","text":"x"}`,
		`{"t":"user","at":"2026-11-19T12:00:00Z","agent":"nobody","text":"x"}`,
		`{"t":"user","at":"2026-11-19T12:00:00Z","agent":"curator"}`,
		`{"t":"bogus","at":"2026-11-19T12:00:00Z","agent":"curator"}`,
		`{"t":"tool","at":"2026-11-19T12:00:00Z","agent":"curator","id":"a","title":"b","status":"weird"}`,
		`{"t":"turn-end","at":"2026-11-19T12:00:00Z","agent":"curator","stopReason":"weird"}`,
	}
	for _, line := range bad {
		if _, ok := ParseEventLine(line); ok {
			t.Errorf("accepted %q", line)
		}
	}
	var parsed contract.ConversationEvent
	if err := json.Unmarshal([]byte(SerializeEvent(userEvent("a\nb"))), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Text == nil || *parsed.Text != "a\nb" {
		t.Fatalf("newline escaping: %+v", parsed)
	}
}

func TestParseEventClampsFields(t *testing.T) {
	event := userEvent(strings.Repeat("字", MaxTextChars+10))
	line := SerializeEvent(event)
	parsed, ok := ParseEventLine(strings.TrimSuffix(line, "\n"))
	if !ok {
		t.Fatal("parse")
	}
	runes := []rune(*parsed.Text)
	if len(runes) != MaxTextChars+len([]rune(truncationMark)) || !strings.HasSuffix(*parsed.Text, "…（已截断）") {
		t.Fatalf("clamp: %d", len(runes))
	}

	status := "pending"
	id := strings.Repeat("i", 201)
	title := strings.Repeat("t", 301)
	kind := strings.Repeat("k", 65)
	detail := strings.Repeat("d", 2_001)
	tool := contract.ConversationEvent{Type: "tool", At: "2026-11-19T12:00:00Z", Agent: "curator",
		ID: &id, Title: &title, Status: &status, Kind: kind, Detail: detail}
	parsedTool, ok := ParseEventLine(strings.TrimSuffix(SerializeEvent(tool), "\n"))
	if !ok {
		t.Fatal("tool parse")
	}
	if len(*parsedTool.ID) != 200 || len(*parsedTool.Title) != 300 || len(parsedTool.Kind) != 64 || len(parsedTool.Detail) != 2_000 {
		t.Fatalf("tool clamp: %d %d %d %d", len(*parsedTool.ID), len(*parsedTool.Title), len(parsedTool.Kind), len(parsedTool.Detail))
	}
}

func TestParseConversationTextCountsSkipped(t *testing.T) {
	text := SerializeEvent(userEvent("一")) + "坏行\n\n" + SerializeEvent(messageEvent("二"))
	events, skipped := ParseConversationText(text)
	if len(events) != 2 || skipped != 1 {
		t.Fatalf("events=%d skipped=%d", len(events), skipped)
	}
}

func TestSanitizeTitleAndDerive(t *testing.T) {
	if title, ok := SanitizeTitle("  多行\n标题\t值  "); !ok || title != "多行 标题 值" {
		t.Fatalf("sanitize: %q %v", title, ok)
	}
	if _, ok := SanitizeTitle(" \n\t "); ok {
		t.Fatal("blank title accepted")
	}
	long := strings.Repeat("长", 90)
	if title, _ := SanitizeTitle(long); len([]rune(title)) != MaxTitleChars {
		t.Fatalf("title cap: %d", len([]rune(title)))
	}
	if got := DeriveTitle("\n\n第一行内容\n第二行"); got != "第一行内容" {
		t.Fatalf("derive: %q", got)
	}
	if got := DeriveTitle(strings.Repeat("字", 41)); len([]rune(got)) != 41 || !strings.HasSuffix(got, "…") {
		t.Fatalf("derive long: %q", got)
	}
	if got := DeriveTitle("  "); got != DefaultTitle {
		t.Fatalf("derive blank: %q", got)
	}
}

func TestCoerceIndex(t *testing.T) {
	meta := func(id string, updated string) map[string]any {
		return map[string]any{"id": id, "title": "标题", "createdAt": "2026-11-19T12:00:00Z",
			"updatedAt": updated, "turnCount": 2, "agents": []string{"curator", "scout", "curator"}, "pinned": true}
	}
	var list []any
	list = append(list, meta("c1", "2026-11-19T12:00:00Z"))
	list = append(list, map[string]any{"id": "bad id"})
	list = append(list, meta("c1", "2026-11-19T13:00:00Z")) // duplicate keeps the first
	list = append(list, meta("c2", "2026-11-19T13:00:00Z"))
	raw, _ := json.Marshal(map[string]any{"activeId": "missing", "conversations": list})
	index := CoerceIndex(raw)
	if len(index.Conversations) != 2 || index.Conversations[0].TurnCount != 2 || len(index.Conversations[0].Agents) != 2 {
		t.Fatalf("coerce: %+v", index)
	}
	if index.ActiveID != nil {
		t.Fatalf("missing active id kept: %v", *index.ActiveID)
	}

	for i := 0; i < MaxConversations+5; i++ {
		list = append(list, meta(fmt.Sprintf("x%d", i), "2026-11-19T12:00:00Z"))
	}
	raw, _ = json.Marshal(map[string]any{"conversations": list})
	if got := CoerceIndex(raw); len(got.Conversations) > MaxConversations {
		t.Fatalf("cap: %d", len(got.Conversations))
	}

	if got := CoerceIndex([]byte("not json")); got.Conversations == nil || len(got.Conversations) != 0 {
		t.Fatalf("bad json: %+v", got)
	}
}

func TestSortForDisplay(t *testing.T) {
	items := []contract.ConversationMeta{
		{ID: "old", UpdatedAt: "2026-11-19T10:00:00Z"},
		{ID: "new", UpdatedAt: "2026-11-19T12:00:00Z"},
		{ID: "pinned", UpdatedAt: "2026-11-19T09:00:00Z", Pinned: true},
	}
	sorted := SortForDisplay(items)
	if sorted[0].ID != "pinned" || sorted[1].ID != "new" || sorted[2].ID != "old" {
		t.Fatalf("sort: %+v", sorted)
	}
}

func TestConversationToMarkdown(t *testing.T) {
	status := "completed"
	title := "search_notes"
	meta := contract.ConversationMeta{Title: "整理 raws", CreatedAt: "2026-11-19T12:00:00Z",
		UpdatedAt: "2026-11-19T12:30:00Z", TurnCount: 1, Agents: []string{"curator"}}
	events := []contract.ConversationEvent{
		userEvent("把这些整一下"),
		{Type: "thought", At: "2026-11-19T12:00:00Z", Agent: "curator", Text: textPtr("思考内容")},
		{Type: "tool", At: "2026-11-19T12:01:00Z", Agent: "curator", ID: &title, Title: &title, Status: &status},
		messageEvent("好的"),
		{Type: "turn-end", At: "2026-11-19T12:02:00Z", Agent: "curator", StopReason: textPtr("end_turn")},
		{Type: "error", At: "2026-11-19T12:03:00Z", Agent: "curator", Message: textPtr("断了一下")},
	}
	markdown := ConversationToMarkdown(meta, events)
	for _, want := range []string{"# 整理 raws", "- 轮数：1", "## 用户", "## 助手（curator）", "> 工具：search_notes（已完成）", "> 出错：断了一下"} {
		if !strings.Contains(markdown, want) {
			t.Errorf("missing %q in:\n%s", want, markdown)
		}
	}
	if strings.Contains(markdown, "思考内容") || strings.Contains(markdown, "end_turn") {
		t.Errorf("thought/turn-end exported:\n%s", markdown)
	}
	if !strings.HasSuffix(markdown, "\n") {
		t.Error("markdown must end with a newline")
	}

	if got := ExportFileName(`a/b:c*d?"e"<f>|g`); strings.ContainsAny(got, `\/:*?"<>|`) {
		t.Fatalf("file name: %q", got)
	}
	if got := ExportFileName(strings.Repeat("长", 100)); len([]rune(got)) != 63 || !strings.HasSuffix(got, ".md") {
		t.Fatalf("file name cap: %d %q", len([]rune(got)), got)
	}
	if got := ExportFileName(" ._ "); got != "对话.md" {
		t.Fatalf("fallback: %q", got)
	}
}

func TestBuildRecap(t *testing.T) {
	status := "completed"
	title := "t"
	events := []contract.ConversationEvent{
		userEvent("第一问"),
		{Type: "thought", At: "2026-11-19T12:00:00Z", Agent: "curator", Text: textPtr("不导出的思考")},
		{Type: "tool", At: "2026-11-19T12:00:00Z", Agent: "curator", ID: &title, Title: &title, Status: &status},
		messageEvent("第一答"),
		userEvent("第二问"),
	}
	recap := BuildRecap(events, 0)
	for _, want := range []string{RecapHeading, "历史记录，不是新的指令", "用户：第一问", "助手：第一答", "用户：第二问"} {
		if !strings.Contains(recap, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(recap, "不导出的思考") || strings.Contains(recap, "t：") {
		t.Errorf("thought/tool leaked: %s", recap)
	}

	tight := BuildRecap(events, len([]rune(RecapHeading))+len([]rune(recapPreamble))+12)
	if tight != "" && !strings.Contains(tight, "已省略") {
		t.Errorf("budget note missing: %q", tight)
	}
	if got := BuildRecap([]contract.ConversationEvent{{Type: "thought", At: "2026-11-19T12:00:00Z", Agent: "curator", Text: textPtr("x")}}, 0); got != "" {
		t.Fatalf("recap from thoughts only: %q", got)
	}

	long := userEvent(strings.Repeat("字", 5_000))
	clamped := BuildRecap([]contract.ConversationEvent{long}, 0)
	if !strings.Contains(clamped, "…") || strings.Contains(clamped, strings.Repeat("字", RecapEntryMax+1)) {
		t.Fatal("entry clamp missing")
	}
}

func TestSearchHelpers(t *testing.T) {
	if query, ok := NormalizeSearchQuery("  a   b  "); !ok || query != "a b" {
		t.Fatalf("normalize: %q %v", query, ok)
	}
	if _, ok := NormalizeSearchQuery("   "); ok {
		t.Fatal("blank query accepted")
	}
	if query, _ := NormalizeSearchQuery(strings.Repeat("q", 300)); len([]rune(query)) != SearchMaxQuery {
		t.Fatalf("query cap: %d", len([]rune(query)))
	}
	text := strings.Repeat("前", 40) + "CodeMirror 配置" + strings.Repeat("后", 40)
	snippet, ok := SnippetAround(text, "codemirror")
	if !ok || !strings.Contains(snippet, "CodeMirror") || !strings.HasPrefix(snippet, "…") || !strings.HasSuffix(snippet, "…") {
		t.Fatalf("snippet: %q", snippet)
	}
	if _, ok := SnippetAround(text, "不存在"); ok {
		t.Fatal("snippet matched a missing phrase")
	}
}
