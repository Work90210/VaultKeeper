import { getServerSession } from 'next-auth';
import { redirect } from 'next/navigation';
import { authOptions } from '@/lib/auth';
import { authenticatedFetch } from '@/lib/api';
import { Shell } from '@/components/layout/shell';
import { EvidencePageClient } from '@/components/evidence/evidence-page-client';
import type { EvidenceItem, CaseRole } from '@/types';

interface CaseData {
  id: string;
  reference_code: string;
  title: string;
}

const UPLOAD_ROLES = new Set(['investigator', 'prosecutor', 'case_admin']);

export default async function EvidenceListPage({
  params,
  searchParams,
}: {
  params: { id: string };
  searchParams: { q?: string; classification?: string; cursor?: string };
}) {
  const session = await getServerSession(authOptions);
  if (!session) redirect('/en/login');

  const [caseRes, evidenceRes, rolesRes] = await Promise.all([
    authenticatedFetch<CaseData>(`/api/cases/${params.id}`),
    authenticatedFetch<EvidenceItem[]>(
      `/api/cases/${params.id}/evidence${buildQuery(searchParams)}`
    ),
    authenticatedFetch<CaseRole[]>(`/api/cases/${params.id}/roles`),
  ]);

  const caseData = caseRes.data;
  const evidence = evidenceRes.data || [];
  const total = evidenceRes.meta?.total || 0;
  const nextCursor = evidenceRes.meta?.next_cursor || '';
  const hasMore = evidenceRes.meta?.has_more || false;
  const roles = rolesRes.data || [];
  const error = evidenceRes.error || caseRes.error;

  const isSystemAdmin = session.user.systemRole === 'system_admin';
  const isCaseAdmin = session.user.systemRole === 'case_admin';
  const hasUploadRole = roles.some(
    (r) =>
      r.user_id === session.user.id && UPLOAD_ROLES.has(r.role)
  );
  const canUpload = isSystemAdmin || isCaseAdmin || hasUploadRole;

  return (
    <Shell>
      <div className="max-w-6xl mx-auto px-[var(--space-lg)] py-[var(--space-xl)]">
        {/* Breadcrumb */}
        <nav className="flex items-center gap-[var(--space-xs)] mb-[var(--space-md)]">
          <a href="/en/cases" className="link-subtle text-xs uppercase tracking-wider font-medium">
            Cases
          </a>
          <span className="text-xs" style={{ color: 'var(--text-tertiary)' }}>/</span>
          <a
            href={`/en/cases/${params.id}`}
            className="link-subtle text-xs uppercase tracking-wider font-medium"
          >
            {caseData?.reference_code || 'Case'}
          </a>
          <span className="text-xs" style={{ color: 'var(--text-tertiary)' }}>/</span>
          <span className="text-xs uppercase tracking-wider font-medium" style={{ color: 'var(--text-primary)' }}>
            Evidence
          </span>
        </nav>

        {/* Page header */}
        <div className="flex items-end justify-between mb-[var(--space-lg)]">
          <div>
            <h1
              className="font-[family-name:var(--font-heading)] text-2xl"
              style={{ color: 'var(--text-primary)' }}
            >
              Evidence
            </h1>
            <p className="text-sm mt-[var(--space-xs)]" style={{ color: 'var(--text-tertiary)' }}>
              {total} {total === 1 ? 'item' : 'items'}
            </p>
          </div>
        </div>

        {error && <div className="banner-error mb-[var(--space-md)]">{error}</div>}

        <EvidencePageClient
          caseId={params.id}
          accessToken={session.accessToken as string}
          canUpload={canUpload}
          evidence={evidence}
          nextCursor={nextCursor}
          hasMore={hasMore}
          currentQuery={searchParams.q || ''}
          currentClassification={searchParams.classification || ''}
        />
      </div>
    </Shell>
  );
}

function buildQuery(searchParams: {
  q?: string;
  classification?: string;
  cursor?: string;
}): string {
  const params = new URLSearchParams();
  params.set('current_only', 'true');
  if (searchParams.q) params.set('q', searchParams.q);
  if (searchParams.classification) params.set('classification', searchParams.classification);
  if (searchParams.cursor) params.set('cursor', searchParams.cursor);
  return `?${params.toString()}`;
}
