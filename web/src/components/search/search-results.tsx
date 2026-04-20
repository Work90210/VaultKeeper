'use client';

import { useRouter } from 'next/navigation';
import { useLocale, useTranslations } from 'next-intl';
import DOMPurify from 'isomorphic-dompurify';
import { Search as SearchIcon } from 'lucide-react';
import type { SearchHit } from '@/lib/search-api';
import { mimeLabel } from '@/lib/evidence-utils';

function HighlightedText({ html }: { html: string }) {
  const clean = DOMPurify.sanitize(html, { ALLOWED_TAGS: ['em', 'mark'], ALLOWED_ATTR: [] });
  return (
    <span
      dangerouslySetInnerHTML={{ __html: clean }}
      className="[&_em]:font-medium [&_em]:not-italic [&_em]:text-[color:var(--ink)] [&_em]:bg-[rgba(200,126,94,.2)] [&_em]:px-[2px] [&_mark]:bg-[rgba(200,126,94,.2)] [&_mark]:text-[color:var(--ink)] [&_mark]:font-medium [&_mark]:px-[2px]"
    />
  );
}

export function SearchResults({
  hits,
  totalHits,
  processingTimeMs,
  query,
  isLoading,
  facets,
}: {
  hits: SearchHit[];
  totalHits: number;
  processingTimeMs: number;
  query: string;
  isLoading: boolean;
  facets?: Record<string, Record<string, number>>;
}) {
  const t = useTranslations('search');
  const locale = useLocale();
  const router = useRouter();

  if (isLoading && hits.length === 0) {
    return <LoadingState />;
  }

  if (!isLoading && hits.length === 0 && query) {
    return <SearchEmpty query={query} />;
  }

  return (
    <div className={`g2-wide${isLoading ? ' opacity-60 pointer-events-none' : ''}`}>
      {/* Results panel */}
      <div className="panel">
        <div className="panel-h">
          <h3>Results</h3>
          <span className="meta">ranked &middot; relevance &middot; recency</span>
        </div>
        <div className="panel-body" style={{ display: 'flex', flexDirection: 'column', gap: 14, padding: 0 }}>
          {hits.map((hit) => {
            const titleHighlight = hit.highlights?.title?.[0];
            const descHighlight = hit.highlights?.description?.[0];
            const displayTitle = titleHighlight
              ? titleHighlight
              : (hit.title || hit.file_name || t('untitled'));
            const metaParts = [
              hit.mime_type ? mimeLabel(hit.mime_type) : null,
              hit.case_id || null,
              hit.uploaded_at
                ? new Date(hit.uploaded_at).toLocaleDateString('en-GB', {
                    day: '2-digit',
                    month: 'short',
                  })
                : null,
            ].filter(Boolean);

            return (
              <div
                key={hit.evidence_id}
                style={{
                  padding: '16px 22px',
                  borderBottom: '1px solid var(--line)',
                  display: 'grid',
                  gridTemplateColumns: '1fr auto',
                  gap: 12,
                  alignItems: 'start',
                  cursor: 'pointer',
                }}
                onClick={() => router.push(`/${locale}/evidence/${hit.evidence_id}`)}
                role="button"
                tabIndex={0}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault();
                    router.push(`/${locale}/evidence/${hit.evidence_id}`);
                  }
                }}
              >
                <div>
                  {/* Title */}
                  <a
                    href={`/${locale}/evidence/${hit.evidence_id}`}
                    onClick={(e) => e.preventDefault()}
                    style={{
                      fontFamily: "'Fraunces', serif",
                      fontSize: 17,
                      letterSpacing: '-.005em',
                      color: 'var(--ink)',
                      textDecoration: 'none',
                    }}
                  >
                    {titleHighlight ? (
                      <HighlightedText html={titleHighlight} />
                    ) : (
                      displayTitle
                    )}
                  </a>

                  {/* Snippet */}
                  {(descHighlight || hit.description) && (
                    <div
                      style={{
                        fontSize: 13.5,
                        color: 'var(--muted)',
                        lineHeight: 1.55,
                        marginTop: 6,
                      }}
                    >
                      {descHighlight ? (
                        <HighlightedText html={descHighlight} />
                      ) : (
                        hit.description
                      )}
                    </div>
                  )}

                  {/* Meta row */}
                  <div
                    style={{
                      fontFamily: "'JetBrains Mono', monospace",
                      fontSize: 10.5,
                      color: 'var(--muted)',
                      letterSpacing: '.04em',
                      textTransform: 'uppercase',
                      marginTop: 8,
                    }}
                  >
                    {hit.evidence_number && `${hit.evidence_number} \u00b7 `}
                    {metaParts.join(' \u00b7 ')}
                  </div>
                </div>

                {/* Score badge */}
                {hit.score !== undefined && (
                  <span className="tag a" style={{ fontFamily: "'JetBrains Mono', monospace" }}>
                    score {hit.score.toFixed(2)}
                  </span>
                )}
              </div>
            );
          })}
        </div>
      </div>

      {/* Facets sidebar panel */}
      <div className="panel">
        <div className="panel-h">
          <h3>Facets</h3>
          <span className="meta">this query</span>
        </div>
        <div className="panel-body">
          <dl className="kvs">
            {facets?.case && (
              <>
                <dt>Case</dt>
                <dd>
                  {Object.entries(facets.case)
                    .map(([name, count]) => `${name} \u00b7 ${count}`)
                    .join(', ')}
                </dd>
              </>
            )}
            {facets?.mime_type && (
              <>
                <dt>Kind</dt>
                <dd>
                  {Object.entries(facets.mime_type)
                    .map(([type, count]) => `${mimeLabel(type)} ${count}`)
                    .join(' \u00b7 ')}
                </dd>
              </>
            )}
            {facets?.classification && (
              <>
                <dt>Classification</dt>
                <dd>
                  {Object.entries(facets.classification)
                    .map(([cls, count]) => `${cls} ${count}`)
                    .join(' \u00b7 ')}
                </dd>
              </>
            )}
            {facets?.language && (
              <>
                <dt>Language</dt>
                <dd>
                  {Object.entries(facets.language)
                    .map(([lang, count]) => `${lang.toUpperCase()} ${count}`)
                    .join(' \u00b7 ')}
                </dd>
              </>
            )}
            <dt>Model</dt>
            <dd>
              <code>bge-m3</code> &middot; on-box
            </dd>
            <dt>Query ledger</dt>
            <dd>Signed &middot; written to audit chain</dd>
          </dl>
          <div
            style={{
              marginTop: 18,
              paddingTop: 18,
              borderTop: '1px solid var(--line)',
              fontSize: 12.5,
              color: 'var(--muted)',
              lineHeight: 1.55,
            }}
          >
            Your queries are written to the case audit log and are visible to defence counsel on
            disclosure. No query ever leaves this instance.
          </div>
        </div>
      </div>
    </div>
  );
}

function LoadingState() {
  return (
    <div className="g2-wide">
      <div className="panel">
        <div className="panel-h">
          <h3>Results</h3>
          <span className="meta">loading&hellip;</span>
        </div>
        <div className="panel-body" style={{ padding: 0 }}>
          {Array.from({ length: 5 }).map((_, i) => (
            <div
              key={i}
              className="animate-pulse"
              style={{
                padding: '16px 22px',
                borderBottom: '1px solid var(--line)',
              }}
            >
              <div
                style={{
                  height: 18,
                  borderRadius: 4,
                  width: '60%',
                  background: 'var(--bg-2)',
                  marginBottom: 8,
                }}
              />
              <div
                style={{
                  height: 14,
                  borderRadius: 4,
                  width: '90%',
                  background: 'var(--bg-2)',
                  marginBottom: 6,
                }}
              />
              <div
                style={{
                  height: 10,
                  borderRadius: 4,
                  width: '40%',
                  background: 'var(--bg-2)',
                }}
              />
            </div>
          ))}
        </div>
      </div>
      <div className="panel">
        <div className="panel-h">
          <h3>Facets</h3>
          <span className="meta">loading&hellip;</span>
        </div>
        <div className="panel-body animate-pulse">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} style={{ display: 'flex', gap: 18, marginBottom: 12 }}>
              <div style={{ height: 12, width: 80, borderRadius: 4, background: 'var(--bg-2)' }} />
              <div style={{ height: 12, flex: 1, borderRadius: 4, background: 'var(--bg-2)' }} />
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

function SearchEmpty({ query }: { query: string }) {
  const t = useTranslations('search');

  return (
    <div className="g2-wide">
      <div className="panel">
        <div className="panel-h">
          <h3>Results</h3>
          <span className="meta">no matches</span>
        </div>
        <div
          className="panel-body"
          style={{
            textAlign: 'center',
            color: 'var(--muted)',
            padding: '48px 22px',
          }}
        >
          <div
            style={{
              width: 48,
              height: 48,
              borderRadius: '50%',
              background: 'var(--bg-2)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              margin: '0 auto 16px',
              color: 'var(--muted)',
            }}
          >
            <SearchIcon size={18} />
          </div>
          <div
            style={{
              fontFamily: "'Fraunces', serif",
              fontSize: 16,
              color: 'var(--ink)',
              marginBottom: 8,
            }}
          >
            {t('noResults')}
          </div>
          <p style={{ maxWidth: 420, margin: '0 auto', lineHeight: 1.5, fontSize: 13.5 }}>
            {t('noResultsHint', { query })}
          </p>
        </div>
      </div>
      <div className="panel">
        <div className="panel-h">
          <h3>Facets</h3>
          <span className="meta">this query</span>
        </div>
        <div className="panel-body">
          <p style={{ color: 'var(--muted)', fontSize: 13 }}>No facets available.</p>
        </div>
      </div>
    </div>
  );
}
