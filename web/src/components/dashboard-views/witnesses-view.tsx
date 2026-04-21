'use client';

import { useState } from 'react';
import type { Witness } from '@/types';
import {
  KPIStrip,
  Panel,
  DataTable,
  FilterBar,
  StatusPill,
  Tag,
  LinkArrow,
  EyebrowLabel,
  Timeline,
  KeyValueList,
  Modal,
} from '@/components/ui/dashboard';

/* ─── Props ─── */

export interface WitnessesViewProps {
  readonly witnesses?: Witness[];
  readonly total?: number;
}

/* ─── Stub data matching design prototype ─── */

interface StubWitness {
  readonly id: string;
  readonly pseudonymStatus: string;
  readonly risk: 'low' | 'medium' | 'high' | 'extreme';
  readonly sig: string;
  readonly age: string;
  readonly exhibits: string;
  readonly score: string;
  readonly voiceMasked: string;
  readonly source: string;
}

const WITNESSES: readonly StubWitness[] = [
  { id: 'W-0144', pseudonymStatus: 'Pseudonymised', risk: 'high', sig: 'signed', age: '14 d', exhibits: '4 exhibits', score: '0.82', voiceMasked: 'voice-masked', source: 'field intake' },
  { id: 'W-0143', pseudonymStatus: 'Pseudonymised', risk: 'high', sig: 'signed', age: '14 d', exhibits: '2 exhibits', score: '0.71', voiceMasked: 'voice-masked', source: 'field intake' },
  { id: 'W-0142', pseudonymStatus: 'Pseudonymised', risk: 'medium', sig: 'signed', age: '16 d', exhibits: '1 exhibit', score: '0.60', voiceMasked: '\u2014', source: 'remote \u00b7 encrypted' },
  { id: 'W-0141', pseudonymStatus: 'Cleared name', risk: 'low', sig: 'signed', age: '18 d', exhibits: '6 exhibits', score: '0.88', voiceMasked: '\u2014', source: 'sworn deposition' },
  { id: 'W-0140', pseudonymStatus: 'Pseudonymised', risk: 'extreme', sig: 'duress-armed', age: '21 d', exhibits: '3 exhibits', score: '0.76', voiceMasked: 'voice-masked', source: 'protected channel' },
  { id: 'W-0139', pseudonymStatus: 'Pseudonymised', risk: 'high', sig: 'signed', age: '22 d', exhibits: '1 exhibit', score: '0.54', voiceMasked: 'voice-masked', source: 'intermediary' },
  { id: 'W-0138', pseudonymStatus: 'Cleared name', risk: 'medium', sig: 'signed', age: '24 d', exhibits: '2 exhibits', score: '0.80', voiceMasked: '\u2014', source: 'field intake' },
  { id: 'W-0137', pseudonymStatus: 'Pseudonymised', risk: 'high', sig: 'signed', age: '25 d', exhibits: '5 exhibits', score: '0.91', voiceMasked: 'voice-masked', source: 'protected channel' },
];

const RISK_PILL: Record<string, 'sealed' | 'disc' | 'hold' | 'broken'> = {
  low: 'sealed',
  medium: 'disc',
  high: 'hold',
  extreme: 'broken',
};

const DURESS_EVENTS = [
  { content: <><em>Juliane</em> unsealed <strong>W-0140</strong> real-name for notary transmission</>, subline: 'countersigned by H. Morel \u00b7 reason: cross-border warrant', time: '2 d', accent: true },
  { content: <>Decoy vault opened for <strong>W-0137</strong> &mdash; phishing probe detected</>, subline: 'origin: 45.61.x.x \u00b7 auto-blocked', time: '6 d' },
  { content: <>Ceremony: key rotation &middot; <strong>2-of-3 quorum</strong> reached</>, subline: 'Morel \u00b7 Nyoka \u00b7 H\u00e4mmerli', time: '11 d' },
  { content: <><em>Amir H.</em> re-pseudonymised <strong>W-0139</strong> on legal officer request</>, subline: 'mapping re-sealed \u00b7 3 exhibits re-linked', time: '14 d' },
];

const POSTURE_ITEMS = [
  { label: 'Mapping vault', value: <>Sealed by <strong>2-of-3</strong> quorum (Morel / Nyoka / H{'\u00e4'}mmerli). Rotates every 90 days.</> },
  { label: 'Default intake', value: 'Pseudonym auto-assigned at first save. Real name never reaches disk without quorum.' },
  { label: 'Geo-fuzzing', value: '50 km default radius on mentioned locations; tunable per-witness.' },
  { label: 'Duress passphrase', value: 'Opens a decoy vault containing fabricated statements. Event sealed.' },
  { label: 'Voice masking', value: 'Real-time DSP pitch-shift + formant flatten on all pseudonymised recordings.' },
  { label: 'Defence access', value: 'Pseudonymised view only. Real-name requests require judicial order + ceremony.' },
];

const TABLE_COLUMNS = [
  { key: 'pseudonym', label: 'Pseudonym' },
  { key: 'risk', label: 'Risk' },
  { key: 'intake', label: 'Intake' },
  { key: 'exhibits', label: 'Exhibits' },
  { key: 'corrob', label: 'Corrob. score' },
  { key: 'voice', label: 'Voice' },
  { key: 'signature', label: 'Signature' },
  { key: 'age', label: 'Age' },
  { key: 'actions', label: '' },
];

const CHIPS = [
  { key: 'all', label: 'All \u00b7218', active: true },
  { key: 'high-risk', label: 'High-risk', count: 41 },
  { key: 'duress', label: 'Duress-armed', count: 42 },
  { key: 'voice', label: 'Voice-masked', count: 164 },
  { key: 'intermediary', label: 'Intermediary \u00b7 any \u25BE' },
];

/* ─── Component ─── */

export function WitnessesView({ witnesses, total }: WitnessesViewProps) {
  const [modalOpen, setModalOpen] = useState(false);

  const hasRealData = witnesses !== undefined && witnesses.length > 0;

  // Map real Witness[] to stub shape for rendering when real data is available
  const displayWitnesses: readonly StubWitness[] = hasRealData
    ? witnesses.map((w): StubWitness => ({
        id: w.witness_code,
        pseudonymStatus: w.identity_visible ? 'Cleared name' : 'Pseudonymised',
        risk: w.protection_status === 'high_risk' ? 'high' : w.protection_status === 'protected' ? 'medium' : 'low',
        sig: 'signed',
        age: `${Math.max(1, Math.round((Date.now() - new Date(w.created_at).getTime()) / 86400000))} d`,
        exhibits: `${w.related_evidence.length} exhibit${w.related_evidence.length !== 1 ? 's' : ''}`,
        score: '\u2014',
        voiceMasked: '\u2014',
        source: 'API',
      }))
    : WITNESSES;

  const displayTotal = hasRealData ? (total ?? witnesses.length) : 218;

  return (
    <>
      <section className="d-pagehead">
        <div>
          <EyebrowLabel>Case &middot; ICC-UKR-2024 &middot; witness-sensitive</EyebrowLabel>
          <h1>Witness <em>register</em></h1>
          <p className="sub">
            Every record is sealed with AES-256-GCM application-level encryption. Defence counsel see pseudonyms only; the mapping lives in a separate vault sealed by a two-of-three key ceremony. Break-the-glass unsealing is itself a sealed event.
          </p>
        </div>
        <div className="actions">
          <a className="btn ghost" href="#">Ceremony log</a>
          <button type="button" className="btn" onClick={() => setModalOpen(true)}>
            Add witness <span className="arr">&rarr;</span>
          </button>
        </div>
      </section>

      <KPIStrip items={[
        { label: 'Total protected', value: String(displayTotal), sub: hasRealData ? `${displayWitnesses.filter(w => w.pseudonymStatus === 'Pseudonymised').length} pseudonymised \u00b7 ${displayWitnesses.filter(w => w.pseudonymStatus === 'Cleared name').length} cleared` : '198 pseudonymised \u00b7 20 cleared' },
        { label: 'Duress armed', value: '42', sub: 'decoy vault ready' },
        { label: 'Voice-masked', value: '164', sub: 'real-time DSP pipeline' },
        { label: 'Break-the-glass \u00b7 90 d', value: '2', delta: '\u25CF both countersigned', deltaNegative: true },
      ]} />

      <div style={{ marginBottom: 22 }} />

      <div className="panel">
        <FilterBar
          searchPlaceholder="pseudonym, linked exhibit, intermediary\u2026"
          chips={CHIPS}
        />
        <DataTable columns={TABLE_COLUMNS}>
          {displayWitnesses.map((w) => (
            <tr key={w.id}>
              <td>
                <div className="ref">
                  {w.id}
                  <small>{w.pseudonymStatus}</small>
                </div>
              </td>
              <td>
                <StatusPill status={RISK_PILL[w.risk]}>{w.risk}</StatusPill>
              </td>
              <td style={{ fontSize: 13, color: 'var(--muted)' }}>{w.source}</td>
              <td className="num">{w.exhibits}</td>
              <td>
                <Tag accent={parseFloat(w.score) >= 0.8}>{w.score}</Tag>
              </td>
              <td><Tag>{w.voiceMasked}</Tag></td>
              <td>
                <StatusPill status={w.sig === 'duress-armed' ? 'broken' : 'sealed'}>
                  {w.sig}
                </StatusPill>
              </td>
              <td className="mono">{w.age}</td>
              <td className="actions">
                <LinkArrow href="#">Open</LinkArrow>
              </td>
            </tr>
          ))}
        </DataTable>
      </div>

      <div className="g2" style={{ marginTop: 22 }}>
        <Panel title="Duress & break-the-glass" meta="last 30 events">
          <Timeline items={DURESS_EVENTS} />
        </Panel>

        <Panel title="Protection posture" meta="posture v3">
          <KeyValueList items={POSTURE_ITEMS} />
        </Panel>
      </div>

      {/* ── Add witness modal ── */}
      <Modal open={modalOpen} onClose={() => setModalOpen(false)} title="Add witness">
        <form style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <label style={{ display: 'flex', flexDirection: 'column', gap: 4, fontSize: 13 }}>
            <span style={{ fontWeight: 500, color: 'var(--ink)' }}>Real name <span style={{ color: 'var(--muted)', fontWeight: 400 }}>(encrypted at rest)</span></span>
            <input type="text" placeholder="Full legal name" style={{ padding: '8px 10px', border: '1px solid var(--line)', borderRadius: 6, fontSize: 13 }} />
          </label>
          <label style={{ display: 'flex', flexDirection: 'column', gap: 4, fontSize: 13 }}>
            <span style={{ fontWeight: 500, color: 'var(--ink)' }}>Pseudonym <span style={{ color: 'var(--muted)', fontWeight: 400 }}>(auto-generated)</span></span>
            <input type="text" placeholder="W-0145" readOnly style={{ padding: '8px 10px', border: '1px solid var(--line)', borderRadius: 6, fontSize: 13, background: 'var(--bg-2)', color: 'var(--muted)' }} />
          </label>
          <label style={{ display: 'flex', flexDirection: 'column', gap: 4, fontSize: 13 }}>
            <span style={{ fontWeight: 500, color: 'var(--ink)' }}>Risk level</span>
            <select style={{ padding: '8px 10px', border: '1px solid var(--line)', borderRadius: 6, fontSize: 13 }}>
              <option value="low">Low</option>
              <option value="medium">Medium</option>
              <option value="high">High</option>
              <option value="extreme">Extreme</option>
            </select>
          </label>
          <label style={{ display: 'flex', flexDirection: 'column', gap: 4, fontSize: 13 }}>
            <span style={{ fontWeight: 500, color: 'var(--ink)' }}>Source method</span>
            <select style={{ padding: '8px 10px', border: '1px solid var(--line)', borderRadius: 6, fontSize: 13 }}>
              <option value="field">Field intake</option>
              <option value="remote">Remote (encrypted)</option>
              <option value="deposition">Sworn deposition</option>
              <option value="protected">Protected channel</option>
              <option value="intermediary">Intermediary</option>
            </select>
          </label>
          <label style={{ display: 'flex', flexDirection: 'column', gap: 4, fontSize: 13 }}>
            <span style={{ fontWeight: 500, color: 'var(--ink)' }}>Voice masking</span>
            <select style={{ padding: '8px 10px', border: '1px solid var(--line)', borderRadius: 6, fontSize: 13 }}>
              <option value="off">Off</option>
              <option value="on">Enabled (real-time DSP)</option>
            </select>
          </label>
          <label style={{ display: 'flex', flexDirection: 'column', gap: 4, fontSize: 13 }}>
            <span style={{ fontWeight: 500, color: 'var(--ink)' }}>Duress passphrase</span>
            <input type="password" placeholder="Optional decoy-vault trigger" style={{ padding: '8px 10px', border: '1px solid var(--line)', borderRadius: 6, fontSize: 13 }} />
          </label>
          <label style={{ display: 'flex', flexDirection: 'column', gap: 4, fontSize: 13 }}>
            <span style={{ fontWeight: 500, color: 'var(--ink)' }}>Notes</span>
            <textarea rows={3} placeholder="Additional context (encrypted)" style={{ padding: '8px 10px', border: '1px solid var(--line)', borderRadius: 6, fontSize: 13, resize: 'vertical' }} />
          </label>
          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 10, paddingTop: 8 }}>
            <button type="button" className="btn ghost" onClick={() => setModalOpen(false)}>Cancel</button>
            <button type="submit" className="btn">Add witness <span className="arr">&rarr;</span></button>
          </div>
        </form>
      </Modal>
    </>
  );
}

export default WitnessesView;
