DROP TABLE IF EXISTS corroboration_witnesses;
DROP TABLE IF EXISTS inquiry_log_evidence;

ALTER TABLE investigation_inquiry_logs DROP COLUMN IF EXISTS assigned_to;
ALTER TABLE investigation_inquiry_logs DROP COLUMN IF EXISTS priority;
ALTER TABLE investigation_inquiry_logs DROP COLUMN IF EXISTS sealed_status;
ALTER TABLE investigation_inquiry_logs DROP COLUMN IF EXISTS sealed_at;

ALTER TABLE evidence_assessments DROP COLUMN IF EXISTS assigned_to;
ALTER TABLE evidence_assessments DROP COLUMN IF EXISTS status;

ALTER TABLE cases DROP COLUMN IF EXISTS classification;
ALTER TABLE cases DROP COLUMN IF EXISTS start_date;

ALTER TABLE investigative_analysis_notes DROP COLUMN IF EXISTS tags;
ALTER TABLE investigative_analysis_notes DROP COLUMN IF EXISTS is_counter_evidence;

ALTER TABLE corroboration_claims DROP COLUMN IF EXISTS score;
ALTER TABLE corroboration_claims DROP COLUMN IF EXISTS status;
ALTER TABLE corroboration_claims DROP COLUMN IF EXISTS assigned_to;
