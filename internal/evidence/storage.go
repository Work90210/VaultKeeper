package evidence

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/encrypt"
)

// ObjectStorage defines the interface for evidence file storage.
type ObjectStorage interface {
	PutObject(ctx context.Context, key string, reader io.ReadSeeker, size int64, contentType string) error
	GetObject(ctx context.Context, key string) (io.ReadCloser, int64, string, error)
	DeleteObject(ctx context.Context, key string) error
	StatObject(ctx context.Context, key string) (int64, error)
	BucketName() string
}

// minioClient abstracts the minio.Client methods used by MinIOStorage,
// enabling unit testing of error paths without a real MinIO server.
type minioClient interface {
	BucketExists(ctx context.Context, bucket string) (bool, error)
	MakeBucket(ctx context.Context, bucket string, opts minio.MakeBucketOptions) error
	PutObject(ctx context.Context, bucket, key string, reader io.Reader, size int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
	GetObject(ctx context.Context, bucket, key string, opts minio.GetObjectOptions) (*minio.Object, error)
	RemoveObject(ctx context.Context, bucket, key string, opts minio.RemoveObjectOptions) error
	StatObject(ctx context.Context, bucket, key string, opts minio.StatObjectOptions) (minio.ObjectInfo, error)
	ListObjects(ctx context.Context, bucket string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo
}

// MinIOStorage implements ObjectStorage using MinIO with SSE-S3 encryption.
type MinIOStorage struct {
	client minioClient
	bucket string
	sse    encrypt.ServerSide
}

// NewMinIOStorage creates a MinIO client and ensures the bucket exists.
func NewMinIOStorage(ctx context.Context, endpoint, accessKey, secretKey, bucket string, useSSL bool) (*MinIOStorage, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}
	return initStorage(ctx, client, bucket)
}

// initStorage configures the bucket and SSE probe. Separated from
// NewMinIOStorage so unit tests can inject a mock minioClient.
func initStorage(ctx context.Context, client minioClient, bucket string) (*MinIOStorage, error) {
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("check bucket exists: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("create bucket: %w", err)
		}
	}

	// Test if SSE-S3 encryption works; if KMS isn't configured, disable SSE
	var sse encrypt.ServerSide
	testKey := "__vaultkeeper_sse_test"
	_, putErr := client.PutObject(ctx, bucket, testKey, io.LimitReader(nil, 0), 0, minio.PutObjectOptions{
		ServerSideEncryption: encrypt.NewSSE(),
	})
	if putErr != nil {
		slog.Warn("MinIO SSE-S3 not available, storing files without server-side encryption", "error", putErr)
	} else {
		sse = encrypt.NewSSE()
		_ = client.RemoveObject(ctx, bucket, testKey, minio.RemoveObjectOptions{})
		slog.Info("MinIO SSE-S3 encryption enabled")
	}

	return &MinIOStorage{
		client: client,
		bucket: bucket,
		sse:    sse,
	}, nil
}

// BucketName returns the configured bucket name.
func (s *MinIOStorage) BucketName() string {
	return s.bucket
}

// BucketExists checks whether the given bucket exists in MinIO.
func (s *MinIOStorage) BucketExists(ctx context.Context, bucket string) (bool, error) {
	return s.client.BucketExists(ctx, bucket)
}

func (s *MinIOStorage) PutObject(ctx context.Context, key string, reader io.ReadSeeker, size int64, contentType string) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			if _, err := reader.Seek(0, io.SeekStart); err != nil {
				return fmt.Errorf("seek reader for retry: %w", err)
			}
		}
		opts := minio.PutObjectOptions{ContentType: contentType}
		if s.sse != nil {
			opts.ServerSideEncryption = s.sse
		}
		_, err := s.client.PutObject(ctx, s.bucket, key, reader, size, opts)
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt < 2 {
			select {
			case <-ctx.Done():
				return fmt.Errorf("put object cancelled: %w", ctx.Err())
			case <-time.After(time.Duration(attempt+1) * 500 * time.Millisecond):
			}
		}
	}
	return fmt.Errorf("put object %s after retries: %w", key, lastErr)
}

func (s *MinIOStorage) GetObject(ctx context.Context, key string) (io.ReadCloser, int64, string, error) {
	getOpts := minio.GetObjectOptions{}
	if s.sse != nil {
		getOpts.ServerSideEncryption = s.sse
	}
	obj, err := s.client.GetObject(ctx, s.bucket, key, getOpts)
	if err != nil {
		return nil, 0, "", fmt.Errorf("get object %s: %w", key, err)
	}

	info, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		obj.Close()
		return nil, 0, "", fmt.Errorf("stat object %s: %w", key, err)
	}

	return obj, info.Size, info.ContentType, nil
}

func (s *MinIOStorage) DeleteObject(ctx context.Context, key string) error {
	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("delete object %s: %w", key, err)
	}
	return nil
}

// ListObjects returns all object keys in the bucket with the given prefix.
func (s *MinIOStorage) ListObjects(ctx context.Context, prefix string) ([]string, error) {
	opts := minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	}

	var keys []string
	for obj := range s.client.ListObjects(ctx, s.bucket, opts) {
		if obj.Err != nil {
			return nil, fmt.Errorf("list objects: %w", obj.Err)
		}
		keys = append(keys, obj.Key)
	}

	return keys, nil
}

func (s *MinIOStorage) StatObject(ctx context.Context, key string) (int64, error) {
	statOpts := minio.StatObjectOptions{}
	if s.sse != nil {
		statOpts.ServerSideEncryption = s.sse
	}
	info, err := s.client.StatObject(ctx, s.bucket, key, statOpts)
	if err != nil {
		return 0, fmt.Errorf("stat object %s: %w", key, err)
	}
	return info.Size, nil
}
