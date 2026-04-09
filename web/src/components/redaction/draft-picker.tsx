'use client';

import { useCallback, useEffect, useState } from 'react';
import type { RedactionDraft, RedactionPurpose } from '@/types';
import { REDACTION_PURPOSE_LABELS } from '@/types';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

const PURPOSES: RedactionPurpose[] = [
  'disclosure_defence',
  'disclosure_prosecution',
  'public_release',
  'court_submission',
  'witness_protection',
  'internal_review',
];

interface DraftPickerProps {
  evidenceId: string;
  accessToken: string;
  onSelect: (draft: RedactionDraft) => void;
  onClose: () => void;
}

export function DraftPicker({
  evidenceId,
  accessToken,
  onSelect,
  onClose,
}: DraftPickerProps) {
  const [drafts, setDrafts] = useState<RedactionDraft[]>([]);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // New draft form
  const [name, setName] = useState('');
  const [purpose, setPurpose] = useState<RedactionPurpose>('disclosure_defence');

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const res = await fetch(
          `${API_BASE}/api/evidence/${evidenceId}/redact/drafts`,
          { headers: { Authorization: `Bearer ${accessToken}` } }
        );
        if (res.ok) {
          const json = await res.json();
          if (!cancelled) setDrafts(json.data || []);
        } else if (!cancelled) {
          setError('Failed to load existing drafts');
        }
      } catch {
        if (!cancelled) setError('Failed to load existing drafts');
      }
      if (!cancelled) setLoading(false);
    })();
    return () => { cancelled = true; };
  }, [evidenceId, accessToken]);

  const handleCreate = useCallback(async () => {
    if (!name.trim()) return;
    setCreating(true);
    setError(null);

    try {
      const res = await fetch(
        `${API_BASE}/api/evidence/${evidenceId}/redact/drafts`,
        {
          method: 'POST',
          headers: {
            Authorization: `Bearer ${accessToken}`,
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({ name: name.trim(), purpose }),
        }
      );

      if (!res.ok) {
        const body = await res.json().catch(() => null);
        setError(body?.error || 'Failed to create draft');
        setCreating(false);
        return;
      }

      const json = await res.json();
      onSelect(json.data);
    } catch {
      setError('An unexpected error occurred');
    }
    setCreating(false);
  }, [name, purpose, evidenceId, accessToken, onSelect]);

  const activeDrafts = drafts.filter((d) => d.status === 'draft');

  return (
    <div
      className="fixed inset-0 z-[100] flex items-center justify-center"
      style={{ backgroundColor: 'oklch(0.2 0.01 50 / 0.6)' }}
    >
      <div className="card max-w-lg w-full mx-[var(--space-lg)] p-[var(--space-lg)] space-y-[var(--space-lg)]">
        <div className="flex items-center justify-between">
          <h3
            className="font-[family-name:var(--font-heading)] text-lg"
            style={{ color: 'var(--text-primary)' }}
          >
            New Redacted Version
          </h3>
          <button type="button" onClick={onClose} className="btn-ghost text-xs">
            Cancel
          </button>
        </div>

        {/* Resume existing drafts */}
        {!loading && activeDrafts.length > 0 && (
          <div>
            <p
              className="text-xs font-semibold uppercase tracking-wider mb-[var(--space-sm)]"
              style={{ color: 'var(--text-tertiary)' }}
            >
              Resume Draft
            </p>
            <div className="space-y-[var(--space-xs)]">
              {activeDrafts.map((draft) => (
                <button
                  key={draft.id}
                  type="button"
                  onClick={() => onSelect(draft)}
                  className="w-full text-left p-[var(--space-sm)] rounded-[var(--radius-md)] transition-colors"
                  style={{
                    border: '1px solid var(--border-default)',
                    backgroundColor: 'var(--bg-elevated)',
                  }}
                >
                  <div className="flex items-center justify-between">
                    <span
                      className="text-sm font-medium"
                      style={{ color: 'var(--text-primary)' }}
                    >
                      {draft.name}
                    </span>
                    <span
                      className="badge"
                      style={{
                        backgroundColor: 'var(--bg-inset)',
                        color: 'var(--text-secondary)',
                        fontSize: '0.625rem',
                      }}
                    >
                      {REDACTION_PURPOSE_LABELS[draft.purpose]?.split(' ').pop()}
                    </span>
                  </div>
                  <p
                    className="text-xs mt-[var(--space-xs)]"
                    style={{ color: 'var(--text-tertiary)' }}
                  >
                    {draft.area_count} area{draft.area_count !== 1 ? 's' : ''}
                    {' \u00b7 '}
                    Last saved{' '}
                    {new Date(draft.last_saved_at).toLocaleDateString('en-GB', {
                      day: '2-digit',
                      month: 'short',
                      hour: '2-digit',
                      minute: '2-digit',
                    })}
                  </p>
                </button>
              ))}
            </div>
          </div>
        )}

        {/* Create new draft */}
        <div>
          {activeDrafts.length > 0 && (
            <p
              className="text-xs font-semibold uppercase tracking-wider mb-[var(--space-sm)]"
              style={{ color: 'var(--text-tertiary)' }}
            >
              Or Create New
            </p>
          )}
          <div className="space-y-[var(--space-sm)]">
            <div>
              <label className="field-label" htmlFor="draft-name">
                Name
              </label>
              <input
                id="draft-name"
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. Q1 Defence Disclosure"
                className="input-field w-full"
                autoFocus
              />
            </div>
            <div>
              <label className="field-label" htmlFor="draft-purpose">
                Purpose
              </label>
              <select
                id="draft-purpose"
                value={purpose}
                onChange={(e) => setPurpose(e.target.value as RedactionPurpose)}
                className="input-field w-full"
              >
                {PURPOSES.map((p) => (
                  <option key={p} value={p}>
                    {REDACTION_PURPOSE_LABELS[p]}
                  </option>
                ))}
              </select>
            </div>
          </div>
        </div>

        {error && <div className="banner-error">{error}</div>}

        <div className="flex gap-[var(--space-sm)] justify-end">
          <button type="button" onClick={onClose} className="btn-ghost">
            Cancel
          </button>
          <button
            type="button"
            onClick={handleCreate}
            disabled={creating || !name.trim()}
            className="btn-primary"
          >
            {creating ? 'Creating\u2026' : 'Create & Edit'}
          </button>
        </div>
      </div>
    </div>
  );
}
