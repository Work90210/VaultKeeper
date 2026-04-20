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
  processingTimeMs,
  totalHits,
}: {
  defaultValue?: string;
  compact?: boolean;
  onSearch?: (query: string) => void;
  processingTimeMs?: number;
  totalHits?: number;
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
      <div
        className="flex items-center gap-[10px]"
        style={{
          padding: compact ? '8px 14px' : '10px 14px',
          border: '1px solid var(--ink)',
          borderRadius: 12,
          background: 'var(--paper)',
        }}
      >
        {/* Search icon */}
        <svg
          width="18"
          height="18"
          viewBox="0 0 16 16"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.5"
          className="shrink-0"
          style={{ color: 'var(--muted)' }}
        >
          <circle cx="7" cy="7" r="4" />
          <path d="M10 10l3 3" />
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
          className="flex-1 min-w-0"
          style={{
            border: 'none',
            outline: 'none',
            background: 'none',
            font: 'inherit',
            fontSize: compact ? 14 : 16,
            fontFamily: "'Fraunces', serif",
            letterSpacing: '-.005em',
            color: 'var(--ink)',
          }}
          aria-label={t('placeholder')}
        />

        {/* Clear button */}
        {value && (
          <button
            type="button"
            onClick={handleClear}
            className="shrink-0 p-0.5 rounded"
            style={{ color: 'var(--muted)' }}
            aria-label={t('clear')}
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none">
              <path d="M18 6L6 18M6 6l12 12" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
            </svg>
          </button>
        )}

        {/* Inline meta: processing time + hits */}
        {totalHits !== undefined && processingTimeMs !== undefined && value && (
          <span
            className="shrink-0"
            style={{
              fontFamily: "'JetBrains Mono', monospace",
              fontSize: 11,
              color: 'var(--muted)',
              marginLeft: 6,
              whiteSpace: 'nowrap',
            }}
          >
            {processingTimeMs} ms &middot; {totalHits} hits
          </span>
        )}

        {/* Keyboard shortcut hint */}
        {!compact && !value && (
          <kbd
            className="hidden sm:inline-flex items-center shrink-0"
            style={{
              fontFamily: "'JetBrains Mono', monospace",
              fontSize: 10.5,
              color: 'var(--muted)',
              padding: '2px 8px',
              borderRadius: 5,
              border: '1px solid var(--line)',
              background: 'var(--bg-2)',
              letterSpacing: '.04em',
              textTransform: 'uppercase',
            }}
          >
            {typeof navigator !== 'undefined' && /Mac/.test(navigator.userAgent) ? '\u2318' : 'Ctrl+'}K
          </kbd>
        )}
      </div>

      {/* Recent queries dropdown */}
      {showSuggestions && recentQueries.length > 0 && !value && (
        <div
          className="absolute z-50 w-full mt-1 py-1"
          style={{
            backgroundColor: 'var(--paper)',
            border: '1px solid var(--line)',
            borderRadius: 10,
            boxShadow: '0 8px 24px rgba(0,0,0,.08)',
          }}
        >
          <p
            className="px-[14px] py-1"
            style={{
              fontFamily: "'JetBrains Mono', monospace",
              fontSize: 10.5,
              letterSpacing: '.08em',
              textTransform: 'uppercase',
              color: 'var(--muted)',
            }}
          >
            {t('recentSearches')}
          </p>
          {recentQueries.map((q) => (
            <button
              key={q}
              type="button"
              className="w-full text-left px-[14px] py-1.5 text-sm transition-colors"
              style={{ color: 'var(--ink-2)' }}
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
