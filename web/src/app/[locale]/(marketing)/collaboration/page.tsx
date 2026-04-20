import type { Metadata } from 'next';
import Link from 'next/link';

export const metadata: Metadata = {
  title: 'Live collaboration',
  description:
    'Real-time collaboration inside an append-only ledger. Every edit, every redaction, every witness link is a CRDT operation sealed to the same cryptographic chain.',
  alternates: { languages: { en: '/en/collaboration', fr: '/fr/collaboration' } },
};

const pageStyles = `
  .stage{background:var(--paper);border:1px solid var(--line);border-radius:var(--radius-lg);padding:28px;position:relative;overflow:hidden}
  .stage .top{display:flex;justify-content:space-between;align-items:center;padding-bottom:16px;border-bottom:1px solid var(--line);margin-bottom:18px;font-family:"JetBrains Mono",monospace;font-size:11px;letter-spacing:.06em;text-transform:uppercase;color:var(--muted)}
  .stage .presence{display:flex;gap:-6px}
  .stage .av{width:28px;height:28px;border-radius:50%;border:2px solid var(--paper);display:inline-flex;align-items:center;justify-content:center;font-family:"Fraunces",serif;font-style:italic;font-size:13px;color:var(--paper);margin-left:-6px}
  .stage .av.a{background:#c87e5e}.stage .av.b{background:#4a6b3a}.stage .av.c{background:#3a4a6b}.stage .av.d{background:#6b3a4a}
  .doc{font-family:"Fraunces",serif;font-size:16.5px;line-height:1.7;color:var(--ink-2);position:relative}
  .doc p{margin:0 0 14px}
  .doc .mark{background:rgba(200,126,94,.18);padding:0 2px;border-bottom:2px solid var(--accent)}
  .doc .caret{display:inline-block;width:2px;height:18px;background:#4a6b3a;vertical-align:-3px;margin:0 1px;animation:blink 1s steps(2) infinite}
  @keyframes blink{0%,50%{opacity:1}50.01%,100%{opacity:0}}
  .doc .tag{position:absolute;background:#4a6b3a;color:var(--bg);font-family:"Inter",sans-serif;font-size:10px;padding:2px 6px;border-radius:3px 3px 3px 0;font-weight:500;letter-spacing:.02em;transform:translate(2px,-22px)}
  .doc .red{background:rgba(176,84,58,.14);text-decoration:line-through;text-decoration-color:#b0543a;color:var(--muted)}

  .splitview{display:grid;grid-template-columns:1.2fr .8fr;gap:24px;margin-top:32px;align-items:stretch}
  @media (max-width:860px){.splitview{grid-template-columns:1fr}}
  .activity{background:var(--ink);color:var(--bg);border-radius:var(--radius);padding:22px;font-family:"JetBrains Mono",monospace;font-size:11.5px;line-height:1.8}
  .activity h5{font-family:"JetBrains Mono",monospace;font-size:10.5px;letter-spacing:.08em;text-transform:uppercase;color:var(--muted-2);margin-bottom:14px;padding-bottom:10px;border-bottom:1px solid rgba(255,255,255,.14)}
  .activity .row{display:grid;grid-template-columns:48px 1fr auto;gap:10px;padding:5px 0;align-items:baseline}
  .activity .row .t{color:var(--muted-2)}
  .activity .row .who em{color:var(--accent-soft);font-family:"Fraunces",serif;font-style:italic;font-size:13px}
  .activity .row .sig{color:#6c6258;font-size:10px}

  .fed-diag{margin-top:48px;padding:48px 32px;border:1px solid var(--line);border-radius:var(--radius-lg);background:var(--paper);position:relative}
  .fed-diag h4{font-family:"Fraunces",serif;font-weight:400;font-size:20px;margin-bottom:8px;letter-spacing:-.01em}
  .fed-diag p{color:var(--muted);font-size:14px;line-height:1.55;max-width:60ch}
  .fed-row{display:grid;grid-template-columns:1fr 80px 1fr;gap:24px;margin-top:28px;align-items:center}
  @media (max-width:720px){.fed-row{grid-template-columns:1fr;gap:16px}.fed-arrow{transform:rotate(90deg);justify-self:start}}
  .fed-inst{padding:22px;border:1px solid var(--line);border-radius:var(--radius);background:var(--bg)}
  .fed-inst .k{font-family:"JetBrains Mono",monospace;font-size:10.5px;letter-spacing:.08em;color:var(--muted);text-transform:uppercase;margin-bottom:6px}
  .fed-inst strong{font-family:"Fraunces",serif;font-weight:400;font-size:19px;letter-spacing:-.01em;display:block;margin-bottom:10px}
  .fed-inst ul{list-style:none;margin:0;padding:0;font-size:13px;color:var(--muted);line-height:1.7}
  .fed-inst li::before{content:"\u00B7  ";color:var(--accent)}
  .fed-arrow{text-align:center;font-family:"JetBrains Mono",monospace;font-size:11px;color:var(--muted);letter-spacing:.06em}
  .fed-arrow .line{height:1px;background:var(--accent);margin:8px 0;position:relative}
  .fed-arrow .line::after{content:"";position:absolute;right:-2px;top:-3px;width:0;height:0;border-left:6px solid var(--accent);border-top:3px solid transparent;border-bottom:3px solid transparent}

  .crdt-diagram{margin-top:44px;background:var(--paper);border:1px solid var(--line);border-radius:var(--radius-lg);display:grid;grid-template-columns:1fr 1fr;gap:0;align-items:stretch;overflow:hidden}
  @media (max-width:560px){.crdt-diagram{grid-template-columns:1fr}.crdt-diagram .lane-a{border-right:none;border-bottom:1px solid var(--line)}}
  .crdt-diagram .lane{padding:26px;display:flex;flex-direction:column;gap:16px;position:relative;min-width:0}
  .crdt-diagram .lane-a{border-right:1px solid var(--line)}
  .crdt-diagram .who{display:grid;grid-template-columns:auto 1fr;column-gap:10px;row-gap:3px;align-items:baseline;padding-bottom:14px;border-bottom:1px solid var(--line);min-width:0}
  .crdt-diagram .who .dot{grid-row:1 / 3;align-self:center}
  .crdt-diagram .who strong{font-family:"Fraunces",serif;font-weight:400;font-size:16px;letter-spacing:-.005em;color:var(--ink);white-space:nowrap}
  .crdt-diagram .who small{font-family:"JetBrains Mono",monospace;font-size:10px;letter-spacing:.06em;color:var(--muted);text-transform:uppercase;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;grid-column:2}
  .crdt-diagram .dot{width:8px;height:8px;border-radius:50%;flex-shrink:0}
  .crdt-diagram .dot.a{background:#c87e5e}
  .crdt-diagram .dot.b{background:#4a6b3a}
  .crdt-diagram .ops{display:flex;flex-direction:column;gap:10px}
  .crdt-diagram .op{font-family:"JetBrains Mono",monospace;font-size:11.5px;padding:10px 12px;background:var(--bg);border:1px solid var(--line);border-radius:8px;display:grid;grid-template-columns:auto 1fr auto;gap:10px;align-items:baseline;color:var(--ink);min-width:0}
  .crdt-diagram .op code{color:var(--accent);font-weight:500;background:none;padding:0}
  .crdt-diagram .op .val{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;min-width:0}
  .crdt-diagram .op .sig{font-size:10px;color:#6ea56f;letter-spacing:.02em;white-space:nowrap}
  .crdt-diagram .op.a{border-left:2px solid #c87e5e}
  .crdt-diagram .op.b{border-left:2px solid #4a6b3a}

  .crdt-diagram .lane-a::after{content:"\u2192";position:absolute;right:-10px;top:calc(50% - 12px);width:20px;height:20px;display:flex;align-items:center;justify-content:center;font-family:"JetBrains Mono",monospace;font-size:13px;color:var(--accent);background:var(--paper);border:1px solid var(--line);border-radius:50%;z-index:3}
  @media (max-width:560px){.crdt-diagram .lane-a::after{display:none}}

  .crdt-diagram .sealed{grid-column:1 / -1;background:#16140f;color:#e9e2d4;padding:26px;display:grid;grid-template-columns:1fr 1fr;gap:14px 28px;align-items:start;border-top:1px solid rgba(0,0,0,.06)}
  @media (max-width:560px){.crdt-diagram .sealed{grid-template-columns:1fr}}
  .crdt-diagram .seal-head{grid-column:1 / -1;font-family:"JetBrains Mono",monospace;font-size:10.5px;letter-spacing:.08em;text-transform:uppercase;color:#9a8f7d;padding-bottom:14px;border-bottom:1px solid rgba(255,255,255,.08);display:flex;align-items:center;gap:10px}
  .crdt-diagram .seal-head em{color:var(--accent);font-style:italic;font-family:"Fraunces",serif;text-transform:none;letter-spacing:-.005em;font-size:13px}
  .crdt-diagram .seal-head .pill{margin-left:auto;padding:3px 9px;border:1px solid rgba(200,126,94,.4);border-radius:999px;color:var(--accent);font-size:9.5px;flex-shrink:0}
  .crdt-diagram .seal-blocks{display:flex;flex-direction:column;gap:6px}
  .crdt-diagram .blk{display:grid;grid-template-columns:auto 1fr;gap:14px;padding:11px 13px;background:rgba(255,255,255,.04);border:1px solid rgba(255,255,255,.06);border-radius:6px;font-family:"JetBrains Mono",monospace;font-size:11.5px;align-items:baseline}
  .crdt-diagram .blk .h{color:var(--accent);letter-spacing:.04em}
  .crdt-diagram .blk .n{color:#9a8f7d;text-align:right;white-space:nowrap}
  .crdt-diagram .blk.merge{background:rgba(74,107,58,.12);border-color:rgba(74,107,58,.3)}
  .crdt-diagram .blk.merge .n{color:#a8c9a0}
  .crdt-diagram .blk.curr{background:rgba(200,126,94,.14);border-color:rgba(200,126,94,.35)}
  .crdt-diagram .blk.curr .n{color:#e9e2d4}
  .crdt-diagram .blk.curr .n::before{content:"\u25CF  ";color:var(--accent)}
  .crdt-diagram .seal-meta{display:flex;flex-direction:column;gap:16px}
  .crdt-diagram .seal-meta h5{font-family:"Fraunces",serif;font-weight:400;font-size:15px;letter-spacing:-.005em;color:#e9e2d4;margin:0}
  .crdt-diagram .seal-meta p{font-size:12.5px;line-height:1.55;color:#9a8f7d;margin:0}
  .crdt-diagram .seal-meta .kvs{display:grid;grid-template-columns:auto 1fr;gap:8px 14px;font-family:"JetBrains Mono",monospace;font-size:10.5px;letter-spacing:.04em;padding-top:14px;border-top:1px solid rgba(255,255,255,.08)}
  .crdt-diagram .seal-meta .kvs dt{color:#6e6757;text-transform:uppercase}
  .crdt-diagram .seal-meta .kvs dd{color:#c3bba9;margin:0}
  .crdt-diagram .seal-meta .kvs dd em{color:#c87e5e;font-style:italic;font-family:"Fraunces",serif;text-transform:none;font-size:12.5px;letter-spacing:-.005em}
`;

export default function CollaborationPage() {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: pageStyles }} />

      <section className="sp-hero">
        <div className="blob a"></div>
        <div className="wrap sp-hero-inner">
          <span className="sp-eyebrow"><span className="eb-dot"></span>Platform &middot; 04 of 05</span>
          <h1>Forty analysts, one exhibit, <em>zero</em> broken seals.</h1>
          <p className="lead">Real-time collaboration inside an append-only ledger sounds like a contradiction. It isn&rsquo;t. Every edit, every redaction, every witness link is a CRDT operation that gets sealed to the same cryptographic chain as ingest &mdash; so when two analysts in two tribunals type on the same line, the history is reconstructable byte-by-byte.</p>
          <div className="sp-hero-meta">
            <div><span className="k">Concurrency</span><span className="v">CRDT &middot; Yjs</span></div>
            <div><span className="k">Every op</span><span className="v">signed &middot; sealed</span></div>
            <div><span className="k">Cross-tribunal</span><span className="v">federated <em>VKE1</em></span></div>
            <div><span className="k">Latency</span><span className="v">&lt; 120 ms p95</span></div>
          </div>
        </div>
      </section>

      <section className="sp-section">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">The surface<small>What analysts see</small></div>
          <div className="sp-body">
            <h2>Four people inside the <em>same</em> witness statement.</h2>
            <p className="sp-lead">Redactions flow from the corroboration lead. The translator updates the gloss. The legal officer marks a passage as &ldquo;admissible &mdash; direct quote.&rdquo; Every keystroke is a CRDT mutation, signed by the originating user, countersigned by a witness node, and appended to the same ledger that seals the exhibit.</p>

            <div className="splitview">
              <div className="stage">
                <div className="top">
                  <span>Statement &middot; W-0144 &middot; 19 Apr &middot; 14:21</span>
                  <span className="presence">
                    <span className="av a">W</span><span className="av b">M</span><span className="av c">A</span><span className="av d">J</span>
                  </span>
                </div>
                <div className="doc">
                  <p>The witness recalls arriving at the checkpoint near <span className="mark">Andriivka<span className="tag">Martyna &middot; geo-fuzz applied</span></span> at approximately <span className="red">17:40</span> 17:52 local time, after the second convoy had passed.</p>
                  <p>She identified the officer in command as <span className="mark">subject S-038<span className="tag">Juliane &middot; pseudonym &middot; linked to 3 exhibits</span></span>, who she recognised from prior encounters at the administrative building.</p>
                  <p>The statement was given voluntarily, in the presence of two intermediaries, one of whom is <span className="red">also a witness in this case (W-0099)</span> unrelated to other proceedings<span className="caret"></span>.</p>
                </div>
              </div>
              <div className="activity">
                <h5>Sealed activity &middot; last 12 ops</h5>
                <div className="row"><span className="t">14:21:02</span><span className="who"><em>W. Nyoka</em> &middot; linked exhibit E-0412</span><span className="sig">sig:d9&hellip;</span></div>
                <div className="row"><span className="t">14:21:11</span><span className="who"><em>Martyna</em> &middot; geo-fuzzed &ldquo;Andriivka&rdquo; (50 km)</span><span className="sig">sig:2c&hellip;</span></div>
                <div className="row"><span className="t">14:21:18</span><span className="who"><em>Amir H.</em> &middot; timestamp correction 17:40 &rarr; 17:52</span><span className="sig">sig:7f&hellip;</span></div>
                <div className="row"><span className="t">14:21:30</span><span className="who"><em>Juliane</em> &middot; pseudonymised &ldquo;Col. M.&rdquo; &rarr; S-038</span><span className="sig">sig:a1&hellip;</span></div>
                <div className="row"><span className="t">14:21:44</span><span className="who"><em>W. Nyoka</em> &middot; struck witness-identifying phrase</span><span className="sig">sig:e3&hellip;</span></div>
                <div className="row"><span className="t">14:21:58</span><span className="who"><em>Amir H.</em> &middot; corroboration score: 0.78</span><span className="sig">sig:bb&hellip;</span></div>
                <div className="row"><span className="t">14:22:06</span><span className="who"><em>Martyna</em> &middot; insert: &ldquo;unrelated to other proceedings&rdquo;</span><span className="sig">sig:4d&hellip;</span></div>
                <div className="row"><span className="t">14:22:14</span><span className="who"><em>witness-02</em> &middot; countersign &middot; block sealed</span><span className="sig">sig:90&hellip;</span></div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-section">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">Concurrency<small>How edits merge</small></div>
          <div className="sp-body">
            <h2>A CRDT <em>inside</em> an append-only chain.</h2>
            <p className="sp-lead">Every keystroke is a Yjs operation with a stable, signed identifier. The CRDT guarantees convergence; the signature guarantees attribution; the chain guarantees order. A compacted snapshot is sealed every 60 seconds &mdash; so replaying history costs a bounded number of ops, never the full lifetime.</p>

            <div className="crdt-diagram">
              <div className="lane lane-a">
                <div className="who"><span className="dot a"></span><strong>Analyst A</strong><small>Hague &middot; online</small></div>
                <div className="ops">
                  <div className="op a"><code>ins</code><span className="val">&ldquo;redacted&rdquo;</span><span className="sig">&check; Ed25519</span></div>
                  <div className="op a"><code>del</code><span className="val">412&ndash;437</span><span className="sig">&check; Ed25519</span></div>
                  <div className="op a"><code>mark</code><span className="val">admissible</span><span className="sig">&check; Ed25519</span></div>
                </div>
              </div>
              <div className="lane lane-b">
                <div className="who"><span className="dot b"></span><strong>Analyst B</strong><small>Gaziantep &middot; air-gap</small></div>
                <div className="ops">
                  <div className="op b"><code>ins</code><span className="val">gloss (ar&rarr;en)</span><span className="sig">&check; Ed25519</span></div>
                  <div className="op b"><code>mark</code><span className="val">contested</span><span className="sig">&check; Ed25519</span></div>
                </div>
              </div>
              <div className="sealed">
                <div className="seal-head">Sealed chain &middot; <em>converged</em><span className="pill">LIVE</span></div>
                <div className="seal-blocks">
                  <div className="blk"><span className="h">a7f4&hellip;</span><span className="n">3 ops &middot; A</span></div>
                  <div className="blk"><span className="h">3e11&hellip;</span><span className="n">2 ops &middot; B</span></div>
                  <div className="blk merge"><span className="h">91ab&hellip;</span><span className="n">&udarr; merge commit</span></div>
                  <div className="blk"><span className="h">c44d&hellip;</span><span className="n">snapshot</span></div>
                  <div className="blk curr"><span className="h">f208&hellip;</span><span className="n">head</span></div>
                </div>
                <div className="seal-meta">
                  <h5>Order doesn&rsquo;t matter. Order is <em>recorded</em>.</h5>
                  <p>Both analysts&rsquo; signed ops land in the same chain. CRDT rules resolve the concurrent edits; a merge commit records <em>how</em> they were resolved; a snapshot seals the window.</p>
                  <dl className="kvs">
                    <dt>Timestamp</dt><dd><em>ts-eu-west</em> &middot; RFC 3161</dd>
                    <dt>Integrity</dt><dd>SHA-256 &middot; BLAKE3</dd>
                    <dt>Cadence</dt><dd>snapshot every 60s</dd>
                  </dl>
                </div>
              </div>
            </div>

            <div className="sp-cols3" style={{ marginTop: '48px' }}>
              <div className="c"><span className="n">Attribution</span><h4>Every op <em>signed</em> at source</h4><p>The browser signs each op with the user&rsquo;s Ed25519 key before transmission. Unsigned ops are dropped at the gateway &mdash; there is no &ldquo;server-mutated&rdquo; state.</p></div>
              <div className="c"><span className="n">Convergence</span><h4>Order-free merge</h4><p>Because CRDTs are order-free, two analysts working offline (air-gapped tribunals) can reconcile when they meet again. The chain records the merge itself as a sealed event.</p></div>
              <div className="c"><span className="n">Replay</span><h4>Every <em>keystroke</em> reconstructable</h4><p>Defence counsel can request a per-character replay of any sentence in any exhibit. The clerk validator produces the exact edit sequence, with attribution and timestamps, from the sealed log.</p></div>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-section dark">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">Federation<small>Across institutions</small></div>
          <div className="sp-body">
            <h2>Two tribunals, one sealed <em>exhibit.</em></h2>
            <p className="sp-lead">Collaboration does not stop at the institutional firewall. The <strong>VKE1 federation spec</strong> lets two VaultKeeper instances share a scoped sub-chain with cryptographic provenance preserved at every step. Neither instance is &ldquo;primary&rdquo;; both hold full signing authority over the sub-chain.</p>

            <div className="fed-diag" style={{ background: 'rgba(255,255,255,.03)', borderColor: 'rgba(255,255,255,.12)' }}>
              <div className="fed-row">
                <div className="fed-inst" style={{ background: 'transparent', borderColor: 'rgba(255,255,255,.18)' }}>
                  <div className="k" style={{ color: 'var(--muted-2)' }}>Instance A &middot; Eurojust &middot; The Hague</div>
                  <strong style={{ color: 'var(--bg)' }}>Primary custodian</strong>
                  <ul style={{ color: 'var(--muted-2)' }}>
                    <li>Case ICC-UKR-2024</li>
                    <li>48,217 sealed events</li>
                    <li>Authority: custody + witness sealing</li>
                  </ul>
                </div>
                <div className="fed-arrow" style={{ color: 'var(--muted-2)' }}>VKE1<div className="line"></div>signed sub-chain</div>
                <div className="fed-inst" style={{ background: 'transparent', borderColor: 'rgba(255,255,255,.18)' }}>
                  <div className="k" style={{ color: 'var(--muted-2)' }}>Instance B &middot; CIJA &middot; Berlin</div>
                  <strong style={{ color: 'var(--bg)' }}>Co-investigator</strong>
                  <ul style={{ color: 'var(--muted-2)' }}>
                    <li>Sub-case ICC-UKR-2024/CIJA</li>
                    <li>reads all &middot; writes to sub-chain only</li>
                    <li>Authority: translation + corroboration</li>
                  </ul>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-quote">
        <div className="wrap">
          <blockquote>&ldquo;We stopped e-mailing exhibit folders to The Hague the week we went live. The cost in clerk-hours alone paid for the migration <em>three times over</em>.&rdquo;</blockquote>
          <p className="who"><strong>Laila Mensah</strong> &mdash; Head of digital operations, Residual Special Court for Sierra Leone</p>
        </div>
      </section>

      <section className="wrap" style={{ padding: '40px 0 32px' }}>
        <div className="cta-banner">
          <div>
            <h2>Pair with a <em>sister</em> tribunal.</h2>
            <p>Already share exhibits with another institution? We&rsquo;ll federate your two instances in a four-hour onboarding call, with both sides in the room.</p>
          </div>
          <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap', justifyContent: 'flex-end' }}>
            <Link className="btn ghost" href="/federation">Federation spec</Link>
            <Link className="btn" href="/contact">Book a demo <span className="arr">&rarr;</span></Link>
          </div>
        </div>
      </section>

      <section className="wrap">
        <div className="sp-nextprev">
          <Link href="/witness"><span className="k">Prev &middot; 03/05</span><h5>Witness protection</h5><p>Pseudonymisation, break-the-glass unsealing, voice masking.</p></Link>
          <Link href="/search-discovery"><span className="k">Next &middot; 05/05</span><h5>Search &amp; discovery</h5><p>Cross-exhibit semantic search &mdash; on-box models, zero telemetry.</p></Link>
        </div>
      </section>
    </>
  );
}
