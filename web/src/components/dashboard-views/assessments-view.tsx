'use client';

import { useState } from 'react';
import type { EvidenceAssessment, Recommendation, SourceCredibility } from '@/types';
import {
  KPIStrip,
  Panel,
  FilterBar,
  Modal,
  StatusPill,
  Tag,
  BPIndicator,
  LinkArrow,
  EyebrowLabel,
} from '@/components/ui/dashboard';

// --- Types ---

interface UnassessedItem {
  readonly id: string;
  readonly ref: string;
  readonly filename: string;
  readonly type: string;
  readonly uploadedBy: string;
  readonly uploadedAt: string;
  readonly size: string;
  readonly source: string;
  readonly method: string;
  readonly location: string;
  readonly coordinates: string;
  readonly hash: string;
  readonly custodyStatus: string;
  readonly tags: string[];
}

interface CompletedAssessment {
  readonly id: string;
  readonly evidenceRef: string;
  readonly evidenceName: string;
  readonly relevanceScore: number;
  readonly reliabilityScore: number;
  readonly recommendation: Recommendation;
  readonly assessor: string;
  readonly assessedAt: string;
  readonly rationale: string;
  readonly bpPhases: {
    readonly name: string;
    readonly status: 'complete' | 'in_progress' | 'not_started';
  }[];
  readonly bpPhaseLabel: string;
}

// --- Stub data ---

const UNASSESSED_ITEMS: readonly UnassessedItem[] = [
  {
    id: 'UA-001',
    ref: 'VK-EX-0934',
    filename: 'drone_footage_north_sector.mp4',
    type: 'video/mp4',
    uploadedBy: 'Lt. Hana Osman',
    uploadedAt: '18 Apr 2026',
    size: '2.4 GB',
    source: 'Field unit Alpha-3',
    method: 'Direct capture',
    location: 'North sector, Zone B',
    coordinates: '48.8566 N, 2.3522 E',
    hash: 'sha256:9f3c…e71a',
    custodyStatus: 'Sealed',
    tags: ['aerial', 'infrastructure'],
  },
  {
    id: 'UA-002',
    ref: 'VK-EX-0935',
    filename: 'witness_transcript_abara.pdf',
    type: 'application/pdf',
    uploadedBy: 'R. Castillo',
    uploadedAt: '17 Apr 2026',
    size: '840 KB',
    source: 'Interview room 3',
    method: 'Transcription',
    location: 'Field office, Sector HQ',
    coordinates: '48.8601 N, 2.3376 E',
    hash: 'sha256:a1b2…c3d4',
    custodyStatus: 'Sealed',
    tags: ['testimony', 'primary'],
  },
  {
    id: 'UA-003',
    ref: 'VK-EX-0936',
    filename: 'satellite_overlay_march.tiff',
    type: 'image/tiff',
    uploadedBy: 'M. Petrov',
    uploadedAt: '16 Apr 2026',
    size: '1.1 GB',
    source: 'Partner agency',
    method: 'Satellite acquisition',
    location: 'Region 4 overview',
    coordinates: '48.8490 N, 2.3412 E',
    hash: 'sha256:d4e5…f6a7',
    custodyStatus: 'Hold',
    tags: ['satellite', 'geospatial'],
  },
  {
    id: 'UA-004',
    ref: 'VK-EX-0937',
    filename: 'communications_intercept_12.wav',
    type: 'audio/wav',
    uploadedBy: 'J. Nakamura',
    uploadedAt: '15 Apr 2026',
    size: '340 MB',
    source: 'Signals intelligence',
    method: 'Intercept',
    location: 'Undisclosed',
    coordinates: 'Redacted',
    hash: 'sha256:b8c9…d0e1',
    custodyStatus: 'Sealed',
    tags: ['audio', 'comms'],
  },
];

const COMPLETED_ASSESSMENTS: readonly CompletedAssessment[] = [
  {
    id: 'AS-001',
    evidenceRef: 'VK-EX-0891',
    evidenceName: 'Aerial survey — compound perimeter',
    relevanceScore: 9,
    reliabilityScore: 8,
    recommendation: 'collect',
    assessor: 'Dr. Elena Vasquez',
    assessedAt: '14 Apr 2026',
    rationale:
      'High-resolution imagery clearly shows structural changes consistent with the alleged timeline. Corroborates witness testimony from Phase 1 interviews.',
    bpPhases: [
      { name: 'Survey', status: 'complete' },
      { name: 'Assess', status: 'complete' },
      { name: 'Collect', status: 'in_progress' },
      { name: 'Verify', status: 'not_started' },
      { name: 'Analyse', status: 'not_started' },
      { name: 'Report', status: 'not_started' },
    ],
    bpPhaseLabel: 'Phase 3',
  },
  {
    id: 'AS-002',
    evidenceRef: 'VK-EX-0887',
    evidenceName: 'Financial transfer records — Jan to Mar',
    relevanceScore: 7,
    reliabilityScore: 9,
    recommendation: 'collect',
    assessor: 'R. Castillo',
    assessedAt: '13 Apr 2026',
    rationale:
      'Bank records verified through two independent sources. Transaction patterns align with suspected supply chain. Recommend priority collection.',
    bpPhases: [
      { name: 'Survey', status: 'complete' },
      { name: 'Assess', status: 'complete' },
      { name: 'Collect', status: 'complete' },
      { name: 'Verify', status: 'in_progress' },
      { name: 'Analyse', status: 'not_started' },
      { name: 'Report', status: 'not_started' },
    ],
    bpPhaseLabel: 'Phase 4',
  },
  {
    id: 'AS-003',
    evidenceRef: 'VK-EX-0874',
    evidenceName: 'Social media archive — suspect account',
    relevanceScore: 6,
    reliabilityScore: 4,
    recommendation: 'monitor',
    assessor: 'Lt. Hana Osman',
    assessedAt: '12 Apr 2026',
    rationale:
      'Content relevant but source authenticity cannot be fully established. Account may have been compromised. Recommend continued monitoring before collection.',
    bpPhases: [
      { name: 'Survey', status: 'complete' },
      { name: 'Assess', status: 'complete' },
      { name: 'Collect', status: 'not_started' },
      { name: 'Verify', status: 'not_started' },
      { name: 'Analyse', status: 'not_started' },
      { name: 'Report', status: 'not_started' },
    ],
    bpPhaseLabel: 'Phase 2',
  },
  {
    id: 'AS-004',
    evidenceRef: 'VK-EX-0862',
    evidenceName: 'Vehicle registration database extract',
    relevanceScore: 3,
    reliabilityScore: 7,
    recommendation: 'deprioritize',
    assessor: 'M. Petrov',
    assessedAt: '11 Apr 2026',
    rationale:
      'Registration data is reliable but only tangentially connected to the primary investigation. Low relevance to the central allegations. May revisit if scope expands.',
    bpPhases: [
      { name: 'Survey', status: 'complete' },
      { name: 'Assess', status: 'complete' },
      { name: 'Collect', status: 'not_started' },
      { name: 'Verify', status: 'not_started' },
      { name: 'Analyse', status: 'not_started' },
      { name: 'Report', status: 'not_started' },
    ],
    bpPhaseLabel: 'Phase 2',
  },
  {
    id: 'AS-005',
    evidenceRef: 'VK-EX-0851',
    evidenceName: 'Anonymous tip — unverified document',
    relevanceScore: 2,
    reliabilityScore: 2,
    recommendation: 'discard',
    assessor: 'Dr. Elena Vasquez',
    assessedAt: '10 Apr 2026',
    rationale:
      'Document origin unknown. Metadata inconsistent with claimed date. Multiple indicators of fabrication detected. Preserved with full discard rationale per protocol.',
    bpPhases: [
      { name: 'Survey', status: 'complete' },
      { name: 'Assess', status: 'complete' },
      { name: 'Collect', status: 'not_started' },
      { name: 'Verify', status: 'not_started' },
      { name: 'Analyse', status: 'not_started' },
      { name: 'Report', status: 'not_started' },
    ],
    bpPhaseLabel: 'Phase 2',
  },
  {
    id: 'AS-006',
    evidenceRef: 'VK-EX-0843',
    evidenceName: 'Geolocation verification — checkpoint B',
    relevanceScore: 8,
    reliabilityScore: 7,
    recommendation: 'collect',
    assessor: 'J. Nakamura',
    assessedAt: '9 Apr 2026',
    rationale:
      'Geolocation data strongly corroborated by satellite imagery and ground reports. Timestamps verified against three independent sources.',
    bpPhases: [
      { name: 'Survey', status: 'complete' },
      { name: 'Assess', status: 'complete' },
      { name: 'Collect', status: 'complete' },
      { name: 'Verify', status: 'complete' },
      { name: 'Analyse', status: 'in_progress' },
      { name: 'Report', status: 'not_started' },
    ],
    bpPhaseLabel: 'Phase 5',
  },
];

// --- Helpers ---

const _REC_COLORS: Record<Recommendation, string> = {
  collect: 'var(--ok)',
  monitor: 'var(--accent)',
  deprioritize: '#b35c5c',
  discard: '#6b3a4a',
};

const _REC_BGS: Record<Recommendation, string> = {
  collect: 'rgba(74,107,58,.1)',
  monitor: 'rgba(184,66,28,.1)',
  deprioritize: 'rgba(179,92,92,.1)',
  discard: 'rgba(107,58,74,.1)',
};

const CREDIBILITY_OPTIONS: { value: SourceCredibility; label: string }[] = [
  { value: 'established', label: 'Established' },
  { value: 'credible', label: 'Probable' },
  { value: 'uncertain', label: 'Unconfirmed' },
  { value: 'unreliable', label: 'Doubtful' },
];

const RECOMMENDATION_OPTIONS: { value: Recommendation; label: string }[] = [
  { value: 'collect', label: 'Collect' },
  { value: 'monitor', label: 'Monitor' },
  { value: 'deprioritize', label: 'Deprioritize' },
  { value: 'discard', label: 'Discard' },
];

function scoreColor(score: number): string {
  if (score >= 7) return 'var(--ok)';
  if (score >= 5) return 'var(--accent)';
  return '#b35c5c';
}

function typeGlyph(type: string): string {
  if (type.startsWith('video/')) return '\u25B8';
  if (type.startsWith('audio/')) return '\u266A';
  if (type.startsWith('image/')) return '\u25E9';
  return '\u00B6';
}

// --- Guide steps ---

const HOW_TO_ASSESS_STEPS = [
  {
    num: 1,
    title: 'Review the exhibit',
    desc: 'Open the evidence item and examine its content, metadata, and chain of custody status.',
  },
  {
    num: 2,
    title: 'Score relevance (1\u201310)',
    desc: 'How directly does this evidence relate to the core allegations and investigative questions?',
  },
  {
    num: 3,
    title: 'Score reliability (1\u201310)',
    desc: 'Evaluate source credibility, chain of custody integrity, and corroboration from independent sources.',
  },
  {
    num: 4,
    title: 'Assess source credibility',
    desc: 'Classify the source as Established, Probable, Unconfirmed, or Doubtful based on track record and verifiability.',
  },
  {
    num: 5,
    title: 'Make a recommendation',
    desc: 'Collect, Monitor, Deprioritize, or Discard. Every recommendation requires a signed rationale.',
  },
  {
    num: 6,
    title: 'Document rationale',
    desc: 'Provide clear reasoning for your scores and recommendation. Flag any misleading indicators detected.',
  },
];

// --- Sub-components ---

function UnassessedQueue({
  items,
  onSelect,
}: {
  readonly items: readonly UnassessedItem[];
  readonly onSelect: (item: UnassessedItem) => void;
}) {
  return (
    <Panel title="Unassessed" titleAccent="queue" meta={`${items.length} awaiting`}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 0 }}>
        {items.map((item) => (
          <button
            key={item.id}
            type="button"
            onClick={() => onSelect(item)}
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 14,
              padding: '14px 4px',
              borderBottom: '1px solid var(--line)',
              background: 'none',
              border: 'none',
              borderBlockEnd: '1px solid var(--line)',
              cursor: 'pointer',
              textAlign: 'left',
              width: '100%',
              transition: 'background .12s',
            }}
          >
            <span
              style={{
                width: 34,
                height: 34,
                borderRadius: 8,
                background: 'var(--bg-2)',
                display: 'grid',
                placeItems: 'center',
                fontSize: 16,
                color: 'var(--muted)',
                flexShrink: 0,
              }}
            >
              {typeGlyph(item.type)}
            </span>
            <div style={{ flex: 1, minWidth: 0 }}>
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 8,
                  marginBottom: 2,
                }}
              >
                <span
                  style={{
                    fontFamily: '"JetBrains Mono", monospace',
                    fontSize: 11,
                    color: 'var(--accent)',
                    letterSpacing: '.02em',
                  }}
                >
                  {item.ref}
                </span>
                <span
                  style={{
                    fontSize: 13,
                    fontWeight: 500,
                    color: 'var(--ink)',
                    whiteSpace: 'nowrap',
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                  }}
                >
                  {item.filename}
                </span>
              </div>
              <div
                style={{
                  fontSize: 12,
                  color: 'var(--muted)',
                }}
              >
                {item.uploadedBy} &middot; {item.uploadedAt}
              </div>
            </div>
            <span
              style={{
                fontSize: 14,
                color: 'var(--muted)',
                flexShrink: 0,
              }}
            >
              &rarr;
            </span>
          </button>
        ))}
      </div>
    </Panel>
  );
}

function HowToAssessGuide() {
  return (
    <Panel title="How to" titleAccent="assess">
      <div style={{ display: 'flex', flexDirection: 'column', gap: 18 }}>
        {HOW_TO_ASSESS_STEPS.map((step) => (
          <div key={step.num} style={{ display: 'flex', gap: 14, alignItems: 'flex-start' }}>
            <span
              style={{
                width: 28,
                height: 28,
                borderRadius: '50%',
                background: 'var(--bg-2)',
                color: 'var(--ink)',
                display: 'grid',
                placeItems: 'center',
                fontFamily: '"JetBrains Mono", monospace',
                fontSize: 12,
                fontWeight: 600,
                flexShrink: 0,
              }}
            >
              {step.num}
            </span>
            <div>
              <div
                style={{
                  fontFamily: '"Fraunces", serif',
                  fontSize: 15,
                  letterSpacing: '-.01em',
                  marginBottom: 3,
                }}
              >
                {step.title}
              </div>
              <div
                style={{
                  fontSize: 13,
                  color: 'var(--muted)',
                  lineHeight: 1.55,
                }}
              >
                {step.desc}
              </div>
            </div>
          </div>
        ))}
      </div>
    </Panel>
  );
}

function AssessmentModalLeft({ item }: { readonly item: UnassessedItem }) {
  const previewBg = item.type.startsWith('video/')
    ? 'linear-gradient(135deg, var(--bg-2) 0%, rgba(184,66,28,.06) 100%)'
    : item.type.startsWith('image/')
      ? 'linear-gradient(135deg, var(--bg-2) 0%, rgba(74,107,58,.06) 100%)'
      : 'var(--bg-2)';

  const metaRows: [string, string][] = [
    ['Exhibit ID', item.ref],
    ['Filename', item.filename],
    ['Size', item.size],
    ['Uploaded by', item.uploadedBy],
    ['Date', item.uploadedAt],
    ['Source', item.source],
    ['Method', item.method],
    ['Location', item.location],
    ['Coordinates', item.coordinates],
    ['Hash', item.hash],
    ['Custody status', item.custodyStatus],
  ];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 18 }}>
      {/* Preview area */}
      <div
        style={{
          background: previewBg,
          borderRadius: 10,
          height: 180,
          display: 'grid',
          placeItems: 'center',
          fontSize: 40,
          color: 'var(--muted)',
        }}
      >
        {typeGlyph(item.type)}
      </div>

      {/* Metadata grid */}
      <div>
        <EyebrowLabel>Exhibit metadata</EyebrowLabel>
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: '1fr 1fr',
            gap: '8px 16px',
            marginTop: 10,
          }}
        >
          {metaRows.map(([label, value]) => (
            <div key={label}>
              <div
                style={{
                  fontFamily: '"JetBrains Mono", monospace',
                  fontSize: 9,
                  letterSpacing: '.08em',
                  textTransform: 'uppercase',
                  color: 'var(--muted)',
                  marginBottom: 2,
                }}
              >
                {label}
              </div>
              <div
                style={{
                  fontSize: 12.5,
                  color: 'var(--ink)',
                  wordBreak: 'break-all',
                }}
              >
                {value}
              </div>
            </div>
          ))}
        </div>

        {/* Tags */}
        <div style={{ display: 'flex', gap: 6, marginTop: 12, flexWrap: 'wrap' }}>
          {item.tags.map((t) => (
            <Tag key={t}>{t}</Tag>
          ))}
        </div>
      </div>

      <LinkArrow href="#">Open full evidence detail</LinkArrow>
    </div>
  );
}

function AssessmentModalRight({ onClose }: { readonly onClose: () => void }) {
  const fieldLabel: React.CSSProperties = {
    fontFamily: '"JetBrains Mono", monospace',
    fontSize: 10,
    letterSpacing: '.08em',
    textTransform: 'uppercase',
    color: 'var(--muted)',
    marginBottom: 6,
    display: 'block',
  };

  const inputStyle: React.CSSProperties = {
    width: '100%',
    padding: '8px 12px',
    borderRadius: 8,
    border: '1px solid var(--line)',
    background: 'var(--paper)',
    fontSize: 13,
    color: 'var(--ink)',
    fontFamily: 'inherit',
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 18 }}>
      {/* Score pair */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
        <div>
          <span style={fieldLabel}>Relevance score (1-10)</span>
          <input type="number" min={1} max={10} defaultValue={7} style={inputStyle} />
        </div>
        <div>
          <span style={fieldLabel}>Reliability score (1-10)</span>
          <input type="number" min={1} max={10} defaultValue={6} style={inputStyle} />
        </div>
      </div>

      {/* Source credibility */}
      <div>
        <span style={fieldLabel}>Source credibility</span>
        <select style={inputStyle} defaultValue="established">
          {CREDIBILITY_OPTIONS.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          ))}
        </select>
      </div>

      {/* Recommendation */}
      <div>
        <span style={fieldLabel}>Recommendation</span>
        <select style={inputStyle} defaultValue="collect">
          {RECOMMENDATION_OPTIONS.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          ))}
        </select>
      </div>

      {/* Assigned to */}
      <div>
        <span style={fieldLabel}>Assigned to</span>
        <input type="text" defaultValue="Dr. Elena Vasquez" style={inputStyle} />
      </div>

      {/* Rationale */}
      <div>
        <span style={fieldLabel}>Rationale</span>
        <textarea
          rows={4}
          placeholder="Provide reasoning for your relevance and reliability scores, and your recommendation..."
          style={{ ...inputStyle, resize: 'vertical' }}
        />
      </div>

      {/* Misleading indicators */}
      <div>
        <span style={fieldLabel}>Misleading indicators</span>
        <textarea
          rows={2}
          placeholder="Flag any misleading elements, manipulated content, or provenance concerns..."
          style={{ ...inputStyle, resize: 'vertical' }}
        />
      </div>

      {/* Actions */}
      <div style={{ display: 'flex', gap: 10, justifyContent: 'flex-end', marginTop: 4 }}>
        <button
          type="button"
          onClick={onClose}
          className="btn ghost"
          style={{ fontSize: 13 }}
        >
          Cancel
        </button>
        <button type="button" className="btn" style={{ fontSize: 13 }}>
          Submit assessment <span className="arr">&rarr;</span>
        </button>
      </div>
    </div>
  );
}

function CompletedRow({ assessment }: { readonly assessment: CompletedAssessment }) {
  const relColor = scoreColor(assessment.relevanceScore);
  const rlbColor = scoreColor(assessment.reliabilityScore);
  const rationale =
    assessment.rationale.length > 160
      ? assessment.rationale.slice(0, 160) + '\u2026'
      : assessment.rationale;

  return (
    <div
      style={{
        padding: '18px 22px',
        borderBottom: '1px solid var(--line)',
        display: 'grid',
        gridTemplateColumns: '80px 1fr 160px 100px',
        gap: 20,
        alignItems: 'start',
        transition: 'background .12s',
        cursor: 'pointer',
      }}
    >
      {/* Scores */}
      <div style={{ display: 'flex', gap: 8, justifyContent: 'center' }}>
        <div style={{ textAlign: 'center' }}>
          <div
            style={{
              fontFamily: '"Fraunces", serif',
              fontSize: 28,
              letterSpacing: '-.02em',
              color: relColor,
              lineHeight: 1,
            }}
          >
            {assessment.relevanceScore}
          </div>
          <div
            style={{
              fontFamily: '"JetBrains Mono", monospace',
              fontSize: 8.5,
              letterSpacing: '.08em',
              textTransform: 'uppercase',
              color: 'var(--muted)',
              marginTop: 3,
            }}
          >
            REL
          </div>
        </div>
        <div style={{ textAlign: 'center' }}>
          <div
            style={{
              fontFamily: '"Fraunces", serif',
              fontSize: 28,
              letterSpacing: '-.02em',
              color: rlbColor,
              lineHeight: 1,
            }}
          >
            {assessment.reliabilityScore}
          </div>
          <div
            style={{
              fontFamily: '"JetBrains Mono", monospace',
              fontSize: 8.5,
              letterSpacing: '.08em',
              textTransform: 'uppercase',
              color: 'var(--muted)',
              marginTop: 3,
            }}
          >
            RLB
          </div>
        </div>
      </div>

      {/* Details */}
      <div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 5 }}>
          <span
            style={{
              fontFamily: '"JetBrains Mono", monospace',
              fontSize: 11,
              color: 'var(--accent)',
              letterSpacing: '.02em',
            }}
          >
            {assessment.evidenceRef}
          </span>
          <span style={{ fontSize: 13, fontWeight: 500, color: 'var(--ink)' }}>
            {assessment.evidenceName}
          </span>
          <StatusPill
            status={
              assessment.recommendation === 'collect'
                ? 'active'
                : assessment.recommendation === 'monitor'
                  ? 'hold'
                  : assessment.recommendation === 'deprioritize'
                    ? 'locked'
                    : 'broken'
            }
          >
            {assessment.recommendation.charAt(0).toUpperCase() +
              assessment.recommendation.slice(1)}
          </StatusPill>
        </div>
        <div
          style={{
            fontSize: 13,
            color: 'var(--muted)',
            lineHeight: 1.55,
            maxWidth: '55ch',
            marginBottom: 8,
          }}
        >
          {rationale}
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <BPIndicator
            variant="dots"
            phases={assessment.bpPhases}
          />
          <span
            style={{
              fontFamily: '"JetBrains Mono", monospace',
              fontSize: 10,
              letterSpacing: '.06em',
              textTransform: 'uppercase',
              color: 'var(--muted)',
            }}
          >
            {assessment.bpPhaseLabel}
          </span>
        </div>
      </div>

      {/* Assessor + date */}
      <div style={{ textAlign: 'right' }}>
        <div
          style={{
            fontFamily: '"Fraunces", serif',
            fontSize: 13.5,
            color: 'var(--ink)',
            letterSpacing: '-.005em',
            marginBottom: 3,
          }}
        >
          {assessment.assessor}
        </div>
        <div
          style={{
            fontFamily: '"JetBrains Mono", monospace',
            fontSize: 10.5,
            color: 'var(--muted)',
            letterSpacing: '.02em',
          }}
        >
          {assessment.assessedAt}
        </div>
      </div>

      {/* Action */}
      <div style={{ textAlign: 'right' }}>
        <LinkArrow href="#">View</LinkArrow>
      </div>
    </div>
  );
}

// --- Main component ---

export interface AssessmentsViewProps {
  readonly assessments?: EvidenceAssessment[];
  readonly totalEvidence?: number;
  readonly unassessedCount?: number;
}

export function AssessmentsView({ assessments, totalEvidence: _totalEvidence, unassessedCount }: AssessmentsViewProps = {}) {
  const [selectedItem, setSelectedItem] = useState<UnassessedItem | null>(null);
  const [activeFilter, setActiveFilter] = useState('all');

  const hasRealData = assessments !== undefined && assessments.length > 0;

  // Map real assessments to the stub shape when real data is available
  const displayAssessments: readonly CompletedAssessment[] = hasRealData
    ? assessments.map((a, _i): CompletedAssessment => ({
        id: a.id,
        evidenceRef: a.evidence_id.slice(0, 12),
        evidenceName: a.relevance_rationale.slice(0, 40),
        relevanceScore: a.relevance_score,
        reliabilityScore: a.reliability_score,
        recommendation: a.recommendation,
        assessor: a.assessed_by,
        assessedAt: new Date(a.created_at).toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' }),
        rationale: a.relevance_rationale,
        bpPhases: [
          { name: 'Survey', status: 'complete' },
          { name: 'Assess', status: 'complete' },
          { name: 'Collect', status: 'not_started' },
          { name: 'Verify', status: 'not_started' },
          { name: 'Analyse', status: 'not_started' },
          { name: 'Report', status: 'not_started' },
        ],
        bpPhaseLabel: 'Phase 2',
      }))
    : COMPLETED_ASSESSMENTS;

  const assessedCount = displayAssessments.length;
  const awaitingCount = hasRealData ? (unassessedCount ?? 0) : 847;
  const avgRelevance =
    displayAssessments.reduce((sum, a) => sum + a.relevanceScore, 0) / (assessedCount || 1);
  const deprioritizedCount = displayAssessments.filter(
    (a) => a.recommendation === 'deprioritize' || a.recommendation === 'discard',
  ).length;

  const collectCount = displayAssessments.filter((a) => a.recommendation === 'collect').length;
  const monitorCount = displayAssessments.filter((a) => a.recommendation === 'monitor').length;
  const depCount = displayAssessments.filter((a) => a.recommendation === 'deprioritize').length;
  const discardCount = displayAssessments.filter((a) => a.recommendation === 'discard').length;

  const filteredAssessments =
    activeFilter === 'all'
      ? displayAssessments
      : displayAssessments.filter((a) => a.recommendation === activeFilter);

  const chips = [
    { key: 'all', label: 'All', count: assessedCount, active: activeFilter === 'all' },
    { key: 'collect', label: 'Collect', count: collectCount, active: activeFilter === 'collect' },
    { key: 'monitor', label: 'Monitor', count: monitorCount, active: activeFilter === 'monitor' },
    {
      key: 'deprioritize',
      label: 'Deprioritize',
      count: depCount,
      active: activeFilter === 'deprioritize',
    },
    { key: 'discard', label: 'Discard', count: discardCount, active: activeFilter === 'discard' },
  ];

  return (
    <>
      {/* Page header */}
      <section className="d-pagehead">
        <div>
          <EyebrowLabel>Berkeley Protocol Phase 2</EyebrowLabel>
          <h1>
            Preliminary <em>assessments</em>
          </h1>
          <p className="sub">
            Every exhibit must have a signed assessment before entering the
            investigative record. Score relevance and reliability, flag
            misleading indicators, and recommend an action path.
          </p>
        </div>
        <div className="actions">
          <a className="btn ghost" href="#">
            Export assessments
          </a>
          <a className="btn" href="#">
            New assessment <span className="arr">&rarr;</span>
          </a>
        </div>
      </section>

      {/* KPI strip */}
      <KPIStrip
        items={[
          {
            label: 'Assessed \u00b7 this case',
            value: assessedCount,
            sub: `of ${(assessedCount + awaitingCount).toLocaleString()} exhibits`,
          },
          {
            label: 'Awaiting assessment',
            value: awaitingCount,
            delta: '\u25CF 12 uploaded today',
            deltaNegative: true,
          },
          {
            label: 'Avg. relevance',
            value: avgRelevance.toFixed(1),
            sub: 'across assessed',
          },
          {
            label: 'Deprioritized / discarded',
            value: deprioritizedCount,
            sub: 'transparent \u00b7 preserved',
          },
        ]}
      />

      {/* Two-column: Unassessed queue + How to assess guide */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: '1fr 1fr',
          gap: 22,
          marginTop: 22,
        }}
      >
        <UnassessedQueue items={UNASSESSED_ITEMS} onSelect={setSelectedItem} />
        <HowToAssessGuide />
      </div>

      {/* Completed assessments */}
      <div style={{ marginTop: 22 }}>
        <Panel title="Completed" titleAccent="assessments">
          <div style={{ margin: '-20px -20px 0' }}>
            <FilterBar
              searchPlaceholder="exhibit, assessor, recommendation\u2026"
              chips={chips}
              onChipClick={setActiveFilter}
            />
          </div>
          <div style={{ margin: '0 -20px -20px' }}>
            {filteredAssessments.length === 0 && (
              <div
                style={{
                  padding: '48px 28px',
                  textAlign: 'center',
                  color: 'var(--muted)',
                }}
              >
                No assessments match this filter.
              </div>
            )}
            {filteredAssessments.map((a) => (
              <CompletedRow key={a.id} assessment={a} />
            ))}
          </div>
        </Panel>
      </div>

      {/* Assessment modal (split layout) */}
      <Modal
        open={selectedItem !== null}
        onClose={() => setSelectedItem(null)}
        title="Assess exhibit"
        layout="split"
        left={selectedItem ? <AssessmentModalLeft item={selectedItem} /> : undefined}
      >
        <AssessmentModalRight onClose={() => setSelectedItem(null)} />
      </Modal>
    </>
  );
}

export default AssessmentsView;
