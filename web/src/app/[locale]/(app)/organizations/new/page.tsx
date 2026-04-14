'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { useSession } from 'next-auth/react';
import { useOrg } from '@/hooks/use-org';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

export default function NewOrganizationPage() {
  const router = useRouter();
  const { data: session } = useSession();
  const { refresh } = useOrg();
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) {
      setError('Organization name is required');
      return;
    }

    setSubmitting(true);
    setError('');

    try {
      const res = await fetch(`${API_BASE}/api/organizations`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${session?.accessToken}`,
        },
        body: JSON.stringify({ name: name.trim(), description: description.trim() }),
      });

      if (!res.ok) {
        const body = await res.json().catch(() => null);
        setError(body?.error ?? 'Failed to create organization');
        return;
      }

      const body = await res.json();
      const org = body.data ?? body;
      await refresh();
      router.push(`/en/organizations/${org.id}`);
    } catch {
      setError('Network error');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div style={{ maxWidth: '32rem', marginInline: 'auto', padding: 'var(--space-xl) var(--space-lg)' }}>
      <h1
        className="font-[family-name:var(--font-heading)]"
        style={{ fontSize: 'var(--text-xl)', color: 'var(--text-primary)' }}
      >
        Create Organization
      </h1>
      <p style={{ marginTop: 'var(--space-xs)', fontSize: 'var(--text-sm)', color: 'var(--text-tertiary)' }}>
        Organizations group cases and team members together.
      </p>

      <form onSubmit={handleSubmit} style={{ marginTop: 'var(--space-xl)' }} className="space-y-[var(--space-md)]">
        <div>
          <label htmlFor="org-name" className="field-label">Name</label>
          <input
            id="org-name"
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="e.g. ICC Investigation Unit"
            className="input-field"
            autoFocus
          />
        </div>

        <div>
          <label htmlFor="org-desc" className="field-label">Description</label>
          <textarea
            id="org-desc"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            rows={3}
            placeholder="Optional description"
            className="input-field resize-y"
          />
        </div>

        {error && <div className="banner-error">{error}</div>}

        <div className="flex items-center" style={{ gap: 'var(--space-sm)' }}>
          <button type="submit" disabled={submitting} className="btn-primary">
            {submitting ? 'Creating\u2026' : 'Create Organization'}
          </button>
          <button type="button" onClick={() => router.back()} className="btn-ghost">
            Cancel
          </button>
        </div>
      </form>
    </div>
  );
}
