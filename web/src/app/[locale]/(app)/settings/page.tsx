'use client';

import { useState, useEffect, useCallback } from 'react';
import { useSession } from 'next-auth/react';
import { useLocale } from 'next-intl';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { Shell } from '@/components/layout/shell';
import { useOrg } from '@/hooks/use-org';
import type { ApiKey } from '@/types';
import { listApiKeys, createApiKey, revokeApiKey } from '@/lib/apikeys-api';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

type Theme = 'system' | 'light' | 'dark';
type SettingsTab = 'general' | 'notifications' | 'api-keys' | 'security' | 'organization';

// ── Sidebar Navigation ──────────────────────────────────────────────────────

const SETTINGS_TABS: { key: SettingsTab; label: string; adminOnly?: boolean }[] = [
  { key: 'general', label: 'General' },
  { key: 'notifications', label: 'Notifications' },
  { key: 'security', label: 'Security' },
  { key: 'api-keys', label: 'API Keys' },
  { key: 'organization', label: 'Organization', adminOnly: true },
];

// ── General Settings ────────────────────────────────────────────────────────

function GeneralSection() {
  const locale = useLocale();
  const router = useRouter();
  const [theme, setTheme] = useState<Theme>('system');

  useEffect(() => {
    const stored = localStorage.getItem('vk-theme') as Theme | null;
    if (stored && ['system', 'light', 'dark'].includes(stored)) setTheme(stored);
  }, []);

  const handleThemeChange = (next: Theme) => {
    setTheme(next);
    localStorage.setItem('vk-theme', next);
    const root = document.documentElement;
    if (next === 'system') root.removeAttribute('data-theme');
    else root.setAttribute('data-theme', next);
  };

  const handleLocaleChange = (next: string) => {
    const path = window.location.pathname.replace(/^\/(en|fr)/, `/${next}`);
    router.push(path);
  };

  const themeOptions: { value: Theme; label: string; icon: string }[] = [
    { value: 'system', label: 'System', icon: 'M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z' },
    { value: 'light', label: 'Light', icon: 'M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z' },
    { value: 'dark', label: 'Dark', icon: 'M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z' },
  ];

  return (
    <div className="stagger-in space-y-[var(--space-lg)]">
      {/* Appearance */}
      <div>
        <h3 className="field-label">Appearance</h3>
        <p className="text-sm mb-[var(--space-sm)]" style={{ color: 'var(--text-secondary)' }}>
          Choose how VaultKeeper looks on your device.
        </p>
        <div className="flex gap-[var(--space-sm)]">
          {themeOptions.map((opt) => (
            <button
              key={opt.value}
              type="button"
              onClick={() => handleThemeChange(opt.value)}
              className="flex items-center gap-[var(--space-xs)]"
              style={{
                padding: 'var(--space-xs) var(--space-md)',
                borderRadius: 'var(--radius-md)',
                border: `1.5px solid ${theme === opt.value ? 'var(--amber-accent)' : 'var(--border-default)'}`,
                backgroundColor: theme === opt.value ? 'var(--amber-subtle)' : 'transparent',
                color: theme === opt.value ? 'var(--text-primary)' : 'var(--text-secondary)',
                fontSize: 'var(--text-sm)',
                fontWeight: 500,
                cursor: 'pointer',
              }}
            >
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                <path d={opt.icon} />
              </svg>
              {opt.label}
            </button>
          ))}
        </div>
      </div>

      {/* Language */}
      <div>
        <h3 className="field-label">Language</h3>
        <p className="text-sm mb-[var(--space-sm)]" style={{ color: 'var(--text-secondary)' }}>
          Select your preferred language.
        </p>
        <div className="flex gap-[var(--space-sm)]">
          {[{ value: 'en', label: 'English' }, { value: 'fr', label: 'Fran\u00e7ais' }].map((lang) => (
            <button
              key={lang.value}
              type="button"
              onClick={() => handleLocaleChange(lang.value)}
              style={{
                padding: 'var(--space-xs) var(--space-md)',
                borderRadius: 'var(--radius-md)',
                border: `1.5px solid ${locale === lang.value ? 'var(--amber-accent)' : 'var(--border-default)'}`,
                backgroundColor: locale === lang.value ? 'var(--amber-subtle)' : 'transparent',
                color: locale === lang.value ? 'var(--text-primary)' : 'var(--text-secondary)',
                fontSize: 'var(--text-sm)',
                fontWeight: 500,
                cursor: 'pointer',
              }}
            >
              {lang.label}
            </button>
          ))}
        </div>
      </div>

      {/* Profile link */}
      <div
        className="flex items-center justify-between"
        style={{
          padding: 'var(--space-md)',
          borderRadius: 'var(--radius-md)',
          border: '1px solid var(--border-subtle)',
          backgroundColor: 'var(--bg-elevated)',
        }}
      >
        <div>
          <p style={{ fontSize: 'var(--text-sm)', fontWeight: 500, color: 'var(--text-primary)' }}>Profile</p>
          <p style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)' }}>
            Manage your display name, bio, and timezone.
          </p>
        </div>
        <Link href={`/${locale}/profile`} className="btn-secondary" style={{ fontSize: 'var(--text-sm)' }}>
          Go to Profile
        </Link>
      </div>
    </div>
  );
}

// ── Notification Preferences ────────────────────────────────────────────────

function NotificationsSection() {
  const { data: session } = useSession();
  const [prefs, setPrefs] = useState({
    email_enabled: false,
    evidence_uploaded: true,
    evidence_destroyed: true,
    case_status_changed: true,
    legal_hold_changed: true,
    member_joined: true,
    member_removed: false,
    custody_chain_event: true,
    backup_failed: true,
    retention_expiring: true,
  });
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!session?.accessToken) return;
    setIsLoading(true);
    fetch(`${API_BASE}/api/settings/notifications`, {
      headers: { Authorization: `Bearer ${session.accessToken}` },
    })
      .then((r) => {
        if (!r.ok) throw new Error('Failed to load preferences');
        return r.json();
      })
      .then((envelope) => {
        if (envelope.data) setPrefs((prev) => ({ ...prev, ...envelope.data }));
      })
      .catch(() => setError('Failed to load notification preferences.'))
      .finally(() => setIsLoading(false));
  }, [session?.accessToken]);

  const toggle = (key: keyof typeof prefs) => {
    setPrefs((prev) => {
      const next = { ...prev, [key]: !prev[key] };
      // Persist to backend
      if (session?.accessToken) {
        fetch(`${API_BASE}/api/settings/notifications`, {
          method: 'PUT',
          headers: {
            Authorization: `Bearer ${session.accessToken}`,
            'Content-Type': 'application/json',
          },
          body: JSON.stringify(next),
        }).catch(() => setError('Failed to save notification preferences.'));
      }
      return next;
    });
  };

  const groups = [
    {
      label: 'Delivery',
      items: [
        { key: 'email_enabled' as const, label: 'Email notifications', desc: 'Receive email for important events (requires SMTP configured)' },
      ],
    },
    {
      label: 'Evidence',
      items: [
        { key: 'evidence_uploaded' as const, label: 'Evidence uploaded', desc: 'When new evidence is added to your cases' },
        { key: 'evidence_destroyed' as const, label: 'Evidence destroyed', desc: 'When evidence is permanently destroyed' },
        { key: 'custody_chain_event' as const, label: 'Custody chain events', desc: 'Changes to the chain of custody' },
      ],
    },
    {
      label: 'Cases',
      items: [
        { key: 'case_status_changed' as const, label: 'Case status changes', desc: 'When a case is closed or archived' },
        { key: 'legal_hold_changed' as const, label: 'Legal hold changes', desc: 'When legal hold is set or released' },
        { key: 'retention_expiring' as const, label: 'Retention expiring', desc: 'When evidence retention periods are about to expire' },
      ],
    },
    {
      label: 'Organization',
      items: [
        { key: 'member_joined' as const, label: 'Member joined', desc: 'When someone joins your organization' },
        { key: 'member_removed' as const, label: 'Member removed', desc: 'When someone is removed from your organization' },
        { key: 'backup_failed' as const, label: 'Backup failures', desc: 'When a scheduled backup fails' },
      ],
    },
  ];

  if (isLoading) {
    return (
      <div className="stagger-in space-y-[var(--space-lg)]">
        {[1, 2, 3, 4].map((i) => (
          <div key={i}>
            <div className="skeleton" style={{ height: '0.75rem', width: '4rem', borderRadius: 'var(--radius-sm)', marginBottom: 'var(--space-sm)' }} />
            <div className="skeleton" style={{ height: '4rem', borderRadius: 'var(--radius-md)' }} />
          </div>
        ))}
      </div>
    );
  }

  return (
    <div className="stagger-in space-y-[var(--space-lg)]">
      {error && <div className="banner-error" style={{ marginBottom: 'var(--space-sm)' }}>{error}</div>}
      {groups.map((group) => (
        <div key={group.label}>
          <h3 className="field-label">{group.label}</h3>
          <div className="card-inset" style={{ overflow: 'hidden' }}>
            {group.items.map((item, i) => (
              <div
                key={item.key}
                className="flex items-center justify-between"
                style={{
                  padding: 'var(--space-sm) var(--space-md)',
                  borderBottom: i < group.items.length - 1 ? '1px solid var(--border-subtle)' : undefined,
                }}
              >
                <div>
                  <p style={{ fontSize: 'var(--text-sm)', fontWeight: 500, color: 'var(--text-primary)' }}>{item.label}</p>
                  <p style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)' }}>{item.desc}</p>
                </div>
                <button
                  type="button"
                  onClick={() => toggle(item.key)}
                  style={{
                    width: '2.5rem',
                    height: '1.375rem',
                    borderRadius: 'var(--radius-full)',
                    backgroundColor: prefs[item.key] ? 'var(--amber-accent)' : 'var(--border-default)',
                    position: 'relative',
                    cursor: 'pointer',
                    border: 'none',
                    transition: 'background-color var(--duration-normal) ease',
                    flexShrink: 0,
                  }}
                  role="switch"
                  aria-checked={prefs[item.key]}
                >
                  <span
                    style={{
                      position: 'absolute',
                      top: '2px',
                      left: prefs[item.key] ? '1.25rem' : '2px',
                      width: '1rem',
                      height: '1rem',
                      borderRadius: 'var(--radius-full)',
                      backgroundColor: 'var(--bg-elevated)',
                      boxShadow: 'var(--shadow-xs)',
                      transition: 'left var(--duration-normal) var(--ease-out-expo)',
                    }}
                  />
                </button>
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}

// ── Security ────────────────────────────────────────────────────────────────

function SecuritySection() {
  const { data: session } = useSession();
  const [systemInfo, setSystemInfo] = useState<{ version?: string; status?: string } | null>(null);

  useEffect(() => {
    fetch(`${API_BASE}/health`)
      .then((r) => r.json())
      .then((d) => setSystemInfo(d.data))
      .catch(() => {});
  }, []);

  return (
    <div className="stagger-in space-y-[var(--space-lg)]">
      {/* Session info */}
      <div>
        <h3 className="field-label">Current Session</h3>
        <div className="card-inset" style={{ padding: 'var(--space-md)' }}>
          <div className="grid grid-cols-2 gap-[var(--space-md)]">
            <div>
              <p style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)' }}>User</p>
              <p style={{ fontSize: 'var(--text-sm)', fontWeight: 500, color: 'var(--text-primary)' }}>
                {session?.user?.name ?? 'Unknown'}
              </p>
            </div>
            <div>
              <p style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)' }}>Email</p>
              <p style={{ fontSize: 'var(--text-sm)', color: 'var(--text-primary)' }}>
                {session?.user?.email ?? '\u2014'}
              </p>
            </div>
            <div>
              <p style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)' }}>Role</p>
              <p style={{ fontSize: 'var(--text-sm)', color: 'var(--text-primary)' }}>
                <span className="badge" style={{ backgroundColor: 'var(--bg-inset)', color: 'var(--text-secondary)' }}>
                  {session?.user?.systemRole?.replace('_', ' ') ?? 'user'}
                </span>
              </p>
            </div>
            <div>
              <p style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)' }}>Session Expires</p>
              <p style={{ fontSize: 'var(--text-sm)', color: 'var(--text-primary)' }}>
                {session?.expires ? new Date(session.expires).toLocaleString() : '\u2014'}
              </p>
            </div>
          </div>
        </div>
      </div>

      {/* Password */}
      <div>
        <h3 className="field-label">Password</h3>
        <p className="text-sm mb-[var(--space-sm)]" style={{ color: 'var(--text-secondary)' }}>
          Password management is handled by your identity provider (Keycloak).
        </p>
        <a
          href={`${process.env.NEXT_PUBLIC_KEYCLOAK_URL ?? 'http://localhost:8180'}/realms/${process.env.NEXT_PUBLIC_KEYCLOAK_REALM ?? 'vaultkeeper'}/account/`}
          target="_blank"
          rel="noopener noreferrer"
          className="btn-secondary"
          style={{ fontSize: 'var(--text-sm)' }}
        >
          Manage Account in Keycloak
        </a>
      </div>

      {/* System */}
      <div>
        <h3 className="field-label">System Information</h3>
        <div className="card-inset" style={{ padding: 'var(--space-md)' }}>
          <div className="grid grid-cols-2 gap-[var(--space-md)]">
            <div>
              <p style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)' }}>Version</p>
              <p className="font-[family-name:var(--font-mono)]" style={{ fontSize: 'var(--text-sm)', color: 'var(--text-primary)' }}>
                {systemInfo?.version ?? '\u2014'}
              </p>
            </div>
            <div>
              <p style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)' }}>Status</p>
              <p style={{ fontSize: 'var(--text-sm)' }}>
                <span className="badge" style={{
                  backgroundColor: systemInfo?.status === 'healthy' ? 'var(--status-active-bg)' : 'var(--status-hold-bg)',
                  color: systemInfo?.status === 'healthy' ? 'var(--status-active)' : 'var(--status-hold)',
                }}>
                  {systemInfo?.status ?? 'checking...'}
                </span>
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

// ── API Keys (reused from previous) ─────────────────────────────────────────

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  const handleCopy = async () => {
    try { await navigator.clipboard.writeText(text); setCopied(true); setTimeout(() => setCopied(false), 2000); } catch { /* */ }
  };
  return (
    <button type="button" onClick={handleCopy} className="btn-ghost" style={{ fontSize: 'var(--text-xs)' }}>
      {copied ? 'Copied' : 'Copy'}
    </button>
  );
}

function ApiKeysSection() {
  const { data: session } = useSession();
  const [keys, setKeys] = useState<ApiKey[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [newKeyName, setNewKeyName] = useState('');
  const [newKeyPermissions, setNewKeyPermissions] = useState<'read' | 'read_write'>('read');
  const [creating, setCreating] = useState(false);
  const [createdRawKey, setCreatedRawKey] = useState<string | null>(null);
  const [revokeTarget, setRevokeTarget] = useState<string | null>(null);
  const [revoking, setRevoking] = useState(false);

  const fetchKeys = useCallback(async () => {
    if (!session?.accessToken) return;
    setIsLoading(true);
    try {
      const result = await listApiKeys(session.accessToken);
      if (result.data) setKeys(result.data);
      else if (result.error) setError(typeof result.error === 'string' ? result.error : 'Failed to load API keys');
    } catch { setError('Failed to load API keys'); }
    finally { setIsLoading(false); }
  }, [session?.accessToken]);

  useEffect(() => { fetchKeys(); }, [fetchKeys]);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!session?.accessToken) return;
    setError(''); setSuccess(''); setCreating(true);
    try {
      const result = await createApiKey(session.accessToken, newKeyName.trim(), newKeyPermissions);
      if (result.data) {
        setCreatedRawKey(result.data.raw_key);
        setKeys((prev) => [result.data!.key, ...prev]);
        setNewKeyName(''); setNewKeyPermissions('read'); setShowCreateForm(false);
        setSuccess('API key created.');
      } else if (result.error) setError(typeof result.error === 'string' ? result.error : 'Failed to create API key');
    } catch { setError('Failed to create API key'); }
    finally { setCreating(false); }
  };

  const handleRevoke = async (id: string) => {
    if (!session?.accessToken) return;
    setError(''); setSuccess(''); setRevoking(true);
    try {
      const result = await revokeApiKey(session.accessToken, id);
      if (result.error) setError(typeof result.error === 'string' ? result.error : 'Failed to revoke');
      else { setKeys((prev) => prev.filter((k) => k.id !== id)); setSuccess('Key revoked.'); setRevokeTarget(null); }
    } catch { setError('Failed to revoke'); }
    finally { setRevoking(false); }
  };

  const fmtDate = (s: string) => new Date(s).toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' });

  return (
    <div className="stagger-in">
      <div className="flex items-center justify-between" style={{ marginBottom: 'var(--space-sm)' }}>
        <div>
          <h3 className="field-label" style={{ marginBottom: 0 }}>API Keys</h3>
          <p style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)', marginTop: '0.125rem' }}>
            Programmatic access to VaultKeeper. Keys are shown only once at creation.
          </p>
        </div>
        {!showCreateForm && (
          <button type="button" onClick={() => { setShowCreateForm(true); setCreatedRawKey(null); setError(''); setSuccess(''); }} className="btn-primary" style={{ fontSize: 'var(--text-sm)' }}>
            Create API Key
          </button>
        )}
      </div>

      {error && <div className="banner-error" style={{ marginBottom: 'var(--space-sm)' }}>{error}</div>}
      {success && !createdRawKey && <div className="banner-success" style={{ marginBottom: 'var(--space-sm)' }}>{success}</div>}

      {createdRawKey && (
        <div className="card-inset" style={{ padding: 'var(--space-md)', marginBottom: 'var(--space-sm)', border: '1.5px solid var(--amber-accent)' }}>
          <p style={{ fontSize: 'var(--text-sm)', fontWeight: 600, color: 'var(--text-primary)', marginBottom: 'var(--space-xs)' }}>
            Save this key now — it will not be shown again.
          </p>
          <div className="flex items-center gap-[var(--space-sm)]">
            <code className="flex-1 font-[family-name:var(--font-mono)] break-all" style={{
              fontSize: 'var(--text-sm)', padding: 'var(--space-xs) var(--space-sm)', borderRadius: 'var(--radius-sm)',
              backgroundColor: 'var(--bg-primary)', border: '1px solid var(--border-default)', color: 'var(--text-primary)',
            }}>
              {createdRawKey}
            </code>
            <CopyButton text={createdRawKey} />
          </div>
          <button type="button" onClick={() => setCreatedRawKey(null)} className="link-subtle" style={{ fontSize: 'var(--text-xs)', marginTop: 'var(--space-xs)', background: 'none', border: 'none', cursor: 'pointer' }}>
            Dismiss
          </button>
        </div>
      )}

      {showCreateForm && (
        <form onSubmit={handleCreate} className="card-inset" style={{ padding: 'var(--space-md)', marginBottom: 'var(--space-sm)' }}>
          <div className="flex items-end gap-[var(--space-sm)]">
            <div className="flex-1">
              <label className="field-label">Name</label>
              <input type="text" value={newKeyName} onChange={(e) => setNewKeyName(e.target.value)} placeholder="e.g. CI Pipeline" className="input-field" maxLength={100} required />
            </div>
            <div>
              <label className="field-label">Permissions</label>
              <select value={newKeyPermissions} onChange={(e) => setNewKeyPermissions(e.target.value as 'read' | 'read_write')} className="input-field">
                <option value="read">Read Only</option>
                <option value="read_write">Read & Write</option>
              </select>
            </div>
            <button type="submit" disabled={creating || !newKeyName.trim()} className="btn-primary" style={{ fontSize: 'var(--text-sm)' }}>
              {creating ? 'Creating\u2026' : 'Create'}
            </button>
            <button type="button" onClick={() => { setShowCreateForm(false); setNewKeyName(''); }} className="btn-ghost" style={{ fontSize: 'var(--text-sm)' }}>
              Cancel
            </button>
          </div>
        </form>
      )}

      {isLoading ? (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-xs)' }}>
          {[1, 2, 3].map((i) => <div key={i} className="skeleton" style={{ height: '3rem', borderRadius: 'var(--radius-md)' }} />)}
        </div>
      ) : keys.length === 0 ? (
        <div className="card-inset" style={{ padding: 'var(--space-xl)', textAlign: 'center' }}>
          <p style={{ fontSize: 'var(--text-sm)', color: 'var(--text-tertiary)' }}>No API keys yet.</p>
        </div>
      ) : (
        <div className="card-inset" style={{ overflow: 'hidden', padding: 0 }}>
          <table className="w-full" style={{ fontSize: 'var(--text-sm)', borderCollapse: 'collapse' }}>
            <thead>
              <tr style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                <th className="text-left field-label" style={{ padding: 'var(--space-sm) var(--space-md)', marginBottom: 0 }}>Name</th>
                <th className="text-left field-label" style={{ padding: 'var(--space-sm) var(--space-md)', marginBottom: 0 }}>Permissions</th>
                <th className="text-left field-label" style={{ padding: 'var(--space-sm) var(--space-md)', marginBottom: 0 }}>Created</th>
                <th className="text-left field-label" style={{ padding: 'var(--space-sm) var(--space-md)', marginBottom: 0 }}>Last Used</th>
                <th className="text-right field-label" style={{ padding: 'var(--space-sm) var(--space-md)', marginBottom: 0 }}></th>
              </tr>
            </thead>
            <tbody>
              {keys.map((key) => (
                <tr key={key.id} style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                  <td style={{ padding: 'var(--space-sm) var(--space-md)', fontWeight: 500, color: 'var(--text-primary)' }}>{key.name}</td>
                  <td style={{ padding: 'var(--space-sm) var(--space-md)' }}>
                    <span className="badge" style={{ backgroundColor: 'var(--bg-inset)', color: 'var(--text-secondary)' }}>
                      {key.permissions === 'read_write' ? 'Read & Write' : 'Read Only'}
                    </span>
                  </td>
                  <td style={{ padding: 'var(--space-sm) var(--space-md)', color: 'var(--text-tertiary)' }}>{fmtDate(key.created_at)}</td>
                  <td style={{ padding: 'var(--space-sm) var(--space-md)', color: 'var(--text-tertiary)' }}>{key.last_used_at ? fmtDate(key.last_used_at) : 'Never'}</td>
                  <td className="text-right" style={{ padding: 'var(--space-sm) var(--space-md)' }}>
                    {revokeTarget === key.id ? (
                      <span className="flex items-center justify-end gap-[var(--space-xs)]">
                        <button type="button" onClick={() => handleRevoke(key.id)} disabled={revoking} className="btn-danger" style={{ fontSize: 'var(--text-xs)' }}>
                          {revoking ? 'Revoking\u2026' : 'Confirm'}
                        </button>
                        <button type="button" onClick={() => setRevokeTarget(null)} className="btn-ghost" style={{ fontSize: 'var(--text-xs)' }}>Cancel</button>
                      </span>
                    ) : (
                      <button type="button" onClick={() => setRevokeTarget(key.id)} style={{
                        fontSize: 'var(--text-xs)', fontWeight: 500, color: 'var(--status-hold)',
                        background: 'none', border: 'none', cursor: 'pointer',
                      }}>Revoke</button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

// ── Organization Settings ───────────────────────────────────────────────────

function OrganizationSection() {
  const { activeOrg, isOrgAdmin, isOrgOwner } = useOrg();
  const { data: session } = useSession();
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  useEffect(() => {
    if (activeOrg) {
      setName(activeOrg.name);
      setDescription(activeOrg.description);
    }
  }, [activeOrg]);

  if (!activeOrg) return <p style={{ fontSize: 'var(--text-sm)', color: 'var(--text-tertiary)' }}>No organization selected.</p>;
  if (!isOrgAdmin) return <p style={{ fontSize: 'var(--text-sm)', color: 'var(--text-tertiary)' }}>Only organization admins can manage settings.</p>;

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true); setError(''); setSuccess('');
    try {
      const res = await fetch(`${API_BASE}/api/organizations/${activeOrg.id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${session?.accessToken}` },
        body: JSON.stringify({ name: name.trim(), description: description.trim() }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => null);
        setError(body?.error ?? 'Failed to update');
      } else {
        setSuccess('Organization updated.');
      }
    } catch { setError('Network error'); }
    finally { setSaving(false); }
  };

  return (
    <div className="stagger-in space-y-[var(--space-lg)]">
      <div>
        <h3 className="field-label">Organization Details</h3>
        <p style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)', marginBottom: 'var(--space-sm)' }}>
          Managing: <strong style={{ color: 'var(--text-primary)' }}>{activeOrg.name}</strong>
        </p>
        <form onSubmit={handleSave} className="space-y-[var(--space-md)]">
          <div>
            <label className="field-label">Name</label>
            <input type="text" value={name} onChange={(e) => setName(e.target.value)} className="input-field" required />
          </div>
          <div>
            <label className="field-label">Description</label>
            <textarea value={description} onChange={(e) => setDescription(e.target.value)} rows={3} className="input-field resize-y" />
          </div>
          {error && <div className="banner-error">{error}</div>}
          {success && <div className="banner-success">{success}</div>}
          <button type="submit" disabled={saving} className="btn-primary" style={{ fontSize: 'var(--text-sm)' }}>
            {saving ? 'Saving\u2026' : 'Save Changes'}
          </button>
        </form>
      </div>

      <div>
        <h3 className="field-label">Members</h3>
        <p style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)', marginBottom: 'var(--space-sm)' }}>
          {activeOrg.member_count} member{activeOrg.member_count !== 1 ? 's' : ''} &middot; {activeOrg.case_count} case{activeOrg.case_count !== 1 ? 's' : ''}
        </p>
        <Link href={`/en/organizations/${activeOrg.id}`} className="btn-secondary" style={{ fontSize: 'var(--text-sm)' }}>
          Manage Members
        </Link>
      </div>

      {isOrgOwner && (
        <div style={{ padding: 'var(--space-md)', borderRadius: 'var(--radius-md)', border: '1px solid var(--status-hold-bg)' }}>
          <h3 className="field-label" style={{ color: 'var(--status-hold)' }}>Danger Zone</h3>
          <p style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)', marginBottom: 'var(--space-sm)' }}>
            Deleting an organization is permanent. All cases must be archived first.
          </p>
          <button type="button" className="btn-danger" style={{ fontSize: 'var(--text-sm)' }} disabled>
            Delete Organization
          </button>
        </div>
      )}
    </div>
  );
}

// ── Main Settings Page ──────────────────────────────────────────────────────

export default function SettingsPage() {
  const [activeTab, setActiveTab] = useState<SettingsTab>('general');
  const { isOrgAdmin } = useOrg();

  const visibleTabs = SETTINGS_TABS.filter((t) => !t.adminOnly || isOrgAdmin);

  return (
    <Shell>
      <div className="px-[var(--space-lg)] py-[var(--space-xl)]">
        <div style={{ marginBottom: 'var(--space-lg)' }}>
          <h1
            className="font-[family-name:var(--font-heading)]"
            style={{ fontSize: 'var(--text-xl)', color: 'var(--text-primary)' }}
          >
            Settings
          </h1>
        </div>

        <div className="flex gap-0" style={{ minHeight: 'calc(100vh - 16rem)' }}>
          {/* Settings sidebar */}
          <nav className="hidden lg:block w-[180px] shrink-0" style={{ paddingRight: 'var(--space-md)' }}>
            <div className="sticky" style={{ top: 'var(--space-lg)' }}>
              <div className="flex flex-col gap-[1px]">
                {visibleTabs.map((tab) => (
                  <button
                    key={tab.key}
                    type="button"
                    onClick={() => setActiveTab(tab.key)}
                    style={{
                      padding: 'var(--space-xs) var(--space-sm)',
                      borderRadius: 'var(--radius-sm)',
                      fontSize: 'var(--text-sm)',
                      fontWeight: activeTab === tab.key ? 500 : 400,
                      color: activeTab === tab.key ? 'var(--text-primary)' : 'var(--text-secondary)',
                      backgroundColor: activeTab === tab.key ? 'var(--bg-inset)' : 'transparent',
                      borderLeft: activeTab === tab.key ? '2px solid var(--amber-accent)' : '2px solid transparent',
                      textAlign: 'left',
                      cursor: 'pointer',
                      border: 'none',
                      borderLeftStyle: 'solid',
                      borderLeftWidth: '2px',
                      borderLeftColor: activeTab === tab.key ? 'var(--amber-accent)' : 'transparent',
                      background: activeTab === tab.key ? 'var(--bg-inset)' : 'transparent',
                      transition: 'all var(--duration-fast) ease',
                    }}
                  >
                    {tab.label}
                  </button>
                ))}
              </div>
            </div>
          </nav>

          {/* Content */}
          <div className="flex-1 min-w-0 lg:pl-[var(--space-lg)] lg:border-l" style={{ borderColor: 'var(--border-subtle)' }}>
            {/* Mobile tab picker */}
            <div className="lg:hidden" style={{ marginBottom: 'var(--space-md)' }}>
              <select
                className="input-field"
                value={activeTab}
                onChange={(e) => setActiveTab(e.target.value as SettingsTab)}
              >
                {visibleTabs.map((tab) => (
                  <option key={tab.key} value={tab.key}>{tab.label}</option>
                ))}
              </select>
            </div>

            {activeTab === 'general' && <GeneralSection />}
            {activeTab === 'notifications' && <NotificationsSection />}
            {activeTab === 'security' && <SecuritySection />}
            {activeTab === 'api-keys' && <ApiKeysSection />}
            {activeTab === 'organization' && <OrganizationSection />}
          </div>
        </div>
      </div>
    </Shell>
  );
}
