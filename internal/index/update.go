package index

import (
	"context"
	"database/sql"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/pathguard"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/vault"
)

// Update applies the directories reported by the file watcher: files that
// appeared or changed are re-indexed, files that vanished are removed. All
// affected rows are written in a single transaction (P-1, P-7).
func Update(ctx context.Context, s *Store, root string, dirs []string, progress func(Status)) error {
	realRoot, f := canonicalRoot(root)
	if f != nil {
		return &FailureError{Failure: *f}
	}

	selected := normaliseDirs(dirs)
	excluded := excludedTopLevel(vault.ReadCategories(root))

	// The disk side: every file under the selected directories.
	fresh := map[string]entry{}
	for _, dir := range selected {
		seen := map[string]bool{realRoot: true}
		if err := walkVault(realRoot, dir, seen, func(e entry) error {
			fresh[e.rel] = e
			return nil
		}); err != nil {
			return err
		}
	}

	// The index side: rows that live under the selected directories.
	known, err := rowsUnder(ctx, s, selected)
	if err != nil {
		return err
	}

	upserts := make([]entry, 0, len(fresh))
	for rel, e := range fresh {
		old, ok := known[rel]
		if !ok || old.size != e.size || old.mtimeMs != e.mtimeMs {
			upserts = append(upserts, e)
		}
	}
	removed := make([]string, 0, len(known))
	for rel := range known {
		if _, ok := fresh[rel]; !ok {
			removed = append(removed, rel)
		}
	}
	sort.Strings(removed)
	sort.Slice(upserts, func(i, j int) bool { return upserts[i].rel < upserts[j].rel })

	status := Status{Phase: "scanning"}
	if len(upserts) == 0 && len(removed) == 0 {
		// Nothing changed (our own writes land here too). Leave the revision
		// alone so caches keyed on it stay valid.
		status.Phase = "ready"
		if progress != nil {
			progress(status)
		}
		return nil
	}

	if err := s.withTx(ctx, func(tx *sql.Tx) error {
		changed := make([]string, 0, len(upserts))
		for _, e := range upserts {
			changed = append(changed, e.rel)
		}
		// Titles before the writes feed the link layer's title matching.
		oldTitles, err := titlesFor(ctx, tx, append(append([]string{}, changed...), removed...))
		if err != nil {
			return err
		}

		for _, e := range upserts {
			if _, _, err := indexEntry(ctx, tx, e, excluded); err != nil {
				return err
			}
			status.Scanned++
		}
		for _, rel := range removed {
			if err := deleteDocument(ctx, tx, rel); err != nil {
				return err
			}
			status.Skipped++
		}

		// Only documents whose edges can change are recomputed (P-1).
		newTitles, err := titlesFor(ctx, tx, changed)
		if err != nil {
			return err
		}
		resolver, err := resolverTx(ctx, tx)
		if err != nil {
			return err
		}
		affected, err := affectedByChange(ctx, tx, changed, removed, append(oldTitles, newTitles...))
		if err != nil {
			return err
		}
		if err := refreshLinksTx(ctx, tx, affected, resolver); err != nil {
			return err
		}

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
	if err := s.setMeta("token_count", strconv.Itoa(status.TokenCount)); err != nil {
		return err
	}
	if err := s.setMeta("last_built_at", time.Now().UTC().Format(time.RFC3339)); err != nil {
		return err
	}
	status.Phase = "ready"
	status.Revision = revision
	status.LastBuiltAt = time.Now().UTC().Format(time.RFC3339)
	if progress != nil {
		progress(status)
	}
	return nil
}

// normaliseDirs validates the watcher's directory list, drops nested entries
// (walking the outer one covers them) and turns "" into the whole vault.
func normaliseDirs(dirs []string) []string {
	cleaned := make([]string, 0, len(dirs))
	seeAll := false
	seen := map[string]bool{}
	for _, dir := range dirs {
		norm, f := pathguard.ValidateRel(dir)
		if f != nil {
			continue
		}
		if norm == "" {
			seeAll = true
			continue
		}
		if !seen[norm] {
			seen[norm] = true
			cleaned = append(cleaned, norm)
		}
	}
	if seeAll {
		return []string{""}
	}
	sort.Strings(cleaned)
	outer := make([]string, 0, len(cleaned))
	for _, dir := range cleaned {
		nested := false
		for _, kept := range outer {
			if dir == kept || strings.HasPrefix(dir, kept+"/") {
				nested = true
				break
			}
		}
		if !nested {
			outer = append(outer, dir)
		}
	}
	return outer
}

// rowsUnder returns the indexed rows below the selected directories.
func rowsUnder(ctx context.Context, s *Store, dirs []string) (map[string]entry, error) {
	out := map[string]entry{}
	for _, dir := range dirs {
		var (
			rows *sql.Rows
			err  error
		)
		if dir == "" {
			rows, err = s.read.QueryContext(ctx, `SELECT path, size, mtime_ms FROM documents`)
		} else {
			// A range scan on the primary key replaces LIKE (no escaping).
			rows, err = s.read.QueryContext(ctx,
				`SELECT path, size, mtime_ms FROM documents WHERE path = ? OR (path >= ? AND path < ?)`,
				dir, dir+"/", dir+"0")
		}
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var e entry
			if err := rows.Scan(&e.rel, &e.size, &e.mtimeMs); err != nil {
				_ = rows.Close()
				return nil, err
			}
			out[e.rel] = e
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		_ = rows.Close()
	}
	return out, nil
}

// deleteDocument removes a document and everything derived from it.
func deleteDocument(ctx context.Context, tx *sql.Tx, rel string) error {
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM doc_fts WHERE rowid IN (SELECT rowid FROM documents WHERE path = ?)`, rel); err != nil {
		return err
	}
	for _, stmt := range []string{
		`DELETE FROM documents WHERE path = ?`,
		`DELETE FROM doc_body WHERE path = ?`,
		`DELETE FROM links WHERE from_path = ?`,
	} {
		if _, err := tx.ExecContext(ctx, stmt, rel); err != nil {
			return err
		}
	}
	return nil
}

// PathExists reports whether the index knows this document.
func (s *Store) PathExists(ctx context.Context, rel string) (bool, error) {
	var n int
	err := s.read.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents WHERE path = ?`, rel).Scan(&n)
	return n > 0, err
}

// BodyText returns the stored body copy used for excerpts.
func (s *Store) BodyText(ctx context.Context, rel string) (string, bool, error) {
	var body string
	err := s.read.QueryRowContext(ctx, `SELECT body_text FROM doc_body WHERE path = ?`, rel).Scan(&body)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return body, true, nil
}

// DocumentMeta is the per-document row the search layer needs.
type DocumentMeta struct {
	Path       string
	Title      string
	Area       string
	Tags       string
	Extension  string
	MtimeMs    int64
	Size       int64
	Ignored    bool
	BodyExists bool
}

// Meta returns one document's row.
func (s *Store) Meta(ctx context.Context, rel string) (DocumentMeta, bool, error) {
	var m DocumentMeta
	var ignored, bodyIndexed int
	err := s.read.QueryRowContext(ctx, `
		SELECT path, title, area, tags, extension, mtime_ms, size, ignored, body_indexed
		FROM documents WHERE path = ?`, rel).
		Scan(&m.Path, &m.Title, &m.Area, &m.Tags, &m.Extension, &m.MtimeMs, &m.Size, &ignored, &bodyIndexed)
	if err == sql.ErrNoRows {
		return DocumentMeta{}, false, nil
	}
	if err != nil {
		return DocumentMeta{}, false, err
	}
	m.Ignored = ignored == 1
	m.BodyExists = bodyIndexed == 1
	return m, true, nil
}

// FTSQuery runs a raw FTS5 MATCH and returns the matching paths with their
// bm25 rank; used by the search layer and by tests.
func (s *Store) FTSQuery(ctx context.Context, match string, limit int) ([]string, error) {
	rows, err := s.read.QueryContext(ctx, `
		SELECT d.path FROM doc_fts JOIN documents d ON d.rowid = doc_fts.rowid
		WHERE doc_fts MATCH ? ORDER BY bm25(doc_fts, 12, 8, 6, 4, 1) LIMIT ?`, match, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		out = append(out, path)
	}
	return out, rows.Err()
}
