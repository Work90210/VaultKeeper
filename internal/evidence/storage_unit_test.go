package evidence

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/minio/minio-go/v7"
)

// mockMinioClient implements minioClient for unit testing storage.go
type mockMinioClient struct {
	bucketExistsVal bool
	bucketExistsErr error
	makeBucketErr   error
	putObjectErr    error
	getObjectErr    error
	removeObjectErr error
	statObjectErr   error
	statObjectInfo  minio.ObjectInfo
	// SSE probe: putObjectCount tracks calls to toggle SSE success
	putObjectCount int
	sseProbeOK     bool
}

func (m *mockMinioClient) BucketExists(_ context.Context, _ string) (bool, error) {
	return m.bucketExistsVal, m.bucketExistsErr
}

func (m *mockMinioClient) MakeBucket(_ context.Context, _ string, _ minio.MakeBucketOptions) error {
	return m.makeBucketErr
}

func (m *mockMinioClient) PutObject(_ context.Context, _, _ string, _ io.Reader, _ int64, _ minio.PutObjectOptions) (minio.UploadInfo, error) {
	m.putObjectCount++
	// First PutObject call in initStorage is the SSE probe
	if m.sseProbeOK && m.putObjectCount == 1 {
		return minio.UploadInfo{}, nil
	}
	return minio.UploadInfo{}, m.putObjectErr
}

func (m *mockMinioClient) GetObject(_ context.Context, _, _ string, _ minio.GetObjectOptions) (*minio.Object, error) {
	return nil, m.getObjectErr
}

func (m *mockMinioClient) RemoveObject(_ context.Context, _, _ string, _ minio.RemoveObjectOptions) error {
	return m.removeObjectErr
}

func (m *mockMinioClient) StatObject(_ context.Context, _, _ string, _ minio.StatObjectOptions) (minio.ObjectInfo, error) {
	return m.statObjectInfo, m.statObjectErr
}

func TestInitStorage_BucketExistsError(t *testing.T) {
	mock := &mockMinioClient{bucketExistsErr: fmt.Errorf("connection refused")}
	_, err := initStorage(context.Background(), mock, "test")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestInitStorage_MakeBucketError(t *testing.T) {
	mock := &mockMinioClient{bucketExistsVal: false, makeBucketErr: fmt.Errorf("permission denied")}
	_, err := initStorage(context.Background(), mock, "test")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestInitStorage_SSEProbeSuccess(t *testing.T) {
	mock := &mockMinioClient{bucketExistsVal: true, sseProbeOK: true}
	storage, err := initStorage(context.Background(), mock, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if storage.sse == nil {
		t.Error("expected SSE to be enabled when probe succeeds")
	}
}

func TestInitStorage_SSEProbeFail(t *testing.T) {
	mock := &mockMinioClient{
		bucketExistsVal: true,
		putObjectErr:    fmt.Errorf("KMS not configured"),
	}
	storage, err := initStorage(context.Background(), mock, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if storage.sse != nil {
		t.Error("expected SSE to be nil when probe fails")
	}
}

func TestInitStorage_BucketCreated(t *testing.T) {
	mock := &mockMinioClient{
		bucketExistsVal: false,
		putObjectErr:    fmt.Errorf("no KMS"),
	}
	storage, err := initStorage(context.Background(), mock, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if storage == nil {
		t.Fatal("expected non-nil storage")
	}
}

func TestMinIOStorage_GetObject_ClientError(t *testing.T) {
	s := &MinIOStorage{
		client: &mockMinioClient{getObjectErr: fmt.Errorf("connection refused")},
		bucket: "test",
	}
	_, _, _, err := s.GetObject(context.Background(), "key")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMinIOStorage_GetObject_StatError(t *testing.T) {
	s := &MinIOStorage{
		client: &mockMinioClient{statObjectErr: fmt.Errorf("not found")},
		bucket: "test",
	}
	_, _, _, err := s.GetObject(context.Background(), "key")
	if err == nil {
		t.Fatal("expected error from stat after get")
	}
}

func TestMinIOStorage_GetObject_Success(t *testing.T) {
	s := &MinIOStorage{
		client: &mockMinioClient{
			statObjectInfo: minio.ObjectInfo{Size: 100, ContentType: "text/plain"},
		},
		bucket: "test",
	}
	// GetObject returns nil obj from mock, but we can check size/contentType
	_, size, ct, err := s.GetObject(context.Background(), "key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size != 100 {
		t.Fatalf("expected size 100, got %d", size)
	}
	if ct != "text/plain" {
		t.Fatalf("expected text/plain, got %s", ct)
	}
}

func TestMinIOStorage_DeleteObject_Error(t *testing.T) {
	s := &MinIOStorage{
		client: &mockMinioClient{removeObjectErr: fmt.Errorf("permission denied")},
		bucket: "test",
	}
	err := s.DeleteObject(context.Background(), "key")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMinIOStorage_PutObject_SeekError(t *testing.T) {
	s := &MinIOStorage{
		client: &mockMinioClient{putObjectErr: fmt.Errorf("transient")},
		bucket: "test",
	}
	// failSeeker fails on Seek after first call
	reader := &failSeeker{data: []byte("test"), failOnSeek: true}
	err := s.PutObject(context.Background(), "key", reader, 4, "text/plain")
	if err == nil {
		t.Fatal("expected error from seek failure")
	}
}

type failSeeker struct {
	data       []byte
	pos        int
	failOnSeek bool
	readCount  int
}

func (f *failSeeker) Read(p []byte) (int, error) {
	f.readCount++
	if f.pos >= len(f.data) {
		return 0, io.EOF
	}
	n := copy(p, f.data[f.pos:])
	f.pos += n
	return n, nil
}

func (f *failSeeker) Seek(_ int64, _ int) (int64, error) {
	if f.failOnSeek {
		return 0, fmt.Errorf("seek failed")
	}
	f.pos = 0
	return 0, nil
}

func TestMinIOStorage_StatObject_Error(t *testing.T) {
	s := &MinIOStorage{
		client: &mockMinioClient{statObjectErr: fmt.Errorf("not found")},
		bucket: "test",
	}
	_, err := s.StatObject(context.Background(), "key")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewMinIOStorage_InvalidEndpoint(t *testing.T) {
	// minio.New with empty endpoint returns an error
	_, err := NewMinIOStorage(context.Background(), "", "", "", "test", false)
	if err == nil {
		t.Fatal("expected error for empty endpoint")
	}
}

func TestMinIOStorage_PutObject_Success(t *testing.T) {
	mock := &mockMinioClient{}
	s := &MinIOStorage{client: mock, bucket: "test"}
	err := s.PutObject(context.Background(), "key", bytes.NewReader([]byte("data")), 4, "text/plain")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMinIOStorage_DeleteObject_Success(t *testing.T) {
	s := &MinIOStorage{client: &mockMinioClient{}, bucket: "test"}
	err := s.DeleteObject(context.Background(), "key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMinIOStorage_StatObject_Success(t *testing.T) {
	s := &MinIOStorage{
		client: &mockMinioClient{statObjectInfo: minio.ObjectInfo{Size: 42}},
		bucket: "test",
	}
	size, err := s.StatObject(context.Background(), "key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size != 42 {
		t.Fatalf("expected size 42, got %d", size)
	}
}
