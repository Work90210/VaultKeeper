'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useAuth } from '@/hooks/use-auth';
import { OrgSwitcher } from './org-switcher';
import { useCaseContext } from '@/components/providers/case-provider';

/* ─── Icon helper ─── */
function SidebarIcon({ d, size = 16 }: { d: string; size?: number }) {
  return (
    <svg
      className="shrink-0"
      style={{ width: `${size}px`, height: `${size}px` }}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      strokeWidth={1.5}
    >
      <path strokeLinecap="round" strokeLinejoin="round" d={d} />
    </svg>
  );
}

/* ─── Icon paths ─── */
const ICONS = {
  // App nav
  cases:
    'M19 20H5a2 2 0 01-2-2V6a2 2 0 012-2h4l2 3h8a2 2 0 012 2v9a2 2 0 01-2 2z',
  search: 'M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z',
  organizations:
    'M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4',
  notifications:
    'M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9',
  settings:
    'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z',
  profile:
    'M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z',
  // Case nav
  overview:
    'M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2',
  evidence:
    'M15.172 7l-6.586 6.586a2 2 0 102.828 2.828l6.414-6.586a4 4 0 00-5.656-5.656l-6.415 6.585a6 6 0 108.486 8.486L20.5 13',
  witnesses:
    'M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z',
  disclosures:
    'M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4',
  members:
    'M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z',
  'inquiry-logs': 'M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z',
  assessments:
    'M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z',
  verifications:
    'M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z',
  corroborations:
    'M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1',
  analysis:
    'M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01',
  safety:
    'M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z',
  templates:
    'M4 5a1 1 0 011-1h14a1 1 0 011 1v2a1 1 0 01-1 1H5a1 1 0 01-1-1V5zM4 13a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H5a1 1 0 01-1-1v-6zM16 13a1 1 0 011-1h2a1 1 0 011 1v6a1 1 0 01-1 1h-2a1 1 0 01-1-1v-6z',
  reports:
    'M9 17v-2m3 2v-4m3 4v-6m2 10H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z',
} as const;

/* ─── Shared row height for compact spacing ─── */
const ROW_STYLE = {
  height: '30px',
  padding: '0 var(--space-sm)',
  fontSize: 'var(--text-sm)',
  borderRadius: 'var(--radius-sm)',
  transition: `all var(--duration-fast) ease`,
} as const;

/* ─── App nav items (main items, excludes bottom utilities) ─── */
const APP_NAV_ITEMS = [
  { href: '/en/cases', label: 'Cases', iconKey: 'cases' },
  { href: '/en/search', label: 'Search', iconKey: 'search' },
  { href: '/en/organizations', label: 'Organizations', iconKey: 'organizations' },
  { href: '/en/notifications', label: 'Notifications', iconKey: 'notifications' },
] as const;

/* ─── Case sidebar groups ─── */
interface CaseSidebarGroup {
  label: string;
  items: { key: string; label: string; iconKey: string; adminOnly?: boolean }[];
}

const CASE_SIDEBAR_GROUPS: CaseSidebarGroup[] = [
  {
    label: 'Case',
    items: [
      { key: 'overview', label: 'Overview', iconKey: 'overview' },
      { key: 'evidence', label: 'Evidence', iconKey: 'evidence' },
      { key: 'witnesses', label: 'Witnesses', iconKey: 'witnesses' },
      { key: 'disclosures', label: 'Disclosures', iconKey: 'disclosures' },
      { key: 'members', label: 'Members', iconKey: 'members' },
    ],
  },
  {
    label: 'Investigation',
    items: [
      { key: 'inquiry-logs', label: 'Inquiry Logs', iconKey: 'inquiry-logs' },
      { key: 'assessments', label: 'Assessments', iconKey: 'assessments' },
      { key: 'verifications', label: 'Verifications', iconKey: 'verifications' },
      { key: 'corroborations', label: 'Corroborations', iconKey: 'corroborations' },
      { key: 'analysis', label: 'Analysis', iconKey: 'analysis' },
      { key: 'safety', label: 'Safety', iconKey: 'safety' },
    ],
  },
  {
    label: 'Output',
    items: [
      { key: 'templates', label: 'Templates', iconKey: 'templates' },
      { key: 'reports', label: 'Reports', iconKey: 'reports' },
    ],
  },
  {
    label: 'Admin',
    items: [
      { key: 'settings', label: 'Settings', iconKey: 'settings', adminOnly: true },
    ],
  },
];

/* ─── Main Sidebar ─── */
export function Sidebar() {
  const pathname = usePathname();
  const { signOut } = useAuth();
  const { caseData, activeTab, setActiveTab, sidebarCounts } = useCaseContext();

  const isCaseView = caseData !== null;

  return (
    <aside
      className="flex h-full w-[220px] flex-col shrink-0"
      style={{
        backgroundColor: 'var(--bg-secondary)',
        borderRight: '1px solid var(--border-subtle)',
      }}
    >
      {isCaseView ? (
        <CaseSidebarContent
          caseData={caseData}
          activeTab={activeTab}
          setActiveTab={setActiveTab}
          sidebarCounts={sidebarCounts}
        />
      ) : (
        <AppSidebarContent pathname={pathname} />
      )}

      {/* Bottom utility section */}
      <div
        style={{
          borderTop: '1px solid var(--border-subtle)',
          padding: 'var(--space-xs)',
        }}
      >
        {!isCaseView && (
          <Link
            href="/en/settings"
            className="flex items-center gap-[var(--space-sm)]"
            style={{
              ...ROW_STYLE,
              fontWeight: pathname.startsWith('/en/settings') ? 500 : 400,
              color: pathname.startsWith('/en/settings')
                ? 'var(--text-primary)'
                : 'var(--text-secondary)',
              backgroundColor: pathname.startsWith('/en/settings')
                ? 'var(--bg-inset)'
                : undefined,
              borderLeft: pathname.startsWith('/en/settings')
                ? '2px solid var(--amber-accent)'
                : '2px solid transparent',
              display: 'flex',
              alignItems: 'center',
              textDecoration: 'none',
            }}
          >
            <SidebarIcon d={ICONS.settings} />
            Settings
          </Link>
        )}
        <Link
          href="/en/profile"
          className="flex items-center gap-[var(--space-sm)]"
          style={{
            ...ROW_STYLE,
            fontWeight: pathname.startsWith('/en/profile') ? 500 : 400,
            color: pathname.startsWith('/en/profile')
              ? 'var(--text-primary)'
              : 'var(--text-secondary)',
            backgroundColor: pathname.startsWith('/en/profile')
              ? 'var(--bg-inset)'
              : undefined,
            borderLeft: pathname.startsWith('/en/profile')
              ? '2px solid var(--amber-accent)'
              : '2px solid transparent',
            display: 'flex',
            alignItems: 'center',
            textDecoration: 'none',
          }}
        >
          <SidebarIcon d={ICONS.profile} />
          Profile
        </Link>
        <button
          type="button"
          onClick={signOut}
          className="flex w-full items-center gap-[var(--space-sm)]"
          style={{
            ...ROW_STYLE,
            color: 'var(--text-tertiary)',
            background: 'none',
            border: 'none',
            cursor: 'pointer',
            textAlign: 'left',
          }}
        >
          <SidebarIcon d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
          Sign out
        </button>
      </div>
    </aside>
  );
}

/* ─── App mode navigation ─── */
function AppSidebarContent({ pathname }: { pathname: string }) {
  return (
    <>
      {/* Org Switcher row */}
      <div
        style={{
          padding: 'var(--space-xs)',
          borderBottom: '1px solid var(--border-subtle)',
        }}
      >
        <OrgSwitcher />
      </div>

      {/* Primary navigation */}
      <nav className="flex-1 overflow-y-auto" style={{ padding: 'var(--space-xs)' }}>
        <div className="flex flex-col gap-[1px]">
          {APP_NAV_ITEMS.map((item) => {
            const active = pathname.startsWith(item.href);
            return (
              <Link
                key={item.href}
                href={item.href}
                className="flex items-center gap-[var(--space-sm)]"
                style={{
                  ...ROW_STYLE,
                  fontWeight: active ? 500 : 400,
                  color: active ? 'var(--text-primary)' : 'var(--text-secondary)',
                  backgroundColor: active ? 'var(--bg-inset)' : undefined,
                  borderLeft: active
                    ? '2px solid var(--amber-accent)'
                    : '2px solid transparent',
                  display: 'flex',
                  alignItems: 'center',
                  textDecoration: 'none',
                }}
              >
                <SidebarIcon d={ICONS[item.iconKey as keyof typeof ICONS]} />
                {item.label}
              </Link>
            );
          })}
        </div>
      </nav>
    </>
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
      <div
        style={{
          padding: 'var(--space-sm)',
          borderBottom: '1px solid var(--border-subtle)',
        }}
      >
        <Link
          href="/en/cases"
          className="flex items-center gap-[var(--space-xs)]"
          style={{
            fontSize: '0.6875rem',
            color: 'var(--text-tertiary)',
            fontWeight: 600,
            textTransform: 'uppercase',
            letterSpacing: '0.06em',
            transition: `color var(--duration-fast) ease`,
            textDecoration: 'none',
            marginBottom: 'var(--space-sm)',
          }}
        >
          <svg
            style={{ width: '0.625rem', height: '0.625rem' }}
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2.5}
          >
            <path strokeLinecap="round" strokeLinejoin="round" d="M15 19l-7-7 7-7" />
          </svg>
          Cases
        </Link>
        <div
          className="font-[family-name:var(--font-mono)]"
          style={{
            fontSize: '0.6875rem',
            letterSpacing: '0.04em',
            color: 'var(--text-tertiary)',
            marginBottom: '2px',
          }}
        >
          {caseData.reference_code}
        </div>
        <div
          className="font-[family-name:var(--font-heading)] truncate"
          style={{
            fontSize: 'var(--text-sm)',
            fontWeight: 500,
            lineHeight: 1.3,
            color: 'var(--text-primary)',
          }}
          title={caseData.title}
        >
          {caseData.title}
        </div>
      </div>

      {/* Case nav groups */}
      <nav className="flex-1 overflow-y-auto" style={{ padding: 'var(--space-xs)' }}>
        {CASE_SIDEBAR_GROUPS.map((group, gi) => {
          const visibleItems = group.items.filter(
            (item) => !item.adminOnly || caseData.canEdit,
          );
          if (visibleItems.length === 0) return null;
          return (
            <div key={group.label} style={{ marginTop: gi > 0 ? 'var(--space-md)' : 0 }}>
              {/* Section header */}
              <h3
                style={{
                  fontSize: '0.625rem',
                  fontWeight: 600,
                  textTransform: 'uppercase',
                  letterSpacing: '0.08em',
                  color: 'var(--text-tertiary)',
                  padding: '0 var(--space-sm)',
                  marginBottom: '2px',
                }}
              >
                {group.label}
              </h3>
              <div className="flex flex-col gap-[1px]">
                {visibleItems.map((item) => {
                  const isActive = activeTab === item.key;
                  const count = sidebarCounts[item.key];
                  const iconPath = ICONS[item.iconKey as keyof typeof ICONS];
                  return (
                    <button
                      key={item.key}
                      type="button"
                      onClick={() => setActiveTab(item.key)}
                      aria-current={isActive ? 'page' : undefined}
                      className="w-full text-left flex items-center gap-[var(--space-sm)]"
                      style={{
                        ...ROW_STYLE,
                        fontWeight: isActive ? 500 : 400,
                        color: isActive ? 'var(--text-primary)' : 'var(--text-secondary)',
                        backgroundColor: isActive ? 'var(--bg-inset)' : 'transparent',
                        borderLeft: isActive
                          ? '2px solid var(--amber-accent)'
                          : '2px solid transparent',
                        borderTop: 'none',
                        borderRight: 'none',
                        borderBottom: 'none',
                        cursor: 'pointer',
                        display: 'flex',
                        alignItems: 'center',
                      }}
                    >
                      <SidebarIcon d={iconPath} />
                      <span className="flex-1 truncate">{item.label}</span>
                      {count != null && count > 0 && (
                        <span
                          className="font-[family-name:var(--font-mono)] tabular-nums shrink-0"
                          style={{
                            fontSize: '0.6875rem',
                            color: isActive
                              ? 'var(--amber-accent)'
                              : 'var(--text-tertiary)',
                          }}
                        >
                          {count}
                        </span>
                      )}
                    </button>
                  );
                })}
              </div>
            </div>
          );
        })}
      </nav>
    </>
  );
}
