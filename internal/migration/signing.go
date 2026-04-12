package migration

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"sync"
)

// Signer signs attestation certificate bodies with an Ed25519 keypair.
//
// The private key is loaded from the INSTANCE_ED25519_KEY environment
// variable (base64-encoded 64-byte seed+public ed25519 private key). For
// development, LoadOrGenerate generates an ephemeral keypair in memory so
// unit tests and local dev work without key provisioning.
type Signer struct {
	priv ed25519.PrivateKey
	pub  ed25519.PublicKey
}

// ed25519GenerateKey is the indirection point tests use to inject a
// failing key generator. Production always uses crypto/rand.
var ed25519GenerateKey = ed25519.GenerateKey

// LoadOrGenerate reads the signing key from the environment, or generates
// an ephemeral keypair if missing. In production the environment variable
// MUST be set — startup code should verify this with RequireConfiguredKey.
func LoadOrGenerate() (*Signer, error) {
	raw := os.Getenv("INSTANCE_ED25519_KEY")
	if raw == "" {
		pub, priv, err := ed25519GenerateKey(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("generate ephemeral signing key: %w", err)
		}
		return &Signer{priv: priv, pub: pub}, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("decode INSTANCE_ED25519_KEY: %w", err)
	}
	if len(decoded) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("INSTANCE_ED25519_KEY: expected %d bytes, got %d", ed25519.PrivateKeySize, len(decoded))
	}
	priv := ed25519.PrivateKey(decoded)
	// ed25519.PrivateKey.Public() is documented to always return
	// ed25519.PublicKey for any private key of the correct size (which
	// we just validated). Direct assertion is idiomatic.
	pub := priv.Public().(ed25519.PublicKey)
	return &Signer{priv: priv, pub: pub}, nil
}

// RequireConfiguredKey returns an error if the signer was initialised from
// an ephemeral (non-env) keypair. Production wiring should call this once
// during startup to refuse to boot without a configured key.
func RequireConfiguredKey() error {
	if os.Getenv("INSTANCE_ED25519_KEY") == "" {
		return errors.New("INSTANCE_ED25519_KEY is not set; refusing to sign attestation certificates with an ephemeral key")
	}
	return nil
}

// Sign returns an Ed25519 signature over the supplied body.
func (s *Signer) Sign(body []byte) []byte {
	return ed25519.Sign(s.priv, body)
}

// Verify returns true if sig is a valid signature over body.
func (s *Signer) Verify(body, sig []byte) bool {
	return ed25519.Verify(s.pub, body, sig)
}

// PublicKeyBase64 returns the base64-encoded public key for the
// /.well-known/vaultkeeper-signing-key endpoint.
func (s *Signer) PublicKeyBase64() string {
	return base64.StdEncoding.EncodeToString(s.pub)
}

// PublicKey returns the raw public key bytes.
func (s *Signer) PublicKey() ed25519.PublicKey {
	return s.pub
}

// GenerateKeyBase64 returns a freshly generated private key encoded as
// base64, suitable for populating INSTANCE_ED25519_KEY in production. Used
// by the CLI's `vaultkeeper migrate genkey` helper.
func GenerateKeyBase64() (string, error) {
	_, priv, err := ed25519GenerateKey(rand.Reader)
	if err != nil {
		return "", fmt.Errorf("generate key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(priv), nil
}

// --- package-level singleton (thin convenience) ---

var (
	defaultSignerOnce sync.Once
	defaultSigner     *Signer
	defaultSignerErr  error
)

// DefaultSigner returns a process-wide signer initialised lazily via
// LoadOrGenerate. Tests get a fresh signer per call via LoadOrGenerate
// directly.
func DefaultSigner() (*Signer, error) {
	defaultSignerOnce.Do(func() {
		defaultSigner, defaultSignerErr = LoadOrGenerate()
	})
	return defaultSigner, defaultSignerErr
}
