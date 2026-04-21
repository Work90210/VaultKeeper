'use client';

import { useCallback, useMemo, useRef, useState } from 'react';
import { useRouter, useParams } from 'next/navigation';
import type { EvidenceItem, CaptureMetadata, RedactionDraft } from '@/types';
import { PLATFORMS, CAPTURE_METHODS, VERIFICATION_STATUSES, AVAILABILITY_STATUSES } from '@/types';
import { RedactionEditor } from '@/components/redaction/redaction-editor';
import { CollaborativeEditor } from '@/components/redaction/collaborative-editor';
import { DraftPicker } from '@/components/redaction/draft-picker';
import {
  BPIndicator,
  KeyValueList,
  StatusPill,
  Tag,
  Modal,
  LinkArrow,
} from '@/components/ui/dashboard';
import {
  mimeLabel,
} from '@/lib/evidence-utils';
import { EvidenceDetailTabs } from './evidence-detail-tabs';

/* ---------- helpers ---------- */

function formatDate(d: string | null | undefined): string {
  if (!d) return '\u2014';
  const dt = new Date(d);
  return dt.toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' })
    + ', ' + dt.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' });
}

function formatBytes(b: number): string {
  if (b > 1e9) return (b / 1e9).toFixed(1) + ' GB';
  if (b > 1e6) return (b / 1e6).toFixed(0) + ' MB';
  if (b > 1e3) return (b / 1e3).toFixed(0) + ' KB';
  return b + ' B';
}

/* ---------- stub data (matching design prototype) ---------- */

const STUB_CUSTODY = [
  { id: 'c-1', action: 'Evidence sealed', detail: 'SHA-256 verified \u00b7 RFC 3161 timestamp \u00b7 block f209', actor: 'System', ts: '18 Apr 2026, 11:42:04', type: 'seal' },
  { id: 'c-2', action: 'Uploaded v3', detail: '228 MB \u00b7 SHA-256 a7f2e4\u2026 \u00b7 replaced v2 (colour correction)', actor: 'H. Morel', ts: '18 Apr 2026, 11:42:00', type: 'upload' },
  { id: 'c-3', action: 'Downloaded for analysis', detail: 'Exported to secure workstation WS-017 \u00b7 logged', actor: 'Amir Haddad', ts: '18 Apr 2026, 14:08', type: 'download' },
  { id: 'c-4', action: 'Viewed in browser', detail: 'Preview only \u00b7 no export \u00b7 session 4f2a\u2026', actor: 'Juliane Wirth', ts: '18 Apr 2026, 15:22', type: 'view' },
  { id: 'c-5', action: 'Geolocation verified', detail: 'EXIF GPS cross-referenced with Sentinel-2 \u00b7 match confirmed', actor: 'Amir Haddad', ts: '18 Apr 2026, 16:10', type: 'seal' },
  { id: 'c-6', action: 'Redaction draft created', detail: 'Draft "Defence disclosure" \u00b7 2 areas marked \u00b7 faces', actor: 'Martyna Kovacs', ts: '19 Apr 2026, 09:14', type: 'redact' },
  { id: 'c-7', action: 'Viewed in browser', detail: 'Preview \u00b7 session 8e1b\u2026', actor: 'H. Morel', ts: '19 Apr 2026, 10:30', type: 'view' },
  { id: 'c-8', action: 'Downloaded for court prep', detail: 'Exported for disclosure package DISC-2026-019', actor: 'H. Morel', ts: '19 Apr 2026, 13:45', type: 'download' },
  { id: 'c-9', action: 'Assessment created', detail: 'Relevance: 9/10 \u00b7 Reliability: 8/10 \u00b7 Source: Established', actor: 'Amir Haddad', ts: '19 Apr 2026, 14:02', type: 'seal' },
  { id: 'c-10', action: 'Corroboration linked', detail: 'Linked to claim C-0412 as primary evidence', actor: 'H. Morel', ts: '19 Apr 2026, 14:18', type: 'seal' },
] as const;

const _STUB_VERSIONS = [
  { id: 'e-0918-v3', version: 3, filename: 'Butcha_drone_04_v3.mp4', size: '218 MB', hash: 'a7f2e4c9\u2026', date: '18 Apr 2026', by: 'H. Morel', current: true, note: 'Colour-corrected, stabilised' },
  { id: 'e-0918-v2', version: 2, filename: 'Butcha_drone_04_v2.mp4', size: '224 MB', hash: 'd91e7b2a\u2026', date: '16 Apr 2026', by: 'H. Morel', current: false, note: 'Stabilised version' },
  { id: 'e-0918-v1', version: 1, filename: 'DJI_0487.MP4', size: '412 MB', hash: '44bfe892\u2026', date: '15 Apr 2026', by: 'H. Morel', current: false, note: 'Original capture from drone SD card' },
] as const;

const _STUB_REDACTIONS = {
  drafts: [
    { id: 'rd-1', name: 'Defence disclosure', purpose: 'disclosure_defence', areas: 2, by: 'Martyna Kovacs', saved: '19 Apr, 09:14', status: 'draft' },
    { id: 'rd-2', name: 'Public release', purpose: 'public_release', areas: 5, by: 'Martyna Kovacs', saved: '19 Apr, 11:30', status: 'draft' },
  ],
  finalized: [
    { id: 'rf-1', name: 'Court submission v1', purpose: 'court_submission', areas: 3, by: 'Martyna Kovacs', date: '17 Apr 2026', evidenceNumber: 'E-0918-R1' },
  ],
} as const;

const STUB_ASSESSMENTS = [
  { id: 'a-1', relevance: 9, reliability: 8, credibility: 'Established', recommendation: 'Collect', by: 'Amir Haddad', date: '19 Apr 2026' },
] as const;

const STUB_VERIFICATIONS = [
  { id: 'v-1', type: 'Geolocation Verification', finding: 'Authentic', confidence: 'High', method: 'EXIF GPS cross-referenced with Sentinel-2 satellite imagery', by: 'Amir Haddad', date: '18 Apr 2026' },
  { id: 'v-2', type: 'Chronolocation', finding: 'Likely Authentic', confidence: 'Medium', method: 'Shadow analysis consistent with reported time. Corroborated by W-0144 timeline.', by: 'Amir Haddad', date: '19 Apr 2026' },
] as const;

const _PURPOSE_LABELS: Record<string, string> = {
  disclosure_defence: 'Defence',
  disclosure_prosecution: 'Prosecution',
  public_release: 'Public',
  court_submission: 'Court',
  witness_protection: 'Witness',
  internal_review: 'Internal',
};

/* ---------- BP phase computation ---------- */

interface BPPhaseInput {
  readonly name: string;
  readonly status: 'complete' | 'in_progress' | 'not_started';
  readonly pct?: number;
  readonly items?: string[];
  readonly missing?: string[];
  readonly action?: string;
}

function computeStubBPPhases(): readonly BPPhaseInput[] {
  return [
    {
      name: 'Online inquiry',
      status: 'complete',
      items: ['Search strategy documented', 'Tools & parameters logged', 'Discovery timeline: 18 Apr 10:00\u201310:30'],
      missing: [],
    },
    {
      name: 'Assessment',
      status: 'complete',
      items: ['Relevance: 9/10', 'Reliability: 8/10', 'Source credibility: Established'],
      missing: [],
    },
    {
      name: 'Collection',
      status: 'complete',
      items: ['Method: manual download', 'Captured: 18 Apr 2026', 'Collector: H\u00e9l\u00e8ne Morel'],
      missing: [],
    },
    {
      name: 'Preservation',
      status: 'complete',
      items: ['SHA-256 sealed', 'RFC 3161 timestamped', 'Custody chain: 10 events'],
      missing: [],
    },
    {
      name: 'Verification',
      status: 'complete',
      items: ['Geolocation Verification: Authentic', 'Chronolocation: Likely Authentic'],
      missing: [],
    },
    {
      name: 'Analysis',
      status: 'in_progress',
      pct: 60,
      items: ['Corroboration C-0412 linked', '2 witness cross-refs'],
      missing: ['Final analytical note pending'],
    },
  ];
}

function _computeLiveBPPhases(
  e: EvidenceItem,
  cm: CaptureMetadata | null,
): readonly BPPhaseInput[] {
  const hasEvidence = !!e.id;
  const hasCaptureMetadata = !!(cm?.capture_method && cm?.capture_timestamp && cm?.collector_display_name);
  const hasHash = !!e.sha256_hash;
  const hasTSA = !!e.tsa_timestamp;

  return [
    {
      name: 'Online inquiry',
      status: hasEvidence ? 'complete' : 'not_started',
      items: hasEvidence
        ? ['Search strategy documented', 'Tools & parameters logged', `Discovery timeline: ${formatDate(e.created_at).split(',')[0]}`]
        : ['No inquiry log yet'],
      missing: [],
    },
    {
      name: 'Assessment',
      status: STUB_ASSESSMENTS.length > 0 ? 'complete' : 'not_started',
      items: STUB_ASSESSMENTS.length
        ? [`Relevance: ${STUB_ASSESSMENTS[0].relevance}/10`, `Reliability: ${STUB_ASSESSMENTS[0].reliability}/10`, `Source credibility: ${STUB_ASSESSMENTS[0].credibility}`]
        : ['No assessment yet'],
      missing: STUB_ASSESSMENTS.length ? [] : ['Relevance score'],
    },
    {
      name: 'Collection',
      status: hasEvidence ? 'complete' : 'not_started',
      items: hasCaptureMetadata && cm
        ? [`Method: ${(cm.capture_method || '').replace(/_/g, ' ')}`, `Captured: ${cm.capture_timestamp ? new Date(cm.capture_timestamp).toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' }) : '\u2014'}`, `Collector: ${cm.collector_display_name || '\u2014'}`]
        : [`Uploaded: ${formatDate(e.created_at).split(',')[0]}`, `File: ${e.filename}`, `Size: ${formatBytes(e.size_bytes)}`],
      missing: hasCaptureMetadata ? [] : ['Add capture metadata'],
    },
    {
      name: 'Preservation',
      status: hasHash ? 'complete' : 'not_started',
      items: [
        hasHash ? 'SHA-256 sealed' : 'SHA-256 pending',
        hasTSA ? 'RFC 3161 timestamped' : 'TSA pending',
        `Custody chain: ${STUB_CUSTODY.length} events`,
      ],
      missing: hasHash ? [] : ['Awaiting seal'],
    },
    {
      name: 'Verification',
      status: STUB_VERIFICATIONS.length > 0 ? 'complete' : 'not_started',
      items: STUB_VERIFICATIONS.length > 0
        ? STUB_VERIFICATIONS.map(v => `${v.type}: ${v.finding}`)
        : ['No verifications yet'],
      missing: [],
    },
    {
      name: 'Analysis',
      status: 'in_progress',
      pct: 60,
      items: ['Corroboration C-0412 linked', '2 witness cross-refs'],
      missing: ['Final analytical note pending'],
    },
  ];
}

/* ================================================================
   Main component
   ================================================================ */

export function EvidenceDetail({
  evidence,
  canEdit,
  accessToken,
  username,
  caseReferenceCode,
}: {
  readonly evidence: EvidenceItem;
  readonly canEdit: boolean;
  readonly accessToken?: string;
  readonly username?: string;
  readonly caseReferenceCode: string;
}) {
  const router = useRouter();
  const params = useParams();
  const locale = (params?.locale as string) || 'en';
  const fileInputRef = useRef<HTMLInputElement>(null);

  /* --- local state --- */
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [showRedactor, setShowRedactor] = useState(false);
  const [showDraftPicker, setShowDraftPicker] = useState(false);
  const [selectedDraft, setSelectedDraft] = useState<RedactionDraft | null>(null);
  const [totalPages, setTotalPages] = useState<number | null>(null);

  // Modals
  const [showHoldModal, setShowHoldModal] = useState(false);
  const [showDestroyModal, setShowDestroyModal] = useState(false);
  const [showRedactModal, setShowRedactModal] = useState(false);
  const [showVersionModal, setShowVersionModal] = useState(false);

  /* --- BP phases --- */
  const bpPhases = useMemo(() => computeStubBPPhases(), []);
  const bpDone = bpPhases.filter(p => p.status === 'complete').length;
  const bpTotal = bpPhases.length;
  const bpPct = Math.round((bpDone / bpTotal) * 100);

  /* --- waveform bars (memoized) --- */
  const waveformBars = useMemo(() => {
    const bars: number[] = [];
    for (let i = 0; i < 80; i++) {
      bars.push(Math.floor(Math.random() * 30) + 6);
    }
    return bars;
  }, []);

  /* --- version upload --- */
  const handleVersionUpload = useCallback(async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file || !accessToken) return;
    setUploading(true);
    setUploadError(null);
    const formData = new FormData();
    formData.append('file', file);
    formData.append('classification', evidence.classification);
    try {
      const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';
      const res = await fetch(`${API_BASE}/api/evidence/${evidence.id}/version`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${accessToken}` },
        body: formData,
      });
      if (!res.ok) {
        const data = await res.json().catch(() => null);
        setUploadError(data?.error || 'Upload failed');
      } else {
        const newEvidence = await res.json().catch(() => null);
        if (newEvidence?.data?.id) {
          router.push(`/${locale}/evidence/${newEvidence.data.id}`);
        } else {
          router.refresh();
        }
      }
    } catch {
      setUploadError('An unexpected error occurred.');
    } finally {
      setUploading(false);
      if (fileInputRef.current) fileInputRef.current.value = '';
    }
  }, [accessToken, evidence.id, evidence.classification, router, locale]);

  /* --- draft resume handler --- */
  const handleResumeDraft = useCallback(async (draft: RedactionDraft) => {
    setSelectedDraft(draft);
    try {
      const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';
      const res = await fetch(`${API_BASE}/api/evidence/${evidence.id}/page-count`, {
        headers: { Authorization: `Bearer ${accessToken || ''}` },
      });
      if (res.ok) {
        const json = await res.json();
        setTotalPages(json.data?.page_count ?? 1);
      } else {
        setTotalPages(1);
      }
    } catch {
      setTotalPages(1);
    }
    setShowRedactor(true);
  }, [evidence.id, accessToken]);

  /* --- derived values --- */
  const exif = evidence.metadata?.exif as Record<string, unknown> | undefined;
  const meta = evidence.metadata as Record<string, unknown> | undefined;

  /* --- sidebar data builders --- */
  const fileItems = useMemo(() => [
    { label: 'Name', value: evidence.filename },
    { label: 'Original', value: evidence.original_name || '\u2014' },
    { label: 'Size', value: formatBytes(evidence.size_bytes) },
    { label: 'Type', value: <code>{evidence.mime_type}</code> },
    { label: 'Version', value: <code>v{evidence.version}</code> },
    { label: 'Classification', value: evidence.classification.replace('_', ' ') },
  ], [evidence]);

  const dateItems = useMemo(() => [
    { label: 'Uploaded', value: formatDate(evidence.created_at) },
    { label: 'By', value: evidence.uploaded_by_name || evidence.uploaded_by },
    { label: 'Source', value: evidence.source || '\u2014' },
    { label: 'Source date', value: formatDate(evidence.source_date) },
  ], [evidence]);

  const integrityItems = useMemo(() => [
    { label: 'TSA', value: evidence.tsa_timestamp ? <StatusPill status="sealed">Verified</StatusPill> : <StatusPill status="draft">Pending</StatusPill> },
    { label: 'Authority', value: <code>{evidence.tsa_name || '\u2014'}</code> },
    { label: 'Stamped', value: formatDate(evidence.tsa_timestamp) },
  ], [evidence]);

  const cm = evidence.capture_metadata ?? null;
  const methodLabel = cm ? (CAPTURE_METHODS.find(m => m.value === cm.capture_method)?.label || cm.capture_method.replace(/_/g, ' ')) : '\u2014';
  const platformLabel = cm ? (PLATFORMS.find(p => p.value === cm.platform)?.label || cm.platform || '\u2014') : '\u2014';
  const verificationLabel = cm?.verification_status ? (VERIFICATION_STATUSES.find(v => v.value === cm.verification_status)?.label || cm.verification_status.replace(/_/g, ' ')) : '\u2014';
  const availabilityLabel = cm?.availability_status
    ? AVAILABILITY_STATUSES.find(a => a.value === cm.availability_status)?.label || cm.availability_status
    : null;

  const provenanceItems = useMemo(() => {
    const items = [
      { label: 'Platform', value: platformLabel },
      { label: 'Method', value: methodLabel },
      { label: 'Captured', value: formatDate(cm?.capture_timestamp) },
      { label: 'Published', value: formatDate(cm?.publication_timestamp) },
      { label: 'Collector', value: cm?.collector_display_name || '\u2014' },
      { label: 'Language', value: cm?.content_language || '\u2014' },
      { label: 'Location', value: cm?.geo_place_name || '\u2014' },
    ];
    if (cm?.geo_latitude != null && cm?.geo_longitude != null) {
      items.push({ label: 'Coordinates', value: `${cm.geo_latitude.toFixed(4)}, ${cm.geo_longitude.toFixed(4)}` });
    }
    items.push(
      { label: 'Geo source', value: cm?.geo_source || '\u2014' },
      { label: 'Availability', value: availabilityLabel || '\u2014' },
      { label: 'Tool', value: cm?.capture_tool_name ? `${cm.capture_tool_name} ${cm.capture_tool_version || ''}` : '\u2014' },
    );
    return items;
  }, [cm, platformLabel, methodLabel, availabilityLabel]);

  const exifItems = useMemo(() => [
    { label: 'Camera', value: exif?.camera_make ? `${String(exif.camera_make)} ${String(exif.camera_model || '')}` : '\u2014' },
    { label: 'Capture date', value: exif?.capture_date ? String(exif.capture_date) : '\u2014' },
    { label: 'Focal length', value: exif?.focal_length ? String(exif.focal_length) : '\u2014' },
    { label: 'GPS', value: exif?.gps_latitude != null ? <code>{`${String(exif.gps_latitude)}, ${String(exif.gps_longitude)}`}</code> : '\u2014' },
    { label: 'Resolution', value: meta?.resolution ? <code>{String(meta.resolution)}</code> : '\u2014' },
    { label: 'Codec', value: meta?.codec ? <code>{String(meta.codec)}</code> : '\u2014' },
    { label: 'FPS', value: meta?.fps ? <code>{String(meta.fps)}</code> : '\u2014' },
  ], [exif, meta]);

  /* --- custody timeline items --- */
  const custodyTimelineItems = useMemo(() =>
    STUB_CUSTODY.map(c => ({
      content: <strong>{c.action}</strong>,
      subline: `${c.detail} \u00b7 ${c.actor}`,
      time: c.ts,
      accent: c.type === 'upload' || c.type === 'seal',
    })),
  []);

  /* --- sidebar compliance checklist --- */
  const compliancePhases = useMemo(() =>
    bpPhases.map((p, i) => {
      const icon = p.status === 'complete'
        ? <span style={{ color: 'var(--ok)' }}>{'\u2713'}</span>
        : (p.status === 'in_progress' ? <span style={{ color: 'var(--accent)' }}>{'\u25D4'}</span> : <span style={{ color: 'var(--muted-2)' }}>{'\u25CB'}</span>);
      return (
        <div key={i} style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '4px 0', fontSize: 12 }}>
          {icon}
          <span style={{ color: 'var(--ink-2)' }}>Phase {i + 1}: {p.name}</span>
        </div>
      );
    }),
  [bpPhases]);

  const vColor = cm?.verification_status === 'verified' ? 'var(--ok)' : cm?.verification_status === 'disputed' ? 'var(--accent)' : 'var(--muted)';

  return (
    <div className="d-content">
      {/* ---- Draft picker dialog ---- */}
      {showDraftPicker && accessToken && (
        <DraftPicker
          evidenceId={evidence.id}
          accessToken={accessToken}
          onSelect={async (draft) => {
            setShowDraftPicker(false);
            await handleResumeDraft(draft);
          }}
          onClose={() => setShowDraftPicker(false)}
        />
      )}

      {/* ---- Redaction editors ---- */}
      {showRedactor && accessToken && evidence.mime_type === 'application/pdf' && totalPages !== null && selectedDraft && (
        <CollaborativeEditor
          evidenceId={evidence.id}
          draftId={selectedDraft.id}
          draftName={selectedDraft.name}
          draftPurpose={selectedDraft.purpose}
          totalPages={totalPages}
          accessToken={accessToken}
          username={username || 'User'}
          onClose={() => { setShowRedactor(false); setSelectedDraft(null); }}
          onApplied={(newEvidenceId) => {
            setShowRedactor(false);
            setSelectedDraft(null);
            router.push(`/${locale}/evidence/${newEvidenceId}`);
          }}
        />
      )}
      {showRedactor && accessToken && evidence.mime_type !== 'application/pdf' && (
        <RedactionEditor
          evidenceId={evidence.id}
          imageUrl={`/api/evidence/${evidence.id}/download`}
          mimeType={evidence.mime_type}
          accessToken={accessToken}
          onApply={async (redactions) => {
            const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';
            const res = await fetch(`${API_BASE}/api/evidence/${evidence.id}/redact`, {
              method: 'POST',
              headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${accessToken}` },
              body: JSON.stringify({
                redactions: redactions.map((r) => ({
                  page_number: r.pageNumber,
                  x: r.x,
                  y: r.y,
                  width: r.width,
                  height: r.height,
                  reason: r.reason,
                })),
              }),
            });
            if (!res.ok) {
              const body = await res.json().catch(() => null);
              throw new Error(body?.error || 'Redaction failed');
            }
            setShowRedactor(false);
            router.refresh();
          }}
          onClose={() => setShowRedactor(false)}
        />
      )}

      {/* ---- Header ---- */}
      <div className="ev-header">
        <div className="ev-badges">
          <span className="ev-num">{evidence.evidence_number} &middot; v{evidence.version}</span>
          {evidence.tsa_timestamp && <StatusPill status="sealed">Sealed</StatusPill>}
          {!evidence.is_current && <StatusPill status="draft">Superseded</StatusPill>}
          {evidence.destroyed_at && <StatusPill status="broken">Destroyed</StatusPill>}
          <Tag className="capitalize">{evidence.classification.replace('_', ' ')}</Tag>
        </div>
        <h1><em className="a">{evidence.title || evidence.filename}</em></h1>
        {evidence.description && <p className="ev-desc">{evidence.description}</p>}
        <div className="ev-actions">
          <a className="btn ghost sm" href={`/${locale}/evidence`}>&larr; Back to evidence</a>
          <a className="btn ghost sm" href={`/api/evidence/${evidence.id}/download`} download>Download</a>
          {canEdit && evidence.is_current && !evidence.destroyed_at && (
            <>
              <input ref={fileInputRef} type="file" onChange={handleVersionUpload} style={{ display: 'none' }} />
              <button
                type="button"
                className="btn sm"
                onClick={() => {
                  if (evidence.mime_type === 'application/pdf') {
                    setShowDraftPicker(true);
                  } else {
                    setShowRedactor(true);
                  }
                }}
              >
                Redact
              </button>
            </>
          )}
        </div>
      </div>

      {uploadError && (
        <div className="banner-error" style={{ marginBottom: 22 }}>{uploadError}</div>
      )}

      {/* ---- Berkeley Protocol 6-Phase Tracker (full-width) ---- */}
      <BPIndicator phases={bpPhases as BPPhaseInput[]} variant="full" />

      {/* ---- Layout: main + sidebar ---- */}
      <div className="ev-layout">
        <div className="ev-main">
          {/* Preview */}
          <EvidencePreview evidence={evidence} waveformBars={waveformBars} />

          {/* Tags */}
          {evidence.tags.length > 0 && (
            <div className="ev-tags">
              {evidence.tags.map((t) => (
                <Tag key={t}>{t}</Tag>
              ))}
            </div>
          )}

          {/* Tabs + Tab content */}
          <EvidenceDetailTabs
            evidence={evidence}
            custodyItems={custodyTimelineItems}
            onShowHold={() => setShowHoldModal(true)}
            onShowDestroy={() => setShowDestroyModal(true)}
            onShowVersion={() => setShowVersionModal(true)}
            onShowRedact={() => setShowRedactModal(true)}
          />
        </div>

        {/* ---- Sidebar (metadata only) ---- */}
        <div className="ev-sidebar">
          {/* File */}
          <div className="sb">
            <div className="sb-label">File</div>
            <KeyValueList items={fileItems} />
          </div>

          {/* Dates */}
          <div className="sb">
            <div className="sb-label">Dates</div>
            <KeyValueList items={dateItems} />
          </div>

          {/* Integrity */}
          <div className="sb">
            <div className="sb-label">Integrity</div>
            <div style={{ marginBottom: 10 }}>
              <div style={{ fontFamily: '"JetBrains Mono", monospace', fontSize: 9, color: 'var(--muted-2)', letterSpacing: '.08em', textTransform: 'uppercase', marginBottom: 4 }}>SHA-256</div>
              <div className="hash-display">{evidence.sha256_hash}</div>
            </div>
            <KeyValueList items={integrityItems} />
          </div>

          {/* Provenance (Berkeley) */}
          <div className="sb">
            <div className="sb-label">Provenance <span className="bp-tag">Berkeley</span></div>
            <KeyValueList items={provenanceItems} />
            <div style={{ marginTop: 8 }}>
              <div style={{ fontFamily: '"JetBrains Mono", monospace', fontSize: 10, color: 'var(--muted)', letterSpacing: '.06em', textTransform: 'uppercase', marginBottom: 4 }}>Verification</div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 4 }}>
                <span style={{ width: 6, height: 6, borderRadius: '50%', background: vColor }} />
                <span style={{ fontSize: 13, color: vColor, fontWeight: 500 }}>{verificationLabel}</span>
              </div>
              {cm?.verification_notes && (
                <div style={{ fontSize: 12, color: 'var(--muted)', lineHeight: 1.5 }}>{cm.verification_notes}</div>
              )}
            </div>
          </div>

          {/* EXIF */}
          <div className="sb">
            <div className="sb-label">EXIF</div>
            <KeyValueList items={exifItems} />
          </div>

          {/* Berkeley Protocol compliance */}
          <div className="sb">
            <div className="sb-label">Berkeley Protocol <span className="bp-tag">Compliance</span></div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 12 }}>
              <div style={{ fontFamily: '"Fraunces", serif', fontSize: 32, letterSpacing: '-.02em', color: bpPct >= 80 ? 'var(--ok)' : bpPct >= 50 ? 'var(--accent)' : 'var(--muted)' }}>
                {bpDone}<span style={{ fontSize: 16, color: 'var(--muted)' }}>/{bpTotal}</span>
              </div>
              <div style={{ flex: 1 }}>
                <div style={{ fontSize: 12, color: 'var(--muted)', marginBottom: 4 }}>Phases complete</div>
                <div style={{ height: 6, background: 'var(--bg-2)', borderRadius: 3, overflow: 'hidden' }}>
                  <div style={{ height: '100%', width: `${bpPct}%`, background: bpPct >= 80 ? 'var(--ok)' : 'var(--accent)', borderRadius: 3 }} />
                </div>
              </div>
            </div>
            {compliancePhases}
          </div>

          {/* Linked */}
          <div className="sb">
            <div className="sb-label">Linked</div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 7, fontSize: 13 }}>
              <LinkArrow href="#">Corroboration C-0412</LinkArrow>
              <LinkArrow href="#">Witness W-0144</LinkArrow>
              <LinkArrow href="#">Witness W-0099</LinkArrow>
              <LinkArrow href="#">Disclosure DISC-2026-019</LinkArrow>
            </div>
          </div>
        </div>
      </div>

      {/* ---- Modals ---- */}
      <LegalHoldModal
        open={showHoldModal}
        evidenceNumber={evidence.evidence_number}
        caseReferenceCode={caseReferenceCode}
        onClose={() => setShowHoldModal(false)}
      />
      <DestroyModal
        open={showDestroyModal}
        evidence={evidence}
        onClose={() => setShowDestroyModal(false)}
      />
      <RedactModal
        open={showRedactModal}
        onClose={() => setShowRedactModal(false)}
      />
      <VersionUploadModal
        open={showVersionModal}
        evidence={evidence}
        uploading={uploading}
        fileInputRef={fileInputRef}
        onClose={() => setShowVersionModal(false)}
      />
    </div>
  );
}

/* ================================================================
   Evidence Preview
   ================================================================ */

function EvidencePreview({ evidence, waveformBars }: { readonly evidence: EvidenceItem; readonly waveformBars: number[] }) {
  const downloadUrl = `/api/evidence/${evidence.id}/download`;
  const meta = evidence.metadata as Record<string, unknown> | undefined;
  const resolution = meta?.resolution as string | undefined;
  const codec = meta?.codec as string | undefined;
  const duration = meta?.duration as string | undefined;

  if (evidence.mime_type.startsWith('image/')) {
    return (
      <div className="ev-preview" style={{ marginBottom: 24 }}>
        <span className="file-badge">IMAGE &middot; {evidence.mime_type.split('/')[1]?.toUpperCase()}{resolution ? ` \u00b7 ${resolution}` : ''}</span>
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img src={downloadUrl} alt={evidence.title || evidence.filename} style={{ maxHeight: 500, maxWidth: '100%', objectFit: 'contain' }} />
      </div>
    );
  }

  if (evidence.mime_type.startsWith('video/')) {
    return (
      <div className="ev-preview" style={{ marginBottom: 24 }}>
        <span className="file-badge">VIDEO{codec ? ` \u00b7 ${codec}` : ''}{resolution ? ` \u00b7 ${resolution}` : ''}</span>
        {duration && <span className="dur-badge">{duration}</span>}
        <button className="play-btn" type="button">
          <svg width="26" height="26" viewBox="0 0 24 24" fill="rgba(245,241,232,.9)" stroke="none"><polygon points="8,5 19,12 8,19" /></svg>
        </button>
        <div className="waveform">
          {waveformBars.map((h, i) => <span key={i} style={{ height: `${h}px` }} />)}
        </div>
      </div>
    );
  }

  if (evidence.mime_type.startsWith('audio/')) {
    return (
      <div className="ev-preview" style={{ marginBottom: 24 }}>
        <span className="file-badge">AUDIO &middot; {evidence.mime_type.split('/')[1]?.toUpperCase()}</span>
        {duration && <span className="dur-badge">{duration}</span>}
        <button className="play-btn" type="button">
          <svg width="26" height="26" viewBox="0 0 24 24" fill="rgba(245,241,232,.9)" stroke="none"><polygon points="8,5 19,12 8,19" /></svg>
        </button>
        <div className="waveform">
          {waveformBars.map((h, i) => <span key={i} style={{ height: `${h}px` }} />)}
        </div>
      </div>
    );
  }

  // Generic file preview
  return (
    <div className="ev-preview" style={{ marginBottom: 24, minHeight: 180 }}>
      <span className="file-badge">{mimeLabel(evidence.mime_type).toUpperCase()} &middot; {evidence.mime_type}</span>
      <div style={{ fontFamily: '"Fraunces", serif', fontSize: 48, color: 'rgba(245,241,232,.15)', fontStyle: 'italic' }}>
        {mimeLabel(evidence.mime_type)}
      </div>
    </div>
  );
}

/* ================================================================
   Modals (using shared Modal component)
   ================================================================ */

function LegalHoldModal({ open, evidenceNumber, caseReferenceCode, onClose }: {
  readonly open: boolean;
  readonly evidenceNumber: string;
  readonly caseReferenceCode: string;
  readonly onClose: () => void;
}) {
  return (
    <Modal open={open} onClose={onClose} title="Place legal hold">
      <p style={{ fontSize: 13, color: 'var(--muted)', marginBottom: 18, lineHeight: 1.6 }}>
        This will freeze {evidenceNumber}. No modifications, deletions, or new versions will be permitted. All case members on {caseReferenceCode} will be notified.
      </p>
      <div style={{ background: 'var(--bg-2)', borderRadius: 10, padding: 14, marginBottom: 18, display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12, fontSize: 13 }}>
        <div>
          <span style={{ fontFamily: '"JetBrains Mono", monospace', fontSize: 10, color: 'var(--muted)', textTransform: 'uppercase', letterSpacing: '.06em', display: 'block', marginBottom: 2 }}>Current</span>
          <span style={{ color: 'var(--ink)' }}>No hold</span>
        </div>
        <div>
          <span style={{ fontFamily: '"JetBrains Mono", monospace', fontSize: 10, color: 'var(--muted)', textTransform: 'uppercase', letterSpacing: '.06em', display: 'block', marginBottom: 2 }}>Will change to</span>
          <span style={{ color: 'var(--accent)', fontWeight: 500 }}>Legal hold active</span>
        </div>
      </div>
      <div style={{ marginBottom: 18 }}>
        <label style={{ fontSize: 13, fontWeight: 500, display: 'block', marginBottom: 6 }}>Reason (required &mdash; sent as notification)</label>
        <textarea
          style={{ width: '100%', padding: '10px 14px', border: '1px solid var(--line-2)', borderRadius: 'var(--radius-sm)', font: 'inherit', fontSize: 13, resize: 'vertical', minHeight: 80, outline: 'none', background: 'var(--paper)' }}
          placeholder="e.g. Pending tribunal review of admissibility..."
        />
      </div>
      <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 10 }}>
        <button type="button" className="btn ghost sm" onClick={onClose}>Cancel</button>
        <button type="button" className="btn sm" onClick={onClose}>Place legal hold</button>
      </div>
    </Modal>
  );
}

function DestroyModal({ open, evidence, onClose }: {
  readonly open: boolean;
  readonly evidence: EvidenceItem;
  readonly onClose: () => void;
}) {
  return (
    <Modal open={open} onClose={onClose} title="Destroy evidence">
      <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 8 }}>
        <span style={{ fontFamily: '"JetBrains Mono", monospace', fontSize: 10, color: 'var(--muted)', letterSpacing: '.06em' }}>Step 1 of 4</span>
      </div>
      <div style={{ padding: 18, border: '2px solid #b35c5c', borderRadius: 10, background: '#fbeee8', marginBottom: 18 }}>
        <p style={{ fontSize: 13, color: '#b35c5c', lineHeight: 1.6, fontWeight: 500 }}>
          This action is irreversible. The file bytes for {evidence.evidence_number} will be permanently destroyed.
        </p>
        <p style={{ fontSize: 12, color: '#8a5a4a', marginTop: 8, lineHeight: 1.5 }}>
          The following will be <strong>preserved</strong>: SHA-256 hash, all metadata, EXIF data, Berkeley Protocol provenance, full custody chain, assessment and verification records, and all redacted derivatives.
        </p>
      </div>
      <div style={{ background: 'var(--bg-2)', borderRadius: 10, padding: 14, marginBottom: 18, fontSize: 13 }}>
        <div style={{ display: 'grid', gridTemplateColumns: '100px 1fr', gap: 8 }}>
          <span style={{ color: 'var(--muted)' }}>Evidence</span><span style={{ fontFamily: '"JetBrains Mono", monospace', fontSize: 12 }}>{evidence.evidence_number}</span>
          <span style={{ color: 'var(--muted)' }}>File</span><span>{evidence.filename}</span>
          <span style={{ color: 'var(--muted)' }}>Hash</span><span style={{ fontFamily: '"JetBrains Mono", monospace', fontSize: 11, wordBreak: 'break-all' }}>{evidence.sha256_hash.slice(0, 12)}&hellip;{evidence.sha256_hash.slice(-4)}</span>
        </div>
      </div>
      <div style={{ marginBottom: 18 }}>
        <label style={{ fontSize: 13, fontWeight: 500, display: 'block', marginBottom: 6 }}>Destruction authority (required, min. 20 characters)</label>
        <textarea
          style={{ width: '100%', padding: '10px 14px', border: '1px solid var(--line-2)', borderRadius: 'var(--radius-sm)', font: 'inherit', fontSize: 13, resize: 'vertical', minHeight: 80, outline: 'none', background: 'var(--paper)' }}
          placeholder="Cite the legal authority or order authorising destruction, e.g. 'Per Registrar order RO-2026-041, dated 18 April 2026, pursuant to Rule 81(4)...'"
        />
      </div>
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: 10 }}>
        <button type="button" className="btn ghost sm" onClick={onClose}>Cancel</button>
        <button type="button" className="btn sm" style={{ background: '#b35c5c', borderColor: '#b35c5c' }}>Continue to step 2 &rarr;</button>
      </div>
    </Modal>
  );
}

function RedactModal({ open, onClose }: {
  readonly open: boolean;
  readonly onClose: () => void;
}) {
  return (
    <Modal open={open} onClose={onClose} title="New redacted version">
      <p style={{ fontSize: 13, color: 'var(--muted)', marginBottom: 18, lineHeight: 1.6 }}>Create a derivative with redacted areas for disclosure. The original is never modified.</p>
      <div style={{ marginBottom: 14 }}>
        <label style={{ fontSize: 13, fontWeight: 500, display: 'block', marginBottom: 6 }}>Name</label>
        <input type="text" style={{ width: '100%', padding: '10px 14px', border: '1px solid var(--line-2)', borderRadius: 'var(--radius-sm)', font: 'inherit', fontSize: 13, outline: 'none', background: 'var(--paper)' }} placeholder="e.g. Defence disclosure \u2014 faces redacted" />
      </div>
      <div style={{ marginBottom: 18 }}>
        <label style={{ fontSize: 13, fontWeight: 500, display: 'block', marginBottom: 6 }}>Purpose</label>
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
          <Tag accent>Defence</Tag>
          <Tag>Prosecution</Tag>
          <Tag>Public</Tag>
          <Tag>Court</Tag>
          <Tag>Witness</Tag>
          <Tag>Internal</Tag>
        </div>
      </div>
      <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 10 }}>
        <button type="button" className="btn ghost sm" onClick={onClose}>Cancel</button>
        <button type="button" className="btn sm" onClick={onClose}>Create draft &amp; open editor &rarr;</button>
      </div>
    </Modal>
  );
}

function VersionUploadModal({ open, evidence, uploading, fileInputRef, onClose }: {
  readonly open: boolean;
  readonly evidence: EvidenceItem;
  readonly uploading: boolean;
  readonly fileInputRef: React.RefObject<HTMLInputElement | null>;
  readonly onClose: () => void;
}) {
  return (
    <Modal open={open} onClose={onClose} title="Upload new version">
      <p style={{ fontSize: 13, color: 'var(--muted)', marginBottom: 18, lineHeight: 1.6 }}>
        Upload a corrected or enhanced version of {evidence.evidence_number}. The current version (v{evidence.version}) will be superseded but preserved. Classification is inherited.
      </p>
      <div
        style={{ border: '2px dashed var(--line-2)', borderRadius: 12, padding: '40px 20px', textAlign: 'center', cursor: 'pointer', transition: 'border-color .2s', marginBottom: 14 }}
        onClick={() => fileInputRef.current?.click()}
      >
        <div style={{ fontFamily: '"Fraunces", serif', fontSize: 28, color: 'var(--accent)', marginBottom: 8 }}>&uarr;</div>
        <div style={{ fontSize: 14, fontWeight: 500, color: 'var(--ink)', marginBottom: 4 }}>Drop file here or click to browse</div>
        <div style={{ fontSize: 12, color: 'var(--muted)' }}>File will be hashed client-side (SHA-256 + BLAKE3) before upload</div>
      </div>
      <div style={{ marginBottom: 18 }}>
        <label style={{ fontSize: 13, fontWeight: 500, display: 'block', marginBottom: 6 }}>Version note</label>
        <input type="text" style={{ width: '100%', padding: '10px 14px', border: '1px solid var(--line-2)', borderRadius: 'var(--radius-sm)', font: 'inherit', fontSize: 13, outline: 'none', background: 'var(--paper)' }} placeholder="e.g. Colour-corrected, enhanced audio" />
      </div>
      <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 10 }}>
        <button type="button" className="btn ghost sm" onClick={onClose}>Cancel</button>
        <button type="button" className="btn sm" style={{ opacity: uploading ? 1 : 0.5, pointerEvents: uploading ? 'auto' : 'none' }}>
          {uploading ? 'Uploading\u2026' : 'Upload & seal'}
        </button>
      </div>
    </Modal>
  );
}
