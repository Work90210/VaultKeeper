'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useSession } from 'next-auth/react';
import type { Organization, OrgMembership, Case } from '@/types';
import { MemberManagement } from './member-management';
import { useOrg } from '@/hooks/use-org';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

interface Props {
  org: Organization;
  members: OrgMembership[];
  cases: Case[];
}

export function OrgDashboard({ org, members, cases }: Props) {
  const { isOrgAdmin, isOrgOwner } = useOrg();
  const { data: session } = useSession();
  const [name, setName] = useState(org.name);
  const [description, setDescription] = useState(org.description);
  const [saving, setSaving] = useState(false);
  const [saveSuccess, setSaveSuccess] = useState(false);
  const [saveError, setSaveError] = useState('');

  const initial = org.name.charAt(0).toUpperCase();
  const activeCases = cases.filter((c) => c.status === 'active').length;

  const handleSave = async () => {
    setSaving(true);
    setSaveError('');
    setSaveSuccess(false);
    try {
      const res = await fetch(`${API_BASE}/api/organizations/${org.id}`, {
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
        setSaveSuccess(true);
        setTimeout(() => setSaveSuccess(false), 3000);
      }
    } catch {
      setSaveError('Network error');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div style={{ padding: 'var(--space-lg)', maxWidth: '56rem' }}>
      <div className="stagger-in" style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-xl)' }}>

        {/* ── Name & Logo ── */}
        <section>
          <h2 className="field-label" style={{ marginBottom: 'var(--space-sm)' }}>Name & Logo</h2>
          <div className="card" style={{ padding: 'var(--space-lg)' }}>
            <div className="flex items-start" style={{ gap: 'var(--space-lg)' }}>
              {/* Avatar */}
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

              {/* Fields */}
              <div className="flex-1" style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-md)' }}>
                {isOrgAdmin ? (
                  <>
                    <div>
                      <label className="field-label">Organization Name</label>
                      <input
                        type="text"
                        value={name}
                        onChange={(e) => setName(e.target.value)}
                        className="input-field"
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
                    {saveSuccess && <div className="banner-success">Organization updated.</div>}
                    <div>
                      <button
                        type="button"
                        onClick={handleSave}
                        disabled={saving || (!name.trim())}
                        className="btn-primary"
                        style={{ fontSize: 'var(--text-sm)' }}
                      >
                        {saving ? 'Saving\u2026' : 'Save'}
                      </button>
                    </div>
                  </>
                ) : (
                  <>
                    <div>
                      <p style={{ fontSize: 'var(--text-lg)', fontWeight: 600, color: 'var(--text-primary)' }}>
                        {org.name}
                      </p>
                      {org.description && (
                        <p style={{ fontSize: 'var(--text-sm)', color: 'var(--text-secondary)', marginTop: 'var(--space-xs)' }}>
                          {org.description}
                        </p>
                      )}
                    </div>
                  </>
                )}
              </div>
            </div>

            {/* Slug + stats inline */}
            <div
              className="grid grid-cols-4"
              style={{
                marginTop: 'var(--space-lg)',
                padding: 'var(--space-sm) var(--space-md)',
                borderRadius: 'var(--radius-md)',
                backgroundColor: 'var(--bg-inset)',
              }}
            >
              <div>
                <p className="font-[family-name:var(--font-mono)]" style={{ fontSize: 'var(--text-sm)', color: 'var(--text-primary)' }}>
                  {org.slug || '\u2014'}
                </p>
                <p style={{ fontSize: '0.6875rem', color: 'var(--text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.05em' }}>Slug</p>
              </div>
              <div>
                <p className="font-[family-name:var(--font-mono)] tabular-nums" style={{ fontSize: 'var(--text-sm)', fontWeight: 600, color: 'var(--text-primary)' }}>
                  {members.length}
                </p>
                <p style={{ fontSize: '0.6875rem', color: 'var(--text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.05em' }}>Members</p>
              </div>
              <div>
                <p className="font-[family-name:var(--font-mono)] tabular-nums" style={{ fontSize: 'var(--text-sm)', fontWeight: 600, color: 'var(--text-primary)' }}>
                  {cases.length}
                </p>
                <p style={{ fontSize: '0.6875rem', color: 'var(--text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.05em' }}>Cases</p>
              </div>
              <div>
                <p className="font-[family-name:var(--font-mono)] tabular-nums" style={{ fontSize: 'var(--text-sm)', fontWeight: 600, color: 'var(--status-active)' }}>
                  {activeCases}
                </p>
                <p style={{ fontSize: '0.6875rem', color: 'var(--text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.05em' }}>Active</p>
              </div>
            </div>
          </div>
        </section>

        {/* ── Members ── */}
        <section>
          <h2 className="field-label" style={{ marginBottom: 'var(--space-sm)' }}>Members</h2>
          <MemberManagement orgId={org.id} members={members} />
        </section>

        {/* ── Cases ── */}
        <section>
          <div className="flex items-center justify-between" style={{ marginBottom: 'var(--space-sm)' }}>
            <h2 className="field-label" style={{ marginBottom: 0 }}>Cases</h2>
            <span style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)' }}>
              {cases.length} total
            </span>
          </div>
          {cases.length === 0 ? (
            <div className="card-inset" style={{ padding: 'var(--space-lg)', textAlign: 'center' }}>
              <p style={{ fontSize: 'var(--text-sm)', color: 'var(--text-tertiary)' }}>
                No cases yet.
              </p>
            </div>
          ) : (
            <div className="card-inset" style={{ overflow: 'hidden', padding: 0 }}>
              <table className="w-full" style={{ fontSize: 'var(--text-sm)', borderCollapse: 'collapse' }}>
                <thead>
                  <tr style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                    <th className="text-left field-label" style={{ padding: 'var(--space-sm) var(--space-md)', marginBottom: 0 }}>Reference</th>
                    <th className="text-left field-label" style={{ padding: 'var(--space-sm) var(--space-md)', marginBottom: 0 }}>Title</th>
                    <th className="text-left field-label" style={{ padding: 'var(--space-sm) var(--space-md)', marginBottom: 0 }}>Status</th>
                    <th className="text-right field-label" style={{ padding: 'var(--space-sm) var(--space-md)', marginBottom: 0 }}>Created</th>
                  </tr>
                </thead>
                <tbody>
                  {cases.map((c) => (
                    <tr
                      key={c.id}
                      className="table-row"
                      onClick={() => window.location.href = `/en/cases/${c.id}`}
                      style={{ borderBottom: '1px solid var(--border-subtle)' }}
                    >
                      <td
                        className="font-[family-name:var(--font-mono)]"
                        style={{ padding: 'var(--space-sm) var(--space-md)', color: 'var(--text-tertiary)', fontSize: 'var(--text-xs)' }}
                      >
                        {c.reference_code}
                      </td>
                      <td style={{ padding: 'var(--space-sm) var(--space-md)', color: 'var(--text-primary)', fontWeight: 500 }}>
                        {c.title}
                      </td>
                      <td style={{ padding: 'var(--space-sm) var(--space-md)' }}>
                        <span
                          className="badge"
                          style={{
                            backgroundColor: c.status === 'active' ? 'var(--status-active-bg)' : c.status === 'closed' ? 'var(--status-closed-bg)' : 'var(--status-archived-bg)',
                            color: c.status === 'active' ? 'var(--status-active)' : c.status === 'closed' ? 'var(--status-closed)' : 'var(--status-archived)',
                          }}
                        >
                          {c.status}
                        </span>
                      </td>
                      <td className="text-right" style={{ padding: 'var(--space-sm) var(--space-md)', color: 'var(--text-tertiary)', fontSize: 'var(--text-xs)' }}>
                        {new Date(c.created_at).toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' })}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>

        {/* ── Danger Zone ── */}
        {isOrgOwner && (
          <section>
            <h2 className="field-label" style={{ color: 'var(--status-hold)', marginBottom: 'var(--space-sm)' }}>Danger Zone</h2>
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
    </div>
  );
}
