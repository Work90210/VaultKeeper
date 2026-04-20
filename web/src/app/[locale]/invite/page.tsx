'use client';

import { useState, useEffect } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { useSession, signIn } from 'next-auth/react';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

export default function InvitePage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { data: session, status: sessionStatus } = useSession();
  const token = searchParams.get('token');

  const [pageStatus, setPageStatus] = useState<'loading' | 'ready' | 'accepting' | 'accepted' | 'error'>('loading');
  const [error, setError] = useState('');

  // Basic token format validation: must be a non-empty alphanumeric+hyphen string.
  const isTokenValid = token !== null && /^[A-Za-z0-9_-]{8,256}$/.test(token);

  // If not logged in, redirect to sign in with callback back to this page
  useEffect(() => {
    if (sessionStatus === 'loading') return;
    if (sessionStatus === 'unauthenticated') {
      if (isTokenValid) {
        signIn('keycloak', { callbackUrl: `/en/invite?token=${encodeURIComponent(token!)}` });
      }
      return;
    }
    setPageStatus('ready');
  }, [sessionStatus, token, isTokenValid]);

  if (!token || !isTokenValid) {
    return (
      <div
        className="flex min-h-screen items-center justify-center"
        style={{ backgroundColor: 'var(--bg-primary)' }}
      >
        <div className="card" style={{ padding: 'var(--space-xl)', maxWidth: '28rem', width: '100%' }}>
          <h1
            className="font-[family-name:var(--font-heading)]"
            style={{ fontSize: 'var(--text-lg)', color: 'var(--text-primary)' }}
          >
            Invalid Invitation
          </h1>
          <p style={{ marginTop: 'var(--space-sm)', fontSize: 'var(--text-sm)', color: 'var(--text-tertiary)' }}>
            This invitation link is invalid or has expired. Please ask the organization admin to send a new invitation.
          </p>
        </div>
      </div>
    );
  }

  async function handleAccept() {
    setPageStatus('accepting');
    setError('');
    try {
      const res = await fetch(`${API_BASE}/api/invitations/accept`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(session?.accessToken ? { Authorization: `Bearer ${session.accessToken}` } : {}),
        },
        body: JSON.stringify({ token }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => null);
        setError(body?.error ?? 'Failed to accept invitation');
        setPageStatus('error');
        return;
      }
      setPageStatus('accepted');
      setTimeout(() => router.push('/en/cases'), 1500);
    } catch {
      setError('Network error. Please try again.');
      setPageStatus('error');
    }
  }

  async function handleDecline() {
    try {
      await fetch(`${API_BASE}/api/invitations/decline`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(session?.accessToken ? { Authorization: `Bearer ${session.accessToken}` } : {}),
        },
        body: JSON.stringify({ token }),
      });
    } catch { /* empty */ }
    router.push('/en/cases');
  }

  if (pageStatus === 'loading' || sessionStatus === 'loading') {
    return (
      <div
        className="flex min-h-screen items-center justify-center"
        style={{ backgroundColor: 'var(--bg-primary)' }}
      >
        <div className="flex flex-col items-center gap-4">
          <div
            className="w-8 h-8 rounded-full border-2 border-t-transparent animate-spin"
            style={{ borderColor: 'var(--amber-accent)', borderTopColor: 'transparent' }}
          />
          <p className="text-sm" style={{ color: 'var(--text-tertiary)' }}>
            Preparing your invitation...
          </p>
        </div>
      </div>
    );
  }

  return (
    <div
      className="flex min-h-screen items-center justify-center px-[var(--space-lg)]"
      style={{ backgroundColor: 'var(--bg-primary)' }}
    >
      <div style={{ maxWidth: '28rem', width: '100%' }}>
        {/* Logo */}
        <div className="flex items-center gap-[var(--space-sm)] mb-[var(--space-lg)]">
          <svg width="24" height="30" viewBox="0 0 22 26" fill="none" aria-hidden="true">
            <path
              d="M11 1L2 5v7c0 6.075 3.75 10.35 9 12 5.25-1.65 9-5.925 9-12V5L11 1z"
              stroke="var(--amber-accent)" strokeWidth="1.5" fill="none"
            />
            <path
              d="M8 12l2.5 2.5L15 9"
              stroke="var(--amber-accent)" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"
            />
          </svg>
          <span
            className="font-[family-name:var(--font-heading)] text-xl"
            style={{ color: 'var(--text-primary)' }}
          >
            VaultKeeper
          </span>
        </div>

        <div className="card" style={{ padding: 'var(--space-xl)' }}>
          {pageStatus === 'accepted' ? (
            <>
              <div className="flex items-center gap-[var(--space-sm)]" style={{ marginBottom: 'var(--space-md)' }}>
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="var(--status-active)" strokeWidth="2">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <h1
                  className="font-[family-name:var(--font-heading)]"
                  style={{ fontSize: 'var(--text-lg)', color: 'var(--text-primary)' }}
                >
                  Welcome aboard
                </h1>
              </div>
              <p style={{ fontSize: 'var(--text-sm)', color: 'var(--text-tertiary)' }}>
                You&apos;ve joined the organization. Redirecting to your cases...
              </p>
            </>
          ) : (
            <>
              <h1
                className="font-[family-name:var(--font-heading)]"
                style={{ fontSize: 'var(--text-lg)', color: 'var(--text-primary)' }}
              >
                You&apos;ve been invited
              </h1>
              <p style={{ marginTop: 'var(--space-xs)', fontSize: 'var(--text-sm)', color: 'var(--text-tertiary)' }}>
                You&apos;ve been invited to join an organization on VaultKeeper.
                {session?.user?.email && (
                  <> Signed in as <strong style={{ color: 'var(--text-secondary)' }}>{session.user.email}</strong>.</>
                )}
              </p>

              {error && <div className="banner-error" style={{ marginTop: 'var(--space-md)' }}>{error}</div>}

              <div className="flex" style={{ gap: 'var(--space-sm)', marginTop: 'var(--space-lg)' }}>
                <button
                  onClick={handleAccept}
                  disabled={pageStatus === 'accepting'}
                  className="btn-primary flex-1"
                  type="button"
                >
                  {pageStatus === 'accepting' ? 'Joining...' : 'Accept & Join'}
                </button>
                <button onClick={handleDecline} className="btn-secondary flex-1" type="button">
                  Decline
                </button>
              </div>
            </>
          )}
        </div>

        <p
          className="mt-[var(--space-lg)] text-xs text-center"
          style={{ color: 'var(--text-tertiary)' }}
        >
          Sovereign evidence management for legal investigations
        </p>
      </div>
    </div>
  );
}
