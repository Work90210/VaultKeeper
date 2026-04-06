package httputil

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestRespondJSON(t *testing.T) {
	rr := httptest.NewRecorder()

	data := map[string]string{"id": "123", "name": "test"}
	RespondJSON(rr, 200, data)

	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
	if rr.Code != 200 {
		t.Errorf("status = %d, want %d", rr.Code, 200)
	}

	var body envelope
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if body.Data == nil {
		t.Error("expected non-nil data")
	}
	if body.Error != nil {
		t.Errorf("expected nil error, got %v", body.Error)
	}
	if body.Meta != nil {
		t.Errorf("expected nil meta, got %v", body.Meta)
	}
}

func TestRespondJSON_Created(t *testing.T) {
	rr := httptest.NewRecorder()
	RespondJSON(rr, 201, map[string]string{"id": "new"})

	if rr.Code != 201 {
		t.Errorf("status = %d, want %d", rr.Code, 201)
	}
}

func TestRespondError(t *testing.T) {
	rr := httptest.NewRecorder()
	RespondError(rr, 400, "bad request")

	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
	if rr.Code != 400 {
		t.Errorf("status = %d, want %d", rr.Code, 400)
	}

	var body envelope
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if body.Data != nil {
		t.Errorf("expected nil data, got %v", body.Data)
	}
	errStr, ok := body.Error.(string)
	if !ok || errStr != "bad request" {
		t.Errorf("error = %v, want %q", body.Error, "bad request")
	}
}

func TestRespondError_InternalDoesNotExposeDetails(t *testing.T) {
	rr := httptest.NewRecorder()
	RespondError(rr, 500, "internal error")

	var body envelope
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	errStr, _ := body.Error.(string)
	if errStr != "internal error" {
		t.Errorf("error = %q, want generic message", errStr)
	}
}

func TestRespondError_StatusCodes(t *testing.T) {
	codes := []int{400, 401, 403, 404, 409, 413, 429, 500, 502, 503, 507}
	for _, code := range codes {
		rr := httptest.NewRecorder()
		RespondError(rr, code, "error")

		if rr.Code != code {
			t.Errorf("status = %d, want %d", rr.Code, code)
		}
	}
}

func TestRespondPaginated(t *testing.T) {
	rr := httptest.NewRecorder()
	data := []string{"item1", "item2"}
	RespondPaginated(rr, 200, data, 150, "cursor123", true)

	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
	if rr.Code != 200 {
		t.Errorf("status = %d, want %d", rr.Code, 200)
	}

	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if body["data"] == nil {
		t.Error("expected non-nil data")
	}
	if body["error"] != nil {
		t.Errorf("expected nil error, got %v", body["error"])
	}

	meta, ok := body["meta"].(map[string]any)
	if !ok {
		t.Fatal("expected meta to be an object")
	}

	if total, _ := meta["total"].(float64); total != 150 {
		t.Errorf("meta.total = %v, want 150", total)
	}
	if cursor, _ := meta["next_cursor"].(string); cursor != "cursor123" {
		t.Errorf("meta.next_cursor = %q, want %q", cursor, "cursor123")
	}
	if hasMore, _ := meta["has_more"].(bool); !hasMore {
		t.Error("meta.has_more = false, want true")
	}
}

func TestRespondPaginated_NoMore(t *testing.T) {
	rr := httptest.NewRecorder()
	RespondPaginated(rr, 200, []string{}, 0, "", false)

	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	meta, ok := body["meta"].(map[string]any)
	if !ok {
		t.Fatal("expected meta to be an object")
	}
	if hasMore, _ := meta["has_more"].(bool); hasMore {
		t.Error("meta.has_more = true, want false")
	}
}
