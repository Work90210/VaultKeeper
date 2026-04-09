'use client';

import { useState, useCallback, useEffect } from 'react';
import { useSearchParams, useRouter } from 'next/navigation';
import { useSession } from 'next-auth/react';
import { useLocale, useTranslations } from 'next-intl';
import { Shell } from '@/components/layout/shell';
import { SearchBar } from '@/components/search/search-bar';
import { SearchResults } from '@/components/search/search-results';
import { SearchFilters, type SearchFilterValues } from '@/components/search/search-filters';
import { searchEvidence, type SearchHit } from '@/lib/search-api';
import { useDebounce } from '@/hooks/use-debounce';
import type { Case } from '@/types';

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
  const [isLoading, setIsLoading] = useState(false);
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
      setIsLoading(true);
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
        if (result.data) {
          setHits(result.data.hits);
          setTotalHits(result.data.total_hits);
          setProcessingTimeMs(result.data.processing_time_ms);
        }
      } finally {
        setIsLoading(false);
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

  return (
    <Shell>
      <div className="max-w-6xl mx-auto px-[var(--space-lg)] py-[var(--space-lg)]">
        <h1
          className="font-[family-name:var(--font-heading)] text-xl mb-[var(--space-lg)]"
          style={{ color: 'var(--text-primary)' }}
        >
          {t('title')}
        </h1>

        <div className="mb-[var(--space-lg)]">
          <SearchBar
            defaultValue={initialQuery}
            onSearch={(q) => {
              setQuery(q);
              setOffset(0);
            }}
          />
        </div>

        <div className="flex flex-col lg:flex-row gap-[var(--space-lg)]">
          {/* Filters sidebar */}
          <div className="lg:w-64 shrink-0">
            <SearchFilters
              filters={filters}
              cases={cases}
              onFilterChange={handleFilterChange}
            />
          </div>

          {/* Results */}
          <div className="flex-1 min-w-0">
            <SearchResults
              hits={hits}
              totalHits={totalHits}
              processingTimeMs={processingTimeMs}
              query={debouncedQuery}
              isLoading={isLoading}
            />

            {/* Pagination */}
            {totalHits > 50 && (
              <div className="flex justify-center gap-[var(--space-sm)] mt-[var(--space-lg)]">
                <button
                  type="button"
                  className="btn-secondary"
                  disabled={offset === 0}
                  onClick={() => setOffset(Math.max(0, offset - 50))}
                >
                  {t('previous')}
                </button>
                <span className="text-xs self-center" style={{ color: 'var(--text-tertiary)' }}>
                  {offset + 1}–{Math.min(offset + 50, totalHits)} / {totalHits}
                </span>
                <button
                  type="button"
                  className="btn-secondary"
                  disabled={offset + 50 >= totalHits}
                  onClick={() => setOffset(offset + 50)}
                >
                  {t('next')}
                </button>
              </div>
            )}
          </div>
        </div>
      </div>
    </Shell>
  );
}
