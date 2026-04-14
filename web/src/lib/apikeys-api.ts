import type { ApiResponse } from './api';
import type { ApiKey } from '@/types';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

interface CreateKeyResult {
  key: ApiKey;
  raw_key: string;
}

async function apiKeysRequest<T>(
  path: string,
  token: string,
  options?: RequestInit
): Promise<ApiResponse<T>> {
  const url = `${API_BASE}${path}`;
  const response = await fetch(url, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
      ...options?.headers,
    },
  });

  if (!response.ok) {
    const body = await response.json().catch(() => null);
    return { data: null, error: body?.error || `Request failed (${response.status})` };
  }

  return response.json();
}

export async function listApiKeys(token: string): Promise<ApiResponse<ApiKey[]>> {
  return apiKeysRequest('/api/settings/api-keys', token);
}

export async function createApiKey(
  token: string,
  name: string,
  permissions: 'read' | 'read_write'
): Promise<ApiResponse<CreateKeyResult>> {
  return apiKeysRequest('/api/settings/api-keys', token, {
    method: 'POST',
    body: JSON.stringify({ name, permissions }),
  });
}

export async function revokeApiKey(
  token: string,
  id: string
): Promise<ApiResponse<{ status: string }>> {
  return apiKeysRequest(`/api/settings/api-keys/${id}`, token, {
    method: 'DELETE',
  });
}
