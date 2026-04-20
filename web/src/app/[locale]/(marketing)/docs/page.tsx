import type { Metadata } from 'next';
import Link from 'next/link';

export const metadata: Metadata = {
  title: 'Open source & docs',
  description:
    'No dark code. Read the custody engine, audit the crypto, diff the release tags. Everything on this page is also in the repository.',
  alternates: { languages: { en: '/en/docs', fr: '/fr/docs' } },
};

const pageStyles = `
  .d-hero{padding:48px 0 24px}
  .d-hero h1{font-size:clamp(44px,5.4vw,84px)}
  .d-hero h1 em{color:var(--accent);font-style:italic}

  .repo-card{background:var(--ink);color:var(--bg);border-radius:var(--radius-lg);padding:40px;display:grid;grid-template-columns:1.2fr .8fr;gap:40px;align-items:center;margin-top:40px;position:relative;overflow:hidden}
  @media (max-width:820px){.repo-card{grid-template-columns:1fr;padding:28px}}
  .repo-card h3{font-family:"Fraunces",serif;font-size:32px;color:var(--bg);font-weight:400}
  .repo-card h3 em{color:var(--accent-soft);font-style:italic}
  .repo-card p{color:var(--muted-2);margin-top:10px;max-width:52ch;font-size:15px;line-height:1.55}
  .repo-stats{display:grid;grid-template-columns:1fr 1fr;gap:16px;margin-top:24px}
  .repo-stats div{font-family:"Fraunces",serif;font-size:28px;letter-spacing:-.02em;color:var(--bg)}
  .repo-stats div small{display:block;font-size:12px;font-family:"Inter",sans-serif;color:var(--muted-2);margin-top:3px}
  .repo-term{background:#0a0907;border:1px solid rgba(255,255,255,.1);border-radius:14px;padding:22px;font-family:"JetBrains Mono",monospace;font-size:12.5px;color:var(--muted-2);line-height:1.7}
  .repo-term .p{color:var(--accent-soft)}
  .repo-term .k{color:var(--bg)}

  .doc-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:20px;margin-top:32px}
  @media (max-width:820px){.doc-grid{grid-template-columns:1fr}}
  .doc{padding:28px;border-radius:var(--radius);background:var(--paper);border:1px solid var(--line);display:flex;flex-direction:column;gap:12px;transition:all .25s}
  .doc:hover{transform:translateY(-3px);box-shadow:var(--shadow);border-color:var(--accent-soft)}
  .doc .k{font-family:"JetBrains Mono",monospace;font-size:11px;color:var(--muted);letter-spacing:.04em}
  .doc h3{font-family:"Fraunces",serif;font-size:22px;letter-spacing:-.015em}
  .doc p{color:var(--muted);font-size:14px;line-height:1.55;flex:1}
  .doc .meta{font-size:12px;color:var(--muted);display:flex;justify-content:space-between;padding-top:12px;border-top:1px solid var(--line)}

  .spec-block{padding:56px 0}
  .spec-block code{font-family:"JetBrains Mono",monospace;font-size:12.5px;background:var(--bg-2);padding:1px 6px;border-radius:4px}

  .validator{padding:56px 0}
  .val-card{padding:40px;border-radius:var(--radius-lg);background:var(--paper);border:1px solid var(--line);display:grid;grid-template-columns:1fr 1fr;gap:48px;align-items:center}
  @media (max-width:820px){.val-card{grid-template-columns:1fr;padding:28px}}
  .val-card h3{font-size:30px;margin-bottom:14px}
  .val-card h3 em{color:var(--accent);font-style:italic}
  .val-card p{color:var(--muted);font-size:15px;line-height:1.6}
  .val-result{padding:24px;background:var(--ink);color:var(--bg);border-radius:14px;font-family:"JetBrains Mono",monospace;font-size:12px;line-height:1.75}
  .val-result .ok{color:#9bc18c}
  .val-result .dim{color:var(--muted-2)}

  .changelog{border-radius:var(--radius);border:1px solid var(--line);background:var(--paper)}
  .cl-row{display:grid;grid-template-columns:160px 1fr;gap:32px;padding:24px 28px;border-top:1px solid var(--line)}
  .cl-row:first-child{border-top:none}
  @media (max-width:620px){.cl-row{grid-template-columns:1fr;gap:8px}}
  .cl-row .v{font-family:"Fraunces",serif;font-style:italic;font-size:22px;color:var(--accent)}
  .cl-row .v small{display:block;font-family:"Inter",sans-serif;font-style:normal;font-size:12px;color:var(--muted);margin-top:3px}
  .cl-row h4{font-family:"Fraunces",serif;font-size:20px;margin-bottom:8px;font-weight:400}
  .cl-row ul{margin:6px 0 0;padding:0;list-style:none;display:flex;flex-direction:column;gap:6px;color:var(--muted);font-size:14px}
  .cl-row li::before{content:"\\2014 ";color:var(--accent)}
`;

export default function DocsPage() {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: pageStyles }} />

      <section className="wrap d-hero">
        <span className="eyebrow"><span className="eb-dot"></span>AGPL-3.0 &middot; read it, fork it, run it</span>
        <h1 style={{ marginTop: '20px' }}>The whole thing <em>is the spec.</em></h1>
        <p className="lead" style={{ marginTop: '24px' }}>No dark code. Read the custody engine, audit the crypto, diff the release tags. Everything on this page is also in the repository; this page just orders it.</p>

        <div className="repo-card">
          <div>
            <span className="mono" style={{ color: 'var(--muted-2)', fontSize: '12px' }}>github.com / vaultkeeper / vaultkeeper</span>
            <h3 style={{ marginTop: '12px' }}>The entire platform, in <em>one AGPL repo.</em></h3>
            <p>Backend (Go, 29 packages), frontend (Next.js + TypeScript), custody engine, validator, federation protocol, Ansible + Terraform, and all audit artefacts. Released tags are reproducible from source.</p>
            <div className="repo-stats">
              <div>4.2k<small>stars</small></div>
              <div>318<small>forks</small></div>
              <div>42<small>contributors</small></div>
              <div>v1.2.0<small>latest &middot; Apr 7 2026</small></div>
            </div>
          </div>
          <div className="repo-term">
            <span className="p">$</span> <span className="k">git clone https://github.com/vaultkeeper/vaultkeeper</span><br />
            Cloning into &rsquo;vaultkeeper&rsquo;...<br />
            <span className="dim">done. 29 Go packages, 70 React components.</span><br /><br />
            <span className="p">$</span> <span className="k">cd vaultkeeper &amp;&amp; make up</span><br />
            <span className="dim">docker compose up --build -d</span><br />
            &check; postgres   &middot; healthy<br />
            &check; minio      &middot; healthy<br />
            &check; keycloak   &middot; healthy<br />
            &check; meili      &middot; healthy<br />
            &check; vaultkeep  &middot; listening on :8443<br /><br />
            <span className="p">$</span> <span className="k">open https://localhost:8443</span>
          </div>
        </div>
      </section>

      <section className="section wrap" id="self">
        <span className="eyebrow">Documentation</span>
        <h2 style={{ marginTop: '14px' }}>Everything you need to <em className="a">run your own.</em></h2>
        <div className="doc-grid">
          <a href="#" className="doc">
            <div className="k">GUIDE &middot; 18 min</div>
            <h3>Self-hosting on Hetzner</h3>
            <p>From a blank CX22 to a production VaultKeeper instance with TLS, SSO, and nightly backups. Walk-through with Terraform.</p>
            <div className="meta"><span>Updated Apr 2026</span><span>&rarr;</span></div>
          </a>
          <a href="#" className="doc">
            <div className="k">GUIDE &middot; 32 min</div>
            <h3>Air-gapped deployment</h3>
            <p>The sealed-appliance workflow: key ceremony, offline update packages, chain verification without network.</p>
            <div className="meta"><span>Updated Mar 2026</span><span>&rarr;</span></div>
          </a>
          <a href="#" className="doc">
            <div className="k">REFERENCE</div>
            <h3>Custody engine internals</h3>
            <p>How the append-only log works. Row layout, RLS policies, hash chaining, advisory locks, failure modes.</p>
            <div className="meta"><span>v1.2.0</span><span>&rarr;</span></div>
          </a>
          <a href="#" className="doc" id="validator">
            <div className="k">TOOL</div>
            <h3>Clerk validator &middot; vk-verify</h3>
            <p>MIT-licensed CLI that verifies a VaultKeeper export without any network access. Single static binary for Linux / macOS / Windows.</p>
            <div className="meta"><span>v0.9.4</span><span>&rarr;</span></div>
          </a>
          <a href="#" className="doc" id="spec">
            <div className="k">SPEC &middot; RFC-STYLE</div>
            <h3>VKE1 federation protocol</h3>
            <p>Merkle-proof selective disclosure for cross-instance evidence exchange. Wire format, key rotation, revocation.</p>
            <div className="meta"><span>Draft-03</span><span>&rarr;</span></div>
          </a>
          <a href="#" className="doc" id="audit">
            <div className="k">AUDITS</div>
            <h3>Security reports &amp; threat model</h3>
            <p>NCC Group 2025 review, Cure53 penetration test, and the 38-page VaultKeeper threat model. NDA on request.</p>
            <div className="meta"><span>2 reports</span><span>&rarr;</span></div>
          </a>
          <a href="#" className="doc">
            <div className="k">API &middot; OPENAPI 3.1</div>
            <h3>REST API reference</h3>
            <p>Every endpoint, with case-scoped rate limits and custody-log side effects annotated.</p>
            <div className="meta"><span>v1.2.0</span><span>&rarr;</span></div>
          </a>
          <a href="#" className="doc">
            <div className="k">SDK</div>
            <h3>TypeScript &amp; Go SDKs</h3>
            <p>First-party clients with automatic retry, hash verification, and session refresh built in.</p>
            <div className="meta"><span>MIT</span><span>&rarr;</span></div>
          </a>
          <a href="#" className="doc">
            <div className="k">COOKBOOK</div>
            <h3>Connector SDK</h3>
            <p>Build your own signed ingest agent for a forensic tool we don&rsquo;t cover yet. Worked example with Autopsy.</p>
            <div className="meta"><span>10 min</span><span>&rarr;</span></div>
          </a>
        </div>
      </section>

      <section className="section wrap validator">
        <div className="val-card">
          <div>
            <span className="eyebrow">vk-verify</span>
            <h3 style={{ marginTop: '14px' }}>A court clerk verifies an export in <em>one command.</em></h3>
            <p>No VaultKeeper login required, no internet required. The clerk drops the exported bundle into <code style={{ fontFamily: "'JetBrains Mono',monospace", fontSize: '12px' }}>vk-verify</code>, and the tool walks the hash chain, checks every RFC 3161 timestamp, verifies the optional public-chain anchor, and prints either a green pass or a red failure with the offending row.</p>
            <p style={{ marginTop: '12px' }}>MIT licensed. Single static binary. Reproducibly built.</p>
            <div style={{ display: 'flex', gap: '12px', marginTop: '22px', flexWrap: 'wrap' }}>
              <a className="btn" href="#">Download v0.9.4 <span className="arr">&rarr;</span></a>
              <a className="btn ghost" href="#">Build from source</a>
            </div>
          </div>
          <div className="val-result">
            <span className="dim">$</span> vk-verify ICC-UKR-2024.vk.zip<br /><br />
            &rarr; loading bundle (218 MB, 1284 exhibits)<br />
            &rarr; reconstructing custody chain<br />
            <span className="ok">&check;</span> 14,218 events &middot; hash chain intact<br />
            <span className="ok">&check;</span> 1,284 exhibits &middot; leaf hashes match<br />
            <span className="ok">&check;</span> Merkle root matches signed manifest<br />
            <span className="ok">&check;</span> RFC 3161 token verified (GlobalSign EU)<br />
            <span className="ok">&check;</span> OpenTimestamps anchor: BTC 899,412 (+8)<br />
            <span className="ok">&check;</span> Witness-redaction ledger consistent<br /><br />
            <span className="ok">PASS.</span> Bundle is verifiable.<br />
            <span className="dim">Total: 4.1s &middot; offline</span>
          </div>
        </div>
      </section>

      <section className="section wrap">
        <span className="eyebrow">Changelog</span>
        <h2 style={{ marginTop: '14px' }}>Recent <em className="a">releases.</em></h2>

        <div className="changelog" style={{ marginTop: '28px' }}>
          <div className="cl-row">
            <div className="v">v1.2.0<small>Apr 7, 2026</small></div>
            <div>
              <h4>Sealed evidence escrow (beta)</h4>
              <ul>
                <li>Time-locked exhibit release with N-of-M threshold unlock</li>
                <li>Dead-man&rsquo;s-switch recovery path for whistleblower workflows</li>
                <li>Federation protocol (VKE1): witness-pseudonym preservation across hops</li>
                <li>Performance: search p95 down 38% on 10M+ exhibit corpora</li>
              </ul>
            </div>
          </div>
          <div className="cl-row">
            <div className="v">v1.1.0<small>Feb 12, 2026</small></div>
            <div>
              <h4>On-prem AI appliance support</h4>
              <ul>
                <li>First-class integration with local Whisper, OCR, entity extraction</li>
                <li>Semantic search over local embeddings; nothing leaves the box</li>
                <li>Cellebrite UFED 2026.1 connector</li>
              </ul>
            </div>
          </div>
          <div className="cl-row">
            <div className="v">v1.0.0<small>Nov 30, 2025</small></div>
            <div>
              <h4>General availability</h4>
              <ul>
                <li>Post-audit (NCC Group, Cure53) release</li>
                <li>Cryptographic federation protocol VKE1 shipped</li>
                <li>Formal-verification work on custody engine begins (TLA+)</li>
              </ul>
            </div>
          </div>
          <div className="cl-row">
            <div className="v">v0.9<small>Jul 1, 2025</small></div>
            <div>
              <h4>Public beta</h4>
              <ul>
                <li>First deployments outside the founding NGO cohort</li>
                <li>vk-verify validator tool MIT-released</li>
              </ul>
            </div>
          </div>
        </div>
      </section>

      <section className="wrap" style={{ padding: '24px 32px 64px' }}>
        <div className="cta-banner">
          <div>
            <h2>Contribute? Audit? <em>Fork?</em></h2>
            <p>Pull requests welcome from anyone. A few specific areas &mdash; federation, on-prem AI, formal verification &mdash; carry a contributor grant. Email the team.</p>
          </div>
          <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap', justifyContent: 'flex-end' }}>
            <a className="btn ghost" href="https://github.com" target="_blank" rel="noopener">View on GitHub</a>
            <Link className="btn" href="/contact">Reach the team <span className="arr">&rarr;</span></Link>
          </div>
        </div>
      </section>
    </>
  );
}
