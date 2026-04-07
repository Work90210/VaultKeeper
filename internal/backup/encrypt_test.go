package backup

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestEncryptDecryptRoundtrip(t *testing.T) {
	tests := []struct {
		name      string
		plaintext string
		key       []byte
	}{
		{
			name:      "short message",
			plaintext: "hello world",
			key:       []byte("my-secret-key"),
		},
		{
			name:      "exactly 32 byte key",
			plaintext: "some data to encrypt with exact key size",
			key:       bytes.Repeat([]byte("A"), 32),
		},
		{
			name:      "empty input",
			plaintext: "",
			key:       []byte("key"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			encrypted, err := Encrypt(strings.NewReader(tc.plaintext), tc.key)
			if err != nil {
				t.Fatalf("Encrypt: %v", err)
			}

			decrypted, err := Decrypt(encrypted, tc.key)
			if err != nil {
				t.Fatalf("Decrypt: %v", err)
			}

			got, err := io.ReadAll(decrypted)
			if err != nil {
				t.Fatalf("ReadAll: %v", err)
			}

			if string(got) != tc.plaintext {
				t.Errorf("roundtrip mismatch: got %q, want %q", got, tc.plaintext)
			}
		})
	}
}

func TestEncryptMultipleChunks(t *testing.T) {
	// Create data larger than chunkSize (64 KiB) to exercise multi-chunk path.
	data := make([]byte, chunkSize*3+1234)
	if _, err := rand.Read(data); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}

	key := []byte("multi-chunk-key")

	encrypted, err := Encrypt(bytes.NewReader(data), key)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	decrypted, err := Decrypt(encrypted, key)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	got, err := io.ReadAll(decrypted)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	if !bytes.Equal(got, data) {
		t.Error("multi-chunk roundtrip produced different data")
	}
}

func TestEncryptEmptyKeyReturnsError(t *testing.T) {
	_, err := Encrypt(strings.NewReader("data"), []byte{})
	if err == nil {
		t.Fatal("expected error for empty key")
	}
	if !strings.Contains(err.Error(), "encryption key must not be empty") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDecryptEmptyKeyReturnsError(t *testing.T) {
	_, err := Decrypt(strings.NewReader("data"), []byte{})
	if err == nil {
		t.Fatal("expected error for empty key")
	}
	if !strings.Contains(err.Error(), "decryption key must not be empty") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDecryptWrongKeyFails(t *testing.T) {
	plaintext := "sensitive data"
	correctKey := []byte("correct-key")
	wrongKey := []byte("wrong-key")

	encrypted, err := Encrypt(strings.NewReader(plaintext), correctKey)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Read all encrypted bytes so we can try decrypting with wrong key.
	encBytes, _ := io.ReadAll(encrypted)

	_, err = Decrypt(bytes.NewReader(encBytes), wrongKey)
	if err == nil {
		t.Fatal("expected error when decrypting with wrong key")
	}
}

func TestDecryptTamperedCiphertextFails(t *testing.T) {
	encrypted, err := Encrypt(strings.NewReader("important data"), []byte("key"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	encBytes, _ := io.ReadAll(encrypted)

	// Tamper with a byte near the end of the ciphertext (past the header).
	if len(encBytes) > 60 {
		encBytes[len(encBytes)-5] ^= 0xff
	}

	_, err = Decrypt(bytes.NewReader(encBytes), []byte("key"))
	if err == nil {
		t.Fatal("expected error when decrypting tampered ciphertext")
	}
}

func TestDecryptOversizedChunkLengthFails(t *testing.T) {
	// Construct a valid header followed by an oversized chunk length.
	key := []byte("key")
	encrypted, err := Encrypt(strings.NewReader("x"), key)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	encBytes, _ := io.ReadAll(encrypted)

	// The header is binary.Size(header{}) bytes. After the header comes the
	// 4-byte chunk length. Overwrite it with a value exceeding 1 MiB.
	headerSize := binary.Size(header{})
	if len(encBytes) < headerSize+4 {
		t.Fatal("encrypted data too short")
	}

	binary.BigEndian.PutUint32(encBytes[headerSize:headerSize+4], (1<<20)+1)

	_, err = Decrypt(bytes.NewReader(encBytes), key)
	if err == nil {
		t.Fatal("expected error for oversized chunk length")
	}
	if !strings.Contains(err.Error(), "exceeds maximum") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHeaderContainsCorrectMagicAndVersion(t *testing.T) {
	encrypted, err := Encrypt(strings.NewReader("data"), []byte("key"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	encBytes, _ := io.ReadAll(encrypted)

	// Check magic bytes.
	if string(encBytes[:4]) != headerMagic {
		t.Errorf("magic bytes: got %q, want %q", encBytes[:4], headerMagic)
	}

	// Check version (5th byte).
	if encBytes[4] != headerVersion {
		t.Errorf("version: got %d, want %d", encBytes[4], headerVersion)
	}
}

func TestDecrypt_InvalidHeader(t *testing.T) {
	// Decrypt should return an error when the header has bad magic bytes.
	_, err := Decrypt(strings.NewReader("XXXX" + strings.Repeat("\x00", 45)), []byte("key"))
	if err == nil {
		t.Fatal("expected error for bad header in Decrypt")
	}
	if !strings.Contains(err.Error(), "bad magic bytes") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReadHeaderBadMagic(t *testing.T) {
	// Write a header with bad magic bytes.
	var buf bytes.Buffer
	buf.Write([]byte("XXXX"))                         // bad magic
	buf.WriteByte(1)                                   // version
	buf.Write(make([]byte, 12))                        // algorithm
	buf.Write(make([]byte, 32))                        // salt

	_, err := readHeader(&buf)
	if err == nil {
		t.Fatal("expected error for bad magic")
	}
	if !strings.Contains(err.Error(), "bad magic bytes") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReadHeaderBadVersion(t *testing.T) {
	var h header
	copy(h.Magic[:], headerMagic)
	h.Version = 99 // unsupported version

	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.BigEndian, h); err != nil {
		t.Fatalf("write header: %v", err)
	}

	_, err := readHeader(&buf)
	if err == nil {
		t.Fatal("expected error for bad version")
	}
	if !strings.Contains(err.Error(), "unsupported backup version") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReadHeaderTruncated(t *testing.T) {
	_, err := readHeader(bytes.NewReader([]byte("VK")))
	if err == nil {
		t.Fatal("expected error for truncated header")
	}
}

func TestDecryptChunkTooShortForNonce(t *testing.T) {
	// Build valid header + a chunk whose sealed length is less than nonceLen.
	key := []byte("test-key")
	salt := make([]byte, saltLen)

	var buf bytes.Buffer
	if err := writeHeader(&buf, salt); err != nil {
		t.Fatalf("writeHeader: %v", err)
	}

	// Write a chunk of length 5 (< nonceLen=12).
	chunkLen := make([]byte, 4)
	binary.BigEndian.PutUint32(chunkLen, 5)
	buf.Write(chunkLen)
	buf.Write([]byte("short"))

	_, err := Decrypt(bytes.NewReader(buf.Bytes()), key)
	if err == nil {
		t.Fatal("expected error for chunk too short for nonce")
	}
	if !strings.Contains(err.Error(), "too short for nonce") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDeriveKeyAlwaysRunsPBKDF2(t *testing.T) {
	salt := make([]byte, 32)
	key32 := bytes.Repeat([]byte("A"), 32)

	// deriveKey with a 32-byte key should still derive (not pass through).
	derived := deriveKey(key32, salt)
	if bytes.Equal(derived, key32) {
		t.Error("deriveKey returned raw key; expected PBKDF2 derivation")
	}
	if len(derived) != 32 {
		t.Errorf("derived key length: got %d, want 32", len(derived))
	}
}

// --- randReader error paths ---

type failingReader struct {
	bytesBeforeError int
	bytesRead        int
}

func (f *failingReader) Read(p []byte) (int, error) {
	remaining := f.bytesBeforeError - f.bytesRead
	if remaining <= 0 {
		return 0, fmt.Errorf("simulated rand failure")
	}
	n := len(p)
	if n > remaining {
		n = remaining
	}
	for i := 0; i < n; i++ {
		p[i] = 0xAA
	}
	f.bytesRead += n
	return n, nil
}

func TestEncrypt_SaltGenerationError(t *testing.T) {
	original := randReader
	defer func() { randReader = original }()

	// Fail immediately (before salt can be read).
	randReader = &failingReader{bytesBeforeError: 0}

	_, err := Encrypt(strings.NewReader("data"), []byte("key"))
	if err == nil {
		t.Fatal("expected error when salt generation fails")
	}
	if !strings.Contains(err.Error(), "generate salt") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestEncrypt_NonceGenerationError(t *testing.T) {
	original := randReader
	defer func() { randReader = original }()

	// Allow salt (32 bytes) to succeed, but fail on nonce (12 bytes).
	randReader = &failingReader{bytesBeforeError: saltLen}

	_, err := Encrypt(strings.NewReader("data"), []byte("key"))
	if err == nil {
		t.Fatal("expected error when nonce generation fails")
	}
	if !strings.Contains(err.Error(), "generate nonce") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestEncrypt_ReadPlaintextError(t *testing.T) {
	errReader := &errAfterNReader{data: []byte("partial"), err: fmt.Errorf("disk I/O error")}

	_, err := Encrypt(errReader, []byte("key"))
	if err == nil {
		t.Fatal("expected error when reading plaintext fails")
	}
	if !strings.Contains(err.Error(), "read plaintext") {
		t.Errorf("unexpected error: %v", err)
	}
}

// errAfterNReader returns data first, then an error (not EOF).
type errAfterNReader struct {
	data []byte
	pos  int
	err  error
}

func (r *errAfterNReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, r.err
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	// Return data and error simultaneously on last read.
	if r.pos >= len(r.data) {
		return n, r.err
	}
	return n, nil
}

func TestDecrypt_TruncatedChunkData(t *testing.T) {
	// Build valid header + chunk length that claims more data than available.
	key := []byte("test-key")
	salt := make([]byte, saltLen)

	var buf bytes.Buffer
	if err := writeHeader(&buf, salt); err != nil {
		t.Fatalf("writeHeader: %v", err)
	}

	// Claim 100 bytes of chunk data but only provide 5.
	chunkLen := make([]byte, 4)
	binary.BigEndian.PutUint32(chunkLen, 100)
	buf.Write(chunkLen)
	buf.Write([]byte("short"))

	_, err := Decrypt(bytes.NewReader(buf.Bytes()), key)
	if err == nil {
		t.Fatal("expected error for truncated chunk data")
	}
	if !strings.Contains(err.Error(), "read encrypted chunk") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDecrypt_TruncatedChunkLength(t *testing.T) {
	// Build valid header + partial (2-byte) chunk length.
	key := []byte("test-key")
	salt := make([]byte, saltLen)

	var buf bytes.Buffer
	if err := writeHeader(&buf, salt); err != nil {
		t.Fatalf("writeHeader: %v", err)
	}

	// Only 2 bytes of a 4-byte length prefix — but this is "unexpected EOF"
	// which is treated like EOF, so no error.
	buf.Write([]byte{0x00, 0x01})

	_, err := Decrypt(bytes.NewReader(buf.Bytes()), key)
	// Partial chunk length is treated as end-of-stream.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDecrypt_ReadChunkLengthError(t *testing.T) {
	// Build valid header then provide a reader that returns a non-EOF error
	// when reading the chunk length.
	key := []byte("test-key")
	salt := make([]byte, saltLen)

	var headerBuf bytes.Buffer
	if err := writeHeader(&headerBuf, salt); err != nil {
		t.Fatalf("writeHeader: %v", err)
	}

	headerBytes := headerBuf.Bytes()

	// Create a reader that returns the header successfully, then errors
	// with a non-EOF error on subsequent reads.
	r := &headerThenErrorReader{
		header: headerBytes,
		err:    fmt.Errorf("network error"),
	}

	_, err := Decrypt(r, key)
	if err == nil {
		t.Fatal("expected error for read chunk length failure")
	}
	if !strings.Contains(err.Error(), "read chunk length") {
		t.Errorf("unexpected error: %v", err)
	}
}

// headerThenErrorReader returns header bytes first, then a custom error.
type headerThenErrorReader struct {
	header []byte
	pos    int
	err    error
}

func (r *headerThenErrorReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.header) {
		return 0, r.err
	}
	n := copy(p, r.header[r.pos:])
	r.pos += n
	return n, nil
}
