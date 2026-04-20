'use client';

import { signIn } from 'next-auth/react';
import { useSearchParams } from 'next/navigation';
import { useState } from 'react';
import Link from 'next/link';

function isSafeCallbackUrl(url: string): boolean {
  if (!url.startsWith('/')) return false;
  try {
    const parsed = new URL(url, 'http://localhost');
    return parsed.hostname === 'localhost';
  } catch {
    return false;
  }
}

type View = 'signin' | 'forgot' | 'check-email' | 'signup' | 'signup-done';

export default function LoginPage() {
  const searchParams = useSearchParams();
  const error = searchParams.get('error');
  const raw = searchParams.get('callbackUrl') || '';
  const callbackUrl = isSafeCallbackUrl(raw) ? raw : '/en/cases';

  const [view, setView] = useState<View>('signin');
  const [loading, setLoading] = useState(false);
  const [sentEmail, setSentEmail] = useState('');

  const handleSSO = () => {
    setLoading(true);
    signIn('keycloak', { callbackUrl });
  };

  const handleEmailSignIn = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setLoading(true);
    signIn('keycloak', { callbackUrl });
  };

  const handleForgot = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const form = e.currentTarget;
    const email = (form.elements.namedItem('reset-email') as HTMLInputElement).value;
    setLoading(true);
    setTimeout(() => {
      setLoading(false);
      setSentEmail(email);
      setView('check-email');
    }, 1200);
  };

  const handleSignup = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setLoading(true);
    setTimeout(() => {
      setLoading(false);
      setView('signup-done');
    }, 1200);
  };

  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: `
        .auth-shell{min-height:100vh;display:grid;grid-template-columns:1fr 1fr}
        @media(max-width:900px){.auth-shell{grid-template-columns:1fr}.auth-side{display:none}}
        .auth-side{background:var(--ink);color:var(--bg);display:flex;flex-direction:column;justify-content:space-between;padding:48px;position:relative;overflow:hidden}
        .auth-side::before{content:"";position:absolute;inset:0;background:radial-gradient(600px 400px at 30% 80%,rgba(184,66,28,.2),transparent 60%),radial-gradient(500px 500px at 80% 10%,rgba(184,66,28,.12),transparent 60%)}
        .auth-side .side-brand{font-family:"Fraunces",serif;font-size:24px;letter-spacing:-.02em;position:relative;z-index:1}
        .auth-side .side-brand em{color:var(--accent-soft);font-style:italic}
        .side-quote{position:relative;z-index:1;max-width:400px}
        .side-quote blockquote{font-family:"Fraunces",serif;font-size:28px;line-height:1.25;letter-spacing:-.02em;font-weight:300;font-style:italic;margin:0;color:rgba(245,241,232,.85)}
        .side-quote cite{display:block;margin-top:20px;font-family:"Inter",sans-serif;font-size:13px;font-style:normal;color:rgba(245,241,232,.45);letter-spacing:.02em}
        .side-stats{display:flex;gap:40px;position:relative;z-index:1}
        .side-stat{display:flex;flex-direction:column;gap:2px}
        .side-stat .val{font-family:"Fraunces",serif;font-size:32px;letter-spacing:-.02em;color:var(--accent-soft)}
        .side-stat .lbl{font-family:"JetBrains Mono",monospace;font-size:10px;text-transform:uppercase;letter-spacing:.1em;color:rgba(245,241,232,.35)}
        .auth-main{display:flex;align-items:center;justify-content:center;padding:48px 32px;position:relative}
        .auth-form-wrap{width:100%;max-width:400px}
        .auth-form-wrap .brand-sm{display:none;font-family:"Fraunces",serif;font-size:22px;letter-spacing:-.02em;margin-bottom:40px}
        .auth-form-wrap .brand-sm em{color:var(--accent);font-style:italic}
        @media(max-width:900px){.auth-form-wrap .brand-sm{display:block}}
        .auth-form-wrap h2{font-size:32px;letter-spacing:-.03em;margin-bottom:6px}
        .auth-form-wrap .sub{color:var(--muted);font-size:15px;margin-bottom:36px;line-height:1.5}
        .field{display:flex;flex-direction:column;gap:6px;margin-bottom:20px}
        .field label{font-size:13px;font-weight:500;color:var(--ink-2);letter-spacing:.01em}
        .field input{padding:12px 16px;border-radius:var(--radius-sm);border:1px solid var(--line-2);background:var(--paper);font:inherit;font-size:15px;color:var(--ink);transition:border-color .2s,box-shadow .2s;outline:none}
        .field input:focus{border-color:var(--accent);box-shadow:0 0 0 3px rgba(184,66,28,.1)}
        .field input::placeholder{color:var(--muted-2)}
        .field .hint{font-size:12px;color:var(--muted);margin-top:2px}
        .field-row{display:flex;justify-content:space-between;align-items:center}
        .field-row label{display:flex;align-items:center;gap:8px;font-size:13px;color:var(--ink-2);cursor:pointer}
        .field-row input[type="checkbox"]{width:16px;height:16px;accent-color:var(--accent);cursor:pointer}
        .forgot-link{font-size:13px;color:var(--accent);font-weight:500;transition:opacity .2s;cursor:pointer;background:none;border:none;font:inherit}
        .forgot-link:hover{opacity:.7}
        .auth-btn{width:100%;padding:14px 20px;border-radius:999px;border:none;cursor:pointer;background:var(--ink);color:var(--bg);font:inherit;font-size:15px;font-weight:500;transition:background .2s,transform .15s,box-shadow .2s;margin-top:28px}
        .auth-btn:hover{background:var(--accent);transform:translateY(-1px);box-shadow:0 10px 24px rgba(184,66,28,.3)}
        .auth-btn:active{transform:translateY(0)}
        .auth-btn.loading{pointer-events:none;opacity:.7}
        .auth-btn.loading::after{content:"";display:inline-block;width:16px;height:16px;border:2px solid transparent;border-top-color:var(--bg);border-radius:50%;animation:spin .6s linear infinite;margin-left:8px}
        @keyframes spin{to{transform:rotate(360deg)}}
        .auth-divider{display:flex;align-items:center;gap:16px;margin:28px 0;color:var(--muted-2);font-size:13px}
        .auth-divider::before,.auth-divider::after{content:"";flex:1;height:1px;background:var(--line-2)}
        .sso-btn{width:100%;padding:12px 20px;border-radius:999px;border:1px solid var(--line-2);cursor:pointer;background:var(--paper);color:var(--ink);font:inherit;font-size:14px;font-weight:500;display:flex;align-items:center;justify-content:center;gap:10px;transition:border-color .2s,background .2s;margin-bottom:10px}
        .sso-btn:hover{border-color:var(--ink);background:var(--bg-2)}
        .auth-footer{margin-top:32px;text-align:center;font-size:14px;color:var(--muted)}
        .auth-footer a{color:var(--accent);font-weight:500;cursor:pointer}
        .auth-footer a:hover{text-decoration:underline}
        .back-link{display:inline-flex;align-items:center;gap:6px;font-size:13px;color:var(--muted);margin-bottom:28px;transition:color .2s;cursor:pointer;background:none;border:none;font:inherit;padding:0}
        .back-link:hover{color:var(--accent)}
        .success-icon{width:56px;height:56px;border-radius:50%;background:rgba(74,107,58,.1);display:grid;place-items:center;margin-bottom:20px}
        .success-icon svg{color:var(--ok)}
        .side-grid{position:absolute;inset:0;opacity:.06;background-image:linear-gradient(rgba(245,241,232,.3) 1px,transparent 1px),linear-gradient(90deg,rgba(245,241,232,.3) 1px,transparent 1px);background-size:48px 48px;pointer-events:none}
        .legal-line{position:absolute;bottom:28px;left:32px;right:32px;text-align:center;font-size:12px;color:var(--muted-2)}
        .legal-line a{color:var(--muted);text-decoration:underline;text-underline-offset:2px}
      `}} />

      <div className="auth-shell">
        {/* Left decorative panel */}
        <div className="auth-side">
          <div className="side-grid"></div>
          <div className="side-brand">
            <span className="brand-mark" style={{ display: 'inline-block', verticalAlign: 'middle', marginRight: '10px' }}></span>
            Vault<em>Keeper</em>
          </div>
          <div className="side-quote">
            <blockquote>&ldquo;Every exhibit, every testimony, sealed on an append-only chain — because truth shouldn&apos;t depend on trust.&rdquo;</blockquote>
            <cite>— VaultKeeper design principles</cite>
          </div>
          <div className="side-stats">
            <div className="side-stat"><span className="val">256-bit</span><span className="lbl">SHA chain</span></div>
            <div className="side-stat"><span className="val">Ed25519</span><span className="lbl">Signatures</span></div>
            <div className="side-stat"><span className="val">RFC 3161</span><span className="lbl">Timestamps</span></div>
          </div>
        </div>

        {/* Right form panel */}
        <div className="auth-main">
          <div className="auth-form-wrap">
            <div className="brand-sm">
              <span className="brand-mark" style={{ display: 'inline-block', verticalAlign: 'middle', marginRight: '8px' }}></span>
              Vault<em>Keeper</em>
            </div>

            {/* Error banner */}
            {error && (
              <div className="banner-error" style={{ marginBottom: '20px' }}>
                {error === 'OAuthSignin' ? 'Unable to reach the identity provider. Is the SSO service running?' :
                 error === 'OAuthCallback' ? 'Authentication callback failed. Please try again.' :
                 'An authentication error occurred. Please try again.'}
              </div>
            )}

            {/* VIEW: Sign In */}
            {view === 'signin' && (
              <div>
                <h2>Welcome back</h2>
                <p className="sub">Sign in to your workspace to continue.</p>

                <button className="sso-btn" onClick={handleSSO} type="button">
                  <svg width="18" height="18" viewBox="0 0 18 18" fill="none">
                    <rect x="1" y="1" width="7" height="7" rx="1" fill="#b8421c" />
                    <rect x="10" y="1" width="7" height="7" rx="1" fill="#b8421c" opacity=".6" />
                    <rect x="1" y="10" width="7" height="7" rx="1" fill="#b8421c" opacity=".6" />
                    <rect x="10" y="10" width="7" height="7" rx="1" fill="#b8421c" opacity=".3" />
                  </svg>
                  Continue with SSO
                </button>

                <div className="auth-divider">or sign in with email</div>

                <form onSubmit={handleEmailSignIn}>
                  <div className="field">
                    <label htmlFor="email">Email address</label>
                    <input type="email" id="email" placeholder="name@organisation.org" autoComplete="email" required />
                  </div>
                  <div className="field">
                    <label htmlFor="password">Password</label>
                    <input type="password" id="password" placeholder="Enter your password" autoComplete="current-password" required />
                  </div>
                  <div className="field-row" style={{ marginTop: '-4px' }}>
                    <label><input type="checkbox" defaultChecked /> Remember me</label>
                    <button type="button" className="forgot-link" onClick={() => setView('forgot')}>Forgot password?</button>
                  </div>
                  <button type="submit" className={`auth-btn${loading ? ' loading' : ''}`}>
                    {loading ? 'Please wait\u2026' : 'Sign in'}
                  </button>
                </form>

                <p className="auth-footer">
                  Don&apos;t have an account?{' '}
                  <a onClick={() => setView('signup')}>Request access</a>
                </p>
              </div>
            )}

            {/* VIEW: Forgot Password */}
            {view === 'forgot' && (
              <div>
                <button type="button" className="back-link" onClick={() => setView('signin')}>&larr; Back to sign in</button>
                <h2>Reset password</h2>
                <p className="sub">Enter the email associated with your account and we&apos;ll send a reset link.</p>

                <form onSubmit={handleForgot}>
                  <div className="field">
                    <label htmlFor="reset-email">Email address</label>
                    <input type="email" id="reset-email" name="reset-email" placeholder="name@organisation.org" autoComplete="email" required />
                  </div>
                  <button type="submit" className={`auth-btn${loading ? ' loading' : ''}`}>
                    {loading ? 'Please wait\u2026' : 'Send reset link'}
                  </button>
                </form>

                <p className="auth-footer">
                  Remember your password? <a onClick={() => setView('signin')}>Sign in</a>
                </p>
              </div>
            )}

            {/* VIEW: Check Email */}
            {view === 'check-email' && (
              <div>
                <button type="button" className="back-link" onClick={() => setView('signin')}>&larr; Back to sign in</button>
                <div className="success-icon">
                  <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round"><path d="M22 6L9.5 18 3 12" /></svg>
                </div>
                <h2>Check your email</h2>
                <p className="sub">We&apos;ve sent a password reset link to <strong>{sentEmail}</strong>. It expires in 30 minutes.</p>
                <p style={{ fontSize: '13px', color: 'var(--muted)', lineHeight: 1.6, marginTop: '8px' }}>
                  Didn&apos;t receive it? Check your spam folder or{' '}
                  <a style={{ color: 'var(--accent)', cursor: 'pointer' }} onClick={() => setView('forgot')}>try again</a>.
                </p>
              </div>
            )}

            {/* VIEW: Request Access */}
            {view === 'signup' && (
              <div>
                <button type="button" className="back-link" onClick={() => setView('signin')}>&larr; Back to sign in</button>
                <h2>Request access</h2>
                <p className="sub">VaultKeeper is available to verified legal institutions. Submit your details and we&apos;ll be in touch.</p>

                <form onSubmit={handleSignup}>
                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px' }}>
                    <div className="field"><label htmlFor="first">First name</label><input type="text" id="first" placeholder="Jane" required /></div>
                    <div className="field"><label htmlFor="last">Last name</label><input type="text" id="last" placeholder="Doe" required /></div>
                  </div>
                  <div className="field"><label htmlFor="signup-email">Work email</label><input type="email" id="signup-email" placeholder="name@organisation.org" required /></div>
                  <div className="field"><label htmlFor="org">Organisation</label><input type="text" id="org" placeholder="e.g. ICC, UNHCR, ICRC" required /></div>
                  <div className="field"><label htmlFor="role-field">Role</label><input type="text" id="role-field" placeholder="e.g. Lead Investigator" /></div>
                  <button type="submit" className={`auth-btn${loading ? ' loading' : ''}`}>
                    {loading ? 'Please wait\u2026' : 'Submit request'}
                  </button>
                </form>

                <p className="auth-footer">
                  Already have access? <a onClick={() => setView('signin')}>Sign in</a>
                </p>
              </div>
            )}

            {/* VIEW: Request Sent */}
            {view === 'signup-done' && (
              <div>
                <div className="success-icon">
                  <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round"><path d="M22 6L9.5 18 3 12" /></svg>
                </div>
                <h2>Request submitted</h2>
                <p className="sub">Thank you. Our team will review your request and reach out within 2 business days.</p>
                <button type="button" className="auth-btn" onClick={() => setView('signin')}>Back to sign in</button>
              </div>
            )}
          </div>

          <div className="legal-line">
            <Link href="/en/about">Privacy</Link> &middot;{' '}
            <Link href="/en/about">Terms</Link> &middot;{' '}
            <Link href="/en/about">Security</Link>
          </div>
        </div>
      </div>
    </>
  );
}
