import { useFragmentSync } from '../navigation'
import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { APIError, mutate, request } from '../api'
import type { User } from '../api'
import { Icon } from './Icon'
import { FilterSelect, FormSelect } from './FilterSelect'
import { PortalDialog } from './PortalDialog'
import './Complaints.css'

type Home = { id: string; label: string }
export type Complaint = { id: string; number: string; flat_id: string; home: string; reported_by: string; reporter: string; category: string; subject: string; description: string; priority: string; status: string; assigned_to: string; assigned_name: string; created_at: number; updated_at: number; version: number }
type Detail = Complaint & { can_participate: boolean; handlers: { id: string; name: string }[]; history_page: number; history_total: number; history_page_size: number; updates: { id: string; action: string; message: string; visibility: string; actor: string; at: number; status: string }[] }
type CasePage = { items: Complaint[]; total: number; page: number; page_size: number; homes: Home[] }
const categories: Record<string, string> = { PLUMBING: 'Plumbing', LIFT: 'Lift', ELECTRICAL: 'Electrical', SECURITY: 'Security', CLEANING: 'Cleaning', WATER: 'Water', PARKING: 'Parking', COMMON_AREA: 'Common areas', OTHER: 'Something else' }
const states: Record<string, string> = { OPEN: 'Open', ACKNOWLEDGED: 'Acknowledged', IN_PROGRESS: 'In progress', WAITING: 'Waiting', RESOLVED: 'Resolved', CLOSED: 'Closed' }
const priorities: Record<string, string> = { NORMAL: 'Normal', HIGH: 'High', URGENT: 'Urgent' }
const actions: Record<string, string> = { CREATED: 'Request reported', COMMENT: 'Conversation update', STATUS: 'Progress update', ASSIGN: 'Handler assignment', PRIORITY: 'Priority update' }
const transitions: Record<string, string[]> = { OPEN: ['ACKNOWLEDGED', 'IN_PROGRESS', 'WAITING'], ACKNOWLEDGED: ['IN_PROGRESS', 'WAITING'], IN_PROGRESS: ['WAITING', 'RESOLVED'], WAITING: ['IN_PROGRESS', 'RESOLVED'], RESOLVED: ['IN_PROGRESS', 'CLOSED'], CLOSED: ['OPEN'] }
const options = (map: Record<string, string>) => Object.entries(map).map(([value, label]) => ({ value, label }))
const date = (at: number) => new Intl.DateTimeFormat('en-IN', { day: 'numeric', month: 'short', year: 'numeric', timeZone: 'Asia/Kolkata' }).format(new Date(at * 1000))
const linkedCase = () => { const id = new URLSearchParams(window.location.hash.split('?')[1] ?? '').get('case') ?? ''; return /^[A-Za-z0-9_-]{43}$/.test(id) ? id : '' }
const feedbackIntoView = (element: HTMLElement | null) => {
  if (!element) return
  const scroll = element.closest<HTMLElement>('.dialog-scroll')
  if (!scroll) { element.scrollIntoView({ block: 'center', behavior: 'instant' }); return }
  const area = scroll.getBoundingClientRect(), target = element.getBoundingClientRect()
  scroll.scrollTo({ top: scroll.scrollTop + target.top - area.top - Math.max(0, (area.height - target.height) / 2), behavior: 'instant' })
}


export function Complaints({ user }: { user: User }) {
  const [data, setData] = useState<CasePage | null>(null)
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState('')
  const [page, setPage] = useState(1)
  const [revision, setRevision] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [creating, setCreating] = useState(false)
  const [selected, setSelected] = useState(linkedCase)
  const feedback = useRef<HTMLDivElement>(null)
  useEffect(() => { if (error) feedbackIntoView(feedback.current) }, [error])
  useFragmentSync(()=>{ const update = () => setSelected(linkedCase()); update()},[])
  useEffect(() => {
    const controller = new AbortController(); setLoading(true); setError('')
    const timer = setTimeout(() => request<CasePage>('/api/complaints?' + new URLSearchParams({ q: query.trim(), status, page: String(page) }), controller.signal).then(value => { setData(value); setLoading(false) }).catch((err: Error) => { if (!controller.signal.aborted) { setError(err.message); setLoading(false) } }), query ? 200 : 0)
    return () => { controller.abort(); clearTimeout(timer) }
  }, [query, status, page, revision])
  const clear = () => { setQuery(''); setStatus(''); setPage(1) }
  const newCase = !!data?.homes.length && !loading && !error
  return <div className="page-enter complaints-page">
    <header className="service-heading"><div><span className="eyebrow">EVERYDAY CARE</span><h1>Complaints &amp; service requests</h1><p>{user.can_handle_complaints ? 'Assign a handler, track progress and follow through to resolution.' : 'Report a problem and follow the conversation with your handling team.'}</p></div><button className="button button-dark" disabled={!newCase} onClick={() => setCreating(true)}>Report an issue<Icon name="plus" /></button></header>
    <section className="registry-panel service-desk" aria-labelledby="service-list-title"><div className="registry-toolbar"><h2 id="service-list-title">{user.can_handle_complaints ? 'The service desk' : 'Your service requests'}</h2><span className="result-count" role="status">{loading ? 'Opening your requests…' : error ? 'Connection unavailable' : `${data?.total ?? 0} cases found`}</span></div>
      <div className="filter-row review-filters"><label className="search-control"><span className="sr-only">Search service requests</span><Icon name="search" /><input type="search" value={query} onChange={event => { setQuery(event.target.value); setPage(1) }} maxLength={100} placeholder="Find a request or its number…" /></label><FilterSelect label="Filter service requests by status" value={status} onChange={value => { setStatus(value); setPage(1) }} options={[{ value: '', label: 'All statuses' }, ...options(states)]} /><button className="clear-button" disabled={!query && !status} onClick={clear}>Clear<Icon name="close" /></button></div>
      <div aria-busy={loading}>{error ? <div className="empty-state" role="alert" ref={feedback}><Icon name="globe" /><h3>Let’s open that again.</h3><p>{error}</p><button className="button button-dark" onClick={() => setRevision(n => n + 1)}>Try again<Icon name="refresh" /></button></div> : loading ? <div className="service-loading" aria-hidden="true">{[0, 1, 2].map(index => <div key={index} className="service-row-skeleton skeleton" />)}</div> : !data?.items.length ? <div className="records-empty"><span className="chapter-icon"><Icon name="leaf" /></span><h3>{query || status ? 'Nothing matches just yet.' : 'A little calm, for now.'}</h3><p>{query || status ? 'Try another word or status.' : newCase ? 'When something needs attention, this is where the conversation begins.' : 'Your own case history stays here. New reports need a current home relationship.'}</p>{query || status ? <button className="text-link" onClick={clear}>Clear filters<Icon name="arrow" /></button> : newCase && <button className="text-link" onClick={() => setCreating(true)}>Report your first issue<Icon name="arrow" /></button>}</div> : <div className="service-list"><div className="service-columns" aria-hidden="true"><span>Request / home</span><span>Status</span><span>Handler / latest update</span></div><ul className="service-rows" aria-label="Service requests">{data.items.map(item => <li key={item.id}><button className="service-row" aria-label={`Open case ${item.subject}`} aria-describedby={`case-context-${item.id} case-status-${item.id} case-progress-${item.id}`} onClick={() => setSelected(item.id)}>
        <span className="service-row-main"><strong>{item.subject}</strong><span className="service-row-context" id={`case-context-${item.id}`}><span>{item.home} · {categories[item.category]}</span><span>{item.number}</span><span className={`case-priority priority-${item.priority.toLowerCase()}`}>{priorities[item.priority]} priority</span></span></span>
        <span className={`case-status case-${item.status.toLowerCase()}`} id={`case-status-${item.id}`}>{states[item.status]}</span>
        <span className="service-row-progress" id={`case-progress-${item.id}`}><strong>{item.assigned_name ? 'With ' + item.assigned_name : ['RESOLVED', 'CLOSED'].includes(item.status) ? 'No handler recorded' : 'Awaiting a handler'}</strong><small>{item.status === 'RESOLVED' ? 'Review resolution · ' + date(item.updated_at) : item.status === 'CLOSED' ? 'View history · ' + date(item.updated_at) : 'Updated ' + date(item.updated_at)}</small></span><Icon name="chevron" />
      </button></li>)}</ul></div>}</div>
      {data && !loading && !error && data.total > 0 && <div className="pagination"><span>Showing {(data.page - 1) * 12 + 1}–{Math.min(data.page * 12, data.total)} of {data.total} cases</span><div><button aria-label="Previous service requests page" onClick={() => setPage(data.page - 1)} disabled={data.page === 1}><Icon name="left" /></button><span>Page {data.page} of {Math.ceil(data.total / 12)}</span><button aria-label="Next service requests page" onClick={() => setPage(data.page + 1)} disabled={data.page * 12 >= data.total}><Icon name="chevron" /></button></div></div>}
    </section><p className="records-footnote"><Icon name="shield" />For urgent safety issues, contact your building’s security or emergency service directly.</p>
    {creating && <NewCase homes={data?.homes ?? []} onClose={() => setCreating(false)} onSaved={id => { setCreating(false); setSelected(id); setRevision(n => n + 1) }} />}
    {selected && <CaseDetail key={selected} id={selected} user={user} onClose={() => setSelected('')} onSaved={() => setRevision(n => n + 1)} />}
  </div>
}

function NewCase({ homes, onClose, onSaved }: { homes: Home[]; onClose: () => void; onSaved: (id: string) => void }) {
  const [home, setHome] = useState('')
  const [category, setCategory] = useState('')
  const [subject, setSubject] = useState('')
  const [description, setDescription] = useState('')
  const [priority, setPriority] = useState('NORMAL')
  const [busy, setBusy] = useState(false)
  const [locked, setLocked] = useState(false)
  const [error, setError] = useState('')
  const pending = useRef<Record<string, unknown> | null>(null)
  const feedback = useRef<HTMLParagraphElement>(null)
  useEffect(() => { if (error) feedbackIntoView(feedback.current) }, [error])
  const submit = async (event: FormEvent) => {
    event.preventDefault(); if (busy) return; setBusy(true); setError('')
    pending.current ??= { operation_key: crypto.randomUUID(), flat_id: home, category, subject: subject.trim(), description: description.trim(), priority }
    try { const result = await mutate<{ id: string }>('/api/complaints', 'POST', pending.current); onSaved(result.id) }
    catch (err) { setError((err as Error).message); if (err instanceof APIError && [400, 403, 404].includes(err.status)) { pending.current = null; setLocked(false) } else setLocked(true) }
    finally { setBusy(false) }
  }
  return <PortalDialog titleId="new-case-title" closeLabel="Close service request form" busy={busy} onClose={onClose}><div className="dialog-heading reviews-dialog-heading"><span className="eyebrow">A LITTLE HELP GOES A LONG WAY</span><h2 id="new-case-title">Tell us what<br /><em>needs care.</em></h2><p>Share the details with your society’s authorized handling team.</p></div><div className="dialog-scroll detail-body"><form className="portal-form" onSubmit={submit}><fieldset className="records-fieldset" disabled={busy || locked}>
    <label>Home<FormSelect label="Service request home" value={home} onChange={setHome} required options={[{ value: '', label: 'Choose your home…' }, ...homes.map(item => ({ value: item.id, label: 'Home ' + item.label }))]} /></label>
    <label>Category<FormSelect label="Service category" value={category} onChange={setCategory} required options={[{ value: '', label: 'What needs attention?' }, ...options(categories)]} /></label>
    <label>Subject<input value={subject} onChange={event => setSubject(event.target.value)} minLength={5} maxLength={120} required placeholder="The issue, in a few words" /></label>
    <label>Details<textarea value={description} onChange={event => setDescription(event.target.value)} minLength={10} maxLength={4000} required rows={5} placeholder="What happened and what would help?" /></label>
    <label>Priority<FormSelect label="Service priority" value={priority} onChange={setPriority} options={options(priorities)} /></label>
    </fieldset>{priority === 'URGENT' && <p className="form-help">Contact security or emergency services directly for urgent safety issues. This portal saves a request for the handling team.</p>}{error && <p className="form-error" role="alert" ref={feedback}>{error}</p>}{locked && <p className="form-help">Retry keeps these details and the same request identity.</p>}<button className="button button-dark" disabled={busy}>{busy ? 'Saving your request…' : locked ? 'Retry this report' : 'Save service request'}<Icon name="arrow" /></button><p className="form-help">Your conversation is visible to you and the authorized handling team.</p></form></div></PortalDialog>
}

function CaseDetail({ id, user, onClose, onSaved }: { id: string; user: User; onClose: () => void; onSaved: () => void }) {
  const [item, setItem] = useState<Detail | null>(null)
  const [historyPage, setHistoryPage] = useState(1)
  const [revision, setRevision] = useState(0)
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState('')
  const [kind, setKind] = useState('COMMENT')
  const [status, setStatus] = useState('')
  const [handler, setHandler] = useState('')
  const [priority, setPriority] = useState('')
  const [visibility, setVisibility] = useState('RESIDENT_VISIBLE')
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)
  const [locked, setLocked] = useState(false)
  const [error, setError] = useState('')
  const pending = useRef<Record<string, unknown> | null>(null)
  const feedback = useRef<HTMLDivElement>(null)
  useEffect(() => { if (error) feedbackIntoView(feedback.current) }, [error])
  useEffect(() => {
    const controller = new AbortController(); setLoading(true); setLoadError('')
    request<Detail>(`/api/complaints/${encodeURIComponent(id)}?history_page=${historyPage}`, controller.signal).then(value => { setItem(value); setLoading(false); if (value.status === 'CLOSED') setKind('STATUS') }).catch((err: Error) => { if (!controller.signal.aborted) { setLoadError(err.message); setLoading(false) } })
    return () => controller.abort()
  }, [id, historyPage, revision])
  const reload = () => { pending.current = null; setError(''); setLocked(false); setHistoryPage(1); setRevision(n => n + 1) }
  const save = async (event: FormEvent) => {
    event.preventDefault(); if (!item || busy || loading) return; setBusy(true); setError('')
    pending.current ??= { operation_key: crypto.randomUUID(), version: item.version, action: kind, message: message.trim(), visibility: kind === 'COMMENT' ? visibility : 'RESIDENT_VISIBLE', status: kind === 'STATUS' ? status : '', assigned_to: kind === 'ASSIGN' ? handler : '', priority: kind === 'PRIORITY' ? priority : '' }
    try { await mutate(`/api/complaints/${item.id}/updates`, 'POST', pending.current); pending.current = null; setLocked(false); setMessage(''); setStatus(''); setHandler(''); setPriority(''); setVisibility('RESIDENT_VISIBLE'); setKind('COMMENT'); setHistoryPage(1); setRevision(n => n + 1); onSaved() }
    catch (err) { setError((err as Error).message); if (err instanceof APIError && [400, 403, 404, 409].includes(err.status)) { pending.current = null; setLocked(false) } else setLocked(true) }
    finally { setBusy(false) }
  }
  const staff = user.can_handle_complaints
  const nextStates = item ? staff ? transitions[item.status] ?? [] : item.status === 'RESOLVED' ? ['CLOSED', 'IN_PROGRESS'] : item.status === 'CLOSED' ? ['OPEN'] : [] : []
  const updateKinds = item?.status === 'CLOSED' ? [{ value: 'STATUS', label: 'Reopen this request' }] : [{ value: 'COMMENT', label: 'Add to the conversation' }, ...(nextStates.length ? [{ value: 'STATUS', label: staff ? 'Change the status' : 'Close or reopen after resolution' }] : []), ...(staff ? [{ value: 'ASSIGN', label: 'Assign a handler' }, { value: 'PRIORITY', label: 'Change the priority' }] : [])]
  return <PortalDialog titleId="case-detail-title" closeLabel="Close service request details" busy={busy} onClose={onClose}><div className="dialog-heading reviews-dialog-heading review-detail-heading"><span className="eyebrow">{item?.number ?? 'YOUR SERVICE REQUEST'}</span><h2 id="case-detail-title">{item?.subject ?? 'Opening the details…'}</h2>{item && <span className={`case-status case-${item.status.toLowerCase()}`}>{states[item.status]}</span>}</div><div className="dialog-scroll detail-body" aria-busy={loading}>
    {loadError ? <div className="empty-state" role="alert"><p>{loadError}</p><button className="button button-dark" onClick={reload}>Reload service request<Icon name="refresh" /></button></div> : !item ? <p role="status">Opening the conversation…</p> : <><p className="review-body">{item.description}</p><dl className="record-facts"><div><dt>Home & category</dt><dd>{item.home} · {categories[item.category]}</dd></div><div><dt>Priority</dt><dd>{priorities[item.priority]}</dd></div><div><dt>Handler</dt><dd>{item.assigned_name || 'Awaiting assignment'}</dd></div><div><dt>Reported</dt><dd>{item.reporter} · {date(item.created_at)}</dd></div></dl>
      {staff && <a className="text-link upkeep-private-link" href={'#upkeep?case='+item.id}>Plan upkeep for this request<Icon name="arrow" /></a>}
      <section className="review-history case-conversation" aria-label="Service request conversation"><h3>The conversation</h3><ol>{item.updates.map(update => <li key={update.id} className={update.visibility === 'STAFF_ONLY' ? 'staff-note' : ''}><strong>{actions[update.action]}{update.action === 'STATUS' ? ' · ' + states[update.status] : ''}</strong>{update.visibility === 'STAFF_ONLY' && <span className="staff-note-label"><Icon name="shield" />Staff only</span>}<p>{update.message}</p><small>{update.actor} · {date(update.at)}</small></li>)}</ol>{item.history_total > 30 && <div className="pagination history-pagination"><span>{item.history_total} visible updates</span><div><button aria-label="Newer conversation updates" disabled={loading || item.history_page === 1} onClick={() => setHistoryPage(item.history_page - 1)}><Icon name="left" /></button><span>{item.history_page}/{Math.ceil(item.history_total / 30)}</span><button aria-label="Older conversation updates" disabled={loading || item.history_page * 30 >= item.history_total} onClick={() => setHistoryPage(item.history_page + 1)}><Icon name="chevron" /></button></div></div>}</section>
      {item.can_participate ? <form className="portal-form case-update" onSubmit={save}><h3>Keep the story moving.</h3><fieldset className="records-fieldset" disabled={busy || locked || loading}><label>Update type<FormSelect label="Service update type" value={kind} onChange={value => { setKind(value); setStatus(''); setPriority(''); setVisibility('RESIDENT_VISIBLE') }} options={updateKinds} /></label>
        {kind === 'STATUS' && <label>Next status<FormSelect label="Next service status" value={status} onChange={setStatus} required options={[{ value: '', label: 'Choose the next step…' }, ...nextStates.map(value => ({ value, label: states[value] }))]} /></label>}
        {kind === 'ASSIGN' && <label>Handler<FormSelect label="Service handler" value={handler} onChange={setHandler} options={[{ value: '', label: 'Leave unassigned' }, ...item.handlers.map(person => ({ value: person.id, label: person.name }))]} /></label>}
        {kind === 'PRIORITY' && <label>Priority<FormSelect label="Updated service priority" value={priority} onChange={setPriority} required options={[{ value: '', label: 'Choose a priority…' }, ...options(priorities).filter(option => option.value !== item.priority)]} /></label>}
        {kind === 'COMMENT' && staff && <label>Who sees this update?<FormSelect label="Update visibility" value={visibility} onChange={setVisibility} options={[{ value: 'RESIDENT_VISIBLE', label: 'Resident and handling team' }, { value: 'STAFF_ONLY', label: 'Handling team only' }]} /></label>}
        <label>{kind === 'COMMENT' ? 'Your message' : 'Reason for this update'}<textarea aria-label="Service update message" value={message} onChange={event => setMessage(event.target.value)} minLength={5} maxLength={2000} rows={4} required placeholder={visibility === 'STAFF_ONLY' ? 'Private coordination for the handling team' : 'A clear update that helps the next person'} /></label>
      </fieldset>{visibility === 'STAFF_ONLY' && kind === 'COMMENT' && <p className="form-help"><Icon name="shield" />Only authorized handlers can read this note.</p>}{error && <div className="form-error" role="alert" ref={feedback}><p>{error}</p>{!locked && <button className="text-link" type="button" onClick={reload}>Reload latest details<Icon name="refresh" /></button>}</div>}{locked && <p className="form-help">Retry keeps the same details and update identity.</p>}<button className="button button-dark" disabled={busy || loading}>{busy ? 'Saving your update…' : locked ? 'Retry this update' : 'Save update'}<Icon name="check" /></button><p className="form-help">Status and assignment decisions remain in the case history.</p></form> : <p className="form-help">This is your retained personal history. Your former home relationship has ended, so new updates are closed.</p>}
    </>}
  </div></PortalDialog>
}
