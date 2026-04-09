'use client';

import { useCallback, useEffect, useRef, useState } from 'react';

interface RedactionRect {
  id: string;
  x: number;
  y: number;
  width: number;
  height: number;
  reason: string;
  pageNumber: number;
}

interface RedactionEditorProps {
  evidenceId: string;
  imageUrl: string;
  mimeType: string;
  accessToken: string;
  onApply: (redactions: RedactionRect[]) => Promise<void>;
  onClose: () => void;
}

export function RedactionEditor({
  imageUrl,
  onApply,
  onClose,
}: RedactionEditorProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const [rects, setRects] = useState<RedactionRect[]>([]);
  const [drawing, setDrawing] = useState(false);
  const [resizing, setResizing] = useState<{ id: string; edge: 'n' | 's' | 'e' | 'w' | 'ne' | 'nw' | 'se' | 'sw' } | null>(null);
  const [startPos, setStartPos] = useState<{ x: number; y: number } | null>(null);
  const [currentRect, setCurrentRect] = useState<{ x: number; y: number; width: number; height: number } | null>(null);
  const [selectedRect, setSelectedRect] = useState<string | null>(null);
  const [previewMode, setPreviewMode] = useState(false);
  const [applying, setApplying] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);
  const [imageLoaded, setImageLoaded] = useState(false);
  const imgRef = useRef<HTMLImageElement | null>(null);

  // Load image
  useEffect(() => {
    const img = new Image();
    img.crossOrigin = 'anonymous';
    img.onload = () => {
      imgRef.current = img;
      setImageLoaded(true);
    };
    img.src = imageUrl;
  }, [imageUrl]);

  // Draw canvas
  useEffect(() => {
    const canvas = canvasRef.current;
    const img = imgRef.current;
    if (!canvas || !img || !imageLoaded) return;

    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    canvas.width = img.naturalWidth;
    canvas.height = img.naturalHeight;

    ctx.drawImage(img, 0, 0);

    for (const rect of rects) {
      const x = (rect.x / 100) * canvas.width;
      const y = (rect.y / 100) * canvas.height;
      const w = (rect.width / 100) * canvas.width;
      const h = (rect.height / 100) * canvas.height;

      if (previewMode) {
        ctx.fillStyle = 'rgba(0, 0, 0, 1)';
      } else {
        ctx.fillStyle = rect.id === selectedRect ? 'rgba(200, 50, 50, 0.4)' : 'rgba(200, 50, 50, 0.25)';
      }
      ctx.fillRect(x, y, w, h);

      if (!previewMode) {
        ctx.strokeStyle = rect.id === selectedRect ? 'rgb(200, 50, 50)' : 'rgba(200, 50, 50, 0.6)';
        ctx.lineWidth = 2;
        ctx.strokeRect(x, y, w, h);
      }
    }

    // Draw current rect being drawn
    if (currentRect) {
      const x = (currentRect.x / 100) * canvas.width;
      const y = (currentRect.y / 100) * canvas.height;
      const w = (currentRect.width / 100) * canvas.width;
      const h = (currentRect.height / 100) * canvas.height;
      ctx.fillStyle = 'rgba(200, 50, 50, 0.25)';
      ctx.fillRect(x, y, w, h);
      ctx.strokeStyle = 'rgba(200, 50, 50, 0.8)';
      ctx.lineWidth = 2;
      ctx.setLineDash([5, 5]);
      ctx.strokeRect(x, y, w, h);
      ctx.setLineDash([]);
    }
  }, [rects, currentRect, previewMode, selectedRect, imageLoaded]);

  const getCanvasCoords = useCallback((e: React.MouseEvent) => {
    const canvas = canvasRef.current;
    if (!canvas) return { x: 0, y: 0 };
    const rect = canvas.getBoundingClientRect();
    return {
      x: ((e.clientX - rect.left) / rect.width) * 100,
      y: ((e.clientY - rect.top) / rect.height) * 100,
    };
  }, []);

  // Detect if click is near the edge of a selected rect for resizing
  const detectEdge = (pos: { x: number; y: number }): { id: string; edge: 'n' | 's' | 'e' | 'w' | 'ne' | 'nw' | 'se' | 'sw' } | null => {
    const threshold = 1.5; // percentage
    for (const rect of rects) {
      const nearLeft = Math.abs(pos.x - rect.x) < threshold && pos.y >= rect.y - threshold && pos.y <= rect.y + rect.height + threshold;
      const nearRight = Math.abs(pos.x - (rect.x + rect.width)) < threshold && pos.y >= rect.y - threshold && pos.y <= rect.y + rect.height + threshold;
      const nearTop = Math.abs(pos.y - rect.y) < threshold && pos.x >= rect.x - threshold && pos.x <= rect.x + rect.width + threshold;
      const nearBottom = Math.abs(pos.y - (rect.y + rect.height)) < threshold && pos.x >= rect.x - threshold && pos.x <= rect.x + rect.width + threshold;

      if (nearTop && nearLeft) return { id: rect.id, edge: 'nw' };
      if (nearTop && nearRight) return { id: rect.id, edge: 'ne' };
      if (nearBottom && nearLeft) return { id: rect.id, edge: 'sw' };
      if (nearBottom && nearRight) return { id: rect.id, edge: 'se' };
      if (nearLeft) return { id: rect.id, edge: 'w' };
      if (nearRight) return { id: rect.id, edge: 'e' };
      if (nearTop) return { id: rect.id, edge: 'n' };
      if (nearBottom) return { id: rect.id, edge: 's' };
    }
    return null;
  };

  const handleMouseDown = (e: React.MouseEvent) => {
    if (previewMode) return;
    const pos = getCanvasCoords(e);

    // Check for resize on existing rect edges
    const edge = detectEdge(pos);
    if (edge) {
      setResizing(edge);
      setSelectedRect(edge.id);
      setStartPos(pos);
      return;
    }

    setDrawing(true);
    setStartPos(pos);
    setSelectedRect(null);
  };

  const handleMouseMove = (e: React.MouseEvent) => {
    const pos = getCanvasCoords(e);

    // Handle resize
    if (resizing && startPos) {
      setRects((prev) =>
        prev.map((r) => {
          if (r.id !== resizing.id) return r;
          const dx = pos.x - startPos.x;
          const dy = pos.y - startPos.y;
          let { x, y, width, height } = r;

          if (resizing.edge.includes('w')) { x += dx; width -= dx; }
          if (resizing.edge.includes('e')) { width += dx; }
          if (resizing.edge.includes('n')) { y += dy; height -= dy; }
          if (resizing.edge.includes('s')) { height += dy; }

          // Enforce minimum size
          if (width < 0.5) { width = 0.5; }
          if (height < 0.5) { height = 0.5; }

          return { ...r, x, y, width, height };
        })
      );
      setStartPos(pos);
      return;
    }

    // Handle draw
    if (!drawing || !startPos) return;
    setCurrentRect({
      x: Math.min(startPos.x, pos.x),
      y: Math.min(startPos.y, pos.y),
      width: Math.abs(pos.x - startPos.x),
      height: Math.abs(pos.y - startPos.y),
    });
  };

  const handleMouseUp = () => {
    if (resizing) {
      setResizing(null);
      setStartPos(null);
      return;
    }

    if (!drawing || !currentRect) {
      setDrawing(false);
      setStartPos(null);
      return;
    }

    // Minimum size check
    if (currentRect.width > 0.5 && currentRect.height > 0.5) {
      const newRect: RedactionRect = {
        id: crypto.randomUUID(),
        ...currentRect,
        reason: '',
        pageNumber: 0,
      };
      setRects((prev) => [...prev, newRect]);
      setSelectedRect(newRect.id);
    }

    setDrawing(false);
    setStartPos(null);
    setCurrentRect(null);
  };

  const updateReason = (id: string, reason: string) => {
    setRects((prev) =>
      prev.map((r) => (r.id === id ? { ...r, reason } : r))
    );
  };

  const deleteRect = (id: string) => {
    setRects((prev) => prev.filter((r) => r.id !== id));
    if (selectedRect === id) setSelectedRect(null);
  };

  const handleApply = async () => {
    setApplying(true);
    try {
      await onApply(rects);
    } finally {
      setApplying(false);
      setShowConfirm(false);
    }
  };

  const allReasonsProvided = rects.every((r) => r.reason.trim() !== '');

  return (
    <div
      className="fixed inset-0 z-50 flex"
      style={{ backgroundColor: 'var(--bg-primary)' }}
    >
      {/* Sidebar */}
      <div
        className="w-80 flex-shrink-0 overflow-y-auto border-r p-[var(--space-md)] space-y-[var(--space-md)]"
        style={{ borderColor: 'var(--border-default)', backgroundColor: 'var(--bg-elevated)' }}
      >
        <div className="flex items-center justify-between">
          <h3
            className="font-[family-name:var(--font-heading)] text-lg"
            style={{ color: 'var(--text-primary)' }}
          >
            Redaction Editor
          </h3>
          <button onClick={onClose} className="btn-ghost text-sm">Close</button>
        </div>

        <p className="text-xs" style={{ color: 'var(--text-tertiary)' }}>
          Click and drag on the image to draw redaction areas. Each area requires a reason.
        </p>

        {rects.length === 0 ? (
          <div className="text-center py-[var(--space-lg)]">
            <p className="text-sm" style={{ color: 'var(--text-tertiary)' }}>
              No redaction areas yet
            </p>
          </div>
        ) : (
          <div className="space-y-[var(--space-sm)]">
            {rects.map((rect, i) => (
              <div
                key={rect.id}
                className="p-[var(--space-sm)] rounded"
                style={{
                  backgroundColor: rect.id === selectedRect ? 'var(--amber-subtle)' : 'transparent',
                  border: `1px solid ${rect.id === selectedRect ? 'var(--amber-accent)' : 'var(--border-default)'}`,
                }}
                onClick={() => setSelectedRect(rect.id)}
              >
                <div className="flex items-center justify-between mb-[var(--space-xs)]">
                  <span className="text-xs font-medium" style={{ color: 'var(--text-primary)' }}>
                    Area {i + 1}
                  </span>
                  <button
                    onClick={(e) => { e.stopPropagation(); deleteRect(rect.id); }}
                    className="text-xs"
                    style={{ color: 'var(--status-hold)' }}
                  >
                    Remove
                  </button>
                </div>
                <input
                  type="text"
                  value={rect.reason}
                  onChange={(e) => updateReason(rect.id, e.target.value)}
                  placeholder="Reason for redaction..."
                  className="input-field text-xs"
                  required
                />
              </div>
            ))}
          </div>
        )}

        <div className="space-y-[var(--space-sm)] pt-[var(--space-md)]" style={{ borderTop: '1px solid var(--border-default)' }}>
          <button
            onClick={() => setPreviewMode(!previewMode)}
            disabled={rects.length === 0}
            className="btn-secondary w-full"
          >
            {previewMode ? 'Edit Mode' : 'Preview'}
          </button>
          <button
            onClick={() => setShowConfirm(true)}
            disabled={rects.length === 0 || !allReasonsProvided}
            className="btn-primary w-full"
          >
            Apply Redactions
          </button>
          {rects.length > 0 && !allReasonsProvided && (
            <p className="text-xs text-center" style={{ color: 'var(--amber-accent)' }}>
              All areas require a reason
            </p>
          )}
        </div>
      </div>

      {/* Canvas area */}
      <div
        ref={containerRef}
        className="flex-1 overflow-auto flex items-center justify-center p-[var(--space-lg)]"
        style={{ backgroundColor: 'var(--bg-inset)' }}
      >
        {imageLoaded ? (
          <canvas
            ref={canvasRef}
            className="max-w-full max-h-full shadow-lg"
            style={{ cursor: previewMode ? 'default' : 'crosshair' }}
            onMouseDown={handleMouseDown}
            onMouseMove={handleMouseMove}
            onMouseUp={handleMouseUp}
            onMouseLeave={handleMouseUp}
          />
        ) : (
          <p className="text-sm" style={{ color: 'var(--text-tertiary)' }}>Loading image...</p>
        )}
      </div>

      {/* Confirmation dialog */}
      {showConfirm && (
        <div className="fixed inset-0 z-60 flex items-center justify-center" style={{ backgroundColor: 'oklch(0.2 0.01 50 / 0.6)' }}>
          <div className="card max-w-md w-full mx-[var(--space-lg)] p-[var(--space-lg)] space-y-[var(--space-md)]">
            <h4 className="font-[family-name:var(--font-heading)] text-lg" style={{ color: 'var(--text-primary)' }}>
              Apply Redactions
            </h4>
            <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
              This will create a new copy with {rects.length} redacted area{rects.length !== 1 ? 's' : ''}.
              The original file is preserved.
            </p>
            <div className="flex gap-[var(--space-sm)] justify-end">
              <button onClick={() => setShowConfirm(false)} className="btn-ghost">Cancel</button>
              <button onClick={handleApply} disabled={applying} className="btn-primary">
                {applying ? 'Applying...' : 'Confirm'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
