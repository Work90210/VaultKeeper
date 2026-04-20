import type { Metadata } from 'next';
import Link from 'next/link';

export const metadata: Metadata = {
  title: 'Mid-tier tribunals',
  description:
    'VaultKeeper for specialist chambers, residual mechanisms, hybrid courts, and national war-crimes units. Tribunal-grade custody at NGO-grade operating cost.',
  alternates: { languages: { en: '/en/midtier', fr: '/fr/midtier' } },
};

const pageStyles = `
  .case-stat{display:grid;grid-template-columns:1fr 1fr 1fr;gap:0;margin-top:36px;border:1px solid var(--line);border-radius:var(--radius)}
  @media (max-width:720px){.case-stat{grid-template-columns:1fr}}
  .case-stat > div{padding:28px;border-right:1px solid var(--line)}
  .case-stat > div:last-child{border-right:none}
  @media (max-width:720px){.case-stat > div{border-right:none;border-bottom:1px solid var(--line)}.case-stat > div:last-child{border-bottom:none}}
  .case-stat .k{font-family:"JetBrains Mono",monospace;font-size:11px;letter-spacing:.06em;color:var(--muted);text-transform:uppercase;margin-bottom:8px}
  .case-stat strong{font-family:"Fraunces",serif;font-weight:400;font-size:36px;letter-spacing:-.02em;line-height:1;display:block;margin-bottom:8px}
  .case-stat strong em{color:var(--accent);font-style:italic}
  .case-stat p{font-size:13px;color:var(--muted);line-height:1.55}
`;

export default function MidtierPage() {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: pageStyles }} />

      <section className="sp-hero">
        <div className="blob a"></div>
        <div className="wrap sp-hero-inner">
          <span className="sp-eyebrow"><span className="eb-dot"></span>For institutions &middot; Mid-tier</span>
          <h1>For the tribunals the <em>UN doesn&rsquo;t fund.</em></h1>
          <p className="lead">Specialist chambers, residual mechanisms, hybrid courts, national war-crimes units. You have the mandate of an international court and the budget of a medium-sized municipality. VaultKeeper was built with your constraints in mind &mdash; tribunal-grade custody at NGO-grade operating cost.</p>
          <div className="sp-hero-meta">
            <div><span className="k">Typical team</span><span className="v">20&ndash;150 staff</span></div>
            <div><span className="k">Professional tier</span><span className="v"><em>&euro;6,500</em> / month</span></div>
            <div><span className="k">Evidence volume</span><span className="v">1&ndash;400 TB</span></div>
            <div><span className="k">Deployment</span><span className="v">self-host &middot; on-prem</span></div>
          </div>
        </div>
      </section>

      <section className="sp-section">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">The shape<small>Tribunals live today</small></div>
          <div className="sp-body">
            <h2>Four mid-tier tribunals already running <em>on us.</em></h2>
            <p className="sp-lead">These are not pilots. Each of these institutions runs VaultKeeper as its primary evidence system, on their own hardware, under their own administrators, with active proceedings.</p>

            <div className="sp-rows">
              <div className="sp-row"><span className="idx">XK &middot; Hague</span><h4>Kosovo Specialist Chambers</h4><p><strong>Live since 2024 &middot; 94 TB sealed &middot; 480+ seats.</strong> Migrated from Relativity in 6 weeks. Runs a dual-site deployment (primary at KSC HQ, secondary on sovereign cloud in Luxembourg).</p></div>
              <div className="sp-row"><span className="idx">KH &middot; Phnom Penh</span><h4>Extraordinary Chambers in the Courts of Cambodia (residual)</h4><p><strong>Live since 2023 &middot; 12 TB archive &middot; 40 seats.</strong> Residual operations for case-file maintenance and defence-counsel access. Fully air-gapped.</p></div>
              <div className="sp-row"><span className="idx">SL &middot; Freetown</span><h4>Residual Special Court for Sierra Leone</h4><p><strong>Live since 2024 &middot; 8 TB &middot; 25 seats.</strong> Long-tail witness-protection and detainee monitoring. Federates with ICC on active RUF matters.</p></div>
              <div className="sp-row"><span className="idx">TZ &middot; Arusha</span><h4>IRMCT &mdash; UN Mechanism</h4><p><strong>Live since 2025 (rolling) &middot; 280 TB &middot; 320 seats.</strong> ICTR/ICTY residual archive migration. First two cases imported; remaining import in progress.</p></div>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-section" style={{ background: 'var(--paper)' }}>
        <div className="wrap sp-grid-12">
          <div className="sp-rail">The math<small>KSC &middot; first 12 months</small></div>
          <div className="sp-body">
            <h2>What the Kosovo Specialist Chambers <em>saved</em> in year one.</h2>
            <p className="sp-lead">Figures released by KSC IT in their 2025 annual report (redacted where confidential). The migration paid for itself before the end of the second quarter &mdash; we share the arithmetic because the next tribunal has every right to see it.</p>

            <div className="case-stat">
              <div><div className="k">Licence spend</div><strong>&ndash;&euro;<em>1.84m</em></strong><p>Legacy e-discovery vendor renewal that wasn&rsquo;t signed.</p></div>
              <div><div className="k">Total VK cost, y1</div><strong>&euro;<em>72k</em></strong><p>Infra (primary + secondary), staff time for migration, AGPL support tier.</p></div>
              <div><div className="k">Clerk audits</div><strong>&ndash;<em>11</em></strong><p>Audits that did not need to happen, because the sealed exports self-verified.</p></div>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-section">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">What scales<small>Without the per-seat fee</small></div>
          <div className="sp-body">
            <h2>Everything scales. <em>The bill doesn&rsquo;t.</em></h2>
            <p className="sp-lead">Mid-tier tribunals typically run VaultKeeper at two sites with active/passive failover, HSMs for key material, and between 5 and 15 witness-node peers. The reference architecture is the same one we run internally.</p>

            <div className="sp-cols3">
              <div className="c"><span className="n">01</span><h4>Per-seat pricing <em>is a lie</em></h4><p>You don&rsquo;t pay per seat. The &euro;6,500/month Professional tier includes unlimited seats, unlimited exhibits, and the on-prem AI pipeline. If your headcount doubles, your bill doesn&rsquo;t.</p></div>
              <div className="c"><span className="n">02</span><h4>Air-gap included</h4><p>Full offline operation &mdash; no licence server phone-home, no activation. Delta imports via sealed USB are a first-class operation in the custody chain.</p></div>
              <div className="c"><span className="n">03</span><h4>Procurement-friendly</h4><p>AGPL-3.0 licence, Dutch co&ouml;p legal entity, public pricing, no NDA for source code. Your legal officer has approved VaultKeeper before lunch.</p></div>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-quote">
        <div className="wrap">
          <blockquote>&ldquo;The migration ran six weeks. The last two weeks were us <em>finding reasons to slow down</em> &mdash; everything worked and we kept expecting it not to.&rdquo;</blockquote>
          <p className="who"><strong>Dragan Tufek&#269;i&#263;</strong> &mdash; Head of IT, Kosovo Specialist Chambers</p>
        </div>
      </section>

      <section className="wrap" style={{ padding: '40px 0 32px' }}>
        <div className="cta-banner">
          <div>
            <h2>Meet our tribunal <em>customer-success</em> lead.</h2>
            <p>Dr. Ingrid Velasco spent seven years inside the KSC before joining VaultKeeper. She&rsquo;ll walk your registrar and head of IT through migration planning in 45 minutes.</p>
          </div>
          <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap', justifyContent: 'flex-end' }}>
            <Link className="btn ghost" href="/pricing">Plans &amp; pricing</Link>
            <Link className="btn" href="/contact">Book the session <span className="arr">&rarr;</span></Link>
          </div>
        </div>
      </section>

      <section className="wrap">
        <div className="sp-nextprev">
          <Link href="/icc"><span className="k">Larger</span><h5>ICC-scale bodies</h5><p>Multi-jurisdiction, petabyte-scale, sovereign residency.</p></Link>
          <Link href="/ngos"><span className="k">Smaller</span><h5>NGOs in The Hague</h5><p>The Starter tier, for three-person documentation teams.</p></Link>
        </div>
      </section>
    </>
  );
}
