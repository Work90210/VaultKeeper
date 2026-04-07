package evidence

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/encrypt"
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

func (m *mockMinioClient) ListObjects(_ context.Context, _ string, _ minio.ListObjectsOptions) <-chan minio.ObjectInfo {
	ch := make(chan minio.ObjectInfo)
	close(ch)
	return ch
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

func TestNewMinIOStorage_ValidEndpointFailsInit(t *testing.T) {
	// minio.New succeeds with a valid-looking endpoint (no actual connection),
	// but initStorage fails because BucketExists can't reach a real server.
	// This covers the success path of minio.New (line 51: return initStorage).
	_, err := NewMinIOStorage(context.Background(), "localhost:9999", "key", "secret", "test", false)
	if err == nil {
		t.Fatal("expected error from initStorage when no server is running")
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

func TestMinIOStorage_BucketExists_Success(t *testing.T) {
	s := &MinIOStorage{
		client: &mockMinioClient{bucketExistsVal: true},
		bucket: "test",
	}
	exists, err := s.BucketExists(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Fatal("expected bucket to exist")
	}
}

func TestMinIOStorage_BucketExists_Error(t *testing.T) {
	s := &MinIOStorage{
		client: &mockMinioClient{bucketExistsErr: fmt.Errorf("connection refused")},
		bucket: "test",
	}
	_, err := s.BucketExists(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error from BucketExists")
	}
}

func TestMinIOStorage_BucketExists_NotFound(t *testing.T) {
	s := &MinIOStorage{
		client: &mockMinioClient{bucketExistsVal: false},
		bucket: "test",
	}
	exists, err := s.BucketExists(context.Background(), "other-bucket")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Fatal("expected bucket to not exist")
	}
}

func TestMinIOStorage_ListObjects_Empty(t *testing.T) {
	s := &MinIOStorage{
		client: &mockMinioClient{},
		bucket: "test",
	}
	keys, err := s.ListObjects(context.Background(), "prefix/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("expected empty keys, got %v", keys)
	}
}

func TestMinIOStorage_ListObjects_WithKeys(t *testing.T) {
	mock := &mockMinioClientWithListData{
		objects: []minio.ObjectInfo{
			{Key: "prefix/file1.txt"},
			{Key: "prefix/file2.txt"},
		},
	}
	s := &MinIOStorage{
		client: mock,
		bucket: "test",
	}
	keys, err := s.ListObjects(context.Background(), "prefix/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
	if keys[0] != "prefix/file1.txt" || keys[1] != "prefix/file2.txt" {
		t.Fatalf("unexpected keys: %v", keys)
	}
}

func TestMinIOStorage_PutObject_WithSSE(t *testing.T) {
	mock := &mockMinioClient{}
	s := &MinIOStorage{client: mock, bucket: "test", sse: encrypt.NewSSE()}
	err := s.PutObject(context.Background(), "key", bytes.NewReader([]byte("data")), 4, "text/plain")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMinIOStorage_PutObject_RetriesExhausted(t *testing.T) {
	mock := &mockMinioClient{putObjectErr: fmt.Errorf("transient error")}
	s := &MinIOStorage{client: mock, bucket: "test"}
	err := s.PutObject(context.Background(), "key", bytes.NewReader([]byte("data")), 4, "text/plain")
	if err == nil {
		t.Fatal("expected error after retries exhausted")
	}
	if !strings.Contains(err.Error(), "after retries") {
		t.Errorf("expected 'after retries' in error, got: %v", err)
	}
}

func TestMinIOStorage_PutObject_ContextCancelled(t *testing.T) {
	mock := &mockMinioClient{putObjectErr: fmt.Errorf("transient")}
	s := &MinIOStorage{client: mock, bucket: "test"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	err := s.PutObject(ctx, "key", bytes.NewReader([]byte("data")), 4, "text/plain")
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("expected 'cancelled' in error, got: %v", err)
	}
}

func TestMinIOStorage_GetObject_WithSSE(t *testing.T) {
	mock := &mockMinioClient{
		statObjectInfo: minio.ObjectInfo{Size: 50, ContentType: "application/octet-stream"},
	}
	s := &MinIOStorage{client: mock, bucket: "test", sse: encrypt.NewSSE()}
	_, size, ct, err := s.GetObject(context.Background(), "key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size != 50 {
		t.Fatalf("expected size 50, got %d", size)
	}
	if ct != "application/octet-stream" {
		t.Fatalf("expected application/octet-stream, got %s", ct)
	}
}

func TestMinIOStorage_StatObject_WithSSE(t *testing.T) {
	mock := &mockMinioClient{statObjectInfo: minio.ObjectInfo{Size: 99}}
	s := &MinIOStorage{client: mock, bucket: "test", sse: encrypt.NewSSE()}
	size, err := s.StatObject(context.Background(), "key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size != 99 {
		t.Fatalf("expected size 99, got %d", size)
	}
}

func TestMinIOStorage_ListObjects_Error(t *testing.T) {
	mock := &mockMinioClientWithListData{
		objects: []minio.ObjectInfo{
			{Err: fmt.Errorf("list error")},
		},
	}
	s := &MinIOStorage{
		client: mock,
		bucket: "test",
	}
	_, err := s.ListObjects(context.Background(), "prefix/")
	if err == nil {
		t.Fatal("expected error")
	}
}

// mockMinioClientWithListData extends mockMinioClient to return actual objects from ListObjects.
type mockMinioClientWithListData struct {
	mockMinioClient
	objects []minio.ObjectInfo
}

func (m *mockMinioClientWithListData) ListObjects(_ context.Context, _ string, _ minio.ListObjectsOptions) <-chan minio.ObjectInfo {
	ch := make(chan minio.ObjectInfo, len(m.objects))
	for _, obj := range m.objects {
		ch <- obj
	}
	close(ch)
	return ch
}
