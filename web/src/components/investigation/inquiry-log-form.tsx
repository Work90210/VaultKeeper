'use client';

import { useState } from 'react';
import type { InquiryLog } from '@/types';

interface InquiryLogFormState {
  readonly objective: string;
  readonly searchStrategy: string;
  readonly keywords: string;
  readonly searchTool: string;
  readonly searchUrl: string;
  readonly startedAt: string;
  readonly endedAt: string;
  readonly resultsCount: string;
  readonly resultsRelevant: string;
  readonly resultsCollected: string;
  readonly notes: string;
}

const INITIAL_STATE: InquiryLogFormState = {
  objective: '',
  searchStrategy: '',
  keywords: '',
  searchTool: '',
  searchUrl: '',
  startedAt: '',
  endedAt: '',
  resultsCount: '',
  resultsRelevant: '',
  resultsCollected: '',
  notes: '',
};

function toLocalDatetime(iso: string): string {
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function stateFromLog(log: InquiryLog): InquiryLogFormState {
  return {
    objective: log.objective,
    searchStrategy: log.search_strategy,
    keywords: log.search_keywords.join(', '),
    searchTool: log.search_tool,
    searchUrl: log.search_url || '',
    startedAt: toLocalDatetime(log.search_started_at),
    endedAt: log.search_ended_at ? toLocalDatetime(log.search_ended_at) : '',
    resultsCount: log.results_count != null ? String(log.results_count) : '',
    resultsRelevant: log.results_relevant != null ? String(log.results_relevant) : '',
    resultsCollected: log.results_collected != null ? String(log.results_collected) : '',
    notes: log.notes || '',
  };
}

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

export function InquiryLogForm({
  caseId,
  accessToken,
  onSaved,
  existingLog,
}: {
  caseId: string;
  accessToken: string;
  onSaved: () => void;
  existingLog?: InquiryLog;
}) {
  const [form, setForm] = useState<InquiryLogFormState>(
    existingLog ? stateFromLog(existingLog) : INITIAL_STATE,
  );
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const updateField = (field: keyof InquiryLogFormState, value: string) => {
    setForm((prev) => ({ ...prev, [field]: value }));
  };

  const canSubmit =
    form.objective.trim() !== '' &&
    form.searchStrategy.trim() !== '' &&
    form.searchTool.trim() !== '' &&
    form.startedAt !== '';

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!canSubmit) return;

    setSaving(true);
    setError(null);

    const keywords = form.keywords
      .split(',')
      .map((k) => k.trim())
      .filter(Boolean);

    const body = {
      objective: form.objective.trim(),
      search_strategy: form.searchStrategy.trim(),
      search_keywords: keywords,
      search_tool: form.searchTool.trim(),
      search_url: form.searchUrl.trim() || undefined,
      search_started_at: new Date(form.startedAt).toISOString(),
      search_ended_at: form.endedAt
        ? new Date(form.endedAt).toISOString()
        : undefined,
      results_count: form.resultsCount ? Number(form.resultsCount) : undefined,
      results_relevant: form.resultsRelevant
        ? Number(form.resultsRelevant)
        : undefined,
      results_collected: form.resultsCollected
        ? Number(form.resultsCollected)
        : undefined,
      notes: form.notes.trim() || undefined,
    };

    try {
      const url = existingLog
        ? `${API_BASE}/api/inquiry-logs/${existingLog.id}`
        : `${API_BASE}/api/cases/${caseId}/inquiry-logs`;
      const res = await fetch(url, {
        method: existingLog ? 'PUT' : 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${accessToken}`,
        },
        body: JSON.stringify(body),
      });

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
        {existingLog ? 'Edit Inquiry Log' : 'New Inquiry Log'}
      </h2>

      {error && (
        <div className="banner-error">{error}</div>
      )}

      {/* Objective */}
      <div>
        <label className="field-label" htmlFor="ilf-objective">
          Objective <span style={{ color: 'var(--status-hold)' }}>*</span>
        </label>
        <input
          id="ilf-objective"
          type="text"
          value={form.objective}
          onChange={(e) => updateField('objective', e.target.value)}
          className="input-field"
          placeholder="What is the goal of this inquiry?"
          required
        />
      </div>

      {/* Search Strategy */}
      <div>
        <label className="field-label" htmlFor="ilf-search-strategy">
          Search Strategy <span style={{ color: 'var(--status-hold)' }}>*</span>
        </label>
        <textarea
          id="ilf-search-strategy"
          value={form.searchStrategy}
          onChange={(e) => updateField('searchStrategy', e.target.value)}
          className="input-field"
          rows={3}
          placeholder="Describe the search methodology and approach"
          required
        />
      </div>

      {/* Keywords */}
      <div>
        <label className="field-label" htmlFor="ilf-keywords">Keywords (comma-separated)</label>
        <input
          id="ilf-keywords"
          type="text"
          value={form.keywords}
          onChange={(e) => updateField('keywords', e.target.value)}
          className="input-field"
          placeholder="e.g. location, suspect name, date range"
        />
      </div>

      {/* Search Tool & URL row */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-[var(--space-md)]">
        <div>
          <label className="field-label" htmlFor="ilf-search-tool">
            Search Tool <span style={{ color: 'var(--status-hold)' }}>*</span>
          </label>
          <input
            id="ilf-search-tool"
            type="text"
            value={form.searchTool}
            onChange={(e) => updateField('searchTool', e.target.value)}
            className="input-field"
            placeholder="e.g. Google, Maltego, Shodan"
            required
          />
        </div>
        <div>
          <label className="field-label" htmlFor="ilf-search-url">Search URL</label>
          <input
            id="ilf-search-url"
            type="url"
            value={form.searchUrl}
            onChange={(e) => updateField('searchUrl', e.target.value)}
            className="input-field"
            placeholder="https://..."
          />
        </div>
      </div>

      {/* Time range row */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-[var(--space-md)]">
        <div>
          <label className="field-label" htmlFor="ilf-started-at">
            Started At <span style={{ color: 'var(--status-hold)' }}>*</span>
          </label>
          <input
            id="ilf-started-at"
            type="datetime-local"
            value={form.startedAt}
            onChange={(e) => updateField('startedAt', e.target.value)}
            className="input-field"
            required
          />
        </div>
        <div>
          <label className="field-label" htmlFor="ilf-ended-at">Ended At</label>
          <input
            id="ilf-ended-at"
            type="datetime-local"
            value={form.endedAt}
            onChange={(e) => updateField('endedAt', e.target.value)}
            className="input-field"
          />
        </div>
      </div>

      {/* Results row */}
      <div className="grid grid-cols-3 gap-[var(--space-md)]">
        <div>
          <label className="field-label" htmlFor="ilf-results-count">Results Count</label>
          <input
            id="ilf-results-count"
            type="number"
            min="0"
            value={form.resultsCount}
            onChange={(e) => updateField('resultsCount', e.target.value)}
            className="input-field"
            placeholder="0"
          />
        </div>
        <div>
          <label className="field-label" htmlFor="ilf-relevant">Relevant</label>
          <input
            id="ilf-relevant"
            type="number"
            min="0"
            value={form.resultsRelevant}
            onChange={(e) => updateField('resultsRelevant', e.target.value)}
            className="input-field"
            placeholder="0"
          />
        </div>
        <div>
          <label className="field-label" htmlFor="ilf-collected">Collected</label>
          <input
            id="ilf-collected"
            type="number"
            min="0"
            value={form.resultsCollected}
            onChange={(e) => updateField('resultsCollected', e.target.value)}
            className="input-field"
            placeholder="0"
          />
        </div>
      </div>

      {/* Notes */}
      <div>
        <label className="field-label" htmlFor="ilf-notes">Notes</label>
        <textarea
          id="ilf-notes"
          value={form.notes}
          onChange={(e) => updateField('notes', e.target.value)}
          className="input-field"
          rows={3}
          placeholder="Additional observations or context"
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
          {saving ? 'Saving\u2026' : existingLog ? 'Update' : 'Save Inquiry Log'}
        </button>
      </div>
    </form>
  );
}
