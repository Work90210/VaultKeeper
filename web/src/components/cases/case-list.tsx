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
          <div className="relative flex-1">
            {/* Search icon */}
            <svg
              className="absolute left-[var(--space-sm)] top-1/2 -translate-y-1/2 pointer-events-none"
              width="16"
              height="16"
              viewBox="0 0 24 24"
              fill="none"
              stroke="var(--text-tertiary)"
              strokeWidth="2"
              strokeLinecap="round"
              aria-hidden="true"
            >
              <circle cx="11" cy="11" r="8" />
              <path d="m21 21-4.35-4.35" />
            </svg>
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search by reference, title, or description..."
              className="input-field pl-[var(--space-xl)] !py-[var(--space-xs)]"
            />
          </div>
          <button type="submit" className="btn-secondary">
            Search
          </button>
        </form>
        <select
          value={currentStatus}
          onChange={(e) => handleStatusFilter(e.target.value)}
          className="input-field !w-auto"
          style={{ minWidth: '140px', padding: 'var(--space-xs) var(--space-sm)' }}
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
          className="card py-[var(--space-2xl)] text-center"
        >
          <p
            className="font-[family-name:var(--font-heading)] text-[var(--text-xl)]"
            style={{ color: 'var(--text-tertiary)' }}
          >
            No cases found
          </p>
          <p
            className="mt-[var(--space-xs)] text-[var(--text-sm)]"
            style={{ color: 'var(--text-tertiary)' }}
          >
            {currentQuery
              ? 'Try adjusting your search terms.'
              : 'Create a case to get started.'}
          </p>
        </div>
      ) : (
        <div className="card overflow-hidden">
          {/* Column headers */}
          <div
            className="grid grid-cols-[140px_1fr_90px_1fr_100px] gap-[var(--space-md)] px-[var(--space-md)] py-[var(--space-sm)] text-[var(--text-xs)] uppercase tracking-wider font-semibold"
            style={{
              color: 'var(--text-tertiary)',
              borderBottom: '1px solid var(--border-default)',
              backgroundColor: 'var(--bg-inset)',
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
                  role="button"
                  tabIndex={0}
                  aria-label={`Open case ${c.reference_code}: ${c.title}`}
                  onClick={() => router.push(`/en/cases/${c.id}`)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault();
                      router.push(`/en/cases/${c.id}`);
                    }
                  }}
                  className="table-row grid grid-cols-[140px_1fr_90px_1fr_100px] gap-[var(--space-md)] px-[var(--space-md)] py-[var(--space-sm)] items-center"
                  style={{ borderBottom: '1px solid var(--border-subtle)' }}
                >
                  <span
                    className="font-[family-name:var(--font-mono)] text-[var(--text-xs)] font-medium"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    {c.reference_code}
                  </span>

                  <span
                    className="text-[var(--text-sm)] font-medium truncate flex items-center gap-[var(--space-sm)]"
                    style={{ color: 'var(--text-primary)' }}
                  >
                    {c.title}
                    {c.legal_hold && (
                      <span
                        className="badge shrink-0"
                        style={{
                          backgroundColor: 'var(--status-hold-bg)',
                          color: 'var(--status-hold)',
                        }}
                      >
                        HOLD
                      </span>
                    )}
                  </span>

                  <span
                    className="badge w-fit"
                    style={{ backgroundColor: style.bg, color: style.color }}
                  >
                    {c.status}
                  </span>

                  <span
                    className="text-[var(--text-sm)] truncate"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    {c.jurisdiction || '\u2014'}
                  </span>

                  <span
                    className="text-[var(--text-xs)] text-right tabular-nums"
                    style={{ color: 'var(--text-tertiary)' }}
                  >
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
            className="link-accent text-[var(--text-sm)]"
          >
            Load more results &rarr;
          </a>
        </div>
      )}
    </div>
  );
}
