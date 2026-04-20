BEGIN;

CREATE TABLE evidence_verification_records (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    evidence_id         UUID NOT NULL REFERENCES evidence_items(id) ON DELETE RESTRICT,
    case_id             UUID NOT NULL REFERENCES cases(id) ON DELETE RESTRICT,

    verification_type   TEXT NOT NULL CHECK (verification_type IN (
                            'source_authentication', 'content_verification',
                            'reverse_image_search', 'geolocation_verification',
                            'chronolocation', 'metadata_analysis',
                            'witness_corroboration', 'expert_analysis',
                            'open_source_cross_reference', 'other'
                        )),
    methodology         TEXT NOT NULL,
    tools_used          TEXT[],
    sources_consulted   TEXT[],

    finding             TEXT NOT NULL CHECK (finding IN (
                            'authentic', 'likely_authentic', 'inconclusive',
                            'likely_manipulated', 'manipulated', 'unable_to_verify'
                        )),
    finding_rationale   TEXT NOT NULL,
    confidence_level    TEXT NOT NULL CHECK (confidence_level IN (
                            'high', 'medium', 'low'
                        )),

    limitations         TEXT,
    caveats             TEXT[],

    verified_by         UUID NOT NULL,
    reviewer            UUID,
    reviewer_approved   BOOLEAN,
    reviewer_notes      TEXT,
    reviewed_at         TIMESTAMPTZ,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_verification_evidence ON evidence_verification_records(evidence_id);
CREATE INDEX idx_verification_case ON evidence_verification_records(case_id);
CREATE INDEX idx_verification_type ON evidence_verification_records(verification_type);
CREATE INDEX idx_verification_finding ON evidence_verification_records(finding);

COMMIT;
