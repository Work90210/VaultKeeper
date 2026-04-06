'use client';

import { signIn } from 'next-auth/react';
import { useSearchParams } from 'next/navigation';

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

  return (
    <div className="flex min-h-screen">
      {/* Left: brand panel */}
      <div
        className="hidden lg:flex lg:w-[45%] flex-col justify-between p-[var(--space-xl)]"
        style={{ backgroundColor: 'var(--stone-900)' }}
      >
        <div>
          <h1
            className="font-[family-name:var(--font-heading)] text-[var(--text-3xl)] leading-tight"
            style={{ color: 'var(--stone-100)' }}
          >
            VaultKeeper
          </h1>
          <p
            className="mt-[var(--space-xs)] text-[var(--text-sm)] tracking-wide uppercase"
            style={{ color: 'var(--stone-400)', letterSpacing: '0.1em' }}
          >
            Sovereign Evidence Management
          </p>
        </div>

        <div className="space-y-[var(--space-lg)]">
          <div style={{ borderLeft: '2px solid var(--amber-accent)', paddingLeft: 'var(--space-md)' }}>
            <p
              className="text-[var(--text-sm)] leading-relaxed"
              style={{ color: 'var(--stone-300)' }}
            >
              Tamper-evident chain of custody. Role-based access control.
              Cryptographic integrity verification for every piece of evidence.
            </p>
          </div>
          <p
            className="text-[var(--text-xs)]"
            style={{ color: 'var(--stone-500)' }}
          >
            All access is logged and auditable.
          </p>
        </div>
      </div>

      {/* Right: sign-in form */}
      <div className="flex flex-1 flex-col items-center justify-center px-[var(--space-lg)]">
        <div className="w-full max-w-sm">
          {/* Mobile-only brand mark */}
          <div className="lg:hidden mb-[var(--space-xl)]">
            <h1
              className="font-[family-name:var(--font-heading)] text-[var(--text-2xl)]"
              style={{ color: 'var(--text-primary)' }}
            >
              VaultKeeper
            </h1>
            <p className="text-[var(--text-xs)] uppercase tracking-widest" style={{ color: 'var(--text-tertiary)' }}>
              Evidence Management
            </p>
          </div>

          <h2
            className="text-[var(--text-lg)] font-semibold"
            style={{ color: 'var(--text-primary)' }}
          >
            Sign in
          </h2>
          <p
            className="mt-[var(--space-xs)] text-[var(--text-sm)]"
            style={{ color: 'var(--text-secondary)' }}
          >
            Authenticate via your organization's identity provider.
          </p>

          {errorMessage && (
            <div
              className="mt-[var(--space-md)] px-[var(--space-md)] py-[var(--space-sm)] text-[var(--text-sm)]"
              style={{
                backgroundColor: 'var(--status-hold-bg)',
                color: 'var(--status-hold)',
                borderLeft: '3px solid var(--status-hold)',
              }}
            >
              {errorMessage}
            </div>
          )}

          <button
            onClick={() => signIn('keycloak', { callbackUrl })}
            className="mt-[var(--space-lg)] w-full py-[var(--space-sm)] px-[var(--space-md)] text-[var(--text-sm)] font-medium transition-colors"
            style={{
              backgroundColor: 'var(--amber-accent)',
              color: 'var(--stone-950)',
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.backgroundColor = 'var(--amber-hover)';
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.backgroundColor = 'var(--amber-accent)';
            }}
            type="button"
          >
            Continue with Keycloak
          </button>

          <p
            className="mt-[var(--space-xl)] text-[var(--text-xs)] text-center"
            style={{ color: 'var(--text-tertiary)' }}
          >
            Access restricted to authorized personnel
          </p>
        </div>
      </div>
    </div>
  );
}
