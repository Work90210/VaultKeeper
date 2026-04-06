import { getServerSession } from 'next-auth';
import { redirect } from 'next/navigation';
import { authOptions } from '@/lib/auth';
import { authenticatedFetch, type ApiResponse } from '@/lib/api';
import { CaseList } from '@/components/cases/case-list';

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

export default async function CasesPage({
  searchParams,
}: {
  searchParams: { q?: string; status?: string; cursor?: string };
}) {
  const session = await getServerSession(authOptions);
  if (!session) redirect('/en/login');

  const params = new URLSearchParams();
  if (searchParams.q) params.set('q', searchParams.q);
  if (searchParams.status) params.set('status', searchParams.status);
  if (searchParams.cursor) params.set('cursor', searchParams.cursor);

  const query = params.toString();
  const path = `/api/cases${query ? `?${query}` : ''}`;

  let cases: CaseData[] = [];
  let total = 0;
  let nextCursor = '';
  let hasMore = false;
  let error: string | null = null;

  try {
    const res: ApiResponse<CaseData[]> = await authenticatedFetch(path);
    if (res.data) {
      cases = res.data;
    }
    if (res.meta) {
      total = res.meta.total;
      nextCursor = res.meta.next_cursor;
      hasMore = res.meta.has_more;
    }
    if (res.error) {
      error = res.error;
    }
  } catch {
    error = 'Failed to load cases';
  }

  return (
    <main className="container mx-auto px-6 py-8">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Cases</h1>
          <p className="text-sm text-zinc-500">{total} total</p>
        </div>
        {session.user.systemRole === 'system_admin' ||
        session.user.systemRole === 'case_admin' ? (
          <a
            href="/en/cases/new"
            className="rounded-md bg-zinc-900 px-4 py-2 text-sm font-medium text-white hover:bg-zinc-800"
          >
            Create Case
          </a>
        ) : null}
      </div>

      {error && (
        <div className="mb-4 rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          {error}
        </div>
      )}

      <CaseList
        cases={cases}
        nextCursor={nextCursor}
        hasMore={hasMore}
        currentQuery={searchParams.q || ''}
        currentStatus={searchParams.status || ''}
      />
    </main>
  );
}
