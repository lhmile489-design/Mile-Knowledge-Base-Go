package conversation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

const testVault = "vault1"

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store := NewStore(t.TempDir())
	store.now = func() time.Time { return time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC) }
	t.Cleanup(store.Close)
	return store
}

func mustAppend(t *testing.T, store *Store, event contract.ConversationEvent) string {
	t.Helper()
	id, ok, f := store.Append(testVault, event)
	if f != nil || !ok {
		t.Fatalf("append: %+v ok=%v", f, ok)
	}
	return id
}

func TestStoreCreateReusesEmptyActive(t *testing.T) {
	store := newTestStore(t)
	first := store.Create(testVault)
	if first.Failure != nil || first.Conversation == nil {
		t.Fatalf("create: %+v", first)
	}
	second := store.Create(testVault)
	if second.Conversation == nil || second.Conversation.ID != first.Conversation.ID {
		t.Fatalf("empty active not reused: %+v", second.Conversation)
	}
	if got := store.List(testVault); len(got.Index.Conversations) != 1 {
		t.Fatalf("list: %+v", got.Index)
	}

	mustAppend(t, store, userEvent("开始对话"))
	third := store.Create(testVault)
	if third.Conversation == nil || third.Conversation.ID == first.Conversation.ID {
		t.Fatalf("used active still reused: %+v", third.Conversation)
	}
	if got := store.List(testVault); len(got.Index.Conversations) != 2 {
		t.Fatalf("count: %+v", got.Index)
	}
}

func TestStoreAppendDerivesMeta(t *testing.T) {
	store := newTestStore(t)
	id := mustAppend(t, store, userEvent("帮我整理这些笔记"))
	if id == "" {
		t.Fatal("no id")
	}
	messageAt := contract.ConversationEvent{Type: "message", At: "2026-11-19T12:01:00Z", Agent: "scout", Text: textPtr("好的")}
	if _, ok, f := store.Append(testVault, messageAt); f != nil || !ok {
		t.Fatalf("message append: %+v", f)
	}
	turnEnd := contract.ConversationEvent{Type: "turn-end", At: "2026-11-19T12:02:00Z", Agent: "scout", StopReason: textPtr("end_turn")}
	if _, ok, f := store.Append(testVault, turnEnd); f != nil || !ok {
		t.Fatalf("turn-end append: %+v", f)
	}
	list := store.List(testVault)
	if len(list.Index.Conversations) != 1 {
		t.Fatalf("list: %+v", list.Index)
	}
	meta := list.Index.Conversations[0]
	if meta.TurnCount != 1 || meta.Title != "帮我整理这些笔记" {
		t.Fatalf("meta: %+v", meta)
	}
	if len(meta.Agents) != 2 || meta.Agents[0] != "curator" || meta.Agents[1] != "scout" {
		t.Fatalf("agents: %+v", meta.Agents)
	}
	if meta.UpdatedAt != "2026-11-19T12:02:00Z" {
		t.Fatalf("updatedAt: %s", meta.UpdatedAt)
	}
	read := store.Get(testVault, id)
	if read.Failure != nil || len(read.Events) != 3 || read.Skipped != 0 || read.Truncated {
		t.Fatalf("get: %+v", read)
	}
}

func TestStoreDropsUnattributedEvents(t *testing.T) {
	store := newTestStore(t)
	if _, ok, f := store.Append(testVault, messageEvent("没有归属")); f != nil || ok {
		t.Fatalf("message without active conversation accepted: ok=%v %+v", ok, f)
	}
	if got := store.List(testVault); len(got.Index.Conversations) != 0 {
		t.Fatalf("conversation created: %+v", got.Index)
	}
}

func TestStoreLineBoundaryRepair(t *testing.T) {
	store := newTestStore(t)
	created := store.Create(testVault)
	dir, _ := store.vaultDir(testVault)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, created.Conversation.ID+".jsonl")
	partial := strings.TrimSuffix(SerializeEvent(userEvent("第一")), "\n")
	if err := os.WriteFile(path, []byte(partial), 0o644); err != nil {
		t.Fatal(err)
	}
	mustAppend(t, store, userEvent("第二"))
	read := store.Get(testVault, created.Conversation.ID)
	if read.Failure != nil || len(read.Events) != 2 || read.Skipped != 0 {
		t.Fatalf("boundary repair: events=%d skipped=%d %+v", len(read.Events), read.Skipped, read.Failure)
	}
}

func TestStoreGetTruncatesAtReadLimit(t *testing.T) {
	store := newTestStore(t)
	created := store.Create(testVault)
	dir, _ := store.vaultDir(testVault)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var builder strings.Builder
	for i := 0; i < MaxEventsPerRead+2; i++ {
		builder.WriteString(SerializeEvent(userEvent(fmt.Sprintf("问 %d", i))))
	}
	builder.WriteString("坏行\n")
	if err := os.WriteFile(filepath.Join(dir, created.Conversation.ID+".jsonl"), []byte(builder.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	read := store.Get(testVault, created.Conversation.ID)
	if !read.Truncated || len(read.Events) != MaxEventsPerRead || read.Skipped != 1 {
		t.Fatalf("truncated=%v events=%d skipped=%d", read.Truncated, len(read.Events), read.Skipped)
	}
	if text := deref(read.Events[len(read.Events)-1].Text); text != fmt.Sprintf("问 %d", MaxEventsPerRead+1) {
		t.Fatalf("tail: %q", text)
	}
}

func TestStoreSizeLimitStartsNewConversation(t *testing.T) {
	store := newTestStore(t)
	first := mustAppend(t, store, userEvent("第一轮"))
	dir, _ := store.vaultDir(testVault)
	path := filepath.Join(dir, first+".jsonl")
	if err := os.WriteFile(path, make([]byte, MaxBytes), 0o644); err != nil {
		t.Fatal(err)
	}
	second := mustAppend(t, store, userEvent("第二轮"))
	if second == first {
		t.Fatalf("size limit did not start a new conversation (%s)", second)
	}
	if got := store.List(testVault); len(got.Index.Conversations) != 2 {
		t.Fatalf("count: %+v", got.Index)
	}
}

func TestStoreMaxConversations(t *testing.T) {
	store := newTestStore(t)
	dir, _ := store.vaultDir(testVault)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	metas := make([]contract.ConversationMeta, 0, MaxConversations)
	for i := 0; i < MaxConversations; i++ {
		metas = append(metas, contract.ConversationMeta{
			ID: fmt.Sprintf("c%d", i), Title: "标题", CreatedAt: "2026-11-19T12:00:00Z",
			UpdatedAt: "2026-11-19T12:00:00Z", Agents: []string{},
		})
	}
	raw, _ := json.Marshal(contract.ConversationIndex{Conversations: metas})
	if err := os.WriteFile(filepath.Join(dir, indexFileName), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	result := store.Create(testVault)
	if result.Failure == nil || result.Failure.Code != contract.CodeTooLarge {
		t.Fatalf("cap: %+v", result)
	}
}

func TestStoreIndexCorruptionRebuildsFromFiles(t *testing.T) {
	store := newTestStore(t)
	dir, _ := store.vaultDir(testVault)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	older := contract.ConversationEvent{Type: "user", At: "2026-11-19T10:00:00Z", Agent: "curator", Text: textPtr("旧对话")}
	newer := contract.ConversationEvent{Type: "user", At: "2026-11-19T11:00:00Z", Agent: "scout", Text: textPtr("新对话")}
	os.WriteFile(filepath.Join(dir, "aaaa.jsonl"), []byte(SerializeEvent(older)), 0o644)
	os.WriteFile(filepath.Join(dir, "bbbb.jsonl"), []byte(SerializeEvent(newer)), 0o644)
	os.WriteFile(filepath.Join(dir, indexFileName), []byte("not json"), 0o644)

	list := store.List(testVault)
	if list.Failure != nil || len(list.Index.Conversations) != 2 {
		t.Fatalf("rebuild: %+v", list)
	}
	if list.Index.Conversations[0].ID != "bbbb" || list.Index.Conversations[1].ID != "aaaa" {
		t.Fatalf("order: %+v", list.Index.Conversations)
	}
	if list.Index.Conversations[0].Title != "新对话" || list.Index.Conversations[1].TurnCount != 1 {
		t.Fatalf("derived meta: %+v", list.Index.Conversations)
	}
	raw, err := os.ReadFile(filepath.Join(dir, indexFileName))
	if err != nil {
		t.Fatal(err)
	}
	if rebuilt := CoerceIndex(raw); len(rebuilt.Conversations) != 2 {
		t.Fatalf("index not rewritten: %q", raw)
	}
}

func TestStoreRemoveActivateClear(t *testing.T) {
	store := newTestStore(t)
	first := mustAppend(t, store, userEvent("第一"))
	second := mustAppend(t, store, userEvent("第二")) // second turn? same conversation; create new explicitly below
	_ = second
	created := store.Create(testVault)
	if created.Conversation == nil {
		t.Fatalf("create: %+v", created)
	}

	if f := store.Activate(testVault, first); f.Failure != nil {
		t.Fatalf("activate: %+v", f)
	}
	list := store.List(testVault)
	if list.Index.ActiveID == nil || *list.Index.ActiveID != first {
		t.Fatalf("active: %+v", list.Index)
	}

	if f := store.Remove(testVault, first); f != nil {
		t.Fatalf("remove: %+v", f)
	}
	if f := store.Remove(testVault, first); f == nil || f.Code != contract.CodeNotFound {
		t.Fatalf("second remove: %+v", f)
	}
	if got := store.List(testVault); got.Index.ActiveID != nil {
		t.Fatalf("active not cleared: %+v", got.Index.ActiveID)
	}

	dir, _ := store.vaultDir(testVault)
	os.WriteFile(filepath.Join(dir, "stray.jsonl"), []byte{}, 0o644)
	cleared := store.ClearAll(testVault)
	if cleared.Failure != nil || cleared.Removed != 1 { // the registered file was deleted above; the stray remains
		t.Fatalf("clear: %+v", cleared)
	}
	if got := store.List(testVault); len(got.Index.Conversations) != 0 {
		t.Fatalf("after clear: %+v", got.Index)
	}
	if _, err := os.Stat(filepath.Join(dir, "search.db")); !os.IsNotExist(err) {
		t.Fatalf("search.db kept")
	}
}

func TestStoreSearch(t *testing.T) {
	store := newTestStore(t)
	mustAppend(t, store, userEvent("请检索知识库里的 CodeMirror 配置"))
	mustAppend(t, store, messageEvent("CodeMirror 的配置在 raws/a.md 里"))
	thought := contract.ConversationEvent{Type: "thought", At: "2026-11-19T12:00:01Z", Agent: "curator", Text: textPtr("思考 CodeMirror")}
	store.Append(testVault, thought)

	result := store.Search(testVault, "codemirror")
	if result.Failure != nil || len(result.Hits) != 1 {
		t.Fatalf("search: %+v", result)
	}
	hit := result.Hits[0]
	if !strings.Contains(hit.Excerpt, "CodeMirror") || hit.ConversationID == "" || hit.Title == "" || hit.MatchedAt == "" {
		t.Fatalf("hit: %+v", hit)
	}
	if got := store.Search(testVault, "思考"); len(got.Hits) != 0 {
		t.Fatalf("thought searched: %+v", got.Hits)
	}
	if got := store.Search(testVault, "   "); got.Failure == nil || got.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("blank query: %+v", got)
	}

	if f := store.Remove(testVault, hit.ConversationID); f != nil {
		t.Fatal(f)
	}
	if got := store.Search(testVault, "codemirror"); len(got.Hits) != 0 {
		t.Fatalf("removed conversation still searchable: %+v", got.Hits)
	}
}

func TestStoreSearchRebuildsCorruptDatabase(t *testing.T) {
	store := newTestStore(t)
	mustAppend(t, store, userEvent("知识库里有 CodeMirror 吗"))

	store.mu.Lock()
	if search := store.search[testVault]; search != nil {
		_ = search.Close()
		delete(store.search, testVault)
	}
	dir, _ := store.vaultDir(testVault)
	store.mu.Unlock()
	DropSearchFiles(filepath.Join(dir, "search.db"))
	os.WriteFile(filepath.Join(dir, "search.db"), []byte("garbage"), 0o644)

	result := store.Search(testVault, "CodeMirror")
	if result.Failure != nil || len(result.Hits) != 1 {
		t.Fatalf("rebuild search: %+v", result)
	}
}

func TestStoreSearchSchemaMismatchRebuilds(t *testing.T) {
	store := newTestStore(t)
	mustAppend(t, store, userEvent("知识库里有 CodeMirror 吗"))

	store.mu.Lock()
	search := store.search[testVault]
	if search != nil {
		_, _ = search.db.Exec(`UPDATE meta SET value = '99' WHERE key = 'schema_version'`)
		_ = search.Close()
		delete(store.search, testVault)
	}
	store.mu.Unlock()

	result := store.Search(testVault, "CodeMirror")
	if result.Failure != nil || len(result.Hits) != 1 {
		t.Fatalf("mismatch rebuild: %+v", result)
	}
}
