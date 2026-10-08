import { useFragmentSync } from '../navigation'
import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { APIError, mutate, request } from '../api'
import type { User } from '../api'
import { Icon } from './Icon'
import { FilterSelect, FormSelect } from './FilterSelect'
import { PortalDialog } from './PortalDialog'

export type Review = { id: string; kind: string; title: string; body: string; flat_id: string; home: string; audience: string; building_code: string; estimate_paise: number; state: string; author_id: string; author: string; submitted_at: number; updated_at: number; version: number; events?: { action: string; reason: string; actor: string; at: number; version: number }[] }
type Home = { id: string; label: string }
type ReviewPage = { items: Review[]; total: number; page: number; page_size: number; homes: Home[] }
const kinds: Record<string, string> = { MAINTENANCE: 'Maintenance request', NOTICE: 'Notice proposal', REGISTRY_CHANGE: 'Registry change request', EXPENSE: 'Expense proposal' }
const states: Record<string, string> = { PENDING: 'Awaiting review', CHANGES_REQUESTED: 'Changes requested', APPROVED: 'Approved', REJECTED: 'Declined', WITHDRAWN: 'Withdrawn', ARCHIVED: 'Archived' }
const audiences: Record<string, string> = { ALL_RESIDENTS: 'All current residents', OWNERS_ONLY: 'Current owners', TENANTS_ONLY: 'Current tenants', COMMITTEE_ONLY: 'Committee & administrators', BUILDING: 'A selected wing' }
const when = (at: number) => new Intl.DateTimeFormat('en-IN', { day: 'numeric', month: 'short', year: 'numeric', timeZone: 'Asia/Kolkata' }).format(new Date(at * 1000))
const money = (paise: number) => new Intl.NumberFormat('en-IN', { style: 'currency', currency: 'INR' }).format(paise / 100)
const linkedReview = (notices: boolean) => {
  const id = new URLSearchParams(window.location.hash.split('?')[1] ?? '').get(notices ? 'notice' : 'request') ?? ''
  return /^[A-Za-z0-9_-]{43}$/.test(id) ? id : ''
}

function IdeasArt() {
  return <svg className="ideas-art" aria-hidden="true" viewBox="0 0 250 215" fill="none"><ellipse cx="130" cy="113" rx="108" ry="89" fill="#dce3ce" /><g transform="rotate(-10 92 106)"><rect x="37" y="43" width="108" height="136" rx="9" fill="#fffdf5" stroke="#a5b28f" /><path d="M59 73h62M59 91h46M59 133h56M59 148h35" stroke="#b9c3a5" strokeWidth="3" strokeLinecap="round" /></g><g transform="rotate(12 171 128)"><rect x="129" y="81" width="92" height="117" rx="8" fill="#ecdcc9" /><path d="M149 113h50M149 129h36M149 163h45" stroke="#a49477" strokeWidth="3" strokeLinecap="round" /></g><circle cx="188" cy="51" r="28" fill="#294b3e" /><path d="m175 51 9 9 18-20" stroke="#fbf9ef" strokeWidth="2" strokeLinecap="round" /></svg>
}

export function Reviews({ user, notices = false }: { user: User; notices?: boolean }) {
  const [data, setData] = useState<ReviewPage | null>(null)
  const [query, setQuery] = useState('')
  const [state, setState] = useState('')
  const [page, setPage] = useState(1)
  const [revision, setRevision] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [creating, setCreating] = useState(false)
  const [selected, setSelected] = useState(() => linkedReview(notices))
  const [editing, setEditing] = useState<Review | null>(null)
  const loadFeedback = useRef<HTMLDivElement>(null)
  useEffect(() => { if (error) loadFeedback.current?.scrollIntoView({ block: 'center', behavior: 'instant' }) }, [error])
  useFragmentSync(()=>{ const update = () => setSelected(linkedReview(notices)); update()},[notices])
  useEffect(() => {
    const controller = new AbortController(); setLoading(true); setError('')
    const timer = setTimeout(() => request<ReviewPage>(`/api/${notices ? 'notices' : 'reviews'}?${new URLSearchParams({ q: query, state, page: String(page) })}`, controller.signal).then(value => { setData(value); setLoading(false) }).catch((err: Error) => { if (!controller.signal.aborted) { setError(err.message); setLoading(false) } }), query ? 200 : 0)
    return () => { controller.abort(); clearTimeout(timer) }
  }, [notices, query, state, page, revision])
  const clear = () => { setQuery(''); setState(''); setPage(1) }
  return <div className="page-enter reviews-page">
    <section className="reviews-hero"><div><span className="eyebrow">{notices ? 'THE COMMUNITY NOTICEBOARD' : 'A LITTLE THOUGHT. A BETTER TOMORROW.'}</span><h1>{notices ? <>Good to know.<br /><em>Good to share.</em></> : <>Every idea deserves<br /><em>a thoughtful review.</em></>}</h1><p>{notices ? 'Approved updates for the people who need to see them.' : user.can_review_requests ? 'Hear the requests. Review the details. Help the right things move forward.' : 'Share a request, follow its review and keep the conversation in one place.'}</p><button className="button button-dark" disabled={!data || loading || !!error} onClick={() => setCreating(true)}>{notices ? 'Propose a notice' : 'Submit a request'}<Icon name="plus" /></button></div><IdeasArt /></section>
    {!notices && <div className="review-principles"><span><Icon name="community" />Submitted by one person</span><span><Icon name="shield" />Reviewed by another</span><span><Icon name="records" />Every decision remembered</span></div>}
    <section className="registry-panel"><div className="registry-toolbar"><div><h2>{notices ? 'Around the community' : user.can_review_requests ? 'The review desk' : 'Your requests'}</h2><p>{notices ? 'Only approved notices matching your current access appear here.' : user.can_review_requests ? 'Approve, request changes or decline. Your own submissions need another reviewer.' : 'Your submissions and their decisions are private to you and the reviewers.'}</p></div><span className="result-count" role="status">{loading ? 'Opening the desk…' : error ? 'Connection unavailable' : `${data?.total ?? 0} ${notices ? 'notices' : 'requests'} found`}</span></div>
      <div className={`filter-row review-filters ${notices ? 'notice-filters' : ''}`}><label className="search-control"><span className="sr-only">{notices ? 'Search notices' : 'Search requests'}</span><Icon name="search" /><input type="search" placeholder={notices ? 'Find a community update…' : 'Find a request or an idea…'} maxLength={100} value={query} onChange={event => { setQuery(event.target.value); setPage(1) }} /></label>{!notices && <FilterSelect label="Filter requests by status" value={state} onChange={value => { setState(value); setPage(1) }} options={[{ value: '', label: 'All statuses' }, ...Object.entries(states).map(([value, label]) => ({ value, label }))]} />}<button className="clear-button" disabled={!query && !state} onClick={clear}>Clear<Icon name="close" /></button></div>
      <div aria-busy={loading}>{error ? <div className="empty-state" role="alert" ref={loadFeedback}><Icon name="globe" /><h3>Let’s open that again.</h3><p>{error}</p><button className="button button-dark" onClick={() => setRevision(n => n + 1)}>Try again<Icon name="refresh" /></button></div> : loading ? <div className="home-skeleton skeleton" aria-hidden="true" /> : !data?.items.length ? <div className="records-empty"><span className="chapter-icon"><Icon name={notices ? 'document' : 'community'} /></span><h3>{query || state ? 'Nothing matches just yet.' : notices ? 'A little quiet, for now.' : 'Your next idea starts here.'}</h3><p>{query || state ? 'Try another word or status.' : notices ? 'The next approved community update will appear here.' : 'Submit a request and its review history will stay together.'}</p>{query || state ? <button className="text-link" onClick={clear}>Clear filters<Icon name="close" /></button> : <button className="text-link" onClick={() => setCreating(true)}>{notices ? 'Propose the first notice' : 'Write the first request'}<Icon name="arrow" /></button>}</div> : <div className={notices ? 'notice-grid' : 'review-list'}>{data.items.map(item => <button key={item.id} className={notices ? 'notice-card' : 'review-row'} aria-label={`Open ${item.title}`} onClick={() => setSelected(item.id)}><span className="review-row-top"><span className="eyebrow">{notices ? item.audience === 'BUILDING' ? 'WING ' + item.building_code : audiences[item.audience] : kinds[item.kind]}</span>{!notices && <span className={`review-status review-${item.state.toLowerCase()}`}>{states[item.state]}</span>}</span><strong>{item.title}</strong><p>{item.body}</p><span className="review-row-bottom"><span>{notices ? when(item.updated_at) : item.author + (item.home ? ' · Home ' + item.home : '')}</span><Icon name="arrow" /></span></button>)}</div>}</div>
      {data && !loading && !error && data.total > 0 && <div className="pagination"><span>Showing {(data.page - 1) * 12 + 1}–{Math.min(data.page * 12, data.total)} of {data.total} {notices ? 'notices' : 'requests'}</span><div><button aria-label="Previous requests page" onClick={() => setPage(data.page - 1)} disabled={data.page === 1}><Icon name="left" /></button><span>Page {data.page} of {Math.ceil(data.total / 12)}</span><button aria-label="Next requests page" onClick={() => setPage(data.page + 1)} disabled={data.page * 12 >= data.total}><Icon name="chevron" /></button></div></div>}
    </section>
    {!notices && <p className="records-footnote"><Icon name="leaf" />An approved proposal records permission for follow-up. Financial entries and registry changes retain their own controls.</p>}
    {(creating || editing) && <NewReview homes={data?.homes ?? []} notice={notices} existing={editing} onClose={() => { setCreating(false); setEditing(null) }} onSaved={id => { setCreating(false); setEditing(null); setSelected(notices ? '' : id); setRevision(n => n + 1) }} />}
    {selected && <ReviewDetail id={selected} notices={notices} user={user} onClose={() => { setSelected(''); if (linkedReview(notices)) window.history.replaceState(null, '', notices ? '#community' : '#reviews') }} onSaved={() => setRevision(n => n + 1)} onEdit={item => { setSelected(''); setEditing(item) }} />}
  </div>
}

function NewReview({ homes, notice, existing, onClose, onSaved }: { homes: Home[]; notice: boolean; existing: Review | null; onClose: () => void; onSaved: (id: string) => void }) {
  const [kind, setKind] = useState(existing?.kind ?? (notice ? 'NOTICE' : 'MAINTENANCE'))
  const [home, setHome] = useState(existing?.flat_id ?? '')
  const [title, setTitle] = useState(existing?.title ?? '')
  const [body, setBody] = useState(existing?.body ?? '')
  const [audience, setAudience] = useState(existing?.audience ?? '')
  const [wing, setWing] = useState(existing?.building_code ?? '')
  const [estimate, setEstimate] = useState(existing?.estimate_paise ? String(existing.estimate_paise / 100) : '')
  const [busy, setBusy] = useState(false)
  const [locked, setLocked] = useState(false)
  const [error, setError] = useState('')
  const feedback = useRef<HTMLParagraphElement>(null)
  useEffect(() => { if (error) feedback.current?.scrollIntoView({ block: 'nearest' }) }, [error])
  const pending = useRef<Record<string, unknown> | null>(null)
  const submit = async (event: FormEvent) => {
    event.preventDefault(); if (busy) return; setBusy(true); setError('')
    pending.current ??= { operation_key: crypto.randomUUID(), version: existing?.version ?? 0, kind, title: title.trim(), body: body.trim(), flat_id: kind === 'NOTICE' ? '' : home, audience: kind === 'NOTICE' ? audience : '', building_code: kind === 'NOTICE' && audience === 'BUILDING' ? wing : '', estimate: kind === 'EXPENSE' ? estimate : '' }
    try { const result = await mutate<{ id: string }>(existing ? `/api/reviews/${existing.id}/resubmit` : '/api/reviews', 'POST', pending.current); onSaved(result.id) }
    catch (err) { setError((err as Error).message); if (err instanceof APIError && [400, 403, 404].includes(err.status)) { pending.current = null; setLocked(false) } else setLocked(true) }
    finally { setBusy(false) }
  }
  return <PortalDialog titleId="new-review-title" closeLabel="Close request form" busy={busy} onClose={onClose}><div className="dialog-heading reviews-dialog-heading"><span className="eyebrow">A THOUGHTFUL NEXT STEP</span><h2 id="new-review-title">{existing ? <>A little <em>revision.</em></> : <>Share your <em>idea.</em></>}</h2><p>{kind === 'NOTICE' ? 'A separate reviewer will approve the audience and content before this notice is published.' : 'Send the details for review. Approval does not make a payment or change a financial balance.'}</p></div><div className="dialog-scroll detail-body"><form className="portal-form" onSubmit={submit}><fieldset className="records-fieldset" disabled={busy || locked}>
    {!notice && !existing && <label>Request type<FormSelect label="Request type" value={kind} onChange={setKind} options={Object.entries(kinds).map(([value, label]) => ({ value, label }))} /></label>}
    <label>Title<input value={title} onChange={event => setTitle(event.target.value)} minLength={5} maxLength={120} required placeholder="The idea, in a few words" /></label><label>Description<textarea value={body} onChange={event => setBody(event.target.value)} minLength={10} maxLength={4000} rows={5} required placeholder="The details that help someone review your request" /></label>
    {kind === 'NOTICE' ? <><label>Who should see this?<FormSelect label="Notice audience" value={audience} onChange={setAudience} required options={[{ value: '', label: 'Choose an audience…' }, ...Object.entries(audiences).map(([value, label]) => ({ value, label }))]} /></label>{audience === 'BUILDING' && <label>Wing<FormSelect label="Notice wing" value={wing} onChange={setWing} required options={[{ value: '', label: 'Choose a wing…' }, ...['A', 'B', 'C'].map(value => ({ value, label: 'Wing ' + value }))]} /></label>}</> : <label>Home (optional)<FormSelect label="Request home" value={home} onChange={setHome} options={[{ value: '', label: 'Shared area / no specific home' }, ...homes.map(item => ({ value: item.id, label: 'Home ' + item.label }))]} /></label>}
    {kind === 'EXPENSE' && <label>Estimated amount in rupees (optional)<input inputMode="decimal" value={estimate} onChange={event => setEstimate(event.target.value)} pattern="(0|[1-9][0-9]{0,7})([.][0-9]{1,2})?" maxLength={11} placeholder="For review only" /></label>}
    </fieldset>{error && <p className="form-error" role="alert" ref={feedback}>{error}</p>}{locked && <p className="form-help">Retry uses the same submission identity and details.</p>}<button className="button button-dark" disabled={busy}>{busy ? 'Sending for review…' : locked ? 'Retry this submission' : existing ? 'Resubmit for review' : 'Send for review'}<Icon name="arrow" /></button><p className="form-help">The submitter cannot approve their own request. Every decision keeps its author and reason.</p></form></div></PortalDialog>
}

function ReviewDetail({ id, notices, user, onClose, onSaved, onEdit }: { id: string; notices: boolean; user: User; onClose: () => void; onSaved: () => void; onEdit: (item: Review) => void }) {
  const [item, setItem] = useState<Review | null>(null)
  const [loadError, setLoadError] = useState('')
  const [revision, setRevision] = useState(0)
  const [decision, setDecision] = useState('APPROVED')
  const [reason, setReason] = useState('')
  const [confirmed, setConfirmed] = useState(false)
  const [busy, setBusy] = useState(false)
  const [locked, setLocked] = useState(false)
  const [error, setError] = useState('')
  const pending = useRef<Record<string, unknown> | null>(null)
  const feedback = useRef<HTMLParagraphElement>(null)
  useEffect(() => { if (error) feedback.current?.scrollIntoView({ block: 'nearest' }) }, [error])
  useEffect(() => {
    const controller = new AbortController(); setLoadError(''); setItem(null)
    request<Review>(`/api/${notices ? 'notices' : 'reviews'}/${encodeURIComponent(id)}`, controller.signal).then(setItem).catch((err: Error) => { if (!controller.signal.aborted) setLoadError(err.message) })
    return () => controller.abort()
  }, [id, notices, revision])
  const action = async (choice: string) => {
    if (!item || busy) return
    setBusy(true); setError('')
    pending.current ??= { operation_key: crypto.randomUUID(), version: item.version, decision: choice, reason: reason.trim(), confirmed: true }
    try { await mutate(`/api/reviews/${item.id}/decision`, 'POST', pending.current); pending.current = null; setLocked(false); setConfirmed(false); setReason(''); setRevision(n => n + 1); onSaved() }
    catch (err) { setError((err as Error).message); if (err instanceof APIError && [400, 403, 404, 409].includes(err.status)) { pending.current = null; setLocked(false) } else setLocked(true) }
    finally { setBusy(false) }
  }
  const reload = () => { pending.current = null; setLocked(false); setConfirmed(false); setError(''); setRevision(n => n + 1) }
  const canDecide = item && !notices && user.can_review_requests && item.author_id !== user.id && item.state === 'PENDING'
  const canWithdraw = item && !notices && item.author_id === user.id && ['PENDING', 'CHANGES_REQUESTED'].includes(item.state)
  const canArchive = item && !notices && user.can_review_requests && item.kind === 'NOTICE' && item.state === 'APPROVED'
  return <PortalDialog titleId="review-detail-title" closeLabel="Close request details" busy={busy} onClose={onClose}><div className="dialog-heading reviews-dialog-heading review-detail-heading"><span className="eyebrow">{item ? notices ? 'COMMUNITY NOTICE' : kinds[item.kind] : 'YOUR COMMUNITY'}</span><h2 id="review-detail-title">{loadError ? 'Request unavailable' : item?.title ?? 'Opening this request…'}</h2>{item && <span className={`review-status review-${item.state.toLowerCase()}`}>{notices ? 'Published' : states[item.state]}</span>}</div><div className="dialog-scroll detail-body">{loadError ? <div className="empty-state" role="alert"><p>{loadError}</p><button className="button button-dark" onClick={reload}>Reload request<Icon name="refresh" /></button></div> : !item ? <p role="status">Opening the details…</p> : <>
    <p className="review-full-body">{item.body}</p><dl className="record-facts">{item.home && <div><dt>Home</dt><dd>{item.home}</dd></div>}{item.audience && <div><dt>Audience</dt><dd>{item.audience === 'BUILDING' ? 'Current residents of Wing ' + item.building_code : audiences[item.audience]}</dd></div>}{item.estimate_paise > 0 && <div><dt>Proposed estimate</dt><dd>{money(item.estimate_paise)} · no financial entry</dd></div>}<div><dt>{notices ? 'Published' : 'Submitted'}</dt><dd>{when(notices ? item.updated_at : item.submitted_at)}</dd></div></dl>
    {notices && <a className="text-link notice-share" href={'https://wa.me/?text=' + encodeURIComponent('A community update is available in your society portal: ' + window.location.origin + window.location.pathname + '#community?notice=' + item.id)} target="_blank" rel="noopener noreferrer">Share notice link on WhatsApp<Icon name="arrow" /></a>}
    {!notices && <><h3 className="review-history-title">The story so far</h3><ol className="review-history">{item.events?.map(event => <li key={event.version}><span className="review-history-dot" /><div><strong>{event.action === 'SUBMITTED' ? 'Sent for review' : event.action === 'RESUBMITTED' ? 'Revised and resubmitted' : states[event.action]}</strong><p>{event.reason}</p><small>{event.actor} · {when(event.at)}</small></div></li>)}</ol>{item.author_id === user.id && item.state === 'PENDING' && <p className="review-own-note"><Icon name="shield" />Another reviewer will decide this request.</p>}{item.author_id === user.id && item.state === 'CHANGES_REQUESTED' && <button className="button button-dark" disabled={busy} onClick={() => onEdit(item)}>Revise this request<Icon name="arrow" /></button>}</>}
    {(canDecide || canWithdraw || canArchive) && <form className="portal-form record-confirm" onSubmit={event => { event.preventDefault(); void action(canDecide ? decision : canWithdraw ? 'WITHDRAWN' : 'ARCHIVED') }}><h3>{canDecide ? 'Your thoughtful decision' : canWithdraw ? 'Withdraw this request' : 'Archive this notice'}</h3><fieldset className="records-fieldset" disabled={busy || locked}>{canDecide && <label>Decision<FormSelect label="Review decision" value={decision} onChange={value => { setDecision(value); setConfirmed(false) }} options={[{ value: 'APPROVED', label: item.kind === 'NOTICE' ? 'Approve & publish notice' : 'Approve for follow-up' }, { value: 'CHANGES_REQUESTED', label: 'Request changes' }, { value: 'REJECTED', label: 'Decline request' }]} /></label>}<label>Reason for this decision<input value={reason} onChange={event => setReason(event.target.value)} required minLength={5} maxLength={300} placeholder="A clear reason for the record" /></label><label className="checkbox-label"><input type="checkbox" checked={confirmed} onChange={event => setConfirmed(event.target.checked)} required />{canDecide ? item.kind === 'NOTICE' && decision === 'APPROVED' ? 'I reviewed the content and audience. Approval publishes this notice.' : 'I reviewed this request and confirm the selected decision.' : canWithdraw ? 'I confirm this request should be withdrawn.' : 'I confirm this notice should be archived and removed from the noticeboard.'}</label></fieldset><button className="button button-dark" disabled={busy || !confirmed}>{busy ? 'Recording your decision…' : locked ? 'Retry this decision' : canDecide ? decision === 'APPROVED' ? item.kind === 'NOTICE' ? 'Approve & publish' : 'Approve request' : decision === 'CHANGES_REQUESTED' ? 'Request these changes' : 'Decline request' : canWithdraw ? 'Withdraw request' : 'Archive notice'}<Icon name="check" /></button>{canDecide && item.kind !== 'NOTICE' && <p className="form-help">Approval records permission for follow-up. It does not pay an expense, post money or change the registry.</p>}</form>}
    {error && <div><p className="form-error" role="alert" ref={feedback}>{error}</p>{!locked && <button className="text-link" disabled={busy} onClick={reload}>Reload latest details<Icon name="refresh" /></button>}</div>}
  </>}</div></PortalDialog>
}
