export default function RedactionView() {
  const marks = [
    { t: "Andriivka → geo-fuzz 50 km", who: "Martyna", c: "b", sig: "d9a7", k: "geo" },
    { t: "17:40 → 17:52", who: "Amir H.", c: "c", sig: "2c14", k: "edit" },
    { t: "Col. M. → S-038", who: "Juliane", c: "d", sig: "a1be", k: "pseudo" },
    { t: "Strike — operational source ref", who: "W. Nyoka", c: "e", sig: "e3f0", k: "redact" },
    { t: "Strike — ongoing enquiry", who: "H. Morel", c: "a", sig: "7f22", k: "redact" },
    { t: "Strike — witness-identifying", who: "W. Nyoka", c: "e", sig: "e3f1", k: "redact" },
  ];

  return (
    <>
      <section className="d-pagehead">
        <div>
          <span className="eyebrow-m">Berkeley Protocol Reporting &middot; E-0912 &middot; W-0144 sworn statement v3 &middot; draft v4</span>
          <h1>Collaborative <em>redaction</em></h1>
          <p className="sub">Four analysts in one document. Every mark, every pseudonym swap, every gloss is a signed CRDT op written to the same chain as ingest. Defence replay works keystroke-by-keystroke.</p>
        </div>
        <div className="actions">
          <span className="avs" style={{ marginRight: 8 }}>
            <span className="av a">H</span>
            <span className="av b">M</span>
            <span className="av c">A</span>
            <span className="av d">J</span>
          </span>
          <a className="btn ghost" href="#">Version history</a>
          <a className="btn" href="#">Seal draft v4 <span className="arr">&rarr;</span></a>
        </div>
      </section>

      <div className="panel" style={{ marginBottom: 22 }}>
        <div className="fbar">
          <span className="chip active">Mark redaction</span>
          <span className="chip">Pseudonymise</span>
          <span className="chip">Translate gloss</span>
          <span className="chip">Add note</span>
          <span className="chip">Geo-fuzz</span>
          <span style={{ marginLeft: "auto", fontFamily: "'JetBrains Mono', monospace", fontSize: "11.5px", color: "var(--muted)" }}>
            Page <strong style={{ color: "var(--ink)", fontWeight: 500 }}>2</strong> of 6 · 12 marks in draft
          </span>
          <span className="chip">&#9666; Prev</span>
          <span className="chip">Next &#9656;</span>
        </div>
        <div className="panel-body" style={{ padding: 30, background: "var(--bg-2)" }}>
          <div style={{ display: "grid", gridTemplateColumns: "1fr 300px", gap: 24, alignItems: "start" }}>
            <div className="doc-paper">
              <span className="pn">P. 02 / 06</span>
              <p>The statement was recorded at the regional intake office on <mark>19 April 2026 at approximately 14:21 local time</mark>, in the presence of two intermediaries.</p>
              <p>The witness, hereafter <mark className="pseudo">W-0144</mark>, recalls arriving at the checkpoint near <mark>Andriivka</mark> at <span style={{ textDecoration: "line-through", color: "var(--muted)" }}>17:40</span> <strong>17:52</strong> local time, after the second convoy had passed.</p>
              <p>She identified the officer in command as <mark className="pseudo">subject S-038</mark>, <span className="redact">&nbsp;redacted — operational source reference&nbsp;</span>, who she recognised from prior encounters at the administrative building.</p>
              <p>The witness further reported that the officer referenced <mark>&quot;Unit 28B&quot;</mark> and instructed subordinates to <span className="redact">&nbsp;redacted — ongoing enquiry&nbsp;</span>.</p>
              <p>The statement was given voluntarily, in the presence of two intermediaries, one of whom is <span className="redact">&nbsp;redacted — witness-identifying&nbsp;</span> unrelated to other proceedings.</p>
            </div>
            <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
              <div className="panel" style={{ margin: 0 }}>
                <div className="panel-h" style={{ padding: "12px 14px" }}>
                  <h3 style={{ fontSize: 15 }}>Marks · this page</h3>
                  <span className="meta">12</span>
                </div>
                <div className="panel-body" style={{ padding: "4px 0" }}>
                  {marks.map((m) => (
                    <div
                      key={m.sig}
                      style={{
                        display: "grid",
                        gridTemplateColumns: "22px 1fr auto",
                        gap: 10,
                        alignItems: "start",
                        padding: "10px 14px",
                        borderBottom: "1px solid var(--line)",
                        fontSize: "12.5px",
                      }}
                    >
                      <span className="avs" style={{ marginTop: 1 }}>
                        <span
                          className={`av ${m.c}`}
                          style={{ width: 22, height: 22, fontSize: 10, borderWidth: "1.5px" }}
                        >
                          {m.who[0]}
                        </span>
                      </span>
                      <div>
                        <div style={{ color: "var(--ink-2)" }}>{m.t}</div>
                        <div
                          style={{
                            color: "var(--muted)",
                            fontFamily: "'JetBrains Mono', monospace",
                            fontSize: "10.5px",
                            marginTop: 2,
                          }}
                        >
                          {m.who} &middot; sig {m.sig}&hellip;
                        </div>
                      </div>
                      <span className="tag">{m.k}</span>
                    </div>
                  ))}
                </div>
              </div>

              <div className="panel" style={{ margin: 0 }}>
                <div className="panel-h" style={{ padding: "12px 14px" }}>
                  <h3 style={{ fontSize: 15 }}>Presence</h3>
                  <span className="meta">4 live</span>
                </div>
                <div
                  className="panel-body"
                  style={{
                    padding: 14,
                    display: "flex",
                    flexDirection: "column",
                    gap: 8,
                    fontSize: "12.5px",
                  }}
                >
                  <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                    <span className="avs"><span className="av a">H</span></span>
                    <span>H. Morel <span style={{ color: "var(--muted)", fontFamily: "'JetBrains Mono', monospace", fontSize: 10 }}>· page 2</span></span>
                  </div>
                  <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                    <span className="avs"><span className="av b">M</span></span>
                    <span>Martyna <span style={{ color: "var(--muted)", fontFamily: "'JetBrains Mono', monospace", fontSize: 10 }}>· page 3 · typing</span></span>
                  </div>
                  <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                    <span className="avs"><span className="av c">A</span></span>
                    <span>Amir H. <span style={{ color: "var(--muted)", fontFamily: "'JetBrains Mono', monospace", fontSize: 10 }}>· page 4</span></span>
                  </div>
                  <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                    <span className="avs"><span className="av d">J</span></span>
                    <span>Juliane <span style={{ color: "var(--muted)", fontFamily: "'JetBrains Mono', monospace", fontSize: 10 }}>· idle 2m</span></span>
                  </div>
                  <div
                    style={{
                      fontFamily: "'JetBrains Mono', monospace",
                      fontSize: "10.5px",
                      color: "var(--muted)",
                      borderTop: "1px solid var(--line)",
                      paddingTop: 8,
                      marginTop: 4,
                      letterSpacing: ".04em",
                      textTransform: "uppercase",
                    }}
                  >
                    head · f208…bc91 · 142 ms p95
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </>
  );
}
