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
        <div className="d-content">
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
      <EvidenceDetail
        evidence={evidence}
        canEdit={canEdit}
        accessToken={session.accessToken as string}
        username={session.user.name || session.user.email || 'User'}
        caseReferenceCode={caseData?.reference_code || 'Case'}
      />
    </Shell>
  );
}
