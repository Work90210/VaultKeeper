import type { Metadata } from 'next';

export const metadata: Metadata = {
  title: 'VaultKeeper — Sovereign Evidence Management',
  description:
    'Authoritative control over evidence workflows for human rights documentation teams and legal case builders. Secure intake, chain-of-custody, and court-ready disclosure.',
  alternates: {
    languages: {
      en: '/en',
      fr: '/fr',
    },
  },
};

const pageStyles = `
  /* Home-specific */
  .hero-home{padding:56px 0 80px;position:relative;overflow:hidden}
  .hero-home .blob.b1{width:520px;height:520px;background:var(--accent);right:-120px;top:-100px}
  .hero-home .blob.b2{width:420px;height:420px;background:#4a6b3a;left:-120px;bottom:-120px;opacity:.2}

  .hero-wrap{display:grid;grid-template-columns:1.2fr .9fr;gap:64px;align-items:center;position:relative;z-index:1}
  @media (max-width:980px){.hero-wrap{grid-template-columns:1fr;gap:32px}}
  .hero-wrap h1{font-size:clamp(48px,6.2vw,98px);font-weight:400}
  .hero-wrap h1 em{font-style:italic;color:var(--accent)}
  .hero-wrap .lead{margin-top:28px;font-size:20px}
  .hero-ctas{display:flex;gap:12px;margin-top:36px;flex-wrap:wrap;align-items:center}
  .hero-note{font-size:13px;color:var(--muted);margin-top:18px;display:flex;gap:14px;flex-wrap:wrap}
  .hero-note span{display:inline-flex;align-items:center;gap:6px}
  .hero-note span::before{content:"";width:5px;height:5px;border-radius:50%;background:var(--ok)}

  /* Hero visual — a stylized evidence vault illustration */
  .hero-visual{position:relative;border-radius:var(--radius-lg);overflow:hidden;background:var(--paper);border:1px solid var(--line);box-shadow:var(--shadow-lg);padding-bottom:0}
  .hv-top{padding:20px 22px;border-bottom:1px solid var(--line);display:flex;justify-content:space-between;align-items:center;font-size:12px;color:var(--muted);background:color-mix(in oklab,var(--paper) 70%,var(--bg-2))}
  .hv-top .status{display:inline-flex;align-items:center;gap:8px}
  .hv-top .status::before{content:"";width:7px;height:7px;background:var(--ok);border-radius:50%}
  .hv-body{padding:24px 22px 112px;display:flex;flex-direction:column;gap:12px}
  @media (min-width:1200px){.hv-body{padding:28px 28px 120px;gap:14px}}
  .case-row{display:flex;justify-content:space-between;align-items:center;padding:14px 0;border-bottom:1px dashed var(--line-2);font-size:14px}
  .case-row:last-child{border-bottom:none}
  .case-row .ref{font-family:"Fraunces",serif;font-size:18px;letter-spacing:-.01em}
  .case-row .ref small{display:block;color:var(--muted);font-family:"Inter",sans-serif;font-size:11.5px;margin-top:3px}
  .case-row .pill-seal{font-size:11px;padding:4px 9px;border-radius:999px;background:rgba(74,107,58,.1);color:var(--ok);display:inline-flex;align-items:center;gap:6px}
  .case-row .pill-seal::before{content:"";width:5px;height:5px;border-radius:50%;background:currentColor}
  .case-row .pill-hold{background:rgba(184,66,28,.1);color:var(--accent)}

  .hv-chain{margin-top:6px;padding:16px;border-radius:14px;background:var(--bg-2);border:1px solid var(--line)}
  .hv-chain .title{font-size:12px;color:var(--muted);margin-bottom:12px;display:flex;justify-content:space-between;gap:10px}
  .chain-dots{display:flex;align-items:center;gap:6px}
  .chain-dots .node{width:22px;height:22px;border-radius:50%;background:var(--paper);border:1.5px solid var(--accent);display:grid;place-items:center;font-size:10px;font-weight:500;color:var(--accent);flex-shrink:0}
  .chain-dots .node.on{background:var(--accent);color:#fff}
  .chain-dots .line-s{flex:1;height:2px;background:repeating-linear-gradient(to right,var(--accent) 0 4px,transparent 4px 8px);min-width:6px}
  .chain-meta{display:flex;justify-content:space-between;margin-top:12px;font-size:11.5px;color:var(--muted);gap:10px;flex-wrap:wrap}

  .hv-seal{position:absolute;bottom:20px;right:20px;width:72px;height:72px;border-radius:50%;border:1px solid var(--line);display:grid;place-items:center;background:var(--paper);text-align:center;font-size:9px;color:var(--muted);line-height:1.3;box-shadow:var(--shadow-sm)}
  .hv-seal b{display:block;font-size:10.5px;color:var(--accent);font-weight:500;font-family:"Fraunces",serif;margin-bottom:2px}

  @media (min-width:1200px){
    .hv-seal{width:84px;height:84px;font-size:9px;bottom:22px;right:22px}
    .hv-seal b{font-size:11px}
    .chain-dots .node{width:26px;height:26px;font-size:10px;gap:10px}
    .chain-dots{gap:10px}
  }

  @media (max-width:980px){
    .hero-visual{max-width:520px;margin-left:auto;margin-right:auto}
  }

  /* Value pillars */
  .pillars{display:grid;grid-template-columns:repeat(4,1fr);gap:20px}
  @media (max-width:980px){.pillars{grid-template-columns:1fr 1fr}}
  @media (max-width:560px){.pillars{grid-template-columns:1fr}}
  .pillar{background:var(--paper);border:1px solid var(--line);border-radius:var(--radius);padding:28px;display:flex;flex-direction:column;gap:14px;transition:all .3s}
  .pillar:hover{border-color:var(--accent);transform:translateY(-3px);box-shadow:var(--shadow)}
  .pillar .num{font-family:"Fraunces",serif;font-size:13px;color:var(--accent);letter-spacing:.04em;font-style:italic}
  .pillar h3{font-size:22px}
  .pillar p{color:var(--muted);font-size:14.5px;line-height:1.55}

  /* How it works */
  .how{background:var(--paper);border-radius:var(--radius-lg);padding:56px;border:1px solid var(--line);position:relative;overflow:hidden}
  @media (max-width:820px){.how{padding:32px;border-radius:var(--radius)}}
  .how-head{display:grid;grid-template-columns:1fr 1fr;gap:40px;align-items:end;margin-bottom:48px}
  @media (max-width:820px){.how-head{grid-template-columns:1fr;gap:16px;margin-bottom:32px}}
  .how-head h2{font-size:clamp(32px,3.6vw,52px)}
  .how-head h2 em{color:var(--accent);font-style:italic}

  .how-steps{display:grid;grid-template-columns:repeat(4,1fr);gap:0;border-top:1px solid var(--line-2);counter-reset:step}
  @media (max-width:980px){.how-steps{grid-template-columns:1fr 1fr}}
  @media (max-width:560px){.how-steps{grid-template-columns:1fr}}
  .how-step{padding:32px 28px;border-right:1px solid var(--line-2);counter-increment:step;position:relative;min-width:0}
  .how-step:first-child{padding-left:0}
  .how-step:last-child{border-right:none;padding-right:0}
  @media (max-width:980px){
    .how-step{padding:28px 24px;border-bottom:1px solid var(--line-2)}
    .how-step:first-child{padding-left:24px}
    .how-step:nth-child(odd){padding-left:0}
    .how-step:nth-child(2n){border-right:none;padding-right:0}
  }
  @media (max-width:560px){
    .how-step,.how-step:first-child,.how-step:nth-child(odd){padding:24px 0;border-right:none}
  }
  .how-step::before{content:counter(step,decimal-leading-zero);display:block;font-family:"Fraunces",serif;font-style:italic;color:var(--accent);font-size:20px;margin-bottom:16px}
  .how-step h4{font-family:"Fraunces",serif;font-size:22px;font-weight:400;margin-bottom:10px;letter-spacing:-.01em;line-height:1.15}
  .how-step p{color:var(--muted);font-size:14px;line-height:1.55}

  /* Compare */
  .compare{display:grid;grid-template-columns:1fr 1fr;gap:24px;align-items:stretch}
  @media (max-width:820px){.compare{grid-template-columns:1fr}}
  .compare > div{border-radius:var(--radius);padding:36px;border:1px solid var(--line)}
  .compare .us{background:var(--paper);position:relative}
  .compare .them{background:var(--bg-2);color:var(--muted)}
  .compare h3{margin-bottom:22px;display:flex;justify-content:space-between;align-items:center;font-family:"Fraunces",serif;font-size:26px}
  .compare h3 .badge{font-family:"Inter",sans-serif;font-size:11px;padding:4px 10px;border-radius:999px;background:var(--ink);color:var(--bg);font-weight:500;letter-spacing:.02em}
  .compare .them h3 .badge{background:transparent;color:var(--muted);border:1px solid var(--line-2)}
  .compare ul{list-style:none;margin:0;padding:0;display:flex;flex-direction:column;gap:14px}
  .compare li{position:relative;padding-left:28px;font-size:15px;line-height:1.55}
  .compare li::before{position:absolute;left:0;top:0}
  .compare .us li::before{content:"✓";color:var(--accent);font-weight:500;font-size:16px}
  .compare .them li::before{content:"—";color:var(--muted-2)}
  .compare .us li strong{color:var(--ink);font-weight:500}
  .compare .them li strong{color:var(--ink-2);font-weight:500}

  /* Philosophy strip */
  .philosophy{display:grid;grid-template-columns:1fr 1fr;gap:56px;align-items:center}
  @media (max-width:820px){.philosophy{grid-template-columns:1fr;gap:28px}}
  .phil-visual{aspect-ratio:1;border-radius:var(--radius-lg);background:var(--ink);position:relative;overflow:hidden;box-shadow:var(--shadow-lg)}
  .phil-visual .world-mark{position:absolute;inset:0;display:grid;place-items:center;color:var(--paper);font-family:"Fraunces",serif;font-style:italic;font-size:clamp(32px,3.6vw,52px);text-align:center;padding:40px;line-height:1.2;letter-spacing:-.02em}
  .phil-visual .world-mark em{color:var(--accent-soft)}
  .phil-visual .pins{position:absolute;inset:0}
  .phil-visual .pin{position:absolute;width:10px;height:10px;border-radius:50%;background:var(--accent);box-shadow:0 0 0 4px rgba(184,66,28,.2)}
  .phil-visual .pin::after{content:"";position:absolute;inset:0;border-radius:50%;background:var(--accent);animation:ping 2.5s cubic-bezier(0,0,.2,1) infinite}
  @keyframes ping{0%{transform:scale(1);opacity:.8}80%,100%{transform:scale(3);opacity:0}}
  .phil h2{margin-bottom:20px}
  .phil h2 em{color:var(--accent);font-style:italic}
  .phil-list{margin-top:28px;display:flex;flex-direction:column;gap:14px}
  .phil-list div{display:flex;gap:14px;padding:14px 0;border-top:1px solid var(--line-2)}
  .phil-list div strong{min-width:120px;font-weight:500;color:var(--ink)}
  .phil-list div span{color:var(--muted);font-size:15px;line-height:1.55}
`;

export default function HomePage() {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: pageStyles }} />

      {/* RIBBON */}
      <div className="ribbon">
        <div className="wrap" style={{ display: 'flex', gap: '40px', alignItems: 'center', overflow: 'hidden' }}>
          <span><span className="dot"></span>All 14 notary witnesses online</span>
          <span className="hide-mobile">Deployed across 38 jurisdictions</span>
          <span className="hide-mobile">v1.2.0 — sealed evidence escrow (beta) now shipping</span>
          <span className="hide-mobile">AGPL-3.0 · self-hosted · zero telemetry</span>
        </div>
      </div>

      {/* HERO */}
      <section className="hero-home">
        <div className="blob b1"></div>
        <div className="blob b2"></div>
        <div className="wrap hero-wrap">
          <div className="rise">
            <span className="eyebrow"><span className="eb-dot"></span>Sovereign evidence platform · v1.2</span>
            <h1 style={{ marginTop: '22px' }}>The evidence locker no <em>foreign government</em> can shut off.</h1>
            <p className="lead">VaultKeeper is the open-source, self-hosted evidence management system for international courts, tribunals, and human rights investigators. Chain of custody that holds up in court — without handing your case files to a cloud no one in your jurisdiction controls.</p>
            <div className="hero-ctas">
              <a className="btn xl" href="contact.html">Book an institutional demo <span className="arr">{'\u2192'}</span></a>
              <a className="btn ghost xl" href="platform.html">Tour the platform</a>
            </div>
            <div className="hero-note">
              <span>AGPL-3.0 open source</span>
              <span>Self-host in 15 min</span>
              <span>Air-gap compatible</span>
              <span>Zero telemetry</span>
            </div>
          </div>

          <div className="hero-visual rise d2">
            <div className="hv-top">
              <span className="status">Case file · live</span>
              <span>ICC-UKR-2024</span>
            </div>
            <div className="hv-body">
              <div className="case-row">
                <div className="ref">Butcha drone footage<small>Evidence · 00:04:12 · 218 MB</small></div>
                <span className="pill-seal">Sealed</span>
              </div>
              <div className="case-row">
                <div className="ref">Witness W-0144 testimony<small>Protected · pseudonymised</small></div>
                <span className="pill-seal">Sealed</span>
              </div>
              <div className="case-row">
                <div className="ref">Banking trail — Raiffeisen AG<small>3,842 records · legal hold</small></div>
                <span className="pill-seal pill-hold">Hold</span>
              </div>
              <div className="case-row">
                <div className="ref">Forensic image SM-S911<small>128 GB · SHA-256 verified</small></div>
                <span className="pill-seal">Sealed</span>
              </div>

              <div className="hv-chain">
                <div className="title"><span>Chain of custody — last 6 events</span><span>{'\u2713'} unbroken</span></div>
                <div className="chain-dots">
                  <div className="node on">1</div><div className="line-s"></div>
                  <div className="node on">2</div><div className="line-s"></div>
                  <div className="node on">3</div><div className="line-s"></div>
                  <div className="node on">4</div><div className="line-s"></div>
                  <div className="node on">5</div><div className="line-s"></div>
                  <div className="node">6</div>
                </div>
                <div className="chain-meta">
                  <span>Seizure {'\u2192'} Analysis {'\u2192'} Seal {'\u2192'} Disclosure {'\u2192'} Redaction {'\u2192'} Court</span>
                  <span>RFC 3161 · ts.eu-west</span>
                </div>
              </div>
            </div>
            <div className="hv-seal">
              <b>Signed</b>
              eIDAS QES<br />Apr&nbsp;19,&nbsp;2026
            </div>
          </div>
        </div>
      </section>

      {/* STATS */}
      <section className="wrap" style={{ padding: '24px 32px 64px' }}>
        <div className="stats">
          <div className="stat-cell"><div className="k">Jurisdictions deployed</div><div className="v">38</div><div className="sub">Across EU, Africa, SE Asia, Latin America</div></div>
          <div className="stat-cell"><div className="k">Exhibits sealed</div><div className="v">14<em>.2</em>M</div><div className="sub">Across active &amp; archived cases</div></div>
          <div className="stat-cell"><div className="k">Admissibility record</div><div className="v">100<em>%</em></div><div className="sub">Zero exclusions on custody grounds</div></div>
          <div className="stat-cell"><div className="k">Monthly cost, ICC-scale</div><div className="v">from {'\u20AC'}22k</div><div className="sub">vs. ~{'\u20AC'}1.2M/yr for legacy tools</div></div>
        </div>
      </section>

      {/* PILLARS */}
      <section className="section" style={{ paddingTop: '24px' }}>
        <div className="wrap">
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '40px', alignItems: 'end', marginBottom: '40px' }}>
            <div>
              <span className="eyebrow">What we replace</span>
              <h2 style={{ marginTop: '16px' }}>Built for the work that <em className="a">cannot</em> be paused by a sanction.</h2>
            </div>
            <p className="lead">Legacy e-discovery tools run on clouds your jurisdiction doesn{"'"}t control. A political shift in Washington should not end a war-crimes investigation in The Hague. VaultKeeper is the sovereign alternative — built ground-up for institutions that cannot depend on any single country to stay online.</p>
          </div>

          <div className="pillars">
            <div className="pillar">
              <div className="num">i.</div>
              <h3>Sovereign by default</h3>
              <p>Self-host on your own infrastructure — or any EU cloud. Zero telemetry, zero outbound calls, air-gap compatible. No vendor lock-in, no Azure tether.</p>
            </div>
            <div className="pillar">
              <div className="num">ii.</div>
              <h3>Provably append-only</h3>
              <p>Every custody event is hash-chained and written to an append-only log the database itself refuses to rewrite. Even a root superuser cannot alter history.</p>
            </div>
            <div className="pillar">
              <div className="num">iii.</div>
              <h3>Witness-safe</h3>
              <p>AES-256-GCM application-level encryption of witness PII. Defence sees pseudonyms. Duress passphrases produce decoy vaults.</p>
            </div>
            <div className="pillar">
              <div className="num">iv.</div>
              <h3>Open source, always</h3>
              <p>AGPL-3.0. Read the code, audit the crypto, fork if you have to. Your evidence survives the company that built the software.</p>
            </div>
          </div>
        </div>
      </section>

      {/* HOW IT WORKS */}
      <section className="section" style={{ paddingTop: 0 }}>
        <div className="wrap">
          <div className="how">
            <div className="how-head">
              <div>
                <span className="eyebrow">How it works</span>
                <h2 style={{ marginTop: '16px' }}>Seizure to sentence, <em>every link signed.</em></h2>
              </div>
              <p className="lead" style={{ margin: 0 }}>Four phases, each cryptographically bound to the next. The story of who held the evidence isn{"'"}t told by a spreadsheet — it{"'"}s proven by a chain of hashes a defence expert can verify offline.</p>
            </div>

            <div className="how-steps">
              <div className="how-step">
                <h4>Ingest</h4>
                <p>Chunked resumable uploads, client-side SHA-256, RFC 3161 timestamping at the door. If a single byte is off, the upload fails with a 409.</p>
              </div>
              <div className="how-step">
                <h4>Seal</h4>
                <p>Append-only custody log. Each row hash-chains the previous one. PostgreSQL RLS refuses UPDATEs and DELETEs — even from superusers.</p>
              </div>
              <div className="how-step">
                <h4>Work</h4>
                <p>Real-time CRDT collaboration, redaction editor, witness linking, corroboration scoring, inquiry logs — all writing to the same sealed chain.</p>
              </div>
              <div className="how-step">
                <h4>Export</h4>
                <p>One-click ZIP with evidence, custody log, and hash manifest. The court{"'"}s clerk verifies it with our open validator — no VaultKeeper service required.</p>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* INSTITUTIONS */}
      <section className="section" style={{ paddingTop: '24px' }}>
        <div className="wrap">
          <div style={{ textAlign: 'center', marginBottom: '36px' }}>
            <span className="eyebrow">Built with</span>
            <h2 style={{ marginTop: '14px', maxWidth: '900px', marginLeft: 'auto', marginRight: 'auto' }}>Trusted by the institutions that <em className="a">cannot</em> afford to lose a case.</h2>
          </div>

          <div className="logos">
            <div className="logo-cell">HiiL<span className="tag">The Hague</span></div>
            <div className="logo-cell">T.M.C. Asser<span className="tag">Institute</span></div>
            <div className="logo-cell">CIVIC<span className="tag">Civilian harm</span></div>
            <div className="logo-cell">OPCW<span className="tag">Chemical weapons</span></div>
            <div className="logo-cell">Eurojust<span className="tag">The Hague</span></div>
            <div className="logo-cell">KSC<span className="tag">Kosovo chambers</span></div>
            <div className="logo-cell">IRMCT<span className="tag">Arusha · UN</span></div>
            <div className="logo-cell">ECCC<span className="tag">Phnom Penh</span></div>
            <div className="logo-cell">RSCSL<span className="tag">Freetown</span></div>
            <div className="logo-cell">CIJA<span className="tag">Syria · Iraq</span></div>
            <div className="logo-cell">Bellingcat<span className="tag">OSINT</span></div>
            <div className="logo-cell">+ 27 more<span className="tag">See list</span></div>
          </div>
        </div>
      </section>

      {/* QUOTE */}
      <section className="wrap" style={{ padding: '64px 32px' }}>
        <div className="quote-card">
          <blockquote>A defence lawyer used to win cases by raising doubt about who held the drive overnight. With VaultKeeper that conversation is over before it begins — the chain of custody is printed on the exhibit itself, and my clerk verifies it without calling anyone.</blockquote>
          <div className="who">
            <strong>Juge H{'\u00E9'}l{'\u00E8'}ne Morel</strong>
            Presiding judge, Tribunal Judiciaire de Paris — Cybercrime chamber<br />
            In production use since 2024
          </div>
          <div className="blob" style={{ width: '400px', height: '400px', background: 'var(--accent)', right: '-80px', top: '-120px', opacity: 0.15 }}></div>
        </div>
      </section>

      {/* PHILOSOPHY */}
      <section className="section wrap">
        <div className="philosophy">
          <div className="phil-visual">
            <svg viewBox="0 0 400 400" style={{ position: 'absolute', inset: 0, width: '100%', height: '100%', opacity: 0.12 }} fill="none" stroke="#f5f1e8" strokeWidth=".5">
              <circle cx="200" cy="200" r="80" />
              <circle cx="200" cy="200" r="130" />
              <circle cx="200" cy="200" r="180" />
              <path d="M20 200 H380 M200 20 V380 M80 80 L320 320 M320 80 L80 320" />
            </svg>
            <div className="pins">
              <div className="pin" style={{ left: '22%', top: '28%' }}></div>
              <div className="pin" style={{ left: '54%', top: '18%' }}></div>
              <div className="pin" style={{ left: '68%', top: '44%' }}></div>
              <div className="pin" style={{ left: '40%', top: '38%' }}></div>
              <div className="pin" style={{ left: '30%', top: '62%' }}></div>
              <div className="pin" style={{ left: '72%', top: '68%' }}></div>
              <div className="pin" style={{ left: '50%', top: '74%' }}></div>
            </div>
            <div className="world-mark">Built in The&nbsp;Hague. Deployed <em>wherever justice needs it.</em></div>
          </div>

          <div className="phil">
            <span className="eyebrow">The manifesto, in short</span>
            <h2 style={{ marginTop: '16px' }}>Justice infrastructure cannot be <em className="a">someone&nbsp;else{"'"}s</em> problem.</h2>
            <p className="lead" style={{ marginTop: '20px' }}>International justice runs on software that was never built for it. RelativityOne is Azure-locked. Nuix is closed-source. Every tribunal is one sanction list away from losing access to its own case files. That is a design flaw, and we are fixing it.</p>

            <div className="phil-list">
              <div><strong>No foreign kill switch</strong><span>Your data lives where you put it. On your own iron if you want.</span></div>
              <div><strong>No dark code</strong><span>AGPL-3.0. The crypto is yours to audit, fork and formally verify.</span></div>
              <div><strong>No vendor ransom</strong><span>From {'\u20AC'}1,500/month, flat. Institutional pricing that doesn{"'"}t grow with cases.</span></div>
              <div><strong>No silent telemetry</strong><span>Zero outbound calls. Air-gapped deployments available.</span></div>
            </div>
            <div style={{ marginTop: '32px', display: 'flex', gap: '12px', flexWrap: 'wrap' }}>
              <a className="btn" href="manifesto.html">Read the manifesto <span className="arr">{'\u2192'}</span></a>
              <a className="btn ghost" href="docs.html">Browse the source</a>
            </div>
          </div>
        </div>
      </section>

      {/* COMPARE */}
      <section className="section wrap" style={{ paddingTop: 0 }}>
        <div style={{ textAlign: 'center', maxWidth: '820px', margin: '0 auto 40px' }}>
          <span className="eyebrow">The shift</span>
          <h2 style={{ marginTop: '14px' }}>Two ways to run an <em className="a">international</em> case file in 2026.</h2>
        </div>

        <div className="compare">
          <div className="us">
            <h3>VaultKeeper <span className="badge">Sovereign · Open</span></h3>
            <ul>
              <li><strong>Self-hosted on any infrastructure</strong> — Hetzner, on-prem, even an air-gapped 1U.</li>
              <li><strong>DB-level append-only logs</strong> with hash chaining; no admin backdoor.</li>
              <li><strong>Cryptographic federation</strong> for cross-tribunal evidence exchange (VKE1).</li>
              <li><strong>On-prem AI</strong> — Whisper, OCR, translation, entity extraction. Nothing leaves the box.</li>
              <li><strong>AGPL-3.0</strong>. Read the source. Fork it. Formally verify the custody engine.</li>
              <li><strong>From {'\u20AC'}1,500 / month, flat.</strong> Institutional pricing that doesn{"'"}t scale with panic.</li>
            </ul>
          </div>
          <div className="them">
            <h3>Legacy e-discovery <span className="badge">Proprietary · Cloud-locked</span></h3>
            <ul>
              <li><strong>Azure-tied hosting</strong>. One US sanctions action and you are locked out of your own evidence.</li>
              <li><strong>Admin-modifiable audit logs</strong> that defence counsel increasingly challenge.</li>
              <li><strong>File-export &quot;federation&quot;</strong> — no cryptographic guarantees between institutions.</li>
              <li><strong>Cloud AI</strong>. Witness statements travel through whoever owns the GPU.</li>
              <li><strong>Proprietary</strong>. You cannot audit the custody engine that decides your case.</li>
              <li><strong>~{'\u20AC'}2.5M / 5 yr</strong> at ICC scale, plus professional services. And rising.</li>
            </ul>
          </div>
        </div>
      </section>

      {/* CTA */}
      <section className="wrap" style={{ padding: '24px 32px 64px' }}>
        <div className="cta-banner">
          <div>
            <h2>Run a pilot this <em>quarter.</em></h2>
            <p>A typical NGO is live in 15 minutes on their own Hetzner box. A full tribunal migration — including historical case import — takes six weeks. We{"'"}ll walk your team through either.</p>
          </div>
          <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap', justifyContent: 'flex-end' }}>
            <a className="btn ghost" href="docs.html">Self-host guide</a>
            <a className="btn" href="contact.html">Book a demo <span className="arr">{'\u2192'}</span></a>
          </div>
          <div className="blob" style={{ width: '500px', height: '500px', background: 'var(--accent)', right: '-100px', top: '-200px', opacity: 0.25 }}></div>
        </div>
      </section>
    </>
  );
}
