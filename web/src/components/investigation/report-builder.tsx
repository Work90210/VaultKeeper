'use client';

import { useState } from 'react';
import { REPORT_TYPES, type ReportType } from '@/types';
import { EvidencePicker } from '@/components/investigation/evidence-picker';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

const SECTION_TYPES = [
  { value: 'purpose', label: 'Purpose' },
  { value: 'methodology', label: 'Methodology' },
  { value: 'findings', label: 'Findings' },
  { value: 'evidence_summary', label: 'Evidence Summary' },
  { value: 'analysis', label: 'Analysis' },
  { value: 'conclusions', label: 'Conclusions' },
  { value: 'recommendations', label: 'Recommendations' },
  { value: 'limitations', label: 'Limitations' },
  { value: 'appendix', label: 'Appendix' },
  { value: 'custom', label: 'Custom' },
] as const;

interface ReportSectionDraft {
  readonly id: string;
  readonly sectionType: string;
  readonly title: string;
  readonly content: string;
}

interface FormState {
  readonly title: string;
  readonly reportType: ReportType;
  readonly sections: readonly ReportSectionDraft[];
  readonly limitations: readonly string[];
  readonly caveats: readonly string[];
  readonly assumptions: readonly string[];
  readonly referencedEvidenceIds: readonly string[];
}

let sectionCounter = 0;
function nextSectionId(): string {
  sectionCounter += 1;
  return `sec-${sectionCounter}`;
}

function defaultForm(): FormState {
  return {
    title: '',
    reportType: 'interim',
    sections: [],
    limitations: [],
    caveats: [],
    assumptions: [],
    referencedEvidenceIds: [],
  };
}

function TagInput({
  label,
  tags,
  onChange,
}: {
  label: string;
  tags: readonly string[];
  onChange: (tags: readonly string[]) => void;
}) {
  const [input, setInput] = useState('');

  const addTag = () => {
    const value = input.trim();
    if (value && !tags.includes(value)) {
      onChange([...tags, value]);
    }
    setInput('');
  };

  const removeTag = (idx: number) => {
    onChange(tags.filter((_, i) => i !== idx));
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      addTag();
    }
  };

  return (
    <div>
      <label style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: '10.5px', letterSpacing: '.06em', textTransform: 'uppercase', color: 'var(--muted)', marginBottom: 6, display: 'block' }}>
        {label}
      </label>
      <div style={{ display: 'flex', gap: 8, marginBottom: 8 }}>
        <input
          type="text"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Type and press Enter"
          style={{
            flex: 1,
            padding: '8px 12px',
            fontSize: '13.5px',
            border: '1px solid var(--line)',
            borderRadius: 6,
            background: 'var(--paper)',
            color: 'var(--ink)',
          }}
        />
        <button type="button" className="btn ghost" onClick={addTag}>Add</button>
      </div>
      {tags.length > 0 && (
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
          {tags.map((tag, idx) => (
            <span key={tag + idx} className="tag" style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}>
              {tag}
              <button
                type="button"
                onClick={() => removeTag(idx)}
                style={{ all: 'unset', cursor: 'pointer', fontWeight: 700, fontSize: 11, lineHeight: 1, color: 'var(--muted)' }}
                aria-label={`Remove ${tag}`}
              >
                &times;
              </button>
            </span>
          ))}
        </div>
      )}
    </div>
  );
}

export function ReportBuilder({
  caseId,
  accessToken,
  onSaved,
  existingReport,
}: {
  caseId: string;
  accessToken: string;
  onSaved: () => void;
  existingReport?: {
    id: string;
    title: string;
    report_type: string;
    sections: { section_type: string; title: string; content: string; order: number }[];
    limitations: string[];
    caveats: string[];
    assumptions: string[];
  };
}) {
  const [form, setForm] = useState<FormState>(() => {
    if (existingReport) {
      return {
        title: existingReport.title,
        reportType: existingReport.report_type as ReportType,
        sections: existingReport.sections.map((s) => ({
          id: nextSectionId(),
          sectionType: s.section_type,
          title: s.title,
          content: s.content,
        })),
        limitations: existingReport.limitations,
        caveats: existingReport.caveats,
        assumptions: existingReport.assumptions,
        referencedEvidenceIds: [],
      };
    }
    return defaultForm();
  });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const updateField = <K extends keyof FormState>(
    key: K,
    value: FormState[K],
  ) => {
    setForm((prev) => ({ ...prev, [key]: value }));
  };

  const addSection = () => {
    const section: ReportSectionDraft = {
      id: nextSectionId(),
      sectionType: 'purpose',
      title: '',
      content: '',
    };
    updateField('sections', [...form.sections, section]);
  };

  const updateSection = (
    id: string,
    patch: Partial<Omit<ReportSectionDraft, 'id'>>,
  ) => {
    updateField(
      'sections',
      form.sections.map((s) => (s.id === id ? { ...s, ...patch } : s)),
    );
  };

  const removeSection = (id: string) => {
    updateField(
      'sections',
      form.sections.filter((s) => s.id !== id),
    );
  };

  const moveSection = (idx: number, direction: -1 | 1) => {
    const target = idx + direction;
    if (target < 0 || target >= form.sections.length) return;
    const arr = [...form.sections];
    const temp = arr[idx];
    arr[idx] = arr[target];
    arr[target] = temp;
    updateField('sections', arr);
  };

  const handleSave = async () => {
    if (!form.title.trim()) {
      setError('Title is required.');
      return;
    }

    setSaving(true);
    setError(null);

    try {
      const body = {
        title: form.title.trim(),
        report_type: form.reportType,
        sections: form.sections.map((s, idx) => ({
          section_type: s.sectionType,
          title: s.title,
          content: s.content,
          order: idx,
        })),
        limitations: [...form.limitations],
        caveats: [...form.caveats],
        assumptions: [...form.assumptions],
        referenced_evidence_ids: [...form.referencedEvidenceIds],
      };

      const url = existingReport
        ? `${API_BASE}/api/reports/${existingReport.id}`
        : `${API_BASE}/api/cases/${caseId}/reports`;
      const res = await fetch(url, {
        method: existingReport ? 'PUT' : 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${accessToken}`,
        },
        body: JSON.stringify(body),
      });

      if (!res.ok) {
        const data = await res.json().catch(() => null);
        throw new Error(
          data?.error || `Failed to save report (${res.status})`,
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
    <>
      <section className="d-pagehead">
        <div>
          <span className="eyebrow-m">Reports</span>
          <h1>{existingReport ? 'Edit' : 'Build'} <em>report</em></h1>
          <p className="sub">
            Reports are sealed snapshots. Once saved, the report can be reviewed, approved, and cryptographically signed.
          </p>
        </div>
        <div className="actions">
          <button
            type="button"
            className="btn"
            disabled={saving}
            onClick={handleSave}
          >
            {saving ? 'Saving\u2026' : existingReport ? 'Update report' : 'Save report'} <span className="arr">&rarr;</span>
          </button>
        </div>
      </section>

      {error && (
        <div className="panel" style={{ marginBottom: 16, borderLeft: '3px solid var(--err)' }}>
          <div className="panel-body" style={{ color: 'var(--err)', fontSize: '13.5px' }}>
            {error}
          </div>
        </div>
      )}

      <div className="g2-wide">
        {/* Main form */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          {/* Title + Type */}
          <div className="panel" style={{ margin: 0 }}>
            <div className="panel-h"><h3>Details</h3></div>
            <div className="panel-body" style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
              <div>
                <label htmlFor="rb-title" style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: '10.5px', letterSpacing: '.06em', textTransform: 'uppercase', color: 'var(--muted)', marginBottom: 6, display: 'block' }}>
                  Title *
                </label>
                <input
                  id="rb-title"
                  type="text"
                  placeholder="Report title"
                  value={form.title}
                  onChange={(e) => updateField('title', e.target.value)}
                  style={{
                    width: '100%',
                    padding: '8px 12px',
                    fontSize: '13.5px',
                    border: '1px solid var(--line)',
                    borderRadius: 6,
                    background: 'var(--paper)',
                    color: 'var(--ink)',
                  }}
                />
              </div>
              <div>
                <label htmlFor="rb-type" style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: '10.5px', letterSpacing: '.06em', textTransform: 'uppercase', color: 'var(--muted)', marginBottom: 6, display: 'block' }}>
                  Report type
                </label>
                <select
                  id="rb-type"
                  value={form.reportType}
                  onChange={(e) => updateField('reportType', e.target.value as ReportType)}
                  style={{
                    width: '100%',
                    padding: '8px 12px',
                    fontSize: '13.5px',
                    border: '1px solid var(--line)',
                    borderRadius: 6,
                    background: 'var(--paper)',
                    color: 'var(--ink)',
                  }}
                >
                  {REPORT_TYPES.map((t) => (
                    <option key={t.value} value={t.value}>{t.label}</option>
                  ))}
                </select>
              </div>
            </div>
          </div>

          {/* Sections */}
          <div className="panel" style={{ margin: 0 }}>
            <div className="panel-h">
              <h3>Sections</h3>
              <button type="button" className="btn ghost" onClick={addSection} style={{ fontSize: 13 }}>
                + Add section
              </button>
            </div>
            <div className="panel-body">
              {form.sections.length === 0 && (
                <p style={{ fontSize: '13.5px', color: 'var(--muted)', margin: 0 }}>
                  No sections yet. Click &quot;Add section&quot; to begin.
                </p>
              )}

              <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
                {form.sections.map((section, idx) => (
                  <div
                    key={section.id}
                    style={{
                      padding: 14,
                      border: '1px solid var(--line)',
                      borderRadius: 8,
                      background: 'var(--bg-2)',
                    }}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 10 }}>
                      <span style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: 11, fontWeight: 600, color: 'var(--muted)', minWidth: 24 }}>
                        {idx + 1}.
                      </span>
                      <select
                        value={section.sectionType}
                        onChange={(e) => updateSection(section.id, { sectionType: e.target.value })}
                        style={{
                          flex: 1,
                          padding: '6px 10px',
                          fontSize: '13px',
                          border: '1px solid var(--line)',
                          borderRadius: 6,
                          background: 'var(--paper)',
                          color: 'var(--ink)',
                        }}
                      >
                        {SECTION_TYPES.map((st) => (
                          <option key={st.value} value={st.value}>{st.label}</option>
                        ))}
                      </select>
                      <button type="button" className="chip" onClick={() => moveSection(idx, -1)} disabled={idx === 0} aria-label="Move up">&#9650;</button>
                      <button type="button" className="chip" onClick={() => moveSection(idx, 1)} disabled={idx === form.sections.length - 1} aria-label="Move down">&#9660;</button>
                      <button type="button" className="chip" onClick={() => removeSection(section.id)} style={{ color: 'var(--err)' }} aria-label="Remove section">&#10005;</button>
                    </div>
                    <input
                      type="text"
                      placeholder="Section title"
                      value={section.title}
                      onChange={(e) => updateSection(section.id, { title: e.target.value })}
                      style={{
                        width: '100%',
                        padding: '6px 10px',
                        fontSize: '13px',
                        border: '1px solid var(--line)',
                        borderRadius: 6,
                        background: 'var(--paper)',
                        color: 'var(--ink)',
                        marginBottom: 8,
                      }}
                    />
                    <textarea
                      rows={5}
                      placeholder="Section content..."
                      value={section.content}
                      onChange={(e) => updateSection(section.id, { content: e.target.value })}
                      style={{
                        width: '100%',
                        padding: '8px 10px',
                        fontSize: '13px',
                        border: '1px solid var(--line)',
                        borderRadius: 6,
                        background: 'var(--paper)',
                        color: 'var(--ink)',
                        resize: 'vertical',
                      }}
                    />
                  </div>
                ))}
              </div>
            </div>
          </div>

          {/* Evidence References */}
          <EvidencePicker
            caseId={caseId}
            accessToken={accessToken}
            selectedIds={form.referencedEvidenceIds}
            onSelect={(ids) => updateField('referencedEvidenceIds', ids)}
            label="Referenced Evidence"
          />
        </div>

        {/* Sidebar: Transparency */}
        <div className="panel" style={{ margin: 0, alignSelf: 'start' }}>
          <div className="panel-h" style={{ padding: '12px 14px' }}>
            <h3 style={{ fontSize: 15 }}>Transparency</h3>
          </div>
          <div className="panel-body" style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
            <TagInput
              label="Limitations"
              tags={form.limitations}
              onChange={(tags) => updateField('limitations', tags)}
            />
            <TagInput
              label="Caveats"
              tags={form.caveats}
              onChange={(tags) => updateField('caveats', tags)}
            />
            <TagInput
              label="Assumptions"
              tags={form.assumptions}
              onChange={(tags) => updateField('assumptions', tags)}
            />
          </div>
        </div>
      </div>
    </>
  );
}
