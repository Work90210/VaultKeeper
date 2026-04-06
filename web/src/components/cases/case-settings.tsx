'use client';

import { useRouter } from 'next/navigation';
import { useState } from 'react';

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

    const res = await fetch(`${API_BASE}/api/cases/${caseData.id}`, {
      method: 'PATCH',
      headers,
      body: JSON.stringify({ title, description, jurisdiction }),
    });

    const data = await res.json();
    setLoading(false);

    if (!res.ok) {
      setError(data.error || 'Update failed');
      return;
    }
    setSuccess('Case updated');
  };

  const handleLegalHold = async () => {
    const newHold = !legalHold;
    const msg = newHold
      ? 'Set legal hold? This prevents archival.'
      : 'Release legal hold?';
    if (!window.confirm(msg)) return;

    const res = await fetch(`${API_BASE}/api/cases/${caseData.id}/legal-hold`, {
      method: 'POST',
      headers,
      body: JSON.stringify({ hold: newHold }),
    });

    if (res.ok) {
      setLegalHold(newHold);
      setSuccess(newHold ? 'Legal hold set' : 'Legal hold released');
      setError('');
    } else {
      const data = await res.json();
      setError(data.error || 'Failed to toggle legal hold');
    }
  };

  const handleArchive = async () => {
    if (!window.confirm('Archive this case? This cannot be undone.')) return;

    const res = await fetch(`${API_BASE}/api/cases/${caseData.id}/archive`, {
      method: 'POST',
      headers,
    });

    if (res.ok) {
      router.push(`/en/cases/${caseData.id}`);
    } else {
      const data = await res.json();
      setError(data.error || 'Archive failed');
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold tracking-tight">Case Settings</h1>
        <span className="font-mono text-sm text-zinc-500">{caseData.reference_code}</span>
      </div>

      {error && (
        <div className="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          {error}
        </div>
      )}
      {success && (
        <div className="rounded-md border border-green-200 bg-green-50 px-4 py-3 text-sm text-green-700">
          {success}
        </div>
      )}

      <form onSubmit={handleUpdate} className="space-y-4 rounded-md border p-4">
        <h2 className="font-semibold">Edit Case</h2>
        <div>
          <label htmlFor="title" className="mb-1 block text-sm font-medium">Title</label>
          <input
            id="title"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            className="w-full rounded-md border px-3 py-2 text-sm"
            maxLength={500}
          />
        </div>
        <div>
          <label htmlFor="description" className="mb-1 block text-sm font-medium">Description</label>
          <textarea
            id="description"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            className="w-full rounded-md border px-3 py-2 text-sm"
            rows={3}
            maxLength={10000}
          />
        </div>
        <div>
          <label htmlFor="jurisdiction" className="mb-1 block text-sm font-medium">Jurisdiction</label>
          <input
            id="jurisdiction"
            value={jurisdiction}
            onChange={(e) => setJurisdiction(e.target.value)}
            className="w-full rounded-md border px-3 py-2 text-sm"
            maxLength={200}
          />
        </div>
        <button
          type="submit"
          disabled={loading}
          className="rounded-md bg-zinc-900 px-4 py-2 text-sm font-medium text-white hover:bg-zinc-800 disabled:opacity-50"
        >
          {loading ? 'Saving...' : 'Save Changes'}
        </button>
      </form>

      <div className="space-y-3 rounded-md border p-4">
        <h2 className="font-semibold">Legal Hold</h2>
        <p className="text-sm text-zinc-500">
          {legalHold
            ? 'Legal hold is active. Case cannot be archived.'
            : 'No legal hold. Case can be archived.'}
        </p>
        <button
          onClick={handleLegalHold}
          className={`rounded-md px-4 py-2 text-sm font-medium ${
            legalHold
              ? 'border border-green-600 text-green-700 hover:bg-green-50'
              : 'border border-red-600 text-red-700 hover:bg-red-50'
          }`}
          type="button"
        >
          {legalHold ? 'Release Legal Hold' : 'Set Legal Hold'}
        </button>
      </div>

      {caseData.status !== 'archived' && (
        <div className="space-y-3 rounded-md border border-red-200 p-4">
          <h2 className="font-semibold text-red-700">Danger Zone</h2>
          <p className="text-sm text-zinc-500">
            Archiving a case is permanent. The case must be closed first.
          </p>
          <button
            onClick={handleArchive}
            disabled={legalHold || caseData.status !== 'closed'}
            className="rounded-md bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700 disabled:opacity-50"
            type="button"
          >
            Archive Case
          </button>
          {legalHold && (
            <p className="text-xs text-red-600">Release legal hold before archiving.</p>
          )}
          {caseData.status === 'active' && (
            <p className="text-xs text-zinc-500">Close the case before archiving.</p>
          )}
        </div>
      )}

      <a
        href={`/en/cases/${caseData.id}`}
        className="inline-block text-sm text-zinc-500 hover:text-zinc-900"
      >
        Back to case
      </a>
    </div>
  );
}
