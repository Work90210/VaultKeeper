BEGIN;

CREATE TABLE investigation_inquiry_logs (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id             UUID NOT NULL REFERENCES cases(id) ON DELETE RESTRICT,
    evidence_id         UUID REFERENCES evidence_items(id) ON DELETE SET NULL,

    search_strategy     TEXT NOT NULL,
    search_keywords     TEXT[],
    search_operators    TEXT,

    search_tool         TEXT NOT NULL,
    search_tool_version TEXT,
    search_url          TEXT,

    search_started_at   TIMESTAMPTZ NOT NULL,
    search_ended_at     TIMESTAMPTZ,
    results_count       INTEGER,
    results_relevant    INTEGER,
    results_collected   INTEGER,

    objective           TEXT NOT NULL,
    notes               TEXT,
    performed_by        UUID NOT NULL,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_inquiry_logs_case ON investigation_inquiry_logs(case_id);
CREATE INDEX idx_inquiry_logs_performer ON investigation_inquiry_logs(performed_by);
CREATE INDEX idx_inquiry_logs_started ON investigation_inquiry_logs(search_started_at);

COMMIT;
