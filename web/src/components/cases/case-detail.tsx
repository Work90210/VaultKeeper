'use client';

import { useState, useCallback, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { EvidencePageClient } from '@/components/evidence/evidence-page-client';
import { ImportArchive } from '@/components/evidence/import-archive';
import { WitnessList } from '@/components/witnesses/witness-list';
import { DisclosureList } from '@/components/disclosures/disclosure-list';
import { InvestigationPageClient } from '@/components/investigation/investigation-page-client';
import type { EvidenceItem, Witness, Disclosure, CaseRole } from '@/types';
import { CaseMembersPanel } from './case-members-panel';
import { useCaseContext } from '@/components/providers/case-provider';

interface CaseData {
  id: string;
  organization_id?: string;
  reference_code: string;
  title: string;
  description: string;
  jurisdiction: string;
  status: string;
  legal_hold: boolean;
  created_by: string;
  created_by_name: string;
  created_at: string;
  updated_at: string;
}

// Status pill class mapping for .pl design component


export type TabKey =
  | 'overview' | 'evidence'
  | 'witnesses' | 'disclosures' | 'members'
  | 'inquiry-logs' | 'assessments' | 'verifications' | 'corroborations' | 'analysis' | 'safety'
  | 'templates' | 'reports'
  | 'settings';

const INVESTIGATION_SECTIONS = new Set<TabKey>([
  'inquiry-logs', 'assessments', 'verifications', 'corroborations', 'analysis', 'safety', 'templates', 'reports',
]);

export function CaseDetail({
  caseData,
  canEdit,
  accessToken,
  userId,
  evidence,
  evidenceTotal,
  evidenceNextCursor,
  evidenceHasMore,
  canUpload,
  initialTab,
  witnesses = [],
  disclosures = [],
  caseMembers = [],
  isProsecutor = false,
}: {
  caseData: CaseData;
  canEdit: boolean;
  accessToken: string;
  userId: string;
  evidence: EvidenceItem[];
  evidenceTotal: number;
  evidenceNextCursor: string;
  evidenceHasMore: boolean;
  canUpload: boolean;
  initialTab?: TabKey;
  witnesses?: Witness[];
  disclosures?: Disclosure[];
  caseMembers?: CaseRole[];
  isProsecutor?: boolean;
}) {
  const {
    setCaseData,
    activeTab: contextTab,
    setActiveTab: contextSetActiveTab,
    setSidebarCounts,
    updateSidebarCounts,
  } = useCaseContext();

  const resolvedInitial: TabKey =
    initialTab === ('investigation' as string) ? 'inquiry-logs' : (initialTab || 'overview');

  // Register case data in context for the sidebar
  useEffect(() => {
    setCaseData({
      id: caseData.id,
      reference_code: caseData.reference_code,
      title: caseData.title,
      status: caseData.status,
      canEdit,
    });
    contextSetActiveTab(resolvedInitial);
    return () => {
      setCaseData(null);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [caseData.id, caseData.reference_code, caseData.title, caseData.status, canEdit]);

  const activeTab = contextTab as TabKey || resolvedInitial;
  const setActiveTab = contextSetActiveTab;

  const isInvestigationSection = INVESTIGATION_SECTIONS.has(activeTab);

  const handleCountsLoaded = useCallback((counts: Record<string, number>) => {
    updateSidebarCounts(counts);
  }, [updateSidebarCounts]);

  // Eagerly fetch investigation counts for sidebar badges
  useEffect(() => {
    const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';
    const headers = { Authorization: `Bearer ${accessToken}` };
    const fetchCount = (url: string) =>
      fetch(url, { headers })
        .then(r => r.ok ? r.json() : null)
        .then(json => (json?.data ?? json ?? []).length)
        .catch(() => 0);

    Promise.all([
      fetchCount(`${API_BASE}/api/cases/${caseData.id}/inquiry-logs`),
      fetchCount(`${API_BASE}/api/cases/${caseData.id}/assessments`),
      fetchCount(`${API_BASE}/api/cases/${caseData.id}/verifications`),
      fetchCount(`${API_BASE}/api/cases/${caseData.id}/corroborations`),
      fetchCount(`${API_BASE}/api/cases/${caseData.id}/analysis-notes`),
      fetchCount(`${API_BASE}/api/cases/${caseData.id}/safety-profiles`),
      fetchCount(`${API_BASE}/api/cases/${caseData.id}/template-instances`),
      fetchCount(`${API_BASE}/api/cases/${caseData.id}/reports`),
    ]).then(([inq, assess, verify, corr, analysis, safety, templates, reports]) => {
      updateSidebarCounts({
        'inquiry-logs': inq,
        assessments: assess,
        verifications: verify,
        corroborations: corr,
        analysis,
        safety,
        templates,
        reports,
      });
    });
  }, [caseData.id, accessToken, updateSidebarCounts]);

  // Push static counts to sidebar context
  useEffect(() => {
    setSidebarCounts({
      evidence: evidenceTotal,
      witnesses: witnesses.length,
      disclosures: disclosures.length,
      members: caseMembers.length,
    });
  }, [evidenceTotal, witnesses.length, disclosures.length, caseMembers.length, setSidebarCounts]);

  const PL_STATUS: Record<string, string> = {
    active: 'sealed',
    closed: 'disc',
    archived: 'draft',
  };

  return (
    <div>
      {/* ── Case header ── */}
      <section className="d-pagehead">
        <div>
          <span className="eyebrow-m">
            Case · {caseData.jurisdiction || 'Criminal'} · since {formatDate(caseData.created_at)}
          </span>
          <h1>
            {caseData.reference_code} · <em>{caseData.title}</em>
          </h1>
          <p className="sub">
            {caseData.description || '\u2014'}
            {' '}
            <span className={`pl ${PL_STATUS[caseData.status] || 'draft'}`}>{caseData.status}</span>
            {caseData.legal_hold && <span className="pl hold" style={{ marginLeft: 6 }}>Legal hold</span>}
          </p>
        </div>
        <div className="actions">
          <button type="button" onClick={() => setActiveTab('disclosures')} className="btn ghost">
            Create disclosure
          </button>
          <button type="button" onClick={() => setActiveTab('evidence')} className="btn">
            Upload evidence <span className="arr">{'\u2192'}</span>
          </button>
        </div>
      </section>

      {/* ── Tabs ── */}
      <div className="panel" style={{ marginBottom: 22 }}>
        <div className="tabs">
          <a href="#" className={activeTab === 'overview' ? 'active' : ''} onClick={(e) => { e.preventDefault(); setActiveTab('overview'); }}>
            Overview<span className="ct">{'\u00B7'}</span>
          </a>
          <a href="#" className={activeTab === 'evidence' ? 'active' : ''} onClick={(e) => { e.preventDefault(); setActiveTab('evidence'); }}>
            Evidence<span className="ct">{evidenceTotal}</span>
          </a>
          <a href="#" className={activeTab === 'witnesses' ? 'active' : ''} onClick={(e) => { e.preventDefault(); setActiveTab('witnesses'); }}>
            Witnesses<span className="ct">{witnesses.length}</span>
          </a>
          <a href="#" className={activeTab === 'analysis' ? 'active' : ''} onClick={(e) => { e.preventDefault(); setActiveTab('analysis'); }}>
            Notes
          </a>
          <a href="#" className={activeTab === 'corroborations' ? 'active' : ''} onClick={(e) => { e.preventDefault(); setActiveTab('corroborations'); }}>
            Corroborations
          </a>
          <a href="#" className={activeTab === 'disclosures' ? 'active' : ''} onClick={(e) => { e.preventDefault(); setActiveTab('disclosures'); }}>
            Disclosures<span className="ct">{disclosures.length}</span>
          </a>
          <a href="#" className={activeTab === 'inquiry-logs' ? 'active' : ''} onClick={(e) => { e.preventDefault(); setActiveTab('inquiry-logs'); }}>
            Chain
          </a>
          {canEdit && (
            <a href="#" className={activeTab === 'settings' ? 'active' : ''} onClick={(e) => { e.preventDefault(); setActiveTab('settings'); }}>
              Settings
            </a>
          )}
        </div>
      </div>

      {/* ── Content area ── */}
      <div>
        {activeTab === 'overview' && (
          <OverviewPanel
            caseData={caseData}
            evidence={evidence}
            evidenceTotal={evidenceTotal}
            onViewEvidence={() => setActiveTab('evidence')}
          />
        )}

        {activeTab === 'evidence' && (
          <EvidencePageClient
            caseId={caseData.id}
            accessToken={accessToken}
            canUpload={canUpload}
            evidence={evidence}
            nextCursor={evidenceNextCursor}
            hasMore={evidenceHasMore}
            currentQuery=""
            currentClassification=""
          />
        )}

        {activeTab === 'witnesses' && (
          <WitnessList
            witnesses={witnesses}
            onSelect={(id) => window.location.href = `/en/witnesses/${id}`}
            onAddNew={canUpload ? () => window.location.href = `/en/cases/${caseData.id}/witnesses/new` : undefined}
            canEdit={canUpload}
          />
        )}

        {activeTab === 'disclosures' && (
          <DisclosureList
            disclosures={disclosures}
            onSelect={(id) => window.location.href = `/en/disclosures/${id}`}
            onCreateNew={isProsecutor ? () => window.location.href = `/en/cases/${caseData.id}/disclosures/new` : undefined}
            canCreate={isProsecutor}
          />
        )}

        {activeTab === 'members' && (
          <CaseMembersPanel
            caseId={caseData.id}
            members={caseMembers}
            canManage={canEdit}
            organizationId={caseData.organization_id}
            accessToken={accessToken}
          />
        )}

        {isInvestigationSection && (
          <InvestigationPageClient
            caseId={caseData.id}
            accessToken={accessToken}
            userId={userId}
            activeSection={activeTab}
            onCountsLoaded={handleCountsLoaded}
            inquiryLogs={[]}
            assessments={[]}
            verifications={[]}
            corroborations={[]}
            analysisNotes={[]}
            templates={[]}
            templateInstances={[]}
            reports={[]}
            safetyProfiles={[]}
          />
        )}

        {activeTab === 'settings' && canEdit && (
          <SettingsPanel caseData={caseData} accessToken={accessToken} />
        )}
      </div>
    </div>
  );
}

/* ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
   Overview Panel
   ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ */

function OverviewPanel({
  caseData,
  evidence,
  evidenceTotal,
  onViewEvidence,
}: {
  caseData: CaseData;
  evidence: EvidenceItem[];
  evidenceTotal: number;
  onViewEvidence: () => void;
}) {
  return (
    <div>
      {/* Case metadata + chain */}
      <div className="panel" style={{ marginBottom: 22 }}>
        <div className="panel-body">
          <div className="g2-wide" style={{ gap: 32, alignItems: 'start' }}>
            <dl className="kvs">
              <dt>Reference</dt>
              <dd><strong>{caseData.reference_code}</strong></dd>
              <dt>Jurisdiction</dt>
              <dd>{caseData.jurisdiction || '\u2014'}</dd>
              <dt>Classification</dt>
              <dd>{caseData.legal_hold ? 'Legal hold active' : 'Standard lifecycle'}</dd>
              <dt>Custodian</dt>
              <dd>{caseData.created_by_name || caseData.created_by.slice(0, 8)}</dd>
              <dt>Created</dt>
              <dd>{formatDate(caseData.created_at)}</dd>
              <dt>Last updated</dt>
              <dd>{formatDate(caseData.updated_at)}</dd>
            </dl>
            <div style={{ background: 'var(--bg-2)', border: '1px solid var(--line)', borderRadius: 12, padding: 18 }}>
              <div style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: '10.5px', letterSpacing: '.08em', color: 'var(--muted)', textTransform: 'uppercase' as const, marginBottom: 12 }}>
                Chain of custody
              </div>
              <div style={{ display: 'flex', flexDirection: 'column' as const, gap: 8, fontFamily: "'JetBrains Mono', monospace", fontSize: '11.5px' }}>
                <div style={{ display: 'flex', gap: 10, padding: '8px 10px', background: 'var(--paper)', border: '1px solid var(--line)', borderRadius: 6 }}>
                  <span style={{ color: 'var(--accent)' }}>head</span>
                  <span style={{ color: 'var(--muted)', marginLeft: 'auto' }}>{'\u25CF'} {evidenceTotal} exhibits</span>
                </div>
              </div>
              <div style={{ marginTop: 14, display: 'flex', justifyContent: 'space-between', fontSize: '11.5px', color: 'var(--muted)', fontFamily: "'JetBrains Mono', monospace" }}>
                <span>Status: {caseData.status}</span>
                <button type="button" onClick={onViewEvidence} className="linkarrow" style={{ fontSize: 12 }}>
                  View all evidence {'\u2192'}
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Recent evidence table */}
      {evidence.length > 0 && (
        <div className="panel" style={{ marginBottom: 22 }}>
          <div className="panel-h">
            <h3>Recent <em>evidence</em></h3>
            <span className="meta">{evidenceTotal} exhibits</span>
          </div>
          <div className="panel-body flush">
            <table className="tbl">
              <thead>
                <tr>
                  <th>File</th>
                  <th>Reference</th>
                  <th>Classification</th>
                  <th>Date</th>
                </tr>
              </thead>
              <tbody>
                {evidence.slice(0, 5).map((item) => (
                  <tr key={item.id} onClick={() => window.location.href = `/en/evidence/${item.id}`} style={{ cursor: 'pointer' }}>
                    <td>
                      <span className="ref">{item.title || item.original_name}<small>{item.evidence_number}</small></span>
                    </td>
                    <td><code>{item.evidence_number}</code></td>
                    <td><span className={`pl ${item.classification === 'public' ? 'sealed' : item.classification === 'restricted' ? 'hold' : 'draft'}`}>{item.classification}</span></td>
                    <td>{formatDate(item.created_at)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {evidence.length === 0 && (
        <div className="ph" style={{ textAlign: 'center', padding: '48px 24px' }}>
          No evidence uploaded yet. Upload your first exhibit to begin the chain of custody.
        </div>
      )}
    </div>
  );
}

/* ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
   Settings Panel
   ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ */

function SettingsPanel({
  caseData,
  accessToken,
}: {
  caseData: CaseData;
  accessToken: string;
}) {
  const router = useRouter();
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const [title, setTitle] = useState(caseData.title);
  const [description, setDescription] = useState(caseData.description);
  const [jurisdiction, setJurisdiction] = useState(caseData.jurisdiction);
  const [legalHold, setLegalHold] = useState(caseData.legal_hold);
  const [caseStatus, setCaseStatus] = useState(caseData.status);
  const [loading, setLoading] = useState(false);

  const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

  const headers = {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${accessToken}`,
  };

  const handleUpdate = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setSuccess('');
    setLoading(true);
    try {
      const res = await fetch(`${API_BASE}/api/cases/${caseData.id}`, {
        method: 'PATCH',
        headers,
        body: JSON.stringify({ title, description, jurisdiction }),
      });
      const data = await res.json();
      if (!res.ok) {
        setError(data.error || 'Update failed');
        return;
      }
      setSuccess('Changes saved');
    } catch {
      setError('An unexpected error occurred. Please try again.');
    } finally {
      setLoading(false);
    }
  };

  const handleLegalHold = async () => {
    const newHold = !legalHold;
    if (
      !window.confirm(
        newHold
          ? 'Set legal hold? This prevents archival.'
          : 'Release legal hold?',
      )
    )
      return;
    const res = await fetch(
      `${API_BASE}/api/cases/${caseData.id}/legal-hold`,
      { method: 'POST', headers, body: JSON.stringify({ hold: newHold }) },
    );
    if (res.ok) {
      setLegalHold(newHold);
      setSuccess(newHold ? 'Legal hold set' : 'Legal hold released');
      setError('');
    } else {
      const data = await res.json();
      setError(data.error || 'Failed');
    }
  };

  const handleCloseCase = async () => {
    if (!window.confirm('Close this case? It can still be reopened by creating a new case.')) return;
    setError('');
    setSuccess('');
    try {
      const res = await fetch(`${API_BASE}/api/cases/${caseData.id}`, {
        method: 'PATCH',
        headers,
        body: JSON.stringify({ status: 'closed' }),
      });
      if (res.ok) {
        setCaseStatus('closed');
        setSuccess('Case closed');
      } else {
        const data = await res.json();
        setError(data.error || 'Failed to close case');
      }
    } catch {
      setError('An unexpected error occurred.');
    }
  };

  const handleArchive = async () => {
    if (!window.confirm('Archive this case? This cannot be undone.')) return;
    const res = await fetch(`${API_BASE}/api/cases/${caseData.id}/archive`, {
      method: 'POST',
      headers,
    });
    if (res.ok) router.refresh();
    else {
      const data = await res.json();
      setError(data.error || 'Archive failed');
    }
  };

  const isArchived = caseStatus === 'archived';

  return (
    <div>
      {error && <div className="pl hold" style={{ padding: '12px 18px', borderRadius: 10, marginBottom: 16 }}>{error}</div>}
      {success && <div className="pl sealed" style={{ padding: '12px 18px', borderRadius: 10, marginBottom: 16 }}>{success}</div>}

      <div className="g2-wide" style={{ alignItems: 'start' }}>
        {/* Left: edit form (hidden when archived) */}
        {isArchived ? (
          <div className="ph" style={{ textAlign: 'center', padding: '48px 24px' }}>
            This case is archived. No changes can be made.
          </div>
        ) : (
        <div className="panel">
          <div className="panel-h"><h3>Case <em>details</em></h3></div>
          <div className="panel-body">
            <form onSubmit={handleUpdate} style={{ display: 'flex', flexDirection: 'column', gap: 18 }}>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 18 }}>
                <div>
                  <label htmlFor="settings-title" style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: '10.5px', letterSpacing: '.08em', color: 'var(--muted)', textTransform: 'uppercase' as const, display: 'block', marginBottom: 6 }}>Title</label>
                  <input id="settings-title" value={title} onChange={(e) => setTitle(e.target.value)} maxLength={500} style={{ width: '100%', padding: '10px 14px', border: '1px solid var(--line-2)', borderRadius: 10, background: 'var(--paper)', font: 'inherit', fontSize: '14px', color: 'var(--ink)' }} />
                </div>
                <div>
                  <label htmlFor="settings-jurisdiction" style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: '10.5px', letterSpacing: '.08em', color: 'var(--muted)', textTransform: 'uppercase' as const, display: 'block', marginBottom: 6 }}>Jurisdiction</label>
                  <input id="settings-jurisdiction" value={jurisdiction} onChange={(e) => setJurisdiction(e.target.value)} maxLength={200} style={{ width: '100%', padding: '10px 14px', border: '1px solid var(--line-2)', borderRadius: 10, background: 'var(--paper)', font: 'inherit', fontSize: '14px', color: 'var(--ink)' }} />
                </div>
              </div>
              <div>
                <label htmlFor="settings-description" style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: '10.5px', letterSpacing: '.08em', color: 'var(--muted)', textTransform: 'uppercase' as const, display: 'block', marginBottom: 6 }}>Description</label>
                <textarea id="settings-description" value={description} onChange={(e) => setDescription(e.target.value)} rows={4} maxLength={10000} style={{ width: '100%', padding: '10px 14px', border: '1px solid var(--line-2)', borderRadius: 10, background: 'var(--paper)', font: 'inherit', fontSize: '14px', color: 'var(--ink)', resize: 'vertical' }} />
              </div>
              <button type="submit" disabled={loading} className="btn" style={{ alignSelf: 'flex-start' }}>
                {loading ? 'Saving\u2026' : 'Save changes'}
              </button>
            </form>
          </div>
        </div>
        )}

        {/* Right: case actions */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div className="panel">
            <div className="panel-h">
              <h3>Status</h3>
              <span className={`pl ${caseStatus === 'active' ? 'sealed' : caseStatus === 'closed' ? 'disc' : 'draft'}`}>{caseStatus}</span>
            </div>
            <div className="panel-body">
              {caseStatus === 'active' && (
                <>
                  <p style={{ fontSize: '13px', color: 'var(--muted)', lineHeight: 1.5, marginBottom: 14 }}>Close the case when investigation is complete.</p>
                  <button onClick={handleCloseCase} className="btn ghost" style={{ width: '100%', justifyContent: 'center' }} type="button">Close case</button>
                </>
              )}
              {caseStatus === 'closed' && <p style={{ fontSize: '13px', color: 'var(--muted)', lineHeight: 1.5 }}>This case is closed. It can now be archived below.</p>}
              {isArchived && <p style={{ fontSize: '13px', color: 'var(--muted)', lineHeight: 1.5 }}>This case is archived and read-only.</p>}
            </div>
          </div>

          {!isArchived && (
          <div className="panel">
            <div className="panel-h">
              <h3>Legal hold</h3>
              <span className={`pl ${legalHold ? 'hold' : 'sealed'}`}>{legalHold ? 'Active' : 'Off'}</span>
            </div>
            <div className="panel-body">
              <p style={{ fontSize: '13px', color: 'var(--muted)', lineHeight: 1.5, marginBottom: 14 }}>
                {legalHold ? 'Evidence cannot be deleted and the case cannot be archived.' : 'Standard lifecycle rules apply.'}
              </p>
              <button onClick={handleLegalHold} className={`btn ${legalHold ? '' : 'ghost'}`} style={{ width: '100%', justifyContent: 'center' }} type="button">
                {legalHold ? 'Release hold' : 'Set legal hold'}
              </button>
            </div>
          </div>
          )}

          {!isArchived && (
          <div className="panel" style={{ borderColor: 'rgba(184,66,28,.3)' }}>
            <div className="panel-h" style={{ borderBottomColor: 'rgba(184,66,28,.15)' }}>
              <h3 style={{ color: 'var(--accent)' }}>Danger zone</h3>
            </div>
            <div className="panel-body">
              <p style={{ fontSize: '13px', color: 'var(--muted)', lineHeight: 1.5, marginBottom: 14 }}>Archiving is permanent. Case must be closed first.</p>
              <button onClick={handleArchive} disabled={legalHold || caseStatus !== 'closed'} className="btn" style={{ width: '100%', justifyContent: 'center', background: 'var(--accent)', borderColor: 'var(--accent)', opacity: (legalHold || caseStatus !== 'closed') ? 0.4 : 1 }} type="button">
                Archive case
              </button>
              {legalHold && <p style={{ marginTop: 8, fontSize: '12px', color: 'var(--accent)' }}>Release legal hold first.</p>}
            </div>
          </div>
          )}
        </div>
      </div>

      {!isArchived && (
        <div style={{ marginTop: 22 }}>
          <div className="panel">
            <div className="panel-h"><h3>Data <em>import</em></h3></div>
            <div className="panel-body">
              <p style={{ fontSize: '13.5px', color: 'var(--muted)', lineHeight: 1.55, marginBottom: 18 }}>
                Bulk-import evidence from another system (e.g. RelativityOne).
                If your archive contains a <code>manifest.csv</code> at the root,
                every file{"'"}s source hash is verified on ingestion, the batch is
                stamped with a trusted RFC 3161 timestamp, and you receive a signed
                attestation certificate for court submission.
              </p>
              <ImportArchive
                caseId={caseData.id}
                accessToken={accessToken}
                onImportComplete={() => window.location.reload()}
              />
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

/* ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
   Shared helpers
   ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ */

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('en-GB', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
  });
}
