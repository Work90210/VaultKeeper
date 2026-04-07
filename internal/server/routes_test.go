package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// mockRegistrar tracks whether RegisterRoutes was called.
type mockRegistrar struct {
	called bool
}

func (m *mockRegistrar) RegisterRoutes(_ chi.Router) {
	m.called = true
}

func TestRegisterRoutes_HealthEndpointAccessible(t *testing.T) {
	r := chi.NewRouter()
	RegisterRoutes(r, "1.0.0", nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var env struct {
		Data  map[string]string `json:"data"`
		Error any               `json:"error"`
	}
	// The static fallback returns a plain JSON object (not wrapped in httputil envelope).
	var raw map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &raw); err != nil {
		// Try envelope format
		if err2 := json.Unmarshal(rr.Body.Bytes(), &env); err2 != nil {
			t.Fatalf("unmarshal response: %v / %v", err, err2)
		}
		raw = env.Data
	}

	if raw["status"] != "healthy" {
		t.Errorf("expected status %q, got %q", "healthy", raw["status"])
	}
	if raw["version"] != "1.0.0" {
		t.Errorf("expected version %q, got %q", "1.0.0", raw["version"])
	}
}

func TestRegisterRoutes_WithHealthHandler(t *testing.T) {
	searchSrv := healthySearchServer(t)
	keycloakSrv := healthyKeycloakServer(t)

	h := newHealthHandler(t,
		&mockDatabaseChecker{err: nil},
		&mockStorageChecker{exists: true, err: nil},
		searchSrv, keycloakSrv,
	)

	r := chi.NewRouter()
	RegisterRoutes(r, "1.0.0", h)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// Should be the live PublicHealth via httputil envelope.
	ph := decodePublicHealth(t, rr.Body.Bytes())
	if ph.Status != statusHealthy {
		t.Errorf("expected %q, got %q", statusHealthy, ph.Status)
	}
}

func TestRegisterRoutes_RegistrarsAreCalled(t *testing.T) {
	r := chi.NewRouter()
	reg1 := &mockRegistrar{}
	reg2 := &mockRegistrar{}

	RegisterRoutes(r, "1.0.0", nil, reg1, reg2)

	if !reg1.called {
		t.Error("registrar 1 was not called")
	}
	if !reg2.called {
		t.Error("registrar 2 was not called")
	}
}

func TestRegisterRoutes_NilRegistrarSkipped(t *testing.T) {
	r := chi.NewRouter()
	reg := &mockRegistrar{}

	// Should not panic with nil registrar in the list.
	RegisterRoutes(r, "1.0.0", nil, nil, reg)

	if !reg.called {
		t.Error("non-nil registrar was not called")
	}
}

func TestRegisterRoutes_StaticFallback_ContentType(t *testing.T) {
	r := chi.NewRouter()
	RegisterRoutes(r, "2.0.0", nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(rr, req)

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}
