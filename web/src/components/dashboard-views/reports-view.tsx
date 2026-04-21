'use client';

import type { InvestigationReport } from '@/types';
import {
  DataTable,
  Tag,
  LinkArrow,
  EyebrowLabel,
  AvatarStack,
} from '@/components/ui/dashboard';

/* --- Re-exported types for backward compat --- */

export interface ReportWithCase extends InvestigationReport {
  case_reference: string;
  case_title: string;
}

interface ReportsViewProps {
  reports?: ReportWithCase[];
  caseRef?: string;
}

/* --- Stub data matching design prototype --- */

interface ReportTemplate {
  readonly n: string;
  readonly d: string;
  readonly ic: string;
  readonly used: string;
  readonly t: string;
}

const TEMPLATES: readonly ReportTemplate[] = [
  {
    n: 'Custody summary',
    d: 'PDF/A with full hash chain, signature ledger, and RFC 3161 timestamps per exhibit \u2014 paginated for filing.',
    ic: '\u00b6',
    used: '194 generated',
    t: 'standard',
  },
  {
    n: 'Disclosure dossier',
    d: 'Bundled ZIP for defence/prosecution: exhibits, redaction maps, custody summary, and offline validator.',
    ic: '\u25e9',
    used: '42 generated',
    t: 'standard',
  },
  {
    n: 'Ceremony minutes',
    d: 'Break-the-glass and quorum events with timestamps, quorum members, and biometric attestations.',
    ic: '\u25c6',
    used: '11 generated',
    t: 'governance',
  },
  {
    n: 'Quarterly retention report',
    d: "What\u2019s sealed, what\u2019s held, what\u2019s due for review. Filed automatically with supervisory board.",
    ic: '\u25a4',
    used: '24 generated',
    t: 'governance',
  },
  {
    n: 'Counter-evidence preserved',
    d: 'Rule 77 disclosure: all preserved exculpatory material, per-case, with reasoning notes.',
    ic: '\u21c5',
    used: '8 generated',
    t: 'legal',
  },
  {
    n: 'Federation diff',
    d: 'Cross-chain reconciliation between peer instances \u2014 CIJA, KSC, Bellingcat \u2014 with divergence log.',
    ic: '\u29d6',
    used: '3 generated',
    t: 'platform',
  },
];

interface RecentReport {
  readonly ref: string;
  readonly caseRef: string;
  readonly author: string;
  readonly avatarColor: 'a' | 'b' | 'c' | 'd' | 'e';
  readonly template: string;
  readonly hash: string;
  readonly date: string;
}

const RECENT: readonly RecentReport[] = [
  { ref: 'RPT-2026-0418', caseRef: 'ICC-UKR-2024', author: 'H. Morel', avatarColor: 'a', template: 'Custody summary', hash: 'f208\u2026bc91', date: '2 h ago' },
  { ref: 'RPT-2026-0417', caseRef: 'ICC-UKR-2024', author: 'W. Nyoka', avatarColor: 'e', template: 'Counter-evidence preserved', hash: '7f22\u20269e14', date: '1 d ago' },
  { ref: 'RPT-2026-0416', caseRef: 'ICC-UKR-2024', author: 'Martyna K.', avatarColor: 'b', template: 'Disclosure dossier (DISC-018)', hash: 'c44d\u202647e2', date: '3 d ago' },
  { ref: 'RPT-2026-0415', caseRef: 'Platform', author: 'system', avatarColor: 'e', template: 'Federation diff', hash: '91ab\u20268204', date: '5 d ago' },
  { ref: 'RPT-2026-0414', caseRef: 'Eurojust', author: 'H. Morel', avatarColor: 'a', template: 'Quarterly retention', hash: '3e11\u20269a32', date: '9 d ago' },
];

const RECENT_TABLE_COLUMNS = [
  { key: 'report', label: 'Report' },
  { key: 'case', label: 'Case' },
  { key: 'by', label: 'By' },
  { key: 'template', label: 'Template' },
  { key: 'hash', label: 'Hash' },
  { key: 'generated', label: 'Generated' },
  { key: 'actions', label: '' },
];

/* --- Component --- */

export function ReportsView(_props: ReportsViewProps) {
  return (
    <>
      {/* Page header */}
      <section className="d-pagehead">
        <div>
          <EyebrowLabel>
            Case &middot; ICC-UKR-2024 &middot; Berkeley Protocol Reporting
          </EyebrowLabel>
          <h1>Signed <em>reports</em></h1>
          <p className="sub">
            Reports are sealed snapshots, not live documents. Once generated
            they are written to the chain, cryptographically frozen, and
            recipients can verify them offline with the open validator &mdash;
            even if VaultKeeper disappears tomorrow.
          </p>
        </div>
        <div className="actions">
          <a className="btn ghost" href="#">Report history</a>
          <a className="btn" href="#">New report <span className="arr">&rarr;</span></a>
        </div>
      </section>

      {/* Template grid */}
      <div className="panel" style={{ marginBottom: 22 }}>
        <div className="panel-h">
          <h3>Report templates</h3>
          <span className="meta">6 standard &middot; 3 custom</span>
        </div>
        <div
          className="panel-body"
          style={{
            padding: 0,
            display: 'grid',
            gridTemplateColumns: 'repeat(3, 1fr)',
            gap: 0,
          }}
        >
          {TEMPLATES.map((tmpl, i) => (
            <div
              key={tmpl.n}
              style={{
                padding: 28,
                borderRight: (i + 1) % 3 ? '1px solid var(--line)' : 'none',
                borderBottom: i < 3 ? '1px solid var(--line)' : 'none',
                cursor: 'pointer',
                transition: 'background .2s',
              }}
            >
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  marginBottom: 16,
                }}
              >
                <span
                  style={{
                    width: 40,
                    height: 40,
                    borderRadius: 10,
                    background: 'var(--paper)',
                    border: '1px solid var(--line)',
                    display: 'grid',
                    placeItems: 'center',
                    fontFamily: "'Fraunces', serif",
                    fontSize: 20,
                    color: 'var(--accent)',
                  }}
                >
                  {tmpl.ic}
                </span>
                <Tag>{tmpl.t}</Tag>
              </div>
              <div
                style={{
                  fontFamily: "'Fraunces', serif",
                  fontSize: 22,
                  letterSpacing: '-.015em',
                  lineHeight: 1.2,
                  marginBottom: 8,
                }}
              >
                {tmpl.n}
              </div>
              <p
                style={{
                  color: 'var(--muted)',
                  fontSize: 13.5,
                  lineHeight: 1.55,
                  marginBottom: 14,
                }}
              >
                {tmpl.d}
              </p>
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  paddingTop: 14,
                  borderTop: '1px solid var(--line)',
                  fontFamily: "'JetBrains Mono', monospace",
                  fontSize: 10.5,
                  letterSpacing: '.06em',
                  textTransform: 'uppercase',
                  color: 'var(--muted)',
                }}
              >
                <span>{tmpl.used}</span>
                <LinkArrow href="#">Generate</LinkArrow>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Recently generated table */}
      <div className="panel">
        <div className="panel-h">
          <h3>Recently generated</h3>
          <span className="meta">last 5 &middot; sealed</span>
        </div>
        <DataTable columns={RECENT_TABLE_COLUMNS}>
          {RECENT.map((r) => (
            <tr key={r.ref}>
              <td className="ref">
                <strong>{r.ref}</strong>
              </td>
              <td style={{ fontSize: 13.5 }}>{r.caseRef}</td>
              <td>
                <span
                  style={{
                    display: 'inline-flex',
                    alignItems: 'center',
                    gap: 8,
                  }}
                >
                  <AvatarStack
                    users={[
                      { initial: r.author[0], color: r.avatarColor },
                    ]}
                  />
                  <span style={{ fontSize: 13.5 }}>{r.author}</span>
                </span>
              </td>
              <td style={{ fontSize: 13.5, color: 'var(--muted)' }}>
                {r.template}
              </td>
              <td className="mono" style={{ color: 'var(--accent)' }}>
                {r.hash}
              </td>
              <td className="mono">{r.date}</td>
              <td className="actions">
                <LinkArrow href="#">Verify</LinkArrow>
              </td>
            </tr>
          ))}
        </DataTable>
      </div>
    </>
  );
}

export default ReportsView;
