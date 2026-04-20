'use client';

import type { AnalysisNote } from '@/types';

const statusPillClass: Record<string, string> = {
  approved: 'sealed',
  in_review: 'disc',
  draft: 'draft',
  superseded: 'broken',
};

const statusLabel: Record<string, string> = {
  approved: 'signed',
  in_review: 'peer-review',
  draft: 'draft',
  superseded: 'superseded',
};

function formatLabel(value: string): string {
  return value.replace(/_/g, ' ');
}

const avatarColors = ['a', 'b', 'c', 'd', 'e'];

function avatarColor(id: string): string {
  let hash = 0;
  for (let i = 0; i < id.length; i++) {
    hash = (hash * 31 + id.charCodeAt(i)) | 0;
  }
  return avatarColors[Math.abs(hash) % avatarColors.length];
}

function formatTimestamp(iso: string): string {
  return new Date(iso).toLocaleString('en-GB', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

export function AnalysisNoteDetail({
  note,
  accessToken: _accessToken,
}: {
  readonly note: AnalysisNote;
  readonly accessToken: string;
}) {
  const relatedCount =
    (note.related_evidence_ids?.length ?? 0) +
    (note.related_inquiry_ids?.length ?? 0) +
    (note.related_assessment_ids?.length ?? 0) +
    (note.related_verification_ids?.length ?? 0);

  const displayStatus = statusLabel[note.status] ?? note.status;
  const pillClass = statusPillClass[note.status] ?? 'draft';
  const avClass = avatarColor(note.author_id);

  return (
    <>
      {/* Back link */}
      <nav style={{ marginBottom: 18 }}>
        <a
          href={`/en/cases/${note.case_id}?tab=analysis`}
          className="linkarrow"
          style={{ fontSize: 13 }}
        >
          &larr; Analysis
        </a>
      </nav>

      {/* Header badges */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          marginBottom: 12,
          flexWrap: 'wrap',
        }}
      >
        <span
          style={{
            fontFamily: "'JetBrains Mono', monospace",
            fontSize: 13,
            color: 'var(--muted)',
            letterSpacing: '.04em',
          }}
        >
          {note.id.slice(0, 8).toUpperCase()}
        </span>
        <span className={`pl ${pillClass}`}>{displayStatus}</span>
        <span className="tag">{formatLabel(note.analysis_type)}</span>
      </div>

      {/* Title */}
      <h1
        style={{
          fontFamily: "'Fraunces', serif",
          fontWeight: 400,
          fontSize: 'clamp(28px, 3vw, 42px)',
          letterSpacing: '-.025em',
          lineHeight: 1.05,
          marginBottom: 24,
        }}
      >
        <em style={{ color: 'var(--accent)', fontStyle: 'italic' }}>
          {note.title}
        </em>
      </h1>

      {/* Main content + sidebar layout */}
      <div className="g2-wide">
        {/* Main content */}
        <div>
          {/* Status + tags header */}
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 10,
              marginBottom: 22,
            }}
          >
            <span className={`pl ${pillClass}`}>{displayStatus}</span>
            <span className="tag">{formatLabel(note.analysis_type)}</span>
          </div>

          {/* Content panel */}
          <div className="panel" style={{ marginBottom: 22 }}>
            <div className="panel-h">
              <h3>Content</h3>
            </div>
            <div className="panel-body">
              <p
                style={{
                  fontSize: 14,
                  lineHeight: 1.6,
                  color: 'var(--ink-2)',
                  whiteSpace: 'pre-wrap',
                  maxWidth: '68ch',
                }}
              >
                {note.content}
              </p>
            </div>
          </div>

          {/* Methodology panel */}
          {note.methodology && (
            <div className="panel">
              <div className="panel-h">
                <h3>
                  <em>Methodology</em>
                </h3>
              </div>
              <div className="panel-body">
                <p
                  style={{
                    fontSize: 13.5,
                    lineHeight: 1.6,
                    color: 'var(--muted)',
                    whiteSpace: 'pre-wrap',
                  }}
                >
                  {note.methodology}
                </p>
              </div>
            </div>
          )}
        </div>

        {/* Sidebar */}
        <div>
          <div className="panel">
            <div className="panel-h">
              <h3>Details</h3>
            </div>
            <div className="panel-body">
              <dl className="kvs">
                <dt>Analysis type</dt>
                <dd>
                  <span className="tag">{formatLabel(note.analysis_type)}</span>
                </dd>

                <dt>Status</dt>
                <dd>
                  <span className={`pl ${pillClass}`}>{displayStatus}</span>
                </dd>

                <dt>Author</dt>
                <dd>
                  <div
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 8,
                    }}
                  >
                    <span className="avs">
                      <span className={`av ${avClass}`}>
                        {note.author_id
                          ? note.author_id[0].toUpperCase()
                          : '?'}
                      </span>
                    </span>
                    <span
                      style={{
                        fontFamily: "'Fraunces', serif",
                        fontSize: 14,
                        color: 'var(--ink)',
                        letterSpacing: '-.005em',
                      }}
                    >
                      {note.author_id ?? '\u2014'}
                    </span>
                  </div>
                </dd>

                {note.reviewer_id && (
                  <>
                    <dt>Reviewer</dt>
                    <dd>
                      <code>{note.reviewer_id}</code>
                      {note.reviewed_at && (
                        <div
                          style={{
                            fontFamily: "'JetBrains Mono', monospace",
                            fontSize: 10.5,
                            color: 'var(--muted)',
                            marginTop: 2,
                          }}
                        >
                          {formatTimestamp(note.reviewed_at)}
                        </div>
                      )}
                    </dd>
                  </>
                )}

                {relatedCount > 0 && (
                  <>
                    <dt>Related</dt>
                    <dd>
                      <code>
                        {relatedCount} record{relatedCount !== 1 ? 's' : ''}
                      </code>
                    </dd>
                  </>
                )}

                {note.superseded_by && (
                  <>
                    <dt>Superseded by</dt>
                    <dd>
                      <code
                        style={{
                          color: 'var(--accent)',
                        }}
                      >
                        {note.superseded_by}
                      </code>
                    </dd>
                  </>
                )}

                <dt>Created</dt>
                <dd>{formatTimestamp(note.created_at)}</dd>

                {note.updated_at !== note.created_at && (
                  <>
                    <dt>Updated</dt>
                    <dd>{formatTimestamp(note.updated_at)}</dd>
                  </>
                )}
              </dl>
            </div>
          </div>

          {/* Record ID */}
          <div style={{ marginTop: 14 }}>
            <div
              style={{
                fontFamily: "'JetBrains Mono', monospace",
                fontSize: 10,
                color: 'var(--muted)',
                letterSpacing: '.04em',
              }}
            >
              {note.id}
            </div>
          </div>
        </div>
      </div>
    </>
  );
}
