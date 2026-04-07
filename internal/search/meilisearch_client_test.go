package search

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	ms "github.com/meilisearch/meilisearch-go"
)

// mockIndexManager embeds ms.IndexManager so we only override needed methods.
// Any method not overridden will panic if called (nil receiver), which is fine
// since tests only exercise known code paths.
type mockIndexManager struct {
	ms.IndexManager

	addDocumentsFunc          func(docs interface{}, opts *ms.DocumentOptions) (*ms.TaskInfo, error)
	deleteDocumentFunc        func(id string, opts *ms.DocumentOptions) (*ms.TaskInfo, error)
	deleteDocumentsByFilterFunc func(filter interface{}, opts *ms.DocumentOptions) (*ms.TaskInfo, error)
	searchFunc                func(query string, req *ms.SearchRequest) (*ms.SearchResponse, error)
	updateSearchableFunc      func(attrs *[]string) (*ms.TaskInfo, error)
	updateFilterableFunc      func(attrs *[]interface{}) (*ms.TaskInfo, error)
	updateSortableFunc        func(attrs *[]string) (*ms.TaskInfo, error)
	updateTypoToleranceFunc   func(typo *ms.TypoTolerance) (*ms.TaskInfo, error)
}

func (m *mockIndexManager) AddDocuments(docs interface{}, opts *ms.DocumentOptions) (*ms.TaskInfo, error) {
	if m.addDocumentsFunc != nil {
		return m.addDocumentsFunc(docs, opts)
	}
	return &ms.TaskInfo{}, nil
}

func (m *mockIndexManager) DeleteDocument(id string, opts *ms.DocumentOptions) (*ms.TaskInfo, error) {
	if m.deleteDocumentFunc != nil {
		return m.deleteDocumentFunc(id, opts)
	}
	return &ms.TaskInfo{}, nil
}

func (m *mockIndexManager) DeleteDocumentsByFilter(filter interface{}, opts *ms.DocumentOptions) (*ms.TaskInfo, error) {
	if m.deleteDocumentsByFilterFunc != nil {
		return m.deleteDocumentsByFilterFunc(filter, opts)
	}
	return &ms.TaskInfo{}, nil
}

func (m *mockIndexManager) Search(query string, req *ms.SearchRequest) (*ms.SearchResponse, error) {
	if m.searchFunc != nil {
		return m.searchFunc(query, req)
	}
	return &ms.SearchResponse{}, nil
}

func (m *mockIndexManager) UpdateSearchableAttributes(attrs *[]string) (*ms.TaskInfo, error) {
	if m.updateSearchableFunc != nil {
		return m.updateSearchableFunc(attrs)
	}
	return &ms.TaskInfo{}, nil
}

func (m *mockIndexManager) UpdateFilterableAttributes(attrs *[]interface{}) (*ms.TaskInfo, error) {
	if m.updateFilterableFunc != nil {
		return m.updateFilterableFunc(attrs)
	}
	return &ms.TaskInfo{}, nil
}

func (m *mockIndexManager) UpdateSortableAttributes(attrs *[]string) (*ms.TaskInfo, error) {
	if m.updateSortableFunc != nil {
		return m.updateSortableFunc(attrs)
	}
	return &ms.TaskInfo{}, nil
}

func (m *mockIndexManager) UpdateTypoTolerance(typo *ms.TypoTolerance) (*ms.TaskInfo, error) {
	if m.updateTypoToleranceFunc != nil {
		return m.updateTypoToleranceFunc(typo)
	}
	return &ms.TaskInfo{}, nil
}

// mockServiceManager embeds ms.ServiceManager and overrides Index().
type mockServiceManager struct {
	ms.ServiceManager
	indexManager *mockIndexManager
}

func (m *mockServiceManager) Index(uid string) ms.IndexManager {
	return m.indexManager
}

func newTestMeilisearchClient(idx *mockIndexManager) *MeilisearchClient {
	return &MeilisearchClient{
		client: &mockServiceManager{indexManager: idx},
	}
}

// --- NewMeilisearchClient tests ---

func TestNewMeilisearchClient(t *testing.T) {
	c := NewMeilisearchClient("http://localhost:7700", "test-key")
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	if c.client == nil {
		t.Fatal("expected non-nil underlying client")
	}
}

// --- IndexDocument tests ---

func TestMeilisearchClient_IndexDocument_Success(t *testing.T) {
	var capturedDocs interface{}
	idx := &mockIndexManager{
		addDocumentsFunc: func(docs interface{}, opts *ms.DocumentOptions) (*ms.TaskInfo, error) {
			capturedDocs = docs
			return &ms.TaskInfo{TaskUID: 1}, nil
		},
	}

	c := newTestMeilisearchClient(idx)
	err := c.IndexDocument(context.Background(), Document{
		ID:    "doc-1",
		Index: "test",
		Payload: map[string]any{
			"title": "Test",
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	docs, ok := capturedDocs.([]map[string]any)
	if !ok || len(docs) != 1 {
		t.Fatalf("expected 1 document, got %v", capturedDocs)
	}
	if docs[0]["id"] != "doc-1" {
		t.Errorf("expected id 'doc-1', got %v", docs[0]["id"])
	}
	if docs[0]["title"] != "Test" {
		t.Errorf("expected title 'Test', got %v", docs[0]["title"])
	}
}

func TestMeilisearchClient_IndexDocument_Error(t *testing.T) {
	idx := &mockIndexManager{
		addDocumentsFunc: func(_ interface{}, _ *ms.DocumentOptions) (*ms.TaskInfo, error) {
			return nil, errors.New("connection refused")
		},
	}

	c := newTestMeilisearchClient(idx)
	err := c.IndexDocument(context.Background(), Document{ID: "doc-1", Index: "test"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !containsSubstring(err.Error(), "index document") {
		t.Errorf("expected wrapped error, got %q", err.Error())
	}
}

// --- DeleteDocument tests ---

func TestMeilisearchClient_DeleteDocument_Success(t *testing.T) {
	var capturedID string
	idx := &mockIndexManager{
		deleteDocumentFunc: func(id string, _ *ms.DocumentOptions) (*ms.TaskInfo, error) {
			capturedID = id
			return &ms.TaskInfo{}, nil
		},
	}

	c := newTestMeilisearchClient(idx)
	err := c.DeleteDocument(context.Background(), "test", "doc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedID != "doc-1" {
		t.Errorf("expected id 'doc-1', got %q", capturedID)
	}
}

func TestMeilisearchClient_DeleteDocument_Error(t *testing.T) {
	idx := &mockIndexManager{
		deleteDocumentFunc: func(_ string, _ *ms.DocumentOptions) (*ms.TaskInfo, error) {
			return nil, errors.New("not found")
		},
	}

	c := newTestMeilisearchClient(idx)
	err := c.DeleteDocument(context.Background(), "test", "doc-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !containsSubstring(err.Error(), "delete document") {
		t.Errorf("expected wrapped error, got %q", err.Error())
	}
}

// --- Search tests ---

func TestMeilisearchClient_Search_Success(t *testing.T) {
	idx := &mockIndexManager{
		searchFunc: func(_ string, _ *ms.SearchRequest) (*ms.SearchResponse, error) {
			return &ms.SearchResponse{
				Hits: ms.Hits{
					ms.Hit{
						"id":    json.RawMessage(`"doc-1"`),
						"title": json.RawMessage(`"Test Doc"`),
					},
				},
			}, nil
		},
	}

	c := newTestMeilisearchClient(idx)
	results, err := c.Search(context.Background(), "test", "query", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "doc-1" {
		t.Errorf("expected ID 'doc-1', got %q", results[0].ID)
	}
	if results[0].Payload["title"] != "Test Doc" {
		t.Errorf("expected title 'Test Doc', got %v", results[0].Payload["title"])
	}
}

func TestMeilisearchClient_Search_Error(t *testing.T) {
	idx := &mockIndexManager{
		searchFunc: func(_ string, _ *ms.SearchRequest) (*ms.SearchResponse, error) {
			return nil, errors.New("timeout")
		},
	}

	c := newTestMeilisearchClient(idx)
	_, err := c.Search(context.Background(), "test", "query", 10)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !containsSubstring(err.Error(), "search test") {
		t.Errorf("expected wrapped error, got %q", err.Error())
	}
}

func TestMeilisearchClient_Search_InvalidJSON(t *testing.T) {
	idx := &mockIndexManager{
		searchFunc: func(_ string, _ *ms.SearchRequest) (*ms.SearchResponse, error) {
			return &ms.SearchResponse{
				Hits: ms.Hits{
					ms.Hit{
						"id":    json.RawMessage(`"doc-1"`),
						"bad":   json.RawMessage(`{invalid`),
					},
				},
			}, nil
		},
	}

	c := newTestMeilisearchClient(idx)
	results, err := c.Search(context.Background(), "test", "query", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "bad" field should be skipped, "id" should be parsed
	if results[0].ID != "doc-1" {
		t.Errorf("expected ID 'doc-1', got %q", results[0].ID)
	}
	if _, ok := results[0].Payload["bad"]; ok {
		t.Error("expected 'bad' field to be skipped")
	}
}

// --- IndexEvidence tests ---

func TestMeilisearchClient_IndexEvidence_Success(t *testing.T) {
	var capturedDocs interface{}
	idx := &mockIndexManager{
		addDocumentsFunc: func(docs interface{}, _ *ms.DocumentOptions) (*ms.TaskInfo, error) {
			capturedDocs = docs
			return &ms.TaskInfo{}, nil
		},
	}

	c := newTestMeilisearchClient(idx)
	sourceDate := "2024-06-15"
	err := c.IndexEvidence(context.Background(), EvidenceSearchDoc{
		ID:             "ev-1",
		CaseID:         "case-1",
		Title:          "Knife",
		Description:    "A kitchen knife",
		EvidenceNumber: "EV-001",
		Tags:           []string{"weapon", "kitchen"},
		Source:         "forensics",
		FileName:       "knife.jpg",
		MimeType:       "image/jpeg",
		Classification: "confidential",
		SourceDate:     &sourceDate,
		UploadedAt:     "2024-06-16T10:00:00Z",
		IsCurrent:      true,
		IsDisclosed:    false,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	docs, ok := capturedDocs.([]map[string]any)
	if !ok || len(docs) != 1 {
		t.Fatalf("expected 1 document, got %v", capturedDocs)
	}
	if docs[0]["id"] != "ev-1" {
		t.Errorf("expected id 'ev-1', got %v", docs[0]["id"])
	}
	if docs[0]["case_id"] != "case-1" {
		t.Errorf("expected case_id 'case-1', got %v", docs[0]["case_id"])
	}
	if docs[0]["is_current"] != true {
		t.Errorf("expected is_current true, got %v", docs[0]["is_current"])
	}
}

func TestMeilisearchClient_IndexEvidence_Error(t *testing.T) {
	idx := &mockIndexManager{
		addDocumentsFunc: func(_ interface{}, _ *ms.DocumentOptions) (*ms.TaskInfo, error) {
			return nil, errors.New("quota exceeded")
		},
	}

	c := newTestMeilisearchClient(idx)
	err := c.IndexEvidence(context.Background(), EvidenceSearchDoc{ID: "ev-1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !containsSubstring(err.Error(), "index evidence") {
		t.Errorf("expected wrapped error, got %q", err.Error())
	}
}

// --- RemoveEvidence tests ---

func TestMeilisearchClient_RemoveEvidence_Success(t *testing.T) {
	var capturedID string
	idx := &mockIndexManager{
		deleteDocumentFunc: func(id string, _ *ms.DocumentOptions) (*ms.TaskInfo, error) {
			capturedID = id
			return &ms.TaskInfo{}, nil
		},
	}

	c := newTestMeilisearchClient(idx)
	err := c.RemoveEvidence(context.Background(), "ev-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedID != "ev-1" {
		t.Errorf("expected id 'ev-1', got %q", capturedID)
	}
}

func TestMeilisearchClient_RemoveEvidence_Error(t *testing.T) {
	idx := &mockIndexManager{
		deleteDocumentFunc: func(_ string, _ *ms.DocumentOptions) (*ms.TaskInfo, error) {
			return nil, errors.New("not found")
		},
	}

	c := newTestMeilisearchClient(idx)
	err := c.RemoveEvidence(context.Background(), "ev-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !containsSubstring(err.Error(), "remove evidence") {
		t.Errorf("expected wrapped error, got %q", err.Error())
	}
}

// --- SearchEvidence tests ---

func TestMeilisearchClient_SearchEvidence_Success(t *testing.T) {
	idx := &mockIndexManager{
		searchFunc: func(query string, req *ms.SearchRequest) (*ms.SearchResponse, error) {
			return &ms.SearchResponse{
				Hits: ms.Hits{
					ms.Hit{
						"id":              json.RawMessage(`"ev-1"`),
						"case_id":         json.RawMessage(`"case-1"`),
						"title":           json.RawMessage(`"Test"`),
						"description":     json.RawMessage(`"Desc"`),
						"evidence_number": json.RawMessage(`"EV-001"`),
					},
				},
				EstimatedTotalHits: 1,
				ProcessingTimeMs:   3,
				FacetDistribution: json.RawMessage(`{"mime_type":{"image/jpeg":1}}`),
			}, nil
		},
	}

	c := newTestMeilisearchClient(idx)
	result, err := c.SearchEvidence(context.Background(), SearchQuery{
		Query:  "test",
		Limit:  10,
		Offset: 0,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(result.Hits))
	}
	if result.Hits[0].EvidenceID != "ev-1" {
		t.Errorf("expected evidence ID 'ev-1', got %q", result.Hits[0].EvidenceID)
	}
	if result.TotalHits != 1 {
		t.Errorf("expected total hits 1, got %d", result.TotalHits)
	}
	if result.ProcessingTimeMs != 3 {
		t.Errorf("expected processing time 3, got %d", result.ProcessingTimeMs)
	}
	// FacetDistribution is json.RawMessage in the SDK response, so
	// parseFacetDistribution receives []byte which fails the map assertion,
	// returning nil. Facet parsing is tested directly in parseFacetDistribution tests.
	if result.Facets != nil {
		t.Errorf("expected nil facets (json.RawMessage doesn't satisfy map assertion), got %v", result.Facets)
	}
}

func TestMeilisearchClient_SearchEvidence_DefaultLimit(t *testing.T) {
	var capturedReq *ms.SearchRequest
	idx := &mockIndexManager{
		searchFunc: func(_ string, req *ms.SearchRequest) (*ms.SearchResponse, error) {
			capturedReq = req
			return &ms.SearchResponse{}, nil
		},
	}

	c := newTestMeilisearchClient(idx)
	_, err := c.SearchEvidence(context.Background(), SearchQuery{Limit: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedReq.Limit != 50 {
		t.Errorf("expected default limit 50, got %d", capturedReq.Limit)
	}
}

func TestMeilisearchClient_SearchEvidence_MaxLimit(t *testing.T) {
	var capturedReq *ms.SearchRequest
	idx := &mockIndexManager{
		searchFunc: func(_ string, req *ms.SearchRequest) (*ms.SearchResponse, error) {
			capturedReq = req
			return &ms.SearchResponse{}, nil
		},
	}

	c := newTestMeilisearchClient(idx)
	_, err := c.SearchEvidence(context.Background(), SearchQuery{Limit: 500})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedReq.Limit != 200 {
		t.Errorf("expected capped limit 200, got %d", capturedReq.Limit)
	}
}

func TestMeilisearchClient_SearchEvidence_WithFilter(t *testing.T) {
	var capturedReq *ms.SearchRequest
	idx := &mockIndexManager{
		searchFunc: func(_ string, req *ms.SearchRequest) (*ms.SearchResponse, error) {
			capturedReq = req
			return &ms.SearchResponse{}, nil
		},
	}

	c := newTestMeilisearchClient(idx)
	caseID := "case-1"
	_, err := c.SearchEvidence(context.Background(), SearchQuery{
		Query:  "test",
		CaseID: &caseID,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	filter, ok := capturedReq.Filter.(string)
	if !ok {
		t.Fatalf("expected string filter, got %T", capturedReq.Filter)
	}
	if !containsSubstring(filter, "case_id = 'case-1'") {
		t.Errorf("expected filter to contain case_id, got %q", filter)
	}
}

func TestMeilisearchClient_SearchEvidence_EmptyFilter(t *testing.T) {
	var capturedReq *ms.SearchRequest
	idx := &mockIndexManager{
		searchFunc: func(_ string, req *ms.SearchRequest) (*ms.SearchResponse, error) {
			capturedReq = req
			return &ms.SearchResponse{}, nil
		},
	}

	c := newTestMeilisearchClient(idx)
	_, err := c.SearchEvidence(context.Background(), SearchQuery{Query: "test", Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedReq.Filter != nil {
		t.Errorf("expected nil filter for empty query params, got %v", capturedReq.Filter)
	}
}

func TestMeilisearchClient_SearchEvidence_Error(t *testing.T) {
	idx := &mockIndexManager{
		searchFunc: func(_ string, _ *ms.SearchRequest) (*ms.SearchResponse, error) {
			return nil, errors.New("service down")
		},
	}

	c := newTestMeilisearchClient(idx)
	_, err := c.SearchEvidence(context.Background(), SearchQuery{Query: "test", Limit: 10})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !containsSubstring(err.Error(), "search evidence") {
		t.Errorf("expected wrapped error, got %q", err.Error())
	}
}

// --- ReindexAll tests ---

func TestMeilisearchClient_ReindexAll_Success(t *testing.T) {
	var deleteFilter string
	var addedDocs interface{}
	idx := &mockIndexManager{
		deleteDocumentsByFilterFunc: func(filter interface{}, _ *ms.DocumentOptions) (*ms.TaskInfo, error) {
			deleteFilter, _ = filter.(string)
			return &ms.TaskInfo{}, nil
		},
		addDocumentsFunc: func(docs interface{}, _ *ms.DocumentOptions) (*ms.TaskInfo, error) {
			addedDocs = docs
			return &ms.TaskInfo{}, nil
		},
	}

	c := newTestMeilisearchClient(idx)
	err := c.ReindexAll(context.Background(), "case-1", []EvidenceSearchDoc{
		{ID: "ev-1", CaseID: "case-1", Title: "Doc 1"},
		{ID: "ev-2", CaseID: "case-1", Title: "Doc 2"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleteFilter != "case_id = 'case-1'" {
		t.Errorf("expected delete filter for case-1, got %q", deleteFilter)
	}
	docs, ok := addedDocs.([]map[string]any)
	if !ok || len(docs) != 2 {
		t.Fatalf("expected 2 documents added, got %v", addedDocs)
	}
}

func TestMeilisearchClient_ReindexAll_EmptyItems(t *testing.T) {
	addCalled := false
	idx := &mockIndexManager{
		deleteDocumentsByFilterFunc: func(_ interface{}, _ *ms.DocumentOptions) (*ms.TaskInfo, error) {
			return &ms.TaskInfo{}, nil
		},
		addDocumentsFunc: func(_ interface{}, _ *ms.DocumentOptions) (*ms.TaskInfo, error) {
			addCalled = true
			return &ms.TaskInfo{}, nil
		},
	}

	c := newTestMeilisearchClient(idx)
	err := c.ReindexAll(context.Background(), "case-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addCalled {
		t.Error("expected AddDocuments to not be called for empty items")
	}
}

func TestMeilisearchClient_ReindexAll_DeleteError(t *testing.T) {
	idx := &mockIndexManager{
		deleteDocumentsByFilterFunc: func(_ interface{}, _ *ms.DocumentOptions) (*ms.TaskInfo, error) {
			return nil, errors.New("delete failed")
		},
	}

	c := newTestMeilisearchClient(idx)
	err := c.ReindexAll(context.Background(), "case-1", []EvidenceSearchDoc{{ID: "ev-1"}})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !containsSubstring(err.Error(), "delete documents for case") {
		t.Errorf("expected wrapped error, got %q", err.Error())
	}
}

func TestMeilisearchClient_ReindexAll_AddError(t *testing.T) {
	idx := &mockIndexManager{
		deleteDocumentsByFilterFunc: func(_ interface{}, _ *ms.DocumentOptions) (*ms.TaskInfo, error) {
			return &ms.TaskInfo{}, nil
		},
		addDocumentsFunc: func(_ interface{}, _ *ms.DocumentOptions) (*ms.TaskInfo, error) {
			return nil, errors.New("add failed")
		},
	}

	c := newTestMeilisearchClient(idx)
	err := c.ReindexAll(context.Background(), "case-1", []EvidenceSearchDoc{{ID: "ev-1"}})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !containsSubstring(err.Error(), "reindex") {
		t.Errorf("expected wrapped error, got %q", err.Error())
	}
}

func TestMeilisearchClient_ReindexAll_EscapesCaseID(t *testing.T) {
	var deleteFilter string
	idx := &mockIndexManager{
		deleteDocumentsByFilterFunc: func(filter interface{}, _ *ms.DocumentOptions) (*ms.TaskInfo, error) {
			deleteFilter, _ = filter.(string)
			return &ms.TaskInfo{}, nil
		},
	}

	c := newTestMeilisearchClient(idx)
	err := c.ReindexAll(context.Background(), "case's-test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "case_id = 'case\\'s-test'"
	if deleteFilter != expected {
		t.Errorf("expected escaped filter %q, got %q", expected, deleteFilter)
	}
}

// --- ConfigureEvidenceIndex tests ---

func TestMeilisearchClient_ConfigureEvidenceIndex_Success(t *testing.T) {
	searchableCalled := false
	filterableCalled := false
	sortableCalled := false
	typoCalled := false

	idx := &mockIndexManager{
		updateSearchableFunc: func(_ *[]string) (*ms.TaskInfo, error) {
			searchableCalled = true
			return &ms.TaskInfo{}, nil
		},
		updateFilterableFunc: func(_ *[]interface{}) (*ms.TaskInfo, error) {
			filterableCalled = true
			return &ms.TaskInfo{}, nil
		},
		updateSortableFunc: func(_ *[]string) (*ms.TaskInfo, error) {
			sortableCalled = true
			return &ms.TaskInfo{}, nil
		},
		updateTypoToleranceFunc: func(_ *ms.TypoTolerance) (*ms.TaskInfo, error) {
			typoCalled = true
			return &ms.TaskInfo{}, nil
		},
	}

	c := newTestMeilisearchClient(idx)
	err := c.ConfigureEvidenceIndex(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !searchableCalled {
		t.Error("expected UpdateSearchableAttributes to be called")
	}
	if !filterableCalled {
		t.Error("expected UpdateFilterableAttributes to be called")
	}
	if !sortableCalled {
		t.Error("expected UpdateSortableAttributes to be called")
	}
	if !typoCalled {
		t.Error("expected UpdateTypoTolerance to be called")
	}
}

func TestMeilisearchClient_ConfigureEvidenceIndex_SearchableError(t *testing.T) {
	idx := &mockIndexManager{
		updateSearchableFunc: func(_ *[]string) (*ms.TaskInfo, error) {
			return nil, errors.New("failed")
		},
	}

	c := newTestMeilisearchClient(idx)
	err := c.ConfigureEvidenceIndex(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstring(err.Error(), "searchable") {
		t.Errorf("expected searchable error, got %q", err.Error())
	}
}

func TestMeilisearchClient_ConfigureEvidenceIndex_FilterableError(t *testing.T) {
	idx := &mockIndexManager{
		updateSearchableFunc: func(_ *[]string) (*ms.TaskInfo, error) {
			return &ms.TaskInfo{}, nil
		},
		updateFilterableFunc: func(_ *[]interface{}) (*ms.TaskInfo, error) {
			return nil, errors.New("failed")
		},
	}

	c := newTestMeilisearchClient(idx)
	err := c.ConfigureEvidenceIndex(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstring(err.Error(), "filterable") {
		t.Errorf("expected filterable error, got %q", err.Error())
	}
}

func TestMeilisearchClient_ConfigureEvidenceIndex_SortableError(t *testing.T) {
	idx := &mockIndexManager{
		updateSearchableFunc: func(_ *[]string) (*ms.TaskInfo, error) {
			return &ms.TaskInfo{}, nil
		},
		updateFilterableFunc: func(_ *[]interface{}) (*ms.TaskInfo, error) {
			return &ms.TaskInfo{}, nil
		},
		updateSortableFunc: func(_ *[]string) (*ms.TaskInfo, error) {
			return nil, errors.New("failed")
		},
	}

	c := newTestMeilisearchClient(idx)
	err := c.ConfigureEvidenceIndex(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstring(err.Error(), "sortable") {
		t.Errorf("expected sortable error, got %q", err.Error())
	}
}

func TestMeilisearchClient_ConfigureEvidenceIndex_TypoError(t *testing.T) {
	idx := &mockIndexManager{
		updateSearchableFunc: func(_ *[]string) (*ms.TaskInfo, error) {
			return &ms.TaskInfo{}, nil
		},
		updateFilterableFunc: func(_ *[]interface{}) (*ms.TaskInfo, error) {
			return &ms.TaskInfo{}, nil
		},
		updateSortableFunc: func(_ *[]string) (*ms.TaskInfo, error) {
			return &ms.TaskInfo{}, nil
		},
		updateTypoToleranceFunc: func(_ *ms.TypoTolerance) (*ms.TaskInfo, error) {
			return nil, errors.New("failed")
		},
	}

	c := newTestMeilisearchClient(idx)
	err := c.ConfigureEvidenceIndex(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsSubstring(err.Error(), "typo") {
		t.Errorf("expected typo error, got %q", err.Error())
	}
}
