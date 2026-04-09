'use client';

import { useRouter } from 'next/navigation';
import { DisclosureDetail } from '@/components/disclosures/disclosure-detail';
import type { Disclosure, EvidenceItem } from '@/types';

export function DisclosureDetailClient({
  disclosure,
  evidence,
}: {
  disclosure: Disclosure;
  evidence: EvidenceItem[];
}) {
  const router = useRouter();

  return (
    <DisclosureDetail
      disclosure={disclosure}
      evidence={evidence}
      onBack={() => router.push(`/en/cases/${disclosure.case_id}?tab=disclosures`)}
    />
  );
}
