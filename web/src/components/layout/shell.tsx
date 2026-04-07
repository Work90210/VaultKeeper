'use client';

import { useTranslations } from 'next-intl';
import { Header } from './header';

export function Shell({ children }: { children: React.ReactNode }) {
  const t = useTranslations('common');

  return (
    <div className="min-h-screen flex flex-col" style={{ backgroundColor: 'var(--bg-primary)' }}>
      <Header />
      <main className="flex-1">{children}</main>
      <footer
        className="flex items-center justify-center py-[var(--space-md)] text-xs"
        style={{
          borderTop: '1px solid var(--border-subtle)',
          color: 'var(--text-tertiary)',
        }}
      >
        {t('appName')} &middot; {t('tagline')}
      </footer>
    </div>
  );
}
