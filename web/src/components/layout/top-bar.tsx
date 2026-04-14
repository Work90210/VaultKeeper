'use client';

import { NotificationBell } from '@/components/notifications/notification-bell';
import { SearchBar } from '@/components/search/search-bar';

export function TopBar() {
  return (
    <div
      className="flex items-center justify-between shrink-0"
      style={{
        height: '2.5rem',
        paddingInline: 'var(--space-lg)',
        borderBottom: '1px solid var(--border-subtle)',
        backgroundColor: 'var(--bg-primary)',
      }}
    >
      {/* Search */}
      <div className="hidden md:block" style={{ width: '16rem' }}>
        <SearchBar compact />
      </div>
      <div className="md:hidden" />

      {/* Right side */}
      <div className="flex items-center" style={{ gap: 'var(--space-sm)' }}>
        <NotificationBell />
      </div>
    </div>
  );
}
