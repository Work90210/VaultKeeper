import type { InquiryLog } from "@/types";

function formatTime(dateStr: string): string {
  const d = new Date(dateStr);
  return d.toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit" });
}

function formatDay(dateStr: string): string {
  const d = new Date(dateStr);
  return d.toLocaleDateString("en-GB", {
    day: "numeric",
    month: "short",
    year: "numeric",
  });
}

const KIND_MAP: Record<string, { label: string; cls: string }> = {
  decision: { label: "Decision", cls: "sealed" },
  question: { label: "Open question", cls: "disc" },
  action: { label: "Action", cls: "hold" },
  request: { label: "External request", cls: "draft" },
  federation: { label: "Federation", cls: "live" },
};

function inferKind(log: InquiryLog): string {
  const text = (
    log.objective +
    " " +
    (log.notes ?? "") +
    " " +
    log.search_strategy
  ).toLowerCase();
  if (
    text.includes("decision") ||
    text.includes("recommend") ||
    text.includes("accept")
  )
    return "decision";
  if (
    text.includes("question") ||
    text.includes("confirmation") ||
    text.includes("request review")
  )
    return "question";
  if (
    text.includes("request") ||
    text.includes("formal") ||
    text.includes("liaison")
  )
    return "request";
  if (
    text.includes("federation") ||
    text.includes("federated") ||
    text.includes("mirror")
  )
    return "federation";
  return "action";
}

export interface InquiryViewProps {
  logs: InquiryLog[];
  caseRef?: string;
}

export default function InquiryView({ logs, caseRef }: InquiryViewProps) {
  const total = logs.length;
  const decisions = logs.filter((l) => inferKind(l) === "decision").length;
  const questions = logs.filter((l) => inferKind(l) === "question").length;
  const actions = logs.filter((l) => inferKind(l) === "action").length;
  const requests = logs.filter((l) => inferKind(l) === "request").length;
  const federated = logs.filter((l) => inferKind(l) === "federation").length;

  // Sort by most recent first
  const sorted = [...logs].sort(
    (a, b) =>
      new Date(b.search_started_at).getTime() -
      new Date(a.search_started_at).getTime()
  );

  // Group by day
  const byDay: Record<string, InquiryLog[]> = {};
  for (const log of sorted) {
    const day = formatDay(log.search_started_at);
    if (!byDay[day]) byDay[day] = [];
    byDay[day].push(log);
  }

  const AVATAR_CLASSES = ["a", "b", "c", "d", "e"];

  return (
    <>
      <section className="d-pagehead">
        <div>
          <span className="eyebrow-m">
            {caseRef
              ? `Case \u00b7 ${caseRef} \u00b7 Berkeley Protocol Phase 1`
              : "Berkeley Protocol Phase 1"}
          </span>
          <h1>
            Inquiry <em>log</em>
          </h1>
          <p className="sub">
            Phase 1 of the Berkeley Protocol requires documenting all search
            strategies, tools, and discovery timelines. Each entry is a signed
            record on the same chain as evidence &mdash; this is how we show{" "}
            <em>how</em> the case was built, not just <em>what</em> it found.
          </p>
        </div>
        <div className="actions">
          <a className="btn ghost" href="#">
            Export to PDF
          </a>
          <a className="btn" href="#">
            New entry <span className="arr">&rarr;</span>
          </a>
        </div>
      </section>

      <div className="panel" style={{ marginBottom: 22 }}>
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
            <input
              placeholder="decision, linked exhibit, keyword&hellip;"
            />
          </div>
          <span className="chip active">All {total}</span>
          <span className="chip">Decisions {decisions}</span>
          <span className="chip">Open questions {questions}</span>
          <span className="chip">Actions {actions}</span>
          <span className="chip">External requests {requests}</span>
          <span className="chip">Federation {federated}</span>
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
              No inquiry actions logged yet.
            </div>
          )}
          {Object.entries(byDay).map(([day, items]) => (
            <div key={day}>
              <div
                style={{
                  padding: "14px 28px",
                  background: "var(--bg-2)",
                  borderBottom: "1px solid var(--line)",
                  fontFamily: "'JetBrains Mono', monospace",
                  fontSize: "11px",
                  letterSpacing: ".08em",
                  textTransform: "uppercase",
                  color: "var(--muted)",
                }}
              >
                {day}
              </div>
              {items.map((log, idx) => {
                const kind = inferKind(log);
                const km = KIND_MAP[kind] ?? KIND_MAP.action;
                const avClass = AVATAR_CLASSES[idx % AVATAR_CLASSES.length];
                const initial = (log.performed_by || "?")[0].toUpperCase();
                const linked = [
                  log.evidence_id
                    ? `E-${log.evidence_id.slice(0, 4)}`
                    : null,
                  log.search_tool || null,
                ]
                  .filter(Boolean)
                  .join(" \u00b7 ");

                return (
                  <div
                    key={log.id}
                    style={{
                      padding: "22px 28px",
                      borderBottom: "1px solid var(--line)",
                      display: "grid",
                      gridTemplateColumns: "72px 40px 1fr 220px",
                      gap: 20,
                      alignItems: "start",
                    }}
                  >
                    <div
                      style={{
                        fontFamily: "'JetBrains Mono', monospace",
                        fontSize: 13,
                        color: "var(--muted)",
                      }}
                    >
                      {formatTime(log.search_started_at)}
                    </div>
                    <span className="avs">
                      <span className={`av ${avClass}`}>{initial}</span>
                    </span>
                    <div>
                      <div
                        style={{
                          display: "flex",
                          alignItems: "center",
                          gap: 10,
                          marginBottom: 6,
                        }}
                      >
                        <span
                          style={{
                            fontFamily: "'Fraunces', serif",
                            fontSize: 15,
                            letterSpacing: "-.005em",
                            color: "var(--ink)",
                          }}
                        >
                          {log.performed_by || "\u2014"}
                        </span>
                        <span className={`pl ${km.cls}`}>{km.label}</span>
                      </div>
                      <div
                        style={{
                          fontSize: 14.5,
                          lineHeight: 1.55,
                          color: "var(--ink-2)",
                          maxWidth: "62ch",
                        }}
                      >
                        {log.objective || log.search_strategy}
                      </div>
                    </div>
                    <div
                      style={{
                        fontFamily: "'JetBrains Mono', monospace",
                        fontSize: 11.5,
                        color: "var(--accent)",
                        textAlign: "right",
                        letterSpacing: ".02em",
                      }}
                    >
                      {linked || "\u2014"}
                    </div>
                  </div>
                );
              })}
            </div>
          ))}
        </div>
      </div>
    </>
  );
}

export { InquiryView };
