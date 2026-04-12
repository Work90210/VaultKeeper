package search

import (
	"encoding/json"
	"testing"
)

// TestParseEvidenceHit_WithExParteSide covers the ex_parte_side branch
// in parseEvidenceHit which was previously 0% because existing fixtures
// did not include the field.
func TestParseEvidenceHit_WithExParteSide(t *testing.T) {
	raw := map[string]json.RawMessage{
		"id":              json.RawMessage(`"evid-1"`),
		"case_id":         json.RawMessage(`"case-1"`),
		"title":           json.RawMessage(`"t"`),
		"description":     json.RawMessage(`"d"`),
		"evidence_number": json.RawMessage(`"E1"`),
		"classification":  json.RawMessage(`"ex_parte"`),
		"ex_parte_side":   json.RawMessage(`"prosecution"`),
	}
	hit := parseEvidenceHit(raw)
	if hit.ExParteSide == nil || *hit.ExParteSide != "prosecution" {
		t.Errorf("ExParteSide = %v, want prosecution", hit.ExParteSide)
	}
	if hit.Classification != "ex_parte" {
		t.Errorf("Classification = %q", hit.Classification)
	}
}

// TestParseEvidenceHit_EmptyExParteSide verifies the string-empty guard
// (ex_parte_side field present but equal to "") leaves the pointer nil.
func TestParseEvidenceHit_EmptyExParteSide(t *testing.T) {
	raw := map[string]json.RawMessage{
		"id":            json.RawMessage(`"evid-1"`),
		"case_id":       json.RawMessage(`"case-1"`),
		"ex_parte_side": json.RawMessage(`""`),
	}
	hit := parseEvidenceHit(raw)
	if hit.ExParteSide != nil {
		t.Errorf("empty ex_parte_side should leave pointer nil, got %v", *hit.ExParteSide)
	}
}

// TestParseEvidenceHit_MalformedExParteSide exercises the Unmarshal
// error branch (field present but not a JSON string).
func TestParseEvidenceHit_MalformedExParteSide(t *testing.T) {
	raw := map[string]json.RawMessage{
		"id":            json.RawMessage(`"evid-1"`),
		"ex_parte_side": json.RawMessage(`12345`), // number, not string
	}
	hit := parseEvidenceHit(raw)
	if hit.ExParteSide != nil {
		t.Error("malformed ex_parte_side should leave pointer nil")
	}
}
