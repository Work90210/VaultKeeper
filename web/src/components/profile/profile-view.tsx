'use client';

import { useState } from 'react';
import Link from 'next/link';
import type { UserProfile, OrgWithRole } from '@/types';
import { ProfileForm } from './profile-form';

interface Props {
  profile: UserProfile;
  email: string;
  systemRole: string;
  organizations: OrgWithRole[];
}

export function ProfileView({ profile, email, systemRole, organizations }: Props) {
  const [editing, setEditing] = useState(false);

  return (
    <div style={{ maxWidth: '48rem', marginInline: 'auto', padding: 'var(--space-lg)' }}>
      <div style={{ animation: 'fade-in var(--duration-slow) var(--ease-out-expo)' }}>
        {/* Header */}
        <div className="flex items-start justify-between">
          <div className="flex items-center" style={{ gap: 'var(--space-md)' }}>
            <div
              className="flex items-center justify-center font-[family-name:var(--font-heading)]"
              style={{
                width: '3.5rem',
                height: '3.5rem',
                borderRadius: 'var(--radius-full)',
                backgroundColor: 'var(--amber-subtle)',
                color: 'var(--amber-accent)',
                fontSize: 'var(--text-xl)',
                fontWeight: 600,
              }}
            >
              {(profile.display_name || email)?.[0]?.toUpperCase() ?? '?'}
            </div>
            <div>
              <h1 style={{ fontSize: 'var(--text-lg)', fontWeight: 600, color: 'var(--text-primary)' }}>
                {profile.display_name || 'Unnamed User'}
              </h1>
              <p style={{ fontSize: 'var(--text-sm)', color: 'var(--text-tertiary)' }}>{email}</p>
              <span
                className="badge"
                style={{
                  marginTop: 'var(--space-xs)',
                  backgroundColor: systemRole === 'system_admin' ? 'var(--status-hold-bg)' : 'var(--bg-inset)',
                  color: systemRole === 'system_admin' ? 'var(--status-hold)' : 'var(--text-tertiary)',
                }}
              >
                {systemRole.replace('_', ' ')}
              </span>
            </div>
          </div>
          <button onClick={() => setEditing(!editing)} className="btn-secondary" type="button">
            {editing ? 'Cancel' : 'Edit'}
          </button>
        </div>

        {editing && (
          <div style={{ marginTop: 'var(--space-lg)' }}>
            <ProfileForm profile={profile} onSaved={() => setEditing(false)} />
          </div>
        )}

        {!editing && profile.bio && (
          <p style={{ marginTop: 'var(--space-lg)', fontSize: 'var(--text-sm)', color: 'var(--text-secondary)', lineHeight: 1.6 }}>
            {profile.bio}
          </p>
        )}

        {/* Organizations */}
        <section style={{ marginTop: 'var(--space-xl)' }}>
          <h2 className="field-label">My Organizations</h2>
          {organizations.length === 0 ? (
            <p style={{ fontSize: 'var(--text-sm)', color: 'var(--text-tertiary)' }}>
              No organizations yet.{' '}
              <Link href="/en/organizations/new" className="link-accent">Create one</Link>
            </p>
          ) : (
            <div className="stagger-in" style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-xs)', marginTop: 'var(--space-sm)' }}>
              {organizations.map((org) => (
                <Link
                  key={org.id}
                  href={`/en/organizations/${org.id}`}
                  className="card table-row flex items-center justify-between"
                  style={{ padding: 'var(--space-sm) var(--space-md)' }}
                >
                  <div>
                    <p style={{ fontSize: 'var(--text-sm)', fontWeight: 500, color: 'var(--text-primary)' }}>{org.name}</p>
                    <p style={{ fontSize: 'var(--text-xs)', color: 'var(--text-tertiary)' }}>
                      {org.member_count} members &middot; {org.case_count} cases
                    </p>
                  </div>
                  <span className="badge" style={{ backgroundColor: 'var(--bg-inset)', color: 'var(--text-tertiary)' }}>
                    {org.role}
                  </span>
                </Link>
              ))}
            </div>
          )}
        </section>
      </div>
    </div>
  );
}
