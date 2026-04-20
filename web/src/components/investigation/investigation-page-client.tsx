'use client';

import { useCallback, useEffect, useState } from 'react';
import type {
  InquiryLog,
  EvidenceAssessment,
  VerificationRecord,
  CorroborationClaim,
  AnalysisNote,
  InvestigationTemplate,
  TemplateInstance,
  InvestigationReport,
  SafetyProfile,
} from '@/types';
import { InquiryLogForm } from '@/components/investigation/inquiry-log-form';
import { AnalysisNoteEditor } from '@/components/investigation/analysis-note-editor';
import { ReportBuilder } from '@/components/investigation/report-builder';
import { SafetyProfileForm } from '@/components/investigation/safety-profile-form';
import { CorroborationBuilder } from '@/components/investigation/corroboration-builder';
import { TemplateEditor } from '@/components/investigation/template-editor';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

type SectionKey = 'inquiry-logs' | 'assessments' | 'verifications' | 'corroborations' | 'analysis' | 'templates' | 'reports' | 'safety';

interface InvestigationPageClientProps {
  readonly caseId: string;
  readonly accessToken: string;
  readonly userId: string;
  readonly activeSection: string;
  readonly onCountsLoaded?: (counts: Record<string, number>) => void;
  readonly inquiryLogs: readonly InquiryLog[];
  readonly assessments: readonly EvidenceAssessment[];
  readonly verifications: readonly VerificationRecord[];
  readonly corroborations: readonly CorroborationClaim[];
  readonly analysisNotes: readonly AnalysisNote[];
  readonly templates: readonly InvestigationTemplate[];
  readonly templateInstances: readonly TemplateInstance[];
  readonly reports: readonly InvestigationReport[];
  readonly safetyProfiles: readonly SafetyProfile[];
}

export function InvestigationPageClient({
  caseId,
  accessToken,
  userId,
  activeSection,
  onCountsLoaded,
  inquiryLogs,
  assessments,
  verifications,
  corroborations,
  analysisNotes,
  templates,
  templateInstances,
  reports,
  safetyProfiles,
}: InvestigationPageClientProps) {
  const activeTab = (activeSection as SectionKey) || 'inquiry-logs';

  // Live data state
  const [liveInquiryLogs, setLiveInquiryLogs] = useState(inquiryLogs);
  const [liveAssessments, setLiveAssessments] = useState(assessments);
  const [liveVerifications, setLiveVerifications] = useState(verifications);
  const [liveCorroborations, setLiveCorroborations] = useState(corroborations);
  const [liveAnalysisNotes, setLiveAnalysisNotes] = useState(analysisNotes);
  const [liveTemplates, setLiveTemplates] = useState(templates);
  const [liveTemplateInstances, setLiveTemplateInstances] = useState(templateInstances);
  const [liveReports, setLiveReports] = useState(reports);
  const [liveSafetyProfiles, setLiveSafetyProfiles] = useState(safetyProfiles);

  // Loading, error & search state
  const [loading, setLoading] = useState(true);
  const [fetchError, setFetchError] = useState<string | null>(null);
  const [searchQuery, _setSearchQuery] = useState('');

  const fetchJSON = useCallback(
    (url: string) => fetch(url, { headers: { Authorization: `Bearer ${accessToken}` } }).then(r => r.ok ? r.json() : null),
    [accessToken],
  );

  // Refresh functions — replace all window.location.reload()
  const refreshInquiryLogs = useCallback(async () => {
    const json = await fetchJSON(`${API_BASE}/api/cases/${caseId}/inquiry-logs`);
    if (json?.data) setLiveInquiryLogs(json.data);
  }, [caseId, fetchJSON]);

  const refreshCorroborations = useCallback(async () => {
    const json = await fetchJSON(`${API_BASE}/api/cases/${caseId}/corroborations`);
    if (json?.data) setLiveCorroborations(json.data);
  }, [caseId, fetchJSON]);

  const refreshAnalysisNotes = useCallback(async () => {
    const json = await fetchJSON(`${API_BASE}/api/cases/${caseId}/analysis-notes`);
    if (json?.data) setLiveAnalysisNotes(json.data);
  }, [caseId, fetchJSON]);

  const refreshTemplates = useCallback(async () => {
    const json = await fetchJSON(`${API_BASE}/api/templates`);
    if (json?.data) setLiveTemplates(json.data);
    else if (Array.isArray(json)) setLiveTemplates(json);
  }, [fetchJSON]);

  const refreshTemplateInstances = useCallback(async () => {
    const json = await fetchJSON(`${API_BASE}/api/cases/${caseId}/template-instances`);
    if (json?.data) setLiveTemplateInstances(json.data);
  }, [caseId, fetchJSON]);

  const refreshReports = useCallback(async () => {
    const json = await fetchJSON(`${API_BASE}/api/cases/${caseId}/reports`);
    if (json?.data) setLiveReports(json.data);
  }, [caseId, fetchJSON]);

  const refreshSafetyProfiles = useCallback(async () => {
    const json = await fetchJSON(`${API_BASE}/api/cases/${caseId}/safety-profiles`);
    if (json?.data) setLiveSafetyProfiles(json.data);
  }, [caseId, fetchJSON]);

  // Initial data fetch
  const loadAll = useCallback(async () => {
    setLoading(true);
    setFetchError(null);
    try {
      const [inq, assess, verify, cor, ana, tpl, inst, rep, saf] = await Promise.all([
        fetchJSON(`${API_BASE}/api/cases/${caseId}/inquiry-logs`),
        fetchJSON(`${API_BASE}/api/cases/${caseId}/assessments`),
        fetchJSON(`${API_BASE}/api/cases/${caseId}/verifications`),
        fetchJSON(`${API_BASE}/api/cases/${caseId}/corroborations`),
        fetchJSON(`${API_BASE}/api/cases/${caseId}/analysis-notes`),
        fetchJSON(`${API_BASE}/api/templates`),
        fetchJSON(`${API_BASE}/api/cases/${caseId}/template-instances`),
        fetchJSON(`${API_BASE}/api/cases/${caseId}/reports`),
        fetchJSON(`${API_BASE}/api/cases/${caseId}/safety-profiles`),
      ]);
      if (inq?.data) setLiveInquiryLogs(inq.data);
      if (assess?.data) setLiveAssessments(assess.data);
      else if (Array.isArray(assess)) setLiveAssessments(assess);
      if (verify?.data) setLiveVerifications(verify.data);
      else if (Array.isArray(verify)) setLiveVerifications(verify);
      if (cor?.data) setLiveCorroborations(cor.data);
      if (ana?.data) setLiveAnalysisNotes(ana.data);
      if (tpl?.data) setLiveTemplates(tpl.data);
      else if (Array.isArray(tpl)) setLiveTemplates(tpl);
      if (inst?.data) setLiveTemplateInstances(inst.data);
      if (rep?.data) setLiveReports(rep.data);
      if (saf?.data) setLiveSafetyProfiles(saf.data);
    } catch {
      setFetchError('Failed to load investigation data.');
    } finally {
      setLoading(false);
    }
  }, [caseId, fetchJSON]);

  useEffect(() => {
    if (!accessToken) return;
    loadAll();
  }, [accessToken, loadAll]);

  // Report counts to parent sidebar
  useEffect(() => {
    if (loading || !onCountsLoaded) return;
    onCountsLoaded({
      'inquiry-logs': liveInquiryLogs.length,
      assessments: liveAssessments.length,
      verifications: liveVerifications.length,
      corroborations: liveCorroborations.length,
      analysis: liveAnalysisNotes.length,
      safety: liveSafetyProfiles.length,
      templates: liveTemplateInstances.length,
      reports: liveReports.length,
    });
  }, [
    loading, onCountsLoaded,
    liveInquiryLogs.length, liveAssessments.length, liveVerifications.length,
    liveCorroborations.length, liveAnalysisNotes.length, liveSafetyProfiles.length,
    liveTemplateInstances.length, liveReports.length,
  ]);

  return (
    <div>
      {/* Error banner */}
      {fetchError && (
        <div className="banner-error mb-[var(--space-md)] flex items-center justify-between">
          <span>{fetchError}</span>
          <button onClick={loadAll} className="btn-ghost text-sm">Retry</button>
        </div>
      )}

      {/* Loading skeleton */}
      {loading && (
        <div className="space-y-[var(--space-sm)]">
          <div className="skeleton h-8 w-2/3 rounded" />
          <div className="skeleton h-24 w-full rounded" />
          <div className="skeleton h-24 w-full rounded" />
        </div>
      )}

      {/* Section content — rendered directly, no sub-tabs */}
      {!loading && (
        <>
          {activeTab === 'inquiry-logs' && (
            <InquiryLogsSection caseId={caseId} accessToken={accessToken} items={filterBySearch(liveInquiryLogs, searchQuery, ['objective', 'search_tool', 'search_strategy', 'notes'])} onRefresh={refreshInquiryLogs} />
          )}
          {activeTab === 'assessments' && (
            <AssessmentsSection caseId={caseId} accessToken={accessToken} items={filterBySearch(liveAssessments, searchQuery, ['source_credibility', 'recommendation', 'relevance_rationale'])} />
          )}
          {activeTab === 'verifications' && (
            <VerificationsSection caseId={caseId} accessToken={accessToken} items={filterBySearch(liveVerifications, searchQuery, ['verification_type', 'finding', 'methodology'])} />
          )}
          {activeTab === 'corroborations' && (
            <CorroborationsSection caseId={caseId} accessToken={accessToken} items={filterBySearch(liveCorroborations, searchQuery, ['claim_summary', 'claim_type', 'strength'])} onRefresh={refreshCorroborations} />
          )}
          {activeTab === 'analysis' && (
            <AnalysisSection caseId={caseId} accessToken={accessToken} items={filterBySearch(liveAnalysisNotes, searchQuery, ['title', 'content', 'analysis_type', 'methodology'])} onRefresh={refreshAnalysisNotes} />
          )}
          {activeTab === 'templates' && (
            <TemplatesSection
              caseId={caseId}
              accessToken={accessToken}
              templates={liveTemplates}
              instances={liveTemplateInstances}
              onRefreshInstances={refreshTemplateInstances}
              onRefreshTemplates={refreshTemplates}
            />
          )}
          {activeTab === 'reports' && (
            <ReportsSection caseId={caseId} accessToken={accessToken} items={filterBySearch(liveReports, searchQuery, ['title', 'report_type', 'status'])} onRefresh={refreshReports} />
          )}
          {activeTab === 'safety' && (
            <SafetySection caseId={caseId} accessToken={accessToken} userId={userId} items={liveSafetyProfiles} onRefresh={refreshSafetyProfiles} />
          )}
        </>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Section components                                                 */
/* ------------------------------------------------------------------ */

function SectionHeader({
  title,
  count,
  onNew,
  newLabel,
  sortOrder,
  onToggleSort,
  filterOptions,
  filterValue,
  onFilter,
}: {
  readonly title: string;
  readonly count: number;
  readonly onNew: () => void;
  readonly newLabel: string;
  readonly sortOrder?: 'newest' | 'oldest';
  readonly onToggleSort?: () => void;
  readonly filterOptions?: readonly { value: string; label: string }[];
  readonly filterValue?: string;
  readonly onFilter?: (value: string) => void;
}) {
  return (
    <div className="flex flex-col gap-[var(--space-sm)] mb-[var(--space-md)]">
      <div className="flex items-end justify-between">
        <div>
          <h2
            className="font-[family-name:var(--font-heading)] text-lg"
            style={{ color: 'var(--text-primary)' }}
          >
            {title}
          </h2>
          <p className="text-sm mt-[var(--space-2xs)]" style={{ color: 'var(--text-tertiary)' }}>
            {count} {count === 1 ? 'record' : 'records'}
          </p>
        </div>
        <div className="flex items-center gap-[var(--space-sm)]">
          {onToggleSort && (
            <button onClick={onToggleSort} className="btn-ghost text-xs" type="button">
              {sortOrder === 'oldest' ? 'Oldest first' : 'Newest first'}
            </button>
          )}
          <button onClick={onNew} className="btn-primary text-sm">
            {newLabel}
          </button>
        </div>
      </div>
      {filterOptions && filterOptions.length > 0 && onFilter && (
        <div className="flex gap-[var(--space-xs)]">
          <select
            className="input-field text-xs py-1"
            value={filterValue || ''}
            onChange={(e) => onFilter(e.target.value)}
            style={{ maxWidth: '12rem' }}
          >
            <option value="">All</option>
            {filterOptions.map((opt) => (
              <option key={opt.value} value={opt.value}>{opt.label}</option>
            ))}
          </select>
        </div>
      )}
    </div>
  );
}

const EMPTY_ICONS: Record<string, string> = {
  search: '\u{1F50D}',
  file: '\u{1F4CB}',
  shield: '\u{1F6E1}',
  chart: '\u{1F4CA}',
};

function EmptyState({ message, actionLabel, onAction, icon }: {
  readonly message: string;
  readonly actionLabel?: string;
  readonly onAction?: () => void;
  readonly icon?: 'search' | 'file' | 'shield' | 'chart';
}) {
  return (
    <div
      className="rounded-md border py-[var(--space-xl)] text-center text-sm"
      style={{ borderColor: 'var(--border-default)', color: 'var(--text-tertiary)' }}
    >
      {icon && <p className="text-2xl mb-[var(--space-sm)]">{EMPTY_ICONS[icon]}</p>}
      <p>{message}</p>
      {actionLabel && onAction && (
        <button onClick={onAction} className="btn-secondary text-sm mt-[var(--space-md)]">
          {actionLabel}
        </button>
      )}
    </div>
  );
}

// eslint-disable-next-line @typescript-eslint/no-unused-vars
function SearchInput({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  return (
    <div className="mb-[var(--space-sm)]">
      <input
        type="text"
        className="input-field w-full"
        placeholder="Search investigation records..."
        value={value}
        onChange={(e) => onChange(e.target.value)}
      />
    </div>
  );
}

function filterBySearch<T>(items: readonly T[], query: string, fields: (keyof T)[]): readonly T[] {
  if (!query.trim()) return items;
  const lower = query.toLowerCase();
  return items.filter(item =>
    fields.some(f => {
      const val = item[f];
      return typeof val === 'string' && val.toLowerCase().includes(lower);
    }),
  );
}

function StatusBadge({ status }: { readonly status: string }) {
  const colors: Record<string, { color: string; bg: string }> = {
    draft: { color: 'var(--status-archived)', bg: 'var(--status-archived-bg)' },
    in_review: { color: 'var(--amber-accent)', bg: 'var(--amber-subtle)' },
    approved: { color: 'var(--status-active)', bg: 'var(--status-active-bg)' },
    published: { color: 'var(--status-active)', bg: 'var(--status-active-bg)' },
    withdrawn: { color: 'var(--status-hold)', bg: 'var(--status-hold-bg)' },
    superseded: { color: 'var(--status-hold)', bg: 'var(--status-hold-bg)' },
    active: { color: 'var(--status-active)', bg: 'var(--status-active-bg)' },
    completed: { color: 'var(--status-active)', bg: 'var(--status-active-bg)' },
  };
  const style = colors[status] || colors.draft;
  return (
    <span className="badge" style={{ backgroundColor: style.bg, color: style.color }}>
      {status.replace(/_/g, ' ')}
    </span>
  );
}

/* --- Inquiry Log Row (links to detail page) --- */

function InquiryLogRow({ log }: { readonly log: InquiryLog }) {
  const startDate = new Date(log.search_started_at);
  const endDate = log.search_ended_at ? new Date(log.search_ended_at) : null;
  const durationMs = endDate ? endDate.getTime() - startDate.getTime() : null;
  const durationMin = durationMs != null ? Math.round(durationMs / 60000) : null;

  return (
    <a
      href={`/en/inquiry-logs/${log.id}`}
      className="rounded-md border phase-inquiry p-[var(--space-md)] flex items-start justify-between gap-[var(--space-md)] transition-colors hover:bg-[var(--bg-inset)]"
      style={{ borderColor: 'var(--border-default)', background: 'var(--bg-elevated)', textDecoration: 'none', display: 'flex' }}
    >
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
          {log.objective}
        </p>
        <div className="flex items-center gap-[var(--space-sm)] mt-[var(--space-xs)] flex-wrap">
          <span
            className="inline-flex items-center text-xs px-[var(--space-xs)] py-[1px] rounded-[var(--radius-sm)]"
            style={{ backgroundColor: 'var(--bg-inset)', color: 'var(--text-secondary)', border: '1px solid var(--border-subtle)' }}
          >
            {log.search_tool}
          </span>
          <span className="text-xs" style={{ color: 'var(--text-tertiary)' }}>
            {startDate.toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' })}
          </span>
          {durationMin != null && (
            <>
              <span className="text-xs" style={{ color: 'var(--border-default)' }}>&middot;</span>
              <span className="text-xs" style={{ color: 'var(--text-tertiary)' }}>
                {durationMin < 60 ? `${durationMin}m` : `${Math.floor(durationMin / 60)}h ${durationMin % 60}m`}
              </span>
            </>
          )}
        </div>
      </div>
      <div className="flex items-center gap-[var(--space-sm)] shrink-0 pt-[2px]">
        {log.results_count != null && (
          <span
            className="text-xs font-[family-name:var(--font-mono)] tabular-nums"
            style={{ color: 'var(--amber-accent)' }}
          >
            {log.results_relevant ?? 0}/{log.results_count}
          </span>
        )}
        <span className="text-xs" style={{ color: 'var(--text-tertiary)' }}>&rarr;</span>
      </div>
    </a>
  );
}

/* --- Evidence-linked section header (no inline create) --- */

function ReadOnlyHeader({ title, count }: {
  readonly title: string;
  readonly count: number;
}) {
  return (
    <div className="mb-[var(--space-md)]">
      <h2
        className="font-[family-name:var(--font-heading)] text-lg"
        style={{ color: 'var(--text-primary)' }}
      >
        {title}
      </h2>
      <p className="text-sm mt-[var(--space-2xs)]" style={{ color: 'var(--text-tertiary)' }}>
        {count} {count === 1 ? 'record' : 'records'}
      </p>
    </div>
  );
}

/* --- Generic Record Row (links to detail page) --- */

function RecordRow({ href, title, subtitle, badge, phase }: {
  readonly href: string;
  readonly title: string;
  readonly subtitle: string;
  readonly badge?: string;
  readonly phase?: string;
}) {
  return (
    <a
      href={href}
      className={`rounded-md border p-[var(--space-md)] flex items-start justify-between gap-[var(--space-md)] transition-colors hover:bg-[var(--bg-inset)] ${phase || ''}`}
      style={{ borderColor: 'var(--border-default)', background: 'var(--bg-elevated)', textDecoration: 'none', display: 'flex' }}
    >
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
          {title}
        </p>
        <p className="text-xs mt-[var(--space-xs)]" style={{ color: 'var(--text-tertiary)' }}>
          {subtitle}
        </p>
      </div>
      <div className="flex items-center gap-[var(--space-sm)] shrink-0 pt-[2px]">
        {badge && (
          <span className="text-xs font-[family-name:var(--font-mono)] tabular-nums" style={{ color: 'var(--text-tertiary)' }}>
            {badge}
          </span>
        )}
        <span className="text-xs" style={{ color: 'var(--text-tertiary)' }}>&rarr;</span>
      </div>
    </a>
  );
}

/* --- Inquiry Logs --- */

function InquiryLogsSection({
  caseId,
  accessToken,
  items,
  onRefresh,
}: {
  readonly caseId: string;
  readonly accessToken: string;
  readonly items: readonly InquiryLog[];
  readonly onRefresh: () => Promise<void>;
}) {
  const [showForm, setShowForm] = useState(false);

  return (
    <div>
      <SectionHeader
        title="Inquiry Logs"
        count={items.length}
        onNew={() => setShowForm(true)}
        newLabel="New Inquiry Log"
      />
      {showForm && (
        <div className="mb-[var(--space-md)]">
          <InquiryLogForm caseId={caseId} accessToken={accessToken} onSaved={() => { setShowForm(false); onRefresh(); }} />
          <button onClick={() => setShowForm(false)} className="btn-ghost text-sm mt-[var(--space-sm)]">Cancel</button>
        </div>
      )}
      {items.length === 0 && !showForm ? (
        <EmptyState message="No inquiry logs recorded yet." actionLabel="Create First Log" onAction={() => setShowForm(true)} />
      ) : (
        <div className="flex flex-col gap-[var(--space-sm)]">
          {items.map((log) => (
            <InquiryLogRow key={log.id} log={log} />
          ))}
        </div>
      )}
    </div>
  );
}

/* --- Assessments --- */

function AssessmentsSection({
  caseId: _caseId,
  accessToken: _accessToken,
  items,
}: {
  readonly caseId: string;
  readonly accessToken: string;
  readonly items: readonly EvidenceAssessment[];
}) {
  return (
    <div>
      <ReadOnlyHeader title="Assessments" count={items.length} />
      {items.length === 0 ? (
        <EmptyState message="No evidence assessments recorded yet." />
      ) : (
        <div className="flex flex-col gap-[var(--space-sm)]">
          {items.map((a) => (
            <RecordRow
              key={a.id}
              href={`/en/assessments/${a.id}`}
              title={`Relevance ${a.relevance_score}/10 \u00b7 Reliability ${a.reliability_score}/10`}
              subtitle={`${a.source_credibility} \u00b7 ${a.recommendation}`}
              badge={new Date(a.created_at).toLocaleDateString()}
              phase="phase-assessment"
            />
          ))}
        </div>
      )}
    </div>
  );
}

/* --- Verifications --- */

function VerificationsSection({
  caseId: _caseId,
  accessToken: _accessToken,
  items,
}: {
  readonly caseId: string;
  readonly accessToken: string;
  readonly items: readonly VerificationRecord[];
}) {
  return (
    <div>
      <ReadOnlyHeader title="Verifications" count={items.length} />
      {items.length === 0 ? (
        <EmptyState message="No verification records yet." />
      ) : (
        <div className="flex flex-col gap-[var(--space-sm)]">
          {items.map((v) => (
            <RecordRow
              key={v.id}
              href={`/en/verifications/${v.id}`}
              title={v.verification_type.replace(/_/g, ' ')}
              subtitle={`${v.finding.replace(/_/g, ' ')} \u00b7 ${v.confidence_level} confidence`}
              badge={new Date(v.created_at).toLocaleDateString()}
              phase="phase-verification"
            />
          ))}
        </div>
      )}
    </div>
  );
}

/* --- Corroborations --- */

function CorroborationsSection({
  caseId,
  accessToken,
  items,
  onRefresh,
}: {
  readonly caseId: string;
  readonly accessToken: string;
  readonly items: readonly CorroborationClaim[];
  readonly onRefresh: () => Promise<void>;
}) {
  const [showForm, setShowForm] = useState(false);
  const [evidenceItems, setEvidenceItems] = useState<{ id: string; evidence_number: string; title: string }[]>([]);

  useEffect(() => {
    if (!showForm) return;
    fetch(`${API_BASE}/api/cases/${caseId}/evidence?current_only=true`, {
      headers: { Authorization: `Bearer ${accessToken}` },
    })
      .then(r => r.ok ? r.json() : null)
      .then(json => {
        const items = (json?.data || []).map((e: { id: string; evidence_number: string; title?: string; original_name: string }) => ({
          id: e.id,
          evidence_number: e.evidence_number,
          title: e.title || e.original_name,
        }));
        setEvidenceItems(items);
      })
      .catch(() => {});
  }, [showForm, caseId, accessToken]);

  return (
    <div>
      <SectionHeader
        title="Corroborations"
        count={items.length}
        onNew={() => setShowForm(true)}
        newLabel="New Claim"
      />
      {showForm && (
        <div className="mb-[var(--space-md)]">
          <CorroborationBuilder
            caseId={caseId}
            evidenceItems={evidenceItems}
            accessToken={accessToken}
            onSaved={() => { setShowForm(false); onRefresh(); }}
          />
          <button onClick={() => setShowForm(false)} className="btn-ghost text-sm mt-[var(--space-sm)]">Cancel</button>
        </div>
      )}
      {items.length === 0 && !showForm ? (
        <EmptyState message="No corroboration claims recorded yet." actionLabel="Create First Claim" onAction={() => setShowForm(true)} />
      ) : (
        <div className="flex flex-col gap-[var(--space-sm)]">
          {items.map((c) => (
            <RecordRow
              key={c.id}
              href={`/en/corroborations/${c.id}`}
              title={c.claim_summary}
              subtitle={`${c.claim_type.replace(/_/g, ' ')} \u00b7 ${c.strength} \u00b7 ${c.evidence.length} evidence`}
              badge={new Date(c.created_at).toLocaleDateString()}
              phase="phase-corroboration"
            />
          ))}
        </div>
      )}
    </div>
  );
}

/* --- Analysis --- */

function AnalysisSection({
  caseId,
  accessToken,
  items,
  onRefresh,
}: {
  readonly caseId: string;
  readonly accessToken: string;
  readonly items: readonly AnalysisNote[];
  readonly onRefresh: () => Promise<void>;
}) {
  const [showForm, setShowForm] = useState(false);

  return (
    <div>
      <SectionHeader
        title="Analysis Notes"
        count={items.length}
        onNew={() => setShowForm(true)}
        newLabel="New Note"
      />
      {showForm && (
        <div className="mb-[var(--space-md)]">
          <AnalysisNoteEditor caseId={caseId} accessToken={accessToken} onSaved={() => { setShowForm(false); onRefresh(); }} />
          <button onClick={() => setShowForm(false)} className="btn-ghost text-sm mt-[var(--space-sm)]">Cancel</button>
        </div>
      )}
      {items.length === 0 && !showForm ? (
        <EmptyState message="No analysis notes recorded yet." actionLabel="Create First Note" onAction={() => setShowForm(true)} />
      ) : (
        <div className="flex flex-col gap-[var(--space-sm)]">
          {items.map((n) => (
            <RecordRow
              key={n.id}
              href={`/en/analysis-notes/${n.id}`}
              title={n.title}
              subtitle={`${n.analysis_type.replace(/_/g, ' ')} \u00b7 ${n.status}`}
              badge={new Date(n.created_at).toLocaleDateString()}
              phase="phase-analysis"
            />
          ))}
        </div>
      )}
    </div>
  );
}

/* --- Templates --- */

function TemplatesSection({
  caseId,
  accessToken,
  templates,
  instances,
  onRefreshInstances,
  onRefreshTemplates: _onRefreshTemplates,
}: {
  readonly caseId: string;
  readonly accessToken: string;
  readonly templates: readonly InvestigationTemplate[];
  readonly instances: readonly TemplateInstance[];
  readonly onRefreshInstances: () => Promise<void>;
  readonly onRefreshTemplates: () => Promise<void>;
}) {
  const [selectedTemplateId, setSelectedTemplateId] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);

  return (
    <div>
      <SectionHeader
        title="Templates"
        count={instances.length}
        onNew={() => setShowForm(true)}
        newLabel="Fill Template"
      />

      {/* Template picker cards */}
      {(showForm || selectedTemplateId) && templates.length > 0 && !selectedTemplateId && (
        <div className="mb-[var(--space-lg)]">
          <h3 className="text-xs uppercase tracking-wider font-medium mb-[var(--space-sm)]" style={{ color: 'var(--text-tertiary)' }}>
            Select a Template
          </h3>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-[var(--space-sm)]">
            {templates.map((t) => (
              <button
                key={t.id}
                type="button"
                onClick={() => setSelectedTemplateId(t.id)}
                className="card text-left p-[var(--space-md)] hover:border-[var(--amber-accent)] transition-colors cursor-pointer"
                style={{ borderColor: 'var(--border-default)' }}
              >
                <p className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>{t.name}</p>
                <p className="text-xs mt-[var(--space-2xs)]" style={{ color: 'var(--text-tertiary)' }}>
                  {t.template_type.replace(/_/g, ' ')}
                </p>
                {t.description && (
                  <p className="text-xs mt-[var(--space-xs)] line-clamp-2" style={{ color: 'var(--text-secondary)' }}>
                    {t.description}
                  </p>
                )}
                <span className="btn-secondary text-xs mt-[var(--space-sm)] inline-block">Use Template</span>
              </button>
            ))}
          </div>
          <button onClick={() => { setShowForm(false); setSelectedTemplateId(null); }} className="btn-ghost text-sm mt-[var(--space-sm)]">Cancel</button>
        </div>
      )}

      {/* Template editor */}
      {selectedTemplateId && (
        <div className="mb-[var(--space-md)]">
          <TemplateEditor
            templateId={selectedTemplateId}
            caseId={caseId}
            accessToken={accessToken}
            onSaved={() => { setShowForm(false); setSelectedTemplateId(null); onRefreshInstances(); }}
          />
          <button onClick={() => { setShowForm(false); setSelectedTemplateId(null); }} className="btn-ghost text-sm mt-[var(--space-sm)]">Cancel</button>
        </div>
      )}

      {/* Available templates (when not in form mode) */}
      {!showForm && !selectedTemplateId && templates.length > 0 && (
        <div className="mb-[var(--space-md)]">
          <h3 className="text-xs uppercase tracking-wider font-medium mb-[var(--space-xs)]" style={{ color: 'var(--text-tertiary)' }}>
            Available Templates
          </h3>
          <div className="flex flex-col gap-[var(--space-xs)]">
            {templates.map((t) => (
              <button
                key={t.id}
                type="button"
                onClick={() => { setShowForm(true); setSelectedTemplateId(t.id); }}
                className="rounded-md border p-[var(--space-sm)] text-sm text-left hover:border-[var(--amber-accent)] transition-colors cursor-pointer"
                style={{ borderColor: 'var(--border-default)', color: 'var(--text-secondary)' }}
              >
                {t.name} &middot; {t.template_type.replace(/_/g, ' ')}
              </button>
            ))}
          </div>
        </div>
      )}

      {/* Template instances */}
      {instances.length === 0 && !showForm ? (
        <EmptyState message="No template instances created yet." actionLabel="Fill a Template" onAction={() => setShowForm(true)} />
      ) : (
        <div className="flex flex-col gap-[var(--space-sm)]">
          {instances.map((inst) => {
            const templateName = templates.find(t => t.id === inst.template_id)?.name || 'Template';
            return (
              <ExpandableCard
                key={inst.id}
                title={templateName}
                subtitle={inst.status}
                badge={new Date(inst.created_at).toLocaleDateString()}
              >
                <div className="grid grid-cols-2 gap-[var(--space-md)]">
                  <div><span className="field-label">Status</span><div className="mt-[var(--space-xs)]"><StatusBadge status={inst.status} /></div></div>
                  <DetailField label="Created" value={new Date(inst.created_at).toLocaleString()} />
                  {inst.content && Object.keys(inst.content).length > 0 && (
                    <div className="col-span-2">
                      <span className="field-label">Content Preview</span>
                      <div className="mt-[var(--space-xs)] space-y-[var(--space-xs)]">
                        {Object.entries(inst.content).slice(0, 3).map(([key, val]) => (
                          <div key={key}>
                            <span className="text-xs font-medium" style={{ color: 'var(--text-tertiary)' }}>{key}:</span>
                            <span className="text-sm ml-[var(--space-xs)]" style={{ color: 'var(--text-secondary)' }}>
                              {String(val).slice(0, 100)}{String(val).length > 100 ? '...' : ''}
                            </span>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}
                </div>
                {/* Status transition + edit buttons */}
                <div className="mt-[var(--space-md)] pt-[var(--space-sm)] flex gap-[var(--space-sm)]" style={{ borderTop: '1px solid var(--border-subtle)' }}>
                  {(inst.status === 'draft' || inst.status === 'active') && (
                    <button
                      type="button"
                      className="btn-secondary text-xs"
                      onClick={(e) => { e.stopPropagation(); setSelectedTemplateId(inst.template_id); setShowForm(true); }}
                    >
                      Edit
                    </button>
                  )}
                  {inst.status === 'draft' && (
                    <TemplateStatusButton instanceId={inst.id} accessToken={accessToken} newStatus="active" label="Activate" onSuccess={onRefreshInstances} />
                  )}
                  {inst.status === 'active' && (
                    <TemplateStatusButton instanceId={inst.id} accessToken={accessToken} newStatus="completed" label="Mark Complete" onSuccess={onRefreshInstances} />
                  )}
                </div>
              </ExpandableCard>
            );
          })}
        </div>
      )}
    </div>
  );
}

function TemplateStatusButton({ instanceId, accessToken, newStatus, label, onSuccess }: {
  instanceId: string;
  accessToken: string;
  newStatus: string;
  label: string;
  onSuccess: () => Promise<void>;
}) {
  const [loading, setLoading] = useState(false);
  const handleClick = async () => {
    setLoading(true);
    try {
      await fetch(`${API_BASE}/api/template-instances/${instanceId}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${accessToken}` },
        body: JSON.stringify({ status: newStatus, content: {} }),
      });
      await onSuccess();
    } finally {
      setLoading(false);
    }
  };
  return (
    <button type="button" className="btn-secondary text-xs" onClick={handleClick} disabled={loading}>
      {loading ? 'Updating...' : label}
    </button>
  );
}

/* --- Reports --- */

function ReportsSection({
  caseId,
  accessToken,
  items,
  onRefresh,
}: {
  readonly caseId: string;
  readonly accessToken: string;
  readonly items: readonly InvestigationReport[];
  readonly onRefresh: () => Promise<void>;
}) {
  const [showForm, setShowForm] = useState(false);
  const [editingReport, setEditingReport] = useState<InvestigationReport | null>(null);
  const [viewingReport, setViewingReport] = useState<InvestigationReport | null>(null);
  const [sortOrder, setSortOrder] = useState<'newest' | 'oldest'>('newest');
  const [statusFilter, setStatusFilter] = useState('');

  const filtered = items
    .filter(r => !statusFilter || r.status === statusFilter)
    .slice()
    .sort((a, b) => sortOrder === 'newest'
      ? new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
      : new Date(a.created_at).getTime() - new Date(b.created_at).getTime());

  if (viewingReport) {
    return <ReportPreview report={viewingReport} onClose={() => setViewingReport(null)} />;
  }

  return (
    <div>
      <SectionHeader
        title="Reports"
        count={filtered.length}
        onNew={() => { setShowForm(true); setEditingReport(null); }}
        newLabel="New Report"
        sortOrder={sortOrder}
        onToggleSort={() => setSortOrder(s => s === 'newest' ? 'oldest' : 'newest')}
        filterOptions={[
          { value: 'draft', label: 'Draft' },
          { value: 'in_review', label: 'In Review' },
          { value: 'approved', label: 'Approved' },
          { value: 'published', label: 'Published' },
          { value: 'withdrawn', label: 'Withdrawn' },
        ]}
        filterValue={statusFilter}
        onFilter={setStatusFilter}
      />
      {(showForm || editingReport) && (
        <div className="mb-[var(--space-md)]">
          <ReportBuilder caseId={caseId} accessToken={accessToken} existingReport={editingReport ?? undefined} onSaved={() => { setShowForm(false); setEditingReport(null); onRefresh(); }} />
          <button onClick={() => { setShowForm(false); setEditingReport(null); }} className="btn-ghost text-sm mt-[var(--space-sm)]">Cancel</button>
        </div>
      )}
      {filtered.length === 0 && !showForm ? (
        <EmptyState message="No investigation reports created yet." actionLabel="Create First Report" onAction={() => setShowForm(true)} />
      ) : (
        <div className="flex flex-col gap-[var(--space-sm)]">
          {filtered.map((r) => (
            <RecordRow
              key={r.id}
              href={`/en/reports/${r.id}`}
              title={r.title}
              subtitle={`${r.report_type} \u00b7 ${r.status} \u00b7 ${r.sections.length} sections`}
              badge={new Date(r.created_at).toLocaleDateString()}
              phase="phase-report"
            />
          ))}
        </div>
      )}
    </div>
  );
}

// eslint-disable-next-line @typescript-eslint/no-unused-vars
function ReportStatusButton({ reportId, accessToken, newStatus, label, variant, onSuccess, usePublishEndpoint }: {
  reportId: string;
  accessToken: string;
  newStatus: string;
  label: string;
  variant?: 'danger';
  onSuccess: () => Promise<void>;
  usePublishEndpoint?: boolean;
}) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleClick = async () => {
    setLoading(true);
    setError(null);
    try {
      const url = usePublishEndpoint
        ? `${API_BASE}/api/reports/${reportId}/publish`
        : `${API_BASE}/api/reports/${reportId}/status`;
      const res = await fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${accessToken}` },
        body: usePublishEndpoint ? undefined : JSON.stringify({ status: newStatus }),
      });
      if (!res.ok) {
        const data = await res.json().catch(() => null);
        setError(data?.error || `Failed (${res.status})`);
        return;
      }
      await onSuccess();
    } catch {
      setError('Request failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="inline-flex flex-col">
      <button
        type="button"
        className={variant === 'danger' ? 'btn-ghost text-xs' : 'btn-secondary text-xs'}
        style={variant === 'danger' ? { color: 'var(--status-hold)' } : undefined}
        onClick={handleClick}
        disabled={loading}
      >
        {loading ? 'Processing...' : label}
      </button>
      {error && <span className="text-[10px] mt-[2px]" style={{ color: 'var(--status-hold)' }}>{error}</span>}
    </div>
  );
}

function ReportPreview({ report, onClose }: { report: InvestigationReport; onClose: () => void }) {
  return (
    <div>
      <button onClick={onClose} className="link-subtle text-xs uppercase tracking-wider font-medium mb-[var(--space-md)]">
        &larr; Back to Reports
      </button>

      <h2 className="font-[family-name:var(--font-heading)] text-xl mb-[var(--space-xs)]" style={{ color: 'var(--text-primary)' }}>
        {report.title}
      </h2>
      <div className="flex items-center gap-[var(--space-sm)] mb-[var(--space-lg)]">
        <span className="text-sm" style={{ color: 'var(--text-secondary)' }}>{report.report_type} Report</span>
        <StatusBadge status={report.status} />
        <span className="text-xs" style={{ color: 'var(--text-tertiary)' }}>{new Date(report.created_at).toLocaleDateString()}</span>
      </div>

      <div className="space-y-[var(--space-lg)]">
        {report.sections.map((sec, i) => (
          <div key={i}>
            <h3 className="text-xs font-medium uppercase tracking-wider mb-[var(--space-xs)]" style={{ color: 'var(--text-tertiary)' }}>
              {i + 1}. {sec.section_type.replace(/_/g, ' ')}
            </h3>
            <p className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>{sec.title}</p>
            <p className="text-sm mt-[var(--space-xs)] whitespace-pre-wrap" style={{ color: 'var(--text-secondary)' }}>{sec.content}</p>
          </div>
        ))}
      </div>

      {(report.limitations.length > 0 || report.caveats.length > 0 || report.assumptions.length > 0) && (
        <div className="card-inset p-[var(--space-md)] mt-[var(--space-lg)]">
          <span className="field-label">Transparency</span>
          <div className="space-y-[var(--space-sm)] mt-[var(--space-sm)]">
            {report.limitations.length > 0 && (
              <div>
                <span className="text-xs font-medium" style={{ color: 'var(--text-tertiary)' }}>Limitations:</span>
                <span className="text-sm ml-[var(--space-xs)]" style={{ color: 'var(--text-secondary)' }}>{report.limitations.join(', ')}</span>
              </div>
            )}
            {report.caveats.length > 0 && (
              <div>
                <span className="text-xs font-medium" style={{ color: 'var(--text-tertiary)' }}>Caveats:</span>
                <span className="text-sm ml-[var(--space-xs)]" style={{ color: 'var(--text-secondary)' }}>{report.caveats.join(', ')}</span>
              </div>
            )}
            {report.assumptions.length > 0 && (
              <div>
                <span className="text-xs font-medium" style={{ color: 'var(--text-tertiary)' }}>Assumptions:</span>
                <span className="text-sm ml-[var(--space-xs)]" style={{ color: 'var(--text-secondary)' }}>{report.assumptions.join(', ')}</span>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

/* --- Safety --- */

function SafetySection({
  caseId,
  accessToken,
  userId,
  items,
  onRefresh,
}: {
  readonly caseId: string;
  readonly accessToken: string;
  readonly userId: string;
  readonly items: readonly SafetyProfile[];
  readonly onRefresh: () => Promise<void>;
}) {
  const [showForm, setShowForm] = useState(false);

  return (
    <div>
      <SectionHeader
        title="Safety Profiles"
        count={items.length}
        onNew={() => setShowForm(true)}
        newLabel="New Profile"
      />
      {showForm && (
        <div className="mb-[var(--space-md)]">
          <SafetyProfileForm
            caseId={caseId}
            userId={userId}
            accessToken={accessToken}
            onSaved={() => { setShowForm(false); onRefresh(); }}
          />
          <button onClick={() => setShowForm(false)} className="btn-ghost text-sm mt-[var(--space-sm)]">Cancel</button>
        </div>
      )}
      {items.length === 0 && !showForm ? (
        <EmptyState message="No safety profiles configured yet." actionLabel="Create Profile" onAction={() => setShowForm(true)} />
      ) : (
        <div className="flex flex-col gap-[var(--space-sm)]">
          {items.map((s) => (
            <ExpandableCard
              key={s.id}
              title={s.pseudonym || 'Unnamed profile'}
              subtitle={`OPSEC ${s.opsec_level} · Threat: ${s.threat_level}`}
              badge={new Date(s.created_at).toLocaleDateString()}
            >
              <div className="grid grid-cols-2 gap-[var(--space-md)]">
                <DetailField label="OPSEC Level" value={s.opsec_level} />
                <DetailField label="Threat Level" value={s.threat_level} />
                {s.pseudonym && <DetailField label="Pseudonym" value={s.pseudonym} />}
                <DetailField label="VPN Required" value={s.required_vpn ? 'Yes' : 'No'} />
                <DetailField label="Tor Required" value={s.required_tor ? 'Yes' : 'No'} />
                {s.approved_devices.length > 0 && <DetailField label="Approved Devices" value={s.approved_devices.join(', ')} full />}
                {s.prohibited_platforms.length > 0 && <DetailField label="Prohibited Platforms" value={s.prohibited_platforms.join(', ')} full />}
                {s.threat_notes && <DetailField label="Threat Notes" value={s.threat_notes} full />}
                <DetailField label="Briefing Completed" value={s.safety_briefing_completed ? 'Yes' : 'No'} />
              </div>
            </ExpandableCard>
          ))}
        </div>
      )}
    </div>
  );
}

/* --- Reusable Detail Components --- */

function ExpandableCard({
  title,
  subtitle,
  badge,
  children,
  onEdit,
}: {
  title: string;
  subtitle: string;
  badge?: string;
  children: React.ReactNode;
  onEdit?: () => void;
}) {
  const [expanded, setExpanded] = useState(false);
  return (
    <div
      className="rounded-md border overflow-hidden"
      style={{ borderColor: 'var(--border-default)', background: 'var(--bg-elevated)' }}
    >
      <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        className="w-full text-left p-[var(--space-md)] flex items-start justify-between gap-[var(--space-md)]"
        style={{ cursor: 'pointer' }}
        aria-expanded={expanded}
      >
        <div className="flex-1 min-w-0">
          <p className="text-sm font-medium truncate" style={{ color: 'var(--text-primary)' }}>
            {title}
          </p>
          <p className="text-xs mt-[var(--space-2xs)] truncate" style={{ color: 'var(--text-tertiary)' }}>
            {subtitle}
          </p>
        </div>
        <div className="flex items-center gap-[var(--space-sm)] shrink-0">
          {badge && (
            <span className="text-xs font-[family-name:var(--font-mono)]" style={{ color: 'var(--text-secondary)' }}>
              {badge}
            </span>
          )}
          <span className="text-xs" style={{ color: 'var(--text-tertiary)' }}>
            {expanded ? '▴' : '▾'}
          </span>
        </div>
      </button>
      {expanded && (
        <div
          className="px-[var(--space-md)] pb-[var(--space-md)]"
          style={{ borderTop: '1px solid var(--border-subtle)' }}
          role="region"
        >
          <div className="pt-[var(--space-md)]">{children}</div>
          {onEdit && (
            <div className="mt-[var(--space-md)] pt-[var(--space-sm)]" style={{ borderTop: '1px solid var(--border-subtle)' }}>
              <button
                type="button"
                onClick={(e) => { e.stopPropagation(); onEdit(); }}
                className="btn-secondary text-xs"
              >
                Edit
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

function DetailField({
  label,
  value,
  full,
}: {
  label: string;
  value: string;
  full?: boolean;
}) {
  return (
    <div className={full ? 'col-span-2' : ''}>
      <dt className="field-label">{label}</dt>
      <dd
        className="mt-[var(--space-xs)] text-sm whitespace-pre-wrap"
        style={{ color: 'var(--text-primary)' }}
      >
        {value}
      </dd>
    </div>
  );
}
