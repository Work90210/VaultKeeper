'use client';

import { useState, useEffect, useMemo } from 'react';
import { useRouter } from 'next/navigation';
import type { CaseRole, OrgMembership, RoleDefinition } from '@/types';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

interface Props {
  caseId: string;
  members: CaseRole[];
  canManage: boolean;
  organizationId?: string;
  accessToken?: string;
}

export function CaseMembersPanel({ caseId, members, canManage, organizationId, accessToken }: Props) {
  const [showAdd, setShowAdd] = useState(false);

  return (
    <div className="stagger-in">
      <div className="flex items-center justify-between" style={{ marginBottom: 'var(--space-sm)' }}>
        <h2 className="field-label" style={{ marginBottom: 0 }}>Case Members</h2>
        {canManage && (
          <button onClick={() => setShowAdd(!showAdd)} className="btn-ghost" type="button">
            {showAdd ? 'Cancel' : '+ Add member'}
          </button>
        )}
      </div>

      {showAdd && (
        <AddMemberForm
          caseId={caseId}
          existingMembers={members}
          organizationId={organizationId}
          accessToken={accessToken}
          onDone={() => setShowAdd(false)}
        />
      )}

      {members.length === 0 ? (
        <div className="card-inset" style={{ padding: 'var(--space-lg)', textAlign: 'center' }}>
          <p style={{ fontSize: 'var(--text-sm)', color: 'var(--text-tertiary)' }}>No members assigned.</p>
        </div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-xs)' }}>
          {members.map((m) => (
            <MemberItem key={m.id} member={m} caseId={caseId} canManage={canManage} accessToken={accessToken} />
          ))}
        </div>
      )}
    </div>
  );
}

function MemberItem({ member, caseId, canManage, accessToken }: { member: CaseRole; caseId: string; canManage: boolean; accessToken?: string }) {
  const router = useRouter();
  const [removing, setRemoving] = useState(false);

  async function handleRemove() {
    if (!confirm('Remove this member from the case?')) return;
    setRemoving(true);
    try {
      await fetch(`${API_BASE}/api/cases/${caseId}/roles/${member.id}`, {
        method: 'DELETE',
        headers: accessToken ? { Authorization: `Bearer ${accessToken}` } : {},
      });
      router.refresh();
    } finally {
      setRemoving(false);
    }
  }

  return (
    <div
      className="card-inset flex items-center justify-between"
      style={{ padding: 'var(--space-xs) var(--space-sm)' }}
    >
      <div className="flex items-center" style={{ gap: 'var(--space-sm)' }}>
        <span
          className="badge"
          style={{
            backgroundColor: 'var(--amber-subtle)',
            color: 'var(--amber-accent)',
          }}
        >
          {member.role.replace('_', ' ')}
        </span>
        <span
          className="font-[family-name:var(--font-mono)]"
          style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)' }}
        >
          {member.user_id.slice(0, 8)}&hellip;
        </span>
      </div>
      {canManage && (
        <button
          onClick={handleRemove}
          disabled={removing}
          type="button"
          style={{
            fontSize: 'var(--text-xs)',
            fontWeight: 500,
            color: 'var(--status-hold)',
            background: 'none',
            border: 'none',
            cursor: 'pointer',
            opacity: removing ? 0.4 : 1,
          }}
        >
          Remove
        </button>
      )}
    </div>
  );
}

function AddMemberForm({
  caseId,
  existingMembers,
  organizationId,
  accessToken,
  onDone,
}: {
  caseId: string;
  existingMembers: CaseRole[];
  organizationId?: string;
  accessToken?: string;
  onDone: () => void;
}) {
  const router = useRouter();
  const [selectedUserId, setSelectedUserId] = useState('');
  const [roleDefId, setRoleDefId] = useState('');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [orgMembers, setOrgMembers] = useState<OrgMembership[]>([]);
  const [roleDefs, setRoleDefs] = useState<RoleDefinition[]>([]);
  const [loadingMembers, setLoadingMembers] = useState(false);

  // Fetch org members and role definitions when the form opens
  useEffect(() => {
    if (!organizationId || !accessToken) return;
    setLoadingMembers(true);
    const headers = { Authorization: `Bearer ${accessToken}` };
    Promise.all([
      fetch(`${API_BASE}/api/organizations/${organizationId}/members`, { headers })
        .then((res) => (res.ok ? res.json() : null))
        .then((body) => {
          const members: OrgMembership[] = Array.isArray(body) ? body : (body?.data ?? []);
          setOrgMembers(members);
        }),
      fetch(`${API_BASE}/api/organizations/${organizationId}/role-definitions`, { headers })
        .then((res) => (res.ok ? res.json() : null))
        .then((body) => {
          const defs: RoleDefinition[] = Array.isArray(body) ? body : (body?.data ?? []);
          setRoleDefs(defs);
          if (defs.length > 0 && !roleDefId) {
            setRoleDefId(defs[0].id);
          }
        }),
    ])
      .catch(() => { setOrgMembers([]); })
      .finally(() => { setLoadingMembers(false); });
  }, [organizationId, accessToken, roleDefId]);

  // Filter out users already assigned to the case
  const existingUserIds = useMemo(
    () => new Set(existingMembers.map((m) => m.user_id)),
    [existingMembers]
  );

  const availableMembers = useMemo(
    () => orgMembers.filter((m) => m.status === 'active' && !existingUserIds.has(m.user_id)),
    [orgMembers, existingUserIds]
  );

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!selectedUserId) return;
    setSubmitting(true);
    setError('');
    try {
      const res = await fetch(`${API_BASE}/api/cases/${caseId}/roles`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}),
        },
        body: JSON.stringify({
          user_id: selectedUserId,
          role: roleDefs.find((d) => d.id === roleDefId)?.slug ?? 'investigator',
          role_definition_id: roleDefId,
        }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => null);
        setError(body?.error ?? 'Failed to add member');
        return;
      }
      router.refresh();
      onDone();
    } catch {
      setError('Network error');
    } finally {
      setSubmitting(false);
    }
  }

  const hasOrgContext = Boolean(organizationId && accessToken);

  return (
    <form
      onSubmit={handleSubmit}
      className="card-inset"
      style={{ padding: 'var(--space-sm)', marginBottom: 'var(--space-sm)' }}
    >
      <div className="flex items-end" style={{ gap: 'var(--space-xs)' }}>
        <div className="flex-1">
          <label className="field-label">Member</label>
          {hasOrgContext ? (
            loadingMembers ? (
              <div
                className="input-field flex items-center"
                style={{ color: 'var(--text-tertiary)', fontSize: 'var(--text-sm)' }}
              >
                Loading members...
              </div>
            ) : availableMembers.length === 0 ? (
              <div
                className="input-field flex items-center"
                style={{ color: 'var(--text-tertiary)', fontSize: 'var(--text-sm)' }}
              >
                {orgMembers.length === 0
                  ? 'No organization members found'
                  : 'All organization members already assigned'}
              </div>
            ) : (
              <select
                value={selectedUserId}
                onChange={(e) => setSelectedUserId(e.target.value)}
                className="input-field"
              >
                <option value="">Select a member...</option>
                {availableMembers.map((m) => (
                  <option key={m.user_id} value={m.user_id}>
                    {m.display_name || m.email || m.user_id.slice(0, 8)}
                    {m.email && m.display_name ? ` (${m.email})` : ''}
                  </option>
                ))}
              </select>
            )
          ) : (
            <input
              type="text"
              value={selectedUserId}
              onChange={(e) => setSelectedUserId(e.target.value)}
              placeholder="User ID"
              className="input-field"
            />
          )}
        </div>
        <div>
          <label className="field-label">Role</label>
          <select
            value={roleDefId}
            onChange={(e) => setRoleDefId(e.target.value)}
            className="input-field"
          >
            {roleDefs.map((rd) => (
              <option key={rd.id} value={rd.id}>
                {rd.name}
              </option>
            ))}
          </select>
        </div>
        <button
          type="submit"
          disabled={submitting || !selectedUserId}
          className="btn-primary"
        >
          {submitting ? 'Adding\u2026' : 'Add'}
        </button>
      </div>
      {error && (
        <div className="banner-error" style={{ marginTop: 'var(--space-xs)' }}>
          {error}
        </div>
      )}
    </form>
  );
}
