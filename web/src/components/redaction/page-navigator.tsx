'use client';

import { useCallback, useState } from 'react';

interface PageNavigatorProps {
  currentPage: number;
  totalPages: number;
  onPageChange: (page: number) => void;
}

export function PageNavigator({ currentPage, totalPages, onPageChange }: PageNavigatorProps) {
  const [inputValue, setInputValue] = useState('');
  const [editing, setEditing] = useState(false);

  const handleSubmit = useCallback(() => {
    const num = parseInt(inputValue, 10);
    if (!isNaN(num) && num >= 1 && num <= totalPages) {
      onPageChange(num);
    }
    setEditing(false);
    setInputValue('');
  }, [inputValue, totalPages, onPageChange]);

  return (
    <div className="flex items-center gap-[var(--space-xs)]">
      <button
        onClick={() => onPageChange(currentPage - 1)}
        disabled={currentPage <= 1}
        className="btn-ghost text-xs px-[var(--space-xs)] py-0.5"
        title="Previous page (PageUp)"
      >
        &larr;
      </button>

      {editing ? (
        <input
          type="number"
          min={1}
          max={totalPages}
          value={inputValue}
          onChange={(e) => setInputValue(e.target.value)}
          onBlur={handleSubmit}
          onKeyDown={(e) => {
            if (e.key === 'Enter') handleSubmit();
            if (e.key === 'Escape') { setEditing(false); setInputValue(''); }
          }}
          className="input-field w-12 text-xs text-center py-0.5"
          autoFocus
        />
      ) : (
        <button
          onClick={() => { setEditing(true); setInputValue(String(currentPage)); }}
          className="text-xs tabular-nums px-[var(--space-xs)] py-0.5 rounded hover:bg-[var(--bg-elevated)]"
          style={{ color: 'var(--text-primary)' }}
          title="Click to jump to page"
        >
          {currentPage} / {totalPages}
        </button>
      )}

      <button
        onClick={() => onPageChange(currentPage + 1)}
        disabled={currentPage >= totalPages}
        className="btn-ghost text-xs px-[var(--space-xs)] py-0.5"
        title="Next page (PageDown)"
      >
        &rarr;
      </button>
    </div>
  );
}
