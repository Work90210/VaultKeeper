'use client';

import { useState } from 'react';

interface ChipCounts {
  readonly all: number;
  readonly doc?: number;
  readonly media?: number;
  readonly audio?: number;
  readonly forensic?: number;
  readonly hold?: number;
}

interface EvidenceFilterBarProps {
  readonly chipCounts: ChipCounts;
}

function formatNumber(n: number): string {
  return n.toLocaleString('en-US');
}

type FilterKey =
  | 'all'
  | 'doc'
  | 'media'
  | 'audio'
  | 'forensic'
  | 'hold'
  | 'redactions'
  | 'contributor';

type ViewMode = 'grid' | 'table';

export function EvidenceFilterBar({ chipCounts }: EvidenceFilterBarProps) {
  const [activeFilter, setActiveFilter] = useState<FilterKey>('all');
  const [viewMode, setViewMode] = useState<ViewMode>('grid');
  const [search, setSearch] = useState('');

  const chips: readonly {
    readonly key: FilterKey;
    readonly label: string;
    readonly count?: number;
    readonly chevron?: boolean;
  }[] = [
    { key: 'all', label: 'All', count: chipCounts.all },
    { key: 'doc', label: 'Document', count: chipCounts.doc },
    { key: 'media', label: 'Image / video', count: chipCounts.media },
    { key: 'audio', label: 'Audio', count: chipCounts.audio },
    { key: 'forensic', label: 'Forensic', count: chipCounts.forensic },
    { key: 'hold', label: 'Legal hold', count: chipCounts.hold },
    { key: 'redactions', label: 'Has redactions', chevron: true },
    { key: 'contributor', label: 'Contributor', chevron: true },
  ];

  return (
    <div className="fbar">
      <div className="fsearch">
        <svg
          width="14"
          height="14"
          viewBox="0 0 16 16"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.5"
          style={{ color: 'var(--muted)' }}
        >
          <circle cx="7" cy="7" r="4" />
          <path d="M10 10l3 3" />
        </svg>
        <input
          type="text"
          placeholder="hash, filename, tag, contributor\u2026"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
      </div>

      {chips.map((c) => (
        <button
          key={c.key}
          type="button"
          className={`chip${activeFilter === c.key ? ' active' : ''}`}
          onClick={() => setActiveFilter(c.key)}
        >
          {c.label}{' '}
          {c.count != null ? (
            <span className={activeFilter === c.key ? 'x' : 'chev'}>
              {activeFilter === c.key
                ? `\u00B7${formatNumber(c.count)}`
                : formatNumber(c.count)}
            </span>
          ) : c.chevron ? (
            <span className="chev">&#9662;</span>
          ) : (
            <span className="chev">&mdash;</span>
          )}
        </button>
      ))}

      <button
        type="button"
        className={`chip${viewMode === 'grid' ? ' active' : ''}`}
        style={{ marginLeft: 'auto' }}
        onClick={() => setViewMode('grid')}
      >
        &#8862; Grid
      </button>
      <button
        type="button"
        className={`chip${viewMode === 'table' ? ' active' : ''}`}
        onClick={() => setViewMode('table')}
      >
        &#9776; Table
      </button>
    </div>
  );
}
