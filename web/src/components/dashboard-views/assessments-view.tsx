import type { EvidenceAssessment, Recommendation, SourceCredibility } from "@/types";

const recColors: Record<Recommendation, string> = {
  collect: "var(--ok)",
  monitor: "var(--accent)",
  deprioritize: "#b35c5c",
  discard: "#6b3a4a",
};

const recBgs: Record<Recommendation, string> = {
  collect: "rgba(74,107,58,.1)",
  monitor: "rgba(184,66,28,.1)",
  deprioritize: "rgba(179,92,92,.1)",
  discard: "rgba(107,58,74,.1)",
};

const credColors: Record<string, string> = {
  established: "var(--ok)",
  credible: "var(--ok)",
  uncertain: "var(--accent)",
  unconfirmed: "var(--accent)",
  unreliable: "var(--muted)",
  unassessed: "var(--muted)",
};

function credLabel(c: SourceCredibility): string {
  return c.charAt(0).toUpperCase() + c.slice(1);
}

const avatarColors = ["a", "b", "c", "d", "e"];

function avatarColor(id: string): string {
  let hash = 0;
  for (let i = 0; i < id.length; i++) {
    hash = (hash * 31 + id.charCodeAt(i)) | 0;
  }
  return avatarColors[Math.abs(hash) % avatarColors.length];
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString("en-GB", {
    day: "numeric",
    month: "short",
    year: "numeric",
  });
}

function scoreColor(score: number): string {
  if (score >= 7) return "var(--ok)";
  if (score >= 5) return "var(--accent)";
  return "#b35c5c";
}

export interface AssessmentsViewProps {
  assessments: EvidenceAssessment[];
  totalEvidence?: number;
  unassessedCount?: number;
  caseRef?: string;
}

export default function AssessmentsView({
  assessments,
  totalEvidence,
  unassessedCount,
  caseRef,
}: AssessmentsViewProps) {
  const total = assessments.length;
  const collectCount = assessments.filter((a) => a.recommendation === "collect").length;
  const monitorCount = assessments.filter((a) => a.recommendation === "monitor").length;
  const deprioritizeCount = assessments.filter((a) => a.recommendation === "deprioritize").length;
  const discardCount = assessments.filter((a) => a.recommendation === "discard").length;
  const misleadingCount = assessments.filter((a) => a.misleading_indicators.length > 0).length;

  const avgRelevance =
    total > 0
      ? (assessments.reduce((sum, a) => sum + a.relevance_score, 0) / total).toFixed(1)
      : "0";

  const awaiting = unassessedCount ?? 0;
  const evidenceTotal = totalEvidence ?? total;

  const WORKFLOW_STEPS = [
    {
      step: "1",
      title: "Discover",
      desc: "Evidence identified during Phase 1 inquiry. Flagged for assessment.",
      active: false,
    },
    {
      step: "2",
      title: "Assess",
      desc: "Score relevance (1\u201310), reliability (1\u201310), source credibility. Flag misleading indicators.",
      active: true,
    },
    {
      step: "3",
      title: "Recommend",
      desc: "Collect, monitor, deprioritize, or discard. Signed rationale required for each.",
      active: false,
    },
    {
      step: "4",
      title: "Proceed",
      desc: "Approved exhibits advance to Phase 3 (Collection). Discarded items preserved with reasoning.",
      active: false,
    },
  ];

  return (
    <>
      <section className="d-pagehead">
        <div>
          <span className="eyebrow-m">
            {caseRef
              ? `Case \u00b7 ${caseRef} \u00b7 Berkeley Protocol Phase 2`
              : "Berkeley Protocol Phase 2"}
          </span>
          <h1>
            Preliminary <em>assessments</em>
          </h1>
          <p className="sub">
            Phase 2 of the Berkeley Protocol requires evaluating relevance and
            reliability <em>before</em> evidence enters the investigative
            record. Every exhibit must have a signed assessment &mdash; this is
            the gate between discovery and collection.
          </p>
        </div>
        <div className="actions">
          <a className="btn ghost" href="#">
            Unassessed exhibits
          </a>
          <a className="btn" href="#">
            New assessment <span className="arr">&rarr;</span>
          </a>
        </div>
      </section>

      <div className="d-kpis" style={{ marginBottom: 22 }}>
        <div className="d-kpi">
          <div className="k">Assessed &middot; this case</div>
          <div className="v">{total}</div>
          <div className="sub">of {evidenceTotal.toLocaleString()} exhibits</div>
        </div>
        <div className="d-kpi">
          <div className="k">Awaiting assessment</div>
          <div className="v">{awaiting}</div>
          <div className="delta n">&#9679; uploaded today</div>
        </div>
        <div className="d-kpi">
          <div className="k">Avg. relevance</div>
          <div className="v">{avgRelevance}</div>
          <div className="sub">across assessed</div>
        </div>
        <div className="d-kpi">
          <div className="k">Deprioritized / discarded</div>
          <div className="v">{deprioritizeCount + discardCount}</div>
          <div className="sub">transparent &middot; preserved</div>
        </div>
      </div>

      {awaiting > 0 && (
        <div
          style={{
            padding: "16px 20px",
            border: "1px solid rgba(184,66,28,.2)",
            borderRadius: 12,
            background: "rgba(184,66,28,.03)",
            marginBottom: 22,
            display: "flex",
            alignItems: "center",
            gap: 14,
          }}
        >
          <span style={{ fontSize: 20 }}>{"\u26A0"}</span>
          <div>
            <div
              style={{ fontSize: 14, fontWeight: 500, color: "var(--ink)" }}
            >
              {awaiting} exhibits have no preliminary assessment
            </div>
            <div
              style={{
                fontSize: 13,
                color: "var(--muted)",
                marginTop: 2,
              }}
            >
              The Berkeley Protocol requires assessment before an exhibit
              enters the investigative record.{" "}
              <a href="#" style={{ color: "var(--accent)" }}>
                Review unassessed queue &rarr;
              </a>
            </div>
          </div>
        </div>
      )}

      <div className="panel">
        <div className="fbar">
          <div className="fsearch">
            <svg
              width="14"
              height="14"
              viewBox="0 0 16 16"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.5"
              style={{ color: "var(--muted)" }}
            >
              <circle cx="7" cy="7" r="4" />
              <path d="M10 10l3 3" />
            </svg>
            <input placeholder="exhibit, assessor, recommendation&hellip;" />
          </div>
          <span className="chip active">All {total}</span>
          <span className="chip">Collect {collectCount}</span>
          <span className="chip">Monitor {monitorCount}</span>
          <span className="chip">Deprioritize {deprioritizeCount}</span>
          <span className="chip">Discard {discardCount}</span>
          <span className="chip">Misleading flagged {misleadingCount}</span>
        </div>
        <div className="panel-body" style={{ padding: 0 }}>
          {assessments.length === 0 && (
            <div
              style={{
                padding: "48px 28px",
                textAlign: "center",
                color: "var(--muted)",
              }}
            >
              No assessments yet.
            </div>
          )}
          {assessments.map((a) => {
            const relColor = scoreColor(a.relevance_score);
            const rlbColor = scoreColor(a.reliability_score);
            const authorInitial = a.assessed_by
              ? a.assessed_by[0].toUpperCase()
              : "?";
            const authorDisplay = a.assessed_by ?? "\u2014";
            const avClass = avatarColor(a.assessed_by);
            const rationale =
              a.relevance_rationale.length > 200
                ? a.relevance_rationale.slice(0, 200) + "\u2026"
                : a.relevance_rationale;

            return (
              <div
                key={a.id}
                style={{
                  padding: "22px 28px",
                  borderBottom: "1px solid var(--line)",
                  display: "grid",
                  gridTemplateColumns: "80px 1fr 200px 120px",
                  gap: 24,
                  alignItems: "start",
                  transition: "background .12s",
                  cursor: "pointer",
                }}
              >
                {/* Scores */}
                <div style={{ textAlign: "center" }}>
                  <div
                    style={{
                      display: "flex",
                      gap: 8,
                      justifyContent: "center",
                    }}
                  >
                    <div>
                      <div
                        style={{
                          fontFamily: "'Fraunces', serif",
                          fontSize: 28,
                          letterSpacing: "-.02em",
                          color: relColor,
                          lineHeight: 1,
                        }}
                      >
                        {a.relevance_score}
                      </div>
                      <div
                        style={{
                          fontFamily: "'JetBrains Mono', monospace",
                          fontSize: 8.5,
                          letterSpacing: ".08em",
                          textTransform: "uppercase",
                          color: "var(--muted)",
                          marginTop: 3,
                        }}
                      >
                        REL
                      </div>
                    </div>
                    <div>
                      <div
                        style={{
                          fontFamily: "'Fraunces', serif",
                          fontSize: 28,
                          letterSpacing: "-.02em",
                          color: rlbColor,
                          lineHeight: 1,
                        }}
                      >
                        {a.reliability_score}
                      </div>
                      <div
                        style={{
                          fontFamily: "'JetBrains Mono', monospace",
                          fontSize: 8.5,
                          letterSpacing: ".08em",
                          textTransform: "uppercase",
                          color: "var(--muted)",
                          marginTop: 3,
                        }}
                      >
                        RLB
                      </div>
                    </div>
                  </div>
                </div>

                {/* Details */}
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
                      <strong>{a.id.slice(0, 8).toUpperCase()}</strong>
                    </span>
                    <span
                      style={{
                        fontFamily: "'JetBrains Mono', monospace",
                        fontSize: 11,
                        color: "var(--accent)",
                        letterSpacing: ".02em",
                      }}
                    >
                      {a.evidence_id.slice(0, 8).toUpperCase()}
                    </span>
                    <span
                      style={{
                        padding: "2px 8px",
                        borderRadius: 999,
                        fontSize: 11,
                        fontWeight: 500,
                        color: recColors[a.recommendation],
                        background: recBgs[a.recommendation],
                        textTransform: "capitalize",
                      }}
                    >
                      {a.recommendation}
                    </span>
                    <span
                      style={{
                        padding: "2px 8px",
                        borderRadius: 999,
                        fontSize: 11,
                        color:
                          credColors[a.source_credibility] ?? "var(--muted)",
                        border: "1px solid currentColor",
                        opacity: 0.7,
                      }}
                    >
                      {credLabel(a.source_credibility)}
                    </span>
                  </div>
                  <div
                    style={{
                      fontSize: 13,
                      color: "var(--muted)",
                      lineHeight: 1.55,
                      maxWidth: "60ch",
                    }}
                  >
                    {rationale}
                  </div>
                  {a.misleading_indicators.length > 0 && (
                    <div
                      style={{
                        marginTop: 8,
                        padding: "8px 12px",
                        borderRadius: 8,
                        background: "rgba(184,66,28,.04)",
                        border: "1px solid rgba(184,66,28,.12)",
                      }}
                    >
                      <div
                        style={{
                          fontFamily: "'JetBrains Mono', monospace",
                          fontSize: 9,
                          letterSpacing: ".08em",
                          textTransform: "uppercase",
                          color: "var(--accent)",
                          marginBottom: 4,
                        }}
                      >
                        Misleading indicators
                      </div>
                      {a.misleading_indicators.map((m) => (
                        <div
                          key={m}
                          style={{
                            fontSize: 12,
                            color: "var(--ink-2)",
                            lineHeight: 1.5,
                          }}
                        >
                          &middot; {m}
                        </div>
                      ))}
                    </div>
                  )}
                </div>

                {/* Author + date */}
                <div
                  style={{
                    display: "flex",
                    flexDirection: "column",
                    gap: 8,
                    alignItems: "flex-end",
                    textAlign: "right",
                  }}
                >
                  <div
                    style={{
                      display: "flex",
                      alignItems: "center",
                      gap: 8,
                    }}
                  >
                    <span className="avs">
                      <span className={`av ${avClass}`}>
                        {authorInitial}
                      </span>
                    </span>
                    <span
                      style={{
                        fontFamily: "'Fraunces', serif",
                        fontSize: 14,
                        color: "var(--ink)",
                        letterSpacing: "-.005em",
                      }}
                    >
                      {authorDisplay}
                    </span>
                  </div>
                  <div
                    style={{
                      fontFamily: "'JetBrains Mono', monospace",
                      fontSize: 10.5,
                      color: "var(--muted)",
                      letterSpacing: ".02em",
                    }}
                  >
                    {formatDate(a.created_at)}
                  </div>
                </div>

                {/* Actions */}
                <div
                  style={{
                    display: "flex",
                    flexDirection: "column",
                    alignItems: "flex-end",
                    gap: 8,
                  }}
                >
                  <a
                    className="linkarrow"
                    href="#"
                    style={{ fontSize: 12.5 }}
                  >
                    View exhibit &rarr;
                  </a>
                  <a
                    className="linkarrow"
                    href="#"
                    style={{ fontSize: 12.5 }}
                  >
                    Edit &rarr;
                  </a>
                </div>
              </div>
            );
          })}
        </div>
      </div>

      {/* Workflow panel */}
      <div className="panel" style={{ marginTop: 22 }}>
        <div className="panel-h">
          <h3>
            Assessment <em>workflow</em>
          </h3>
          <span className="meta">Berkeley Protocol Phase 2</span>
        </div>
        <div className="panel-body">
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "repeat(4, 1fr)",
              gap: 0,
            }}
          >
            {WORKFLOW_STEPS.map((s, i) => (
              <div
                key={s.step}
                style={{
                  padding: 20,
                  ...(i < WORKFLOW_STEPS.length - 1
                    ? { borderRight: "1px solid var(--line)" }
                    : {}),
                }}
              >
                <div
                  style={{
                    display: "flex",
                    alignItems: "center",
                    gap: 8,
                    marginBottom: 10,
                  }}
                >
                  <span
                    style={{
                      width: 28,
                      height: 28,
                      borderRadius: "50%",
                      background: s.active ? "var(--accent)" : "var(--bg-2)",
                      color: s.active ? "#fff" : "var(--muted)",
                      display: "grid",
                      placeItems: "center",
                      fontFamily: "'JetBrains Mono', monospace",
                      fontSize: 12,
                      fontWeight: 600,
                    }}
                  >
                    {s.step}
                  </span>
                  <span
                    style={{
                      fontFamily: "'Fraunces', serif",
                      fontSize: 17,
                      letterSpacing: "-.01em",
                    }}
                  >
                    {s.title}
                  </span>
                </div>
                <div
                  style={{
                    fontSize: 13,
                    color: "var(--muted)",
                    lineHeight: 1.55,
                  }}
                >
                  {s.desc}
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </>
  );
}
