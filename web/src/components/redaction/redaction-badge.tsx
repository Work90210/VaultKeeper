'use client';

export function RedactionBadge() {
  return (
    <span
      className="inline-flex items-center gap-1 px-[var(--space-xs)] py-0.5 rounded text-xs font-medium"
      style={{ backgroundColor: 'rgba(30, 40, 60, 0.08)', color: 'rgb(60, 70, 90)' }}
    >
      <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round">
        <rect x="3" y="3" width="18" height="18" rx="2" />
        <rect x="7" y="7" width="10" height="4" fill="currentColor" />
      </svg>
      Redacted
    </span>
  );
}
