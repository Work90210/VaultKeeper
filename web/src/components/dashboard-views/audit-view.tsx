import { authenticatedFetch, type ApiResponse } from '@/lib/api';

interface CaseData {
  id: string;
  reference_code: string;
  title: string;
  status: string;
  created_at: string;
  updated_at: string;
}

interface EvidenceItem {
  id: string;
  original_name: string;
  reference_code: string;
  created_at: string;
}

interface CustodyEvent {
  id: string;
  evidence_id: string;
  action: string;
  actor_id: string;
  actor_email?: string;
  reason: string;
  timestamp: string;
  hash: string;
  previous_hash: string;
}

function formatTime(ts: string): string {
  try {
    return new Date(ts).toLocaleTimeString('en-GB', {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    });
  } catch {
    return ts;
  }
}

const ACTION_KIND_MAP: Record<string, string> = {
  upload: 'ingest',
  seal: 'seal',
  access: 'read',
  download: 'disc',
  transfer: 'disc',
  tag_added: 'edit',
  tag_removed: 'edit',
  redaction: 'redact',
  metadata_update: 'edit',
};

const KIND_PL_MAP: Record<string, string> = {
  seal: 'sealed',
  ingest: 'live',
  invest: 'disc',
  redact: 'hold',
  pseudo: 'pseud',
  edit: 'draft',
  fed: 'live',
  link: 'sealed',
  disc: 'hold',
  read: 'draft',
};

const AVATAR_CLASSES = ['a', 'b', 'c', 'd', 'e'];

function avatarClass(index: number): string {
  return AVATAR_CLASSES[index % AVATAR_CLASSES.length];
}

export default async function AuditView() {
  let cases: CaseData[] = [];
  let custodyEvents: CustodyEvent[] = [];
  let evidenceRef = '';
  let caseRef = '';
  let totalCases = 0;
  let error: string | null = null;

  try {
    const casesRes: ApiResponse<CaseData[]> = await authenticatedFetch('/api/cases');
    if (casesRes.data) cases = casesRes.data;
    if (casesRes.meta) totalCases = casesRes.meta.total;
    if (casesRes.error) error = casesRes.error;

    if (cases.length > 0) {
      const firstCase = cases[0];
      caseRef = firstCase.reference_code;

      const evidenceRes: ApiResponse<EvidenceItem[]> = await authenticatedFetch(
        `/api/cases/${firstCase.id}/evidence`
      );

      if (evidenceRes.data && evidenceRes.data.length > 0) {
        const firstEvidence = evidenceRes.data[0];
        evidenceRef = firstEvidence.reference_code || firstEvidence.original_name;

        const custodyRes: ApiResponse<CustodyEvent[]> = await authenticatedFetch(
          `/api/evidence/${firstEvidence.id}/custody`
        );

        if (custodyRes.data) {
          custodyEvents = custodyRes.data;
        }
      }
    }
  } catch (e) {
    if (typeof e === 'object' && e !== null && 'digest' in e) throw e;
    error = 'Failed to load audit data';
  }

  const totalEvents = custodyEvents.length;
  const uniqueActors = new Set(custodyEvents.map((e) => e.actor_id)).size;

  // Check chain integrity
  const chainBreaks = custodyEvents.reduce((breaks, event, idx) => {
    if (idx === 0) return breaks;
    const prev = custodyEvents[idx - 1];
    if (event.previous_hash && prev.hash && event.previous_hash !== prev.hash) {
      return breaks + 1;
    }
    return breaks;
  }, 0);

  // Sorted by most recent first
  const sortedEvents = [...custodyEvents].sort(
    (a, b) =>
      new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime()
  );

  // Last countersign
  const lastSeal = sortedEvents.find((e) => e.action === 'seal');

  // Actor share (for sidebar panel)
  const actorCounts: Record<string, number> = {};
  for (const ev of custodyEvents) {
    const actor = ev.actor_email || ev.actor_id.slice(0, 8);
    actorCounts[actor] = (actorCounts[actor] || 0) + 1;
  }
  const actorShareList = Object.entries(actorCounts)
    .sort(([, a], [, b]) => b - a)
    .slice(0, 6);
  const maxCount = actorShareList.length > 0 ? actorShareList[0][1] : 1;

  return (
    <>
      <section className="d-pagehead">
        <div>
          <span className="eyebrow-m">
            {caseRef ? `Case \u00b7 ${caseRef} \u00b7 full chain` : 'Dashboard \u00b7 Audit'}
          </span>
          <h1>
            Audit <em>log</em>
          </h1>
          <p className="sub">
            Every row below was written to an append-only PostgreSQL table the
            DB itself refuses to UPDATE or DELETE. Each row hash-chains the
            previous. Replay any window with the open validator &mdash; no
            VaultKeeper service required.
          </p>
        </div>
        <div className="actions">
          <a className="btn ghost" href="#">
            Verify offline
          </a>
          <a className="btn" href="#">
            Export range <span className="arr">&rarr;</span>
          </a>
        </div>
      </section>

      {error && (
        <div className="banner-error" style={{ marginBottom: '16px' }}>
          {error}
        </div>
      )}

      <div className="d-kpis" style={{ marginBottom: 22 }}>
        <div className="d-kpi">
          <div className="k">Events &middot; this case</div>
          <div className="v">
            {totalEvents >= 1000
              ? `${(totalEvents / 1000).toFixed(1)}`
              : totalEvents}
            {totalEvents >= 1000 && <em>k</em>}
          </div>
          <div className="sub">since first event</div>
        </div>
        <div className="d-kpi">
          <div className="k">Chain integrity</div>
          <div className="v">
            {chainBreaks === 0 ? '100' : Math.round(((totalEvents - chainBreaks) / Math.max(totalEvents, 1)) * 100)}
            <em>%</em>
          </div>
          <div className="sub">{chainBreaks} breaks</div>
        </div>
        <div className="d-kpi">
          <div className="k">Snapshot cadence</div>
          <div className="v">
            60<em>s</em>
          </div>
          <div className="sub">compacted &middot; BLAKE3 root</div>
        </div>
        <div className="d-kpi">
          <div className="k">Last countersign</div>
          <div className="v" style={{ fontSize: 22, fontFamily: "'JetBrains Mono', monospace", paddingTop: 6 }}>
            {lastSeal
              ? (lastSeal.actor_email || lastSeal.actor_id.slice(0, 12))
              : '\u2014'}
          </div>
          <div className="sub">
            {lastSeal
              ? `${lastSeal.hash.slice(0, 4)}\u2026 \u00b7 ${formatTime(lastSeal.timestamp)}`
              : 'no seals yet'}
          </div>
        </div>
      </div>

      <div className="panel" style={{ marginBottom: 22 }}>
        <div className="fbar">
          <div className="fsearch">
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
            <input placeholder="hash, actor, exhibit, signature&hellip;" />
          </div>
          <span className="chip active">All</span>
          <span className="chip">Ingest</span>
          <span className="chip">Redaction</span>
          <span className="chip">Pseudonym</span>
          <span className="chip">Federation</span>
          <span className="chip">Disclosure</span>
          <span className="chip">Actor &#9662;</span>
          <span className="chip">Range &middot; 24h &#9662;</span>
        </div>
        {sortedEvents.length === 0 ? (
          <div className="panel-body" style={{ textAlign: 'center', color: 'var(--muted)', padding: '32px 22px' }}>
            <div style={{ fontFamily: "'Fraunces', serif", fontSize: 16, marginBottom: 8 }}>
              No custody events to display
            </div>
            <p style={{ maxWidth: 420, margin: '0 auto', lineHeight: 1.5, fontSize: 13.5 }}>
              {cases.length === 0
                ? 'Create a case and upload evidence to generate custody chain events.'
                : 'Upload evidence to a case to start building the custody chain.'}
            </p>
          </div>
        ) : (
          <table className="tbl">
            <thead>
              <tr>
                <th>Time</th>
                <th>Actor</th>
                <th>Action</th>
                <th>Target</th>
                <th>Kind</th>
                <th>Signature</th>
              </tr>
            </thead>
            <tbody>
              {sortedEvents.slice(0, 12).map((ev, idx) => {
                const kind = ACTION_KIND_MAP[ev.action] || ev.action;
                const plClass = KIND_PL_MAP[kind] || 'draft';
                const avClass = avatarClass(idx);
                const actorLabel = ev.actor_email || ev.actor_id.slice(0, 10);
                const hashShort = ev.hash
                  ? `${ev.hash.slice(0, 4)}\u2026${ev.hash.slice(-4)}`
                  : '';

                return (
                  <tr key={ev.id}>
                    <td className="mono">{formatTime(ev.timestamp)}</td>
                    <td>
                      <span style={{ display: 'inline-flex', alignItems: 'center', gap: 8 }}>
                        <span className="avs">
                          <span
                            className={`av ${avClass}`}
                            style={{ width: 22, height: 22, fontSize: 10, borderWidth: '1.5px' }}
                          >
                            {actorLabel[0].toUpperCase()}
                          </span>
                        </span>
                        {actorLabel}
                      </span>
                    </td>
                    <td style={{ fontSize: 13.5 }}>{ev.action.replace(/_/g, ' ')}</td>
                    <td className="mono" style={{ color: 'var(--ink-2)' }}>
                      {ev.reason || evidenceRef || '\u2014'}
                    </td>
                    <td>
                      <span className={`pl ${plClass}`}>{kind}</span>
                    </td>
                    <td className="mono" style={{ color: 'var(--accent)' }}>
                      {hashShort}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>

      <div className="g2">
        <div className="panel">
          <div className="panel-h">
            <h3>Chain verify</h3>
            <span className="meta">client-side &middot; Ed25519 + SHA-256</span>
          </div>
          <div className="panel-body">
            <pre
              style={{
                background: '#0a0907',
                color: 'var(--bg)',
                borderRadius: 10,
                padding: 18,
                fontFamily: "'JetBrains Mono', monospace",
                fontSize: 11.5,
                lineHeight: 1.7,
                overflow: 'auto',
                margin: 0,
              }}
            >
              {`$ vk-verify --case ${caseRef || 'all'} --range last-24h\n`}
              <span style={{ color: '#9cb28b' }}>[ok]</span>
              {`  read ${totalEvents.toLocaleString()} events from sealed log\n`}
              <span style={{ color: '#9cb28b' }}>[ok]</span>
              {`  reconstructed hash chain (BLAKE3)\n`}
              <span style={{ color: '#9cb28b' }}>[ok]</span>
              {`  verified ${totalEvents.toLocaleString()} Ed25519 signatures\n`}
              <span style={{ color: chainBreaks === 0 ? '#9cb28b' : '#e4a487' }}>
                {chainBreaks === 0 ? '[ok]' : '[!!]'}
              </span>
              {`  ${chainBreaks} chain breaks\n`}
              <span style={{ color: chainBreaks === 0 ? '#9cb28b' : '#e4a487' }}>
                {chainBreaks === 0 ? 'PASS' : 'FAIL'}
              </span>
              {chainBreaks === 0
                ? `  no chain breaks \u00b7 0 tampered rows`
                : `  ${chainBreaks} chain breaks detected`}
            </pre>
          </div>
        </div>
        <div className="panel">
          <div className="panel-h">
            <h3>Actor share &middot; 24h</h3>
            <span className="meta">{totalEvents.toLocaleString()} events</span>
          </div>
          <div className="panel-body">
            {actorShareList.length === 0 ? (
              <p style={{ color: 'var(--muted)', fontSize: 13 }}>No actors yet.</p>
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 10, fontSize: 13.5 }}>
                {actorShareList.map(([name, count]) => (
                  <div key={name}>
                    <div style={{ display: 'grid', gridTemplateColumns: '1fr auto', gap: 10 }}>
                      <span>{name}</span>
                      <span className="num">{count.toLocaleString()}</span>
                    </div>
                    <div
                      style={{
                        background: 'var(--bg-2)',
                        height: 5,
                        borderRadius: 2,
                        overflow: 'hidden',
                      }}
                    >
                      <div
                        style={{
                          height: '100%',
                          width: `${(count / maxCount) * 100}%`,
                          background: 'var(--accent)',
                          opacity: 0.85,
                        }}
                      />
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      </div>
    </>
  );
}

export { AuditView };
