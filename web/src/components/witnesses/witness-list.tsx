'use client';

import type { Witness } from '@/types';

const PROTECTION_STYLES: Record<string, { bg: string; text: string; label: string }> = {
  standard: { bg: 'rgba(128, 128, 80, 0.12)', text: 'rgb(100, 110, 70)', label: 'Standard' },
  protected: { bg: 'rgba(180, 140, 50, 0.12)', text: 'rgb(160, 120, 30)', label: 'Protected' },
  high_risk: { bg: 'rgba(170, 70, 60, 0.12)', text: 'rgb(160, 60, 50)', label: 'High Risk' },
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
    <div className="space-y-[var(--space-md)]">
      <div className="flex items-center justify-between">
        <h3
          className="font-[family-name:var(--font-heading)] text-lg"
          style={{ color: 'var(--text-primary)' }}
        >
          Witnesses
        </h3>
        {canEdit && onAddNew && (
          <button onClick={onAddNew} className="btn-primary">
            Add Witness
          </button>
        )}
      </div>

      {witnesses.length === 0 ? (
        <div className="card py-[var(--space-2xl)] text-center">
          <p
            className="font-[family-name:var(--font-heading)] text-xl"
            style={{ color: 'var(--text-tertiary)' }}
          >
            No witnesses recorded
          </p>
          <p className="mt-[var(--space-xs)] text-sm" style={{ color: 'var(--text-tertiary)' }}>
            {canEdit ? 'Add witnesses to track identity information securely.' : 'No witnesses have been added to this case yet.'}
          </p>
        </div>
      ) : (
        <div className="card overflow-hidden">
          <table className="w-full text-sm" style={{ borderCollapse: 'collapse' }}>
            <thead>
              <tr style={{ borderBottom: '1px solid var(--border-primary)' }}>
                <th className="text-left px-[var(--space-md)] py-[var(--space-sm)] text-xs uppercase tracking-wider font-medium" style={{ color: 'var(--text-tertiary)' }}>
                  Code
                </th>
                <th className="text-left px-[var(--space-md)] py-[var(--space-sm)] text-xs uppercase tracking-wider font-medium" style={{ color: 'var(--text-tertiary)' }}>
                  Name
                </th>
                <th className="text-left px-[var(--space-md)] py-[var(--space-sm)] text-xs uppercase tracking-wider font-medium" style={{ color: 'var(--text-tertiary)' }}>
                  Protection
                </th>
                <th className="text-left px-[var(--space-md)] py-[var(--space-sm)] text-xs uppercase tracking-wider font-medium" style={{ color: 'var(--text-tertiary)' }}>
                  Statement
                </th>
              </tr>
            </thead>
            <tbody>
              {witnesses.map((w) => {
                const protection = PROTECTION_STYLES[w.protection_status] || PROTECTION_STYLES.standard;
                return (
                  <tr
                    key={w.id}
                    onClick={() => onSelect(w.id)}
                    style={{
                      cursor: 'pointer',
                      borderBottom: '1px solid var(--border-primary)',
                    }}
                  >
                    <td className="px-[var(--space-md)] py-[var(--space-sm)]">
                      <span
                        className="font-[family-name:var(--font-mono)] text-xs font-medium"
                        style={{ color: 'var(--accent-primary)' }}
                      >
                        {w.witness_code}
                      </span>
                    </td>
                    <td className="px-[var(--space-md)] py-[var(--space-sm)]">
                      {w.identity_visible && w.full_name ? (
                        <span style={{ color: 'var(--text-primary)' }}>{w.full_name}</span>
                      ) : (
                        <span
                          className="text-xs font-medium uppercase tracking-wider"
                          style={{ color: 'var(--text-tertiary)', opacity: 0.7 }}
                        >
                          [Restricted]
                        </span>
                      )}
                    </td>
                    <td className="px-[var(--space-md)] py-[var(--space-sm)]">
                      <span
                        className="inline-block px-[var(--space-xs)] py-0.5 rounded text-xs font-medium"
                        style={{ backgroundColor: protection.bg, color: protection.text }}
                      >
                        {protection.label}
                      </span>
                    </td>
                    <td className="px-[var(--space-md)] py-[var(--space-sm)]">
                      <span
                        className="text-sm line-clamp-1"
                        style={{ color: 'var(--text-secondary)' }}
                      >
                        {w.statement_summary || '—'}
                      </span>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
