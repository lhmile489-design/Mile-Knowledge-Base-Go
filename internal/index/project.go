package index

import (
	"context"
	"database/sql"
	"sort"
	"strings"
)

// ResolvedLink is one outgoing edge as the UI sees it (API.md M27).
type ResolvedLink struct {
	Target     string  `json:"target"`
	Text       string  `json:"text"`
	Kind       string  `json:"kind"`
	Path       *string `json:"path"`
	Candidates int     `json:"candidates"`
}

// LinkGraph is one document's link view.
type LinkGraph struct {
	Path      string         `json:"path"`
	Outgoing  []ResolvedLink `json:"outgoing"`
	Backlinks []string       `json:"backlinks"`
}

// resolverTx builds a resolver from the documents table inside a transaction.
func resolverTx(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}) (*Resolver, error) {
	rows, err := q.QueryContext(ctx, `SELECT path, title FROM documents WHERE ignored = 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	targets := make([]LinkTarget, 0, 1024)
	for rows.Next() {
		var t LinkTarget
		if err := rows.Scan(&t.Path, &t.Title); err != nil {
			return nil, err
		}
		targets = append(targets, t)
	}
	return NewResolver(targets), rows.Err()
}

// resolverStore builds a resolver from the read pool.
func resolverStore(ctx context.Context, s *Store) (*Resolver, error) {
	return resolverTx(ctx, s.read)
}

// ResolverFor builds a resolver snapshot from the current documents.
func ResolverFor(ctx context.Context, s *Store) (*Resolver, error) {
	return resolverStore(ctx, s)
}

// projectAllLinks rebuilds every edge from the stored bodies. Only non-ignored
// documents participate (PRD FR-LINK: the system area stays out of the graph).
func projectAllLinks(ctx context.Context, s *Store) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		resolver, err := resolverTx(ctx, tx)
		if err != nil {
			return err
		}
		paths, err := allProjectablePaths(ctx, tx)
		if err != nil {
			return err
		}
		return refreshLinksTx(ctx, tx, paths, resolver)
	})
}

func allProjectablePaths(ctx context.Context, tx *sql.Tx) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT d.path FROM documents d
		JOIN doc_body b ON b.path = d.path
		WHERE d.ignored = 0
		ORDER BY d.path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, rows.Err()
}

// refreshLinksTx recomputes the outgoing edges of the given documents.
func refreshLinksTx(ctx context.Context, tx *sql.Tx, paths []string, resolver *Resolver) error {
	for _, path := range paths {
		if _, err := tx.ExecContext(ctx, `DELETE FROM links WHERE from_path = ?`, path); err != nil {
			return err
		}
		var (
			body    sql.NullString
			ignored int
		)
		err := tx.QueryRowContext(ctx, `
			SELECT b.body_text, d.ignored FROM documents d
			LEFT JOIN doc_body b ON b.path = d.path
			WHERE d.path = ?`, path).Scan(&body, &ignored)
		if err == sql.ErrNoRows {
			continue // deleted document: edges are gone, nothing to add
		}
		if err != nil {
			return err
		}
		if ignored == 1 || !body.Valid {
			continue
		}
		seen := map[string]bool{}
		for _, link := range ExtractLinks(body.String) {
			key := link.Target
			if seen[key] {
				continue
			}
			seen[key] = true
			toPath, candidates := resolver.Resolve(link.Target)
			var to any
			if candidates == 1 {
				to = toPath
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO links(from_path, target, kind, to_path, candidates) VALUES(?, ?, ?, ?, ?)
				ON CONFLICT(from_path, target) DO UPDATE SET
				  kind = excluded.kind, to_path = excluded.to_path, candidates = excluded.candidates`,
				path, link.Target, link.Kind, to, candidates); err != nil {
				return err
			}
		}
	}
	return nil
}

// affectedByChange returns the documents whose outgoing edges must be recomputed
// after a batch: the documents themselves plus every document that links at a
// path, file name or title that appeared, changed or disappeared.
func affectedByChange(ctx context.Context, tx *sql.Tx, changed, removed, titles []string) ([]string, error) {
	set := map[string]bool{}
	for _, path := range changed {
		set[path] = true
	}
	for _, path := range removed {
		delete(set, path)
	}

	forms := map[string]bool{}
	addForms := func(path string) {
		lower := strings.ToLower(path)
		forms[lower] = true
		forms[stripExt(lower)] = true
		base := lower
		if at := strings.LastIndex(base, "/"); at >= 0 {
			base = base[at+1:]
		}
		forms[stripExt(base)] = true
	}
	for _, path := range append(append([]string{}, changed...), removed...) {
		addForms(path)
	}

	// Titles (old and new) are what the title layer matches on.
	for _, title := range titles {
		if title = strings.ToLower(strings.TrimSpace(title)); title != "" {
			forms[title] = true
		}
	}

	if len(forms) > 0 {
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(forms)), ",")
		args := make([]any, 0, len(forms)+len(removed))
		args = append(args, anyArgs(forms)...)
		query := `SELECT DISTINCT from_path FROM links WHERE lower(target) IN (` + placeholders + `)`
		if len(removed) > 0 {
			query += ` OR to_path IN (` + strings.TrimSuffix(strings.Repeat("?,", len(removed)), ",") + `)`
			for _, path := range removed {
				args = append(args, path)
			}
		}
		rows, err := tx.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var from string
			if err := rows.Scan(&from); err != nil {
				_ = rows.Close()
				return nil, err
			}
			set[from] = true
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		_ = rows.Close()
	}

	out := make([]string, 0, len(set))
	for path := range set {
		out = append(out, path)
	}
	sort.Strings(out)
	return out, nil
}

func titlesFor(ctx context.Context, tx *sql.Tx, paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(paths)), ",")
	rows, err := tx.QueryContext(ctx, `SELECT title FROM documents WHERE path IN (`+placeholders+`)`,
		strArgs(paths)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var titles []string
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			return nil, err
		}
		titles = append(titles, title)
	}
	return titles, rows.Err()
}

func anyArgs(set map[string]bool) []any {
	out := make([]any, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	return out
}

func strArgs(values []string) []any {
	out := make([]any, len(values))
	for i, value := range values {
		out[i] = value
	}
	return out
}

// Links returns one document's outgoing edges and backlinks (M27) from the
// index. ok=false means the document is not in the index.
func Links(ctx context.Context, s *Store, rel string) (LinkGraph, bool, error) {
	graph := LinkGraph{Path: rel, Outgoing: []ResolvedLink{}, Backlinks: []string{}}

	var exists int
	if err := s.read.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents WHERE path = ?`, rel).Scan(&exists); err != nil {
		return graph, false, err
	}
	if exists == 0 {
		return graph, false, nil
	}

	rows, err := s.read.QueryContext(ctx, `
		SELECT target, kind, to_path, candidates FROM links WHERE from_path = ? ORDER BY rowid`, rel)
	if err != nil {
		return graph, true, err
	}
	for rows.Next() {
		var (
			target, kind string
			toPath       sql.NullString
			candidates   int
		)
		if err := rows.Scan(&target, &kind, &toPath, &candidates); err != nil {
			_ = rows.Close()
			return graph, true, err
		}
		link := ResolvedLink{Target: target, Text: target, Kind: kind, Candidates: candidates}
		if toPath.Valid {
			path := toPath.String
			link.Path = &path
		}
		graph.Outgoing = append(graph.Outgoing, link)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return graph, true, err
	}
	_ = rows.Close()

	backRows, err := s.read.QueryContext(ctx, `
		SELECT DISTINCT from_path FROM links WHERE to_path = ? AND from_path != ? ORDER BY from_path`, rel, rel)
	if err != nil {
		return graph, true, err
	}
	defer backRows.Close()
	for backRows.Next() {
		var from string
		if err := backRows.Scan(&from); err != nil {
			return graph, true, err
		}
		graph.Backlinks = append(graph.Backlinks, from)
	}
	return graph, true, backRows.Err()
}
