'use client';

import { useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { useOrg } from '@/hooks/use-org';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

export default function InvitePage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { refresh } = useOrg();
  const token = searchParams.get('token');

  const [status, setStatus] = useState<'ready' | 'accepting' | 'error'>('ready');
  const [error, setError] = useState('');

  if (!token) {
    return (
      <div className="flex items-center justify-center" style={{ height: '16rem', color: 'var(--text-tertiary)', fontSize: 'var(--text-sm)' }}>
        Invalid invitation link. No token provided.
      </div>
    );
  }

  async function handleAccept() {
    setStatus('accepting');
    setError('');
    try {
      const res = await fetch(`${API_BASE}/api/invitations/accept`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ token }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => null);
        setError(body?.error ?? 'Failed to accept invitation');
        setStatus('error');
        return;
      }
      const org = await res.json();
      await refresh();
      router.push(`/en/organizations/${org.id}`);
    } catch {
      setError('Network error');
      setStatus('error');
    }
  }

  async function handleDecline() {
    try {
      await fetch(`${API_BASE}/api/invitations/decline`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ token }),
      });
      router.push('/en/cases');
    } catch {
      setError('Failed to decline invitation');
    }
  }

  return (
    <div style={{ maxWidth: '28rem', marginInline: 'auto', padding: 'var(--space-2xl) var(--space-lg)' }}>
      <div className="card" style={{ padding: 'var(--space-xl)' }}>
        <h1
          className="font-[family-name:var(--font-heading)]"
          style={{ fontSize: 'var(--text-lg)', color: 'var(--text-primary)' }}
        >
          Organization Invitation
        </h1>
        <p style={{ marginTop: 'var(--space-xs)', fontSize: 'var(--text-sm)', color: 'var(--text-tertiary)' }}>
          You have been invited to join an organization.
        </p>

        {error && <div className="banner-error" style={{ marginTop: 'var(--space-md)' }}>{error}</div>}

        <div className="flex" style={{ gap: 'var(--space-sm)', marginTop: 'var(--space-lg)' }}>
          <button onClick={handleAccept} disabled={status === 'accepting'} className="btn-primary flex-1" type="button">
            {status === 'accepting' ? 'Accepting\u2026' : 'Accept'}
          </button>
          <button onClick={handleDecline} className="btn-secondary flex-1" type="button">
            Decline
          </button>
        </div>
      </div>
    </div>
  );
}
