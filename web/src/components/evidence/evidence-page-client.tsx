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
    setShowUploader(false);
  }, [router]);

  return (
    <>
      {/* Page header */}
      <section className="d-pagehead">
        <div>
          <span className="eyebrow-m">Berkeley Protocol phases 2&ndash;4</span>
          <h1>Evidence <em>locker</em></h1>
          <p className="sub">
            Every upload is chunked, hashed client-side (SHA-256 + BLAKE3) and
            RFC 3161 timestamped at the gateway. Phase indicators show each
            exhibit&apos;s progress through the Berkeley Protocol&apos;s
            six-phase investigative cycle.
          </p>
        </div>
        {canUpload && (
          <div className="actions">
            <button
              type="button"
              className="btn ghost"
              onClick={() => setShowUploader((prev) => !prev)}
            >
              {showUploader ? 'Hide uploader' : 'Import archive'}
            </button>
            <button
              type="button"
              className="btn"
              onClick={() => setShowUploader(true)}
            >
              Upload exhibit <span className="arr">&rarr;</span>
            </button>
          </div>
        )}
      </section>

      {/* Uploader panel */}
      {showUploader && canUpload && (
        <EvidenceUploader
          caseId={caseId}
          accessToken={accessToken}
          onUploadComplete={handleUploadComplete}
        />
      )}

      {/* Evidence card grid with filter bar */}
      <EvidenceGrid
        caseId={caseId}
        evidence={evidence}
        nextCursor={nextCursor}
        hasMore={hasMore}
        currentQuery={currentQuery}
        currentClassification={currentClassification}
      />

      {/* Bottom panels */}
      <div className="g2-wide">
        <div className="panel">
          <div className="panel-h">
            <h3>Upload queue</h3>
            <span className="meta">&mdash;</span>
          </div>
          <div
            className="panel-body"
            style={{
              display: 'flex',
              flexDirection: 'column',
              gap: 14,
              padding: '24px 0',
              textAlign: 'center',
              color: 'var(--muted)',
              fontFamily: "'JetBrains Mono', monospace",
              fontSize: '12px',
            }}
          >
            No uploads in progress.
          </div>
        </div>

        <div className="panel">
          <div className="panel-h">
            <h3>Integrity summary</h3>
            <span className="meta">real-time</span>
          </div>
          <div className="panel-body">
            <dl className="kvs">
              <dt>Hash algorithm</dt>
              <dd>SHA-256 primary &middot; BLAKE3 secondary</dd>
              <dt>Timestamp authority</dt>
              <dd><code>ts-eu-west</code> &middot; RFC 3161</dd>
              <dt>Total items</dt>
              <dd>{evidence.length}</dd>
              <dt>Validator</dt>
              <dd>
                <a className="linkarrow" href="/validator">
                  Offline verify (0.3 MB binary) &rarr;
                </a>
              </dd>
            </dl>
          </div>
        </div>
      </div>
    </>
  );
}
