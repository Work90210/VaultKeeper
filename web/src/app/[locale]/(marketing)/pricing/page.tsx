import type { Metadata } from 'next';

export const metadata: Metadata = {
  title: 'Pricing',
  description:
    'VaultKeeper pricing plans for human rights documentation teams and legal case builders. Start with a free pilot and scale to sovereign evidence management.',
  alternates: {
    languages: {
      en: '/en/pricing',
      fr: '/fr/pricing',
    },
  },
};

const pageStyles = `
  .pr-hero{padding:48px 0 24px}
  .pr-hero h1{font-size:clamp(44px,5.4vw,84px)}
  .pr-hero h1 em{color:var(--accent);font-style:italic}

  .tiers{display:grid;grid-template-columns:repeat(3,1fr);gap:20px;margin-top:48px}
  @media (max-width:980px){.tiers{grid-template-columns:1fr}}
  .tier{padding:36px;border-radius:var(--radius-lg);background:var(--paper);border:1px solid var(--line);display:flex;flex-direction:column;gap:18px;position:relative;transition:all .3s}
  .tier:hover{transform:translateY(-4px);box-shadow:var(--shadow-lg)}
  .tier.feat{background:var(--ink);color:var(--bg)}
  .tier.feat .muted{color:var(--muted-2)}
  .tier.feat .price em, .tier.feat h3 em{color:var(--accent-soft)}
  .tier.feat .btn{background:var(--bg);color:var(--ink);border-color:var(--bg)}
  .tier.feat .btn:hover{background:var(--accent);color:#fff;border-color:var(--accent)}
  .tier.feat ul li::before{color:var(--accent-soft)}
  .tier.feat .tag-recommended{position:absolute;top:18px;right:18px;font-size:11px;padding:4px 10px;border-radius:999px;background:var(--accent);color:#fff;letter-spacing:.02em}

  .tier h3{font-family:"Fraunces",serif;font-size:26px;font-weight:400;letter-spacing:-.015em}
  .tier .audience{font-size:13px;color:var(--muted)}
  .tier.feat .audience{color:var(--muted-2)}
  .tier .price{font-family:"Fraunces",serif;font-size:clamp(44px,5vw,64px);line-height:1;letter-spacing:-.035em;font-weight:400}
  .tier .price em{font-style:italic;color:var(--accent)}
  .tier .price small{font-size:13px;font-family:"Inter",sans-serif;color:var(--muted);margin-left:8px;letter-spacing:0;font-weight:400}
  .tier.feat .price small{color:var(--muted-2)}
  .tier .dashrule{height:1px;background:var(--line)}
  .tier.feat .dashrule{background:rgba(255,255,255,.1)}
  .tier ul{list-style:none;padding:0;margin:0;display:flex;flex-direction:column;gap:10px;flex:1}
  .tier ul li{display:grid;grid-template-columns:16px 1fr;gap:12px;font-size:14.5px;line-height:1.45;align-items:start}
  .tier ul li::before{content:"\\2713";color:var(--accent);font-weight:500;margin-top:1px}
  .tier .muted{color:var(--muted);font-size:14px;line-height:1.55}

  .compare-table{border-radius:var(--radius-lg);border:1px solid var(--line);background:var(--paper);overflow:hidden}
  .ct-row{display:grid;grid-template-columns:1.4fr 1fr 1fr 1fr;padding:16px 28px;border-top:1px solid var(--line);align-items:center;font-size:14.5px}
  .ct-row:first-child{border-top:none;background:var(--bg-2);font-family:"Fraunces",serif;font-size:15px}
  .ct-row .hh{text-align:center;font-size:12px;color:var(--muted);text-transform:uppercase;letter-spacing:.04em}
  .ct-row .feat-name{font-weight:500}
  .ct-row .v{text-align:center;color:var(--muted)}
  .ct-row .v.y{color:var(--ok)}
  .ct-row .v.y::before{content:"\\2713 "}
  .ct-row .v.n::before{content:"\\2014 "}
  @media (max-width:760px){.compare-table{overflow-x:auto}.ct-row{min-width:580px}}

  .faq details{padding:22px 0;border-top:1px solid var(--line)}
  .faq details summary{list-style:none;cursor:pointer;display:grid;grid-template-columns:1fr 22px;gap:20px;font-family:"Fraunces",serif;font-size:22px;letter-spacing:-.01em}
  .faq details summary::-webkit-details-marker{display:none}
  .faq details summary .ico{width:22px;height:22px;border-radius:50%;border:1px solid var(--line-2);display:grid;place-items:center;position:relative;transition:transform .3s}
  .faq details[open] summary .ico{background:var(--accent);border-color:var(--accent);color:#fff}
  .faq details summary .ico::before{content:"+";font-family:"Inter";font-size:14px}
  .faq details[open] summary .ico::before{content:"\\2013"}
  .faq details .ans{padding:18px 0 0;color:var(--muted);font-size:15.5px;line-height:1.6;max-width:68ch}

  .addons{display:grid;grid-template-columns:repeat(3,1fr);gap:20px}
  @media (max-width:760px){.addons{grid-template-columns:1fr}}
  .addon{padding:24px;border-radius:var(--radius);background:var(--paper);border:1px solid var(--line)}
  .addon h4{font-family:"Fraunces",serif;font-size:20px;margin-bottom:6px}
  .addon p{font-size:14px;color:var(--muted);line-height:1.5}
  .addon .p{margin-top:14px;font-family:"Fraunces",serif;font-size:20px;color:var(--accent);font-style:italic}
`;

export default function PricingPage() {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: pageStyles }} />

      {/* HERO */}
      <section className="wrap pr-hero">
        <div className="crumb"><a href="/">VaultKeeper</a> <span className="sep">/</span> <span>Pricing</span></div>
        <span className="eyebrow"><span className="eb-dot"></span>Pricing</span>
        <h1 style={{ marginTop: '20px' }}>Institutional pricing that doesn&apos;t <em>scale with panic.</em></h1>
        <p className="lead" style={{ marginTop: '24px', maxWidth: '68ch' }}>One flat monthly fee per deployment. No per-case surcharge, no per-user creep, no &quot;active review&quot; meters that turn a visible piece of evidence into a billing line. If you outgrow a tier, we move you up — not a lawyer-negotiated upgrade.</p>

        <div className="tiers">
          {/* STARTER */}
          <div className="tier">
            <h3>Starter</h3>
            <div className="audience">Small NGOs &middot; truth commissions &middot; field teams &middot; 5{'\u2013'}40 staff</div>
            <div className="price">{'\u20AC'}1,500<small>/month, per deployment</small></div>
            <div className="muted">Grant-fundable. Fits within a single OSF, Sigrid Rausing, or EU cohesion grant. Serious institutional infrastructure — not a free tier in disguise.</div>
            <div className="dashrule"></div>
            <ul>
              <li>Single-org, up to 40 active users</li>
              <li>Unlimited cases and exhibits</li>
              <li>Full custody chain + RFC 3161 timestamping</li>
              <li>Meilisearch, redaction editor, disclosure packages</li>
              <li>Nightly encrypted backups, 30-day retention</li>
              <li>Community Slack &middot; weekday-hours email support</li>
              <li>Hetzner-managed hosting included (EU-resident)</li>
            </ul>
            <div className="dashrule"></div>
            <a className="btn ghost" href="/contact">Start a free pilot <span className="arr">{'\u2192'}</span></a>
          </div>

          {/* PROFESSIONAL */}
          <div className="tier feat">
            <span className="tag-recommended">Most institutions</span>
            <h3>Professional</h3>
            <div className="audience muted">Bellingcat &middot; OPCW &middot; Eurojust &middot; KSC &middot; Forensic Architecture &middot; 40{'\u2013'}300 staff</div>
            <div className="price">{'\u20AC'}6,500<small>/month, per deployment</small></div>
            <div className="muted">Multi-org, Yjs collaboration, forensic connectors, VKE1 federation, named engineer, 24/5 SLA. Under the {'\u20AC'}100k EU procurement threshold.</div>
            <div className="dashrule"></div>
            <ul>
              <li>Multi-org deployment, unlimited seats</li>
              <li>On-prem or hybrid deployment available</li>
              <li>Forensic connectors (X-Ways, Cellebrite, Magnet, Autopsy)</li>
              <li>Keycloak SSO / SAML, custom role templates</li>
              <li>Cryptographic federation (VKE1) for cross-institution exchange</li>
              <li>24 / 5 SLA &middot; 4-hour response &middot; named engineer</li>
              <li>Quarterly threat-model review with your CISO</li>
            </ul>
            <div className="dashrule"></div>
            <a className="btn" href="/contact">Book institutional demo <span className="arr">{'\u2192'}</span></a>
          </div>

          {/* INSTITUTION */}
          <div className="tier">
            <h3>Institution</h3>
            <div className="audience">The ICC-tier &middot; national prosecutor services &middot; ministries &middot; 300+ staff</div>
            <div className="price">from {'\u20AC'}22,000<small>/month, per deployment</small></div>
            <div className="muted">Sealed appliance, customer-held HSM keys, classified builds, source escrow, 24/7 with 1-hour response. Bespoke legal-opinion work quoted separately.</div>
            <div className="dashrule"></div>
            <ul>
              <li>Sealed 1U appliance or air-gapped cluster</li>
              <li>Customer-held root keys, HSM ceremony on delivery</li>
              <li>Classified-up-to-SECRET build option</li>
              <li>Source escrow with neutral trustee</li>
              <li>Formal verification artefacts of the custody engine</li>
              <li>24 / 7 SLA &middot; 1-hour response &middot; named officer</li>
              <li>Bespoke compliance &amp; legal-opinion support per jurisdiction</li>
            </ul>
            <div className="dashrule"></div>
            <a className="btn ghost" href="/contact">Contact sovereign desk <span className="arr">{'\u2192'}</span></a>
          </div>
        </div>

        <p className="small" style={{ textAlign: 'center', marginTop: '20px' }}>All prices are flat deployment fees. Storage passes through at Hetzner cost. Migration and onboarding are included for every tier.</p>
      </section>

      {/* COMPARE */}
      <section className="section wrap">
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '40px', alignItems: 'end', marginBottom: '36px' }}>
          <div>
            <span className="eyebrow">Compare tiers</span>
            <h2 style={{ marginTop: '14px' }}>What&apos;s in every <em className="a">box.</em></h2>
          </div>
          <p className="lead">Every tier gets the full custody engine. What changes across tiers is how much infrastructure, response, and ceremony sits around it.</p>
        </div>

        <div className="compare-table">
          <div className="ct-row"><div></div><div className="hh">Starter</div><div className="hh">Professional</div><div className="hh">Institution</div></div>
          <div className="ct-row"><div className="feat-name">Append-only custody chain</div><div className="v y"></div><div className="v y"></div><div className="v y"></div></div>
          <div className="ct-row"><div className="feat-name">RFC 3161 timestamping</div><div className="v y"></div><div className="v y"></div><div className="v y"></div></div>
          <div className="ct-row"><div className="feat-name">Public chain anchoring (OpenTimestamps)</div><div className="v">Optional</div><div className="v y"></div><div className="v y"></div></div>
          <div className="ct-row"><div className="feat-name">Witness PII encryption</div><div className="v y"></div><div className="v y"></div><div className="v y"></div></div>
          <div className="ct-row"><div className="feat-name">Live collaboration (Yjs)</div><div className="v y"></div><div className="v y"></div><div className="v y"></div></div>
          <div className="ct-row"><div className="feat-name">Cryptographic federation (VKE1)</div><div className="v n"></div><div className="v y"></div><div className="v y"></div></div>
          <div className="ct-row"><div className="feat-name">Air-gapped deployment</div><div className="v n"></div><div className="v">Available</div><div className="v y"></div></div>
          <div className="ct-row"><div className="feat-name">Customer-held root keys (HSM)</div><div className="v n"></div><div className="v">Optional</div><div className="v y"></div></div>
          <div className="ct-row"><div className="feat-name">Source escrow</div><div className="v n"></div><div className="v n"></div><div className="v y"></div></div>
          <div className="ct-row"><div className="feat-name">Named response officer</div><div className="v n"></div><div className="v y"></div><div className="v y"></div></div>
          <div className="ct-row"><div className="feat-name">SLA</div><div className="v">Weekday email</div><div className="v">24 / 5 &middot; 4 hr</div><div className="v">24 / 7 &middot; 1 hr</div></div>
        </div>
      </section>

      {/* ADD-ONS */}
      <section className="section wrap" style={{ paddingTop: 0 }}>
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '40px', alignItems: 'end', marginBottom: '30px' }}>
          <div>
            <span className="eyebrow">Add-ons</span>
            <h2 style={{ marginTop: '14px' }}>Bolt on only <em className="a">what you need.</em></h2>
          </div>
          <p className="lead">Everything below is optional and priced in the open. You&apos;re never shopping through a sales team.</p>
        </div>

        <div className="addons">
          <div className="addon"><h4>On-prem AI appliance</h4><p>1U GPU box running Whisper, OCR, entity extraction, and Llama-class models — entirely local. No data ever leaves.</p><div className="p">from {'\u20AC'}14,000 / once</div></div>
          <div className="addon"><h4>Migration from RelativityOne</h4><p>ICC-scale custody rebase with hash verification. 6{'\u2013'}10 week engagement, fixed scope, turn-key handoff.</p><div className="p">from {'\u20AC'}45,000</div></div>
          <div className="addon"><h4>Formal verification report</h4><p>TLA+ / Coq artefacts for your legal opinion memo, witnessed re-run on your own infrastructure. Deeply specialist labour.</p><div className="p">from {'\u20AC'}28,000</div></div>
          <div className="addon"><h4>Extra Hetzner capacity</h4><p>Pass-through. You pay what we pay. Transparent invoice line for each additional storage box or compute node.</p><div className="p">at cost</div></div>
          <div className="addon"><h4>Custom connector build</h4><p>Signed ingest agent for a tool we don&apos;t yet support (Nuix, Veritone, bespoke agency pipelines).</p><div className="p">from {'\u20AC'}18,000</div></div>
          <div className="addon"><h4>Training &amp; certification</h4><p>Two-day clerk training in The Hague or on-site. Court-submittable certification of operator proficiency.</p><div className="p">{'\u20AC'}4,200</div></div>
        </div>
      </section>

      {/* FAQ */}
      <section className="section wrap" style={{ paddingTop: 0 }}>
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '40px', alignItems: 'end', marginBottom: '20px' }}>
          <div>
            <span className="eyebrow">FAQ</span>
            <h2 style={{ marginTop: '14px' }}>The <em className="a">institutional</em> questions.</h2>
          </div>
          <p className="lead">If what you need isn&apos;t here, our team answers in writing with a named engineer on every response.</p>
        </div>
        <div className="faq">
          <details open>
            <summary><span>Why is it so much cheaper than RelativityOne?</span><span className="ico"></span></summary>
            <div className="ans">Because there is no sales organisation, no Azure margin, and no professional-services extraction layer. Our stack is self-hostable open source. You pay us for the work of keeping it excellent — for an ICC-scale deployment that&apos;s from {'\u20AC'}264k / year, vs. ~{'\u20AC'}1.2M / year for a Relativity contract of comparable scope. And you keep full sovereignty.</div>
          </details>
          <details>
            <summary><span>What happens to our evidence if VaultKeeper, the company, disappears?</span><span className="ico"></span></summary>
            <div className="ans">Nothing legal. Your deployment keeps running — it&apos;s on your infrastructure. The source is AGPL-3.0 and mirrored to a Swiss neutral code repository. Institution-tier customers get a formal escrow arrangement with a trustee. An independent clerk validator tool is MIT-licensed so court export verification survives us entirely.</div>
          </details>
          <details>
            <summary><span>Can we run VaultKeeper completely offline, without even pulling updates?</span><span className="ico"></span></summary>
            <div className="ans">Yes. Institution tier ships as a signed appliance image. Updates arrive as signed delta packages you can verify and apply on your own schedule. No telemetry, no phone-home, no runtime checks against our servers.</div>
          </details>
          <details>
            <summary><span>What&apos;s the migration path from Relativity or Nuix?</span><span className="ico"></span></summary>
            <div className="ans">We&apos;ve done it five times. A typical tribunal moves in six weeks: week 1 provisioning and SSO; weeks 2{'\u2013'}3 parallel ingest with hash verification; week 4 custody-chain rebase; weeks 5{'\u2013'}6 historical case import and clerk training. Fixed price via the migration add-on; we don&apos;t charge by the hour.</div>
          </details>
          <details>
            <summary><span>Do you need a DPA, and under what jurisdiction?</span><span className="ico"></span></summary>
            <div className="ans">Starter deployments on our managed Hetzner instances come with a GDPR Art. 28 DPA under Dutch law. Professional and Institution deployments are typically self-hosted and therefore do not require a DPA with us — we are not a processor of your data. Sovereign deployments can include bespoke DPAs under your jurisdiction&apos;s law.</div>
          </details>
          <details>
            <summary><span>Who is actually behind VaultKeeper?</span><span className="ico"></span></summary>
            <div className="ans">A co{'\u00F6'}peratie registered in The Hague. Team of eleven (April 2026) — three former ICC systems engineers, two cryptographers, a former Nuix lead, and five engineers from the open-source digital-forensics community. Funded by a mix of NGO grants and institutional customer revenue. No venture capital.</div>
          </details>
        </div>
      </section>

      {/* CTA */}
      <section className="wrap" style={{ padding: '24px 32px 64px' }}>
        <div className="cta-banner">
          <div>
            <h2>Not sure which tier <em>fits?</em></h2>
            <p>Send us the shape of your team and your caseload. We&apos;ll reply with a sized pilot and a number — within a working day, in writing.</p>
          </div>
          <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap', justifyContent: 'flex-end' }}>
            <a className="btn ghost" href="/docs">Self-host guide</a>
            <a className="btn" href="/contact">Tell us about your team <span className="arr">{'\u2192'}</span></a>
          </div>
        </div>
      </section>
    </>
  );
}
