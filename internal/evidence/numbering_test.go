package evidence

import (
	"context"
	"errors"
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
// Unit test for GenerateRedactionNumber using an injected failing pool.
// ---------------------------------------------------------------------------

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
