package search

import (
	"context"
	"testing"
)

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
