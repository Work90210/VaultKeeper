'use client';

interface ZoomControlsProps {
  zoom: number;
  onZoomIn: () => void;
  onZoomOut: () => void;
  onFitToWidth: () => void;
}

export function ZoomControls({ zoom, onZoomIn, onZoomOut, onFitToWidth }: ZoomControlsProps) {
  return (
    <div className="flex items-center gap-[var(--space-xs)]">
      <button
        onClick={onZoomOut}
        disabled={zoom <= 0.5}
        className="btn-ghost text-xs px-[var(--space-xs)] py-0.5"
        title="Zoom out (-)"
      >
        &minus;
      </button>
      <span
        className="text-xs tabular-nums min-w-[3rem] text-center"
        style={{ color: 'var(--text-secondary)' }}
      >
        {Math.round(zoom * 100)}%
      </span>
      <button
        onClick={onZoomIn}
        disabled={zoom >= 2}
        className="btn-ghost text-xs px-[var(--space-xs)] py-0.5"
        title="Zoom in (+)"
      >
        +
      </button>
      <button
        onClick={onFitToWidth}
        className="btn-ghost text-xs px-[var(--space-xs)] py-0.5"
        title="Fit to page (Ctrl+0)"
      >
        Fit
      </button>
    </div>
  );
}
