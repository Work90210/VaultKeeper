'use client';

import { useCallback, useEffect, useState } from 'react';
import { useRouter, useParams } from 'next/navigation';
import type {
  FinalizedRedaction,
  RedactionDraft,
  RedactionManagementView,
} from '@/types';
import { REDACTION_PURPOSE_LABELS } from '@/types';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

interface RedactedVersionsProps {
  evidenceId: string;
  accessToken: string;
  onResumeDraft: (draft: RedactionDraft) => void;
  onNewDraft: () => void;
}

export function RedactedVersions({
  evidenceId,
  accessToken,
  onResumeDraft,
  onNewDraft,
}: RedactedVersionsProps) {
  const router = useRouter();
  const params = useParams();
  const locale = (params?.locale as string) || 'en';
  const [view, setView] = useState<RedactionManagementView | null>(null);
  const [loading, setLoading] = useState(true);
  const [discarding, setDiscarding] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const loadView = useCallback(async () => {
    try {
      const res = await fetch(
        `${API_BASE}/api/evidence/${evidenceId}/redactions`,
        { headers: { Authorization: `Bearer ${accessToken}` } }
      );
      if (res.ok) {
        const json = await res.json();
        setView(json.data);
        setError(null);
      } else {
        setError('Failed to load redacted versions');
      }
    } catch {
      setError('Failed to load redacted versions');
    }
    setLoading(false);
  }, [evidenceId, accessToken]);

  useEffect(() => {
    loadView();
  }, [loadView]);

  const handleDiscard = useCallback(
    async (draftId: string) => {
      if (!confirm('Discard this draft? This cannot be undone.')) return;
      setDiscarding(draftId);
      try {
        await fetch(
          `${API_BASE}/api/evidence/${evidenceId}/redact/drafts/${draftId}`,
          {
            method: 'DELETE',
            headers: { Authorization: `Bearer ${accessToken}` },
          }
        );
        await loadView();
      } catch {
        // ignore
      }
      setDiscarding(null);
    },
    [evidenceId, accessToken, loadView]
  );

  if (loading) {
    return (
      <div className="card p-[var(--space-lg)]">
        <h2 className="field-label mb-[var(--space-sm)]">Redacted Versions</h2>
        <p className="text-sm" style={{ color: 'var(--text-tertiary)' }}>
          Loading...
        </p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="card p-[var(--space-lg)]">
        <h2 className="field-label mb-[var(--space-sm)]">Redacted Versions</h2>
        <p className="text-sm" style={{ color: 'var(--status-hold)' }}>{error}</p>
      </div>
    );
  }

  if (!view) return null;

  const hasDrafts = view.drafts.filter((d) => d.status === 'draft').length > 0;
  const hasFinalized = view.finalized.length > 0;

  if (!hasDrafts && !hasFinalized) {
    return (
      <div className="card p-[var(--space-lg)]">
        <div className="flex items-center justify-between mb-[var(--space-sm)]">
          <h2 className="field-label">Redacted Versions</h2>
          <button type="button" onClick={onNewDraft} className="btn-secondary text-xs">
            + New Redacted Version
          </button>
        </div>
        <p className="text-sm" style={{ color: 'var(--text-tertiary)' }}>
          No redacted versions yet. Create one to start redacting areas of this document.
        </p>
      </div>
    );
  }

  return (
    <div className="card overflow-hidden">
      <div className="p-[var(--space-lg)]">
        <div className="flex items-center justify-between mb-[var(--space-md)]">
          <h2 className="field-label">Redacted Versions</h2>
          <button type="button" onClick={onNewDraft} className="btn-secondary text-xs">
            + New Redacted Version
          </button>
        </div>

        {/* Drafts */}
        {hasDrafts && (
          <div className="mb-[var(--space-lg)]">
            <p
              className="text-xs font-semibold uppercase tracking-wider mb-[var(--space-sm)]"
              style={{ color: 'var(--text-tertiary)' }}
            >
              Drafts in Progress
            </p>
            <div className="space-y-[var(--space-sm)]">
              {view.drafts
                .filter((d) => d.status === 'draft')
                .map((draft) => (
                  <DraftRow
                    key={draft.id}
                    draft={draft}
                    onResume={() => onResumeDraft(draft)}
                    onDiscard={() => handleDiscard(draft.id)}
                    discarding={discarding === draft.id}
                  />
                ))}
            </div>
          </div>
        )}

        {/* Finalized */}
        {hasFinalized && (
          <div>
            {hasDrafts && (
              <div
                className="mb-[var(--space-md)]"
                style={{ borderTop: '1px solid var(--border-subtle)' }}
              />
            )}
            <p
              className="text-xs font-semibold uppercase tracking-wider mb-[var(--space-sm)]"
              style={{ color: 'var(--text-tertiary)' }}
            >
              Finalized
            </p>
            <div className="space-y-[var(--space-sm)]">
              {view.finalized.map((f) => (
                <FinalizedRow
                  key={f.id}
                  item={f}
                  onView={() => router.push(`/${locale}/evidence/${f.id}`)}
                />
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function DraftRow({
  draft,
  onResume,
  onDiscard,
  discarding,
}: {
  draft: RedactionDraft;
  onResume: () => void;
  onDiscard: () => void;
  discarding: boolean;
}) {
  return (
    <div
      className="flex items-center justify-between p-[var(--space-sm)] rounded-[var(--radius-md)]"
      style={{
        border: '1px solid var(--border-default)',
        backgroundColor: 'var(--bg-elevated)',
      }}
    >
      <div>
        <div className="flex items-center gap-[var(--space-sm)]">
          <span
            className="text-sm font-medium"
            style={{ color: 'var(--text-primary)' }}
          >
            {draft.name}
          </span>
          <PurposeBadge purpose={draft.purpose} />
        </div>
        <p className="text-xs mt-[var(--space-xs)]" style={{ color: 'var(--text-tertiary)' }}>
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
      </div>
      <div className="flex gap-[var(--space-xs)] shrink-0">
        <button type="button" onClick={onResume} className="btn-secondary text-xs">
          Resume
        </button>
        <button
          type="button"
          onClick={onDiscard}
          disabled={discarding}
          className="btn-ghost text-xs"
          style={{ color: 'var(--status-hold)' }}
        >
          {discarding ? 'Discarding\u2026' : 'Discard'}
        </button>
      </div>
    </div>
  );
}

function FinalizedRow({
  item,
  onView,
}: {
  item: FinalizedRedaction;
  onView: () => void;
}) {
  return (
    <div
      className="flex items-center justify-between p-[var(--space-sm)] rounded-[var(--radius-md)]"
      style={{
        border: '1px solid var(--border-default)',
        backgroundColor: 'var(--bg-elevated)',
      }}
    >
      <div>
        <div className="flex items-center gap-[var(--space-sm)]">
          <span
            className="text-sm font-medium"
            style={{ color: 'var(--text-primary)' }}
          >
            {item.name}
          </span>
          <PurposeBadge purpose={item.purpose} />
        </div>
        <p className="text-xs mt-[var(--space-xs)]" style={{ color: 'var(--text-tertiary)' }}>
          {item.area_count} area{item.area_count !== 1 ? 's' : ''}
          {' \u00b7 '}
          Finalized{' '}
          {new Date(item.finalized_at).toLocaleDateString('en-GB', {
            day: '2-digit',
            month: 'short',
            year: 'numeric',
          })}
          {' \u00b7 '}
          {item.author}
        </p>
        <p
          className="text-xs font-[family-name:var(--font-mono)] mt-[var(--space-xs)]"
          style={{ color: 'var(--text-tertiary)' }}
        >
          {item.evidence_number}
        </p>
      </div>
      <div className="flex gap-[var(--space-xs)] shrink-0">
        <button type="button" onClick={onView} className="btn-secondary text-xs">
          View
        </button>
        <a
          href={`/api/evidence/${item.id}/download`}
          className="btn-secondary text-xs"
          download
        >
          Download
        </a>
      </div>
    </div>
  );
}

function PurposeBadge({ purpose }: { purpose: string }) {
  const label =
    REDACTION_PURPOSE_LABELS[purpose as keyof typeof REDACTION_PURPOSE_LABELS] ||
    purpose;
  const shortLabel = label.split(' ').pop() || label;

  return (
    <span
      className="badge"
      style={{
        backgroundColor: 'var(--bg-inset)',
        color: 'var(--text-secondary)',
        fontSize: '0.625rem',
      }}
    >
      {shortLabel}
    </span>
  );
}
