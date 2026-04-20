'use client';

import { useEffect, useState } from 'react';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

interface EvidenceOption {
  readonly id: string;
  readonly evidence_number: string;
  readonly title: string;
}

interface EvidencePickerProps {
  readonly caseId: string;
  readonly accessToken: string;
  readonly selectedIds: readonly string[];
  readonly onSelect: (ids: readonly string[]) => void;
  readonly label?: string;
  readonly maxItems?: number;
}

export function EvidencePicker({
  caseId,
  accessToken,
  selectedIds,
  onSelect,
  label = 'Evidence References',
  maxItems,
}: EvidencePickerProps) {
  const [items, setItems] = useState<readonly EvidenceOption[]>([]);
  const [search, setSearch] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    fetch(`${API_BASE}/api/cases/${caseId}/evidence?current_only=true`, {
      headers: { Authorization: `Bearer ${accessToken}` },
    })
      .then(r => r.ok ? r.json() : null)
      .then(json => {
        if (cancelled) return;
        const data = (json?.data || []).map((e: { id: string; evidence_number: string; title?: string; original_name: string }) => ({
          id: e.id,
          evidence_number: e.evidence_number,
          title: e.title || e.original_name,
        }));
        setItems(data);
      })
      .catch(() => {})
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [caseId, accessToken]);

  const filtered = items.filter(
    (item) =>
      item.evidence_number.toLowerCase().includes(search.toLowerCase()) ||
      item.title.toLowerCase().includes(search.toLowerCase()),
  );

  const toggleItem = (id: string) => {
    if (selectedIds.includes(id)) {
      onSelect(selectedIds.filter(s => s !== id));
    } else if (!maxItems || selectedIds.length < maxItems) {
      onSelect([...selectedIds, id]);
    }
  };

  return (
    <div>
      <label className="field-label">{label}</label>

      <input
        type="text"
        className="input-field mb-[var(--space-xs)]"
        placeholder="Search evidence..."
        value={search}
        onChange={(e) => setSearch(e.target.value)}
      />

      {loading ? (
        <p className="text-xs" style={{ color: 'var(--text-tertiary)' }}>Loading evidence...</p>
      ) : (
        <div
          className="border rounded-md overflow-y-auto"
          style={{ borderColor: 'var(--border-default)', maxHeight: '12rem' }}
        >
          {filtered.length === 0 ? (
            <p className="text-xs p-[var(--space-sm)] text-center" style={{ color: 'var(--text-tertiary)' }}>
              No evidence found.
            </p>
          ) : (
            filtered.map((item) => {
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
                  <span className="text-xs font-[family-name:var(--font-mono)]" style={{ color: 'var(--text-tertiary)' }}>
                    {item.evidence_number}
                  </span>
                  <span className="text-sm truncate" style={{ color: 'var(--text-primary)' }}>
                    {item.title}
                  </span>
                </label>
              );
            })
          )}
        </div>
      )}

      {selectedIds.length > 0 && (
        <div className="flex flex-wrap gap-1 mt-[var(--space-xs)]">
          {selectedIds.map((id) => {
            const item = items.find(i => i.id === id);
            return (
              <span key={id} className="badge" style={{ backgroundColor: 'var(--amber-subtle)', color: 'var(--amber-accent)' }}>
                {item?.evidence_number || id.slice(0, 8)}
                <button
                  type="button"
                  onClick={() => onSelect(selectedIds.filter(s => s !== id))}
                  style={{ all: 'unset', cursor: 'pointer', marginLeft: '0.25rem', fontWeight: 700 }}
                  aria-label={`Remove ${item?.evidence_number || id}`}
                >
                  &times;
                </button>
              </span>
            );
          })}
        </div>
      )}
    </div>
  );
}
