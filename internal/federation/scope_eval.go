package federation

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/evidence"
)

// EvaluatePredicate checks whether an evidence item matches a scope
// predicate. Returns true if the item satisfies the predicate.
func EvaluatePredicate(item evidence.EvidenceItem, pred ScopePredicate) (bool, error) {
	switch pred.Op {
	case "and":
		for _, child := range pred.Children {
			match, err := EvaluatePredicate(item, child)
			if err != nil {
				return false, err
			}
			if !match {
				return false, nil
			}
		}
		return true, nil

	case "eq":
		return evalEq(item, pred.Field, pred.Value)
	case "contains":
		return evalContains(item, pred.Field, pred.Value)
	case "in":
		return evalIn(item, pred.Field, pred.Value)
	case "gte":
		return evalGte(item, pred.Field, pred.Value)
	case "lt":
		return evalLt(item, pred.Field, pred.Value)
	default:
		return false, fmt.Errorf("unsupported operator: %q", pred.Op)
	}
}

// EvaluateScope filters evidence items by a scope predicate.
// Returns only items matching the predicate, excluding destroyed items.
func EvaluateScope(items []evidence.EvidenceItem, scope ScopeDescriptor) ([]evidence.EvidenceItem, error) {
	var matched []evidence.EvidenceItem
	for _, item := range items {
		if item.DestroyedAt != nil {
			continue
		}
		match, err := EvaluatePredicate(item, scope.Predicate)
		if err != nil {
			return nil, fmt.Errorf("evaluate predicate for %s: %w", item.ID, err)
		}
		if match {
			matched = append(matched, item)
		}
	}
	return matched, nil
}

// --- field extractors and comparators ---

func getFieldString(item evidence.EvidenceItem, field string) (string, bool) {
	switch field {
	case "classification":
		return item.Classification, true
	case "evidence_number":
		if item.EvidenceNumber != nil {
			return *item.EvidenceNumber, true
		}
		return "", false
	default:
		return "", false
	}
}

func getFieldInt(item evidence.EvidenceItem, field string) (int, bool) {
	switch field {
	case "version":
		return item.Version, true
	default:
		return 0, false
	}
}

func getFieldTime(item evidence.EvidenceItem, field string) (*time.Time, bool) {
	switch field {
	case "source_date":
		return item.SourceDate, item.SourceDate != nil
	case "retention_until":
		return item.RetentionUntil, item.RetentionUntil != nil
	default:
		return nil, false
	}
}

func getFieldUUID(item evidence.EvidenceItem, field string) (*uuid.UUID, bool) {
	switch field {
	case "parent_id":
		return item.ParentID, item.ParentID != nil
	default:
		return nil, false
	}
}

func evalEq(item evidence.EvidenceItem, field string, value json.RawMessage) (bool, error) {
	switch field {
	case "classification", "evidence_number":
		s, ok := getFieldString(item, field)
		if !ok {
			return false, nil
		}
		var target string
		if err := json.Unmarshal(value, &target); err != nil {
			return false, fmt.Errorf("unmarshal eq value for %s: %w", field, err)
		}
		return s == target, nil

	case "version":
		v, ok := getFieldInt(item, field)
		if !ok {
			return false, nil
		}
		var target int
		if err := json.Unmarshal(value, &target); err != nil {
			return false, fmt.Errorf("unmarshal eq value for %s: %w", field, err)
		}
		return v == target, nil

	case "parent_id":
		pid, ok := getFieldUUID(item, field)
		if !ok {
			return false, nil
		}
		var target string
		if err := json.Unmarshal(value, &target); err != nil {
			return false, fmt.Errorf("unmarshal eq value for %s: %w", field, err)
		}
		return pid.String() == target, nil

	case "source_date", "retention_until":
		t, ok := getFieldTime(item, field)
		if !ok {
			return false, nil
		}
		var target time.Time
		if err := json.Unmarshal(value, &target); err != nil {
			return false, fmt.Errorf("unmarshal eq value for %s: %w", field, err)
		}
		return t.Equal(target), nil

	default:
		return false, fmt.Errorf("eq not supported for field %q", field)
	}
}

func evalContains(item evidence.EvidenceItem, field string, value json.RawMessage) (bool, error) {
	if field != "tags" {
		return false, fmt.Errorf("contains only supported on tags field, got %q", field)
	}

	var target string
	if err := json.Unmarshal(value, &target); err != nil {
		return false, fmt.Errorf("unmarshal contains value: %w", err)
	}
	target = strings.ToLower(target)

	for _, tag := range item.Tags {
		if strings.ToLower(tag) == target {
			return true, nil
		}
	}
	return false, nil
}

func evalIn(item evidence.EvidenceItem, field string, value json.RawMessage) (bool, error) {
	switch field {
	case "classification", "evidence_number":
		s, ok := getFieldString(item, field)
		if !ok {
			return false, nil
		}
		var targets []string
		if err := json.Unmarshal(value, &targets); err != nil {
			return false, fmt.Errorf("unmarshal in value for %s: %w", field, err)
		}
		for _, t := range targets {
			if s == t {
				return true, nil
			}
		}
		return false, nil

	case "tags":
		var targets []string
		if err := json.Unmarshal(value, &targets); err != nil {
			return false, fmt.Errorf("unmarshal in value for tags: %w", err)
		}
		for _, target := range targets {
			target = strings.ToLower(target)
			for _, tag := range item.Tags {
				if strings.ToLower(tag) == target {
					return true, nil
				}
			}
		}
		return false, nil

	default:
		return false, fmt.Errorf("in not supported for field %q", field)
	}
}

func evalGte(item evidence.EvidenceItem, field string, value json.RawMessage) (bool, error) {
	switch field {
	case "version":
		v, ok := getFieldInt(item, field)
		if !ok {
			return false, nil
		}
		var target int
		if err := json.Unmarshal(value, &target); err != nil {
			return false, fmt.Errorf("unmarshal gte value: %w", err)
		}
		return v >= target, nil

	case "source_date", "retention_until":
		t, ok := getFieldTime(item, field)
		if !ok {
			return false, nil
		}
		var target time.Time
		if err := json.Unmarshal(value, &target); err != nil {
			return false, fmt.Errorf("unmarshal gte value: %w", err)
		}
		return !t.Before(target), nil

	default:
		return false, fmt.Errorf("gte not supported for field %q", field)
	}
}

func evalLt(item evidence.EvidenceItem, field string, value json.RawMessage) (bool, error) {
	switch field {
	case "version":
		v, ok := getFieldInt(item, field)
		if !ok {
			return false, nil
		}
		var target int
		if err := json.Unmarshal(value, &target); err != nil {
			return false, fmt.Errorf("unmarshal lt value: %w", err)
		}
		return v < target, nil

	case "source_date", "retention_until":
		t, ok := getFieldTime(item, field)
		if !ok {
			return false, nil
		}
		var target time.Time
		if err := json.Unmarshal(value, &target); err != nil {
			return false, fmt.Errorf("unmarshal lt value: %w", err)
		}
		return t.Before(target), nil

	default:
		return false, fmt.Errorf("lt not supported for field %q", field)
	}
}
