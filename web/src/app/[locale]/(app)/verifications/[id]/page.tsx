import { getServerSession } from 'next-auth';
import { redirect, notFound } from 'next/navigation';
import { authOptions } from '@/lib/auth';
import { authenticatedFetch } from '@/lib/api';
import { Shell } from '@/components/layout/shell';
import { VerificationDetail } from '@/components/investigation/verification-detail';
import type { VerificationRecord } from '@/types';

export default async function VerificationDetailPage({
  params,
}: {
  params: { id: string };
}) {
  const session = await getServerSession(authOptions);
  if (!session) redirect('/en/login');

  const res = await authenticatedFetch<VerificationRecord>(`/api/verifications/${params.id}`);

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
      <VerificationDetail
        record={res.data}
        accessToken={session.accessToken as string}
      />
    </Shell>
  );
}
