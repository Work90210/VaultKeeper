import { getServerSession } from 'next-auth';
import { redirect, notFound } from 'next/navigation';
import { authOptions } from '@/lib/auth';
import { authenticatedFetch } from '@/lib/api';
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
    <main className="container mx-auto max-w-2xl px-6 py-8">
      <CaseSettings caseData={res.data} accessToken={session.accessToken || ''} />
    </main>
  );
}
