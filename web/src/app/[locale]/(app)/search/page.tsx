'use client';

import { useState, useCallback, useEffect, useMemo } from 'react';
import { useSearchParams, useRouter } from 'next/navigation';
import { useSession } from 'next-auth/react';
import { useLocale, useTranslations } from 'next-intl';
import { Search as SearchIcon, SlidersHorizontal, X } from 'lucide-react';
import { Shell } from '@/components/layout/shell';
import { SearchBar } from '@/components/search/search-bar';
import { SearchResults } from '@/components/search/search-results';
import { SearchFilters, type SearchFilterValues } from '@/components/search/search-filters';
import { searchEvidence, type SearchHit } from '@/lib/search-api';
import { useDebounce } from '@/hooks/use-debounce';
import type { Case } from '@/types';

type ViewState = 'idle' | 'loading' | 'ready' | 'error';

export default function SearchPage() {
  const t = useTranslations('search');
  const locale = useLocale();
  const { data: session } = useSession();
  const searchParams = useSearchParams();
  const router = useRouter();

  const initialQuery = searchParams.get('q') || '';

  const [query, setQuery] = useState(initialQuery);
  const [hits, setHits] = useState<SearchHit[]>([]);
  const [totalHits, setTotalHits] = useState(0);
  const [processingTimeMs, setProcessingTimeMs] = useState(0);
  const [facets, setFacets] = useState<Record<string, Record<string, number>> | undefined>();
  const [viewState, setViewState] = useState<ViewState>('idle');
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [cases, setCases] = useState<Case[]>([]);
  const [offset, setOffset] = useState(0);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [filters, setFilters] = useState<SearchFilterValues>({
    caseId: searchParams.get('case_id') || undefined,
    mimeTypes: searchParams.get('type')?.split(',').filter(Boolean) || [],
    classification: searchParams.get('classification') || undefined,
    dateFrom: searchParams.get('from') || undefined,
    dateTo: searchParams.get('to') || undefined,
    tags: searchParams.get('tag')?.split(',').filter(Boolean) || [],
  });

  const debouncedQuery = useDebounce(query, 300);

  useEffect(() => {
    if (!session?.accessToken) return;

    const apiBase = process.env.NEXT_PUBLIC_API_URL || '';
    fetch(`${apiBase}/api/cases?limit=200`, {
      headers: { Authorization: `Bearer ${session.accessToken}` },
    })
      .then((res) => (res.ok ? res.json() : null))
      .then((json) => {
        if (json?.data) {
          setCases(json.data);
        }
      })
      .catch(() => {
        // Silently fail — case filter will remain empty
      });
  }, [session?.accessToken]);

  const performSearch = useCallback(
    async (q: string, f: SearchFilterValues, page: number) => {
      if (!session?.accessToken) return;
      setViewState('loading');
      setErrorMessage(null);
      try {
        const result = await searchEvidence(
          {
            q: q || undefined,
            case_id: f.caseId,
            type: f.mimeTypes.join(',') || undefined,
            classification: f.classification,
            from: f.dateFrom,
            to: f.dateTo,
            tag: f.tags.join(',') || undefined,
            limit: 50,
            offset: page,
          },
          session.accessToken
        );
        if (result.error) {
          setViewState('error');
          setErrorMessage(result.error);
          setHits([]);
          setTotalHits(0);
          setFacets(undefined);
          return;
        }
        if (result.data) {
          setHits(result.data.hits);
          setTotalHits(result.data.total_hits);
          setProcessingTimeMs(result.data.processing_time_ms);
          setFacets(result.data.facets);
          setViewState('ready');
        }
      } catch (err) {
        setViewState('error');
        setErrorMessage(err instanceof Error ? err.message : 'Unknown error');
      }
    },
    [session?.accessToken]
  );

  useEffect(() => {
    performSearch(debouncedQuery, filters, offset);
  }, [debouncedQuery, filters, offset, performSearch]);

  // Sync URL params
  useEffect(() => {
    const params = new URLSearchParams();
    if (query) params.set('q', query);
    if (filters.caseId) params.set('case_id', filters.caseId);
    if (filters.mimeTypes.length) params.set('type', filters.mimeTypes.join(','));
    if (filters.classification) params.set('classification', filters.classification);
    if (filters.dateFrom) params.set('from', filters.dateFrom);
    if (filters.dateTo) params.set('to', filters.dateTo);
    if (filters.tags.length) params.set('tag', filters.tags.join(','));
    const paramStr = params.toString();
    const currentParams = searchParams.toString();
    if (paramStr !== currentParams) {
      router.replace(`/${locale}/search${paramStr ? `?${paramStr}` : ''}`, { scroll: false });
    }
  }, [query, filters, router, searchParams, locale]);

  const handleFilterChange = (newFilters: SearchFilterValues) => {
    setFilters(newFilters);
    setOffset(0);
  };

  const activeFilterCount = useMemo(
    () =>
      (filters.caseId ? 1 : 0) +
      filters.mimeTypes.length +
      (filters.classification ? 1 : 0) +
      (filters.dateFrom || filters.dateTo ? 1 : 0) +
      filters.tags.length,
    [filters]
  );

  const isIdle = viewState === 'idle' && !debouncedQuery && activeFilterCount === 0;

  return (
    <Shell>
      <div
        className="max-w-[1400px] mx-auto px-[var(--space-lg)] py-[var(--space-lg)]"
        style={{ animation: 'fade-in var(--duration-slow) var(--ease-out-expo)' }}
      >
        {/* ============ HEADER ============ */}
        <header className="mb-[var(--space-xl)]">
          <div className="flex items-center gap-[var(--space-xs)] mb-[var(--space-sm)]">
            <span
              className="text-[10px] uppercase tracking-widest font-semibold"
              style={{ color: 'var(--text-tertiary)' }}
            >
              VaultKeeper
            </span>
            <span style={{ color: 'var(--text-tertiary)' }}>·</span>
            <span
              className="text-[10px] uppercase tracking-widest font-semibold"
              style={{ color: 'var(--text-tertiary)' }}
            >
              {t('title')}
            </span>
          </div>

          <div className="flex items-end justify-between gap-[var(--space-md)] flex-wrap">
            <div>
              <h1
                className="font-[family-name:var(--font-heading)] text-3xl leading-tight"
                style={{ color: 'var(--text-primary)' }}
              >
                {t('title')}
              </h1>
              <p className="text-sm mt-1" style={{ color: 'var(--text-tertiary)' }}>
                {t('subtitle')}
              </p>
            </div>

            {/* Live status pill */}
            <StatusPill
              state={viewState}
              totalHits={totalHits}
              processingTimeMs={processingTimeMs}
              hasQuery={Boolean(debouncedQuery || activeFilterCount)}
            />
          </div>
        </header>

        {/* ============ SEARCH COMMAND BAR ============ */}
        <div
          className="mb-[var(--space-lg)] p-[var(--space-md)] rounded-[var(--radius-lg)]"
          style={{
            backgroundColor: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            boxShadow: 'var(--shadow-sm)',
          }}
        >
          <SearchBar
            defaultValue={initialQuery}
            onSearch={(q) => {
              setQuery(q);
              setOffset(0);
            }}
          />

          {/* Active filter chips */}
          {activeFilterCount > 0 && (
            <div className="flex items-center flex-wrap gap-[var(--space-xs)] mt-[var(--space-sm)] pt-[var(--space-sm)]"
              style={{ borderTop: '1px dashed var(--border-subtle)' }}
            >
              <span
                className="text-[10px] uppercase tracking-wider font-semibold mr-1"
                style={{ color: 'var(--text-tertiary)' }}
              >
                {t('activeFilters')}
              </span>
              {filters.caseId && (
                <FilterChip
                  label={cases.find((c) => c.id === filters.caseId)?.reference_code || filters.caseId}
                  onRemove={() => handleFilterChange({ ...filters, caseId: undefined })}
                />
              )}
              {filters.mimeTypes.map((mt) => (
                <FilterChip
                  key={mt}
                  label={mt.split('/')[0]}
                  onRemove={() =>
                    handleFilterChange({
                      ...filters,
                      mimeTypes: filters.mimeTypes.filter((m) => m !== mt),
                    })
                  }
                />
              ))}
              {filters.classification && (
                <FilterChip
                  label={filters.classification.replace('_', ' ')}
                  onRemove={() => handleFilterChange({ ...filters, classification: undefined })}
                />
              )}
              {(filters.dateFrom || filters.dateTo) && (
                <FilterChip
                  label={`${filters.dateFrom || '…'} → ${filters.dateTo || '…'}`}
                  onRemove={() =>
                    handleFilterChange({ ...filters, dateFrom: undefined, dateTo: undefined })
                  }
                />
              )}
              {filters.tags.map((tag) => (
                <FilterChip
                  key={tag}
                  label={`#${tag}`}
                  onRemove={() =>
                    handleFilterChange({
                      ...filters,
                      tags: filters.tags.filter((x) => x !== tag),
                    })
                  }
                />
              ))}
              <button
                type="button"
                className="text-[11px] link-accent ml-auto"
                onClick={() => handleFilterChange({ mimeTypes: [], tags: [] })}
              >
                {t('clearAll')}
              </button>
            </div>
          )}
        </div>

        {/* ============ MAIN LAYOUT ============ */}
        <div className="flex gap-[var(--space-lg)]">
          {/* Sidebar — desktop */}
          <aside className="hidden lg:block lg:w-72 shrink-0">
            <div className="sticky" style={{ top: 'calc(var(--space-lg) + 4rem)' }}>
              <SearchFilters
                filters={filters}
                cases={cases}
                facets={facets}
                onFilterChange={handleFilterChange}
              />
            </div>
          </aside>

          {/* Mobile filter button */}
          <button
            type="button"
            className="lg:hidden fixed bottom-[var(--space-lg)] right-[var(--space-lg)] z-40 btn-primary flex items-center gap-[var(--space-xs)] shadow-lg"
            onClick={() => setFiltersOpen(true)}
          >
            <SlidersHorizontal size={14} />
            {t('filters')}
            {activeFilterCount > 0 && (
              <span
                className="inline-flex items-center justify-center rounded-full text-[10px] font-mono ml-1"
                style={{
                  minWidth: '1.25rem',
                  height: '1.25rem',
                  backgroundColor: 'var(--amber-accent)',
                  color: 'var(--bg-base)',
                }}
              >
                {activeFilterCount}
              </span>
            )}
          </button>

          {/* Mobile drawer */}
          {filtersOpen && (
            <div
              className="lg:hidden fixed inset-0 z-50 flex justify-end"
              style={{ backgroundColor: 'rgba(0,0,0,0.4)' }}
              onClick={() => setFiltersOpen(false)}
            >
              <div
                className="w-80 max-w-full h-full overflow-y-auto p-[var(--space-lg)]"
                style={{ backgroundColor: 'var(--bg-elevated)' }}
                onClick={(e) => e.stopPropagation()}
              >
                <div className="flex items-center justify-between mb-[var(--space-lg)]">
                  <h3 className="font-[family-name:var(--font-heading)] text-lg" style={{ color: 'var(--text-primary)' }}>
                    {t('filters')}
                  </h3>
                  <button type="button" className="btn-ghost" onClick={() => setFiltersOpen(false)}>
                    <X size={16} />
                  </button>
                </div>
                <SearchFilters
                  filters={filters}
                  cases={cases}
                  facets={facets}
                  onFilterChange={handleFilterChange}
                />
              </div>
            </div>
          )}

          {/* Results pane */}
          <section className="flex-1 min-w-0">
            {viewState === 'error' && (
              <ErrorBanner message={errorMessage} onRetry={() => performSearch(debouncedQuery, filters, offset)} />
            )}

            {isIdle ? (
              <IdleState />
            ) : (
              <SearchResults
                hits={hits}
                totalHits={totalHits}
                processingTimeMs={processingTimeMs}
                query={debouncedQuery}
                isLoading={viewState === 'loading'}
              />
            )}

            {/* Pagination */}
            {totalHits > 50 && viewState !== 'error' && (
              <nav
                className="flex items-center justify-between mt-[var(--space-xl)] pt-[var(--space-md)]"
                style={{ borderTop: '1px solid var(--border-subtle)' }}
                aria-label="Pagination"
              >
                <button
                  type="button"
                  className="btn-secondary"
                  disabled={offset === 0}
                  onClick={() => setOffset(Math.max(0, offset - 50))}
                >
                  ← {t('previous')}
                </button>
                <span className="text-xs font-mono" style={{ color: 'var(--text-tertiary)' }}>
                  {offset + 1}–{Math.min(offset + 50, totalHits)} / {totalHits}
                </span>
                <button
                  type="button"
                  className="btn-secondary"
                  disabled={offset + 50 >= totalHits}
                  onClick={() => setOffset(offset + 50)}
                >
                  {t('next')} →
                </button>
              </nav>
            )}
          </section>
        </div>
      </div>
    </Shell>
  );
}

function StatusPill({
  state,
  totalHits,
  processingTimeMs,
  hasQuery,
}: {
  state: ViewState;
  totalHits: number;
  processingTimeMs: number;
  hasQuery: boolean;
}) {
  const t = useTranslations('search');

  let color = 'var(--text-tertiary)';
  let bg = 'var(--bg-inset)';
  let text: string = t('idle');

  if (state === 'loading') {
    color = 'var(--amber-accent)';
    bg = 'var(--amber-subtle)';
    text = t('searching');
  } else if (state === 'error') {
    color = 'var(--status-closed)';
    bg = 'var(--status-closed-bg)';
    text = t('errorLabel');
  } else if (state === 'ready' && hasQuery) {
    color = 'var(--status-active)';
    bg = 'var(--status-active-bg)';
    text = t('liveResults', { count: totalHits, time: processingTimeMs });
  }

  return (
    <div
      className="inline-flex items-center gap-[var(--space-xs)] px-[var(--space-sm)] py-1 rounded-full text-[11px] font-mono"
      style={{ backgroundColor: bg, color }}
    >
      <span
        className="inline-block w-1.5 h-1.5 rounded-full"
        style={{
          backgroundColor: color,
          animation: state === 'loading' ? 'pulse 1.4s ease-in-out infinite' : undefined,
        }}
      />
      {text}
    </div>
  );
}

function FilterChip({ label, onRemove }: { label: string; onRemove: () => void }) {
  return (
    <span
      className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[11px]"
      style={{
        backgroundColor: 'var(--bg-inset)',
        color: 'var(--text-secondary)',
        border: '1px solid var(--border-subtle)',
      }}
    >
      <span className="capitalize">{label}</span>
      <button
        type="button"
        onClick={onRemove}
        className="inline-flex items-center justify-center rounded-full p-0.5 transition-colors"
        style={{ color: 'var(--text-tertiary)' }}
        aria-label={`Remove ${label}`}
      >
        <X size={10} />
      </button>
    </span>
  );
}

function ErrorBanner({ message, onRetry }: { message: string | null; onRetry: () => void }) {
  const t = useTranslations('search');
  return (
    <div
      className="p-[var(--space-md)] rounded-[var(--radius-md)] mb-[var(--space-md)] flex items-start gap-[var(--space-sm)]"
      style={{
        backgroundColor: 'var(--status-closed-bg)',
        border: '1px solid var(--status-closed)',
      }}
    >
      <div className="flex-1">
        <p className="text-sm font-medium" style={{ color: 'var(--status-closed)' }}>
          {t('errorTitle')}
        </p>
        {message && (
          <p className="text-xs mt-1 font-mono" style={{ color: 'var(--text-secondary)' }}>
            {message}
          </p>
        )}
      </div>
      <button type="button" className="btn-secondary text-xs" onClick={onRetry}>
        {t('retry')}
      </button>
    </div>
  );
}

function IdleState() {
  const t = useTranslations('search');
  return (
    <div
      className="card-inset p-[var(--space-2xl)] text-center"
      style={{ minHeight: '24rem', display: 'flex', flexDirection: 'column', justifyContent: 'center' }}
    >
      <div
        className="mx-auto mb-[var(--space-md)] flex items-center justify-center rounded-full"
        style={{
          width: '3.5rem',
          height: '3.5rem',
          backgroundColor: 'var(--amber-subtle)',
          color: 'var(--amber-accent)',
        }}
      >
        <SearchIcon size={22} />
      </div>
      <h3
        className="font-[family-name:var(--font-heading)] text-lg"
        style={{ color: 'var(--text-primary)' }}
      >
        {t('idleTitle')}
      </h3>
      <p className="text-sm mt-1 max-w-md mx-auto" style={{ color: 'var(--text-tertiary)' }}>
        {t('idleHint')}
      </p>
      <div className="mt-[var(--space-lg)] flex flex-wrap justify-center gap-[var(--space-xs)] text-[11px]">
        {['title:', 'tag:', 'case:', 'from:', 'to:'].map((hint) => (
          <code
            key={hint}
            className="px-2 py-0.5 rounded font-mono"
            style={{
              backgroundColor: 'var(--bg-base)',
              color: 'var(--text-secondary)',
              border: '1px solid var(--border-subtle)',
            }}
          >
            {hint}
          </code>
        ))}
      </div>
    </div>
  );
}
