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
      method: 'PATCH', headers,
      body: JSON.stringify({ title, description, jurisdiction }),
    });
    const data = await res.json();
    setLoading(false);
    if (!res.ok) { setError(data.error || 'Update failed'); return; }
    setSuccess('Changes saved');
  };

  const handleLegalHold = async () => {
    const newHold = !legalHold;
    if (!window.confirm(newHold ? 'Set legal hold? This prevents archival.' : 'Release legal hold?')) return;
    const res = await fetch(`${API_BASE}/api/cases/${caseData.id}/legal-hold`, {
      method: 'POST', headers,
      body: JSON.stringify({ hold: newHold }),
    });
    if (res.ok) { setLegalHold(newHold); setSuccess(newHold ? 'Legal hold set' : 'Legal hold released'); setError(''); }
    else { const data = await res.json(); setError(data.error || 'Failed'); }
  };

  const handleArchive = async () => {
    if (!window.confirm('Archive this case? This cannot be undone.')) return;
    const res = await fetch(`${API_BASE}/api/cases/${caseData.id}/archive`, { method: 'POST', headers });
    if (res.ok) router.push(`/en/cases/${caseData.id}`);
    else { const data = await res.json(); setError(data.error || 'Archive failed'); }
  };

  const inputStyle = {
    backgroundColor: 'var(--bg-elevated)',
    border: '1px solid var(--border-default)',
    color: 'var(--text-primary)',
  };

  return (
    <div className="space-y-[var(--space-xl)]" style={{ animation: 'fade-in var(--duration-slow) var(--ease-out-expo)' }}>
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1
            className="font-[family-name:var(--font-heading)] text-[var(--text-2xl)]"
            style={{ color: 'var(--text-primary)' }}
          >
            Settings
          </h1>
          <p
            className="font-[family-name:var(--font-mono)] text-[var(--text-xs)] mt-[var(--space-xs)]"
            style={{ color: 'var(--text-tertiary)' }}
          >
            {caseData.reference_code}
          </p>
        </div>
        <a
          href={`/en/cases/${caseData.id}`}
          className="text-[var(--text-sm)]"
          style={{ color: 'var(--amber-accent)' }}
        >
          &larr; Back
        </a>
      </div>

      {/* Feedback */}
      {error && (
        <div
          className="px-[var(--space-md)] py-[var(--space-sm)] text-[var(--text-sm)]"
          style={{ backgroundColor: 'var(--status-hold-bg)', color: 'var(--status-hold)', borderLeft: '3px solid var(--status-hold)' }}
        >
          {error}
        </div>
      )}
      {success && (
        <div
          className="px-[var(--space-md)] py-[var(--space-sm)] text-[var(--text-sm)]"
          style={{ backgroundColor: 'var(--status-active-bg)', color: 'var(--status-active)', borderLeft: '3px solid var(--status-active)' }}
        >
          {success}
        </div>
      )}

      {/* Edit form */}
      <form onSubmit={handleUpdate} className="space-y-[var(--space-md)]">
        <h2 className="text-[var(--text-xs)] uppercase tracking-wider font-medium" style={{ color: 'var(--text-tertiary)' }}>
          Case details
        </h2>
        <div>
          <label className="block text-[var(--text-xs)] uppercase tracking-wider font-medium mb-[var(--space-xs)]" style={{ color: 'var(--text-tertiary)' }}>Title</label>
          <input value={title} onChange={(e) => setTitle(e.target.value)} maxLength={500}
            className="w-full px-[var(--space-sm)] py-[var(--space-xs)] text-[var(--text-base)] transition-colors"
            style={inputStyle}
            onFocus={(e) => { e.currentTarget.style.borderColor = 'var(--amber-accent)'; }}
            onBlur={(e) => { e.currentTarget.style.borderColor = 'var(--border-default)'; }}
          />
        </div>
        <div>
          <label className="block text-[var(--text-xs)] uppercase tracking-wider font-medium mb-[var(--space-xs)]" style={{ color: 'var(--text-tertiary)' }}>Description</label>
          <textarea value={description} onChange={(e) => setDescription(e.target.value)} rows={3} maxLength={10000}
            className="w-full px-[var(--space-sm)] py-[var(--space-xs)] text-[var(--text-base)] resize-y transition-colors"
            style={inputStyle}
            onFocus={(e) => { e.currentTarget.style.borderColor = 'var(--amber-accent)'; }}
            onBlur={(e) => { e.currentTarget.style.borderColor = 'var(--border-default)'; }}
          />
        </div>
        <div>
          <label className="block text-[var(--text-xs)] uppercase tracking-wider font-medium mb-[var(--space-xs)]" style={{ color: 'var(--text-tertiary)' }}>Jurisdiction</label>
          <input value={jurisdiction} onChange={(e) => setJurisdiction(e.target.value)} maxLength={200}
            className="w-full px-[var(--space-sm)] py-[var(--space-xs)] text-[var(--text-base)] transition-colors"
            style={inputStyle}
            onFocus={(e) => { e.currentTarget.style.borderColor = 'var(--amber-accent)'; }}
            onBlur={(e) => { e.currentTarget.style.borderColor = 'var(--border-default)'; }}
          />
        </div>
        <button
          type="submit" disabled={loading}
          className="px-[var(--space-md)] py-[var(--space-sm)] text-[var(--text-sm)] font-medium transition-all disabled:opacity-40"
          style={{ backgroundColor: 'var(--amber-accent)', color: 'var(--stone-950)' }}
        >
          {loading ? 'Saving...' : 'Save changes'}
        </button>
      </form>

      {/* Legal hold */}
      <div style={{ borderTop: '1px solid var(--border-default)' }} className="pt-[var(--space-lg)]">
        <h2 className="text-[var(--text-xs)] uppercase tracking-wider font-medium mb-[var(--space-sm)]" style={{ color: 'var(--text-tertiary)' }}>
          Legal hold
        </h2>
        <p className="text-[var(--text-sm)] mb-[var(--space-md)]" style={{ color: 'var(--text-secondary)' }}>
          {legalHold
            ? 'Legal hold is active. This case cannot be archived or have evidence deleted.'
            : 'No legal hold. Case follows standard lifecycle rules.'}
        </p>
        <button
          onClick={handleLegalHold}
          className="px-[var(--space-md)] py-[var(--space-xs)] text-[var(--text-sm)] font-medium transition-colors"
          style={{
            border: `1px solid ${legalHold ? 'var(--status-active)' : 'var(--status-hold)'}`,
            color: legalHold ? 'var(--status-active)' : 'var(--status-hold)',
          }}
          type="button"
        >
          {legalHold ? 'Release hold' : 'Set legal hold'}
        </button>
      </div>

      {/* Archive */}
      {caseData.status !== 'archived' && (
        <div style={{ borderTop: '1px solid var(--border-default)' }} className="pt-[var(--space-lg)]">
          <h2 className="text-[var(--text-xs)] uppercase tracking-wider font-medium mb-[var(--space-sm)]" style={{ color: 'var(--status-hold)' }}>
            Danger zone
          </h2>
          <p className="text-[var(--text-sm)] mb-[var(--space-md)]" style={{ color: 'var(--text-secondary)' }}>
            Archiving is permanent. The case must be closed first.
          </p>
          <button
            onClick={handleArchive}
            disabled={legalHold || caseData.status !== 'closed'}
            className="px-[var(--space-md)] py-[var(--space-xs)] text-[var(--text-sm)] font-medium transition-all disabled:opacity-30"
            style={{ backgroundColor: 'var(--status-hold)', color: 'var(--text-inverse)' }}
            type="button"
          >
            Archive case
          </button>
          {legalHold && (
            <p className="mt-[var(--space-xs)] text-[var(--text-xs)]" style={{ color: 'var(--status-hold)' }}>
              Release legal hold before archiving.
            </p>
          )}
        </div>
      )}
    </div>
  );
}
