import type { ApiResponse } from './api';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

export interface SearchHit {
  evidence_id: string;
  case_id: string;
  title: string;
  description: string;
  evidence_number: string;
  file_name?: string;
  highlights: Record<string, string[]>;
  score: number;
  mime_type?: string;
  classification?: string;
  uploaded_at?: string;
}

export interface SearchResultData {
  hits: SearchHit[];
  total_hits: number;
  query: string;
  processing_time_ms: number;
  facets?: Record<string, Record<string, number>>;
}

export interface SearchParams {
  q?: string;
  case_id?: string;
  type?: string;
  tag?: string;
  classification?: string;
  from?: string;
  to?: string;
  limit?: number;
  offset?: number;
}

export async function searchEvidence(
  params: SearchParams,
  token: string
): Promise<ApiResponse<SearchResultData>> {
  const searchParams = new URLSearchParams();
  if (params.q) searchParams.set('q', params.q);
  if (params.case_id) searchParams.set('case_id', params.case_id);
  if (params.type) searchParams.set('type', params.type);
  if (params.tag) searchParams.set('tag', params.tag);
  if (params.classification) searchParams.set('classification', params.classification);
  if (params.from) searchParams.set('from', params.from);
  if (params.to) searchParams.set('to', params.to);
  searchParams.set('limit', String(params.limit ?? 50));
  searchParams.set('offset', String(params.offset ?? 0));

  const url = `${API_BASE}/api/search?${searchParams.toString()}`;
  const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
  const headers: Record<string, string> = { Authorization: `Bearer ${token}` };
  const orgMatch = document.cookie.match(/(?:^|; )vk-active-org=([^;]*)/);
  if (orgMatch) {
    const orgId = decodeURIComponent(orgMatch[1]);
    if (UUID_RE.test(orgId)) {
      headers['X-Organization-ID'] = orgId;
    }
  }

  const response = await fetch(url, { headers });

  if (!response.ok) {
    const body = await response.json().catch(() => null);
    return { data: null, error: body?.error || `Search failed (${response.status})` };
  }

  return response.json();
}
