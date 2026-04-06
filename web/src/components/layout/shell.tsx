import { Header } from './header';

export function Shell({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-screen flex flex-col" style={{ backgroundColor: 'var(--bg-primary)' }}>
      <Header />
      <main className="flex-1">{children}</main>
      <footer
        className="flex items-center justify-center py-[var(--space-md)] text-[var(--text-xs)]"
        style={{
          borderTop: '1px solid var(--border-subtle)',
          color: 'var(--text-tertiary)',
        }}
      >
        VaultKeeper &middot; Sovereign Evidence Management
      </footer>
    </div>
  );
}
