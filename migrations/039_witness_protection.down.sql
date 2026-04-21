ALTER TABLE witnesses DROP COLUMN IF EXISTS risk_level;
ALTER TABLE witnesses DROP COLUMN IF EXISTS voice_masking_enabled;
ALTER TABLE witnesses DROP COLUMN IF EXISTS duress_passphrase_enabled;
ALTER TABLE witnesses DROP COLUMN IF EXISTS signed_at;
