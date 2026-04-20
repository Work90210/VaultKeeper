'use client';

import type { ReactNode } from 'react';

interface RecordDetailLayoutProps {
  readonly backHref: string;
  readonly backLabel: string;
  readonly recordType: string;
  readonly title: string;
  readonly subtitle?: string;
  readonly content: ReactNode;
  readonly sidebar: ReactNode;
  readonly actions?: ReactNode;
  readonly recordId: string;
  readonly createdAt: string;
  readonly updatedAt: string;
  readonly createdBy?: string;
}

function formatTimestamp(iso: string): string {
  return new Date(iso).toLocaleString('en-GB', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

export function RecordDetailLayout({
  backHref,
  backLabel,
  recordType,
  title,
  subtitle,
  content,
  sidebar,
  actions,
  recordId,
  createdAt,
  updatedAt,
  createdBy,
}: RecordDetailLayoutProps) {
  return (
    <div
      className="max-w-7xl mx-auto px-[var(--space-lg)] py-[var(--space-xl)]"
      style={{ animation: 'fade-in var(--duration-slow) var(--ease-out-expo)' }}
    >
      {/* Back link */}
      <nav className="mb-[var(--space-md)]">
        <a
          href={backHref}
          className="link-subtle text-xs uppercase tracking-wider font-medium inline-flex items-center gap-[var(--space-xs)]"
        >
          &larr; {backLabel}
        </a>
      </nav>

      {/* Header */}
      <header className="mb-[var(--space-lg)]">
        <div className="flex items-center gap-[var(--space-sm)] mb-[var(--space-xs)]">
          <span
            className="text-[10px] uppercase tracking-[0.08em] font-semibold px-[var(--space-sm)] py-[1px] rounded-[var(--radius-sm)]"
            style={{
              backgroundColor: 'var(--bg-inset)',
              color: 'var(--text-tertiary)',
              border: '1px solid var(--border-subtle)',
            }}
          >
            {recordType}
          </span>
          {subtitle && (
            <span className="text-xs" style={{ color: 'var(--text-tertiary)' }}>
              {subtitle}
            </span>
          )}
        </div>
        <div className="flex items-start justify-between gap-[var(--space-lg)]">
          <h1
            className="font-[family-name:var(--font-heading)] text-xl leading-tight text-balance flex-1"
            style={{ color: 'var(--text-primary)' }}
          >
            {title}
          </h1>
          {actions && (
            <div className="flex items-center gap-[var(--space-sm)] shrink-0">
              {actions}
            </div>
          )}
        </div>
      </header>

      {/* Content + Sidebar */}
      <div className="grid grid-cols-1 lg:grid-cols-[1fr_18rem] gap-0">
        {/* Main content */}
        <div
          className="space-y-[var(--space-lg)] pr-[var(--space-lg)]"
          style={{ borderRight: '1px solid var(--border-subtle)' }}
        >
          {content}
        </div>

        {/* Sidebar */}
        <div className="pl-[var(--space-lg)] space-y-[var(--space-md)]">
          {sidebar}
        </div>
      </div>

      {/* History / Audit Trail */}
      <div
        className="mt-[var(--space-xl)] pt-[var(--space-lg)]"
        style={{ borderTop: '1px solid var(--border-subtle)' }}
      >
        <h2
          className="text-[10px] uppercase tracking-[0.08em] font-semibold mb-[var(--space-sm)]"
          style={{ color: 'var(--text-tertiary)' }}
        >
          History
        </h2>
        <div className="space-y-[var(--space-xs)]">
          <div className="flex items-center gap-[var(--space-sm)]">
            <span
              className="w-1.5 h-1.5 rounded-full shrink-0"
              style={{ backgroundColor: 'var(--status-active)' }}
            />
            <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
              Created {formatTimestamp(createdAt)}
              {createdBy && (
                <span
                  className="ml-[var(--space-xs)] font-[family-name:var(--font-mono)] text-xs"
                  style={{ color: 'var(--text-tertiary)' }}
                >
                  by {createdBy}
                </span>
              )}
            </p>
          </div>
          {updatedAt !== createdAt && (
            <div className="flex items-center gap-[var(--space-sm)]">
              <span
                className="w-1.5 h-1.5 rounded-full shrink-0"
                style={{ backgroundColor: 'var(--amber-accent)' }}
              />
              <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
                Updated {formatTimestamp(updatedAt)}
              </p>
            </div>
          )}
        </div>
        <p
          className="text-[10px] font-[family-name:var(--font-mono)] mt-[var(--space-sm)]"
          style={{ color: 'var(--text-tertiary)' }}
        >
          {recordId}
        </p>
      </div>
    </div>
  );
}

export function MetaBlock({
  label,
  children,
}: {
  readonly label: string;
  readonly children: ReactNode;
}) {
  return (
    <div>
      <h4
        className="text-[10px] uppercase tracking-[0.08em] font-semibold mb-[var(--space-xs)]"
        style={{ color: 'var(--text-tertiary)' }}
      >
        {label}
      </h4>
      {children}
    </div>
  );
}

export function ContentSection({
  label,
  children,
}: {
  readonly label: string;
  readonly children: ReactNode;
}) {
  return (
    <div>
      <h3
        className="text-[10px] uppercase tracking-[0.08em] font-semibold mb-[var(--space-xs)]"
        style={{ color: 'var(--text-tertiary)' }}
      >
        {label}
      </h3>
      {children}
    </div>
  );
}

export function MetaRow({
  label,
  value,
  highlight,
}: {
  readonly label: string;
  readonly value: string | number;
  readonly highlight?: boolean;
}) {
  return (
    <div className="flex justify-between text-sm">
      <span style={{ color: 'var(--text-secondary)' }}>{label}</span>
      <span
        className="font-[family-name:var(--font-mono)] tabular-nums"
        style={{ color: highlight ? 'var(--amber-accent)' : 'var(--text-primary)' }}
      >
        {value}
      </span>
    </div>
  );
}

export function KeywordBadge({ children }: { readonly children: string }) {
  return (
    <span
      className="text-xs px-[var(--space-sm)] py-[2px] rounded-[var(--radius-sm)]"
      style={{
        backgroundColor: 'var(--bg-inset)',
        color: 'var(--text-secondary)',
        border: '1px solid var(--border-subtle)',
      }}
    >
      {children}
    </span>
  );
}
