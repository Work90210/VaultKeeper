'use client';

import { useCallback, useState } from 'react';
import { useDropzone } from 'react-dropzone';
import { formatFileSize } from '@/lib/evidence-utils';

interface BulkJob {
  readonly id: string;
  readonly total_files: number;
  readonly processed_files: number;
  readonly failed_files: number;
  readonly status: string;
  readonly archive_sha256?: string;
  readonly errors?: ReadonlyArray<{
    readonly filename: string;
    readonly reason: string;
  }>;
}

interface MigrationInfo {
  readonly id: string;
  readonly total_items: number;
  readonly matched_items: number;
  readonly mismatched_items: number;
  readonly status: string;
  readonly tsa_name?: string;
  readonly tsa_timestamp?: string | null;
}

type ImportResult =
  | { kind: 'bulk'; bulk_job: BulkJob }
  | { kind: 'migration'; migration: MigrationInfo };

type UIState =
  | { kind: 'idle' }
  | { kind: 'uploading'; filename: string; size: number }
  | { kind: 'done'; result: ImportResult; archiveName: string }
  | { kind: 'error'; message: string };

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

export function ImportArchive({
  caseId,
  accessToken,
  onImportComplete,
}: {
  caseId: string;
  accessToken: string;
  onImportComplete: () => void;
}) {
  const [state, setState] = useState<UIState>({ kind: 'idle' });
  const [sourceSystem, setSourceSystem] = useState<string>('RelativityOne');
  const [classification, setClassification] = useState<string>('restricted');
  const [haltOnMismatch, setHaltOnMismatch] = useState<boolean>(true);
  const [certDownloading, setCertDownloading] = useState<boolean>(false);

  const submitZip = useCallback(
    async (file: File) => {
      setState({ kind: 'uploading', filename: file.name, size: file.size });

      const formData = new FormData();
      formData.append('archive', file);
      formData.append('source_system', sourceSystem);
      formData.append('classification', classification);
      formData.append('halt_on_mismatch', haltOnMismatch ? 'true' : 'false');

      try {
        const res = await fetch(
          `${API_BASE}/api/cases/${caseId}/evidence/import`,
          {
            method: 'POST',
            headers: { Authorization: `Bearer ${accessToken}` },
            body: formData,
          }
        );
        const json = await res.json().catch(() => null);
        if (!res.ok) {
          const msg = json?.error || `Import failed (${res.status} ${res.statusText})`;
          setState({ kind: 'error', message: msg });
          return;
        }
        const result = json?.data as ImportResult | undefined;
        if (!result || !result.kind) {
          setState({
            kind: 'error',
            message: 'Server returned an unexpected response shape',
          });
          return;
        }
        setState({ kind: 'done', result, archiveName: file.name });
        onImportComplete();
      } catch (err) {
        setState({
          kind: 'error',
          message: err instanceof Error ? err.message : 'Network error during upload',
        });
      }
    },
    [caseId, accessToken, sourceSystem, classification, haltOnMismatch, onImportComplete]
  );

  const onDrop = useCallback(
    (acceptedFiles: File[]) => {
      if (acceptedFiles.length === 0) return;
      const file = acceptedFiles[0];
      if (!file.name.toLowerCase().endsWith('.zip')) {
        setState({
          kind: 'error',
          message: 'Only .zip archives are accepted',
        });
        return;
      }
      submitZip(file);
    },
    [submitZip]
  );

  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    onDrop,
    maxFiles: 1,
    accept: { 'application/zip': ['.zip'] },
    disabled: state.kind === 'uploading',
  });

  const downloadCertificate = useCallback(
    async (migrationId: string) => {
      setCertDownloading(true);
      try {
        const res = await fetch(
          `${API_BASE}/api/migrations/${migrationId}/certificate`,
          {
            headers: { Authorization: `Bearer ${accessToken}` },
          }
        );
        if (!res.ok) {
          setState({
            kind: 'error',
            message: `Certificate download failed (${res.status})`,
          });
          return;
        }
        const blob = await res.blob();
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `migration-attestation-${migrationId}.pdf`;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
      } finally {
        setCertDownloading(false);
      }
    },
    [accessToken]
  );

  const reset = () => setState({ kind: 'idle' });

  return (
    <div className="space-y-[var(--space-sm)]">
      {/* Explainer */}
      <div
        className="px-[var(--space-md)] py-[var(--space-sm)]"
        style={{
          backgroundColor: 'var(--bg-elevated)',
          border: '1px solid var(--border-subtle)',
          borderRadius: 'var(--radius-md)',
        }}
      >
        <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
          Drag a ZIP archive here. If it contains a{' '}
          <code className="font-[family-name:var(--font-mono)]">manifest.csv</code>{' '}
          at the root (e.g. a RelativityOne export), each file&apos;s source
          hash is verified against a fresh hash on ingestion, the whole
          batch is stamped with a trusted RFC 3161 timestamp, and you get a
          signed attestation certificate. Otherwise the files are imported
          as plain evidence with the default classification below.
        </p>
      </div>

      {/* Options */}
      <div className="grid grid-cols-2 gap-[var(--space-sm)]">
        <div>
          <label className="field-label">Source system</label>
          <input
            type="text"
            value={sourceSystem}
            onChange={(e) => setSourceSystem(e.target.value)}
            className="input-field"
            placeholder="RelativityOne"
            disabled={state.kind === 'uploading'}
          />
        </div>
        <div>
          <label className="field-label">Default classification</label>
          <select
            value={classification}
            onChange={(e) => setClassification(e.target.value)}
            className="input-field"
            disabled={state.kind === 'uploading'}
          >
            <option value="public">Public</option>
            <option value="restricted">Restricted</option>
            <option value="confidential">Confidential</option>
            <option value="ex_parte">Ex parte</option>
          </select>
        </div>
      </div>

      <label
        className="flex items-center gap-[var(--space-xs)] text-sm"
        style={{ color: 'var(--text-secondary)' }}
      >
        <input
          type="checkbox"
          checked={haltOnMismatch}
          onChange={(e) => setHaltOnMismatch(e.target.checked)}
          disabled={state.kind === 'uploading'}
        />
        Halt on hash mismatch (verified migrations only)
      </label>

      {/* Drop zone */}
      <div
        {...getRootProps()}
        className="card text-center"
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
          cursor: state.kind === 'uploading' ? 'not-allowed' : 'pointer',
          opacity: state.kind === 'uploading' ? 0.6 : 1,
          transition: 'all var(--duration-normal) ease',
        }}
      >
        <input {...getInputProps()} />
        <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
          {isDragActive
            ? 'Drop the ZIP archive here'
            : state.kind === 'uploading'
              ? 'Importing…'
              : 'Drag a .zip archive here or click to browse'}
        </p>
      </div>

      {/* Uploading state */}
      {state.kind === 'uploading' && (
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
              {state.filename}
            </p>
            <p className="text-xs" style={{ color: 'var(--text-tertiary)' }}>
              {formatFileSize(state.size)} — extracting, hashing, and
              ingesting…
            </p>
          </div>
          <div
            className="w-24 rounded-full overflow-hidden"
            style={{ height: '4px', backgroundColor: 'var(--bg-inset)' }}
          >
            <div
              className="h-full rounded-full animate-pulse"
              style={{
                width: '60%',
                backgroundColor: 'var(--amber-accent)',
              }}
            />
          </div>
        </div>
      )}

      {/* Error state */}
      {state.kind === 'error' && (
        <div
          className="px-[var(--space-md)] py-[var(--space-sm)] flex items-start justify-between gap-[var(--space-sm)]"
          style={{
            backgroundColor: 'var(--bg-elevated)',
            border: '1px solid var(--status-hold)',
            borderRadius: 'var(--radius-md)',
          }}
        >
          <div className="min-w-0">
            <p
              className="text-sm font-medium"
              style={{ color: 'var(--status-hold)' }}
            >
              Import failed
            </p>
            <p
              className="text-xs mt-[var(--space-2xs)]"
              style={{ color: 'var(--text-secondary)' }}
            >
              {state.message}
            </p>
          </div>
          <button
            type="button"
            onClick={reset}
            className="btn-ghost text-xs shrink-0"
          >
            Dismiss
          </button>
        </div>
      )}

      {/* Success — bulk result */}
      {state.kind === 'done' && state.result.kind === 'bulk' && (
        <BulkResultCard
          job={state.result.bulk_job}
          archiveName={state.archiveName}
          onDismiss={reset}
        />
      )}

      {/* Success — migration result */}
      {state.kind === 'done' && state.result.kind === 'migration' && (
        <MigrationResultCard
          migration={state.result.migration}
          archiveName={state.archiveName}
          onDownloadCertificate={() =>
            downloadCertificate(state.result.kind === 'migration' ? state.result.migration.id : '')
          }
          downloading={certDownloading}
          onDismiss={reset}
        />
      )}
    </div>
  );
}

function BulkResultCard({
  job,
  archiveName,
  onDismiss,
}: {
  job: BulkJob;
  archiveName: string;
  onDismiss: () => void;
}) {
  const partial = job.status === 'completed_with_errors' || job.failed_files > 0;
  const badgeColor = partial ? 'var(--amber-accent)' : 'var(--status-active)';
  const badgeText = partial ? 'Imported with errors' : 'Imported';

  return (
    <div className="card p-[var(--space-md)] space-y-[var(--space-sm)]">
      <div className="flex items-start justify-between gap-[var(--space-sm)]">
        <div className="min-w-0">
          <p
            className="text-xs font-medium uppercase tracking-wider"
            style={{ color: 'var(--text-tertiary)' }}
          >
            Bulk upload · no hash verification
          </p>
          <p
            className="text-sm font-medium truncate mt-[var(--space-2xs)]"
            style={{ color: 'var(--text-primary)' }}
          >
            {archiveName}
          </p>
        </div>
        <span
          className="text-xs font-medium shrink-0"
          style={{ color: badgeColor }}
        >
          {badgeText}
        </span>
      </div>

      <div
        className="grid grid-cols-3 gap-[var(--space-sm)] pt-[var(--space-sm)]"
        style={{ borderTop: '1px solid var(--border-subtle)' }}
      >
        <Stat label="Total files" value={job.total_files.toString()} />
        <Stat
          label="Imported"
          value={job.processed_files.toString()}
          color="var(--status-active)"
        />
        <Stat
          label="Failed"
          value={job.failed_files.toString()}
          color={job.failed_files > 0 ? 'var(--status-hold)' : undefined}
        />
      </div>

      {job.archive_sha256 && (
        <div>
          <p className="text-xs" style={{ color: 'var(--text-tertiary)' }}>
            Archive SHA-256
          </p>
          <p
            className="text-xs font-[family-name:var(--font-mono)] break-all"
            style={{ color: 'var(--text-secondary)' }}
          >
            {job.archive_sha256}
          </p>
        </div>
      )}

      {job.errors && job.errors.length > 0 && (
        <div
          className="mt-[var(--space-sm)] pt-[var(--space-sm)]"
          style={{ borderTop: '1px solid var(--border-subtle)' }}
        >
          <p
            className="text-xs font-medium mb-[var(--space-xs)]"
            style={{ color: 'var(--status-hold)' }}
          >
            Per-file failures
          </p>
          <ul className="space-y-[var(--space-2xs)]">
            {job.errors.map((e, i) => (
              <li
                key={`${e.filename}-${i}`}
                className="text-xs"
                style={{ color: 'var(--text-secondary)' }}
              >
                <span className="font-[family-name:var(--font-mono)]">
                  {e.filename}
                </span>
                : {e.reason}
              </li>
            ))}
          </ul>
        </div>
      )}

      <div className="flex justify-end">
        <button type="button" onClick={onDismiss} className="btn-secondary">
          Close
        </button>
      </div>
    </div>
  );
}

function MigrationResultCard({
  migration,
  archiveName,
  onDownloadCertificate,
  downloading,
  onDismiss,
}: {
  migration: MigrationInfo;
  archiveName: string;
  onDownloadCertificate: () => void;
  downloading: boolean;
  onDismiss: () => void;
}) {
  return (
    <div
      className="card p-[var(--space-md)] space-y-[var(--space-sm)]"
      style={{ borderColor: 'var(--status-active)' }}
    >
      <div className="flex items-start justify-between gap-[var(--space-sm)]">
        <div className="min-w-0">
          <p
            className="text-xs font-medium uppercase tracking-wider"
            style={{ color: 'var(--status-active)' }}
          >
            Verified migration · hash-bridged
          </p>
          <p
            className="text-sm font-medium truncate mt-[var(--space-2xs)]"
            style={{ color: 'var(--text-primary)' }}
          >
            {archiveName}
          </p>
          <p
            className="text-xs font-[family-name:var(--font-mono)] truncate mt-[var(--space-2xs)]"
            style={{ color: 'var(--text-tertiary)' }}
          >
            {migration.id}
          </p>
        </div>
        <span
          className="text-xs font-medium shrink-0"
          style={{ color: 'var(--status-active)' }}
        >
          {migration.status}
        </span>
      </div>

      <div
        className="grid grid-cols-3 gap-[var(--space-sm)] pt-[var(--space-sm)]"
        style={{ borderTop: '1px solid var(--border-subtle)' }}
      >
        <Stat label="Total" value={migration.total_items.toString()} />
        <Stat
          label="Matched"
          value={migration.matched_items.toString()}
          color="var(--status-active)"
        />
        <Stat
          label="Mismatched"
          value={migration.mismatched_items.toString()}
          color={
            migration.mismatched_items > 0 ? 'var(--status-hold)' : undefined
          }
        />
      </div>

      {migration.tsa_timestamp && (
        <div>
          <p className="text-xs" style={{ color: 'var(--text-tertiary)' }}>
            RFC 3161 timestamp
          </p>
          <p
            className="text-xs"
            style={{ color: 'var(--text-secondary)' }}
          >
            {migration.tsa_name} —{' '}
            {new Date(migration.tsa_timestamp).toLocaleString()}
          </p>
        </div>
      )}

      <div className="flex gap-[var(--space-sm)] justify-end pt-[var(--space-xs)]">
        <button
          type="button"
          onClick={onDownloadCertificate}
          disabled={downloading}
          className="btn-primary"
        >
          {downloading ? 'Downloading…' : 'Download attestation PDF'}
        </button>
        <button type="button" onClick={onDismiss} className="btn-secondary">
          Close
        </button>
      </div>
    </div>
  );
}

function Stat({
  label,
  value,
  color,
}: {
  label: string;
  value: string;
  color?: string;
}) {
  return (
    <div>
      <p className="text-xs" style={{ color: 'var(--text-tertiary)' }}>
        {label}
      </p>
      <p
        className="text-sm font-medium tabular-nums font-[family-name:var(--font-mono)]"
        style={{ color: color || 'var(--text-primary)' }}
      >
        {value}
      </p>
    </div>
  );
}
