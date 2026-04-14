package collaboration

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"nhooyr.io/websocket"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
	"github.com/vaultkeeper/vaultkeeper/internal/httputil"
)

const maxWSMessageSize = 1 * 1024 * 1024 // 1 MB

// TokenValidator validates JWT tokens for WebSocket connections.
type TokenValidator interface {
	ValidateToken(ctx context.Context, rawToken string) (auth.AuthContext, error)
}

// CaseRoleLoader checks whether a user has a role in a given case.
type CaseRoleLoader interface {
	LoadCaseRole(ctx context.Context, caseID, userID string) (auth.CaseRole, error)
}

// handlerDB is the narrow Postgres surface Handler needs for case-id
// lookups. Declared as an interface (satisfied by *pgxpool.Pool) so
// unit tests can inject a fake without a live database.
type handlerDB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// OrgMembershipChecker verifies whether a user belongs to a case's organization.
type OrgMembershipChecker interface {
	IsActiveMember(ctx context.Context, orgID uuid.UUID, userID string) (bool, error)
}

// Handler provides the WebSocket endpoint for collaborative redaction.
type Handler struct {
	hub            *Hub
	db             handlerDB
	validator      TokenValidator
	roleLoader     CaseRoleLoader
	audit          auth.AuditLogger
	logger         *slog.Logger
	allowedOrigins []string
	orgChecker     OrgMembershipChecker
	caseLookupOrg  func(ctx context.Context, caseID uuid.UUID) (uuid.UUID, error)
}

// NewHandler creates a new collaboration WebSocket handler.
func NewHandler(hub *Hub, db *pgxpool.Pool, validator TokenValidator, roleLoader CaseRoleLoader, audit auth.AuditLogger, logger *slog.Logger, allowedOrigins []string) *Handler {
	return &Handler{
		hub:            hub,
		db:             db,
		validator:      validator,
		roleLoader:     roleLoader,
		audit:          audit,
		logger:         logger,
		allowedOrigins: allowedOrigins,
	}
}

// SetOrgMembershipChecker wires the org membership checker. When set,
// collaboration access requires that the caller is a member of the case's organization.
func (h *Handler) SetOrgMembershipChecker(checker OrgMembershipChecker, caseLookup func(ctx context.Context, caseID uuid.UUID) (uuid.UUID, error)) {
	h.orgChecker = checker
	h.caseLookupOrg = caseLookup
}

// RegisterRoutes mounts the collaboration WebSocket endpoint.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/evidence/{id}/redact/collaborate", h.Collaborate)
}

// Collaborate upgrades an HTTP connection to WebSocket and joins the
// collaboration room for the given evidence item.
func (h *Handler) Collaborate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	evidenceID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "invalid evidence ID")
		return
	}

	// Authenticate via query param (browsers can't set WS headers)
	token := r.URL.Query().Get("token")
	if token == "" {
		httputil.RespondError(w, http.StatusUnauthorized, "missing token")
		return
	}

	ac, err := h.validator.ValidateToken(ctx, token)
	if err != nil {
		httputil.RespondError(w, http.StatusUnauthorized, "invalid or expired token")
		return
	}

	// Look up case for this evidence
	caseID, err := h.lookupCaseID(ctx, evidenceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httputil.RespondError(w, http.StatusNotFound, "evidence not found")
			return
		}
		h.logger.Error("lookup case ID for collaboration failed",
			"evidence_id", evidenceID, "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Org membership gate: verify the caller belongs to the case's org.
	if ac.SystemRole < auth.RoleSystemAdmin && h.orgChecker != nil && h.caseLookupOrg != nil {
		orgID, orgErr := h.caseLookupOrg(ctx, caseID)
		if orgErr != nil {
			h.logger.Error("lookup org for case failed", "case_id", caseID, "error", orgErr)
			httputil.RespondError(w, http.StatusInternalServerError, "internal error")
			return
		}
		isMember, memberErr := h.orgChecker.IsActiveMember(ctx, orgID, ac.UserID)
		if memberErr != nil {
			h.logger.Error("org membership check failed", "org_id", orgID, "error", memberErr)
			httputil.RespondError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if !isMember {
			if h.audit != nil {
				h.audit.LogAccessDenied(ctx, ac.UserID, "/api/evidence/"+evidenceID.String()+"/redact/collaborate", "org_member", "non_member", auth.GetClientIP(r))
			}
			httputil.RespondError(w, http.StatusForbidden, "insufficient permissions")
			return
		}
	}

	// Authorize — system admins bypass, others need a case role
	if ac.SystemRole < auth.RoleSystemAdmin {
		_, roleErr := h.roleLoader.LoadCaseRole(ctx, caseID.String(), ac.UserID)
		if roleErr != nil {
			if errors.Is(roleErr, auth.ErrNoCaseRole) {
				if h.audit != nil {
					h.audit.LogAccessDenied(ctx, ac.UserID, "/api/evidence/"+evidenceID.String()+"/redact/collaborate", "case_member", "none", auth.GetClientIP(r))
				}
				httputil.RespondError(w, http.StatusForbidden, "insufficient permissions")
				return
			}
			h.logger.Error("case role check failed", "evidence_id", evidenceID, "error", roleErr)
			httputil.RespondError(w, http.StatusInternalServerError, "authorization check failed")
			return
		}
	}

	// Determine origin patterns for WebSocket
	originPatterns := h.allowedOrigins
	if len(originPatterns) == 0 {
		originPatterns = []string{"localhost:*"}
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns:  originPatterns,
		CompressionMode: websocket.CompressionDisabled,
	})
	if err != nil {
		// unreachable: in tests — requires a real browser sending a mismatched Origin
		// or a network-level upgrade failure that cannot be induced via httptest.
		h.logger.Error("accept WebSocket failed",
			"evidence_id", evidenceID, "error", err)
		return
	}

	// Set read limit to prevent oversized messages
	conn.SetReadLimit(maxWSMessageSize)

	room, err := h.hub.GetOrCreateRoom(ctx, evidenceID, caseID, ac.UserID)
	if err != nil {
		// unreachable: in tests — requires DraftStore.LoadDraft to fail after the
		// WebSocket upgrade has already succeeded. Tested via unit test
		// TestHub_GetOrCreateRoom_LoadError_ReturnsError before the upgrade.
		h.logger.Error("create collaboration room failed",
			"evidence_id", evidenceID, "error", err)
		conn.Close(websocket.StatusInternalError, "room initialization failed")
		return
	}

	client := &Client{
		User: ac,
		Conn: conn,
		Send: make(chan []byte, 64),
	}
	room.AddClient(client)
	defer room.RemoveClient(client)

	errCh := make(chan error, 2)
	go h.readPump(ctx, room, client, errCh)
	go h.writePump(ctx, client, errCh)

	// Wait for either pump to finish; the other exits via channel close or ctx
	pumpErr := <-errCh
	if pumpErr != nil &&
		websocket.CloseStatus(pumpErr) != websocket.StatusNormalClosure &&
		websocket.CloseStatus(pumpErr) != websocket.StatusGoingAway {
		h.logger.Debug("collaboration session ended",
			"evidence_id", evidenceID, "user_id", ac.UserID, "error", pumpErr)
	}

	conn.Close(websocket.StatusNormalClosure, "")
}

func (h *Handler) readPump(ctx context.Context, room *Room, client *Client, errCh chan<- error) {
	for {
		readCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		msgType, payload, err := client.Conn.Read(readCtx)
		cancel()
		if err != nil {
			errCh <- err
			return
		}
		if msgType != websocket.MessageBinary {
			continue
		}
		if err := room.HandleMessage(ctx, client, payload); err != nil {
			// unreachable: in practice — HandleMessage only errors on empty payload,
			// which nhooyr/websocket's Read never produces for binary frames.
			errCh <- err
			return
		}
	}
}

func (h *Handler) writePump(ctx context.Context, client *Client, errCh chan<- error) {
	for {
		select {
		case <-ctx.Done():
			errCh <- ctx.Err()
			return
		case payload, ok := <-client.Send:
			if !ok {
				errCh <- nil
				return
			}
			writeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			err := client.Conn.Write(writeCtx, websocket.MessageBinary, payload)
			cancel()
			if err != nil {
				// unreachable: in unit tests — requires the WebSocket write to fail
				// mid-session (e.g., broken pipe). Covered indirectly by integration
				// tests that close the connection from the client side.
				errCh <- err
				return
			}
		}
	}
}

func (h *Handler) lookupCaseID(ctx context.Context, evidenceID uuid.UUID) (uuid.UUID, error) {
	var caseID uuid.UUID
	err := h.db.QueryRow(ctx,
		`SELECT case_id FROM evidence_items WHERE id = $1`,
		evidenceID,
	).Scan(&caseID)
	if err != nil {
		return uuid.Nil, err
	}
	return caseID, nil
}
