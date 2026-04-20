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

const PROTECTION_PILL: Record<string, { cls: string }> = {
  standard: { cls: 'pl sealed' },
  protected: { cls: 'pl hold' },
  high_risk: { cls: 'pl broken' },
};

const labelStyle: React.CSSProperties = {
  fontFamily: '"JetBrains Mono", monospace',
  fontSize: '10.5px',
  letterSpacing: '.08em',
  textTransform: 'uppercase',
  color: 'var(--muted)',
  marginBottom: '6px',
  display: 'block',
};

const inputStyle: React.CSSProperties = {
  padding: '11px 16px',
  borderRadius: 'var(--radius-sm)',
  border: '1px solid var(--line-2)',
  background: 'var(--paper)',
  font: 'inherit',
  fontSize: '14px',
  color: 'var(--ink)',
  outline: 'none',
  width: '100%',
  boxSizing: 'border-box',
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

  const protPill = PROTECTION_PILL[formData.protection_status] || PROTECTION_PILL.standard;

  return (
    <form onSubmit={handleSubmit}>
      {error && <div className="banner-error" style={{ marginBottom: '16px' }}>{error}</div>}

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 22rem', gap: '22px' }}>
        {/* Left: main form */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
          {/* Top row: code + protection */}
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 12rem', gap: '16px' }}>
            <div>
              <label htmlFor="wf-code" style={labelStyle}>Witness code</label>
              <input
                id="wf-code"
                type="text"
                value={formData.witness_code}
                onChange={(e) => setFormData({ ...formData, witness_code: e.target.value })}
                placeholder="e.g. W-001"
                style={inputStyle}
                required
              />
            </div>
            <div>
              <label htmlFor="wf-protection" style={labelStyle}>Protection</label>
              <div style={{ position: 'relative' }}>
                <select
                  id="wf-protection"
                  value={formData.protection_status}
                  onChange={(e) => setFormData({ ...formData, protection_status: e.target.value })}
                  style={{ ...inputStyle, paddingLeft: '28px' }}
                >
                  <option value="standard">Standard</option>
                  <option value="protected">Protected</option>
                  <option value="high_risk">High Risk</option>
                </select>
                <span
                  className={protPill.cls}
                  style={{
                    position: 'absolute',
                    left: '10px',
                    top: '50%',
                    transform: 'translateY(-50%)',
                    width: '7px',
                    height: '7px',
                    padding: 0,
                    borderRadius: '50%',
                    fontSize: 0,
                    lineHeight: 0,
                    minWidth: 'unset',
                  }}
                />
              </div>
            </div>
          </div>

          {/* Identity section */}
          <div className="panel" style={{ borderLeft: '2px solid var(--accent)' }}>
            <div className="panel-h" style={{ padding: '14px 18px' }}>
              <h3 style={{ fontSize: '14px' }}>Identity</h3>
            </div>
            <div className="panel-body" style={{ padding: '18px' }}>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px' }}>
                <div>
                  <label htmlFor="wf-name" style={labelStyle}>Full name</label>
                  <input
                    id="wf-name"
                    type="text"
                    value={formData.full_name || ''}
                    onChange={(e) => setFormData({ ...formData, full_name: e.target.value })}
                    style={inputStyle}
                  />
                </div>
                <div>
                  <label htmlFor="wf-contact" style={labelStyle}>Contact</label>
                  <input
                    id="wf-contact"
                    type="text"
                    value={formData.contact_info || ''}
                    onChange={(e) => setFormData({ ...formData, contact_info: e.target.value })}
                    style={inputStyle}
                  />
                </div>
                <div style={{ gridColumn: '1 / -1' }}>
                  <label htmlFor="wf-location" style={labelStyle}>Location</label>
                  <input
                    id="wf-location"
                    type="text"
                    value={formData.location || ''}
                    onChange={(e) => setFormData({ ...formData, location: e.target.value })}
                    style={inputStyle}
                  />
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Right: sidebar */}
        <aside style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
          {/* Related evidence */}
          <div className="panel">
            <div className="panel-h" style={{ padding: '14px 18px' }}>
              <h3 style={{ fontSize: '14px' }}>Related evidence</h3>
              {formData.related_evidence.length > 0 && (
                <span className="meta">{formData.related_evidence.length} selected</span>
              )}
            </div>
            <div className="panel-body" style={{ padding: '12px 18px', maxHeight: '256px', overflowY: 'auto' }}>
              {evidence.length === 0 ? (
                <p style={{ fontSize: '12px', color: 'var(--muted)', margin: 0 }}>
                  No evidence uploaded to this case yet.
                </p>
              ) : (
                <div style={{ display: 'flex', flexDirection: 'column', gap: '2px' }}>
                  {evidence.map((item) => {
                    const checked = formData.related_evidence.includes(item.id);
                    return (
                      <label
                        key={item.id}
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          gap: '10px',
                          padding: '6px 8px',
                          borderRadius: '6px',
                          cursor: 'pointer',
                          background: checked ? 'rgba(184,66,28,.06)' : 'transparent',
                          transition: 'background .12s',
                        }}
                      >
                        <input
                          type="checkbox"
                          checked={checked}
                          onChange={() => toggleEvidence(item.id)}
                          style={{ accentColor: 'var(--accent)' }}
                        />
                        <span style={{ fontSize: '12px', color: 'var(--ink-2)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                          <span className="tag" style={{ marginRight: '6px' }}>{item.evidence_number}</span>
                          {item.original_name}
                        </span>
                      </label>
                    );
                  })}
                </div>
              )}
            </div>
          </div>
        </aside>
      </div>

      {/* Statement summary -- full width below grid */}
      <div style={{ marginTop: '20px' }}>
        <label htmlFor="wf-statement" style={labelStyle}>Statement summary</label>
        <textarea
          id="wf-statement"
          value={formData.statement_summary}
          onChange={(e) => setFormData({ ...formData, statement_summary: e.target.value })}
          rows={5}
          style={{ ...inputStyle, resize: 'vertical' }}
        />
      </div>

      {/* Actions */}
      <div style={{ display: 'flex', gap: '10px', justifyContent: 'flex-end', marginTop: '24px' }}>
        <button type="button" onClick={onCancel} className="btn ghost">
          Cancel
        </button>
        <button type="submit" disabled={saving} className="btn">
          {saving ? 'Saving\u2026' : witness ? 'Update Witness' : 'Create Witness'}
        </button>
      </div>
    </form>
  );
}
