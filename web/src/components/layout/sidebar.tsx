'use client';

import { useState, useEffect } from 'react';
import Link from 'next/link';
import { usePathname, useSearchParams, useRouter } from 'next/navigation';
import { useAuth } from '@/hooks/use-auth';
import { useOrg } from '@/hooks/use-org';
import { OrgSwitcher } from './org-switcher';
import { useCaseContext } from '@/components/providers/case-provider';

/* ─── SVG Icons (16x16 viewBox, matching design prototype) ─── */
const ICONS: Record<string, React.ReactNode> = {
  overview: (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.4}>
      <rect x="2" y="2" width="5" height="5" rx="1" />
      <rect x="9" y="2" width="5" height="5" rx="1" />
      <rect x="2" y="9" width="5" height="5" rx="1" />
      <rect x="9" y="9" width="5" height="5" rx="1" />
    </svg>
  ),
  cases: (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.4}>
      <path d="M2 4.5h12v9H2z" />
      <path d="M6 4.5V3h4v1.5" />
    </svg>
  ),
  evidence: (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.4}>
      <path d="M3 2h7l3 3v9H3z" />
      <path d="M10 2v3h3" />
    </svg>
  ),
  witnesses: (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.4}>
      <circle cx="8" cy="6" r="2.5" />
      <path d="M3 13.5c.8-2.5 2.7-4 5-4s4.2 1.5 5 4" />
    </svg>
  ),
  analysis: (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.4}>
      <path d="M3 2.5h10v11H3z" />
      <path d="M5.5 6h5M5.5 9h5M5.5 11.5h3" />
    </svg>
  ),
  corroborations: (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.4}>
      <circle cx="5" cy="5" r="2.5" />
      <circle cx="11" cy="11" r="2.5" />
      <path d="M7 7l2 2" />
    </svg>
  ),
  inquiry: (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.4}>
      <circle cx="7" cy="7" r="4" />
      <path d="M10 10l3 3" />
    </svg>
  ),
  assess: (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.4}>
      <path d="M3 13V5h2v8M7 13V3h2v10M11 13V7h2v6" />
    </svg>
  ),
  redaction: (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.4}>
      <path d="M2 5h5v2H2zM9 9h5v2H9z" />
      <path d="M2 9h3v2H2zM11 5h3v2h-3z" />
    </svg>
  ),
  disclosures: (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.4}>
      <path d="M3 3h7l3 3v8H3z" />
      <path d="M5.5 9h5M5.5 11.5h5" />
    </svg>
  ),
  reports: (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.4}>
      <path d="M3 13V4l5-1.5 5 1.5v9" />
      <path d="M3 13h10" />
    </svg>
  ),
  search: (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.4}>
      <circle cx="7" cy="7" r="4" />
      <path d="M10 10l3 3" />
    </svg>
  ),
  audit: (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.4}>
      <path d="M3 3l3 3 4-4M3 8l3 3 4-4M3 13h10" />
    </svg>
  ),
  federation: (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.4}>
      <circle cx="4" cy="8" r="2" />
      <circle cx="12" cy="8" r="2" />
      <path d="M6 8h4" />
    </svg>
  ),
  settings: (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.4}>
      <circle cx="8" cy="8" r="2" />
      <path d="M8 2v2M8 12v2M2 8h2M12 8h2M3.5 3.5l1.4 1.4M11.1 11.1l1.4 1.4M3.5 12.5l1.4-1.4M11.1 4.9l1.4-1.4" />
    </svg>
  ),
  members: (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.4}>
      <circle cx="6" cy="5" r="2" />
      <path d="M2 13c.6-2 2-3.5 4-3.5s3.4 1.5 4 3.5" />
      <circle cx="11.5" cy="5.5" r="1.5" />
      <path d="M11 8c1.2 0 2.2.8 2.8 2" />
    </svg>
  ),
};

/* ─── Nav item types ─── */
interface NavItem {
  key: string;
  href: string;
  label: string;
  iconKey: string;
  badge?: string;
  badgeAccent?: boolean;
}

interface NavGroup {
  label: string;
  items: NavItem[];
}

/* ─── App nav groups (cross-case "all cases" view) ─── */
const APP_NAV_GROUPS: NavGroup[] = [
  {
    label: 'Workspace',
    items: [
      { key: 'overview', href: '/en/cases?view=overview', label: 'Overview', iconKey: 'overview' },
      { key: 'cases', href: '/en/cases', label: 'Cases', iconKey: 'cases' },
    ],
  },
  {
    label: 'Investigation',
    items: [
      { key: 'inquiry', href: '/en/cases?view=inquiry', label: '1 \u00b7 Inquiry log', iconKey: 'inquiry' },
      { key: 'assessments', href: '/en/cases?view=assessments', label: '2 \u00b7 Assessments', iconKey: 'assess' },
      { key: 'evidence', href: '/en/cases?view=evidence', label: '3 \u00b7 Evidence', iconKey: 'evidence' },
      { key: 'witnesses', href: '/en/cases?view=witnesses', label: '3 \u00b7 Witnesses', iconKey: 'witnesses' },
      { key: 'audit', href: '/en/cases?view=audit', label: '4 \u00b7 Audit log', iconKey: 'audit' },
      { key: 'corroborations', href: '/en/cases?view=corroborations', label: '5 \u00b7 Corroborations', iconKey: 'corroborations' },
      { key: 'analysis', href: '/en/cases?view=analysis', label: '6 \u00b7 Analysis notes', iconKey: 'analysis' },
    ],
  },
  {
    label: 'Reporting',
    items: [
      { key: 'redaction', href: '/en/cases?view=redaction', label: 'Redaction', iconKey: 'redaction', badgeAccent: true },
      { key: 'disclosures', href: '/en/cases?view=disclosures', label: 'Disclosures', iconKey: 'disclosures' },
      { key: 'reports', href: '/en/cases?view=reports', label: 'Reports', iconKey: 'reports' },
    ],
  },
  {
    label: 'Platform',
    items: [
      { key: 'search', href: '/en/search', label: 'Search', iconKey: 'search' },
      { key: 'federation', href: '/en/cases?view=federation', label: 'Federation', iconKey: 'federation' },
      { key: 'settings', href: '/en/settings', label: 'Settings', iconKey: 'settings' },
    ],
  },
];

/* ─── Case sidebar groups ─── */
const CASE_SIDEBAR_GROUPS: NavGroup[] = [
  {
    label: 'Case',
    items: [
      { key: 'overview', href: '', label: 'Overview', iconKey: 'overview' },
    ],
  },
  {
    label: 'Investigation',
    items: [
      { key: 'inquiry-logs', href: '', label: '1 \u00b7 Inquiry log', iconKey: 'inquiry' },
      { key: 'assessments', href: '', label: '2 \u00b7 Assessments', iconKey: 'assess' },
      { key: 'evidence', href: '', label: '3 \u00b7 Evidence', iconKey: 'evidence' },
      { key: 'witnesses', href: '', label: '3 \u00b7 Witnesses', iconKey: 'witnesses' },
      { key: 'audit', href: '', label: '4 \u00b7 Audit log', iconKey: 'audit' },
      { key: 'corroborations', href: '', label: '5 \u00b7 Corroborations', iconKey: 'corroborations' },
      { key: 'analysis', href: '', label: '6 \u00b7 Analysis notes', iconKey: 'analysis' },
    ],
  },
  {
    label: 'Reporting',
    items: [
      { key: 'redaction', href: '', label: 'Redaction', iconKey: 'redaction', badgeAccent: true },
      { key: 'disclosures', href: '', label: 'Disclosures', iconKey: 'disclosures' },
      { key: 'reports', href: '', label: 'Reports', iconKey: 'reports' },
    ],
  },
  {
    label: 'Platform',
    items: [
      { key: 'search', href: '', label: 'Search', iconKey: 'search' },
      { key: 'settings', href: '', label: 'Settings', iconKey: 'settings' },
    ],
  },
];

/* ─── Settings sidebar groups ─── */
interface SettingsGroup {
  label: string;
  labelStyle?: React.CSSProperties;
  items: {
    key: string;
    label: string;
    adminOnly?: boolean;
    inactiveStyle?: React.CSSProperties;
  }[];
}

const SETTINGS_SIDEBAR_GROUPS: SettingsGroup[] = [
  {
    label: 'People',
    items: [
      { key: 'team', label: 'Team members', adminOnly: true },
      { key: 'roles', label: 'Roles & permissions', adminOnly: true },
      { key: 'invites', label: 'Pending invites', adminOnly: true },
    ],
  },
  {
    label: 'Organisation',
    items: [
      { key: 'organization', label: 'General', adminOnly: true },
      { key: 'orgs', label: 'Switch organisation', adminOnly: true },
      { key: 'sso', label: 'SSO & identity', adminOnly: true },
    ],
  },
  {
    label: 'Security',
    items: [
      { key: 'policy', label: 'Retention policy', adminOnly: true },
      { key: 'keys', label: 'Keys & ceremonies', adminOnly: true },
      { key: 'storage', label: 'Storage', adminOnly: true },
      { key: 'api-keys', label: 'API keys' },
    ],
  },
  {
    label: 'System',
    labelStyle: { color: '#b35c5c' },
    items: [
      {
        key: 'danger',
        label: 'Danger zone',
        adminOnly: true,
        inactiveStyle: { color: '#b35c5c' },
      },
    ],
  },
];

/* ─── Nav Link (design: grid 18px/1fr/auto with .ico span + .badge span) ─── */
function NavLink({
  iconKey,
  label,
  active,
  badge,
  badgeAccent,
  onClick,
  href,
}: {
  iconKey: string;
  label: string;
  active: boolean;
  badge?: string;
  badgeAccent?: boolean;
  onClick?: () => void;
  href?: string;
}) {
  const icon = ICONS[iconKey] || ICONS.overview;
  const cls = active ? 'active' : '';

  const inner = (
    <>
      <span className="ico">{icon}</span>
      <span>{label}</span>
      {badge && (
        <span className={`badge${badgeAccent ? ' a' : ''}`}>{badge}</span>
      )}
    </>
  );

  if (onClick) {
    return (
      <button type="button" onClick={onClick} className={`nav-link ${cls}`}>
        {inner}
      </button>
    );
  }

  return (
    <Link href={href || '#'} className={cls}>
      {inner}
    </Link>
  );
}

/* ─── Case Picker Dropdown ─── */
function CasePicker({ cases }: { cases: { id: string; reference_code: string; title: string; status: string }[] }) {
  const [open, setOpen] = useState(false);
  const router = useRouter();
  const searchParams = useSearchParams?.() ?? null;
  const selectedCaseId = searchParams?.get('caseId') ?? null;
  const selectedCase = cases.find((c) => c.id === selectedCaseId);

  if (cases.length === 0) return null;

  const statusDot: Record<string, string> = {
    active: 'var(--ok)',
    hold: 'var(--accent)',
    closed: 'var(--muted)',
    archived: 'var(--muted-2)',
  };

  const currentLabel = selectedCase ? selectedCase.reference_code : 'All cases';
  const currentSub = selectedCase ? selectedCase.title : undefined;
  const currentDot = selectedCase ? (statusDot[selectedCase.status] || 'var(--muted)') : 'var(--ok)';

  function selectCase(caseId: string | null) {
    const params = new URLSearchParams(searchParams?.toString() || '');
    if (caseId) {
      params.set('caseId', caseId);
    } else {
      params.delete('caseId');
    }
    // Keep current view if set
    const _view = params.get('view');
    const base = '/en/cases';
    const qs = params.toString();
    router.push(qs ? `${base}?${qs}` : base);
    setOpen(false);
  }

  return (
    <div style={{ position: 'relative' }}>
      <button
        type="button"
        onClick={() => setOpen(!open)}
        style={{
          display: 'grid',
          gridTemplateColumns: '6px 1fr auto',
          gap: '10px',
          alignItems: 'center',
          width: '100%',
          padding: '10px 12px',
          borderRadius: '10px',
          border: '1px solid var(--line)',
          background: 'var(--paper)',
          cursor: 'pointer',
          fontSize: '13px',
          color: 'var(--ink)',
          fontFamily: '"Fraunces", serif',
          transition: 'border-color 0.15s',
          textAlign: 'left' as const,
        }}
      >
        <span className="cd" style={{ width: '6px', height: '6px', borderRadius: '50%', background: currentDot }} />
        <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const }}>
          {currentLabel}
          {currentSub && <small style={{ display: 'block', fontSize: '10px', color: 'var(--muted)', fontWeight: 400, marginTop: 1 }}>{currentSub}</small>}
        </span>
        <span className="chev" style={{ color: 'var(--muted)', fontSize: '11px' }}>{'\u25BE'}</span>
      </button>
      {open && (
        <div className={`ctx-dd${open ? ' open' : ''}`} style={{ position: 'absolute', top: '100%', left: 0, right: 0, marginTop: 4, width: 'auto' }}>
          <div className="dd-label">Cases</div>
          <button
            type="button"
            onClick={() => selectCase(null)}
            className={`dd-case${!selectedCaseId ? ' active' : ''}`}
            style={{ width: '100%', border: 'none', fontFamily: 'inherit', textAlign: 'left' as const }}
          >
            <span className="cd" style={{ width: '6px', height: '6px', borderRadius: '50%', background: 'var(--ok)' }} />
            <span>All cases</span>
          </button>
          <div className="dd-sep" />
          {cases.map((c) => (
            <button
              key={c.id}
              type="button"
              onClick={() => selectCase(c.id)}
              className={`dd-case${selectedCaseId === c.id ? ' active' : ''}`}
              style={{ width: '100%', border: 'none', fontFamily: 'inherit', textAlign: 'left' as const }}
            >
              <span className="cd" style={{ width: '6px', height: '6px', borderRadius: '50%', background: statusDot[c.status] || 'var(--muted)' }} />
              <span>
                {c.reference_code}
                <small style={{ display: 'block', fontSize: '10px', color: 'var(--muted)', fontWeight: 400, marginTop: 1 }}>{c.title}</small>
              </span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

/* ─── Main Sidebar ─── */
export function Sidebar() {
  const pathname = usePathname();
  const { user } = useAuth();
  const { caseData, activeTab, setActiveTab, sidebarCounts } = useCaseContext();
  const [sidebarData, setSidebarData] = useState<{ counts: Record<string, string>; cases: { id: string; reference_code: string; title: string; status: string }[] }>({ counts: {}, cases: [] });

  useEffect(() => {
    async function load() {
      try {
        const res = await fetch('/api/sidebar-counts');
        if (!res.ok) return;
        const data = await res.json();
        const c: Record<string, string> = {};
        if (data.counts) {
          for (const [key, val] of Object.entries(data.counts)) {
            const n = Number(val);
            if (n > 0) {
              c[key] = n >= 1000 ? `${(n / 1000).toFixed(1)}k` : String(n);
            }
          }
        }
        setSidebarData({ counts: c, cases: Array.isArray(data.cases) ? data.cases : [] });
      } catch {}
    }
    load();
  }, []);

  const isCaseView = caseData !== null;

  const userName = user?.name || 'User';
  const userInitial = userName.charAt(0).toUpperCase();

  return (
    <aside className="d-side">
      {/* Brand */}
      <Link href="/en/cases" className="brand">
        <span className="brand-mark" />
        Vault<em>Keeper</em>
      </Link>

      {/* Org picker */}
      <OrgSwitcher />

      {/* Case picker — only in app mode */}
      {!isCaseView && <CasePicker cases={sidebarData.cases} />}

      {/* Navigation */}
      {isCaseView ? (
        <CaseSidebarContent
          caseData={caseData}
          activeTab={activeTab}
          setActiveTab={setActiveTab}
          sidebarCounts={sidebarCounts}
        />
      ) : (
        <AppSidebarContent pathname={pathname} counts={sidebarData.counts} cases={sidebarData.cases} />
      )}

      {/* Who: .who > .av + .n (with small) + .dot */}
      <Link href="/en/profile" className="who">
        <span className="av">{userInitial}</span>
        <span className="n">
          {userName}
          <small>Signed in</small>
        </span>
        <span className="dot" title="Signed in" />
      </Link>
    </aside>
  );
}

/* ─── App mode navigation (grouped with .nav-label) ─── */
function AppSidebarContent({ pathname, counts, cases }: { pathname: string; counts: Record<string, string>; cases: { id: string; reference_code: string; title: string; status: string }[] }) {
  const searchParams = useSearchParams?.() ?? null;
  const view = searchParams?.get('view') ?? null;
  const selectedCaseId = searchParams?.get('caseId') ?? null;
  const selectedCase = cases.find((c) => c.id === selectedCaseId);

  // Fetch case-specific counts via the server-side sidebar-counts route
  const [caseCounts, setCaseCounts] = useState<Record<string, string>>({});
  useEffect(() => {
    if (!selectedCaseId) { setCaseCounts({}); return; }
    let cancelled = false;
    (async () => {
      try {
        const res = await fetch(`/api/sidebar-counts?caseId=${encodeURIComponent(selectedCaseId)}`);
        if (!res.ok || cancelled) return;
        const json = await res.json();
        if (cancelled || !json?.counts) return;
        const c: Record<string, string> = {};
        for (const [key, val] of Object.entries(json.counts as Record<string, number>)) {
          const n = Number(val);
          if (n > 0) c[key] = n >= 1000 ? `${(n / 1000).toFixed(1)}k` : String(n);
        }
        setCaseCounts(c);
      } catch {
        // Silently fail — badges just won't update
      }
    })();
    return () => { cancelled = true; };
  }, [selectedCaseId]);

  const PATH_TO_NAV: Record<string, string> = {
    search: 'search', settings: 'settings', evidence: 'evidence',
    witnesses: 'witnesses', disclosures: 'disclosures', corroborations: 'corroborations',
    'analysis-notes': 'analysis', 'inquiry-logs': 'inquiry', assessments: 'assessments',
    verifications: 'corroborations', reports: 'reports',
  };
  const segment = pathname.split('/')[2] || '';
  const activeKey = PATH_TO_NAV[segment]
    || (segment === 'cases' ? (view || 'cases') : 'overview');

  // Use case-specific counts when a case is selected, otherwise global counts
  const displayCounts = selectedCaseId ? caseCounts : counts;

  // When a case is selected, show case-scoped nav (no Cases/Overview top-level)
  const navGroups = selectedCaseId ? APP_NAV_GROUPS : APP_NAV_GROUPS;
  const hiddenKeys = selectedCaseId ? new Set(['cases']) : new Set<string>();

  return (
    <nav className="d-nav" style={{ flex: 1, overflowY: 'auto' }}>
      {selectedCase && (
        <div className="nav-label" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <span>{selectedCase.reference_code}</span>
          <span style={{
            fontSize: '9px', letterSpacing: '.06em', padding: '2px 6px', borderRadius: '4px',
            background: selectedCase.status === 'hold' ? 'rgba(184,66,28,.1)' : 'rgba(74,107,58,.1)',
            color: selectedCase.status === 'hold' ? 'var(--accent)' : 'var(--ok)',
          }}>
            {selectedCase.status}
          </span>
        </div>
      )}
      {navGroups.map((group) => {
        const visibleItems = group.items.filter((item) => !hiddenKeys.has(item.key));
        if (visibleItems.length === 0) return null;
        return (
        <div key={group.label}>
          <div className="nav-label">{group.label}</div>
          {visibleItems.map((item) => (
            <NavLink
              key={item.key}
              iconKey={item.iconKey}
              label={item.label}
              active={activeKey === item.key}
              badge={selectedCaseId ? displayCounts[item.key] : (counts[item.key] || item.badge)}
              badgeAccent={item.badgeAccent}
              href={item.href}
            />
          ))}
        </div>
        );
      })}
    </nav>
  );
}

/* ─── Case mode navigation ─── */
function CaseSidebarContent({
  caseData,
  activeTab,
  setActiveTab,
  sidebarCounts,
}: {
  caseData: { id: string; reference_code: string; title: string; canEdit: boolean };
  activeTab: string;
  setActiveTab: (tab: string) => void;
  sidebarCounts: Partial<Record<string, number>>;
}) {
  return (
    <>
      {/* Back link + case info */}
      <div style={{ paddingBottom: '12px', borderBottom: '1px solid var(--line)' }}>
        <Link
          href="/en/cases"
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: '4px',
            fontSize: '11px',
            fontFamily: '"JetBrains Mono", monospace',
            letterSpacing: '0.08em',
            textTransform: 'uppercase',
            color: 'var(--muted)',
            textDecoration: 'none',
            marginBottom: '8px',
          }}
        >
          <svg style={{ width: '10px', height: '10px' }} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M15 19l-7-7 7-7" />
          </svg>
          Cases
        </Link>
        <div
          style={{
            fontFamily: '"JetBrains Mono", monospace',
            fontSize: '11px',
            letterSpacing: '0.04em',
            color: 'var(--muted)',
            marginBottom: '2px',
          }}
        >
          {caseData.reference_code}
        </div>
        <div
          style={{
            fontFamily: '"Fraunces", serif',
            fontSize: '15px',
            fontWeight: 500,
            lineHeight: 1.3,
            color: 'var(--ink)',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}
          title={caseData.title}
        >
          {caseData.title}
        </div>
      </div>

      {/* Case nav groups */}
      <nav className="d-nav" style={{ flex: 1, overflowY: 'auto' }}>
        {CASE_SIDEBAR_GROUPS.map((group) => {
          const visibleItems = group.items.filter(
            (item) => item.key !== 'settings' || caseData.canEdit,
          );
          if (visibleItems.length === 0) return null;
          return (
            <div key={group.label}>
              <div className="nav-label">{group.label}</div>
              {visibleItems.map((item) => {
                const count = sidebarCounts[item.key];
                return (
                  <NavLink
                    key={item.key}
                    iconKey={item.iconKey}
                    label={item.label}
                    active={activeTab === item.key}
                    badge={count != null && count > 0 ? String(count) : undefined}
                    onClick={() => setActiveTab(item.key)}
                  />
                );
              })}
            </div>
          );
        })}
      </nav>
    </>
  );
}

/* ─── Settings mode navigation ─── */
// eslint-disable-next-line @typescript-eslint/no-unused-vars
function SettingsSidebarContent() {
  const searchParams = useSearchParams?.() ?? null;
  const activeTab = searchParams?.get('tab') || 'team';
  const { isOrgAdmin } = useOrg();

  return (
    <>
      {/* Back to app */}
      <div style={{ paddingBottom: '12px', borderBottom: '1px solid var(--line)' }}>
        <Link
          href="/en/cases"
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: '4px',
            fontSize: '11px',
            fontFamily: '"JetBrains Mono", monospace',
            letterSpacing: '0.08em',
            textTransform: 'uppercase',
            color: 'var(--muted)',
            textDecoration: 'none',
            marginBottom: '6px',
          }}
        >
          <svg style={{ width: '10px', height: '10px' }} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M15 19l-7-7 7-7" />
          </svg>
          Back
        </Link>
        <div
          style={{
            fontFamily: '"Fraunces", serif',
            fontSize: '18px',
            fontWeight: 400,
            color: 'var(--ink)',
          }}
        >
          Settings
        </div>
      </div>

      {/* Settings nav groups — uses s-nav class matching design prototype */}
      <nav className="s-nav" style={{ flex: 1, overflowY: 'auto' }}>
        {SETTINGS_SIDEBAR_GROUPS.map((group) => {
          const visibleItems = group.items.filter(
            (item) => !item.adminOnly || isOrgAdmin,
          );
          if (visibleItems.length === 0) return null;
          return (
            <div key={group.label}>
              <div className="s-label" style={group.labelStyle}>{group.label}</div>
              {visibleItems.map((item) => {
                const isActive = activeTab === item.key;
                return (
                  <Link
                    key={item.key}
                    href={`/en/settings?tab=${item.key}`}
                    className={isActive ? 'active' : ''}
                    style={!isActive ? item.inactiveStyle : undefined}
                  >
                    {item.label}
                  </Link>
                );
              })}
            </div>
          );
        })}
      </nav>
    </>
  );
}
