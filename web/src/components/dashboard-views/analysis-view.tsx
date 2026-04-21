'use client';

import { useState } from 'react';
import type { AnalysisNote } from '@/types';
import {
  KPIStrip,
  FilterBar,
  LinkArrow,
  EyebrowLabel,
  StatusPill,
  Tag,
  Modal,
} from '@/components/ui/dashboard';

/* ─── Props ─── */

export interface AnalysisViewProps {
  readonly notes?: AnalysisNote[];
}

/* ─── Stub data matching design prototype ─── */

interface StubNote {
  readonly ref: string;
  readonly title: string;
  readonly author: string;
  readonly avatarColor: 'a' | 'b' | 'c' | 'd' | 'e';
  readonly tags: readonly string[];
  readonly links: string;
  readonly age: string;
  readonly status: 'signed' | 'peer-review' | 'draft';
  readonly excerpt: string;
}

const NOTES: readonly StubNote[] = [
  {
    ref: 'NOTE-184',
    title: 'Command structure \u2014 Unit 28B, April 2024',
    author: 'H. Morel',
    avatarColor: 'a',
    tags: ['hypothesis', 'cmd-structure'],
    links: '14 exhibits \u00b7 3 witnesses',
    age: '2 d',
    status: 'peer-review',
    excerpt: 'Working hypothesis is that S-038 reported through M-112, not directly to regional command. See cross-reference with intercept E-0916 at 14:03 EET\u2026',
  },
  {
    ref: 'NOTE-183',
    title: 'Timeline reconciliation \u2014 19 Apr checkpoint events',
    author: 'Amir H.',
    avatarColor: 'c',
    tags: ['timeline', 'corroborated'],
    links: '9 exhibits \u00b7 4 witnesses',
    age: '3 d',
    status: 'signed',
    excerpt: 'Revised W-0144 intake time from 17:40 to 17:52 after drone pass confirmation at 17:54. Now consistent with W-0139 and body-cam BC-17 excerpt\u2026',
  },
  {
    ref: 'NOTE-182',
    title: 'Unverified \u2014 alleged second convoy',
    author: 'Martyna K.',
    avatarColor: 'b',
    tags: ['uncorroborated', 'open'],
    links: '2 exhibits \u00b7 1 witness',
    age: '5 d',
    status: 'draft',
    excerpt: 'Single source currently (W-0139). Flagging as uncorroborated pending independent confirmation. Intercept E-0916 mentions "second formation" but ambiguous\u2026',
  },
  {
    ref: 'NOTE-181',
    title: 'Financial trail \u2014 Raiffeisen ledger anomalies',
    author: 'W. Nyoka',
    avatarColor: 'e',
    tags: ['financial', 'in-progress'],
    links: '1 exhibit',
    age: '7 d',
    status: 'draft',
    excerpt: 'Three transfers on ledger E-0914 show identical round amounts to intermediary accounts within 72h of checkpoint incident. Requesting OLAF review\u2026',
  },
  {
    ref: 'NOTE-180',
    title: 'Counter-evidence \u2014 defence exculpatory claim',
    author: 'H. Morel',
    avatarColor: 'a',
    tags: ['defence', 'rule-77'],
    links: '3 exhibits',
    age: '9 d',
    status: 'signed',
    excerpt: 'Defence filed motion asserting S-038 was off-duty on 19 Apr. Our drone footage E-0918 contradicts this at 17:54. Nonetheless, per Rule 77, preserving and disclosing\u2026',
  },
  {
    ref: 'NOTE-179',
    title: 'OSINT cross-check \u00b7 Bellingcat satellite trace',
    author: 'Amir H.',
    avatarColor: 'c',
    tags: ['osint', 'peer'],
    links: '4 exhibits',
    age: '11 d',
    status: 'peer-review',
    excerpt: 'Sentinel-2 pass on 18 Apr 09:12Z confirms vehicle concentration at coordinates consistent with W-0144 account. Bellingcat read-only peer has independently noted\u2026',
  },
];

const STATUS_PILL_MAP: Record<string, 'sealed' | 'disc' | 'draft'> = {
  signed: 'sealed',
  'peer-review': 'disc',
  draft: 'draft',
};

const CHIPS = [
  { key: 'all', label: 'All 184', active: true },
  { key: 'signed', label: 'Signed 62' },
  { key: 'peer-review', label: 'Peer-review 38' },
  { key: 'draft', label: 'Draft 84' },
  { key: 'hypothesis', label: 'Hypothesis \u25BE' },
  { key: 'author', label: 'Author \u25BE' },
];

const ANALYSIS_TYPES = [
  'Factual finding',
  'Pattern analysis',
  'Timeline reconstruction',
  'Geographic analysis',
  'Network analysis',
  'Legal assessment',
  'Hypothesis testing',
];

/* ─── Component ─── */

export function AnalysisView({ notes }: AnalysisViewProps) {
  const [modalOpen, setModalOpen] = useState(false);

  const hasRealData = notes !== undefined && notes.length > 0;

  const AVATAR_COLORS: readonly ('a' | 'b' | 'c' | 'd' | 'e')[] = ['a', 'b', 'c', 'd', 'e'];

  const STATUS_MAP: Record<string, 'signed' | 'peer-review' | 'draft'> = {
    final: 'signed',
    peer_review: 'peer-review',
    draft: 'draft',
    superseded: 'draft',
  };

  const displayNotes: readonly StubNote[] = hasRealData
    ? notes.map((n, i): StubNote => ({
        ref: `NOTE-${n.id.slice(0, 4).toUpperCase()}`,
        title: n.title,
        author: n.author_id.slice(0, 10),
        avatarColor: AVATAR_COLORS[i % AVATAR_COLORS.length],
        tags: [n.analysis_type],
        links: `${n.related_evidence_ids.length} exhibits`,
        age: `${Math.max(1, Math.round((Date.now() - new Date(n.created_at).getTime()) / 86400000))} d`,
        status: STATUS_MAP[n.status] ?? 'draft',
        excerpt: n.content.length > 200 ? n.content.slice(0, 200) + '\u2026' : n.content,
      }))
    : NOTES;

  return (
    <>
      <section className="d-pagehead">
        <div>
          <EyebrowLabel>Case &middot; ICC-UKR-2024 &middot; Berkeley Protocol Phase 6</EyebrowLabel>
          <h1>Analysis <em>notes</em></h1>
          <p className="sub">
            Phase 6 of the Berkeley Protocol requires documented analytical reasoning with iterative refinement. Every note is a CRDT document tied to the exhibits it cites. Hypotheses, dead-ends, counter-evidence &mdash; all sealed into the chain so defence counsel sees the reasoning, not just the conclusion.
          </p>
        </div>
        <div className="actions">
          <a className="btn ghost" href="#">Templates</a>
          <button type="button" className="btn" onClick={() => setModalOpen(true)}>
            New note <span className="arr">&rarr;</span>
          </button>
        </div>
      </section>

      <KPIStrip items={[
        { label: 'Notes \u00b7 this case', value: '184', sub: '62 signed \u00b7 38 peer-review \u00b7 84 draft' },
        { label: 'Uncorroborated \u00b7 open', value: '11', delta: '\u25CF flagged to prosecution', deltaNegative: true },
        { label: 'Counter-evidence \u00b7 preserved', value: '7', sub: 'per Rule 77 disclosure' },
        { label: 'Avg. citations', value: '6.2', sub: 'exhibits per note' },
      ]} />

      <div style={{ marginBottom: 22 }} />

      <div className="panel">
        <FilterBar
          searchPlaceholder="reference, tag, author, linked exhibit\u2026"
          chips={CHIPS}
        />
        <div className="panel-body" style={{ padding: 0 }}>
          {displayNotes.map((n) => (
            <div
              key={n.ref}
              style={{
                padding: '22px 28px',
                borderBottom: '1px solid var(--line)',
                display: 'grid',
                gridTemplateColumns: '1fr 220px',
                gap: 32,
                alignItems: 'start',
              }}
            >
              {/* Left: meta + title + excerpt */}
              <div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 10 }}>
                  <span className="ref" style={{ display: 'block' }}>
                    <strong>{n.ref}</strong>
                  </span>
                  <StatusPill status={STATUS_PILL_MAP[n.status] ?? 'draft'}>
                    {n.status}
                  </StatusPill>
                  {n.tags.map((t) => (
                    <Tag key={t}>{t}</Tag>
                  ))}
                </div>
                <a
                  href="#"
                  style={{
                    fontFamily: "'Fraunces', serif",
                    fontSize: 22,
                    letterSpacing: '-.015em',
                    color: 'var(--ink)',
                    lineHeight: 1.2,
                    display: 'block',
                  }}
                >
                  {n.title}
                </a>
                <p
                  style={{
                    color: 'var(--muted)',
                    fontSize: 13.5,
                    lineHeight: 1.6,
                    marginTop: 8,
                    maxWidth: '68ch',
                  }}
                >
                  {n.excerpt}
                </p>
              </div>

              {/* Right: author, links, age, open */}
              <div
                style={{
                  display: 'flex',
                  flexDirection: 'column',
                  gap: 12,
                  alignItems: 'flex-end',
                  fontSize: 12.5,
                  color: 'var(--muted)',
                  textAlign: 'right',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <span className="avs">
                    <span className={`av ${n.avatarColor}`}>
                      {n.author[0]}
                    </span>
                  </span>
                  <span
                    style={{
                      fontFamily: "'Fraunces', serif",
                      fontSize: 14,
                      color: 'var(--ink)',
                      letterSpacing: '-.005em',
                    }}
                  >
                    {n.author}
                  </span>
                </div>
                <div
                  style={{
                    fontFamily: "'JetBrains Mono', monospace",
                    fontSize: 10.5,
                    letterSpacing: '.04em',
                    textTransform: 'uppercase',
                  }}
                >
                  {n.links}
                </div>
                <div
                  style={{
                    fontFamily: "'JetBrains Mono', monospace",
                    fontSize: 10.5,
                  }}
                >
                  {n.age} ago
                </div>
                <LinkArrow href="#">Open</LinkArrow>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* ── New note modal ── */}
      <Modal open={modalOpen} onClose={() => setModalOpen(false)} title="New analysis note" wide>
        <form style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <label style={{ display: 'flex', flexDirection: 'column', gap: 4, fontSize: 13 }}>
            <span style={{ fontWeight: 500, color: 'var(--ink)' }}>Title</span>
            <input
              type="text"
              placeholder="Descriptive title for this analysis note"
              style={{ padding: '8px 10px', border: '1px solid var(--line)', borderRadius: 6, fontSize: 13 }}
            />
          </label>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 4, fontSize: 13 }}>
            <span style={{ fontWeight: 500, color: 'var(--ink)' }}>Analysis type</span>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
              {ANALYSIS_TYPES.map((t) => (
                <button
                  key={t}
                  type="button"
                  className="chip"
                  style={{ cursor: 'pointer' }}
                >
                  {t}
                </button>
              ))}
            </div>
          </div>
          <label style={{ display: 'flex', flexDirection: 'column', gap: 4, fontSize: 13 }}>
            <span style={{ fontWeight: 500, color: 'var(--ink)' }}>Assigned to</span>
            <input
              type="text"
              placeholder="Analyst name"
              style={{ padding: '8px 10px', border: '1px solid var(--line)', borderRadius: 6, fontSize: 13 }}
            />
          </label>
          <label style={{ display: 'flex', flexDirection: 'column', gap: 4, fontSize: 13 }}>
            <span style={{ fontWeight: 500, color: 'var(--ink)' }}>Methodology</span>
            <textarea
              rows={2}
              placeholder="Describe the analytical methodology\u2026"
              style={{ padding: '8px 10px', border: '1px solid var(--line)', borderRadius: 6, fontSize: 13, resize: 'vertical' }}
            />
          </label>
          <label style={{ display: 'flex', flexDirection: 'column', gap: 4, fontSize: 13 }}>
            <span style={{ fontWeight: 500, color: 'var(--ink)' }}>Content</span>
            <textarea
              rows={5}
              placeholder="Analysis content\u2026"
              style={{ padding: '8px 10px', border: '1px solid var(--line)', borderRadius: 6, fontSize: 13, resize: 'vertical' }}
            />
          </label>
          <label style={{ display: 'flex', flexDirection: 'column', gap: 4, fontSize: 13 }}>
            <span style={{ fontWeight: 500, color: 'var(--ink)' }}>Referenced items</span>
            <input
              type="text"
              placeholder="E-XXXX, W-XXXX, C-XXXX (comma-separated)"
              style={{ padding: '8px 10px', border: '1px solid var(--line)', borderRadius: 6, fontSize: 13 }}
            />
          </label>
          <label style={{ display: 'flex', flexDirection: 'column', gap: 4, fontSize: 13 }}>
            <span style={{ fontWeight: 500, color: 'var(--ink)' }}>Limitations</span>
            <textarea
              rows={2}
              placeholder="Known limitations, caveats, or gaps\u2026"
              style={{ padding: '8px 10px', border: '1px solid var(--line)', borderRadius: 6, fontSize: 13, resize: 'vertical' }}
            />
          </label>
          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 10, paddingTop: 8 }}>
            <button type="button" className="btn ghost" onClick={() => setModalOpen(false)}>Cancel</button>
            <button type="submit" className="btn">Create note <span className="arr">&rarr;</span></button>
          </div>
        </form>
      </Modal>
    </>
  );
}

export default AnalysisView;
