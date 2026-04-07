package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/config"
)

// --- isOriginAllowed tests ---

func TestIsOriginAllowed(t *testing.T) {
	tests := []struct {
		name    string
		origin  string
		allowed map[string]struct{}
		want    bool
	}{
		{
			name:    "empty allowed set denies all",
			origin:  "https://example.com",
			allowed: map[string]struct{}{},
			want:    false,
		},
		{
			name:    "nil map denies all",
			origin:  "https://example.com",
			allowed: nil,
			want:    false,
		},
		{
			name:    "wildcard allows all",
			origin:  "https://anything.example.com",
			allowed: map[string]struct{}{"*": {}},
			want:    true,
		},
		{
			name:    "exact match allowed",
			origin:  "https://app.example.com",
			allowed: map[string]struct{}{"https://app.example.com": {}},
			want:    true,
		},
		{
			name:    "non-matching origin denied",
			origin:  "https://evil.com",
			allowed: map[string]struct{}{"https://app.example.com": {}},
			want:    false,
		},
		{
			name:   "multiple origins one matches",
			origin: "https://b.com",
			allowed: map[string]struct{}{
				"https://a.com": {},
				"https://b.com": {},
			},
			want: true,
		},
		{
			name:   "multiple origins none matches",
			origin: "https://c.com",
			allowed: map[string]struct{}{
				"https://a.com": {},
				"https://b.com": {},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isOriginAllowed(tt.origin, tt.allowed)
			if got != tt.want {
				t.Errorf("isOriginAllowed(%q, %v) = %v, want %v", tt.origin, tt.allowed, got, tt.want)
			}
		})
	}
}

// --- corsMiddleware tests ---

func TestCORSMiddleware_AllowedOrigin_SetsHeaders(t *testing.T) {
	handler := corsMiddleware(
		[]string{"https://app.example.com"},
		"",
	)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://app.example.com")
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Access-Control-Allow-Origin") != "https://app.example.com" {
		t.Errorf("expected ACAO header, got %q", rr.Header().Get("Access-Control-Allow-Origin"))
	}
	if rr.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Error("expected Access-Control-Allow-Credentials: true")
	}
	if rr.Header().Get("Vary") != "Origin" {
		t.Error("expected Vary: Origin")
	}
	if rr.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Error("expected Access-Control-Allow-Headers to be set")
	}
	if rr.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("expected Access-Control-Allow-Methods to be set")
	}
	if rr.Header().Get("Access-Control-Expose-Headers") == "" {
		t.Error("expected Access-Control-Expose-Headers to be set")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestCORSMiddleware_DisallowedOrigin_NoCORSHeaders(t *testing.T) {
	handler := corsMiddleware(
		[]string{"https://app.example.com"},
		"",
	)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://evil.com")
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("expected no ACAO header for disallowed origin, got %q", rr.Header().Get("Access-Control-Allow-Origin"))
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestCORSMiddleware_EmptyAllowedSet_DeniesAll(t *testing.T) {
	handler := corsMiddleware(
		nil,
		"",
	)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://anything.com")
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("expected no ACAO header when allowed set is empty, got %q", rr.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSMiddleware_Wildcard_AllowsAll(t *testing.T) {
	handler := corsMiddleware(
		[]string{"*"},
		"",
	)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://any-origin.example.org")
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Access-Control-Allow-Origin") != "https://any-origin.example.org" {
		t.Errorf("expected ACAO header with wildcard, got %q", rr.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSMiddleware_OptionsRequest_Returns204(t *testing.T) {
	nextCalled := false
	handler := corsMiddleware(
		[]string{"https://app.example.com"},
		"",
	)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "https://app.example.com")
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204 for OPTIONS, got %d", rr.Code)
	}
	if nextCalled {
		t.Error("next handler should not be called for OPTIONS")
	}
}

func TestCORSMiddleware_OptionsWithoutOrigin_Returns204(t *testing.T) {
	handler := corsMiddleware(
		[]string{"https://app.example.com"},
		"",
	)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204 for OPTIONS, got %d", rr.Code)
	}
	// No CORS headers when origin is not set.
	if rr.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("expected no ACAO header without Origin")
	}
}

func TestCORSMiddleware_NoOriginHeader_PassesThrough(t *testing.T) {
	nextCalled := false
	handler := corsMiddleware(
		[]string{"https://app.example.com"},
		"",
	)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	handler.ServeHTTP(rr, req)

	if !nextCalled {
		t.Error("next handler should be called when no Origin")
	}
	if rr.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("expected no ACAO header when no Origin header")
	}
}

func TestCORSMiddleware_AppURL_AddedToAllowed(t *testing.T) {
	handler := corsMiddleware(
		nil,
		"https://myapp.example.com",
	)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://myapp.example.com")
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Access-Control-Allow-Origin") != "https://myapp.example.com" {
		t.Errorf("expected appURL in allowed set, got ACAO %q", rr.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSMiddleware_AppURL_And_Origins_Combined(t *testing.T) {
	handler := corsMiddleware(
		[]string{"https://other.com"},
		"https://myapp.example.com",
	)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Test appURL origin
	rr1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.Header.Set("Origin", "https://myapp.example.com")
	handler.ServeHTTP(rr1, req1)

	if rr1.Header().Get("Access-Control-Allow-Origin") != "https://myapp.example.com" {
		t.Error("appURL origin should be allowed")
	}

	// Test explicit origin
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set("Origin", "https://other.com")
	handler.ServeHTTP(rr2, req2)

	if rr2.Header().Get("Access-Control-Allow-Origin") != "https://other.com" {
		t.Error("explicit origin should be allowed")
	}
}

// --- NewHTTPServer tests ---

func TestNewHTTPServer_CreatesServerWithCorrectAddress(t *testing.T) {
	cfg := config.Config{
		ServerPort:     9090,
		CORSOrigins:    []string{"https://app.example.com"},
		AppURL:         "https://app.example.com",
		KeycloakURL:    "http://localhost:8080",
		KeycloakRealm:  "test",
		KeycloakClientID: "test-client",
	}
	logger := slog.Default()
	jwks := auth.NewJWKSFetcher("http://localhost:8080", "test")

	srv := NewHTTPServer(cfg, logger, "1.0.0", jwks, nil, nil)

	if srv.Addr != ":9090" {
		t.Errorf("expected server Addr %q, got %q", ":9090", srv.Addr)
	}
	if srv.Handler == nil {
		t.Error("expected non-nil handler")
	}
	if srv.ReadHeaderTimeout == 0 {
		t.Error("expected ReadHeaderTimeout to be set")
	}
	if srv.ReadTimeout == 0 {
		t.Error("expected ReadTimeout to be set")
	}
	if srv.WriteTimeout == 0 {
		t.Error("expected WriteTimeout to be set")
	}
	if srv.IdleTimeout == 0 {
		t.Error("expected IdleTimeout to be set")
	}
}

func TestNewHTTPServer_DifferentPort(t *testing.T) {
	cfg := config.Config{
		ServerPort:     3000,
		KeycloakURL:    "http://localhost:8080",
		KeycloakRealm:  "test",
		KeycloakClientID: "test-client",
	}
	logger := slog.Default()
	jwks := auth.NewJWKSFetcher("http://localhost:8080", "test")

	srv := NewHTTPServer(cfg, logger, "2.0.0", jwks, nil, nil)

	if srv.Addr != ":3000" {
		t.Errorf("expected server Addr %q, got %q", ":3000", srv.Addr)
	}
}

func TestNewHTTPServer_WithRegistrars(t *testing.T) {
	cfg := config.Config{
		ServerPort:     8080,
		KeycloakURL:    "http://localhost:8080",
		KeycloakRealm:  "test",
		KeycloakClientID: "test-client",
	}
	logger := slog.Default()
	jwks := auth.NewJWKSFetcher("http://localhost:8080", "test")
	reg := &mockRegistrar{}

	srv := NewHTTPServer(cfg, logger, "1.0.0", jwks, nil, nil, reg)

	if srv == nil {
		t.Fatal("expected non-nil server")
	}
	if !reg.called {
		t.Error("expected registrar to be called")
	}
}
