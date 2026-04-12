'use client';

import { useCallback, useState } from 'react';
import { useRouter } from 'next/navigation';
import { EvidenceUploader } from './evidence-uploader';
import { EvidenceGrid } from './evidence-grid';
import type { EvidenceItem } from '@/types';

export function EvidencePageClient({
  caseId,
  accessToken,
  canUpload,
  evidence,
  nextCursor,
  hasMore,
  currentQuery,
  currentClassification,
}: {
  caseId: string;
  accessToken: string;
  canUpload: boolean;
  evidence: EvidenceItem[];
  nextCursor: string;
  hasMore: boolean;
  currentQuery: string;
  currentClassification: string;
}) {
  const [showUploader, setShowUploader] = useState(false);
  const router = useRouter();

  const handleUploadComplete = useCallback(() => {
    router.refresh();
  }, [router]);

  return (
    <div className="space-y-[var(--space-md)]">
      {/* Upload controls — day-to-day single-file upload only.
          Bulk-import from another system lives under Settings →
          Data import (it's a one-off case-setup action, not daily
          workflow). */}
      {canUpload && (
        <>
          <button
            type="button"
            onClick={() => setShowUploader((prev) => !prev)}
            className={showUploader ? 'btn-secondary' : 'btn-primary'}
          >
            {showUploader ? 'Hide uploader' : 'Upload evidence'}
          </button>

          {showUploader && (
            <EvidenceUploader
              caseId={caseId}
              accessToken={accessToken}
              onUploadComplete={handleUploadComplete}
            />
          )}
        </>
      )}

      {/* Evidence grid */}
      <EvidenceGrid
        caseId={caseId}
        evidence={evidence}
        nextCursor={nextCursor}
        hasMore={hasMore}
        currentQuery={currentQuery}
        currentClassification={currentClassification}
      />
    </div>
  );
}
