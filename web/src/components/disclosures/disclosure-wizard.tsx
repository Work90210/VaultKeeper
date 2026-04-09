'use client';

import { useState } from 'react';
import type { EvidenceItem } from '@/types';

interface DisclosureWizardProps {
  caseId: string;
  evidence: readonly EvidenceItem[];
  onSubmit: (data: DisclosureFormData) => Promise<void>;
  onCancel: () => void;
}

export interface DisclosureFormData {
  evidence_ids: string[];
  disclosed_to: string;
  notes: string;
  redacted: boolean;
}

const STEPS = ['Evidence', 'Recipient', 'Review'] as const;

const CLASSIFICATION_STYLES: Record<string, { color: string; bg: string }> = {
  public: { color: 'var(--status-active)', bg: 'var(--status-active-bg)' },
  restricted: { color: 'var(--status-closed)', bg: 'var(--status-closed-bg)' },
  confidential: { color: 'var(--status-hold)', bg: 'var(--status-hold-bg)' },
  ex_parte: { color: 'var(--status-hold)', bg: 'var(--status-hold-bg)' },
};

export function DisclosureWizard({
  evidence,
  onSubmit,
  onCancel,
}: DisclosureWizardProps) {
  const [step, setStep] = useState(0);
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [disclosedTo, setDisclosedTo] = useState('');
  const [notes, setNotes] = useState('');
  const [redacted, setRedacted] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const toggleEvidence = (id: string) => {
    setSelectedIds((prev) =>
      prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]
    );
  };

  const handleSubmit = async () => {
    setSubmitting(true);
    setError(null);
    try {
      await onSubmit({
        evidence_ids: selectedIds,
        disclosed_to: disclosedTo,
        notes,
        redacted,
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create disclosure');
      setSubmitting(false);
    }
  };

  const canAdvance = step === 0 ? selectedIds.length > 0 : step === 1 ? disclosedTo !== '' : true;

  return (
    <div>
      {error && <div className="banner-error mb-[var(--space-md)]">{error}</div>}

      {/* ── Step indicator ── */}
      <nav className="flex items-center mb-[var(--space-lg)]">
        {STEPS.map((label, i) => {
          const done = i < step;
          const active = i === step;
          return (
            <div key={label} className="flex items-center">
              <div className="flex items-center gap-[var(--space-xs)]">
                <span
                  className="text-xs font-semibold w-5 h-5 flex items-center justify-center rounded-full"
                  style={{
                    backgroundColor: done || active ? 'var(--amber-accent)' : 'var(--bg-inset)',
                    color: done || active ? 'var(--stone-950)' : 'var(--text-tertiary)',
                  }}
                >
                  {done ? '\u2713' : i + 1}
                </span>
                <span
                  className="text-sm font-medium"
                  style={{ color: active ? 'var(--text-primary)' : 'var(--text-tertiary)' }}
                >
                  {label}
                </span>
              </div>
              {i < STEPS.length - 1 && (
                <div
                  className="w-12 h-px mx-[var(--space-md)]"
                  style={{ backgroundColor: done ? 'var(--amber-accent)' : 'var(--border-default)' }}
                />
              )}
            </div>
          );
        })}
      </nav>

      {/* ── Step 1: Select Evidence ── */}
      {step === 0 && (
        <div>
          <div className="flex items-baseline justify-between mb-[var(--space-sm)]">
            <h3 className="field-label" style={{ marginBottom: 0 }}>
              Select evidence to disclose
            </h3>
            <span className="text-xs" style={{ color: 'var(--text-tertiary)' }}>
              {selectedIds.length} selected
            </span>
          </div>
          <div className="space-y-px max-h-96 overflow-y-auto">
            {evidence.map((item) => {
              const checked = selectedIds.includes(item.id);
              const cls = CLASSIFICATION_STYLES[item.classification] || CLASSIFICATION_STYLES.restricted;
              return (
                <label
                  key={item.id}
                  className="flex items-center gap-[var(--space-sm)] p-[var(--space-sm)] rounded-[var(--radius-sm)] cursor-pointer transition-colors"
                  style={{
                    backgroundColor: checked ? 'var(--amber-subtle)' : 'transparent',
                  }}
                >
                  <input
                    type="checkbox"
                    checked={checked}
                    onChange={() => toggleEvidence(item.id)}
                    className="rounded accent-[var(--amber-accent)]"
                  />
                  <span
                    className="font-[family-name:var(--font-mono)] text-xs shrink-0"
                    style={{ color: 'var(--text-tertiary)', minWidth: '10rem' }}
                  >
                    {item.evidence_number}
                  </span>
                  <span className="text-sm truncate" style={{ color: 'var(--text-primary)' }}>
                    {item.original_name}
                  </span>
                  <span
                    className="badge shrink-0 ml-auto"
                    style={{ backgroundColor: cls.bg, color: cls.color }}
                  >
                    {item.classification.replace('_', ' ')}
                  </span>
                </label>
              );
            })}
          </div>
        </div>
      )}

      {/* ── Step 2: Recipient ── */}
      {step === 1 && (
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-[var(--space-md)]">
          <div>
            <label className="field-label" htmlFor="dw-recipient">Recipient</label>
            <select
              id="dw-recipient"
              value={disclosedTo}
              onChange={(e) => setDisclosedTo(e.target.value)}
              className="input-field"
            >
              <option value="">Select recipient\u2026</option>
              <option value="defence">Defence</option>
              <option value="judge">Judge</option>
              <option value="observer">Observer</option>
              <option value="victim_representative">Victim Representative</option>
            </select>

            <label className="flex items-center gap-[var(--space-sm)] cursor-pointer mt-[var(--space-md)]">
              <input
                type="checkbox"
                checked={redacted}
                onChange={(e) => setRedacted(e.target.checked)}
                className="rounded accent-[var(--amber-accent)]"
              />
              <span className="text-sm" style={{ color: 'var(--text-secondary)' }}>
                Provide redacted versions
              </span>
            </label>
          </div>

          <div>
            <label className="field-label" htmlFor="dw-notes">Notes</label>
            <textarea
              id="dw-notes"
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              rows={5}
              className="input-field resize-y"
              placeholder="Optional notes about this disclosure\u2026"
            />
          </div>
        </div>
      )}

      {/* ── Step 3: Review ── */}
      {step === 2 && (
        <div className="space-y-[var(--space-md)]">
          <div
            className="card-inset p-[var(--space-md)]"
            style={{ borderLeft: '2px solid var(--amber-accent)' }}
          >
            <p className="text-sm font-medium" style={{ color: 'var(--amber-accent)' }}>
              Disclosure is permanent. Once confirmed, these items will be visible to the recipient.
            </p>
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-[1fr_20rem] gap-[var(--space-md)]">
            {/* Evidence list */}
            <div>
              <h3 className="field-label">
                Evidence ({selectedIds.length} {selectedIds.length === 1 ? 'item' : 'items'})
              </h3>
              <div className="space-y-px">
                {evidence
                  .filter((item) => selectedIds.includes(item.id))
                  .map((item) => {
                    const cls = CLASSIFICATION_STYLES[item.classification] || CLASSIFICATION_STYLES.restricted;
                    return (
                      <div
                        key={item.id}
                        className="flex items-center gap-[var(--space-sm)] p-[var(--space-xs)]"
                      >
                        <span
                          className="font-[family-name:var(--font-mono)] text-xs"
                          style={{ color: 'var(--text-tertiary)' }}
                        >
                          {item.evidence_number}
                        </span>
                        <span className="text-sm truncate" style={{ color: 'var(--text-primary)' }}>
                          {item.original_name}
                        </span>
                        <span
                          className="badge shrink-0 ml-auto"
                          style={{ backgroundColor: cls.bg, color: cls.color }}
                        >
                          {item.classification.replace('_', ' ')}
                        </span>
                      </div>
                    );
                  })}
              </div>
            </div>

            {/* Summary sidebar */}
            <aside className="card-inset p-[var(--space-md)] space-y-[var(--space-sm)]">
              <div>
                <span className="field-label">Recipient</span>
                <p className="text-sm" style={{ color: 'var(--text-primary)' }}>
                  {disclosedTo}
                </p>
              </div>
              <div>
                <span className="field-label">Redacted</span>
                <p className="text-sm" style={{ color: 'var(--text-primary)' }}>
                  {redacted ? 'Yes' : 'No'}
                </p>
              </div>
              {notes && (
                <div>
                  <span className="field-label">Notes</span>
                  <p
                    className="text-sm whitespace-pre-wrap"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    {notes}
                  </p>
                </div>
              )}
            </aside>
          </div>
        </div>
      )}

      {/* ── Navigation ── */}
      <div className="flex gap-[var(--space-sm)] justify-between mt-[var(--space-lg)]">
        <button
          type="button"
          onClick={step === 0 ? onCancel : () => setStep(step - 1)}
          className="btn-ghost"
        >
          {step === 0 ? 'Cancel' : 'Back'}
        </button>
        {step < 2 ? (
          <button
            type="button"
            onClick={() => setStep(step + 1)}
            disabled={!canAdvance}
            className="btn-primary"
          >
            Next
          </button>
        ) : (
          <button
            type="button"
            onClick={handleSubmit}
            disabled={submitting}
            className="btn-primary"
          >
            {submitting ? 'Disclosing\u2026' : 'Confirm Disclosure'}
          </button>
        )}
      </div>
    </div>
  );
}
