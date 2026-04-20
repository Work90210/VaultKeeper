'use client';

import { useState } from 'react';
import type { UserProfile, OrgWithRole } from '@/types';
import { ProfileForm } from './profile-form';

interface Props {
  profile: UserProfile;
  email: string;
  systemRole: string;
  organizations: OrgWithRole[];
}

export function ProfileView({ profile, email, systemRole, organizations: _organizations }: Props) {
  const [editing, setEditing] = useState(false);
  const displayName = profile.display_name || 'Unnamed User';
  const initial = (displayName || email)?.[0]?.toUpperCase() ?? '?';
  const roleLabel = systemRole.replace('_', ' ');

  return (
    <>
      {/* Page header */}
      <section className="d-pagehead">
        <div>
          <span className="eyebrow-m">Account</span>
          <h1>
            Your <em>profile</em>
          </h1>
          <p className="sub">
            Manage your personal information, signing keys, and security preferences. Changes are
            sealed audit events.
          </p>
        </div>
        <div className="actions">
          <button
            onClick={() => setEditing(!editing)}
            className={editing ? 'btn ghost' : 'btn'}
            type="button"
          >
            {editing ? 'Cancel' : 'Save changes'}
          </button>
        </div>
      </section>

      <div style={{ maxWidth: 600 }}>
        {/* Avatar section */}
        <div className="prof-section">
          <div className="prof-avatar-section">
            <span className="prof-avatar">{initial}</span>
            <div>
              <div style={{ fontWeight: 500, fontSize: 16 }}>{displayName}</div>
              <div style={{ fontSize: 13, color: 'var(--muted)', marginTop: 2 }}>
                {roleLabel} &middot; {roleLabel}
              </div>
              <button className="btn ghost sm" type="button" style={{ marginTop: 10 }}>
                Change avatar
              </button>
            </div>
          </div>
        </div>

        {/* Personal information */}
        <div className="prof-section">
          <h3>Personal information</h3>
          <p className="desc">
            Your name and contact details. These appear in audit logs and on signed events.
          </p>
          {editing ? (
            <ProfileForm profile={profile} onSaved={() => setEditing(false)} />
          ) : (
            <div className="prof-form">
              <div className="prof-row">
                <div className="prof-field">
                  <label>First name</label>
                  <input type="text" defaultValue={displayName.split(' ')[0] ?? ''} readOnly disabled />
                </div>
                <div className="prof-field">
                  <label>Last name</label>
                  <input type="text" defaultValue={displayName.split(' ').slice(1).join(' ')} readOnly disabled />
                </div>
              </div>
              <div className="prof-field">
                <label>Display name</label>
                <input type="text" defaultValue={displayName} readOnly disabled />
                <span className="hint">How your name appears in the workspace</span>
              </div>
              <div className="prof-field">
                <label>Email address</label>
                <input type="email" defaultValue={email} readOnly disabled />
                <span className="hint">
                  Managed by your organisation&rsquo;s SSO. Contact admin to change.
                </span>
              </div>
              <div className="prof-field">
                <label>Phone</label>
                <input type="tel" defaultValue="" readOnly disabled />
              </div>
              <div className="prof-field">
                <label>Role</label>
                <input type="text" defaultValue={roleLabel} disabled />
                <span className="hint">Set by organisation admin</span>
              </div>
              <div className="prof-field">
                <label>Time zone</label>
                <select disabled>
                  <option>Europe/Amsterdam (UTC+1)</option>
                  <option>Europe/London (UTC+0)</option>
                  <option>America/New_York (UTC-5)</option>
                  <option>Asia/Tokyo (UTC+9)</option>
                </select>
              </div>
              <div className="prof-field">
                <label>Language</label>
                <select disabled>
                  <option>English</option>
                  <option>Nederlands</option>
                  <option>Deutsch</option>
                  <option>Fran&ccedil;ais</option>
                </select>
              </div>
            </div>
          )}
        </div>

        {/* Signing key */}
        <div className="prof-section">
          <h3>Signing key</h3>
          <p className="desc">
            Your Ed25519 key pair used to sign and countersign chain events. Bound to your hardware
            token.
          </p>
          <div className="key-card">
            <span className="key-icon">
              <svg
                width="18"
                height="18"
                viewBox="0 0 16 16"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.4"
              >
                <path d="M10 2l4 4-7 7H3v-4z" />
                <path d="M8.5 3.5l4 4" />
              </svg>
            </span>
            <div className="key-info">
              <strong>Key A &middot; YubiHSM2</strong>
              <small>pub: vke1-ed25519-a4f8&hellip; &middot; last signed 2 min ago</small>
            </div>
            <a className="linkarrow" style={{ fontSize: 12 }}>
              Manage &rarr;
            </a>
          </div>
        </div>

        {/* Security */}
        <div className="prof-section">
          <h3>Security</h3>
          <p className="desc">Authentication and session preferences.</p>
          <div className="prof-form">
            <div
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                padding: '12px 0',
                borderBottom: '1px solid var(--line)',
              }}
            >
              <div>
                <div style={{ fontSize: 14, fontWeight: 500 }}>Two-factor authentication</div>
                <div style={{ fontSize: 12, color: 'var(--muted)', marginTop: 2 }}>
                  YubiKey &middot; configured 14 Mar 2026
                </div>
              </div>
              <span
                style={{
                  padding: '4px 10px',
                  borderRadius: 999,
                  background: 'rgba(74,107,58,.1)',
                  color: 'var(--ok)',
                  fontSize: 11.5,
                  fontWeight: 500,
                }}
              >
                Enabled
              </span>
            </div>
            <div
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                padding: '12px 0',
                borderBottom: '1px solid var(--line)',
              }}
            >
              <div>
                <div style={{ fontSize: 14, fontWeight: 500 }}>Session timeout</div>
                <div style={{ fontSize: 12, color: 'var(--muted)', marginTop: 2 }}>
                  Auto-lock after 8 hours idle
                </div>
              </div>
              <a className="linkarrow" style={{ fontSize: 12, cursor: 'pointer' }}>
                Change &rarr;
              </a>
            </div>
            <div
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                padding: '12px 0',
                borderBottom: '1px solid var(--line)',
              }}
            >
              <div>
                <div style={{ fontSize: 14, fontWeight: 500 }}>Active sessions</div>
                <div style={{ fontSize: 12, color: 'var(--muted)', marginTop: 2 }}>
                  1 active &middot; this device
                </div>
              </div>
              <a className="linkarrow" style={{ fontSize: 12, cursor: 'pointer' }}>
                View all &rarr;
              </a>
            </div>
            <div
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                padding: '12px 0',
              }}
            >
              <div>
                <div style={{ fontSize: 14, fontWeight: 500 }}>Password</div>
                <div style={{ fontSize: 12, color: 'var(--muted)', marginTop: 2 }}>
                  Last changed 42 days ago
                </div>
              </div>
              <button className="btn ghost sm" type="button">
                Change password
              </button>
            </div>
          </div>
        </div>

        {/* Danger zone */}
        <div className="prof-section">
          <h3 style={{ color: '#b35c5c' }}>Danger</h3>
          <p className="desc">Irreversible actions affecting your account.</p>
          <div style={{ display: 'flex', gap: 12 }}>
            <button
              className="btn ghost sm"
              type="button"
              style={{ color: '#b35c5c', borderColor: 'rgba(179,92,92,.3)' }}
            >
              Sign out of all sessions
            </button>
            <button
              className="btn ghost sm"
              type="button"
              style={{ color: '#b35c5c', borderColor: 'rgba(179,92,92,.3)' }}
            >
              Deactivate account
            </button>
          </div>
        </div>
      </div>
    </>
  );
}
