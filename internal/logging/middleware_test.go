package logging

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func TestMiddleware_SkipsHealthLogging(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger("development", slog.LevelInfo, &buffer)

	router := chi.NewRouter()
	router.Use(chimiddleware.RequestID)
	router.Use(Middleware(logger))
	router.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if buffer.Len() != 0 {
		t.Fatalf("expected no logs for /health, got %s", buffer.String())
	}
}

func TestMiddleware_LogsRequestFields(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger("development", slog.LevelInfo, &buffer)

	router := chi.NewRouter()
	router.Use(chimiddleware.RequestID)
	router.Use(Middleware(logger))
	router.Get("/cases", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	req := httptest.NewRequest(http.MethodGet, "/cases?access_token=super-secret&case_reference=CASE-2026", nil)
	req.Header.Set("Authorization", "Bearer top-secret")
	req.Header.Set("X-API-Key", "header-secret")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	logOutput := buffer.String()

	if !strings.Contains(logOutput, "request.started") {
		t.Fatal("expected request.started log")
	}
	if !strings.Contains(logOutput, "request.completed") {
		t.Fatal("expected request.completed log")
	}
	if !strings.Contains(logOutput, "request_id=") {
		t.Fatal("expected request_id in log output")
	}
	if strings.Contains(logOutput, "super-secret") || strings.Contains(logOutput, "top-secret") || strings.Contains(logOutput, "header-secret") {
		t.Fatal("expected secrets to be redacted in logs")
	}
	if !strings.Contains(logOutput, redactedValue) {
		t.Fatal("expected redacted marker in logs")
	}
	if !strings.Contains(logOutput, "status_code=201") {
		t.Fatal("expected response status in logs")
	}
}

func TestNewLogger_ProductionUsesJSON(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger("production", slog.LevelInfo, &buffer)
	logger.Info("test message")

	if !strings.Contains(buffer.String(), `"msg"`) {
		t.Fatal("expected JSON formatted log in production")
	}
}

func TestNewLogger_DevelopmentUsesText(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger("development", slog.LevelInfo, &buffer)
	logger.Info("test message")

	if strings.Contains(buffer.String(), `"msg"`) {
		t.Fatal("expected text formatted log in development")
	}
}
