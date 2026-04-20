import { authenticatedFetch } from '@/lib/api';
import { getProfile } from '@/lib/org-api';
import type { Case } from '@/types';

function getGreeting(): string {
  const hour = new Date().getHours();
  if (hour < 12) return 'Good morning';
  if (hour < 18) return 'Good afternoon';
  return 'Good evening';
}

function formatEyebrowDate(): string {
  const now = new Date();
  const weekday = now.toLocaleDateString('en-GB', { weekday: 'long' });
  const day = now.getDate();
  const month = now.toLocaleDateString('en-GB', { month: 'long' });
  const year = now.getFullYear();
  const time = now.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit', hour12: false });
  const offset = -now.getTimezoneOffset();
  const sign = offset >= 0 ? '+' : '-';
  const absOffset = Math.abs(offset);
  const offsetH = Math.floor(absOffset / 60);
  const offsetM = absOffset % 60;
  const tz = `UTC${sign}${offsetH}${offsetM > 0 ? `:${String(offsetM).padStart(2, '0')}` : ''}`;
  return `${weekday} \u00b7 ${day} ${month} ${year} \u00b7 ${time} ${tz}`;
}

function formatNowTime(): string {
  const now = new Date();
  return now.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false });
}

function getFirstName(displayName: string): string {
  return displayName.split(' ')[0] || displayName;
}

function timeAgo(dateStr: string): string {
  const updatedAt = new Date(dateStr);
  const now = Date.now();
  const diffMs = now - updatedAt.getTime();
  const diffMin = Math.floor(diffMs / 60000);
  const diffH = Math.floor(diffMs / 3600000);
  const diffD = Math.floor(diffMs / 86400000);
  if (diffMin < 1) return 'Just now';
  if (diffMin < 60) return `${diffMin} min ago`;
  if (diffH < 24) return `${diffH}h ago`;
  if (diffD === 1) return 'Yesterday';
  if (diffD < 7) return `${diffD} d ago`;
  return updatedAt.toLocaleDateString('en-GB', { day: 'numeric', month: 'short' });
}

const BERKELEY_PHASES = [
  { n: '1 \u00b7 Inquiry', pct: 83 },
  { n: '2 \u00b7 Assessment', pct: 69 },
  { n: '3 \u00b7 Collection', pct: 99 },
  { n: '4 \u00b7 Preservation', pct: 100 },
  { n: '5 \u00b7 Verification', pct: 58 },
  { n: '6 \u00b7 Analysis', pct: 43 },
] as const;

function phaseColor(pct: number): string {
  if (pct >= 90) return 'var(--ok)';
  if (pct >= 60) return 'var(--accent)';
  return '#b35c5c';
}

export default async function OverviewView() {
  const [casesRes, profileRes] = await Promise.all([
    authenticatedFetch<Case[]>('/api/cases'),
    getProfile(),
  ]);

  const cases = casesRes.data ?? [];
  const profile = profileRes.data;
  const firstName = profile?.profile?.display_name
    ? getFirstName(profile.profile.display_name)
    : 'there';

  const error = casesRes.error;

  if (error) {
    return (
      <div className="banner-error" style={{ marginBottom: '16px' }}>
        {error}
      </div>
    );
  }

  const activeCases = cases.filter((c) => c.status === 'active');
  const activeCaseCount = activeCases.length;
  const holdCases = cases.filter((c) => c.status !== 'active');
  const greeting = getGreeting();
  const eyebrow = formatEyebrowDate();
  const nowTime = formatNowTime();

  const activeCaseWord = activeCaseCount === 1
    ? 'One case active'
    : activeCaseCount === 0
      ? 'No cases active'
      : `${activeCaseCount} case${activeCaseCount > 1 ? 's' : ''} active`;

  return (
    <>
      {/* ---- Page header ---- */}
      <section className="d-pagehead">
        <div>
          <span className="eyebrow-m">{eyebrow}</span>
          <h1>{greeting}, <em>{firstName}.</em></h1>
          <p className="sub">
            {activeCaseWord} across the workspace.
          </p>
        </div>
        <div className="actions">
          <a className="btn ghost" href="?view=cases">All cases</a>
          <a className="btn" href="?view=evidence">Upload evidence <span className="arr">&rarr;</span></a>
        </div>
      </section>

      {/* ---- KPI strip ---- */}
      <div className="d-kpis" style={{ marginBottom: 28 }}>
        <div className="d-kpi">
          <div className="k">Exhibits sealed &middot; today</div>
          <div className="v">142</div>
          <div className="delta">&#9650; 18 vs. yesterday</div>
        </div>
        <div className="d-kpi">
          <div className="k">Chain integrity</div>
          <div className="v">100<em>%</em></div>
          <div className="sub">48,217 events &middot; zero breaks</div>
        </div>
        <div className="d-kpi">
          <div className="k">Pending corroborations</div>
          <div className="v">7</div>
          <div className="delta n">&#9679; 2 awaiting you</div>
        </div>
        <div className="d-kpi">
          <div className="k">Last RFC 3161 stamp</div>
          <div className="v" style={{
            fontSize: 22,
            fontFamily: "'JetBrains Mono', monospace",
            letterSpacing: '-.01em',
            paddingTop: 6,
          }}>{nowTime}</div>
          <div className="sub">ts-eu-west &middot; signed &#10003;</div>
        </div>
      </div>

      {/* ---- Berkeley Protocol compliance ---- */}
      <div className="panel" style={{ marginBottom: 22 }}>
        <div className="panel-h">
          <h3>Berkeley Protocol <em>compliance</em></h3>
          <span className="meta" style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
            <span style={{
              padding: '2px 7px',
              borderRadius: 4,
              background: 'rgba(184,66,28,.06)',
              border: '1px solid rgba(184,66,28,.12)',
              fontSize: 9,
              letterSpacing: '.06em',
              color: 'var(--accent)',
            }}>OHCHR / UC BERKELEY</span>
            {' '}6-phase cycle
          </span>
        </div>
        <div className="panel-body">
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(6,1fr)', gap: 0, marginBottom: 20 }}>
            {BERKELEY_PHASES.map((p) => {
              const col = phaseColor(p.pct);
              return (
                <div key={p.n} style={{
                  padding: '14px 16px',
                  borderRight: '1px solid var(--line)',
                  textAlign: 'center',
                }}>
                  <div style={{
                    fontFamily: 'JetBrains Mono, monospace',
                    fontSize: '9.5px',
                    letterSpacing: '.08em',
                    textTransform: 'uppercase',
                    color: 'var(--muted)',
                    marginBottom: 8,
                  }}>{p.n}</div>
                  <div style={{
                    fontFamily: 'Fraunces, serif',
                    fontSize: 28,
                    letterSpacing: '-.02em',
                    color: col,
                  }}>{p.pct}<span style={{ fontSize: 14, color: 'var(--muted)' }}>%</span></div>
                  <div style={{
                    height: 4,
                    background: 'var(--bg-2)',
                    borderRadius: 2,
                    overflow: 'hidden',
                    margin: '8px auto 6px',
                    maxWidth: 80,
                  }}>
                    <div style={{
                      height: '100%',
                      width: `${p.pct}%`,
                      background: col,
                      borderRadius: 2,
                    }} />
                  </div>
                </div>
              );
            })}
          </div>
          <div style={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            paddingTop: 14,
            borderTop: '1px solid var(--line)',
          }}>
            <div style={{ fontSize: 13, color: 'var(--muted)' }}>
              <strong style={{ color: 'var(--ink)' }}>4,108 exhibits</strong> need attention &mdash; most missing Phase 5 (verification) or Phase 6 (analysis).
            </div>
            <a className="linkarrow" href="?view=evidence" style={{ flexShrink: 0 }}>View by phase gap &rarr;</a>
          </div>
        </div>
      </div>

      {/* ---- Cases in flight + Needs you ---- */}
      <div className="g2-wide" style={{ marginBottom: 22 }}>
        <div className="panel">
          <div className="panel-h">
            <h3>Your cases <em>in flight</em></h3>
            <span className="meta">
              {activeCaseCount} active
              {holdCases.length > 0 && ` \u00b7 ${holdCases.length} on hold`}
            </span>
          </div>
          {cases.length === 0 ? (
            <div className="panel-body">
              <p style={{ color: 'var(--muted)', fontSize: 13 }}>
                No cases yet. Create your first case to get started.
              </p>
            </div>
          ) : (
            <table className="tbl">
              <thead>
                <tr>
                  <th>Case</th>
                  <th>Role</th>
                  <th>Exhibits</th>
                  <th>Chain</th>
                  <th>Last activity</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {cases.map((c) => {
                  const isActive = c.status === 'active';
                  const lastActivity = timeAgo(c.updated_at);

                  return (
                    <tr key={c.id} style={{ cursor: 'pointer' }}>
                      <td>
                        <div className="ref">
                          {c.reference_code}
                          <small>{c.title}</small>
                        </div>
                      </td>
                      <td><span className="tag">Lead</span></td>
                      <td className="num">&mdash;</td>
                      <td>
                        {isActive ? (
                          <div className="chain">
                            <span className="node on"></span>
                            <span className="seg"></span>
                            <span className="node on"></span>
                            <span className="seg"></span>
                            <span className="node on"></span>
                            <span className="seg"></span>
                            <span className="node on"></span>
                            <span className="seg"></span>
                            <span className="node on"></span>
                          </div>
                        ) : (
                          <span className="pl hold">Legal hold</span>
                        )}
                      </td>
                      <td className="mono">{lastActivity}</td>
                      <td className="actions">
                        <a className="linkarrow" href={`?caseId=${c.id}&view=overview`}>Open &rarr;</a>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          )}
        </div>

        {/* ---- Needs you ---- */}
        <div className="panel">
          <div className="panel-h">
            <h3>Needs you</h3>
            <span className="meta">5 items</span>
          </div>
          <div className="panel-body" style={{ padding: '14px 18px' }}>
            <div className="tl-list">
              <div className="tl-item accent">
                <div>
                  <div className="what"><strong>Countersign</strong> federated merge <em>VKE1&middot;91ab&hellip;</em> from <strong>CIJA</strong>.</div>
                  <div className="who-line">ICC-UKR-2024 &middot; 12 ops &middot; due today</div>
                </div>
                <div className="sig">14:21</div>
              </div>
              <div className="tl-item accent">
                <div>
                  <div className="what"><strong>Review</strong> redaction draft on <em>W-0144 statement</em>.</div>
                  <div className="who-line">Martyna &middot; 3 passages flagged</div>
                </div>
                <div className="sig">13:48</div>
              </div>
              <div className="tl-item">
                <div>
                  <div className="what"><strong>Approve</strong> disclosure package <em>DISC-2026-019</em> for defence counsel.</div>
                  <div className="who-line">Exports 48 exhibits &middot; Fri cutoff</div>
                </div>
                <div className="sig">11:02</div>
              </div>
              <div className="tl-item">
                <div>
                  <div className="what">Corroboration <em>C-0412</em> scored 0.78 &mdash; awaiting second analyst.</div>
                  <div className="who-line">Amir H. assigned</div>
                </div>
                <div className="sig">10:14</div>
              </div>
              <div className="tl-item">
                <div>
                  <div className="what">Inquiry log entry flagged: <em>provenance gap</em> in exhibit E-0918.</div>
                  <div className="who-line">Auto-detected &middot; 2 witnesses to contact</div>
                </div>
                <div className="sig">08:30</div>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* ---- Recent activity ---- */}
      <div className="panel">
        <div className="panel-h">
          <h3>Recent activity &middot; signed &amp; sealed</h3>
          <a className="linkarrow" href="?view=audit">Full audit log &rarr;</a>
        </div>
        <div className="panel-body" style={{ padding: '18px 22px' }}>
          <div className="tl-list">
            <div className="tl-item">
              <div>
                <div className="what"><em>Martyna</em> applied geo-fuzz (50km) to &ldquo;Andriivka&rdquo; in <strong>W-0144</strong></div>
                <div className="who-line">sig d9a7&hellip; &middot; ICC-UKR-2024 &middot; ts 14:21:11</div>
              </div>
              <div className="sig">2m</div>
            </div>
            <div className="tl-item">
              <div>
                <div className="what"><em>Amir H.</em> corrected timestamp 17:40 &rarr; 17:52 on W-0144</div>
                <div className="who-line">sig 2c14&hellip; &middot; signed Ed25519</div>
              </div>
              <div className="sig">3m</div>
            </div>
            <div className="tl-item">
              <div>
                <div className="what"><em>witness-node-02</em> countersigned block <strong>f208&hellip;</strong> &middot; 3 ops sealed</div>
                <div className="who-line">merge commit &middot; CRDT converged</div>
              </div>
              <div className="sig">4m</div>
            </div>
            <div className="tl-item">
              <div>
                <div className="what"><em>Juliane</em> pseudonymised &ldquo;Col. M.&rdquo; &rarr; <strong>S-038</strong></div>
                <div className="who-line">linked to 3 exhibits &middot; sig a1be&hellip;</div>
              </div>
              <div className="sig">6m</div>
            </div>
          </div>
        </div>
      </div>
    </>
  );
}

export { OverviewView };
