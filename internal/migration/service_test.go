package migration

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// fakeRepo is an in-memory Repository for service tests.
type fakeRepo struct {
	mu      sync.Mutex
	records map[uuid.UUID]*Record
	resume  map[uuid.UUID]map[string]bool
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		records: make(map[uuid.UUID]*Record),
		resume:  make(map[uuid.UUID]map[string]bool),
	}
}

func (f *fakeRepo) Create(_ context.Context, rec Record) (Record, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if rec.ID == uuid.Nil {
		rec.ID = uuid.New()
	}
	rec.CreatedAt = time.Now().UTC()
	cp := rec
	f.records[rec.ID] = &cp
	return cp, nil
}

func (f *fakeRepo) FinalizeSuccess(_ context.Context, id uuid.UUID, matched, mismatched int, hash string, token []byte, tsaName string, tsTime *time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.records[id]
	if !ok {
		return ErrNotFound
	}
	r.MatchedItems = matched
	r.MismatchedItems = mismatched
	r.MigrationHash = hash
	r.TSAToken = token
	r.TSAName = tsaName
	r.TSATimestamp = tsTime
	r.Status = StatusCompleted
	return nil
}

func (f *fakeRepo) Delete(_ context.Context, id uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.records[id]; ok && r.Status == StatusInProgress {
		delete(f.records, id)
		return nil
	}
	return ErrNotFound
}

func (f *fakeRepo) FinalizeFailure(_ context.Context, id uuid.UUID, status MigrationStatus) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.records[id]
	if !ok {
		return ErrNotFound
	}
	r.Status = status
	return nil
}

func (f *fakeRepo) FindByID(_ context.Context, id uuid.UUID) (Record, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.records[id]
	if !ok {
		return Record{}, ErrNotFound
	}
	return *r, nil
}

func (f *fakeRepo) ListByCase(_ context.Context, caseID uuid.UUID) ([]Record, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Record
	for _, r := range f.records {
		if r.CaseID == caseID {
			out = append(out, *r)
		}
	}
	return out, nil
}

func (f *fakeRepo) IsProcessed(_ context.Context, id uuid.UUID, path string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	set := f.resume[id]
	return set[path], nil
}

func (f *fakeRepo) MarkProcessed(_ context.Context, id uuid.UUID, path string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.resume[id] == nil {
		f.resume[id] = make(map[string]bool)
	}
	f.resume[id][path] = true
	return nil
}

// stubTSA always returns a deterministic fake timestamp token.
type stubTSA struct {
	failCount int
	calls     int
}

func (s *stubTSA) IssueTimestamp(_ context.Context, _ []byte) ([]byte, string, time.Time, error) {
	s.calls++
	if s.calls <= s.failCount {
		return nil, "", time.Time{}, errors.New("tsa unavailable")
	}
	return []byte("FAKE-TSA-TOKEN"), "FakeTSA", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), nil
}

func TestComputeMigrationHash_Deterministic(t *testing.T) {
	mk := func(path, hash string) IngestedItem {
		return IngestedItem{ManifestEntry: ManifestEntry{FilePath: path}, ComputedHash: hash}
	}
	items := []IngestedItem{
		mk("docs/a.txt", "aaaa"),
		mk("docs/b.txt", "bbbb"),
		mk("docs/c.txt", "cccc"),
	}
	reversed := []IngestedItem{items[2], items[1], items[0]}
	start := time.Date(2026, 4, 9, 14, 0, 0, 0, time.UTC)
	h1 := ComputeMigrationHash("RelativityOne", start, items)
	h2 := ComputeMigrationHash("RelativityOne", start, reversed)
	if h1 != h2 {
		t.Errorf("hash not deterministic: %s vs %s", h1, h2)
	}
	if len(h1) != 64 {
		t.Errorf("hash length = %d, want 64", len(h1))
	}
}

func TestComputeMigrationHash_ChangesWithInput(t *testing.T) {
	start := time.Date(2026, 4, 9, 14, 0, 0, 0, time.UTC)
	base := []IngestedItem{{ManifestEntry: ManifestEntry{FilePath: "a.txt"}, ComputedHash: "aaaa"}}
	h1 := ComputeMigrationHash("A", start, base)
	h2 := ComputeMigrationHash("B", start, base)
	if h1 == h2 {
		t.Error("source system change did not affect hash")
	}
}

func TestComputeMigrationHash_FilePathBinding(t *testing.T) {
	// Two items with identical content (hash) but different paths must
	// produce a different migration hash than two items with identical
	// content AND identical paths — the filepath binds file identity.
	start := time.Date(2026, 4, 9, 14, 0, 0, 0, time.UTC)
	a := []IngestedItem{
		{ManifestEntry: ManifestEntry{FilePath: "a.txt"}, ComputedHash: "aaaa"},
		{ManifestEntry: ManifestEntry{FilePath: "b.txt"}, ComputedHash: "aaaa"},
	}
	b := []IngestedItem{
		{ManifestEntry: ManifestEntry{FilePath: "a.txt"}, ComputedHash: "aaaa"},
		{ManifestEntry: ManifestEntry{FilePath: "a.txt"}, ComputedHash: "aaaa"},
	}
	if ComputeMigrationHash("src", start, a) == ComputeMigrationHash("src", start, b) {
		t.Error("migration hash should distinguish distinct filepaths even with identical hashes")
	}
}

func TestComputeManifestHash_SortedStable(t *testing.T) {
	a := []ManifestEntry{{FilePath: "z.txt"}, {FilePath: "a.txt"}}
	b := []ManifestEntry{{FilePath: "a.txt"}, {FilePath: "z.txt"}}
	if ComputeManifestHash(a) != ComputeManifestHash(b) {
		t.Error("manifest hash not stable under reordering")
	}
}

func TestService_Run_HappyPath(t *testing.T) {
	dir := t.TempDir()
	hashA := writeTestFile(t, dir, "a.txt", "A")
	hashB := writeTestFile(t, dir, "b.txt", "B")
	csvBody := "filename,sha256_hash\na.txt," + hashA + "\nb.txt," + hashB + "\n"

	repo := newFakeRepo()
	writer := newFakeWriter()
	svc := NewService(nil, NewIngester(writer, repo), repo, &stubTSA{}, slog.New(slog.NewTextHandler(os.Stderr, nil)))

	res, err := svc.Run(context.Background(), RunInput{
		CaseID:         uuid.New(),
		SourceSystem:   "RelativityOne",
		PerformedBy:    "tester",
		ManifestSource: strings.NewReader(csvBody),
		ManifestFormat: FormatCSV,
		SourceRoot:     dir,
		Options:        BatchOptions{Concurrency: 2},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Record.Status != StatusCompleted {
		t.Errorf("status = %q, want completed", res.Record.Status)
	}
	if res.Record.MatchedItems != 2 {
		t.Errorf("matched = %d, want 2", res.Record.MatchedItems)
	}
	if len(res.Record.TSAToken) == 0 {
		t.Error("TSA token missing from completed migration")
	}
	if res.Record.TSATimestamp == nil {
		t.Error("TSA timestamp missing from completed migration")
	}
	if len(res.Record.MigrationHash) != 64 {
		t.Errorf("MigrationHash length = %d", len(res.Record.MigrationHash))
	}
}

func TestService_Run_HaltOnMismatch(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "a.txt", "A")
	csvBody := "filename,sha256_hash\na.txt," + strings.Repeat("0", 64) + "\n"

	repo := newFakeRepo()
	svc := NewService(nil, NewIngester(newFakeWriter(), repo), repo, &stubTSA{}, nil)

	res, err := svc.Run(context.Background(), RunInput{
		CaseID:         uuid.New(),
		SourceSystem:   "RelativityOne",
		PerformedBy:    "tester",
		ManifestSource: strings.NewReader(csvBody),
		ManifestFormat: FormatCSV,
		SourceRoot:     dir,
		Options:        BatchOptions{HaltOnMismatch: true, Concurrency: 1},
	})
	if err == nil {
		t.Fatal("want HashMismatchError")
	}
	if !IsHashMismatch(err) {
		t.Errorf("err = %v, want HashMismatchError", err)
	}
	if res.Record.Status != StatusHaltedOnMismatch {
		t.Errorf("status = %q, want halted_mismatch", res.Record.Status)
	}
}

func TestService_Run_ResumeByIDSkipsProcessed(t *testing.T) {
	dir := t.TempDir()
	hashA := writeTestFile(t, dir, "a.txt", "A")
	hashB := writeTestFile(t, dir, "b.txt", "B")
	csvBody := "filename,sha256_hash\na.txt," + hashA + "\nb.txt," + hashB + "\n"

	repo := newFakeRepo()
	writer := newFakeWriter()
	svc := NewService(nil, NewIngester(writer, repo), repo, &stubTSA{}, nil)

	caseID := uuid.New()
	// First run: complete both files so resume state is populated, but
	// leave the row in_progress by using a TSA stub that blocks the
	// finalize path. Simpler: run once, then seed resume state manually
	// for a fresh in-progress row and invoke Run again with ResumeMigrationID.
	rec, err := repo.Create(context.Background(), Record{
		CaseID:       caseID,
		SourceSystem: "RelativityOne",
		TotalItems:   2,
		ManifestHash: ComputeManifestHash([]ManifestEntry{
			{Index: 1, FilePath: "a.txt", OriginalHash: hashA},
			{Index: 2, FilePath: "b.txt", OriginalHash: hashB},
		}),
		MigrationHash: "placeholder",
		PerformedBy:   "tester",
		Status:        StatusInProgress,
	})
	if err != nil {
		t.Fatalf("seed migration: %v", err)
	}
	// Mark a.txt as already processed.
	if err := repo.MarkProcessed(context.Background(), rec.ID, "a.txt"); err != nil {
		t.Fatal(err)
	}

	res, err := svc.Run(context.Background(), RunInput{
		CaseID:            caseID,
		SourceSystem:      "RelativityOne",
		PerformedBy:       "tester",
		ManifestSource:    strings.NewReader(csvBody),
		ManifestFormat:    FormatCSV,
		SourceRoot:        dir,
		ResumeMigrationID: &rec.ID,
	})
	if err != nil {
		t.Fatalf("Run resume: %v", err)
	}
	// Only b.txt should have been ingested on the resume pass.
	if len(writer.stored) != 1 {
		t.Errorf("writer stored %d files, want 1", len(writer.stored))
	}
	if _, ok := writer.stored["b.txt"]; !ok {
		t.Errorf("want b.txt in writer.stored, got %v", writer.stored)
	}
	if res.Record.Status != StatusCompleted {
		t.Errorf("resume status = %q, want completed", res.Record.Status)
	}
}

func TestService_Run_ResumeManifestMismatch(t *testing.T) {
	dir := t.TempDir()
	hashA := writeTestFile(t, dir, "a.txt", "A")
	csvBody := "filename,sha256_hash\na.txt," + hashA + "\n"

	repo := newFakeRepo()
	svc := NewService(nil, NewIngester(newFakeWriter(), repo), repo, &stubTSA{}, nil)

	rec, err := repo.Create(context.Background(), Record{
		CaseID:        uuid.New(),
		SourceSystem:  "RelativityOne",
		TotalItems:    1,
		ManifestHash:  "deadbeef" + strings.Repeat("0", 56), // deliberate wrong hash
		MigrationHash: "placeholder",
		PerformedBy:   "tester",
		Status:        StatusInProgress,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, runErr := svc.Run(context.Background(), RunInput{
		CaseID:            rec.CaseID,
		SourceSystem:      "RelativityOne",
		PerformedBy:       "tester",
		ManifestSource:    strings.NewReader(csvBody),
		ManifestFormat:    FormatCSV,
		SourceRoot:        dir,
		ResumeMigrationID: &rec.ID,
	})
	if !errors.Is(runErr, ErrResumeManifestMismatch) {
		t.Errorf("want ErrResumeManifestMismatch, got %v", runErr)
	}
}

func TestService_Run_DryRunSkipsTSA(t *testing.T) {
	dir := t.TempDir()
	hashA := writeTestFile(t, dir, "a.txt", "A")
	csvBody := "filename,sha256_hash\na.txt," + hashA + "\n"

	repo := newFakeRepo()
	tsa := &stubTSA{}
	svc := NewService(nil, NewIngester(newFakeWriter(), repo), repo, tsa, nil)

	res, err := svc.Run(context.Background(), RunInput{
		CaseID:         uuid.New(),
		SourceSystem:   "RelativityOne",
		PerformedBy:    "tester",
		ManifestSource: strings.NewReader(csvBody),
		ManifestFormat: FormatCSV,
		SourceRoot:     dir,
		Options:        BatchOptions{DryRun: true},
	})
	if err != nil {
		t.Fatalf("Run dry-run: %v", err)
	}
	if tsa.calls != 0 {
		t.Errorf("TSA called %d times in dry-run, want 0", tsa.calls)
	}
	if res.Record.Status != StatusInProgress {
		t.Errorf("dry-run status = %q, want in_progress", res.Record.Status)
	}
	// The dry-run row must be deleted from the repository so the
	// evidence_migrations table doesn't accumulate phantom rows.
	if _, err := repo.FindByID(context.Background(), res.Record.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("dry-run row was not deleted: FindByID returned err=%v", err)
	}
}
