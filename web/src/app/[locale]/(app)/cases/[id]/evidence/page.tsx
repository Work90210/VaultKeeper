import { redirect } from 'next/navigation';

export default function EvidenceListPage({
  params,
}: {
  params: { id: string };
}) {
  redirect(`/en/cases/${params.id}?tab=evidence`);
}
