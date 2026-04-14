'use client';

import { useState, useEffect, useCallback } from 'react';
import { useSession } from 'next-auth/react';
import { useOrg } from '@/hooks/use-org';
import { MemberManagement } from '@/components/organizations/member-management';
import type { OrgMembership, OrgInvitation, CaseAssignment } from '@/types';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

const ROLE_STYLES: Record<string, { bg: string; color: string }> = {
  owner: { bg: 'var(--amber-subtle)', color: 'var(--amber-accent)' },
  admin: { bg: 'var(--status-active-bg)', color: 'var(--status-active)' },
  member: { bg: 'var(--bg-inset)', color: 'var(--text-tertiary)' },
};

const CASE_STATUS_STYLES: Record<string, { bg: string; color: string }> = {
  active: { bg: 'var(--status-active-bg)', color: 'var(--status-active)' },
  closed: { bg: 'var(--status-closed-bg)', color: 'var(--status-closed)' },
  archived: { bg: 'var(--status-archived-bg)', color: 'var(--status-archived)' },
};

function fmtDate(s: string) {
  return new Date(s).toLocaleDateString('en-GB', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
  });
}

function getRoleStyle(role: string) {
  return ROLE_STYLES[role] ?? ROLE_STYLES.member;
}

function getCaseStatusStyle(status: string) {
  return CASE_STATUS_STYLES[status] ?? { bg: 'var(--bg-inset)', color: 'var(--text-tertiary)' };
}

export function OrganizationSection() {
  const { activeOrg, isOrgAdmin, isOrgOwner } = useOrg();
  const { data: session } = useSession();

  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState('');
  const [saveSuccess, setSaveSuccess] = useState('');

  const [members, setMembers] = useState<OrgMembership[]>([]);
  const [membersLoading, setMembersLoading] = useState(true);

  const [invitations, setInvitations] = useState<OrgInvitation[]>([]);
  const [invitationsLoading, setInvitationsLoading] = useState(true);
  const [revokingId, setRevokingId] = useState<string | null>(null);

  const [assignments, setAssignments] = useState<CaseAssignment[]>([]);
  const [assignmentsLoading, setAssignmentsLoading] = useState(true);

  useEffect(() => {
    if (activeOrg) {
      setName(activeOrg.name);
      setDescription(activeOrg.description);
    }
  }, [activeOrg]);

  const fetchMembers = useCallback(async () => {
    if (!activeOrg || !session?.accessToken) return;
    setMembersLoading(true);
    try {
      const res = await fetch(`${API_BASE}/api/organizations/${activeOrg.id}/members`, {
        headers: { Authorization: `Bearer ${session.accessToken}` },
      });
      if (res.ok) {
        const body = await res.json();
        setMembers(Array.isArray(body) ? body : (body.data ?? []));
      }
    } catch {
      // silently fail — members will show empty
    } finally {
      setMembersLoading(false);
    }
  }, [activeOrg, session?.accessToken]);

  const fetchInvitations = useCallback(async () => {
    if (!activeOrg || !session?.accessToken) return;
    setInvitationsLoading(true);
    try {
      const res = await fetch(`${API_BASE}/api/organizations/${activeOrg.id}/invitations`, {
        headers: { Authorization: `Bearer ${session.accessToken}` },
      });
      if (res.ok) {
        const body = await res.json();
        setInvitations(Array.isArray(body) ? body : (body.data ?? []));
      }
    } catch {
      // silently fail
    } finally {
      setInvitationsLoading(false);
    }
  }, [activeOrg, session?.accessToken]);

  const fetchAssignments = useCallback(async () => {
    if (!activeOrg || !session?.accessToken) return;
    setAssignmentsLoading(true);
    try {
      const res = await fetch(`${API_BASE}/api/organizations/${activeOrg.id}/case-assignments`, {
        headers: { Authorization: `Bearer ${session.accessToken}` },
      });
      if (res.ok) {
        const body = await res.json();
        setAssignments(Array.isArray(body) ? body : (body.data ?? []));
      }
    } catch {
      // silently fail
    } finally {
      setAssignmentsLoading(false);
    }
  }, [activeOrg, session?.accessToken]);

  useEffect(() => {
    fetchMembers();
    fetchInvitations();
    fetchAssignments();
  }, [fetchMembers, fetchInvitations, fetchAssignments]);

  if (!activeOrg) {
    return (
      <p style={{ fontSize: 'var(--text-sm)', color: 'var(--text-tertiary)' }}>
        No organization selected.
      </p>
    );
  }

  if (!isOrgAdmin) {
    return (
      <p style={{ fontSize: 'var(--text-sm)', color: 'var(--text-tertiary)' }}>
        Only organization admins can manage settings.
      </p>
    );
  }

  const initial = activeOrg.name.charAt(0).toUpperCase();

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setSaveError('');
    setSaveSuccess('');
    try {
      const res = await fetch(`${API_BASE}/api/organizations/${activeOrg.id}`, {
        method: 'PATCH',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${session?.accessToken}`,
        },
        body: JSON.stringify({ name: name.trim(), description: description.trim() }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => null);
        setSaveError(body?.error ?? 'Failed to update');
      } else {
        setSaveSuccess('Organization updated.');
        setTimeout(() => setSaveSuccess(''), 3000);
      }
    } catch {
      setSaveError('Network error');
    } finally {
      setSaving(false);
    }
  };

  const handleRevokeInvitation = async (inviteId: string) => {
    if (!session?.accessToken) return;
    setRevokingId(inviteId);
    try {
      const res = await fetch(
        `${API_BASE}/api/organizations/${activeOrg.id}/invitations/${inviteId}`,
        {
          method: 'DELETE',
          headers: { Authorization: `Bearer ${session.accessToken}` },
        }
      );
      if (res.ok) {
        setInvitations((prev) => prev.filter((inv) => inv.id !== inviteId));
      }
    } catch {
      // silently fail
    } finally {
      setRevokingId(null);
    }
  };

  const pendingInvitations = invitations.filter((inv) => inv.status === 'pending');

  return (
    <div className="stagger-in" style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-xl)' }}>

      {/* ── A. Organization Details ── */}
      <section>
        <h3 className="field-label" style={{ marginBottom: 'var(--space-sm)' }}>
          Organization Details
        </h3>
        <div className="card" style={{ padding: 'var(--space-lg)' }}>
          <div className="flex items-start" style={{ gap: 'var(--space-lg)' }}>
            <div
              className="flex items-center justify-center shrink-0 font-[family-name:var(--font-heading)]"
              style={{
                width: '4rem',
                height: '4rem',
                borderRadius: 'var(--radius-lg)',
                backgroundColor: 'var(--amber-subtle)',
                color: 'var(--amber-accent)',
                fontSize: 'var(--text-2xl)',
                fontWeight: 700,
              }}
            >
              {initial}
            </div>

            <form
              onSubmit={handleSave}
              className="flex-1"
              style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-md)' }}
            >
              <div>
                <label className="field-label">Organization Name</label>
                <input
                  type="text"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  className="input-field"
                  required
                />
              </div>
              <div>
                <label className="field-label">Description</label>
                <textarea
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  rows={2}
                  className="input-field resize-y"
                />
              </div>
              {saveError && <div className="banner-error">{saveError}</div>}
              {saveSuccess && <div className="banner-success">{saveSuccess}</div>}
              <div>
                <button
                  type="submit"
                  disabled={saving || !name.trim()}
                  className="btn-primary"
                  style={{ fontSize: 'var(--text-sm)' }}
                >
                  {saving ? 'Saving\u2026' : 'Save Changes'}
                </button>
              </div>
            </form>
          </div>
        </div>
      </section>

      {/* ── B. Members ── */}
      <section>
        <h3 className="field-label" style={{ marginBottom: 'var(--space-sm)' }}>
          Members
        </h3>
        {membersLoading ? (
          <div className="card-inset" style={{ padding: 'var(--space-lg)' }}>
            <div className="skeleton" style={{ height: '4rem', borderRadius: 'var(--radius-md)' }} />
          </div>
        ) : (
          <MemberManagement orgId={activeOrg.id} members={members} />
        )}
      </section>

      {/* ── C. Pending Invitations ── */}
      <section>
        <h3 className="field-label" style={{ marginBottom: 'var(--space-sm)' }}>
          Pending Invitations
        </h3>
        {invitationsLoading ? (
          <div className="card-inset" style={{ padding: 'var(--space-lg)' }}>
            <div className="skeleton" style={{ height: '3rem', borderRadius: 'var(--radius-md)' }} />
          </div>
        ) : pendingInvitations.length === 0 ? (
          <div className="card-inset" style={{ padding: 'var(--space-lg)', textAlign: 'center' }}>
            <p style={{ fontSize: 'var(--text-sm)', color: 'var(--text-tertiary)' }}>
              No pending invitations.
            </p>
          </div>
        ) : (
          <div className="card-inset" style={{ overflow: 'hidden', padding: 0 }}>
            <table className="w-full" style={{ fontSize: 'var(--text-sm)', borderCollapse: 'collapse' }}>
              <thead>
                <tr style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                  <th className="text-left field-label" style={{ padding: 'var(--space-sm) var(--space-md)', marginBottom: 0 }}>Email</th>
                  <th className="text-left field-label" style={{ padding: 'var(--space-sm) var(--space-md)', marginBottom: 0 }}>Role</th>
                  <th className="text-left field-label" style={{ padding: 'var(--space-sm) var(--space-md)', marginBottom: 0 }}>Expires</th>
                  <th className="text-right field-label" style={{ padding: 'var(--space-sm) var(--space-md)', marginBottom: 0 }}>Actions</th>
                </tr>
              </thead>
              <tbody>
                {pendingInvitations.map((inv) => {
                  const roleStyle = getRoleStyle(inv.role);
                  return (
                    <tr key={inv.id} style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                      <td style={{ padding: 'var(--space-sm) var(--space-md)', color: 'var(--text-primary)', fontWeight: 500 }}>
                        {inv.email}
                      </td>
                      <td style={{ padding: 'var(--space-sm) var(--space-md)' }}>
                        <span
                          className="badge"
                          style={{ backgroundColor: roleStyle.bg, color: roleStyle.color, textTransform: 'capitalize' }}
                        >
                          {inv.role}
                        </span>
                      </td>
                      <td style={{ padding: 'var(--space-sm) var(--space-md)', color: 'var(--text-tertiary)', fontSize: 'var(--text-xs)' }}>
                        {fmtDate(inv.expires_at)}
                      </td>
                      <td className="text-right" style={{ padding: 'var(--space-sm) var(--space-md)' }}>
                        <button
                          type="button"
                          onClick={() => handleRevokeInvitation(inv.id)}
                          disabled={revokingId === inv.id}
                          style={{
                            fontSize: 'var(--text-xs)',
                            fontWeight: 500,
                            color: 'var(--status-hold)',
                            background: 'none',
                            border: 'none',
                            cursor: 'pointer',
                            opacity: revokingId === inv.id ? 0.4 : 1,
                          }}
                        >
                          Revoke
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </section>

      {/* ── D. Case Role Overview ── */}
      <section>
        <h3 className="field-label" style={{ marginBottom: 'var(--space-sm)' }}>
          Case Role Overview
        </h3>
        <p style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)', marginBottom: 'var(--space-sm)' }}>
          All case role assignments across your organization.
        </p>
        {assignmentsLoading ? (
          <div className="card-inset" style={{ padding: 'var(--space-lg)' }}>
            <div className="skeleton" style={{ height: '4rem', borderRadius: 'var(--radius-md)' }} />
          </div>
        ) : assignments.length === 0 ? (
          <div className="card-inset" style={{ padding: 'var(--space-lg)', textAlign: 'center' }}>
            <p style={{ fontSize: 'var(--text-sm)', color: 'var(--text-tertiary)' }}>
              No case role assignments found.
            </p>
          </div>
        ) : (
          <div className="card-inset" style={{ overflow: 'hidden', padding: 0 }}>
            <table className="w-full" style={{ fontSize: 'var(--text-sm)', borderCollapse: 'collapse' }}>
              <thead>
                <tr style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                  <th className="text-left field-label" style={{ padding: 'var(--space-sm) var(--space-md)', marginBottom: 0 }}>User</th>
                  <th className="text-left field-label" style={{ padding: 'var(--space-sm) var(--space-md)', marginBottom: 0 }}>Reference</th>
                  <th className="text-left field-label" style={{ padding: 'var(--space-sm) var(--space-md)', marginBottom: 0 }}>Case</th>
                  <th className="text-left field-label" style={{ padding: 'var(--space-sm) var(--space-md)', marginBottom: 0 }}>Role</th>
                  <th className="text-left field-label" style={{ padding: 'var(--space-sm) var(--space-md)', marginBottom: 0 }}>Status</th>
                </tr>
              </thead>
              <tbody>
                {assignments.map((a) => {
                  const roleStyle = getRoleStyle(a.role);
                  const statusStyle = getCaseStatusStyle(a.case_status);
                  return (
                    <tr key={a.id} style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                      <td style={{ padding: 'var(--space-sm) var(--space-md)', color: 'var(--text-primary)', fontWeight: 500 }}>
                        {a.user_id.slice(0, 8)}
                      </td>
                      <td
                        className="font-[family-name:var(--font-mono)]"
                        style={{ padding: 'var(--space-sm) var(--space-md)', color: 'var(--text-tertiary)', fontSize: 'var(--text-xs)' }}
                      >
                        {a.reference_code}
                      </td>
                      <td style={{ padding: 'var(--space-sm) var(--space-md)', color: 'var(--text-primary)' }}>
                        {a.case_title}
                      </td>
                      <td style={{ padding: 'var(--space-sm) var(--space-md)' }}>
                        <span
                          className="badge"
                          style={{ backgroundColor: roleStyle.bg, color: roleStyle.color, textTransform: 'capitalize' }}
                        >
                          {a.role.replace('_', ' ')}
                        </span>
                      </td>
                      <td style={{ padding: 'var(--space-sm) var(--space-md)' }}>
                        <span
                          className="badge"
                          style={{ backgroundColor: statusStyle.bg, color: statusStyle.color, textTransform: 'capitalize' }}
                        >
                          {a.case_status}
                        </span>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </section>

      {/* ── E. Danger Zone ── */}
      {isOrgOwner && (
        <section>
          <h3 className="field-label" style={{ color: 'var(--status-hold)', marginBottom: 'var(--space-sm)' }}>
            Danger Zone
          </h3>
          <div
            className="card"
            style={{ padding: 'var(--space-md)', borderColor: 'var(--status-hold-bg)' }}
          >
            <p style={{ fontSize: 'var(--text-sm)', color: 'var(--text-secondary)', marginBottom: 'var(--space-sm)' }}>
              Deleting an organization is permanent. All cases must be archived first.
            </p>
            <button type="button" className="btn-danger" style={{ fontSize: 'var(--text-sm)' }} disabled>
              Delete Organization
            </button>
          </div>
        </section>
      )}
    </div>
  );
}
