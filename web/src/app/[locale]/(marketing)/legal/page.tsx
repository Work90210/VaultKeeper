import type { Metadata } from 'next';
import Link from 'next/link';

export const metadata: Metadata = {
  title: 'Legal & imprint',
  description:
    'The legal shape of VaultKeeper Co\u00F6peratief U.A. Entity, officers, terms, warrant canary.',
  alternates: { languages: { en: '/en/legal', fr: '/fr/legal' } },
};

const pageStyles = `
  .legal h3{font-family:"Fraunces",serif;font-weight:400;font-size:24px;letter-spacing:-.015em;margin:40px 0 12px;color:var(--ink)}
  .legal h3 em{color:var(--accent);font-style:italic}
  .legal p{font-size:15.5px;line-height:1.75;color:var(--ink);max-width:68ch;margin-bottom:14px}
  .legal ul{margin:0 0 16px 20px;font-size:15px;line-height:1.75;color:var(--muted)}
  .legal ul li{margin-bottom:6px}
  .legal ul li strong{color:var(--ink);font-weight:500}
  .canary{margin-top:32px;padding:28px 32px;border:1px solid var(--line);border-radius:var(--radius);background:var(--paper);font-family:"JetBrains Mono",monospace;font-size:13.5px;line-height:1.75;color:var(--ink)}
  .canary .dot{display:inline-block;width:8px;height:8px;border-radius:50%;background:#6ea56f;margin-right:10px;vertical-align:middle}
  .canary strong{color:var(--accent)}
  .imprint{margin-top:40px;display:grid;grid-template-columns:1fr 1fr;gap:0;border:1px solid var(--line);border-radius:var(--radius)}
  @media (max-width:640px){.imprint{grid-template-columns:1fr}}
  .imprint > div{padding:24px 28px;border-right:1px solid var(--line);border-bottom:1px solid var(--line)}
  .imprint > div:nth-child(2n){border-right:none}
  .imprint > div:nth-last-child(-n+2){border-bottom:none}
  @media (max-width:640px){.imprint > div{border-right:none;border-bottom:1px solid var(--line)!important}.imprint > div:last-child{border-bottom:none!important}}
  .imprint .k{font-family:"JetBrains Mono",monospace;font-size:11px;color:var(--muted);letter-spacing:.06em;text-transform:uppercase;margin-bottom:8px}
  .imprint .v{font-family:"Fraunces",serif;font-size:18px;color:var(--ink);letter-spacing:-.005em;line-height:1.4}
`;

export default function LegalPage() {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: pageStyles }} />

      <section className="sp-hero">
        <div className="blob a"></div>
        <div className="wrap sp-hero-inner">
          <span className="sp-eyebrow"><span className="eb-dot"></span>Company &middot; Legal</span>
          <h1>The legal <em>shape</em> of the organisation you are trusting.</h1>
          <p className="lead">VaultKeeper Co&ouml;peratief U.A. is a Dutch cooperative &mdash; a form of legal entity older than Dutch joint-stock law itself, designed for member-owned institutions with a public mandate. We chose it deliberately. This page lists the entity, its officers, its subprocessors of record, its terms, and the current state of the warrant canary.</p>
        </div>
      </section>

      <section className="sp-section">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">The entity<small>Who you are contracting with</small></div>
          <div className="sp-body legal">
            <div className="imprint">
              <div><div className="k">Legal name</div><div className="v">VaultKeeper Co&ouml;peratief U.A.</div></div>
              <div><div className="k">Entity type</div><div className="v">Dutch cooperative, limited liability</div></div>
              <div><div className="k">Registered office</div><div className="v">Prinsengracht 54, 1015 DV Amsterdam</div></div>
              <div><div className="k">Chamber of Commerce (KvK)</div><div className="v">89214762</div></div>
              <div><div className="k">VAT number</div><div className="v">NL864891823B01</div></div>
              <div><div className="k">IBAN (operations)</div><div className="v">NL91 TRIO 0320 0044 15</div></div>
              <div><div className="k">Board &mdash; chair</div><div className="v">Dr. Ingrid Velasco</div></div>
              <div><div className="k">Board &mdash; CTO</div><div className="v">Jonas Arvidsson</div></div>
              <div><div className="k">Board &mdash; secretary</div><div className="v">Marieke van Loon</div></div>
              <div><div className="k">External counsel</div><div className="v">Houthoff Buruma, Amsterdam</div></div>
            </div>

            <h3>1 &middot; Terms of service</h3>
            <p>Our customer contracts are short &mdash; a 9-page master service agreement, plus a per-order schedule with the specifics. There is <em>no</em> separate &ldquo;enterprise agreement&rdquo; we negotiate against you: all customers receive the same MSA, with jurisdiction-specific annexes (DPA for EU, SPA for institutions under UN Charter privileges and immunities, and an air-gap annex where applicable). We publish the full MSA as a PDF on our contracts page.</p>

            <h3>2 &middot; Licensing of the software</h3>
            <p>VaultKeeper core is AGPL-3.0. A commercial exception is offered to signed customers who need protection from AGPL copyleft obligations in their internal fork (for example, tribunals that integrate national secure-communications modules they cannot publish). The exception does not permit closing the source for redistribution.</p>

            <h3>3 &middot; Trademarks</h3>
            <p>&ldquo;VaultKeeper&rdquo; and the chevron wordmark are trademarks of the cooperative. You may use the name to refer to us, describe interoperability, or cite our work. You may not imply endorsement or official partnership without a signed partner agreement.</p>

            <h3>4 &middot; Warrant canary</h3>
            <div className="canary">
              <p style={{ margin: '0 0 12px', color: 'var(--muted)', fontFamily: "'Inter',sans-serif", fontSize: '13px', fontStyle: 'italic' }}>Statement updated every calendar quarter. Absence of the statement is itself a notification.</p>
              <p style={{ margin: '0 0 8px' }}><span className="dot"></span><strong>Q1 2026 canary &mdash; as of 2026-01-14:</strong></p>
              <p style={{ margin: 0 }}>VaultKeeper Co&ouml;peratief U.A. has not received any national-security letter, secret subpoena, FISA order, gag order, or equivalent instrument in any jurisdiction. No such order is currently pending. No backdoor has been requested, installed, or integrated. Signed by the board: Velasco &middot; Arvidsson &middot; van Loon. PGP: 3F9A 12C4&hellip;</p>
            </div>

            <h3>5 &middot; Transparency report</h3>
            <p>Semi-annual. Q2 2025: two requests received (NL + DE), both for routine subscriber-billing information on paying customers; both disclosed to the customer; no evidence contents disclosed (we had none). Full archive on our transparency page.</p>

            <h3>6 &middot; Dispute resolution</h3>
            <p>Governing law: Netherlands. Forum: Rechtbank Amsterdam (District Court of Amsterdam). For institutions with UN privileges &amp; immunities, we sign a standard SPA that respects those privileges; disputes in that scope go to the institution&rsquo;s own internal tribunal with the cooperative voluntarily submitting to its jurisdiction.</p>

            <h3>7 &middot; Acquisitions &amp; control</h3>
            <p>As a cooperative, VaultKeeper cannot be acquired by tender offer. Control rests with the membership &mdash; a vote of two-thirds of members, weighted 40% staff / 30% customers / 30% founders. Any change of control requires a public 90-day comment period. This is structural, not aspirational.</p>

            <h3>8 &middot; Contact for legal matters</h3>
            <p>
              <strong>Legal:</strong> legal@vaultkeeper.coop &middot; <strong>Notices of claim:</strong> to the registered office above, by registered post.<br />
              <strong>PGP:</strong> 3F9A 12C4 8B7D 4E22 &nbsp; 7F55 C104 A9AB 6620 44D8
            </p>
          </div>
        </div>
      </section>

      <section className="wrap" style={{ padding: '40px 0 32px' }}>
        <div className="cta-banner">
          <div>
            <h2>Ask for a <em>signed MSA redline</em> before you sign.</h2>
            <p>Our counsel at Houthoff will sit with yours. We have closed these contracts in six weeks with tribunals whose procurement usually takes twelve months.</p>
          </div>
          <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap', justifyContent: 'flex-end' }}>
            <Link className="btn ghost" href="/privacy">Privacy policy</Link>
            <Link className="btn" href="/contact">Contact legal <span className="arr">&rarr;</span></Link>
          </div>
        </div>
      </section>
    </>
  );
}
