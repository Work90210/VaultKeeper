import type { Metadata } from 'next';
import Link from 'next/link';

export const metadata: Metadata = {
  title: 'Security',
  description:
    'Every claim VaultKeeper makes can be checked independently. The database enforces append-only at the engine level. The crypto is open. The threat model is written down.',
  alternates: { languages: { en: '/en/security', fr: '/fr/security' } },
};

const pageStyles = `
  .sec-hero{padding:48px 0 24px}
  .sec-hero h1{font-size:clamp(44px,5.4vw,84px)}
  .sec-hero h1 em{color:var(--accent);font-style:italic}

  .layers{display:grid;grid-template-columns:repeat(4,1fr);gap:16px;margin-top:56px}
  @media (max-width:980px){.layers{grid-template-columns:1fr 1fr}}
  @media (max-width:560px){.layers{grid-template-columns:1fr}}
  .layer{padding:24px;border-radius:var(--radius);background:var(--paper);border:1px solid var(--line);position:relative;overflow:hidden}
  .layer .num{font-family:"Fraunces",serif;font-style:italic;color:var(--accent);font-size:14px;margin-bottom:14px}
  .layer h3{font-size:20px;margin-bottom:8px}
  .layer p{color:var(--muted);font-size:13.5px;line-height:1.5}

  .threat{padding:64px;border-radius:var(--radius-lg);background:var(--ink);color:var(--bg);position:relative;overflow:hidden;display:grid;grid-template-columns:1fr 1fr;gap:56px}
  @media (max-width:920px){.threat{grid-template-columns:1fr;padding:40px;border-radius:var(--radius)}}
  .threat h2{color:var(--bg);margin-bottom:18px}
  .threat h2 em{color:var(--accent-soft);font-style:italic}
  .threat p{color:var(--muted-2);font-size:16px;line-height:1.6;max-width:52ch}
  .threat .scenarios{display:flex;flex-direction:column;gap:14px}
  .scen{padding:18px 22px;border:1px solid rgba(255,255,255,.1);border-radius:14px;background:rgba(255,255,255,.02);display:grid;grid-template-columns:28px 1fr;gap:16px}
  .scen .num{font-family:"Fraunces",serif;font-style:italic;color:var(--accent-soft);font-size:16px}
  .scen h4{color:var(--bg);font-family:"Fraunces",serif;font-size:20px;font-weight:400;margin-bottom:5px}
  .scen p{color:var(--muted-2);font-size:13.5px;line-height:1.5;margin:0}

  .architecture{padding:56px 0}
  .arch-diagram{border-radius:var(--radius-lg);border:1px solid var(--line);background:var(--paper);padding:56px;overflow:hidden;position:relative;min-height:540px}
  @media (max-width:820px){.arch-diagram{padding:28px}}
  .arch-title{text-align:center;font-size:12px;color:var(--muted);margin-bottom:32px;letter-spacing:.04em;text-transform:uppercase}

  .arch-grid{display:grid;grid-template-columns:1fr 1fr 1fr;gap:0;border-top:1px dashed var(--line-2);border-bottom:1px dashed var(--line-2)}
  @media (max-width:820px){.arch-grid{grid-template-columns:1fr}}
  .arch-col{padding:28px;border-right:1px dashed var(--line-2);position:relative}
  .arch-col:last-child{border-right:none}
  .arch-col h4{font-family:"Fraunces",serif;font-size:24px;margin-bottom:6px}
  .arch-col .sub{color:var(--muted);font-size:13px;margin-bottom:22px}
  .arch-box{padding:14px 16px;border-radius:10px;background:var(--bg-2);border:1px solid var(--line);margin-bottom:10px;font-size:13px;display:flex;justify-content:space-between;align-items:center}
  .arch-box .t{font-weight:500}
  .arch-box .t small{display:block;color:var(--muted);font-weight:400;font-size:11px;margin-top:3px}
  .arch-box .s{font-size:10px;color:var(--ok);padding:3px 7px;border-radius:999px;background:rgba(74,107,58,.12);font-family:"JetBrains Mono",monospace}
  .arch-box .s.w{color:var(--accent);background:rgba(184,66,28,.1)}

  .crypto-block{padding:64px 0}
  .crypto-row{display:grid;grid-template-columns:1fr 1fr;gap:48px;align-items:center}
  @media (max-width:820px){.crypto-row{grid-template-columns:1fr}}
  .merkle{aspect-ratio:4/3;border-radius:var(--radius-lg);background:var(--paper);border:1px solid var(--line);padding:32px;position:relative;display:grid;place-items:center}
  .merkle svg{width:100%;height:100%;max-height:320px}

  .standards{display:grid;grid-template-columns:repeat(4,1fr);gap:0;border-radius:var(--radius);overflow:hidden;border:1px solid var(--line);background:var(--paper)}
  @media (max-width:820px){.standards{grid-template-columns:1fr 1fr}}
  .std{padding:24px;border-right:1px solid var(--line);border-bottom:1px solid var(--line)}
  .std:nth-child(4n){border-right:none}
  .std:nth-last-child(-n+4){border-bottom:none}
  @media (max-width:820px){.std{border-right:1px solid var(--line)!important;border-bottom:1px solid var(--line)!important}.std:nth-child(2n){border-right:none!important}.std:nth-last-child(-n+2){border-bottom:none!important}}
  .std .name{font-family:"Fraunces",serif;font-size:22px}
  .std .name em{color:var(--accent);font-style:italic}
  .std .desc{font-size:12.5px;color:var(--muted);margin-top:4px}

  .audit{display:grid;grid-template-columns:1fr 1fr;gap:24px}
  @media (max-width:820px){.audit{grid-template-columns:1fr}}
  .audit-card{padding:32px;border-radius:var(--radius);background:var(--paper);border:1px solid var(--line)}
  .audit-card .firm{font-size:12px;color:var(--muted)}
  .audit-card h3{font-size:24px;margin:6px 0 14px}
  .audit-card p{color:var(--muted);font-size:14.5px;line-height:1.55}
  .audit-card .meta{margin-top:18px;display:flex;gap:20px;font-size:12px;color:var(--muted)}
  .audit-card a{color:var(--accent)}
`;

export default function SecurityPage() {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: pageStyles }} />

      <section className="wrap sec-hero">
        <span className="eyebrow"><span className="eb-dot"></span>Security model</span>
        <h1 style={{ marginTop: '20px' }}>We don&rsquo;t ask for <em>trust.</em><br />We give you <em>proofs.</em></h1>
        <p className="lead" style={{ marginTop: '24px' }}>Every claim VaultKeeper makes can be checked independently. The database enforces append-only at the engine level. The custody chain is verifiable with a MIT-licensed clerk tool &mdash; without us. The crypto is open. The threat model is written down. This page is what&rsquo;s in it.</p>

        <div className="layers">
          <div className="layer"><div className="num">i.</div><h3>Transport</h3><p>TLS 1.3 end-to-end via Caddy with automatic provisioning, HSTS, strict CSP, X-Frame DENY.</p></div>
          <div className="layer"><div className="num">ii.</div><h3>At rest</h3><p>MinIO server-side encryption (SSE-S3). Postgres disk encryption on the underlying volume.</p></div>
          <div className="layer"><div className="num">iii.</div><h3>Application</h3><p>AES-256-GCM envelope encryption of witness PII. Parameterized SQL, HTML escaping, filename sanitisation.</p></div>
          <div className="layer"><div className="num">iv.</div><h3>Network</h3><p>DB and MinIO never internet-exposed. Zero outbound calls. Air-gap compatible out of the box.</p></div>
        </div>
      </section>

      <section className="wrap section-tight">
        <div className="threat">
          <div>
            <span className="eyebrow" style={{ background: 'rgba(255,255,255,.05)', borderColor: 'rgba(255,255,255,.12)', color: 'var(--muted-2)' }}><span className="eb-dot"></span>Threat model</span>
            <h2 style={{ marginTop: '18px' }}>The four <em>attacks</em> that keep tribunals up at night.</h2>
            <p>Most security tools defend against an opportunistic attacker. International justice needs something harder: defence against a hostile state, a compromised cloud, and a corrupt insider &mdash; simultaneously. VaultKeeper&rsquo;s architecture makes each of these attacks fail loudly instead of silently.</p>
          </div>
          <div className="scenarios">
            <div className="scen"><div className="num">A1</div><div><h4>Sanction-driven shut-off</h4><p>A foreign government pressures a cloud to freeze your instance. Self-hosted deployment + AGPL source + public docker images means a tribunal relocates in hours, not quarters.</p></div></div>
            <div className="scen"><div className="num">A2</div><div><h4>Custody tampering by insider</h4><p>A root DB admin tries to edit the custody log. Row-level security refuses the UPDATE at the engine. Even if they bypass it, hash chaining makes the next read fail verification.</p></div></div>
            <div className="scen"><div className="num">A3</div><div><h4>Witness identity exposure</h4><p>Defence counsel should never see cleartext names. Application-layer AES-256-GCM means even a database dump contains only opaque blobs; pseudonyms are generated per role.</p></div></div>
            <div className="scen"><div className="num">A4</div><div><h4>Forced access under duress</h4><p>An investigator is coerced into logging in. A duress passphrase (planned, v1.3) unlocks a decoy vault while silently flagging the session in a signed audit record.</p></div></div>
          </div>
          <div className="blob" style={{ width: '500px', height: '500px', background: 'var(--accent)', right: '-180px', top: '-150px', opacity: 0.2 }}></div>
        </div>
      </section>

      <section className="section wrap architecture">
        <div style={{ textAlign: 'center', maxWidth: '760px', margin: '0 auto 36px' }}>
          <span className="eyebrow">Architecture</span>
          <h2 style={{ marginTop: '14px' }}>Three sealed layers, <em className="a">one</em> provable chain.</h2>
          <p className="lead" style={{ margin: '20px auto 0' }}>Every request passes through the same three gates: who are you, what may you do, and does this action belong on the chain? Each gate writes its outcome into the append-only log.</p>
        </div>
        <div className="arch-diagram">
          <div className="arch-title">Request flow &middot; all operations</div>
          <div className="arch-grid">
            <div className="arch-col">
              <h4>Perimeter</h4>
              <div className="sub">Caddy reverse proxy</div>
              <div className="arch-box"><div className="t">TLS 1.3<small>auto-provisioned certs</small></div><div className="s">on</div></div>
              <div className="arch-box"><div className="t">Rate limit<small>auth 100/min &middot; upload 10/min</small></div><div className="s">on</div></div>
              <div className="arch-box"><div className="t">CSP + HSTS<small>strict, no inline script</small></div><div className="s">on</div></div>
              <div className="arch-box"><div className="t">Outbound<small>air-gap mode available</small></div><div className="s w">denied</div></div>
            </div>
            <div className="arch-col">
              <h4>Identity</h4>
              <div className="sub">Keycloak OIDC / SAML</div>
              <div className="arch-box"><div className="t">MFA<small>WebAuthn or TOTP</small></div><div className="s">on</div></div>
              <div className="arch-box"><div className="t">System role<small>Admin &middot; Case &middot; User &middot; API</small></div><div className="s">matched</div></div>
              <div className="arch-box"><div className="t">Case role<small>Prosecutor &middot; Defence &middot; Judge &middot; Observer</small></div><div className="s">matched</div></div>
              <div className="arch-box"><div className="t">Session<small>15 min access &middot; 8 hr refresh</small></div><div className="s">fresh</div></div>
            </div>
            <div className="arch-col">
              <h4>Engine</h4>
              <div className="sub">Go &middot; PostgreSQL 16</div>
              <div className="arch-box"><div className="t">Permission matrix<small>role &times; action middleware</small></div><div className="s">allowed</div></div>
              <div className="arch-box"><div className="t">Row-level security<small>append-only, no UPDATE/DELETE</small></div><div className="s">enforced</div></div>
              <div className="arch-box"><div className="t">Hash chain<small>SHA-256 of previous row</small></div><div className="s">linked</div></div>
              <div className="arch-box"><div className="t">Advisory lock<small>serialised writes</small></div><div className="s">held</div></div>
            </div>
          </div>
          <div style={{ textAlign: 'center', marginTop: '24px', fontSize: '12px', color: 'var(--muted)' }}>Every successful and every rejected request is written to the custody log with reason.</div>
        </div>
      </section>

      <section className="section wrap crypto-block">
        <div className="crypto-row">
          <div className="merkle">
            <svg viewBox="0 0 420 320" fill="none" stroke="currentColor">
              <g stroke="var(--line-2)" strokeWidth="1">
                <line x1="210" y1="50" x2="110" y2="130" />
                <line x1="210" y1="50" x2="310" y2="130" />
                <line x1="110" y1="130" x2="60" y2="220" />
                <line x1="110" y1="130" x2="160" y2="220" />
                <line x1="310" y1="130" x2="260" y2="220" />
                <line x1="310" y1="130" x2="360" y2="220" />
              </g>
              <circle cx="210" cy="50" r="26" fill="var(--accent)" stroke="none" />
              <text x="210" y="54" textAnchor="middle" fill="#fff" fontFamily="JetBrains Mono" fontSize="9">root</text>
              <circle cx="110" cy="130" r="18" fill="var(--paper)" stroke="var(--accent)" strokeWidth="1.5" />
              <circle cx="310" cy="130" r="18" fill="var(--paper)" stroke="var(--accent)" strokeWidth="1.5" />
              <text x="110" y="134" textAnchor="middle" fill="var(--accent)" fontFamily="JetBrains Mono" fontSize="8">H(a,b)</text>
              <text x="310" y="134" textAnchor="middle" fill="var(--accent)" fontFamily="JetBrains Mono" fontSize="8">H(c,d)</text>
              <g>
                <rect x="40" y="205" width="40" height="30" rx="5" fill="var(--bg-2)" stroke="var(--line-2)" />
                <text x="60" y="224" textAnchor="middle" fill="var(--ink)" fontFamily="JetBrains Mono" fontSize="9">e01</text>
                <rect x="140" y="205" width="40" height="30" rx="5" fill="var(--bg-2)" stroke="var(--line-2)" />
                <text x="160" y="224" textAnchor="middle" fill="var(--ink)" fontFamily="JetBrains Mono" fontSize="9">e02</text>
                <rect x="240" y="205" width="40" height="30" rx="5" fill="var(--bg-2)" stroke="var(--line-2)" />
                <text x="260" y="224" textAnchor="middle" fill="var(--ink)" fontFamily="JetBrains Mono" fontSize="9">e03</text>
                <rect x="340" y="205" width="40" height="30" rx="5" fill="var(--bg-2)" stroke="var(--line-2)" />
                <text x="360" y="224" textAnchor="middle" fill="var(--ink)" fontFamily="JetBrains Mono" fontSize="9">e04</text>
              </g>
              <text x="210" y="290" textAnchor="middle" fill="var(--muted)" fontFamily="Fraunces" fontStyle="italic" fontSize="14">Each exhibit is a leaf. The root anchors them all.</text>
            </svg>
          </div>
          <div>
            <span className="eyebrow">Cryptographic spine</span>
            <h2 style={{ marginTop: '16px' }}>Hashes of hashes, <em>all the way up.</em></h2>
            <p className="lead" style={{ marginTop: '20px' }}>Every custody event is hashed with SHA-256 and combined into a Merkle tree. The root is signed, timestamped under RFC 3161, and optionally anchored to a public blockchain. A defence expert can verify any single event with a ~2&nbsp;KB proof &mdash; without trusting us, without being online.</p>
            <ul style={{ listStyle: 'none', padding: 0, margin: '28px 0 0', display: 'flex', flexDirection: 'column', gap: '10px' }}>
              <li style={{ padding: '10px 0', borderTop: '1px solid var(--line)', fontSize: '15px' }}><strong>SHA-256 + BLAKE3</strong> &mdash; primary + defence-in-depth hash</li>
              <li style={{ padding: '10px 0', borderTop: '1px solid var(--line)', fontSize: '15px' }}><strong>Ed25519</strong> &mdash; signing keys on a YubiHSM 2</li>
              <li style={{ padding: '10px 0', borderTop: '1px solid var(--line)', fontSize: '15px' }}><strong>RFC 3161 timestamps</strong> from eIDAS-registered TSAs</li>
              <li style={{ padding: '10px 0', borderTop: '1px solid var(--line)', fontSize: '15px' }}><strong>OpenTimestamps</strong> daily anchor to Bitcoin L1</li>
              <li style={{ padding: '10px 0', borderTop: '1px solid var(--line)', borderBottom: '1px solid var(--line)', fontSize: '15px' }}><strong>AES-256-GCM</strong> envelope encryption for witness PII</li>
            </ul>
          </div>
        </div>
      </section>

      <section className="section wrap" style={{ paddingTop: 0 }}>
        <div style={{ textAlign: 'center', maxWidth: '760px', margin: '0 auto 36px' }}>
          <span className="eyebrow">Standards</span>
          <h2 style={{ marginTop: '14px' }}>Recognised by the <em className="a">courts</em> you already appear before.</h2>
        </div>
        <div className="standards">
          <div className="std"><div className="name">ISO/IEC <em>27037</em></div><div className="desc">Digital evidence handling</div></div>
          <div className="std"><div className="name">ETSI EN <em>319 142</em></div><div className="desc">PAdES advanced signatures</div></div>
          <div className="std"><div className="name">RFC <em>3161</em></div><div className="desc">Trusted time-stamping</div></div>
          <div className="std"><div className="name">eIDAS <em>QES</em></div><div className="desc">Qualified electronic signature</div></div>
          <div className="std"><div className="name">SOC 2 <em>Type II</em></div><div className="desc">Audited annually</div></div>
          <div className="std"><div className="name">ISO <em>27001</em></div><div className="desc">Information security management</div></div>
          <div className="std"><div className="name">GDPR <em>Art. 28</em></div><div className="desc">Data processing agreement</div></div>
          <div className="std"><div className="name">FIPS <em>140-3</em></div><div className="desc">Optional HSM module (YubiHSM 2)</div></div>
        </div>
      </section>

      <section className="section wrap" style={{ paddingTop: 0 }}>
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '40px', alignItems: 'end', marginBottom: '30px' }}>
          <div>
            <span className="eyebrow">Independent review</span>
            <h2 style={{ marginTop: '14px' }}>The code has been <em className="a">read</em> by people who aren&rsquo;t us.</h2>
          </div>
          <p className="lead">Two external firms have audited VaultKeeper &mdash; full reports available under NDA to prospective institutional customers. A formal verification effort of the custody engine (TLA+ / Coq) is under way with an academic partner.</p>
        </div>

        <div className="audit">
          <div className="audit-card">
            <div className="firm">2025 &middot; NCC Group</div>
            <h3>Cryptographic architecture review</h3>
            <p>Full review of the custody engine, signing flow, and hash chaining. Two medium-severity findings, both remediated pre-v1.0. No critical or high findings.</p>
            <div className="meta"><span>58 pages</span><span>2 findings &middot; closed</span><a href="#">Request report &rarr;</a></div>
          </div>
          <div className="audit-card">
            <div className="firm">2025 &middot; Cure53</div>
            <h3>Web application &amp; API penetration test</h3>
            <p>Eight-person-week test across the Next.js frontend, Go API, redaction engine, and witness encryption paths. Zero high findings. All low findings remediated.</p>
            <div className="meta"><span>42 pages</span><span>0 highs</span><a href="#">Request report &rarr;</a></div>
          </div>
        </div>
      </section>

      <section className="wrap" style={{ padding: '24px 32px 64px' }}>
        <div className="cta-banner">
          <div>
            <h2>Send this page to your <em>CISO.</em></h2>
            <p>We answer security questionnaires in writing, on letterhead, with a named engineer on every response. Our threat model is a 38-page document, not a marketing page.</p>
          </div>
          <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap', justifyContent: 'flex-end' }}>
            <Link className="btn ghost" href="/docs">Read the threat model</Link>
            <Link className="btn" href="/contact">Talk to security <span className="arr">&rarr;</span></Link>
          </div>
          <div className="blob" style={{ width: '500px', height: '500px', background: 'var(--accent)', right: '-100px', top: '-200px', opacity: 0.25 }}></div>
        </div>
      </section>
    </>
  );
}
