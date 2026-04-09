import { getServerSession } from 'next-auth';
import { redirect } from 'next/navigation';
import { authOptions } from '@/lib/auth';
import { authenticatedFetch } from '@/lib/api';
import { Shell } from '@/components/layout/shell';
import { NewDisclosureClient } from './client';
import type { EvidenceItem } from '@/types';

export default async function NewDisclosurePage({
  params,
}: {
  params: { id: string };
}) {
  const session = await getServerSession(authOptions);
  if (!session) redirect('/en/login');

  const evidenceRes = await authenticatedFetch<EvidenceItem[]>(
    `/api/cases/${params.id}/evidence`
  );

  const evidence = evidenceRes.data || [];

  return (
    <Shell>
      <div className="max-w-5xl mx-auto px-[var(--space-lg)] py-[var(--space-xl)]">
        <nav className="flex items-center gap-[var(--space-xs)] mb-[var(--space-md)]">
          <a
            href={`/en/cases/${params.id}`}
            className="link-subtle text-xs uppercase tracking-wider font-medium"
          >
            Case
          </a>
          <span className="text-xs" style={{ color: 'var(--text-tertiary)' }}>/</span>
          <a
            href={`/en/cases/${params.id}?tab=disclosures`}
            className="link-subtle text-xs uppercase tracking-wider font-medium"
          >
            Disclosures
          </a>
          <span className="text-xs" style={{ color: 'var(--text-tertiary)' }}>/</span>
          <span
            className="text-xs uppercase tracking-wider font-medium"
            style={{ color: 'var(--text-primary)' }}
          >
            New
          </span>
        </nav>

        <h1
          className="font-[family-name:var(--font-heading)] text-2xl mb-[var(--space-lg)]"
          style={{ color: 'var(--text-primary)' }}
        >
          Create Disclosure
        </h1>

        <NewDisclosureClient
          caseId={params.id}
          evidence={evidence}
          accessToken={session.accessToken as string}
        />
      </div>
    </Shell>
  );
}
