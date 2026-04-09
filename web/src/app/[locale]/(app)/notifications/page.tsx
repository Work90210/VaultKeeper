'use client';

import { useTranslations } from 'next-intl';
import { Shell } from '@/components/layout/shell';
import { NotificationList } from '@/components/notifications/notification-list';

export default function NotificationsPage() {
  const t = useTranslations('notifications');

  return (
    <Shell>
      <div className="max-w-3xl mx-auto px-[var(--space-lg)] py-[var(--space-lg)]">
        <h1
          className="font-[family-name:var(--font-heading)] text-xl mb-[var(--space-lg)]"
          style={{ color: 'var(--text-primary)' }}
        >
          {t('title')}
        </h1>
        <NotificationList />
      </div>
    </Shell>
  );
}
