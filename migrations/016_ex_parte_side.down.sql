-- Migration 016 (down): remove ex_parte_side column.

DROP INDEX IF EXISTS idx_evidence_items_ex_parte_side;

ALTER TABLE evidence_items
    DROP CONSTRAINT IF EXISTS chk_ex_parte_side;

ALTER TABLE evidence_items
    DROP COLUMN IF EXISTS ex_parte_side;
