import { authenticatedFetch } from './api';
import type {
  Organization,
  OrgWithRole,
  OrgMembership,
  OrgInvitation,
  OrgRole,
  Case,
  CaseAssignment,
  UserProfile,
} from '@/types';

export async function createOrganization(data: {
  name: string;
  description?: string;
}) {
  return authenticatedFetch<Organization>('/api/organizations', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function getOrganizations() {
  return authenticatedFetch<OrgWithRole[]>('/api/organizations');
}

export async function getOrganization(id: string) {
  return authenticatedFetch<Organization>(`/api/organizations/${id}`);
}

export async function updateOrganization(
  id: string,
  data: { name?: string; description?: string }
) {
  return authenticatedFetch<Organization>(`/api/organizations/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(data),
  });
}

export async function deleteOrganization(id: string) {
  return authenticatedFetch<void>(`/api/organizations/${id}`, {
    method: 'DELETE',
  });
}

export async function getOrgMembers(orgId: string) {
  return authenticatedFetch<OrgMembership[]>(
    `/api/organizations/${orgId}/members`
  );
}

export async function updateMemberRole(
  orgId: string,
  userId: string,
  role: OrgRole
) {
  return authenticatedFetch<void>(
    `/api/organizations/${orgId}/members/${userId}`,
    {
      method: 'PATCH',
      body: JSON.stringify({ role }),
    }
  );
}

export async function removeMember(orgId: string, userId: string) {
  return authenticatedFetch<void>(
    `/api/organizations/${orgId}/members/${userId}`,
    { method: 'DELETE' }
  );
}

export async function transferOwnership(orgId: string, targetUserId: string) {
  return authenticatedFetch<void>(
    `/api/organizations/${orgId}/ownership-transfer`,
    {
      method: 'POST',
      body: JSON.stringify({ target_user_id: targetUserId }),
    }
  );
}

export async function inviteMember(
  orgId: string,
  email: string,
  role: OrgRole
) {
  return authenticatedFetch<{ invitation: OrgInvitation; token: string }>(
    `/api/organizations/${orgId}/invitations`,
    {
      method: 'POST',
      body: JSON.stringify({ email, role }),
    }
  );
}

export async function getOrgInvitations(orgId: string) {
  return authenticatedFetch<OrgInvitation[]>(
    `/api/organizations/${orgId}/invitations`
  );
}

export async function revokeInvitation(orgId: string, inviteId: string) {
  return authenticatedFetch<void>(
    `/api/organizations/${orgId}/invitations/${inviteId}`,
    { method: 'DELETE' }
  );
}

export async function acceptInvitation(token: string) {
  return authenticatedFetch<Organization>('/api/invitations/accept', {
    method: 'POST',
    body: JSON.stringify({ token }),
  });
}

export async function declineInvitation(token: string) {
  return authenticatedFetch<void>('/api/invitations/decline', {
    method: 'POST',
    body: JSON.stringify({ token }),
  });
}

export async function getProfile() {
  return authenticatedFetch<{
    profile: UserProfile;
    email: string;
    role: string;
  }>('/api/me');
}

export async function updateProfile(data: {
  display_name?: string;
  bio?: string;
  timezone?: string;
  avatar_url?: string;
}) {
  return authenticatedFetch<UserProfile>('/api/me', {
    method: 'PATCH',
    body: JSON.stringify(data),
  });
}

export async function getOrgCaseAssignments(orgId: string) {
  return authenticatedFetch<CaseAssignment[]>(
    `/api/organizations/${orgId}/case-assignments`
  );
}

export async function getOrgCases(orgId: string) {
  return authenticatedFetch<Case[]>(
    `/api/cases?organization_id=${orgId}`
  );
}
