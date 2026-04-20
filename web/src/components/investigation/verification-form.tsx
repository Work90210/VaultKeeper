'use client';

import { useState, useCallback } from 'react';
import type { VerificationType, Finding, ConfidenceLevel } from '@/types';
import { VERIFICATION_TYPES, FINDINGS, CONFIDENCE_LEVELS } from '@/types';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

interface VerificationFormProps {
  readonly evidenceId: string;
  readonly caseId: string;
  readonly accessToken: string;
  readonly onSaved: () => void;
}

interface FormState {
  readonly verificationType: VerificationType | '';
  readonly methodology: string;
  readonly toolsUsed: string;
  readonly sourcesConsulted: string;
  readonly finding: Finding | '';
  readonly findingRationale: string;
  readonly confidenceLevel: ConfidenceLevel | '';
  readonly limitations: string;
  readonly caveats: string;
}

const INITIAL_FORM: FormState = {
  verificationType: '',
  methodology: '',
  toolsUsed: '',
  sourcesConsulted: '',
  finding: '',
  findingRationale: '',
  confidenceLevel: '',
  limitations: '',
  caveats: '',
};

function parseTags(raw: string): string[] {
  return raw
    .split(',')
    .map((t) => t.trim())
    .filter(Boolean);
}

export function VerificationForm({
  evidenceId,
  caseId,
  accessToken,
  onSaved,
}: VerificationFormProps) {
  const [form, setForm] = useState<FormState>(INITIAL_FORM);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const updateField = useCallback(
    <K extends keyof FormState>(field: K, value: FormState[K]) => {
      setForm((prev) => ({ ...prev, [field]: value }));
    },
    [],
  );

  const isValid =
    form.verificationType !== '' &&
    form.methodology.trim().length > 0 &&
    form.finding !== '' &&
    form.findingRationale.trim().length > 0 &&
    form.confidenceLevel !== '';

  const handleSave = useCallback(async () => {
    if (!isValid) return;

    setSaving(true);
    setError(null);

    const body = {
      case_id: caseId,
      verification_type: form.verificationType,
      methodology: form.methodology.trim(),
      tools_used: parseTags(form.toolsUsed),
      sources_consulted: parseTags(form.sourcesConsulted),
      finding: form.finding,
      finding_rationale: form.findingRationale.trim(),
      confidence_level: form.confidenceLevel,
      limitations: form.limitations.trim() || undefined,
      caveats: parseTags(form.caveats),
    };

    try {
      const res = await fetch(
        `${API_BASE}/api/evidence/${evidenceId}/verifications`,
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
        throw new Error(
          data?.error || `Server responded with ${res.status}`,
        );
      }

      setForm(INITIAL_FORM);
      onSaved();
    } catch (err) {
      setError(
        err instanceof Error ? err.message : 'Failed to save verification',
      );
    } finally {
      setSaving(false);
    }
  }, [isValid, form, evidenceId, caseId, accessToken, onSaved]);

  return (
    <div className="card p-[var(--space-lg)] space-y-[var(--space-md)]">
      <h3
        style={{
          fontSize: 'var(--text-lg)',
          fontWeight: 600,
          color: 'var(--text-primary)',
          marginBottom: 'var(--space-sm)',
        }}
      >
        New Verification Record
      </h3>

      {error && <div className="banner-error">{error}</div>}

      {/* Verification Type */}
      <div>
        <label className="field-label" htmlFor="vf-type">Verification Type *</label>
        <select
          id="vf-type"
          className="input-field"
          value={form.verificationType}
          onChange={(e) =>
            updateField('verificationType', e.target.value as VerificationType)
          }
        >
          <option value="">Select type...</option>
          {VERIFICATION_TYPES.map((vt) => (
            <option key={vt.value} value={vt.value}>
              {vt.label}
            </option>
          ))}
        </select>
      </div>

      {/* Methodology */}
      <div>
        <label className="field-label" htmlFor="vf-methodology">Methodology *</label>
        <textarea
          id="vf-methodology"
          className="input-field"
          rows={3}
          placeholder="Describe the verification methodology used..."
          value={form.methodology}
          onChange={(e) => updateField('methodology', e.target.value)}
        />
      </div>

      {/* Tools Used */}
      <div>
        <label className="field-label" htmlFor="vf-tools-used">Tools Used</label>
        <input
          type="text"
          className="input-field"
          placeholder="e.g. InVID, FotoForensics, ExifTool (comma-separated)"
          value={form.toolsUsed}
          onChange={(e) => updateField('toolsUsed', e.target.value)}
        />
        {form.toolsUsed && (
          <div
            style={{
              display: 'flex',
              flexWrap: 'wrap',
              gap: 'var(--space-xs)',
              marginTop: 'var(--space-xs)',
            }}
          >
            {parseTags(form.toolsUsed).map((tag) => (
              <span
                key={tag}
                className="badge"
                style={{
                  backgroundColor: 'var(--bg-inset)',
                  color: 'var(--text-secondary)',
                  border: '1px solid var(--border-subtle)',
                }}
              >
                {tag}
              </span>
            ))}
          </div>
        )}
      </div>

      {/* Sources Consulted */}
      <div>
        <label className="field-label" htmlFor="vf-sources">Sources Consulted</label>
        <input
          type="text"
          className="input-field"
          placeholder="e.g. Google Earth, Sentinel Hub (comma-separated)"
          value={form.sourcesConsulted}
          onChange={(e) => updateField('sourcesConsulted', e.target.value)}
        />
        {form.sourcesConsulted && (
          <div
            style={{
              display: 'flex',
              flexWrap: 'wrap',
              gap: 'var(--space-xs)',
              marginTop: 'var(--space-xs)',
            }}
          >
            {parseTags(form.sourcesConsulted).map((tag) => (
              <span
                key={tag}
                className="badge"
                style={{
                  backgroundColor: 'var(--bg-inset)',
                  color: 'var(--text-secondary)',
                  border: '1px solid var(--border-subtle)',
                }}
              >
                {tag}
              </span>
            ))}
          </div>
        )}
      </div>

      {/* Finding */}
      <div>
        <label className="field-label" htmlFor="vf-finding">Finding *</label>
        <select
          id="vf-finding"
          className="input-field"
          value={form.finding}
          onChange={(e) =>
            updateField('finding', e.target.value as Finding)
          }
        >
          <option value="">Select finding...</option>
          {FINDINGS.map((f) => (
            <option key={f.value} value={f.value}>
              {f.label}
            </option>
          ))}
        </select>
      </div>

      {/* Finding Rationale */}
      <div>
        <label className="field-label" htmlFor="vf-rationale">Finding Rationale *</label>
        <textarea
          className="input-field"
          rows={3}
          placeholder="Explain the basis for this finding..."
          value={form.findingRationale}
          onChange={(e) => updateField('findingRationale', e.target.value)}
        />
      </div>

      {/* Confidence Level */}
      <div>
        <label className="field-label" htmlFor="vf-confidence">Confidence Level *</label>
        <select
          className="input-field"
          value={form.confidenceLevel}
          onChange={(e) =>
            updateField('confidenceLevel', e.target.value as ConfidenceLevel)
          }
        >
          <option value="">Select confidence...</option>
          {CONFIDENCE_LEVELS.map((cl) => (
            <option key={cl.value} value={cl.value}>
              {cl.label}
            </option>
          ))}
        </select>
      </div>

      {/* Limitations */}
      <div>
        <label className="field-label" htmlFor="vf-limitations">Limitations</label>
        <textarea
          className="input-field"
          rows={2}
          placeholder="Note any limitations of this verification..."
          value={form.limitations}
          onChange={(e) => updateField('limitations', e.target.value)}
        />
      </div>

      {/* Caveats */}
      <div>
        <label className="field-label" htmlFor="vf-caveats">Caveats</label>
        <input
          type="text"
          className="input-field"
          placeholder="e.g. Low resolution source, Partial metadata (comma-separated)"
          value={form.caveats}
          onChange={(e) => updateField('caveats', e.target.value)}
        />
        {form.caveats && (
          <div
            style={{
              display: 'flex',
              flexWrap: 'wrap',
              gap: 'var(--space-xs)',
              marginTop: 'var(--space-xs)',
            }}
          >
            {parseTags(form.caveats).map((tag) => (
              <span
                key={tag}
                className="badge"
                style={{
                  backgroundColor: 'var(--bg-inset)',
                  color: 'var(--text-secondary)',
                  border: '1px solid var(--border-subtle)',
                }}
              >
                {tag}
              </span>
            ))}
          </div>
        )}
      </div>

      {/* Auto-upgrade note */}
      <div
        style={{
          fontSize: 'var(--text-xs)',
          color: 'var(--text-tertiary)',
          backgroundColor: 'var(--bg-inset)',
          border: '1px solid var(--border-subtle)',
          borderRadius: 'var(--radius-md)',
          padding: 'var(--space-sm) var(--space-md)',
          lineHeight: 1.5,
        }}
      >
        When finding is &lsquo;Authentic&rsquo; with &lsquo;High&rsquo;
        confidence, capture metadata verification status will be auto-upgraded
        to &lsquo;Verified&rsquo;.
      </div>

      {/* Submit */}
      <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
        <button
          className="btn-primary"
          disabled={!isValid || saving}
          onClick={handleSave}
        >
          {saving ? 'Saving...' : 'Save Verification'}
        </button>
      </div>
    </div>
  );
}
