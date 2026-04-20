'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import type { UserProfile } from '@/types';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

const labelStyle: React.CSSProperties = {
  fontFamily: '"JetBrains Mono", monospace',
  fontSize: '10.5px',
  letterSpacing: '.08em',
  textTransform: 'uppercase',
  color: 'var(--muted)',
};

const inputStyle: React.CSSProperties = {
  padding: '11px 16px',
  borderRadius: 'var(--radius-sm)',
  border: '1px solid var(--line-2)',
  background: 'var(--paper)',
  font: 'inherit',
  fontSize: '14px',
  color: 'var(--ink)',
  outline: 'none',
  width: '100%',
  boxSizing: 'border-box',
};

interface Props {
  profile: UserProfile;
  onSaved: () => void;
}

export function ProfileForm({ profile, onSaved }: Props) {
  const router = useRouter();
  const [displayName, setDisplayName] = useState(profile.display_name);
  const [bio, setBio] = useState(profile.bio);
  const [timezone, setTimezone] = useState(profile.timezone);
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError('');
    try {
      const res = await fetch(`${API_BASE}/api/me`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ display_name: displayName, bio, timezone }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => null);
        setError(body?.error ?? 'Failed to update profile');
        return;
      }
      router.refresh();
      onSaved();
    } catch {
      setError('Network error');
    } finally {
      setSaving(false);
    }
  }

  return (
    <form onSubmit={handleSubmit} className="prof-form" style={{ display: 'flex', flexDirection: 'column', gap: '20px', maxWidth: '520px' }}>
      <div className="prof-field" style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
        <label htmlFor="profile-name" style={labelStyle}>Display Name</label>
        <input
          id="profile-name"
          type="text"
          value={displayName}
          onChange={(e) => setDisplayName(e.target.value)}
          style={inputStyle}
        />
      </div>
      <div className="prof-field" style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
        <label htmlFor="profile-bio" style={labelStyle}>Bio</label>
        <textarea
          id="profile-bio"
          value={bio}
          onChange={(e) => setBio(e.target.value)}
          rows={3}
          style={{ ...inputStyle, resize: 'vertical' }}
        />
      </div>
      <div className="prof-field" style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
        <label htmlFor="profile-tz" style={labelStyle}>Timezone</label>
        <select
          id="profile-tz"
          value={timezone}
          onChange={(e) => setTimezone(e.target.value)}
          style={inputStyle}
        >
          {Intl.supportedValuesOf('timeZone').map((tz) => (
            <option key={tz} value={tz}>{tz}</option>
          ))}
        </select>
      </div>
      {error && <div className="banner-error">{error}</div>}
      <button type="submit" disabled={saving} className="btn">
        {saving ? 'Saving\u2026' : 'Save changes'}
      </button>
    </form>
  );
}
