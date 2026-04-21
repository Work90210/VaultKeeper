-- Federation peer roles + exchange latency tracking

ALTER TABLE peer_instances ADD COLUMN IF NOT EXISTS role TEXT CHECK (role IN ('sub-chain','full-peer','read-only','cold-mirror','staging'));
ALTER TABLE federation_exchanges ADD COLUMN IF NOT EXISTS exchange_duration_ms INTEGER;
