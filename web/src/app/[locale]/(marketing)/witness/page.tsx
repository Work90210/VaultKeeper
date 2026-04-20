import type { Metadata } from 'next';
import Link from 'next/link';

export const metadata: Metadata = {
  title: 'Witness protection',
  description:
    'Pseudonymisation, voice masking, geographic fuzzing, and break-the-glass real-name access — structurally enforced by the database, not policy.',
  alternates: { languages: { en: '/en/witness', fr: '/fr/witness' } },
};

const pageStyles = `
  .idcard{background:var(--paper);border:1px solid var(--line);border-radius:var(--radius);padding:28px;display:grid;grid-template-columns:120px 1fr;gap:22px;align-items:start}
  @media (max-width:560px){.idcard{grid-template-columns:1fr}}
  .idcard .silhouette{aspect-ratio:1;border-radius:50%;background:linear-gradient(135deg,#e8dccb,#d4c5b0);position:relative;overflow:hidden;display:flex;align-items:center;justify-content:center;font-family:"Fraunces",serif;font-style:italic;font-size:34px;color:var(--paper)}
  .idcard .silhouette::after{content:"";position:absolute;inset:0;backdrop-filter:blur(8px);background:rgba(232,220,203,.3)}
  .idcard .silhouette span{position:relative;z-index:1}
  .idcard dl{margin:0;display:grid;grid-template-columns:auto 1fr;gap:6px 16px;font-size:13.5px}
  .idcard dt{font-family:"JetBrains Mono",monospace;font-size:10.5px;letter-spacing:.06em;text-transform:uppercase;color:var(--muted);align-self:center}
  .idcard dd{margin:0;font-family:"Fraunces",serif;font-size:16px;color:var(--ink);letter-spacing:-.005em}
  .idcard dd.redacted{font-family:"JetBrains Mono",monospace;font-size:13px;color:var(--muted-2);background:var(--bg-2);padding:2px 8px;border-radius:4px;display:inline-block;width:fit-content}
  .idcard .badge-row{grid-column:1/-1;display:flex;gap:8px;flex-wrap:wrap;margin-top:12px;padding-top:14px;border-top:1px solid var(--line)}
  .idcard .b{font-family:"JetBrains Mono",monospace;font-size:10.5px;padding:3px 9px;border-radius:999px;border:1px solid var(--line);color:var(--muted);letter-spacing:.04em;text-transform:uppercase}
  .idcard .b.ok{color:var(--accent);border-color:var(--accent-soft)}
  .break-glass{background:var(--ink);color:var(--bg);border-radius:var(--radius);padding:32px;margin-top:36px}
  .break-glass h4{font-family:"Fraunces",serif;font-weight:400;font-size:24px;color:var(--bg);margin-bottom:8px}
  .break-glass h4 em{color:var(--accent-soft);font-style:italic}
  .break-glass p{color:var(--muted-2);font-size:14.5px;line-height:1.6;max-width:62ch}
  .bg-flow{display:grid;grid-template-columns:repeat(4,1fr);gap:0;margin-top:24px;border-top:1px solid rgba(255,255,255,.14)}
  @media (max-width:720px){.bg-flow{grid-template-columns:1fr 1fr}}
  .bg-flow .step{padding:20px 16px 0 0;border-right:1px solid rgba(255,255,255,.14)}
  .bg-flow .step:last-child{border-right:none}
  @media (max-width:720px){.bg-flow .step:nth-child(2n){border-right:none}}
  .bg-flow .n{font-family:"Fraunces",serif;font-style:italic;color:var(--accent-soft);font-size:13px;margin-bottom:8px;display:block}
  .bg-flow h5{font-family:"Fraunces",serif;font-size:15px;font-weight:400;color:var(--bg);margin-bottom:4px}
  .bg-flow p{font-size:12.5px;color:var(--muted-2);line-height:1.5}
`;

export default function WitnessPage() {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: pageStyles }} />

      <section className="sp-hero">
        <div className="blob a"></div>
        <div className="wrap sp-hero-inner">
          <span className="sp-eyebrow"><span className="eb-dot"></span>Platform · 03 of 05</span>
          <h1>The investigator sees a <em>pseudonym.</em> The judge sees a name.</h1>
          <p className="lead">A witness who spoke to your team in Donetsk in 2022 is still alive in 2026 because their real name never touched a database. VaultKeeper enforces that discipline structurally — pseudonymisation is not a policy, it is a column the application literally cannot SELECT without quorum approval.</p>
          <div className="sp-hero-meta">
            <div><span className="k">Default identity</span><span className="v">pseudonymised</span></div>
            <div><span className="k">Real name</span><span className="v">break-the-glass</span></div>
            <div><span className="k">Voice masking</span><span className="v">pitch · formant</span></div>
            <div><span className="k">Geo</span><span className="v">fuzz · per-role</span></div>
          </div>
        </div>
      </section>

      <section className="sp-section">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">How they appear<small>Default view for all roles</small></div>
          <div className="sp-body">
            <h2>This is what your lead analyst sees.<br /><em>Every time.</em></h2>
            <p className="sp-lead">The witness record as it appears to investigators, translators, corroboration analysts, and the rest of your team. The pseudonym <strong>W-0144</strong> is a stable handle — cross-referenced across 34 exhibits, but decoupled from any personally-identifying information.</p>
            <div className="idcard" style={{ marginTop: '36px', maxWidth: '640px' }}>
              <div className="silhouette"><span>W</span></div>
              <dl>
                <dt>Handle</dt><dd>W-0144</dd>
                <dt>Given name</dt><dd className="redacted">[ sealed · access denied ]</dd>
                <dt>Surname</dt><dd className="redacted">[ sealed · access denied ]</dd>
                <dt>Region</dt><dd>Donetsk oblast (fuzz: 50 km)</dd>
                <dt>Age band</dt><dd>40–55</dd>
                <dt>Languages</dt><dd>uk · ru · en</dd>
                <dt>Exhibits</dt><dd>34 linked · 7 corroborated</dd>
                <dt>First contact</dt><dd>March 2024</dd>
                <div className="badge-row">
                  <span className="b ok">voice-masked</span>
                  <span className="b ok">geo-fuzzed</span>
                  <span className="b">relocation · tier 2</span>
                  <span className="b">contact via · intermediary</span>
                </div>
              </dl>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-section dark">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">Break-the-glass<small>The only path to a real name</small></div>
          <div className="sp-body">
            <h2>Real names <em>exist</em> — but only on the day the judge demands them.</h2>
            <p className="sp-lead">Real identity lives in a separate tablespace with its own encryption key, held by the presiding witness-protection officer. Accessing it is a four-party event: the officer, a second officer, the case lead, and the judge. Every attempt — successful or not — is sealed into the chain.</p>
            <div className="break-glass">
              <h4>The <em>break-the-glass</em> quorum</h4>
              <p>When the real identity of W-0144 is required — typically when the witness must appear in court under their own name — the unseal ceremony runs like this:</p>
              <div className="bg-flow">
                <div className="step"><span className="n">01</span><h5>Judge&rsquo;s order</h5><p>Signed PDF, filed in the case, referenced by exhibit ID.</p></div>
                <div className="step"><span className="n">02</span><h5>WP officer</h5><p>Primary custodian unlocks with HSM-held key #1.</p></div>
                <div className="step"><span className="n">03</span><h5>WP deputy</h5><p>Second custodian countersigns with HSM-held key #2.</p></div>
                <div className="step"><span className="n">04</span><h5>Sealed view</h5><p>Case lead sees the name. View is timeboxed to 12h. Sealed into chain.</p></div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-section">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">What we mask<small>Every surface, by default</small></div>
          <div className="sp-body">
            <h2>Your witness&rsquo;s <em>voice</em> is also data that can kill them.</h2>
            <p className="sp-lead">Identifying information is not just a name. It is a voice a neighbour recognises; a room a satellite can find; a phrase that only one person on Earth uses. VaultKeeper ships the masking pipeline on-box — no upload to a third-party speech API, no cloud model, no telemetry.</p>
            <div className="sp-rows">
              <div className="sp-row"><span className="idx">01 · Voice</span><h4>Pitch &amp; formant <em>shifted</em> — offline</h4><p>Per-witness pitch/formant keys are stored with the witness record. Playback for investigators uses the masked track; the clean track lives in the sealed tier and requires the same break-the-glass as real-name access.</p></div>
              <div className="sp-row"><span className="idx">02 · Face</span><h4>Redaction <em>baked</em> at export</h4><p>Face detection runs on-premises. Redactions are cryptographically linked to the source frame, so defence counsel can verify coverage without seeing the underlying face.</p></div>
              <div className="sp-row"><span className="idx">03 · Geography</span><h4>Fuzz <em>per role</em></h4><p>Analysts see 50&nbsp;km radius. Corroboration leads see 10&nbsp;km. Presiding judge sees the exact coordinates. Every tier step is a custody event.</p></div>
              <div className="sp-row"><span className="idx">04 · Text</span><h4>Named-entity <em>scrub</em></h4><p>LLM-assisted NER replaces names, villages, call-signs, and relatives in witness statements with stable handles. Original text kept sealed; scrubbed version is the default view.</p></div>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-quote">
        <div className="wrap">
          <blockquote>&ldquo;The first time a witness from 2022 contacted us again, in 2025, <em>we still had her real name.</em> We also had, in the same system, a full accounting of who had unsealed it, when, and why. She asked us to do exactly that accounting on the phone. We could.&rdquo;</blockquote>
          <p className="who"><strong>Marta Vasylenko</strong> — Head of witness programme, Ukrainian Truth Initiative</p>
        </div>
      </section>

      <section className="wrap" style={{ padding: '40px 0 32px' }}>
        <div className="cta-banner">
          <div>
            <h2>Walk through a <em>witness-protection</em> ceremony.</h2>
            <p>We&rsquo;ll simulate a break-the-glass unseal against a dummy witness on your infrastructure, with your officers, in about 40 minutes.</p>
          </div>
          <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap', justifyContent: 'flex-end' }}>
            <Link className="btn ghost" href="/security">Security architecture</Link>
            <Link className="btn" href="/contact">Book a demo <span className="arr">→</span></Link>
          </div>
        </div>
      </section>

      <section className="wrap">
        <div className="sp-nextprev">
          <Link href="/custody"><span className="k">Prev · 02/05</span><h5>Chain of custody</h5><p>How the append-only ledger makes forgery impossible.</p></Link>
          <Link href="/collaboration"><span className="k">Next · 04/05</span><h5>Live collaboration</h5><p>CRDT editing inside a sealed chain. How two tribunals work on one exhibit.</p></Link>
        </div>
      </section>
    </>
  );
}
