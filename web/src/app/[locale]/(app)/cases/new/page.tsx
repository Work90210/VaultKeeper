'use client';

import { useRouter } from 'next/navigation';
import { useSession } from 'next-auth/react';
import { useState, useEffect, useMemo, useCallback } from 'react';
import { Shell } from '@/components/layout/shell';
import { useOrg } from '@/hooks/use-org';
import type { OrgMembership, RoleDefinition } from '@/types';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

interface PendingMember {
  userId: string;
  displayName: string;
  email: string;
  roleSlug: string;
  roleName: string;
  roleDefId: string;
}

export default function NewCasePage() {
  const router = useRouter();
  const { data: session } = useSession();
  const { activeOrg } = useOrg();
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  // Org members and role definitions for the picker
  const [orgMembers, setOrgMembers] = useState<OrgMembership[]>([]);
  const [roleDefs, setRoleDefs] = useState<RoleDefinition[]>([]);
  const [membersLoading, setMembersLoading] = useState(false);

  // Members to assign after creation
  const [pendingMembers, setPendingMembers] = useState<PendingMember[]>([]);
  const [pickUserId, setPickUserId] = useState('');
  const [pickRoleDefId, setPickRoleDefId] = useState('');

  const authHeaders = useMemo((): Record<string, string> => {
    if (!session?.accessToken) return { 'Content-Type': 'application/json' };
    return {
      Authorization: `Bearer ${session.accessToken}`,
      'Content-Type': 'application/json',
    };
  }, [session?.accessToken]);

  const fetchOrgMembers = useCallback(async () => {
    if (!activeOrg?.id || !session?.accessToken) return;
    setMembersLoading(true);
    const headers = { Authorization: `Bearer ${session.accessToken}` };
    try {
      const [membersRes, roleDefsRes] = await Promise.all([
        fetch(`${API_BASE}/api/organizations/${activeOrg.id}/members`, { headers }),
        fetch(`${API_BASE}/api/organizations/${activeOrg.id}/role-definitions`, { headers }),
      ]);
      if (membersRes.ok) {
        const body = await membersRes.json();
        setOrgMembers(Array.isArray(body) ? body : (body.data ?? []));
      }
      if (roleDefsRes.ok) {
        const body = await roleDefsRes.json();
        const defs: RoleDefinition[] = Array.isArray(body) ? body : (body.data ?? []);
        setRoleDefs(defs);
        if (defs.length > 0 && !pickRoleDefId) {
          setPickRoleDefId(defs[0].id);
        }
      }
    } catch {
      /* empty */
    } finally {
      setMembersLoading(false);
    }
  }, [activeOrg?.id, session?.accessToken, pickRoleDefId]);

  useEffect(() => {
    fetchOrgMembers();
  }, [fetchOrgMembers]);

  const pendingUserIds = useMemo(() => new Set(pendingMembers.map((m) => m.userId)), [pendingMembers]);

  const availableMembers = useMemo(
    () => orgMembers.filter((m) => !pendingUserIds.has(m.user_id)),
    [orgMembers, pendingUserIds],
  );

  const handleAddMember = () => {
    if (!pickUserId || !pickRoleDefId) return;
    const member = orgMembers.find((m) => m.user_id === pickUserId);
    const roleDef = roleDefs.find((d) => d.id === pickRoleDefId);
    if (!member || !roleDef) return;
    setPendingMembers((prev) => [
      ...prev,
      {
        userId: member.user_id,
        displayName: member.display_name || member.email || member.user_id.slice(0, 8),
        email: member.email || '',
        roleSlug: roleDef.slug,
        roleName: roleDef.name,
        roleDefId: roleDef.id,
      },
    ]);
    setPickUserId('');
  };

  const handleRemovePending = (userId: string) => {
    setPendingMembers((prev) => prev.filter((m) => m.userId !== userId));
  };

  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setError('');

    if (!activeOrg) {
      setError('No organization selected. Switch to an organization first.');
      return;
    }

    setLoading(true);

    const formData = new FormData(e.currentTarget);
    const body = {
      organization_id: activeOrg.id,
      reference_code: formData.get('reference_code'),
      title: formData.get('title'),
      description: formData.get('description'),
      jurisdiction: formData.get('jurisdiction'),
    };

    try {
      // 1. Create the case
      const res = await fetch(`${API_BASE}/api/cases`, {
        method: 'POST',
        headers: authHeaders,
        body: JSON.stringify(body),
      });

      const data = await res.json();
      if (!res.ok) {
        setError(data.error || 'Failed to create case');
        setLoading(false);
        return;
      }

      const caseId = data.data?.id ?? data.id;

      // 2. Assign members in parallel
      if (pendingMembers.length > 0) {
        const assignments = pendingMembers.map((m) =>
          fetch(`${API_BASE}/api/cases/${caseId}/roles`, {
            method: 'POST',
            headers: authHeaders,
            body: JSON.stringify({
              user_id: m.userId,
              role: m.roleSlug,
              role_definition_id: m.roleDefId,
            }),
          }),
        );
        await Promise.allSettled(assignments);
      }

      router.push(`/en/cases/${caseId}`);
    } catch {
      setError('Failed to create case');
      setLoading(false);
    }
  };

  return (
    <Shell>
      <div className="max-w-2xl mx-auto px-[var(--space-lg)] py-[var(--space-xl)]">
        <a
          href="/en/cases"
          className="link-subtle text-xs uppercase tracking-wider font-medium mb-[var(--space-lg)] inline-block"
        >
          &larr; Cases
        </a>

        <h1
          className="font-[family-name:var(--font-heading)] text-2xl mb-[var(--space-lg)]"
          style={{ color: 'var(--text-primary)' }}
        >
          New Case
        </h1>

        {error && <div className="banner-error mb-[var(--space-md)]">{error}</div>}

        <div className="card p-[var(--space-lg)]">
          <form onSubmit={handleSubmit} className="space-y-[var(--space-lg)]">
            {/* ── Case Details ── */}
            <div>
              <h2
                style={{
                  fontSize: 'var(--text-base)',
                  fontWeight: 600,
                  color: 'var(--text-primary)',
                  marginBottom: 'var(--space-md)',
                }}
              >
                Case details
              </h2>
              <div className="space-y-[var(--space-md)]">
                <FormField
                  label="Reference Code"
                  name="reference_code"
                  required
                  placeholder="ICC-01/04-01/06"
                  hint="Your institution's case reference, e.g. ICC-01/04-01/06, KSC-BC-2020-06"
                />
                <FormField label="Title" name="title" required maxLength={500} />
                <FormField label="Description" name="description" multiline maxLength={10000} />
                <FormField label="Jurisdiction" name="jurisdiction" maxLength={200} />
              </div>
            </div>

            {/* ── Case Members ── */}
            <div style={{ borderTop: '1px solid var(--border-subtle)', paddingTop: 'var(--space-lg)' }}>
              <div className="flex items-center justify-between" style={{ marginBottom: 'var(--space-sm)' }}>
                <div>
                  <h2
                    style={{
                      fontSize: 'var(--text-base)',
                      fontWeight: 600,
                      color: 'var(--text-primary)',
                      marginBottom: '0.125rem',
                    }}
                  >
                    Case members
                  </h2>
                  <p style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)' }}>
                    Assign organization members to this case with specific roles.
                  </p>
                </div>
              </div>

              {/* Add member picker */}
              <div
                className="flex items-end"
                style={{
                  gap: 'var(--space-sm)',
                  marginBottom: 'var(--space-md)',
                }}
              >
                <div className="flex-1">
                  <label className="field-label">Member</label>
                  {membersLoading ? (
                    <div className="input-field flex items-center" style={{ color: 'var(--text-tertiary)', fontSize: 'var(--text-sm)' }}>
                      Loading members...
                    </div>
                  ) : availableMembers.length === 0 ? (
                    <div className="input-field flex items-center" style={{ color: 'var(--text-tertiary)', fontSize: 'var(--text-sm)' }}>
                      {orgMembers.length === 0 ? 'No organization members found' : 'All members added'}
                    </div>
                  ) : (
                    <select
                      value={pickUserId}
                      onChange={(e) => setPickUserId(e.target.value)}
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
                  )}
                </div>
                <div style={{ width: '180px' }}>
                  <label className="field-label">Role</label>
                  <select
                    value={pickRoleDefId}
                    onChange={(e) => setPickRoleDefId(e.target.value)}
                    className="input-field"
                  >
                    {roleDefs.map((rd) => (
                      <option key={rd.id} value={rd.id}>{rd.name}</option>
                    ))}
                  </select>
                </div>
                <button
                  type="button"
                  onClick={handleAddMember}
                  disabled={!pickUserId || !pickRoleDefId}
                  className="btn-secondary"
                  style={{ fontSize: 'var(--text-sm)' }}
                >
                  Add
                </button>
              </div>

              {/* Pending members list */}
              {pendingMembers.length > 0 && (
                <div
                  style={{
                    borderRadius: 'var(--radius-md)',
                    border: '1px solid var(--border-subtle)',
                    overflow: 'hidden',
                  }}
                >
                  {pendingMembers.map((m, i) => (
                    <div
                      key={m.userId}
                      className="flex items-center"
                      style={{
                        padding: 'var(--space-sm) var(--space-md)',
                        borderBottom: i < pendingMembers.length - 1 ? '1px solid var(--border-subtle)' : undefined,
                        gap: 'var(--space-md)',
                      }}
                    >
                      <div
                        className="flex items-center justify-center shrink-0 font-[family-name:var(--font-heading)]"
                        style={{
                          width: '28px',
                          height: '28px',
                          borderRadius: 'var(--radius-full)',
                          backgroundColor: 'var(--amber-subtle)',
                          color: 'var(--amber-accent)',
                          fontSize: '11px',
                          fontWeight: 700,
                        }}
                      >
                        {m.displayName.charAt(0).toUpperCase()}
                      </div>
                      <div className="flex-1 min-w-0">
                        <p className="truncate" style={{ fontSize: 'var(--text-sm)', fontWeight: 500, color: 'var(--text-primary)' }}>
                          {m.displayName}
                        </p>
                        {m.email && (
                          <p className="truncate" style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)' }}>
                            {m.email}
                          </p>
                        )}
                      </div>
                      <span
                        className="badge shrink-0"
                        style={{
                          backgroundColor: 'var(--amber-subtle)',
                          color: 'var(--amber-accent)',
                          textTransform: 'capitalize',
                        }}
                      >
                        {m.roleName}
                      </span>
                      <button
                        type="button"
                        onClick={() => handleRemovePending(m.userId)}
                        style={{
                          fontSize: 'var(--text-xs)',
                          fontWeight: 500,
                          color: 'var(--status-hold)',
                          background: 'none',
                          border: 'none',
                          cursor: 'pointer',
                        }}
                      >
                        Remove
                      </button>
                    </div>
                  ))}
                </div>
              )}

              {pendingMembers.length === 0 && (
                <p style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)', textAlign: 'center', padding: 'var(--space-sm) 0' }}>
                  You can also add members after creating the case.
                </p>
              )}
            </div>

            {/* ── Submit ── */}
            <div
              className="flex gap-[var(--space-md)] pt-[var(--space-md)]"
              style={{ borderTop: '1px solid var(--border-subtle)' }}
            >
              <button type="submit" disabled={loading} className="btn-primary">
                {loading
                  ? pendingMembers.length > 0
                    ? 'Creating & assigning...'
                    : 'Creating...'
                  : 'Create case'}
              </button>
              <button type="button" onClick={() => router.back()} className="btn-ghost">
                Cancel
              </button>
            </div>
          </form>
        </div>
      </div>
    </Shell>
  );
}

function FormField({
  label,
  name,
  required,
  placeholder,
  hint,
  multiline,
  maxLength,
}: {
  label: string;
  name: string;
  required?: boolean;
  placeholder?: string;
  hint?: string;
  multiline?: boolean;
  maxLength?: number;
}) {
  return (
    <div>
      <label htmlFor={name} className="field-label">
        {label}
      </label>
      {multiline ? (
        <textarea
          id={name}
          name={name}
          rows={4}
          maxLength={maxLength}
          className="input-field resize-y"
        />
      ) : (
        <input
          id={name}
          name={name}
          type="text"
          required={required}
          placeholder={placeholder}
          maxLength={maxLength}
          className="input-field"
        />
      )}
      {hint && (
        <p className="mt-[var(--space-xs)] text-xs" style={{ color: 'var(--text-tertiary)' }}>
          {hint}
        </p>
      )}
    </div>
  );
}
