import { authenticatedFetch, type ApiResponse } from '@/lib/api';
import type { Witness } from '@/types';

export interface WitnessListParams {
  cursor?: string;
  limit?: number;
}

export interface CreateWitnessData {
  witness_code: string;
  full_name?: string;
  contact_info?: string;
  location?: string;
  protection_status: string;
  statement_summary: string;
  related_evidence?: string[];
}

export interface UpdateWitnessData {
  full_name?: string;
  contact_info?: string;
  location?: string;
  protection_status?: string;
  statement_summary?: string;
  related_evidence?: string[];
  judge_identity_visible?: boolean;
}

export async function listWitnesses(
  caseId: string,
  params?: WitnessListParams
): Promise<ApiResponse<Witness[]>> {
  const query = new URLSearchParams();
  if (params?.cursor) query.set('cursor', params.cursor);
  if (params?.limit) query.set('limit', String(params.limit));

  const qs = query.toString();
  const path = `/api/cases/${caseId}/witnesses${qs ? `?${qs}` : ''}`;
  return authenticatedFetch<Witness[]>(path);
}

export async function getWitness(
  id: string
): Promise<ApiResponse<Witness>> {
  return authenticatedFetch<Witness>(`/api/witnesses/${id}`);
}

export async function createWitness(
  caseId: string,
  data: CreateWitnessData
): Promise<ApiResponse<Witness>> {
  return authenticatedFetch<Witness>(`/api/cases/${caseId}/witnesses`, {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function updateWitness(
  id: string,
  data: UpdateWitnessData
): Promise<ApiResponse<Witness>> {
  return authenticatedFetch<Witness>(`/api/witnesses/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(data),
  });
}
