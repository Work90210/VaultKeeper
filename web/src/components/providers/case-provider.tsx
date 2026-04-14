'use client';

import { createContext, useContext, useState, useCallback, type ReactNode } from 'react';

export interface CaseContextData {
  id: string;
  reference_code: string;
  title: string;
  status: string;
  canEdit: boolean;
}

type SidebarCounts = Record<string, number>;

interface CaseContextValue {
  caseData: CaseContextData | null;
  setCaseData: (data: CaseContextData | null) => void;
  activeTab: string;
  setActiveTab: (tab: string) => void;
  sidebarCounts: SidebarCounts;
  setSidebarCounts: (counts: SidebarCounts) => void;
  updateSidebarCounts: (counts: SidebarCounts) => void;
}

const CaseContext = createContext<CaseContextValue>({
  caseData: null,
  setCaseData: () => {},
  activeTab: 'overview',
  setActiveTab: () => {},
  sidebarCounts: {},
  setSidebarCounts: () => {},
  updateSidebarCounts: () => {},
});

export function CaseProvider({ children }: { children: ReactNode }) {
  const [caseData, setCaseData] = useState<CaseContextData | null>(null);
  const [activeTab, setActiveTab] = useState('overview');
  const [sidebarCounts, setSidebarCounts] = useState<SidebarCounts>({});

  const updateSidebarCounts = useCallback((counts: SidebarCounts) => {
    setSidebarCounts((prev) => ({ ...prev, ...counts }));
  }, []);

  return (
    <CaseContext.Provider
      value={{
        caseData,
        setCaseData,
        activeTab,
        setActiveTab,
        sidebarCounts,
        setSidebarCounts,
        updateSidebarCounts,
      }}
    >
      {children}
    </CaseContext.Provider>
  );
}

export function useCaseContext() {
  return useContext(CaseContext);
}
