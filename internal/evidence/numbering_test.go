package evidence

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---------------------------------------------------------------------------
// Unit tests for generateNameSlug
// ---------------------------------------------------------------------------

func TestGenerateNameSlug_EmptyString(t *testing.T) {
	got := generateNameSlug("", 20)
	if got != "UNNAMED" {
		t.Errorf("empty string: got %q, want %q", got, "UNNAMED")
	}
}

func TestGenerateNameSlug_SpacesOnly(t *testing.T) {
	got := generateNameSlug("   ", 20)
	if got != "UNNAMED" {
		t.Errorf("spaces-only: got %q, want %q", got, "UNNAMED")
	}
}

func TestGenerateNameSlug_NormalText(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "simple lowercase",
			input:  "hello world",
			maxLen: 20,
			want:   "HELLO-WORLD",
		},
		{
			name:   "already uppercase",
			input:  "JOHN DOE",
			maxLen: 20,
			want:   "JOHN-DOE",
		},
		{
			name:   "mixed case with numbers",
			input:  "Evidence 42",
			maxLen: 20,
			want:   "EVIDENCE-42",
		},
		{
			name:   "single word",
			input:  "witness",
			maxLen: 20,
			want:   "WITNESS",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := generateNameSlug(tc.input, tc.maxLen)
			if got != tc.want {
				t.Errorf("generateNameSlug(%q, %d) = %q, want %q", tc.input, tc.maxLen, got, tc.want)
			}
		})
	}
}

func TestGenerateNameSlug_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "hyphens and underscores",
			input:  "john-doe_smith",
			maxLen: 30,
			want:   "JOHN-DOE-SMITH",
		},
		{
			name:   "dots and slashes",
			input:  "file.name/here",
			maxLen: 30,
			want:   "FILE-NAME-HERE",
		},
		{
			name:   "multiple consecutive specials collapsed to one hyphen",
			input:  "a!!!b",
			maxLen: 20,
			want:   "A-B",
		},
		{
			name:   "leading special characters stripped",
			input:  "!!!hello",
			maxLen: 20,
			want:   "HELLO",
		},
		{
			name:   "trailing special characters stripped",
			input:  "hello!!!",
			maxLen: 20,
			want:   "HELLO",
		},
		{
			name:   "SQL special characters",
			input:  "test'; DROP TABLE",
			maxLen: 40,
			want:   "TEST-DROP-TABLE",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := generateNameSlug(tc.input, tc.maxLen)
			if got != tc.want {
				t.Errorf("generateNameSlug(%q, %d) = %q, want %q", tc.input, tc.maxLen, got, tc.want)
			}
		})
	}
}

func TestGenerateNameSlug_Unicode(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		// Unicode non-ASCII letters are non-alphanumeric under [^A-Z0-9]+
		// so they become hyphens and then get trimmed.
		wantUnnamed bool
		want        string
	}{
		{
			name:        "pure emoji",
			input:       "🔒🔒🔒",
			maxLen:      20,
			wantUnnamed: true,
		},
		{
			name:        "arabic letters only",
			input:       "مرحبا",
			maxLen:      20,
			wantUnnamed: true,
		},
		{
			name:   "ascii mixed with emoji",
			input:  "case🔒123",
			maxLen: 20,
			want:   "CASE-123",
		},
		{
			name:  "accented characters become hyphens then trimmed",
			input: "café",
			// ToUpper("café") = "CAFé" — the 'é' is non-ASCII so slugRe replaces it with '-'
			// giving "CAF-", then Trim("-") strips the trailing dash, leaving "CAF".
			maxLen: 20,
			want:   "CAF",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := generateNameSlug(tc.input, tc.maxLen)
			if tc.wantUnnamed {
				if got != "UNNAMED" {
					t.Errorf("generateNameSlug(%q, %d) = %q, want %q", tc.input, tc.maxLen, got, "UNNAMED")
				}
				return
			}
			if got != tc.want {
				t.Errorf("generateNameSlug(%q, %d) = %q, want %q", tc.input, tc.maxLen, got, tc.want)
			}
		})
	}
}

func TestGenerateNameSlug_Truncation(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "truncated at exact boundary within word",
			input:  "ABCDEFGHIJ",
			maxLen: 5,
			want:   "ABCDE",
		},
		{
			name:   "trailing hyphen removed after truncation",
			input:  "ABC-DEFGHIJ",
			maxLen: 4,
			// After uppercase + slug: "ABC-DEFGHIJ"
			// Rune slice [:4] = "ABC-"
			// TrimRight("-") = "ABC"
			want: "ABC",
		},
		{
			name:   "exactly at maxLen, no trailing hyphen",
			input:  "ABCDE FGHIJ",
			maxLen: 9,
			// Slug: "ABCDE-FGHIJ" (11 runes), truncated to 9 => "ABCDE-FGH"
			want: "ABCDE-FGH",
		},
		{
			name:   "truncation leaves only a hyphen — falls back to UNNAMED",
			input:  "---",
			maxLen: 3,
			// After slug: "" (all hyphens trimmed) => UNNAMED
			want: "UNNAMED",
		},
		{
			name:   "maxLen zero produces UNNAMED",
			input:  "hello",
			maxLen: 0,
			// runes[:0] = "" → TrimRight → "" → UNNAMED
			want: "UNNAMED",
		},
		{
			name:   "maxLen larger than slug length — no truncation",
			input:  "hi",
			maxLen: 100,
			want:   "HI",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := generateNameSlug(tc.input, tc.maxLen)
			if got != tc.want {
				t.Errorf("generateNameSlug(%q, %d) = %q, want %q", tc.input, tc.maxLen, got, tc.want)
			}
		})
	}
}

func TestGenerateNameSlug_MultipleHyphensCollapsed(t *testing.T) {
	// Verifies that [^A-Z0-9]+ (one or more non-alphanumeric) maps to exactly one hyphen.
	got := generateNameSlug("a   b   c", 20)
	if strings.Contains(got, "--") {
		t.Errorf("slug should not contain consecutive hyphens: %q", got)
	}
	if got != "A-B-C" {
		t.Errorf("got %q, want %q", got, "A-B-C")
	}
}

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

func TestNumbering_GenerateRedactionNumber_RepoError(t *testing.T) {
	// Use a broken pool by providing a bad DSN so that QueryRow fails.
	// We inject a failing dbPool via the internal field directly.
	repo := &PGRepository{pool: &failingPool{}}
	ctx := context.Background()

	_, err := GenerateRedactionNumber(ctx, repo, "EV-ERR", PurposeInternalReview, "Test")
	if err == nil {
		t.Fatal("expected error from repo failure, got nil")
	}
	if !strings.Contains(err.Error(), "check evidence number collision") {
		t.Errorf("error %q should mention 'check evidence number collision'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// failingPool — a minimal dbPool implementation that always errors.
// It satisfies the unexported dbPool interface defined in repository.go.
// ---------------------------------------------------------------------------

type failingPool struct{}

func (f *failingPool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return &failingRow{}
}

func (f *failingPool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, errors.New("failingPool: Query not implemented")
}

func (f *failingPool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("failingPool: Exec not implemented")
}

func (f *failingPool) BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error) {
	return nil, errors.New("failingPool: BeginTx not implemented")
}

type failingRow struct{}

func (r *failingRow) Scan(dest ...any) error {
	return errors.New("failingPool: simulated DB error")
}
