BEGIN;

CREATE TABLE evidence_capture_metadata (
    id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- RESTRICT: capture provenance is part of chain of custody — must be
    -- explicitly removed before evidence can be deleted
    evidence_id                 UUID NOT NULL UNIQUE
                                REFERENCES evidence_items(id) ON DELETE RESTRICT,

    -- Source identification
    source_url                  TEXT,
    canonical_url               TEXT,
    platform                    TEXT CHECK (platform IN (
                                    'x', 'facebook', 'instagram', 'youtube',
                                    'telegram', 'tiktok', 'whatsapp', 'signal',
                                    'reddit', 'web', 'other'
                                )),
    platform_content_type       TEXT CHECK (platform_content_type IN (
                                    'post', 'profile', 'video', 'image',
                                    'comment', 'story', 'livestream',
                                    'channel', 'page', 'other'
                                )),

    -- Capture context
    capture_method              TEXT NOT NULL CHECK (capture_method IN (
                                    'screenshot', 'screen_recording', 'web_archive',
                                    'api_export', 'manual_download', 'browser_save',
                                    'forensic_tool', 'other'
                                )),
    capture_timestamp           TIMESTAMPTZ NOT NULL,
    publication_timestamp       TIMESTAMPTZ,

    -- Collector identity (no FK — users are in Keycloak/SSO, not a local table)
    -- SECURITY: collector_display_name is encrypted at the application layer
    -- (same pattern as witness PII) to protect investigator identity at rest.
    collector_user_id           UUID,
    collector_display_name_encrypted BYTEA,

    -- Content creator
    creator_account_handle      TEXT,
    creator_account_display_name TEXT,
    creator_account_url         TEXT,
    creator_account_id          TEXT,

    -- Content metadata
    content_description         TEXT,
    content_language            TEXT,

    -- Geolocation
    geo_latitude                NUMERIC(9,6),
    geo_longitude               NUMERIC(9,6),
    geo_place_name              TEXT,
    geo_source                  TEXT CHECK (geo_source IN (
                                    'exif', 'platform_metadata', 'manual_entry',
                                    'derived', 'unknown'
                                )),

    -- Availability
    availability_status         TEXT CHECK (availability_status IN (
                                    'accessible', 'deleted', 'geo_blocked',
                                    'login_required', 'account_suspended',
                                    'removed', 'unavailable', 'unknown'
                                )),
    was_live                    BOOLEAN,
    was_deleted                 BOOLEAN,

    -- Capture environment
    capture_tool_name           TEXT,
    capture_tool_version        TEXT,
    browser_name                TEXT,
    browser_version             TEXT,
    browser_user_agent          TEXT,

    -- Network context (JSONB for flexibility + sensitivity)
    -- Expected shape: {"vpn_used": bool, "tor_used": bool, "proxy_used": bool,
    --                   "capture_ip_region": "XX", "notes": "..."}
    -- SECURITY: NEVER indexed in search. Access gated to investigator/prosecutor roles.
    network_context             JSONB,

    -- Preservation & verification
    preservation_notes          TEXT,
    verification_status         TEXT NOT NULL DEFAULT 'unverified' CHECK (verification_status IN (
                                    'unverified', 'partially_verified', 'verified', 'disputed'
                                )),
    verification_notes          TEXT,

    -- Schema evolution
    metadata_schema_version     INTEGER NOT NULL DEFAULT 1,

    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Geo pair constraint: both or neither
ALTER TABLE evidence_capture_metadata
ADD CONSTRAINT chk_capture_geo_pair
CHECK (
    (geo_latitude IS NULL AND geo_longitude IS NULL) OR
    (geo_latitude IS NOT NULL AND geo_longitude IS NOT NULL)
);

-- Indexes
CREATE INDEX idx_capture_metadata_platform
    ON evidence_capture_metadata(platform);
CREATE INDEX idx_capture_metadata_capture_ts
    ON evidence_capture_metadata(capture_timestamp);
CREATE INDEX idx_capture_metadata_verification
    ON evidence_capture_metadata(verification_status);
CREATE INDEX idx_capture_metadata_collector
    ON evidence_capture_metadata(collector_user_id)
    WHERE collector_user_id IS NOT NULL;

COMMIT;
