package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"
)

type jwkKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
	Alg string `json:"alg"`
}

type jwksResponse struct {
	Keys []jwkKey `json:"keys"`
}

var ErrJWKSUnavailable = fmt.Errorf("JWKS endpoint unavailable")

type cachedKeys struct {
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
}

type JWKSFetcher struct {
	endpoint    string
	cacheTTL    time.Duration
	maxCacheAge time.Duration
	httpClient  *http.Client

	mu    sync.RWMutex
	cache *cachedKeys
}

func NewJWKSFetcher(keycloakURL, realm string) *JWKSFetcher {
	return &JWKSFetcher{
		endpoint:    fmt.Sprintf("%s/realms/%s/protocol/openid-connect/certs", keycloakURL, realm),
		cacheTTL:    5 * time.Minute,
		maxCacheAge: 15 * time.Minute,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (f *JWKSFetcher) Prefetch(ctx context.Context) error {
	_, err := f.fetchAndCache(ctx)
	return err
}

func (f *JWKSFetcher) GetKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	// Try cache first
	f.mu.RLock()
	if f.cache != nil {
		if key, ok := f.cache.keys[kid]; ok {
			cacheAge := time.Since(f.cache.fetchedAt)
			if cacheAge < f.cacheTTL {
				f.mu.RUnlock()
				return key, nil
			}
			// Cache expired but still within max age — try refresh, fall back to cached
			f.mu.RUnlock()
			return f.refreshOrFallback(ctx, kid, key)
		}
		f.mu.RUnlock()
	} else {
		f.mu.RUnlock()
	}

	// Cache miss or key not found — fetch fresh
	keys, err := f.fetchAndCache(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrJWKSUnavailable, err)
	}

	key, ok := keys[kid]
	if !ok {
		return nil, fmt.Errorf("key ID %q not found in JWKS", kid)
	}
	return key, nil
}

func (f *JWKSFetcher) ForceRefresh(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	keys, err := f.fetchAndCache(ctx)
	if err != nil {
		return nil, fmt.Errorf("force refresh JWKS: %w", err)
	}
	key, ok := keys[kid]
	if !ok {
		return nil, fmt.Errorf("key ID %q not found after refresh", kid)
	}
	return key, nil
}

func (f *JWKSFetcher) HasCachedKeys() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.cache != nil && len(f.cache.keys) > 0
}

func (f *JWKSFetcher) refreshOrFallback(ctx context.Context, kid string, fallbackKey *rsa.PublicKey) (*rsa.PublicKey, error) {
	keys, err := f.fetchAndCache(ctx)
	if err != nil {
		// Fetch failed — use stale key if within max age
		f.mu.RLock()
		defer f.mu.RUnlock()
		if f.cache != nil && time.Since(f.cache.fetchedAt) < f.maxCacheAge {
			return fallbackKey, nil
		}
		return nil, fmt.Errorf("%w: cache expired and refresh failed: %w", ErrJWKSUnavailable, err)
	}

	key, ok := keys[kid]
	if !ok {
		return nil, fmt.Errorf("key ID %q not found after refresh", kid)
	}
	return key, nil
}

func (f *JWKSFetcher) fetchAndCache(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create JWKS request: %w", err)
	}

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("JWKS HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return nil, fmt.Errorf("read JWKS response: %w", err)
	}

	var jwks jwksResponse
	if err := json.Unmarshal(body, &jwks); err != nil {
		return nil, fmt.Errorf("parse JWKS response: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" || k.Use != "sig" {
			continue
		}
		pubKey, err := parseRSAPublicKey(k.N, k.E)
		if err != nil {
			continue
		}
		keys[k.Kid] = pubKey
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("no valid RSA signing keys in JWKS response")
	}

	f.mu.Lock()
	f.cache = &cachedKeys{
		keys:      keys,
		fetchedAt: time.Now(),
	}
	f.mu.Unlock()

	return keys, nil
}

func parseRSAPublicKey(nStr, eStr string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nStr)
	if err != nil {
		return nil, fmt.Errorf("decode modulus: %w", err)
	}

	eBytes, err := base64.RawURLEncoding.DecodeString(eStr)
	if err != nil {
		return nil, fmt.Errorf("decode exponent: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)
	if !e.IsInt64() {
		return nil, fmt.Errorf("exponent too large")
	}

	return &rsa.PublicKey{
		N: n,
		E: int(e.Int64()),
	}, nil
}
