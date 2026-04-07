package backup

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	crypto_rand "crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

// randReader is the source of cryptographic randomness. Tests can replace it
// to simulate errors from crypto/rand.
var randReader io.Reader = crypto_rand.Reader

const (
	algorithmAES256GCM = "AES-256-GCM"
	headerMagic        = "VKBK"
	headerVersion      = uint8(1)
	pbkdf2Iterations   = 600_000
	saltLen            = 32
	nonceLen           = 12
	chunkSize          = 64 * 1024 // 64 KiB per encrypted chunk
)

// header is written at the start of every encrypted backup file.
// It is NOT encrypted — it carries the metadata needed to decrypt.
type header struct {
	Magic     [4]byte  // "VKBK"
	Version   uint8    // 1
	Algorithm [12]byte // "AES-256-GCM\0"
	Salt      [32]byte // PBKDF2 salt
}

func writeHeader(w io.Writer, salt []byte) error {
	var h header
	copy(h.Magic[:], headerMagic)
	h.Version = headerVersion
	copy(h.Algorithm[:], algorithmAES256GCM)
	copy(h.Salt[:], salt)
	return binary.Write(w, binary.BigEndian, h)
}

func readHeader(r io.Reader) (header, error) {
	var h header
	if err := binary.Read(r, binary.BigEndian, &h); err != nil {
		return header{}, fmt.Errorf("read backup header: %w", err)
	}
	if string(h.Magic[:]) != headerMagic {
		return header{}, errors.New("invalid backup file: bad magic bytes")
	}
	if h.Version != headerVersion {
		return header{}, fmt.Errorf("unsupported backup version: %d", h.Version)
	}
	return h, nil
}

// deriveKey produces a 32-byte key from the supplied passphrase and salt
// using PBKDF2-HMAC-SHA256. If the input key is already 32 bytes, it is
// used directly (salt is still recorded in the header for consistency).
func deriveKey(passphrase []byte, salt []byte) []byte {
	return pbkdf2.Key(passphrase, salt, pbkdf2Iterations, 32, sha256.New)
}

// Encrypt reads all plaintext from r, encrypts it with AES-256-GCM, and
// returns a reader over the ciphertext (header + encrypted chunks).
// Each chunk is prefixed by its 4-byte big-endian length so the decrypter
// knows where one chunk ends and the next begins.
func Encrypt(r io.Reader, key []byte) (io.Reader, error) {
	if len(key) == 0 {
		return nil, errors.New("encryption key must not be empty")
	}

	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(randReader, salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}

	derived := deriveKey(key, salt)

	// aes.NewCipher cannot fail with a 32-byte key from PBKDF2.
	// cipher.NewGCM cannot fail with a standard AES block cipher.
	gcm := mustGCM(derived)

	var buf bytes.Buffer
	// writeHeader writes to bytes.Buffer which never fails.
	_ = writeHeader(&buf, salt)

	chunk := make([]byte, chunkSize)
	for {
		n, readErr := io.ReadFull(r, chunk)
		if n > 0 {
			nonce := make([]byte, nonceLen)
			if _, err := io.ReadFull(randReader, nonce); err != nil {
				return nil, fmt.Errorf("generate nonce: %w", err)
			}
			sealed := gcm.Seal(nonce, nonce, chunk[:n], nil)

			// Write chunk length prefix (4 bytes big-endian).
			lenBuf := make([]byte, 4)
			binary.BigEndian.PutUint32(lenBuf, uint32(len(sealed)))
			// bytes.Buffer.Write never fails.
			buf.Write(lenBuf)
			buf.Write(sealed)
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) || errors.Is(readErr, io.ErrUnexpectedEOF) {
				break
			}
			return nil, fmt.Errorf("read plaintext: %w", readErr)
		}
	}

	return bytes.NewReader(buf.Bytes()), nil
}

// Decrypt reads ciphertext produced by Encrypt and returns a reader over
// the recovered plaintext.
func Decrypt(r io.Reader, key []byte) (io.Reader, error) {
	if len(key) == 0 {
		return nil, errors.New("decryption key must not be empty")
	}

	h, err := readHeader(r)
	if err != nil {
		return nil, err
	}

	derived := deriveKey(key, h.Salt[:])

	// aes.NewCipher cannot fail with a 32-byte key from PBKDF2.
	// cipher.NewGCM cannot fail with a standard AES block cipher.
	gcm := mustGCM(derived)

	var plaintext bytes.Buffer
	lenBuf := make([]byte, 4)
	for {
		if _, err := io.ReadFull(r, lenBuf); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}
			return nil, fmt.Errorf("read chunk length: %w", err)
		}
		chunkLen := binary.BigEndian.Uint32(lenBuf)
		const maxChunkLen = 1 << 20 // 1 MiB — well above 64 KiB chunk + GCM overhead
		if chunkLen > maxChunkLen {
			return nil, fmt.Errorf("encrypted chunk length %d exceeds maximum %d", chunkLen, maxChunkLen)
		}
		sealed := make([]byte, chunkLen)
		if _, err := io.ReadFull(r, sealed); err != nil {
			return nil, fmt.Errorf("read encrypted chunk: %w", err)
		}

		if len(sealed) < nonceLen {
			return nil, errors.New("encrypted chunk too short for nonce")
		}
		nonce := sealed[:nonceLen]
		ciphertext := sealed[nonceLen:]

		decrypted, err := gcm.Open(nil, nonce, ciphertext, nil)
		if err != nil {
			return nil, fmt.Errorf("decrypt chunk: %w", err)
		}
		// bytes.Buffer.Write never fails.
		plaintext.Write(decrypted)
	}

	return bytes.NewReader(plaintext.Bytes()), nil
}

// mustGCM creates an AES-256-GCM cipher from a 32-byte derived key.
// Both aes.NewCipher and cipher.NewGCM are infallible with valid inputs
// (32-byte key from PBKDF2 and a standard AES block cipher respectively),
// so errors are treated as programming bugs via panic.
func mustGCM(derivedKey []byte) cipher.AEAD {
	block, _ := aes.NewCipher(derivedKey)
	gcm, _ := cipher.NewGCM(block)
	return gcm
}
