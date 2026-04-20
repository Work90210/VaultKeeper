'use client';

import { useState } from 'react';
import {
  ANALYSIS_TYPES,
  type AnalysisType,
  type AnalysisStatus,
} from '@/types';
import { EvidencePicker } from '@/components/investigation/evidence-picker';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

const STATUS_COLORS: Record<AnalysisStatus, string> = {
  draft: 'var(--status-archived)',
  in_review: 'var(--amber-accent)',
  approved: 'var(--status-active)',
  superseded: 'var(--status-hold)',
};

const STATUS_BG: Record<AnalysisStatus, string> = {
  draft: 'var(--status-archived-bg)',
  in_review: 'var(--amber-subtle)',
  approved: 'var(--status-active-bg)',
  superseded: 'var(--status-hold-bg)',
};

interface FormState {
  readonly title: string;
  readonly analysisType: AnalysisType;
  readonly content: string;
  readonly methodology: string;
  readonly relatedEvidenceIds: readonly string[];
  readonly relatedInquiryIds: string;
  readonly relatedAssessmentIds: string;
  readonly relatedVerificationIds: string;
}

function defaultForm(): FormState {
  return {
    title: '',
    analysisType: 'factual_finding',
    content: '',
    methodology: '',
    relatedEvidenceIds: [],
    relatedInquiryIds: '',
    relatedAssessmentIds: '',
    relatedVerificationIds: '',
  };
}

function parseIds(raw: string | readonly string[]): string[] {
  if (Array.isArray(raw)) return [...raw];
  return (raw as string)
    .split(',')
    .map((s) => s.trim())
    .filter((s) => s.length > 0);
}

export function AnalysisNoteEditor({
  caseId,
  accessToken,
  onSaved,
}: {
  caseId: string;
  accessToken: string;
  onSaved: () => void;
}) {
  const [form, setForm] = useState<FormState>(defaultForm);
  const [status] = useState<AnalysisStatus>('draft');
  const [relatedOpen, setRelatedOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const updateField = <K extends keyof FormState>(
    key: K,
    value: FormState[K],
  ) => {
    setForm((prev) => ({ ...prev, [key]: value }));
  };

  const handleSave = async () => {
    if (!form.title.trim() || !form.content.trim()) {
      setError('Title and content are required.');
      return;
    }

    setSaving(true);
    setError(null);

    try {
      const body = {
        title: form.title.trim(),
        analysis_type: form.analysisType,
        content: form.content,
        methodology: form.methodology || undefined,
        related_evidence_ids: parseIds(form.relatedEvidenceIds),
        related_inquiry_ids: parseIds(form.relatedInquiryIds),
        related_assessment_ids: parseIds(form.relatedAssessmentIds),
        related_verification_ids: parseIds(form.relatedVerificationIds),
      };

      const res = await fetch(
        `${API_BASE}/api/cases/${caseId}/analysis-notes`,
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
          data?.error || `Failed to save analysis note (${res.status})`,
        );
      }

      onSaved();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'An error occurred.');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="card" style={{ padding: 'var(--space-lg)' }}>
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          marginBottom: 'var(--space-lg)',
        }}
      >
        <h2
          style={{
            fontSize: 'var(--text-xl)',
            fontWeight: 600,
            color: 'var(--text-primary)',
            margin: 0,
          }}
        >
          New Analysis Note
        </h2>
        <span
          className="badge"
          style={{
            color: STATUS_COLORS[status],
            backgroundColor: STATUS_BG[status],
          }}
        >
          {status.replace('_', ' ')}
        </span>
      </div>

      {error && (
        <div className="banner-error" style={{ marginBottom: 'var(--space-md)' }}>
          {error}
        </div>
      )}

      <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-md)' }}>
        {/* Title */}
        <div>
          <label className="field-label" htmlFor="an-title">
            Title *
          </label>
          <input
            id="an-title"
            type="text"
            className="input-field"
            placeholder="Analysis note title"
            value={form.title}
            onChange={(e) => updateField('title', e.target.value)}
          />
        </div>

        {/* Analysis Type */}
        <div>
          <label className="field-label" htmlFor="an-type">
            Analysis Type
          </label>
          <select
            id="an-type"
            className="input-field"
            value={form.analysisType}
            onChange={(e) =>
              updateField('analysisType', e.target.value as AnalysisType)
            }
          >
            {ANALYSIS_TYPES.map((t) => (
              <option key={t.value} value={t.value}>
                {t.label}
              </option>
            ))}
          </select>
        </div>

        {/* Content */}
        <div>
          <label className="field-label" htmlFor="an-content">
            Content *
          </label>
          <textarea
            id="an-content"
            className="input-field"
            rows={12}
            placeholder="Write your analysis here..."
            value={form.content}
            onChange={(e) => updateField('content', e.target.value)}
            style={{ resize: 'vertical' }}
          />
        </div>

        {/* Methodology */}
        <div>
          <label className="field-label" htmlFor="an-methodology">
            Methodology
          </label>
          <textarea
            id="an-methodology"
            className="input-field"
            rows={4}
            placeholder="Describe the methodology used..."
            value={form.methodology}
            onChange={(e) => updateField('methodology', e.target.value)}
            style={{ resize: 'vertical' }}
          />
        </div>

        {/* Related Items — Collapsible */}
        <div
          className="card-inset"
          style={{ padding: 'var(--space-md)', marginTop: 'var(--space-xs)' }}
        >
          <button
            type="button"
            onClick={() => setRelatedOpen((prev) => !prev)}
            style={{
              all: 'unset',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: 'var(--space-sm)',
              width: '100%',
              fontSize: 'var(--text-sm)',
              fontWeight: 600,
              color: 'var(--text-secondary)',
              letterSpacing: '0.04em',
              textTransform: 'uppercase',
            }}
          >
            <span
              style={{
                display: 'inline-block',
                transform: relatedOpen ? 'rotate(90deg)' : 'rotate(0deg)',
                transition: 'transform var(--duration-fast) ease',
              }}
            >
              &#9654;
            </span>
            Related Items
          </button>

          {relatedOpen && (
            <div
              style={{
                display: 'flex',
                flexDirection: 'column',
                gap: 'var(--space-md)',
                marginTop: 'var(--space-md)',
              }}
            >
              <EvidencePicker
                caseId={caseId}
                accessToken={accessToken}
                selectedIds={form.relatedEvidenceIds}
                onSelect={(ids) => updateField('relatedEvidenceIds', ids)}
                label="Related Evidence"
              />
              <div>
                <label className="field-label" htmlFor="an-rel-inquiry">
                  Inquiry IDs (comma-separated)
                </label>
                <input
                  id="an-rel-inquiry"
                  type="text"
                  className="input-field"
                  placeholder="uuid-1, uuid-2, ..."
                  value={form.relatedInquiryIds}
                  onChange={(e) =>
                    updateField('relatedInquiryIds', e.target.value)
                  }
                />
              </div>
              <div>
                <label className="field-label" htmlFor="an-rel-assessment">
                  Assessment IDs (comma-separated)
                </label>
                <input
                  id="an-rel-assessment"
                  type="text"
                  className="input-field"
                  placeholder="uuid-1, uuid-2, ..."
                  value={form.relatedAssessmentIds}
                  onChange={(e) =>
                    updateField('relatedAssessmentIds', e.target.value)
                  }
                />
              </div>
              <div>
                <label className="field-label" htmlFor="an-rel-verification">
                  Verification IDs (comma-separated)
                </label>
                <input
                  id="an-rel-verification"
                  type="text"
                  className="input-field"
                  placeholder="uuid-1, uuid-2, ..."
                  value={form.relatedVerificationIds}
                  onChange={(e) =>
                    updateField('relatedVerificationIds', e.target.value)
                  }
                />
              </div>
            </div>
          )}
        </div>

        {/* Save */}
        <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 'var(--space-sm)' }}>
          <button
            type="button"
            className="btn-primary"
            disabled={saving}
            onClick={handleSave}
          >
            {saving ? 'Saving...' : 'Save Analysis Note'}
          </button>
        </div>
      </div>
    </div>
  );
}
