package federation

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/evidence"
)

func TestBuildDescriptor_TagsNormalized(t *testing.T) {
	item := evidence.EvidenceItem{
		ID:             uuid.New(),
		CaseID:         uuid.New(),
		Version:        1,
		SHA256Hash:     "abc123",
		Classification: "public",
		Tags:           []string{"Bravo", "alpha", "CHARLIE"},
	}

	d := BuildDescriptor(item)

	if len(d.Tags) != 3 {
		t.Fatalf("expected 3 tags, got %d", len(d.Tags))
	}
	// Tags should be lowercased and sorted.
	expected := []string{"alpha", "bravo", "charlie"}
	for i, tag := range d.Tags {
		if tag != expected[i] {
			t.Errorf("tag[%d] = %q, want %q", i, tag, expected[i])
		}
	}
}

func TestBuildDescriptor_EmptyTags(t *testing.T) {
	item := evidence.EvidenceItem{
		ID:             uuid.New(),
		CaseID:         uuid.New(),
		Version:        1,
		SHA256Hash:     "abc",
		Classification: "public",
	}

	d := BuildDescriptor(item)
	if d.Tags == nil {
		t.Error("empty tags should be non-nil empty slice")
	}
	if len(d.Tags) != 0 {
		t.Errorf("expected 0 tags, got %d", len(d.Tags))
	}
}

func TestBuildDescriptor_TSATokenHash(t *testing.T) {
	item := evidence.EvidenceItem{
		ID:             uuid.New(),
		CaseID:         uuid.New(),
		Version:        1,
		SHA256Hash:     "abc",
		Classification: "public",
		TSAToken:       []byte("some-tsa-token-bytes"),
	}

	d := BuildDescriptor(item)
	if d.TSATokenHash == nil {
		t.Fatal("expected TSATokenHash to be set")
	}
	if len(*d.TSATokenHash) != 64 { // hex SHA-256
		t.Errorf("TSATokenHash length = %d, want 64", len(*d.TSATokenHash))
	}
}

func TestBuildDescriptor_NoTSAToken(t *testing.T) {
	item := evidence.EvidenceItem{
		ID:             uuid.New(),
		CaseID:         uuid.New(),
		Version:        1,
		SHA256Hash:     "abc",
		Classification: "public",
	}

	d := BuildDescriptor(item)
	if d.TSATokenHash != nil {
		t.Error("expected nil TSATokenHash when no TSA token")
	}
}

func TestBuildDescriptor_SourceDateUTC(t *testing.T) {
	loc := time.FixedZone("EST", -5*60*60)
	srcDate := time.Date(2024, 6, 15, 10, 0, 0, 0, loc)

	item := evidence.EvidenceItem{
		ID:             uuid.New(),
		CaseID:         uuid.New(),
		Version:        1,
		SHA256Hash:     "abc",
		Classification: "public",
		SourceDate:     &srcDate,
	}

	d := BuildDescriptor(item)
	if d.SourceDate == nil {
		t.Fatal("expected SourceDate to be set")
	}
	if d.SourceDate.Location() != time.UTC {
		t.Error("SourceDate not normalized to UTC")
	}
}

func TestLeafHash_Deterministic(t *testing.T) {
	d := EvidenceDescriptor{
		EvidenceID:     uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		CaseID:         uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		Version:        1,
		SHA256:         "deadbeef",
		Classification: "restricted",
		Tags:           []string{"alpha", "beta"},
	}

	h1, err := LeafHash(d)
	if err != nil {
		t.Fatal(err)
	}
	h2, _ := LeafHash(d)

	if string(h1) != string(h2) {
		t.Error("leaf hash not deterministic")
	}
}

func TestSortDescriptors(t *testing.T) {
	d1 := EvidenceDescriptor{
		EvidenceID: uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
		Version:    1,
	}
	d2 := EvidenceDescriptor{
		EvidenceID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		Version:    1,
	}
	d3 := EvidenceDescriptor{
		EvidenceID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		Version:    2,
	}

	sorted := SortDescriptors([]EvidenceDescriptor{d1, d3, d2})

	if sorted[0].EvidenceID != d2.EvidenceID || sorted[0].Version != 1 {
		t.Errorf("first = %s v%d, want aaaa v1", sorted[0].EvidenceID, sorted[0].Version)
	}
	if sorted[1].EvidenceID != d3.EvidenceID || sorted[1].Version != 2 {
		t.Errorf("second = %s v%d, want aaaa v2", sorted[1].EvidenceID, sorted[1].Version)
	}
	if sorted[2].EvidenceID != d1.EvidenceID {
		t.Errorf("third = %s, want bbbb", sorted[2].EvidenceID)
	}
}

func TestBuildScopedMerkleTree_EmptyDescriptors(t *testing.T) {
	_, err := BuildScopedMerkleTree(nil)
	if err == nil {
		t.Error("expected error for empty descriptors")
	}
}

func TestBuildScopedMerkleTree_ProofsVerify(t *testing.T) {
	descriptors := make([]EvidenceDescriptor, 5)
	for i := range descriptors {
		descriptors[i] = EvidenceDescriptor{
			EvidenceID:     uuid.New(),
			CaseID:         uuid.New(),
			Version:        1,
			SHA256:         "abc",
			Classification: "public",
			Tags:           []string{},
		}
	}

	tree, err := BuildScopedMerkleTree(descriptors)
	if err != nil {
		t.Fatal(err)
	}

	// Verify proofs for all leaves.
	sorted := SortDescriptors(descriptors)
	for i, d := range sorted {
		leaf, _ := LeafHash(d)
		proof, err := tree.Proof(i)
		if err != nil {
			t.Fatalf("proof(%d): %v", i, err)
		}
		if !VerifyProof(leaf, proof, tree.Root) {
			t.Errorf("proof for descriptor %d failed", i)
		}
	}
}
