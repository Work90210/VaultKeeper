import type { Metadata } from 'next';
import Link from 'next/link';

export const metadata: Metadata = {
  title: 'Privacy',
  description:
    'Privacy policy for VaultKeeper Co\u00F6peratief U.A. We run software that protects witnesses. We had better take this seriously.',
  alternates: { languages: { en: '/en/privacy', fr: '/fr/privacy' } },
};

const pageStyles = `
  .legal h3{font-family:"Fraunces",serif;font-weight:400;font-size:24px;letter-spacing:-.015em;margin:40px 0 12px;color:var(--ink)}
  .legal h3 em{color:var(--accent);font-style:italic}
  .legal p{font-size:15.5px;line-height:1.75;color:var(--ink);max-width:68ch;margin-bottom:14px}
  .legal p em{color:var(--accent);font-family:"Fraunces",serif;font-size:16.5px}
  .legal ul{margin:0 0 16px 20px;font-size:15px;line-height:1.75;color:var(--muted)}
  .legal ul li{margin-bottom:6px}
  .legal ul li strong{color:var(--ink);font-weight:500}
  .legal .meta{font-family:"JetBrains Mono",monospace;font-size:12px;color:var(--muted);letter-spacing:.04em;padding:14px 0 26px;border-bottom:1px solid var(--line);margin-bottom:24px}
`;

export default function PrivacyPage() {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: pageStyles }} />

      <section className="sp-hero">
        <div className="blob a"></div>
        <div className="wrap sp-hero-inner">
          <span className="sp-eyebrow"><span className="eb-dot"></span>Company &middot; Privacy</span>
          <h1>We run software that <em>protects witnesses.</em> We had better take this seriously.</h1>
          <p className="lead">This policy covers the data <em>VaultKeeper Co&ouml;peratief U.A.</em> &mdash; the Dutch cooperative &mdash; collects about people who use our website, request pilots, or hold paid contracts with us. It does <strong>not</strong> cover the evidence data inside a customer&rsquo;s self-hosted VaultKeeper instance: we have no access to that, by architecture. This page is written by a human; a lawyer at Houthoff Amsterdam reviewed it on 2026-01-14.</p>
        </div>
      </section>

      <section className="sp-section">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">Plain English<small>Eight short sections</small></div>
          <div className="sp-body legal">
            <div className="meta">Effective 2026-01-14 &middot; GDPR &middot; Dutch implementation act (UAVG) &middot; version 4.2</div>

            <h3>1 &middot; What we <em>don&rsquo;t</em> see</h3>
            <p>VaultKeeper customers run the software themselves. Evidence, witness identities, case metadata, AI pipeline results, and user activity inside those instances never leave the customer&rsquo;s perimeter. We cannot read them. No backup, no analytics, no &ldquo;model training&rdquo;, no telemetry. If a hostile party sent us a subpoena for a tribunal&rsquo;s case file, our honest answer would be that we do not have it.</p>

            <h3>2 &middot; What we <em>do</em> collect, about you</h3>
            <p>Only what we need to run a small website and schedule pilot calls:</p>
            <ul>
              <li><strong>Pilot request form.</strong> Your name, institution, jurisdiction, role, team size, and free-text response. Held in Proton Mail; retained for 24 months unless you ask us to delete it sooner.</li>
              <li><strong>Paid-customer contacts.</strong> Billing address, VAT number, and up to three technical contacts. Held in our accounting system (Moneybird, EU-hosted). Retained for seven years per Dutch fiscal law.</li>
              <li><strong>Support conversations.</strong> If you open a support thread, we retain the thread for 18 months. Ours is the only system it lives in.</li>
              <li><strong>Website logs.</strong> IP, user-agent, URL, timestamp. Retained 14 days, then discarded. No cookies; no trackers; no analytics vendor.</li>
            </ul>

            <h3>3 &middot; What we <em>will not</em> do</h3>
            <p>We will not sell, rent, or share your data with marketers, brokers, analytics firms, or AI companies. We will not use contract data to train any model. We will not set advertising cookies. We will not deploy session-replay tooling. These are not forward-looking commitments &mdash; they are present-day refusals.</p>

            <h3>4 &middot; Sub-processors</h3>
            <p>A short, auditable list:</p>
            <ul>
              <li><strong>Proton Mail</strong> &mdash; mail, calendar, shared drive. Swiss jurisdiction.</li>
              <li><strong>Moneybird</strong> &mdash; bookkeeping &amp; invoicing. NL jurisdiction.</li>
              <li><strong>Hetzner</strong> &mdash; this website. DE jurisdiction. Logs retained 14 days.</li>
              <li><strong>Stripe</strong> &mdash; payment processing, <em>EU entity only</em>. IE jurisdiction.</li>
            </ul>
            <p>We change sub-processors only by a board-approved resolution, posted publicly 30 days in advance. The full resolution log is linked from our legal page.</p>

            <h3>5 &middot; Your rights under the <em>GDPR</em></h3>
            <p>You may request access, rectification, erasure, restriction of processing, data portability, or withdrawal of consent at any time. Requests go to <strong>privacy@vaultkeeper.coop</strong> and are answered, in writing, within 14 days. We have never refused a request. You can also complain to the <em>Autoriteit Persoonsgegevens</em> (Dutch DPA); we will not retaliate.</p>

            <h3>6 &middot; Transfers outside the EU</h3>
            <p>We do not transfer personal data outside the EU/EEA. If a sub-processor did operate non-EU infrastructure, we would switch. Our legal entity, staff, and infrastructure are all EU-resident.</p>

            <h3>7 &middot; How we handle government requests</h3>
            <p>We receive lawful requests. We respond to them. We also publish a semi-annual transparency report disclosing the <em>number</em> of requests received, the <em>jurisdiction</em>, and the <em>disposition</em> (complied, challenged, refused). If we are ever compelled not to publish &mdash; a gag order &mdash; we will discontinue the &ldquo;no gag received&rdquo; canary on our legal page. Its absence is itself a notification. As of publication, the canary remains.</p>

            <h3>8 &middot; Contact</h3>
            <p>
              <strong>Data controller:</strong> VaultKeeper Co&ouml;peratief U.A.<br />
              <strong>Address:</strong> Prinsengracht 54, 1015 DV Amsterdam, the Netherlands<br />
              <strong>KvK:</strong> 89214762 &nbsp;&middot;&nbsp; <strong>VAT:</strong> NL864891823B01<br />
              <strong>DPO:</strong> Marieke van Loon &middot; privacy@vaultkeeper.coop<br />
              <strong>PGP fingerprint:</strong> 3F9A 12C4 8B7D 4E22 &nbsp; 7F55 C104 A9AB 6620 44D8
            </p>
          </div>
        </div>
      </section>

      <section className="wrap" style={{ padding: '40px 0 32px' }}>
        <div className="cta-banner">
          <div>
            <h2>If any of the above is <em>untrue,</em> tell us.</h2>
            <p>We&rsquo;d rather correct a mistake in this document than let it sit. Write to <strong>privacy@vaultkeeper.coop</strong> with what you think is wrong and we&rsquo;ll answer in writing.</p>
          </div>
          <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap', justifyContent: 'flex-end' }}>
            <Link className="btn ghost" href="/legal">Legal &amp; imprint</Link>
            <Link className="btn" href="/disclosure">Responsible disclosure <span className="arr">&rarr;</span></Link>
          </div>
        </div>
      </section>
    </>
  );
}
