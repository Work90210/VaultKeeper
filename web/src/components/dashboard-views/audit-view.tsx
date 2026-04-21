'use client';

import {
  KPIStrip,
  Panel,
  DataTable,
  FilterBar,
  StatusPill,
  BarChart,
  EyebrowLabel,
} from '@/components/ui/dashboard';

/* ─── Stub data matching design prototype ─── */

interface AuditEvent {
  readonly t: string;
  readonly who: string;
  readonly action: string;
  readonly target: string;
  readonly sig: string;
  readonly avatarColor: 'a' | 'b' | 'c' | 'd' | 'e';
  readonly kind: string;
}

const EVENTS: readonly AuditEvent[] = [
  { t: '14:22:04', who: 'ts-eu-west', action: 'RFC 3161 timestamp', target: 'block f208\u2026bc91', sig: 'ts:9fa1', avatarColor: 'e', kind: 'seal' },
  { t: '14:22:02', who: 'H. Morel', action: 'sealed evidence upload', target: 'E-0918 Butcha_drone_04.mp4 \u00b7 218 MB', sig: 'ed25519:7f22', avatarColor: 'a', kind: 'ingest' },
  { t: '14:21:58', who: 'Amir H.', action: 'corroboration score assigned 0.78', target: 'C-0412', sig: 'ed25519:bb09', avatarColor: 'c', kind: 'invest' },
  { t: '14:21:44', who: 'W. Nyoka', action: 'strike \u2014 witness-identifying', target: 'E-0912 p2', sig: 'ed25519:e3f1', avatarColor: 'e', kind: 'redact' },
  { t: '14:21:30', who: 'Juliane', action: 'pseudonymise Col. M. \u2192 S-038', target: 'E-0912', sig: 'ed25519:a1be', avatarColor: 'd', kind: 'pseudo' },
  { t: '14:21:18', who: 'Amir H.', action: 'edit timestamp 17:40 \u2192 17:52', target: 'E-0912 p2', sig: 'ed25519:2c14', avatarColor: 'c', kind: 'edit' },
  { t: '14:21:11', who: 'Martyna', action: 'geo-fuzz 50 km \u00b7 "Andriivka"', target: 'E-0912', sig: 'ed25519:d9a7', avatarColor: 'b', kind: 'redact' },
  { t: '14:20:58', who: 'CIJA (federated)', action: 'submit 12 CRDT ops', target: 'sub-chain /CIJA', sig: 'vke1:91ab', avatarColor: 'e', kind: 'fed' },
  { t: '14:20:12', who: 'witness-node-02', action: 'countersign block', target: '91ab\u20268204', sig: 'ed25519:cc20', avatarColor: 'e', kind: 'seal' },
  { t: '14:19:44', who: 'W. Nyoka', action: 'linked exhibit', target: 'E-0412 \u2194 W-0144', sig: 'ed25519:d9fa', avatarColor: 'e', kind: 'link' },
  { t: '14:18:02', who: 'H. Morel', action: 'opened disclosure wizard', target: 'DISC-2026-019', sig: 'ed25519:7e12', avatarColor: 'a', kind: 'disc' },
  { t: '14:17:21', who: 'api-key \u00b7 clerk-verify', action: 'chain head read', target: 'head f208\u2026', sig: 'api:4a91', avatarColor: 'e', kind: 'read' },
];

const KIND_PILL: Record<string, 'sealed' | 'live' | 'disc' | 'hold' | 'pseud' | 'draft'> = {
  seal: 'sealed',
  ingest: 'live',
  invest: 'disc',
  redact: 'hold',
  pseudo: 'pseud',
  edit: 'draft',
  fed: 'live',
  link: 'sealed',
  disc: 'hold',
  read: 'draft',
};

const TABLE_COLUMNS = [
  { key: 'time', label: 'Time' },
  { key: 'actor', label: 'Actor' },
  { key: 'action', label: 'Action' },
  { key: 'target', label: 'Target' },
  { key: 'kind', label: 'Kind' },
  { key: 'signature', label: 'Signature' },
];

const CHIPS = [
  { key: 'all', label: 'All', active: true },
  { key: 'ingest', label: 'Ingest' },
  { key: 'redaction', label: 'Redaction' },
  { key: 'pseudonym', label: 'Pseudonym' },
  { key: 'federation', label: 'Federation' },
  { key: 'disclosure', label: 'Disclosure' },
  { key: 'actor', label: 'Actor \u25BE' },
  { key: 'range', label: 'Range \u00b7 24h \u25BE' },
];

const ACTOR_SHARE = [
  { label: 'H. Morel', value: 1219 },
  { label: 'Martyna K.', value: 914 },
  { label: 'Amir H.', value: 685 },
  { label: 'Juliane', value: 381 },
  { label: 'CIJA (federated)', value: 343 },
  { label: 'witness-nodes', value: 270 },
];

/* ─── Component ─── */

export function AuditView() {
  return (
    <>
      <section className="d-pagehead">
        <div>
          <EyebrowLabel>Case &middot; ICC-UKR-2024 &middot; full chain</EyebrowLabel>
          <h1>Audit <em>log</em></h1>
          <p className="sub">
            Every row below was written to an append-only PostgreSQL table the DB itself refuses to UPDATE or DELETE. Each row hash-chains the previous. Replay any window with the open validator &mdash; no VaultKeeper service required.
          </p>
        </div>
        <div className="actions">
          <a className="btn ghost" href="#">Verify offline</a>
          <a className="btn" href="#">Export range <span className="arr">&rarr;</span></a>
        </div>
      </section>

      <KPIStrip items={[
        { label: 'Events \u00b7 this case', value: '48.2k', sub: 'since 14 Feb 2024' },
        { label: 'Chain integrity', value: '100%', sub: '0 breaks \u00b7 last verified 14:22' },
        { label: 'Snapshot cadence', value: '60s', sub: 'compacted \u00b7 BLAKE3 root' },
        { label: 'Last countersign', value: 'witness-node-02', sub: 'cc20\u2026 \u00b7 2 min ago' },
      ]} />

      <div style={{ marginBottom: 22 }} />

      <div className="panel" style={{ marginBottom: 22 }}>
        <FilterBar
          searchPlaceholder="hash, actor, exhibit, signature\u2026"
          chips={CHIPS}
        />
        <DataTable columns={TABLE_COLUMNS}>
          {EVENTS.map((ev) => {
            const pillStatus = KIND_PILL[ev.kind] ?? 'draft';
            return (
              <tr key={`${ev.t}-${ev.kind}-${ev.sig}`}>
                <td className="mono">{ev.t}</td>
                <td>
                  <span style={{ display: 'inline-flex', alignItems: 'center', gap: 8 }}>
                    <span className="avs">
                      <span
                        className={`av ${ev.avatarColor}`}
                        style={{ width: 22, height: 22, fontSize: 10, borderWidth: '1.5px' }}
                      >
                        {ev.who[0].toUpperCase()}
                      </span>
                    </span>
                    {ev.who}
                  </span>
                </td>
                <td style={{ fontSize: 13.5 }}>{ev.action}</td>
                <td className="mono" style={{ color: 'var(--ink-2)' }}>{ev.target}</td>
                <td>
                  <StatusPill status={pillStatus}>{ev.kind}</StatusPill>
                </td>
                <td className="mono" style={{ color: 'var(--accent)' }}>{ev.sig}</td>
              </tr>
            );
          })}
        </DataTable>
      </div>

      <div className="g2">
        <Panel title="Chain verify" meta="client-side \u00b7 Ed25519 + SHA-256">
          <pre
            style={{
              background: '#0a0907',
              color: 'var(--bg)',
              borderRadius: 10,
              padding: 18,
              fontFamily: "'JetBrains Mono', monospace",
              fontSize: 11.5,
              lineHeight: 1.7,
              overflow: 'auto',
              margin: 0,
            }}
          >
            {'$ vk-verify --case ICC-UKR-2024 --range last-24h\n'}
            <span style={{ color: '#9cb28b' }}>[ok]</span>{'  read 3,812 events from sealed log\n'}
            <span style={{ color: '#9cb28b' }}>[ok]</span>{'  reconstructed hash chain (BLAKE3)\n'}
            <span style={{ color: '#9cb28b' }}>[ok]</span>{'  verified 3,812 Ed25519 signatures\n'}
            <span style={{ color: '#9cb28b' }}>[ok]</span>{'  verified 63 RFC 3161 timestamps\n'}
            <span style={{ color: '#9cb28b' }}>[ok]</span>{'  4 federated merges \u00b7 countersigned\n'}
            <span style={{ color: '#e4a487' }}>head</span>{'  f208e1a94bc91...\n'}
            <span style={{ color: '#9cb28b' }}>PASS</span>{'  no chain breaks \u00b7 0 tampered rows'}
          </pre>
        </Panel>

        <Panel title="Actor share \u00b7 24h" meta="3,812 events">
          <BarChart bars={ACTOR_SHARE} />
        </Panel>
      </div>
    </>
  );
}

export default AuditView;
