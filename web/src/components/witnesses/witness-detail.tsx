'use client';

import type { Witness } from '@/types';
import {
  ArrowLeft,
  Shield,
  AlertTriangle,
} from 'lucide-react';

const PROTECTION_PILL: Record<string, { cls: string; label: string }> = {
  standard: { cls: 'pl sealed', label: 'Standard' },
  protected: { cls: 'pl hold', label: 'Protected' },
  high_risk: { cls: 'pl broken', label: 'High Risk' },
};

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('en-GB', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
  });
}

export function WitnessDetail({
  witness,
  onEdit,
  onBack,
  canEdit,
}: {
  witness: Witness;
  onEdit?: () => void;
  onBack: () => void;
  canEdit: boolean;
}) {
  const protection =
    PROTECTION_PILL[witness.protection_status] || PROTECTION_PILL.standard;

  return (
    <div style={{ maxWidth: '860px', margin: '0 auto' }}>
      {/* Header */}
      <header style={{ marginBottom: '24px' }}>
        <button
          onClick={onBack}
          className="btn ghost sm"
          type="button"
          style={{ marginBottom: '16px', display: 'inline-flex', alignItems: 'center', gap: '6px' }}
        >
          <ArrowLeft size={14} />
          Back to case
        </button>

        <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '16px' }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '6px' }}>
              <span className="tag">{witness.witness_code}</span>
              <span className={protection.cls}>{protection.label}</span>
            </div>
            <h1 style={{ fontFamily: '"Fraunces", serif', fontSize: '28px', letterSpacing: '-.01em', color: 'var(--ink)', fontWeight: 400 }}>
              {witness.identity_visible
                ? witness.full_name || 'Unnamed Witness'
                : `Witness ${witness.witness_code}`}
            </h1>
          </div>
          {canEdit && onEdit && (
            <button type="button" onClick={onEdit} className="btn ghost sm">
              Edit witness
            </button>
          )}
        </div>
      </header>

      {/* Identity section */}
      {witness.identity_visible ? (
        <div className="panel" style={{ marginBottom: '22px' }}>
          <div className="panel-h">
            <h3>Identity</h3>
          </div>
          <div className="panel-body">
            <dl className="kvs">
              <dt>Full name</dt>
              <dd><strong>{witness.full_name || '\u2014'}</strong></dd>
              <dt>Contact</dt>
              <dd>{witness.contact_info || '\u2014'}</dd>
              <dt>Location</dt>
              <dd>{witness.location || '\u2014'}</dd>
            </dl>
          </div>
        </div>
      ) : (
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: '10px',
            padding: '14px 18px',
            borderRadius: '10px',
            marginBottom: '22px',
            background: 'rgba(91,74,107,.08)',
            borderLeft: '3px solid #5b4a6b',
          }}
        >
          <Shield size={14} style={{ color: '#5b4a6b' }} />
          <p style={{ fontSize: '13px', color: '#5b4a6b', margin: 0 }}>
            Identity information restricted for your role.
          </p>
        </div>
      )}

      {/* Statement summary */}
      <div className="panel" style={{ marginBottom: '22px' }}>
        <div className="panel-h">
          <h3>Statement <em>summary</em></h3>
        </div>
        <div className="panel-body">
          <p style={{
            fontSize: '13.5px',
            lineHeight: 1.6,
            whiteSpace: 'pre-wrap',
            color: witness.statement_summary ? 'var(--ink-2)' : 'var(--muted)',
            margin: 0,
          }}>
            {witness.statement_summary || 'No statement recorded.'}
          </p>
        </div>
      </div>

      {/* Linked evidence */}
      <div className="panel" style={{ marginBottom: '22px' }}>
        <div className="panel-h">
          <h3>Linked <em>evidence</em></h3>
          <span className="meta">{witness.related_evidence.length} items</span>
        </div>
        {witness.related_evidence.length === 0 ? (
          <div className="panel-body" style={{ textAlign: 'center' }}>
            <p style={{ fontSize: '13px', color: 'var(--muted)' }}>
              No evidence linked to this witness.
            </p>
          </div>
        ) : (
          <div className="panel-body flush">
            <table className="tbl">
              <tbody>
                {witness.related_evidence.map((id) => (
                  <tr key={id}>
                    <td>
                      <a
                        href={`/en/evidence/${id}`}
                        className="ref"
                        style={{ textDecoration: 'none' }}
                      >
                        {id.slice(0, 12)}&hellip;
                      </a>
                    </td>
                    <td className="actions">
                      <a href={`/en/evidence/${id}`} className="linkarrow" style={{ textDecoration: 'none' }}>
                        View &rarr;
                      </a>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Metadata */}
      <div className="panel" style={{ marginBottom: '22px' }}>
        <div className="panel-h">
          <h3>Metadata</h3>
        </div>
        <div className="panel-body">
          <dl className="kvs">
            <dt>Created</dt>
            <dd><strong>{formatDate(witness.created_at)}</strong></dd>
            <dt>Updated</dt>
            <dd><strong>{formatDate(witness.updated_at)}</strong></dd>
          </dl>
        </div>
      </div>

      {/* Protection notice for high-risk */}
      {witness.protection_status === 'high_risk' && (
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: '10px',
            padding: '14px 18px',
            borderRadius: '10px',
            background: 'rgba(168,62,43,.08)',
            borderLeft: '3px solid var(--accent)',
          }}
        >
          <AlertTriangle size={14} style={{ color: 'var(--accent)' }} />
          <p style={{ fontSize: '13px', color: 'var(--accent)', margin: 0 }}>
            This witness has high-risk protection status. All access is logged and restricted.
          </p>
        </div>
      )}
    </div>
  );
}
