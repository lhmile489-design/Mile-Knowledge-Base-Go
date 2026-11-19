package index

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	_ "modernc.org/sqlite"
)

// SchemaVersion is the version this build understands. A database written by a
// newer build (or an unreadable one) is deleted and rebuilt: the index is
// derived data (docs/数据库设计.md §4.5, §8).
const SchemaVersion = 2

//go:embed migrations/*.sql
var migrations embed.FS

// ErrSchemaMismatch reports a database this build cannot use as-is.
var ErrSchemaMismatch = errors.New("index schema version mismatch")

// Store is one vault's SQLite index: a single-connection writer (SQLite allows
// one writer) plus a read pool, both in WAL mode.
type Store struct {
	path  string
	write *sql.DB
	read  *sql.DB
	mu    sync.Mutex // serialises write transactions
}

// DSN is the documented connection string (kb-index-sqlite skill §2).
func dsn(path string) string {
	return "file:" + filepath.ToSlash(path) +
		"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)" +
		"&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)"
}

// OpenIndex opens (creating if needed) the index database at path.
func OpenIndex(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	s, err := openAt(path)
	if err == nil {
		return s, nil
	}
	if !errors.Is(err, ErrSchemaMismatch) {
		return nil, err
	}
	// Derived data: drop the file and rebuild rather than migrate.
	removeDatabase(path)
	return openAt(path)
}

func openAt(path string) (*Store, error) {
	write, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, err
	}
	write.SetMaxOpenConns(1)
	read, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		_ = write.Close()
		return nil, err
	}
	s := &Store{path: path, write: write, read: read}
	if err := s.migrate(); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	if _, err := s.write.Exec(string(mustReadMigration("001_init.sql"))); err != nil {
		return fmt.Errorf("apply 001_init.sql: %w", err)
	}
	current, ok, err := s.metaValue("schema_version")
	if err != nil {
		return err
	}
	if !ok {
		return s.setMeta("schema_version", strconv.Itoa(SchemaVersion))
	}
	version, err := strconv.Atoi(current)
	if err != nil || version != SchemaVersion {
		return ErrSchemaMismatch
	}
	return nil
}

func mustReadMigration(name string) []byte {
	body, err := migrations.ReadFile("migrations/" + name)
	if err != nil {
		panic("embedded migration missing: " + name)
	}
	return body
}

// removeDatabase deletes the database and its WAL side files.
func removeDatabase(path string) {
	_ = os.Remove(path)
	_ = os.Remove(path + "-wal")
	_ = os.Remove(path + "-shm")
}

// Close releases both pools.
func (s *Store) Close() error {
	var first error
	if s.write != nil {
		first = s.write.Close()
	}
	if s.read != nil {
		if err := s.read.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// Path returns the database file path.
func (s *Store) Path() string { return s.path }

// withTx runs fn inside one write transaction. Transactions are serialised, so
// a batch of changes commits once (P-1, P-7).
func (s *Store) withTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) metaValue(key string) (string, bool, error) {
	var value string
	err := s.read.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func (s *Store) setMeta(key, value string) error {
	_, err := s.write.Exec(
		`INSERT INTO meta(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value)
	return err
}

// revision returns meta.index_revision, creating it on first use.
func (s *Store) revision() (int64, error) {
	value, ok, err := s.metaValue("index_revision")
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, s.setMeta("index_revision", "0")
	}
	return strconv.ParseInt(value, 10, 64)
}

// bumpRevision increments meta.index_revision and returns the new value.
func (s *Store) bumpRevision() (int64, error) {
	current, err := s.revision()
	if err != nil {
		return 0, err
	}
	next := current + 1
	return next, s.setMeta("index_revision", strconv.FormatInt(next, 10))
}

// DocumentCount reports how many documents the index holds.
// SkillDocument is the row the skill catalog needs from the index.
type SkillDocument struct {
	Path  string
	Title string
}

// SkillDocuments lists every indexed skill file. The caller applies the depth
// and name rules (BuildSkillCatalog), so a single query is enough.
func (s *Store) SkillDocuments(ctx context.Context) ([]SkillDocument, error) {
	rows, err := s.read.QueryContext(ctx,
		`SELECT path, title FROM documents WHERE area = 'skills' AND lower(path) LIKE '%/skill.md' ORDER BY path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SkillDocument
	for rows.Next() {
		var doc SkillDocument
		if err := rows.Scan(&doc.Path, &doc.Title); err != nil {
			return nil, err
		}
		out = append(out, doc)
	}
	return out, rows.Err()
}

func (s *Store) DocumentCount(ctx context.Context) (int, error) {
	var n int
	err := s.read.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents`).Scan(&n)
	return n, err
}

// Revision reports the current index revision.
func (s *Store) Revision(ctx context.Context) (int64, error) {
	return s.revision()
}
