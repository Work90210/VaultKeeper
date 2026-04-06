'use client';

import { useRouter, useSearchParams } from 'next/navigation';
import { useState } from 'react';

interface CaseItem {
  id: string;
  reference_code: string;
  title: string;
  status: string;
  legal_hold: boolean;
  jurisdiction: string;
  created_at: string;
}

export function CaseList({
  cases,
  nextCursor,
  hasMore,
  currentQuery,
  currentStatus,
}: {
  cases: CaseItem[];
  nextCursor: string;
  hasMore: boolean;
  currentQuery: string;
  currentStatus: string;
}) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [search, setSearch] = useState(currentQuery);

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    const params = new URLSearchParams(searchParams.toString());
    if (search) {
      params.set('q', search);
    } else {
      params.delete('q');
    }
    params.delete('cursor');
    router.push(`/en/cases?${params.toString()}`);
  };

  const handleStatusFilter = (status: string) => {
    const params = new URLSearchParams(searchParams.toString());
    if (status) {
      params.set('status', status);
    } else {
      params.delete('status');
    }
    params.delete('cursor');
    router.push(`/en/cases?${params.toString()}`);
  };

  return (
    <div className="space-y-4">
      <div className="flex gap-3">
        <form onSubmit={handleSearch} className="flex flex-1 gap-2">
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search cases..."
            className="flex-1 rounded-md border px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-zinc-900"
          />
          <button
            type="submit"
            className="rounded-md border px-3 py-2 text-sm hover:bg-zinc-50"
          >
            Search
          </button>
        </form>
        <select
          value={currentStatus}
          onChange={(e) => handleStatusFilter(e.target.value)}
          className="rounded-md border px-3 py-2 text-sm"
        >
          <option value="">All statuses</option>
          <option value="active">Active</option>
          <option value="closed">Closed</option>
          <option value="archived">Archived</option>
        </select>
      </div>

      {cases.length === 0 ? (
        <div className="rounded-md border border-dashed p-12 text-center">
          <p className="text-sm text-zinc-500">No cases found</p>
        </div>
      ) : (
        <div className="overflow-hidden rounded-md border">
          <table className="w-full text-sm">
            <thead className="border-b bg-zinc-50">
              <tr>
                <th className="px-4 py-3 text-left font-medium">Reference</th>
                <th className="px-4 py-3 text-left font-medium">Title</th>
                <th className="px-4 py-3 text-left font-medium">Status</th>
                <th className="px-4 py-3 text-left font-medium">Jurisdiction</th>
                <th className="px-4 py-3 text-left font-medium">Created</th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {cases.map((c) => (
                <tr
                  key={c.id}
                  className="cursor-pointer hover:bg-zinc-50"
                  onClick={() => router.push(`/en/cases/${c.id}`)}
                >
                  <td className="px-4 py-3 font-mono text-xs">
                    {c.reference_code}
                  </td>
                  <td className="px-4 py-3">
                    <span>{c.title}</span>
                    {c.legal_hold && (
                      <span className="ml-2 rounded-full bg-red-100 px-2 py-0.5 text-xs font-medium text-red-700">
                        Legal Hold
                      </span>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <StatusBadge status={c.status} />
                  </td>
                  <td className="px-4 py-3 text-zinc-500">{c.jurisdiction}</td>
                  <td className="px-4 py-3 text-zinc-500">
                    {new Date(c.created_at).toLocaleDateString()}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {hasMore && (
        <div className="flex justify-center pt-2">
          <a
            href={`/en/cases?${new URLSearchParams({
              ...(currentQuery ? { q: currentQuery } : {}),
              ...(currentStatus ? { status: currentStatus } : {}),
              cursor: nextCursor,
            }).toString()}`}
            className="rounded-md border px-4 py-2 text-sm hover:bg-zinc-50"
          >
            Load more
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
      className={`rounded-full px-2 py-0.5 text-xs font-medium ${styles[status] || 'bg-zinc-100 text-zinc-600'}`}
    >
      {status}
    </span>
  );
}
