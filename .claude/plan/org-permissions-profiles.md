# Implementation Plan: Organizations, Case Permissions & Profiles

## Task Type
- [x] Frontend
- [x] Backend
- [x] Fullstack (Parallel)

---

## Technical Solution

**Architecture**: Single-organization case ownership model. Each case belongs to exactly one organization. Users access cases through org membership + case role assignment. Three permission layers: system roles (Keycloak) → org roles → case roles.

**Permission Model**:
```
User can access case IF:
  system_admin
  OR (
    active member of case.organization_id
    AND (
      org_role IN [owner, admin]    → full case management
      OR has case_role on that case → scoped access
    )
  )
```

**Organization Roles**: `owner` | `admin` | `member`
- **owner**: transfer ownership, manage settings, manage all memberships, manage all org cases
- **admin**: manage memberships (except ownership transfer), manage org cases, assign case participants
- **member**: view org summary, access only assigned cases

---

## Implementation Steps

### Phase 1: Database Schema (Migration 030-033)

#### Step 1.1 — Organizations table
```sql
CREATE TABLE organizations (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          TEXT NOT NULL,
    slug          TEXT UNIQUE,
    description   TEXT DEFAULT '',
    logo_asset_id UUID,
    settings      JSONB NOT NULL DEFAULT '{}',
    created_by    UUID NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ
);
```

#### Step 1.2 — Organization memberships
```sql
CREATE TABLE organization_memberships (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL,
    role            TEXT NOT NULL CHECK (role IN ('owner','admin','member')),
    status          TEXT NOT NULL CHECK (status IN ('active','invited','suspended','removed')),
    joined_at       TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, user_id)
);
CREATE INDEX idx_org_memberships_user ON organization_memberships(user_id, status);
CREATE INDEX idx_org_memberships_org ON organization_memberships(organization_id, status);
```

#### Step 1.3 — Organization invitations
```sql
CREATE TABLE organization_invitations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email           TEXT NOT NULL,
    role            TEXT NOT NULL CHECK (role IN ('admin','member')),
    token_hash      TEXT NOT NULL,
    invited_by      UUID NOT NULL,
    status          TEXT NOT NULL CHECK (status IN ('pending','accepted','declined','expired','revoked')),
    expires_at      TIMESTAMPTZ NOT NULL,
    accepted_by     UUID,
    accepted_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_org_invitations_pending
  ON organization_invitations(organization_id, email)
  WHERE status = 'pending';
```

#### Step 1.4 — User profiles table
```sql
CREATE TABLE user_profiles (
    user_id      UUID PRIMARY KEY,
    display_name TEXT,
    avatar_url   TEXT,
    bio          TEXT DEFAULT '',
    timezone     TEXT DEFAULT 'UTC',
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

#### Step 1.5 — Add organization_id to cases
```sql
ALTER TABLE cases ADD COLUMN organization_id UUID REFERENCES organizations(id);
CREATE INDEX idx_cases_org ON cases(organization_id, status, created_at);
-- Initially nullable for migration compatibility
```

#### Step 1.5b — Update reference_code uniqueness
```sql
-- reference_code is currently UNIQUE globally (migration 001, line 9)
-- Must become unique per-org, not globally
ALTER TABLE cases DROP CONSTRAINT cases_reference_code_key;
-- Add back after org_id is NOT NULL (Phase 6 Step 6.3)
-- ALTER TABLE cases ADD CONSTRAINT cases_reference_code_org_unique UNIQUE (organization_id, reference_code);
```
**CRITICAL**: Without this, two orgs cannot independently use the same reference code format.

#### Step 1.6 — Indexes on case_roles
```sql
CREATE INDEX IF NOT EXISTS idx_case_roles_user ON case_roles(user_id, case_id);
```

---

### Phase 2: Backend Domain Models & Repositories

#### Step 2.1 — Organization domain models
Create `internal/organization/models.go`:
```go
type OrgRole string  // "owner" | "admin" | "member"

type Organization struct {
    ID, Name, Slug, Description, LogoAssetID, Settings, CreatedBy, CreatedAt, UpdatedAt
}

type Membership struct {
    ID, OrganizationID, UserID, Role(OrgRole), Status, JoinedAt
}

type Invitation struct {
    ID, OrganizationID, Email, Role, TokenHash, Status, ExpiresAt, InvitedBy, AcceptedBy
}
```

#### Step 2.2 — Organization repository
Create `internal/organization/repository.go`:
- `Create(ctx, org) error`
- `GetByID(ctx, id) (Organization, error)`
- `Update(ctx, org) error`
- `ListForUser(ctx, userID) ([]Organization, error)`
- `Delete(ctx, id) error` (soft delete)

#### Step 2.3 — Membership repository
Create `internal/organization/membership_repository.go`:
- `GetMembership(ctx, orgID, userID) (Membership, error)`
- `ListMembers(ctx, orgID) ([]Membership, error)`
- `Upsert(ctx, membership) error`
- `Remove(ctx, orgID, userID) error`
- `CountOwners(ctx, orgID) (int, error)`

#### Step 2.4 — Invitation repository
Create `internal/organization/invitation_repository.go`:
- `Create(ctx, invitation) error`
- `GetByTokenHash(ctx, hash) (Invitation, error)`
- `ListByOrg(ctx, orgID) ([]Invitation, error)`
- `MarkAccepted(ctx, inviteID, userID) error`
- `MarkDeclined(ctx, inviteID) error`
- `Revoke(ctx, inviteID) error`

#### Step 2.5 — User profile repository
Create `internal/profile/repository.go`:
- `GetByUserID(ctx, userID) (Profile, error)`
- `Upsert(ctx, profile) error`

---

### Phase 3: Backend Authorization Services

#### Step 3.1 — Organization authorization service
Create `internal/organization/authz.go`:
```go
func (s *OrgAuthzService) RequireOrgRole(ctx, auth, orgID, allowed ...OrgRole) error
  - system_admin bypasses
  - check active membership + role match

func (s *OrgAuthzService) RequireOrgMember(ctx, auth, orgID) error
  - any active member passes
```

#### Step 3.2 — Extended case authorization
Create `internal/cases/authz.go`:
```go
func (s *CaseAuthzService) CanViewCase(ctx, auth, caseID) (bool, error)
  - system_admin → true
  - load case → check org membership
  - org owner/admin → true
  - has case_role → true
  - else → false

func (s *CaseAuthzService) CanManageCase(ctx, auth, caseID) (bool, error)
  - system_admin or org owner/admin only
```

#### Step 3.3 — Update RequireCaseRole middleware
Modify `internal/auth/permissions.go`:
- Add org membership check before case role check
- system_admin still bypasses everything
- Inject org membership loader into middleware chain

---

### Phase 4: Backend Handlers & API Endpoints

#### Step 4.1 — Organization endpoints
Create `internal/organization/handler.go`:
```
POST   /api/organizations                              → Create org (any user)
GET    /api/organizations                              → List user's orgs
GET    /api/organizations/{orgId}                      → Get org detail
PATCH  /api/organizations/{orgId}                      → Update org (owner/admin)
DELETE /api/organizations/{orgId}                      → Soft delete (owner only)
```

#### Step 4.2 — Membership endpoints
```
GET    /api/organizations/{orgId}/members              → List members (any member)
PATCH  /api/organizations/{orgId}/members/{userId}     → Update member role (owner/admin)
DELETE /api/organizations/{orgId}/members/{userId}     → Remove member (owner/admin)
POST   /api/organizations/{orgId}/ownership-transfer   → Transfer ownership (owner only)
```

#### Step 4.3 — Invitation endpoints
```
POST   /api/organizations/{orgId}/invitations          → Send invite (owner/admin)
GET    /api/organizations/{orgId}/invitations           → List pending invites
DELETE /api/organizations/{orgId}/invitations/{id}      → Revoke invite
POST   /api/invitations/accept                          → Accept invite (token in body)
POST   /api/invitations/decline                         → Decline invite (token in body)
```

#### Step 4.4 — User profile endpoints
Create `internal/profile/handler.go`:
```
GET    /api/me                    → Get current user profile + orgs + cases summary
PATCH  /api/me                    → Update profile (display_name, bio, timezone, avatar)
GET    /api/me/organizations      → List user's org memberships
GET    /api/me/cases              → List user's assigned cases across orgs
```

#### Step 4.5 — Organization cases endpoint
```
GET    /api/organizations/{orgId}/cases                → List org's cases (member: assigned only, admin: all)
```

#### Step 4.6 — Case handover endpoint
```
POST   /api/cases/{caseId}/handover                    → Transfer case responsibility
```
Request body:
```json
{
  "from_user_id": "uuid",
  "to_user_id": "uuid",
  "new_roles": ["investigator"],
  "preserve_existing_roles": false,
  "reason": "Reassignment due to caseload"
}
```

#### Step 4.7 — Update case creation
Modify `POST /api/cases` to require `organization_id` in body. Validate creator is active org member.

---

### Phase 5: Invitation Flow

#### Step 5.1 — Token generation
- Generate 32-byte cryptographic random token
- Store SHA-256 hash in DB, never plaintext
- 7-day expiry, single-use

#### Step 5.2 — Email delivery
- SMTP already implemented in `internal/notifications/email.go` (async queue, 256 buffer)
- Reuse existing email infrastructure; add org invitation email template
- Invite link format: `{APP_URL}/invite?token={rawToken}`
- Graceful fallback: in-app notification when SMTP not configured

#### Step 5.3 — Accept flow
1. User clicks invite link → frontend sends `POST /api/invitations/accept` with token
2. Server validates: token exists, status=pending, not expired, user email matches invite email
3. Atomic transaction: create membership + mark invite accepted + audit log
4. Return org details for redirect

---

### Phase 6: Migration Strategy (Existing Data)

#### Step 6.1 — Create default organizations
For each distinct `created_by` in `cases`:
- Create a "Personal Workspace" org
- Add creator as `owner`
- Assign all their created cases to that org

#### Step 6.2 — Backfill case_roles members
For each user in `case_roles` who isn't already an org member:
- Add them as `member` to the case's org

#### Step 6.3 — Enforce NOT NULL and composite unique
After backfill verification:
```sql
ALTER TABLE cases ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE cases ADD CONSTRAINT cases_reference_code_org_unique UNIQUE (organization_id, reference_code);
```

---

### Phase 7: Org-Scope Existing Modules (Critical Security Phase)

This phase retrofits ALL existing modules to enforce org boundaries. **Must complete before enabling org features in production.**

#### Step 7.1 — Update CaseFilter struct
Modify `internal/cases/models.go`:
```go
type CaseFilter struct {
    UserID         string
    OrganizationID string   // NEW: filter by org
    SystemAdmin    bool
    Status         []string
    Jurisdiction   string
    SearchQuery    string
}
```

#### Step 7.2 — Update case repository queries
Modify `internal/cases/repository.go` `FindAll()`:
- Add `organization_id = $X` to WHERE clause when OrganizationID set
- For org admin/owner: return all org cases (not just role-assigned)
- For org member: keep existing case_roles subquery scoped to org

#### Step 7.3 — Update evidence handler
Modify `internal/evidence/handler.go` `loadCallerCaseRole()`:
- After loading case, verify user is active member of `case.OrganizationID`
- Org admin/owner bypasses case role check (can view all org evidence)

#### Step 7.4 — Update search module
Modify `internal/search/case_loader.go`:
- `PGCaseIDsLoader` must include org admin's full case list, not just role-assigned
- Add org membership join to case ID loading query

Modify `internal/search/models.go`:
- Add `OrganizationID string` to `EvidenceSearchDoc`
- Add `organization_id` to MeiliSearch filterable attributes

Modify `internal/search/handler.go`:
- Include `organization_id` filter in search queries

#### Step 7.5 — Update notifications
Modify `internal/notifications/service.go`:
- Notification delivery: only send case notifications to org members
- New notification types: `org_invite_received`, `org_member_joined`, `org_member_removed`

#### Step 7.6 — Update collaboration/WebSocket
Modify WebSocket connection handler:
- On room join: verify user is org member of the case's org
- Reject connection if org membership check fails

#### Step 7.7 — Update witnesses module
- Witness access requires org membership check before case role check
- Document: witness encryption keys are per-case (inherited org scope), no key isolation change needed now

#### Step 7.8 — Update disclosures module
- Disclosure creation: verify both `disclosed_by` and `disclosed_to` are org members
- Cross-org disclosure: explicitly flag and require admin approval (future enhancement)

#### Step 7.9 — Update investigation module
- All investigation endpoints (analysis notes, safety profiles, verification records): add org membership gate
- Safety profiles must not be queryable across org boundaries

#### Step 7.10 — Update reports/export
- `internal/reports/handler.go`: add org membership check
- `internal/cases/export_handler.go`: add org membership check

#### Step 7.11 — Update backup module
- Backup metadata must include `organization_id`
- Restore operation must validate target org matches backup source org

#### Step 7.12 — Update GDPR erasure endpoints
- `POST /api/evidence/{id}/erasure-requests` and `POST /api/erasure-requests/{id}/resolve`
- Both require org membership validation (CaseAdmin + org member)

#### Step 7.13 — Update evidence migration/import
- `POST /api/cases/{caseID}/evidence/import` (ZIP/manifest import)
- `POST /api/cases/{caseID}/migrations`, `GET /api/cases/{caseID}/migrations`
- All case-scoped migration endpoints need org membership gate

#### Step 7.14 — Update case verification
- `GET /api/cases/{id}/verify` and `/verify/status` (currently SystemAdmin-only)
- These are fine as-is (SystemAdmin bypasses org checks), but document the decision

#### Step 7.15 — Update investigation templates
- `GET /api/templates` and `GET /api/templates/{id}` appear to be global (not case-scoped)
- Decision: templates remain global (shared across orgs) OR become org-scoped
- Recommendation: keep global for now; template *instances* are case-scoped and inherit org gate

#### Step 7.16 — MinIO storage path scoping
- Current: flat storage (single bucket, no org prefix)
- Required: add org prefix to new evidence storage keys: `{org_id}/{evidence_id}`
- Migration: existing evidence retains flat keys; add org prefix for new uploads only
- Alternative: defer to bucket-per-org if strict isolation required (future)

#### Step 7.18 — Update background workers
Five background processes need org awareness:

| Worker | File | Interval | Change Required |
|--------|------|----------|-----------------|
| Cleanup outbox | `internal/evidence/cleanup/worker.go:71-85` | 30s ticker | Org-scoped notification delivery; MinIO deletions respect org storage paths |
| TSA retry job | `internal/integrity/tsa_retry.go:68-87` | 5min ticker | Evidence timestamps are case-scoped; inherits org from case — verify no cross-org batch mixing |
| Retention notifier | `cmd/server/main.go:475-501` | Daily | `NotifyExpiringRetention()` must filter by org; send notifications only to org members |
| Collaboration rooms | `internal/collaboration/room.go:84-104` | Autosave ticker | Room membership already case-scoped; add org check on room join (covered in Step 7.6) |
| Email sender | `internal/notifications/email.go:50-59` | Queue drain | Email content must include org-specific context (org name, branding) |

Backup scheduler (`internal/backup/runner.go:304-332`, daily 03:00 UTC) operates on entire database — remains global, no per-org change needed.

#### Step 7.19 — MeiliSearch index org isolation
Current: single index, filtered by `UserCaseIDs` list.
Options:
- **Option A (recommended)**: Add `organization_id` as filterable attribute to existing index. Search queries include `organization_id IN [user's org IDs]` filter. Simpler, works with current MeiliSearch setup.
- **Option B**: Separate index per org (`evidence_{org_id}`). Stronger isolation but operationally complex. Defer unless compliance requires it.

#### Step 7.20 — Org deletion guard
Org soft-deletion (`DELETE /api/organizations/{orgId}`) must enforce:
- All org cases must be `archived` status — reject if any active/closed cases remain
- Legal hold cases block org deletion entirely
- Service-layer check before setting `deleted_at`
- After soft-delete: all org queries exclude `deleted_at IS NOT NULL`

#### Step 7.21 — API key org scoping
- API key requests must include `X-Organization-ID` header
- Validate: key's `user_id` has active membership in specified org
- If header missing and user is in exactly one org: use that org implicitly
- If header missing and user is in multiple orgs: return 400 with "organization required"
- Future: consider adding `organization_id` column to `api_keys` table

#### Step 7.22 — Cross-org case transfer (explicitly out of scope)
- Handover endpoint (`POST /api/cases/{caseId}/handover`) validates both users are in the same org
- Moving a case between orgs is NOT supported in v1
- Document: would require re-scoping evidence storage paths, re-indexing MeiliSearch, custody log entries

#### Step 7.23 — Update case mutation endpoints
These case endpoints currently require `RequireSystemRole(CaseAdmin)` — they need org membership validation added:
- `POST /api/cases/{id}/archive` — org admin/owner only
- `POST /api/cases/{id}/legal-hold` — org admin/owner only
- `PATCH /api/cases/{id}` — org admin/owner or assigned case role with edit permission

#### Step 7.24 — Update evidence sub-endpoints
These evidence endpoints inherit case scope but should be explicitly verified:
- `GET /api/evidence/{id}/versions` — org membership via parent case
- `GET /api/evidence/{id}/thumbnail` — org membership via parent case
- `GET /api/evidence/{id}/page-count` and `/pages/{pageNum}` — org membership via parent case
- `GET /api/evidence/{id}/custody` and `/custody/export` — org membership via parent case
- All inherit from the `loadCallerCaseRole()` check updated in Step 7.3

#### Step 7.25 — Update tag autocomplete
- `GET /api/evidence/tags/autocomplete` — currently global
- Must filter tags to only those used within the user's org's cases
- Without this, tag names from org A leak to org B (information disclosure)

#### Step 7.17 — Keycloak integration decision
- Current: JWT contains only `realm_access.roles` (system roles), no org info
- Decision: org membership stays **purely DB-driven** (no JWT claims for org)
- Rationale: org membership changes frequently; JWT refresh lag would cause stale permissions
- No Keycloak realm config changes needed

---

### Phase 7.5: Frontend Architecture Foundation (before org pages)

This phase establishes the cross-cutting frontend infrastructure needed by ALL subsequent frontend phases.

#### Step 7.5.1 — URL structure decision
**Decision**: Use implicit org context (NOT URL-embedded org ID).
- Current: `/[locale]/(app)/cases/...`
- Keep as-is. Org context comes from org switcher selection, stored in cookie.
- Rationale: embedding orgId in every URL is noisy and breaks existing bookmarks/links.
- Exception: org management pages use `/[locale]/(app)/organizations/[orgId]/...`

#### Step 7.5.2 — OrgContext provider
Create `web/src/components/providers/org-provider.tsx`:
```tsx
// Provides active org to all (app) routes
// Loads user's orgs on mount, sets active org from cookie or first org
// Exposes: activeOrg, setActiveOrg, userOrgs, orgRole, isOrgAdmin
```
Update `web/src/components/providers.tsx` to wrap OrgProvider inside SessionProvider.

#### Step 7.5.3 — useOrg hook
Create `web/src/hooks/use-org.ts`:
```tsx
// Wraps OrgContext for components
// Returns { activeOrg, setActiveOrg, userOrgs, orgRole, isOrgAdmin, isOrgOwner }
```

#### Step 7.5.4 — Update authenticatedFetch
Modify `web/src/lib/api.ts`:
- `authenticatedFetch` must include active org ID as `X-Organization-ID` header or query param
- Backend reads this header to scope queries (or derives from case ownership)

#### Step 7.5.5 — Update AuthGuard
Modify `web/src/components/layout/auth-guard.tsx`:
- After session check, verify user has at least one org membership
- If no orgs: redirect to org creation page (first-time flow)

#### Step 7.5.6 — Update Next.js middleware (optional)
`web/src/middleware.ts` currently handles only i18n.
- Consider: redirect unauthenticated users to login from middleware level
- Org routing: not needed if using implicit context

---

### Phase 8: Frontend — Organization Management (was Phase 7)

#### Step 8.1 — API client functions
Add to `web/src/lib/org-api.ts`:
```ts
createOrganization(data) → Organization
getOrganizations() → Organization[]
getOrganization(id) → Organization
updateOrganization(id, data) → Organization
getOrgMembers(orgId) → Member[]
inviteMember(orgId, email, role) → Invitation
acceptInvitation(token) → Organization
getOrgCases(orgId) → Case[]
```

#### Step 8.2 — TypeScript types
Add to `web/src/types/index.ts`:
```ts
interface Organization { id, name, slug, description, logoUrl, memberCount, caseCount, role }
interface OrgMembership { id, organizationId, userId, role, status, joinedAt, displayName, email }
interface OrgInvitation { id, email, role, status, expiresAt, invitedBy }
```

#### Step 8.3 — Org creation page
Create `web/src/app/[locale]/(app)/organizations/new/page.tsx`:
- Form: name, description, logo upload
- On submit → create org → redirect to org dashboard

#### Step 8.4 — Org dashboard/profile page
Create `web/src/app/[locale]/(app)/organizations/[orgId]/page.tsx`:
- Header: org name, description, logo, edit button (owner/admin)
- Stats bar: member count, case count, active cases
- Tabs: Members | Cases | Settings (owner/admin only)
- Member list with role badges, invite button
- Case list (recent cases in this org)

#### Step 8.5 — Org member management
Create `web/src/components/organizations/member-management.tsx`:
- Member table: avatar, name, email, role badge, joined date, actions dropdown
- Invite modal: email input, role selector (admin/member), send button
- Role change dropdown (owner/admin can change member→admin, admin→member)
- Remove member with confirmation dialog
- Pending invitations list with revoke option

#### Step 8.6 — Org switcher
Create `web/src/components/layout/org-switcher.tsx`:
- Dropdown in sidebar header showing current org
- List of user's orgs with role indicator
- "Create organization" option at bottom
- Selected org stored in cookie (synced with OrgContext provider)
- Changing org updates case list filter

#### Step 8.7 — Invitation acceptance page
Create `web/src/app/[locale]/(app)/invite/page.tsx`:
- Reads `?token=` from URL
- Shows org name, inviter, role offered
- Accept / Decline buttons
- Success → redirect to org dashboard

---

### Phase 9: Frontend — Case Permission Management

#### Step 9.1 — Case member panel
Create `web/src/components/cases/case-members-panel.tsx`:
- Side panel or tab within case detail page
- List current case members with role badges
- Add member button → modal to select from org members
- Role selector for new member (6 case roles)
- Remove member button with confirmation

#### Step 9.2 — Case handover dialog
Create `web/src/components/cases/case-handover-dialog.tsx`:
- Select target user from org members
- Select new role(s) for target
- Checkbox: preserve existing roles for source user
- Reason text input (required for audit)
- Confirmation step showing summary

#### Step 9.3 — Permission indicators
Update `web/src/components/cases/case-detail.tsx`:
- Show user's role badge on case header
- "Members" count badge linking to members panel
- Visual indicator for org admin vs case role access

#### Step 9.4 — Update case list
Update `web/src/app/[locale]/(app)/cases/page.tsx`:
- Filter cases by active org from OrgContext
- Show org name on case cards
- Role badge per case in list view

---

### Phase 10: Frontend — User Profile

#### Step 10.1 — Profile page
Create `web/src/app/[locale]/(app)/profile/page.tsx`:
- Header: avatar (with upload), display name, email, system role badge
- Sections:
  - **My Organizations**: list with role, member count, link to org
  - **My Cases**: grouped by org, showing case title + role
  - **Activity**: recent audit log entries (optional, Phase 2)

#### Step 10.2 — Profile edit form
Create `web/src/components/profile/profile-form.tsx`:
- Display name input
- Bio textarea
- Timezone selector
- Avatar upload with preview
- Save button with optimistic update

#### Step 10.3 — Profile menu
Update `web/src/components/layout/` sidebar:
- User avatar + name at bottom of sidebar
- Click → dropdown: Profile, Settings, Sign out
- Currently just has sign out

#### Step 10.4 — Merge with existing settings page
- Existing `/[locale]/(app)/settings/page.tsx` may overlap with profile
- Decision: profile page = user identity/orgs/cases; settings page = preferences/API keys
- Link between them in navigation

---

### Phase 11: Frontend — Navigation & UX Updates

#### Step 11.1 — Sidebar updates
- Add "Organizations" nav item (building icon)
- Add org switcher to sidebar header
- Profile link at sidebar bottom

#### Step 11.2 — Breadcrumb integration
Update breadcrumbs to show: Org Name → Cases → Case Title

#### Step 11.3 — Empty states
- New org: "No cases yet. Create your first case."
- New org members: "Invite team members to collaborate."
- No org: "Create an organization to get started."
- First-time user: onboarding flow → create org → invite members → create first case

#### Step 11.4 — i18n messages
Add to `en.json` and `fr.json`:
- `organizations.*` namespace (~40 keys)
- `profile.*` namespace (~20 keys)
- `invitations.*` namespace (~15 keys)
- `caseMembers.*` namespace (~15 keys)

---

### Phase 12: Audit Trail Integration

#### Step 12.1 — New audit event types
Add to existing auth_audit_log pattern:
```
organization_created, organization_updated, organization_deleted
member_invited, member_joined, member_role_changed, member_removed
invitation_accepted, invitation_declined, invitation_revoked
ownership_transferred
case_handover
```

#### Step 12.2 — Custody log integration
Case handovers get custody_log entries (immutable chain).

---

### Phase 13: Testing

#### Step 13.1 — Backend unit tests
- Org authorization matrix (all role combinations)
- Invite token validation (expired, wrong email, reuse)
- Ownership transfer invariants (cannot remove last owner)
- Case handover transaction semantics
- Permission boundary: org A member cannot see org B cases

#### Step 13.2 — Backend integration tests
- Full invite → accept → case access flow
- Member removal cascades case role revocation
- Org admin sees all org cases, member sees only assigned
- System admin bypasses all org checks

#### Step 13.3 — Cross-org isolation tests (CRITICAL)
- User in org A cannot list org B cases via `/api/cases`
- User in org A cannot access org B case via `/api/cases/{id}`
- User in org A cannot search org B evidence via MeiliSearch
- User in org A cannot access org B witnesses, disclosures, investigation data
- User in org A cannot join org B's WebSocket collaboration room
- User in org A cannot export org B case
- User in org A cannot generate org B report
- Removed org member immediately loses all case access
- Expired invitation cannot be accepted
- Invitation for `alice@example.com` cannot be accepted by `bob@example.com`
- Concurrent invite acceptance for same email doesn't create duplicate membership
- Org with active/legal-hold cases cannot be soft-deleted
- Case handover rejects if target user is in a different org
- API key request without `X-Organization-ID` fails for multi-org users
- Tag autocomplete only returns tags from current org's cases
- Soft-deleted org's cases are inaccessible to former members

#### Step 13.4 — Frontend tests
- Org creation flow
- Member invite and management
- Case member panel interactions
- Profile edit form
- Org switcher state management
- Org-scoped case list filtering
- Search results respect org boundaries

---

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `migrations/030_organizations.up.sql` | Create | Organizations + memberships + invitations tables |
| `migrations/031_user_profiles.up.sql` | Create | User profiles table |
| `migrations/032_cases_org_id.up.sql` | Create | Add organization_id to cases |
| `migrations/033_backfill_orgs.up.sql` | Create | Backfill default orgs for existing data |
| `internal/organization/models.go` | Create | Organization, Membership, Invitation models |
| `internal/organization/repository.go` | Create | Org CRUD repository |
| `internal/organization/membership_repository.go` | Create | Membership repository |
| `internal/organization/invitation_repository.go` | Create | Invitation repository |
| `internal/organization/service.go` | Create | Org business logic + invitation flow |
| `internal/organization/authz.go` | Create | Org authorization service |
| `internal/organization/handler.go` | Create | HTTP handlers for org endpoints |
| `internal/profile/models.go` | Create | User profile model |
| `internal/profile/repository.go` | Create | Profile CRUD |
| `internal/profile/handler.go` | Create | Profile HTTP handlers |
| `internal/cases/authz.go` | Create | Org-aware case authorization |
| `internal/auth/permissions.go` | Modify | Extend middleware for org context |
| `internal/cases/handler.go` | Modify | Require org_id on case creation |
| `internal/cases/repository.go` | Modify | Org-scoped case queries |
| `cmd/server/main.go` | Modify | Wire new services + routes |
| `web/src/types/index.ts` | Modify | Add Organization, Membership, Invitation types |
| `web/src/lib/org-api.ts` | Create | Organization API client |
| `web/src/app/[locale]/(app)/organizations/` | Create | Org pages (list, detail, new) |
| `web/src/app/[locale]/(app)/profile/page.tsx` | Create | User profile page |
| `web/src/app/[locale]/(app)/invite/page.tsx` | Create | Invitation acceptance page |
| `web/src/components/organizations/` | Create | Org management components |
| `web/src/components/cases/case-members-panel.tsx` | Create | Case member management |
| `web/src/components/cases/case-handover-dialog.tsx` | Create | Case handover flow |
| `web/src/components/layout/org-switcher.tsx` | Create | Org context switcher |
| `web/src/components/profile/profile-form.tsx` | Create | Profile edit form |
| `web/src/messages/en.json` | Modify | Add ~90 i18n keys |
| `web/src/messages/fr.json` | Modify | Add ~90 i18n keys |
| **Org-scoping existing modules (Phase 7)** | | |
| `internal/cases/models.go` | Modify | Add OrganizationID to CaseFilter |
| `internal/cases/repository.go` | Modify | Org-scoped case queries in FindAll |
| `internal/evidence/handler.go` | Modify | Add org membership check to loadCallerCaseRole |
| `internal/evidence/bulk_repository.go` | Modify | Org validation on bulk operations |
| `internal/evidence/redaction.go` | Modify | Org check on WebSocket room join |
| `internal/search/case_loader.go` | Modify | Include org admin full case list |
| `internal/search/models.go` | Modify | Add organization_id to EvidenceSearchDoc |
| `internal/search/handler.go` | Modify | Org filter in search queries |
| `internal/notifications/service.go` | Modify | Org-scoped notification delivery |
| `internal/witnesses/handler.go` | Modify | Org membership gate |
| `internal/disclosures/handler.go` | Modify | Org membership validation |
| `internal/investigation/handler.go` | Modify | Org membership gate on all endpoints |
| `internal/reports/handler.go` | Modify | Org membership check |
| `internal/cases/export_handler.go` | Modify | Org membership check |
| `internal/backup/` | Modify | Org isolation on backup/restore |
| `web/src/components/search/search-filters.tsx` | Modify | Add org filter option |
| `web/src/app/[locale]/(app)/cases/page.tsx` | Modify | Pass org context to API |
| `web/src/app/[locale]/(app)/cases/new/page.tsx` | Modify | Require org selection |
| `internal/evidence/storage.go` | Modify | Org-prefixed storage keys |
| `internal/notifications/email.go` | Modify | Add org invitation email template |
| **Frontend architecture (Phase 7.5)** | | |
| `web/src/components/providers/org-provider.tsx` | Create | OrgContext provider for active org state |
| `web/src/components/providers.tsx` | Modify | Wrap OrgProvider inside SessionProvider |
| `web/src/hooks/use-org.ts` | Create | Hook for accessing active org context |
| `web/src/lib/api.ts` | Modify | Add org header to authenticatedFetch |
| `web/src/components/layout/auth-guard.tsx` | Modify | Add org membership check + first-time redirect |
| **Schema changes** | | |
| `migrations/030` | Create | Drop global reference_code unique, prep for composite |

---

## Modules Requiring Org-Scoping (Audit Checklist)

Every module that touches cases must be updated to enforce org boundaries. Missing any one of these creates a cross-org data leakage vulnerability.

### Backend Modules

| Module | File(s) | Current Access Check | Required Change |
|--------|---------|---------------------|-----------------|
| **Cases CRUD** | `internal/cases/handler.go:28-34` | `RequireSystemRole(CaseAdmin)` | Add org membership check; `CaseFilter` needs `OrganizationID` field |
| **Cases Repository** | `internal/cases/repository.go:86-160` | `FindAll` uses `case_roles` subquery | Add `organization_id` to WHERE clause; join `org_memberships` |
| **Case Roles** | `internal/cases/roles.go:124-130` | `RequireSystemRole(CaseAdmin)` | Verify target user is same-org member before assigning |
| **Case Export** | `internal/cases/export_handler.go:28` | `RequireSystemRole(CaseAdmin)` | Add org membership validation |
| **Evidence** | `internal/evidence/handler.go:72-92` | `loadCallerCaseRole()` → `caseRoleLoader` | Add org membership check before case role check |
| **Evidence Bulk** | `internal/evidence/bulk_repository.go` | Case-scoped queries | Ensure org validation on bulk operations |
| **Evidence Redaction** | `internal/evidence/redaction.go` | Case-scoped via collaboration rooms | WebSocket rooms must validate org membership on connect |
| **Search/MeiliSearch** | `internal/search/handler.go:60-97` | Filters by `UserCaseIDs` | Extend to filter by org's case IDs; add `organization_id` to search docs |
| **Search Case Loader** | `internal/search/case_loader.go:33-57` | `SELECT case_id FROM case_roles` | Include org admin/owner access (all org cases, not just role-assigned) |
| **Notifications** | `internal/notifications/service.go` | Case-scoped events | Only deliver to org members; filter notification queries by org |
| **Witnesses** | `internal/witnesses/` | Case-scoped | Org membership required before witness access; key isolation per org |
| **Disclosures** | `internal/disclosures/` | Case-scoped | Validate org membership; cross-org disclosure needs explicit approval |
| **Investigation** | `internal/investigation/pg_repository.go` | Case-scoped | Analysis notes, safety profiles, verification records all need org gate |
| **Reports/PDF** | `internal/reports/handler.go:28-29` | `RequireSystemRole` | Add org membership validation |
| **Custody Log** | `internal/custody/` + RLS in migration 003 | RLS on `custody_log` | Review RLS policy; may need org context in session vars |
| **Collaboration/WebSocket** | `cmd/server/main.go:70` | Uses `caseRoleLoader` | Add org membership check on WebSocket connection upgrade |
| **Backup/Restore** | `internal/backup/` | Case-level encryption | Org isolation during restore; prevent cross-org data injection |
| **API Keys** | DB table `api_keys` | User-scoped | Consider org-scoped keys in future; document limitation |
| **Safety Profiles** | `internal/investigation/` (migration 029) | Case + user scoped | Org isolation required; safety data cannot leak across orgs |
| **Integrity/TSA** | `internal/integrity/` | Evidence-scoped | Inherits case org scope; no direct change needed |
| **GDPR Erasure** | `internal/evidence/handler.go` | `RequireSystemRole(CaseAdmin)` | Add org membership check on erasure request/resolve |
| **Evidence Import** | `internal/evidence/` (import handler) | Case-scoped | Org membership gate on ZIP/manifest import |
| **Case Migrations** | `internal/migration/` | Case-scoped | Org membership gate on migration endpoints |
| **Investigation Templates** | `internal/investigation/` | Global (no case scope) | Keep global; template *instances* are case-scoped |
| **MinIO Storage** | `internal/evidence/storage.go` | Flat bucket | Add org prefix to new storage keys |
| **Case Archive** | `internal/cases/handler.go:33` | `RequireSystemRole(CaseAdmin)` | Add org admin/owner check |
| **Case Legal Hold** | `internal/cases/handler.go:34` | `RequireSystemRole(CaseAdmin)` | Add org admin/owner check |
| **Tag Autocomplete** | `internal/evidence/handler.go` | Global (no case scope) | Filter to org's cases only — tag names are information |
| **Evidence Sub-endpoints** | `internal/evidence/handler.go` | `loadCallerCaseRole()` | Inherits from Step 7.3 update; verify all paths |
| **Cleanup Worker** | `internal/evidence/cleanup/worker.go` | Background goroutine | Verify no cross-org batch mixing |
| **TSA Retry Job** | `internal/integrity/tsa_retry.go` | Background goroutine | Verify org isolation in batch processing |
| **Retention Notifier** | `cmd/server/main.go:475-501` | Daily background job | Filter by org; notify only org members |

### Frontend Modules

| Module | File(s) | Required Change |
|--------|---------|-----------------|
| **Case List** | `web/src/app/[locale]/(app)/cases/page.tsx:44` | Pass org context to API; show org badge on cases |
| **Case Detail** | `web/src/app/[locale]/(app)/cases/[id]/page.tsx` | Show org name in breadcrumb; members panel |
| **Evidence Page** | `web/src/app/[locale]/(app)/cases/[id]/evidence/page.tsx` | Inherit org context from case |
| **Evidence Detail** | `web/src/app/[locale]/(app)/evidence/[id]/page.tsx` | Verify org context |
| **Search Filters** | `web/src/components/search/search-filters.tsx` | Add org filter option |
| **Sidebar** | `web/src/components/layout/` | Add org switcher, org nav item, profile link |
| **Evidence Uploader** | `web/src/components/evidence/evidence-uploader.tsx` | Org context for case selection |
| **MeiliSearch Models** | `internal/search/models.go` | Add `organization_id` to `EvidenceSearchDoc` filterable attributes |
| **Investigation Page** | `web/src/app/[locale]/(app)/cases/[id]/investigation/page.tsx` | Inherit org context from case |
| **Witness Detail** | `web/src/app/[locale]/(app)/witnesses/[id]/page.tsx` | Verify org context |
| **Disclosure Detail** | `web/src/app/[locale]/(app)/disclosures/[id]/page.tsx` | Verify org context |
| **Report Detail** | `web/src/app/[locale]/(app)/reports/[id]/page.tsx` | Verify org context |
| **Investigation Detail Pages** | `inquiry-logs/[id]`, `assessments/[id]`, `verifications/[id]`, `corroborations/[id]`, `analysis-notes/[id]` | All inherit org context via parent case |
| **Case Settings** | `web/src/app/[locale]/(app)/cases/[id]/settings/page.tsx` | Org admin/owner access for case settings |
| **User Settings** | `web/src/app/[locale]/(app)/settings/page.tsx` | Already exists; integrate with profile page or merge |
| **Case New** | `web/src/app/[locale]/(app)/cases/new/page.tsx` | Require org selection before case creation |

### Critical Security Invariants

1. **Every case query MUST include org scope** — no unscoped `SELECT * FROM cases`
2. **Search index MUST filter by org** — MeiliSearch `UserCaseIDs` must include org admin's full case list
3. **WebSocket collaboration MUST validate org** — real-time redaction rooms cannot expose cross-org data
4. **Member removal MUST cascade** — removing user from org revokes ALL case_roles in that org atomically
5. **Witness encryption keys MUST be org-isolated** — compromised key in org A cannot decrypt org B witnesses
6. **Backup restore MUST validate org** — cannot restore org A's backup into org B's namespace

---

## Risks and Mitigation

| Risk | Mitigation |
|------|------------|
| Tenant data leakage (org A sees org B cases) | Centralize auth checks in CaseAuthzService; add org_id filter to ALL case queries; integration tests for cross-org isolation |
| Invite token abuse (guessing, reuse, stale) | SHA-256 hashed tokens, 7-day expiry, single-use, email match validation |
| Last owner removal leaves org ownerless | DB/service invariant: org must always have ≥1 active owner; check in removal transaction |
| Role drift (removed org member retains case_roles) | Cascade: remove all case_roles for user in org within same transaction as membership removal |
| Migration breaks existing case access | Phased: nullable org_id first → backfill → enforce NOT NULL; compatibility mode during rollout |
| Case listing performance with org joins | Indexes on cases(organization_id, status), org_memberships(user_id, status), case_roles(case_id, user_id) |
| Privilege escalation (org admin bypasses case-role restrictions) | Clear capability matrix: org admin can manage membership + view cases, but case-specific actions (evidence redaction) still require case role |
| MinIO storage flat namespace allows cross-org key guessing | Add org prefix to storage keys; validate org ownership before serving downloads |
| No rate limiting on org endpoints | Add rate limiting middleware to invite, create org, and membership endpoints |
| Keycloak JWT stale org claims | Keep org membership DB-driven only; no JWT claims for org membership |
| GDPR erasure crosses org boundary | Erasure handler must validate org membership before processing |
| Evidence import injects data into wrong org | Validate target case's org matches importer's active org membership |
| `cases.reference_code` globally unique blocks multi-org | Drop global unique, add composite `UNIQUE (organization_id, reference_code)` |
| Background workers process cross-org data in batches | Verify cleanup/TSA/retention workers don't mix org data in batch operations |
| First-time user has no org → blank app | AuthGuard redirects to org creation; onboarding empty state |
| Concurrent invite acceptance creates duplicate membership | `UNIQUE (organization_id, user_id)` on memberships catches it; handle conflict with upsert |
| Org soft-deletion with legal hold cases still attached | Require all cases archived/transferred before org deletion; validate in service layer |
| Cross-org case transfer not addressed | Explicitly out of scope v1; handover endpoint rejects if users in different orgs |
| API key ambiguity for multi-org users | Require `X-Organization-ID` header on API key requests; validate key's user is member of specified org |
| Pagination cursors break with org filter | Cursor format (UUID-based) is orthogonal to org filter; no issue — org_id is additive WHERE clause |

---

## Suggested Sprint Breakdown

**Sprint A (Foundation)**: Phases 1-3 — Schema, models, repositories, authorization services
**Sprint B (Backend API)**: Phases 4-5 — Handlers, invitation flow, case handover
**Sprint C (Migration + Security Retrofit)**: Phases 6-7 — Data migration, backfill, org-scope ALL existing modules (CRITICAL)
**Sprint D (Frontend Org)**: Phases 8, 11 — Org pages, switcher, navigation, i18n
**Sprint E (Frontend Cases + Profile)**: Phases 9-10 — Case members, handover, profile
**Sprint F (Testing + Audit)**: Phases 12-13 — Audit events, comprehensive testing including cross-org isolation tests

---

## SESSION_ID (for /ccg:execute use)
- CODEX_SESSION: codex-1776088551-32370
- GEMINI_SESSION: 15 (failed — quota exhausted, frontend plan synthesized by Claude)
