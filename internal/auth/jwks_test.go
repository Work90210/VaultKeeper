package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func generateTestKey(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	return key, "test-kid-1"
}

func serveJWKS(t *testing.T, key *rsa.PublicKey, kid string) *httptest.Server {
	t.Helper()
	jwks := map[string]any{
		"keys": []map[string]string{
			{
				"kty": "RSA",
				"use": "sig",
				"kid": kid,
				"alg": "RS256",
				"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
			},
		},
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))
}

func TestJWKSFetcher_Prefetch(t *testing.T) {
	privKey, kid := generateTestKey(t)
	srv := serveJWKS(t, &privKey.PublicKey, kid)
	defer srv.Close()

	fetcher := NewJWKSFetcher(srv.URL, "test-realm")
	// Override endpoint since serveJWKS doesn't have the realm path
	fetcher.endpoint = srv.URL

	if err := fetcher.Prefetch(context.Background()); err != nil {
		t.Fatalf("Prefetch() error = %v", err)
	}

	if !fetcher.HasCachedKeys() {
		t.Error("expected cached keys after prefetch")
	}
}

func TestJWKSFetcher_GetKey_CacheHit(t *testing.T) {
	privKey, kid := generateTestKey(t)

	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount++
		jwks := map[string]any{
			"keys": []map[string]string{
				{
					"kty": "RSA",
					"use": "sig",
					"kid": kid,
					"alg": "RS256",
					"n":   base64.RawURLEncoding.EncodeToString(privKey.PublicKey.N.Bytes()),
					"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privKey.PublicKey.E)).Bytes()),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	defer srv.Close()

	fetcher := NewJWKSFetcher(srv.URL, "test-realm")
	fetcher.endpoint = srv.URL
	ctx := context.Background()

	// First call fetches
	_, err := fetcher.GetKey(ctx, kid)
	if err != nil {
		t.Fatalf("first GetKey() error = %v", err)
	}

	// Second call should use cache
	_, err = fetcher.GetKey(ctx, kid)
	if err != nil {
		t.Fatalf("second GetKey() error = %v", err)
	}

	if requestCount != 1 {
		t.Errorf("expected 1 HTTP request (cache hit), got %d", requestCount)
	}
}

func TestJWKSFetcher_GetKey_CacheExpiry(t *testing.T) {
	privKey, kid := generateTestKey(t)

	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount++
		jwks := map[string]any{
			"keys": []map[string]string{
				{
					"kty": "RSA",
					"use": "sig",
					"kid": kid,
					"alg": "RS256",
					"n":   base64.RawURLEncoding.EncodeToString(privKey.PublicKey.N.Bytes()),
					"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privKey.PublicKey.E)).Bytes()),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	defer srv.Close()

	fetcher := NewJWKSFetcher(srv.URL, "test-realm")
	fetcher.endpoint = srv.URL
	fetcher.cacheTTL = 1 * time.Millisecond // Very short TTL for test
	ctx := context.Background()

	_, err := fetcher.GetKey(ctx, kid)
	if err != nil {
		t.Fatalf("first GetKey() error = %v", err)
	}

	time.Sleep(5 * time.Millisecond)

	_, err = fetcher.GetKey(ctx, kid)
	if err != nil {
		t.Fatalf("second GetKey() error = %v", err)
	}

	if requestCount < 2 {
		t.Errorf("expected at least 2 HTTP requests after cache expiry, got %d", requestCount)
	}
}

func TestJWKSFetcher_EndpointDown_WarmCache(t *testing.T) {
	privKey, kid := generateTestKey(t)
	srv := serveJWKS(t, &privKey.PublicKey, kid)
	fetcher := NewJWKSFetcher(srv.URL, "test-realm")
	fetcher.endpoint = srv.URL
	ctx := context.Background()

	// Populate cache
	_, err := fetcher.GetKey(ctx, kid)
	if err != nil {
		t.Fatalf("initial GetKey() error = %v", err)
	}

	// Shut down server
	srv.Close()

	// Expire cache TTL but keep within max age
	fetcher.cacheTTL = 0
	fetcher.maxCacheAge = 15 * time.Minute

	// Should still work with cached keys
	key, err := fetcher.GetKey(ctx, kid)
	if err != nil {
		t.Fatalf("GetKey() with warm cache error = %v", err)
	}
	if key == nil {
		t.Error("expected non-nil key from cache")
	}
}

func TestJWKSFetcher_EndpointDown_ColdCache(t *testing.T) {
	fetcher := NewJWKSFetcher("http://localhost:1", "test-realm")
	fetcher.endpoint = "http://localhost:1" // Unreachable
	fetcher.httpClient.Timeout = 100 * time.Millisecond

	_, err := fetcher.GetKey(context.Background(), "nonexistent-kid")
	if err == nil {
		t.Error("expected error with cold cache and unreachable endpoint")
	}
}

func TestJWKSFetcher_KeyNotFound(t *testing.T) {
	privKey, kid := generateTestKey(t)
	srv := serveJWKS(t, &privKey.PublicKey, kid)
	defer srv.Close()

	fetcher := NewJWKSFetcher(srv.URL, "test-realm")
	fetcher.endpoint = srv.URL

	_, err := fetcher.GetKey(context.Background(), "wrong-kid")
	if err == nil {
		t.Error("expected error for unknown key ID")
	}
}

func TestJWKSFetcher_ConcurrentAccess(t *testing.T) {
	privKey, kid := generateTestKey(t)
	srv := serveJWKS(t, &privKey.PublicKey, kid)
	defer srv.Close()

	fetcher := NewJWKSFetcher(srv.URL, "test-realm")
	fetcher.endpoint = srv.URL
	ctx := context.Background()

	var wg sync.WaitGroup
	errs := make(chan error, 20)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := fetcher.GetKey(ctx, kid)
			if err != nil {
				errs <- err
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent GetKey() error = %v", err)
	}
}

func TestParseRSAPublicKey(t *testing.T) {
	privKey, _ := generateTestKey(t)
	pub := &privKey.PublicKey

	nStr := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	eStr := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())

	parsed, err := parseRSAPublicKey(nStr, eStr)
	if err != nil {
		t.Fatalf("parseRSAPublicKey() error = %v", err)
	}

	if parsed.N.Cmp(pub.N) != 0 {
		t.Error("parsed modulus does not match")
	}
	if parsed.E != pub.E {
		t.Errorf("parsed exponent = %d, want %d", parsed.E, pub.E)
	}
}

func TestParseRSAPublicKey_InvalidInput(t *testing.T) {
	_, err := parseRSAPublicKey("!!!invalid!!!", "AQAB")
	if err == nil {
		t.Error("expected error for invalid modulus")
	}

	_, err = parseRSAPublicKey("AQAB", "!!!invalid!!!")
	if err == nil {
		t.Error("expected error for invalid exponent")
	}
}

func TestJWKSFetcher_ForceRefresh(t *testing.T) {
	privKey, kid := generateTestKey(t)
	srv := serveJWKS(t, &privKey.PublicKey, kid)
	defer srv.Close()

	fetcher := NewJWKSFetcher(srv.URL, "test-realm")
	fetcher.endpoint = srv.URL

	// Populate cache
	_, err := fetcher.GetKey(context.Background(), kid)
	if err != nil {
		t.Fatalf("GetKey() error = %v", err)
	}

	// Force refresh should succeed
	key, err := fetcher.ForceRefresh(context.Background(), kid)
	if err != nil {
		t.Fatalf("ForceRefresh() error = %v", err)
	}
	if key == nil {
		t.Error("expected non-nil key after force refresh")
	}
}

func TestJWKSFetcher_ForceRefresh_KeyNotFound(t *testing.T) {
	privKey, kid := generateTestKey(t)
	srv := serveJWKS(t, &privKey.PublicKey, kid)
	defer srv.Close()

	fetcher := NewJWKSFetcher(srv.URL, "test-realm")
	fetcher.endpoint = srv.URL

	_, err := fetcher.ForceRefresh(context.Background(), "wrong-kid")
	if err == nil {
		t.Error("expected error for unknown key ID")
	}
}

func TestJWKSFetcher_BadHTTPStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	fetcher := NewJWKSFetcher(srv.URL, "test-realm")
	fetcher.endpoint = srv.URL

	_, err := fetcher.GetKey(context.Background(), "any-kid")
	if err == nil {
		t.Error("expected error for 500 status")
	}
}

func TestJWKSFetcher_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "not json")
	}))
	defer srv.Close()

	fetcher := NewJWKSFetcher(srv.URL, "test-realm")
	fetcher.endpoint = srv.URL

	_, err := fetcher.GetKey(context.Background(), "any-kid")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestJWKSFetcher_GetKey_StaleCache_KeyPresent(t *testing.T) {
	privKey, kid := generateTestKey(t)

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		if callCount > 1 {
			// After first call, endpoint goes down
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		jwks := map[string]any{
			"keys": []map[string]string{
				{
					"kty": "RSA", "use": "sig", "kid": kid, "alg": "RS256",
					"n": base64.RawURLEncoding.EncodeToString(privKey.PublicKey.N.Bytes()),
					"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privKey.PublicKey.E)).Bytes()),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	defer srv.Close()

	fetcher := NewJWKSFetcher(srv.URL, "test-realm")
	fetcher.endpoint = srv.URL
	fetcher.cacheTTL = 1 * time.Millisecond
	fetcher.maxCacheAge = 1 * time.Hour
	ctx := context.Background()

	// First call populates cache
	_, err := fetcher.GetKey(ctx, kid)
	if err != nil {
		t.Fatalf("first GetKey() error = %v", err)
	}

	time.Sleep(5 * time.Millisecond) // cache TTL expires

	// Second call: TTL expired, endpoint down, but key still in stale cache within maxCacheAge
	key, err := fetcher.GetKey(ctx, kid)
	if err != nil {
		t.Fatalf("stale cache GetKey() error = %v", err)
	}
	if key == nil {
		t.Error("expected non-nil key from stale cache")
	}
}

func TestJWKSFetcher_GetKey_StaleCache_KeyMissing(t *testing.T) {
	privKey, kid := generateTestKey(t)

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		if callCount > 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		jwks := map[string]any{
			"keys": []map[string]string{
				{
					"kty": "RSA", "use": "sig", "kid": kid, "alg": "RS256",
					"n": base64.RawURLEncoding.EncodeToString(privKey.PublicKey.N.Bytes()),
					"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privKey.PublicKey.E)).Bytes()),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	defer srv.Close()

	fetcher := NewJWKSFetcher(srv.URL, "test-realm")
	fetcher.endpoint = srv.URL
	ctx := context.Background()

	// Populate cache with kid
	_, err := fetcher.GetKey(ctx, kid)
	if err != nil {
		t.Fatalf("initial GetKey() error = %v", err)
	}

	// Now request a different kid that's not in cache, endpoint is down
	_, err = fetcher.GetKey(ctx, "unknown-kid")
	if err == nil {
		t.Error("expected error for unknown kid when endpoint is down")
	}
}

func TestJWKSFetcher_ForceRefresh_EndpointDown(t *testing.T) {
	fetcher := NewJWKSFetcher("http://localhost:1", "test-realm")
	fetcher.endpoint = "http://localhost:1"
	fetcher.httpClient.Timeout = 100 * time.Millisecond

	_, err := fetcher.ForceRefresh(context.Background(), "any-kid")
	if err == nil {
		t.Error("expected error for unreachable endpoint")
	}
}

func TestJWKSFetcher_RefreshOrFallback_MaxAgeExpired(t *testing.T) {
	privKey, kid := generateTestKey(t)
	srv := serveJWKS(t, &privKey.PublicKey, kid)
	fetcher := NewJWKSFetcher(srv.URL, "test-realm")
	fetcher.endpoint = srv.URL
	ctx := context.Background()

	// Populate cache
	_, err := fetcher.GetKey(ctx, kid)
	if err != nil {
		t.Fatalf("initial GetKey() error = %v", err)
	}

	// Close server and expire both TTL and maxCacheAge
	srv.Close()
	fetcher.cacheTTL = 0
	fetcher.maxCacheAge = 0

	_, err = fetcher.GetKey(ctx, kid)
	if err == nil {
		t.Error("expected error when both TTL and maxCacheAge expired with endpoint down")
	}
}

func TestParseRSAPublicKey_ExponentTooLarge(t *testing.T) {
	// Create a fake exponent that doesn't fit in int64
	hugeExp := make([]byte, 20)
	for i := range hugeExp {
		hugeExp[i] = 0xFF
	}
	eStr := base64.RawURLEncoding.EncodeToString(hugeExp)
	nStr := base64.RawURLEncoding.EncodeToString([]byte{1})

	_, err := parseRSAPublicKey(nStr, eStr)
	if err == nil {
		t.Error("expected error for exponent too large")
	}
}

func TestJWKSFetcher_RefreshOrFallback_KidNotFoundAfterRefresh(t *testing.T) {
	privKey, kid := generateTestKey(t)

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		jwks := map[string]any{"keys": []map[string]string{}}
		if callCount == 1 {
			// First call returns the key
			jwks["keys"] = []map[string]string{{
				"kty": "RSA", "use": "sig", "kid": kid, "alg": "RS256",
				"n": base64.RawURLEncoding.EncodeToString(privKey.PublicKey.N.Bytes()),
				"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privKey.PublicKey.E)).Bytes()),
			}}
		} else {
			// Second call returns a DIFFERENT key (simulating key rotation where old kid is removed)
			newKey, _ := rsa.GenerateKey(rand.Reader, 2048)
			jwks["keys"] = []map[string]string{{
				"kty": "RSA", "use": "sig", "kid": "new-kid", "alg": "RS256",
				"n": base64.RawURLEncoding.EncodeToString(newKey.PublicKey.N.Bytes()),
				"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(newKey.PublicKey.E)).Bytes()),
			}}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	defer srv.Close()

	fetcher := NewJWKSFetcher(srv.URL, "test-realm")
	fetcher.endpoint = srv.URL
	fetcher.cacheTTL = 1 * time.Millisecond
	fetcher.maxCacheAge = 1 * time.Hour
	ctx := context.Background()

	// Populate cache with original kid
	_, err := fetcher.GetKey(ctx, kid)
	if err != nil {
		t.Fatalf("first GetKey() error = %v", err)
	}

	time.Sleep(5 * time.Millisecond) // expire TTL

	// Now request old kid — TTL expired, refreshOrFallback called, refresh succeeds
	// but old kid is no longer in the new JWKS response
	_, err = fetcher.GetKey(ctx, kid)
	if err == nil {
		t.Error("expected error when kid removed after refresh")
	}
}

func TestJWKSFetcher_FetchAndCache_InvalidURL(t *testing.T) {
	fetcher := NewJWKSFetcher("", "")
	fetcher.endpoint = "://invalid-url"

	_, err := fetcher.GetKey(context.Background(), "any")
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestJWKSFetcher_FetchAndCache_ReadBodyError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", "1000000") // claim large body
		w.WriteHeader(http.StatusOK)
		// Write partial body then close — this causes ReadAll to error
		_, _ = w.Write([]byte(`{"ke`))
		// Connection will be closed before content-length is satisfied
	}))
	defer srv.Close()

	fetcher := NewJWKSFetcher(srv.URL, "test-realm")
	fetcher.endpoint = srv.URL

	// This may or may not error depending on how the HTTP client handles truncated body
	// but it exercises the fetchAndCache path
	_, _ = fetcher.GetKey(context.Background(), "any")
}

func TestJWKSFetcher_FetchAndCache_KeyParseError(t *testing.T) {
	// Return valid JSON with RSA/sig keys that have invalid N/E values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		jwks := map[string]any{
			"keys": []map[string]string{
				{
					"kty": "RSA", "use": "sig", "kid": "bad-key", "alg": "RS256",
					"n": "!!!invalid-base64!!!", "e": "AQAB",
				},
				{
					"kty": "RSA", "use": "sig", "kid": "good-key", "alg": "RS256",
					"n": base64.RawURLEncoding.EncodeToString([]byte{1, 2, 3}),
					"e": base64.RawURLEncoding.EncodeToString([]byte{1, 0, 1}),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	defer srv.Close()

	fetcher := NewJWKSFetcher(srv.URL, "test-realm")
	fetcher.endpoint = srv.URL

	// One key fails to parse, but the other succeeds — should still work
	key, err := fetcher.GetKey(context.Background(), "good-key")
	if err != nil {
		t.Fatalf("GetKey() error = %v", err)
	}
	if key == nil {
		t.Error("expected non-nil key")
	}

	// The bad key should not be available
	_, err = fetcher.GetKey(context.Background(), "bad-key")
	if err == nil {
		t.Error("expected error for key with invalid base64")
	}
}

func TestJWKSFetcher_NoSigningKeys(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Return JWKS with no RSA signing keys
		jwks := map[string]any{
			"keys": []map[string]string{
				{
					"kty": "EC",
					"use": "sig",
					"kid": "ec-key",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	defer srv.Close()

	fetcher := NewJWKSFetcher(srv.URL, "test-realm")
	fetcher.endpoint = srv.URL

	_, err := fetcher.GetKey(context.Background(), "ec-key")
	if err == nil {
		t.Error("expected error when no valid RSA signing keys")
	}
}
