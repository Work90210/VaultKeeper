'use client';

import { useState, useEffect, useCallback } from 'react';
import { useRouter } from 'next/navigation';
import { useSession } from 'next-auth/react';
import { useLocale, useTranslations } from 'next-intl';
import type { Notification } from '@/types';
import { listNotifications, markAllRead, markRead } from '@/lib/notifications-api';

function timeAgo(dateStr: string): string {
  const seconds = Math.floor((Date.now() - new Date(dateStr).getTime()) / 1000);
  if (seconds < 60) return 'just now';
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

function NotificationTypeIcon({ type }: { type?: string }) {
  const iconColor =
    type === 'integrity_warning' || type === 'backup_failed'
      ? 'var(--status-hold)'
      : type === 'evidence_uploaded'
        ? 'var(--status-active)'
        : 'var(--amber-accent)';

  return (
    <div
      className="flex items-center justify-center w-8 h-8 rounded-full shrink-0"
      style={{ backgroundColor: `color-mix(in oklch, ${iconColor} 15%, transparent)` }}
    >
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" style={{ color: iconColor }}>
        {type === 'evidence_uploaded' ? (
          <path d="M12 4v16m-8-8h16" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
        ) : type === 'integrity_warning' || type === 'backup_failed' ? (
          <path d="M12 9v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
        ) : type === 'user_added_to_case' ? (
          <path d="M16 21v-2a4 4 0 00-4-4H6a4 4 0 00-4 4v2M12 3a4 4 0 100 8 4 4 0 000-8z" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
        ) : (
          <path d="M15 17h5l-1.4-1.4A2 2 0 0118 14.2V11a6 6 0 00-4-5.7V5a2 2 0 10-4 0v.3A6 6 0 006 11v3.2c0 .5-.2 1-.6 1.4L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
        )}
      </svg>
    </div>
  );
}

export function NotificationDropdown({
  onClose,
  onCountChange,
}: {
  onClose: () => void;
  onCountChange: (count: number) => void;
}) {
  const t = useTranslations('notifications');
  const locale = useLocale();
  const router = useRouter();
  const { data: session } = useSession();
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  const fetchNotifications = useCallback(async () => {
    if (!session?.accessToken) return;
    setIsLoading(true);
    try {
      const result = await listNotifications(session.accessToken, 5);
      if (result.data) setNotifications(result.data);
    } finally {
      setIsLoading(false);
    }
  }, [session?.accessToken]);

  useEffect(() => {
    fetchNotifications();
  }, [fetchNotifications]);

  const handleMarkAllRead = async () => {
    if (!session?.accessToken) return;
    await markAllRead(session.accessToken);
    setNotifications((prev) => prev.map((n) => ({ ...n, read: true })));
    onCountChange(0);
  };

  const handleClickNotification = async (notification: Notification) => {
    if (!session?.accessToken) return;
    if (!notification.read) {
      await markRead(session.accessToken, notification.id);
      setNotifications((prev) =>
        prev.map((n) => (n.id === notification.id ? { ...n, read: true } : n))
      );
      onCountChange(Math.max(0, notifications.filter((n) => !n.read && n.id !== notification.id).length));
    }
    onClose();
    if (notification.case_id) {
      router.push(`/${locale}/cases/${notification.case_id}`);
    }
  };

  return (
    <div
      className="absolute right-0 top-full mt-2 w-96 max-h-[480px] overflow-y-auto rounded-[var(--radius-lg)] z-50"
      style={{
        backgroundColor: 'var(--bg-elevated)',
        border: '1px solid var(--border-default)',
        boxShadow: 'var(--shadow-lg)',
        animation: 'fade-in var(--duration-fast) ease',
      }}
    >
      {/* Header */}
      <div
        className="flex items-center justify-between px-[var(--space-md)] py-[var(--space-sm)]"
        style={{ borderBottom: '1px solid var(--border-subtle)' }}
      >
        <h3
          className="text-sm font-semibold"
          style={{ color: 'var(--text-primary)' }}
        >
          {t('title')}
        </h3>
        <button
          type="button"
          onClick={handleMarkAllRead}
          className="text-xs link-accent"
        >
          {t('markAllRead')}
        </button>
      </div>

      {/* Notifications list */}
      <div>
        {isLoading ? (
          <div className="p-[var(--space-md)]">
            {Array.from({ length: 3 }).map((_, i) => (
              <div key={i} className="flex gap-[var(--space-sm)] mb-[var(--space-sm)] animate-pulse">
                <div className="w-8 h-8 rounded-full" style={{ backgroundColor: 'var(--bg-inset)' }} />
                <div className="flex-1 space-y-1.5">
                  <div className="h-3 rounded w-3/4" style={{ backgroundColor: 'var(--bg-inset)' }} />
                  <div className="h-2.5 rounded w-1/2" style={{ backgroundColor: 'var(--bg-inset)' }} />
                </div>
              </div>
            ))}
          </div>
        ) : notifications.length === 0 ? (
          <div className="py-[var(--space-xl)] text-center">
            <p className="text-sm" style={{ color: 'var(--text-tertiary)' }}>
              {t('empty')}
            </p>
          </div>
        ) : (
          notifications.map((n) => (
            <button
              key={n.id}
              type="button"
              onClick={() => handleClickNotification(n)}
              className="w-full text-left flex gap-[var(--space-sm)] px-[var(--space-md)] py-[var(--space-sm)] transition-colors hover:bg-[var(--bg-inset)]"
              style={{
                borderBottom: '1px solid var(--border-subtle)',
                opacity: n.read ? 0.6 : 1,
              }}
            >
              <NotificationTypeIcon type={(n as Notification & { type?: string }).type} />
              <div className="flex-1 min-w-0">
                <p
                  className="text-sm font-medium truncate"
                  style={{ color: 'var(--text-primary)' }}
                >
                  {n.title}
                </p>
                {n.body && (
                  <p
                    className="text-xs truncate mt-0.5"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    {n.body}
                  </p>
                )}
                <p className="text-[10px] mt-0.5" style={{ color: 'var(--text-tertiary)' }}>
                  {timeAgo(n.created_at)}
                </p>
              </div>
              {!n.read && (
                <div
                  className="w-2 h-2 rounded-full mt-1.5 shrink-0"
                  style={{ backgroundColor: 'var(--amber-accent)' }}
                />
              )}
            </button>
          ))
        )}
      </div>

      {/* Footer */}
      <div
        className="px-[var(--space-md)] py-[var(--space-sm)] text-center"
        style={{ borderTop: '1px solid var(--border-subtle)' }}
      >
        <a
          href={`/${locale}/notifications`}
          className="text-xs link-accent"
          onClick={onClose}
        >
          {t('viewAll')}
        </a>
      </div>
    </div>
  );
}
