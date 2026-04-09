import { AuthGuard } from '@/components/layout/auth-guard';

export default async function AppLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return <AuthGuard>{children}</AuthGuard>;
}
