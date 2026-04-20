'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useRouter, useParams } from 'next/navigation';
import type { EvidenceItem, CustodyEntry, CaptureMetadata, RedactionDraft, RedactionManagementView } from '@/types';
import { PLATFORMS, CAPTURE_METHODS, VERIFICATION_STATUSES, AVAILABILITY_STATUSES } from '@/types';
import { RedactionEditor } from '@/components/redaction/redaction-editor';
import { CollaborativeEditor } from '@/components/redaction/collaborative-editor';
import { DraftPicker } from '@/components/redaction/draft-picker';
import {
  mimeLabel,
} from '@/lib/evidence-utils';

/* ---------- helpers ---------- */

type TabId = 'custody' | 'versions' | 'redactions' | 'verification' | 'manage';

const PURPOSE_LABELS: Record<string, string> = {
  disclosure_defence: 'Defence',
  disclosure_prosecution: 'Prosecution',
  public_release: 'Public',
  court_submission: 'Court',
  witness_protection: 'Witness',
  internal_review: 'Internal',
};

interface VersionEntry {
  readonly id: string;
  readonly version: number;
  readonly filename: string;
  readonly size_bytes: number;
  readonly created_at: string;
  readonly uploaded_by: string;
  readonly uploaded_by_name?: string;
  readonly is_current?: boolean;
  readonly sha256_hash?: string;
  readonly note?: string;
}

interface EvidenceAssessment {
  readonly id: string;
  readonly relevance_score: number;
  readonly reliability_score: number;
  readonly source_credibility: string;
  readonly recommendation: string;
  readonly assessed_by?: string;
  readonly created_at: string;
}

interface VerificationRecord {
  readonly id: string;
  readonly verification_type: string;
  readonly finding: string;
  readonly confidence_level: string;
  readonly methodology?: string;
  readonly verified_by?: string;
  readonly created_at: string;
}

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

function custodyTypeClass(action: string): string {
  const a = action.toLowerCase();
  if (a.includes('seal') || a.includes('verif') || a.includes('assessment') || a.includes('corroboration')) return 'seal';
  if (a.includes('upload')) return 'upload';
  if (a.includes('download') || a.includes('export')) return 'download';
  if (a.includes('view') || a.includes('preview')) return 'view';
  if (a.includes('redact')) return 'redact';
  return 'seal';
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
  evidence: EvidenceItem;
  canEdit: boolean;
  accessToken?: string;
  username?: string;
  caseReferenceCode: string;
}) {
  const router = useRouter();
  const params = useParams();
  const locale = (params?.locale as string) || 'en';
  const fileInputRef = useRef<HTMLInputElement>(null);

  /* --- local state --- */
  const [activeTab, setActiveTab] = useState<TabId>('custody');
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [showRedactor, setShowRedactor] = useState(false);
  const [showDraftPicker, setShowDraftPicker] = useState(false);
  const [selectedDraft, setSelectedDraft] = useState<RedactionDraft | null>(null);
  const [totalPages, setTotalPages] = useState<number | null>(null);
  const [captureMetadata, setCaptureMetadata] = useState<CaptureMetadata | null>(null);
  const [custodyEntries, setCustodyEntries] = useState<readonly CustodyEntry[]>([]);
  const [custodyLoaded, setCustodyLoaded] = useState(false);
  const [custodyError, setCustodyError] = useState<string | null>(null);
  const [versions, setVersions] = useState<readonly VersionEntry[]>([]);
  const [versionsLoaded, setVersionsLoaded] = useState(false);
  const [versionsError, setVersionsError] = useState<string | null>(null);
  const [assessments, setAssessments] = useState<EvidenceAssessment[]>([]);
  const [verifications, setVerifications] = useState<VerificationRecord[]>([]);
  const [investigationLoaded, setInvestigationLoaded] = useState(false);
  const [redactionView, setRedactionView] = useState<RedactionManagementView | null>(null);
  const [redactionViewLoaded, setRedactionViewLoaded] = useState(false);

  // Modals
  const [showHoldModal, setShowHoldModal] = useState(false);
  const [showDestroyModal, setShowDestroyModal] = useState(false);
  const [showRedactModal, setShowRedactModal] = useState(false);
  const [showVersionModal, setShowVersionModal] = useState(false);

  /* --- fetch capture metadata --- */
  useEffect(() => {
    const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';
    if (!accessToken) return;
    fetch(`${API_BASE}/api/evidence/${evidence.id}/capture-metadata`, {
      headers: { Authorization: `Bearer ${accessToken}` },
    })
      .then((res) => (res.ok ? res.json() : null))
      .then((json) => {
        if (json?.data) setCaptureMetadata(json.data);
      })
      .catch(() => {});
  }, [evidence.id, accessToken]);

  /* --- fetch custody on mount --- */
  const loadCustody = useCallback(async () => {
    if (custodyLoaded) return;
    try {
      const res = await fetch(`/api/evidence/${evidence.id}/custody`);
      if (res.ok) {
        const json = await res.json();
        setCustodyEntries(json.data || []);
      } else {
        setCustodyError('Failed to load custody log');
      }
    } catch {
      setCustodyError('Failed to load custody log');
    }
    setCustodyLoaded(true);
  }, [evidence.id, custodyLoaded]);

  const loadVersions = useCallback(async () => {
    if (versionsLoaded) return;
    try {
      const res = await fetch(`/api/evidence/${evidence.id}/versions`);
      if (res.ok) {
        const json = await res.json();
        setVersions(json.data || []);
      } else {
        setVersionsError('Failed to load version history');
      }
    } catch {
      setVersionsError('Failed to load version history');
    }
    setVersionsLoaded(true);
  }, [evidence.id, versionsLoaded]);

  const loadInvestigation = useCallback(async () => {
    if (investigationLoaded || !accessToken) return;
    const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';
    const headers = { Authorization: `Bearer ${accessToken}` };
    try {
      const [aRes, vRes] = await Promise.all([
        fetch(`${API_BASE}/api/evidence/${evidence.id}/assessments`, { headers }).then(r => r.ok ? r.json() : null),
        fetch(`${API_BASE}/api/evidence/${evidence.id}/verifications`, { headers }).then(r => r.ok ? r.json() : null),
      ]);
      if (Array.isArray(aRes)) setAssessments(aRes);
      else if (aRes?.data) setAssessments(aRes.data);
      if (Array.isArray(vRes)) setVerifications(vRes);
      else if (vRes?.data) setVerifications(vRes.data);
    } catch { /* ignore */ }
    setInvestigationLoaded(true);
  }, [evidence.id, accessToken, investigationLoaded]);

  const loadRedactionView = useCallback(async () => {
    if (redactionViewLoaded || !accessToken) return;
    const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';
    try {
      const res = await fetch(`${API_BASE}/api/evidence/${evidence.id}/redactions`, {
        headers: { Authorization: `Bearer ${accessToken}` },
      });
      if (res.ok) {
        const json = await res.json();
        setRedactionView(json.data || json);
      }
    } catch { /* ignore */ }
    setRedactionViewLoaded(true);
  }, [evidence.id, accessToken, redactionViewLoaded]);

  useEffect(() => {
    loadCustody();
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const handleTabChange = useCallback((tab: TabId) => {
    setActiveTab(tab);
    if (tab === 'custody') loadCustody();
    if (tab === 'versions') loadVersions();
    if (tab === 'verification') loadInvestigation();
    if (tab === 'redactions') loadRedactionView();
  }, [loadCustody, loadVersions, loadInvestigation, loadRedactionView]);

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

  /* --- derived values --- */
  const exif = evidence.metadata?.exif as Record<string, unknown> | undefined;
  const cm = captureMetadata;
  const custodyCount = custodyLoaded ? custodyEntries.length : 0;
  const versionsCount = versionsLoaded ? versions.length : 0;
  const redactionCount = redactionView
    ? (redactionView.drafts?.length || 0) + (redactionView.finalized?.length || 0)
    : 0;
  const verificationCount = assessments.length + verifications.length;

  /* --- BP phases --- */
  const bpPhases = useMemo(() => computeBPPhases(evidence, cm, assessments, verifications, custodyEntries), [evidence, cm, assessments, verifications, custodyEntries]);
  const bpDone = bpPhases.filter(p => p.complete).length;
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

  return (
    <div className="d-content">
      {/* ---- Draft picker dialog ---- */}
      {showDraftPicker && accessToken && (
        <DraftPicker
          evidenceId={evidence.id}
          accessToken={accessToken}
          onSelect={async (draft) => {
            setShowDraftPicker(false);
            setSelectedDraft(draft);
            try {
              const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';
              const res = await fetch(`${API_BASE}/api/evidence/${evidence.id}/page-count`, {
                headers: { Authorization: `Bearer ${accessToken}` },
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
          {evidence.tsa_timestamp && <span className="pl sealed">Sealed</span>}
          {!evidence.is_current && <span className="pl draft">Superseded</span>}
          {evidence.destroyed_at && <span className="pl broken">Destroyed</span>}
          <span className="tag" style={{ textTransform: 'capitalize' }}>{evidence.classification.replace('_', ' ')}</span>
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

      {/* ---- Berkeley Protocol phase tracker ---- */}
      <BPTracker phases={bpPhases} />

      {/* ---- Layout: main + sidebar ---- */}
      <div className="ev-layout">
        <div className="ev-main">
          {/* Preview */}
          <EvidencePreview evidence={evidence} waveformBars={waveformBars} />

          {/* Tags */}
          {evidence.tags.length > 0 && (
            <div className="ev-tags">
              {evidence.tags.map((t) => (
                <span key={t} className="tag">{t}</span>
              ))}
            </div>
          )}

          {/* Tabs */}
          <div className="ev-tabs">
            <a className={activeTab === 'custody' ? 'active' : ''} onClick={() => handleTabChange('custody')}>
              Custody log<span className="ct">{custodyCount || ''}</span>
            </a>
            <a className={activeTab === 'versions' ? 'active' : ''} onClick={() => handleTabChange('versions')}>
              Versions<span className="ct">{versionsCount || ''}</span>
            </a>
            <a className={activeTab === 'redactions' ? 'active' : ''} onClick={() => handleTabChange('redactions')}>
              Redactions<span className="ct">{redactionCount || ''}</span>
            </a>
            <a className={activeTab === 'verification' ? 'active' : ''} onClick={() => handleTabChange('verification')}>
              Verification<span className="ct">{verificationCount || ''}</span>
            </a>
            <a className={activeTab === 'manage' ? 'active' : ''} onClick={() => handleTabChange('manage')}>
              Manage
            </a>
          </div>

          {/* Tab content */}
          {activeTab === 'custody' && (
            <CustodyTabContent entries={custodyEntries} loaded={custodyLoaded} error={custodyError} />
          )}
          {activeTab === 'versions' && (
            <VersionsTabContent versions={versions} loaded={versionsLoaded} error={versionsError} locale={locale} />
          )}
          {activeTab === 'redactions' && (
            <RedactionsTabContent
              redactionView={redactionView}
              loaded={redactionViewLoaded}
              accessToken={accessToken}
              evidenceId={evidence.id}
              onResumeDraft={async (draft) => {
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
              }}
              onNewRedaction={() => setShowRedactModal(true)}
            />
          )}
          {activeTab === 'verification' && (
            <VerificationTabContent assessments={assessments} verifications={verifications} loaded={investigationLoaded} />
          )}
          {activeTab === 'manage' && (
            <ManageTabContent
              evidence={evidence}
              onShowHold={() => setShowHoldModal(true)}
              onShowDestroy={() => setShowDestroyModal(true)}
              onShowVersion={() => setShowVersionModal(true)}
              onShowRedact={() => setShowRedactModal(true)}
            />
          )}
        </div>

        {/* ---- Sidebar ---- */}
        <div className="ev-sidebar">
          <SidebarFile evidence={evidence} />
          <SidebarDates evidence={evidence} />
          <SidebarIntegrity evidence={evidence} />
          <SidebarProvenance cm={cm} />
          <SidebarExif evidence={evidence} exif={exif || {}} />
          <SidebarBerkeleyCompliance phases={bpPhases} done={bpDone} total={bpTotal} pct={bpPct} />
          <SidebarLinked evidence={evidence} />
        </div>
      </div>

      {/* ---- Modals ---- */}
      {showHoldModal && <LegalHoldModal evidenceNumber={evidence.evidence_number} caseReferenceCode={caseReferenceCode} onClose={() => setShowHoldModal(false)} />}
      {showDestroyModal && <DestroyModal evidence={evidence} onClose={() => setShowDestroyModal(false)} />}
      {showRedactModal && <RedactModal onClose={() => setShowRedactModal(false)} />}
      {showVersionModal && (
        <VersionUploadModal
          evidence={evidence}
          uploading={uploading}
          fileInputRef={fileInputRef}
          onClose={() => setShowVersionModal(false)}
        />
      )}
    </div>
  );
}

/* ================================================================
   Berkeley Protocol phase computation
   ================================================================ */

interface BPPhase {
  readonly num: string;
  readonly name: string;
  readonly complete: boolean;
  readonly partial?: boolean;
  readonly items: readonly string[];
  readonly missing: readonly string[];
  readonly action?: string;
  readonly link?: string;
}

function computeBPPhases(
  e: EvidenceItem,
  cm: CaptureMetadata | null,
  assessmentsArr: readonly EvidenceAssessment[],
  verificationsArr: readonly VerificationRecord[],
  custodyArr: readonly CustodyEntry[]
): readonly BPPhase[] {
  // Evidence exists → it was collected and preserved at minimum
  const hasEvidence = !!e.id;
  const hasCaptureMetadata = !!(cm?.capture_method && cm?.capture_timestamp && cm?.collector_display_name);
  const hasHash = !!e.sha256_hash;
  const hasTSA = !!e.tsa_timestamp;
  const hasCustody = custodyArr.length > 0;
  const hasVerifications = verificationsArr.length > 0;
  const isFullyVerified = hasVerifications && cm?.verification_status === 'verified';

  return [
    {
      num: 'Phase 1', name: 'Online inquiry',
      // If evidence was uploaded, inquiry happened (even if not formally logged)
      complete: hasEvidence,
      items: hasEvidence
        ? ['Search strategy documented', 'Tools & parameters logged', `Discovery timeline: ${formatDate(e.created_at).split(',')[0]}`]
        : ['No inquiry log yet'],
      missing: [],
      action: 'View inquiry log',
      link: '#',
    },
    {
      num: 'Phase 2', name: 'Assessment',
      complete: assessmentsArr.length > 0,
      items: assessmentsArr.length
        ? [`Relevance: ${assessmentsArr[0].relevance_score}/10`, `Reliability: ${assessmentsArr[0].reliability_score}/10`, `Source credibility: ${assessmentsArr[0].source_credibility}`, `Recommendation: ${assessmentsArr[0].recommendation}`]
        : ['No assessment yet'],
      missing: assessmentsArr.length ? [] : ['Relevance score'],
      action: 'Create assessment',
      link: '#',
    },
    {
      num: 'Phase 3', name: 'Collection',
      complete: hasEvidence,
      items: hasCaptureMetadata && cm
        ? [`Method: ${(cm.capture_method || '').replace(/_/g, ' ')}`, `Captured: ${cm.capture_timestamp ? new Date(cm.capture_timestamp).toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' }) : '\u2014'}`, `Collector: ${cm.collector_display_name || '\u2014'}`]
        : [`Uploaded: ${formatDate(e.created_at).split(',')[0]}`, `File: ${e.filename}`, `Size: ${formatBytes(e.size_bytes)}`],
      missing: hasCaptureMetadata ? [] : ['Add capture metadata'],
      action: 'Edit capture metadata',
      link: '#',
    },
    {
      num: 'Phase 4', name: 'Preservation',
      complete: hasHash,
      items: [
        hasHash ? 'SHA-256 sealed' : 'SHA-256 pending',
        hasTSA ? 'RFC 3161 timestamped' : 'TSA pending',
        `Custody chain: ${custodyArr.length} events`,
      ],
      missing: hasHash ? [] : ['Awaiting seal'],
      action: 'View custody log',
      link: '#',
    },
    {
      num: 'Phase 5', name: 'Verification',
      complete: isFullyVerified,
      partial: hasVerifications && !isFullyVerified,
      items: hasVerifications
        ? verificationsArr.map(v => `${v.verification_type.replace(/_/g, ' ')}: ${v.finding.replace(/_/g, ' ')}`)
        : ['No verifications yet'],
      missing: isFullyVerified ? [] : ['Verification pending'],
      action: 'Add verification',
      link: '#',
    },
    {
      num: 'Phase 6', name: 'Analysis',
      complete: false,
      partial: hasVerifications, // If verifications exist, analysis is in progress
      items: hasVerifications
        ? [`${verificationsArr.length} verification${verificationsArr.length !== 1 ? 's' : ''} linked`]
        : ['No analysis note yet'],
      missing: ['Final analytical note pending'],
      action: 'Create analysis note',
      link: '#',
    },
  ];
}

/* ================================================================
   BP Tracker
   ================================================================ */

function BPTracker({ phases }: { phases: readonly BPPhase[] }) {
  return (
    <div className="bp-tracker">
      {phases.map((p) => {
        const st = p.complete ? 'done' : (p.partial ? 'part' : 'none');
        const stLabel = p.complete ? 'Complete' : (p.partial ? 'In progress' : 'Not started');
        const stClass = p.complete ? 'complete' : (p.partial ? 'partial' : 'missing');
        const showAction = !p.complete && p.missing.length > 0 && p.action;
        return (
          <div key={p.num} className={`bp-phase ${st}`}>
            <div className="bp-num">{p.num}</div>
            <div className="bp-name">{p.name}</div>
            <div className={`bp-status ${stClass}`}><span className="dot" />{stLabel}</div>
            <div className="bp-detail">
              {p.items.slice(0, 3).map((item, i) => <div key={i}>{item}</div>)}
              {p.missing.length > 0 && (
                <div style={{ color: 'var(--accent)', marginTop: 3 }}>{p.missing[0]}</div>
              )}
            </div>
            {showAction && (
              <a
                href={p.link || '#'}
                style={{
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: 4,
                  marginTop: 8,
                  padding: '4px 10px',
                  borderRadius: 6,
                  background: p.partial ? 'rgba(184,66,28,.08)' : 'var(--bg-2)',
                  fontSize: 11,
                  color: p.partial ? 'var(--accent)' : 'var(--muted)',
                  fontFamily: 'Fraunces, serif',
                  fontStyle: 'italic',
                  transition: 'background .15s',
                }}
              >
                {p.action} &rarr;
              </a>
            )}
            <div className="bp-bar"><div className="bp-fill" /></div>
          </div>
        );
      })}
    </div>
  );
}

/* ================================================================
   Evidence Preview
   ================================================================ */

function EvidencePreview({ evidence, waveformBars }: { evidence: EvidenceItem; waveformBars: number[] }) {
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
   Tab: Custody
   ================================================================ */

function CustodyTabContent({ entries, loaded, error }: { entries: readonly CustodyEntry[]; loaded: boolean; error: string | null }) {
  if (!loaded) return <div className="ph" style={{ margin: '14px 0' }}>Loading&hellip;</div>;
  if (error) return <div className="ph" style={{ margin: '14px 0', color: 'var(--accent)' }}>{error}</div>;
  if (entries.length === 0) return <div className="ph" style={{ margin: '14px 0' }}>No custody entries</div>;

  return (
    <div className="cust-list" style={{ paddingTop: 14 }}>
      {entries.map((c) => {
        const typeClass = custodyTypeClass(c.action);
        const detail = formatCustodyDetail(c.detail);
        return (
          <div key={c.id} className={`cust-item ${typeClass}`}>
            <div>
              <div className="act"><strong>{c.action}</strong></div>
              <div className="detail">{detail} &middot; {c.actor_user_id.slice(0, 8)}&hellip;</div>
            </div>
            <div className="ts">
              {new Date(c.timestamp).toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' })}
              {', '}
              {new Date(c.timestamp).toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' })}
            </div>
          </div>
        );
      })}
    </div>
  );
}

/* ================================================================
   Tab: Versions
   ================================================================ */

function VersionsTabContent({ versions: versionsList, loaded, error, locale }: { versions: readonly VersionEntry[]; loaded: boolean; error: string | null; locale: string }) {
  const router = useRouter();

  if (!loaded) return <div className="ph" style={{ margin: '14px 0' }}>Loading&hellip;</div>;
  if (error) return <div className="ph" style={{ margin: '14px 0', color: 'var(--accent)' }}>{error}</div>;
  if (versionsList.length === 0) return <div className="ph" style={{ margin: '14px 0' }}>No previous versions</div>;

  return (
    <div style={{ paddingTop: 14 }}>
      {versionsList.map((v) => (
        <div
          key={v.id}
          className={`ver-row${v.is_current ? ' current' : ''}`}
          onClick={() => router.push(`/${locale}/evidence/${v.id}`)}
        >
          <div className="vn-wrap">v{v.version}</div>
          <div>
            <div style={{ fontSize: 14, fontWeight: 500, color: 'var(--ink)', display: 'flex', alignItems: 'center', gap: 8 }}>
              {v.filename}
              {v.is_current
                ? <span className="pl sealed" style={{ fontSize: 10, padding: '2px 8px' }}>Current</span>
                : <span className="pl draft" style={{ fontSize: 10, padding: '2px 8px' }}>Superseded</span>
              }
            </div>
            <div style={{ fontSize: 12, color: 'var(--muted)', marginTop: 4, lineHeight: 1.5 }}>
              {v.note || ''}{v.note ? ' \u00b7 ' : ''}
              <span style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: 11 }}>{v.sha256_hash?.slice(0, 8)}&hellip;</span>
            </div>
          </div>
          <div style={{ textAlign: 'right' }}>
            <div style={{ fontSize: 12, color: 'var(--ink-2)' }}>
              {new Date(v.created_at).toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' })}
            </div>
            <div style={{ fontSize: 11, color: 'var(--muted-2)', marginTop: 2 }}>{v.uploaded_by_name || v.uploaded_by.slice(0, 8)}</div>
          </div>
        </div>
      ))}
    </div>
  );
}

/* ================================================================
   Tab: Redactions
   ================================================================ */

function RedactionsTabContent({
  redactionView,
  loaded,
  accessToken: _at,
  evidenceId: _eid,
  onResumeDraft,
  onNewRedaction,
}: {
  redactionView: RedactionManagementView | null;
  loaded: boolean;
  accessToken?: string;
  evidenceId: string;
  onResumeDraft: (draft: RedactionDraft) => void;
  onNewRedaction: () => void;
}) {
  if (!loaded) return <div className="ph" style={{ margin: '14px 0' }}>Loading&hellip;</div>;

  const drafts = redactionView?.drafts || [];
  const finalized = redactionView?.finalized || [];

  return (
    <div style={{ paddingTop: 14 }}>
      {drafts.length > 0 && (
        <>
          <div className="sb-label" style={{ marginBottom: 10 }}>Drafts in progress</div>
          {drafts.map((d) => (
            <div key={d.id} className="red-card">
              <div className="red-head">
                <div>
                  <span className="red-name">{d.name}</span>
                  {' '}<span className="tag" style={{ marginLeft: 6 }}>{PURPOSE_LABELS[d.purpose] || d.purpose}</span>
                </div>
                <div style={{ display: 'flex', gap: 6 }}>
                  <a className="linkarrow" style={{ fontSize: 12, cursor: 'pointer' }} onClick={() => onResumeDraft(d)}>Resume</a>
                  <a style={{ fontSize: 12, color: '#b35c5c', cursor: 'pointer' }}>Discard</a>
                </div>
              </div>
              <div className="red-meta">
                {d.area_count} area{d.area_count !== 1 ? 's' : ''} &middot; Last saved {formatDate(d.last_saved_at)} &middot; {d.created_by.slice(0, 8)}
              </div>
            </div>
          ))}
        </>
      )}
      {finalized.length > 0 && (
        <>
          <div className="sb-label" style={{ marginTop: 18, marginBottom: 10 }}>Finalized</div>
          {finalized.map((f) => (
            <div key={f.id} className="red-card">
              <div className="red-head">
                <div>
                  <span className="red-name">{f.name}</span>
                  {' '}<span className="tag" style={{ marginLeft: 6 }}>{PURPOSE_LABELS[f.purpose] || f.purpose}</span>
                  {' '}<span className="pl sealed" style={{ fontSize: 10, padding: '2px 7px' }}>Applied</span>
                </div>
                <div style={{ display: 'flex', gap: 6 }}>
                  <a className="linkarrow" style={{ fontSize: 12, cursor: 'pointer' }}>View</a>
                  <a className="linkarrow" style={{ fontSize: 12, cursor: 'pointer' }}>Download</a>
                </div>
              </div>
              <div className="red-meta">
                {f.area_count} area{f.area_count !== 1 ? 's' : ''} &middot; Finalized {formatDate(f.finalized_at)} &middot; {f.author.slice(0, 8)} &middot;{' '}
                <span style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: 11 }}>{f.evidence_number}</span>
              </div>
            </div>
          ))}
        </>
      )}
      <button type="button" className="btn ghost sm" style={{ marginTop: 14 }} onClick={onNewRedaction}>+ New redacted version</button>
    </div>
  );
}

/* ================================================================
   Tab: Verification
   ================================================================ */

function VerificationTabContent({
  assessments: aList,
  verifications: vList,
  loaded,
}: {
  assessments: readonly EvidenceAssessment[];
  verifications: readonly VerificationRecord[];
  loaded: boolean;
}) {
  if (!loaded) return <div className="ph" style={{ margin: '14px 0' }}>Loading&hellip;</div>;

  return (
    <div style={{ paddingTop: 14 }}>
      {aList.length > 0 && (
        <>
          <div className="sb-label" style={{ marginBottom: 10 }}>Assessment</div>
          {aList.map((a) => (
            <div key={a.id} className="inv-card">
              <div className="inv-head">
                <span style={{ fontWeight: 500 }}>Evidence Assessment</span>
                <span style={{ fontSize: 12, color: 'var(--muted)' }}>{new Date(a.created_at).toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' })}</span>
              </div>
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 12, marginTop: 8 }}>
                <div>
                  <div style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: 10, color: 'var(--muted)', textTransform: 'uppercase' as const, letterSpacing: '.06em' }}>Relevance</div>
                  <div style={{ fontFamily: 'Fraunces, serif', fontSize: 24, color: 'var(--ink)' }}>{a.relevance_score}<span style={{ fontSize: 14, color: 'var(--muted)' }}>/10</span></div>
                </div>
                <div>
                  <div style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: 10, color: 'var(--muted)', textTransform: 'uppercase' as const, letterSpacing: '.06em' }}>Reliability</div>
                  <div style={{ fontFamily: 'Fraunces, serif', fontSize: 24, color: 'var(--ink)' }}>{a.reliability_score}<span style={{ fontSize: 14, color: 'var(--muted)' }}>/10</span></div>
                </div>
                <div>
                  <div style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: 10, color: 'var(--muted)', textTransform: 'uppercase' as const, letterSpacing: '.06em' }}>Credibility</div>
                  <div style={{ fontSize: 13, color: 'var(--ink)', marginTop: 6 }}><span className="pl sealed">{a.source_credibility}</span></div>
                </div>
                <div>
                  <div style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: 10, color: 'var(--muted)', textTransform: 'uppercase' as const, letterSpacing: '.06em' }}>Recommendation</div>
                  <div style={{ fontSize: 13, color: 'var(--ink)', marginTop: 6 }}><span className="pl live">{a.recommendation}</span></div>
                </div>
              </div>
              <div style={{ fontSize: 12, color: 'var(--muted)', marginTop: 8 }}>By {a.assessed_by?.slice(0, 8) || 'Unknown'}</div>
            </div>
          ))}
        </>
      )}
      {vList.length > 0 && (
        <>
          <div className="sb-label" style={{ marginTop: 18, marginBottom: 10 }}>Verification records</div>
          {vList.map((v) => {
            const findingCls = v.finding === 'authentic' ? 'sealed' : v.finding === 'likely_authentic' ? 'live' : 'draft';
            return (
              <div key={v.id} className="inv-card">
                <div className="inv-head">
                  <span style={{ fontWeight: 500 }}>{v.verification_type.replace(/_/g, ' ')}</span>
                  <span className={`pl ${findingCls}`}>{v.finding.replace(/_/g, ' ')}</span>
                </div>
                {v.methodology && <div style={{ fontSize: 13, color: 'var(--ink-2)', lineHeight: 1.55, marginTop: 6 }}>{v.methodology}</div>}
                <div style={{ display: 'flex', gap: 14, marginTop: 8, fontSize: 12, color: 'var(--muted)' }}>
                  <span>Confidence: <strong style={{ color: 'var(--ink)' }}>{v.confidence_level}</strong></span>
                  <span>By {v.verified_by?.slice(0, 8) || 'Unknown'} &middot; {new Date(v.created_at).toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' })}</span>
                </div>
              </div>
            );
          })}
        </>
      )}
      <div style={{ display: 'flex', gap: 8, marginTop: 14 }}>
        <button type="button" className="btn ghost sm">+ New assessment</button>
        <button type="button" className="btn ghost sm">+ New verification</button>
      </div>
    </div>
  );
}

/* ================================================================
   Tab: Manage
   ================================================================ */

function ManageTabContent({
  evidence,
  onShowHold,
  onShowDestroy,
  onShowVersion,
  onShowRedact,
}: {
  evidence: EvidenceItem;
  onShowHold: () => void;
  onShowDestroy: () => void;
  onShowVersion: () => void;
  onShowRedact: () => void;
}) {
  const classificationDescriptions: Record<string, string> = {
    public: 'Accessible to all case members and may be shared publicly.',
    restricted: 'Accessible to assigned team members only. Default for most evidence.',
    confidential: 'Restricted to lead investigators and admins. Every access is logged.',
    ex_parte: 'Visible only to one party (prosecution or defence). Requires side designation.',
  };

  return (
    <div style={{ paddingTop: 18, display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 18 }}>
      {/* Classification card */}
      <div className="inv-card" style={{ gridColumn: 'span 2' }}>
        <div className="inv-head">
          <span style={{ fontWeight: 500, fontSize: 15 }}>Classification</span>
          <span style={{ fontSize: 12, color: 'var(--muted)' }}>Controls who can access this evidence</span>
        </div>
        <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap', marginTop: 10 }}>
          {(['public', 'restricted', 'confidential', 'ex_parte'] as const).map((c) => {
            const active = c === evidence.classification;
            const colors: Record<string, string> = { public: 'var(--ok)', restricted: 'var(--accent)', confidential: '#b35c5c', ex_parte: '#5b4a6b' };
            const bgs: Record<string, string> = { public: 'rgba(74,107,58,.1)', restricted: 'rgba(184,66,28,.1)', confidential: 'rgba(179,92,92,.1)', ex_parte: 'rgba(91,74,107,.1)' };
            const labels: Record<string, string> = { public: 'Public', restricted: 'Restricted', confidential: 'Confidential', ex_parte: 'Ex Parte' };
            return (
              <span key={c} style={{
                padding: '8px 16px', borderRadius: 999, fontSize: 13, fontWeight: 500, cursor: 'pointer',
                border: `2px solid ${active ? colors[c] : 'var(--line-2)'}`,
                background: active ? bgs[c] : 'transparent',
                color: active ? colors[c] : 'var(--muted)',
                transition: 'all .15s',
              }}>
                {labels[c]}
              </span>
            );
          })}
        </div>
        <div style={{ fontSize: 12.5, color: 'var(--muted)', marginTop: 12, lineHeight: 1.6 }}>
          {classificationDescriptions[evidence.classification] || ''}
        </div>
      </div>

      {/* Legal hold card */}
      <div className="inv-card">
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 10 }}>
          <span style={{ fontWeight: 500, fontSize: 15 }}>Legal hold</span>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: 14, border: '1px solid var(--line)', borderRadius: 10, background: 'var(--bg-2)', marginBottom: 10 }}>
          <span style={{ width: 10, height: 10, borderRadius: '50%', background: 'var(--ok)' }} />
          <div>
            <div style={{ fontSize: 13, fontWeight: 500, color: 'var(--ink)' }}>Not active</div>
            <div style={{ fontSize: 11.5, color: 'var(--muted)', marginTop: 1 }}>Evidence can be modified and versioned</div>
          </div>
        </div>
        <button type="button" className="btn ghost sm" style={{ width: '100%', justifyContent: 'center' }} onClick={onShowHold}>Place legal hold</button>
        <div style={{ fontSize: 11.5, color: 'var(--muted)', marginTop: 8, lineHeight: 1.55 }}>
          Freezes the evidence &mdash; no modifications, deletions, or new versions. All case members are notified.
        </div>
      </div>

      {/* Versioning card */}
      <div className="inv-card">
        <div style={{ fontWeight: 500, fontSize: 15, marginBottom: 10 }}>Versioning</div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          <button type="button" className="btn ghost sm" style={{ width: '100%', justifyContent: 'center' }} onClick={onShowVersion}>Upload new version</button>
          <button type="button" className="btn ghost sm" style={{ width: '100%', justifyContent: 'center' }} onClick={onShowRedact}>Create redacted version</button>
        </div>
        <div style={{ fontSize: 11.5, color: 'var(--muted)', marginTop: 8, lineHeight: 1.55 }}>
          Current: v{evidence.version}. Previous versions are preserved and accessible in the Versions tab.
        </div>
      </div>

      {/* Integrity card */}
      <div className="inv-card" style={{ gridColumn: 'span 2' }}>
        <div style={{ fontWeight: 500, fontSize: 15, marginBottom: 12 }}>Integrity actions</div>
        <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap' }}>
          <button type="button" className="btn ghost sm">Re-verify hash integrity</button>
          <button type="button" className="btn ghost sm">Request new RFC 3161 timestamp</button>
          <button type="button" className="btn ghost sm">Export custody chain (PDF)</button>
          <a className="btn ghost sm" href={`/api/evidence/${evidence.id}/download`} download style={{ textDecoration: 'none' }}>Download original</a>
        </div>
      </div>

      {/* Retention card */}
      <div className="inv-card" style={{ gridColumn: 'span 2' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
          <span style={{ fontWeight: 500, fontSize: 15 }}>Retention &amp; destruction</span>
          <span className="pl sealed" style={{ fontSize: 10, padding: '2px 8px' }}>Active</span>
        </div>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 14, marginBottom: 14 }}>
          <div style={{ padding: 12, border: '1px solid var(--line)', borderRadius: 8, background: 'var(--bg-2)' }}>
            <div style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: 9, color: 'var(--muted-2)', textTransform: 'uppercase' as const, letterSpacing: '.08em', marginBottom: 4 }}>Policy</div>
            <div style={{ fontSize: 13, color: 'var(--ink)' }}>Case closed + 50 yr</div>
          </div>
          <div style={{ padding: 12, border: '1px solid var(--line)', borderRadius: 8, background: 'var(--bg-2)' }}>
            <div style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: 9, color: 'var(--muted-2)', textTransform: 'uppercase' as const, letterSpacing: '.08em', marginBottom: 4 }}>Review date</div>
            <div style={{ fontSize: 13, color: 'var(--ink)' }}>30 Jun 2076</div>
          </div>
          <div style={{ padding: 12, border: '1px solid var(--line)', borderRadius: 8, background: 'var(--bg-2)' }}>
            <div style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: 9, color: 'var(--muted-2)', textTransform: 'uppercase' as const, letterSpacing: '.08em', marginBottom: 4 }}>Status</div>
            <div style={{ fontSize: 13, color: 'var(--ok)' }}>Not destroyed</div>
          </div>
        </div>
        <button type="button" className="btn ghost sm" style={{ color: '#b35c5c', borderColor: 'rgba(179,92,92,.3)' }} onClick={onShowDestroy}>Initiate destruction</button>
        <div style={{ fontSize: 11.5, color: 'var(--muted)', marginTop: 8, lineHeight: 1.55 }}>
          Destruction removes file bytes permanently. Hash, metadata, and full custody chain are preserved. Requires written authority &mdash; a 4-step sealed process.
        </div>
      </div>
    </div>
  );
}

/* ================================================================
   Sidebar blocks
   ================================================================ */

function SbRow({ k, v, cls }: { k: string; v: React.ReactNode; cls?: string }) {
  return (
    <div className="sb-row">
      <span className="k">{k}</span>
      <span className={`v${cls ? ` ${cls}` : ''}`}>{v}</span>
    </div>
  );
}

function SidebarFile({ evidence }: { evidence: EvidenceItem }) {
  return (
    <div className="sb">
      <div className="sb-label">File</div>
      <SbRow k="Name" v={evidence.filename} />
      <SbRow k="Original" v={evidence.original_name || '\u2014'} />
      <SbRow k="Size" v={formatBytes(evidence.size_bytes)} />
      <SbRow k="Type" v={evidence.mime_type} cls="mono" />
      <SbRow k="Version" v={`v${evidence.version}`} cls="mono" />
      <SbRow k="Classification" v={evidence.classification.replace('_', ' ')} />
    </div>
  );
}

function SidebarDates({ evidence }: { evidence: EvidenceItem }) {
  return (
    <div className="sb">
      <div className="sb-label">Dates</div>
      <SbRow k="Uploaded" v={formatDate(evidence.created_at)} />
      <SbRow k="By" v={evidence.uploaded_by_name || evidence.uploaded_by} />
      <SbRow k="Source" v={evidence.source || '\u2014'} />
      <SbRow k="Source date" v={formatDate(evidence.source_date)} />
    </div>
  );
}

function SidebarIntegrity({ evidence }: { evidence: EvidenceItem }) {
  return (
    <div className="sb">
      <div className="sb-label">Integrity</div>
      <div style={{ marginBottom: 10 }}>
        <div style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: 9, color: 'var(--muted-2)', letterSpacing: '.08em', textTransform: 'uppercase' as const, marginBottom: 4 }}>SHA-256</div>
        <div className="hash-display">{evidence.sha256_hash}</div>
      </div>
      <SbRow k="TSA" v={evidence.tsa_timestamp ? <span className="pl sealed" style={{ fontSize: 10, padding: '2px 7px' }}>Verified</span> : <span className="pl draft" style={{ fontSize: 10, padding: '2px 7px' }}>Pending</span>} />
      <SbRow k="Authority" v={evidence.tsa_name || '\u2014'} cls="mono" />
      <SbRow k="Stamped" v={formatDate(evidence.tsa_timestamp)} />
    </div>
  );
}

function SidebarProvenance({ cm }: { cm: CaptureMetadata | null }) {
  const vColor = cm?.verification_status === 'verified' ? 'var(--ok)' : cm?.verification_status === 'disputed' ? 'var(--accent)' : 'var(--muted)';
  const methodLabel = cm ? (CAPTURE_METHODS.find(m => m.value === cm.capture_method)?.label || cm.capture_method.replace(/_/g, ' ')) : '\u2014';
  const platformLabel = cm ? (PLATFORMS.find(p => p.value === cm.platform)?.label || cm.platform || '\u2014') : '\u2014';
  const verificationLabel = cm?.verification_status ? (VERIFICATION_STATUSES.find(v => v.value === cm.verification_status)?.label || cm.verification_status.replace(/_/g, ' ')) : '\u2014';
  const availabilityLabel = cm?.availability_status
    ? AVAILABILITY_STATUSES.find(a => a.value === cm.availability_status)?.label || cm.availability_status
    : null;

  return (
    <div className="sb">
      <div className="sb-label">Provenance <span className="bp-tag">Berkeley</span></div>
      <SbRow k="Platform" v={platformLabel} />
      <SbRow k="Method" v={methodLabel} />
      <SbRow k="Captured" v={formatDate(cm?.capture_timestamp)} />
      <SbRow k="Published" v={formatDate(cm?.publication_timestamp)} />
      <SbRow k="Collector" v={cm?.collector_display_name || '\u2014'} />
      <SbRow k="Language" v={cm?.content_language || '\u2014'} />
      <SbRow k="Location" v={cm?.geo_place_name || '\u2014'} />
      {cm?.geo_latitude != null && cm?.geo_longitude != null && (
        <SbRow k="Coordinates" v={`${cm.geo_latitude.toFixed(4)}, ${cm.geo_longitude.toFixed(4)}`} cls="mono" />
      )}
      <SbRow k="Geo source" v={cm?.geo_source || '\u2014'} />
      <SbRow k="Availability" v={availabilityLabel ? <span className="pl sealed" style={{ fontSize: 10, padding: '2px 6px' }}>{availabilityLabel}</span> : '\u2014'} />
      <SbRow k="Tool" v={cm?.capture_tool_name ? `${cm.capture_tool_name} ${cm.capture_tool_version || ''}` : '\u2014'} cls="mono" />
      <div style={{ marginTop: 8 }}>
        <div style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: 10, color: 'var(--muted)', letterSpacing: '.06em', textTransform: 'uppercase' as const, marginBottom: 4 }}>Verification</div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 4 }}>
          <span style={{ width: 6, height: 6, borderRadius: '50%', background: vColor }} />
          <span style={{ fontSize: 13, color: vColor, fontWeight: 500 }}>{verificationLabel}</span>
        </div>
        {cm?.verification_notes && (
          <div style={{ fontSize: 12, color: 'var(--muted)', lineHeight: 1.5 }}>{cm.verification_notes}</div>
        )}
      </div>
    </div>
  );
}

function SidebarExif({ evidence, exif }: { evidence: EvidenceItem; exif: Record<string, unknown> }) {
  const meta = evidence.metadata as Record<string, unknown> | undefined;
  return (
    <div className="sb">
      <div className="sb-label">EXIF</div>
      <SbRow k="Camera" v={exif?.camera_make ? `${String(exif.camera_make)} ${String(exif.camera_model || '')}` : '\u2014'} />
      <SbRow k="Capture date" v={exif?.capture_date ? String(exif.capture_date) : '\u2014'} />
      <SbRow k="Focal length" v={exif?.focal_length ? String(exif.focal_length) : '\u2014'} />
      <SbRow k="GPS" v={exif?.gps_latitude != null ? `${String(exif.gps_latitude)}, ${String(exif.gps_longitude)}` : '\u2014'} cls="mono" />
      <SbRow k="Resolution" v={meta?.resolution ? String(meta.resolution) : '\u2014'} cls="mono" />
      <SbRow k="Codec" v={meta?.codec ? String(meta.codec) : '\u2014'} cls="mono" />
      <SbRow k="FPS" v={meta?.fps ? String(meta.fps) : '\u2014'} cls="mono" />
    </div>
  );
}

function SidebarBerkeleyCompliance({ phases, done, total, pct }: { phases: readonly BPPhase[]; done: number; total: number; pct: number }) {
  return (
    <div className="sb">
      <div className="sb-label">Berkeley Protocol <span className="bp-tag">Compliance</span></div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 12 }}>
        <div style={{ fontFamily: 'Fraunces, serif', fontSize: 32, letterSpacing: '-.02em', color: pct >= 80 ? 'var(--ok)' : pct >= 50 ? 'var(--accent)' : 'var(--muted)' }}>
          {done}<span style={{ fontSize: 16, color: 'var(--muted)' }}>/{total}</span>
        </div>
        <div style={{ flex: 1 }}>
          <div style={{ fontSize: 12, color: 'var(--muted)', marginBottom: 4 }}>Phases complete</div>
          <div style={{ height: 6, background: 'var(--bg-2)', borderRadius: 3, overflow: 'hidden' }}>
            <div style={{ height: '100%', width: `${pct}%`, background: pct >= 80 ? 'var(--ok)' : 'var(--accent)', borderRadius: 3 }} />
          </div>
        </div>
      </div>
      {phases.map((p) => {
        const icon = p.complete
          ? <span style={{ color: 'var(--ok)' }}>{'\u2713'}</span>
          : (p.partial ? <span style={{ color: 'var(--accent)' }}>{'\u25D4'}</span> : <span style={{ color: 'var(--muted-2)' }}>{'\u25CB'}</span>);
        return (
          <div key={p.num} style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '4px 0', fontSize: 12 }}>
            {icon}
            <span style={{ color: 'var(--ink-2)' }}>{p.num}: {p.name}</span>
          </div>
        );
      })}
    </div>
  );
}

function SidebarLinked({ evidence }: { evidence: EvidenceItem }) {
  // Build linked items from evidence relationships
  const links: { label: string; href: string }[] = [];

  // Add corroboration links if available
  const corrobs = (evidence as unknown as Record<string, unknown>).corroborations as Array<{ id: string; claim_reference: string }> | undefined;
  if (corrobs?.length) {
    corrobs.forEach(c => links.push({ label: `Corroboration ${c.claim_reference}`, href: `/en/corroborations/${c.id}` }));
  }

  // Add witness links if available
  const witnessRefs = (evidence as unknown as Record<string, unknown>).witnesses as Array<{ id: string; pseudonym: string }> | undefined;
  if (witnessRefs?.length) {
    witnessRefs.forEach(w => links.push({ label: `Witness ${w.pseudonym}`, href: `/en/witnesses/${w.id}` }));
  }

  // Add disclosure links if available
  const discRefs = (evidence as unknown as Record<string, unknown>).disclosures as Array<{ id: string; reference: string }> | undefined;
  if (discRefs?.length) {
    discRefs.forEach(d => links.push({ label: `Disclosure ${d.reference}`, href: `/en/disclosures/${d.id}` }));
  }

  // Fallback if no linked data is available from the API
  if (links.length === 0) {
    links.push(
      { label: 'Corroborations', href: '#' },
      { label: 'Witnesses', href: '#' },
      { label: 'Disclosures', href: '#' },
    );
  }

  return (
    <div className="sb">
      <div className="sb-label">Linked</div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 7, fontSize: 13 }}>
        {links.map((link, i) => (
          <a key={i} href={link.href} style={{ color: 'var(--accent)', cursor: 'pointer' }}>{link.label}</a>
        ))}
      </div>
    </div>
  );
}

/* ================================================================
   Modals
   ================================================================ */

function LegalHoldModal({ evidenceNumber, caseReferenceCode, onClose }: { evidenceNumber: string; caseReferenceCode: string; onClose: () => void }) {
  return (
    <div
      style={{ display: 'flex', position: 'fixed', inset: 0, zIndex: 200, background: 'rgba(20,17,12,.5)', alignItems: 'center', justifyContent: 'center' }}
      onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}
    >
      <div style={{ background: 'var(--paper)', borderRadius: 'var(--radius)', maxWidth: 480, width: '100%', padding: 32, boxShadow: 'var(--shadow-lg)', margin: 20 }}>
        <h3 style={{ fontFamily: 'Fraunces, serif', fontSize: 22, marginBottom: 6 }}>Place <em style={{ color: 'var(--accent)' }}>legal hold</em></h3>
        <p style={{ fontSize: 13, color: 'var(--muted)', marginBottom: 18, lineHeight: 1.6 }}>
          This will freeze {evidenceNumber}. No modifications, deletions, or new versions will be permitted. All case members on {caseReferenceCode} will be notified.
        </p>
        <div style={{ background: 'var(--bg-2)', borderRadius: 10, padding: 14, marginBottom: 18, display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12, fontSize: 13 }}>
          <div>
            <span style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: 10, color: 'var(--muted)', textTransform: 'uppercase' as const, letterSpacing: '.06em', display: 'block', marginBottom: 2 }}>Current</span>
            <span style={{ color: 'var(--ink)' }}>No hold</span>
          </div>
          <div>
            <span style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: 10, color: 'var(--muted)', textTransform: 'uppercase' as const, letterSpacing: '.06em', display: 'block', marginBottom: 2 }}>Will change to</span>
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
      </div>
    </div>
  );
}

function DestroyModal({ evidence, onClose }: { evidence: EvidenceItem; onClose: () => void }) {
  return (
    <div
      style={{ display: 'flex', position: 'fixed', inset: 0, zIndex: 200, background: 'rgba(20,17,12,.5)', alignItems: 'center', justifyContent: 'center' }}
      onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}
    >
      <div style={{ background: 'var(--paper)', borderRadius: 'var(--radius)', maxWidth: 520, width: '100%', padding: 32, boxShadow: 'var(--shadow-lg)', margin: 20 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 18 }}>
          <h3 style={{ fontFamily: 'Fraunces, serif', fontSize: 22, color: '#b35c5c' }}>Destroy evidence</h3>
          <span style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: 10, color: 'var(--muted)', letterSpacing: '.06em' }}>Step 1 of 4</span>
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
            <span style={{ color: 'var(--muted)' }}>Evidence</span><span style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: 12 }}>{evidence.evidence_number}</span>
            <span style={{ color: 'var(--muted)' }}>File</span><span>{evidence.filename}</span>
            <span style={{ color: 'var(--muted)' }}>Hash</span><span style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: 11, wordBreak: 'break-all' }}>{evidence.sha256_hash.slice(0, 12)}&hellip;{evidence.sha256_hash.slice(-4)}</span>
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
      </div>
    </div>
  );
}

function RedactModal({ onClose }: { onClose: () => void }) {
  return (
    <div
      style={{ display: 'flex', position: 'fixed', inset: 0, zIndex: 200, background: 'rgba(20,17,12,.5)', alignItems: 'center', justifyContent: 'center' }}
      onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}
    >
      <div style={{ background: 'var(--paper)', borderRadius: 'var(--radius)', maxWidth: 480, width: '100%', padding: 32, boxShadow: 'var(--shadow-lg)', margin: 20 }}>
        <h3 style={{ fontFamily: 'Fraunces, serif', fontSize: 22, marginBottom: 6 }}>New <em style={{ color: 'var(--accent)' }}>redacted version</em></h3>
        <p style={{ fontSize: 13, color: 'var(--muted)', marginBottom: 18, lineHeight: 1.6 }}>Create a derivative with redacted areas for disclosure. The original is never modified.</p>
        <div style={{ marginBottom: 14 }}>
          <label style={{ fontSize: 13, fontWeight: 500, display: 'block', marginBottom: 6 }}>Name</label>
          <input type="text" style={{ width: '100%', padding: '10px 14px', border: '1px solid var(--line-2)', borderRadius: 'var(--radius-sm)', font: 'inherit', fontSize: 13, outline: 'none', background: 'var(--paper)' }} placeholder="e.g. Defence disclosure \u2014 faces redacted" />
        </div>
        <div style={{ marginBottom: 18 }}>
          <label style={{ fontSize: 13, fontWeight: 500, display: 'block', marginBottom: 6 }}>Purpose</label>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
            <span className="tag" style={{ cursor: 'pointer', padding: '6px 12px', border: '1.5px solid var(--accent)', background: 'rgba(184,66,28,.08)', color: 'var(--accent)' }}>Defence</span>
            <span className="tag" style={{ cursor: 'pointer', padding: '6px 12px' }}>Prosecution</span>
            <span className="tag" style={{ cursor: 'pointer', padding: '6px 12px' }}>Public</span>
            <span className="tag" style={{ cursor: 'pointer', padding: '6px 12px' }}>Court</span>
            <span className="tag" style={{ cursor: 'pointer', padding: '6px 12px' }}>Witness</span>
            <span className="tag" style={{ cursor: 'pointer', padding: '6px 12px' }}>Internal</span>
          </div>
        </div>
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 10 }}>
          <button type="button" className="btn ghost sm" onClick={onClose}>Cancel</button>
          <button type="button" className="btn sm" onClick={onClose}>Create draft &amp; open editor &rarr;</button>
        </div>
      </div>
    </div>
  );
}

function VersionUploadModal({ evidence, uploading, fileInputRef, onClose }: {
  evidence: EvidenceItem;
  uploading: boolean;
  fileInputRef: React.RefObject<HTMLInputElement | null>;
  onClose: () => void;
}) {
  return (
    <div
      style={{ display: 'flex', position: 'fixed', inset: 0, zIndex: 200, background: 'rgba(20,17,12,.5)', alignItems: 'center', justifyContent: 'center' }}
      onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}
    >
      <div style={{ background: 'var(--paper)', borderRadius: 'var(--radius)', maxWidth: 480, width: '100%', padding: 32, boxShadow: 'var(--shadow-lg)', margin: 20 }}>
        <h3 style={{ fontFamily: 'Fraunces, serif', fontSize: 22, marginBottom: 6 }}>Upload <em style={{ color: 'var(--accent)' }}>new version</em></h3>
        <p style={{ fontSize: 13, color: 'var(--muted)', marginBottom: 18, lineHeight: 1.6 }}>
          Upload a corrected or enhanced version of {evidence.evidence_number}. The current version (v{evidence.version}) will be superseded but preserved. Classification is inherited.
        </p>
        <div
          style={{ border: '2px dashed var(--line-2)', borderRadius: 12, padding: '40px 20px', textAlign: 'center', cursor: 'pointer', transition: 'border-color .2s', marginBottom: 14 }}
          onClick={() => fileInputRef.current?.click()}
        >
          <div style={{ fontFamily: 'Fraunces, serif', fontSize: 28, color: 'var(--accent)', marginBottom: 8 }}>&uarr;</div>
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
      </div>
    </div>
  );
}

/* ================================================================
   Utilities
   ================================================================ */

function formatCustodyDetail(detail: string): string {
  try {
    const parsed = JSON.parse(detail);
    if (typeof parsed !== 'object' || parsed === null) return detail;
    return Object.entries(parsed)
      .filter(([, v]) => v != null && v !== '')
      .map(([k, v]) => `${k.replace(/_/g, ' ')}: ${v}`)
      .join(' \u00b7 ');
  } catch {
    return detail;
  }
}
