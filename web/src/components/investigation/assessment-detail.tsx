'use client';

import type { EvidenceAssessment, Recommendation } from '@/types';

const recColors: Record<Recommendation, string> = {
  collect: 'var(--ok)',
  monitor: 'var(--accent)',
  deprioritize: '#b35c5c',
  discard: '#6b3a4a',
};

const recBgs: Record<Recommendation, string> = {
  collect: 'rgba(74,107,58,.1)',
  monitor: 'rgba(184,66,28,.1)',
  deprioritize: 'rgba(179,92,92,.1)',
  discard: 'rgba(107,58,74,.1)',
};

const credColors: Record<string, string> = {
  established: 'var(--ok)',
  credible: 'var(--ok)',
  uncertain: 'var(--accent)',
  unreliable: 'var(--muted)',
  unassessed: 'var(--muted)',
};

function scoreColor(score: number): string {
  if (score >= 7) return 'var(--ok)';
  if (score >= 5) return 'var(--accent)';
  return '#b35c5c';
}

function formatLabel(value: string): string {
  return value.charAt(0).toUpperCase() + value.slice(1).replace(/_/g, ' ');
}

function formatTimestamp(iso: string): string {
  return new Date(iso).toLocaleString('en-GB', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

export function AssessmentDetail({
  assessment,
}: {
  readonly assessment: EvidenceAssessment;
}) {
  return (
    <>
      {/* Back link */}
      <nav style={{ marginBottom: 18 }}>
        <a
          href={`/en/cases/${assessment.case_id}?tab=assessments`}
          className="linkarrow"
          style={{ fontSize: 13 }}
        >
          &larr; Assessments
        </a>
      </nav>

      {/* Header badges */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          marginBottom: 12,
          flexWrap: 'wrap',
        }}
      >
        <span
          style={{
            fontFamily: "'JetBrains Mono', monospace",
            fontSize: 13,
            color: 'var(--muted)',
            letterSpacing: '.04em',
          }}
        >
          {assessment.evidence_id.slice(0, 8).toUpperCase()}
        </span>
        <span
          style={{
            padding: '2px 8px',
            borderRadius: 999,
            fontSize: 11,
            fontWeight: 500,
            color: recColors[assessment.recommendation],
            background: recBgs[assessment.recommendation],
            textTransform: 'capitalize',
          }}
        >
          {assessment.recommendation}
        </span>
        <span
          style={{
            padding: '2px 8px',
            borderRadius: 999,
            fontSize: 11,
            color:
              credColors[assessment.source_credibility] ?? 'var(--muted)',
            border: '1px solid currentColor',
          }}
        >
          {formatLabel(assessment.source_credibility)}
        </span>
      </div>

      {/* Title */}
      <h1
        style={{
          fontFamily: "'Fraunces', serif",
          fontWeight: 400,
          fontSize: 'clamp(28px, 3vw, 42px)',
          letterSpacing: '-.025em',
          lineHeight: 1.05,
          marginBottom: 24,
        }}
      >
        <em style={{ color: 'var(--accent)', fontStyle: 'italic' }}>
          {assessment.evidence_id.slice(0, 8).toUpperCase()}
        </em>
      </h1>

      {/* KPIs strip */}
      <div className="d-kpis" style={{ marginBottom: 28 }}>
        <div className="d-kpi">
          <div className="k">Relevance</div>
          <div className="v" style={{ color: scoreColor(assessment.relevance_score) }}>
            {assessment.relevance_score}
            <span style={{ fontSize: 18, color: 'var(--muted)' }}>/10</span>
          </div>
          <div className="sub">relevance score</div>
        </div>
        <div className="d-kpi">
          <div className="k">Reliability</div>
          <div className="v" style={{ color: scoreColor(assessment.reliability_score) }}>
            {assessment.reliability_score}
            <span style={{ fontSize: 18, color: 'var(--muted)' }}>/10</span>
          </div>
          <div className="sub">reliability score</div>
        </div>
        <div className="d-kpi">
          <div className="k">Source credibility</div>
          <div className="v" style={{ fontSize: 24 }}>
            <span
              className={`pl ${assessment.source_credibility === 'established' || assessment.source_credibility === 'credible' ? 'sealed' : assessment.source_credibility === 'uncertain' ? 'hold' : 'draft'}`}
            >
              {formatLabel(assessment.source_credibility)}
            </span>
          </div>
        </div>
        <div className="d-kpi">
          <div className="k">Recommendation</div>
          <div className="v" style={{ fontSize: 24 }}>
            <span
              style={{
                padding: '4px 14px',
                borderRadius: 999,
                fontSize: 14,
                fontWeight: 500,
                color: recColors[assessment.recommendation],
                background: recBgs[assessment.recommendation],
                textTransform: 'capitalize',
              }}
            >
              {assessment.recommendation}
            </span>
          </div>
        </div>
      </div>

      {/* Main content + sidebar layout */}
      <div className="g2-wide">
        {/* Main content */}
        <div>
          {/* Relevance Rationale panel */}
          <div className="panel" style={{ marginBottom: 22 }}>
            <div className="panel-h">
              <h3>
                Relevance <em>rationale</em>
              </h3>
            </div>
            <div className="panel-body">
              <p
                style={{
                  fontSize: 14,
                  lineHeight: 1.6,
                  color: 'var(--ink-2)',
                  whiteSpace: 'pre-wrap',
                }}
              >
                {assessment.relevance_rationale}
              </p>
            </div>
          </div>

          {/* Reliability Rationale panel */}
          <div className="panel" style={{ marginBottom: 22 }}>
            <div className="panel-h">
              <h3>
                Reliability <em>rationale</em>
              </h3>
            </div>
            <div className="panel-body">
              <p
                style={{
                  fontSize: 14,
                  lineHeight: 1.6,
                  color: 'var(--ink-2)',
                  whiteSpace: 'pre-wrap',
                }}
              >
                {assessment.reliability_rationale}
              </p>
            </div>
          </div>

          {/* Methodology panel */}
          {assessment.methodology && (
            <div className="panel" style={{ marginBottom: 22 }}>
              <div className="panel-h">
                <h3>
                  <em>Methodology</em>
                </h3>
              </div>
              <div className="panel-body">
                <p
                  style={{
                    fontSize: 13.5,
                    lineHeight: 1.6,
                    color: 'var(--muted)',
                    whiteSpace: 'pre-wrap',
                  }}
                >
                  {assessment.methodology}
                </p>
              </div>
            </div>
          )}

          {/* Misleading Indicators panel */}
          {assessment.misleading_indicators &&
            assessment.misleading_indicators.length > 0 && (
              <div className="panel">
                <div className="panel-h">
                  <h3>
                    Misleading <em>indicators</em>
                  </h3>
                </div>
                <div className="panel-body">
                  <div
                    style={{
                      padding: '12px 16px',
                      borderRadius: 8,
                      background: 'rgba(184,66,28,.04)',
                      border: '1px solid rgba(184,66,28,.12)',
                    }}
                  >
                    {assessment.misleading_indicators.map((indicator) => (
                      <div
                        key={indicator}
                        style={{
                          fontSize: 13,
                          color: 'var(--ink-2)',
                          lineHeight: 1.55,
                        }}
                      >
                        &middot; {indicator}
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            )}
        </div>

        {/* Sidebar */}
        <div>
          <div className="panel">
            <div className="panel-h">
              <h3>Details</h3>
            </div>
            <div className="panel-body">
              <dl className="kvs">
                <dt>Credibility</dt>
                <dd>
                  <span
                    style={{
                      padding: '2px 8px',
                      borderRadius: 999,
                      fontSize: 11,
                      color:
                        credColors[assessment.source_credibility] ??
                        'var(--muted)',
                      border: '1px solid currentColor',
                    }}
                  >
                    {formatLabel(assessment.source_credibility)}
                  </span>
                </dd>

                <dt>Recommendation</dt>
                <dd>
                  <span
                    style={{
                      padding: '2px 8px',
                      borderRadius: 999,
                      fontSize: 11,
                      fontWeight: 500,
                      color: recColors[assessment.recommendation],
                      background: recBgs[assessment.recommendation],
                      textTransform: 'capitalize',
                    }}
                  >
                    {assessment.recommendation}
                  </span>
                </dd>

                <dt>Relevance</dt>
                <dd>
                  <strong>{assessment.relevance_score}/10</strong>
                </dd>

                <dt>Reliability</dt>
                <dd>
                  <strong>{assessment.reliability_score}/10</strong>
                </dd>

                {assessment.reviewed_by && (
                  <>
                    <dt>Reviewed by</dt>
                    <dd>
                      <code>{assessment.reviewed_by}</code>
                      {assessment.reviewed_at && (
                        <div
                          style={{
                            fontFamily: "'JetBrains Mono', monospace",
                            fontSize: 10.5,
                            color: 'var(--muted)',
                            marginTop: 2,
                          }}
                        >
                          {formatTimestamp(assessment.reviewed_at)}
                        </div>
                      )}
                    </dd>
                  </>
                )}

                <dt>Assessed by</dt>
                <dd>
                  <code>{assessment.assessed_by}</code>
                </dd>

                <dt>Created</dt>
                <dd>{formatTimestamp(assessment.created_at)}</dd>

                {assessment.updated_at !== assessment.created_at && (
                  <>
                    <dt>Updated</dt>
                    <dd>{formatTimestamp(assessment.updated_at)}</dd>
                  </>
                )}
              </dl>
            </div>
          </div>

          {/* Record ID */}
          <div style={{ marginTop: 14 }}>
            <div
              style={{
                fontFamily: "'JetBrains Mono', monospace",
                fontSize: 10,
                color: 'var(--muted)',
                letterSpacing: '.04em',
              }}
            >
              {assessment.id}
            </div>
          </div>
        </div>
      </div>
    </>
  );
}
