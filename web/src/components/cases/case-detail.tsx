'use client';

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

export function CaseDetail({
  caseData,
  canEdit,
}: {
  caseData: CaseData;
  canEdit: boolean;
}) {
  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between">
        <div>
          <p className="text-sm font-mono text-zinc-500">{caseData.reference_code}</p>
          <h1 className="mt-1 text-2xl font-bold tracking-tight">{caseData.title}</h1>
        </div>
        <div className="flex items-center gap-2">
          {caseData.legal_hold && (
            <span className="rounded-full bg-red-100 px-3 py-1 text-xs font-medium text-red-700">
              Legal Hold
            </span>
          )}
          <StatusBadge status={caseData.status} />
        </div>
      </div>

      <div className="grid gap-6 md:grid-cols-2">
        <div className="space-y-4 rounded-md border p-4">
          <h2 className="font-semibold">Details</h2>
          <dl className="space-y-2 text-sm">
            <div>
              <dt className="text-zinc-500">Jurisdiction</dt>
              <dd>{caseData.jurisdiction || 'Not specified'}</dd>
            </div>
            <div>
              <dt className="text-zinc-500">Created</dt>
              <dd>{new Date(caseData.created_at).toLocaleString()}</dd>
            </div>
            <div>
              <dt className="text-zinc-500">Last Updated</dt>
              <dd>{new Date(caseData.updated_at).toLocaleString()}</dd>
            </div>
          </dl>
        </div>

        {caseData.description && (
          <div className="space-y-2 rounded-md border p-4">
            <h2 className="font-semibold">Description</h2>
            <p className="text-sm text-zinc-600 whitespace-pre-wrap">{caseData.description}</p>
          </div>
        )}
      </div>

      {canEdit && (
        <div className="flex gap-3 border-t pt-4">
          <a
            href={`/en/cases/${caseData.id}/settings`}
            className="rounded-md border px-4 py-2 text-sm hover:bg-zinc-50"
          >
            Settings
          </a>
        </div>
      )}
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const styles: Record<string, string> = {
    active: 'bg-green-100 text-green-700',
    closed: 'bg-yellow-100 text-yellow-700',
    archived: 'bg-zinc-100 text-zinc-600',
  };

  return (
    <span
      className={`rounded-full px-3 py-1 text-xs font-medium ${styles[status] || 'bg-zinc-100 text-zinc-600'}`}
    >
      {status}
    </span>
  );
}
