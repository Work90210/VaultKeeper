package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type RouteRegistrar interface {
	RegisterRoutes(r chi.Router)
}

func RegisterRoutes(r chi.Router, version string, registrars ...RouteRegistrar) {
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "healthy",
			"version": version,
		})
	})

	for _, reg := range registrars {
		if reg != nil {
			reg.RegisterRoutes(r)
		}
	}
}
