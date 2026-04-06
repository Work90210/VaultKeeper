package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type testJWTBuilder struct {
	key *rsa.PrivateKey
	kid string
}

func newTestJWTBuilder(t *testing.T) *testJWTBuilder {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return &testJWTBuilder{key: key, kid: "test-kid"}
}

func (b *testJWTBuilder) buildJWT(header, payload map[string]any) string {
	headerJSON, _ := json.Marshal(header)
	payloadJSON, _ := json.Marshal(payload)

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

	signedContent := headerB64 + "." + payloadB64
	hash := sha256.Sum256([]byte(signedContent))

	signature, err := rsa.SignPKCS1v15(rand.Reader, b.key, crypto.SHA256, hash[:])
	if err != nil {
		panic(fmt.Sprintf("sign JWT: %v", err))
	}

	sigB64 := base64.RawURLEncoding.EncodeToString(signature)
	return headerB64 + "." + payloadB64 + "." + sigB64
}

func (b *testJWTBuilder) validToken(issuer, audience string) string {
	header := map[string]any{
		"alg": "RS256",
		"kid": b.kid,
		"typ": "JWT",
	}
	payload := map[string]any{
		"sub":                "user-123",
		"email":              "test@example.com",
		"preferred_username": "testuser",
		"exp":                time.Now().Add(15 * time.Minute).Unix(),
		"iss":                issuer,
		"aud":                audience,
		"sid":                "session-456",
		"realm_access": map[string]any{
			"roles": []string{"case_admin", "user"},
		},
	}
	return b.buildJWT(header, payload)
}

func (b *testJWTBuilder) expiredToken(issuer, audience string) string {
	header := map[string]any{
		"alg": "RS256",
		"kid": b.kid,
		"typ": "JWT",
	}
	payload := map[string]any{
		"sub":                "user-123",
		"email":              "test@example.com",
		"preferred_username": "testuser",
		"exp":                time.Now().Add(-5 * time.Minute).Unix(),
		"iss":                issuer,
		"aud":                audience,
		"sid":                "session-456",
		"realm_access": map[string]any{
			"roles": []string{"user"},
		},
	}
	return b.buildJWT(header, payload)
}

func (b *testJWTBuilder) serveJWKS() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		jwks := map[string]any{
			"keys": []map[string]string{
				{
					"kty": "RSA",
					"use": "sig",
					"kid": b.kid,
					"alg": "RS256",
					"n":   base64.RawURLEncoding.EncodeToString(b.key.PublicKey.N.Bytes()),
					"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(b.key.PublicKey.E)).Bytes()),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))
}

func setupMiddleware(t *testing.T, builder *testJWTBuilder) (*Middleware, *httptest.Server) {
	t.Helper()
	jwksSrv := builder.serveJWKS()
	t.Cleanup(jwksSrv.Close)

	fetcher := NewJWKSFetcher(jwksSrv.URL, "test-realm")
	fetcher.endpoint = jwksSrv.URL

	if err := fetcher.Prefetch(context.Background()); err != nil {
		t.Fatalf("prefetch JWKS: %v", err)
	}

	logger := slog.Default()
	mw := NewMiddleware(fetcher, jwksSrv.URL, "test-realm", "test-client", logger, nil)
	mw.issuer = jwksSrv.URL + "/realms/test-realm"

	return mw, jwksSrv
}

func TestMiddleware_ValidJWT(t *testing.T) {
	builder := newTestJWTBuilder(t)
	mw, jwksSrv := setupMiddleware(t, builder)

	issuer := jwksSrv.URL + "/realms/test-realm"
	token := builder.validToken(issuer, "test-client")

	var gotAuth AuthContext
	handler := mw.Authenticate(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		ac, ok := GetAuthContext(r.Context())
		if !ok {
			t.Error("expected AuthContext in request context")
			return
		}
		gotAuth = ac
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/cases", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.RemoteAddr = "192.168.1.1:1234"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if gotAuth.UserID != "user-123" {
		t.Errorf("UserID = %q, want %q", gotAuth.UserID, "user-123")
	}
	if gotAuth.Email != "test@example.com" {
		t.Errorf("Email = %q, want %q", gotAuth.Email, "test@example.com")
	}
	if gotAuth.Username != "testuser" {
		t.Errorf("Username = %q, want %q", gotAuth.Username, "testuser")
	}
	if gotAuth.SystemRole != RoleCaseAdmin {
		t.Errorf("SystemRole = %v, want %v", gotAuth.SystemRole, RoleCaseAdmin)
	}
	if gotAuth.IPAddress != "192.168.1.1" {
		t.Errorf("IPAddress = %q, want %q", gotAuth.IPAddress, "192.168.1.1")
	}
}

func TestMiddleware_ExpiredJWT(t *testing.T) {
	builder := newTestJWTBuilder(t)
	mw, jwksSrv := setupMiddleware(t, builder)

	issuer := jwksSrv.URL + "/realms/test-realm"
	token := builder.expiredToken(issuer, "test-client")

	handler := mw.Authenticate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("handler should not be called for expired token")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/cases", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
	assertResponseError(t, rr, "invalid or expired token")
}

func TestMiddleware_WrongSignature(t *testing.T) {
	builder := newTestJWTBuilder(t)
	mw, jwksSrv := setupMiddleware(t, builder)

	// Create a token signed with a different key
	otherBuilder := newTestJWTBuilder(t)
	otherBuilder.kid = builder.kid // Same kid but different key
	issuer := jwksSrv.URL + "/realms/test-realm"
	token := otherBuilder.validToken(issuer, "test-client")

	handler := mw.Authenticate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("handler should not be called for wrong signature")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/cases", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestMiddleware_WrongAudience(t *testing.T) {
	builder := newTestJWTBuilder(t)
	mw, jwksSrv := setupMiddleware(t, builder)

	issuer := jwksSrv.URL + "/realms/test-realm"
	token := builder.validToken(issuer, "wrong-client")

	handler := mw.Authenticate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("handler should not be called for wrong audience")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/cases", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestMiddleware_WrongIssuer(t *testing.T) {
	builder := newTestJWTBuilder(t)
	mw, _ := setupMiddleware(t, builder)

	token := builder.validToken("https://wrong-issuer/realms/test", "test-client")

	handler := mw.Authenticate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("handler should not be called for wrong issuer")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/cases", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestMiddleware_MissingAuthHeader(t *testing.T) {
	builder := newTestJWTBuilder(t)
	mw, _ := setupMiddleware(t, builder)

	handler := mw.Authenticate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("handler should not be called without auth header")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/cases", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
	assertResponseError(t, rr, "authentication required")
}

func TestMiddleware_MalformedAuthHeader(t *testing.T) {
	builder := newTestJWTBuilder(t)
	mw, _ := setupMiddleware(t, builder)

	handler := mw.Authenticate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("handler should not be called with malformed header")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/cases", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
	assertResponseError(t, rr, "invalid or expired token")
}

func TestMiddleware_HealthBypassesAuth(t *testing.T) {
	builder := newTestJWTBuilder(t)
	mw, _ := setupMiddleware(t, builder)

	called := false
	handler := mw.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("handler should be called for /health without auth")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestMiddleware_OPTIONSBypassesAuth(t *testing.T) {
	builder := newTestJWTBuilder(t)
	mw, _ := setupMiddleware(t, builder)

	called := false
	handler := mw.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/api/cases", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("handler should be called for OPTIONS without auth")
	}
}

func TestMiddleware_SystemRoleExtraction(t *testing.T) {
	builder := newTestJWTBuilder(t)
	mw, jwksSrv := setupMiddleware(t, builder)
	issuer := jwksSrv.URL + "/realms/test-realm"

	validTests := []struct {
		name     string
		roles    []string
		wantRole SystemRole
	}{
		{"system_admin", []string{"system_admin", "user"}, RoleSystemAdmin},
		{"case_admin only", []string{"case_admin"}, RoleCaseAdmin},
		{"user only", []string{"user"}, RoleUser},
		{"api_service", []string{"api_service"}, RoleAPIService},
	}

	for _, tt := range validTests {
		t.Run(tt.name, func(t *testing.T) {
			header := map[string]any{"alg": "RS256", "kid": builder.kid, "typ": "JWT"}
			payload := map[string]any{
				"sub":                "user-123",
				"email":              "test@example.com",
				"preferred_username": "testuser",
				"exp":                time.Now().Add(15 * time.Minute).Unix(),
				"iss":                issuer,
				"aud":                "test-client",
				"realm_access":       map[string]any{"roles": tt.roles},
			}
			token := builder.buildJWT(header, payload)

			var gotRole SystemRole
			handler := mw.Authenticate(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				ac, _ := GetAuthContext(r.Context())
				gotRole = ac.SystemRole
			}))

			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if gotRole != tt.wantRole {
				t.Errorf("SystemRole = %v, want %v", gotRole, tt.wantRole)
			}
		})
	}

	// Tokens with no recognized role should be rejected
	rejectedTests := []struct {
		name  string
		roles []string
	}{
		{"unknown role rejected", []string{"unknown_role"}},
		{"empty roles rejected", []string{}},
	}

	for _, tt := range rejectedTests {
		t.Run(tt.name, func(t *testing.T) {
			header := map[string]any{"alg": "RS256", "kid": builder.kid, "typ": "JWT"}
			payload := map[string]any{
				"sub":                "user-123",
				"email":              "test@example.com",
				"preferred_username": "testuser",
				"exp":                time.Now().Add(15 * time.Minute).Unix(),
				"iss":                issuer,
				"aud":                "test-client",
				"realm_access":       map[string]any{"roles": tt.roles},
			}
			token := builder.buildJWT(header, payload)

			handler := mw.Authenticate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Error("handler should not be called for token without recognized role")
			}))

			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestMiddleware_AudienceArray(t *testing.T) {
	builder := newTestJWTBuilder(t)
	mw, jwksSrv := setupMiddleware(t, builder)
	issuer := jwksSrv.URL + "/realms/test-realm"

	header := map[string]any{"alg": "RS256", "kid": builder.kid, "typ": "JWT"}
	payload := map[string]any{
		"sub":                "user-123",
		"email":              "test@example.com",
		"preferred_username": "testuser",
		"exp":                time.Now().Add(15 * time.Minute).Unix(),
		"iss":                issuer,
		"aud":                []string{"other-client", "test-client"},
		"realm_access":       map[string]any{"roles": []string{"user"}},
	}
	token := builder.buildJWT(header, payload)

	called := false
	handler := mw.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("handler should be called for valid audience in array")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestMiddleware_ValidBase64ButInvalidJSONHeader(t *testing.T) {
	builder := newTestJWTBuilder(t)
	mw, _ := setupMiddleware(t, builder)

	// Valid base64 but not valid JSON in header
	badHeader := base64.RawURLEncoding.EncodeToString([]byte("not json"))
	payload := base64.RawURLEncoding.EncodeToString([]byte("{}"))
	token := badHeader + "." + payload + ".fakesig"

	handler := mw.Authenticate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestMiddleware_InvalidBase64Signature(t *testing.T) {
	builder := newTestJWTBuilder(t)
	mw, jwksSrv := setupMiddleware(t, builder)
	issuer := jwksSrv.URL + "/realms/test-realm"

	header := map[string]any{"alg": "RS256", "kid": builder.kid, "typ": "JWT"}
	headerJSON, _ := json.Marshal(header)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	payload := map[string]any{
		"sub": "user-123", "email": "test@example.com", "preferred_username": "testuser",
		"exp": time.Now().Add(15 * time.Minute).Unix(), "iss": issuer, "aud": "test-client",
		"realm_access": map[string]any{"roles": []string{"user"}},
	}
	payloadJSON, _ := json.Marshal(payload)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

	// Invalid base64 in signature position
	token := headerB64 + "." + payloadB64 + ".!!!not-base64!!!"

	handler := mw.Authenticate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestMiddleware_InvalidBase64Payload(t *testing.T) {
	builder := newTestJWTBuilder(t)
	mw, _ := setupMiddleware(t, builder)

	// Craft a token where payload is invalid base64 but signature is valid over header.payload
	header := map[string]any{"alg": "RS256", "kid": builder.kid, "typ": "JWT"}
	headerJSON, _ := json.Marshal(header)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	invalidPayload := "!!!not-base64!!!"
	signedContent := headerB64 + "." + invalidPayload
	hash := sha256.Sum256([]byte(signedContent))
	sig, _ := rsa.SignPKCS1v15(rand.Reader, builder.key, crypto.SHA256, hash[:])
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)

	token := headerB64 + "." + invalidPayload + "." + sigB64

	handler := mw.Authenticate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestMiddleware_MissingExpClaim(t *testing.T) {
	builder := newTestJWTBuilder(t)
	mw, jwksSrv := setupMiddleware(t, builder)
	issuer := jwksSrv.URL + "/realms/test-realm"

	header := map[string]any{"alg": "RS256", "kid": builder.kid, "typ": "JWT"}
	payload := map[string]any{
		"sub": "user-123", "email": "test@example.com", "preferred_username": "testuser",
		"iss": issuer, "aud": "test-client",
		"realm_access": map[string]any{"roles": []string{"user"}},
		// exp is intentionally missing
	}
	token := builder.buildJWT(header, payload)

	handler := mw.Authenticate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("should not be called for missing exp")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestMiddleware_SignatureFailAndForceRefreshFails(t *testing.T) {
	// Use a valid key for JWKS, but sign with a different key
	// Then make JWKS endpoint fail on ForceRefresh
	goodKey, kid := generateTestKey(t)

	callCount := 0
	jwksSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		if callCount > 1 {
			// ForceRefresh call — return error
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		jwks := map[string]any{
			"keys": []map[string]string{{
				"kty": "RSA", "use": "sig", "kid": kid, "alg": "RS256",
				"n":   base64.RawURLEncoding.EncodeToString(goodKey.PublicKey.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(goodKey.PublicKey.E)).Bytes()),
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	defer jwksSrv.Close()

	fetcher := NewJWKSFetcher(jwksSrv.URL, "test-realm")
	fetcher.endpoint = jwksSrv.URL
	if err := fetcher.Prefetch(context.Background()); err != nil {
		t.Fatal(err)
	}

	logger := slog.Default()
	mw := NewMiddleware(fetcher, jwksSrv.URL, "test-realm", "test-client", logger, nil)
	mw.issuer = jwksSrv.URL + "/realms/test-realm"

	// Sign with a DIFFERENT key (wrong signature)
	badBuilder := newTestJWTBuilder(t)
	badBuilder.kid = kid // same kid but different private key
	token := badBuilder.validToken(mw.issuer, "test-client")

	handler := mw.Authenticate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Signature verification fails, ForceRefresh also fails → 401 (not 502, because
	// the initial GetKey succeeded, only ForceRefresh fails)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}
}

func TestMiddleware_ValidBase64ButInvalidJSONPayload(t *testing.T) {
	builder := newTestJWTBuilder(t)
	mw, jwksSrv := setupMiddleware(t, builder)

	// Build a JWT with valid header and signature, but payload is valid base64 containing non-JSON
	header := map[string]any{"alg": "RS256", "kid": builder.kid, "typ": "JWT"}
	payload := map[string]any{"not": "valid"} // will be replaced
	_ = payload

	// Manually craft: valid header, non-JSON payload, valid signature
	headerJSON, _ := json.Marshal(header)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	payloadB64 := base64.RawURLEncoding.EncodeToString([]byte("not-json-at-all"))

	signedContent := headerB64 + "." + payloadB64
	hash := sha256.Sum256([]byte(signedContent))
	signature, _ := rsa.SignPKCS1v15(rand.Reader, builder.key, crypto.SHA256, hash[:])
	sigB64 := base64.RawURLEncoding.EncodeToString(signature)

	token := headerB64 + "." + payloadB64 + "." + sigB64
	_ = jwksSrv

	handlerFn := mw.Authenticate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handlerFn.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestMiddleware_MalformedJWT(t *testing.T) {
	builder := newTestJWTBuilder(t)
	mw, _ := setupMiddleware(t, builder)

	handler := mw.Authenticate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("handler should not be called for malformed JWT")
	}))

	tokens := []string{
		"not.a.jwt.at.all",
		"onlyonepart",
		"two.parts",
		"!!!.???.$$$",
	}

	for _, token := range tokens {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("token %q: status = %d, want %d", token, rr.Code, http.StatusUnauthorized)
		}
	}
}

func TestMiddleware_JWKSUnavailable_Returns502(t *testing.T) {
	// Middleware with no cached keys and unreachable endpoint
	fetcher := NewJWKSFetcher("http://localhost:1", "test-realm")
	fetcher.endpoint = "http://localhost:1"
	fetcher.httpClient.Timeout = 100 * time.Millisecond

	logger := slog.Default()
	mw := NewMiddleware(fetcher, "http://localhost:1", "test-realm", "test-client", logger, nil)

	builder := newTestJWTBuilder(t)
	token := builder.validToken("http://localhost:1/realms/test-realm", "test-client")

	handler := mw.Authenticate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("handler should not be called when JWKS unavailable")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadGateway)
	}
	assertResponseError(t, rr, "authentication service unavailable")
}

func TestMiddleware_NbfFutureToken(t *testing.T) {
	builder := newTestJWTBuilder(t)
	mw, jwksSrv := setupMiddleware(t, builder)
	issuer := jwksSrv.URL + "/realms/test-realm"

	header := map[string]any{"alg": "RS256", "kid": builder.kid, "typ": "JWT"}
	payload := map[string]any{
		"sub":                "user-123",
		"email":              "test@example.com",
		"preferred_username": "testuser",
		"exp":                time.Now().Add(15 * time.Minute).Unix(),
		"nbf":                time.Now().Add(10 * time.Minute).Unix(), // not valid yet
		"iss":                issuer,
		"aud":                "test-client",
		"realm_access":       map[string]any{"roles": []string{"user"}},
	}
	token := builder.buildJWT(header, payload)

	handler := mw.Authenticate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("handler should not be called for nbf-future token")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestMiddleware_NbfPastToken(t *testing.T) {
	builder := newTestJWTBuilder(t)
	mw, jwksSrv := setupMiddleware(t, builder)
	issuer := jwksSrv.URL + "/realms/test-realm"

	header := map[string]any{"alg": "RS256", "kid": builder.kid, "typ": "JWT"}
	payload := map[string]any{
		"sub":                "user-123",
		"email":              "test@example.com",
		"preferred_username": "testuser",
		"exp":                time.Now().Add(15 * time.Minute).Unix(),
		"nbf":                time.Now().Add(-5 * time.Minute).Unix(), // already valid
		"iss":                issuer,
		"aud":                "test-client",
		"realm_access":       map[string]any{"roles": []string{"user"}},
	}
	token := builder.buildJWT(header, payload)

	called := false
	handler := mw.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("handler should be called for nbf-past token")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestMiddleware_UnsupportedAlgorithm(t *testing.T) {
	builder := newTestJWTBuilder(t)
	mw, jwksSrv := setupMiddleware(t, builder)
	issuer := jwksSrv.URL + "/realms/test-realm"

	header := map[string]any{"alg": "HS256", "kid": builder.kid, "typ": "JWT"}
	payload := map[string]any{
		"sub": "user-123", "email": "test@example.com", "preferred_username": "testuser",
		"exp": time.Now().Add(15 * time.Minute).Unix(), "iss": issuer, "aud": "test-client",
		"realm_access": map[string]any{"roles": []string{"user"}},
	}
	token := builder.buildJWT(header, payload)

	handler := mw.Authenticate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("should not be called for HS256")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestMiddleware_AudienceNil(t *testing.T) {
	builder := newTestJWTBuilder(t)
	mw, jwksSrv := setupMiddleware(t, builder)
	issuer := jwksSrv.URL + "/realms/test-realm"

	header := map[string]any{"alg": "RS256", "kid": builder.kid, "typ": "JWT"}
	payload := map[string]any{
		"sub": "user-123", "email": "test@example.com", "preferred_username": "testuser",
		"exp": time.Now().Add(15 * time.Minute).Unix(), "iss": issuer,
		"realm_access": map[string]any{"roles": []string{"user"}},
	}
	// aud is missing entirely
	token := builder.buildJWT(header, payload)

	handler := mw.Authenticate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("should not be called for nil audience")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

type mockAuditLogger struct {
	events []string
}

func (m *mockAuditLogger) LogAccessDenied(_ context.Context, userID, endpoint, requiredRole, actualRole string, _ string) {
	m.events = append(m.events, fmt.Sprintf("%s:%s:%s->%s", userID, endpoint, actualRole, requiredRole))
}

func TestRequireSystemRole_AuditLogged(t *testing.T) {
	audit := &mockAuditLogger{}
	handler := RequireSystemRole(RoleCaseAdmin, audit)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := requestWithAuth(http.MethodPost, "/api/cases", AuthContext{
		UserID:     "user-123",
		SystemRole: RoleUser,
		IPAddress:  "1.2.3.4",
	})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}
	if len(audit.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(audit.events))
	}
	if audit.events[0] != "user-123:/api/cases:user->case_admin" {
		t.Errorf("audit event = %q", audit.events[0])
	}
}

func assertResponseError(t *testing.T, rr *httptest.ResponseRecorder, wantError string) {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response body: %v", err)
	}
	gotError, _ := body["error"].(string)
	if gotError != wantError {
		t.Errorf("error = %q, want %q", gotError, wantError)
	}
}
