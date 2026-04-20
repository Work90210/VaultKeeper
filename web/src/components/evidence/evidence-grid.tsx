'use client';

import { useRouter, useSearchParams } from 'next/navigation';
import { useState } from 'react';
import type { EvidenceItem } from '@/types';

/* ---------- helpers ---------- */

function mimeToKind(mime: string): { kind: string; glyph: string } {
  if (mime.startsWith('video/')) return { kind: 'VIDEO', glyph: '\u25B8' };
  if (mime.startsWith('audio/')) return { kind: 'AUDIO', glyph: '\u266A' };
  if (mime.startsWith('image/')) return { kind: 'IMG', glyph: '\u25E9' };
  if (mime.includes('forensic') || mime.includes('encase') || mime.includes('e01'))
    return { kind: 'FORENSIC', glyph: '\u25C6' };
  return { kind: 'DOC', glyph: '\u00B6' };
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  const value = bytes / Math.pow(1024, i);
  return `${value < 10 ? value.toFixed(1) : Math.round(value)} ${units[i]}`;
}

function classificationToStatus(classification: string, destroyedAt: string | null): string {
  if (destroyedAt) return 'broken';
  switch (classification) {
    case 'ex_parte': return 'hold';
    case 'restricted': return 'sealed';
    case 'confidential': return 'sealed';
    case 'public': return 'draft';
    default: return 'sealed';
  }
}

const statusLabel: Record<string, string> = {
  sealed: 'Sealed',
  hold: 'Legal hold',
  draft: 'Draft',
  broken: 'Chain broken',
};

const BP_PHASE_MAX = 6;

function bpColor(phase: number): string {
  if (phase >= 5) return 'var(--ok)';
  if (phase >= 3) return 'var(--accent)';
  return '#b35c5c';
}

function mimeToExtension(mime: string, filename: string): string {
  const ext = filename.split('.').pop()?.toUpperCase();
  if (ext) return ext;
  const map: Record<string, string> = {
    'application/pdf': 'PDF',
    'text/csv': 'CSV',
    'video/mp4': 'MP4',
    'audio/flac': 'FLAC',
    'image/jpeg': 'JPG',
    'image/png': 'PNG',
  };
  return map[mime] ?? mime.split('/').pop()?.toUpperCase() ?? '';
}

function formatNumber(n: number): string {
  return n.toLocaleString('en-US');
}

/* ---------- component ---------- */

export function EvidenceGrid({
  caseId,
  evidence,
  nextCursor,
  hasMore,
  currentQuery,
  currentClassification,
}: {
  caseId: string;
  evidence: readonly EvidenceItem[];
  nextCursor: string;
  hasMore: boolean;
  currentQuery: string;
  currentClassification: string;
}) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [search, setSearch] = useState(currentQuery);

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    const params = new URLSearchParams(searchParams.toString());
    if (search) params.set('q', search);
    else params.delete('q');
    params.delete('cursor');
    router.push(`/en/cases/${caseId}/evidence?${params.toString()}`);
  };

  const handleClassificationFilter = (classification: string) => {
    const params = new URLSearchParams(searchParams.toString());
    if (classification) params.set('classification', classification);
    else params.delete('classification');
    params.delete('cursor');
    router.push(`/en/cases/${caseId}/evidence?${params.toString()}`);
  };

  // Compute type counts
  const docCount = evidence.filter((e) => mimeToKind(e.mime_type).kind === 'DOC').length;
  const mediaCount = evidence.filter((e) => {
    const k = mimeToKind(e.mime_type).kind;
    return k === 'IMG' || k === 'VIDEO';
  }).length;
  const audioCount = evidence.filter((e) => mimeToKind(e.mime_type).kind === 'AUDIO').length;
  const forensicCount = evidence.filter((e) => mimeToKind(e.mime_type).kind === 'FORENSIC').length;
  const holdCount = evidence.filter((e) => e.classification === 'ex_parte').length;

  const totalCount = evidence.length;
  const totalPages = hasMore ? 2 : 1;

  return (
    <div className="panel" style={{ marginBottom: 22 }}>
      <form onSubmit={handleSearch}>
        <div className="fbar">
          <div className="fsearch">
            <svg
              width="14"
              height="14"
              viewBox="0 0 16 16"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.5"
              style={{ color: 'var(--muted)' }}
            >
              <circle cx="7" cy="7" r="4" />
              <path d="M10 10l3 3" />
            </svg>
            <input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="hash, filename, tag, contributor\u2026"
            />
          </div>
          <span
            className={`chip${!currentClassification ? ' active' : ''}`}
            onClick={() => handleClassificationFilter('')}
            style={{ cursor: 'pointer' }}
          >
            All <span className="x">&middot;{formatNumber(totalCount)}</span>
          </span>
          <span className="chip">
            Document <span className="chev">{docCount > 0 ? formatNumber(docCount) : '\u2014'}</span>
          </span>
          <span className="chip">
            Image / video <span className="chev">{mediaCount > 0 ? formatNumber(mediaCount) : '\u2014'}</span>
          </span>
          <span className="chip">
            Audio <span className="chev">{audioCount > 0 ? formatNumber(audioCount) : '\u2014'}</span>
          </span>
          <span className="chip">
            Forensic <span className="chev">{forensicCount > 0 ? formatNumber(forensicCount) : '\u2014'}</span>
          </span>
          <span
            className={`chip${currentClassification === 'ex_parte' ? ' active' : ''}`}
            onClick={() => handleClassificationFilter(currentClassification === 'ex_parte' ? '' : 'ex_parte')}
            style={{ cursor: 'pointer' }}
          >
            Legal hold <span className="chev">{holdCount > 0 ? formatNumber(holdCount) : '\u2014'}</span>
          </span>
          <span className="chip">
            Has redactions <span className="chev">&#9662;</span>
          </span>
          <span className="chip">
            Contributor <span className="chev">&#9662;</span>
          </span>
          <span className="chip" style={{ marginLeft: 'auto' }}>&#8862; Grid</span>
          <span className="chip">&#9776; Table</span>
        </div>
      </form>
      <div className="panel-body">
        {evidence.length === 0 ? (
          <div
            style={{
              padding: '48px 0',
              textAlign: 'center',
              color: 'var(--muted)',
              fontFamily: "'JetBrains Mono', monospace",
              fontSize: '13px',
            }}
          >
            {currentQuery
              ? 'No evidence items match your search.'
              : 'No evidence items found.'}
          </div>
        ) : (
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(4, 1fr)',
              gap: 18,
            }}
          >
            {evidence.map((item) => {
              const { kind, glyph } = mimeToKind(item.mime_type);
              const st = classificationToStatus(item.classification, item.destroyed_at);
              const ext = mimeToExtension(item.mime_type, item.original_name);
              const meta = `${formatBytes(item.size_bytes)} \u00B7 ${ext}`;

              return (
                <a
                  href={`/en/evidence/${item.id}`}
                  className="ev-card"
                  key={item.id}
                  style={{ textDecoration: 'none', color: 'inherit' }}
                >
                  <div className="ev-thumb">
                    <span className="chip-k">{kind}</span>
                    <span className="chip-s">
                      <span
                        className={`pl ${st}`}
                        style={{ fontSize: 10, padding: '2px 7px' }}
                      >
                        {statusLabel[st]}
                      </span>
                    </span>
                    <span className="glyph">{glyph}</span>
                  </div>
                  <div className="ev-meta">
                    <div className="ref">{item.title || item.original_name}</div>
                    <div className="sm">
                      <span>{item.evidence_number}</span>
                      <span>{meta}</span>
                    </div>
                    <div className="tags">
                      {(item.tags ?? []).map((t) => (
                        <span className="tag" key={t}>{t}</span>
                      ))}
                    </div>
                    {item.bp_phase != null && (
                      <div
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          gap: 6,
                          marginTop: 4,
                          paddingTop: 8,
                          borderTop: '1px solid var(--line)',
                        }}
                      >
                        <div style={{ display: 'flex', gap: 3, alignItems: 'center' }}>
                          {Array.from({ length: item.bp_phase_max ?? BP_PHASE_MAX }, (_, i) => (
                            <span
                              key={i}
                              style={{
                                width: 6,
                                height: 6,
                                borderRadius: '50%',
                                background: i < item.bp_phase! ? bpColor(item.bp_phase!) : 'var(--bg-2)',
                              }}
                            />
                          ))}
                        </div>
                        <span
                          style={{
                            fontFamily: "'JetBrains Mono', monospace",
                            fontSize: '9.5px',
                            letterSpacing: '.04em',
                            color: bpColor(item.bp_phase),
                          }}
                        >
                          {item.bp_phase}/{item.bp_phase_max ?? BP_PHASE_MAX} phases
                        </span>
                      </div>
                    )}
                  </div>
                </a>
              );
            })}
          </div>
        )}
        <div
          style={{
            marginTop: 22,
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            fontSize: '12.5px',
            color: 'var(--muted)',
            fontFamily: "'JetBrains Mono', monospace",
            letterSpacing: '.02em',
          }}
        >
          <span>
            {evidence.length} item{evidence.length !== 1 ? 's' : ''} &middot; page 1 of {formatNumber(totalPages)}
          </span>
          <span style={{ display: 'flex', gap: 8 }}>
            <span className="chip">&laquo; Prev</span>
            <span className="chip active">1</span>
            {hasMore && (
              <a
                href={`/en/cases/${caseId}/evidence?${new URLSearchParams({
                  ...(currentQuery ? { q: currentQuery } : {}),
                  ...(currentClassification ? { classification: currentClassification } : {}),
                  cursor: nextCursor,
                }).toString()}`}
                className="chip"
                style={{ textDecoration: 'none', color: 'inherit' }}
              >
                Next &raquo;
              </a>
            )}
          </span>
        </div>
      </div>
    </div>
  );
}
