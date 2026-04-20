package federation

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/vaultkeeper/vaultkeeper/internal/migration"
)

// ManifestSigner signs and verifies exchange manifests using the
// instance's Ed25519 keypair.
type ManifestSigner interface {
	SignManifest(manifestHash []byte) []byte
	VerifyManifest(manifestHash []byte, signature []byte) bool
	PublicKeyBase64() string
	Fingerprint() string
}

type manifestSigner struct {
	signer *migration.Signer
}

// NewManifestSigner wraps an existing migration.Signer for manifest
// signing operations.
func NewManifestSigner(signer *migration.Signer) ManifestSigner {
	return &manifestSigner{signer: signer}
}

func (s *manifestSigner) SignManifest(manifestHash []byte) []byte {
	return s.signer.Sign(manifestHash)
}

func (s *manifestSigner) VerifyManifest(manifestHash []byte, signature []byte) bool {
	return s.signer.Verify(manifestHash, signature)
}

func (s *manifestSigner) PublicKeyBase64() string {
	return s.signer.PublicKeyBase64()
}

// Fingerprint returns "sha256:base64(sha256(pubkey))" — a
// human-readable identifier for the instance's public key.
func (s *manifestSigner) Fingerprint() string {
	return ComputeFingerprint(s.signer.PublicKey())
}

// ComputeFingerprint computes "sha256:base64(sha256(pubkey))" from
// a raw Ed25519 public key.
func ComputeFingerprint(pubkey []byte) string {
	h := sha256.Sum256(pubkey)
	return "sha256:" + base64.StdEncoding.EncodeToString(h[:])
}

// SignManifestHex signs a hex-encoded manifest hash, returning the
// raw Ed25519 signature. Convenience for the common flow where
// ComputeManifestHash returns hex.
func SignManifestHex(signer ManifestSigner, manifestHashHex string) ([]byte, error) {
	hashBytes, err := hex.DecodeString(manifestHashHex)
	if err != nil {
		return nil, fmt.Errorf("decode manifest hash hex: %w", err)
	}
	return signer.SignManifest(hashBytes), nil
}

// VerifyManifestHex verifies a hex-encoded manifest hash against a
// signature. Convenience for the common verification flow.
func VerifyManifestHex(signer ManifestSigner, manifestHashHex string, signature []byte) (bool, error) {
	hashBytes, err := hex.DecodeString(manifestHashHex)
	if err != nil {
		return false, fmt.Errorf("decode manifest hash hex: %w", err)
	}
	return signer.VerifyManifest(hashBytes, signature), nil
}
