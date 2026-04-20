'use client';

import type { Witness } from '@/types';

const PROTECTION_PILL: Record<string, { cls: string; label: string }> = {
  standard: { cls: 'pl sealed', label: 'Standard' },
  protected: { cls: 'pl hold', label: 'Protected' },
  high_risk: { cls: 'pl broken', label: 'High Risk' },
};

export function WitnessList({
  witnesses,
  onSelect,
  onAddNew,
  canEdit,
}: {
  witnesses: readonly Witness[];
  onSelect: (id: string) => void;
  onAddNew?: () => void;
  canEdit: boolean;
}) {
  return (
    <div className="panel">
      {/* Filter bar */}
      <div className="fbar">
        <div className="fsearch">
          <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" style={{ color: 'var(--muted)' }}>
            <circle cx="7" cy="7" r="4" />
            <path d="M10 10l3 3" />
          </svg>
          <input placeholder="Search witnesses\u2026" />
        </div>
        <span className="chip active">All <span className="x">&middot;{witnesses.length}</span></span>
        <span className="chip">Protected</span>
        <span className="chip">High Risk</span>
        {canEdit && onAddNew && (
          <button onClick={onAddNew} className="btn sm" type="button" style={{ marginLeft: 'auto' }}>
            Add witness <span className="arr">&rarr;</span>
          </button>
        )}
      </div>

      {witnesses.length === 0 ? (
        <div className="panel-body" style={{ textAlign: 'center', padding: '48px 22px' }}>
          <p style={{ fontFamily: '"Fraunces", serif', fontSize: '20px', color: 'var(--muted)', letterSpacing: '-.01em' }}>
            No witnesses recorded
          </p>
          <p style={{ fontSize: '13px', color: 'var(--muted)', marginTop: '8px' }}>
            {canEdit ? 'Add witnesses to track identity information securely.' : 'No witnesses have been added to this case yet.'}
          </p>
        </div>
      ) : (
        <table className="tbl">
          <thead>
            <tr>
              <th>Code</th>
              <th>Name</th>
              <th>Protection</th>
              <th>Statement</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {witnesses.map((w) => {
              const protection = PROTECTION_PILL[w.protection_status] || PROTECTION_PILL.standard;
              return (
                <tr
                  key={w.id}
                  onClick={() => onSelect(w.id)}
                  style={{ cursor: 'pointer' }}
                >
                  <td>
                    <div className="ref">
                      {w.witness_code}
                      <small>{w.identity_visible ? 'Cleared' : 'Pseudonymised'}</small>
                    </div>
                  </td>
                  <td>
                    {w.identity_visible && w.full_name ? (
                      <span style={{ color: 'var(--ink)' }}>{w.full_name}</span>
                    ) : (
                      <span className="pl pseud">[Restricted]</span>
                    )}
                  </td>
                  <td>
                    <span className={protection.cls}>{protection.label}</span>
                  </td>
                  <td>
                    <span style={{ fontSize: '13px', color: 'var(--muted)' }}>
                      {w.statement_summary || '\u2014'}
                    </span>
                  </td>
                  <td className="actions">
                    <span className="linkarrow">Open &rarr;</span>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      )}
    </div>
  );
}
