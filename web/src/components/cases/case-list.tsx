'use client';

import { useRouter, useSearchParams } from 'next/navigation';
import { useState } from 'react';

interface CaseItem {
  id: string;
  reference_code: string;
  title: string;
  status: string;
  legal_hold: boolean;
  jurisdiction: string;
  created_at: string;
}

const STATUS_STYLES: Record<string, { color: string; bg: string }> = {
  active: { color: 'var(--status-active)', bg: 'var(--status-active-bg)' },
  closed: { color: 'var(--status-closed)', bg: 'var(--status-closed-bg)' },
  archived: { color: 'var(--status-archived)', bg: 'var(--status-archived-bg)' },
};

export function CaseList({
  cases,
  nextCursor,
  hasMore,
  currentQuery,
  currentStatus,
}: {
  cases: CaseItem[];
  nextCursor: string;
  hasMore: boolean;
  currentQuery: string;
  currentStatus: string;
}) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [search, setSearch] = useState(currentQuery);

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    const params = new URLSearchParams(searchParams.toString());
    if (search) params.set('q', search);
    else params.delete('q');
    params.delete('cursor');
    router.push(`/en/cases?${params.toString()}`);
  };

  const handleStatusFilter = (status: string) => {
    const params = new URLSearchParams(searchParams.toString());
    if (status) params.set('status', status);
    else params.delete('status');
    params.delete('cursor');
    router.push(`/en/cases?${params.toString()}`);
  };

  return (
    <div className="space-y-[var(--space-md)]">
      {/* Toolbar */}
      <div className="flex flex-col sm:flex-row gap-[var(--space-sm)]">
        <form onSubmit={handleSearch} className="flex flex-1 gap-[var(--space-xs)]">
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search by reference, title, or description..."
            className="flex-1 px-[var(--space-sm)] py-[var(--space-xs)] text-[var(--text-sm)] transition-colors"
            style={{
              backgroundColor: 'var(--bg-elevated)',
              border: '1px solid var(--border-default)',
              color: 'var(--text-primary)',
            }}
            onFocus={(e) => {
              e.currentTarget.style.borderColor = 'var(--amber-accent)';
            }}
            onBlur={(e) => {
              e.currentTarget.style.borderColor = 'var(--border-default)';
            }}
          />
          <button
            type="submit"
            className="px-[var(--space-md)] py-[var(--space-xs)] text-[var(--text-sm)] font-medium transition-colors"
            style={{
              border: '1px solid var(--border-default)',
              color: 'var(--text-secondary)',
              backgroundColor: 'var(--bg-elevated)',
            }}
          >
            Search
          </button>
        </form>
        <select
          value={currentStatus}
          onChange={(e) => handleStatusFilter(e.target.value)}
          className="px-[var(--space-sm)] py-[var(--space-xs)] text-[var(--text-sm)]"
          style={{
            backgroundColor: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-secondary)',
          }}
        >
          <option value="">All statuses</option>
          <option value="active">Active</option>
          <option value="closed">Closed</option>
          <option value="archived">Archived</option>
        </select>
      </div>

      {/* Table */}
      {cases.length === 0 ? (
        <div
          className="py-[var(--space-2xl)] text-center"
          style={{ borderTop: '1px solid var(--border-default)' }}
        >
          <p
            className="font-[family-name:var(--font-heading)] text-[var(--text-xl)]"
            style={{ color: 'var(--text-tertiary)' }}
          >
            No cases found
          </p>
          <p className="mt-[var(--space-xs)] text-[var(--text-sm)]" style={{ color: 'var(--text-tertiary)' }}>
            {currentQuery
              ? 'Try adjusting your search terms.'
              : 'Create a case to get started.'}
          </p>
        </div>
      ) : (
        <div style={{ borderTop: '1px solid var(--border-strong)' }}>
          {/* Column headers */}
          <div
            className="grid grid-cols-[140px_1fr_90px_1fr_100px] gap-[var(--space-md)] px-[var(--space-sm)] py-[var(--space-xs)] text-[var(--text-xs)] uppercase tracking-wider font-medium"
            style={{
              color: 'var(--text-tertiary)',
              borderBottom: '1px solid var(--border-default)',
            }}
          >
            <span>Reference</span>
            <span>Title</span>
            <span>Status</span>
            <span>Jurisdiction</span>
            <span className="text-right">Date</span>
          </div>

          {/* Rows */}
          <div className="stagger-in">
            {cases.map((c) => {
              const style = STATUS_STYLES[c.status] || STATUS_STYLES.archived;
              return (
                <div
                  key={c.id}
                  onClick={() => router.push(`/en/cases/${c.id}`)}
                  className="grid grid-cols-[140px_1fr_90px_1fr_100px] gap-[var(--space-md)] px-[var(--space-sm)] py-[var(--space-sm)] items-center cursor-pointer transition-colors"
                  style={{ borderBottom: '1px solid var(--border-subtle)' }}
                  onMouseEnter={(e) => {
                    e.currentTarget.style.backgroundColor = 'var(--bg-inset)';
                  }}
                  onMouseLeave={(e) => {
                    e.currentTarget.style.backgroundColor = 'transparent';
                  }}
                >
                  <span
                    className="font-[family-name:var(--font-mono)] text-[var(--text-xs)] font-medium"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    {c.reference_code}
                  </span>

                  <span className="text-[var(--text-sm)] font-medium truncate" style={{ color: 'var(--text-primary)' }}>
                    {c.title}
                    {c.legal_hold && (
                      <span
                        className="ml-[var(--space-sm)] text-[var(--text-xs)] font-medium px-[var(--space-xs)] py-px inline-block"
                        style={{ backgroundColor: 'var(--status-hold-bg)', color: 'var(--status-hold)' }}
                      >
                        HOLD
                      </span>
                    )}
                  </span>

                  <span
                    className="text-[var(--text-xs)] font-medium px-[var(--space-xs)] py-px w-fit"
                    style={{ backgroundColor: style.bg, color: style.color }}
                  >
                    {c.status}
                  </span>

                  <span className="text-[var(--text-sm)] truncate" style={{ color: 'var(--text-secondary)' }}>
                    {c.jurisdiction || '\u2014'}
                  </span>

                  <span className="text-[var(--text-xs)] text-right tabular-nums" style={{ color: 'var(--text-tertiary)' }}>
                    {new Date(c.created_at).toLocaleDateString('en-GB', {
                      day: '2-digit',
                      month: 'short',
                      year: 'numeric',
                    })}
                  </span>
                </div>
              );
            })}
          </div>
        </div>
      )}

      {/* Pagination */}
      {hasMore && (
        <div className="pt-[var(--space-sm)]">
          <a
            href={`/en/cases?${new URLSearchParams({
              ...(currentQuery ? { q: currentQuery } : {}),
              ...(currentStatus ? { status: currentStatus } : {}),
              cursor: nextCursor,
            }).toString()}`}
            className="text-[var(--text-sm)] font-medium transition-colors"
            style={{ color: 'var(--amber-accent)' }}
            onMouseEnter={(e) => {
              e.currentTarget.style.color = 'var(--amber-hover)';
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.color = 'var(--amber-accent)';
            }}
          >
            Load more results &rarr;
          </a>
        </div>
      )}
    </div>
  );
}
