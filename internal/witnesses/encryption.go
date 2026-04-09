package witnesses

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

// ErrDecryptionFailed is returned when ciphertext fails GCM authentication.
var ErrDecryptionFailed = errors.New("decryption failed: authentication error")

// ErrMissingEncryptionKey is returned when no encryption key is configured.
var ErrMissingEncryptionKey = errors.New("WITNESS_ENCRYPTION_KEY is required")

// ErrUnsupportedKeyVersion is returned for unknown key version bytes.
var ErrUnsupportedKeyVersion = errors.New("unsupported key version")

const (
	keyVersionSize = 1
	nonceSize      = 12
	gcmTagSize     = 16
	derivedKeySize = 32 // AES-256

	// hkdfInfo is the fixed info string for HKDF derivation.
	hkdfInfo = "vaultkeeper-witness-identity"
)

// EncryptionKey holds a versioned master key for witness identity encryption.
type EncryptionKey struct {
	Version byte
	Key     []byte // raw master key (32 bytes for AES-256)
}

// Encryptor handles AES-256-GCM encryption of witness identity fields.
type Encryptor struct {
	keys map[byte]EncryptionKey // version → key
	current byte                // current key version for encryption
}

// NewEncryptor creates an encryptor with one or more versioned keys.
// The last key provided is used for new encryptions.
func NewEncryptor(keys ...EncryptionKey) (*Encryptor, error) {
	if len(keys) == 0 {
		return nil, ErrMissingEncryptionKey
	}

	keyMap := make(map[byte]EncryptionKey, len(keys))
	var currentVersion byte
	for _, k := range keys {
		if len(k.Key) < 16 {
			return nil, fmt.Errorf("encryption key version %d too short (min 16 bytes)", k.Version)
		}
		keyMap[k.Version] = k
		currentVersion = k.Version
	}

	return &Encryptor{
		keys:    keyMap,
		current: currentVersion,
	}, nil
}

// deriveKey uses HKDF-SHA256 to derive a unique key per witness from the master key.
func (e *Encryptor) deriveKey(masterKey []byte, witnessID string) ([]byte, error) {
	salt := []byte(witnessID)
	hkdfReader := hkdf.New(sha256.New, masterKey, salt, []byte(hkdfInfo))

	derived := make([]byte, derivedKeySize)
	if _, err := io.ReadFull(hkdfReader, derived); err != nil {
		// unreachable: hkdf.New returns an infinite SHA-256 HMAC reader that never errors.
		return nil, fmt.Errorf("derive witness key: %w", err)
	}
	return derived, nil
}

// Encrypt encrypts plaintext using AES-256-GCM with a per-witness+field derived key.
// Returns ciphertext in format: [1 byte version][12 bytes nonce][N bytes ciphertext][16 bytes GCM tag]
// The fieldName parameter binds the ciphertext to a specific column, preventing cross-field swaps.
func (e *Encryptor) Encrypt(plaintext []byte, witnessID, fieldName string) ([]byte, error) {
	key, ok := e.keys[e.current]
	if !ok {
		// unreachable: NewEncryptor always sets current to the last key version,
		// and current is only updated from keys stored in e.keys.
		return nil, fmt.Errorf("current key version %d not found", e.current)
	}

	derivedKey, err := e.deriveKey(key.Key, witnessID+":"+fieldName)
	if err != nil {
		// unreachable: deriveKey only errors when io.ReadFull fails on HKDF, which
		// cannot happen (see deriveKey comment above).
		return nil, err
	}

	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		// unreachable: HKDF always produces exactly derivedKeySize (32) bytes,
		// which is a valid AES-256 key length.
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		// unreachable: cipher.NewGCM only fails for non-standard block sizes;
		// AES blocks are always 16 bytes so this never fires.
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	// Encrypt: nonce is prepended to ciphertext by GCM Seal
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// Final format: [version][nonce][ciphertext+tag]
	result := make([]byte, 0, keyVersionSize+nonceSize+len(ciphertext))
	result = append(result, e.current)
	result = append(result, nonce...)
	result = append(result, ciphertext...)

	return result, nil
}

// Decrypt decrypts ciphertext using the key version indicated in the ciphertext header.
// The fieldName must match the one used during encryption for key derivation.
func (e *Encryptor) Decrypt(ciphertext []byte, witnessID, fieldName string) ([]byte, error) {
	minLen := keyVersionSize + nonceSize + gcmTagSize
	if len(ciphertext) < minLen {
		return nil, fmt.Errorf("ciphertext too short (min %d bytes, got %d)", minLen, len(ciphertext))
	}

	version := ciphertext[0]
	key, ok := e.keys[version]
	if !ok {
		return nil, fmt.Errorf("%w: version %d", ErrUnsupportedKeyVersion, version)
	}

	nonce := ciphertext[keyVersionSize : keyVersionSize+nonceSize]
	encrypted := ciphertext[keyVersionSize+nonceSize:]

	derivedKey, err := e.deriveKey(key.Key, witnessID+":"+fieldName)
	if err != nil {
		// unreachable: see deriveKey comment; HKDF reader never errors.
		return nil, err
	}

	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		// unreachable: HKDF always produces 32 bytes — valid AES-256 key.
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		// unreachable: cipher.NewGCM only fails for non-16-byte block ciphers;
		// AES always uses 16-byte blocks.
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// CurrentVersion returns the key version used for new encryptions.
func (e *Encryptor) CurrentVersion() byte {
	return e.current
}

// EncryptField encrypts a string field. Returns nil for nil input (null stays null).
// fieldName binds the ciphertext to a specific column to prevent cross-field swaps.
func (e *Encryptor) EncryptField(value *string, witnessID, fieldName string) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	return e.Encrypt([]byte(*value), witnessID, fieldName)
}

// DecryptField decrypts a byte field to a string pointer. Returns nil for nil input.
func (e *Encryptor) DecryptField(ciphertext []byte, witnessID, fieldName string) (*string, error) {
	if ciphertext == nil {
		return nil, nil
	}
	plaintext, err := e.Decrypt(ciphertext, witnessID, fieldName)
	if err != nil {
		return nil, err
	}
	s := string(plaintext)
	return &s, nil
}
