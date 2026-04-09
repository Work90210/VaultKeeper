'use client';

import { useTranslations } from 'next-intl';
import Link from 'next/link';

const PRODUCT_LINKS = [
  { key: 'features', href: '/features' },
  { key: 'pricing', href: '/pricing' },
  { key: 'security', href: '/about' },
  { key: 'documentation', href: '/about' },
] as const;

const COMPANY_LINKS = [
  { key: 'about', href: '/about' },
  { key: 'contact', href: '/contact' },
  { key: 'pilot', href: '/contact' },
] as const;

const LEGAL_LINKS = [
  { key: 'privacy', href: '/about' },
  { key: 'terms', href: '/about' },
  { key: 'compliance', href: '/about' },
] as const;

export function MarketingFooter({ locale }: { locale: string }) {
  const t = useTranslations('marketing.footer');
  const currentYear = new Date().getFullYear();

  return (
    <footer
      style={{
        borderTop: '1px solid var(--border-default)',
        backgroundColor: 'var(--bg-secondary)',
      }}
    >
      <div className="marketing-section py-16 md:py-20">
        {/* Top row */}
        <div className="grid grid-cols-1 md:grid-cols-12 gap-12 md:gap-8">
          {/* Brand column */}
          <div className="md:col-span-4">
            <Link
              href={`/${locale}`}
              className="flex items-center gap-[var(--space-sm)] mb-4"
            >
              <svg
                width="24"
                height="28"
                viewBox="0 0 22 26"
                fill="none"
                aria-hidden="true"
              >
                <path
                  d="M11 1L2 5v7c0 6.075 3.75 10.35 9 12 5.25-1.65 9-5.925 9-12V5L11 1z"
                  stroke="var(--amber-accent)"
                  strokeWidth="1.5"
                  fill="none"
                />
                <path
                  d="M8 12l2.5 2.5L15 9"
                  stroke="var(--amber-accent)"
                  strokeWidth="1.5"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              </svg>
              <span
                className="font-[family-name:var(--font-heading)] text-lg"
                style={{ color: 'var(--text-primary)' }}
              >
                VaultKeeper
              </span>
            </Link>
            <p
              className="text-sm leading-relaxed max-w-xs"
              style={{ color: 'var(--text-secondary)' }}
            >
              {t('description')}
            </p>
          </div>

          {/* Link columns */}
          <div className="md:col-span-2">
            <h3
              className="text-xs font-semibold uppercase tracking-widest mb-4"
              style={{ color: 'var(--text-tertiary)' }}
            >
              {t('productTitle')}
            </h3>
            <ul className="space-y-3">
              {PRODUCT_LINKS.map(({ key, href }) => (
                <li key={key}>
                  <Link
                    href={`/${locale}${href}`}
                    className="text-sm transition-colors hover:text-text-primary"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    {t(key)}
                  </Link>
                </li>
              ))}
            </ul>
          </div>

          <div className="md:col-span-2">
            <h3
              className="text-xs font-semibold uppercase tracking-widest mb-4"
              style={{ color: 'var(--text-tertiary)' }}
            >
              {t('companyTitle')}
            </h3>
            <ul className="space-y-3">
              {COMPANY_LINKS.map(({ key, href }) => (
                <li key={key}>
                  <Link
                    href={`/${locale}${href}`}
                    className="text-sm transition-colors hover:text-text-primary"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    {t(key)}
                  </Link>
                </li>
              ))}
            </ul>
          </div>

          <div className="md:col-span-2">
            <h3
              className="text-xs font-semibold uppercase tracking-widest mb-4"
              style={{ color: 'var(--text-tertiary)' }}
            >
              {t('legalTitle')}
            </h3>
            <ul className="space-y-3">
              {LEGAL_LINKS.map(({ key, href }) => (
                <li key={key}>
                  <Link
                    href={`/${locale}${href}`}
                    className="text-sm transition-colors hover:text-text-primary"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    {t(key)}
                  </Link>
                </li>
              ))}
            </ul>
          </div>
        </div>

        {/* Bottom bar */}
        <div className="marketing-divider mt-12 mb-6" />
        <div className="flex flex-col sm:flex-row items-center justify-between gap-4">
          <p className="text-xs" style={{ color: 'var(--text-tertiary)' }}>
            &copy; {currentYear} VaultKeeper. {t('rights')}
          </p>
          <div className="flex items-center gap-4">
            <span
              className="inline-flex items-center gap-1.5 text-xs font-medium px-2.5 py-1 rounded-full"
              style={{
                backgroundColor: 'var(--status-active-bg)',
                color: 'var(--status-active)',
              }}
            >
              <span className="w-1.5 h-1.5 rounded-full bg-current" />
              {t('systemOperational')}
            </span>
          </div>
        </div>
      </div>
    </footer>
  );
}
