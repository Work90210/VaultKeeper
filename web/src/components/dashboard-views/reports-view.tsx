import type { InvestigationReport, ReportStatus, ReportType } from '@/types';

export interface ReportWithCase extends InvestigationReport {
  case_reference: string;
  case_title: string;
}

interface ReportsViewProps {
  reports: ReportWithCase[];
  caseRef?: string;
}

const STATUS_DISPLAY: Record<ReportStatus, { label: string; cls: string }> = {
  draft: { label: 'draft', cls: 'pl draft' },
  in_review: { label: 'in review', cls: 'pl draft' },
  approved: { label: 'approved', cls: 'pl sealed' },
  published: { label: 'published', cls: 'pl sealed' },
  withdrawn: { label: 'withdrawn', cls: 'pl broken' },
};

const TYPE_LABELS: Record<ReportType, string> = {
  interim: 'interim',
  final: 'final',
  supplementary: 'supplementary',
  expert_opinion: 'expert opinion',
};

function formatDate(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleDateString('en-GB', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  });
}

function relativeTime(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const hours = Math.floor(diff / 3600000);
  if (hours < 1) return 'just now';
  if (hours < 24) return `${hours} h ago`;
  const days = Math.floor(hours / 24);
  return `${days} d ago`;
}

/* report template cards (static, matching the design) */
const TEMPLATES = [
  { n: 'Custody summary', d: 'PDF/A with full hash chain, signature ledger, and RFC 3161 timestamps per exhibit \u2014 paginated for filing.', ic: '\u00b6', t: 'standard' },
  { n: 'Disclosure dossier', d: 'Bundled ZIP for defence/prosecution: exhibits, redaction maps, custody summary, and offline validator.', ic: '\u25e9', t: 'standard' },
  { n: 'Ceremony minutes', d: 'Break-the-glass and quorum events with timestamps, quorum members, and biometric attestations.', ic: '\u25c6', t: 'governance' },
  { n: 'Quarterly retention report', d: 'What\u2019s sealed, what\u2019s held, what\u2019s due for review. Filed automatically with supervisory board.', ic: '\u25a4', t: 'governance' },
  { n: 'Counter-evidence preserved', d: 'Rule 77 disclosure: all preserved exculpatory material, per-case, with reasoning notes.', ic: '\u21c5', t: 'legal' },
  { n: 'Federation diff', d: 'Cross-chain reconciliation between peer instances \u2014 CIJA, KSC, Bellingcat \u2014 with divergence log.', ic: '\u29d6', t: 'platform' },
];

const AVATAR_CLASSES = ['a', 'b', 'c', 'd', 'e'];

export default function ReportsView({ reports, caseRef }: ReportsViewProps) {
  const sorted = [...reports].sort(
    (a, b) =>
      new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
  );
  const recent = sorted.slice(0, 5);

  return (
    <>
      <section className="d-pagehead">
        <div>
          <span className="eyebrow-m">
            {caseRef ? `Case \u00b7 ${caseRef} \u00b7 ` : ''}Berkeley Protocol Reporting
          </span>
          <h1>
            Signed <em>reports</em>
          </h1>
          <p className="sub">
            Reports are sealed snapshots, not live documents. Once generated
            they are written to the chain, cryptographically frozen, and
            recipients can verify them offline with the open validator &mdash;
            even if VaultKeeper disappears tomorrow.
          </p>
        </div>
        <div className="actions">
          <a className="btn ghost" href="#">
            Report history
          </a>
          <a className="btn" href="#">
            New report <span className="arr">&rarr;</span>
          </a>
        </div>
      </section>

      {/* Template grid */}
      <div className="panel" style={{ marginBottom: 22 }}>
        <div className="panel-h">
          <h3>Report templates</h3>
          <span className="meta">{TEMPLATES.length} standard</span>
        </div>
        <div
          className="panel-body"
          style={{
            padding: 0,
            display: 'grid',
            gridTemplateColumns: 'repeat(3, 1fr)',
            gap: 0,
          }}
        >
          {TEMPLATES.map((tmpl, i) => (
            <div
              key={tmpl.n}
              style={{
                padding: 28,
                borderRight: (i + 1) % 3 ? '1px solid var(--line)' : 'none',
                borderBottom: i < 3 ? '1px solid var(--line)' : 'none',
                cursor: 'pointer',
                transition: 'background .2s',
              }}
            >
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  marginBottom: 16,
                }}
              >
                <span
                  style={{
                    width: 40,
                    height: 40,
                    borderRadius: 10,
                    background: 'var(--paper)',
                    border: '1px solid var(--line)',
                    display: 'grid',
                    placeItems: 'center',
                    fontFamily: "'Fraunces', serif",
                    fontSize: 20,
                    color: 'var(--accent)',
                  }}
                >
                  {tmpl.ic}
                </span>
                <span className="tag">{tmpl.t}</span>
              </div>
              <div
                style={{
                  fontFamily: "'Fraunces', serif",
                  fontSize: 22,
                  letterSpacing: '-.015em',
                  lineHeight: 1.2,
                  marginBottom: 8,
                }}
              >
                {tmpl.n}
              </div>
              <p
                style={{
                  color: 'var(--muted)',
                  fontSize: 13.5,
                  lineHeight: 1.55,
                  marginBottom: 14,
                }}
              >
                {tmpl.d}
              </p>
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  paddingTop: 14,
                  borderTop: '1px solid var(--line)',
                  fontFamily: "'JetBrains Mono', monospace",
                  fontSize: 10.5,
                  letterSpacing: '.06em',
                  textTransform: 'uppercase',
                  color: 'var(--muted)',
                }}
              >
                <span>
                  {reports.filter(
                    (r) =>
                      r.title
                        ?.toLowerCase()
                        .includes(tmpl.n.toLowerCase().split(' ')[0]) ||
                      false
                  ).length || 0}{' '}
                  generated
                </span>
                <a className="linkarrow" href="#" style={{ fontSize: 12 }}>
                  Generate &rarr;
                </a>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Recently generated table */}
      <div className="panel">
        <div className="panel-h">
          <h3>Recently generated</h3>
          <span className="meta">
            {recent.length > 0 ? `last ${recent.length} \u00b7 sealed` : 'none'}
          </span>
        </div>
        {recent.length === 0 ? (
          <div className="panel-body">
            <p style={{ padding: '24px 16px', opacity: 0.6 }}>
              No report bundles generated yet.
            </p>
          </div>
        ) : (
          <table className="tbl">
            <thead>
              <tr>
                <th>Report</th>
                <th>Case</th>
                <th>By</th>
                <th>Template</th>
                <th>Hash</th>
                <th>Generated</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {recent.map((r, idx) => {
                const display = STATUS_DISPLAY[r.status] ?? {
                  label: r.status,
                  cls: 'pl draft',
                };
                const typeLabel = TYPE_LABELS[r.report_type] ?? r.report_type;
                const avClass = AVATAR_CLASSES[idx % AVATAR_CLASSES.length];
                const evidenceCount = r.referenced_evidence_ids?.length ?? 0;
                const hashSnippet = r.id
                  ? `${r.id.slice(0, 4)}\u2026${r.id.slice(-4)}`
                  : '\u2014';

                return (
                  <tr key={r.id}>
                    <td className="ref">
                      <strong>RPT-{r.id.slice(0, 8)}</strong>
                    </td>
                    <td style={{ fontSize: 13.5 }}>{r.case_reference}</td>
                    <td>
                      <span className="avs">
                        <span className={`av ${avClass}`}>
                          {(r.case_reference || 'R')[0]}
                        </span>
                      </span>{' '}
                      <span style={{ fontSize: 13.5 }}>
                        {evidenceCount > 0 ? `${evidenceCount} exhibits` : '\u2014'}
                      </span>
                    </td>
                    <td style={{ fontSize: 13.5, color: 'var(--muted)' }}>
                      {typeLabel}
                    </td>
                    <td
                      className="mono"
                      style={{ color: 'var(--accent)' }}
                    >
                      {hashSnippet}
                    </td>
                    <td className="mono">{relativeTime(r.created_at)}</td>
                    <td className="actions">
                      <a className="linkarrow" href="#">
                        Verify &rarr;
                      </a>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>
    </>
  );
}

export { ReportsView };
