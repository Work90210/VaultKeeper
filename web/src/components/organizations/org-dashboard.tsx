'use client';

import { useState } from 'react';
import Link from 'next/link';
import type { Organization, OrgMembership, Case } from '@/types';
import { MemberManagement } from './member-management';

interface Props {
  org: Organization;
  members: OrgMembership[];
  cases: Case[];
}

type Tab = 'members' | 'cases';

export function OrgDashboard({ org, members, cases }: Props) {
  const [activeTab, setActiveTab] = useState<Tab>('members');

  return (
    <div style={{ maxWidth: '64rem', marginInline: 'auto', padding: 'var(--space-lg)' }}>
      {/* Header */}
      <header style={{ marginBottom: 'var(--space-lg)' }}>
        <h1
          className="font-[family-name:var(--font-heading)] text-balance"
          style={{ fontSize: 'var(--text-xl)', color: 'var(--text-primary)' }}
        >
          {org.name}
        </h1>
        {org.description && (
          <p style={{ marginTop: 'var(--space-xs)', fontSize: 'var(--text-sm)', color: 'var(--text-secondary)' }}>
            {org.description}
          </p>
        )}
      </header>

      {/* Stats */}
      <div
        className="card-inset grid grid-cols-3"
        style={{ gap: 'var(--space-md)', padding: 'var(--space-md)', marginBottom: 'var(--space-lg)' }}
      >
        <StatCard label="Members" value={members.length} />
        <StatCard label="Cases" value={cases.length} />
        <StatCard label="Active" value={cases.filter((c) => c.status === 'active').length} />
      </div>

      {/* Tabs */}
      <div style={{ borderBottom: '1px solid var(--border-subtle)', marginBottom: 'var(--space-md)' }}>
        <nav className="flex" style={{ gap: 'var(--space-lg)', marginBottom: '-1px' }}>
          <TabButton label="Members" active={activeTab === 'members'} onClick={() => setActiveTab('members')} />
          <TabButton label="Cases" active={activeTab === 'cases'} onClick={() => setActiveTab('cases')} />
        </nav>
      </div>

      {/* Content */}
      <div className="stagger-in">
        {activeTab === 'members' && <MemberManagement orgId={org.id} members={members} />}
        {activeTab === 'cases' && <CaseList cases={cases} />}
      </div>
    </div>
  );
}

function StatCard({ label, value }: { label: string; value: number }) {
  return (
    <div>
      <p className="field-label" style={{ marginBottom: '0.125rem' }}>{label}</p>
      <p
        className="font-[family-name:var(--font-mono)] tabular-nums"
        style={{ fontSize: 'var(--text-xl)', fontWeight: 600, color: 'var(--text-primary)' }}
      >
        {value}
      </p>
    </div>
  );
}

function TabButton({ label, active, onClick }: { label: string; active: boolean; onClick: () => void }) {
  return (
    <button
      onClick={onClick}
      type="button"
      style={{
        paddingBottom: 'var(--space-sm)',
        fontSize: 'var(--text-sm)',
        fontWeight: 500,
        borderBottom: active ? '2px solid var(--amber-accent)' : '2px solid transparent',
        color: active ? 'var(--text-primary)' : 'var(--text-tertiary)',
        background: 'none',
        border: 'none',
        borderBottomStyle: 'solid',
        borderBottomWidth: '2px',
        borderBottomColor: active ? 'var(--amber-accent)' : 'transparent',
        cursor: 'pointer',
        transition: `all var(--duration-fast) ease`,
      }}
    >
      {label}
    </button>
  );
}

function CaseList({ cases }: { cases: Case[] }) {
  if (cases.length === 0) {
    return (
      <div className="card-inset" style={{ padding: 'var(--space-xl)', textAlign: 'center' }}>
        <p style={{ fontSize: 'var(--text-sm)', color: 'var(--text-tertiary)' }}>
          No cases yet. Create your first case.
        </p>
      </div>
    );
  }

  return (
    <div className="card-inset" style={{ padding: 0, overflow: 'hidden' }}>
      <table className="w-full" style={{ fontSize: 'var(--text-sm)', borderCollapse: 'collapse' }}>
        <thead>
          <tr style={{ borderBottom: '1px solid var(--border-subtle)' }}>
            <th className="text-left field-label" style={{ padding: 'var(--space-sm) var(--space-md)', marginBottom: 0 }}>Title</th>
            <th className="text-left field-label" style={{ padding: 'var(--space-sm) var(--space-md)', marginBottom: 0 }}>Reference</th>
            <th className="text-right field-label" style={{ padding: 'var(--space-sm) var(--space-md)', marginBottom: 0 }}>Status</th>
          </tr>
        </thead>
        <tbody>
          {cases.map((c) => (
            <tr
              key={c.id}
              className="table-row"
              onClick={() => window.location.href = `/en/cases/${c.id}`}
              style={{ borderBottom: '1px solid var(--border-subtle)' }}
            >
              <td style={{ padding: 'var(--space-sm) var(--space-md)', color: 'var(--text-primary)' }}>
                {c.title}
              </td>
              <td
                className="font-[family-name:var(--font-mono)]"
                style={{ padding: 'var(--space-sm) var(--space-md)', color: 'var(--text-tertiary)', fontSize: 'var(--text-xs)' }}
              >
                {c.reference_code}
              </td>
              <td className="text-right" style={{ padding: 'var(--space-sm) var(--space-md)' }}>
                <span
                  className="badge"
                  style={{
                    backgroundColor: c.status === 'active' ? 'var(--status-active-bg)' : c.status === 'closed' ? 'var(--status-closed-bg)' : 'var(--status-archived-bg)',
                    color: c.status === 'active' ? 'var(--status-active)' : c.status === 'closed' ? 'var(--status-closed)' : 'var(--status-archived)',
                  }}
                >
                  {c.status}
                </span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
