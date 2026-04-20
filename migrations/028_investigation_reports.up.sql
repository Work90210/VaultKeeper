BEGIN;

CREATE TABLE investigation_reports (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id                 UUID NOT NULL REFERENCES cases(id) ON DELETE RESTRICT,

    title                   TEXT NOT NULL,
    report_type             TEXT NOT NULL CHECK (report_type IN (
                                'interim', 'final', 'supplementary', 'expert_opinion'
                            )),

    sections                JSONB NOT NULL DEFAULT '[]',

    limitations             TEXT[],
    caveats                 TEXT[],
    assumptions             TEXT[],

    referenced_evidence_ids UUID[],
    referenced_analysis_ids UUID[],

    status                  TEXT NOT NULL DEFAULT 'draft' CHECK (status IN (
                                'draft', 'in_review', 'approved', 'published', 'withdrawn'
                            )),

    author_id               UUID NOT NULL,
    reviewer_id             UUID,
    reviewed_at             TIMESTAMPTZ,
    approved_by             UUID,
    approved_at             TIMESTAMPTZ,

    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_reports_case ON investigation_reports(case_id);
CREATE INDEX idx_reports_type ON investigation_reports(report_type);
CREATE INDEX idx_reports_status ON investigation_reports(status);

COMMIT;
