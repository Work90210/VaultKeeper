import type { Metadata } from 'next';
import Link from 'next/link';

export const metadata: Metadata = {
  title: 'ICC-scale bodies',
  description:
    'VaultKeeper at sovereign scale. Multi-site, cryptographically-federated mesh for institutions whose adversaries include nation-states.',
  alternates: { languages: { en: '/en/icc', fr: '/fr/icc' } },
};

const pageStyles = `
  .bigstat{padding:48px 0 16px;border-top:1px solid var(--line);border-bottom:1px solid var(--line);margin-top:40px;display:grid;grid-template-columns:repeat(4,1fr);gap:0}
  @media (max-width:760px){.bigstat{grid-template-columns:1fr 1fr}}
  .bigstat > div{padding:0 28px 32px 0;border-right:1px solid var(--line)}
  .bigstat > div:last-child{border-right:none}
  @media (max-width:760px){.bigstat > div:nth-child(2n){border-right:none}}
  .bigstat .big{font-family:"Fraunces",serif;font-weight:300;font-size:72px;letter-spacing:-.03em;line-height:.95;color:var(--ink)}
  .bigstat .big em{color:var(--accent);font-style:italic}
  .bigstat small{display:block;font-family:"JetBrains Mono",monospace;font-size:11px;letter-spacing:.06em;color:var(--muted);text-transform:uppercase;margin-top:14px;line-height:1.5}
`;

export default function IccPage() {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: pageStyles }} />

      <section className="sp-hero">
        <div className="blob a"></div>
        <div className="wrap sp-hero-inner">
          <span className="sp-eyebrow"><span className="eb-dot"></span>For institutions &middot; ICC-scale</span>
          <h1>For courts whose <em>evidence locker</em> is a datacentre floor.</h1>
          <p className="lead">The International Criminal Court. The ICJ. The OPCW. Institutions whose adversaries include nation-states, whose procurement cycles are measured in years, and whose single outage becomes a newspaper headline. VaultKeeper at this tier is a sovereign, multi-site, cryptographically-federated mesh &mdash; not a SaaS tenant.</p>
          <div className="sp-hero-meta">
            <div><span className="k">Scale</span><span className="v">PB &middot; kEvents/s</span></div>
            <div><span className="k">Architecture</span><span className="v">multi-site &middot; active-active</span></div>
            <div><span className="k">Residency</span><span className="v">sovereign &middot; on-prem</span></div>
            <div><span className="k">SLA</span><span className="v">99.99% &middot; named</span></div>
          </div>
        </div>
      </section>

      <section className="sp-section">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">Operating envelope<small>What &ldquo;Sovereign tier&rdquo; means</small></div>
          <div className="sp-body">
            <h2>Four proof points, at <em>court-of-last-resort</em> scale.</h2>
            <p className="sp-lead">The Sovereign tier exists because one customer kept asking us whether VaultKeeper could survive a coordinated state-actor incident. These are the operating commitments we made to answer &ldquo;yes&rdquo; &mdash; and the operating figures we run to today.</p>

            <div className="bigstat">
              <div><div className="big">4.2 <em>PB</em></div><small>Largest single deployment<br />under active seal</small></div>
              <div><div className="big">12,400 <em>ev/s</em></div><small>Sustained custody-event rate<br />(sealed, signed, persisted)</small></div>
              <div><div className="big">38 <em>jur.</em></div><small>Jurisdictions with at least<br />one VaultKeeper instance</small></div>
              <div><div className="big">99.994%</div><small>Measured availability<br />trailing 12 months</small></div>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-section dark">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">The architecture<small>Not a cluster. A mesh.</small></div>
          <div className="sp-body">
            <h2>Three sites. Independent authority. <em>One</em> sealed ledger.</h2>
            <p className="sp-lead">At ICC-scale, a single-site deployment is a political liability. The Sovereign tier deploys as three independent instances in three jurisdictions, each holding full signing authority, mutually federating via VKE1. Loss of any one site &mdash; legal, physical, or cryptographic &mdash; does not stop operations.</p>

            <div className="sp-cols3">
              <div className="c"><span className="n">01 &middot; PRIMARY</span><h4>Institutional HQ</h4><p>The main operating site &mdash; typically the institution&rsquo;s own datacentre in The Hague, Geneva, or Arusha. Full write authority. HSM-backed.</p></div>
              <div className="c"><span className="n">02 &middot; SOVEREIGN</span><h4>State-sponsored peer</h4><p>A second instance in a jurisdiction the primary institution selects &mdash; a friendly state agency or allied tribunal. Independent HSM, independent key-ceremony ritual.</p></div>
              <div className="c"><span className="n">03 &middot; ARCHIVAL</span><h4>Air-gapped mirror</h4><p>A third, fully offline mirror &mdash; updated via sealed-drive courier. Purpose-built for catastrophic recovery: if the first two sites are both compromised, this one is the court of record.</p></div>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-section">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">What ships<small>Included in Sovereign</small></div>
          <div className="sp-body">
            <h2>The engagement looks nothing like a <em>SaaS contract.</em></h2>

            <div className="sp-rows">
              <div className="sp-row"><span className="idx">01</span><h4>Dedicated deployment team</h4><p><strong>Named, resident engineers for 12 months.</strong> Two VaultKeeper principals embed with your registrar and head of IT through migration, key ceremony, and the first major exhibit cycle.</p></div>
              <div className="sp-row"><span className="idx">02</span><h4>Source escrow</h4><p><strong>A copy of the source tree sealed with a Dutch notary.</strong> If VaultKeeper co&ouml;p ceases to exist, your registrar receives the escrow on 30 days&rsquo; notice. You have our source whether or not we do.</p></div>
              <div className="sp-row"><span className="idx">03</span><h4>Annual third-party audit</h4><p><strong>NCC Group or Cure53, funded by us, scope set by you.</strong> Full report goes to you unredacted; summary goes public. Latest audit is linked on our security page.</p></div>
              <div className="sp-row"><span className="idx">04</span><h4>Quarterly key ceremony</h4><p><strong>HSM key rotations at your site, witnessed by your custodians.</strong> Old keys are kept sealed in a tamper-evident bag under your custody &mdash; never ours.</p></div>
              <div className="sp-row"><span className="idx">05</span><h4>Formal verification of the custody engine</h4><p><strong>The append-only ledger module is F*-verified.</strong> We publish proofs of its critical invariants &mdash; monotonicity, signature completeness, merge associativity.</p></div>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-quote">
        <div className="wrap">
          <blockquote>&ldquo;The <em>source-escrow</em> clause was the one that got this past our legal committee. It meant our 30-year archive would outlive the vendor, the licence, and three rounds of re-tender.&rdquo;</blockquote>
          <p className="who"><strong>Office of the Registrar</strong> &mdash; Letter of reference, redacted institution, 2025</p>
        </div>
      </section>

      <section className="wrap" style={{ padding: '40px 0 32px' }}>
        <div className="cta-banner">
          <div>
            <h2>A closed <em>architecture review</em> for your head of IT.</h2>
            <p>Two hours, under NDA, with our principal engineer and our CTO. We show you our running sovereign deployment and the formal proofs behind it.</p>
          </div>
          <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap', justifyContent: 'flex-end' }}>
            <Link className="btn ghost" href="/security">Security architecture</Link>
            <Link className="btn" href="/contact">Request the review <span className="arr">&rarr;</span></Link>
          </div>
        </div>
      </section>

      <section className="wrap">
        <div className="sp-nextprev">
          <Link href="/midtier"><span className="k">Smaller</span><h5>Mid-tier tribunals</h5><p>Specialist chambers, residual mechanisms, hybrid courts.</p></Link>
          <Link href="/commissions"><span className="k">Also</span><h5>Truth commissions</h5><p>Witness-centred mandates, 30-year archives.</p></Link>
        </div>
      </section>
    </>
  );
}
