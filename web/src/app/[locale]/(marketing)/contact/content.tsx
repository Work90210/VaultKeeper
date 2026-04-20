'use client';

import { useState, FormEvent } from 'react';

const pageStyles = `
  .c-hero{padding:48px 0 24px}
  .c-hero h1{font-size:clamp(44px,5.4vw,84px)}
  .c-hero h1 em{color:var(--accent);font-style:italic}

  .form-wrap{display:grid;grid-template-columns:1.1fr .9fr;gap:56px;margin-top:32px}
  @media (max-width:920px){.form-wrap{grid-template-columns:1fr;gap:32px}}

  form.c-form{background:var(--paper);border:1px solid var(--line);border-radius:var(--radius-lg);padding:40px;display:flex;flex-direction:column;gap:20px}
  @media (max-width:560px){form.c-form{padding:24px}}
  .field{display:flex;flex-direction:column;gap:8px}
  .field label{font-size:13px;color:var(--muted);font-weight:500}
  .field input, .field textarea, .field select{
    padding:14px 16px;border-radius:12px;border:1px solid var(--line-2);
    background:var(--bg);font:inherit;font-size:15px;color:var(--ink);outline:none;transition:border-color .2s, box-shadow .2s;
  }
  .field input:focus, .field textarea:focus, .field select:focus{border-color:var(--accent);box-shadow:0 0 0 4px rgba(184,66,28,.1)}
  .field textarea{resize:vertical;min-height:120px;font-family:inherit}
  .row2{display:grid;grid-template-columns:1fr 1fr;gap:16px}
  @media (max-width:560px){.row2{grid-template-columns:1fr}}

  .chip-field{display:flex;flex-wrap:wrap;gap:8px}
  .chip-field label{display:inline-flex;align-items:center;gap:8px;padding:8px 14px;border-radius:999px;background:var(--bg);border:1px solid var(--line-2);cursor:pointer;font-size:13.5px;transition:all .2s}
  .chip-field label input{display:none}
  .chip-field label:has(input:checked){background:var(--ink);color:var(--bg);border-color:var(--ink)}

  .side{display:flex;flex-direction:column;gap:20px}
  .side-card{padding:28px;border-radius:var(--radius);background:var(--paper);border:1px solid var(--line)}
  .side-card h4{font-family:"Fraunces",serif;font-size:22px;margin-bottom:10px}
  .side-card p{color:var(--muted);font-size:14.5px;line-height:1.55}
  .side-card a{color:var(--accent)}
  .side-card .k{font-size:12px;color:var(--muted);margin-bottom:4px}
  .side-card .v{font-family:"Fraunces",serif;font-size:18px;font-style:italic}

  .office{padding:40px;border-radius:var(--radius);background:var(--ink);color:var(--bg);display:flex;flex-direction:column;gap:14px;position:relative;overflow:hidden}
  .office h4{color:var(--bg);font-size:24px;font-family:"Fraunces",serif;font-weight:400}
  .office p{color:var(--muted-2);font-size:14px;line-height:1.55}
  .office .addr{font-family:"Fraunces",serif;font-style:italic;font-size:18px;color:var(--bg);margin-top:8px;line-height:1.4}
  .office .hours{border-top:1px solid rgba(255,255,255,.12);padding-top:14px;margin-top:10px;color:var(--muted-2);font-size:13px}

  .success{display:none;padding:24px;background:rgba(74,107,58,.1);border:1px solid rgba(74,107,58,.3);border-radius:12px;color:var(--ok);font-size:14.5px;line-height:1.5;gap:14px}
  .success.on{display:flex}
  .success .icon{width:36px;height:36px;border-radius:50%;background:var(--ok);color:#fff;display:grid;place-items:center;flex-shrink:0;font-weight:500}
  .success strong{display:block;color:var(--ink);font-family:"Fraunces",serif;font-style:italic;font-weight:400;font-size:18px;margin-bottom:4px}
`;

export function ContactPageContent() {
  const [submitted, setSubmitted] = useState(false);

  const handleSubmit = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setSubmitted(true);
  };

  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: pageStyles }} />

      <section className="wrap c-hero">
        <div className="crumb"><a href="/">VaultKeeper</a> <span className="sep">/</span> <span>Contact</span></div>
        <span className="eyebrow"><span className="eb-dot"></span>Reach us directly</span>
        <h1 style={{ marginTop: '20px' }}>Tell us about your <em>work.</em></h1>
        <p className="lead" style={{ marginTop: '24px', maxWidth: '64ch' }}>We answer within one working day — with a named engineer on the thread, in writing, on letterhead. If you&apos;d rather skip the form, the addresses on the right work too.</p>

        <div className="form-wrap">
          <form className="c-form" onSubmit={handleSubmit}>
            <div className={`success${submitted ? ' on' : ''}`}>
              <div className="icon">{'\u2713'}</div>
              <div><strong>Received. You&apos;ll hear from us within a working day.</strong>A real human reads every inbound. We&apos;ll reply from <span className="mono">hello@vaultkeeper.eu</span>.</div>
            </div>

            <div className="row2">
              <div className="field">
                <label htmlFor="n">Your name</label>
                <input id="n" required placeholder="H&#233;l&#232;ne Mercier" />
              </div>
              <div className="field">
                <label htmlFor="org">Institution</label>
                <input id="org" required placeholder="ICC &#183; Office of the Prosecutor" />
              </div>
            </div>
            <div className="row2">
              <div className="field">
                <label htmlFor="e">Email</label>
                <input id="e" type="email" required placeholder="helene@icc-cpi.int" />
              </div>
              <div className="field">
                <label htmlFor="r">Role</label>
                <select id="r">
                  <option>Investigator / analyst</option>
                  <option>Prosecutor</option>
                  <option>Judge / chamber staff</option>
                  <option>IT / systems lead</option>
                  <option>CISO / compliance</option>
                  <option>Defence counsel</option>
                  <option>Other</option>
                </select>
              </div>
            </div>

            <div className="field">
              <label>What brings you here?</label>
              <div className="chip-field">
                <label><input type="checkbox" /> Request a demo</label>
                <label><input type="checkbox" /> Pricing for my institution</label>
                <label><input type="checkbox" /> Migration from RelativityOne</label>
                <label><input type="checkbox" /> Security review</label>
                <label><input type="checkbox" /> Air-gapped deployment</label>
                <label><input type="checkbox" /> Open-source contribution</label>
                <label><input type="checkbox" /> Press / research</label>
              </div>
            </div>

            <div className="row2">
              <div className="field">
                <label htmlFor="t">Team size</label>
                <select id="t">
                  <option>1{'\u2013'}10</option>
                  <option>11{'\u2013'}40</option>
                  <option>41{'\u2013'}150</option>
                  <option>150{'\u2013'}500</option>
                  <option>500+</option>
                </select>
              </div>
              <div className="field">
                <label htmlFor="j">Primary jurisdiction</label>
                <input id="j" placeholder="e.g. The Hague &#183; Phnom Penh &#183; Pristina" />
              </div>
            </div>

            <div className="field">
              <label htmlFor="msg">Tell us a little about the work</label>
              <textarea id="msg" placeholder="Typical caseload, what you're using today, what keeps you up at night about it..."></textarea>
            </div>

            <div style={{ display: 'flex', gap: '14px', alignItems: 'center', flexWrap: 'wrap' }}>
              <button type="submit" className="btn">Send &middot; expect a reply in a working day <span className="arr">{'\u2192'}</span></button>
              <span className="small">No tracking pixels &middot; no newsletter signup &middot; GDPR by default</span>
            </div>
          </form>

          <div className="side">
            <div className="side-card">
              <div className="k">Institutional sales</div>
              <div className="v" style={{ color: 'var(--accent)' }}>hello@vaultkeeper.eu</div>
              <p style={{ marginTop: '10px' }}>For demos, pricing, migration planning, and pilots. Reply within one working day.</p>
            </div>
            <div className="side-card">
              <div className="k">Security &amp; responsible disclosure</div>
              <div className="v" style={{ color: 'var(--accent)' }}>security@vaultkeeper.eu</div>
              <p style={{ marginTop: '10px' }}>PGP key on the website footer. 90-day coordinated disclosure. Hall of fame, no bug bounty — we prefer paying in money.</p>
            </div>
            <div className="side-card">
              <div className="k">Press &amp; research</div>
              <div className="v" style={{ color: 'var(--accent)' }}>press@vaultkeeper.eu</div>
              <p style={{ marginTop: '10px' }}>Happy to speak with journalists and academic researchers on the record about sovereignty in justice tech.</p>
            </div>
            <div className="office">
              <span className="eyebrow" style={{ background: 'rgba(255,255,255,.05)', color: 'var(--muted-2)', borderColor: 'rgba(255,255,255,.14)' }}><span className="eb-dot"></span>Headquarters</span>
              <h4 style={{ marginTop: '12px' }}>The Hague, Netherlands</h4>
              <p>By appointment only. We share a floor with two partner NGOs; institutional visits are welcome.</p>
              <div className="addr">Lange Voorhout 14<br />2514 ED Den Haag<br />Nederland</div>
              <div className="hours">Mon{'\u2013'}Fri &middot; 09:00{'\u2013'}18:00 CET &middot; KvK 94128-041</div>
            </div>
          </div>
        </div>
      </section>
    </>
  );
}
