package notifications

import (
	"context"

	"github.com/google/uuid"
)

type NotificationReader interface {
	ListUnread(ctx context.Context, userID uuid.UUID, limit int) ([]Notification, error)
	MarkRead(ctx context.Context, id uuid.UUID) error
}
