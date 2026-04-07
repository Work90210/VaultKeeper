package cases

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/custody"
	"github.com/vaultkeeper/vaultkeeper/internal/evidence"
)

// --- Mock implementations ---

type mockEvidenceExporter struct {
	items []evidence.EvidenceItem
	err   error
}

func (m *mockEvidenceExporter) ListByCaseForExport(_ context.Context, _ uuid.UUID, _ string) ([]evidence.EvidenceItem, error) {
	return m.items, m.err
}

type mockCustodyExporter struct {
	events []custody.Event
	err    error
}

func (m *mockCustodyExporter) ListAllByCase(_ context.Context, _ uuid.UUID) ([]custody.Event, error) {
	return m.events, m.err
}

type mockCaseExporter struct {
	caseData Case
	err      error
}

func (m *mockCaseExporter) FindByID(_ context.Context, _ uuid.UUID) (Case, error) {
	return m.caseData, m.err
}

type mockFileDownloader struct {
	content string
	err     error
}

func (m *mockFileDownloader) GetObject(_ context.Context, _ string) (io.ReadCloser, int64, string, error) {
	if m.err != nil {
		return nil, 0, "", m.err
	}
	rc := io.NopCloser(strings.NewReader(m.content))
	return rc, int64(len(m.content)), "application/octet-stream", nil
}

type mockExportCustodyLogger struct {
	err error
}

func (m *mockExportCustodyLogger) RecordCaseEvent(_ context.Context, _ uuid.UUID, _ string, _ string, _ map[string]string) error {
	return m.err
}

// --- Helpers ---

func sampleCase() Case {
	return Case{
		ID:            uuid.New(),
		ReferenceCode: "FED-CASE-2025",
		Title:         "Test Case",
		Description:   "A test case for export",
		Jurisdiction:  "Federal",
		Status:        StatusActive,
		CreatedBy:     "user-1",
		CreatedAt:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
	}
}

func sampleExportEvidence() evidence.EvidenceItem {
	num := "EV-001"
	ts := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)
	key := "evidence/abc/def/1/contract.pdf"
	return evidence.EvidenceItem{
		ID:             uuid.New(),
		CaseID:         uuid.New(),
		EvidenceNumber: &num,
		Filename:       "contract.pdf",
		OriginalName:   "contract_original.pdf",
		StorageKey:     &key,
		MimeType:       "application/pdf",
		SizeBytes:      2048,
		SHA256Hash:     "deadbeef1234567890abcdef1234567890abcdef1234567890abcdef12345678",
		Classification: "restricted",
		Description:    "A contract document",
		UploadedBy:     "user-1",
		TSAStatus:      "stamped",
		TSATimestamp:   &ts,
		CreatedAt:      time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC),
	}
}

func sampleCustodyEvents() []custody.Event {
	caseID := uuid.New()
	return []custody.Event{
		{
			ID:           uuid.New(),
			CaseID:       caseID,
			EvidenceID:   uuid.New(),
			Action:       "uploaded",
			ActorUserID:  "user-1",
			Detail:       "Initial upload",
			HashValue:    "hash1",
			PreviousHash: "",
			Timestamp:    time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC),
		},
		{
			ID:           uuid.New(),
			CaseID:       caseID,
			EvidenceID:   uuid.Nil,
			Action:       "reviewed",
			ActorUserID:  "user-2",
			Detail:       "Review complete",
			HashValue:    "hash2",
			PreviousHash: "hash1",
			Timestamp:    time.Date(2025, 3, 11, 0, 0, 0, 0, time.UTC),
		},
	}
}

func newExportService(
	evMock *mockEvidenceExporter,
	custMock *mockCustodyExporter,
	caseMock *mockCaseExporter,
	dlMock *mockFileDownloader,
	logMock *mockExportCustodyLogger,
) *ExportService {
	return NewExportService(evMock, custMock, caseMock, dlMock, logMock)
}

func readZip(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("failed to open zip: %v", err)
	}
	files := make(map[string][]byte)
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("failed to open zip entry %s: %v", f.Name, err)
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("failed to read zip entry %s: %v", f.Name, err)
		}
		files[f.Name] = content
	}
	return files
}

// --- Tests for ExportCase ---

func TestExportCase_ProducesValidZIP(t *testing.T) {
	c := sampleCase()
	items := []evidence.EvidenceItem{sampleExportEvidence()}
	events := sampleCustodyEvents()

	svc := newExportService(
		&mockEvidenceExporter{items: items},
		&mockCustodyExporter{events: events},
		&mockCaseExporter{caseData: c},
		&mockFileDownloader{content: "file-content-here"},
		&mockExportCustodyLogger{},
	)

	var buf bytes.Buffer
	err := svc.ExportCase(context.Background(), c.ID, "case_admin", "user-1", &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	files := readZip(t, buf.Bytes())
	prefix := c.ReferenceCode + "-export/"

	expectedFiles := []string{
		prefix + "manifest.json",
		prefix + "case.json",
		prefix + "metadata.csv",
		prefix + "custody_log.csv",
		prefix + "hashes.csv",
		prefix + "README.txt",
	}
	for _, name := range expectedFiles {
		if _, ok := files[name]; !ok {
			t.Errorf("expected file %q in ZIP", name)
		}
	}

	// Evidence file should also be present
	evFile := prefix + "evidence/EV-001_contract.pdf"
	if _, ok := files[evFile]; !ok {
		t.Errorf("expected evidence file %q in ZIP", evFile)
	}
}

func TestExportCase_ManifestStructure(t *testing.T) {
	c := sampleCase()
	items := []evidence.EvidenceItem{sampleExportEvidence()}

	svc := newExportService(
		&mockEvidenceExporter{items: items},
		&mockCustodyExporter{events: nil},
		&mockCaseExporter{caseData: c},
		&mockFileDownloader{content: "data"},
		&mockExportCustodyLogger{},
	)

	var buf bytes.Buffer
	err := svc.ExportCase(context.Background(), c.ID, "case_admin", "user-1", &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	files := readZip(t, buf.Bytes())
	prefix := c.ReferenceCode + "-export/"

	var manifest map[string]any
	if err := json.Unmarshal(files[prefix+"manifest.json"], &manifest); err != nil {
		t.Fatalf("failed to parse manifest.json: %v", err)
	}

	requiredKeys := []string{"export_date", "version", "case_id", "reference_code", "title", "jurisdiction", "status", "total_items", "exported_by"}
	for _, key := range requiredKeys {
		if _, ok := manifest[key]; !ok {
			t.Errorf("manifest.json missing key %q", key)
		}
	}
	if manifest["case_id"] != c.ID.String() {
		t.Errorf("manifest case_id = %v, want %s", manifest["case_id"], c.ID.String())
	}
	if manifest["reference_code"] != c.ReferenceCode {
		t.Errorf("manifest reference_code = %v, want %s", manifest["reference_code"], c.ReferenceCode)
	}
	if int(manifest["total_items"].(float64)) != len(items) {
		t.Errorf("manifest total_items = %v, want %d", manifest["total_items"], len(items))
	}
	if manifest["exported_by"] != "user-1" {
		t.Errorf("manifest exported_by = %v, want user-1", manifest["exported_by"])
	}
}

func TestExportCase_MetadataCSV(t *testing.T) {
	c := sampleCase()
	items := []evidence.EvidenceItem{sampleExportEvidence()}

	svc := newExportService(
		&mockEvidenceExporter{items: items},
		&mockCustodyExporter{events: nil},
		&mockCaseExporter{caseData: c},
		&mockFileDownloader{content: "data"},
		&mockExportCustodyLogger{},
	)

	var buf bytes.Buffer
	err := svc.ExportCase(context.Background(), c.ID, "case_admin", "user-1", &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	files := readZip(t, buf.Bytes())
	prefix := c.ReferenceCode + "-export/"

	r := csv.NewReader(bytes.NewReader(files[prefix+"metadata.csv"]))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("failed to parse metadata.csv: %v", err)
	}

	expectedHeaders := []string{
		"evidence_number", "filename", "original_name", "mime_type",
		"size_bytes", "sha256_hash", "classification", "description",
		"uploaded_by", "tsa_status", "tsa_timestamp", "created_at",
	}
	if len(records) < 2 {
		t.Fatalf("metadata.csv should have header + %d rows, got %d rows total", len(items), len(records))
	}
	for i, h := range expectedHeaders {
		if records[0][i] != h {
			t.Errorf("metadata.csv header[%d] = %q, want %q", i, records[0][i], h)
		}
	}
	// 1 header + 1 data row
	if len(records) != 1+len(items) {
		t.Errorf("metadata.csv has %d rows, want %d (header + items)", len(records), 1+len(items))
	}
}

func TestExportCase_CustodyLogCSV(t *testing.T) {
	c := sampleCase()
	events := sampleCustodyEvents()

	svc := newExportService(
		&mockEvidenceExporter{items: nil},
		&mockCustodyExporter{events: events},
		&mockCaseExporter{caseData: c},
		&mockFileDownloader{content: "data"},
		&mockExportCustodyLogger{},
	)

	var buf bytes.Buffer
	err := svc.ExportCase(context.Background(), c.ID, "case_admin", "user-1", &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	files := readZip(t, buf.Bytes())
	prefix := c.ReferenceCode + "-export/"

	r := csv.NewReader(bytes.NewReader(files[prefix+"custody_log.csv"]))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("failed to parse custody_log.csv: %v", err)
	}

	expectedHeaders := []string{
		"id", "case_id", "evidence_id", "action", "actor_user_id",
		"detail", "hash_value", "previous_hash", "timestamp",
	}
	for i, h := range expectedHeaders {
		if records[0][i] != h {
			t.Errorf("custody_log.csv header[%d] = %q, want %q", i, records[0][i], h)
		}
	}
	// header + event rows
	if len(records) != 1+len(events) {
		t.Errorf("custody_log.csv has %d rows, want %d", len(records), 1+len(events))
	}
}

func TestExportCase_HashesCSV(t *testing.T) {
	c := sampleCase()
	items := []evidence.EvidenceItem{sampleExportEvidence()}

	svc := newExportService(
		&mockEvidenceExporter{items: items},
		&mockCustodyExporter{events: nil},
		&mockCaseExporter{caseData: c},
		&mockFileDownloader{content: "data"},
		&mockExportCustodyLogger{},
	)

	var buf bytes.Buffer
	err := svc.ExportCase(context.Background(), c.ID, "case_admin", "user-1", &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	files := readZip(t, buf.Bytes())
	prefix := c.ReferenceCode + "-export/"

	r := csv.NewReader(bytes.NewReader(files[prefix+"hashes.csv"]))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("failed to parse hashes.csv: %v", err)
	}

	expectedHeaders := []string{"evidence_number", "filename", "sha256_hash", "tsa_status", "tsa_timestamp"}
	for i, h := range expectedHeaders {
		if records[0][i] != h {
			t.Errorf("hashes.csv header[%d] = %q, want %q", i, records[0][i], h)
		}
	}
	if len(records) != 1+len(items) {
		t.Errorf("hashes.csv has %d rows, want %d", len(records), 1+len(items))
	}
	// Verify hash value matches evidence item
	if records[1][2] != items[0].SHA256Hash {
		t.Errorf("hashes.csv hash = %q, want %q", records[1][2], items[0].SHA256Hash)
	}
}

func TestExportCase_EmptyEvidence(t *testing.T) {
	c := sampleCase()

	svc := newExportService(
		&mockEvidenceExporter{items: nil},
		&mockCustodyExporter{events: nil},
		&mockCaseExporter{caseData: c},
		&mockFileDownloader{content: ""},
		&mockExportCustodyLogger{},
	)

	var buf bytes.Buffer
	err := svc.ExportCase(context.Background(), c.ID, "case_admin", "user-1", &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	files := readZip(t, buf.Bytes())
	prefix := c.ReferenceCode + "-export/"

	// Should still have all structural files
	for _, name := range []string{"manifest.json", "case.json", "metadata.csv", "custody_log.csv", "hashes.csv", "README.txt"} {
		if _, ok := files[prefix+name]; !ok {
			t.Errorf("expected file %q in ZIP even with empty evidence", prefix+name)
		}
	}

	// No evidence/ files
	for name := range files {
		if strings.Contains(name, "evidence/") {
			t.Errorf("did not expect evidence files in ZIP, found %q", name)
		}
	}
}

func TestExportCase_NilStorageKey(t *testing.T) {
	c := sampleCase()
	item := sampleExportEvidence()
	item.StorageKey = nil // no file to download

	svc := newExportService(
		&mockEvidenceExporter{items: []evidence.EvidenceItem{item}},
		&mockCustodyExporter{events: nil},
		&mockCaseExporter{caseData: c},
		&mockFileDownloader{content: "data"},
		&mockExportCustodyLogger{},
	)

	var buf bytes.Buffer
	err := svc.ExportCase(context.Background(), c.ID, "case_admin", "user-1", &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	files := readZip(t, buf.Bytes())
	// Metadata should still have the item, but no evidence file
	for name := range files {
		if strings.Contains(name, "evidence/") {
			t.Errorf("did not expect evidence file for nil StorageKey, found %q", name)
		}
	}
}

func TestExportCase_NilEvidenceNumber(t *testing.T) {
	c := sampleCase()
	item := sampleExportEvidence()
	item.EvidenceNumber = nil
	item.TSATimestamp = nil

	svc := newExportService(
		&mockEvidenceExporter{items: []evidence.EvidenceItem{item}},
		&mockCustodyExporter{events: nil},
		&mockCaseExporter{caseData: c},
		&mockFileDownloader{content: "data"},
		&mockExportCustodyLogger{},
	)

	var buf bytes.Buffer
	err := svc.ExportCase(context.Background(), c.ID, "case_admin", "user-1", &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	files := readZip(t, buf.Bytes())
	prefix := c.ReferenceCode + "-export/"

	// Evidence file should be named _contract.pdf (empty evidence number)
	evFile := prefix + "evidence/_contract.pdf"
	if _, ok := files[evFile]; !ok {
		t.Errorf("expected evidence file %q in ZIP", evFile)
	}
}

func TestExportCase_CaseRepoError(t *testing.T) {
	svc := newExportService(
		&mockEvidenceExporter{},
		&mockCustodyExporter{},
		&mockCaseExporter{err: fmt.Errorf("db error")},
		&mockFileDownloader{},
		&mockExportCustodyLogger{},
	)

	var buf bytes.Buffer
	err := svc.ExportCase(context.Background(), uuid.New(), "case_admin", "user-1", &buf)
	if err == nil {
		t.Fatal("expected error when case repo fails")
	}
}

func TestExportCase_EvidenceRepoError(t *testing.T) {
	svc := newExportService(
		&mockEvidenceExporter{err: fmt.Errorf("evidence error")},
		&mockCustodyExporter{},
		&mockCaseExporter{caseData: sampleCase()},
		&mockFileDownloader{},
		&mockExportCustodyLogger{},
	)

	var buf bytes.Buffer
	err := svc.ExportCase(context.Background(), uuid.New(), "case_admin", "user-1", &buf)
	if err == nil {
		t.Fatal("expected error when evidence repo fails")
	}
}

func TestExportCase_CustodyRepoError(t *testing.T) {
	svc := newExportService(
		&mockEvidenceExporter{items: nil},
		&mockCustodyExporter{err: fmt.Errorf("custody error")},
		&mockCaseExporter{caseData: sampleCase()},
		&mockFileDownloader{},
		&mockExportCustodyLogger{},
	)

	var buf bytes.Buffer
	err := svc.ExportCase(context.Background(), uuid.New(), "case_admin", "user-1", &buf)
	if err == nil {
		t.Fatal("expected error when custody repo fails")
	}
}

func TestExportCase_FileDownloadError(t *testing.T) {
	c := sampleCase()
	items := []evidence.EvidenceItem{sampleExportEvidence()}

	svc := newExportService(
		&mockEvidenceExporter{items: items},
		&mockCustodyExporter{events: nil},
		&mockCaseExporter{caseData: c},
		&mockFileDownloader{err: fmt.Errorf("s3 error")},
		&mockExportCustodyLogger{},
	)

	var buf bytes.Buffer
	err := svc.ExportCase(context.Background(), c.ID, "case_admin", "user-1", &buf)
	if err == nil {
		t.Fatal("expected error when file download fails")
	}
}

func TestExportCase_CustodyLoggerErrorIgnored(t *testing.T) {
	c := sampleCase()

	svc := newExportService(
		&mockEvidenceExporter{items: nil},
		&mockCustodyExporter{events: nil},
		&mockCaseExporter{caseData: c},
		&mockFileDownloader{content: ""},
		&mockExportCustodyLogger{err: fmt.Errorf("log error")},
	)

	var buf bytes.Buffer
	err := svc.ExportCase(context.Background(), c.ID, "case_admin", "user-1", &buf)
	if err != nil {
		t.Fatalf("custody logger error should be ignored, got: %v", err)
	}
}

func TestExportCase_READMEContent(t *testing.T) {
	c := sampleCase()

	svc := newExportService(
		&mockEvidenceExporter{items: nil},
		&mockCustodyExporter{events: nil},
		&mockCaseExporter{caseData: c},
		&mockFileDownloader{content: ""},
		&mockExportCustodyLogger{},
	)

	var buf bytes.Buffer
	err := svc.ExportCase(context.Background(), c.ID, "case_admin", "user-1", &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	files := readZip(t, buf.Bytes())
	prefix := c.ReferenceCode + "-export/"
	readme := string(files[prefix+"README.txt"])

	if !strings.Contains(readme, c.ReferenceCode) {
		t.Error("README should contain reference code")
	}
	if !strings.Contains(readme, c.Title) {
		t.Error("README should contain case title")
	}
	if !strings.Contains(readme, c.Jurisdiction) {
		t.Error("README should contain jurisdiction")
	}
}

func TestExportCase_CaseJSON(t *testing.T) {
	c := sampleCase()

	svc := newExportService(
		&mockEvidenceExporter{items: nil},
		&mockCustodyExporter{events: nil},
		&mockCaseExporter{caseData: c},
		&mockFileDownloader{content: ""},
		&mockExportCustodyLogger{},
	)

	var buf bytes.Buffer
	err := svc.ExportCase(context.Background(), c.ID, "case_admin", "user-1", &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	files := readZip(t, buf.Bytes())
	prefix := c.ReferenceCode + "-export/"

	var caseJSON map[string]any
	if err := json.Unmarshal(files[prefix+"case.json"], &caseJSON); err != nil {
		t.Fatalf("failed to parse case.json: %v", err)
	}
	if caseJSON["reference_code"] != c.ReferenceCode {
		t.Errorf("case.json reference_code = %v, want %s", caseJSON["reference_code"], c.ReferenceCode)
	}
}

func TestExportCase_CustodyLogNilEvidenceID(t *testing.T) {
	c := sampleCase()
	events := []custody.Event{
		{
			ID:           uuid.New(),
			CaseID:       c.ID,
			EvidenceID:   uuid.Nil,
			Action:       "case_created",
			ActorUserID:  "user-1",
			Detail:       "Created",
			HashValue:    "h1",
			PreviousHash: "",
			Timestamp:    time.Now(),
		},
	}

	svc := newExportService(
		&mockEvidenceExporter{items: nil},
		&mockCustodyExporter{events: events},
		&mockCaseExporter{caseData: c},
		&mockFileDownloader{content: ""},
		&mockExportCustodyLogger{},
	)

	var buf bytes.Buffer
	err := svc.ExportCase(context.Background(), c.ID, "case_admin", "user-1", &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	files := readZip(t, buf.Bytes())
	prefix := c.ReferenceCode + "-export/"

	r := csv.NewReader(bytes.NewReader(files[prefix+"custody_log.csv"]))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("failed to parse custody_log.csv: %v", err)
	}
	// evidence_id column (index 2) should be empty for uuid.Nil
	if records[1][2] != "" {
		t.Errorf("expected empty evidence_id for Nil UUID, got %q", records[1][2])
	}
}

// --- Tests for GetReferenceCode ---

func TestExportService_GetReferenceCode_Success(t *testing.T) {
	c := sampleCase()
	svc := newExportService(
		&mockEvidenceExporter{},
		&mockCustodyExporter{},
		&mockCaseExporter{caseData: c},
		&mockFileDownloader{},
		&mockExportCustodyLogger{},
	)

	code, err := svc.GetReferenceCode(context.Background(), c.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != c.ReferenceCode {
		t.Errorf("got %q, want %q", code, c.ReferenceCode)
	}
}

func TestExportService_GetReferenceCode_Error(t *testing.T) {
	svc := newExportService(
		&mockEvidenceExporter{},
		&mockCustodyExporter{},
		&mockCaseExporter{err: fmt.Errorf("not found")},
		&mockFileDownloader{},
		&mockExportCustodyLogger{},
	)

	_, err := svc.GetReferenceCode(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- Test multiple evidence items ---

// --- Tests for io.Copy error during evidence file write ---

type failingReadCloser struct{}

func (f *failingReadCloser) Read(_ []byte) (int, error) {
	return 0, fmt.Errorf("disk read error")
}

func (f *failingReadCloser) Close() error { return nil }

type failingFileDownloader struct{}

func (m *failingFileDownloader) GetObject(_ context.Context, _ string) (io.ReadCloser, int64, string, error) {
	return &failingReadCloser{}, 100, "application/octet-stream", nil
}

func TestExportCase_CopyError(t *testing.T) {
	c := sampleCase()
	items := []evidence.EvidenceItem{sampleExportEvidence()}

	svc := NewExportService(
		&mockEvidenceExporter{items: items},
		&mockCustodyExporter{events: nil},
		&mockCaseExporter{caseData: c},
		&failingFileDownloader{},
		&mockExportCustodyLogger{},
	)

	var buf bytes.Buffer
	err := svc.ExportCase(context.Background(), c.ID, "case_admin", "user-1", &buf)
	if err == nil {
		t.Fatal("expected error when io.Copy fails")
	}
	if !strings.Contains(err.Error(), "copy evidence file") {
		t.Errorf("error = %q, want 'copy evidence file'", err.Error())
	}
}

// --- Tests for error branches in write* functions ---

// --- Tests that exercise error branches in write* methods ---
// zip.Writer calls the underlying writer's Write during Create() (for the
// local file header) and during Close() (for central directory). By using
// a writer that fails at precise byte offsets, we can trigger errors at
// different stages.

// --- Tests for write* error branches using a mock zipCreator ---

// failingZipCreator returns a writer that fails after a certain number of
// bytes, or fails on Create entirely.
type failingZipCreator struct {
	createErr    error
	writeErr     error
	failAfterN   int // number of bytes to succeed before failing (-1 = always succeed)
	createCalled int
	failOnCreate int // fail on the Nth Create call (0 = never fail on create)
}

func (f *failingZipCreator) Create(_ string) (io.Writer, error) {
	f.createCalled++
	if f.failOnCreate > 0 && f.createCalled >= f.failOnCreate {
		return nil, f.createErr
	}
	return &limitedWriter{err: f.writeErr, limit: f.failAfterN}, nil
}

// limitedWriter writes up to limit bytes then returns an error.
type limitedWriter struct {
	err     error
	limit   int
	written int
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if w.limit >= 0 && w.written+len(p) > w.limit {
		// Write partial
		n := w.limit - w.written
		if n < 0 {
			n = 0
		}
		w.written += n
		if w.err != nil {
			return n, w.err
		}
		return n, fmt.Errorf("writer limit exceeded")
	}
	w.written += len(p)
	return len(p), nil
}

func TestWriteManifest_CreateError(t *testing.T) {
	svc := NewExportService(nil, nil, nil, nil, nil)
	zc := &failingZipCreator{createErr: fmt.Errorf("create failed"), failOnCreate: 1}
	err := svc.writeManifest(zc, "p/", sampleCase(), nil, "u")
	if err == nil {
		t.Fatal("expected error from writeManifest Create")
	}
	if !strings.Contains(err.Error(), "create manifest entry") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestWriteManifest_EncodeError(t *testing.T) {
	svc := NewExportService(nil, nil, nil, nil, nil)
	zc := &failingZipCreator{failAfterN: 5, writeErr: fmt.Errorf("write failed")}
	err := svc.writeManifest(zc, "p/", sampleCase(), nil, "u")
	if err == nil {
		t.Fatal("expected error from writeManifest Encode")
	}
	if !strings.Contains(err.Error(), "encode manifest") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestWriteCaseJSON_CreateError(t *testing.T) {
	svc := NewExportService(nil, nil, nil, nil, nil)
	zc := &failingZipCreator{createErr: fmt.Errorf("create failed"), failOnCreate: 1}
	err := svc.writeCaseJSON(zc, "p/", sampleCase())
	if err == nil {
		t.Fatal("expected error from writeCaseJSON Create")
	}
	if !strings.Contains(err.Error(), "create case.json entry") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestWriteCaseJSON_EncodeError(t *testing.T) {
	svc := NewExportService(nil, nil, nil, nil, nil)
	zc := &failingZipCreator{failAfterN: 5, writeErr: fmt.Errorf("write failed")}
	err := svc.writeCaseJSON(zc, "p/", sampleCase())
	if err == nil {
		t.Fatal("expected error from writeCaseJSON Encode")
	}
	if !strings.Contains(err.Error(), "encode case.json") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestWriteEvidenceFiles_CreateError(t *testing.T) {
	svc := NewExportService(nil, nil, nil, &mockFileDownloader{content: "data"}, nil)
	zc := &failingZipCreator{createErr: fmt.Errorf("create failed"), failOnCreate: 1}
	item := sampleExportEvidence()
	err := svc.writeEvidenceFiles(context.Background(), zc, "p/", []evidence.EvidenceItem{item})
	if err == nil {
		t.Fatal("expected error from writeEvidenceFiles Create")
	}
	if !strings.Contains(err.Error(), "create evidence zip entry") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestWriteMetadataCSV_CreateError(t *testing.T) {
	svc := NewExportService(nil, nil, nil, nil, nil)
	zc := &failingZipCreator{createErr: fmt.Errorf("create failed"), failOnCreate: 1}
	err := svc.writeMetadataCSV(zc, "p/", []evidence.EvidenceItem{sampleExportEvidence()})
	if err == nil {
		t.Fatal("expected error from writeMetadataCSV Create")
	}
	if !strings.Contains(err.Error(), "create metadata.csv entry") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestWriteMetadataCSV_HeaderWriteError(t *testing.T) {
	svc := NewExportService(nil, nil, nil, nil, nil)
	zc := &failingZipCreator{failAfterN: 5, writeErr: fmt.Errorf("write failed")}
	err := svc.writeMetadataCSV(zc, "p/", []evidence.EvidenceItem{sampleExportEvidence()})
	if err == nil {
		t.Fatal("expected error from writeMetadataCSV header write")
	}
}

func TestWriteMetadataCSV_RowWriteError(t *testing.T) {
	svc := NewExportService(nil, nil, nil, nil, nil)
	// Allow header to succeed but fail on data row
	zc := &failingZipCreator{failAfterN: 200, writeErr: fmt.Errorf("write failed")}
	err := svc.writeMetadataCSV(zc, "p/", []evidence.EvidenceItem{sampleExportEvidence()})
	if err == nil {
		t.Fatal("expected error from writeMetadataCSV row write")
	}
}

func TestWriteCustodyCSV_CreateError(t *testing.T) {
	svc := NewExportService(nil, nil, nil, nil, nil)
	zc := &failingZipCreator{createErr: fmt.Errorf("create failed"), failOnCreate: 1}
	err := svc.writeCustodyCSV(zc, "p/", sampleCustodyEvents())
	if err == nil {
		t.Fatal("expected error from writeCustodyCSV Create")
	}
	if !strings.Contains(err.Error(), "create custody_log.csv entry") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestWriteCustodyCSV_HeaderWriteError(t *testing.T) {
	svc := NewExportService(nil, nil, nil, nil, nil)
	zc := &failingZipCreator{failAfterN: 5, writeErr: fmt.Errorf("write failed")}
	err := svc.writeCustodyCSV(zc, "p/", sampleCustodyEvents())
	if err == nil {
		t.Fatal("expected error from writeCustodyCSV header write")
	}
}

func TestWriteCustodyCSV_RowWriteError(t *testing.T) {
	svc := NewExportService(nil, nil, nil, nil, nil)
	zc := &failingZipCreator{failAfterN: 200, writeErr: fmt.Errorf("write failed")}
	err := svc.writeCustodyCSV(zc, "p/", sampleCustodyEvents())
	if err == nil {
		t.Fatal("expected error from writeCustodyCSV row write")
	}
}

func TestWriteHashesCSV_CreateError(t *testing.T) {
	svc := NewExportService(nil, nil, nil, nil, nil)
	zc := &failingZipCreator{createErr: fmt.Errorf("create failed"), failOnCreate: 1}
	err := svc.writeHashesCSV(zc, "p/", []evidence.EvidenceItem{sampleExportEvidence()})
	if err == nil {
		t.Fatal("expected error from writeHashesCSV Create")
	}
	if !strings.Contains(err.Error(), "create hashes.csv entry") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestWriteHashesCSV_HeaderWriteError(t *testing.T) {
	svc := NewExportService(nil, nil, nil, nil, nil)
	zc := &failingZipCreator{failAfterN: 5, writeErr: fmt.Errorf("write failed")}
	err := svc.writeHashesCSV(zc, "p/", []evidence.EvidenceItem{sampleExportEvidence()})
	if err == nil {
		t.Fatal("expected error from writeHashesCSV header write")
	}
}

func TestWriteHashesCSV_RowWriteError(t *testing.T) {
	svc := NewExportService(nil, nil, nil, nil, nil)
	// csv.Writer buffers internally. It flushes on Flush() at the end.
	// We need the limit to be enough for the header but fail on the data row flush.
	// The header "evidence_number,filename,sha256_hash,tsa_status,tsa_timestamp\n" is ~60 bytes.
	// The data row with a 64-char hash is ~100+ bytes. Total flush ~170+ bytes.
	// Set limit high enough for header but not for header + data + flush.
	zc := &failingZipCreator{failAfterN: 100, writeErr: fmt.Errorf("write failed")}
	err := svc.writeHashesCSV(zc, "p/", []evidence.EvidenceItem{sampleExportEvidence()})
	if err == nil {
		t.Fatal("expected error from writeHashesCSV row write")
	}
}

func TestWriteREADME_CreateError(t *testing.T) {
	svc := NewExportService(nil, nil, nil, nil, nil)
	zc := &failingZipCreator{createErr: fmt.Errorf("create failed"), failOnCreate: 1}
	err := svc.writeREADME(zc, "p/", sampleCase())
	if err == nil {
		t.Fatal("expected error from writeREADME Create")
	}
	if !strings.Contains(err.Error(), "create README.txt entry") {
		t.Errorf("error = %q", err.Error())
	}
}

// --- Tests for buildArchive error propagation ---
// Each write* function's error is propagated through buildArchive.
// By making failOnCreate trigger at different call numbers, we can exercise
// each error return in buildArchive.

func TestBuildArchive_ManifestError(t *testing.T) {
	svc := NewExportService(nil, nil, nil, &mockFileDownloader{content: "data"}, nil)
	// Fail on 1st Create (manifest)
	zc := &failingZipCreator{createErr: fmt.Errorf("fail"), failOnCreate: 1}
	err := svc.buildArchive(context.Background(), zc, "p/", sampleCase(), nil, nil, "u")
	if err == nil {
		t.Fatal("expected error from manifest")
	}
}

func TestBuildArchive_CaseJSONError(t *testing.T) {
	svc := NewExportService(nil, nil, nil, &mockFileDownloader{content: "data"}, nil)
	// Fail on 2nd Create (case.json)
	zc := &failingZipCreator{createErr: fmt.Errorf("fail"), failOnCreate: 2, failAfterN: -1}
	err := svc.buildArchive(context.Background(), zc, "p/", sampleCase(), nil, nil, "u")
	if err == nil {
		t.Fatal("expected error from case.json")
	}
}

func TestBuildArchive_EvidenceError(t *testing.T) {
	svc := NewExportService(nil, nil, nil, &mockFileDownloader{err: fmt.Errorf("s3 fail")}, nil)
	item := sampleExportEvidence()
	// Fail on 3rd Create (evidence file) - but evidence error comes from download, not Create
	zc := &failingZipCreator{failAfterN: -1}
	err := svc.buildArchive(context.Background(), zc, "p/", sampleCase(), []evidence.EvidenceItem{item}, nil, "u")
	if err == nil {
		t.Fatal("expected error from evidence")
	}
}

func TestBuildArchive_MetadataError(t *testing.T) {
	svc := NewExportService(nil, nil, nil, &mockFileDownloader{content: "data"}, nil)
	item := sampleExportEvidence()
	// 1=manifest, 2=case.json, 3=evidence, 4=metadata
	zc := &failingZipCreator{createErr: fmt.Errorf("fail"), failOnCreate: 4, failAfterN: -1}
	err := svc.buildArchive(context.Background(), zc, "p/", sampleCase(), []evidence.EvidenceItem{item}, nil, "u")
	if err == nil {
		t.Fatal("expected error from metadata")
	}
}

func TestBuildArchive_CustodyError(t *testing.T) {
	svc := NewExportService(nil, nil, nil, &mockFileDownloader{content: "data"}, nil)
	item := sampleExportEvidence()
	// 1=manifest, 2=case.json, 3=evidence, 4=metadata, 5=custody
	zc := &failingZipCreator{createErr: fmt.Errorf("fail"), failOnCreate: 5, failAfterN: -1}
	err := svc.buildArchive(context.Background(), zc, "p/", sampleCase(), []evidence.EvidenceItem{item}, sampleCustodyEvents(), "u")
	if err == nil {
		t.Fatal("expected error from custody")
	}
}

func TestBuildArchive_HashesError(t *testing.T) {
	svc := NewExportService(nil, nil, nil, &mockFileDownloader{content: "data"}, nil)
	item := sampleExportEvidence()
	// 1=manifest, 2=case.json, 3=evidence, 4=metadata, 5=custody, 6=hashes
	zc := &failingZipCreator{createErr: fmt.Errorf("fail"), failOnCreate: 6, failAfterN: -1}
	err := svc.buildArchive(context.Background(), zc, "p/", sampleCase(), []evidence.EvidenceItem{item}, sampleCustodyEvents(), "u")
	if err == nil {
		t.Fatal("expected error from hashes")
	}
}

func TestBuildArchive_READMEError(t *testing.T) {
	svc := NewExportService(nil, nil, nil, &mockFileDownloader{content: "data"}, nil)
	item := sampleExportEvidence()
	// 1=manifest, 2=case.json, 3=evidence, 4=metadata, 5=custody, 6=hashes, 7=readme
	zc := &failingZipCreator{createErr: fmt.Errorf("fail"), failOnCreate: 7, failAfterN: -1}
	err := svc.buildArchive(context.Background(), zc, "p/", sampleCase(), []evidence.EvidenceItem{item}, sampleCustodyEvents(), "u")
	if err == nil {
		t.Fatal("expected error from README")
	}
}

// --- Tests for CSV row write errors ---
// csv.Writer.Write returns an error when its internal bufio.Writer
// returns an error. This only happens when the buffer overflows and
// the underlying writer fails.

func TestWriteMetadataCSV_RowFlushError(t *testing.T) {
	svc := NewExportService(nil, nil, nil, nil, nil)
	// The header + row together exceed 100 bytes, so Flush will trigger the error
	zc := &failingZipCreator{failAfterN: 100, writeErr: fmt.Errorf("write failed")}
	err := svc.writeMetadataCSV(zc, "p/", []evidence.EvidenceItem{sampleExportEvidence()})
	if err == nil {
		t.Fatal("expected error from writeMetadataCSV flush")
	}
}

func TestWriteCustodyCSV_RowFlushError(t *testing.T) {
	svc := NewExportService(nil, nil, nil, nil, nil)
	zc := &failingZipCreator{failAfterN: 100, writeErr: fmt.Errorf("write failed")}
	err := svc.writeCustodyCSV(zc, "p/", sampleCustodyEvents())
	if err == nil {
		t.Fatal("expected error from writeCustodyCSV flush")
	}
}

// --- Test ExportCase close error + buildArchive error ---

func TestExportCase_CloseError(t *testing.T) {
	c := sampleCase()
	items := []evidence.EvidenceItem{sampleExportEvidence()}

	svc := NewExportService(
		&mockEvidenceExporter{items: items},
		&mockCustodyExporter{events: sampleCustodyEvents()},
		&mockCaseExporter{caseData: c},
		&mockFileDownloader{content: "file-data"},
		&mockExportCustodyLogger{},
	)

	// First measure how many bytes a successful export produces
	var buf bytes.Buffer
	if err := svc.ExportCase(context.Background(), c.ID, "case_admin", "user-1", &buf); err != nil {
		t.Fatalf("measuring: %v", err)
	}

	// Now use a writer that accepts all entry data but fails when
	// zip.Close writes the central directory (which happens after
	// all entries are written). Set the limit just before the total.
	for cutoff := buf.Len() - 1; cutoff >= buf.Len()-200; cutoff-- {
		w := &cappedWriter{limit: cutoff}
		err := svc.ExportCase(context.Background(), c.ID, "case_admin", "user-1", w)
		if err != nil && strings.Contains(err.Error(), "close zip writer") {
			return // success - we hit the close error path
		}
	}
	t.Fatal("could not trigger close zip writer error")
}

type cappedWriter struct {
	limit   int
	written int
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	if w.written+len(p) > w.limit {
		return 0, fmt.Errorf("disk full")
	}
	w.written += len(p)
	return len(p), nil
}

// --- Tests for csv.Write() error with buffer overflow ---
// csv.Writer uses a 4096-byte bufio.Writer internally. To trigger
// an error from csv.Write(), the accumulated data must exceed the
// buffer size, forcing a flush to the underlying writer.

func largeEvidenceItems(n int) []evidence.EvidenceItem {
	items := make([]evidence.EvidenceItem, n)
	for i := 0; i < n; i++ {
		num := fmt.Sprintf("EV-%03d", i)
		ts := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)
		key := "evidence/" + strings.Repeat("x", 100) + fmt.Sprintf("/%d.pdf", i)
		items[i] = evidence.EvidenceItem{
			ID:             uuid.New(),
			CaseID:         uuid.New(),
			EvidenceNumber: &num,
			Filename:       strings.Repeat("f", 200) + ".pdf",
			OriginalName:   strings.Repeat("o", 200) + ".pdf",
			StorageKey:     &key,
			MimeType:       "application/pdf",
			SizeBytes:      99999,
			SHA256Hash:     strings.Repeat("a", 64),
			Classification: "restricted",
			Description:    strings.Repeat("d", 500),
			UploadedBy:     "user-1",
			TSAStatus:      "stamped",
			TSATimestamp:   &ts,
			CreatedAt:      time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC),
		}
	}
	return items
}

func largeCustodyEvents(n int) []custody.Event {
	events := make([]custody.Event, n)
	for i := 0; i < n; i++ {
		events[i] = custody.Event{
			ID:           uuid.New(),
			CaseID:       uuid.New(),
			EvidenceID:   uuid.New(),
			Action:       strings.Repeat("a", 200),
			ActorUserID:  strings.Repeat("u", 100),
			Detail:       strings.Repeat("d", 500),
			HashValue:    strings.Repeat("h", 64),
			PreviousHash: strings.Repeat("p", 64),
			Timestamp:    time.Now(),
		}
	}
	return events
}

// immediateFailWriter returns an error on the very first Write call with
// any data, no matter how small. csv.Writer wraps this in a bufio.Writer,
// but bufio.WriteString / WriteRune also check b.err which is set after
// any failed flush. To trigger csv.Write header errors, we need to cause
// the first bufio write to fail, which only happens if data exceeds the
// bufio buffer or a previous flush failed.
//
// Since csv.NewWriter creates a fresh bufio with no error, and headers are
// small, we cannot trigger the header Write error through normal means.
// Instead, we exercise the Flush/Error path which catches the same errors.

func TestWriteMetadataCSV_HeaderWriteError_Overflow(t *testing.T) {
	svc := NewExportService(nil, nil, nil, nil, nil)
	// Use 0 bytes limit to fail immediately on first write
	zc := &failingZipCreator{failAfterN: 0, writeErr: fmt.Errorf("write failed")}
	items := largeEvidenceItems(10)
	err := svc.writeMetadataCSV(zc, "p/", items)
	if err == nil {
		t.Fatal("expected error from writeMetadataCSV header overflow")
	}
}

func TestWriteMetadataCSV_RowWriteError_Overflow(t *testing.T) {
	svc := NewExportService(nil, nil, nil, nil, nil)
	// Allow the header (~120 bytes) but fail soon after, before the large rows
	zc := &failingZipCreator{failAfterN: 200, writeErr: fmt.Errorf("write failed")}
	items := largeEvidenceItems(10)
	err := svc.writeMetadataCSV(zc, "p/", items)
	if err == nil {
		t.Fatal("expected error from writeMetadataCSV row overflow")
	}
}

func TestWriteCustodyCSV_HeaderWriteError_Overflow(t *testing.T) {
	svc := NewExportService(nil, nil, nil, nil, nil)
	zc := &failingZipCreator{failAfterN: 0, writeErr: fmt.Errorf("write failed")}
	events := largeCustodyEvents(10)
	err := svc.writeCustodyCSV(zc, "p/", events)
	if err == nil {
		t.Fatal("expected error from writeCustodyCSV header overflow")
	}
}

func TestWriteCustodyCSV_RowWriteError_Overflow(t *testing.T) {
	svc := NewExportService(nil, nil, nil, nil, nil)
	zc := &failingZipCreator{failAfterN: 200, writeErr: fmt.Errorf("write failed")}
	events := largeCustodyEvents(10)
	err := svc.writeCustodyCSV(zc, "p/", events)
	if err == nil {
		t.Fatal("expected error from writeCustodyCSV row overflow")
	}
}

func TestWriteHashesCSV_HeaderWriteError_Overflow(t *testing.T) {
	svc := NewExportService(nil, nil, nil, nil, nil)
	zc := &failingZipCreator{failAfterN: 0, writeErr: fmt.Errorf("write failed")}
	items := largeEvidenceItems(10)
	err := svc.writeHashesCSV(zc, "p/", items)
	if err == nil {
		t.Fatal("expected error from writeHashesCSV header overflow")
	}
}

func TestWriteHashesCSV_RowWriteError_Overflow(t *testing.T) {
	svc := NewExportService(nil, nil, nil, nil, nil)
	zc := &failingZipCreator{failAfterN: 200, writeErr: fmt.Errorf("write failed")}
	items := largeEvidenceItems(10)
	err := svc.writeHashesCSV(zc, "p/", items)
	if err == nil {
		t.Fatal("expected error from writeHashesCSV row overflow")
	}
}

func TestWriteREADME_WriteStringError(t *testing.T) {
	svc := NewExportService(nil, nil, nil, nil, nil)
	zc := &failingZipCreator{failAfterN: 5, writeErr: fmt.Errorf("write failed")}
	err := svc.writeREADME(zc, "p/", sampleCase())
	if err == nil {
		t.Fatal("expected error from writeREADME WriteString")
	}
	if !strings.Contains(err.Error(), "write README.txt") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestExportCase_MultipleEvidenceItems(t *testing.T) {
	c := sampleCase()
	item1 := sampleExportEvidence()
	item2 := sampleExportEvidence()
	num2 := "EV-002"
	item2.EvidenceNumber = &num2
	item2.Filename = "photo.jpg"

	svc := newExportService(
		&mockEvidenceExporter{items: []evidence.EvidenceItem{item1, item2}},
		&mockCustodyExporter{events: nil},
		&mockCaseExporter{caseData: c},
		&mockFileDownloader{content: "binary-data"},
		&mockExportCustodyLogger{},
	)

	var buf bytes.Buffer
	err := svc.ExportCase(context.Background(), c.ID, "case_admin", "user-1", &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	files := readZip(t, buf.Bytes())
	prefix := c.ReferenceCode + "-export/"

	if _, ok := files[prefix+"evidence/EV-001_contract.pdf"]; !ok {
		t.Error("missing first evidence file")
	}
	if _, ok := files[prefix+"evidence/EV-002_photo.jpg"]; !ok {
		t.Error("missing second evidence file")
	}

	// Verify metadata has 2 data rows
	r := csv.NewReader(bytes.NewReader(files[prefix+"metadata.csv"]))
	records, _ := r.ReadAll()
	if len(records) != 3 { // header + 2 items
		t.Errorf("metadata.csv has %d rows, want 3", len(records))
	}
}
