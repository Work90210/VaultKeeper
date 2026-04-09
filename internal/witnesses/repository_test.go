package witnesses

import (
	"encoding/base64"
	"testing"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// encodeCursor / decodeCursor — pure unit tests, no DB required
// ---------------------------------------------------------------------------

func TestEncodeCursor_DecodeCursor_RoundTrip(t *testing.T) {
	id := uuid.New()
	encoded := encodeCursor(id)
	if encoded == "" {
		t.Fatal("encodeCursor returned empty string")
	}

	decoded, err := decodeCursor(encoded)
	if err != nil {
		t.Fatalf("decodeCursor: %v", err)
	}
	if decoded != id {
		t.Errorf("round-trip mismatch: got %v, want %v", decoded, id)
	}
}

func TestEncodeCursor_NilUUID(t *testing.T) {
	encoded := encodeCursor(uuid.Nil)
	if encoded == "" {
		t.Fatal("encodeCursor returned empty string for nil UUID")
	}

	decoded, err := decodeCursor(encoded)
	if err != nil {
		t.Fatalf("decodeCursor: %v", err)
	}
	if decoded != uuid.Nil {
		t.Errorf("got %v, want uuid.Nil", decoded)
	}
}

func TestDecodeCursor_InvalidBase64(t *testing.T) {
	_, err := decodeCursor("!!!not-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64, got nil")
	}
}

func TestDecodeCursor_ValidBase64ButNotUUID(t *testing.T) {
	// Base64-encode a string that is not a valid UUID.
	notUUID := base64.RawURLEncoding.EncodeToString([]byte("not-a-uuid"))
	_, err := decodeCursor(notUUID)
	if err == nil {
		t.Fatal("expected error for non-UUID content, got nil")
	}
}

func TestDecodeCursor_EmptyString(t *testing.T) {
	// An empty string decodes to an empty byte slice; uuid.Parse("") returns an error.
	_, err := decodeCursor("")
	if err == nil {
		t.Fatal("expected error for empty cursor, got nil")
	}
}

func TestEncodeCursor_IsURLSafe(t *testing.T) {
	// Raw URL-safe base64 must not contain '+', '/', or '=' padding characters.
	id := uuid.New()
	encoded := encodeCursor(id)
	for _, ch := range encoded {
		if ch == '+' || ch == '/' || ch == '=' {
			t.Errorf("encodeCursor contains non-URL-safe character %q in %q", ch, encoded)
		}
	}
}

func TestEncodeCursor_Deterministic(t *testing.T) {
	id := uuid.New()
	a := encodeCursor(id)
	b := encodeCursor(id)
	if a != b {
		t.Errorf("encodeCursor not deterministic: %q != %q", a, b)
	}
}

func TestDecodeCursor_MultipleUUIDs(t *testing.T) {
	// Verify different UUIDs produce different cursors and decode back correctly.
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	seen := make(map[string]bool)
	for _, id := range ids {
		encoded := encodeCursor(id)
		if seen[encoded] {
			t.Errorf("collision: two different UUIDs produced the same cursor %q", encoded)
		}
		seen[encoded] = true

		decoded, err := decodeCursor(encoded)
		if err != nil {
			t.Fatalf("decodeCursor(%q): %v", encoded, err)
		}
		if decoded != id {
			t.Errorf("decoded %v, want %v", decoded, id)
		}
	}
}
