package apikeys

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrKeyNotFound = errors.New("api key not found")
	ErrNotOwner    = errors.New("api key not owned by user")
)

// Repository defines the data-access contract for API keys.
type Repository interface {
	Create(ctx context.Context, userID string, input CreateKeyInput) (CreateKeyResult, error)
	ListByUser(ctx context.Context, userID string) ([]APIKey, error)
	Revoke(ctx context.Context, keyID uuid.UUID, userID string) error
}

// PGRepository implements Repository backed by PostgreSQL.
type PGRepository struct {
	pool *pgxpool.Pool
}

// NewRepository returns a new PostgreSQL-backed API key repository.
func NewRepository(pool *pgxpool.Pool) *PGRepository {
	return &PGRepository{pool: pool}
}

// generateKey produces a cryptographically random key with the vk_ prefix
// and returns both the raw key string and its SHA-256 hash.
func generateKey() (raw string, hash string, err error) {
	b := make([]byte, 16) // 16 bytes = 32 hex chars
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate random bytes: %w", err)
	}
	raw = KeyPrefix + hex.EncodeToString(b)
	h := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(h[:])
	return raw, hash, nil
}

// Create generates a new API key, stores its hash, and returns the raw key once.
// A user may hold at most 20 active (non-revoked) keys at a time.
// The count check and insert are performed inside a single transaction to prevent
// a TOCTOU race that would allow more than 20 keys to be created concurrently.
func (r *PGRepository) Create(ctx context.Context, userID string, input CreateKeyInput) (CreateKeyResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return CreateKeyResult{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var count int
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM api_keys WHERE user_id = $1 AND revoked_at IS NULL`,
		userID,
	).Scan(&count); err != nil {
		return CreateKeyResult{}, fmt.Errorf("count api keys: %w", err)
	}
	if count >= 20 {
		return CreateKeyResult{}, fmt.Errorf("maximum number of API keys reached (20)")
	}

	rawKey, keyHash, err := generateKey()
	if err != nil {
		return CreateKeyResult{}, fmt.Errorf("create api key: %w", err)
	}

	perm := input.Permissions
	if perm != "read" && perm != "read_write" {
		perm = "read"
	}

	var key APIKey
	err = tx.QueryRow(ctx,
		`INSERT INTO api_keys (user_id, name, key_hash, permissions)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, user_id, name, permissions, last_used_at, revoked_at, created_at`,
		userID, input.Name, keyHash, perm,
	).Scan(&key.ID, &key.UserID, &key.Name, &key.Permissions,
		&key.LastUsedAt, &key.RevokedAt, &key.CreatedAt)
	if err != nil {
		return CreateKeyResult{}, fmt.Errorf("insert api key: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return CreateKeyResult{}, fmt.Errorf("commit api key creation: %w", err)
	}

	return CreateKeyResult{Key: key, RawKey: rawKey}, nil
}

// ListByUser returns all non-revoked API keys for the given user.
func (r *PGRepository) ListByUser(ctx context.Context, userID string) ([]APIKey, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, name, permissions, last_used_at, revoked_at, created_at
		 FROM api_keys
		 WHERE user_id = $1 AND revoked_at IS NULL
		 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.ID, &k.UserID, &k.Name, &k.Permissions,
			&k.LastUsedAt, &k.RevokedAt, &k.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api keys: %w", err)
	}

	if keys == nil {
		keys = []APIKey{}
	}
	return keys, nil
}

// Revoke sets revoked_at on the key, but only if owned by the given user.
func (r *PGRepository) Revoke(ctx context.Context, keyID uuid.UUID, userID string) error {
	// First verify ownership.
	var ownerID string
	err := r.pool.QueryRow(ctx,
		`SELECT user_id FROM api_keys WHERE id = $1`,
		keyID,
	).Scan(&ownerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrKeyNotFound
		}
		return fmt.Errorf("lookup api key owner: %w", err)
	}

	if ownerID != userID {
		return ErrNotOwner
	}

	now := time.Now().UTC()
	tag, err := r.pool.Exec(ctx,
		`UPDATE api_keys SET revoked_at = $1 WHERE id = $2 AND revoked_at IS NULL`,
		now, keyID,
	)
	if err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrKeyNotFound // already revoked
	}
	return nil
}
