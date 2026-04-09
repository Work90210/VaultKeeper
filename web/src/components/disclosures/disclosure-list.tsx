'use client';

import { useState } from 'react';
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
  const [hoveredRow, setHoveredRow] = useState<string | null>(null);

  return (
    <div className="space-y-[var(--space-md)]">
      <div className="flex items-center justify-between">
        <h3
          className="font-[family-name:var(--font-heading)] text-lg"
          style={{ color: 'var(--text-primary)' }}
        >
          Disclosures
        </h3>
        {canCreate && onCreateNew && (
          <button onClick={onCreateNew} className="btn-primary">
            Create Disclosure
          </button>
        )}
      </div>

      {disclosures.length === 0 ? (
        <div className="card py-[var(--space-2xl)] text-center">
          <p
            className="font-[family-name:var(--font-heading)] text-xl"
            style={{ color: 'var(--text-tertiary)' }}
          >
            No disclosures
          </p>
          <p className="mt-[var(--space-xs)] text-sm" style={{ color: 'var(--text-tertiary)' }}>
            Evidence disclosures to defence or other parties will appear here.
          </p>
        </div>
      ) : (
        <div className="card overflow-hidden">
          <table className="w-full text-sm" style={{ borderCollapse: 'collapse' }}>
            <thead>
              <tr style={{ borderBottom: '1px solid var(--border-primary)' }}>
                <th className="text-left px-[var(--space-md)] py-[var(--space-sm)] text-xs uppercase tracking-wider font-medium" style={{ color: 'var(--text-tertiary)' }}>
                  Date
                </th>
                <th className="text-left px-[var(--space-md)] py-[var(--space-sm)] text-xs uppercase tracking-wider font-medium" style={{ color: 'var(--text-tertiary)' }}>
                  Recipient
                </th>
                <th className="text-left px-[var(--space-md)] py-[var(--space-sm)] text-xs uppercase tracking-wider font-medium" style={{ color: 'var(--text-tertiary)' }}>
                  Items
                </th>
                <th className="text-left px-[var(--space-md)] py-[var(--space-sm)] text-xs uppercase tracking-wider font-medium" style={{ color: 'var(--text-tertiary)' }}>
                  Redacted
                </th>
                <th className="text-left px-[var(--space-md)] py-[var(--space-sm)] text-xs uppercase tracking-wider font-medium" style={{ color: 'var(--text-tertiary)' }}>
                  Notes
                </th>
              </tr>
            </thead>
            <tbody>
              {disclosures.map((d) => (
                <tr
                  key={d.id}
                  onClick={() => onSelect(d.id)}
                  onMouseEnter={() => setHoveredRow(d.id)}
                  onMouseLeave={() => setHoveredRow(null)}
                  style={{
                    cursor: 'pointer',
                    borderBottom: '1px solid var(--border-primary)',
                    backgroundColor: hoveredRow === d.id ? 'var(--bg-secondary)' : 'transparent',
                  }}
                >
                  <td className="px-[var(--space-md)] py-[var(--space-sm)]" style={{ color: 'var(--text-primary)' }}>
                    {new Date(d.disclosed_at).toLocaleDateString()}
                  </td>
                  <td className="px-[var(--space-md)] py-[var(--space-sm)]" style={{ color: 'var(--text-secondary)' }}>
                    {d.disclosed_to}
                  </td>
                  <td className="px-[var(--space-md)] py-[var(--space-sm)]">
                    <span
                      className="font-[family-name:var(--font-mono)] text-xs"
                      style={{ color: 'var(--accent-primary)' }}
                    >
                      {d.evidence_ids.length}
                    </span>
                  </td>
                  <td className="px-[var(--space-md)] py-[var(--space-sm)]">
                    {d.redacted && (
                      <span
                        className="inline-block px-[var(--space-xs)] py-0.5 rounded text-xs font-medium"
                        style={{ backgroundColor: 'rgba(180, 140, 50, 0.12)', color: 'rgb(160, 120, 30)' }}
                      >
                        Redacted
                      </span>
                    )}
                  </td>
                  <td className="px-[var(--space-md)] py-[var(--space-sm)]">
                    <span className="text-sm line-clamp-1" style={{ color: 'var(--text-tertiary)' }}>
                      {d.notes || '—'}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
