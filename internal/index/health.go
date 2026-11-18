package index

import (
	"context"
	"sort"
	"strings"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// HealthMaxItems caps every list (shared/kbHealth.ts KB_HEALTH_MAX_ITEMS).
const HealthMaxItems = 200

// Health computes the four health lists from the index (PRD FR-HLTH). The
// service layer fills in partial and vaultId.
func Health(ctx context.Context, s *Store) (unresolved []contract.UnresolvedLinkItem, orphans []contract.OrphanItem,
	duplicates []contract.DuplicateNameItem, untitled []contract.UntitledItem, revision int64, err error) {

	if revision, err = s.Revision(ctx); err != nil {
		return
	}

	unresolved, err = unresolvedLinks(ctx, s)
	if err != nil {
		return
	}

	rows, err := s.read.QueryContext(ctx, `
		SELECT d.path, d.title, d.extension, d.ignored, d.body_indexed,
		       NOT EXISTS (SELECT 1 FROM links l WHERE l.from_path = d.path) AS no_out,
		       NOT EXISTS (SELECT 1 FROM links l WHERE l.to_path = d.path) AS no_in
		FROM documents d`)
	if err != nil {
		return
	}
	defer rows.Close()

	byName := map[string][]string{}
	names := make([]string, 0, 64)
	for rows.Next() {
		var (
			path, title, ext string
			ignored, indexed int
			noOut, noIn      bool
		)
		if err := rows.Scan(&path, &title, &ext, &ignored, &indexed, &noOut, &noIn); err != nil {
			return nil, nil, nil, nil, 0, err
		}
		if ignored == 1 {
			continue
		}
		name := path
		if at := strings.LastIndex(name, "/"); at >= 0 {
			name = name[at+1:]
		}
		lowerName := strings.ToLower(name)

		// Duplicates: same file name in different directories.
		if _, ok := byName[lowerName]; !ok {
			names = append(names, lowerName)
		}
		byName[lowerName] = append(byName[lowerName], path)

		// Untitled: the title fell back to the file name (no frontmatter title,
		// no heading).
		if title == name && len(untitled) < HealthMaxItems {
			untitled = append(untitled, contract.UntitledItem{Path: path, FileName: name})
		}

		// Orphans: no edges at all. README/SKILL.md are entry points, binary
		// files and path-only rows have no reliable edge data.
		if lowerName == "readme.md" || lowerName == "skill.md" {
			continue
		}
		if !textExtensions[ext] || indexed != 1 {
			continue
		}
		if noOut && noIn && len(orphans) < HealthMaxItems {
			orphans = append(orphans, contract.OrphanItem{Path: path, Title: title})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, nil, nil, 0, err
	}

	sort.Strings(names)
	for _, name := range names {
		paths := byName[name]
		if len(paths) < 2 {
			continue
		}
		sort.Strings(paths)
		duplicates = append(duplicates, contract.DuplicateNameItem{Name: baseName(paths[0]), Paths: paths})
		if len(duplicates) >= HealthMaxItems {
			break
		}
	}
	return unresolved, orphans, duplicates, untitled, revision, nil
}

// unresolvedLinks lists broken wiki/internal links, capped.
func unresolvedLinks(ctx context.Context, s *Store) ([]contract.UnresolvedLinkItem, error) {
	rows, err := s.read.QueryContext(ctx, `
		SELECT from_path, target, candidates FROM links
		WHERE to_path IS NULL AND kind IN ('wiki', 'internal')
		ORDER BY from_path, target LIMIT ?`, HealthMaxItems)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]contract.UnresolvedLinkItem, 0, 16)
	for rows.Next() {
		var item contract.UnresolvedLinkItem
		if err := rows.Scan(&item.From, &item.Target, &item.Candidates); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func baseName(path string) string {
	if at := strings.LastIndex(path, "/"); at >= 0 {
		return path[at+1:]
	}
	return path
}
