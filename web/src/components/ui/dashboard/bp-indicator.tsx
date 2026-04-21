interface BPPhase {
  readonly name: string;
  readonly status: 'complete' | 'in_progress' | 'not_started';
  readonly pct?: number;
  readonly items?: string[];
  readonly missing?: string[];
  readonly action?: string;
}

interface BPIndicatorProps {
  readonly phases: BPPhase[];
  readonly variant: 'full' | 'dots' | 'inline';
}

const STATUS_CLASS: Record<BPPhase['status'], string> = {
  complete: 'complete',
  in_progress: 'partial',
  not_started: 'missing',
};

const PHASE_CLASS: Record<BPPhase['status'], string> = {
  complete: 'done',
  in_progress: 'part',
  not_started: 'none',
};

const DOT_COLOR: Record<BPPhase['status'], string> = {
  complete: 'var(--ok)',
  in_progress: 'var(--accent)',
  not_started: 'var(--muted-2)',
};

function completedCount(phases: BPPhase[]): number {
  return phases.filter((p) => p.status === 'complete').length;
}

function FullVariant({ phases }: { readonly phases: BPPhase[] }) {
  return (
    <div className="bp-tracker">
      {phases.map((phase, i) => (
        <div key={phase.name} className={`bp-phase ${PHASE_CLASS[phase.status]}`}>
          <div className="bp-num">Phase {i + 1}</div>
          <div className="bp-name">{phase.name}</div>
          <div className={`bp-status ${STATUS_CLASS[phase.status]}`}>
            <span className="dot" />
            {phase.status === 'complete'
              ? 'Complete'
              : phase.status === 'in_progress'
                ? 'In Progress'
                : 'Not Started'}
          </div>
          {phase.items && phase.items.length > 0 && (
            <div className="bp-detail">
              {phase.items.map((item) => (
                <div key={item}>{item}</div>
              ))}
            </div>
          )}
          {phase.missing && phase.missing.length > 0 && (
            <div className="bp-detail" style={{ color: 'var(--accent)' }}>
              {phase.missing.map((m) => (
                <div key={m}>Missing: {m}</div>
              ))}
            </div>
          )}
          {phase.action && (
            <div className="bp-detail" style={{ marginTop: 4 }}>
              <a href={phase.action} className="linkarrow">
                Take action <span>→</span>
              </a>
            </div>
          )}
          <div className="bp-bar">
            <div
              className="bp-fill"
              style={{ width: phase.pct != null ? `${phase.pct}%` : undefined }}
            />
          </div>
        </div>
      ))}
    </div>
  );
}

function DotsVariant({ phases }: { readonly phases: BPPhase[] }) {
  const done = completedCount(phases);
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 3 }}>
      {phases.map((phase, i) => (
        <span
          key={i}
          style={{
            width: 6,
            height: 6,
            borderRadius: '50%',
            background: DOT_COLOR[phase.status],
            flexShrink: 0,
          }}
        />
      ))}
      <span
        style={{
          marginLeft: 6,
          fontFamily: '"JetBrains Mono", monospace',
          fontSize: 11,
          color: 'var(--muted)',
          letterSpacing: '.02em',
        }}
      >
        {done}/{phases.length}
      </span>
    </span>
  );
}

function InlineVariant({ phases }: { readonly phases: BPPhase[] }) {
  const done = completedCount(phases);
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 5 }}>
      {phases.map((phase, i) => (
        <span
          key={i}
          style={{
            width: 5,
            height: 5,
            borderRadius: '50%',
            background: DOT_COLOR[phase.status],
            flexShrink: 0,
          }}
        />
      ))}
      <span
        style={{
          fontFamily: '"JetBrains Mono", monospace',
          fontSize: 11,
          color: 'var(--muted)',
          letterSpacing: '.02em',
        }}
      >
        {done}/{phases.length} phases
      </span>
    </span>
  );
}

export function BPIndicator({ phases, variant }: BPIndicatorProps) {
  switch (variant) {
    case 'full':
      return <FullVariant phases={phases} />;
    case 'dots':
      return <DotsVariant phases={phases} />;
    case 'inline':
      return <InlineVariant phases={phases} />;
  }
}
