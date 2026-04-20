'use client';

import { useTranslations } from 'next-intl';

export interface SearchFilterValues {
  caseId?: string;
  mimeTypes: string[];
  classification?: string;
  dateFrom?: string;
  dateTo?: string;
  tags: string[];
  platform?: string;
  verificationStatus?: string;
  captureDateFrom?: string;
  captureDateTo?: string;
}

interface FilterChipDef {
  key: string;
  label: string;
  count?: number;
}

export function SearchFilters({
  filters,
  facets,
  totalHits,
  onFilterChange,
}: {
  filters: SearchFilterValues;
  facets?: Record<string, Record<string, number>>;
  totalHits?: number;
  onFilterChange: (filters: SearchFilterValues) => void;
}) {
  const t = useTranslations('search');

  // Derive counts from facets
  const evidenceCount = facets?.kind?.evidence ?? facets?.mime_type
    ? Object.values(facets.mime_type || {}).reduce((s, n) => s + n, 0)
    : undefined;
  const witnessCount = facets?.kind?.witness;
  const noteCount = facets?.kind?.note;
  const inquiryCount = facets?.kind?.inquiry;

  const chips: FilterChipDef[] = [
    { key: 'all', label: t('filterAll'), count: totalHits },
    { key: 'evidence', label: t('filterEvidence'), count: evidenceCount },
    { key: 'witnesses', label: t('filterWitnesses'), count: witnessCount },
    { key: 'notes', label: t('filterNotes'), count: noteCount },
    { key: 'inquiry', label: t('filterInquiry'), count: inquiryCount },
  ];

  const activeChip = filters.mimeTypes.length === 0 ? 'all' : filters.mimeTypes[0];

  const handleChipClick = (key: string) => {
    if (key === 'all') {
      onFilterChange({ ...filters, mimeTypes: [] });
    } else {
      onFilterChange({ ...filters, mimeTypes: [key] });
    }
  };

  // Date range chip label
  const dateLabel = filters.dateFrom || filters.dateTo
    ? `${filters.dateFrom || '\u2026'} \u2013 ${filters.dateTo || '\u2026'}`
    : t('filterDateRange');

  return (
    <div className="flex gap-[8px] flex-wrap items-center">
      {chips.map((chip) => {
        const isActive = chip.key === activeChip;
        return (
          <button
            key={chip.key}
            type="button"
            onClick={() => handleChipClick(chip.key)}
            className={`chip${isActive ? ' active' : ''}`}
          >
            {chip.label}
            {chip.count !== undefined && chip.count > 0 && ` ${chip.count}`}
          </button>
        );
      })}

      {/* Right-aligned utility chips */}
      <span className="ml-auto flex gap-[8px]">
        <button
          type="button"
          className="chip"
          onClick={() => {
            // Toggle language dropdown (placeholder for future implementation)
          }}
        >
          Lang &middot; any &#9662;
        </button>
        <button
          type="button"
          className={`chip${filters.dateFrom || filters.dateTo ? ' active' : ''}`}
          onClick={() => {
            if (filters.dateFrom || filters.dateTo) {
              onFilterChange({ ...filters, dateFrom: undefined, dateTo: undefined });
            }
          }}
        >
          {dateLabel} &#9662;
        </button>
      </span>
    </div>
  );
}
