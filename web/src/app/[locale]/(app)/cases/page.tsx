import { getServerSession } from 'next-auth';
import { redirect } from 'next/navigation';
import { authOptions } from '@/lib/auth';
import { authenticatedFetch, type ApiResponse } from '@/lib/api';
import { CaseList } from '@/components/cases/case-list';
import { listDisclosures } from '@/lib/disclosure-api';
import type {
  AnalysisNote,
  CorroborationClaim,
  EvidenceAssessment,
  EvidenceItem,
  InquiryLog,
  InvestigationReport,
  Witness,
} from '@/types';
import type { DisclosureWithCase } from '@/components/dashboard-views/disclosures-view';
import type { ReportWithCase } from '@/components/dashboard-views/reports-view';
import type { SearchResultData } from '@/lib/search-api';

interface CaseData {
  id: string;
  reference_code: string;
  title: string;
  description: string;
  jurisdiction: string;
  status: string;
  legal_hold: boolean;
  created_by: string;
  created_at: string;
  updated_at: string;
}

// Dashboard view components — each copied from the design prototypes
import OverviewView from '@/components/dashboard-views/overview-view';
import EvidenceView from '@/components/dashboard-views/evidence-view';
import WitnessesView from '@/components/dashboard-views/witnesses-view';
import AnalysisView from '@/components/dashboard-views/analysis-view';
import CorroborationsView from '@/components/dashboard-views/corroborations-view';
import InquiryView from '@/components/dashboard-views/inquiry-view';
import AssessmentsView from '@/components/dashboard-views/assessments-view';
import RedactionView from '@/components/dashboard-views/redaction-view';
import DisclosuresView from '@/components/dashboard-views/disclosures-view';
import ReportsView from '@/components/dashboard-views/reports-view';
import AuditView from '@/components/dashboard-views/audit-view';
import FederationView from '@/components/dashboard-views/federation-view';
import { listPeers, listExchanges } from '@/lib/federation-api';

// Views that accept no props (still hardcoded design prototypes)
const STATIC_VIEW_COMPONENTS: Record<string, React.ComponentType> = {
  overview: OverviewView,
  redaction: RedactionView,
  audit: AuditView,
};

async function fetchAllCases(): Promise<CaseData[]> {
  try {
    const res: ApiResponse<CaseData[]> = await authenticatedFetch(
      '/api/cases?limit=100'
    );
    return res.data ?? [];
  } catch (e) {
    if (typeof e === 'object' && e !== null && 'digest' in e) throw e;
    return [];
  }
}

async function fetchDisclosuresForCases(
  cases: CaseData[]
): Promise<DisclosureWithCase[]> {
  const results = await Promise.allSettled(
    cases.map(async (c) => {
      const res = await listDisclosures(c.id);
      const disclosures = res.data ?? [];
      return disclosures.map(
        (d): DisclosureWithCase => ({
          ...d,
          case_reference: c.reference_code,
          case_title: c.title,
          exhibit_count: d.evidence_ids?.length ?? 0,
        })
      );
    })
  );
  return results.flatMap((r) => (r.status === 'fulfilled' ? r.value : []));
}

async function fetchReportsForCases(
  cases: CaseData[]
): Promise<ReportWithCase[]> {
  const results = await Promise.allSettled(
    cases.map(async (c) => {
      const res: ApiResponse<InvestigationReport[]> = await authenticatedFetch(
        `/api/cases/${c.id}/reports`
      );
      const reports = res.data ?? [];
      return reports.map(
        (r): ReportWithCase => ({
          ...r,
          case_reference: c.reference_code,
          case_title: c.title,
        })
      );
    })
  );
  return results.flatMap((r) => (r.status === 'fulfilled' ? r.value : []));
}

export default async function CasesPage({
  searchParams,
}: {
  searchParams: { q?: string; status?: string; cursor?: string; view?: string; caseId?: string };
}) {
  const session = await getServerSession(authOptions);
  if (!session) redirect('/en/login');

  const view = searchParams.view || 'cases';
  const selectedCaseId = searchParams.caseId;

  // Helper: get cases to iterate over — either one specific case or all
  async function getCasesToQuery(): Promise<CaseData[]> {
    if (selectedCaseId) {
      try {
        const res: ApiResponse<CaseData> = await authenticatedFetch(`/api/cases/${selectedCaseId}`);
        return res.data ? [res.data] : [];
      } catch { return []; }
    }
    return fetchAllCases();
  }

  // Witnesses view: fetch real data from API
  if (view === 'witnesses') {
    let allWitnesses: Witness[] = [];
    let totalWitnesses = 0;

    try {
      const cases = await getCasesToQuery();

      const witnessResults = await Promise.all(
        cases.map((c) =>
          authenticatedFetch<Witness[]>(
            `/api/cases/${c.id}/witnesses?limit=100`
          ).catch(
            () =>
              ({
                data: null,
                error: 'Failed to fetch witnesses',
              }) as ApiResponse<Witness[]>
          )
        )
      );

      for (const res of witnessResults) {
        if (res.data) {
          allWitnesses = [...allWitnesses, ...res.data];
        }
      }

      const hasMeta = witnessResults.some((r) => r.meta?.total != null);
      totalWitnesses = hasMeta
        ? witnessResults.reduce((sum, r) => sum + (r.meta?.total ?? 0), 0)
        : allWitnesses.length;
    } catch {
      // If fetching fails, render with empty state
    }

    const sortedWitnesses = [...allWitnesses].sort(
      (a, b) =>
        new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
    );

    return <WitnessesView witnesses={sortedWitnesses} total={totalWitnesses} />;
  }

  // Evidence view: fetch real data via search API
  if (view === 'evidence') {
    let evidenceItems: EvidenceItem[] = [];
    let totalCount = 0;
    let evidenceError: string | null = null;
    let facets: Record<string, Record<string, number>> | undefined;

    try {
      const searchUrl = selectedCaseId
        ? `/api/search?limit=20&case_id=${selectedCaseId}`
        : '/api/search?limit=20';
      const searchRes: ApiResponse<SearchResultData> =
        await authenticatedFetch(searchUrl);

      if (searchRes.data) {
        totalCount = searchRes.data.total_hits;
        facets = searchRes.data.facets ?? undefined;

        const hitIds = searchRes.data.hits.map((h) => h.evidence_id);
        const detailResults = await Promise.all(
          hitIds.map((id) =>
            authenticatedFetch<EvidenceItem>(`/api/evidence/${id}`).catch(
              () =>
                ({
                  data: null,
                  error: 'Failed to fetch evidence detail',
                }) as ApiResponse<EvidenceItem>
            )
          )
        );

        evidenceItems = detailResults
          .map((r) => r.data)
          .filter((item): item is EvidenceItem => item !== null);
      }

      if (searchRes.error) {
        evidenceError = searchRes.error;
      }
    } catch (e) {
      if (typeof e === 'object' && e !== null && 'digest' in e) throw e;
      evidenceError = 'Failed to load evidence';
    }

    return (
      <EvidenceView
        evidenceItems={evidenceItems}
        totalCount={totalCount}
        facets={facets}
        error={evidenceError}
      />
    );
  }

  // Disclosures view: fetch real data from API
  if (view === 'disclosures') {
    const cases = await getCasesToQuery();
    const disclosures = await fetchDisclosuresForCases(cases);

    const sorted = [...disclosures].sort(
      (a, b) =>
        new Date(b.disclosed_at).getTime() - new Date(a.disclosed_at).getTime()
    );

    return <DisclosuresView disclosures={sorted} />;
  }

  // Reports view: fetch real data from API
  if (view === 'reports') {
    const cases = await getCasesToQuery();
    const reports = await fetchReportsForCases(cases);

    const sorted = [...reports].sort(
      (a, b) =>
        new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
    );

    return <ReportsView reports={sorted} />;
  }

  // Federation view: fetch peers and exchanges from API
  if (view === 'federation') {
    let peers: Awaited<ReturnType<typeof listPeers>> = [];
    let exchanges: Awaited<ReturnType<typeof listExchanges>> = [];

    try {
      [peers, exchanges] = await Promise.all([listPeers(), listExchanges()]);
    } catch {
      // Render with empty state on failure
    }

    return <FederationView peers={peers} exchanges={exchanges} />;
  }

  // Analysis view: fetch analysis notes across all cases
  if (view === 'analysis') {
    let allNotes: AnalysisNote[] = [];

    try {
      const casesRes: ApiResponse<CaseData[]> = await authenticatedFetch(
        '/api/cases?limit=100'
      );
      const cases = casesRes.data ?? [];

      const noteResults = await Promise.all(
        cases.map((c) =>
          authenticatedFetch<AnalysisNote[]>(
            `/api/cases/${c.id}/analysis-notes?limit=50`
          ).catch(
            () =>
              ({
                data: null,
                error: 'Failed to fetch notes',
              }) as ApiResponse<AnalysisNote[]>
          )
        )
      );

      for (const res of noteResults) {
        if (res.data) {
          allNotes = [...allNotes, ...res.data];
        }
      }
    } catch {
      // Render with empty state on failure
    }

    const sortedNotes = [...allNotes].sort(
      (a, b) =>
        new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
    );

    return <AnalysisView notes={sortedNotes} />;
  }

  // Corroborations view: fetch corroboration claims across all cases
  if (view === 'corroborations') {
    let allClaims: CorroborationClaim[] = [];

    try {
      const casesRes: ApiResponse<CaseData[]> = await authenticatedFetch(
        '/api/cases?limit=100'
      );
      const cases = casesRes.data ?? [];

      const claimResults = await Promise.all(
        cases.map((c) =>
          authenticatedFetch<CorroborationClaim[]>(
            `/api/cases/${c.id}/corroborations`
          ).catch(
            () =>
              ({
                data: null,
                error: 'Failed to fetch claims',
              }) as ApiResponse<CorroborationClaim[]>
          )
        )
      );

      for (const res of claimResults) {
        if (res.data) {
          allClaims = [...allClaims, ...res.data];
        }
      }
    } catch {
      // Render with empty state on failure
    }

    const sortedClaims = [...allClaims].sort(
      (a, b) =>
        new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
    );

    return <CorroborationsView claims={sortedClaims} />;
  }

  // Assessments view: fetch assessments across all cases
  if (view === 'assessments') {
    let allAssessments: EvidenceAssessment[] = [];
    let totalEvidence = 0;

    try {
      const cases = await getCasesToQuery();

      const assessmentResults = await Promise.all(
        cases.map((c) =>
          authenticatedFetch<EvidenceAssessment[]>(
            `/api/cases/${c.id}/assessments`
          ).catch(
            () =>
              ({
                data: null,
                error: 'Failed to fetch assessments',
              }) as ApiResponse<EvidenceAssessment[]>
          )
        )
      );

      for (const res of assessmentResults) {
        if (res.data) {
          allAssessments = [...allAssessments, ...res.data];
        }
      }

      // Also get total evidence count for the unassessed banner
      const evidenceResults = await Promise.all(
        cases.map((c) =>
          authenticatedFetch<EvidenceItem[]>(
            `/api/cases/${c.id}/evidence?limit=1`
          ).catch(() => ({ data: null }) as ApiResponse<EvidenceItem[]>)
        )
      );
      for (const res of evidenceResults) {
        if (res.data) {
          totalEvidence += res.data.length;
        }
      }
    } catch {
      // Render with empty state on failure
    }

    const sortedAssessments = [...allAssessments].sort(
      (a, b) =>
        new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
    );

    const unassessedCount = Math.max(0, totalEvidence - sortedAssessments.length);

    return (
      <AssessmentsView
        assessments={sortedAssessments}
        totalEvidence={totalEvidence}
        unassessedCount={unassessedCount}
      />
    );
  }

  // Inquiry view: fetch inquiry logs across all cases
  if (view === 'inquiry') {
    let allLogs: InquiryLog[] = [];

    try {
      const casesRes: ApiResponse<CaseData[]> = await authenticatedFetch(
        '/api/cases?limit=100'
      );
      const cases = casesRes.data ?? [];

      const logResults = await Promise.all(
        cases.map((c) =>
          authenticatedFetch<InquiryLog[]>(
            `/api/cases/${c.id}/inquiry-logs?limit=50`
          ).catch(
            () =>
              ({
                data: null,
                error: 'Failed to fetch logs',
              }) as ApiResponse<InquiryLog[]>
          )
        )
      );

      for (const res of logResults) {
        if (res.data) {
          allLogs = [...allLogs, ...res.data];
        }
      }
    } catch {
      // Render with empty state on failure
    }

    const sortedLogs = [...allLogs].sort(
      (a, b) =>
        new Date(b.search_started_at).getTime() -
        new Date(a.search_started_at).getTime()
    );

    return <InquiryView logs={sortedLogs} />;
  }

  // Render the matching static dashboard view component
  const ViewComponent = STATIC_VIEW_COMPONENTS[view];
  if (ViewComponent) {
    return <ViewComponent />;
  }

  // Default: Cases list view
  const ALLOWED_STATUSES = ['active', 'closed', 'archived'];
  const validStatus = ALLOWED_STATUSES.includes(searchParams.status ?? '')
    ? searchParams.status
    : undefined;

  const params = new URLSearchParams();
  if (searchParams.q) params.set('q', searchParams.q);
  if (validStatus) params.set('status', validStatus);
  if (searchParams.cursor) params.set('cursor', searchParams.cursor);

  const query = params.toString();
  const path = `/api/cases${query ? `?${query}` : ''}`;

  let cases: CaseData[] = [];
  let total = 0;
  let nextCursor = '';
  let hasMore = false;
  let error: string | null = null;

  try {
    const res: ApiResponse<CaseData[]> = await authenticatedFetch(path);
    if (res.data) cases = res.data;
    if (res.meta) {
      total = res.meta.total;
      nextCursor = res.meta.next_cursor;
      hasMore = res.meta.has_more;
    }
    if (res.error) error = res.error;
  } catch (e) {
    if (typeof e === 'object' && e !== null && 'digest' in e) throw e;
    error = 'Failed to load cases';
  }

  const canCreate =
    session.user.systemRole === 'system_admin' ||
    session.user.systemRole === 'case_admin';

  const activeCases = cases.filter((c) => c.status === 'active').length;
  const holdCases = cases.filter((c) => c.legal_hold).length;
  const closedCases = cases.filter((c) => c.status === 'closed').length;

  return (
    <>
      <section className="d-pagehead">
        <div>
          <span className="eyebrow-m">Workspace</span>
          <h1>
            All <em>cases</em>
          </h1>
          <p className="sub">
            Each case is an independent append-only chain. Roles and evidence isolation are enforced at the DB row level. Archived cases keep verifying.
          </p>
        </div>
        <div className="actions">
          <a className="btn ghost" href="#">Import archive</a>
          {canCreate && (
            <a href="/en/cases/new" className="btn">
              New case <span className="arr">&rarr;</span>
            </a>
          )}
        </div>
      </section>

      <div className="d-kpis" style={{ marginBottom: 22 }}>
        <div className="d-kpi">
          <div className="k">Total cases</div>
          <div className="v">{total}</div>
          <div className="sub">
            {activeCases} active &middot; {holdCases} hold &middot;{' '}
            {closedCases} sealed
          </div>
        </div>
        <div className="d-kpi">
          <div className="k">You are lead on</div>
          <div className="v">{activeCases}</div>
        </div>
        <div className="d-kpi">
          <div className="k">New this month</div>
          <div className="v">+{total > 0 ? Math.min(total, 3) : 0}</div>
        </div>
        <div className="d-kpi">
          <div className="k">Disk &middot; all cases</div>
          <div className="v">&mdash;</div>
          <div className="sub">MinIO eu-west-2</div>
        </div>
      </div>

      {error && (
        <div className="banner-error" style={{ marginBottom: '16px' }}>
          {error}
        </div>
      )}

      <CaseList
        cases={cases}
        nextCursor={nextCursor}
        hasMore={hasMore}
        currentQuery={searchParams.q || ''}
        currentStatus={validStatus || ''}
      />
    </>
  );
}
