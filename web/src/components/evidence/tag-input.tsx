'use client';

/**
 * TagInput — multi-tag input with async autocomplete, inline creation,
 * validation and optional colour-coded tag categories.
 *
 * TODO: integrate via Sprint 9 wiring pass
 */

import {
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
} from 'react';
import { useTranslations } from 'next-intl';

export type TagCategory =
  | 'evidence-type'
  | 'source-type'
  | 'relevance'
  | 'default';

export interface CategorisedTag {
  readonly name: string;
  readonly category?: TagCategory;
}

export interface TagInputProps {
  readonly value: readonly string[];
  readonly onChange: (next: readonly string[]) => void;
  readonly fetchSuggestions: (query: string) => Promise<readonly string[]>;
  readonly disabled?: boolean;
  readonly placeholder?: string;
  readonly categoryFor?: (tag: string) => TagCategory | undefined;
  readonly maxTags?: number;
}

const DEFAULT_MAX_TAGS = 50;
const MAX_TAG_LENGTH = 100;
// Tags are stored lowercase server-side (see internal/evidence/tags.go
// NormalizeTags). The pattern is strict-lowercase here so the UI never
// briefly displays an uppercase tag that will be re-cased on save.
const TAG_PATTERN = /^[a-z0-9_-]+$/;
const DEBOUNCE_MS = 200;

const CATEGORY_STYLES: Record<
  TagCategory,
  { readonly color: string; readonly bg: string; readonly border: string }
> = {
  'evidence-type': {
    color: 'var(--amber-accent)',
    bg: 'var(--amber-subtle)',
    border: 'var(--amber-accent)',
  },
  'source-type': {
    color: 'var(--teal-accent, var(--status-active))',
    bg: 'var(--teal-subtle, var(--status-active-bg))',
    border: 'var(--teal-accent, var(--status-active))',
  },
  relevance: {
    color: 'var(--navy-accent, var(--status-closed))',
    bg: 'var(--navy-subtle, var(--status-closed-bg))',
    border: 'var(--navy-accent, var(--status-closed))',
  },
  default: {
    color: 'var(--text-secondary)',
    bg: 'var(--bg-inset)',
    border: 'var(--border-subtle)',
  },
};

function validateTag(
  tag: string,
  existing: readonly string[],
  maxTags: number,
  messages: {
    readonly invalidChars: string;
    readonly tooLong: string;
    readonly tooMany: string;
    readonly duplicate: string;
  }
): string | null {
  if (existing.length >= maxTags) return messages.tooMany;
  if (tag.length > MAX_TAG_LENGTH) return messages.tooLong;
  if (!TAG_PATTERN.test(tag)) return messages.invalidChars;
  if (existing.includes(tag)) return messages.duplicate;
  return null;
}

export function TagInput({
  value,
  onChange,
  fetchSuggestions,
  disabled = false,
  placeholder,
  categoryFor,
  maxTags = DEFAULT_MAX_TAGS,
}: TagInputProps) {
  const t = useTranslations('evidence.tagInput');
  const inputRef = useRef<HTMLInputElement>(null);
  const listId = useId();
  const errorId = useId();

  const [query, setQuery] = useState('');
  const [suggestions, setSuggestions] = useState<readonly string[]>([]);
  const [loading, setLoading] = useState(false);
  const [open, setOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [activeIndex, setActiveIndex] = useState(-1);

  const errorMessages = useMemo(
    () => ({
      invalidChars: t('invalidChars'),
      tooLong: t('tooLong'),
      tooMany: t('tooMany'),
      duplicate: t('duplicate'),
    }),
    [t]
  );

  // Debounced suggestion fetch
  useEffect(() => {
    if (!query.trim()) {
      setSuggestions([]);
      setLoading(false);
      return;
    }
    let cancelled = false;
    setLoading(true);
    const timer = setTimeout(async () => {
      try {
        const result = await fetchSuggestions(query.trim());
        if (!cancelled) {
          const filtered = result.filter((s) => !value.includes(s));
          setSuggestions(filtered);
          setActiveIndex(filtered.length > 0 ? 0 : -1);
        }
      } catch {
        if (!cancelled) setSuggestions([]);
      } finally {
        if (!cancelled) setLoading(false);
      }
    }, DEBOUNCE_MS);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [query, fetchSuggestions, value]);

  const addTag = useCallback(
    (tag: string) => {
      // Lowercase + trim to match server-side NormalizeTags. This avoids
      // the "type WITNESS, see it flip to witness on save" flicker.
      const normalised = tag.trim().toLowerCase();
      if (!normalised) return;
      const err = validateTag(normalised, value, maxTags, errorMessages);
      if (err) {
        setError(err);
        return;
      }
      setError(null);
      onChange([...value, normalised]);
      setQuery('');
      setSuggestions([]);
      setOpen(false);
      setActiveIndex(-1);
    },
    [value, onChange, maxTags, errorMessages]
  );

  const removeTag = useCallback(
    (tag: string) => {
      onChange(value.filter((v) => v !== tag));
      setError(null);
    },
    [value, onChange]
  );

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      if (activeIndex >= 0 && suggestions[activeIndex]) {
        addTag(suggestions[activeIndex]);
      } else if (query.trim()) {
        addTag(query.trim());
      }
    } else if (e.key === 'ArrowDown') {
      if (suggestions.length === 0) return;
      e.preventDefault();
      setActiveIndex((idx) => (idx + 1) % suggestions.length);
    } else if (e.key === 'ArrowUp') {
      if (suggestions.length === 0) return;
      e.preventDefault();
      setActiveIndex((idx) =>
        idx <= 0 ? suggestions.length - 1 : idx - 1
      );
    } else if (e.key === 'Escape') {
      setOpen(false);
      setSuggestions([]);
    } else if (e.key === 'Backspace' && !query && value.length > 0) {
      // Remove last tag on backspace when input empty
      removeTag(value[value.length - 1]);
    }
  };

  const showCreateHint =
    query.trim().length > 0 &&
    !loading &&
    !suggestions.some(
      (s) => s.toLowerCase() === query.trim().toLowerCase()
    ) &&
    TAG_PATTERN.test(query.trim());

  return (
    <div className="space-y-[var(--space-xs)]">
      <div
        className="flex flex-wrap items-center gap-[var(--space-xs)] p-[var(--space-xs)] rounded-[var(--radius-md)]"
        style={{
          border: '1px solid var(--border-default)',
          backgroundColor: 'var(--bg-elevated)',
          minHeight: '2.5rem',
        }}
        onClick={() => inputRef.current?.focus()}
      >
        {value.length === 0 && !query && (
          <span
            className="text-xs px-[var(--space-xs)]"
            style={{ color: 'var(--text-tertiary)' }}
          >
            {t('empty')}
          </span>
        )}
        {value.map((tag) => {
          const category = categoryFor?.(tag) ?? 'default';
          const style = CATEGORY_STYLES[category];
          return (
            <span
              key={tag}
              className="inline-flex items-center gap-[var(--space-xs)] px-[var(--space-sm)] py-[2px] rounded-full text-xs font-medium"
              style={{
                color: style.color,
                backgroundColor: style.bg,
                border: `1px solid ${style.border}`,
              }}
            >
              <span>{tag}</span>
              <button
                type="button"
                disabled={disabled}
                onClick={(e) => {
                  e.stopPropagation();
                  removeTag(tag);
                }}
                aria-label={t('remove', { tag })}
                className="inline-flex items-center justify-center rounded-full"
                style={{
                  width: '14px',
                  height: '14px',
                  color: style.color,
                  backgroundColor: 'transparent',
                  fontSize: '14px',
                  lineHeight: 1,
                }}
              >
                {'\u00d7'}
              </button>
            </span>
          );
        })}
        <input
          ref={inputRef}
          type="text"
          value={query}
          onChange={(e) => {
            setQuery(e.target.value);
            setOpen(true);
            if (error) setError(null);
          }}
          onKeyDown={handleKeyDown}
          onFocus={() => setOpen(true)}
          onBlur={() => {
            // Delay so clicks on suggestions still register
            setTimeout(() => setOpen(false), 120);
          }}
          disabled={disabled || value.length >= maxTags}
          placeholder={placeholder ?? t('placeholder')}
          className="flex-1 min-w-[8rem] bg-transparent outline-none text-sm"
          style={{ color: 'var(--text-primary)' }}
          role="combobox"
          aria-expanded={open && suggestions.length > 0}
          aria-controls={listId}
          aria-autocomplete="list"
          aria-invalid={error !== null}
          aria-describedby={error ? errorId : undefined}
        />
      </div>

      {open && (loading || suggestions.length > 0 || showCreateHint) && (
        <ul
          id={listId}
          role="listbox"
          className="rounded-[var(--radius-md)] overflow-hidden"
          style={{
            border: '1px solid var(--border-default)',
            backgroundColor: 'var(--bg-elevated)',
            maxHeight: '12rem',
            overflowY: 'auto',
          }}
        >
          {loading && (
            <li
              className="px-[var(--space-sm)] py-[var(--space-xs)] text-xs"
              style={{ color: 'var(--text-tertiary)' }}
            >
              {t('suggestionsLoading')}
            </li>
          )}
          {!loading &&
            suggestions.map((s, idx) => (
              <li
                key={s}
                role="option"
                aria-selected={idx === activeIndex}
                onMouseDown={(e) => {
                  e.preventDefault();
                  addTag(s);
                }}
                onMouseEnter={() => setActiveIndex(idx)}
                className="px-[var(--space-sm)] py-[var(--space-xs)] text-sm cursor-pointer"
                style={{
                  backgroundColor:
                    idx === activeIndex
                      ? 'var(--bg-inset)'
                      : 'transparent',
                  color: 'var(--text-primary)',
                }}
              >
                {s}
              </li>
            ))}
          {!loading && showCreateHint && (
            <li
              className="px-[var(--space-sm)] py-[var(--space-xs)] text-xs border-t"
              style={{
                color: 'var(--text-tertiary)',
                borderColor: 'var(--border-subtle)',
              }}
            >
              {t('createHint', { query: query.trim() })}
            </li>
          )}
          {!loading &&
            suggestions.length === 0 &&
            !showCreateHint && (
              <li
                className="px-[var(--space-sm)] py-[var(--space-xs)] text-xs"
                style={{ color: 'var(--text-tertiary)' }}
              >
                {t('noSuggestions')}
              </li>
            )}
        </ul>
      )}

      {error && (
        <p
          id={errorId}
          className="text-xs"
          style={{ color: 'var(--status-hold)' }}
          aria-live="polite"
        >
          {error}
        </p>
      )}
    </div>
  );
}
