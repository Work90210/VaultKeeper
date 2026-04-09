'use client';

import type { Witness } from '@/types';
import {
  ArrowLeft,
  User,
  Mail,
  MapPin,
  Shield,
  Calendar,
  FileText,
  AlertTriangle,
} from 'lucide-react';

const PROTECTION_STYLES: Record<string, { color: string; bg: string; label: string }> = {
  standard: { color: 'var(--status-active)', bg: 'var(--status-active-bg)', label: 'Standard' },
  protected: { color: 'var(--status-closed)', bg: 'var(--status-closed-bg)', label: 'Protected' },
  high_risk: { color: 'var(--status-hold)', bg: 'var(--status-hold-bg)', label: 'High Risk' },
};

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('en-GB', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
  });
}

export function WitnessDetail({
  witness,
  onEdit,
  onBack,
  canEdit,
}: {
  witness: Witness;
  onEdit?: () => void;
  onBack: () => void;
  canEdit: boolean;
}) {
  const protection =
    PROTECTION_STYLES[witness.protection_status] || PROTECTION_STYLES.standard;

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
                className="font-[family-name:var(--font-mono)] text-xs tracking-wide"
                style={{ color: 'var(--text-tertiary)' }}
              >
                {witness.witness_code}
              </span>
              <span
                className="badge"
                style={{ backgroundColor: protection.bg, color: protection.color }}
              >
                {protection.label}
              </span>
            </div>
            <h1
              className="font-[family-name:var(--font-heading)] text-2xl"
              style={{ color: 'var(--text-primary)' }}
            >
              {witness.identity_visible
                ? witness.full_name || 'Unnamed Witness'
                : `Witness ${witness.witness_code}`}
            </h1>
          </div>
          {canEdit && onEdit && (
            <button type="button" onClick={onEdit} className="btn-secondary text-xs">
              Edit witness
            </button>
          )}
        </div>
      </header>

      {/* Identity section */}
      {witness.identity_visible ? (
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-[var(--space-md)] mb-[var(--space-lg)]">
          <div className="card-inset p-[var(--space-md)] flex items-start gap-[var(--space-sm)]">
            <div
              className="flex items-center justify-center w-8 h-8 rounded-lg shrink-0"
              style={{ backgroundColor: 'var(--amber-subtle)' }}
            >
              <User size={14} style={{ color: 'var(--amber-accent)' }} />
            </div>
            <div>
              <dt className="field-label" style={{ marginBottom: '2px' }}>Full Name</dt>
              <dd className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                {witness.full_name || '\u2014'}
              </dd>
            </div>
          </div>

          <div className="card-inset p-[var(--space-md)] flex items-start gap-[var(--space-sm)]">
            <div
              className="flex items-center justify-center w-8 h-8 rounded-lg shrink-0"
              style={{ backgroundColor: 'var(--amber-subtle)' }}
            >
              <Mail size={14} style={{ color: 'var(--amber-accent)' }} />
            </div>
            <div>
              <dt className="field-label" style={{ marginBottom: '2px' }}>Contact</dt>
              <dd className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                {witness.contact_info || '\u2014'}
              </dd>
            </div>
          </div>

          <div className="card-inset p-[var(--space-md)] flex items-start gap-[var(--space-sm)]">
            <div
              className="flex items-center justify-center w-8 h-8 rounded-lg shrink-0"
              style={{ backgroundColor: 'var(--amber-subtle)' }}
            >
              <MapPin size={14} style={{ color: 'var(--amber-accent)' }} />
            </div>
            <div>
              <dt className="field-label" style={{ marginBottom: '2px' }}>Location</dt>
              <dd className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                {witness.location || '\u2014'}
              </dd>
            </div>
          </div>
        </div>
      ) : (
        <div
          className="flex items-center gap-[var(--space-sm)] p-[var(--space-md)] rounded-[var(--radius-md)] mb-[var(--space-lg)]"
          style={{
            backgroundColor: 'var(--status-closed-bg)',
            borderLeft: '3px solid var(--status-closed)',
          }}
        >
          <Shield size={14} style={{ color: 'var(--status-closed)' }} />
          <p className="text-xs" style={{ color: 'var(--status-closed)' }}>
            Identity information restricted for your role.
          </p>
        </div>
      )}

      {/* Statement */}
      <div className="mb-[var(--space-lg)]">
        <h2 className="field-label">Statement summary</h2>
        <div className="card-inset p-[var(--space-md)] mt-[var(--space-xs)]">
          <p
            className="text-sm leading-relaxed whitespace-pre-wrap"
            style={{
              color: witness.statement_summary
                ? 'var(--text-secondary)'
                : 'var(--text-tertiary)',
            }}
          >
            {witness.statement_summary || 'No statement recorded.'}
          </p>
        </div>
      </div>

      {/* Linked evidence */}
      <div className="mb-[var(--space-lg)]">
        <h2 className="field-label" style={{ marginBottom: 'var(--space-sm)' }}>
          Linked evidence ({witness.related_evidence.length})
        </h2>
        <div className="card-inset overflow-hidden" style={{ padding: 0 }}>
          {witness.related_evidence.length === 0 ? (
            <div className="p-[var(--space-lg)] text-center">
              <p className="text-sm" style={{ color: 'var(--text-tertiary)' }}>
                No evidence linked to this witness.
              </p>
            </div>
          ) : (
            <div>
              {witness.related_evidence.map((id, i) => (
                <a
                  key={id}
                  href={`/en/evidence/${id}`}
                  className="table-row flex items-center gap-[var(--space-sm)] px-[var(--space-md)] py-[var(--space-sm)]"
                  style={{
                    textDecoration: 'none',
                    borderBottom:
                      i < witness.related_evidence.length - 1
                        ? '1px solid var(--border-subtle)'
                        : 'none',
                  }}
                >
                  <FileText
                    size={15}
                    strokeWidth={1.5}
                    style={{ color: 'var(--text-tertiary)' }}
                  />
                  <span
                    className="font-[family-name:var(--font-mono)] text-xs"
                    style={{ color: 'var(--amber-accent)' }}
                  >
                    {id.slice(0, 12)}&hellip;
                  </span>
                  <span
                    className="text-xs ml-auto"
                    style={{ color: 'var(--text-tertiary)' }}
                  >
                    View &rarr;
                  </span>
                </a>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Metadata row */}
      <div
        className="grid grid-cols-2 gap-[var(--space-md)] mb-[var(--space-lg)]"
      >
        <div className="card-inset p-[var(--space-md)] flex items-start gap-[var(--space-sm)]">
          <div
            className="flex items-center justify-center w-8 h-8 rounded-lg shrink-0"
            style={{ backgroundColor: 'var(--amber-subtle)' }}
          >
            <Calendar size={14} style={{ color: 'var(--amber-accent)' }} />
          </div>
          <div>
            <dt className="field-label" style={{ marginBottom: '2px' }}>Created</dt>
            <dd className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
              {formatDate(witness.created_at)}
            </dd>
          </div>
        </div>
        <div className="card-inset p-[var(--space-md)] flex items-start gap-[var(--space-sm)]">
          <div
            className="flex items-center justify-center w-8 h-8 rounded-lg shrink-0"
            style={{ backgroundColor: 'var(--amber-subtle)' }}
          >
            <Calendar size={14} style={{ color: 'var(--amber-accent)' }} />
          </div>
          <div>
            <dt className="field-label" style={{ marginBottom: '2px' }}>Updated</dt>
            <dd className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
              {formatDate(witness.updated_at)}
            </dd>
          </div>
        </div>
      </div>

      {/* Protection notice for high-risk */}
      {witness.protection_status === 'high_risk' && (
        <div
          className="flex items-center gap-[var(--space-sm)] p-[var(--space-md)] rounded-[var(--radius-md)]"
          style={{
            backgroundColor: 'var(--status-hold-bg)',
            borderLeft: '3px solid var(--status-hold)',
          }}
        >
          <AlertTriangle size={14} style={{ color: 'var(--status-hold)' }} />
          <p className="text-xs" style={{ color: 'var(--status-hold)' }}>
            This witness has high-risk protection status. All access is logged and restricted.
          </p>
        </div>
      )}
    </div>
  );
}
