package federation

import (
	"testing"

	"github.com/google/uuid"
)

func TestBuildDerivationRecord(t *testing.T) {
	parentID := uuid.New()
	childID := uuid.New()
	parentHash := "abc123"

	record, commitment, err := BuildDerivationRecord(DerivationInput{
		ParentEvidenceID: parentID,
		ChildEvidenceID:  childID,
		ChildSHA256:      "def456",
		ParentSHA256:     &parentHash,
		RedactionMethod:  "pdf-redact-v1",
		RedactionPurpose: "witness_protection",
		Parameters:       map[string]any{"areas": []map[string]any{{"page": 1, "x": 10, "y": 20}}},
		InstanceID:       "test-instance",
	})
	if err != nil {
		t.Fatal(err)
	}

	if record.Type != "redaction" {
		t.Errorf("type = %q", record.Type)
	}
	if record.ParentEvidenceID != parentID {
		t.Error("parent ID mismatch")
	}
	if record.ParametersCommitment == "" {
		t.Error("parameters commitment empty")
	}
	if commitment == "" {
		t.Error("derivation commitment empty")
	}
	if len(commitment) != 64 {
		t.Errorf("commitment length = %d, want 64", len(commitment))
	}
}

func TestBuildDerivationCommitment_Deterministic(t *testing.T) {
	record, _, err := BuildDerivationRecord(DerivationInput{
		ParentEvidenceID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		ChildEvidenceID:  uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
		ChildSHA256:      "child-hash",
		RedactionMethod:  "image-blur-v1",
		RedactionPurpose: "public_release",
		Parameters:       map[string]string{"method": "gaussian"},
		InstanceID:       "inst",
	})
	if err != nil {
		t.Fatal(err)
	}

	c1, _ := BuildDerivationCommitment(record)
	c2, _ := BuildDerivationCommitment(record)

	if c1 != c2 {
		t.Error("derivation commitment not deterministic")
	}
}

func TestBuildParametersCommitment_DifferentParams(t *testing.T) {
	c1, _ := BuildParametersCommitment(map[string]string{"a": "1"})
	c2, _ := BuildParametersCommitment(map[string]string{"a": "2"})

	if c1 == c2 {
		t.Error("different params should produce different commitments")
	}
}
