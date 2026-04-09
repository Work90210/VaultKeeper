import { getServerSession } from 'next-auth';
import { redirect, notFound } from 'next/navigation';
import { authOptions } from '@/lib/auth';
import { authenticatedFetch } from '@/lib/api';
import { Shell } from '@/components/layout/shell';
import { WitnessDetailClient } from './client';
import type { Witness, EvidenceItem } from '@/types';

export default async function WitnessDetailPage({
  params,
}: {
  params: { id: string };
}) {
  const session = await getServerSession(authOptions);
  if (!session) redirect('/en/login');

  const res = await authenticatedFetch<Witness>(`/api/witnesses/${params.id}`);

  if (res.error === 'not found') notFound();

  if (res.error || !res.data) {
    return (
      <Shell>
        <div className="max-w-5xl mx-auto px-[var(--space-lg)] py-[var(--space-xl)]">
          <div className="banner-error">{res.error || 'Failed to load witness'}</div>
        </div>
      </Shell>
    );
  }

  const isSystemAdmin = session.user.systemRole === 'system_admin';
  const isCaseAdmin = session.user.systemRole === 'case_admin';
  const canEdit = isSystemAdmin || isCaseAdmin;

  // Fetch case evidence for the edit form
  let evidence: EvidenceItem[] = [];
  if (canEdit) {
    const evRes = await authenticatedFetch<EvidenceItem[]>(
      `/api/cases/${res.data.case_id}/evidence?current_only=true`
    );
    if (evRes.data) evidence = evRes.data;
  }

  return (
    <Shell>
      <div className="max-w-5xl mx-auto px-[var(--space-lg)] py-[var(--space-xl)]">
        <WitnessDetailClient
          witness={res.data}
          canEdit={canEdit}
          evidence={evidence}
          accessToken={session.accessToken}
        />
      </div>
    </Shell>
  );
}
