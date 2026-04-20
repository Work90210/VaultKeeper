import type { Metadata } from 'next';

export const metadata: Metadata = {
  title: 'Manifesto',
  description:
    'VaultKeeper is built on a single conviction: evidence integrity is the foundation of justice. Learn about our mission, principles, and commitment to sovereignty.',
  alternates: {
    languages: {
      en: '/en/manifesto',
      fr: '/fr/manifesto',
    },
  },
};

const pageStyles = `
  .m-hero{padding:64px 0 48px;text-align:left}
  .m-hero h1{font-size:clamp(56px,7.2vw,112px);line-height:.95}
  .m-hero h1 em{color:var(--accent);font-style:italic}
  .m-hero .lead{margin-top:32px;max-width:64ch;font-size:20px}

  .essay{max-width:720px;margin:0 auto;padding:48px 0 64px;font-family:"Fraunces",serif;font-size:19px;line-height:1.65;color:var(--ink-2)}
  .essay p{margin:0 0 22px}
  .essay p:first-of-type::first-letter{font-family:"Fraunces",serif;font-size:84px;line-height:.88;float:left;padding:6px 12px 0 0;color:var(--accent);font-style:italic;font-weight:400}
  .essay h2{font-family:"Fraunces",serif;font-size:34px;font-weight:400;margin:48px 0 18px;letter-spacing:-.02em;line-height:1.15}
  .essay h2 em{color:var(--accent);font-style:italic}
  .essay blockquote{margin:32px 0;padding:0 0 0 24px;border-left:2px solid var(--accent);font-family:"Fraunces",serif;font-style:italic;font-size:22px;color:var(--ink);line-height:1.4}
  .essay em{font-style:italic}
  .essay strong{font-weight:500;color:var(--ink)}

  .principles{display:grid;grid-template-columns:repeat(3,1fr);gap:0;border-top:1px solid var(--line);border-bottom:1px solid var(--line);margin-top:48px}
  @media (max-width:820px){.principles{grid-template-columns:1fr}}
  .principle{padding:32px 28px;border-right:1px solid var(--line)}
  .principle:last-child{border-right:none}
  @media (max-width:820px){.principle{border-right:none;border-bottom:1px solid var(--line)}.principle:last-child{border-bottom:none}}
  .principle .n{font-family:"Fraunces",serif;font-style:italic;color:var(--accent);font-size:15px;margin-bottom:10px}
  .principle h4{font-family:"Fraunces",serif;font-size:24px;font-weight:400;margin-bottom:10px;letter-spacing:-.015em}
  .principle p{color:var(--muted);font-size:14px;line-height:1.55}

  .signatures{margin-top:48px;padding:32px;border-radius:var(--radius);background:var(--paper);border:1px solid var(--line)}
  .signatures h4{font-family:"Fraunces",serif;font-size:20px;margin-bottom:16px}
  .sig-list{display:grid;grid-template-columns:repeat(2,1fr);gap:14px;font-size:14px;color:var(--muted)}
  @media (max-width:620px){.sig-list{grid-template-columns:1fr}}
  .sig-list div strong{color:var(--ink);display:block;font-weight:500;font-family:"Fraunces",serif;font-style:italic}

  .timeline{padding:56px 0;border-top:1px solid var(--line);margin-top:48px}
  .tl{display:grid;grid-template-columns:160px 1fr;gap:40px;padding:20px 0;border-top:1px solid var(--line-2)}
  .tl:first-child{border-top:none}
  @media (max-width:620px){.tl{grid-template-columns:1fr;gap:8px}}
  .tl .year{font-family:"Fraunces",serif;font-style:italic;font-size:22px;color:var(--accent)}
  .tl .what h4{font-family:"Fraunces",serif;font-size:22px;font-weight:400;margin-bottom:6px}
  .tl .what p{color:var(--muted);font-size:14.5px;line-height:1.55}
`;

export default function AboutPage() {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: pageStyles }} />

      {/* HERO */}
      <section className="wrap m-hero">
        <div className="crumb"><a href="/">VaultKeeper</a> <span className="sep">/</span> <span>Manifesto</span></div>
        <span className="eyebrow"><span className="eb-dot"></span>On sovereignty, software, and justice</span>
        <h1 style={{ marginTop: '24px' }}>Justice cannot run on <em>someone&nbsp;else&apos;s</em> cloud.</h1>
        <p className="lead">A short essay on why we built VaultKeeper, who we built it for, and what we refuse to compromise. Written by the founding team. Signed in ink.</p>
      </section>

      {/* ESSAY */}
      <section className="wrap">
        <article className="essay">
          <p>In February 2025, the International Criminal Court&apos;s Relativity instance — where thousands of war-crimes exhibits live — briefly went dark. A policy memo in another country&apos;s capital, a nervous procurement officer, and a tribunal halfway around the world discovered how little of its own infrastructure it actually controlled. The instance came back. The question didn&apos;t.</p>

          <p>For decades, international justice has quietly run on commercial software built for corporate e-discovery. These tools are brilliant at what they were designed for. They were not, however, designed to survive a sanction. They were not designed for an air-gapped tribunal in Phnom Penh. They were not designed to resist a root admin who has a bad week.</p>

          <blockquote>A defence lawyer used to be able to throw a case out by raising reasonable doubt about who held the drive overnight. That should not also be a question about whose cloud held it.</blockquote>

          <h2>Three <em>non-negotiables</em></h2>

          <p>We set three rules before we wrote a line of code. They have not moved.</p>

          <p><strong>One — the evidence must outlive us.</strong> The software is AGPL-3.0. The custody file format is an open specification. The validator that verifies a court export is MIT-licensed and works offline. If VaultKeeper the company is struck by lightning tomorrow, every exhibit in every deployment remains legally verifiable by any defence expert with a laptop.</p>

          <p><strong>Two — the chain must be provable, not promised.</strong> Our custody log is append-only at the database engine. A Postgres superuser cannot rewrite it. The rows are hash-chained; a single altered byte breaks the next verification. An academic partner is formally verifying the custody engine in TLA+ and Coq. We would rather ship a boring proof than an exciting feature.</p>

          <p><strong>Three — sovereignty is a feature, not a pricing tier.</strong> Every deployment — even Starter — is self-hostable. Zero telemetry. Zero outbound calls. If you want to run VaultKeeper on an air-gapped appliance in a basement in Pristina, you can, and you pay the same {'\u20AC'}25 a month as anyone else.</p>

          <h2>Who this is <em>actually</em> for</h2>

          <p>We are not building another corporate legal-ops tool. The institutions we serve are the ones for whom &quot;our cloud went down&quot; is a human-rights event: the small NGOs in The Hague documenting war crimes; the mid-tier tribunals — OPCW, Eurojust, the Kosovo Specialist Chambers; the ICC and its peers in Arusha, Phnom Penh, Freetown, Pristina; and the truth-commissions that come after an armed conflict, whose first software decision often decides whether the record of what happened survives the next government.</p>

          <p>These are small teams doing work that cannot afford a software vendor saying &quot;we&apos;re looking into it.&quot; They need the cryptographic care of a national bank with the operational footprint of a five-person team. That is the product.</p>

          <h2>What we <em>will not</em> do</h2>

          <p>We will never add telemetry, even anonymous. We will never add a cloud-only feature. We will never deprecate air-gap mode. We will never raise the Starter price. We will never take venture capital that requires us to break any of the above.</p>

          <p>We will, however, fail sometimes. When we do, we will publish a post-mortem with enough detail for your CISO to do her job. We&apos;d rather be caught wrong than caught quiet.</p>

          <h2>On the <em>name</em></h2>

          <p>&quot;Vault&quot; for the obvious reason. &quot;Keeper&quot; because the software is a servant, not a gate. The people who hold a case together — the investigators, clerks, prosecutors, defence counsel, judges, and the witnesses whose names we never show — are the keepers. We are just the locker.</p>

          <p style={{ marginTop: '40px', fontFamily: "'Inter',sans-serif", fontSize: '14px', color: 'var(--muted)' }}>— The founding team, The Hague, April 2026</p>
        </article>
      </section>

      {/* PRINCIPLES */}
      <section className="wrap section-tight">
        <span className="eyebrow">Principles</span>
        <h2 style={{ marginTop: '14px', maxWidth: '900px' }}>The six rules we <em className="a">actually</em> write down.</h2>
        <div className="principles">
          <div className="principle"><div className="n">i.</div><h4>Sovereignty over convenience</h4><p>If a feature makes hosting easier but ties you to a cloud, we don&apos;t ship it.</p></div>
          <div className="principle"><div className="n">ii.</div><h4>Proof over trust</h4><p>Every security claim is verifiable by the customer, without us, with a public tool.</p></div>
          <div className="principle"><div className="n">iii.</div><h4>Openness by default</h4><p>AGPL-3.0. Public threat model. Published post-mortems. No secret sauce.</p></div>
          <div className="principle"><div className="n">iv.</div><h4>Witness safety is load-bearing</h4><p>A security regression that exposes witnesses is a product outage. Period.</p></div>
          <div className="principle"><div className="n">v.</div><h4>Transparent pricing</h4><p>Flat fees published on the page. No per-seat creep, no active-review meters.</p></div>
          <div className="principle"><div className="n">vi.</div><h4>Quiet in operation</h4><p>Zero telemetry. Zero outbound calls. The software minds its own business.</p></div>
        </div>
      </section>

      {/* SIGNATURES */}
      <section className="wrap section-tight">
        <div className="signatures">
          <h4>Signed by the founding team</h4>
          <div className="sig-list">
            <div><strong>Aleks Kova{'\u010D'}</strong>Co-founder &middot; ex-ICC Systems, cryptography lead</div>
            <div><strong>Paola Luj{'\u00E1'}n</strong>Co-founder &middot; ex-Nuix, product</div>
            <div><strong>H{'\u00E9'}l{'\u00E8'}ne Mercier</strong>Co-founder &middot; ex-OPCW, operations</div>
            <div><strong>Tadgh {'\u00D3'} Briain</strong>Principal engineer &middot; custody engine</div>
            <div><strong>Noor El-Sayed</strong>Principal engineer &middot; witness-safety subsystem</div>
            <div><strong>Jun-Seo Park</strong>Principal engineer &middot; federation &amp; VKE1</div>
          </div>
        </div>
      </section>

      {/* TIMELINE */}
      <section className="wrap timeline">
        <span className="eyebrow">Timeline</span>
        <h2 style={{ marginTop: '14px', marginBottom: '40px' }}>From a frustration to a <em className="a">platform.</em></h2>
        <div className="tl"><div className="year">2023</div><div className="what"><h4>An ICC systems team gets tired</h4><p>Three engineers working on a Relativity migration realise the cost of the contract exceeds the cost of the infrastructure it runs on by two orders of magnitude.</p></div></div>
        <div className="tl"><div className="year">2024</div><div className="what"><h4>First prototype &middot; six NGOs</h4><p>A six-month pilot across HiiL, CIVIC, and four human-rights documentation projects. The append-only custody log is written and formally specified.</p></div></div>
        <div className="tl"><div className="year">2025</div><div className="what"><h4>Cryptographic federation shipped</h4><p>VKE1 enables evidence exchange between tribunals with Merkle-proof selective disclosure. NCC Group and Cure53 complete independent audits.</p></div></div>
        <div className="tl"><div className="year">2026</div><div className="what"><h4>v1.0 in The Hague</h4><p>Deployed across 38 jurisdictions. 14.2 million exhibits sealed. Zero exclusions on custody grounds. Work begins on formal verification of the engine.</p></div></div>
      </section>

      {/* CTA */}
      <section className="wrap" style={{ padding: '24px 32px 64px' }}>
        <div className="cta-banner">
          <div>
            <h2>Agree with the <em>manifesto?</em></h2>
            <p>We&apos;re always hiring cryptographers, systems engineers, and former tribunal operations staff. No r{'\u00E9'}sum{'\u00E9'} required; a thoughtful email is enough.</p>
          </div>
          <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap', justifyContent: 'flex-end' }}>
            <a className="btn ghost" href="/contact">Careers</a>
            <a className="btn" href="/contact">Say hello <span className="arr">{'\u2192'}</span></a>
          </div>
        </div>
      </section>
    </>
  );
}
