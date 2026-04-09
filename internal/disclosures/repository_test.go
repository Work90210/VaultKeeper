package disclosures

import (
	"testing"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// decodeCursor / encodeCursor – pure unit tests (no DB required)
// ---------------------------------------------------------------------------

func TestEncodeCursor_ProducesNonEmptyString(t *testing.T) {
	id := uuid.New()
	got := encodeCursor(id)
	if got == "" {
		t.Fatal("encodeCursor returned empty string")
	}
}

func TestEncodeCursor_DifferentIDsProduceDifferentCursors(t *testing.T) {
	a := encodeCursor(uuid.New())
	b := encodeCursor(uuid.New())
	if a == b {
		t.Error("two different UUIDs produced the same cursor")
	}
}

func TestDecodeCursor_RoundTrip(t *testing.T) {
	original := uuid.New()
	encoded := encodeCursor(original)
	decoded, err := decodeCursor(encoded)
	if err != nil {
		t.Fatalf("decodeCursor returned unexpected error: %v", err)
	}
	if decoded != original {
		t.Errorf("round-trip mismatch: got %s, want %s", decoded, original)
	}
}

func TestDecodeCursor_InvalidBase64_ReturnsError(t *testing.T) {
	_, err := decodeCursor("!!!not-valid-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64 cursor, got nil")
	}
}

func TestDecodeCursor_ValidBase64ButNotUUID_ReturnsError(t *testing.T) {
	// Base64-encode something that is not a UUID string.
	import64 := encodeCursor(uuid.Nil) // known-good encoding path
	// Corrupt it by appending extra bytes after base64 decoding would give garbage.
	_, err := decodeCursor(import64 + "AAAA")
	if err == nil {
		t.Fatal("expected error for base64 that does not decode to a UUID, got nil")
	}
}

func TestDecodeCursor_EmptyString_ReturnsError(t *testing.T) {
	// An empty cursor string cannot represent a UUID.
	_, err := decodeCursor("")
	if err == nil {
		t.Fatal("expected error for empty cursor, got nil")
	}
}

func TestEncodeCursor_NilUUID_Decodable(t *testing.T) {
	// uuid.Nil (all-zeros) should round-trip cleanly.
	encoded := encodeCursor(uuid.Nil)
	decoded, err := decodeCursor(encoded)
	if err != nil {
		t.Fatalf("decodeCursor of nil UUID cursor returned error: %v", err)
	}
	if decoded != uuid.Nil {
		t.Errorf("decoded = %s, want nil UUID", decoded)
	}
}
