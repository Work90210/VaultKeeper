import type { AnalysisNote, AnalysisType } from "@/types";

const statusPillClass: Record<string, string> = {
  approved: "sealed",
  in_review: "disc",
  draft: "draft",
  superseded: "broken",
};

const statusLabel: Record<string, string> = {
  approved: "signed",
  in_review: "peer-review",
  draft: "draft",
  superseded: "superseded",
};

const analysisTypeTag: Record<AnalysisType, string> = {
  factual_finding: "factual",
  pattern_analysis: "pattern",
  timeline_reconstruction: "timeline",
  geographic_analysis: "geographic",
  network_analysis: "network",
  legal_assessment: "legal",
  credibility_assessment: "credibility",
  gap_identification: "gap",
  hypothesis_testing: "hypothesis",
  other: "other",
};

const avatarColors = ["a", "b", "c", "d", "e"];

function timeAgo(dateStr: string): string {
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  const diffDays = Math.floor((now - then) / (1000 * 60 * 60 * 24));
  if (diffDays === 0) return "today";
  if (diffDays === 1) return "1 d";
  if (diffDays < 30) return `${diffDays} d`;
  if (diffDays < 365) return `${Math.floor(diffDays / 30)} mo`;
  return `${Math.floor(diffDays / 365)} y`;
}

function avatarColor(id: string): string {
  let hash = 0;
  for (let i = 0; i < id.length; i++) {
    hash = (hash * 31 + id.charCodeAt(i)) | 0;
  }
  return avatarColors[Math.abs(hash) % avatarColors.length];
}

function citationCount(n: AnalysisNote): number {
  return (
    n.related_evidence_ids.length +
    n.related_inquiry_ids.length +
    n.related_assessment_ids.length +
    n.related_verification_ids.length
  );
}

export interface AnalysisViewProps {
  notes: AnalysisNote[];
}

export default function AnalysisView({ notes }: AnalysisViewProps) {
  const total = notes.length;
  const approved = notes.filter((n) => n.status === "approved").length;
  const inReview = notes.filter((n) => n.status === "in_review").length;
  const draft = notes.filter((n) => n.status === "draft").length;

  const avgCitations =
    total > 0
      ? (notes.reduce((sum, n) => sum + citationCount(n), 0) / total).toFixed(1)
      : "0";

  const hypothesisCount = notes.filter(
    (n) => n.analysis_type === "hypothesis_testing"
  ).length;

  return (
    <>
      <section className="d-pagehead">
        <div>
          <span className="eyebrow-m">
            Berkeley Protocol Phase 6
          </span>
          <h1>
            Analysis <em>notes</em>
          </h1>
          <p className="sub">
            Phase 6 of the Berkeley Protocol requires documented analytical
            reasoning with iterative refinement. Every note is a CRDT document
            tied to the exhibits it cites. Hypotheses, dead-ends,
            counter-evidence — all sealed into the chain so defence counsel sees
            the reasoning, not just the conclusion.
          </p>
        </div>
        <div className="actions">
          <a className="btn ghost" href="#">
            Templates
          </a>
          <a className="btn" href="#">
            New note <span className="arr">&rarr;</span>
          </a>
        </div>
      </section>

      <div className="d-kpis" style={{ marginBottom: 22 }}>
        <div className="d-kpi">
          <div className="k">Notes &middot; this case</div>
          <div className="v">{total}</div>
          <div className="sub">
            {approved} signed &middot; {inReview} peer-review &middot; {draft}{" "}
            draft
          </div>
        </div>
        <div className="d-kpi">
          <div className="k">Uncorroborated &middot; open</div>
          <div className="v">{draft}</div>
          <div className="delta n">&#9679; flagged to prosecution</div>
        </div>
        <div className="d-kpi">
          <div className="k">Counter-evidence &middot; preserved</div>
          <div className="v">
            {notes.filter((n) => n.status === "superseded").length || hypothesisCount}
          </div>
          <div className="sub">per Rule 77 disclosure</div>
        </div>
        <div className="d-kpi">
          <div className="k">Avg. citations</div>
          <div className="v">{avgCitations}</div>
          <div className="sub">exhibits per note</div>
        </div>
      </div>

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
            <input placeholder="reference, tag, author, linked exhibit…" />
          </div>
          <span className="chip active">All {total}</span>
          <span className="chip">Signed {approved}</span>
          <span className="chip">Peer-review {inReview}</span>
          <span className="chip">Draft {draft}</span>
          <span className="chip">Hypothesis &#9662;</span>
          <span className="chip">Author &#9662;</span>
        </div>
        <div className="panel-body" style={{ padding: 0 }}>
          {notes.length === 0 && (
            <div
              style={{
                padding: "48px 28px",
                textAlign: "center",
                color: "var(--muted)",
              }}
            >
              No analysis notes yet.
            </div>
          )}
          {notes.map((n) => {
            const _refs = citationCount(n);
            const evidenceRefs = n.related_evidence_ids.length;
            const linksLabel =
              evidenceRefs > 0
                ? `${evidenceRefs} exhibit${evidenceRefs !== 1 ? "s" : ""}`
                : "\u2014";
            const displayStatus = statusLabel[n.status] ?? n.status;
            const pillClass = statusPillClass[n.status] ?? "draft";
            const tag = analysisTypeTag[n.analysis_type] ?? n.analysis_type;
            const authorInitial = n.author_id ? n.author_id[0].toUpperCase() : "?";
            const authorDisplay = n.author_id ?? "\u2014";
            const excerpt =
              n.content.length > 180
                ? n.content.slice(0, 180) + "\u2026"
                : n.content;

            return (
              <div
                key={n.id}
                style={{
                  padding: "22px 28px",
                  borderBottom: "1px solid var(--line)",
                  display: "grid",
                  gridTemplateColumns: "1fr 220px",
                  gap: 32,
                  alignItems: "start",
                }}
              >
                <div>
                  <div
                    style={{
                      display: "flex",
                      alignItems: "center",
                      gap: 10,
                      marginBottom: 10,
                    }}
                  >
                    <span className="ref" style={{ display: "block" }}>
                      <strong>{n.id.slice(0, 8).toUpperCase()}</strong>
                    </span>
                    <span className={`pl ${pillClass}`}>{displayStatus}</span>
                    <span className="tag">{tag}</span>
                    {n.status === "draft" && (
                      <span className="tag">draft</span>
                    )}
                  </div>
                  <a
                    href="#"
                    style={{
                      fontFamily: "'Fraunces', serif",
                      fontSize: 22,
                      letterSpacing: "-.015em",
                      color: "var(--ink)",
                      lineHeight: 1.2,
                      display: "block",
                    }}
                  >
                    {n.title || "\u2014"}
                  </a>
                  <p
                    style={{
                      color: "var(--muted)",
                      fontSize: 13.5,
                      lineHeight: 1.6,
                      marginTop: 8,
                      maxWidth: "68ch",
                    }}
                  >
                    {excerpt || "\u2014"}
                  </p>
                </div>
                <div
                  style={{
                    display: "flex",
                    flexDirection: "column",
                    gap: 12,
                    alignItems: "flex-end",
                    fontSize: 12.5,
                    color: "var(--muted)",
                    textAlign: "right",
                  }}
                >
                  <div
                    style={{ display: "flex", alignItems: "center", gap: 8 }}
                  >
                    <span className="avs">
                      <span className={`av ${avatarColor(n.author_id)}`}>
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
                      letterSpacing: ".04em",
                      textTransform: "uppercase",
                    }}
                  >
                    {linksLabel}
                  </div>
                  <div
                    style={{
                      fontFamily: "'JetBrains Mono', monospace",
                      fontSize: 10.5,
                    }}
                  >
                    {timeAgo(n.created_at)} ago
                  </div>
                  <a
                    className="linkarrow"
                    href="#"
                    style={{ fontSize: 12.5 }}
                  >
                    Open &rarr;
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
