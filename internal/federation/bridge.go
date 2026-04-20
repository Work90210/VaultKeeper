package federation

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// Custody chain action types for federation bridge events.
const (
	ActionDisclosedToInstance   = "disclosed_to_instance"
	ActionImportedFromInstance = "imported_from_instance"
)

// CustodyRecorder is the subset of the custody logger needed by the
// federation package. Defined here (accept interfaces, return structs).
type CustodyRecorder interface {
	RecordCaseEvent(ctx context.Context, caseID uuid.UUID, action string, actorUserID string, detail map[string]string) error
}

// DisclosureBridgeInput holds the parameters for creating a
// DISCLOSED_TO_INSTANCE custody event on the sender's chain.
type DisclosureBridgeInput struct {
	CaseID                uuid.UUID
	ExchangeID            uuid.UUID
	ManifestHash          string
	RecipientInstanceID   string
	RecipientFingerprint  string
	DisclosedEvidenceIDs  []uuid.UUID
	ScopeHash             string
	MerkleRoot            string
	ScopeCardinality      int
	ActorUserID           string
}

// ImportBridgeInput holds the parameters for creating an
// IMPORTED_FROM_INSTANCE custody event on the receiver's chain.
type ImportBridgeInput struct {
	CaseID                uuid.UUID
	ExchangeID            uuid.UUID
	ManifestHash          string
	SenderInstanceID      string
	SenderFingerprint     string
	SenderCustodyHead     string
	SenderBridgeEventHash string
	ImportedEvidenceIDs   []uuid.UUID
	ActorUserID           string
}

// RecordDisclosureBridgeEvent creates a DISCLOSED_TO_INSTANCE custody
// event on the sender's chain. This event captures the cryptographic
// binding between the sender's custody chain and the exchange.
func RecordDisclosureBridgeEvent(ctx context.Context, recorder CustodyRecorder, input DisclosureBridgeInput) error {
	evidenceIDs, err := json.Marshal(input.DisclosedEvidenceIDs)
	if err != nil {
		return fmt.Errorf("marshal disclosed evidence IDs: %w", err)
	}

	detail := map[string]string{
		"exchange_id":                input.ExchangeID.String(),
		"manifest_hash":             input.ManifestHash,
		"recipient_instance_id":     input.RecipientInstanceID,
		"recipient_pubkey_fingerprint": input.RecipientFingerprint,
		"disclosed_evidence_ids":    string(evidenceIDs),
		"scope_hash":                input.ScopeHash,
		"merkle_root":               input.MerkleRoot,
		"scope_cardinality":         fmt.Sprintf("%d", input.ScopeCardinality),
	}

	return recorder.RecordCaseEvent(ctx, input.CaseID, ActionDisclosedToInstance, input.ActorUserID, detail)
}

// RecordImportBridgeEvent creates an IMPORTED_FROM_INSTANCE custody
// event on the receiver's chain. The sender_bridge_event_hash creates
// the cryptographic graph edge between the two chains.
func RecordImportBridgeEvent(ctx context.Context, recorder CustodyRecorder, input ImportBridgeInput) error {
	evidenceIDs, err := json.Marshal(input.ImportedEvidenceIDs)
	if err != nil {
		return fmt.Errorf("marshal imported evidence IDs: %w", err)
	}

	detail := map[string]string{
		"exchange_id":              input.ExchangeID.String(),
		"manifest_hash":           input.ManifestHash,
		"sender_instance_id":      input.SenderInstanceID,
		"sender_pubkey_fingerprint": input.SenderFingerprint,
		"sender_custody_head":     input.SenderCustodyHead,
		"sender_bridge_event_hash": input.SenderBridgeEventHash,
		"imported_evidence_ids":   string(evidenceIDs),
	}

	return recorder.RecordCaseEvent(ctx, input.CaseID, ActionImportedFromInstance, input.ActorUserID, detail)
}
