package federation

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Trust modes for peer instances.
const (
	TrustModeUntrusted   = "untrusted"
	TrustModeTOFUPending = "tofu_pending"
	TrustModeManualPinned = "manual_pinned"
	TrustModePKIVerified = "pki_verified"
	TrustModeRevoked     = "revoked"
)

// PeerInstance represents a registered peer VaultKeeper instance.
type PeerInstance struct {
	ID                  uuid.UUID  `json:"id"`
	InstanceID          string     `json:"instance_id"`
	DisplayName         string     `json:"display_name"`
	WellKnownURL        string     `json:"well_known_url,omitempty"`
	TrustMode           string     `json:"trust_mode"`
	VerifiedBy          *string    `json:"verified_by,omitempty"`
	VerifiedAt          *time.Time `json:"verified_at,omitempty"`
	VerificationChannel *string    `json:"verification_channel,omitempty"`
	OrgID               *uuid.UUID `json:"org_id,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// PeerInstanceKey represents an Ed25519 public key for a peer.
type PeerInstanceKey struct {
	ID             uuid.UUID  `json:"id"`
	PeerInstanceID uuid.UUID  `json:"peer_instance_id"`
	PublicKey      []byte     `json:"-"`
	Fingerprint    string     `json:"fingerprint"`
	Status         string     `json:"status"`
	ValidFrom      time.Time  `json:"valid_from"`
	ValidTo        *time.Time `json:"valid_to,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// PeerStore manages peer instances and their keys.
type PeerStore interface {
	CreatePeer(ctx context.Context, peer PeerInstance) (PeerInstance, error)
	GetPeer(ctx context.Context, id uuid.UUID) (PeerInstance, error)
	GetPeerByInstanceID(ctx context.Context, instanceID string) (PeerInstance, error)
	ListPeers(ctx context.Context, orgID uuid.UUID) ([]PeerInstance, error)
	UpdateTrustMode(ctx context.Context, id uuid.UUID, trustMode string, verifiedBy string, channel string) error
	DeletePeer(ctx context.Context, id uuid.UUID) error

	AddKey(ctx context.Context, peerInstanceID uuid.UUID, publicKey []byte, fingerprint string) (PeerInstanceKey, error)
	GetActiveKey(ctx context.Context, peerInstanceID uuid.UUID) (PeerInstanceKey, error)
	RotateKey(ctx context.Context, peerInstanceID uuid.UUID, newKey []byte, newFingerprint string, rotSigOld, rotSigNew []byte) error
	ResolvePublicKey(ctx context.Context, instanceID string) (ed25519.PublicKey, error)
}

// PGPeerStore implements PeerStore with PostgreSQL.
type PGPeerStore struct {
	pool *pgxpool.Pool
}

// NewPeerStore creates a new PostgreSQL-backed peer store.
func NewPeerStore(pool *pgxpool.Pool) *PGPeerStore {
	return &PGPeerStore{pool: pool}
}

func (s *PGPeerStore) CreatePeer(ctx context.Context, peer PeerInstance) (PeerInstance, error) {
	peer.ID = uuid.New()
	peer.CreatedAt = time.Now().UTC()
	peer.UpdatedAt = peer.CreatedAt
	if peer.TrustMode == "" {
		peer.TrustMode = TrustModeUntrusted
	}

	_, err := s.pool.Exec(ctx,
		`INSERT INTO peer_instances (id, instance_id, display_name, well_known_url, trust_mode, org_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		peer.ID, peer.InstanceID, peer.DisplayName, peer.WellKnownURL,
		peer.TrustMode, peer.OrgID, peer.CreatedAt, peer.UpdatedAt)
	if err != nil {
		return PeerInstance{}, fmt.Errorf("insert peer: %w", err)
	}
	return peer, nil
}

func (s *PGPeerStore) GetPeer(ctx context.Context, id uuid.UUID) (PeerInstance, error) {
	var p PeerInstance
	err := s.pool.QueryRow(ctx,
		`SELECT id, instance_id, display_name, well_known_url, trust_mode,
		        verified_by, verified_at, verification_channel, org_id, created_at, updated_at
		 FROM peer_instances WHERE id = $1`, id).
		Scan(&p.ID, &p.InstanceID, &p.DisplayName, &p.WellKnownURL, &p.TrustMode,
			&p.VerifiedBy, &p.VerifiedAt, &p.VerificationChannel, &p.OrgID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return PeerInstance{}, fmt.Errorf("get peer %s: %w", id, err)
	}
	return p, nil
}

func (s *PGPeerStore) GetPeerByInstanceID(ctx context.Context, instanceID string) (PeerInstance, error) {
	var p PeerInstance
	err := s.pool.QueryRow(ctx,
		`SELECT id, instance_id, display_name, well_known_url, trust_mode,
		        verified_by, verified_at, verification_channel, org_id, created_at, updated_at
		 FROM peer_instances WHERE instance_id = $1`, instanceID).
		Scan(&p.ID, &p.InstanceID, &p.DisplayName, &p.WellKnownURL, &p.TrustMode,
			&p.VerifiedBy, &p.VerifiedAt, &p.VerificationChannel, &p.OrgID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return PeerInstance{}, fmt.Errorf("get peer by instance_id %q: %w", instanceID, err)
	}
	return p, nil
}

func (s *PGPeerStore) ListPeers(ctx context.Context, orgID uuid.UUID) ([]PeerInstance, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, instance_id, display_name, well_known_url, trust_mode,
		        verified_by, verified_at, verification_channel, org_id, created_at, updated_at
		 FROM peer_instances WHERE org_id = $1 ORDER BY display_name`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list peers: %w", err)
	}
	defer rows.Close()

	var peers []PeerInstance
	for rows.Next() {
		var p PeerInstance
		if err := rows.Scan(&p.ID, &p.InstanceID, &p.DisplayName, &p.WellKnownURL, &p.TrustMode,
			&p.VerifiedBy, &p.VerifiedAt, &p.VerificationChannel, &p.OrgID, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan peer: %w", err)
		}
		peers = append(peers, p)
	}
	return peers, nil
}

func (s *PGPeerStore) UpdateTrustMode(ctx context.Context, id uuid.UUID, trustMode string, verifiedBy string, channel string) error {
	now := time.Now().UTC()
	_, err := s.pool.Exec(ctx,
		`UPDATE peer_instances
		 SET trust_mode = $1, verified_by = $2, verified_at = $3, verification_channel = $4, updated_at = $5
		 WHERE id = $6`,
		trustMode, verifiedBy, now, channel, now, id)
	if err != nil {
		return fmt.Errorf("update trust mode: %w", err)
	}
	return nil
}

func (s *PGPeerStore) DeletePeer(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM peer_instances WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete peer: %w", err)
	}
	return nil
}

func (s *PGPeerStore) AddKey(ctx context.Context, peerInstanceID uuid.UUID, publicKey []byte, fingerprint string) (PeerInstanceKey, error) {
	key := PeerInstanceKey{
		ID:             uuid.New(),
		PeerInstanceID: peerInstanceID,
		PublicKey:      publicKey,
		Fingerprint:    fingerprint,
		Status:         "active",
		ValidFrom:      time.Now().UTC(),
		CreatedAt:      time.Now().UTC(),
	}

	_, err := s.pool.Exec(ctx,
		`INSERT INTO peer_instance_keys (id, peer_instance_id, public_key, fingerprint, status, valid_from, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		key.ID, key.PeerInstanceID, key.PublicKey, key.Fingerprint, key.Status, key.ValidFrom, key.CreatedAt)
	if err != nil {
		return PeerInstanceKey{}, fmt.Errorf("add key: %w", err)
	}
	return key, nil
}

func (s *PGPeerStore) GetActiveKey(ctx context.Context, peerInstanceID uuid.UUID) (PeerInstanceKey, error) {
	var k PeerInstanceKey
	err := s.pool.QueryRow(ctx,
		`SELECT id, peer_instance_id, public_key, fingerprint, status, valid_from, valid_to, created_at
		 FROM peer_instance_keys
		 WHERE peer_instance_id = $1 AND status = 'active'`, peerInstanceID).
		Scan(&k.ID, &k.PeerInstanceID, &k.PublicKey, &k.Fingerprint, &k.Status, &k.ValidFrom, &k.ValidTo, &k.CreatedAt)
	if err != nil {
		return PeerInstanceKey{}, fmt.Errorf("get active key for peer %s: %w", peerInstanceID, err)
	}
	return k, nil
}

func (s *PGPeerStore) RotateKey(ctx context.Context, peerInstanceID uuid.UUID, newKey []byte, newFingerprint string, rotSigOld, rotSigNew []byte) error {
	// Verify the rotation is authorized by the current active key.
	currentKey, err := s.GetActiveKey(ctx, peerInstanceID)
	if err != nil {
		return fmt.Errorf("get current active key: %w", err)
	}

	if len(currentKey.PublicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("current active key has invalid size: %d", len(currentKey.PublicKey))
	}

	// Old key must sign the new key to authorize rotation.
	if len(rotSigOld) == 0 {
		return fmt.Errorf("rotation signature from old key is required")
	}
	if !ed25519.Verify(ed25519.PublicKey(currentKey.PublicKey), newKey, rotSigOld) {
		return fmt.Errorf("rotation signature from old key is invalid")
	}

	// New key must sign a statement to prove possession.
	if len(rotSigNew) == 0 {
		return fmt.Errorf("rotation signature from new key is required")
	}
	if len(newKey) != ed25519.PublicKeySize {
		return fmt.Errorf("new key has invalid size: %d", len(newKey))
	}
	rotationStatement := []byte("VK:KEY_ROTATION:" + currentKey.Fingerprint + ":" + newFingerprint)
	if !ed25519.Verify(ed25519.PublicKey(newKey), rotationStatement, rotSigNew) {
		return fmt.Errorf("rotation signature from new key is invalid")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	now := time.Now().UTC()

	// Deactivate current active key.
	_, err = tx.Exec(ctx,
		`UPDATE peer_instance_keys
		 SET status = 'rotated', valid_to = $1
		 WHERE peer_instance_id = $2 AND status = 'active'`,
		now, peerInstanceID)
	if err != nil {
		return fmt.Errorf("deactivate old key: %w", err)
	}

	// Insert new active key.
	_, err = tx.Exec(ctx,
		`INSERT INTO peer_instance_keys (id, peer_instance_id, public_key, fingerprint, status, valid_from, rotation_sig_old, rotation_sig_new, created_at)
		 VALUES ($1, $2, $3, $4, 'active', $5, $6, $7, $8)`,
		uuid.New(), peerInstanceID, newKey, newFingerprint, now, rotSigOld, rotSigNew, now)
	if err != nil {
		return fmt.Errorf("insert new key: %w", err)
	}

	return tx.Commit(ctx)
}

// ResolvePublicKey implements PeerTrustStore by looking up the peer's
// active key by instance ID. This is used by the in-app verifier.
func (s *PGPeerStore) ResolvePublicKey(ctx context.Context, instanceID string) (ed25519.PublicKey, error) {
	peer, err := s.GetPeerByInstanceID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("resolve peer %q: %w", instanceID, err)
	}

	if peer.TrustMode == TrustModeUntrusted || peer.TrustMode == TrustModeRevoked {
		return nil, fmt.Errorf("peer %q is not trusted (mode: %s)", instanceID, peer.TrustMode)
	}

	key, err := s.GetActiveKey(ctx, peer.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("no active key for peer %q", instanceID)
		}
		return nil, err
	}

	if len(key.PublicKey) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid key size for peer %q: %d bytes", instanceID, len(key.PublicKey))
	}


	return ed25519.PublicKey(key.PublicKey), nil
}
