import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { APIError, mutate, request } from '../api'
import type { User } from '../api'
import { Icon } from './Icon'
import { FilterSelect, FormSelect } from './FilterSelect'
import { PortalDialog } from './PortalDialog'

type Entry = { id: string; flat_id: string; home: string; kind: string; amount_paise: number; date: string; description: string; payer: string; method: string; reference: string; source_note: string; state: string; created_by: string; created_at: number; posted_by: string; posted_at: number; reversal_reason: string; reversed_by: string; reversed_at: number; receipt_id: string; receipt_number: string; pdf_state: string }
type Home = { id: string; label: string }
type RecordPage = { items: Entry[]; total: number; page: number; page_size: number; debit_paise: number; credit_paise: number; balance_paise: number; drafts: number; homes: Home[] }
const stateLabel = (state: string) => ({ POSTED: 'Confirmed', DRAFT: 'Draft', REVERSED: 'Reversed', DISCARDED: 'Discarded' })[state] ?? state
const kinds: Record<string, string> = { RECEIVED: 'Money received', CHARGE: 'Given charge', OPENING_DEBIT: 'Opening amount due', OPENING_CREDIT: 'Opening credit' }
const methods: Record<string, string> = { CASH: 'Cash', BANK_TRANSFER: 'Bank transfer', UPI: 'UPI', CHEQUE: 'Cheque' }
const money = (paise: number) => new Intl.NumberFormat('en-IN', { style: 'currency', currency: 'INR', minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(paise / 100)
const date = () => new Intl.DateTimeFormat('en-CA', { year: 'numeric', month: '2-digit', day: '2-digit', timeZone: 'Asia/Kolkata' }).format(new Date())
const displayDate = (value: string) => new Intl.DateTimeFormat('en-IN', { day: 'numeric', month: 'short', year: 'numeric', timeZone: 'Asia/Kolkata' }).format(new Date(value + 'T00:00:00+05:30'))

function ReceiptArt() {
  return <svg className="records-art" viewBox="0 0 220 230" fill="none" aria-hidden="true"><circle cx="115" cy="112" r="95" fill="#d9dfca" /><g transform="rotate(9 108 118)"><path d="M51 29h116v179l-11-7-12 7-11-7-12 7-11-7-12 7-12-7-12 7-11-7-11 7Z" fill="#fbfaf4" stroke="#a4b092" /><path d="M74 64h65M74 83h45M74 133h69M74 150h46" stroke="#b7bda9" strokeWidth="3" strokeLinecap="round" /><circle cx="111" cy="108" r="13" fill="#e4e9d9" /><path d="m105 108 4 4 8-8" stroke="#5c744c" strokeWidth="2" strokeLinecap="round" /><path d="M130 175h13" stroke="#83936c" strokeWidth="3" strokeLinecap="round" /></g><circle cx="177" cy="170" r="27" fill="#2c5041" /><path d="m164 170 9 9 17-19" stroke="#f8f7ee" strokeWidth="2" strokeLinecap="round" /></svg>
}

export function Records({ user, receipts }: { user: User; receipts: boolean }) {
  const [data, setData] = useState<RecordPage | null>(null)
  const [home, setHome] = useState('')
  const [query, setQuery] = useState('')
  const [state, setState] = useState('')
  const [page, setPage] = useState(1)
  const [revision, setRevision] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const feedback = useRef<HTMLParagraphElement>(null)
  useEffect(() => { if (error) feedback.current?.scrollIntoView({ block: 'nearest' }) }, [error])
  const [creating, setCreating] = useState(false)
  const [selected, setSelected] = useState('')
  useEffect(() => {
    const controller = new AbortController(); setLoading(true); setError('')
    const timer = window.setTimeout(() => request<RecordPage>(`/api/entries?${new URLSearchParams({ home, q: query, state, receipts: String(receipts), page: String(page) })}`, controller.signal).then(value => { setData(value); setLoading(false) }).catch((err: Error) => { if (!controller.signal.aborted) { setError(err.message); setLoading(false) } }), query ? 200 : 0)
    return () => { controller.abort(); clearTimeout(timer) }
  }, [home, query, state, receipts, page, revision])
  const clear = () => { setHome(''); setQuery(''); setState(''); setPage(1) }
  if (!user.can_read_records) return <section className="empty-state"><Icon name="shield" /><h1>These records need access.</h1><p>Financial access is granted separately for each home. Ask the society officer to review your access.</p><a className="button button-dark" href="#homes">Return to your homes<Icon name="arrow" /></a></section>
  return <div className="page-enter records-page">
    <section className="records-hero"><div><span className="eyebrow">{receipts ? 'A CLEAR RECORD, ALWAYS' : 'EVERYDAY RECORDS, BEAUTIFULLY IN ORDER'}</span><h1>{receipts ? <>Good records.<br /><em>Peace of mind.</em></> : <>The little details,<br /><em>all accounted for.</em></>}</h1><p>{receipts ? 'A receipt for every confirmed amount received. Find it here, whenever you need it.' : 'Keep given charges, opening balances and money already received together.'}</p>{!receipts && user.can_manage_records && <button className="button button-dark" onClick={() => setCreating(true)} disabled={!data || loading || !!error}>Add an entry<Icon name="plus" /></button>}</div><ReceiptArt /></section>
    <div className="records-metrics" aria-label="Financial summary">
      <div><span>Charges & opening dues</span><strong>{data && !loading && !error ? money(data.debit_paise) : '—'}</strong><small>Confirmed amounts supplied manually</small></div>
      <div><span>Received & opening credits</span><strong>{data && !loading && !error ? money(data.credit_paise) : '—'}</strong><small>Confirmed credits for these homes</small></div>
      <div className="records-balance"><span>{(data?.balance_paise ?? 0) < 0 ? 'Credit balance' : 'Balance to account for'}</span><strong>{data && !loading && !error ? money(Math.abs(data.balance_paise)) : '—'}</strong><small>{home ? 'For the selected home' : user.can_read_all_records ? 'Across the community' : 'Across your permitted homes'} · reversals excluded</small></div>
    </div>
    <section className="registry-panel" aria-labelledby="records-title">
      <div className="registry-toolbar"><div><h2 id="records-title">{receipts ? 'Your receipt collection' : 'The entry book'}</h2><p>{receipts ? 'Open a receipt to view the record and download its PDF.' : user.can_manage_records ? 'Save a draft, review the details, then confirm. Every change has a trail.' : 'Confirmed entries for your permitted homes.'}</p></div><span className="result-count" role="status">{loading ? 'Opening records…' : error ? 'Connection unavailable' : `${data?.total ?? 0} ${receipts ? 'receipts' : 'entries'} found`}</span></div>
      <div className="filter-row records-filters"><label className="search-control"><span className="sr-only">Search records</span><Icon name="search" /><input type="search" placeholder={receipts ? 'Find a receipt, home or person…' : 'Find a home, person or note…'} value={query} maxLength={100} onChange={event => { setQuery(event.target.value); setPage(1) }} /></label>
        <FilterSelect label="Filter records by home" value={home} onChange={value => { setHome(value); setPage(1) }} options={[{ value: '', label: 'All homes' }, ...data?.homes.map(item => ({ value: item.id, label: 'Home ' + item.label })) ?? []]} />
        <FilterSelect label="Filter records by status" value={state} onChange={value => { setState(value); setPage(1) }} options={[{ value: '', label: 'All statuses' }, ...(!receipts && user.can_read_all_records ? [{ value: 'DRAFT', label: 'Draft' }, { value: 'DISCARDED', label: 'Discarded' }] : []), { value: 'POSTED', label: 'Confirmed' }, { value: 'REVERSED', label: 'Reversed' }]} />
        <button className="clear-button" disabled={!home && !query && !state} onClick={clear}>Clear<Icon name="close" /></button>
      </div>
      <div aria-busy={loading}>
        {error ? <div className="empty-state" role="alert"><Icon name="globe" /><h3>Let’s try that again.</h3><p>{error}</p><button className="button button-dark" onClick={() => setRevision(n => n + 1)}>Try again<Icon name="refresh" /></button></div> : loading ? <div className="home-skeleton skeleton" aria-hidden="true" /> : !data?.items.length ? <div className="records-empty"><span className="chapter-icon"><Icon name={receipts ? 'receipt' : 'records'} /></span><h3>{home || query || state ? 'Nothing matches just yet.' : receipts ? 'The collection begins here.' : 'A fresh page for your community.'}</h3><p>{home || query || state ? 'Try another home, word or status.' : receipts ? 'Confirm a money-received entry and its receipt will appear here.' : 'Start with an opening balance, a given charge or an amount already received.'}</p>{home || query || state ? <button className="text-link" onClick={clear}>Clear filters<Icon name="close" /></button> : !receipts && user.can_manage_records ? <button className="text-link" onClick={() => setCreating(true)}>Write the first entry<Icon name="arrow" /></button> : receipts && user.can_manage_records ? <a className="text-link" href="#entries">Open the entry book<Icon name="arrow" /></a> : null}</div> : <div className="record-list">{data.items.map(entry => <button className="record-row" key={entry.id} aria-label={`Open ${receipts ? entry.receipt_number : entry.description}, ${entry.home}`} onClick={() => setSelected(entry.id)}><span className={`record-icon ${entry.kind === 'RECEIVED' ? 'received' : ''}`}><Icon name={receipts ? 'receipt' : entry.kind === 'RECEIVED' ? 'check' : 'records'} /></span><span className="record-copy"><strong>{receipts ? entry.receipt_number : entry.description}</strong><span>Home {entry.home} <i /> {receipts ? entry.payer : kinds[entry.kind]} <i /> {displayDate(entry.date)}</span></span><span className="record-amount">{money(entry.amount_paise)}<small className={`record-status record-status-${entry.state.toLowerCase()}`}>{stateLabel(entry.state)}{receipts && entry.pdf_state !== 'READY' && entry.state !== 'REVERSED' ? entry.pdf_state === 'FAILED' ? ' · PDF needs attention' : ' · Preparing PDF' : ''}</small></span><Icon name="arrow" /></button>)}</div>}
      </div>
      {data && !loading && !error && data.total > 0 && <div className="pagination"><span>Showing {(data.page - 1) * 12 + 1}–{Math.min(data.page * 12, data.total)} of {data.total} records</span><div><button aria-label="Previous records page" onClick={() => setPage(data.page - 1)} disabled={data.page === 1}><Icon name="left" /></button><span>Page {data.page} of {Math.ceil(data.total / 12)}</span><button aria-label="Next records page" onClick={() => setPage(data.page + 1)} disabled={data.page * 12 >= data.total}><Icon name="chevron" /></button></div></div>}
    </section>
    <p className="records-footnote"><Icon name="leaf" />Payments happen outside the portal. These totals reflect supplied entries; they are not an automatic bill.</p>
    {creating && data && <NewEntry homes={data.homes} initialHome={home} onClose={() => setCreating(false)} onCreated={id => { setCreating(false); setSelected(id); setRevision(n => n + 1) }} />}
    {selected && <RecordDetail id={selected} user={user} onClose={() => setSelected('')} onSaved={() => setRevision(n => n + 1)} />}
  </div>
}

function NewEntry({ homes, initialHome, onClose, onCreated }: { homes: Home[]; initialHome: string; onClose: () => void; onCreated: (id: string) => void }) {
  const [home, setHome] = useState(initialHome)
  const [kind, setKind] = useState('RECEIVED')
  const [amount, setAmount] = useState('')
  const [entryDate, setEntryDate] = useState(date)
  const [description, setDescription] = useState('')
  const [payer, setPayer] = useState('')
  const [method, setMethod] = useState('BANK_TRANSFER')
  const [reference, setReference] = useState('')
  const [sourceNote, setSourceNote] = useState('')
  const [error, setError] = useState('')
  const feedback = useRef<HTMLParagraphElement>(null)
  useEffect(() => { if (error) feedback.current?.scrollIntoView({ block: 'nearest' }) }, [error])
  const [busy, setBusy] = useState(false)
  const [locked, setLocked] = useState(false)
  const pending = useRef<Record<string, unknown> | null>(null)
  const submit = async (event: FormEvent) => {
    event.preventDefault(); if (busy) return; setBusy(true); setError('')
    pending.current ??= { operation_key: crypto.randomUUID(), flat_id: home, kind, amount, date: entryDate, description: description.trim(), payer: kind === 'RECEIVED' ? payer.trim() : '', method: kind === 'RECEIVED' ? method : '', reference: kind === 'RECEIVED' ? reference.trim() : '', source_note: sourceNote.trim() }
    try { const result = await mutate<{id: string}>('/api/entries', 'POST', pending.current); onCreated(result.id) }
    catch (err) { setError((err as Error).message); if (err instanceof APIError && [400, 403, 404].includes(err.status)) { pending.current = null; setLocked(false) } else setLocked(true) }
    finally { setBusy(false) }
  }
  return <PortalDialog titleId="new-entry-title" closeLabel="Close new entry" busy={busy} onClose={onClose}><div className="dialog-heading records-dialog-heading"><span className="eyebrow">ONE DETAIL AT A TIME</span><h2 id="new-entry-title">A new <em>entry.</em></h2><p>Save a draft first. You’ll review it before it becomes a confirmed record.</p></div><div className="dialog-scroll detail-body"><form className="portal-form" onSubmit={submit}>
    <fieldset disabled={busy || locked} className="records-fieldset"><div className="form-pair"><label>Home<FormSelect label="Entry home" value={home} onChange={setHome} required options={[{ value: '', label: 'Choose a home…' }, ...homes.map(item => ({ value: item.id, label: 'Home ' + item.label }))]} /></label><label>Entry type<FormSelect label="Entry type" value={kind} onChange={setKind} options={Object.entries(kinds).map(([value, label]) => ({ value, label }))} /></label></div>
    <div className="form-pair"><label>Amount in rupees<input inputMode="decimal" value={amount} onChange={event => setAmount(event.target.value)} required pattern="(0|[1-9][0-9]{0,7})([.][0-9]{1,2})?" maxLength={11} placeholder="For example, 1500.00" /></label><label>Entry date<input type="date" value={entryDate} onChange={event => setEntryDate(event.target.value)} required min="1900-01-01" max={date()} /></label></div>
    <label>Description<input value={description} onChange={event => setDescription(event.target.value)} required minLength={5} maxLength={300} placeholder={kind === 'RECEIVED' ? 'For example, maintenance already paid for October' : 'For example, given maintenance charge for October'} /></label>
    {kind === 'RECEIVED' && <><label>Received from<input value={payer} onChange={event => setPayer(event.target.value)} required minLength={2} maxLength={120} autoComplete="off" placeholder="Name as supplied" /></label><div className="form-pair"><label>Received via<FormSelect label="Received via" value={method} onChange={setMethod} options={Object.entries(methods).map(([value, label]) => ({ value, label }))} /></label><label>Reference{method === 'CASH' && ' (optional)'}<input value={reference} onChange={event => setReference(event.target.value)} required={method !== 'CASH'} maxLength={120} autoComplete="off" placeholder={method === 'CASH' ? 'A note, if supplied' : 'Transfer or cheque reference'} /></label></div></>}
    <label>Source / evidence note (optional)<input value={sourceNote} onChange={event => setSourceNote(event.target.value)} maxLength={300} placeholder="For example, from the supplied committee record" /></label>
    </fieldset>
    {error && <p ref={feedback} className="form-error" role="alert">{error}</p>}
    {locked && <p className="form-help">Retry sends the same draft with the same identity, so a lost response cannot create it twice.</p>}
    <button className="button button-dark" disabled={busy}>{busy ? 'Saving your draft…' : locked ? 'Retry saving this draft' : 'Save draft for review'}<Icon name="arrow" /></button>
    <p className="form-help">A draft does not change the balance or produce a receipt.</p>
  </form></div></PortalDialog>
}

function RecordDetail({ id, user, onClose, onSaved }: { id: string; user: User; onClose: () => void; onSaved: () => void }) {
  const [entry, setEntry] = useState<Entry | null>(null)
  const [loadError, setLoadError] = useState('')
  const [error, setError] = useState('')
  const feedback = useRef<HTMLParagraphElement>(null)
  useEffect(() => { if (error) feedback.current?.scrollIntoView({ block: 'nearest' }) }, [error])
  const [revision, setRevision] = useState(0)
  const [busy, setBusy] = useState(false)
  const [confirmed, setConfirmed] = useState(false)
  const [correcting, setCorrecting] = useState(false)
  const [reason, setReason] = useState('')
  const [reversalLocked, setReversalLocked] = useState(false)
  const operations = useRef({ post: crypto.randomUUID(), reverse: crypto.randomUUID(), discard: crypto.randomUUID() })
  const retryReason = useRef('')
  useEffect(() => {
    const controller = new AbortController(); let timer: number | undefined
    const load = () => request<Entry>(`/api/entries/${encodeURIComponent(id)}`, controller.signal).then(value => { setEntry(value); setLoadError(''); if (value.pdf_state === 'PENDING' || value.pdf_state === 'RUNNING') timer = window.setTimeout(load, 1000) }).catch((err: Error) => { if (!controller.signal.aborted) setLoadError(err.message) })
    void load(); return () => { controller.abort(); clearTimeout(timer) }
  }, [id, revision])
  const action = async (kind: 'post' | 'reverse' | 'discard' | 'retry') => {
    if (!entry || busy) return; setBusy(true); setError('')
    if ((kind === 'reverse' || kind === 'discard') && !reversalLocked) retryReason.current = reason.trim()
    try {
      if (kind === 'retry') await mutate(`/api/receipts/${encodeURIComponent(entry.receipt_id)}/retry`, 'POST', {})
      else await mutate(`/api/entries/${encodeURIComponent(id)}/${kind}`, 'POST', { operation_key: operations.current[kind], confirmed: true, reason: kind === 'reverse' || kind === 'discard' ? retryReason.current : '' })
      setCorrecting(false); setConfirmed(false); setRevision(n => n + 1); onSaved()
    } catch (err) { setError((err as Error).message); if (kind === 'reverse' || kind === 'discard') setReversalLocked(!(err instanceof APIError && [400, 403, 404].includes(err.status))) }
    finally { setBusy(false) }
  }
  const download = async () => {
    if (!entry || busy) return; setBusy(true); setError('')
    try {
      const response = await fetch(`/api/receipts/${encodeURIComponent(entry.receipt_id)}/download`, { cache: 'no-store', credentials: 'same-origin' })
      if (!response.ok) { if (response.status === 401) window.dispatchEvent(new Event('session-expired')); throw new Error(response.status === 403 || response.status === 404 ? 'Your current account cannot download this receipt.' : 'The receipt could not be downloaded. Try again.') }
      const url = URL.createObjectURL(await response.blob()); const link = document.createElement('a'); link.href = url; link.download = entry.receipt_number + (entry.state === 'REVERSED' ? '-original-reversed' : '') + '.pdf'; document.body.append(link); link.click(); link.remove(); window.setTimeout(() => URL.revokeObjectURL(url), 1000)
    } catch (err) { setError((err as Error).message) }
    finally { setBusy(false) }
  }
  return <PortalDialog titleId="record-detail-title" closeLabel="Close entry details" onClose={onClose} busy={busy}>
    <div className="dialog-heading records-dialog-heading"><span className="eyebrow">{entry ? `HOME ${entry.home} · ${kinds[entry.kind].toUpperCase()}` : 'OPENING THE RECORD'}</span><h2 id="record-detail-title">{entry?.state === 'DRAFT' ? <>Review this <em>draft.</em></> : <>A clear <em>record.</em></>}</h2>{entry && <span className={`record-status record-status-${entry.state.toLowerCase()}`}>{entry.state === 'POSTED' ? 'Confirmed' : entry.state === 'DRAFT' ? 'Draft · awaiting your review' : entry.state === 'DISCARDED' ? 'Discarded · draft preserved' : 'Reversed · original preserved'}</span>}</div>
    <div className="dialog-scroll detail-body">{loadError ? <div className="empty-state" role="alert"><p>{loadError}</p><button className="button button-dark" onClick={() => setRevision(n => n + 1)}>Reload record<Icon name="refresh" /></button></div> : !entry ? <p role="status">Opening this entry…</p> : <>
      <div className="record-detail-amount"><small>{entry.kind === 'RECEIVED' ? 'AMOUNT ALREADY RECEIVED' : kinds[entry.kind].toUpperCase()}</small><strong>{money(entry.amount_paise)}</strong></div>
      <dl className="record-facts"><div><dt>Description</dt><dd>{entry.description}</dd></div><div><dt>Entry date</dt><dd>{displayDate(entry.date)}</dd></div>{entry.kind === 'RECEIVED' && <><div><dt>Received from</dt><dd>{entry.payer}</dd></div><div><dt>Received via</dt><dd>{methods[entry.method]}</dd></div>{entry.reference && <div><dt>Reference</dt><dd>{entry.reference}</dd></div>}</>}{entry.receipt_number && <div><dt>Receipt number</dt><dd>{entry.receipt_number}</dd></div>}</dl>
      {(entry.state === 'REVERSED' || entry.state === 'DISCARDED') && <div className="record-reversal"><Icon name="refresh" /><div><strong>{entry.state === 'DISCARDED' ? 'This draft has been discarded.' : 'This entry has been reversed.'}</strong><p>{entry.reversal_reason}</p><small>{entry.reversed_by} · {new Date(entry.reversed_at * 1000).toLocaleDateString('en-IN', { timeZone: 'Asia/Kolkata' })}</small></div></div>}
      {entry.source_note && <dl className="record-facts"><div><dt>Source / evidence</dt><dd>{entry.source_note}</dd></div></dl>}
      <p className="record-provenance">Drafted by {entry.created_by}.{entry.posted_by && ` Confirmed by ${entry.posted_by} on ${new Date(entry.posted_at * 1000).toLocaleDateString('en-IN', { timeZone: 'Asia/Kolkata' })}.`}</p>
      {entry.receipt_id && <div className="receipt-ready"><Icon name="receipt" /><div><strong>{entry.pdf_state === 'READY' ? 'Your receipt is ready.' : entry.pdf_state === 'FAILED' ? 'The PDF needs another try.' : 'Preparing your receipt…'}</strong><p>{entry.pdf_state === 'READY' ? entry.state === 'REVERSED' ? 'This downloads the original receipt for the reversed entry.' : 'A PDF copy for your records.' : 'The confirmed entry and receipt number are safely recorded.'}</p></div>{entry.pdf_state === 'READY' ? <button className="button button-dark" disabled={busy} onClick={() => void download()}>{busy ? 'Downloading…' : entry.state === 'REVERSED' ? 'Download original' : 'Download PDF'}<Icon name="arrow" /></button> : entry.pdf_state === 'FAILED' && user.can_manage_records ? <button className="button button-dark" disabled={busy} onClick={() => void action('retry')}>Retry PDF<Icon name="refresh" /></button> : null}</div>}
      {user.can_manage_records && entry.state === 'DRAFT' && !correcting && <form className="portal-form record-confirm" onSubmit={event => { event.preventDefault(); void action('post') }}><label className="checkbox-label"><input type="checkbox" checked={confirmed} onChange={event => setConfirmed(event.target.checked)} disabled={busy} required />{entry.kind === 'RECEIVED' ? 'I reviewed these details and confirm the money was already received outside this portal.' : 'I reviewed this supplied amount, home, date and description.'}</label><button className="button button-dark" disabled={busy || !confirmed}>{busy ? 'Confirming your entry…' : 'Confirm this entry'}<Icon name="check" /></button></form>}
      {user.can_manage_records && (entry.state === 'POSTED' || entry.state === 'DRAFT') && (correcting ? <form className="portal-form record-confirm" onSubmit={event => { event.preventDefault(); void action(entry.state === 'DRAFT' ? 'discard' : 'reverse') }}><h3>{entry.state === 'DRAFT' ? 'Close this draft. Keep its history.' : 'Preserve the original. Correct the balance.'}</h3><p className="form-help">{entry.state === 'DRAFT' ? 'Discarding this draft leaves the balance unchanged. Create a new draft with the corrected details.' : 'A reversal removes this amount from the current totals. Add a new entry separately if the supplied details need replacing.'}</p><label>{entry.state === 'DRAFT' ? 'Reason for discarding draft' : 'Reason for reversal'}<input value={reason} disabled={busy || reversalLocked} onChange={event => setReason(event.target.value)} required minLength={5} maxLength={300} /></label><label className="checkbox-label"><input type="checkbox" checked={confirmed} disabled={busy} onChange={event => setConfirmed(event.target.checked)} required />{entry.state === 'DRAFT' ? 'I confirm this draft should be discarded.' : 'I confirm this entry should be reversed.'}</label><div className="record-actions"><button className="button button-dark" disabled={busy || !confirmed}>{busy ? 'Recording the correction…' : entry.state === 'DRAFT' ? 'Confirm discard' : 'Confirm reversal'}<Icon name="refresh" /></button><button type="button" className="text-link" disabled={busy} onClick={() => { setCorrecting(false); setConfirmed(false); setError('') }}>Keep entry</button></div></form> : <button className="text-link record-correction" disabled={busy} onClick={() => { setCorrecting(true); setConfirmed(false); setError('') }}>{entry.state === 'DRAFT' ? 'Discard this draft' : 'Correct this entry'}<Icon name="refresh" /></button>)}
      {error && <p ref={feedback} className="form-error" role="alert">{error}</p>}
    </>}</div>
  </PortalDialog>
}
