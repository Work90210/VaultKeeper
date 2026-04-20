import { NextRequest, NextResponse } from 'next/server';
import { getServerSession } from 'next-auth';
import { authOptions } from '@/lib/auth';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

// Only allow image content types for thumbnails.
const SAFE_IMAGE_TYPES = new Set([
  'image/jpeg',
  'image/png',
  'image/gif',
  'image/webp',
  'image/avif',
]);

export async function GET(
  _request: NextRequest,
  { params }: { params: { id: string } }
) {
  const { id } = params;

  if (!UUID_RE.test(id)) {
    return NextResponse.json({ error: 'Invalid evidence ID' }, { status: 400 });
  }

  const session = await getServerSession(authOptions);

  if (!session?.accessToken) {
    return NextResponse.json({ error: 'Unauthorized' }, { status: 401 });
  }

  const upstream = await fetch(
    `${API_BASE}/api/evidence/${id}/thumbnail`,
    {
      headers: { Authorization: `Bearer ${session.accessToken}` },
    }
  );

  if (!upstream.ok) {
    return new NextResponse(upstream.body, {
      status: upstream.status,
      headers: {
        'Content-Type': 'application/octet-stream',
        'X-Content-Type-Options': 'nosniff',
      },
    });
  }

  const upstreamContentType = upstream.headers.get('Content-Type') || '';
  // Strip parameters (e.g. "image/jpeg; charset=utf-8") for the allowlist check.
  const mimeType = upstreamContentType.split(';')[0].trim().toLowerCase();

  if (!SAFE_IMAGE_TYPES.has(mimeType)) {
    return NextResponse.json({ error: 'Invalid thumbnail type' }, { status: 415 });
  }

  const responseHeaders = new Headers();
  responseHeaders.set('Content-Type', mimeType);
  responseHeaders.set('X-Content-Type-Options', 'nosniff');

  const contentLength = upstream.headers.get('Content-Length');
  const cacheControl = upstream.headers.get('Cache-Control');

  if (contentLength) responseHeaders.set('Content-Length', contentLength);
  if (cacheControl) responseHeaders.set('Cache-Control', cacheControl);

  return new NextResponse(upstream.body, { status: 200, headers: responseHeaders });
}
