'use client';

import { useState, useMemo, useCallback } from 'react';
import type { InquiryLog } from '@/types';
import {
  Panel,
  FilterBar,
  Modal,
  StatusPill,
  EyebrowLabel,
  LinkArrow,
  KPIStrip,
} from '@/components/ui/dashboard';

// ── Types ──────────────────────────────────────────────────────────

type EntryKind = 'decision' | 'question' | 'action' | 'request' | 'federation';
type InquiryStatus = 'active' | 'locked' | 'complete';

interface InquiryEntry {
  readonly id: string;
  readonly timestamp: string;
  readonly author: string;
  readonly avatarClass: string;
  readonly kind: EntryKind;
  readonly content: string;
  readonly linked: readonly string[];
  readonly itemsCount: number;
}

interface InquiryDay {
  readonly label: string;
  readonly entries: readonly InquiryEntry[];
}

// ── Constants ──────────────────────────────────────────────────────

const KIND_META: Record<EntryKind, { readonly label: string; readonly pill: 'sealed' | 'disc' | 'hold' | 'draft' | 'live' }> = {
  decision: { label: 'Decision', pill: 'sealed' },
  question: { label: 'Open question', pill: 'disc' },
  action: { label: 'Action', pill: 'hold' },
  request: { label: 'External request', pill: 'draft' },
  federation: { label: 'Federation', pill: 'live' },
};

const KIND_OPTIONS: readonly EntryKind[] = ['decision', 'question', 'action', 'request', 'federation'];

const PRIORITY_OPTIONS = ['Low', 'Medium', 'High', 'Critical'] as const;

// ── Stub Data ──────────────────────────────────────────────────────

const STUB_ENTRIES: readonly InquiryEntry[] = [
  { id: '1', timestamp: '2026-04-19T14:22:00Z', author: 'H. Morel', avatarClass: 'a', kind: 'decision', content: 'Recommend inclusion of E-0918 in DISC-2026-019 bundle. Countersign required from W. Nyoka before seal.', linked: ['E-0918', 'DISC-2026-019'], itemsCount: 2 },
  { id: '2', timestamp: '2026-04-19T11:06:00Z', author: 'Amir H.', avatarClass: 'c', kind: 'question', content: 'Do we have independent confirmation of the 17:52 timing beyond W-0144 and drone E-0918? Requesting a second satellite pass review.', linked: ['C-0412'], itemsCount: 1 },
  { id: '3', timestamp: '2026-04-19T09:32:00Z', author: 'Martyna K.', avatarClass: 'b', kind: 'action', content: 'Applied geo-fuzz (50 km) to all mentions of Andriivka in E-0912 redaction draft v4. Awaiting peer-review.', linked: ['E-0912', 'NOTE-182'], itemsCount: 2 },
  { id: '4', timestamp: '2026-04-18T16:41:00Z', author: 'W. Nyoka', avatarClass: 'e', kind: 'request', content: 'Formal request to OLAF for cross-reference on Raiffeisen ledger entries; routing via liaison officer Brussels.', linked: ['E-0914'], itemsCount: 1 },
  { id: '5', timestamp: '2026-04-18T10:15:00Z', author: 'H. Morel', avatarClass: 'a', kind: 'decision', content: 'Accept counter-evidence preservation per Rule 77 for defence exculpatory motion. NOTE-180 signed, disclosable.', linked: ['NOTE-180'], itemsCount: 1 },
  { id: '6', timestamp: '2026-04-17T13:28:00Z', author: 'Juliane', avatarClass: 'd', kind: 'action', content: 'Re-pseudonymised W-0139 \u2192 new intermediary token on legal officer request. Mapping re-sealed.', linked: ['W-0139'], itemsCount: 1 },
  { id: '7', timestamp: '2026-04-17T09:04:00Z', author: 'CIJA Berlin', avatarClass: 'e', kind: 'federation', content: 'Federated case mirrored 612 exhibits into sub-chain /CIJA. Parallel investigation opened under German code.', linked: ['sub-chain /CIJA'], itemsCount: 612 },
  { id: '8', timestamp: '2026-04-16T15:12:00Z', author: 'Amir H.', avatarClass: 'c', kind: 'question', content: 'W-0139 is currently single-source for second-convoy claim (C-0411). Is a secondary witness intake realistic before the Fri 25 deadline?', linked: ['C-0411', 'W-0139'], itemsCount: 2 },
  { id: '9', timestamp: '2026-04-16T08:55:00Z', author: 'H. Morel', avatarClass: 'a', kind: 'decision', content: 'De-prioritise C-0411 for DISC-2026-019 bundle. Single-source; will be disclosed as uncorroborated in separate note.', linked: ['C-0411'], itemsCount: 1 },
];

const STUB_KPI_ITEMS = [
  { label: 'Total entries', value: 184, sub: 'Berkeley Protocol Phase 1' },
  { label: 'Decisions', value: 42, delta: '+3 this week' },
  { label: 'Open questions', value: 9, delta: '-2 resolved', deltaNegative: true },
  { label: 'Federation', value: 31, sub: '4 active mirrors' },
];

// ── Helpers ────────────────────────────────────────────────────────

function formatTime(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' });
}

function formatDayLabel(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' });
}

function groupByDay(entries: readonly InquiryEntry[]): readonly InquiryDay[] {
  const map = new Map<string, InquiryEntry[]>();
  for (const entry of entries) {
    const dayKey = formatDayLabel(entry.timestamp);
    const existing = map.get(dayKey);
    if (existing) {
      existing.push(entry);
    } else {
      map.set(dayKey, [entry]);
    }
  }
  return Array.from(map.entries()).map(([label, items]) => ({
    label,
    entries: items,
  }));
}

function countByKind(entries: readonly InquiryEntry[], kind: EntryKind): number {
  return entries.filter((e) => e.kind === kind).length;
}

// ── Sub-components ─────────────────────────────────────────────────

function DayHeader({ label }: { readonly label: string }) {
  return (
    <div
      style={{
        padding: '14px 28px',
        background: 'var(--bg-2)',
        borderBottom: '1px solid var(--line)',
        fontFamily: "'JetBrains Mono', monospace",
        fontSize: 11,
        letterSpacing: '.08em',
        textTransform: 'uppercase',
        color: 'var(--muted)',
      }}
    >
      {label}
    </div>
  );
}

function EntryRow({ entry, linkable }: { readonly entry: InquiryEntry; readonly linkable: boolean }) {
  const meta = KIND_META[entry.kind];
  const initial = entry.author[0].toUpperCase();

  return (
    <div
      style={{
        padding: '22px 28px',
        borderBottom: '1px solid var(--line)',
        display: 'grid',
        gridTemplateColumns: '72px 40px 1fr 220px',
        gap: 20,
        alignItems: 'start',
        cursor: linkable ? 'pointer' : 'default',
        transition: 'background .12s',
      }}
      onClick={linkable ? () => { window.location.href = `/en/inquiry-logs/${entry.id}`; } : undefined}
    >
      {/* Time */}
      <div
        style={{
          fontFamily: "'JetBrains Mono', monospace",
          fontSize: 13,
          color: 'var(--muted)',
        }}
      >
        {formatTime(entry.timestamp)}
      </div>

      {/* Avatar */}
      <span className="avs">
        <span className={`av ${entry.avatarClass}`}>{initial}</span>
      </span>

      {/* Author + type pill + content + view link */}
      <div>
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 10,
            marginBottom: 6,
          }}
        >
          <span
            style={{
              fontFamily: "'Fraunces', serif",
              fontSize: 15,
              letterSpacing: '-.005em',
              color: 'var(--ink)',
            }}
          >
            {entry.author}
          </span>
          <StatusPill status={meta.pill}>{meta.label}</StatusPill>
        </div>
        <div
          style={{
            fontSize: 14.5,
            lineHeight: 1.55,
            color: 'var(--ink-2)',
            maxWidth: '62ch',
            marginBottom: 8,
          }}
        >
          {entry.content}
        </div>
        {linkable ? (
          <LinkArrow href={`/en/inquiry-logs/${entry.id}`}>View full entry</LinkArrow>
        ) : (
          <span style={{ fontSize: 13, color: 'var(--accent)' }}>View full entry &rarr;</span>
        )}
      </div>

      {/* Linked items + count */}
      <div style={{ textAlign: 'right' }}>
        <div
          style={{
            fontFamily: "'JetBrains Mono', monospace",
            fontSize: 11.5,
            color: 'var(--accent)',
            letterSpacing: '.02em',
            marginBottom: 4,
          }}
        >
          {entry.linked.join(' \u00b7 ')}
        </div>
        <div
          style={{
            fontFamily: "'JetBrains Mono', monospace",
            fontSize: 10,
            color: 'var(--muted)',
            letterSpacing: '.04em',
          }}
        >
          {entry.itemsCount} item{entry.itemsCount !== 1 ? 's' : ''}
        </div>
      </div>
    </div>
  );
}

function LockedBanner({ status, onLock, onComplete }: {
  readonly status: InquiryStatus;
  readonly onLock: () => void;
  readonly onComplete: () => void;
}) {
  if (status === 'active') {
    return (
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'flex-end',
          gap: 12,
          padding: '12px 28px',
          borderBottom: '1px solid var(--line)',
          background: 'var(--bg-2)',
        }}
      >
        <StatusPill status="active">Active</StatusPill>
        <button type="button" className="btn ghost" style={{ fontSize: 13 }} onClick={onLock}>
          Lock inquiry
        </button>
      </div>
    );
  }

  if (status === 'locked') {
    return (
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 12,
          padding: '14px 28px',
          borderBottom: '1px solid var(--line)',
          background: 'var(--bg-warn, #fdf6e3)',
        }}
      >
        <StatusPill status="locked">Locked</StatusPill>
        <span style={{ fontSize: 13, color: 'var(--ink-2)' }}>
          This inquiry is locked. No new entries can be added.
        </span>
        <button
          type="button"
          className="btn ghost"
          style={{ marginLeft: 'auto', fontSize: 13 }}
          onClick={onComplete}
        >
          Mark complete
        </button>
      </div>
    );
  }

  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 12,
        padding: '14px 28px',
        borderBottom: '1px solid var(--line)',
        background: 'var(--bg-success, #f0fdf4)',
      }}
    >
      <StatusPill status="complete">Complete</StatusPill>
      <span style={{ fontSize: 13, color: 'var(--ink-2)' }}>
        This inquiry has been permanently sealed. No modifications permitted.
      </span>
    </div>
  );
}

function NewEntryModal({ open, onClose }: {
  readonly open: boolean;
  readonly onClose: () => void;
}) {
  const [selectedKind, setSelectedKind] = useState<EntryKind>('action');

  return (
    <Modal open={open} onClose={onClose} title="New inquiry entry" wide>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
        {/* Entry type chips */}
        <div>
          <EyebrowLabel>Entry type</EyebrowLabel>
          <div style={{ display: 'flex', gap: 8, marginTop: 8, flexWrap: 'wrap' }}>
            {KIND_OPTIONS.map((kind) => {
              const meta = KIND_META[kind];
              const isActive = kind === selectedKind;
              return (
                <button
                  key={kind}
                  type="button"
                  className={`chip${isActive ? ' active' : ''}`}
                  onClick={() => setSelectedKind(kind)}
                >
                  {meta.label}
                </button>
              );
            })}
          </div>
        </div>

        {/* Content */}
        <div>
          <EyebrowLabel>Content</EyebrowLabel>
          <textarea
            rows={4}
            placeholder="Describe the inquiry action, decision, or question..."
            style={{
              width: '100%',
              marginTop: 8,
              padding: '10px 12px',
              fontFamily: 'Inter, sans-serif',
              fontSize: 14,
              lineHeight: 1.55,
              border: '1px solid var(--line)',
              borderRadius: 'var(--radius, 6px)',
              background: 'var(--bg-2)',
              color: 'var(--ink)',
              resize: 'vertical',
            }}
          />
        </div>

        {/* Assigned to + Priority row */}
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
          <div>
            <EyebrowLabel>Assigned to</EyebrowLabel>
            <select
              style={{
                width: '100%',
                marginTop: 8,
                padding: '8px 12px',
                border: '1px solid var(--line)',
                borderRadius: 'var(--radius, 6px)',
                background: 'var(--bg-2)',
                color: 'var(--ink)',
                fontSize: 14,
              }}
            >
              <option value="">Select assignee...</option>
              <option value="h-morel">H. Morel</option>
              <option value="amir-h">Amir H.</option>
              <option value="martyna-k">Martyna K.</option>
              <option value="w-nyoka">W. Nyoka</option>
              <option value="juliane">Juliane</option>
            </select>
          </div>
          <div>
            <EyebrowLabel>Priority</EyebrowLabel>
            <select
              style={{
                width: '100%',
                marginTop: 8,
                padding: '8px 12px',
                border: '1px solid var(--line)',
                borderRadius: 'var(--radius, 6px)',
                background: 'var(--bg-2)',
                color: 'var(--ink)',
                fontSize: 14,
              }}
            >
              {PRIORITY_OPTIONS.map((p) => (
                <option key={p} value={p.toLowerCase()}>{p}</option>
              ))}
            </select>
          </div>
        </div>

        {/* Search strategy + Search tool row */}
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
          <div>
            <EyebrowLabel>Search strategy</EyebrowLabel>
            <input
              type="text"
              placeholder="e.g. Reverse image, OSINT, satellite..."
              style={{
                width: '100%',
                marginTop: 8,
                padding: '8px 12px',
                border: '1px solid var(--line)',
                borderRadius: 'var(--radius, 6px)',
                background: 'var(--bg-2)',
                color: 'var(--ink)',
                fontSize: 14,
              }}
            />
          </div>
          <div>
            <EyebrowLabel>Search tool</EyebrowLabel>
            <input
              type="text"
              placeholder="e.g. Google Earth, TinEye, Hunchly..."
              style={{
                width: '100%',
                marginTop: 8,
                padding: '8px 12px',
                border: '1px solid var(--line)',
                borderRadius: 'var(--radius, 6px)',
                background: 'var(--bg-2)',
                color: 'var(--ink)',
                fontSize: 14,
              }}
            />
          </div>
        </div>

        {/* Search dates + results row */}
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr 1fr', gap: 16 }}>
          <div>
            <EyebrowLabel>Search started</EyebrowLabel>
            <input
              type="datetime-local"
              style={{
                width: '100%',
                marginTop: 8,
                padding: '8px 12px',
                border: '1px solid var(--line)',
                borderRadius: 'var(--radius, 6px)',
                background: 'var(--bg-2)',
                color: 'var(--ink)',
                fontSize: 13,
              }}
            />
          </div>
          <div>
            <EyebrowLabel>Search ended</EyebrowLabel>
            <input
              type="datetime-local"
              style={{
                width: '100%',
                marginTop: 8,
                padding: '8px 12px',
                border: '1px solid var(--line)',
                borderRadius: 'var(--radius, 6px)',
                background: 'var(--bg-2)',
                color: 'var(--ink)',
                fontSize: 13,
              }}
            />
          </div>
          <div>
            <EyebrowLabel>Results found</EyebrowLabel>
            <input
              type="number"
              min={0}
              placeholder="0"
              style={{
                width: '100%',
                marginTop: 8,
                padding: '8px 12px',
                border: '1px solid var(--line)',
                borderRadius: 'var(--radius, 6px)',
                background: 'var(--bg-2)',
                color: 'var(--ink)',
                fontSize: 14,
              }}
            />
          </div>
          <div>
            <EyebrowLabel>Results relevant</EyebrowLabel>
            <input
              type="number"
              min={0}
              placeholder="0"
              style={{
                width: '100%',
                marginTop: 8,
                padding: '8px 12px',
                border: '1px solid var(--line)',
                borderRadius: 'var(--radius, 6px)',
                background: 'var(--bg-2)',
                color: 'var(--ink)',
                fontSize: 14,
              }}
            />
          </div>
        </div>

        {/* Linked exhibits/witnesses */}
        <div>
          <EyebrowLabel>Linked exhibits / witnesses</EyebrowLabel>
          <input
            type="text"
            placeholder="E-0918, W-0144, C-0412..."
            style={{
              width: '100%',
              marginTop: 8,
              padding: '8px 12px',
              border: '1px solid var(--line)',
              borderRadius: 'var(--radius, 6px)',
              background: 'var(--bg-2)',
              color: 'var(--ink)',
              fontSize: 14,
            }}
          />
        </div>

        {/* Submit */}
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 12, paddingTop: 8 }}>
          <button type="button" className="btn ghost" onClick={onClose}>
            Cancel
          </button>
          <button type="button" className="btn" onClick={onClose}>
            Create entry <span className="arr">&rarr;</span>
          </button>
        </div>
      </div>
    </Modal>
  );
}

// ── Main Component ─────────────────────────────────────────────────

export interface InquiryViewProps {
  readonly caseRef?: string;
  readonly logs?: InquiryLog[];
}

export function InquiryView({ caseRef, logs }: InquiryViewProps) {
  const [activeFilter, setActiveFilter] = useState<string>('all');
  const [searchQuery, setSearchQuery] = useState('');
  const [inquiryStatus, setInquiryStatus] = useState<InquiryStatus>('active');
  const [modalOpen, setModalOpen] = useState(false);

  const hasRealData = logs !== undefined && logs.length > 0;

  const entries: readonly InquiryEntry[] = hasRealData
    ? logs.map((log): InquiryEntry => ({
        id: log.id,
        timestamp: log.search_started_at,
        author: log.performed_by,
        avatarClass: 'a',
        kind: 'action',
        content: `${log.search_strategy}: ${log.objective}`,
        linked: log.evidence_id ? [log.evidence_id] : [],
        itemsCount: log.results_relevant ?? log.results_count ?? 0,
      }))
    : STUB_ENTRIES;

  // Filter entries
  const filtered = useMemo(() => {
    const byKind = activeFilter === 'all'
      ? entries
      : entries.filter((e) => e.kind === activeFilter);

    if (!searchQuery.trim()) return byKind;

    const q = searchQuery.toLowerCase();
    return byKind.filter(
      (e) =>
        e.content.toLowerCase().includes(q) ||
        e.author.toLowerCase().includes(q) ||
        e.linked.some((l) => l.toLowerCase().includes(q)),
    );
  }, [entries, activeFilter, searchQuery]);

  const days = useMemo(() => groupByDay(filtered), [filtered]);

  const chips = useMemo(
    () => [
      { key: 'all', label: 'All', count: entries.length, active: activeFilter === 'all' },
      { key: 'decision', label: 'Decisions', count: countByKind(entries, 'decision'), active: activeFilter === 'decision' },
      { key: 'question', label: 'Open questions', count: countByKind(entries, 'question'), active: activeFilter === 'question' },
      { key: 'action', label: 'Actions', count: countByKind(entries, 'action'), active: activeFilter === 'action' },
      { key: 'request', label: 'External requests', count: countByKind(entries, 'request'), active: activeFilter === 'request' },
      { key: 'federation', label: 'Federation', count: countByKind(entries, 'federation'), active: activeFilter === 'federation' },
    ],
    [entries, activeFilter],
  );

  const handleChipClick = useCallback((key: string) => {
    setActiveFilter(key);
  }, []);

  const handleLock = useCallback(() => {
    setInquiryStatus('locked');
  }, []);

  const handleComplete = useCallback(() => {
    setInquiryStatus('complete');
  }, []);

  const isSealed = inquiryStatus === 'locked' || inquiryStatus === 'complete';

  return (
    <>
      {/* Page header */}
      <section className="d-pagehead">
        <div>
          <EyebrowLabel>
            {caseRef
              ? `Case \u00b7 ${caseRef} \u00b7 Berkeley Protocol Phase 1`
              : 'Berkeley Protocol Phase 1'}
          </EyebrowLabel>
          <h1>
            Inquiry <em>log</em>
          </h1>
          <p className="sub">
            Phase 1 of the Berkeley Protocol requires documenting all search
            strategies, tools, and discovery timelines. Each entry is a signed
            record on the same chain as evidence &mdash; this is how we show{' '}
            <em>how</em> the case was built, not just <em>what</em> it found.
          </p>
        </div>
        <div className="actions">
          <a className="btn ghost" href="#">
            Export to PDF
          </a>
          {!isSealed && (
            <button type="button" className="btn" onClick={() => setModalOpen(true)}>
              New entry <span className="arr">&rarr;</span>
            </button>
          )}
        </div>
      </section>

      {/* KPI strip */}
      <KPIStrip items={STUB_KPI_ITEMS} />

      {/* Main panel */}
      <Panel className="inquiry-log-panel">
        {/* Lock/Complete banner */}
        <LockedBanner
          status={inquiryStatus}
          onLock={handleLock}
          onComplete={handleComplete}
        />

        {/* Filter bar */}
        <FilterBar
          searchPlaceholder="decision, linked exhibit, keyword\u2026"
          searchValue={searchQuery}
          onSearchChange={setSearchQuery}
          chips={chips}
          onChipClick={handleChipClick}
        />

        {/* Entry list */}
        <div style={{ padding: 0 }}>
          {filtered.length === 0 && (
            <div
              style={{
                padding: '48px 28px',
                textAlign: 'center',
                color: 'var(--muted)',
              }}
            >
              No inquiry entries match the current filter.
            </div>
          )}
          {days.map((day) => (
            <div key={day.label}>
              <DayHeader label={day.label} />
              {day.entries.map((entry) => (
                <EntryRow key={entry.id} entry={entry} linkable={hasRealData} />
              ))}
            </div>
          ))}
        </div>
      </Panel>

      {/* New entry modal */}
      <NewEntryModal open={modalOpen} onClose={() => setModalOpen(false)} />
    </>
  );
}

export default InquiryView;
