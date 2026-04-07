package logging

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// RedactValue — additional branches
// ---------------------------------------------------------------------------

func TestRedactValue_NonSensitiveString(t *testing.T) {
	result := RedactValue("status", "active")
	if result != "active" {
		t.Errorf("result = %v, want %q", result, "active")
	}
}

func TestRedactValue_SensitiveString(t *testing.T) {
	result := RedactValue("password", "secret123")
	if result != redactedValue {
		t.Errorf("result = %v, want %q", result, redactedValue)
	}
}

func TestRedactValue_NonSensitiveSliceAny(t *testing.T) {
	input := []any{"value1", "value2"}
	result := RedactValue("status", input)

	arr, ok := result.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", result)
	}
	if len(arr) != 2 {
		t.Fatalf("len = %d, want 2", len(arr))
	}
	if arr[0] != "value1" {
		t.Errorf("arr[0] = %v", arr[0])
	}
}

func TestRedactValue_SensitiveSliceString(t *testing.T) {
	input := []string{"val1", "val2"}
	result := RedactValue("api_key", input)

	if result != redactedValue {
		t.Errorf("result = %v, want %q", result, redactedValue)
	}
}

func TestRedactValue_NonSensitiveSliceString(t *testing.T) {
	input := []string{"val1", "val2"}
	result := RedactValue("status", input)

	arr, ok := result.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", result)
	}
	if len(arr) != 2 {
		t.Fatalf("len = %d, want 2", len(arr))
	}
}

func TestRedactValue_NestedMap(t *testing.T) {
	input := map[string]any{
		"password": "secret",
		"status":   "ok",
	}
	result := RedactValue("data", input)

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", result)
	}
	if m["password"] != redactedValue {
		t.Errorf("password = %v, want redacted", m["password"])
	}
	if m["status"] != "ok" {
		t.Errorf("status = %v, want ok", m["status"])
	}
}

func TestRedactValue_IntValue(t *testing.T) {
	result := RedactValue("count", 42)
	if result != 42 {
		t.Errorf("result = %v, want 42", result)
	}
}

func TestRedactValue_NilValue(t *testing.T) {
	result := RedactValue("field", nil)
	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}
}

func TestRedactValue_BoolValue(t *testing.T) {
	result := RedactValue("enabled", true)
	if result != true {
		t.Errorf("result = %v, want true", result)
	}
}

// ---------------------------------------------------------------------------
// IsSensitiveField — additional markers
// ---------------------------------------------------------------------------

func TestIsSensitiveField_Authorization(t *testing.T) {
	if !IsSensitiveField("Authorization") {
		t.Error("Authorization should be sensitive")
	}
}

func TestIsSensitiveField_WhitespaceAndCase(t *testing.T) {
	if !IsSensitiveField("  API_KEY  ") {
		t.Error("whitespace-padded API_KEY should be sensitive")
	}
}

func TestIsSensitiveField_PartialMatch(t *testing.T) {
	if !IsSensitiveField("x_api_key_header") {
		t.Error("x_api_key_header contains 'key' and should be sensitive")
	}
}

func TestIsSensitiveField_EmptyString(t *testing.T) {
	if IsSensitiveField("") {
		t.Error("empty string should not be sensitive")
	}
}

// ---------------------------------------------------------------------------
// Middleware — additional branches
// ---------------------------------------------------------------------------

func TestMiddleware_SetsRequestID(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger("development", slog.LevelInfo, &buffer)

	handler := Middleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/cases", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	rid := w.Header().Get("X-Request-ID")
	if rid == "" {
		t.Error("expected X-Request-ID header to be set")
	}
}

func TestMiddleware_ResponseRecorder_Write(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger("development", slog.LevelInfo, &buffer)

	handler := Middleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("hello world"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
	if !strings.Contains(buffer.String(), "bytes_written=11") {
		t.Errorf("expected bytes_written=11 in log, got: %s", buffer.String())
	}
}

func TestMiddleware_WithQueryParams(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger("development", slog.LevelInfo, &buffer)

	handler := Middleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/cases?status=active&page=1", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	logOutput := buffer.String()
	if !strings.Contains(logOutput, "request.started") {
		t.Error("expected request.started log")
	}
}

func TestMiddleware_WithMultiValueQuery(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger("development", slog.LevelInfo, &buffer)

	handler := Middleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/cases?tag=a&tag=b&tag=c", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !strings.Contains(buffer.String(), "request.started") {
		t.Error("expected request.started log")
	}
}

func TestMiddleware_WithMultiValueHeader(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger("development", slog.LevelInfo, &buffer)

	handler := Middleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/cases", nil)
	req.Header.Add("Accept", "text/html")
	req.Header.Add("Accept", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !strings.Contains(buffer.String(), "request.started") {
		t.Error("expected request.started log")
	}
}

func TestMiddleware_NoHeaders(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger("development", slog.LevelInfo, &buffer)

	handler := Middleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/cases", nil)
	// Clear all default headers
	req.Header = http.Header{}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !strings.Contains(buffer.String(), "request.started") {
		t.Error("expected request.started log")
	}
}

func TestMiddleware_NoQueryParams(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger("development", slog.LevelInfo, &buffer)

	handler := Middleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/cases", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !strings.Contains(buffer.String(), "request.completed") {
		t.Error("expected request.completed log")
	}
}

// ---------------------------------------------------------------------------
// queryFields and headerFields edge cases
// ---------------------------------------------------------------------------

func TestQueryFields_EmptyQuery(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/path", nil)
	result := queryFields(req)
	if result != nil {
		t.Errorf("expected nil for empty query, got %v", result)
	}
}

func TestQueryFields_SingleValues(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/path?a=1&b=2", nil)
	result := queryFields(req)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result["a"] != "1" {
		t.Errorf("a = %v", result["a"])
	}
}

func TestQueryFields_MultipleValues(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/path?a=1&a=2&a=3", nil)
	result := queryFields(req)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	arr, ok := result["a"].([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", result["a"])
	}
	if len(arr) != 3 {
		t.Errorf("len = %d, want 3", len(arr))
	}
}

func TestHeaderFields_Empty(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/path", nil)
	req.Header = http.Header{}
	result := headerFields(req)
	if result != nil {
		t.Errorf("expected nil for empty headers, got %v", result)
	}
}

func TestHeaderFields_SingleValue(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/path", nil)
	req.Header = http.Header{"X-Custom": {"value1"}}
	result := headerFields(req)
	if result == nil {
		t.Fatal("expected non-nil")
	}
	if result["X-Custom"] != "value1" {
		t.Errorf("X-Custom = %v", result["X-Custom"])
	}
}

func TestHeaderFields_MultipleValues(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/path", nil)
	req.Header = http.Header{"Accept": {"text/html", "application/json"}}
	result := headerFields(req)
	if result == nil {
		t.Fatal("expected non-nil")
	}
	arr, ok := result["Accept"].([]any)
	if !ok {
		t.Fatalf("expected []any for multi-value header, got %T", result["Accept"])
	}
	if len(arr) != 2 {
		t.Errorf("len = %d, want 2", len(arr))
	}
}

// ---------------------------------------------------------------------------
// responseRecorder tests
// ---------------------------------------------------------------------------

func TestResponseRecorder_DefaultStatus(t *testing.T) {
	inner := httptest.NewRecorder()
	rec := &responseRecorder{ResponseWriter: inner, statusCode: http.StatusOK}
	if rec.statusCode != http.StatusOK {
		t.Errorf("default statusCode = %d, want %d", rec.statusCode, http.StatusOK)
	}
}

func TestResponseRecorder_WriteHeader(t *testing.T) {
	inner := httptest.NewRecorder()
	rec := &responseRecorder{ResponseWriter: inner, statusCode: http.StatusOK}
	rec.WriteHeader(http.StatusNotFound)
	if rec.statusCode != http.StatusNotFound {
		t.Errorf("statusCode = %d, want %d", rec.statusCode, http.StatusNotFound)
	}
}

func TestResponseRecorder_Write(t *testing.T) {
	inner := httptest.NewRecorder()
	rec := &responseRecorder{ResponseWriter: inner, statusCode: http.StatusOK}

	n, err := rec.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 5 {
		t.Errorf("n = %d, want 5", n)
	}
	if rec.bytesWritten != 5 {
		t.Errorf("bytesWritten = %d, want 5", rec.bytesWritten)
	}

	n, err = rec.Write([]byte(" world"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.bytesWritten != 11 {
		t.Errorf("bytesWritten = %d, want 11", rec.bytesWritten)
	}
}

// ---------------------------------------------------------------------------
// RedactMap — edge cases
// ---------------------------------------------------------------------------

func TestRedactMap_EmptyMap(t *testing.T) {
	result := RedactMap(map[string]any{})
	if result == nil {
		t.Error("expected non-nil empty map")
	}
	if len(result) != 0 {
		t.Errorf("len = %d, want 0", len(result))
	}
}

func TestRedactMap_DeepNesting(t *testing.T) {
	input := map[string]any{
		"level1": map[string]any{
			"level2": map[string]any{
				"secret_key": "deep-secret",
				"visible":    "yes",
			},
		},
	}
	result := RedactMap(input)
	l1, _ := result["level1"].(map[string]any)
	l2, _ := l1["level2"].(map[string]any)
	if l2["secret_key"] != redactedValue {
		t.Errorf("deep secret_key should be redacted, got %v", l2["secret_key"])
	}
	if l2["visible"] != "yes" {
		t.Errorf("visible = %v, want yes", l2["visible"])
	}
}
