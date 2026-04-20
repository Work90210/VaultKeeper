package federation

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"io"

	"github.com/vaultkeeper/vaultkeeper/pkg/vkverify"
)

// PeerTrustStore resolves a sender's instance ID to a trusted public key.
type PeerTrustStore interface {
	ResolvePublicKey(ctx context.Context, instanceID string) (ed25519.PublicKey, error)
}

// VerifyBundleWithTrustStore verifies a VKE1 bundle using a database-
// backed peer trust store to resolve the sender's public key. This is
// the in-app verification path — it wraps pkg/vkverify with peer
// resolution.
func VerifyBundleWithTrustStore(ctx context.Context, r io.ReaderAt, size int64, trustStore PeerTrustStore) (*vkverify.VerificationResult, error) {
	bundle, err := UnpackBundle(r, size)
	if err != nil {
		return nil, fmt.Errorf("parse bundle for peer resolution: %w", err)
	}

	pubkey, err := trustStore.ResolvePublicKey(ctx, bundle.Identity.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("resolve peer public key for %q: %w", bundle.Identity.InstanceID, err)
	}

	return vkverify.VerifyBundle(r, size, pubkey)
}
