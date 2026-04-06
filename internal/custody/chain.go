package custody

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type ChainVerifier interface {
	VerifyCaseChain(ctx context.Context, caseID uuid.UUID) (ChainVerification, error)
}

type PGChainVerifier struct {
	repo *PGRepository
}

func NewChainVerifier(repo *PGRepository) *PGChainVerifier {
	return &PGChainVerifier{repo: repo}
}

func (v *PGChainVerifier) VerifyCaseChain(ctx context.Context, caseID uuid.UUID) (ChainVerification, error) {
	events, err := v.repo.ListAllByCase(ctx, caseID)
	if err != nil {
		return ChainVerification{}, fmt.Errorf("list events for verification: %w", err)
	}

	result := ChainVerification{
		Valid:        true,
		TotalEntries: len(events),
		VerifiedAt:   time.Now().UTC(),
	}

	if len(events) == 0 {
		return result, nil
	}

	previousHash := ""
	for i, e := range events {
		expected := ComputeLogHash(previousHash, e)
		if e.HashValue != expected {
			result.Valid = false
			result.Breaks = append(result.Breaks, ChainBreak{
				EntryID:      e.ID,
				Position:     i,
				ExpectedHash: expected,
				ActualHash:   e.HashValue,
				Timestamp:    e.Timestamp,
			})
		}
		if e.PreviousHash != previousHash {
			result.Valid = false
			result.Breaks = append(result.Breaks, ChainBreak{
				EntryID:      e.ID,
				Position:     i,
				ExpectedHash: previousHash,
				ActualHash:   e.PreviousHash,
				Timestamp:    e.Timestamp,
			})
		}
		previousHash = e.HashValue
	}

	return result, nil
}

func ComputeLogHash(prev string, entry Event) string {
	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s",
		prev,
		entry.ID.String(),
		entry.ActorUserID,
		entry.Action,
		entry.Timestamp.UTC().Format(time.RFC3339Nano),
		entry.EvidenceID.String(),
		entry.CaseID.String(),
		canonicalJSON(entry.Detail),
	)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func canonicalJSON(v string) string {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(v), &parsed); err != nil {
		return v
	}
	return sortedJSON(parsed)
}

func sortedJSON(m map[string]any) string {
	// json.Marshal iterates map keys in sorted order (Go stdlib guarantee since 1.12).
	b, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(b)
}
