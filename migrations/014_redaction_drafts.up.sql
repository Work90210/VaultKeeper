CREATE TABLE redaction_drafts (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    evidence_id   UUID NOT NULL REFERENCES evidence_items(id) ON DELETE CASCADE,
    case_id       UUID NOT NULL REFERENCES cases(id) ON DELETE RESTRICT,
    created_by    UUID NOT NULL,
    yjs_state     BYTEA,
    status        TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'applied', 'discarded')),
    last_saved_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Only one active draft per evidence item
CREATE UNIQUE INDEX idx_redaction_drafts_evidence_draft
    ON redaction_drafts(evidence_id) WHERE status = 'draft';

-- FK index for cascade operations on cases
CREATE INDEX idx_redaction_drafts_case_id ON redaction_drafts(case_id);
