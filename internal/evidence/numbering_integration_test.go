//go:build integration

package evidence

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Integration tests for GenerateRedactionNumber (uses real Postgres via testcontainers)
// ---------------------------------------------------------------------------

func TestNumbering_GenerateRedactionNumber_NormalCase(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	number, err := GenerateRedactionNumber(ctx, repo, "EV-001", PurposeDisclosureDefence, "John Smith")
	if err != nil {
		t.Fatalf("GenerateRedactionNumber: %v", err)
	}

	want := "EV-001-R-DEFENCE-JOHN-SMITH"
	if number != want {
		t.Errorf("got %q, want %q", number, want)
	}
}

func TestNumbering_GenerateRedactionNumber_AllPurposeCodes(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	tests := []struct {
		purpose  RedactionPurpose
		wantCode string
	}{
		{PurposeDisclosureDefence, "DEFENCE"},
		{PurposeDisclosureProsecution, "PROSECUTION"},
		{PurposePublicRelease, "PUBLIC"},
		{PurposeCourtSubmission, "COURT"},
		{PurposeWitnessProtection, "WITNESS"},
		{PurposeInternalReview, "INTERNAL"},
	}

	for _, tc := range tests {
		t.Run(string(tc.purpose), func(t *testing.T) {
			// Use a unique original number per sub-test to avoid cross-collisions.
			origNum := fmt.Sprintf("EV-%s", tc.wantCode)
			number, err := GenerateRedactionNumber(ctx, repo, origNum, tc.purpose, "Test Name")
			if err != nil {
				t.Fatalf("GenerateRedactionNumber: %v", err)
			}
			want := fmt.Sprintf("%s-R-%s-TEST-NAME", origNum, tc.wantCode)
			if number != want {
				t.Errorf("purpose %q: got %q, want %q", tc.purpose, number, want)
			}
		})
	}
}

func TestNumbering_GenerateRedactionNumber_UnknownPurposeFallback(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	number, err := GenerateRedactionNumber(ctx, repo, "EV-UNK", RedactionPurpose("unknown_purpose"), "Test")
	if err != nil {
		t.Fatalf("GenerateRedactionNumber: %v", err)
	}

	// Unknown purpose falls back to "REDACTED"
	want := "EV-UNK-R-REDACTED-TEST"
	if number != want {
		t.Errorf("got %q, want %q", number, want)
	}
}

func TestNumbering_GenerateRedactionNumber_EmptyName(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	number, err := GenerateRedactionNumber(ctx, repo, "EV-002", PurposeCourtSubmission, "")
	if err != nil {
		t.Fatalf("GenerateRedactionNumber: %v", err)
	}

	want := "EV-002-R-COURT-UNNAMED"
	if number != want {
		t.Errorf("got %q, want %q", number, want)
	}
}

func TestNumbering_GenerateRedactionNumber_VeryLongName(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	longName := "This Is An Extremely Long Witness Name That Exceeds The Maximum Slug Length"
	number, err := GenerateRedactionNumber(ctx, repo, "EV-003", PurposeWitnessProtection, longName)
	if err != nil {
		t.Fatalf("GenerateRedactionNumber: %v", err)
	}

	// The slug portion must be at most 20 runes (per generateNameSlug maxLen=20).
	// Format: EV-003-R-WITNESS-{slug}
	prefix := "EV-003-R-WITNESS-"
	if !strings.HasPrefix(number, prefix) {
		t.Errorf("number %q does not start with %q", number, prefix)
	}
	slug := strings.TrimPrefix(number, prefix)
	if len([]rune(slug)) > 20 {
		t.Errorf("slug %q has %d runes, want <= 20", slug, len([]rune(slug)))
	}
	// Slug must not end with a hyphen.
	if strings.HasSuffix(slug, "-") {
		t.Errorf("slug %q has a trailing hyphen", slug)
	}
}

func TestNumbering_GenerateRedactionNumber_CollisionHandled(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := seedCase(t, pool, "CR-COLL")

	// Seed evidence with the number that GenerateRedactionNumber would produce first.
	baseNumber := "EV-COL-R-DEFENCE-SMITH"
	_, err := pool.Exec(ctx,
		`INSERT INTO evidence_items
			(case_id, evidence_number, filename, original_name, storage_key, mime_type,
			 size_bytes, sha256_hash, classification, tags, uploaded_by, tsa_status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		caseID, baseNumber, "col.pdf", "col.pdf",
		"k/col.pdf", "application/pdf", 100,
		strings.Repeat("c", 64), ClassificationPublic,
		[]string{}, "00000000-0000-4000-8000-000000000001", TSAStatusPending,
	)
	if err != nil {
		t.Fatalf("seed collision evidence: %v", err)
	}

	// Now GenerateRedactionNumber should detect the collision and append -2.
	number, err := GenerateRedactionNumber(ctx, repo, "EV-COL", PurposeDisclosureDefence, "Smith")
	if err != nil {
		t.Fatalf("GenerateRedactionNumber: %v", err)
	}

	want := "EV-COL-R-DEFENCE-SMITH-2"
	if number != want {
		t.Errorf("got %q, want %q", number, want)
	}
}

func TestNumbering_GenerateRedactionNumber_MultipleCollisions(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := seedCase(t, pool, "CR-MCOL")

	// Pre-occupy both the base number and the -2 variant.
	for _, num := range []string{"EV-MC-R-PUBLIC-JONES", "EV-MC-R-PUBLIC-JONES-2"} {
		hash := fmt.Sprintf("%064x", len(num)) // unique hash per row
		_, err := pool.Exec(ctx,
			`INSERT INTO evidence_items
				(case_id, evidence_number, filename, original_name, storage_key, mime_type,
				 size_bytes, sha256_hash, classification, tags, uploaded_by, tsa_status)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
			caseID, num, num+".pdf", num+".pdf",
			"k/"+num, "application/pdf", 100,
			hash, ClassificationPublic,
			[]string{}, "00000000-0000-4000-8000-000000000001", TSAStatusPending,
		)
		if err != nil {
			t.Fatalf("seed %q: %v", num, err)
		}
	}

	number, err := GenerateRedactionNumber(ctx, repo, "EV-MC", PurposePublicRelease, "Jones")
	if err != nil {
		t.Fatalf("GenerateRedactionNumber: %v", err)
	}

	want := "EV-MC-R-PUBLIC-JONES-3"
	if number != want {
		t.Errorf("got %q, want %q", number, want)
	}
}
