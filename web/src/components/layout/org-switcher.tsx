'use client';

import { useState, useRef, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useOrg } from '@/hooks/use-org';

const ROLE_LABELS: Record<string, string> = {
  owner: 'Owner',
  admin: 'Admin',
  member: 'Member',
};

export function OrgSwitcher() {
  const { activeOrg, userOrgs, setActiveOrg, loading } = useOrg();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const router = useRouter();

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  if (loading) {
    return (
      <div
        className="skeleton"
        style={{ height: '2rem', borderRadius: 'var(--radius-sm)' }}
      />
    );
  }

  if (!activeOrg) {
    return (
      <button
        onClick={() => router.push('/en/organizations/new')}
        className="flex w-full items-center gap-[var(--space-sm)]"
        style={{
          padding: '6px var(--space-sm)',
          fontSize: 'var(--text-sm)',
          color: 'var(--text-tertiary)',
          background: 'none',
          border: 'none',
          cursor: 'pointer',
          transition: `all var(--duration-fast) ease`,
        }}
      >
        <svg
          style={{ width: '1rem', height: '1rem' }}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2}
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M12 4v16m8-8H4"
          />
        </svg>
        Create Organization
      </button>
    );
  }

  const initial = activeOrg.name.charAt(0).toUpperCase();

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen(!open)}
        className="flex w-full items-center gap-[var(--space-sm)]"
        style={{
          padding: '6px var(--space-sm)',
          background: 'none',
          border: 'none',
          cursor: 'pointer',
          textAlign: 'left',
          borderRadius: 'var(--radius-sm)',
          transition: `background-color var(--duration-fast) ease`,
        }}
      >
        {/* Org avatar */}
        <div
          className="flex items-center justify-center shrink-0"
          style={{
            width: '1.25rem',
            height: '1.25rem',
            borderRadius: 'var(--radius-full)',
            backgroundColor: 'var(--amber-accent)',
            fontSize: '0.6875rem',
            fontWeight: 700,
            color: 'var(--bg-primary)',
            lineHeight: 1,
          }}
        >
          {initial}
        </div>

        {/* Org name */}
        <span
          className="flex-1 truncate"
          style={{
            fontSize: 'var(--text-sm)',
            fontWeight: 500,
            color: 'var(--text-primary)',
          }}
        >
          {activeOrg.name}
        </span>

        {/* Chevron */}
        <svg
          className="shrink-0"
          style={{
            width: '0.75rem',
            height: '0.75rem',
            color: 'var(--text-tertiary)',
            transform: open ? 'rotate(180deg)' : 'none',
            transition: `transform var(--duration-fast) ease`,
          }}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M19 9l-7 7-7-7"
          />
        </svg>
      </button>

      {open && (
        <div
          className="absolute left-0 right-0 z-50"
          style={{
            marginTop: '2px',
            borderRadius: 'var(--radius-md)',
            border: '1px solid var(--border-default)',
            backgroundColor: 'var(--bg-elevated)',
            boxShadow: 'var(--shadow-lg)',
            padding: '0.25rem 0',
          }}
        >
          {userOrgs.map((org) => {
            const isSelected = org.id === activeOrg.id;
            return (
              <button
                key={org.id}
                onClick={() => {
                  setActiveOrg(org);
                  setOpen(false);
                }}
                className="flex w-full items-center gap-[var(--space-sm)]"
                style={{
                  padding: '6px var(--space-sm)',
                  fontSize: 'var(--text-sm)',
                  color: isSelected
                    ? 'var(--text-primary)'
                    : 'var(--text-secondary)',
                  fontWeight: isSelected ? 500 : 400,
                  backgroundColor: isSelected
                    ? 'var(--bg-inset)'
                    : 'transparent',
                  border: 'none',
                  cursor: 'pointer',
                  textAlign: 'left',
                  width: '100%',
                  transition: `background-color var(--duration-fast) ease`,
                }}
              >
                <div
                  className="flex items-center justify-center shrink-0"
                  style={{
                    width: '1.25rem',
                    height: '1.25rem',
                    borderRadius: 'var(--radius-full)',
                    backgroundColor: isSelected
                      ? 'var(--amber-accent)'
                      : 'var(--bg-inset)',
                    fontSize: '0.6875rem',
                    fontWeight: 700,
                    color: isSelected
                      ? 'var(--bg-primary)'
                      : 'var(--text-tertiary)',
                    lineHeight: 1,
                  }}
                >
                  {org.name.charAt(0).toUpperCase()}
                </div>
                <span className="flex-1 truncate">{org.name}</span>
                <span
                  className="badge shrink-0"
                  style={{
                    backgroundColor: 'var(--bg-inset)',
                    color: 'var(--text-tertiary)',
                    fontSize: '0.6875rem',
                  }}
                >
                  {ROLE_LABELS[org.role] ?? org.role}
                </span>
              </button>
            );
          })}
          <div
            style={{
              borderTop: '1px solid var(--border-subtle)',
              marginTop: '0.25rem',
              paddingTop: '0.25rem',
            }}
          >
            <button
              onClick={() => {
                router.push('/en/organizations/new');
                setOpen(false);
              }}
              className="flex w-full items-center gap-[var(--space-sm)]"
              style={{
                padding: '6px var(--space-sm)',
                fontSize: 'var(--text-sm)',
                color: 'var(--text-tertiary)',
                border: 'none',
                background: 'none',
                cursor: 'pointer',
                textAlign: 'left',
                width: '100%',
              }}
            >
              <svg
                style={{ width: '0.875rem', height: '0.875rem' }}
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M12 4v16m8-8H4"
                />
              </svg>
              Create organization
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
