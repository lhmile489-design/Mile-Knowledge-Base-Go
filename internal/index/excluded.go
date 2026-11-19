package index

import (
	"context"
	"database/sql"
	"strings"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/vault"
)

// The include-in-search toggle (D-013): a top-level directory with
// indexed=false keeps its documents out of full-text search. Links, backlinks
// and health still see them — those read `ignored`, which this never touches.

// excludedTopLevel maps lower-cased directory names to "out of search".
func excludedTopLevel(categories []vault.Category) map[string]bool {
	out := map[string]bool{}
	for _, category := range categories {
		if !category.Indexed {
			out[strings.ToLower(category.Directory)] = true
		}
	}
	return out
}

// SetSearchExclusions retags every indexed document after a categories save
// (M65). Rows in the system area keep search_excluded = 0: `ignored` already
// hides them, and the toggle must not widen what search sees.
func (s *Store) SetSearchExclusions(ctx context.Context, dirs []string) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`UPDATE documents SET search_excluded = 0 WHERE ignored = 0`); err != nil {
			return err
		}
		for _, dir := range dirs {
			if _, err := tx.ExecContext(ctx, `
				UPDATE documents SET search_excluded = 1
				WHERE ignored = 0
				  AND instr(path, '/') > 0
				  AND lower(substr(path, 1, instr(path, '/') - 1)) = ?`,
				strings.ToLower(dir)); err != nil {
				return err
			}
		}
		return nil
	})
}
