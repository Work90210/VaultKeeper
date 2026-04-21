'use client';

import { useState } from 'react';
import type { InquiryLog } from '@/types';

const INQ = {
  id: 'INQ-0184',
  date: '19 Apr 2026, 14:22',
  author: 'H. Morel',
  role: 'Senior analyst',
  type: 'Decision',
  priority: 'Normal',
  assigned: 'W. Nyoka',
  sig: 'ed25519:7f22\u2026bc91',
  block: 'f209\u2026bc91',
  content: 'Recommend inclusion of E-0918 in DISC-2026-019 bundle. Countersign required from W. Nyoka before seal. Drone footage corroborated by Sentinel-2 satellite pass and two independent witness statements (W-0144, W-0099). Assessment score 9/10 relevance, 8/10 reliability.',
  strategy: 'Drone footage and satellite imagery of Butcha residential district, 18\u201319 April 2024, showing damage to civilian infrastructure and military vehicle movements at the northeastern checkpoint on Vokzalna Street.',
  keywords: 'Butcha drone footage 2024, \u0412\u043e\u043a\u0437\u0430\u043b\u044c\u043d\u0430 \u0432\u0443\u043b\u0438\u0446\u044f checkpoint, DJI aerial Bucha',
  tool: 'Telegram search, Copernicus Open Access Hub',
  period: '18 Apr 2026, 10:00\u201310:30',
  results: { found: 14, relevant: 3, collected: 3 },
};

interface Phase { done?: boolean; partial?: boolean; date?: string; note?: string; rel?: number; rlb?: number; rec?: string; by?: string; method?: string; collector?: string; geo?: string; hash?: string; tsa?: string; events?: number; type?: string; finding?: string; confidence?: string; }
interface TrackedItem { ref: string; name: string; kind: string; size: string; phase1: Phase; phase2: Phase; phase3: Phase; phase4: Phase; phase5: Phase; phase6: Phase; [key: string]: unknown; }

const ITEMS: TrackedItem[] = [
  { ref:'E-0918', name:'Butcha_drone_04.mp4', kind:'VIDEO', size:'218 MB', phase1:{done:true,date:'18 Apr',note:'Found via Telegram @ua_evidence'}, phase2:{done:true,date:'19 Apr',rel:9,rlb:8,rec:'Collect',by:'Amir Haddad'}, phase3:{done:true,date:'18 Apr',method:'Manual download',collector:'H. Morel',geo:'Butcha, Kyiv Oblast'}, phase4:{done:true,date:'18 Apr',hash:'a7f2e4c9\u2026',tsa:'ts-eu-west',events:10}, phase5:{done:true,date:'18 Apr',type:'Geolocation',finding:'Authentic',confidence:'High'}, phase6:{done:false,partial:true,note:'Corroboration C-0412 linked, analysis note pending'} },
  { ref:'E-0911', name:'Satellite \u2014 18 Apr 09:12Z', kind:'IMG', size:'81 MB', phase1:{done:true,date:'18 Apr',note:'Copernicus Sentinel-2 pass'}, phase2:{done:true,date:'18 Apr',rel:8,rlb:9,rec:'Collect',by:'H. Morel'}, phase3:{done:true,date:'18 Apr',method:'API capture',collector:'H. Morel',geo:'Butcha region'}, phase4:{done:true,date:'18 Apr',hash:'91ab\u2026f208',tsa:'ts-eu-west',events:4}, phase5:{done:true,date:'19 Apr',type:'Cross-reference',finding:'Corroborates E-0918',confidence:'High'}, phase6:{done:true,date:'19 Apr',note:'NOTE-183: Timeline reconciliation'} },
  { ref:'E-0916', name:'Intercept 14:03 EET', kind:'AUDIO', size:'17 MB', phase1:{done:true,date:'18 Apr',note:'Intercepted radio communication'}, phase2:{done:true,date:'18 Apr',rel:7,rlb:6,rec:'Collect',by:'Amir Haddad'}, phase3:{done:true,date:'18 Apr',method:'Forensic tool',collector:'Amir Haddad',geo:'Classified'}, phase4:{done:true,date:'18 Apr',hash:'b3e1f7\u2026',tsa:'ts-eu-west',events:6}, phase5:{done:false,partial:true,note:'Speaker ID pending voice analysis'}, phase6:{} },
  { ref:'E-0928', name:'W-0146_intake_draft.pdf', kind:'DOC', size:'1.2 MB', phase1:{done:true,date:'18 Apr',note:'Field interview lead from W-0144'}, phase2:{done:true,date:'19 Apr',rel:6,rlb:7,rec:'Monitor',by:'Juliane Wirth'}, phase3:{done:false,note:'Monitoring \u2014 awaiting second interview'}, phase4:{}, phase5:{}, phase6:{} },
  { ref:'E-0929', name:'Checkpoint_CCTV_seg4.mp4', kind:'VIDEO', size:'312 MB', phase1:{done:true,date:'18 Apr',note:'CCTV footage from police handover'}, phase2:{done:true,date:'19 Apr',rel:9,rlb:8,rec:'Collect',by:'H. Morel'}, phase3:{done:true,date:'19 Apr',method:'Physical transfer',collector:'H. Morel',geo:'Butcha police station'}, phase4:{done:true,date:'19 Apr',hash:'d4a2c8\u2026',tsa:'ts-eu-west',events:3}, phase5:{done:false,note:'Awaiting geolocation verification'}, phase6:{} },
  { ref:'E-0930', name:'Fake_news_article.html', kind:'DOC', size:'340 KB', phase1:{done:true,date:'18 Apr',note:'Disinformation site, flagged for review'}, phase2:{done:true,date:'18 Apr',rel:2,rlb:1,rec:'Discard',by:'Amir Haddad'}, phase3:{done:false,note:'Discarded \u2014 disinformation, reasoning preserved'}, phase4:{}, phase5:{}, phase6:{} },
  { ref:'E-0927', name:'Unverified_clip_TikTok.mp4', kind:'VIDEO', size:'4 MB', phase1:{done:true,date:'18 Apr',note:'TikTok repost, unknown origin'}, phase2:{done:true,date:'18 Apr',rel:3,rlb:2,rec:'Deprioritize',by:'Amir Haddad'}, phase3:{done:false,note:'Deprioritized \u2014 not collected'}, phase4:{}, phase5:{}, phase6:{} },
  { ref:'E-0931', name:'Raiffeisen_batch3.csv', kind:'DOC', size:'42 KB', phase1:{done:true,date:'18 Apr',note:'Financial records from OLAF referral'}, phase2:{done:false,note:'Awaiting assessment'}, phase3:{}, phase4:{}, phase5:{}, phase6:{} },
];

function phaseStatus(item: TrackedItem, n: number): 'done' | 'active' | 'pending' {
  const p = item[`phase${n}`] as Phase | undefined;
  if (!p) return 'pending';
  if (p.done) return 'done';
  if (p.partial) return 'active';
  return 'pending';
}

function PhaseDetail({ item, n }: { item: TrackedItem; n: number }) {
  const p = item[`phase${n}`] as Phase | undefined;
  if (!p || !p.done) {
    if (p?.partial) return <div style={{ fontSize: 11, color: 'var(--accent)', lineHeight: 1.4 }}>{p.note || 'In progress'}</div>;
    if (p?.note) return <div style={{ fontSize: 11, color: 'var(--muted)', lineHeight: 1.4 }}>{p.note}</div>;
    return <div style={{ fontSize: 11, color: 'var(--muted)' }}>Not started</div>;
  }
  return (
    <div style={{ fontSize: 11, color: 'var(--ink-2)', lineHeight: 1.5 }}>
      {n === 1 && p.note}
      {n === 2 && <>Rel: {p.rel}/10 &middot; Rlb: {p.rlb}/10<br />{p.rec} &middot; {p.by}</>}
      {n === 3 && <>{p.method}<br />{p.collector} &middot; {p.geo}</>}
      {n === 4 && <><span style={{ fontFamily: 'JetBrains Mono,monospace', fontSize: 10 }}>{p.hash}</span><br />{p.events} custody events</>}
      {n === 5 && <>{p.type}: {p.finding}<br />Confidence: {p.confidence}</>}
      {n === 6 && p.note}
    </div>
  );
}

const PHASE_NAMES = ['', 'Inquiry', 'Assess', 'Collect', 'Preserve', 'Verify', 'Analyse'];

function VKModal({ open, onClose, children }: { open: boolean; onClose: () => void; children: React.ReactNode }) {
  if (!open) return null;
  return (
    <div className="vk-modal open" onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}>
      <div className="vk-modal-box">{children}</div>
    </div>
  );
}

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

async function apiCall(path: string, method: string, token: string, body?: Record<string, unknown>) {
  const res = await fetch(`${API_BASE}${path}`, {
    method,
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) {
    const data = await res.json().catch(() => null);
    throw new Error(data?.error || `${method} ${path} failed (${res.status})`);
  }
  return res.json().catch(() => ({}));
}

export function InquiryLogDetail({ log, accessToken }: { readonly log: InquiryLog; readonly accessToken: string }) {
  const [inquiryStatus, setInquiryStatus] = useState<'active' | 'locked' | 'complete'>('active');
  const [activeModal, setActiveModal] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const openModal = (id: string) => { setError(null); setActiveModal(id); };
  const closeModal = () => setActiveModal(null);
  const isEditable = inquiryStatus === 'active';

  async function handleLock(reason: string) {
    setSubmitting(true);
    setError(null);
    try {
      await apiCall(`/api/inquiry-logs/${log.id}/lock`, 'POST', accessToken, { reason });
      setInquiryStatus('locked');
      closeModal();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Lock failed');
    } finally {
      setSubmitting(false);
    }
  }

  async function handleComplete(finalNote: string) {
    setSubmitting(true);
    setError(null);
    try {
      await apiCall(`/api/inquiry-logs/${log.id}/seal`, 'POST', accessToken, { note: finalNote });
      setInquiryStatus('complete');
      closeModal();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Seal failed');
    } finally {
      setSubmitting(false);
    }
  }

  async function handleCreateAssessment(evidenceId: string, data: { relevance: number; reliability: number; credibility: string; recommendation: string; rationale: string }) {
    setSubmitting(true);
    setError(null);
    try {
      await apiCall(`/api/evidence/${evidenceId}/assessments`, 'POST', accessToken, {
        relevance_score: data.relevance,
        relevance_rationale: data.rationale,
        reliability_score: data.reliability,
        reliability_rationale: data.rationale,
        source_credibility: data.credibility.toLowerCase(),
        recommendation: data.recommendation.toLowerCase(),
      });
      closeModal();
      window.location.reload();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Assessment failed');
    } finally {
      setSubmitting(false);
    }
  }

  async function handleCreateVerification(evidenceId: string, data: { type: string; finding: string; confidence: string; tools: string; methodology: string }) {
    setSubmitting(true);
    setError(null);
    try {
      await apiCall(`/api/evidence/${evidenceId}/verifications`, 'POST', accessToken, {
        verification_type: data.type.toLowerCase().replace(/ /g, '_'),
        methodology: data.methodology,
        tools: data.tools,
        finding: data.finding.toLowerCase().replace(/ /g, '_'),
        confidence: data.confidence.toLowerCase(),
        finding_rationale: data.methodology,
      });
      closeModal();
      window.location.reload();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Verification failed');
    } finally {
      setSubmitting(false);
    }
  }

  async function handleCreateAnalysis(data: { title: string; content: string; limitations: string }) {
    setSubmitting(true);
    setError(null);
    try {
      await apiCall(`/api/cases/${log.case_id}/analysis-notes`, 'POST', accessToken, {
        title: data.title,
        analysis_type: 'other',
        methodology: '',
        content: data.content,
        limitations: data.limitations ? [data.limitations] : [],
        caveats: [],
        assumptions: [],
      });
      closeModal();
      window.location.reload();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Analysis note failed');
    } finally {
      setSubmitting(false);
    }
  }

  async function handleAddNote(note: string) {
    setSubmitting(true);
    setError(null);
    try {
      const existingNotes = log.notes || '';
      const updatedNotes = `${existingNotes}\n[${new Date().toISOString()}] ${note}`.trim();
      await apiCall(`/api/inquiry-logs/${log.id}`, 'PUT', accessToken, {
        search_strategy: log.search_strategy,
        search_keywords: log.search_keywords,
        search_tool: log.search_tool,
        search_started_at: log.search_started_at,
        objective: log.objective,
        notes: updatedNotes,
      });
      closeModal();
      window.location.reload();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Add note failed');
    } finally {
      setSubmitting(false);
    }
  }

  // Form state for modals
  const [lockReason, setLockReason] = useState('');
  const [completeNote, setCompleteNote] = useState('');
  const [noteText, setNoteText] = useState('');
  const [assessForm, setAssessForm] = useState({ relevance: '', reliability: '', credibility: 'Established', recommendation: 'Collect', rationale: '' });
  const [verifyForm, setVerifyForm] = useState({ type: 'Geolocation', finding: 'Authentic', confidence: 'High', tools: '', methodology: '' });
  const [analysisForm, setAnalysisForm] = useState({ title: '', content: '', limitations: '' });

  const inqId = log.id ? `INQ-${log.id.slice(0, 4)}` : INQ.id;
  const inqContent = log.objective || INQ.content;
  const inqAuthor = log.performed_by || INQ.author;
  const inqStrategy = log.search_strategy || INQ.strategy;
  const inqKeywords = log.search_keywords?.join(', ') || INQ.keywords;
  const inqTool = log.search_tool || INQ.tool;

  function statusPill() {
    if (inquiryStatus === 'complete') return <span className="pl sealed">Complete</span>;
    if (inquiryStatus === 'locked') return <span className="pl hold">Locked</span>;
    return <span className="pl live">Active</span>;
  }

  return (
    <>
      {/* Error banner */}
      {error && <div style={{ padding: '12px 18px', background: 'rgba(179,92,92,.06)', border: '1px solid rgba(179,92,92,.2)', borderRadius: 10, marginBottom: 16, fontSize: 13, color: '#b35c5c' }}>{error}</div>}

      {/* Header */}
      <section className="d-pagehead"><div>
        <span className="eyebrow-m">Case &middot; ICC-UKR-2024 &middot; Berkeley Protocol Phase 1</span>
        <h1><em>{inqId}</em> {statusPill()}</h1>
        <p className="sub">{inqContent}</p>
        <div style={{ display: 'flex', gap: 8, marginTop: 16, flexWrap: 'wrap' }}>
          <a className="btn ghost sm" href={`/en/cases/${log.case_id}?view=inquiry`}>&larr; Back to inquiry log</a>
          <a className="btn ghost sm" href="#">Export PDF</a>
          {isEditable && <button className="btn ghost sm" onClick={() => openModal('add-item')}>Link new item</button>}
          {isEditable && <button className="btn ghost sm" style={{ color: 'var(--accent)', borderColor: 'rgba(184,66,28,.3)' }} onClick={() => openModal('lock')}>Lock inquiry</button>}
          {inquiryStatus === 'locked' && <button className="btn sm" onClick={() => openModal('complete')}>Mark complete <span className="arr">&rarr;</span></button>}
          {inquiryStatus === 'complete' && <span style={{ fontSize: 13, color: 'var(--ok)', fontFamily: 'Fraunces,serif', fontStyle: 'italic' }}>{'\u2713'} Sealed &amp; complete</span>}
        </div>
      </div></section>

      {/* Status banner */}
      {inquiryStatus !== 'active' && (
        <div style={{ padding: '14px 20px', border: `1px solid ${inquiryStatus === 'complete' ? 'rgba(74,107,58,.3)' : 'rgba(184,66,28,.2)'}`, borderRadius: 12, background: inquiryStatus === 'complete' ? 'rgba(74,107,58,.03)' : 'rgba(184,66,28,.03)', marginBottom: 22, display: 'flex', alignItems: 'center', gap: 12 }}>
          <span style={{ fontSize: 18 }}>{inquiryStatus === 'complete' ? '\u2713' : '\ud83d\udd12'}</span>
          <div>
            <div style={{ fontSize: 14, fontWeight: 500, color: 'var(--ink)' }}>{inquiryStatus === 'complete' ? 'This inquiry is complete and sealed' : 'This inquiry is locked for review'}</div>
            <div style={{ fontSize: 12, color: 'var(--muted)', marginTop: 2 }}>{inquiryStatus === 'complete' ? 'No further edits. All linked items continue through their phases independently.' : 'No new items can be linked. Existing items continue through their phases. Lock can be lifted by the lead investigator.'}</div>
          </div>
          {inquiryStatus === 'locked' && <button onClick={() => setInquiryStatus('active')} style={{ marginLeft: 'auto', fontSize: 12, color: 'var(--accent)', cursor: 'pointer', fontFamily: 'Fraunces,serif', fontStyle: 'italic', flexShrink: 0, background: 'none', border: 'none' }}>Unlock &rarr;</button>}
        </div>
      )}

      {/* Metadata panels */}
      <div className="g2-wide" style={{ marginBottom: 22 }}>
        <div className="panel"><div className="panel-h"><h3>Inquiry <em>record</em></h3><span className="pl sealed">Sealed</span></div>
        <div className="panel-body"><div style={{ display: 'grid', gridTemplateColumns: '100px 1fr', gap: '6px 16px', fontSize: 13 }}>
          <span style={{ color: 'var(--muted)' }}>Author</span><span><strong>{inqAuthor}</strong> &middot; {INQ.role}</span>
          <span style={{ color: 'var(--muted)' }}>Date</span><span>{INQ.date}</span>
          <span style={{ color: 'var(--muted)' }}>Type</span><span className="pl sealed" style={{ width: 'fit-content' }}>{INQ.type}</span>
          <span style={{ color: 'var(--muted)' }}>Assigned</span><span>{INQ.assigned}</span>
          <span style={{ color: 'var(--muted)' }}>Signature</span><span style={{ fontFamily: 'JetBrains Mono,monospace', fontSize: 11 }}>{INQ.sig}</span>
          <span style={{ color: 'var(--muted)' }}>Block</span><span style={{ fontFamily: 'JetBrains Mono,monospace', fontSize: 11 }}>{INQ.block}</span>
        </div></div></div>

        <div className="panel"><div className="panel-h"><h3>Search <em>parameters</em></h3><span className="meta">Phase 1</span></div>
        <div className="panel-body"><div style={{ display: 'grid', gridTemplateColumns: '90px 1fr', gap: '6px 16px', fontSize: 13 }}>
          <span style={{ color: 'var(--muted)' }}>Strategy</span><span>{inqStrategy}</span>
          <span style={{ color: 'var(--muted)' }}>Keywords</span><span style={{ fontFamily: 'JetBrains Mono,monospace', fontSize: 12 }}>{inqKeywords}</span>
          <span style={{ color: 'var(--muted)' }}>Tool</span><span>{inqTool}</span>
          <span style={{ color: 'var(--muted)' }}>Period</span><span>{INQ.period}</span>
          <span style={{ color: 'var(--muted)' }}>Results</span><span><strong>{INQ.results.found}</strong> found &middot; <strong>{INQ.results.relevant}</strong> relevant &middot; <strong>{INQ.results.collected}</strong> collected</span>
        </div></div></div>
      </div>

      {/* Berkeley Protocol lifecycle */}
      <div className="panel" style={{ marginBottom: 22 }}>
        <div className="panel-h"><h3>Berkeley Protocol <em>lifecycle</em></h3><span className="meta">{ITEMS.length} items tracked across 6 phases</span></div>
        <div className="panel-body" style={{ padding: 0, overflowX: 'auto' }}>
          {ITEMS.map((item, idx) => {
            const phases = [1, 2, 3, 4, 5, 6];
            let maxPhase = 0;
            phases.forEach((n) => { if (phaseStatus(item, n) === 'done') maxPhase = n; });
            const isStopped = item.phase2?.done && (item.phase2.rec === 'Deprioritize' || item.phase2.rec === 'Discard' || item.phase2.rec === 'Monitor') && !item.phase3?.done;
            const isComplete = maxPhase === 6;
            const recClass = item.phase2?.done ? ({ Collect: 'sealed', Monitor: 'disc', Deprioritize: 'hold', Discard: 'broken' } as Record<string, string>)[item.phase2.rec!] || 'draft' : 'draft';

            return (
              <div key={idx} style={{ padding: '18px 22px', borderBottom: '1px solid var(--line)' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 14 }}>
                  <span style={{ fontFamily: 'JetBrains Mono,monospace', fontSize: 12, color: 'var(--accent)', letterSpacing: '.02em' }}>{item.ref}</span>
                  <span className="tag">{item.kind}</span>
                  <span style={{ fontSize: 15, fontWeight: 500, color: 'var(--ink)' }}>{item.name}</span>
                  {item.phase2?.done && <span className={`pl ${recClass}`} style={{ fontSize: 10 }}>{item.phase2.rec}</span>}
                  <span style={{ fontSize: 12, color: 'var(--muted)', marginLeft: 'auto' }}>{item.size}</span>
                  <a href="#" className="linkarrow" style={{ fontSize: 12 }}>Open &rarr;</a>
                </div>

                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(6,1fr)', gap: 0 }}>
                  {phases.map((n) => {
                    const st = phaseStatus(item, n);
                    const dotBg = st === 'done' ? 'var(--ok)' : st === 'active' ? 'var(--accent)' : 'var(--bg-2)';
                    const dotClr = st === 'done' ? '#fff' : st === 'active' ? '#fff' : 'var(--muted)';
                    const dotBdr = st === 'pending' ? '1.5px solid var(--line-2)' : 'none';
                    const icon = st === 'done' ? '\u2713' : String(n);
                    return (
                      <div key={n} style={{ padding: '10px 12px', borderRight: n < 6 ? '1px solid var(--line)' : 'none' }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 8 }}>
                          <span style={{ width: 22, height: 22, borderRadius: '50%', background: dotBg, color: dotClr, border: dotBdr, display: 'grid', placeItems: 'center', fontFamily: 'JetBrains Mono,monospace', fontSize: 10, flexShrink: 0 }}>{icon}</span>
                          <span style={{ fontFamily: 'JetBrains Mono,monospace', fontSize: 9, letterSpacing: '.06em', textTransform: 'uppercase', color: 'var(--muted)' }}>{PHASE_NAMES[n]}</span>
                        </div>
                        <PhaseDetail item={item} n={n} />
                      </div>
                    );
                  })}
                </div>

                <div style={{ marginTop: 14, paddingTop: 12, borderTop: '1px solid var(--line)', display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                  {isComplete ? (
                    <span style={{ fontSize: 12, color: 'var(--ok)', fontFamily: 'Fraunces,serif', fontStyle: 'italic' }}>{'\u2713'} All 6 phases complete</span>
                  ) : isStopped ? (<>
                    <span style={{ fontSize: 12, color: 'var(--muted)' }}>{item.phase2.rec} at Phase 2</span>
                    <button onClick={() => openModal('reactivate')} style={{ padding: '4px 12px', borderRadius: 6, background: 'rgba(184,66,28,.08)', fontSize: 12, color: 'var(--accent)', cursor: 'pointer', border: 'none' }}>Reactivate &rarr;</button>
                    <button onClick={() => openModal('note')} style={{ padding: '4px 12px', borderRadius: 6, background: 'var(--bg-2)', fontSize: 12, color: 'var(--ink-2)', cursor: 'pointer', border: 'none' }}>Add note</button>
                  </>) : (<>
                    <span style={{ fontSize: 12, color: 'var(--muted)' }}>Next:</span>
                    <button onClick={() => openModal(`advance-${maxPhase + 1}`)} style={{ padding: '4px 12px', borderRadius: 6, background: 'rgba(184,66,28,.08)', fontSize: 12, color: 'var(--accent)', cursor: 'pointer', border: 'none', fontFamily: 'Fraunces,serif', fontStyle: 'italic' }}>{PHASE_NAMES[maxPhase + 1]} &rarr;</button>
                  </>)}
                  <div style={{ marginLeft: 'auto', display: 'flex', gap: 6 }}>
                    <button onClick={() => openModal('note')} style={{ padding: '4px 10px', borderRadius: 6, border: '1px solid var(--line-2)', fontSize: 11, color: 'var(--muted)', cursor: 'pointer', background: 'none' }}>Add note</button>
                    <a href="#" style={{ padding: '4px 10px', borderRadius: 6, border: '1px solid var(--line-2)', fontSize: 11, color: 'var(--muted)', cursor: 'pointer', textDecoration: 'none' }}>View detail</a>
                    <button onClick={() => openModal('reassign')} style={{ padding: '4px 10px', borderRadius: 6, border: '1px solid var(--line-2)', fontSize: 11, color: 'var(--muted)', cursor: 'pointer', background: 'none' }}>Reassign</button>
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      </div>

      {/* Linked items */}
      <div className="g2">
        <div className="panel"><div className="panel-h"><h3>Witnesses</h3><span className="meta">2 linked</span></div>
        <div className="panel-body" style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          <a href="#" className="item-row"><div><span className="ir-ref">W-0144</span><div className="ir-name">Pseudonymised witness</div><div className="ir-meta">High risk &middot; voice-masked &middot; field intake</div></div><span className="linkarrow" style={{ fontSize: 12 }}>View &rarr;</span></a>
          <a href="#" className="item-row"><div><span className="ir-ref">W-0099</span><div className="ir-name">Pseudonymised witness</div><div className="ir-meta">Medium risk &middot; sworn deposition</div></div><span className="linkarrow" style={{ fontSize: 12 }}>View &rarr;</span></a>
        </div></div>
        <div className="panel"><div className="panel-h"><h3>Disclosures &amp; reports</h3><span className="meta">1 pending</span></div>
        <div className="panel-body" style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          <a href="#" className="item-row"><div><span className="ir-ref">DISC-2026-019</span><div className="ir-name">Defence disclosure bundle</div><div className="ir-meta">48 exhibits &middot; pending countersign &middot; due Fri 25 Apr</div></div><span className="pl hold" style={{ fontSize: 10 }}>Review</span></a>
          <a href="#" className="item-row"><div><span className="ir-ref">C-0412</span><div className="ir-name">S-038 gave checkpoint order at 17:52</div><div className="ir-meta">Score 0.91 &middot; corroborated &middot; 4 sources</div></div><span className="pl sealed" style={{ fontSize: 10 }}>Corroborated</span></a>
        </div></div>
      </div>

      {/* Modal: Lock */}
      <VKModal open={activeModal === 'lock'} onClose={closeModal}>
        <h3>Lock <em>inquiry</em></h3>
        <p className="msub">Locking prevents new items from being linked to this inquiry. Existing items continue through their Berkeley Protocol phases independently. The lock can be lifted by the lead investigator.</p>
        <div className="mfield"><label>Reason <small>required</small></label><textarea placeholder="e.g. All relevant items identified and assessed. Ready for review before sealing." value={lockReason} onChange={(e) => setLockReason(e.target.value)} /></div>
        <div className="mfield"><label>Notify</label><div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}><span className="chip active" style={{ cursor: 'pointer' }}>Case team</span><span className="chip" style={{ cursor: 'pointer' }}>Lead only</span></div></div>
        <div className="mactions"><button className="btn ghost sm" onClick={closeModal} disabled={submitting}>Cancel</button><button className="btn sm" style={{ color: 'var(--accent)', background: 'rgba(184,66,28,.08)', borderColor: 'rgba(184,66,28,.2)' }} onClick={() => handleLock(lockReason)} disabled={submitting || !lockReason.trim()}>{submitting ? 'Locking\u2026' : 'Lock inquiry'}</button></div>
      </VKModal>

      {/* Modal: Complete */}
      <VKModal open={activeModal === 'complete'} onClose={closeModal}>
        <h3>Mark <em>complete</em></h3>
        <p className="msub">Completing an inquiry seals it permanently. The record, all search parameters, and linked items become immutable. This action is signed and cannot be reversed.</p>
        <div style={{ padding: 14, border: '1px solid var(--line)', borderRadius: 10, background: 'var(--bg-2)', marginBottom: 16 }}>
          <div style={{ fontFamily: 'JetBrains Mono,monospace', fontSize: 9, letterSpacing: '.08em', textTransform: 'uppercase', color: 'var(--muted)', marginBottom: 8 }}>Summary</div>
          <div style={{ display: 'grid', gridTemplateColumns: '120px 1fr', gap: '4px 12px', fontSize: 13 }}>
            <span style={{ color: 'var(--muted)' }}>Items tracked</span><span>{ITEMS.length}</span>
            <span style={{ color: 'var(--muted)' }}>Fully complete</span><span>{ITEMS.filter((it, _i) => { let m = 0; [1,2,3,4,5,6].forEach(n => { if (phaseStatus(it, n) === 'done') m = n; }); return m === 6; }).length}</span>
            <span style={{ color: 'var(--muted)' }}>In progress</span><span>{ITEMS.filter(it => it.phase2?.done && it.phase2.rec === 'Collect' && !it.phase6?.done).length}</span>
            <span style={{ color: 'var(--muted)' }}>Stopped</span><span>{ITEMS.filter(it => it.phase2?.done && (it.phase2.rec === 'Deprioritize' || it.phase2.rec === 'Discard' || it.phase2.rec === 'Monitor') && !it.phase3?.done).length}</span>
          </div>
        </div>
        <div style={{ padding: 14, border: '2px solid rgba(74,107,58,.3)', borderRadius: 10, background: 'rgba(74,107,58,.03)', marginBottom: 16 }}>
          <div style={{ fontSize: 13, color: 'var(--ink-2)', lineHeight: 1.55 }}>Once sealed, this inquiry and its search parameters become part of the permanent investigative record. Linked items continue through their phases independently.</div>
        </div>
        <div className="mfield"><label>Final note <small>optional &mdash; sealed with the inquiry</small></label><textarea placeholder="Any final observations about this inquiry..." value={completeNote} onChange={(e) => setCompleteNote(e.target.value)} /></div>
        <div className="mactions"><button className="btn ghost sm" onClick={closeModal} disabled={submitting}>Cancel</button><button className="btn sm" onClick={() => handleComplete(completeNote)} disabled={submitting}>{submitting ? 'Sealing\u2026' : 'Sign & seal as complete'}</button></div>
      </VKModal>

      {/* Modal: Link item */}
      <VKModal open={activeModal === 'add-item'} onClose={closeModal}>
        <h3>Link <em>item</em> to inquiry</h3>
        <p className="msub">Add an existing exhibit, witness, or claim to this inquiry&rsquo;s tracking. The item will appear in the phase lifecycle above.</p>
        <div className="mfield"><label>Item type</label><div style={{ display: 'flex', gap: 6 }}><span className="chip active" style={{ cursor: 'pointer' }}>Evidence</span><span className="chip" style={{ cursor: 'pointer' }}>Witness</span><span className="chip" style={{ cursor: 'pointer' }}>Corroboration</span><span className="chip" style={{ cursor: 'pointer' }}>Disclosure</span></div></div>
        <div className="mfield"><label>Search</label><input type="text" placeholder="Search by ID, filename, hash, or name..." /></div>
        <div className="mfield"><label>Role in inquiry</label><select><option>Primary evidence</option><option>Supporting</option><option>Contextual</option><option>Contradicting</option></select></div>
        <div className="mfield"><label>Note</label><textarea placeholder="Why is this item linked to this inquiry?" /></div>
        <div className="mactions"><button className="btn ghost sm" onClick={closeModal}>Cancel</button><button className="btn sm" onClick={closeModal}>Link item</button></div>
      </VKModal>

      {/* Modal: Reactivate */}
      <VKModal open={activeModal === 'reactivate'} onClose={closeModal}>
        <h3>Reactivate <em>item</em></h3>
        <p className="msub">This item was previously deprioritized, discarded, or set to monitor. Reactivating moves it back into the active pipeline for collection.</p>
        <div className="mfield"><label>New recommendation</label><select><option>Collect &mdash; proceed to Phase 3</option><option>Monitor &mdash; keep watching</option></select></div>
        <div className="mfield"><label>Reason for reactivation <small>required</small></label><textarea placeholder="e.g. New corroborating evidence found, second interview completed..." /></div>
        <div className="mfield"><label>Assigned to</label><select><option value="">Select team member...</option><option>H. Morel</option><option>Amir Haddad</option><option>Martyna Kovacs</option><option>Juliane Wirth</option><option>W. Nyoka</option></select></div>
        <div className="mactions"><button className="btn ghost sm" onClick={closeModal}>Cancel</button><button className="btn sm" onClick={closeModal}>Reactivate &amp; advance</button></div>
      </VKModal>

      {/* Modal: Add note */}
      <VKModal open={activeModal === 'note'} onClose={closeModal}>
        <h3>Add <em>note</em></h3>
        <p className="msub">Add a timestamped note to this item&rsquo;s record. Notes are sealed to the chain and visible to all case team members.</p>
        <div className="mfield"><label>Note <small>required</small></label><textarea placeholder="e.g. Second interview completed with W-0146. Transcript ready for upload." value={noteText} onChange={(e) => setNoteText(e.target.value)} /></div>
        <div className="mfield"><label>Update status?</label><div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}><span className="chip active" style={{ cursor: 'pointer' }}>No change</span><span className="chip" style={{ cursor: 'pointer' }}>Ready for next phase</span><span className="chip" style={{ cursor: 'pointer' }}>Blocked</span><span className="chip" style={{ cursor: 'pointer' }}>Needs review</span></div></div>
        <div className="mactions"><button className="btn ghost sm" onClick={closeModal} disabled={submitting}>Cancel</button><button className="btn sm" onClick={() => handleAddNote(noteText)} disabled={submitting || !noteText.trim()}>{submitting ? 'Adding\u2026' : 'Add note'}</button></div>
      </VKModal>

      {/* Modal: Reassign */}
      <VKModal open={activeModal === 'reassign'} onClose={closeModal}>
        <h3>Reassign <em>item</em></h3>
        <p className="msub">Transfer responsibility for the next phase action to another team member.</p>
        <div className="mfield"><label>Assign to</label><select><option value="">Select team member...</option><option>H. Morel &mdash; Lead Investigator</option><option>Amir Haddad &mdash; Evidence Technician</option><option>Martyna Kovacs &mdash; Redaction Analyst</option><option>Juliane Wirth &mdash; Witness Liaison</option><option>W. Nyoka &mdash; External Liaison</option></select></div>
        <div className="mfield"><label>Note <small>optional</small></label><textarea placeholder="Any context for the handoff..." /></div>
        <div className="mactions"><button className="btn ghost sm" onClick={closeModal}>Cancel</button><button className="btn sm" onClick={closeModal}>Reassign</button></div>
      </VKModal>

      {/* Modal: Advance Phase 2 */}
      <VKModal open={activeModal === 'advance-2'} onClose={closeModal}>
        <h3>Assess <em>exhibit</em> <span className="mphase">Phase 2</span></h3>
        <p className="msub">Score this item&rsquo;s relevance and reliability to advance it to Phase 2.</p>
        <div className="mgrid mgrid-2"><div className="mfield"><label>Relevance <small>1&ndash;10</small></label><input type="number" min={1} max={10} placeholder="Score" value={assessForm.relevance} onChange={(e) => setAssessForm({ ...assessForm, relevance: e.target.value })} /></div><div className="mfield"><label>Reliability <small>1&ndash;10</small></label><input type="number" min={1} max={10} placeholder="Score" value={assessForm.reliability} onChange={(e) => setAssessForm({ ...assessForm, reliability: e.target.value })} /></div></div>
        <div className="mgrid mgrid-2"><div className="mfield"><label>Source credibility</label><select value={assessForm.credibility} onChange={(e) => setAssessForm({ ...assessForm, credibility: e.target.value })}><option>Established</option><option>Probable</option><option>Unconfirmed</option><option>Doubtful</option></select></div><div className="mfield"><label>Recommendation</label><select value={assessForm.recommendation} onChange={(e) => setAssessForm({ ...assessForm, recommendation: e.target.value })}><option>Collect</option><option>Monitor</option><option>Deprioritize</option><option>Discard</option></select></div></div>
        <div className="mfield"><label>Rationale <small>required</small></label><textarea placeholder="Explain your scoring..." value={assessForm.rationale} onChange={(e) => setAssessForm({ ...assessForm, rationale: e.target.value })} /></div>
        <div className="mactions"><button className="btn ghost sm" onClick={closeModal} disabled={submitting}>Cancel</button><button className="btn sm" onClick={() => handleCreateAssessment(log.evidence_id || ITEMS[0].ref, { relevance: Number(assessForm.relevance), reliability: Number(assessForm.reliability), credibility: assessForm.credibility, recommendation: assessForm.recommendation, rationale: assessForm.rationale })} disabled={submitting || !assessForm.relevance || !assessForm.reliability || !assessForm.rationale.trim()}>{submitting ? 'Submitting\u2026' : 'Complete Phase 2'}</button></div>
      </VKModal>

      {/* Modal: Advance Phase 3 */}
      <VKModal open={activeModal === 'advance-3'} onClose={closeModal}>
        <h3>Collect <em>exhibit</em> <span className="mphase">Phase 3</span></h3>
        <p className="msub">Upload the exhibit with Berkeley Protocol capture metadata.</p>
        <div style={{ border: '2px dashed var(--line-2)', borderRadius: 12, padding: '32px 20px', textAlign: 'center', marginBottom: 14 }}><div style={{ fontSize: 24, color: 'var(--accent)', marginBottom: 6 }}>&uarr;</div><div style={{ fontSize: 14, fontWeight: 500 }}>Drop file or click to browse</div><div style={{ fontSize: 12, color: 'var(--muted)' }}>SHA-256 + BLAKE3 hashed client-side</div></div>
        <div className="mgrid mgrid-2"><div className="mfield"><label>Source platform</label><select><option>Telegram</option><option>Twitter/X</option><option>Website</option><option>Physical transfer</option><option>Other</option></select></div><div className="mfield"><label>Capture method</label><select><option>Manual download</option><option>Screenshot</option><option>Web archive</option><option>Forensic tool</option><option>Physical transfer</option></select></div></div>
        <div className="mgrid mgrid-2"><div className="mfield"><label>Geolocation</label><input type="text" placeholder="Place name" /></div><div className="mfield"><label>Content language</label><input type="text" placeholder="BCP 47, e.g. uk, ru" /></div></div>
        <div className="mactions"><button className="btn ghost sm" onClick={closeModal}>Cancel</button><button className="btn sm" onClick={closeModal}>Upload &amp; seal</button></div>
      </VKModal>

      {/* Modal: Advance Phase 5 */}
      <VKModal open={activeModal === 'advance-5'} onClose={closeModal}>
        <h3>Verify <em>exhibit</em> <span className="mphase">Phase 5</span></h3>
        <p className="msub">Create a verification record for this exhibit.</p>
        <div className="mgrid mgrid-2"><div className="mfield"><label>Verification type</label><select value={verifyForm.type} onChange={(e) => setVerifyForm({ ...verifyForm, type: e.target.value })}><option>Geolocation</option><option>Chronolocation</option><option>Source authentication</option><option>Content authenticity</option><option>Reverse image search</option><option>Forensic audio</option></select></div><div className="mfield"><label>Finding</label><select value={verifyForm.finding} onChange={(e) => setVerifyForm({ ...verifyForm, finding: e.target.value })}><option>Authentic</option><option>Likely authentic</option><option>Inconclusive</option><option>Likely inauthentic</option></select></div></div>
        <div className="mgrid mgrid-2"><div className="mfield"><label>Confidence</label><select value={verifyForm.confidence} onChange={(e) => setVerifyForm({ ...verifyForm, confidence: e.target.value })}><option>High</option><option>Medium</option><option>Low</option></select></div><div className="mfield"><label>Tools used</label><input type="text" placeholder="e.g. Google Earth, SunCalc" value={verifyForm.tools} onChange={(e) => setVerifyForm({ ...verifyForm, tools: e.target.value })} /></div></div>
        <div className="mfield"><label>Methodology &amp; rationale</label><textarea placeholder="Describe verification approach and findings..." value={verifyForm.methodology} onChange={(e) => setVerifyForm({ ...verifyForm, methodology: e.target.value })} /></div>
        <div className="mactions"><button className="btn ghost sm" onClick={closeModal} disabled={submitting}>Cancel</button><button className="btn sm" onClick={() => handleCreateVerification(log.evidence_id || ITEMS[0].ref, verifyForm)} disabled={submitting || !verifyForm.methodology.trim()}>{submitting ? 'Submitting\u2026' : 'Complete verification'}</button></div>
      </VKModal>

      {/* Modal: Advance Phase 6 */}
      <VKModal open={activeModal === 'advance-6'} onClose={closeModal}>
        <h3>Analyse <em>exhibit</em> <span className="mphase">Phase 6</span></h3>
        <p className="msub">Create an analysis note linking this exhibit to the investigative reasoning.</p>
        <div className="mfield"><label>Analysis title</label><input type="text" placeholder="e.g. Timeline reconciliation — checkpoint events" value={analysisForm.title} onChange={(e) => setAnalysisForm({ ...analysisForm, title: e.target.value })} /></div>
        <div className="mfield"><label>Analysis type</label><div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}><span className="chip" style={{ cursor: 'pointer' }}>Hypothesis</span><span className="chip" style={{ cursor: 'pointer' }}>Timeline</span><span className="chip" style={{ cursor: 'pointer' }}>Command structure</span><span className="chip" style={{ cursor: 'pointer' }}>Counter-evidence</span><span className="chip" style={{ cursor: 'pointer' }}>OSINT</span></div></div>
        <div className="mfield"><label>Content <small>required</small></label><textarea style={{ minHeight: 100 }} placeholder="Document your analytical reasoning..." value={analysisForm.content} onChange={(e) => setAnalysisForm({ ...analysisForm, content: e.target.value })} /></div>
        <div className="mfield"><label>Limitations</label><textarea placeholder="Caveats, assumptions, gaps..." value={analysisForm.limitations} onChange={(e) => setAnalysisForm({ ...analysisForm, limitations: e.target.value })} /></div>
        <div className="mactions"><button className="btn ghost sm" onClick={closeModal} disabled={submitting}>Cancel</button><button className="btn ghost sm" onClick={closeModal} disabled={submitting}>Save draft</button><button className="btn sm" onClick={() => handleCreateAnalysis(analysisForm)} disabled={submitting || !analysisForm.title.trim() || !analysisForm.content.trim()}>{submitting ? 'Sealing\u2026' : 'Sign & seal note'}</button></div>
      </VKModal>
    </>
  );
}
