import { getServerSession } from 'next-auth';
import { redirect, notFound } from 'next/navigation';
import { authOptions } from '@/lib/auth';
import { authenticatedFetch } from '@/lib/api';
import { Shell } from '@/components/layout/shell';
import { DisclosureDetailClient } from './client';
import type { Disclosure, EvidenceItem } from '@/types';

export default async function DisclosureDetailPage({
  params,
}: {
  params: { id: string };
}) {
  const session = await getServerSession(authOptions);
  if (!session) redirect('/en/login');

  const res = await authenticatedFetch<Disclosure>(`/api/disclosures/${params.id}`);

  if (res.error === 'not found' || !res.data) notFound();

  if (res.error) {
    return (
      <Shell>
        <div className="max-w-5xl mx-auto px-[var(--space-lg)] py-[var(--space-xl)]">
          <div className="banner-error">{res.error}</div>
        </div>
      </Shell>
    );
  }

  // Fetch evidence items for the case to show in the detail view
  const evidenceRes = await authenticatedFetch<EvidenceItem[]>(
    `/api/cases/${res.data.case_id}/evidence`
  );
  const evidence = evidenceRes.data || [];

  return (
    <Shell>
      <div className="max-w-5xl mx-auto px-[var(--space-lg)] py-[var(--space-xl)]">
        <DisclosureDetailClient
          disclosure={res.data}
          evidence={evidence}
        />
      </div>
    </Shell>
  );
}
