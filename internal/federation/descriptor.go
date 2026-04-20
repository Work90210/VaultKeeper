package federation

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/evidence"
)

// EvidenceDescriptor is the leaf node in the scoped Merkle tree.
// One descriptor per evidence item in the evaluated scope.
type EvidenceDescriptor struct {
	EvidenceID           uuid.UUID  `json:"evidence_id"`
	CaseID               uuid.UUID  `json:"case_id"`
	Version              int        `json:"version"`
	SHA256               string     `json:"sha256"`
	Classification       string     `json:"classification"`
	Tags                 []string   `json:"tags"`
	SourceDate           *time.Time `json:"source_date,omitempty"`
	ParentID             *uuid.UUID `json:"parent_id,omitempty"`
	DerivationCommitment *string    `json:"derivation_commitment,omitempty"`
	TSATokenHash         *string    `json:"tsa_token_hash,omitempty"`
}

// BuildDescriptor creates an EvidenceDescriptor from an evidence item.
// Tags are lowercased and sorted. TSATokenHash is SHA-256 of the TSA
// token bytes (hex-encoded), computed only if the item has a token.
func BuildDescriptor(item evidence.EvidenceItem) EvidenceDescriptor {
	tags := normalizeTags(item.Tags)

	var tsaTokenHash *string
	if len(item.TSAToken) > 0 {
		h := sha256.Sum256(item.TSAToken)
		s := hex.EncodeToString(h[:])
		tsaTokenHash = &s
	}

	return EvidenceDescriptor{
		EvidenceID:     item.ID,
		CaseID:         item.CaseID,
		Version:        item.Version,
		SHA256:         item.SHA256Hash,
		Classification: item.Classification,
		Tags:           tags,
		SourceDate:     normalizeTime(item.SourceDate),
		ParentID:       item.ParentID,
		TSATokenHash:   tsaTokenHash,
	}
}

// LeafHash computes SHA-256("VK:MERKLE:LEAF:v1" || canonical_json(descriptor)).
func LeafHash(d EvidenceDescriptor) ([]byte, error) {
	return CanonicalHash(LeafDomain, d)
}

// SortKey returns the deterministic sort key for ordering descriptors
// within a Merkle tree: "evidence_id:version" (lexicographic).
func (d EvidenceDescriptor) SortKey() string {
	return fmt.Sprintf("%s:%d", d.EvidenceID.String(), d.Version)
}

// SortDescriptors sorts a slice of descriptors by their sort key.
// Returns a new slice; the input is not mutated.
func SortDescriptors(descriptors []EvidenceDescriptor) []EvidenceDescriptor {
	sorted := make([]EvidenceDescriptor, len(descriptors))
	copy(sorted, descriptors)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].SortKey() < sorted[j].SortKey()
	})
	return sorted
}

// BuildScopedMerkleTree sorts descriptors deterministically and builds
// a Merkle tree from their leaf hashes.
func BuildScopedMerkleTree(descriptors []EvidenceDescriptor) (*MerkleTree, error) {
	if len(descriptors) == 0 {
		return nil, fmt.Errorf("cannot build scoped merkle tree from zero descriptors")
	}

	sorted := SortDescriptors(descriptors)

	leaves := make([][]byte, len(sorted))
	for i, d := range sorted {
		leaf, err := LeafHash(d)
		if err != nil {
			return nil, fmt.Errorf("leaf hash for %s: %w", d.SortKey(), err)
		}
		leaves[i] = leaf
	}

	return BuildMerkleTree(leaves)
}

// normalizeTags lowercases and sorts tags, returning a new slice.
func normalizeTags(tags []string) []string {
	if len(tags) == 0 {
		return []string{}
	}
	result := make([]string, len(tags))
	for i, t := range tags {
		result[i] = strings.ToLower(t)
	}
	sort.Strings(result)
	return result
}

// normalizeTime converts a time pointer to UTC if non-nil.
func normalizeTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	utc := t.UTC()
	return &utc
}
