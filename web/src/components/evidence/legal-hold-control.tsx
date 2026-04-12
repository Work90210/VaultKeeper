'use client';

/**
 * LegalHoldControl — toggle + confirmation dialog for placing/releasing
 * evidence legal holds.
 *
 * ⚠️ Sprint 9 backend contract: the `reason` captured below is sent as the
 * BODY of the notification to case members, NOT persisted as a separate
 * audit column. The custody chain records the hold state change with
 * actor, timestamp, and previous value — the reason text is delivered
 * through the notification channel only. If you need the reason
 * permanently archived, attach it to the case file separately.
 *
 * TODO: integrate via Sprint 9 wiring pass
 */

import { useCallback, useEffect, useId, useRef, useState } from 'react';
import { useTranslations } from 'next-intl';

export interface LegalHoldControlProps {
  readonly value: boolean;
  readonly disabled?: boolean;
  /**
   * Invoked when the operator confirms the toggle. `reason` is the
   * notification body — see the file-level comment for the persistence
   * contract.
   */
  readonly onChange: (nextValue: boolean, reason: string) => void;
}

export function LegalHoldControl({
  value,
  disabled = false,
  onChange,
}: LegalHoldControlProps) {
  const t = useTranslations('evidence.legalHoldControl');
  const [dialogOpen, setDialogOpen] = useState(false);
  const [reason, setReason] = useState('');
  const [showReasonError, setShowReasonError] = useState(false);

  const openDialog = useCallback(() => {
    setReason('');
    setShowReasonError(false);
    setDialogOpen(true);
  }, []);

  const closeDialog = useCallback(() => {
    setDialogOpen(false);
  }, []);

  const handleConfirm = useCallback(() => {
    const trimmed = reason.trim();
    if (trimmed.length === 0) {
      setShowReasonError(true);
      return;
    }
    onChange(!value, trimmed);
    setDialogOpen(false);
  }, [reason, value, onChange]);

  return (
    <div>
      <div className="flex items-center justify-between gap-[var(--space-md)]">
        <div>
          <p className="field-label" style={{ marginBottom: 0 }}>
            {t('label')}
          </p>
          <p
            className="text-sm"
            style={{ color: 'var(--text-secondary)' }}
          >
            {value ? t('statusOn') : t('statusOff')}
          </p>
        </div>
        <button
          type="button"
          onClick={openDialog}
          disabled={disabled}
          className={value ? 'btn-secondary' : 'btn-primary'}
        >
          {value ? t('release') : t('place')}
        </button>
      </div>

      {dialogOpen && (
        <LegalHoldDialog
          currentValue={value}
          reason={reason}
          showReasonError={showReasonError}
          onReasonChange={(next) => {
            setReason(next);
            if (showReasonError && next.trim().length > 0) {
              setShowReasonError(false);
            }
          }}
          onConfirm={handleConfirm}
          onClose={closeDialog}
        />
      )}
    </div>
  );
}

interface DialogProps {
  readonly currentValue: boolean;
  readonly reason: string;
  readonly showReasonError: boolean;
  readonly onReasonChange: (next: string) => void;
  readonly onConfirm: () => void;
  readonly onClose: () => void;
}

function LegalHoldDialog({
  currentValue,
  reason,
  showReasonError,
  onReasonChange,
  onConfirm,
  onClose,
}: DialogProps) {
  const t = useTranslations('evidence.legalHoldControl');
  const tCommon = useTranslations('common');
  const titleId = useId();
  const reasonId = useId();
  const errorId = useId();
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    textareaRef.current?.focus();
  }, []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onClose]);

  const nextValue = !currentValue;
  const title = nextValue ? t('dialogTitlePlace') : t('dialogTitleRelease');
  const confirmLabel = nextValue ? t('confirmPlace') : t('confirmRelease');
  const warning = nextValue ? t('warningPlace') : t('warningRelease');
  const notificationText = nextValue
    ? t('notificationPlaceText')
    : t('notificationReleaseText');

  return (
    <div
      className="fixed inset-0 z-[100] flex items-center justify-center"
      style={{ backgroundColor: 'oklch(0.2 0.01 50 / 0.6)' }}
      role="dialog"
      aria-modal="true"
      aria-labelledby={titleId}
    >
      <div className="card max-w-lg w-full mx-[var(--space-lg)] p-[var(--space-lg)] space-y-[var(--space-md)]">
        <h3
          id={titleId}
          className="font-[family-name:var(--font-heading)] text-lg"
          style={{ color: 'var(--text-primary)' }}
        >
          {title}
        </h3>

        <div className="card-inset p-[var(--space-md)] grid grid-cols-2 gap-[var(--space-md)]">
          <div>
            <p className="field-label">{t('currentState')}</p>
            <p className="text-sm" style={{ color: 'var(--text-primary)' }}>
              {currentValue ? t('statusOn') : t('statusOff')}
            </p>
          </div>
          <div>
            <p className="field-label">{t('willChangeTo')}</p>
            <p className="text-sm" style={{ color: 'var(--text-primary)' }}>
              {nextValue ? t('statusOn') : t('statusOff')}
            </p>
          </div>
        </div>

        <p
          className="text-xs"
          style={{ color: 'var(--text-tertiary)' }}
        >
          {warning}
        </p>

        <div>
          <p className="field-label">{t('notificationPreview')}</p>
          <div
            className="p-[var(--space-sm)] rounded-[var(--radius-md)] text-sm"
            style={{
              backgroundColor: 'var(--bg-inset)',
              border: '1px solid var(--border-subtle)',
              color: 'var(--text-secondary)',
            }}
          >
            {notificationText}
          </div>
          <p
            className="text-xs mt-[var(--space-xs)]"
            style={{ color: 'var(--text-tertiary)' }}
          >
            {t('notificationHelp')}
          </p>
        </div>

        <div>
          <label className="field-label" htmlFor={reasonId}>
            {t('reasonLabel')}
          </label>
          <textarea
            id={reasonId}
            ref={textareaRef}
            value={reason}
            onChange={(e) => onReasonChange(e.target.value)}
            className="input-field w-full"
            rows={3}
            placeholder={t('reasonPlaceholder')}
            aria-invalid={showReasonError}
            aria-describedby={showReasonError ? errorId : undefined}
            required
          />
          {showReasonError && (
            <p
              id={errorId}
              className="text-xs mt-[var(--space-xs)]"
              style={{ color: 'var(--status-hold)' }}
              aria-live="polite"
            >
              {t('reasonRequiredError')}
            </p>
          )}
        </div>

        <div className="flex justify-end gap-[var(--space-sm)]">
          <button type="button" onClick={onClose} className="btn-ghost">
            {tCommon('cancel')}
          </button>
          <button
            type="button"
            onClick={onConfirm}
            className="btn-primary"
          >
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
}
