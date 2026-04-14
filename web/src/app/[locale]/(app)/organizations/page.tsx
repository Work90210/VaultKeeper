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
    <div style={{ padding: 'var(--space-lg)', maxWidth: '56rem' }}>
      <div className="flex items-center justify-between" style={{ marginBottom: 'var(--space-lg)' }}>
        <div>
          <h1
            className="font-[family-name:var(--font-heading)]"
            style={{ fontSize: 'var(--text-xl)', color: 'var(--text-primary)' }}
          >
            Organizations
          </h1>
          <p style={{ fontSize: 'var(--text-sm)', color: 'var(--text-tertiary)', marginTop: '0.125rem' }}>
            Manage your organizations and team access.
          </p>
        </div>
        <Link href="/en/organizations/new" className="btn-primary">
          Create Organization
        </Link>
      </div>

      {orgs.length === 0 ? (
        <div className="card-inset" style={{ padding: 'var(--space-2xl)', textAlign: 'center' }}>
          <p style={{ fontSize: 'var(--text-sm)', color: 'var(--text-tertiary)', marginBottom: 'var(--space-md)' }}>
            No organizations yet. Create one to get started.
          </p>
          <Link href="/en/organizations/new" className="btn-primary">
            Create your first organization
          </Link>
        </div>
      ) : (
        <div className="card-inset" style={{ overflow: 'hidden', padding: 0 }}>
          <table className="w-full" style={{ fontSize: 'var(--text-sm)', borderCollapse: 'collapse' }}>
            <thead>
              <tr style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                <th className="text-left field-label" style={{ padding: 'var(--space-sm) var(--space-md)', marginBottom: 0 }}>
                  Organization
                </th>
                <th className="text-left field-label" style={{ padding: 'var(--space-sm) var(--space-md)', marginBottom: 0 }}>
                  Your Role
                </th>
                <th className="text-right field-label" style={{ padding: 'var(--space-sm) var(--space-md)', marginBottom: 0 }}>
                  Members
                </th>
                <th className="text-right field-label" style={{ padding: 'var(--space-sm) var(--space-md)', marginBottom: 0 }}>
                  Cases
                </th>
              </tr>
            </thead>
            <tbody className="stagger-in">
              {orgs.map((org) => {
                const roleStyle = ROLE_STYLES[org.role] ?? ROLE_STYLES.member;
                return (
                  <tr
                    key={org.id}
                    className="table-row"
                    onClick={() => {}}
                    style={{ borderBottom: '1px solid var(--border-subtle)' }}
                  >
                    <td style={{ padding: 'var(--space-sm) var(--space-md)' }}>
                      <Link
                        href={`/en/organizations/${org.id}`}
                        className="flex items-center"
                        style={{ gap: 'var(--space-sm)', textDecoration: 'none' }}
                      >
                        <div
                          className="flex items-center justify-center shrink-0"
                          style={{
                            width: '1.75rem',
                            height: '1.75rem',
                            borderRadius: 'var(--radius-md)',
                            backgroundColor: 'var(--amber-subtle)',
                            color: 'var(--amber-accent)',
                            fontSize: 'var(--text-xs)',
                            fontWeight: 700,
                          }}
                        >
                          {org.name.charAt(0).toUpperCase()}
                        </div>
                        <div>
                          <p style={{ fontWeight: 500, color: 'var(--text-primary)' }}>
                            {org.name}
                          </p>
                          {org.description && (
                            <p className="truncate" style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)', maxWidth: '20rem' }}>
                              {org.description}
                            </p>
                          )}
                        </div>
                      </Link>
                    </td>
                    <td style={{ padding: 'var(--space-sm) var(--space-md)' }}>
                      <span
                        className="badge"
                        style={{ backgroundColor: roleStyle.bg, color: roleStyle.color, textTransform: 'capitalize' }}
                      >
                        {org.role}
                      </span>
                    </td>
                    <td
                      className="text-right font-[family-name:var(--font-mono)] tabular-nums"
                      style={{ padding: 'var(--space-sm) var(--space-md)', color: 'var(--text-secondary)' }}
                    >
                      {org.member_count}
                    </td>
                    <td
                      className="text-right font-[family-name:var(--font-mono)] tabular-nums"
                      style={{ padding: 'var(--space-sm) var(--space-md)', color: 'var(--text-secondary)' }}
                    >
                      {org.case_count}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
