'use client';

type TabKey =
  | 'inquiry-logs'
  | 'assessments'
  | 'verifications'
  | 'corroborations'
  | 'analysis'
  | 'templates'
  | 'reports'
  | 'safety';

interface InvestigationDashboardProps {
  readonly inquiryLogCount: number;
  readonly assessmentCount: number;
  readonly verificationCount: number;
  readonly corroborationCount: number;
  readonly analysisNoteCount: number;
  readonly reportsByStatus: Record<string, number>;
  readonly totalEvidence: number;
  readonly assessedEvidenceCount: number;
  readonly verifiedEvidenceCount: number;
  readonly onNavigateToTab: (tab: TabKey) => void;
}

const PHASES = [
  { label: 'Inquiry', key: 'inquiry' as const },
  { label: 'Assess', key: 'assess' as const },
  { label: 'Collect', key: 'collect' as const },
  { label: 'Preserve', key: 'preserve' as const },
  { label: 'Verify', key: 'verify' as const },
  { label: 'Analyze', key: 'analyze' as const },
] as const;

export function InvestigationDashboard({
  inquiryLogCount,
  assessmentCount,
  verificationCount,
  corroborationCount: _corroborationCount,
  analysisNoteCount,
  reportsByStatus,
  totalEvidence,
  assessedEvidenceCount,
  verifiedEvidenceCount,
  onNavigateToTab,
}: InvestigationDashboardProps) {
  const unassessedCount = totalEvidence - assessedEvidenceCount;
  const unverifiedCount = totalEvidence - verifiedEvidenceCount;
  const draftReports = reportsByStatus['draft'] || 0;
  const publishedReports = reportsByStatus['published'] || 0;
  const totalReports = Object.values(reportsByStatus).reduce((a, b) => a + b, 0);

  const phaseActive: Record<string, boolean> = {
    inquiry: inquiryLogCount > 0,
    assess: assessmentCount > 0,
    collect: totalEvidence > 0,
    preserve: totalEvidence > 0,
    verify: verificationCount > 0,
    analyze: analysisNoteCount > 0,
  };

  const phaseCounts: Record<string, number> = {
    inquiry: inquiryLogCount,
    assess: assessmentCount,
    collect: totalEvidence,
    preserve: totalEvidence,
    verify: verificationCount,
    analyze: analysisNoteCount,
  };

  const needsAttention: { message: string; tab: TabKey }[] = [];
  if (unassessedCount > 0) {
    needsAttention.push({ message: `${unassessedCount} evidence items unassessed`, tab: 'assessments' });
  }
  if (unverifiedCount > 0) {
    needsAttention.push({ message: `${unverifiedCount} evidence items unverified`, tab: 'verifications' });
  }
  if (draftReports > 0) {
    needsAttention.push({ message: `${draftReports} reports in draft`, tab: 'reports' });
  }

  return (
    <div className="mb-[var(--space-lg)]">
      {/* Counter cards */}
      <div className="card-inset grid grid-cols-2 sm:grid-cols-5 gap-[var(--space-md)] p-[var(--space-md)] mb-[var(--space-md)]">
        <CounterCard label="Evidence" value={totalEvidence} />
        <CounterCard label="Assessed" value={assessedEvidenceCount} />
        <CounterCard label="Verified" value={verifiedEvidenceCount} />
        <CounterCard label="Unverified" value={unverifiedCount} highlight={unverifiedCount > 0} />
        <CounterCard label="Reports" value={totalReports} subtitle={`${draftReports} draft, ${publishedReports} pub`} />
      </div>

      {/* Berkeley Protocol phases */}
      <div className="card-inset p-[var(--space-md)] mb-[var(--space-md)]">
        <h3 className="text-xs uppercase tracking-wider font-medium mb-[var(--space-sm)]" style={{ color: 'var(--text-tertiary)' }}>
          Berkeley Protocol Phases
        </h3>
        <div className="flex items-center gap-[var(--space-xs)]">
          {PHASES.map((phase, i) => (
            <div key={phase.key} className="flex items-center gap-[var(--space-xs)]">
              <div className="flex flex-col items-center gap-[var(--space-2xs)]">
                <div
                  className="w-3 h-3 rounded-full border-2"
                  style={{
                    borderColor: phaseActive[phase.key] ? 'var(--status-active)' : 'var(--border-default)',
                    backgroundColor: phaseActive[phase.key] ? 'var(--status-active)' : 'transparent',
                  }}
                />
                <span className="text-[10px] leading-none" style={{ color: 'var(--text-tertiary)' }}>
                  {phase.label}
                </span>
                <span className="text-[10px] leading-none font-[family-name:var(--font-mono)]" style={{ color: 'var(--text-tertiary)' }}>
                  ({phaseCounts[phase.key]})
                </span>
              </div>
              {i < PHASES.length - 1 && (
                <div
                  className="w-6 h-0.5 mb-6"
                  style={{
                    backgroundColor: phaseActive[phase.key] ? 'var(--status-active)' : 'var(--border-default)',
                  }}
                />
              )}
            </div>
          ))}
        </div>
      </div>

      {/* Needs attention */}
      {needsAttention.length > 0 && (
        <div className="card-inset p-[var(--space-md)]">
          <h3 className="text-xs uppercase tracking-wider font-medium mb-[var(--space-xs)]" style={{ color: 'var(--text-tertiary)' }}>
            Needs Attention
          </h3>
          <div className="space-y-[var(--space-2xs)]">
            {needsAttention.map((item) => (
              <button
                key={item.tab}
                type="button"
                onClick={() => onNavigateToTab(item.tab)}
                className="text-sm block hover:underline"
                style={{ color: 'var(--amber-accent)', background: 'none', border: 'none', cursor: 'pointer', padding: 0 }}
              >
                &middot; {item.message} &rarr;
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function CounterCard({ label, value, subtitle, highlight }: {
  label: string;
  value: number;
  subtitle?: string;
  highlight?: boolean;
}) {
  return (
    <div className="text-center">
      <p
        className="text-xl font-[family-name:var(--font-heading)] font-semibold"
        style={{ color: highlight ? 'var(--amber-accent)' : 'var(--text-primary)' }}
      >
        {value}
      </p>
      <p className="text-xs" style={{ color: 'var(--text-tertiary)' }}>{label}</p>
      {subtitle && <p className="text-[10px]" style={{ color: 'var(--text-tertiary)' }}>{subtitle}</p>}
    </div>
  );
}
