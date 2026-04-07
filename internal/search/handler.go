package search

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/httputil"
)

// UserCaseIDsLoader retrieves the case IDs a user has access to.
type UserCaseIDsLoader interface {
	GetUserCaseIDs(ctx context.Context, userID string) ([]string, error)
}

// UserCaseRolesLoader retrieves the case-level roles for a user.
type UserCaseRolesLoader interface {
	GetUserCaseRoles(ctx context.Context, userID string) (map[string]string, error) // caseID -> role
}

// Handler provides HTTP endpoints for full-text search.
type Handler struct {
	searcher        EvidenceSearcher
	caseIDLoader    UserCaseIDsLoader
	caseRolesLoader UserCaseRolesLoader
	audit           auth.AuditLogger
}

// NewHandler creates a new search HTTP handler.
func NewHandler(searcher EvidenceSearcher, caseIDLoader UserCaseIDsLoader, caseRolesLoader UserCaseRolesLoader, audit auth.AuditLogger) *Handler {
	return &Handler{
		searcher:        searcher,
		caseIDLoader:    caseIDLoader,
		caseRolesLoader: caseRolesLoader,
		audit:           audit,
	}
}

// RegisterRoutes mounts search routes on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/search", h.Search)
}

// Search handles GET /api/search with query parameters for full-text evidence search.
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	query := parseSearchParams(r)

	// Apply role-based case filtering for non-admin users.
	if ac.SystemRole < auth.RoleSystemAdmin {
		caseIDs, err := h.caseIDLoader.GetUserCaseIDs(r.Context(), ac.UserID)
		if err != nil {
			httputil.RespondError(w, http.StatusInternalServerError, "failed to load user permissions")
			return
		}
		if len(caseIDs) == 0 {
			httputil.RespondJSON(w, http.StatusOK, EvidenceSearchResult{})
			return
		}
		query.UserCaseIDs = caseIDs

		// Defence users may only see disclosed evidence.
		caseRoles, err := h.caseRolesLoader.GetUserCaseRoles(r.Context(), ac.UserID)
		if err != nil {
			httputil.RespondError(w, http.StatusInternalServerError, "failed to load user permissions")
			return
		}
		for _, role := range caseRoles {
			if role == "defence" {
				query.DisclosedOnly = true
				break
			}
		}
	}

	result, err := h.searcher.SearchEvidence(r.Context(), query)
	if err != nil {
		httputil.RespondError(w, http.StatusServiceUnavailable, "search service unavailable")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, result)
}

// parseSearchParams extracts and validates search parameters from the request.
func parseSearchParams(r *http.Request) SearchQuery {
	q := r.URL.Query()

	limit := 50
	if v := q.Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 200 {
		limit = 200
	}

	offset := 0
	if v := q.Get("offset"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	query := SearchQuery{
		Query:  q.Get("q"),
		Limit:  limit,
		Offset: offset,
	}

	if caseID := q.Get("case_id"); caseID != "" {
		query.CaseID = &caseID
	}

	if types := q.Get("type"); types != "" {
		query.MimeTypes = splitAndTrim(types)
	}

	if tags := q.Get("tag"); tags != "" {
		query.Tags = splitAndTrim(tags)
	}

	if classifications := q.Get("classification"); classifications != "" {
		query.Classifications = splitAndTrim(classifications)
	}

	if from := q.Get("from"); from != "" {
		query.DateFrom = &from
	}

	if to := q.Get("to"); to != "" {
		query.DateTo = &to
	}

	return query
}

// splitAndTrim splits a comma-separated string and trims whitespace from each element.
func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
