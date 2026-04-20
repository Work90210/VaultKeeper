package federation

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

const (
	// ScopeDomain is the domain separator for scope hashes.
	ScopeDomain = "VK:SCOPE:v1"
)

// ScopeDescriptor defines what evidence is being shared — a
// deterministic, versioned predicate over a case's evidence set.
type ScopeDescriptor struct {
	ScopeVersion int            `json:"scope_version"`
	CaseID       uuid.UUID      `json:"case_id"`
	Predicate    ScopePredicate `json:"predicate"`
	Snapshot     SnapshotAnchor `json:"snapshot"`
}

// ScopePredicate is a node in the scope predicate tree.
// Leaf nodes have Op+Field+Value. Branch nodes have Op="and"+Children.
type ScopePredicate struct {
	Op       string           `json:"op"`
	Field    string           `json:"field,omitempty"`
	Value    json.RawMessage  `json:"value,omitempty"`
	Children []ScopePredicate `json:"children,omitempty"`
}

// SnapshotAnchor binds a scope evaluation to a specific point in the
// custody chain, making the scope deterministic and auditable.
type SnapshotAnchor struct {
	AsOfCustodyHash string    `json:"as_of_custody_hash"`
	EvaluatedAt     time.Time `json:"evaluated_at"`
}

// v1 supported fields and operators.
var (
	v1Fields = map[string]bool{
		"tags":             true,
		"source_date":      true,
		"classification":   true,
		"evidence_number":  true,
		"version":          true,
		"parent_id":        true,
		"retention_until":  true,
	}

	v1Operators = map[string]bool{
		"eq":       true,
		"contains": true,
		"in":       true,
		"gte":      true,
		"lt":       true,
		"and":      true,
	}
)

// ValidateScope checks that a scope descriptor uses only v1 fields
// and operators, and that the structure is well-formed.
func ValidateScope(scope ScopeDescriptor) error {
	if scope.ScopeVersion != 1 {
		return fmt.Errorf("unsupported scope version: %d", scope.ScopeVersion)
	}
	if scope.CaseID == uuid.Nil {
		return fmt.Errorf("scope case_id is required")
	}
	if scope.Snapshot.AsOfCustodyHash == "" {
		return fmt.Errorf("snapshot as_of_custody_hash is required")
	}
	if scope.Snapshot.EvaluatedAt.IsZero() {
		return fmt.Errorf("snapshot evaluated_at is required")
	}
	return validatePredicate(scope.Predicate)
}

func validatePredicate(p ScopePredicate) error {
	if !v1Operators[p.Op] {
		return fmt.Errorf("unsupported operator: %q", p.Op)
	}

	if p.Op == "and" {
		if len(p.Children) == 0 {
			return fmt.Errorf("'and' predicate requires at least one child")
		}
		for i, child := range p.Children {
			if err := validatePredicate(child); err != nil {
				return fmt.Errorf("and.children[%d]: %w", i, err)
			}
		}
		return nil
	}

	// Leaf predicate: must have field and value.
	if p.Field == "" {
		return fmt.Errorf("operator %q requires a field", p.Op)
	}
	if !v1Fields[p.Field] {
		return fmt.Errorf("unsupported field: %q", p.Field)
	}
	if len(p.Value) == 0 {
		return fmt.Errorf("operator %q on field %q requires a value", p.Op, p.Field)
	}
	if len(p.Children) > 0 {
		return fmt.Errorf("leaf predicate %q must not have children", p.Op)
	}

	return nil
}

// CanonicalizePredicate returns a copy of the predicate with children
// sorted lexicographically by (field, op, canonical_json(value)).
// This ensures deterministic scope hashing regardless of input order.
func CanonicalizePredicate(p ScopePredicate) ScopePredicate {
	result := ScopePredicate{
		Op:    p.Op,
		Field: p.Field,
		Value: p.Value,
	}

	if len(p.Children) > 0 {
		children := make([]ScopePredicate, len(p.Children))
		for i, child := range p.Children {
			children[i] = CanonicalizePredicate(child)
		}

		sort.Slice(children, func(i, j int) bool {
			if children[i].Field != children[j].Field {
				return children[i].Field < children[j].Field
			}
			if children[i].Op != children[j].Op {
				return children[i].Op < children[j].Op
			}
			return string(children[i].Value) < string(children[j].Value)
		})

		result.Children = children
	}

	return result
}

// CanonicalizeScope returns a copy of the scope with predicates
// canonicalized and the EvaluatedAt timestamp normalized to UTC.
func CanonicalizeScope(scope ScopeDescriptor) ScopeDescriptor {
	return ScopeDescriptor{
		ScopeVersion: scope.ScopeVersion,
		CaseID:       scope.CaseID,
		Predicate:    CanonicalizePredicate(scope.Predicate),
		Snapshot: SnapshotAnchor{
			AsOfCustodyHash: scope.Snapshot.AsOfCustodyHash,
			EvaluatedAt:     scope.Snapshot.EvaluatedAt.UTC(),
		},
	}
}

// ScopeHash computes SHA-256("VK:SCOPE:v1" || canonical_json(scope)).
// The scope is canonicalized before hashing.
func ScopeHash(scope ScopeDescriptor) (string, error) {
	canonical := CanonicalizeScope(scope)
	return HexHash(ScopeDomain, canonical)
}
