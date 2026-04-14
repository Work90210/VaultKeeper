import Link from 'next/link';
import { getOrganizations } from '@/lib/org-api';

export default async function OrganizationsPage() {
  const res = await getOrganizations();
  const orgs = res.data ?? [];

  return (
    <div style={{ maxWidth: '52rem', marginInline: 'auto', padding: 'var(--space-lg)' }}>
      <div className="flex items-center justify-between" style={{ marginBottom: 'var(--space-lg)' }}>
        <h1
          className="font-[family-name:var(--font-heading)]"
          style={{ fontSize: 'var(--text-xl)', color: 'var(--text-primary)' }}
        >
          Organizations
        </h1>
        <Link href="/en/organizations/new" className="btn-primary">
          Create Organization
        </Link>
      </div>

      {orgs.length === 0 ? (
        <div
          className="card-inset flex flex-col items-center justify-center"
          style={{ padding: 'var(--space-2xl)', textAlign: 'center' }}
        >
          <p style={{ fontSize: 'var(--text-sm)', color: 'var(--text-tertiary)' }}>
            No organizations yet. Create one to get started.
          </p>
        </div>
      ) : (
        <div className="stagger-in" style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-sm)' }}>
          {orgs.map((org) => (
            <Link
              key={org.id}
              href={`/en/organizations/${org.id}`}
              className="card table-row flex items-center justify-between"
              style={{ padding: 'var(--space-md) var(--space-lg)' }}
            >
              <div>
                <p style={{ fontWeight: 500, color: 'var(--text-primary)', fontSize: 'var(--text-base)' }}>
                  {org.name}
                </p>
                {org.description && (
                  <p style={{ fontSize: 'var(--text-sm)', color: 'var(--text-secondary)', marginTop: '0.125rem' }}>
                    {org.description}
                  </p>
                )}
                <p style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)', marginTop: 'var(--space-xs)' }}>
                  {org.member_count} members &middot; {org.case_count} cases
                </p>
              </div>
              <span
                className="badge"
                style={{ backgroundColor: 'var(--bg-inset)', color: 'var(--text-tertiary)', textTransform: 'capitalize' }}
              >
                {org.role}
              </span>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
