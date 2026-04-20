'use client';

import { useState } from 'react';
import {
  OPSEC_LEVELS,
  THREAT_LEVELS,
  type OpsecLevel,
  type ThreatLevel,
} from '@/types';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

interface FormState {
  readonly opsecLevel: OpsecLevel;
  readonly threatLevel: ThreatLevel;
  readonly usePseudonym: boolean;
  readonly pseudonym: string;
  readonly requireVpn: boolean;
  readonly requireTor: boolean;
  readonly approvedDevices: readonly string[];
  readonly prohibitedPlatforms: readonly string[];
  readonly threatNotes: string;
  readonly safetyBriefingCompleted: boolean;
  readonly briefingDate: string;
}

function defaultForm(): FormState {
  return {
    opsecLevel: 'standard',
    threatLevel: 'low',
    usePseudonym: false,
    pseudonym: '',
    requireVpn: false,
    requireTor: false,
    approvedDevices: [],
    prohibitedPlatforms: [],
    threatNotes: '',
    safetyBriefingCompleted: false,
    briefingDate: '',
  };
}

function TagInput({
  label,
  tags,
  onChange,
}: {
  label: string;
  tags: readonly string[];
  onChange: (tags: readonly string[]) => void;
}) {
  const [input, setInput] = useState('');

  const addTag = () => {
    const value = input.trim();
    if (value && !tags.includes(value)) {
      onChange([...tags, value]);
    }
    setInput('');
  };

  const removeTag = (idx: number) => {
    onChange(tags.filter((_, i) => i !== idx));
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      addTag();
    }
  };

  return (
    <div>
      <label className="field-label">{label}</label>
      <div style={{ display: 'flex', gap: 'var(--space-xs)', marginBottom: 'var(--space-xs)' }}>
        <input
          type="text"
          className="input-field"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Type and press Enter"
          style={{ flex: 1 }}
        />
        <button type="button" className="btn-secondary" onClick={addTag}>
          Add
        </button>
      </div>
      {tags.length > 0 && (
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 'var(--space-xs)' }}>
          {tags.map((tag, idx) => (
            <span
              key={tag + idx}
              className="badge"
              style={{
                backgroundColor: 'var(--amber-subtle)',
                color: 'var(--text-primary)',
                display: 'inline-flex',
                alignItems: 'center',
                gap: '0.25rem',
                cursor: 'default',
              }}
            >
              {tag}
              <button
                type="button"
                onClick={() => removeTag(idx)}
                style={{
                  all: 'unset',
                  cursor: 'pointer',
                  fontWeight: 700,
                  fontSize: 'var(--text-xs)',
                  lineHeight: 1,
                  color: 'var(--text-secondary)',
                }}
                aria-label={`Remove ${tag}`}
              >
                ×
              </button>
            </span>
          ))}
        </div>
      )}
    </div>
  );
}

export function SafetyProfileForm({
  caseId,
  userId,
  accessToken,
  onSaved,
}: {
  caseId: string;
  userId: string;
  accessToken: string;
  onSaved: () => void;
}) {
  const [form, setForm] = useState<FormState>(defaultForm);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const updateField = <K extends keyof FormState>(
    key: K,
    value: FormState[K],
  ) => {
    setForm((prev) => ({ ...prev, [key]: value }));
  };

  const handleSave = async () => {
    setSaving(true);
    setError(null);

    try {
      const body = {
        opsec_level: form.opsecLevel,
        threat_level: form.threatLevel,
        use_pseudonym: form.usePseudonym,
        pseudonym: form.usePseudonym ? form.pseudonym || undefined : undefined,
        required_vpn: form.requireVpn,
        required_tor: form.requireTor,
        approved_devices: [...form.approvedDevices],
        prohibited_platforms: [...form.prohibitedPlatforms],
        threat_notes: form.threatNotes || undefined,
        safety_briefing_completed: form.safetyBriefingCompleted,
        safety_briefing_date: form.briefingDate || undefined,
      };

      const res = await fetch(
        `${API_BASE}/api/cases/${caseId}/safety-profiles/${userId}`,
        {
          method: 'PUT',
          headers: {
            'Content-Type': 'application/json',
            Authorization: `Bearer ${accessToken}`,
          },
          body: JSON.stringify(body),
        },
      );

      if (!res.ok) {
        const data = await res.json().catch(() => null);
        throw new Error(
          data?.error || `Failed to save safety profile (${res.status})`,
        );
      }

      onSaved();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'An error occurred.');
    } finally {
      setSaving(false);
    }
  };

  const checkboxStyle: React.CSSProperties = {
    display: 'flex',
    alignItems: 'center',
    gap: 'var(--space-sm)',
    cursor: 'pointer',
  };

  const checkboxInputStyle: React.CSSProperties = {
    width: '1.125rem',
    height: '1.125rem',
    accentColor: 'var(--amber-accent)',
    cursor: 'pointer',
  };

  return (
    <div className="card" style={{ padding: 'var(--space-lg)' }}>
      <h2
        style={{
          fontSize: 'var(--text-xl)',
          fontWeight: 600,
          color: 'var(--text-primary)',
          margin: 0,
          marginBottom: 'var(--space-lg)',
        }}
      >
        Safety Profile
      </h2>

      {error && (
        <div className="banner-error" style={{ marginBottom: 'var(--space-md)' }}>
          {error}
        </div>
      )}

      <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-md)' }}>
        {/* OPSEC Level */}
        <div>
          <label className="field-label" htmlFor="sp-opsec">
            OPSEC Level
          </label>
          <select
            id="sp-opsec"
            className="input-field"
            value={form.opsecLevel}
            onChange={(e) =>
              updateField('opsecLevel', e.target.value as OpsecLevel)
            }
          >
            {OPSEC_LEVELS.map((l) => (
              <option key={l.value} value={l.value}>
                {l.label}
              </option>
            ))}
          </select>
        </div>

        {/* Threat Level */}
        <div>
          <label className="field-label" htmlFor="sp-threat">
            Threat Level
          </label>
          <select
            id="sp-threat"
            className="input-field"
            value={form.threatLevel}
            onChange={(e) =>
              updateField('threatLevel', e.target.value as ThreatLevel)
            }
          >
            {THREAT_LEVELS.map((l) => (
              <option key={l.value} value={l.value}>
                {l.label}
              </option>
            ))}
          </select>
        </div>

        {/* Pseudonym */}
        <div
          className="card-inset"
          style={{ padding: 'var(--space-md)' }}
        >
          <label style={checkboxStyle}>
            <input
              type="checkbox"
              style={checkboxInputStyle}
              checked={form.usePseudonym}
              onChange={(e) => updateField('usePseudonym', e.target.checked)}
            />
            <span style={{ fontSize: 'var(--text-sm)', fontWeight: 500, color: 'var(--text-primary)' }}>
              Use Pseudonym
            </span>
          </label>

          {form.usePseudonym && (
            <div style={{ marginTop: 'var(--space-sm)' }}>
              <label className="field-label" htmlFor="sp-pseudonym">
                Pseudonym
              </label>
              <input
                id="sp-pseudonym"
                type="text"
                className="input-field"
                placeholder="Enter pseudonym"
                value={form.pseudonym}
                onChange={(e) => updateField('pseudonym', e.target.value)}
              />
            </div>
          )}
        </div>

        {/* Network requirements */}
        <div
          className="card-inset"
          style={{
            padding: 'var(--space-md)',
            display: 'flex',
            flexDirection: 'column',
            gap: 'var(--space-sm)',
          }}
        >
          <span className="field-label" style={{ marginBottom: 0 }}>
            Network Requirements
          </span>
          <label style={checkboxStyle}>
            <input
              type="checkbox"
              style={checkboxInputStyle}
              checked={form.requireVpn}
              onChange={(e) => updateField('requireVpn', e.target.checked)}
            />
            <span style={{ fontSize: 'var(--text-sm)', color: 'var(--text-primary)' }}>
              Require VPN
            </span>
          </label>
          <label style={checkboxStyle}>
            <input
              type="checkbox"
              style={checkboxInputStyle}
              checked={form.requireTor}
              onChange={(e) => updateField('requireTor', e.target.checked)}
            />
            <span style={{ fontSize: 'var(--text-sm)', color: 'var(--text-primary)' }}>
              Require Tor
            </span>
          </label>
        </div>

        {/* Approved Devices */}
        <TagInput
          label="Approved Devices"
          tags={form.approvedDevices}
          onChange={(tags) => updateField('approvedDevices', tags)}
        />

        {/* Prohibited Platforms */}
        <TagInput
          label="Prohibited Platforms"
          tags={form.prohibitedPlatforms}
          onChange={(tags) => updateField('prohibitedPlatforms', tags)}
        />

        {/* Threat Notes */}
        <div>
          <label className="field-label" htmlFor="sp-threat-notes">
            Threat Notes
          </label>
          <textarea
            id="sp-threat-notes"
            className="input-field"
            rows={4}
            placeholder="Any relevant threat notes..."
            value={form.threatNotes}
            onChange={(e) => updateField('threatNotes', e.target.value)}
            style={{ resize: 'vertical' }}
          />
        </div>

        {/* Safety Briefing */}
        <div
          className="card-inset"
          style={{ padding: 'var(--space-md)' }}
        >
          <label style={checkboxStyle}>
            <input
              type="checkbox"
              style={checkboxInputStyle}
              checked={form.safetyBriefingCompleted}
              onChange={(e) =>
                updateField('safetyBriefingCompleted', e.target.checked)
              }
            />
            <span style={{ fontSize: 'var(--text-sm)', fontWeight: 500, color: 'var(--text-primary)' }}>
              Safety Briefing Completed
            </span>
          </label>

          {form.safetyBriefingCompleted && (
            <div style={{ marginTop: 'var(--space-sm)' }}>
              <label className="field-label" htmlFor="sp-briefing-date">
                Briefing Date
              </label>
              <input
                id="sp-briefing-date"
                type="date"
                className="input-field"
                value={form.briefingDate}
                onChange={(e) => updateField('briefingDate', e.target.value)}
              />
            </div>
          )}
        </div>

        {/* Save */}
        <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 'var(--space-sm)' }}>
          <button
            type="button"
            className="btn-primary"
            disabled={saving}
            onClick={handleSave}
          >
            {saving ? 'Saving...' : 'Save Safety Profile'}
          </button>
        </div>
      </div>
    </div>
  );
}
