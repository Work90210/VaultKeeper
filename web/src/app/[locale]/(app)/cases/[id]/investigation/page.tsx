import { getServerSession } from 'next-auth';
import { redirect } from 'next/navigation';
import { authOptions } from '@/lib/auth';
import { authenticatedFetch } from '@/lib/api';
import { Shell } from '@/components/layout/shell';
import { InvestigationPageClient } from '@/components/investigation/investigation-page-client';
import type {
  InquiryLog,
  CorroborationClaim,
  AnalysisNote,
  InvestigationTemplate,
  TemplateInstance,
  InvestigationReport,
  SafetyProfile,
} from '@/types';

interface CaseData {
  id: string;
  reference_code: string;
  title: string;
}

const VALID_TABS = [
  'inquiry-logs',
  'assessments',
  'verifications',
  'corroborations',
  'analysis',
  'templates',
  'reports',
  'safety',
] as const;

type TabKey = (typeof VALID_TABS)[number];

export default async function InvestigationPage({
  params,
  searchParams,
}: {
  params: { id: string };
  searchParams: { tab?: string };
}) {
  const session = await getServerSession(authOptions);
  if (!session) redirect('/en/login');

  const tab: TabKey = VALID_TABS.includes(searchParams.tab as TabKey)
    ? (searchParams.tab as TabKey)
    : 'inquiry-logs';

  // Fetch case data first, then investigation data.
  // Assessments and verifications are evidence-scoped (not case-scoped),
  // so we pass empty arrays — they're loaded client-side per evidence item.
  const [
    caseRes,
    inquiryRes,
    corroborationRes,
    analysisRes,
    templateRes,
    instanceRes,
    reportRes,
    safetyRes,
  ] = await Promise.all([
    authenticatedFetch<CaseData>(`/api/cases/${params.id}`),
    authenticatedFetch<InquiryLog[]>(`/api/cases/${params.id}/inquiry-logs`),
    authenticatedFetch<CorroborationClaim[]>(`/api/cases/${params.id}/corroborations`),
    authenticatedFetch<AnalysisNote[]>(`/api/cases/${params.id}/analysis-notes`),
    authenticatedFetch<InvestigationTemplate[]>(`/api/templates`),
    authenticatedFetch<TemplateInstance[]>(`/api/cases/${params.id}/template-instances`),
    authenticatedFetch<InvestigationReport[]>(`/api/cases/${params.id}/reports`),
    authenticatedFetch<SafetyProfile[]>(`/api/cases/${params.id}/safety-profiles`),
  ]);

  const caseData = caseRes.data;
  // Only show error if the case itself failed to load
  const error = caseRes.error;

  return (
    <Shell>
      <div className="max-w-6xl mx-auto px-[var(--space-lg)] py-[var(--space-xl)]">
        {/* Breadcrumb */}
        <nav className="flex items-center gap-[var(--space-xs)] mb-[var(--space-md)]">
          <a
            href="/en/cases"
            className="link-subtle text-xs uppercase tracking-wider font-medium"
          >
            Cases
          </a>
          <span className="text-xs" style={{ color: 'var(--text-tertiary)' }}>
            /
          </span>
          <a
            href={`/en/cases/${params.id}`}
            className="link-subtle text-xs uppercase tracking-wider font-medium"
          >
            {caseData?.reference_code || 'Case'}
          </a>
          <span className="text-xs" style={{ color: 'var(--text-tertiary)' }}>
            /
          </span>
          <span
            className="text-xs uppercase tracking-wider font-medium"
            style={{ color: 'var(--text-primary)' }}
          >
            Investigation
          </span>
        </nav>

        {/* Page header */}
        <div className="flex items-end justify-between mb-[var(--space-lg)]">
          <div>
            <h1
              className="font-[family-name:var(--font-heading)] text-2xl"
              style={{ color: 'var(--text-primary)' }}
            >
              Investigation
            </h1>
            <p
              className="text-sm mt-[var(--space-xs)]"
              style={{ color: 'var(--text-tertiary)' }}
            >
              Berkeley Protocol investigation workflow
            </p>
          </div>
        </div>

        {error && (
          <div className="banner-error mb-[var(--space-md)]">{error}</div>
        )}

        <InvestigationPageClient
          caseId={params.id}
          accessToken={session.accessToken as string}
          userId={session.user.id}
          activeSection={tab || 'inquiry-logs'}
          inquiryLogs={inquiryRes.data || []}
          assessments={[]}
          verifications={[]}
          corroborations={corroborationRes.data || []}
          analysisNotes={analysisRes.data || []}
          templates={templateRes.data || []}
          templateInstances={instanceRes.data || []}
          reports={reportRes.data || []}
          safetyProfiles={safetyRes.data || []}
        />
      </div>
    </Shell>
  );
}
