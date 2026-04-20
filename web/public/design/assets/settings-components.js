/* =========================================================
   VaultKeeper — Settings page reusable components
   Shared rendering helpers for dash-settings.html
   ========================================================= */

// ---- Atomic components ----

function Panel(header, body, opts) {
  opts = opts || {};
  var cls = opts.bodyClass || '';
  var headerStyle = opts.headerStyle || '';
  return '<div class="panel">' +
    (header ? '<div class="panel-h"' + (headerStyle ? ' style="' + headerStyle + '"' : '') + '>' + header + '</div>' : '') +
    '<div class="panel-body' + (cls ? ' ' + cls : '') + '"' + (opts.bodyStyle ? ' style="' + opts.bodyStyle + '"' : '') + '>' + body + '</div>' +
    '</div>';
}

function PanelHeader(title, meta, opts) {
  opts = opts || {};
  var titleStyle = opts.titleStyle || '';
  return '<h3' + (titleStyle ? ' style="' + titleStyle + '"' : '') + '>' + title + '</h3>' +
    '<span class="meta">' + meta + '</span>';
}

function PanelRaw(header, innerHtml, opts) {
  opts = opts || {};
  var headerStyle = opts.headerStyle || '';
  return '<div class="panel">' +
    (header ? '<div class="panel-h"' + (headerStyle ? ' style="' + headerStyle + '"' : '') + '>' + header + '</div>' : '') +
    innerHtml +
    '</div>';
}

function KVList(pairs) {
  return '<dl class="kvs">' +
    pairs.map(function(p) {
      return '<dt>' + p[0] + '</dt><dd>' + p[1] + '</dd>';
    }).join('') +
    '</dl>';
}

function Toggle(on, label) {
  return '<span class="toggle' + (on ? ' on' : '') + '" onclick="this.classList.toggle(\'on\')"></span>' +
    (label ? ' ' + label : '');
}

function RoleBadge(role) {
  return '<span class="role-badge ' + role + '">' +
    role.charAt(0).toUpperCase() + role.slice(1) +
    '</span>';
}

function CaseChip(caseName) {
  var hold = caseName === 'IRMCT-99' ? ' hold' : '';
  return '<span class="case-chip' + hold + '"><span class="cd"></span>' + caseName + '</span>';
}

function StatusDot(status, label) {
  var color = status === 'online' ? 'var(--ok)' : 'var(--line-2)';
  return '<span style="display:flex;align-items:center;gap:6px">' +
    '<span style="width:6px;height:6px;border-radius:50%;background:' + color + '"></span>' +
    '<span style="font-size:12px;color:var(--muted)">' + label + '</span></span>';
}

function Avatar(letter, color, opts) {
  opts = opts || {};
  var size = opts.size || 36;
  var shape = opts.shape || 'circle';
  var radius = shape === 'square' ? '8px' : '50%';
  var fontSize = opts.fontSize || (size < 36 ? 14 : (size < 40 ? 15 : 16));
  var cls = opts.cls || '';
  return '<span class="' + (cls || 'av') + '" style="width:' + size + 'px;height:' + size + 'px;border-radius:' + radius + ';background:' + color + ';display:grid;place-items:center;font-family:\'Fraunces\',serif;font-style:italic;font-size:' + fontSize + 'px;color:' + (opts.textColor || '#fff') + ';flex-shrink:0">' + letter + '</span>';
}

function LinkArrow(text, opts) {
  opts = opts || {};
  var style = opts.style || '';
  var href = opts.href ? ' href="' + opts.href + '"' : '';
  return '<a class="linkarrow"' + href + (style ? ' style="' + style + '"' : '') + '>' + text + '</a>';
}

function DangerLink(text) {
  return '<a style="font-size:12px;color:#b35c5c;cursor:pointer">' + text + '</a>';
}

// ---- Table header row ----

function GridHeader(columns, templateColumns) {
  return '<div style="font-family:\'JetBrains Mono\',monospace;font-size:10px;letter-spacing:.06em;text-transform:uppercase;color:var(--muted);display:grid;grid-template-columns:' + templateColumns + ';gap:16px;padding:10px 18px;border-bottom:1px solid var(--line);background:color-mix(in oklab,var(--paper) 50%,var(--bg-2))">' +
    columns.map(function(c) { return '<span>' + c + '</span>'; }).join('') +
    '</div>';
}

// ---- Grid row ----

function GridRow(cells, templateColumns, opts) {
  opts = opts || {};
  var style = 'display:grid;grid-template-columns:' + templateColumns + ';gap:16px;align-items:center;padding:14px 18px;border-bottom:1px solid var(--line);font-size:13.5px';
  return '<div style="' + style + '">' + cells.join('') + '</div>';
}

// ---- Invite / action bar at bottom of panel ----

function InviteBar(buttonLabel, roleOptions) {
  roleOptions = roleOptions || ['Analyst', 'Lead Investigator', 'Clerk', 'Viewer', 'Admin'];
  return '<div class="invite-bar">' +
    '<input type="email" placeholder="Email address" />' +
    '<select>' + roleOptions.map(function(r) { return '<option>' + r + '</option>'; }).join('') + '</select>' +
    '<button class="btn sm">' + buttonLabel + '</button>' +
    '</div>';
}

// ---- Permission check mark ----

function PermCheck(value) {
  var cls = value === true ? 'on' : value === 'partial' ? 'partial' : '';
  return '<span class="perm-check ' + cls + '"></span>';
}

// ---- Key card (for Keys & ceremonies) ----

function KeyCard(key) {
  var dot = key.status === 'active' ? 'var(--ok)' : 'var(--line-2)';
  return '<div style="padding:18px;border:1px solid var(--line);border-radius:12px;background:var(--paper)">' +
    '<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">' +
    '<span style="font-family:\'JetBrains Mono\',monospace;font-size:10.5px;letter-spacing:.06em;text-transform:uppercase;color:var(--muted)">' + key.n + '</span>' +
    StatusDot(key.status, '').replace(/font-size:12px;color:var\(--muted\)/, 'display:none') +
    '<span style="width:6px;height:6px;border-radius:50%;background:' + dot + '"></span>' +
    '</div>' +
    '<div style="font-family:\'Fraunces\',serif;font-size:20px;letter-spacing:-.01em;margin-bottom:4px">' + key.who + '</div>' +
    '<div style="font-size:12.5px;color:var(--muted);margin-bottom:10px">' + key.hw + '</div>' +
    '<div style="font-family:\'JetBrains Mono\',monospace;font-size:10.5px;color:var(--muted);letter-spacing:.02em">' + key.last + '</div></div>';
}

// ---- Role definition card ----

function RoleCard(role, memberCount) {
  return '<div style="display:grid;grid-template-columns:1fr auto;gap:16px;align-items:start;padding:14px;border:1px solid var(--line);border-radius:10px">' +
    '<div>' + RoleBadge(role.id) +
    '<p style="font-size:13px;color:var(--muted);line-height:1.5;margin-top:6px">' + role.desc + '</p></div>' +
    '<div style="text-align:right"><span style="font-family:\'JetBrains Mono\',monospace;font-size:20px;color:var(--ink)">' + memberCount + '</span>' +
    '<small style="display:block;font-size:11px;color:var(--muted);margin-top:2px">member' + (memberCount !== 1 ? 's' : '') + '</small></div></div>';
}

// ---- Org switch row ----

function OrgRow(org) {
  return '<div class="org-switch-row' + (org.current ? ' current' : '') + '">' +
    Avatar(org.letter, org.c, { shape: 'square', cls: 'oa', textColor: 'var(--bg)' }) +
    '<div><div style="font-weight:500;font-size:14px">' + org.name + '</div>' +
    '<div style="font-size:12px;color:var(--muted);margin-top:1px">' + org.sub + '</div></div>' +
    (org.current ? '' : LinkArrow('Switch \u2192', { style: 'font-size:12px' })) +
    '</div>';
}
