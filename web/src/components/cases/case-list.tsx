'use client';

import { useRouter, useSearchParams } from 'next/navigation';
import { useMemo, useState } from 'react';

interface CaseItem {
  id: string;
  reference_code: string;
  title: string;
  status: string;
  legal_hold: boolean;
  jurisdiction: string;
  created_at: string;
}

const STATUS_PILL: Record<string, string> = {
  active: 'pl live',
  closed: 'pl sealed',
  archived: 'pl draft',
};

const STATUS_LABEL: Record<string, string> = {
  active: 'active',
  closed: 'sealed',
  archived: 'draft',
};

function timeSince(dateStr: string): string {
  const seconds = Math.floor((Date.now() - new Date(dateStr).getTime()) / 1000);
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

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

  const statusCounts = useMemo(() => {
    const counts: Record<string, number> = { active: 0, closed: 0, archived: 0 };
    for (const c of cases) {
      if (c.legal_hold) {
        counts.hold = (counts.hold ?? 0) + 1;
      } else if (counts[c.status] !== undefined) {
        counts[c.status] += 1;
      }
    }
    return counts;
  }, [cases]);

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

  const filters: { key: string; label: string; count?: number }[] = [
    { key: '', label: 'All', count: cases.length },
    { key: 'active', label: 'Active', count: statusCounts.active },
    { key: 'hold', label: 'Legal hold', count: statusCounts.hold ?? 0 },
    { key: 'closed', label: 'Archived', count: statusCounts.closed },
    { key: 'archived', label: 'Draft', count: statusCounts.archived },
  ];

  return (
    <div className="panel">
      {/* Filter bar */}
      <div className="fbar">
        <form onSubmit={handleSearch} className="fsearch">
          <svg
            width="14"
            height="14"
            viewBox="0 0 16 16"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            style={{ color: 'var(--muted)' }}
          >
            <circle cx="7" cy="7" r="4" />
            <path d="M10 10l3 3" />
          </svg>
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Filter by ref, subject, jurisdiction&#8230;"
          />
        </form>
        {filters.map((f) => (
          <button
            key={f.key}
            type="button"
            className={`chip${currentStatus === f.key ? ' active' : ''}`}
            onClick={() => handleStatusFilter(f.key)}
          >
            {f.label}{' '}
            <span className={currentStatus === f.key ? 'x' : 'chev'}>
              {currentStatus === f.key ? `\u00b7${f.count}` : f.count}
            </span>
          </button>
        ))}
        <span className="chip">Role &middot; any <span className="chev">&#9662;</span></span>
        <span className="chip">Jurisdiction &middot; any <span className="chev">&#9662;</span></span>
      </div>

      {/* Table */}
      {cases.length === 0 ? (
        <div style={{ padding: '64px', textAlign: 'center' }}>
          <p
            style={{
              fontFamily: 'var(--font-heading)',
              fontSize: '22px',
              color: 'var(--muted)',
            }}
          >
            No cases found
          </p>
          <p style={{ fontSize: '14px', color: 'var(--muted)', marginTop: '8px' }}>
            {currentQuery
              ? 'Try adjusting your search terms.'
              : 'Create a case to get started.'}
          </p>
        </div>
      ) : (
        <table className="tbl">
          <thead>
            <tr>
              <th>Case</th>
              <th>Class.</th>
              <th>Your role</th>
              <th>Exhibits</th>
              <th>Witnesses</th>
              <th>Chain</th>
              <th>Team</th>
              <th>Status</th>
              <th>Last</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {cases.map((c) => {
              const pillClass = c.legal_hold
                ? 'pl hold'
                : STATUS_PILL[c.status] || 'pl draft';
              const statusLabel = c.legal_hold
                ? 'legal hold'
                : STATUS_LABEL[c.status] || c.status;

              const chainCell = c.legal_hold ? (
                <span className="pl hold">held</span>
              ) : c.status === 'archived' ? (
                <span className="pl draft">&mdash;</span>
              ) : (
                <div className="chain">
                  <span className="node on" />
                  <span className="seg" />
                  <span className="node on" />
                  <span className="seg" />
                  <span className="node on" />
                  <span className="seg" />
                  <span className="node on" />
                  <span className="seg" />
                  <span className="node on" />
                </div>
              );

              return (
                <tr
                  key={c.id}
                  style={{ cursor: 'pointer' }}
                  onClick={() => router.push(`/en/cases/${c.id}`)}
                >
                  <td>
                    <div className="ref">
                      {c.reference_code}
                      <small>{c.title}</small>
                    </div>
                  </td>
                  <td><span className="tag">{c.jurisdiction || '\u2014'}</span></td>
                  <td><span className="tag">&mdash;</span></td>
                  <td className="num">&mdash;</td>
                  <td className="num">&mdash;</td>
                  <td>{chainCell}</td>
                  <td>&mdash;</td>
                  <td>
                    <span className={pillClass}>{statusLabel}</span>
                  </td>
                  <td className="mono">{timeSince(c.created_at)}</td>
                  <td className="actions">
                    <a className="linkarrow">Open &rarr;</a>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      )}

      {/* Pagination */}
      {hasMore && (
        <div style={{ padding: '14px 22px', borderTop: '1px solid var(--line)' }}>
          <a
            href={`/en/cases?${new URLSearchParams({
              ...(currentQuery ? { q: currentQuery } : {}),
              ...(currentStatus ? { status: currentStatus } : {}),
              cursor: nextCursor,
            }).toString()}`}
            className="linkarrow"
          >
            Load more results &rarr;
          </a>
        </div>
      )}
    </div>
  );
}
