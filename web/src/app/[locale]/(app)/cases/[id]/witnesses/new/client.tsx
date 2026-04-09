'use client';

import { useRouter } from 'next/navigation';
import { WitnessForm, type WitnessFormData } from '@/components/witnesses/witness-form';
import type { EvidenceItem } from '@/types';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

export function NewWitnessClient({
  caseId,
  evidence,
  accessToken,
}: {
  caseId: string;
  evidence: EvidenceItem[];
  accessToken: string;
}) {
  const router = useRouter();

  const handleSave = async (data: WitnessFormData) => {
    const res = await fetch(`${API_BASE}/api/cases/${caseId}/witnesses`, {
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

    router.push(`/en/cases/${caseId}?tab=witnesses`);
    router.refresh();
  };

  const handleCancel = () => {
    router.push(`/en/cases/${caseId}?tab=witnesses`);
  };

  return (
    <WitnessForm
      caseId={caseId}
      evidence={evidence}
      accessToken={accessToken}
      onSave={handleSave}
      onCancel={handleCancel}
    />
  );
}
