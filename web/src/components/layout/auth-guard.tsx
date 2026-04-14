import { getServerSession } from 'next-auth';
import { redirect } from 'next/navigation';
import { authOptions } from '@/lib/auth';
import { getOrganizations } from '@/lib/org-api';

export async function AuthGuard({ children }: { children: React.ReactNode }) {
  const session = await getServerSession(authOptions);

  if (!session || session.error === 'RefreshAccessTokenError') {
    redirect('/en/login');
  }

  const orgsRes = await getOrganizations();
  const orgs = orgsRes.data;

  if (!orgs || orgs.length === 0) {
    // Allow access to the org creation page itself.
    return <>{children}</>;
  }

  return <>{children}</>;
}
