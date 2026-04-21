'use client';

interface Chip {
  readonly key: string;
  readonly label: string;
  readonly count?: number;
  readonly active?: boolean;
}

interface FilterBarProps {
  readonly searchPlaceholder?: string;
  readonly searchValue?: string;
  readonly onSearchChange?: (value: string) => void;
  readonly chips: Chip[];
  readonly onChipClick?: (key: string) => void;
  readonly children?: React.ReactNode;
}

export function FilterBar({
  searchPlaceholder = 'Search…',
  searchValue = '',
  onSearchChange,
  chips,
  onChipClick,
  children,
}: FilterBarProps) {
  return (
    <div className="fbar">
      <div className="fsearch">
        <svg
          width="14"
          height="14"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          style={{ color: 'var(--muted)', flexShrink: 0 }}
        >
          <circle cx="11" cy="11" r="8" />
          <line x1="21" y1="21" x2="16.65" y2="16.65" />
        </svg>
        <input
          type="text"
          placeholder={searchPlaceholder}
          value={searchValue}
          onChange={(e) => onSearchChange?.(e.target.value)}
        />
      </div>
      {chips.map((c) => (
        <button
          key={c.key}
          type="button"
          className={`chip${c.active ? ' active' : ''}`}
          onClick={() => onChipClick?.(c.key)}
        >
          {c.label}
          {c.count != null && <span>({c.count})</span>}
        </button>
      ))}
      {children}
    </div>
  );
}
