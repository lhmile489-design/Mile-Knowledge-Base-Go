package index

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/pathguard"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/vault"
)

// MaxIndexedBytes is the size above which a file is indexed by path only
// (PRD FR-SRCH).
const MaxIndexedBytes = 2 << 20

// textExtensions are the extensions whose body is read (PRD FR-SRCH).
var textExtensions = map[string]bool{
	"md": true, "markdown": true, "txt": true, "json": true, "yaml": true,
	"yml": true, "toml": true, "ts": true, "tsx": true, "js": true, "jsx": true,
	"py": true, "go": true, "java": true, "rs": true, "sh": true, "css": true,
	"html": true,
}

// neverIndex names directories that are skipped entirely, at any depth.
var neverIndex = map[string]bool{"node_modules": true, ".git": true, ".mile-kb": true}

// BatchSize is how many files one write transaction covers during a build.
const BatchSize = 500

// Status is the state API.md M25 reports and E02 pushes.
type Status struct {
	Phase         string `json:"phase"` // idle | scanning | ready | error
	DocumentCount int    `json:"documentCount"`
	TokenCount    int    `json:"tokenCount"`
	Scanned       int    `json:"scanned"`
	Skipped       int    `json:"skipped"`
	Revision      int64  `json:"revision"`
	LastBuiltAt   string `json:"lastBuiltAt,omitempty"`
	ErrorMessage  string `json:"errorMessage,omitempty"`
}

// FailureError carries a contract failure out of the index package; the service
// layer unwraps it into the binding's failure shape.
type FailureError struct{ Failure contract.Failure }

func (e *FailureError) Error() string { return e.Failure.Message }

type entry struct {
	rel     string // canonical vault-relative path
	abs     string
	size    int64
	mtimeMs int64
}

// Build scans the whole vault and rebuilds the index from scratch. It is meant
// to run in a background goroutine; progress is called after every batch.
func Build(ctx context.Context, s *Store, root string, progress func(Status)) error {
	realRoot, f := canonicalRoot(root)
	if f != nil {
		return &FailureError{Failure: *f}
	}

	if err := s.withTx(ctx, func(tx *sql.Tx) error {
		for _, stmt := range []string{
			`DELETE FROM doc_fts`, `DELETE FROM doc_body`, `DELETE FROM documents`,
			`DELETE FROM links`, `DELETE FROM meta WHERE key IN ('token_count','last_built_at')`,
		} {
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}

	status := Status{Phase: "scanning"}
	batch := make([]entry, 0, BatchSize)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := s.withTx(ctx, func(tx *sql.Tx) error {
			for _, e := range batch {
				_, skipped, err := indexEntry(ctx, tx, e)
				if err != nil {
					return err
				}
				status.Scanned++
				if skipped {
					status.Skipped++
				}
			}
			return nil
		}); err != nil {
			return err
		}
		batch = batch[:0]
		if progress != nil {
			progress(status)
		}
		return nil
	}

	seen := map[string]bool{realRoot: true}
	emitErr := walkVault(realRoot, "", seen, func(e entry) error {
		batch = append(batch, e)
		if len(batch) >= BatchSize {
			return flush()
		}
		return nil
	})
	if emitErr != nil {
		return emitErr
	}
	if err := flush(); err != nil {
		return err
	}

	// Link edges need the whole document set, so they are projected after the
	// scan rather than per batch.
	if err := projectAllLinks(ctx, s); err != nil {
		return err
	}

	// Counts come from the table so a skipped directory cannot make them lie.
	if err := s.withTx(ctx, func(tx *sql.Tx) error {
		var docs, tokens int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*), COALESCE(SUM(token_count), 0) FROM documents`).Scan(&docs, &tokens); err != nil {
			return err
		}
		status.DocumentCount = docs
		status.TokenCount = tokens
		return nil
	}); err != nil {
		return err
	}

	revision, err := s.bumpRevision()
	if err != nil {
		return err
	}
	builtAt := time.Now().UTC().Format(time.RFC3339)
	if err := s.setMeta("token_count", strconv.Itoa(status.TokenCount)); err != nil {
		return err
	}
	if err := s.setMeta("last_built_at", builtAt); err != nil {
		return err
	}
	status.Phase = "ready"
	status.Revision = revision
	status.LastBuiltAt = builtAt
	if progress != nil {
		progress(status)
	}
	return nil
}

// canonicalRoot resolves the vault root to its real path.
func canonicalRoot(root string) (string, *contract.Failure) {
	real, err := pathguard.Canon(root)
	if err != nil {
		return "", contract.Fail(contract.CodeVaultUnavailable, "知识库根目录不可用")
	}
	return real, nil
}

// walkVault walks the vault, skipping never-indexed directories and refusing
// paths that leave the root through a junction (every entry is resolved).
func walkVault(realRoot, rel string, seen map[string]bool, emit func(entry) error) error {
	dir := realRoot
	if rel != "" {
		dir = filepath.Join(realRoot, filepath.FromSlash(rel))
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // unreadable directory: skip it, do not fail the build
	}
	for _, de := range entries {
		name := de.Name()
		if neverIndex[name] {
			continue
		}
		childRel := name
		if rel != "" {
			childRel = rel + "/" + name
		}
		abs, f := pathguard.Resolve(realRoot, childRel)
		if f != nil {
			continue
		}
		info, err := os.Lstat(abs)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeIrregular != 0 {
			continue
		}
		if info.IsDir() {
			if seen[abs] {
				continue
			}
			seen[abs] = true
			if err := walkVault(realRoot, childRel, seen, emit); err != nil {
				return err
			}
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		canonicalRel, err := filepath.Rel(realRoot, abs)
		if err != nil {
			continue
		}
		if err := emit(entry{
			rel:     filepath.ToSlash(canonicalRel),
			abs:     abs,
			size:    info.Size(),
			mtimeMs: info.ModTime().UnixMilli(),
		}); err != nil {
			return err
		}
	}
	return nil
}

// indexEntry writes one document and returns its token count. skipped counts
// files whose body was not read (binary, invalid UTF-8 or oversize).
func indexEntry(ctx context.Context, tx *sql.Tx, e entry) (tokens int, skipped bool, err error) {
	top := e.rel
	if at := strings.IndexByte(top, '/'); at >= 0 {
		top = top[:at]
	}
	area := vault.AreaFor(top)
	ignored := area == vault.AreaSystem

	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(e.rel), "."))
	name := e.rel
	if at := strings.LastIndex(name, "/"); at >= 0 {
		name = name[at+1:]
	}

	title := name
	var tags, headings, body string
	bodyIndexed := false

	if textExtensions[ext] && e.size <= MaxIndexedBytes {
		raw, readErr := os.ReadFile(e.abs)
		if readErr == nil && utf8.Valid(raw) {
			parsed := ParseDocument(e.rel, string(raw))
			title = parsed.Title
			tags = strings.Join(parsed.Tags, ",")
			headings = strings.Join(parsed.Headings, "\n")
			body = parsed.Body
			bodyIndexed = true
		} else {
			skipped = true
		}
	} else if e.size > MaxIndexedBytes {
		skipped = true
	}

	titleTokens := Tokenize(title)
	tagTokens := Tokenize(strings.ReplaceAll(tags, ",", " "))
	headingTokens := Tokenize(headings)
	pathTokens := TokenizePath(e.rel)
	bodyTokens := Tokenize(body)
	tokens = len(titleTokens) + len(tagTokens) + len(headingTokens) + len(pathTokens) + len(bodyTokens)

	var rowid int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO documents(path, title, area, tags, extension, mtime_ms, size, ignored, body_indexed, updated_at, token_count)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
		  title = excluded.title, area = excluded.area, tags = excluded.tags,
		  extension = excluded.extension, mtime_ms = excluded.mtime_ms, size = excluded.size,
		  ignored = excluded.ignored, body_indexed = excluded.body_indexed,
		  updated_at = excluded.updated_at, token_count = excluded.token_count
		RETURNING rowid`,
		e.rel, title, area, tags, ext, e.mtimeMs, e.size, boolInt(ignored), boolInt(bodyIndexed),
		time.Now().UTC().Format(time.RFC3339), tokens).Scan(&rowid)
	if err != nil {
		return 0, false, err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM doc_fts WHERE rowid = ?`, rowid); err != nil {
		return 0, false, err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO doc_fts(rowid, title, tags, headings, path, body) VALUES(?, ?, ?, ?, ?, ?)`,
		rowid, joinTokens(titleTokens), joinTokens(tagTokens), joinTokens(headingTokens),
		joinTokens(pathTokens), joinTokens(bodyTokens)); err != nil {
		return 0, false, err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM doc_body WHERE path = ?`, e.rel); err != nil {
		return 0, false, err
	}
	if bodyIndexed && body != "" {
		if _, err := tx.ExecContext(ctx, `INSERT INTO doc_body(path, body_text) VALUES(?, ?)`, e.rel, body); err != nil {
			return 0, false, err
		}
	}
	// Link edges are rebuilt by the link pass (T04); a stale edge would point at
	// text the document no longer contains.
	if _, err := tx.ExecContext(ctx, `DELETE FROM links WHERE from_path = ?`, e.rel); err != nil {
		return 0, false, err
	}
	return tokens, skipped, nil
}

func joinTokens(tokens []string) string { return strings.Join(tokens, " ") }

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
