'use client';

import { useState } from 'react';
import { useRouter, useParams } from 'next/navigation';
import { WitnessDetail } from '@/components/witnesses/witness-detail';
import { WitnessForm, type WitnessFormData } from '@/components/witnesses/witness-form';
import type { Witness, EvidenceItem } from '@/types';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

export function WitnessDetailClient({
  witness,
  canEdit,
  evidence,
  accessToken,
}: {
  witness: Witness;
  canEdit: boolean;
  evidence: EvidenceItem[];
  accessToken?: string;
}) {
  const router = useRouter();
  const params = useParams();
  const locale = (params?.locale as string) || 'en';
  const [editing, setEditing] = useState(false);

  const handleSave = async (data: WitnessFormData) => {
    if (!accessToken) return;

    const res = await fetch(`${API_BASE}/api/witnesses/${witness.id}`, {
      method: 'PATCH',
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

    setEditing(false);
    router.refresh();
  };

  if (editing && accessToken) {
    return (
      <WitnessForm
        caseId={witness.case_id}
        witness={witness}
        evidence={evidence}
        accessToken={accessToken}
        onSave={handleSave}
        onCancel={() => setEditing(false)}
      />
    );
  }

  return (
    <WitnessDetail
      witness={witness}
      canEdit={canEdit}
      onBack={() => router.push(`/${locale}/cases/${witness.case_id}?tab=witnesses`)}
      onEdit={canEdit ? () => setEditing(true) : undefined}
    />
  );
}
