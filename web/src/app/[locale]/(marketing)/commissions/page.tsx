import type { Metadata } from 'next';
import Link from 'next/link';

export const metadata: Metadata = {
  title: 'Truth commissions',
  description:
    'Evidence management for truth commissions. Radical durability — a 30-year cryptographic commitment to a survivor\'s testimony.',
  alternates: { languages: { en: '/en/commissions', fr: '/fr/commissions' } },
};

const pageStyles = `
  .timeline{margin-top:40px;position:relative;padding-left:24px;border-left:1px solid var(--line)}
  .tl{padding:20px 0 20px 20px;position:relative;border-bottom:1px solid var(--line)}
  .tl:last-child{border-bottom:none}
  .tl::before{content:"";position:absolute;left:-29px;top:26px;width:10px;height:10px;border-radius:50%;background:var(--accent);border:2px solid var(--bg)}
  .tl .y{font-family:"Fraunces",serif;font-style:italic;color:var(--accent);font-size:22px;letter-spacing:-.01em}
  .tl h4{font-family:"Fraunces",serif;font-weight:400;font-size:20px;letter-spacing:-.015em;margin:4px 0 8px}
  .tl p{color:var(--muted);font-size:14.5px;line-height:1.55;max-width:60ch}
`;

export default function CommissionsPage() {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: pageStyles }} />

      <section className="sp-hero">
        <div className="blob a"></div>
        <div className="wrap sp-hero-inner">
          <span className="sp-eyebrow"><span className="eb-dot"></span>For institutions &middot; Commissions</span>
          <h1>A witness who <em>speaks</em> today must be heard in <em>2056.</em></h1>
          <p className="lead">Truth commissions are not trials. You are building a record that will outlive your commissioners, your government, and quite possibly your constitution. Evidence management at this tier is less about admissibility and more about <em>radical durability</em> &mdash; a 30-year cryptographic commitment to a survivor&rsquo;s testimony.</p>
          <div className="sp-hero-meta">
            <div><span className="k">Typical mandate</span><span className="v">2&ndash;8 years</span></div>
            <div><span className="k">Archive horizon</span><span className="v"><em>30&ndash;50 years</em></span></div>
            <div><span className="k">Testimonies</span><span className="v">10k &ndash; 200k</span></div>
            <div><span className="k">Survivor-first</span><span className="v">by design</span></div>
          </div>
        </div>
      </section>

      <section className="sp-section">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">The brief<small>What a commission needs</small></div>
          <div className="sp-body">
            <h2>The system must work after <em>we are all dead.</em></h2>
            <p className="sp-lead">Commissions have a unique procurement profile: they need an intake capable of scale, a witness-protection regime that survives administration changes, an archive that remains cryptographically verifiable long after the commission has been dissolved, and a licence that permits their successor institution to continue operations.</p>

            <div className="sp-cols3">
              <div className="c"><span className="n">01</span><h4>Survivor-centred intake</h4><p>Low-bandwidth field apps, trauma-informed interview flows, voice-first recording, per-session consent trails. Ingest in places with 2G and no stable electricity.</p></div>
              <div className="c"><span className="n">02</span><h4>30-year cryptographic seal</h4><p>The custody engine uses BLAKE3 alongside SHA-256 &mdash; two families of hash. Timestamp tokens refreshable on a 5-year cycle without invalidating historical seals.</p></div>
              <div className="c"><span className="n">03</span><h4>Post-mandate handoff</h4><p>When your commission dissolves, the archive hands off to a successor &mdash; a national archive, a university, the ICC. VaultKeeper&rsquo;s export format is readable without VaultKeeper running.</p></div>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-section" style={{ background: 'var(--paper)' }}>
        <div className="wrap sp-grid-12">
          <div className="sp-rail">Who we work with<small>Commissions live today</small></div>
          <div className="sp-body">
            <h2>The commissions <em>already</em> running on VaultKeeper.</h2>
            <div className="sp-rows">
              <div className="sp-row"><span className="idx">CO &middot; Bogot&aacute;</span><h4>Comisi&oacute;n para el Esclarecimiento de la Verdad (residual unit)</h4><p><strong>Live since 2023 &middot; 140k testimonies migrated from legacy systems.</strong> The commission concluded its mandate in 2022; VaultKeeper hosts the archival tier and serves validated exports to the Special Jurisdiction for Peace.</p></div>
              <div className="sp-row"><span className="idx">GM &middot; Banjul</span><h4>TRRC archive (The Gambia)</h4><p><strong>Live since 2024 &middot; 1,780 testimony hours &middot; air-gapped.</strong> Archival custody of the Truth, Reconciliation and Reparations Commission&rsquo;s full video record. Successor body: Office of the Attorney General.</p></div>
              <div className="sp-row"><span className="idx">TN &middot; Tunis</span><h4>Instance V&eacute;rit&eacute; et Dignit&eacute; (successor archive)</h4><p><strong>Live since 2025 &middot; 62,000 files &middot; 12 TB.</strong> Custody of IVD materials transferred to the National Archives after the commission&rsquo;s dissolution. Public-interest search index under construction.</p></div>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-section">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">The arc<small>A testimony&rsquo;s 30-year life</small></div>
          <div className="sp-body">
            <h2>What happens to a <em>single</em> testimony, over three decades.</h2>
            <p className="sp-lead">A hypothetical walk-through, based on the operating choices the Colombian commission actually made. The testimony is recorded in 2024, handed off to a successor institution in 2027, refreshed on 5-year cycles, and remains cryptographically verifiable through 2054.</p>

            <div className="timeline">
              <div className="tl"><span className="y">2024</span><h4>Recorded in a village, ingested offline</h4><p>Field team records on a hardened tablet. Testimony is sealed into the local VaultKeeper instance that evening; hashes uploaded to HQ over a sat phone the following morning.</p></div>
              <div className="tl"><span className="y">2025</span><h4>Witness protection applied</h4><p>Commission legal team pseudonymises the real name. Break-the-glass access requires quorum of commissioners; unsealing triggers a sealed event visible to the witness themselves via a signed letter.</p></div>
              <div className="tl"><span className="y">2027</span><h4>Mandate ends &middot; archive handed off</h4><p>Commission dissolves. Archive migrates (cryptographically, not by re-ingestion) to the National Archive. The custody chain continues &mdash; one more sealed row records the institutional handover.</p></div>
              <div className="tl"><span className="y">2032</span><h4>First 5-year timestamp refresh</h4><p>RFC 3161 tokens refreshed against current TSA pool. Old tokens retained; new tokens chained. A witness hash that held in 2024 still holds, <em>and</em> gains a fresh 2032 anchor.</p></div>
              <div className="tl"><span className="y">2046</span><h4>The witness&rsquo;s grandchild requests the record</h4><p>Forty years after speaking, the family of a deceased witness requests access under the successor institution&rsquo;s policy. Sealed access event recorded. The testimony plays. It is still the same one.</p></div>
              <div className="tl"><span className="y">2054</span><h4>Post-quantum migration</h4><p>Hash family upgraded from SHA-256 to SHA-3 + Kyber-bound timestamps. Every historical row is re-sealed under the new family without invalidating the old chain.</p></div>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-quote">
        <div className="wrap">
          <blockquote>&ldquo;We measure the value of the archive in <em>generations.</em> VaultKeeper was the first system we looked at whose designers also measured it that way.&rdquo;</blockquote>
          <p className="who"><strong>Archivo Nacional de Colombia</strong> &mdash; Letter of reference, 2025</p>
        </div>
      </section>

      <section className="wrap" style={{ padding: '40px 0 32px' }}>
        <div className="cta-banner">
          <div>
            <h2>A <em>commission-specific</em> consultation.</h2>
            <p>We&rsquo;ll walk your registrar, head of intake, and legal counsel through commission-specific configurations: trauma-informed recording, post-mandate handoff, cryptographic refresh cycles.</p>
          </div>
          <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap', justifyContent: 'flex-end' }}>
            <Link className="btn ghost" href="/pilot">Start a pilot</Link>
            <Link className="btn" href="/contact">Book a consultation <span className="arr">&rarr;</span></Link>
          </div>
        </div>
      </section>

      <section className="wrap">
        <div className="sp-nextprev">
          <Link href="/ngos"><span className="k">Also</span><h5>NGOs in The Hague</h5><p>Small documentation teams, Starter tier.</p></Link>
          <Link href="/icc"><span className="k">Larger</span><h5>ICC-scale bodies</h5><p>Multi-site, sovereign residency, PB scale.</p></Link>
        </div>
      </section>
    </>
  );
}
