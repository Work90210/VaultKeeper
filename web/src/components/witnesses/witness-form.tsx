'use client';

import { useState } from 'react';
import type { Witness, EvidenceItem } from '@/types';

interface WitnessFormProps {
  caseId: string;
  witness?: Witness;
  evidence: readonly EvidenceItem[];
  accessToken: string;
  onSave: (data: WitnessFormData) => Promise<void>;
  onCancel: () => void;
}

export interface WitnessFormData {
  witness_code: string;
  full_name?: string;
  contact_info?: string;
  location?: string;
  protection_status: string;
  statement_summary: string;
  related_evidence: string[];
}

const PROTECTION_STYLES: Record<string, { color: string; bg: string }> = {
  standard: { color: 'var(--status-active)', bg: 'var(--status-active-bg)' },
  protected: { color: 'var(--status-closed)', bg: 'var(--status-closed-bg)' },
  high_risk: { color: 'var(--status-hold)', bg: 'var(--status-hold-bg)' },
};

export function WitnessForm({
  witness,
  evidence,
  onSave,
  onCancel,
}: WitnessFormProps) {
  const [formData, setFormData] = useState<WitnessFormData>({
    witness_code: witness?.witness_code || '',
    full_name: witness?.full_name || '',
    contact_info: witness?.contact_info || '',
    location: witness?.location || '',
    protection_status: witness?.protection_status || 'standard',
    statement_summary: witness?.statement_summary || '',
    related_evidence: witness?.related_evidence || [],
  });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      await onSave(formData);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save witness');
    } finally {
      setSaving(false);
    }
  };

  const toggleEvidence = (evidenceId: string) => {
    setFormData((prev) => ({
      ...prev,
      related_evidence: prev.related_evidence.includes(evidenceId)
        ? prev.related_evidence.filter((id) => id !== evidenceId)
        : [...prev.related_evidence, evidenceId],
    }));
  };

  const protStyle = PROTECTION_STYLES[formData.protection_status] || PROTECTION_STYLES.standard;

  return (
    <form onSubmit={handleSubmit}>
      {error && <div className="banner-error mb-[var(--space-md)]">{error}</div>}

      <div className="grid grid-cols-1 lg:grid-cols-[1fr_22rem] gap-[var(--space-lg)]">
        {/* ── Left: main form ── */}
        <div className="space-y-[var(--space-md)]">
          {/* Top row: code + protection */}
          <div className="grid grid-cols-1 sm:grid-cols-[1fr_12rem] gap-[var(--space-md)]">
            <div>
              <label className="field-label" htmlFor="wf-code">Witness code</label>
              <input
                id="wf-code"
                type="text"
                value={formData.witness_code}
                onChange={(e) => setFormData({ ...formData, witness_code: e.target.value })}
                placeholder="e.g. W-001"
                className="input-field"
                required
              />
            </div>
            <div>
              <label className="field-label" htmlFor="wf-protection">Protection</label>
              <div className="relative">
                <select
                  id="wf-protection"
                  value={formData.protection_status}
                  onChange={(e) => setFormData({ ...formData, protection_status: e.target.value })}
                  className="input-field"
                  style={{ paddingLeft: 'calc(var(--space-sm) + 1rem)' }}
                >
                  <option value="standard">Standard</option>
                  <option value="protected">Protected</option>
                  <option value="high_risk">High Risk</option>
                </select>
                <span
                  className="absolute left-[var(--space-sm)] top-1/2 -translate-y-1/2 w-2 h-2 rounded-full"
                  style={{ backgroundColor: protStyle.color }}
                />
              </div>
            </div>
          </div>

          {/* Identity section */}
          <div
            className="card-inset p-[var(--space-md)] space-y-[var(--space-sm)]"
            style={{ borderLeft: '2px solid var(--amber-accent)' }}
          >
            <p
              className="text-xs font-semibold uppercase tracking-wider"
              style={{ color: 'var(--amber-accent)' }}
            >
              Identity
            </p>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-[var(--space-sm)]">
              <div>
                <label className="field-label" htmlFor="wf-name">Full name</label>
                <input
                  id="wf-name"
                  type="text"
                  value={formData.full_name || ''}
                  onChange={(e) => setFormData({ ...formData, full_name: e.target.value })}
                  className="input-field"
                />
              </div>
              <div>
                <label className="field-label" htmlFor="wf-contact">Contact</label>
                <input
                  id="wf-contact"
                  type="text"
                  value={formData.contact_info || ''}
                  onChange={(e) => setFormData({ ...formData, contact_info: e.target.value })}
                  className="input-field"
                />
              </div>
              <div className="sm:col-span-2">
                <label className="field-label" htmlFor="wf-location">Location</label>
                <input
                  id="wf-location"
                  type="text"
                  value={formData.location || ''}
                  onChange={(e) => setFormData({ ...formData, location: e.target.value })}
                  className="input-field"
                />
              </div>
            </div>
          </div>
        </div>

        {/* ── Right: sidebar ── */}
        <aside className="space-y-[var(--space-md)]">
          {/* Related evidence */}
          {evidence.length > 0 && (
            <div className="card-inset p-[var(--space-md)]">
              <h3 className="field-label" style={{ marginBottom: 'var(--space-sm)' }}>
                Related evidence
              </h3>
              <div className="max-h-64 overflow-y-auto space-y-px">
                {evidence.map((item) => {
                  const checked = formData.related_evidence.includes(item.id);
                  return (
                    <label
                      key={item.id}
                      className="flex items-center gap-[var(--space-sm)] p-[var(--space-xs)] rounded-[var(--radius-sm)] cursor-pointer transition-colors"
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
                      <span className="truncate text-xs" style={{ color: 'var(--text-secondary)' }}>
                        <span
                          className="font-[family-name:var(--font-mono)]"
                          style={{ color: 'var(--text-tertiary)' }}
                        >
                          {item.evidence_number}
                        </span>
                        {' '}
                        {item.original_name}
                      </span>
                    </label>
                  );
                })}
              </div>
              {formData.related_evidence.length > 0 && (
                <p
                  className="text-xs mt-[var(--space-sm)]"
                  style={{ color: 'var(--text-tertiary)' }}
                >
                  {formData.related_evidence.length} selected
                </p>
              )}
            </div>
          )}

          {evidence.length === 0 && (
            <div className="card-inset p-[var(--space-md)]">
              <h3 className="field-label" style={{ marginBottom: 'var(--space-xs)' }}>
                Related evidence
              </h3>
              <p className="text-xs" style={{ color: 'var(--text-tertiary)' }}>
                No evidence uploaded to this case yet.
              </p>
            </div>
          )}
        </aside>
      </div>

      {/* Statement summary — full width below grid */}
      <div className="mt-[var(--space-md)]">
        <label className="field-label" htmlFor="wf-statement">Statement summary</label>
        <textarea
          id="wf-statement"
          value={formData.statement_summary}
          onChange={(e) => setFormData({ ...formData, statement_summary: e.target.value })}
          rows={5}
          className="input-field resize-y"
        />
      </div>

      {/* Actions */}
      <div className="flex gap-[var(--space-sm)] justify-end mt-[var(--space-lg)]">
        <button type="button" onClick={onCancel} className="btn-ghost">
          Cancel
        </button>
        <button type="submit" disabled={saving} className="btn-primary">
          {saving ? 'Saving\u2026' : witness ? 'Update Witness' : 'Create Witness'}
        </button>
      </div>
    </form>
  );
}
