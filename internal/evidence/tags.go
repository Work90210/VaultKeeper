package evidence

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Tag validation constants. MaxTagCount and MaxTagLength are declared in
// models.go (set to 50 and 100 respectively, matching the Sprint 9 spec).
const (
	// MaxTagAutocompleteLimit caps the number of suggestions returned from
	// AutocompleteTags so that the API cannot be coerced into returning a
	// huge result set.
	MaxTagAutocompleteLimit = 20
)

// caseCustodyRecorder is an optional extension of CustodyRecorder for recording
// case-level (evidence-less) custody events. The concrete *custody.Logger
// implements it. The Service obtains a value by type-asserting s.custody, so
// existing mock implementations that only satisfy CustodyRecorder continue to
// work unchanged.
type caseCustodyRecorder interface {
	RecordCaseEvent(ctx context.Context, caseID uuid.UUID, action, actorUserID string, detail map[string]string) error
}

// ValidateTag checks a single tag against the Sprint 9 rules:
//   - not empty (after trimming whitespace)
//   - no longer than MaxTagLength once lowercased
//   - may only contain lowercase ASCII letters, digits, hyphens, and underscores
//
// ValidateTag is case-insensitive: callers may pass mixed-case input, but the
// value is lowercased before validation so that the canonical form is what is
// actually checked.
func ValidateTag(tag string) error {
	trimmed := strings.TrimSpace(tag)
	if trimmed == "" {
		return &ValidationError{Field: "tag", Message: "tag must not be empty"}
	}
	normalized := strings.ToLower(trimmed)
	if len(normalized) > MaxTagLength {
		return &ValidationError{Field: "tag", Message: fmt.Sprintf("tag exceeds %d characters", MaxTagLength)}
	}
	for _, r := range normalized {
		if !isTagRune(r) {
			return &ValidationError{
				Field:   "tag",
				Message: "tag may only contain alphanumeric characters, hyphens, and underscores",
			}
		}
	}
	return nil
}

func isTagRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z':
		return true
	case r >= '0' && r <= '9':
		return true
	case r == '-' || r == '_':
		return true
	}
	return false
}

// NormalizeTags trims, lowercases, deduplicates, and validates a list of tags,
// returning the canonical form. It rejects the list if any tag fails validation
// or if the deduplicated count exceeds MaxTagCount.
func NormalizeTags(tags []string) ([]string, error) {
	if len(tags) == 0 {
		return []string{}, nil
	}

	seen := make(map[string]struct{}, len(tags))
	result := make([]string, 0, len(tags))

	for _, raw := range tags {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return nil, &ValidationError{Field: "tags", Message: "tag must not be empty"}
		}
		normalized := strings.ToLower(trimmed)
		if err := ValidateTag(normalized); err != nil {
			return nil, err
		}
		if _, dup := seen[normalized]; dup {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}

	if len(result) > MaxTagCount {
		return nil, &ValidationError{
			Field:   "tags",
			Message: fmt.Sprintf("too many tags (max %d)", MaxTagCount),
		}
	}
	return result, nil
}

// AutocompleteTags returns up to `limit` distinct tags for a case that share a
// common case-insensitive prefix with `query`. Results are drawn only from
// non-destroyed evidence. `limit` is clamped to MaxTagAutocompleteLimit.
func (s *Service) AutocompleteTags(ctx context.Context, caseID uuid.UUID, query string, limit int) ([]string, error) {
	if caseID == uuid.Nil {
		return nil, &ValidationError{Field: "case_id", Message: "case ID is required"}
	}
	if limit <= 0 || limit > MaxTagAutocompleteLimit {
		limit = MaxTagAutocompleteLimit
	}
	prefix := strings.ToLower(strings.TrimSpace(query))
	tags, err := s.repo.ListDistinctTags(ctx, caseID, prefix, limit)
	if err != nil {
		return nil, fmt.Errorf("list distinct tags: %w", err)
	}
	if tags == nil {
		tags = []string{}
	}
	return tags, nil
}

// RenameTag rewrites every occurrence of oldTag to newTag across all
// non-destroyed evidence items in a case. It records a single case-level
// custody event summarising the operation. It returns the number of rows
// affected.
func (s *Service) RenameTag(ctx context.Context, caseID uuid.UUID, oldTag, newTag, actorID string) (int, error) {
	if caseID == uuid.Nil {
		return 0, &ValidationError{Field: "case_id", Message: "case ID is required"}
	}
	if err := ValidateTag(oldTag); err != nil {
		return 0, err
	}
	if err := ValidateTag(newTag); err != nil {
		return 0, err
	}
	oldNormalized := strings.ToLower(strings.TrimSpace(oldTag))
	newNormalized := strings.ToLower(strings.TrimSpace(newTag))
	if oldNormalized == newNormalized {
		return 0, &ValidationError{Field: "new", Message: "new tag must differ from old tag"}
	}

	count, err := s.repo.RenameTagInCase(ctx, caseID, oldNormalized, newNormalized)
	if err != nil {
		return 0, fmt.Errorf("rename tag in case: %w", err)
	}

	s.recordCaseCustodyEvent(ctx, caseID, "tag_renamed", actorID, map[string]string{
		"old":   oldNormalized,
		"new":   newNormalized,
		"count": fmt.Sprintf("%d", count),
	})

	return int(count), nil
}

// MergeTags removes every tag in `sources` from all evidence in a case and
// appends `target` (deduped) in one atomic transaction. Records a single
// case-level custody event.
func (s *Service) MergeTags(ctx context.Context, caseID uuid.UUID, sources []string, target, actorID string) (int, error) {
	if caseID == uuid.Nil {
		return 0, &ValidationError{Field: "case_id", Message: "case ID is required"}
	}
	if len(sources) == 0 {
		return 0, &ValidationError{Field: "sources", Message: "at least one source tag is required"}
	}
	if err := ValidateTag(target); err != nil {
		return 0, err
	}
	targetNormalized := strings.ToLower(strings.TrimSpace(target))

	normalizedSources := make([]string, 0, len(sources))
	seen := make(map[string]struct{}, len(sources))
	for _, src := range sources {
		if err := ValidateTag(src); err != nil {
			return 0, err
		}
		n := strings.ToLower(strings.TrimSpace(src))
		if n == targetNormalized {
			// Collapsing a source onto itself is a no-op but not an error;
			// skip so we don't thrash the array.
			continue
		}
		if _, dup := seen[n]; dup {
			continue
		}
		seen[n] = struct{}{}
		normalizedSources = append(normalizedSources, n)
	}
	if len(normalizedSources) == 0 {
		return 0, nil
	}

	count, err := s.repo.MergeTagsInCase(ctx, caseID, normalizedSources, targetNormalized)
	if err != nil {
		return 0, fmt.Errorf("merge tags in case: %w", err)
	}

	s.recordCaseCustodyEvent(ctx, caseID, "tags_merged", actorID, map[string]string{
		"sources": strings.Join(normalizedSources, ","),
		"target":  targetNormalized,
		"count":   fmt.Sprintf("%d", count),
	})

	return int(count), nil
}

// DeleteTag removes a tag from every evidence item in a case.
func (s *Service) DeleteTag(ctx context.Context, caseID uuid.UUID, tag, actorID string) (int, error) {
	if caseID == uuid.Nil {
		return 0, &ValidationError{Field: "case_id", Message: "case ID is required"}
	}
	if err := ValidateTag(tag); err != nil {
		return 0, err
	}
	normalized := strings.ToLower(strings.TrimSpace(tag))

	count, err := s.repo.DeleteTagFromCase(ctx, caseID, normalized)
	if err != nil {
		return 0, fmt.Errorf("delete tag from case: %w", err)
	}

	s.recordCaseCustodyEvent(ctx, caseID, "tag_deleted", actorID, map[string]string{
		"tag":   normalized,
		"count": fmt.Sprintf("%d", count),
	})

	return int(count), nil
}

// recordCaseCustodyEvent writes a case-level custody entry if the custody
// recorder supports it. Callers should not fail the whole operation when the
// custody write fails; the error is logged instead (mirroring the behaviour of
// recordCustodyEvent for evidence-level events).
func (s *Service) recordCaseCustodyEvent(ctx context.Context, caseID uuid.UUID, action, actorID string, detail map[string]string) {
	if s.custody == nil {
		return
	}
	cr, ok := s.custody.(caseCustodyRecorder)
	if !ok {
		s.logger.Warn("custody recorder does not support case events; skipping",
			"case_id", caseID, "action", action)
		return
	}
	if err := cr.RecordCaseEvent(ctx, caseID, action, actorID, detail); err != nil {
		s.logger.Error("failed to record case custody event",
			"case_id", caseID, "action", action, "error", err)
	}
}
