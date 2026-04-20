import type { Metadata } from 'next';
import Link from 'next/link';

export const metadata: Metadata = {
  title: 'Chain of custody',
  description:
    'An append-only cryptographic structure, hash-chained and witness-signed, that the database refuses to modify — even under court subpoena.',
  alternates: { languages: { en: '/en/custody', fr: '/fr/custody' } },
};

const pageStyles = `
  .cu-chain{background:var(--ink);color:var(--bg);border-radius:var(--radius-lg);padding:32px;font-family:"JetBrains Mono",monospace;font-size:12px;line-height:1.8;overflow-x:auto}
  .cu-chain .row{display:grid;grid-template-columns:56px 160px 1fr 140px;gap:18px;padding:8px 0;border-bottom:1px dashed rgba(255,255,255,.08);align-items:baseline;min-width:820px}
  .cu-chain .row:last-child{border-bottom:none}
  .cu-chain .idx{color:var(--accent-soft);font-style:italic;font-family:"Fraunces",serif;font-size:14px}
  .cu-chain .ts{color:var(--muted-2)}
  .cu-chain .ev{color:var(--bg)}
  .cu-chain .ev strong{color:var(--accent-soft);font-weight:400;font-style:italic;font-family:"Fraunces",serif}
  .cu-chain .hash{color:var(--muted-2);font-size:10.5px;text-overflow:ellipsis;overflow:hidden;white-space:nowrap}
  .cu-chain .bind{color:#6c6258;font-size:10px;margin-left:74px;padding-left:14px;border-left:1px solid rgba(255,255,255,.12);display:block}
  .lemma{display:grid;grid-template-columns:1fr 1fr;gap:0;margin-top:32px;border:1px solid var(--line);border-radius:var(--radius)}
  @media (max-width:760px){.lemma{grid-template-columns:1fr}}
  .lemma > div{padding:28px}
  .lemma > div:first-child{border-right:1px solid var(--line)}
  @media (max-width:760px){.lemma > div:first-child{border-right:none;border-bottom:1px solid var(--line)}}
  .lemma h5{font-family:"JetBrains Mono",monospace;font-size:11px;letter-spacing:.08em;text-transform:uppercase;color:var(--muted);margin-bottom:12px}
  .lemma .claim{font-family:"Fraunces",serif;font-size:22px;font-weight:400;letter-spacing:-.015em;line-height:1.25}
  .lemma .claim em{color:var(--accent);font-style:italic}
  .lemma code{font-family:"JetBrains Mono",monospace;font-size:12.5px;background:var(--bg-2);padding:2px 6px;border-radius:4px}
`;

export default function CustodyPage() {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: pageStyles }} />

      <section className="sp-hero">
        <div className="blob a"></div>
        <div className="wrap sp-hero-inner">
          <span className="sp-eyebrow"><span className="eb-dot"></span>Platform · 02 of 05</span>
          <h1>A chain defence counsel <em>cannot</em> cut.</h1>
          <p className="lead">The custody log is not a table with a timestamp column. It is an append-only cryptographic structure, hash-chained and witness-signed, that the database engine itself refuses to modify — even if ordered to by a superuser with a court subpoena.</p>
          <div className="sp-hero-meta">
            <div><span className="k">Storage</span><span className="v">PostgreSQL RLS</span></div>
            <div><span className="k">Chain</span><span className="v">SHA-256 linked</span></div>
            <div><span className="k">Mutation</span><span className="v"><em>impossible</em></span></div>
            <div><span className="k">Verification</span><span className="v">offline · open</span></div>
          </div>
        </div>
      </section>

      <section className="sp-section">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">The ledger<small>Hash-chained rows</small></div>
          <div className="sp-body">
            <h2>Every action <em>points</em> to the one before it.</h2>
            <p className="sp-lead">Each row in the custody ledger includes the SHA-256 of the previous row. To forge a single event, an attacker must recompute every subsequent row and re-collect every subsequent witness signature. The ledger becomes more expensive to fake with every passing hour.</p>
            <div className="cu-chain" style={{ marginTop: '40px' }}>
              <div className="row"><span className="idx">48214</span><span className="ts">19.04 · 14:02:41</span><span className="ev"><strong>seal</strong> · drone_footage_butcha.mp4 · ICC-UKR-2024</span><span className="hash">sha:a7f4…d029</span></div>
              <span className="bind">↳ binds prev 48213 · sha:e1c0…4812 · witness-01, 03</span>
              <div className="row"><span className="idx">48215</span><span className="ts">19.04 · 14:08:12</span><span className="ev"><strong>access</strong> · viewed by investigator · W. Nyoka</span><span className="hash">sha:b802…76aa</span></div>
              <span className="bind">↳ binds prev 48214 · sha:a7f4…d029 · witness-02</span>
              <div className="row"><span className="idx">48216</span><span className="ts">19.04 · 14:21:05</span><span className="ev"><strong>annotate</strong> · redaction applied · face/faces</span><span className="hash">sha:c44d…9912</span></div>
              <span className="bind">↳ binds prev 48215 · sha:b802…76aa · witness-01, 02, 03 (quorum)</span>
              <div className="row"><span className="idx">48217</span><span className="ts">19.04 · 14:44:30</span><span className="ev"><strong>link</strong> · corroborated with witness W-0144 testimony</span><span className="hash">sha:3e11…0055</span></div>
              <span className="bind">↳ binds prev 48216 · sha:c44d…9912 · witness-02, 03</span>
              <div className="row"><span className="idx">48218</span><span className="ts">19.04 · 15:02:18</span><span className="ev"><strong>export</strong> · packaged to defence · ICC-UKR-2024-PKG-039</span><span className="hash">sha:91ab…ccf2</span></div>
              <span className="bind">↳ binds prev 48217 · sha:3e11…0055 · witness-01, 02, 03 (full quorum for export)</span>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-section dark">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">The refusal<small>Why superusers can&rsquo;t help</small></div>
          <div className="sp-body">
            <h2>We told PostgreSQL to refuse, and we <em>meant it.</em></h2>
            <p className="sp-lead">Row-level security policies on <code style={{ background: 'rgba(255,255,255,.08)', padding: '2px 6px', borderRadius: '4px', color: 'var(--accent-soft)' }}>custody_events</code> deny <strong>UPDATE</strong> and <strong>DELETE</strong> to every role — including <code style={{ background: 'rgba(255,255,255,.08)', padding: '2px 6px', borderRadius: '4px', color: 'var(--accent-soft)' }}>postgres</code> itself. The only way around it is to drop the table, which breaks the chain — defence counsel will see it, the clerk validator will see it, and the case will collapse at admissibility.</p>
            <div className="sp-cols3">
              <div className="c"><span className="n">RLS</span><h4>DB-enforced</h4><p>Not app-enforced. The refusal lives below ORM, below the API layer, below any admin panel. Even <code style={{ background: 'rgba(255,255,255,.08)', padding: '1px 5px', borderRadius: '3px', fontFamily: "'JetBrains Mono',monospace", fontSize: '11px', color: 'var(--accent-soft)' }}>psql</code> cannot edit a sealed row.</p></div>
              <div className="c"><span className="n">WORM</span><h4>Disk-enforced</h4><p>Optional: back the tablespace with WORM-compliant storage (Cohesity, NetApp SnapLock). Now even a DROP TABLE requires physical intervention.</p></div>
              <div className="c"><span className="n">PROOF</span><h4>Chain-enforced</h4><p>If someone <em>does</em> drop and reconstruct: the chain&rsquo;s Merkle root diverges from every peer&rsquo;s copy. The defence&rsquo;s clerk validator reports the break in under a second.</p></div>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-section">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">Proofs<small>What the validator checks</small></div>
          <div className="sp-body">
            <h2>Every exhibit ships with its own <em>proof of life.</em></h2>
            <p className="sp-lead">When you export a case file, VaultKeeper bundles the evidence, the custody log, a timestamped hash manifest, and a 240-line Python validator. Defence counsel&rsquo;s clerk runs the validator offline. Either the chain holds, or it doesn&rsquo;t — no phone call to our support team required.</p>
            <div className="lemma">
              <div><h5>The claim</h5><p className="claim">Row 48,217 <em>cannot</em> have been forged after 19 April 2026, 14:44:30 UTC.</p></div>
              <div><h5>The proof</h5><p style={{ fontSize: '14.5px', color: 'var(--muted)', lineHeight: 1.6 }}>The row&rsquo;s hash is bound to row 48,216, which was RFC-3161 timestamped by <code>tsa.dfn.de</code>. A forgery would require breaking SHA-256, the TSA&rsquo;s RSA key, <em>and</em> collecting post-hoc signatures from witnesses 02 and 03.</p></div>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-quote">
        <div className="wrap">
          <blockquote>&ldquo;The custody log used to be a <em>theatre</em>. Now it is an artefact. The cross-examination on chain-of-custody at last month&rsquo;s sentencing hearing lasted eight minutes.&rdquo;</blockquote>
          <p className="who"><strong>Juge Hélène Morel</strong> — Presiding judge, Tribunal Judiciaire de Paris · cybercrime chamber</p>
        </div>
      </section>

      <section className="wrap" style={{ padding: '40px 0 32px' }}>
        <div className="cta-banner">
          <div>
            <h2>Run the validator on a <em>real</em> case file.</h2>
            <p>Download a sample sealed export. Run the clerk validator. See the chain verify on your own machine in under 90 seconds.</p>
          </div>
          <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap', justifyContent: 'flex-end' }}>
            <Link className="btn ghost" href="/validator">Clerk validator</Link>
            <Link className="btn" href="/contact">Book a demo <span className="arr">→</span></Link>
          </div>
        </div>
      </section>

      <section className="wrap">
        <div className="sp-nextprev">
          <Link href="/evidence"><span className="k">Prev · 01/05</span><h5>Evidence management</h5><p>The ingest checkpoint — where bytes become exhibits.</p></Link>
          <Link href="/witness"><span className="k">Next · 03/05</span><h5>Witness protection</h5><p>Pseudonymisation, voice masking, and real-name break-the-glass.</p></Link>
        </div>
      </section>
    </>
  );
}
