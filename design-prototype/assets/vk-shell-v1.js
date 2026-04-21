// VaultKeeper dashboard shell — v5
(function(){
  var ICONS = {
    overview: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><rect x="2" y="2" width="5" height="5" rx="1"/><rect x="9" y="2" width="5" height="5" rx="1"/><rect x="2" y="9" width="5" height="5" rx="1"/><rect x="9" y="9" width="5" height="5" rx="1"/></svg>',
    cases: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><path d="M2 4.5h12v9H2z"/><path d="M6 4.5V3h4v1.5"/></svg>',
    evidence: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><path d="M3 2h7l3 3v9H3z"/><path d="M10 2v3h3"/></svg>',
    witnesses: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><circle cx="8" cy="6" r="2.5"/><path d="M3 13.5c.8-2.5 2.7-4 5-4s4.2 1.5 5 4"/></svg>',
    analysis: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><path d="M3 2.5h10v11H3z"/><path d="M5.5 6h5M5.5 9h5M5.5 11.5h3"/></svg>',
    corrob: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><circle cx="5" cy="5" r="2.5"/><circle cx="11" cy="11" r="2.5"/><path d="M7 7l2 2"/></svg>',
    inquiry: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><circle cx="7" cy="7" r="4"/><path d="M10 10l3 3"/></svg>',
    redact: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><path d="M2 5h5v2H2zM9 9h5v2H9z"/><path d="M2 9h3v2H2zM11 5h3v2h-3z"/></svg>',
    disclosures: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><path d="M3 3h7l3 3v8H3z"/><path d="M5.5 9h5M5.5 11.5h5"/></svg>',
    reports: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><path d="M3 13V4l5-1.5 5 1.5v9"/><path d="M3 13h10"/></svg>',
    search: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><circle cx="7" cy="7" r="4"/><path d="M10 10l3 3"/></svg>',
    audit: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><path d="M3 3l3 3 4-4M3 8l3 3 4-4M3 13h10"/></svg>',
    federation: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><circle cx="4" cy="8" r="2"/><circle cx="12" cy="8" r="2"/><path d="M6 8h4"/></svg>',
    settings: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><circle cx="8" cy="8" r="2"/><path d="M8 2v2M8 12v2M2 8h2M12 8h2M3.5 3.5l1.4 1.4M11.1 11.1l1.4 1.4M3.5 12.5l1.4-1.4M11.1 4.9l1.4-1.4"/></svg>'
  };

  var ORGS = [
    {c:'var(--ink)',letter:'E',name:'Eurojust \xb7 Hague',cases:'3 active cases',active:true},
    {c:'#4a6b3a',letter:'I',name:'ICC \xb7 The Hague',cases:'12 active cases'},
    {c:'#3a4a6b',letter:'C',name:'CIJA \xb7 Brussels',cases:'5 active cases'},
    {c:'#6b3a4a',letter:'K',name:'KSC \xb7 The Hague',cases:'2 active cases'},
    {c:'#8a6a3a',letter:'U',name:'UNHCR \xb7 Geneva',cases:'8 active cases'},
    {c:'#5b4a6b',letter:'R',name:'RSCSL \xb7 Freetown',cases:'1 active case'}
  ];

  window.VK_CASES = [
    {id:'all',name:'All cases',sub:'Cross-case overview',color:'var(--ink)',status:'active'},
    {id:'icc-ukr-2024',name:'ICC-UKR-2024',sub:'Crimes against humanity \xb7 Butcha',color:'var(--ok)',status:'active',role:'Lead',exhibits:'12,417',witnesses:24,corrob:4,disclosures:2},
    {id:'ksc-23-042',name:'KSC-23-042',sub:'Witness intimidation \xb7 Pristina',color:'var(--ok)',status:'active',role:'Analyst',exhibits:'1,204',witnesses:8,corrob:2,disclosures:1},
    {id:'rscsl-12',name:'RSCSL-Residual-12',sub:'Sierra Leone residual',color:'var(--ok)',status:'active',role:'Observer',exhibits:'842',witnesses:3,corrob:1,disclosures:0},
    {id:'irmct-99',name:'IRMCT-99-17',sub:'Archival re-verification',color:'var(--accent)',status:'hold',role:'Clerk',exhibits:'28,918',witnesses:0,corrob:0,disclosures:0}
  ];

  window.vkCurrentCase = window.vkCurrentCase || 'all';

  function navLink(key,label,href,isActive,badge,badgeAccent){
    var cls = isActive?'active':'';
    var b = badge?'<span class="badge'+(badgeAccent?' a':'')+'">'+badge+'</span>':'';
    return '<a href="'+href+'" class="'+cls+'"><span class="ico">'+ICONS[key]+'</span><span>'+label+'</span>'+b+'</a>';
  }

  function buildNav(active){
    var navEl = document.querySelector('.d-nav');
    if(!navEl) return;
    var cc = window.vkCurrentCase;
    var c = cc==='all'?null:VK_CASES.find(function(x){return x.id===cc});

    if(cc==='all'){
      navEl.innerHTML =
        '<div class="nav-label">Workspace</div>'+
        navLink('overview','Overview','dash.html',active==='overview')+
        navLink('cases','Cases','dash-cases.html',active==='cases','38')+
        navLink('evidence','Evidence','dash-evidence.html',active==='evidence','14.2k')+
        '<div class="nav-label">Investigation</div>'+
        navLink('witnesses','Witnesses','dash-witnesses.html',active==='witnesses')+
        navLink('analysis','Analysis notes','dash-analysis.html',active==='analysis')+
        navLink('corrob','Corroborations','dash-corroborations.html',active==='corrob','7')+
        navLink('inquiry','Inquiry log','dash-inquiry.html',active==='inquiry')+
        '<div class="nav-label">Disclosure</div>'+
        navLink('redact','Redaction','dash-redaction.html',active==='redact','live',true)+
        navLink('disclosures','Disclosures','dash-disclosures.html',active==='disclosures','3')+
        navLink('reports','Reports','dash-reports.html',active==='reports')+
        '<div class="nav-label">Platform</div>'+
        navLink('search','Search','dash-search.html',active==='search')+
        navLink('audit','Audit log','dash-audit.html',active==='audit')+
        navLink('federation','Federation','dash-federation.html',active==='federation','VKE1')+
        navLink('settings','Settings','dash-settings.html',active==='settings');
    } else {
      navEl.innerHTML =
        '<div class="nav-label" style="display:flex;justify-content:space-between;align-items:center"><span>'+c.name+'</span><span style="font-size:9px;letter-spacing:.06em;padding:2px 6px;border-radius:4px;background:'+(c.status==='hold'?'rgba(184,66,28,.1)':'rgba(74,107,58,.1)')+';color:'+(c.status==='hold'?'var(--accent)':'var(--ok)')+'">'+c.role+'</span></div>'+
        navLink('overview','Overview','dash.html',active==='overview')+
        navLink('evidence','Evidence','dash-evidence.html',active==='evidence',c.exhibits)+
        navLink('witnesses','Witnesses','dash-witnesses.html',active==='witnesses',String(c.witnesses))+
        '<div class="nav-label">Investigation</div>'+
        navLink('analysis','Analysis notes','dash-analysis.html',active==='analysis')+
        navLink('corrob','Corroborations','dash-corroborations.html',active==='corrob',String(c.corrob))+
        navLink('inquiry','Inquiry log','dash-inquiry.html',active==='inquiry')+
        '<div class="nav-label">Disclosure</div>'+
        navLink('redact','Redaction','dash-redaction.html',active==='redact','live',true)+
        navLink('disclosures','Disclosures','dash-disclosures.html',active==='disclosures',String(c.disclosures))+
        navLink('reports','Reports','dash-reports.html',active==='reports')+
        '<div class="nav-label">Platform</div>'+
        navLink('search','Search','dash-search.html',active==='search')+
        navLink('audit','Audit log','dash-audit.html',active==='audit')+
        navLink('settings','Settings','dash-settings.html',active==='settings');
    }
  }

  window.renderDashShell = function(active, crumbHtml){
    var shell = '\
    <aside class="d-side">\
      <a class="brand" href="index.html"><span class="brand-mark"></span>Vault<em>Keeper</em></a>\
      <div class="d-org" id="d-org-toggle">\
        <span class="av">E</span>\
        <span class="name">Eurojust \xb7 Hague<small>3 active cases</small></span>\
      </div>\
      <div class="case-pick" id="case-pick"></div>\
      <nav class="d-nav"></nav>\
      <div class="who" id="who-btn" style="cursor:pointer">\
        <span class="av">H</span>\
        <span class="n">H. Morel<small>Senior analyst \xb7 \ud83d\udd11 Ed25519</small></span>\
        <span class="dot" title="Signed in"></span>\
      </div>\
    </aside>\
    <div class="d-main">\
      <div class="d-top">\
        <nav class="d-crumb">'+crumbHtml+'</nav>\
        <div class="d-top-actions">\
          <div class="d-search" onclick="location=\'dash-search.html\'">\
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="7" cy="7" r="4"/><path d="M10 10l3 3"/></svg>\
            <span>Search exhibits, witnesses, hashes\u2026</span>\
            <kbd>\u2318K</kbd>\
          </div>\
          <button class="d-iconbtn" title="Notifications" id="notif-btn">\
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><path d="M4 10.5a4 4 0 118 0v1H4zM6.5 12.5a1.5 1.5 0 003 0"/></svg>\
            <span class="notif"></span>\
          </button>\
          <button class="d-iconbtn" title="Help" id="help-btn">\
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><circle cx="8" cy="8" r="6"/><path d="M6.4 6.4a1.6 1.6 0 113 .6c-.6.3-1 .7-1 1.4M8 10.5v.5"/></svg>\
          </button>\
        </div>\
      </div>\
      <main class="d-content" id="d-content"></main>\
    </div>';
    document.getElementById('d-shell').innerHTML = shell;

    // -- Case picker button --
    var caseBtn = document.getElementById('case-pick');
    caseBtn.style.cssText = 'display:grid;grid-template-columns:6px 1fr auto;gap:10px;align-items:center;padding:9px 12px;border:1px solid var(--line);border-radius:10px;cursor:pointer;background:var(--bg);font-size:13px;color:var(--ink);transition:border-color .15s';
    updateCaseBtn();

    // -- Org dropdown (admin, subtle hover icon) --
    var orgEl = document.getElementById('d-org-toggle');
    var swapIcon = document.createElement('span');
    swapIcon.innerHTML = '<svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" style="display:block"><path d="M4 6l-2 2 2 2M12 6l2 2-2 2M6 8h4"/></svg>';
    swapIcon.style.cssText = 'opacity:0;transition:opacity .2s;color:var(--muted);display:grid;place-items:center';
    orgEl.appendChild(swapIcon);
    orgEl.addEventListener('mouseenter',function(){swapIcon.style.opacity='1'});
    orgEl.addEventListener('mouseleave',function(){swapIcon.style.opacity='0'});

    var orgDD = document.createElement('div');
    orgDD.className = 'ctx-dd';
    orgDD.innerHTML = ORGS.map(function(o){
      return '<div class="dd-org'+(o.active?' active':'')+'" style="cursor:pointer"><span class="oa" style="background:'+o.c+'">'+o.letter+'</span><span>'+o.name+'<small>'+o.cases+'</small></span></div>';
    }).join('');
    document.querySelector('.d-side').appendChild(orgDD);

    orgEl.style.cursor = 'pointer';
    orgEl.addEventListener('click',function(e){
      e.stopPropagation();
      caseDD.classList.remove('open');
      var r=orgEl.getBoundingClientRect();
      orgDD.style.top=(r.bottom+4)+'px';
      orgDD.classList.toggle('open');
    });

    // -- Case dropdown --
    var caseDD = document.createElement('div');
    caseDD.className = 'ctx-dd';
    document.querySelector('.d-side').appendChild(caseDD);

    function buildCaseDD(){
      caseDD.innerHTML = VK_CASES.map(function(c){
        return '<div class="dd-case'+(c.id===window.vkCurrentCase?' active':'')+'" data-case="'+c.id+'">'+
          '<span class="cd" style="background:'+(c.id==='all'?'var(--ink)':c.status==='hold'?'var(--accent)':'var(--ok)')+'"></span>'+
          '<span>'+c.name+'<small>'+c.sub+'</small></span></div>';
      }).join('');
    }

    caseBtn.addEventListener('click',function(e){
      e.stopPropagation();
      orgDD.classList.remove('open');
      buildCaseDD();
      var r=caseBtn.getBoundingClientRect();
      caseDD.style.top=(r.bottom+4)+'px';
      caseDD.classList.toggle('open');
    });

    caseDD.addEventListener('click',function(e){
      e.stopPropagation();
      var item = e.target.closest('.dd-case');
      if(!item) return;
      window.vkCurrentCase = item.dataset.case;
      caseDD.classList.remove('open');
      updateCaseBtn();
      updateOrgSub();
      buildNav(active);
      if(typeof window.onCaseChanged === 'function') window.onCaseChanged(window.vkCurrentCase);
    });

    document.addEventListener('click',function(){orgDD.classList.remove('open');caseDD.classList.remove('open');var pm=document.getElementById('profile-menu');if(pm)pm.classList.remove('open');notifPanel.style.display='none';helpPanel.style.display='none'});

    // -- Profile menu --
    var whoBtn = document.getElementById('who-btn');
    var profileMenu = document.createElement('div');
    profileMenu.className = 'ctx-dd';
    profileMenu.id = 'profile-menu';
    profileMenu.innerHTML =
      '<div style="padding:14px 16px;border-bottom:1px solid var(--line)">'+
        '<div style="font-weight:500;font-size:14px">H\u00e9l\u00e8ne Morel</div>'+
        '<div style="font-size:12px;color:var(--muted);margin-top:2px">h.morel@eurojust.example</div>'+
        '<div style="font-size:11px;color:var(--muted);margin-top:4px;font-family:JetBrains Mono,monospace;letter-spacing:.02em">Senior analyst \xb7 Admin</div>'+
      '</div>'+
      '<a href="dash-profile.html" style="display:flex;align-items:center;gap:10px;padding:10px 16px;font-size:13px;color:var(--ink-2);cursor:pointer;transition:background .12s" onmouseover="this.style.background=\'var(--bg-2)\'" onmouseout="this.style.background=\'transparent\'">'+
        '<svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><circle cx="8" cy="6" r="2.5"/><path d="M3 13.5c.8-2.5 2.7-4 5-4s4.2 1.5 5 4"/></svg>'+
        'Edit profile</a>'+
      '<a href="dash-settings.html" style="display:flex;align-items:center;gap:10px;padding:10px 16px;font-size:13px;color:var(--ink-2);cursor:pointer;transition:background .12s" onmouseover="this.style.background=\'var(--bg-2)\'" onmouseout="this.style.background=\'transparent\'">'+
        '<svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><circle cx="8" cy="8" r="2"/><path d="M8 2v2M8 12v2M2 8h2M12 8h2M3.5 3.5l1.4 1.4M11.1 11.1l1.4 1.4M3.5 12.5l1.4-1.4M11.1 4.9l1.4-1.4"/></svg>'+
        'Settings</a>'+
      '<div style="height:1px;background:var(--line);margin:4px 0"></div>'+
      '<a href="sign-in.html" style="display:flex;align-items:center;gap:10px;padding:10px 16px;font-size:13px;color:#b35c5c;cursor:pointer;transition:background .12s" onmouseover="this.style.background=\'var(--bg-2)\'" onmouseout="this.style.background=\'transparent\'">'+
        '<svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="#b35c5c" stroke-width="1.4"><path d="M6 2H3v12h3M11 5l3 3-3 3M14 8H7"/></svg>'+
        'Sign out</a>';
    document.querySelector('.d-side').appendChild(profileMenu);

    whoBtn.addEventListener('click',function(e){
      e.stopPropagation();
      orgDD.classList.remove('open');
      caseDD.classList.remove('open');
      var r=whoBtn.getBoundingClientRect();
      profileMenu.style.top = 'auto';
      profileMenu.style.bottom = (window.innerHeight - r.top + 4) + 'px';
      profileMenu.classList.toggle('open');
    });
    profileMenu.addEventListener('click',function(e){e.stopPropagation()});

    // -- Notifications panel --
    var notifBtn = document.getElementById('notif-btn');
    var notifPanel = document.createElement('div');
    notifPanel.id = 'notif-panel';
    notifPanel.style.cssText = 'display:none;position:absolute;top:54px;right:80px;width:360px;max-height:480px;overflow-y:auto;background:var(--paper);border:1px solid var(--line-2);border-radius:12px;box-shadow:0 2px 6px rgba(20,17,12,.05),0 16px 40px rgba(20,17,12,.08);z-index:999';
    notifPanel.innerHTML =
      '<div style="padding:14px 18px;border-bottom:1px solid var(--line);display:flex;justify-content:space-between;align-items:center"><span style="font-weight:500;font-size:14px">Notifications</span><span style="font-family:JetBrains Mono,monospace;font-size:10px;color:var(--muted);letter-spacing:.06em;text-transform:uppercase">3 unread</span></div>'+
      [
        {t:'Countersign required',d:'Federated merge VKE1\xb791ab\u2026 from CIJA waiting for your signature.',time:'2 min ago',urgent:true},
        {t:'Redaction review',d:'Martyna flagged 3 passages in W-0144 statement for your review.',time:'28 min ago',urgent:true},
        {t:'Disclosure deadline',d:'DISC-2026-019 for defence counsel due Friday. 48 exhibits.',time:'1 h ago',urgent:true},
        {t:'Chain verified',d:'witness-node-02 countersigned block f208\u2026 \xb7 3 ops sealed.',time:'4 h ago',urgent:false},
        {t:'Federation sync complete',d:'CIJA \xb7 Berlin sub-chain synced. 12 new ops.',time:'6 h ago',urgent:false},
        {t:'Key rotation reminder',d:'Quarterly rotation for Key A scheduled in 11 days.',time:'Yesterday',urgent:false}
      ].map(function(n){
        return '<div style="padding:12px 18px;border-bottom:1px solid var(--line);cursor:pointer;transition:background .12s'+(n.urgent?';background:rgba(184,66,28,.03)':'')+'" onmouseover="this.style.background=\'var(--bg-2)\'" onmouseout="this.style.background=\''+(n.urgent?'rgba(184,66,28,.03)':'transparent')+'\'">'+
          '<div style="display:flex;justify-content:space-between;align-items:start;gap:10px">'+
            '<div style="font-size:13.5px;font-weight:500;color:var(--ink)">'+n.t+'</div>'+
            (n.urgent?'<span style="width:6px;height:6px;border-radius:50%;background:var(--accent);flex-shrink:0;margin-top:6px"></span>':'')+
          '</div>'+
          '<div style="font-size:12.5px;color:var(--muted);line-height:1.5;margin-top:4px">'+n.d+'</div>'+
          '<div style="font-family:JetBrains Mono,monospace;font-size:10.5px;color:var(--muted-2);margin-top:6px;letter-spacing:.02em">'+n.time+'</div>'+
        '</div>';
      }).join('')+
      '<div style="padding:12px 18px;text-align:center"><a href="dash-audit.html" style="font-size:13px;color:var(--accent);font-weight:500">View all activity \u2192</a></div>';
    document.querySelector('.d-main').appendChild(notifPanel);

    notifBtn.addEventListener('click',function(e){
      e.stopPropagation();
      helpPanel.style.display='none';
      notifPanel.style.display = notifPanel.style.display==='none'?'block':'none';
    });
    notifPanel.addEventListener('click',function(e){e.stopPropagation()});

    // -- Help panel --
    var helpBtn = document.getElementById('help-btn');
    var helpPanel = document.createElement('div');
    helpPanel.id = 'help-panel';
    helpPanel.style.cssText = 'display:none;position:absolute;top:54px;right:16px;width:300px;background:var(--paper);border:1px solid var(--line-2);border-radius:12px;box-shadow:0 2px 6px rgba(20,17,12,.05),0 16px 40px rgba(20,17,12,.08);z-index:999';
    helpPanel.innerHTML =
      '<div style="padding:14px 18px;border-bottom:1px solid var(--line);font-weight:500;font-size:14px">Help & resources</div>'+
      [
        {icon:'<svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><path d="M3 2.5h10v11H3z"/><path d="M5.5 6h5M5.5 9h5M5.5 11.5h3"/></svg>',label:'Documentation',desc:'Guides, API reference, best practices'},
        {icon:'<svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><circle cx="8" cy="8" r="6"/><path d="M6.4 6.4a1.6 1.6 0 113 .6c-.6.3-1 .7-1 1.4M8 10.5v.5"/></svg>',label:'FAQ',desc:'Common questions about evidence handling'},
        {icon:'<svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><rect x="2" y="2" width="5" height="5" rx="1"/><rect x="9" y="2" width="5" height="5" rx="1"/><rect x="2" y="9" width="5" height="5" rx="1"/><rect x="9" y="9" width="5" height="5" rx="1"/></svg>',label:'Keyboard shortcuts',desc:'Speed up your workflow'},
        {icon:'<svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><path d="M2 4l6-2 6 2v5c0 3-3 5-6 6-3-1-6-3-6-6z"/></svg>',label:'Security & compliance',desc:'Certifications, audit policies'},
        {icon:'<svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><circle cx="8" cy="6" r="2.5"/><path d="M3 13.5c.8-2.5 2.7-4 5-4s4.2 1.5 5 4"/></svg>',label:'Contact support',desc:'support@vaultkeeper.example'}
      ].map(function(h){
        return '<a style="display:flex;align-items:start;gap:12px;padding:12px 18px;cursor:pointer;transition:background .12s;text-decoration:none;color:inherit" onmouseover="this.style.background=\'var(--bg-2)\'" onmouseout="this.style.background=\'transparent\'">'+
          '<span style="color:var(--muted);flex-shrink:0;margin-top:2px">'+h.icon+'</span>'+
          '<div><div style="font-size:13.5px;font-weight:500">'+h.label+'</div><div style="font-size:12px;color:var(--muted);margin-top:1px">'+h.desc+'</div></div></a>';
      }).join('')+
      '<div style="padding:12px 18px;border-top:1px solid var(--line);font-family:JetBrains Mono,monospace;font-size:10.5px;color:var(--muted-2);letter-spacing:.02em">VaultKeeper v2.4.1 \xb7 Build 2026.04.19</div>';
    document.querySelector('.d-main').appendChild(helpPanel);

    helpBtn.addEventListener('click',function(e){
      e.stopPropagation();
      notifPanel.style.display='none';
      helpPanel.style.display = helpPanel.style.display==='none'?'block':'none';
    });
    helpPanel.addEventListener('click',function(e){e.stopPropagation()});

    function updateCaseBtn(){
      var c = VK_CASES.find(function(x){return x.id===window.vkCurrentCase});
      var dotColor = window.vkCurrentCase==='all'?'var(--ink)':c.status==='hold'?'var(--accent)':'var(--ok)';
      caseBtn.innerHTML = '<span style="width:6px;height:6px;border-radius:50%;background:'+dotColor+'"></span><span style="font-weight:500;line-height:1.2">'+c.name+'<small style="display:block;font-weight:400;font-size:11px;color:var(--muted);margin-top:1px">'+c.sub+'</small></span><span style="color:var(--muted);font-size:11px">\u25be</span>';
    }

    function updateOrgSub(){
      var c = VK_CASES.find(function(x){return x.id===window.vkCurrentCase});
      orgEl.querySelector('.name small').textContent = window.vkCurrentCase==='all'?'3 active cases':c.name;
    }

    // Init nav
    updateOrgSub();
    buildNav(active);

    // Expose for pages that need to trigger updates
    window.vkUpdateNav = function(){ buildNav(active); };
    window.vkUpdateCasePicker = function(){ updateCaseBtn(); updateOrgSub(); buildNav(active); };
  };
})();
