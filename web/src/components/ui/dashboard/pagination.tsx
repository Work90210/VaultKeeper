'use client';

interface PaginationProps {
  readonly current: number;
  readonly total: number;
  readonly pageSize: number;
  readonly onChange: (page: number) => void;
}

export function Pagination({ current, total, pageSize, onChange }: PaginationProps) {
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const rangeStart = (current - 1) * pageSize + 1;
  const rangeEnd = Math.min(current * pageSize, total);

  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 10,
        fontFamily: '"JetBrains Mono", monospace',
        fontSize: 12,
        color: 'var(--muted)',
        letterSpacing: '.02em',
      }}
    >
      <button
        type="button"
        className="chip"
        disabled={current <= 1}
        onClick={() => onChange(current - 1)}
        style={{ opacity: current <= 1 ? 0.4 : 1 }}
      >
        Prev
      </button>
      <span>
        {rangeStart}&ndash;{rangeEnd} of {total} &middot; page {current} of{' '}
        {totalPages}
      </span>
      <button
        type="button"
        className="chip"
        disabled={current >= totalPages}
        onClick={() => onChange(current + 1)}
        style={{ opacity: current >= totalPages ? 0.4 : 1 }}
      >
        Next
      </button>
    </div>
  );
}
