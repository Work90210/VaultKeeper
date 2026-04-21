-- Key ceremonies tracking

CREATE TABLE IF NOT EXISTS key_ceremonies (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  type TEXT NOT NULL,
  hardware_provider TEXT,
  holders UUID[] NOT NULL,
  quorum_required INTEGER NOT NULL DEFAULT 2,
  quorum_achieved INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'initiated',
  initiated_by UUID NOT NULL,
  witnessed_by UUID[],
  completed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Disclosure countersigns
CREATE TABLE IF NOT EXISTS disclosure_countersigns (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  disclosure_id UUID NOT NULL REFERENCES disclosures(id) ON DELETE CASCADE,
  user_id UUID NOT NULL,
  signed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  signature TEXT NOT NULL,
  UNIQUE(disclosure_id, user_id)
);

-- Disclosure enhancements
ALTER TABLE disclosures ADD COLUMN IF NOT EXISTS due_date DATE;
ALTER TABLE disclosures ADD COLUMN IF NOT EXISTS owner_user_ids UUID[];
ALTER TABLE disclosures ADD COLUMN IF NOT EXISTS wizard_step INTEGER DEFAULT 1;
ALTER TABLE disclosures ADD COLUMN IF NOT EXISTS countersign_required INTEGER DEFAULT 0;
ALTER TABLE disclosures ADD COLUMN IF NOT EXISTS countersigns_received INTEGER DEFAULT 0;
ALTER TABLE disclosures ADD COLUMN IF NOT EXISTS manifest_hash TEXT;
ALTER TABLE disclosures ADD COLUMN IF NOT EXISTS bundle_url TEXT;

-- Redaction operation log (for defence replay)
CREATE TABLE IF NOT EXISTS redaction_operations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  draft_id UUID NOT NULL REFERENCES redaction_drafts(id) ON DELETE CASCADE,
  evidence_id UUID NOT NULL,
  sequence_number BIGINT NOT NULL,
  operation_type TEXT NOT NULL CHECK (operation_type IN ('add_mark','modify_mark','delete_mark')),
  mark_id TEXT NOT NULL,
  mark_type TEXT NOT NULL CHECK (mark_type IN ('redact','pseudonymise','geo_fuzz','translate','annotate')),
  mark_data JSONB NOT NULL,
  previous_state JSONB,
  author_user_id UUID NOT NULL,
  author_username TEXT NOT NULL,
  op_hash TEXT NOT NULL,
  previous_op_hash TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(draft_id, sequence_number)
);

-- Retention policy versions
CREATE TABLE IF NOT EXISTS retention_policy_versions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  version INTEGER NOT NULL,
  policy_json JSONB NOT NULL,
  changed_by UUID NOT NULL,
  changed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Report templates
CREATE TABLE IF NOT EXISTS report_templates (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  description TEXT,
  type TEXT NOT NULL CHECK (type IN ('standard','governance','legal','platform')),
  icon TEXT,
  generator_key TEXT NOT NULL UNIQUE
);

-- Generated reports
CREATE TABLE IF NOT EXISTS generated_reports (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  template_id UUID NOT NULL REFERENCES report_templates(id),
  case_id UUID,
  generated_by UUID NOT NULL,
  hash TEXT NOT NULL,
  sealed_at TIMESTAMPTZ,
  file_url TEXT,
  file_size BIGINT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Seed report templates
INSERT INTO report_templates (name, description, type, icon, generator_key) VALUES
  ('Custody summary', 'Complete chain of custody for all exhibits in a case', 'standard', 'document', 'custody_summary'),
  ('Disclosure dossier', 'Sealed disclosure package with redaction map and manifest', 'legal', 'shield', 'disclosure_dossier'),
  ('Ceremony minutes', 'Record of key ceremony proceedings and quorum', 'governance', 'key', 'ceremony_minutes'),
  ('Quarterly retention', 'Retention policy compliance report', 'governance', 'calendar', 'quarterly_retention'),
  ('Counter-evidence register', 'Rule 77 exculpatory material index', 'legal', 'scale', 'counter_evidence'),
  ('Federation diff', 'Cross-instance synchronization delta report', 'platform', 'globe', 'federation_diff')
ON CONFLICT (generator_key) DO NOTHING;
