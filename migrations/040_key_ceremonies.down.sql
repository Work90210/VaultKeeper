DROP TABLE IF EXISTS generated_reports;
DROP TABLE IF EXISTS report_templates;
DROP TABLE IF EXISTS retention_policy_versions;
DROP TABLE IF EXISTS redaction_operations;
DROP TABLE IF EXISTS disclosure_countersigns;
DROP TABLE IF EXISTS key_ceremonies;

ALTER TABLE disclosures DROP COLUMN IF EXISTS due_date;
ALTER TABLE disclosures DROP COLUMN IF EXISTS owner_user_ids;
ALTER TABLE disclosures DROP COLUMN IF EXISTS wizard_step;
ALTER TABLE disclosures DROP COLUMN IF EXISTS countersign_required;
ALTER TABLE disclosures DROP COLUMN IF EXISTS countersigns_received;
ALTER TABLE disclosures DROP COLUMN IF EXISTS manifest_hash;
ALTER TABLE disclosures DROP COLUMN IF EXISTS bundle_url;
