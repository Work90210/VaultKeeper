package integrity

import (
	"context"
	"io"

	"github.com/google/uuid"
)

// StorageFileReader adapts an object storage with extra return values to the FileReader interface.
type StorageFileReader struct {
	GetFn func(ctx context.Context, key string) (io.ReadCloser, int64, string, error)
}

func (s *StorageFileReader) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	rc, _, _, err := s.GetFn(ctx, key)
	return rc, err
}

// EvidenceVerificationAdapter adapts evidence repository items to VerifiableItem.
type EvidenceVerificationAdapter struct {
	ListFn func(ctx context.Context, caseID uuid.UUID) ([]VerifiableItem, error)
}

// VerifiableItemAdapter holds external verifiable item data and converts it.
type VerifiableItemAdapter[T any] struct {
	ListFn    func(ctx context.Context, caseID uuid.UUID) ([]T, error)
	ConvertFn func(T) VerifiableItem
}

func (a *VerifiableItemAdapter[T]) ListByCaseForVerification(ctx context.Context, caseID uuid.UUID) ([]VerifiableItem, error) {
	items, err := a.ListFn(ctx, caseID)
	if err != nil {
		return nil, err
	}
	result := make([]VerifiableItem, len(items))
	for i, item := range items {
		result[i] = a.ConvertFn(item)
	}
	return result, nil
}

// NotificationAdapter adapts a notification service to the Notifier interface.
type NotificationAdapter struct {
	NotifyFn func(ctx context.Context, event NotificationEvent) error
}

func (a *NotificationAdapter) Notify(ctx context.Context, event NotificationEvent) error {
	return a.NotifyFn(ctx, event)
}
