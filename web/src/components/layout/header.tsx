'use client';

import { useAuth } from '@/hooks/use-auth';

export function Header() {
  const { user, isAuthenticated, signOut } = useAuth();

  return (
    <header className="flex items-center justify-between border-b px-6 py-4">
      <div className="flex items-center gap-3">
        <span className="text-lg font-semibold tracking-tight">VaultKeeper</span>
      </div>

      {isAuthenticated && user && (
        <div className="flex items-center gap-4">
          <div className="text-right">
            <p className="text-sm font-medium">{user.name}</p>
            <p className="text-xs text-zinc-500">{user.systemRole}</p>
          </div>
          <button
            onClick={signOut}
            className="rounded-md border px-3 py-1.5 text-sm text-zinc-600 transition-colors hover:bg-zinc-50"
            type="button"
          >
            Sign out
          </button>
        </div>
      )}
    </header>
  );
}
