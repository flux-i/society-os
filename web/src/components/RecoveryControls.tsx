import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { APIError, mutate, request } from '../api'
import type { User } from '../api'
import type { RecoveryProposal } from '../recovery-review'
import { Icon } from './Icon'
import { PortalDialog } from './PortalDialog'

export function useRecoveryRead<T>(path: string, revision = 0) {
  const [snapshot, setSnapshot] = useState<{ path: string; revision: number; data: T | null; error: string; busy: boolean }>({ path, revision, data: null, error: '', busy: Boolean(path) })
  useEffect(() => { const controller = new AbortController(); setSnapshot({ path, revision, data: null, error: '', busy: Boolean(path) })
    if (path) void request<T>(path, controller.signal).then(value => { if (!controller.signal.aborted) setSnapshot({ path, revision, data: value, error: '', busy: false }) }).catch((err: Error) => { if (!controller.signal.aborted) setSnapshot({ path, revision, data: null, error: err.message, busy: false }) })
    return () => controller.abort()
  }, [path, revision])
  return snapshot.path === path && snapshot.revision === revision ? snapshot : { data: null, error: '', busy: Boolean(path) }
}

export function Pager({ page, total, onPage }: { page: number; total: number; onPage: (page: number) => void }) {
  if (total <= 12 && page === 1) return null
  return <nav className="review-desk-pager" aria-label="Recovery result pages"><button type="button" className="button button-light" disabled={page <= 1} onClick={() => onPage(page - 1)}>Previous</button><span>Page {page} · {total} records</span><button type="button" className="button button-light" disabled={page * 12 >= total} onClick={() => onPage(page + 1)}>Next</button></nav>
}

type Frozen = { operation_key: string }
export function useRetainedWrite<T extends Frozen, R extends { id: string; state: string } = RecoveryProposal>(path: string, saved: (proposal: R) => void, conflicted?: () => void, operationsPath = '/api/recovery/operations/') {
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [uncertain, setUncertain] = useState(false)
  const frozen = useRef<T | null>(null), inFlight = useRef(false), alive = useRef(true)
  useEffect(() => { alive.current = true; return () => { alive.current = false } }, [])
  const finish = (value: R) => { if (!alive.current) return; frozen.current = null; setUncertain(false); saved(value) }
  const run = async (input: T) => {
    if (inFlight.current) return; inFlight.current = true; frozen.current = input; setBusy(true); setError('')
    try { finish(await mutate<R>(path, 'POST', input)) }
    catch (err) { if (alive.current) {
      if (!(err instanceof APIError) || err.status >= 500) { setUncertain(true); setError('The reply is uncertain. Keep these details and check the saved result, or retry this same submission.') }
      else if (err.code === 'reauthentication_required' && uncertain) { setUncertain(true); setError('Confirm your identity to retry this same submission. Its earlier result remains uncertain.'); window.dispatchEvent(new Event('session-recheck')) }
      else { frozen.current = null; setUncertain(false); setError((err as Error).message); if (err.code === 'reauthentication_required') window.dispatchEvent(new Event('session-recheck')); if (err.status === 409) conflicted?.() }
    } } finally { inFlight.current = false; if (alive.current) setBusy(false) }
  }
  const check = async () => {
    if (!frozen.current || inFlight.current) return; inFlight.current = true; setBusy(true); setError('')
    try { finish(await request<R>(operationsPath + encodeURIComponent(frozen.current.operation_key))) }
    catch (err) { if (alive.current) setError(err instanceof APIError && err.status === 404 ? 'No saved result is visible yet. Retry the same submission to resolve it; the details stay frozen.' : (err as Error).message) }
    finally { inFlight.current = false; if (alive.current) setBusy(false) }
  }
  return { busy, uncertain, error, run, held: busy || uncertain, check, retry: () => frozen.current && run(frozen.current) }
}

export function RecoveryReauth({ user, onUser, onClose }: { user: User; onUser: (user: User) => void; onClose: () => void }) {
  const [password, setPassword] = useState(''), [code, setCode] = useState(''), [recovery, setRecovery] = useState(false), [busy, setBusy] = useState(false), [error, setError] = useState('')
  const alive = useRef(true)
  useLayoutEffect(() => { alive.current = true; return () => { alive.current = false } }, [])
  const submit = async (event: FormEvent) => { event.preventDefault(); if (busy) return; setBusy(true); setError('')
    try { const me = await mutate<User>('/api/auth/reauthenticate', 'POST', { password, code, recovery }); if (!alive.current) return; onUser(me); onClose() }
    catch (err) { if (alive.current) setError((err as Error).message) } finally { if (alive.current) setBusy(false) }
  }
  return <PortalDialog titleId="recovery-confirm-title" closeLabel="Close identity confirmation" onClose={onClose} busy={busy} className="review-desk-dialog"><div className="dialog-heading review-desk-dialog-top"><span className="eyebrow">CURRENT REVIEW</span></div><div className="dialog-scroll"><span className="eyebrow">A LITTLE EXTRA CARE</span><h2 id="recovery-confirm-title">Confirm <em>it’s you.</em></h2><p className="form-help">{user.name}, confirm your password and a fresh authenticator code before recording a sensitive decision.</p><form className="portal-form" onSubmit={event => void submit(event)}><fieldset disabled={busy}><label>Current password<input type="password" autoComplete="current-password" required maxLength={256} value={password} onChange={event => setPassword(event.target.value)} /></label><label>{recovery ? 'Unused recovery code' : 'Authenticator code'}<input autoComplete="one-time-code" required maxLength={64} value={code} onChange={event => setCode(event.target.value)} /></label><label className="checkbox-label"><input type="checkbox" checked={recovery} onChange={event => { setRecovery(event.target.checked); setCode('') }} />Use an unused recovery code.</label></fieldset>{error && <p className="form-error" role="alert">{error}</p>}<button type="submit" className="button button-dark" disabled={busy}>{busy ? 'Confirming…' : 'Confirm identity'}<Icon name="shield" /></button></form></div></PortalDialog>
}

