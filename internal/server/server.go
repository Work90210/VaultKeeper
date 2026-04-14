package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/config"
	"github.com/vaultkeeper/vaultkeeper/internal/logging"
)

func NewHTTPServer(cfg config.Config, logger *slog.Logger, version string, jwks *auth.JWKSFetcher, audit auth.AuditLogger, health *HealthHandler, registrars ...RouteRegistrar) *http.Server {
	router := chi.NewRouter()
	router.Use(chimiddleware.RequestID)
	router.Use(logging.Middleware(logger))
	router.Use(corsMiddleware(cfg.CORSOrigins, cfg.AppURL))
	router.Use(chimiddleware.Recoverer)

	authMiddleware := auth.NewMiddleware(jwks, cfg.KeycloakURL, cfg.KeycloakRealm, cfg.KeycloakClientID, logger, audit)
	router.Use(authMiddleware.Authenticate)

	RegisterRoutes(router, version, health, registrars...)

	return &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.ServerPort),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

func corsMiddleware(origins []string, appURL string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(origins)+1)
	for _, origin := range origins {
		allowed[origin] = struct{}{}
	}
	if appURL != "" {
		allowed[appURL] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && isOriginAllowed(origin, allowed) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Headers", strings.Join([]string{
					"Accept",
					"Authorization",
					"Content-Type",
					"Content-Disposition",
					"X-Organization-ID",
					"X-Request-ID",
					"X-Content-SHA256",
					"Upload-Length",
					"Upload-Offset",
					"Tus-Resumable",
					"Upgrade",
					"Connection",
					"Sec-WebSocket-Key",
					"Sec-WebSocket-Version",
					"Sec-WebSocket-Protocol",
				}, ", "))
				w.Header().Set("Access-Control-Expose-Headers", strings.Join([]string{
					"Content-Disposition",
					"Content-Length",
					"X-Request-ID",
				}, ", "))
				w.Header().Set("Access-Control-Allow-Methods", strings.Join([]string{
					http.MethodGet,
					http.MethodPost,
					http.MethodPut,
					http.MethodPatch,
					http.MethodDelete,
					http.MethodOptions,
				}, ", "))
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func isOriginAllowed(origin string, allowed map[string]struct{}) bool {
	if len(allowed) == 0 {
		return false // deny all when no origins are configured
	}
	if _, ok := allowed["*"]; ok {
		return true
	}
	_, ok := allowed[origin]
	return ok
}
