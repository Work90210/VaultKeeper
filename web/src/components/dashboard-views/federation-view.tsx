import type { FederationPeer, FederationExchange } from '@/lib/federation-api';

const TRUST_MODE_LABELS: Record<string, string> = {
  tofu_pending: 'TOFU pending',
  manual_pinned: 'pinned',
  pki_verified: 'PKI verified',
  revoked: 'revoked',
};

const TRUST_MODE_TO_PL: Record<string, string> = {
  tofu_pending: 'hold',
  manual_pinned: 'sealed',
  pki_verified: 'sealed',
  revoked: 'draft',
};

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

function truncateHash(hash: string): string {
  if (hash.length <= 12) return hash;
  return `${hash.slice(0, 4)}…${hash.slice(-4)}`;
}

interface FederationViewProps {
  peers: FederationPeer[];
  exchanges: FederationExchange[];
}

export default function FederationView({ peers, exchanges }: FederationViewProps) {
  const healthyCount = peers.filter((p) => p.trust_mode === 'pki_verified' || p.trust_mode === 'manual_pinned').length;
  const pendingCount = peers.filter((p) => p.trust_mode === 'tofu_pending').length;
  const revokedCount = peers.filter((p) => p.trust_mode === 'revoked').length;

  return (
    <>
      <section className="d-pagehead">
        <div>
          <span className="eyebrow-m">Platform · VKE1 federation protocol</span>
          <h1>Federated <em>chain of custody</em></h1>
          <p className="sub">Peer instances exchange sealed, signed operations over the VKE1 protocol — no shared database, no central ledger, no single trust anchor. Each party keeps their own chain; cross-chain merges are themselves sealed events on both sides.</p>
        </div>
        <div className="actions">
          <a className="btn ghost" href="#">Protocol spec</a>
          <a className="btn" href="#">Invite peer <span className="arr">→</span></a>
        </div>
      </section>

      <div className="d-kpis" style={{ marginBottom: 22 }}>
        <div className="d-kpi"><div className="k">Active peers</div><div className="v">{peers.length}</div><div className="sub">{healthyCount} verified · {pendingCount} pending · {revokedCount} revoked</div></div>
        <div className="d-kpi"><div className="k">Exchanges</div><div className="v">{exchanges.length}</div><div className="sub">total recorded</div></div>
        <div className="d-kpi"><div className="k">Merge p95 · 24h</div><div className="v">—</div><div className="sub">roundtrip seal</div></div>
        <div className="d-kpi"><div className="k">Divergences · ever</div><div className="v">0</div><div className="sub">no reconciliation needed</div></div>
      </div>

      <div className="g2-wide" style={{ marginBottom: 22 }}>
        <div className="panel">
          <div className="panel-h"><h3>Peer instances</h3><span className="meta">{peers.length} registered</span></div>
          <div className="panel-body" style={{ padding: 0 }}>
            {peers.length === 0 && (
              <div style={{ padding: '22px 26px', color: 'var(--muted)', fontSize: 13 }}>
                No peers registered yet.
              </div>
            )}
            {peers.map((p) => (
              <div key={p.id} style={{ padding: '22px 26px', borderBottom: '1px solid var(--line)', display: 'grid', gridTemplateColumns: '1fr auto', gap: 18, alignItems: 'start' }}>
                <div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 6 }}>
                    <span style={{ fontFamily: "'Fraunces',serif", fontSize: 20, letterSpacing: '-.01em' }}>{p.display_name}</span>
                    <span className="tag">{p.instance_id}</span>
                    <span className={`pl ${TRUST_MODE_TO_PL[p.trust_mode] ?? 'draft'}`}>{TRUST_MODE_LABELS[p.trust_mode] ?? p.trust_mode}</span>
                  </div>
                  {p.well_known_url && (
                    <div style={{ fontSize: 13, color: 'var(--muted)', marginBottom: 10 }}>{p.well_known_url}</div>
                  )}
                  <div style={{ display: 'flex', gap: 18, fontFamily: "'JetBrains Mono',monospace", fontSize: 11, color: 'var(--muted)', letterSpacing: '.02em' }}>
                    {p.verification_channel && <span>{p.verification_channel}</span>}
                    <span>created {formatRelativeTime(p.created_at)}</span>
                  </div>
                </div>
                <div style={{ textAlign: 'right' }}>
                  <div style={{ fontFamily: "'JetBrains Mono',monospace", fontSize: 11, color: 'var(--muted)', letterSpacing: '.04em', textTransform: 'uppercase' }}>last updated</div>
                  <div style={{ fontFamily: "'Fraunces',serif", fontSize: 18, letterSpacing: '-.005em' }}>{formatRelativeTime(p.updated_at)}</div>
                </div>
              </div>
            ))}
          </div>
        </div>

        <div className="panel">
          <div className="panel-h"><h3>Protocol</h3><span className="meta">VKE1 · open spec</span></div>
          <div className="panel-body">
            <dl className="kvs">
              <dt>Transport</dt><dd>HTTPS/2 + mTLS · self-signed root per peer</dd>
              <dt>Identity</dt><dd>Ed25519 per instance · rotated ≤ 90 d</dd>
              <dt>Op format</dt><dd>CRDT-flavoured, canonical CBOR</dd>
              <dt>Seal</dt><dd>SHA-256 hash chain + RFC 3161 countersign</dd>
              <dt>Conflict</dt><dd>Last-writer-wins on body · first-seal-wins on metadata</dd>
              <dt>Revocation</dt><dd>Broadcast to all peers within 30 s</dd>
              <dt>Governance</dt><dd>Spec maintained in public · <a className="linkarrow" href="#">RFC VKE1 →</a></dd>
            </dl>
          </div>
        </div>
      </div>

      <div className="panel">
        <div className="panel-h"><h3>Recent exchanges</h3><span className="meta">{exchanges.length} recorded</span></div>
        <div className="panel-body" style={{ padding: 0 }}>
          {exchanges.length === 0 && (
            <div style={{ padding: '14px 26px', color: 'var(--muted)', fontSize: 13 }}>
              No exchanges recorded yet.
            </div>
          )}
          {exchanges.map((e) => (
            <div key={e.exchange_id} style={{ padding: '14px 26px', borderBottom: '1px solid var(--line)', display: 'grid', gridTemplateColumns: '90px 40px 1fr auto auto', gap: 18, alignItems: 'center', fontSize: 13 }}>
              <span className="mono" style={{ fontFamily: "'JetBrains Mono',monospace", color: 'var(--muted)' }}>
                {new Date(e.created_at).toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
              </span>
              <span className="avs"><span className={`av ${e.peer_display_name?.[0]?.toLowerCase() ?? 'a'}`}>{e.peer_display_name?.[0] ?? '?'}</span></span>
              <span style={{ fontFamily: "'Fraunces',serif", fontSize: 14.5, letterSpacing: '-.005em' }}>
                {e.peer_display_name || e.peer_instance_id}{' '}
                <span style={{ color: 'var(--muted)', fontSize: 12 }}>· {e.scope_cardinality} items · {e.status}</span>
              </span>
              <span className={`pl ${e.direction === 'incoming' ? 'sealed' : 'disc'}`} style={{ textTransform: 'uppercase' }}>
                {e.direction === 'incoming' ? '← in' : 'out →'}
              </span>
              <span className="mono" style={{ fontFamily: "'JetBrains Mono',monospace", color: 'var(--accent)', fontSize: 11.5 }}>
                {truncateHash(e.manifest_hash)}
              </span>
            </div>
          ))}
        </div>
      </div>
    </>
  );
}
