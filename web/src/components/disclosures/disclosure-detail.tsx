'use client';

import type { Disclosure, EvidenceItem } from '@/types';

function formatBytes(bytes: number): string {
  if (!bytes || bytes === 0) return '\u2014';
  const units = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0)} ${units[i]}`;
}

export function DisclosureDetail({
  disclosure,
  evidence,
  onBack,
}: {
  disclosure: Disclosure;
  evidence: readonly EvidenceItem[];
  onBack: () => void;
}) {
  const disclosedItems = evidence.filter((e) =>
    disclosure.evidence_ids.includes(e.id)
  );

  const formattedDate = new Date(disclosure.disclosed_at).toLocaleDateString(
    'en-GB',
    { day: '2-digit', month: 'short', year: 'numeric' }
  );

  return (
    <>
      <section className="d-pagehead">
        <div>
          <span className="eyebrow-m">
            <a href="#" onClick={(e) => { e.preventDefault(); onBack(); }}>Disclosures</a>
            <span style={{ margin: '0 6px', color: 'var(--muted)' }}>/</span>
            Disclosure to {disclosure.disclosed_to}
          </span>
          <h1>Disclosure <em>detail</em></h1>
          <p className="sub">
            Disclosure is permanent and cannot be reversed. All access is logged
            and auditable.
          </p>
        </div>
        <div className="actions">
          <a className="btn ghost" href="#" onClick={(e) => { e.preventDefault(); onBack(); }}>
            &larr; Back
          </a>
        </div>
      </section>

      {/* Metadata */}
      <div className="panel" style={{ marginBottom: 22 }}>
        <div className="panel-h">
          <h3>Bundle details</h3>
          <span className="meta">{disclosedItems.length} items</span>
        </div>
        <div className="panel-body">
          <dl className="kvs">
            <dt>Recipient</dt>
            <dd><strong>{disclosure.disclosed_to}</strong></dd>
            <dt>Date</dt>
            <dd>{formattedDate}</dd>
            <dt>Items disclosed</dt>
            <dd>{disclosedItems.length}</dd>
            <dt>Redacted</dt>
            <dd>
              {disclosure.redacted ? (
                <span className="pl hold">redacted</span>
              ) : (
                <span className="pl sealed">full disclosure</span>
              )}
            </dd>
            {disclosure.notes && (
              <>
                <dt>Notes</dt>
                <dd>{disclosure.notes}</dd>
              </>
            )}
          </dl>
        </div>
      </div>

      {/* Disclosed evidence table */}
      <div className="panel">
        <div className="panel-h">
          <h3>Disclosed evidence</h3>
          <span className="meta">{disclosedItems.length} items</span>
        </div>
        {disclosedItems.length === 0 ? (
          <div className="panel-body">
            <p style={{ padding: '24px 16px', opacity: 0.6 }}>
              No evidence items linked to this disclosure.
            </p>
          </div>
        ) : (
          <table className="tbl">
            <thead>
              <tr>
                <th>File</th>
                <th>Reference</th>
                <th>Classification</th>
                <th>Size</th>
              </tr>
            </thead>
            <tbody>
              {disclosedItems.map((item) => (
                <tr key={item.id}>
                  <td style={{ color: 'var(--ink)' }}>
                    {item.title || item.original_name}
                  </td>
                  <td className="mono" style={{ fontSize: '12px' }}>
                    {item.evidence_number}
                  </td>
                  <td>
                    <span className={`pl ${item.classification === 'public' ? 'sealed' : 'hold'}`}>
                      {item.classification.replace('_', ' ')}
                    </span>
                  </td>
                  <td className="mono">{formatBytes(item.size_bytes)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </>
  );
}
