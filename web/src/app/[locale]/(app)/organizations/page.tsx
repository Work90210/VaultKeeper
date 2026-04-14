import Link from 'next/link';
import { getOrganizations } from '@/lib/org-api';
import type { OrgWithRole } from '@/types';

const ROLE_STYLES: Record<string, { bg: string; color: string }> = {
  owner: { bg: 'var(--amber-subtle)', color: 'var(--amber-accent)' },
  admin: { bg: 'var(--status-active-bg)', color: 'var(--status-active)' },
  member: { bg: 'var(--bg-inset)', color: 'var(--text-tertiary)' },
};

export default async function OrganizationsPage() {
  const res = await getOrganizations();
  const orgs = res.data ?? [];

  return (
    <div style={{ padding: 'var(--space-lg)' }}>
      <div className="flex items-center justify-between" style={{ marginBottom: 'var(--space-lg)' }}>
        <div>
          <h1
            className="font-[family-name:var(--font-heading)]"
            style={{ fontSize: 'var(--text-xl)', color: 'var(--text-primary)' }}
          >
            Organizations
          </h1>
          <p style={{ fontSize: 'var(--text-sm)', color: 'var(--text-tertiary)', marginTop: '0.125rem' }}>
            {orgs.length} organization{orgs.length !== 1 ? 's' : ''}
          </p>
        </div>
        <Link href="/en/organizations/new" className="btn-primary">
          Create Organization
        </Link>
      </div>

      {orgs.length === 0 ? (
        <div
          className="card flex flex-col items-center justify-center"
          style={{ padding: 'var(--space-2xl)', textAlign: 'center' }}
        >
          <svg
            style={{ width: '3rem', height: '3rem', color: 'var(--text-tertiary)', marginBottom: 'var(--space-md)' }}
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={1}
          >
            <path strokeLinecap="round" strokeLinejoin="round" d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
          </svg>
          <p style={{ fontSize: 'var(--text-base)', fontWeight: 500, color: 'var(--text-primary)', marginBottom: 'var(--space-xs)' }}>
            No organizations yet
          </p>
          <p style={{ fontSize: 'var(--text-sm)', color: 'var(--text-tertiary)', marginBottom: 'var(--space-md)' }}>
            Create an organization to start managing cases and team members.
          </p>
          <Link href="/en/organizations/new" className="btn-primary">
            Create your first organization
          </Link>
        </div>
      ) : (
        <div className="stagger-in grid grid-cols-1 lg:grid-cols-2 xl:grid-cols-3" style={{ gap: 'var(--space-md)' }}>
          {orgs.map((org) => (
            <OrgCard key={org.id} org={org} />
          ))}
        </div>
      )}
    </div>
  );
}

function OrgCard({ org }: { org: OrgWithRole }) {
  const roleStyle = ROLE_STYLES[org.role] ?? ROLE_STYLES.member;
  const initial = org.name.charAt(0).toUpperCase();

  return (
    <Link
      href={`/en/organizations/${org.id}`}
      className="card"
      style={{
        padding: 'var(--space-lg)',
        display: 'flex',
        flexDirection: 'column',
        gap: 'var(--space-md)',
        transition: 'box-shadow var(--duration-normal) var(--ease-out-expo), transform var(--duration-normal) var(--ease-out-expo)',
        textDecoration: 'none',
      }}
    >
      {/* Header: avatar + name + role */}
      <div className="flex items-start justify-between">
        <div className="flex items-center" style={{ gap: 'var(--space-sm)' }}>
          <div
            className="flex items-center justify-center shrink-0"
            style={{
              width: '2.5rem',
              height: '2.5rem',
              borderRadius: 'var(--radius-lg)',
              backgroundColor: 'var(--amber-subtle)',
              color: 'var(--amber-accent)',
              fontSize: 'var(--text-lg)',
              fontWeight: 700,
            }}
          >
            {initial}
          </div>
          <div>
            <p
              className="font-[family-name:var(--font-heading)]"
              style={{ fontWeight: 600, color: 'var(--text-primary)', fontSize: 'var(--text-base)', lineHeight: 1.3 }}
            >
              {org.name}
            </p>
            {org.description && (
              <p
                className="truncate"
                style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)', maxWidth: '14rem', marginTop: '1px' }}
              >
                {org.description}
              </p>
            )}
          </div>
        </div>
        <span
          className="badge shrink-0"
          style={{ backgroundColor: roleStyle.bg, color: roleStyle.color, textTransform: 'capitalize' }}
        >
          {org.role}
        </span>
      </div>

      {/* Stats row */}
      <div
        className="grid grid-cols-3"
        style={{
          padding: 'var(--space-sm) var(--space-md)',
          borderRadius: 'var(--radius-md)',
          backgroundColor: 'var(--bg-inset)',
        }}
      >
        <div>
          <p
            className="font-[family-name:var(--font-mono)] tabular-nums"
            style={{ fontSize: 'var(--text-lg)', fontWeight: 600, color: 'var(--text-primary)', lineHeight: 1.2 }}
          >
            {org.member_count}
          </p>
          <p style={{ fontSize: '0.6875rem', color: 'var(--text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
            Members
          </p>
        </div>
        <div>
          <p
            className="font-[family-name:var(--font-mono)] tabular-nums"
            style={{ fontSize: 'var(--text-lg)', fontWeight: 600, color: 'var(--text-primary)', lineHeight: 1.2 }}
          >
            {org.case_count}
          </p>
          <p style={{ fontSize: '0.6875rem', color: 'var(--text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
            Cases
          </p>
        </div>
        <div>
          <p
            className="font-[family-name:var(--font-mono)] tabular-nums"
            style={{ fontSize: 'var(--text-lg)', fontWeight: 600, color: 'var(--text-primary)', lineHeight: 1.2 }}
          >
            {org.slug?.slice(0, 6) ?? '\u2014'}
          </p>
          <p style={{ fontSize: '0.6875rem', color: 'var(--text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
            Slug
          </p>
        </div>
      </div>
    </Link>
  );
}
