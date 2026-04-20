package federation

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/evidence"
)

func testItem(opts ...func(*evidence.EvidenceItem)) evidence.EvidenceItem {
	item := evidence.EvidenceItem{
		ID:             uuid.New(),
		CaseID:         uuid.New(),
		Version:        1,
		SHA256Hash:     "abc",
		Classification: "public",
		Tags:           []string{"alpha", "beta"},
	}
	for _, opt := range opts {
		opt(&item)
	}
	return item
}

func TestEvaluatePredicate_Contains_Tags(t *testing.T) {
	item := testItem()

	tests := []struct {
		name  string
		tag   string
		match bool
	}{
		{"matching tag", "alpha", true},
		{"case insensitive", "Alpha", true},
		{"missing tag", "gamma", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pred := ScopePredicate{Op: "contains", Field: "tags", Value: json.RawMessage(`"` + tt.tag + `"`)}
			got, err := EvaluatePredicate(item, pred)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.match {
				t.Errorf("got %v, want %v", got, tt.match)
			}
		})
	}
}

func TestEvaluatePredicate_Eq_Classification(t *testing.T) {
	item := testItem(func(i *evidence.EvidenceItem) {
		i.Classification = "restricted"
	})

	match, _ := EvaluatePredicate(item, ScopePredicate{Op: "eq", Field: "classification", Value: json.RawMessage(`"restricted"`)})
	if !match {
		t.Error("expected match for classification=restricted")
	}

	noMatch, _ := EvaluatePredicate(item, ScopePredicate{Op: "eq", Field: "classification", Value: json.RawMessage(`"public"`)})
	if noMatch {
		t.Error("expected no match for classification=public")
	}
}

func TestEvaluatePredicate_Eq_Version(t *testing.T) {
	item := testItem(func(i *evidence.EvidenceItem) {
		i.Version = 3
	})

	match, _ := EvaluatePredicate(item, ScopePredicate{Op: "eq", Field: "version", Value: json.RawMessage(`3`)})
	if !match {
		t.Error("expected match for version=3")
	}

	noMatch, _ := EvaluatePredicate(item, ScopePredicate{Op: "eq", Field: "version", Value: json.RawMessage(`1`)})
	if noMatch {
		t.Error("expected no match for version=1")
	}
}

func TestEvaluatePredicate_Gte_SourceDate(t *testing.T) {
	d := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	item := testItem(func(i *evidence.EvidenceItem) {
		i.SourceDate = &d
	})

	match, _ := EvaluatePredicate(item, ScopePredicate{Op: "gte", Field: "source_date", Value: json.RawMessage(`"2024-01-01T00:00:00Z"`)})
	if !match {
		t.Error("expected match: source_date >= 2024-01-01")
	}

	noMatch, _ := EvaluatePredicate(item, ScopePredicate{Op: "gte", Field: "source_date", Value: json.RawMessage(`"2025-01-01T00:00:00Z"`)})
	if noMatch {
		t.Error("expected no match: source_date < 2025-01-01")
	}
}

func TestEvaluatePredicate_Lt_SourceDate(t *testing.T) {
	d := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	item := testItem(func(i *evidence.EvidenceItem) {
		i.SourceDate = &d
	})

	match, _ := EvaluatePredicate(item, ScopePredicate{Op: "lt", Field: "source_date", Value: json.RawMessage(`"2025-01-01T00:00:00Z"`)})
	if !match {
		t.Error("expected match: source_date < 2025-01-01")
	}
}

func TestEvaluatePredicate_In_Classification(t *testing.T) {
	item := testItem(func(i *evidence.EvidenceItem) {
		i.Classification = "restricted"
	})

	match, _ := EvaluatePredicate(item, ScopePredicate{Op: "in", Field: "classification", Value: json.RawMessage(`["public","restricted"]`)})
	if !match {
		t.Error("expected match: classification in [public, restricted]")
	}

	noMatch, _ := EvaluatePredicate(item, ScopePredicate{Op: "in", Field: "classification", Value: json.RawMessage(`["confidential"]`)})
	if noMatch {
		t.Error("expected no match: classification not in [confidential]")
	}
}

func TestEvaluatePredicate_And(t *testing.T) {
	item := testItem(func(i *evidence.EvidenceItem) {
		i.Classification = "restricted"
		i.Tags = []string{"ukraine-2024"}
	})

	pred := ScopePredicate{
		Op: "and",
		Children: []ScopePredicate{
			{Op: "eq", Field: "classification", Value: json.RawMessage(`"restricted"`)},
			{Op: "contains", Field: "tags", Value: json.RawMessage(`"ukraine-2024"`)},
		},
	}

	match, err := EvaluatePredicate(item, pred)
	if err != nil {
		t.Fatal(err)
	}
	if !match {
		t.Error("expected match for AND(classification=restricted, tags contains ukraine-2024)")
	}

	// Change classification — AND should fail.
	item.Classification = "public"
	noMatch, _ := EvaluatePredicate(item, pred)
	if noMatch {
		t.Error("expected no match when classification != restricted")
	}
}

func TestEvaluateScope_FiltersCorrectly(t *testing.T) {
	items := []evidence.EvidenceItem{
		testItem(func(i *evidence.EvidenceItem) { i.Classification = "public"; i.Tags = []string{"ukraine"} }),
		testItem(func(i *evidence.EvidenceItem) { i.Classification = "restricted"; i.Tags = []string{"ukraine"} }),
		testItem(func(i *evidence.EvidenceItem) { i.Classification = "public"; i.Tags = []string{"other"} }),
	}

	scope := ScopeDescriptor{
		ScopeVersion: 1,
		CaseID:       items[0].CaseID,
		Predicate: ScopePredicate{
			Op: "and",
			Children: []ScopePredicate{
				{Op: "eq", Field: "classification", Value: json.RawMessage(`"public"`)},
				{Op: "contains", Field: "tags", Value: json.RawMessage(`"ukraine"`)},
			},
		},
		Snapshot: SnapshotAnchor{AsOfCustodyHash: "abc", EvaluatedAt: time.Now()},
	}

	matched, err := EvaluateScope(items, scope)
	if err != nil {
		t.Fatal(err)
	}
	if len(matched) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matched))
	}
	if matched[0].Classification != "public" {
		t.Error("wrong item matched")
	}
}

func TestEvaluateScope_ExcludesDestroyed(t *testing.T) {
	now := time.Now()
	items := []evidence.EvidenceItem{
		testItem(func(i *evidence.EvidenceItem) { i.Tags = []string{"test"} }),
		testItem(func(i *evidence.EvidenceItem) { i.Tags = []string{"test"}; i.DestroyedAt = &now }),
	}

	scope := ScopeDescriptor{
		ScopeVersion: 1,
		CaseID:       items[0].CaseID,
		Predicate:    ScopePredicate{Op: "contains", Field: "tags", Value: json.RawMessage(`"test"`)},
		Snapshot:     SnapshotAnchor{AsOfCustodyHash: "abc", EvaluatedAt: time.Now()},
	}

	matched, err := EvaluateScope(items, scope)
	if err != nil {
		t.Fatal(err)
	}
	if len(matched) != 1 {
		t.Errorf("expected 1 match (destroyed excluded), got %d", len(matched))
	}
}

func TestEvaluatePredicate_NilFieldReturnsNoMatch(t *testing.T) {
	item := testItem() // no source_date

	match, _ := EvaluatePredicate(item, ScopePredicate{Op: "gte", Field: "source_date", Value: json.RawMessage(`"2024-01-01T00:00:00Z"`)})
	if match {
		t.Error("nil source_date should not match gte")
	}
}
