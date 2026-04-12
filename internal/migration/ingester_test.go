package migration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/google/uuid"
)

// fakeWriter is an in-memory EvidenceWriter for ingester tests.
type fakeWriter struct {
	mu     sync.Mutex
	stored map[string]StoreInput
}

func newFakeWriter() *fakeWriter {
	return &fakeWriter{stored: make(map[string]StoreInput)}
}

func (f *fakeWriter) StoreMigratedFile(_ context.Context, in StoreInput) (StoreResult, error) {
	// Drain the reader so tests can assert on size.
	data, err := io.ReadAll(in.Reader)
	if err != nil {
		return StoreResult{}, err
	}
	in.SizeBytes = int64(len(data))
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stored[in.Filename] = in
	return StoreResult{EvidenceID: uuid.New(), SizeBytes: in.SizeBytes}, nil
}

// memoryResume is an in-memory ResumeStore for tests.
type memoryResume struct {
	mu   sync.Mutex
	seen map[string]bool
}

func newMemoryResume() *memoryResume { return &memoryResume{seen: make(map[string]bool)} }
func (m *memoryResume) IsProcessed(_ context.Context, _ uuid.UUID, path string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.seen[path], nil
}
func (m *memoryResume) MarkProcessed(_ context.Context, _ uuid.UUID, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seen[path] = true
	return nil
}

// writeTestFile creates a file under dir with the given relative path and
// content, and returns the hex SHA-256 of the content.
func writeTestFile(t *testing.T, dir, rel, content string) string {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func TestIngestFile_HashMatch(t *testing.T) {
	dir := t.TempDir()
	hash := writeTestFile(t, dir, "a.txt", "alpha")
	writer := newFakeWriter()
	ig := NewIngester(writer, nil)

	entry := ManifestEntry{FilePath: "a.txt", OriginalHash: hash, Title: "Alpha"}
	item, err := ig.IngestFile(context.Background(), uuid.New(), dir, "tester", entry, false)
	if err != nil {
		t.Fatalf("IngestFile: %v", err)
	}
	if !item.HashMatched {
		t.Error("HashMatched = false, want true")
	}
	if item.ComputedHash != hash {
		t.Errorf("ComputedHash = %q, want %q", item.ComputedHash, hash)
	}
	if len(writer.stored) != 1 {
		t.Errorf("writer stored %d files, want 1", len(writer.stored))
	}
}

func TestIngestFile_HashMismatch(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "a.txt", "alpha")
	writer := newFakeWriter()
	ig := NewIngester(writer, nil)

	entry := ManifestEntry{
		FilePath:     "a.txt",
		OriginalHash: "0000000000000000000000000000000000000000000000000000000000000000",
	}
	_, err := ig.IngestFile(context.Background(), uuid.New(), dir, "tester", entry, false)
	if err == nil {
		t.Fatal("want HashMismatchError, got nil")
	}
	if !IsHashMismatch(err) {
		t.Errorf("IsHashMismatch = false, want true (err = %v)", err)
	}
	if len(writer.stored) != 0 {
		t.Errorf("writer stored %d files after mismatch, want 0", len(writer.stored))
	}
}

func TestBatchIngest_HaltOnMismatch(t *testing.T) {
	dir := t.TempDir()
	good := writeTestFile(t, dir, "good.txt", "ok")
	writeTestFile(t, dir, "bad.txt", "wrong content") // hash intentionally mismatches entry

	writer := newFakeWriter()
	ig := NewIngester(writer, nil)

	req := BatchRequest{
		CaseID:     uuid.New(),
		SourceRoot: dir,
		UploadedBy: "tester",
		Entries: []ManifestEntry{
			{FilePath: "good.txt", OriginalHash: good},
			{FilePath: "bad.txt", OriginalHash: "dead" + string(make([]byte, 60))},
		},
		Options: BatchOptions{Concurrency: 1, HaltOnMismatch: true},
	}

	// The bad hash is not valid hex; ingester will compare strings literally
	// so the comparison still fails. Build a real 64-char fake hash instead.
	req.Entries[1].OriginalHash = "1111111111111111111111111111111111111111111111111111111111111111"

	report, err := ig.BatchIngest(context.Background(), req, uuid.New(), nil)
	if err != nil {
		t.Fatalf("BatchIngest returned unexpected error: %v", err)
	}
	if report.MismatchedItems != 1 {
		t.Errorf("MismatchedItems = %d, want 1", report.MismatchedItems)
	}
	if !report.Halted {
		t.Error("Halted = false, want true")
	}
	if report.HaltedFile != "bad.txt" {
		t.Errorf("HaltedFile = %q, want bad.txt", report.HaltedFile)
	}
}

func TestBatchIngest_HappyPathParallel(t *testing.T) {
	dir := t.TempDir()
	var entries []ManifestEntry
	for i := 0; i < 10; i++ {
		name := filepath.Join("batch", "file"+string(rune('0'+i))+".txt")
		hash := writeTestFile(t, dir, name, "content-"+string(rune('0'+i)))
		entries = append(entries, ManifestEntry{FilePath: filepath.ToSlash(name), OriginalHash: hash})
	}
	writer := newFakeWriter()
	ig := NewIngester(writer, nil)

	var progressCalls int
	req := BatchRequest{
		CaseID:     uuid.New(),
		SourceRoot: dir,
		UploadedBy: "tester",
		Entries:    entries,
		Options:    BatchOptions{Concurrency: 4},
	}
	report, err := ig.BatchIngest(context.Background(), req, uuid.New(), func(cur, total int, _ string) {
		progressCalls++
	})
	if err != nil {
		t.Fatalf("BatchIngest: %v", err)
	}
	if len(report.Processed) != 10 {
		t.Errorf("Processed = %d, want 10", len(report.Processed))
	}
	if report.MatchedItems != 10 {
		t.Errorf("MatchedItems = %d, want 10", report.MatchedItems)
	}
	if progressCalls != 10 {
		t.Errorf("progress calls = %d, want 10", progressCalls)
	}
	// Deterministic ordering.
	for i := 1; i < len(report.Processed); i++ {
		if report.Processed[i-1].ManifestEntry.FilePath >= report.Processed[i].ManifestEntry.FilePath {
			t.Errorf("Processed not sorted at index %d", i)
		}
	}
}

func TestBatchIngest_ResumeSkipsAlreadyProcessed(t *testing.T) {
	dir := t.TempDir()
	hA := writeTestFile(t, dir, "a.txt", "A")
	hB := writeTestFile(t, dir, "b.txt", "B")
	writer := newFakeWriter()
	resume := newMemoryResume()
	ig := NewIngester(writer, resume)
	migrationID := uuid.New()

	// First run processes both.
	req := BatchRequest{
		CaseID:     uuid.New(),
		SourceRoot: dir,
		UploadedBy: "tester",
		Entries: []ManifestEntry{
			{FilePath: "a.txt", OriginalHash: hA},
			{FilePath: "b.txt", OriginalHash: hB},
		},
		Options: BatchOptions{Resume: true, Concurrency: 2},
	}
	if _, err := ig.BatchIngest(context.Background(), req, migrationID, nil); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if len(writer.stored) != 2 {
		t.Fatalf("first run stored %d, want 2", len(writer.stored))
	}

	// Second run should skip both.
	writer2 := newFakeWriter()
	ig2 := NewIngester(writer2, resume)
	report, err := ig2.BatchIngest(context.Background(), req, migrationID, nil)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if len(writer2.stored) != 0 {
		t.Errorf("resumed run stored %d files, want 0", len(writer2.stored))
	}
	if len(report.Processed) != 0 {
		t.Errorf("resumed run Processed = %d, want 0", len(report.Processed))
	}
}

func TestBatchIngest_PreCheckMissingFile(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "present.txt", "x")
	ig := NewIngester(newFakeWriter(), nil)

	req := BatchRequest{
		CaseID:     uuid.New(),
		SourceRoot: dir,
		UploadedBy: "tester",
		Entries: []ManifestEntry{
			{FilePath: "present.txt"},
			{FilePath: "missing.txt"},
		},
		Options: BatchOptions{},
	}
	_, err := ig.BatchIngest(context.Background(), req, uuid.New(), nil)
	if err == nil {
		t.Fatal("want pre-check error for missing file")
	}
}
