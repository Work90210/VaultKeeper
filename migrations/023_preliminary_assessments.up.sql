BEGIN;

CREATE TABLE evidence_assessments (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    evidence_id           UUID NOT NULL REFERENCES evidence_items(id) ON DELETE RESTRICT,
    case_id               UUID NOT NULL REFERENCES cases(id) ON DELETE RESTRICT,

    relevance_score       INTEGER NOT NULL CHECK (relevance_score BETWEEN 1 AND 5),
    relevance_rationale   TEXT NOT NULL,

    reliability_score     INTEGER NOT NULL CHECK (reliability_score BETWEEN 1 AND 5),
    reliability_rationale TEXT NOT NULL,

    source_credibility    TEXT NOT NULL CHECK (source_credibility IN (
                              'established', 'credible', 'uncertain',
                              'unreliable', 'unassessed'
                          )),
    misleading_indicators TEXT[],
    recommendation        TEXT NOT NULL CHECK (recommendation IN (
                              'collect', 'monitor', 'deprioritize', 'discard'
                          )),

    methodology           TEXT,
    assessed_by           UUID NOT NULL,
    reviewed_by           UUID,
    reviewed_at           TIMESTAMPTZ,

    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_assessments_evidence ON evidence_assessments(evidence_id);
CREATE INDEX idx_assessments_case ON evidence_assessments(case_id);
CREATE INDEX idx_assessments_recommendation ON evidence_assessments(recommendation);

COMMIT;
