'use client';

import { useState } from 'react';
import { useTranslations } from 'next-intl';
import { ChevronDown, FileText, ImageIcon, Film, Music, Shield, Calendar, Folder, Tag, Plus, type LucideIcon } from 'lucide-react';

export interface SearchFilterValues {
  caseId?: string;
  mimeTypes: string[];
  classification?: string;
  dateFrom?: string;
  dateTo?: string;
  tags: string[];
}

const FILE_TYPE_GROUPS = [
  {
    key: 'documents',
    types: ['application/pdf', 'application/msword', 'application/vnd.openxmlformats-officedocument.wordprocessingml.document'],
    Icon: FileText,
  },
  { key: 'images', types: ['image/'], Icon: ImageIcon },
  { key: 'video', types: ['video/'], Icon: Film },
  { key: 'audio', types: ['audio/'], Icon: Music },
] as const;

const CLASSIFICATIONS = [
  { key: 'public', color: 'var(--status-active)', bg: 'var(--status-active-bg)' },
  { key: 'restricted', color: 'var(--status-hold)', bg: 'var(--status-hold-bg)' },
  { key: 'confidential', color: 'var(--status-closed)', bg: 'var(--status-closed-bg)' },
  { key: 'ex_parte', color: 'var(--amber-accent)', bg: 'var(--amber-subtle)' },
] as const;

export function SearchFilters({
  filters,
  cases,
  facets,
  onFilterChange,
}: {
  filters: SearchFilterValues;
  cases: { id: string; title: string; reference_code: string }[];
  facets?: Record<string, Record<string, number>>;
  onFilterChange: (filters: SearchFilterValues) => void;
}) {
  const t = useTranslations('search');
  const tEvidence = useTranslations('evidence');
  const [tagDraft, setTagDraft] = useState('');

  const activeCount =
    (filters.caseId ? 1 : 0) +
    filters.mimeTypes.length +
    (filters.classification ? 1 : 0) +
    (filters.dateFrom || filters.dateTo ? 1 : 0) +
    filters.tags.length;

  const addTag = () => {
    const clean = tagDraft.trim().replace(/^#/, '');
    if (!clean || filters.tags.includes(clean)) return;
    onFilterChange({ ...filters, tags: [...filters.tags, clean] });
    setTagDraft('');
  };

  return (
    <aside
      className="rounded-[var(--radius-lg)] overflow-hidden"
      style={{
        backgroundColor: 'var(--bg-elevated)',
        border: '1px solid var(--border-default)',
        boxShadow: 'var(--shadow-xs)',
      }}
    >
      {/* Header */}
      <div
        className="flex items-center justify-between px-[var(--space-md)] py-[var(--space-sm)]"
        style={{
          backgroundColor: 'var(--bg-inset)',
          borderBottom: '1px solid var(--border-subtle)',
        }}
      >
        <div className="flex items-center gap-[var(--space-xs)]">
          <h3
            className="text-[11px] uppercase tracking-widest font-semibold"
            style={{ color: 'var(--text-secondary)' }}
          >
            {t('filters')}
          </h3>
          {activeCount > 0 && (
            <span
              className="inline-flex items-center justify-center text-[10px] font-mono font-semibold rounded-full"
              style={{
                minWidth: '1.125rem',
                height: '1.125rem',
                padding: '0 0.375rem',
                backgroundColor: 'var(--amber-accent)',
                color: 'var(--bg-base)',
              }}
            >
              {activeCount}
            </span>
          )}
        </div>
        {activeCount > 0 && (
          <button
            type="button"
            className="text-[11px] link-accent"
            onClick={() => onFilterChange({ mimeTypes: [], tags: [] })}
          >
            {t('reset')}
          </button>
        )}
      </div>

      <div className="max-h-[calc(100vh-14rem)] overflow-y-auto">
        {/* Case filter */}
        {cases.length > 0 && (
          <FilterSection title={t('filterCase')} Icon={Folder}>
            <select
              value={filters.caseId || ''}
              onChange={(e) =>
                onFilterChange({ ...filters, caseId: e.target.value || undefined })
              }
              className="input-field text-sm w-full"
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
        <FilterSection title={t('filterType')} Icon={FileText}>
          <div className="space-y-0.5">
            {FILE_TYPE_GROUPS.map(({ key, types, Icon }) => {
              const isActive = types.some((type) =>
                filters.mimeTypes.some((mt) => mt.includes(type))
              );
              const facetCount = facets?.mime_type
                ? Object.entries(facets.mime_type)
                    .filter(([mt]) => types.some((type) => mt.includes(type)))
                    .reduce((sum, [, n]) => sum + n, 0)
                : undefined;

              return (
                <button
                  key={key}
                  type="button"
                  onClick={() => {
                    const next = isActive
                      ? filters.mimeTypes.filter((mt) => !types.some((type) => mt.includes(type)))
                      : [...filters.mimeTypes, ...types];
                    onFilterChange({ ...filters, mimeTypes: next });
                  }}
                  className="w-full flex items-center justify-between gap-[var(--space-sm)] px-[var(--space-sm)] py-1.5 rounded-[var(--radius-sm)] text-left transition-colors"
                  style={{
                    backgroundColor: isActive ? 'var(--amber-subtle)' : 'transparent',
                    color: isActive ? 'var(--amber-accent)' : 'var(--text-secondary)',
                    border: `1px solid ${isActive ? 'var(--amber-accent)' : 'transparent'}`,
                  }}
                >
                  <span className="flex items-center gap-[var(--space-sm)]">
                    <Icon size={14} />
                    <span className="text-sm">{t(`fileType.${key}`)}</span>
                  </span>
                  {facetCount !== undefined && facetCount > 0 && (
                    <span
                      className="text-[10px] font-mono"
                      style={{ color: 'var(--text-tertiary)' }}
                    >
                      {facetCount}
                    </span>
                  )}
                </button>
              );
            })}
          </div>
        </FilterSection>

        {/* Classification filter */}
        <FilterSection title={tEvidence('classification')} Icon={Shield}>
          <div className="grid grid-cols-2 gap-1">
            {CLASSIFICATIONS.map(({ key, color, bg }) => {
              const isActive = filters.classification === key;
              const label = tEvidence(key === 'ex_parte' ? 'exParte' : key);
              const facetCount = facets?.classification?.[key];

              return (
                <button
                  key={key}
                  type="button"
                  onClick={() =>
                    onFilterChange({
                      ...filters,
                      classification: isActive ? undefined : key,
                    })
                  }
                  className="px-[var(--space-sm)] py-1.5 rounded-[var(--radius-sm)] text-[11px] font-medium uppercase tracking-wider transition-all"
                  style={{
                    backgroundColor: isActive ? bg : 'var(--bg-inset)',
                    color: isActive ? color : 'var(--text-tertiary)',
                    border: `1px solid ${isActive ? color : 'var(--border-subtle)'}`,
                  }}
                >
                  <div className="truncate">{label}</div>
                  {facetCount !== undefined && facetCount > 0 && (
                    <div
                      className="text-[9px] font-mono mt-0.5"
                      style={{ color: isActive ? color : 'var(--text-tertiary)', opacity: 0.8 }}
                    >
                      {facetCount}
                    </div>
                  )}
                </button>
              );
            })}
          </div>
        </FilterSection>

        {/* Date range */}
        <FilterSection title={t('filterDateRange')} Icon={Calendar}>
          <div className="space-y-[var(--space-xs)]">
            <div>
              <label
                className="text-[10px] uppercase tracking-wider"
                style={{ color: 'var(--text-tertiary)' }}
              >
                {t('dateFrom')}
              </label>
              <input
                type="date"
                value={filters.dateFrom || ''}
                onChange={(e) =>
                  onFilterChange({ ...filters, dateFrom: e.target.value || undefined })
                }
                className="input-field text-sm w-full mt-0.5"
                aria-label={t('dateFrom')}
              />
            </div>
            <div>
              <label
                className="text-[10px] uppercase tracking-wider"
                style={{ color: 'var(--text-tertiary)' }}
              >
                {t('dateTo')}
              </label>
              <input
                type="date"
                value={filters.dateTo || ''}
                onChange={(e) =>
                  onFilterChange({ ...filters, dateTo: e.target.value || undefined })
                }
                className="input-field text-sm w-full mt-0.5"
                aria-label={t('dateTo')}
              />
            </div>
          </div>
        </FilterSection>

        {/* Tags */}
        <FilterSection title={t('filterTags')} Icon={Tag}>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              addTag();
            }}
            className="flex gap-1"
          >
            <input
              type="text"
              value={tagDraft}
              onChange={(e) => setTagDraft(e.target.value)}
              placeholder={t('tagPlaceholder')}
              className="input-field text-sm flex-1 min-w-0"
              aria-label={t('filterTags')}
            />
            <button
              type="submit"
              className="btn-secondary shrink-0"
              style={{ padding: '0 0.625rem' }}
              aria-label={t('addTag')}
              disabled={!tagDraft.trim()}
            >
              <Plus size={14} />
            </button>
          </form>
          {filters.tags.length > 0 && (
            <div className="flex flex-wrap gap-1 mt-[var(--space-xs)]">
              {filters.tags.map((tag) => (
                <button
                  key={tag}
                  type="button"
                  onClick={() =>
                    onFilterChange({
                      ...filters,
                      tags: filters.tags.filter((x) => x !== tag),
                    })
                  }
                  className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[11px] font-mono"
                  style={{
                    backgroundColor: 'var(--amber-subtle)',
                    color: 'var(--amber-accent)',
                    border: '1px solid var(--amber-accent)',
                  }}
                >
                  #{tag}
                  <span style={{ opacity: 0.6 }}>×</span>
                </button>
              ))}
            </div>
          )}
        </FilterSection>
      </div>
    </aside>
  );
}

function FilterSection({
  title,
  Icon,
  children,
}: {
  title: string;
  Icon?: LucideIcon;
  children: React.ReactNode;
}) {
  const [open, setOpen] = useState(true);
  return (
    <div
      className="px-[var(--space-md)] py-[var(--space-sm)]"
      style={{ borderBottom: '1px solid var(--border-subtle)' }}
    >
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="w-full flex items-center justify-between mb-[var(--space-xs)] group"
      >
        <span className="flex items-center gap-[var(--space-xs)]">
          {Icon && <Icon size={12} />}
          <span
            className="text-[10px] font-semibold uppercase tracking-widest"
            style={{ color: 'var(--text-tertiary)' }}
          >
            {title}
          </span>
        </span>
        <ChevronDown
          size={12}
          style={{
            color: 'var(--text-tertiary)',
            transform: open ? 'rotate(0deg)' : 'rotate(-90deg)',
            transition: 'transform var(--duration-normal) var(--ease-out-expo)',
          }}
        />
      </button>
      {open && children}
    </div>
  );
}
