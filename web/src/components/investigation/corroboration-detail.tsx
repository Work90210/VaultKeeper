'use client';

import type { CorroborationClaim, ClaimStrength, RoleInClaim } from '@/types';

const strengthToPill: Record<ClaimStrength, string> = {
  strong: 'sealed',
  moderate: 'hold',
  weak: 'disc',
  contested: 'broken',
};

const roleToPill: Record<RoleInClaim, string> = {
  primary: 'a',
  supporting: 'c',
  contextual: 'd',
  contradicting: 'b',
};

function strengthToScore(s: ClaimStrength): number {
  switch (s) {
    case 'strong':
      return 0.9;
    case 'moderate':
      return 0.65;
    case 'weak':
      return 0.35;
    case 'contested':
      return 0.15;
  }
}

function scoreColor(sc: number): string {
  if (sc >= 0.75) return 'var(--ok)';
  if (sc >= 0.5) return 'var(--accent)';
  return 'var(--muted)';
}

function barColor(sc: number): string {
  if (sc >= 0.75) return 'var(--ok)';
  if (sc >= 0.5) return 'var(--accent)';
  return '#b35c5c';
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

export function CorroborationDetail({
  claim,
  accessToken: _accessToken,
}: {
  readonly claim: CorroborationClaim;
  readonly accessToken: string;
}) {
  const sc = strengthToScore(claim.strength);

  return (
    <>
      {/* Back link */}
      <nav style={{ marginBottom: 18 }}>
        <a
          href={`/en/cases/${claim.case_id}?tab=corroborations`}
          className="linkarrow"
          style={{ fontSize: 13 }}
        >
          &larr; Corroborations
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
          {claim.id.slice(0, 8).toUpperCase()}
        </span>
        <span className={`pl ${strengthToPill[claim.strength]}`}>
          {claim.strength}
        </span>
        <span className="tag">{claim.claim_type.replace(/_/g, ' ')}</span>
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
          {claim.claim_summary}
        </em>
      </h1>

      {/* KPIs strip */}
      <div className="d-kpis" style={{ marginBottom: 28 }}>
        <div className="d-kpi">
          <div className="k">Score</div>
          <div className="v" style={{ color: scoreColor(sc) }}>
            {sc.toFixed(2)}
          </div>
          <div className="sub">corroboration strength</div>
        </div>
        <div className="d-kpi">
          <div className="k">Strength</div>
          <div className="v">{claim.strength}</div>
          <div className="sub">
            <span className={`pl ${strengthToPill[claim.strength]}`}>
              {claim.strength}
            </span>
          </div>
        </div>
        <div className="d-kpi">
          <div className="k">Sources</div>
          <div className="v">{claim.evidence.length}</div>
          <div className="sub">linked evidence items</div>
        </div>
        <div className="d-kpi">
          <div className="k">Progress</div>
          <div className="v">
            <div
              style={{
                width: '100%',
                height: 6,
                background: 'var(--bg-2)',
                borderRadius: 3,
                overflow: 'hidden',
                marginTop: 18,
              }}
            >
              <div
                style={{
                  height: '100%',
                  width: `${sc * 100}%`,
                  background: barColor(sc),
                }}
              />
            </div>
          </div>
          <div className="sub">{(sc * 100).toFixed(0)}%</div>
        </div>
      </div>

      {/* Main content + sidebar layout */}
      <div className="g2-wide">
        {/* Main content */}
        <div>
          {/* Claim Summary panel */}
          <div className="panel" style={{ marginBottom: 22 }}>
            <div className="panel-h">
              <h3>
                Claim <em>summary</em>
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
                {claim.claim_summary}
              </p>
            </div>
          </div>

          {/* Analysis Notes panel */}
          {claim.analysis_notes && (
            <div className="panel" style={{ marginBottom: 22 }}>
              <div className="panel-h">
                <h3>
                  Analysis <em>notes</em>
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
                  {claim.analysis_notes}
                </p>
              </div>
            </div>
          )}

          {/* Linked Evidence panel */}
          {claim.evidence.length > 0 && (
            <div className="panel">
              <div className="panel-h">
                <h3>
                  Linked <em>evidence</em>
                </h3>
                <span className="meta">
                  sources &middot; {claim.evidence.length}
                </span>
              </div>
              <div className="panel-body flush">
                {claim.evidence.map((item) => (
                  <div
                    key={item.id}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 10,
                      padding: '14px 22px',
                      borderBottom: '1px solid var(--line)',
                    }}
                  >
                    <span
                      className={`av ${roleToPill[item.role_in_claim] ?? 'd'}`}
                      style={{
                        width: 20,
                        height: 20,
                        fontSize: 9,
                        borderWidth: 1,
                      }}
                    >
                      {item.role_in_claim[0].toUpperCase()}
                    </span>
                    <span style={{ color: 'var(--ink-2)', fontSize: 13.5 }}>
                      {item.contribution_notes ||
                        item.evidence_id.slice(0, 8).toUpperCase()}
                    </span>
                    <span className="pl draft" style={{ marginLeft: 'auto' }}>
                      {item.role_in_claim}
                    </span>
                    <span
                      style={{
                        fontFamily: "'JetBrains Mono', monospace",
                        fontSize: 10,
                        color: 'var(--muted)',
                      }}
                    >
                      {item.added_by}
                    </span>
                  </div>
                ))}
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
                <dt>Claim type</dt>
                <dd>
                  <span className="tag">
                    {claim.claim_type.replace(/_/g, ' ')}
                  </span>
                </dd>

                <dt>Strength</dt>
                <dd>
                  <span className={`pl ${strengthToPill[claim.strength]}`}>
                    {claim.strength}
                  </span>
                </dd>

                <dt>Score</dt>
                <dd>
                  <strong
                    style={{
                      fontFamily: "'Fraunces', serif",
                      fontSize: 22,
                      letterSpacing: '-.02em',
                      color: scoreColor(sc),
                    }}
                  >
                    {sc.toFixed(2)}
                  </strong>
                </dd>

                <dt>Evidence</dt>
                <dd>
                  <code>
                    {claim.evidence.length} source
                    {claim.evidence.length !== 1 ? 's' : ''}
                  </code>
                </dd>

                <dt>Created by</dt>
                <dd>
                  <code>{claim.created_by}</code>
                </dd>

                <dt>Created</dt>
                <dd>{formatTimestamp(claim.created_at)}</dd>

                {claim.updated_at !== claim.created_at && (
                  <>
                    <dt>Updated</dt>
                    <dd>{formatTimestamp(claim.updated_at)}</dd>
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
              {claim.id}
            </div>
          </div>
        </div>
      </div>
    </>
  );
}
