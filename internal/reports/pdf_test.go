package reports

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/custody"
	"github.com/vaultkeeper/vaultkeeper/internal/evidence"
)

// --- Mock implementations ---

type mockCustodySource struct {
	allByCaseEvents    []custody.Event
	allByCaseErr       error
	byEvidenceEvents   []custody.Event
	byEvidenceTotal    int
	byEvidenceErr      error
}

func (m *mockCustodySource) ListAllByCase(_ context.Context, _ uuid.UUID) ([]custody.Event, error) {
	return m.allByCaseEvents, m.allByCaseErr
}

func (m *mockCustodySource) ListByEvidence(_ context.Context, _ uuid.UUID, limit int, _ string) ([]custody.Event, int, error) {
	return m.byEvidenceEvents, m.byEvidenceTotal, m.byEvidenceErr
}

type mockEvidenceSource struct {
	findByCaseItems []evidence.EvidenceItem
	findByCaseTotal int
	findByCaseErr   error
	findByIDItem    evidence.EvidenceItem
	findByIDErr     error
}

func (m *mockEvidenceSource) FindByCase(_ context.Context, _ evidence.EvidenceFilter, _ evidence.Pagination) ([]evidence.EvidenceItem, int, error) {
	return m.findByCaseItems, m.findByCaseTotal, m.findByCaseErr
}

func (m *mockEvidenceSource) FindByID(_ context.Context, _ uuid.UUID) (evidence.EvidenceItem, error) {
	return m.findByIDItem, m.findByIDErr
}

type mockCaseSource struct {
	caseRecord CaseRecord
	err        error
}

func (m *mockCaseSource) FindByID(_ context.Context, _ uuid.UUID) (CaseRecord, error) {
	return m.caseRecord, m.err
}

// --- Helpers ---

func sampleCaseRecord() CaseRecord {
	return CaseRecord{
		ID:            uuid.New(),
		ReferenceCode: "REF-TEST-2025",
		Title:         "Test Case Alpha",
		Jurisdiction:  "Federal District Court",
		Status:        "active",
	}
}

func sampleEvidenceItem() evidence.EvidenceItem {
	num := "EV-001"
	ts := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	key := "evidence/abc/def/1/file.pdf"
	return evidence.EvidenceItem{
		ID:             uuid.New(),
		CaseID:         uuid.New(),
		EvidenceNumber: &num,
		Filename:       "contract.pdf",
		OriginalName:   "contract_original.pdf",
		MimeType:       "application/pdf",
		SizeBytes:      1024,
		SHA256Hash:     "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		Classification: "restricted",
		Description:    "Sample contract",
		UploadedBy:     "user-1",
		TSAStatus:      "stamped",
		TSATimestamp:    &ts,
		StorageKey:     &key,
		CreatedAt:      time.Date(2025, 1, 10, 8, 0, 0, 0, time.UTC),
	}
}

func buildValidChain(count int) []custody.Event {
	events := make([]custody.Event, count)
	prevHash := ""
	for i := 0; i < count; i++ {
		e := custody.Event{
			ID:           uuid.New(),
			CaseID:       uuid.New(),
			EvidenceID:   uuid.New(),
			Action:       "uploaded",
			ActorUserID:  "user-1",
			Detail:       fmt.Sprintf(`{"step":%d}`, i),
			PreviousHash: prevHash,
			Timestamp:    time.Date(2025, 1, 1, 0, 0, i, 0, time.UTC),
		}
		e.HashValue = custody.ComputeLogHash(prevHash, e)
		prevHash = e.HashValue
		events[i] = e
	}
	return events
}

// --- Tests for GenerateCustodyPDF ---

func TestGenerateCustodyPDF_ValidPDF(t *testing.T) {
	caseRec := sampleCaseRecord()
	item := sampleEvidenceItem()
	events := buildValidChain(3)

	gen := NewCustodyReportGenerator(
		&mockCustodySource{allByCaseEvents: events},
		&mockEvidenceSource{findByCaseItems: []evidence.EvidenceItem{item}, findByCaseTotal: 1},
		&mockCaseSource{caseRecord: caseRec},
	)

	pdfBytes, err := gen.GenerateCustodyPDF(context.Background(), caseRec.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !bytes.HasPrefix(pdfBytes, []byte("%PDF")) {
		t.Error("output does not start with %PDF header")
	}

	if len(pdfBytes) < 500 {
		t.Errorf("PDF seems too small (%d bytes), expected substantial content", len(pdfBytes))
	}
}

func TestGenerateCustodyPDF_EmptyEvidence(t *testing.T) {
	caseRec := sampleCaseRecord()
	events := buildValidChain(2)

	gen := NewCustodyReportGenerator(
		&mockCustodySource{allByCaseEvents: events},
		&mockEvidenceSource{findByCaseItems: nil, findByCaseTotal: 0},
		&mockCaseSource{caseRecord: caseRec},
	)

	pdfBytes, err := gen.GenerateCustodyPDF(context.Background(), caseRec.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF")) {
		t.Error("output does not start with %PDF header")
	}
}

func TestGenerateCustodyPDF_EmptyCustodyEvents(t *testing.T) {
	caseRec := sampleCaseRecord()

	gen := NewCustodyReportGenerator(
		&mockCustodySource{allByCaseEvents: nil},
		&mockEvidenceSource{findByCaseItems: nil, findByCaseTotal: 0},
		&mockCaseSource{caseRecord: caseRec},
	)

	pdfBytes, err := gen.GenerateCustodyPDF(context.Background(), caseRec.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF")) {
		t.Error("output does not start with %PDF header")
	}
}

func TestGenerateCustodyPDF_ValidHashChain(t *testing.T) {
	caseRec := sampleCaseRecord()
	events := buildValidChain(5)

	gen := NewCustodyReportGenerator(
		&mockCustodySource{allByCaseEvents: events},
		&mockEvidenceSource{findByCaseItems: nil, findByCaseTotal: 0},
		&mockCaseSource{caseRecord: caseRec},
	)

	pdfBytes, err := gen.GenerateCustodyPDF(context.Background(), caseRec.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF")) {
		t.Error("output does not start with %PDF header")
	}

	// With a valid chain of 5 events, the PDF should be generated without error.
	// We can't easily search for "VERIFIED" in binary PDF content.
	if len(pdfBytes) < 500 {
		t.Errorf("PDF seems too small (%d bytes)", len(pdfBytes))
	}
}

func TestGenerateCustodyPDF_BrokenHashChain(t *testing.T) {
	caseRec := sampleCaseRecord()
	events := buildValidChain(4)
	// Break the chain by mutating a hash
	events[2].HashValue = "tampered_hash_value"

	gen := NewCustodyReportGenerator(
		&mockCustodySource{allByCaseEvents: events},
		&mockEvidenceSource{findByCaseItems: nil, findByCaseTotal: 0},
		&mockCaseSource{caseRecord: caseRec},
	)

	pdfBytes, err := gen.GenerateCustodyPDF(context.Background(), caseRec.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF")) {
		t.Error("output does not start with %PDF header")
	}
	// Broken chain should still produce a valid PDF (with BROKEN status text).
	if len(pdfBytes) < 500 {
		t.Errorf("PDF seems too small (%d bytes)", len(pdfBytes))
	}
}

func TestGenerateCustodyPDF_CaseRepoError(t *testing.T) {
	gen := NewCustodyReportGenerator(
		&mockCustodySource{},
		&mockEvidenceSource{},
		&mockCaseSource{err: fmt.Errorf("db down")},
	)

	_, err := gen.GenerateCustodyPDF(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error when case repo fails")
	}
}

func TestGenerateCustodyPDF_EvidenceRepoError(t *testing.T) {
	gen := NewCustodyReportGenerator(
		&mockCustodySource{},
		&mockEvidenceSource{findByCaseErr: fmt.Errorf("evidence db error")},
		&mockCaseSource{caseRecord: sampleCaseRecord()},
	)

	_, err := gen.GenerateCustodyPDF(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error when evidence repo fails")
	}
}

func TestGenerateCustodyPDF_CustodyRepoError(t *testing.T) {
	gen := NewCustodyReportGenerator(
		&mockCustodySource{allByCaseErr: fmt.Errorf("custody db error")},
		&mockEvidenceSource{findByCaseItems: nil, findByCaseTotal: 0},
		&mockCaseSource{caseRecord: sampleCaseRecord()},
	)

	_, err := gen.GenerateCustodyPDF(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error when custody repo fails")
	}
}

// --- Tests for GenerateEvidenceCustodyPDF ---

func TestGenerateEvidenceCustodyPDF_ValidPDF(t *testing.T) {
	item := sampleEvidenceItem()
	caseRec := sampleCaseRecord()
	events := buildValidChain(2)

	gen := NewCustodyReportGenerator(
		&mockCustodySource{byEvidenceEvents: events, byEvidenceTotal: 2},
		&mockEvidenceSource{findByIDItem: item},
		&mockCaseSource{caseRecord: caseRec},
	)

	pdfBytes, err := gen.GenerateEvidenceCustodyPDF(context.Background(), item.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF")) {
		t.Error("output does not start with %PDF header")
	}
}

func TestGenerateEvidenceCustodyPDF_NilOptionalFields(t *testing.T) {
	item := sampleEvidenceItem()
	item.EvidenceNumber = nil
	item.TSATimestamp = nil
	caseRec := sampleCaseRecord()

	gen := NewCustodyReportGenerator(
		&mockCustodySource{byEvidenceEvents: nil, byEvidenceTotal: 0},
		&mockEvidenceSource{findByIDItem: item},
		&mockCaseSource{caseRecord: caseRec},
	)

	pdfBytes, err := gen.GenerateEvidenceCustodyPDF(context.Background(), item.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF")) {
		t.Error("output does not start with %PDF header")
	}
}

func TestGenerateEvidenceCustodyPDF_EvidenceRepoError(t *testing.T) {
	gen := NewCustodyReportGenerator(
		&mockCustodySource{},
		&mockEvidenceSource{findByIDErr: fmt.Errorf("not found")},
		&mockCaseSource{caseRecord: sampleCaseRecord()},
	)

	_, err := gen.GenerateEvidenceCustodyPDF(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error when evidence item not found")
	}
}

func TestGenerateEvidenceCustodyPDF_CaseRepoError(t *testing.T) {
	item := sampleEvidenceItem()
	gen := NewCustodyReportGenerator(
		&mockCustodySource{},
		&mockEvidenceSource{findByIDItem: item},
		&mockCaseSource{err: fmt.Errorf("case not found")},
	)

	_, err := gen.GenerateEvidenceCustodyPDF(context.Background(), item.ID)
	if err == nil {
		t.Fatal("expected error when case repo fails")
	}
}

func TestGenerateEvidenceCustodyPDF_CustodyRepoError(t *testing.T) {
	item := sampleEvidenceItem()
	gen := NewCustodyReportGenerator(
		&mockCustodySource{byEvidenceErr: fmt.Errorf("custody error")},
		&mockEvidenceSource{findByIDItem: item},
		&mockCaseSource{caseRecord: sampleCaseRecord()},
	)

	_, err := gen.GenerateEvidenceCustodyPDF(context.Background(), item.ID)
	if err == nil {
		t.Fatal("expected error when custody repo fails")
	}
}

// --- Tests for GetReferenceCode ---

func TestGetReferenceCode_Success(t *testing.T) {
	caseRec := sampleCaseRecord()
	gen := NewCustodyReportGenerator(
		&mockCustodySource{},
		&mockEvidenceSource{},
		&mockCaseSource{caseRecord: caseRec},
	)

	code, err := gen.GetReferenceCode(context.Background(), caseRec.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != caseRec.ReferenceCode {
		t.Errorf("got %q, want %q", code, caseRec.ReferenceCode)
	}
}

func TestGetReferenceCode_Error(t *testing.T) {
	gen := NewCustodyReportGenerator(
		&mockCustodySource{},
		&mockEvidenceSource{},
		&mockCaseSource{err: fmt.Errorf("db error")},
	)

	_, err := gen.GetReferenceCode(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- Tests for truncate ---

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is too long", 10, "this is..."},
		{"abc", 3, "abc"},
	}
	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}

// --- Tests for PDF output error ---

func TestGenerateCustodyPDF_OutputError(t *testing.T) {
	original := pdfOutput
	t.Cleanup(func() { pdfOutput = original })

	injected := fmt.Errorf("pdf output error")
	pdfOutput = func(_ *fpdf.Fpdf, _ *bytes.Buffer) error { return injected }

	caseRec := sampleCaseRecord()
	gen := NewCustodyReportGenerator(
		&mockCustodySource{allByCaseEvents: nil},
		&mockEvidenceSource{findByCaseItems: nil, findByCaseTotal: 0},
		&mockCaseSource{caseRecord: caseRec},
	)

	_, err := gen.GenerateCustodyPDF(context.Background(), caseRec.ID)
	if err == nil {
		t.Fatal("expected error when pdf output fails")
	}
}

func TestGenerateEvidenceCustodyPDF_OutputError(t *testing.T) {
	original := pdfOutput
	t.Cleanup(func() { pdfOutput = original })

	injected := fmt.Errorf("pdf output error")
	pdfOutput = func(_ *fpdf.Fpdf, _ *bytes.Buffer) error { return injected }

	item := sampleEvidenceItem()
	caseRec := sampleCaseRecord()
	gen := NewCustodyReportGenerator(
		&mockCustodySource{byEvidenceEvents: nil, byEvidenceTotal: 0},
		&mockEvidenceSource{findByIDItem: item},
		&mockCaseSource{caseRecord: caseRec},
	)

	_, err := gen.GenerateEvidenceCustodyPDF(context.Background(), item.ID)
	if err == nil {
		t.Fatal("expected error when pdf output fails")
	}
}

// --- Tests for evidence summary with alternating fill ---

func TestGenerateCustodyPDF_MultipleEvidenceItems(t *testing.T) {
	caseRec := sampleCaseRecord()
	item1 := sampleEvidenceItem()
	item2 := sampleEvidenceItem()
	num2 := "EV-002"
	item2.EvidenceNumber = &num2
	item2.Filename = "photo.jpg"
	item2.SHA256Hash = "abcdef1234567890abcdef1234567890"

	gen := NewCustodyReportGenerator(
		&mockCustodySource{allByCaseEvents: nil},
		&mockEvidenceSource{findByCaseItems: []evidence.EvidenceItem{item1, item2}, findByCaseTotal: 2},
		&mockCaseSource{caseRecord: caseRec},
	)

	pdfBytes, err := gen.GenerateCustodyPDF(context.Background(), caseRec.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF")) {
		t.Error("output does not start with %PDF header")
	}
}

// --- Test custody table with long strings (covers truncation in table) ---

func TestGenerateCustodyPDF_LongActionAndDetail(t *testing.T) {
	caseRec := sampleCaseRecord()
	events := []custody.Event{
		{
			ID:          uuid.New(),
			CaseID:      caseRec.ID,
			EvidenceID:  uuid.New(),
			Action:      "this_action_is_very_long_and_should_be_truncated",
			ActorUserID: "user_with_a_really_long_id_value",
			Detail:      "A very long detail string that exceeds the maximum column width for the custody table",
			HashValue:   "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			Timestamp:   time.Now(),
		},
	}

	gen := NewCustodyReportGenerator(
		&mockCustodySource{allByCaseEvents: events},
		&mockEvidenceSource{findByCaseItems: nil, findByCaseTotal: 0},
		&mockCaseSource{caseRecord: caseRec},
	)

	pdfBytes, err := gen.GenerateCustodyPDF(context.Background(), caseRec.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF")) {
		t.Error("output does not start with %PDF header")
	}
}

// --- Test hash chain with single event (no comparison needed) ---

func TestGenerateCustodyPDF_SingleEvent(t *testing.T) {
	caseRec := sampleCaseRecord()
	events := buildValidChain(1)

	gen := NewCustodyReportGenerator(
		&mockCustodySource{allByCaseEvents: events},
		&mockEvidenceSource{findByCaseItems: nil, findByCaseTotal: 0},
		&mockCaseSource{caseRecord: caseRec},
	)

	pdfBytes, err := gen.GenerateCustodyPDF(context.Background(), caseRec.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Single event should verify as intact and produce valid PDF
	if len(pdfBytes) < 500 {
		t.Errorf("PDF seems too small (%d bytes)", len(pdfBytes))
	}
}

// --- Test evidence item with short hash (no truncation) ---

func TestGenerateCustodyPDF_ShortHash(t *testing.T) {
	caseRec := sampleCaseRecord()
	item := sampleEvidenceItem()
	item.SHA256Hash = "abc123" // shorter than 16 chars

	gen := NewCustodyReportGenerator(
		&mockCustodySource{allByCaseEvents: nil},
		&mockEvidenceSource{findByCaseItems: []evidence.EvidenceItem{item}, findByCaseTotal: 1},
		&mockCaseSource{caseRecord: caseRec},
	)

	pdfBytes, err := gen.GenerateCustodyPDF(context.Background(), caseRec.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF")) {
		t.Error("output does not start with %PDF header")
	}
}

// --- Test event with short hash (no truncation in custody table) ---

func TestGenerateCustodyPDF_EventShortHash(t *testing.T) {
	caseRec := sampleCaseRecord()
	events := []custody.Event{
		{
			ID:          uuid.New(),
			CaseID:      caseRec.ID,
			EvidenceID:  uuid.New(),
			Action:      "upload",
			ActorUserID: "user-1",
			Detail:      "test",
			HashValue:   "short",
			Timestamp:   time.Now(),
		},
	}

	gen := NewCustodyReportGenerator(
		&mockCustodySource{allByCaseEvents: events},
		&mockEvidenceSource{findByCaseItems: nil, findByCaseTotal: 0},
		&mockCaseSource{caseRecord: caseRec},
	)

	pdfBytes, err := gen.GenerateCustodyPDF(context.Background(), caseRec.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF")) {
		t.Error("output does not start with %PDF header")
	}
}
