import type { Metadata } from 'next';
import Link from 'next/link';

export const metadata: Metadata = {
  title: 'Source code',
  description:
    'VaultKeeper\'s full source tree is AGPL-3.0. No binary blobs, no enterprise fork, no proprietary kernel.',
  alternates: { languages: { en: '/en/source', fr: '/fr/source' } },
};

const pageStyles = `
  .repo-meta{display:grid;grid-template-columns:repeat(4,1fr);gap:0;margin-top:48px;border:1px solid var(--line);border-radius:var(--radius);overflow:hidden}
  @media (max-width:760px){.repo-meta{grid-template-columns:1fr 1fr}}
  .repo-meta > div{padding:24px;border-right:1px solid var(--line)}
  .repo-meta > div:last-child{border-right:none}
  @media (max-width:760px){.repo-meta > div:nth-child(2n){border-right:none}.repo-meta > div{border-bottom:1px solid var(--line)}}
  .repo-meta .k{font-family:"JetBrains Mono",monospace;font-size:10.5px;letter-spacing:.06em;color:var(--muted);text-transform:uppercase;margin-bottom:8px}
  .repo-meta strong{font-family:"Fraunces",serif;font-weight:400;font-size:24px;letter-spacing:-.01em;display:block;line-height:1.1}
  .repo-meta strong em{color:var(--accent);font-style:italic}
  .repo-meta small{display:block;font-size:12px;color:var(--muted);margin-top:6px}

  .mod-grid{display:grid;grid-template-columns:1fr 1fr;gap:0;margin-top:40px;border:1px solid var(--line);border-radius:var(--radius);overflow:hidden}
  @media (max-width:760px){.mod-grid{grid-template-columns:1fr}}
  .mod{padding:28px;border-right:1px solid var(--line);border-bottom:1px solid var(--line)}
  .mod:nth-child(2n){border-right:none}
  .mod:nth-last-child(-n+2){border-bottom:none}
  @media (max-width:760px){.mod{border-right:none;border-bottom:1px solid var(--line)!important}.mod:last-child{border-bottom:none!important}}
  .mod .path{font-family:"JetBrains Mono",monospace;font-size:12px;color:var(--accent);margin-bottom:10px}
  .mod h4{font-family:"Fraunces",serif;font-weight:400;font-size:22px;letter-spacing:-.015em;margin-bottom:8px}
  .mod p{color:var(--muted);font-size:14px;line-height:1.55}
  .mod .lang{margin-top:14px;display:flex;gap:8px;flex-wrap:wrap}
  .mod .lang span{font-family:"JetBrains Mono",monospace;font-size:10.5px;padding:3px 9px;border-radius:999px;background:var(--bg-2);color:var(--muted);letter-spacing:.04em}

  .recent{margin-top:48px}
  .commit{display:grid;grid-template-columns:100px 1fr auto;gap:20px;padding:16px 0;border-bottom:1px solid var(--line);align-items:baseline;font-size:14px}
  @media (max-width:640px){.commit{grid-template-columns:80px 1fr;gap:10px}.commit .a{grid-column:2;justify-self:start;margin-top:4px}}
  .commit .h{font-family:"JetBrains Mono",monospace;font-size:12px;color:var(--accent)}
  .commit .m{color:var(--ink)}
  .commit .a{font-size:12.5px;color:var(--muted);font-family:"Fraunces",serif;font-style:italic}
`;

export default function SourcePage() {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: pageStyles }} />

      <section className="sp-hero">
        <div className="blob a"></div>
        <div className="wrap sp-hero-inner">
          <span className="sp-eyebrow"><span className="eb-dot"></span>Open source &middot; Source code</span>
          <h1>Fork it. Run it. <em>Verify</em> it. It is yours.</h1>
          <p className="lead">VaultKeeper&rsquo;s full source tree &mdash; including the custody engine, federation daemon, witness-node quorum logic, and the on-prem AI pipeline &mdash; is AGPL-3.0. No binary blobs, no &ldquo;enterprise&rdquo; fork, no proprietary kernel. If the Dutch co&ouml;p ceases to exist tomorrow, your registrar still has the entire system.</p>
          <div className="repo-meta">
            <div><div className="k">Licence</div><strong><em>AGPL-3.0</em></strong><small>with commercial exception for signed customers</small></div>
            <div><div className="k">Repo</div><strong>vaultkeeper/core</strong><small>sr.ht &middot; self-hosted mirror</small></div>
            <div><div className="k">Languages</div><strong>Rust &middot; TS</strong><small>74% &middot; 18% &middot; F* 6% &middot; others 2%</small></div>
            <div><div className="k">Contributors</div><strong>94 people</strong><small>across 22 institutions</small></div>
          </div>
        </div>
      </section>

      <section className="sp-section">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">Layout<small>The 11 modules</small></div>
          <div className="sp-body">
            <h2>Read the pieces you care about <em>first.</em></h2>
            <p className="sp-lead">The critical parts &mdash; the custody engine and the federation daemon &mdash; are small enough for one engineer to hold in their head in a week. Start there. Everything else is composition.</p>

            <div className="mod-grid">
              <div className="mod"><div className="path">vk-custody &middot; Rust &middot; 4,200 LoC</div><h4>Custody engine</h4><p>The append-only ledger. Hash-chaining, witness quorum, Merkle snapshots, F*-verified invariants. Read this first.</p><div className="lang"><span>Rust</span><span>F* proofs</span><span>PostgreSQL RLS</span></div></div>
              <div className="mod"><div className="path">vk-ingest &middot; Rust &middot; 2,100 LoC</div><h4>Ingest gateway</h4><p>Chunked resumable uploads, RFC 3161 TSA integration, client-side hash reconciliation.</p><div className="lang"><span>Rust</span><span>tus protocol</span></div></div>
              <div className="mod"><div className="path">vk-witness &middot; Rust &middot; 1,600 LoC</div><h4>Witness node</h4><p>Quorum signer. Ed25519/P-256 HSM integration (PKCS#11). Standalone binary &mdash; can run independently from core.</p><div className="lang"><span>Rust</span><span>PKCS#11</span></div></div>
              <div className="mod"><div className="path">vk-federate &middot; Rust &middot; 3,000 LoC</div><h4>Federation daemon</h4><p>VKE1 protocol implementation. Sub-chain federation, cross-institutional provenance.</p><div className="lang"><span>Rust</span><span>VKE1 spec</span></div></div>
              <div className="mod"><div className="path">vk-app &middot; TS &middot; 18,400 LoC</div><h4>Web application</h4><p>Investigator workspace. CRDT editing (Yjs), redaction editor, search UI, exhibit pages.</p><div className="lang"><span>TypeScript</span><span>Yjs CRDT</span><span>SvelteKit</span></div></div>
              <div className="mod"><div className="path">vk-ai &middot; Python &middot; 6,800 LoC</div><h4>On-prem AI pipeline</h4><p>Whisper, docTR, BGE-M3, GLiNER, NLLB. Dockerised. CPU or CUDA. Nothing phones home.</p><div className="lang"><span>Python</span><span>Docker</span></div></div>
              <div className="mod"><div className="path">vk-clerk &middot; Python &middot; 240 LoC</div><h4>Clerk validator</h4><p>Standalone validator &mdash; verifies a sealed export end-to-end, offline. 240 lines, no dependencies except hashlib and cryptography.</p><div className="lang"><span>Python</span><span>zero-dep</span></div></div>
              <div className="mod"><div className="path">vk-ops &middot; Ansible &middot; 1,400 LoC</div><h4>Deployment</h4><p>Ansible roles for Hetzner, Scaleway, OVH, AWS, on-prem Proxmox. 15-minute single-node; 90-minute multi-site.</p><div className="lang"><span>Ansible</span><span>Terraform</span></div></div>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-section" style={{ background: 'var(--paper)' }}>
        <div className="wrap sp-grid-12">
          <div className="sp-rail">Activity<small>Last 30 days</small></div>
          <div className="sp-body">
            <h2>Read the <em>commits.</em> They are small.</h2>
            <p className="sp-lead">VaultKeeper does not ship commits like <code style={{ fontFamily: "'JetBrains Mono',monospace", fontSize: '13px', background: 'var(--bg-2)', padding: '2px 6px', borderRadius: '4px' }}>&ldquo;misc updates&rdquo;</code>. Every commit is a single change, a single rationale, a single signer. Here is what has landed recently.</p>

            <div className="recent">
              <div className="commit"><span className="h">a7f4d029</span><span className="m">vk-custody: tighten F* proof for merge associativity under concurrent export</span><span className="a">J. Arvidsson</span></div>
              <div className="commit"><span className="h">3e110055</span><span className="m">vk-federate: VKE1 v1.2 &mdash; scoped sub-chain authority delegation</span><span className="a">I. Velasco</span></div>
              <div className="commit"><span className="h">91abccf2</span><span className="m">vk-app: survivor-first intake flow &mdash; low-bandwidth voice-first mode</span><span className="a">N. Bagrationi + UTI team</span></div>
              <div className="commit"><span className="h">b80276aa</span><span className="m">vk-ai: swap nllb-200 &rarr; nllb-3.3B for uk/ru/en quality on field hardware</span><span className="a">A. Hassan</span></div>
              <div className="commit"><span className="h">c44d9912</span><span className="m">vk-clerk: add pqc-aware validation (draft &mdash; flag-gated)</span><span className="a">J. Arvidsson</span></div>
              <div className="commit"><span className="h">e1c04812</span><span className="m">vk-witness: PKCS#11 support for SmartCard-HSM 4.0</span><span className="a">community &middot; @tuppi</span></div>
              <div className="commit"><span className="h">d947b311</span><span className="m">vk-ops: Ansible role for air-gapped delta import via sealed USB</span><span className="a">IRMCT team</span></div>
              <div className="commit"><span className="h">008fa102</span><span className="m">docs: federation spec &sect;4 &mdash; disagreement resolution between peers</span><span className="a">I. Velasco</span></div>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-section dark">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">Clone<small>How to get running</small></div>
          <div className="sp-body">
            <h2>One command. <em>Fifteen</em> minutes.</h2>
            <p className="sp-lead">Installs on Debian/Ubuntu LTS, RHEL 9, and OpenSUSE Leap. Requires 4 vCPU, 16&nbsp;GB RAM, 200&nbsp;GB storage as an absolute minimum &mdash; though most institutions run far more. Runs offline from commit 1.</p>

            <div className="code-card" style={{ marginTop: '36px' }}>
              <span className="c"># Clone and run &mdash; single-node, self-contained</span><br />
              <span className="k">$</span> <span className="p">git</span> clone https://git.vaultkeeper.coop/vaultkeeper/core<br />
              <span className="k">$</span> <span className="p">cd</span> core &amp;&amp; ./vk up<br />
              <br />
              <span className="c">&rarr; generating witness keypairs (Ed25519)              &check;</span><br />
              <span className="c">&rarr; bootstrapping custody ledger (PostgreSQL 16)       &check;</span><br />
              <span className="c">&rarr; provisioning TSA connections (tsa.dfn.de default)  &check;</span><br />
              <span className="c">&rarr; loading on-prem AI models (~4.2 GB, one-time)      &check;</span><br />
              <span className="c">&rarr; sealing genesis block                              &check;</span><br />
              <br />
              <span className="s">VaultKeeper running at https://localhost:8443</span><br />
              <span className="s">Admin token written to ./vk-admin.token</span><br />
            </div>
          </div>
        </div>
      </section>

      <section className="wrap" style={{ padding: '40px 0 32px' }}>
        <div className="cta-banner">
          <div>
            <h2>Our GPG key, our social media, our <em>office.</em></h2>
            <p>Prinsengracht 54, 1015 DV Amsterdam. Walk in Tuesdays; we&rsquo;ll make coffee. Or contribute remotely &mdash; there is no corporate contributor agreement to sign.</p>
          </div>
          <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap', justifyContent: 'flex-end' }}>
            <Link className="btn ghost" href="/federation">Federation spec</Link>
            <Link className="btn" href="/docs">Self-host guide <span className="arr">&rarr;</span></Link>
          </div>
        </div>
      </section>

      <section className="wrap">
        <div className="sp-nextprev">
          <Link href="/federation"><span className="k">Also open</span><h5>Federation spec (VKE1)</h5><p>The 34-page RFC for cross-institutional evidence exchange.</p></Link>
          <Link href="/validator"><span className="k">Also open</span><h5>Clerk validator</h5><p>240 lines of Python. Downloadable. Verifies any sealed export.</p></Link>
        </div>
      </section>
    </>
  );
}
