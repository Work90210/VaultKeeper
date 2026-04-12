package evidence

import (
	"errors"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestIsValidSHA256Hex(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"64 lowercase hex", strings.Repeat("a", 64), true},
		{"63 chars", strings.Repeat("a", 63), false},
		{"65 chars", strings.Repeat("a", 65), false},
		{"uppercase accepted", strings.Repeat("A", 64), true},
		{"mixed case", strings.Repeat("aB", 32), true},
		{"non hex char", strings.Repeat("g", 64), false},
		{"empty string", "", false},
		{"real sha256", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidSHA256Hex(tt.in); got != tt.want {
				t.Fatalf("isValidSHA256Hex(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestValidateClientHashHeader(t *testing.T) {
	t.Run("missing header", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/", nil)
		_, err := validateClientHashHeader(req)
		if !errors.Is(err, ErrMissingClientHash) {
			t.Fatalf("err = %v, want ErrMissingClientHash", err)
		}
		if !errors.Is(err, ErrMissingClientHashHeader) {
			t.Fatalf("err = %v, want ErrMissingClientHashHeader specifically", err)
		}
	})

	t.Run("malformed header", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/", nil)
		req.Header.Set("X-Content-SHA256", "not-a-hash")
		_, err := validateClientHashHeader(req)
		if !errors.Is(err, ErrMalformedClientHash) {
			t.Fatalf("err = %v, want ErrMalformedClientHash", err)
		}
	})

	t.Run("valid header returns lowercase", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/", nil)
		req.Header.Set("X-Content-SHA256", strings.Repeat("AB", 32))
		got, err := validateClientHashHeader(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != strings.Repeat("ab", 32) {
			t.Fatalf("got %q, want lowercase", got)
		}
	})
}

func TestValidateClientHashForm(t *testing.T) {
	headerHash := strings.Repeat("a", 64)

	t.Run("missing form field", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/", nil)
		req.Form = url.Values{}
		_, err := validateClientHashForm(req, headerHash)
		if !errors.Is(err, ErrMissingClientHash) {
			t.Fatalf("err = %v, want ErrMissingClientHash", err)
		}
		if !errors.Is(err, ErrMissingClientHashFormField) {
			t.Fatalf("err = %v, want ErrMissingClientHashFormField specifically", err)
		}
	})

	t.Run("malformed form field", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/", nil)
		req.Form = url.Values{"client_sha256": {"bad"}}
		_, err := validateClientHashForm(req, headerHash)
		if !errors.Is(err, ErrMalformedClientHash) {
			t.Fatalf("err = %v, want ErrMalformedClientHash", err)
		}
	})

	t.Run("disagreement", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/", nil)
		req.Form = url.Values{"client_sha256": {strings.Repeat("b", 64)}}
		_, err := validateClientHashForm(req, headerHash)
		if !errors.Is(err, ErrHashFieldDisagreement) {
			t.Fatalf("err = %v, want ErrHashFieldDisagreement", err)
		}
	})

	t.Run("agreement case-insensitive", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/", nil)
		req.Form = url.Values{"client_sha256": {strings.Repeat("A", 64)}}
		got, err := validateClientHashForm(req, headerHash)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != strings.Repeat("a", 64) {
			t.Fatalf("got %q, want lowercase", got)
		}
	})
}
