'use client';

import { useTranslations } from 'next-intl';

export interface SearchFilterValues {
  caseId?: string;
  mimeTypes: string[];
  classification?: string;
  dateFrom?: string;
  dateTo?: string;
  tags: string[];
}

const FILE_TYPE_GROUPS = [
  { key: 'documents', label: 'Documents', types: ['application/pdf', 'application/msword', 'application/vnd.openxmlformats-officedocument.wordprocessingml.document'] },
  { key: 'images', label: 'Images', types: ['image/'] },
  { key: 'video', label: 'Video', types: ['video/'] },
  { key: 'audio', label: 'Audio', types: ['audio/'] },
] as const;

const CLASSIFICATIONS = ['public', 'restricted', 'confidential', 'ex_parte'] as const;

export function SearchFilters({
  filters,
  cases,
  onFilterChange,
}: {
  filters: SearchFilterValues;
  cases: { id: string; title: string; reference_code: string }[];
  onFilterChange: (filters: SearchFilterValues) => void;
}) {
  const t = useTranslations('search');
  const tEvidence = useTranslations('evidence');

  const activeCount =
    (filters.caseId ? 1 : 0) +
    filters.mimeTypes.length +
    (filters.classification ? 1 : 0) +
    (filters.dateFrom || filters.dateTo ? 1 : 0) +
    filters.tags.length;

  return (
    <aside className="space-y-[var(--space-lg)]">
      <div className="flex items-center justify-between">
        <h3
          className="text-xs uppercase tracking-widest font-semibold"
          style={{ color: 'var(--text-tertiary)' }}
        >
          {t('filters')}
        </h3>
        {activeCount > 0 && (
          <button
            type="button"
            className="text-xs link-accent"
            onClick={() =>
              onFilterChange({
                mimeTypes: [],
                tags: [],
              })
            }
          >
            {t('clearAll')} ({activeCount})
          </button>
        )}
      </div>

      {/* Case filter */}
      {cases.length > 0 && (
        <FilterSection title={t('filterCase')}>
          <select
            value={filters.caseId || ''}
            onChange={(e) =>
              onFilterChange({ ...filters, caseId: e.target.value || undefined })
            }
            className="input-field text-sm"
            style={{ height: '2.25rem' }}
          >
            <option value="">{t('allCases')}</option>
            {cases.map((c) => (
              <option key={c.id} value={c.id}>
                {c.reference_code}
              </option>
            ))}
          </select>
        </FilterSection>
      )}

      {/* File type filter */}
      <FilterSection title={t('filterType')}>
        <div className="space-y-1">
          {FILE_TYPE_GROUPS.map(({ key, types }) => {
            const isActive = types.some((t) =>
              filters.mimeTypes.some((mt) => mt.includes(t))
            );
            return (
              <label key={key} className="flex items-center gap-[var(--space-sm)] cursor-pointer group">
                <input
                  type="checkbox"
                  checked={isActive}
                  onChange={(e) => {
                    const next = e.target.checked
                      ? [...filters.mimeTypes, ...types]
                      : filters.mimeTypes.filter((mt) => !types.some((t) => mt.includes(t)));
                    onFilterChange({ ...filters, mimeTypes: next });
                  }}
                  className="accent-[var(--amber-accent)]"
                />
                <span
                  className="text-sm group-hover:text-[var(--text-primary)]"
                  style={{ color: 'var(--text-secondary)' }}
                >
                  {t(`fileType.${key}`)}
                </span>
              </label>
            );
          })}
        </div>
      </FilterSection>

      {/* Classification filter */}
      <FilterSection title={tEvidence('classification')}>
        <div className="space-y-1">
          {CLASSIFICATIONS.map((cls) => (
            <label key={cls} className="flex items-center gap-[var(--space-sm)] cursor-pointer group">
              <input
                type="radio"
                name="classification"
                checked={filters.classification === cls}
                onChange={() =>
                  onFilterChange({
                    ...filters,
                    classification: filters.classification === cls ? undefined : cls,
                  })
                }
                className="accent-[var(--amber-accent)]"
              />
              <span
                className="text-sm group-hover:text-[var(--text-primary)] capitalize"
                style={{ color: 'var(--text-secondary)' }}
              >
                {tEvidence(cls === 'ex_parte' ? 'exParte' : cls)}
              </span>
            </label>
          ))}
        </div>
      </FilterSection>

      {/* Date range */}
      <FilterSection title={t('filterDateRange')}>
        <div className="space-y-[var(--space-xs)]">
          <input
            type="date"
            value={filters.dateFrom || ''}
            onChange={(e) =>
              onFilterChange({ ...filters, dateFrom: e.target.value || undefined })
            }
            className="input-field text-sm"
            style={{ height: '2.25rem' }}
            aria-label={t('dateFrom')}
          />
          <input
            type="date"
            value={filters.dateTo || ''}
            onChange={(e) =>
              onFilterChange({ ...filters, dateTo: e.target.value || undefined })
            }
            className="input-field text-sm"
            style={{ height: '2.25rem' }}
            aria-label={t('dateTo')}
          />
        </div>
      </FilterSection>
    </aside>
  );
}

function FilterSection({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div>
      <p
        className="text-xs font-semibold uppercase tracking-wider mb-[var(--space-xs)]"
        style={{ color: 'var(--text-tertiary)' }}
      >
        {title}
      </p>
      {children}
    </div>
  );
}
