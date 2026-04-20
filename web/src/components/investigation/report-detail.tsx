'use client';

import type { InvestigationReport } from '@/types';

const STATUS_PILL: Record<string, string> = {
  draft: 'pl draft',
  in_review: 'pl hold',
  approved: 'pl sealed',
  published: 'pl sealed',
  withdrawn: 'pl broken',
};

function formatTime(iso: string): string {
  return new Date(iso).toLocaleString('en-GB', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

export function ReportDetail({
  report,
}: {
  readonly report: InvestigationReport;
  readonly accessToken: string;
}) {
  const sortedSections = [...report.sections].sort(
    (a, b) => a.order - b.order,
  );

  const hasTransparency =
    report.limitations.length > 0 ||
    report.caveats.length > 0 ||
    report.assumptions.length > 0;

  const referencedCount =
    report.referenced_evidence_ids.length +
    report.referenced_analysis_ids.length;

  const hashSnippet = report.id
    ? `${report.id.slice(0, 4)}\u2026${report.id.slice(-4)}`
    : '\u2014';

  return (
    <>
      <section className="d-pagehead">
        <div>
          <span className="eyebrow-m">
            <a href={`/en/cases/${report.case_id}?tab=reports`}>Reports</a>
            <span style={{ margin: '0 6px', color: 'var(--muted)' }}>/</span>
            {report.title}
          </span>
          <h1>{report.title}</h1>
          <p className="sub">
            {report.report_type.replace(/_/g, ' ')} &middot; {sortedSections.length} section{sortedSections.length !== 1 ? 's' : ''} &middot; {referencedCount} referenced items
          </p>
        </div>
        <div className="actions">
          <span className={STATUS_PILL[report.status] ?? 'pl draft'}>
            {report.status.replace(/_/g, ' ')}
          </span>
          <a className="btn ghost" href={`/en/cases/${report.case_id}?tab=reports`}>
            &larr; Back
          </a>
        </div>
      </section>

      <div className="g2-wide">
        {/* Main content */}
        <div>
          {/* Sections */}
          {sortedSections.map((section, idx) => (
            <div className="panel" key={`${section.section_type}-${section.order}`} style={{ marginBottom: idx < sortedSections.length - 1 ? 16 : 0 }}>
              <div className="panel-h">
                <h3>{section.title || section.section_type.replace(/_/g, ' ')}</h3>
                <span className="tag">{section.section_type.replace(/_/g, ' ')}</span>
              </div>
              <div className="panel-body">
                <p style={{ color: 'var(--ink-2)', fontSize: '13.5px', lineHeight: 1.65, whiteSpace: 'pre-wrap' }}>
                  {section.content}
                </p>
              </div>
            </div>
          ))}

          {/* Transparency */}
          {hasTransparency && (
            <div className="panel" style={{ marginTop: 16 }}>
              <div className="panel-h">
                <h3>Transparency</h3>
              </div>
              <div className="panel-body" style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
                {report.limitations.length > 0 && (
                  <div>
                    <div style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: '10.5px', letterSpacing: '.06em', textTransform: 'uppercase', color: 'var(--muted)', marginBottom: 6 }}>
                      Limitations
                    </div>
                    <ul style={{ margin: 0, paddingLeft: 16 }}>
                      {report.limitations.map((item) => (
                        <li key={item} style={{ fontSize: '13.5px', color: 'var(--ink-2)', lineHeight: 1.6 }}>{item}</li>
                      ))}
                    </ul>
                  </div>
                )}
                {report.caveats.length > 0 && (
                  <div>
                    <div style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: '10.5px', letterSpacing: '.06em', textTransform: 'uppercase', color: 'var(--muted)', marginBottom: 6 }}>
                      Caveats
                    </div>
                    <ul style={{ margin: 0, paddingLeft: 16 }}>
                      {report.caveats.map((item) => (
                        <li key={item} style={{ fontSize: '13.5px', color: 'var(--ink-2)', lineHeight: 1.6 }}>{item}</li>
                      ))}
                    </ul>
                  </div>
                )}
                {report.assumptions.length > 0 && (
                  <div>
                    <div style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: '10.5px', letterSpacing: '.06em', textTransform: 'uppercase', color: 'var(--muted)', marginBottom: 6 }}>
                      Assumptions
                    </div>
                    <ul style={{ margin: 0, paddingLeft: 16 }}>
                      {report.assumptions.map((item) => (
                        <li key={item} style={{ fontSize: '13.5px', color: 'var(--ink-2)', lineHeight: 1.6 }}>{item}</li>
                      ))}
                    </ul>
                  </div>
                )}
              </div>
            </div>
          )}
        </div>

        {/* Sidebar */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <div className="panel" style={{ margin: 0 }}>
            <div className="panel-h" style={{ padding: '12px 14px' }}>
              <h3 style={{ fontSize: 15 }}>Metadata</h3>
            </div>
            <div className="panel-body" style={{ padding: 0 }}>
              <dl className="kvs" style={{ padding: '16px 14px' }}>
                <dt>Report type</dt>
                <dd>{report.report_type.replace(/_/g, ' ')}</dd>
                <dt>Status</dt>
                <dd><span className={STATUS_PILL[report.status] ?? 'pl draft'}>{report.status.replace(/_/g, ' ')}</span></dd>
                <dt>Author</dt>
                <dd><code>{report.author_id.slice(0, 8)}&hellip;</code></dd>
                {report.reviewer_id && (
                  <>
                    <dt>Reviewer</dt>
                    <dd>
                      <code>{report.reviewer_id.slice(0, 8)}&hellip;</code>
                      {report.reviewed_at && (
                        <span style={{ display: 'block', color: 'var(--muted)', marginTop: 2, fontSize: '12px' }}>
                          {formatTime(report.reviewed_at)}
                        </span>
                      )}
                    </dd>
                  </>
                )}
                {report.approved_by && (
                  <>
                    <dt>Approved by</dt>
                    <dd>
                      <code>{report.approved_by.slice(0, 8)}&hellip;</code>
                      {report.approved_at && (
                        <span style={{ display: 'block', color: 'var(--muted)', marginTop: 2, fontSize: '12px' }}>
                          {formatTime(report.approved_at)}
                        </span>
                      )}
                    </dd>
                  </>
                )}
                <dt>Evidence refs</dt>
                <dd>{report.referenced_evidence_ids.length}</dd>
                <dt>Analysis refs</dt>
                <dd>{report.referenced_analysis_ids.length}</dd>
                <dt>Hash</dt>
                <dd><code style={{ color: 'var(--accent)' }}>{hashSnippet}</code></dd>
                <dt>Created</dt>
                <dd>{formatTime(report.created_at)}</dd>
                <dt>Updated</dt>
                <dd>{formatTime(report.updated_at)}</dd>
              </dl>
            </div>
          </div>
        </div>
      </div>
    </>
  );
}
