'use client';

import { useEffect, useState, useCallback } from 'react';
import type { InquiryLog } from '@/types';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';
const PAGE_SIZE = 25;

const KIND_MAP: Record<string, { label: string; cls: string }> = {
  decision: { label: 'Decision', cls: 'sealed' },
  question: { label: 'Open question', cls: 'disc' },
  action: { label: 'Action', cls: 'hold' },
  request: { label: 'External request', cls: 'draft' },
  federation: { label: 'Federation', cls: 'live' },
};

const AVATAR_CLASSES = ['a', 'b', 'c', 'd', 'e'];

function inferKind(log: InquiryLog): string {
  const text = (
    log.objective +
    ' ' +
    (log.notes ?? '') +
    ' ' +
    log.search_strategy
  ).toLowerCase();
  if (text.includes('decision') || text.includes('recommend') || text.includes('accept'))
    return 'decision';
  if (text.includes('question') || text.includes('confirmation') || text.includes('request review'))
    return 'question';
  if (text.includes('request') || text.includes('formal') || text.includes('liaison'))
    return 'request';
  if (text.includes('federation') || text.includes('federated') || text.includes('mirror'))
    return 'federation';
  return 'action';
}

function formatTime(dateStr: string): string {
  const d = new Date(dateStr);
  return d.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' });
}

function formatDay(dateStr: string): string {
  const d = new Date(dateStr);
  return d.toLocaleDateString('en-GB', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  });
}

export function InquiryLogList({
  caseId,
  accessToken,
}: {
  caseId: string;
  accessToken: string;
}) {
  const [logs, setLogs] = useState<readonly InquiryLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [offset, setOffset] = useState(0);
  const [hasMore, setHasMore] = useState(false);

  const fetchLogs = useCallback(
    async (currentOffset: number, append: boolean) => {
      setLoading(true);
      setError(null);

      try {
        const res = await fetch(
          `${API_BASE}/api/cases/${caseId}/inquiry-logs?limit=${PAGE_SIZE}&offset=${currentOffset}`,
          {
            headers: { Authorization: `Bearer ${accessToken}` },
          },
        );

        if (!res.ok) {
          const body = await res.json().catch(() => null);
          setError(body?.error || `Failed to load inquiry logs (${res.status})`);
          setLoading(false);
          return;
        }

        const json = await res.json();
        const items: InquiryLog[] = json.data ?? json ?? [];

        setLogs((prev) => (append ? [...prev, ...items] : items));
        setHasMore(items.length >= PAGE_SIZE);
      } catch {
        setError('An unexpected error occurred.');
      } finally {
        setLoading(false);
      }
    },
    [caseId, accessToken],
  );

  useEffect(() => {
    fetchLogs(0, false);
  }, [fetchLogs]);

  const handleLoadMore = () => {
    const nextOffset = offset + PAGE_SIZE;
    setOffset(nextOffset);
    fetchLogs(nextOffset, true);
  };

  if (loading && logs.length === 0) {
    return (
      <div className="panel">
        <div className="panel-body" style={{ padding: '48px 28px', textAlign: 'center' }}>
          <p style={{ color: 'var(--muted)', fontSize: 14 }}>
            Loading inquiry logs...
          </p>
        </div>
      </div>
    );
  }

  if (error && logs.length === 0) {
    return (
      <div
        style={{
          padding: '16px 20px',
          border: '1px solid rgba(184,66,28,.2)',
          borderRadius: 12,
          background: 'rgba(184,66,28,.03)',
          color: 'var(--ink)',
          fontSize: 14,
        }}
      >
        {error}
      </div>
    );
  }

  if (logs.length === 0) {
    return (
      <div className="panel">
        <div className="panel-body" style={{ padding: '48px 28px', textAlign: 'center' }}>
          <p
            style={{
              fontFamily: "'Fraunces', serif",
              fontSize: 18,
              color: 'var(--muted)',
            }}
          >
            No inquiry logs yet
          </p>
          <p style={{ color: 'var(--muted)', fontSize: 13, marginTop: 6 }}>
            Create an inquiry log to document your search activities.
          </p>
        </div>
      </div>
    );
  }

  // Sort by most recent first
  const sorted = [...logs].sort(
    (a, b) =>
      new Date(b.search_started_at).getTime() -
      new Date(a.search_started_at).getTime(),
  );

  // Group by day
  const byDay: Record<string, InquiryLog[]> = {};
  for (const log of sorted) {
    const day = formatDay(log.search_started_at);
    if (!byDay[day]) byDay[day] = [];
    byDay[day].push(log);
  }

  return (
    <>
      <div className="panel">
        <div className="panel-body flush">
          {Object.entries(byDay).map(([day, items]) => (
            <div key={day}>
              <div
                style={{
                  padding: '14px 28px',
                  background: 'var(--bg-2)',
                  borderBottom: '1px solid var(--line)',
                  fontFamily: "'JetBrains Mono', monospace",
                  fontSize: 11,
                  letterSpacing: '.08em',
                  textTransform: 'uppercase',
                  color: 'var(--muted)',
                }}
              >
                {day}
              </div>
              {items.map((log, idx) => {
                const kind = inferKind(log);
                const km = KIND_MAP[kind] ?? KIND_MAP.action;
                const avClass = AVATAR_CLASSES[idx % AVATAR_CLASSES.length];
                const initial = (log.performed_by || '?')[0].toUpperCase();
                const linked = [
                  log.evidence_id ? `E-${log.evidence_id.slice(0, 4)}` : null,
                  log.search_tool || null,
                ]
                  .filter(Boolean)
                  .join(' \u00b7 ');

                return (
                  <div
                    key={log.id}
                    style={{
                      padding: '22px 28px',
                      borderBottom: '1px solid var(--line)',
                      display: 'grid',
                      gridTemplateColumns: '72px 40px 1fr 220px',
                      gap: 20,
                      alignItems: 'start',
                    }}
                  >
                    <div
                      style={{
                        fontFamily: "'JetBrains Mono', monospace",
                        fontSize: 13,
                        color: 'var(--muted)',
                      }}
                    >
                      {formatTime(log.search_started_at)}
                    </div>
                    <span className="avs">
                      <span className={`av ${avClass}`}>{initial}</span>
                    </span>
                    <div>
                      <div
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          gap: 10,
                          marginBottom: 6,
                        }}
                      >
                        <span
                          style={{
                            fontFamily: "'Fraunces', serif",
                            fontSize: 15,
                            letterSpacing: '-.005em',
                            color: 'var(--ink)',
                          }}
                        >
                          {log.performed_by || '\u2014'}
                        </span>
                        <span className={`pl ${km.cls}`}>{km.label}</span>
                      </div>
                      <div
                        style={{
                          fontSize: 14.5,
                          lineHeight: 1.55,
                          color: 'var(--ink-2)',
                          maxWidth: '62ch',
                        }}
                      >
                        {log.objective || log.search_strategy}
                      </div>
                    </div>
                    <div
                      style={{
                        fontFamily: "'JetBrains Mono', monospace",
                        fontSize: 11.5,
                        color: 'var(--accent)',
                        textAlign: 'right',
                        letterSpacing: '.02em',
                      }}
                    >
                      {linked || '\u2014'}
                    </div>
                  </div>
                );
              })}
            </div>
          ))}
        </div>
      </div>

      {/* Load more */}
      {hasMore && (
        <div style={{ display: 'flex', justifyContent: 'center', marginTop: 22 }}>
          <button
            type="button"
            onClick={handleLoadMore}
            disabled={loading}
            className="btn ghost"
          >
            {loading ? 'Loading\u2026' : 'Load more'}
          </button>
        </div>
      )}

      {error && logs.length > 0 && (
        <div
          style={{
            padding: '16px 20px',
            border: '1px solid rgba(184,66,28,.2)',
            borderRadius: 12,
            background: 'rgba(184,66,28,.03)',
            marginTop: 12,
            color: 'var(--ink)',
            fontSize: 14,
          }}
        >
          {error}
        </div>
      )}
    </>
  );
}
