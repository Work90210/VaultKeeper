import { getServerSession } from 'next-auth';
import { redirect, notFound } from 'next/navigation';
import { authOptions } from '@/lib/auth';
import { authenticatedFetch } from '@/lib/api';
import { Shell } from '@/components/layout/shell';
import { CaseDetail } from '@/components/cases/case-detail';

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

export default async function CaseDetailPage({
  params,
}: {
  params: { id: string };
}) {
  const session = await getServerSession(authOptions);
  if (!session) redirect('/en/login');

  const res = await authenticatedFetch<CaseData>(`/api/cases/${params.id}`);

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

  return (
    <Shell>
      <div className="max-w-4xl mx-auto px-[var(--space-lg)] py-[var(--space-xl)]">
        <a
          href="/en/cases"
          className="link-subtle text-[var(--text-xs)] uppercase tracking-wider font-medium mb-[var(--space-lg)] inline-block"
        >
          &larr; All cases
        </a>

        <CaseDetail
          caseData={res.data}
          canEdit={
            session.user.systemRole === 'system_admin' ||
            session.user.systemRole === 'case_admin'
          }
        />
      </div>
    </Shell>
  );
}
