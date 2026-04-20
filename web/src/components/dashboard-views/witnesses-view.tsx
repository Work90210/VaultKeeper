import type { Witness } from '@/types';

export interface WitnessesViewProps {
  witnesses: Witness[];
  total: number;
}

const riskClassMap: Record<string, string> = {
  standard: 'pl sealed',
  protected: 'pl disc',
  high_risk: 'pl hold',
};

const riskLabelMap: Record<string, string> = {
  standard: 'standard',
  protected: 'protected',
  high_risk: 'high-risk',
};

function formatAge(createdAt: string): string {
  const created = new Date(createdAt);
  const now = new Date();
  const diffMs = now.getTime() - created.getTime();
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));
  if (diffDays === 0) return '<1 d';
  return `${diffDays} d`;
}

export default function WitnessesView({ witnesses, total }: WitnessesViewProps) {
  const pseudonymisedCount = witnesses.filter((w) => !w.identity_visible).length;
  const clearedCount = witnesses.filter((w) => w.identity_visible).length;
  const highRiskCount = witnesses.filter((w) => w.protection_status === 'high_risk').length;
  const protectedCount = witnesses.filter((w) => w.protection_status === 'protected').length;

  return (
    <>
      <section className="d-pagehead">
        <div>
          <span className="eyebrow-m">Workspace &middot; witness-sensitive</span>
          <h1>Witness <em>register</em></h1>
          <p className="sub">Every record is sealed with AES-256-GCM application-level encryption. Defence counsel see pseudonyms only; the mapping lives in a separate vault sealed by a two-of-three key ceremony. Break-the-glass unsealing is itself a sealed event.</p>
        </div>
        <div className="actions">
          <a className="btn ghost" href="#">Ceremony log</a>
          <a className="btn" href="#">Add witness <span className="arr">&rarr;</span></a>
        </div>
      </section>

      <div className="d-kpis" style={{ marginBottom: 22 }}>
        <div className="d-kpi"><div className="k">Total protected</div><div className="v">{total}</div><div className="sub">{pseudonymisedCount} pseudonymised &middot; {clearedCount} cleared</div></div>
        <div className="d-kpi"><div className="k">Duress armed</div><div className="v">{highRiskCount}</div><div className="sub">decoy vault ready</div></div>
        <div className="d-kpi"><div className="k">Voice-masked</div><div className="v">{protectedCount + highRiskCount}</div><div className="sub">real-time DSP pipeline</div></div>
        <div className="d-kpi"><div className="k">Break-the-glass &middot; 90 d</div><div className="v">{total > 0 ? 2 : 0}</div><div className="delta n">&#9679; both countersigned</div></div>
      </div>

      <div className="panel">
        <div className="fbar">
          <div className="fsearch">
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" style={{ color: "var(--muted)" }}>
              <circle cx="7" cy="7" r="4" />
              <path d="M10 10l3 3" />
            </svg>
            <input placeholder="pseudonym, linked exhibit, intermediary&#8230;" />
          </div>
          <span className="chip active">All <span className="x">&middot;{total}</span></span>
          <span className="chip">High-risk <span className="chev">{highRiskCount}</span></span>
          <span className="chip">Duress-armed <span className="chev">{highRiskCount}</span></span>
          <span className="chip">Voice-masked <span className="chev">{protectedCount + highRiskCount}</span></span>
          <span className="chip">Intermediary &middot; any <span className="chev">&#9662;</span></span>
        </div>
        <table className="tbl">
          <thead>
            <tr>
              <th>Pseudonym</th>
              <th>Risk</th>
              <th>Intake</th>
              <th>Exhibits</th>
              <th>Corrob. score</th>
              <th>Voice</th>
              <th>Signature</th>
              <th>Age</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {witnesses.length === 0 && (
              <tr>
                <td colSpan={9} style={{ textAlign: 'center', padding: '32px 0', color: 'var(--muted)' }}>
                  No witnesses found.
                </td>
              </tr>
            )}
            {witnesses.map((w) => (
              <tr key={w.id}>
                <td><div className="ref">{w.witness_code}<small>{w.identity_visible ? 'Cleared name' : 'Pseudonymised'}</small></div></td>
                <td><span className={riskClassMap[w.protection_status] ?? 'pl sealed'}>{riskLabelMap[w.protection_status] ?? w.protection_status}</span></td>
                <td style={{ fontSize: 13, color: "var(--muted)" }}>{w.statement_summary ? w.statement_summary.slice(0, 40) : '\u2014'}</td>
                <td className="num">{w.related_evidence.length > 0 ? `${w.related_evidence.length} exhibit${w.related_evidence.length !== 1 ? 's' : ''}` : '\u2014'}</td>
                <td><span className="tag">&mdash;</span></td>
                <td><span className="tag">{w.protection_status === 'high_risk' ? 'voice-masked' : '\u2014'}</span></td>
                <td><span className={`pl ${w.protection_status === 'high_risk' ? 'broken' : 'sealed'}`}>{w.protection_status === 'high_risk' ? 'duress-armed' : 'signed'}</span></td>
                <td className="mono">{formatAge(w.created_at)}</td>
                <td className="actions"><a className="linkarrow" href={`/en/cases/${w.case_id}?tab=witnesses`}>Open &rarr;</a></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="g2" style={{ marginTop: 22 }}>
        <div className="panel">
          <div className="panel-h"><h3>Duress &amp; break-the-glass</h3><span className="meta">last 30 events</span></div>
          <div className="panel-body">
            <div className="tl-list">
              <div className="tl-item accent">
                <div>
                  <div className="what"><em>Juliane</em> unsealed <strong>W-0140</strong> real-name for notary transmission</div>
                  <div className="who-line">countersigned by H. Morel &middot; reason: cross-border warrant</div>
                </div>
                <div className="sig">2 d</div>
              </div>
              <div className="tl-item">
                <div>
                  <div className="what">Decoy vault opened for <strong>W-0137</strong> &mdash; phishing probe detected</div>
                  <div className="who-line">origin: <code>45.61.x.x</code> &middot; auto-blocked</div>
                </div>
                <div className="sig">6 d</div>
              </div>
              <div className="tl-item">
                <div>
                  <div className="what">Ceremony: key rotation &middot; <strong>2-of-3 quorum</strong> reached</div>
                  <div className="who-line">Morel &middot; Nyoka &middot; H&auml;mmerli</div>
                </div>
                <div className="sig">11 d</div>
              </div>
              <div className="tl-item">
                <div>
                  <div className="what"><em>Amir H.</em> re-pseudonymised <strong>W-0139</strong> on legal officer request</div>
                  <div className="who-line">mapping re-sealed &middot; 3 exhibits re-linked</div>
                </div>
                <div className="sig">14 d</div>
              </div>
            </div>
          </div>
        </div>

        <div className="panel">
          <div className="panel-h"><h3>Protection posture</h3><span className="meta">posture v3</span></div>
          <div className="panel-body">
            <dl className="kvs">
              <dt>Mapping vault</dt><dd>Sealed by <strong>2-of-3</strong> quorum (Morel / Nyoka / H&auml;mmerli). Rotates every 90 days.</dd>
              <dt>Default intake</dt><dd>Pseudonym auto-assigned at first save. Real name never reaches disk without quorum.</dd>
              <dt>Geo-fuzzing</dt><dd>50 km default radius on mentioned locations; tunable per-witness.</dd>
              <dt>Duress passphrase</dt><dd>Opens a decoy vault containing fabricated statements. Event sealed.</dd>
              <dt>Voice masking</dt><dd>Real-time DSP pitch-shift + formant flatten on all pseudonymised recordings.</dd>
              <dt>Defence access</dt><dd>Pseudonymised view only. Real-name requests require judicial order + ceremony.</dd>
            </dl>
          </div>
        </div>
      </div>
    </>
  );
}
