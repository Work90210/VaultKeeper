'use client';

import { useState, useCallback } from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';

const NAV_LINKS = [
  { label: 'Platform', href: '/platform' },
  { label: 'Security', href: '/security' },
  { label: 'Pricing', href: '/pricing' },
  { label: 'Manifesto', href: '/manifesto' },
  { label: 'Open source', href: '/docs' },
] as const;

export function MarketingHeader() {
  const pathname = usePathname();
  const [menuOpen, setMenuOpen] = useState(false);

  const isActive = (href: string) => pathname === href || pathname.endsWith(href);

  const toggleMenu = useCallback(() => {
    setMenuOpen((prev) => {
      const next = !prev;
      document.body.classList.toggle('menu-open', next);
      return next;
    });
  }, []);

  const closeMenu = useCallback(() => {
    setMenuOpen(false);
    document.body.classList.remove('menu-open');
  }, []);

  return (
    <nav className="nav">
      <div className="nav-inner">
        <Link className="brand" href="/">
          <span className="brand-mark"></span>
          Vault<em>Keeper</em>
        </Link>
        <div className="nav-links">
          {NAV_LINKS.map(({ label, href }) => (
            <Link key={href} href={href} className={isActive(href) ? 'active' : ''}>
              {label}
            </Link>
          ))}
        </div>
        <div className="nav-cta">
          <Link className="btn ghost sm" href="/en/login">Sign in</Link>
          <Link className="btn sm" href="/contact">Request a demo <span className="arr">&rarr;</span></Link>
        </div>
        <button
          className={`nav-burger${menuOpen ? ' open' : ''}`}
          aria-label="Open menu"
          onClick={toggleMenu}
        >
          <span></span>
        </button>
      </div>
      <div className={`mobile-menu${menuOpen ? ' open' : ''}`}>
        {NAV_LINKS.map(({ label, href }) => (
          <Link key={href} href={href} className={isActive(href) ? 'active' : ''} onClick={closeMenu}>
            {label}
          </Link>
        ))}
        <div className="mob-cta">
          <Link className="btn ghost" href="/contact" onClick={closeMenu}>Sign in</Link>
          <Link className="btn" href="/contact" onClick={closeMenu}>Request a demo <span className="arr">&rarr;</span></Link>
        </div>
      </div>
    </nav>
  );
}
