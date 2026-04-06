package custody

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestComputeLogHash_Deterministic(t *testing.T) {
	e := Event{
		ID:          uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
		CaseID:      uuid.MustParse("660e8400-e29b-41d4-a716-446655440000"),
		EvidenceID:  uuid.Nil,
		Action:      "case_created",
		ActorUserID: "user-1",
		Detail:      `{"title":"Test"}`,
		Timestamp:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	h1 := ComputeLogHash("", e)
	h2 := ComputeLogHash("", e)

	if h1 != h2 {
		t.Error("hash should be deterministic")
	}
	if len(h1) != 64 {
		t.Errorf("hash length = %d, want 64", len(h1))
	}
}

func TestComputeLogHash_DifferentInputs(t *testing.T) {
	base := Event{
		ID:          uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
		CaseID:      uuid.MustParse("660e8400-e29b-41d4-a716-446655440000"),
		Action:      "case_created",
		ActorUserID: "user-1",
		Detail:      "{}",
		Timestamp:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	h1 := ComputeLogHash("", base)

	modified := base
	modified.Action = "case_updated"
	h2 := ComputeLogHash("", modified)

	if h1 == h2 {
		t.Error("different action should produce different hash")
	}

	h3 := ComputeLogHash("prev-hash", base)
	if h1 == h3 {
		t.Error("different previous hash should produce different hash")
	}
}

func TestComputeLogHash_ChainIntegrity(t *testing.T) {
	e1 := Event{
		ID: uuid.New(), CaseID: uuid.New(), Action: "created",
		ActorUserID: "user-1", Detail: "{}", Timestamp: time.Now().UTC(),
	}
	hash1 := ComputeLogHash("", e1)

	e2 := Event{
		ID: uuid.New(), CaseID: e1.CaseID, Action: "updated",
		ActorUserID: "user-1", Detail: "{}", Timestamp: time.Now().UTC(),
	}
	hash2 := ComputeLogHash(hash1, e2)

	if hash1 == hash2 {
		t.Error("chained hashes should differ")
	}
}

func TestCanonicalJSON(t *testing.T) {
	// Valid JSON
	result := canonicalJSON(`{"b":"2","a":"1"}`)
	if result == "" {
		t.Error("expected non-empty result for valid JSON")
	}

	// Invalid JSON returns input as-is
	result = canonicalJSON("not json")
	if result != "not json" {
		t.Errorf("expected input back for invalid JSON, got %q", result)
	}
}
