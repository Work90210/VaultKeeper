'use client';

import { useAuth } from '@/hooks/use-auth';

export function Header() {
  const { user, isAuthenticated, signOut } = useAuth();

  return (
    <header
      className="flex items-center justify-between px-[var(--space-lg)] h-14"
      style={{
        borderBottom: '1px solid var(--border-default)',
        backgroundColor: 'var(--bg-elevated)',
        boxShadow: 'var(--shadow-xs)',
      }}
    >
      <div className="flex items-center gap-[var(--space-lg)]">
        <a href="/en/cases" className="flex items-center gap-[var(--space-sm)]">
          {/* Shield mark */}
          <svg
            width="22"
            height="26"
            viewBox="0 0 22 26"
            fill="none"
            className="shrink-0"
            aria-hidden="true"
          >
            <path
              d="M11 1L2 5v7c0 6.075 3.75 10.35 9 12 5.25-1.65 9-5.925 9-12V5L11 1z"
              stroke="var(--amber-accent)"
              strokeWidth="1.5"
              fill="none"
            />
            <path
              d="M11 7v6m0 2.5v.5"
              stroke="var(--amber-accent)"
              strokeWidth="1.5"
              strokeLinecap="round"
            />
          </svg>
          <span
            className="font-[family-name:var(--font-heading)] text-[var(--text-lg)]"
            style={{ color: 'var(--text-primary)' }}
          >
            VaultKeeper
          </span>
        </a>
        {isAuthenticated && (
          <nav className="hidden sm:flex items-center gap-[var(--space-xs)]">
            <a
              href="/en/cases"
              className="btn-ghost text-[var(--text-sm)]"
            >
              Cases
            </a>
          </nav>
        )}
      </div>

      {isAuthenticated && user && (
        <div className="flex items-center gap-[var(--space-md)]">
          <div className="hidden sm:flex items-center gap-[var(--space-sm)]">
            {/* User avatar */}
            <div
              className="flex items-center justify-center w-8 h-8 text-[var(--text-xs)] font-semibold"
              style={{
                borderRadius: 'var(--radius-full)',
                backgroundColor: 'var(--amber-subtle)',
                color: 'var(--amber-accent)',
              }}
            >
              {Array.from(user.name || '?').slice(0, 2).join('').toUpperCase()}
            </div>
            <div className="text-right">
              <p className="text-[var(--text-sm)] font-medium" style={{ color: 'var(--text-primary)' }}>
                {user.name}
              </p>
              <p className="text-[var(--text-xs)]" style={{ color: 'var(--text-tertiary)' }}>
                {user.systemRole?.replace('_', ' ')}
              </p>
            </div>
          </div>
          <div
            className="w-px h-6"
            style={{ backgroundColor: 'var(--border-default)' }}
          />
          <button
            onClick={signOut}
            className="btn-ghost text-[var(--text-xs)] uppercase tracking-wide"
            type="button"
          >
            Sign out
          </button>
        </div>
      )}
    </header>
  );
}
