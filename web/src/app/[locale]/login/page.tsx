'use client';

import { signIn } from 'next-auth/react';
import { useSearchParams } from 'next/navigation';
import { useEffect } from 'react';

const ERROR_MESSAGES: Record<string, string> = {
  OAuthSignin: 'Unable to start sign in. Please try again.',
  OAuthCallback: 'Authentication callback failed. Please try again.',
  OAuthAccountNotLinked: 'This account is linked to another provider.',
  SessionRequired: 'Please sign in to access this page.',
  Default: 'An authentication error occurred. Please try again.',
};

function isSafeCallbackUrl(url: string): boolean {
  if (!url.startsWith('/')) return false;
  try {
    const parsed = new URL(url, 'http://localhost');
    return parsed.hostname === 'localhost';
  } catch {
    return false;
  }
}

export default function LoginPage() {
  const searchParams = useSearchParams();
  const error = searchParams.get('error');
  const raw = searchParams.get('callbackUrl') || '';
  const callbackUrl = isSafeCallbackUrl(raw) ? raw : '/en/cases';

  const errorMessage = error
    ? ERROR_MESSAGES[error] || ERROR_MESSAGES.Default
    : null;

  // No error → skip this page entirely, go straight to Keycloak
  useEffect(() => {
    if (!error) {
      signIn('keycloak', { callbackUrl });
    }
  }, [error, callbackUrl]);

  // While redirecting (no error), show a minimal loading state
  if (!error) {
    return (
      <div
        className="flex min-h-screen items-center justify-center"
        style={{ backgroundColor: 'var(--bg-primary)' }}
      >
        <p className="text-[var(--text-sm)]" style={{ color: 'var(--text-tertiary)' }}>
          Redirecting to sign in...
        </p>
      </div>
    );
  }

  // Error state → show the error with a retry button
  return (
    <div
      className="flex min-h-screen items-center justify-center px-[var(--space-lg)]"
      style={{ backgroundColor: 'var(--bg-primary)' }}
    >
      <div className="w-full max-w-sm">
        <div className="flex items-center gap-[var(--space-sm)] mb-[var(--space-lg)]">
          <svg
            width="24"
            height="30"
            viewBox="0 0 22 26"
            fill="none"
            aria-hidden="true"
          >
            <path
              d="M11 1L2 5v7c0 6.075 3.75 10.35 9 12 5.25-1.65 9-5.925 9-12V5L11 1z"
              stroke="var(--amber-accent)"
              strokeWidth="1.5"
              fill="none"
            />
            <path
              d="M11 7v6m0 2.5v.5"
              stroke="var(--amber-accent)"
              strokeWidth="1.5"
              strokeLinecap="round"
            />
          </svg>
          <h1
            className="font-[family-name:var(--font-heading)] text-[var(--text-2xl)]"
            style={{ color: 'var(--text-primary)' }}
          >
            VaultKeeper
          </h1>
        </div>

        <div
          className="card p-[var(--space-xl)]"
          style={{ animation: 'fade-in var(--duration-slow) var(--ease-out-expo)' }}
        >
          <h2
            className="font-[family-name:var(--font-heading)] text-[var(--text-xl)]"
            style={{ color: 'var(--text-primary)' }}
          >
            Sign in failed
          </h2>

          <div className="banner-error mt-[var(--space-md)]">
            {errorMessage}
          </div>

          <button
            onClick={() => signIn('keycloak', { callbackUrl })}
            className="btn-primary w-full mt-[var(--space-lg)]"
            type="button"
          >
            Try again
          </button>
        </div>

        <p
          className="mt-[var(--space-lg)] text-[var(--text-xs)] text-center"
          style={{ color: 'var(--text-tertiary)' }}
        >
          Access restricted to authorized personnel
        </p>
      </div>
    </div>
  );
}
