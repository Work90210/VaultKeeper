'use client';

import { useState, useRef, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useOrg } from '@/hooks/use-org';
import type { OrgWithRole } from '@/types';

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
      <div className="skeleton" style={{ height: '2.25rem', borderRadius: 'var(--radius-md)' }} />
    );
  }

  if (!activeOrg) {
    return (
      <button
        onClick={() => router.push('/en/organizations/new')}
        style={{
          width: '100%',
          borderRadius: 'var(--radius-md)',
          border: '1px dashed var(--border-default)',
          padding: 'var(--space-xs) var(--space-sm)',
          fontSize: 'var(--text-sm)',
          color: 'var(--text-tertiary)',
          background: 'none',
          cursor: 'pointer',
          transition: `all var(--duration-fast) ease`,
        }}
      >
        Create Organization
      </button>
    );
  }

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen(!open)}
        className="flex w-full items-center justify-between"
        style={{
          borderRadius: 'var(--radius-md)',
          border: '1px solid var(--border-default)',
          backgroundColor: 'var(--bg-elevated)',
          padding: 'var(--space-xs) var(--space-sm)',
          fontSize: 'var(--text-sm)',
          fontWeight: 500,
          color: 'var(--text-primary)',
          cursor: 'pointer',
          boxShadow: 'var(--shadow-xs)',
          transition: `all var(--duration-fast) ease`,
          textAlign: 'left',
        }}
      >
        <span className="truncate">{activeOrg.name}</span>
        <svg
          className="shrink-0"
          style={{
            width: '0.875rem',
            height: '0.875rem',
            marginLeft: 'var(--space-xs)',
            color: 'var(--text-tertiary)',
            transform: open ? 'rotate(180deg)' : 'none',
            transition: `transform var(--duration-fast) ease`,
          }}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
        </svg>
      </button>

      {open && (
        <div
          className="absolute left-0 right-0 z-50"
          style={{
            marginTop: '0.25rem',
            borderRadius: 'var(--radius-md)',
            border: '1px solid var(--border-default)',
            backgroundColor: 'var(--bg-elevated)',
            boxShadow: 'var(--shadow-lg)',
            padding: '0.25rem 0',
          }}
        >
          {userOrgs.map((org) => (
            <button
              key={org.id}
              onClick={() => { setActiveOrg(org); setOpen(false); }}
              className="flex w-full items-center justify-between"
              style={{
                padding: 'var(--space-xs) var(--space-sm)',
                fontSize: 'var(--text-sm)',
                color: org.id === activeOrg.id ? 'var(--text-primary)' : 'var(--text-secondary)',
                fontWeight: org.id === activeOrg.id ? 500 : 400,
                backgroundColor: org.id === activeOrg.id ? 'var(--bg-inset)' : 'transparent',
                border: 'none',
                cursor: 'pointer',
                textAlign: 'left',
                width: '100%',
                transition: `background-color var(--duration-fast) ease`,
              }}
            >
              <span className="truncate">{org.name}</span>
              <span
                className="badge shrink-0"
                style={{
                  marginLeft: 'var(--space-xs)',
                  backgroundColor: 'var(--bg-inset)',
                  color: 'var(--text-tertiary)',
                }}
              >
                {ROLE_LABELS[org.role] ?? org.role}
              </span>
            </button>
          ))}
          <div style={{ borderTop: '1px solid var(--border-subtle)', marginTop: '0.25rem', paddingTop: '0.25rem' }}>
            <button
              onClick={() => { router.push('/en/organizations/new'); setOpen(false); }}
              className="flex w-full items-center gap-[var(--space-xs)]"
              style={{
                padding: 'var(--space-xs) var(--space-sm)',
                fontSize: 'var(--text-sm)',
                color: 'var(--text-tertiary)',
                border: 'none',
                background: 'none',
                cursor: 'pointer',
                textAlign: 'left',
                width: '100%',
              }}
            >
              <svg style={{ width: '0.875rem', height: '0.875rem' }} fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
              </svg>
              Create organization
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
