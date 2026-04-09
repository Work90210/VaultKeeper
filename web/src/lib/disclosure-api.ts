import { authenticatedFetch, type ApiResponse } from '@/lib/api';
import type { Disclosure } from '@/types';

export interface CreateDisclosureData {
  evidence_ids: string[];
  disclosed_to: string;
  notes: string;
  redacted: boolean;
}

export interface DisclosureListParams {
  cursor?: string;
  limit?: number;
}

export async function createDisclosure(
  caseId: string,
  data: CreateDisclosureData
): Promise<ApiResponse<Disclosure>> {
  return authenticatedFetch<Disclosure>(`/api/cases/${caseId}/disclosures`, {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function listDisclosures(
  caseId: string,
  params?: DisclosureListParams
): Promise<ApiResponse<Disclosure[]>> {
  const query = new URLSearchParams();
  if (params?.cursor) query.set('cursor', params.cursor);
  if (params?.limit) query.set('limit', String(params.limit));

  const qs = query.toString();
  const path = `/api/cases/${caseId}/disclosures${qs ? `?${qs}` : ''}`;
  return authenticatedFetch<Disclosure[]>(path);
}

export async function getDisclosure(
  id: string
): Promise<ApiResponse<Disclosure>> {
  return authenticatedFetch<Disclosure>(`/api/disclosures/${id}`);
}
