'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import type { OrgMembership, OrgRole } from '@/types';
import { useOrg } from '@/hooks/use-org';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';
const ROLE_LABELS: Record<OrgRole, string> = { owner: 'Owner', admin: 'Admin', member: 'Member' };

interface Props {
  orgId: string;
  members: OrgMembership[];
}

export function MemberManagement({ orgId, members }: Props) {
  const { isOrgAdmin } = useOrg();
  const [showInvite, setShowInvite] = useState(false);

  return (
    <div>
      {isOrgAdmin && (
        <div className="flex justify-end" style={{ marginBottom: 'var(--space-sm)' }}>
          <button onClick={() => setShowInvite(!showInvite)} className="btn-primary" type="button">
            Invite Member
          </button>
        </div>
      )}

      {showInvite && <InviteForm orgId={orgId} onClose={() => setShowInvite(false)} />}

      {members.length === 0 ? (
        <div className="card-inset" style={{ padding: 'var(--space-xl)', textAlign: 'center' }}>
          <p style={{ fontSize: 'var(--text-sm)', color: 'var(--text-tertiary)' }}>
            No members yet. Invite team members to collaborate.
          </p>
        </div>
      ) : (
        <div className="card-inset" style={{ overflow: 'hidden', padding: 0 }}>
          <table className="w-full" style={{ fontSize: 'var(--text-sm)', borderCollapse: 'collapse' }}>
            <thead>
              <tr style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                <th className="text-left field-label" style={{ padding: 'var(--space-sm) var(--space-md)', marginBottom: 0 }}>Member</th>
                <th className="text-left field-label" style={{ padding: 'var(--space-sm) var(--space-md)', marginBottom: 0 }}>Role</th>
                <th className="text-left field-label" style={{ padding: 'var(--space-sm) var(--space-md)', marginBottom: 0 }}>Joined</th>
                {isOrgAdmin && (
                  <th className="text-right field-label" style={{ padding: 'var(--space-sm) var(--space-md)', marginBottom: 0 }}>Actions</th>
                )}
              </tr>
            </thead>
            <tbody>
              {members.map((m) => (
                <MemberRow key={m.id} member={m} orgId={orgId} canManage={isOrgAdmin} />
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function MemberRow({ member, orgId, canManage }: { member: OrgMembership; orgId: string; canManage: boolean }) {
  const router = useRouter();
  const [removing, setRemoving] = useState(false);

  async function handleRemove() {
    if (!confirm('Remove this member from the organization?')) return;
    setRemoving(true);
    try {
      await fetch(`${API_BASE}/api/organizations/${orgId}/members/${member.user_id}`, { method: 'DELETE', credentials: 'include' });
      router.refresh();
    } finally {
      setRemoving(false);
    }
  }

  return (
    <tr style={{ borderBottom: '1px solid var(--border-subtle)' }}>
      <td style={{ padding: 'var(--space-sm) var(--space-md)' }}>
        <p style={{ fontWeight: 500, color: 'var(--text-primary)' }}>
          {member.display_name || member.user_id.slice(0, 8)}
        </p>
        {member.email && (
          <p style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)' }}>{member.email}</p>
        )}
      </td>
      <td style={{ padding: 'var(--space-sm) var(--space-md)' }}>
        <span className="badge" style={{ backgroundColor: 'var(--bg-inset)', color: 'var(--text-secondary)' }}>
          {ROLE_LABELS[member.role]}
        </span>
      </td>
      <td style={{ padding: 'var(--space-sm) var(--space-md)', color: 'var(--text-tertiary)', fontSize: 'var(--text-xs)' }}>
        {member.joined_at ? new Date(member.joined_at).toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' }) : '\u2014'}
      </td>
      {canManage && (
        <td className="text-right" style={{ padding: 'var(--space-sm) var(--space-md)' }}>
          {member.role !== 'owner' && (
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
        </td>
      )}
    </tr>
  );
}

function InviteForm({ orgId, onClose }: { orgId: string; onClose: () => void }) {
  const router = useRouter();
  const [email, setEmail] = useState('');
  const [role, setRole] = useState<OrgRole>('member');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!email.trim()) return;
    setSubmitting(true);
    setError('');

    try {
      const res = await fetch(`${API_BASE}/api/organizations/${orgId}/invitations`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ email: email.trim(), role }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => null);
        setError(body?.error ?? 'Failed to send invitation');
        return;
      }
      onClose();
      router.refresh();
    } catch {
      setError('Network error');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form
      onSubmit={handleSubmit}
      className="card-inset"
      style={{ padding: 'var(--space-md)', marginBottom: 'var(--space-md)' }}
    >
      <div className="flex items-end" style={{ gap: 'var(--space-sm)' }}>
        <div className="flex-1">
          <label className="field-label">Email</label>
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="colleague@example.com" className="input-field" required />
        </div>
        <div>
          <label className="field-label">Role</label>
          <select value={role} onChange={(e) => setRole(e.target.value as OrgRole)} className="input-field">
            <option value="member">Member</option>
            <option value="admin">Admin</option>
          </select>
        </div>
        <button type="submit" disabled={submitting} className="btn-primary">
          {submitting ? 'Sending\u2026' : 'Send'}
        </button>
        <button type="button" onClick={onClose} className="btn-ghost">Cancel</button>
      </div>
      {error && <div className="banner-error" style={{ marginTop: 'var(--space-sm)' }}>{error}</div>}
    </form>
  );
}
