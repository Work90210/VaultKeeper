BEGIN;

-- Organizations: top-level multi-tenancy unit
CREATE TABLE organizations (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          TEXT NOT NULL,
    slug          TEXT UNIQUE,
    description   TEXT NOT NULL DEFAULT '',
    logo_asset_id UUID,
    settings      JSONB NOT NULL DEFAULT '{}',
    created_by    UUID NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ
);

-- Organization memberships
CREATE TABLE organization_memberships (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL,
    role            TEXT NOT NULL CHECK (role IN ('owner', 'admin', 'member')),
    status          TEXT NOT NULL CHECK (status IN ('active', 'invited', 'suspended', 'removed')),
    joined_at       TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, user_id)
);

CREATE INDEX idx_org_memberships_user ON organization_memberships(user_id, status);
CREATE INDEX idx_org_memberships_org ON organization_memberships(organization_id, status);

-- Organization invitations
CREATE TABLE organization_invitations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email           TEXT NOT NULL,
    role            TEXT NOT NULL CHECK (role IN ('admin', 'member')),
    token_hash      TEXT NOT NULL,
    invited_by      UUID NOT NULL,
    status          TEXT NOT NULL CHECK (status IN ('pending', 'accepted', 'declined', 'expired', 'revoked')),
    expires_at      TIMESTAMPTZ NOT NULL,
    accepted_by     UUID,
    accepted_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_org_invitations_pending
    ON organization_invitations(organization_id, email)
    WHERE status = 'pending';

COMMIT;
