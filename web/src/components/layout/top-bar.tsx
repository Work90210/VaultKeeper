'use client';

import { useState, useEffect, useRef } from 'react';
import { useRouter } from 'next/navigation';

/* ─── Notification data (stub — Sprint E will connect to real API) ─── */
const NOTIFICATIONS = [
  { title: 'Countersign required', desc: 'Federated merge VKE1\u00b791ab\u2026 from CIJA waiting for your signature.', time: '2 min ago', urgent: true },
  { title: 'Redaction review', desc: 'Martyna flagged 3 passages in W-0144 statement for your review.', time: '28 min ago', urgent: true },
  { title: 'Disclosure deadline', desc: 'DISC-2026-019 for defence counsel due Friday. 48 exhibits.', time: '1 h ago', urgent: true },
  { title: 'Chain verified', desc: 'witness-node-02 countersigned block f208\u2026 \u00b7 3 ops sealed.', time: '4 h ago', urgent: false },
  { title: 'Federation sync complete', desc: 'CIJA \u00b7 Berlin sub-chain synced. 12 new ops.', time: '6 h ago', urgent: false },
  { title: 'Key rotation reminder', desc: 'Quarterly rotation for Key A scheduled in 11 days.', time: 'Yesterday', urgent: false },
] as const;

const HELP_ITEMS = [
  {
    icon: (
      <svg width={15} height={15} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.4}>
        <path d="M3 2.5h10v11H3z" /><path d="M5.5 6h5M5.5 9h5M5.5 11.5h3" />
      </svg>
    ),
    label: 'Documentation',
    desc: 'Guides, API reference & tutorials',
  },
  {
    icon: (
      <svg width={15} height={15} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.4}>
        <circle cx="8" cy="8" r="6" /><path d="M6.4 6.4a1.6 1.6 0 113 .6c-.6.3-1 .7-1 1.4M8 10.5v.5" />
      </svg>
    ),
    label: 'FAQ',
    desc: 'Common questions & troubleshooting',
  },
  {
    icon: (
      <svg width={15} height={15} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.4}>
        <rect x="2" y="2" width="5" height="5" rx="1" /><rect x="9" y="2" width="5" height="5" rx="1" />
        <rect x="2" y="9" width="5" height="5" rx="1" /><rect x="9" y="9" width="5" height="5" rx="1" />
      </svg>
    ),
    label: 'Keyboard shortcuts',
    desc: 'View all shortcuts (\u2318/)',
  },
  {
    icon: (
      <svg width={15} height={15} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.4}>
        <path d="M2 4l6-2 6 2v5c0 3-3 5-6 6-3-1-6-3-6-6z" />
      </svg>
    ),
    label: 'Security & compliance',
    desc: 'Certifications & policies',
  },
  {
    icon: (
      <svg width={15} height={15} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.4}>
        <circle cx="8" cy="6" r="2.5" /><path d="M3 13.5c.8-2.5 2.7-4 5-4s4.2 1.5 5 4" />
      </svg>
    ),
    label: 'Contact support',
    desc: 'Get help from our team',
  },
] as const;

/* ─── Dropdown wrapper with click-outside ─── */
function useClickOutside(onClose: () => void) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    function handle(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose();
    }
    document.addEventListener('click', handle);
    return () => document.removeEventListener('click', handle);
  }, [onClose]);
  return ref;
}

export function TopBar() {
  const router = useRouter();
  const [notifOpen, setNotifOpen] = useState(false);
  const [helpOpen, setHelpOpen] = useState(false);

  const notifRef = useClickOutside(() => setNotifOpen(false));
  const helpRef = useClickOutside(() => setHelpOpen(false));

  const unreadCount = NOTIFICATIONS.filter((n) => n.urgent).length;

  // ⌘K shortcut
  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        router.push('/en/search');
      }
    }
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [router]);

  return (
    <div className="d-top" style={{ backdropFilter: 'blur(10px)' }}>
      {/* Breadcrumb area */}
      <nav className="d-crumb">
        {/* Breadcrumbs populated by page-level context */}
      </nav>

      {/* Top actions */}
      <div className="d-top-actions">
        {/* Search pill */}
        <div
          className="d-search"
          onClick={() => router.push('/en/search')}
          role="button"
          tabIndex={0}
          onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ' ') router.push('/en/search');
          }}
        >
          <svg width={14} height={14} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.5}>
            <circle cx="7" cy="7" r="4" />
            <path d="M10 10l3 3" />
          </svg>
          <span>Search exhibits, witnesses, hashes&hellip;</span>
          <kbd>{'\u2318'}K</kbd>
        </div>

        {/* Notification bell */}
        <div ref={notifRef} style={{ position: 'relative' }}>
          <button
            type="button"
            className="d-iconbtn"
            title="Notifications"
            onClick={(e) => {
              e.stopPropagation();
              setHelpOpen(false);
              setNotifOpen(!notifOpen);
            }}
          >
            <svg width={16} height={16} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.4}>
              <path d="M4 10.5a4 4 0 118 0v1H4zM6.5 12.5a1.5 1.5 0 003 0" />
            </svg>
            {unreadCount > 0 && <span className="notif" />}
          </button>

          {notifOpen && (
            <div
              style={{
                position: 'absolute',
                top: '100%',
                right: 0,
                marginTop: 8,
                width: 360,
                maxHeight: 480,
                overflowY: 'auto',
                background: 'var(--paper)',
                border: '1px solid var(--line-2)',
                borderRadius: 12,
                boxShadow: '0 2px 6px rgba(20,17,12,.05), 0 16px 40px rgba(20,17,12,.08)',
                zIndex: 999,
              }}
            >
              <div style={{
                padding: '14px 18px',
                borderBottom: '1px solid var(--line)',
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
              }}>
                <span style={{ fontWeight: 500, fontSize: 14 }}>Notifications</span>
                <span style={{
                  fontFamily: 'JetBrains Mono, monospace',
                  fontSize: 10,
                  color: 'var(--muted)',
                  letterSpacing: '.06em',
                  textTransform: 'uppercase',
                }}>
                  {unreadCount} unread
                </span>
              </div>
              {NOTIFICATIONS.map((n, i) => (
                <div
                  key={i}
                  style={{
                    padding: '12px 18px',
                    borderBottom: '1px solid var(--line)',
                    cursor: 'pointer',
                    transition: 'background .12s',
                    background: n.urgent ? 'rgba(184,66,28,.03)' : 'transparent',
                  }}
                >
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'start', gap: 10 }}>
                    <div style={{ fontSize: 13.5, fontWeight: 500, color: 'var(--ink)' }}>{n.title}</div>
                    {n.urgent && (
                      <span style={{ width: 6, height: 6, borderRadius: '50%', background: 'var(--accent)', flexShrink: 0, marginTop: 6 }} />
                    )}
                  </div>
                  <div style={{ fontSize: 12.5, color: 'var(--muted)', lineHeight: 1.5, marginTop: 4 }}>{n.desc}</div>
                  <div style={{
                    fontFamily: 'JetBrains Mono, monospace',
                    fontSize: 10.5,
                    color: 'var(--muted-2)',
                    marginTop: 6,
                    letterSpacing: '.02em',
                  }}>
                    {n.time}
                  </div>
                </div>
              ))}
              <div style={{ padding: '12px 18px', textAlign: 'center' }}>
                <a
                  href="/en/cases?view=audit"
                  style={{ fontSize: 13, color: 'var(--accent)', fontWeight: 500, textDecoration: 'none' }}
                >
                  View all activity &rarr;
                </a>
              </div>
            </div>
          )}
        </div>

        {/* Help button */}
        <div ref={helpRef} style={{ position: 'relative' }}>
          <button
            type="button"
            className="d-iconbtn"
            title="Help"
            onClick={(e) => {
              e.stopPropagation();
              setNotifOpen(false);
              setHelpOpen(!helpOpen);
            }}
          >
            <svg width={16} height={16} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.4}>
              <circle cx="8" cy="8" r="6" />
              <path d="M6.4 6.4a1.6 1.6 0 113 .6c-.6.3-1 .7-1 1.4M8 10.5v.5" />
            </svg>
          </button>

          {helpOpen && (
            <div
              style={{
                position: 'absolute',
                top: '100%',
                right: 0,
                marginTop: 8,
                width: 300,
                background: 'var(--paper)',
                border: '1px solid var(--line-2)',
                borderRadius: 12,
                boxShadow: '0 2px 6px rgba(20,17,12,.05), 0 16px 40px rgba(20,17,12,.08)',
                zIndex: 999,
              }}
            >
              <div style={{
                padding: '14px 18px',
                borderBottom: '1px solid var(--line)',
                fontWeight: 500,
                fontSize: 14,
              }}>
                Help & resources
              </div>
              {HELP_ITEMS.map((h, i) => (
                <button
                  key={i}
                  type="button"
                  style={{
                    display: 'flex',
                    alignItems: 'start',
                    gap: 12,
                    padding: '12px 18px',
                    cursor: 'pointer',
                    transition: 'background .12s',
                    textDecoration: 'none',
                    color: 'inherit',
                    width: '100%',
                    border: 'none',
                    background: 'none',
                    textAlign: 'left',
                    fontFamily: 'inherit',
                  }}
                >
                  <span style={{ color: 'var(--muted)', flexShrink: 0, marginTop: 2 }}>{h.icon}</span>
                  <div>
                    <div style={{ fontSize: 13.5, fontWeight: 500 }}>{h.label}</div>
                    <div style={{ fontSize: 12, color: 'var(--muted)', marginTop: 1 }}>{h.desc}</div>
                  </div>
                </button>
              ))}
              <div style={{
                padding: '12px 18px',
                borderTop: '1px solid var(--line)',
                fontFamily: 'JetBrains Mono, monospace',
                fontSize: 10.5,
                color: 'var(--muted-2)',
                letterSpacing: '.02em',
              }}>
                VaultKeeper v2.4.1 &middot; Build 2026.04.20
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
