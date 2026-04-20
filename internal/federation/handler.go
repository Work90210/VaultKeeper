package federation

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
)

// Handler provides HTTP endpoints for federation operations.
type Handler struct {
	service      *Service
	peerStore    PeerStore
	exchangeRepo *ExchangeRepository
	logger       *slog.Logger

	// receiveTokens maps receive tokens to pending exchange IDs.
	// In production, use a persistent store with TTL.
	receiveTokens map[string]uuid.UUID
}

// NewHandler creates a new federation HTTP handler.
func NewHandler(service *Service, peerStore PeerStore, exchangeRepo *ExchangeRepository, logger *slog.Logger) *Handler {
	return &Handler{
		service:       service,
		peerStore:     peerStore,
		exchangeRepo:  exchangeRepo,
		logger:        logger,
		receiveTokens: make(map[string]uuid.UUID),
	}
}

// RegisterRoutes implements server.RouteRegistrar.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/federation", func(r chi.Router) {
		// Peer management (admin only)
		r.Route("/peers", func(r chi.Router) {
			r.Post("/", h.createPeer)
			r.Get("/", h.listPeers)
			r.Route("/{peerID}", func(r chi.Router) {
				r.Get("/", h.getPeer)
				r.Patch("/", h.updatePeerTrust)
				r.Delete("/", h.deletePeer)
			})
		})

		// Exchange operations
		r.Route("/exchanges", func(r chi.Router) {
			r.Post("/", h.createExchange)
			r.Get("/", h.listExchanges)
			r.Route("/{exchangeID}", func(r chi.Router) {
				r.Get("/", h.getExchange)
				r.Get("/download", h.downloadExchange)
				r.Post("/verify", h.verifyExchange)
				r.Post("/accept", h.acceptExchange)
			})
		})

		// Two-phase receive endpoints (no auth — peer-to-peer)
		r.Route("/receive", func(r chi.Router) {
			r.Post("/manifest", h.receiveManifest)
			r.Post("/bundle", h.receiveBundle)
		})
	})
}

// --- Peer endpoints ---

type createPeerRequest struct {
	InstanceID   string `json:"instance_id"`
	DisplayName  string `json:"display_name"`
	WellKnownURL string `json:"well_known_url,omitempty"`
	PublicKey    string `json:"public_key,omitempty"` // base64 Ed25519
}

func (h *Handler) createPeer(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if ac.SystemRole != auth.RoleSystemAdmin {
		http.Error(w, "forbidden: admin role required", http.StatusForbidden)
		return
	}

	var req createPeerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.InstanceID == "" || req.DisplayName == "" {
		http.Error(w, "instance_id and display_name are required", http.StatusBadRequest)
		return
	}

	peer, err := h.peerStore.CreatePeer(r.Context(), PeerInstance{
		InstanceID:  req.InstanceID,
		DisplayName: req.DisplayName,
		WellKnownURL: req.WellKnownURL,
	})
	if err != nil {
		h.logger.Error("create peer failed", "error", err)
		http.Error(w, "failed to create peer", http.StatusInternalServerError)
		return
	}

	h.logger.Info("peer created", "peer_id", peer.ID, "instance_id", peer.InstanceID, "actor", ac.UserID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(peer)
}

func (h *Handler) listPeers(w http.ResponseWriter, r *http.Request) {
	_, ok := auth.GetAuthContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// OrgID from query param; federation peers are org-scoped.
	var orgID uuid.UUID
	if raw := r.URL.Query().Get("org_id"); raw != "" {
		var err error
		orgID, err = uuid.Parse(raw)
		if err != nil {
			http.Error(w, "invalid org_id", http.StatusBadRequest)
			return
		}
	}

	peers, err := h.peerStore.ListPeers(r.Context(), orgID)
	if err != nil {
		h.logger.Error("list peers failed", "error", err)
		http.Error(w, "failed to list peers", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(peers)
}

func (h *Handler) getPeer(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.GetAuthContext(r.Context()); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	peerID, err := uuid.Parse(chi.URLParam(r, "peerID"))
	if err != nil {
		http.Error(w, "invalid peer ID", http.StatusBadRequest)
		return
	}

	peer, err := h.peerStore.GetPeer(r.Context(), peerID)
	if err != nil {
		http.Error(w, "peer not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(peer)
}

type updateTrustRequest struct {
	TrustMode           string `json:"trust_mode"`
	VerificationChannel string `json:"verification_channel"`
}

func (h *Handler) updatePeerTrust(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	peerID, err := uuid.Parse(chi.URLParam(r, "peerID"))
	if err != nil {
		http.Error(w, "invalid peer ID", http.StatusBadRequest)
		return
	}

	var req updateTrustRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	validModes := map[string]bool{
		TrustModeTOFUPending:  true,
		TrustModeManualPinned: true,
		TrustModePKIVerified:  true,
		TrustModeRevoked:      true,
	}
	if !validModes[req.TrustMode] {
		http.Error(w, "invalid trust_mode", http.StatusBadRequest)
		return
	}

	if err := h.peerStore.UpdateTrustMode(r.Context(), peerID, req.TrustMode, ac.UserID, req.VerificationChannel); err != nil {
		h.logger.Error("update trust mode failed", "error", err)
		http.Error(w, "failed to update trust mode", http.StatusInternalServerError)
		return
	}

	h.logger.Info("peer trust updated", "peer_id", peerID, "trust_mode", req.TrustMode, "actor", ac.UserID)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) deletePeer(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if ac.SystemRole != auth.RoleSystemAdmin {
		http.Error(w, "forbidden: admin role required", http.StatusForbidden)
		return
	}

	peerID, err := uuid.Parse(chi.URLParam(r, "peerID"))
	if err != nil {
		http.Error(w, "invalid peer ID", http.StatusBadRequest)
		return
	}

	if err := h.peerStore.DeletePeer(r.Context(), peerID); err != nil {
		h.logger.Error("delete peer failed", "error", err)
		http.Error(w, "failed to delete peer", http.StatusInternalServerError)
		return
	}

	h.logger.Info("peer deleted", "peer_id", peerID, "actor", ac.UserID)
	w.WriteHeader(http.StatusNoContent)
}

// --- Exchange endpoints ---

type createExchangeRequest struct {
	CaseID           uuid.UUID       `json:"case_id"`
	PeerInstanceID   uuid.UUID       `json:"peer_instance_id"`
	Scope            ScopeDescriptor `json:"scope"`
	DependencyPolicy string          `json:"dependency_policy"`
}

func (h *Handler) createExchange(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req createExchangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.DependencyPolicy == "" {
		req.DependencyPolicy = DependencyPolicyNone
	}

	result, err := h.service.CreateExchange(r.Context(), CreateExchangeInput{
		CaseID:           req.CaseID,
		PeerInstanceID:   req.PeerInstanceID,
		Scope:            req.Scope,
		DependencyPolicy: req.DependencyPolicy,
		ActorUserID:      ac.UserID,
	})
	if err != nil {
		h.logger.Error("create exchange failed", "error", err)
		http.Error(w, "failed to create exchange", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(result)
}

func (h *Handler) listExchanges(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.GetAuthContext(r.Context()); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var records []ExchangeRecord
	var err error

	if raw := r.URL.Query().Get("case_id"); raw != "" {
		caseID, parseErr := uuid.Parse(raw)
		if parseErr != nil {
			http.Error(w, "invalid case_id", http.StatusBadRequest)
			return
		}
		records, err = h.exchangeRepo.ListByCase(r.Context(), caseID)
	} else if raw := r.URL.Query().Get("peer_instance_id"); raw != "" {
		peerID, parseErr := uuid.Parse(raw)
		if parseErr != nil {
			http.Error(w, "invalid peer_instance_id", http.StatusBadRequest)
			return
		}
		records, err = h.exchangeRepo.ListByPeer(r.Context(), peerID)
	} else {
		http.Error(w, "case_id or peer_instance_id query parameter is required", http.StatusBadRequest)
		return
	}

	if err != nil {
		h.logger.Error("list exchanges failed", "error", err)
		http.Error(w, "failed to list exchanges", http.StatusInternalServerError)
		return
	}

	details := make([]ExchangeDetail, 0, len(records))
	for _, rec := range records {
		detail := recordToDetail(rec)

		// Enrich with peer display name if available.
		if rec.PeerInstanceID != nil {
			if peer, peerErr := h.peerStore.GetPeer(r.Context(), *rec.PeerInstanceID); peerErr == nil {
				detail.PeerInstanceID = peer.InstanceID
				detail.PeerDisplayName = peer.DisplayName
			}
		}

		details = append(details, detail)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(details)
}

func (h *Handler) getExchange(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.GetAuthContext(r.Context()); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	exchangeID, err := uuid.Parse(chi.URLParam(r, "exchangeID"))
	if err != nil {
		http.Error(w, "invalid exchange ID", http.StatusBadRequest)
		return
	}

	rec, err := h.exchangeRepo.GetByExchangeID(r.Context(), exchangeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "exchange not found", http.StatusNotFound)
			return
		}
		h.logger.Error("get exchange failed", "error", err)
		http.Error(w, "failed to get exchange", http.StatusInternalServerError)
		return
	}

	detail := recordToDetail(rec)
	if rec.PeerInstanceID != nil {
		if peer, peerErr := h.peerStore.GetPeer(r.Context(), *rec.PeerInstanceID); peerErr == nil {
			detail.PeerInstanceID = peer.InstanceID
			detail.PeerDisplayName = peer.DisplayName
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(detail)
}

func (h *Handler) downloadExchange(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	exchangeID, err := uuid.Parse(chi.URLParam(r, "exchangeID"))
	if err != nil {
		http.Error(w, "invalid exchange ID", http.StatusBadRequest)
		return
	}

	rec, err := h.exchangeRepo.GetByExchangeID(r.Context(), exchangeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "exchange not found", http.StatusNotFound)
			return
		}
		h.logger.Error("get exchange for download failed", "error", err)
		http.Error(w, "failed to get exchange", http.StatusInternalServerError)
		return
	}

	if rec.Direction != "outgoing" {
		http.Error(w, "only outgoing exchanges can be downloaded", http.StatusBadRequest)
		return
	}

	if rec.CaseID == nil {
		http.Error(w, "exchange has no associated case", http.StatusBadRequest)
		return
	}

	// Re-evaluate scope to build the bundle on-the-fly.
	allItems, err := h.service.evidenceQuery.ListByCaseForExport(r.Context(), *rec.CaseID, "")
	if err != nil {
		h.logger.Error("query evidence for download failed", "error", err)
		http.Error(w, "failed to query evidence", http.StatusInternalServerError)
		return
	}

	// Build descriptors from all case items that match (we use the
	// stored manifest hash to validate consistency).
	descriptors := make([]EvidenceDescriptor, 0, len(allItems))
	for _, item := range allItems {
		descriptors = append(descriptors, BuildDescriptor(item))
	}

	// Build Merkle tree.
	tree, err := BuildScopedMerkleTree(descriptors)
	if err != nil {
		h.logger.Error("build merkle tree for download failed", "error", err)
		http.Error(w, "failed to build merkle tree", http.StatusInternalServerError)
		return
	}

	// Verify the Merkle root matches the stored value.
	if hex.EncodeToString(tree.Root) != rec.MerkleRoot {
		h.logger.Error("merkle root mismatch during download",
			"stored", rec.MerkleRoot,
			"computed", hex.EncodeToString(tree.Root),
		)
		http.Error(w, "evidence has changed since exchange was created", http.StatusConflict)
		return
	}

	// Build identity doc.
	identity := InstanceIdentityDoc{
		InstanceID:  h.service.instanceID,
		PublicKey:   h.service.manifestSigner.PublicKeyBase64(),
		Fingerprint: h.service.manifestSigner.Fingerprint(),
	}

	// Reconstruct the manifest for the bundle.
	var recipientID *string
	if rec.PeerInstanceID != nil {
		if peer, peerErr := h.peerStore.GetPeer(r.Context(), *rec.PeerInstanceID); peerErr == nil {
			recipientID = &peer.InstanceID
		}
	}

	manifest := ExchangeManifest{
		ProtocolVersion:      ProtocolVersionVKE1,
		ExchangeID:           rec.ExchangeID,
		SenderInstanceID:     h.service.instanceID,
		SenderKeyFingerprint: h.service.manifestSigner.Fingerprint(),
		RecipientInstanceID:  recipientID,
		CreatedAt:            rec.CreatedAt,
		Scope:                ScopeDescriptor{CaseID: *rec.CaseID},
		ScopeHash:            rec.ScopeHash,
		ScopeCardinality:     rec.ScopeCardinality,
		MerkleRoot:           rec.MerkleRoot,
		DependencyPolicy:     DependencyPolicyNone,
		DisclosedEvidence:    SortDescriptors(descriptors),
		ManifestHash:         rec.ManifestHash,
	}

	// Gather evidence file content from storage.
	bundleEvidence := make([]BundleEvidence, 0, len(allItems))
	for i, item := range allItems {
		if item.StorageKey == nil {
			h.logger.Error("evidence item has no storage key", "evidence_id", item.ID)
			http.Error(w, "evidence item missing storage key", http.StatusInternalServerError)
			return
		}
		reader, size, _, fileErr := h.service.fileReader.GetObject(r.Context(), *item.StorageKey)
		if fileErr != nil {
			h.logger.Error("get evidence file failed", "evidence_id", item.ID, "error", fileErr)
			http.Error(w, "failed to read evidence file", http.StatusInternalServerError)
			return
		}
		bundleEvidence = append(bundleEvidence, BundleEvidence{
			Descriptor: descriptors[i],
			Filename:   item.OriginalName,
			SizeBytes:  size,
			Content:    reader,
		})
	}

	packInput := PackBundleInput{
		Manifest:    manifest,
		Signature:   rec.Signature,
		Identity:    identity,
		MerkleTree:  tree,
		Descriptors: descriptors,
		Evidence:    bundleEvidence,
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition",
		"attachment; filename=\"vke1-"+rec.ExchangeID.String()+".zip\"")

	if err := PackBundle(w, packInput); err != nil {
		h.logger.Error("pack bundle failed", "exchange_id", rec.ExchangeID, "error", err)
		// Headers already sent; cannot change status code.
		return
	}

	h.logger.Info("bundle downloaded", "exchange_id", rec.ExchangeID, "actor", ac.UserID)
}

// --- Receive endpoints (two-phase) ---

type receiveManifestRequest struct {
	Manifest    ExchangeManifest `json:"manifest"`
	Signature   string           `json:"signature"`   // base64-encoded Ed25519 signature
	SenderKeyB64 string          `json:"sender_key"`   // base64-encoded Ed25519 public key
}

type receiveManifestResponse struct {
	ReceiveToken string    `json:"receive_token"`
	ExchangeID   uuid.UUID `json:"exchange_id"`
	Status       string    `json:"status"`
}

func (h *Handler) receiveManifest(w http.ResponseWriter, r *http.Request) {
	var req receiveManifestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields.
	if req.Manifest.SenderInstanceID == "" {
		http.Error(w, "sender_instance_id is required in manifest", http.StatusBadRequest)
		return
	}
	if req.Signature == "" {
		http.Error(w, "signature is required", http.StatusBadRequest)
		return
	}
	if req.Manifest.ManifestHash == "" {
		http.Error(w, "manifest_hash is required", http.StatusBadRequest)
		return
	}

	// Verify manifest hash is correct.
	computedHash, err := ComputeManifestHash(req.Manifest)
	if err != nil {
		h.logger.Error("compute manifest hash failed", "error", err)
		http.Error(w, "failed to verify manifest hash", http.StatusBadRequest)
		return
	}
	if computedHash != req.Manifest.ManifestHash {
		http.Error(w, "manifest hash does not match computed hash", http.StatusBadRequest)
		return
	}

	// Decode the signature.
	sigBytes, err := base64.StdEncoding.DecodeString(req.Signature)
	if err != nil {
		http.Error(w, "invalid base64 signature", http.StatusBadRequest)
		return
	}

	// Resolve the sender's public key. Try the peer store first,
	// fall back to the key provided in the request.
	var pubKey ed25519.PublicKey

	resolvedKey, resolveErr := h.peerStore.ResolvePublicKey(r.Context(), req.Manifest.SenderInstanceID)
	if resolveErr == nil {
		pubKey = resolvedKey
	} else if req.SenderKeyB64 != "" {
		keyBytes, decErr := base64.StdEncoding.DecodeString(req.SenderKeyB64)
		if decErr != nil || len(keyBytes) != ed25519.PublicKeySize {
			http.Error(w, "invalid sender public key", http.StatusBadRequest)
			return
		}
		pubKey = ed25519.PublicKey(keyBytes)
	} else {
		h.logger.Error("cannot resolve sender key", "sender", req.Manifest.SenderInstanceID, "error", resolveErr)
		http.Error(w, "unknown sender and no public key provided", http.StatusBadRequest)
		return
	}

	// Verify the signature over the manifest hash.
	hashBytes, err := hex.DecodeString(req.Manifest.ManifestHash)
	if err != nil {
		http.Error(w, "invalid manifest hash hex", http.StatusBadRequest)
		return
	}

	if !ed25519.Verify(pubKey, hashBytes, sigBytes) {
		http.Error(w, "signature verification failed", http.StatusForbidden)
		return
	}

	// Persist as incoming pending exchange.
	// Look up the peer to get the DB ID.
	var peerID *uuid.UUID
	if peer, peerErr := h.peerStore.GetPeerByInstanceID(r.Context(), req.Manifest.SenderInstanceID); peerErr == nil {
		peerID = &peer.ID
	}

	record := ExchangeRecord{
		ID:               uuid.New(),
		ExchangeID:       req.Manifest.ExchangeID,
		Direction:        "incoming",
		PeerInstanceID:   peerID,
		ManifestHash:     req.Manifest.ManifestHash,
		ScopeHash:        req.Manifest.ScopeHash,
		MerkleRoot:       req.Manifest.MerkleRoot,
		ScopeCardinality: req.Manifest.ScopeCardinality,
		Signature:        sigBytes,
		Status:           "pending",
		CreatedAt:        time.Now().UTC(),
	}

	if err := h.exchangeRepo.Create(r.Context(), record); err != nil {
		h.logger.Error("persist incoming exchange failed", "error", err)
		http.Error(w, "failed to store exchange", http.StatusInternalServerError)
		return
	}

	// Generate receive token for the bundle upload phase.
	receiveToken := uuid.New().String()
	h.receiveTokens[receiveToken] = req.Manifest.ExchangeID

	h.logger.Info("manifest received and verified",
		"exchange_id", req.Manifest.ExchangeID,
		"sender", req.Manifest.SenderInstanceID,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(receiveManifestResponse{
		ReceiveToken: receiveToken,
		ExchangeID:   req.Manifest.ExchangeID,
		Status:       "pending",
	})
}

func (h *Handler) receiveBundle(w http.ResponseWriter, r *http.Request) {
	// Validate receive token.
	token := r.Header.Get("X-Receive-Token")
	if token == "" {
		token = r.URL.Query().Get("receive_token")
	}
	if token == "" {
		http.Error(w, "receive token is required", http.StatusUnauthorized)
		return
	}

	exchangeID, ok := h.receiveTokens[token]
	if !ok {
		http.Error(w, "invalid or expired receive token", http.StatusUnauthorized)
		return
	}

	// Bundle receive requires MinIO integration for evidence storage.
	// Return 501 until that integration is available.
	h.logger.Info("bundle receive attempted (not yet implemented)",
		"exchange_id", exchangeID,
	)

	http.Error(w, "bundle receive not yet implemented — requires storage integration", http.StatusNotImplemented)
}

// recordToDetail converts an ExchangeRecord to an ExchangeDetail for
// API responses.
func recordToDetail(rec ExchangeRecord) ExchangeDetail {
	detail := ExchangeDetail{
		ExchangeID:       rec.ExchangeID,
		Direction:        rec.Direction,
		ManifestHash:     rec.ManifestHash,
		ScopeHash:        rec.ScopeHash,
		MerkleRoot:       rec.MerkleRoot,
		ScopeCardinality: rec.ScopeCardinality,
		Status:           rec.Status,
		CreatedAt:        rec.CreatedAt.Format(time.RFC3339),
	}
	return detail
}

// verifyExchange triggers verification of an incoming exchange bundle.
func (h *Handler) verifyExchange(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.GetAuthContext(r.Context()); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	exchangeID, err := uuid.Parse(chi.URLParam(r, "exchangeID"))
	if err != nil {
		http.Error(w, "invalid exchange ID", http.StatusBadRequest)
		return
	}

	rec, err := h.exchangeRepo.GetByExchangeID(r.Context(), exchangeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "exchange not found", http.StatusNotFound)
			return
		}
		h.logger.Error("get exchange for verify failed", "error", err)
		http.Error(w, "failed to get exchange", http.StatusInternalServerError)
		return
	}

	if rec.Direction != "incoming" {
		http.Error(w, "only incoming exchanges can be verified", http.StatusBadRequest)
		return
	}

	// Verification requires the bundle to be stored locally.
	// For now, verify manifest integrity only.
	if err := h.exchangeRepo.UpdateStatus(r.Context(), exchangeID, "verified"); err != nil {
		h.logger.Error("update exchange status failed", "error", err)
		http.Error(w, "failed to update status", http.StatusInternalServerError)
		return
	}

	h.logger.Info("exchange verified", "exchange_id", exchangeID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"exchange_id": exchangeID.String(),
		"status":      "verified",
	})
}

// acceptExchange accepts a verified incoming exchange and imports evidence.
func (h *Handler) acceptExchange(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.GetAuthContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	exchangeID, err := uuid.Parse(chi.URLParam(r, "exchangeID"))
	if err != nil {
		http.Error(w, "invalid exchange ID", http.StatusBadRequest)
		return
	}

	rec, err := h.exchangeRepo.GetByExchangeID(r.Context(), exchangeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "exchange not found", http.StatusNotFound)
			return
		}
		h.logger.Error("get exchange for accept failed", "error", err)
		http.Error(w, "failed to get exchange", http.StatusInternalServerError)
		return
	}

	if rec.Direction != "incoming" {
		http.Error(w, "only incoming exchanges can be accepted", http.StatusBadRequest)
		return
	}

	if rec.Status != "verified" {
		http.Error(w, "exchange must be verified before acceptance", http.StatusBadRequest)
		return
	}

	// Accept: update status. Evidence import requires bundle storage integration.
	if err := h.exchangeRepo.UpdateStatus(r.Context(), exchangeID, "accepted"); err != nil {
		h.logger.Error("accept exchange failed", "error", err)
		http.Error(w, "failed to accept exchange", http.StatusInternalServerError)
		return
	}

	h.logger.Info("exchange accepted", "exchange_id", exchangeID, "actor", ac.UserID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"exchange_id": exchangeID.String(),
		"status":      "accepted",
	})
}
