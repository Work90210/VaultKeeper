package integrity

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
)

// HashVerifier interface for verifying file integrity.
type HashVerifier interface {
	VerifyHash(ctx context.Context, algorithm string, expected string, actual string) error
}

// FileReader abstracts reading objects from storage (e.g., MinIO).
type FileReader interface {
	GetObject(ctx context.Context, key string) (io.ReadCloser, error)
}

// ComputeSHA256 reads from r and returns the hex-encoded SHA-256 hash.
func ComputeSHA256(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", fmt.Errorf("compute SHA-256: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyFileHash reads the file from storage, computes its SHA-256 hash,
// and compares it against the expected value.
func VerifyFileHash(ctx context.Context, reader FileReader, storageKey, expectedHash string) (string, error) {
	rc, err := reader.GetObject(ctx, storageKey)
	if err != nil {
		return "", fmt.Errorf("read object %q: %w", storageKey, err)
	}
	defer rc.Close()

	computed, err := ComputeSHA256(rc)
	if err != nil {
		return "", fmt.Errorf("hash object %q: %w", storageKey, err)
	}

	if subtle.ConstantTimeCompare([]byte(computed), []byte(expectedHash)) != 1 {
		return computed, fmt.Errorf("hash mismatch for %q: expected %s, got %s", storageKey, expectedHash, computed)
	}

	return computed, nil
}
