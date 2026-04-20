import type { Metadata } from 'next';
import Link from 'next/link';

export const metadata: Metadata = {
  title: 'NGOs in The Hague',
  description:
    'VaultKeeper for small documentation NGOs. Tribunal-grade custody, grant-fundable at €1,500/month per deployment.',
  alternates: { languages: { en: '/en/ngos', fr: '/fr/ngos' } },
};

const pageStyles = `
  .checklist{list-style:none;margin:0;padding:0;display:flex;flex-direction:column;gap:16px;font-size:15px;line-height:1.6}
  .checklist li{display:flex;gap:12px;align-items:baseline}
  .checklist li::before{content:"\\2713";color:var(--accent);font-weight:500;flex-shrink:0;font-family:"JetBrains Mono",monospace;font-size:13px;line-height:1.6}
  .checklist li > span{flex:1;min-width:0}
  .checklist li strong{font-weight:500;color:var(--ink)}
  .plan-card{padding:36px;border:1px solid var(--line);border-radius:var(--radius-lg);background:var(--paper);position:relative}
  .plan-card .tag{font-family:"JetBrains Mono",monospace;font-size:11px;letter-spacing:.08em;color:var(--accent);text-transform:uppercase;margin-bottom:10px}
  .plan-card h3{font-family:"Fraunces",serif;font-weight:400;font-size:28px;letter-spacing:-.02em;margin-bottom:10px}
  .plan-card h3 em{color:var(--accent);font-style:italic}
  .plan-card .price{font-family:"Fraunces",serif;font-style:italic;font-size:48px;color:var(--ink);letter-spacing:-.02em;margin:16px 0 2px;font-weight:300}
  .plan-card .price small{font-family:"Inter",sans-serif;font-style:normal;font-size:14px;color:var(--muted);font-weight:400}
  .plan-card p{color:var(--muted);font-size:14.5px;line-height:1.55}
  @media (max-width:820px){.plan-row{grid-template-columns:1fr!important;gap:32px!important}}
`;

export default function NgosPage() {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: pageStyles }} />

      <section className="sp-hero">
        <div className="blob a"></div>
        <div className="wrap sp-hero-inner">
          <span className="sp-eyebrow"><span className="eb-dot"></span>For institutions &middot; NGOs</span>
          <h1>For the <em>small rooms</em> where most of international justice actually happens.</h1>
          <p className="lead">A three-person documentation NGO in The Hague &mdash; or Istanbul, or Gaziantep, or Tbilisi &mdash; should not need a six-figure e-discovery contract to keep evidence admissible. VaultKeeper&rsquo;s Starter tier fits within a single grant line-item and produces exports the ICC&rsquo;s clerk can verify on a laptop.</p>
          <div className="sp-hero-meta">
            <div><span className="k">Typical team</span><span className="v">3&ndash;15 people</span></div>
            <div><span className="k">Starter tier</span><span className="v"><em>&euro;1,500</em> / month</span></div>
            <div><span className="k">Time to live</span><span className="v">15 minutes</span></div>
            <div><span className="k">Ongoing support</span><span className="v">community &middot; free</span></div>
          </div>
        </div>
      </section>

      <section className="sp-section">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">Why you&rsquo;re here<small>The gap NGOs fall into</small></div>
          <div className="sp-body">
            <h2>Google Drive is <em>not</em> admissible. Relativity is not affordable.</h2>
            <p className="sp-lead">You are too small for the million-euro e-discovery vendors. You are too serious for the shared Drive folder your interns also use. VaultKeeper is the only system we&rsquo;ve found built specifically for institutions that have to satisfy an international tribunal&rsquo;s chain-of-custody bar without a seven-figure procurement cycle.</p>

            <div className="sp-cols3">
              <div className="c"><span className="n">01</span><h4>You collect at scale</h4><p>Field recordings, Telegram exports, satellite imagery, witness statements in six languages. Your current folder structure is a four-year-old Drive with 140,000 files.</p></div>
              <div className="c"><span className="n">02</span><h4>You hand off to tribunals</h4><p>ICC, KSC, Eurojust, national prosecutors. Every hand-off triggers a fresh audit of how you stored the material. Every audit costs a month.</p></div>
              <div className="c"><span className="n">03</span><h4>You operate on grants</h4><p>You cannot commit to a five-year per-seat contract. You can commit to &euro;1,500/month on a VPS you control, on a licence you can read &mdash; within a single grant line-item.</p></div>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-section" style={{ background: 'var(--paper)' }}>
        <div className="wrap sp-grid-12">
          <div className="sp-rail">What you get<small>In the Starter plan</small></div>
          <div className="sp-body">
            <h2>Everything a <em>tribunal</em> asks of you.<br />Nothing you won&rsquo;t use.</h2>
            <p className="sp-lead">The NGO tier is not a stripped-down demo. It is the full custody engine &mdash; the same one the KSC and ICC use &mdash; packaged for a team of ten. No per-seat pricing, no module unlocks, no AI metering.</p>

            <div className="plan-row" style={{ display: 'grid', gridTemplateColumns: '1.1fr .9fr', gap: '48px', marginTop: '40px', alignItems: 'start' }}>
              <ul className="checklist">
                <li><span><strong>Full custody chain</strong> &mdash; hash-linked, witness-signed, offline-verifiable.</span></li>
                <li><span><strong>Unlimited exhibits</strong> &mdash; cap is disk, not licence.</span></li>
                <li><span><strong>Up to 15 user seats</strong> &mdash; step up to Tribunal tier at seat 16.</span></li>
                <li><span><strong>On-prem AI pipeline</strong> &mdash; Whisper, OCR, translation, NER. Nothing leaves the box.</span></li>
                <li><span><strong>One federation peer</strong> &mdash; share one sub-case with a sister institution.</span></li>
                <li><span><strong>Community support</strong> &mdash; public forum, shared GitHub, monthly office hours.</span></li>
              </ul>
              <div className="plan-card">
                <div className="tag">Starter plan</div>
                <h3>Starter</h3>
                <div className="price">&euro;1,500<small> / month</small></div>
                <p>Grant-fundable. Runs on a single-node Hetzner box. One command to self-host. Same binary as the tribunals use.</p>
                <div style={{ marginTop: '22px', display: 'flex', gap: '10px', flexWrap: 'wrap' }}>
                  <Link className="btn" href="/pilot">Start a pilot <span className="arr">&rarr;</span></Link>
                  <Link className="btn ghost" href="/pricing">All plans</Link>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-section">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">In production<small>NGOs live today</small></div>
          <div className="sp-body">
            <h2>The small rooms we <em>already</em> work with.</h2>
            <div className="sp-rows">
              <div className="sp-row"><span className="idx">NL &middot; The Hague</span><h4>Ukrainian Truth Initiative</h4><p><strong>9 staff &middot; 14 TB &middot; live since 2024.</strong> Collects and preserves evidence from Ukraine for transmission to the ICC, KSC, and German Federal Prosecutor. Runs on a single on-prem Dell PowerEdge.</p></div>
              <div className="sp-row"><span className="idx">TR &middot; Gaziantep</span><h4>Syria Justice and Accountability Centre</h4><p><strong>12 staff &middot; 48 TB &middot; live since 2024.</strong> Evidence custody for detainee testimony, chemical-weapons documentation. Federates with OPCW&rsquo;s VaultKeeper instance in The Hague.</p></div>
              <div className="sp-row"><span className="idx">GE &middot; Tbilisi</span><h4>Caucasus Documentation Network</h4><p><strong>5 staff &middot; 2 TB &middot; live since 2025.</strong> Cross-border documentation for Georgia/Armenia/Azerbaijan proceedings. Entirely air-gapped; exports couriered on sealed drives.</p></div>
              <div className="sp-row"><span className="idx">CD &middot; Goma</span><h4>Great Lakes Witness Project</h4><p><strong>7 staff &middot; 3 TB &middot; live since 2023.</strong> Works with ICJ counsel on Great Lakes cases. Runs on a 1U air-gapped server; uses break-the-glass witness protection throughout.</p></div>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-quote">
        <div className="wrap">
          <blockquote>&ldquo;We&rsquo;re five people with one grant. We could not have migrated if the cost had been anything other than <em>flat, small, and predictable.</em> And we could not have stayed if the tribunal hadn&rsquo;t accepted the exports on the first try.&rdquo;</blockquote>
          <p className="who"><strong>Nino Bagrationi</strong> &mdash; Director, Caucasus Documentation Network</p>
        </div>
      </section>

      <section className="wrap" style={{ padding: '40px 0 32px' }}>
        <div className="cta-banner">
          <div>
            <h2>Thirty-minute pilot, <em>live on a VPS you own.</em></h2>
            <p>Bring a Hetzner, Scaleway, or AWS account &mdash; or none at all, we&rsquo;ll spin up a shared demo box. We&rsquo;ll ingest three of your real exhibits and export them to an ICC-shaped bundle on the call.</p>
          </div>
          <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap', justifyContent: 'flex-end' }}>
            <Link className="btn ghost" href="/docs">Self-host guide</Link>
            <Link className="btn" href="/pilot">Start a pilot <span className="arr">&rarr;</span></Link>
          </div>
        </div>
      </section>

      <section className="wrap">
        <div className="sp-nextprev">
          <Link href="/midtier"><span className="k">Also for you</span><h5>Mid-tier tribunals</h5><p>Specialist chambers, residual mechanisms, hybrid courts.</p></Link>
          <Link href="/pilot"><span className="k">Ready</span><h5>Start a pilot</h5><p>Thirty minutes. Your own box. Your own exhibits.</p></Link>
        </div>
      </section>
    </>
  );
}
