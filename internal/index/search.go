package index

import (
	"context"
	"database/sql"
	"sort"
	"strings"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// SearchLimits (PRD FR-SRCH).
const (
	DefaultSearchLimit = 50
	MaxSearchLimit     = 200
	MaxQueryLength     = 200
	// maxCandidates caps how many FTS rows are inspected in Go; the ranking
	// rules (all tokens matched first) need the field columns, which FTS5
	// cannot rank on directly.
	maxCandidates = 2000
)

// Search runs M24 against one vault's index.
func Search(ctx context.Context, s *Store, q contract.SearchQuery) (contract.SearchResult, error) {
	started := time.Now()
	result := contract.SearchResult{VaultID: q.VaultID, Hits: []contract.SearchHit{}}

	text := strings.TrimSpace(q.Text)
	if r := []rune(text); len(r) > MaxQueryLength {
		text = string(r[:MaxQueryLength])
	}
	result.Query = text
	tokens := Tokenize(text)
	if len(tokens) == 0 {
		result.TookMs = time.Since(started).Milliseconds()
		return result, nil
	}

	limit := q.Limit
	if limit <= 0 {
		limit = DefaultSearchLimit
	}
	if limit > MaxSearchLimit {
		limit = MaxSearchLimit
	}

	filters := contract.SearchFilters{}
	if q.Filters != nil {
		filters = *q.Filters
	}
	where, args := searchWhere(tokens, filters)
	rows, err := s.read.QueryContext(ctx, `
		SELECT d.path, d.title, d.area, d.tags, d.mtime_ms, d.size, d.ignored,
		       doc_fts.title, doc_fts.tags, doc_fts.headings, doc_fts.path, doc_fts.body,
		       bm25(doc_fts, 12, 8, 6, 4, 1) AS rank,
		       b.body_text
		FROM doc_fts
		JOIN documents d ON d.rowid = doc_fts.rowid
		LEFT JOIN doc_body b ON b.path = d.path
		WHERE `+where+`
		ORDER BY rank
		LIMIT ?`, append(args, maxCandidates)...)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	candidates := make([]candidate, 0, 64)
	tagFilter := normaliseTags(filters.Tags)

	for rows.Next() {
		var (
			c         candidate
			ignored   int
			fTitle    string
			fTags     string
			fHeadings string
			fPath     string
			fBody     sql.NullString
			bBody     sql.NullString
			rawTags   string
		)
		if err := rows.Scan(&c.hit.Path, &c.hit.Title, &c.hit.Area, &rawTags, &c.hit.MtimeMs,
			&c.hit.Size, &ignored, &fTitle, &fTags, &fHeadings, &fPath, &fBody, &c.rank, &bBody); err != nil {
			return result, err
		}
		c.hit.Ignored = ignored == 1
		c.hit.Tags = splitTags(rawTags)
		if !tagsPass(c.hit.Tags, tagFilter) {
			continue
		}
		bodyText := ""
		if fBody.Valid {
			bodyText = fBody.String
		}
		c.hit.MatchedFields, c.fullMatch = matchedFields(tokens, map[string]string{
			"title": fTitle, "tag": fTags, "heading": fHeadings, "path": fPath, "body": bodyText,
		})
		if bBody.Valid {
			c.body = bBody.String
		}
		c.hit.Score = -c.rank
		c.hit.Excerpt = BuildExcerpt(c.body, tokens, 60)
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return result, err
	}

	sortCandidates(candidates, q.Sort)
	result.Total = len(candidates)
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	for _, c := range candidates {
		result.Hits = append(result.Hits, c.hit)
	}
	result.TookMs = time.Since(started).Milliseconds()
	return result, nil
}

// searchWhere builds the MATCH plus filters. Every token needs the "other"
// recall rule from PRD FR-SRCH: any token may match (OR).
func searchWhere(tokens []string, filters contract.SearchFilters) (string, []any) {
	quoted := make([]string, len(tokens))
	for i, token := range tokens {
		quoted[i] = `"` + strings.ReplaceAll(token, `"`, `""`) + `"`
	}
	where := `doc_fts MATCH ?`
	args := []any{strings.Join(quoted, " OR ")}

	// A directory the user turned out of search never matches, whatever the
	// other filters say (D-013).
	where += ` AND d.search_excluded = 0`
	if !filters.IncludeSystem {
		where += ` AND d.ignored = 0`
	}
	if len(filters.Areas) > 0 {
		where += ` AND d.area IN (` + placeholders(len(filters.Areas)) + `)`
		for _, area := range filters.Areas {
			args = append(args, area)
		}
	}
	if len(filters.Extensions) > 0 {
		where += ` AND d.extension IN (` + placeholders(len(filters.Extensions)) + `)`
		for _, ext := range filters.Extensions {
			args = append(args, strings.ToLower(strings.TrimPrefix(ext, ".")))
		}
	}
	if filters.UpdatedAfter > 0 {
		where += ` AND d.mtime_ms >= ?`
		args = append(args, filters.UpdatedAfter)
	}
	return where, args
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func normaliseTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag != "" {
			out = append(out, tag)
		}
	}
	return out
}

func splitTags(raw string) []string {
	if raw == "" {
		return []string{}
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// tagsPass applies the OR semantics of filters.tags.
func tagsPass(hitTags, filter []string) bool {
	if len(filter) == 0 {
		return true
	}
	for _, tag := range hitTags {
		lower := strings.ToLower(tag)
		for _, want := range filter {
			if lower == want {
				return true
			}
		}
	}
	return false
}

// matchedFields reports which fields contain a query token, in display order,
// and whether every token was found somewhere (the legacy x2 boost becomes
// "full matches first").
func matchedFields(tokens []string, columns map[string]string) ([]string, bool) {
	sets := map[string]map[string]bool{}
	for field, raw := range columns {
		set := map[string]bool{}
		for _, token := range strings.Fields(raw) {
			set[token] = true
		}
		sets[field] = set
	}
	found := map[string]bool{}
	fields := make([]string, 0, 5)
	for _, field := range []string{"title", "heading", "tag", "path", "body"} {
		set := sets[field]
		hit := false
		for _, token := range tokens {
			if set[token] {
				hit = true
				found[token] = true
			}
		}
		if hit {
			fields = append(fields, field)
		}
	}
	return fields, len(found) == len(tokens)
}

type candidate struct {
	hit       contract.SearchHit
	rank      float64
	fullMatch bool
	body      string
}

// sortCandidates applies the requested order. "relevance" puts documents that
// matched every token first (the legacy x2 rule), then bm25, then path.
func sortCandidates(candidates []candidate, mode string) {
	switch mode {
	case "updated":
		sort.SliceStable(candidates, func(i, j int) bool {
			if candidates[i].hit.MtimeMs != candidates[j].hit.MtimeMs {
				return candidates[i].hit.MtimeMs > candidates[j].hit.MtimeMs
			}
			return candidates[i].hit.Path < candidates[j].hit.Path
		})
	case "path":
		sort.SliceStable(candidates, func(i, j int) bool {
			return candidates[i].hit.Path < candidates[j].hit.Path
		})
	default: // relevance
		sort.SliceStable(candidates, func(i, j int) bool {
			a, b := candidates[i], candidates[j]
			if a.fullMatch != b.fullMatch {
				return a.fullMatch
			}
			if a.rank != b.rank {
				return a.rank < b.rank
			}
			return a.hit.Path < b.hit.Path
		})
	}
}
