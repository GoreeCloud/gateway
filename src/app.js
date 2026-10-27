import { withProxyDraft, pretty, formatNumber, formatTime } from './model.js';

const app = document.querySelector('#app');
const toastElement = document.querySelector('#toast');
const nav = [
  ['overview', 'Overview', 'grid'], ['routes', 'Routes & sites', 'route'],
  ['upstreams', 'Upstreams', 'network'], ['configuration', 'Configuration', 'code'],
  ['activity', 'Activity', 'activity'], ['settings', 'Settings', 'settings'],
];
const glyph = {
  grid: '<rect x="3" y="3" width="7" height="7" rx="2"/><rect x="14" y="3" width="7" height="7" rx="2"/><rect x="3" y="14" width="7" height="7" rx="2"/><rect x="14" y="14" width="7" height="7" rx="2"/>',
  route: '<circle cx="6" cy="5" r="2"/><circle cx="18" cy="19" r="2"/><path d="M6 7v5a4 4 0 0 0 4 4h4a4 4 0 0 1 4 3"/>',
  network: '<rect x="2" y="3" width="20" height="7" rx="2"/><rect x="2" y="14" width="20" height="7" rx="2"/><path d="M6 6.5h.01M6 17.5h.01M11 6.5h7M11 17.5h7"/>',
  code: '<path d="m8 17-5-5 5-5m8 0 5 5-5 5M14 4l-4 16"/>',
  activity: '<path d="M3 12h4l3-8 4 16 3-8h4"/>',
  settings: '<circle cx="12" cy="12" r="3"/><path d="M19.4 15a2 2 0 0 1 .4 2.2l-1.2 1.9a2 2 0 0 1-2.2.8l-1.6-.6a7 7 0 0 1-1.4.8l-.3 1.7H11l-.3-1.7a7 7 0 0 1-1.4-.8l-1.6.6a2 2 0 0 1-2.2-.8l-1.2-1.9a2 2 0 0 1 .4-2.2l1.3-1a8 8 0 0 1 0-1.9l-1.3-1a2 2 0 0 1-.4-2.2l1.2-1.9a2 2 0 0 1 2.2-.8l1.6.6a7 7 0 0 1 1.4-.8L11 3h2.1l.3 1.7a7 7 0 0 1 1.4.8l1.6-.6a2 2 0 0 1 2.2.8l1.2 1.9a2 2 0 0 1-.4 2.2l-1.3 1a8 8 0 0 1 0 1.9z"/>',
  chevron: '<path d="m9 18 6-6-6-6"/>',
  arrow: '<path d="M5 12h14m-6-6 6 6-6 6"/>',
  plus: '<path d="M12 5v14M5 12h14"/>',
  refresh: '<path d="M20 11a8 8 0 0 0-14-5L4 8m0-4v4h4M4 13a8 8 0 0 0 14 5l2-2m0 4v-4h-4"/>',
  search: '<circle cx="11" cy="11" r="7"/><path d="m20 20-4-4"/>',
  shield: '<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><path d="m9 12 2 2 4-4"/>',
  clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
  external: '<path d="M13 5h6v6m0-6-9 9"/><path d="M19 13v6H5V5h6"/>',
  copy: '<rect x="8" y="8" width="12" height="12" rx="2"/><path d="M16 8V4H4v12h4"/>',
  download: '<path d="M12 3v12m-5-5 5 5 5-5M4 17v4h16v-4"/>',
  check: '<path d="m5 12 4 4L19 6"/>',
  lock: '<rect x="5" y="11" width="14" height="11" rx="2"/><path d="M8 11V7a4 4 0 0 1 8 0v4"/>',
  alert: '<path d="m12 3 10 18H2L12 3z"/><path d="M12 9v5m0 3h.01"/>',
  menu: '<path d="M4 6h16M4 12h16M4 18h16"/>',
  sun: '<circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.93 4.93l1.42 1.42m11.3 11.3 1.42 1.42M2 12h2m16 0h2M4.93 19.07l1.42-1.42m11.3-11.3 1.42-1.42"/>',
  moon: '<path d="M20.2 15.5A8.5 8.5 0 0 1 8.5 3.8 8.5 8.5 0 1 0 20.2 15.5z"/>',
  link: '<path d="M10 13a5 5 0 0 0 7.1 0l2-2a5 5 0 1 0-7.1-7.1l-1 1"/><path d="M14 11a5 5 0 0 0-7.1 0l-2 2a5 5 0 1 0 7.1 7.1l1-1"/>',
  logout: '<path d="M10 17l5-5-5-5m5 5H3"/><path d="M12 3h7a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-7"/>',
};
const icon = (name, size = 18) => `<svg width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${glyph[name] ?? glyph.grid}</svg>`;
const escape = v => String(v ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
const state = { session: null, page: 'overview', overview: null, events: [], config: null, editor: '', base: '', dirty: false, validated: '', review: null, search: '', routeFilter: 'all', modal: null, error: null, theme: localStorage.getItem('gateway-theme') || 'dark', menu: false, busy: false };
document.documentElement.dataset.theme = state.theme;

async function request(url, { method = 'GET', data } = {}) {
  const response = await fetch(url, { method, credentials: 'same-origin', headers: { ...(data ? { 'Content-Type': 'application/json' } : {}), ...(method !== 'GET' && state.session?.csrf ? { 'X-CSRF-Token': state.session.csrf } : {}) }, body: data ? JSON.stringify(data) : undefined });
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(payload.error || `Request failed (${response.status})`);
  return payload;
}
let toastTimer;
function toast(message, type = '') {
  toastElement.textContent = message;
  toastElement.className = `toast visible ${type}`;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => toastElement.className = 'toast', 4400);
}
async function initialize() {
  try {
    state.session = await request('/api/session');
    if (state.session.authenticated) await refresh();
    render();
  } catch (error) { state.error = error.message; render(); }
}
async function refresh({ quietly = false } = {}) {
  if (!state.session?.authenticated) return;
  try {
    const [overview, events] = await Promise.all([request('/api/overview'), request('/api/events')]);
    state.overview = overview; state.events = events.events;
    state.error = null;
    if (!quietly) toast(overview.demo ? 'Demo data refreshed' : 'Live Caddy data refreshed', 'success');
  } catch (error) { state.error = error.message; if (!quietly) toast(error.message, 'error'); }
  render();
}
async function ensureConfig({ force = false } = {}) {
  if (state.config && !force) return;
  const record = await request('/api/config');
  state.config = JSON.parse(record.content);
  state.editor = record.content;
  state.base = record.fingerprint;
  state.dirty = false;
  state.validated = '';
  state.review = null;
}
function setPage(page) {
  state.page = page; state.menu = false; state.search = '';
  if (page === 'configuration' && !state.config) ensureConfig().then(render).catch(e => toast(e.message, 'error'));
  render();
  document.querySelector('#main')?.focus();
}
const pageLabels = { overview: 'Overview', routes: 'Routes & sites', upstreams: 'Upstreams', configuration: 'Configuration studio', activity: 'Activity log', settings: 'Settings' };
function shell() {
  const mode = state.overview?.demo ? 'Demo workspace' : state.error ? 'Caddy unreachable' : state.overview?.connected ? (state.overview?.sources && Object.values(state.overview.sources).some(v => v !== 'available') ? 'Caddy connected · partial telemetry' : 'Caddy connected') : 'Checking Caddy';
  return `<div class="layout ${state.menu ? 'menu-open' : ''}">
    <div class="mobile-scrim" data-action="menu" aria-hidden="true"></div>
    <aside class="sidebar" aria-label="Primary navigation">
      <div class="brand"><img src="/mark.svg" alt="" width="43" height="43"><div><strong>GoreeCloud</strong><span>GATEWAY</span></div></div>
      <div class="side-label">WORKSPACE</div>
      <nav class="nav-list">${nav.map(([id, text, glyphName]) => `<button type="button" class="nav-item ${state.page === id ? 'active' : ''}" data-nav="${id}" ${state.page === id ? 'aria-current="page"' : ''}>${icon(glyphName, 19)}<span>${text}</span>${state.page === id ? '<i></i>' : ''}</button>`).join('')}</nav>
      <div class="side-bottom"><div class="side-panel"><div class="side-panel-heading">${icon('shield', 17)} CONTROL PLANE</div><strong>Private by design.</strong><p>Caddy’s admin API stays behind this protected console.</p><span class="side-panel-dot"></span></div>
        <button type="button" class="profile" data-nav="settings"><span class="avatar">GC</span><span class="profile-text"><strong>Administrator</strong><small>GoreeCloud workspace</small></span>${icon('chevron', 17)}</button>
      </div>
    </aside>
    <div class="workspace"><header class="topbar">
      <div class="top-left"><button type="button" class="icon-button mobile-menu" data-action="menu" aria-label="Open navigation">${icon('menu', 21)}</button><div class="breadcrumb"><span>Workspace</span>${icon('chevron', 14)}<strong>${escape(pageLabels[state.page])}</strong></div></div>
      <div class="top-actions"><div class="connection ${state.overview?.demo ? 'is-demo' : state.error ? 'is-error' : ''}"><span class="pulse"></span>${escape(mode)}</div>
      <button type="button" class="icon-button" data-action="theme" title="Toggle appearance" aria-label="Toggle appearance">${icon(state.theme === 'dark' ? 'sun' : 'moon', 19)}</button>
      <button type="button" class="icon-button" data-action="refresh" title="Refresh live data" aria-label="Refresh live data">${icon('refresh', 18)}</button></div>
    </header>
    <main id="main" tabindex="-1" class="main">${state.overview?.demo ? `<div class="demo-banner">${icon('alert', 17)} <span><strong>Preview mode:</strong> all hosts, routes and metrics below are synthetic. No Caddy server is connected; applying is disabled.</span></div>` : ''}${state.error ? `<div class="error-banner" role="alert">${icon('alert', 19)} <span>${escape(state.error)} — verify that Caddy’s loopback admin API is reachable.</span><button type="button" data-action="refresh">Retry</button></div>` : ''}${pageContent()}</main>
    </div>
  </div>${modalMarkup()}`;
}
function header(eyebrow, title, description, action = '') {
  return `<div class="page-heading"><div><div class="eyebrow">${escape(eyebrow)}</div><h1>${escape(title)}</h1><p>${escape(description)}</p></div>${action}</div>`;
}
function button(label, action, type = 'primary', ico = '') {
  return `<button type="button" class="btn ${type}" data-action="${action}">${ico ? icon(ico, 17) : ''}${escape(label)}</button>`;
}
function pageContent() {
  if (!state.overview && state.page !== 'settings') return `<div class="loading-card">${icon('refresh', 25)}<h2>Connecting to Caddy</h2><p>Waiting for current configuration and upstream information.</p>${button('Retry connection', 'refresh', 'secondary')}</div>`;
  switch (state.page) {
    case 'routes': return routesPage();
    case 'upstreams': return upstreamsPage();
    case 'configuration': return configPage();
    case 'activity': return activityPage();
    case 'settings': return settingsPage();
    default: return overviewPage();
  }
}
const smallMeta = (k, v) => `<div class="meta"><span>${escape(k)}</span><strong>${escape(v)}</strong></div>`;
const metricCard = (label, value, extra, ico, color = '') => `<article class="metric-card"><div class="metric-top"><span>${escape(label)}</span><span class="metric-icon ${color}">${icon(ico, 20)}</span></div><strong class="metric-value">${escape(value)}</strong><div class="metric-detail">${escape(extra)}</div></article>`;
function overviewPage() {
  const o = state.overview; const recent = o.routes.slice(0, 5);
  return `${header('SYSTEM OVERVIEW', 'Your edge, at a glance.', 'Manage your reverse proxy infrastructure from a single calm workspace.', button('New reverse proxy', 'new-route', 'primary', 'plus'))}
  <section class="hero"><div class="hero-content"><div class="hero-eyebrow"><span class="hero-dot"></span> GATEWAY CONTROL CENTER</div><h2>Everything flows<br>through here.</h2><p>One place for routes, services and a more confident configuration workflow.</p><div class="hero-footer"><span>${icon('shield', 16)} ${o.demo ? 'Synthetic demonstration' : o.readOnly ? 'Read-only protection is on' : 'Change management enabled'}</span><span>${icon('clock', 16)} Updated ${new Date(o.updatedAt).toLocaleTimeString()}</span></div></div><div class="hero-art" aria-hidden="true"><div class="orbit orbit-one"></div><div class="orbit orbit-two"></div><div class="orbit orbit-three"></div><span class="orb-core">${icon('route', 36)}</span><span class="orb-point p1"></span><span class="orb-point p2"></span><span class="orb-point p3"></span></div></section>
  <section aria-label="Key metrics" class="metric-grid">${metricCard('Hosted sites', formatNumber(o.sites), 'Unique configured hostnames', 'grid')}${metricCard('HTTP routes', formatNumber(o.routeCount), 'Across all active HTTP servers', 'route', 'violet')}${metricCard('HTTP servers', formatNumber(o.servers), 'Configured Caddy listeners', 'network', 'blue')}${metricCard('Requests observed', o.metrics.caddy_http_requests_total == null ? '—' : formatNumber(o.metrics.caddy_http_requests_total), o.sources?.metrics === 'unavailable' ? 'Metrics endpoint unavailable' : o.metrics.caddy_http_requests_total == null ? 'Counter not reported' : 'Counter since Caddy start / reset', 'activity', 'green')}</section>
  <div class="split-grid"><section class="panel"><div class="panel-title"><div><h2>Active routes</h2><p>Your configured destinations</p></div><button type="button" class="text-link" data-nav="routes">View all ${icon('arrow', 15)}</button></div>${recent.length ? `<div class="route-preview-list">${recent.map(route => routePreview(route)).join('')}</div>` : emptyState('No routes configured', 'When Caddy exposes routes they will appear here.')}</section>
  <section class="panel health-panel"><div class="panel-title"><div><h2>System summary</h2><p>Read-only information from Caddy</p></div><span class="quiet-badge">${o.demo ? 'DEMO' : 'LIVE'}</span></div><div class="health-emblem">${icon('shield', 33)}</div><h3>${o.demo ? 'Preview environment' : 'Configuration API reachable'}</h3><p class="muted center">${o.demo ? 'Explore a realistic, synthetic workspace without touching a server.' : 'Latest configuration loaded successfully. This does not establish individual backend health.'}</p><div class="health-stats">${smallMeta('Known upstreams', o.sources?.upstreams === 'unavailable' || o.sources?.upstreams === 'invalid' ? 'Status unavailable' : String(o.upstreams.length))}${smallMeta('Configuration revision', `${o.fingerprint.slice(0, 10)}…`)}${smallMeta('Last snapshot', formatTime(o.updatedAt))}</div></section></div>`;
}
function routePreview(route) {
  const name = route.hosts[0] || 'Catch-all route';
  const target = route.upstreams[0] || route.handlers.join(', ') || 'Handler not identified';
  return `<button type="button" data-action="select-route" data-route="${escape(route.id)}" class="route-preview"><span class="route-avatar">${icon('link', 18)}</span><span class="route-primary"><strong>${escape(name)}</strong><small>${escape(target)}</small></span><span class="route-server">${escape(route.server)}</span>${icon('chevron', 17)}</button>`;
}
function emptyState(title, subtitle) { return `<div class="empty-state">${icon('network', 28)}<strong>${escape(title)}</strong><p>${escape(subtitle)}</p></div>`; }
function routesPage() {
  const o = state.overview;
  const term = state.search.toLowerCase();
  const rows = o.routes.filter(route => (!term || [...route.hosts, ...route.upstreams, route.server].join(' ').toLowerCase().includes(term)) && (state.routeFilter === 'all' || (state.routeFilter === 'proxy' ? route.upstreams.length > 0 : route.upstreams.length === 0)));
  return `${header('TRAFFIC MANAGEMENT', 'Routes & sites', 'Explore every configured route and compose changes as drafts.', button('Add route', 'new-route', 'primary', 'plus'))}
  <div class="section-toolbar"><label class="searchbox">${icon('search', 19)}<span class="sr-only">Filter routes</span><input id="route-search" type="search" placeholder="Search hostnames, upstreams, servers…" value="${escape(state.search)}"><kbd>/</kbd></label><div class="segmented" role="group" aria-label="Route filter"><button type="button" data-filter="all" class="${state.routeFilter === 'all' ? 'selected' : ''}">All <span>${o.routeCount}</span></button><button type="button" data-filter="proxy" class="${state.routeFilter === 'proxy' ? 'selected' : ''}">Proxies</button><button type="button" data-filter="other" class="${state.routeFilter === 'other' ? 'selected' : ''}">Other</button></div></div>
  <section class="panel table-panel"><div class="table-wrap"><table><thead><tr><th>HOSTNAME</th><th>DESTINATION / HANDLER</th><th>SERVER</th><th>LISTENERS</th><th>DETAILS</th></tr></thead><tbody>${rows.map(r => `<tr><td><div class="host-cell"><span class="host-icon">${icon('link', 17)}</span><div><strong>${escape(r.hosts.join(', ') || 'All hosts')}</strong><small>${escape(r.upstreams.length ? 'Reverse proxy' : r.handlers.join(' → ') || 'Route')}</small></div></div></td><td><code>${escape(r.upstreams.join(', ') || r.handlers.join(', ') || '—')}</code></td><td><span class="pill">${escape(r.server)}</span></td><td class="soft">${escape(r.listeners.join(', ') || '—')}</td><td><button type="button" class="mini-button" data-action="select-route" data-route="${escape(r.id)}">Inspect ${icon('chevron', 15)}</button></td></tr>`).join('')}</tbody></table></div>${rows.length ? `<div class="table-footer">Showing ${rows.length} of ${o.routeCount} configured routes · Status is not a runtime health check</div>` : emptyState('No matching routes', 'Adjust the search or filter to see more results.')}</section>`;
}
function upstreamsPage() {
  const o = state.overview;
  const values = o.upstreams;
  const source = o.sources?.upstreams || 'unknown';
  const observed = source === 'available' || source === 'synthetic';
  const missingReason = source === 'invalid' ? 'Caddy returned unrecognized upstream-status data.' : 'Caddy upstream-status endpoint is unavailable.';
  return `${header('BACKEND SERVICES', 'Upstreams', 'Inspect the upstreams currently reported by Caddy. Health checks require Caddy configuration.')}
  <div class="note" role="status">${icon('alert', 17)} ${observed ? 'Request and failure counters do not establish current backend health.' : escape(missingReason) + ' Counters are unknown, not zero.'}</div>
  <section class="metric-grid three">${metricCard('Discovered upstreams', observed ? formatNumber(values.length) : '—', observed ? 'From Caddy upstream status' : 'Status not available', 'network')}${metricCard('Tracked requests', observed ? formatNumber(values.reduce((n, v) => n + v.numRequests, 0)) : '—', observed ? 'Caddy status counters' : 'Status not available', 'activity', 'blue')}${metricCard('Recorded failures', observed ? formatNumber(values.reduce((n, v) => n + v.fails, 0)) : '—', observed ? 'Not a live availability probe' : 'Status not available', 'alert', 'violet')}</section>
  <section class="panel table-panel"><div class="panel-title pad"><div><h2>Reported upstreams</h2><p>Addresses and operational counters</p></div></div><div class="table-wrap"><table><thead><tr><th>UPSTREAM ADDRESS</th><th>REQUESTS</th><th>FAILURES</th><th>OBSERVATION</th></tr></thead><tbody>${values.map(v => `<tr><td><div class="host-cell"><span class="host-icon">${icon('network', 17)}</span><strong>${escape(v.address)}</strong></div></td><td>${formatNumber(v.numRequests)}</td><td>${formatNumber(v.fails)}</td><td><span class="pill neutral">Reported by Caddy</span></td></tr>`).join('')}</tbody></table></div>${values.length ? `<div class="table-footer">${values.length} upstream records returned by Caddy</div>` : emptyState(observed ? 'No upstreams reported' : 'Status endpoint unavailable', observed ? 'Caddy returned no upstream records. This is not a health result.' : 'Live upstream counters cannot be determined from this request.')}</section>`;
}
function configPage() {
  if (!state.config) return `${header('CONFIGURATION', 'Configuration studio', 'Loading the current configuration…')}<div class="loading-card">Loading configuration…</div>`;
  const serverNames = Object.keys(state.config?.apps?.http?.servers ?? {});
  const parsedBytes = new TextEncoder().encode(state.editor).length;
  return `${header('CONFIGURATION STUDIO', 'Work with confidence.', 'Review, edit and validate a JSON draft before considering a live reload.', button('Reload from Caddy', 'reload-config', 'secondary', 'refresh'))}
  <div class="config-grid"><section class="panel editor-panel"><div class="editor-toolbar"><div><span class="editor-dot"></span><strong>caddy.json</strong><span class="editor-path">Live configuration · editable draft</span></div><span class="editor-meta">${formatNumber(parsedBytes)} bytes</span></div>
   <label for="config-editor" class="sr-only">Caddy JSON draft</label><textarea id="config-editor" class="config-editor" spellcheck="false" autocapitalize="off" autocomplete="off" autocorrect="off">${escape(state.editor)}</textarea>
   <div class="editor-bottom"><div class="revision">${icon('lock', 14)} Base revision <code>${escape(state.base.slice(0, 12))}</code>${state.dirty ? '<span class="unsaved">• Unsaved draft</span>' : '<span class="saved">• Unchanged</span>'}</div><div class="editor-actions">${button('Download', 'download', 'subtle', 'download')}${button('Validate JSON', 'validate', 'secondary', 'check')}${button('Preview changes', 'preview-draft', 'secondary', 'search')}${button('Review & apply', 'review-apply', 'primary', 'arrow')}</div></div></section>
  <aside class="config-aside"><section class="panel info-card"><span class="card-symbol violet-bg">${icon('shield', 22)}</span><h3>Protected by default</h3><p>${state.session.readOnly ? 'Live changes are disabled. Enable them explicitly in the server environment after review.' : 'Live reload requires a matching base revision and a deliberate APPLY confirmation.'}</p><div class="quiet-badge">${state.session.readOnly ? 'READ ONLY' : 'RELOAD ENABLED'}</div></section>
  <section class="panel info-card"><span class="card-symbol blue-bg">${icon('code', 21)}</span><h3>Draft verification</h3><p>JSON validation checks syntax and basic structure. It is not a full Caddy runtime validation or deployment guarantee.</p><div class="validation-result">${state.validated ? `${icon('check', 17)} ${escape(state.validated)}` : 'No validation run on this draft yet'}</div></section>
  <section class="panel info-card"><span class="card-symbol teal-bg">${icon('route', 21)}</span><h3>Available HTTP servers</h3><div class="server-pills">${serverNames.length ? serverNames.map(s => `<span class="pill">${escape(s)}</span>`).join('') : '<span class="soft">None</span>'}</div><p>New proxy routes can be inserted into an existing server as an editable draft.</p></section></aside></div>`;
}
function activityPage() {
  return `${header('AUDIT & HISTORY', 'Recent activity', 'Session and reload events recorded by this gateway process. This is not a durable system audit trail.')}
  <section class="panel activity-panel"><div class="panel-title"><div><h2>Gateway events</h2><p>Newest first · in-memory history</p></div><span class="quiet-badge">${state.events.length} ENTRIES</span></div>${state.events.length ? `<div class="timeline">${state.events.map(e => `<div class="timeline-row"><div class="timeline-icon">${icon(e.type === 'apply' ? 'code' : e.type === 'login' ? 'lock' : 'activity', 17)}</div><div><strong>${escape(({ login: 'Administrator signed in', logout: 'Administrator signed out', apply: 'Caddy configuration applied', demo: 'Demo workspace initialized' })[e.type] || e.type)}</strong><p>${escape(e.detail)}</p></div><time datetime="${escape(e.time)}">${escape(formatTime(e.time))}</time></div>`).join('')}</div>` : emptyState('No activity yet', 'Sign-in and configuration actions will appear here.')}</section>`;
}
function settingsPage() {
  return `${header('WORKSPACE', 'Settings', 'Adjust local appearance and review this instance’s security posture.')}
  <div class="settings-grid"><section class="panel settings-card"><h2>Appearance</h2><p>Choose a comfortable workspace theme. Only this preference is stored locally.</p><div class="theme-choices"><button type="button" data-theme="dark" class="theme-choice ${state.theme === 'dark' ? 'chosen' : ''}">${icon('moon', 19)} Dark canvas ${state.theme === 'dark' ? icon('check', 18) : ''}</button><button type="button" data-theme="light" class="theme-choice ${state.theme === 'light' ? 'chosen' : ''}">${icon('sun', 19)} Light canvas ${state.theme === 'light' ? icon('check', 18) : ''}</button></div></section>
  <section class="panel settings-card"><h2>Security</h2><p>Administrative access is managed by the gateway’s server process.</p><div class="settings-rows">${smallMeta('Session', state.session.authenticated ? 'Authenticated' : 'Signed out')}${smallMeta('Live config writes', state.session.readOnly ? 'Disabled' : 'Explicitly enabled')}${smallMeta('Caddy API exposure', 'Server-side loopback only')}${smallMeta('Config storage in browser', 'No persistent draft storage')}</div></section>
  <section class="panel settings-card"><h2>About Gateway</h2><p>An original GoreeCloud interface, aligned with the GC UI design language.</p><div class="settings-rows">${smallMeta('Edition', 'Gateway Alpha 0.2.1')}${smallMeta('Source', 'GoreeCloud/gateway')}${smallMeta('Caddy data', state.overview?.demo ? 'Synthetic preview' : 'Live API read')}</div><p><a href="https://caddyserver.com/docs/api" target="_blank" rel="noopener noreferrer">Caddy API documentation ${icon('external', 14)}</a></p></section>
  <section class="panel settings-card"><h2>Session</h2><p>Sign out to invalidate this browser session. Existing other sessions remain valid until they expire or the gateway restarts.</p>${state.session.demo ? '<span class="quiet-badge">DEMO SESSION</span>' : button('Sign out', 'logout', 'secondary', 'logout')}</section></div>`;
}
function loginMarkup() {
  return `<div class="login-screen"><div class="login-glow" aria-hidden="true"></div><div class="login-panel"><div class="login-brand"><img src="/mark.svg" width="48" height="48" alt=""><div>GoreeCloud <span>GATEWAY</span></div></div><div class="login-icon">${icon('shield', 32)}</div><div class="eyebrow">SECURE CONTROL PLANE</div><h1>Welcome to your edge.</h1><p>Sign in to view and manage your Caddy infrastructure. The access token is never stored in your browser.</p><form id="login-form"><label for="access-token">Administrator access token</label><input type="password" name="token" id="access-token" placeholder="Enter your access token" required autocomplete="off" minlength="32"><button type="submit" class="btn primary login-button">Unlock workspace ${icon('arrow', 18)}</button></form><small class="login-foot">${icon('lock', 14)} Private access · HttpOnly session · No cloud dependency</small></div><div class="login-bottom">GoreeCloud Gateway <span>Built for ownership and control</span></div></div>`;
}
function reviewMarkup() {
  const review = state.review;
  if (!review) return '';
  return `<section class="review-impact" aria-label="Configuration changes">
    <div class="review-stats"><span><strong>${review.counts.added}</strong> added</span><span><strong>${review.counts.removed}</strong> removed</span><span><strong>${review.counts.modified}</strong> modified</span></div>
    <p class="review-note">Structural JSON paths only. Previous and proposed values are intentionally not displayed. No Caddy runtime validation has occurred.</p>
    ${review.changes.length ? `<ol class="review-changes">${review.changes.slice(0, 20).map(change => `<li><span class="change-kind ${escape(change.operation)}">${escape(change.operation)}</span><code title="${escape(change.path)}">${escape(change.path)}</code>${change.note ? `<small>${escape(change.note)}</small>` : ''}</li>`).join('')}</ol>` : '<p class="review-note">No semantic changes from the live revision.</p>'}
    ${review.changes.length > 20 || review.truncated ? '<p class="review-note">Preview is abbreviated. Inspect the complete draft before applying.</p>' : ''}
    ${review.warnings.map(w => `<p class="review-warning">${icon('alert', 15)} ${escape(w)}</p>`).join('')}
  </section>`;
}
function modalMarkup() {
  if (!state.modal) return '';
  let body = '';
  if (state.modal.type === 'new-route') {
    const servers = Object.keys(state.config?.apps?.http?.servers ?? {});
    body = `<div class="dialog-head"><span class="dialog-icon">${icon('plus', 21)}</span><button type="button" data-action="close-modal" class="icon-button" aria-label="Close dialog">×</button></div><h2 id="dialog-title">New reverse proxy draft</h2><p>Create an editable route on an existing Caddy HTTP server. Nothing changes live until you review and apply the draft.</p><form id="new-route-form"><label for="new-host">Public hostname</label><input id="new-host" name="host" placeholder="app.example.com" required><label for="new-upstream">Upstream address</label><input id="new-upstream" name="dial" placeholder="127.0.0.1:8080" required><label for="new-server">HTTP server</label><select name="server" id="new-server" required>${servers.map(s => `<option value="${escape(s)}">${escape(s)}</option>`).join('')}</select><div class="dialog-note">This is a draft-only change. Review listener, TLS and routing semantics before a live reload.</div><div class="dialog-actions"><button type="button" class="btn secondary" data-action="close-modal">Cancel</button><button type="submit" class="btn primary" ${servers.length ? '' : 'disabled'}>Create draft ${icon('arrow', 17)}</button></div></form>`;
  } else if (state.modal.type === 'preview') {
    body = `<div class="dialog-head"><span class="dialog-icon">${icon('search', 21)}</span><button type="button" data-action="close-modal" class="icon-button" aria-label="Close dialog">×</button></div><h2 id="dialog-title">Change preview</h2><p>Compared this draft with the current Caddy JSON revision. This preview does not modify the server.</p>${reviewMarkup()}<div class="dialog-actions">${button('Close', 'close-modal', 'secondary')}${state.session.readOnly || !state.review?.changed ? '' : button('Continue to apply review', 'review-apply', 'primary', 'arrow')}</div>`;
  } else if (state.modal.type === 'apply') {
    body = `<div class="dialog-head"><span class="dialog-icon danger">${icon('alert', 22)}</span><button type="button" data-action="close-modal" class="icon-button" aria-label="Close dialog">×</button></div><h2 id="dialog-title">Review live configuration change</h2><p>A successful request reloads Caddy immediately. Gateway creates a private pre-change snapshot, but changing proxy configuration can affect traffic. Basic JSON validation is not a runtime guarantee.</p>${reviewMarkup()}<div class="apply-summary">${smallMeta('Base revision', `${state.base.slice(0, 14)}…`)}${smallMeta('Editor bytes', String(new TextEncoder().encode(state.editor).length))}${smallMeta('Write access', state.session.readOnly ? 'Disabled' : 'Enabled')}</div><form id="apply-form"><label for="apply-confirm">Type <strong>APPLY</strong> to confirm</label><input name="confirm" id="apply-confirm" placeholder="APPLY" required autocomplete="off"><div class="dialog-actions"><button type="button" class="btn secondary" data-action="close-modal">Cancel</button><button type="submit" class="btn danger-btn" ${state.session.readOnly ? 'disabled' : ''}>Apply to Caddy</button></div></form>`;
  } else if (state.modal.type === 'route') {
    const route = state.overview?.routes.find(r => r.id === state.modal.route);
    body = `<div class="dialog-head"><span class="dialog-icon">${icon('route', 21)}</span><button type="button" data-action="close-modal" class="icon-button" aria-label="Close dialog">×</button></div><h2 id="dialog-title">Route details</h2><p>${escape(route?.hosts.join(', ') || 'Catch-all / unnamed route')}</p><div class="apply-summary">${smallMeta('HTTP server', route?.server || '—')}${smallMeta('Listeners', route?.listeners.join(', ') || '—')}${smallMeta('Handlers', route?.handlers.join(' → ') || '—')}${smallMeta('Upstreams', route?.upstreams.join(', ') || '—')}</div><div class="dialog-actions">${button('Close', 'close-modal', 'secondary')}${button('Open configuration', 'go-config', 'primary', 'code')}</div>`;
  }
  return `<div class="dialog-backdrop" data-action="close-modal"><div class="dialog" role="dialog" aria-modal="true" aria-labelledby="dialog-title" tabindex="-1">${body}</div></div>`;
}
function render() {
  const focused = document.activeElement;
  const preserveSearch = focused?.id === 'route-search' ? { id: 'route-search', index: focused.selectionStart } : null;
  // Avoid overwriting a user's active draft editor while they type.
  if (focused?.id === 'config-editor' && !state.modal && state.page === 'configuration' && app.querySelector('.layout')) return;
  app.innerHTML = state.session?.authenticated ? shell() : loginMarkup();
  if (preserveSearch) { const el = document.getElementById(preserveSearch.id); el?.focus(); el?.setSelectionRange(preserveSearch.index, preserveSearch.index); }
  document.querySelector('.dialog')?.focus();
}
async function openRouteModal() {
  try { await ensureConfig(); state.modal = { type: 'new-route' }; render(); document.getElementById('new-host')?.focus(); }
  catch (e) { toast(e.message, 'error'); }
}
async function validate() {
  try { const r = await request('/api/validate', { method: 'POST', data: { content: state.editor } }); state.validated = `${r.summary} parsed route(s) · basic JSON valid`; toast('Draft passed basic JSON checks', 'success'); render(); return true; }
  catch (e) { state.validated = ''; state.review = null; toast(e.message, 'error'); return false; }
}
async function reviewDraft() {
  const result = await request('/api/review', { method: 'POST', data: { content: state.editor, baseFingerprint: state.base } });
  state.review = result;
  return result;
}
function download() {
  const blob = new Blob([state.editor], { type: 'application/json' });
  const href = URL.createObjectURL(blob);
  const anchor = document.createElement('a'); anchor.href = href; anchor.download = 'caddy-draft.json'; document.body.appendChild(anchor); anchor.click(); anchor.remove(); URL.revokeObjectURL(href);
  toast('Draft downloaded to your device', 'success');
}
async function action(name, el) {
  try {
    switch (name) {
      case 'menu': state.menu = !state.menu; render(); break;
      case 'refresh': await refresh(); break;
      case 'theme': chooseTheme(state.theme === 'dark' ? 'light' : 'dark'); break;
      case 'new-route': await openRouteModal(); break;
      case 'close-modal': state.modal = null; render(); break;
      case 'select-route': state.modal = { type: 'route', route: el.dataset.route }; render(); break;
      case 'go-config': state.modal = null; setPage('configuration'); break;
      case 'reload-config': if (state.dirty && !confirm('Discard your current draft and reload from Caddy?')) return; await ensureConfig({ force: true }); render(); toast('Live configuration fetched again', 'success'); break;
      case 'validate': await validate(); break;
      case 'download': download(); break;
      case 'preview-draft': await reviewDraft(); state.modal = { type: 'preview' }; render(); break;
      case 'review-apply': {
        if (state.session.readOnly) return toast('Live writes are disabled by server policy', 'error');
        if (!await validate()) return;
        const review = await reviewDraft();
        if (!review.changed) return toast('The draft matches the live configuration', 'success');
        state.modal = { type: 'apply' }; render(); break;
      }
      case 'logout': await request('/api/logout', { method: 'POST', data: {} }); state.session = await request('/api/session'); state.overview = null; state.config = null; state.review = null; state.events = []; render(); break;
    }
  } catch (e) { toast(e.message, 'error'); }
}
function chooseTheme(next) { state.theme = next; localStorage.setItem('gateway-theme', next); document.documentElement.dataset.theme = next; render(); }
document.addEventListener('click', e => {
  const navBtn = e.target.closest('[data-nav]');
  if (navBtn) return void setPage(navBtn.dataset.nav);
  const filter = e.target.closest('[data-filter]');
  if (filter) { state.routeFilter = filter.dataset.filter; return render(); }
  const theme = e.target.closest('[data-theme]');
  if (theme) return chooseTheme(theme.dataset.theme);
  const button = e.target.closest('[data-action]');
  if (!button) return;
  if (button.classList.contains('dialog-backdrop') && e.target !== button) return;
  void action(button.dataset.action, button);
});
document.addEventListener('input', e => {
  if (e.target.id === 'route-search') { state.search = e.target.value; render(); }
  if (e.target.id === 'config-editor') { state.editor = e.target.value; state.dirty = true; state.validated = ''; state.review = null; }
});
document.addEventListener('submit', async e => {
  if (!['login-form', 'new-route-form', 'apply-form'].includes(e.target.id)) return;
  e.preventDefault();
  if (state.busy) return;
  state.busy = true;
  try {
    const fields = Object.fromEntries(new FormData(e.target));
    if (e.target.id === 'login-form') {
      state.session = await request('/api/login', { method: 'POST', data: { token: fields.token } });
      await refresh({ quietly: true }); render(); toast('Workspace unlocked', 'success');
    } else if (e.target.id === 'new-route-form') {
      await ensureConfig();
      const current = JSON.parse(state.editor);
      const draft = withProxyDraft(current, fields);
      state.editor = pretty(draft); state.dirty = true; state.validated = ''; state.review = null; state.modal = null;
      setPage('configuration'); toast('Proxy route added to your local draft', 'success');
    } else {
      const result = await request('/api/apply', { method: 'POST', data: { content: state.editor, baseFingerprint: state.base, confirm: fields.confirm } });
      state.modal = null; await ensureConfig({ force: true }); await refresh({ quietly: true }); render();
      toast(result.changed ? `Caddy reloaded; snapshot ${result.backup}` : 'No changes to apply', 'success');
    }
  } catch (err) { toast(err.message, 'error'); }
  finally { state.busy = false; }
});
document.addEventListener('keydown', e => {
  if (e.key === 'Escape' && state.modal) { state.modal = null; render(); }
  const tag = document.activeElement?.tagName;
  if (e.key === '/' && !['INPUT', 'TEXTAREA', 'SELECT'].includes(tag) && state.session?.authenticated) {
    e.preventDefault(); setPage('routes'); document.querySelector('#route-search')?.focus();
  }
});
setInterval(() => { if (state.session?.authenticated && !state.modal && state.page !== 'configuration') void refresh({ quietly: true }); }, 30000);
void initialize();
