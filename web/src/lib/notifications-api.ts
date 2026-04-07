import type { ApiResponse } from './api';
import type { Notification } from '@/types';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

async function notificationsRequest<T>(
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

export async function listNotifications(
  token: string,
  limit = 20,
  cursor?: string
): Promise<ApiResponse<Notification[]>> {
  const params = new URLSearchParams({ limit: String(limit) });
  if (cursor) params.set('cursor', cursor);
  return notificationsRequest(`/api/notifications?${params}`, token);
}

export async function getUnreadCount(
  token: string
): Promise<ApiResponse<{ count: number }>> {
  return notificationsRequest('/api/notifications/unread-count', token);
}

export async function markRead(
  token: string,
  id: string
): Promise<ApiResponse<null>> {
  return notificationsRequest(`/api/notifications/${id}/read`, token, {
    method: 'PATCH',
  });
}

export async function markAllRead(
  token: string
): Promise<ApiResponse<null>> {
  return notificationsRequest('/api/notifications/read-all', token, {
    method: 'POST',
  });
}
