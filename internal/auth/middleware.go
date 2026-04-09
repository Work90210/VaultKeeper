package auth

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type jwtHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	Typ string `json:"typ"`
}

type jwtPayload struct {
	Sub              string      `json:"sub"`
	Email            string      `json:"email"`
	PreferredUsername string      `json:"preferred_username"`
	Exp              float64     `json:"exp"`
	Nbf              float64     `json:"nbf"`
	Iss              string      `json:"iss"`
	Aud              any         `json:"aud"`
	Sid              string      `json:"sid"`
	RealmAccess      realmAccess `json:"realm_access"`
}

type realmAccess struct {
	Roles []string `json:"roles"`
}

type AuditLogger interface {
	LogAccessDenied(ctx context.Context, userID, endpoint, requiredRole, actualRole string, ip string)
}

type Middleware struct {
	jwks     *JWKSFetcher
	issuer   string
	audience string
	logger   *slog.Logger
	audit    AuditLogger
}

func NewMiddleware(jwks *JWKSFetcher, keycloakURL, realm, clientID string, logger *slog.Logger, audit AuditLogger) *Middleware {
	return &Middleware{
		jwks:     jwks,
		issuer:   fmt.Sprintf("%s/realms/%s", keycloakURL, realm),
		audience: clientID,
		logger:   logger,
		audit:    audit,
	}
}

func (m *Middleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for health check, CORS preflight, and WebSocket upgrade requests
		// (WebSocket endpoints do their own token auth via query param)
		isWSUpgrade := strings.EqualFold(r.Header.Get("Upgrade"), "websocket") && strings.HasSuffix(r.URL.Path, "/redact/collaborate")
		if r.URL.Path == "/health" || r.Method == http.MethodOptions || isWSUpgrade {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			respondError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			respondError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}
		rawToken := authHeader[7:]

		ac, err := m.validateAndExtract(r.Context(), rawToken)
		if err != nil {
			m.logger.Debug("token validation failed", "error", err)
			if isUpstreamError(err) {
				respondError(w, http.StatusBadGateway, "authentication service unavailable")
				return
			}
			respondError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		ac.IPAddress = GetClientIP(r)
		ctx := WithAuthContext(r.Context(), ac)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *Middleware) validateAndExtract(ctx context.Context, rawToken string) (AuthContext, error) {
	parts := strings.Split(rawToken, ".")
	if len(parts) != 3 {
		return AuthContext{}, fmt.Errorf("malformed JWT: expected 3 parts, got %d", len(parts))
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return AuthContext{}, fmt.Errorf("decode JWT header: %w", err)
	}

	var header jwtHeader
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return AuthContext{}, fmt.Errorf("parse JWT header: %w", err)
	}

	if header.Alg != "RS256" {
		return AuthContext{}, fmt.Errorf("unsupported algorithm: %s", header.Alg)
	}

	// Verify signature
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return AuthContext{}, fmt.Errorf("decode JWT signature: %w", err)
	}

	signedContent := parts[0] + "." + parts[1]
	hash := sha256.Sum256([]byte(signedContent))

	pubKey, err := m.jwks.GetKey(ctx, header.Kid)
	if err != nil {
		return AuthContext{}, fmt.Errorf("get signing key: %w", err)
	}

	err = rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, hash[:], signature)
	if err != nil {
		// Signature failed — try force refresh (key rotation scenario)
		pubKey, err = m.jwks.ForceRefresh(ctx, header.Kid)
		if err != nil {
			return AuthContext{}, fmt.Errorf("signature verification failed and key refresh failed: %w", err)
		}
		if err := rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, hash[:], signature); err != nil {
			return AuthContext{}, fmt.Errorf("signature verification failed after key refresh: %w", err)
		}
	}

	// Parse payload
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return AuthContext{}, fmt.Errorf("decode JWT payload: %w", err)
	}

	var payload jwtPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return AuthContext{}, fmt.Errorf("parse JWT payload: %w", err)
	}

	// Validate issuer
	if payload.Iss != m.issuer {
		return AuthContext{}, fmt.Errorf("invalid issuer: got %q, want %q", payload.Iss, m.issuer)
	}

	// Validate audience
	if !m.audienceMatches(payload.Aud) {
		return AuthContext{}, fmt.Errorf("invalid audience")
	}

	// Validate expiry
	if payload.Exp == 0 {
		return AuthContext{}, fmt.Errorf("missing exp claim")
	}
	expTime := time.Unix(int64(payload.Exp), 0)
	if time.Now().After(expTime) {
		return AuthContext{}, fmt.Errorf("token expired at %s", expTime)
	}

	// Validate not-before (nbf) if present
	if payload.Nbf > 0 {
		nbfTime := time.Unix(int64(payload.Nbf), 0)
		if time.Now().Before(nbfTime) {
			return AuthContext{}, fmt.Errorf("token not yet valid until %s", nbfTime)
		}
	}

	// Extract system role from realm_access.roles
	systemRole, hasRole := extractSystemRole(payload.RealmAccess.Roles)
	if !hasRole {
		return AuthContext{}, fmt.Errorf("token has no recognized VaultKeeper role")
	}

	return AuthContext{
		UserID:      payload.Sub,
		Email:       payload.Email,
		Username:    payload.PreferredUsername,
		SystemRole:  systemRole,
		TokenExpiry: int64(payload.Exp),
		SessionID:   payload.Sid,
	}, nil
}

func (m *Middleware) audienceMatches(aud any) bool {
	switch v := aud.(type) {
	case string:
		return v == m.audience
	case []any:
		for _, a := range v {
			if s, ok := a.(string); ok && s == m.audience {
				return true
			}
		}
	}
	return false
}

func extractSystemRole(roles []string) (SystemRole, bool) {
	highest := RoleNone
	for _, r := range roles {
		parsed, ok := ParseSystemRole(r)
		if ok && parsed > highest {
			highest = parsed
		}
	}
	if highest == RoleNone {
		return RoleNone, false
	}
	return highest, true
}

// ValidateToken validates a raw JWT string and returns the auth context.
// Use this for WebSocket connections where the token is passed via query param.
func (m *Middleware) ValidateToken(ctx context.Context, rawToken string) (AuthContext, error) {
	return m.validateAndExtract(ctx, rawToken)
}

func isUpstreamError(err error) bool {
	return err != nil && errors.Is(err, ErrJWKSUnavailable)
}

