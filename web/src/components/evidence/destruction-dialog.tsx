'use client';

/**
 * DestructionDialog — 4-step multi-step confirmation for irreversible
 * evidence destruction. File bytes are destroyed; hash, metadata and
 * custody chain are preserved.
 *
 * TODO: integrate via Sprint 9 wiring pass
 */

import { useCallback, useEffect, useId, useRef, useState } from 'react';
import { useTranslations } from 'next-intl';

import { MIN_DESTRUCTION_AUTHORITY_LENGTH } from '@/lib/constants/evidence';

export interface DestructionEvidence {
  readonly id: string;
  readonly evidenceNumber: string;
  readonly sha256Hash: string;
}

export interface DestructionDialogProps {
  readonly evidence: DestructionEvidence;
  readonly open: boolean;
  readonly onOpenChange: (open: boolean) => void;
  readonly onConfirm: (authority: string) => Promise<void> | void;
}

type Step = 1 | 2 | 3 | 4;

// Sync with Go: internal/evidence/destruction.go MinDestructionAuthorityLength
const MIN_AUTHORITY_LENGTH = MIN_DESTRUCTION_AUTHORITY_LENGTH;

export function DestructionDialog({
  evidence,
  open,
  onOpenChange,
  onConfirm,
}: DestructionDialogProps) {
  const t = useTranslations('evidence.destructionDialog');
  const tCommon = useTranslations('common');
  const titleId = useId();
  const authorityId = useId();
  const authorityErrorId = useId();
  const dialogRef = useRef<HTMLDivElement>(null);

  const [step, setStep] = useState<Step>(1);
  const [authority, setAuthority] = useState('');
  const [authorityError, setAuthorityError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

  // Reset state on open
  useEffect(() => {
    if (open) {
      setStep(1);
      setAuthority('');
      setAuthorityError(null);
      setLoading(false);
      setSubmitError(null);
    }
  }, [open]);

  // Escape key to close (blocked while loading or on success)
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !loading && step !== 4) {
        onOpenChange(false);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [open, loading, step, onOpenChange]);

  // Focus first focusable element when dialog or step changes
  useEffect(() => {
    if (!open) return;
    const node = dialogRef.current;
    if (!node) return;
    const focusable = node.querySelector<HTMLElement>(
      'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
    );
    focusable?.focus();
  }, [open, step]);

  const handleAuthorityChange = useCallback(
    (value: string) => {
      setAuthority(value);
      if (authorityError) setAuthorityError(null);
    },
    [authorityError]
  );

  const handleAuthorityNext = useCallback(() => {
    const trimmed = authority.trim();
    if (trimmed.length < MIN_AUTHORITY_LENGTH) {
      setAuthorityError(t('step2MinLengthError'));
      return;
    }
    setAuthorityError(null);
    setStep(3);
  }, [authority, t]);

  const handleConfirmDestruction = useCallback(async () => {
    setLoading(true);
    setSubmitError(null);
    try {
      await onConfirm(authority.trim());
      setStep(4);
    } catch (err) {
      const message =
        err instanceof Error && err.message
          ? err.message
          : t('errorGeneric');
      setSubmitError(message);
    } finally {
      setLoading(false);
    }
  }, [authority, onConfirm, t]);

  if (!open) return null;

  const truncatedHash = `${evidence.sha256Hash.slice(0, 10)}\u2026${evidence.sha256Hash.slice(-6)}`;

  return (
    <div
      className="fixed inset-0 z-[100] flex items-center justify-center"
      style={{ backgroundColor: 'oklch(0.2 0.01 50 / 0.6)' }}
      role="dialog"
      aria-modal="true"
      aria-labelledby={titleId}
    >
      <div
        ref={dialogRef}
        className="card max-w-lg w-full mx-[var(--space-lg)] p-[var(--space-lg)] space-y-[var(--space-md)]"
      >
        <div className="flex items-center justify-between">
          <h3
            id={titleId}
            className="font-[family-name:var(--font-heading)] text-lg"
            style={{ color: 'var(--text-primary)' }}
          >
            {t('title')}
          </h3>
          <span
            className="text-xs font-[family-name:var(--font-mono)]"
            style={{ color: 'var(--text-tertiary)' }}
          >
            {t('step', { current: step, total: 4 })}
          </span>
        </div>

        {step === 1 && (
          <>
            <h4
              className="text-sm font-semibold"
              style={{ color: 'var(--status-hold)' }}
            >
              {t('step1Title')}
            </h4>
            <p
              className="text-sm"
              style={{ color: 'var(--text-secondary)' }}
            >
              {t('step1Body')}
            </p>
            <div className="flex justify-end gap-[var(--space-sm)]">
              <button
                type="button"
                onClick={() => onOpenChange(false)}
                className="btn-ghost"
              >
                {tCommon('cancel')}
              </button>
              <button
                type="button"
                onClick={() => setStep(2)}
                className="btn-primary"
              >
                {t('step1Continue')}
              </button>
            </div>
          </>
        )}

        {step === 2 && (
          <>
            <h4
              className="text-sm font-semibold"
              style={{ color: 'var(--text-primary)' }}
            >
              {t('step2Title')}
            </h4>
            <p
              className="text-sm"
              style={{ color: 'var(--text-secondary)' }}
            >
              {t('step2Body')}
            </p>
            <div>
              <textarea
                id={authorityId}
                value={authority}
                onChange={(e) => handleAuthorityChange(e.target.value)}
                className="input-field w-full"
                rows={3}
                placeholder={t('step2Placeholder')}
                aria-invalid={authorityError !== null}
                aria-describedby={
                  authorityError ? authorityErrorId : undefined
                }
              />
              {authorityError && (
                <p
                  id={authorityErrorId}
                  className="text-xs mt-[var(--space-xs)]"
                  style={{ color: 'var(--status-hold)' }}
                  aria-live="polite"
                >
                  {authorityError}
                </p>
              )}
            </div>
            <div className="flex justify-between gap-[var(--space-sm)]">
              <button
                type="button"
                onClick={() => setStep(1)}
                className="btn-ghost"
              >
                {tCommon('back')}
              </button>
              <button
                type="button"
                onClick={handleAuthorityNext}
                className="btn-primary"
              >
                {tCommon('next')}
              </button>
            </div>
          </>
        )}

        {step === 3 && (
          <>
            <h4
              className="text-sm font-semibold"
              style={{ color: 'var(--status-hold)' }}
            >
              {t('step3Title')}
            </h4>
            <p
              className="text-sm"
              style={{ color: 'var(--text-secondary)' }}
            >
              {t('step3Body')}
            </p>
            <div className="card-inset p-[var(--space-md)] space-y-[var(--space-sm)]">
              <div>
                <p className="field-label">
                  {t('step3EvidenceNumberLabel')}
                </p>
                <p
                  className="text-sm font-[family-name:var(--font-mono)]"
                  style={{ color: 'var(--text-primary)' }}
                >
                  {evidence.evidenceNumber}
                </p>
              </div>
              <div>
                <p className="field-label">{t('step3HashLabel')}</p>
                <span
                  className="text-xs font-[family-name:var(--font-mono)] break-all"
                  style={{ color: 'var(--text-primary)' }}
                  title={evidence.sha256Hash}
                  aria-label={evidence.sha256Hash}
                >
                  {truncatedHash}
                </span>
                <p
                  className="text-xs mt-[var(--space-xs)]"
                  style={{ color: 'var(--text-tertiary)' }}
                >
                  {t('step3FullHashHint')}
                </p>
              </div>
            </div>
            {submitError && (
              <p
                className="text-sm"
                style={{ color: 'var(--status-hold)' }}
                aria-live="assertive"
              >
                {submitError}
              </p>
            )}
            <div className="flex justify-between gap-[var(--space-sm)]">
              <button
                type="button"
                onClick={() => setStep(2)}
                disabled={loading}
                className="btn-ghost"
              >
                {tCommon('back')}
              </button>
              <button
                type="button"
                onClick={handleConfirmDestruction}
                disabled={loading}
                className="btn-primary"
                style={{ backgroundColor: 'var(--status-hold)' }}
              >
                {loading ? t('step3Loading') : t('step3Confirm')}
              </button>
            </div>
          </>
        )}

        {step === 4 && (
          <>
            <h4
              className="text-sm font-semibold"
              style={{ color: 'var(--status-active)' }}
            >
              {t('step4Title')}
            </h4>
            <p
              className="text-sm"
              style={{ color: 'var(--text-secondary)' }}
            >
              {t('step4Body')}
            </p>
            <ul
              className="text-sm space-y-[var(--space-xs)] list-disc pl-[var(--space-lg)]"
              style={{ color: 'var(--text-secondary)' }}
            >
              <li>{t('step4ItemHash')}</li>
              <li>{t('step4ItemMetadata')}</li>
              <li>{t('step4ItemCustody')}</li>
            </ul>
            <div className="flex justify-end">
              <button
                type="button"
                onClick={() => onOpenChange(false)}
                className="btn-primary"
              >
                {t('step4Done')}
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
