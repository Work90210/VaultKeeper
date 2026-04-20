package federation

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/evidence"
	"github.com/vaultkeeper/vaultkeeper/internal/integrity"
	"github.com/vaultkeeper/vaultkeeper/internal/migration"
)

// EvidenceQuerier retrieves evidence items for scope evaluation.
type EvidenceQuerier interface {
	ListByCaseForExport(ctx context.Context, caseID uuid.UUID, userRole string) ([]evidence.EvidenceItem, error)
}

// EvidenceFileReader retrieves evidence file content from storage.
type EvidenceFileReader interface {
	GetObject(ctx context.Context, key string) (io.ReadCloser, int64, string, error)
}

// CustodyQuerier retrieves custody events for a case.
type CustodyQuerier interface {
	ListAllByCase(ctx context.Context, caseID uuid.UUID) ([]any, error)
}

// Service orchestrates federation exchange operations: scope evaluation,
// tree construction, manifest signing, and bundle packing.
type Service struct {
	instanceID     string
	signer         *migration.Signer
	manifestSigner ManifestSigner
	peerStore      PeerStore
	exchangeRepo   *ExchangeRepository
	evidenceQuery  EvidenceQuerier
	fileReader     EvidenceFileReader
	custody        CustodyRecorder
	tsa            integrity.TimestampAuthority
	logger         *slog.Logger
}

// NewService creates a new federation service.
func NewService(
	instanceID string,
	signer *migration.Signer,
	peerStore PeerStore,
	exchangeRepo *ExchangeRepository,
	evidenceQuery EvidenceQuerier,
	fileReader EvidenceFileReader,
	custody CustodyRecorder,
	tsa integrity.TimestampAuthority,
	logger *slog.Logger,
) *Service {
	return &Service{
		instanceID:     instanceID,
		signer:         signer,
		manifestSigner: NewManifestSigner(signer),
		peerStore:      peerStore,
		exchangeRepo:   exchangeRepo,
		evidenceQuery:  evidenceQuery,
		fileReader:     fileReader,
		custody:        custody,
		tsa:            tsa,
		logger:         logger,
	}
}

// CreateExchangeInput holds the parameters for creating a new exchange.
type CreateExchangeInput struct {
	CaseID            uuid.UUID
	PeerInstanceID    uuid.UUID
	Scope             ScopeDescriptor
	DependencyPolicy  string
	ActorUserID       string
	CustodyHead       string
}

// CreateExchangeResult holds the output of a successful exchange creation.
type CreateExchangeResult struct {
	ExchangeID   uuid.UUID
	ManifestHash string
	MerkleRoot   string
	ItemCount    int
}

// CreateExchange evaluates a scope, builds the Merkle tree, constructs
// and signs the manifest, records custody bridge events, and returns
// the exchange metadata. The bundle is NOT packed here — that happens
// in the download handler for streaming.
func (s *Service) CreateExchange(ctx context.Context, input CreateExchangeInput) (CreateExchangeResult, error) {
	// Validate scope.
	if err := ValidateScope(input.Scope); err != nil {
		return CreateExchangeResult{}, fmt.Errorf("validate scope: %w", err)
	}

	// Resolve peer.
	peer, err := s.peerStore.GetPeer(ctx, input.PeerInstanceID)
	if err != nil {
		return CreateExchangeResult{}, fmt.Errorf("get peer: %w", err)
	}
	if peer.TrustMode == TrustModeUntrusted || peer.TrustMode == TrustModeRevoked {
		return CreateExchangeResult{}, fmt.Errorf("peer %q is not trusted (mode: %s)", peer.InstanceID, peer.TrustMode)
	}

	// Fetch all case evidence, then filter by scope predicate.
	allItems, err := s.evidenceQuery.ListByCaseForExport(ctx, input.Scope.CaseID, "")
	if err != nil {
		return CreateExchangeResult{}, fmt.Errorf("query evidence: %w", err)
	}

	// Evaluate scope predicate — only matching items are disclosed.
	matchedItems, err := EvaluateScope(allItems, input.Scope)
	if err != nil {
		return CreateExchangeResult{}, fmt.Errorf("evaluate scope: %w", err)
	}

	// Build descriptors from matched items only.
	descriptors := make([]EvidenceDescriptor, 0, len(matchedItems))
	for _, item := range matchedItems {
		descriptors = append(descriptors, BuildDescriptor(item))
	}

	if len(descriptors) == 0 {
		return CreateExchangeResult{}, fmt.Errorf("no evidence items match scope")
	}

	// Build Merkle tree.
	tree, err := BuildScopedMerkleTree(descriptors)
	if err != nil {
		return CreateExchangeResult{}, fmt.Errorf("build merkle tree: %w", err)
	}

	// Build exchange manifest.
	exchangeID := uuid.New()
	recipientID := peer.InstanceID
	manifest, err := BuildExchangeManifest(
		exchangeID,
		s.instanceID,
		s.manifestSigner.Fingerprint(),
		&recipientID,
		input.Scope,
		descriptors,
		tree.Root,
		input.DependencyPolicy,
		input.CustodyHead,
		"", // bridge event hash computed after recording
	)
	if err != nil {
		return CreateExchangeResult{}, fmt.Errorf("build manifest: %w", err)
	}

	// Sign manifest.
	sig, err := SignManifestHex(s.manifestSigner, manifest.ManifestHash)
	if err != nil {
		return CreateExchangeResult{}, fmt.Errorf("sign manifest: %w", err)
	}

	// Record custody bridge event.
	evidenceIDs := make([]uuid.UUID, len(descriptors))
	for i, d := range descriptors {
		evidenceIDs[i] = d.EvidenceID
	}

	if err := RecordDisclosureBridgeEvent(ctx, s.custody, DisclosureBridgeInput{
		CaseID:               input.Scope.CaseID,
		ExchangeID:           exchangeID,
		ManifestHash:         manifest.ManifestHash,
		RecipientInstanceID:  peer.InstanceID,
		RecipientFingerprint: "", // resolved from peer key
		DisclosedEvidenceIDs: evidenceIDs,
		ScopeHash:            manifest.ScopeHash,
		MerkleRoot:           manifest.MerkleRoot,
		ScopeCardinality:     manifest.ScopeCardinality,
		ActorUserID:          input.ActorUserID,
	}); err != nil {
		return CreateExchangeResult{}, fmt.Errorf("record custody bridge: %w", err)
	}

	// Persist exchange manifest to DB.
	caseID := input.Scope.CaseID
	record := ExchangeRecord{
		ID:               uuid.New(),
		ExchangeID:       exchangeID,
		Direction:        "outgoing",
		PeerInstanceID:   &peer.ID,
		CaseID:           &caseID,
		ManifestHash:     manifest.ManifestHash,
		ScopeHash:        manifest.ScopeHash,
		MerkleRoot:       manifest.MerkleRoot,
		ScopeCardinality: manifest.ScopeCardinality,
		Signature:        sig,
		Status:           "pending",
		CreatedAt:        manifest.CreatedAt,
	}
	if err := s.exchangeRepo.Create(ctx, record); err != nil {
		return CreateExchangeResult{}, fmt.Errorf("persist exchange: %w", err)
	}

	// Link evidence items to the exchange manifest.
	if err := s.exchangeRepo.AddEvidenceItems(ctx, record.ID, evidenceIDs); err != nil {
		return CreateExchangeResult{}, fmt.Errorf("persist exchange evidence items: %w", err)
	}

	s.logger.Info("exchange created",
		"exchange_id", exchangeID,
		"peer", peer.InstanceID,
		"items", len(descriptors),
		"manifest_hash", manifest.ManifestHash,
	)

	return CreateExchangeResult{
		ExchangeID:   exchangeID,
		ManifestHash: manifest.ManifestHash,
		MerkleRoot:   manifest.MerkleRoot,
		ItemCount:    len(descriptors),
	}, nil
}

// ExchangeDetail holds the full details of an exchange for API responses.
type ExchangeDetail struct {
	ExchangeID       uuid.UUID `json:"exchange_id"`
	Direction        string    `json:"direction"`
	PeerInstanceID   string    `json:"peer_instance_id"`
	PeerDisplayName  string    `json:"peer_display_name"`
	ManifestHash     string    `json:"manifest_hash"`
	ScopeHash        string    `json:"scope_hash"`
	MerkleRoot       string    `json:"merkle_root"`
	ScopeCardinality int       `json:"scope_cardinality"`
	Status           string    `json:"status"`
	CreatedAt        string    `json:"created_at"`
}

