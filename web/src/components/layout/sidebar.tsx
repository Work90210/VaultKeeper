'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { OrgSwitcher } from './org-switcher';
import { useCaseContext } from '@/components/providers/case-provider';

const NAV_ITEMS = [
  { href: '/en/cases', label: 'Cases', icon: 'M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z' },
  { href: '/en/search', label: 'Search', icon: 'M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z' },
  { href: '/en/organizations', label: 'Organizations', icon: 'M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4' },
  { href: '/en/notifications', label: 'Notifications', icon: 'M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9' },
  { href: '/en/settings', label: 'Settings', icon: 'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z' },
];

interface CaseSidebarGroup {
  label: string;
  items: { key: string; label: string; adminOnly?: boolean }[];
}

const CASE_SIDEBAR_GROUPS: CaseSidebarGroup[] = [
  {
    label: 'Case',
    items: [
      { key: 'overview', label: 'Overview' },
      { key: 'evidence', label: 'Evidence' },
      { key: 'witnesses', label: 'Witnesses' },
      { key: 'disclosures', label: 'Disclosures' },
      { key: 'members', label: 'Members' },
    ],
  },
  {
    label: 'Investigation',
    items: [
      { key: 'inquiry-logs', label: 'Inquiry Logs' },
      { key: 'assessments', label: 'Assessments' },
      { key: 'verifications', label: 'Verifications' },
      { key: 'corroborations', label: 'Corroborations' },
      { key: 'analysis', label: 'Analysis' },
      { key: 'safety', label: 'Safety' },
    ],
  },
  {
    label: 'Output',
    items: [
      { key: 'templates', label: 'Templates' },
      { key: 'reports', label: 'Reports' },
    ],
  },
  {
    label: 'Admin',
    items: [
      { key: 'settings', label: 'Settings', adminOnly: true },
    ],
  },
];

export function Sidebar() {
  const pathname = usePathname();
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

      {/* Profile link */}
      <div style={{ padding: 'var(--space-sm)', borderTop: '1px solid var(--border-subtle)' }}>
        <Link
          href="/en/profile"
          className="flex items-center gap-[var(--space-sm)]"
          style={{
            padding: 'var(--space-xs) var(--space-sm)',
            borderRadius: 'var(--radius-sm)',
            fontSize: 'var(--text-sm)',
            color: 'var(--text-secondary)',
            transition: `color var(--duration-fast) ease`,
          }}
        >
          <div
            className="flex items-center justify-center shrink-0"
            style={{
              width: '1.5rem',
              height: '1.5rem',
              borderRadius: 'var(--radius-full)',
              backgroundColor: 'var(--bg-inset)',
              fontSize: 'var(--text-xs)',
              fontWeight: 600,
              color: 'var(--text-tertiary)',
            }}
          >
            P
          </div>
          Profile
        </Link>
      </div>
    </aside>
  );
}

/** Default app navigation (Cases, Search, Organizations, etc.) */
function AppSidebarContent({ pathname }: { pathname: string }) {
  return (
    <>
      {/* Org Switcher */}
      <div
        style={{
          padding: 'var(--space-sm)',
          borderBottom: '1px solid var(--border-subtle)',
        }}
      >
        <OrgSwitcher />
      </div>

      {/* Navigation */}
      <nav className="flex-1" style={{ padding: 'var(--space-xs)' }}>
        <div className="flex flex-col gap-[1px]">
          {NAV_ITEMS.map((item) => {
            const active = pathname.startsWith(item.href);
            return (
              <Link
                key={item.href}
                href={item.href}
                className="flex items-center gap-[var(--space-sm)]"
                style={{
                  padding: 'var(--space-xs) var(--space-sm)',
                  borderRadius: 'var(--radius-sm)',
                  fontSize: 'var(--text-sm)',
                  fontWeight: active ? 500 : 400,
                  color: active ? 'var(--text-primary)' : 'var(--text-secondary)',
                  backgroundColor: active ? 'var(--bg-inset)' : undefined,
                  borderLeft: active ? '2px solid var(--amber-accent)' : '2px solid transparent',
                  transition: `all var(--duration-fast) ease`,
                }}
              >
                <svg
                  className="shrink-0"
                  style={{ width: '1rem', height: '1rem' }}
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth={1.5}
                >
                  <path strokeLinecap="round" strokeLinejoin="round" d={item.icon} />
                </svg>
                {item.label}
              </Link>
            );
          })}
        </div>
      </nav>
    </>
  );
}

/** Case-specific navigation when viewing a case */
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
      {/* Back to cases + case info */}
      <div
        style={{
          padding: 'var(--space-sm)',
          borderBottom: '1px solid var(--border-subtle)',
        }}
      >
        <Link
          href="/en/cases"
          className="flex items-center gap-[var(--space-xs)] mb-[var(--space-sm)]"
          style={{
            fontSize: 'var(--text-xs)',
            color: 'var(--text-tertiary)',
            fontWeight: 500,
            textTransform: 'uppercase',
            letterSpacing: '0.05em',
            transition: `color var(--duration-fast) ease`,
          }}
        >
          <svg
            style={{ width: '0.75rem', height: '0.75rem' }}
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2}
          >
            <path strokeLinecap="round" strokeLinejoin="round" d="M15 19l-7-7 7-7" />
          </svg>
          Cases
        </Link>
        <div
          className="font-[family-name:var(--font-mono)] text-[11px] tracking-wide mb-[2px]"
          style={{ color: 'var(--text-tertiary)' }}
        >
          {caseData.reference_code}
        </div>
        <div
          className="font-[family-name:var(--font-heading)] text-sm font-medium leading-snug truncate"
          style={{ color: 'var(--text-primary)' }}
          title={caseData.title}
        >
          {caseData.title}
        </div>
      </div>

      {/* Case navigation */}
      <nav className="flex-1 overflow-y-auto" style={{ padding: 'var(--space-xs)' }}>
        {CASE_SIDEBAR_GROUPS.map((group, gi) => {
          const visibleItems = group.items.filter(
            (item) => !item.adminOnly || caseData.canEdit
          );
          if (visibleItems.length === 0) return null;
          return (
            <div key={group.label} className={gi > 0 ? 'mt-[var(--space-md)]' : ''}>
              <h3
                className="text-[10px] uppercase tracking-[0.08em] font-semibold mb-[var(--space-xs)] px-[var(--space-sm)]"
                style={{ color: 'var(--text-tertiary)' }}
              >
                {group.label}
              </h3>
              <div className="flex flex-col gap-[1px]">
                {visibleItems.map((item) => {
                  const isActive = activeTab === item.key;
                  const count = sidebarCounts[item.key];
                  return (
                    <button
                      key={item.key}
                      type="button"
                      onClick={() => setActiveTab(item.key)}
                      aria-current={isActive ? 'page' : undefined}
                      className="w-full text-left flex items-center justify-between"
                      style={{
                        padding: 'var(--space-xs) var(--space-sm)',
                        borderRadius: 'var(--radius-sm)',
                        fontSize: 'var(--text-sm)',
                        fontWeight: isActive ? 500 : 400,
                        color: isActive ? 'var(--text-primary)' : 'var(--text-secondary)',
                        backgroundColor: isActive ? 'var(--bg-inset)' : 'transparent',
                        borderLeft: isActive
                          ? '2px solid var(--amber-accent)'
                          : '2px solid transparent',
                        borderTop: 'none',
                        borderRight: 'none',
                        borderBottom: 'none',
                        transition: `all var(--duration-fast) ease`,
                        cursor: 'pointer',
                      }}
                    >
                      <span className="truncate">{item.label}</span>
                      {count != null && count > 0 && (
                        <span
                          className="text-[11px] font-[family-name:var(--font-mono)] tabular-nums shrink-0 ml-[var(--space-sm)]"
                          style={{
                            color: isActive ? 'var(--amber-accent)' : 'var(--text-tertiary)',
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
