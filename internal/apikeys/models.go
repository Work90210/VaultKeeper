package apikeys

import (
	"time"

	"github.com/google/uuid"
)

// KeyPrefix is prepended to all generated API keys for easy identification.
const KeyPrefix = "vk_"

// APIKey represents a stored API key record.
type APIKey struct {
	ID          uuid.UUID  `json:"id"`
	UserID      string     `json:"user_id"`
	Name        string     `json:"name"`
	KeyHash     string     `json:"-"` // never exposed via JSON
	Permissions string     `json:"permissions"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	RevokedAt   *time.Time `json:"revoked_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// CreateKeyInput holds the parameters needed to create a new API key.
type CreateKeyInput struct {
	Name        string `json:"name"`
	Permissions string `json:"permissions"`
}

// CreateKeyResult is returned once on creation; RawKey is never stored or retrievable again.
type CreateKeyResult struct {
	Key    APIKey `json:"key"`
	RawKey string `json:"raw_key"`
}
