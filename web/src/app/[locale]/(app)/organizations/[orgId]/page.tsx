import { getOrganization, getOrgMembers, getOrgCases } from '@/lib/org-api';
import { OrgDashboard } from '@/components/organizations/org-dashboard';

interface Props {
  params: Promise<{ orgId: string; locale: string }>;
}

export default async function OrganizationPage({ params }: Props) {
  const { orgId } = await params;

  const [orgRes, membersRes, casesRes] = await Promise.all([
    getOrganization(orgId),
    getOrgMembers(orgId),
    getOrgCases(orgId),
  ]);

  if (orgRes.error || !orgRes.data) {
    return (
      <div className="flex h-64 items-center justify-center text-stone-500">
        Organization not found
      </div>
    );
  }

  return (
    <OrgDashboard
      org={orgRes.data}
      members={membersRes.data ?? []}
      cases={casesRes.data ?? []}
    />
  );
}
