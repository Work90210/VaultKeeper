import Link from 'next/link';

export function MarketingFooter() {
  return (
    <footer className="site-foot">
      <div className="wrap">
        <div className="foot-grid">
          <div className="foot-about">
            <Link className="brand" href="/">
              <span className="brand-mark"></span>
              Vault<em>Keeper</em>
            </Link>
            <p>Sovereign, open-source evidence management for international courts, tribunals, and human-rights investigators. The evidence locker no foreign government can shut off.</p>
          </div>
          <div>
            <h5>Platform</h5>
            <ul>
              <li><Link href="/evidence">Evidence management</Link></li>
              <li><Link href="/custody">Chain of custody</Link></li>
              <li><Link href="/witness">Witness protection</Link></li>
              <li><Link href="/collaboration">Live collaboration</Link></li>
              <li><Link href="/search-discovery">Search &amp; discovery</Link></li>
            </ul>
          </div>
          <div>
            <h5>For institutions</h5>
            <ul>
              <li><Link href="/ngos">NGOs in The Hague</Link></li>
              <li><Link href="/midtier">Mid-tier tribunals</Link></li>
              <li><Link href="/icc">ICC-scale bodies</Link></li>
              <li><Link href="/commissions">Truth commissions</Link></li>
              <li><Link href="/pilot">Start a pilot</Link></li>
            </ul>
          </div>
          <div>
            <h5>Open source</h5>
            <ul>
              <li><Link href="/source">Source code</Link></li>
              <li><Link href="/docs">Self-hosting guide</Link></li>
              <li><Link href="/federation">Federation spec (VKE1)</Link></li>
              <li><Link href="/validator">Clerk validator</Link></li>
              <li><Link href="/security">Security audits</Link></li>
            </ul>
          </div>
          <div>
            <h5>Company</h5>
            <ul>
              <li><Link href="/manifesto">Manifesto</Link></li>
              <li><Link href="/contact">Contact</Link></li>
              <li><Link href="/privacy">Privacy</Link></li>
              <li><Link href="/legal">Legal</Link></li>
              <li><Link href="/disclosure">Responsible disclosure</Link></li>
            </ul>
          </div>
        </div>
        <div className="foot-wordmark">VaultKeeper</div>
        <div className="foot-bottom">
          <span>&copy; 2026 VaultKeeper co&ouml;p &middot; The Hague &middot; KvK 94128-041</span>
          <span>AGPL-3.0 &middot; Zero telemetry &middot; Air-gap compatible</span>
          <span>v1.2.0 &middot; released 2026-04-07</span>
        </div>
      </div>
    </footer>
  );
}
