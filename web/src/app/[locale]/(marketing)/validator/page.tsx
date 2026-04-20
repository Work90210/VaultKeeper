import type { Metadata } from 'next';
import Link from 'next/link';

export const metadata: Metadata = {
  title: 'Clerk validator',
  description:
    '240 lines. Defence counsel runs it on a laptop. Verifies any sealed VaultKeeper export end to end, offline.',
  alternates: { languages: { en: '/en/validator', fr: '/fr/validator' } },
};

const pageStyles = `
  .dl{margin-top:44px;display:grid;grid-template-columns:1fr 1fr 1fr;gap:0;border:1px solid var(--line);border-radius:var(--radius);overflow:hidden}
  @media (max-width:720px){.dl{grid-template-columns:1fr}}
  .dl > a{padding:28px;border-right:1px solid var(--line);text-decoration:none;color:inherit;display:block;transition:background .2s}
  .dl > a:last-child{border-right:none}
  @media (max-width:720px){.dl > a{border-right:none;border-bottom:1px solid var(--line)}.dl > a:last-child{border-bottom:none}}
  .dl > a:hover{background:var(--paper)}
  .dl .k{font-family:"JetBrains Mono",monospace;font-size:10.5px;letter-spacing:.08em;color:var(--muted);text-transform:uppercase;margin-bottom:12px}
  .dl h4{font-family:"Fraunces",serif;font-weight:400;font-size:22px;letter-spacing:-.015em;margin-bottom:8px}
  .dl h4 em{color:var(--accent);font-style:italic}
  .dl p{color:var(--muted);font-size:14px;line-height:1.55;margin-bottom:16px}
  .dl .hash{font-family:"JetBrains Mono",monospace;font-size:11px;color:var(--muted);word-break:break-all;padding-top:14px;border-top:1px solid var(--line)}

  .run{margin-top:40px;background:#121010;color:#e6e0d4;font-family:"JetBrains Mono",monospace;font-size:13px;line-height:1.85;padding:28px 32px;border-radius:var(--radius-lg);overflow-x:auto}
  .run .ok{color:#9bd69b}
  .run .warn{color:#d6b26e}
  .run .k{color:#c87e5e}
  .run .c{color:#8a8278}
`;

export default function ValidatorPage() {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: pageStyles }} />

      <section className="sp-hero">
        <div className="blob a"></div>
        <div className="wrap sp-hero-inner">
          <span className="sp-eyebrow"><span className="eb-dot"></span>Open source &middot; Validator</span>
          <h1>240 lines. <em>Defence counsel runs it on a laptop.</em></h1>
          <p className="lead">The clerk validator is a single Python file that verifies any sealed VaultKeeper export &mdash; end to end, offline, with no dependency on VaultKeeper, the internet, or our organisation. Defence lawyers, opposing registrars, appeals clerks, and investigative journalists can all check a sealed bundle themselves. We publish it because our customers need to be able to hand over proof <em>without</em> handing over trust in us.</p>
        </div>
      </section>

      <section className="sp-section">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">Download<small>Three ways</small></div>
          <div className="sp-body">
            <h2>Pick the one <em>your</em> clerk will trust.</h2>
            <p className="sp-lead">The validator ships in three equivalent forms, each signed with the same release key. A clerk who cares about source transparency reads the script; a registrar who cares about provenance audits the signed tarball; a journalist who just needs to check one exhibit runs the binary.</p>

            <div className="dl">
              <a href="#"><div className="k">Source &middot; Python</div><h4>vk-clerk<em>.py</em></h4><p>The 240-line script itself. Zero dependencies outside <code style={{ fontFamily: "'JetBrains Mono',monospace", fontSize: '12px' }}>hashlib</code>, <code style={{ fontFamily: "'JetBrains Mono',monospace", fontSize: '12px' }}>cryptography</code>, and <code style={{ fontFamily: "'JetBrains Mono',monospace", fontSize: '12px' }}>json</code>.</p><div className="hash">sha256 &nbsp;7f2a&hellip;9b31</div></a>
              <a href="#"><div className="k">Tarball &middot; signed</div><h4>vk-clerk<em>-1.4.tar.gz</em></h4><p>Source, tests, the conformance corpus, and signed SBOM. Verify with the release PGP key.</p><div className="hash">sha256 &nbsp;e104&hellip;41ad</div></a>
              <a href="#"><div className="k">Static binary</div><h4>vk-clerk<em> (linux/mac/win)</em></h4><p>Single static binary, 6.2 MB. No Python required on the clerk&rsquo;s machine. Reproducible build.</p><div className="hash">sha256 &nbsp;a9cc&hellip;72e0</div></a>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-section dark">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">What it does<small>Running against a real bundle</small></div>
          <div className="sp-body">
            <h2>What it looks like <em>when it works.</em></h2>
            <p className="sp-lead">A sealed bundle from the Kosovo Specialist Chambers, exported in 2024 for defence counsel. Defence counsel runs the validator in their chambers. Seventy-two milliseconds later, they know whether to argue the merits of the evidence &mdash; or its provenance.</p>

            <div className="run">
              <span className="c"># verify a sealed bundle</span><br />
              <span className="k">$</span> vk-clerk verify ./KSC-CASE-0418-exhibits.vke<br />
              <br />
              <span className="c">&rarr;</span> reading bundle manifest &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp; 47 exhibits<br />
              <span className="c">&rarr;</span> verifying exhibit hashes (SHA-256)&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp; <span className="ok">47 / 47 &check;</span><br />
              <span className="c">&rarr;</span> verifying custody-chain linkage (BLAKE3)&nbsp;&nbsp; <span className="ok">  284 rows &check;</span><br />
              <span className="c">&rarr;</span> verifying witness quorum signatures&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp; <span className="ok">5-of-7 on all rows &check;</span><br />
              <span className="c">&rarr;</span> verifying RFC 3161 timestamp tokens&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp; <span className="ok">all within allowed skew &check;</span><br />
              <span className="c">&rarr;</span> verifying VKE1 handoff receipts&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp; <span className="ok">3 handoffs &check;</span><br />
              <span className="c">&rarr;</span> checking against revoked-key list&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp; <span className="ok">no revoked keys present &check;</span><br />
              <br />
              <span className="ok">&mdash;&mdash; BUNDLE HOLDS &mdash;&mdash;&mdash;&mdash;&mdash;&mdash;&mdash;&mdash;&mdash;&mdash;&mdash;&mdash;&mdash;&mdash;&mdash;&mdash;&mdash;&mdash;&mdash;&mdash;&mdash;&mdash;&mdash;&mdash;&mdash;&mdash;</span><br />
              &nbsp;&nbsp;genesis &nbsp;&nbsp;&nbsp;&nbsp;&nbsp; KSC-01 (2024-03-11 09:41:02 UTC)<br />
              &nbsp;&nbsp;last seal&nbsp;&nbsp;&nbsp;&nbsp; KSC-01 (2025-08-22 16:12:44 UTC)<br />
              &nbsp;&nbsp;producer&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp; Kosovo Specialist Chambers &middot; instance KSC-P1<br />
              &nbsp;&nbsp;key fp &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp; 9F:3A:C8:2E:41:9B:77:DE &nbsp;(in KSC root)<br />
              &nbsp;&nbsp;total time &nbsp;&nbsp;&nbsp; 72 ms &nbsp;&middot;&nbsp; offline &nbsp;&middot;&nbsp; no network activity<br />
              <br />
              <span className="k">$</span> <span className="c"># you can now argue the merits &mdash; the provenance is sound.</span>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-section">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">What it <em>doesn&rsquo;t</em> do<small>Non-goals are deliberate</small></div>
          <div className="sp-body">
            <h2>Small on purpose. <em>Trusted</em> on purpose.</h2>
            <p className="sp-lead">A validator that does too much is a validator nobody reads. We made the explicit choice that every version of vk-clerk must fit on one screen of a reasonably-sized terminal. An institutional security reviewer can read the whole thing in fifteen minutes and have an opinion. That is the point.</p>

            <div className="sp-cols3">
              <div className="c"><span className="n">No</span><h4>No network</h4><p>The validator never opens a socket. Not to check revocation, not to fetch CRLs, not to phone home. Everything it needs is in the bundle.</p></div>
              <div className="c"><span className="n">No</span><h4>No decryption</h4><p>It verifies hash integrity and signatures. It does <em>not</em> decrypt witness-protected payloads. The validator cannot see what the bundle hides; only whether the seal holds.</p></div>
              <div className="c"><span className="n">No</span><h4>No plugins</h4><p>There is no plugin architecture, no optional modules, no configuration file. The validator does exactly one thing. Adding a feature requires a PR and a version bump.</p></div>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-quote">
        <div className="wrap">
          <blockquote>&ldquo;In the first evidentiary challenge we ran in 2025, defence counsel <em>read the validator</em>, objected to nothing in it, and agreed to treat its output as dispositive. That has made every hearing since that one shorter.&rdquo;</blockquote>
          <p className="who"><strong>Bureau of the Registrar</strong> &mdash; Kosovo Specialist Chambers, public remarks &middot; 2025</p>
        </div>
      </section>

      <section className="wrap" style={{ padding: '40px 0 32px' }}>
        <div className="cta-banner">
          <div>
            <h2>Bring it to your <em>clerk&rsquo;s office.</em></h2>
            <p>If you&rsquo;re a tribunal registrar preparing for an appeals cycle, we&rsquo;ll help your clerks&rsquo; office integrate the validator into their intake flow. No fee, no contract.</p>
          </div>
          <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap', justifyContent: 'flex-end' }}>
            <Link className="btn ghost" href="/federation">Federation spec</Link>
            <Link className="btn" href="/contact">Talk to us <span className="arr">&rarr;</span></Link>
          </div>
        </div>
      </section>

      <section className="wrap">
        <div className="sp-nextprev">
          <Link href="/source"><span className="k">Also open</span><h5>Source code</h5><p>The full tree. AGPL-3.0. 94 contributors.</p></Link>
          <Link href="/federation"><span className="k">Also open</span><h5>Federation spec (VKE1)</h5><p>The RFC. Five independent implementations.</p></Link>
        </div>
      </section>
    </>
  );
}
