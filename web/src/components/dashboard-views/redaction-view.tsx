'use client';

import {
  DocPaper,
  AvatarStack,
  Tag,
  EyebrowLabel,
} from '@/components/ui/dashboard';

/* --- Stub data matching design prototype --- */

interface Mark {
  readonly t: string;
  readonly who: string;
  readonly c: 'a' | 'b' | 'c' | 'd' | 'e';
  readonly sig: string;
  readonly k: string;
}

const MARKS: readonly Mark[] = [
  { t: 'Andriivka \u2192 geo-fuzz 50 km', who: 'Martyna', c: 'b', sig: 'd9a7', k: 'geo' },
  { t: '17:40 \u2192 17:52', who: 'Amir H.', c: 'c', sig: '2c14', k: 'edit' },
  { t: 'Col. M. \u2192 S-038', who: 'Juliane', c: 'd', sig: 'a1be', k: 'pseudo' },
  { t: 'Strike \u2014 operational source ref', who: 'W. Nyoka', c: 'e', sig: 'e3f0', k: 'redact' },
  { t: 'Strike \u2014 ongoing enquiry', who: 'H. Morel', c: 'a', sig: '7f22', k: 'redact' },
  { t: 'Strike \u2014 witness-identifying', who: 'W. Nyoka', c: 'e', sig: 'e3f1', k: 'redact' },
];

interface PresenceUser {
  readonly name: string;
  readonly initial: string;
  readonly c: 'a' | 'b' | 'c' | 'd' | 'e';
  readonly status: string;
}

const PRESENCE: readonly PresenceUser[] = [
  { name: 'H. Morel', initial: 'H', c: 'a', status: 'page 2' },
  { name: 'Martyna', initial: 'M', c: 'b', status: 'page 3 \u00b7 typing' },
  { name: 'Amir H.', initial: 'A', c: 'c', status: 'page 4' },
  { name: 'Juliane', initial: 'J', c: 'd', status: 'idle 2m' },
];

const HEADER_AVATARS = [
  { initial: 'H', color: 'a' as const },
  { initial: 'M', color: 'b' as const },
  { initial: 'A', color: 'c' as const },
  { initial: 'J', color: 'd' as const },
];

const TOOLBAR_CHIPS = [
  { label: 'Mark redaction', active: true },
  { label: 'Pseudonymise', active: false },
  { label: 'Translate gloss', active: false },
  { label: 'Add note', active: false },
  { label: 'Geo-fuzz', active: false },
];

/* --- Component --- */

export function RedactionView() {
  return (
    <>
      {/* Page header */}
      <section className="d-pagehead">
        <div>
          <EyebrowLabel>
            E-0912 &middot; W-0144 sworn statement v3 &middot; draft v4
          </EyebrowLabel>
          <h1>Collaborative <em>redaction</em></h1>
          <p className="sub">
            Four analysts in one document. Every mark, every pseudonym swap,
            every gloss is a signed CRDT op written to the same chain as
            ingest. Defence replay works keystroke-by-keystroke.
          </p>
        </div>
        <div className="actions">
          <AvatarStack users={HEADER_AVATARS} />
          <a className="btn ghost" href="#">Version history</a>
          <a className="btn" href="#">Seal draft v4 <span className="arr">&rarr;</span></a>
        </div>
      </section>

      {/* Toolbar + two-column editor */}
      <div className="panel" style={{ marginBottom: 22 }}>
        <div className="fbar">
          {TOOLBAR_CHIPS.map((chip) => (
            <span
              key={chip.label}
              className={`chip${chip.active ? ' active' : ''}`}
            >
              {chip.label}
            </span>
          ))}
          <span
            style={{
              marginLeft: 'auto',
              fontFamily: "'JetBrains Mono', monospace",
              fontSize: '11.5px',
              color: 'var(--muted)',
            }}
          >
            Page{' '}
            <strong style={{ color: 'var(--ink)', fontWeight: 500 }}>2</strong>{' '}
            of 6 &middot; 12 marks in draft
          </span>
          <span className="chip">&#9666; Prev</span>
          <span className="chip">Next &#9656;</span>
        </div>

        <div
          className="panel-body"
          style={{ padding: 30, background: 'var(--bg-2)' }}
        >
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: '1fr 300px',
              gap: 24,
              alignItems: 'start',
            }}
          >
            {/* Document paper */}
            <DocPaper>
              <span className="pn">P. 02 / 06</span>
              <p>
                The statement was recorded at the regional intake office on{' '}
                <mark>19 April 2026 at approximately 14:21 local time</mark>, in
                the presence of two intermediaries.
              </p>
              <p>
                The witness, hereafter <mark className="pseudo">W-0144</mark>,
                recalls arriving at the checkpoint near{' '}
                <mark>Andriivka</mark> at{' '}
                <span
                  style={{
                    textDecoration: 'line-through',
                    color: 'var(--muted)',
                  }}
                >
                  17:40
                </span>{' '}
                <strong>17:52</strong> local time, after the second convoy had
                passed.
              </p>
              <p>
                She identified the officer in command as{' '}
                <mark className="pseudo">subject S-038</mark>,{' '}
                <span className="redact">
                  &nbsp;redacted &mdash; operational source reference&nbsp;
                </span>
                , who she recognised from prior encounters at the administrative
                building.
              </p>
              <p>
                The witness further reported that the officer referenced{' '}
                <mark>&quot;Unit 28B&quot;</mark> and instructed subordinates to{' '}
                <span className="redact">
                  &nbsp;redacted &mdash; ongoing enquiry&nbsp;
                </span>
                .
              </p>
              <p>
                The statement was given voluntarily, in the presence of two
                intermediaries, one of whom is{' '}
                <span className="redact">
                  &nbsp;redacted &mdash; witness-identifying&nbsp;
                </span>{' '}
                unrelated to other proceedings.
              </p>
            </DocPaper>

            {/* Side panels */}
            <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
              {/* Marks panel */}
              <div className="panel" style={{ margin: 0 }}>
                <div className="panel-h" style={{ padding: '12px 14px' }}>
                  <h3 style={{ fontSize: 15 }}>Marks &middot; this page</h3>
                  <span className="meta">12</span>
                </div>
                <div className="panel-body" style={{ padding: '4px 0' }}>
                  {MARKS.map((m) => (
                    <div
                      key={m.sig}
                      style={{
                        display: 'grid',
                        gridTemplateColumns: '22px 1fr auto',
                        gap: 10,
                        alignItems: 'start',
                        padding: '10px 14px',
                        borderBottom: '1px solid var(--line)',
                        fontSize: '12.5px',
                      }}
                    >
                      <span className="avs" style={{ marginTop: 1 }}>
                        <span
                          className={`av ${m.c}`}
                          style={{
                            width: 22,
                            height: 22,
                            fontSize: 10,
                            borderWidth: '1.5px',
                          }}
                        >
                          {m.who[0]}
                        </span>
                      </span>
                      <div>
                        <div style={{ color: 'var(--ink-2)' }}>{m.t}</div>
                        <div
                          style={{
                            color: 'var(--muted)',
                            fontFamily: "'JetBrains Mono', monospace",
                            fontSize: '10.5px',
                            marginTop: 2,
                          }}
                        >
                          {m.who} &middot; sig {m.sig}&hellip;
                        </div>
                      </div>
                      <Tag>{m.k}</Tag>
                    </div>
                  ))}
                </div>
              </div>

              {/* Presence panel */}
              <div className="panel" style={{ margin: 0 }}>
                <div className="panel-h" style={{ padding: '12px 14px' }}>
                  <h3 style={{ fontSize: 15 }}>Presence</h3>
                  <span className="meta">4 live</span>
                </div>
                <div
                  className="panel-body"
                  style={{
                    padding: 14,
                    display: 'flex',
                    flexDirection: 'column',
                    gap: 8,
                    fontSize: '12.5px',
                  }}
                >
                  {PRESENCE.map((u) => (
                    <div
                      key={u.name}
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: 10,
                      }}
                    >
                      <AvatarStack users={[{ initial: u.initial, color: u.c }]} />
                      <span>
                        {u.name}{' '}
                        <span
                          style={{
                            color: 'var(--muted)',
                            fontFamily: "'JetBrains Mono', monospace",
                            fontSize: 10,
                          }}
                        >
                          &middot; {u.status}
                        </span>
                      </span>
                    </div>
                  ))}
                  <div
                    style={{
                      fontFamily: "'JetBrains Mono', monospace",
                      fontSize: '10.5px',
                      color: 'var(--muted)',
                      borderTop: '1px solid var(--line)',
                      paddingTop: 8,
                      marginTop: 4,
                      letterSpacing: '.04em',
                      textTransform: 'uppercase',
                    }}
                  >
                    head &middot; f208&hellip;bc91 &middot; 142 ms p95
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

export default RedactionView;
