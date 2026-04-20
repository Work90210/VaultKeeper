'use client';

import { useState, useEffect, useCallback } from 'react';
import { useSession } from 'next-auth/react';
import { useSearchParams } from 'next/navigation';
import { Shell } from '@/components/layout/shell';
import { useOrg } from '@/hooks/use-org';
import { RoleEditor as RoleEditorComponent } from '@/components/settings/role-editor';
import type { ApiKey, OrgMembership, OrgInvitation } from '@/types';
import { listApiKeys, createApiKey, revokeApiKey } from '@/lib/apikeys-api';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

type SettingsTab =
  | 'team' | 'roles' | 'invites'
  | 'organization' | 'orgs' | 'sso'
  | 'policy' | 'keys' | 'storage' | 'api-keys'
  | 'danger';

// ── Reusable layout primitives (matching design) ────────────────────────────

function Panel({
  header,
  children,
  headerStyle,
  bodyClass,
}: {
  header?: React.ReactNode;
  children: React.ReactNode;
  headerStyle?: React.CSSProperties;
  bodyClass?: string;
}) {
  return (
    <div className="panel">
      {header && (
        <div className="panel-h" style={headerStyle}>
          {header}
        </div>
      )}
      <div className={`panel-body${bodyClass ? ` ${bodyClass}` : ''}`}>
        {children}
      </div>
    </div>
  );
}

function PanelRaw({
  header,
  children,
  headerStyle,
}: {
  header?: React.ReactNode;
  children: React.ReactNode;
  headerStyle?: React.CSSProperties;
}) {
  return (
    <div className="panel">
      {header && (
        <div className="panel-h" style={headerStyle}>
          {header}
        </div>
      )}
      {children}
    </div>
  );
}

function PanelHeader({
  title,
  meta,
  titleStyle,
}: {
  title: string;
  meta: string;
  titleStyle?: React.CSSProperties;
}) {
  return (
    <>
      <h3 style={titleStyle}>{title}</h3>
      <span className="meta">{meta}</span>
    </>
  );
}

function KVList({ pairs }: { pairs: [string, React.ReactNode][] }) {
  return (
    <dl className="kvs">
      {pairs.map(([label, value]) => (
        <div key={label} style={{ display: 'contents' }}>
          <dt>{label}</dt>
          <dd>{value}</dd>
        </div>
      ))}
    </dl>
  );
}

function Toggle({
  on,
  onToggle,
}: {
  on: boolean;
  onToggle?: () => void;
}) {
  return (
    <button
      type="button"
      className={`toggle${on ? ' on' : ''}`}
      onClick={onToggle}
      role="switch"
      aria-checked={on}
    />
  );
}

function RoleBadge({ role }: { role: string }) {
  const label = role.charAt(0).toUpperCase() + role.slice(1).replace(/_/g, ' ');
  return <span className={`role-badge ${role}`}>{label}</span>;
}

function StatusDot({ status, label }: { status: 'online' | 'offline'; label: string }) {
  const color = status === 'online' ? 'var(--ok)' : 'var(--line-2)';
  return (
    <span style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
      <span style={{ width: '6px', height: '6px', borderRadius: '50%', background: color }} />
      <span style={{ fontSize: '12px', color: 'var(--muted)' }}>{label}</span>
    </span>
  );
}

function LinkArrow({ text, style, href }: { text: string; style?: React.CSSProperties; href?: string }) {
  const Tag = href ? 'a' : 'span';
  return (
    <Tag className="linkarrow" href={href} style={{ cursor: 'pointer', ...style }}>
      {text}
    </Tag>
  );
}

function DangerLink({ text, onClick }: { text: string; onClick?: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      style={{
        fontSize: '12px',
        color: '#b35c5c',
        cursor: 'pointer',
        background: 'none',
        border: 'none',
        fontFamily: 'inherit',
      }}
    >
      {text}
    </button>
  );
}

function GridHeader({ columns, templateColumns }: { columns: string[]; templateColumns: string }) {
  return (
    <div
      style={{
        fontFamily: "'JetBrains Mono', monospace",
        fontSize: '10px',
        letterSpacing: '.06em',
        textTransform: 'uppercase',
        color: 'var(--muted)',
        display: 'grid',
        gridTemplateColumns: templateColumns,
        gap: '16px',
        padding: '10px 18px',
        borderBottom: '1px solid var(--line)',
        background: 'color-mix(in oklab, var(--paper) 50%, var(--bg-2))',
      }}
    >
      {columns.map((c) => (
        <span key={c}>{c}</span>
      ))}
    </div>
  );
}

function GridRow({ cells, templateColumns }: { cells: React.ReactNode[]; templateColumns: string }) {
  return (
    <div
      style={{
        display: 'grid',
        gridTemplateColumns: templateColumns,
        gap: '16px',
        alignItems: 'center',
        padding: '14px 18px',
        borderBottom: '1px solid var(--line)',
        fontSize: '13.5px',
      }}
    >
      {cells.map((cell, i) => (
        <span key={i}>{cell}</span>
      ))}
    </div>
  );
}

function InviteBar({
  buttonLabel,
  roleOptions,
  onSubmit,
  email,
  setEmail,
  role,
  setRole,
  submitting,
}: {
  buttonLabel: string;
  roleOptions?: string[];
  onSubmit: () => void;
  email: string;
  setEmail: (v: string) => void;
  role: string;
  setRole: (v: string) => void;
  submitting?: boolean;
}) {
  const opts = roleOptions ?? ['Analyst', 'Lead Investigator', 'Clerk', 'Viewer', 'Admin'];
  return (
    <div className="invite-bar">
      <input
        type="email"
        placeholder="Email address"
        value={email}
        onChange={(e) => setEmail(e.target.value)}
      />
      <select value={role} onChange={(e) => setRole(e.target.value)}>
        {opts.map((r) => (
          <option key={r} value={r.toLowerCase().replace(/ /g, '_')}>{r}</option>
        ))}
      </select>
      <button
        type="button"
        className="btn sm"
        onClick={onSubmit}
        disabled={submitting || !email.trim()}
      >
        {submitting ? 'Sending...' : buttonLabel}
      </button>
    </div>
  );
}

function _CaseChip({ name }: { name: string }) {
  return (
    <span className="case-chip">
      <span className="cd" />
      {name}
    </span>
  );
}

function MemberAvatar({ letter, color }: { letter: string; color: string }) {
  return (
    <span className="av" style={{ background: color }}>
      {letter}
    </span>
  );
}

// ── Team Members ────────────────────────────────────────────────────────────

function TeamSection() {
  const { data: session } = useSession();
  const { activeOrg } = useOrg();
  const [members, setMembers] = useState<OrgMembership[]>([]);
  const [loading, setLoading] = useState(true);
  const [invEmail, setInvEmail] = useState('');
  const [invRole, setInvRole] = useState('analyst');
  const [inviting, setInviting] = useState(false);

  const fetchMembers = useCallback(async () => {
    if (!activeOrg?.id || !session?.accessToken) return;
    setLoading(true);
    try {
      const res = await fetch(`${API_BASE}/api/organizations/${activeOrg.id}/members`, {
        headers: { Authorization: `Bearer ${session.accessToken}` },
      });
      if (res.ok) {
        const body = await res.json();
        setMembers(Array.isArray(body) ? body : (body.data ?? []));
      }
    } catch { /* empty */ } finally { setLoading(false); }
  }, [activeOrg?.id, session?.accessToken]);

  useEffect(() => { fetchMembers(); }, [fetchMembers]);

  const handleInvite = async () => {
    if (!activeOrg?.id || !session?.accessToken || !invEmail.trim()) return;
    setInviting(true);
    try {
      await fetch(`${API_BASE}/api/organizations/${activeOrg.id}/invitations`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${session.accessToken}`, 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: invEmail.trim(), role: invRole }),
      });
      setInvEmail('');
      fetchMembers();
    } catch { /* empty */ } finally { setInviting(false); }
  };

  const AVATAR_COLORS = ['#c87e5e', '#4a6b3a', '#3a4a6b', '#6b3a4a', '#8a6a3a', '#5b4a6b'];

  if (loading) {
    return (
      <div className="panel">
        <div className="panel-h">
          <PanelHeader title="Team members" meta="loading..." />
        </div>
        <div className="panel-body">
          {[1, 2, 3].map((i) => (
            <div key={i} className="skeleton" style={{ height: '3rem', borderRadius: '8px', marginBottom: '8px' }} />
          ))}
        </div>
      </div>
    );
  }

  const cols = '36px 1.4fr 1fr 1fr auto';

  return (
    <PanelRaw
      header={<PanelHeader title="Team members" meta={`${members.length} people`} />}
    >
      <GridHeader columns={['', 'Member', 'Role', 'Joined', 'Status']} templateColumns={cols} />
      {members.map((m, i) => {
        const displayName = m.display_name || m.email || m.user_id.slice(0, 8);
        const initial = displayName.charAt(0).toUpperCase();
        const color = AVATAR_COLORS[i % AVATAR_COLORS.length];
        return (
          <div key={m.id} className="member-row">
            <MemberAvatar letter={initial} color={color} />
            <span className="name">
              {displayName}
              {m.email && <small>{m.email}</small>}
            </span>
            <span>
              <RoleBadge role={m.role} />
            </span>
            <span style={{ fontSize: '12px', color: 'var(--muted)' }}>
              {m.joined_at ? new Date(m.joined_at).toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' }) : '\u2014'}
            </span>
            <StatusDot status="offline" label={m.joined_at ? 'Active' : '\u2014'} />
          </div>
        );
      })}
      <InviteBar
        buttonLabel="Invite"
        email={invEmail}
        setEmail={setInvEmail}
        role={invRole}
        setRole={setInvRole}
        onSubmit={handleInvite}
        submitting={inviting}
      />
    </PanelRaw>
  );
}

// ── Roles & Permissions ─────────────────────────────────────────────────────

function RolesSection() {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '18px' }}>
      <Panel
        header={<PanelHeader title="Roles & permissions" meta="" />}
      >
        <p style={{ fontSize: '13px', color: 'var(--muted)', lineHeight: 1.5, marginBottom: '14px' }}>
          Define what each case role can do. System roles can be customized but not deleted.
        </p>
        <RoleEditorComponent />
      </Panel>
    </div>
  );
}

// ── Pending Invites ─────────────────────────────────────────────────────────

function InvitesSection() {
  const { data: session } = useSession();
  const { activeOrg } = useOrg();
  const [invitations, setInvitations] = useState<OrgInvitation[]>([]);
  const [loading, setLoading] = useState(true);
  const [revokingId, setRevokingId] = useState<string | null>(null);
  const [invEmail, setInvEmail] = useState('');
  const [invRole, setInvRole] = useState('analyst');
  const [inviting, setInviting] = useState(false);

  const fetchInvitations = useCallback(async () => {
    if (!activeOrg?.id || !session?.accessToken) return;
    setLoading(true);
    try {
      const res = await fetch(`${API_BASE}/api/organizations/${activeOrg.id}/invitations`, {
        headers: { Authorization: `Bearer ${session.accessToken}` },
      });
      if (res.ok) {
        const body = await res.json();
        const all: OrgInvitation[] = Array.isArray(body) ? body : (body.data ?? []);
        setInvitations(all.filter((inv) => inv.status === 'pending'));
      }
    } catch { /* empty */ } finally { setLoading(false); }
  }, [activeOrg?.id, session?.accessToken]);

  useEffect(() => { fetchInvitations(); }, [fetchInvitations]);

  const handleRevoke = async (inviteId: string) => {
    if (!activeOrg?.id || !session?.accessToken) return;
    setRevokingId(inviteId);
    try {
      await fetch(`${API_BASE}/api/organizations/${activeOrg.id}/invitations/${inviteId}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${session.accessToken}`, 'Content-Type': 'application/json' },
      });
      setInvitations((prev) => prev.filter((inv) => inv.id !== inviteId));
    } catch { /* empty */ } finally { setRevokingId(null); }
  };

  const handleInvite = async () => {
    if (!activeOrg?.id || !session?.accessToken || !invEmail.trim()) return;
    setInviting(true);
    try {
      await fetch(`${API_BASE}/api/organizations/${activeOrg.id}/invitations`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${session.accessToken}`, 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: invEmail.trim(), role: invRole }),
      });
      setInvEmail('');
      fetchInvitations();
    } catch { /* empty */ } finally { setInviting(false); }
  };

  if (loading) {
    return (
      <div className="panel">
        <div className="panel-h">
          <PanelHeader title="Pending invites" meta="loading..." />
        </div>
        <div className="panel-body">
          {[1, 2].map((i) => (
            <div key={i} className="skeleton" style={{ height: '3rem', borderRadius: '8px', marginBottom: '8px' }} />
          ))}
        </div>
      </div>
    );
  }

  const cols = '1.5fr 1fr 1fr auto';

  return (
    <PanelRaw
      header={<PanelHeader title="Pending invites" meta={`${invitations.length} pending`} />}
    >
      <GridHeader columns={['Email', 'Role', 'Invited by', 'Actions']} templateColumns={cols} />
      {invitations.map((inv) => (
        <GridRow
          key={inv.id}
          templateColumns={cols}
          cells={[
            <span key="email" style={{ fontWeight: 500 }}>{inv.email}</span>,
            <RoleBadge key="role" role={inv.role} />,
            <span key="by" style={{ color: 'var(--muted)', fontSize: '12px' }}>
              {inv.expires_at ? `Expires ${new Date(inv.expires_at).toLocaleDateString('en-GB', { day: '2-digit', month: 'short' })}` : '\u2014'}
            </span>,
            <span key="actions" style={{ display: 'flex', gap: '8px' }}>
              <DangerLink
                text={revokingId === inv.id ? 'Revoking...' : 'Revoke'}
                onClick={() => handleRevoke(inv.id)}
              />
            </span>,
          ]}
        />
      ))}
      <InviteBar
        buttonLabel="Send invite"
        email={invEmail}
        setEmail={setInvEmail}
        role={invRole}
        setRole={setInvRole}
        onSubmit={handleInvite}
        submitting={inviting}
      />
    </PanelRaw>
  );
}

// ── Organisation General ────────────────────────────────────────────────────

function OrgGeneralSection() {
  const { activeOrg } = useOrg();
  const { data: session } = useSession();
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [_saving, setSaving] = useState(false);
  const [saveMsg, setSaveMsg] = useState<{ type: 'error' | 'success'; text: string } | null>(null);

  useEffect(() => {
    if (activeOrg) {
      setName(activeOrg.name);
      setDescription(activeOrg.description);
    }
  }, [activeOrg]);

  const _handleSave = async () => {
    if (!activeOrg?.id || !session?.accessToken) return;
    setSaving(true);
    setSaveMsg(null);
    try {
      const res = await fetch(`${API_BASE}/api/organizations/${activeOrg.id}`, {
        method: 'PATCH',
        headers: { Authorization: `Bearer ${session.accessToken}`, 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: name.trim(), description: description.trim() }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => null);
        setSaveMsg({ type: 'error', text: body?.error ?? 'Failed to update' });
      } else {
        setSaveMsg({ type: 'success', text: 'Organisation updated.' });
        setTimeout(() => setSaveMsg(null), 3000);
      }
    } catch {
      setSaveMsg({ type: 'error', text: 'Network error' });
    } finally {
      setSaving(false);
    }
  };

  if (!activeOrg) {
    return (
      <Panel header={<PanelHeader title="Organisation" meta="" />}>
        <p style={{ color: 'var(--muted)', fontSize: '13px' }}>No organisation selected.</p>
      </Panel>
    );
  }

  return (
    <Panel header={<PanelHeader title="Organisation" meta="" />}>
      <KVList
        pairs={[
          ['Display name', (
            <span key="name">
              <strong>{activeOrg.name}</strong>
              <LinkArrow text="edit \u2192" style={{ marginLeft: '10px', fontSize: '12px' }} />
            </span>
          )],
          ['Instance ID', (
            <span key="id">
              <code>{activeOrg.id.slice(0, 16)}</code>
            </span>
          )],
          ['Description', (
            <span key="desc">{activeOrg.description || '\u2014'}</span>
          )],
        ]}
      />
      {saveMsg && (
        <div
          className={saveMsg.type === 'error' ? 'banner-error' : 'banner-success'}
          style={{ marginTop: '14px' }}
        >
          {saveMsg.text}
        </div>
      )}
    </Panel>
  );
}

// ── Switch Organisation ─────────────────────────────────────────────────────

function OrgSwitchSection() {
  const { data: session } = useSession();
  const { activeOrg } = useOrg();
  const [orgs, setOrgs] = useState<Array<{ id: string; name: string; description: string }>>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!session?.accessToken) return;
    setLoading(true);
    fetch(`${API_BASE}/api/organizations`, {
      headers: { Authorization: `Bearer ${session.accessToken}` },
    })
      .then((r) => r.json())
      .then((body) => {
        setOrgs(Array.isArray(body) ? body : (body.data ?? []));
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, [session?.accessToken]);

  const AVATAR_COLORS = ['var(--ink)', '#4a6b3a', '#3a4a6b', '#6b3a4a', '#8a6a3a', '#5b4a6b'];

  if (loading) {
    return (
      <div className="panel">
        <div className="panel-h">
          <PanelHeader title="Switch organisation" meta="loading..." />
        </div>
        <div className="panel-body">
          {[1, 2, 3].map((i) => (
            <div key={i} className="skeleton" style={{ height: '3rem', borderRadius: '8px', marginBottom: '8px' }} />
          ))}
        </div>
      </div>
    );
  }

  return (
    <>
      <PanelRaw
        header={<PanelHeader title="Switch organisation" meta="Admin only" />}
      >
        {orgs.map((org, i) => {
          const isCurrent = activeOrg?.id === org.id;
          const letter = org.name.charAt(0).toUpperCase();
          const color = AVATAR_COLORS[i % AVATAR_COLORS.length];
          return (
            <div key={org.id} className={`org-switch-row${isCurrent ? ' current' : ''}`}>
              <span className="oa" style={{ background: color }}>{letter}</span>
              <div>
                <div style={{ fontWeight: 500, fontSize: '14px' }}>{org.name}</div>
                <div style={{ fontSize: '12px', color: 'var(--muted)', marginTop: '1px' }}>
                  {org.description || '\u2014'}
                </div>
              </div>
              {isCurrent ? (
                <span
                  style={{
                    fontFamily: "'JetBrains Mono', monospace",
                    fontSize: '10px',
                    letterSpacing: '.06em',
                    textTransform: 'uppercase',
                    color: 'var(--ok)',
                  }}
                >
                  Current
                </span>
              ) : (
                <LinkArrow text="Switch \u2192" style={{ fontSize: '12px' }} />
              )}
            </div>
          );
        })}
      </PanelRaw>
      <p style={{ fontSize: '12px', color: 'var(--muted)', marginTop: '12px', lineHeight: 1.6 }}>
        Organisation switching is restricted to Admin roles. All switches are logged as sealed audit events.
        Switching does not revoke access to other organisations.
      </p>
    </>
  );
}

// ── SSO & Identity ──────────────────────────────────────────────────────────

function SSOSection() {
  const [mfaEnabled, setMfaEnabled] = useState(true);
  const [autoProvision, setAutoProvision] = useState(true);

  return (
    <Panel header={<PanelHeader title="SSO & identity" meta="SAML 2.0" />}>
      <KVList
        pairs={[
          ['Provider', 'Keycloak \u00b7 self-hosted'],
          ['Protocol', 'SAML 2.0 \u00b7 OIDC fallback enabled'],
          ['MFA required', (
            <span key="mfa" style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
              <Toggle on={mfaEnabled} onToggle={() => setMfaEnabled((prev) => !prev)} />
              All users must use hardware token or TOTP
            </span>
          )],
          ['Session timeout', '8 hours idle \u00b7 24 hours max'],
          ['Auto-provision', (
            <span key="auto" style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
              <Toggle on={autoProvision} onToggle={() => setAutoProvision((prev) => !prev)} />
              New SSO users get Viewer role by default
            </span>
          )],
          ['Directory sync', 'SCIM 2.0 \u00b7 every 15 min'],
        ]}
      />
    </Panel>
  );
}

// ── Retention Policy ────────────────────────────────────────────────────────

function PolicySection() {
  return (
    <Panel header={<PanelHeader title="Retention policy" meta="quarterly review" />}>
      <KVList
        pairs={[
          ['Default retention', 'Case closed + 50 years, then review'],
          ['Witness records', 'Case closed + 75 years \u00b7 never auto-destroyed'],
          ['Counter-evidence', 'Same as parent case, non-deletable'],
          ['Auto review cadence', 'Every 90 days \u00b7 notify custodian'],
          ['Legal hold override', 'Blocks all deletion, regardless of policy'],
          ['Policy history', (
            <span key="hist">
              Policy history is immutable \u00b7{' '}
              <LinkArrow text="diff \u2192" style={{ fontSize: '12px' }} />
            </span>
          )],
        ]}
      />
    </Panel>
  );
}

// ── Keys & Ceremonies ───────────────────────────────────────────────────────

function KeysSection() {
  const keysData = [
    { n: 'Key A', who: 'Admin', hw: 'YubiHSM2', last: 'signed \u00b7 recently', status: 'active' as const },
    { n: 'Key B', who: 'Operator', hw: 'YubiHSM2', last: 'signed \u00b7 recently', status: 'active' as const },
    { n: 'Key C', who: 'Recovery', hw: 'Nitrokey HSM', last: 'offline', status: 'offline' as const },
  ];

  return (
    <Panel header={<PanelHeader title="Keys & ceremonies" meta="2-of-3 quorum" />}>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: '16px', marginBottom: '18px' }}>
        {keysData.map((key) => {
          const dot = key.status === 'active' ? 'var(--ok)' : 'var(--line-2)';
          return (
            <div
              key={key.n}
              style={{
                padding: '18px',
                border: '1px solid var(--line)',
                borderRadius: '12px',
                background: 'var(--paper)',
              }}
            >
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
                <span
                  style={{
                    fontFamily: "'JetBrains Mono', monospace",
                    fontSize: '10.5px',
                    letterSpacing: '.06em',
                    textTransform: 'uppercase',
                    color: 'var(--muted)',
                  }}
                >
                  {key.n}
                </span>
                <span style={{ width: '6px', height: '6px', borderRadius: '50%', background: dot }} />
              </div>
              <div
                style={{
                  fontFamily: "'Fraunces', serif",
                  fontSize: '20px',
                  letterSpacing: '-.01em',
                  marginBottom: '4px',
                }}
              >
                {key.who}
              </div>
              <div style={{ fontSize: '12.5px', color: 'var(--muted)', marginBottom: '10px' }}>{key.hw}</div>
              <div
                style={{
                  fontFamily: "'JetBrains Mono', monospace",
                  fontSize: '10.5px',
                  color: 'var(--muted)',
                  letterSpacing: '.02em',
                }}
              >
                {key.last}
              </div>
            </div>
          );
        })}
      </div>
      <KVList
        pairs={[
          ['Quorum policy', '2-of-3 for seal \u00b7 3-of-3 for re-issue'],
          ['Next rotation', 'Scheduled'],
          ['Ceremony history', (
            <span key="hist">
              Ceremony events \u00b7{' '}
              <LinkArrow text="open ceremonies \u2192" style={{ fontSize: '12px' }} />
            </span>
          )],
        ]}
      />
    </Panel>
  );
}

// ── Storage ─────────────────────────────────────────────────────────────────

function StorageSection() {
  return (
    <Panel header={<PanelHeader title="Storage" meta="S3-compatible" />}>
      <KVList
        pairs={[
          ['Primary', 'MinIO \u00b7 S3-compatible'],
          ['Mirror', 'On-prem NAS'],
          ['Cold archive', 'LTO-9 tape \u00b7 rotated quarterly to secured vault'],
          ['Object encryption', 'AES-256-GCM \u00b7 envelope keys in HSM'],
          ['Hash algorithms', 'SHA-256 primary \u00b7 BLAKE3 secondary'],
        ]}
      />
    </Panel>
  );
}

// ── API Keys ────────────────────────────────────────────────────────────────

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch { /* empty */ }
  };
  return (
    <button type="button" onClick={handleCopy} className="btn sm ghost">
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

  const cols = '1.5fr 1fr 1fr auto';

  return (
    <div>
      {error && <div className="banner-error" style={{ marginBottom: '12px' }}>{error}</div>}
      {success && !createdRawKey && <div className="banner-success" style={{ marginBottom: '12px' }}>{success}</div>}

      {createdRawKey && (
        <div className="panel" style={{ marginBottom: '12px', borderColor: 'var(--accent)' }}>
          <div className="panel-body">
            <p style={{ fontSize: '13px', fontWeight: 600, color: 'var(--ink)', marginBottom: '8px' }}>
              Save this key now — it will not be shown again.
            </p>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <code
                style={{
                  flex: 1,
                  fontFamily: "'JetBrains Mono', monospace",
                  fontSize: '12px',
                  padding: '8px 12px',
                  borderRadius: '6px',
                  background: 'var(--bg-2)',
                  border: '1px solid var(--line)',
                  color: 'var(--ink)',
                  wordBreak: 'break-all',
                }}
              >
                {createdRawKey}
              </code>
              <CopyButton text={createdRawKey} />
            </div>
            <button
              type="button"
              onClick={() => setCreatedRawKey(null)}
              className="linkarrow"
              style={{ fontSize: '12px', marginTop: '6px', background: 'none', border: 'none', cursor: 'pointer' }}
            >
              Dismiss
            </button>
          </div>
        </div>
      )}

      {showCreateForm && (
        <form onSubmit={handleCreate} className="panel" style={{ marginBottom: '12px' }}>
          <div className="panel-body">
            <div style={{ display: 'flex', gap: '10px', alignItems: 'flex-end' }}>
              <div style={{ flex: 1 }}>
                <label style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: '10px', letterSpacing: '.08em', textTransform: 'uppercase', color: 'var(--muted)', display: 'block', marginBottom: '4px' }}>Name</label>
                <input
                  type="text"
                  value={newKeyName}
                  onChange={(e) => setNewKeyName(e.target.value)}
                  placeholder="e.g. CI Pipeline"
                  className="invite-bar"
                  style={{
                    flex: 'none', width: '100%', padding: '10px 14px',
                    border: '1px solid var(--line-2)', borderRadius: '8px',
                    background: 'var(--paper)', font: 'inherit', fontSize: '13.5px',
                  }}
                  maxLength={100}
                  required
                />
              </div>
              <div>
                <label style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: '10px', letterSpacing: '.08em', textTransform: 'uppercase', color: 'var(--muted)', display: 'block', marginBottom: '4px' }}>Permissions</label>
                <select
                  value={newKeyPermissions}
                  onChange={(e) => setNewKeyPermissions(e.target.value as 'read' | 'read_write')}
                  style={{
                    padding: '10px 12px', border: '1px solid var(--line-2)', borderRadius: '8px',
                    background: 'var(--paper)', font: 'inherit', fontSize: '13px', color: 'var(--ink-2)',
                  }}
                >
                  <option value="read">Read Only</option>
                  <option value="read_write">Read & Write</option>
                </select>
              </div>
              <button type="submit" disabled={creating || !newKeyName.trim()} className="btn sm">
                {creating ? 'Creating\u2026' : 'Create'}
              </button>
              <button type="button" onClick={() => { setShowCreateForm(false); setNewKeyName(''); }} className="btn sm ghost">
                Cancel
              </button>
            </div>
          </div>
        </form>
      )}

      <PanelRaw
        header={
          <>
            <PanelHeader title="API keys" meta={`${keys.length} active`} />
            {!showCreateForm && (
              <button
                type="button"
                onClick={() => { setShowCreateForm(true); setCreatedRawKey(null); setError(''); setSuccess(''); }}
                className="btn sm ghost"
              >
                Generate new key
              </button>
            )}
          </>
        }
      >
        {isLoading ? (
          <div className="panel-body">
            {[1, 2].map((i) => (
              <div key={i} className="skeleton" style={{ height: '3rem', borderRadius: '8px', marginBottom: '8px' }} />
            ))}
          </div>
        ) : keys.length === 0 ? (
          <div className="panel-body" style={{ textAlign: 'center', color: 'var(--muted)', fontSize: '13px', padding: '32px 22px' }}>
            No API keys yet.
          </div>
        ) : (
          <>
            <GridHeader columns={['Key', 'Scope', 'Created', 'Actions']} templateColumns={cols} />
            {keys.map((key) => (
              <GridRow
                key={key.id}
                templateColumns={cols}
                cells={[
                  <div key="name">
                    <div style={{ fontWeight: 500 }}>{key.name}</div>
                    <code style={{ fontSize: '11px', marginTop: '2px', fontFamily: "'JetBrains Mono', monospace", color: 'var(--muted)' }}>
                      {key.id.slice(0, 12)}...
                    </code>
                  </div>,
                  <span key="scope" style={{ color: 'var(--ink-2)' }}>
                    {key.permissions === 'read_write' ? 'Read & Write' : 'Read Only'}
                  </span>,
                  <span key="created" style={{ color: 'var(--muted)', fontSize: '12px' }}>
                    {fmtDate(key.created_at)}
                  </span>,
                  <span key="actions" style={{ display: 'flex', gap: '8px' }}>
                    {revokeTarget === key.id ? (
                      <>
                        <button
                          type="button"
                          onClick={() => handleRevoke(key.id)}
                          disabled={revoking}
                          style={{ fontSize: '12px', fontWeight: 600, color: '#b35c5c', background: 'none', border: 'none', cursor: 'pointer' }}
                        >
                          {revoking ? 'Revoking...' : 'Confirm'}
                        </button>
                        <button
                          type="button"
                          onClick={() => setRevokeTarget(null)}
                          style={{ fontSize: '12px', color: 'var(--muted)', background: 'none', border: 'none', cursor: 'pointer' }}
                        >
                          Cancel
                        </button>
                      </>
                    ) : (
                      <>
                        <LinkArrow text="Rotate" style={{ fontSize: '12px' }} />
                        <DangerLink text="Revoke" onClick={() => setRevokeTarget(key.id)} />
                      </>
                    )}
                  </span>,
                ]}
              />
            ))}
          </>
        )}
      </PanelRaw>
    </div>
  );
}

// ── Danger Zone ─────────────────────────────────────────────────────────────

function DangerSection() {
  return (
    <Panel
      header={<PanelHeader title="Danger zone" meta="irreversible \u00b7 sealed" titleStyle={{ color: '#b35c5c' }} />}
      headerStyle={{ background: '#fbeee8' }}
    >
      <KVList
        pairs={[
          ['Rotate instance key', (
            <span key="rotate">
              Requires 3-of-3 quorum. All peers must re-validate within 72 h.{' '}
              <LinkArrow text="rotate \u2192" style={{ color: '#b35c5c' }} />
            </span>
          )],
          ['Decommission instance', (
            <span key="decom">
              Sealed archive handed to supervisory board. No data deleted.{' '}
              <LinkArrow text="start \u2192" style={{ color: '#b35c5c' }} />
            </span>
          )],
          ['Emergency revocation', (
            <span key="revoke">
              Broadcast key revocation to all peers within 30 s.{' '}
              <LinkArrow text="revoke \u2192" style={{ color: '#b35c5c' }} />
            </span>
          )],
        ]}
      />
    </Panel>
  );
}

// ── Main Settings Page ──────────────────────────────────────────────────────

const NAV_SECTIONS = [
  {
    label: 'People',
    items: [
      { id: 'team' as SettingsTab, text: 'Team members' },
      { id: 'roles' as SettingsTab, text: 'Roles & permissions' },
      { id: 'invites' as SettingsTab, text: 'Pending invites' },
    ],
  },
  {
    label: 'Organisation',
    items: [
      { id: 'organization' as SettingsTab, text: 'General' },
      { id: 'orgs' as SettingsTab, text: 'Switch organisation' },
      { id: 'sso' as SettingsTab, text: 'SSO & identity' },
    ],
  },
  {
    label: 'Security',
    items: [
      { id: 'policy' as SettingsTab, text: 'Retention policy' },
      { id: 'keys' as SettingsTab, text: 'Keys & ceremonies' },
      { id: 'storage' as SettingsTab, text: 'Storage' },
      { id: 'api-keys' as SettingsTab, text: 'API keys' },
    ],
  },
  {
    label: 'System',
    labelStyle: { color: '#b35c5c' } as React.CSSProperties,
    items: [
      { id: 'danger' as SettingsTab, text: 'Danger zone', inactiveStyle: { color: '#b35c5c' } as React.CSSProperties },
    ],
  },
];

function SettingsNav({ activeTab }: { activeTab: SettingsTab }) {
  return (
    <aside className="s-nav">
      {NAV_SECTIONS.map((sec) => (
        <div key={sec.label}>
          <div className="s-label" style={sec.labelStyle}>
            {sec.label}
          </div>
          {sec.items.map((item) => {
            const isActive = activeTab === item.id;
            const style = !isActive && 'inactiveStyle' in item ? item.inactiveStyle : undefined;
            return (
              <a
                key={item.id}
                href={`?tab=${item.id}`}
                className={isActive ? 'active' : ''}
                style={style}
              >
                {item.text}
              </a>
            );
          })}
        </div>
      ))}
    </aside>
  );
}

export default function SettingsPage() {
  const searchParams = useSearchParams();
  const activeTab = (searchParams.get('tab') as SettingsTab) || 'team';
  const { activeOrg } = useOrg();

  const orgName = activeOrg?.name ?? 'Organisation';

  return (
    <Shell>
      <div className="d-content">
        {/* Page header */}
        <section className="d-pagehead">
          <div>
            <span className="eyebrow-m">Organisation &middot; {orgName}</span>
            <h1>Instance <em>settings</em></h1>
            <p className="sub">
              Every change is a sealed event. Policy history is immutable and auditable.
            </p>
          </div>
        </section>

        {/* Settings layout: sidebar nav + content */}
        <div style={{ display: 'grid', gridTemplateColumns: '200px 1fr', gap: 36, alignItems: 'start' }}>
          <SettingsNav activeTab={activeTab} />
          <div>
            {activeTab === 'team' && <TeamSection />}
            {activeTab === 'roles' && <RolesSection />}
            {activeTab === 'invites' && <InvitesSection />}
            {activeTab === 'organization' && <OrgGeneralSection />}
            {activeTab === 'orgs' && <OrgSwitchSection />}
            {activeTab === 'sso' && <SSOSection />}
            {activeTab === 'policy' && <PolicySection />}
            {activeTab === 'keys' && <KeysSection />}
            {activeTab === 'storage' && <StorageSection />}
            {activeTab === 'api-keys' && <ApiKeysSection />}
            {activeTab === 'danger' && <DangerSection />}
          </div>
        </div>
      </div>
    </Shell>
  );
}
