-- Investigation enhancements: inquiry log workflow, assessment scores, analysis tags

-- Inquiry logs: lock/seal workflow + assignment
ALTER TABLE investigation_inquiry_logs ADD COLUMN IF NOT EXISTS assigned_to UUID;
ALTER TABLE investigation_inquiry_logs ADD COLUMN IF NOT EXISTS priority TEXT DEFAULT 'normal' CHECK (priority IN ('normal','urgent','low'));
ALTER TABLE investigation_inquiry_logs ADD COLUMN IF NOT EXISTS sealed_status TEXT DEFAULT 'active' CHECK (sealed_status IN ('active','locked','complete'));
ALTER TABLE investigation_inquiry_logs ADD COLUMN IF NOT EXISTS sealed_at TIMESTAMPTZ;

-- Inquiry log ↔ evidence many-to-many (replaces single FK)
CREATE TABLE IF NOT EXISTS inquiry_log_evidence (
  inquiry_log_id UUID NOT NULL REFERENCES investigation_inquiry_logs(id) ON DELETE CASCADE,
  evidence_id UUID NOT NULL REFERENCES evidence_items(id) ON DELETE CASCADE,
  PRIMARY KEY (inquiry_log_id, evidence_id)
);

-- Migrate existing single FK data to join table
INSERT INTO inquiry_log_evidence (inquiry_log_id, evidence_id)
SELECT id, evidence_id FROM investigation_inquiry_logs
WHERE evidence_id IS NOT NULL
ON CONFLICT DO NOTHING;

-- Assessments: widen scores 1-5 → 1-10 + add fields
ALTER TABLE evidence_assessments ALTER COLUMN relevance_score TYPE INTEGER;
ALTER TABLE evidence_assessments DROP CONSTRAINT IF EXISTS evidence_assessments_relevance_score_check;
ALTER TABLE evidence_assessments ADD CONSTRAINT evidence_assessments_relevance_score_check CHECK (relevance_score BETWEEN 1 AND 10);
ALTER TABLE evidence_assessments ALTER COLUMN reliability_score TYPE INTEGER;
ALTER TABLE evidence_assessments DROP CONSTRAINT IF EXISTS evidence_assessments_reliability_score_check;
ALTER TABLE evidence_assessments ADD CONSTRAINT evidence_assessments_reliability_score_check CHECK (reliability_score BETWEEN 1 AND 10);
ALTER TABLE evidence_assessments ADD COLUMN IF NOT EXISTS assigned_to UUID;
ALTER TABLE evidence_assessments ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'sealed' CHECK (status IN ('draft','sealed'));

-- Fix credibility labels
UPDATE evidence_assessments SET source_credibility = 'probable' WHERE source_credibility = 'credible';
UPDATE evidence_assessments SET source_credibility = 'unconfirmed' WHERE source_credibility = 'uncertain';
UPDATE evidence_assessments SET source_credibility = 'doubtful' WHERE source_credibility = 'unreliable';

-- Cases: classification + start date
ALTER TABLE cases ADD COLUMN IF NOT EXISTS classification TEXT CHECK (classification IN ('criminal','investigation','monitoring','archival'));
ALTER TABLE cases ADD COLUMN IF NOT EXISTS start_date DATE;

-- Analysis notes: tags + counter-evidence flag
ALTER TABLE investigative_analysis_notes ADD COLUMN IF NOT EXISTS tags TEXT[];
ALTER TABLE investigative_analysis_notes ADD COLUMN IF NOT EXISTS is_counter_evidence BOOLEAN DEFAULT false;

-- Corroboration claims: numeric score + status + assignment
ALTER TABLE corroboration_claims ADD COLUMN IF NOT EXISTS score NUMERIC(3,2);
ALTER TABLE corroboration_claims ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'investigating' CHECK (status IN ('investigating','corroborated','weak','refuted'));
ALTER TABLE corroboration_claims ADD COLUMN IF NOT EXISTS assigned_to UUID;

-- Corroboration ↔ witness linking
CREATE TABLE IF NOT EXISTS corroboration_witnesses (
  claim_id UUID NOT NULL REFERENCES corroboration_claims(id) ON DELETE CASCADE,
  witness_id UUID NOT NULL REFERENCES witnesses(id) ON DELETE CASCADE,
  role_in_claim TEXT NOT NULL DEFAULT 'supporting',
  PRIMARY KEY (claim_id, witness_id)
);
