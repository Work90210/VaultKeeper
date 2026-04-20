BEGIN;

CREATE TABLE investigator_safety_profiles (
    id                        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id                   UUID NOT NULL REFERENCES cases(id) ON DELETE RESTRICT,
    user_id                   UUID NOT NULL,

    pseudonym                 TEXT,
    use_pseudonym             BOOLEAN NOT NULL DEFAULT false,

    opsec_level               TEXT NOT NULL DEFAULT 'standard' CHECK (opsec_level IN (
                                  'standard', 'elevated', 'high_risk'
                              )),
    required_vpn              BOOLEAN NOT NULL DEFAULT false,
    required_tor              BOOLEAN NOT NULL DEFAULT false,
    approved_devices          TEXT[],
    prohibited_platforms      TEXT[],

    threat_level              TEXT NOT NULL DEFAULT 'low' CHECK (threat_level IN (
                                  'low', 'medium', 'high', 'critical'
                              )),
    threat_notes              TEXT,

    safety_briefing_completed BOOLEAN NOT NULL DEFAULT false,
    safety_briefing_date      TIMESTAMPTZ,
    safety_officer_id         UUID,

    created_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (case_id, user_id)
);

CREATE INDEX idx_safety_profiles_case ON investigator_safety_profiles(case_id);
CREATE INDEX idx_safety_profiles_user ON investigator_safety_profiles(user_id);

COMMIT;
