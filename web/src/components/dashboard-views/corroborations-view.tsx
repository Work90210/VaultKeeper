import type { CorroborationClaim, ClaimStrength, RoleInClaim } from "@/types";

const strengthToPill: Record<ClaimStrength, string> = {
  strong: "sealed",
  moderate: "hold",
  weak: "disc",
  contested: "broken",
};

const roleToPill: Record<RoleInClaim, string> = {
  primary: "a",
  supporting: "c",
  contextual: "d",
  contradicting: "b",
};

function strengthToScore(s: ClaimStrength): number {
  switch (s) {
    case "strong":
      return 0.9;
    case "moderate":
      return 0.65;
    case "weak":
      return 0.35;
    case "contested":
      return 0.15;
  }
}

function scoreColor(sc: number): string {
  if (sc >= 0.75) return "var(--ok)";
  if (sc >= 0.5) return "var(--accent)";
  return "var(--muted)";
}

function barColor(sc: number): string {
  if (sc >= 0.75) return "var(--ok)";
  if (sc >= 0.5) return "var(--accent)";
  return "#b35c5c";
}

function median(values: number[]): number {
  if (values.length === 0) return 0;
  const sorted = [...values].sort((a, b) => a - b);
  const mid = Math.floor(sorted.length / 2);
  return sorted.length % 2 !== 0
    ? sorted[mid]
    : (sorted[mid - 1] + sorted[mid]) / 2;
}

export interface CorroborationsViewProps {
  claims: CorroborationClaim[];
}

export default function CorroborationsView({
  claims,
}: CorroborationsViewProps) {
  const total = claims.length;
  const strong = claims.filter((c) => c.strength === "strong").length;
  const moderate = claims.filter((c) => c.strength === "moderate").length;
  const weak = claims.filter((c) => c.strength === "weak").length;
  const contested = claims.filter((c) => c.strength === "contested").length;

  const scores = claims.map((c) => strengthToScore(c.strength));
  const medianScore = median(scores).toFixed(2);

  const singleSource = claims.filter(
    (c) => c.evidence.length <= 1
  ).length;

  // Sort by score descending
  const sorted = [...claims].sort(
    (a, b) => strengthToScore(b.strength) - strengthToScore(a.strength)
  );

  return (
    <>
      <section className="d-pagehead">
        <div>
          <span className="eyebrow-m">
            Berkeley Protocol Phase 5
          </span>
          <h1>
            Multi-source <em>corroboration</em>
          </h1>
          <p className="sub">
            Phase 5 of the Berkeley Protocol requires source authentication,
            content verification, and multi-source corroboration. Every factual
            claim carries a score computed from independent sources, each
            weighted by evidence kind and custody strength. Single-source claims
            are shown, never hidden &mdash; that&apos;s the point.
          </p>
        </div>
        <div className="actions">
          <a className="btn ghost" href="#">
            Matrix view
          </a>
          <a className="btn" href="#">
            New claim <span className="arr">&rarr;</span>
          </a>
        </div>
      </section>

      <div className="d-kpis" style={{ marginBottom: 22 }}>
        <div className="d-kpi">
          <div className="k">Claims tracked</div>
          <div className="v">{total}</div>
          <div className="sub">
            {strong} corroborated &middot; {moderate} investigating &middot; {contested} refuted
          </div>
        </div>
        <div className="d-kpi">
          <div className="k">Median score</div>
          <div className="v">{medianScore}</div>
          <div className="sub">across corroborated</div>
        </div>
        <div className="d-kpi">
          <div className="k">Single-source &middot; flagged</div>
          <div className="v">{singleSource}</div>
          <div className="delta n">&#9679; preserved &amp; disclosed</div>
        </div>
        <div className="d-kpi">
          <div className="k">Cross-case links</div>
          <div className="v">
            {claims.reduce((sum, c) => sum + c.evidence.length, 0)}
          </div>
          <div className="sub">to CIJA Berlin sub-chain</div>
        </div>
      </div>

      <div className="panel">
        <div className="panel-h">
          <h3>Claims</h3>
          <span className="meta">sorted by score</span>
        </div>
        <div className="panel-body" style={{ padding: 0 }}>
          {sorted.length === 0 && (
            <div
              style={{
                padding: "48px 28px",
                textAlign: "center",
                color: "var(--muted)",
              }}
            >
              No corroboration claims yet.
            </div>
          )}
          {sorted.map((c) => {
            const sc = strengthToScore(c.strength);
            return (
              <div
                key={c.id}
                style={{
                  padding: "20px 28px",
                  borderBottom: "1px solid var(--line)",
                  display: "grid",
                  gridTemplateColumns: "90px 1fr 220px 140px",
                  gap: 28,
                  alignItems: "center",
                }}
              >
                <div style={{ textAlign: "center" }}>
                  <div
                    style={{
                      fontFamily: "'Fraunces', serif",
                      fontSize: 36,
                      letterSpacing: "-.03em",
                      color: scoreColor(sc),
                      lineHeight: 1,
                    }}
                  >
                    {sc.toFixed(2)}
                  </div>
                  <div
                    style={{
                      fontFamily: "'JetBrains Mono', monospace",
                      fontSize: 10,
                      color: "var(--muted)",
                      letterSpacing: ".06em",
                      textTransform: "uppercase",
                      marginTop: 4,
                    }}
                  >
                    score
                  </div>
                </div>
                <div>
                  <div
                    style={{
                      display: "flex",
                      alignItems: "center",
                      gap: 10,
                      marginBottom: 6,
                    }}
                  >
                    <span className="ref" style={{ display: "block" }}>
                      <strong>{c.id.slice(0, 8).toUpperCase()}</strong>
                    </span>
                    <span className={`pl ${strengthToPill[c.strength]}`}>
                      {c.strength}
                    </span>
                    <span className="tag">{c.claim_type.replace(/_/g, " ")}</span>
                  </div>
                  <div
                    style={{
                      fontFamily: "'Fraunces', serif",
                      fontSize: 18,
                      letterSpacing: "-.01em",
                      lineHeight: 1.3,
                    }}
                  >
                    {c.claim_summary || "\u2014"}
                  </div>
                </div>
                <div
                  style={{ display: "flex", flexDirection: "column", gap: 5 }}
                >
                  <div
                    style={{
                      fontFamily: "'JetBrains Mono', monospace",
                      fontSize: 10,
                      letterSpacing: ".08em",
                      textTransform: "uppercase",
                      color: "var(--muted)",
                      marginBottom: 3,
                    }}
                  >
                    sources &middot; {c.evidence.length}
                  </div>
                  {c.evidence.map((e) => (
                    <div
                      key={e.id}
                      style={{
                        display: "flex",
                        alignItems: "center",
                        gap: 8,
                        fontSize: 12.5,
                      }}
                    >
                      <span
                        className={`av ${roleToPill[e.role_in_claim] ?? "d"}`}
                        style={{
                          width: 20,
                          height: 20,
                          fontSize: 9,
                          borderWidth: 1,
                        }}
                      >
                        {e.role_in_claim[0].toUpperCase()}
                      </span>
                      <span style={{ color: "var(--ink-2)" }}>
                        {e.contribution_notes ||
                          e.evidence_id.slice(0, 8).toUpperCase()}
                      </span>
                    </div>
                  ))}
                  {c.evidence.length === 0 && (
                    <div style={{ fontSize: 12.5, color: "var(--muted)" }}>
                      &mdash;
                    </div>
                  )}
                </div>
                <div
                  style={{
                    display: "flex",
                    flexDirection: "column",
                    alignItems: "flex-end",
                    gap: 6,
                  }}
                >
                  <div
                    style={{
                      width: "100%",
                      height: 6,
                      background: "var(--bg-2)",
                      borderRadius: 3,
                      overflow: "hidden",
                    }}
                  >
                    <div
                      style={{
                        height: "100%",
                        width: `${sc * 100}%`,
                        background: barColor(sc),
                      }}
                    />
                  </div>
                  <a
                    className="linkarrow"
                    href="#"
                    style={{ fontSize: 12.5 }}
                  >
                    Open claim &rarr;
                  </a>
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </>
  );
}
