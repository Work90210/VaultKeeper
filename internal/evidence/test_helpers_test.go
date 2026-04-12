package evidence

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Shared test helpers used by both unit and integration tests.
// Kept in an untagged file so unit-only test builds can access them.
// ---------------------------------------------------------------------------

// inMemStorage is a minimal ObjectStorage backed by an in-memory map.
type inMemStorage struct {
	objects map[string][]byte
}

func (s *inMemStorage) PutObject(_ context.Context, key string, reader io.ReadSeeker, _ int64, _ string) error {
	data, _ := io.ReadAll(reader)
	s.objects[key] = data
	return nil
}

func (s *inMemStorage) GetObject(_ context.Context, key string) (io.ReadCloser, int64, string, error) {
	data, ok := s.objects[key]
	if !ok {
		return nil, 0, "", &ValidationError{Field: "key", Message: "not found"}
	}
	return io.NopCloser(bytes.NewReader(data)), int64(len(data)), "application/octet-stream", nil
}

func (s *inMemStorage) DeleteObject(_ context.Context, key string) error {
	delete(s.objects, key)
	return nil
}

func (s *inMemStorage) StatObject(_ context.Context, key string) (int64, error) {
	data, ok := s.objects[key]
	if !ok {
		return 0, &ValidationError{Field: "key", Message: "not found"}
	}
	return int64(len(data)), nil
}

func (s *inMemStorage) BucketName() string {
	return "test-evidence"
}

// noopCustody satisfies CustodyRecorder but discards events.
type noopCustody struct{}

func (c *noopCustody) RecordEvidenceEvent(_ context.Context, _, _ uuid.UUID, _, _ string, _ map[string]string) error {
	return nil
}

// createSmallPNG generates a valid single-color PNG image.
func createSmallPNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}
