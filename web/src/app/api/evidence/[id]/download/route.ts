import { NextRequest, NextResponse } from 'next/server';
import { getServerSession } from 'next-auth';
import { authOptions } from '@/lib/auth';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

export async function GET(
  _request: NextRequest,
  { params }: { params: { id: string } }
) {
  const session = await getServerSession(authOptions);

  if (!session?.accessToken) {
    return NextResponse.json({ error: 'Unauthorized' }, { status: 401 });
  }

  const upstream = await fetch(
    `${API_BASE}/api/evidence/${params.id}/download`,
    {
      headers: { Authorization: `Bearer ${session.accessToken}` },
    }
  );

  if (!upstream.ok) {
    return new NextResponse(upstream.body, {
      status: upstream.status,
      headers: {
        'Content-Type':
          upstream.headers.get('Content-Type') || 'application/octet-stream',
      },
    });
  }

  const headers = new Headers();
  const contentType = upstream.headers.get('Content-Type');
  const contentDisposition = upstream.headers.get('Content-Disposition');
  const contentLength = upstream.headers.get('Content-Length');

  if (contentType) headers.set('Content-Type', contentType);
  if (contentDisposition) headers.set('Content-Disposition', contentDisposition);
  if (contentLength) headers.set('Content-Length', contentLength);

  return new NextResponse(upstream.body, { status: 200, headers });
}
