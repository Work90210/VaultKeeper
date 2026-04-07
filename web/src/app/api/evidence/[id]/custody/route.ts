import { NextRequest, NextResponse } from 'next/server';
import { getServerSession } from 'next-auth';
import { authOptions } from '@/lib/auth';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

export async function GET(
  request: NextRequest,
  { params }: { params: { id: string } }
) {
  const session = await getServerSession(authOptions);

  if (!session?.accessToken) {
    return NextResponse.json({ error: 'Unauthorized' }, { status: 401 });
  }

  const searchParams = request.nextUrl.searchParams;
  const qs = searchParams.toString();
  const url = `${API_BASE}/api/evidence/${params.id}/custody${qs ? `?${qs}` : ''}`;

  const upstream = await fetch(url, {
    headers: { Authorization: `Bearer ${session.accessToken}` },
  });

  const data = await upstream.json();
  return NextResponse.json(data, { status: upstream.status });
}
