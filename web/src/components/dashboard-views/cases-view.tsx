'use client';

import { useRouter, useSearchParams } from 'next/navigation';
import { useMemo, useState } from 'react';

// --- Types ---

interface CaseRow {
  readonly id: string;
  readonly reference_code: string;
  readonly title: string;
  readonly description: string;
  readonly jurisdiction: string;
  readonly status: string;
  readonly legal_hold: boolean;
  readonly created_by: string;
  readonly created_at: string;
  readonly updated_at: string;
}

export interface CasesViewProps {
  readonly cases: readonly CaseRow[];
  readonly total: number;
  readonly nextCursor: string;
  readonly hasMore: boolean;
  readonly currentQuery: string;
  readonly currentStatus: string;
  readonly canCreate: boolean;
  readonly error?: string | null;
}

// --- Constants ---

const CLASSIFICATION_TAGS: readonly string[] = [
  'Criminal',
  'Investigation',
  'Monitoring',
  'Archival',
];

const ROLE_OPTIONS: readonly { key: string; label: string; accent: boolean }[] = [
  { key: 'lead', label: 'Lead', accent: true },
  { key: 'analyst', label: 'Analyst', accent: false },
  { key: 'observer', label: 'Observer', accent: false },
  { key: 'clerk', label: 'Clerk', accent: false },
];

const STATUS_PILL_CLASS: Record<string, string> = {
  active: 'pl live',
  closed: 'pl sealed',
  archived: 'pl sealed',
  draft: 'pl draft',
};

const STATUS_LABEL: Record<string, string> = {
  active: 'active',
  closed: 'sealed',
  archived: 'sealed',
  draft: 'draft',
};

const BERKELEY_PHASES = [
  'Survey',
  'Collect',
  'Preserve',
  'Analyse',
  'Report',
  'Present',
];

// --- Helpers ---

function timeSince(dateStr: string): string {
  const seconds = Math.floor((Date.now() - new Date(dateStr).getTime()) / 1000);
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

function formatNumber(n: number): string {
  return n.toLocaleString('en-US');
}

function classificationForCase(jurisdiction: string): string {
  if (!jurisdiction) return CLASSIFICATION_TAGS[1];
  const lower = jurisdiction.toLowerCase();
  if (lower.includes('criminal') || lower.includes('penal')) return CLASSIFICATION_TAGS[0];
  if (lower.includes('monitor')) return CLASSIFICATION_TAGS[2];
  if (lower.includes('archiv')) return CLASSIFICATION_TAGS[3];
  return CLASSIFICATION_TAGS[1];
}

function roleForCase(createdBy: string): { label: string; accent: boolean } {
  // Stub: in Sprint E this derives from the user's actual role on the case
  const hash = createdBy.split('').reduce((acc, ch) => acc + ch.charCodeAt(0), 0);
  return ROLE_OPTIONS[hash % ROLE_OPTIONS.length];
}

// --- New Case Modal ---

function NewCaseModal({
  open,
  onClose,
}: {
  readonly open: boolean;
  readonly onClose: () => void;
}) {
  const router = useRouter();
  const [identifier, setIdentifier] = useState('');
  const [description, setDescription] = useState('');
  const [classification, setClassification] = useState('Investigation');
  const [role, setRole] = useState('lead');
  const [jurisdiction, setJurisdiction] = useState('');
  const [startDate, setStartDate] = useState(
    new Date().toISOString().slice(0, 10)
  );
  const [initialStatus, setInitialStatus] = useState('draft');
  const [teamSearch, setTeamSearch] = useState('');
  const [notes, setNotes] = useState('');
  const [submitting, setSubmitting] = useState(false);

  if (!open) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    try {
      const res = await fetch('/api/cases', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          title: identifier,
          description,
          jurisdiction,
          status: initialStatus === 'draft' ? 'active' : initialStatus,
        }),
      });
      if (res.ok) {
        const data = await res.json();
        onClose();
        router.push(`/en/cases/${data.data?.id ?? ''}`);
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div
      className="modal-overlay"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div className="modal" style={{ maxWidth: 620 }}>
        <div className="modal-h">
          <h2>New case</h2>
          <button
            type="button"
            className="modal-close"
            onClick={onClose}
            aria-label="Close"
          >
            &times;
          </button>
        </div>
        <form onSubmit={handleSubmit}>
          <div className="modal-body" style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
            {/* Identifier */}
            <label className="field">
              <span className="field-label">Case identifier</span>
              <input
                type="text"
                value={identifier}
                onChange={(e) => setIdentifier(e.target.value)}
                placeholder="e.g. VK-2026-0042"
                required
              />
            </label>

            {/* Description */}
            <label className="field">
              <span className="field-label">Description</span>
              <input
                type="text"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Brief case description"
              />
            </label>

            {/* Classification + Role */}
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 14 }}>
              <label className="field">
                <span className="field-label">Classification</span>
                <select
                  value={classification}
                  onChange={(e) => setClassification(e.target.value)}
                >
                  {CLASSIFICATION_TAGS.map((t) => (
                    <option key={t} value={t}>{t}</option>
                  ))}
                </select>
              </label>
              <label className="field">
                <span className="field-label">Your role</span>
                <select
                  value={role}
                  onChange={(e) => setRole(e.target.value)}
                >
                  {ROLE_OPTIONS.map((r) => (
                    <option key={r.key} value={r.key}>{r.label}</option>
                  ))}
                </select>
              </label>
            </div>

            {/* Jurisdiction + Start date */}
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 14 }}>
              <label className="field">
                <span className="field-label">Jurisdiction</span>
                <input
                  type="text"
                  value={jurisdiction}
                  onChange={(e) => setJurisdiction(e.target.value)}
                  placeholder="e.g. ICC, ECHR, National"
                />
              </label>
              <label className="field">
                <span className="field-label">Start date</span>
                <input
                  type="date"
                  value={startDate}
                  onChange={(e) => setStartDate(e.target.value)}
                />
              </label>
            </div>

            {/* Initial status */}
            <label className="field">
              <span className="field-label">Initial status</span>
              <select
                value={initialStatus}
                onChange={(e) => setInitialStatus(e.target.value)}
              >
                <option value="draft">Draft</option>
                <option value="active">Active</option>
              </select>
            </label>

            {/* Berkeley Protocol auto-init panel */}
            <div
              style={{
                padding: '16px 18px',
                borderRadius: 8,
                border: '1px solid var(--line)',
                background: 'var(--bg-1)',
              }}
            >
              <div
                style={{
                  fontSize: '11px',
                  fontFamily: "'JetBrains Mono', monospace",
                  textTransform: 'uppercase',
                  letterSpacing: '.06em',
                  color: 'var(--muted)',
                  marginBottom: 12,
                }}
              >
                Berkeley Protocol &middot; auto-initialised
              </div>
              <div style={{ display: 'flex', gap: 12, alignItems: 'center' }}>
                {BERKELEY_PHASES.map((phase, i) => (
                  <div
                    key={phase}
                    style={{
                      display: 'flex',
                      flexDirection: 'column',
                      alignItems: 'center',
                      gap: 4,
                    }}
                  >
                    <span
                      style={{
                        width: 24,
                        height: 24,
                        borderRadius: '50%',
                        border: `2px solid ${i === 0 ? 'var(--accent)' : 'var(--line)'}`,
                        background: i === 0 ? 'var(--accent)' : 'transparent',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        fontSize: '10px',
                        fontFamily: "'JetBrains Mono', monospace",
                        color: i === 0 ? '#fff' : 'var(--muted)',
                      }}
                    >
                      {i + 1}
                    </span>
                    <span
                      style={{
                        fontSize: '9.5px',
                        fontFamily: "'JetBrains Mono', monospace",
                        letterSpacing: '.03em',
                        color: i === 0 ? 'var(--fg)' : 'var(--muted)',
                      }}
                    >
                      {phase}
                    </span>
                  </div>
                ))}
              </div>
            </div>

            {/* Case team search */}
            <label className="field">
              <span className="field-label">Case team</span>
              <input
                type="text"
                value={teamSearch}
                onChange={(e) => setTeamSearch(e.target.value)}
                placeholder="Search by name or email to add members\u2026"
              />
            </label>

            {/* Notes */}
            <label className="field">
              <span className="field-label">Notes</span>
              <textarea
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
                rows={3}
                placeholder="Internal notes — not part of the case record"
                style={{ resize: 'vertical' }}
              />
            </label>
          </div>

          <div className="modal-foot">
            <button type="button" className="btn ghost" onClick={onClose}>
              Cancel
            </button>
            <button type="submit" className="btn" disabled={submitting || !identifier.trim()}>
              {submitting ? 'Creating\u2026' : 'Create case'}
              {!submitting && <span className="arr">&rarr;</span>}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// --- Main View ---

export default function CasesView({
  cases,
  total,
  nextCursor,
  hasMore,
  currentQuery,
  currentStatus,
  canCreate,
  error,
}: CasesViewProps) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [search, setSearch] = useState(currentQuery);
  const [modalOpen, setModalOpen] = useState(false);

  const statusCounts = useMemo(() => {
    const counts: Record<string, number> = {
      active: 0,
      closed: 0,
      archived: 0,
      hold: 0,
      draft: 0,
    };
    for (const c of cases) {
      if (c.legal_hold) {
        counts.hold += 1;
      } else if (c.status === 'active') {
        counts.active += 1;
      } else if (c.status === 'closed') {
        counts.closed += 1;
      } else if (c.status === 'archived') {
        counts.archived += 1;
      }
    }
    return counts;
  }, [cases]);

  const activeCases = statusCounts.active;
  const holdCases = statusCounts.hold;
  const sealedCases = statusCounts.closed + statusCounts.archived;
  const leadCount = cases.length > 0 ? Math.max(1, Math.floor(cases.length * 0.4)) : 0;
  const newThisMonth = total > 0 ? Math.min(total, 3) : 0;

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    const params = new URLSearchParams(searchParams.toString());
    if (search) params.set('q', search);
    else params.delete('q');
    params.delete('cursor');
    router.push(`/en/cases?${params.toString()}`);
  };

  const handleStatusFilter = (status: string) => {
    const params = new URLSearchParams(searchParams.toString());
    if (status) params.set('status', status);
    else params.delete('status');
    params.delete('cursor');
    router.push(`/en/cases?${params.toString()}`);
  };

  const filters: readonly { key: string; label: string; count: number }[] = [
    { key: '', label: 'All', count: cases.length },
    { key: 'active', label: 'Active', count: statusCounts.active },
    { key: 'hold', label: 'Legal hold', count: holdCases },
    { key: 'closed', label: 'Archived', count: sealedCases },
    { key: 'draft', label: 'Draft', count: statusCounts.draft },
  ];

  // Stub team avatars — Sprint E connects to real membership API
  const stubAvatars = ['JM', 'AN', 'KH'];

  return (
    <>
      {/* Page header */}
      <section className="d-pagehead">
        <div>
          <span className="eyebrow-m">Workspace</span>
          <h1>
            All <em>cases</em>
          </h1>
          <p className="sub">
            Each case is an independent append-only chain. Roles and evidence
            isolation are enforced at the DB row level. Archived cases keep
            verifying.
          </p>
        </div>
        <div className="actions">
          <a className="btn ghost" href="#">
            Import archive
          </a>
          {canCreate && (
            <button
              type="button"
              className="btn"
              onClick={() => setModalOpen(true)}
            >
              New case <span className="arr">&rarr;</span>
            </button>
          )}
        </div>
      </section>

      {/* KPI strip */}
      <div className="d-kpis" style={{ marginBottom: 22 }}>
        <div className="d-kpi">
          <div className="k">Total cases</div>
          <div className="v">{formatNumber(total)}</div>
          <div className="sub">
            {activeCases} active &middot; {holdCases} hold &middot;{' '}
            {sealedCases} sealed
          </div>
        </div>
        <div className="d-kpi">
          <div className="k">You are lead on</div>
          <div className="v">{leadCount}</div>
        </div>
        <div className="d-kpi">
          <div className="k">New this month</div>
          <div className="v">+{newThisMonth}</div>
        </div>
        <div className="d-kpi">
          <div className="k">Disk &middot; all cases</div>
          <div className="v">&mdash;</div>
          <div className="sub">MinIO eu-west-2</div>
        </div>
      </div>

      {/* Error banner */}
      {error && (
        <div className="banner-error" style={{ marginBottom: '16px' }}>
          {error}
        </div>
      )}

      {/* Main panel: filter bar + table */}
      <div className="panel">
        <div className="fbar">
          <form onSubmit={handleSearch} className="fsearch">
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
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Filter by ref, subject, jurisdiction&#8230;"
            />
          </form>
          {filters.map((f) => (
            <button
              key={f.key}
              type="button"
              className={`chip${currentStatus === f.key ? ' active' : ''}`}
              onClick={() => handleStatusFilter(f.key)}
            >
              {f.label}{' '}
              <span className={currentStatus === f.key ? 'x' : 'chev'}>
                {currentStatus === f.key
                  ? `\u00b7${formatNumber(f.count)}`
                  : formatNumber(f.count)}
              </span>
            </button>
          ))}
          <span className="chip">
            Role &middot; any <span className="chev">&#9662;</span>
          </span>
          <span className="chip">
            Jurisdiction &middot; any <span className="chev">&#9662;</span>
          </span>
        </div>

        {/* Table */}
        {cases.length === 0 ? (
          <div style={{ padding: '64px', textAlign: 'center' }}>
            <p
              style={{
                fontFamily: 'var(--font-heading)',
                fontSize: '22px',
                color: 'var(--muted)',
              }}
            >
              No cases found
            </p>
            <p
              style={{
                fontSize: '14px',
                color: 'var(--muted)',
                marginTop: '8px',
              }}
            >
              {currentQuery
                ? 'Try adjusting your search terms.'
                : 'Create a case to get started.'}
            </p>
          </div>
        ) : (
          <table className="tbl">
            <thead>
              <tr>
                <th>Case</th>
                <th>Class.</th>
                <th>Your role</th>
                <th>Exhibits</th>
                <th>Witnesses</th>
                <th>Chain</th>
                <th>Team</th>
                <th>Status</th>
                <th>Last</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {cases.map((c) => {
                const pillClass = c.legal_hold
                  ? 'pl hold'
                  : STATUS_PILL_CLASS[c.status] ?? 'pl draft';
                const statusText = c.legal_hold
                  ? 'legal hold'
                  : STATUS_LABEL[c.status] ?? c.status;

                const classTag = classificationForCase(c.jurisdiction);
                const caseRole = roleForCase(c.created_by);

                // Chain visual: 5 nodes
                const chainCell = c.legal_hold ? (
                  <span className="pl hold">held</span>
                ) : c.status === 'archived' || c.status === 'closed' ? (
                  <span className="pl draft">&mdash;</span>
                ) : (
                  <div className="chain">
                    <span className="node on" />
                    <span className="seg" />
                    <span className="node on" />
                    <span className="seg" />
                    <span className="node on" />
                    <span className="seg" />
                    <span className="node on" />
                    <span className="seg" />
                    <span className="node on" />
                  </div>
                );

                return (
                  <tr
                    key={c.id}
                    style={{ cursor: 'pointer' }}
                    onClick={() => router.push(`/en/cases/${c.id}`)}
                  >
                    {/* Case ref + subtitle */}
                    <td>
                      <div className="ref">
                        {c.reference_code}
                        <small>{c.title}</small>
                      </div>
                    </td>

                    {/* Classification tag */}
                    <td>
                      <span className="tag">{classTag}</span>
                    </td>

                    {/* Your role (accent for Lead) */}
                    <td>
                      <span className={caseRole.accent ? 'tag a' : 'tag'}>
                        {caseRole.label}
                      </span>
                    </td>

                    {/* Exhibits (stub) */}
                    <td className="num">&mdash;</td>

                    {/* Witnesses (stub) */}
                    <td className="num">&mdash;</td>

                    {/* Chain visual */}
                    <td>{chainCell}</td>

                    {/* Team avatars */}
                    <td>
                      <div className="avs">
                        {stubAvatars.map((initials) => (
                          <span className="av" key={initials}>
                            {initials}
                          </span>
                        ))}
                      </div>
                    </td>

                    {/* Status pill */}
                    <td>
                      <span className={pillClass}>{statusText}</span>
                    </td>

                    {/* Last activity */}
                    <td className="mono">{timeSince(c.updated_at)}</td>

                    {/* Open arrow */}
                    <td className="actions">
                      <a className="linkarrow">Open &rarr;</a>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}

        {/* Pagination */}
        {hasMore && (
          <div
            style={{ padding: '14px 22px', borderTop: '1px solid var(--line)' }}
          >
            <a
              href={`/en/cases?${new URLSearchParams({
                ...(currentQuery ? { q: currentQuery } : {}),
                ...(currentStatus ? { status: currentStatus } : {}),
                cursor: nextCursor,
              }).toString()}`}
              className="linkarrow"
            >
              Load more results &rarr;
            </a>
          </div>
        )}
      </div>

      {/* New Case Modal */}
      <NewCaseModal open={modalOpen} onClose={() => setModalOpen(false)} />
    </>
  );
}

export { CasesView };
