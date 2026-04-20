import type { Metadata } from 'next';
import Link from 'next/link';

export const metadata: Metadata = {
  title: 'Search & discovery',
  description:
    'Cross-exhibit semantic search, powered by on-box embedding models. No cloud API. No telemetry. Every result carries the custody hash of the source exhibit.',
  alternates: { languages: { en: '/en/search-discovery', fr: '/fr/search-discovery' } },
};

const pageStyles = `
  .search-demo{background:var(--paper);border:1px solid var(--line);border-radius:var(--radius-lg);padding:28px;margin-top:40px}
  .search-bar{display:flex;align-items:center;gap:14px;padding:16px 20px;border:1px solid var(--line);border-radius:12px;background:var(--bg);font-family:"Fraunces",serif;font-size:18px;color:var(--ink);font-style:italic}
  .search-bar::before{content:"/";color:var(--accent);font-family:"JetBrains Mono",monospace;font-style:normal;font-size:15px}
  .search-chips{display:flex;gap:8px;flex-wrap:wrap;margin-top:16px}
  .schip{font-family:"JetBrains Mono",monospace;font-size:11px;padding:5px 11px;border-radius:999px;background:var(--bg-2);color:var(--muted);letter-spacing:.04em;border:1px solid var(--line)}
  .schip.on{background:var(--ink);color:var(--bg);border-color:var(--ink)}
  .search-res{margin-top:22px;border-top:1px solid var(--line)}
  .res{display:grid;grid-template-columns:60px 1fr auto;gap:18px;padding:18px 0;border-bottom:1px solid var(--line);align-items:start}
  .res .score{font-family:"Fraunces",serif;font-style:italic;font-size:20px;color:var(--accent)}
  .res .meta{font-family:"JetBrains Mono",monospace;font-size:10.5px;letter-spacing:.06em;color:var(--muted);margin-bottom:4px;text-transform:uppercase}
  .res h5{font-family:"Fraunces",serif;font-weight:400;font-size:17px;letter-spacing:-.01em}
  .res p{color:var(--muted);font-size:13.5px;line-height:1.55;margin-top:4px}
  .res p mark{background:rgba(200,126,94,.2);padding:0 2px;color:var(--ink);font-weight:500}
  .res .tag{font-family:"JetBrains Mono",monospace;font-size:10.5px;color:var(--muted);padding:3px 8px;border-radius:999px;border:1px solid var(--line);white-space:nowrap}
`;

export default function SearchDiscoveryPage() {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: pageStyles }} />

      <section className="sp-hero">
        <div className="blob a"></div>
        <div className="wrap sp-hero-inner">
          <span className="sp-eyebrow"><span className="eb-dot"></span>Platform &middot; 05 of 05</span>
          <h1>Ask the archive <em>in your own words.</em> Get an answer it can prove.</h1>
          <p className="lead">Cross-exhibit semantic search, powered by on-box embedding models. No cloud API. No telemetry. Every result carries the custody hash of the source exhibit, so what you find is ready for a pleading, not just a browser tab.</p>
          <div className="sp-hero-meta">
            <div><span className="k">Models</span><span className="v">on-prem &middot; open</span></div>
            <div><span className="k">Index</span><span className="v">pgvector &middot; HNSW</span></div>
            <div><span className="k">Languages</span><span className="v">47 &middot; full-text</span></div>
            <div><span className="k">Telemetry</span><span className="v"><em>zero</em></span></div>
          </div>
        </div>
      </section>

      <section className="sp-section">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">In the app<small>A real query</small></div>
          <div className="sp-body">
            <h2>&ldquo;Every mention of the Andriivka checkpoint, <em>in any language.</em>&rdquo;</h2>
            <p className="sp-lead">A single query returns matches across Ukrainian radio intercepts, English investigator notes, Russian Telegram exports, and a French corroboration memo. Each result is ranked by semantic relevance and annotated with its sealed custody status.</p>

            <div className="search-demo">
              <div className="search-bar">Andriivka checkpoint &middot; 17:30&ndash;18:30 local &middot; any language</div>
              <div className="search-chips">
                <span className="schip on">All exhibits</span>
                <span className="schip">Sealed only</span>
                <span className="schip">Corroborated &ge; 2</span>
                <span className="schip">Date 17&ndash;20 Apr 2022</span>
                <span className="schip">Langs: uk &middot; ru &middot; en &middot; fr</span>
                <span className="schip">Exclude redacted</span>
              </div>
              <div className="search-res">
                <div className="res">
                  <span className="score">0.91</span>
                  <div>
                    <div className="meta">E-0412 &middot; radio intercept &middot; uk &middot; 17 Apr 17:41</div>
                    <h5>Patrol call-in, Andriivka junction</h5>
                    <p>&ldquo;&hellip;&#1087;&#1088;&#1080;&#1073;&#1091;&#1083;&#1080; &#1076;&#1086; <mark>&#1073;&#1083;&#1086;&#1082;&#1087;&#1086;&#1089;&#1090;&#1072;</mark> &#1073;&#1110;&#1083;&#1103; <mark>&#1040;&#1085;&#1076;&#1088;&#1110;&#1111;&#1074;&#1082;&#1080;</mark>, &#1076;&#1088;&#1091;&#1075;&#1072; &#1082;&#1086;&#1083;&#1086;&#1085;&#1072; &#1087;&#1088;&#1086;&#1081;&#1096;&#1083;&#1072; &#1093;&#1074;&#1080;&#1083;&#1080;&#1085; &#1087;&rsquo;&#1103;&#1090;&#1085;&#1072;&#1076;&#1094;&#1103;&#1090;&#1100; &#1090;&#1086;&#1084;&#1091;&hellip;&rdquo; <em style={{ color: 'var(--muted-2)' }}>(translated inline)</em></p>
                  </div>
                  <span className="tag">sealed &#9635;</span>
                </div>
                <div className="res">
                  <span className="score">0.87</span>
                  <div>
                    <div className="meta">E-0287 &middot; witness statement &middot; en &middot; W-0144</div>
                    <h5>Witness account &mdash; arrival at checkpoint</h5>
                    <p>&ldquo;The <mark>Andriivka</mark> <mark>checkpoint</mark> was unusually quiet at 17:52. The soldier who waved us through was the same officer I had seen on 14 April&hellip;&rdquo;</p>
                  </div>
                  <span className="tag">sealed &#9635;</span>
                </div>
                <div className="res">
                  <span className="score">0.82</span>
                  <div>
                    <div className="meta">E-0551 &middot; telegram export &middot; ru &middot; 17 Apr 18:02</div>
                    <h5>Channel &ldquo;news_donetsk_live&rdquo; &middot; post #8422</h5>
                    <p>&ldquo;<mark>&#1041;&#1083;&#1086;&#1082;&#1087;&#1086;&#1089;&#1090;</mark> &#1091; &#1040;&#1085;&#1076;&#1088;&#1110;&#1111;&#1074;&#1082;&#1080; &#1089;&#1085;&#1086;&#1074;&#1072; &#1079;&#1072;&#1082;&#1088;&#1099;&#1090;. &#1059;&#1078;&#1077; &#1090;&#1088;&#1077;&#1090;&#1080;&#1081; &#1088;&#1072;&#1079; &#1079;&#1072; &#1089;&#1091;&#1090;&#1082;&#1080;. &#1057;&#1086;&#1089;&#1077;&#1076;&#1080; &#1073;&#1086;&#1103;&#1090;&#1089;&#1103; &#1074;&#1099;&#1093;&#1086;&#1076;&#1080;&#1090;&#1100;&hellip;&rdquo;</p>
                  </div>
                  <span className="tag">sealed &#9635;</span>
                </div>
                <div className="res">
                  <span className="score">0.74</span>
                  <div>
                    <div className="meta">E-0198 &middot; corroboration memo &middot; fr &middot; analyst M. Vasylenko</div>
                    <h5>Recoupement &mdash; trois sources distinctes</h5>
                    <p>&ldquo;Le <mark>point de contr&ocirc;le</mark> d&rsquo;<mark>Andriivka</mark> appara&icirc;t dans E-0287, E-0412, et E-0551 avec un &eacute;cart temporel de moins de 25 minutes&hellip;&rdquo;</p>
                  </div>
                  <span className="tag">sealed &#9635;</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-section">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">The pipeline<small>What runs on your box</small></div>
          <div className="sp-body">
            <h2>Every model runs <em>inside</em> your jurisdiction.</h2>
            <p className="sp-lead">Embedding, transcription, OCR, translation, named-entity extraction &mdash; all ship as containerised models that run on CPU or your own GPU. Nothing crosses your perimeter. The witnesses in your database are not product data for somebody else&rsquo;s frontier lab.</p>

            <div className="sp-rows">
              <div className="sp-row"><span className="idx">01 &middot; STT</span><h4>Whisper-large-v3 <em>on-prem</em></h4><p>Transcription for audio and video. <strong>47 languages</strong>, speaker diarisation, timestamp-anchored captions sealed with the parent exhibit.</p></div>
              <div className="sp-row"><span className="idx">02 &middot; OCR</span><h4>docTR + <em>handwriting</em> adapter</h4><p>Document and handwritten page OCR. Per-character confidence scores preserved; low-confidence ranges are flagged to the analyst rather than silently corrected.</p></div>
              <div className="sp-row"><span className="idx">03 &middot; EMB</span><h4>BGE-M3 <em>multilingual</em></h4><p>Cross-lingual dense embeddings for semantic search. A Ukrainian query matches Russian, English, and French content at the same ranking call.</p></div>
              <div className="sp-row"><span className="idx">04 &middot; NER</span><h4>GLiNER &mdash; <em>custom labels</em></h4><p>Names, places, call-signs, vehicle IDs, weapon types. Operators train per-case label sets without shipping data anywhere &mdash; fine-tunes stay on the box.</p></div>
              <div className="sp-row"><span className="idx">05 &middot; TRANS</span><h4>NLLB-200 &middot; <em>pivot</em> for rare pairs</h4><p>Neural translation, 200 languages. Translations are rendered as annotations &mdash; never silently replace source text in the sealed exhibit.</p></div>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-section dark">
        <div className="wrap sp-grid-12">
          <div className="sp-rail">Provenance<small>Every answer, cited</small></div>
          <div className="sp-body">
            <h2>An answer <em>you can file.</em></h2>
            <p className="sp-lead">Every result carries the sealed exhibit ID, the hash, the chain index, and the custody state. An investigator can export a cross-exhibit finding as a PDF that defence counsel&rsquo;s clerk validator can verify &mdash; without ever touching VaultKeeper&rsquo;s servers.</p>

            <div className="sp-cols3">
              <div className="c"><span className="n">Hash-bound</span><h4>Every hit <em>cites</em> a sealed row</h4><p>Search results are not a separate index. They reference the custody chain&rsquo;s own row IDs. If the chain is broken, the search result is marked broken, too.</p></div>
              <div className="c"><span className="n">Offline</span><h4>Exports verify <em>without</em> us</h4><p>A finding exported from search becomes a ZIP with the cited exhibit slices and a 240-line validator. Defence counsel runs it on their laptop. They don&rsquo;t need VaultKeeper to check your work.</p></div>
              <div className="c"><span className="n">Zero telemetry</span><h4>Queries <em>never</em> leave the perimeter</h4><p>Search logs live in the same sealed ledger as ingest. They are not pushed to VaultKeeper &mdash; we cannot see them, and neither can anyone who gains access to our infrastructure.</p></div>
            </div>
          </div>
        </div>
      </section>

      <section className="sp-quote">
        <div className="wrap">
          <blockquote>&ldquo;A query that took three analysts <em>four days</em> in our old system &mdash; cross-referencing Telegram exports with witness statements in three languages &mdash; took one analyst thirty-one minutes. And the result was sealed, cited, and exportable.&rdquo;</blockquote>
          <p className="who"><strong>Bellingcat</strong> &mdash; Open-source investigations team, Ukraine project</p>
        </div>
      </section>

      <section className="wrap" style={{ padding: '40px 0 32px' }}>
        <div className="cta-banner">
          <div>
            <h2>Bring your <em>hardest</em> query.</h2>
            <p>Send us a redacted version of a query that took your team days in your old tool. We&rsquo;ll run it live on a sample corpus in a 45-minute call.</p>
          </div>
          <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap', justifyContent: 'flex-end' }}>
            <Link className="btn ghost" href="/platform">Platform overview</Link>
            <Link className="btn" href="/contact">Book a demo <span className="arr">&rarr;</span></Link>
          </div>
        </div>
      </section>

      <section className="wrap">
        <div className="sp-nextprev">
          <Link href="/collaboration"><span className="k">Prev &middot; 04/05</span><h5>Live collaboration</h5><p>Forty analysts on one exhibit, every keystroke sealed.</p></Link>
          <Link href="/platform"><span className="k">Platform overview</span><h5>All five pillars</h5><p>Back to the index of the VaultKeeper platform.</p></Link>
        </div>
      </section>
    </>
  );
}
