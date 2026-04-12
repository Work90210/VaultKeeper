'use client';

/**
 * RetentionDatePicker — date input with a "no expiry" checkbox. Surfaces
 * an inline hint when the selected date lies in the past.
 *
 * TODO: integrate via Sprint 9 wiring pass
 */

import { useId, useMemo } from 'react';
import { useTranslations } from 'next-intl';

export interface RetentionDatePickerProps {
  readonly value: Date | null;
  readonly onChange: (next: Date | null) => void;
  readonly minDate?: Date;
  readonly disabled?: boolean;
}

function toInputValue(date: Date | null): string {
  if (!date) return '';
  const yyyy = date.getFullYear().toString().padStart(4, '0');
  const mm = (date.getMonth() + 1).toString().padStart(2, '0');
  const dd = date.getDate().toString().padStart(2, '0');
  return `${yyyy}-${mm}-${dd}`;
}

function fromInputValue(input: string): Date | null {
  if (!input) return null;
  const parts = input.split('-').map((p) => Number.parseInt(p, 10));
  if (parts.length !== 3 || parts.some((n) => Number.isNaN(n))) return null;
  const [y, m, d] = parts;
  const date = new Date(y, m - 1, d);
  return Number.isNaN(date.getTime()) ? null : date;
}

export function RetentionDatePicker({
  value,
  onChange,
  minDate,
  disabled = false,
}: RetentionDatePickerProps) {
  const t = useTranslations('evidence.retentionDatePicker');
  const dateId = useId();
  const checkboxId = useId();
  const hintId = useId();

  const noExpiry = value === null;

  const isPast = useMemo(() => {
    if (!value) return false;
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    const check = new Date(value);
    check.setHours(0, 0, 0, 0);
    return check.getTime() < today.getTime();
  }, [value]);

  const handleDateChange = (input: string) => {
    onChange(fromInputValue(input));
  };

  const handleToggleNoExpiry = (checked: boolean) => {
    if (checked) {
      onChange(null);
    } else {
      // Restore to today as a sensible default
      onChange(new Date());
    }
  };

  return (
    <div className="space-y-[var(--space-xs)]">
      <label className="field-label" htmlFor={dateId}>
        {t('label')}
      </label>
      <div className="flex items-center gap-[var(--space-sm)]">
        <input
          id={dateId}
          type="date"
          disabled={disabled || noExpiry}
          value={toInputValue(value)}
          min={minDate ? toInputValue(minDate) : undefined}
          onChange={(e) => handleDateChange(e.target.value)}
          className="input-field flex-1"
          aria-describedby={isPast ? hintId : undefined}
        />
      </div>
      <div className="flex items-center gap-[var(--space-xs)]">
        <input
          id={checkboxId}
          type="checkbox"
          disabled={disabled}
          checked={noExpiry}
          onChange={(e) => handleToggleNoExpiry(e.target.checked)}
        />
        <label
          htmlFor={checkboxId}
          className="text-sm"
          style={{ color: 'var(--text-secondary)' }}
        >
          {t('noExpiry')}
        </label>
      </div>
      {isPast && (
        <p
          id={hintId}
          className="text-xs"
          style={{ color: 'var(--status-hold)' }}
          aria-live="polite"
        >
          {t('pastHint')}
        </p>
      )}
      {!isPast && (
        <p
          className="text-xs"
          style={{ color: 'var(--text-tertiary)' }}
        >
          {t('helpText')}
        </p>
      )}
    </div>
  );
}
