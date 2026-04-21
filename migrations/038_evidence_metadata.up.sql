-- Evidence metadata: publication date, counter-evidence flag, media metadata, BP phases

ALTER TABLE evidence_items ADD COLUMN IF NOT EXISTS publication_date TIMESTAMPTZ;
ALTER TABLE evidence_items ADD COLUMN IF NOT EXISTS is_counter_evidence BOOLEAN DEFAULT false;
ALTER TABLE evidence_items ADD COLUMN IF NOT EXISTS rule_77_disclosed BOOLEAN DEFAULT false;
ALTER TABLE evidence_items ADD COLUMN IF NOT EXISTS media_metadata JSONB;

-- BP phase tracking (FOUNDATIONAL — everything depends on this)
CREATE TABLE IF NOT EXISTS evidence_bp_phases (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  evidence_id UUID NOT NULL REFERENCES evidence_items(id) ON DELETE CASCADE,
  phase INTEGER NOT NULL CHECK (phase BETWEEN 1 AND 6),
  status TEXT NOT NULL DEFAULT 'not_started' CHECK (status IN ('not_started','in_progress','complete')),
  completed_at TIMESTAMPTZ,
  completed_by UUID,
  UNIQUE(evidence_id, phase)
);

-- Initialize 6 phases for all existing evidence items
INSERT INTO evidence_bp_phases (evidence_id, phase, status)
SELECT e.id, p.phase, 'not_started'
FROM evidence_items e
CROSS JOIN (VALUES (1),(2),(3),(4),(5),(6)) AS p(phase)
ON CONFLICT DO NOTHING;

-- Index for dashboard queries
CREATE INDEX IF NOT EXISTS idx_evidence_bp_phases_evidence ON evidence_bp_phases(evidence_id);
CREATE INDEX IF NOT EXISTS idx_evidence_bp_phases_status ON evidence_bp_phases(status);
