package federation

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/vaultkeeper/vaultkeeper/internal/migration"
)

// WellKnownResponse is the JSON response for /.well-known/vaultkeeper-instance.json.
type WellKnownResponse struct {
	Protocol              string   `json:"protocol"`
	InstanceID            string   `json:"instance_id"`
	DisplayName           string   `json:"display_name"`
	Ed25519PublicKey      string   `json:"ed25519_public_key"`
	KeyFingerprint        string   `json:"key_fingerprint"`
	SupportedBundleVersions []string `json:"supported_bundle_versions"`
	FederationEndpoint    string   `json:"federation_endpoint"`
	KeyRotation           KeyRotationInfo `json:"key_rotation"`
}

// KeyRotationInfo holds key rotation metadata.
type KeyRotationInfo struct {
	PreviousKeyFingerprint *string `json:"previous_key_fingerprint"`
	RotationStatementURL   *string `json:"rotation_statement_url"`
}

// IdentityHandler serves the .well-known instance identity document.
type IdentityHandler struct {
	instanceID  string
	displayName string
	signer      *migration.Signer
}

// NewIdentityHandler creates a handler for /.well-known/vaultkeeper-instance.json.
func NewIdentityHandler(instanceID, displayName string, signer *migration.Signer) *IdentityHandler {
	return &IdentityHandler{
		instanceID:  instanceID,
		displayName: displayName,
		signer:      signer,
	}
}

// RegisterRoutes implements server.RouteRegistrar.
func (h *IdentityHandler) RegisterRoutes(r chi.Router) {
	r.Get("/.well-known/vaultkeeper-instance.json", h.ServeHTTP)
}

// ServeHTTP handles GET /.well-known/vaultkeeper-instance.json.
func (h *IdentityHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ms := NewManifestSigner(h.signer)

	resp := WellKnownResponse{
		Protocol:                "vaultkeeper-instance/v1",
		InstanceID:              h.instanceID,
		DisplayName:             h.displayName,
		Ed25519PublicKey:        h.signer.PublicKeyBase64(),
		KeyFingerprint:          ms.Fingerprint(),
		SupportedBundleVersions: []string{ProtocolVersionVKE1},
		FederationEndpoint:      "/api/federation/receive",
		KeyRotation: KeyRotationInfo{
			PreviousKeyFingerprint: nil,
			RotationStatementURL:   nil,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	json.NewEncoder(w).Encode(resp)
}
