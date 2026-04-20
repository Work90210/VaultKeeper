import type { Metadata } from 'next';

export const metadata: Metadata = {
  title: 'Platform',
  description:
    'Explore VaultKeeper capabilities: secure evidence intake, chain-of-custody tracking, role-based access, disclosure management, collaborative redaction, and intelligent search.',
  alternates: {
    languages: {
      en: '/en/platform',
      fr: '/fr/platform',
    },
  },
};

const pageStyles = `
  .plat-hero{padding:48px 0 32px;position:relative}
  .plat-hero h1{font-size:clamp(44px,5.4vw,84px)}
  .plat-hero h1 em{color:var(--accent);font-style:italic}
  .plat-hero .lead{margin-top:24px;max-width:68ch}

  .jumpnav{display:flex;gap:6px;flex-wrap:wrap;margin-top:36px}
  .jumpnav a{padding:8px 14px;border-radius:999px;background:var(--paper);border:1px solid var(--line);font-size:13px;color:var(--ink-2);transition:all .2s}
  .jumpnav a:hover{background:var(--ink);color:var(--bg);border-color:var(--ink)}

  .feat-block{padding:80px 0;border-top:1px solid var(--line)}
  .feat-block:first-of-type{border-top:none}
  .feat-row{display:grid;grid-template-columns:.9fr 1.1fr;gap:64px;align-items:center}
  @media (max-width:920px){.feat-row{grid-template-columns:1fr;gap:32px}}
  .feat-row.rev{grid-template-columns:1.1fr .9fr}
  .feat-row.rev > .copy{order:2}
  @media (max-width:920px){.feat-row.rev{grid-template-columns:1fr}.feat-row.rev > .copy{order:0}}

  .copy h2{font-size:clamp(30px,3.2vw,44px);margin-bottom:18px}
  .copy h2 em{color:var(--accent);font-style:italic}
  .copy .eyebrow{margin-bottom:18px}
  .copy p{color:var(--muted);font-size:16px;line-height:1.6;max-width:52ch}
  .copy ul{list-style:none;margin:24px 0 0;padding:0;display:block}
  .copy li{position:relative;padding:0 0 10px 22px;font-size:15px;line-height:1.55}
  .copy li::before{content:"";position:absolute;left:2px;top:10px;width:6px;height:6px;border-radius:50%;background:var(--accent)}

  .visual{border-radius:var(--radius-lg);border:1px solid var(--line);background:var(--paper);overflow:hidden;box-shadow:var(--shadow-lg);position:relative}

  .vis-upload{padding:32px;display:flex;flex-direction:column;gap:16px;min-height:460px}
  .up-head{display:flex;justify-content:space-between;font-size:12px;color:var(--muted)}
  .up-drop{border:2px dashed var(--line-2);border-radius:14px;padding:28px;text-align:center;background:var(--bg-2)}
  .up-drop .big{font-family:"Fraunces",serif;font-size:24px;margin-bottom:6px}
  .up-files{display:flex;flex-direction:column;gap:10px}
  .up-file{display:grid;grid-template-columns:32px 1fr auto;gap:14px;padding:12px;background:var(--bg);border-radius:10px;align-items:center;border:1px solid var(--line)}
  .up-file .thumb{width:32px;height:32px;border-radius:6px;background:var(--bg-2);display:grid;place-items:center;color:var(--muted);font-size:11px;font-weight:500}
  .up-file .name{font-size:13.5px}
  .up-file .name small{display:block;color:var(--muted);font-size:11px;margin-top:2px;font-family:"JetBrains Mono",monospace}
  .up-file .bar{width:80px;height:3px;background:var(--line);border-radius:3px;overflow:hidden;position:relative}
  .up-file .bar > span{position:absolute;inset:0;background:var(--accent);border-radius:3px}
  .up-file.done .bar > span{background:var(--ok)}
  .up-file .pct{font-size:11px;color:var(--muted);text-align:right;min-width:28px}

  .vis-chain{padding:40px;min-height:460px;position:relative}
  .vis-chain .chain-title{font-size:12px;color:var(--muted);margin-bottom:20px;display:flex;justify-content:space-between}
  .chain-list{display:flex;flex-direction:column;gap:0;position:relative}
  .chain-list::before{content:"";position:absolute;left:18px;top:20px;bottom:20px;width:2px;background:repeating-linear-gradient(to bottom,var(--accent) 0 4px,transparent 4px 8px)}
  .chain-item{display:grid;grid-template-columns:36px 1fr auto;gap:16px;padding:14px 0;align-items:start;position:relative;z-index:1}
  .chain-item .dot{width:36px;height:36px;border-radius:50%;background:var(--paper);border:2px solid var(--accent);display:grid;place-items:center;font-family:"Fraunces",serif;font-style:italic;font-size:14px;color:var(--accent)}
  .chain-item.now .dot{background:var(--accent);color:#fff}
  .chain-item .what{font-size:14px;font-weight:500}
  .chain-item .what small{display:block;color:var(--muted);font-weight:400;font-size:12px;margin-top:3px}
  .chain-item .hash{font-family:"JetBrains Mono",monospace;font-size:10.5px;color:var(--muted);text-align:right}
  .chain-item .hash .v{color:var(--ok)}

  .vis-witness{padding:32px;min-height:460px;display:flex;flex-direction:column;gap:18px}
  .w-tabs{display:flex;gap:8px;padding:4px;background:var(--bg-2);border-radius:999px;font-size:12px}
  .w-tabs span{padding:6px 12px;border-radius:999px;color:var(--muted);cursor:pointer}
  .w-tabs span.on{background:var(--paper);color:var(--ink);box-shadow:var(--shadow-sm)}
  .w-card{padding:20px;border-radius:14px;background:var(--bg-2);border:1px solid var(--line);position:relative}
  .w-card .w-name{font-family:"Fraunces",serif;font-size:22px;display:flex;align-items:center;gap:10px}
  .w-card .redacted{display:inline-block;background:var(--ink);color:var(--ink);border-radius:4px;padding:2px 8px;font-family:"Fraunces",serif;letter-spacing:.04em;filter:blur(0)}
  .w-card .redacted.real{background:transparent;color:var(--ink)}
  .w-meta{display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-top:14px;font-size:13px}
  .w-meta .k{color:var(--muted);font-size:11px;display:block;margin-bottom:3px}
  .w-banner{display:flex;gap:10px;padding:10px 14px;background:rgba(184,66,28,.08);border:1px solid rgba(184,66,28,.2);border-radius:10px;font-size:12.5px;color:var(--accent)}

  .vis-search{padding:32px;min-height:460px}
  .srch-bar{display:flex;gap:10px;padding:14px 16px;background:var(--bg-2);border:1px solid var(--line);border-radius:12px;align-items:center;font-size:14px}
  .srch-bar .srch-input{flex:1;background:transparent;border:0;outline:0;font:inherit;color:var(--ink)}
  .srch-chips{display:flex;gap:8px;margin-top:14px;flex-wrap:wrap}
  .srch-chips span{padding:5px 11px;background:var(--bg-2);border-radius:999px;font-size:12px;color:var(--muted);border:1px solid var(--line)}
  .srch-chips span.on{background:var(--ink);color:var(--bg);border-color:var(--ink)}
  .srch-res{margin-top:24px;display:flex;flex-direction:column;gap:14px}
  .srch-item{padding:16px;border-radius:12px;border:1px solid var(--line);background:var(--bg)}
  .srch-item .type{font-size:11px;color:var(--muted);text-transform:uppercase;letter-spacing:.05em;margin-bottom:6px}
  .srch-item .title{font-family:"Fraunces",serif;font-size:19px;margin-bottom:4px}
  .srch-item .snippet{font-size:13px;color:var(--muted);line-height:1.5}
  .srch-item .snippet mark{background:rgba(184,66,28,.18);color:var(--accent);padding:0 3px;border-radius:3px}
  .srch-item .tags{display:flex;gap:6px;margin-top:10px;font-size:11px;color:var(--muted)}
  .srch-item .tags span{padding:2px 8px;border-radius:999px;background:var(--bg-2)}

  .vis-collab{padding:32px;min-height:460px;position:relative}
  .coll-doc{padding:24px;border-radius:14px;background:var(--bg-2);border:1px solid var(--line);min-height:340px;font-family:"Fraunces",serif;line-height:1.55;font-size:15px;position:relative}
  .coll-doc h5{font-family:"Fraunces",serif;font-size:22px;margin:0 0 10px;font-weight:400}
  .coll-doc p{margin:10px 0;color:var(--ink-2)}
  .coll-doc .hl1{background:rgba(184,66,28,.15);border-bottom:1.5px solid var(--accent);padding:0 2px}
  .coll-doc .hl2{background:rgba(74,107,58,.18);border-bottom:1.5px solid var(--ok);padding:0 2px}
  .coll-cursor{position:absolute;display:inline-flex;flex-direction:column;align-items:flex-start}
  .coll-cursor::before{content:"";width:2px;height:18px;background:var(--accent)}
  .coll-cursor .tag{background:var(--accent);color:#fff;font-size:10px;padding:2px 6px;border-radius:4px;margin-top:2px;font-family:"Inter",sans-serif;white-space:nowrap}
  .coll-people{display:flex;gap:-6px;margin-bottom:14px}
  .coll-people .av{width:28px;height:28px;border-radius:50%;border:2px solid var(--paper);background:var(--accent);color:#fff;font-size:11px;display:grid;place-items:center;margin-left:-6px;font-weight:500}
  .coll-people .av:first-child{margin-left:0}
  .coll-people .av.g{background:var(--ok)}
  .coll-people .av.b{background:#3b5b8a}
  .coll-head{display:flex;justify-content:space-between;align-items:center;margin-bottom:14px;font-size:12px;color:var(--muted)}

  .vis-redact{padding:32px;min-height:460px}
  .red-head{display:flex;justify-content:space-between;font-size:12px;color:var(--muted);margin-bottom:18px}
  .red-compare{display:grid;grid-template-columns:1fr 1fr;gap:14px}
  .red-pane{padding:20px;border-radius:12px;border:1px solid var(--line);background:var(--bg-2);font-size:13px;line-height:1.55;font-family:"Fraunces",serif;min-height:300px}
  .red-pane .label{font-family:"Inter",sans-serif;font-size:11px;color:var(--muted);margin-bottom:10px;text-transform:uppercase;letter-spacing:.05em}
  .red-pane .block{background:var(--ink);color:var(--ink);padding:0 4px;border-radius:3px;display:inline-block;height:1em;width:var(--w,120px);vertical-align:middle}
  .red-pane .b-short{--w:60px}
  .red-pane .b-med{--w:110px}
  .red-pane .b-long{--w:180px}

  .vis-ts{padding:32px;min-height:460px;display:flex;flex-direction:column;gap:18px}
  .ts-card{padding:22px;border-radius:14px;background:var(--bg-2);border:1px solid var(--line)}
  .ts-card h4{font-family:"Fraunces",serif;font-size:20px;margin-bottom:6px}
  .ts-card p{color:var(--muted);font-size:13px;line-height:1.5}
  .ts-meta{margin-top:14px;display:grid;grid-template-columns:1fr 1fr;gap:10px;font-size:12px}
  .ts-meta .k{font-family:"JetBrains Mono",monospace;font-size:10px;color:var(--muted);text-transform:uppercase;letter-spacing:.05em;display:block;margin-bottom:3px}
  .ts-meta .v{font-family:"JetBrains Mono",monospace;font-size:11.5px;color:var(--ink);word-break:break-all}

  .mini-features{display:grid;grid-template-columns:repeat(3,1fr);gap:20px;margin-top:24px}
  @media (max-width:760px){.mini-features{grid-template-columns:1fr}}
  .mini-features .mf{padding:18px;border-radius:12px;background:var(--bg-2);border:1px solid var(--line)}
  .mini-features .mf h5{font-family:"Fraunces",serif;font-size:17px;margin-bottom:4px}
  .mini-features .mf p{font-size:13px;color:var(--muted);line-height:1.45}
`;

export default function FeaturesPage() {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: pageStyles }} />

      {/* HERO */}
      <section className="wrap plat-hero">
        <div className="crumb"><a href="/">VaultKeeper</a> <span className="sep">/</span> <span>Platform</span></div>
        <span className="eyebrow"><span className="eb-dot"></span>The platform</span>
        <h1 style={{ marginTop: '20px' }}>A case file that holds up — from the <em>field</em> to the <em>verdict.</em></h1>
        <p className="lead">Thirty-odd features, one cryptographic spine. Every action you take — an upload, a redaction, a disclosure, a shared witness note — lands on the same append-only chain that the court&apos;s clerk will later verify. What follows is the platform, phase by phase.</p>
        <div className="jumpnav">
          <a href="#evidence">Evidence management</a>
          <a href="#custody">Chain of custody</a>
          <a href="#witness">Witness protection</a>
          <a href="#search">Search &amp; discovery</a>
          <a href="#collab">Live collaboration</a>
          <a href="#redact">Redactions &amp; disclosure</a>
          <a href="#timestamp">Trusted timestamping</a>
          <a href="#ops">Ops, backups, API</a>
        </div>
      </section>

      {/* EVIDENCE */}
      <section className="feat-block" id="evidence">
        <div className="wrap feat-row">
          <div className="copy">
            <span className="eyebrow">01 &middot; Evidence management</span>
            <h2>Upload a 10 GB drone capture from a <em>field laptop on satellite.</em></h2>
            <p>Resumable chunked uploads over the tus protocol. The browser hashes every chunk with SHA-256 before it&apos;s sent; the server verifies on arrival and refuses with a 409 if a single byte is off. EXIF and metadata are extracted automatically. Versioning is a parent-child chain — no &quot;overwrite&quot;, ever.</p>
            <ul>
              <li>Resumable uploads up to 10 GB, auto-retry on drop</li>
              <li>Client-side SHA-256 with Web Crypto API</li>
              <li>EXIF, codec, and file-system metadata captured on ingest</li>
              <li>Immutable version history; previews for image, PDF, audio, video</li>
              <li>Classification levels: Public / Restricted / Confidential / Ex-Parte</li>
            </ul>
          </div>
          <div className="visual">
            <div className="vis-upload">
              <div className="up-head"><span>Evidence &middot; upload</span><span>ICC-UKR-2024</span></div>
              <div className="up-drop">
                <div className="big">Drop files, or resume in progress</div>
                <div className="small">Chunked &middot; verified &middot; signed on arrival</div>
              </div>
              <div className="up-files">
                <div className="up-file done">
                  <div className="thumb">MP4</div>
                  <div className="name">butcha_drone_cam3.mp4<small>sha256 9f4a 2e1b c7d0 88ac &middot; 218 MB</small></div>
                  <div style={{ display: 'flex', gap: '10px', alignItems: 'center' }}><div className="bar"><span style={{ width: '100%' }}></span></div><div className="pct" style={{ color: 'var(--ok)' }}>{'\u2713'}</div></div>
                </div>
                <div className="up-file">
                  <div className="thumb">ISO</div>
                  <div className="name">forensic_image_SM-S911.dd<small>128 GB &middot; 87% &middot; chunk 4218/4822</small></div>
                  <div style={{ display: 'flex', gap: '10px', alignItems: 'center' }}><div className="bar"><span style={{ width: '87%' }}></span></div><div className="pct">87%</div></div>
                </div>
                <div className="up-file">
                  <div className="thumb">PDF</div>
                  <div className="name">banking_raiffeisen_Q3.pdf<small>sha256 41de 9c02 88... &middot; 4.1 MB</small></div>
                  <div style={{ display: 'flex', gap: '10px', alignItems: 'center' }}><div className="bar"><span style={{ width: '52%' }}></span></div><div className="pct">52%</div></div>
                </div>
                <div className="up-file done">
                  <div className="thumb">WAV</div>
                  <div className="name">interview_w0144_pt3.wav<small>00:42:18 &middot; AES-256-GCM at rest</small></div>
                  <div style={{ display: 'flex', gap: '10px', alignItems: 'center' }}><div className="bar"><span style={{ width: '100%' }}></span></div><div className="pct" style={{ color: 'var(--ok)' }}>{'\u2713'}</div></div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* CUSTODY */}
      <section className="feat-block" id="custody">
        <div className="wrap feat-row rev">
          <div className="visual">
            <div className="vis-chain">
              <div className="chain-title"><span>Chain of custody &middot; EX-01842</span><span>{'\u2713'} unbroken &middot; 7 events</span></div>
              <div className="chain-list">
                <div className="chain-item">
                  <div className="dot">i.</div>
                  <div className="what">Seizure — Field ingest<small>det.abellan@interpol &middot; Zagreb</small></div>
                  <div className="hash">09:41 <span className="v">{'\u2713'}</span><br />9f4a 2e1b</div>
                </div>
                <div className="chain-item">
                  <div className="dot">ii.</div>
                  <div className="what">Forensic image<small>lab-03 &middot; write-blocked dd</small></div>
                  <div className="hash">10:58 <span className="v">{'\u2713'}</span><br />c7d0 88ac</div>
                </div>
                <div className="chain-item">
                  <div className="dot">iii.</div>
                  <div className="what">Sealed<small>RFC 3161 timestamp &middot; ts.eu-west</small></div>
                  <div className="hash">11:04 <span className="v">{'\u2713'}</span><br />6f39 1e22</div>
                </div>
                <div className="chain-item">
                  <div className="dot">iv.</div>
                  <div className="what">Classification &middot; Confidential<small>prosecutor.lang</small></div>
                  <div className="hash">12:18 <span className="v">{'\u2713'}</span><br />e812 91bb</div>
                </div>
                <div className="chain-item">
                  <div className="dot">v.</div>
                  <div className="what">Transfer {'\u2192'} defence disclosure<small>kovac {'\u2192'} def.counsel</small></div>
                  <div className="hash">13:47 <span className="v">{'\u2713'}</span><br />41de 9c02</div>
                </div>
                <div className="chain-item now">
                  <div className="dot">vi.</div>
                  <div className="what">Court export &middot; PAdES-LTA<small>judge.morel@tj-paris</small></div>
                  <div className="hash">14:02 <span className="v">{'\u2713'}</span><br />b14c 7a05</div>
                </div>
              </div>
            </div>
          </div>
          <div className="copy">
            <span className="eyebrow">02 &middot; Chain of custody</span>
            <h2>An audit trail the database <em>itself</em> refuses to rewrite.</h2>
            <p>The custody log is enforced at the Postgres level with row-level security. Every entry carries the SHA-256 of the previous entry — a tamper-evident linked list that fails verification if anyone, including a root superuser, alters history. Advisory locks serialize concurrent writes; there are no race conditions to exploit.</p>
            <ul>
              <li>Append-only, RLS-enforced; <code>UPDATE</code> and <code>DELETE</code> are statically impossible</li>
              <li>Hash chaining across every row; a break is visible immediately</li>
              <li>Every action logged — upload, view, share, classify, disclose, destroy, redact</li>
              <li>Court-submittable PDF custody reports, one click</li>
              <li>Formal verification of the engine in progress (TLA+ / Coq)</li>
            </ul>
          </div>
        </div>
      </section>

      {/* WITNESS */}
      <section className="feat-block" id="witness">
        <div className="wrap feat-row">
          <div className="copy">
            <span className="eyebrow">03 &middot; Witness protection</span>
            <h2>Names that <em>only</em> the right people ever see.</h2>
            <p>Witness PII — full name, contact info, location — is encrypted at the application layer with AES-256-GCM, not just at rest. Defence counsel never sees cleartext; they see a pseudonym like <span className="mono">W-0144</span>. Protection levels, threat models, and safety profiles live alongside the witness record, all searchable without ever exposing the identity.</p>
            <ul>
              <li>AES-256-GCM envelope encryption of <code>full_name</code>, <code>contact_info</code>, <code>location</code></li>
              <li>Defence view: pseudonyms only. Prosecution view: clear-text on signed access.</li>
              <li>Standard / Protected / High-risk tiers with per-witness threat profiles</li>
              <li>Duress passphrase (planned): yields a decoy vault, signals silently</li>
              <li>Every access written to the custody chain, with reason and role</li>
            </ul>
          </div>
          <div className="visual">
            <div className="vis-witness">
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <div style={{ fontFamily: "'Fraunces',serif", fontSize: '20px' }}>Witness register</div>
                <div className="w-tabs">
                  <span className="on">Defence view</span>
                  <span>Prosecution</span>
                  <span>Judge</span>
                </div>
              </div>
              <div className="w-banner">{'\u25CF'} Identity fields are encrypted. Defence sees pseudonyms by policy.</div>
              <div className="w-card">
                <div className="w-name"><span className="redacted">&nbsp;W-0144&nbsp;</span> <span className="small" style={{ color: 'var(--muted)' }}>&middot; Protected</span></div>
                <div className="w-meta">
                  <div><span className="k">Real name</span><span className="redacted">&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;</span></div>
                  <div><span className="k">Location</span><span className="redacted">&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;</span></div>
                  <div><span className="k">Protection tier</span> High risk</div>
                  <div><span className="k">Linked exhibits</span> 4</div>
                </div>
              </div>
              <div className="w-card">
                <div className="w-name"><span className="redacted">&nbsp;W-0145&nbsp;</span> <span className="small" style={{ color: 'var(--muted)' }}>&middot; Standard</span></div>
                <div className="w-meta">
                  <div><span className="k">Real name</span><span className="redacted">&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;</span></div>
                  <div><span className="k">Location</span><span className="redacted">&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;</span></div>
                  <div><span className="k">Protection tier</span> Standard</div>
                  <div><span className="k">Linked exhibits</span> 2</div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* SEARCH */}
      <section className="feat-block" id="search">
        <div className="wrap feat-row rev">
          <div className="visual">
            <div className="vis-search">
              <div className="srch-bar">
                <span style={{ color: 'var(--muted)' }}>{'\u2315'}</span>
                <span className="srch-input">raiffeisen transfer 2023</span>
                <span className="small">{'\u2318'}K</span>
              </div>
              <div className="srch-chips">
                <span className="on">case: ICC-UKR-2024</span>
                <span className="on">classification: confidential</span>
                <span>type: document</span>
                <span>tag: banking</span>
                <span>+ add filter</span>
              </div>
              <div className="srch-res">
                <div className="srch-item">
                  <div className="type">Document &middot; EX-01842</div>
                  <div className="title">Raiffeisen AG transfer ledger, Q3 2023</div>
                  <div className="snippet">...wire from account 0x4a91 to <mark>Raiffeisen</mark> correspondent in Nicosia on 2023-09-14, <mark>transfer</mark> reference ICC-UKR... referenced in witness testimony W-0144...</div>
                  <div className="tags"><span>banking</span><span>wire-transfer</span><span>3 versions</span></div>
                </div>
                <div className="srch-item">
                  <div className="type">Witness statement &middot; W-0144</div>
                  <div className="title">Pseudonymous testimony, session 03</div>
                  <div className="snippet">...&quot;my supervisor asked me to authorise the <mark>Raiffeisen</mark> <mark>transfer</mark> even though the counterparty was not on our list...&quot; (audio 00:14:22)...</div>
                  <div className="tags"><span>testimony</span><span>protected</span><span>audio</span></div>
                </div>
                <div className="srch-item">
                  <div className="type">Analysis note &middot; prosecutor.lang</div>
                  <div className="title">Corroboration — banking trail {'\u2194'} W-0144</div>
                  <div className="snippet">Cross-case reference to ICC-RUS-2023 exhibit EX-00912 which shows similar <mark>Raiffeisen</mark> routing...</div>
                  <div className="tags"><span>analysis</span><span>cross-case</span><span>strength: strong</span></div>
                </div>
              </div>
            </div>
          </div>
          <div className="copy">
            <span className="eyebrow">04 &middot; Search &amp; discovery</span>
            <h2>A search bar that <em>respects</em> the permission matrix.</h2>
            <p>Meilisearch indexes every exhibit, note, and witness record — with typo tolerance and faceted filters — but every query is scoped by role. Defence never surfaces witness identities. Observers never surface ex-parte material. The same query returns different, legally correct result sets depending on who is asking.</p>
            <ul>
              <li>Typo-tolerant full-text search, sub-50ms p95 on 10M+ exhibits</li>
              <li>Faceted filtering by case, classification, type, tag, date range</li>
              <li>Role-scoped results — defence, prosecution, judge, observer, victim rep</li>
              <li>Highlighting in snippets so you know why a result matched</li>
              <li>Saved searches, shareable filter states, CSV export for subsets</li>
            </ul>
          </div>
        </div>
      </section>

      {/* COLLAB */}
      <section className="feat-block" id="collab">
        <div className="wrap feat-row">
          <div className="copy">
            <span className="eyebrow">05 &middot; Live collaboration</span>
            <h2>Three investigators, <em>one analysis note.</em> No merge conflicts.</h2>
            <p>Analysis notes, assessments, inquiry logs, and case timelines are backed by Yjs CRDTs over WebSockets. Presence indicators show who is reading and who is typing. Network drops? The document reconciles itself when connectivity returns. Every save is a new version on the custody chain — so you can always trace who changed what, when.</p>
            <ul>
              <li>Yjs CRDT — conflict-free concurrent editing, offline-first</li>
              <li>Presence tracking — cursors, selections, avatars in the margin</li>
              <li>Automatic reconnection; no lost edits on a dropped satellite link</li>
              <li>Version history stored on the same custody chain as exhibits</li>
              <li>Rich text + linked mentions to exhibits, witnesses, and other notes</li>
            </ul>
          </div>
          <div className="visual">
            <div className="vis-collab">
              <div className="coll-head">
                <span>Analysis note &middot; corroboration — banking {'\u2194'} W-0144</span>
                <div className="coll-people">
                  <div className="av">HM</div>
                  <div className="av g">PL</div>
                  <div className="av b">AK</div>
                </div>
              </div>
              <div className="coll-doc">
                <h5>Summary</h5>
                <p>The <span className="hl1">Raiffeisen correspondent in Nicosia</span> appears in both Q3 2023 ledger (EX-01842) and in testimony W-0144 session 03. Witness identifies the authorising officer by nickname &quot;Kolya&quot; which matches the signature block on the scanned wire authorisation.</p>
                <p>Timing is within a 48-hour window: the transfer instruction is time-stamped <span className="hl2">2023-09-14 11:04 UTC</span>, and the witness places the meeting at Kyiv, afternoon of the same day.</p>
                <p>Proposed strength: <strong>Strong corroboration</strong>. Recommend linking to ICC-RUS-2023 (EX-00912)...</p>
                <div className="coll-cursor" style={{ left: '42%', top: '115px' }}><span className="tag">Paola L.</span></div>
                <div className="coll-cursor" style={{ left: '12%', top: '185px', color: 'var(--ok)' }}><span className="tag" style={{ background: 'var(--ok)' }}>Aleks K.</span></div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* REDACTION */}
      <section className="feat-block" id="redact">
        <div className="wrap feat-row rev">
          <div className="visual">
            <div className="vis-redact">
              <div className="red-head"><span>Redaction editor &middot; EX-01842 p.14</span><span>OCR + visual mask</span></div>
              <div className="red-compare">
                <div className="red-pane">
                  <div className="label">Original (never modified)</div>
                  <p>On 14 September 2023, Oleksandr Kovalenko authorised a transfer of {'\u20AC'}2,400,000 to Raiffeisen Zentralbank&apos;s Nicosia branch, reference ICC-UKR/9142.</p>
                  <p>Beneficiary account: AT48 3200 0000 1237 4432, held by Olivia Marchenko, registered at Kyrylivska 56, Kyiv.</p>
                </div>
                <div className="red-pane">
                  <div className="label">Disclosure copy — defence</div>
                  <p>On 14 September 2023, <span className="block b-med"></span> authorised a transfer of {'\u20AC'}2,400,000 to Raiffeisen Zentralbank&apos;s Nicosia branch, reference ICC-UKR/9142.</p>
                  <p>Beneficiary account: <span className="block b-long"></span>, held by <span className="block b-med"></span>, registered at <span className="block b-long"></span>.</p>
                </div>
              </div>
              <div style={{ marginTop: '18px', display: 'flex', justifyContent: 'space-between', fontSize: '12px', color: 'var(--muted)' }}>
                <span>4 masks &middot; 1 draft &middot; not finalized</span>
                <span>side-by-side &middot; version 2</span>
              </div>
            </div>
          </div>
          <div className="copy">
            <span className="eyebrow">06 &middot; Redactions &amp; disclosure</span>
            <h2>Defence gets the <em>redacted copy.</em> The original never moves.</h2>
            <p>Our redaction editor OCRs the page, lets you drag masks over names, accounts, and locations — and writes a new disclosure version alongside the original. Drafts aren&apos;t finalized until you explicitly say so. Disclosure packages bundle redacted versions with a manifest, signed and hash-chained.</p>
            <ul>
              <li>Visual mask editor on top of OCR text layer</li>
              <li>Side-by-side compare: original vs. each redacted version</li>
              <li>Multi-item disclosure packages with full recipient tracking</li>
              <li>Drafts, finalization states, and revocation paths</li>
              <li>Every redaction written to the custody chain with reason</li>
            </ul>
          </div>
        </div>
      </section>

      {/* TIMESTAMPING */}
      <section className="feat-block" id="timestamp">
        <div className="wrap feat-row">
          <div className="copy">
            <span className="eyebrow">07 &middot; Trusted timestamping</span>
            <h2>A time the <em>court</em> already trusts.</h2>
            <p>Every sealed exhibit gets a RFC 3161 timestamp from an external, qualified Time-Stamping Authority — the same legal instrument used in EU qualified e-signatures. Your case doesn&apos;t depend on VaultKeeper&apos;s clock. It depends on a certificate a European court already recognises.</p>
            <ul>
              <li>RFC 3161 timestamps from eIDAS-registered TSAs</li>
              <li>Optional secondary anchor to a public blockchain (OpenTimestamps)</li>
              <li>Verifiable offline with our MIT-licensed clerk validator</li>
              <li>Timestamps survive the company that issued them</li>
            </ul>
          </div>
          <div className="visual">
            <div className="vis-ts">
              <div className="ts-card">
                <h4>RFC 3161 timestamp</h4>
                <p>Signed by GlobalSign EU QTSP on 19 April 2026 at 14:02:31 UTC. Token validated against the EU Trusted List.</p>
                <div className="ts-meta">
                  <div><span className="k">TSA</span><span className="v">GlobalSign &middot; EU QTSP</span></div>
                  <div><span className="k">Algorithm</span><span className="v">ECDSA P-384</span></div>
                  <div><span className="k">Time</span><span className="v">2026-04-19T14:02:31Z</span></div>
                  <div><span className="k">Serial</span><span className="v">0x4f21 8a1c 9bd0</span></div>
                </div>
              </div>
              <div className="ts-card">
                <h4>Public anchor (OpenTimestamps)</h4>
                <p>Secondary proof anchored into the Bitcoin blockchain at block height 899,412. 6 confirmations required before considered final.</p>
                <div className="ts-meta">
                  <div><span className="k">Chain</span><span className="v">Bitcoin L1</span></div>
                  <div><span className="k">Height</span><span className="v">899,412</span></div>
                  <div><span className="k">Confirmations</span><span className="v">8 {'\u2713'}</span></div>
                  <div><span className="k">Proof size</span><span className="v">~2 KB</span></div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* OPS */}
      <section className="feat-block" id="ops">
        <div className="wrap">
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '64px', alignItems: 'end', marginBottom: '36px' }}>
            <div>
              <span className="eyebrow">08 &middot; Ops &amp; integration</span>
              <h2 style={{ marginTop: '14px' }}>The <em>invisible</em> half that keeps tribunals running.</h2>
            </div>
            <p className="lead">Chain of custody is the part users see. Underneath sits the infrastructure that keeps an air-gapped deployment in Phnom Penh as happy as a ten-seat NGO in The Hague.</p>
          </div>

          <div className="mini-features" style={{ gridTemplateColumns: 'repeat(4,1fr)' }}>
            <div className="mf"><h5>Auth &middot; Keycloak</h5><p>OIDC + SAML. Delegated MFA. System + case roles. 15-min access tokens, 8-hr refresh, 3 concurrent sessions.</p></div>
            <div className="mf"><h5>Backups</h5><p>Nightly encrypted Postgres dumps, MinIO snapshots to a separate Storage Box. RTO &lt; 1 hr, RPO 24 hrs.</p></div>
            <div className="mf"><h5>Health &amp; alerts</h5><p>Public <code>/health</code>. Admin dashboard covers Postgres, MinIO, Meilisearch, disk, last-backup age.</p></div>
            <div className="mf"><h5>API keys</h5><p>Case-scoped, read-only or read-write. Rate-limited. Every call written to the custody chain.</p></div>
            <div className="mf"><h5>Multi-org</h5><p>Each institution in its own org. Users span multiple. Hard data isolation at the DB row level.</p></div>
            <div className="mf"><h5>Deployment</h5><p>Docker Compose + Terraform + Ansible. Automated Hetzner provisioning in minutes.</p></div>
            <div className="mf"><h5>Ingest connectors</h5><p>Signed agents for X-Ways, Cellebrite UFED, Magnet AXIOM, Autopsy, OpenText EnCase.</p></div>
            <div className="mf"><h5>On-prem AI</h5><p>Whisper transcription, OCR, translation, entity extraction, semantic search. Nothing leaves the box.</p></div>
          </div>
        </div>
      </section>

      {/* CTA */}
      <section className="wrap" style={{ padding: '24px 32px 64px' }}>
        <div className="cta-banner">
          <div>
            <h2>See it with <em>your</em> case file.</h2>
            <p>A 45-minute walkthrough on a spun-up instance, seeded with anonymised data that mirrors the kind of work your team does.</p>
          </div>
          <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap', justifyContent: 'flex-end' }}>
            <a className="btn ghost" href="/security">Security details</a>
            <a className="btn" href="/contact">Book a demo <span className="arr">{'\u2192'}</span></a>
          </div>
          <div className="blob" style={{ width: '500px', height: '500px', background: 'var(--accent)', right: '-100px', top: '-200px', opacity: '.25' }}></div>
        </div>
      </section>
    </>
  );
}
