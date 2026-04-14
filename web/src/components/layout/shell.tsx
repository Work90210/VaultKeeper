'use client';

export function Shell({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-full" style={{ backgroundColor: 'var(--bg-primary)' }}>
      {children}
    </div>
  );
}
