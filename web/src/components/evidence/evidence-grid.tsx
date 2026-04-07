'use client';

import { useRouter, useSearchParams } from 'next/navigation';
import { useState } from 'react';
import type { EvidenceItem } from '@/types';
import {
  formatFileSize,
  mimeLabel,
  mimeIcon,
  CLASSIFICATION_STYLES,
} from '@/lib/evidence-utils';

export function EvidenceGrid({
  caseId,
  evidence,
  nextCursor,
  hasMore,
  currentQuery,
  currentClassification,
}: {
  caseId: string;
  evidence: readonly EvidenceItem[];
  nextCursor: string;
  hasMore: boolean;
  currentQuery: string;
  currentClassification: string;
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
    router.push(`/en/cases/${caseId}/evidence?${params.toString()}`);
  };

  const handleClassificationFilter = (classification: string) => {
    const params = new URLSearchParams(searchParams.toString());
    if (classification) params.set('classification', classification);
    else params.delete('classification');
    params.delete('cursor');
    router.push(`/en/cases/${caseId}/evidence?${params.toString()}`);
  };

  return (
    <div className="space-y-[var(--space-md)]">
      {/* Toolbar */}
      <div className="flex flex-col sm:flex-row gap-[var(--space-sm)]">
        <form onSubmit={handleSearch} className="flex flex-1 gap-[var(--space-xs)]">
          <div className="relative flex-1">
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
              placeholder="Search evidence..."
              className="input-field pl-[var(--space-xl)] !py-[var(--space-xs)]"
            />
          </div>
          <button type="submit" className="btn-secondary">
            Search
          </button>
        </form>
        <select
          value={currentClassification}
          onChange={(e) => handleClassificationFilter(e.target.value)}
          className="input-field !w-auto"
          style={{ minWidth: '160px', padding: 'var(--space-xs) var(--space-sm)' }}
        >
          <option value="">All classifications</option>
          <option value="public">Public</option>
          <option value="restricted">Restricted</option>
          <option value="confidential">Confidential</option>
          <option value="ex_parte">Ex parte</option>
        </select>
      </div>

      {/* Table */}
      {evidence.length === 0 ? (
        <div className="card py-[var(--space-2xl)] text-center">
          <p
            className="font-[family-name:var(--font-heading)] text-xl"
            style={{ color: 'var(--text-tertiary)' }}
          >
            No evidence yet
          </p>
          <p
            className="mt-[var(--space-xs)] text-sm"
            style={{ color: 'var(--text-tertiary)' }}
          >
            {currentQuery
              ? 'Try adjusting your search terms.'
              : 'Upload files to get started.'}
          </p>
        </div>
      ) : (
        <div className="card overflow-hidden">
          {/* Column headers */}
          <div
            className="grid grid-cols-[100px_40px_1fr_70px_80px_110px_100px] gap-[var(--space-sm)] px-[var(--space-md)] py-[var(--space-sm)] text-xs uppercase tracking-wider font-semibold"
            style={{
              color: 'var(--text-tertiary)',
              borderBottom: '1px solid var(--border-default)',
              backgroundColor: 'var(--bg-inset)',
            }}
          >
            <span>Evidence #</span>
            <span aria-hidden="true" />
            <span>Title</span>
            <span>Type</span>
            <span className="text-right">Size</span>
            <span>Classification</span>
            <span className="text-right">Uploaded</span>
          </div>

          {/* Rows */}
          <div className="stagger-in">
            {evidence.map((item) => {
              const clsStyle =
                CLASSIFICATION_STYLES[item.classification] ||
                CLASSIFICATION_STYLES.restricted;

              return (
                <div
                  key={item.id}
                  role="button"
                  tabIndex={0}
                  aria-label={`View evidence ${item.evidence_number}: ${item.title}`}
                  onClick={() => router.push(`/en/evidence/${item.id}`)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault();
                      router.push(`/en/evidence/${item.id}`);
                    }
                  }}
                  className="table-row grid grid-cols-[100px_40px_1fr_70px_80px_110px_100px] gap-[var(--space-sm)] px-[var(--space-md)] py-[var(--space-sm)] items-center"
                  style={{ borderBottom: '1px solid var(--border-subtle)' }}
                >
                  <span
                    className="font-[family-name:var(--font-mono)] text-xs font-medium"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    {item.evidence_number}
                  </span>

                  {/* Thumbnail or icon */}
                  <span className="flex items-center justify-center">
                    {item.thumbnail_key ? (
                      /* eslint-disable-next-line @next/next/no-img-element */
                      <img
                        src={`/api/evidence/${item.id}/thumbnail`}
                        alt=""
                        className="rounded"
                        style={{
                          width: '32px',
                          height: '32px',
                          objectFit: 'cover',
                        }}
                      />
                    ) : (
                      <span
                        className="text-base"
                        aria-hidden="true"
                      >
                        {mimeIcon(item.mime_type)}
                      </span>
                    )}
                  </span>

                  <span
                    className="text-sm font-medium truncate"
                    style={{ color: 'var(--text-primary)' }}
                  >
                    {item.title || item.filename}
                  </span>

                  <span
                    className="text-xs"
                    style={{ color: 'var(--text-tertiary)' }}
                  >
                    {mimeLabel(item.mime_type)}
                  </span>

                  <span
                    className="text-xs text-right tabular-nums"
                    style={{ color: 'var(--text-tertiary)' }}
                  >
                    {formatFileSize(item.size_bytes)}
                  </span>

                  <span
                    className="badge w-fit"
                    style={{
                      backgroundColor: clsStyle.bg,
                      color: clsStyle.color,
                    }}
                  >
                    {item.classification.replace('_', ' ')}
                  </span>

                  <span
                    className="text-xs text-right tabular-nums"
                    style={{ color: 'var(--text-tertiary)' }}
                  >
                    {new Date(item.created_at).toLocaleDateString('en-GB', {
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
            href={`/en/cases/${caseId}/evidence?${new URLSearchParams({
              ...(currentQuery ? { q: currentQuery } : {}),
              ...(currentClassification
                ? { classification: currentClassification }
                : {}),
              cursor: nextCursor,
            }).toString()}`}
            className="link-accent text-sm"
          >
            Load more results &rarr;
          </a>
        </div>
      )}
    </div>
  );
}
