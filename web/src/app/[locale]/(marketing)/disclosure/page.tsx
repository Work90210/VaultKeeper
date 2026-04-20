import type { Metadata } from 'next';
import Link from 'next/link';

export const metadata: Metadata = {
  title: 'Responsible disclosure',
  description:
    'If you find a flaw in VaultKeeper, we want it. Bug bounty, safe harbour, and hall of thanks.',
  alternates: { languages: { en: '/en/disclosure', fr: '/fr/disclosure' } },
};

const pageStyles = `
  .legal h3{font-family:"Fraunces",serif;font-weight:400;font-size:24px;letter-spacing:-.015em;margin:40px 0 12px;color:var(--ink)}
  .legal h3 em{color:var(--accent);font-style:italic}
  .legal p{font-size:15.5px;line-height:1.75;color:var(--ink);max-width:68ch;margin-bottom:14px}
  .bounty{margin-top:40px;display:grid;grid-template-columns:repeat(4,1fr);gap:0;border:1px solid var(--line);border-radius:var(--radius);overflow:hidden}
  @media (max-width:760px){.bounty{grid-template-columns:1fr 1fr}}
  .bounty > div{padding:26px;border-right:1px solid var(--line)}
  .bounty > div:last-child{border-right:none}
  @media (max-width:760px){.bounty > div:nth-child(2n){border-right:none}.bounty > div{border-bottom:1px solid var(--line)}}
  .bounty .sev{font-family:"JetBrains Mono",monospace;font-size:10.5px;letter-spacing:.08em;color:var(--muted);text-transform:uppercase;margin-bottom:10px}
  .bounty .amt{font-family:"Fraunces",serif;font-weight:300;font-size:40px;letter-spacing:-.02em;color:var(--ink);line-height:1;display:block;margin-bottom:8px}
  .bounty .amt em{color:var(--accent);font-style:italic}
  .bounty p{font-size:13px;color:var(--muted);line-height:1.55}

  .hof{margin-top:40px}
  .hof-row{display:grid;grid-template-columns:100px 1fr auto;gap:20px;padding:14px 0;border-bottom:1px solid var(--line);align-items:baseline}
  @media (max-width:640px){.hof-row{grid-template-columns:80px 1fr}.hof-row .p{grid-column:2;justify-self:start;margin-top:4px}}
  .hof-row .d{font-family:"JetBrains Mono",monospace;font-size:12px;color:var(--accent)}
  .hof-row .n{font-family:"Fraunces",serif;font-style:italic;font-size:17px;color:var(--ink)}
  .hof-row .p{font-size:13px;color:var(--muted);font-family:"JetBrains Mono",monospace}
`;

export default function DisclosurePage() {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: pageStyles }} />

      <section className="sp-hero">
        <div className="blob a"></div>
        <div className="wrap sp-hero-inner">
          <span className="sp-eyebrow"><span className="eb-dot"></span>Company &middot; Disclosure</span>
          <h1>If you find a flaw, we <em>want</em> it.</h1>
          <p className="lead">Our customers are tribunals and witness-protection programs. A bug in our custody engine does not mean an inconvenienced user &mdash; it can mean a case file whose chain of custody silently breaks. If you have found a vulnerability in VaultKeeper, please report it. This page is how.</p>
        </div>
      </section>

      <section className="sp-section">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">How to report<small>Three channels</small></div>
          <div className="sp-body legal">
            <h3>1 &middot; Preferred: <em>encrypted</em> email</h3>
            <p><strong>security@vaultkeeper.coop</strong>, encrypted to our PGP key <code style={{ fontFamily: "'JetBrains Mono',monospace", fontSize: '13px', background: 'var(--bg-2)', padding: '2px 6px', borderRadius: '4px' }}>3F9A 12C4 8B7D 4E22 7F55 C104 A9AB 6620 44D8</code>. Acknowledgement in 24 hours, triage in 72 hours, fix timeline communicated within 10 business days. We do not ask you to sign an NDA to report.</p>

            <h3>2 &middot; Signal</h3>
            <p>For urgent or high-sensitivity reports, Signal username <strong>vk.security.01</strong>. Two engineers monitor it on rotation. If you need anonymity, use a burner Signal identity &mdash; we will not log or correlate metadata.</p>

            <h3>3 &middot; Paper</h3>
            <p>If digital channels feel unsafe, send a sealed envelope to <strong>Security, VaultKeeper Co&ouml;peratief U.A., Prinsengracht 54, 1015 DV Amsterdam</strong>. Include a return Signal handle or a PGP key on paper. Yes, we do receive these.</p>

            <h3>What to include</h3>
            <p>A clear reproducer, affected version(s), impact you believe it has, and &mdash; if you want public credit &mdash; the name or handle you want listed in the advisory. You do <em>not</em> need to include a suggested fix; you do <em>not</em> need to prove exploitability beyond a reasonable test case.</p>
          </div>
        </div>
      </section>

      <section className="sp-section" style={{ background: 'var(--paper)' }}>
        <div className="wrap sp-grid-12">
          <div className="sp-rail">The bounty<small>What we pay</small></div>
          <div className="sp-body legal">
            <h2 style={{ fontFamily: "'Fraunces',serif", fontWeight: 300, fontSize: '42px', letterSpacing: '-.025em', lineHeight: 1.1, maxWidth: '24ch' }}>Modest, <em>predictable,</em> and we pay within 30 days.</h2>
            <p style={{ marginTop: '20px' }}>The VaultKeeper bounty is not Silicon-Valley scale. We are a cooperative. But every bounty we have committed to, we have paid &mdash; and we publish the ledger. Severity is scored against <strong>impact on a tribunal&rsquo;s custody chain</strong>, not against generic CVSS.</p>

            <div className="bounty">
              <div><div className="sev">Critical</div><span className="amt">&euro;<em>15,000</em></span><p>Breaks custody-chain integrity &mdash; silent tampering undetectable by vk-clerk.</p></div>
              <div><div className="sev">High</div><span className="amt">&euro;<em>6,000</em></span><p>Bypasses witness-quorum or escalates cross-tenant under normal configuration.</p></div>
              <div><div className="sev">Medium</div><span className="amt">&euro;<em>1,500</em></span><p>Data disclosure, privilege escalation, or DoS with production impact.</p></div>
              <div><div className="sev">Low</div><span className="amt">&euro;<em>300</em></span><p>Hardening improvements, information leaks without direct impact.</p></div>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-section">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">Safe harbour<small>Our promise to researchers</small></div>
          <div className="sp-body legal">
            <h3>We will <em>not</em> take legal action against good-faith research</h3>
            <p>If you are acting in good faith &mdash; not exfiltrating data, not targeting live customer deployments, not harming availability &mdash; we will not pursue civil or criminal action against you. This commitment applies against the Dutch Wet Computercriminaliteit, the US CFAA, and any equivalent under which we might otherwise have standing. We do not sue researchers. We hire them.</p>
            <p>Researchers acting under this safe harbour may test against our public demo instance at <strong>demo.vaultkeeper.coop</strong>, or a self-hosted instance you have set up yourself. Do not test against customer instances; if you believe a customer instance is vulnerable, tell us and we&rsquo;ll coordinate.</p>

            <h3>Public disclosure timeline</h3>
            <p>We commit to patching critical issues within 30 days of a validated report, high-severity within 60, medium within 90. Public advisories are published after a patch is deployed to all supported customers, typically 14 days later. You are free to publish your own write-up after our advisory; we will link to it in the security page.</p>
          </div>
        </div>
      </section>

      <section className="sp-section" style={{ background: 'var(--paper)' }}>
        <div className="wrap sp-grid-12">
          <div className="sp-rail">Hall of thanks<small>Researchers, credited</small></div>
          <div className="sp-body">
            <div className="hof">
              <div className="hof-row"><span className="d">2025-11-14</span><span className="n">Shira Almog &middot; timing oracle in PKCS#11 HSM wrapper</span><span className="p">VK-2025-0014 &middot; HIGH</span></div>
              <div className="hof-row"><span className="d">2025-09-02</span><span className="n">@tuppi (indep.) &middot; VKE1 peer-pairing replay within 4-hour window</span><span className="p">VK-2025-0011 &middot; MEDIUM</span></div>
              <div className="hof-row"><span className="d">2025-06-27</span><span className="n">Radix Labs (Berlin) &middot; witness-quorum threshold bypass, specific misconfig</span><span className="p">VK-2025-0008 &middot; HIGH</span></div>
              <div className="hof-row"><span className="d">2025-04-11</span><span className="n">Luc&iacute;a Mor&aacute;n &middot; TLS cert pinning bypass in air-gap import flow</span><span className="p">VK-2025-0005 &middot; MEDIUM</span></div>
              <div className="hof-row"><span className="d">2024-12-01</span><span className="n">Ahmed Bilal &middot; race condition in CRDT merge with conflicting witness signatures</span><span className="p">VK-2024-0022 &middot; HIGH</span></div>
              <div className="hof-row"><span className="d">2024-10-09</span><span className="n">@qaz (indep.) &middot; SSRF in on-prem AI callback registration</span><span className="p">VK-2024-0018 &middot; MEDIUM</span></div>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-quote">
        <div className="wrap">
          <blockquote>&ldquo;The process was clean. I had an encrypted reply in six hours, a patch branch in four days, and a bounty paid in three weeks. They <em>wrote me into the advisory</em> the way I asked to be written. No surprises.&rdquo;</blockquote>
          <p className="who"><strong>Shira Almog</strong> &mdash; Independent researcher, VK-2025-0014</p>
        </div>
      </section>

      <section className="wrap" style={{ padding: '40px 0 32px' }}>
        <div className="cta-banner">
          <div>
            <h2>We also <em>proactively</em> commission audits.</h2>
            <p>NCC Group runs a full audit of the custody engine annually. The 2025 report is linked from our security page &mdash; redactions limited to customer-specific deployment detail.</p>
          </div>
          <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap', justifyContent: 'flex-end' }}>
            <Link className="btn ghost" href="/security">Security architecture</Link>
            <Link className="btn" href="/legal">Legal &amp; imprint <span className="arr">&rarr;</span></Link>
          </div>
        </div>
      </section>
    </>
  );
}
