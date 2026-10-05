import { useEffect, useRef, useState } from 'react'
import { request } from '../api'
import type { User } from '../api'
import { Icon } from './Icon'
import type { IconName } from './Icon'

export type OverviewSection = 'finance' | 'reviews' | 'service' | 'notices' | 'documents' | 'maintenance'
type Item = { id: string; title: string; home: string; kind: string; state: string; priority: string; date: string; at: number; amount_paise: number }
type Data = { section: OverviewSection; as_of: number; day: string; period_start: string; counts: Record<string, number>; items: Item[] }
type Source = { section: OverviewSection; data: Data | null; busy: boolean; error: string; retry: () => void }
const money = (paise: number) => new Intl.NumberFormat('en-IN', { style: 'currency', currency: 'INR', minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(paise / 100)
const day = (date: string) => new Intl.DateTimeFormat('en-IN', { day: 'numeric', month: 'short' }).format(new Date(date + 'T12:00:00Z'))
const when = (at: number) => new Intl.DateTimeFormat('en-IN', { day: 'numeric', month: 'short', timeZone: 'Asia/Kolkata' }).format(new Date(at * 1000))
const labels: Record<OverviewSection, string> = { finance: 'Financial records', reviews: 'Requests', service: 'Service requests', notices: 'Notices', documents: 'Documents', maintenance: 'Maintenance' }
const destinations: Record<OverviewSection, string> = { finance: '#entries', reviews: '#reviews', service: '#help', notices: '#community', documents: '#documents', maintenance: '#maintenance' }
function useSource(section: OverviewSection, enabled: boolean, refresh: number, actor: string): Source {
  const [data, setData] = useState<Data | null>(null)
  const [busy, setBusy] = useState(enabled)
  const [error, setError] = useState('')
  const [retry, setRetry] = useState(0)
  useEffect(() => {
    setData(null); setError(''); setBusy(enabled)
    if (!enabled) return
    const controller = new AbortController()
    request<Data>('/api/overview/' + section, controller.signal).then(value => { setData(value); setBusy(false) }).catch((err: Error) => { if (!controller.signal.aborted) { setError(err.message); setBusy(false) } })
    return () => controller.abort()
  }, [section, enabled, refresh, retry, actor])
  return { section, data: busy || error ? null : data, busy, error, retry: () => setRetry(value => value + 1) }
}
function CalendarArt() {
  const today = new Intl.DateTimeFormat('en-IN', { day: 'numeric', timeZone: 'Asia/Kolkata' }).format(new Date())
  return <svg className="overview-calendar" viewBox="0 0 180 140" fill="none" aria-hidden="true"><ellipse cx="94" cy="116" rx="69" ry="11" fill="#dce0d2" /><g transform="rotate(7 90 70)"><rect x="41" y="20" width="100" height="99" rx="10" fill="#fdfcf5" stroke="#bbc4a9" /><path d="M42 49h98" stroke="#d3dac5" /><path d="M66 14v17M116 14v17" stroke="#657a55" strokeWidth="4" strokeLinecap="round" /><text x="91" y="91" textAnchor="middle" fill="#294b3e" fontSize="42" fontFamily="Instrument Serif, Georgia, serif">{today}</text><path d="M68 104h44" stroke="#c7cdb9" strokeLinecap="round" /></g><circle cx="145" cy="99" r="22" fill="#dfe9a7" /><path d="m135 99 7 7 13-15" stroke="#294b3e" strokeWidth="2" strokeLinecap="round" /></svg>
}
function SourceState({ source }: { source: Source }) {
  if (source.busy) return <div className="overview-source-state" role="status"><span className="overview-small-skeleton skeleton" aria-hidden="true" />Updating {labels[source.section].toLowerCase()}…</div>
  if (!source.error) return null
  return <div className="overview-source-state overview-source-error" role="alert"><div><strong>{labels[source.section]} unavailable</strong><p>{source.error}</p></div><button className="text-link" onClick={source.retry}>Retry {labels[source.section].toLowerCase()}<Icon name="refresh" /></button></div>
}
function Metric({ source, label, value, hint, icon, tone }: { source: Source; label: string; value: number | undefined; hint: string; icon: IconName; tone: string }) {
  return <section className={'overview-metric ' + tone} aria-label={label}><div><span>{label}</span><Icon name={icon} /></div><strong>{value ?? '—'}</strong><small>{source.busy ? 'Updating…' : source.error ? 'Temporarily unavailable' : hint}</small><a href={destinations[source.section]} aria-label={'Open ' + label.toLowerCase()} className="overview-metric-link"><span className="sr-only">Open {label.toLowerCase()}</span><Icon name="arrow" /></a></section>
}
function link(section: OverviewSection, item: Item) {
  return section === 'maintenance' ? '#maintenance?period=' + item.id : section === 'finance' ? '#entries?entry=' + item.id : section === 'reviews' ? '#reviews?request=' + item.id : section === 'service' ? '#help?case=' + item.id : section === 'documents' ? '#documents?file=' + item.id : '#community?notice=' + item.id
}
function action(section: OverviewSection, item: Item) {
  if (section === 'maintenance') return item.state === 'PENDING' ? 'Review a maintenance period' : 'Maintenance past its due date'
  if (section === 'finance') return item.state === 'DRAFT' ? 'Confirm supplied entry' : item.state === 'FAILED' ? 'Receipt PDF needs attention' : 'Receipt PDF is preparing'
  if (section === 'reviews') return item.state === 'CHANGES_REQUESTED' ? 'Update your submission' : 'A decision from you'
  if (section === 'documents') return ({ UPLOAD_ATTENTION: 'Your upload needs attention', READY_REVIEW: 'Review this document', EXPIRED: 'Document expiry passed', EXPIRING: 'Document expiry approaching' })[item.state] ?? 'Open document'
  return item.state === 'RESOLVED' ? 'Check the resolution' : item.priority === 'URGENT' ? 'Urgent service request' : 'Active service request'
}
function rank(section: OverviewSection, item: Item) {
  if (section === 'maintenance') return item.state === 'OVERDUE' ? 1 : 2
  if (section === 'service' && item.priority === 'URGENT') return 0
  if (item.state === 'FAILED' || item.state === 'CHANGES_REQUESTED' || item.state === 'UPLOAD_ATTENTION') return 1
  if (section === 'reviews' || item.state === 'EXPIRED' || item.priority === 'HIGH') return 2
  return 3
}

export function Overview({ user }: { user: User }) {
  const root = useRef<HTMLDivElement>(null)
  const refreshButton = useRef<HTMLButtonElement>(null)
  const refreshFocus = useRef(false)
  const [refresh, setRefresh] = useState(0)
  const reviews = useSource('reviews', true, refresh, user.id)
  const service = useSource('service', true, refresh, user.id)
  const documents = useSource('documents', true, refresh, user.id)
  const notices = useSource('notices', true, refresh, user.id)
  const finance = useSource('finance', user.can_read_records, refresh, user.id)
  const maintenance = useSource('maintenance', user.can_read_records, refresh, user.id)
  const sources = [reviews, service, documents, ...(user.can_read_records ? [finance, maintenance] : [])]
  const allSources = [...sources, notices]
  const busy = allSources.some(source => source.busy)
  const complete = allSources.every(source => source.data)
  useEffect(() => {
    if (!busy && refreshFocus.current) {
      refreshFocus.current = false
      if (document.activeElement === document.body || document.activeElement === refreshButton.current) refreshButton.current?.focus({ preventScroll: true })
    }
  }, [busy])
  useEffect(() => {
    if (busy) return
    const state = window.history.state as { overviewActor?: string; overviewFocus?: string } | null
    if (state?.overviewActor !== user.id || !state.overviewFocus) return
    const target = [...root.current?.querySelectorAll<HTMLAnchorElement>('a[href]') ?? []].find(anchor => anchor.getAttribute('href') === state.overviewFocus)
    const remaining = { ...state }; delete remaining.overviewActor; delete remaining.overviewFocus
    window.history.replaceState(remaining, '', window.location.href)
    if (target) { target.focus({ preventScroll: true }); target.scrollIntoView({ block: 'center' }) }
    else document.getElementById('main-content')?.focus({ preventScroll: true })
  }, [busy, user.id])
  const rows = sources.flatMap(source => (source.data?.items ?? []).map(item => ({ item, section: source.section }))).sort((a, b) => rank(a.section, a.item) - rank(b.section, b.item) || a.item.at - b.item.at || a.item.id.localeCompare(b.item.id))
  const r = reviews.data?.counts; const c = service.data?.counts; const d = documents.data?.counts; const f = finance.data?.counts; const m = maintenance.data?.counts
  const total = (r ? r.needs_your_decision + r.changes_requested : 0) + (c ? c.active + c.needs_closure : 0) + (d ? d.ready_review + d.upload_attention + d.expiring + d.expired : 0) + (f ? f.awaiting_confirmation + f.receipt_pending + f.receipt_failed : 0) + (m ? m.pending_review + m.overdue_periods : 0)
  const updated = complete ? Math.min(...allSources.map(source => source.data!.as_of)) : 0
  return <div ref={root} className="page-enter overview-page" onClick={event => {
    const anchor = (event.target as Element).closest('a[href]')
    if (anchor) window.history.replaceState({ ...window.history.state, overviewActor: user.id, overviewFocus: anchor.getAttribute('href') }, '', window.location.href)
  }}>
    <section className="overview-heading" aria-labelledby="overview-title"><div><span className="eyebrow">THE DAY AHEAD</span><h1 id="overview-title">Good things, <em>in order.</em></h1><p>{user.can_read_registry ? 'The decisions, details and everyday things that need attention.' : 'Your requests, community updates and the details that need you.'}</p><div className="overview-freshness"><span role="status">{busy ? 'Updating your overview…' : complete ? 'Checked at ' + new Intl.DateTimeFormat('en-IN', { hour: 'numeric', minute: '2-digit', second: '2-digit', timeZone: 'Asia/Kolkata' }).format(new Date(updated * 1000)) : 'Some sections need a retry'}</span><button ref={refreshButton} className="text-link" disabled={busy} onClick={event => { refreshFocus.current = document.activeElement === event.currentTarget; setRefresh(value => value + 1) }} aria-label="Refresh overview">Refresh<Icon name="refresh" /></button></div></div><CalendarArt /></section>
    <section className="overview-metrics" aria-label="What is changing">
      <Metric source={reviews} label={user.can_review_requests ? 'For your decision' : 'Changes to make'} value={r && (user.can_review_requests ? r.needs_your_decision : r.changes_requested)} hint={r ? r.awaiting_others + ' of your requests await a reviewer' : ''} icon="check" tone="overview-sage" />
      <Metric source={service} label={user.can_handle_complaints ? 'Active service requests' : 'Your active service requests'} value={c?.active} hint={c ? c.urgent ? c.urgent + ' urgent · ' + (user.can_handle_complaints ? c.unassigned + ' unassigned' : c.needs_closure + ' resolutions to check') : user.can_handle_complaints ? c.unassigned + ' unassigned' : c.needs_closure + ' resolutions to check' : ''} icon="leaf" tone="overview-clay" />
      <Metric source={documents} label="Document attention" value={d && d.ready_review + d.upload_attention + d.expiring + d.expired} hint={d ? d.ready_review + ' ready for review · ' + (d.expiring + d.expired) + ' deadlines' : ''} icon="document" tone="overview-lilac" />
      <Metric source={notices} label="Recent notices" value={notices.data?.counts.recent} hint={notices.data ? day(notices.data.period_start) + ' – ' + day(notices.data.day) + ' · approved for you' : ''} icon="community" tone="overview-cream" />
    </section>
    <div className={'overview-main-grid ' + (!user.can_read_records ? 'overview-without-finance' : '')}>
      <section className="overview-panel overview-attention" aria-labelledby="attention-title"><div className="overview-panel-heading"><div><span className="eyebrow">A LITTLE FOCUS GOES A LONG WAY</span><h2 id="attention-title">Needs attention</h2></div><span className="overview-count">{sources.every(source => source.data) ? total + ' items' : 'Partial view'}</span></div>
        {sources.filter(source => source.section !== 'finance' && source.section !== 'maintenance').map(source => <SourceState key={source.section} source={source} />)}
        {rows.length ? <ol className="overview-action-list">{rows.slice(0, 8).map(({ item, section }) => <li key={section + item.id}><a href={link(section, item)} aria-label={action(section, item) + ': ' + item.title}><span className={'overview-action-icon ' + (item.priority === 'URGENT' || item.state === 'FAILED' ? 'overview-action-urgent' : '')}><Icon name={section === 'service' || section === 'maintenance' ? 'leaf' : section === 'finance' ? 'receipt' : section === 'reviews' ? 'check' : 'document'} /></span><span className="overview-action-copy"><small>{action(section, item)}</small><strong>{item.title}</strong><span>{item.home ? 'Home ' + item.home + ' · ' : ''}{section === 'documents' && ['EXPIRED', 'EXPIRING'].includes(item.state) && item.date ? day(item.date) : section === 'maintenance' ? money(item.amount_paise) + ' · Due ' + day(item.date) : section === 'finance' ? money(item.amount_paise) : when(item.at)}</span></span><Icon name="arrow" /></a></li>)}</ol> : sources.every(source => source.data) ? <div className="overview-clear"><span><Icon name="check" /></span><h3>All clear for now.</h3><p>Your visible queues have no open work. Check back after new records or requests are added.</p></div> : null}
        <div className="overview-attention-foot"><span>{rows.length ? 'Showing ' + Math.min(rows.length, 8) + (sources.every(source => source.data) ? ' of ' + total : ' visible') + ' items' : 'Current permissions apply to every item.'}</span><a className="text-link" href="#help" aria-label="Open help & repairs">Help & repairs<Icon name="arrow" /></a></div>
      </section>
      {user.can_read_records && <section className="overview-panel overview-finance" aria-labelledby="overview-finance-title"><div className="overview-panel-heading"><div><span className="eyebrow">THE SUPPLIED RECORDS</span><h2 id="overview-finance-title">Money, accounted for.</h2></div><span className="overview-finance-mark"><Icon name="records" /></span></div><SourceState source={finance} /><div className="overview-finance-balance"><span>Recorded balances to account for</span><strong>{f ? money(f.positive_balance_paise) : '—'}</strong><small>{f ? f.homes_with_balance + (f.homes_with_balance === 1 ? ' home has' : ' homes have') + ' a positive balance' : 'Current permitted homes'}</small></div><div className="overview-finance-lines"><div><span>Credits held by homes</span><strong>{f ? money(f.credit_balance_paise) : '—'}</strong></div><div><span>Net recorded balance</span><strong>{f ? money(f.balance_paise) : '—'}</strong></div><div><span>Money received{finance.data && <small>{day(finance.data.period_start)} – {day(finance.data.day)} · 30 calendar days</small>}</span><strong>{f ? money(f.received_paise) : '—'}</strong></div></div><p className="overview-finance-note">{user.can_read_all_records ? 'Across the community.' : 'For your financially permitted homes.'} Confirmed supplied entries; reversals excluded. These records do not establish overdue bills.</p><div className="overview-finance-links"><a className="text-link" href="#entries">Open entries<Icon name="arrow" /></a><a className="text-link" href="#receipts">View receipts<Icon name="arrow" /></a></div></section>}
    </div>
    {user.can_read_records && <section className="overview-panel overview-maintenance" aria-labelledby="overview-maintenance-title"><div className="overview-panel-heading"><div><span className="eyebrow">THE CARE THAT KEEPS US GOING</span><h2 id="overview-maintenance-title">Maintenance, in view.</h2></div><a className="text-link" href="#maintenance">Open maintenance<Icon name="arrow" /></a></div><SourceState source={maintenance} /><div className="overview-maintenance-amounts"><div><span>Outstanding</span><strong>{m?money(m.outstanding_paise):'—'}</strong><small>Published charges less live allocations</small></div><div><span>Past due</span><strong>{m?money(m.overdue_paise):'—'}</strong><small>Based on each supplied due date</small></div><div><span>For your review</span><strong>{m?m.pending_review:'—'}</strong><small>{m?m.awaiting_other_reviewer+' of your periods await another reviewer':'Current treasury permission applies'}</small></div></div><p className="overview-finance-note">{user.can_read_all_records?'Across the community.':'For your financially entitled homes.'} Available credits remain separate until explicitly allocated.</p></section>}
    <section className="overview-panel overview-notices" aria-labelledby="overview-notices-title"><div className="overview-panel-heading"><div><span className="eyebrow">FROM YOUR COMMUNITY</span><h2 id="overview-notices-title">The noticeboard</h2></div><a className="text-link" href="#community">All notices<Icon name="arrow" /></a></div><SourceState source={notices} />{notices.data && (notices.data.items.length ? <div className="overview-notice-grid">{notices.data.items.map(item => <a key={item.id} href={link('notices', item)} aria-label={'Read notice: ' + item.title}><span className="overview-notice-date">{when(item.at)}<Icon name="arrow" /></span><h3>{item.title}</h3><span>Approved community notice</span></a>)}</div> : <div className="overview-notice-empty"><Icon name="community" /><p>No approved notices for your current audience yet.</p></div>)}</section>
    <div className="overview-shortcuts"><a href="#homes" aria-label={user.can_read_registry ? "Open homes & people" : "Open your homes"}><Icon name="homes" />{user.can_read_registry ? 'Homes & people' : 'Your homes'}<Icon name="arrow" /></a><a href="#documents"><Icon name="document" />Document library<Icon name="arrow" /></a><a href="#reviews" aria-label={user.can_review_requests ? "Open requests & approvals" : "Open your requests"}><Icon name="check" />{user.can_review_requests ? 'Requests & approvals' : 'Your requests'}<Icon name="arrow" /></a></div>
  </div>
}
