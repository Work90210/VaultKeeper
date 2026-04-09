'use client';

import type { EvidenceItem } from '@/types';

export function VersionHistory({
  versions,
  currentId,
  onSelectVersion,
}: {
  versions: readonly EvidenceItem[];
  currentId: string;
  onSelectVersion: (id: string) => void;
}) {
  if (versions.length <= 1) {
    return null;
  }

  return (
    <div className="card space-y-[var(--space-md)]">
      <h4
        className="text-xs uppercase tracking-wider font-medium"
        style={{ color: 'var(--text-tertiary)' }}
      >
        Version History ({versions.length})
      </h4>

      <div className="relative">
        {/* Timeline line */}
        <div
          className="absolute left-[11px] top-0 bottom-0 w-px"
          style={{ backgroundColor: 'var(--border-primary)' }}
        />

        <div className="space-y-[var(--space-sm)]">
          {versions.map((version) => {
            const isCurrent = version.is_current;
            const isSelected = version.id === currentId;

            return (
              <button
                key={version.id}
                onClick={() => onSelectVersion(version.id)}
                className="flex items-start gap-[var(--space-sm)] w-full text-left group"
                style={{
                  position: 'relative',
                  padding: 'var(--space-sm) var(--space-sm) var(--space-sm) var(--space-lg)',
                  borderRadius: '6px',
                  backgroundColor: isSelected ? 'var(--bg-secondary)' : 'transparent',
                  border: isSelected ? '1px solid var(--border-primary)' : '1px solid transparent',
                }}
              >
                {/* Timeline dot */}
                <div
                  className="absolute left-[6px] top-[14px] w-[11px] h-[11px] rounded-full border-2"
                  style={{
                    borderColor: isCurrent ? 'var(--accent-primary)' : 'var(--text-tertiary)',
                    backgroundColor: isCurrent ? 'var(--accent-primary)' : 'var(--bg-primary)',
                  }}
                />

                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-[var(--space-sm)]">
                    <span
                      className="text-sm font-medium"
                      style={{ color: 'var(--text-primary)' }}
                    >
                      v{version.version}
                    </span>
                    {isCurrent && (
                      <span
                        className="text-xs px-[var(--space-xs)] py-0.5 rounded font-medium"
                        style={{
                          backgroundColor: 'rgba(128, 128, 80, 0.12)',
                          color: 'rgb(100, 110, 70)',
                        }}
                      >
                        Current
                      </span>
                    )}
                  </div>

                  <div className="flex items-center gap-[var(--space-md)] mt-[var(--space-xs)]">
                    <span
                      className="font-[family-name:var(--font-mono)] text-xs"
                      style={{ color: 'var(--text-tertiary)' }}
                    >
                      {version.sha256_hash.slice(0, 16)}...
                    </span>
                    <span className="text-xs" style={{ color: 'var(--text-tertiary)' }}>
                      {new Date(version.created_at).toLocaleDateString()}
                    </span>
                    <span className="text-xs" style={{ color: 'var(--text-tertiary)' }}>
                      {version.uploaded_by_name}
                    </span>
                  </div>

                  {version.tsa_timestamp && (
                    <div className="flex items-center gap-[var(--space-xs)] mt-[var(--space-xs)]">
                      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="rgb(100, 110, 70)" strokeWidth="2" strokeLinecap="round">
                        <path d="M12 22c5.523 0 10-4.477 10-10S17.523 2 12 2 2 6.477 2 12s4.477 10 10 10Z" />
                        <path d="m9 12 2 2 4-4" />
                      </svg>
                      <span className="text-xs" style={{ color: 'rgb(100, 110, 70)' }}>
                        TSA verified
                      </span>
                    </div>
                  )}
                </div>
              </button>
            );
          })}
        </div>
      </div>
    </div>
  );
}
