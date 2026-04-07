import { getServerSession } from 'next-auth';
import { redirect, notFound } from 'next/navigation';
import { authOptions } from '@/lib/auth';
import { authenticatedFetch } from '@/lib/api';
import { Shell } from '@/components/layout/shell';
import { CaseDetail } from '@/components/cases/case-detail';
import type { EvidenceItem, CaseRole } from '@/types';

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

const UPLOAD_ROLES = new Set(['investigator', 'prosecutor', 'case_admin']);

export default async function CaseDetailPage({
  params,
  searchParams,
}: {
  params: { id: string };
  searchParams: { tab?: string };
}) {
  const session = await getServerSession(authOptions);
  if (!session) redirect('/en/login');

  const [caseRes, evidenceRes, rolesRes] = await Promise.all([
    authenticatedFetch<CaseData>(`/api/cases/${params.id}`),
    authenticatedFetch<EvidenceItem[]>(`/api/cases/${params.id}/evidence`),
    authenticatedFetch<CaseRole[]>(`/api/cases/${params.id}/roles`),
  ]);

  if (caseRes.error === 'not found' || !caseRes.data) notFound();

  if (caseRes.error) {
    return (
      <Shell>
        <div className="max-w-5xl mx-auto px-[var(--space-lg)] py-[var(--space-xl)]">
          <div className="banner-error">{caseRes.error}</div>
        </div>
      </Shell>
    );
  }

  const evidence = evidenceRes.data || [];
  const evidenceTotal = evidenceRes.meta?.total || 0;
  const evidenceNextCursor = evidenceRes.meta?.next_cursor || '';
  const evidenceHasMore = evidenceRes.meta?.has_more || false;
  const roles = rolesRes.data || [];

  const isSystemAdmin = session.user.systemRole === 'system_admin';
  const isCaseAdmin = session.user.systemRole === 'case_admin';
  const canEdit = isSystemAdmin || isCaseAdmin;
  const hasUploadRole = roles.some(
    (r) => r.user_id === session.user.id && UPLOAD_ROLES.has(r.role)
  );
  const caseIsActive = caseRes.data.status === 'active';
  const canUpload = caseIsActive && (isSystemAdmin || isCaseAdmin || hasUploadRole);

  const validTabs = ['overview', 'evidence', 'settings'] as const;
  const tab = validTabs.includes(searchParams.tab as typeof validTabs[number])
    ? (searchParams.tab as typeof validTabs[number])
    : undefined;

  return (
    <Shell>
      <div className="max-w-5xl mx-auto px-[var(--space-lg)] py-[var(--space-xl)]">
        <nav className="flex items-center gap-[var(--space-xs)] mb-[var(--space-md)]">
          <a
            href="/en/cases"
            className="link-subtle text-xs uppercase tracking-wider font-medium"
          >
            Cases
          </a>
          <span className="text-xs" style={{ color: 'var(--text-tertiary)' }}>/</span>
          <span
            className="text-xs uppercase tracking-wider font-medium font-[family-name:var(--font-mono)]"
            style={{ color: 'var(--text-primary)' }}
          >
            {caseRes.data.reference_code}
          </span>
        </nav>

        <CaseDetail
          caseData={caseRes.data}
          canEdit={canEdit}
          accessToken={session.accessToken as string}
          evidence={evidence}
          evidenceTotal={evidenceTotal}
          evidenceNextCursor={evidenceNextCursor}
          evidenceHasMore={evidenceHasMore}
          canUpload={canUpload}
          initialTab={tab}
        />
      </div>
    </Shell>
  );
}
