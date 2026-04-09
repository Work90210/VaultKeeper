'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { usePdf } from '@/hooks/use-pdf';
import { PageNavigator } from './page-navigator';
import { ZoomControls } from './zoom-controls';
import { getUserColor, PresenceIndicator } from './presence-indicator';
interface CollaborativeEditorProps {
  evidenceId: string;
  draftId: string;
  draftName: string;
  draftPurpose: string;
  totalPages: number;
  accessToken: string;
  username: string;
  onClose: () => void;
  onApplied?: (newEvidenceId: string) => void;
}

interface RedactionRect {
  id: string;
  page: number;
  x: number;
  y: number;
  w: number;
  h: number;
  reason: string;
  author: string;
  color: string;
}

type SaveStatus = 'idle' | 'saving' | 'saved' | 'unsaved' | 'error';

interface DraftResponse {
  data: {
    draft_id: string;
    areas: RedactionRect[];
    last_saved_at: string;
  };
}

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

async function fetchDraft(
  accessToken: string,
  evidenceId: string,
  draftId: string
): Promise<{ data: DraftResponse['data'] | null; error: string | null }> {
  const res = await fetch(`${API_BASE}/api/evidence/${evidenceId}/redact/drafts/${draftId}`, {
    headers: { Authorization: `Bearer ${accessToken}` },
  });
  if (res.status === 404) return { data: null, error: null };
  if (!res.ok) {
    const body = await res.json().catch(() => null);
    return { data: null, error: body?.error ?? `Server error ${res.status}` };
  }
  const json = await res.json();
  return { data: json.data, error: null };
}

async function saveDraft(
  accessToken: string,
  evidenceId: string,
  draftId: string,
  areas: RedactionRect[]
): Promise<{ data: { draft_id: string; last_saved_at: string } | null; error: string | null }> {
  const res = await fetch(`${API_BASE}/api/evidence/${evidenceId}/redact/drafts/${draftId}`, {
    method: 'PUT',
    headers: {
      Authorization: `Bearer ${accessToken}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ areas }),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => null);
    return { data: null, error: body?.error ?? `Server error ${res.status}` };
  }
  const json = await res.json();
  return { data: json.data, error: null };
}

async function discardDraft(
  accessToken: string,
  evidenceId: string,
  draftId: string
): Promise<{ error: string | null }> {
  const res = await fetch(`${API_BASE}/api/evidence/${evidenceId}/redact/drafts/${draftId}`, {
    method: 'DELETE',
    headers: { Authorization: `Bearer ${accessToken}` },
  });
  if (!res.ok) {
    const body = await res.json().catch(() => null);
    return { error: body?.error ?? `Server error ${res.status}` };
  }
  return { error: null };
}

async function finalizeDraft(
  accessToken: string,
  evidenceId: string,
  draftId: string,
  description: string,
  classification: string
) {
  const res = await fetch(
    `${API_BASE}/api/evidence/${evidenceId}/redact/drafts/${draftId}/finalize`,
    {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${accessToken}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ description, classification }),
    }
  );
  if (!res.ok) {
    const body = await res.json().catch(() => null);
    return { data: null, error: body?.error ?? `Server error ${res.status}` };
  }
  return res.json();
}

export function CollaborativeEditor({
  evidenceId,
  draftId,
  draftName,
  draftPurpose,
  totalPages,
  accessToken,
  username,
  onClose,
  onApplied,
}: CollaborativeEditorProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  const [imageElement, setImageElement] = useState<HTMLImageElement | null>(null);
  const [canvasCursor, setCanvasCursor] = useState('crosshair');
  const [rects, setRects] = useState<RedactionRect[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [previewMode, setPreviewMode] = useState(false);
  const [applying, setApplying] = useState(false);
  const [applyError, setApplyError] = useState<string | null>(null);
  const [showConfirm, setShowConfirm] = useState(false);
  const [imageLoaded, setImageLoaded] = useState(false);
  const [currentRect, setCurrentRect] = useState<{ x: number; y: number; w: number; h: number } | null>(null);
  const [localResizeOverride, setLocalResizeOverride] = useState<Partial<RedactionRect> & { id: string } | null>(null);
  // Use refs for drag state — must be synchronous across rapid mouse events
  const dragRef = useRef<{
    mode: 'none' | 'draw' | 'resize' | 'move';
    startPos: { x: number; y: number } | null;
    targetId: string | null;
    edge: string | null;
  }>({ mode: 'none', startPos: null, targetId: null, edge: null });
  const preDragRectRef = useRef<RedactionRect | null>(null);
  const blobUrlRef = useRef<string | null>(null);
  const applyingRef = useRef(false);
  const [connectedUsers] = useState<{ id: string; name: string; color: string }[]>([]);
  const [saveStatus, setSaveStatus] = useState<SaveStatus>('idle');
  const [lastSavedAt, setLastSavedAt] = useState<string | null>(null);
  const [draftLoaded, setDraftLoaded] = useState(false);
  const [showDiscardConfirm, setShowDiscardConfirm] = useState(false);

  // History stack for undo/redo
  const historyRef = useRef<RedactionRect[][]>([[]]);
  const historyIndexRef = useRef(0);

  const pushHistory = useCallback((snapshot: RedactionRect[]) => {
    const history = historyRef.current;
    const idx = historyIndexRef.current;
    // Truncate any redo entries beyond current index
    const next = [...history.slice(0, idx + 1), snapshot];
    historyRef.current = next;
    historyIndexRef.current = next.length - 1;
  }, []);

  const undo = useCallback(() => {
    if (historyIndexRef.current <= 0) return;
    historyIndexRef.current -= 1;
    setRects(historyRef.current[historyIndexRef.current]);
  }, []);

  const redo = useCallback(() => {
    if (historyIndexRef.current >= historyRef.current.length - 1) return;
    historyIndexRef.current += 1;
    setRects(historyRef.current[historyIndexRef.current]);
  }, []);

  const pdf = usePdf({ evidenceId, totalPages });

  // Load draft on mount
  useEffect(() => {
    let cancelled = false;
    (async () => {
      const result = await fetchDraft(accessToken, evidenceId, draftId);
      if (cancelled) return;
      if (result.data) {
        setRects(result.data.areas);
        historyRef.current = [result.data.areas];
        historyIndexRef.current = 0;
        setLastSavedAt(result.data.last_saved_at);
        setSaveStatus('saved');
      }
      setDraftLoaded(true);
    })();
    return () => { cancelled = true; };
  }, [accessToken, evidenceId, draftId]);

  // Auto-save helper
  const rectsForSave = useRef(rects);
  rectsForSave.current = rects;
  const doSave = useCallback(async () => {
    setSaveStatus('saving');
    const result = await saveDraft(accessToken, evidenceId, draftId, rectsForSave.current);
    if (result.data) {
      setLastSavedAt(result.data.last_saved_at);
      setSaveStatus('saved');
    } else {
      setSaveStatus('error');
    }
  }, [accessToken, evidenceId, draftId]);

  // Auto-save with 1.5s debounce on rects change
  useEffect(() => {
    if (!draftLoaded) return;
    if (rects.length === 0 && saveStatus === 'idle') return;

    setSaveStatus('unsaved');
    const timer = setTimeout(doSave, 1500);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rects, draftLoaded, doSave]);

  // Also save immediately on page navigation (flush any pending changes)
  const prevPageRef = useRef(pdf.currentPage);
  useEffect(() => {
    if (prevPageRef.current !== pdf.currentPage && draftLoaded && rects.length > 0) {
      doSave();
    }
    prevPageRef.current = pdf.currentPage;
  }, [pdf.currentPage, draftLoaded, rects.length, doSave]);

  // Load page image with deterministic blob URL lifecycle via ref
  useEffect(() => {
    if (totalPages === 0) return;
    setImageLoaded(false);
    let cancelled = false;

    const headers: HeadersInit = { Authorization: `Bearer ${accessToken}` };
    fetch(pdf.pageImageUrl, { headers })
      .then((res) => res.blob())
      .then((blob) => {
        if (cancelled) return;
        const newUrl = URL.createObjectURL(blob);
        const img = new Image();
        img.onload = () => {
          if (cancelled) {
            URL.revokeObjectURL(newUrl);
            return;
          }
          // Revoke previous blob URL deterministically via ref
          if (blobUrlRef.current) URL.revokeObjectURL(blobUrlRef.current);
          blobUrlRef.current = newUrl;
          setImageElement(img);
          setImageLoaded(true);
        };
        img.onerror = () => {
          URL.revokeObjectURL(newUrl);
          if (!cancelled) setImageLoaded(false);
        };
        img.src = newUrl;
      })
      .catch(() => {
        if (!cancelled) setImageLoaded(false);
      });

    return () => { cancelled = true; };
  }, [pdf.pageImageUrl, accessToken, totalPages]);

  // Render canvas — uses imageElement state (reactive) instead of ref
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas || !imageElement || !imageLoaded) return;

    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    canvas.width = imageElement.naturalWidth;
    canvas.height = imageElement.naturalHeight;
    ctx.drawImage(imageElement, 0, 0);

    const pageRects = rects.filter((r) => r.page === pdf.currentPage);

    for (const rect of pageRects) {
      // Apply local resize override if this rect is being dragged
      const effective = localResizeOverride?.id === rect.id
        ? { ...rect, ...localResizeOverride }
        : rect;

      const x = (effective.x / 100) * canvas.width;
      const y = (effective.y / 100) * canvas.height;
      const w = (effective.w / 100) * canvas.width;
      const h = (effective.h / 100) * canvas.height;

      if (previewMode) {
        ctx.fillStyle = 'rgba(0, 0, 0, 1)';
      } else {
        ctx.fillStyle = rect.id === selectedId
          ? 'rgba(200, 50, 50, 0.4)'
          : 'rgba(200, 50, 50, 0.25)';
      }
      ctx.fillRect(x, y, w, h);

      if (!previewMode) {
        ctx.strokeStyle = rect.id === selectedId ? 'rgb(200, 50, 50)' : 'rgba(200, 50, 50, 0.6)';
        ctx.lineWidth = 2;
        ctx.strokeRect(x, y, w, h);
      }
    }

    // Draw rect being created
    if (currentRect) {
      const x = (currentRect.x / 100) * canvas.width;
      const y = (currentRect.y / 100) * canvas.height;
      const w = (currentRect.w / 100) * canvas.width;
      const h = (currentRect.h / 100) * canvas.height;
      ctx.fillStyle = 'rgba(200, 50, 50, 0.25)';
      ctx.fillRect(x, y, w, h);
      ctx.strokeStyle = 'rgba(200, 50, 50, 0.8)';
      ctx.lineWidth = 2;
      ctx.setLineDash([5, 5]);
      ctx.strokeRect(x, y, w, h);
      ctx.setLineDash([]);
    }
  }, [rects, currentRect, previewMode, selectedId, imageLoaded, imageElement, pdf.currentPage, localResizeOverride]);

  // Canvas coordinate helpers — getBoundingClientRect returns post-transform
  // dimensions, dividing by rect.width cancels the CSS scale factor naturally
  const getCanvasCoords = useCallback((e: React.MouseEvent) => {
    const canvas = canvasRef.current;
    if (!canvas) return { x: 0, y: 0 };
    const rect = canvas.getBoundingClientRect();
    return {
      x: ((e.clientX - rect.left) / rect.width) * 100,
      y: ((e.clientY - rect.top) / rect.height) * 100,
    };
  }, []);

  // Detect edge for resizing
  const detectEdge = useCallback((pos: { x: number; y: number }) => {
    const threshold = 1.5;
    const pageRects = rects.filter((r) => r.page === pdf.currentPage);

    for (const rect of pageRects) {
      const nearLeft = Math.abs(pos.x - rect.x) < threshold && pos.y >= rect.y - threshold && pos.y <= rect.y + rect.h + threshold;
      const nearRight = Math.abs(pos.x - (rect.x + rect.w)) < threshold && pos.y >= rect.y - threshold && pos.y <= rect.y + rect.h + threshold;
      const nearTop = Math.abs(pos.y - rect.y) < threshold && pos.x >= rect.x - threshold && pos.x <= rect.x + rect.w + threshold;
      const nearBottom = Math.abs(pos.y - (rect.y + rect.h)) < threshold && pos.x >= rect.x - threshold && pos.x <= rect.x + rect.w + threshold;

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
  }, [rects, pdf.currentPage]);

  // Hit test — check if pos is inside a rect
  const hitTest = useCallback((pos: { x: number; y: number }) => {
    const pageRects = rects.filter((r) => r.page === pdf.currentPage);
    for (const rect of pageRects) {
      if (pos.x >= rect.x && pos.x <= rect.x + rect.w &&
          pos.y >= rect.y && pos.y <= rect.y + rect.h) {
        return rect.id;
      }
    }
    return null;
  }, [rects, pdf.currentPage]);

  // Mutation helpers (plain React state with history)
  const addRedaction = useCallback((rect: Omit<RedactionRect, 'id' | 'author' | 'color'>) => {
    const id = crypto.randomUUID();
    const newRect: RedactionRect = {
      ...rect,
      id,
      author: username,
      color: getUserColor(0),
    };
    setRects((prev) => {
      const next = [...prev, newRect];
      pushHistory(next);
      return next;
    });
    return id;
  }, [username, pushHistory]);

  const updateRedaction = useCallback((id: string, updates: Partial<RedactionRect>) => {
    setRects((prev) => {
      const next = prev.map((r) => (r.id === id ? { ...r, ...updates } : r));
      pushHistory(next);
      return next;
    });
  }, [pushHistory]);

  const deleteRedaction = useCallback((id: string) => {
    setRects((prev) => {
      const next = prev.filter((r) => r.id !== id);
      pushHistory(next);
      return next;
    });
    if (selectedId === id) setSelectedId(null);
  }, [selectedId, pushHistory]);

  // Mouse handlers — use dragRef for synchronous state across rapid events
  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    if (previewMode) return;
    const pos = getCanvasCoords(e);

    // 1. Edge resize
    const edge = detectEdge(pos);
    if (edge) {
      const rect = rects.find((r) => r.id === edge.id);
      if (rect) preDragRectRef.current = { ...rect };
      dragRef.current = { mode: 'resize', startPos: pos, targetId: edge.id, edge: edge.edge };
      setSelectedId(edge.id);
      return;
    }

    // 2. Click inside rect → select + move
    const hitId = hitTest(pos);
    if (hitId) {
      const rect = rects.find((r) => r.id === hitId);
      if (rect) preDragRectRef.current = { ...rect };
      dragRef.current = { mode: 'move', startPos: pos, targetId: hitId, edge: null };
      setSelectedId(hitId);
      setCanvasCursor('grabbing');
      return;
    }

    // 3. Draw new rect
    dragRef.current = { mode: 'draw', startPos: pos, targetId: null, edge: null };
    setSelectedId(null);
  }, [previewMode, getCanvasCoords, detectEdge, hitTest, rects]);

  const handleMouseMove = useCallback((e: React.MouseEvent) => {
    const pos = getCanvasCoords(e);
    const drag = dragRef.current;

    if (drag.mode === 'resize' && drag.startPos && drag.targetId && drag.edge) {
      const rect = localResizeOverride?.id === drag.targetId
        ? { ...rects.find((r) => r.id === drag.targetId)!, ...localResizeOverride }
        : rects.find((r) => r.id === drag.targetId);
      if (!rect) return;

      const dx = pos.x - drag.startPos.x;
      const dy = pos.y - drag.startPos.y;
      let { x, y, w, h } = rect;

      if (drag.edge.includes('w')) { x += dx; w -= dx; }
      if (drag.edge.includes('e')) { w += dx; }
      if (drag.edge.includes('n')) { y += dy; h -= dy; }
      if (drag.edge.includes('s')) { h += dy; }
      if (w < 0.5) w = 0.5;
      if (h < 0.5) h = 0.5;

      setLocalResizeOverride({ id: drag.targetId, x, y, w, h });
      drag.startPos = pos;
      return;
    }

    if (drag.mode === 'move' && drag.startPos && drag.targetId) {
      const rect = localResizeOverride?.id === drag.targetId
        ? { ...rects.find((r) => r.id === drag.targetId)!, ...localResizeOverride }
        : rects.find((r) => r.id === drag.targetId);
      if (!rect) return;

      const dx = pos.x - drag.startPos.x;
      const dy = pos.y - drag.startPos.y;
      setLocalResizeOverride({ id: drag.targetId, x: rect.x + dx, y: rect.y + dy, w: rect.w, h: rect.h });
      drag.startPos = pos;
      return;
    }

    if (drag.mode === 'draw' && drag.startPos) {
      setCurrentRect({
        x: Math.min(drag.startPos.x, pos.x),
        y: Math.min(drag.startPos.y, pos.y),
        w: Math.abs(pos.x - drag.startPos.x),
        h: Math.abs(pos.y - drag.startPos.y),
      });
      return;
    }

    // Idle — update cursor
    if (!previewMode) {
      const edgeHit = detectEdge(pos);
      if (edgeHit) {
        const cursorMap: Record<string, string> = {
          n: 'ns-resize', s: 'ns-resize', e: 'ew-resize', w: 'ew-resize',
          nw: 'nwse-resize', se: 'nwse-resize', ne: 'nesw-resize', sw: 'nesw-resize',
        };
        setCanvasCursor(cursorMap[edgeHit.edge] || 'crosshair');
      } else if (hitTest(pos)) {
        setCanvasCursor('grab');
      } else {
        setCanvasCursor('crosshair');
      }
    }
  }, [getCanvasCoords, rects, localResizeOverride, previewMode, detectEdge, hitTest]);

  const handleMouseUp = useCallback(() => {
    const drag = dragRef.current;

    if (drag.mode === 'resize' && drag.targetId && localResizeOverride) {
      updateRedaction(drag.targetId, {
        x: localResizeOverride.x, y: localResizeOverride.y,
        w: localResizeOverride.w, h: localResizeOverride.h,
      });
      setLocalResizeOverride(null);
      preDragRectRef.current = null;
    } else if (drag.mode === 'move' && drag.targetId && localResizeOverride) {
      updateRedaction(drag.targetId, {
        x: localResizeOverride.x, y: localResizeOverride.y,
      });
      setLocalResizeOverride(null);
      preDragRectRef.current = null;
    } else if (drag.mode === 'draw' && currentRect && currentRect.w > 0.5 && currentRect.h > 0.5) {
      const id = addRedaction({
        page: pdf.currentPage,
        x: currentRect.x, y: currentRect.y,
        w: currentRect.w, h: currentRect.h,
        reason: '',
      });
      setSelectedId(id);
    }

    dragRef.current = { mode: 'none', startPos: null, targetId: null, edge: null };
    setCurrentRect(null);
    setCanvasCursor('crosshair');
  }, [localResizeOverride, currentRect, addRedaction, pdf.currentPage, updateRedaction]);

  // Keyboard shortcuts — destructure to avoid object identity churn
  const { prevPage, nextPage, zoomIn, zoomOut, fitToWidth } = pdf;
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      const metaOrCtrl = e.metaKey || e.ctrlKey;
      const isTextInput = e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement;

      // Ctrl/Cmd shortcuts always work (undo/redo redaction operations)
      if (metaOrCtrl && e.key === 'z' && !e.shiftKey) {
        e.preventDefault();
        undo();
        return;
      }
      if (metaOrCtrl && e.key === 'z' && e.shiftKey) {
        e.preventDefault();
        redo();
        return;
      }
      if (metaOrCtrl && e.key === '0') { e.preventDefault(); fitToWidth(); return; }

      // Escape always works — blurs input or deselects
      if (e.key === 'Escape') {
        if (isTextInput) {
          (e.target as HTMLElement).blur();
        } else {
          setSelectedId(null);
          setPreviewMode(false);
        }
        return;
      }

      // All other shortcuts are blocked when typing in a text field
      if (isTextInput) return;

      if ((e.key === 'Delete' || e.key === 'Backspace') && selectedId) {
        e.preventDefault();
        deleteRedaction(selectedId);
      }
      if (e.key === 'PageUp') { e.preventDefault(); prevPage(); }
      if (e.key === 'PageDown') { e.preventDefault(); nextPage(); }
      if (e.key === '+' || e.key === '=') { e.preventDefault(); zoomIn(); }
      if (e.key === '-') { e.preventDefault(); zoomOut(); }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [undo, redo, selectedId, deleteRedaction, prevPage, nextPage, zoomIn, zoomOut, fitToWidth]);

  // Finalize description state
  const [finalizeDescription, setFinalizeDescription] = useState('');

  // Finalize draft — creates permanent redacted copy via the finalize endpoint
  const handleApply = useCallback(async () => {
    if (applyingRef.current) return;
    applyingRef.current = true;
    setApplying(true);
    setApplyError(null);
    try {
      // Flush save first
      await doSave().catch(() => {});

      const result = await finalizeDraft(accessToken, evidenceId, draftId, finalizeDescription, '');
      if (result.error || !result.data) {
        setApplyError(result.error ?? 'Failed to finalize. Please try again.');
        return;
      }
      onApplied?.(result.data.new_evidence_id);
    } catch {
      setApplyError('An unexpected error occurred. Please try again.');
    } finally {
      applyingRef.current = false;
      setApplying(false);
    }
  }, [evidenceId, draftId, onApplied, accessToken, finalizeDescription, doSave]);

  // Close — flush pending save before closing
  const handleClose = useCallback(async () => {
    await doSave().catch(() => {});
    onClose();
  }, [doSave, onClose]);

  // Discard draft — DELETE endpoint, clear state, close editor
  const handleDiscard = useCallback(async () => {
    setShowDiscardConfirm(false);
    try {
      await discardDraft(accessToken, evidenceId, draftId);
    } catch {
      // Best-effort — close anyway
    }
    setRects([]);
    historyRef.current = [[]];
    historyIndexRef.current = 0;
    setLastSavedAt(null);
    setSaveStatus('idle');
    onClose();
  }, [accessToken, evidenceId, draftId, onClose]);

  const allReasonsProvided = rects.every((r) => r.reason.trim() !== '');

  // Group rects by page for sidebar (immutable)
  const rectsByPage = useMemo(() => {
    const groups: Map<number, RedactionRect[]> = new Map();
    for (const r of rects) {
      groups.set(r.page, [...(groups.get(r.page) ?? []), r]);
    }
    return groups;
  }, [rects]);

  return (
    <div className="fixed inset-0 z-[100] flex" style={{ backgroundColor: 'var(--bg-primary)' }}>
      {/* ── Sidebar ── */}
      <div
        className="w-72 flex-shrink-0 flex flex-col border-r overflow-hidden"
        style={{ borderColor: 'var(--border-default)', backgroundColor: 'var(--bg-elevated)' }}
      >
        {/* Sidebar header: title + toolbar */}
        <div className="p-[var(--space-sm)] border-b" style={{ borderColor: 'var(--border-default)' }}>
          <div className="flex items-center justify-between mb-[var(--space-sm)]">
            <div className="flex items-center gap-[var(--space-sm)] min-w-0">
              <h3 className="text-sm font-semibold truncate" style={{ color: 'var(--text-primary)' }} title={draftName}>
                {draftName}
              </h3>
              <span
                className="text-xs"
                style={{
                  color: saveStatus === 'saved' ? 'var(--status-active)'
                    : saveStatus === 'saving' ? 'var(--text-tertiary)'
                    : saveStatus === 'error' ? 'var(--status-hold)'
                    : saveStatus === 'unsaved' ? 'var(--amber-accent)'
                    : 'var(--text-tertiary)',
                }}
                title={lastSavedAt ? `Last saved: ${new Date(lastSavedAt).toLocaleTimeString()}` : undefined}
              >
                {saveStatus === 'saved' && lastSavedAt
                  ? `Saved ${new Date(lastSavedAt).toLocaleTimeString()}`
                  : saveStatus === 'saving' ? 'Saving\u2026'
                  : saveStatus === 'unsaved' ? 'Unsaved changes'
                  : saveStatus === 'error' ? 'Save failed'
                  : ''}
              </span>
            </div>
            <button type="button" onClick={handleClose} className="btn-ghost text-xs">
              Close
            </button>
          </div>

          {/* Undo/Redo + Presence in one row */}
          <div className="flex items-center justify-between">
            <div className="flex gap-px">
              <button
                type="button"
                onClick={undo}
                className="btn-ghost text-xs px-[var(--space-sm)]"
                title="Undo (Ctrl+Z)"
              >
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M3 7v6h6"/><path d="M21 17a9 9 0 0 0-9-9 9 9 0 0 0-6 2.3L3 13"/></svg>
              </button>
              <button
                type="button"
                onClick={redo}
                className="btn-ghost text-xs px-[var(--space-sm)]"
                title="Redo (Ctrl+Shift+Z)"
              >
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M21 7v6h-6"/><path d="M3 17a9 9 0 0 1 9-9 9 9 0 0 1 6 2.3L21 13"/></svg>
              </button>
            </div>
            <PresenceIndicator users={connectedUsers} />
          </div>
        </div>

        {/* Area list */}
        <div className="flex-1 overflow-y-auto p-[var(--space-sm)]">
          {rects.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-full text-center px-[var(--space-md)]">
              <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="var(--text-tertiary)" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" className="mb-[var(--space-sm)]" style={{ opacity: 0.5 }}>
                <rect x="3" y="3" width="18" height="18" rx="2"/>
                <path d="M3 9h18"/>
                <path d="M9 21V9"/>
              </svg>
              <p className="text-xs" style={{ color: 'var(--text-tertiary)' }}>
                Click and drag on the document to mark areas for redaction
              </p>
            </div>
          ) : (
            <div className="space-y-[var(--space-sm)]">
              {Array.from(rectsByPage.entries())
                .sort(([a], [b]) => a - b)
                .map(([page, groupRects]) => (
                  <div key={page}>
                    {rectsByPage.size > 1 && (
                      <p
                        className="text-xs font-semibold uppercase tracking-wider mb-[var(--space-xs)] px-[var(--space-xs)]"
                        style={{ color: 'var(--text-tertiary)' }}
                      >
                        Page {page}
                      </p>
                    )}
                    <div className="space-y-[var(--space-xs)]">
                      {groupRects.map((rect, i) => (
                        <div
                          key={rect.id}
                          className="p-[var(--space-sm)] rounded-[var(--radius-md)] cursor-pointer transition-colors"
                          style={{
                            backgroundColor: rect.id === selectedId ? 'var(--amber-subtle)' : 'transparent',
                            border: `1px solid ${rect.id === selectedId ? 'var(--amber-accent)' : 'var(--border-subtle)'}`,
                          }}
                          onClick={() => {
                            setSelectedId(rect.id);
                            if (rect.page !== pdf.currentPage) pdf.setPage(rect.page);
                          }}
                        >
                          <div className="flex items-center justify-between mb-[var(--space-xs)]">
                            <span className="text-xs font-semibold" style={{ color: 'var(--text-primary)' }}>
                              {rectsByPage.size > 1 ? `${page}.` : ''}{i + 1}
                            </span>
                            <button
                              type="button"
                              onClick={(e) => { e.stopPropagation(); deleteRedaction(rect.id); }}
                              className="text-xs font-medium"
                              style={{ color: 'var(--status-hold)', background: 'none', border: 'none', cursor: 'pointer' }}
                            >
                              Remove
                            </button>
                          </div>
                          <input
                            type="text"
                            value={rect.reason}
                            onChange={(e) => updateRedaction(rect.id, { reason: e.target.value })}
                            placeholder="Reason\u2026"
                            className="input-field text-xs"
                            style={{ padding: 'var(--space-xs) var(--space-sm)' }}
                            required
                            onClick={(e) => e.stopPropagation()}
                          />
                        </div>
                      ))}
                    </div>
                  </div>
                ))}
            </div>
          )}
        </div>

        {/* Bottom actions */}
        <div className="p-[var(--space-sm)] space-y-[var(--space-xs)] border-t" style={{ borderColor: 'var(--border-default)' }}>
          {rects.length > 0 && !allReasonsProvided && (
            <p className="text-xs text-center" style={{ color: 'var(--amber-accent)' }}>
              All areas need a reason
            </p>
          )}
          <div className="flex gap-[var(--space-xs)]">
            <button
              type="button"
              onClick={() => setPreviewMode(!previewMode)}
              disabled={rects.length === 0}
              className="btn-secondary flex-1 text-xs"
            >
              {previewMode ? 'Edit' : 'Preview'}
            </button>
            <button
              type="button"
              onClick={() => setShowConfirm(true)}
              disabled={rects.length === 0 || !allReasonsProvided}
              className="btn-primary flex-1 text-xs"
            >
              Finalize ({rects.length})
            </button>
          </div>
          <button
            type="button"
            onClick={() => setShowDiscardConfirm(true)}
            disabled={rects.length === 0}
            className="btn-ghost w-full text-xs"
            style={{ color: 'var(--status-hold)' }}
          >
            Discard draft
          </button>
        </div>
      </div>

      {/* ── Main area ── */}
      <div className="flex-1 flex flex-col overflow-hidden">
        {/* Toolbar */}
        <div
          className="flex items-center justify-between px-[var(--space-md)] py-[var(--space-xs)] border-b"
          style={{ borderColor: 'var(--border-default)', backgroundColor: 'var(--bg-elevated)' }}
        >
          <PageNavigator
            currentPage={pdf.currentPage}
            totalPages={pdf.totalPages}
            onPageChange={pdf.setPage}
          />
          <ZoomControls
            zoom={pdf.zoom}
            onZoomIn={pdf.zoomIn}
            onZoomOut={pdf.zoomOut}
            onFitToWidth={pdf.fitToWidth}
          />
        </div>

        {/* Canvas */}
        <div
          ref={containerRef}
          className="flex-1 overflow-auto flex items-start justify-center p-[var(--space-lg)]"
          style={{ backgroundColor: 'var(--bg-inset)' }}
        >
          {imageLoaded && imageElement ? (
            <canvas
              ref={canvasRef}
              style={{
                cursor: previewMode ? 'default' : canvasCursor,
                transform: `scale(${pdf.zoom})`,
                transformOrigin: 'top center',
                maxWidth: '100%',
                boxShadow: 'var(--shadow-md)',
              }}
              onMouseDown={handleMouseDown}
              onMouseMove={handleMouseMove}
              onMouseUp={handleMouseUp}
              onMouseLeave={handleMouseUp}
            />
          ) : (
            <div className="flex items-center justify-center h-full">
              <p className="text-sm" style={{ color: 'var(--text-tertiary)' }}>Loading page\u2026</p>
            </div>
          )}
        </div>
      </div>

      {/* ── Confirmation overlay ── */}
      {/* ── Finalize confirmation overlay ── */}
      {showConfirm && (
        <div
          className="fixed inset-0 z-[110] flex items-center justify-center"
          style={{ backgroundColor: 'oklch(0.2 0.01 50 / 0.6)' }}
        >
          <div className="card max-w-md w-full mx-[var(--space-lg)] p-[var(--space-lg)] space-y-[var(--space-md)]">
            <h4
              className="font-[family-name:var(--font-heading)] text-lg"
              style={{ color: 'var(--text-primary)' }}
            >
              Finalize Redaction
            </h4>

            <div className="card-inset p-[var(--space-md)] space-y-[var(--space-xs)]">
              <div className="flex items-center gap-[var(--space-sm)]">
                <span className="field-label" style={{ marginBottom: 0 }}>Name:</span>
                <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>{draftName}</span>
              </div>
              <div className="flex items-center gap-[var(--space-sm)]">
                <span className="field-label" style={{ marginBottom: 0 }}>Purpose:</span>
                <span className="text-sm" style={{ color: 'var(--text-secondary)' }}>
                  {draftPurpose.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())}
                </span>
              </div>
              <div className="flex items-center gap-[var(--space-sm)]">
                <span className="field-label" style={{ marginBottom: 0 }}>Areas:</span>
                <span className="text-sm" style={{ color: 'var(--text-secondary)' }}>
                  {rects.length} across {rectsByPage.size} page{rectsByPage.size !== 1 ? 's' : ''}
                </span>
              </div>
            </div>

            <div
              className="card-inset p-[var(--space-md)]"
              style={{ borderLeft: '2px solid var(--status-hold)' }}
            >
              <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
                This will create a permanently redacted copy.
                Text layers in redacted areas will be destroyed.
                The original evidence is preserved.
              </p>
            </div>

            <div>
              <label className="field-label" htmlFor="finalize-desc">
                Description (optional)
              </label>
              <input
                id="finalize-desc"
                type="text"
                value={finalizeDescription}
                onChange={(e) => setFinalizeDescription(e.target.value)}
                placeholder="e.g. Prepared for Q1 defence disclosure"
                className="input-field w-full"
              />
            </div>

            {applyError && <div className="banner-error">{applyError}</div>}
            <div className="flex gap-[var(--space-sm)] justify-end">
              <button type="button" onClick={() => { setShowConfirm(false); setApplyError(null); }} className="btn-ghost">
                Cancel
              </button>
              <button type="button" onClick={handleApply} disabled={applying} className="btn-danger">
                {applying ? 'Finalizing\u2026' : 'Finalize'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ── Discard confirmation overlay ── */}
      {showDiscardConfirm && (
        <div
          className="fixed inset-0 z-[110] flex items-center justify-center"
          style={{ backgroundColor: 'oklch(0.2 0.01 50 / 0.6)' }}
        >
          <div className="card max-w-md w-full mx-[var(--space-lg)] p-[var(--space-lg)] space-y-[var(--space-md)]">
            <h4
              className="font-[family-name:var(--font-heading)] text-lg"
              style={{ color: 'var(--text-primary)' }}
            >
              Discard draft
            </h4>
            <div
              className="card-inset p-[var(--space-md)]"
              style={{ borderLeft: '2px solid var(--status-hold)' }}
            >
              <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
                This will permanently delete your redaction draft with{' '}
                <strong>{rects.length}</strong> area{rects.length !== 1 ? 's' : ''}.
                This action cannot be undone.
              </p>
            </div>
            <div className="flex gap-[var(--space-sm)] justify-end">
              <button type="button" onClick={() => setShowDiscardConfirm(false)} className="btn-ghost">
                Cancel
              </button>
              <button type="button" onClick={handleDiscard} className="btn-danger">
                Discard
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
