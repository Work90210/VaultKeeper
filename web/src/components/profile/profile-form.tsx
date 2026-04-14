'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import type { UserProfile } from '@/types';

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '';

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
    <form onSubmit={handleSubmit} className="space-y-[var(--space-md)]">
      <div>
        <label className="field-label" htmlFor="profile-name">Display Name</label>
        <input id="profile-name" type="text" value={displayName} onChange={(e) => setDisplayName(e.target.value)} className="input-field" />
      </div>
      <div>
        <label className="field-label" htmlFor="profile-bio">Bio</label>
        <textarea id="profile-bio" value={bio} onChange={(e) => setBio(e.target.value)} rows={3} className="input-field resize-y" />
      </div>
      <div>
        <label className="field-label" htmlFor="profile-tz">Timezone</label>
        <select id="profile-tz" value={timezone} onChange={(e) => setTimezone(e.target.value)} className="input-field">
          {Intl.supportedValuesOf('timeZone').map((tz) => (
            <option key={tz} value={tz}>{tz}</option>
          ))}
        </select>
      </div>
      {error && <div className="banner-error">{error}</div>}
      <button type="submit" disabled={saving} className="btn-primary">
        {saving ? 'Saving\u2026' : 'Save changes'}
      </button>
    </form>
  );
}
