'use client';

import { useState, useEffect, useCallback } from 'react';
import { useSession } from 'next-auth/react';
import { useOrg } from '@/hooks/use-org';
import type { OrgMembership, OrgInvitation, OrgRole, CaseAssignment, RoleDefinition } from '@/types';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

const ROLE_COLORS: Record<string, { bg: string; fg: string }> = {
  owner: { bg: 'var(--amber-subtle)', fg: 'var(--amber-accent)' },
  admin: { bg: 'var(--status-active-bg)', fg: 'var(--status-active)' },
  member: { bg: 'var(--bg-inset)', fg: 'var(--text-tertiary)' },
  lead_investigator: { bg: 'var(--amber-subtle)', fg: 'var(--amber-accent)' },
  investigator: { bg: 'var(--status-active-bg)', fg: 'var(--status-active)' },
  analyst: { bg: 'var(--bg-inset)', fg: 'var(--text-secondary)' },
  reviewer: { bg: 'var(--status-closed-bg)', fg: 'var(--status-closed)' },
  viewer: { bg: 'var(--bg-inset)', fg: 'var(--text-tertiary)' },
};

const STATUS_COLORS: Record<string, { bg: string; fg: string }> = {
  active: { bg: 'var(--status-active-bg)', fg: 'var(--status-active)' },
  closed: { bg: 'var(--status-closed-bg)', fg: 'var(--status-closed)' },
  archived: { bg: 'var(--status-archived-bg)', fg: 'var(--status-archived)' },
};

function fmtDate(s: string) {
  return new Date(s).toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' });
}

function roleBadge(role: string) {
  const c = ROLE_COLORS[role] ?? ROLE_COLORS.member;
  return { backgroundColor: c.bg, color: c.fg };
}

function statusBadge(status: string) {
  const c = STATUS_COLORS[status] ?? { bg: 'var(--bg-inset)', fg: 'var(--text-tertiary)' };
  return { backgroundColor: c.bg, color: c.fg };
}

function SectionHeader({ title, description }: { title: string; description: string }) {
  return (
    <div style={{ marginBottom: 'var(--space-md)' }}>
      <h3
        style={{
          fontSize: 'var(--text-base)',
          fontWeight: 600,
          color: 'var(--text-primary)',
          marginBottom: '0.25rem',
        }}
      >
        {title}
      </h3>
      <p style={{ fontSize: 'var(--text-sm)', color: 'var(--text-tertiary)', lineHeight: 1.5 }}>
        {description}
      </p>
    </div>
  );
}

function SectionDivider() {
  return <hr style={{ border: 'none', borderTop: '1px solid var(--border-subtle)', margin: 'var(--space-xl) 0' }} />;
}

function EmptyState({ message }: { message: string }) {
  return (
    <div
      style={{
        padding: 'var(--space-lg) var(--space-md)',
        textAlign: 'center',
        color: 'var(--text-tertiary)',
        fontSize: 'var(--text-sm)',
      }}
    >
      {message}
    </div>
  );
}

function Avatar({ name, size = 32 }: { name: string; size?: number }) {
  const initial = name.charAt(0).toUpperCase();
  return (
    <div
      className="flex items-center justify-center shrink-0 font-[family-name:var(--font-heading)]"
      style={{
        width: `${size}px`,
        height: `${size}px`,
        borderRadius: 'var(--radius-full)',
        backgroundColor: 'var(--amber-subtle)',
        color: 'var(--amber-accent)',
        fontSize: `${Math.round(size * 0.4)}px`,
        fontWeight: 700,
      }}
    >
      {initial}
    </div>
  );
}

export function OrganizationSection() {
  const { activeOrg, isOrgAdmin, isOrgOwner } = useOrg();
  const { data: session } = useSession();

  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [saving, setSaving] = useState(false);
  const [saveMsg, setSaveMsg] = useState<{ type: 'error' | 'success'; text: string } | null>(null);

  const [members, setMembers] = useState<OrgMembership[]>([]);
  const [membersLoading, setMembersLoading] = useState(true);

  const [invitations, setInvitations] = useState<OrgInvitation[]>([]);
  const [invitationsLoading, setInvitationsLoading] = useState(true);
  const [revokingId, setRevokingId] = useState<string | null>(null);

  const [assignments, setAssignments] = useState<CaseAssignment[]>([]);
  const [assignmentsLoading, setAssignmentsLoading] = useState(true);

  const [orgCases, setOrgCases] = useState<{ id: string; title: string; reference_code: string; status: string }[]>([]);
  const [roleDefs, setRoleDefs] = useState<RoleDefinition[]>([]);
  const [expandedMemberId, setExpandedMemberId] = useState<string | null>(null);
  const [assigningCase, setAssigningCase] = useState('');
  const [assigningRole, setAssigningRole] = useState('');
  const [assigningSubmit, setAssigningSubmit] = useState(false);

  const [showInvite, setShowInvite] = useState(false);
  const [invEmail, setInvEmail] = useState('');
  const [invRole, setInvRole] = useState<OrgRole>('member');
  const [inviting, setInviting] = useState(false);
  const [invError, setInvError] = useState('');

  const [removingId, setRemovingId] = useState<string | null>(null);

  const authHeaders = session?.accessToken
    ? { Authorization: `Bearer ${session.accessToken}`, 'Content-Type': 'application/json' }
    : undefined;

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
    } catch { /* empty */ } finally { setMembersLoading(false); }
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
    } catch { /* empty */ } finally { setInvitationsLoading(false); }
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
    } catch { /* empty */ } finally { setAssignmentsLoading(false); }
  }, [activeOrg, session?.accessToken]);

  const fetchOrgCasesAndRoles = useCallback(async () => {
    if (!activeOrg || !session?.accessToken) return;
    const headers = { Authorization: `Bearer ${session.accessToken}` };
    const [casesRes, roleDefsRes] = await Promise.all([
      fetch(`${API_BASE}/api/cases?organization_id=${activeOrg.id}`, { headers }).catch(() => null),
      fetch(`${API_BASE}/api/organizations/${activeOrg.id}/role-definitions`, { headers }).catch(() => null),
    ]);
    if (casesRes?.ok) {
      const body = await casesRes.json();
      const cases = Array.isArray(body) ? body : (body.data ?? []);
      setOrgCases(cases);
    }
    if (roleDefsRes?.ok) {
      const body = await roleDefsRes.json();
      const defs = Array.isArray(body) ? body : (body.data ?? []);
      setRoleDefs(defs);
      if (defs.length > 0 && !assigningRole) setAssigningRole(defs[0].slug);
    }
  }, [activeOrg, session?.accessToken, assigningRole]);

  useEffect(() => {
    fetchMembers();
    fetchInvitations();
    fetchAssignments();
    fetchOrgCasesAndRoles();
  }, [fetchMembers, fetchInvitations, fetchAssignments, fetchOrgCasesAndRoles]);

  if (!activeOrg) {
    return <EmptyState message="No organization selected." />;
  }

  if (!isOrgAdmin) {
    return <EmptyState message="Only organization admins can manage settings." />;
  }

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setSaveMsg(null);
    try {
      const res = await fetch(`${API_BASE}/api/organizations/${activeOrg.id}`, {
        method: 'PATCH',
        headers: authHeaders,
        body: JSON.stringify({ name: name.trim(), description: description.trim() }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => null);
        setSaveMsg({ type: 'error', text: body?.error ?? 'Failed to update' });
      } else {
        setSaveMsg({ type: 'success', text: 'Organization updated.' });
        setTimeout(() => setSaveMsg(null), 3000);
      }
    } catch {
      setSaveMsg({ type: 'error', text: 'Network error' });
    } finally {
      setSaving(false);
    }
  };

  const handleInvite = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!invEmail.trim()) return;
    setInviting(true);
    setInvError('');
    try {
      const res = await fetch(`${API_BASE}/api/organizations/${activeOrg.id}/invitations`, {
        method: 'POST',
        headers: authHeaders,
        body: JSON.stringify({ email: invEmail.trim(), role: invRole }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => null);
        setInvError(body?.error ?? 'Failed to send invitation');
      } else {
        setInvEmail('');
        setInvRole('member');
        setShowInvite(false);
        fetchInvitations();
      }
    } catch {
      setInvError('Network error');
    } finally {
      setInviting(false);
    }
  };

  const handleRevoke = async (inviteId: string) => {
    setRevokingId(inviteId);
    try {
      const res = await fetch(
        `${API_BASE}/api/organizations/${activeOrg.id}/invitations/${inviteId}`,
        { method: 'DELETE', headers: authHeaders },
      );
      if (res.ok) {
        setInvitations((prev) => prev.filter((inv) => inv.id !== inviteId));
      }
    } catch { /* empty */ } finally { setRevokingId(null); }
  };

  const handleRemoveMember = async (userId: string) => {
    if (!confirm('Remove this member from the organization?')) return;
    setRemovingId(userId);
    try {
      await fetch(`${API_BASE}/api/organizations/${activeOrg.id}/members/${userId}`, {
        method: 'DELETE',
        headers: authHeaders,
      });
      fetchMembers();
    } catch { /* empty */ } finally { setRemovingId(null); }
  };

  const handleChangeRole = async (userId: string, newRole: OrgRole) => {
    try {
      await fetch(`${API_BASE}/api/organizations/${activeOrg.id}/members/${userId}`, {
        method: 'PATCH',
        headers: authHeaders,
        body: JSON.stringify({ role: newRole }),
      });
      fetchMembers();
    } catch { /* empty */ }
  };

  const pendingInvitations = invitations.filter((inv) => inv.status === 'pending');

  return (
    <div className="stagger-in">

      {/* ── Organization Details ── */}
      <SectionHeader
        title="General"
        description="Your organization's name and description visible to all members."
      />
      <form onSubmit={handleSave} style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-md)' }}>
        <div className="flex items-start" style={{ gap: 'var(--space-md)' }}>
          <Avatar name={activeOrg.name} size={48} />
          <div className="flex-1" style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-md)' }}>
            <div>
              <label className="field-label">Organization name</label>
              <input
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="input-field"
                style={{ maxWidth: '400px' }}
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
                placeholder="What does this organization do?"
              />
            </div>
          </div>
        </div>
        {saveMsg && (
          <div className={saveMsg.type === 'error' ? 'banner-error' : 'banner-success'}>
            {saveMsg.text}
          </div>
        )}
        <div>
          <button
            type="submit"
            disabled={saving || !name.trim()}
            className="btn-primary"
            style={{ fontSize: 'var(--text-sm)' }}
          >
            {saving ? 'Saving\u2026' : 'Save changes'}
          </button>
        </div>
      </form>

      <SectionDivider />

      {/* ── Members ── */}
      <div className="flex items-center justify-between" style={{ marginBottom: 'var(--space-md)' }}>
        <SectionHeader
          title="Members"
          description="People who have access to this organization and its cases."
        />
        <button
          type="button"
          onClick={() => setShowInvite(!showInvite)}
          className="btn-primary shrink-0"
          style={{ fontSize: 'var(--text-sm)', alignSelf: 'flex-start' }}
        >
          Invite member
        </button>
      </div>

      {showInvite && (
        <form
          onSubmit={handleInvite}
          style={{
            padding: 'var(--space-md)',
            marginBottom: 'var(--space-md)',
            borderRadius: 'var(--radius-md)',
            border: '1px solid var(--border-default)',
            backgroundColor: 'var(--bg-elevated)',
          }}
        >
          <div className="flex items-end" style={{ gap: 'var(--space-sm)' }}>
            <div className="flex-1">
              <label className="field-label">Email address</label>
              <input
                type="email"
                value={invEmail}
                onChange={(e) => setInvEmail(e.target.value)}
                placeholder="colleague@example.com"
                className="input-field"
                required
              />
            </div>
            <div style={{ width: '120px' }}>
              <label className="field-label">Role</label>
              <select
                value={invRole}
                onChange={(e) => setInvRole(e.target.value as OrgRole)}
                className="input-field"
              >
                <option value="member">Member</option>
                <option value="admin">Admin</option>
              </select>
            </div>
            <button type="submit" disabled={inviting} className="btn-primary" style={{ fontSize: 'var(--text-sm)' }}>
              {inviting ? 'Sending\u2026' : 'Send invite'}
            </button>
            <button type="button" onClick={() => setShowInvite(false)} className="btn-ghost" style={{ fontSize: 'var(--text-sm)' }}>
              Cancel
            </button>
          </div>
          {invError && <div className="banner-error" style={{ marginTop: 'var(--space-sm)' }}>{invError}</div>}
        </form>
      )}

      {membersLoading ? (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-xs)' }}>
          {[1, 2, 3].map((i) => (
            <div key={i} className="skeleton" style={{ height: '3.5rem', borderRadius: 'var(--radius-md)' }} />
          ))}
        </div>
      ) : members.length === 0 ? (
        <EmptyState message="No members yet. Invite your team to get started." />
      ) : (
        <div
          style={{
            borderRadius: 'var(--radius-md)',
            border: '1px solid var(--border-subtle)',
            overflow: 'hidden',
          }}
        >
          {members.map((m, i) => {
            const displayName = m.display_name || m.email || m.user_id.slice(0, 8);
            const rc = roleBadge(m.role);
            return (
              <div
                key={m.id}
                className="flex items-center"
                style={{
                  padding: 'var(--space-sm) var(--space-md)',
                  borderBottom: i < members.length - 1 ? '1px solid var(--border-subtle)' : undefined,
                  gap: 'var(--space-md)',
                }}
              >
                <Avatar name={displayName} size={28} />
                <div className="flex-1 min-w-0">
                  <p
                    className="truncate"
                    style={{ fontSize: 'var(--text-sm)', fontWeight: 500, color: 'var(--text-primary)' }}
                  >
                    {displayName}
                  </p>
                  {m.email && (
                    <p
                      className="truncate"
                      style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)' }}
                    >
                      {m.email}
                    </p>
                  )}
                </div>
                {m.role === 'owner' ? (
                  <span className="badge shrink-0" style={{ ...rc, textTransform: 'capitalize' }}>
                    {m.role}
                  </span>
                ) : (
                  <select
                    value={m.role}
                    onChange={(e) => handleChangeRole(m.user_id, e.target.value as OrgRole)}
                    className="shrink-0"
                    style={{
                      fontSize: 'var(--text-xs)',
                      fontWeight: 600,
                      padding: '0.125rem var(--space-sm)',
                      borderRadius: 'var(--radius-full)',
                      border: '1px solid var(--border-subtle)',
                      backgroundColor: rc.backgroundColor,
                      color: rc.color,
                      cursor: 'pointer',
                      textTransform: 'capitalize',
                      appearance: 'auto',
                    }}
                  >
                    <option value="admin">Admin</option>
                    <option value="member">Member</option>
                  </select>
                )}
                <span
                  className="shrink-0"
                  style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)', minWidth: '5rem' }}
                >
                  {m.joined_at ? fmtDate(m.joined_at) : '\u2014'}
                </span>
                {m.role !== 'owner' && (
                  <button
                    type="button"
                    onClick={() => handleRemoveMember(m.user_id)}
                    disabled={removingId === m.user_id}
                    style={{
                      fontSize: 'var(--text-xs)',
                      fontWeight: 500,
                      color: 'var(--status-hold)',
                      background: 'none',
                      border: 'none',
                      cursor: 'pointer',
                      opacity: removingId === m.user_id ? 0.4 : 1,
                      flexShrink: 0,
                    }}
                  >
                    Remove
                  </button>
                )}
              </div>
            );
          })}
        </div>
      )}

      {/* Pending invitations inline */}
      {!invitationsLoading && pendingInvitations.length > 0 && (
        <div style={{ marginTop: 'var(--space-md)' }}>
          <p
            style={{
              fontSize: 'var(--text-xs)',
              fontWeight: 600,
              textTransform: 'uppercase',
              letterSpacing: '0.06em',
              color: 'var(--text-tertiary)',
              marginBottom: 'var(--space-xs)',
            }}
          >
            Pending invitations
          </p>
          <div
            style={{
              borderRadius: 'var(--radius-md)',
              border: '1px solid var(--border-subtle)',
              overflow: 'hidden',
            }}
          >
            {pendingInvitations.map((inv, i) => {
              const rc = roleBadge(inv.role);
              return (
                <div
                  key={inv.id}
                  className="flex items-center"
                  style={{
                    padding: 'var(--space-sm) var(--space-md)',
                    borderBottom: i < pendingInvitations.length - 1 ? '1px solid var(--border-subtle)' : undefined,
                    gap: 'var(--space-md)',
                  }}
                >
                  <div
                    className="flex items-center justify-center shrink-0"
                    style={{
                      width: '28px',
                      height: '28px',
                      borderRadius: 'var(--radius-full)',
                      border: '1.5px dashed var(--border-strong)',
                      color: 'var(--text-tertiary)',
                    }}
                  >
                    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                      <path strokeLinecap="round" strokeLinejoin="round" d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
                    </svg>
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="truncate" style={{ fontSize: 'var(--text-sm)', fontWeight: 500, color: 'var(--text-primary)' }}>
                      {inv.email}
                    </p>
                    <p style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)' }}>
                      Expires {fmtDate(inv.expires_at)}
                    </p>
                  </div>
                  <span className="badge shrink-0" style={{ ...rc, textTransform: 'capitalize' }}>
                    {inv.role}
                  </span>
                  <button
                    type="button"
                    onClick={() => handleRevoke(inv.id)}
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
                </div>
              );
            })}
          </div>
        </div>
      )}

      <SectionDivider />

      {/* ── Case Assignments — member-centric ── */}
      <SectionHeader
        title="Case assignments"
        description="Click a member to view and manage their case role assignments."
      />
      {membersLoading || assignmentsLoading ? (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-xs)' }}>
          {[1, 2, 3].map((i) => (
            <div key={i} className="skeleton" style={{ height: '3rem', borderRadius: 'var(--radius-md)' }} />
          ))}
        </div>
      ) : members.length === 0 ? (
        <EmptyState message="No members yet." />
      ) : (
        <div
          style={{
            borderRadius: 'var(--radius-md)',
            border: '1px solid var(--border-subtle)',
            overflow: 'hidden',
          }}
        >
          {members.map((m, i) => {
            const displayName = m.display_name || m.email || m.user_id.slice(0, 8);
            const memberAssignments = assignments.filter((a) => a.user_id === m.user_id);
            const isExpanded = expandedMemberId === m.user_id;
            return (
              <div key={m.id}>
                <button
                  type="button"
                  onClick={() => setExpandedMemberId(isExpanded ? null : m.user_id)}
                  className="w-full text-left flex items-center"
                  style={{
                    padding: 'var(--space-sm) var(--space-md)',
                    borderBottom: (isExpanded || i < members.length - 1) ? '1px solid var(--border-subtle)' : undefined,
                    gap: 'var(--space-md)',
                    background: isExpanded ? 'var(--bg-inset)' : 'transparent',
                    border: 'none',
                    cursor: 'pointer',
                    transition: 'background var(--duration-fast) ease',
                  }}
                >
                  <Avatar name={displayName} size={24} />
                  <span className="flex-1 truncate" style={{ fontSize: 'var(--text-sm)', fontWeight: 500, color: 'var(--text-primary)' }}>
                    {displayName}
                  </span>
                  <span style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)', flexShrink: 0 }}>
                    {memberAssignments.length} case{memberAssignments.length !== 1 ? 's' : ''}
                  </span>
                  <svg
                    width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
                    style={{
                      color: 'var(--text-tertiary)',
                      transition: 'transform var(--duration-fast) ease',
                      transform: isExpanded ? 'rotate(180deg)' : 'rotate(0)',
                      flexShrink: 0,
                    }}
                  >
                    <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
                  </svg>
                </button>

                {isExpanded && (
                  <div style={{ padding: 'var(--space-sm) var(--space-md)', borderBottom: i < members.length - 1 ? '1px solid var(--border-subtle)' : undefined, backgroundColor: 'var(--bg-primary)' }}>
                    {memberAssignments.length > 0 ? (
                      <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-xs)', marginBottom: 'var(--space-sm)' }}>
                        {memberAssignments.map((a) => (
                          <div
                            key={a.id}
                            className="flex items-center"
                            style={{ gap: 'var(--space-sm)', padding: 'var(--space-xs) 0' }}
                          >
                            <span style={{ fontSize: 'var(--text-sm)', color: 'var(--text-primary)', fontWeight: 500, flex: 1 }}>
                              {a.case_title}
                            </span>
                            <span
                              className="font-[family-name:var(--font-mono)]"
                              style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)' }}
                            >
                              {a.reference_code}
                            </span>
                            <select
                              value={a.role}
                              onChange={async (e) => {
                                const newRole = e.target.value;
                                try {
                                  // Remove old role, assign new one
                                  await fetch(`${API_BASE}/api/cases/${a.case_id}/roles/${m.user_id}`, {
                                    method: 'DELETE',
                                    headers: authHeaders,
                                  });
                                  await fetch(`${API_BASE}/api/cases/${a.case_id}/roles`, {
                                    method: 'POST',
                                    headers: authHeaders,
                                    body: JSON.stringify({ user_id: m.user_id, role: newRole }),
                                  });
                                  fetchAssignments();
                                } catch { /* empty */ }
                              }}
                              style={{
                                fontSize: 'var(--text-xs)',
                                fontWeight: 600,
                                padding: '0.125rem var(--space-sm)',
                                borderRadius: 'var(--radius-full)',
                                border: '1px solid var(--border-subtle)',
                                ...roleBadge(a.role),
                                cursor: 'pointer',
                                textTransform: 'capitalize',
                              }}
                            >
                              {roleDefs.length > 0
                                ? roleDefs.map((rd) => (
                                    <option key={rd.slug} value={rd.slug}>{rd.name}</option>
                                  ))
                                : ['investigator', 'prosecutor', 'defence', 'judge', 'observer', 'victim_representative'].map((r) => (
                                    <option key={r} value={r}>{r.replace(/_/g, ' ')}</option>
                                  ))
                              }
                            </select>
                            <span className="badge" style={{ ...statusBadge(a.case_status), textTransform: 'capitalize' }}>
                              {a.case_status}
                            </span>
                            <button
                              type="button"
                              onClick={async () => {
                                if (!confirm(`Remove ${displayName} from "${a.case_title}"?`)) return;
                                try {
                                  await fetch(`${API_BASE}/api/cases/${a.case_id}/roles/${m.user_id}`, {
                                    method: 'DELETE',
                                    headers: authHeaders,
                                  });
                                  fetchAssignments();
                                } catch { /* empty */ }
                              }}
                              style={{
                                fontSize: 'var(--text-xs)',
                                fontWeight: 500,
                                color: 'var(--status-hold)',
                                background: 'none',
                                border: 'none',
                                cursor: 'pointer',
                                flexShrink: 0,
                              }}
                            >
                              Remove
                            </button>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <p style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)', marginBottom: 'var(--space-sm)' }}>
                        Not assigned to any cases yet.
                      </p>
                    )}

                    {/* Assign to case inline form */}
                    {orgCases.length > 0 && (
                      <div className="flex items-end" style={{ gap: 'var(--space-xs)' }}>
                        <div className="flex-1">
                          <label style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)', fontWeight: 600 }}>Case</label>
                          <select
                            value={assigningCase}
                            onChange={(e) => setAssigningCase(e.target.value)}
                            className="input-field"
                            style={{ fontSize: 'var(--text-xs)' }}
                          >
                            <option value="">Select a case...</option>
                            {orgCases
                              .filter((c) => !memberAssignments.some((a) => a.case_id === c.id))
                              .map((c) => (
                                <option key={c.id} value={c.id}>
                                  {c.title} ({c.reference_code})
                                </option>
                              ))}
                          </select>
                        </div>
                        <div style={{ width: '140px' }}>
                          <label style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)', fontWeight: 600 }}>Role</label>
                          <select
                            value={assigningRole}
                            onChange={(e) => setAssigningRole(e.target.value)}
                            className="input-field"
                            style={{ fontSize: 'var(--text-xs)' }}
                          >
                            {roleDefs.length > 0
                              ? roleDefs.map((rd) => (
                                  <option key={rd.id} value={rd.slug}>{rd.name}</option>
                                ))
                              : ['investigator', 'prosecutor', 'defence', 'judge', 'observer'].map((r) => (
                                  <option key={r} value={r}>{r.charAt(0).toUpperCase() + r.slice(1)}</option>
                                ))
                            }
                          </select>
                        </div>
                        <button
                          type="button"
                          disabled={!assigningCase || assigningSubmit}
                          className="btn-primary"
                          style={{ fontSize: 'var(--text-xs)' }}
                          onClick={async () => {
                            if (!assigningCase || !activeOrg) return;
                            setAssigningSubmit(true);
                            try {
                              await fetch(`${API_BASE}/api/cases/${assigningCase}/roles`, {
                                method: 'POST',
                                headers: authHeaders,
                                body: JSON.stringify({ user_id: m.user_id, role: assigningRole }),
                              });
                              setAssigningCase('');
                              fetchAssignments();
                            } catch { /* empty */ } finally { setAssigningSubmit(false); }
                          }}
                        >
                          {assigningSubmit ? 'Adding...' : 'Assign'}
                        </button>
                      </div>
                    )}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}

      {/* ── Danger Zone ── */}
      {isOrgOwner && (
        <>
          <SectionDivider />
          <SectionHeader
            title="Danger zone"
            description="Irreversible actions. Deleting an organization removes all associated data permanently."
          />
          <div
            style={{
              padding: 'var(--space-md)',
              borderRadius: 'var(--radius-md)',
              border: '1px solid var(--status-hold-bg)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              gap: 'var(--space-md)',
            }}
          >
            <div>
              <p style={{ fontSize: 'var(--text-sm)', fontWeight: 500, color: 'var(--text-primary)' }}>
                Delete this organization
              </p>
              <p style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)' }}>
                All cases must be archived or transferred first.
              </p>
            </div>
            <button type="button" className="btn-danger" style={{ fontSize: 'var(--text-sm)', flexShrink: 0 }} disabled>
              Delete organization
            </button>
          </div>
        </>
      )}
    </div>
  );
}
