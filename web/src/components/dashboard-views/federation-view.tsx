'use client';

import {
  KPIStrip,
  StatusPill,
  Tag,
  KeyValueList,
  LinkArrow,
} from '@/components/ui/dashboard';
import type { FederationPeer, FederationExchange } from '@/lib/federation-api';

// ── Stub data (used when API returns empty) ─────────────────────────────────

const STUB_PEERS: StubPeer[] = [
  {
    n: 'CIJA Berlin',
    r: 'sub-chain \u00b7 /CIJA',
    cases: 4,
    last: '2 min',
    st: 'healthy',
    lag: '0.4 s',
    keys: 'Ed25519 \u00b7 rotated 14 d ago',
    ops: '12,418 ops \u00b7 94 MB',
    note: 'Parallel investigation mirror',
  },
  {
    n: 'KSC Pristina',
    r: 'full peer',
    cases: 2,
    last: '8 min',
    st: 'healthy',
    lag: '1.1 s',
    keys: 'Ed25519 \u00b7 rotated 22 d ago',
    ops: '48,217 ops \u00b7 612 MB',
    note: 'Full bidirectional mirror \u00b7 Kosovo Specialist Chambers',
  },
  {
    n: 'Bellingcat',
    r: 'read-only peer',
    cases: 1,
    last: '11 min',
    st: 'healthy',
    lag: '2.3 s',
    keys: 'Ed25519 \u00b7 rotated 9 d ago',
    ops: '2,104 ops \u00b7 48 MB',
    note: 'OSINT corroboration \u00b7 reads only',
  },
  {
    n: 'OTP Hague archive',
    r: 'cold mirror',
    cases: 38,
    last: '1 h',
    st: 'lag',
    lag: '52 min',
    keys: 'Ed25519 \u00b7 rotated 31 d ago',
    ops: 'nightly snapshot only',
    note: 'Read-only cold mirror for ICC OTP',
  },
  {
    n: 'vke-dev.eurojust',
    r: 'staging',
    cases: 0,
    last: '\u2014',
    st: 'paused',
    lag: '\u2014',
    keys: '\u2014',
    ops: '\u2014',
    note: 'Dev instance \u00b7 federation paused',
  },
];

const STUB_EXCHANGES: StubExchange[] = [
  { time: '14:20:58', peer: 'CIJA Berlin', dir: 'in', desc: '12 ops \u00b7 sub-chain /CIJA', sig: '91ab\u20268204', av: 'a' },
  { time: '14:14:11', peer: 'KSC Pristina', dir: 'out', desc: '4 ops \u00b7 ICC-KOS-2024', sig: 'c44d\u202647e2', av: 'b' },
  { time: '13:58:02', peer: 'Bellingcat', dir: 'in', desc: '1 op \u00b7 OSINT corroboration', sig: '7f22\u20269e14', av: 'c' },
  { time: '13:44:19', peer: 'KSC Pristina', dir: 'in', desc: '9 ops', sig: '3e11\u20269a32', av: 'b' },
  { time: '13:22:08', peer: 'OTP Hague archive', dir: 'out', desc: 'nightly snapshot (delayed)', sig: 'a7f4\u2026b618', av: 'd' },
  { time: '13:04:44', peer: 'CIJA Berlin', dir: 'in', desc: '3 ops', sig: 'd9a7\u20262e18', av: 'a' },
  { time: '12:48:31', peer: 'CIJA Berlin', dir: 'out', desc: 'countersign block', sig: 'f208\u2026bc91', av: 'a' },
  { time: '12:30:02', peer: 'KSC Pristina', dir: 'in', desc: '2 ops', sig: 'e3f0\u2026a1be', av: 'b' },
];

// ── Types ───────────────────────────────────────────────────────────────────

interface StubPeer {
  readonly n: string;
  readonly r: string;
  readonly cases: number;
  readonly last: string;
  readonly st: 'healthy' | 'lag' | 'paused';
  readonly lag: string;
  readonly keys: string;
  readonly ops: string;
  readonly note: string;
}

interface StubExchange {
  readonly time: string;
  readonly peer: string;
  readonly dir: 'in' | 'out';
  readonly desc: string;
  readonly sig: string;
  readonly av: string;
}

type PillStatus = 'sealed' | 'hold' | 'draft';

const STATUS_MAP: Record<string, PillStatus> = {
  healthy: 'sealed',
  lag: 'hold',
  paused: 'draft',
};

// ── Protocol KV data ────────────────────────────────────────────────────────

const PROTOCOL_ITEMS = [
  { label: 'Transport', value: 'HTTPS/2 + mTLS \u00b7 self-signed root per peer' },
  { label: 'Identity', value: 'Ed25519 per instance \u00b7 rotated \u2264 90 d' },
  { label: 'Op format', value: 'CRDT-flavoured, canonical CBOR' },
  { label: 'Seal', value: 'SHA-256 hash chain + RFC 3161 countersign' },
  { label: 'Conflict', value: 'Last-writer-wins on body \u00b7 first-seal-wins on metadata' },
  { label: 'Revocation', value: 'Broadcast to all peers within 30 s' },
];

// ── Helpers ─────────────────────────────────────────────────────────────────

function truncateHash(hash: string): string {
  if (hash.length <= 12) return hash;
  return `${hash.slice(0, 4)}\u2026${hash.slice(-4)}`;
}

function formatRelativeTime(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  if (diff < 0) return 'just now';
  const seconds = Math.floor(diff / 1000);
  if (seconds < 60) return `${seconds} s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes} min ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} h ago`;
  const days = Math.floor(hours / 24);
  return `${days} d ago`;
}

// ── Peer Row ────────────────────────────────────────────────────────────────

function PeerRow({ peer }: { readonly peer: StubPeer }) {
  return (
    <div
      style={{
        padding: '22px 26px',
        borderBottom: '1px solid var(--line)',
        display: 'grid',
        gridTemplateColumns: '1fr auto',
        gap: 18,
        alignItems: 'start',
      }}
    >
      <div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 6 }}>
          <span
            style={{
              fontFamily: "'Fraunces', serif",
              fontSize: 20,
              letterSpacing: '-.01em',
            }}
          >
            {peer.n}
          </span>
          <Tag>{peer.r}</Tag>
          <StatusPill status={STATUS_MAP[peer.st] ?? 'draft'}>{peer.st}</StatusPill>
        </div>
        <div style={{ fontSize: 13, color: 'var(--muted)', marginBottom: 10 }}>{peer.note}</div>
        <div
          style={{
            display: 'flex',
            gap: 18,
            fontFamily: "'JetBrains Mono', monospace",
            fontSize: 11,
            color: 'var(--muted)',
            letterSpacing: '.02em',
          }}
        >
          <span>{peer.cases} cases</span>
          <span>{peer.ops}</span>
          <span>{peer.keys}</span>
        </div>
      </div>
      <div style={{ textAlign: 'right' }}>
        <div
          style={{
            fontFamily: "'JetBrains Mono', monospace",
            fontSize: 11,
            color: 'var(--muted)',
            letterSpacing: '.04em',
            textTransform: 'uppercase',
          }}
        >
          last exchange
        </div>
        <div
          style={{
            fontFamily: "'Fraunces', serif",
            fontSize: 18,
            letterSpacing: '-.005em',
          }}
        >
          {peer.last}
        </div>
        <div
          style={{
            fontFamily: "'JetBrains Mono', monospace",
            fontSize: 10.5,
            color: 'var(--muted)',
            marginTop: 2,
          }}
        >
          lag {peer.lag}
        </div>
      </div>
    </div>
  );
}

// ── Live Peer Row (from API data) ───────────────────────────────────────────

const TRUST_MODE_LABELS: Record<string, string> = {
  tofu_pending: 'TOFU pending',
  manual_pinned: 'pinned',
  pki_verified: 'PKI verified',
  revoked: 'revoked',
};

const TRUST_MODE_TO_PL: Record<string, PillStatus> = {
  tofu_pending: 'hold',
  manual_pinned: 'sealed',
  pki_verified: 'sealed',
  revoked: 'draft',
};

function LivePeerRow({ peer }: { readonly peer: FederationPeer }) {
  return (
    <div
      style={{
        padding: '22px 26px',
        borderBottom: '1px solid var(--line)',
        display: 'grid',
        gridTemplateColumns: '1fr auto',
        gap: 18,
        alignItems: 'start',
      }}
    >
      <div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 6 }}>
          <span
            style={{
              fontFamily: "'Fraunces', serif",
              fontSize: 20,
              letterSpacing: '-.01em',
            }}
          >
            {peer.display_name}
          </span>
          <Tag>{peer.instance_id}</Tag>
          <StatusPill status={TRUST_MODE_TO_PL[peer.trust_mode] ?? 'draft'}>
            {TRUST_MODE_LABELS[peer.trust_mode] ?? peer.trust_mode}
          </StatusPill>
        </div>
        {peer.well_known_url && (
          <div style={{ fontSize: 13, color: 'var(--muted)', marginBottom: 10 }}>
            {peer.well_known_url}
          </div>
        )}
        <div
          style={{
            display: 'flex',
            gap: 18,
            fontFamily: "'JetBrains Mono', monospace",
            fontSize: 11,
            color: 'var(--muted)',
            letterSpacing: '.02em',
          }}
        >
          {peer.verification_channel && <span>{peer.verification_channel}</span>}
          <span>created {formatRelativeTime(peer.created_at)}</span>
        </div>
      </div>
      <div style={{ textAlign: 'right' }}>
        <div
          style={{
            fontFamily: "'JetBrains Mono', monospace",
            fontSize: 11,
            color: 'var(--muted)',
            letterSpacing: '.04em',
            textTransform: 'uppercase',
          }}
        >
          last updated
        </div>
        <div
          style={{
            fontFamily: "'Fraunces', serif",
            fontSize: 18,
            letterSpacing: '-.005em',
          }}
        >
          {formatRelativeTime(peer.updated_at)}
        </div>
      </div>
    </div>
  );
}

// ── Exchange Row ────────────────────────────────────────────────────────────

function ExchangeRow({ exchange }: { readonly exchange: StubExchange }) {
  return (
    <div
      style={{
        padding: '14px 26px',
        borderBottom: '1px solid var(--line)',
        display: 'grid',
        gridTemplateColumns: '90px 40px 1fr auto auto',
        gap: 18,
        alignItems: 'center',
        fontSize: 13,
      }}
    >
      <span
        style={{
          fontFamily: "'JetBrains Mono', monospace",
          color: 'var(--muted)',
        }}
      >
        {exchange.time}
      </span>
      <span className="avs">
        <span className={`av ${exchange.av}`}>{exchange.peer[0]}</span>
      </span>
      <span
        style={{
          fontFamily: "'Fraunces', serif",
          fontSize: 14.5,
          letterSpacing: '-.005em',
        }}
      >
        {exchange.peer}{' '}
        <span style={{ color: 'var(--muted)', fontSize: 12 }}>\u00b7 {exchange.desc}</span>
      </span>
      <StatusPill status={exchange.dir === 'in' ? 'sealed' : 'disc'}>
        <span style={{ textTransform: 'uppercase' }}>
          {exchange.dir === 'in' ? '\u2190 in' : 'out \u2192'}
        </span>
      </StatusPill>
      <span
        style={{
          fontFamily: "'JetBrains Mono', monospace",
          color: 'var(--accent)',
          fontSize: 11.5,
        }}
      >
        {exchange.sig}
      </span>
    </div>
  );
}

function LiveExchangeRow({ exchange }: { readonly exchange: FederationExchange }) {
  const initial = exchange.peer_display_name?.[0] ?? '?';
  return (
    <div
      style={{
        padding: '14px 26px',
        borderBottom: '1px solid var(--line)',
        display: 'grid',
        gridTemplateColumns: '90px 40px 1fr auto auto',
        gap: 18,
        alignItems: 'center',
        fontSize: 13,
      }}
    >
      <span
        style={{
          fontFamily: "'JetBrains Mono', monospace",
          color: 'var(--muted)',
        }}
      >
        {new Date(exchange.created_at).toLocaleTimeString('en-GB', {
          hour: '2-digit',
          minute: '2-digit',
          second: '2-digit',
        })}
      </span>
      <span className="avs">
        <span className={`av ${initial.toLowerCase()}`}>{initial}</span>
      </span>
      <span
        style={{
          fontFamily: "'Fraunces', serif",
          fontSize: 14.5,
          letterSpacing: '-.005em',
        }}
      >
        {exchange.peer_display_name || exchange.peer_instance_id}{' '}
        <span style={{ color: 'var(--muted)', fontSize: 12 }}>
          \u00b7 {exchange.scope_cardinality} items \u00b7 {exchange.status}
        </span>
      </span>
      <StatusPill status={exchange.direction === 'incoming' ? 'sealed' : 'disc'}>
        <span style={{ textTransform: 'uppercase' }}>
          {exchange.direction === 'incoming' ? '\u2190 in' : 'out \u2192'}
        </span>
      </StatusPill>
      <span
        style={{
          fontFamily: "'JetBrains Mono', monospace",
          color: 'var(--accent)',
          fontSize: 11.5,
        }}
      >
        {truncateHash(exchange.manifest_hash)}
      </span>
    </div>
  );
}

// ── Main View ───────────────────────────────────────────────────────────────

interface FederationViewProps {
  peers: FederationPeer[];
  exchanges: FederationExchange[];
}

export function FederationView({ peers, exchanges }: FederationViewProps) {
  const useLiveData = peers.length > 0;
  const useStubExchanges = exchanges.length === 0;

  // KPI calculations
  const healthyCount = useLiveData
    ? peers.filter((p) => p.trust_mode === 'pki_verified' || p.trust_mode === 'manual_pinned').length
    : STUB_PEERS.filter((p) => p.st === 'healthy').length;
  const lagCount = useLiveData
    ? peers.filter((p) => p.trust_mode === 'tofu_pending').length
    : STUB_PEERS.filter((p) => p.st === 'lag').length;
  const pausedCount = useLiveData
    ? peers.filter((p) => p.trust_mode === 'revoked').length
    : STUB_PEERS.filter((p) => p.st === 'paused').length;
  const totalPeers = useLiveData ? peers.length : STUB_PEERS.length;
  const federatedCases = useLiveData ? 6 : 6;
  const totalCases = 38;

  return (
    <>
      {/* Page header */}
      <section className="d-pagehead">
        <div>
          <span className="eyebrow-m">Platform \u00b7 VKE1 federation protocol</span>
          <h1>
            Federated <em>chain of custody</em>
          </h1>
          <p className="sub">
            Peer instances exchange sealed, signed operations over the VKE1 protocol
            \u2014 no shared database, no central ledger, no single trust anchor. Each
            party keeps their own chain; cross-chain merges are themselves sealed events
            on both sides.
          </p>
        </div>
        <div className="actions">
          <a className="btn ghost" href="#">
            Protocol spec
          </a>
          <a className="btn" href="#">
            Invite peer <span className="arr">\u2192</span>
          </a>
        </div>
      </section>

      {/* KPI strip */}
      <KPIStrip
        items={[
          {
            label: 'Active peers',
            value: totalPeers,
            sub: `${healthyCount} healthy \u00b7 ${lagCount} lagging \u00b7 ${pausedCount} paused`,
          },
          {
            label: 'Federated cases',
            value: federatedCases,
            sub: `of ${totalCases} total`,
          },
          {
            label: 'Merge p95 \u00b7 24h',
            value: '1.4s',
            sub: 'roundtrip seal',
          },
          {
            label: 'Divergences \u00b7 ever',
            value: 0,
            sub: 'no reconciliation needed',
          },
        ]}
      />

      {/* Two-column: peers + protocol */}
      <div className="g2-wide" style={{ marginTop: 22, marginBottom: 22 }}>
        {/* Peers panel */}
        <div className="panel">
          <div className="panel-h">
            <h3>Peer instances</h3>
            <span className="meta">{totalPeers} registered</span>
          </div>
          <div className="panel-body" style={{ padding: 0 }}>
            {useLiveData
              ? peers.map((p) => <LivePeerRow key={p.id} peer={p} />)
              : STUB_PEERS.map((p) => <PeerRow key={p.n} peer={p} />)}
            {totalPeers === 0 && (
              <div style={{ padding: '22px 26px', color: 'var(--muted)', fontSize: 13 }}>
                No peers registered yet.
              </div>
            )}
          </div>
        </div>

        {/* Protocol panel */}
        <div className="panel">
          <div className="panel-h">
            <h3>Protocol</h3>
            <span className="meta">VKE1 \u00b7 open spec</span>
          </div>
          <div className="panel-body">
            <KeyValueList items={PROTOCOL_ITEMS} />
            <div style={{ marginTop: 14 }}>
              <LinkArrow href="#">RFC VKE1 \u2192</LinkArrow>
            </div>
          </div>
        </div>
      </div>

      {/* Recent exchanges table */}
      <div className="panel">
        <div className="panel-h">
          <h3>Recent exchanges</h3>
          <span className="meta">
            {useStubExchanges ? 'last 8' : `${exchanges.length} recorded`}
          </span>
        </div>
        <div className="panel-body" style={{ padding: 0 }}>
          {useStubExchanges
            ? STUB_EXCHANGES.map((e) => <ExchangeRow key={`${e.time}-${e.peer}`} exchange={e} />)
            : exchanges.map((e) => <LiveExchangeRow key={e.exchange_id} exchange={e} />)}
          {!useStubExchanges && exchanges.length === 0 && (
            <div style={{ padding: '14px 26px', color: 'var(--muted)', fontSize: 13 }}>
              No exchanges recorded yet.
            </div>
          )}
        </div>
      </div>
    </>
  );
}

export default FederationView;
