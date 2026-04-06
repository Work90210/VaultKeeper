import { getServerSession } from 'next-auth';
import { redirect, notFound } from 'next/navigation';
import { authOptions } from '@/lib/auth';
import { authenticatedFetch } from '@/lib/api';
import { Shell } from '@/components/layout/shell';
import { CaseSettings } from '@/components/cases/case-settings';

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

export default async function CaseSettingsPage({
  params,
}: {
  params: { id: string };
}) {
  const session = await getServerSession(authOptions);
  if (!session) redirect('/en/login');

  if (
    session.user.systemRole !== 'system_admin' &&
    session.user.systemRole !== 'case_admin'
  ) {
    redirect(`/en/cases/${params.id}`);
  }

  const res = await authenticatedFetch<CaseData>(`/api/cases/${params.id}`);
  if (res.error || !res.data) notFound();

  return (
    <Shell>
      <div className="max-w-xl mx-auto px-[var(--space-lg)] py-[var(--space-xl)]">
        <CaseSettings caseData={res.data} accessToken={session.accessToken || ''} />
      </div>
    </Shell>
  );
}
