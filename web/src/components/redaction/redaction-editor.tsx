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

  const detectEdge = (pos: { x: number; y: number }): { id: string; edge: 'n' | 's' | 'e' | 'w' | 'ne' | 'nw' | 'se' | 'sw' } | null => {
    const threshold = 1.5;
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

          if (width < 0.5) { width = 0.5; }
          if (height < 0.5) { height = 0.5; }

          return { ...r, x, y, width, height };
        })
      );
      setStartPos(pos);
      return;
    }

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
      style={{
        position: 'fixed',
        inset: 0,
        zIndex: 50,
        display: 'flex',
        background: 'var(--paper)',
      }}
    >
      {/* Sidebar */}
      <div
        style={{
          width: 320,
          flexShrink: 0,
          overflowY: 'auto',
          borderRight: '1px solid var(--line)',
          background: 'var(--bg)',
          padding: 16,
          display: 'flex',
          flexDirection: 'column',
          gap: 14,
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <h3 style={{ fontFamily: "'Fraunces', serif", fontSize: 18, margin: 0 }}>
            Redaction <em>editor</em>
          </h3>
          <button onClick={onClose} className="chip">Close</button>
        </div>

        <p style={{ fontSize: '12.5px', color: 'var(--muted)', margin: 0 }}>
          Click and drag on the image to draw redaction areas. Each area requires a reason.
        </p>

        {rects.length === 0 ? (
          <div style={{ textAlign: 'center', padding: '30px 0' }}>
            <p style={{ fontSize: '13.5px', color: 'var(--muted)' }}>
              No redaction areas yet
            </p>
          </div>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            {rects.map((rect, i) => (
              <div
                key={rect.id}
                onClick={() => setSelectedRect(rect.id)}
                style={{
                  padding: '10px 12px',
                  borderRadius: 8,
                  border: `1px solid ${rect.id === selectedRect ? 'var(--accent)' : 'var(--line)'}`,
                  background: rect.id === selectedRect ? 'var(--bg-2)' : 'transparent',
                  cursor: 'pointer',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 6 }}>
                  <span style={{ fontSize: '12px', fontWeight: 500, color: 'var(--ink)' }}>
                    Area {i + 1}
                  </span>
                  <button
                    onClick={(e) => { e.stopPropagation(); deleteRect(rect.id); }}
                    style={{ all: 'unset', cursor: 'pointer', fontSize: 12, color: 'var(--err)' }}
                  >
                    Remove
                  </button>
                </div>
                <input
                  type="text"
                  value={rect.reason}
                  onChange={(e) => updateReason(rect.id, e.target.value)}
                  placeholder="Reason for redaction..."
                  style={{
                    width: '100%',
                    padding: '6px 10px',
                    fontSize: '12.5px',
                    border: '1px solid var(--line)',
                    borderRadius: 6,
                    background: 'var(--paper)',
                    color: 'var(--ink)',
                  }}
                />
              </div>
            ))}
          </div>
        )}

        <div style={{ borderTop: '1px solid var(--line)', paddingTop: 14, display: 'flex', flexDirection: 'column', gap: 8, marginTop: 'auto' }}>
          <button
            onClick={() => setPreviewMode(!previewMode)}
            disabled={rects.length === 0}
            className="btn ghost"
            style={{ width: '100%', justifyContent: 'center' }}
          >
            {previewMode ? 'Edit Mode' : 'Preview'}
          </button>
          <button
            onClick={() => setShowConfirm(true)}
            disabled={rects.length === 0 || !allReasonsProvided}
            className="btn"
            style={{ width: '100%', justifyContent: 'center' }}
          >
            Apply Redactions <span className="arr">&rarr;</span>
          </button>
          {rects.length > 0 && !allReasonsProvided && (
            <p style={{ fontSize: '11px', textAlign: 'center', color: 'var(--err)', margin: 0 }}>
              All areas require a reason
            </p>
          )}
        </div>
      </div>

      {/* Canvas area */}
      <div
        ref={containerRef}
        style={{
          flex: 1,
          overflow: 'auto',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          padding: 30,
          background: 'var(--bg-2)',
        }}
      >
        {imageLoaded ? (
          <canvas
            ref={canvasRef}
            style={{
              maxWidth: '100%',
              maxHeight: '100%',
              boxShadow: '0 4px 24px rgba(0,0,0,.12)',
              cursor: previewMode ? 'default' : 'crosshair',
            }}
            onMouseDown={handleMouseDown}
            onMouseMove={handleMouseMove}
            onMouseUp={handleMouseUp}
            onMouseLeave={handleMouseUp}
          />
        ) : (
          <p style={{ fontSize: '13.5px', color: 'var(--muted)' }}>Loading image&hellip;</p>
        )}
      </div>

      {/* Confirmation dialog */}
      {showConfirm && (
        <div
          style={{
            position: 'fixed',
            inset: 0,
            zIndex: 60,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            background: 'rgba(0,0,0,.45)',
          }}
        >
          <div className="panel" style={{ maxWidth: 440, width: '100%', margin: '0 24px' }}>
            <div className="panel-h">
              <h3>Apply Redactions</h3>
            </div>
            <div className="panel-body" style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
              <p style={{ fontSize: '13.5px', color: 'var(--ink-2)', margin: 0 }}>
                This will create a new copy with {rects.length} redacted area{rects.length !== 1 ? 's' : ''}.
                The original file is preserved.
              </p>
              <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
                <button onClick={() => setShowConfirm(false)} className="btn ghost">Cancel</button>
                <button onClick={handleApply} disabled={applying} className="btn">
                  {applying ? 'Applying\u2026' : 'Confirm'} <span className="arr">&rarr;</span>
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
