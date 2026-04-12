package evidence

// tag_repository.go isolates the Sprint 9 tag-taxonomy SQL from the main
// repository file (repository.go was pushing past the 1000-line ceiling).
// These methods still hang off *PGRepository so the repository interface
// and its consumers are unchanged.

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ListDistinctTags returns distinct lowercased tags for a case that share a
// common prefix with `prefix`, capped at `limit`. Draws only from non-destroyed
// evidence. Results are ordered alphabetically. (Sprint 9 Step 5.)
func (r *PGRepository) ListDistinctTags(ctx context.Context, caseID uuid.UUID, prefix string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = MaxTagAutocompleteLimit
	}
	query := `
		SELECT DISTINCT lower(tag) AS t
		FROM (
			SELECT unnest(tags) AS tag
			FROM evidence_items
			WHERE case_id = $1 AND destroyed_at IS NULL
		) s
		WHERE lower(tag) LIKE $2 ESCAPE '\'
		ORDER BY t
		LIMIT $3`

	pattern := escapeLikePattern(prefix) + "%"
	rows, err := r.pool.Query(ctx, query, caseID, pattern, limit)
	if err != nil {
		return nil, fmt.Errorf("list distinct tags: %w", err)
	}
	defer rows.Close()

	tags := make([]string, 0, limit)
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, fmt.Errorf("scan distinct tag: %w", err)
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// tagAdvisoryLockID derives a stable 64-bit advisory lock key from a case
// UUID, namespaced to tag operations. Different from the evidence-counter
// lock so tag mutations don't contend with uploads.
func tagAdvisoryLockID(caseID uuid.UUID) int64 {
	id := int64(caseID[0])<<56 | int64(caseID[1])<<48 | int64(caseID[2])<<40 |
		int64(caseID[3])<<32 | int64(caseID[4])<<24 | int64(caseID[5])<<16 |
		int64(caseID[6])<<8 | int64(caseID[7])
	return id ^ 0x5441_475f // "TAG_"
}

// withCaseTagLock runs fn inside a transaction holding a case-scoped
// advisory lock, so concurrent tag rename/merge/delete operations on the
// same case serialise cleanly without a full table lock.
func (r *PGRepository) withCaseTagLock(ctx context.Context, caseID uuid.UUID, fn func(tx pgx.Tx) (int64, error)) (int64, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("begin tag tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, tagAdvisoryLockID(caseID)); err != nil {
		return 0, fmt.Errorf("acquire tag lock: %w", err)
	}

	n, err := fn(tx)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit tag tx: %w", err)
	}
	return n, nil
}

// RenameTagInCase replaces every occurrence of oldTag with newTag across all
// non-destroyed evidence items in the case, under a case-scoped advisory
// lock so concurrent renames cannot interleave.
func (r *PGRepository) RenameTagInCase(ctx context.Context, caseID uuid.UUID, oldTag, newTag string) (int64, error) {
	return r.withCaseTagLock(ctx, caseID, func(tx pgx.Tx) (int64, error) {
		tag, err := tx.Exec(ctx,
			`UPDATE evidence_items
			 SET tags = array_replace(tags, $2, $3)
			 WHERE case_id = $1
			   AND destroyed_at IS NULL
			   AND $2 = ANY(tags)`,
			caseID, oldTag, newTag)
		if err != nil {
			return 0, fmt.Errorf("rename tag in case: %w", err)
		}
		return tag.RowsAffected(), nil
	})
}

// MergeTagsInCase strips every tag in `sources` and appends `target` (deduped)
// on every non-destroyed evidence item in the case that currently carries at
// least one source. Held under a case-scoped advisory lock for atomicity
// across rows.
func (r *PGRepository) MergeTagsInCase(ctx context.Context, caseID uuid.UUID, sources []string, target string) (int64, error) {
	if len(sources) == 0 {
		return 0, nil
	}
	return r.withCaseTagLock(ctx, caseID, func(tx pgx.Tx) (int64, error) {
		ct, err := tx.Exec(ctx,
			`UPDATE evidence_items
			 SET tags = (
				SELECT COALESCE(array_agg(DISTINCT t ORDER BY t), '{}'::text[])
				FROM unnest(tags || ARRAY[$3]::text[]) AS t
				WHERE NOT (t = ANY($2::text[]))
			 )
			 WHERE case_id = $1
			   AND destroyed_at IS NULL
			   AND tags && $2::text[]`,
			caseID, sources, target)
		if err != nil {
			return 0, fmt.Errorf("merge tags in case: %w", err)
		}
		return ct.RowsAffected(), nil
	})
}

// DeleteTagFromCase removes `tag` from the tags array of every non-destroyed
// evidence item in the case. Held under the case-scoped advisory lock.
func (r *PGRepository) DeleteTagFromCase(ctx context.Context, caseID uuid.UUID, tag string) (int64, error) {
	return r.withCaseTagLock(ctx, caseID, func(tx pgx.Tx) (int64, error) {
		ct, err := tx.Exec(ctx,
			`UPDATE evidence_items
			 SET tags = array_remove(tags, $2)
			 WHERE case_id = $1
			   AND destroyed_at IS NULL
			   AND $2 = ANY(tags)`,
			caseID, tag)
		if err != nil {
			return 0, fmt.Errorf("delete tag from case: %w", err)
		}
		return ct.RowsAffected(), nil
	})
}

// escapeLikePattern escapes %, _, and \ in a LIKE pattern so callers can treat
// input as a literal string prefix.
func escapeLikePattern(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "%", `\%`)
	s = strings.ReplaceAll(s, "_", `\_`)
	return s
}
