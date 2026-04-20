'use client';

import { useState } from 'react';
import { SOURCE_CREDIBILITIES, RECOMMENDATIONS } from '@/types';
import type { SourceCredibility, Recommendation } from '@/types';

interface AssessmentFormState {
  readonly relevanceScore: number;
  readonly relevanceRationale: string;
  readonly reliabilityScore: number;
  readonly reliabilityRationale: string;
  readonly sourceCredibility: SourceCredibility;
  readonly misleadingIndicators: string;
  readonly recommendation: Recommendation;
  readonly methodology: string;
}

const INITIAL_STATE: AssessmentFormState = {
  relevanceScore: 0,
  relevanceRationale: '',
  reliabilityScore: 0,
  reliabilityRationale: '',
  sourceCredibility: 'unassessed',
  misleadingIndicators: '',
  recommendation: 'collect',
  methodology: '',
};

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

function ScoreDots({
  label,
  value,
  onChange,
}: {
  label: string;
  value: number;
  onChange: (score: number) => void;
}) {
  return (
    <div>
      <span className="field-label">{label}</span>
      <div
        className="flex items-center gap-[var(--space-sm)] mt-[var(--space-xs)]"
        role="radiogroup"
        aria-label={`${label} score`}
        tabIndex={0}
        onKeyDown={(e) => {
          if (e.key === 'ArrowRight' || e.key === 'ArrowDown') {
            e.preventDefault();
            onChange(Math.min(value + 1, 5));
          } else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') {
            e.preventDefault();
            onChange(Math.max(value - 1, 1));
          } else if (e.key === 'Home') {
            e.preventDefault();
            onChange(1);
          } else if (e.key === 'End') {
            e.preventDefault();
            onChange(5);
          }
        }}
      >
        {[1, 2, 3, 4, 5].map((n) => (
          <button
            key={n}
            type="button"
            role="radio"
            aria-checked={value === n}
            aria-label={`Score ${n}`}
            tabIndex={-1}
            onClick={() => onChange(n)}
            style={{
              width: '28px',
              height: '28px',
              borderRadius: 'var(--radius-full)',
              border: `2px solid ${value >= n ? 'var(--amber-accent)' : 'var(--border-default)'}`,
              backgroundColor: value >= n ? 'var(--amber-accent)' : 'transparent',
              cursor: 'pointer',
              transition: `all var(--duration-fast) ease`,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontSize: 'var(--text-xs)',
              fontWeight: 600,
              color: value >= n ? 'var(--stone-950)' : 'var(--text-tertiary)',
            }}
          >
            {n}
          </button>
        ))}
      </div>
    </div>
  );
}

export function AssessmentForm({
  evidenceId,
  caseId,
  accessToken,
  onSaved,
}: {
  evidenceId: string;
  caseId: string;
  accessToken: string;
  onSaved: () => void;
}) {
  const [form, setForm] = useState<AssessmentFormState>(INITIAL_STATE);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const updateField = <K extends keyof AssessmentFormState>(
    field: K,
    value: AssessmentFormState[K],
  ) => {
    setForm((prev) => ({ ...prev, [field]: value }));
  };

  const canSubmit =
    form.relevanceScore >= 1 &&
    form.relevanceScore <= 5 &&
    form.relevanceRationale.trim() !== '' &&
    form.reliabilityScore >= 1 &&
    form.reliabilityScore <= 5 &&
    form.reliabilityRationale.trim() !== '';

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!canSubmit) return;

    setSaving(true);
    setError(null);

    const misleadingIndicators = form.misleadingIndicators
      .split(',')
      .map((t) => t.trim())
      .filter(Boolean);

    const body = {
      relevance_score: form.relevanceScore,
      relevance_rationale: form.relevanceRationale.trim(),
      reliability_score: form.reliabilityScore,
      reliability_rationale: form.reliabilityRationale.trim(),
      source_credibility: form.sourceCredibility,
      misleading_indicators: misleadingIndicators,
      recommendation: form.recommendation,
      methodology: form.methodology.trim() || undefined,
      case_id: caseId,
    };

    try {
      const res = await fetch(
        `${API_BASE}/api/evidence/${evidenceId}/assessments`,
        {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${accessToken}`,
          },
          body: JSON.stringify(body),
        },
      );

      if (!res.ok) {
        const data = await res.json().catch(() => null);
        setError(data?.error || `Save failed (${res.status})`);
        setSaving(false);
        return;
      }

      setForm(INITIAL_STATE);
      setSaving(false);
      onSaved();
    } catch {
      setError('An unexpected error occurred.');
      setSaving(false);
    }
  };

  return (
    <form
      onSubmit={handleSubmit}
      className="card p-[var(--space-lg)] space-y-[var(--space-md)]"
      style={{ animation: 'fade-in var(--duration-slow) var(--ease-out-expo)' }}
    >
      <h2
        className="font-[family-name:var(--font-heading)] text-lg"
        style={{ color: 'var(--text-primary)' }}
      >
        Evidence Assessment
      </h2>

      {error && (
        <div className="banner-error">{error}</div>
      )}

      {/* Relevance */}
      <div className="space-y-[var(--space-sm)]">
        <ScoreDots
          label="Relevance Score *"
          value={form.relevanceScore}
          onChange={(score) => updateField('relevanceScore', score)}
        />
        <div>
          <label className="field-label">
            Relevance Rationale{' '}
            <span style={{ color: 'var(--status-hold)' }}>*</span>
          </label>
          <textarea
            value={form.relevanceRationale}
            onChange={(e) => updateField('relevanceRationale', e.target.value)}
            className="input-field"
            rows={3}
            placeholder="Explain why this evidence is or is not relevant to the case"
            required
          />
        </div>
      </div>

      {/* Reliability */}
      <div className="space-y-[var(--space-sm)]">
        <ScoreDots
          label="Reliability Score *"
          value={form.reliabilityScore}
          onChange={(score) => updateField('reliabilityScore', score)}
        />
        <div>
          <label className="field-label">
            Reliability Rationale{' '}
            <span style={{ color: 'var(--status-hold)' }}>*</span>
          </label>
          <textarea
            value={form.reliabilityRationale}
            onChange={(e) => updateField('reliabilityRationale', e.target.value)}
            className="input-field"
            rows={3}
            placeholder="Explain the basis for the reliability assessment"
            required
          />
        </div>
      </div>

      {/* Source Credibility & Recommendation row */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-[var(--space-md)]">
        <div>
          <label className="field-label">Source Credibility</label>
          <select
            value={form.sourceCredibility}
            onChange={(e) =>
              updateField('sourceCredibility', e.target.value as SourceCredibility)
            }
            className="input-field"
          >
            {SOURCE_CREDIBILITIES.map((sc) => (
              <option key={sc.value} value={sc.value}>
                {sc.label}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label className="field-label">Recommendation</label>
          <select
            value={form.recommendation}
            onChange={(e) =>
              updateField('recommendation', e.target.value as Recommendation)
            }
            className="input-field"
          >
            {RECOMMENDATIONS.map((r) => (
              <option key={r.value} value={r.value}>
                {r.label}
              </option>
            ))}
          </select>
        </div>
      </div>

      {/* Misleading Indicators */}
      <div>
        <label className="field-label">
          Misleading Indicators (comma-separated)
        </label>
        <input
          type="text"
          value={form.misleadingIndicators}
          onChange={(e) => updateField('misleadingIndicators', e.target.value)}
          className="input-field"
          placeholder="e.g. metadata inconsistency, visual artifacts, temporal mismatch"
        />
      </div>

      {/* Methodology */}
      <div>
        <label className="field-label">Methodology</label>
        <textarea
          value={form.methodology}
          onChange={(e) => updateField('methodology', e.target.value)}
          className="input-field"
          rows={3}
          placeholder="Describe the assessment methodology used (optional)"
        />
      </div>

      {/* Actions */}
      <div className="flex justify-end gap-[var(--space-sm)]">
        <button
          type="button"
          onClick={() => setForm(INITIAL_STATE)}
          className="btn-secondary"
          disabled={saving}
        >
          Reset
        </button>
        <button
          type="submit"
          className="btn-primary"
          disabled={!canSubmit || saving}
        >
          {saving ? 'Saving\u2026' : 'Save Assessment'}
        </button>
      </div>
    </form>
  );
}
