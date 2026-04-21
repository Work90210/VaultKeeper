DROP TABLE IF EXISTS evidence_bp_phases;
ALTER TABLE evidence_items DROP COLUMN IF EXISTS publication_date;
ALTER TABLE evidence_items DROP COLUMN IF EXISTS is_counter_evidence;
ALTER TABLE evidence_items DROP COLUMN IF EXISTS rule_77_disclosed;
ALTER TABLE evidence_items DROP COLUMN IF EXISTS media_metadata;
