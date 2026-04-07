package integrity

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// FileReader fake
// ---------------------------------------------------------------------------

type fakeFileReader struct {
	content []byte
	err     error
}

func (f *fakeFileReader) GetObject(_ context.Context, _ string) (io.ReadCloser, error) {
	if f.err != nil {
		return nil, f.err
	}
	return io.NopCloser(bytes.NewReader(f.content)), nil
}

// fakeFileReaderReadErr returns a reader that errors during reading.
type fakeFileReaderReadErr struct{}

func (f *fakeFileReaderReadErr) GetObject(_ context.Context, _ string) (io.ReadCloser, error) {
	return io.NopCloser(&errorReader{}), nil
}

type errorReader struct{}

func (e *errorReader) Read(_ []byte) (int, error) {
	return 0, errors.New("read error")
}

// ---------------------------------------------------------------------------
// ComputeSHA256 tests
// ---------------------------------------------------------------------------

func TestComputeSHA256_EmptyInput(t *testing.T) {
	hash, err := ComputeSHA256(bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// SHA-256 of empty string
	expected := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if hash != expected {
		t.Errorf("hash = %q, want %q", hash, expected)
	}
}

func TestComputeSHA256_KnownInput(t *testing.T) {
	hash, err := ComputeSHA256(strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if hash != expected {
		t.Errorf("hash = %q, want %q", hash, expected)
	}
}

func TestComputeSHA256_ReadError(t *testing.T) {
	_, err := ComputeSHA256(&errorReader{})
	if err == nil {
		t.Fatal("expected error from read failure")
	}
}

func TestComputeSHA256_LargeInput(t *testing.T) {
	data := make([]byte, 1024*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}
	hash, err := ComputeSHA256(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hash) != 64 {
		t.Errorf("hash length = %d, want 64", len(hash))
	}
}

// ---------------------------------------------------------------------------
// VerifyFileHash tests
// ---------------------------------------------------------------------------

func TestVerifyFileHash_Match(t *testing.T) {
	content := []byte("test content")
	// Pre-compute hash
	expectedHash, _ := ComputeSHA256(bytes.NewReader(content))

	reader := &fakeFileReader{content: content}
	computed, err := VerifyFileHash(context.Background(), reader, "key", expectedHash)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if computed != expectedHash {
		t.Errorf("computed = %q, want %q", computed, expectedHash)
	}
}

func TestVerifyFileHash_Mismatch(t *testing.T) {
	content := []byte("test content")
	reader := &fakeFileReader{content: content}

	computed, err := VerifyFileHash(context.Background(), reader, "key", "wrong-hash")
	if err == nil {
		t.Fatal("expected error for hash mismatch")
	}
	if computed == "" {
		t.Error("expected computed hash to be returned on mismatch")
	}
	if !strings.Contains(err.Error(), "hash mismatch") {
		t.Errorf("error = %q, want to contain 'hash mismatch'", err.Error())
	}
}

func TestVerifyFileHash_GetObjectError(t *testing.T) {
	reader := &fakeFileReader{err: errors.New("object not found")}

	computed, err := VerifyFileHash(context.Background(), reader, "missing-key", "hash")
	if err == nil {
		t.Fatal("expected error for GetObject failure")
	}
	if computed != "" {
		t.Errorf("computed should be empty on read error, got %q", computed)
	}
	if !strings.Contains(err.Error(), "read object") {
		t.Errorf("error = %q, want to contain 'read object'", err.Error())
	}
}

func TestVerifyFileHash_HashComputeError(t *testing.T) {
	reader := &fakeFileReaderReadErr{}

	computed, err := VerifyFileHash(context.Background(), reader, "key", "hash")
	if err == nil {
		t.Fatal("expected error for hash computation failure")
	}
	if computed != "" {
		t.Errorf("computed should be empty on hash error, got %q", computed)
	}
	if !strings.Contains(err.Error(), "hash object") {
		t.Errorf("error = %q, want to contain 'hash object'", err.Error())
	}
}
