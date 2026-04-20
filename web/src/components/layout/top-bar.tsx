'use client';

import { useRouter } from 'next/navigation';
import { NotificationBell } from '@/components/notifications/notification-bell';

export function TopBar() {
  const router = useRouter();

  return (
    <div className="d-top">
      {/* Breadcrumb area (left side of grid) */}
      <nav className="d-crumb">
        {/* Breadcrumbs can be populated by page-level components if needed */}
      </nav>

      {/* Top actions (right side of grid) */}
      <div className="d-top-actions">
        {/* Search pill: .d-search with icon + placeholder + kbd shortcut */}
        <div
          className="d-search"
          onClick={() => router.push('/en/search')}
          role="button"
          tabIndex={0}
          onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ' ') router.push('/en/search');
          }}
        >
          <svg
            width="14"
            height="14"
            viewBox="0 0 16 16"
            fill="none"
            stroke="currentColor"
            strokeWidth={1.5}
          >
            <circle cx="7" cy="7" r="4" />
            <path d="M10 10l3 3" />
          </svg>
          <span>Search exhibits, witnesses, hashes&hellip;</span>
          <kbd>{'\u2318'}K</kbd>
        </div>

        {/* Notification bell: .d-iconbtn with .notif dot */}
        <NotificationBell />

        {/* Help button: .d-iconbtn */}
        <button
          type="button"
          className="d-iconbtn"
          title="Help"
        >
          <svg
            width="16"
            height="16"
            viewBox="0 0 16 16"
            fill="none"
            stroke="currentColor"
            strokeWidth={1.4}
          >
            <circle cx="8" cy="8" r="6" />
            <path d="M6.4 6.4a1.6 1.6 0 113 .6c-.6.3-1 .7-1 1.4M8 10.5v.5" />
          </svg>
        </button>
      </div>
    </div>
  );
}
