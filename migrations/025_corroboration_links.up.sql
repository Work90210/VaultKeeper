BEGIN;

CREATE TABLE corroboration_claims (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id             UUID NOT NULL REFERENCES cases(id) ON DELETE RESTRICT,

    claim_summary       TEXT NOT NULL,
    claim_type          TEXT NOT NULL CHECK (claim_type IN (
                            'event_occurrence', 'identity_confirmation',
                            'location_confirmation', 'timeline_confirmation',
                            'pattern_of_conduct', 'contextual_corroboration',
                            'other'
                        )),
    strength            TEXT NOT NULL CHECK (strength IN (
                            'strong', 'moderate', 'weak', 'contested'
                        )),
    analysis_notes      TEXT,

    created_by          UUID NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE corroboration_evidence (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    claim_id            UUID NOT NULL REFERENCES corroboration_claims(id) ON DELETE CASCADE,
    evidence_id         UUID NOT NULL REFERENCES evidence_items(id) ON DELETE RESTRICT,
    role_in_claim       TEXT NOT NULL CHECK (role_in_claim IN (
                            'primary', 'supporting', 'contextual', 'contradicting'
                        )),
    contribution_notes  TEXT,
    added_by            UUID NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (claim_id, evidence_id)
);

CREATE INDEX idx_corroboration_claims_case ON corroboration_claims(case_id);
CREATE INDEX idx_corroboration_evidence_claim ON corroboration_evidence(claim_id);
CREATE INDEX idx_corroboration_evidence_item ON corroboration_evidence(evidence_id);

COMMIT;
