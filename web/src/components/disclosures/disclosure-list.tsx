'use client';

import type { Disclosure } from '@/types';

export function DisclosureList({
  disclosures,
  onSelect,
  onCreateNew,
  canCreate,
}: {
  disclosures: readonly Disclosure[];
  onSelect: (id: string) => void;
  onCreateNew?: () => void;
  canCreate: boolean;
}) {
  return (
    <div className="panel">
      <div className="panel-h">
        <h3>Disclosures</h3>
        <span className="meta">{disclosures.length} total</span>
      </div>
      {canCreate && onCreateNew && (
        <div className="fbar">
          <span style={{ marginLeft: 'auto' }}>
            <button onClick={onCreateNew} className="btn">
              Create Disclosure <span className="arr">&rarr;</span>
            </button>
          </span>
        </div>
      )}
      {disclosures.length === 0 ? (
        <div className="panel-body" style={{ textAlign: 'center', padding: '40px 16px' }}>
          <p style={{ fontFamily: "'Fraunces', serif", fontSize: 20, color: 'var(--muted)', marginBottom: 6 }}>
            No disclosures
          </p>
          <p style={{ fontSize: '13.5px', color: 'var(--muted)' }}>
            Evidence disclosures to defence or other parties will appear here.
          </p>
        </div>
      ) : (
        <table className="tbl">
          <thead>
            <tr>
              <th>Date</th>
              <th>Recipient</th>
              <th>Items</th>
              <th>Redacted</th>
              <th>Notes</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {disclosures.map((d) => (
              <tr key={d.id} onClick={() => onSelect(d.id)} style={{ cursor: 'pointer' }}>
                <td style={{ color: 'var(--ink)' }}>
                  {new Date(d.disclosed_at).toLocaleDateString('en-GB', { day: 'numeric', month: 'short' })}
                </td>
                <td style={{ fontSize: '13px', color: 'var(--ink-2)' }}>
                  {d.disclosed_to}
                </td>
                <td className="mono" style={{ color: 'var(--accent)' }}>
                  {d.evidence_ids.length}
                </td>
                <td>
                  {d.redacted ? (
                    <span className="pl hold">redacted</span>
                  ) : (
                    <span className="chip">full</span>
                  )}
                </td>
                <td style={{ fontSize: '13px', color: 'var(--muted)', maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {d.notes || '\u2014'}
                </td>
                <td className="actions">
                  <a className="linkarrow" href="#" onClick={(e) => { e.preventDefault(); onSelect(d.id); }}>
                    Open &rarr;
                  </a>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
