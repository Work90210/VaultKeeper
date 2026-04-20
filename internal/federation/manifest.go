package federation

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	// ManifestDomain is the domain separator for manifest hashes.
	ManifestDomain = "VK:MANIFEST:v1"
	// ProtocolVersionVKE1 is the current protocol version string.
	ProtocolVersionVKE1 = "VKE1"
)

// Dependency policy constants.
const (
	DependencyPolicyNone         = "none"
	DependencyPolicyDirectParent = "direct_parent"
	DependencyPolicyFullAncestry = "full_ancestry"
)

// ExchangeManifest is the signed object that attests to the contents
// and integrity of a VKE1 evidence exchange.
type ExchangeManifest struct {
	ProtocolVersion       string               `json:"protocol_version"`
	ExchangeID            uuid.UUID            `json:"exchange_id"`
	SenderInstanceID      string               `json:"sender_instance_id"`
	SenderKeyFingerprint  string               `json:"sender_key_fingerprint"`
	RecipientInstanceID   *string              `json:"recipient_instance_id,omitempty"`
	CreatedAt             time.Time            `json:"created_at"`
	Scope                 ScopeDescriptor      `json:"scope"`
	ScopeHash             string               `json:"scope_hash"`
	ScopeCardinality      int                  `json:"scope_cardinality"`
	MerkleRoot            string               `json:"merkle_root"`
	DependencyPolicy      string               `json:"dependency_policy"`
	DisclosedEvidence     []EvidenceDescriptor `json:"disclosed_evidence"`
	SenderCustodyHead     string               `json:"sender_custody_head"`
	SenderBridgeEventHash string               `json:"sender_bridge_event_hash"`
	ManifestHash          string               `json:"manifest_hash"`
}

// manifestForHashing is the manifest with ManifestHash zeroed, used
// as input to the manifest hash computation. This ensures the hash
// covers all fields except itself.
type manifestForHashing struct {
	ProtocolVersion       string               `json:"protocol_version"`
	ExchangeID            uuid.UUID            `json:"exchange_id"`
	SenderInstanceID      string               `json:"sender_instance_id"`
	SenderKeyFingerprint  string               `json:"sender_key_fingerprint"`
	RecipientInstanceID   *string              `json:"recipient_instance_id,omitempty"`
	CreatedAt             time.Time            `json:"created_at"`
	Scope                 ScopeDescriptor      `json:"scope"`
	ScopeHash             string               `json:"scope_hash"`
	ScopeCardinality      int                  `json:"scope_cardinality"`
	MerkleRoot            string               `json:"merkle_root"`
	DependencyPolicy      string               `json:"dependency_policy"`
	DisclosedEvidence     []EvidenceDescriptor `json:"disclosed_evidence"`
	SenderCustodyHead     string               `json:"sender_custody_head"`
	SenderBridgeEventHash string               `json:"sender_bridge_event_hash"`
}

// ComputeManifestHash computes SHA-256("VK:MANIFEST:v1" || canonical_json(manifest_without_hash)).
func ComputeManifestHash(m ExchangeManifest) (string, error) {
	forHash := manifestForHashing{
		ProtocolVersion:       m.ProtocolVersion,
		ExchangeID:            m.ExchangeID,
		SenderInstanceID:      m.SenderInstanceID,
		SenderKeyFingerprint:  m.SenderKeyFingerprint,
		RecipientInstanceID:   m.RecipientInstanceID,
		CreatedAt:             m.CreatedAt,
		Scope:                 m.Scope,
		ScopeHash:             m.ScopeHash,
		ScopeCardinality:      m.ScopeCardinality,
		MerkleRoot:            m.MerkleRoot,
		DependencyPolicy:      m.DependencyPolicy,
		DisclosedEvidence:     m.DisclosedEvidence,
		SenderCustodyHead:     m.SenderCustodyHead,
		SenderBridgeEventHash: m.SenderBridgeEventHash,
	}

	return HexHash(ManifestDomain, forHash)
}

// BuildExchangeManifest constructs a complete manifest from its
// constituent parts. The manifest hash is computed and set before
// returning. The caller must sign the ManifestHash separately.
func BuildExchangeManifest(
	exchangeID uuid.UUID,
	senderInstanceID string,
	senderKeyFingerprint string,
	recipientInstanceID *string,
	scope ScopeDescriptor,
	descriptors []EvidenceDescriptor,
	merkleRoot []byte,
	dependencyPolicy string,
	senderCustodyHead string,
	senderBridgeEventHash string,
) (ExchangeManifest, error) {
	scopeHash, err := ScopeHash(scope)
	if err != nil {
		return ExchangeManifest{}, fmt.Errorf("compute scope hash: %w", err)
	}

	m := ExchangeManifest{
		ProtocolVersion:       ProtocolVersionVKE1,
		ExchangeID:            exchangeID,
		SenderInstanceID:      senderInstanceID,
		SenderKeyFingerprint:  senderKeyFingerprint,
		RecipientInstanceID:   recipientInstanceID,
		CreatedAt:             time.Now().UTC(),
		Scope:                 CanonicalizeScope(scope),
		ScopeHash:             scopeHash,
		ScopeCardinality:      len(descriptors),
		MerkleRoot:            hex.EncodeToString(merkleRoot),
		DependencyPolicy:      dependencyPolicy,
		DisclosedEvidence:     SortDescriptors(descriptors),
		SenderCustodyHead:     senderCustodyHead,
		SenderBridgeEventHash: senderBridgeEventHash,
	}

	hash, err := ComputeManifestHash(m)
	if err != nil {
		return ExchangeManifest{}, fmt.Errorf("compute manifest hash: %w", err)
	}
	m.ManifestHash = hash

	return m, nil
}
