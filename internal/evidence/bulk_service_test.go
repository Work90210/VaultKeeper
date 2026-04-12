package evidence

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakeBulkRepo is an in-memory BulkJobRepository for service tests.
type fakeBulkRepo struct {
	mu   sync.Mutex
	jobs map[uuid.UUID]*BulkJob
}

func newFakeBulkRepo() *fakeBulkRepo {
	return &fakeBulkRepo{jobs: make(map[uuid.UUID]*BulkJob)}
}

func (f *fakeBulkRepo) Create(_ context.Context, caseID uuid.UUID, performedBy, archiveKey string) (BulkJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := uuid.New()
	job := &BulkJob{
		ID:          id,
		CaseID:      caseID,
		PerformedBy: performedBy,
		Status:      BulkStatusExtracting,
	}
	f.jobs[id] = job
	return *job, nil
}

func (f *fakeBulkRepo) SetArchiveHash(_ context.Context, id uuid.UUID, sha string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.jobs[id]; !ok {
		return ErrBulkJobNotFound
	}
	return nil
}

func (f *fakeBulkRepo) UpdateProgress(_ context.Context, id uuid.UUID, processed, failed, total int, status string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	j, ok := f.jobs[id]
	if !ok {
		return ErrBulkJobNotFound
	}
	j.Processed = processed
	j.Failed = failed
	j.Total = total
	j.Status = status
	return nil
}

func (f *fakeBulkRepo) AppendError(_ context.Context, id uuid.UUID, entry BulkJobError) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	j, ok := f.jobs[id]
	if !ok {
		return ErrBulkJobNotFound
	}
	j.Errors = append(j.Errors, entry)
	return nil
}

func (f *fakeBulkRepo) Finalize(_ context.Context, id uuid.UUID, status string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	j, ok := f.jobs[id]
	if !ok {
		return ErrBulkJobNotFound
	}
	j.Status = status
	return nil
}

func (f *fakeBulkRepo) FindByID(_ context.Context, caseID, id uuid.UUID) (BulkJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	j, ok := f.jobs[id]
	if !ok || j.CaseID != caseID {
		return BulkJob{}, ErrBulkJobNotFound
	}
	return *j, nil
}

// bulkTestZip builds an in-memory zip archive.
func bulkTestZip(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range entries {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestBulkService_Submit_HappyPath(t *testing.T) {
	evSvc, _, _, custody := newTestService(t)
	// The evidence service's mock case lookup returns "active" by default.
	fakeRepo := newFakeBulkRepo()
	logger := discardLogger()
	bulkSvc := NewBulkService(fakeRepo, evSvc, logger, 10*1024*1024)

	data := bulkTestZip(t, map[string]string{
		"a.txt":         "alpha",
		"sub/b.txt":     "bravo",
		"_metadata.csv": "filename,title,description,tags\na.txt,Alpha,First doc,legal;review\n",
	})
	job, err := bulkSvc.Submit(context.Background(), BulkSubmitInput{
		CaseID:         uuid.New(),
		UploadedBy:     "tester",
		UploadedByName: "Tester User",
		ArchiveBytes:   data,
		ArchiveName:    "bulk.zip",
		Classification: ClassificationRestricted,
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if job.Status != BulkStatusCompleted {
		t.Errorf("Status = %q, want %q", job.Status, BulkStatusCompleted)
	}
	if job.Processed != 2 {
		t.Errorf("Processed = %d, want 2", job.Processed)
	}
	if job.Failed != 0 {
		t.Errorf("Failed = %d, want 0", job.Failed)
	}
	// Both files must have produced custody entries from the Upload path.
	if n := len(custody.events); n < 2 {
		t.Errorf("custody events = %d, want >= 2", n)
	}
}

func TestBulkService_Submit_RejectsZipBomb(t *testing.T) {
	evSvc, _, _, _ := newTestService(t)
	bulkSvc := NewBulkService(newFakeBulkRepo(), evSvc, discardLogger(), 1024)

	// A single entry 2KB larger than the per-file limit.
	big := strings.Repeat("X", 2048)
	data := bulkTestZip(t, map[string]string{"big.bin": big})

	_, err := bulkSvc.Submit(context.Background(), BulkSubmitInput{
		CaseID:      uuid.New(),
		UploadedBy:  "tester",
		ArchiveBytes: data,
		ArchiveName: "bomb.zip",
	})
	if err == nil || !errors.Is(err, ErrZipRejected) {
		t.Fatalf("want ErrZipRejected, got %v", err)
	}
}

func TestBulkService_Submit_RejectsAbsolutePath(t *testing.T) {
	evSvc, _, _, _ := newTestService(t)
	bulkSvc := NewBulkService(newFakeBulkRepo(), evSvc, discardLogger(), 10*1024*1024)

	data := bulkTestZip(t, map[string]string{"/etc/passwd": "pwned"})
	_, err := bulkSvc.Submit(context.Background(), BulkSubmitInput{
		CaseID:      uuid.New(),
		UploadedBy:  "tester",
		ArchiveBytes: data,
	})
	if err == nil || !errors.Is(err, ErrZipRejected) {
		t.Fatalf("want ErrZipRejected, got %v", err)
	}
}

func TestBulkService_Submit_PartialFailureFinalizesWithErrors(t *testing.T) {
	// Wire a deliberately failing "evidence" service that rejects any
	// file named "bad.txt" via a custom case lookup that mimics a closed
	// case. For simplicity we instead truncate the archive so one entry
	// is extractable but Upload will reject it by classification.
	evSvc, _, _, _ := newTestService(t)
	bulkSvc := NewBulkService(newFakeBulkRepo(), evSvc, discardLogger(), 10*1024*1024)

	// Ship two files plus a metadata row that assigns a bogus
	// classification to one of them. Upload should reject the bogus
	// classification and accept the other file.
	data := bulkTestZip(t, map[string]string{
		"good.txt":      "G",
		"bad.txt":       "B",
		"_metadata.csv": "filename,classification\nbad.txt,not_a_real_classification\n",
	})
	job, err := bulkSvc.Submit(context.Background(), BulkSubmitInput{
		CaseID:         uuid.New(),
		UploadedBy:     "tester",
		ArchiveBytes:   data,
		Classification: ClassificationRestricted,
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if job.Processed != 1 {
		t.Errorf("Processed = %d, want 1", job.Processed)
	}
	if job.Failed != 1 {
		t.Errorf("Failed = %d, want 1", job.Failed)
	}
	if job.Status != BulkStatusCompletedWithErrors {
		t.Errorf("Status = %q, want %q", job.Status, BulkStatusCompletedWithErrors)
	}
}
