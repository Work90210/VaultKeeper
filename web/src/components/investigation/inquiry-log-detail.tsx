'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import type { InquiryLog } from '@/types';
import { InquiryLogForm } from '@/components/investigation/inquiry-log-form';
import {
  RecordDetailLayout,
  ContentSection,
  MetaBlock,
  MetaRow,
  KeywordBadge,
} from '@/components/investigation/record-detail-layout';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

export function InquiryLogDetail({
  log,
  accessToken,
}: {
  readonly log: InquiryLog;
  readonly accessToken: string;
}) {
  const router = useRouter();
  const [editing, setEditing] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const startDate = new Date(log.search_started_at);
  const endDate = log.search_ended_at ? new Date(log.search_ended_at) : null;
  const durationMs = endDate ? endDate.getTime() - startDate.getTime() : null;
  const durationMin = durationMs != null ? Math.round(durationMs / 60000) : null;

  const handleDelete = async () => {
    if (!window.confirm('Delete this inquiry log? This cannot be undone.')) return;
    setDeleting(true);
    setError(null);
    try {
      const res = await fetch(`${API_BASE}/api/inquiry-logs/${log.id}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${accessToken}` },
      });
      if (res.ok) {
        router.push(`/en/cases/${log.case_id}?tab=inquiry-logs`);
      } else {
        const data = await res.json().catch(() => null);
        setError(data?.error || 'Delete failed');
      }
    } catch {
      setError('Request failed');
    } finally {
      setDeleting(false);
    }
  };

  const formatTime = (iso: string) =>
    new Date(iso).toLocaleString('en-GB', {
      day: '2-digit',
      month: 'short',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });

  if (editing) {
    return (
      <div className="max-w-7xl mx-auto px-[var(--space-lg)] py-[var(--space-xl)]">
        <nav className="mb-[var(--space-md)]">
          <button
            type="button"
            onClick={() => setEditing(false)}
            className="link-subtle text-xs uppercase tracking-wider font-medium"
            style={{ background: 'none', border: 'none', cursor: 'pointer' }}
          >
            &larr; Back to record
          </button>
        </nav>
        <InquiryLogForm
          caseId={log.case_id}
          accessToken={accessToken}
          existingLog={log}
          onSaved={() => router.refresh()}
        />
      </div>
    );
  }

  return (
    <>
      {error && (
        <div className="max-w-7xl mx-auto px-[var(--space-lg)] pt-[var(--space-md)]">
          <div className="banner-error">{error}</div>
        </div>
      )}
      <RecordDetailLayout
        backHref={`/en/cases/${log.case_id}?tab=inquiry-logs`}
        backLabel="Inquiry Logs"
        recordType="Inquiry Log"
        title={log.objective}
        subtitle={`${log.search_tool} \u00b7 ${startDate.toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' })}`}
        actions={
          <>
            <button
              type="button"
              className="btn-secondary text-sm"
              onClick={() => setEditing(true)}
            >
              Edit
            </button>
            <button
              type="button"
              className="btn-ghost text-sm"
              style={{ color: 'var(--status-hold)' }}
              onClick={handleDelete}
              disabled={deleting}
            >
              {deleting ? 'Deleting\u2026' : 'Delete'}
            </button>
          </>
        }
        content={
          <>
            {/* Search Strategy */}
            <ContentSection label="Search Strategy">
              <p
                className="text-sm leading-relaxed whitespace-pre-wrap"
                style={{ color: 'var(--text-primary)' }}
              >
                {log.search_strategy}
              </p>
            </ContentSection>

            {/* Keywords */}
            {log.search_keywords && log.search_keywords.length > 0 && (
              <ContentSection label="Keywords">
                <div className="flex flex-wrap gap-[var(--space-xs)]">
                  {log.search_keywords.map((kw) => (
                    <KeywordBadge key={kw}>{kw}</KeywordBadge>
                  ))}
                </div>
              </ContentSection>
            )}

            {/* Search Operators */}
            {log.search_operators && (
              <ContentSection label="Search Operators">
                <p
                  className="text-sm font-[family-name:var(--font-mono)] whitespace-pre-wrap"
                  style={{ color: 'var(--text-secondary)' }}
                >
                  {log.search_operators}
                </p>
              </ContentSection>
            )}

            {/* Notes */}
            {log.notes && (
              <ContentSection label="Notes">
                <p
                  className="text-sm leading-relaxed whitespace-pre-wrap"
                  style={{ color: 'var(--text-secondary)' }}
                >
                  {log.notes}
                </p>
              </ContentSection>
            )}

            {/* Search URL */}
            {log.search_url && (
              <ContentSection label="Search URL">
                <p
                  className="text-xs font-[family-name:var(--font-mono)] break-all"
                  style={{ color: 'var(--text-secondary)' }}
                >
                  {log.search_url}
                </p>
              </ContentSection>
            )}
          </>
        }
        sidebar={
          <>
            {/* Results */}
            {log.results_count != null && (
              <MetaBlock label="Results">
                <div className="space-y-[2px]">
                  <MetaRow label="Total" value={log.results_count} />
                  <MetaRow label="Relevant" value={log.results_relevant ?? 0} highlight />
                  <MetaRow label="Collected" value={log.results_collected ?? 0} />
                </div>
              </MetaBlock>
            )}

            {/* Tool */}
            <MetaBlock label="Tool">
              <p className="text-sm" style={{ color: 'var(--text-primary)' }}>
                {log.search_tool}
              </p>
              {log.search_tool_version && (
                <p className="text-xs mt-[1px]" style={{ color: 'var(--text-tertiary)' }}>
                  v{log.search_tool_version}
                </p>
              )}
            </MetaBlock>

            {/* Time */}
            <MetaBlock label="Time">
              <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                {formatTime(log.search_started_at)}
              </p>
              {endDate && (
                <p className="text-xs mt-[1px]" style={{ color: 'var(--text-secondary)' }}>
                  to {endDate.toLocaleString('en-GB', { hour: '2-digit', minute: '2-digit' })}
                  {durationMin != null && (
                    <span style={{ color: 'var(--text-tertiary)' }}>
                      {' '}({durationMin < 60 ? `${durationMin}m` : `${Math.floor(durationMin / 60)}h ${durationMin % 60}m`})
                    </span>
                  )}
                </p>
              )}
            </MetaBlock>

            {/* Performed by */}
            <MetaBlock label="Performed by">
              <p
                className="text-xs font-[family-name:var(--font-mono)] break-all"
                style={{ color: 'var(--text-secondary)' }}
              >
                {log.performed_by}
              </p>
            </MetaBlock>
          </>
        }
        recordId={log.id}
        createdAt={log.created_at}
        updatedAt={log.updated_at}
        createdBy={log.performed_by}
      />
    </>
  );
}
