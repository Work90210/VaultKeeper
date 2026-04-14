import { AuthGuard } from '@/components/layout/auth-guard';
import { OrgProvider } from '@/components/providers/org-provider';
import { CaseProvider } from '@/components/providers/case-provider';
import { Sidebar } from '@/components/layout/sidebar';
import { TopBar } from '@/components/layout/top-bar';

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
            <div className="flex-1 flex flex-col min-w-0">
              <TopBar />
              <main className="flex-1 overflow-y-auto">{children}</main>
            </div>
          </div>
        </CaseProvider>
      </OrgProvider>
    </AuthGuard>
  );
}
