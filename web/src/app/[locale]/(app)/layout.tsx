import { AuthGuard } from '@/components/layout/auth-guard';
import { OrgProvider } from '@/components/providers/org-provider';
import { CaseProvider } from '@/components/providers/case-provider';
import { Sidebar } from '@/components/layout/sidebar';

export default async function AppLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <AuthGuard>
      <OrgProvider>
        <CaseProvider>
          <div className="flex h-screen">
            <Sidebar />
            <main className="flex-1 overflow-y-auto">{children}</main>
          </div>
        </CaseProvider>
      </OrgProvider>
    </AuthGuard>
  );
}
