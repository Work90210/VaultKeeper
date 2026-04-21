-- Witness protection enhancements: risk levels, voice masking, duress

ALTER TABLE witnesses ADD COLUMN IF NOT EXISTS risk_level TEXT DEFAULT 'low' CHECK (risk_level IN ('low','medium','high','extreme'));
ALTER TABLE witnesses ADD COLUMN IF NOT EXISTS voice_masking_enabled BOOLEAN DEFAULT false;
ALTER TABLE witnesses ADD COLUMN IF NOT EXISTS duress_passphrase_enabled BOOLEAN DEFAULT false;
ALTER TABLE witnesses ADD COLUMN IF NOT EXISTS signed_at TIMESTAMPTZ;
