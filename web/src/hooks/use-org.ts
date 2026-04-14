'use client';

import { useContext } from 'react';
import { OrgContext } from '@/components/providers/org-provider';

export function useOrg() {
  return useContext(OrgContext);
}
