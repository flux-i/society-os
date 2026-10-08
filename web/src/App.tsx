import { useFragmentSync } from './navigation'
import { lazy, Suspense, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { canReadChecklists } from './checklists'
import { APIError, mutate, request, setSession, statuses, userAccessScope } from './api'
import type { Building, Flat, FlatDetail, FlatPage, Summary, User } from './api'
import { Icon } from './components/Icon'
import type { IconName } from './components/Icon'
import { Overview } from './components/Overview'
import { Login } from './components/Login'
import { RegistryEditor, RegistryHistory, relationshipLabel } from './components/RegistryEditor'
import { Access, AccountLink, AccountSecurity, MFAGate } from './components/Identity'

import { PortalDialog } from './components/PortalDialog'
import { FilterSelect } from './components/FilterSelect'
import { useSocietyTools } from './webmcp'

import { ScreenBoundary, ScreenLoading } from './components/ScreenBoundary'

const MoveChecklists=lazy(()=>import('./components/MoveChecklists').then(module=>({default:module.MoveChecklists})))
const Records=lazy(()=>import('./components/Records').then(module=>({default:module.Records})))
const Reviews=lazy(()=>import('./components/Reviews').then(module=>({default:module.Reviews})))
const Community=lazy(()=>import('./components/Community').then(module=>({default:module.Community})))
const Complaints=lazy(()=>import('./components/Complaints').then(module=>({default:module.Complaints})))
const Incidents=lazy(()=>import('./components/Incidents').then(module=>({default:module.Incidents})))
const Documents=lazy(()=>import('./components/Documents').then(module=>({default:module.Documents})))
const Maintenance=lazy(()=>import('./components/Maintenance').then(module=>({default:module.Maintenance})))
const Upkeep=lazy(()=>import('./components/Upkeep').then(module=>({default:module.Upkeep})))
const Collections=lazy(()=>import('./components/Collections').then(module=>({default:module.Collections})))
const Fines=lazy(()=>import('./components/Fines').then(module=>({default:module.Fines})))
const Contacts=lazy(()=>import('./components/Contacts').then(module=>({default:module.Contacts})))
const Messages=lazy(()=>import('./components/Messages').then(module=>({default:module.Messages})))
const Statements=lazy(()=>import('./components/Statements').then(module=>({default:module.Statements})))

type View = 'overview' | 'homes' | 'access' | 'security' | 'entries' | 'receipts' | 'reviews' | 'community' | 'help' | 'documents' | 'maintenance' | 'upkeep' | 'collections' | 'conduct' | 'fines' | 'contacts' | 'messages' | 'statements'
const currentView = (): View => { const hash = window.location.hash.slice(1).split('?')[0]; return ['homes', 'access', 'security', 'entries', 'receipts', 'reviews', 'community', 'help', 'documents', 'maintenance', 'upkeep', 'collections', 'conduct', 'fines', 'contacts', 'messages', 'statements'].includes(hash) ? hash as View : 'overview' }
const takeLink = () => {
  const match = window.location.hash.match(/^#(?:activate|reset)=([A-Za-z0-9_-]{43})$/)
  if (!match) return ''
  window.history.replaceState(null, '', window.location.pathname)
  return match[1]
}
const initialLink = takeLink()

function Brand() {
  return <a href="#overview" className="brand" aria-label="Society OS overview"><span className="brand-mark"><svg aria-hidden="true" viewBox="0 0 32 32" fill="none"><path d="M6 26V12l10-7 10 7v14h-7v-9h-6v9Z" stroke="currentColor" strokeWidth="1.8" /><circle cx="16" cy="12" r="2" fill="currentColor" /></svg></span><span>society<span className="brand-dot">.</span><small>A place for everything</small></span></a>
}

function Sidebar({ view, user, onAbout }: { view: View; user: User; onAbout: () => void }) {
  const [menuOpen, setMenuOpen] = useState(false)
  const menuButton = useRef<HTMLButtonElement>(null)
  useEffect(() => { setMenuOpen(false) }, [view])
  return <aside className="sidebar" onKeyDown={event => { if (event.key === 'Escape') { setMenuOpen(false); menuButton.current?.focus() } }}>
    <Brand />
    <button ref={menuButton} className="mobile-nav-toggle" aria-expanded={menuOpen} aria-controls="workspace-navigation" onClick={() => setMenuOpen(value => !value)}>Menu<Icon name={menuOpen ? 'close' : 'menu'} /></button>
    <div id="workspace-navigation" className={`sidebar-space ${menuOpen ? 'menu-open' : ''}`} onClick={event => { if ((event.target as Element).closest('a')) { setMenuOpen(false); if (menuOpen) requestAnimationFrame(() => document.getElementById('main-content')?.focus({ preventScroll: true })) } }}><span className="eyebrow">YOUR WORKSPACE</span>
      <nav aria-label="Main navigation" className="nav-list">
        {<a href="#overview" className={`nav-item ${view === 'overview' ? 'active' : ''}`} aria-current={view === 'overview' ? 'page' : undefined}><Icon name="overview" /><span>Overview</span>{view === 'overview' && <span className="active-dot" />}</a>}
        <a href="#homes" className={`nav-item ${view === 'homes' ? 'active' : ''}`} aria-current={view === 'homes' ? 'page' : undefined}><Icon name="homes" /><span>{user.can_read_registry ? 'Homes & people' : 'Your homes'}</span>{view === 'homes' && <span className="active-dot" />}</a>
        {user.can_manage_accounts && <a href="#access" className={`nav-item ${view === 'access' ? 'active' : ''}`} aria-current={view === 'access' ? 'page' : undefined}><Icon name="community" /><span>Access & invitations</span></a>}
        {user.can_read_records && (['entries', 'receipts'] as const).map(item => <a key={item} href={'#' + item} className={`nav-item ${view === item ? 'active' : ''}`} aria-current={view === item ? 'page' : undefined}><Icon name={item === 'entries' ? 'records' : 'receipt'} /><span>{item === 'entries' ? 'Entries' : 'Receipts'}</span></a>)}
        {user.can_read_records && <a href="#maintenance" className={`nav-item ${view === 'maintenance' ? 'active' : ''}`} aria-current={view === 'maintenance' ? 'page' : undefined}><Icon name="leaf" /><span>Maintenance</span></a>}
        {user.can_read_records && <a href="#collections" className={`nav-item ${view === 'collections' ? 'active' : ''}`} aria-current={view === 'collections' ? 'page' : undefined}><Icon name="leaf" /><span>Collections</span></a>}
        <a href="#reviews" className={`nav-item ${view === 'reviews' ? 'active' : ''}`} aria-current={view === 'reviews' ? 'page' : undefined}><Icon name="check" /><span>{user.can_review_requests ? 'Requests & approvals' : 'Your requests'}</span></a>
        <a href="#community" className={`nav-item ${view === 'community' ? 'active' : ''}`} aria-current={view === 'community' ? 'page' : undefined}><Icon name="community" /><span>Community</span></a>
        <a href="#help" className={`nav-item ${view === 'help' ? 'active' : ''}`} aria-current={view === 'help' ? 'page' : undefined}><Icon name="leaf" /><span>Help & repairs</span></a>
        <a href="#upkeep" className={`nav-item ${view === 'upkeep' ? 'active' : ''}`} aria-current={view === 'upkeep' ? 'page' : undefined}><Icon name="leaf" /><span>Upkeep</span></a>
        <a href="#conduct" className={`nav-item ${view === 'conduct' ? 'active' : ''}`} aria-current={view === 'conduct' ? 'page' : undefined}><Icon name="shield" /><span>Rules & conduct</span></a>
        <a href="#fines" className={`nav-item ${view === 'fines' ? 'active' : ''}`} aria-current={view === 'fines' ? 'page' : undefined}><Icon name="shield" /><span>Fines & appeals</span></a>
        {user.can_read_contacts && <a href="#contacts" className={`nav-item ${view === 'contacts' ? 'active' : ''}`} aria-current={view === 'contacts' ? 'page' : undefined}><Icon name="community" /><span>{user.can_manage_contacts ? 'Contacts' : 'Your preferences'}</span></a>}
        <a href="#statements" className={`nav-item ${view === 'statements' ? 'active' : ''}`} aria-current={view === 'statements' ? 'page' : undefined}><Icon name="document" /><span>Statements</span></a>
        <a href="#messages" className={`nav-item ${view === 'messages' ? 'active' : ''}`} aria-current={view === 'messages' ? 'page' : undefined}><Icon name="community" /><span>Messages</span></a>
        <a href="#security" className={`nav-item ${view === 'security' ? 'active' : ''}`} aria-current={view === 'security' ? 'page' : undefined}><Icon name="shield" /><span>Account security</span></a>
        <a href="#documents" className={`nav-item ${view === 'documents' ? 'active' : ''}`} aria-current={view === 'documents' ? 'page' : undefined}><Icon name="document" /><span>Documents</span></a>
      </nav>
    </div>
    <div className="sidebar-bottom">
      <div className="sidebar-note"><Icon name="leaf" /><p>Less administration.<br /><span>More community.</span></p></div>
      <button className="workspace-profile" onClick={onAbout}><span className="avatar avatar-profile">{user.can_manage_registry ? 'RO' : 'ME'}</span><span>{user.name}<small>{user.can_manage_registry ? 'Registry officer' : user.can_read_registry ? 'Community access' : 'Resident access'}</small></span><Icon name="chevron" /></button>
    </div>
  </aside>
}

function WingArt({ code }: { code: string }) {
  const heights = code === 'A' ? [62, 92, 74] : code === 'B' ? [86, 66, 98] : [72, 101, 58]
  return <svg aria-hidden="true" className="wing-art" viewBox="0 0 150 115" fill="none">
    <path d="M10 108h130" stroke="currentColor" opacity=".2" />
    {heights.map((h, i) => <g key={i}><rect x={20 + i * 39} y={108 - h} width="32" height={h} rx="2" fill="currentColor" opacity={i === 1 ? '.25' : '.13'} />{Array.from({ length: Math.floor(h / 17) - 1 }, (_, j) => <g key={j}><rect x={27 + i * 39} y={116 - h + j * 17} width="5" height="8" rx="2.5" fill="currentColor" opacity=".5" /><rect x={39 + i * 39} y={116 - h + j * 17} width="5" height="8" rx="2.5" fill="currentColor" opacity=".5" /></g>)}</g>)}
    <path d="M12 108V88m0 8-7-7m7 4 7-10" stroke="currentColor" strokeWidth="2" /><circle cx="12" cy="84" r="10" fill="currentColor" opacity=".2" />
  </svg>
}

function WingCard({ wing, onClick, selected = false }: { wing: Building; onClick: () => void; selected?: boolean }) {
  return <button onClick={onClick} className={`wing-card wing-${wing.code.toLowerCase()} ${selected ? 'selected-wing' : ''}`} aria-label={`Explore ${wing.name}, ${wing.flats} homes`} aria-pressed={selected}>
    <span className="wing-card-top"><span className="wing-label">{wing.name}</span><span className="circle-arrow"><Icon name={selected ? 'check' : 'arrow'} /></span></span>
    <span className="wing-card-middle"><span><strong>{wing.flats}</strong><span className="wing-homes">homes</span></span><WingArt code={wing.code} /></span>
    <span className="wing-card-bottom"><span><i className="dot dot-green" />{wing.owner_occupied + wing.rented} occupied</span><span>{wing.vacant} vacant</span></span>
    <span className="wing-people"><span>{wing.owners} owners</span><span>{wing.tenants} tenants</span></span>
  </button>
}

function CommunityCounts({ summary }: { summary: Summary | null }) {
  const metrics: { label: string; value: number | undefined; hint: string; icon: IconName }[] = [
    { label: 'Homes', value: summary?.counts.flats, hint: `Across ${summary?.counts.buildings ?? '—'} wings`, icon: 'homes' },
    { label: 'Occupied', value: summary?.community.occupied, hint: 'Homes with occupants', icon: 'leaf' },
    { label: 'Vacant', value: summary?.community.vacant, hint: 'Unoccupied homes', icon: 'homes' },
    { label: 'Owners', value: summary?.community.owners, hint: 'Distinct active people', icon: 'community' },
    { label: 'Tenants', value: summary?.community.tenants, hint: 'Distinct active people', icon: 'community' },
  ]
  return <><section aria-label="Community at a glance" className="stats-strip registry-stats">{metrics.map(metric => <div className="stat-item" key={metric.label}><span className="stat-label">{metric.label}<Icon name={metric.icon} /></span><strong>{metric.value ?? '—'}</strong><small>{metric.hint}</small></div>)}</section><div className="occupancy-caption"><span><i className="dot dot-green" />{summary?.community.owner_occupied ?? '—'} owner-occupied homes</span><span>{summary?.community.rented ?? '—'} rented homes</span><span>People are counted once per role; joint owners are included.</span></div></>
}

function Registry(props: { summary: Summary | null; user: User; refresh: number; initialWing: string; onOpen: (id: string) => void }) {
 const permitted=canReadChecklists(props.user),[panel,setPanel]=useState(()=>new URLSearchParams(window.location.hash.split('?')[1]??'').get('panel')==='checklists')
 useFragmentSync(()=>{const change=()=>setPanel(new URLSearchParams(window.location.hash.split('?')[1]??'').get('panel')==='checklists');change()},[])
 const choose=(checklists:boolean)=>{setPanel(checklists);window.location.hash=checklists?'homes?panel=checklists':'homes'}
 return <>{permitted&&<nav className="homes-views" aria-label="Home views"><button type="button" aria-current={!panel?'page':undefined} onClick={()=>choose(false)}>{props.user.can_read_registry?'Homes & people':'Your homes'}</button><button type="button" aria-current={panel?'page':undefined} onClick={()=>choose(true)}>Move & contact reviews<Icon name="arrow"/></button></nav>}{permitted&&panel?<MoveChecklists key={props.user.scope_key} user={props.user}/>:<RegistryRegister {...props}/>}</>
}

function RegistryRegister({ summary, user, refresh, initialWing, onOpen }: { summary: Summary | null; user: User; refresh: number; initialWing: string; onOpen: (id: string) => void }) {
  const [query, setQuery] = useState('')
  const [building, setBuilding] = useState(initialWing)
  const [status, setStatus] = useState('')
  const [page, setPage] = useState(1)
  const [data, setData] = useState<FlatPage | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [retry, setRetry] = useState(0)

  useEffect(() => {
    const controller = new AbortController()
    setLoading(true)
    setError('')
    const timer = window.setTimeout(() => {
      const params = new URLSearchParams({ q: query, building, status, page: String(page), page_size: '12' })
      request<FlatPage>(`/api/flats?${params}`, controller.signal).then(result => { setData(result); setLoading(false) }).catch((err: Error) => {
        if (controller.signal.aborted) return
        setError(err.message); setLoading(false)
      })
    }, query ? 200 : 0)
    return () => { clearTimeout(timer); controller.abort() }
  }, [query, building, status, page, retry, refresh])

  const clear = () => { setQuery(''); setBuilding(''); setStatus(''); setPage(1) }
  const chooseWing = (code: string) => { setBuilding(old => old === code ? '' : code); setPage(1) }
  return <div className="page-enter">
    <div className="registry-heading"><div><span className="eyebrow">{user.can_read_registry ? 'OUR NEIGHBOURHOOD' : 'YOUR CORNER OF THE COMMUNITY'}</span><h1>{user.can_read_registry ? <>Every home has <em>a story.</em></> : <>A place to call <em>yours.</em></>}</h1><p>{user.can_read_registry ? 'Find a home. Meet the people. Keep the details together.' : 'Your active homes and the people who share them.'}</p></div>{summary && <span className="registry-count"><strong>{summary.counts.flats}</strong><span>homes,<br />one community</span></span>}</div>
    {user.can_read_registry && <CommunityCounts summary={summary} />}
    <div className="wing-grid registry-wings">{summary?.buildings.map(wing => <WingCard key={wing.id} wing={wing} onClick={() => chooseWing(wing.code)} selected={building === wing.code} />)}</div>
    <section className="registry-panel" aria-labelledby="registry-title">
      <div className="registry-toolbar"><div><h2 id="registry-title">{user.can_read_registry ? 'Homes & people' : 'Your homes'}</h2><p>{user.can_manage_registry ? 'Open a home to manage its occupancy, people and history.' : user.can_read_registry ? 'A view of the community. Registry changes are made by the officer.' : 'Access follows your current relationships with each home.'}</p></div><span className="result-count" role="status">{loading ? 'Finding homes…' : error ? 'Connection unavailable' : `${data?.total ?? 0} homes found`}</span></div>
      <div className="filter-row">
        <label className="search-control"><span className="sr-only">Search homes or people</span><Icon name="search" /><input type="search" value={query} onChange={e => { setQuery(e.target.value); setPage(1) }} placeholder="Search a home or a person…" autoComplete="off" /></label>
        {summary && <FilterSelect label="Filter by wing" value={building} onChange={value => { setBuilding(value); setPage(1) }} options={[{ value: '', label: 'All wings' }, ...summary.buildings.map(wing => ({ value: wing.code, label: wing.name }))]} />}
        <FilterSelect label="Filter by occupancy" value={status} onChange={value => { setStatus(value); setPage(1) }} options={[{ value: '', label: 'All occupancy' }, ...Object.entries(statuses).map(([value, label]) => ({ value, label }))]} />
        <button className="clear-button" disabled={!query && !building && !status} onClick={clear}>Clear<Icon name="close" /></button>
      </div>
      <div aria-busy={loading} className="registry-results">
        {error ? <div className="empty-state" role="alert"><Icon name="globe" /><h3>Let’s try that again.</h3><p>{error}</p><button className="button button-dark" onClick={() => setRetry(n => n + 1)}>Try again<Icon name="refresh" /></button></div> : loading ? <div className="home-grid">{Array.from({length:6}, (_, i) => <div key={i} className="home-skeleton skeleton" aria-hidden="true" />)}</div> : data?.items.length === 0 ? <div className="empty-state"><Icon name="search" /><h3>No homes found.</h3><p>Try another name, home number or filter.</p><button className="button button-dark" onClick={clear}>Clear filters<Icon name="close" /></button></div> : <div className="home-grid">{data?.items.map(flat => <HomeCard key={flat.id} flat={flat} onClick={() => onOpen(flat.id)} />)}</div>}
      </div>
      {data && !error && !loading && data.total > 0 && <div className="pagination"><span>Showing {(data.page - 1) * data.page_size + 1}–{Math.min(data.page * data.page_size, data.total)} of {data.total} homes</span><div><button onClick={() => setPage(p => p - 1)} disabled={page === 1} aria-label="Previous page"><Icon name="left" /></button><span>Page {data.page} of {Math.ceil(data.total / data.page_size)}</span><button onClick={() => setPage(p => p + 1)} disabled={page * data.page_size >= data.total} aria-label="Next page"><Icon name="chevron" /></button></div></div>}
    </section>
  </div>
}

function HomeCard({ flat, onClick }: { flat: Flat; onClick: () => void }) {
  return <button className="home-card" onClick={onClick} aria-label={`View home ${flat.building_code}-${flat.number}`}>
    <span className="home-card-top"><span className={`home-wing home-wing-${flat.building_code.toLowerCase()}`}>{flat.building_code}</span><span className={`status status-${flat.status.toLowerCase()}`}><i />{statuses[flat.status]}</span></span>
    <span className="home-number">{flat.number}<span>Floor {flat.floor.toString().padStart(2, '0')}</span></span>
    <span className="home-card-bottom"><span><span className="home-contact-label">PRIMARY CONTACT</span>{flat.primary_contact || 'Not assigned'}</span><Icon name="arrow" /></span>
  </button>
}

function DetailDialog({ id, about, user, onSaved, onClose }: { id: string | null; about: boolean; user: User; onSaved: () => void; onClose: () => void }) {
  const [detail, setDetail] = useState<FlatDetail | null>(null)
  const [error, setError] = useState('')
  const [retry, setRetry] = useState(0)
  const [tab, setTab] = useState<'people' | 'manage' | 'history'>('people')
  const [success, setSuccess] = useState('')
  useEffect(() => {
    const controller = new AbortController()
    if (id) {
      setDetail(null); setError('')
      request<FlatDetail>(`/api/flats/${encodeURIComponent(id)}`, controller.signal).then(setDetail).catch((err: Error) => { if (!controller.signal.aborted) setError(err.message) })
    }
    return () => controller.abort()
  }, [id, retry])
  return <PortalDialog titleId="detail-title" closeLabel="Close details" onClose={onClose}>
    {about ? <div className="dialog-scroll about-content"><span className="about-mark"><Icon name="leaf" /></span><span className="eyebrow">WELCOME TO SOCIETY OS</span><h2 id="detail-title">A little less admin.<br /><em>A lot more living.</em></h2><p>This is a working local preview with fictional homes and people. Explore the neighbourhood, find a home and open its details.</p><p>Manual entries and receipts keep your supplied records together. Approved notices and service requests keep the community informed. The document library keeps approved versions and private originals together. Payments happen outside the portal.</p><button className="button button-dark" onClick={onClose}>Make yourself at home<Icon name="arrow" /></button></div> : error ? <div className="dialog-scroll empty-state" role="alert"><h2 id="detail-title">Home unavailable</h2><p>{error}</p><button className="button button-dark" onClick={() => setRetry(n => n + 1)}>Try again<Icon name="refresh" /></button></div> : !detail ? <div className="dialog-scroll detail-loading" role="status"><h2 id="detail-title">Opening this home…</h2><div className="skeleton home-skeleton" /></div> : <>
      <div className="dialog-heading"><div className={`detail-banner detail-banner-${detail.building_code.toLowerCase()}`}><span className="eyebrow">WING {detail.building_code} · FLOOR {detail.floor.toString().padStart(2, '0')}</span><h2 id="detail-title">Home <em>{detail.number}</em></h2><span className={`status status-${detail.status.toLowerCase()}`}><i />{statuses[detail.status]}</span><WingArt code={detail.building_code} /></div>
      {user.can_manage_registry && <div className="detail-tabs" aria-label="Home details">{(['people', 'manage', 'history'] as const).map(value => <button key={value} aria-pressed={tab === value} onClick={() => { setTab(value); setSuccess('') }}>{value === 'people' ? 'People' : value === 'manage' ? 'Manage home' : 'History'}</button>)}</div>}
      </div><div className="dialog-scroll detail-body">{success && <p className="form-success" role="status">{success}</p>}{tab === 'manage' && user.can_manage_registry ? <RegistryEditor key={detail.version} detail={detail} onReload={() => { setSuccess(''); setRetry(n => n + 1) }} onSaved={message => { setSuccess(message); setRetry(n => n + 1); onSaved() }} /> : tab === 'history' && user.can_manage_registry ? <RegistryHistory id={detail.id} version={detail.version} /> : <><span className="eyebrow">THE PEOPLE BEHIND THE DOOR</span><h3>People & relationships</h3><p className="detail-intro">{user.can_read_registry ? 'Current and past relationships for this home.' : 'Current relationships for your home.'}</p><div className="member-list">{detail.members.map((member, i) => <div key={member.membership_id} className="member-row"><span className={`avatar member-avatar member-tone-${i % 3}`}>{member.name.split(' ').map(word => word[0]).slice(-2).join('')}</span><div><strong>{member.name}</strong><span>{relationshipLabel(member.relationship)}{!member.active && ' · Former member'}{member.active && member.primary_contact && ' · Primary contact'}</span></div><span className="member-date">{member.end_date ? `Ended ${member.end_date}` : `Since ${member.start_date}`}</span></div>)}</div><div className="detail-footnote"><Icon name="leaf" /><span>This preview uses fictional people and relationships.</span></div></>}
      </div>
    </>}
  </PortalDialog>
}

function Workspace({ user, onLogout, onUser }: { user: User; onLogout: () => void; onUser: (user: User) => void }) {
  const [view, setView] = useState<View>(currentView)
  const [summary, setSummary] = useState<Summary | null>(null)
  const [error, setError] = useState('')
  const [retry, setRetry] = useState(0)
  const [initialWing, setInitialWing] = useState('')
  const [selected, setSelected] = useState<string | null>(()=>currentView()==='homes'?new URLSearchParams(window.location.hash.split('?')[1]??'').get('home'):null)
  const [about, setAbout] = useState(false)
  const [refresh, setRefresh] = useState(0)
  useSocietyTools(user, setSelected)
  useFragmentSync(()=>{
    const handler = () => { const next = currentView(); setView(next); if (next !== 'homes') setInitialWing(''); window.scrollTo({ top: 0, behavior: 'instant' }); setSelected(next==='homes'?new URLSearchParams(window.location.hash.split('?')[1]??'').get('home'):null); setAbout(false) }
    handler()},[])
  useEffect(() => {
    if (!user.can_read_registry || view !== 'homes') { setSummary(null); setError(''); return }
    const controller = new AbortController()
    setError('')
    Promise.all([request<Summary>('/api/registry/summary', controller.signal), request<{status:string}>('/ready', controller.signal)]).then(([result]) => setSummary(result)).catch((err: Error) => { if (!controller.signal.aborted) { setError(err.message); setSummary(null) } })
    return () => controller.abort()
  }, [retry, refresh, user.can_read_registry, view])
  const date = new Intl.DateTimeFormat('en-IN', { day: 'numeric', month: 'short', year: 'numeric', timeZone: 'Asia/Kolkata' }).format(new Date())
  return <div className="app-shell">
    <a className="skip-link" href="#main-content" onClick={event => { event.preventDefault(); document.getElementById('main-content')?.focus() }}>Skip to content</a>
    <Sidebar view={view} user={user} onAbout={() => setAbout(true)} />
    <div className="main-shell">
      <header className="topbar"><span className="breadcrumb">Your workspace<span>/</span><strong>{view === 'statements' ? 'Financial statements' : view === 'messages' ? 'Messages' : view === 'contacts' ? 'Contacts & preferences' : view === 'fines' ? 'Fines & appeals' : view === 'conduct' ? 'Rules & conduct' : view === 'collections' ? 'Collections' : view === 'upkeep' ? 'Upkeep' : view === 'maintenance' ? 'Maintenance' : view === 'documents' ? 'Documents' : view === 'help' ? 'Help & repairs' : view === 'reviews' ? user.can_review_requests ? 'Requests & approvals' : 'Your requests' : view === 'community' ? 'Community' : view === 'entries' ? 'Entries' : view === 'receipts' ? 'Receipts' : view === 'security' ? 'Account security' : view === 'access' && user.can_manage_accounts ? 'Access & invitations' : view === 'overview' ? 'Overview' : user.can_read_registry ? 'Homes & people' : 'Your homes'}</strong></span><div className="topbar-right"><span className="today">{date}</span><span className="preview-pill"><i />Local preview</span><button className="signout-button" onClick={onLogout}>Sign out</button></div></header>
      <div className="preview-banner"><span><Icon name="spark" />A first look at your community workspace.</span><span>Fictional data <i /> Live local registry</span></div>
      <main id="main-content" tabIndex={-1} className="main-content">
        {view === 'homes' && error && <div className="connection-error" role="alert"><p>{error}</p><button onClick={() => setRetry(n => n + 1)}>Reconnect<Icon name="refresh" /></button></div>}
        <ScreenBoundary key={view}><Suspense fallback={<ScreenLoading/>}>{view === 'statements' ? <Statements user={user} /> : view === 'messages' ? <Messages user={user} /> : view === 'contacts' ? <Contacts user={user} /> : view === 'fines' ? <Fines user={user} /> : view === 'conduct' ? <Incidents user={user} /> : view === 'collections' ? <Collections user={user} /> : view === 'upkeep' ? <Upkeep user={user} /> : view === 'maintenance' ? <Maintenance user={user} /> : view === 'documents' ? <Documents user={user} /> : view === 'help' ? <Complaints user={user} /> : view === 'community' ? <Community user={user} /> : view === 'reviews' ? <Reviews user={user} /> : view === 'entries' || view === 'receipts' ? <Records key={view} user={user} receipts={view === 'receipts'} /> : view === 'security' ? <AccountSecurity user={user} onUser={onUser} onLogout={onLogout} /> : view === 'access' && user.can_manage_accounts ? <Access user={user} /> : view === 'overview' ? <Overview user={user} /> : <Registry summary={summary} user={user} refresh={refresh} initialWing={initialWing} onOpen={setSelected} />}</Suspense></ScreenBoundary>
        <footer className="page-footer"><span><span className="footer-wordmark">society.</span> Made for everyday life.</span><button onClick={() => setAbout(true)}>About this preview<Icon name="arrow" /></button></footer>
      </main>
    </div>
    {(selected || about) && <DetailDialog key={selected ?? 'about'} id={selected} about={about} user={user} onSaved={() => setRefresh(n => n + 1)} onClose={() => { setSelected(null); setAbout(false); if(currentView()==='homes'){const params=new URLSearchParams(window.location.hash.split('?')[1]??'');if(params.has('home')){params.delete('home');window.history.replaceState(null,'','#homes'+(params.toString()?'?'+params:''))}} }} />}
  </div>
}

export default function App() {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)
  const [message, setMessage] = useState('')
  const [linkToken, setLinkToken] = useState(initialLink)
  const screen = loading ? 'loading' : linkToken ? 'link' : !user ? 'login' : user.mfa_pending ? 'verification' : 'workspace'
  useLayoutEffect(() => { window.scrollTo({ top: 0, behavior: 'instant' }) }, [screen])
  const identity = useRef<User | null>(null)
  const storeUser = (value: User | null) => {
    const previous = identity.current
    if (previous && value && userAccessScope(previous) !== userAccessScope(value)) {
      // Detail links belong to the scope under which they were opened. Clear
      // them before the new workspace mounts, so they cannot reopen old dialogs.
      window.history.replaceState(null, '', '#' + currentView())
    }
    identity.current = value; setSession(value); setUser(value)
  }
  const acceptUser = (value: User) => { storeUser(value); setMessage('') }
  useEffect(() => { const handler = () => { const token = takeLink(); if (token) setLinkToken(token) }; window.addEventListener('hashchange', handler); return () => window.removeEventListener('hashchange', handler) }, [])
  useEffect(() => {
    const controller = new AbortController()
    request<User>('/api/auth/me', controller.signal).then(value => { storeUser(value) }).catch((err: Error) => { if (!controller.signal.aborted && !(err instanceof APIError && err.status === 401)) setMessage(err.message) }).finally(() => { if (!controller.signal.aborted) setLoading(false) })
    const expire = () => { storeUser(null); setMessage('Your session ended. Sign in again to continue.') }
    window.addEventListener('session-expired', expire)
    return () => { controller.abort(); window.removeEventListener('session-expired', expire) }
  }, [])
  useEffect(() => {
    if (!user || user.mfa_pending) return
    const controller = new AbortController()
    const check = () => request<User>('/api/auth/me', controller.signal).then(value => { storeUser(value) }).catch((err: Error) => {
      if (!controller.signal.aborted && err instanceof APIError && err.status === 401) { storeUser(null); setMessage('Your session ended. Sign in again to continue.') }
    })
    const timer = window.setInterval(check, 60000)
    const onVisible = () => { if (document.visibilityState === 'visible') void check() }
    document.addEventListener('visibilitychange', onVisible)
    window.addEventListener('session-recheck', check)
    return () => { controller.abort(); clearInterval(timer); document.removeEventListener('visibilitychange', onVisible); window.removeEventListener('session-recheck', check) }
  }, [user?.id, user?.mfa_pending])
  const logout = async () => {
    try { await mutate('/api/auth/logout', 'POST', {}) }
    catch (err) { if (!(err instanceof APIError && err.status === 401)) { setMessage((err as Error).message); return } }
    storeUser(null); setMessage(''); window.location.hash = 'overview'
  }
  if (loading) return <main className="session-loading" role="status"><span className="footer-wordmark">society.</span><p>Opening your workspace…</p></main>
  if (linkToken) return <AccountLink key={linkToken} token={linkToken} onDone={() => { setLinkToken(''); if (user) void logout() }} />
  if (user?.mfa_pending) return <MFAGate key={user.id} user={user} onVerified={acceptUser} onLogout={() => void logout()} />
  return user ? <>{message && <div className="session-message" role="alert">{message}</div>}<Workspace key={userAccessScope(user)} user={user} onUser={acceptUser} onLogout={() => { setMessage(''); void logout() }} /></> : <Login message={message} onLogin={acceptUser} />
}
