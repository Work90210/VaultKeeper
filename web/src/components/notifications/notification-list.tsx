'use client';

import { useState, useEffect, useCallback } from 'react';
import { useRouter } from 'next/navigation';
import { useSession } from 'next-auth/react';
import { useLocale, useTranslations } from 'next-intl';
import type { Notification } from '@/types';
import { listNotifications, markRead, markAllRead } from '@/lib/notifications-api';

function timeAgo(dateStr: string): string {
  const seconds = Math.floor((Date.now() - new Date(dateStr).getTime()) / 1000);
  if (seconds < 60) return 'just now';
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;
  return new Date(dateStr).toLocaleDateString();
}

function NotificationIcon({ type }: { type?: string }) {
  const critical = type === 'integrity_warning' || type === 'backup_failed';
  const color = critical
    ? 'var(--status-hold)'
    : type === 'evidence_uploaded'
      ? 'var(--status-active)'
      : type === 'legal_hold_changed'
        ? 'var(--status-closed)'
        : 'var(--amber-accent)';

  return (
    <div
      className="flex items-center justify-center w-10 h-10 rounded-full shrink-0"
      style={{ backgroundColor: `color-mix(in oklch, ${color} 12%, transparent)` }}
    >
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" style={{ color }}>
        {type === 'evidence_uploaded' ? (
          <path d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
        ) : type === 'user_added_to_case' ? (
          <path d="M18 9v3m0 0v3m0-3h3m-3 0h-3m-2-5a4 4 0 11-8 0 4 4 0 018 0zM3 20a6 6 0 0112 0v1H3v-1z" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
        ) : critical ? (
          <path d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4.5c-.77-.833-2.694-.833-3.464 0L3.34 16.5c-.77.833.192 2.5 1.732 2.5z" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
        ) : type === 'legal_hold_changed' ? (
          <path d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
        ) : (
          <path d="M15 17h5l-1.4-1.4A2 2 0 0118 14.2V11a6 6 0 00-4-5.7V5a2 2 0 10-4 0v.3A6 6 0 006 11v3.2c0 .5-.2 1-.6 1.4L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
        )}
      </svg>
    </div>
  );
}

export function NotificationList() {
  const t = useTranslations('notifications');
  const locale = useLocale();
  const router = useRouter();
  const { data: session } = useSession();
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [filter, setFilter] = useState<'all' | 'unread'>('all');
  const [cursor, setCursor] = useState<string | undefined>();
  const [hasMore, setHasMore] = useState(false);

  const fetchNotifications = useCallback(
    async (append = false) => {
      if (!session?.accessToken) return;
      if (!append) setIsLoading(true);
      try {
        const result = await listNotifications(
          session.accessToken,
          20,
          append ? cursor : undefined
        );
        if (result.data) {
          setNotifications((prev) => (append ? [...prev, ...result.data!] : result.data!));
          setHasMore(result.meta?.has_more ?? false);
          setCursor(result.meta?.next_cursor);
        }
      } finally {
        setIsLoading(false);
      }
    },
    [session?.accessToken, cursor]
  );

  useEffect(() => {
    fetchNotifications();
  }, [session?.accessToken]); // eslint-disable-line react-hooks/exhaustive-deps

  const handleMarkAllRead = async () => {
    if (!session?.accessToken) return;
    await markAllRead(session.accessToken);
    setNotifications((prev) => prev.map((n) => ({ ...n, read: true })));
  };

  const handleClick = async (n: Notification) => {
    if (!session?.accessToken) return;
    if (!n.read) {
      await markRead(session.accessToken, n.id);
      setNotifications((prev) =>
        prev.map((item) => (item.id === n.id ? { ...item, read: true } : item))
      );
    }
    if (n.case_id) router.push(`/${locale}/cases/${n.case_id}`);
  };

  const displayed = filter === 'unread' ? notifications.filter((n) => !n.read) : notifications;

  if (isLoading) {
    return (
      <div className="space-y-[var(--space-sm)]">
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className="card p-[var(--space-md)] animate-pulse">
            <div className="flex gap-[var(--space-sm)]">
              <div className="w-10 h-10 rounded-full" style={{ backgroundColor: 'var(--bg-inset)' }} />
              <div className="flex-1 space-y-2">
                <div className="h-4 rounded w-2/3" style={{ backgroundColor: 'var(--bg-inset)' }} />
                <div className="h-3 rounded w-full" style={{ backgroundColor: 'var(--bg-inset)' }} />
              </div>
            </div>
          </div>
        ))}
      </div>
    );
  }

  return (
    <div>
      {/* Toolbar */}
      <div className="flex items-center justify-between mb-[var(--space-md)]">
        <div className="flex gap-1">
          {(['all', 'unread'] as const).map((f) => (
            <button
              key={f}
              type="button"
              onClick={() => setFilter(f)}
              className="px-[var(--space-sm)] py-1 text-xs font-medium rounded-[var(--radius-md)] transition-colors"
              style={{
                backgroundColor: filter === f ? 'var(--bg-inset)' : 'transparent',
                color: filter === f ? 'var(--text-primary)' : 'var(--text-tertiary)',
              }}
            >
              {t(f)}
            </button>
          ))}
        </div>
        <button type="button" onClick={handleMarkAllRead} className="text-xs link-accent">
          {t('markAllRead')}
        </button>
      </div>

      {/* List */}
      {displayed.length === 0 ? (
        <div className="text-center py-[var(--space-xl)]">
          <p className="text-sm" style={{ color: 'var(--text-tertiary)' }}>
            {t('empty')}
          </p>
        </div>
      ) : (
        <div className="space-y-[var(--space-xs)] stagger-in">
          {displayed.map((n) => (
            <button
              key={n.id}
              type="button"
              onClick={() => handleClick(n)}
              className="card w-full text-left flex gap-[var(--space-md)] p-[var(--space-md)] transition-all hover:shadow-md"
              style={{ opacity: n.read ? 0.6 : 1 }}
            >
              <NotificationIcon type={(n as Notification & { type?: string }).type} />
              <div className="flex-1 min-w-0">
                <div className="flex items-start justify-between gap-[var(--space-sm)]">
                  <p
                    className="text-sm font-medium"
                    style={{ color: 'var(--text-primary)' }}
                  >
                    {n.title}
                  </p>
                  <span
                    className="text-[10px] shrink-0 mt-0.5"
                    style={{ color: 'var(--text-tertiary)' }}
                  >
                    {timeAgo(n.created_at)}
                  </span>
                </div>
                {n.body && (
                  <p
                    className="text-xs mt-0.5 line-clamp-2"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    {n.body}
                  </p>
                )}
              </div>
              {!n.read && (
                <div
                  className="w-2.5 h-2.5 rounded-full mt-1 shrink-0"
                  style={{ backgroundColor: 'var(--amber-accent)' }}
                />
              )}
            </button>
          ))}
        </div>
      )}

      {/* Load more */}
      {hasMore && (
        <div className="text-center mt-[var(--space-lg)]">
          <button
            type="button"
            onClick={() => fetchNotifications(true)}
            className="btn-secondary"
          >
            {t('loadMore')}
          </button>
        </div>
      )}
    </div>
  );
}
