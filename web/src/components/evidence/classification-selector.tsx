'use client';

/**
 * ClassificationSelector — dropdown for the four evidence classification levels
 * with a conditional ex parte side selector.
 *
 * TODO: integrate via Sprint 9 wiring pass
 */

import { useId } from 'react';
import { useTranslations } from 'next-intl';

export type Classification =
  | 'public'
  | 'restricted'
  | 'confidential'
  | 'ex_parte';

export type ExParteSide = 'prosecution' | 'defence';

interface ClassificationOption {
  readonly value: Classification;
  readonly labelKey:
    | 'public'
    | 'restricted'
    | 'confidential'
    | 'exParte';
  readonly descriptionKey:
    | 'publicDescription'
    | 'restrictedDescription'
    | 'confidentialDescription'
    | 'exParteDescription';
  readonly color: string;
  readonly bg: string;
}

const OPTIONS: readonly ClassificationOption[] = [
  {
    value: 'public',
    labelKey: 'public',
    descriptionKey: 'publicDescription',
    color: 'var(--status-active)',
    bg: 'var(--status-active-bg)',
  },
  {
    value: 'restricted',
    labelKey: 'restricted',
    descriptionKey: 'restrictedDescription',
    color: 'var(--status-closed)',
    bg: 'var(--status-closed-bg)',
  },
  {
    value: 'confidential',
    labelKey: 'confidential',
    descriptionKey: 'confidentialDescription',
    color: 'var(--status-hold)',
    bg: 'var(--status-hold-bg)',
  },
  {
    value: 'ex_parte',
    labelKey: 'exParte',
    descriptionKey: 'exParteDescription',
    color: 'var(--status-destroyed, var(--status-hold))',
    bg: 'var(--status-destroyed-bg, var(--status-hold-bg))',
  },
] as const;

export function getClassificationStyle(
  value: Classification
): { readonly color: string; readonly bg: string } {
  const match = OPTIONS.find((o) => o.value === value);
  return match ?? OPTIONS[1];
}

export interface ClassificationSelectorProps {
  readonly value: Classification;
  readonly onChange: (next: Classification) => void;
  readonly disabled?: boolean;
  readonly exParteSide?: ExParteSide | null;
  readonly onExParteSideChange?: (next: ExParteSide) => void;
}

export function ClassificationSelector({
  value,
  onChange,
  disabled = false,
  exParteSide = null,
  onExParteSideChange,
}: ClassificationSelectorProps) {
  const t = useTranslations('evidence.classificationSelector');
  const selectId = useId();
  const sideId = useId();
  const errorId = useId();

  const current = OPTIONS.find((o) => o.value === value) ?? OPTIONS[1];
  const showSideError =
    value === 'ex_parte' && (exParteSide === null || exParteSide === undefined);

  return (
    <div className="space-y-[var(--space-sm)]">
      <div>
        <label className="field-label" htmlFor={selectId}>
          {t('label')}
        </label>
        <div className="flex items-center gap-[var(--space-sm)]">
          <select
            id={selectId}
            value={value}
            disabled={disabled}
            onChange={(e) => onChange(e.target.value as Classification)}
            className="input-field flex-1"
            aria-describedby={`${selectId}-desc`}
          >
            {OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {t(opt.labelKey)}
              </option>
            ))}
          </select>
          <span
            className="badge shrink-0"
            style={{ backgroundColor: current.bg, color: current.color }}
          >
            {t(current.labelKey)}
          </span>
        </div>
        <p
          id={`${selectId}-desc`}
          className="text-xs mt-[var(--space-xs)]"
          style={{ color: 'var(--text-tertiary)' }}
        >
          {t(current.descriptionKey)}
        </p>
      </div>

      {value === 'ex_parte' && (
        <div>
          <label className="field-label" htmlFor={sideId}>
            {t('sideLabel')}
          </label>
          <div
            id={sideId}
            role="radiogroup"
            aria-label={t('sideLabel')}
            aria-invalid={showSideError}
            aria-describedby={showSideError ? errorId : undefined}
            className="flex gap-[var(--space-xs)]"
          >
            {(['prosecution', 'defence'] as const).map((side) => {
              const selected = exParteSide === side;
              return (
                <button
                  key={side}
                  type="button"
                  role="radio"
                  aria-checked={selected}
                  disabled={disabled}
                  onClick={() => onExParteSideChange?.(side)}
                  className="px-[var(--space-md)] py-[var(--space-xs)] text-sm rounded-[var(--radius-md)] transition-colors"
                  style={{
                    border: selected
                      ? '1px solid var(--amber-accent)'
                      : '1px solid var(--border-default)',
                    backgroundColor: selected
                      ? 'var(--amber-subtle)'
                      : 'var(--bg-elevated)',
                    color: selected
                      ? 'var(--text-primary)'
                      : 'var(--text-secondary)',
                  }}
                >
                  {t(side)}
                </button>
              );
            })}
          </div>
          {showSideError && (
            <p
              id={errorId}
              className="text-xs mt-[var(--space-xs)]"
              style={{ color: 'var(--status-hold)' }}
              aria-live="polite"
            >
              {t('sideRequiredError')}
            </p>
          )}
        </div>
      )}
    </div>
  );
}
