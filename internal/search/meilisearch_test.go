package search

import (
	"context"
	"encoding/json"
	"testing"
)

// --- NoopSearchIndexer tests ---

func TestNoopSearchIndexer_IndexDocument(t *testing.T) {
	noop := &NoopSearchIndexer{}
	err := noop.IndexDocument(context.Background(), Document{
		ID:    "test-id",
		Index: "test-index",
		Payload: map[string]any{
			"field": "value",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNoopSearchIndexer_DeleteDocument(t *testing.T) {
	noop := &NoopSearchIndexer{}
	err := noop.DeleteDocument(context.Background(), "test-index", "test-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- NoopEvidenceSearcher tests ---

func TestNoopEvidenceSearcher_IndexEvidence(t *testing.T) {
	noop := &NoopEvidenceSearcher{}
	err := noop.IndexEvidence(context.Background(), EvidenceSearchDoc{
		ID:     "ev-1",
		CaseID: "case-1",
		Title:  "Test Evidence",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNoopEvidenceSearcher_RemoveEvidence(t *testing.T) {
	noop := &NoopEvidenceSearcher{}
	err := noop.RemoveEvidence(context.Background(), "ev-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNoopEvidenceSearcher_SearchEvidence(t *testing.T) {
	noop := &NoopEvidenceSearcher{}
	result, err := noop.SearchEvidence(context.Background(), SearchQuery{Query: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Hits) != 0 {
		t.Errorf("expected 0 hits, got %d", len(result.Hits))
	}
	if result.TotalHits != 0 {
		t.Errorf("expected 0 total hits, got %d", result.TotalHits)
	}
}

func TestNoopEvidenceSearcher_ConfigureEvidenceIndex(t *testing.T) {
	noop := &NoopEvidenceSearcher{}
	err := noop.ConfigureEvidenceIndex(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNoopEvidenceSearcher_ReindexAll(t *testing.T) {
	noop := &NoopEvidenceSearcher{}
	err := noop.ReindexAll(context.Background(), "case-1", []EvidenceSearchDoc{
		{ID: "ev-1", CaseID: "case-1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- escapeFilterValue tests ---

func TestEscapeFilterValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "no quotes", input: "hello", expected: "hello"},
		{name: "single quote", input: "it's", expected: `it\'s`},
		{name: "multiple quotes", input: "it's a test's value", expected: `it\'s a test\'s value`},
		{name: "empty string", input: "", expected: ""},
		{name: "only quotes", input: "'''", expected: `\'\'\'`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeFilterValue(tt.input)
			if got != tt.expected {
				t.Errorf("escapeFilterValue(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// --- buildEvidenceFilter tests ---

func TestBuildEvidenceFilter_Empty(t *testing.T) {
	filter := buildEvidenceFilter(SearchQuery{})
	if filter != "" {
		t.Errorf("expected empty filter, got %q", filter)
	}
}

func TestBuildEvidenceFilter_CaseID(t *testing.T) {
	caseID := "case-123"
	filter := buildEvidenceFilter(SearchQuery{CaseID: &caseID})
	expected := "case_id = 'case-123'"
	if filter != expected {
		t.Errorf("expected %q, got %q", expected, filter)
	}
}

func TestBuildEvidenceFilter_EmptyCaseID(t *testing.T) {
	empty := ""
	filter := buildEvidenceFilter(SearchQuery{CaseID: &empty})
	if filter != "" {
		t.Errorf("expected empty filter for empty case_id, got %q", filter)
	}
}

func TestBuildEvidenceFilter_UserCaseIDs(t *testing.T) {
	filter := buildEvidenceFilter(SearchQuery{UserCaseIDs: []string{"c1", "c2", "c3"}})
	expected := "case_id IN ['c1', 'c2', 'c3']"
	if filter != expected {
		t.Errorf("expected %q, got %q", expected, filter)
	}
}

func TestBuildEvidenceFilter_MimeTypes(t *testing.T) {
	filter := buildEvidenceFilter(SearchQuery{MimeTypes: []string{"image/jpeg", "application/pdf"}})
	expected := "mime_type IN ['image/jpeg', 'application/pdf']"
	if filter != expected {
		t.Errorf("expected %q, got %q", expected, filter)
	}
}

func TestBuildEvidenceFilter_Classifications(t *testing.T) {
	filter := buildEvidenceFilter(SearchQuery{Classifications: []string{"confidential", "restricted"}})
	expected := "classification IN ['confidential', 'restricted']"
	if filter != expected {
		t.Errorf("expected %q, got %q", expected, filter)
	}
}

func TestBuildEvidenceFilter_Tags(t *testing.T) {
	filter := buildEvidenceFilter(SearchQuery{Tags: []string{"photo", "weapon"}})
	expected := "(tags = 'photo' OR tags = 'weapon')"
	if filter != expected {
		t.Errorf("expected %q, got %q", expected, filter)
	}
}

func TestBuildEvidenceFilter_SingleTag(t *testing.T) {
	filter := buildEvidenceFilter(SearchQuery{Tags: []string{"evidence"}})
	expected := "(tags = 'evidence')"
	if filter != expected {
		t.Errorf("expected %q, got %q", expected, filter)
	}
}

func TestBuildEvidenceFilter_DisclosedOnly(t *testing.T) {
	filter := buildEvidenceFilter(SearchQuery{DisclosedOnly: true})
	expected := "is_disclosed = true"
	if filter != expected {
		t.Errorf("expected %q, got %q", expected, filter)
	}
}

func TestBuildEvidenceFilter_DateFrom(t *testing.T) {
	from := "2024-01-01"
	filter := buildEvidenceFilter(SearchQuery{DateFrom: &from})
	expected := "source_date >= '2024-01-01'"
	if filter != expected {
		t.Errorf("expected %q, got %q", expected, filter)
	}
}

func TestBuildEvidenceFilter_EmptyDateFrom(t *testing.T) {
	empty := ""
	filter := buildEvidenceFilter(SearchQuery{DateFrom: &empty})
	if filter != "" {
		t.Errorf("expected empty filter for empty date_from, got %q", filter)
	}
}

func TestBuildEvidenceFilter_DateTo(t *testing.T) {
	to := "2024-12-31"
	filter := buildEvidenceFilter(SearchQuery{DateTo: &to})
	expected := "source_date <= '2024-12-31'"
	if filter != expected {
		t.Errorf("expected %q, got %q", expected, filter)
	}
}

func TestBuildEvidenceFilter_EmptyDateTo(t *testing.T) {
	empty := ""
	filter := buildEvidenceFilter(SearchQuery{DateTo: &empty})
	if filter != "" {
		t.Errorf("expected empty filter for empty date_to, got %q", filter)
	}
}

func TestBuildEvidenceFilter_AllFilters(t *testing.T) {
	caseID := "case-1"
	from := "2024-01-01"
	to := "2024-12-31"

	filter := buildEvidenceFilter(SearchQuery{
		CaseID:          &caseID,
		UserCaseIDs:     []string{"case-1", "case-2"},
		MimeTypes:       []string{"image/jpeg"},
		Classifications: []string{"confidential"},
		Tags:            []string{"photo"},
		DisclosedOnly:   true,
		DateFrom:        &from,
		DateTo:          &to,
	})

	// Verify all parts are present and joined with AND
	expectedParts := []string{
		"case_id = 'case-1'",
		"case_id IN ['case-1', 'case-2']",
		"mime_type IN ['image/jpeg']",
		"classification IN ['confidential']",
		"(tags = 'photo')",
		"is_disclosed = true",
		"source_date >= '2024-01-01'",
		"source_date <= '2024-12-31'",
	}

	for _, part := range expectedParts {
		if !containsSubstring(filter, part) {
			t.Errorf("filter %q missing expected part %q", filter, part)
		}
	}

	// Should contain ANDs
	if !containsSubstring(filter, " AND ") {
		t.Errorf("expected filter parts joined with AND, got %q", filter)
	}
}

func TestBuildEvidenceFilter_EscapesQuotes(t *testing.T) {
	caseID := "case's-test"
	filter := buildEvidenceFilter(SearchQuery{CaseID: &caseID})
	expected := "case_id = 'case\\'s-test'"
	if filter != expected {
		t.Errorf("expected %q, got %q", expected, filter)
	}
}

// --- parseFacetDistribution tests ---

func TestParseFacetDistribution_Nil(t *testing.T) {
	result := parseFacetDistribution(nil)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestParseFacetDistribution_WrongType(t *testing.T) {
	result := parseFacetDistribution("not a map")
	if result != nil {
		t.Errorf("expected nil for wrong type, got %v", result)
	}
}

func TestParseFacetDistribution_ValidData(t *testing.T) {
	input := map[string]interface{}{
		"mime_type": map[string]interface{}{
			"image/jpeg":      float64(10),
			"application/pdf": float64(5),
		},
		"classification": map[string]interface{}{
			"confidential": float64(3),
			"restricted":   float64(7),
		},
	}

	result := parseFacetDistribution(input)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 facets, got %d", len(result))
	}
	if result["mime_type"]["image/jpeg"] != 10 {
		t.Errorf("expected mime_type image/jpeg=10, got %d", result["mime_type"]["image/jpeg"])
	}
	if result["classification"]["restricted"] != 7 {
		t.Errorf("expected classification restricted=7, got %d", result["classification"]["restricted"])
	}
}

func TestParseFacetDistribution_IntValues(t *testing.T) {
	input := map[string]interface{}{
		"tags": map[string]interface{}{
			"photo": int(4),
		},
	}

	result := parseFacetDistribution(input)
	if result["tags"]["photo"] != 4 {
		t.Errorf("expected tags photo=4, got %d", result["tags"]["photo"])
	}
}

func TestParseFacetDistribution_NonMapInner(t *testing.T) {
	input := map[string]interface{}{
		"tags": "not a map",
	}

	result := parseFacetDistribution(input)
	if len(result) != 0 {
		t.Errorf("expected empty result for non-map inner, got %v", result)
	}
}

func TestParseFacetDistribution_UnsupportedValueType(t *testing.T) {
	input := map[string]interface{}{
		"tags": map[string]interface{}{
			"photo": "not a number",
		},
	}

	result := parseFacetDistribution(input)
	// The "photo" key should be absent since "not a number" isn't float64 or int
	if _, ok := result["tags"]["photo"]; ok {
		t.Errorf("expected photo to be absent for unsupported type")
	}
}

func TestParseFacetDistribution_EmptyOuter(t *testing.T) {
	input := map[string]interface{}{}
	result := parseFacetDistribution(input)
	if result == nil {
		t.Fatal("expected non-nil empty map")
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

// --- parseEvidenceHit tests ---

func TestParseEvidenceHit_BasicFields(t *testing.T) {
	hit := map[string]json.RawMessage{
		"id":              mustMarshal(t, "ev-1"),
		"case_id":         mustMarshal(t, "case-1"),
		"title":           mustMarshal(t, "Test Document"),
		"description":     mustMarshal(t, "A test description"),
		"evidence_number": mustMarshal(t, "EV-001"),
	}

	result := parseEvidenceHit(hit)
	if result.EvidenceID != "ev-1" {
		t.Errorf("expected evidence ID 'ev-1', got %q", result.EvidenceID)
	}
	if result.CaseID != "case-1" {
		t.Errorf("expected case ID 'case-1', got %q", result.CaseID)
	}
	if result.Title != "Test Document" {
		t.Errorf("expected title 'Test Document', got %q", result.Title)
	}
	if result.Description != "A test description" {
		t.Errorf("expected description 'A test description', got %q", result.Description)
	}
	if result.EvidenceNumber != "EV-001" {
		t.Errorf("expected evidence number 'EV-001', got %q", result.EvidenceNumber)
	}
}

func TestParseEvidenceHit_EmptyHit(t *testing.T) {
	hit := map[string]json.RawMessage{}
	result := parseEvidenceHit(hit)
	if result.EvidenceID != "" {
		t.Errorf("expected empty evidence ID, got %q", result.EvidenceID)
	}
	if result.Score != 0 {
		t.Errorf("expected zero score, got %f", result.Score)
	}
	if len(result.Highlights) != 0 {
		t.Errorf("expected empty highlights, got %v", result.Highlights)
	}
}

func TestParseEvidenceHit_WithHighlights(t *testing.T) {
	formatted := map[string]json.RawMessage{
		"title":           mustMarshal(t, "<em>highlighted</em> title"),
		"description":     mustMarshal(t, "some <em>highlighted</em> text"),
		"evidence_number": mustMarshal(t, "EV-<em>001</em>"),
	}

	hit := map[string]json.RawMessage{
		"id":         mustMarshal(t, "ev-1"),
		"_formatted": mustMarshal(t, formatted),
	}

	result := parseEvidenceHit(hit)
	if len(result.Highlights) != 3 {
		t.Fatalf("expected 3 highlight fields, got %d", len(result.Highlights))
	}
	if result.Highlights["title"][0] != "<em>highlighted</em> title" {
		t.Errorf("unexpected title highlight: %q", result.Highlights["title"][0])
	}
}

func TestParseEvidenceHit_WithRankingScore(t *testing.T) {
	hit := map[string]json.RawMessage{
		"id":            mustMarshal(t, "ev-1"),
		"_rankingScore": mustMarshal(t, 0.95),
	}

	result := parseEvidenceHit(hit)
	if result.Score != 0.95 {
		t.Errorf("expected score 0.95, got %f", result.Score)
	}
}

func TestParseEvidenceHit_InvalidJSONField(t *testing.T) {
	hit := map[string]json.RawMessage{
		"id":    []byte(`not valid json`),
		"title": mustMarshal(t, "valid title"),
	}

	result := parseEvidenceHit(hit)
	// id should be empty since unmarshal fails
	if result.EvidenceID != "" {
		t.Errorf("expected empty evidence ID for invalid JSON, got %q", result.EvidenceID)
	}
	if result.Title != "valid title" {
		t.Errorf("expected title 'valid title', got %q", result.Title)
	}
}

func TestParseEvidenceHit_FormattedNotMap(t *testing.T) {
	hit := map[string]json.RawMessage{
		"id":         mustMarshal(t, "ev-1"),
		"_formatted": mustMarshal(t, "not a map"),
	}

	result := parseEvidenceHit(hit)
	// Should not crash, highlights should be empty
	if len(result.Highlights) != 0 {
		t.Errorf("expected empty highlights for non-map _formatted, got %v", result.Highlights)
	}
}

func TestParseEvidenceHit_FormattedEmptyHighlightValue(t *testing.T) {
	formatted := map[string]json.RawMessage{
		"title": mustMarshal(t, ""),
	}

	hit := map[string]json.RawMessage{
		"id":         mustMarshal(t, "ev-1"),
		"_formatted": mustMarshal(t, formatted),
	}

	result := parseEvidenceHit(hit)
	// Empty string should not be added as highlight
	if _, ok := result.Highlights["title"]; ok {
		t.Errorf("expected no highlight for empty title, got %v", result.Highlights["title"])
	}
}

func TestParseEvidenceHit_FormattedNonStringValue(t *testing.T) {
	formatted := map[string]json.RawMessage{
		"title": mustMarshal(t, 12345),
	}

	hit := map[string]json.RawMessage{
		"id":         mustMarshal(t, "ev-1"),
		"_formatted": mustMarshal(t, formatted),
	}

	result := parseEvidenceHit(hit)
	// Non-string value should not produce a highlight
	if _, ok := result.Highlights["title"]; ok {
		t.Errorf("expected no highlight for non-string value, got %v", result.Highlights["title"])
	}
}

// --- helpers ---

func mustMarshal(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
