import { NextRequest, NextResponse } from 'next/server';
import { getServerSession } from 'next-auth';
import { authOptions } from '@/lib/auth';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

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
    `${API_BASE}/api/evidence/${id}/download`,
    {
      headers: { Authorization: `Bearer ${session.accessToken}` },
    }
  );

  if (!upstream.ok) {
    return NextResponse.json({ error: 'Download failed' }, { status: upstream.status });
  }

  const SAFE_TYPES = new Set([
    'application/pdf',
    'image/jpeg',
    'image/png',
    'image/gif',
    'image/webp',
    'video/mp4',
    'audio/mpeg',
    'application/zip',
  ]);

  const headers = new Headers();
  const rawType = upstream.headers.get('Content-Type') || '';
  const mimeOnly = rawType.split(';')[0].trim();
  const contentDisposition = upstream.headers.get('Content-Disposition');
  const contentLength = upstream.headers.get('Content-Length');

  headers.set('Content-Type', SAFE_TYPES.has(mimeOnly) ? rawType : 'application/octet-stream');
  headers.set('X-Content-Type-Options', 'nosniff');
  // Force attachment disposition to prevent inline execution in browser
  if (contentDisposition && contentDisposition.startsWith('attachment')) {
    headers.set('Content-Disposition', contentDisposition);
  } else {
    const filename = contentDisposition?.match(/filename[^;=\n]*=((['"]).*?\2|[^;\n]*)/)?.[1]?.replace(/['"]/g, '') ?? 'download';
    headers.set('Content-Disposition', `attachment; filename="${filename}"`);
  }
  if (contentLength) headers.set('Content-Length', contentLength);

  return new NextResponse(upstream.body, { status: 200, headers });
}
