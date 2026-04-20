import { getServerSession } from 'next-auth';
import { redirect, notFound } from 'next/navigation';
import { authOptions } from '@/lib/auth';
import { authenticatedFetch } from '@/lib/api';
import { Shell } from '@/components/layout/shell';
import { AssessmentDetail } from '@/components/investigation/assessment-detail';
import type { EvidenceAssessment } from '@/types';

export default async function AssessmentDetailPage({
  params,
}: {
  params: { id: string };
}) {
  const session = await getServerSession(authOptions);
  if (!session) redirect('/en/login');

  const res = await authenticatedFetch<EvidenceAssessment>(`/api/assessments/${params.id}`);

  if (res.error === 'not found' || !res.data) notFound();
  if (res.error) {
    return (
      <Shell>
        <div className="max-w-7xl mx-auto px-[var(--space-lg)] py-[var(--space-xl)]">
          <div className="banner-error">{res.error}</div>
        </div>
      </Shell>
    );
  }

  return (
    <Shell>
      <AssessmentDetail assessment={res.data} />
    </Shell>
  );
}
