package federation

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestValidateScope_ValidScope(t *testing.T) {
	scope := ScopeDescriptor{
		ScopeVersion: 1,
		CaseID:       uuid.New(),
		Predicate: ScopePredicate{
			Op:    "and",
			Children: []ScopePredicate{
				{Op: "contains", Field: "tags", Value: json.RawMessage(`"ukraine-2024"`)},
				{Op: "eq", Field: "classification", Value: json.RawMessage(`"restricted"`)},
			},
		},
		Snapshot: SnapshotAnchor{
			AsOfCustodyHash: "abc123",
			EvaluatedAt:     time.Now().UTC(),
		},
	}
	if err := ValidateScope(scope); err != nil {
		t.Fatalf("valid scope rejected: %v", err)
	}
}

func TestValidateScope_UnsupportedVersion(t *testing.T) {
	scope := ScopeDescriptor{ScopeVersion: 2, CaseID: uuid.New()}
	if err := ValidateScope(scope); err == nil {
		t.Error("expected error for version 2")
	}
}

func TestValidateScope_UnsupportedOperator(t *testing.T) {
	scope := ScopeDescriptor{
		ScopeVersion: 1,
		CaseID:       uuid.New(),
		Predicate:    ScopePredicate{Op: "or", Field: "tags", Value: json.RawMessage(`"x"`)},
		Snapshot:     SnapshotAnchor{AsOfCustodyHash: "abc", EvaluatedAt: time.Now()},
	}
	if err := ValidateScope(scope); err == nil {
		t.Error("expected error for 'or' operator")
	}
}

func TestValidateScope_UnsupportedField(t *testing.T) {
	scope := ScopeDescriptor{
		ScopeVersion: 1,
		CaseID:       uuid.New(),
		Predicate:    ScopePredicate{Op: "eq", Field: "arbitrary_field", Value: json.RawMessage(`"x"`)},
		Snapshot:     SnapshotAnchor{AsOfCustodyHash: "abc", EvaluatedAt: time.Now()},
	}
	if err := ValidateScope(scope); err == nil {
		t.Error("expected error for unsupported field")
	}
}

func TestValidateScope_EmptyAndChildren(t *testing.T) {
	scope := ScopeDescriptor{
		ScopeVersion: 1,
		CaseID:       uuid.New(),
		Predicate:    ScopePredicate{Op: "and"},
		Snapshot:     SnapshotAnchor{AsOfCustodyHash: "abc", EvaluatedAt: time.Now()},
	}
	if err := ValidateScope(scope); err == nil {
		t.Error("expected error for empty and children")
	}
}

func TestValidateScope_MissingCaseID(t *testing.T) {
	scope := ScopeDescriptor{
		ScopeVersion: 1,
		Predicate:    ScopePredicate{Op: "eq", Field: "tags", Value: json.RawMessage(`"x"`)},
		Snapshot:     SnapshotAnchor{AsOfCustodyHash: "abc", EvaluatedAt: time.Now()},
	}
	if err := ValidateScope(scope); err == nil {
		t.Error("expected error for nil case_id")
	}
}

func TestCanonicalizePredicate_SortsChildren(t *testing.T) {
	pred := ScopePredicate{
		Op: "and",
		Children: []ScopePredicate{
			{Op: "eq", Field: "classification", Value: json.RawMessage(`"public"`)},
			{Op: "contains", Field: "tags", Value: json.RawMessage(`"alpha"`)},
			{Op: "gte", Field: "source_date", Value: json.RawMessage(`"2024-01-01"`)},
		},
	}

	canonical := CanonicalizePredicate(pred)

	// Expected order: classification < source_date < tags (by field)
	if canonical.Children[0].Field != "classification" {
		t.Errorf("first child field = %s, want classification", canonical.Children[0].Field)
	}
	if canonical.Children[1].Field != "source_date" {
		t.Errorf("second child field = %s, want source_date", canonical.Children[1].Field)
	}
	if canonical.Children[2].Field != "tags" {
		t.Errorf("third child field = %s, want tags", canonical.Children[2].Field)
	}
}

func TestScopeHash_Deterministic(t *testing.T) {
	caseID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	evalAt := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)

	scope := ScopeDescriptor{
		ScopeVersion: 1,
		CaseID:       caseID,
		Predicate: ScopePredicate{
			Op:    "contains",
			Field: "tags",
			Value: json.RawMessage(`"test"`),
		},
		Snapshot: SnapshotAnchor{
			AsOfCustodyHash: "deadbeef",
			EvaluatedAt:     evalAt,
		},
	}

	h1, err := ScopeHash(scope)
	if err != nil {
		t.Fatal(err)
	}
	h2, _ := ScopeHash(scope)

	if h1 != h2 {
		t.Errorf("scope hash non-deterministic: %s vs %s", h1, h2)
	}

	if len(h1) != 64 { // hex-encoded SHA-256
		t.Errorf("unexpected hash length: %d", len(h1))
	}
}

func TestScopeHash_DifferentPredicatesDifferentHash(t *testing.T) {
	caseID := uuid.New()
	evalAt := time.Now().UTC()
	base := ScopeDescriptor{
		ScopeVersion: 1,
		CaseID:       caseID,
		Snapshot:     SnapshotAnchor{AsOfCustodyHash: "abc", EvaluatedAt: evalAt},
	}

	s1 := base
	s1.Predicate = ScopePredicate{Op: "eq", Field: "classification", Value: json.RawMessage(`"public"`)}
	s2 := base
	s2.Predicate = ScopePredicate{Op: "eq", Field: "classification", Value: json.RawMessage(`"restricted"`)}

	h1, _ := ScopeHash(s1)
	h2, _ := ScopeHash(s2)

	if h1 == h2 {
		t.Error("different predicates should produce different hashes")
	}
}

func TestCanonicalizeScope_NormalizesTimezone(t *testing.T) {
	loc := time.FixedZone("PST", -8*60*60)
	evalAt := time.Date(2026, 4, 15, 4, 0, 0, 0, loc) // same instant as 12:00 UTC

	scope := ScopeDescriptor{
		ScopeVersion: 1,
		CaseID:       uuid.New(),
		Predicate:    ScopePredicate{Op: "eq", Field: "tags", Value: json.RawMessage(`"x"`)},
		Snapshot:     SnapshotAnchor{AsOfCustodyHash: "abc", EvaluatedAt: evalAt},
	}

	canonical := CanonicalizeScope(scope)
	if canonical.Snapshot.EvaluatedAt.Location() != time.UTC {
		t.Error("EvaluatedAt not normalized to UTC")
	}
}
