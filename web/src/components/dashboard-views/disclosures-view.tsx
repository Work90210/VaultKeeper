'use client';

import type { Disclosure } from '@/types';
import {
  KPIStrip,
  Panel,
  DataTable,
  StatusPill,
  AvatarStack,
  LinkArrow,
  EyebrowLabel,
} from '@/components/ui/dashboard';

/* --- Re-exported types for backward compat --- */

export interface DisclosureWithCase extends Disclosure {
  case_reference: string;
  case_title: string;
  exhibit_count: number;
}

interface DisclosuresViewProps {
  disclosures?: DisclosureWithCase[];
  caseRef?: string;
}

/* --- Stub data matching design prototype --- */

interface DisclosureRow {
  readonly ref: string;
  readonly note: string;
  readonly to: string;
  readonly ex: number;
  readonly st: 'review' | 'sent' | 'draft';
  readonly due: string;
  readonly who: { initial: string; color: 'a' | 'b' | 'c' | 'd' | 'e' }[];
}

const DISCLOSURES: readonly DisclosureRow[] = [
  { ref: 'DISC-2026-019', note: 'redactions pending countersign', to: 'Defence \u00b7 M. Petrenko counsel', ex: 48, st: 'review', due: 'Fri 25 Apr', who: [{ initial: 'A', color: 'a' }, { initial: 'B', color: 'b' }] },
  { ref: 'DISC-2026-018', note: 'ack received \u00b7 9 clarifications', to: 'Prosecution \u00b7 OTP Hague', ex: 182, st: 'sent', due: '18 Apr', who: [{ initial: 'A', color: 'a' }, { initial: 'C', color: 'c' }] },
  { ref: 'DISC-2026-017', note: 'federated sub-chain \u00b7 mirror', to: 'Federated \u00b7 CIJA Berlin', ex: 612, st: 'sent', due: '16 Apr', who: [{ initial: 'A', color: 'a' }] },
  { ref: 'DISC-2026-016', note: 'awaiting exhibit E-0917', to: 'Trial chamber', ex: 12, st: 'draft', due: '\u2014', who: [{ initial: 'B', color: 'b' }] },
  { ref: 'DISC-2026-015', note: 'signed bundle verified', to: 'Defence \u00b7 second-chair', ex: 91, st: 'sent', due: '11 Apr', who: [{ initial: 'A', color: 'a' }, { initial: 'D', color: 'd' }] },
];

const STATUS_MAP: Record<DisclosureRow['st'], 'hold' | 'sealed' | 'draft'> = {
  review: 'hold',
  sent: 'sealed',
  draft: 'draft',
};

const TABLE_COLUMNS = [
  { key: 'bundle', label: 'Bundle' },
  { key: 'recipient', label: 'Recipient' },
  { key: 'exhibits', label: 'Exhibits' },
  { key: 'status', label: 'Status' },
  { key: 'due', label: 'Due' },
  { key: 'owners', label: 'Owners' },
  { key: 'actions', label: '' },
];

interface WizardStep {
  readonly t: string;
  readonly d: string;
  readonly ok: boolean;
  readonly cur: boolean;
}

const WIZARD_STEPS: readonly WizardStep[] = [
  { t: '1 \u00b7 Scope', d: '48 exhibits selected \u00b7 witness statements excluded', ok: true, cur: false },
  { t: '2 \u00b7 Redactions', d: "Applied Martyna's draft v4 \u00b7 12 passages", ok: true, cur: false },
  { t: '3 \u00b7 Countersigns', d: '1 of 2 obtained \u00b7 awaiting H. Morel', ok: false, cur: true },
  { t: '4 \u00b7 Manifest', d: 'SHA-256 \u00b7 BLAKE3 \u00b7 RFC 3161 timestamp', ok: false, cur: false },
  { t: '5 \u00b7 Deliver', d: 'Encrypted bundle + validator \u00b7 4.2 GB', ok: false, cur: false },
];

/* --- Component --- */

export function DisclosuresView(_props: DisclosuresViewProps) {
  return (
    <>
      {/* Page header */}
      <section className="d-pagehead">
        <div>
          <EyebrowLabel>
            Case &middot; ICC-UKR-2024 &middot; Berkeley Protocol Reporting
          </EyebrowLabel>
          <h1>Disclosure <em>bundles</em></h1>
          <p className="sub">
            A disclosure bundle is a one-click ZIP of exhibits, custody log,
            hash manifest, and (where required) redaction maps &mdash; plus
            the open validator binary so the recipient&apos;s clerk can verify
            offline, without talking to us.
          </p>
        </div>
        <div className="actions">
          <a className="btn ghost" href="#">Templates</a>
          <a className="btn" href="#">New disclosure <span className="arr">&rarr;</span></a>
        </div>
      </section>

      {/* KPI strip */}
      <KPIStrip
        items={[
          { label: 'This quarter', value: 19, sub: 'sent across 3 cases' },
          {
            label: 'Pending your review',
            value: 3,
            delta: '\u25cf DISC-2026-019 due Fri',
            deltaNegative: true,
          },
          { label: 'Avg. bundle', value: 214, sub: 'exhibits \u00b7 4.1 GB median' },
          { label: 'Rejected \u00b7 ever', value: 0, sub: 'on custody grounds' },
        ]}
      />

      <div style={{ marginBottom: 22 }} />

      {/* Two-column: table + wizard */}
      <div className="g2-wide">
        {/* Disclosures table */}
        <div className="panel">
          <div className="panel-h">
            <h3>All disclosures</h3>
            <span className="meta">5 of 42</span>
          </div>
          <DataTable columns={TABLE_COLUMNS}>
            {DISCLOSURES.map((d) => (
              <tr key={d.ref}>
                <td>
                  <div className="ref">
                    {d.ref}
                    <small>{d.note}</small>
                  </div>
                </td>
                <td style={{ fontSize: 13, color: 'var(--ink-2)' }}>{d.to}</td>
                <td className="num">{d.ex}</td>
                <td>
                  <StatusPill status={STATUS_MAP[d.st]}>{d.st}</StatusPill>
                </td>
                <td className="mono">{d.due}</td>
                <td>
                  <AvatarStack users={d.who} />
                </td>
                <td className="actions">
                  <LinkArrow href="#">Open</LinkArrow>
                </td>
              </tr>
            ))}
          </DataTable>
        </div>

        {/* Bundle wizard */}
        <Panel
          title="Bundle wizard"
          titleAccent="DISC-2026-019"
          meta="step 3 of 5"
        >
          <div style={{ display: 'flex', flexDirection: 'column', gap: 0 }}>
            {WIZARD_STEPS.map((s, i) => (
              <div
                key={s.t}
                style={{
                  display: 'grid',
                  gridTemplateColumns: '28px 1fr',
                  gap: 14,
                  padding: '12px 0',
                  borderBottom:
                    i < WIZARD_STEPS.length - 1
                      ? '1px solid var(--line)'
                      : 'none',
                }}
              >
                <span
                  style={{
                    width: 24,
                    height: 24,
                    borderRadius: '50%',
                    border: `1.5px solid ${
                      s.ok
                        ? 'var(--ok)'
                        : s.cur
                          ? 'var(--accent)'
                          : 'var(--line-2)'
                    }`,
                    background: s.ok ? 'var(--ok)' : 'transparent',
                    color: s.ok
                      ? '#fff'
                      : s.cur
                        ? 'var(--accent)'
                        : 'var(--muted)',
                    display: 'grid',
                    placeItems: 'center',
                    fontSize: 11,
                    fontFamily: "'JetBrains Mono', monospace",
                  }}
                >
                  {s.ok ? '\u2713' : i + 1}
                </span>
                <div>
                  <strong
                    style={{
                      fontFamily: "'Fraunces', serif",
                      fontSize: 15,
                      color:
                        s.cur || s.ok ? 'var(--ink)' : 'var(--muted)',
                    }}
                  >
                    {s.t}
                  </strong>
                  <div
                    style={{
                      fontSize: 12.5,
                      color: 'var(--muted)',
                      marginTop: 3,
                    }}
                  >
                    {s.d}
                  </div>
                </div>
              </div>
            ))}
          </div>
          <a
            className="btn"
            style={{
              marginTop: 14,
              width: '100%',
              justifyContent: 'center',
            }}
            href="#"
          >
            Continue step 3 <span className="arr">&rarr;</span>
          </a>
        </Panel>
      </div>
    </>
  );
}

export default DisclosuresView;
