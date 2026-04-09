'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { useRouter, useParams } from 'next/navigation';
import type { EvidenceItem, CustodyEntry } from '@/types';
import { RedactionEditor } from '@/components/redaction/redaction-editor';
import { CollaborativeEditor } from '@/components/redaction/collaborative-editor';
import { DraftPicker } from '@/components/redaction/draft-picker';
import { RedactedVersions } from '@/components/evidence/redacted-versions';
import type { RedactionDraft } from '@/types';
import {
  formatFileSize,
  mimeIcon,
  CLASSIFICATION_STYLES,
} from '@/lib/evidence-utils';

function tsaStatusLabel(
  item: EvidenceItem
): { label: string; color: string; bg: string } {
  if (item.tsa_timestamp) {
    return {
      label: 'Verified',
      color: 'var(--status-active)',
      bg: 'var(--status-active-bg)',
    };
  }
  if (item.tsa_token) {
    return {
      label: 'Pending',
      color: 'var(--status-closed)',
      bg: 'var(--status-closed-bg)',
    };
  }
  return {
    label: 'Unavailable',
    color: 'var(--status-archived)',
    bg: 'var(--status-archived-bg)',
  };
}

export function EvidenceDetail({
  evidence,
  canEdit,
  accessToken,
  username,
}: {
  evidence: EvidenceItem;
  canEdit: boolean;
  accessToken?: string;
  username?: string;
}) {
  const router = useRouter();
  const params = useParams();
  const locale = (params?.locale as string) || 'en';
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [showRedactor, setShowRedactor] = useState(false);
  const [showDraftPicker, setShowDraftPicker] = useState(false);
  const [selectedDraft, setSelectedDraft] = useState<RedactionDraft | null>(null);
  const [totalPages, setTotalPages] = useState<number | null>(null);

  const clsStyle =
    CLASSIFICATION_STYLES[evidence.classification] ||
    CLASSIFICATION_STYLES.restricted;
  const tsaStatus = tsaStatusLabel(evidence);

  const exif = evidence.metadata?.exif as
    | Record<string, unknown>
    | undefined;

  const handleVersionUpload = useCallback(async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file || !accessToken) return;

    setUploading(true);
    setUploadError(null);

    const formData = new FormData();
    formData.append('file', file);
    formData.append('classification', evidence.classification);

    try {
      const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';
      const res = await fetch(`${API_BASE}/api/evidence/${evidence.id}/version`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${accessToken}` },
        body: formData,
      });

      if (!res.ok) {
        const data = await res.json().catch(() => null);
        setUploadError(data?.error || 'Upload failed');
      } else {
        const newEvidence = await res.json().catch(() => null);
        if (newEvidence?.data?.id) {
          router.push(`/${locale}/evidence/${newEvidence.data.id}`);
        } else {
          router.refresh();
        }
      }
    } catch {
      setUploadError('An unexpected error occurred.');
    } finally {
      setUploading(false);
      if (fileInputRef.current) fileInputRef.current.value = '';
    }
  }, [accessToken, evidence.id, evidence.classification, router]);

  return (
    <div
      className="space-y-[var(--space-lg)]"
      style={{ animation: 'fade-in var(--duration-slow) var(--ease-out-expo)' }}
    >
      {/* Header */}
      <div>
        <div className="flex items-center gap-[var(--space-sm)] mb-[var(--space-xs)]">
          <span
            className="font-[family-name:var(--font-mono)] text-xs tracking-wide"
            style={{ color: 'var(--text-tertiary)' }}
          >
            {evidence.evidence_number}
          </span>
          <span
            className="badge"
            style={{ backgroundColor: clsStyle.bg, color: clsStyle.color }}
          >
            {evidence.classification.replace('_', ' ')}
          </span>
          {!evidence.is_current && (
            <span
              className="badge"
              style={{
                backgroundColor: 'var(--status-archived-bg)',
                color: 'var(--status-archived)',
              }}
            >
              Superseded
            </span>
          )}
          {evidence.destroyed_at && (
            <span
              className="badge"
              style={{
                backgroundColor: 'var(--status-hold-bg)',
                color: 'var(--status-hold)',
              }}
            >
              Destroyed
            </span>
          )}
        </div>
        <div className="flex items-start justify-between gap-[var(--space-md)]">
          <h1
            className="font-[family-name:var(--font-heading)] text-2xl leading-tight text-balance"
            style={{ color: 'var(--text-primary)' }}
          >
            {evidence.title || evidence.filename}
          </h1>
          {canEdit && evidence.is_current && !evidence.destroyed_at && accessToken && (
            <div className="shrink-0 flex gap-[var(--space-sm)]">
              {(evidence.mime_type.startsWith('image/') || evidence.mime_type === 'application/pdf') && (
                <button
                  type="button"
                  onClick={async () => {
                    if (evidence.mime_type === 'application/pdf') {
                      // PDF: show draft picker for named multi-draft workflow
                      setShowDraftPicker(true);
                    } else {
                      // Image: direct redaction (legacy single-shot)
                      setShowRedactor(true);
                    }
                  }}
                  className="btn-secondary text-xs"
                >
                  Redact
                </button>
              )}
              <input
                ref={fileInputRef}
                type="file"
                onChange={handleVersionUpload}
                className="hidden"
              />
              <button
                type="button"
                onClick={() => fileInputRef.current?.click()}
                disabled={uploading}
                className="btn-secondary text-xs"
              >
                {uploading ? 'Uploading\u2026' : 'Upload new version'}
              </button>
            </div>
          )}
        </div>
        {uploadError && (
          <div className="banner-error mt-[var(--space-sm)]">{uploadError}</div>
        )}
      </div>

      {/* Draft picker dialog */}
      {showDraftPicker && accessToken && (
        <DraftPicker
          evidenceId={evidence.id}
          accessToken={accessToken}
          onSelect={async (draft) => {
            setShowDraftPicker(false);
            setSelectedDraft(draft);
            // Fetch page count for PDF
            try {
              const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';
              const res = await fetch(`${API_BASE}/api/evidence/${evidence.id}/page-count`, {
                headers: { Authorization: `Bearer ${accessToken}` },
              });
              if (res.ok) {
                const json = await res.json();
                setTotalPages(json.data?.page_count ?? 1);
              } else {
                setTotalPages(1);
              }
            } catch {
              setTotalPages(1);
            }
            setShowRedactor(true);
          }}
          onClose={() => setShowDraftPicker(false)}
        />
      )}

      {showRedactor && accessToken && evidence.mime_type === 'application/pdf' && totalPages !== null && selectedDraft && (
        <CollaborativeEditor
          evidenceId={evidence.id}
          draftId={selectedDraft.id}
          draftName={selectedDraft.name}
          draftPurpose={selectedDraft.purpose}
          totalPages={totalPages}
          accessToken={accessToken}
          username={username || 'User'}
          onClose={() => {
            setShowRedactor(false);
            setSelectedDraft(null);
          }}
          onApplied={(newEvidenceId) => {
            setShowRedactor(false);
            setSelectedDraft(null);
            router.push(`/${locale}/evidence/${newEvidenceId}`);
          }}
        />
      )}

      {showRedactor && accessToken && evidence.mime_type !== 'application/pdf' && (
        <RedactionEditor
          evidenceId={evidence.id}
          imageUrl={`/api/evidence/${evidence.id}/download`}
          mimeType={evidence.mime_type}
          accessToken={accessToken}
          onApply={async (redactions) => {
            const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';
            const res = await fetch(`${API_BASE}/api/evidence/${evidence.id}/redact`, {
              method: 'POST',
              headers: {
                'Content-Type': 'application/json',
                Authorization: `Bearer ${accessToken}`,
              },
              body: JSON.stringify({
                redactions: redactions.map((r) => ({
                  page_number: r.pageNumber,
                  x: r.x,
                  y: r.y,
                  width: r.width,
                  height: r.height,
                  reason: r.reason,
                })),
              }),
            });
            if (!res.ok) {
              const body = await res.json().catch(() => null);
              throw new Error(body?.error || 'Redaction failed');
            }
            setShowRedactor(false);
            router.refresh();
          }}
          onClose={() => setShowRedactor(false)}
        />
      )}

      {/* Metadata card */}
      <div className="card-inset grid grid-cols-2 sm:grid-cols-4 gap-[var(--space-lg)] p-[var(--space-md)]">
        <MetaField label="File name" value={evidence.filename} />
        <MetaField label="Size" value={formatFileSize(evidence.size_bytes)} />
        <MetaField label="MIME type" value={evidence.mime_type} mono />
        <MetaField
          label="Uploaded"
          value={new Date(evidence.created_at).toLocaleDateString('en-GB', {
            day: '2-digit',
            month: 'short',
            year: 'numeric',
          })}
        />
        <MetaField
          label="Uploaded by"
          value={evidence.uploaded_by_name || evidence.uploaded_by.slice(0, 8) + '\u2026'}
        />
        <MetaField label="Source" value={evidence.source || '\u2014'} />
        <MetaField
          label="Source date"
          value={
            evidence.source_date
              ? new Date(evidence.source_date).toLocaleDateString('en-GB', {
                  day: '2-digit',
                  month: 'short',
                  year: 'numeric',
                })
              : '\u2014'
          }
        />
        <MetaField
          label="Version"
          value={`v${evidence.version}`}
          mono
        />
      </div>

      {/* Tags */}
      {evidence.tags.length > 0 && (
        <div className="flex flex-wrap gap-[var(--space-xs)]">
          {evidence.tags.map((tag) => (
            <span
              key={tag}
              className="badge"
              style={{
                backgroundColor: 'var(--bg-inset)',
                color: 'var(--text-secondary)',
                border: '1px solid var(--border-subtle)',
              }}
            >
              {tag}
            </span>
          ))}
        </div>
      )}

      {/* Description */}
      {evidence.description && (
        <div className="card p-[var(--space-lg)]">
          <h2 className="field-label mb-[var(--space-sm)]">Description</h2>
          <p
            className="text-base leading-relaxed whitespace-pre-wrap max-w-2xl"
            style={{ color: 'var(--text-secondary)' }}
          >
            {evidence.description}
          </p>
        </div>
      )}

      {/* Hash verification */}
      <div className="card p-[var(--space-lg)]">
        <h2 className="field-label mb-[var(--space-sm)]">
          Hash verification
        </h2>
        <div className="space-y-[var(--space-sm)]">
          <div>
            <span
              className="field-label"
              style={{ marginBottom: 'var(--space-xs)', display: 'block' }}
            >
              SHA-256
            </span>
            <code
              className="font-[family-name:var(--font-mono)] text-xs break-all"
              style={{ color: 'var(--text-primary)' }}
            >
              {evidence.sha256_hash}
            </code>
          </div>
          <div className="flex items-center gap-[var(--space-sm)]">
            <span className="field-label" style={{ marginBottom: 0 }}>
              Timestamp authority
            </span>
            <span
              className="badge"
              style={{ backgroundColor: tsaStatus.bg, color: tsaStatus.color }}
            >
              {tsaStatus.label}
            </span>
          </div>
          {evidence.tsa_timestamp && (
            <div>
              <span className="field-label">TSA timestamp</span>
              <span
                className="font-[family-name:var(--font-mono)] text-xs"
                style={{ color: 'var(--text-primary)' }}
              >
                {new Date(evidence.tsa_timestamp).toLocaleString('en-GB')}
              </span>
            </div>
          )}
          {evidence.tsa_name && (
            <div>
              <span className="field-label">TSA name</span>
              <span
                className="text-sm"
                style={{ color: 'var(--text-secondary)' }}
              >
                {evidence.tsa_name}
              </span>
            </div>
          )}
        </div>
      </div>

      {/* File preview */}
      <div className="card p-[var(--space-lg)]">
        <h2 className="field-label mb-[var(--space-sm)]">Preview</h2>
        <FilePreview evidence={evidence} />
      </div>

      {/* EXIF data */}
      {exif && Object.keys(exif).length > 0 && (
        <div className="card p-[var(--space-lg)]">
          <h2 className="field-label mb-[var(--space-sm)]">
            EXIF metadata
          </h2>
          <div className="card-inset grid grid-cols-2 sm:grid-cols-3 gap-[var(--space-md)] p-[var(--space-md)]">
            {exif.gps_latitude != null && exif.gps_longitude != null && (
              <MetaField
                label="GPS coordinates"
                value={`${exif.gps_latitude}, ${exif.gps_longitude}`}
                mono
              />
            )}
            {exif.camera_make != null && (
              <MetaField
                label="Camera"
                value={`${String(exif.camera_make)}${exif.camera_model != null ? ` ${String(exif.camera_model)}` : ''}`}
              />
            )}
            {exif.capture_date != null && (
              <MetaField
                label="Capture date"
                value={String(exif.capture_date)}
              />
            )}
            {exif.focal_length != null && (
              <MetaField label="Focal length" value={String(exif.focal_length)} />
            )}
            {exif.exposure_time != null && (
              <MetaField label="Exposure" value={String(exif.exposure_time)} />
            )}
            {exif.iso != null && (
              <MetaField label="ISO" value={String(exif.iso)} mono />
            )}
          </div>
        </div>
      )}

      {/* Actions */}
      <div className="flex flex-wrap gap-[var(--space-sm)]">
        <a
          href={`/api/evidence/${evidence.id}/download`}
          className="btn-primary inline-flex items-center gap-[var(--space-xs)]"
          download
        >
          <svg
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            aria-hidden="true"
          >
            <path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4" />
            <polyline points="7,10 12,15 17,10" />
            <line x1="12" y1="15" x2="12" y2="3" />
          </svg>
          Download file
        </a>
        {canEdit && (
          <a
            href={`/${locale}/evidence/${evidence.id}`}
            className="btn-secondary"
          >
            Edit metadata
          </a>
        )}
      </div>
      <p
        className="text-xs"
        style={{ color: 'var(--text-tertiary)' }}
      >
        This download will be logged in the chain of custody.
      </p>

      {/* Redacted Versions panel — only for PDFs with edit access */}
      {canEdit && accessToken && evidence.mime_type === 'application/pdf' && !evidence.destroyed_at && (
        <RedactedVersions
          evidenceId={evidence.id}
          accessToken={accessToken}
          onResumeDraft={async (draft) => {
            setSelectedDraft(draft);
            try {
              const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';
              const res = await fetch(`${API_BASE}/api/evidence/${evidence.id}/page-count`, {
                headers: { Authorization: `Bearer ${accessToken}` },
              });
              if (res.ok) {
                const json = await res.json();
                setTotalPages(json.data?.page_count ?? 1);
              } else {
                setTotalPages(1);
              }
            } catch {
              setTotalPages(1);
            }
            setShowRedactor(true);
          }}
          onNewDraft={() => setShowDraftPicker(true)}
        />
      )}

      {/* Custody Log & Version History tabs */}
      <EvidenceHistoryTabs evidenceId={evidence.id} />
    </div>
  );
}

function FilePreview({ evidence }: { evidence: EvidenceItem }) {
  const downloadUrl = `/api/evidence/${evidence.id}/download`;

  if (evidence.mime_type.startsWith('image/')) {
    return (
      <div
        className="rounded overflow-hidden"
        style={{
          maxHeight: '500px',
          backgroundColor: 'var(--bg-inset)',
        }}
      >
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img
          src={downloadUrl}
          alt={evidence.title || evidence.filename}
          className="mx-auto"
          style={{ maxHeight: '500px', objectFit: 'contain' }}
        />
      </div>
    );
  }

  if (evidence.mime_type.startsWith('audio/')) {
    return (
      <audio controls className="w-full" preload="metadata">
        <source src={downloadUrl} type={evidence.mime_type} />
        Your browser does not support the audio element.
      </audio>
    );
  }

  if (evidence.mime_type.startsWith('video/')) {
    return (
      <div
        className="rounded overflow-hidden"
        style={{ backgroundColor: 'var(--bg-inset)' }}
      >
        <video
          controls
          className="w-full"
          preload="metadata"
          style={{ maxHeight: '500px' }}
        >
          <source src={downloadUrl} type={evidence.mime_type} />
          Your browser does not support the video element.
        </video>
      </div>
    );
  }

  // Fallback for non-previewable files
  return (
    <div
      className="flex flex-col items-center justify-center py-[var(--space-xl)]"
      style={{ color: 'var(--text-tertiary)' }}
    >
      <span className="text-3xl mb-[var(--space-sm)]" aria-hidden="true">
        {mimeIcon(evidence.mime_type)}
      </span>
      <p className="text-sm mb-[var(--space-sm)]">
        Preview not available for {evidence.mime_type}
      </p>
      <a href={downloadUrl} className="btn-secondary" download>
        Download to view
      </a>
    </div>
  );
}

type HistoryTab = 'custody' | 'versions';

interface VersionEntry {
  readonly id: string;
  readonly version: number;
  readonly filename: string;
  readonly size_bytes: number;
  readonly created_at: string;
  readonly uploaded_by: string;
  readonly is_current?: boolean;
  readonly sha256_hash?: string;
}

function EvidenceHistoryTabs({ evidenceId }: { evidenceId: string }) {
  const [activeTab, setActiveTab] = useState<HistoryTab>('custody');
  const [custodyEntries, setCustodyEntries] = useState<
    readonly CustodyEntry[]
  >([]);
  const [versions, setVersions] = useState<readonly VersionEntry[]>([]);
  const [custodyLoaded, setCustodyLoaded] = useState(false);
  const [versionsLoaded, setVersionsLoaded] = useState(false);
  const [custodyError, setCustodyError] = useState<string | null>(null);
  const [versionsError, setVersionsError] = useState<string | null>(null);

  const loadCustody = useCallback(async () => {
    if (custodyLoaded) return;
    try {
      const res = await fetch(`/api/evidence/${evidenceId}/custody`);
      if (res.ok) {
        const json = await res.json();
        setCustodyEntries(json.data || []);
      } else {
        setCustodyError('Failed to load custody log');
      }
    } catch {
      setCustodyError('Failed to load custody log');
    }
    setCustodyLoaded(true);
  }, [evidenceId, custodyLoaded]);

  const loadVersions = useCallback(async () => {
    if (versionsLoaded) return;
    try {
      const res = await fetch(`/api/evidence/${evidenceId}/versions`);
      if (res.ok) {
        const json = await res.json();
        setVersions(json.data || []);
      } else {
        setVersionsError('Failed to load version history');
      }
    } catch {
      setVersionsError('Failed to load version history');
    }
    setVersionsLoaded(true);
  }, [evidenceId, versionsLoaded]);

  const handleTabChange = useCallback((tab: HistoryTab) => {
    setActiveTab(tab);
    if (tab === 'custody') {
      loadCustody();
    } else {
      loadVersions();
    }
  }, [loadCustody, loadVersions]);

  // Load custody on initial mount
  useEffect(() => {
    loadCustody();
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div className="card overflow-hidden">
      {/* Tab buttons */}
      <div
        className="flex"
        style={{ borderBottom: '1px solid var(--border-default)' }}
      >
        <button
          type="button"
          onClick={() => handleTabChange('custody')}
          className="px-[var(--space-md)] py-[var(--space-sm)] text-sm font-medium"
          style={{
            color:
              activeTab === 'custody'
                ? 'var(--text-primary)'
                : 'var(--text-tertiary)',
            borderBottom:
              activeTab === 'custody'
                ? '2px solid var(--amber-accent)'
                : '2px solid transparent',
            backgroundColor: 'transparent',
          }}
        >
          Custody Log
        </button>
        <button
          type="button"
          onClick={() => handleTabChange('versions')}
          className="px-[var(--space-md)] py-[var(--space-sm)] text-sm font-medium"
          style={{
            color:
              activeTab === 'versions'
                ? 'var(--text-primary)'
                : 'var(--text-tertiary)',
            borderBottom:
              activeTab === 'versions'
                ? '2px solid var(--amber-accent)'
                : '2px solid transparent',
            backgroundColor: 'transparent',
          }}
        >
          Version History
        </button>
      </div>

      {/* Tab content */}
      <div className="p-[var(--space-lg)]">
        {activeTab === 'custody' && (
          <CustodyLogPanel
            entries={custodyEntries}
            loaded={custodyLoaded}
            error={custodyError}
          />
        )}
        {activeTab === 'versions' && (
          <VersionHistoryPanel
            versions={versions}
            loaded={versionsLoaded}
            error={versionsError}
          />
        )}
      </div>
    </div>
  );
}

function CustodyLogPanel({
  entries,
  loaded,
  error,
}: {
  entries: readonly CustodyEntry[];
  loaded: boolean;
  error: string | null;
}) {
  if (!loaded) {
    return (
      <p className="text-sm" style={{ color: 'var(--text-tertiary)' }}>
        Loading...
      </p>
    );
  }

  if (error) {
    return (
      <p className="text-sm" style={{ color: 'var(--status-hold)' }}>
        {error}
      </p>
    );
  }

  if (entries.length === 0) {
    return (
      <p className="text-sm" style={{ color: 'var(--text-tertiary)' }}>
        No custody entries
      </p>
    );
  }

  return (
    <div className="space-y-[var(--space-md)]">
      {entries.map((entry) => (
        <div
          key={entry.id}
          className="flex gap-[var(--space-sm)]"
          style={{
            paddingLeft: 'var(--space-md)',
            borderLeft: '2px solid var(--border-subtle)',
          }}
        >
          <div className="flex-1">
            <p className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
              {entry.action}
            </p>
            {entry.detail && (
              <p className="text-xs mt-[var(--space-xs)]" style={{ color: 'var(--text-secondary)' }}>
                {entry.detail}
              </p>
            )}
            <p
              className="text-xs mt-[var(--space-xs)] font-[family-name:var(--font-mono)]"
              style={{ color: 'var(--text-tertiary)' }}
            >
              {new Date(entry.timestamp).toLocaleString('en-GB')}
              {' \u2014 '}
              {entry.actor_user_id.slice(0, 8)}...
            </p>
          </div>
        </div>
      ))}
    </div>
  );
}

function VersionHistoryPanel({
  versions,
  loaded,
  error,
}: {
  versions: readonly VersionEntry[];
  loaded: boolean;
  error: string | null;
}) {
  const router = useRouter();
  const params = useParams();
  const locale = (params?.locale as string) || 'en';
  if (!loaded) {
    return (
      <p className="text-sm" style={{ color: 'var(--text-tertiary)' }}>
        Loading...
      </p>
    );
  }

  if (error) {
    return (
      <p className="text-sm" style={{ color: 'var(--status-hold)' }}>
        {error}
      </p>
    );
  }

  if (versions.length === 0) {
    return (
      <p className="text-sm" style={{ color: 'var(--text-tertiary)' }}>
        No previous versions
      </p>
    );
  }

  return (
    <table className="w-full text-sm" style={{ borderCollapse: 'collapse' }}>
      <thead>
        <tr style={{ borderBottom: '1px solid var(--border-subtle)' }}>
          <th className="text-left py-[var(--space-xs)] pr-[var(--space-md)] text-xs uppercase tracking-wider font-semibold" style={{ color: 'var(--text-tertiary)', width: '3.5rem' }}>Ver</th>
          <th className="text-left py-[var(--space-xs)] pr-[var(--space-md)] text-xs uppercase tracking-wider font-semibold" style={{ color: 'var(--text-tertiary)' }}>File</th>
          <th className="text-left py-[var(--space-xs)] pr-[var(--space-md)] text-xs uppercase tracking-wider font-semibold" style={{ color: 'var(--text-tertiary)', width: '9rem' }}>Hash</th>
          <th className="text-right py-[var(--space-xs)] text-xs uppercase tracking-wider font-semibold" style={{ color: 'var(--text-tertiary)', width: '7rem' }}>Date</th>
        </tr>
      </thead>
      <tbody>
        {versions.map((v) => (
          <tr
            key={v.id}
            className="table-row"
            onClick={() => router.push(`/${locale}/evidence/${v.id}`)}
            style={{ borderBottom: '1px solid var(--border-subtle)' }}
          >
            <td className="py-[var(--space-sm)] pr-[var(--space-md)]">
              <span
                className="font-[family-name:var(--font-mono)] font-semibold"
                style={{ color: 'var(--text-primary)' }}
              >
                v{v.version}
              </span>
            </td>
            <td className="py-[var(--space-sm)] pr-[var(--space-md)]">
              <div className="flex items-center gap-[var(--space-sm)]">
                {v.is_current && (
                  <span
                    className="badge shrink-0"
                    style={{
                      backgroundColor: 'var(--status-active-bg)',
                      color: 'var(--status-active)',
                    }}
                  >
                    Current
                  </span>
                )}
                <span className="truncate" style={{ color: 'var(--text-secondary)' }}>
                  {v.filename}
                </span>
              </div>
            </td>
            <td className="py-[var(--space-sm)] pr-[var(--space-md)]">
              <span
                className="font-[family-name:var(--font-mono)] text-xs"
                style={{ color: 'var(--text-tertiary)' }}
              >
                {v.sha256_hash?.slice(0, 12)}{'\u2026'}
              </span>
            </td>
            <td className="py-[var(--space-sm)] text-right">
              <span
                className="text-xs tabular-nums"
                style={{ color: 'var(--text-tertiary)' }}
              >
                {new Date(v.created_at).toLocaleDateString('en-GB', {
                  day: '2-digit',
                  month: 'short',
                  year: 'numeric',
                })}
              </span>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function MetaField({
  label,
  value,
  mono,
}: {
  label: string;
  value: string;
  mono?: boolean;
}) {
  return (
    <div>
      <dt className="field-label">{label}</dt>
      <dd
        className={`mt-[var(--space-xs)] text-sm break-all ${mono ? 'font-[family-name:var(--font-mono)]' : ''}`}
        style={{ color: 'var(--text-primary)' }}
      >
        {value}
      </dd>
    </div>
  );
}
