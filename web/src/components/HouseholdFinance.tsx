import { useEffect, useRef, useState } from 'react'
import { APIError, mutate, request, setSession } from '../api'
import type { User } from '../api'
import { Icon } from './Icon'
import { FormSelect } from './FilterSelect'
import { relationshipLabel } from './RegistryEditor'

interface Person { id: string; name: string; relationships: string[]; visible: boolean; all_visible: boolean; is_actor: boolean }
interface AccessPage { flat_id: string; home: string; version: number; items: Person[]; total: number; page: number; page_size: number }
interface AccessInput { operation_key: string; person_id: string; version: number; action: 'GRANT' | 'REVOKE'; confirmed: boolean; note: string }
interface AccessResult { id: string; person_name: string; visible: boolean; version: number }

export function HouseholdFinance({ homeId, onBusy, onSaved, onUser }: { homeId: string; onBusy: (busy: boolean) => void; onSaved: (version: number) => void; onUser: (user: User) => void }) {
  const [data, setData] = useState<AccessPage | null>(null)
  const [query, setQuery] = useState(''), [page, setPage] = useState(1), [reload, setReload] = useState(0)
  const [readError, setReadError] = useState(''), [actionError, setActionError] = useState(''), [message, setMessage] = useState('')
  const [person, setPerson] = useState<Person | null>(null), [action, setAction] = useState<'GRANT' | 'REVOKE'>('GRANT')
  const [note, setNote] = useState(''), [confirmed, setConfirmed] = useState(false), [busy, setBusy] = useState(false)
  const [frozen, setFrozen] = useState<AccessInput | null>(null), [needsAuth, setNeedsAuth] = useState(false)
  const [password, setPassword] = useState(''), [code, setCode] = useState(''), [recovery, setRecovery] = useState(false)
  const alive = useRef(true)
  useEffect(() => { alive.current = true; return () => { alive.current = false } }, [])
  useEffect(() => { onBusy(busy); return () => onBusy(false) }, [busy, onBusy])
  useEffect(() => {
    const controller = new AbortController()
    setReadError(''); setData(null)
    void request<AccessPage>(`/api/flats/${encodeURIComponent(homeId)}/finance-visibility?q=${encodeURIComponent(query.trim())}&page=${page}`, controller.signal).then(value => { if (!controller.signal.aborted) setData(value) }).catch(err => { if (!controller.signal.aborted) setReadError((err as Error).message) })
    return () => controller.abort()
  }, [homeId, query, page, reload])
  const done = (result: AccessResult) => {
    if (!alive.current) return
    setFrozen(null); setPerson(null); setNote(''); setConfirmed(false); setActionError(''); setNeedsAuth(false)
    setMessage(`The original ${result.visible ? 'grant' : 'revocation'} for ${result.person_name} is recorded. Current household access is refreshed below.`)
    setReload(value => value + 1); onSaved(result.version)
  }
  const save = async () => {
    if (!data || !person || busy || (!frozen && !confirmed)) return
    const input = frozen ?? { operation_key: crypto.randomUUID(), person_id: person.id, version: data.version, action, confirmed, note: note.trim() }
    setFrozen(input); setBusy(true); setActionError(''); setMessage('')
    try { done(await mutate<AccessResult>(`/api/flats/${encodeURIComponent(homeId)}/finance-visibility`, 'POST', input)) }
    catch (err) {
      if (!alive.current) return
      setActionError((err as Error).message)
      if (err instanceof APIError && err.code === 'reauthentication_required') { setNeedsAuth(true); setActionError('Confirm your identity below, then retry the retained access decision.') }
      // Definite rejected validation is editable; uncertain replies retain the exact request.
      if (err instanceof APIError && [400, 404, 409, 422].includes(err.status)) setFrozen(null)
    } finally { if (alive.current) setBusy(false) }
  }
  const checkSaved = async () => {
    if (!frozen || busy) return
    setBusy(true); setActionError('')
    try { done(await request<AccessResult>(`/api/flats/${encodeURIComponent(homeId)}/finance-visibility/operations/${encodeURIComponent(frozen.operation_key)}`)) }
    catch (err) { if (alive.current) setActionError(err instanceof APIError && err.status === 404 ? 'No saved reply was found. Retry the same frozen request deliberately; no new request has been sent.' : (err as Error).message) }
    finally { if (alive.current) setBusy(false) }
  }
  const reauthenticate = async () => {
    if (busy) return
    setBusy(true); setActionError('')
    try {
      const current = await mutate<User>('/api/auth/reauthenticate', 'POST', { password, code, recovery })
      if (!alive.current) return
      setSession(current); onUser(current); setPassword(''); setCode(''); setNeedsAuth(false)
      setMessage('Identity confirmed. Retry the retained request when you are ready.')
    } catch (err) { if (alive.current) setActionError((err as Error).message) }
    finally { if (alive.current) setBusy(false) }
  }
  return <section className="household-finance">
    <span className="eyebrow">A CONSIDERED VIEW</span><h3>Financial <em>access.</em></h3>
    <p className="detail-intro">Decide who can see this home’s financial history. Staff appointments remain separate.</p>
    {message && <p className="form-success" role="status">{message}</p>}
    {readError ? <div className="form-error" role="alert"><p>{readError}</p><button className="text-link" disabled={busy} onClick={() => setReload(value => value + 1)}>Try financial access again<Icon name="refresh" /></button></div> : !data ? <p role="status">Opening current financial access…</p> : <>
      <label className="search-control household-finance-search"><Icon name="search" /><input type="search" aria-label="Search financial access" placeholder="Find a current person…" value={query} maxLength={100} disabled={busy || Boolean(person)} onChange={event => { setPage(1); setQuery(event.target.value) }} /></label>
      {!data.items.length ? <div className="empty-state"><h3>{query ? 'No current people match.' : 'No current relationships.'}</h3><p>{query ? 'Clear your search to review this home.' : 'Verified current relationships are needed before financial access can be granted.'}</p>{query && <button className="text-link" onClick={() => { setQuery(''); setPage(1) }}>Clear financial search<Icon name="close" /></button>}</div> : <div className="household-finance-people">{data.items.map(value => <article key={value.id} className="household-finance-person">
        <div><strong>{value.name}</strong><span>{value.relationships.map(relationshipLabel).join(' · ')}</span>{value.is_actor && <small>Another finance officer can grant your personal household view.</small>}</div>
        <span className={`status ${value.visible ? 'status-owner_occupied' : 'status-vacant'}`}><i />{value.visible ? 'Allowed' : 'Withheld'}</span>
        <button type="button" className="text-link" aria-label={`Review financial access for ${value.name}`} disabled={busy || Boolean(person) || (value.is_actor && !value.visible)} onClick={() => { setPerson(value); setAction(value.visible ? 'REVOKE' : 'GRANT'); setNote(''); setConfirmed(false); setFrozen(null); setActionError(''); setMessage('') }}>Review access<Icon name="arrow" /></button>
      </article>)}</div>}
      {data.total > data.page_size && <div className="pagination"><span>{data.total} current people · Page {data.page}</span><div><button aria-label="Previous financial access page" disabled={busy || Boolean(person) || page === 1} onClick={() => setPage(value => value - 1)}><Icon name="left" /></button><button aria-label="Next financial access page" disabled={busy || Boolean(person) || page * data.page_size >= data.total} onClick={() => setPage(value => value + 1)}><Icon name="chevron" /></button></div></div>}
      {person && <div className="household-finance-review">
        <span className="eyebrow">ONE DELIBERATE DECISION</span><h4>{person.name}</h4><p>Update all their current relationships in {data.home}. Past relationships and other homes keep their existing access.</p>
        <form className="portal-form" onSubmit={event => { event.preventDefault(); void save() }}>
          <fieldset className="household-finance-fields" disabled={busy || Boolean(frozen)}>
            <label>Household view<FormSelect label="Household financial visibility" value={action} disabled={busy || Boolean(frozen)} onChange={value => setAction(value as 'GRANT' | 'REVOKE')} options={person.is_actor ? [{ value: 'REVOKE', label: 'Withhold household view' }] : [{ value: 'GRANT', label: 'Allow household view' }, { value: 'REVOKE', label: 'Withhold household view' }]} /></label>
            <label>Verification note<textarea aria-label="Financial access verification note" value={note} onChange={event => setNote(event.target.value)} minLength={10} maxLength={300} rows={3} required /></label>
            <label className="checkbox-label"><input type="checkbox" checked={confirmed} onChange={event => setConfirmed(event.target.checked)} required />I verified this person’s current household relationship and financial access.</label>
          </fieldset>
          {actionError && <div className="form-error" role="alert"><p>{actionError}</p>{frozen && <button className="text-link" type="button" disabled={busy} onClick={() => void checkSaved()}>Check saved access decision<Icon name="refresh" /></button>}{!frozen && <button className="text-link" type="button" disabled={busy} onClick={() => setReload(value => value + 1)}>Reload current people<Icon name="refresh" /></button>}</div>}
          <button className="button button-dark" disabled={busy || needsAuth || (!frozen && !confirmed)}>{busy ? 'Recording this decision…' : frozen ? 'Retry the same access decision' : action === 'GRANT' ? 'Allow household view' : 'Withhold household view'}<Icon name="check" /></button>
          {!frozen && <button className="text-link" type="button" disabled={busy} onClick={() => { setPerson(null); setActionError(''); setNeedsAuth(false) }}>Back to current people<Icon name="left" /></button>}
        </form>
        {needsAuth && <form className="portal-form household-finance-reauth" onSubmit={event => { event.preventDefault(); void reauthenticate() }}><h4>Confirm it’s you.</h4><p>Your verification note and original request are kept.</p><label>Current password<input type="password" autoComplete="current-password" value={password} onChange={event => setPassword(event.target.value)} maxLength={256} required disabled={busy} /></label><label>{recovery ? 'Recovery code' : 'Authenticator code'}<input value={code} onChange={event => setCode(event.target.value)} autoComplete="one-time-code" inputMode={recovery ? 'text' : 'numeric'} maxLength={64} required disabled={busy} /></label><button type="button" className="text-link" disabled={busy} onClick={() => { setRecovery(value => !value); setCode('') }}>{recovery ? 'Use my authenticator app' : 'Use a recovery code'}</button><button className="button button-outline" disabled={busy}>Confirm my identity<Icon name="key" /></button></form>}
      </div>}
    </>}
  </section>
}
