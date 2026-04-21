import type { EvidenceItem } from '@/types';
import {
  Panel,
  KeyValueList,
  LinkArrow,
  EyebrowLabel,
  StatusPill,
  Tag,
} from '@/components/ui/dashboard';
import { EvidenceFilterBar } from './evidence-filter-bar';

// --- Helpers ---

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
    case 'confidential':
      return 'sealed';
    case 'public':
      return 'draft';
    default:
      return 'sealed';
  }
}

const STATUS_LABEL: Record<string, string> = {
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

// --- Upload queue stub data (matching design prototype) ---

const UPLOAD_QUEUE = [
  { name: 'Butcha_drone_05.mp4', size: '412 MB', pct: 78, status: 'sha256 + timestamp' },
  { name: 'W-0212_statement_signed.pdf', size: '1.4 MB', pct: 100, status: 'sealing\u2026' },
  { name: 'Intercept_14:08.flac', size: '19 MB', pct: 34, status: 'uploading \u00B7 4/12 chunks' },
] as const;

// --- Integrity summary stub data (matching design prototype) ---

function integrityItems(totalCount: number): readonly {
  readonly label: string;
  readonly value: React.ReactNode;
}[] {
  return [
    { label: 'Hash algorithm', value: 'SHA-256 primary \u00B7 BLAKE3 secondary' },
    {
      label: 'Timestamp authority',
      value: (
        <>
          <code>ts-eu-west</code> &middot; RFC 3161
        </>
      ),
    },
    { label: 'Last verification', value: '14:22:04 \u00B7 0 mismatches' },
    {
      label: 'Chain breaks',
      value: (
        <>
          <StatusPill status="broken">1</StatusPill> on E-0908 &mdash; isolated,
          pending re-ingest
        </>
      ),
    },
    { label: 'Storage backend', value: 'MinIO \u00B7 EU-WEST-2 \u00B7 1.8 TB used' },
    { label: 'Total items', value: formatNumber(totalCount) },
    {
      label: 'Validator',
      value: <LinkArrow href="/validator">Offline verify (0.3 MB binary)</LinkArrow>,
    },
  ];
}

// --- Props ---

export interface EvidenceViewProps {
  readonly evidenceItems: readonly EvidenceItem[];
  readonly totalCount: number;
  readonly facets?: Readonly<Record<string, Record<string, number>>>;
  readonly error?: string | null;
}

// --- Evidence card (server-rendered, matches design prototype exactly) ---

function EvidenceCardItem({ item }: { readonly item: EvidenceItem }) {
  const { kind, glyph } = mimeToKind(item.mime_type);
  const st = classificationToStatus(item.classification, item.destroyed_at);
  const ext = mimeToExtension(item.mime_type, item.original_name);
  const meta = `${formatBytes(item.size_bytes)} \u00B7 ${ext}`;
  const phaseMax = item.bp_phase_max ?? BP_PHASE_MAX;
  const phase = item.bp_phase ?? 0;
  const color = bpColor(phase);

  return (
    <a
      href={`/en/evidence/${item.id}`}
      className="ev-card"
      style={{ textDecoration: 'none', color: 'inherit' }}
    >
      <div className="ev-thumb">
        <span className="chip-k">{kind}</span>
        <span className="chip-s">
          <StatusPill status={st as 'sealed' | 'hold' | 'draft' | 'broken'}>
            {STATUS_LABEL[st]}
          </StatusPill>
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
            <Tag key={t}>{t}</Tag>
          ))}
        </div>
        {phase > 0 && (
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
              {Array.from({ length: phaseMax }, (_, i) => (
                <span
                  key={i}
                  style={{
                    width: 6,
                    height: 6,
                    borderRadius: '50%',
                    background: i < phase ? color : 'var(--bg-2)',
                  }}
                />
              ))}
            </div>
            <span
              style={{
                fontFamily: "'JetBrains Mono', monospace",
                fontSize: '9.5px',
                letterSpacing: '.04em',
                color,
              }}
            >
              {phase}/{phaseMax} phases
            </span>
          </div>
        )}
      </div>
    </a>
  );
}

// --- Upload queue item ---

function UploadQueueItem({
  item,
}: {
  readonly item: (typeof UPLOAD_QUEUE)[number];
}) {
  return (
    <div
      style={{
        display: 'grid',
        gridTemplateColumns: '28px 1fr auto',
        gap: 12,
        alignItems: 'center',
      }}
    >
      <span
        style={{
          width: 28,
          height: 28,
          borderRadius: 7,
          background: 'var(--bg-2)',
          display: 'grid',
          placeItems: 'center',
          fontFamily: "'Fraunces', serif",
          fontSize: 14,
          color: 'var(--accent)',
        }}
      >
        &uarr;
      </span>
      <div>
        <div
          style={{
            fontFamily: "'Fraunces', serif",
            fontSize: '14.5px',
            letterSpacing: '-.005em',
          }}
        >
          {item.name}
        </div>
        <div
          style={{
            height: 4,
            background: 'var(--bg-2)',
            borderRadius: 2,
            overflow: 'hidden',
            marginTop: 8,
          }}
        >
          <div
            style={{
              height: '100%',
              width: `${item.pct}%`,
              background: item.pct === 100 ? 'var(--ok)' : 'var(--accent)',
            }}
          />
        </div>
        <div
          style={{
            fontFamily: "'JetBrains Mono', monospace",
            fontSize: '10.5px',
            color: 'var(--muted)',
            marginTop: 4,
            letterSpacing: '.04em',
            textTransform: 'uppercase',
          }}
        >
          {item.size} &middot; {item.status}
        </div>
      </div>
      <span
        style={{
          fontFamily: "'JetBrains Mono', monospace",
          fontSize: 11,
          color: 'var(--muted)',
        }}
      >
        {item.pct}%
      </span>
    </div>
  );
}

// --- Main view ---

export function EvidenceView({
  evidenceItems,
  totalCount,
  facets,
  error,
}: EvidenceViewProps) {
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

  const chipCounts = {
    all: totalCount,
    doc: facets?.mime_type ? undefined : docCount || undefined,
    media: mediaCount || undefined,
    audio: audioCount || undefined,
    forensic: forensicCount || undefined,
    hold: holdCount || undefined,
  };

  return (
    <>
      {/* Page header */}
      <section className="d-pagehead">
        <div>
          <EyebrowLabel>Berkeley Protocol phases 2&ndash;4</EyebrowLabel>
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

      {/* Error banner */}
      {error && (
        <div className="banner-error" style={{ marginBottom: 16 }}>
          {error}
        </div>
      )}

      {/* Evidence grid panel */}
      <div className="panel" style={{ marginBottom: 22 }}>
        {/* Filter bar (client component for interactivity) */}
        <EvidenceFilterBar chipCounts={chipCounts} />

        <div className="panel-body">
          {evidenceItems.length === 0 ? (
            <div
              style={{
                padding: '48px 0',
                textAlign: 'center',
                color: 'var(--muted)',
                fontFamily: "'JetBrains Mono', monospace",
                fontSize: 13,
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
              {evidenceItems.map((item) => (
                <EvidenceCardItem key={item.id} item={item} />
              ))}
            </div>
          )}

          {/* Pagination */}
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

      {/* Bottom two-column panels */}
      <div className="g2-wide">
        {/* Upload queue */}
        <Panel title="Upload queue" meta="3 in flight">
          <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
            {UPLOAD_QUEUE.map((u) => (
              <UploadQueueItem key={u.name} item={u} />
            ))}
          </div>
        </Panel>

        {/* Integrity summary */}
        <Panel title="Integrity summary" meta="real-time">
          <KeyValueList items={[...integrityItems(totalCount)]} />
        </Panel>
      </div>
    </>
  );
}

export default EvidenceView;
