'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

interface Props {
  caseId: string;
  open: boolean;
  onClose: () => void;
}

export function CaseHandoverDialog({ caseId, open, onClose }: Props) {
  const router = useRouter();
  const [fromUserId, setFromUserId] = useState('');
  const [toUserId, setToUserId] = useState('');
  const [newRoles, setNewRoles] = useState('investigator');
  const [preserveExisting, setPreserveExisting] = useState(false);
  const [reason, setReason] = useState('');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  if (!open) return null;

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!reason.trim()) { setError('Reason is required for audit purposes'); return; }
    setSubmitting(true);
    setError('');
    try {
      const res = await fetch(`${API_BASE}/api/cases/${caseId}/handover`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          from_user_id: fromUserId, to_user_id: toUserId,
          new_roles: [newRoles], preserve_existing_roles: preserveExisting,
          reason: reason.trim(),
        }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => null);
        setError(body?.error ?? 'Handover failed');
        return;
      }
      router.refresh();
      onClose();
    } catch {
      setError('Network error');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center"
      style={{ backgroundColor: 'oklch(0.1 0 0 / 0.5)' }}
    >
      <div className="card" style={{ width: '100%', maxWidth: '28rem', margin: 'var(--space-md)', padding: 'var(--space-lg)' }}>
        <h2
          className="font-[family-name:var(--font-heading)]"
          style={{ fontSize: 'var(--text-lg)', color: 'var(--text-primary)' }}
        >
          Case Handover
        </h2>
        <p style={{ marginTop: 'var(--space-xs)', fontSize: 'var(--text-sm)', color: 'var(--text-tertiary)' }}>
          Transfer case responsibility to another team member.
        </p>

        <form onSubmit={handleSubmit} style={{ marginTop: 'var(--space-md)' }} className="space-y-[var(--space-sm)]">
          <div>
            <label className="field-label">From User ID</label>
            <input type="text" value={fromUserId} onChange={(e) => setFromUserId(e.target.value)} className="input-field" required />
          </div>
          <div>
            <label className="field-label">To User ID</label>
            <input type="text" value={toUserId} onChange={(e) => setToUserId(e.target.value)} className="input-field" required />
          </div>
          <div>
            <label className="field-label">New Role for Target</label>
            <select value={newRoles} onChange={(e) => setNewRoles(e.target.value)} className="input-field">
              <option value="investigator">Investigator</option>
              <option value="prosecutor">Prosecutor</option>
              <option value="defence">Defence</option>
              <option value="judge">Judge</option>
              <option value="observer">Observer</option>
              <option value="victim_representative">Victim Representative</option>
            </select>
          </div>
          <label className="flex items-center" style={{ gap: 'var(--space-xs)', fontSize: 'var(--text-sm)', color: 'var(--text-secondary)' }}>
            <input type="checkbox" checked={preserveExisting} onChange={(e) => setPreserveExisting(e.target.checked)} />
            Preserve existing roles for source user
          </label>
          <div>
            <label className="field-label">Reason (required)</label>
            <textarea value={reason} onChange={(e) => setReason(e.target.value)} rows={2} className="input-field resize-y" required />
          </div>

          {error && <div className="banner-error">{error}</div>}

          <div className="flex justify-end" style={{ gap: 'var(--space-sm)', paddingTop: 'var(--space-xs)' }}>
            <button type="button" onClick={onClose} className="btn-secondary">Cancel</button>
            <button type="submit" disabled={submitting} className="btn-primary">
              {submitting ? 'Transferring\u2026' : 'Transfer'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
