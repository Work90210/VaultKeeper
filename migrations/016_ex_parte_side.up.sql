-- Migration 016: Ex parte side
-- Adds ex_parte_side column for one-sided ("ex parte") evidence items.
-- When classification = 'ex_parte' the side must be 'prosecution' or 'defence'.
-- For any other classification, ex_parte_side must be NULL.

ALTER TABLE evidence_items
    ADD COLUMN ex_parte_side TEXT;

ALTER TABLE evidence_items
    ADD CONSTRAINT chk_ex_parte_side
        CHECK (
            (classification  = 'ex_parte' AND ex_parte_side IN ('prosecution', 'defence'))
         OR (classification != 'ex_parte' AND ex_parte_side IS NULL)
        );

CREATE INDEX idx_evidence_items_ex_parte_side
    ON evidence_items (case_id, ex_parte_side)
    WHERE ex_parte_side IS NOT NULL;
