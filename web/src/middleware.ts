import { type NextRequest, NextResponse } from 'next/server';
import createMiddleware from 'next-intl/middleware';
import { routing } from './i18n/routing';

const intlMiddleware = createMiddleware(routing);

function generateNonce(): string {
  const array = new Uint8Array(16);
  crypto.getRandomValues(array);
  return btoa(String.fromCharCode(...Array.from(array)));
}

export default function middleware(request: NextRequest): NextResponse {
  const nonce = generateNonce();
  const isDev = process.env.NODE_ENV === 'development';
  const csp = isDev
    ? '' // CSP disabled in development — Next.js HMR requires unsafe-eval
    : [
        "default-src 'self'",
        `script-src 'self' 'nonce-${nonce}'`,
        "style-src 'self' 'unsafe-inline'",
        "img-src 'self' data: blob:",
        "connect-src 'self' wss:",
        "font-src 'self'",
        "frame-ancestors 'none'",
        "object-src 'none'",
        "base-uri 'self'",
      ].join('; ');

  // Set the nonce on the request headers so server components can read it
  // via headers().get('x-nonce'). This does NOT appear in the HTTP response.
  request.headers.set('x-nonce', nonce);

  // Run the i18n middleware with the augmented request.
  const response = intlMiddleware(request) as NextResponse;

  // Set CSP on the response — this is public and must contain the nonce
  // so the browser can enforce the policy.
  if (csp) {
    response.headers.set('Content-Security-Policy', csp);
  }

  return response;
}

export const config = {
  matcher: ['/((?!_next/static|_next/image|favicon.ico|design/|api/).*)'],
};
