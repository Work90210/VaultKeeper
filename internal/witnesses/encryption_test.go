package witnesses

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// testKey32 is a valid 32-byte master key for use across tests.
var testKey32 = []byte("test-master-key-32bytes-padded!!")

// testKey32v2 is a distinct 32-byte key for key-rotation tests.
var testKey32v2 = []byte("rotated-master-key-32bytes-pad!!")

// mustNewEncryptor is a test helper that panics if NewEncryptor returns an error.
func mustNewEncryptor(t *testing.T, keys ...EncryptionKey) *Encryptor {
	t.Helper()
	e, err := NewEncryptor(keys...)
	if err != nil {
		t.Fatalf("mustNewEncryptor: unexpected error: %v", err)
	}
	return e
}

// ptr returns a pointer to the given string, for EncryptField/DecryptField tests.
func ptr(s string) *string { return &s }

// ──────────────────────────────────────────────────────────────────────────────
// NewEncryptor
// ──────────────────────────────────────────────────────────────────────────────

func TestNewEncryptor_NoKeys(t *testing.T) {
	_, err := NewEncryptor()
	if !errors.Is(err, ErrMissingEncryptionKey) {
		t.Fatalf("want ErrMissingEncryptionKey, got %v", err)
	}
}

func TestNewEncryptor_KeyTooShort(t *testing.T) {
	cases := []struct {
		name    string
		keyLen  int
		wantErr bool
	}{
		{"15 bytes — too short", 15, true},
		{"16 bytes — minimum valid", 16, false},
		{"32 bytes — normal", 32, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k := EncryptionKey{Version: 1, Key: bytes.Repeat([]byte("x"), tc.keyLen)}
			_, err := NewEncryptor(k)
			if tc.wantErr && err == nil {
				t.Fatal("want error for short key, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("want no error, got %v", err)
			}
		})
	}
}

func TestNewEncryptor_CurrentVersionIsLastKey(t *testing.T) {
	k1 := EncryptionKey{Version: 1, Key: testKey32}
	k2 := EncryptionKey{Version: 2, Key: testKey32v2}

	e := mustNewEncryptor(t, k1, k2)

	if got := e.CurrentVersion(); got != 2 {
		t.Fatalf("want CurrentVersion=2, got %d", got)
	}
}

func TestNewEncryptor_SingleKey(t *testing.T) {
	k := EncryptionKey{Version: 5, Key: testKey32}
	e := mustNewEncryptor(t, k)

	if got := e.CurrentVersion(); got != 5 {
		t.Fatalf("want CurrentVersion=5, got %d", got)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// CurrentVersion
// ──────────────────────────────────────────────────────────────────────────────

func TestCurrentVersion(t *testing.T) {
	cases := []struct {
		name    string
		version byte
	}{
		{"version 0", 0},
		{"version 1", 1},
		{"version 255", 255},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := mustNewEncryptor(t, EncryptionKey{Version: tc.version, Key: testKey32})
			if got := e.CurrentVersion(); got != tc.version {
				t.Fatalf("want %d, got %d", tc.version, got)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Encrypt → Decrypt roundtrip
// ──────────────────────────────────────────────────────────────────────────────

func TestEncryptDecrypt_Roundtrip(t *testing.T) {
	cases := []struct {
		name      string
		plaintext string
		witness   string
		field     string
	}{
		{"typical name field", "Jane Doe", "witness-001", "full_name"},
		{"phone number", "+1-555-867-5309", "witness-999", "phone"},
		{"empty string", "", "witness-abc", "address"},
		{"unicode text", "Ján Novák — 日本語テスト", "witness-xyz", "full_name"},
		{"special chars", "O'Brien & \"Associates\" <test>", "witness-special", "employer"},
		{"long value", strings.Repeat("A", 10000), "witness-big", "notes"},
		{"sql injection chars", "'; DROP TABLE witnesses; --", "witness-sql", "alias"},
	}

	e := mustNewEncryptor(t, EncryptionKey{Version: 1, Key: testKey32})

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ct, err := e.Encrypt([]byte(tc.plaintext), tc.witness, tc.field)
			if err != nil {
				t.Fatalf("Encrypt: %v", err)
			}

			got, err := e.Decrypt(ct, tc.witness, tc.field)
			if err != nil {
				t.Fatalf("Decrypt: %v", err)
			}

			if string(got) != tc.plaintext {
				t.Fatalf("roundtrip mismatch: want %q, got %q", tc.plaintext, string(got))
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Ciphertext format
// ──────────────────────────────────────────────────────────────────────────────

func TestEncrypt_CiphertextFormat(t *testing.T) {
	e := mustNewEncryptor(t, EncryptionKey{Version: 3, Key: testKey32})

	plaintext := []byte("format test")
	ct, err := e.Encrypt(plaintext, "witness-1", "full_name")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Minimum: 1 (version) + 12 (nonce) + 16 (GCM tag) = 29 bytes
	// With plaintext: 29 + len(plaintext)
	minLen := keyVersionSize + nonceSize + gcmTagSize
	wantLen := minLen + len(plaintext)

	if len(ct) != wantLen {
		t.Fatalf("ciphertext length: want %d, got %d", wantLen, len(ct))
	}

	// First byte must be the key version.
	if ct[0] != 3 {
		t.Fatalf("version byte: want 3, got %d", ct[0])
	}
}

func TestEncrypt_KeyVersionByteCorrectlyPrepended(t *testing.T) {
	cases := []struct{ version byte }{{1}, {2}, {100}, {255}}

	for _, tc := range cases {
		t.Run("", func(t *testing.T) {
			e := mustNewEncryptor(t, EncryptionKey{Version: tc.version, Key: testKey32})
			ct, err := e.Encrypt([]byte("hello"), "w1", "field")
			if err != nil {
				t.Fatalf("Encrypt: %v", err)
			}
			if ct[0] != tc.version {
				t.Fatalf("want version byte %d, got %d", tc.version, ct[0])
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Uniqueness guarantees
// ──────────────────────────────────────────────────────────────────────────────

func TestEncrypt_DifferentWitnessesProduceDifferentCiphertext(t *testing.T) {
	e := mustNewEncryptor(t, EncryptionKey{Version: 1, Key: testKey32})

	plaintext := []byte("same plaintext")
	ct1, err := e.Encrypt(plaintext, "witness-AAA", "full_name")
	if err != nil {
		t.Fatalf("Encrypt witness-AAA: %v", err)
	}
	ct2, err := e.Encrypt(plaintext, "witness-BBB", "full_name")
	if err != nil {
		t.Fatalf("Encrypt witness-BBB: %v", err)
	}

	if bytes.Equal(ct1, ct2) {
		t.Fatal("expected different ciphertext for different witness IDs, got equal")
	}
}

func TestEncrypt_DifferentFieldNamesProduceDifferentCiphertext(t *testing.T) {
	e := mustNewEncryptor(t, EncryptionKey{Version: 1, Key: testKey32})

	plaintext := []byte("same plaintext")
	ct1, err := e.Encrypt(plaintext, "witness-1", "full_name")
	if err != nil {
		t.Fatalf("Encrypt full_name: %v", err)
	}
	ct2, err := e.Encrypt(plaintext, "witness-1", "address")
	if err != nil {
		t.Fatalf("Encrypt address: %v", err)
	}

	if bytes.Equal(ct1, ct2) {
		t.Fatal("expected different ciphertext for different field names, got equal")
	}
}

func TestEncrypt_SameInputProducesUniqueNonces(t *testing.T) {
	// Two encryptions of the same plaintext should produce different ciphertexts
	// because each call generates a fresh random nonce.
	e := mustNewEncryptor(t, EncryptionKey{Version: 1, Key: testKey32})

	ct1, err := e.Encrypt([]byte("hello"), "witness-1", "field")
	if err != nil {
		t.Fatalf("first Encrypt: %v", err)
	}
	ct2, err := e.Encrypt([]byte("hello"), "witness-1", "field")
	if err != nil {
		t.Fatalf("second Encrypt: %v", err)
	}

	if bytes.Equal(ct1, ct2) {
		t.Fatal("expected unique ciphertexts on repeated encryption, got equal (nonce reuse)")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Tamper / authentication failure
// ──────────────────────────────────────────────────────────────────────────────

func TestDecrypt_TamperedCiphertextFails(t *testing.T) {
	e := mustNewEncryptor(t, EncryptionKey{Version: 1, Key: testKey32})

	ct, err := e.Encrypt([]byte("sensitive data"), "witness-1", "full_name")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Flip a byte in the ciphertext body (after version+nonce).
	tampered := make([]byte, len(ct))
	copy(tampered, ct)
	tampered[keyVersionSize+nonceSize] ^= 0xFF

	_, err = e.Decrypt(tampered, "witness-1", "full_name")
	if !errors.Is(err, ErrDecryptionFailed) {
		t.Fatalf("want ErrDecryptionFailed, got %v", err)
	}
}

func TestDecrypt_TamperedNonceFails(t *testing.T) {
	e := mustNewEncryptor(t, EncryptionKey{Version: 1, Key: testKey32})

	ct, err := e.Encrypt([]byte("sensitive data"), "witness-1", "full_name")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	tampered := make([]byte, len(ct))
	copy(tampered, ct)
	tampered[keyVersionSize] ^= 0xFF // flip first nonce byte

	_, err = e.Decrypt(tampered, "witness-1", "full_name")
	if !errors.Is(err, ErrDecryptionFailed) {
		t.Fatalf("want ErrDecryptionFailed, got %v", err)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Wrong field name — field binding
// ──────────────────────────────────────────────────────────────────────────────

func TestDecrypt_WrongFieldNameFails(t *testing.T) {
	e := mustNewEncryptor(t, EncryptionKey{Version: 1, Key: testKey32})

	ct, err := e.Encrypt([]byte("Jane Doe"), "witness-1", "full_name")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	_, err = e.Decrypt(ct, "witness-1", "address") // wrong field
	if !errors.Is(err, ErrDecryptionFailed) {
		t.Fatalf("want ErrDecryptionFailed for wrong field, got %v", err)
	}
}

func TestDecrypt_WrongWitnessIDFails(t *testing.T) {
	e := mustNewEncryptor(t, EncryptionKey{Version: 1, Key: testKey32})

	ct, err := e.Encrypt([]byte("Jane Doe"), "witness-1", "full_name")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	_, err = e.Decrypt(ct, "witness-2", "full_name") // wrong witness
	if !errors.Is(err, ErrDecryptionFailed) {
		t.Fatalf("want ErrDecryptionFailed for wrong witness, got %v", err)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Ciphertext too short
// ──────────────────────────────────────────────────────────────────────────────

func TestDecrypt_CiphertextTooShort(t *testing.T) {
	e := mustNewEncryptor(t, EncryptionKey{Version: 1, Key: testKey32})

	cases := []struct {
		name string
		ct   []byte
	}{
		{"nil slice", nil},
		{"empty slice", []byte{}},
		{"one byte", []byte{0x01}},
		{"28 bytes — one under minimum", bytes.Repeat([]byte{0x00}, 28)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// nil is handled specially by DecryptField, but Decrypt itself should error.
			if tc.ct == nil {
				// Decrypt with nil is a short ciphertext (len=0 < minLen=29).
				_, err := e.Decrypt(tc.ct, "w", "f")
				if err == nil {
					t.Fatal("want error for nil ciphertext, got nil")
				}
				return
			}
			_, err := e.Decrypt(tc.ct, "w", "f")
			if err == nil {
				t.Fatalf("want error for ciphertext of length %d, got nil", len(tc.ct))
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Unsupported key version
// ──────────────────────────────────────────────────────────────────────────────

func TestDecrypt_UnsupportedKeyVersion(t *testing.T) {
	e := mustNewEncryptor(t, EncryptionKey{Version: 1, Key: testKey32})

	ct, err := e.Encrypt([]byte("hello"), "witness-1", "field")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Override the version byte to a version the encryptor doesn't know.
	ct[0] = 99

	_, err = e.Decrypt(ct, "witness-1", "field")
	if !errors.Is(err, ErrUnsupportedKeyVersion) {
		t.Fatalf("want ErrUnsupportedKeyVersion, got %v", err)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Key rotation
// ──────────────────────────────────────────────────────────────────────────────

func TestKeyRotation_OldDataDecryptableAfterRotation(t *testing.T) {
	// Encryptor with only key v1.
	eV1 := mustNewEncryptor(t, EncryptionKey{Version: 1, Key: testKey32})

	plaintext := []byte("protected witness name")
	ctV1, err := eV1.Encrypt(plaintext, "witness-1", "full_name")
	if err != nil {
		t.Fatalf("Encrypt with v1: %v", err)
	}

	// After rotation: encryptor knows both v1 and v2; v2 is current.
	eRotated := mustNewEncryptor(t,
		EncryptionKey{Version: 1, Key: testKey32},
		EncryptionKey{Version: 2, Key: testKey32v2},
	)

	if eRotated.CurrentVersion() != 2 {
		t.Fatalf("want current version 2, got %d", eRotated.CurrentVersion())
	}

	// Old v1 ciphertext must still decrypt.
	got, err := eRotated.Decrypt(ctV1, "witness-1", "full_name")
	if err != nil {
		t.Fatalf("Decrypt old v1 ciphertext: %v", err)
	}
	if string(got) != string(plaintext) {
		t.Fatalf("roundtrip mismatch: want %q, got %q", plaintext, string(got))
	}
}

func TestKeyRotation_NewDataUsesNewKey(t *testing.T) {
	eRotated := mustNewEncryptor(t,
		EncryptionKey{Version: 1, Key: testKey32},
		EncryptionKey{Version: 2, Key: testKey32v2},
	)

	ct, err := eRotated.Encrypt([]byte("new data"), "witness-1", "field")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// New ciphertext must carry version byte 2.
	if ct[0] != 2 {
		t.Fatalf("want version byte 2, got %d", ct[0])
	}
}

func TestKeyRotation_CannotDecryptWithoutOldKey(t *testing.T) {
	// Encrypt with v1.
	eV1 := mustNewEncryptor(t, EncryptionKey{Version: 1, Key: testKey32})
	ct, err := eV1.Encrypt([]byte("hello"), "witness-1", "field")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Encryptor that knows only v2 (old key dropped).
	eV2Only := mustNewEncryptor(t, EncryptionKey{Version: 2, Key: testKey32v2})

	_, err = eV2Only.Decrypt(ct, "witness-1", "field")
	if !errors.Is(err, ErrUnsupportedKeyVersion) {
		t.Fatalf("want ErrUnsupportedKeyVersion, got %v", err)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// EncryptField
// ──────────────────────────────────────────────────────────────────────────────

func TestEncryptField_NilInputReturnsNil(t *testing.T) {
	e := mustNewEncryptor(t, EncryptionKey{Version: 1, Key: testKey32})

	ct, err := e.EncryptField(nil, "witness-1", "full_name")
	if err != nil {
		t.Fatalf("EncryptField(nil): unexpected error: %v", err)
	}
	if ct != nil {
		t.Fatalf("EncryptField(nil): want nil ciphertext, got %v", ct)
	}
}

func TestEncryptField_EmptyStringIsEncrypted(t *testing.T) {
	e := mustNewEncryptor(t, EncryptionKey{Version: 1, Key: testKey32})

	s := ""
	ct, err := e.EncryptField(&s, "witness-1", "full_name")
	if err != nil {
		t.Fatalf("EncryptField empty string: %v", err)
	}
	if ct == nil {
		t.Fatal("EncryptField empty string: want non-nil ciphertext, got nil")
	}

	// Minimum ciphertext length: version + nonce + GCM tag (no plaintext bytes).
	minLen := keyVersionSize + nonceSize + gcmTagSize
	if len(ct) < minLen {
		t.Fatalf("ciphertext too short: want >= %d, got %d", minLen, len(ct))
	}
}

func TestEncryptField_Roundtrip(t *testing.T) {
	e := mustNewEncryptor(t, EncryptionKey{Version: 1, Key: testKey32})

	cases := []struct {
		name  string
		value *string
	}{
		{"nil", nil},
		{"empty string", ptr("")},
		{"normal value", ptr("Jane Doe")},
		{"unicode", ptr("日本語")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ct, err := e.EncryptField(tc.value, "witness-1", "full_name")
			if err != nil {
				t.Fatalf("EncryptField: %v", err)
			}

			got, err := e.DecryptField(ct, "witness-1", "full_name")
			if err != nil {
				t.Fatalf("DecryptField: %v", err)
			}

			if tc.value == nil {
				if got != nil {
					t.Fatalf("want nil, got %q", *got)
				}
				return
			}
			if got == nil {
				t.Fatal("want non-nil result, got nil")
			}
			if *got != *tc.value {
				t.Fatalf("roundtrip mismatch: want %q, got %q", *tc.value, *got)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// DecryptField
// ──────────────────────────────────────────────────────────────────────────────

func TestDecryptField_NilInputReturnsNil(t *testing.T) {
	e := mustNewEncryptor(t, EncryptionKey{Version: 1, Key: testKey32})

	got, err := e.DecryptField(nil, "witness-1", "full_name")
	if err != nil {
		t.Fatalf("DecryptField(nil): unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("DecryptField(nil): want nil, got %q", *got)
	}
}

func TestDecryptField_TamperedCiphertextFails(t *testing.T) {
	e := mustNewEncryptor(t, EncryptionKey{Version: 1, Key: testKey32})

	s := "sensitive"
	ct, err := e.EncryptField(&s, "witness-1", "full_name")
	if err != nil {
		t.Fatalf("EncryptField: %v", err)
	}

	ct[keyVersionSize+nonceSize] ^= 0xFF

	_, err = e.DecryptField(ct, "witness-1", "full_name")
	if !errors.Is(err, ErrDecryptionFailed) {
		t.Fatalf("want ErrDecryptionFailed, got %v", err)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Null vs empty-string distinction
// ──────────────────────────────────────────────────────────────────────────────

func TestNullAndEmptyStringAreDistinct(t *testing.T) {
	e := mustNewEncryptor(t, EncryptionKey{Version: 1, Key: testKey32})

	ctNull, err := e.EncryptField(nil, "witness-1", "full_name")
	if err != nil {
		t.Fatalf("EncryptField(nil): %v", err)
	}

	empty := ""
	ctEmpty, err := e.EncryptField(&empty, "witness-1", "full_name")
	if err != nil {
		t.Fatalf("EncryptField(empty): %v", err)
	}

	// Null produces nil ciphertext; empty string produces non-nil ciphertext.
	if ctNull != nil {
		t.Fatal("null input must produce nil ciphertext")
	}
	if ctEmpty == nil {
		t.Fatal("empty string input must produce non-nil ciphertext")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Minimum-length ciphertext (29 bytes exactly) is accepted
// ──────────────────────────────────────────────────────────────────────────────

func TestDecrypt_MinimumLengthCiphertextAccepted(t *testing.T) {
	e := mustNewEncryptor(t, EncryptionKey{Version: 1, Key: testKey32})

	// Encrypt empty plaintext → version(1) + nonce(12) + tag(16) = 29 bytes.
	ct, err := e.Encrypt([]byte{}, "witness-1", "field")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	minLen := keyVersionSize + nonceSize + gcmTagSize
	if len(ct) != minLen {
		t.Fatalf("want ciphertext length %d, got %d", minLen, len(ct))
	}

	got, err := e.Decrypt(ct, "witness-1", "field")
	if err != nil {
		t.Fatalf("Decrypt minimum-length ciphertext: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want empty plaintext, got %q", string(got))
	}
}
