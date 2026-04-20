package federation

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	// DerivationDomain is the domain separator for derivation commitment hashes.
	DerivationDomain = "VK:DERIVATION:v1"
)

// DerivationRecord links a redacted evidence item to its original,
// with a cryptographic commitment proving the derivation relationship.
type DerivationRecord struct {
	Type                 string    `json:"type"` // "redaction"
	ParentEvidenceID     uuid.UUID `json:"parent_evidence_id"`
	ChildEvidenceID      uuid.UUID `json:"child_evidence_id"`
	ChildSHA256          string    `json:"child_sha256"`
	ParentHashCommitment *string   `json:"parent_hash_commitment,omitempty"`
	RedactionMethod      string    `json:"redaction_method"`
	RedactionPurpose     string    `json:"redaction_purpose"`
	ParametersCommitment string    `json:"parameters_commitment"`
	CreatedAt            time.Time `json:"created_at"`
	SignedByInstance      string   `json:"signed_by_instance"`
}

// BuildDerivationCommitment computes the derivation commitment hash:
// SHA-256("VK:DERIVATION:v1" || canonical_json(record)).
func BuildDerivationCommitment(record DerivationRecord) (string, error) {
	return HexHash(DerivationDomain, record)
}

// BuildParametersCommitment computes SHA-256 of the redaction
// parameters (areas, method, etc.) for inclusion in the derivation
// record without revealing the exact parameters.
func BuildParametersCommitment(params any) (string, error) {
	return HexHash("VK:PARAMS:v1", params)
}

// DerivationInput holds the parameters for creating a derivation record.
type DerivationInput struct {
	ParentEvidenceID uuid.UUID
	ChildEvidenceID  uuid.UUID
	ChildSHA256      string
	ParentSHA256     *string // optional: SHA-256 of original
	RedactionMethod  string  // "pdf-redact-v1", "image-blur-v1"
	RedactionPurpose string
	Parameters       any // redaction parameters for commitment
	InstanceID       string
}

// BuildDerivationRecord creates a complete derivation record with
// computed commitments.
func BuildDerivationRecord(input DerivationInput) (DerivationRecord, string, error) {
	paramsCommitment, err := BuildParametersCommitment(input.Parameters)
	if err != nil {
		return DerivationRecord{}, "", fmt.Errorf("build parameters commitment: %w", err)
	}

	var parentHash *string
	if input.ParentSHA256 != nil {
		parentHash = input.ParentSHA256
	}

	record := DerivationRecord{
		Type:                 "redaction",
		ParentEvidenceID:     input.ParentEvidenceID,
		ChildEvidenceID:      input.ChildEvidenceID,
		ChildSHA256:          input.ChildSHA256,
		ParentHashCommitment: parentHash,
		RedactionMethod:      input.RedactionMethod,
		RedactionPurpose:     input.RedactionPurpose,
		ParametersCommitment: paramsCommitment,
		CreatedAt:            time.Now().UTC(),
		SignedByInstance:      input.InstanceID,
	}

	commitment, err := BuildDerivationCommitment(record)
	if err != nil {
		return DerivationRecord{}, "", fmt.Errorf("build derivation commitment: %w", err)
	}

	return record, commitment, nil
}
