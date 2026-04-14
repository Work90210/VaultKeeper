'use client';

import {
  createContext,
  useCallback,
  useEffect,
  useState,
  type ReactNode,
} from 'react';
import { useSession } from 'next-auth/react';
import type { OrgWithRole, OrgRole } from '@/types';

interface OrgContextValue {
  activeOrg: OrgWithRole | null;
  setActiveOrg: (org: OrgWithRole) => void;
  userOrgs: OrgWithRole[];
  orgRole: OrgRole | null;
  isOrgAdmin: boolean;
  isOrgOwner: boolean;
  loading: boolean;
  refresh: () => Promise<void>;
}

export const OrgContext = createContext<OrgContextValue>({
  activeOrg: null,
  setActiveOrg: () => {},
  userOrgs: [],
  orgRole: null,
  isOrgAdmin: false,
  isOrgOwner: false,
  loading: true,
  refresh: async () => {},
});

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';
const ORG_COOKIE_KEY = 'vk-active-org';

function getOrgFromCookie(): string | null {
  if (typeof document === 'undefined') return null;
  const match = document.cookie.match(
    new RegExp(`(?:^|; )${ORG_COOKIE_KEY}=([^;]*)`)
  );
  return match ? decodeURIComponent(match[1]) : null;
}

function setOrgCookie(orgId: string) {
  document.cookie = `${ORG_COOKIE_KEY}=${encodeURIComponent(orgId)}; path=/; max-age=${60 * 60 * 24 * 365}; SameSite=Lax`;
}

export function OrgProvider({ children }: { children: ReactNode }) {
  const { data: session } = useSession();
  const [userOrgs, setUserOrgs] = useState<OrgWithRole[]>([]);
  const [activeOrg, setActiveOrgState] = useState<OrgWithRole | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchOrgs = useCallback(async () => {
    if (!session?.accessToken) return;
    try {
      const res = await fetch(`${API_BASE}/api/organizations`, {
        headers: { Authorization: `Bearer ${session.accessToken}` },
      });
      if (!res.ok) return;
      const body = await res.json();
      // Backend wraps response in { data, error, meta } envelope
      const orgs: OrgWithRole[] = Array.isArray(body) ? body : (body.data ?? []);
      setUserOrgs(orgs);

      const savedOrgId = getOrgFromCookie();
      const saved = orgs.find((o) => o.id === savedOrgId);
      const selected = saved ?? orgs[0] ?? null;
      setActiveOrgState(selected);
      if (selected) setOrgCookie(selected.id);
    } finally {
      setLoading(false);
    }
  }, [session?.accessToken]);

  useEffect(() => {
    fetchOrgs();
  }, [fetchOrgs, session?.accessToken]);

  const setActiveOrg = useCallback(
    (org: OrgWithRole) => {
      setActiveOrgState(org);
      setOrgCookie(org.id);
      // Server components read the org from cookie — reload to re-render with new org scope
      window.location.reload();
    },
    []
  );

  const orgRole = activeOrg?.role ?? null;

  return (
    <OrgContext.Provider
      value={{
        activeOrg,
        setActiveOrg,
        userOrgs,
        orgRole,
        isOrgAdmin: orgRole === 'owner' || orgRole === 'admin',
        isOrgOwner: orgRole === 'owner',
        loading,
        refresh: fetchOrgs,
      }}
    >
      {children}
    </OrgContext.Provider>
  );
}
