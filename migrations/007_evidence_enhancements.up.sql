-- Sprint 4: Evidence upload enhancements
-- Add missing columns to evidence_items, evidence counter on cases, and new indexes

BEGIN;

-- Add evidence counter to cases for gap-free sequential numbering
ALTER TABLE cases ADD COLUMN IF NOT EXISTS evidence_counter INTEGER NOT NULL DEFAULT 0;

-- Add missing columns to evidence_items
ALTER TABLE evidence_items ADD COLUMN IF NOT EXISTS evidence_number TEXT;
ALTER TABLE evidence_items ADD COLUMN IF NOT EXISTS storage_key TEXT;
ALTER TABLE evidence_items ADD COLUMN IF NOT EXISTS thumbnail_key TEXT;
ALTER TABLE evidence_items ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
ALTER TABLE evidence_items ADD COLUMN IF NOT EXISTS tags TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE evidence_items ADD COLUMN IF NOT EXISTS parent_id UUID REFERENCES evidence_items(id);
ALTER TABLE evidence_items ADD COLUMN IF NOT EXISTS tsa_name TEXT;
ALTER TABLE evidence_items ADD COLUMN IF NOT EXISTS tsa_timestamp TIMESTAMPTZ;
ALTER TABLE evidence_items ADD COLUMN IF NOT EXISTS tsa_status TEXT NOT NULL DEFAULT 'pending'
    CHECK (tsa_status IN ('pending', 'stamped', 'failed', 'disabled'));
ALTER TABLE evidence_items ADD COLUMN IF NOT EXISTS tsa_retry_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE evidence_items ADD COLUMN IF NOT EXISTS tsa_last_retry TIMESTAMPTZ;
ALTER TABLE evidence_items ADD COLUMN IF NOT EXISTS destroyed_at TIMESTAMPTZ;
ALTER TABLE evidence_items ADD COLUMN IF NOT EXISTS destroyed_by TEXT;
ALTER TABLE evidence_items ADD COLUMN IF NOT EXISTS destroy_reason TEXT;
ALTER TABLE evidence_items ADD COLUMN IF NOT EXISTS exif_data JSONB;

-- Indexes for evidence queries
CREATE INDEX IF NOT EXISTS idx_evidence_sha256 ON evidence_items (sha256_hash);
CREATE INDEX IF NOT EXISTS idx_evidence_case_current ON evidence_items (case_id, is_current)
    WHERE is_current = true AND destroyed_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_evidence_pending_tsa ON evidence_items (tsa_status)
    WHERE tsa_status = 'pending';
CREATE INDEX IF NOT EXISTS idx_evidence_parent ON evidence_items (parent_id)
    WHERE parent_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_evidence_tags ON evidence_items USING GIN (tags);

COMMIT;
