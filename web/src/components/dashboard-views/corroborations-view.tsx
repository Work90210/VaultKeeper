'use client';

import { useState } from 'react';
import type { CorroborationClaim } from '@/types';
import {
  KPIStrip,
  Panel,
  StatusPill,
  LinkArrow,
  EyebrowLabel,
  Modal,
} from '@/components/ui/dashboard';

/* ─── Props ─── */

export interface CorroborationsViewProps {
  readonly claims?: CorroborationClaim[];
}

/* ─── Stub data matching design prototype ─── */

interface ClaimSource {
  readonly kind: string;
  readonly name: string;
}

interface StubClaim {
  readonly ref: string;
  readonly text: string;
  readonly score: number;
  readonly status: string;
  readonly sources: readonly ClaimSource[];
}

const CLAIMS: readonly StubClaim[] = [
  {
    ref: 'C-0412',
    text: 'S-038 gave checkpoint order at 17:52, 19 Apr 2026',
    score: 0.91,
    status: 'corroborated',
    sources: [
      { kind: 'witness', name: 'W-0144 statement' },
      { kind: 'video', name: 'E-0918 drone 03:18' },
      { kind: 'audio', name: 'E-0916 intercept 14:03' },
      { kind: 'osint', name: 'Sentinel-2 18 Apr' },
    ],
  },
  {
    ref: 'C-0411',
    text: 'Second convoy movement through Andriivka region',
    score: 0.46,
    status: 'weak',
    sources: [{ kind: 'witness', name: 'W-0139 only' }],
  },
  {
    ref: 'C-0410',
    text: 'Unit 28B referenced in direct command chain',
    score: 0.78,
    status: 'corroborated',
    sources: [
      { kind: 'audio', name: 'E-0916 intercept' },
      { kind: 'doc', name: 'E-0917 intake p.4' },
      { kind: 'doc', name: 'E-0914 ledger notation' },
    ],
  },
  {
    ref: 'C-0409',
    text: 'Body-cam BC-17 excerpt matches W-0144 account',
    score: 0.87,
    status: 'corroborated',
    sources: [
      { kind: 'video', name: 'E-0910 body-cam' },
      { kind: 'witness', name: 'W-0144 statement' },
      { kind: 'video', name: 'E-0918 drone' },
    ],
  },
  {
    ref: 'C-0408',
    text: 'Financial transfers coincide with incident window',
    score: 0.62,
    status: 'investigating',
    sources: [
      { kind: 'doc', name: 'E-0914 Raiffeisen' },
      { kind: 'note', name: 'NOTE-181' },
    ],
  },
  {
    ref: 'C-0407',
    text: 'Plate BX-4281 matches administrative-building records',
    score: 0.69,
    status: 'investigating',
    sources: [
      { kind: 'img', name: 'E-0907 plate' },
      { kind: 'osint', name: 'external reg' },
    ],
  },
  {
    ref: 'C-0406',
    text: 'Defence exculpatory: S-038 off-duty on 19 Apr',
    score: 0.18,
    status: 'refuted',
    sources: [
      { kind: 'video', name: 'E-0918 drone 17:54' },
      { kind: 'witness', name: 'W-0141 deposition' },
    ],
  },
];

const STATUS_PILL: Record<string, 'sealed' | 'disc' | 'hold' | 'broken'> = {
  corroborated: 'sealed',
  weak: 'disc',
  investigating: 'hold',
  refuted: 'broken',
};

const KIND_AVATAR: Record<string, 'a' | 'b' | 'c' | 'd' | 'e'> = {
  witness: 'c',
  video: 'a',
  audio: 'b',
  doc: 'd',
  osint: 'e',
  img: 'e',
  note: 'a',
};

function scoreColor(sc: number): string {
  if (sc >= 0.75) return 'var(--ok)';
  if (sc >= 0.5) return 'var(--accent)';
  return 'var(--muted)';
}

function barColor(sc: number): string {
  if (sc >= 0.75) return 'var(--ok)';
  if (sc >= 0.5) return 'var(--accent)';
  return '#b35c5c';
}

/* ─── Component ─── */

export function CorroborationsView({ claims }: CorroborationsViewProps) {
  const [modalOpen, setModalOpen] = useState(false);

  const hasRealData = claims !== undefined && claims.length > 0;

  const STRENGTH_STATUS: Record<string, string> = {
    strong: 'corroborated',
    moderate: 'investigating',
    weak: 'weak',
    contradicted: 'refuted',
  };

  const displayClaims: readonly StubClaim[] = hasRealData
    ? claims.map((c): StubClaim => ({
        ref: `C-${c.id.slice(0, 4).toUpperCase()}`,
        text: c.claim_summary,
        score: c.strength === 'strong' ? 0.85 : c.strength === 'moderate' ? 0.65 : c.strength === 'weak' ? 0.4 : 0.15,
        status: STRENGTH_STATUS[c.strength] ?? 'investigating',
        sources: c.evidence.map((e) => ({
          kind: 'doc',
          name: e.evidence_id.slice(0, 12),
        })),
      }))
    : CLAIMS;

  return (
    <>
      <section className="d-pagehead">
        <div>
          <EyebrowLabel>Case &middot; ICC-UKR-2024 &middot; Berkeley Protocol Phase 5</EyebrowLabel>
          <h1>Multi-source <em>corroboration</em></h1>
          <p className="sub">
            Phase 5 of the Berkeley Protocol requires source authentication, content verification, and multi-source corroboration. Every factual claim carries a score computed from independent sources, each weighted by evidence kind and custody strength. Single-source claims are shown, never hidden &mdash; that&apos;s the point.
          </p>
        </div>
        <div className="actions">
          <a className="btn ghost" href="#">Matrix view</a>
          <button type="button" className="btn" onClick={() => setModalOpen(true)}>
            New claim <span className="arr">&rarr;</span>
          </button>
        </div>
      </section>

      <KPIStrip items={[
        { label: 'Claims tracked', value: '62', sub: '41 corroborated \u00b7 14 investigating \u00b7 7 refuted' },
        { label: 'Median score', value: '0.74', sub: 'across corroborated' },
        { label: 'Single-source \u00b7 flagged', value: '9', delta: '\u25CF preserved & disclosed', deltaNegative: true },
        { label: 'Cross-case links', value: '23', sub: 'to CIJA Berlin sub-chain' },
      ]} />

      <div style={{ marginBottom: 22 }} />

      <Panel title="Claims" meta="sorted by score">
        <div style={{ padding: 0 }}>
          {displayClaims.map((c) => (
            <div
              key={c.ref}
              style={{
                padding: '20px 28px',
                borderBottom: '1px solid var(--line)',
                display: 'grid',
                gridTemplateColumns: '90px 1fr 220px 140px',
                gap: 28,
                alignItems: 'center',
              }}
            >
              {/* Score */}
              <div style={{ textAlign: 'center' }}>
                <div
                  style={{
                    fontFamily: "'Fraunces', serif",
                    fontSize: 36,
                    letterSpacing: '-.03em',
                    color: scoreColor(c.score),
                    lineHeight: 1,
                  }}
                >
                  {c.score.toFixed(2)}
                </div>
                <div
                  style={{
                    fontFamily: "'JetBrains Mono', monospace",
                    fontSize: 10,
                    color: 'var(--muted)',
                    letterSpacing: '.06em',
                    textTransform: 'uppercase',
                    marginTop: 4,
                  }}
                >
                  score
                </div>
              </div>

              {/* Claim text */}
              <div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 6 }}>
                  <span className="ref" style={{ display: 'block' }}>
                    <strong>{c.ref}</strong>
                  </span>
                  <StatusPill status={STATUS_PILL[c.status] ?? 'disc'}>{c.status}</StatusPill>
                </div>
                <div
                  style={{
                    fontFamily: "'Fraunces', serif",
                    fontSize: 18,
                    letterSpacing: '-.01em',
                    lineHeight: 1.3,
                  }}
                >
                  {c.text}
                </div>
              </div>

              {/* Sources */}
              <div style={{ display: 'flex', flexDirection: 'column', gap: 5 }}>
                <div
                  style={{
                    fontFamily: "'JetBrains Mono', monospace",
                    fontSize: 10,
                    letterSpacing: '.08em',
                    textTransform: 'uppercase',
                    color: 'var(--muted)',
                    marginBottom: 3,
                  }}
                >
                  sources &middot; {c.sources.length}
                </div>
                {c.sources.map((s) => (
                  <div
                    key={s.name}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 8,
                      fontSize: 12.5,
                    }}
                  >
                    <span
                      className={`av ${KIND_AVATAR[s.kind] ?? 'd'}`}
                      style={{ width: 20, height: 20, fontSize: 9, borderWidth: 1 }}
                    >
                      {s.kind[0].toUpperCase()}
                    </span>
                    <span style={{ color: 'var(--ink-2)' }}>{s.name}</span>
                  </div>
                ))}
              </div>

              {/* Progress bar + link */}
              <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-end', gap: 6 }}>
                <div
                  style={{
                    width: '100%',
                    height: 6,
                    background: 'var(--bg-2)',
                    borderRadius: 3,
                    overflow: 'hidden',
                  }}
                >
                  <div
                    style={{
                      height: '100%',
                      width: `${c.score * 100}%`,
                      background: barColor(c.score),
                    }}
                  />
                </div>
                <LinkArrow href="#">Open claim</LinkArrow>
              </div>
            </div>
          ))}
        </div>
      </Panel>

      {/* ── New claim modal ── */}
      <Modal open={modalOpen} onClose={() => setModalOpen(false)} title="New corroboration claim" wide>
        <form style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <label style={{ display: 'flex', flexDirection: 'column', gap: 4, fontSize: 13 }}>
            <span style={{ fontWeight: 500, color: 'var(--ink)' }}>Factual claim</span>
            <textarea
              rows={3}
              placeholder="Describe the factual assertion to be corroborated\u2026"
              style={{ padding: '8px 10px', border: '1px solid var(--line)', borderRadius: 6, fontSize: 13, resize: 'vertical' }}
            />
          </label>
          <label style={{ display: 'flex', flexDirection: 'column', gap: 4, fontSize: 13 }}>
            <span style={{ fontWeight: 500, color: 'var(--ink)' }}>Primary evidence</span>
            <input type="text" placeholder="E-XXXX or reference" style={{ padding: '8px 10px', border: '1px solid var(--line)', borderRadius: 6, fontSize: 13 }} />
          </label>
          <label style={{ display: 'flex', flexDirection: 'column', gap: 4, fontSize: 13 }}>
            <span style={{ fontWeight: 500, color: 'var(--ink)' }}>Supporting evidence</span>
            <input type="text" placeholder="Comma-separated references" style={{ padding: '8px 10px', border: '1px solid var(--line)', borderRadius: 6, fontSize: 13 }} />
          </label>
          <label style={{ display: 'flex', flexDirection: 'column', gap: 4, fontSize: 13 }}>
            <span style={{ fontWeight: 500, color: 'var(--ink)' }}>Contradicting evidence</span>
            <input type="text" placeholder="Comma-separated references (if any)" style={{ padding: '8px 10px', border: '1px solid var(--line)', borderRadius: 6, fontSize: 13 }} />
          </label>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
            <label style={{ display: 'flex', flexDirection: 'column', gap: 4, fontSize: 13 }}>
              <span style={{ fontWeight: 500, color: 'var(--ink)' }}>Initial score</span>
              <input type="number" min="0" max="1" step="0.01" placeholder="0.00" style={{ padding: '8px 10px', border: '1px solid var(--line)', borderRadius: 6, fontSize: 13 }} />
            </label>
            <label style={{ display: 'flex', flexDirection: 'column', gap: 4, fontSize: 13 }}>
              <span style={{ fontWeight: 500, color: 'var(--ink)' }}>Status</span>
              <select style={{ padding: '8px 10px', border: '1px solid var(--line)', borderRadius: 6, fontSize: 13 }}>
                <option value="investigating">Investigating</option>
                <option value="corroborated">Corroborated</option>
                <option value="weak">Weak</option>
                <option value="refuted">Refuted</option>
              </select>
            </label>
          </div>
          <label style={{ display: 'flex', flexDirection: 'column', gap: 4, fontSize: 13 }}>
            <span style={{ fontWeight: 500, color: 'var(--ink)' }}>Assigned to</span>
            <input type="text" placeholder="Analyst name" style={{ padding: '8px 10px', border: '1px solid var(--line)', borderRadius: 6, fontSize: 13 }} />
          </label>
          <label style={{ display: 'flex', flexDirection: 'column', gap: 4, fontSize: 13 }}>
            <span style={{ fontWeight: 500, color: 'var(--ink)' }}>Notes</span>
            <textarea rows={2} placeholder="Additional context\u2026" style={{ padding: '8px 10px', border: '1px solid var(--line)', borderRadius: 6, fontSize: 13, resize: 'vertical' }} />
          </label>
          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 10, paddingTop: 8 }}>
            <button type="button" className="btn ghost" onClick={() => setModalOpen(false)}>Cancel</button>
            <button type="submit" className="btn">Create claim <span className="arr">&rarr;</span></button>
          </div>
        </form>
      </Modal>
    </>
  );
}

export default CorroborationsView;
