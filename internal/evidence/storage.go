package evidence

import "context"

type FileStorage interface {
	PutObject(ctx context.Context, objectKey string, contentType string, size int64) (uploadURL string, err error)
	GetObject(ctx context.Context, objectKey string) (downloadURL string, err error)
	DeleteObject(ctx context.Context, objectKey string) error
}
