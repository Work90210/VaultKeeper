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

// UserOrgIDsLoader retrieves the org IDs a user is a member of.
type UserOrgIDsLoader interface {
	GetUserOrgIDs(ctx context.Context, userID string) ([]string, error)
}

// Handler provides HTTP endpoints for full-text search.
type Handler struct {
	searcher        EvidenceSearcher
	caseIDLoader    UserCaseIDsLoader
	caseRolesLoader UserCaseRolesLoader
	orgIDLoader     UserOrgIDsLoader
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

// SetOrgIDLoader wires the org ID loader for org-scoped search.
func (h *Handler) SetOrgIDLoader(l UserOrgIDsLoader) { h.orgIDLoader = l }

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
	// Org scoping is handled implicitly: GetUserCaseIDs already joins
	// organization_memberships, so the case ID list is org-aware.
	// A MeiliSearch organization_id filter is not applied here because
	// existing indexed documents may lack that field (pre-org migration).
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

	// Sprint 9: enforce the classification access matrix as a post-filter
	// on every search result. The filter is keyed on the per-case role
	// loader so defence users never see prosecution ex_parte items even
	// when they happen to match the full-text query. System admins skip
	// this filter for cross-case support workflows.
	if ac.SystemRole < auth.RoleSystemAdmin && h.caseRolesLoader != nil {
		caseRoles, rerr := h.caseRolesLoader.GetUserCaseRoles(r.Context(), ac.UserID)
		if rerr == nil {
			result = filterHitsByAccess(result, caseRoles)
		}
	}

	httputil.RespondJSON(w, http.StatusOK, result)
}

// userSideForRole mirrors evidence.UserSideForRole without importing the
// evidence package (search has no evidence dependency today and we'd
// rather not introduce one for a 4-line helper).
func userSideForRole(role string) string {
	switch role {
	case "investigator", "prosecutor":
		return "prosecution"
	case "defence":
		return "defence"
	}
	return ""
}

// checkAccess mirrors evidence.CheckAccess. Keeping a local copy avoids
// an import cycle while the access matrix is small and stable. If the
// matrix changes, update both places. Coverage: classification_test.go
// in the evidence package asserts the canonical truth table.
func checkAccess(role, classification string, exParteSide *string, userSide string) bool {
	switch classification {
	case "public", "restricted":
		return isKnownRole(role)
	case "confidential":
		return role == "investigator" || role == "prosecutor" || role == "judge"
	case "ex_parte":
		if exParteSide == nil {
			return false
		}
		if role == "judge" {
			return *exParteSide == "prosecution" || *exParteSide == "defence"
		}
		if *exParteSide == "prosecution" {
			return role == "investigator" || role == "prosecutor"
		}
		if *exParteSide == "defence" {
			return role == "defence"
		}
	}
	return false
}

func isKnownRole(role string) bool {
	switch role {
	case "investigator", "prosecutor", "defence", "judge", "observer", "victim_representative":
		return true
	}
	return false
}

// filterHitsByAccess removes any search hit the caller's case role cannot
// see per the classification matrix. Items with an unknown case (caller
// has no role mapping) are dropped to be safe.
func filterHitsByAccess(result EvidenceSearchResult, caseRoles map[string]string) EvidenceSearchResult {
	filtered := make([]EvidenceSearchHit, 0, len(result.Hits))
	for _, hit := range result.Hits {
		role, ok := caseRoles[hit.CaseID]
		if !ok {
			continue
		}
		class := hit.Classification
		if class == "" {
			class = "restricted"
		}
		if checkAccess(role, class, hit.ExParteSide, userSideForRole(role)) {
			filtered = append(filtered, hit)
		}
	}
	return EvidenceSearchResult{
		Hits:             filtered,
		TotalHits:        len(filtered),
		Query:            result.Query,
		ProcessingTimeMs: result.ProcessingTimeMs,
		Facets:           result.Facets,
	}
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
