'use client';

import { useRouter } from 'next/navigation';
import { DisclosureWizard, type DisclosureFormData } from '@/components/disclosures/disclosure-wizard';
import type { EvidenceItem } from '@/types';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

export function NewDisclosureClient({
  caseId,
  evidence,
  accessToken,
}: {
  caseId: string;
  evidence: EvidenceItem[];
  accessToken: string;
}) {
  const router = useRouter();

  const handleSubmit = async (data: DisclosureFormData) => {
    const res = await fetch(`${API_BASE}/api/cases/${caseId}/disclosures`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${accessToken}`,
      },
      body: JSON.stringify(data),
    });

    if (!res.ok) {
      const body = await res.json().catch(() => null);
      throw new Error(body?.error || `Failed with status ${res.status}`);
    }

    router.push(`/en/cases/${caseId}?tab=disclosures`);
    router.refresh();
  };

  const handleCancel = () => {
    router.push(`/en/cases/${caseId}?tab=disclosures`);
  };

  return (
    <DisclosureWizard
      caseId={caseId}
      evidence={evidence}
      onSubmit={handleSubmit}
      onCancel={handleCancel}
    />
  );
}
