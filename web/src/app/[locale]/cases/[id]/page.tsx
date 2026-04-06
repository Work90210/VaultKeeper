import { getServerSession } from 'next-auth';
import { redirect, notFound } from 'next/navigation';
import { authOptions } from '@/lib/auth';
import { authenticatedFetch } from '@/lib/api';
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
    if (res.error === 'not found') {
      notFound();
    }
    return (
      <main className="container mx-auto px-6 py-8">
        <div className="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          {res.error}
        </div>
      </main>
    );
  }

  if (!res.data) notFound();

  return (
    <main className="container mx-auto px-6 py-8">
      <CaseDetail
        caseData={res.data}
        canEdit={
          session.user.systemRole === 'system_admin' ||
          session.user.systemRole === 'case_admin'
        }
      />
    </main>
  );
}
