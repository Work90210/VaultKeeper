package federation

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ExchangeRecord represents a persisted exchange manifest row.
type ExchangeRecord struct {
	ID               uuid.UUID
	ExchangeID       uuid.UUID
	Direction        string // "outgoing" or "incoming"
	PeerInstanceID   *uuid.UUID
	CaseID           *uuid.UUID
	ManifestHash     string
	ScopeHash        string
	MerkleRoot       string
	ScopeCardinality int
	Signature        []byte
	Status           string
	CreatedAt        time.Time
	CompletedAt      *time.Time
}

// ExchangeRepository provides PostgreSQL access for exchange_manifests
// and exchange_evidence_items tables.
type ExchangeRepository struct {
	pool *pgxpool.Pool
}

// NewExchangeRepository creates a new exchange repository.
func NewExchangeRepository(pool *pgxpool.Pool) *ExchangeRepository {
	return &ExchangeRepository{pool: pool}
}

// Create inserts a new exchange manifest record.
func (r *ExchangeRepository) Create(ctx context.Context, record ExchangeRecord) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO exchange_manifests
			(id, exchange_id, direction, peer_instance_id, case_id,
			 manifest_hash, scope_hash, merkle_root, scope_cardinality,
			 signature, status, created_at, completed_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		record.ID, record.ExchangeID, record.Direction, record.PeerInstanceID,
		record.CaseID, record.ManifestHash, record.ScopeHash, record.MerkleRoot,
		record.ScopeCardinality, record.Signature, record.Status,
		record.CreatedAt, record.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("insert exchange manifest: %w", err)
	}
	return nil
}

// GetByExchangeID retrieves a single exchange by its exchange_id.
func (r *ExchangeRepository) GetByExchangeID(ctx context.Context, exchangeID uuid.UUID) (ExchangeRecord, error) {
	var rec ExchangeRecord
	err := r.pool.QueryRow(ctx,
		`SELECT id, exchange_id, direction, peer_instance_id, case_id,
		        manifest_hash, scope_hash, merkle_root, scope_cardinality,
		        signature, status, created_at, completed_at
		 FROM exchange_manifests
		 WHERE exchange_id = $1`, exchangeID).
		Scan(&rec.ID, &rec.ExchangeID, &rec.Direction, &rec.PeerInstanceID,
			&rec.CaseID, &rec.ManifestHash, &rec.ScopeHash, &rec.MerkleRoot,
			&rec.ScopeCardinality, &rec.Signature, &rec.Status,
			&rec.CreatedAt, &rec.CompletedAt)
	if err != nil {
		return ExchangeRecord{}, fmt.Errorf("get exchange %s: %w", exchangeID, err)
	}
	return rec, nil
}

// ListByCase returns all exchanges associated with a case.
func (r *ExchangeRepository) ListByCase(ctx context.Context, caseID uuid.UUID) ([]ExchangeRecord, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, exchange_id, direction, peer_instance_id, case_id,
		        manifest_hash, scope_hash, merkle_root, scope_cardinality,
		        signature, status, created_at, completed_at
		 FROM exchange_manifests
		 WHERE case_id = $1
		 ORDER BY created_at DESC`, caseID)
	if err != nil {
		return nil, fmt.Errorf("list exchanges by case %s: %w", caseID, err)
	}
	defer rows.Close()

	return scanExchangeRows(rows)
}

// ListByPeer returns all exchanges associated with a peer instance.
func (r *ExchangeRepository) ListByPeer(ctx context.Context, peerInstanceID uuid.UUID) ([]ExchangeRecord, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, exchange_id, direction, peer_instance_id, case_id,
		        manifest_hash, scope_hash, merkle_root, scope_cardinality,
		        signature, status, created_at, completed_at
		 FROM exchange_manifests
		 WHERE peer_instance_id = $1
		 ORDER BY created_at DESC`, peerInstanceID)
	if err != nil {
		return nil, fmt.Errorf("list exchanges by peer %s: %w", peerInstanceID, err)
	}
	defer rows.Close()

	return scanExchangeRows(rows)
}

// UpdateStatus sets the status of an exchange manifest.
func (r *ExchangeRepository) UpdateStatus(ctx context.Context, exchangeID uuid.UUID, status string) error {
	var completedAt *time.Time
	if status == "completed" || status == "failed" {
		now := time.Now().UTC()
		completedAt = &now
	}

	tag, err := r.pool.Exec(ctx,
		`UPDATE exchange_manifests
		 SET status = $1, completed_at = $2
		 WHERE exchange_id = $3`,
		status, completedAt, exchangeID)
	if err != nil {
		return fmt.Errorf("update exchange status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("exchange %s not found", exchangeID)
	}
	return nil
}

// AddEvidenceItems associates evidence IDs with an exchange manifest.
func (r *ExchangeRepository) AddEvidenceItems(ctx context.Context, exchangeManifestID uuid.UUID, evidenceIDs []uuid.UUID) error {
	if len(evidenceIDs) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for _, eid := range evidenceIDs {
		batch.Queue(
			`INSERT INTO exchange_evidence_items (exchange_manifest_id, evidence_id)
			 VALUES ($1, $2)
			 ON CONFLICT DO NOTHING`,
			exchangeManifestID, eid,
		)
	}

	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()

	for range evidenceIDs {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("add exchange evidence item: %w", err)
		}
	}

	return nil
}

// scanExchangeRows scans multiple exchange rows into a slice.
func scanExchangeRows(rows pgx.Rows) ([]ExchangeRecord, error) {
	var records []ExchangeRecord
	for rows.Next() {
		var rec ExchangeRecord
		if err := rows.Scan(&rec.ID, &rec.ExchangeID, &rec.Direction, &rec.PeerInstanceID,
			&rec.CaseID, &rec.ManifestHash, &rec.ScopeHash, &rec.MerkleRoot,
			&rec.ScopeCardinality, &rec.Signature, &rec.Status,
			&rec.CreatedAt, &rec.CompletedAt); err != nil {
			return nil, fmt.Errorf("scan exchange row: %w", err)
		}
		records = append(records, rec)
	}
	return records, nil
}
