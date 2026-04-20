'use client';

import { useState } from 'react';

interface RecordPickerProps {
  readonly items: readonly { id: string; label: string; subtitle?: string }[];
  readonly selectedIds: readonly string[];
  readonly onSelect: (ids: readonly string[]) => void;
  readonly label: string;
}

export function RecordPicker({ items, selectedIds, onSelect, label }: RecordPickerProps) {
  const [search, setSearch] = useState('');

  const filtered = items.filter(
    (item) =>
      item.label.toLowerCase().includes(search.toLowerCase()) ||
      (item.subtitle && item.subtitle.toLowerCase().includes(search.toLowerCase())),
  );

  const toggleItem = (id: string) => {
    if (selectedIds.includes(id)) {
      onSelect(selectedIds.filter(s => s !== id));
    } else {
      onSelect([...selectedIds, id]);
    }
  };

  return (
    <div>
      <label className="field-label">{label}</label>

      {items.length > 5 && (
        <input
          type="text"
          className="input-field mb-[var(--space-xs)]"
          placeholder="Search..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
      )}

      {items.length === 0 ? (
        <p className="text-xs" style={{ color: 'var(--text-tertiary)' }}>No records available.</p>
      ) : (
        <div
          className="border rounded-md overflow-y-auto"
          style={{ borderColor: 'var(--border-default)', maxHeight: '10rem' }}
        >
          {filtered.map((item) => {
            const checked = selectedIds.includes(item.id);
            return (
              <label
                key={item.id}
                className="flex items-center gap-[var(--space-sm)] px-[var(--space-sm)] py-[var(--space-xs)] cursor-pointer hover:bg-[var(--bg-inset)]"
                style={{ borderBottom: '1px solid var(--border-subtle)' }}
              >
                <input
                  type="checkbox"
                  checked={checked}
                  onChange={() => toggleItem(item.id)}
                  style={{ accentColor: 'var(--amber-accent)' }}
                />
                <div className="min-w-0 flex-1">
                  <span className="text-sm truncate block" style={{ color: 'var(--text-primary)' }}>
                    {item.label}
                  </span>
                  {item.subtitle && (
                    <span className="text-xs truncate block" style={{ color: 'var(--text-tertiary)' }}>
                      {item.subtitle}
                    </span>
                  )}
                </div>
              </label>
            );
          })}
        </div>
      )}

      {selectedIds.length > 0 && (
        <p className="text-xs mt-[var(--space-2xs)]" style={{ color: 'var(--text-secondary)' }}>
          {selectedIds.length} selected
        </p>
      )}
    </div>
  );
}
