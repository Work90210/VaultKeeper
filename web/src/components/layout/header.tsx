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
      }}
    >
      <div className="flex items-center gap-[var(--space-lg)]">
        <a href="/en/cases" className="flex items-center gap-[var(--space-sm)]">
          <span
            className="font-[family-name:var(--font-heading)] text-[var(--text-lg)]"
            style={{ color: 'var(--text-primary)' }}
          >
            VaultKeeper
          </span>
        </a>
        {isAuthenticated && (
          <nav className="hidden sm:flex items-center gap-[var(--space-md)]">
            <a
              href="/en/cases"
              className="text-[var(--text-sm)] transition-colors"
              style={{ color: 'var(--text-secondary)' }}
              onMouseEnter={(e) => {
                e.currentTarget.style.color = 'var(--text-primary)';
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.color = 'var(--text-secondary)';
              }}
            >
              Cases
            </a>
          </nav>
        )}
      </div>

      {isAuthenticated && user && (
        <div className="flex items-center gap-[var(--space-md)]">
          <div className="hidden sm:block text-right">
            <p className="text-[var(--text-sm)] font-medium" style={{ color: 'var(--text-primary)' }}>
              {user.name}
            </p>
            <p className="text-[var(--text-xs)]" style={{ color: 'var(--text-tertiary)' }}>
              {user.systemRole?.replace('_', ' ')}
            </p>
          </div>
          <div
            className="w-px h-6"
            style={{ backgroundColor: 'var(--border-default)' }}
          />
          <button
            onClick={signOut}
            className="text-[var(--text-xs)] font-medium uppercase tracking-wide transition-colors"
            style={{ color: 'var(--text-tertiary)' }}
            onMouseEnter={(e) => {
              e.currentTarget.style.color = 'var(--text-primary)';
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.color = 'var(--text-tertiary)';
            }}
            type="button"
          >
            Sign out
          </button>
        </div>
      )}
    </header>
  );
}
