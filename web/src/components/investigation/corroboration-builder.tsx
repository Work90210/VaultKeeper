'use client';

import { useState, useCallback } from 'react';
import type { ClaimType, ClaimStrength, RoleInClaim } from '@/types';
import { CLAIM_TYPES, CLAIM_STRENGTHS } from '@/types';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

const ROLE_OPTIONS: { value: RoleInClaim; label: string }[] = [
  { value: 'primary', label: 'Primary' },
  { value: 'supporting', label: 'Supporting' },
  { value: 'contextual', label: 'Contextual' },
  { value: 'contradicting', label: 'Contradicting' },
];

interface EvidenceOption {
  readonly id: string;
  readonly evidence_number: string;
  readonly title: string;
}

interface LinkedEvidence {
  readonly evidenceId: string;
  readonly evidenceNumber: string;
  readonly title: string;
  readonly role: RoleInClaim;
  readonly contributionNotes: string;
}

interface FormState {
  readonly claimSummary: string;
  readonly claimType: ClaimType | '';
  readonly strength: ClaimStrength | '';
  readonly analysisNotes: string;
}

interface CorroborationBuilderProps {
  readonly caseId: string;
  readonly evidenceItems: readonly EvidenceOption[];
  readonly accessToken: string;
  readonly onSaved: () => void;
}

const INITIAL_FORM: FormState = {
  claimSummary: '',
  claimType: '',
  strength: '',
  analysisNotes: '',
};

export function CorroborationBuilder({
  caseId,
  evidenceItems,
  accessToken,
  onSaved,
}: CorroborationBuilderProps) {
  const [form, setForm] = useState<FormState>(INITIAL_FORM);
  const [linkedEvidence, setLinkedEvidence] = useState<readonly LinkedEvidence[]>(
    [],
  );
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [addingEvidence, setAddingEvidence] = useState(false);
  const [selectedEvidenceId, setSelectedEvidenceId] = useState('');

  const updateField = useCallback(
    <K extends keyof FormState>(field: K, value: FormState[K]) => {
      setForm((prev) => ({ ...prev, [field]: value }));
    },
    [],
  );

  const availableEvidence = evidenceItems.filter(
    (ei) => !linkedEvidence.some((le) => le.evidenceId === ei.id),
  );

  const addEvidenceItem = useCallback(() => {
    if (!selectedEvidenceId) return;
    const item = evidenceItems.find((ei) => ei.id === selectedEvidenceId);
    if (!item) return;

    setLinkedEvidence((prev) => [
      ...prev,
      {
        evidenceId: item.id,
        evidenceNumber: item.evidence_number,
        title: item.title,
        role: 'supporting' as RoleInClaim,
        contributionNotes: '',
      },
    ]);
    setSelectedEvidenceId('');
    setAddingEvidence(false);
  }, [selectedEvidenceId, evidenceItems]);

  const removeEvidenceItem = useCallback((evidenceId: string) => {
    setLinkedEvidence((prev) =>
      prev.filter((le) => le.evidenceId !== evidenceId),
    );
  }, []);

  const updateLinkedEvidence = useCallback(
    (evidenceId: string, patch: Partial<Pick<LinkedEvidence, 'role' | 'contributionNotes'>>) => {
      setLinkedEvidence((prev) =>
        prev.map((le) =>
          le.evidenceId === evidenceId ? { ...le, ...patch } : le,
        ),
      );
    },
    [],
  );

  const isValid =
    form.claimSummary.trim().length > 0 &&
    form.claimType !== '' &&
    form.strength !== '' &&
    linkedEvidence.length >= 2;

  const handleSave = useCallback(async () => {
    if (!isValid) return;

    setSaving(true);
    setError(null);

    const body = {
      claim_summary: form.claimSummary.trim(),
      claim_type: form.claimType,
      strength: form.strength,
      analysis_notes: form.analysisNotes.trim() || undefined,
      evidence: linkedEvidence.map((le) => ({
        evidence_id: le.evidenceId,
        role_in_claim: le.role,
        contribution_notes: le.contributionNotes.trim() || undefined,
      })),
    };

    try {
      const res = await fetch(
        `${API_BASE}/api/cases/${caseId}/corroborations`,
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
      setLinkedEvidence([]);
      onSaved();
    } catch (err) {
      setError(
        err instanceof Error ? err.message : 'Failed to save corroboration',
      );
    } finally {
      setSaving(false);
    }
  }, [isValid, form, linkedEvidence, caseId, accessToken, onSaved]);

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
        New Corroboration Claim
      </h3>

      {error && <div className="banner-error">{error}</div>}

      {/* Claim Summary */}
      <div>
        <label className="field-label" htmlFor="cb-claim-summary">Claim Summary *</label>
        <textarea
          id="cb-claim-summary"
          className="input-field"
          rows={3}
          placeholder="Summarize the corroboration claim..."
          value={form.claimSummary}
          onChange={(e) => updateField('claimSummary', e.target.value)}
        />
      </div>

      {/* Claim Type */}
      <div>
        <label className="field-label" htmlFor="cb-claim-type">Claim Type *</label>
        <select
          id="cb-claim-type"
          className="input-field"
          value={form.claimType}
          onChange={(e) =>
            updateField('claimType', e.target.value as ClaimType)
          }
        >
          <option value="">Select claim type...</option>
          {CLAIM_TYPES.map((ct) => (
            <option key={ct.value} value={ct.value}>
              {ct.label}
            </option>
          ))}
        </select>
      </div>

      {/* Strength */}
      <div>
        <label className="field-label" htmlFor="cb-strength">Strength *</label>
        <select
          id="cb-strength"
          className="input-field"
          value={form.strength}
          onChange={(e) =>
            updateField('strength', e.target.value as ClaimStrength)
          }
        >
          <option value="">Select strength...</option>
          {CLAIM_STRENGTHS.map((cs) => (
            <option key={cs.value} value={cs.value}>
              {cs.label}
            </option>
          ))}
        </select>
      </div>

      {/* Analysis Notes */}
      <div>
        <label className="field-label" htmlFor="cb-analysis-notes">Analysis Notes</label>
        <textarea
          id="cb-analysis-notes"
          className="input-field"
          rows={3}
          placeholder="Additional analysis notes..."
          value={form.analysisNotes}
          onChange={(e) => updateField('analysisNotes', e.target.value)}
        />
      </div>

      {/* Evidence Linking */}
      <div>
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            marginBottom: 'var(--space-sm)',
          }}
        >
          <label className="field-label" style={{ marginBottom: 0 }}>
            Linked Evidence ({linkedEvidence.length}/2 minimum)
          </label>
          {availableEvidence.length > 0 && (
            <button
              className="btn-secondary"
              onClick={() => setAddingEvidence(true)}
              style={{ fontSize: 'var(--text-xs)' }}
            >
              + Add Evidence Item
            </button>
          )}
        </div>

        {/* Add evidence selector */}
        {addingEvidence && (
          <div
            className="card-inset"
            style={{
              padding: 'var(--space-sm) var(--space-md)',
              marginBottom: 'var(--space-sm)',
              display: 'flex',
              gap: 'var(--space-sm)',
              alignItems: 'center',
            }}
          >
            <select
              className="input-field"
              style={{ flex: 1 }}
              value={selectedEvidenceId}
              onChange={(e) => setSelectedEvidenceId(e.target.value)}
            >
              <option value="">Select evidence item...</option>
              {availableEvidence.map((ei) => (
                <option key={ei.id} value={ei.id}>
                  {ei.evidence_number} — {ei.title}
                </option>
              ))}
            </select>
            <button
              className="btn-primary"
              disabled={!selectedEvidenceId}
              onClick={addEvidenceItem}
              style={{ whiteSpace: 'nowrap' }}
            >
              Add
            </button>
            <button
              className="btn-ghost"
              onClick={() => {
                setAddingEvidence(false);
                setSelectedEvidenceId('');
              }}
            >
              Cancel
            </button>
          </div>
        )}

        {/* Linked evidence list */}
        {linkedEvidence.length === 0 && (
          <div
            className="card-inset"
            style={{
              padding: 'var(--space-md)',
              textAlign: 'center',
              color: 'var(--text-tertiary)',
              fontSize: 'var(--text-sm)',
            }}
          >
            No evidence items linked yet. Add at least 2 to create a
            corroboration claim.
          </div>
        )}

        <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-sm)' }}>
          {linkedEvidence.map((le) => (
            <div
              key={le.evidenceId}
              className="card-inset"
              style={{ padding: 'var(--space-sm) var(--space-md)' }}
            >
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  marginBottom: 'var(--space-xs)',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-sm)' }}>
                  <span
                    className="badge"
                    style={{
                      backgroundColor: 'var(--amber-subtle)',
                      color: 'var(--amber-accent)',
                      fontFamily: 'monospace',
                    }}
                  >
                    {le.evidenceNumber}
                  </span>
                  <span
                    style={{
                      fontSize: 'var(--text-sm)',
                      fontWeight: 500,
                      color: 'var(--text-primary)',
                    }}
                  >
                    {le.title}
                  </span>
                </div>
                <button
                  className="btn-ghost"
                  onClick={() => removeEvidenceItem(le.evidenceId)}
                  style={{
                    color: 'var(--status-hold)',
                    fontSize: 'var(--text-sm)',
                    padding: 'var(--space-xs)',
                    lineHeight: 1,
                  }}
                  aria-label={`Remove ${le.evidenceNumber}`}
                >
                  X
                </button>
              </div>

              <div
                style={{
                  display: 'grid',
                  gridTemplateColumns: '1fr 2fr',
                  gap: 'var(--space-sm)',
                  alignItems: 'start',
                }}
              >
                <div>
                  <label className="field-label">Role</label>
                  <select
                    className="input-field"
                    value={le.role}
                    onChange={(e) =>
                      updateLinkedEvidence(le.evidenceId, {
                        role: e.target.value as RoleInClaim,
                      })
                    }
                  >
                    {ROLE_OPTIONS.map((r) => (
                      <option key={r.value} value={r.value}>
                        {r.label}
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="field-label">Contribution Notes</label>
                  <input
                    type="text"
                    className="input-field"
                    placeholder="How this evidence supports the claim..."
                    value={le.contributionNotes}
                    onChange={(e) =>
                      updateLinkedEvidence(le.evidenceId, {
                        contributionNotes: e.target.value,
                      })
                    }
                  />
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Submit */}
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
        }}
      >
        {linkedEvidence.length < 2 && linkedEvidence.length > 0 && (
          <span
            style={{
              fontSize: 'var(--text-xs)',
              color: 'var(--status-closed)',
            }}
          >
            Add at least {2 - linkedEvidence.length} more evidence{' '}
            {2 - linkedEvidence.length === 1 ? 'item' : 'items'}
          </span>
        )}
        <div style={{ marginLeft: 'auto' }}>
          <button
            className="btn-primary"
            disabled={!isValid || saving}
            onClick={handleSave}
          >
            {saving ? 'Saving...' : 'Save Corroboration'}
          </button>
        </div>
      </div>
    </div>
  );
}
