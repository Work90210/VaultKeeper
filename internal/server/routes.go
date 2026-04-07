package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// RouteRegistrar registers handler routes on a chi.Router.
type RouteRegistrar interface {
	RegisterRoutes(r chi.Router)
}

// RegisterRoutes wires public and protected routes onto the router.
// When a HealthHandler is provided it serves the public /health endpoint
// with live service checks; otherwise a static "healthy" response is returned.
func RegisterRoutes(r chi.Router, version string, health *HealthHandler, registrars ...RouteRegistrar) {
	if health != nil {
		r.Get("/health", health.PublicHealthCheck)
	} else {
		r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status":  "healthy",
				"version": version,
			})
		})
	}

	// Register the HealthHandler's protected routes (/api/health).
	if health != nil {
		health.RegisterRoutes(r)
	}

	for _, reg := range registrars {
		if reg != nil {
			reg.RegisterRoutes(r)
		}
	}
}
