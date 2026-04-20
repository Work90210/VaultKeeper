import { authenticatedFetch } from './api';
import type { OrgWithRole, UserProfile } from '@/types';

export async function getOrganizations() {
  return authenticatedFetch<OrgWithRole[]>('/api/organizations');
}

export async function getProfile() {
  return authenticatedFetch<{
    profile: UserProfile;
    email: string;
    role: string;
  }>('/api/me');
}
