'use client';

import { useRouter } from 'next/navigation';
import { useLocale, useTranslations } from 'next-intl';
import { FileText, Image as ImageIcon, Film, Music, File as FileIcon, ArrowUpRight, Search as SearchIcon, type LucideIcon } from 'lucide-react';
import type { SearchHit } from '@/lib/search-api';
import { mimeLabel } from '@/lib/evidence-utils';

const CLASSIFICATION_STYLES: Record<string, { color: string; bg: string }> = {
  public: { color: 'var(--status-active)', bg: 'var(--status-active-bg)' },
  restricted: { color: 'var(--status-hold)', bg: 'var(--status-hold-bg)' },
  confidential: { color: 'var(--status-closed)', bg: 'var(--status-closed-bg)' },
  ex_parte: { color: 'var(--amber-accent)', bg: 'var(--amber-subtle)' },
};

function FileTypeTile({ mimeType }: { mimeType?: string }) {
  const type = mimeType || '';
  let Icon: LucideIcon = FileIcon;
  let color = 'var(--text-tertiary)';
  let bg = 'var(--bg-inset)';

  if (type.startsWith('image/')) {
    Icon = ImageIcon;
    color = 'var(--status-active)';
    bg = 'var(--status-active-bg)';
  } else if (type.startsWith('video/')) {
    Icon = Film;
    color = 'var(--status-hold)';
    bg = 'var(--status-hold-bg)';
  } else if (type.startsWith('audio/')) {
    Icon = Music;
    color = 'var(--amber-accent)';
    bg = 'var(--amber-subtle)';
  } else if (type.includes('pdf') || type.includes('word') || type.includes('text')) {
    Icon = FileText;
    color = 'var(--status-closed)';
    bg = 'var(--status-closed-bg)';
  }

  return (
    <div
      className="flex items-center justify-center rounded-[var(--radius-md)] shrink-0"
      style={{
        width: '2.5rem',
        height: '2.5rem',
        backgroundColor: bg,
        color,
      }}
    >
      <Icon size={16} />
    </div>
  );
}

function HighlightedText({ html }: { html: string }) {
  return (
    <span
      dangerouslySetInnerHTML={{ __html: html }}
      className="[&_em]:font-semibold [&_em]:not-italic [&_em]:text-[color:var(--amber-accent)] [&_em]:bg-[color:var(--amber-subtle)] [&_em]:px-0.5 [&_em]:rounded-sm"
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

  if (isLoading && hits.length === 0) {
    return <LoadingState />;
  }

  if (!isLoading && hits.length === 0 && query) {
    return <SearchEmpty query={query} />;
  }

  return (
    <div className={isLoading ? 'opacity-60 pointer-events-none transition-opacity' : ''}>
      {/* Results meta */}
      {query && (
        <div
          className="flex items-baseline justify-between mb-[var(--space-md)] pb-[var(--space-sm)]"
          style={{ borderBottom: '1px solid var(--border-subtle)' }}
        >
          <p className="text-xs font-mono" style={{ color: 'var(--text-tertiary)' }}>
            {t('resultsCount', { count: totalHits, time: processingTimeMs })}
          </p>
          <p className="text-[10px] uppercase tracking-widest font-semibold" style={{ color: 'var(--text-tertiary)' }}>
            {t('sortRelevance')}
          </p>
        </div>
      )}

      {/* Result rows */}
      <ul className="space-y-[var(--space-xs)] stagger-in">
        {hits.map((hit) => {
          const titleHighlight = hit.highlights?.title?.[0];
          const descHighlight = hit.highlights?.description?.[0];
          const clsStyle = hit.classification
            ? CLASSIFICATION_STYLES[hit.classification] || CLASSIFICATION_STYLES.restricted
            : null;

          return (
            <li key={hit.evidence_id}>
              <button
                type="button"
                onClick={() => router.push(`/${locale}/evidence/${hit.evidence_id}`)}
                className="w-full text-left p-[var(--space-md)] rounded-[var(--radius-md)] transition-all group"
                style={{
                  backgroundColor: 'var(--bg-elevated)',
                  border: '1px solid var(--border-default)',
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.borderColor = 'var(--amber-accent)';
                  e.currentTarget.style.boxShadow = 'var(--shadow-md)';
                  e.currentTarget.style.transform = 'translateY(-1px)';
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.borderColor = 'var(--border-default)';
                  e.currentTarget.style.boxShadow = '';
                  e.currentTarget.style.transform = '';
                }}
              >
                <div className="flex items-start gap-[var(--space-md)]">
                  <FileTypeTile mimeType={hit.mime_type} />

                  <div className="flex-1 min-w-0">
                    {/* Title row */}
                    <div className="flex items-center justify-between gap-[var(--space-sm)] mb-1">
                      <h3
                        className="text-sm font-semibold truncate font-[family-name:var(--font-heading)]"
                        style={{ color: 'var(--text-primary)' }}
                      >
                        {titleHighlight ? (
                          <HighlightedText html={titleHighlight} />
                        ) : (
                          hit.title || hit.file_name || t('untitled')
                        )}
                      </h3>
                      <ArrowUpRight
                        size={14}
                        className="shrink-0 opacity-0 group-hover:opacity-100 transition-opacity"
                        style={{ color: 'var(--amber-accent)' }}
                      />
                    </div>

                    {/* Description */}
                    {(descHighlight || hit.description) && (
                      <p
                        className="text-xs mb-[var(--space-sm)] line-clamp-2 leading-relaxed"
                        style={{ color: 'var(--text-secondary)' }}
                      >
                        {descHighlight ? (
                          <HighlightedText html={descHighlight} />
                        ) : (
                          hit.description
                        )}
                      </p>
                    )}

                    {/* Meta row */}
                    <div className="flex items-center flex-wrap gap-[var(--space-md)] text-[10px]">
                      {hit.evidence_number && (
                        <MetaItem
                          label={t('evidenceNumber')}
                          value={hit.evidence_number}
                          mono
                        />
                      )}
                      {hit.mime_type && (
                        <MetaItem
                          label={t('type')}
                          value={mimeLabel(hit.mime_type)}
                        />
                      )}
                      {clsStyle && hit.classification && (
                        <span
                          className="inline-flex items-center px-1.5 py-0.5 rounded text-[9px] font-semibold uppercase tracking-widest"
                          style={{ backgroundColor: clsStyle.bg, color: clsStyle.color }}
                        >
                          {hit.classification.replace('_', ' ')}
                        </span>
                      )}
                      {hit.uploaded_at && (
                        <MetaItem
                          label={t('uploaded')}
                          value={new Date(hit.uploaded_at).toLocaleDateString('en-GB', {
                            day: '2-digit',
                            month: 'short',
                            year: 'numeric',
                          })}
                        />
                      )}
                    </div>
                  </div>
                </div>
              </button>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

function MetaItem({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <span className="flex items-center gap-1.5">
      <span
        className="uppercase tracking-widest font-semibold"
        style={{ color: 'var(--text-tertiary)' }}
      >
        {label}
      </span>
      <span
        className={mono ? 'font-mono' : ''}
        style={{ color: 'var(--text-secondary)' }}
      >
        {value}
      </span>
    </span>
  );
}

function LoadingState() {
  return (
    <div className="space-y-[var(--space-xs)]">
      {Array.from({ length: 5 }).map((_, i) => (
        <div
          key={i}
          className="p-[var(--space-md)] rounded-[var(--radius-md)] animate-pulse"
          style={{
            backgroundColor: 'var(--bg-elevated)',
            border: '1px solid var(--border-subtle)',
          }}
        >
          <div className="flex items-start gap-[var(--space-md)]">
            <div
              className="rounded-[var(--radius-md)] shrink-0"
              style={{
                width: '2.5rem',
                height: '2.5rem',
                backgroundColor: 'var(--bg-inset)',
              }}
            />
            <div className="flex-1 space-y-2">
              <div className="h-4 rounded w-2/3" style={{ backgroundColor: 'var(--bg-inset)' }} />
              <div className="h-3 rounded w-full" style={{ backgroundColor: 'var(--bg-inset)' }} />
              <div className="flex gap-2">
                <div className="h-2 rounded w-16" style={{ backgroundColor: 'var(--bg-inset)' }} />
                <div className="h-2 rounded w-20" style={{ backgroundColor: 'var(--bg-inset)' }} />
                <div className="h-2 rounded w-14" style={{ backgroundColor: 'var(--bg-inset)' }} />
              </div>
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}

function SearchEmpty({ query }: { query: string }) {
  const t = useTranslations('search');

  return (
    <div
      className="card-inset text-center py-[var(--space-2xl)] px-[var(--space-lg)]"
      style={{ minHeight: '20rem', display: 'flex', flexDirection: 'column', justifyContent: 'center' }}
    >
      <div
        className="mx-auto mb-[var(--space-md)] flex items-center justify-center rounded-full"
        style={{
          width: '3rem',
          height: '3rem',
          backgroundColor: 'var(--bg-inset)',
          color: 'var(--text-tertiary)',
        }}
      >
        <SearchIcon size={18} />
      </div>
      <p
        className="text-base font-[family-name:var(--font-heading)]"
        style={{ color: 'var(--text-primary)' }}
      >
        {t('noResults')}
      </p>
      <p className="text-xs mt-1 max-w-sm mx-auto" style={{ color: 'var(--text-tertiary)' }}>
        {t('noResultsHint', { query })}
      </p>
    </div>
  );
}
