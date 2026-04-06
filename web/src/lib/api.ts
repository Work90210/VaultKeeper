import { getServerSession } from 'next-auth';
import { redirect } from 'next/navigation';
import { authOptions } from '@/lib/auth';

export interface ApiResponse<T> {
  data: T | null;
  error: string | null;
  meta?: {
    total: number;
    next_cursor: string;
    has_more: boolean;
  };
}

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

export async function api<T>(
  path: string,
  options?: RequestInit
): Promise<ApiResponse<T>> {
  const url = `${API_BASE}${path}`;
  const headers: HeadersInit = {
    'Content-Type': 'application/json',
    ...options?.headers,
  };

  const response = await fetch(url, { ...options, headers });

  if (!response.ok) {
    const body = await response.json().catch(() => null);
    return {
      data: null,
      error: body?.error || `Request failed with status ${response.status}`,
    };
  }

  return response.json();
}

export async function authenticatedFetch<T>(
  path: string,
  options?: RequestInit
): Promise<ApiResponse<T>> {
  const session = await getServerSession(authOptions);

  if (!session?.accessToken || session.error === 'RefreshAccessTokenError') {
    redirect('/en/login');
  }

  return api<T>(path, {
    ...options,
    headers: {
      ...options?.headers,
      Authorization: `Bearer ${session.accessToken}`,
    },
  });
}
