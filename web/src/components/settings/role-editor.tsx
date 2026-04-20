'use client';

import { useState, useEffect, useCallback } from 'react';
import { useSession } from 'next-auth/react';
import { useOrg } from '@/hooks/use-org';
import type { RoleDefinition, CasePermission } from '@/types';
import { PERMISSION_GROUPS } from '@/types';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

export function RoleEditor() {
  const { activeOrg } = useOrg();
  const { data: session } = useSession();
  const [roles, setRoles] = useState<RoleDefinition[]>([]);
  const [loading, setLoading] = useState(true);
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  const authHeaders = useCallback((): Record<string, string> => {
    if (!session?.accessToken) return { 'Content-Type': 'application/json' };
    return { Authorization: `Bearer ${session.accessToken}`, 'Content-Type': 'application/json' };
  }, [session?.accessToken]);

  const fetchRoles = useCallback(async () => {
    if (!activeOrg?.id || !session?.accessToken) return;
    setLoading(true);
    try {
      const res = await fetch(`${API_BASE}/api/organizations/${activeOrg.id}/role-definitions`, {
        headers: authHeaders(),
      });
      if (res.ok) {
        const body = await res.json();
        setRoles(Array.isArray(body) ? body : (body.data ?? []));
      }
    } catch { /* empty */ } finally { setLoading(false); }
  }, [activeOrg?.id, session?.accessToken, authHeaders]);

  useEffect(() => { fetchRoles(); }, [fetchRoles]);

  const handleSavePerms = async (roleId: string, permissions: Record<CasePermission, boolean>) => {
    if (!activeOrg) return;
    setError('');
    try {
      const res = await fetch(`${API_BASE}/api/organizations/${activeOrg.id}/role-definitions/${roleId}`, {
        method: 'PATCH',
        headers: authHeaders(),
        body: JSON.stringify({ permissions }),
      });
      if (res.ok) {
        const body = await res.json();
        const updated = body.data ?? body;
        setRoles((prev) => prev.map((r) => (r.id === roleId ? updated : r)));
        setSuccess('Permissions saved.');
        setTimeout(() => setSuccess(''), 2000);
      } else {
        const body = await res.json().catch(() => null);
        setError(body?.error ?? 'Failed to save');
      }
    } catch { setError('Network error'); }
  };

  const handleReset = async (roleId: string) => {
    if (!activeOrg) return;
    setError('');
    try {
      const res = await fetch(`${API_BASE}/api/organizations/${activeOrg.id}/role-definitions/${roleId}/reset`, {
        method: 'POST',
        headers: authHeaders(),
      });
      if (res.ok) {
        const body = await res.json();
        const updated = body.data ?? body;
        setRoles((prev) => prev.map((r) => (r.id === roleId ? updated : r)));
        setSuccess('Reset to default.');
        setTimeout(() => setSuccess(''), 2000);
      } else {
        const body = await res.json().catch(() => null);
        setError(body?.error ?? 'Failed to reset');
      }
    } catch { setError('Network error'); }
  };

  const handleDelete = async (roleId: string) => {
    if (!activeOrg || !confirm('Delete this custom role?')) return;
    setError('');
    try {
      const res = await fetch(`${API_BASE}/api/organizations/${activeOrg.id}/role-definitions/${roleId}`, {
        method: 'DELETE',
        headers: authHeaders(),
      });
      if (res.ok) {
        setRoles((prev) => prev.filter((r) => r.id !== roleId));
        setExpandedId(null);
        setSuccess('Role deleted.');
        setTimeout(() => setSuccess(''), 2000);
      } else {
        const body = await res.json().catch(() => null);
        setError(body?.error ?? 'Failed to delete');
      }
    } catch { setError('Network error'); }
  };

  const handleCreate = async (name: string, description: string, permissions: Record<CasePermission, boolean>) => {
    if (!activeOrg) return;
    setError('');
    try {
      const res = await fetch(`${API_BASE}/api/organizations/${activeOrg.id}/role-definitions`, {
        method: 'POST',
        headers: authHeaders(),
        body: JSON.stringify({ name, description, permissions }),
      });
      if (res.ok) {
        const body = await res.json();
        const created = body.data ?? body;
        setRoles((prev) => [...prev, created]);
        setShowCreate(false);
        setExpandedId(created.id);
        setSuccess('Role created.');
        setTimeout(() => setSuccess(''), 2000);
      } else {
        const body = await res.json().catch(() => null);
        setError(body?.error ?? 'Failed to create');
      }
    } catch { setError('Network error'); }
  };

  if (loading) {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-xs)' }}>
        {[1, 2, 3].map((i) => (
          <div key={i} className="skeleton" style={{ height: '3rem', borderRadius: 'var(--radius-md)' }} />
        ))}
      </div>
    );
  }

  const enabledCount = (perms: Record<string, boolean>) =>
    Object.values(perms).filter(Boolean).length;

  const totalPerms = PERMISSION_GROUPS.reduce((acc, g) => acc + g.permissions.length, 0);

  return (
    <div>
      {error && <div className="banner-error" style={{ marginBottom: 'var(--space-sm)' }}>{error}</div>}
      {success && <div className="banner-success" style={{ marginBottom: 'var(--space-sm)' }}>{success}</div>}

      <div
        style={{
          borderRadius: 'var(--radius-md)',
          border: '1px solid var(--border-subtle)',
          overflow: 'hidden',
        }}
      >
        {roles.map((role, i) => {
          const isExpanded = expandedId === role.id;
          const count = enabledCount(role.permissions);
          return (
            <div key={role.id}>
              {/* Row */}
              <button
                type="button"
                onClick={() => setExpandedId(isExpanded ? null : role.id)}
                className="w-full text-left flex items-center"
                style={{
                  padding: 'var(--space-sm) var(--space-md)',
                  borderBottom: (isExpanded || i < roles.length - 1) ? '1px solid var(--border-subtle)' : undefined,
                  gap: 'var(--space-md)',
                  background: isExpanded ? 'var(--bg-inset)' : 'transparent',
                  border: 'none',
                  cursor: 'pointer',
                  transition: 'background var(--duration-fast) ease',
                }}
              >
                <div className="flex-1 min-w-0">
                  <span style={{ fontSize: 'var(--text-sm)', fontWeight: 500, color: 'var(--text-primary)' }}>
                    {role.name}
                  </span>
                  {role.is_system && (
                    <span
                      className="badge"
                      style={{
                        marginLeft: 'var(--space-xs)',
                        backgroundColor: 'var(--bg-inset)',
                        color: 'var(--text-tertiary)',
                        fontSize: '0.625rem',
                      }}
                    >
                      SYSTEM
                    </span>
                  )}
                </div>
                <span style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)', flexShrink: 0 }}>
                  {count}/{totalPerms} permissions
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

              {/* Expanded panel */}
              {isExpanded && (
                <RolePanel
                  role={role}
                  onSave={(perms) => handleSavePerms(role.id, perms)}
                  onReset={() => handleReset(role.id)}
                  onDelete={() => handleDelete(role.id)}
                />
              )}
            </div>
          );
        })}
      </div>

      {/* Create custom role */}
      <div style={{ marginTop: 'var(--space-md)' }}>
        {showCreate ? (
          <CreateRoleForm
            onSave={handleCreate}
            onCancel={() => setShowCreate(false)}
          />
        ) : (
          <button
            type="button"
            onClick={() => setShowCreate(true)}
            className="btn-secondary"
            style={{ fontSize: 'var(--text-sm)' }}
          >
            + Create custom role
          </button>
        )}
      </div>
    </div>
  );
}

function RolePanel({
  role,
  onSave,
  onReset,
  onDelete,
}: {
  role: RoleDefinition;
  onSave: (perms: Record<CasePermission, boolean>) => void;
  onReset: () => void;
  onDelete: () => void;
}) {
  const [perms, setPerms] = useState<Record<CasePermission, boolean>>({ ...role.permissions });
  const [dirty, setDirty] = useState(false);

  const toggle = (p: CasePermission) => {
    setPerms((prev) => {
      const next = { ...prev, [p]: !prev[p] };
      setDirty(true);
      return next;
    });
  };

  return (
    <div style={{ padding: 'var(--space-md)', borderBottom: '1px solid var(--border-subtle)' }}>
      {role.description && (
        <p style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)', marginBottom: 'var(--space-md)' }}>
          {role.description}
        </p>
      )}

      {/* Permission grid */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))', gap: 'var(--space-md)' }}>
        {PERMISSION_GROUPS.map((group) => (
          <div key={group.label}>
            <p style={{
              fontSize: 'var(--text-xs)',
              fontWeight: 600,
              textTransform: 'uppercase',
              letterSpacing: '0.06em',
              color: 'var(--text-tertiary)',
              marginBottom: 'var(--space-xs)',
            }}>
              {group.label}
            </p>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
              {group.permissions.map((perm) => (
                <label
                  key={perm.value}
                  className="flex items-center"
                  style={{ gap: 'var(--space-xs)', cursor: 'pointer', fontSize: 'var(--text-sm)' }}
                >
                  <input
                    type="checkbox"
                    checked={perms[perm.value] ?? false}
                    onChange={() => toggle(perm.value)}
                    style={{
                      accentColor: 'var(--amber-accent)',
                      width: '14px',
                      height: '14px',
                    }}
                  />
                  <span style={{ color: perms[perm.value] ? 'var(--text-primary)' : 'var(--text-tertiary)' }}>
                    {perm.label}
                  </span>
                </label>
              ))}
            </div>
          </div>
        ))}
      </div>

      {/* Actions */}
      <div className="flex items-center" style={{ gap: 'var(--space-sm)', marginTop: 'var(--space-md)' }}>
        <button
          type="button"
          onClick={() => { onSave(perms); setDirty(false); }}
          disabled={!dirty}
          className="btn-primary"
          style={{ fontSize: 'var(--text-sm)' }}
        >
          Save permissions
        </button>
        {role.is_default && (
          <button
            type="button"
            onClick={onReset}
            className="btn-ghost"
            style={{ fontSize: 'var(--text-sm)' }}
          >
            Reset to default
          </button>
        )}
        {!role.is_system && (
          <button
            type="button"
            onClick={onDelete}
            style={{
              fontSize: 'var(--text-xs)',
              fontWeight: 500,
              color: 'var(--status-hold)',
              background: 'none',
              border: 'none',
              cursor: 'pointer',
              marginLeft: 'auto',
            }}
          >
            Delete role
          </button>
        )}
      </div>
    </div>
  );
}

function CreateRoleForm({
  onSave,
  onCancel,
}: {
  onSave: (name: string, description: string, perms: Record<CasePermission, boolean>) => void;
  onCancel: () => void;
}) {
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [perms, setPerms] = useState<Record<CasePermission, boolean>>(() => {
    const initial: Record<string, boolean> = {};
    for (const group of PERMISSION_GROUPS) {
      for (const p of group.permissions) {
        initial[p.value] = false;
      }
    }
    return initial as Record<CasePermission, boolean>;
  });

  const toggle = (p: CasePermission) => {
    setPerms((prev) => ({ ...prev, [p]: !prev[p] }));
  };

  return (
    <div
      style={{
        padding: 'var(--space-md)',
        borderRadius: 'var(--radius-md)',
        border: '1px solid var(--border-default)',
        backgroundColor: 'var(--bg-elevated)',
      }}
    >
      <h4 style={{ fontSize: 'var(--text-sm)', fontWeight: 600, color: 'var(--text-primary)', marginBottom: 'var(--space-md)' }}>
        Create custom role
      </h4>
      <div style={{ display: 'flex', gap: 'var(--space-sm)', marginBottom: 'var(--space-md)' }}>
        <div className="flex-1">
          <label className="field-label">Name</label>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="e.g. Senior Analyst"
            className="input-field"
            maxLength={50}
          />
        </div>
        <div className="flex-1">
          <label className="field-label">Description</label>
          <input
            type="text"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Brief role description"
            className="input-field"
            maxLength={200}
          />
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))', gap: 'var(--space-md)', marginBottom: 'var(--space-md)' }}>
        {PERMISSION_GROUPS.map((group) => (
          <div key={group.label}>
            <p style={{
              fontSize: 'var(--text-xs)',
              fontWeight: 600,
              textTransform: 'uppercase',
              letterSpacing: '0.06em',
              color: 'var(--text-tertiary)',
              marginBottom: 'var(--space-xs)',
            }}>
              {group.label}
            </p>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
              {group.permissions.map((perm) => (
                <label
                  key={perm.value}
                  className="flex items-center"
                  style={{ gap: 'var(--space-xs)', cursor: 'pointer', fontSize: 'var(--text-sm)' }}
                >
                  <input
                    type="checkbox"
                    checked={perms[perm.value] ?? false}
                    onChange={() => toggle(perm.value)}
                    style={{ accentColor: 'var(--amber-accent)', width: '14px', height: '14px' }}
                  />
                  <span style={{ color: perms[perm.value] ? 'var(--text-primary)' : 'var(--text-tertiary)' }}>
                    {perm.label}
                  </span>
                </label>
              ))}
            </div>
          </div>
        ))}
      </div>

      <div className="flex items-center" style={{ gap: 'var(--space-sm)' }}>
        <button
          type="button"
          onClick={() => onSave(name.trim(), description.trim(), perms)}
          disabled={!name.trim()}
          className="btn-primary"
          style={{ fontSize: 'var(--text-sm)' }}
        >
          Create role
        </button>
        <button type="button" onClick={onCancel} className="btn-ghost" style={{ fontSize: 'var(--text-sm)' }}>
          Cancel
        </button>
      </div>
    </div>
  );
}
