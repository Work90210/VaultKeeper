'use client';

import { useState } from 'react';
import type { EvidenceItem } from '@/types';
import {
  StatusPill,
  Tag,
  Timeline,
} from '@/components/ui/dashboard';

/* ---------- types ---------- */

type TabId = 'custody' | 'versions' | 'redactions' | 'verification' | 'manage';

interface TimelineItem {
  readonly content: React.ReactNode;
  readonly subline?: string;
  readonly time?: string;
  readonly accent?: boolean;
}

/* ---------- stub data ---------- */

const PURPOSE_LABELS: Record<string, string> = {
  disclosure_defence: 'Defence',
  disclosure_prosecution: 'Prosecution',
  public_release: 'Public',
  court_submission: 'Court',
  witness_protection: 'Witness',
  internal_review: 'Internal',
};

const STUB_VERSIONS = [
  { id: 'e-0918-v3', version: 3, filename: 'Butcha_drone_04_v3.mp4', size: '218 MB', hash: 'a7f2e4c9\u2026', date: '18 Apr 2026', by: 'H. Morel', current: true, note: 'Colour-corrected, stabilised' },
  { id: 'e-0918-v2', version: 2, filename: 'Butcha_drone_04_v2.mp4', size: '224 MB', hash: 'd91e7b2a\u2026', date: '16 Apr 2026', by: 'H. Morel', current: false, note: 'Stabilised version' },
  { id: 'e-0918-v1', version: 1, filename: 'DJI_0487.MP4', size: '412 MB', hash: '44bfe892\u2026', date: '15 Apr 2026', by: 'H. Morel', current: false, note: 'Original capture from drone SD card' },
] as const;

const STUB_REDACTIONS = {
  drafts: [
    { id: 'rd-1', name: 'Defence disclosure', purpose: 'disclosure_defence', areas: 2, by: 'Martyna Kovacs', saved: '19 Apr, 09:14', status: 'draft' as const },
    { id: 'rd-2', name: 'Public release', purpose: 'public_release', areas: 5, by: 'Martyna Kovacs', saved: '19 Apr, 11:30', status: 'draft' as const },
  ],
  finalized: [
    { id: 'rf-1', name: 'Court submission v1', purpose: 'court_submission', areas: 3, by: 'Martyna Kovacs', date: '17 Apr 2026', evidenceNumber: 'E-0918-R1' },
  ],
};

const STUB_ASSESSMENTS = [
  { id: 'a-1', relevance: 9, reliability: 8, credibility: 'Established', recommendation: 'Collect', by: 'Amir Haddad', date: '19 Apr 2026' },
] as const;

const STUB_VERIFICATIONS = [
  { id: 'v-1', type: 'Geolocation Verification', finding: 'Authentic', confidence: 'High', method: 'EXIF GPS cross-referenced with Sentinel-2 satellite imagery', by: 'Amir Haddad', date: '18 Apr 2026' },
  { id: 'v-2', type: 'Chronolocation', finding: 'Likely Authentic', confidence: 'Medium', method: 'Shadow analysis consistent with reported time. Corroborated by W-0144 timeline.', by: 'Amir Haddad', date: '19 Apr 2026' },
] as const;

/* ---------- tab counts ---------- */
const CUSTODY_COUNT = 10;
const VERSIONS_COUNT = STUB_VERSIONS.length;
const REDACTION_COUNT = STUB_REDACTIONS.drafts.length + STUB_REDACTIONS.finalized.length;
const VERIFICATION_COUNT = STUB_ASSESSMENTS.length + STUB_VERIFICATIONS.length;

/* ================================================================
   Tabs component
   ================================================================ */

interface EvidenceDetailTabsProps {
  readonly evidence: EvidenceItem;
  readonly custodyItems: TimelineItem[];
  readonly onShowHold: () => void;
  readonly onShowDestroy: () => void;
  readonly onShowVersion: () => void;
  readonly onShowRedact: () => void;
}

export function EvidenceDetailTabs({
  evidence,
  custodyItems,
  onShowHold,
  onShowDestroy,
  onShowVersion,
  onShowRedact,
}: EvidenceDetailTabsProps) {
  const [activeTab, setActiveTab] = useState<TabId>('custody');

  const tabs: { key: TabId; label: string; count?: number }[] = [
    { key: 'custody', label: 'Custody log', count: CUSTODY_COUNT },
    { key: 'versions', label: 'Versions', count: VERSIONS_COUNT },
    { key: 'redactions', label: 'Redactions', count: REDACTION_COUNT },
    { key: 'verification', label: 'Verification', count: VERIFICATION_COUNT },
    { key: 'manage', label: 'Manage' },
  ];

  return (
    <>
      {/* Tab bar (ev-tabs styling from design prototype CSS) */}
      <div className="ev-tabs">
        {tabs.map((tab) => (
          <a
            key={tab.key}
            className={activeTab === tab.key ? 'active' : ''}
            onClick={() => setActiveTab(tab.key)}
          >
            {tab.label}
            {tab.count != null && <span className="ct">{tab.count}</span>}
          </a>
        ))}
      </div>

      {/* Tab content */}
      {activeTab === 'custody' && <CustodyTab items={custodyItems} />}
      {activeTab === 'versions' && <VersionsTab />}
      {activeTab === 'redactions' && <RedactionsTab onNewRedaction={onShowRedact} />}
      {activeTab === 'verification' && <VerificationTab />}
      {activeTab === 'manage' && (
        <ManageTab
          evidence={evidence}
          onShowHold={onShowHold}
          onShowDestroy={onShowDestroy}
          onShowVersion={onShowVersion}
          onShowRedact={onShowRedact}
        />
      )}
    </>
  );
}

/* ================================================================
   Tab: Custody log (using Timeline component)
   ================================================================ */

function CustodyTab({ items }: { readonly items: TimelineItem[] }) {
  return (
    <div style={{ paddingTop: 14 }}>
      <Timeline items={items} />
    </div>
  );
}

/* ================================================================
   Tab: Versions
   ================================================================ */

function VersionsTab() {
  return (
    <div style={{ paddingTop: 14 }}>
      {STUB_VERSIONS.map((v) => (
        <div key={v.id} className={`ver-row${v.current ? ' current' : ''}`}>
          <div className="vn-wrap">v{v.version}</div>
          <div>
            <div style={{ fontSize: 14, fontWeight: 500, color: 'var(--ink)', display: 'flex', alignItems: 'center', gap: 8 }}>
              {v.filename}
              {v.current
                ? <StatusPill status="sealed">Current</StatusPill>
                : <StatusPill status="draft">Superseded</StatusPill>
              }
            </div>
            <div style={{ fontSize: 12, color: 'var(--muted)', marginTop: 4, lineHeight: 1.5 }}>
              {v.note} &middot; <span style={{ fontFamily: '"JetBrains Mono", monospace', fontSize: 11 }}>{v.hash}</span>
            </div>
          </div>
          <div style={{ textAlign: 'right' }}>
            <div style={{ fontSize: 12, color: 'var(--ink-2)' }}>{v.date}</div>
            <div style={{ fontSize: 11, color: 'var(--muted-2)', marginTop: 2 }}>{v.by}</div>
          </div>
        </div>
      ))}
    </div>
  );
}

/* ================================================================
   Tab: Redactions
   ================================================================ */

function RedactionsTab({ onNewRedaction }: { readonly onNewRedaction: () => void }) {
  return (
    <div style={{ paddingTop: 14 }}>
      {STUB_REDACTIONS.drafts.length > 0 && (
        <>
          <div className="sb-label" style={{ marginBottom: 10 }}>Drafts in progress</div>
          {STUB_REDACTIONS.drafts.map((d) => (
            <div key={d.id} className="red-card">
              <div className="red-head">
                <div>
                  <span className="red-name">{d.name}</span>
                  {' '}<Tag className="ml-1">{PURPOSE_LABELS[d.purpose] || d.purpose}</Tag>
                </div>
                <div style={{ display: 'flex', gap: 6 }}>
                  <a className="linkarrow" style={{ fontSize: 12, cursor: 'pointer' }}>Resume</a>
                  <a style={{ fontSize: 12, color: '#b35c5c', cursor: 'pointer' }}>Discard</a>
                </div>
              </div>
              <div className="red-meta">
                {d.areas} area{d.areas !== 1 ? 's' : ''} &middot; Last saved {d.saved} &middot; {d.by}
              </div>
            </div>
          ))}
        </>
      )}
      {STUB_REDACTIONS.finalized.length > 0 && (
        <>
          <div className="sb-label" style={{ marginTop: 18, marginBottom: 10 }}>Finalized</div>
          {STUB_REDACTIONS.finalized.map((f) => (
            <div key={f.id} className="red-card">
              <div className="red-head">
                <div>
                  <span className="red-name">{f.name}</span>
                  {' '}<Tag className="ml-1">{PURPOSE_LABELS[f.purpose] || f.purpose}</Tag>
                  {' '}<StatusPill status="sealed">Applied</StatusPill>
                </div>
                <div style={{ display: 'flex', gap: 6 }}>
                  <a className="linkarrow" style={{ fontSize: 12, cursor: 'pointer' }}>View</a>
                  <a className="linkarrow" style={{ fontSize: 12, cursor: 'pointer' }}>Download</a>
                </div>
              </div>
              <div className="red-meta">
                {f.areas} area{f.areas !== 1 ? 's' : ''} &middot; Finalized {f.date} &middot; {f.by} &middot;{' '}
                <span style={{ fontFamily: '"JetBrains Mono", monospace', fontSize: 11 }}>{f.evidenceNumber}</span>
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

function VerificationTab() {
  return (
    <div style={{ paddingTop: 14 }}>
      {STUB_ASSESSMENTS.length > 0 && (
        <>
          <div className="sb-label" style={{ marginBottom: 10 }}>Assessment</div>
          {STUB_ASSESSMENTS.map((a) => (
            <div key={a.id} className="inv-card">
              <div className="inv-head">
                <span style={{ fontWeight: 500 }}>Evidence Assessment</span>
                <span style={{ fontSize: 12, color: 'var(--muted)' }}>{a.date}</span>
              </div>
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 12, marginTop: 8 }}>
                <div>
                  <div style={{ fontFamily: '"JetBrains Mono", monospace', fontSize: 10, color: 'var(--muted)', textTransform: 'uppercase', letterSpacing: '.06em' }}>Relevance</div>
                  <div style={{ fontFamily: '"Fraunces", serif', fontSize: 24, color: 'var(--ink)' }}>{a.relevance}<span style={{ fontSize: 14, color: 'var(--muted)' }}>/10</span></div>
                </div>
                <div>
                  <div style={{ fontFamily: '"JetBrains Mono", monospace', fontSize: 10, color: 'var(--muted)', textTransform: 'uppercase', letterSpacing: '.06em' }}>Reliability</div>
                  <div style={{ fontFamily: '"Fraunces", serif', fontSize: 24, color: 'var(--ink)' }}>{a.reliability}<span style={{ fontSize: 14, color: 'var(--muted)' }}>/10</span></div>
                </div>
                <div>
                  <div style={{ fontFamily: '"JetBrains Mono", monospace', fontSize: 10, color: 'var(--muted)', textTransform: 'uppercase', letterSpacing: '.06em' }}>Credibility</div>
                  <div style={{ fontSize: 13, color: 'var(--ink)', marginTop: 6 }}><StatusPill status="sealed">{a.credibility}</StatusPill></div>
                </div>
                <div>
                  <div style={{ fontFamily: '"JetBrains Mono", monospace', fontSize: 10, color: 'var(--muted)', textTransform: 'uppercase', letterSpacing: '.06em' }}>Recommendation</div>
                  <div style={{ fontSize: 13, color: 'var(--ink)', marginTop: 6 }}><StatusPill status="live">{a.recommendation}</StatusPill></div>
                </div>
              </div>
              <div style={{ fontSize: 12, color: 'var(--muted)', marginTop: 8 }}>By {a.by}</div>
            </div>
          ))}
        </>
      )}
      {STUB_VERIFICATIONS.length > 0 && (
        <>
          <div className="sb-label" style={{ marginTop: 18, marginBottom: 10 }}>Verification records</div>
          {STUB_VERIFICATIONS.map((v) => {
            const findingCls = v.finding === 'Authentic' ? 'sealed' as const : v.finding === 'Likely Authentic' ? 'live' as const : 'draft' as const;
            return (
              <div key={v.id} className="inv-card">
                <div className="inv-head">
                  <span style={{ fontWeight: 500 }}>{v.type}</span>
                  <StatusPill status={findingCls}>{v.finding}</StatusPill>
                </div>
                <div style={{ fontSize: 13, color: 'var(--ink-2)', lineHeight: 1.55, marginTop: 6 }}>{v.method}</div>
                <div style={{ display: 'flex', gap: 14, marginTop: 8, fontSize: 12, color: 'var(--muted)' }}>
                  <span>Confidence: <strong style={{ color: 'var(--ink)' }}>{v.confidence}</strong></span>
                  <span>By {v.by} &middot; {v.date}</span>
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
   Tab: Manage (2-column card grid)
   ================================================================ */

function ManageTab({
  evidence,
  onShowHold,
  onShowDestroy,
  onShowVersion,
  onShowRedact,
}: {
  readonly evidence: EvidenceItem;
  readonly onShowHold: () => void;
  readonly onShowDestroy: () => void;
  readonly onShowVersion: () => void;
  readonly onShowRedact: () => void;
}) {
  const classificationDescriptions: Record<string, string> = {
    public: 'Accessible to all case members and may be shared publicly.',
    restricted: 'Accessible to assigned team members only. Default for most evidence.',
    confidential: 'Restricted to lead investigators and admins. Every access is logged.',
    ex_parte: 'Visible only to one party (prosecution or defence). Requires side designation.',
  };

  const classificationColors: Record<string, string> = {
    public: 'var(--ok)',
    restricted: 'var(--accent)',
    confidential: '#b35c5c',
    ex_parte: '#5b4a6b',
  };

  const classificationBgs: Record<string, string> = {
    public: 'rgba(74,107,58,.1)',
    restricted: 'rgba(184,66,28,.1)',
    confidential: 'rgba(179,92,92,.1)',
    ex_parte: 'rgba(91,74,107,.1)',
  };

  const classificationLabels: Record<string, string> = {
    public: 'Public',
    restricted: 'Restricted',
    confidential: 'Confidential',
    ex_parte: 'Ex Parte',
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
            return (
              <span key={c} style={{
                padding: '8px 16px', borderRadius: 999, fontSize: 13, fontWeight: 500, cursor: 'pointer',
                border: `2px solid ${active ? classificationColors[c] : 'var(--line-2)'}`,
                background: active ? classificationBgs[c] : 'transparent',
                color: active ? classificationColors[c] : 'var(--muted)',
                transition: 'all .15s',
              }}>
                {classificationLabels[c]}
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

      {/* Integrity actions card */}
      <div className="inv-card" style={{ gridColumn: 'span 2' }}>
        <div style={{ fontWeight: 500, fontSize: 15, marginBottom: 12 }}>Integrity actions</div>
        <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap' }}>
          <button type="button" className="btn ghost sm">Re-verify hash integrity</button>
          <button type="button" className="btn ghost sm">Request new RFC 3161 timestamp</button>
          <button type="button" className="btn ghost sm">Export custody chain (PDF)</button>
          <a className="btn ghost sm" href={`/api/evidence/${evidence.id}/download`} download style={{ textDecoration: 'none' }}>Download original</a>
        </div>
      </div>

      {/* Retention & destruction card */}
      <div className="inv-card" style={{ gridColumn: 'span 2' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
          <span style={{ fontWeight: 500, fontSize: 15 }}>Retention &amp; destruction</span>
          <StatusPill status="sealed">Active</StatusPill>
        </div>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 14, marginBottom: 14 }}>
          <div style={{ padding: 12, border: '1px solid var(--line)', borderRadius: 8, background: 'var(--bg-2)' }}>
            <div style={{ fontFamily: '"JetBrains Mono", monospace', fontSize: 9, color: 'var(--muted-2)', textTransform: 'uppercase', letterSpacing: '.08em', marginBottom: 4 }}>Policy</div>
            <div style={{ fontSize: 13, color: 'var(--ink)' }}>Case closed + 50 yr</div>
          </div>
          <div style={{ padding: 12, border: '1px solid var(--line)', borderRadius: 8, background: 'var(--bg-2)' }}>
            <div style={{ fontFamily: '"JetBrains Mono", monospace', fontSize: 9, color: 'var(--muted-2)', textTransform: 'uppercase', letterSpacing: '.08em', marginBottom: 4 }}>Review date</div>
            <div style={{ fontSize: 13, color: 'var(--ink)' }}>30 Jun 2076</div>
          </div>
          <div style={{ padding: 12, border: '1px solid var(--line)', borderRadius: 8, background: 'var(--bg-2)' }}>
            <div style={{ fontFamily: '"JetBrains Mono", monospace', fontSize: 9, color: 'var(--muted-2)', textTransform: 'uppercase', letterSpacing: '.08em', marginBottom: 4 }}>Status</div>
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
