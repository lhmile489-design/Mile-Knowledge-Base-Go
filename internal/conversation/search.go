package conversation

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/index"
)

// Search limits (shared/conversationSearch.ts:17-20).
const (
	SearchMaxQuery   = 200
	SearchMaxResults = 50
	snippetRadius    = 30
)

// searchSchemaVersion guards the conversation FTS database; a mismatch means
// derived data that must be rebuilt from the jsonl files.
const searchSchemaVersion = 1

// errSearchSchemaMismatch reports a search database this build cannot use.
var errSearchSchemaMismatch = errors.New("conversation search schema mismatch")

// searchIndex is the derived FTS5 database for one vault's conversations.
// The jsonl files stay the source of truth; every failure here can be repaired
// by scanning them again.
type searchIndex struct {
	db *sql.DB
}

func searchDSN(path string) string {
	return "file:" + filepath.ToSlash(path) +
		"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)" +
		"&_pragma=synchronous(NORMAL)"
}

func openSearchIndex(path string) (*searchIndex, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", searchDSN(path))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := ensureSearchSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &searchIndex{db: db}, nil
}

func ensureSearchSchema(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS conv_meta (
			conversation_id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS conv_text (
			rowid INTEGER PRIMARY KEY AUTOINCREMENT,
			conversation_id TEXT NOT NULL,
			event_seq INTEGER NOT NULL,
			role TEXT NOT NULL,
			text TEXT NOT NULL,
			UNIQUE(conversation_id, event_seq)
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS conv_fts USING fts5(
			conversation_id UNINDEXED,
			event_seq UNINDEXED,
			role UNINDEXED,
			text,
			tokenize='unicode61'
		)`,
		`CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return err
		}
	}
	var version string
	err := db.QueryRow(`SELECT value FROM meta WHERE key = 'schema_version'`).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = db.Exec(`INSERT INTO meta(key, value) VALUES('schema_version', ?)`,
			fmt.Sprintf("%d", searchSchemaVersion))
		return err
	}
	if err != nil {
		return err
	}
	if version != fmt.Sprintf("%d", searchSchemaVersion) {
		return errSearchSchemaMismatch
	}
	return nil
}

func (s *searchIndex) Close() error { return s.db.Close() }

// DropSearchFiles removes the database file and its WAL sidecars.
func DropSearchFiles(path string) {
	for _, candidate := range []string{path, path + "-wal", path + "-shm"} {
		_ = os.Remove(candidate)
	}
}

// UpsertMeta refreshes the derived title and updated time of one conversation.
func (s *searchIndex) UpsertMeta(conversationID, title, updatedAt string) error {
	_, err := s.db.Exec(
		`INSERT INTO conv_meta(conversation_id, title, updated_at) VALUES(?, ?, ?)
		 ON CONFLICT(conversation_id) DO UPDATE SET title = excluded.title, updated_at = excluded.updated_at`,
		conversationID, title, updatedAt)
	return err
}

// DeleteConversation removes every row of one conversation.
func (s *searchIndex) DeleteConversation(conversationID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(
		`DELETE FROM conv_fts WHERE rowid IN (SELECT rowid FROM conv_text WHERE conversation_id = ?)`,
		conversationID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM conv_text WHERE conversation_id = ?`, conversationID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM conv_meta WHERE conversation_id = ?`, conversationID); err != nil {
		return err
	}
	return tx.Commit()
}

// NextSeq returns the next per-conversation event sequence number.
func (s *searchIndex) NextSeq(conversationID string) (int64, error) {
	var next int64
	err := s.db.QueryRow(
		`SELECT COALESCE(MAX(event_seq), 0) + 1 FROM conv_text WHERE conversation_id = ?`,
		conversationID).Scan(&next)
	return next, err
}

// IndexEvent stores one user/message event: the original text for excerpts
// plus its pre-tokenised form for FTS matching.
func (s *searchIndex) IndexEvent(conversationID string, eventSeq int64, role, text string) error {
	tokens := index.Tokenize(text)
	if len(tokens) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.Exec(
		`INSERT OR REPLACE INTO conv_text(conversation_id, event_seq, role, text) VALUES(?, ?, ?, ?)`,
		conversationID, eventSeq, role, text)
	if err != nil {
		return err
	}
	rowid, err := result.LastInsertId()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(
		`INSERT INTO conv_fts(rowid, conversation_id, event_seq, role, text) VALUES(?, ?, ?, ?, ?)`,
		rowid, conversationID, eventSeq, role, strings.Join(tokens, " ")); err != nil {
		return err
	}
	return tx.Commit()
}

// searchRow is one conversation-level match with its excerpt source.
type searchRow struct {
	conversationID string
	title          string
	updatedAt      string
	text           string
}

// Query returns one row per matching conversation (earliest matching event),
// most recently updated first, capped at SearchMaxResults.
func (s *searchIndex) Query(tokens []string) ([]searchRow, error) {
	if len(tokens) == 0 {
		return nil, nil
	}
	quoted := make([]string, len(tokens))
	for i, token := range tokens {
		quoted[i] = `"` + strings.ReplaceAll(token, `"`, `""`) + `"`
	}
	match := strings.Join(quoted, " OR ")
	rows, err := s.db.Query(
		`SELECT conversation_id, MIN(event_seq) FROM conv_fts WHERE conv_fts MATCH ? GROUP BY conversation_id`,
		match)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type matchRow struct {
		conversationID string
		seq            int64
	}
	matches := []matchRow{}
	for rows.Next() {
		var item matchRow
		if err := rows.Scan(&item.conversationID, &item.seq); err != nil {
			return nil, err
		}
		matches = append(matches, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]searchRow, 0, len(matches))
	for _, item := range matches {
		var title, updatedAt string
		err := s.db.QueryRow(
			`SELECT title, updated_at FROM conv_meta WHERE conversation_id = ?`, item.conversationID).
			Scan(&title, &updatedAt)
		if err != nil {
			continue // meta missing: the conversation was removed meanwhile
		}
		var text string
		err = s.db.QueryRow(
			`SELECT text FROM conv_text WHERE conversation_id = ? AND event_seq = ?`,
			item.conversationID, item.seq).Scan(&text)
		if err != nil {
			continue
		}
		out = append(out, searchRow{conversationID: item.conversationID, title: title, updatedAt: updatedAt, text: text})
	}
	parsed := func(value string) int64 {
		t := parseTime(value)
		return t.UnixNano()
	}
	// Most recently updated first; ties keep the FTS order.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && parsed(out[j].updatedAt) > parsed(out[j-1].updatedAt); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	if len(out) > SearchMaxResults {
		out = out[:SearchMaxResults]
	}
	return out, nil
}

// NormalizeSearchQuery collapses whitespace and caps the query at 200
// characters; false means the cleaned query is empty.
func NormalizeSearchQuery(raw string) (string, bool) {
	query := strings.Join(strings.Fields(raw), " ")
	if query == "" {
		return "", false
	}
	runes := []rune(query)
	if len(runes) > SearchMaxQuery {
		runes = runes[:SearchMaxQuery]
	}
	return string(runes), true
}

// SnippetAround cuts ±30 characters around the first case-insensitive match,
// flattening whitespace and adding ellipses on the cut sides.
func SnippetAround(text, query string) (string, bool) {
	if query == "" {
		return "", false
	}
	body := []rune(text)
	lower := []rune(strings.ToLower(text))
	needle := []rune(strings.ToLower(query))
	position := runeIndex(lower, needle)
	if position < 0 {
		return "", false
	}
	start := position - snippetRadius
	if start < 0 {
		start = 0
	}
	end := position + len(needle) + snippetRadius
	if end > len(body) {
		end = len(body)
	}
	excerpt := strings.Join(strings.Fields(string(body[start:end])), " ")
	prefix, suffix := "", ""
	if start > 0 {
		prefix = "…"
	}
	if end < len(body) {
		suffix = "…"
	}
	return prefix + excerpt + suffix, true
}

// fallbackExcerpt returns a usable excerpt when the FTS token match is not a
// contiguous phrase: around the first matching token, else the head.
func fallbackExcerpt(text string, tokens []string) string {
	for _, token := range tokens {
		if snippet, ok := SnippetAround(text, token); ok {
			return snippet
		}
	}
	runes := []rune(strings.Join(strings.Fields(text), " "))
	if len(runes) > 80 {
		return string(runes[:80]) + "…"
	}
	return string(runes)
}

func runeIndex(haystack, needle []rune) int {
	if len(needle) == 0 || len(needle) > len(haystack) {
		return -1
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		matched := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				matched = false
				break
			}
		}
		if matched {
			return i
		}
	}
	return -1
}
