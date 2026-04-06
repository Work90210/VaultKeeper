'use client';

interface CaseData {
  id: string;
  reference_code: string;
  title: string;
  description: string;
  jurisdiction: string;
  status: string;
  legal_hold: boolean;
  created_by: string;
  created_at: string;
  updated_at: string;
}

const STATUS_STYLES: Record<string, { color: string; bg: string }> = {
  active: { color: 'var(--status-active)', bg: 'var(--status-active-bg)' },
  closed: { color: 'var(--status-closed)', bg: 'var(--status-closed-bg)' },
  archived: { color: 'var(--status-archived)', bg: 'var(--status-archived-bg)' },
};

export function CaseDetail({
  caseData,
  canEdit,
}: {
  caseData: CaseData;
  canEdit: boolean;
}) {
  const status = STATUS_STYLES[caseData.status] || STATUS_STYLES.archived;

  return (
    <div
      className="space-y-[var(--space-lg)]"
      style={{ animation: 'fade-in var(--duration-slow) var(--ease-out-expo)' }}
    >
      {/* Header band */}
      <div>
        <div className="flex items-center gap-[var(--space-sm)] mb-[var(--space-xs)]">
          <span
            className="font-[family-name:var(--font-mono)] text-[var(--text-xs)] tracking-wide"
            style={{ color: 'var(--text-tertiary)' }}
          >
            {caseData.reference_code}
          </span>
          <span
            className="badge"
            style={{ backgroundColor: status.bg, color: status.color }}
          >
            {caseData.status}
          </span>
          {caseData.legal_hold && (
            <span
              className="badge"
              style={{
                backgroundColor: 'var(--status-hold-bg)',
                color: 'var(--status-hold)',
              }}
            >
              LEGAL HOLD
            </span>
          )}
        </div>
        <h1
          className="font-[family-name:var(--font-heading)] text-[var(--text-2xl)] leading-tight text-balance"
          style={{ color: 'var(--text-primary)' }}
        >
          {caseData.title}
        </h1>
      </div>

      {/* Metadata card */}
      <div className="card-inset grid grid-cols-2 sm:grid-cols-4 gap-[var(--space-lg)] p-[var(--space-md)]">
        <MetaField label="Jurisdiction" value={caseData.jurisdiction || '\u2014'} />
        <MetaField
          label="Created"
          value={new Date(caseData.created_at).toLocaleDateString('en-GB', {
            day: '2-digit',
            month: 'short',
            year: 'numeric',
          })}
        />
        <MetaField
          label="Updated"
          value={new Date(caseData.updated_at).toLocaleDateString('en-GB', {
            day: '2-digit',
            month: 'short',
            year: 'numeric',
          })}
        />
        <MetaField
          label="Created by"
          value={caseData.created_by.slice(0, 8) + '\u2026'}
          mono
        />
      </div>

      {/* Description */}
      {caseData.description && (
        <div className="card p-[var(--space-lg)]">
          <h2 className="field-label mb-[var(--space-sm)]">
            Description
          </h2>
          <p
            className="text-[var(--text-base)] leading-relaxed whitespace-pre-wrap max-w-2xl"
            style={{ color: 'var(--text-secondary)' }}
          >
            {caseData.description}
          </p>
        </div>
      )}

      {/* Actions */}
      {canEdit && (
        <div>
          <a
            href={`/en/cases/${caseData.id}/settings`}
            className="link-accent text-[var(--text-sm)]"
          >
            Case settings &rarr;
          </a>
        </div>
      )}
    </div>
  );
}

function MetaField({
  label,
  value,
  mono,
}: {
  label: string;
  value: string;
  mono?: boolean;
}) {
  return (
    <div>
      <dt className="field-label">{label}</dt>
      <dd
        className={`mt-[var(--space-xs)] text-[var(--text-sm)] ${mono ? 'font-[family-name:var(--font-mono)]' : ''}`}
        style={{ color: 'var(--text-primary)' }}
      >
        {value}
      </dd>
    </div>
  );
}
