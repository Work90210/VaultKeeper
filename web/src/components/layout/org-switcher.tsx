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
        className="d-org"
        style={{ opacity: 0.5, pointerEvents: 'none' }}
      >
        <span className="av" style={{ background: 'var(--bg-2)' }}>&nbsp;</span>
        <span className="name">Loading&hellip;</span>
      </div>
    );
  }

  if (!activeOrg) {
    return (
      <button
        onClick={() => router.push('/en/organizations/new')}
        className="d-org"
        style={{ border: '1px dashed var(--line-2)' }}
      >
        <span className="av" style={{ background: 'var(--muted)', fontSize: '18px' }}>+</span>
        <span className="name">Create Organization</span>
      </button>
    );
  }

  const initial = activeOrg.name.charAt(0).toUpperCase();

  return (
    <div ref={ref} style={{ position: 'relative' }}>
      {/* Org picker button: .d-org > .av + .name > small */}
      <div
        className="d-org"
        onClick={() => setOpen(!open)}
        role="button"
        tabIndex={0}
        onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') setOpen(!open); }}
      >
        <span className="av">{initial}</span>
        <span className="name">
          {activeOrg.name}
          <small>{ROLE_LABELS[activeOrg.role] ?? activeOrg.role}</small>
        </span>
        <svg
          width="14"
          height="14"
          viewBox="0 0 16 16"
          fill="none"
          stroke="currentColor"
          strokeWidth={1.4}
          style={{
            color: 'var(--muted)',
            opacity: open ? 1 : 0,
            transition: 'opacity 0.2s',
          }}
        >
          <path d="M4 6l-2 2 2 2M12 6l2 2-2 2M6 8h4" />
        </svg>
      </div>

      {/* Dropdown: .d-org-dropdown */}
      {open && (
        <div
          className="d-org-dropdown"
          style={{ display: 'block', position: 'absolute', left: 0, right: 0, zIndex: 50 }}
        >
          {userOrgs.map((org) => {
            const isSelected = org.id === activeOrg.id;
            const orgInitial = org.name.charAt(0).toUpperCase();
            return (
              <button
                key={org.id}
                onClick={() => {
                  setActiveOrg(org);
                  setOpen(false);
                }}
                className={`org-item${isSelected ? ' active' : ''}`}
              >
                <span
                  className="oa"
                  style={{ background: isSelected ? 'var(--ink)' : 'var(--muted)' }}
                >
                  {orgInitial}
                </span>
                <span>
                  {org.name}
                  <small>{ROLE_LABELS[org.role] ?? org.role}</small>
                </span>
              </button>
            );
          })}

          {/* Divider + actions */}
          <div style={{ height: '1px', background: 'var(--line)', margin: '4px 0' }} />

          {activeOrg && (activeOrg.role === 'owner' || activeOrg.role === 'admin') && (
            <button
              onClick={() => {
                router.push('/en/settings?tab=organization');
                setOpen(false);
              }}
              className="org-item"
            >
              <span className="oa" style={{ background: 'transparent' }}>
                <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="var(--muted)" strokeWidth={1.4}>
                  <circle cx="8" cy="8" r="2" />
                  <path d="M8 2v2M8 12v2M2 8h2M12 8h2M3.5 3.5l1.4 1.4M11.1 11.1l1.4 1.4M3.5 12.5l1.4-1.4M11.1 4.9l1.4-1.4" />
                </svg>
              </span>
              <span style={{ color: 'var(--ink-2)' }}>Manage organization</span>
            </button>
          )}

          <button
            onClick={() => {
              router.push('/en/organizations/new');
              setOpen(false);
            }}
            className="org-item"
          >
            <span className="oa" style={{ background: 'transparent' }}>
              <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="var(--muted)" strokeWidth={1.4}>
                <path d="M8 3v10M3 8h10" />
              </svg>
            </span>
            <span style={{ color: 'var(--muted)' }}>Create organization</span>
          </button>
        </div>
      )}
    </div>
  );
}
