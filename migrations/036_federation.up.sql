-- Federation: peer instances, key management, and exchange tracking.

CREATE TABLE peer_instances (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id      TEXT NOT NULL UNIQUE,
    display_name     TEXT NOT NULL,
    well_known_url   TEXT,
    trust_mode       TEXT NOT NULL DEFAULT 'untrusted',
    verified_by      TEXT,
    verified_at      TIMESTAMPTZ,
    verification_channel TEXT,
    org_id           UUID REFERENCES organizations(id),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE peer_instance_keys (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    peer_instance_id UUID NOT NULL REFERENCES peer_instances(id) ON DELETE CASCADE,
    public_key       BYTEA NOT NULL,
    fingerprint      TEXT NOT NULL,
    status           TEXT NOT NULL DEFAULT 'active',
    valid_from       TIMESTAMPTZ NOT NULL DEFAULT now(),
    valid_to         TIMESTAMPTZ,
    rotation_sig_old BYTEA,
    rotation_sig_new BYTEA,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_peer_keys_active
    ON peer_instance_keys(peer_instance_id)
    WHERE status = 'active';

CREATE TABLE exchange_manifests (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    exchange_id           UUID NOT NULL UNIQUE,
    direction             TEXT NOT NULL,
    peer_instance_id      UUID REFERENCES peer_instances(id),
    case_id               UUID REFERENCES cases(id),
    manifest_hash         TEXT NOT NULL,
    scope_hash            TEXT NOT NULL,
    merkle_root           TEXT NOT NULL,
    scope_cardinality     INT NOT NULL,
    signature             BYTEA NOT NULL,
    status                TEXT NOT NULL DEFAULT 'pending',
    verification_details  JSONB,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at          TIMESTAMPTZ
);

CREATE TABLE exchange_evidence_items (
    exchange_manifest_id  UUID NOT NULL REFERENCES exchange_manifests(id) ON DELETE CASCADE,
    evidence_id           UUID NOT NULL REFERENCES evidence_items(id),
    PRIMARY KEY (exchange_manifest_id, evidence_id)
);

CREATE INDEX idx_exchange_evidence_by_item
    ON exchange_evidence_items(evidence_id);

CREATE TABLE derivation_records (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    child_evidence_id     UUID NOT NULL REFERENCES evidence_items(id),
    parent_evidence_id    UUID NOT NULL,
    derivation_type       TEXT NOT NULL,
    derivation_commitment TEXT NOT NULL,
    parameters_commitment TEXT NOT NULL,
    signed_by_instance    TEXT NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
