'use client';

import { useCallback, useState } from 'react';
import { useDropzone } from 'react-dropzone';
import { formatFileSize } from '@/lib/evidence-utils';

interface UploadFile {
  readonly id: string;
  readonly file: File;
  readonly progress: number;
  readonly status: 'uploading' | 'complete' | 'error';
  readonly evidenceId: string | null;
  readonly error: string | null;
}

interface MetadataForm {
  readonly title: string;
  readonly classification: string;
  readonly description: string;
  readonly tags: string;
  readonly source: string;
  readonly sourceDate: string;
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

  const uploadFile = useCallback(
    async (file: File, id: string) => {
      const formData = new FormData();
      formData.append('file', file);

      try {
        const xhr = new XMLHttpRequest();
        xhr.open('POST', `${API_BASE}/api/cases/${caseId}/evidence`);
        xhr.setRequestHeader('Authorization', `Bearer ${accessToken}`);

        xhr.upload.onprogress = (e) => {
          if (e.lengthComputable) {
            const pct = Math.min(
              99,
              Math.round((e.loaded / e.total) * 100)
            );
            setUploads((prev) =>
              prev.map((u) => (u.id === id ? { ...u, progress: pct } : u))
            );
          }
        };

        const result = await new Promise<{
          evidenceId: string | null;
          error: string | null;
        }>((resolve) => {
          xhr.onload = () => {
            if (xhr.status >= 200 && xhr.status < 300) {
              try {
                const data = JSON.parse(xhr.responseText);
                resolve({ evidenceId: data.data?.id || null, error: null });
              } catch {
                resolve({
                  evidenceId: null,
                  error: 'Invalid server response',
                });
              }
            } else {
              try {
                const data = JSON.parse(xhr.responseText);
                resolve({
                  evidenceId: null,
                  error: data.error || `Upload failed (${xhr.status})`,
                });
              } catch {
                resolve({
                  evidenceId: null,
                  error: `Upload failed (${xhr.status})`,
                });
              }
            }
          };
          xhr.onerror = () =>
            resolve({ evidenceId: null, error: 'Network error' });
          xhr.send(formData);
        });

        if (result.error) {
          setUploads((prev) =>
            prev.map((u) =>
              u.id === id
                ? { ...u, status: 'error' as const, error: result.error }
                : u
            )
          );
        } else {
          setUploads((prev) =>
            prev.map((u) =>
              u.id === id
                ? {
                    ...u,
                    status: 'complete' as const,
                    progress: 100,
                    evidenceId: result.evidenceId,
                  }
                : u
            )
          );
          onUploadComplete();
        }
      } catch {
        setUploads((prev) =>
          prev.map((u) =>
            u.id === id
              ? { ...u, status: 'error' as const, error: 'Upload failed' }
              : u
          )
        );
      }
    },
    [caseId, accessToken, onUploadComplete]
  );

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
            status: 'uploading',
            evidenceId: null,
            error: null,
          },
        ]);
        setMetadataForms((prev) => ({
          ...prev,
          [id]: defaultMetadata(file.name),
        }));
        uploadFile(file, id);
      }
    },
    [uploadFile]
  );

  const { getRootProps, getInputProps, isDragActive } = useDropzone({ onDrop });

  const dismiss = (id: string) => {
    setUploads((prev) => prev.filter((u) => u.id !== id));
    setMetadataForms((prev) => {
      // eslint-disable-next-line @typescript-eslint/no-unused-vars
      const { [id]: _removed, ...rest } = prev;
      return rest;
    });
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
              <span
                className="text-xs font-medium"
                style={{ color: 'var(--status-active)' }}
              >
                Uploaded
              </span>
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
