// Berkeley Protocol flow banner — shared across dashboard pages
function bpFlowBanner(current) {
  var phases = [
    {n:1, l:'Inquiry', p:'dash-inquiry.html'},
    {n:2, l:'Assess', p:'dash-assessments.html'},
    {n:3, l:'Collect', p:'dash-evidence.html'},
    {n:4, l:'Preserve', p:'dash-audit.html'},
    {n:5, l:'Verify', p:'dash-corroborations.html'},
    {n:6, l:'Analyse', p:'dash-analysis.html'}
  ];

  var html = '<div style="display:flex;align-items:center;gap:0;padding:12px 20px;border:1px solid var(--line);border-radius:12px;background:var(--paper);margin-bottom:22px">';
  html += '<div style="font-family:JetBrains Mono,monospace;font-size:9px;letter-spacing:.08em;text-transform:uppercase;color:var(--muted);margin-right:16px;flex-shrink:0">Berkeley<br>Protocol</div>';

  for (var i = 0; i < phases.length; i++) {
    var p = phases[i];
    var cur = p.n === current;
    var past = p.n < current;
    var bg = cur ? 'var(--ink)' : past ? 'var(--ok)' : 'transparent';
    var clr = cur ? 'var(--bg)' : past ? '#fff' : 'var(--muted)';
    var bdr = cur ? 'var(--ink)' : past ? 'var(--ok)' : 'var(--line-2)';
    var lc = cur ? 'var(--ink)' : past ? 'var(--ok)' : 'var(--muted)';
    var icon = past ? '\u2713' : p.n;

    html += '<a href="' + p.p + '" style="display:flex;align-items:center;gap:8px;padding:6px 12px;border-radius:8px;transition:background .12s;text-decoration:none' + (cur ? ';background:var(--bg-2)' : '') + '">';
    html += '<span style="width:22px;height:22px;border-radius:50%;border:1.5px solid ' + bdr + ';background:' + bg + ';color:' + clr + ';display:grid;place-items:center;font-family:JetBrains Mono,monospace;font-size:10px;flex-shrink:0">' + icon + '</span>';
    html += '<span style="font-size:13px;color:' + lc + ';font-weight:' + (cur ? '500' : '400') + ';white-space:nowrap">' + p.l + '</span>';
    html += '</a>';

    if (i < phases.length - 1) {
      html += '<span style="width:20px;height:1px;background:' + (past ? 'var(--ok)' : 'var(--line-2)') + ';flex-shrink:0"></span>';
    }
  }

  // Next phase action
  if (current < 6) {
    var next = phases[current];
    html += '<a href="' + next.p + '" style="margin-left:auto;padding:6px 14px;border-radius:8px;background:rgba(184,66,28,.08);color:var(--accent);font-size:12px;font-family:Fraunces,serif;font-style:italic;text-decoration:none;flex-shrink:0;transition:background .15s" onmouseover="this.style.background=\'rgba(184,66,28,.14)\'" onmouseout="this.style.background=\'rgba(184,66,28,.08)\'">Next: ' + next.l + ' \u2192</a>';
  }

  html += '</div>';
  return html;
}
