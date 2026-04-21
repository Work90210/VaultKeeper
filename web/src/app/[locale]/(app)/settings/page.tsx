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

// ── Stub data ──────────────────────────────────────────────────────────────

interface StubMember {
  readonly av: string;
  readonly col: string;
  readonly name: string;
  readonly email: string;
  readonly role: string;
  readonly cases: readonly string[];
  readonly status: 'online' | 'offline';
  readonly lastSeen: string;
  readonly keys: string;
}

const STUB_MEMBERS: StubMember[] = [
  { av: 'H', col: '#c87e5e', name: 'H\u00e9l\u00e8ne Morel', email: 'h.morel@eurojust.example', role: 'admin', cases: ['ICC-UKR-2024', 'KSC-23-042', 'RSCSL-12', 'IRMCT-99'], status: 'online', lastSeen: 'Now', keys: 'Ed25519 \u00b7 YubiHSM2' },
  { av: 'M', col: '#4a6b3a', name: 'Martyna Kovacs', email: 'm.kovacs@eurojust.example', role: 'analyst', cases: ['ICC-UKR-2024'], status: 'online', lastSeen: '2 min ago', keys: 'Ed25519 \u00b7 YubiHSM2' },
  { av: 'A', col: '#3a4a6b', name: 'Amir Haddad', email: 'a.haddad@eurojust.example', role: 'analyst', cases: ['ICC-UKR-2024'], status: 'online', lastSeen: '14 min ago', keys: 'Ed25519 \u00b7 Nitrokey' },
  { av: 'J', col: '#6b3a4a', name: 'Juliane Wirth', email: 'j.wirth@eurojust.example', role: 'lead', cases: ['ICC-UKR-2024', 'KSC-23-042'], status: 'offline', lastSeen: '3 h ago', keys: 'Ed25519 \u00b7 YubiHSM2' },
  { av: 'D', col: '#4a6b3a', name: 'Dragan Markovic', email: 'd.markovic@ksc.example', role: 'lead', cases: ['KSC-23-042'], status: 'offline', lastSeen: '1 d ago', keys: 'Ed25519 \u00b7 Nitrokey' },
  { av: 'K', col: '#8a6a3a', name: 'Kadiatu Sesay', email: 'k.sesay@rscsl.example', role: 'clerk', cases: ['RSCSL-12'], status: 'offline', lastSeen: '2 d ago', keys: 'Ed25519 \u00b7 Software' },
  { av: 'W', col: '#5b4a6b', name: 'Werner Nyoka', email: 'w.nyoka@eurojust.example', role: 'admin', cases: ['IRMCT-99'], status: 'online', lastSeen: '8 min ago', keys: 'Ed25519 \u00b7 YubiHSM2' },
  { av: 'R', col: '#8a6a3a', name: 'Reto H\u00e4mmerli', email: 'r.hammerli@eurojust.example', role: 'viewer', cases: [], status: 'offline', lastSeen: '11 h ago', keys: 'Ed25519 \u00b7 Nitrokey HSM' },
];

interface StubRole {
  readonly id: string;
  readonly label: string;
  readonly desc: string;
}

const STUB_ROLES: StubRole[] = [
  { id: 'admin', label: 'Admin', desc: 'Full access. Manage team, cases, settings, keys. Can assign roles and switch organisations.' },
  { id: 'lead', label: 'Lead Investigator', desc: 'Create and manage assigned cases. Upload evidence, manage witnesses, approve disclosures. Cannot change org settings.' },
  { id: 'analyst', label: 'Analyst', desc: 'View and annotate evidence on assigned cases. Create corroborations, flag items. Cannot approve disclosures or manage witnesses.' },
  { id: 'clerk', label: 'Clerk', desc: 'Upload and catalogue evidence. Manage metadata and chain-of-custody records. Read-only for witness data.' },
  { id: 'viewer', label: 'Viewer', desc: 'Read-only access to assigned cases. Can view evidence and reports but cannot modify anything. Audit log visible.' },
];

interface StubPerm {
  readonly name: string;
  readonly admin: boolean | 'partial';
  readonly lead: boolean | 'partial';
  readonly analyst: boolean | 'partial';
  readonly clerk: boolean | 'partial';
  readonly viewer: boolean | 'partial';
}

const STUB_PERMS: StubPerm[] = [
  { name: 'View evidence', admin: true, lead: true, analyst: true, clerk: true, viewer: true },
  { name: 'Upload evidence', admin: true, lead: true, analyst: false, clerk: true, viewer: false },
  { name: 'Seal / countersign', admin: true, lead: true, analyst: false, clerk: false, viewer: false },
  { name: 'Manage witnesses', admin: true, lead: true, analyst: false, clerk: false, viewer: false },
  { name: 'Create corroborations', admin: true, lead: true, analyst: true, clerk: false, viewer: false },
  { name: 'Approve disclosures', admin: true, lead: true, analyst: false, clerk: false, viewer: false },
  { name: 'Redaction tools', admin: true, lead: true, analyst: 'partial', clerk: false, viewer: false },
  { name: 'Generate reports', admin: true, lead: true, analyst: true, clerk: true, viewer: false },
  { name: 'View audit log', admin: true, lead: true, analyst: true, clerk: true, viewer: true },
  { name: 'Manage team', admin: true, lead: false, analyst: false, clerk: false, viewer: false },
  { name: 'Organisation settings', admin: true, lead: false, analyst: false, clerk: false, viewer: false },
  { name: 'Key ceremonies', admin: true, lead: false, analyst: false, clerk: false, viewer: false },
  { name: 'Switch organisation', admin: true, lead: false, analyst: false, clerk: false, viewer: false },
  { name: 'Danger zone actions', admin: true, lead: false, analyst: false, clerk: false, viewer: false },
];

const STUB_PENDING_INVITES = [
  { email: 's.petrov@cija.example', role: 'analyst', by: 'H. Morel \u00b7 2 d ago' },
  { email: 'n.okafor@icc.example', role: 'lead', by: 'H. Morel \u00b7 2 d ago' },
];

const STUB_API_KEYS = [
  { name: 'CI Pipeline', prefix: 'vke_ci_****f8a2', scope: 'Evidence upload', created: '12 Mar 2026', by: 'H. Morel' },
  { name: 'Federation Sync', prefix: 'vke_fed_****3b71', scope: 'Full access', created: '4 Jan 2026', by: 'W. Nyoka' },
];

const STUB_KEYS_DATA = [
  { n: 'Key A', who: 'H. Morel', hw: 'YubiHSM2', last: 'signed \u00b7 2 min ago', status: 'active' as const },
  { n: 'Key B', who: 'W. Nyoka', hw: 'YubiHSM2', last: 'signed \u00b7 14 min ago', status: 'active' as const },
  { n: 'Key C', who: 'R. H\u00e4mmerli', hw: 'Nitrokey HSM', last: 'offline \u00b7 11 h ago', status: 'offline' as const },
];

const STUB_ORGS = [
  { c: 'var(--ink)', letter: 'E', name: 'Eurojust \u00b7 Hague', sub: '3 active cases \u00b7 8 members', current: true },
  { c: '#4a6b3a', letter: 'I', name: 'ICC \u00b7 The Hague', sub: '12 active cases \u00b7 34 members', current: false },
  { c: '#3a4a6b', letter: 'C', name: 'CIJA \u00b7 Brussels', sub: '5 active cases \u00b7 12 members', current: false },
  { c: '#6b3a4a', letter: 'K', name: 'KSC \u00b7 The Hague', sub: '2 active cases \u00b7 6 members', current: false },
  { c: '#8a6a3a', letter: 'U', name: 'UNHCR \u00b7 Geneva', sub: '8 active cases \u00b7 22 members', current: false },
  { c: '#5b4a6b', letter: 'R', name: 'RSCSL \u00b7 Freetown', sub: '1 active case \u00b7 3 members', current: false },
];

// ── Reusable layout primitives ─────────────────────────────────────────────

function SettingsPanel({
  header,
  children,
  headerStyle,
  bodyPadding = true,
}: {
  header?: React.ReactNode;
  children: React.ReactNode;
  headerStyle?: React.CSSProperties;
  bodyPadding?: boolean;
}) {
  return (
    <div className="panel">
      {header && (
        <div className="panel-h" style={headerStyle}>
          {header}
        </div>
      )}
      {bodyPadding ? (
        <div className="panel-body">{children}</div>
      ) : (
        children
      )}
    </div>
  );
}

function SettingsPanelHeader({
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

function SettingsLinkArrow({ text, style, href }: { text: string; style?: React.CSSProperties; href?: string }) {
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

function CaseChip({ name }: { name: string }) {
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

function PermCheck({ value }: { value: boolean | 'partial' }) {
  if (value === true) return <span className="perm-check on" />;
  if (value === 'partial') return <span className="perm-check partial" />;
  return <span className="perm-check" />;
}

function RoleCard({ role, count }: { role: StubRole; count: number }) {
  return (
    <div
      style={{
        padding: '16px 18px',
        border: '1px solid var(--line)',
        borderRadius: 10,
        background: 'var(--paper)',
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 6 }}>
        <RoleBadge role={role.id} />
        <span
          style={{
            fontFamily: "'JetBrains Mono', monospace",
            fontSize: 10.5,
            color: 'var(--muted)',
            letterSpacing: '.04em',
          }}
        >
          {count} member{count !== 1 ? 's' : ''}
        </span>
      </div>
      <div style={{ fontSize: 13, color: 'var(--muted)', lineHeight: 1.55 }}>
        {role.desc}
      </div>
    </div>
  );
}

// ── Team Members ───────────────────────────────────────────────────────────

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

  const useLive = !loading && members.length > 0;
  const AVATAR_COLORS = ['#c87e5e', '#4a6b3a', '#3a4a6b', '#6b3a4a', '#8a6a3a', '#5b4a6b'];
  const cols = '36px 1.4fr 1fr 1fr auto';
  const displayCount = useLive ? members.length : STUB_MEMBERS.length;

  return (
    <SettingsPanel
      header={<SettingsPanelHeader title="Team members" meta={`${displayCount} people`} />}
      bodyPadding={false}
    >
      <GridHeader columns={['', 'Member', 'Role', 'Cases', 'Status']} templateColumns={cols} />

      {useLive ? (
        members.map((m, i) => {
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
              <span><RoleBadge role={m.role} /></span>
              <span style={{ fontSize: '12px', color: 'var(--muted)' }}>
                {m.joined_at ? new Date(m.joined_at).toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' }) : '\u2014'}
              </span>
              <StatusDot status="offline" label={m.joined_at ? 'Active' : '\u2014'} />
            </div>
          );
        })
      ) : (
        STUB_MEMBERS.map((m) => (
          <div key={m.email} className="member-row">
            <MemberAvatar letter={m.av} color={m.col} />
            <span className="name">
              {m.name}
              <small>{m.email}</small>
            </span>
            <span><RoleBadge role={m.role} /></span>
            <span className="cases-list">
              {m.cases.length > 0
                ? m.cases.map((c) => <CaseChip key={c} name={c} />)
                : <span style={{ color: 'var(--muted)', fontSize: 12 }}>No cases assigned</span>}
            </span>
            <StatusDot status={m.status} label={m.lastSeen} />
          </div>
        ))
      )}

      <InviteBar
        buttonLabel="Invite"
        email={invEmail}
        setEmail={setInvEmail}
        role={invRole}
        setRole={setInvRole}
        onSubmit={handleInvite}
        submitting={inviting}
      />
    </SettingsPanel>
  );
}

// ── Roles & Permissions ────────────────────────────────────────────────────

function RolesSection() {
  const roleCountMap = STUB_MEMBERS.reduce<Record<string, number>>((acc, m) => {
    acc[m.role] = (acc[m.role] ?? 0) + 1;
    return acc;
  }, {});

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '18px' }}>
      {/* Role cards */}
      <SettingsPanel
        header={<SettingsPanelHeader title="Roles" meta={`${STUB_ROLES.length} defined`} />}
      >
        <div style={{ display: 'grid', gap: 14 }}>
          {STUB_ROLES.map((r) => (
            <RoleCard key={r.id} role={r} count={roleCountMap[r.id] ?? 0} />
          ))}
        </div>
      </SettingsPanel>

      {/* Permission matrix table */}
      <SettingsPanel
        header={<SettingsPanelHeader title="Permission matrix" meta="" />}
        bodyPadding={false}
      >
        <div style={{ overflowX: 'auto', padding: 0 }}>
          <table className="perm-grid">
            <thead>
              <tr>
                <th style={{ minWidth: 180, textAlign: 'left' }}>Capability</th>
                {STUB_ROLES.map((r) => (
                  <th key={r.id}>{r.label.split(' ')[0]}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {STUB_PERMS.map((p) => (
                <tr key={p.name}>
                  <td>{p.name}</td>
                  {STUB_ROLES.map((r) => (
                    <td key={r.id}>
                      <PermCheck value={p[r.id as keyof StubPerm] as boolean | 'partial'} />
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </SettingsPanel>

      {/* Live role editor (API-driven) */}
      <SettingsPanel
        header={<SettingsPanelHeader title="Custom role editor" meta="API-driven" />}
      >
        <p style={{ fontSize: '13px', color: 'var(--muted)', lineHeight: 1.5, marginBottom: '14px' }}>
          Define what each case role can do. System roles can be customized but not deleted.
        </p>
        <RoleEditorComponent />
      </SettingsPanel>
    </div>
  );
}

// ── Pending Invites ────────────────────────────────────────────────────────

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

  const useLive = !loading && invitations.length > 0;
  const cols = '1.5fr 1fr 1fr auto';
  const displayCount = useLive ? invitations.length : STUB_PENDING_INVITES.length;

  return (
    <SettingsPanel
      header={<SettingsPanelHeader title="Pending invites" meta={`${displayCount} pending`} />}
      bodyPadding={false}
    >
      <GridHeader columns={['Email', 'Role', 'Invited by', 'Actions']} templateColumns={cols} />

      {useLive ? (
        invitations.map((inv) => (
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
        ))
      ) : (
        STUB_PENDING_INVITES.map((inv) => (
          <GridRow
            key={inv.email}
            templateColumns={cols}
            cells={[
              <span key="email" style={{ fontWeight: 500 }}>{inv.email}</span>,
              <RoleBadge key="role" role={inv.role} />,
              <span key="by" style={{ color: 'var(--muted)' }}>{inv.by}</span>,
              <span key="actions" style={{ display: 'flex', gap: '8px' }}>
                <SettingsLinkArrow text="Resend" style={{ fontSize: 12 }} />
                <DangerLink text="Revoke" />
              </span>,
            ]}
          />
        ))
      )}

      <InviteBar
        buttonLabel="Send invite"
        email={invEmail}
        setEmail={setInvEmail}
        role={invRole}
        setRole={setInvRole}
        onSubmit={handleInvite}
        submitting={inviting}
      />
    </SettingsPanel>
  );
}

// ── Organisation General ───────────────────────────────────────────────────

function OrgGeneralSection() {
  const { activeOrg } = useOrg();

  const pairs: [string, React.ReactNode][] = activeOrg
    ? [
        ['Display name', (
          <span key="name">
            <strong>{activeOrg.name}</strong>
            <SettingsLinkArrow text="edit \u2192" style={{ marginLeft: 10, fontSize: 12 }} />
          </span>
        )],
        ['Instance ID', <span key="id"><code>{activeOrg.id.slice(0, 16)}</code></span>],
        ['Description', <span key="desc">{activeOrg.description || '\u2014'}</span>],
      ]
    : [
        ['Display name', (
          <span key="name">
            <strong>Eurojust \u00b7 Hague</strong>
            <SettingsLinkArrow text="edit \u2192" style={{ marginLeft: 10, fontSize: 12 }} />
          </span>
        )],
        ['Instance ID', <span key="id"><code>vke-eurojust-hague</code> \u00b7 3 regional replicas</span>],
        ['Legal entity', 'European Union Agency for Criminal Justice Cooperation'],
        ['Contact', 'secretariat@eurojust.example \u00b7 +31 70 412 XXXX'],
        ['Locale', 'EN (primary) \u00b7 NL \u00b7 DE \u00b7 FR \u00b7 UK \u00b7 RU'],
        ['Time zone', 'Europe/Amsterdam \u00b7 audit log always UTC'],
      ];

  return (
    <SettingsPanel
      header={<SettingsPanelHeader title="Organisation" meta={activeOrg ? '' : 'sealed 14 d ago'} />}
    >
      <KVList pairs={pairs} />
    </SettingsPanel>
  );
}

// ── Switch Organisation ────────────────────────────────────────────────────

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

  const useLive = !loading && orgs.length > 0;

  return (
    <>
      <SettingsPanel
        header={<SettingsPanelHeader title="Switch organisation" meta="Admin only" />}
        bodyPadding={false}
      >
        {useLive ? (
          orgs.map((org, i) => {
            const isCurrent = activeOrg?.id === org.id;
            const letter = org.name.charAt(0).toUpperCase();
            const COLORS = ['var(--ink)', '#4a6b3a', '#3a4a6b', '#6b3a4a', '#8a6a3a', '#5b4a6b'];
            const color = COLORS[i % COLORS.length];
            return (
              <div key={org.id} className={`org-switch-row${isCurrent ? ' current' : ''}`}>
                <span className="oa" style={{ background: color }}>{letter}</span>
                <div>
                  <div style={{ fontWeight: 500, fontSize: 14 }}>{org.name}</div>
                  <div style={{ fontSize: 12, color: 'var(--muted)', marginTop: 1 }}>
                    {org.description || '\u2014'}
                  </div>
                </div>
                {isCurrent ? (
                  <span
                    style={{
                      fontFamily: "'JetBrains Mono', monospace",
                      fontSize: 10,
                      letterSpacing: '.06em',
                      textTransform: 'uppercase',
                      color: 'var(--ok)',
                    }}
                  >
                    Current
                  </span>
                ) : (
                  <SettingsLinkArrow text="Switch \u2192" style={{ fontSize: 12 }} />
                )}
              </div>
            );
          })
        ) : (
          STUB_ORGS.map((org) => (
            <div key={org.name} className={`org-switch-row${org.current ? ' current' : ''}`}>
              <span className="oa" style={{ background: org.c }}>{org.letter}</span>
              <div>
                <div style={{ fontWeight: 500, fontSize: 14 }}>{org.name}</div>
                <div style={{ fontSize: 12, color: 'var(--muted)', marginTop: 1 }}>{org.sub}</div>
              </div>
              {org.current && (
                <span
                  style={{
                    fontFamily: "'JetBrains Mono', monospace",
                    fontSize: 10,
                    letterSpacing: '.06em',
                    textTransform: 'uppercase',
                    color: 'var(--ok)',
                  }}
                >
                  Current
                </span>
              )}
            </div>
          ))
        )}
      </SettingsPanel>
      <p style={{ fontSize: 12, color: 'var(--muted)', marginTop: 12, lineHeight: 1.6 }}>
        Organisation switching is restricted to Admin roles. All switches are logged as
        sealed audit events. Switching does not revoke access to other organisations.
      </p>
    </>
  );
}

// ── SSO & Identity ─────────────────────────────────────────────────────────

function SSOSection() {
  const [mfaEnabled, setMfaEnabled] = useState(true);
  const [autoProvision, setAutoProvision] = useState(true);

  return (
    <SettingsPanel header={<SettingsPanelHeader title="SSO & identity" meta="SAML 2.0" />}>
      <KVList
        pairs={[
          ['Provider', 'Keycloak \u00b7 self-hosted \u00b7 eurojust.example/auth'],
          ['Protocol', 'SAML 2.0 \u00b7 OIDC fallback enabled'],
          ['MFA required', (
            <span key="mfa" style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
              <Toggle on={mfaEnabled} onToggle={() => setMfaEnabled((prev) => !prev)} />
              All users must use hardware token or TOTP
            </span>
          )],
          ['Session timeout', '8 hours idle \u00b7 24 hours max'],
          ['Auto-provision', (
            <span key="auto" style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
              <Toggle on={autoProvision} onToggle={() => setAutoProvision((prev) => !prev)} />
              New SSO users get Viewer role by default
            </span>
          )],
          ['Directory sync', 'SCIM 2.0 \u00b7 every 15 min \u00b7 last sync 4 min ago'],
        ]}
      />
    </SettingsPanel>
  );
}

// ── Retention Policy ───────────────────────────────────────────────────────

function PolicySection() {
  return (
    <SettingsPanel header={<SettingsPanelHeader title="Retention policy" meta="quarterly review due in 38 d" />}>
      <KVList
        pairs={[
          ['Default retention', 'Case closed + 50 years, then review'],
          ['Witness records', 'Case closed + 75 years \u00b7 never auto-destroyed'],
          ['Counter-evidence', 'Same as parent case, non-deletable'],
          ['Auto review cadence', 'Every 90 days \u00b7 notify custodian'],
          ['Legal hold override', 'Blocks all deletion, regardless of policy'],
          ['Policy history', (
            <span key="hist">
              4 versions \u00b7 last change by W. Nyoka, 14 d ago \u00b7{' '}
              <SettingsLinkArrow text="diff \u2192" style={{ fontSize: 12 }} />
            </span>
          )],
        ]}
      />
    </SettingsPanel>
  );
}

// ── Keys & Ceremonies ──────────────────────────────────────────────────────

function KeysSection() {
  return (
    <SettingsPanel header={<SettingsPanelHeader title="Keys & ceremonies" meta="2-of-3 quorum" />}>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 16, marginBottom: 18 }}>
        {STUB_KEYS_DATA.map((key) => {
          const dot = key.status === 'active' ? 'var(--ok)' : 'var(--line-2)';
          return (
            <div
              key={key.n}
              style={{
                padding: 18,
                border: '1px solid var(--line)',
                borderRadius: 12,
                background: 'var(--paper)',
              }}
            >
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
                <span
                  style={{
                    fontFamily: "'JetBrains Mono', monospace",
                    fontSize: 10.5,
                    letterSpacing: '.06em',
                    textTransform: 'uppercase',
                    color: 'var(--muted)',
                  }}
                >
                  {key.n}
                </span>
                <span style={{ width: 6, height: 6, borderRadius: '50%', background: dot }} />
              </div>
              <div
                style={{
                  fontFamily: "'Fraunces', serif",
                  fontSize: 20,
                  letterSpacing: '-.01em',
                  marginBottom: 4,
                }}
              >
                {key.who}
              </div>
              <div style={{ fontSize: 12.5, color: 'var(--muted)', marginBottom: 10 }}>{key.hw}</div>
              <div
                style={{
                  fontFamily: "'JetBrains Mono', monospace",
                  fontSize: 10.5,
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
          ['Next rotation', 'in 11 days \u00b7 Mon 30 April, 10:00'],
          ['Ceremony history', (
            <span key="hist">
              24 events \u00b7{' '}
              <SettingsLinkArrow text="open ceremonies \u2192" style={{ fontSize: 12 }} />
            </span>
          )],
        ]}
      />
    </SettingsPanel>
  );
}

// ── Storage ────────────────────────────────────────────────────────────────

function StorageSection() {
  return (
    <SettingsPanel header={<SettingsPanelHeader title="Storage" meta="S3-compatible \u00b7 MinIO" />}>
      <KVList
        pairs={[
          ['Primary', 'MinIO \u00b7 EU-WEST-2 \u00b7 2.4 TB of 8 TB used'],
          ['Mirror', 'On-prem NAS \u00b7 Hague basement \u00b7 2.1 TB of 8 TB'],
          ['Cold archive', 'LTO-9 tape \u00b7 rotated quarterly to secured vault'],
          ['Object encryption', 'AES-256-GCM \u00b7 envelope keys in HSM'],
          ['Hash algorithms', 'SHA-256 primary \u00b7 BLAKE3 secondary'],
        ]}
      />
    </SettingsPanel>
  );
}

// ── API Keys ───────────────────────────────────────────────────────────────

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

  const useLive = !isLoading && keys.length > 0;
  const cols = '1.5fr 1fr 1fr auto';

  return (
    <div>
      {error && <div className="banner-error" style={{ marginBottom: 12 }}>{error}</div>}
      {success && !createdRawKey && <div className="banner-success" style={{ marginBottom: 12 }}>{success}</div>}

      {createdRawKey && (
        <div className="panel" style={{ marginBottom: 12, borderColor: 'var(--accent)' }}>
          <div className="panel-body">
            <p style={{ fontSize: 13, fontWeight: 600, color: 'var(--ink)', marginBottom: 8 }}>
              Save this key now \u2014 it will not be shown again.
            </p>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <code
                style={{
                  flex: 1,
                  fontFamily: "'JetBrains Mono', monospace",
                  fontSize: 12,
                  padding: '8px 12px',
                  borderRadius: 6,
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
              style={{ fontSize: 12, marginTop: 6, background: 'none', border: 'none', cursor: 'pointer' }}
            >
              Dismiss
            </button>
          </div>
        </div>
      )}

      {showCreateForm && (
        <form onSubmit={handleCreate} className="panel" style={{ marginBottom: 12 }}>
          <div className="panel-body">
            <div style={{ display: 'flex', gap: 10, alignItems: 'flex-end' }}>
              <div style={{ flex: 1 }}>
                <label style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: 10, letterSpacing: '.08em', textTransform: 'uppercase', color: 'var(--muted)', display: 'block', marginBottom: 4 }}>Name</label>
                <input
                  type="text"
                  value={newKeyName}
                  onChange={(e) => setNewKeyName(e.target.value)}
                  placeholder="e.g. CI Pipeline"
                  style={{
                    width: '100%', padding: '10px 14px',
                    border: '1px solid var(--line-2)', borderRadius: 8,
                    background: 'var(--paper)', font: 'inherit', fontSize: 13.5,
                    outline: 'none',
                  }}
                  maxLength={100}
                  required
                />
              </div>
              <div>
                <label style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: 10, letterSpacing: '.08em', textTransform: 'uppercase', color: 'var(--muted)', display: 'block', marginBottom: 4 }}>Permissions</label>
                <select
                  value={newKeyPermissions}
                  onChange={(e) => setNewKeyPermissions(e.target.value as 'read' | 'read_write')}
                  style={{
                    padding: '10px 12px', border: '1px solid var(--line-2)', borderRadius: 8,
                    background: 'var(--paper)', font: 'inherit', fontSize: 13, color: 'var(--ink-2)',
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

      <SettingsPanel
        header={
          <>
            <SettingsPanelHeader title="API keys" meta={useLive ? `${keys.length} active` : `${STUB_API_KEYS.length} active`} />
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
        bodyPadding={false}
      >
        <GridHeader columns={['Key', 'Scope', 'Created', 'Actions']} templateColumns={cols} />

        {useLive ? (
          keys.map((key) => (
            <GridRow
              key={key.id}
              templateColumns={cols}
              cells={[
                <div key="name">
                  <div style={{ fontWeight: 500 }}>{key.name}</div>
                  <code style={{ fontSize: 11, marginTop: 2, fontFamily: "'JetBrains Mono', monospace", color: 'var(--muted)' }}>
                    {key.id.slice(0, 12)}...
                  </code>
                </div>,
                <span key="scope" style={{ color: 'var(--ink-2)' }}>
                  {key.permissions === 'read_write' ? 'Read & Write' : 'Read Only'}
                </span>,
                <span key="created" style={{ color: 'var(--muted)', fontSize: 12 }}>
                  {new Date(key.created_at).toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' })}
                </span>,
                <span key="actions" style={{ display: 'flex', gap: 8 }}>
                  {revokeTarget === key.id ? (
                    <>
                      <button
                        type="button"
                        onClick={() => handleRevoke(key.id)}
                        disabled={revoking}
                        style={{ fontSize: 12, fontWeight: 600, color: '#b35c5c', background: 'none', border: 'none', cursor: 'pointer' }}
                      >
                        {revoking ? 'Revoking...' : 'Confirm'}
                      </button>
                      <button
                        type="button"
                        onClick={() => setRevokeTarget(null)}
                        style={{ fontSize: 12, color: 'var(--muted)', background: 'none', border: 'none', cursor: 'pointer' }}
                      >
                        Cancel
                      </button>
                    </>
                  ) : (
                    <>
                      <SettingsLinkArrow text="Rotate" style={{ fontSize: 12 }} />
                      <DangerLink text="Revoke" onClick={() => setRevokeTarget(key.id)} />
                    </>
                  )}
                </span>,
              ]}
            />
          ))
        ) : (
          STUB_API_KEYS.map((k) => (
            <GridRow
              key={k.prefix}
              templateColumns={cols}
              cells={[
                <div key="name">
                  <div style={{ fontWeight: 500 }}>{k.name}</div>
                  <code style={{ fontSize: 11, marginTop: 2, fontFamily: "'JetBrains Mono', monospace", color: 'var(--muted)' }}>
                    {k.prefix}
                  </code>
                </div>,
                <span key="scope" style={{ color: 'var(--ink-2)' }}>{k.scope}</span>,
                <span key="created" style={{ color: 'var(--muted)', fontSize: 12 }}>{k.created} \u00b7 {k.by}</span>,
                <span key="actions" style={{ display: 'flex', gap: 8 }}>
                  <SettingsLinkArrow text="Rotate" style={{ fontSize: 12 }} />
                  <DangerLink text="Revoke" />
                </span>,
              ]}
            />
          ))
        )}

        {!useLive && !isLoading && (
          <div style={{ padding: 18 }}>
            <button className="btn sm ghost" type="button">Generate new key</button>
          </div>
        )}
      </SettingsPanel>
    </div>
  );
}

// ── Danger Zone ────────────────────────────────────────────────────────────

function DangerSection() {
  return (
    <SettingsPanel
      header={<SettingsPanelHeader title="Danger zone" meta="irreversible \u00b7 sealed" titleStyle={{ color: '#b35c5c' }} />}
      headerStyle={{ background: '#fbeee8' }}
    >
      <KVList
        pairs={[
          ['Rotate instance key', (
            <span key="rotate">
              Requires 3-of-3 quorum. All peers must re-validate within 72 h.{' '}
              <SettingsLinkArrow text="rotate \u2192" style={{ color: '#b35c5c' }} />
            </span>
          )],
          ['Decommission instance', (
            <span key="decom">
              Sealed archive handed to supervisory board. No data deleted.{' '}
              <SettingsLinkArrow text="start \u2192" style={{ color: '#b35c5c' }} />
            </span>
          )],
          ['Emergency revocation', (
            <span key="revoke">
              Broadcast key revocation to all peers within 30 s.{' '}
              <SettingsLinkArrow text="revoke \u2192" style={{ color: '#b35c5c' }} />
            </span>
          )],
        ]}
      />
    </SettingsPanel>
  );
}

// ── Main Settings Page ─────────────────────────────────────────────────────

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

function SettingsPage() {
  const searchParams = useSearchParams();
  const activeTab = (searchParams.get('tab') as SettingsTab) || 'team';
  const { activeOrg } = useOrg();

  const orgName = activeOrg?.name ?? 'Eurojust \u00b7 Hague';

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

export default SettingsPage;
