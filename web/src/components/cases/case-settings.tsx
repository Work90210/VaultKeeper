'use client';

import { useRouter } from 'next/navigation';
import { useState } from 'react';
import { ImportArchive } from '@/components/evidence/import-archive';

interface CaseData {
  id: string;
  reference_code: string;
  title: string;
  description: string;
  jurisdiction: string;
  status: string;
  legal_hold: boolean;
}

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

export function CaseSettings({
  caseData,
  accessToken,
}: {
  caseData: CaseData;
  accessToken: string;
}) {
  const router = useRouter();
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const [title, setTitle] = useState(caseData.title);
  const [description, setDescription] = useState(caseData.description);
  const [jurisdiction, setJurisdiction] = useState(caseData.jurisdiction);
  const [legalHold, setLegalHold] = useState(caseData.legal_hold);
  const [loading, setLoading] = useState(false);

  const headers = {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${accessToken}`,
  };

  const handleUpdate = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setSuccess('');
    setLoading(true);
    try {
      const res = await fetch(`${API_BASE}/api/cases/${caseData.id}`, {
        method: 'PATCH',
        headers,
        body: JSON.stringify({ title, description, jurisdiction }),
      });
      const data = await res.json();
      if (!res.ok) {
        setError(data.error || 'Update failed');
        return;
      }
      setSuccess('Changes saved');
    } catch {
      setError('An unexpected error occurred. Please try again.');
    } finally {
      setLoading(false);
    }
  };

  const handleLegalHold = async () => {
    const newHold = !legalHold;
    if (
      !window.confirm(
        newHold
          ? 'Set legal hold? This prevents archival.'
          : 'Release legal hold?',
      )
    )
      return;
    const res = await fetch(
      `${API_BASE}/api/cases/${caseData.id}/legal-hold`,
      {
        method: 'POST',
        headers,
        body: JSON.stringify({ hold: newHold }),
      },
    );
    if (res.ok) {
      setLegalHold(newHold);
      setSuccess(newHold ? 'Legal hold set' : 'Legal hold released');
      setError('');
    } else {
      const data = await res.json();
      setError(data.error || 'Failed');
    }
  };

  const handleArchive = async () => {
    if (!window.confirm('Archive this case? This cannot be undone.')) return;
    const res = await fetch(`${API_BASE}/api/cases/${caseData.id}/archive`, {
      method: 'POST',
      headers,
    });
    if (res.ok) router.push(`/en/cases/${caseData.id}`);
    else {
      const data = await res.json();
      setError(data.error || 'Archive failed');
    }
  };

  return (
    <div
      className="space-y-[var(--space-lg)]"
      style={{
        animation: 'fade-in var(--duration-slow) var(--ease-out-expo)',
      }}
    >
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1
            className="font-[family-name:var(--font-heading)] text-2xl"
            style={{ color: 'var(--text-primary)' }}
          >
            Settings
          </h1>
          <p
            className="font-[family-name:var(--font-mono)] text-xs mt-[var(--space-xs)]"
            style={{ color: 'var(--text-tertiary)' }}
          >
            {caseData.reference_code}
          </p>
        </div>
        <a
          href={`/en/cases/${caseData.id}`}
          className="btn-ghost text-sm"
        >
          &larr; Back
        </a>
      </div>

      {/* Feedback */}
      {error && <div className="banner-error">{error}</div>}
      {success && <div className="banner-success">{success}</div>}

      {/* Edit form */}
      <div className="card p-[var(--space-lg)]">
        <form onSubmit={handleUpdate} className="space-y-[var(--space-md)]">
          <h2 className="field-label text-sm">Case details</h2>
          <div>
            <label className="field-label" htmlFor="settings-title">
              Title
            </label>
            <input
              id="settings-title"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              maxLength={500}
              className="input-field"
            />
          </div>
          <div>
            <label className="field-label" htmlFor="settings-description">
              Description
            </label>
            <textarea
              id="settings-description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={3}
              maxLength={10000}
              className="input-field resize-y"
            />
          </div>
          <div>
            <label className="field-label" htmlFor="settings-jurisdiction">
              Jurisdiction
            </label>
            <input
              id="settings-jurisdiction"
              value={jurisdiction}
              onChange={(e) => setJurisdiction(e.target.value)}
              maxLength={200}
              className="input-field"
            />
          </div>
          <button type="submit" disabled={loading} className="btn-primary">
            {loading ? 'Saving...' : 'Save changes'}
          </button>
        </form>
      </div>

      {/* Legal hold */}
      <div className="card p-[var(--space-lg)]">
        <h2 className="field-label text-sm mb-[var(--space-sm)]">
          Legal hold
        </h2>
        <p
          className="text-sm mb-[var(--space-md)]"
          style={{ color: 'var(--text-secondary)' }}
        >
          {legalHold
            ? 'Legal hold is active. This case cannot be archived or have evidence deleted.'
            : 'No legal hold. Case follows standard lifecycle rules.'}
        </p>
        <button
          onClick={handleLegalHold}
          className="btn-secondary"
          style={{
            borderColor: legalHold
              ? 'var(--status-active)'
              : 'var(--status-hold)',
            color: legalHold ? 'var(--status-active)' : 'var(--status-hold)',
          }}
          type="button"
        >
          {legalHold ? 'Release hold' : 'Set legal hold'}
        </button>
      </div>

      {/* Data import — Sprint 10 */}
      <div className="card p-[var(--space-lg)]">
        <h2 className="field-label text-sm mb-[var(--space-sm)]">
          Data import
        </h2>
        <p
          className="text-sm mb-[var(--space-md)]"
          style={{ color: 'var(--text-secondary)' }}
        >
          Bulk-import evidence from another system (e.g. RelativityOne). If
          your archive contains a <code className="font-[family-name:var(--font-mono)]">manifest.csv</code>{' '}
          at the root, every file&apos;s source hash is verified on
          ingestion, the batch is stamped with a trusted RFC 3161
          timestamp, and you receive a signed attestation certificate for
          court submission.
        </p>
        <ImportArchive
          caseId={caseData.id}
          accessToken={accessToken}
          onImportComplete={() => router.refresh()}
        />
      </div>

      {/* Archive */}
      {caseData.status !== 'archived' && (
        <div
          className="card p-[var(--space-lg)]"
          style={{ borderColor: 'var(--status-hold-bg)' }}
        >
          <h2
            className="field-label text-sm mb-[var(--space-sm)]"
            style={{ color: 'var(--status-hold)' }}
          >
            Danger zone
          </h2>
          <p
            className="text-sm mb-[var(--space-md)]"
            style={{ color: 'var(--text-secondary)' }}
          >
            Archiving is permanent. The case must be closed first.
          </p>
          <button
            onClick={handleArchive}
            disabled={legalHold || caseData.status !== 'closed'}
            className="btn-danger"
            type="button"
          >
            Archive case
          </button>
          {legalHold && (
            <p
              className="mt-[var(--space-xs)] text-xs"
              style={{ color: 'var(--status-hold)' }}
            >
              Release legal hold before archiving.
            </p>
          )}
        </div>
      )}
    </div>
  );
}
