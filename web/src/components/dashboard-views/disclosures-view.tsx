import type { Disclosure } from '@/types';

export interface DisclosureWithCase extends Disclosure {
  case_reference: string;
  case_title: string;
  exhibit_count: number;
}

interface DisclosuresViewProps {
  disclosures: DisclosureWithCase[];
  caseRef?: string;
}

function formatDate(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleDateString('en-GB', {
    day: 'numeric',
    month: 'short',
  });
}

function computeStatusLabel(d: DisclosureWithCase): {
  label: string;
  cls: string;
} {
  const notes = (d.notes || '').toLowerCase();
  if (notes.includes('delivered') || notes.includes('sealed') || notes.includes('verified')) {
    return { label: 'sent', cls: 'pl sealed' };
  }
  if (notes.includes('review') || notes.includes('pending')) {
    return { label: 'review', cls: 'pl hold' };
  }
  return { label: 'draft', cls: 'pl draft' };
}

const AVATAR_CLASSES = ['a', 'b', 'c', 'd', 'e'];

export default function DisclosuresView({ disclosures, caseRef }: DisclosuresViewProps) {
  const total = disclosures.length;
  const pending = disclosures.filter(
    (d) => computeStatusLabel(d).label === 'review'
  ).length;
  const avgExhibits = total > 0
    ? Math.round(disclosures.reduce((sum, d) => sum + (d.exhibit_count || 0), 0) / total)
    : 0;
  const rejected = 0; // No rejection tracking yet

  // Nearest due disclosure
  const pendingDisclosures = disclosures.filter(d => computeStatusLabel(d).label === 'review');
  const nextDue = pendingDisclosures.length > 0
    ? pendingDisclosures[0]
    : null;

  return (
    <>
      <section className="d-pagehead">
        <div>
          <span className="eyebrow-m">
            {caseRef ? `Case \u00b7 ${caseRef} \u00b7 ` : ''}Berkeley Protocol Reporting
          </span>
          <h1>
            Disclosure <em>bundles</em>
          </h1>
          <p className="sub">
            A disclosure bundle is a one-click ZIP of exhibits, custody log,
            hash manifest, and (where required) redaction maps &mdash; plus
            the open validator binary so the recipient&apos;s clerk can verify
            offline, without talking to us.
          </p>
        </div>
        <div className="actions">
          <a className="btn ghost" href="#">
            Templates
          </a>
          <a className="btn" href="#">
            New disclosure <span className="arr">&rarr;</span>
          </a>
        </div>
      </section>

      <div className="d-kpis" style={{ marginBottom: 22 }}>
        <div className="d-kpi">
          <div className="k">This quarter</div>
          <div className="v">{total}</div>
          <div className="sub">across all cases</div>
        </div>
        <div className="d-kpi">
          <div className="k">Pending your review</div>
          <div className="v">{pending}</div>
          {nextDue && (
            <div className="delta n">
              {'\u25cf'} {nextDue.disclosed_to?.slice(0, 20) || 'Next'} due
            </div>
          )}
        </div>
        <div className="d-kpi">
          <div className="k">Avg. bundle</div>
          <div className="v">{avgExhibits}</div>
          <div className="sub">exhibits</div>
        </div>
        <div className="d-kpi">
          <div className="k">Rejected &middot; ever</div>
          <div className="v">{rejected}</div>
          <div className="sub">on custody grounds</div>
        </div>
      </div>

      <div className="g2-wide">
        <div className="panel">
          <div className="panel-h">
            <h3>All disclosures</h3>
            <span className="meta">{total} of {total}</span>
          </div>
          {total === 0 ? (
            <div className="panel-body">
              <p style={{ padding: '24px 16px', opacity: 0.6 }}>
                No disclosure packages found across your cases.
              </p>
            </div>
          ) : (
            <table className="tbl">
              <thead>
                <tr>
                  <th>Bundle</th>
                  <th>Recipient</th>
                  <th>Exhibits</th>
                  <th>Status</th>
                  <th>Due</th>
                  <th>Owners</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {disclosures.map((d, idx) => {
                  const status = computeStatusLabel(d);
                  const note = d.notes?.slice(0, 40) || '';
                  const ownerIndices = [idx % 5, (idx + 1) % 5].slice(0, Math.min(2, idx + 1));
                  return (
                    <tr key={d.id}>
                      <td>
                        <div className="ref">
                          {d.case_reference}
                          <small>{note}</small>
                        </div>
                      </td>
                      <td style={{ fontSize: 13, color: 'var(--ink-2)' }}>
                        {d.disclosed_to}
                      </td>
                      <td className="num">{d.exhibit_count}</td>
                      <td>
                        <span className={status.cls}>{status.label}</span>
                      </td>
                      <td className="mono">{formatDate(d.disclosed_at)}</td>
                      <td>
                        <span className="avs">
                          {ownerIndices.map((c) => (
                            <span key={c} className={`av ${AVATAR_CLASSES[c]}`}>
                              {AVATAR_CLASSES[c].toUpperCase()}
                            </span>
                          ))}
                        </span>
                      </td>
                      <td className="actions">
                        <a className="linkarrow" href="#">
                          Open &rarr;
                        </a>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          )}
        </div>

        {pendingDisclosures.length > 0 && (
          <div className="panel">
            <div className="panel-h">
              <h3>
                Bundle wizard &middot;{' '}
                <em>{pendingDisclosures[0].case_reference}</em>
              </h3>
              <span className="meta">in progress</span>
            </div>
            <div className="panel-body">
              <div style={{ display: 'flex', flexDirection: 'column', gap: 0 }}>
                {[
                  { t: '1 \u00b7 Scope', d: `${pendingDisclosures[0].exhibit_count} exhibits selected`, ok: true, cur: false },
                  { t: '2 \u00b7 Redactions', d: pendingDisclosures[0].redacted ? 'Redactions applied' : 'No redactions', ok: pendingDisclosures[0].redacted, cur: false },
                  { t: '3 \u00b7 Countersigns', d: 'Awaiting countersign', ok: false, cur: true },
                  { t: '4 \u00b7 Manifest', d: 'SHA-256 \u00b7 BLAKE3 \u00b7 RFC 3161 timestamp', ok: false, cur: false },
                  { t: '5 \u00b7 Deliver', d: 'Encrypted bundle + validator', ok: false, cur: false },
                ].map((s, i, a) => (
                  <div
                    key={s.t}
                    style={{
                      display: 'grid',
                      gridTemplateColumns: '28px 1fr',
                      gap: 14,
                      padding: '12px 0',
                      borderBottom: i < a.length - 1 ? '1px solid var(--line)' : 'none',
                    }}
                  >
                    <span
                      style={{
                        width: 24,
                        height: 24,
                        borderRadius: '50%',
                        border: `1.5px solid ${s.ok ? 'var(--ok)' : s.cur ? 'var(--accent)' : 'var(--line-2)'}`,
                        background: s.ok ? 'var(--ok)' : 'transparent',
                        color: s.ok ? '#fff' : s.cur ? 'var(--accent)' : 'var(--muted)',
                        display: 'grid',
                        placeItems: 'center',
                        fontSize: 11,
                        fontFamily: "'JetBrains Mono', monospace",
                      }}
                    >
                      {s.ok ? '\u2713' : i + 1}
                    </span>
                    <div>
                      <strong
                        style={{
                          fontFamily: "'Fraunces', serif",
                          fontSize: 15,
                          color: s.cur || s.ok ? 'var(--ink)' : 'var(--muted)',
                        }}
                      >
                        {s.t}
                      </strong>
                      <div
                        style={{
                          fontSize: 12.5,
                          color: 'var(--muted)',
                          marginTop: 3,
                        }}
                      >
                        {s.d}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
              <a
                className="btn"
                style={{
                  marginTop: 14,
                  width: '100%',
                  justifyContent: 'center',
                }}
                href="#"
              >
                Continue step 3 <span className="arr">&rarr;</span>
              </a>
            </div>
          </div>
        )}
      </div>
    </>
  );
}

export { DisclosuresView };
