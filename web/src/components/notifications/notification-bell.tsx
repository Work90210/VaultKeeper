'use client';

import { useState, useEffect, useRef, useCallback } from 'react';
import { useSession } from 'next-auth/react';
import { useTranslations } from 'next-intl';
import { getUnreadCount } from '@/lib/notifications-api';
import { NotificationDropdown } from './notification-dropdown';

const POLL_INTERVAL = 30_000;

export function NotificationBell() {
  const t = useTranslations('notifications');
  const { data: session } = useSession();
  const [count, setCount] = useState(0);
  const [isOpen, setIsOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  const fetchCount = useCallback(async () => {
    if (!session?.accessToken) return;
    const result = await getUnreadCount(session.accessToken);
    if (result.data) setCount(result.data.count);
  }, [session?.accessToken]);

  useEffect(() => {
    fetchCount();
    const interval = setInterval(fetchCount, POLL_INTERVAL);
    return () => clearInterval(interval);
  }, [fetchCount]);

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setIsOpen(false);
      }
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, []);

  return (
    <div ref={containerRef} style={{ position: 'relative' }}>
      {/* .d-iconbtn with .notif dot — matches design exactly */}
      <button
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        className="d-iconbtn"
        title="Notifications"
        aria-label={t('title')}
        aria-expanded={isOpen}
      >
        <svg
          width="16"
          height="16"
          viewBox="0 0 16 16"
          fill="none"
          stroke="currentColor"
          strokeWidth={1.4}
        >
          <path d="M4 10.5a4 4 0 118 0v1H4zM6.5 12.5a1.5 1.5 0 003 0" />
        </svg>
        {count > 0 && <span className="notif" />}
      </button>

      {isOpen && (
        <NotificationDropdown
          onClose={() => setIsOpen(false)}
          onCountChange={setCount}
        />
      )}
    </div>
  );
}
