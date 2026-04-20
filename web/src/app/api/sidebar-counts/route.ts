import { NextRequest, NextResponse } from 'next/server';
import { getServerSession } from 'next-auth';
import { authOptions } from '@/lib/auth';
import { authenticatedFetch, type ApiResponse } from '@/lib/api';

interface CaseSummary {
  id: string;
  reference_code: string;
  title: string;
  status: string;
  legal_hold: boolean;
}

function extractCount(res: PromiseSettledResult<ApiResponse<unknown[]>>): number {
  if (res.status !== 'fulfilled') return 0;
  try {
    const { meta, data } = res.value;
    if (meta?.total != null) return meta.total;
    if (Array.isArray(data)) return data.length;
  } catch { /* malformed response */ }
  return 0;
}

export async function GET(req: NextRequest) {
  const session = await getServerSession(authOptions);
  if (!session) {
    return NextResponse.json({ counts: {}, cases: [] }, { status: 401 });
  }

  const caseId = req.nextUrl.searchParams.get('caseId');

  // Case-scoped counts
  if (caseId) {
    const counts: Record<string, number> = {};
    const endpoints: [string, string][] = [
      ['evidence', `/api/cases/${caseId}/evidence?limit=0`],
      ['witnesses', `/api/cases/${caseId}/witnesses?limit=0`],
      ['disclosures', `/api/cases/${caseId}/disclosures?limit=0`],
      ['corroborations', `/api/cases/${caseId}/corroborations?limit=0`],
      ['assessments', `/api/cases/${caseId}/assessments?limit=0`],
      ['reports', `/api/cases/${caseId}/reports?limit=0`],
    ];
    const results = await Promise.allSettled(
      endpoints.map(([, url]) => authenticatedFetch<unknown[]>(url))
    );
    for (let i = 0; i < endpoints.length; i++) {
      const n = extractCount(results[i]);
      if (n > 0) counts[endpoints[i][0]] = n;
    }
    return NextResponse.json({ counts, cases: [] });
  }

  // Global counts (all cases)
  const counts: Record<string, number> = {};
  let caseList: CaseSummary[] = [];

  try {
    const casesRes: ApiResponse<CaseSummary[]> = await authenticatedFetch('/api/cases?limit=100');
    const cases = casesRes.data ?? [];
    caseList = cases.map((c) => ({
      id: c.id,
      reference_code: c.reference_code,
      title: c.title,
      status: c.legal_hold ? 'hold' : c.status,
      legal_hold: c.legal_hold,
    }));
    counts.cases = casesRes.meta?.total ?? cases.length;

    // Sum counts across all cases using the same endpoints as case-scoped view
    let evidenceTotal = 0;
    let witnessTotal = 0;
    let disclosureTotal = 0;
    let corrobTotal = 0;
    let assessTotal = 0;
    let reportTotal = 0;

    const results = await Promise.allSettled(
      cases.map(async (c) => {
        const [evRes, wRes, dRes, corrRes, assessRes, repRes] = await Promise.allSettled([
          authenticatedFetch<unknown[]>(`/api/cases/${c.id}/evidence?limit=0`),
          authenticatedFetch<unknown[]>(`/api/cases/${c.id}/witnesses?limit=0`),
          authenticatedFetch<unknown[]>(`/api/cases/${c.id}/disclosures?limit=0`),
          authenticatedFetch<unknown[]>(`/api/cases/${c.id}/corroborations?limit=0`),
          authenticatedFetch<unknown[]>(`/api/cases/${c.id}/assessments?limit=0`),
          authenticatedFetch<unknown[]>(`/api/cases/${c.id}/reports?limit=0`),
        ]);
        return {
          evidence: extractCount(evRes),
          witnesses: extractCount(wRes),
          disclosures: extractCount(dRes),
          corroborations: extractCount(corrRes),
          assessments: extractCount(assessRes),
          reports: extractCount(repRes),
        };
      })
    );

    for (const r of results) {
      if (r.status === 'fulfilled') {
        evidenceTotal += r.value.evidence;
        witnessTotal += r.value.witnesses;
        disclosureTotal += r.value.disclosures;
        corrobTotal += r.value.corroborations;
        assessTotal += r.value.assessments;
        reportTotal += r.value.reports;
      }
    }

    if (evidenceTotal > 0) counts.evidence = evidenceTotal;
    if (witnessTotal > 0) counts.witnesses = witnessTotal;
    if (disclosureTotal > 0) counts.disclosures = disclosureTotal;
    if (corrobTotal > 0) counts.corroborations = corrobTotal;
    if (assessTotal > 0) counts.assessments = assessTotal;
    if (reportTotal > 0) counts.reports = reportTotal;
  } catch {}

  return NextResponse.json({ counts, cases: caseList });
}
