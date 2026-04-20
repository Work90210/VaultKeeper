'use client';

import { useState, useCallback, useEffect, useMemo } from 'react';
import { useSearchParams, useRouter } from 'next/navigation';
import { useSession } from 'next-auth/react';
import { useLocale, useTranslations } from 'next-intl';
import { Search as SearchIcon } from 'lucide-react';
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
    const orgMatch = document.cookie.match(/(?:^|; )vk-active-org=([^;]*)/);
    const headers: Record<string, string> = { Authorization: `Bearer ${session.accessToken}` };
    if (orgMatch) headers['X-Organization-ID'] = decodeURIComponent(orgMatch[1]);

    fetch(`${apiBase}/api/cases?limit=200`, { headers })
      .then((res) => (res.ok ? res.json() : null))
      .then((json) => {
        if (json?.data) {
          setCases(json.data);
        }
      })
      .catch(() => {
        // Silently fail -- case filter will remain empty
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
      {/* ============ PAGE HEAD ============ */}
      <section className="d-pagehead">
        <div>
          <span className="eyebrow-m">Cross-exhibit &middot; on-box models &middot; zero telemetry</span>
          <h1>
            Semantic <em>search</em>
          </h1>
          <p className="sub">
            Meilisearch for lexical, an on-box embedding model for semantic recall. Queries never
            leave the VaultKeeper instance. Results respect case-level ACLs row-by-row.
          </p>
        </div>
      </section>

      {/* ============ SEARCH PANEL ============ */}
      <div className="panel" style={{ marginBottom: 22 }}>
        <div className="panel-body" style={{ padding: 18 }}>
          <SearchBar
            defaultValue={initialQuery}
            onSearch={(q) => {
              setQuery(q);
              setOffset(0);
            }}
            processingTimeMs={viewState === 'ready' ? processingTimeMs : undefined}
            totalHits={viewState === 'ready' ? totalHits : undefined}
          />

          {/* Filter chips row */}
          <div style={{ marginTop: 14 }}>
            <SearchFilters
              filters={filters}
              facets={facets}
              totalHits={totalHits}
              onFilterChange={handleFilterChange}
            />
          </div>
        </div>
      </div>

      {/* ============ ERROR BANNER ============ */}
      {viewState === 'error' && (
        <ErrorBanner message={errorMessage} onRetry={() => performSearch(debouncedQuery, filters, offset)} />
      )}

      {/* ============ RESULTS ============ */}
      {isIdle ? (
        <IdleState />
      ) : (
        <SearchResults
          hits={hits}
          totalHits={totalHits}
          processingTimeMs={processingTimeMs}
          query={debouncedQuery}
          isLoading={viewState === 'loading'}
          facets={facets}
        />
      )}

      {/* ============ PAGINATION ============ */}
      {totalHits > 50 && viewState !== 'error' && (
        <nav
          className="flex items-center justify-between"
          style={{
            marginTop: 22,
            paddingTop: 18,
            borderTop: '1px solid var(--line)',
          }}
          aria-label="Pagination"
        >
          <button
            type="button"
            className="btn ghost"
            disabled={offset === 0}
            onClick={() => setOffset(Math.max(0, offset - 50))}
          >
            &larr; {t('previous')}
          </button>
          <span
            style={{
              fontFamily: "'JetBrains Mono', monospace",
              fontSize: 11,
              color: 'var(--muted)',
              letterSpacing: '.04em',
            }}
          >
            {offset + 1}&ndash;{Math.min(offset + 50, totalHits)} / {totalHits}
          </span>
          <button
            type="button"
            className="btn ghost"
            disabled={offset + 50 >= totalHits}
            onClick={() => setOffset(offset + 50)}
          >
            {t('next')} &rarr;
          </button>
        </nav>
      )}
    </Shell>
  );
}

function ErrorBanner({ message, onRetry }: { message: string | null; onRetry: () => void }) {
  const t = useTranslations('search');
  return (
    <div
      className="panel"
      style={{
        marginBottom: 22,
        borderColor: 'var(--accent)',
      }}
    >
      <div
        className="panel-body"
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 16,
        }}
      >
        <div>
          <div
            style={{
              fontFamily: "'Fraunces', serif",
              fontSize: 16,
              color: 'var(--ink)',
              marginBottom: 4,
            }}
          >
            {t('errorTitle')}
          </div>
          {message && (
            <div
              style={{
                fontFamily: "'JetBrains Mono', monospace",
                fontSize: 12,
                color: 'var(--muted)',
              }}
            >
              {message}
            </div>
          )}
        </div>
        <button type="button" className="btn ghost" onClick={onRetry}>
          {t('retry')}
        </button>
      </div>
    </div>
  );
}

function IdleState() {
  const t = useTranslations('search');
  return (
    <div className="panel">
      <div
        className="panel-body"
        style={{
          textAlign: 'center',
          padding: '48px 22px',
          minHeight: 300,
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'center',
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
          <SearchIcon size={20} />
        </div>
        <div
          style={{
            fontFamily: "'Fraunces', serif",
            fontSize: 18,
            color: 'var(--ink)',
            marginBottom: 8,
          }}
        >
          {t('idleTitle')}
        </div>
        <p
          style={{
            fontSize: 13.5,
            color: 'var(--muted)',
            maxWidth: 480,
            margin: '0 auto',
            lineHeight: 1.55,
          }}
        >
          {t('idleHint')}
        </p>
        <div
          style={{
            marginTop: 18,
            display: 'flex',
            flexWrap: 'wrap',
            justifyContent: 'center',
            gap: 8,
          }}
        >
          {['title:', 'tag:', 'case:', 'from:', 'to:'].map((hint) => (
            <code
              key={hint}
              style={{
                fontFamily: "'JetBrains Mono', monospace",
                fontSize: 11,
                padding: '3px 8px',
                borderRadius: 5,
                background: 'var(--bg-2)',
                color: 'var(--ink-2)',
                border: '1px solid var(--line)',
                letterSpacing: '.04em',
              }}
            >
              {hint}
            </code>
          ))}
        </div>
      </div>
    </div>
  );
}
