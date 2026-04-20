import { getServerSession } from 'next-auth';
import { redirect, notFound } from 'next/navigation';
import { authOptions } from '@/lib/auth';
import { authenticatedFetch } from '@/lib/api';
import { Shell } from '@/components/layout/shell';
import { ReportDetail } from '@/components/investigation/report-detail';
import type { InvestigationReport } from '@/types';

export default async function ReportDetailPage({
  params,
}: {
  params: { id: string };
}) {
  const session = await getServerSession(authOptions);
  if (!session) redirect('/en/login');

  const res = await authenticatedFetch<InvestigationReport>(`/api/reports/${params.id}`);

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
      <ReportDetail
        report={res.data}
        accessToken={session.accessToken as string}
      />
    </Shell>
  );
}
