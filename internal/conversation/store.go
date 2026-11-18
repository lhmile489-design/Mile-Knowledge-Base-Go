package conversation

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/index"
)

// indexFileName is the metadata file inside every vault's conversation dir.
const indexFileName = "index.json"

// Store persists conversation history under <userData>/conversations/<vaultId>/
// (数据库设计 4.4). Every change is serialised; index.json is written
// atomically; the jsonl files are append-only and tolerate a half-written last
// line. search.db is derived and rebuilt from the files when damaged.
type Store struct {
	root  string
	now   func() time.Time
	newID func() string

	mu          sync.Mutex
	indexes     map[string]contract.ConversationIndex
	tailChecked map[string]bool
	search      map[string]*searchIndex
	onChanged   func(vaultID string)
	onWarning   func(message string)
}

// NewStore opens (lazily) the conversation history under userDataDir.
func NewStore(userDataDir string) *Store {
	return &Store{
		root:        filepath.Join(userDataDir, "conversations"),
		now:         time.Now,
		newID:       func() string { return randomHex(16) },
		indexes:     map[string]contract.ConversationIndex{},
		tailChecked: map[string]bool{},
		search:      map[string]*searchIndex{},
	}
}

func randomHex(length int) string {
	raw := make([]byte, (length+1)/2)
	_, _ = rand.Read(raw)
	return hex.EncodeToString(raw)[:length]
}

func (s *Store) stamp() string { return s.now().UTC().Format(time.RFC3339Nano) }

// Close releases the open search databases (the JSONL files need no handle).
func (s *Store) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for vaultID, search := range s.search {
		_ = search.Close()
		delete(s.search, vaultID)
	}
}

// SetOnChanged registers the "history changed" hook (E07 wiring is T10).
func (s *Store) SetOnChanged(listener func(vaultID string)) {
	s.mu.Lock()
	s.onChanged = listener
	s.mu.Unlock()
}

// SetOnWarning receives non-fatal search-index repair problems.
func (s *Store) SetOnWarning(listener func(message string)) {
	s.mu.Lock()
	s.onWarning = listener
	s.mu.Unlock()
}

func (s *Store) notifyLocked(vaultID string) {
	if s.onChanged != nil {
		vaultID := vaultID
		listener := s.onChanged
		go func() {
			defer func() { _ = recover() }()
			listener(vaultID)
		}()
	}
}

func (s *Store) warnLocked(message string) {
	if s.onWarning != nil {
		listener := s.onWarning
		go func() {
			defer func() { _ = recover() }()
			listener(message)
		}()
	}
}

func (s *Store) vaultDir(vaultID string) (string, *contract.Failure) {
	if !IsVaultDirName(vaultID) {
		return "", contract.Fail(contract.CodeInvalidInput, "vaultId 非法")
	}
	return filepath.Join(s.root, vaultID), nil
}

func (s *Store) filePath(vaultID, id string) (string, *contract.Failure) {
	if !IsConversationID(id) {
		return "", contract.Fail(contract.CodeInvalidInput, "对话 id 非法")
	}
	dir, f := s.vaultDir(vaultID)
	if f != nil {
		return "", f
	}
	return filepath.Join(dir, id+".jsonl"), nil
}

// ---- reads ----

// List returns the stored index (M50).
func (s *Store) List(vaultID string) contract.ConversationListResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	indexValue, f := s.loadIndexLocked(vaultID)
	if f != nil {
		return contract.ConversationListResult{Failure: f}
	}
	copied := indexValue
	return contract.ConversationListResult{Index: &copied}
}

// Get returns one conversation's events (M51), capped at the newest 5000.
func (s *Store) Get(vaultID, id string) contract.ConversationReadResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getLocked(vaultID, id)
}

func (s *Store) getLocked(vaultID, id string) contract.ConversationReadResult {
	indexValue, f := s.loadIndexLocked(vaultID)
	if f != nil {
		return contract.ConversationReadResult{Failure: f}
	}
	var meta *contract.ConversationMeta
	for i := range indexValue.Conversations {
		if indexValue.Conversations[i].ID == id {
			copied := indexValue.Conversations[i]
			meta = &copied
			break
		}
	}
	if meta == nil {
		return contract.ConversationReadResult{Failure: contract.Fail(contract.CodeNotFound, "对话不存在")}
	}
	path, f := s.filePath(vaultID, id)
	if f != nil {
		return contract.ConversationReadResult{Failure: f}
	}
	raw, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return contract.ConversationReadResult{Failure: contract.Fail(contract.CodeIOFailure, "无法读取对话文件")}
	}
	events, skipped := ParseConversationText(string(raw))
	truncated := len(events) > MaxEventsPerRead
	if truncated {
		events = events[len(events)-MaxEventsPerRead:]
	}
	return contract.ConversationReadResult{Meta: meta, Events: events, Skipped: skipped, Truncated: truncated}
}

// ---- user operations ----

// Create makes a new conversation, reusing the active one while it is still
// empty (M52); repeated "new conversation" clicks do not pile up empties.
func (s *Store) Create(vaultID string) contract.ConversationMetaResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	indexValue, f := s.loadIndexLocked(vaultID)
	if f != nil {
		return contract.ConversationMetaResult{Failure: f}
	}
	if indexValue.ActiveID != nil {
		for i := range indexValue.Conversations {
			meta := indexValue.Conversations[i]
			if meta.ID == *indexValue.ActiveID && meta.TurnCount == 0 {
				return contract.ConversationMetaResult{Conversation: &meta}
			}
		}
	}
	meta, f := s.insertLocked(vaultID, indexValue)
	if f != nil {
		return contract.ConversationMetaResult{Failure: f}
	}
	if f := s.persistIndexLocked(vaultID); f != nil {
		return contract.ConversationMetaResult{Failure: f}
	}
	s.notifyLocked(vaultID)
	return contract.ConversationMetaResult{Conversation: meta}
}

// Rename sets a sanitized title (M53); an empty title is rejected.
func (s *Store) Rename(vaultID, id, title string) contract.ConversationMetaResult {
	clean, ok := SanitizeTitle(title)
	if !ok {
		return contract.ConversationMetaResult{Failure: contract.Fail(contract.CodeInvalidInput, "标题不能为空")}
	}
	return s.mutateMeta(vaultID, id, func(meta contract.ConversationMeta) contract.ConversationMeta {
		meta.Title = clean
		return meta
	})
}

// Pin toggles the pinned flag (M54).
func (s *Store) Pin(vaultID, id string, pinned bool) contract.ConversationMetaResult {
	return s.mutateMeta(vaultID, id, func(meta contract.ConversationMeta) contract.ConversationMeta {
		meta.Pinned = pinned
		return meta
	})
}

// Remove deletes one conversation and its derived search rows (M55).
func (s *Store) Remove(vaultID, id string) *contract.Failure {
	s.mu.Lock()
	defer s.mu.Unlock()
	indexValue, f := s.loadIndexLocked(vaultID)
	if f != nil {
		return f
	}
	found := false
	kept := make([]contract.ConversationMeta, 0, len(indexValue.Conversations))
	for _, meta := range indexValue.Conversations {
		if meta.ID == id {
			found = true
			continue
		}
		kept = append(kept, meta)
	}
	if !found {
		return contract.Fail(contract.CodeNotFound, "对话不存在")
	}
	path, f := s.filePath(vaultID, id)
	if f != nil {
		return f
	}
	_ = os.Remove(path)
	delete(s.tailChecked, vaultID+"/"+id)
	activeID := indexValue.ActiveID
	if activeID != nil && *activeID == id {
		activeID = nil
	}
	s.indexes[vaultID] = contract.ConversationIndex{ActiveID: activeID, Conversations: kept}
	if f := s.persistIndexLocked(vaultID); f != nil {
		return f
	}
	if search := s.search[vaultID]; search != nil {
		if err := search.DeleteConversation(id); err != nil {
			s.warnLocked("对话搜索索引清理失败：" + err.Error())
		}
	}
	s.notifyLocked(vaultID)
	return nil
}

// Activate makes an existing conversation the recording target (M56); the
// service layer adds the running-agent guard.
func (s *Store) Activate(vaultID, id string) contract.ConversationMetaResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	indexValue, f := s.loadIndexLocked(vaultID)
	if f != nil {
		return contract.ConversationMetaResult{Failure: f}
	}
	var found *contract.ConversationMeta
	for i := range indexValue.Conversations {
		if indexValue.Conversations[i].ID == id {
			copied := indexValue.Conversations[i]
			found = &copied
			break
		}
	}
	if found == nil {
		return contract.ConversationMetaResult{Failure: contract.Fail(contract.CodeNotFound, "对话不存在")}
	}
	active := id
	indexValue.ActiveID = &active
	s.indexes[vaultID] = indexValue
	if f := s.persistIndexLocked(vaultID); f != nil {
		return contract.ConversationMetaResult{Failure: f}
	}
	s.notifyLocked(vaultID)
	return contract.ConversationMetaResult{Conversation: found}
}

// ClearAll removes every conversation file of the vault, including stray
// jsonl files not listed in index.json (M59), and drops the search database.
func (s *Store) ClearAll(vaultID string) contract.ConversationClearResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir, f := s.vaultDir(vaultID)
	if f != nil {
		return contract.ConversationClearResult{Failure: f}
	}
	if _, f := s.loadIndexLocked(vaultID); f != nil {
		return contract.ConversationClearResult{Failure: f}
	}
	removed := 0
	names, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return contract.ConversationClearResult{Failure: contract.Fail(contract.CodeIOFailure, "无法读取对话目录")}
	}
	for _, entry := range names {
		if !filePattern.MatchString(entry.Name()) {
			continue
		}
		if err := os.Remove(filepath.Join(dir, entry.Name())); err == nil {
			removed++
		}
	}
	prefix := vaultID + "/"
	for key := range s.tailChecked {
		if strings.HasPrefix(key, prefix) {
			delete(s.tailChecked, key)
		}
	}
	s.indexes[vaultID] = contract.ConversationIndex{Conversations: []contract.ConversationMeta{}}
	if f := s.persistIndexLocked(vaultID); f != nil {
		return contract.ConversationClearResult{Failure: f}
	}
	if search := s.search[vaultID]; search != nil {
		_ = search.Close()
		delete(s.search, vaultID)
	}
	DropSearchFiles(filepath.Join(dir, "search.db"))
	s.notifyLocked(vaultID)
	return contract.ConversationClearResult{Removed: removed}
}

// ---- recording ----

// Append writes one event to the active conversation. Only a user event may
// open a new conversation; other events without an active target are dropped.
// The size limit is checked only when a new turn starts, so a reply is never
// split. It returns the conversation id and whether the event was written.
func (s *Store) Append(vaultID string, event contract.ConversationEvent) (string, bool, *contract.Failure) {
	s.mu.Lock()
	defer s.mu.Unlock()
	indexValue, f := s.loadIndexLocked(vaultID)
	if f != nil {
		return "", false, f
	}
	var meta *contract.ConversationMeta
	if indexValue.ActiveID != nil {
		for i := range indexValue.Conversations {
			if indexValue.Conversations[i].ID == *indexValue.ActiveID {
				copied := indexValue.Conversations[i]
				meta = &copied
				break
			}
		}
	}
	if meta != nil && event.Type == "user" {
		path, f := s.filePath(vaultID, meta.ID)
		if f != nil {
			return "", false, f
		}
		if size := fileSize(path); size >= MaxBytes {
			meta = nil
		}
	}
	if meta == nil {
		if event.Type != "user" {
			return "", false, nil
		}
		created, f := s.insertLocked(vaultID, indexValue)
		if f != nil {
			return "", false, f
		}
		meta = created
		indexValue = s.indexes[vaultID]
	}

	path, f := s.filePath(vaultID, meta.ID)
	if f != nil {
		return "", false, f
	}
	dir, _ := s.vaultDir(vaultID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", false, contract.Fail(contract.CodeIOFailure, "无法创建对话目录")
	}
	prefix := ""
	if s.ensureLineBoundaryLocked(vaultID, meta.ID, path) {
		prefix = "\n"
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return "", false, contract.Fail(contract.CodeIOFailure, "无法写入对话文件")
	}
	_, writeErr := file.WriteString(prefix + SerializeEvent(event))
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		return "", false, contract.Fail(contract.CodeIOFailure, "无法写入对话文件")
	}

	next := applyEvent(*meta, event)
	for i := range indexValue.Conversations {
		if indexValue.Conversations[i].ID == next.ID {
			indexValue.Conversations[i] = next
		}
	}
	s.indexes[vaultID] = indexValue
	if event.Type == "user" || event.Type == "turn-end" {
		if f := s.persistIndexLocked(vaultID); f != nil {
			return next.ID, true, f
		}
		s.notifyLocked(vaultID)
	}
	s.indexEventLocked(vaultID, next.ID, event)
	return next.ID, true, nil
}

// indexEventLocked writes user/message text into the derived search database.
// Failures are reported but never fail the append: the jsonl is the source of
// truth and the next rebuild repairs the index.
func (s *Store) indexEventLocked(vaultID, conversationID string, event contract.ConversationEvent) {
	if event.Type != "user" && event.Type != "message" {
		return
	}
	search := s.searchForLocked(vaultID)
	if search == nil {
		return
	}
	seq, err := search.NextSeq(conversationID)
	if err == nil {
		err = search.IndexEvent(conversationID, seq, event.Type, deref(event.Text))
	}
	if err == nil && (event.Type == "user" || event.Type == "message") {
		indexValue := s.indexes[vaultID]
		for _, meta := range indexValue.Conversations {
			if meta.ID == conversationID {
				err = search.UpsertMeta(conversationID, meta.Title, meta.UpdatedAt)
				break
			}
		}
	}
	if err != nil {
		s.warnLocked("对话搜索索引写入失败：" + err.Error())
	}
}

// ---- search ----

// Search returns up to 50 conversation hits for a query (M57). Only user and
// assistant text is searched; excerpts are ±30 characters around the match.
func (s *Store) Search(vaultID, rawQuery string) contract.ConversationSearchResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	query, ok := NormalizeSearchQuery(rawQuery)
	if !ok {
		return contract.ConversationSearchResult{VaultID: vaultID, Hits: []contract.ConversationSearchHit{},
			Failure: contract.Fail(contract.CodeInvalidInput, "query 不能为空")}
	}
	if _, f := s.vaultDir(vaultID); f != nil {
		return contract.ConversationSearchResult{VaultID: vaultID, Hits: []contract.ConversationSearchHit{}, Failure: f}
	}
	if _, f := s.loadIndexLocked(vaultID); f != nil {
		return contract.ConversationSearchResult{VaultID: vaultID, Hits: []contract.ConversationSearchHit{}, Failure: f}
	}
	tokens := index.Tokenize(query)
	hits := []contract.ConversationSearchHit{}
	search := s.searchForLocked(vaultID)
	if search == nil || len(tokens) == 0 {
		return contract.ConversationSearchResult{VaultID: vaultID, Query: query, Hits: hits}
	}
	rows, err := search.Query(tokens)
	if err != nil {
		s.warnLocked("对话搜索失败：" + err.Error())
		return contract.ConversationSearchResult{VaultID: vaultID, Query: query, Hits: hits}
	}
	for _, row := range rows {
		excerpt, ok := SnippetAround(row.text, query)
		if !ok {
			excerpt = fallbackExcerpt(row.text, tokens)
		}
		hits = append(hits, contract.ConversationSearchHit{
			ConversationID: row.conversationID, Title: row.title,
			Excerpt: excerpt, MatchedAt: row.updatedAt,
		})
	}
	return contract.ConversationSearchResult{VaultID: vaultID, Query: query, Hits: hits}
}

// searchForLocked returns the open search database, rebuilding the derived
// file from the jsonl conversations when it is missing, corrupt or written by
// another schema version.
func (s *Store) searchForLocked(vaultID string) *searchIndex {
	if search := s.search[vaultID]; search != nil {
		return search
	}
	dir, f := s.vaultDir(vaultID)
	if f != nil {
		return nil
	}
	path := filepath.Join(dir, "search.db")
	opened, err := openSearchIndex(path)
	if err == nil {
		s.search[vaultID] = opened
		return opened
	}
	DropSearchFiles(path)
	opened, err = openSearchIndex(path)
	if err != nil {
		s.warnLocked("对话搜索索引不可用：" + err.Error())
		return nil
	}
	s.search[vaultID] = opened
	if err := s.rebuildSearchLocked(vaultID, opened); err != nil {
		s.warnLocked("对话搜索索引重建失败：" + err.Error())
	}
	return opened
}

// rebuildSearchLocked scans every jsonl file and repopulates the FTS rows;
// the files are the source of truth.
func (s *Store) rebuildSearchLocked(vaultID string, search *searchIndex) error {
	dir, f := s.vaultDir(vaultID)
	if f != nil {
		return nil
	}
	names, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, entry := range names {
		match := filePattern.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		events, _ := ParseConversationText(string(raw))
		if len(events) == 0 {
			continue
		}
		meta := deriveMetaFromEvents(match[1], events)
		if err := search.UpsertMeta(meta.ID, meta.Title, meta.UpdatedAt); err != nil {
			return err
		}
		seq := int64(0)
		for _, event := range events {
			if event.Type != "user" && event.Type != "message" {
				continue
			}
			seq++
			if err := search.IndexEvent(meta.ID, seq, event.Type, deref(event.Text)); err != nil {
				return err
			}
		}
	}
	return nil
}

// ---- index handling ----

func (s *Store) loadIndexLocked(vaultID string) (contract.ConversationIndex, *contract.Failure) {
	if cached, ok := s.indexes[vaultID]; ok {
		return cached, nil
	}
	dir, f := s.vaultDir(vaultID)
	if f != nil {
		return contract.ConversationIndex{}, f
	}
	indexPath := filepath.Join(dir, indexFileName)
	raw, err := os.ReadFile(indexPath)
	indexValue := contract.ConversationIndex{Conversations: []contract.ConversationMeta{}}
	rebuilt := false
	corrupt := false
	switch {
	case err == nil:
		if json.Valid(raw) {
			indexValue = CoerceIndex(raw)
		} else {
			corrupt = true
			rebuilt = true
		}
	case errors.Is(err, fs.ErrNotExist):
		rebuilt = true
	default:
		return contract.ConversationIndex{}, contract.Fail(contract.CodeIOFailure, "无法读取对话索引")
	}
	if err != nil || corrupt {
		if corrupt {
			_ = os.Rename(indexPath, indexPath+"."+randomHex(6)+".corrupt")
		}
		indexValue = s.rebuildFromFilesLocked(dir)
		rebuilt = len(indexValue.Conversations) > 0 || corrupt
	}
	s.indexes[vaultID] = indexValue
	if rebuilt {
		if f := s.persistIndexLocked(vaultID); f != nil {
			return contract.ConversationIndex{}, f
		}
	}
	return indexValue, nil
}

// rebuildFromFilesLocked recovers the index by scanning the jsonl files after
// index.json was lost or corrupted.
func (s *Store) rebuildFromFilesLocked(dir string) contract.ConversationIndex {
	names, err := os.ReadDir(dir)
	if err != nil {
		return contract.ConversationIndex{Conversations: []contract.ConversationMeta{}}
	}
	found := []contract.ConversationMeta{}
	for _, entry := range names {
		match := filePattern.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		events, _ := ParseConversationText(string(raw))
		if len(events) == 0 {
			continue
		}
		found = append(found, deriveMetaFromEvents(match[1], events))
	}
	sortStable(found, func(a, b contract.ConversationMeta) bool {
		return parseTime(a.UpdatedAt).After(parseTime(b.UpdatedAt))
	})
	indexValue := contract.ConversationIndex{Conversations: []contract.ConversationMeta{}}
	for _, meta := range found {
		if len(indexValue.Conversations) >= MaxConversations {
			break
		}
		indexValue.Conversations = append(indexValue.Conversations, meta)
	}
	return indexValue
}

func deriveMetaFromEvents(id string, events []contract.ConversationEvent) contract.ConversationMeta {
	title := DefaultTitle
	for _, event := range events {
		if event.Type == "user" {
			title = DeriveTitle(deref(event.Text))
			break
		}
	}
	agents := []string{}
	seen := map[string]bool{}
	turnCount := 0
	for _, event := range events {
		if !seen[event.Agent] {
			seen[event.Agent] = true
			agents = append(agents, event.Agent)
		}
		if event.Type == "user" {
			turnCount++
		}
	}
	return contract.ConversationMeta{
		ID: id, Title: title, CreatedAt: events[0].At, UpdatedAt: events[len(events)-1].At,
		TurnCount: turnCount, Agents: agents, Pinned: false,
	}
}

func (s *Store) persistIndexLocked(vaultID string) *contract.Failure {
	dir, f := s.vaultDir(vaultID)
	if f != nil {
		return f
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return contract.Fail(contract.CodeIOFailure, "无法创建对话目录")
	}
	indexValue := s.indexes[vaultID]
	raw, err := json.MarshalIndent(indexValue, "", "  ")
	if err != nil {
		return contract.Fail(contract.CodeIOFailure, "无法序列化对话索引")
	}
	tmp := filepath.Join(dir, indexFileName+"."+randomHex(8)+".tmp")
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o644); err != nil {
		_ = os.Remove(tmp)
		return contract.Fail(contract.CodeIOFailure, "无法写入对话索引")
	}
	if err := os.Rename(tmp, filepath.Join(dir, indexFileName)); err != nil {
		_ = os.Remove(tmp)
		return contract.Fail(contract.CodeIOFailure, "无法写入对话索引")
	}
	return nil
}

func (s *Store) mutateMeta(vaultID, id string, change func(contract.ConversationMeta) contract.ConversationMeta) contract.ConversationMetaResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	indexValue, f := s.loadIndexLocked(vaultID)
	if f != nil {
		return contract.ConversationMetaResult{Failure: f}
	}
	found := -1
	for i := range indexValue.Conversations {
		if indexValue.Conversations[i].ID == id {
			found = i
			break
		}
	}
	if found < 0 {
		return contract.ConversationMetaResult{Failure: contract.Fail(contract.CodeNotFound, "对话不存在")}
	}
	next := change(indexValue.Conversations[found])
	indexValue.Conversations[found] = next
	s.indexes[vaultID] = indexValue
	if f := s.persistIndexLocked(vaultID); f != nil {
		return contract.ConversationMetaResult{Failure: f}
	}
	if search := s.search[vaultID]; search != nil {
		if err := search.UpsertMeta(next.ID, next.Title, next.UpdatedAt); err != nil {
			s.warnLocked("对话搜索索引更新失败：" + err.Error())
		}
	}
	s.notifyLocked(vaultID)
	return contract.ConversationMetaResult{Conversation: &next}
}

func (s *Store) insertLocked(vaultID string, indexValue contract.ConversationIndex) (*contract.ConversationMeta, *contract.Failure) {
	if len(indexValue.Conversations) >= MaxConversations {
		return nil, contract.Fail(contract.CodeTooLarge, "对话数量已达上限，请先删除一些旧对话")
	}
	at := s.stamp()
	meta := contract.ConversationMeta{
		ID: s.newID(), Title: DefaultTitle, CreatedAt: at, UpdatedAt: at,
		TurnCount: 0, Agents: []string{}, Pinned: false,
	}
	active := meta.ID
	next := contract.ConversationIndex{ActiveID: &active, Conversations: []contract.ConversationMeta{}}
	next.Conversations = append(next.Conversations, meta)
	next.Conversations = append(next.Conversations, indexValue.Conversations...)
	s.indexes[vaultID] = next
	copied := meta
	return &copied, nil
}

func (s *Store) ensureLineBoundaryLocked(vaultID, id, path string) bool {
	key := vaultID + "/" + id
	if s.tailChecked[key] {
		return false
	}
	needsNewline := false
	if size := fileSize(path); size > 0 {
		file, err := os.Open(path)
		if err == nil {
			var last [1]byte
			if _, readErr := file.ReadAt(last[:], size-1); readErr == nil {
				needsNewline = last[0] != '\n'
			}
			_ = file.Close()
		}
	}
	s.tailChecked[key] = true
	return needsNewline
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func applyEvent(meta contract.ConversationMeta, event contract.ConversationEvent) contract.ConversationMeta {
	hasAgent := false
	for _, agent := range meta.Agents {
		if agent == event.Agent {
			hasAgent = true
			break
		}
	}
	if !hasAgent {
		meta.Agents = append(meta.Agents, event.Agent)
	}
	switch event.Type {
	case "user":
		meta.TurnCount++
		meta.UpdatedAt = event.At
		if meta.Title == DefaultTitle {
			meta.Title = DeriveTitle(deref(event.Text))
		}
	case "turn-end":
		meta.UpdatedAt = event.At
	}
	return meta
}
