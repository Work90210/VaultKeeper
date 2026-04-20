import { authenticatedFetch, type ApiResponse } from '@/lib/api';

export interface FederationPeer {
  id: string;
  instance_id: string;
  display_name: string;
  well_known_url?: string;
  trust_mode: string;
  verified_by?: string;
  verified_at?: string;
  verification_channel?: string;
  org_id?: string;
  created_at: string;
  updated_at: string;
}

export interface FederationExchange {
  exchange_id: string;
  direction: string;
  peer_instance_id: string;
  peer_display_name: string;
  manifest_hash: string;
  scope_hash: string;
  merkle_root: string;
  scope_cardinality: number;
  status: string;
  created_at: string;
}

/**
 * Extract the payload from an authenticatedFetch response.
 *
 * The federation Go handler encodes arrays directly (not wrapped in the
 * standard {data,error,meta} envelope), so the parsed JSON may be the
 * raw array itself. This helper normalises both shapes.
 */
function unwrap<T>(res: ApiResponse<T> | T): { data: T | null; error: string | null } {
  if (Array.isArray(res)) {
    return { data: res as T, error: null };
  }
  const envelope = res as ApiResponse<T>;
  if (envelope && typeof envelope === 'object' && 'data' in envelope) {
    return { data: envelope.data, error: envelope.error };
  }
  return { data: res as T, error: null };
}

export async function listPeers(): Promise<FederationPeer[]> {
  const raw = await authenticatedFetch<FederationPeer[]>(
    '/api/federation/peers'
  );
  const { data, error } = unwrap(raw);
  if (error) {
    throw new Error(error);
  }
  return data ?? [];
}

export async function listExchanges(): Promise<FederationExchange[]> {
  // The Go handler requires case_id or peer_instance_id query param.
  // A dashboard-wide listing isn't supported yet, so we gracefully
  // degrade to an empty array when the backend rejects the request.
  try {
    const raw = await authenticatedFetch<FederationExchange[]>(
      '/api/federation/exchanges?case_id=00000000-0000-0000-0000-000000000000'
    );
    const { data, error } = unwrap(raw);
    if (error) {
      return [];
    }
    return data ?? [];
  } catch {
    return [];
  }
}
