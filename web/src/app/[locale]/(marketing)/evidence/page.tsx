import type { Metadata } from 'next';
import Link from 'next/link';

export const metadata: Metadata = {
  title: 'Evidence management',
  description:
    'Ingestion is where evidence either becomes admissible or becomes a liability. Every file timestamped, hashed, and countersigned before it is allowed to exist.',
  alternates: { languages: { en: '/en/evidence', fr: '/fr/evidence' } },
};

const pageStyles = `
  .ev-ingest{background:var(--paper);border:1px solid var(--line);border-radius:var(--radius-lg);padding:28px;font-family:"JetBrains Mono",monospace;font-size:12.5px;line-height:1.75;color:var(--muted)}
  .ev-ingest .h{display:flex;justify-content:space-between;border-bottom:1px solid var(--line);padding-bottom:12px;margin-bottom:16px;font-size:11px;letter-spacing:.06em;text-transform:uppercase}
  .ev-ingest .row{display:grid;grid-template-columns:90px 1fr auto;gap:14px;padding:6px 0;align-items:baseline}
  .ev-ingest .ok{color:var(--accent)}
  .ev-ingest .err{color:#b0543a}
  .ev-ingest .k{color:var(--ink)}
  .ev-ingest .hash{font-size:11px;color:var(--muted-2);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .formats{display:grid;grid-template-columns:repeat(6,1fr);gap:0;margin-top:40px;border-top:1px solid var(--line);border-bottom:1px solid var(--line)}
  @media (max-width:860px){.formats{grid-template-columns:repeat(3,1fr)}}
  @media (max-width:480px){.formats{grid-template-columns:repeat(2,1fr)}}
  .fmt{padding:20px 16px;border-right:1px solid var(--line);font-family:"JetBrains Mono",monospace;font-size:11px;color:var(--muted);letter-spacing:.04em}
  .fmt:last-child{border-right:none}
  .fmt strong{display:block;font-family:"Fraunces",serif;font-style:italic;font-size:22px;color:var(--ink);letter-spacing:-.01em;margin-bottom:6px;font-weight:400}
`;

export default function EvidencePage() {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: pageStyles }} />

      <section className="sp-hero">
        <div className="blob a"></div>
        <div className="wrap sp-hero-inner">
          <span className="sp-eyebrow"><span className="eb-dot"></span>Platform · 01 of 05</span>
          <h1>Every byte <em>witnessed</em> the moment it enters the room.</h1>
          <p className="lead">Ingestion is where evidence either becomes admissible or becomes a liability. VaultKeeper treats the door of the evidence locker as a cryptographic checkpoint — every file timestamped, hashed, and countersigned before it is allowed to exist.</p>
          <div className="sp-hero-meta">
            <div><span className="k">Formats</span><span className="v">218 verified</span></div>
            <div><span className="k">Hashing</span><span className="v">SHA-256 + BLAKE3</span></div>
            <div><span className="k">Timestamp</span><span className="v">RFC 3161 <em>(TSA pool)</em></span></div>
            <div><span className="k">Max file size</span><span className="v">tested to 4 TB</span></div>
          </div>
        </div>
      </section>

      <section className="sp-section">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">Intake · Sealing<small>What ingest actually does</small></div>
          <div className="sp-body">
            <h2>The door is not a <em>web upload.</em> It&rsquo;s a notarised border crossing.</h2>
            <p className="sp-lead">Files cross the boundary through a resumable chunked transfer. Each 4&nbsp;MB chunk is hashed in the browser, reconciled server-side, and countersigned by at least two of your witness nodes before the file manifest is committed. If reconciliation fails, the file is <strong>never written to disk in a usable form</strong> — we don&rsquo;t do partials.</p>
            <div className="sp-cols3">
              <div className="c"><span className="n">01</span><h4>Client-side hash</h4><p>SHA-256 is computed in the investigator&rsquo;s browser before a single byte leaves their machine. The server compares against the hash it independently derives from the reassembled stream.</p></div>
              <div className="c"><span className="n">02</span><h4>RFC 3161 timestamp</h4><p>The combined manifest is sent to a configurable TSA pool (default: three geographically-diverse authorities). The returned token is stored alongside the hash.</p></div>
              <div className="c"><span className="n">03</span><h4>Witness countersignature</h4><p>Quorum of witness nodes — institutional peers you&rsquo;ve federated with — each sign the manifest. The seal is valid only once ≥ 2 of ≥ 3 have signed.</p></div>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-section" style={{ borderTop: 'none', paddingTop: 0 }}>
        <div className="wrap sp-grid-12">
          <div className="sp-rail">Transcript<small>A real ingest log</small></div>
          <div className="sp-body">
            <div className="ev-ingest">
              <div className="h"><span>case · ICC-UKR-2024 · exhibit intake</span><span>19 Apr 2026 · 14:02:07 UTC</span></div>
              <div className="row"><span>14:02:07</span><span><span className="k">OPEN</span> session · auth <span className="ok">witness-03</span></span><span>—</span></div>
              <div className="row"><span>14:02:12</span><span>UPLOAD <span className="k">drone_footage_butcha.mp4</span> · 218 MB · 54 chunks</span><span className="ok">✓</span></div>
              <div className="row"><span>14:02:39</span><span>&nbsp;&nbsp;chunk 54/54 · recv · client sha256 reconciled</span><span className="ok">match</span></div>
              <div className="row"><span>14:02:41</span><span>&nbsp;&nbsp;TSA <span className="k">tsa.dfn.de</span> · RFC 3161 token requested</span><span className="ok">ok</span></div>
              <div className="row"><span>14:02:43</span><span>&nbsp;&nbsp;TSA <span className="k">ts.eu-west</span> · secondary token</span><span className="ok">ok</span></div>
              <div className="row"><span>14:02:45</span><span>&nbsp;&nbsp;witness-01 · eIDAS QES signature</span><span className="ok">ok</span></div>
              <div className="row"><span>14:02:46</span><span>&nbsp;&nbsp;witness-02 · eIDAS QES signature</span><span className="ok">ok</span></div>
              <div className="row"><span>14:02:46</span><span>&nbsp;&nbsp;<span className="k">SEAL COMMITTED</span> · chain-idx 48,217</span><span className="ok">▣</span></div>
              <div className="row"><span>14:02:47</span><span className="hash">&nbsp;&nbsp;sha256:a7f4…d029 · sig:3045022100b8e9…</span><span>—</span></div>
              <div className="row"><span>14:02:52</span><span>UPLOAD <span className="k">witness-0144-corrupt.mp4</span> · 12 MB</span><span className="err">✗</span></div>
              <div className="row"><span>14:02:54</span><span>&nbsp;&nbsp;chunk 3/3 · client hash mismatch · <span className="err">409 CONFLICT</span></span><span className="err">reject</span></div>
              <div className="row"><span>14:02:54</span><span>&nbsp;&nbsp;<span className="k">NOT WRITTEN</span> — no partial exhibit created</span><span>—</span></div>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-section">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">Formats<small>What we understand</small></div>
          <div className="sp-body">
            <h2>218 formats, decoded <em>locally</em>, never in a foreign cloud.</h2>
            <p className="sp-lead">Video, audio, chat exports, mobile forensic images, satellite imagery, proprietary drone formats, encrypted containers, hostile-captured devices — all parsed on the box you own. VaultKeeper ships the parsers; nothing leaves the perimeter.</p>
            <div className="formats">
              <div className="fmt"><strong>AV</strong>H.264 · H.265<br />AV1 · ProRes</div>
              <div className="fmt"><strong>Audio</strong>WAV · FLAC<br />OGG · OPUS</div>
              <div className="fmt"><strong>Mobile</strong>Cellebrite UFDR<br />GrayKey AFF4</div>
              <div className="fmt"><strong>Chat</strong>WhatsApp<br />Signal · Telegram</div>
              <div className="fmt"><strong>Imagery</strong>GeoTIFF · COG<br />NITF · KLV drone</div>
              <div className="fmt"><strong>Docs</strong>PDF · DOCX<br />EML · MSG · MBOX</div>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-quote">
        <div className="wrap">
          <blockquote>&ldquo;On the first day, we tried to upload a 3&nbsp;TB Nuix export that had survived three previous platforms. <em>VaultKeeper rejected 14 files</em> because the hashes didn&rsquo;t match what Nuix claimed. That was the moment I knew we&rsquo;d bought the right tool.&rdquo;</blockquote>
          <p className="who"><strong>Dr. Anna Seibel</strong> — Head of Digital Evidence, CIJA (Commission for International Justice and Accountability)</p>
        </div>
      </section>

      <section className="wrap" style={{ padding: '40px 0 32px' }}>
        <div className="cta-banner">
          <div>
            <h2>See a real ingest in <em>your</em> hardware.</h2>
            <p>We&rsquo;ll run a live 30-minute ingest against a dummy case file on your own infrastructure. You keep the logs.</p>
          </div>
          <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap', justifyContent: 'flex-end' }}>
            <Link className="btn ghost" href="/custody">Chain of custody →</Link>
            <Link className="btn" href="/contact">Book a demo <span className="arr">→</span></Link>
          </div>
        </div>
      </section>

      <section className="wrap">
        <div className="sp-nextprev">
          <Link href="/platform"><span className="k">← Platform overview</span><h5>All five pillars</h5><p>Ingest, custody, witnesses, collaboration, discovery — one sealed system.</p></Link>
          <Link href="/custody"><span className="k">Next · 02/05</span><h5>Chain of custody</h5><p>How an append-only log survives a hostile defence expert.</p></Link>
        </div>
      </section>
    </>
  );
}
