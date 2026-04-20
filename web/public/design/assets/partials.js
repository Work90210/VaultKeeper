/* Shared nav/footer injected client-side so pages stay concise */
(function(){
  function current(href){
    const path = location.pathname.split('/').pop() || 'index.html';
    return path === href ? 'active' : '';
  }
  const navHtml = `
    <nav class="nav">
      <div class="nav-inner">
        <a class="brand" href="index.html">
          <span class="brand-mark"></span>
          Vault<em>Keeper</em>
        </a>
        <div class="nav-links">
          <a href="platform.html" class="${current('platform.html')}">Platform</a>
          <a href="security.html" class="${current('security.html')}">Security</a>
          <a href="pricing.html" class="${current('pricing.html')}">Pricing</a>
          <a href="manifesto.html" class="${current('manifesto.html')}">Manifesto</a>
          <a href="docs.html" class="${current('docs.html')}">Open source</a>
        </div>
        <div class="nav-cta">
          <a class="btn ghost sm" href="contact.html">Sign in</a>
          <a class="btn sm" href="contact.html">Request a demo <span class="arr">→</span></a>
        </div>
        <button class="nav-burger" aria-label="Open menu" id="navBurger"><span></span></button>
      </div>
      <div class="mobile-menu" id="mobileMenu">
        <a href="platform.html" class="${current('platform.html')}">Platform</a>
        <a href="security.html" class="${current('security.html')}">Security</a>
        <a href="pricing.html" class="${current('pricing.html')}">Pricing</a>
        <a href="manifesto.html" class="${current('manifesto.html')}">Manifesto</a>
        <a href="docs.html" class="${current('docs.html')}">Open source</a>
        <div class="mob-cta">
          <a class="btn ghost" href="contact.html">Sign in</a>
          <a class="btn" href="contact.html">Request a demo <span class="arr">→</span></a>
        </div>
      </div>
    </nav>`;

  const footHtml = `
    <footer class="site-foot">
      <div class="wrap">
        <div class="foot-grid">
          <div class="foot-about">
            <a class="brand" href="index.html">
              <span class="brand-mark"></span>
              Vault<em>Keeper</em>
            </a>
            <p>Sovereign, open-source evidence management for international courts, tribunals, and human-rights investigators. The evidence locker no foreign government can shut off.</p>
          </div>
          <div>
            <h5>Platform</h5>
            <ul>
              <li><a href="evidence.html">Evidence management</a></li>
              <li><a href="custody.html">Chain of custody</a></li>
              <li><a href="witness.html">Witness protection</a></li>
              <li><a href="collaboration.html">Live collaboration</a></li>
              <li><a href="search.html">Search & discovery</a></li>
            </ul>
          </div>
          <div>
            <h5>For institutions</h5>
            <ul>
              <li><a href="ngos.html">NGOs in The Hague</a></li>
              <li><a href="midtier.html">Mid-tier tribunals</a></li>
              <li><a href="icc.html">ICC-scale bodies</a></li>
              <li><a href="commissions.html">Truth commissions</a></li>
              <li><a href="pilot.html">Start a pilot</a></li>
            </ul>
          </div>
          <div>
            <h5>Open source</h5>
            <ul>
              <li><a href="source.html">Source code</a></li>
              <li><a href="docs.html">Self-hosting guide</a></li>
              <li><a href="federation.html">Federation spec (VKE1)</a></li>
              <li><a href="validator.html">Clerk validator</a></li>
              <li><a href="security.html">Security audits</a></li>
            </ul>
          </div>
          <div>
            <h5>Company</h5>
            <ul>
              <li><a href="manifesto.html">Manifesto</a></li>
              <li><a href="contact.html">Contact</a></li>
              <li><a href="privacy.html">Privacy</a></li>
              <li><a href="legal.html">Legal</a></li>
              <li><a href="disclosure.html">Responsible disclosure</a></li>
            </ul>
          </div>
        </div>
        <div class="foot-wordmark">VaultKeeper</div>
        <div class="foot-bottom">
          <span>© 2026 VaultKeeper coöp · The Hague · KvK 94128-041</span>
          <span>AGPL-3.0 · Zero telemetry · Air-gap compatible</span>
          <span>v1.2.0 · released 2026-04-07</span>
        </div>
      </div>
    </footer>`;

  document.addEventListener('DOMContentLoaded', ()=>{
    const navSlot = document.getElementById('nav');
    const footSlot = document.getElementById('foot');
    if(navSlot) navSlot.outerHTML = navHtml;
    if(footSlot) footSlot.outerHTML = footHtml;

    // Wire up mobile menu
    const burger = document.getElementById('navBurger');
    const menu = document.getElementById('mobileMenu');
    if(burger && menu){
      burger.addEventListener('click', ()=>{
        const isOpen = menu.classList.toggle('open');
        burger.classList.toggle('open', isOpen);
        document.body.classList.toggle('menu-open', isOpen);
      });
      menu.querySelectorAll('a').forEach(a=>{
        a.addEventListener('click', ()=>{
          menu.classList.remove('open');
          burger.classList.remove('open');
          document.body.classList.remove('menu-open');
        });
      });
    }
  });
})();
