import type { Metadata } from 'next';
import Link from 'next/link';

export const metadata: Metadata = {
  title: 'Federation spec (VKE1)',
  description:
    'VKE1 is the specification for cross-institutional custody. Published under CC-BY-4.0. Implementations are welcome from anyone.',
  alternates: { languages: { en: '/en/federation', fr: '/fr/federation' } },
};

const pageStyles = `
  .rfc-head{border:1px solid var(--line);border-radius:var(--radius);padding:28px;margin-top:40px;background:var(--paper);font-family:"JetBrains Mono",monospace;font-size:13px;line-height:1.8;color:var(--muted)}
  .rfc-head strong{color:var(--ink);font-weight:500}
  .rfc-head em{color:var(--accent);font-style:italic;font-family:"Fraunces",serif;font-size:15px}
  .toc{margin-top:36px;display:grid;grid-template-columns:1fr 1fr;gap:0;border-top:1px solid var(--line)}
  @media (max-width:720px){.toc{grid-template-columns:1fr}}
  .toc a{display:grid;grid-template-columns:40px 1fr;gap:14px;padding:16px 18px 16px 0;border-bottom:1px solid var(--line);text-decoration:none;color:inherit;align-items:baseline}
  .toc a:nth-child(odd){border-right:1px solid var(--line);padding-right:18px}
  @media (max-width:720px){.toc a:nth-child(odd){border-right:none;padding-right:0}}
  .toc a:hover{background:var(--paper)}
  .toc .n{font-family:"JetBrains Mono",monospace;font-size:12px;color:var(--accent)}
  .toc .t{font-family:"Fraunces",serif;font-style:italic;font-size:17px;color:var(--ink);letter-spacing:-.01em}
  .toc small{display:block;color:var(--muted);font-size:13px;margin-top:2px;font-family:"Inter",sans-serif;font-style:normal;line-height:1.45}
`;

export default function FederationPage() {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: pageStyles }} />

      <section className="sp-hero">
        <div className="blob a"></div>
        <div className="wrap sp-hero-inner">
          <span className="sp-eyebrow"><span className="eb-dot"></span>Open source &middot; VKE1</span>
          <h1>An RFC for <em>evidence moving between</em> institutions.</h1>
          <p className="lead">VKE1 &mdash; VaultKeeper Evidence Exchange, version 1 &mdash; is the specification for cross-institutional custody. It is not a file format. It is the protocol by which an NGO in Gaziantep can hand a sealed sub-chain to the ICC in The Hague, have the ICC verify and extend it, and return a sealed handoff receipt that is itself admissible. Published under CC-BY-4.0. Implementations are welcome from anyone.</p>
          <div className="rfc-head">
            <strong>Doc ID:</strong> <em>vke1-2025-02</em> &nbsp;&middot;&nbsp; <strong>Status:</strong> Ratified &nbsp;&middot;&nbsp; <strong>Version:</strong> 1.2<br />
            <strong>Authors:</strong> J. Arvidsson (VaultKeeper) &middot; I. Velasco (VaultKeeper) &middot; M. Hasanovi&#263; (KSC) &middot; L. Okonkwo (ICC, observer)<br />
            <strong>Supersedes:</strong> vke0-2024-01 &nbsp;&middot;&nbsp; <strong>License:</strong> CC-BY-4.0
          </div>
        </div>
      </section>

      <section className="sp-section">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">Table of contents<small>34 pages, 9 sections</small></div>
          <div className="sp-body">
            <h2>The full specification, <em>section by section.</em></h2>
            <p className="sp-lead">Each section is independently readable. Most implementers need sections 2, 3, and 6. Registrars care most about sections 5 and 8. Security engineers should read sections 4 and 7 end-to-end.</p>

            <div className="toc">
              <a href="#"><span className="n">&sect; 1</span><span><span className="t">Terminology</span><small>Exhibit, sub-chain, peer, quorum, handoff receipt, sovereignty scope.</small></span></a>
              <a href="#"><span className="n">&sect; 2</span><span><span className="t">The sub-chain data model</span><small>Merkle-linked custody rows as an exportable, re-importable sub-chain.</small></span></a>
              <a href="#"><span className="n">&sect; 3</span><span><span className="t">Peer discovery &amp; pairing</span><small>Out-of-band pairing via short codes; no central registry required.</small></span></a>
              <a href="#"><span className="n">&sect; 4</span><span><span className="t">Cryptographic envelope</span><small>Ed25519 + X25519 hybrid; Kyber-768 draft annex for PQ readiness.</small></span></a>
              <a href="#"><span className="n">&sect; 5</span><span><span className="t">Authority delegation</span><small>How one institution authorises another to extend its chain &mdash; and revoke.</small></span></a>
              <a href="#"><span className="n">&sect; 6</span><span><span className="t">Transport bindings</span><small>HTTPS (primary) &middot; Signal (field) &middot; sealed USB (air-gap). All three normative.</small></span></a>
              <a href="#"><span className="n">&sect; 7</span><span><span className="t">Disagreement resolution</span><small>When two peers hold divergent histories, which wins and why.</small></span></a>
              <a href="#"><span className="n">&sect; 8</span><span><span className="t">Handoff receipts</span><small>The sealed acknowledgement a receiving institution signs back.</small></span></a>
              <a href="#"><span className="n">&sect; 9</span><span><span className="t">Security considerations</span><small>Threat model &middot; residual risks &middot; non-goals.</small></span></a>
              <a href="#"><span className="n">A</span><span><span className="t">Annex &mdash; PQ migration (draft)</span><small>Forward-compatible bindings for post-quantum signatures and KEMs.</small></span></a>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-section" style={{ background: 'var(--paper)' }}>
        <div className="wrap sp-grid-12">
          <div className="sp-rail">Who&rsquo;s implemented<small>Known conforming peers</small></div>
          <div className="sp-body">
            <h2>Five independent implementations. <em>Including</em> one that isn&rsquo;t ours.</h2>
            <p className="sp-lead">A specification is only meaningful when something other than its author implements it. VKE1 has now shipped in five production systems, one of which &mdash; the OPCW&rsquo;s internal tooling &mdash; is a from-scratch Go implementation that passed our conformance suite in November 2025.</p>

            <div className="sp-rows">
              <div className="sp-row"><span className="idx">VK / core</span><h4>vaultkeeper/core &middot; Rust</h4><p><strong>Reference implementation.</strong> AGPL-3.0. Full VKE1 v1.2 coverage. Runs at every VaultKeeper customer.</p></div>
              <div className="sp-row"><span className="idx">OPCW</span><h4>opcw-custody &middot; Go</h4><p><strong>From-scratch, independent.</strong> OPCW&rsquo;s in-house custody system. Passed conformance in Nov 2025. Validates VKE1 is implementable without our code.</p></div>
              <div className="sp-row"><span className="idx">KSC</span><h4>ksc-bridge &middot; Java</h4><p><strong>Bridge layer.</strong> Kosovo Specialist Chambers&rsquo; bridge between legacy Relativity archives and VKE1 peers. Conforms with waivers &sect;6.2 (no Signal binding).</p></div>
              <div className="sp-row"><span className="idx">community</span><h4>libvke1 &middot; C</h4><p><strong>Community port.</strong> Small-footprint C library for embedded devices. Field recorder tablets, seized-device imagers. LGPL-2.1.</p></div>
              <div className="sp-row"><span className="idx">VK / clerk</span><h4>vk-clerk &middot; Python</h4><p><strong>Validator-only.</strong> Verifies incoming VKE1 bundles without implementing the producer side. 240 lines. Used by tribunal clerks.</p></div>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-section dark">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">Governance<small>How VKE changes</small></div>
          <div className="sp-body">
            <h2>VaultKeeper does <em>not</em> own VKE.</h2>
            <p className="sp-lead">VKE is stewarded by a neutral working group: two seats from VaultKeeper, two from independent implementers, two from institutional users. Changes require five of six. We deliberately cannot unilaterally alter the protocol our customers depend on.</p>

            <div className="sp-cols3">
              <div className="c"><span className="n">WG</span><h4>Six-seat working group</h4><p>Two VaultKeeper, two external implementers (currently OPCW + community libvke1), two institutional users (currently KSC + Bellingcat).</p></div>
              <div className="c"><span className="n">RFC</span><h4>Public RFC process</h4><p>Proposed changes post as draft on the mailing list. 45-day public comment. Then working-group vote. All drafts and votes are archived publicly.</p></div>
              <div className="c"><span className="n">EOL</span><h4>Back-compat guarantee</h4><p>Any VKE1 producer is guaranteed to be readable by any VKE1 consumer for a minimum of 10 years after the receiving version is released.</p></div>
            </div>
          </div>
        </div>
      </section>

      <section className="wrap" style={{ padding: '40px 0 32px' }}>
        <div className="cta-banner">
          <div>
            <h2>Read the <em>PDF.</em> Write an implementation.</h2>
            <p>If you&rsquo;re building evidence tooling and want it to interop with ours, the spec is 34 pages and two afternoons&rsquo; work. We host a conformance suite that runs in your CI.</p>
          </div>
          <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap', justifyContent: 'flex-end' }}>
            <Link className="btn ghost" href="/source">Source code</Link>
            <Link className="btn" href="/validator">Clerk validator <span className="arr">&rarr;</span></Link>
          </div>
        </div>
      </section>

      <section className="wrap">
        <div className="sp-nextprev">
          <Link href="/source"><span className="k">Also open</span><h5>Source code</h5><p>Rust + TS, AGPL, 94 contributors.</p></Link>
          <Link href="/validator"><span className="k">Also open</span><h5>Clerk validator</h5><p>240 lines of Python. Offline verifier.</p></Link>
        </div>
      </section>
    </>
  );
}
