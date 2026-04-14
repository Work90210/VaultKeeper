import { getProfile, getOrganizations } from '@/lib/org-api';
import { ProfileView } from '@/components/profile/profile-view';

export default async function ProfilePage() {
  const [profileRes, orgsRes] = await Promise.all([
    getProfile(),
    getOrganizations(),
  ]);

  const profile = profileRes.data;
  const orgs = orgsRes.data ?? [];

  if (!profile) {
    return (
      <div className="flex h-64 items-center justify-center text-stone-500">
        Failed to load profile
      </div>
    );
  }

  return (
    <ProfileView
      profile={profile.profile}
      email={profile.email}
      systemRole={profile.role}
      organizations={orgs}
    />
  );
}
