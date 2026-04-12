'use client';

import { useCallback, useRef, useState } from 'react';
import { useDropzone } from 'react-dropzone';
import { formatFileSize } from '@/lib/evidence-utils';
import { hashFileStreaming } from '@/lib/upload-hasher';

type UploadStatus =
  | 'hashing'
  | 'uploading'
  | 'complete'
  | 'error';

interface UploadFile {
  readonly id: string;
  readonly file: File;
  readonly progress: number;
  readonly status: UploadStatus;
  readonly evidenceId: string | null;
  readonly error: string | null;
  readonly clientHash: string | null;
  readonly serverHash: string | null;
  readonly hashProgress: number;
}

interface MetadataForm {
  readonly title: string;
  readonly classification: string;
  readonly description: string;
  readonly tags: string;
  readonly source: string;
  readonly sourceDate: string;
}

interface MismatchDiagnostic {
  readonly clientHash: string;
  readonly serverHash: string;
  readonly byteCount: number;
  readonly timestamp: string;
  readonly caseId: string;
  readonly filename: string;
}

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

function defaultMetadata(filename: string): MetadataForm {
  const dotIndex = filename.lastIndexOf('.');
  const title = dotIndex > 0 ? filename.slice(0, dotIndex) : filename;
  return {
    title,
    classification: 'restricted',
    description: '',
    tags: '',
    source: '',
    sourceDate: '',
  };
}

export function EvidenceUploader({
  caseId,
  accessToken,
  onUploadComplete,
}: {
  caseId: string;
  accessToken: string;
  onUploadComplete: () => void;
}) {
  const [uploads, setUploads] = useState<readonly UploadFile[]>([]);
  const [metadataForms, setMetadataForms] = useState<
    Record<string, MetadataForm>
  >({});
  const [savingMetadata, setSavingMetadata] = useState<Record<string, boolean>>(
    {}
  );
  const [metadataSaved, setMetadataSaved] = useState<Record<string, boolean>>(
    {}
  );
  const [metadataErrors, setMetadataErrors] = useState<
    Record<string, string | null>
  >({});
  const [receiptExpanded, setReceiptExpanded] = useState<
    Record<string, boolean>
  >({});
  const [copiedHash, setCopiedHash] = useState<Record<string, boolean>>({});
  const [mismatchModal, setMismatchModal] = useState<{
    uploadId: string;
    diagnostic: MismatchDiagnostic;
  } | null>(null);

  const abortControllers = useRef<Record<string, AbortController>>({});

  const updateUpload = useCallback(
    (id: string, patch: Partial<UploadFile>) => {
      setUploads((prev) =>
        prev.map((u) => (u.id === id ? { ...u, ...patch } : u))
      );
    },
    []
  );

  const hashAndUploadFile = useCallback(
    async (file: File, id: string) => {
      const controller = new AbortController();
      abortControllers.current[id] = controller;

      // Phase 1: Hash
      try {
        const clientHash = await hashFileStreaming(
          file,
          (bytesHashed, total) => {
            const pct = total > 0 ? Math.round((bytesHashed / total) * 100) : 0;
            updateUpload(id, { hashProgress: pct });
          },
          controller.signal
        );

        updateUpload(id, {
          status: 'uploading',
          clientHash,
          hashProgress: 100,
        });

        // Phase 2: Upload
        const formData = new FormData();
        formData.append('file', file);
        formData.append('client_sha256', clientHash);

        const xhr = new XMLHttpRequest();
        xhr.open('POST', `${API_BASE}/api/cases/${caseId}/evidence`);
        xhr.setRequestHeader('Authorization', `Bearer ${accessToken}`);
        xhr.setRequestHeader('X-Content-SHA256', clientHash);

        xhr.upload.onprogress = (e) => {
          if (e.lengthComputable) {
            const pct = Math.min(
              99,
              Math.round((e.loaded / e.total) * 100)
            );
            updateUpload(id, { progress: pct });
          }
        };

        const result = await new Promise<{
          evidenceId: string | null;
          serverHash: string | null;
          error: string | null;
          status: number;
        }>((resolve) => {
          xhr.onload = () => {
            if (xhr.status >= 200 && xhr.status < 300) {
              try {
                const data = JSON.parse(xhr.responseText);
                resolve({
                  evidenceId: data.data?.id || data.id || null,
                  serverHash: data.data?.sha256_hash || data.sha256_hash || null,
                  error: null,
                  status: xhr.status,
                });
              } catch {
                resolve({
                  evidenceId: null,
                  serverHash: null,
                  error: 'Invalid server response',
                  status: xhr.status,
                });
              }
            } else {
              try {
                const data = JSON.parse(xhr.responseText);
                const respData = data.data || data;
                resolve({
                  evidenceId: null,
                  serverHash: respData.actual_sha256 || null,
                  error: respData.error || data.error || `Upload failed (${xhr.status})`,
                  status: xhr.status,
                });
              } catch {
                resolve({
                  evidenceId: null,
                  serverHash: null,
                  error: `Upload failed (${xhr.status})`,
                  status: xhr.status,
                });
              }
            }
          };
          xhr.onerror = () =>
            resolve({
              evidenceId: null,
              serverHash: null,
              error: 'Network error',
              status: 0,
            });
          xhr.send(formData);
        });

        if (result.status === 409) {
          // Hash mismatch — server returns expected_sha256 and actual_sha256
          updateUpload(id, {
            status: 'error',
            error: 'Integrity check failed',
            serverHash: result.serverHash,
          });
          setMismatchModal({
            uploadId: id,
            diagnostic: {
              clientHash,
              serverHash: result.serverHash || 'unknown',
              byteCount: file.size,
              timestamp: new Date().toISOString(),
              caseId,
              filename: file.name,
            },
          });
        } else if (result.error) {
          updateUpload(id, {
            status: 'error',
            error: result.error,
          });
        } else {
          updateUpload(id, {
            status: 'complete',
            progress: 100,
            evidenceId: result.evidenceId,
            serverHash: result.serverHash,
          });
          onUploadComplete();
        }
      } catch (err) {
        if (err instanceof DOMException && err.name === 'AbortError') {
          updateUpload(id, {
            status: 'error',
            error: 'Hashing cancelled',
          });
        } else {
          updateUpload(id, {
            status: 'error',
            error: 'Upload failed',
          });
        }
      } finally {
        delete abortControllers.current[id];
      }
    },
    [caseId, accessToken, onUploadComplete, updateUpload]
  );

  const retryUpload = useCallback(
    (id: string) => {
      const upload = uploads.find((u) => u.id === id);
      if (!upload) return;
      setMismatchModal(null);
      updateUpload(id, {
        status: 'hashing',
        progress: 0,
        hashProgress: 0,
        clientHash: null,
        serverHash: null,
        error: null,
      });
      hashAndUploadFile(upload.file, id);
    },
    [uploads, updateUpload, hashAndUploadFile]
  );

  const cancelHashing = useCallback((id: string) => {
    abortControllers.current[id]?.abort();
  }, []);

  const downloadDiagnostic = useCallback((diagnostic: MismatchDiagnostic) => {
    const blob = new Blob([JSON.stringify(diagnostic, null, 2)], {
      type: 'application/json',
    });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `vaultkeeper-diagnostic-${Date.now()}.json`;
    a.click();
    URL.revokeObjectURL(url);
  }, []);

  const copyHash = useCallback(async (id: string, hash: string) => {
    await navigator.clipboard.writeText(hash);
    setCopiedHash((prev) => ({ ...prev, [id]: true }));
    setTimeout(() => {
      setCopiedHash((prev) => ({ ...prev, [id]: false }));
    }, 2000);
  }, []);

  const onDrop = useCallback(
    (acceptedFiles: File[]) => {
      for (const file of acceptedFiles) {
        const id = crypto.randomUUID();
        setUploads((prev) => [
          ...prev,
          {
            id,
            file,
            progress: 0,
            status: 'hashing',
            evidenceId: null,
            error: null,
            clientHash: null,
            serverHash: null,
            hashProgress: 0,
          },
        ]);
        setMetadataForms((prev) => ({
          ...prev,
          [id]: defaultMetadata(file.name),
        }));
        hashAndUploadFile(file, id);
      }
    },
    [hashAndUploadFile]
  );

  const { getRootProps, getInputProps, isDragActive } = useDropzone({ onDrop });

  const dismiss = (id: string) => {
    cancelHashing(id);
    setUploads((prev) => prev.filter((u) => u.id !== id));
    setMetadataForms((prev) => {
      // eslint-disable-next-line @typescript-eslint/no-unused-vars
      const { [id]: _removed, ...rest } = prev;
      return rest;
    });
    if (mismatchModal?.uploadId === id) {
      setMismatchModal(null);
    }
  };

  const updateMetadataField = (
    id: string,
    field: keyof MetadataForm,
    value: string
  ) => {
    setMetadataForms((prev) => ({
      ...prev,
      [id]: { ...prev[id], [field]: value },
    }));
  };

  const saveMetadata = async (uploadId: string, evidenceId: string) => {
    const form = metadataForms[uploadId];
    if (!form) return;

    setSavingMetadata((prev) => ({ ...prev, [uploadId]: true }));
    setMetadataErrors((prev) => ({ ...prev, [uploadId]: null }));

    const tags = form.tags
      .split(',')
      .map((t) => t.trim())
      .filter(Boolean);

    const body = {
      title: form.title,
      classification: form.classification,
      description: form.description,
      tags,
      source: form.source,
      source_date: form.sourceDate || null,
    };

    try {
      const res = await fetch(`${API_BASE}/api/evidence/${evidenceId}`, {
        method: 'PATCH',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${accessToken}`,
        },
        body: JSON.stringify(body),
      });

      if (!res.ok) {
        const data = await res.json().catch(() => null);
        setMetadataErrors((prev) => ({
          ...prev,
          [uploadId]: data?.error || `Save failed (${res.status})`,
        }));
      } else {
        setMetadataSaved((prev) => ({ ...prev, [uploadId]: true }));
        onUploadComplete();
      }
    } catch {
      setMetadataErrors((prev) => ({
        ...prev,
        [uploadId]: 'Network error',
      }));
    }

    setSavingMetadata((prev) => ({ ...prev, [uploadId]: false }));
  };

  return (
    <div className="space-y-[var(--space-sm)]">
      {/* Compact drop zone */}
      <div
        {...getRootProps()}
        className="card cursor-pointer text-center"
        style={{
          padding: 'var(--space-lg) var(--space-md)',
          borderStyle: 'dashed',
          borderWidth: '2px',
          borderColor: isDragActive
            ? 'var(--amber-accent)'
            : 'var(--border-default)',
          backgroundColor: isDragActive
            ? 'var(--amber-subtle)'
            : 'var(--bg-elevated)',
          transition: 'all var(--duration-normal) ease',
        }}
      >
        <input {...getInputProps()} />
        <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
          {isDragActive
            ? 'Drop files here'
            : 'Drag files here or click to browse'}
        </p>
      </div>

      {/* Upload progress items */}
      {uploads.map((u) => (
        <div key={u.id} className="space-y-[var(--space-xs)]">
          <div
            className="flex items-center gap-[var(--space-sm)] px-[var(--space-md)] py-[var(--space-sm)]"
            style={{
              backgroundColor: 'var(--bg-elevated)',
              border: '1px solid var(--border-subtle)',
              borderRadius: 'var(--radius-md)',
            }}
          >
            <div className="flex-1 min-w-0">
              <p
                className="text-sm font-medium truncate"
                style={{ color: 'var(--text-primary)' }}
              >
                {u.file.name}
              </p>
              <p className="text-xs" style={{ color: 'var(--text-tertiary)' }}>
                {formatFileSize(u.file.size)}
              </p>
            </div>

            {u.status === 'hashing' && (
              <div className="flex items-center gap-[var(--space-sm)] shrink-0">
                <span
                  className="text-xs"
                  style={{ color: 'var(--text-secondary)' }}
                  aria-live="polite"
                >
                  Computing fingerprint…
                </span>
                <div
                  className="w-24 rounded-full overflow-hidden"
                  style={{ height: '4px', backgroundColor: 'var(--bg-inset)' }}
                >
                  <div
                    className="h-full rounded-full"
                    style={{
                      width: `${u.hashProgress}%`,
                      backgroundColor: 'var(--teal-accent)',
                      transition: 'width 200ms ease',
                    }}
                  />
                </div>
                <button
                  type="button"
                  onClick={() => cancelHashing(u.id)}
                  className="btn-ghost text-xs"
                >
                  Cancel
                </button>
              </div>
            )}

            {u.status === 'uploading' && (
              <div className="flex items-center gap-[var(--space-sm)] shrink-0">
                <div
                  className="w-24 rounded-full overflow-hidden"
                  style={{ height: '4px', backgroundColor: 'var(--bg-inset)' }}
                >
                  <div
                    className="h-full rounded-full"
                    style={{
                      width: `${u.progress}%`,
                      backgroundColor: 'var(--amber-accent)',
                      transition: 'width 200ms ease',
                    }}
                  />
                </div>
                <span
                  className="text-xs tabular-nums font-[family-name:var(--font-mono)]"
                  style={{ color: 'var(--text-secondary)' }}
                >
                  {u.progress}%
                </span>
              </div>
            )}

            {u.status === 'complete' && (
              <div className="flex items-center gap-[var(--space-xs)] shrink-0">
                {u.clientHash && u.serverHash && u.clientHash === u.serverHash && (
                  <span
                    className="text-xs"
                    style={{ color: 'var(--status-active)' }}
                    title="Client and server hashes match"
                  >
                    ✓
                  </span>
                )}
                <span
                  className="text-xs font-medium"
                  style={{ color: 'var(--status-active)' }}
                >
                  Uploaded
                </span>
              </div>
            )}

            {u.status === 'error' && (
              <div className="flex items-center gap-[var(--space-xs)] shrink-0">
                <span
                  className="text-xs"
                  style={{ color: 'var(--status-hold)' }}
                >
                  {u.error || 'Failed'}
                </span>
                <button
                  type="button"
                  onClick={() => dismiss(u.id)}
                  className="btn-ghost text-xs"
                >
                  Dismiss
                </button>
              </div>
            )}
          </div>

          {/* Integrity receipt panel */}
          {u.clientHash && (u.status === 'uploading' || u.status === 'complete') && (
            <div
              style={{
                backgroundColor: 'var(--bg-elevated)',
                border: '1px solid var(--border-subtle)',
                borderRadius: 'var(--radius-md)',
              }}
            >
              <button
                type="button"
                onClick={() =>
                  setReceiptExpanded((prev) => ({
                    ...prev,
                    [u.id]: !prev[u.id],
                  }))
                }
                className="w-full flex items-center justify-between px-[var(--space-md)] py-[var(--space-xs)]"
                style={{ color: 'var(--text-secondary)' }}
              >
                <span className="text-xs font-medium">
                  Integrity receipt
                </span>
                <span className="text-xs">
                  {receiptExpanded[u.id] ? '▴' : '▾'}
                </span>
              </button>
              {receiptExpanded[u.id] && (
                <div className="px-[var(--space-md)] pb-[var(--space-sm)] space-y-[var(--space-xs)]">
                  <p
                    className="text-xs"
                    style={{ color: 'var(--text-tertiary)' }}
                  >
                    This is the cryptographic fingerprint of your file. Save
                    this value — you can verify it later against the
                    server-stored hash to confirm nothing was altered in
                    transit.
                  </p>
                  <div className="flex items-center gap-[var(--space-xs)]">
                    <code
                      className="text-xs break-all font-[family-name:var(--font-mono)]"
                      style={{ color: 'var(--text-primary)' }}
                    >
                      {u.clientHash}
                    </code>
                    <button
                      type="button"
                      onClick={() => copyHash(u.id, u.clientHash!)}
                      className="btn-ghost text-xs shrink-0"
                    >
                      {copiedHash[u.id] ? 'Copied' : 'Copy'}
                    </button>
                  </div>
                </div>
              )}
            </div>
          )}

          {/* Post-upload metadata form */}
          {u.status === 'complete' &&
            u.evidenceId &&
            !metadataSaved[u.id] && (
              <MetadataFormCard
                form={metadataForms[u.id] || defaultMetadata(u.file.name)}
                saving={savingMetadata[u.id] || false}
                error={metadataErrors[u.id] || null}
                onFieldChange={(field, value) =>
                  updateMetadataField(u.id, field, value)
                }
                onSave={() => saveMetadata(u.id, u.evidenceId!)}
                onDismiss={() => dismiss(u.id)}
              />
            )}

          {u.status === 'complete' && metadataSaved[u.id] && (
            <div
              className="px-[var(--space-md)] py-[var(--space-sm)]"
              style={{
                backgroundColor: 'var(--bg-elevated)',
                border: '1px solid var(--border-subtle)',
                borderRadius: 'var(--radius-md)',
              }}
            >
              <p
                className="text-xs font-medium"
                style={{ color: 'var(--status-active)' }}
              >
                Metadata saved
              </p>
            </div>
          )}
        </div>
      ))}

      {/* Hash mismatch modal */}
      {mismatchModal && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center"
          style={{ backgroundColor: 'rgba(0,0,0,0.5)' }}
          role="dialog"
          aria-modal="true"
          aria-labelledby="mismatch-title"
        >
          <div
            className="card p-[var(--space-lg)] space-y-[var(--space-md)]"
            style={{
              maxWidth: '480px',
              width: '90vw',
              backgroundColor: 'var(--bg-surface)',
            }}
          >
            <h3
              id="mismatch-title"
              className="text-base font-semibold"
              style={{ color: 'var(--status-hold)' }}
            >
              Upload failed integrity check
            </h3>
            <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
              The file that reached our server does not match the file on your
              device. This is usually caused by a flaky connection or antivirus
              interference. Your original file is untouched.
            </p>
            <div className="flex gap-[var(--space-sm)]">
              <button
                type="button"
                onClick={() => retryUpload(mismatchModal.uploadId)}
                className="btn-primary"
              >
                Retry upload
              </button>
              <button
                type="button"
                onClick={() =>
                  downloadDiagnostic(mismatchModal.diagnostic)
                }
                className="btn-secondary"
              >
                Download diagnostic report
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function MetadataFormCard({
  form,
  saving,
  error,
  onFieldChange,
  onSave,
  onDismiss,
}: {
  form: MetadataForm;
  saving: boolean;
  error: string | null;
  onFieldChange: (field: keyof MetadataForm, value: string) => void;
  onSave: () => void;
  onDismiss: () => void;
}) {
  return (
    <div className="card p-[var(--space-md)] space-y-[var(--space-sm)]">
      <p
        className="text-sm font-medium"
        style={{ color: 'var(--text-primary)' }}
      >
        Add metadata
      </p>

      {error && (
        <p className="text-xs" style={{ color: 'var(--status-hold)' }}>
          {error}
        </p>
      )}

      <div>
        <label className="field-label">Title</label>
        <input
          type="text"
          value={form.title}
          onChange={(e) => onFieldChange('title', e.target.value)}
          className="input-field"
        />
      </div>

      <div>
        <label className="field-label">Classification</label>
        <select
          value={form.classification}
          onChange={(e) => onFieldChange('classification', e.target.value)}
          className="input-field"
        >
          <option value="public">Public</option>
          <option value="restricted">Restricted</option>
          <option value="confidential">Confidential</option>
          <option value="ex_parte">Ex parte</option>
        </select>
      </div>

      <div>
        <label className="field-label">Description</label>
        <textarea
          value={form.description}
          onChange={(e) => onFieldChange('description', e.target.value)}
          className="input-field"
          rows={3}
        />
      </div>

      <div>
        <label className="field-label">Tags (comma-separated)</label>
        <input
          type="text"
          value={form.tags}
          onChange={(e) => onFieldChange('tags', e.target.value)}
          className="input-field"
          placeholder="e.g. photo, scene, exhibit-a"
        />
      </div>

      <div>
        <label className="field-label">Source</label>
        <input
          type="text"
          value={form.source}
          onChange={(e) => onFieldChange('source', e.target.value)}
          className="input-field"
        />
      </div>

      <div>
        <label className="field-label">Source date</label>
        <input
          type="date"
          value={form.sourceDate}
          onChange={(e) => onFieldChange('sourceDate', e.target.value)}
          className="input-field"
        />
      </div>

      <div className="flex gap-[var(--space-sm)]">
        <button
          type="button"
          onClick={onSave}
          disabled={saving}
          className="btn-primary"
        >
          {saving ? 'Saving...' : 'Save metadata'}
        </button>
        <button
          type="button"
          onClick={onDismiss}
          className="btn-secondary"
        >
          Skip
        </button>
      </div>
    </div>
  );
}
