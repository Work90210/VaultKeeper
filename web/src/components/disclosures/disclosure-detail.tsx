'use client';

import type { Disclosure, EvidenceItem } from '@/types';
import { ArrowLeft, FileText, Shield, Calendar, Users, AlertTriangle } from 'lucide-react';

const CLASSIFICATION_STYLES: Record<string, { color: string; bg: string }> = {
  public: { color: 'var(--status-active)', bg: 'var(--status-active-bg)' },
  restricted: { color: 'var(--status-closed)', bg: 'var(--status-closed-bg)' },
  confidential: { color: 'var(--status-hold)', bg: 'var(--status-hold-bg)' },
  ex_parte: { color: 'var(--status-hold)', bg: 'var(--status-hold-bg)' },
};

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
    <div
      className="max-w-4xl mx-auto"
      style={{ animation: 'fade-in var(--duration-slow) var(--ease-out-expo)' }}
    >
      {/* Header */}
      <header className="mb-[var(--space-xl)]">
        <button
          onClick={onBack}
          className="btn-ghost text-xs uppercase tracking-wider flex items-center gap-[var(--space-xs)] mb-[var(--space-md)]"
          type="button"
        >
          <ArrowLeft size={14} />
          Back to case
        </button>

        <div className="flex items-start justify-between gap-[var(--space-md)]">
          <div>
            <div className="flex items-center gap-[var(--space-sm)] mb-[var(--space-xs)]">
              <span
                className="badge"
                style={{
                  backgroundColor: 'var(--status-active-bg)',
                  color: 'var(--status-active)',
                }}
              >
                DISCLOSED
              </span>
              {disclosure.redacted && (
                <span
                  className="badge"
                  style={{
                    backgroundColor: 'var(--status-closed-bg)',
                    color: 'var(--status-closed)',
                  }}
                >
                  REDACTED
                </span>
              )}
            </div>
            <h1
              className="font-[family-name:var(--font-heading)] text-2xl"
              style={{ color: 'var(--text-primary)' }}
            >
              Disclosure to {disclosure.disclosed_to}
            </h1>
          </div>
        </div>
      </header>

      {/* Metadata grid */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-[var(--space-md)] mb-[var(--space-lg)]">
        <div
          className="card-inset p-[var(--space-md)] flex items-start gap-[var(--space-sm)]"
        >
          <div
            className="flex items-center justify-center w-8 h-8 rounded-lg shrink-0"
            style={{ backgroundColor: 'var(--amber-subtle)' }}
          >
            <Users size={14} style={{ color: 'var(--amber-accent)' }} />
          </div>
          <div>
            <dt className="field-label" style={{ marginBottom: '2px' }}>Recipient</dt>
            <dd className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
              {disclosure.disclosed_to}
            </dd>
          </div>
        </div>

        <div
          className="card-inset p-[var(--space-md)] flex items-start gap-[var(--space-sm)]"
        >
          <div
            className="flex items-center justify-center w-8 h-8 rounded-lg shrink-0"
            style={{ backgroundColor: 'var(--amber-subtle)' }}
          >
            <Calendar size={14} style={{ color: 'var(--amber-accent)' }} />
          </div>
          <div>
            <dt className="field-label" style={{ marginBottom: '2px' }}>Date</dt>
            <dd className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
              {formattedDate}
            </dd>
          </div>
        </div>

        <div
          className="card-inset p-[var(--space-md)] flex items-start gap-[var(--space-sm)]"
        >
          <div
            className="flex items-center justify-center w-8 h-8 rounded-lg shrink-0"
            style={{ backgroundColor: 'var(--amber-subtle)' }}
          >
            <Shield size={14} style={{ color: 'var(--amber-accent)' }} />
          </div>
          <div>
            <dt className="field-label" style={{ marginBottom: '2px' }}>Redacted</dt>
            <dd className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
              {disclosure.redacted ? 'Yes \u2014 redactions applied' : 'No \u2014 full disclosure'}
            </dd>
          </div>
        </div>
      </div>

      {/* Notes */}
      {disclosure.notes && (
        <div className="mb-[var(--space-lg)]">
          <h2 className="field-label">Notes</h2>
          <div
            className="card-inset p-[var(--space-md)] mt-[var(--space-xs)]"
          >
            <p
              className="text-sm leading-relaxed whitespace-pre-wrap"
              style={{ color: 'var(--text-secondary)' }}
            >
              {disclosure.notes}
            </p>
          </div>
        </div>
      )}

      {/* Disclosed evidence table */}
      <div className="mb-[var(--space-lg)]">
        <div className="flex items-baseline justify-between mb-[var(--space-sm)]">
          <h2 className="field-label" style={{ marginBottom: 0 }}>
            Disclosed evidence ({disclosedItems.length})
          </h2>
        </div>

        <div className="card-inset overflow-hidden" style={{ padding: 0 }}>
          {disclosedItems.length === 0 ? (
            <div className="p-[var(--space-lg)] text-center">
              <p className="text-sm" style={{ color: 'var(--text-tertiary)' }}>
                No evidence items linked to this disclosure.
              </p>
            </div>
          ) : (
            <table className="w-full text-sm" style={{ borderCollapse: 'collapse', tableLayout: 'fixed' }}>
              <colgroup>
                <col style={{ width: '4%' }} />
                <col style={{ width: '36%' }} />
                <col style={{ width: '30%' }} />
                <col style={{ width: '15%' }} />
                <col style={{ width: '15%' }} />
              </colgroup>
              <thead>
                <tr style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                  <th
                    className="text-left px-[var(--space-md)] py-[var(--space-sm)] text-xs font-semibold uppercase tracking-[0.06em]"
                    style={{ color: 'var(--text-tertiary)' }}
                  />
                  <th
                    className="text-left px-[var(--space-md)] py-[var(--space-sm)] text-xs font-semibold uppercase tracking-[0.06em]"
                    style={{ color: 'var(--text-tertiary)' }}
                  >
                    File
                  </th>
                  <th
                    className="text-left px-[var(--space-md)] py-[var(--space-sm)] text-xs font-semibold uppercase tracking-[0.06em]"
                    style={{ color: 'var(--text-tertiary)' }}
                  >
                    Reference
                  </th>
                  <th
                    className="text-left px-[var(--space-md)] py-[var(--space-sm)] text-xs font-semibold uppercase tracking-[0.06em]"
                    style={{ color: 'var(--text-tertiary)' }}
                  >
                    Classification
                  </th>
                  <th
                    className="text-right px-[var(--space-md)] py-[var(--space-sm)] text-xs font-semibold uppercase tracking-[0.06em]"
                    style={{ color: 'var(--text-tertiary)' }}
                  >
                    Size
                  </th>
                </tr>
              </thead>
              <tbody>
                {disclosedItems.map((item) => {
                  const cls =
                    CLASSIFICATION_STYLES[item.classification] ||
                    CLASSIFICATION_STYLES.public;
                  return (
                    <tr
                      key={item.id}
                      className="table-row"
                      style={{ borderBottom: '1px solid var(--border-subtle)' }}
                      onClick={() => (window.location.href = `/en/evidence/${item.id}`)}
                    >
                      <td className="px-[var(--space-md)] py-[var(--space-sm)]">
                        <FileText
                          size={15}
                          strokeWidth={1.5}
                          style={{ color: 'var(--text-tertiary)' }}
                        />
                      </td>
                      <td
                        className="px-[var(--space-md)] py-[var(--space-sm)]"
                        style={{ color: 'var(--text-primary)' }}
                      >
                        <span className="block truncate">
                          {item.title || item.original_name}
                        </span>
                      </td>
                      <td
                        className="px-[var(--space-md)] py-[var(--space-sm)] font-[family-name:var(--font-mono)] text-xs"
                        style={{ color: 'var(--text-tertiary)' }}
                      >
                        <span className="block truncate">
                          {item.evidence_number}
                        </span>
                      </td>
                      <td className="px-[var(--space-md)] py-[var(--space-sm)]">
                        <span
                          className="badge"
                          style={{ backgroundColor: cls.bg, color: cls.color }}
                        >
                          {item.classification.replace('_', ' ')}
                        </span>
                      </td>
                      <td
                        className="px-[var(--space-md)] py-[var(--space-sm)] text-xs text-right"
                        style={{ color: 'var(--text-tertiary)' }}
                      >
                        {formatBytes(item.file_size)}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          )}
        </div>
      </div>

      {/* Permanence notice */}
      <div
        className="flex items-center gap-[var(--space-sm)] p-[var(--space-md)] rounded-[var(--radius-md)]"
        style={{
          backgroundColor: 'var(--status-closed-bg)',
          borderLeft: '3px solid var(--status-closed)',
        }}
      >
        <AlertTriangle size={14} style={{ color: 'var(--status-closed)', shrink: 0 }} />
        <p className="text-xs" style={{ color: 'var(--status-closed)' }}>
          Disclosure is permanent and cannot be reversed. All access is logged and auditable.
        </p>
      </div>
    </div>
  );
}

function formatBytes(bytes: number): string {
  if (!bytes || bytes === 0) return '\u2014';
  const units = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0)} ${units[i]}`;
}
