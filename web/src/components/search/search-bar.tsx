'use client';

import { useState, useRef, useEffect, useCallback } from 'react';
import { useRouter } from 'next/navigation';
import { useDebounce } from '@/hooks/use-debounce';
import { useLocale, useTranslations } from 'next-intl';

const RECENT_QUERIES_KEY = 'vk_recent_searches';
const MAX_RECENT = 5;

function getRecentQueries(): string[] {
  if (typeof window === 'undefined') return [];
  try {
    const raw = localStorage.getItem(RECENT_QUERIES_KEY);
    return raw ? JSON.parse(raw) : [];
  } catch {
    return [];
  }
}

function saveRecentQuery(query: string) {
  const recent = getRecentQueries().filter((q) => q !== query);
  recent.unshift(query);
  localStorage.setItem(
    RECENT_QUERIES_KEY,
    JSON.stringify(recent.slice(0, MAX_RECENT))
  );
}

export function SearchBar({
  defaultValue = '',
  compact = false,
  onSearch,
}: {
  defaultValue?: string;
  compact?: boolean;
  onSearch?: (query: string) => void;
}) {
  const t = useTranslations('search');
  const locale = useLocale();
  const router = useRouter();
  const inputRef = useRef<HTMLInputElement>(null);
  const [value, setValue] = useState(defaultValue);
  const [showSuggestions, setShowSuggestions] = useState(false);
  const [recentQueries, setRecentQueries] = useState<string[]>([]);
  const debouncedValue = useDebounce(value, 300);

  useEffect(() => {
    setRecentQueries(getRecentQueries());
  }, []);

  useEffect(() => {
    if (debouncedValue && onSearch) {
      onSearch(debouncedValue);
    }
  }, [debouncedValue, onSearch]);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        inputRef.current?.focus();
      }
    };
    document.addEventListener('keydown', handler);
    return () => document.removeEventListener('keydown', handler);
  }, []);

  const handleSubmit = useCallback(
    (e: React.FormEvent) => {
      e.preventDefault();
      const trimmed = value.trim();
      if (!trimmed) return;
      saveRecentQuery(trimmed);
      setShowSuggestions(false);
      if (onSearch) {
        onSearch(trimmed);
      } else {
        router.push(`/${locale}/search?q=${encodeURIComponent(trimmed)}`);
      }
    },
    [value, onSearch, router, locale]
  );

  const handleClear = () => {
    setValue('');
    inputRef.current?.focus();
    if (onSearch) onSearch('');
  };

  return (
    <form onSubmit={handleSubmit} className="relative">
      <div className="relative">
        {/* Search icon */}
        <svg
          className="absolute left-[var(--space-sm)] top-1/2 -translate-y-1/2 pointer-events-none"
          width="16"
          height="16"
          viewBox="0 0 24 24"
          fill="none"
          style={{ color: 'var(--text-tertiary)' }}
        >
          <circle cx="11" cy="11" r="7" stroke="currentColor" strokeWidth="2" />
          <path d="M16 16l4 4" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
        </svg>

        <input
          ref={inputRef}
          type="search"
          value={value}
          onChange={(e) => {
            setValue(e.target.value);
            setShowSuggestions(true);
          }}
          onFocus={() => setShowSuggestions(true)}
          onBlur={() => setTimeout(() => setShowSuggestions(false), 200)}
          placeholder={t('placeholder')}
          className="input-field pl-9 pr-16"
          style={compact ? { height: '2.25rem', fontSize: 'var(--text-sm)' } : undefined}
          aria-label={t('placeholder')}
        />

        {/* Keyboard shortcut + clear */}
        <div className="absolute right-[var(--space-sm)] top-1/2 -translate-y-1/2 flex items-center gap-1">
          {value && (
            <button
              type="button"
              onClick={handleClear}
              className="p-0.5 rounded"
              style={{ color: 'var(--text-tertiary)' }}
              aria-label={t('clear')}
            >
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none">
                <path d="M18 6L6 18M6 6l12 12" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
              </svg>
            </button>
          )}
          {!compact && !value && (
            <kbd
              className="hidden sm:inline-flex items-center px-1.5 text-[10px] font-mono rounded"
              style={{
                border: '1px solid var(--border-default)',
                color: 'var(--text-tertiary)',
                backgroundColor: 'var(--bg-inset)',
              }}
            >
              {typeof navigator !== 'undefined' && /Mac/.test(navigator.userAgent) ? '\u2318' : 'Ctrl+'}K
            </kbd>
          )}
        </div>
      </div>

      {/* Recent queries dropdown */}
      {showSuggestions && recentQueries.length > 0 && !value && (
        <div
          className="absolute z-50 w-full mt-1 py-1 rounded-[var(--radius-md)]"
          style={{
            backgroundColor: 'var(--bg-elevated)',
            border: '1px solid var(--border-default)',
            boxShadow: 'var(--shadow-md)',
          }}
        >
          <p
            className="px-[var(--space-sm)] py-1 text-[10px] uppercase tracking-widest font-semibold"
            style={{ color: 'var(--text-tertiary)' }}
          >
            {t('recentSearches')}
          </p>
          {recentQueries.map((q) => (
            <button
              key={q}
              type="button"
              className="w-full text-left px-[var(--space-sm)] py-1.5 text-sm hover:bg-[var(--bg-inset)] transition-colors"
              style={{ color: 'var(--text-secondary)' }}
              onMouseDown={() => {
                setValue(q);
                setShowSuggestions(false);
                if (onSearch) onSearch(q);
                else router.push(`/${locale}/search?q=${encodeURIComponent(q)}`);
              }}
            >
              {q}
            </button>
          ))}
        </div>
      )}
    </form>
  );
}
