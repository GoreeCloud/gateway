import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { createRoot } from 'react-dom/client'
import {
  Activity, ArrowRight, Check, ChevronDown, CircleHelp, Cloud,
  Command, Copy, ExternalLink, Globe2, HardDrive, KeyRound,
  Layers3, LockKeyhole, LogOut, Menu, Moon, MoreHorizontal,
  Network, Plus, RefreshCw, Search, Server, Settings2,
  ShieldCheck, Sun, Trash2, X
} from 'lucide-react'
import './styles.css'

type Site = { host: string; upstream: string }
type Status = { name: string; engine: string; version: string; siteCount: number; running: boolean; writable: boolean; management: string }
type View = 'overview' | 'sites' | 'security' | 'settings'
const initialStatus: Status = { name: 'GoreeCloud Gateway', engine: 'Caddy v2', version: '0.1.0-dev', siteCount: 0, running: false, writable: false, management: 'loopback' }
const nav: Array<{ id: View; label: string; icon: React.ComponentType<{size?: number; strokeWidth?: number}> }> = [
  { id: 'overview', label: 'Overview', icon: Layers3 },
  { id: 'sites', label: 'Sites & routes', icon: Globe2 },
  { id: 'security', label: 'Security', icon: ShieldCheck },
  { id: 'settings', label: 'Settings', icon: Settings2 }
]

function App() {
  const [token, setToken] = useState('')
  const [draftToken, setDraftToken] = useState('')
  const [loggedIn, setLoggedIn] = useState(false)
  const [status, setStatus] = useState<Status>(initialStatus)
  const [sites, setSites] = useState<Site[]>([])
  const [view, setView] = useState<View>('overview')
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<string | null>(null)
  const [host, setHost] = useState('')
  const [upstream, setUpstream] = useState('http://127.0.0.1:3000')
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [search, setSearch] = useState('')
  const [theme, setTheme] = useState<'dark' | 'light'>('dark')
  const [menuOpen, setMenuOpen] = useState(false)
  const dialogRef = useRef<HTMLElement>(null)
  const formTriggerRef = useRef<HTMLElement | null>(null)
  const mobileMenuRef = useRef<HTMLButtonElement>(null)
  const mobileReturnFocusRef = useRef(false)

  // The dialog owns keyboard focus while open; return focus to its trigger
  // after a close/save so keyboard-only operators do not lose their place.
  useEffect(() => {
    if (!formOpen) {
      const trigger = formTriggerRef.current
      // Save may finish before the busy flag clears, leaving its original
      // trigger temporarily disabled. Retry focus restoration when busy ends.
      if (trigger?.isConnected && trigger.matches(':disabled')) return
      formTriggerRef.current = null
      if (trigger?.isConnected) trigger.focus()
      return
    }
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && !busy) {
        event.preventDefault()
        event.stopPropagation()
        setFormOpen(false)
        return
      }
      if (event.key !== 'Tab') return
      const dialog = dialogRef.current
      if (!dialog) return
      const controls = Array.from(dialog.querySelectorAll<HTMLElement>('button:not(:disabled),input:not(:disabled),a[href]'))
        .filter(control => control.getClientRects().length > 0)
      const first = controls[0]
      const last = controls[controls.length - 1]
      if (!first || !last) return
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault()
        last.focus()
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault()
        first.focus()
      }
    }
    document.addEventListener('keydown', onKeyDown)
    return () => document.removeEventListener('keydown', onKeyDown)
  }, [formOpen, busy])

  useEffect(() => {
    // The menu opener is inside main, which is inert while the menu is shown.
    // Restore focus only after the close render removes that inert state.
    if (!menuOpen) {
      if (mobileReturnFocusRef.current) {
        mobileReturnFocusRef.current = false
        mobileMenuRef.current?.focus()
      }
      return
    }
    mobileReturnFocusRef.current = true
    const onEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setMenuOpen(false)
    }
    document.addEventListener('keydown', onEscape)
    return () => document.removeEventListener('keydown', onEscape)
  }, [menuOpen])

  useEffect(() => { document.documentElement.dataset.theme = theme }, [theme])

  const request = useCallback(async (path: string, init: RequestInit = {}, auth = token) => {
    const response = await fetch(path, { ...init, headers: { Authorization: 'Bearer ' + auth, ...(init.body ? { 'Content-Type': 'application/json' } : {}) }, cache: 'no-store' })
    const data = response.status === 204 ? null : await response.json().catch(() => ({}))
    if (!response.ok) throw new Error(data?.error || 'Request failed (' + response.status + ')')
    return data
  }, [token])

  const refresh = useCallback(async (auth = token) => {
    const [nextStatus, nextSites] = await Promise.all([request('/api/status', {}, auth), request('/api/sites', {}, auth)])
    setStatus(nextStatus); setSites(nextSites.sites || [])
  }, [request, token])

  async function login(e: React.FormEvent) {
    e.preventDefault(); setError(''); setBusy(true)
    try {
      await refresh(draftToken)
      setToken(draftToken); setDraftToken(''); setLoggedIn(true); setMessage('Connected to local Gateway')
    } catch (err) { setError((err as Error).message) } finally { setBusy(false) }
  }
  function logout() { setToken(''); setDraftToken(''); setLoggedIn(false); setSites([]); setStatus(initialStatus); setMessage(''); setView('overview') }
  async function reload() { setBusy(true); setError(''); try { await refresh(); setMessage('Console refreshed') } catch (err) { setError((err as Error).message) } finally { setBusy(false) } }
  function openCreate() { formTriggerRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null; setEditing(null); setHost(''); setUpstream('http://127.0.0.1:3000'); setError(''); setFormOpen(true) }
  function openEdit(site: Site) { formTriggerRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null; setEditing(site.host); setHost(site.host); setUpstream(site.upstream); setError(''); setFormOpen(true) }
  async function save(e: React.FormEvent) {
    e.preventDefault(); setBusy(true); setError('')
    try {
      const path = editing ? '/api/sites/' + encodeURIComponent(editing) : '/api/sites'
      await request(path, { method: editing ? 'PUT' : 'POST', body: JSON.stringify({ host: host.trim().toLowerCase(), upstream: upstream.trim() }) })
      await refresh()
      setFormOpen(false); setEditing(null); setMessage(editing ? 'Route updated' : 'Route created')
    } catch (err) { setError((err as Error).message) } finally { setBusy(false) }
  }
  async function remove(site: Site) {
    if (!window.confirm('Remove ' + site.host + '? Caddy will reload the managed ingress configuration.')) return
    setBusy(true); setError('')
    try {
      await request('/api/sites/' + encodeURIComponent(site.host), { method: 'DELETE' })
      await refresh(); setMessage('Route removed')
    } catch (err) { setError((err as Error).message) } finally { setBusy(false) }
  }
  const filtered = useMemo(() => sites.filter(s => (s.host + ' ' + s.upstream).toLowerCase().includes(search.toLowerCase())), [sites, search])

  if (!loggedIn) return <div className="login-shell">
    <div className="ambient ambient-one" /><div className="ambient ambient-two" />
    <div className="login-top"><span className="brand-glyph"><Network size={21} /></span><span>GoreeCloud <strong>Gateway</strong></span></div>
    <main className="login-card">
      <div className="login-icon"><LockKeyhole size={25} /></div>
      <div className="eyebrow">LOCAL CONTROL PLANE</div>
      <h1>Welcome to your gateway.</h1>
      <p>One secure place to oversee sites and routes on your own infrastructure. Your access token stays in this browser tab's memory.</p>
      <form onSubmit={login}>
        <label htmlFor="token">Management token</label>
        <div className="field-icon"><KeyRound size={19} /><input id="token" type="password" autoComplete="off" spellCheck={false} value={draftToken} onChange={e=>setDraftToken(e.target.value)} minLength={24} placeholder="Enter your admin token" required /></div>
        {error && <div className="inline-error" role="alert">{error}</div>}
        <button className="button primary full" disabled={busy || draftToken.length < 24}>{busy ? 'Connecting…' : 'Unlock console'} <ArrowRight size={18} /></button>
      </form>
      <div className="login-foot"><ShieldCheck size={17}/> Loopback-only connection · No analytics or external sign-in</div>
    </main>
    <footer className="login-bottom">POWERED BY THE CADDY ENGINE <span>·</span> BUILT FOR GOREECLOUD</footer>
  </div>

  return <div className="app-shell">
    <a className="skip-link" href="#main-content">Skip to main content</a>
    <aside inert={formOpen} className={'sidebar ' + (menuOpen ? 'mobile-open' : '')}>
      <div className="side-head"><span className="brand-glyph"><Network size={21}/></span><div className="brand-word">GoreeCloud <strong>Gateway</strong><small>INGRESS CONTROL</small></div><button className="mobile-close icon-button" aria-label="Close navigation" onClick={()=>setMenuOpen(false)}><X size={20}/></button></div>
      <div className="side-section-title">WORKSPACE</div>
      <nav aria-label="Primary navigation">{nav.map(item => {const Icon=item.icon;return <button key={item.id} onClick={()=>{setView(item.id);setMenuOpen(false);setError('');setMessage('')}} className={'nav-link '+(view===item.id?'active':'')} aria-current={view===item.id?'page':undefined}><Icon size={20} strokeWidth={1.9}/>{item.label}{view===item.id&&<span className="nav-current"/>}</button>})}</nav>
      <div className="side-spacer" />
      <div className="side-bottom"><div className="sidebar-instance"><div className="instance-symbol"><Server size={18}/></div><div><strong>Local instance</strong><span>{status.running ? 'Caddy runtime loaded' : 'Awaiting first route'}</span></div><span className={'status-dot '+(status.running?'online':'muted')} /></div><button className="nav-link logout" onClick={logout}><LogOut size={19}/> Lock console</button></div>
    </aside>
    {menuOpen && <button className="backdrop" aria-label="Dismiss navigation" onClick={()=>setMenuOpen(false)}/>}
    <main className="main-area" id="main-content" tabIndex={-1} inert={formOpen || menuOpen}>
      <header className="topbar"><div className="topbar-left"><button ref={mobileMenuRef} className="mobile-menu icon-button" aria-label="Open navigation" onClick={()=>setMenuOpen(true)}><Menu size={22}/></button><div className="breadcrumb">Workspace <span>/</span> <strong>{nav.find(x=>x.id===view)?.label}</strong></div></div><div className="topbar-actions"><span className="local-badge"><span className="tiny-dot"/> Local only</span><button className="icon-button theme-btn" aria-label="Toggle color scheme" title="Toggle color scheme" onClick={()=>setTheme(t=>t==='dark'?'light':'dark')}>{theme==='dark'?<Sun size={19}/>:<Moon size={19}/>}</button><div className="avatar" title="Authenticated operator">GC</div></div></header>
      <div className="content">
        {message && <div className="toast" role="status"><Check size={17}/>{message}<button aria-label="Dismiss message" onClick={()=>setMessage('')}><X size={16}/></button></div>}
        {error && <div className="inline-error page-error" role="alert">{error}<button onClick={()=>setError('')} aria-label="Dismiss error"><X size={16}/></button></div>}
        {view==='overview' && <>
          <div className="page-heading"><div><div className="eyebrow">YOUR INFRASTRUCTURE, IN VIEW</div><h1>Gateway overview<span className="heading-period">.</span></h1><p>Manage your ingress layer with clarity and control.</p></div><div className="heading-actions"><button className="button outline" disabled={busy} onClick={reload}><RefreshCw size={17}/> Refresh</button><button className="button primary" disabled={!status.writable || busy} onClick={openCreate}><Plus size={18}/> Add site</button></div></div>
          <section className="hero"><div className="hero-text"><span className="hero-pill"><span className="hero-pill-dot"/> GATEWAY CONTROL CENTER</span><h2>Everything flows<br/>through here.</h2><p>A private command center for routing applications, websites, and services—powered by Caddy and managed by you.</p><button className="hero-link" onClick={()=>setView('sites')}>Explore your routes <ArrowRight size={17}/></button></div><div className="hero-graphic" aria-hidden="true"><div className="orbit orbit-o"/><div className="orbit orbit-m"/><div className="orbit orbit-i"/><div className="orb orb-left"><Cloud size={21}/></div><div className="orb orb-right"><Server size={20}/></div><div className="orb-center"><Network size={36}/></div><div className="orbit-beam"/></div></section>
          <div className="stats-grid">
            <article className="stat-card"><div className="stat-top"><span className="stat-icon violet"><Globe2 size={21}/></span><MoreHorizontal size={19}/></div><div className="stat-number">{sites.length.toString().padStart(2,'0')}</div><div className="stat-label">Configured routes</div><div className="stat-caption">Hosts under management</div></article>
            <article className="stat-card"><div className="stat-top"><span className="stat-icon teal"><Activity size={21}/></span><MoreHorizontal size={19}/></div><div className="stat-number text-number">{status.running?'Loaded':'Idle'}</div><div className="stat-label">Caddy engine</div><div className="stat-caption">Local configuration state</div></article>
            <article className="stat-card"><div className="stat-top"><span className="stat-icon amber"><LockKeyhole size={21}/></span><MoreHorizontal size={19}/></div><div className="stat-number text-number">{status.writable?'Edit':'View'}</div><div className="stat-label">Console access</div><div className="stat-caption">{status.writable?'Changes enabled':'Read-only protection'}</div></article>
          </div>
          <section className="section-card"><div className="section-head"><div><h2>Recent configuration</h2><p>Managed routes in this instance</p></div><button className="text-link" onClick={()=>setView('sites')}>View all <ArrowRight size={16}/></button></div>{sites.length ? <div className="site-list">{sites.slice(0,4).map(site=><div className="site-list-row" key={site.host}><span className="site-icon"><Globe2 size={18}/></span><div><strong>{site.host}</strong><small>{site.upstream}</small></div><span className="route-tag">Configured</span></div>)}</div> : <div className="empty-state compact"><div className="empty-icon"><Globe2 size={26}/></div><strong>No routes yet</strong><p>Add your first site to connect a domain to an upstream service.</p></div>}</section>
        </>}
        {view==='sites'&&<>
          <div className="page-heading"><div><div className="eyebrow">ROUTING</div><h1>Sites & routes<span className="heading-period">.</span></h1><p>Domains and upstream services managed by this Gateway instance.</p></div><button className="button primary" disabled={!status.writable||busy} onClick={openCreate}><Plus size={18}/> Add site</button></div>
          <section className="section-card"><div className="section-head table-toolbar"><div><h2>All routes <span className="count-chip">{sites.length}</span></h2><p>Changes use Caddy's configuration validation and reload.</p></div><div className="search"><Search size={18}/><input aria-label="Search routes" placeholder="Search routes…" value={search} onChange={e=>setSearch(e.target.value)} /></div></div>
            {filtered.length ? <div className="routes-table"><div className="route-table-head"><span>HOSTNAME</span><span>UPSTREAM</span><span>STATE</span><span>ACTIONS</span></div>{filtered.map(site=><div className="route-table-row" key={site.host}><div className="route-host"><span className="site-icon"><Globe2 size={18}/></span><strong>{site.host}</strong></div><div className="mono upstream" title={site.upstream}>{site.upstream}</div><div><span className="route-tag">Configured</span></div><div className="route-actions"><button className="small-button" disabled={!status.writable||busy} onClick={()=>openEdit(site)}>Edit</button><button className="icon-button danger" disabled={!status.writable||busy} title={'Remove '+site.host} aria-label={'Remove '+site.host} onClick={()=>remove(site)}><Trash2 size={17}/></button></div></div>)}</div> : <div className="empty-state"><div className="empty-icon"><Network size={29}/></div><strong>{search?'No matching routes':'Your routing table is empty'}</strong><p>{search?'Try another search term.':'Connect a hostname to an application to get started.'}</p>{!search && status.writable && <button className="button outline" onClick={openCreate}><Plus size={17}/> Add a route</button>}</div>}
          </section>
          {!status.writable&&<p className="help-line"><LockKeyhole size={16}/> Editing is disabled. Start the service with <code>--allow-write</code> to manage routes.</p>}
        </>}
        {view==='security'&&<>
          <div className="page-heading"><div><div className="eyebrow">TRUST & PRIVACY</div><h1>Security posture<span className="heading-period">.</span></h1><p>Safeguards implemented by the local management console.</p></div></div>
          <div className="security-grid"><article className="info-card"><span className="stat-icon teal"><LockKeyhole size={23}/></span><h2>Loopback by design</h2><p>The management interface only accepts a local loopback IP address. Use an SSH tunnel when administering remotely.</p><span className="info-foot"><Check size={17}/> Enforced by server</span></article><article className="info-card"><span className="stat-icon violet"><KeyRound size={23}/></span><h2>Token-protected API</h2><p>All site and runtime details require a bearer token. This console never puts it in browser storage.</p><span className="info-foot"><Check size={17}/> In-memory session</span></article><article className="info-card"><span className="stat-icon amber"><ShieldCheck size={23}/></span><h2>Explicit change control</h2><p>Write operations are off by default. Configuration is validated, applied, and then saved with rollback on persistence failure.</p><span className="info-foot"><Check size={17}/> Opt-in configuration changes</span></article></div>
          <div className="notice"><CircleHelp size={21}/><div><strong>Production review still required</strong><p>Configured is not the same as healthy. This console does not yet provide certificate inventory, active health probes, multi-user roles, or audit log retention.</p></div></div>
        </>}
        {view==='settings'&&<>
          <div className="page-heading"><div><div className="eyebrow">INSTANCE</div><h1>Gateway settings<span className="heading-period">.</span></h1><p>Environment details and safe local operations.</p></div></div>
          <section className="section-card settings-card"><div className="section-head"><div><h2>Runtime details</h2><p>Information reported by this process</p></div><span className="route-tag">Local</span></div><div className="detail-row"><span>Engine</span><strong>{status.engine}</strong></div><div className="detail-row"><span>Console version</span><strong>{status.version}</strong></div><div className="detail-row"><span>Management access</span><strong>Loopback only</strong></div><div className="detail-row"><span>Editing permission</span><strong>{status.writable?'Enabled':'Read-only'}</strong></div></section>
          <section className="section-card settings-card"><div className="section-head"><div><h2>Quick start</h2><p>Start a local console with your saved secret</p></div></div><div className="command-row"><code>read -rsp "Gateway admin token: " GATEWAY_ADMIN_TOKEN; export GATEWAY_ADMIN_TOKEN; ./gateway serve --allow-write</code><button className="icon-button" title="Copy command" aria-label="Copy command" onClick={async()=>{try{await navigator.clipboard.writeText('read -rsp "Gateway admin token: " GATEWAY_ADMIN_TOKEN; export GATEWAY_ADMIN_TOKEN; ./gateway serve --allow-write');setMessage('Command copied')}catch{setError('Clipboard permission unavailable')}}}><Copy size={18}/></button></div><p className="settings-note">Enter your existing token at the terminal prompt. Your token must be at least 24 characters; never reuse passwords.</p></section>
        </>}
        <footer className="app-footer"><span><Network size={15}/> GoreeCloud Gateway</span><span>Native Caddy engine · Local-first management</span></footer>
      </div>
    </main>
    {formOpen && <div className="modal-backdrop" onMouseDown={e=>{if(e.target===e.currentTarget&&!busy)setFormOpen(false)}}><section ref={dialogRef} className="modal" role="dialog" aria-modal="true" aria-labelledby="dialog-title" aria-describedby="dialog-description"><header><div><div className="eyebrow">MANAGED ROUTING</div><h2 id="dialog-title">{editing?'Edit route':'Add a new site'}</h2></div><button className="icon-button" aria-label="Close dialog" disabled={busy} onClick={()=>setFormOpen(false)}><X size={21}/></button></header><p id="dialog-description" className="modal-intro">Connect a hostname to an existing HTTP or HTTPS application. Domain TLS is handled by the Caddy engine when eligible.</p><form onSubmit={save}><label htmlFor="hostname">Public hostname</label><input id="hostname" value={host} onChange={e=>setHost(e.target.value)} placeholder="app.example.com" required autoFocus spellCheck={false} aria-describedby="hostname-help"/><small id="hostname-help">Use a fully qualified hostname, or localhost for local development.</small><label htmlFor="upstream">Upstream address</label><input id="upstream" value={upstream} onChange={e=>setUpstream(e.target.value)} placeholder="http://127.0.0.1:3000" required spellCheck={false} aria-describedby="upstream-help"/><small id="upstream-help">HTTP(S) URL with no path, credentials, query or fragment.</small>{error&&<div className="inline-error" role="alert">{error}</div>}<div className="modal-actions"><button type="button" className="button outline" disabled={busy} onClick={()=>setFormOpen(false)}>Cancel</button><button type="submit" className="button primary" disabled={busy}>{busy?'Applying…':editing?'Save route':'Create site'} <ArrowRight size={17}/></button></div></form></section></div>}
  </div>
}

createRoot(document.getElementById('root')!).render(<App />)
