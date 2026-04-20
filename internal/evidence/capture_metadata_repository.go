package evidence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrCaptureMetadataNotFound is returned when no capture metadata exists for an evidence item.
var ErrCaptureMetadataNotFound = errors.New("capture metadata not found")

// CaptureMetadataRepository defines the data access interface for capture metadata.
type CaptureMetadataRepository interface {
	GetByEvidenceID(ctx context.Context, evidenceID uuid.UUID) (*EvidenceCaptureMetadata, error)
	UpsertByEvidenceID(ctx context.Context, evidenceID uuid.UUID, metadata *EvidenceCaptureMetadata) error
	DeleteByEvidenceID(ctx context.Context, evidenceID uuid.UUID) error
	GetByEvidenceIDs(ctx context.Context, evidenceIDs []uuid.UUID) (map[uuid.UUID]*EvidenceCaptureMetadata, error)
}

// PGCaptureMetadataRepository is the Postgres implementation.
type PGCaptureMetadataRepository struct {
	pool dbPool
}

// NewCaptureMetadataRepository creates a new Postgres capture metadata repository.
func NewCaptureMetadataRepository(pool dbPool) *PGCaptureMetadataRepository {
	return &PGCaptureMetadataRepository{pool: pool}
}

const captureMetadataColumns = `id, evidence_id, source_url, canonical_url, platform,
	platform_content_type, capture_method, capture_timestamp, publication_timestamp,
	collector_user_id, collector_display_name_encrypted,
	creator_account_handle, creator_account_display_name, creator_account_url, creator_account_id,
	content_description, content_language,
	geo_latitude, geo_longitude, geo_place_name, geo_source,
	availability_status, was_live, was_deleted,
	capture_tool_name, capture_tool_version, browser_name, browser_version, browser_user_agent,
	network_context, preservation_notes, verification_status, verification_notes,
	metadata_schema_version, created_at, updated_at`

func scanCaptureMetadata(row pgx.Row) (*EvidenceCaptureMetadata, error) {
	var m EvidenceCaptureMetadata
	var networkContextJSON []byte
	err := row.Scan(
		&m.ID, &m.EvidenceID, &m.SourceURL, &m.CanonicalURL, &m.Platform,
		&m.PlatformContentType, &m.CaptureMethod, &m.CaptureTimestamp, &m.PublicationTimestamp,
		&m.CollectorUserID, &m.CollectorDisplayNameEncrypted,
		&m.CreatorAccountHandle, &m.CreatorAccountDisplayName, &m.CreatorAccountURL, &m.CreatorAccountID,
		&m.ContentDescription, &m.ContentLanguage,
		&m.GeoLatitude, &m.GeoLongitude, &m.GeoPlaceName, &m.GeoSource,
		&m.AvailabilityStatus, &m.WasLive, &m.WasDeleted,
		&m.CaptureToolName, &m.CaptureToolVersion, &m.BrowserName, &m.BrowserVersion, &m.BrowserUserAgent,
		&networkContextJSON, &m.PreservationNotes, &m.VerificationStatus, &m.VerificationNotes,
		&m.MetadataSchemaVersion, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if networkContextJSON != nil {
		if err := json.Unmarshal(networkContextJSON, &m.NetworkContext); err != nil {
			return nil, fmt.Errorf("unmarshal network_context: %w", err)
		}
	}
	return &m, nil
}

func (r *PGCaptureMetadataRepository) GetByEvidenceID(ctx context.Context, evidenceID uuid.UUID) (*EvidenceCaptureMetadata, error) {
	query := fmt.Sprintf(`SELECT %s FROM evidence_capture_metadata WHERE evidence_id = $1`, captureMetadataColumns)
	row := r.pool.QueryRow(ctx, query, evidenceID)
	m, err := scanCaptureMetadata(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCaptureMetadataNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get capture metadata: %w", err)
	}
	return m, nil
}

func (r *PGCaptureMetadataRepository) UpsertByEvidenceID(ctx context.Context, evidenceID uuid.UUID, m *EvidenceCaptureMetadata) error {
	var networkContextJSON []byte
	if m.NetworkContext != nil {
		var err error
		networkContextJSON, err = json.Marshal(m.NetworkContext)
		if err != nil {
			return fmt.Errorf("marshal network_context: %w", err)
		}
	}

	now := time.Now().UTC()

	query := `INSERT INTO evidence_capture_metadata (
		id, evidence_id, source_url, canonical_url, platform, platform_content_type,
		capture_method, capture_timestamp, publication_timestamp,
		collector_user_id, collector_display_name_encrypted,
		creator_account_handle, creator_account_display_name, creator_account_url, creator_account_id,
		content_description, content_language,
		geo_latitude, geo_longitude, geo_place_name, geo_source,
		availability_status, was_live, was_deleted,
		capture_tool_name, capture_tool_version, browser_name, browser_version, browser_user_agent,
		network_context, preservation_notes, verification_status, verification_notes,
		metadata_schema_version, created_at, updated_at
	) VALUES (
		$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11,
		$12, $13, $14, $15, $16, $17, $18, $19, $20, $21,
		$22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34, $35, $36
	)
	ON CONFLICT (evidence_id) DO UPDATE SET
		source_url = EXCLUDED.source_url,
		canonical_url = EXCLUDED.canonical_url,
		platform = EXCLUDED.platform,
		platform_content_type = EXCLUDED.platform_content_type,
		capture_method = EXCLUDED.capture_method,
		capture_timestamp = EXCLUDED.capture_timestamp,
		publication_timestamp = EXCLUDED.publication_timestamp,
		collector_user_id = EXCLUDED.collector_user_id,
		collector_display_name_encrypted = EXCLUDED.collector_display_name_encrypted,
		creator_account_handle = EXCLUDED.creator_account_handle,
		creator_account_display_name = EXCLUDED.creator_account_display_name,
		creator_account_url = EXCLUDED.creator_account_url,
		creator_account_id = EXCLUDED.creator_account_id,
		content_description = EXCLUDED.content_description,
		content_language = EXCLUDED.content_language,
		geo_latitude = EXCLUDED.geo_latitude,
		geo_longitude = EXCLUDED.geo_longitude,
		geo_place_name = EXCLUDED.geo_place_name,
		geo_source = EXCLUDED.geo_source,
		availability_status = EXCLUDED.availability_status,
		was_live = EXCLUDED.was_live,
		was_deleted = EXCLUDED.was_deleted,
		capture_tool_name = EXCLUDED.capture_tool_name,
		capture_tool_version = EXCLUDED.capture_tool_version,
		browser_name = EXCLUDED.browser_name,
		browser_version = EXCLUDED.browser_version,
		browser_user_agent = EXCLUDED.browser_user_agent,
		network_context = EXCLUDED.network_context,
		preservation_notes = EXCLUDED.preservation_notes,
		verification_status = EXCLUDED.verification_status,
		verification_notes = EXCLUDED.verification_notes,
		metadata_schema_version = EXCLUDED.metadata_schema_version,
		updated_at = EXCLUDED.updated_at`

	id := m.ID
	if id == uuid.Nil {
		id = uuid.New()
	}

	_, err := r.pool.Exec(ctx, query,
		id, evidenceID, m.SourceURL, m.CanonicalURL, m.Platform, m.PlatformContentType,
		m.CaptureMethod, m.CaptureTimestamp, m.PublicationTimestamp,
		m.CollectorUserID, m.CollectorDisplayNameEncrypted,
		m.CreatorAccountHandle, m.CreatorAccountDisplayName, m.CreatorAccountURL, m.CreatorAccountID,
		m.ContentDescription, m.ContentLanguage,
		m.GeoLatitude, m.GeoLongitude, m.GeoPlaceName, m.GeoSource,
		m.AvailabilityStatus, m.WasLive, m.WasDeleted,
		m.CaptureToolName, m.CaptureToolVersion, m.BrowserName, m.BrowserVersion, m.BrowserUserAgent,
		networkContextJSON, m.PreservationNotes, m.VerificationStatus, m.VerificationNotes,
		m.MetadataSchemaVersion, now, now,
	)
	if err != nil {
		return fmt.Errorf("upsert capture metadata: %w", err)
	}
	return nil
}

func (r *PGCaptureMetadataRepository) DeleteByEvidenceID(ctx context.Context, evidenceID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM evidence_capture_metadata WHERE evidence_id = $1`,
		evidenceID,
	)
	if err != nil {
		return fmt.Errorf("delete capture metadata: %w", err)
	}
	return nil
}

func (r *PGCaptureMetadataRepository) GetByEvidenceIDs(ctx context.Context, evidenceIDs []uuid.UUID) (map[uuid.UUID]*EvidenceCaptureMetadata, error) {
	if len(evidenceIDs) == 0 {
		return map[uuid.UUID]*EvidenceCaptureMetadata{}, nil
	}

	query := fmt.Sprintf(`SELECT %s FROM evidence_capture_metadata WHERE evidence_id = ANY($1)`, captureMetadataColumns)
	rows, err := r.pool.Query(ctx, query, evidenceIDs)
	if err != nil {
		return nil, fmt.Errorf("batch get capture metadata: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID]*EvidenceCaptureMetadata, len(evidenceIDs))
	for rows.Next() {
		m, err := scanCaptureMetadata(rows)
		if err != nil {
			return nil, fmt.Errorf("scan capture metadata row: %w", err)
		}
		result[m.EvidenceID] = m
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate capture metadata rows: %w", err)
	}
	return result, nil
}
