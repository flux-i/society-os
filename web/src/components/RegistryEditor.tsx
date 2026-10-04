import { FormSelect } from './FilterSelect'
import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { mutate, request, statuses } from '../api'
import type { AuditEvent, FlatDetail, Person } from '../api'
import { Icon } from './Icon'

const currentDate = () => new Intl.DateTimeFormat('en-CA', { year: 'numeric', month: '2-digit', day: '2-digit', timeZone: 'Asia/Kolkata' }).format(new Date())
export const relationshipLabel = (value: string) => ({ OWNER: 'Owner', TENANT: 'Tenant', FAMILY: 'Family member', AUTHORIZED_OCCUPANT: 'Authorized occupant' })[value] ?? value

export function RegistryEditor({ detail, onSaved, onReload }: { detail: FlatDetail; onSaved: (message: string) => void; onReload: () => void }) {
  const [section, setSection] = useState<'occupancy' | 'add' | 'end'>('occupancy')
  const [status, setStatus] = useState(detail.status)
  const [reason, setReason] = useState('')
  const [kind, setKind] = useState('new')
  const [name, setName] = useState('')
  const [query, setQuery] = useState('')
  const [people, setPeople] = useState<Person[]>([])
  const [residentID, setResidentID] = useState('')
  const [relationship, setRelationship] = useState('OWNER')
  const [startDate, setStartDate] = useState(currentDate)
  const [primary, setPrimary] = useState(false)
  const [membershipID, setMembershipID] = useState('')
  const [endDate, setEndDate] = useState(currentDate)
  const [error, setError] = useState('')
  const [lookupError, setLookupError] = useState('')
  const [busy, setBusy] = useState(false)
  useEffect(() => {
    if (kind !== 'existing') return
    const controller = new AbortController()
    setPeople([]); setResidentID(''); setLookupError('')
    const timer = window.setTimeout(() => request<Person[]>(`/api/people?q=${encodeURIComponent(query)}`, controller.signal).then(setPeople).catch((err: Error) => { if (!controller.signal.aborted) setLookupError(err.message) }), 200)
    return () => { clearTimeout(timer); controller.abort() }
  }, [kind, query])
  const submit = async (event: FormEvent) => {
    event.preventDefault(); setBusy(true); setError('')
    const base = `/api/flats/${encodeURIComponent(detail.id)}`
    const common = { version: detail.version, reason: reason.trim() }
    try {
      if (section === 'occupancy') { await mutate(base, 'PATCH', { ...common, status }); onSaved('Occupancy updated. The change is recorded in history.') }
      else if (section === 'add') { await mutate(`${base}/members`, 'POST', { ...common, name: kind === 'new' ? name.trim() : '', resident_id: kind === 'existing' ? residentID : '', relationship, start_date: startDate, primary_contact: primary }); onSaved('Person added to this home. The relationship is recorded in history.') }
      else { await mutate(`${base}/members/${encodeURIComponent(membershipID)}/end`, 'POST', { ...common, end_date: endDate }); onSaved('Relationship ended. The original record is preserved in history.') }
    } catch (err) { setError((err as Error).message) }
    finally { setBusy(false) }
  }
  const changeSection = (value: typeof section) => { setSection(value); setReason(''); setError('') }
  const selected = detail.members.find(m => m.membership_id === membershipID)
  return <div className="registry-editor">
    <div className="editor-choices" aria-label="Registry action">{([['occupancy', 'Occupancy'], ['add', 'Add person'], ['end', 'End relationship']] as const).map(([value, label]) => <button key={value} type="button" disabled={busy} aria-pressed={section === value} onClick={() => changeSection(value)}>{label}</button>)}</div>
    <form onSubmit={submit} className="portal-form">
      {section === 'occupancy' ? <><h3>A home’s current chapter.</h3><p className="form-help">Occupancy describes the home. Owner and tenant counts come from active relationships.</p><label>Occupancy<FormSelect label="Occupancy" value={status} onChange={value => setStatus(value as FlatDetail['status'])} options={Object.entries(statuses).map(([value, label]) => ({ value, label }))} /></label></> : section === 'add' ? <><h3>Make room for someone.</h3><p className="form-help">Reuse an existing person for joint ownership or another home.</p><label>Person record<FormSelect label="Person record" value={kind} onChange={setKind} options={[{ value: 'new', label: 'Create a new person' }, { value: 'existing', label: 'Link an existing person' }]} /></label>
        {kind === 'new' ? <label>Full name<input value={name} onChange={e => setName(e.target.value)} required minLength={2} maxLength={120} autoComplete="off" /></label> : <><label>Find a person<input type="search" value={query} onChange={e => setQuery(e.target.value)} placeholder="Search by name" maxLength={100} autoComplete="off" /></label><label>Existing person<FormSelect label="Existing person" value={residentID} onChange={setResidentID} required options={[{ value: '', label: 'Choose a person…' }, ...people.map(p => ({ value: p.id, label: p.name }))]} /></label>{lookupError && <p className="form-error" role="alert">{lookupError}</p>}<p className="form-help">Showing up to 30 matches. Search to narrow the list.</p></>}
        <div className="form-pair"><label>Relationship<FormSelect label="Relationship" value={relationship} onChange={setRelationship} options={['OWNER', 'TENANT', 'FAMILY', 'AUTHORIZED_OCCUPANT'].map(value => ({ value, label: relationshipLabel(value) }))} /></label><label>Start date<input type="date" value={startDate} onChange={e => setStartDate(e.target.value)} required min="1900-01-01" max={currentDate()} /></label></div>
        <label className="checkbox-label"><input type="checkbox" checked={primary} onChange={e => setPrimary(e.target.checked)} />Make this person the primary contact</label>{primary && <p className="form-help">This replaces the current primary contact for this home.</p>}
      </> : <><h3>Keep the story. Close the chapter.</h3><p className="form-help">Ending a relationship removes current access to this home. Any other active home or role access continues.</p><label>Relationship to end<FormSelect label="Relationship to end" value={membershipID} onChange={setMembershipID} required options={[{ value: '', label: 'Choose a current relationship…' }, ...detail.members.filter(m => m.active).map(m => ({ value: m.membership_id, label: m.name + ' · ' + relationshipLabel(m.relationship) }))]} /></label><label>End date<input type="date" value={endDate} onChange={e => setEndDate(e.target.value)} required min={selected?.start_date ?? '1900-01-01'} max={currentDate()} /></label><p className="form-help">Access ends on this date. It must be after the start date. Update occupancy separately if the home’s status changed.</p></>}
      <label>Reason for this change<input value={reason} onChange={e => setReason(e.target.value)} required minLength={5} maxLength={300} placeholder="For example, tenant moved out on the recorded date" autoComplete="off" /></label>
      {error && <div className="form-error" role="alert"><p>{error}</p><button type="button" className="text-link" onClick={onReload}>Reload home<Icon name="refresh" /></button></div>}
      <div className="dialog-form-actions"><button className="button button-dark" disabled={busy || (section === 'occupancy' && status === detail.status)}>{busy ? 'Saving the change…' : section === 'occupancy' ? 'Save occupancy' : section === 'add' ? 'Add person to home' : 'End this relationship'}<Icon name="arrow" /></button></div>
      <p className="form-help"><Icon name="leaf" />Changes are saved with your name, date and reason.</p>
    </form>
  </div>
}

export function RegistryHistory({ id, version }: { id: string; version: number }) {
  const [events, setEvents] = useState<AuditEvent[] | null>(null)
  const [error, setError] = useState('')
  const [retry, setRetry] = useState(0)
  useEffect(() => {
    const controller = new AbortController(); setEvents(null); setError('')
    request<AuditEvent[]>(`/api/flats/${encodeURIComponent(id)}/activity`, controller.signal).then(setEvents).catch((err: Error) => { if (!controller.signal.aborted) setError(err.message) })
    return () => controller.abort()
  }, [id, version, retry])
  const action = (event: AuditEvent) => event.action === 'OCCUPANCY_CHANGED' ? `${statuses[String(event.before.status) as FlatDetail['status']]} → ${statuses[String(event.after.status) as FlatDetail['status']]}` : event.action === 'MEMBERSHIP_ADDED' ? `${String(event.after.name)} added as ${relationshipLabel(String(event.after.relationship)).toLowerCase()}` : `${String(event.before.name)} · relationship ended ${String(event.after.end_date)}`
  return <div className="registry-history"><span className="eyebrow">A CLEAR RECORD OF EVERY CHANGE</span><h3>This home’s history.</h3>{error ? <div className="form-error" role="alert">{error}<button className="text-link" onClick={() => setRetry(n => n + 1)}>Try again</button></div> : events === null ? <p className="form-help" role="status">Opening history…</p> : events.length === 0 ? <div className="history-empty"><Icon name="leaf" /><p>A fresh chapter.</p><span>Registry changes will appear here with who, when and why.</span></div> : <><p className="form-help">Latest {events.length} changes. Original records are preserved.</p><ol className="history-list">{events.map(event => <li key={event.id}><span className="history-dot" /><time dateTime={new Date(event.occurred_at * 1000).toISOString()}>{new Intl.DateTimeFormat('en-IN', { dateStyle: 'medium', timeStyle: 'short', timeZone: 'Asia/Kolkata' }).format(new Date(event.occurred_at * 1000))}</time><strong>{action(event)}</strong><p>{event.reason}</p><span>{event.actor}</span></li>)}</ol></>}</div>
}
