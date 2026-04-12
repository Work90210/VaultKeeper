package witnesses

// Coverage-fill tests for the "unreachable in production" defensive
// error branches in encryption.go. The branches guard against stdlib
// primitives failing (HKDF, AES, GCM) or rand.Reader returning short
// reads — events that cannot occur with correct inputs. To cover them
// the tests swap the package-level function variables with failing
// substitutes, verify the error wrapping, and restore the originals.

import (
	"crypto/cipher"
	"errors"
	"hash"
	"io"
	"strings"
	"testing"
)

func withHKDFStub(t *testing.T, stub func(h func() hash.Hash, secret, salt, info []byte) io.Reader) {
	t.Helper()
	orig := hkdfNewReader
	hkdfNewReader = stub
	t.Cleanup(func() { hkdfNewReader = orig })
}

func withAESStub(t *testing.T, stub func([]byte) (cipher.Block, error)) {
	t.Helper()
	orig := aesNewCipherFn
	aesNewCipherFn = stub
	t.Cleanup(func() { aesNewCipherFn = orig })
}

func withGCMStub(t *testing.T, stub func(cipher.Block) (cipher.AEAD, error)) {
	t.Helper()
	orig := cipherNewGCMFn
	cipherNewGCMFn = stub
	t.Cleanup(func() { cipherNewGCMFn = orig })
}

func withRandStub(t *testing.T, r io.Reader) {
	t.Helper()
	orig := randReader
	randReader = r
	t.Cleanup(func() { randReader = orig })
}

// failingReader returns error on every Read.
type failingReader struct{ err error }

func (f *failingReader) Read(_ []byte) (int, error) { return 0, f.err }

func newTestEncryptor(t *testing.T) *Encryptor {
	t.Helper()
	enc, err := NewEncryptor(EncryptionKey{Version: 1, Key: make([]byte, 32)})
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}
	return enc
}

// ---- deriveKey: HKDF short read ----

func TestDeriveKey_HKDFShortRead(t *testing.T) {
	withHKDFStub(t, func(_ func() hash.Hash, _, _, _ []byte) io.Reader {
		return &failingReader{err: errors.New("hkdf exploded")}
	})

	enc := newTestEncryptor(t)
	_, err := enc.deriveKey(make([]byte, 32), "witness-1")
	if err == nil || !strings.Contains(err.Error(), "derive witness key") {
		t.Errorf("want derive witness key error, got %v", err)
	}
}

// ---- Encrypt: deriveKey failure propagates ----

func TestEncrypt_DeriveKeyError(t *testing.T) {
	withHKDFStub(t, func(_ func() hash.Hash, _, _, _ []byte) io.Reader {
		return &failingReader{err: errors.New("hkdf fail")}
	})
	enc := newTestEncryptor(t)
	_, err := enc.Encrypt([]byte("pt"), "w", "field")
	if err == nil {
		t.Fatal("want error")
	}
}

// ---- Encrypt: AES cipher construction failure ----

func TestEncrypt_AESError(t *testing.T) {
	withAESStub(t, func(_ []byte) (cipher.Block, error) {
		return nil, errors.New("aes boom")
	})
	enc := newTestEncryptor(t)
	_, err := enc.Encrypt([]byte("pt"), "w", "field")
	if err == nil || !strings.Contains(err.Error(), "create AES cipher") {
		t.Errorf("want wrapped AES error, got %v", err)
	}
}

// ---- Encrypt: GCM construction failure ----

func TestEncrypt_GCMError(t *testing.T) {
	withGCMStub(t, func(_ cipher.Block) (cipher.AEAD, error) {
		return nil, errors.New("gcm boom")
	})
	enc := newTestEncryptor(t)
	_, err := enc.Encrypt([]byte("pt"), "w", "field")
	if err == nil || !strings.Contains(err.Error(), "create GCM") {
		t.Errorf("want wrapped GCM error, got %v", err)
	}
}

// ---- Encrypt: nonce generation failure ----

func TestEncrypt_NonceError(t *testing.T) {
	withRandStub(t, &failingReader{err: errors.New("no entropy")})
	enc := newTestEncryptor(t)
	_, err := enc.Encrypt([]byte("pt"), "w", "field")
	if err == nil || !strings.Contains(err.Error(), "generate nonce") {
		t.Errorf("want wrapped nonce error, got %v", err)
	}
}

// ---- Encrypt: missing current version (NewEncryptor invariant broken) ----

func TestEncrypt_CurrentKeyMissing(t *testing.T) {
	enc := &Encryptor{
		keys:    map[byte]EncryptionKey{1: {Version: 1, Key: make([]byte, 32)}},
		current: 99, // not in map
	}
	_, err := enc.Encrypt([]byte("pt"), "w", "field")
	if err == nil || !strings.Contains(err.Error(), "current key version") {
		t.Errorf("want current key version error, got %v", err)
	}
}

// ---- Decrypt: AES + GCM failure via stubbed primitives ----

func encryptForDecryptTest(t *testing.T, enc *Encryptor, pt []byte) []byte {
	t.Helper()
	ct, err := enc.Encrypt(pt, "w", "field")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	return ct
}

func TestDecrypt_DeriveKeyError(t *testing.T) {
	enc := newTestEncryptor(t)
	ct := encryptForDecryptTest(t, enc, []byte("payload"))
	withHKDFStub(t, func(_ func() hash.Hash, _, _, _ []byte) io.Reader {
		return &failingReader{err: errors.New("hkdf fail")}
	})
	_, err := enc.Decrypt(ct, "w", "field")
	if err == nil {
		t.Fatal("want error")
	}
}

func TestDecrypt_AESError(t *testing.T) {
	enc := newTestEncryptor(t)
	ct := encryptForDecryptTest(t, enc, []byte("payload"))
	withAESStub(t, func(_ []byte) (cipher.Block, error) {
		return nil, errors.New("aes boom")
	})
	_, err := enc.Decrypt(ct, "w", "field")
	if err == nil || !strings.Contains(err.Error(), "create AES cipher") {
		t.Errorf("want wrapped AES error, got %v", err)
	}
}

func TestDecrypt_GCMError(t *testing.T) {
	enc := newTestEncryptor(t)
	ct := encryptForDecryptTest(t, enc, []byte("payload"))
	withGCMStub(t, func(_ cipher.Block) (cipher.AEAD, error) {
		return nil, errors.New("gcm boom")
	})
	_, err := enc.Decrypt(ct, "w", "field")
	if err == nil || !strings.Contains(err.Error(), "create GCM") {
		t.Errorf("want wrapped GCM error, got %v", err)
	}
}
