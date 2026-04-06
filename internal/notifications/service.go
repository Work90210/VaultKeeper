package notifications

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Notification struct {
	ID        uuid.UUID
	CaseID    *uuid.UUID
	UserID    uuid.UUID
	Title     string
	Body      string
	Read      bool
	CreatedAt time.Time
}

type Notifier interface {
	Send(ctx context.Context, notification Notification) error
}
