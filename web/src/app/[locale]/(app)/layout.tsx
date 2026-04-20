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
          <div className="dash">
            <Sidebar />
            <div className="d-main">
              <TopBar />
              <main className="d-content">{children}</main>
            </div>
          </div>
        </CaseProvider>
      </OrgProvider>
    </AuthGuard>
  );
}
