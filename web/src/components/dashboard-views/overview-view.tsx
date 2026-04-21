import { authenticatedFetch } from '@/lib/api';
import { getProfile } from '@/lib/org-api';
import type { Case } from '@/types';
import { KPIStrip, Panel, Timeline, Tag, StatusPill, LinkArrow, EyebrowLabel } from '@/components/ui/dashboard';

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
  return new Date().toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false });
}

function getFirstName(displayName: string): string {
  return displayName.split(' ')[0] || displayName;
}

function timeAgo(dateStr: string): string {
  const diffMs = Date.now() - new Date(dateStr).getTime();
  const diffMin = Math.floor(diffMs / 60000);
  const diffH = Math.floor(diffMs / 3600000);
  const diffD = Math.floor(diffMs / 86400000);
  if (diffMin < 1) return 'Just now';
  if (diffMin < 60) return `${diffMin} min ago`;
  if (diffH < 24) return `${diffH}h ago`;
  if (diffD === 1) return 'Yesterday';
  if (diffD < 7) return `${diffD} d ago`;
  return new Date(dateStr).toLocaleDateString('en-GB', { day: 'numeric', month: 'short' });
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

/* ─── Stub "needs you" items (Sprint E will connect to real API) ─── */
const NEEDS_YOU_ITEMS = [
  { content: <><strong>Countersign</strong> federated merge <em>VKE1&middot;91ab&hellip;</em> from <strong>CIJA</strong>.</>, subline: 'ICC-UKR-2024 \u00b7 12 ops \u00b7 due today', time: '14:21', accent: true },
  { content: <><strong>Review</strong> redaction draft on <em>W-0144 statement</em>.</>, subline: 'Martyna \u00b7 3 passages flagged', time: '13:48', accent: true },
  { content: <><strong>Approve</strong> disclosure package <em>DISC-2026-019</em> for defence counsel.</>, subline: 'Exports 48 exhibits \u00b7 Fri cutoff', time: '11:02', accent: false },
  { content: <>Corroboration <em>C-0412</em> scored 0.78 &mdash; awaiting second analyst.</>, subline: 'Amir H. assigned', time: '10:14', accent: false },
  { content: <>Inquiry log entry flagged: <em>provenance gap</em> in exhibit E-0918.</>, subline: 'Auto-detected \u00b7 2 witnesses to contact', time: '08:30', accent: false },
];

const RECENT_ACTIVITY = [
  { content: <><em>Martyna</em> applied geo-fuzz (50km) to &ldquo;Andriivka&rdquo; in <strong>W-0144</strong></>, subline: 'sig d9a7\u2026 \u00b7 ICC-UKR-2024 \u00b7 ts 14:21:11', time: '2m' },
  { content: <><em>Amir H.</em> corrected timestamp 17:40 &rarr; 17:52 on W-0144</>, subline: 'sig 2c14\u2026 \u00b7 signed Ed25519', time: '3m' },
  { content: <><em>witness-node-02</em> countersigned block <strong>f208&hellip;</strong> &middot; 3 ops sealed</>, subline: 'merge commit \u00b7 CRDT converged', time: '4m' },
  { content: <><em>Juliane</em> pseudonymised &ldquo;Col. M.&rdquo; &rarr; <strong>S-038</strong></>, subline: 'linked to 3 exhibits \u00b7 sig a1be\u2026', time: '6m' },
];

/* ─── Berkeley Protocol Compliance Panel ─── */
function BerkeleyCompliancePanel() {
  return (
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
              <div key={p.n} style={{ padding: '14px 16px', borderRight: '1px solid var(--line)', textAlign: 'center' }}>
                <div style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: '9.5px', letterSpacing: '.08em', textTransform: 'uppercase', color: 'var(--muted)', marginBottom: 8 }}>{p.n}</div>
                <div style={{ fontFamily: 'Fraunces, serif', fontSize: 28, letterSpacing: '-.02em', color: col }}>{p.pct}<span style={{ fontSize: 14, color: 'var(--muted)' }}>%</span></div>
                <div style={{ height: 4, background: 'var(--bg-2)', borderRadius: 2, overflow: 'hidden', margin: '8px auto 6px', maxWidth: 80 }}>
                  <div style={{ height: '100%', width: `${p.pct}%`, background: col, borderRadius: 2 }} />
                </div>
              </div>
            );
          })}
        </div>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', paddingTop: 14, borderTop: '1px solid var(--line)' }}>
          <div style={{ fontSize: 13, color: 'var(--muted)' }}>
            <strong style={{ color: 'var(--ink)' }}>4,108 exhibits</strong> need attention &mdash; most missing Phase 5 (verification) or Phase 6 (analysis).
          </div>
          <LinkArrow href="?view=evidence">View by phase gap</LinkArrow>
        </div>
      </div>
    </div>
  );
}

/* ─── Case-Scoped View ─── */
function CaseScopedOverview({ caseData, firstName: _firstName }: { caseData: Case; firstName: string }) {
  const statusMap: Record<string, 'active' | 'hold' | 'sealed' | 'draft'> = {
    active: 'active',
    hold: 'hold',
    closed: 'sealed',
    archived: 'sealed',
  };
  const pillStatus = statusMap[caseData.status] || 'active';

  return (
    <>
      <section className="d-pagehead">
        <div>
          <EyebrowLabel>Case workspace &middot; Lead</EyebrowLabel>
          <h1><em>{caseData.reference_code}</em></h1>
          <p className="sub">{caseData.title} &middot; <StatusPill status={pillStatus} /></p>
        </div>
        <div className="actions">
          <a className="btn ghost" href="?view=evidence">
            Evidence <span style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: 11, opacity: 0.6, marginLeft: 4 }}>&mdash;</span>
          </a>
          <a className="btn" href="?view=evidence">Upload evidence <span className="arr">&rarr;</span></a>
        </div>
      </section>

      <KPIStrip items={[
        { label: 'Exhibits', value: '\u2014', sub: 'sealed on chain' },
        { label: 'Awaiting action', value: '0', sub: 'unassessed + pending' },
        { label: 'Through all 6 phases', value: '0', sub: 'fully compliant' },
        { label: 'Open inquiries', value: '0', sub: 'active entries' },
      ]} />

      <div style={{ marginBottom: 22 }} />

      <BerkeleyCompliancePanel />

      <div className="g2-wide" style={{ marginBottom: 22 }}>
        <Panel title="Recent activity">
          <div style={{ padding: '14px 18px' }}>
            <Timeline items={RECENT_ACTIVITY.slice(0, 3)} />
          </div>
        </Panel>

        <Panel title="Needs you">
          <div style={{ padding: '14px 18px' }}>
            <Timeline items={NEEDS_YOU_ITEMS.filter((n) => n.accent).slice(0, 3)} />
            {NEEDS_YOU_ITEMS.filter((n) => n.accent).length === 0 && (
              <p style={{ color: 'var(--muted)', fontSize: 13, padding: '8px 0' }}>
                Nothing pending &mdash; you&rsquo;re all caught up.
              </p>
            )}
          </div>
        </Panel>
      </div>

      {/* Case team */}
      <Panel title="Case team" meta="4 members">
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))', gap: 16 }}>
          {[
            { n: 'H\u00e9l\u00e8ne Morel', r: 'Lead Investigator', av: 'H', col: '#c87e5e' },
            { n: 'Martyna Kovacs', r: 'Redaction Analyst', av: 'M', col: '#4a6b3a' },
            { n: 'Amir Haddad', r: 'Evidence Technician', av: 'A', col: '#3a4a6b' },
            { n: 'Juliane Wirth', r: 'Witness Liaison', av: 'J', col: '#6b3a4a' },
          ].map((m) => (
            <div key={m.n} style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '8px 0' }}>
              <span style={{
                width: 36, height: 36, borderRadius: '50%', background: m.col, color: '#fff',
                display: 'grid', placeItems: 'center', fontFamily: 'Fraunces, serif', fontStyle: 'italic', fontSize: 15, flexShrink: 0,
              }}>{m.av}</span>
              <div>
                <div style={{ fontSize: 14, fontWeight: 500 }}>{m.n}</div>
                <div style={{ fontSize: 12, color: 'var(--muted)' }}>{m.r}</div>
              </div>
            </div>
          ))}
        </div>
      </Panel>
    </>
  );
}

/* ─── All-Cases View (main) ─── */
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

  if (casesRes.error) {
    return (
      <div className="banner-error" style={{ marginBottom: 16 }}>
        {casesRes.error}
      </div>
    );
  }

  const activeCases = cases.filter((c) => c.status === 'active');
  const holdCases = cases.filter((c) => c.status !== 'active');
  const greeting = getGreeting();
  const eyebrow = formatEyebrowDate();
  const nowTime = formatNowTime();
  const activeCaseCount = activeCases.length;

  const activeCaseWord = activeCaseCount === 1
    ? 'One case active'
    : activeCaseCount === 0
      ? 'No cases active'
      : `${activeCaseCount} case${activeCaseCount > 1 ? 's' : ''} active`;

  return (
    <>
      {/* Page header */}
      <section className="d-pagehead">
        <div>
          <EyebrowLabel>{eyebrow}</EyebrowLabel>
          <h1>{greeting}, <em>{firstName}.</em></h1>
          <p className="sub">
            {activeCaseWord} across the workspace. Two disclosures are due this week, and a federated merge from CIJA is waiting for your countersign on ICC-UKR-2024.
          </p>
        </div>
        <div className="actions">
          <a className="btn ghost" href="?view=cases">All cases</a>
          <a className="btn ghost" href="?view=investigation">New investigation</a>
          <a className="btn" href="?view=evidence">Upload evidence <span className="arr">&rarr;</span></a>
        </div>
      </section>

      {/* KPI strip */}
      <KPIStrip items={[
        { label: 'Exhibits sealed \u00b7 today', value: '142', delta: '\u25B2 18 vs. yesterday' },
        { label: 'Awaiting action', value: '847', delta: '\u25CF need assessment or verification', deltaNegative: true },
        { label: 'Fully through protocol', value: '6,102', sub: 'all 6 phases complete' },
        { label: 'Last RFC 3161 stamp', value: nowTime, sub: 'ts-eu-west \u00b7 signed \u2713' },
      ]} />

      <div style={{ marginBottom: 28 }} />

      {/* Berkeley Protocol compliance */}
      <BerkeleyCompliancePanel />

      {/* Cases in flight + Needs you */}
      <div className="g2-wide" style={{ marginBottom: 22 }}>
        <Panel title="Your cases" titleAccent="in flight" meta={<>
          {activeCaseCount} active
          {holdCases.length > 0 && ` \u00b7 ${holdCases.length} on hold`}
        </>}>
          {cases.length === 0 ? (
            <p style={{ color: 'var(--muted)', fontSize: 13 }}>
              No cases yet. Create your first case to get started.
            </p>
          ) : (
            <table className="tbl">
              <thead>
                <tr>
                  <th>Case</th>
                  <th>Role</th>
                  <th>Exhibits</th>
                  <th>BP progress</th>
                  <th>Last activity</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {cases.map((c) => (
                  <tr key={c.id} style={{ cursor: 'pointer' }}>
                    <td>
                      <div className="ref">
                        {c.reference_code}
                        <small>{c.title}</small>
                      </div>
                    </td>
                    <td><Tag>Lead</Tag></td>
                    <td className="num">&mdash;</td>
                    <td>
                      {c.status === 'active' ? (
                        <div>
                          <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 12 }}>
                            <span style={{ color: 'var(--ok)', fontWeight: 500 }}>72%</span>
                            <span style={{ color: 'var(--muted)' }}>8.9k done</span>
                          </div>
                          <div style={{ height: 4, background: 'var(--bg-2)', borderRadius: 2, overflow: 'hidden', margin: '4px 0', maxWidth: 120 }}>
                            <div style={{ height: '100%', width: '72%', background: 'var(--accent)', borderRadius: 2 }} />
                          </div>
                          <div style={{ fontSize: 11, color: 'var(--muted)' }}>3.5k need action</div>
                        </div>
                      ) : (
                        <StatusPill status="hold">Legal hold</StatusPill>
                      )}
                    </td>
                    <td className="mono">{timeAgo(c.updated_at)}</td>
                    <td className="actions">
                      <LinkArrow href={`?caseId=${c.id}&view=overview`}>Open</LinkArrow>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </Panel>

        <Panel title="Needs you" meta="5 items">
          <div style={{ padding: '14px 18px' }}>
            <Timeline items={NEEDS_YOU_ITEMS} />
          </div>
        </Panel>
      </div>

      {/* Recent activity */}
      <Panel
        title="Recent activity &middot; signed & sealed"
        headerRight={<LinkArrow href="?view=audit">Full audit log</LinkArrow>}
      >
        <div style={{ padding: '18px 22px' }}>
          <Timeline items={RECENT_ACTIVITY} />
        </div>
      </Panel>
    </>
  );
}

export { OverviewView, CaseScopedOverview };
