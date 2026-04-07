import { getServerSession } from 'next-auth';
import { redirect, notFound } from 'next/navigation';
import { authOptions } from '@/lib/auth';
import { authenticatedFetch } from '@/lib/api';
import { Shell } from '@/components/layout/shell';
import { EvidenceDetail } from '@/components/evidence/evidence-detail';
import type { EvidenceItem } from '@/types';

interface CaseData {
  id: string;
  reference_code: string;
  title: string;
}

export default async function EvidenceDetailPage({
  params,
}: {
  params: { id: string };
}) {
  const session = await getServerSession(authOptions);
  if (!session) redirect('/en/login');

  const res = await authenticatedFetch<EvidenceItem>(
    `/api/evidence/${params.id}`
  );

  if (res.error) {
    if (res.error === 'not found') notFound();
    return (
      <Shell>
        <div className="max-w-4xl mx-auto px-[var(--space-lg)] py-[var(--space-xl)]">
          <div className="banner-error">{res.error}</div>
        </div>
      </Shell>
    );
  }

  if (!res.data) notFound();

  const evidence = res.data;

  // Fetch case data for breadcrumb
  const caseRes = await authenticatedFetch<CaseData>(
    `/api/cases/${evidence.case_id}`
  );
  const caseData = caseRes.data;

  const canEdit =
    session.user.systemRole === 'system_admin' ||
    session.user.systemRole === 'case_admin';

  return (
    <Shell>
      <div className="max-w-4xl mx-auto px-[var(--space-lg)] py-[var(--space-xl)]">
        {/* Breadcrumb */}
        <div className="flex items-center gap-[var(--space-xs)] mb-[var(--space-lg)]">
          <a
            href="/en/cases"
            className="link-subtle text-xs uppercase tracking-wider font-medium"
          >
            Cases
          </a>
          <span
            className="text-xs"
            style={{ color: 'var(--text-tertiary)' }}
          >
            /
          </span>
          <a
            href={`/en/cases/${evidence.case_id}`}
            className="link-subtle text-xs uppercase tracking-wider font-medium"
          >
            {caseData?.reference_code || 'Case'}
          </a>
          <span
            className="text-xs"
            style={{ color: 'var(--text-tertiary)' }}
          >
            /
          </span>
          <a
            href={`/en/cases/${evidence.case_id}/evidence`}
            className="link-subtle text-xs uppercase tracking-wider font-medium"
          >
            Evidence
          </a>
          <span
            className="text-xs"
            style={{ color: 'var(--text-tertiary)' }}
          >
            /
          </span>
          <span
            className="text-xs uppercase tracking-wider font-medium"
            style={{ color: 'var(--text-primary)' }}
          >
            {evidence.evidence_number}
          </span>
        </div>

        <EvidenceDetail evidence={evidence} canEdit={canEdit} />
      </div>
    </Shell>
  );
}
