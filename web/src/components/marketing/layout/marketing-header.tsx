'use client';

import { useState, useEffect } from 'react';
import { useTranslations } from 'next-intl';
import { motion, AnimatePresence } from 'motion/react';
import { Menu, X, Globe } from 'lucide-react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';

const NAV_LINKS = [
  { key: 'features', href: '/features' },
  { key: 'pricing', href: '/pricing' },
  { key: 'about', href: '/about' },
  { key: 'contact', href: '/contact' },
] as const;

export function MarketingHeader() {
  const t = useTranslations('marketing.nav');
  const pathname = usePathname();
  const [scrolled, setScrolled] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);

  const locale = pathname.startsWith('/fr') ? 'fr' : 'en';
  const altLocale = locale === 'en' ? 'fr' : 'en';
  const altPath = pathname.replace(`/${locale}`, `/${altLocale}`);

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 20);
    window.addEventListener('scroll', onScroll, { passive: true });
    return () => window.removeEventListener('scroll', onScroll);
  }, []);

  useEffect(() => {
    setMobileOpen(false);
  }, [pathname]);

  const isActive = (href: string) => {
    const full = `/${locale}${href}`;
    return pathname === full;
  };

  return (
    <>
      <motion.header
        className="fixed top-0 left-0 right-0 z-50 transition-all"
        initial={false}
        animate={{
          borderBottomColor: scrolled
            ? 'var(--border-default)'
            : 'transparent',
        }}
        style={{
          borderBottom: '1px solid transparent',
        }}
      >
        <div
          className={`transition-all duration-300 ${
            scrolled ? 'marketing-glass' : ''
          }`}
        >
          <div className="marketing-section flex items-center justify-between h-16 md:h-[4.5rem]">
            {/* Logo */}
            <Link
              href={`/${locale}`}
              className="flex items-center gap-[var(--space-sm)] group"
            >
              <svg
                width="26"
                height="30"
                viewBox="0 0 22 26"
                fill="none"
                className="shrink-0 transition-transform duration-300 group-hover:scale-105"
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
              <span className="font-[family-name:var(--font-heading)] text-xl tracking-tight"
                style={{ color: 'var(--text-primary)' }}>
                VaultKeeper
              </span>
            </Link>

            {/* Desktop nav */}
            <nav className="hidden md:flex items-center gap-1">
              {NAV_LINKS.map(({ key, href }) => (
                <Link
                  key={key}
                  href={`/${locale}${href}`}
                  className="relative px-4 py-2 text-sm font-medium transition-colors rounded-lg"
                  style={{
                    color: isActive(href)
                      ? 'var(--amber-accent)'
                      : 'var(--text-secondary)',
                  }}
                  onMouseEnter={(e) => {
                    if (!isActive(href)) {
                      e.currentTarget.style.color = 'var(--text-primary)';
                    }
                  }}
                  onMouseLeave={(e) => {
                    if (!isActive(href)) {
                      e.currentTarget.style.color = 'var(--text-secondary)';
                    }
                  }}
                >
                  {t(key)}
                  {isActive(href) && (
                    <motion.div
                      layoutId="nav-indicator"
                      className="absolute inset-x-2 -bottom-px h-0.5 rounded-full"
                      style={{ backgroundColor: 'var(--amber-accent)' }}
                      transition={{ type: 'spring', stiffness: 380, damping: 30 }}
                    />
                  )}
                </Link>
              ))}
            </nav>

            {/* Right side */}
            <div className="flex items-center gap-3">
              {/* Locale switcher */}
              <Link
                href={altPath}
                className="hidden md:flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium uppercase tracking-wider rounded-md transition-colors"
                style={{
                  color: 'var(--text-tertiary)',
                  border: '1px solid var(--border-subtle)',
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.color = 'var(--text-primary)';
                  e.currentTarget.style.borderColor = 'var(--border-strong)';
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.color = 'var(--text-tertiary)';
                  e.currentTarget.style.borderColor = 'var(--border-subtle)';
                }}
              >
                <Globe size={13} />
                {altLocale.toUpperCase()}
              </Link>

              {/* Sign in */}
              <Link
                href={`/${locale}/login`}
                className="hidden md:block text-sm font-medium transition-colors"
                style={{ color: 'var(--text-secondary)' }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.color = 'var(--text-primary)';
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.color = 'var(--text-secondary)';
                }}
              >
                {t('signIn')}
              </Link>

              {/* CTA */}
              <Link
                href={`/${locale}/contact`}
                className="hidden md:block btn-marketing-primary !py-2.5 !px-5 !text-xs"
              >
                {t('requestAccess')}
              </Link>

              {/* Mobile menu toggle */}
              <button
                type="button"
                className="md:hidden p-2 rounded-lg transition-colors"
                style={{ color: 'var(--text-secondary)' }}
                onClick={() => setMobileOpen(!mobileOpen)}
                aria-label="Toggle menu"
              >
                {mobileOpen ? <X size={22} /> : <Menu size={22} />}
              </button>
            </div>
          </div>
        </div>
      </motion.header>

      {/* Mobile menu */}
      <AnimatePresence>
        {mobileOpen && (
          <motion.div
            className="fixed inset-0 z-40 md:hidden"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.2 }}
          >
            <div
              className="absolute inset-0"
              style={{ backgroundColor: 'oklch(0 0 0 / 0.4)' }}
              onClick={() => setMobileOpen(false)}
            />
            <motion.nav
              className="absolute top-16 left-0 right-0 p-4"
              style={{
                backgroundColor: 'var(--bg-elevated)',
                borderBottom: '1px solid var(--border-default)',
                boxShadow: 'var(--shadow-lg)',
              }}
              initial={{ y: -10, opacity: 0 }}
              animate={{ y: 0, opacity: 1 }}
              exit={{ y: -10, opacity: 0 }}
              transition={{ type: 'spring', stiffness: 400, damping: 30 }}
            >
              <div className="flex flex-col gap-1">
                {NAV_LINKS.map(({ key, href }) => (
                  <Link
                    key={key}
                    href={`/${locale}${href}`}
                    className="px-4 py-3 text-base font-medium rounded-lg transition-colors"
                    style={{
                      color: isActive(href)
                        ? 'var(--amber-accent)'
                        : 'var(--text-primary)',
                      backgroundColor: isActive(href)
                        ? 'var(--amber-subtle)'
                        : 'transparent',
                    }}
                  >
                    {t(key)}
                  </Link>
                ))}
                <div className="marketing-divider my-2" />
                <div className="flex items-center gap-3 px-4 py-2">
                  <Link
                    href={altPath}
                    className="flex items-center gap-1.5 text-sm font-medium"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    <Globe size={14} />
                    {altLocale === 'fr' ? 'Fran\u00e7ais' : 'English'}
                  </Link>
                  <span style={{ color: 'var(--border-default)' }}>|</span>
                  <Link
                    href={`/${locale}/login`}
                    className="text-sm font-medium"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    {t('signIn')}
                  </Link>
                </div>
                <Link
                  href={`/${locale}/contact`}
                  className="btn-marketing-primary mt-2 text-center"
                >
                  {t('requestAccess')}
                </Link>
              </div>
            </motion.nav>
          </motion.div>
        )}
      </AnimatePresence>
    </>
  );
}
