'use client';

import { useRouter } from 'next/navigation';
import { useLocale, useTranslations } from 'next-intl';
import type { SearchHit } from '@/lib/search-api';
import { mimeLabel } from '@/lib/evidence-utils';

function FileTypeIcon({ mimeType }: { mimeType?: string }) {
  const type = mimeType || '';
  let path: string;
  let color: string;

  if (type.startsWith('image/')) {
    path = 'M4 16l4-4 3 3 5-5 4 4V6a2 2 0 00-2-2H6a2 2 0 00-2 2v10z';
    color = 'var(--status-active)';
  } else if (type.startsWith('video/')) {
    path = 'M15 10l4.553-2.276A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14M5 18h8a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z';
    color = 'var(--status-hold)';
  } else if (type.startsWith('audio/')) {
    path = 'M9 19V6l12-3v13M9 19c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zM21 16c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2z';
    color = 'var(--amber-accent)';
  } else if (type.includes('pdf')) {
    path = 'M7 21h10a2 2 0 002-2V9l-5-5H7a2 2 0 00-2 2v13a2 2 0 002 2zM14 4v5h5';
    color = 'var(--status-hold)';
  } else {
    path = 'M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z';
    color = 'var(--text-tertiary)';
  }

  return (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" style={{ color, flexShrink: 0 }}>
      <path d={path} stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function HighlightedText({ html }: { html: string }) {
  return (
    <span
      dangerouslySetInnerHTML={{ __html: html }}
      className="[&_em]:font-semibold [&_em]:not-italic"
      style={{ color: 'var(--text-secondary)' }}
    />
  );
}

export function SearchResults({
  hits,
  totalHits,
  processingTimeMs,
  query,
  isLoading,
}: {
  hits: SearchHit[];
  totalHits: number;
  processingTimeMs: number;
  query: string;
  isLoading: boolean;
}) {
  const t = useTranslations('search');
  const locale = useLocale();
  const router = useRouter();

  if (isLoading) {
    return (
      <div className="space-y-[var(--space-md)]">
        {Array.from({ length: 5 }).map((_, i) => (
          <div
            key={i}
            className="card p-[var(--space-md)] animate-pulse"
          >
            <div className="flex items-start gap-[var(--space-sm)]">
              <div className="w-5 h-5 rounded" style={{ backgroundColor: 'var(--bg-inset)' }} />
              <div className="flex-1 space-y-2">
                <div className="h-4 rounded w-2/3" style={{ backgroundColor: 'var(--bg-inset)' }} />
                <div className="h-3 rounded w-full" style={{ backgroundColor: 'var(--bg-inset)' }} />
                <div className="h-3 rounded w-1/3" style={{ backgroundColor: 'var(--bg-inset)' }} />
              </div>
            </div>
          </div>
        ))}
      </div>
    );
  }

  if (hits.length === 0 && query) {
    return <SearchEmpty query={query} />;
  }

  return (
    <div>
      {query && (
        <p className="text-xs mb-[var(--space-md)]" style={{ color: 'var(--text-tertiary)' }}>
          {t('resultsCount', { count: totalHits, time: processingTimeMs })}
        </p>
      )}

      <div className="space-y-[var(--space-xs)] stagger-in">
        {hits.map((hit) => {
          const titleHighlight = hit.highlights?.title?.[0];
          const descHighlight = hit.highlights?.description?.[0];

          return (
            <button
              key={hit.evidence_id}
              type="button"
              onClick={() => router.push(`/${locale}/evidence/${hit.evidence_id}`)}
              className="card w-full text-left p-[var(--space-md)] transition-all hover:shadow-md"
              style={{ cursor: 'pointer' }}
            >
              <div className="flex items-start gap-[var(--space-sm)]">
                <FileTypeIcon mimeType={hit.mime_type} />
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-[var(--space-sm)]">
                    <h3
                      className="text-sm font-medium truncate"
                      style={{ color: 'var(--text-primary)' }}
                    >
                      {titleHighlight ? (
                        <HighlightedText html={titleHighlight} />
                      ) : (
                        hit.title
                      )}
                    </h3>
                    {hit.evidence_number && (
                      <span
                        className="text-xs font-mono shrink-0"
                        style={{ color: 'var(--text-tertiary)' }}
                      >
                        {hit.evidence_number}
                      </span>
                    )}
                  </div>

                  {(descHighlight || hit.description) && (
                    <p className="text-xs mt-0.5 line-clamp-2" style={{ color: 'var(--text-secondary)' }}>
                      {descHighlight ? (
                        <HighlightedText html={descHighlight} />
                      ) : (
                        hit.description
                      )}
                    </p>
                  )}

                  <div className="flex items-center gap-[var(--space-md)] mt-1.5">
                    {hit.mime_type && (
                      <span className="text-[10px] uppercase tracking-wider font-medium" style={{ color: 'var(--text-tertiary)' }}>
                        {mimeLabel(hit.mime_type)}
                      </span>
                    )}
                    {hit.classification && (
                      <span className="text-[10px] uppercase tracking-wider font-medium" style={{ color: 'var(--text-tertiary)' }}>
                        {hit.classification.replace('_', ' ')}
                      </span>
                    )}
                    {hit.uploaded_at && (
                      <span className="text-[10px]" style={{ color: 'var(--text-tertiary)' }}>
                        {new Date(hit.uploaded_at).toLocaleDateString()}
                      </span>
                    )}
                  </div>
                </div>
              </div>
            </button>
          );
        })}
      </div>
    </div>
  );
}

function SearchEmpty({ query }: { query: string }) {
  const t = useTranslations('search');

  return (
    <div className="text-center py-[var(--space-xl)]">
      <svg
        className="mx-auto mb-[var(--space-md)]"
        width="48"
        height="48"
        viewBox="0 0 24 24"
        fill="none"
        style={{ color: 'var(--text-tertiary)' }}
      >
        <circle cx="11" cy="11" r="7" stroke="currentColor" strokeWidth="1.5" />
        <path d="M16 16l4 4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        <path d="M8 11h6" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      </svg>
      <p className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
        {t('noResults')}
      </p>
      <p className="text-xs mt-1" style={{ color: 'var(--text-tertiary)' }}>
        {t('noResultsHint', { query })}
      </p>
    </div>
  );
}
