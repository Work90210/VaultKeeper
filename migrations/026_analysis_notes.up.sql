BEGIN;

CREATE TABLE investigative_analysis_notes (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id                  UUID NOT NULL REFERENCES cases(id) ON DELETE RESTRICT,

    title                    TEXT NOT NULL,
    analysis_type            TEXT NOT NULL CHECK (analysis_type IN (
                                 'factual_finding', 'pattern_analysis',
                                 'timeline_reconstruction', 'geographic_analysis',
                                 'network_analysis', 'legal_assessment',
                                 'credibility_assessment', 'gap_identification',
                                 'hypothesis_testing', 'other'
                             )),
    content                  TEXT NOT NULL,
    methodology              TEXT,

    related_evidence_ids     UUID[],
    related_inquiry_ids      UUID[],
    related_assessment_ids   UUID[],
    related_verification_ids UUID[],

    status                   TEXT NOT NULL DEFAULT 'draft' CHECK (status IN (
                                 'draft', 'in_review', 'approved', 'superseded'
                             )),
    superseded_by            UUID REFERENCES investigative_analysis_notes(id),

    author_id                UUID NOT NULL,
    reviewer_id              UUID,
    reviewed_at              TIMESTAMPTZ,

    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_analysis_notes_case ON investigative_analysis_notes(case_id);
CREATE INDEX idx_analysis_notes_type ON investigative_analysis_notes(analysis_type);
CREATE INDEX idx_analysis_notes_status ON investigative_analysis_notes(status);

COMMIT;
