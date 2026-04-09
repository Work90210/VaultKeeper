package evidence

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

// slugRe matches non-alphanumeric characters for slug generation.
var slugRe = regexp.MustCompile(`[^A-Z0-9]+`)

// generateNameSlug converts a draft name to an uppercase alphanumeric slug.
func generateNameSlug(name string, maxLen int) string {
	slug := strings.ToUpper(strings.TrimSpace(name))
	slug = slugRe.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	runes := []rune(slug)
	if len(runes) > maxLen {
		slug = string(runes[:maxLen])
		slug = strings.TrimRight(slug, "-")
	}
	if slug == "" {
		slug = "UNNAMED"
	}
	return slug
}

// GenerateRedactionNumber builds a human-readable evidence number for a finalized
// redacted copy, handling collisions by appending a sequential suffix.
//
// Format: {originalNumber}-R-{PURPOSE_CODE}-{NAME_SLUG}
func GenerateRedactionNumber(ctx context.Context, repo *PGRepository, originalNumber string, purpose RedactionPurpose, name string) (string, error) {
	code := PurposeCode[purpose]
	if code == "" {
		code = "REDACTED"
	}

	slug := generateNameSlug(name, 20)
	base := fmt.Sprintf("%s-R-%s-%s", originalNumber, code, slug)

	candidate := base
	for i := 2; i <= 100; i++ {
		exists, err := repo.CheckEvidenceNumberExists(ctx, candidate)
		if err != nil {
			return "", fmt.Errorf("check evidence number collision: %w", err)
		}
		if !exists {
			if i > 2 {
				slog.Warn("evidence number collision resolved", "base", base, "resolved", candidate, "attempts", i-1)
			}
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
	}

	// unreachable: would require >100 pre-existing rows with the same base number
	return "", fmt.Errorf("too many evidence number collisions for base %q", base)
}
