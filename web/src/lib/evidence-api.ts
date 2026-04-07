import { authenticatedFetch, type ApiResponse } from '@/lib/api';
import type { EvidenceItem } from '@/types';

export interface EvidenceListParams {
  q?: string;
  classification?: string;
  cursor?: string;
}

export async function listEvidence(
  caseId: string,
  params?: EvidenceListParams
): Promise<ApiResponse<EvidenceItem[]>> {
  const query = new URLSearchParams();
  if (params?.q) query.set('q', params.q);
  if (params?.classification) query.set('classification', params.classification);
  if (params?.cursor) query.set('cursor', params.cursor);

  const qs = query.toString();
  const path = `/api/cases/${caseId}/evidence${qs ? `?${qs}` : ''}`;
  return authenticatedFetch<EvidenceItem[]>(path);
}

export async function getEvidence(
  id: string
): Promise<ApiResponse<EvidenceItem>> {
  return authenticatedFetch<EvidenceItem>(`/api/evidence/${id}`);
}

export async function updateEvidence(
  id: string,
  data: {
    title?: string;
    description?: string;
    classification?: string;
    tags?: string[];
    source?: string;
    source_date?: string | null;
  }
): Promise<ApiResponse<EvidenceItem>> {
  return authenticatedFetch<EvidenceItem>(`/api/evidence/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(data),
  });
}

export async function destroyEvidence(
  id: string,
  authority: string
): Promise<ApiResponse<null>> {
  return authenticatedFetch<null>(`/api/evidence/${id}`, {
    method: 'DELETE',
    body: JSON.stringify({ authority }),
  });
}

export async function getEvidenceVersions(
  id: string
): Promise<ApiResponse<EvidenceItem[]>> {
  return authenticatedFetch<EvidenceItem[]>(`/api/evidence/${id}/versions`);
}
