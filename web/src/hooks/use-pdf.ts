'use client';

import { useCallback, useState } from 'react';

interface UsePdfOptions {
  evidenceId: string;
  totalPages: number;
  dpi?: number;
}

interface UsePdfReturn {
  currentPage: number;
  totalPages: number;
  setPage: (page: number) => void;
  nextPage: () => void;
  prevPage: () => void;
  zoom: number;
  setZoom: (zoom: number) => void;
  zoomIn: () => void;
  zoomOut: () => void;
  fitToWidth: () => void;
  pageImageUrl: string;
}

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';
const ZOOM_STEPS = [0.5, 0.75, 1, 1.25, 1.5, 1.75, 2];

export function usePdf({ evidenceId, totalPages, dpi = 150 }: UsePdfOptions): UsePdfReturn {
  const [currentPage, setCurrentPage] = useState(1);
  const [zoom, setZoom] = useState(1);

  const clampedPage = Math.max(1, Math.min(currentPage, totalPages));

  const setPage = useCallback(
    (page: number) => setCurrentPage(Math.max(1, Math.min(page, totalPages))),
    [totalPages],
  );

  const nextPage = useCallback(() => setPage(clampedPage + 1), [clampedPage, setPage]);
  const prevPage = useCallback(() => setPage(clampedPage - 1), [clampedPage, setPage]);

  const zoomIn = useCallback(() => {
    const idx = ZOOM_STEPS.findIndex((z) => z >= zoom);
    if (idx < ZOOM_STEPS.length - 1) setZoom(ZOOM_STEPS[idx + 1]);
  }, [zoom]);

  const zoomOut = useCallback(() => {
    const idx = ZOOM_STEPS.findIndex((z) => z >= zoom);
    if (idx > 0) setZoom(ZOOM_STEPS[idx - 1]);
  }, [zoom]);

  const fitToWidth = useCallback(() => setZoom(1), []);

  const pageImageUrl = `${API_BASE}/api/evidence/${evidenceId}/pages/${clampedPage}?dpi=${dpi}`;

  return {
    currentPage: clampedPage,
    totalPages,
    setPage,
    nextPage,
    prevPage,
    zoom,
    setZoom,
    zoomIn,
    zoomOut,
    fitToWidth,
    pageImageUrl,
  };
}
