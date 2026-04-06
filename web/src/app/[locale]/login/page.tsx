'use client';

import { signIn } from 'next-auth/react';
import { useSearchParams } from 'next/navigation';

function isSafeCallbackUrl(url: string): boolean {
  if (!url.startsWith('/')) return false;
  try {
    const parsed = new URL(url, 'http://localhost');
    return parsed.hostname === 'localhost';
  } catch {
    return false;
  }
}

const ERROR_MESSAGES: Record<string, string> = {
  OAuthSignin: 'Unable to start sign in. Please try again.',
  OAuthCallback: 'Authentication callback failed. Please try again.',
  OAuthAccountNotLinked: 'This account is linked to another provider.',
  SessionRequired: 'Please sign in to access this page.',
  Default: 'An authentication error occurred. Please try again.',
};

export default function LoginPage() {
  const searchParams = useSearchParams();
  const error = searchParams.get('error');
  const raw = searchParams.get('callbackUrl') || '';
  const callbackUrl = isSafeCallbackUrl(raw) ? raw : '/en/cases';

  const errorMessage = error ? ERROR_MESSAGES[error] || ERROR_MESSAGES.Default : null;

  return (
    <main className="flex min-h-screen items-center justify-center bg-zinc-50">
      <div className="w-full max-w-md space-y-6 rounded-lg border bg-white p-8 shadow-sm">
        <div className="space-y-2 text-center">
          <h1 className="text-2xl font-bold tracking-tight">VaultKeeper</h1>
          <p className="text-sm text-zinc-500">
            Sovereign evidence management platform
          </p>
        </div>

        {errorMessage && (
          <div className="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            {errorMessage}
          </div>
        )}

        <button
          onClick={() => signIn('keycloak', { callbackUrl })}
          className="w-full rounded-md bg-zinc-900 px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-zinc-800 focus:outline-none focus:ring-2 focus:ring-zinc-900 focus:ring-offset-2"
          type="button"
        >
          Sign in with Keycloak
        </button>
      </div>
    </main>
  );
}
