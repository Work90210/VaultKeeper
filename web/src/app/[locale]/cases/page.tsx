import { getServerSession } from 'next-auth';
import { redirect } from 'next/navigation';
import { authOptions } from '@/lib/auth';
import { authenticatedFetch, type ApiResponse } from '@/lib/api';
import { Shell } from '@/components/layout/shell';
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
    if (res.data) cases = res.data;
    if (res.meta) {
      total = res.meta.total;
      nextCursor = res.meta.next_cursor;
      hasMore = res.meta.has_more;
    }
    if (res.error) error = res.error;
  } catch (e) {
    // Re-throw Next.js redirect errors (they use throw internally)
    if (typeof e === 'object' && e !== null && 'digest' in e) throw e;
    error = 'Failed to load cases';
  }

  const canCreate =
    session.user.systemRole === 'system_admin' ||
    session.user.systemRole === 'case_admin';

  return (
    <Shell>
      <div className="max-w-6xl mx-auto px-[var(--space-lg)] py-[var(--space-xl)]">
        {/* Page header */}
        <div className="flex items-end justify-between mb-[var(--space-lg)]">
          <div>
            <h1
              className="font-[family-name:var(--font-heading)] text-[var(--text-2xl)]"
              style={{ color: 'var(--text-primary)' }}
            >
              Cases
            </h1>
            <p className="text-[var(--text-sm)]" style={{ color: 'var(--text-tertiary)' }}>
              {total} {total === 1 ? 'case' : 'cases'}
            </p>
          </div>
          {canCreate && (
            <a
              href="/en/cases/new"
              className="px-[var(--space-md)] py-[var(--space-xs)] text-[var(--text-sm)] font-medium"
              style={{
                backgroundColor: 'var(--amber-accent)',
                color: 'var(--stone-950)',
              }}
            >
              New case
            </a>
          )}
        </div>

        {error && (
          <div
            className="mb-[var(--space-md)] px-[var(--space-md)] py-[var(--space-sm)] text-[var(--text-sm)]"
            style={{
              backgroundColor: 'var(--status-hold-bg)',
              color: 'var(--status-hold)',
              borderLeft: '3px solid var(--status-hold)',
            }}
          >
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
      </div>
    </Shell>
  );
}
