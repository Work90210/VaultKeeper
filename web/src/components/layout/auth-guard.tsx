import { getServerSession } from 'next-auth';
import { redirect } from 'next/navigation';
import { authOptions } from '@/lib/auth';

export async function AuthGuard({ children }: { children: React.ReactNode }) {
  const session = await getServerSession(authOptions);

  if (!session || session.error === 'RefreshAccessTokenError') {
    redirect('/en/login');
  }

  return <>{children}</>;
}
