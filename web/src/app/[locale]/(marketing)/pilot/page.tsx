import type { Metadata } from 'next';
import Link from 'next/link';
import { PilotForm } from './content';

export const metadata: Metadata = {
  title: 'Start a pilot',
  description:
    'A VaultKeeper pilot is not a salescall. Thirty minutes, your hardware, three real exhibits ingested and sealed.',
  alternates: { languages: { en: '/en/pilot', fr: '/fr/pilot' } },
};

const pageStyles = `
  .steplist{margin-top:48px;display:grid;grid-template-columns:repeat(4,1fr);gap:0;border-top:1px solid var(--line);border-bottom:1px solid var(--line)}
  @media (max-width:760px){.steplist{grid-template-columns:1fr}}
  .steplist .st{padding:32px 28px 32px 0;border-right:1px solid var(--line)}
  .steplist .st:last-child{border-right:none;padding-right:0}
  .steplist .st:first-child{padding-left:0}
  @media (max-width:760px){.steplist .st{padding:28px 0;border-right:none;border-bottom:1px solid var(--line)}.steplist .st:last-child{border-bottom:none}}
  .steplist .t{font-family:"JetBrains Mono",monospace;font-size:11px;letter-spacing:.08em;color:var(--muted);text-transform:uppercase;margin-bottom:10px}
  .steplist .n{font-family:"Fraunces",serif;font-style:italic;color:var(--accent);font-size:20px;line-height:1;display:block;margin-bottom:6px}
  .steplist h4{font-family:"Fraunces",serif;font-weight:400;font-size:22px;letter-spacing:-.015em;margin-bottom:10px;line-height:1.2}
  .steplist p{color:var(--muted);font-size:14px;line-height:1.55}

  .pilot-form{background:var(--paper);border:1px solid var(--line);border-radius:var(--radius-lg);padding:40px;margin-top:48px}
  .pilot-form .fhead{display:flex;justify-content:space-between;align-items:baseline;margin-bottom:24px;padding-bottom:18px;border-bottom:1px solid var(--line)}
  .pilot-form .fhead h3{font-family:"Fraunces",serif;font-weight:400;font-size:26px;letter-spacing:-.015em}
  .pilot-form .fhead h3 em{color:var(--accent);font-style:italic}
  .pilot-form .fhead span{font-family:"JetBrains Mono",monospace;font-size:11px;letter-spacing:.06em;color:var(--muted);text-transform:uppercase}
  .pilot-form .row{display:grid;grid-template-columns:1fr 1fr;gap:18px;margin-bottom:18px}
  @media (max-width:640px){.pilot-form .row{grid-template-columns:1fr}}
  .pilot-form label{display:block;font-size:13px;color:var(--muted);margin-bottom:6px;font-family:"JetBrains Mono",monospace;letter-spacing:.04em;text-transform:uppercase}
  .pilot-form input,.pilot-form select,.pilot-form textarea{width:100%;padding:14px 16px;border:1px solid var(--line);border-radius:10px;background:var(--bg);font-family:"Fraunces",serif;font-size:17px;color:var(--ink);font-style:italic;letter-spacing:-.005em}
  .pilot-form input::placeholder,.pilot-form textarea::placeholder{color:var(--muted-2);font-style:italic}
  .pilot-form input:focus,.pilot-form select:focus,.pilot-form textarea:focus{outline:none;border-color:var(--accent);box-shadow:0 0 0 3px rgba(200,126,94,.12)}
  .pilot-form textarea{min-height:120px;font-family:"Inter",sans-serif;font-style:normal;font-size:15px;line-height:1.5;resize:vertical}
  .pilot-form .submit{display:flex;justify-content:space-between;align-items:center;margin-top:8px}
  .pilot-form .note{font-size:13px;color:var(--muted);max-width:44ch;line-height:1.5}
`;

export default function PilotPage() {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: pageStyles }} />

      <section className="sp-hero">
        <div className="blob a"></div>
        <div className="wrap sp-hero-inner">
          <span className="sp-eyebrow"><span className="eb-dot"></span>For institutions &middot; Pilot</span>
          <h1>Thirty minutes, <em>your</em> hardware, three <em>real</em> exhibits.</h1>
          <p className="lead">A VaultKeeper pilot is not a salescall. It is a working session in which we spin up the system on infrastructure you control, ingest a handful of your actual exhibits, seal them into the custody chain, and hand you an export your tribunal&rsquo;s clerk can verify. If it doesn&rsquo;t work, you walk away. If it does, you keep the box.</p>
          <div className="sp-hero-meta">
            <div><span className="k">Session length</span><span className="v">30&ndash;45 min</span></div>
            <div><span className="k">Cost</span><span className="v"><em>&euro;0</em></span></div>
            <div><span className="k">Your hardware</span><span className="v">or ours &middot; shared</span></div>
            <div><span className="k">Post-call</span><span className="v">the box stays up</span></div>
          </div>
        </div>
      </section>

      <section className="sp-section">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">The arc<small>A pilot, end-to-end</small></div>
          <div className="sp-body">
            <h2>What happens on the <em>call.</em></h2>

            <div className="steplist">
              <div className="st">
                <div className="t">T-0 &middot; 00:00</div>
                <span className="n">01</span>
                <h4>We meet</h4>
                <p>Your head of IT, registrar, and one investigator join the call. We confirm jurisdiction, sovereignty requirements, and what you care most about proving.</p>
              </div>
              <div className="st">
                <div className="t">T+05 &middot; 00:05</div>
                <span className="n">02</span>
                <h4>Hardware up</h4>
                <p>We spin up a VaultKeeper instance on your Hetzner/Scaleway/AWS account (or a shared pilot node, your choice). DNS, TLS, witness nodes: all live.</p>
              </div>
              <div className="st">
                <div className="t">T+15 &middot; 00:15</div>
                <span className="n">03</span>
                <h4>Ingest three exhibits</h4>
                <p>Three real files of yours &mdash; or three from our demo corpus. Hashed, timestamped, witness-signed, sealed. You watch the custody log in real time.</p>
              </div>
              <div className="st">
                <div className="t">T+30 &middot; 00:30</div>
                <span className="n">04</span>
                <h4>Export &amp; verify</h4>
                <p>We export a tribunal-shaped bundle. You run the clerk validator on your laptop. Either the chain holds, or we&rsquo;ve failed the pilot.</p>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-section">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">Request a slot<small>We&rsquo;ll confirm within 48h</small></div>
          <div className="sp-body">
            <h2>Tell us the <em>shape</em> of your institution.</h2>
            <p className="sp-lead">We schedule two pilots per week. Most are run by our tribunal-operations lead, Dr.&nbsp;Ingrid Velasco; specialist sessions (commissions, ICC-scale, air-gapped) are run by our CTO, Jonas Arvidsson. Fill this in and we&rsquo;ll match you.</p>

            <PilotForm />
          </div>
        </div>
      </section>

      <section className="sp-quote">
        <div className="wrap">
          <blockquote>&ldquo;The pilot call ran thirty-two minutes. The ingest worked. The export validated. We <em>kept</em> the box and went to production ninety days later.&rdquo;</blockquote>
          <p className="who"><strong>Pieter De Vries</strong> &mdash; Registrar, a Dutch prosecution unit (name withheld by request)</p>
        </div>
      </section>

      <section className="wrap">
        <div className="sp-nextprev">
          <Link href="/ngos"><span className="k">Small teams</span><h5>NGOs in The Hague</h5><p>Three-person shops, Starter tier.</p></Link>
          <Link href="/icc"><span className="k">Large bodies</span><h5>ICC-scale</h5><p>Multi-site sovereign architecture.</p></Link>
        </div>
      </section>
    </>
  );
}
