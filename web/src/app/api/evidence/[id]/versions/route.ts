import { NextRequest, NextResponse } from 'next/server';
import { getServerSession } from 'next-auth';
import { authOptions } from '@/lib/auth';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export async function GET(
  request: NextRequest,
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

  const url = `${API_BASE}/api/evidence/${id}/versions`;

  const upstream = await fetch(url, {
    headers: { Authorization: `Bearer ${session.accessToken}` },
  });

  if (!upstream.ok) {
    return NextResponse.json({ error: 'Request failed' }, { status: upstream.status });
  }

  const data = await upstream.json().catch(() => null);
  if (!data) {
    return NextResponse.json({ error: 'Invalid response' }, { status: 502 });
  }

  return NextResponse.json(data, { status: upstream.status });
}
