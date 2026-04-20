import type { EvidenceItem } from '@/types';

// --- Helpers to derive display values from real data ---

function mimeToKind(mime: string): { kind: string; glyph: string } {
  if (mime.startsWith('video/')) return { kind: 'VIDEO', glyph: '\u25B8' };
  if (mime.startsWith('audio/')) return { kind: 'AUDIO', glyph: '\u266A' };
  if (mime.startsWith('image/')) return { kind: 'IMG', glyph: '\u25E9' };
  if (
    mime.includes('forensic') ||
    mime.includes('encase') ||
    mime.includes('e01')
  )
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

function classificationToStatus(
  classification: string,
  destroyedAt: string | null
): string {
  if (destroyedAt) return 'broken';
  switch (classification) {
    case 'ex_parte':
      return 'hold';
    case 'restricted':
      return 'sealed';
    case 'confidential':
      return 'sealed';
    case 'public':
      return 'draft';
    default:
      return 'sealed';
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

function formatNumber(n: number): string {
  return n.toLocaleString('en-US');
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

// --- Props ---

export interface EvidenceViewProps {
  readonly evidenceItems: readonly EvidenceItem[];
  readonly totalCount: number;
  readonly facets?: Readonly<Record<string, Record<string, number>>>;
  readonly error?: string | null;
}

export default function EvidenceView({
  evidenceItems,
  totalCount,
  facets,
  error,
}: EvidenceViewProps) {
  // Compute KPIs from real data
  const docCount = evidenceItems.filter(
    (e) => mimeToKind(e.mime_type).kind === 'DOC'
  ).length;
  const mediaCount = evidenceItems.filter((e) => {
    const k = mimeToKind(e.mime_type).kind;
    return k === 'IMG' || k === 'VIDEO';
  }).length;
  const audioCount = evidenceItems.filter(
    (e) => mimeToKind(e.mime_type).kind === 'AUDIO'
  ).length;
  const forensicCount = evidenceItems.filter(
    (e) => mimeToKind(e.mime_type).kind === 'FORENSIC'
  ).length;
  const holdCount = evidenceItems.filter(
    (e) => e.classification === 'ex_parte'
  ).length;

  const pageSize = evidenceItems.length || 1;
  const totalPages = totalCount > 0 ? Math.ceil(totalCount / pageSize) : 1;

  return (
    <>
      <section className="d-pagehead">
        <div>
          <span className="eyebrow-m">Berkeley Protocol phases 2&ndash;4</span>
          <h1>
            Evidence <em>locker</em>
          </h1>
          <p className="sub">
            Every upload is chunked, hashed client-side (SHA-256 + BLAKE3) and
            RFC 3161 timestamped at the gateway. Phase indicators show each
            exhibit&apos;s progress through the Berkeley Protocol&apos;s
            six-phase investigative cycle.
          </p>
        </div>
        <div className="actions">
          <a className="btn ghost" href="#">
            Import archive
          </a>
          <a className="btn" href="#">
            Upload exhibit <span className="arr">&rarr;</span>
          </a>
        </div>
      </section>

      {error && (
        <div className="banner-error" style={{ marginBottom: '16px' }}>
          {error}
        </div>
      )}

      <div className="panel" style={{ marginBottom: 22 }}>
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
            <input placeholder="hash, filename, tag, contributor\u2026" />
          </div>
          <span className="chip active">
            All{' '}
            <span className="x">&middot;{formatNumber(totalCount)}</span>
          </span>
          <span className="chip">
            Document{' '}
            <span className="chev">
              {facets?.mime_type
                ? '\u2014'
                : docCount > 0
                  ? formatNumber(docCount)
                  : '\u2014'}
            </span>
          </span>
          <span className="chip">
            Image / video{' '}
            <span className="chev">
              {mediaCount > 0 ? formatNumber(mediaCount) : '\u2014'}
            </span>
          </span>
          <span className="chip">
            Audio{' '}
            <span className="chev">
              {audioCount > 0 ? formatNumber(audioCount) : '\u2014'}
            </span>
          </span>
          <span className="chip">
            Forensic{' '}
            <span className="chev">
              {forensicCount > 0 ? formatNumber(forensicCount) : '\u2014'}
            </span>
          </span>
          <span className="chip">
            Legal hold{' '}
            <span className="chev">
              {holdCount > 0 ? formatNumber(holdCount) : '\u2014'}
            </span>
          </span>
          <span className="chip">
            Has redactions <span className="chev">&#9662;</span>
          </span>
          <span className="chip">
            Contributor <span className="chev">&#9662;</span>
          </span>
          <span className="chip" style={{ marginLeft: 'auto' }}>
            &#8862; Grid
          </span>
          <span className="chip">&#9776; Table</span>
        </div>
        <div className="panel-body">
          {evidenceItems.length === 0 ? (
            <div
              style={{
                padding: '48px 0',
                textAlign: 'center',
                color: 'var(--muted)',
                fontFamily: "'JetBrains Mono', monospace",
                fontSize: '13px',
              }}
            >
              No evidence items found.
            </div>
          ) : (
            <div
              style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(4, 1fr)',
                gap: 18,
              }}
            >
              {evidenceItems.map((e) => {
                const { kind, glyph } = mimeToKind(e.mime_type);
                const st = classificationToStatus(
                  e.classification,
                  e.destroyed_at
                );
                const ext = mimeToExtension(e.mime_type, e.original_name);
                const meta = `${formatBytes(e.size_bytes)} \u00B7 ${ext}`;

                return (
                  <a href={`/en/evidence/${e.id}`} className="ev-card" key={e.id} style={{ textDecoration: 'none', color: 'inherit' }}>
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
                      <div className="ref">{e.title || e.original_name}</div>
                      <div className="sm">
                        <span>{e.evidence_number}</span>
                        <span>{meta}</span>
                      </div>
                      <div className="tags">
                        {(e.tags ?? []).map((t) => (
                          <span className="tag" key={t}>
                            {t}
                          </span>
                        ))}
                      </div>
                      {e.bp_phase != null && (
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
                          <div
                            style={{
                              display: 'flex',
                              gap: 3,
                              alignItems: 'center',
                            }}
                          >
                            {Array.from(
                              { length: e.bp_phase_max ?? BP_PHASE_MAX },
                              (_, i) => (
                                <span
                                  key={i}
                                  style={{
                                    width: 6,
                                    height: 6,
                                    borderRadius: '50%',
                                    background:
                                      i < e.bp_phase!
                                        ? bpColor(e.bp_phase!)
                                        : 'var(--bg-2)',
                                  }}
                                />
                              )
                            )}
                          </div>
                          <span
                            style={{
                              fontFamily: "'JetBrains Mono', monospace",
                              fontSize: '9.5px',
                              letterSpacing: '.04em',
                              color: bpColor(e.bp_phase),
                            }}
                          >
                            {e.bp_phase}/{e.bp_phase_max ?? BP_PHASE_MAX} phases
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
              {evidenceItems.length} of {formatNumber(totalCount)} &middot; page
              1 of {formatNumber(totalPages)}
            </span>
            <span style={{ display: 'flex', gap: 8 }}>
              <span className="chip">&laquo; Prev</span>
              <span className="chip active">1</span>
              {totalPages > 1 && <span className="chip">2</span>}
              {totalPages > 2 && <span className="chip">3</span>}
              {totalPages > 3 && (
                <span className="chip">
                  &hellip; {formatNumber(totalPages)}
                </span>
              )}
              {totalPages > 1 && <span className="chip">Next &raquo;</span>}
            </span>
          </div>
        </div>
      </div>

      <div className="g2-wide">
        <div className="panel">
          <div className="panel-h">
            <h3>Upload queue</h3>
            <span className="meta">&mdash;</span>
          </div>
          <div
            className="panel-body"
            style={{
              display: 'flex',
              flexDirection: 'column',
              gap: 14,
              padding: '24px 0',
              textAlign: 'center',
              color: 'var(--muted)',
              fontFamily: "'JetBrains Mono', monospace",
              fontSize: '12px',
            }}
          >
            No uploads in progress.
          </div>
        </div>

        <div className="panel">
          <div className="panel-h">
            <h3>Integrity summary</h3>
            <span className="meta">real-time</span>
          </div>
          <div className="panel-body">
            <dl className="kvs">
              <dt>Hash algorithm</dt>
              <dd>SHA-256 primary &middot; BLAKE3 secondary</dd>
              <dt>Timestamp authority</dt>
              <dd>
                <code>ts-eu-west</code> &middot; RFC 3161
              </dd>
              <dt>Total items</dt>
              <dd>{formatNumber(totalCount)}</dd>
              <dt>Displayed</dt>
              <dd>{evidenceItems.length} items</dd>
              <dt>Validator</dt>
              <dd>
                <a className="linkarrow" href="/validator">
                  Offline verify (0.3 MB binary) &rarr;
                </a>
              </dd>
            </dl>
          </div>
        </div>
      </div>
    </>
  );
}

export { EvidenceView };
