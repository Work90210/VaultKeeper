'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { EvidencePageClient } from '@/components/evidence/evidence-page-client';
import type { EvidenceItem } from '@/types';

interface CaseData {
  id: string;
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

const STATUS_STYLES: Record<string, { color: string; bg: string }> = {
  active: { color: 'var(--status-active)', bg: 'var(--status-active-bg)' },
  closed: { color: 'var(--status-closed)', bg: 'var(--status-closed-bg)' },
  archived: { color: 'var(--status-archived)', bg: 'var(--status-archived-bg)' },
};

type TabKey = 'overview' | 'evidence' | 'settings';

const TABS: { key: TabKey; label: string; adminOnly?: boolean }[] = [
  { key: 'overview', label: 'Overview' },
  { key: 'evidence', label: 'Evidence' },
  { key: 'settings', label: 'Settings', adminOnly: true },
];

export function CaseDetail({
  caseData,
  canEdit,
  accessToken,
  evidence,
  evidenceTotal,
  evidenceNextCursor,
  evidenceHasMore,
  canUpload,
  initialTab,
}: {
  caseData: CaseData;
  canEdit: boolean;
  accessToken: string;
  evidence: EvidenceItem[];
  evidenceTotal: number;
  evidenceNextCursor: string;
  evidenceHasMore: boolean;
  canUpload: boolean;
  initialTab?: TabKey;
}) {
  const [activeTab, setActiveTab] = useState<TabKey>(initialTab || 'overview');
  const status = STATUS_STYLES[caseData.status] || STATUS_STYLES.archived;

  const visibleTabs = TABS.filter((t) => !t.adminOnly || canEdit);

  return (
    <div style={{ animation: 'fade-in var(--duration-slow) var(--ease-out-expo)' }}>
      {/* ── Case header ── */}
      <header className="mb-[var(--space-lg)]">
        <div className="flex items-center gap-[var(--space-sm)] mb-[var(--space-xs)]">
          <span
            className="font-[family-name:var(--font-mono)] text-xs tracking-wide"
            style={{ color: 'var(--text-tertiary)' }}
          >
            {caseData.reference_code}
          </span>
          <span
            className="badge"
            style={{ backgroundColor: status.bg, color: status.color }}
          >
            {caseData.status}
          </span>
          {caseData.legal_hold && (
            <span
              className="badge"
              style={{
                backgroundColor: 'var(--status-hold-bg)',
                color: 'var(--status-hold)',
              }}
            >
              LEGAL HOLD
            </span>
          )}
        </div>
        <h1
          className="font-[family-name:var(--font-heading)] text-2xl leading-tight text-balance"
          style={{ color: 'var(--text-primary)' }}
        >
          {caseData.title}
        </h1>
      </header>

      {/* ── Tab bar ── */}
      <nav
        className="flex gap-[var(--space-lg)] mb-[var(--space-lg)]"
        style={{ borderBottom: '1px solid var(--border-default)' }}
        role="tablist"
      >
        {visibleTabs.map((tab) => {
          const isActive = activeTab === tab.key;
          return (
            <button
              key={tab.key}
              role="tab"
              type="button"
              aria-selected={isActive}
              onClick={() => setActiveTab(tab.key)}
              className="relative pb-[var(--space-sm)] text-sm font-medium transition-colors"
              style={{
                color: isActive ? 'var(--text-primary)' : 'var(--text-tertiary)',
                background: 'none',
                border: 'none',
                cursor: 'pointer',
              }}
            >
              {tab.label}
              {tab.key === 'evidence' && (
                <span
                  className="ml-[var(--space-xs)] text-xs font-normal"
                  style={{ color: 'var(--text-tertiary)' }}
                >
                  {evidenceTotal}
                </span>
              )}
              {isActive && (
                <span
                  className="absolute left-0 right-0 -bottom-px"
                  style={{
                    height: '2px',
                    backgroundColor: 'var(--amber-accent)',
                  }}
                />
              )}
            </button>
          );
        })}
      </nav>

      {/* ── Tab panels ── */}
      <div role="tabpanel">
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
  const recentEvidence = evidence.slice(0, 5);

  return (
    <div className="stagger-in space-y-[var(--space-lg)]">
      {/* Metadata strip */}
      <div className="card-inset grid grid-cols-2 sm:grid-cols-4 gap-[var(--space-md)] p-[var(--space-md)]">
        <MetaField label="Jurisdiction" value={caseData.jurisdiction || '\u2014'} />
        <MetaField
          label="Created"
          value={formatDate(caseData.created_at)}
        />
        <MetaField
          label="Updated"
          value={formatDate(caseData.updated_at)}
        />
        <MetaField
          label="Created by"
          value={caseData.created_by_name || caseData.created_by.slice(0, 8) + '\u2026'}
        />
      </div>

      {/* Description */}
      {caseData.description && (
        <div>
          <h2 className="field-label">Description</h2>
          <p
            className="text-sm leading-relaxed whitespace-pre-wrap max-w-2xl mt-[var(--space-xs)]"
            style={{ color: 'var(--text-secondary)' }}
          >
            {caseData.description}
          </p>
        </div>
      )}

      {/* Recent evidence preview */}
      <div>
        <div className="flex items-baseline justify-between mb-[var(--space-sm)]">
          <h2 className="field-label" style={{ marginBottom: 0 }}>
            Recent evidence
          </h2>
          {evidenceTotal > 0 && (
            <button
              type="button"
              onClick={onViewEvidence}
              className="link-accent text-xs font-medium"
              style={{ background: 'none', border: 'none', cursor: 'pointer' }}
            >
              View all {evidenceTotal} &rarr;
            </button>
          )}
        </div>

        {recentEvidence.length === 0 ? (
          <div
            className="card-inset p-[var(--space-lg)] text-center"
          >
            <p
              className="text-sm"
              style={{ color: 'var(--text-tertiary)' }}
            >
              No evidence uploaded yet.
            </p>
          </div>
        ) : (
          <div className="space-y-px">
            {recentEvidence.map((item) => (
              <a
                key={item.id}
                href={`/en/cases/${caseData.id}/evidence?highlight=${item.id}`}
                className="table-row flex items-center gap-[var(--space-md)] p-[var(--space-sm)] rounded-[var(--radius-md)]"
                style={{ textDecoration: 'none' }}
              >
                <span
                  className="font-[family-name:var(--font-mono)] text-xs shrink-0"
                  style={{ color: 'var(--text-tertiary)', minWidth: '5rem' }}
                >
                  {item.evidence_number}
                </span>
                <span
                  className="text-sm truncate"
                  style={{ color: 'var(--text-primary)' }}
                >
                  {item.title || item.original_name}
                </span>
                <ClassificationBadge classification={item.classification} />
                <span
                  className="text-xs ml-auto shrink-0"
                  style={{ color: 'var(--text-tertiary)' }}
                >
                  {formatDate(item.created_at)}
                </span>
              </a>
            ))}
          </div>
        )}
      </div>
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
    <div className="stagger-in">
      {error && <div className="banner-error mb-[var(--space-md)]">{error}</div>}
      {success && <div className="banner-success mb-[var(--space-md)]">{success}</div>}

      <div className="grid grid-cols-1 lg:grid-cols-[1fr_20rem] gap-[var(--space-lg)]">
        {/* Left: edit form (hidden when archived) */}
        {isArchived ? (
          <div
            className="card-inset p-[var(--space-lg)] flex items-center justify-center"
            style={{ minHeight: '12rem' }}
          >
            <p className="text-sm" style={{ color: 'var(--text-tertiary)' }}>
              This case is archived. No changes can be made.
            </p>
          </div>
        ) : (
        <form onSubmit={handleUpdate} className="space-y-[var(--space-md)]">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-[var(--space-md)]">
            <div>
              <label className="field-label" htmlFor="settings-title">Title</label>
              <input
                id="settings-title"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                maxLength={500}
                className="input-field"
              />
            </div>
            <div>
              <label className="field-label" htmlFor="settings-jurisdiction">Jurisdiction</label>
              <input
                id="settings-jurisdiction"
                value={jurisdiction}
                onChange={(e) => setJurisdiction(e.target.value)}
                maxLength={200}
                className="input-field"
              />
            </div>
          </div>
          <div>
            <label className="field-label" htmlFor="settings-description">Description</label>
            <textarea
              id="settings-description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={4}
              maxLength={10000}
              className="input-field resize-y"
            />
          </div>
          <button type="submit" disabled={loading} className="btn-primary">
            {loading ? 'Saving\u2026' : 'Save changes'}
          </button>
        </form>
        )}

        {/* Right: case actions */}
        <aside className="space-y-[var(--space-md)]">
          {/* Case status */}
          {caseStatus === 'active' && (
            <div className="card-inset p-[var(--space-md)]">
              <div className="flex items-center justify-between mb-[var(--space-sm)]">
                <h3 className="field-label" style={{ marginBottom: 0 }}>Status</h3>
                <span
                  className="badge"
                  style={{
                    backgroundColor: 'var(--status-active-bg)',
                    color: 'var(--status-active)',
                  }}
                >
                  Active
                </span>
              </div>
              <p
                className="text-xs mb-[var(--space-sm)]"
                style={{ color: 'var(--text-tertiary)', lineHeight: '1.5' }}
              >
                Close the case when investigation is complete.
              </p>
              <button
                onClick={handleCloseCase}
                className="btn-secondary w-full"
                type="button"
              >
                Close case
              </button>
            </div>
          )}

          {caseStatus === 'closed' && (
            <div className="card-inset p-[var(--space-md)]">
              <div className="flex items-center justify-between mb-[var(--space-sm)]">
                <h3 className="field-label" style={{ marginBottom: 0 }}>Status</h3>
                <span
                  className="badge"
                  style={{
                    backgroundColor: 'var(--status-closed-bg)',
                    color: 'var(--status-closed)',
                  }}
                >
                  Closed
                </span>
              </div>
              <p
                className="text-xs"
                style={{ color: 'var(--text-tertiary)', lineHeight: '1.5' }}
              >
                This case is closed. It can now be archived below.
              </p>
            </div>
          )}

          {isArchived && (
            <div className="card-inset p-[var(--space-md)]">
              <div className="flex items-center justify-between mb-[var(--space-sm)]">
                <h3 className="field-label" style={{ marginBottom: 0 }}>Status</h3>
                <span
                  className="badge"
                  style={{
                    backgroundColor: 'var(--status-archived-bg)',
                    color: 'var(--status-archived)',
                  }}
                >
                  Archived
                </span>
              </div>
              <p
                className="text-xs"
                style={{ color: 'var(--text-tertiary)', lineHeight: '1.5' }}
              >
                This case is archived and read-only. No changes can be made.
              </p>
            </div>
          )}

          {/* Legal hold — hidden when archived */}
          {!isArchived && (
          <div className="card-inset p-[var(--space-md)]">
            <div className="flex items-center justify-between mb-[var(--space-sm)]">
              <h3 className="field-label" style={{ marginBottom: 0 }}>Legal hold</h3>
              <span
                className="badge"
                style={{
                  backgroundColor: legalHold ? 'var(--status-hold-bg)' : 'var(--status-active-bg)',
                  color: legalHold ? 'var(--status-hold)' : 'var(--status-active)',
                }}
              >
                {legalHold ? 'Active' : 'Off'}
              </span>
            </div>
            <p
              className="text-xs mb-[var(--space-sm)]"
              style={{ color: 'var(--text-tertiary)', lineHeight: '1.5' }}
            >
              {legalHold
                ? 'Evidence cannot be deleted and the case cannot be archived.'
                : 'Standard lifecycle rules apply.'}
            </p>
            <button
              onClick={handleLegalHold}
              className="btn-secondary w-full"
              style={{
                borderColor: legalHold ? 'var(--status-active)' : 'var(--status-hold)',
                color: legalHold ? 'var(--status-active)' : 'var(--status-hold)',
              }}
              type="button"
            >
              {legalHold ? 'Release hold' : 'Set legal hold'}
            </button>
          </div>
          )}

          {/* Archive */}
          {!isArchived && (
            <div
              className="card-inset p-[var(--space-md)]"
              style={{ borderColor: 'var(--status-hold-bg)' }}
            >
              <h3
                className="field-label"
                style={{ color: 'var(--status-hold)', marginBottom: 'var(--space-xs)' }}
              >
                Danger zone
              </h3>
              <p
                className="text-xs mb-[var(--space-sm)]"
                style={{ color: 'var(--text-tertiary)', lineHeight: '1.5' }}
              >
                Archiving is permanent. Case must be closed first.
              </p>
              <button
                onClick={handleArchive}
                disabled={legalHold || caseStatus !== 'closed'}
                className="btn-danger w-full"
                type="button"
              >
                Archive case
              </button>
              {legalHold && (
                <p className="mt-[var(--space-xs)] text-xs" style={{ color: 'var(--status-hold)' }}>
                  Release legal hold first.
                </p>
              )}
            </div>
          )}
        </aside>
      </div>
    </div>
  );
}

/* ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
   Shared helpers
   ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ */

const CLASSIFICATION_STYLES: Record<string, { color: string; bg: string }> = {
  public: { color: 'var(--status-active)', bg: 'var(--status-active-bg)' },
  restricted: { color: 'var(--status-closed)', bg: 'var(--status-closed-bg)' },
  confidential: { color: 'var(--status-hold)', bg: 'var(--status-hold-bg)' },
  ex_parte: { color: 'var(--status-hold)', bg: 'var(--status-hold-bg)' },
};

function ClassificationBadge({ classification }: { classification: string }) {
  const style = CLASSIFICATION_STYLES[classification] || CLASSIFICATION_STYLES.public;
  return (
    <span
      className="badge shrink-0"
      style={{ backgroundColor: style.bg, color: style.color }}
    >
      {classification.replace('_', ' ')}
    </span>
  );
}

function MetaField({
  label,
  value,
  mono,
}: {
  label: string;
  value: string;
  mono?: boolean;
}) {
  return (
    <div>
      <dt className="field-label">{label}</dt>
      <dd
        className={`mt-[var(--space-xs)] text-sm ${mono ? 'font-[family-name:var(--font-mono)]' : ''}`}
        style={{ color: 'var(--text-primary)' }}
      >
        {value}
      </dd>
    </div>
  );
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('en-GB', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
  });
}
